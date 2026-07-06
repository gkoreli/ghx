package evals

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// ── ADR-0016.8 D1: union-of-all-turn-reports scoring ─────────────────────────

// TestUnionScoringFoldsDeltaReports pins the instrument fix: a multi-turn
// sidecar that delta-reports (final turn carries only what is new) must be
// scored on the union of all turn reports, not the final report alone.
func TestUnionScoringFoldsDeltaReports(t *testing.T) {
	turn0Report := &sidecar.Report{
		Answer: "Routes are registered in routergroup.go.",
		Verified: []sidecar.Claim{
			{Summary: "handle() registers routes", Evidence: "routergroup.go:72"},
		},
		RelevantFiles: []sidecar.RelevantFile{
			{Path: "routergroup.go", Reason: "route registration"},
		},
		CommandsRun: []string{"ghx read gin-gonic/gin routergroup.go"},
	}
	finalReport := &sidecar.Report{ // delta: only what is new on turn 1
		Answer: "Matching happens in tree.go.",
		RelevantFiles: []sidecar.RelevantFile{
			{Path: "tree.go", Reason: "radix tree matching"},
		},
	}
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{Turn: 0, Report: turn0Report},
			{Turn: 1, Report: finalReport},
		},
		Report: finalReport,
	}
	task := Task{
		ID: "t", Repo: "gin-gonic/gin", Turns: []string{"q1", "q2"},
		Checks: TaskChecks{ExpectedFiles: []string{"routergroup.go", "tree.go"}},
	}

	if got := correctnessReward(task, ep); got != 1.0 {
		t.Fatalf("correctness = %v, want 1.0 (union must see turn-0 files)", got)
	}
	// Union evidence: verified 1/1 cited, commands present, files 2/2 reasoned.
	if got := evidenceReward(ep); got != 1.0 {
		t.Fatalf("evidence = %v, want 1.0 (union must see turn-0 claims and commands)", got)
	}

	// Final-report-only view (the old defect) would have scored evidence 0 on
	// verified/commands; prove the union actually changed the outcome.
	only := &Episode{Profile: ProfileSidecar, Turns: []TurnRecord{{Turn: 0, Report: finalReport}}, Report: finalReport}
	if got := evidenceReward(only); got == 1.0 {
		t.Fatalf("final-only evidence = %v; fixture does not demonstrate the delta-report gap", got)
	}
}

// TestUnionScoringDedupesAcrossTurns: repeating the same claims/files every
// turn must not be rewarded — duplicates dedupe by content.
func TestUnionScoringDedupesAcrossTurns(t *testing.T) {
	repeated := sidecar.Claim{Summary: "compose dispatches middleware", Evidence: "src/compose.ts:32"}
	turn0 := &sidecar.Report{
		Answer:      "Middleware composes in compose.ts.",
		Verified:    []sidecar.Claim{repeated},
		CommandsRun: []string{"ghx read honojs/hono src/compose.ts"},
	}
	turn1 := &sidecar.Report{
		Answer: "Middleware composes in compose.ts.", // repeated answer
		Verified: []sidecar.Claim{
			repeated, // duplicate across turns → deduped
			{Summary: "onError catches throws"}, // new, no evidence
		},
		CommandsRun: []string{"ghx read honojs/hono src/compose.ts"}, // duplicate
	}
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns:   []TurnRecord{{Turn: 0, Report: turn0}, {Turn: 1, Report: turn1}},
		Report:  turn1,
	}

	rep := scoringReport(ep)
	if len(rep.Verified) != 2 {
		t.Fatalf("union verified = %d claims, want 2 (duplicate deduped)", len(rep.Verified))
	}
	if len(rep.CommandsRun) != 1 {
		t.Fatalf("union commandsRun = %d, want 1 (duplicate deduped)", len(rep.CommandsRun))
	}
	if rep.Answer != "Middleware composes in compose.ts." {
		t.Fatalf("union answer should dedupe repeated text, got %q", rep.Answer)
	}
	// Evidence fraction over the deduped union: 1 of 2 verified cites code.
	want := (0.5 + 1.0 + 0.0) / 3 // verified 1/2, commands 1, no relevant files
	if got := evidenceReward(ep); math.Abs(got-want) > 1e-9 {
		t.Fatalf("evidence = %v, want %v over the deduped union", got, want)
	}
}

