package evals

import (
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func sampleTask() Task {
	return Task{
		ID:   "t1",
		Repo: "honojs/hono",
		Turns: []string{
			"Where is middleware composition implemented?",
			"How do errors propagate?",
		},
		Checks: TaskChecks{
			ExpectedFiles:      []string{"src/compose.ts", "src/hono-base.ts"},
			ExpectedSymbols:    []string{"compose", "dispatch"},
			UnacceptableClaims: []string{"middleware lives in src/router.ts"},
			AvoidPaths:         []string{".test.ts"},
		},
	}
}

func sidecarEpisode() *Episode {
	report := &sidecar.Report{
		Answer: "Middleware composition is implemented in src/compose.ts; dispatch drives the chain.",
		Verified: []sidecar.Claim{
			{Summary: "compose() dispatches handlers", Evidence: "src/compose.ts: dispatch(i)"},
		},
		RelevantFiles: []sidecar.RelevantFile{
			{Path: "src/compose.ts", Reason: "defines compose()"},
			{Path: "src/hono-base.ts", Reason: "calls compose with error handler"},
		},
		CommandsRun: []string{
			"ghx tree honojs/hono src --depth 2",
			"ghx read honojs/hono src/compose.ts --map",
		},
	}
	return &Episode{
		TaskID:  "t1",
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{Turn: 0, Text: "exploring...", ToolCalls: []string{"ghx tree honojs/hono src --depth 2 (completed)", "ghx read honojs/hono src/compose.ts --map (completed)"}, Report: report},
			{Turn: 1, Text: "errors propagate via onError", ToolCalls: []string{"ghx read honojs/hono src/hono-base.ts --grep onError (completed)"}, Resumed: true, Report: report},
		},
		Report: report,
		Context: ContextAccounting{
			MainAgentChars:       500,
			SidecarInternalChars: 4500,
			TotalWorkflowChars:   5000,
		},
	}
}

func TestComputeRewardsSidecarEpisode(t *testing.T) {
	task := sampleTask()
	ep := sidecarEpisode()
	r := ComputeRewards(task, ep)

	if r.Correctness != 1.0 {
		t.Errorf("correctness = %v, want 1.0 (both files + both symbols found)", r.Correctness)
	}
	if r.Evidence != 1.0 {
		t.Errorf("evidence = %v, want 1.0 (all claims evidenced, commands present, reasons present)", r.Evidence)
	}
	if r.Compression != 0.9 {
		t.Errorf("compression = %v, want 0.9 (500/5000 main)", r.Compression)
	}
	if !r.MemoryApplies {
		t.Error("memory should apply to a 2-turn task")
	}
	if r.Memory != 1.0 {
		t.Errorf("memory = %v, want 1.0 (resumed, no repeat reads)", r.Memory)
	}
	if r.Safety != 1.0 {
		t.Errorf("safety = %v, want 1.0", r.Safety)
	}
	if r.Overall <= 0 || r.Overall > 1 {
		t.Errorf("overall = %v out of range", r.Overall)
	}
}

func TestCorrectnessZeroOnUnacceptableClaim(t *testing.T) {
	task := sampleTask()
	ep := sidecarEpisode()
	ep.Turns[0].Text = "I believe middleware lives in src/router.ts actually"
	if got := correctnessReward(task, ep); got != 0 {
		t.Errorf("correctness = %v, want 0 when unacceptable claim present", got)
	}
}

func TestCorrectnessPartial(t *testing.T) {
	task := sampleTask()
	ep := &Episode{
		Profile: ProfilePlain,
		Turns: []TurnRecord{
			// Mentions one of two files, one of two symbols.
			{Turn: 0, Text: "The composition happens in src/compose.ts via compose()."},
		},
	}
	got := correctnessReward(task, ep)
	want := (0.5 + 0.5) / 2
	if got != want {
		t.Errorf("correctness = %v, want %v", got, want)
	}
}

