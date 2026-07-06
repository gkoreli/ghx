package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	metricFileName = "metrics.jsonl"

	genAIClientOperationDurationMetric = "gen_ai.client.operation.duration"
	genAIClientTokenUsageMetric        = "gen_ai.client.token.usage"
	genAITokenTypeAttribute            = "gen_ai.token.type"

	ghxEvalRewardMetric     = "ghx.eval.reward"
	ghxEvalAnomalyCount     = "ghx.eval.anomaly.count"
	ghxEvalReportSizeMetric = "ghx.eval.report.size"
)

func EmitEpisodeMetrics(runDir string, ep *Episode) error {
	if ep == nil {
		return nil
	}
	metrics := episodeMetrics(ep)
	if len(metrics) == 0 {
		return nil
	}
	data := &metricspb.MetricsData{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource:     protoMetricResource(runDir, ep),
				ScopeMetrics: []*metricspb.ScopeMetrics{{Scope: instrumentationScopeName(), Metrics: metrics}},
			},
		},
	}
	line, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal OTLP metrics json: %w", err)
	}
	line = append(line, '\n')

	path := filepath.Join(runDir, metricFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

func protoMetricResource(runDir string, ep *Episode) *resourcepb.Resource {
	return protoResource(sdkresource.NewSchemaless(resourceAttributes(runDir, ep)...))
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

	var durationPoints []*metricspb.HistogramDataPoint
	var tokenPoints []*metricspb.NumberDataPoint
	for _, turn := range ep.Turns {
		turnAttrs := appendAttrs(base,
			attribute.String("gen_ai.operation.name", "chat"),
			attribute.Int("ghx.eval.turn", turn.Turn),
		)
		if turn.DurationMs > 0 {
			durationPoints = append(durationPoints, histogramPoint(start, end, float64(turn.DurationMs)/1000, turnAttrs))
		}
		tokenPoints = append(tokenPoints,
			intPoint(start, end, int64(len(turn.Question)), appendAttrs(turnAttrs, attribute.String(genAITokenTypeAttribute, "input"))),
			intPoint(start, end, int64(len(turn.Text)), appendAttrs(turnAttrs, attribute.String(genAITokenTypeAttribute, "output"))),
		)
		if turn.Thinking != "" {
			tokenPoints = append(tokenPoints,
				intPoint(start, end, int64(len(turn.Thinking)), appendAttrs(turnAttrs, attribute.String(genAITokenTypeAttribute, "reasoning"))),
			)
		}
	}

	var metrics []*metricspb.Metric
	if len(durationPoints) > 0 {
		metrics = append(metrics, histogramMetric(
			genAIClientOperationDurationMetric,
			"Duration of GenAI client operations.",
			"s",
			durationPoints,
		))
	}
	if len(tokenPoints) > 0 {
		metrics = append(metrics, sumMetric(
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
		attrs := appendAttrs(base, attribute.String("ghx.eval.reward.component", component.name))
		if component.name == "memory" {
			attrs = appendAttrs(attrs, attribute.Bool("ghx.eval.reward.memory_applies", r.MemoryApplies))
		}
		points = append(points, histogramPoint(start, end, component.value, attrs))
	}
	return histogramMetric(ghxEvalRewardMetric, "ghx deterministic reward components by task and profile.", "1", points)
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
		points = append(points, intPoint(start, end, count, appendAttrs(base,
			attribute.String("ghx.eval.anomaly.kind", kind),
			attribute.String("ghx.eval.anomaly.severity", severity[kind]),
		)))
	}
	return sumMetric(ghxEvalAnomalyCount, "ghx anomaly counts by declarative taxonomy kind.", "{anomaly}", points, true)
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
		points = append(points, histogramPoint(start, end, float64(len(data)), appendAttrs(base, attribute.Int("ghx.eval.turn", turn.Turn))))
	}
	if ep.Report != nil {
		data, err := json.Marshal(ep.Report)
		if err == nil {
			points = append(points, histogramPoint(start, end, float64(len(data)), appendAttrs(base, attribute.String("ghx.eval.report.scope", "episode"))))
		}
	}
	if len(points) == 0 {
		return nil
	}
	return histogramMetric(ghxEvalReportSizeMetric, "Serialized ghx sidecar report size distribution.", "By", points)
}

func histogramMetric(name, description, unit string, points []*metricspb.HistogramDataPoint) *metricspb.Metric {
	return &metricspb.Metric{
		Name:        name,
		Description: description,
		Unit:        unit,
		Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{
			DataPoints:             points,
			AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
		}},
	}
}

func histogramPoint(start, end time.Time, value float64, attrs []attribute.KeyValue) *metricspb.HistogramDataPoint {
	return &metricspb.HistogramDataPoint{
		Attributes:        keyValues(attrs),
		StartTimeUnixNano: unixNano(start),
		TimeUnixNano:      unixNano(end),
		Count:             1,
		Sum:               &value,
		Min:               &value,
		Max:               &value,
	}
}

func sumMetric(name, description, unit string, points []*metricspb.NumberDataPoint, monotonic bool) *metricspb.Metric {
	return &metricspb.Metric{
		Name:        name,
		Description: description,
		Unit:        unit,
		Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
			DataPoints:             points,
			AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
			IsMonotonic:            monotonic,
		}},
	}
}

func intPoint(start, end time.Time, value int64, attrs []attribute.KeyValue) *metricspb.NumberDataPoint {
	return &metricspb.NumberDataPoint{
		Attributes:        keyValues(attrs),
		StartTimeUnixNano: unixNano(start),
		TimeUnixNano:      unixNano(end),
		Value:             &metricspb.NumberDataPoint_AsInt{AsInt: value},
	}
}

func appendAttrs(base []attribute.KeyValue, extra ...attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

func unixNano(t time.Time) uint64 {
	return uint64(maxInt64(0, t.UnixNano()))
}
