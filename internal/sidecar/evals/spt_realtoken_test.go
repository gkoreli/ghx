package evals

import (
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// turnWithUsage builds a TurnRecord carrying provider-reported usage
// (ADR-0016.10 D6 persistence shape).
func turnWithUsage(in, out, cacheRead, cacheCreate int, cost float64, source string) TurnRecord {
	return TurnRecord{
		RawSDK: &sidecar.RawSDKAudit{
			Messages: 1,
			Usage: &sidecar.RawSDKUsage{
				InputTokens:              in,
				OutputTokens:             out,
				CacheReadInputTokens:     cacheRead,
				CacheCreationInputTokens: cacheCreate,
				CostUSD:                  cost,
				Source:                   source,
			},
		},
	}
}

// TestAggregateRealTokenSPTParityFixture pins the ADR-0016.11 D1 math:
// known usage → known session SPT, summing signal and all four usage
// components before dividing (never averaging per-episode ratios).
func TestAggregateRealTokenSPTParityFixture(t *testing.T) {
	eps := []*Episode{
		{
			Profile: ProfileSidecar,
			Rewards: RewardBreakdown{Correctness: 1, Evidence: 0.5},
			Turns: []TurnRecord{
				// 100 + 400 + 1000 + 500 = 2000 tokens
				turnWithUsage(100, 400, 1000, 500, 0.10, "result"),
			},
		},
		{
			Profile: ProfileSidecar,
			Rewards: RewardBreakdown{Correctness: 0.5, Evidence: 1},
			Turns: []TurnRecord{
				// 500 + 500 = 1000 tokens across two turns
				turnWithUsage(100, 100, 200, 100, 0.02, "result"),
				turnWithUsage(100, 100, 200, 100, 0.03, "result"),
			},
		},
	}

	got := AggregateRealTokenSPT(eps)
	sc := got[ProfileSidecar]
	if sc.Episodes != 2 || sc.EpisodesWithUsage != 2 {
		t.Fatalf("sidecar coverage = %d/%d, want 2/2", sc.EpisodesWithUsage, sc.Episodes)
	}
	if !sc.Available {
		t.Fatal("sidecar real-token SPT should be available with full usage coverage")
	}
	if sc.TotalTokens != 3000 {
		t.Fatalf("total tokens = %d, want 3000", sc.TotalTokens)
	}
	// signal sum = 0.5 + 0.5 = 1.0; SPT = 1.0 / 3000 × 1000
	assertSPT(t, "session", sc.SessionSPT, true, 1.0/3.0)

	// Profiles with zero episodes are UNAVAILABLE (0/0), never a zero value.
	gx := got[ProfileGhx]
	if gx.Available || gx.Episodes != 0 || gx.EpisodesWithUsage != 0 {
		t.Fatalf("empty profile should be unavailable 0/0, got %+v", gx)
	}
}

// TestAggregateRealTokenSPTCoverageRule pins ADR-0016.11 D2: one turn
// without usage makes the episode lack usage, and one usage-less episode
// makes the whole profile UNAVAILABLE — no silent partial mean.
func TestAggregateRealTokenSPTCoverageRule(t *testing.T) {
	covered := &Episode{
		Profile: ProfileSidecar,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
		Turns:   []TurnRecord{turnWithUsage(10, 10, 0, 0, 0.01, "result")},
	}
	missingTurnUsage := &Episode{
		Profile: ProfileSidecar,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1},
		Turns: []TurnRecord{
			turnWithUsage(10, 10, 0, 0, 0.01, "result"),
			{}, // pre-ADR-0016.10 shape: no rawSDK at all
		},
	}

	got := AggregateRealTokenSPT([]*Episode{covered, missingTurnUsage})
	sc := got[ProfileSidecar]
	if sc.Available {
		t.Fatal("profile with a usage-less episode must be UNAVAILABLE")
	}
	if sc.Episodes != 2 || sc.EpisodesWithUsage != 1 {
		t.Fatalf("coverage counts = %d/%d, want 1/2", sc.EpisodesWithUsage, sc.Episodes)
	}
	if sc.TotalTokens != 0 || sc.SessionSPT.Defined {
		t.Fatalf("unavailable profile must not report partial tokens/SPT, got %+v", sc)
	}

	// A zero-turn episode also lacks usage (registered edge case).
	zeroTurn := &Episode{Profile: ProfileSidecar, Rewards: RewardBreakdown{Correctness: 1, Evidence: 1}}
	got = AggregateRealTokenSPT([]*Episode{zeroTurn})
	if sc := got[ProfileSidecar]; sc.Available || sc.EpisodesWithUsage != 0 {
		t.Fatalf("zero-turn episode should lack usage, got %+v", sc)
	}
}