// TestUnionScoringSingleTurnUnchanged: a union of one report is that report
// verbatim — even within-report duplicates keep their original fractions, so
// single-turn scoring is bit-identical to the pre-D1 scorer.
func TestUnionScoringSingleTurnUnchanged(t *testing.T) {
	rep := &sidecar.Report{
		Answer: "answer",
		Verified: []sidecar.Claim{
			{Summary: "a", Evidence: "src/a.ts:1"},
			{Summary: "a", Evidence: "src/a.ts:1"}, // within-report duplicate
			{Summary: "b"},
		},
		CommandsRun: []string{"ghx read o/r src/a.ts"},
	}
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns:   []TurnRecord{{Turn: 0, Report: rep}},
		Report:  rep,
	}
	if got := scoringReport(ep); got != rep {
		t.Fatalf("single-turn scoring report must be the original report, got %+v", got)
	}
	want := (2.0/3.0 + 1.0 + 0.0) / 3 // verified 2/3 (duplicates preserved), commands, no files
	if got := evidenceReward(ep); math.Abs(got-want) > 1e-9 {
		t.Fatalf("single-turn evidence = %v, want %v (unchanged)", got, want)
	}
	if scoringReport(&Episode{Profile: ProfilePlain, Turns: []TurnRecord{{Turn: 0, Text: "no report"}}}) != nil {
		t.Fatal("report-less episodes must keep a nil scoring report (direct fallback)")
	}
}

// ── ADR-0016.8 D7: unacceptable-claim exception contexts ─────────────────────

func TestUnacceptableClaimExceptionSuppressesZeroing(t *testing.T) {
	task := Task{
		ID: "t", Repo: "expressjs/express", Turns: []string{"q"},
		Checks: TaskChecks{
			ExpectedFiles:               []string{"lib/application.js"},
			UnacceptableClaims:          []string{"lib/router/index.js"},
			UnacceptableClaimExceptions: []string{"moved from", "express 4"},
		},
	}
	mk := func(text string) *Episode {
		return &Episode{Profile: ProfilePlain, Turns: []TurnRecord{{Turn: 0, Text: text}}}
	}

	honest := mk("Routing is wired in lib/application.js; the router moved from lib/router/index.js to the external router package.")
	if got := correctnessReward(task, honest); got != 1.0 {
		t.Fatalf("correctness = %v, want 1.0 — historical mention must not zero (ADR-0016.8 D7)", got)
	}
	historical := mk("lib/application.js wires routes; Express 4.x had the router inlined at `lib/router/index.js`.")
	if got := correctnessReward(task, historical); got != 1.0 {
		t.Fatalf("correctness = %v, want 1.0 — committed false-zero context must be excused", got)
	}
	wrong := mk("The router is implemented in lib/router/index.js next to lib/application.js.")
	if got := correctnessReward(task, wrong); got != 0.0 {
		t.Fatalf("correctness = %v, want 0 — bare assertion still zeroes", got)
	}
	// One excused occurrence + one bare assertion → still zero.
	mixed := mk("It moved from lib/router/index.js. See lib/application.js. The router lives in lib/router/index.js today.")
	if got := correctnessReward(task, mixed); got != 0.0 {
		t.Fatalf("correctness = %v, want 0 — any unexcused occurrence zeroes", got)
	}
	// Exception outside the pre-registered window does not excuse.
	far := mk("moved from " + string(make([]byte, unacceptableClaimExceptionWindow)) + "lib/router/index.js and lib/application.js")
	if got := correctnessReward(task, far); got != 0.0 {
		t.Fatalf("correctness = %v, want 0 — exception outside the %d-char window", got, unacceptableClaimExceptionWindow)
	}

	// Without exceptions, behavior is exactly the pre-D7 zeroing.
	task.Checks.UnacceptableClaimExceptions = nil
	if got := correctnessReward(task, honest); got != 0.0 {
		t.Fatalf("correctness = %v, want 0 when no exceptions are registered", got)
	}
}

