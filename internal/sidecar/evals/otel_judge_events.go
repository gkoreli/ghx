package evals

import (
	"context"
	"fmt"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/telemetry"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// EmitJudgeResult appends the judge invocation for one episode to the same
// run directory's traces.jsonl (ADR-0023.1 D6: judge spans live in the same
// substrate as the episodes they judge — one viewer, one replay recipe). It
// emits an "eval.judge" span carrying gen_ai.evaluation.result events, one per
// core-rubric dimension plus overall, identically shaped to reward events.
//
// Skipped or nil results emit nothing (there is no judgment to record). This
// function does NOT invoke a model — res is already-computed judge output.
func EmitJudgeResult(ctx context.Context, runDir string, ep *Episode, res *JudgeResult) error {
	if ep == nil || res == nil || res.Skipped {
		return nil
	}
	// The generic tracer-provider construction moved to the telemetry package
	// (ADR-0022 D1); this is the exact equivalent of the pre-refactor
	// episodeTracerProvider(ctx, runDir, ep) the judge originally called.
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
	start := nonZeroTime(res.ScoredAt, time.Now().UTC())
	_, span := tracer.Start(ctx, "eval.judge",
		trace.WithTimestamp(start),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(judgeSpanAttributes(ep, res)...),
	)
	addJudgeEvaluationEvents(span, res)
	span.End(trace.WithTimestamp(start))

	if err := tp.ForceFlush(ctx); err != nil {
		return fmt.Errorf("flush judge traces: %w", err)
	}
	return nil
}

func judgeSpanAttributes(ep *Episode, res *JudgeResult) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.GenAIOperationNameChat,
		attribute.String("gen_ai.system", "ghx-evals"),
		attribute.String(genAIRequestModelAttribute, res.JudgeModelID),
		attribute.String("gen_ai.agent.name", "ghx-eval-judge"),
		attribute.String("ghx.eval.episode_id", ep.ID),
		attribute.String("ghx.eval.task_id", ep.TaskID),
		attribute.String("ghx.judge.prompt_version", res.PromptVersion),
		attribute.String("ghx.judge.rubric_version", res.RubricVersion),
		attribute.Int("ghx.judge.samples", res.Samples),
		attribute.Float64("ghx.judge.overall", res.Overall),
		attribute.Int("ghx.judge.max_disagreement", res.MaxDisagreement),
		attribute.Bool("ghx.judge.calibrated", res.Calibrated),
	}
}
