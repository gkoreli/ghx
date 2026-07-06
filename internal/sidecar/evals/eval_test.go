//go:build agent_e2e

package evals

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TestEpisodes runs every task under every profile against a live ACP agent
// and writes episode artifacts to .ghx-evals/runs/<run-id>/.
//
// Manual invocation (ADR-0016):
//
//	go test ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes -v
//	go test ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes -count=5
//
// The agent command comes from GHX_EVAL_AGENT, falling back to the sidecar
// config (~/.ghx/config.json). Requires a GitHub token and network
// access; the test skips with a preflight diagnostic when the environment
// is not ready, and fails when the agent itself cannot complete an episode.
func TestEpisodes(t *testing.T) {
	t.Setenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", "true")

	agentCmd := os.Getenv("GHX_EVAL_AGENT")
	if agentCmd == "" {
		agentCmd = sidecar.LoadConfig().AgentCmd
	}

	// Preflight must probe the agent this run will exec, not the host's
	// configured agent — GHX_EVAL_AGENT usually points at the pinned
	// eval wrapper (scripts/eval-agent-acp.sh).
	pf := sidecar.RunPreflightForAgent(context.Background(), agentCmd)
	if !pf.Passed {
		t.Skipf("preflight failed — environment not ready for live episodes:\n%s", sidecar.FormatPreflight(pf))
	}

	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}

	// GHX_EVAL_RUN_DIR lets multiple invocations (gate-run trial rounds)
	// accumulate episodes into one directory so the final verdict covers
	// the whole sample; without it each invocation gets a timestamped dir.
	runDir := os.Getenv("GHX_EVAL_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join(".ghx-evals", "runs", time.Now().UTC().Format("20060102-150405"))
	}
	cfg := RunConfig{
		AgentCmd:    agentCmd,
		SessionsDir: t.TempDir(),
	}
	if err := SaveRunManifest(runDir, RunManifest{
		ExpectedEpisodes: len(tasks) * len(AllProfiles()),
		Identity:         agentIdentity(cfg, nil),
	}); err != nil {
		t.Fatalf("save run manifest: %v", err)
	}

	for _, task := range tasks {
		for _, profile := range AllProfiles() {
			t.Run(task.ID+"/"+string(profile), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
				defer cancel()

				ep, err := RunEpisode(ctx, cfg, task, profile)
				if ep != nil {
					if path, saveErr := SaveEpisode(runDir, ep); saveErr != nil {
						t.Errorf("save episode: %v", saveErr)
					} else {
						t.Logf("artifact: %s", path)
					}
					t.Logf("rewards: %+v", ep.Rewards)
					t.Logf("context: %+v", ep.Context)
					// ADR-0016.7 harness alarm: all declarative anomalies are
					// logged; strict smoke runs fail only breaking severities.
					for _, anomaly := range DetectAnomalies(ep) {
						t.Logf("ANOMALY — investigate before further rounds: %s", anomaly.String())
						if os.Getenv("GHX_EVAL_STRICT") == "1" && anomaly.Severity == SeverityBreaking {
							t.Errorf("ANOMALY (strict): %s", anomaly.String())
						}
					}
				}
				if err != nil {
					t.Fatalf("episode failed: %v", err)
				}
			})
		}
	}

	// Evaluate the pre-registered ADR-0016.1 gates over everything this run
	// produced and write verdict.json + verdict.md next to the episodes.
	// A single -count run is usually below gate-run sample size; the verdict
	// notes data sufficiency, and multiple runs into the same dir accumulate.
	eps, err := LoadRunEpisodes(runDir)
	if err != nil {
		t.Fatalf("load run episodes: %v", err)
	}
	manifestIdentity := agentIdentity(cfg, nil)
	if len(eps) > 0 {
		manifestIdentity = eps[0].Identity
	}
	if err := SaveRunManifest(runDir, RunManifest{
		ExpectedEpisodes: len(tasks) * len(AllProfiles()),
		Identity:         manifestIdentity,
	}); err != nil {
		t.Fatalf("save final run manifest: %v", err)
	}
	verdict := EvaluateGates(eps)
	mdPath, err := SaveVerdict(runDir, verdict)
	if err != nil {
		t.Fatalf("save verdict: %v", err)
	}
	t.Logf("verdict: %s", mdPath)
	t.Log("\n" + FormatVerdict(verdict))
}
