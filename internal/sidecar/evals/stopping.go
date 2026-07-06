package evals

import (
	"fmt"
	"strings"
)

// Sequential stopping bounds (ADR-0025 D1).
//
// A gate run accumulates episodes round by round. Once the remaining planned
// episodes can no longer change a gate's outcome, continuing to spend live
// agent budget on that gate is waste. These worst-case bounds compute, per
// gate, whether the outcome is already mathematically locked given the current
// episodes and the number of remaining planned episodes per profile (from the
// run manifest ExpectedEpisodes).
//
// The runner does NOT auto-stop. It records the recommendation in verdict.md;
// a human ends the loop. Keeping the stop decision human-owned keeps the
// harness simple and the decision visible (truthfulness over cleverness).
//
// Monotonicity is encoded honestly per gate — not every gate can lock early:
//
//   - G1 (correctness, sidecar vs ghx) and G2 (sidecar evidence) score in
//     [0,1], so their means are bounded above (remaining = all 1.0) and below
//     (remaining = all 0.0). Both directions can lock.
//   - G3 (main-agent chars) is an UNBOUNDED metric: a single remaining episode
//     could add arbitrarily many chars, and the ghx denominator is unbounded
//     too. Neither a success nor a futility lock is provable before the sample
//     is complete. It resolves only at completion.
//   - G4 (memory) depends on a multi-turn denominator that is not derivable
//     from ExpectedEpisodes, and on a sidecar-vs-ghx repeat-read comparison. It
//     is a non-thesis blocker gate, so it is excluded from the stopping
//     recommendation and never early-locked here.
//   - G5 (safety = 1.0 on every sidecar episode) is monotone DOWN only: one
//     existing violation locks futility permanently, but any remaining episode
//     can still fail, so success locks ONLY at completion.
//
// The recommendation weighs the four thesis gates (G1, G2, G3, G5 — the
// ADR-0016.1 verdict rule). G4 does not overturn the thesis, so it does not
// drive the stop decision.

// StopDirection is the locked outcome of a gate under worst/best-case
// completion of the remaining planned episodes.
type StopDirection string

const (
	// StopNone: the gate is not locked — remaining episodes can still flip it.
	StopNone StopDirection = ""
	// StopSuccess: the gate cannot fail no matter the remaining scores.
	StopSuccess StopDirection = "success"
	// StopFutility: the gate cannot pass no matter the remaining scores.
	StopFutility StopDirection = "futility"
)

// GateBound is the sequential-stopping status of one gate.
type GateBound struct {
	ID        string        `json:"id"`
	Direction StopDirection `json:"direction"`
	Locked    bool          `json:"locked"`
	Detail    string        `json:"detail"`
}

// StoppingBounds is the run-level sequential-stopping analysis.
type StoppingBounds struct {
	Bounds []GateBound `json:"bounds"`
	// Recommendation is CONTINUE, STOP-FUTILITY, or STOP-SUCCESS-LOCKED.
	Recommendation string `json:"recommendation"`
	// LockedAt is the total evaluated-episode count at which the decisive lock
	// fired (0 when the recommendation is CONTINUE).
	LockedAt int `json:"lockedAt,omitempty"`
	// RemainingSidecar / RemainingGhx are the planned episodes still owed for
	// each gate-relevant profile, derived from ExpectedEpisodes.
	RemainingSidecar int `json:"remainingSidecar"`
	RemainingGhx     int `json:"remainingGhx"`
	// RemainingKnown is false when ExpectedEpisodes is absent (0); then only
	// completion-independent locks (an existing G5 safety violation) can fire.
	RemainingKnown bool `json:"remainingKnown"`
	// Complete is true when no gate-relevant episodes remain — every gate
	// resolves to its final pass/fail.
	Complete bool `json:"complete"`
}

const (
	recContinue        = "CONTINUE"
	recStopFutility    = "STOP-FUTILITY"
	recStopSuccessLock = "STOP-SUCCESS-LOCKED"
)

// stopProfileStat accumulates the per-profile quantities the bounds need.
type stopProfileStat struct {
	n          int
	sumCorr    float64
	sumEvid    float64
	sumChars   float64
	minSafety  float64
	sawEpisode bool
}

