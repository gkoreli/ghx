package evals

import (
	"os"
	"path/filepath"
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
