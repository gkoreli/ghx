package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/telemetry"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	otelSemconvVersion = "1.41.0"
	tracerName         = "github.com/gkoreli/ghx/v2/internal/sidecar/evals"
	// traceFileName mirrors the shared telemetry artifact name so the eval
	// report reader (TraceFileStats) keeps referencing it locally.
	traceFileName = telemetry.TraceFileName
)

// EmitEpisodeTraces appends an OTLP JSON File representation of one completed
// episode to <run-dir>/traces.jsonl. The generic OTLP machinery (tracer
// provider, file exporter, log/metric writers) now lives in the shared
// telemetry package (ADR-0022 D1); this function keeps only the eval-specific
// span/attribute shaping so eval artifact output stays byte-identical.
func EmitEpisodeTraces(ctx context.Context, runDir string, ep *Episode) error {
	if ep == nil {
		return nil
	}
	tp, err := telemetry.NewTracerProvider(ctx, runDir, resourceAttributes(runDir, ep))
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutdownCtx)
	}()

	tracer := tp.Tracer(tracerName, trace.WithInstrumentationVersion(otelSemconvVersion))
	logs := emitEpisodeSpans(ctx, tracer, ep)
	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("flush episode traces: %w", err)
	}
	if err := telemetry.WriteLogs(filepath.Join(runDir, logFileName), resourceAttributes(runDir, ep), tracerName, otelSemconvVersion, logs); err != nil {
		return err
	}
	return nil
}

func resourceAttributes(runDir string, ep *Episode) []attribute.KeyValue {
	id := ep.Identity
	return []attribute.KeyValue{
		semconv.ServiceName("ghx-evals"),
		attribute.String("otel.semconv_version", otelSemconvVersion),
		attribute.String("ghx.eval.run_id", filepath.Base(runDir)),
		attribute.String("ghx.eval.task_id", ep.TaskID),
		attribute.String("ghx.eval.profile", string(ep.Profile)),
		attribute.String("ghx.eval.episode_id", ep.ID),
		attribute.String("ghx.eval.subject_model", id.SubjectModel),
		attribute.String("ghx.eval.adapter.name", id.AdapterName),
		attribute.String("ghx.eval.adapter.version", id.AdapterVersion),
	}
}

func emitEpisodeSpans(ctx context.Context, tracer trace.Tracer, ep *Episode) []telemetry.LogRecord {
	var logs []telemetry.LogRecord
	start := nonZeroTime(ep.StartedAt, time.Now().UTC())
	end := nonZeroTime(ep.EndedAt, start)
	episodeCtx, episodeSpan := tracer.Start(ctx, "eval.episode",
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(episodeAttributes(ep)...),
	)

	for _, turn := range ep.Turns {
		logs = append(logs, emitTurnSpan(episodeCtx, tracer, ep, turn)...)
	}
	for _, anomaly := range ep.Anomalies {
		episodeSpan.AddEvent("ghx.eval.anomaly",
			trace.WithAttributes(
				attribute.String("ghx.eval.anomaly.kind", anomaly.Kind),
				attribute.String("ghx.eval.anomaly.severity", string(anomaly.Severity)),
				attribute.Int("ghx.eval.anomaly.turn", anomaly.Turn),
				attribute.String("ghx.eval.anomaly.detail", anomaly.Detail),
			),
		)
	}
	emitRewardSpan(episodeCtx, tracer, ep)
	if ep.Invalid {
		episodeSpan.SetAttributes(attribute.Bool("ghx.eval.invalid", true))
	}
	episodeSpan.End(trace.WithTimestamp(end))
	return logs
}

func emitTurnSpan(parent context.Context, tracer trace.Tracer, ep *Episode, turn TurnRecord) []telemetry.LogRecord {
	start := turnStart(ep, turn)
	end := start.Add(time.Duration(turn.DurationMs) * time.Millisecond)
	turnCtx, span := tracer.Start(parent, "eval.turn",
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(turnAttributes(ep, turn)...),
	)
	if turn.Error != "" {
		span.RecordError(errors.New(turn.Error))
		span.SetAttributes(attribute.String("error.type", "turn"))
	}
	for _, tool := range turn.ToolTraces {
		emitToolSpan(turnCtx, tracer, turn, tool)
	}
	logs := turnContentLogs(span.SpanContext(), ep, turn, start)
	span.End(trace.WithTimestamp(nonZeroTime(end, start)))
	return logs
}

