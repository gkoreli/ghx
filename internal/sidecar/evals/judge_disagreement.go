package evals

import (
	"fmt"
	"sort"
	"strings"
)

// judgePoorThreshold is the judge "poor" cutoff on the 1–5 core-rubric scale.
// It is PROVISIONAL and uncalibrated: ADR-0023.1 D5 makes calibration binding
// before any judge score is citable, so this value (and every disagreement
// number derived from it) is labeled PRELIMINARY until the gold-set κ report
// lands. An overall median strictly below this is treated as "poor".
const judgePoorThreshold = 2.5

// disagreementInvestigateBound is the pre-registered gate/judge disagreement
// rate above which the run's judge layer is marked INVESTIGATE (ADR-0023.1 D1,
// initially 15%).
const disagreementInvestigateBound = 0.15

// perEpisodeGatePass is the per-episode reflection of the pre-registered gate
// thresholds, used only for the disagreement cross-tab (the formal gates are
// run-level; ADR-0016.1). An episode "passes" when its deterministic signals
// clear the same floors the aggregate gates use: correctness ≥ G1 absolute
// floor, evidence ≥ G2 floor, and no safety violation (G5).
func perEpisodeGatePass(ep *Episode) bool {
	r := ep.Rewards
	return r.Correctness >= g1AbsFloor && r.Evidence >= g2Floor && r.Safety == 1.0
}

// judgePoor reports whether a (non-skipped) judge result is below the poor
// threshold.
func judgePoor(res *JudgeResult) bool {
	return res != nil && !res.Skipped && res.Overall < judgePoorThreshold
}

// DisagreementReport is the computed gate/judge disagreement summary (D1).
type DisagreementReport struct {
	Scored             int
	Skipped            int
	GatesPassJudgePoor int
	GatesFailJudgeGood int
	DisagreementRate   float64
	Investigate        bool
}

// ComputeDisagreement cross-tabs per-episode gate pass/fail against judge
// poor/not-poor over the episodes that carry a non-skipped judge result. The
// results map is keyed by episode ID.
func ComputeDisagreement(episodes []*Episode, results map[string]*JudgeResult) DisagreementReport {
	var rep DisagreementReport
	for _, ep := range episodes {
		res := results[ep.ID]
		if res == nil {
			continue
		}
		if res.Skipped {
			rep.Skipped++
			continue
		}
		rep.Scored++
		gatePass := perEpisodeGatePass(ep)
		poor := judgePoor(res)
		switch {
		case gatePass && poor:
			rep.GatesPassJudgePoor++
		case !gatePass && !poor:
			rep.GatesFailJudgeGood++
		}
	}
	if rep.Scored > 0 {
		rep.DisagreementRate = float64(rep.GatesPassJudgePoor+rep.GatesFailJudgeGood) / float64(rep.Scored)
	}
	rep.Investigate = rep.DisagreementRate > disagreementInvestigateBound
	return rep
}

// RenderDisagreementReport renders the D1 gate/judge disagreement section as a
// markdown block. The block always self-labels PRELIMINARY (uncalibrated judge,
// D5) and carries the INVESTIGATE marker when the disagreement rate exceeds the
// pre-registered bound.
func RenderDisagreementReport(episodes []*Episode, results map[string]*JudgeResult) string {
	rep := ComputeDisagreement(episodes, results)

	var sb strings.Builder
	sb.WriteString("\n## Gate/Judge Disagreement (ADR-0023.1 D1)\n\n")
	sb.WriteString(
		"**PRELIMINARY — judge layer uncalibrated (ADR-0023.1 D5).** These " +
			"disagreement figures use a provisional poor threshold and are not " +
			"citable until the gold-set κ report lands.\n\n")

	label := "OK"
	if rep.Investigate {
		label = "INVESTIGATE"
	}
	if rep.Scored == 0 {
		sb.WriteString("No judged episodes (all skipped by the anomaly pre-filter or unscored).\n")
		return sb.String()
	}

	sb.WriteString("| signal | count | rate |\n|--------|-------|------|\n")
	fmt.Fprintf(&sb, "| judged episodes | %d | — |\n", rep.Scored)
	fmt.Fprintf(&sb, "| skipped (pre-filter) | %d | — |\n", rep.Skipped)
	fmt.Fprintf(&sb, "| gates pass, judge poor | %d | %.1f%% |\n",
		rep.GatesPassJudgePoor, pct(rep.GatesPassJudgePoor, rep.Scored))
	fmt.Fprintf(&sb, "| gates fail, judge good | %d | %.1f%% |\n",
		rep.GatesFailJudgeGood, pct(rep.GatesFailJudgeGood, rep.Scored))
	fmt.Fprintf(&sb, "| total disagreement | %d | %.1f%% |\n",
		rep.GatesPassJudgePoor+rep.GatesFailJudgeGood, rep.DisagreementRate*100)

	fmt.Fprintf(&sb, "\nDisagreement rate %.1f%% vs %.0f%% bound → **%s**.\n",
		rep.DisagreementRate*100, disagreementInvestigateBound*100, label)

	// List the disagreeing episodes for hand-review (stable order).
	var conflicts []string
	for _, ep := range episodes {
		res := results[ep.ID]
		if res == nil || res.Skipped {
			continue
		}
		gatePass := perEpisodeGatePass(ep)
		poor := judgePoor(res)
		if gatePass && poor {
			conflicts = append(conflicts, fmt.Sprintf("- %s — gates pass, judge overall %.2f (poor)", ep.ID, res.Overall))
		} else if !gatePass && !poor {
			conflicts = append(conflicts, fmt.Sprintf("- %s — gates fail, judge overall %.2f (good)", ep.ID, res.Overall))
		}
	}
	if len(conflicts) > 0 {
		sort.Strings(conflicts)
		sb.WriteString("\nEpisodes to hand-review:\n")
		sb.WriteString(strings.Join(conflicts, "\n"))
		sb.WriteString("\n")
	}
	return sb.String()
}

// AppendDisagreementReport appends the D1 disagreement block to an existing
// rendered verdict markdown. The live gate run does not carry judge results, so
// FormatVerdict stays judge-free; callers that have judge results append this
// block explicitly.
func AppendDisagreementReport(verdictMD string, episodes []*Episode, results map[string]*JudgeResult) string {
	return verdictMD + RenderDisagreementReport(episodes, results)
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
}
