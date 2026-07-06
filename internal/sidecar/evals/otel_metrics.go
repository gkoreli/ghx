package evals

import (
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/telemetry"
	"go.opentelemetry.io/otel/attribute"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

const (
	metricFileName = telemetry.MetricFileName

	genAIClientOperationDurationMetric = "gen_ai.client.operation.duration"
	genAIClientTokenUsageMetric        = "gen_ai.client.token.usage"
	genAITokenTypeAttribute            = "gen_ai.token.type"

	ghxEvalRewardMetric     = "ghx.eval.reward"
	ghxEvalAnomalyCount     = "ghx.eval.anomaly.count"
	ghxEvalReportSizeMetric = "ghx.eval.report.size"
)

// EmitEpisodeMetrics appends one OTLP MetricsData record for the episode to
// <run-dir>/metrics.jsonl. The generic OTLP metric builders and file writer
// live in the telemetry package (ADR-0022 D1); this keeps only the
// eval-specific metric shaping (GenAI token/duration plus reward, anomaly, and
// report-size metrics) so eval output stays byte-identical.
func EmitEpisodeMetrics(runDir string, ep *Episode) error {
	if ep == nil {
		return nil
	}
	metrics := episodeMetrics(ep)
	if len(metrics) == 0 {
		return nil
	}
	return telemetry.WriteMetrics(filepath.Join(runDir, metricFileName), resourceAttributes(runDir, ep), tracerName, otelSemconvVersion, metrics)
}

func episodeMetrics(ep *Episode) []*metricspb.Metric {
	start := nonZeroTime(ep.StartedAt, time.Now().UTC())
	end := nonZeroTime(ep.EndedAt, start)
	base := []attribute.KeyValue{
		attribute.String("gen_ai.system", "ghx-evals"),
		attribute.String("ghx.eval.episode_id", ep.ID),
		attribute.String("ghx.eval.task_id", ep.TaskID),
		attribute.String("ghx.eval.profile", string(ep.Profile)),
	}
	// Duration honesty (ADR-0025 D3): an episode that ran concurrently with
	// others carries ghx.eval.parallel=true so latency analysis can exclude
	// it — wall-clock duration under parallelism is contended, not a clean
	// per-episode latency. The marker rides on every metric point, including
	// gen_ai.client.operation.duration.
	if ep.Parallel {
		base = telemetry.AppendAttrs(base, attribute.Bool(ghxEvalParallelAttr, true))
	}

	var durationPoints []*metricspb.HistogramDataPoint
	var tokenPoints []*metricspb.NumberDataPoint
	for _, turn := range ep.Turns {
		turnAttrs := telemetry.AppendAttrs(base,
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.Int("ghx.eval.turn", turn.Turn),
		)
		if turn.DurationMs > 0 {
			durationPoints = append(durationPoints, telemetry.HistogramPoint(start, end, float64(turn.DurationMs)/1000, turnAttrs))
		}
		tokenPoints = append(tokenPoints,
			telemetry.IntPoint(start, end, int64(len(turn.Question)), telemetry.AppendAttrs(turnAttrs, attribute.String(genAITokenTypeAttribute, "input"))),
			telemetry.IntPoint(start, end, int64(len(turn.Text)), telemetry.AppendAttrs(turnAttrs, attribute.String(genAITokenTypeAttribute, "output"))),
		)
		if turn.Thinking != "" {
			tokenPoints = append(tokenPoints,
				telemetry.IntPoint(start, end, int64(len(turn.Thinking)), telemetry.AppendAttrs(turnAttrs, attribute.String(genAITokenTypeAttribute, "reasoning"))),
			)
		}
	}

	var metrics []*metricspb.Metric
	if len(durationPoints) > 0 {
		metrics = append(metrics, telemetry.HistogramMetric(
			genAIClientOperationDurationMetric,
			"Duration of GenAI client operations.",
			"s",
			durationPoints,
		))
	}
	if len(tokenPoints) > 0 {
		metrics = append(metrics, telemetry.SumMetric(
			genAIClientTokenUsageMetric,
			"Number of input, output, and reasoning tokens used by GenAI operations.",
			"{token}",
			tokenPoints,
			true,
		))
	}

	metrics = append(metrics, rewardMetric(ep, start, end, base))
	if anomalyMetric := anomaliesMetric(ep, start, end, base); anomalyMetric != nil {
		metrics = append(metrics, anomalyMetric)
	}
	if reportMetric := reportSizeMetric(ep, start, end, base); reportMetric != nil {
		metrics = append(metrics, reportMetric)
	}
	return metrics
}

func rewardMetric(ep *Episode, start, end time.Time, base []attribute.KeyValue) *metricspb.Metric {
	r := ep.Rewards
	components := []struct {
		name  string
		value float64
	}{
		{"correctness", r.Correctness},
		{"evidence", r.Evidence},
		{"trajectory", r.Trajectory},
		{"compression", r.Compression},
		{"memory", r.Memory},
		{"safety", r.Safety},
		{"overall", r.Overall},
	}
	points := make([]*metricspb.HistogramDataPoint, 0, len(components))
	for _, component := range components {
		attrs := telemetry.AppendAttrs(base, attribute.String("ghx.eval.reward.component", component.name))
		if component.name == "memory" {
			attrs = telemetry.AppendAttrs(attrs, attribute.Bool("ghx.eval.reward.memory_applies", r.MemoryApplies))
		}
		points = append(points, telemetry.HistogramPoint(start, end, component.value, attrs))
	}
	return telemetry.HistogramMetric(ghxEvalRewardMetric, "ghx deterministic reward components by task and profile.", "1", points)
}

func anomaliesMetric(ep *Episode, start, end time.Time, base []attribute.KeyValue) *metricspb.Metric {
	if len(ep.Anomalies) == 0 {
		return nil
	}
	counts := map[string]int64{}
	severity := map[string]string{}
	for _, anomaly := range ep.Anomalies {
		counts[anomaly.Kind]++
		severity[anomaly.Kind] = string(anomaly.Severity)
	}
	points := make([]*metricspb.NumberDataPoint, 0, len(counts))
	for kind, count := range counts {
		points = append(points, telemetry.IntPoint(start, end, count, telemetry.AppendAttrs(base,
			attribute.String("ghx.eval.anomaly.kind", kind),
			attribute.String("ghx.eval.anomaly.severity", severity[kind]),
		)))
	}
	return telemetry.SumMetric(ghxEvalAnomalyCount, "ghx anomaly counts by declarative taxonomy kind.", "{anomaly}", points, true)
}

func reportSizeMetric(ep *Episode, start, end time.Time, base []attribute.KeyValue) *metricspb.Metric {
	var points []*metricspb.HistogramDataPoint
	for _, turn := range ep.Turns {
		if turn.Report == nil {
			continue
		}
		data, err := json.Marshal(turn.Report)
		if err != nil {
			continue
		}
		points = append(points, telemetry.HistogramPoint(start, end, float64(len(data)), telemetry.AppendAttrs(base, attribute.Int("ghx.eval.turn", turn.Turn))))
	}
	if ep.Report != nil {
		data, err := json.Marshal(ep.Report)
		if err == nil {
			points = append(points, telemetry.HistogramPoint(start, end, float64(len(data)), telemetry.AppendAttrs(base, attribute.String("ghx.eval.report.scope", "episode"))))
		}
	}
	if len(points) == 0 {
		return nil
	}
	return telemetry.HistogramMetric(ghxEvalReportSizeMetric, "Serialized ghx sidecar report size distribution.", "By", points)
}
