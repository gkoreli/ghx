package evals

import (
	"math"
	"testing"
)

func TestEpisodeSignalPerTokenComputesADRLevels(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Rewards: RewardBreakdown{Correctness: 0.8, Evidence: 0.5},
		Context: ContextAccounting{
			MainAgentChars:       800,
			SidecarInternalChars: 4000,
			TotalWorkflowChars:   4800,
		},
	}

	got := EpisodeSignalPerToken(ep)
	if !almostEqual(got.Signal, 0.4) {
		t.Fatalf("signal = %v, want 0.4", got.Signal)
	}
	assertSPT(t, "main", got.MainAgentSPT, true, 2.0)
	assertSPT(t, "sidecar internal", got.SidecarInternalSPT, true, 0.4)
	assertSPT(t, "workflow", got.WorkflowSPT, true, 1.0/3.0)
}

func TestEpisodeSignalPerTokenDirectSidecarInternalUndefined(t *testing.T) {
	ep := &Episode{
		Profile: ProfileGhx,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
		Context: ContextAccounting{
			MainAgentChars:       1000,
			SidecarInternalChars: 1000,
			TotalWorkflowChars:   1000,
		},
	}

	got := EpisodeSignalPerToken(ep)
	assertSPT(t, "main", got.MainAgentSPT, true, 4.0)
	assertSPT(t, "sidecar internal", got.SidecarInternalSPT, false, 0)
	assertSPT(t, "workflow", got.WorkflowSPT, true, 4.0)
}

func TestEpisodeSignalPerTokenZeroCharsUndefined(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
		Context: ContextAccounting{},
	}

	got := EpisodeSignalPerToken(ep)
	assertSPT(t, "main", got.MainAgentSPT, false, 0)
	assertSPT(t, "sidecar internal", got.SidecarInternalSPT, false, 0)
	assertSPT(t, "workflow", got.WorkflowSPT, false, 0)
}

func TestAggregateSignalPerTokenUsesSummedSignalAndChars(t *testing.T) {
	eps := []*Episode{
		{
			Profile: ProfileSidecar,
			Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
			Context: ContextAccounting{MainAgentChars: 1000, SidecarInternalChars: 3000, TotalWorkflowChars: 4000},
		},
		{
			Profile: ProfileSidecar,
			Rewards: RewardBreakdown{Correctness: 0.5, Evidence: 0.4},
			Context: ContextAccounting{MainAgentChars: 500, SidecarInternalChars: 500, TotalWorkflowChars: 1000},
		},
		{
			Profile: ProfileGhx,
			Rewards: RewardBreakdown{Correctness: 0.5, Evidence: 1},
			Context: ContextAccounting{MainAgentChars: 2000, TotalWorkflowChars: 2000},
		},
	}

	got := AggregateSignalPerToken(eps)
	sc := got[ProfileSidecar]
	if sc.Episodes != 2 {
		t.Fatalf("sidecar episodes = %d, want 2", sc.Episodes)
	}
	if !almostEqual(sc.MeanSignal, 0.6) {
		t.Fatalf("sidecar mean signal = %v, want 0.6", sc.MeanSignal)
	}
	assertSPT(t, "sidecar main", sc.MainAgentSPT, true, 3.2)
	assertSPT(t, "sidecar internal", sc.SidecarInternalSPT, true, 1.3714285714285714)
	assertSPT(t, "sidecar workflow", sc.WorkflowSPT, true, 0.96)

	gx := got[ProfileGhx]
	if gx.Episodes != 1 {
		t.Fatalf("ghx episodes = %d, want 1", gx.Episodes)
	}
	if !almostEqual(gx.MeanSignal, 0.5) {
		t.Fatalf("ghx mean signal = %v, want 0.5", gx.MeanSignal)
	}
	assertSPT(t, "ghx main", gx.MainAgentSPT, true, 1.0)
	assertSPT(t, "ghx internal", gx.SidecarInternalSPT, false, 0)
	assertSPT(t, "ghx workflow", gx.WorkflowSPT, true, 1.0)
}

func TestAggregateSignalPerTokenExcludesInvalidEpisodesLikeGates(t *testing.T) {
	valid := &Episode{
		Profile: ProfilePlain,
		Rewards: RewardBreakdown{Correctness: 0.5, Evidence: 0.5},
		Context: ContextAccounting{MainAgentChars: 1000, TotalWorkflowChars: 1000},
		Turns: []TurnRecord{{
			ToolCalls: []string{"gh api repos/o/r/contents/src/a.ts"},
		}},
	}
	explicitInvalid := &Episode{
		Profile: ProfilePlain,
		Invalid: true,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
		Context: ContextAccounting{MainAgentChars: 1000, TotalWorkflowChars: 1000},
	}
	plainGhx := &Episode{
		Profile: ProfilePlain,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
		Context: ContextAccounting{MainAgentChars: 1000, TotalWorkflowChars: 1000},
		Actions: []Action{{Input: "ghx read o/r src/a.ts"}},
	}

	got := AggregateSignalPerToken([]*Episode{valid, explicitInvalid, plainGhx, nil})
	plain := got[ProfilePlain]
	if plain.Episodes != 1 {
		t.Fatalf("plain episodes = %d, want 1", plain.Episodes)
	}
	if !plainGhx.Invalid {
		t.Fatal("plain ghx episode should be marked invalid by shared gate validation")
	}
	if !almostEqual(plain.MeanSignal, 0.25) {
		t.Fatalf("plain mean signal = %v, want 0.25", plain.MeanSignal)
	}
	assertSPT(t, "plain main", plain.MainAgentSPT, true, 1.0)
}

func assertSPT(t *testing.T, name string, got SPTValue, defined bool, want float64) {
	t.Helper()
	if got.Defined != defined {
		t.Fatalf("%s defined = %v, want %v", name, got.Defined, defined)
	}
	if !defined {
		return
	}
	if !almostEqual(got.Value, want) {
		t.Fatalf("%s value = %v, want %v", name, got.Value, want)
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