// TestComputeRunEconomics pins ADR-0016.11 D4: costs sum over ALL loaded
// episodes (including gate-excluded ones), only turns with terminal
// ("result") usage price an episode, and coverage counts are exact.
func TestComputeRunEconomics(t *testing.T) {
	priced := &Episode{
		Profile: ProfileSidecar,
		Turns:   []TurnRecord{turnWithUsage(1, 1, 0, 0, 0.25, "result")},
	}
	excludedButPriced := &Episode{
		Profile: ProfileGhx,
		Invalid: true, // gate-excluded, still spent money
		Turns:   []TurnRecord{turnWithUsage(1, 1, 0, 0, 0.50, "result")},
	}
	assistantSumOnly := &Episode{
		Profile: ProfilePlain,
		Turns:   []TurnRecord{turnWithUsage(1, 1, 0, 0, 0, "assistant_sum")},
	}
	noUsage := &Episode{Profile: ProfilePlain, Turns: []TurnRecord{{}}}

	econ := ComputeRunEconomics([]*Episode{priced, excludedButPriced, assistantSumOnly, noUsage, nil})
	if econ.Episodes != 4 {
		t.Fatalf("episodes = %d, want 4", econ.Episodes)
	}
	if econ.EpisodesWithCost != 2 {
		t.Fatalf("episodesWithCost = %d, want 2 (assistant_sum and usage-less turns carry no cost)", econ.EpisodesWithCost)
	}
	if !almostEqual(econ.TotalCostUSD, 0.75) {
		t.Fatalf("total cost = %v, want 0.75 (gate-excluded episode must still count)", econ.TotalCostUSD)
	}
	perProfile := map[Profile]ProfileEconomics{}
	for _, pe := range econ.PerProfile {
		perProfile[pe.Profile] = pe
	}
	if pe := perProfile[ProfileGhx]; !almostEqual(pe.CostUSD, 0.50) || pe.EpisodesWithCost != 1 {
		t.Fatalf("ghx economics = %+v, want cost 0.50 over 1/1", pe)
	}
	if pe := perProfile[ProfilePlain]; pe.EpisodesWithCost != 0 || pe.Episodes != 2 {
		t.Fatalf("plain economics = %+v, want 0/2 with cost data", pe)
	}
}

// TestVerdictRendersBothSPTVariantsAndEconomics pins ADR-0016.11 D3: the
// verdict always shows the chars/4 table and the real-token table side by
// side, plus the economics line, without touching gate math.
func TestVerdictRendersBothSPTVariantsAndEconomics(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 0.5, Safety: 1},
		Context: ContextAccounting{MainAgentChars: 1000, SidecarInternalChars: 3000, TotalWorkflowChars: 4000},
		Turns:   []TurnRecord{turnWithUsage(100, 400, 1000, 500, 0.10, "result")},
	}
	v := EvaluateGates([]*Episode{ep})

	if v.SignalPerToken == nil || v.RealTokenSPT == nil || v.Economics == nil {
		t.Fatal("verdict must carry both SPT variants and economics")
	}
	// Gate math is untouched by the informational fields (D3).
	if len(v.Gates) != 5 {
		t.Fatalf("gates = %d, want 5", len(v.Gates))
	}

	md := FormatVerdict(v)
	for _, want := range []string{
		"## Signal per token (informational, not a gate)",
		"chars/4 estimate (ADR-0016.6, primary",
		"provider-reported tokens (ADR-0016.11, session-level",
		"| ghx-sidecar | 1 | 1 | 2000 | 0.250000 |",
		"## Run economics (ADR-0016.11, provider-reported costUsd)",
		"total: $0.1000 over 1/1 episodes with cost data",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("verdict markdown missing %q:\n%s", want, md)
		}
	}
}

// TestVerdictRealTokenUnavailableOnPreUsageArtifacts pins the D2 label on
// old-run shapes: episodes without rawSDK usage (every committed run before
// ADR-0016.10) must render UNAVAILABLE with counts, never zeros.
func TestVerdictRealTokenUnavailableOnPreUsageArtifacts(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Rewards: RewardBreakdown{Correctness: 1, Evidence: 1, Safety: 1},
		Context: ContextAccounting{MainAgentChars: 1000, SidecarInternalChars: 3000, TotalWorkflowChars: 4000},
		Turns:   []TurnRecord{{Text: "answer"}}, // no rawSDK field at all
	}
	v := EvaluateGates([]*Episode{ep})

	md := FormatVerdict(v)
	for _, want := range []string{
		"UNAVAILABLE (0/1 episodes carry usage)",
		"total: UNAVAILABLE (0/1 episodes carry cost data)",
		// The chars/4 variant still reports — it never depends on usage.
		"chars/4 estimate (ADR-0016.6, primary",
	} {
		if !strings.Contains(md, want) {
			t.Fatalf("verdict markdown missing %q:\n%s", want, md)
		}
	}
	if row := v.RealTokenSPT[ProfileSidecar]; row.Available || row.EpisodesWithUsage != 0 {
		t.Fatalf("pre-usage artifact must be unavailable, got %+v", row)
	}
}
