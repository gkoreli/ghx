package evals

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestJudgeLiveSmoke scores ONE committed episode bundle end-to-end against
// the real primary judge (OpenAI-compatible, committed config). It is
// env-gated: it runs only when GHX_JUDGE_LIVE_TEST=1 is set explicitly, and it
// then requires OPENAI_API_KEY (failing loudly rather than skipping — an
// explicit opt-in with missing credentials is an operator error). It uses k=1
// (not the committed k=3) to keep the smoke cheap; it validates plumbing, not
// citable scores — Calibrated stays false regardless (ADR-0023.1 D5).
func TestJudgeLiveSmoke(t *testing.T) {
	if os.Getenv("GHX_JUDGE_LIVE_TEST") != "1" {
		t.Skip("live judge smoke disabled; set GHX_JUDGE_LIVE_TEST=1 to run")
	}
	if os.Getenv(envOpenAIAPIKey) == "" {
		t.Fatalf("GHX_JUDGE_LIVE_TEST=1 but %s is not set", envOpenAIAPIKey)
	}

	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatalf("load tasks: %v", err)
	}
	ep := loadJudgeFixture(t, "sidecar")
	var task Task
	found := false
	for _, tk := range tasks {
		if tk.ID == ep.TaskID {
			task, found = tk, true
			break
		}
	}
	if !found {
		t.Fatalf("no committed task %q for fixture episode %s", ep.TaskID, ep.ID)
	}

	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatalf("judge config: %v", err)
	}
	client, err := NewOpenAIJudgeClient(cfg)
	if err != nil {
		t.Fatalf("build primary judge client: %v", err)
	}

	runner := &JudgeRunner{Client: client, Samples: 1} // smoke: one sample, not the committed k
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	res, err := runner.Score(ctx, task, ep)
	if err != nil {
		t.Fatalf("live judge score: %v", err)
	}
	if res.Skipped {
		t.Fatalf("fixture episode was pre-filtered: %s", res.SkipReason)
	}
	if got, want := len(res.Dimensions), len(coreRubric.Dimensions); got != want {
		t.Errorf("dimensions = %d, want %d", got, want)
	}
	if res.Overall < float64(coreRubric.ScaleMin) || res.Overall > float64(coreRubric.ScaleMax) {
		t.Errorf("overall = %v, out of scale", res.Overall)
	}
	if res.Calibrated {
		t.Error("Calibrated must stay false until D5 lands")
	}
	if res.JudgeModelID != cfg.Primary.Model {
		t.Errorf("judge model = %q, want %q", res.JudgeModelID, cfg.Primary.Model)
	}

	// Round-trip the artifact like a real run would.
	path, err := SaveJudgeResult(t.TempDir(), res)
	if err != nil {
		t.Fatalf("save judge result: %v", err)
	}
	if _, err := LoadJudgeResult(path); err != nil {
		t.Fatalf("reload judge result: %v", err)
	}
	t.Logf("live smoke: model=%s episode=%s overall=%.1f maxDisagreement=%d (PRELIMINARY, uncalibrated)",
		res.JudgeModelID, res.EpisodeID, res.Overall, res.MaxDisagreement)
}
