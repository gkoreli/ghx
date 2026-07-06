package evals

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	ghxRewardCheckEvent   = "ghx.eval.reward.check"
	ghxRewardPenaltyEvent = "ghx.eval.reward.penalty"
)

func addRewardExplanationEvents(span trace.Span, ep *Episode) {
	addEvaluationResultEvents(span, ep)
	addCorrectnessEvents(span, ep)
	addEvidenceEvents(span, ep)
	addTrajectoryEvents(span, ep)
	addCompressionEvents(span, ep)
	addMemoryEvents(span, ep)
	addSafetyEvents(span, ep)
}

// judgeEvent is one gen_ai.evaluation.result event's name and attributes,
// factored out so emission is unit-testable without a span (ADR-0023.1 D6).
type judgeEvent struct {
	Name  string
	Attrs []attribute.KeyValue
}

// judgeEvaluationEvents builds the gen_ai.evaluation.result events for a
// JudgeResult — one per core-rubric dimension plus an overall — reusing the
// SAME event shape as the deterministic reward events (constraint 3: extend,
// never fork). Skipped or nil results emit nothing. The judge's per-dimension
// median score rides as gen_ai.evaluation.score.value, its reasoning as
// gen_ai.evaluation.explanation, and the judge model id / prompt version /
// disagreement as sibling attributes.
func judgeEvaluationEvents(res *JudgeResult) []judgeEvent {
	if res == nil || res.Skipped {
		return nil
	}
	base := func() []attribute.KeyValue {
		return []attribute.KeyValue{
			attribute.String(genAIRequestModelAttribute, res.JudgeModelID),
			attribute.String("ghx.judge.prompt_version", res.PromptVersion),
			attribute.String("ghx.judge.rubric_version", res.RubricVersion),
			attribute.Int("ghx.judge.samples", res.Samples),
			attribute.Bool("ghx.judge.calibrated", res.Calibrated),
		}
	}
	var events []judgeEvent
	for _, d := range res.Dimensions {
		attrs := append(base(),
			attribute.String(genAIEvaluationNameAttribute, "judge."+d.Dimension),
			attribute.Float64(genAIEvaluationScoreValueAttribute, d.MedianScore),
			attribute.String(genAIEvaluationExplanationAttribute, boundedString(d.Explanation, 1024)),
			attribute.Int("ghx.judge.disagreement", d.Disagreement),
		)
		events = append(events, judgeEvent{Name: genAIEvaluationResultEvent, Attrs: attrs})
	}
	overallExplanation := ""
	if len(res.Verdicts) > 0 {
		overallExplanation = res.Verdicts[0].Explanation
	}
	events = append(events, judgeEvent{Name: genAIEvaluationResultEvent, Attrs: append(base(),
		attribute.String(genAIEvaluationNameAttribute, "judge.overall"),
		attribute.Float64(genAIEvaluationScoreValueAttribute, res.Overall),
		attribute.String(genAIEvaluationExplanationAttribute, boundedString(overallExplanation, 1024)),
		attribute.Int("ghx.judge.disagreement", res.MaxDisagreement),
	)})
	return events
}

// addJudgeEvaluationEvents adds a JudgeResult's evaluation events to a span,
// alongside (and identically shaped to) the deterministic reward events.
func addJudgeEvaluationEvents(span trace.Span, res *JudgeResult) {
	for _, e := range judgeEvaluationEvents(res) {
		span.AddEvent(e.Name, trace.WithAttributes(e.Attrs...))
	}
}

func addEvaluationResultEvents(span trace.Span, ep *Episode) {
	r := ep.Rewards
	components := []struct {
		name  string
		value float64
	}{
		{"correctness", r.Correctness},
		{"evidence", r.Evidence},
		{"trajectory", r.Trajectory},
		{"compression", r.Compression},
		{"memory", r.Memory},
		{"safety", r.Safety},
		{"overall", r.Overall},
	}
	for _, component := range components {
		span.AddEvent(genAIEvaluationResultEvent, trace.WithAttributes(
			attribute.String(genAIEvaluationNameAttribute, "ghx.eval.reward."+component.name),
			attribute.Float64(genAIEvaluationScoreValueAttribute, component.value),
			attribute.String("ghx.eval.reward.component", component.name),
		))
	}
}

