package evals

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func attrString(attrs []attribute.KeyValue, key string) (string, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsString(), true
		}
	}
	return "", false
}

func attrFloat(attrs []attribute.KeyValue, key string) (float64, bool) {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsFloat64(), true
		}
	}
	return 0, false
}

func TestJudgeEvaluationEventsShape(t *testing.T) {
	res := &JudgeResult{
		EpisodeID:       "ep1",
		JudgeModelID:    "gpt-5.5-test",
		PromptVersion:   judgePromptVersion,
		RubricVersion:   rubricVersion,
		Samples:         3,
		Overall:         4.0,
		MaxDisagreement: 2,
		Dimensions: []AggregatedDim{
			{Dimension: "evidence_groundedness", MedianScore: 4, Disagreement: 1, Explanation: "well anchored"},
			{Dimension: "exploration_efficiency", MedianScore: 5, Disagreement: 0, Explanation: "tight"},
			{Dimension: "uncertainty_honesty", MedianScore: 3, Disagreement: 2, Explanation: "some overclaim"},
		},
		Verdicts: []JudgeVerdict{{Explanation: "overall solid"}},
	}

	events := judgeEvaluationEvents(res)
	if len(events) != 4 { // 3 dimensions + overall
		t.Fatalf("expected 4 events, got %d", len(events))
	}

	// Every event reuses the shared reward event name (constraint 3).
	byName := map[string][]attribute.KeyValue{}
	for _, e := range events {
		if e.Name != genAIEvaluationResultEvent {
			t.Errorf("event name %q, want %q", e.Name, genAIEvaluationResultEvent)
		}
		name, ok := attrString(e.Attrs, genAIEvaluationNameAttribute)
		if !ok {
			t.Fatalf("event missing %s", genAIEvaluationNameAttribute)
		}
		byName[name] = e.Attrs
		// Shared metadata on every event.
		if model, _ := attrString(e.Attrs, genAIRequestModelAttribute); model != "gpt-5.5-test" {
			t.Errorf("event %s judge model=%q", name, model)
		}
		if pv, _ := attrString(e.Attrs, "ghx.judge.prompt_version"); pv != judgePromptVersion {
			t.Errorf("event %s prompt version=%q", name, pv)
		}
	}

	check := func(name string, wantScore float64) {
		attrs, ok := byName[name]
		if !ok {
			t.Fatalf("missing event %s", name)
		}
		if v, _ := attrFloat(attrs, genAIEvaluationScoreValueAttribute); v != wantScore {
			t.Errorf("%s score=%.1f want %.1f", name, v, wantScore)
		}
		if expl, ok := attrString(attrs, genAIEvaluationExplanationAttribute); !ok || expl == "" {
			t.Errorf("%s missing explanation", name)
		}
	}
	check("judge.evidence_groundedness", 4)
	check("judge.exploration_efficiency", 5)
	check("judge.uncertainty_honesty", 3)
	check("judge.overall", 4.0)
}

func TestEmitJudgeResultWritesTraces(t *testing.T) {
	ep := syntheticSidecarEpisode("emit-ep", "Routes register via RouterGroup.")
	client := FixedJudgeClient{
		Model:   "gpt-5.5-test",
		Verdict: NewVerdict(map[string]int{"evidence_groundedness": 4, "exploration_efficiency": 5, "uncertainty_honesty": 3}, "solid"),
	}
	runner := NewJudgeRunner(client)
	runner.now = func() time.Time { return time.Unix(1783294237, 0).UTC() }
	res, err := runner.Score(context.Background(), baseTask(), ep)
	if err != nil {
		t.Fatalf("score: %v", err)
	}

	dir := t.TempDir()
	if err := EmitJudgeResult(context.Background(), dir, ep, res); err != nil {
		t.Fatalf("emit: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "traces.jsonl"))
	if err != nil {
		t.Fatalf("read traces: %v", err)
	}
	blob := string(data)
	for _, want := range []string{"eval.judge", genAIEvaluationResultEvent, "judge.evidence_groundedness", "judge.overall", "gpt-5.5-test"} {
		if !strings.Contains(blob, want) {
			t.Errorf("traces.jsonl missing %q", want)
		}
	}
}

func TestEmitJudgeResultSkippedWritesNothing(t *testing.T) {
	ep := syntheticSidecarEpisode("skip-ep", "BLOCKED: nope")
	res := &JudgeResult{EpisodeID: "skip-ep", Skipped: true, SkipReason: "BLOCKED"}
	dir := t.TempDir()
	if err := EmitJudgeResult(context.Background(), dir, ep, res); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "traces.jsonl")); !os.IsNotExist(err) {
		t.Errorf("skipped judge result should write no traces, stat err=%v", err)
	}
}

func TestJudgeEvaluationEventsSkippedEmitsNothing(t *testing.T) {
	res := &JudgeResult{EpisodeID: "ep", Skipped: true, SkipReason: "BLOCKED"}
	if events := judgeEvaluationEvents(res); events != nil {
		t.Errorf("skipped result should emit no events, got %d", len(events))
	}
	if events := judgeEvaluationEvents(nil); events != nil {
		t.Errorf("nil result should emit no events")
	}
}
