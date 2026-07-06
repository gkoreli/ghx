package evals

import (
	"strings"
	"testing"
)

func epWithRewards(id string, corr, evid, safety float64) *Episode {
	return &Episode{ID: id, TaskID: "t1", Rewards: RewardBreakdown{Correctness: corr, Evidence: evid, Safety: safety}}
}

func judgeResult(id string, overall float64) *JudgeResult {
	return &JudgeResult{EpisodeID: id, Overall: overall}
}

func TestComputeDisagreementInvestigate(t *testing.T) {
	episodes := []*Episode{
		epWithRewards("agree-good", 1.0, 0.8, 1.0), // gate pass
		epWithRewards("pass-poor", 1.0, 0.8, 1.0),  // gate pass
		epWithRewards("fail-good", 0.3, 0.4, 1.0),  // gate fail
		epWithRewards("agree-bad", 0.3, 0.4, 1.0),  // gate fail
		epWithRewards("skipped-ep", 1.0, 0.8, 1.0), // judge skipped
	}
	results := map[string]*JudgeResult{
		"agree-good": judgeResult("agree-good", 4.0), // gate pass, judge good → agree
		"pass-poor":  judgeResult("pass-poor", 2.0),  // gate pass, judge poor → conflict
		"fail-good":  judgeResult("fail-good", 4.0),  // gate fail, judge good → conflict
		"agree-bad":  judgeResult("agree-bad", 2.0),  // gate fail, judge poor → agree
		"skipped-ep": {EpisodeID: "skipped-ep", Skipped: true, SkipReason: "BLOCKED"},
	}

	rep := ComputeDisagreement(episodes, results)
	if rep.Scored != 4 {
		t.Errorf("scored=%d want 4", rep.Scored)
	}
	if rep.Skipped != 1 {
		t.Errorf("skipped=%d want 1", rep.Skipped)
	}
	if rep.GatesPassJudgePoor != 1 {
		t.Errorf("gatesPassJudgePoor=%d want 1", rep.GatesPassJudgePoor)
	}
	if rep.GatesFailJudgeGood != 1 {
		t.Errorf("gatesFailJudgeGood=%d want 1", rep.GatesFailJudgeGood)
	}
	if rep.DisagreementRate != 0.5 {
		t.Errorf("rate=%.3f want 0.5", rep.DisagreementRate)
	}
	if !rep.Investigate {
		t.Errorf("50%% disagreement should mark INVESTIGATE")
	}

	md := RenderDisagreementReport(episodes, results)
	for _, want := range []string{"PRELIMINARY", "INVESTIGATE", "gates pass, judge poor", "pass-poor", "fail-good"} {
		if !strings.Contains(md, want) {
			t.Errorf("disagreement markdown missing %q", want)
		}
	}
}

func TestComputeDisagreementBelowBound(t *testing.T) {
	episodes := []*Episode{
		epWithRewards("a", 1.0, 0.8, 1.0),
		epWithRewards("b", 1.0, 0.8, 1.0),
		epWithRewards("c", 1.0, 0.8, 1.0),
		epWithRewards("d", 0.2, 0.2, 1.0),
	}
	results := map[string]*JudgeResult{
		"a": judgeResult("a", 4.0), // agree
		"b": judgeResult("b", 4.5), // agree
		"c": judgeResult("c", 4.0), // agree
		"d": judgeResult("d", 2.0), // agree (gate fail, judge poor)
	}
	rep := ComputeDisagreement(episodes, results)
	if rep.DisagreementRate != 0 {
		t.Errorf("rate=%.3f want 0", rep.DisagreementRate)
	}
	if rep.Investigate {
		t.Errorf("full agreement should not mark INVESTIGATE")
	}
	md := AppendDisagreementReport("# Verdict\n", episodes, results)
	if !strings.Contains(md, "# Verdict") || !strings.Contains(md, "Gate/Judge Disagreement") {
		t.Errorf("appended report should preserve base verdict and add section")
	}
}
