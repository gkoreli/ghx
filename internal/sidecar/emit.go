package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// Production emission for sidecar.Ask (ADR-0022 D2). Every Ask turn appends to
// the session's artifact set — traces.jsonl, logs.jsonl, metrics.jsonl — using
// the shared telemetry runtime, the same capability the eval path uses. There
// are NO reward/evaluation events here: that is SAFE's layer. Emission failures
// never fail the ask (warn to stderr, degrade); the answer is the product, the
// trace is the audit.
const (
	sidecarTracerName     = "github.com/gkoreli/ghx/v2/internal/sidecar"
	sidecarSemconvVersion = "1.41.0"
	sidecarSystem         = "ghx-sidecar"

	genAIContentEvent          = "gen_ai.client.inference.operation.details"
	genAIInputMessagesAttr     = "gen_ai.input.messages"
	genAIOutputMessagesAttr    = "gen_ai.output.messages"
	genAIReasoningTokensAttr   = "gen_ai.usage.reasoning.output_tokens"
	genAIDurationMetric        = "gen_ai.client.operation.duration"
	genAITokenUsageMetric      = "gen_ai.client.token.usage"
	genAITokenTypeAttr         = "gen_ai.token.type"
	ghxSidecarReportSizeMetric = "ghx.sidecar.report.size"
)

// turnTelemetry is the neutral input for emitting one production Ask turn's
// artifacts. It carries session/turn identifiers plus the TurnResult, report,
// and timing — not any eval type — so the shared runtime stays domain-neutral.
type turnTelemetry struct {
	SessionsDir string
	Session     string
	Repo        string
	Model       string
	Turn        int
	Question    string
	Result      TurnResult
	Report      *Report
	// Error carries the turn's failure (liveness watchdog, dead peer,
	// unrecovered turn-cap). When set, the turn span is marked as an error and
	// an error log record is written next to it (ADR-0027 D2/D3) — failed
	// explorations are the ones that most need auditing.
	Error          string
	StartedAt      time.Time
	EndedAt        time.Time
	CaptureContent bool
}

// emitTurnArtifacts writes the session's traces/logs/metrics for one Ask turn.
// Any failure is warned to stderr and swallowed so the ask still returns.
// It runs on every Ask exit path, including failures (ADR-0027 D3).
func emitTurnArtifacts(ctx context.Context, t turnTelemetry) {
	// The flush must survive the very cancellation it documents: a
	// watchdog-cancelled or deadline-exceeded context would otherwise abort
	// emission exactly when the artifacts matter most (ADR-0027 D3).
	ctx = context.WithoutCancel(ctx)
	dir := sessionDir(t.SessionsDir, t.Session)
	if err := t.emitTraces(ctx, dir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to emit turn traces/logs: %v\n", err)
	}
	if err := t.emitMetrics(dir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to emit turn metrics: %v\n", err)
	}
}

func (t turnTelemetry) window() (start, end time.Time) {
	start = t.StartedAt
	if start.IsZero() {
		start = time.Now().UTC()
	}
	end = t.EndedAt
	if end.IsZero() || end.Before(start) {
		end = start
	}
	return start, end
}

func (t turnTelemetry) resourceAttrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.ServiceName(sidecarSystem),
		attribute.String("otel.semconv_version", sidecarSemconvVersion),
		attribute.String("ghx.sidecar.session", t.Session),
		attribute.String("ghx.sidecar.repo", t.Repo),
	}
}

func (t turnTelemetry) emitTraces(ctx context.Context, dir string) error {
	tp, err := telemetry.NewTracerProvider(ctx, dir, t.resourceAttrs())
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tp.Shutdown(shutdownCtx)
	}()

	tracer := tp.Tracer(sidecarTracerName, trace.WithInstrumentationVersion(sidecarSemconvVersion))
	start, end := t.window()

	askCtx, askSpan := tracer.Start(ctx, "sidecar.ask",
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(t.askAttributes()...),
	)
	logs := t.emitTurnSpan(askCtx, tracer, start, end)
	askSpan.End(trace.WithTimestamp(end))

	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("flush turn traces: %w", err)
	}
	return telemetry.WriteLogs(filepath.Join(dir, telemetry.LogFileName), t.resourceAttrs(), sidecarTracerName, sidecarSemconvVersion, logs)
}

func (t turnTelemetry) emitTurnSpan(parent context.Context, tracer trace.Tracer, start, end time.Time) []telemetry.LogRecord {
	turnCtx, span := tracer.Start(parent, "sidecar.turn",
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(t.turnAttributes()...),
	)
	for _, tool := range t.Result.ToolTraces {
		t.emitToolSpan(turnCtx, tracer, tool)
	}
	logs := t.contentLogs(span.SpanContext(), start)
	if t.Error != "" {
		span.SetStatus(codes.Error, boundedStr(t.Error, 256))
		logs = append(logs, t.errorLog(span.SpanContext(), end))
	}
	span.End(trace.WithTimestamp(end))
	return logs
}