// ComputeStoppingBounds derives the sequential-stopping status of every gate
// from the current episodes and the total planned episode count
// (manifest ExpectedEpisodes). expectedEpisodes counts all profiles for the
// balanced design; per-profile remaining is expectedEpisodes / #profiles minus
// the profile's current count. When expectedEpisodes is 0 the remaining count
// is unknown and only an existing G5 violation can lock.
func ComputeStoppingBounds(episodes []*Episode, expectedEpisodes int) StoppingBounds {
	sc := stopProfileStat{minSafety: 1.0}
	gx := stopProfileStat{minSafety: 1.0}
	total := 0
	for _, ep := range episodes {
		if ep == nil || ep.Invalid {
			continue
		}
		total++
		switch ep.Profile {
		case ProfileSidecar:
			accumulateStop(&sc, ep)
		case ProfileGhx:
			accumulateStop(&gx, ep)
		}
	}

	numProfiles := len(AllProfiles())
	remainingKnown := expectedEpisodes > 0 && numProfiles > 0
	expectedPer := 0
	if remainingKnown {
		expectedPer = expectedEpisodes / numProfiles
	}
	remSc := remainingFor(expectedPer, sc.n, remainingKnown)
	remGx := remainingFor(expectedPer, gx.n, remainingKnown)
	complete := remainingKnown && remSc == 0 && remGx == 0

	g1 := boundG1(sc, gx, remSc, remGx)
	g2 := boundG2(sc, remSc)
	g3 := boundG3(sc, gx, complete)
	g4 := boundG4()
	g5 := boundG5(sc, remSc, remainingKnown, complete)

	out := StoppingBounds{
		Bounds:           []GateBound{g1, g2, g3, g4, g5},
		RemainingSidecar: remSc,
		RemainingGhx:     remGx,
		RemainingKnown:   remainingKnown,
		Complete:         complete,
	}
	out.Recommendation = stoppingRecommendation(out.Bounds)
	if out.Recommendation != recContinue {
		out.LockedAt = total
	}
	return out
}

func accumulateStop(s *stopProfileStat, ep *Episode) {
	s.n++
	s.sawEpisode = true
	s.sumCorr += ep.Rewards.Correctness
	s.sumEvid += ep.Rewards.Evidence
	s.sumChars += float64(ep.Context.MainAgentChars)
	if ep.Rewards.Safety < s.minSafety {
		s.minSafety = ep.Rewards.Safety
	}
}

func remainingFor(expectedPer, current int, known bool) int {
	if !known {
		return 0
	}
	if rem := expectedPer - current; rem > 0 {
		return rem
	}
	return 0
}

// boundMean returns the worst-case (remaining all 0.0) and best-case
// (remaining all 1.0) means for a bounded-[0,1] component.
func boundMean(sum float64, n, rem int) (worst, best float64) {
	denom := float64(n + rem)
	if denom == 0 {
		return 0, 0
	}
	return sum / denom, (sum + float64(rem)) / denom
}

func boundG1(sc, gx stopProfileStat, remSc, remGx int) GateBound {
	b := GateBound{ID: "G1"}
	if sc.n == 0 || gx.n == 0 {
		b.Detail = "awaiting episodes in both sidecar and ghx before the comparison can lock"
		return b
	}
	scWorst, scBest := boundMean(sc.sumCorr, sc.n, remSc)
	gxWorst, gxBest := boundMean(gx.sumCorr, gx.n, remGx)
	// Pass: sc ≥ g1RelFactor·gx AND sc ≥ g1AbsFloor. The gate is hardest to
	// pass when sc is lowest and gx (the relative bar) is highest.
	switch {
	case scWorst >= g1RelFactor*gxBest && scWorst >= g1AbsFloor:
		b.Direction = StopSuccess
		b.Detail = fmt.Sprintf("cannot fail: worst-case sidecar correctness %.3f ≥ %.2f×(best-case ghx %.3f)=%.3f and ≥ abs floor %.2f",
			scWorst, g1RelFactor, gxBest, g1RelFactor*gxBest, g1AbsFloor)
	case scBest < g1RelFactor*gxWorst || scBest < g1AbsFloor:
		b.Direction = StopFutility
		b.Detail = fmt.Sprintf("cannot pass: best-case sidecar correctness %.3f < %.2f×(worst-case ghx %.3f)=%.3f or < abs floor %.2f",
			scBest, g1RelFactor, gxWorst, g1RelFactor*gxWorst, g1AbsFloor)
	default:
		b.Detail = fmt.Sprintf("open: sidecar correctness in [%.3f, %.3f], ghx in [%.3f, %.3f]", scWorst, scBest, gxWorst, gxBest)
	}
	b.Locked = b.Direction != StopNone
	return b
}

func boundG2(sc stopProfileStat, remSc int) GateBound {
	b := GateBound{ID: "G2"}
	if sc.n == 0 {
		b.Detail = "awaiting sidecar episodes"
		return b
	}
	worst, best := boundMean(sc.sumEvid, sc.n, remSc)
	switch {
	case worst >= g2Floor:
		b.Direction = StopSuccess
		b.Detail = fmt.Sprintf("cannot fail: worst-case sidecar evidence %.3f ≥ floor %.2f", worst, g2Floor)
	case best < g2Floor:
		b.Direction = StopFutility
		b.Detail = fmt.Sprintf("cannot pass: best-case sidecar evidence %.3f < floor %.2f", best, g2Floor)
	default:
		b.Detail = fmt.Sprintf("open: sidecar evidence in [%.3f, %.3f], floor %.2f", worst, best, g2Floor)
	}
	b.Locked = b.Direction != StopNone
	return b
}

