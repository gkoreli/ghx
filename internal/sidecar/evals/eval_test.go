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
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Minute)
	currentIdentity, err := ProbeAgentIdentity(probeCtx, cfg)
	probeCancel()
	if err != nil {
		t.Fatalf("probe agent identity: %v", err)
	}
	// Each invocation plans one trial of the full tasks × profiles matrix;
	// multi-trial gate runs accumulating into one GHX_EVAL_RUN_DIR append a
	// round per invocation, so the manifest carries the whole planned matrix
	// and the derived expected total (ADR-0016.8 D5). The ADR-0025 D1
	// sequential-stopping bounds read that accumulated total; the
	// GHX_EVAL_EXPECTED_EPISODES escape hatch (applied and recorded inside
	// RecordManifestRound) remains for pre-declaring a bigger planned run.
	manifest, err := RecordManifestRound(runDir, PlannedRound{
		Tasks:    len(tasks),
		Profiles: len(AllProfiles()),
		Trials:   1,
	}, currentIdentity)
	if err != nil {
		t.Fatalf("record manifest round: %v", err)
	}
	identityHashes, reason, err := BaselineReuseHashes(cfg, tasks, "testdata/tasks", currentIdentity)
	if err != nil {
		t.Fatalf("record manifest identity hashes: %s: %v", reason, err)
	}
	if reason != "" {
		t.Fatalf("record manifest identity hashes: %s", reason)
	}
	manifest, err = RecordManifestIdentityHashes(runDir, identityHashes)
	if err != nil {
		t.Fatalf("record manifest identity hashes: %v", err)
	}
	expectedEpisodes := manifest.ExpectedEpisodes
	var reuse *BaselineReuse
	if priorRunDir := os.Getenv(BaselineReuseEnv); priorRunDir != "" {
		var ok bool
		var reason string
		plannedTrials := PlannedBaselineTrials(manifest)
		reuse, ok, reason, err = TryReuseBaselines(runDir, priorRunDir, cfg, tasks, "testdata/tasks", plannedTrials, currentIdentity, manifest.CreatedAt)
		if err != nil {
			t.Fatalf("baseline reuse: %v", err)
		}
		if ok {
			manifest.BaselineReuse = reuse
			if err := SaveRunManifest(runDir, *manifest); err != nil {
				t.Fatalf("save baseline reuse manifest: %v", err)
			}
			t.Logf("baseline reuse: copied %d episodes from %s", len(reuse.Episodes), reuse.ReusedFromRunID)
		} else {
			refusal := BaselineReuseRefusalMessage(priorRunDir, reason)
			if !BaselineFreshFallbackAllowed() {
				t.Fatalf("%s", refusal)
			}
			manifest, err = RecordBaselineFallback(runDir, BaselineFallback{
				Mode:             BaselineFallbackFresh,
				RequestedRunDir:  priorRunDir,
				RefusalReason:    reason,
				Remediation:      refusal,
				ControllingEnv:   BaselineFallbackEnv,
				ControllingValue: BaselineFallbackFresh,
			})
			if err != nil {
				t.Fatalf("record baseline fallback: %v", err)
			}
			t.Logf("%s", refusal)
		}
	}

	// ADR-0025 D3: bound episode-level parallelism. Each task × profile cell is
	// a distinct subtest, so trials of the same cell never overlap within one
	// invocation; the gate only caps total fan-out. t.Parallel() subtests run
	// once their parent returns, so the whole matrix is nested under one parent
	// subtest that blocks until every episode is saved before the verdict runs.
	gate := newParallelGate(EvalParallelism())
	t.Logf("episode parallelism: %d (GHX_EVAL_PARALLEL, 1 = sequential)", gate.limit)
	t.Run("episodes", func(t *testing.T) {
		for _, task := range tasks {
			for _, profile := range AllProfiles() {
				if reuse != nil && (profile == ProfilePlain || profile == ProfileGhx) {
					continue
				}
				t.Run(task.ID+"/"+string(profile), func(t *testing.T) {
					if gate.enabled() {
						t.Parallel()
					}
					gate.acquire()
					defer gate.release()

					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
					defer cancel()

					ep, err := RunEpisode(ctx, cfg, task, profile)
					if ep != nil {
						// Duration honesty (ADR-0025 D3): mark episodes that
						// could have run concurrently so latency claims exclude
						// them. Set before SaveEpisode so the marker reaches the
						// episode JSON and the emitted duration metrics.
						ep.Parallel = gate.enabled()
						// SaveEpisode re-derives ep.Anomalies just before
						// persistence (ADR-0016.8 D6) — the Parallel marker
						// above must be set first so the rate-limit detector
						// sees it.
						if path, saveErr := SaveEpisode(runDir, ep); saveErr != nil {
							t.Errorf("save episode: %v", saveErr)
						} else {
							t.Logf("artifact: %s", path)
						}
						t.Logf("rewards: %+v", ep.Rewards)
						t.Logf("context: %+v", ep.Context)
						// ADR-0016.7 harness alarm: all declarative anomalies are
						// logged; strict smoke runs fail only breaking severities.
						for _, anomaly := range ep.Anomalies {
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
	})

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
	// Refresh identity with what the live episodes actually reported; the
	// planned rounds recorded above must not be overwritten (ADR-0016.8 D5).
	if err := UpdateManifestIdentity(runDir, manifestIdentity); err != nil {
		t.Fatalf("update run manifest identity: %v", err)
	}
	verdict := EvaluateGates(eps)
	LabelBaselineReused(&verdict, reuse)
	// ADR-0025 D1: record the sequential-stopping recommendation next to the
	// verdict. The runner never auto-stops — a human ends the loop.
	stopping := ComputeStoppingBounds(eps, expectedEpisodes)
	verdict.Stopping = &stopping
	mdPath, err := SaveVerdict(runDir, verdict)
	if err != nil {
		t.Fatalf("save verdict: %v", err)
	}
	t.Logf("verdict: %s", mdPath)
	t.Log("\n" + FormatVerdict(verdict))
}
