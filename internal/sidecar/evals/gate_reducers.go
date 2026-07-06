package evals

import (
	"fmt"
	"sort"
)

// Gate reducers and fragility annotation (ADR-0025.2).
//
// Every gate in gates.go reduces per-episode metrics to one pass/fail
// verdict. Until this file, there was exactly one reducer — pool a profile's
// trials into a mean (or rate) and compare that scalar against a fixed
// threshold ("ReducerMean"). At the pre-registered gate-run sample size
// (minGateRunTrials = 5), a mean-vs-threshold reducer can flip PASS to FAIL
// because of a single thin-margin trial (TRUST.md H6). This file adds:
//
//   - ReducerAtLeastN: an opt-in second reducer that counts how many trials
//     (or, for G1, paired task cells) individually clear the gate's per-trial
//     threshold, instead of averaging trials first. G2 and G4 landed with
//     ADR-0025.2; the residuals closure added G1 (paired task cells) and G3
//     (pre-registered absolute char ceiling).
//   - Fragility: a deterministic annotation, computed for every gate on
//     every call, reporting whether flipping exactly one already-scored
//     trial's outcome would change that gate's Pass value.
//
// GateOptions{} (the zero value) reproduces every gate's pre-existing
// ReducerMean behavior exactly — EvaluateGates is defined as
// EvaluateGatesWithOptions(episodes, GateOptions{}), so no existing caller's
// scoring changes.

// GateReducer names how a gate reduced its trials to a pass/fail verdict.
// Always populated on GateResult (never the zero value) so a reader of
// verdict.json never has to infer which math scored a gate.
type GateReducer string

const (
	// ReducerMean is the ADR-0016.1 default: pool the gate's per-episode
	// metric into one profile-level mean (or rate) and compare that scalar
	// against the gate's fixed threshold. Every gate uses this reducer
	// unless GateOptions opts it into ReducerAtLeastN.
	ReducerMean GateReducer = "mean"

	// ReducerAtLeastN requires at least N of the gate's evaluated units to
	// individually clear the gate's per-unit threshold. The unit is the
	// individual trial for G2 (evidence ≥ g2Floor), G3 (main-agent chars ≤
	// the spec's pre-registered CharCeiling), and G4 (allFollowupsResumed);
	// for G1 the unit is the paired task cell (same task, sidecar and ghx
	// task-level mean correctness compared by g1Pass), because nothing in
	// the episode model pairs one sidecar trial to "its" ghx trial
	// (ADR-0025.2 D2 + residuals closure). G5 does not take this reducer:
	// it is already the strictest possible instance (n == N) under
	// ReducerMean.
	ReducerAtLeastN GateReducer = "at_least_n"
)

// AtLeastNSpec configures the at_least(n) reducer for one gate: N of the
// gate's evaluated units (trials, or paired task cells for G1) must
// individually clear its per-unit threshold.
type AtLeastNSpec struct {
	// N is the minimum number of units that must individually pass.
	N int
	// CharCeiling is the pre-registered per-episode main-agent chars
	// ceiling used only by G3's at_least(n) form (ADR-0025.2 residuals):
	// a sidecar trial individually passes iff its MainAgentChars is at or
	// under this ceiling. G3's ratio metric is unbounded, so its at_least
	// form needs an explicit absolute budget — a G3 spec without a
	// positive CharCeiling is rejected with a GATE CONFIG verdict note,
	// never silently defaulted. Ignored (with a verdict note) on any
	// other gate.
	CharCeiling int
}

// GateOptions is the opt-in override surface for EvaluateGatesWithOptions.
// The zero value reproduces EvaluateGates exactly (ADR-0025.2 invariant:
// existing gate configs keep their current semantics unless they opt into
// at_least(n)).
type GateOptions struct {
	// AtLeastN maps a gate ID ("G1", "G2", "G3", or "G4") to an
	// at_least(n) override. Gate IDs absent from this map keep
	// ReducerMean. A key for any other gate ID, or a G3 key without a
	// positive CharCeiling, is not silently ignored:
	// EvaluateGatesWithOptions records a verdict note that the override
	// was not applied (ADR-0025.2 D2 + residuals closure).
	AtLeastN map[string]AtLeastNSpec
}

