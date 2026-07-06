//go:build agent_e2e

package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TestClosedBookProbe runs the ADR-0016.9 closed-book memorization audit:
// every task in the eval corpus, answered by the subject model with an
// explicit empty tool surface, scored read-only by the existing deterministic
// checks. Artifacts land under GHX_EVAL_CLOSEDBOOK_DIR (manifest.json,
// closed-book-summary.json, episodes/).
//
// Manual invocation:
//
//	GHX_EVAL_AGENT="$(pwd)/scripts/eval-agent-acp.sh" \
//	GHX_EVAL_SUBJECT_MODEL=claude-sonnet-5 \
//	GHX_EVAL_CLOSEDBOOK_DIR="$(pwd)/docs/evals/memorization-audit" \
//	GHX_EVAL_OPENBOOK_RUN_DIR="$(pwd)/docs/evals/gate-run-2026-07-06-fixbatch" \
//	go test ./internal/sidecar/evals -tags=agent_e2e -run TestClosedBookProbe -v -timeout 90m
//
// Environment:
//
//	GHX_EVAL_CLOSEDBOOK_TRIALS  — trials per task (default 5, ADR-0016.9)
//	GHX_EVAL_OPENBOOK_RUN_DIR   — committed open-book reference run for the
//	                              exploration-signal comparison (required for
//	                              gap labels; omit for closed-book-only stats)
func TestClosedBookProbe(t *testing.T) {
	agentCmd := os.Getenv("GHX_EVAL_AGENT")
	if agentCmd == "" {
		agentCmd = sidecar.LoadConfig().AgentCmd
	}

	pf := sidecar.RunPreflightForAgent(context.Background(), agentCmd)
	if !pf.Passed {
		t.Skipf("preflight failed — environment not ready for live closed-book probe:\n%s", sidecar.FormatPreflight(pf))
	}

	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}

	trials := 5 // ADR-0016.9: n = 5, matching the gate-run trial count
	if v := strings.TrimSpace(os.Getenv("GHX_EVAL_CLOSEDBOOK_TRIALS")); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			trials = n
		}
	}

	outDir := os.Getenv("GHX_EVAL_CLOSEDBOOK_DIR")
	if outDir == "" {
		outDir = filepath.Join(".ghx-evals", "closed-book", time.Now().UTC().Format("20060102-150405"))
	}
	episodesDir := filepath.Join(outDir, "episodes")
	runID := "memorization-audit-" + time.Now().UTC().Format("2006-01-02")

	cfg := RunConfig{AgentCmd: agentCmd, SessionsDir: t.TempDir()}
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	identity, err := ProbeAgentIdentity(probeCtx, cfg)
	probeCancel()
	if err != nil {
		t.Fatalf("probe agent identity: %v", err)
	}
	t.Logf("subject identity: %+v", identity)

	corpusHash, err := taskCorpusHash("testdata/tasks")
	if err != nil {
		t.Fatalf("task corpus hash: %v", err)
	}
	metaJSON, err := json.Marshal(ClosedBookSessionMeta(resolveSubjectModel(agentCmd)))
	if err != nil {
		t.Fatalf("marshal session meta: %v", err)
	}
	// Fail fast if the invariant this whole probe depends on ever regresses.
	if !strings.Contains(string(metaJSON), `"tools":[]`) || !strings.Contains(string(metaJSON), `"allowedTools":[]`) {
		t.Fatalf("closed-book session meta lost the explicit empty tool arrays: %s", metaJSON)
	}

	manifest := ClosedBookManifest{
		ADR:               "ADR-0016.9",
		RunID:             runID,
		GeneratedAt:       time.Now().UTC(),
		GitCommit:         gitHeadCommit(),
		TaskCorpusSHA256:  corpusHash,
		Identity:          identity,
		TrialsPerTask:     trials,
		SessionMetaJSON:   string(metaJSON),
		ScoringCodeCommit: gitHeadCommit(),
	}

	var episodes []*Episode
	for _, task := range tasks {
		for trial := 1; trial <= trials; trial++ {
			label := fmt.Sprintf("%s trial %d/%d", task.ID, trial, trials)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			ep, runErr := RunClosedBookEpisode(ctx, cfg, task, trial)
			cancel()
			if ep != nil {
				if path, saveErr := SaveEpisode(episodesDir, ep); saveErr != nil {
					t.Errorf("%s: save episode: %v", label, saveErr)
				} else {
					t.Logf("%s: artifact %s correctness=%.3f", label, path, ep.Rewards.Correctness)
				}
				episodes = append(episodes, ep)
			}
			if runErr != nil {
				// Tool activity or transport failure: a harness failure per
				// ADR-0016.9 — the trial is invalid and the run must say so.
				t.Errorf("%s: closed-book trial failed: %v", label, runErr)
			}
		}
	}

	var openEps []*Episode
	openRunID := ""
	if openDir := os.Getenv("GHX_EVAL_OPENBOOK_RUN_DIR"); openDir != "" {
		openEps, err = LoadRunEpisodes(openDir)
		if err != nil {
			t.Fatalf("load open-book reference run: %v", err)
		}
		openRunID = filepath.Base(openDir)
		manifest.OpenBookRunID = openRunID
		manifest.OpenBookRunDir = openDir
	} else {
		t.Log("GHX_EVAL_OPENBOOK_RUN_DIR unset — summary will omit exploration-signal comparison")
	}

	if path, err := SaveClosedBookJSON(outDir, "manifest.json", manifest); err != nil {
		t.Fatalf("save manifest: %v", err)
	} else {
		t.Logf("manifest: %s", path)
	}

	summary := ComputeClosedBookSummary(runID, trials, episodes, openEps, openRunID, tasks)
	if path, err := SaveClosedBookJSON(outDir, "closed-book-summary.json", summary); err != nil {
		t.Fatalf("save summary: %v", err)
	} else {
		t.Logf("summary: %s", path)
	}

	for _, ts := range summary.Tasks {
		open := "n/a"
		signal := "n/a"
		if ts.OpenBook != nil {
			open = fmt.Sprintf("%.3f", ts.OpenBook.PooledMeanCorrectness)
			signal = fmt.Sprintf("%.3f", ts.ExplorationSignal)
		}
		t.Logf("CLOSED-BOOK %-24s trials=%d mean=%.3f median=%.3f max=%.3f sd=%.3f open=%s signal=%s labels=%v",
			ts.TaskID, ts.Trials, ts.MeanCorrectness, ts.MedianCorrectness, ts.MaxCorrectness, ts.StdDevCorrectness, open, signal, ts.Labels)
	}
}

// gitHeadCommit best-effort resolves HEAD for the manifest; "unknown" when
// git is unavailable.
func gitHeadCommit() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
