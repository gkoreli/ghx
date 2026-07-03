package evals

import (
	"os"
	"strings"
	"testing"
)

// mkEpisode builds a synthetic scored episode for gate tests.
func mkEpisode(p Profile, corr, evid, safety float64, mainChars, totalChars int, multiTurn, resumed bool) *Episode {
	ep := &Episode{
		Profile: p,
		Rewards: RewardBreakdown{Correctness: corr, Evidence: evid, Safety: safety},
		Context: ContextAccounting{MainAgentChars: mainChars, TotalWorkflowChars: totalChars},
		Turns:   []TurnRecord{{Turn: 0, ToolCalls: []string{"ghx read o/r src/a.ts (completed)"}}},
	}
	if multiTurn {
		ep.Turns = append(ep.Turns, TurnRecord{
			Turn:      1,
			Resumed:   resumed,
			ToolCalls: []string{"ghx read o/r src/b.ts (completed)"},
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
