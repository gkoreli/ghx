package evals

import (
	"os"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// goldenPromptEpisode is a compact, deterministic episode used for the prompt
// golden. It is intentionally small so the golden file stays reviewable.
func goldenPromptTask() Task {
	return Task{
		ID:    "gin-routing",
		Repo:  "gin-gonic/gin",
		Turns: []string{"Where are routes registered?", "How are :name params captured?"},
		Judge: &JudgeRubric{Criteria: []string{
			"Locates route registration and the radix-tree matching path.",
			"Explains parameter capture during tree traversal with evidence.",
		}},
	}
}

func goldenPromptEpisode() *Episode {
	return &Episode{
		TaskID: "gin-routing",
		Repo:   "gin-gonic/gin",
		Turns: []TurnRecord{
			{
				Turn:     0,
				Question: "Where are routes registered?",
				Thinking: "Map routergroup.go first, then tree.go.",
				Text:     "Routes register through RouterGroup in routergroup.go.",
				ToolTraces: []sidecar.ToolCallTrace{{
					Title:         "ghx read gin-gonic/gin routergroup.go --map",
					OutputExcerpt: "func (group *RouterGroup) handle(httpMethod, relativePath string, handlers HandlersChain) IRoutes\nmore signatures follow",
					OutputSize:    1420,
				}},
			},
			{
				Turn:     1,
				Question: "How are :name params captured?",
				Text:     "Params are captured in tree.go during getValue tree traversal.",
				ToolTraces: []sidecar.ToolCallTrace{{
					Title:         "ghx read gin-gonic/gin tree.go --map",
					OutputExcerpt: "func (n *node) getValue(path string, params *Params, ...) nodeValue",
					OutputSize:    980,
				}},
			},
		},
		Report: &sidecar.Report{
			Answer:        "Routes register via RouterGroup (routergroup.go); matching walks the radix tree in tree.go.",
			Verified:      []sidecar.Claim{{Summary: "addRoute builds the routing tree", Evidence: "tree.go:200"}},
			RelevantFiles: []sidecar.RelevantFile{{Path: "routergroup.go", Reason: "route registration entrypoint"}},
			Evidence:      []sidecar.Evidence{{Source: "ghx read routergroup.go --map", Summary: "shows handle() registering routes"}},
			CommandsRun:   []string{"ghx read gin-gonic/gin routergroup.go --map", "ghx read gin-gonic/gin tree.go --map"},
			Uncertainty:   []string{"wildcard param edge cases not fully traced"},
		},
	}
}

func TestRenderJudgePromptGolden(t *testing.T) {
	bundle := BuildJudgeBundle(goldenPromptEpisode())
	got, err := RenderJudgePrompt(goldenPromptTask(), bundle)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	goldenPath := "testdata/judge/prompt_golden.txt"
	if os.Getenv("GHX_UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
		t.Logf("updated golden %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with GHX_UPDATE_GOLDEN=1 to create): %v", err)
	}
	if got != string(want) {
		t.Errorf("rendered prompt does not match golden %s.\n--- got ---\n%s", goldenPath, got)
	}
}

func TestRenderJudgePromptDeterministic(t *testing.T) {
	task := goldenPromptTask()
	bundle := BuildJudgeBundle(goldenPromptEpisode())
	a, err := RenderJudgePrompt(task, bundle)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderJudgePrompt(task, bundle)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("rendering is not deterministic")
	}
}