func addCorrectnessEvents(span trace.Span, ep *Episode) {
	text := answerText(ep)
	c := ep.Checks
	for _, file := range c.ExpectedFiles {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "correctness"),
			attribute.String("ghx.eval.check.kind", "expected_file"),
			attribute.String("ghx.eval.check.expected", file),
			attribute.Bool("ghx.eval.check.matched", fileIdentified(ep, text, file)),
		))
	}
	for _, symbol := range c.ExpectedSymbols {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "correctness"),
			attribute.String("ghx.eval.check.kind", "expected_symbol"),
			attribute.String("ghx.eval.check.expected", symbol),
			attribute.Bool("ghx.eval.check.matched", symbolIdentified(text, symbol)),
		))
	}
	for _, claim := range c.RequiredClaims {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "correctness"),
			attribute.String("ghx.eval.check.kind", "required_claim"),
			attribute.String("ghx.eval.check.expected", claim),
			attribute.Bool("ghx.eval.check.matched", strings.Contains(text, strings.ToLower(claim))),
		))
	}
	for _, claim := range c.UnacceptableClaims {
		hit := strings.Contains(text, strings.ToLower(claim))
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "correctness"),
			attribute.String("ghx.eval.check.kind", "unacceptable_claim"),
			attribute.String("ghx.eval.check.expected", claim),
			attribute.Bool("ghx.eval.check.matched", hit),
			attribute.Bool("ghx.eval.penalty.applied", hit),
		))
	}
}

func addEvidenceEvents(span trace.Span, ep *Episode) {
	if ep.Report == nil {
		toolsUsed := len(allToolCalls(ep)) > 0
		pathsMentioned := len(pathTokens(answerText(ep))) > 0
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "evidence"),
			attribute.String("ghx.eval.check.kind", "direct_tool_usage"),
			attribute.Bool("ghx.eval.check.matched", toolsUsed),
		))
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "evidence"),
			attribute.String("ghx.eval.check.kind", "direct_path_citation"),
			attribute.Bool("ghx.eval.check.matched", pathsMentioned),
		))
		return
	}

	for i, claim := range ep.Report.Verified {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "evidence"),
			attribute.String("ghx.eval.check.kind", "verified_claim_evidence"),
			attribute.Int("ghx.eval.check.index", i),
			attribute.Bool("ghx.eval.check.matched", citesEvidence(claim.Evidence)),
			attribute.String("ghx.eval.check.summary", boundedString(claim.Summary, 512)),
			attribute.String("ghx.eval.check.evidence", boundedString(claim.Evidence, 512)),
		))
	}
	span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
		attribute.String("ghx.eval.reward.component", "evidence"),
		attribute.String("ghx.eval.check.kind", "commands_run"),
		attribute.Bool("ghx.eval.check.matched", len(ep.Report.CommandsRun) > 0),
		attribute.Int("ghx.eval.commands.count", len(ep.Report.CommandsRun)),
	))
	for i, file := range ep.Report.RelevantFiles {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "evidence"),
			attribute.String("ghx.eval.check.kind", "relevant_file_reason"),
			attribute.Int("ghx.eval.check.index", i),
			attribute.String("ghx.eval.file.path", file.Path),
			attribute.Bool("ghx.eval.check.matched", strings.TrimSpace(file.Reason) != ""),
		))
	}
}

func addTrajectoryEvents(span trace.Span, ep *Episode) {
	cmds := allToolCalls(ep)
	if len(cmds) == 0 && ep.Report != nil {
		cmds = ep.Report.CommandsRun
	}
	if len(cmds) == 0 {
		span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "trajectory"),
			attribute.String("ghx.eval.penalty.kind", "no_tool_calls"),
			attribute.Bool("ghx.eval.penalty.applied", true),
		))
		return
	}

	seen := map[string]int{}
	for _, cmd := range cmds {
		seen[strings.TrimSpace(cmd)]++
	}
	dups := 0
	for cmd, count := range seen {
		if count <= 1 {
			continue
		}
		dups += count - 1
		span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "trajectory"),
			attribute.String("ghx.eval.penalty.kind", "duplicate_command"),
			attribute.String("ghx.eval.command", boundedString(cmd, 512)),
			attribute.Int("ghx.eval.command.count", count),
			attribute.Bool("ghx.eval.penalty.applied", true),
		))
	}
	budget := perQuestionBudget * len(ep.Turns)
	over := len(cmds) - budget
	if over < 0 {
		over = 0
	}
	span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
		attribute.String("ghx.eval.reward.component", "trajectory"),
		attribute.String("ghx.eval.penalty.kind", "over_budget"),
		attribute.Int("ghx.eval.command.count", len(cmds)),
		attribute.Int("ghx.eval.command.budget", budget),
		attribute.Int("ghx.eval.command.over_budget", over),
		attribute.Bool("ghx.eval.penalty.applied", over > 0),
	))
	for _, cmd := range cmds {
		lower := strings.ToLower(cmd)
		for _, avoid := range ep.Checks.AvoidPaths {
			if strings.Contains(lower, strings.ToLower(avoid)) {
				span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
					attribute.String("ghx.eval.reward.component", "trajectory"),
					attribute.String("ghx.eval.penalty.kind", "avoid_path"),
					attribute.String("ghx.eval.command", boundedString(cmd, 512)),
					attribute.String("ghx.eval.check.expected", avoid),
					attribute.Bool("ghx.eval.penalty.applied", true),
				))
			}
		}
	}
	span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
		attribute.String("ghx.eval.reward.component", "trajectory"),
		attribute.String("ghx.eval.penalty.kind", "duplicate_command_ratio"),
		attribute.Float64("ghx.eval.penalty.ratio", float64(dups)/float64(len(cmds))),
		attribute.Bool("ghx.eval.penalty.applied", dups > 0),
	))
}