// atLeastNSupportedGates are the gate IDs ReducerAtLeastN is wired for:
// G2/G4 from the ADR-0025.2 landing, G1 (paired task cells) and G3
// (pre-registered char ceiling) from the residuals closure. G5 stays
// ReducerMean — it is already the strictest possible at_least instance.
var atLeastNSupportedGates = map[string]bool{"G1": true, "G2": true, "G3": true, "G4": true}

// atLeastSpec looks up a gate's raw at_least(n) override, or nil if the
// gate has no entry in GateOptions.
func atLeastSpec(opts GateOptions, id string) *AtLeastNSpec {
	if opts.AtLeastN == nil {
		return nil
	}
	spec, ok := opts.AtLeastN[id]
	if !ok {
		return nil
	}
	out := spec
	return &out
}

// effectiveAtLeastSpec returns the at_least(n) spec that will actually be
// applied for gate id, or nil when the gate keeps ReducerMean: no override,
// an unsupported gate, or an incomplete G3 spec missing its pre-registered
// char ceiling. gateConfigNotes reports every rejected override, so the two
// functions must stay in agreement.
func effectiveAtLeastSpec(opts GateOptions, id string) *AtLeastNSpec {
	if !atLeastNSupportedGates[id] {
		return nil
	}
	spec := atLeastSpec(opts, id)
	if spec == nil {
		return nil
	}
	if id == "G3" && spec.CharCeiling <= 0 {
		return nil
	}
	return spec
}