func TestTaskValidateExceptionRules(t *testing.T) {
	base := Task{
		ID: "t", Repo: "o/r", Turns: []string{"where is routing?"},
		Checks: TaskChecks{ExpectedFiles: []string{"a.go"}},
	}

	blankExc := base
	blankExc.Checks.UnacceptableClaims = []string{"bad.go"}
	blankExc.Checks.UnacceptableClaimExceptions = []string{" "}
	if err := blankExc.Validate(); err == nil {
		t.Fatal("blank unacceptableClaimExceptions entry must fail validation")
	}
	orphan := base
	orphan.Checks.UnacceptableClaimExceptions = []string{"moved from"}
	if err := orphan.Validate(); err == nil {
		t.Fatal("exceptions without unacceptableClaims must fail validation")
	}
	blankContam := base
	blankContam.Checks.ContaminationPaths = []string{""}
	if err := blankContam.Validate(); err == nil {
		t.Fatal("blank contaminationPaths entry must fail validation")
	}
	ok := base
	ok.Checks.UnacceptableClaims = []string{"bad.go"}
	ok.Checks.UnacceptableClaimExceptions = []string{"moved from"}
	ok.Checks.ContaminationPaths = []string{"docs/adr/"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid task rejected: %v", err)
	}
}

// ── ADR-0016.8 golden regression ─────────────────────────────────────────────

// TestUnionScoringGoldenPartialRun rescopes the committed 2026-07-05 partial
// run (docs/evals/gate-run-2026-07-05-d2-0021-0022-partial) with the union
// scorer and pins the INSIGHTS.md P2 projection as a regression fixture:
// +0.278 correctness / +0.226 evidence on the six multi-turn sidecar
// episodes, per-episode values matching the P2 table within rounding. The
// committed verdict itself is never rescored (measurement stack frozen);
// this test makes the projection reproducible forever.
func TestUnionScoringGoldenPartialRun(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "docs", "evals", "gate-run-2026-07-05-d2-0021-0022-partial")
	eps, err := LoadRunEpisodes(dir)
	if err != nil {
		t.Fatalf("load committed partial run: %v", err)
	}

	// INSIGHTS.md P2 table (union columns), 2-decimal values.
	want := map[string]struct{ corr, evid float64 }{
		"gin-routing_ghx-sidecar_1783309817215":     {1.00, 0.97},
		"gin-routing_ghx-sidecar_1783311354381":     {1.00, 0.67},
		"gin-routing_ghx-sidecar_1783313384747":     {1.00, 0.67},
		"hono-middleware_ghx-sidecar_1783310129190": {1.00, 1.00},
		"hono-middleware_ghx-sidecar_1783311690396": {1.00, 1.00},
		"hono-middleware_ghx-sidecar_1783313720066": {1.00, 1.00},
	}

	var dCorr, dEvid float64
	n := 0
	for _, ep := range eps {
		if ep.Profile != ProfileSidecar || len(ep.Turns) < 2 {
			continue
		}
		exp, ok := want[ep.ID]
		if !ok {
			t.Fatalf("unexpected multi-turn sidecar episode %s not in the INSIGHTS P2 table", ep.ID)
		}
		task := Task{ID: ep.TaskID, Repo: ep.Repo, Turns: make([]string, len(ep.Turns)), Checks: ep.Checks}
		corr := correctnessReward(task, ep)
		evid := evidenceReward(ep)
		if math.Abs(corr-exp.corr) > 0.005 {
			t.Errorf("%s union correctness = %.4f, want %.2f (INSIGHTS P2)", ep.ID, corr, exp.corr)
		}
		if math.Abs(evid-exp.evid) > 0.005 {
			t.Errorf("%s union evidence = %.4f, want %.2f (INSIGHTS P2)", ep.ID, evid, exp.evid)
		}
		dCorr += corr - ep.Rewards.Correctness
		dEvid += evid - ep.Rewards.Evidence
		n++
	}
	if n != 6 {
		t.Fatalf("multi-turn sidecar episodes = %d, want 6", n)
	}
	dCorr /= float64(n)
	dEvid /= float64(n)
	// Aggregate deltas vs the committed (final-report-only) rewards.
	if math.Abs(dCorr-0.278) > 0.001 {
		t.Errorf("aggregate Δcorrectness = %.4f, want +0.278 (INSIGHTS P2)", dCorr)
	}
	if math.Abs(dEvid-0.226) > 0.001 {
		t.Errorf("aggregate Δevidence = %.4f, want +0.226 (INSIGHTS P2)", dEvid)
	}
}
