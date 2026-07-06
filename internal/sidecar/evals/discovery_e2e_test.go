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

// TestDiscoveryEpisodes runs every ADR-0019.2 discovery task under every
// profile against a live ACP agent and writes episode artifacts plus the
// separate discovery verdict to .ghx-evals/runs/<run-id>/.
//
// It mirrors TestEpisodes' shape for discovery tasks and replaces the
// throwaway driver disclosed in docs/evals/spot-instrumented-2026-07-06
// (discovery/discovery-spot-driver.go.txt): the frozen exported functions
// (RunDiscoveryEpisode, SaveEpisode, EvaluateDiscoveryGates,
// SaveDiscoveryVerdict) do all scoring/detection — this test is
// orchestration only.
//
// Manual invocation:
//
//	go test ./internal/sidecar/evals -tags=agent_e2e -run TestDiscoveryEpisodes -v
//	go test ./internal/sidecar/evals -tags=agent_e2e -run 'TestDiscoveryEpisodes/episodes/<task>/<profile>' -v
//
// Differences from TestEpisodes, both deliberate:
//   - no baseline reuse: TryReuseBaselines/BaselineReuseHashes are defined
//     over repo-scoped tasks and their identity hash inputs; discovery has
//     no seeded baseline source yet (ADR-0025.1 scope).
//   - the verdict is the separate discovery D-G verdict
//     (discovery-verdict.{json,md}), never the repo-scoped G1-G5 one, so
//     D-G metrics cannot be confused with G1-G5 (ADR-0019.2 D4).
func TestDiscoveryEpisodes(t *testing.T) {
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
		t.Skipf("preflight failed — environment not ready for live discovery episodes:\n%s", sidecar.FormatPreflight(pf))
	}

	tasks, err := LoadDiscoveryTasks("testdata/discovery-tasks")
	if err != nil {
		t.Fatal(err)
	}

	// GHX_EVAL_RUN_DIR lets multiple invocations accumulate episodes into one
	// directory so the final verdict covers the whole sample; without it each
	// invocation gets a timestamped dir.
	runDir := os.Getenv("GHX_EVAL_RUN_DIR")
	if runDir == "" {
		runDir = filepath.Join(".ghx-evals", "runs", time.Now().UTC().Format("20060102-150405")+"-discovery")
	}
	cfg := RunConfig{
		AgentCmd:    agentCmd,
		SessionsDir: t.TempDir(),
	}
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	currentIdentity, err := ProbeAgentIdentity(probeCtx, cfg)
	probeCancel()
	if err != nil {
		t.Fatalf("probe agent identity: %v", err)
	}
	// Each invocation plans one trial of the full discovery tasks × profiles
	// matrix (ADR-0016.8 D5); accumulating runs append a round per invocation.
	if _, err := RecordManifestRound(runDir, PlannedRound{
		Tasks:    len(tasks),
		Profiles: len(AllProfiles()),
		Trials:   1,
	}, currentIdentity); err != nil {
		t.Fatalf("record manifest round: %v", err)
	}

	// ADR-0025 D3: bound episode-level parallelism exactly like TestEpisodes —
	// the matrix nests under one parent subtest that blocks until every episode
	// is saved before the discovery verdict runs.
	gate := newParallelGate(EvalParallelism())
	t.Logf("episode parallelism: %d (GHX_EVAL_PARALLEL, 1 = sequential)", gate.limit)
	t.Run("episodes", func(t *testing.T) {
		for _, task := range tasks {
			for _, profile := range AllProfiles() {
				task := task
				t.Run(task.ID+"/"+string(profile), func(t *testing.T) {
					if gate.enabled() {
						t.Parallel()
					}
					gate.acquire()
					defer gate.release()

					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
					defer cancel()

					ep, err := RunDiscoveryEpisode(ctx, cfg, task, profile)
					if ep != nil {
						// Duration honesty (ADR-0025 D3): set before SaveEpisode
						// so the marker reaches the episode JSON and the
						// re-derived anomalies (ADR-0016.8 D6) see it.
						ep.Parallel = gate.enabled()
						if path, saveErr := SaveEpisode(runDir, ep); saveErr != nil {
							t.Errorf("save episode: %v", saveErr)
						} else {
							t.Logf("artifact: %s", path)
						}
						if ep.DiscoveryRewards != nil {
							t.Logf("discoveryRewards: %+v", *ep.DiscoveryRewards)
						}
						t.Logf("context: %+v", ep.Context)
						// ADR-0016.7 harness alarm: all declarative anomalies are
						// logged; strict runs fail only breaking severities.
						for _, anomaly := range ep.Anomalies {
							t.Logf("ANOMALY — investigate before further rounds: %s", anomaly.String())
							if os.Getenv("GHX_EVAL_STRICT") == "1" && anomaly.Severity == SeverityBreaking {
								t.Errorf("ANOMALY (strict): %s", anomaly.String())
							}
						}
					}
					if err != nil {
						t.Fatalf("discovery episode failed: %v", err)
					}
				})
			}
		}
	})

	// Evaluate the pre-registered ADR-0019.2 D4 discovery gates over everything
	// this run produced and write discovery-verdict.{json,md} next to the
	// episodes. A single invocation is below the discovery gate-run sample; the
	// verdict notes data sufficiency, and multiple runs into the same dir
	// accumulate.
	eps, err := LoadRunEpisodes(runDir)
	if err != nil {
		t.Fatalf("load run episodes: %v", err)
	}
	manifestIdentity := currentIdentity
	if len(eps) > 0 {
		manifestIdentity = eps[0].Identity
	}
	// Refresh identity with what the live episodes actually reported; the
	// planned rounds recorded above must not be overwritten (ADR-0016.8 D5).
	if err := UpdateManifestIdentity(runDir, manifestIdentity); err != nil {
		t.Fatalf("update run manifest identity: %v", err)
	}
	verdict := EvaluateDiscoveryGates(eps)
	mdPath, err := SaveDiscoveryVerdict(runDir, verdict)
	if err != nil {
		t.Fatalf("save discovery verdict: %v", err)
	}
	t.Logf("discovery verdict: %s", mdPath)
	t.Log("\n" + FormatDiscoveryVerdict(verdict))
}
