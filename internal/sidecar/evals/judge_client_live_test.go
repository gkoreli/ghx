package evals

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestJudgeLiveSmoke scores ONE committed episode bundle end-to-end through the
// real CLI judge rails. It is env-gated because it spends subscription-model
// calls. It uses k=1 (not committed k=3) to validate plumbing, not citable
// scores; Calibrated stays false regardless (ADR-0023.1 D5).
func TestJudgeLiveSmoke(t *testing.T) {
	if os.Getenv("GHX_JUDGE_LIVE_TEST") != "1" {
		t.Skip("live judge smoke disabled; set GHX_JUDGE_LIVE_TEST=1 to run")
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
	primary, err := NewPrimaryCLIJudgeClient(cfg)
	if err != nil {
		t.Fatalf("build primary CLI judge client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	outDir := os.Getenv("GHX_JUDGE_LIVE_ARTIFACT_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	}
	primaryPath := scoreLiveJudge(t, ctx, outDir, "primary", primary, task, ep)
	t.Logf("live smoke primary artifact: %s", primaryPath)

	secondary, err := NewSecondaryCLIJudgeClient(cfg)
	if err != nil {
		t.Logf("secondary CLI judge unavailable: %v", err)
		return
	}
	secondaryPath, err := tryScoreLiveJudge(t, ctx, outDir, "secondary", secondary, task, ep)
	if err != nil {
		t.Logf("secondary CLI judge did not complete: %v", err)
		return
	}
	t.Logf("live smoke secondary artifact: %s", secondaryPath)
}

func scoreLiveJudge(t *testing.T, ctx context.Context, outDir, label string, client JudgeClient, task Task, ep *Episode) string {
	t.Helper()
	path, err := tryScoreLiveJudge(t, ctx, outDir, label, client, task, ep)
	if err != nil {
		t.Fatalf("live %s judge score: %v", label, err)
	}
	return path
}

func tryScoreLiveJudge(t *testing.T, ctx context.Context, outDir, label string, client JudgeClient, task Task, ep *Episode) (string, error) {
	t.Helper()
	res, err := scoreLiveBundle(ctx, client, task, ep)
	if err != nil {
		return "", err
	}
	if got, want := len(res.Dimensions), len(coreRubric.Dimensions); got != want {
		return "", fmt.Errorf("dimensions = %d, want %d", got, want)
	}
	if res.Overall < float64(coreRubric.ScaleMin) || res.Overall > float64(coreRubric.ScaleMax) {
		return "", fmt.Errorf("overall = %v, out of scale", res.Overall)
	}
	if res.Calibrated {
		return "", fmt.Errorf("Calibrated must stay false until D5 lands")
	}
	if res.JudgeRuntimeVersion == "" {
		return "", fmt.Errorf("JudgeRuntimeVersion is blank")
	}
	path, err := SaveJudgeResult(filepath.Join(outDir, label), res)
	if err != nil {
		return "", err
	}
	if _, err := LoadJudgeResult(path); err != nil {
		return "", err
	}
	t.Logf("live smoke %s: model=%s runtime=%q episode=%s overall=%.1f maxDisagreement=%d (PRELIMINARY, uncalibrated)",
		label, res.JudgeModelID, res.JudgeRuntimeVersion, res.EpisodeID, res.Overall, res.MaxDisagreement)
	return path, nil
}

func scoreLiveBundle(ctx context.Context, client JudgeClient, task Task, ep *Episode) (*JudgeResult, error) {
	bundle := BuildJudgeBundle(ep)
	prompt, err := RenderJudgePrompt(task, bundle)
	if err != nil {
		return nil, fmt.Errorf("render judge prompt: %w", err)
	}
	verdict, err := client.Evaluate(ctx, prompt)
	if err != nil {
		return nil, err
	}
	res := &JudgeResult{
		SchemaVersion: judgeResultSchemaVersion,
		EpisodeID:     ep.ID,
		TaskID:        ep.TaskID,
		JudgeModelID:  client.ModelID(),
		PromptVersion: judgePromptVersion,
		RubricVersion: rubricVersion,
		Samples:       1,
		Verdicts:      []JudgeVerdict{verdict},
		Bundle:        &bundle,
		Calibrated:    false,
		ScoredAt:      time.Now().UTC(),
	}
	if versioned, ok := client.(judgeRuntimeVersionClient); ok {
		res.JudgeRuntimeVersion = versioned.RuntimeVersion()
	}
	res.Dimensions, res.Overall, res.MaxDisagreement = aggregateVerdicts(res.Verdicts)
	return res, nil
}
