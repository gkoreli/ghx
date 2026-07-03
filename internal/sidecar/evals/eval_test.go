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
// config (~/.ghx-sidecar/config.json). Requires a GitHub token and network
// access; the test skips with a preflight diagnostic when the environment
// is not ready, and fails when the agent itself cannot complete an episode.
func TestEpisodes(t *testing.T) {
	pf := sidecar.RunPreflight(context.Background())
	if !pf.Passed {
		t.Skipf("preflight failed — environment not ready for live episodes:\n%s", sidecar.FormatPreflight(pf))
	}

	agentCmd := os.Getenv("GHX_EVAL_AGENT")
	if agentCmd == "" {
		agentCmd = sidecar.LoadConfig().AgentCmd
	}

	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}

	runDir := filepath.Join(".ghx-evals", "runs", time.Now().UTC().Format("20060102-150405"))
	cfg := RunConfig{
		AgentCmd:    agentCmd,
		SessionsDir: t.TempDir(),
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
				}
				if err != nil {
					t.Fatalf("episode failed: %v", err)
				}
			})
		}
	}
}
