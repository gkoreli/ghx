package evals

import (
	"fmt"
	"testing"
)

func secondaryResult(id string, overall float64, skipped bool) *JudgeResult {
	return &JudgeResult{EpisodeID: id, Overall: overall, Skipped: skipped}
}

func TestNeedsSecondOpinionBelowThreshold(t *testing.T) {
	res := secondaryResult("ep-poor", judgePoorThreshold-0.1, false)
	if !NeedsSecondOpinion(res, 0) {
		t.Error("below-threshold result must always get a second opinion, even at sample rate 0 (D4)")
	}
}

func TestNeedsSecondOpinionSkipsPrefiltered(t *testing.T) {
	if NeedsSecondOpinion(secondaryResult("ep-skipped", 0, true), 1) {
		t.Error("skipped (anomaly pre-filtered) episodes have nothing to judge")
	}
	if NeedsSecondOpinion(nil, 1) {
		t.Error("nil result must not be selected")
	}
}

func TestNeedsSecondOpinionSampleRateBounds(t *testing.T) {
	good := secondaryResult("ep-good", 4.5, false)
	if NeedsSecondOpinion(good, 0) {
		t.Error("rate 0 must select only below-threshold results")
	}
	if !NeedsSecondOpinion(good, 1) {
		t.Error("rate 1 must select every non-skipped result")
	}
}

func TestNeedsSecondOpinionIsDeterministic(t *testing.T) {
	res := secondaryResult("gin-routing_x_1783294237204", 4.0, false)
	first := NeedsSecondOpinion(res, 0.2)
	for i := 0; i < 10; i++ {
		if NeedsSecondOpinion(res, 0.2) != first {
			t.Fatal("selection flipped across calls; sampling must be deterministic by episode ID")
		}
	}
}

// TestEpisodeSampleFractionDistribution pins the deterministic hash: fractions
// stay in [0,1) and a 20% rate selects roughly 20% of a synthetic population.
// The check is exact-reproducible (salted SHA-256), not statistical noise.
func TestEpisodeSampleFractionDistribution(t *testing.T) {
	const n = 2000
	selected := 0
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("episode-%04d", i)
		f := episodeSampleFraction(id)
		if f < 0 || f >= 1 {
			t.Fatalf("episodeSampleFraction(%q) = %v, out of [0,1)", id, f)
		}
		if f < 0.2 {
			selected++
		}
	}
	rate := float64(selected) / n
	if rate < 0.15 || rate > 0.25 {
		t.Errorf("selected share = %v, want ≈0.2 (deterministic hash drifted?)", rate)
	}
}

func TestSelectSecondOpinions(t *testing.T) {
	poor := secondaryResult("ep-poor", 1.5, false)
	skipped := secondaryResult("ep-skipped", 0, true)
	good1 := secondaryResult("ep-good-1", 4.0, false)
	good2 := secondaryResult("ep-good-2", 4.5, false)

	got := SelectSecondOpinions([]*JudgeResult{good1, poor, skipped, good2}, 0)
	if len(got) != 1 || got[0] != poor {
		t.Errorf("rate 0: got %d selections, want only the poor episode", len(got))
	}

	got = SelectSecondOpinions([]*JudgeResult{good1, poor, skipped, good2}, 1)
	if len(got) != 3 {
		t.Errorf("rate 1: got %d selections, want 3 (all non-skipped)", len(got))
	}
	if len(got) == 3 && (got[0] != good1 || got[1] != poor || got[2] != good2) {
		t.Error("selection must preserve input order")
	}
}
