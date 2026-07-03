//go:build agent_e2e

package evals

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestTaskGroundTruth is the ADR-0016.2 pre-run freshness check: every
// expectedFiles entry of every fixture must exist on the repo's default
// branch, or the fixture has drifted and a gate run against it would waste
// the spend (or blame the sidecar for a stale path). Needs network only —
// no LLM, no agent, no token. Required green before a formal gate run and
// after any fixture edit.
func TestTaskGroundTruth(t *testing.T) {
	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	for _, task := range tasks {
		for _, file := range task.Checks.ExpectedFiles {
			url := fmt.Sprintf("https://raw.githubusercontent.com/%s/HEAD/%s", task.Repo, file)
			resp, err := client.Head(url)
			if err != nil {
				t.Errorf("task %s: %s: %v", task.ID, url, err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("task %s: expected file %q not found on %s default branch (HTTP %d) — fixture has drifted",
					task.ID, file, task.Repo, resp.StatusCode)
			}
		}
	}
}
