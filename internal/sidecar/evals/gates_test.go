package evals

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// mkEpisode builds a synthetic scored episode for gate tests. Tool-output
// chars and a sidecar report are populated so a healthy synthetic episode
// does not trip the ADR-0016.2 data-quality warnings.
func mkEpisode(p Profile, corr, evid, safety float64, mainChars, totalChars int, multiTurn, resumed bool) *Episode {
	command := "ghx read o/r src/a.ts (completed)"
	if p == ProfilePlain {
		command = "gh api repos/o/r/contents/src/a.ts (completed)"
	}
	ep := &Episode{
		TaskID:  "task-a",
		Profile: p,
		Rewards: RewardBreakdown{Correctness: corr, Evidence: evid, Safety: safety},
		Context: ContextAccounting{MainAgentChars: mainChars, TotalWorkflowChars: totalChars},
		Turns: []TurnRecord{{
			Turn:            0,
			ToolCalls:       []string{command},
			ToolOutputChars: 500,
		}},
	}
	if p == ProfileSidecar {
		ep.Report = &sidecar.Report{Answer: "implemented in src/a.ts"}
	}
	if multiTurn {
		followupCommand := "ghx read o/r src/b.ts (completed)"
		if p == ProfilePlain {
			followupCommand = "gh api repos/o/r/contents/src/b.ts (completed)"
		}
		ep.Turns = append(ep.Turns, TurnRecord{
			Turn:            1,
			Resumed:         resumed,
			ToolCalls:       []string{followupCommand},
			ToolOutputChars: 500,
		})
	}
	return ep
}

func passingEpisodes() []*Episode {
	var eps []*Episode
	for i := 0; i < 5; i++ {
		eps = append(eps,
			mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 800, 9000, true, true),
			mkEpisode(ProfileGhx, 0.85, 0.6, 1.0, 9000, 9000, true, true),
			mkEpisode(ProfilePlain, 0.5, 0.4, 1.0, 12000, 12000, true, true),
		)
	}
	return eps
}

func TestEvaluateGatesAllPass(t *testing.T) {
	v := EvaluateGates(passingEpisodes())
	for _, g := range v.Gates {
		if !g.Pass {
			t.Errorf("%s failed unexpectedly: %s", g.ID, g.Detail)
		}
	}
	if !v.ThesisSupported {
		t.Error("thesis should be supported when all gates pass")
	}
}

func TestG1FailsOnLowCorrectness(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar {
			ep.Rewards.Correctness = 0.5 // below abs floor 0.60
		}
	}
	v := EvaluateGates(eps)
	if v.Gates[0].Pass {
		t.Errorf("G1 should fail: %s", v.Gates[0].Detail)
	}
	if v.ThesisSupported {
		t.Error("thesis must not be supported when G1 fails")
	}
}

func TestG3FailsWhenReportTooLarge(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar {
			ep.Context.MainAgentChars = 5000 // > 0.35 × 9000
		}
	}
	v := EvaluateGates(eps)
	if v.Gates[2].Pass {
		t.Errorf("G3 should fail: %s", v.Gates[2].Detail)
	}
	if v.ThesisSupported {
		t.Error("thesis must not be supported when G3 fails")
	}
}

func TestG4FailureDoesNotOverturnThesis(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar && len(ep.Turns) > 1 {
			ep.Turns[1].Resumed = false
		}
	}
	v := EvaluateGates(eps)
	if v.Gates[3].Pass {
		t.Errorf("G4 should fail: %s", v.Gates[3].Detail)
	}
	if !v.ThesisSupported {
		t.Error("G4 failure must not overturn the thesis (ADR-0016.1 verdict rules)")
	}
	found := false
	for _, n := range v.Notes {
		if strings.Contains(n, "ADR-0017") {
			found = true
		}
	}
	if !found {
		t.Error("G4 failure must note that ADR-0017 is blocked")
	}
}

func TestG5FailsOnSafetyViolation(t *testing.T) {
	eps := passingEpisodes()
	eps[0].Rewards.Safety = 0 // one sidecar episode violated
	v := EvaluateGates(eps)
	if v.Gates[4].Pass {
		t.Errorf("G5 should fail: %s", v.Gates[4].Detail)
	}
	if v.ThesisSupported {
		t.Error("thesis must not be supported when G5 fails")
	}
}

func TestEvaluateGatesInsufficientData(t *testing.T) {
	v := EvaluateGates(nil)
	if v.ThesisSupported {
		t.Error("empty run must not support the thesis")
	}
	if len(v.Notes) == 0 || !strings.Contains(v.Notes[0], "INSUFFICIENT DATA") {
		t.Errorf("expected insufficient-data note, got %v", v.Notes)
	}
}

