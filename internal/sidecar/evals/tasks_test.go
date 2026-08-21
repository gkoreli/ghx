package evals

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTasksFromTestdata(t *testing.T) {
	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) < 3 {
		t.Fatalf("tasks = %d, want >= 3", len(tasks))
	}
	seenMultiTurn := false
	for _, task := range tasks {
		if err := task.Validate(); err != nil {
			t.Errorf("task %s invalid: %v", task.ID, err)
		}
		if len(task.Turns) > 1 {
			seenMultiTurn = true
		}
	}
	if !seenMultiTurn {
		t.Error("task set must include at least one multi-turn task (G4 memory gate)")
	}
}

func TestLoadTasksRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"id":"x","repo":"o/r","turns":["q"],"checks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTasks(dir); err == nil {
		t.Error("expected validation error for task with no positive checks")
	}
}

func TestLoadTasksEmptyDir(t *testing.T) {
	if _, err := LoadTasks(t.TempDir()); err == nil {
		t.Error("expected error for empty task dir")
	}
}

func TestTaskValidate(t *testing.T) {
	valid := Task{ID: "a", Repo: "o/r", Turns: []string{"q"}, Checks: TaskChecks{ExpectedFiles: []string{"f"}}}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid task rejected: %v", err)
	}
	for name, task := range map[string]Task{
		"no id":     {Repo: "o/r", Turns: []string{"q"}, Checks: TaskChecks{ExpectedFiles: []string{"f"}}},
		"no repo":   {ID: "a", Turns: []string{"q"}, Checks: TaskChecks{ExpectedFiles: []string{"f"}}},
		"no turns":  {ID: "a", Repo: "o/r", Checks: TaskChecks{ExpectedFiles: []string{"f"}}},
		"no checks": {ID: "a", Repo: "o/r", Turns: []string{"q"}},
	} {
		if err := task.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

// TestValidateRejectsLeakedChecks enforces the ADR-0016.2 discoverability
// rule: a check string appearing in a question could be scored by parroting.
func TestValidateRejectsLeakedChecks(t *testing.T) {
	for name, task := range map[string]Task{
		"symbol in question": {
			ID: "a", Repo: "o/r",
			Turns:  []string{"Where is the route() decorator implemented?"},
			Checks: TaskChecks{ExpectedSymbols: []string{"route"}},
		},
		"file in question": {
			ID: "a", Repo: "o/r",
			Turns:  []string{"What does src/compose.ts do?"},
			Checks: TaskChecks{ExpectedFiles: []string{"src/compose.ts"}},
		},
		"claim in question": {
			ID: "a", Repo: "o/r",
			Turns:  []string{"Is the router an external package?"},
			Checks: TaskChecks{ExpectedFiles: []string{"lib/app.js"}, RequiredClaims: []string{"external package"}},
		},
		"leaked unacceptable claim": {
			ID: "a", Repo: "o/r",
			Turns:  []string{"Is routing in lib/router?"},
			Checks: TaskChecks{ExpectedFiles: []string{"lib/app.js"}, UnacceptableClaims: []string{"lib/router"}},
		},
		"case-insensitive leak": {
			ID: "a", Repo: "o/r",
			Turns:  []string{"Is the Router implemented here?"},
			Checks: TaskChecks{ExpectedSymbols: []string{"router"}},
		},
		"blank check": {
			ID: "a", Repo: "o/r",
			Turns:  []string{"Where is X?"},
			Checks: TaskChecks{ExpectedFiles: []string{"lib/app.js"}, ExpectedSymbols: []string{"  "}},
		},
	} {
		if err := task.Validate(); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}

	discoverable := Task{
		ID: "a", Repo: "o/r",
		Turns:  []string{"Where is middleware composition implemented?"},
		Checks: TaskChecks{ExpectedFiles: []string{"src/compose.ts"}, ExpectedSymbols: []string{"compose"}},
	}
	if err := discoverable.Validate(); err != nil {
		t.Errorf("discoverable checks rejected: %v", err)
	}
}

// TestADR0016_13TaskFixturesValid pins the ADR-0016.13 R1–R4 corpus-refresh
// fixtures: each new task must load, validate, keep its ground-truth facts
// discoverable (never named in the question — H2 lesson), and carry judge
// criteria. R4's engine-selection task must stay non-self-referential
// (repo != gkoreli/ghx), unlike ghx-mapengine.
func TestADR0016_13TaskFixturesValid(t *testing.T) {
	ids := map[string]string{
		"werkzeug-delegation":       "pallets/werkzeug",
		"gin-route-conflict":        "gin-gonic/gin",
		"hono-smartrouter-fallback": "honojs/hono",
		"gjson-engine-selection":    "tidwall/gjson",
	}
	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Task{}
	for _, task := range tasks {
		byID[task.ID] = task
	}
	for id, repo := range ids {
		task, ok := byID[id]
		if !ok {
			t.Errorf("fixture %s missing from testdata/tasks", id)
			continue
		}
		if err := task.Validate(); err != nil {
			t.Errorf("task %s invalid: %v", id, err)
		}
		if task.Repo != repo {
			t.Errorf("task %s repo = %q, want %q", id, task.Repo, repo)
		}
		// H2: the question must never name the repo under test.
		repoName := strings.TrimSuffix(repo, "/"+path.Base(repo))
		for i, turn := range task.Turns {
			if strings.Contains(strings.ToLower(turn), strings.ToLower(repo)) ||
				strings.Contains(strings.ToLower(turn), strings.ToLower(path.Base(repo))) ||
				strings.Contains(strings.ToLower(turn), strings.ToLower(repoName)) {
				t.Errorf("task %s turn %d names its repo %q (leak)", id, i, repo)
			}
		}
		if len(task.Checks.ExpectedFiles)+len(task.Checks.ExpectedSymbols)+len(task.Checks.RequiredClaims) == 0 {
			t.Errorf("task %s has no pre-registered ground-truth facts", id)
		}
		if task.Judge == nil || len(task.Judge.Criteria) == 0 {
			t.Errorf("task %s missing judge criteria", id)
		}
	}
	// R4 non-self-referential: the engine-selection task must not target ghx.
	if task, ok := byID["gjson-engine-selection"]; !ok || task.Repo == "gkoreli/ghx" {
		t.Errorf("gjson-engine-selection must be non-self-referential, repo = %q", task.Repo)
	}
}
