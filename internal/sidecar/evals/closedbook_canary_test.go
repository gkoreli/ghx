package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TestClosedBookCanaryR1R4 (ADR-0016.13 canary; ADR-0040 P3 slice) runs the
// ADR-0016.9 closed-book probe over ONLY the four replacement fixtures
// (werkzeug-delegation, gin-route-conflict, hono-smartrouter-fallback,
// gjson-engine-selection), at reduced trial count, writing artifacts under
// GHX_EVAL_CLOSEDBOOK_DIR. Acceptance per ADR-0016.9/0016.13: each task's
// closed-book mean must be < 0.40 before the fixture is citable.
//
// Manual invocation:
//
//	GHX_EVAL_AGENT="$(pwd)/scripts/eval-agent-acp.sh" \
//	GHX_EVAL_SUBJECT_MODEL=claude-sonnet-5 \
//	GHX_EVAL_CLOSEDBOOK_DIR="$(pwd)/docs/evals/closedbook-canary-r1r4" \
//	GHX_EVAL_CLOSEDBOOK_TRIALS=3 \
//	go test ./internal/sidecar/evals -tags=agent_e2e -run TestClosedBookCanaryR1R4 -v -timeout 60m
func TestClosedBookCanaryR1R4(t *testing.T) {
	canaryTasks := map[string]bool{
		"werkzeug-delegation":       true,
		"gin-route-conflict":        true,
		"hono-smartrouter-fallback": true,
		"gjson-engine-selection":    true,
	}

	agentCmd := os.Getenv("GHX_EVAL_AGENT")
	if agentCmd == "" {
		agentCmd = sidecar.LoadConfig().AgentCmd
	}
	if agentCmd == "" {
		t.Skip("no ACP agent configured (GHX_EVAL_AGENT / sidecar config) — canary needs a live backend")
	}

	pf := sidecar.RunPreflightForAgent(context.Background(), agentCmd)
	if !pf.Passed {
		t.Skipf("preflight failed — environment not ready for live closed-book canary:\n%s", sidecar.FormatPreflight(pf))
	}

	all, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}
	var tasks []Task
	for _, task := range all {
		if canaryTasks[task.ID] {
			tasks = append(tasks, task)
		}
	}
	if len(tasks) != len(canaryTasks) {
		t.Fatalf("canary task set incomplete: found %d of %d replacement fixtures — %v",
			len(tasks), len(canaryTasks), taskIDs(tasks))
	}

	trials := 3
	if v := strings.TrimSpace(os.Getenv("GHX_EVAL_CLOSEDBOOK_TRIALS")); v != "" {
		if n, convErr := atoi(v); convErr == nil && n > 0 {
			trials = n
		}
	}

	outDir := os.Getenv("GHX_EVAL_CLOSEDBOOK_DIR")
	if outDir == "" {
		outDir = filepath.Join(".ghx-evals", "closedbook-canary-r1r4", time.Now().UTC().Format("20060102-150405"))
	}
	episodesDir := filepath.Join(outDir, "episodes")
	runID := "closedbook-canary-r1r4-" + time.Now().UTC().Format("2006-01-02")

	cfg := RunConfig{AgentCmd: agentCmd, SessionsDir: t.TempDir()}
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	identity, err := ProbeAgentIdentity(probeCtx, cfg)
	probeCancel()
	if err != nil {
		t.Fatalf("probe agent identity: %v", err)
	}

	metaJSON, err := json.Marshal(ClosedBookSessionMeta(resolveSubjectModel(agentCmd)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(metaJSON), `"tools":[]`) || !strings.Contains(string(metaJSON), `"allowedTools":[]`) {
		t.Fatalf("closed-book session meta lost the explicit empty tool arrays: %s", metaJSON)
	}

	corpusHash, err := taskCorpusHash("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}
	manifest := ClosedBookManifest{
		ADR:               "ADR-0016.13 canary (protocol per ADR-0016.9)",
		RunID:             runID,
		GeneratedAt:       time.Now().UTC(),
		GitCommit:         gitHeadCommit(),
		TaskCorpusSHA256:  corpusHash,
		Identity:          identity,
		TrialsPerTask:     trials,
		SessionMetaJSON:   string(metaJSON),
		ScoringCodeCommit: gitHeadCommit(),
	}

	var eps []*Episode
	for _, task := range tasks {
		for trial := 1; trial <= trials; trial++ {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			ep, runErr := RunClosedBookEpisode(ctx, cfg, task, trial)
			cancel()
			if runErr != nil {
				t.Logf("%s trial %d/%d: run error: %v", task.ID, trial, trials, runErr)
				continue
			}
			eps = append(eps, ep)
			path := filepath.Join(episodesDir, fmt.Sprintf("%s_closed-book_%d.json", task.ID, time.Now().UnixNano()))
			if err := os.MkdirAll(episodesDir, 0o755); err == nil {
				data, mErr := json.MarshalIndent(ep, "", "  ")
				if mErr == nil {
					_ = os.WriteFile(path, data, 0o644)
				}
			}
			mean := episodeMeanCorrectness([]*Episode{ep})
			status := "PASS"
			if mean >= closedBookBar {
				status = "FAIL(<0.40 bar)"
			}
			t.Logf("%s trial %d/%d: correctness %.2f [%s]", task.ID, trial, trials, mean, status)
		}
	}

	if len(eps) == 0 {
		t.Skip("no closed-book episodes completed — cannot compute canary means (artifacts skipped; nothing fabricated)")
	}

	summary := ComputeClosedBookSummary(runID, trials, eps, nil, "", tasks)
	if _, err := SaveClosedBookJSON(outDir, "manifest.json", manifest); err != nil {
		t.Logf("warning: manifest write failed: %v", err)
	}
	if _, err := SaveClosedBookJSON(outDir, "closed-book-summary.json", summary); err != nil {
		t.Logf("warning: summary write failed: %v", err)
	}

	passed := true
	for _, ts := range summary.Tasks {
		bar := "PASS"
		if ts.MeanCorrectness >= closedBookBar {
			bar = "FAIL"
			passed = false
		}
		t.Logf("CANARY %s: closed-book mean %.3f over n=%d [%s] (bar <%.2f)",
			ts.TaskID, ts.MeanCorrectness, ts.Trials, bar, closedBookBar)
	}
	if !passed {
		t.Errorf("ADR-0016.13 canary FAILED: at least one replacement fixture scored >= %.2f closed-book — memorization risk, fixture must be hardened before commit/citable use", closedBookBar)
	}
}

// closedBookBar is the ADR-0016.9 acceptance bar: a task whose closed-book
// mean reaches 0.40 or above is memorization-dominated and cannot serve as a
// discriminating eval fixture (ADR-0016.13 canary requirement).
const closedBookBar = 0.40

func taskIDs(tasks []Task) []string {
	var ids []string
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

func atoi(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func episodeMeanCorrectness(eps []*Episode) float64 {
	if len(eps) == 0 {
		return 0
	}
	var sum float64
	for _, ep := range eps {
		sum += ep.Rewards.Correctness
	}
	return sum / float64(len(eps))
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// gitHeadCommit mirrors the helper in closedbook_e2e_test.go for this file.
func gitHeadCommit() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