// errorLog is the production error entry for a failed turn (ADR-0027 D2/D3):
// watchdog timeouts, dead peers, and turn-cap deaths become an auditable log
// record correlated with the turn span, not only a stderr line. Written
// regardless of the content-capture setting — it carries the failure, not
// message content.
func (t turnTelemetry) errorLog(span trace.SpanContext, at time.Time) telemetry.LogRecord {
	return telemetry.LogRecord{
		Time:      at,
		Span:      span,
		EventName: "sidecar.turn.error",
		Attributes: []attribute.KeyValue{
			attribute.String("gen_ai.system", sidecarSystem),
			attribute.String("ghx.sidecar.session", t.Session),
			attribute.String("ghx.sidecar.repo", t.Repo),
			attribute.Int("ghx.sidecar.turn", t.Turn),
			attribute.String("error.message", boundedStr(t.Error, 2048)),
		},
	}
}

func (t turnTelemetry) emitToolSpan(parent context.Context, tracer trace.Tracer, tool ToolCallTrace) {
	start := firstToolTime(tool)
	if start.IsZero() {
		start = time.Now().UTC()
	}
	end := lastToolTime(tool)
	if end.IsZero() || end.Before(start) {
		end = start
	}
	name := "tool.call"
	if tool.Kind != "" {
		name = "tool." + tool.Kind
	}
	_, span := tracer.Start(parent, name,
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(t.toolAttributes(tool)...),
	)
	span.End(trace.WithTimestamp(end))
}

func (t turnTelemetry) askAttributes() []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameInvokeAgent,
		attribute.String("gen_ai.system", sidecarSystem),
		attribute.String("gen_ai.agent.name", "ghx-sidecar"),
		attribute.String("ghx.sidecar.session", t.Session),
		attribute.String("ghx.sidecar.repo", t.Repo),
		attribute.Int("ghx.sidecar.turn", t.Turn),
	}
	if t.Model != "" {
		attrs = append(attrs, semconv.GenAIRequestModel(t.Model))
	}
	return attrs
}

func (t turnTelemetry) turnAttributes() []attribute.KeyValue {
	r := t.Result
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameChat,
		attribute.String("gen_ai.system", sidecarSystem),
		semconv.GenAIUsageInputTokens(len(t.Question)),
		semconv.GenAIUsageOutputTokens(len(r.FullText)),
		attribute.Int("ghx.sidecar.turn", t.Turn),
		attribute.Int("ghx.sidecar.question_chars", len(t.Question)),
		attribute.Int("ghx.sidecar.response_chars", len(r.FullText)),
		attribute.Int("ghx.sidecar.tool_output_chars", r.ToolOutputChars),
		attribute.Int("ghx.sidecar.tool_call_count", len(r.ToolTraces)),
		attribute.Bool("ghx.sidecar.report_retried", r.ReportRetried),
		attribute.Bool("ghx.sidecar.report_coerced", r.ReportCoerced),
		attribute.Bool("ghx.sidecar.wrap_up_recovered", r.WrapUpRecovered),
	}
	if t.Model != "" {
		attrs = append(attrs, semconv.GenAIRequestModel(t.Model))
	}
	if r.Thinking != "" {
		attrs = append(attrs, attribute.Int(genAIReasoningTokensAttr, len(r.Thinking)))
	}
	return attrs
}

func (t turnTelemetry) toolAttributes(tool ToolCallTrace) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		semconv.GenAIOperationNameExecuteTool,
		attribute.String("gen_ai.system", sidecarSystem),
		attribute.String("gen_ai.tool.call.id", tool.ID),
		attribute.String("gen_ai.tool.name", firstNonEmptyStr(tool.Kind, "tool")),
		attribute.Int("ghx.sidecar.turn", t.Turn),
		attribute.String("ghx.sidecar.tool.kind", tool.Kind),
		attribute.String("ghx.sidecar.tool.title", boundedStr(tool.Title, 512)),
		attribute.Int("ghx.sidecar.tool.output_chars", tool.OutputSize),
	}
	if status := lastToolStatus(tool); status != "" {
		attrs = append(attrs, attribute.String("ghx.sidecar.tool.status", status))
	}
	if tool.OutputExcerpt != "" {
		attrs = append(attrs, attribute.String("ghx.sidecar.tool.output_excerpt", boundedStr(tool.OutputExcerpt, 2048)))
	}
	return attrs
}

