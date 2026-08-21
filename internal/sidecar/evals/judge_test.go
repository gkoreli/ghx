package evals

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func baseTask() Task {
	return Task{
		ID:     "t1",
		Repo:   "owner/repo",
		Turns:  []string{"where is X wired up?"},
		Checks: TaskChecks{ExpectedFiles: []string{"lib/foo.go"}},
	}
}

func TestJudgeRubricValidation(t *testing.T) {
	cases := []struct {
		name     string
		criteria []string
		nilBlock bool
		wantErr  bool
	}{
		{name: "nil block ok", nilBlock: true},
		{name: "two ok", criteria: []string{"a", "b"}},
		{name: "five ok", criteria: []string{"a", "b", "c", "d", "e"}},
		{name: "one too few", criteria: []string{"a"}, wantErr: true},
		{name: "six too many", criteria: []string{"a", "b", "c", "d", "e", "f"}, wantErr: true},
		{name: "blank entry", criteria: []string{"a", "   "}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task := baseTask()
			if !tc.nilBlock {
				task.Judge = &JudgeRubric{Criteria: tc.criteria}
			}
			err := task.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestLoadTasksParsesJudgeBlock(t *testing.T) {
	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	if len(tasks) != 10 {
		t.Fatalf("expected 10 tasks, got %d", len(tasks))
	}
	for _, task := range tasks {
		if task.Judge == nil {
			t.Errorf("task %s has no judge block", task.ID)
			continue
		}
		if n := len(task.Judge.Criteria); n < judgeMinCriteria || n > judgeMaxCriteria {
			t.Errorf("task %s has %d criteria, out of bounds", task.ID, n)
		}
	}
}

func TestAggregateVerdictsMedianAndDisagreement(t *testing.T) {
	verdicts := []JudgeVerdict{
		NewVerdict(map[string]int{"evidence_groundedness": 4, "exploration_efficiency": 5, "uncertainty_honesty": 2}, "v1"),
		NewVerdict(map[string]int{"evidence_groundedness": 5, "exploration_efficiency": 5, "uncertainty_honesty": 3}, "v2"),
		NewVerdict(map[string]int{"evidence_groundedness": 3, "exploration_efficiency": 5, "uncertainty_honesty": 4}, "v3"),
	}
	dims, overall, maxDis := aggregateVerdicts(verdicts)

	want := map[string]struct {
		median float64
		spread int
	}{
		"evidence_groundedness":  {4, 2},
		"exploration_efficiency": {5, 0},
		"uncertainty_honesty":    {3, 2},
	}
	if len(dims) != 3 {
		t.Fatalf("expected 3 dimensions, got %d", len(dims))
	}
	// Order must follow the core-rubric canonical order.
	if !reflect.DeepEqual([]string{dims[0].Dimension, dims[1].Dimension, dims[2].Dimension},
		coreRubric.dimensionNames()) {
		t.Errorf("dimension order not canonical: %v", dims)
	}
	for _, d := range dims {
		w := want[d.Dimension]
		if d.MedianScore != w.median {
			t.Errorf("%s median=%.1f want %.1f", d.Dimension, d.MedianScore, w.median)
		}
		if d.Disagreement != w.spread {
			t.Errorf("%s disagreement=%d want %d", d.Dimension, d.Disagreement, w.spread)
		}
	}
	if maxDis != 2 {
		t.Errorf("maxDisagreement=%d want 2", maxDis)
	}
	if overall != 4.0 { // median of [3.667, 4.0, 4.333]
		t.Errorf("overall=%.4f want 4.0", overall)
	}
}

func TestJudgeRunnerScoresHealthyEpisode(t *testing.T) {
	ep := syntheticSidecarEpisode("healthy-sidecar", "Routes are registered in gin.go via RouterGroup; matching walks tree.go.")
	client := &ScriptedJudgeClient{
		Model: "gpt-5.5-test",
		Verdicts: []JudgeVerdict{
			NewVerdict(map[string]int{"evidence_groundedness": 4, "exploration_efficiency": 4, "uncertainty_honesty": 4}, "a"),
			NewVerdict(map[string]int{"evidence_groundedness": 5, "exploration_efficiency": 4, "uncertainty_honesty": 3}, "b"),
			NewVerdict(map[string]int{"evidence_groundedness": 4, "exploration_efficiency": 5, "uncertainty_honesty": 4}, "c"),
		},
	}
	runner := NewJudgeRunner(client)
	runner.now = func() time.Time { return time.Unix(1783294237, 0).UTC() }

	res, err := runner.Score(context.Background(), baseTask(), ep)
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if res.Skipped {
		t.Fatalf("healthy episode should not be skipped: %s", res.SkipReason)
	}
	if res.Samples != 3 {
		t.Errorf("samples=%d want 3", res.Samples)
	}
	if len(res.Dimensions) != 3 {
		t.Errorf("dimensions=%d want 3", len(res.Dimensions))
	}
	if res.JudgeModelID != "gpt-5.5-test" {
		t.Errorf("judge model id=%q", res.JudgeModelID)
	}
	if res.PromptVersion != judgePromptVersion || res.RubricVersion != rubricVersion {
		t.Errorf("version stamps missing: prompt=%q rubric=%q", res.PromptVersion, res.RubricVersion)
	}
	if res.Bundle == nil {
		t.Errorf("result should embed the scored bundle (D2)")
	}
	if res.Calibrated {
		t.Errorf("no score is calibrated before D5")
	}
}

func syntheticSidecarEpisode(id, answer string) *Episode {
	return &Episode{
		ID:      id,
		TaskID:  "t1",
		Repo:    "owner/repo",
		Profile: ProfileSidecar,
		Turns: []TurnRecord{{
			Turn:     0,
			Question: "q",
			Report:   &sidecar.Report{Answer: answer},
		}},
		Report: &sidecar.Report{Answer: answer},
	}
}

func TestJudgePrefilterSkipsBlockedAndNoReport(t *testing.T) {
	blocked := syntheticSidecarEpisode("blocked", "BLOCKED: ghx not available")
	if skip, reason := judgePrefilter(blocked); !skip || reason == "" {
		t.Errorf("BLOCKED episode should be skipped, got skip=%v reason=%q", skip, reason)
	}

	noReport := syntheticSidecarEpisode("noreport", "WARN: sidecar did not emit a report")
	if skip, reason := judgePrefilter(noReport); !skip || reason == "" {
		t.Errorf("WARN-noreport episode should be skipped, got skip=%v reason=%q", skip, reason)
	}

	invalid := &Episode{ID: "inv", Invalid: true, ExclusionReasons: []string{"COMPLIANCE: plain profile invoked ghx"}}
	if skip, _ := judgePrefilter(invalid); !skip {
		t.Errorf("invalid episode should be skipped")
	}

	healthy := syntheticSidecarEpisode("ok", "Routes are registered in gin.go via RouterGroup")
	if skip, _ := judgePrefilter(healthy); skip {
		t.Errorf("healthy episode should not be skipped")
	}
}

func TestJudgeRunnerSkipRecordsReasonAndSkipsClient(t *testing.T) {
	blocked := syntheticSidecarEpisode("blocked", "BLOCKED: nope")
	// A client that fails if called proves the runner never invokes the model.
	client := &ScriptedJudgeClient{Model: "gpt-5.5-test"} // empty verdicts → error if called
	runner := NewJudgeRunner(client)
	runner.now = func() time.Time { return time.Unix(0, 0).UTC() }

	res, err := runner.Score(context.Background(), baseTask(), blocked)
	if err != nil {
		t.Fatalf("skip path must not error: %v", err)
	}
	if !res.Skipped || res.SkipReason == "" {
		t.Errorf("expected skipped result with reason, got %+v", res)
	}
	if len(res.Verdicts) != 0 {
		t.Errorf("skipped result should carry no verdicts")
	}
}

// TestJudgeRunnerSkipsRealWarnFixture proves the pre-filter fires on a real
// committed episode: the gin-routing sidecar fixture warned on turn 0 (no
// extractable report) even though turn 1 succeeded, so D7 excludes it.
func TestJudgeRunnerSkipsRealWarnFixture(t *testing.T) {
	ep := loadJudgeFixture(t, "sidecar")
	client := &ScriptedJudgeClient{Model: "gpt-5.5-test"} // errors if called
	runner := NewJudgeRunner(client)
	runner.now = func() time.Time { return time.Unix(0, 0).UTC() }
	res, err := runner.Score(context.Background(), baseTask(), ep)
	if err != nil {
		t.Fatalf("skip path must not error: %v", err)
	}
	if !res.Skipped {
		t.Fatalf("real WARN-noreport fixture should be skipped by the pre-filter")
	}
}

func TestSaveLoadJudgeResultRoundTrip(t *testing.T) {
	ep := syntheticSidecarEpisode("roundtrip-sidecar", "Routes register via RouterGroup in routergroup.go.")
	client := FixedJudgeClient{
		Model:   "gpt-5.5-test",
		Verdict: NewVerdict(map[string]int{"evidence_groundedness": 4, "exploration_efficiency": 4, "uncertainty_honesty": 4}, "fixed"),
	}
	runner := NewJudgeRunner(client)
	runner.now = func() time.Time { return time.Unix(1783294237, 0).UTC() }
	res, err := runner.Score(context.Background(), baseTask(), ep)
	if err != nil {
		t.Fatalf("score: %v", err)
	}

	dir := t.TempDir()
	path, err := SaveJudgeResult(dir, res)
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if filepath.Base(path) != res.EpisodeID+".judge.json" {
		t.Errorf("unexpected artifact name %q", filepath.Base(path))
	}
	got, err := LoadJudgeResult(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.EpisodeID != res.EpisodeID || got.Overall != res.Overall || got.Samples != res.Samples {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, res)
	}
}