// gateRunEpisodes builds a full pre-registered sample: minGateRunTasks tasks
// × minGateRunTrials trials × all profiles, first two tasks multi-turn.
func gateRunEpisodes() []*Episode {
	var eps []*Episode
	for task := 0; task < minGateRunTasks; task++ {
		multiTurn := task < minGateRunMultiTurnTasks
		for trial := 0; trial < minGateRunTrials; trial++ {
			for _, e := range []*Episode{
				mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 800, 9000, multiTurn, true),
				mkEpisode(ProfileGhx, 0.85, 0.6, 1.0, 9000, 9000, multiTurn, true),
				mkEpisode(ProfilePlain, 0.5, 0.4, 1.0, 12000, 12000, multiTurn, true),
			} {
				e.TaskID = fmt.Sprintf("task-%d", task)
				eps = append(eps, e)
			}
		}
	}
	return eps
}

func TestVerdictPreliminaryBelowSampleMinimum(t *testing.T) {
	v := EvaluateGates(passingEpisodes()) // one task, five trials
	if v.DataSufficient {
		t.Error("a single-task run must not be data-sufficient")
	}
	if !v.ThesisSupported {
		t.Error("gate math itself should still pass on this sample")
	}
	md := FormatVerdict(v)
	if !strings.Contains(md, "PRELIMINARY") {
		t.Error("verdict markdown must be labeled PRELIMINARY below the gate-run sample")
	}
}

func TestVerdictSufficientAtGateRunSample(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes())
	if !v.DataSufficient {
		t.Errorf("full gate-run sample must be data-sufficient; notes: %v", v.Notes)
	}
	if strings.Contains(FormatVerdict(v), "PRELIMINARY") {
		t.Error("a sufficient sample must not be labeled PRELIMINARY")
	}
}

func TestEvaluateGatesCarriesAndRendersAnomalies(t *testing.T) {
	eps := passingEpisodes()
	anomalous := mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 800, 9000, false, false)
	anomalous.ID = "excluded-sidecar-anomaly"
	anomalous.Invalid = true
	anomalous.ExclusionReasons = []string{"test exclusion"}
	anomalous.Turns[0].Report = &sidecar.Report{Answer: "BLOCKED: ghx is unavailable in this sidecar session."}
	eps = append(eps, anomalous)

	v := EvaluateGates(eps)
	if len(v.Anomalies) == 0 {
		t.Fatal("verdict missing anomaly counts")
	}
	blocked := mustVerdictAnomalyCount(t, v.Anomalies, AnomalySidecarBlocked)
	if blocked.Count != 1 || blocked.Episodes != 1 || blocked.Severity != SeverityBreaking {
		t.Fatalf("blocked anomaly count = %+v, want one breaking anomaly from excluded raw episode", blocked)
	}
	md := FormatVerdict(v)
	if !strings.Contains(md, "## Anomalies") {
		t.Fatalf("verdict markdown missing anomalies section:\n%s", md)
	}
	if !strings.Contains(md, "| sidecar_blocked_report | breaking | 1 | 1 |") {
		t.Fatalf("verdict markdown missing anomaly row:\n%s", md)
	}
	if strings.Index(md, "## Gates") > strings.Index(md, "## Anomalies") ||
		strings.Index(md, "## Anomalies") > strings.Index(md, "## Verdict") {
		t.Fatalf("anomalies section should render between Gates and Verdict:\n%s", md)
	}
}

func mustVerdictAnomalyCount(t *testing.T, counts []AnomalyCount, kind string) AnomalyCount {
	t.Helper()
	for _, c := range counts {
		if c.Kind == kind {
			return c
		}
	}
	t.Fatalf("missing anomaly count for %s in %#v", kind, counts)
	return AnomalyCount{}
}

func TestDataQualityNoteMissingToolOutputs(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		if ep.Profile == ProfileGhx {
			for i := range ep.Turns {
				ep.Turns[i].ToolOutputChars = 0
			}
		}
	}
	v := EvaluateGates(eps)
	if !hasNote(v, "zero tool-output") {
		t.Errorf("expected missing-tool-output data-quality note, got %v", v.Notes)
	}
}

func TestDataQualityNoteEmptySidecarReport(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar {
			ep.Report = nil
		}
	}
	v := EvaluateGates(eps)
	if !hasNote(v, "no/empty report") {
		t.Errorf("expected empty-report data-quality note, got %v", v.Notes)
	}
}

func TestDataQualityNoteUnbalancedProfiles(t *testing.T) {
	eps := passingEpisodes()
	eps = append(eps, mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 800, 9000, true, true))
	v := EvaluateGates(eps)
	if !hasNote(v, "unequal episode counts") {
		t.Errorf("expected unbalanced-profiles data-quality note, got %v", v.Notes)
	}
}

func TestDataQualityNoteCollapsedBaseline(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		if ep.Profile == ProfileGhx {
			ep.Rewards.Correctness = 0.3 // below g1AbsFloor
		}
	}
	v := EvaluateGates(eps)
	if !hasNote(v, "collapsed baseline") {
		t.Errorf("expected collapsed-baseline data-quality note, got %v", v.Notes)
	}
}

