package evals

import (
	"encoding/json"
	"fmt"
	"strings"
)

// judgePromptVersion versions the rendered prompt template. It is a committed
// artifact (ADR-0023.1 D4); any change re-triggers calibration before scores
// are citable. It rides on every JudgeResult so a score is always traceable to
// the exact prompt that produced it.
const judgePromptVersion = "judge-prompt-v1"

// RenderJudgePrompt renders the full judge input for one episode: the fixed
// core rubric, the task-specific criteria, the question(s), the profile-blind
// evidence bundle, and the required output schema (ADR-0023.1 D3). Rendering
// is deterministic (golden-tested) so the same bundle always yields byte-
// identical judge input.
func RenderJudgePrompt(task Task, bundle JudgeBundle) (string, error) {
	rubric := CoreRubricSpec()
	bundleJSON, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal judge bundle: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("# ghx Reconnaissance Judge\n\n")
	sb.WriteString(
		"You are an impartial evaluator of code-reconnaissance work on a GitHub " +
			"repository. You are given the question(s) that were asked and a structured, " +
			"profile-blind bundle of what the agent did and answered. Judge only the " +
			"work in the bundle. You do NOT know which tool or architecture produced it " +
			"and you must not speculate about that — score the reconnaissance on its own " +
			"merits.\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Base every judgment strictly on the bundle; do not assume facts not present.\n")
	sb.WriteString("- Length is not quality. A short, well-grounded answer beats a long, unanchored one.\n")
	sb.WriteString("- A correct final answer reached by wasteful or dishonest means is not top-rated.\n\n")

	fmt.Fprintf(&sb, "## Core rubric (fixed) — score each dimension %d–%d\n\n", rubric.ScaleMin, rubric.ScaleMax)
	for i, d := range rubric.Dimensions {
		fmt.Fprintf(&sb, "%d. **%s** (`%s`)\n   %s\n\n", i+1, d.Title, d.Name, d.Guidance)
	}

	sb.WriteString("## Task-specific criteria\n\n")
	if task.Judge != nil && len(task.Judge.Criteria) > 0 {
		sb.WriteString("Weigh these task-specific expectations within the core dimensions above:\n")
		for _, c := range task.Judge.Criteria {
			fmt.Fprintf(&sb, "- %s\n", c)
		}
	} else {
		sb.WriteString("(none provided for this task — apply the core rubric only)\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## Question(s) asked\n\n")
	for i, q := range bundle.Questions {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, q)
	}
	sb.WriteString("\n")

	sb.WriteString("## Evidence bundle\n\n")
	sb.WriteString("```json\n")
	sb.Write(bundleJSON)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Output\n\n")
	sb.WriteString(
		"Return ONLY a JSON object, no prose outside it, in exactly this shape:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString(outputSchemaExample(rubric))
	sb.WriteString("\n```\n")
	sb.WriteString(fmt.Sprintf(
		"\nEach `score` is an integer %d–%d. `overall` is your holistic %d–%d rating "+
			"(not necessarily the mean). Every `explanation` cites the specific bundle "+
			"evidence behind the score.\n", rubric.ScaleMin, rubric.ScaleMax, rubric.ScaleMin, rubric.ScaleMax))

	return sb.String(), nil
}

// outputSchemaExample renders the exact JSON envelope the judge must return,
// enumerating the fixed dimensions so the model cannot invent its own.
func outputSchemaExample(rubric CoreRubric) string {
	var sb strings.Builder
	sb.WriteString("{\n")
	sb.WriteString("  \"dimensions\": [\n")
	for i, d := range rubric.Dimensions {
		comma := ","
		if i == len(rubric.Dimensions)-1 {
			comma = ""
		}
		fmt.Fprintf(&sb, "    {\"dimension\": %q, \"score\": <%d-%d>, \"explanation\": \"<why>\"}%s\n",
			d.Name, rubric.ScaleMin, rubric.ScaleMax, comma)
	}
	sb.WriteString("  ],\n")
	fmt.Fprintf(&sb, "  \"overall\": <%d-%d>,\n", rubric.ScaleMin, rubric.ScaleMax)
	sb.WriteString("  \"explanation\": \"<one-paragraph holistic summary>\"\n")
	sb.WriteString("}")
	return sb.String()
}