// gateConfigNotes reports a verdict note for every GateOptions entry that
// EvaluateGatesWithOptions will not apply as written — an unsupported gate
// ID, a G3 override missing its pre-registered char ceiling, or a char
// ceiling on a gate that does not take one — so a caller cannot believe an
// override took effect when it did not (ADR-0025.2 D2, visibility tenet).
func gateConfigNotes(opts GateOptions) []string {
	var ids []string
	for id := range opts.AtLeastN {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var notes []string
	for _, id := range ids {
		spec := opts.AtLeastN[id]
		if !atLeastNSupportedGates[id] {
			notes = append(notes, fmt.Sprintf(
				"GATE CONFIG: at_least(n) requested for %s but %s does not support this reducer (ADR-0025.2) — %s uses ReducerMean",
				id, id, id))
			continue
		}
		if id == "G3" && spec.CharCeiling <= 0 {
			notes = append(notes,
				"GATE CONFIG: at_least(n) requested for G3 without a positive per-episode char ceiling (AtLeastNSpec.CharCeiling) — the ceiling must be pre-registered before use (ADR-0025.2 residuals); G3 uses ReducerMean")
			continue
		}
		if id != "G3" && spec.CharCeiling != 0 {
			notes = append(notes, fmt.Sprintf(
				"GATE CONFIG: CharCeiling set for %s but only G3 takes a char ceiling (ADR-0025.2 residuals) — the ceiling was ignored",
				id))
		}
	}
	return notes
}

// countAtLeast counts how many values individually clear floor.
func countAtLeast(vals []float64, floor float64) int {
	k := 0
	for _, v := range vals {
		if v >= floor {
			k++
		}
	}
	return k
}

// countAtMost counts how many values individually stay at or under ceiling
// (G3's at_least(n) per-trial predicate against its pre-registered char
// ceiling).
func countAtMost(vals []float64, ceiling float64) int {
	k := 0
	for _, v := range vals {
		if v <= ceiling {
			k++
		}
	}
	return k
}

// atLeastNFragile is the closed-form fragility check for ReducerAtLeastN
// (and for any boolean-rate gate, e.g. G4's default resume-rate reducer):
// k of n units individually pass, and the gate requires at least req. The
// unit names what is being counted in FragileDetail — "trials" for
// G2/G3/G4, "paired task cells" for G1. If currently passing (k >= req),
// the gate is fragile iff k == req exactly — one pass-to-fail flip would
// drop below req. If currently failing (k < req), it is fragile iff
// k == req-1 — one fail-to-pass flip would reach req.
func atLeastNFragile(k, req, total int, unit string, passNow bool) (bool, string) {
	if total == 0 {
		return false, ""
	}
	if passNow {
		if k == req {
			return true, fmt.Sprintf(
				"exactly %d of %d %s pass — one flipping from pass to fail would drop below the required %d",
				k, total, unit, req)
		}
		return false, ""
	}
	if req > 0 && k == req-1 {
		return true, fmt.Sprintf(
			"%d of %d %s pass, one short of the required %d — one flipping from fail to pass would clear it",
			k, total, unit, req)
	}
	return false, ""
}

// rateFragile is the closed-form fragility check for a mean-reducer rate
// gate whose underlying per-episode metric is already boolean (G4's default
// resume-rate: allFollowupsResumed is a per-episode boolean, so the rate is
// mathematically a count k of n against a floor, identical in spirit to
// atLeastNFragile but expressed as a rate threshold rather than a raw count).
func rateFragile(k, n int, floor float64, passNow bool) (bool, string) {
	if n == 0 {
		return false, ""
	}
	if passNow {
		if k == 0 {
			return false, ""
		}
		newRate := float64(k-1) / float64(n)
		if newRate < floor {
			return true, fmt.Sprintf(
				"flipping one resumed trial to not-resumed drops the rate to %.2f (%d/%d), below floor %.2f",
				newRate, k-1, n, floor)
		}
		return false, ""
	}
	if k == n {
		return false, ""
	}
	newRate := float64(k+1) / float64(n)
	if newRate >= floor {
		return true, fmt.Sprintf(
			"flipping one non-resumed trial to resumed raises the rate to %.2f (%d/%d), meeting floor %.2f",
			newRate, k+1, n, floor)
	}
	return false, ""
}

// mean (rewards.go) returns the arithmetic mean of vals, or 0 for an empty
// slice; reused here rather than redeclared.

// indexOfExtreme returns the index of the max (wantMax) or min value in
// vals. Callers must check len(vals) > 0.
func indexOfExtreme(vals []float64, wantMax bool) int {
	idx := 0
	for i := 1; i < len(vals); i++ {
		if wantMax && vals[i] > vals[idx] {
			idx = i
		}
		if !wantMax && vals[i] < vals[idx] {
			idx = i
		}
	}
	return idx
}

// meanWithReplacement recomputes the mean of vals with the value at idx
// replaced by newVal, leaving every other value and the trial count
// unchanged — the single-trial extremization used for bounded [0,1] mean
// gates (G1, default G2, G5).
func meanWithReplacement(vals []float64, idx int, newVal float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for i, v := range vals {
		if i == idx {
			sum += newVal
		} else {
			sum += v
		}
	}
	return sum / float64(len(vals))
}

// meanExcluding recomputes the mean of vals with the value at idx removed
// entirely (both its value and its weight in the denominator) — the
// leave-one-out sensitivity used for G3's unbounded ratio metric, which has
// no finite worst-case extreme to substitute in its place.
func meanExcluding(vals []float64, idx int) float64 {
	if len(vals) <= 1 {
		return 0
	}
	sum := 0.0
	for i, v := range vals {
		if i == idx {
			continue
		}
		sum += v
	}
	return sum / float64(len(vals)-1)
}

// meanFloorFragile is the single-trial extremization fragility check for a
// bounded-[0,1] mean-reducer gate compared against one absolute floor
// (default G2: floor = g2Floor; G5: floor = 1.0). If the gate currently
// passes, the worst single-trial attack is flipping the highest-scoring
// trial down to 0. If it currently fails, the best single-trial rescue is
// flipping the lowest-scoring trial up to 1. Values must already be known
// to lie in [0,1] (rewards.go's contract) for 0/1 to be valid domain
// extremes.
func meanFloorFragile(vals []float64, floor float64, passNow bool) (bool, string) {
	if len(vals) == 0 {
		return false, ""
	}
	if passNow {
		i := indexOfExtreme(vals, true)
		newMean := meanWithReplacement(vals, i, 0)
		if newMean < floor {
			return true, fmt.Sprintf(
				"flipping the highest-scoring trial (%.3f -> 0.00) drops the mean to %.3f, below floor %.2f",
				vals[i], newMean, floor)
		}
		return false, ""
	}
	i := indexOfExtreme(vals, false)
	newMean := meanWithReplacement(vals, i, 1)
	if newMean >= floor {
		return true, fmt.Sprintf(
			"flipping the lowest-scoring trial (%.3f -> 1.00) raises the mean to %.3f, meeting floor %.2f",
			vals[i], newMean, floor)
	}
	return false, ""
}

// g1Pass is the G1 gate condition, factored out so the fragility check
// below reuses the exact comparison the real gate uses.
func g1Pass(scMean, gxMean float64) bool {
	return scMean >= g1RelFactor*gxMean && scMean >= g1AbsFloor
}

// g1Fragile is G1's single-trial extremization fragility check. G1 compares
// two profiles, so a single-trial flip can come from either side: a
// sidecar-side flip toward 0 (hurts the sidecar mean) or a ghx-side flip
// toward 1 (raises the relative bar the sidecar must clear). Fragile if
// either single flip alone would change Pass.
func g1Fragile(scVals, gxVals []float64, passNow bool) (bool, string) {
	if len(scVals) == 0 || len(gxVals) == 0 {
		return false, ""
	}
	scMean := mean(scVals)
	gxMean := mean(gxVals)
	if passNow {
		i := indexOfExtreme(scVals, true)
		newSc := meanWithReplacement(scVals, i, 0)
		if !g1Pass(newSc, gxMean) {
			return true, fmt.Sprintf(
				"flipping the highest-scoring sidecar trial (%.3f -> 0.00) drops sidecar correctness to %.3f, failing G1 against ghx %.3f",
				scVals[i], newSc, gxMean)
		}
		j := indexOfExtreme(gxVals, false)
		newGx := meanWithReplacement(gxVals, j, 1)
		if !g1Pass(scMean, newGx) {
			return true, fmt.Sprintf(
				"flipping the lowest-scoring ghx trial (%.3f -> 1.00) raises ghx correctness to %.3f, failing G1 for sidecar %.3f",
				gxVals[j], newGx, scMean)
		}
		return false, ""
	}
	i := indexOfExtreme(scVals, false)
	newSc := meanWithReplacement(scVals, i, 1)
	if g1Pass(newSc, gxMean) {
		return true, fmt.Sprintf(
			"flipping the lowest-scoring sidecar trial (%.3f -> 1.00) raises sidecar correctness to %.3f, passing G1 against ghx %.3f",
			scVals[i], newSc, gxMean)
	}
	j := indexOfExtreme(gxVals, true)
	newGx := meanWithReplacement(gxVals, j, 0)
	if g1Pass(scMean, newGx) {
		return true, fmt.Sprintf(
			"flipping the highest-scoring ghx trial (%.3f -> 0.00) drops ghx correctness to %.3f, passing G1 for sidecar %.3f",
			gxVals[j], newGx, scMean)
	}
	return false, ""
}

// g1TaskPair is one paired task cell for G1's at_least(n) reducer
// (ADR-0025.2 residuals): the same task's mean correctness under the
// sidecar and ghx profiles. Pairing happens at the task-cell level, not the
// individual-trial level — trial counts can differ per profile and nothing
// in the episode model pairs one sidecar trial to "its" ghx trial, but a
// task cell is comparable by construction (same task, both profiles).
type g1TaskPair struct {
	// TaskID is the shared task the two profile cells ran.
	TaskID string
	// ScMean is the mean sidecar correctness over the task's sidecar trials.
	ScMean float64
	// GxMean is the mean ghx correctness over the task's ghx trials.
	GxMean float64
}

// pass reports whether this paired task cell individually satisfies the G1
// bound — the exact g1Pass comparison the ReducerMean gate uses, applied to
// task-level means instead of run-level means.
func (p g1TaskPair) pass() bool {
	return g1Pass(p.ScMean, p.GxMean)
}

// g1TaskPairs builds the paired task cells present in BOTH profiles, sorted
// by task ID for deterministic output, plus the IDs of unpaired tasks
// (episodes in only one profile) so the verdict reports them instead of
// silently dropping them.
func g1TaskPairs(scEps, gxEps []*Episode) (pairs []g1TaskPair, unpaired []string) {
	scByTask := correctnessByTask(scEps)
	gxByTask := correctnessByTask(gxEps)
	ids := map[string]bool{}
	for id := range scByTask {
		ids[id] = true
	}
	for id := range gxByTask {
		ids[id] = true
	}
	var all []string
	for id := range ids {
		all = append(all, id)
	}
	sort.Strings(all)
	for _, id := range all {
		sc, scOK := scByTask[id]
		gx, gxOK := gxByTask[id]
		if !scOK || !gxOK {
			unpaired = append(unpaired, id)
			continue
		}
		pairs = append(pairs, g1TaskPair{TaskID: id, ScMean: mean(sc), GxMean: mean(gx)})
	}
	return pairs, unpaired
}

// correctnessByTask groups per-episode correctness rewards by task ID.
func correctnessByTask(eps []*Episode) map[string][]float64 {
	out := map[string][]float64{}
	for _, ep := range eps {
		out[ep.TaskID] = append(out[ep.TaskID], ep.Rewards.Correctness)
	}
	return out
}

// g3Pass is the G3 gate condition, factored out so the fragility check
// below reuses the exact comparison the real gate uses.
func g3Pass(scMean, gxMean float64) bool {
	return gxMean > 0 && scMean <= g3Factor*gxMean
}

// g3Fragile is G3's leave-one-out fragility check. Main-agent chars are an
// unbounded metric (no fixed ceiling), so there is no finite worst-case
// single-trial value to substitute in — unlike G1/G2/G5, extremization to a
// domain bound does not apply (stopping.go's boundG3 documents the same
// limitation for sequential-stopping bounds). Instead, G3 asks the
// meaningful question for an unbounded metric: did this one *observed*
// trial, not a hypothetical extreme one, decide the outcome — by excluding
// each trial in turn (both its value and its weight in the mean) and
// checking whether Pass changes.
func g3Fragile(scVals, gxVals []float64, passNow bool) (bool, string) {
	if len(scVals) == 0 || len(gxVals) == 0 {
		return false, ""
	}
	if len(scVals) > 1 {
		baseGx := mean(gxVals)
		for i := range scVals {
			newSc := meanExcluding(scVals, i)
			if g3Pass(newSc, baseGx) != passNow {
				return true, fmt.Sprintf(
					"excluding one sidecar trial (%.0f chars) changes the sidecar mean to %.0f, flipping G3 against ghx %.0f",
					scVals[i], newSc, baseGx)
			}
		}
	}
	if len(gxVals) > 1 {
		baseSc := mean(scVals)
		for i := range gxVals {
			newGx := meanExcluding(gxVals, i)
			if g3Pass(baseSc, newGx) != passNow {
				return true, fmt.Sprintf(
					"excluding one ghx trial (%.0f chars) changes the ghx mean to %.0f, flipping G3 for sidecar %.0f",
					gxVals[i], newGx, baseSc)
			}
		}
	}
	return false, ""
}

// Per-metric value extraction, shared by the fragility checks above.

func correctnessValues(eps []*Episode) []float64 {
	out := make([]float64, len(eps))
	for i, ep := range eps {
		out[i] = ep.Rewards.Correctness
	}
	return out
}

func evidenceValues(eps []*Episode) []float64 {
	out := make([]float64, len(eps))
	for i, ep := range eps {
		out[i] = ep.Rewards.Evidence
	}
	return out
}

func safetyValues(eps []*Episode) []float64 {
	out := make([]float64, len(eps))
	for i, ep := range eps {
		out[i] = ep.Rewards.Safety
	}
	return out
}

func mainCharsValues(eps []*Episode) []float64 {
	out := make([]float64, len(eps))
	for i, ep := range eps {
		out[i] = float64(ep.Context.MainAgentChars)
	}
	return out
}

// profileEpisodes returns the episodes belonging to one profile, preserving
// order.
func profileEpisodes(episodes []*Episode, p Profile) []*Episode {
	var out []*Episode
	for _, ep := range episodes {
		if ep.Profile == p {
			out = append(out, ep)
		}
	}
	return out
}

// multiTurnEpisodes returns the episodes with more than one turn, matching
// Aggregate()'s multi-turn accumulation.
func multiTurnEpisodes(episodes []*Episode) []*Episode {
	var out []*Episode
	for _, ep := range episodes {
		if len(ep.Turns) > 1 {
			out = append(out, ep)
		}
	}
	return out
}