func TestPlainGhxInvocationInvalidAndExcluded(t *testing.T) {
	eps := passingEpisodes()
	plain := mkEpisode(ProfilePlain, 1.0, 1.0, 1.0, 100, 100, false, false)
	plain.ID = "plain-bad"
	plain.Actions = []Action{{Input: "ghx read owner/repo src/a.ts"}}
	eps = append(eps, plain)

	v := EvaluateGates(eps)
	if v.Valid {
		t.Fatal("verdict should be invalid when a plain episode invokes ghx")
	}
	if !plain.Invalid {
		t.Fatal("plain episode was not marked invalid")
	}
	if !hasNote(v, "plain profile invoked ghx") {
		t.Fatalf("expected compliance note, got %v", v.Notes)
	}
	if got := v.Aggregates[ProfilePlain].Episodes; got != 5 {
		t.Fatalf("plain aggregate episodes = %d, want 5 (bad episode excluded)", got)
	}
}

func TestPlainGhxInvocationDetectsNpxAndScopedPackage(t *testing.T) {
	for _, input := range []string{
		"ghx read o/r f",
		"foo | ghx read",
		"x; ghx tree o/r",
		"npx ghx read owner/repo src/a.ts",
		"npx -y @gkoreli/ghx explore o/r",
		"$(ghx search q)",
	} {
		t.Run(input, func(t *testing.T) {
			plain := mkEpisode(ProfilePlain, 1.0, 1.0, 1.0, 100, 100, false, false)
			plain.Actions = []Action{{Input: input}}
			if !invokesGhx(plain) {
				t.Fatalf("invokesGhx(%q) = false, want true", input)
			}
		})
	}
}

func TestPlainGhxInvocationIgnoresRepoPathArguments(t *testing.T) {
	for _, input := range []string{
		"gh api repos/gkoreli/ghx/git/trees/HEAD",
		"gh repo view gkoreli/ghx --json x",
		"echo ghx",
		"grep ghx file",
		`for path in internal/mapengine/parser.go internal/mapengine/parser_test.go; do
  gh api repos/gkoreli/ghx/contents/$path --jq '.content' | base64 -d
done`,
	} {
		t.Run(input, func(t *testing.T) {
			plain := mkEpisode(ProfilePlain, 1.0, 1.0, 1.0, 100, 100, false, false)
			plain.Actions = []Action{{Input: input}}
			if invokesGhx(plain) {
				t.Fatalf("invokesGhx(%q) = true, want false", input)
			}
		})
	}
}

func TestGhxProfileWithoutGhxInvocationFlaggedOnly(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		ep.Identity = AgentIdentity{}
	}
	ghx := mkEpisode(ProfileGhx, 1.0, 1.0, 1.0, 100, 100, false, false)
	ghx.ID = "ghx-no-ghx"
	ghx.Actions = []Action{{Input: "gh api repos/o/r"}}
	eps = append(eps, ghx)

	v := EvaluateGates(eps)
	if !v.Valid {
		t.Fatalf("ghx profile with zero ghx invocations should not invalidate verdict: %v", v.Notes)
	}
	if !hasNote(v, "zero ghx invocations") {
		t.Fatalf("expected ghx compliance note, got %v", v.Notes)
	}
	if got := v.Aggregates[ProfileGhx].Episodes; got != 6 {
		t.Fatalf("ghx aggregate episodes = %d, want 6", got)
	}
}

func TestMixedAgentIdentityInvalidatesVerdict(t *testing.T) {
	eps := passingEpisodes()
	for i, ep := range eps {
		ep.Identity = AgentIdentity{AgentCommand: "agent", AdapterName: "mock", AdapterVersion: "1", SubjectModel: "sonnet"}
		if i == 0 {
			ep.Identity.SubjectModel = "different"
		}
	}
	v := EvaluateGates(eps)
	if v.Valid {
		t.Fatal("mixed identities should invalidate verdict")
	}
	if !hasNote(v, "mixed agent identities") {
		t.Fatalf("expected identity note, got %v", v.Notes)
	}
}

func TestUnknownSubjectModelAddsVerdictNote(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		ep.Identity = AgentIdentity{AgentCommand: "agent", AdapterName: "mock", AdapterVersion: "1", SubjectModel: "unknown"}
	}
	v := EvaluateGates(eps)
	if !hasNote(v, "subject-model identity is unverified") {
		t.Fatalf("expected unverified subject-model note, got %v", v.Notes)
	}
}

func hasNote(v Verdict, substr string) bool {
	for _, n := range v.Notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}

func TestSaveVerdictAndLoadRunEpisodes(t *testing.T) {
	dir := t.TempDir()
	eps := passingEpisodes()
	for i, ep := range eps {
		ep.ID = ep.ID + string(rune('a'+i))
		if _, err := SaveEpisode(dir, ep); err != nil {
			t.Fatal(err)
		}
	}
	v := EvaluateGates(eps)
	mdPath, err := SaveVerdict(dir, v)
	if err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(md), "THESIS SUPPORTED") {
		t.Error("verdict markdown missing thesis line")
	}

	// verdict.json must not be loaded back as an episode.
	loaded, err := LoadRunEpisodes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != len(eps) {
		t.Errorf("loaded %d episodes, want %d (verdict files must be excluded)", len(loaded), len(eps))
	}
}