func TestEvidenceWithoutReportUsesFallback(t *testing.T) {
	ep := &Episode{
		Profile: ProfilePlain,
		Turns: []TurnRecord{
			{Turn: 0, Text: "found src/compose.ts", ToolCalls: []string{"gh api repos/honojs/hono (completed)"}},
		},
	}
	if got := evidenceReward(ep); got != 1.0 {
		t.Errorf("evidence = %v, want 1.0 (tools used + file path in answer)", got)
	}

	noTools := &Episode{Profile: ProfilePlain, Turns: []TurnRecord{{Turn: 0, Text: "probably in the src folder"}}}
	if got := evidenceReward(noTools); got != 0 {
		t.Errorf("evidence = %v, want 0 (no tools, no concrete paths)", got)
	}
}

func TestTrajectoryPenalizesDuplicatesAndAvoidPaths(t *testing.T) {
	task := sampleTask()
	clean := &Episode{Turns: []TurnRecord{{ToolCalls: []string{"a", "b", "c"}}}}
	if got := trajectoryReward(task, clean); got != 1.0 {
		t.Errorf("clean trajectory = %v, want 1.0", got)
	}

	dup := &Episode{Turns: []TurnRecord{{ToolCalls: []string{"a", "a", "a", "b"}}}}
	if got := trajectoryReward(task, dup); got >= 1.0 {
		t.Errorf("duplicate trajectory = %v, want < 1.0", got)
	}

	avoid := &Episode{Turns: []TurnRecord{{ToolCalls: []string{"ghx read honojs/hono src/index.test.ts"}}}}
	if got := trajectoryReward(task, avoid); got >= 1.0 {
		t.Errorf("avoid-path trajectory = %v, want < 1.0", got)
	}
}

func TestTrajectoryOverBudget(t *testing.T) {
	task := sampleTask()
	task.Turns = task.Turns[:1] // single turn → budget 8
	var calls []string
	for i := 0; i < 16; i++ {
		calls = append(calls, string(rune('a'+i)))
	}
	ep := &Episode{Turns: []TurnRecord{{ToolCalls: calls}}}
	got := trajectoryReward(task, ep)
	// 16 commands vs budget 8: overRatio 0.5, no dups, no avoid → 1 - 0.5/3.
	want := 1 - 0.5/3
	if diff := got - want; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("over-budget trajectory = %v, want %v", got, want)
	}
}

func TestTrajectoryFallsBackToReportCommands(t *testing.T) {
	task := sampleTask()
	ep := &Episode{
		Turns:  []TurnRecord{{Turn: 0}},
		Report: &sidecar.Report{Answer: "x", CommandsRun: []string{"ghx explore honojs/hono"}},
	}
	if got := trajectoryReward(task, ep); got != 1.0 {
		t.Errorf("trajectory = %v, want 1.0 via commandsRun fallback", got)
	}
}

func TestCompressionDirectProfileIsZero(t *testing.T) {
	ep := &Episode{
		Profile: ProfileGhx,
		Context: ContextAccounting{MainAgentChars: 5000, TotalWorkflowChars: 5000},
	}
	if got := compressionReward(ep); got != 0 {
		t.Errorf("compression = %v, want 0 for direct profile", got)
	}
}

func TestMemoryZeroWhenFollowupNotResumed(t *testing.T) {
	ep := sidecarEpisode()
	ep.Turns[1].Resumed = false
	if got := memoryReward(ep); got != 0 {
		t.Errorf("memory = %v, want 0 when follow-up did not resume", got)
	}
}

func TestMemoryPenalizesRepeatReads(t *testing.T) {
	ep := sidecarEpisode()
	// Follow-up re-reads the exact file from turn 0.
	ep.Turns[1].ToolCalls = []string{"ghx read honojs/hono src/compose.ts (completed)"}
	got := memoryReward(ep)
	if got != 0.0 {
		t.Errorf("memory = %v, want 0.0 (single follow-up read is a repeat)", got)
	}
}

func TestSafetyZeroOnViolation(t *testing.T) {
	ep := sidecarEpisode()
	ep.Violations = []string{"write attempted: /tmp/x"}
	if got := safetyReward(ep); got != 0 {
		t.Errorf("safety = %v, want 0", got)
	}
}

func TestMemoryNotApplicableSingleTurn(t *testing.T) {
	task := sampleTask()
	task.Turns = task.Turns[:1]
	ep := sidecarEpisode()
	ep.Turns = ep.Turns[:1]
	r := ComputeRewards(task, ep)
	if r.MemoryApplies {
		t.Error("memory must not apply to single-turn tasks")
	}
}