func emitToolSpan(parent context.Context, tracer trace.Tracer, turn TurnRecord, tool ToolCallTrace) {
	start := nonZeroTime(firstStatusTime(tool), time.Now().UTC())
	end := nonZeroTime(lastStatusTime(tool), start)
	name := "tool.call"
	if tool.Kind != "" {
		name = "tool." + tool.Kind
	}
	_, span := tracer.Start(parent, name,
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(toolAttributes(turn, tool)...),
	)
	span.End(trace.WithTimestamp(end))
}

func emitRewardSpan(parent context.Context, tracer trace.Tracer, ep *Episode) {
	start := nonZeroTime(ep.EndedAt, time.Now().UTC())
	_, span := tracer.Start(parent, "eval.reward.compute",
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(rewardAttributes(ep)...),
	)
	span.SetAttributes(attribute.String("ghx.eval.reward.summary", rewardExplanationSummary(ep)))
	addRewardExplanationEvents(span, ep)
	span.End(trace.WithTimestamp(start))
}

func episodeAttributes(ep *Episode) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameInvokeAgent,
		attribute.String("gen_ai.system", "ghx-evals"),
		semconv.GenAIRequestModel(ep.Identity.SubjectModel),
		semconv.GenAIResponseModel(firstNonEmpty(ep.Identity.AdapterSubjectModel, ep.Identity.SubjectModel)),
		attribute.String("gen_ai.agent.name", "ghx-eval-subject"),
		attribute.String("ghx.eval.episode_id", ep.ID),
		attribute.String("ghx.eval.task_id", ep.TaskID),
		attribute.String("ghx.eval.repo", ep.Repo),
		attribute.String("ghx.eval.profile", string(ep.Profile)),
		attribute.String("ghx.eval.subject_model", ep.Identity.SubjectModel),
		attribute.String("ghx.eval.adapter.name", ep.Identity.AdapterName),
		attribute.String("ghx.eval.adapter.version", ep.Identity.AdapterVersion),
		attribute.Int("ghx.eval.turn_count", len(ep.Turns)),
		attribute.Int("ghx.eval.action_count", len(ep.Actions)),
		attribute.Int("ghx.eval.main_agent_chars", ep.Context.MainAgentChars),
		attribute.Int("ghx.eval.sidecar_internal_chars", ep.Context.SidecarInternalChars),
		attribute.Int("ghx.eval.total_workflow_chars", ep.Context.TotalWorkflowChars),
	}
	if ep.Identity.AdapterSubjectModel != "" {
		attrs = append(attrs, attribute.String("ghx.eval.adapter_subject_model", ep.Identity.AdapterSubjectModel))
	}
	if len(ep.ExclusionReasons) > 0 {
		attrs = append(attrs, attribute.StringSlice("ghx.eval.exclusion_reasons", ep.ExclusionReasons))
	}
	return attrs
}

func turnAttributes(ep *Episode, turn TurnRecord) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameChat,
		attribute.String("gen_ai.system", "ghx-evals"),
		semconv.GenAIRequestModel(ep.Identity.SubjectModel),
		semconv.GenAIUsageInputTokens(len(turn.Question)),
		semconv.GenAIUsageOutputTokens(len(turn.Text)),
		attribute.Int("ghx.eval.turn", turn.Turn),
		attribute.Int("ghx.eval.question_chars", len(turn.Question)),
		attribute.Int("ghx.eval.response_chars", len(turn.Text)),
		attribute.Int("ghx.eval.tool_output_chars", turn.ToolOutputChars),
		attribute.Int("ghx.eval.tool_call_count", len(turn.ToolTraces)),
		attribute.Bool("ghx.eval.resumed", turn.Resumed),
	}
	if turn.Error != "" {
		attrs = append(attrs, attribute.String("error.message", boundedString(turn.Error, 512)))
	}
	if turn.Thinking != "" {
		attrs = append(attrs, attribute.Int(genAIUsageReasoningOutputTokensAttr, len(turn.Thinking)))
	}
	return attrs
}