func addCompressionEvents(span trace.Span, ep *Episode) {
	ratio := 0.0
	if ep.Context.TotalWorkflowChars > 0 {
		ratio = float64(ep.Context.MainAgentChars) / float64(ep.Context.TotalWorkflowChars)
	}
	span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
		attribute.String("ghx.eval.reward.component", "compression"),
		attribute.String("ghx.eval.check.kind", "main_agent_context_ratio"),
		attribute.Int("ghx.eval.main_agent_chars", ep.Context.MainAgentChars),
		attribute.Int("ghx.eval.total_workflow_chars", ep.Context.TotalWorkflowChars),
		attribute.Float64("ghx.eval.context.ratio", ratio),
	))
}

func addMemoryEvents(span trace.Span, ep *Episode) {
	if len(ep.Turns) <= 1 {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "memory"),
			attribute.String("ghx.eval.check.kind", "not_applicable_single_turn"),
			attribute.Bool("ghx.eval.reward.memory_applies", false),
		))
		return
	}
	for i, turn := range ep.Turns {
		if i == 0 {
			continue
		}
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "memory"),
			attribute.String("ghx.eval.check.kind", "followup_resumed"),
			attribute.Int("ghx.eval.turn", turn.Turn),
			attribute.Bool("ghx.eval.check.matched", turn.Resumed),
			attribute.Bool("ghx.eval.penalty.applied", !turn.Resumed),
		))
	}
	repeats, reads := repeatReadCounts(ep)
	span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
		attribute.String("ghx.eval.reward.component", "memory"),
		attribute.String("ghx.eval.penalty.kind", "repeat_read_ratio"),
		attribute.Int("ghx.eval.repeat_reads", repeats),
		attribute.Int("ghx.eval.later_reads", reads),
		attribute.Float64("ghx.eval.penalty.ratio", safeRatio(repeats, reads)),
		attribute.Bool("ghx.eval.penalty.applied", repeats > 0),
	))
}

func addSafetyEvents(span trace.Span, ep *Episode) {
	if len(ep.Violations) == 0 {
		span.AddEvent(ghxRewardCheckEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "safety"),
			attribute.String("ghx.eval.check.kind", "no_violations"),
			attribute.Bool("ghx.eval.check.matched", true),
		))
		return
	}
	for i, violation := range ep.Violations {
		span.AddEvent(ghxRewardPenaltyEvent, trace.WithAttributes(
			attribute.String("ghx.eval.reward.component", "safety"),
			attribute.String("ghx.eval.penalty.kind", "safety_violation"),
			attribute.Int("ghx.eval.violation.index", i),
			attribute.String("ghx.eval.violation", boundedString(violation, 512)),
			attribute.Bool("ghx.eval.penalty.applied", true),
		))
	}
}

func safeRatio(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d)
}

func rewardExplanationSummary(ep *Episode) string {
	return fmt.Sprintf("correctness=%.3f evidence=%.3f trajectory=%.3f compression=%.3f memory=%.3f safety=%.3f overall=%.3f",
		ep.Rewards.Correctness, ep.Rewards.Evidence, ep.Rewards.Trajectory, ep.Rewards.Compression, ep.Rewards.Memory, ep.Rewards.Safety, ep.Rewards.Overall)
}