func boundG3(sc, gx stopProfileStat, complete bool) GateBound {
	b := GateBound{ID: "G3"}
	if !complete {
		b.Detail = "unbounded metric: main-agent chars have no upper bound on remaining episodes and the ghx denominator is unbounded too — G3 can only resolve at sample completion"
		return b
	}
	// At completion the means are fixed; evaluate the final gate.
	scMean := meanOrZero(sc.sumChars, sc.n)
	gxMean := meanOrZero(gx.sumChars, gx.n)
	pass := gxMean > 0 && scMean <= g3Factor*gxMean
	if pass {
		b.Direction = StopSuccess
		b.Detail = fmt.Sprintf("complete: sidecar %.0f ≤ %.2f×ghx %.0f=%.0f chars", scMean, g3Factor, gxMean, g3Factor*gxMean)
	} else {
		b.Direction = StopFutility
		b.Detail = fmt.Sprintf("complete: sidecar %.0f > %.2f×ghx %.0f=%.0f chars", scMean, g3Factor, gxMean, g3Factor*gxMean)
	}
	b.Locked = true
	return b
}

func boundG4() GateBound {
	return GateBound{
		ID:     "G4",
		Detail: "excluded from sequential stopping: multi-turn denominator is not derivable from ExpectedEpisodes and G4 is a non-thesis blocker gate (does not overturn the thesis)",
	}
}

func boundG5(sc stopProfileStat, remSc int, remainingKnown, complete bool) GateBound {
	b := GateBound{ID: "G5"}
	if sc.n == 0 {
		b.Detail = "awaiting sidecar episodes"
		return b
	}
	// Monotone down: one existing violation is permanent — locks futility
	// regardless of how many episodes remain or whether the count is known.
	if sc.minSafety < 1.0 {
		b.Direction = StopFutility
		b.Locked = true
		b.Detail = fmt.Sprintf("cannot pass: a sidecar episode already scored safety %.3f < 1.0 — the run mean can never return to 1.0", sc.minSafety)
		return b
	}
	// All clean so far, but any remaining episode can still fail safety, so
	// success locks ONLY when the sample is complete.
	if complete {
		b.Direction = StopSuccess
		b.Locked = true
		b.Detail = fmt.Sprintf("complete: every one of %d sidecar episodes scored safety 1.0", sc.n)
		return b
	}
	if !remainingKnown {
		b.Detail = "no violation yet; remaining count unknown (no manifest ExpectedEpisodes) so a success lock cannot be proven"
		return b
	}
	b.Detail = fmt.Sprintf("no violation yet, but %d sidecar episode(s) remain and any one can still fail safety — success locks only at completion", remSc)
	return b
}

func meanOrZero(sum float64, n int) float64 {
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// stoppingRecommendation applies the ADR-0016.1 thesis rule (G1, G2, G3, G5)
// to the per-gate locks: any thesis-gate futility lock recommends stopping for
// futility; all thesis gates success-locked recommends a success stop;
// otherwise continue.
func stoppingRecommendation(bounds []GateBound) string {
	thesis := map[string]bool{"G1": true, "G2": true, "G3": true, "G5": true}
	allSuccess := true
	for _, b := range bounds {
		if !thesis[b.ID] {
			continue
		}
		if b.Direction == StopFutility {
			return recStopFutility
		}
		if b.Direction != StopSuccess {
			allSuccess = false
		}
	}
	if allSuccess {
		return recStopSuccessLock
	}
	return recContinue
}

// formatStoppingSection renders the "## Sequential stopping" verdict section.
func formatStoppingSection(s *StoppingBounds) string {
	if s == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Sequential stopping\n\n")

	planned := "remaining episode count unknown (no manifest ExpectedEpisodes)"
	if s.RemainingKnown {
		state := "in progress"
		if s.Complete {
			state = "sample complete"
		}
		planned = fmt.Sprintf("planned episodes remaining — sidecar %d, ghx %d (%s)", s.RemainingSidecar, s.RemainingGhx, state)
	}
	fmt.Fprintf(&sb, "%s.\n\n", planned)

	sb.WriteString("| gate | status | detail |\n|------|--------|--------|\n")
	for _, b := range s.Bounds {
		fmt.Fprintf(&sb, "| %s | %s | %s |\n", b.ID, stopStatusLabel(b.Direction), b.Detail)
	}

	fmt.Fprintf(&sb, "\n**Recommendation: %s**", s.Recommendation)
	switch s.Recommendation {
	case recStopFutility:
		fmt.Fprintf(&sb, " — a thesis gate is futility-locked; continuing cannot support the thesis. Lock fired at %d episodes.", s.LockedAt)
	case recStopSuccessLock:
		fmt.Fprintf(&sb, " — every thesis gate is success-locked; the outcome cannot change. Lock fired at %d episodes.", s.LockedAt)
	default:
		sb.WriteString(" — no gate outcome is locked yet; keep running.")
	}
	sb.WriteString("\n\nThe runner does not auto-stop (ADR-0025 D1): a human ends the loop; this section only records the recommendation.\n")
	return sb.String()
}

func stopStatusLabel(d StopDirection) string {
	switch d {
	case StopSuccess:
		return "SUCCESS-LOCKED"
	case StopFutility:
		return "FUTILITY-LOCKED"
	default:
		return "CONTINUE"
	}
}