func toolAttributes(turn TurnRecord, tool ToolCallTrace) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameExecuteTool,
		attribute.String("gen_ai.system", "ghx-evals"),
		attribute.String("gen_ai.tool.call.id", tool.ID),
		attribute.String("gen_ai.tool.name", firstNonEmpty(tool.Kind, "tool")),
		attribute.Int("ghx.eval.turn", turn.Turn),
		attribute.String("ghx.eval.tool.kind", tool.Kind),
		attribute.String("ghx.eval.tool.title", boundedString(tool.Title, 512)),
		attribute.String("ghx.eval.tool.input", boundedString(resolvedToolInput(tool), 2048)),
		attribute.Int("ghx.eval.tool.output_chars", tool.OutputSize),
		attribute.Int("ghx.eval.tool.output_excerpt_chars", len(tool.OutputExcerpt)),
	}
	if status := lastStatus(tool); status != "" {
		attrs = append(attrs, attribute.String("ghx.eval.tool.status", status))
	}
	if len(tool.StatusTransitions) > 0 {
		statuses := make([]string, 0, len(tool.StatusTransitions))
		for _, st := range tool.StatusTransitions {
			statuses = append(statuses, st.Status)
		}
		attrs = append(attrs, attribute.StringSlice("ghx.eval.tool.status_transitions", statuses))
	}
	if tool.OutputExcerpt != "" {
		attrs = append(attrs, attribute.String("ghx.eval.tool.output_excerpt", boundedString(tool.OutputExcerpt, 2048)))
	}
	return attrs
}

func rewardAttributes(ep *Episode) []attribute.KeyValue {
	r := ep.Rewards
	return []attribute.KeyValue{
		attribute.Float64("ghx.eval.reward.correctness", r.Correctness),
		attribute.Float64("ghx.eval.reward.evidence", r.Evidence),
		attribute.Float64("ghx.eval.reward.trajectory", r.Trajectory),
		attribute.Float64("ghx.eval.reward.compression", r.Compression),
		attribute.Float64("ghx.eval.reward.memory", r.Memory),
		attribute.Bool("ghx.eval.reward.memory_applies", r.MemoryApplies),
		attribute.Float64("ghx.eval.reward.safety", r.Safety),
		attribute.Float64("ghx.eval.reward.overall", r.Overall),
	}
}

func turnStart(ep *Episode, turn TurnRecord) time.Time {
	for _, tool := range turn.ToolTraces {
		if t := firstStatusTime(tool); !t.IsZero() {
			return t
		}
	}
	if !ep.StartedAt.IsZero() {
		return ep.StartedAt.Add(time.Duration(turn.Turn) * time.Millisecond)
	}
	return time.Now().UTC()
}

func nonZeroTime(t time.Time, fallback time.Time) time.Time {
	if t.IsZero() {
		return fallback
	}
	return t
}

func lastStatus(tool ToolCallTrace) string {
	if n := len(tool.StatusTransitions); n > 0 {
		return tool.StatusTransitions[n-1].Status
	}
	return ""
}

func boundedString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "unknown"
}

// TraceFileStats summarizes a run's OTLP JSON File artifact for evalreport.
func TraceFileStats(runDir string) (traceCount int, spanCount int, exists bool, err error) {
	path := filepath.Join(runDir, traceFileName)
	data, readErr := os.ReadFile(path)
	if os.IsNotExist(readErr) {
		return 0, 0, false, nil
	}
	if readErr != nil {
		return 0, 0, false, readErr
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	traceIDs := map[string]struct{}{}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var td traceDataAlias
		if err := json.Unmarshal([]byte(line), &td); err != nil {
			return 0, 0, true, fmt.Errorf("parse traces.jsonl line %d: %w", i+1, err)
		}
		for _, rs := range td.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, sp := range ss.Spans {
					spanCount++
					if sp.TraceID != "" {
						traceIDs[sp.TraceID] = struct{}{}
					}
				}
			}
		}
	}
	return len(traceIDs), spanCount, true, nil
}

type traceDataAlias struct {
	ResourceSpans []struct {
		ScopeSpans []struct {
			Spans []struct {
				TraceID string `json:"traceId"`
			} `json:"spans"`
		} `json:"scopeSpans"`
	} `json:"resourceSpans"`
}