func (t turnTelemetry) emitMetrics(dir string) error {
	start, end := t.window()
	base := []attribute.KeyValue{
		attribute.String("gen_ai.system", sidecarSystem),
		attribute.String("ghx.sidecar.session", t.Session),
		attribute.String("ghx.sidecar.repo", t.Repo),
	}
	turnAttrs := telemetry.AppendAttrs(base,
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.Int("ghx.sidecar.turn", t.Turn),
	)

	var metrics []*metricspb.Metric
	if durMs := end.Sub(start).Milliseconds(); durMs > 0 {
		metrics = append(metrics, telemetry.HistogramMetric(
			genAIDurationMetric, "Duration of GenAI client operations.", "s",
			[]*metricspb.HistogramDataPoint{telemetry.HistogramPoint(start, end, float64(durMs)/1000, turnAttrs)},
		))
	}
	tokenPoints := []*metricspb.NumberDataPoint{
		telemetry.IntPoint(start, end, int64(len(t.Question)), telemetry.AppendAttrs(turnAttrs, attribute.String(genAITokenTypeAttr, "input"))),
		telemetry.IntPoint(start, end, int64(len(t.Result.FullText)), telemetry.AppendAttrs(turnAttrs, attribute.String(genAITokenTypeAttr, "output"))),
	}
	if t.Result.Thinking != "" {
		tokenPoints = append(tokenPoints,
			telemetry.IntPoint(start, end, int64(len(t.Result.Thinking)), telemetry.AppendAttrs(turnAttrs, attribute.String(genAITokenTypeAttr, "reasoning"))),
		)
	}
	metrics = append(metrics, telemetry.SumMetric(
		genAITokenUsageMetric,
		"Number of input, output, and reasoning tokens used by GenAI operations.",
		"{token}", tokenPoints, true,
	))
	if t.Report != nil {
		if data, err := json.Marshal(t.Report); err == nil {
			metrics = append(metrics, telemetry.HistogramMetric(
				ghxSidecarReportSizeMetric, "Serialized ghx sidecar report size distribution.", "By",
				[]*metricspb.HistogramDataPoint{telemetry.HistogramPoint(start, end, float64(len(data)), telemetry.AppendAttrs(base, attribute.Int("ghx.sidecar.turn", t.Turn)))},
			))
		}
	}
	return telemetry.WriteMetrics(filepath.Join(dir, telemetry.MetricFileName), t.resourceAttrs(), sidecarTracerName, sidecarSemconvVersion, metrics)
}

// contentLogs builds the GenAI content record for the turn, gated by D4's
// capture-content resolution (default on in production, off via env/config).
func (t turnTelemetry) contentLogs(span trace.SpanContext, at time.Time) []telemetry.LogRecord {
	if !t.CaptureContent {
		return nil
	}
	input, inputOK := marshalGenAIMessages(t.inputMessages())
	output, outputOK := marshalGenAIMessages(t.outputMessages())
	if !inputOK && !outputOK {
		return nil
	}
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", sidecarSystem),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("ghx.sidecar.session", t.Session),
		attribute.String("ghx.sidecar.repo", t.Repo),
		attribute.Int("ghx.sidecar.turn", t.Turn),
	}
	if inputOK {
		attrs = append(attrs, attribute.String(genAIInputMessagesAttr, input))
	}
	if outputOK {
		attrs = append(attrs, attribute.String(genAIOutputMessagesAttr, output))
	}
	return []telemetry.LogRecord{{
		Time:       at,
		Span:       span,
		EventName:  genAIContentEvent,
		Attributes: attrs,
	}}
}

type genAIContentMessage struct {
	Role  string             `json:"role"`
	Parts []genAIContentPart `json:"parts"`
}

type genAIContentPart struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

func (t turnTelemetry) inputMessages() []genAIContentMessage {
	msgs := []genAIContentMessage{{
		Role:  "user",
		Parts: []genAIContentPart{{Type: "text", Content: t.Question}},
	}}
	for _, tool := range t.Result.ToolTraces {
		if strings.TrimSpace(tool.OutputExcerpt) == "" {
			continue
		}
		msgs = append(msgs, genAIContentMessage{
			Role:  "tool",
			Parts: []genAIContentPart{{Type: "text", Content: tool.OutputExcerpt}},
		})
	}
	return msgs
}

func (t turnTelemetry) outputMessages() []genAIContentMessage {
	var parts []genAIContentPart
	if strings.TrimSpace(t.Result.Thinking) != "" {
		parts = append(parts, genAIContentPart{Type: "reasoning", Content: t.Result.Thinking})
	}
	if strings.TrimSpace(t.Result.FullText) != "" {
		parts = append(parts, genAIContentPart{Type: "text", Content: t.Result.FullText})
	}
	if len(parts) == 0 {
		return nil
	}
	return []genAIContentMessage{{Role: "assistant", Parts: parts}}
}

func marshalGenAIMessages(messages []genAIContentMessage) (string, bool) {
	if len(messages) == 0 {
		return "", false
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func firstToolTime(tool ToolCallTrace) time.Time {
	if len(tool.StatusTransitions) > 0 {
		return tool.StatusTransitions[0].At
	}
	return time.Time{}
}

func lastToolTime(tool ToolCallTrace) time.Time {
	if n := len(tool.StatusTransitions); n > 0 {
		return tool.StatusTransitions[n-1].At
	}
	return time.Time{}
}

func lastToolStatus(tool ToolCallTrace) string {
	if n := len(tool.StatusTransitions); n > 0 {
		return tool.StatusTransitions[n-1].Status
	}
	return ""
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "unknown"
}

func boundedStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
