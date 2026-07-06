package evals

import (
	"crypto/sha256"
	"encoding/binary"
)

// D4 secondary-judge selection (ADR-0023.1): the Anthropic opus-class
// secondary scores a 20% sample of episodes plus every episode the primary
// judge scored below the poor threshold. Selection is a small helper on top of
// primary JudgeResults — it is deliberately NOT wired into JudgeRunner, which
// stays a single-client k-sample loop; callers run a second JudgeRunner with
// the AnthropicJudgeClient over the selected episodes and report family
// disagreement, never an average.

// judgeSecondarySalt versions the deterministic sampling hash. Changing it
// reshuffles which episodes fall in the sample and is a measurement-stack
// change (committed diff).
const judgeSecondarySalt = "judge-secondary-v1"

// NeedsSecondOpinion reports whether an episode's primary judge result selects
// it for the Anthropic secondary judge (D4): always when the primary overall
// is below the committed poor threshold, otherwise when the episode falls in
// the deterministic sampleRate share.
//
// Sampling is deterministic by episode ID (salted SHA-256 mapped to [0,1)),
// not random at call time, so the exact secondary set is recomputable from
// committed artifacts (AGENTS.md visibility tenet). Skipped results (anomaly
// pre-filtered) are never selected — there is nothing to judge.
func NeedsSecondOpinion(res *JudgeResult, sampleRate float64) bool {
	if res == nil || res.Skipped {
		return false
	}
	if judgePoor(res) {
		return true
	}
	if sampleRate <= 0 {
		return false
	}
	if sampleRate >= 1 {
		return true
	}
	return episodeSampleFraction(res.EpisodeID) < sampleRate
}

// SelectSecondOpinions filters primary results down to the episodes the
// secondary judge must score, preserving input order.
func SelectSecondOpinions(primary []*JudgeResult, sampleRate float64) []*JudgeResult {
	var out []*JudgeResult
	for _, res := range primary {
		if NeedsSecondOpinion(res, sampleRate) {
			out = append(out, res)
		}
	}
	return out
}

// episodeSampleFraction maps an episode ID to a deterministic uniform value in
// [0,1) via salted SHA-256 (top 53 bits, so the float mantissa is exact).
func episodeSampleFraction(episodeID string) float64 {
	sum := sha256.Sum256([]byte(judgeSecondarySalt + ":" + episodeID))
	u := binary.BigEndian.Uint64(sum[:8])
	return float64(u>>11) / float64(uint64(1)<<53)
}
