package evals

import (
	"fmt"
	"strings"
)

// rubricVersion versions the fixed core rubric below. It is a committed
// measurement artifact (ADR-0023.1 D4): changing any dimension name, guidance,
// or the score scale is a measurement-stack change that must re-trigger
// calibration before scores are citable.
const rubricVersion = "core-rubric-v1"

// Judge criteria-count bounds (ADR-0023.1 D3: "2–5 task-specific criteria").
const (
	judgeMinCriteria = 2
	judgeMaxCriteria = 5
)

// JudgeRubric holds the task author's task-specific judge criteria
// (ADR-0023.1 D3). They are seeded from the task's checks (expectedFiles /
// requiredClaims) plus what a correct exploration of that repo looks like,
// and are authored by a human — LLM-generated rubrics at eval time are
// rejected for now (D3, ADR-0023 OQ-4). These are committed rubric artifacts.
type JudgeRubric struct {
	Criteria []string `json:"criteria"`
}

// validate enforces the criteria-count bounds and non-blank entries. A nil
// rubric is valid (the block is optional); the core rubric always applies.
func (r *JudgeRubric) validate(taskID string) error {
	if r == nil {
		return nil
	}
	if len(r.Criteria) < judgeMinCriteria || len(r.Criteria) > judgeMaxCriteria {
		return fmt.Errorf(
			"task %s: judge.criteria has %d entries, must be between %d and %d (ADR-0023.1 D3)",
			taskID, len(r.Criteria), judgeMinCriteria, judgeMaxCriteria)
	}
	for i, c := range r.Criteria {
		if strings.TrimSpace(c) == "" {
			return fmt.Errorf("task %s: judge.criteria entry %d is blank", taskID, i)
		}
	}
	return nil
}

// RubricDimension is one fixed core-rubric dimension the judge scores 1–5.
type RubricDimension struct {
	// Name is the stable machine identifier used in emission
	// (gen_ai.evaluation.name = "judge.<name>") and aggregation.
	Name string
	// Title is the human-facing label in the prompt.
	Title string
	// Guidance tells the judge what a high vs low score means, with explicit
	// anchors so scoring is length-agnostic and reproducible (the largest
	// measurable κ intervention per ADR-0023 Q2 scaffolding evidence).
	Guidance string
}

// CoreRubric is the fixed three-dimension rubric applied to every judged
// episode (ADR-0023.1 D3). The dimensions are versioned in code, never
// LLM-generated, and never task-varying — only the task criteria vary.
type CoreRubric struct {
	Version    string
	Dimensions []RubricDimension
	// ScaleMin / ScaleMax bound each dimension score.
	ScaleMin int
	ScaleMax int
}

// coreRubric is the single committed instance. The three dimensions come
// straight from ADR-0023.1 D3.
var coreRubric = CoreRubric{
	Version:  rubricVersion,
	ScaleMin: 1,
	ScaleMax: 5,
	Dimensions: []RubricDimension{
		{
			Name:  "evidence_groundedness",
			Title: "Evidence groundedness",
			Guidance: "Do the answer's claims cite the specific files, symbols, and " +
				"commands that support them? 5 = every substantive claim is anchored to " +
				"an inspectable file path or command output; 3 = claims are mostly " +
				"anchored but some assertions float free; 1 = conclusions with no " +
				"traceable evidence. Judge the grounding, not the prose length.",
		},
		{
			Name:  "exploration_efficiency",
			Title: "Exploration efficiency",
			Guidance: "Was the trajectory a tight, purposeful investigation? 5 = went " +
				"to the right files quickly with no flailing and no redundant re-reads; " +
				"3 = reached the answer but with some wasted or repeated steps; 1 = " +
				"aimless, heavily redundant, or budget-blowing exploration. A correct " +
				"answer reached wastefully does not earn a 5.",
		},
		{
			Name:  "uncertainty_honesty",
			Title: "Uncertainty honesty",
			Guidance: "Are unverified or inferred things labeled as such rather than " +
				"asserted as fact? 5 = clean separation of verified vs inferred vs " +
				"unverified, with honest gaps stated; 3 = mostly honest but some " +
				"inferences presented as verified; 1 = confident claims that were never " +
				"actually checked. Overclaiming is penalized even when the guess is right.",
		},
	},
}

// CoreRubricSpec returns the committed core rubric (defensive copy of the
// dimension slice so callers cannot mutate the shared instance).
func CoreRubricSpec() CoreRubric {
	dims := make([]RubricDimension, len(coreRubric.Dimensions))
	copy(dims, coreRubric.Dimensions)
	out := coreRubric
	out.Dimensions = dims
	return out
}

// dimensionNames returns the stable dimension identifiers in canonical order.
func (c CoreRubric) dimensionNames() []string {
	names := make([]string, len(c.Dimensions))
	for i, d := range c.Dimensions {
		names[i] = d.Name
	}
	return names
}
