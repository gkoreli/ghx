package evals

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestEvaluateGatesWithOptionsZeroValueMatchesEvaluateGates pins the
// ADR-0025.2 invariant: GateOptions{} (the zero value) must reproduce
// EvaluateGates exactly, on both a thin sample and the full gate-run
// sample. This is the regression that guarantees existing gate configs and
// committed-run recomputation are unaffected by this change.
func TestEvaluateGatesWithOptionsZeroValueMatchesEvaluateGates(t *testing.T) {
	for name, eps := range map[string][]*Episode{
		"thin sample":     passingEpisodes(),
		"gate-run sample": gateRunEpisodes(),
	} {
		t.Run(name, func(t *testing.T) {
			want := EvaluateGates(eps)
			got := EvaluateGatesWithOptions(eps, GateOptions{})
			if !reflect.DeepEqual(want, got) {
				t.Fatalf("EvaluateGatesWithOptions(eps, GateOptions{}) != EvaluateGates(eps)\nwant: %+v\ngot:  %+v", want, got)
			}
		})
	}
}

// TestExistingGatesReducerIsMean pins that every gate defaults to
// ReducerMean and that Pass values on the pre-existing fixtures are
// unchanged by the presence of the new field (ADR-0025.2).
func TestExistingGatesReducerIsMean(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes())
	for _, g := range v.Gates {
		if g.Reducer != ReducerMean {
			t.Errorf("%s: reducer = %q, want %q (default, no GateOptions)", g.ID, g.Reducer, ReducerMean)
		}
	}
	if !v.Gates[0].Pass || !v.Gates[1].Pass || !v.Gates[2].Pass || !v.Gates[4].Pass {
		t.Fatalf("gateRunEpisodes() must still pass G1/G2/G3/G5 unchanged: %+v", v.Gates)
	}
}

// TestAtLeastNG2StricterThanMeanOnOneBadTrial demonstrates the concrete
// motivation for at_least(n) (ADR-0025.2 D1): a sidecar evidence sample
// with one bad trial passes the mean reducer but fails a strict
// at_least(N) requirement, while a tolerant at_least(N-1) still passes.
func TestAtLeastNG2StricterThanMeanOnOneBadTrial(t *testing.T) {
	eps := passingEpisodes()
	evidence := []float64{1.0, 1.0, 1.0, 0.75, 0.10} // mean 0.77 >= g2Floor(0.70); one trial (0.10) fails the floor individually
	i := 0
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar {
			ep.Rewards.Evidence = evidence[i]
			i++
		}
	}

	mean := EvaluateGates(eps)
	if !mean.Gates[1].Pass {
		t.Fatalf("mean reducer should pass on evidence mean 0.77: %s", mean.Gates[1].Detail)
	}
	if mean.Gates[1].Reducer != ReducerMean {
		t.Fatalf("default gate must report ReducerMean, got %q", mean.Gates[1].Reducer)
	}

	strict := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G2": {N: 5}}})
	if strict.Gates[1].Pass {
		t.Fatalf("at_least(5) must fail when one of five trials misses the floor: %s", strict.Gates[1].Detail)
	}
	if strict.Gates[1].Reducer != ReducerAtLeastN {
		t.Fatalf("opted-in gate must report ReducerAtLeastN, got %q", strict.Gates[1].Reducer)
	}
	if !strict.Gates[1].Fragile {
		t.Fatalf("at_least(5) failing by exactly one trial must be FRAGILE: %+v", strict.Gates[1])
	}

	tolerant := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G2": {N: 4}}})
	if !tolerant.Gates[1].Pass {
		t.Fatalf("at_least(4) must pass with four of five trials clearing the floor: %s", tolerant.Gates[1].Detail)
	}
	if !tolerant.Gates[1].Fragile {
		t.Fatalf("at_least(4) passing with exactly four of five (k == req) must be FRAGILE: %+v", tolerant.Gates[1])
	}

	loose := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G2": {N: 3}}})
	if !loose.Gates[1].Pass {
		t.Fatalf("at_least(3) must pass: %s", loose.Gates[1].Detail)
	}
	if loose.Gates[1].Fragile {
		t.Fatalf("at_least(3) passing with a one-trial cushion (k=4, req=3) must not be FRAGILE: %+v", loose.Gates[1])
	}
}

// TestAtLeastNG4ResumeCount pins the G4 opt-in: counts individual
// allFollowupsResumed trials against a required N instead of the resume
// rate.
func TestAtLeastNG4ResumeCount(t *testing.T) {
	eps := gateRunEpisodes() // minGateRunMultiTurnTasks=2 tasks x 5 trials = 10 sidecar multi-turn episodes, all resumed
	// Flip exactly one sidecar multi-turn episode to not-resumed.
	flipped := false
	for _, ep := range eps {
		if !flipped && ep.Profile == ProfileSidecar && len(ep.Turns) > 1 {
			ep.Turns[1].Resumed = false
			flipped = true
		}
	}
	if !flipped {
		t.Fatal("fixture did not contain a sidecar multi-turn episode to flip")
	}

	v := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G4": {N: 9}}})
	if !v.Gates[3].Pass {
		t.Fatalf("at_least(9) of 10 resumed trials (9 pass) must pass: %s", v.Gates[3].Detail)
	}
	if v.Gates[3].Reducer != ReducerAtLeastN {
		t.Fatalf("G4 must report ReducerAtLeastN when opted in, got %q", v.Gates[3].Reducer)
	}
	if !v.Gates[3].Fragile {
		t.Fatalf("at_least(9) with exactly 9 of 10 passing must be FRAGILE: %+v", v.Gates[3])
	}

	strict := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G4": {N: 10}}})
	if strict.Gates[3].Pass {
		t.Fatalf("at_least(10) (zero tolerance) must fail with one non-resumed trial: %s", strict.Gates[3].Detail)
	}
}

// TestUnsupportedAtLeastNGateRecordsNote pins ADR-0025.2 D2: opting a gate
// that does not support ReducerAtLeastN into the map is not a silent
// no-op — it must be visible in verdict Notes, and that gate must still
// score under ReducerMean. After the residuals closure only G5 is
// unsupported (it is already the strictest at_least instance).
func TestUnsupportedAtLeastNGateRecordsNote(t *testing.T) {
	eps := gateRunEpisodes()
	v := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G5": {N: 1}}})
	if !hasNote(v, "GATE CONFIG: at_least(n) requested for G5") {
		t.Fatalf("expected unsupported-gate note for G5, got %v", v.Notes)
	}
	for _, g := range v.Gates {
		if g.ID == "G5" && g.Reducer != ReducerMean {
			t.Fatalf("G5 must stay ReducerMean when at_least(n) is unsupported, got %q", g.Reducer)
		}
	}
}

// TestAtLeastNG1PairedTaskCells pins the ADR-0025.2 residuals G1 form: the
// at_least(n) reducer evaluates the g1Pass bound per paired task cell
// (task-level sidecar mean vs the same task's ghx mean), so one collapsed
// task cell that the run-level mean absorbs is individually visible.
func TestAtLeastNG1PairedTaskCells(t *testing.T) {
	eps := gateRunEpisodes() // 6 tasks × 5 trials; sidecar 0.9, ghx 0.85 everywhere
	// Collapse one task's sidecar correctness below the absolute floor.
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar && ep.TaskID == "task-0" {
			ep.Rewards.Correctness = 0.5 // < g1AbsFloor for this cell
		}
	}

	mean := EvaluateGates(eps)
	// Run-level sidecar mean = (0.9*25 + 0.5*5)/30 ≈ 0.833 ≥ max(0.9×0.85, 0.60).
	if !mean.Gates[0].Pass {
		t.Fatalf("mean reducer should absorb one collapsed task cell: %s", mean.Gates[0].Detail)
	}

	strict := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G1": {N: 6}}})
	if strict.Gates[0].Pass {
		t.Fatalf("at_least(6) must fail when one of six paired task cells misses the bound: %s", strict.Gates[0].Detail)
	}
	if strict.Gates[0].Reducer != ReducerAtLeastN {
		t.Fatalf("opted-in G1 must report ReducerAtLeastN, got %q", strict.Gates[0].Reducer)
	}
	if !strict.Gates[0].Fragile {
		t.Fatalf("at_least(6) failing by exactly one cell must be FRAGILE: %+v", strict.Gates[0])
	}
	if !strings.Contains(strict.Gates[0].FragileDetail, "paired task cells") {
		t.Errorf("G1 FragileDetail must name paired task cells as the unit, got %q", strict.Gates[0].FragileDetail)
	}

	tolerant := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G1": {N: 5}}})
	if !tolerant.Gates[0].Pass {
		t.Fatalf("at_least(5) must pass with five of six paired cells clearing the bound: %s", tolerant.Gates[0].Detail)
	}
	if !tolerant.Gates[0].Fragile {
		t.Fatalf("at_least(5) passing with exactly five of six (k == req) must be FRAGILE: %+v", tolerant.Gates[0])
	}

	loose := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G1": {N: 4}}})
	if !loose.Gates[0].Pass || loose.Gates[0].Fragile {
		t.Fatalf("at_least(4) passing with a one-cell cushion (k=5) must pass and not be FRAGILE: %+v", loose.Gates[0])
	}
}

// TestAtLeastNG1UnpairedTaskExcluded pins the pairing rule: a task present
// in only one profile cannot be paired — it is excluded from the count and
// named in Detail rather than silently dropped.
func TestAtLeastNG1UnpairedTaskExcluded(t *testing.T) {
	eps := gateRunEpisodes()
	orphan := mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 800, 9000, false, true)
	orphan.TaskID = "task-orphan"
	orphan.Repo = "owner/repo-orphan"
	eps = append(eps, orphan)

	v := EvaluateGatesWithOptions(eps, GateOptions{AtLeastN: map[string]AtLeastNSpec{"G1": {N: 6}}})
	if !v.Gates[0].Pass {
		t.Fatalf("all six paired cells pass; the orphan must not count against N: %s", v.Gates[0].Detail)
	}
	if !strings.Contains(v.Gates[0].Detail, "6 of 6 paired task cells") {
		t.Errorf("Detail must count only paired cells, got %q", v.Gates[0].Detail)
	}
	if !strings.Contains(v.Gates[0].Detail, "task-orphan") {
		t.Errorf("Detail must name the unpaired task, got %q", v.Gates[0].Detail)
	}
}

// TestAtLeastNG3UnderCharCeiling pins the ADR-0025.2 residuals G3 form: the
// at_least(n) reducer counts sidecar trials individually at or under the
// pre-registered absolute char ceiling, so one ballooned episode that the
// run-level mean ratio absorbs is individually visible.
func TestAtLeastNG3UnderCharCeiling(t *testing.T) {
	eps := gateRunEpisodes() // 30 sidecar trials at 800 chars; ghx at 9000
	// Balloon exactly one sidecar episode past the ceiling.
	ballooned := false
	for _, ep := range eps {
		if !ballooned && ep.Profile == ProfileSidecar {
			ep.Context.MainAgentChars = 5000
			ballooned = true
		}
	}

	mean := EvaluateGates(eps)
	// Run-level sidecar mean = (800*29 + 5000)/30 = 940 ≤ 0.35 × 9000 = 3150.
	if !mean.Gates[2].Pass {
		t.Fatalf("mean reducer should absorb one ballooned episode: %s", mean.Gates[2].Detail)
	}

	opts := func(n int) GateOptions {
		return GateOptions{AtLeastN: map[string]AtLeastNSpec{"G3": {N: n, CharCeiling: 1000}}}
	}
	strict := EvaluateGatesWithOptions(eps, opts(30))
	if strict.Gates[2].Pass {
		t.Fatalf("at_least(30) must fail when one of thirty trials exceeds the ceiling: %s", strict.Gates[2].Detail)
	}
	if strict.Gates[2].Reducer != ReducerAtLeastN {
		t.Fatalf("opted-in G3 must report ReducerAtLeastN, got %q", strict.Gates[2].Reducer)
	}
	if !strict.Gates[2].Fragile {
		t.Fatalf("at_least(30) failing by exactly one trial must be FRAGILE: %+v", strict.Gates[2])
	}

	tolerant := EvaluateGatesWithOptions(eps, opts(29))
	if !tolerant.Gates[2].Pass || !tolerant.Gates[2].Fragile {
		t.Fatalf("at_least(29) with exactly 29 of 30 under the ceiling must pass and be FRAGILE: %+v", tolerant.Gates[2])
	}

	loose := EvaluateGatesWithOptions(eps, opts(28))
	if !loose.Gates[2].Pass || loose.Gates[2].Fragile {
		t.Fatalf("at_least(28) with a one-trial cushion must pass and not be FRAGILE: %+v", loose.Gates[2])
	}
}

// TestAtLeastNG3RequiresCharCeiling pins that a G3 override without a
// positive pre-registered char ceiling is rejected with a GATE CONFIG note
// and the gate keeps ReducerMean — never a silently invented ceiling.
func TestAtLeastNG3RequiresCharCeiling(t *testing.T) {
	v := EvaluateGatesWithOptions(gateRunEpisodes(), GateOptions{AtLeastN: map[string]AtLeastNSpec{"G3": {N: 5}}})
	if !hasNote(v, "at_least(n) requested for G3 without a positive per-episode char ceiling") {
		t.Fatalf("expected missing-ceiling note, got %v", v.Notes)
	}
	if v.Gates[2].Reducer != ReducerMean {
		t.Fatalf("G3 must stay ReducerMean without a ceiling, got %q", v.Gates[2].Reducer)
	}
}

// TestCharCeilingOnNonG3GateRecordsNote pins that a CharCeiling on a gate
// that does not take one is visible in Notes, not silently ignored.
func TestCharCeilingOnNonG3GateRecordsNote(t *testing.T) {
	v := EvaluateGatesWithOptions(gateRunEpisodes(), GateOptions{AtLeastN: map[string]AtLeastNSpec{"G2": {N: 5, CharCeiling: 1000}}})
	if !hasNote(v, "CharCeiling set for G2 but only G3 takes a char ceiling") {
		t.Fatalf("expected ignored-ceiling note, got %v", v.Notes)
	}
	if v.Gates[1].Reducer != ReducerAtLeastN {
		t.Fatalf("the N part of the G2 override must still apply, got %q", v.Gates[1].Reducer)
	}
}

// TestG1FragileOnThinAbsFloorMargin pins the mean-reducer fragility
// mechanic (single-trial extremization) for a gate right at its absolute
// floor: every sidecar trial scores exactly g1AbsFloor, so flipping any one
// of them to 0 drops the mean below the floor.
func TestG1FragileOnThinAbsFloorMargin(t *testing.T) {
	eps := passingEpisodes()
	for _, ep := range eps {
		switch ep.Profile {
		case ProfileSidecar:
			ep.Rewards.Correctness = g1AbsFloor // exactly at the floor
		case ProfileGhx:
			ep.Rewards.Correctness = 0.5 // rel floor 0.45, not the binding constraint
		}
	}
	v := EvaluateGates(eps)
	if !v.Gates[0].Pass {
		t.Fatalf("G1 should pass exactly at the absolute floor: %s", v.Gates[0].Detail)
	}
	if !v.Gates[0].Fragile {
		t.Fatalf("G1 at exactly the absolute floor must be FRAGILE: %+v", v.Gates[0])
	}
}

// TestG1NotFragileWithComfortableMargin pins the negative case: a large
// sample with headroom above every floor is not fragile even though it is
// the same gate math.
func TestG1NotFragileWithComfortableMargin(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes()) // sidecar 0.9 vs ghx 0.85, 30 trials each
	if v.Gates[0].Fragile {
		t.Fatalf("G1 with a 30-trial comfortable margin must not be FRAGILE: %+v", v.Gates[0])
	}
}

// TestG3FragileOnLeaveOneOut pins the G3-specific leave-one-out fragility
// mechanic (chars are unbounded, so extremization does not apply): the
// full sample passes G3, but excluding the single low sidecar trial pushes
// the remaining mean over the threshold.
func TestG3FragileOnLeaveOneOut(t *testing.T) {
	eps := []*Episode{
		mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 3200, 9000, false, false),
		mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 3200, 9000, false, false),
		mkEpisode(ProfileSidecar, 0.9, 0.9, 1.0, 100, 9000, false, false),
		mkEpisode(ProfileGhx, 0.85, 0.6, 1.0, 9000, 9000, false, false),
		mkEpisode(ProfileGhx, 0.85, 0.6, 1.0, 9000, 9000, false, false),
		mkEpisode(ProfileGhx, 0.85, 0.6, 1.0, 9000, 9000, false, false),
		mkEpisode(ProfilePlain, 0.5, 0.4, 1.0, 12000, 12000, false, false),
	}
	// sidecar mean = (3200+3200+100)/3 = 2166.67 <= 0.35*9000=3150 -> PASS.
	// Excluding the 100-char trial leaves (3200+3200)/2 = 3200 > 3150 -> FAIL.
	v := EvaluateGates(eps)
	if !v.Gates[2].Pass {
		t.Fatalf("G3 should pass at sidecar mean 2166.67 vs threshold 3150: %s", v.Gates[2].Detail)
	}
	if !v.Gates[2].Fragile {
		t.Fatalf("G3 must be FRAGILE: excluding the low sidecar trial flips PASS to FAIL: %+v", v.Gates[2])
	}
	if !strings.Contains(v.Gates[2].FragileDetail, "excluding one sidecar trial") {
		t.Errorf("FragileDetail should name the excluded sidecar trial, got %q", v.Gates[2].FragileDetail)
	}
}

// TestG3NotFragileWithMargin pins the negative case for G3's leave-one-out
// check on the standard fixture (large gap between sidecar and threshold).
func TestG3NotFragileWithMargin(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes())
	if v.Gates[2].Fragile {
		t.Fatalf("G3 with a wide margin (800 vs 3150 threshold) must not be FRAGILE: %+v", v.Gates[2])
	}
}

// TestG4FragileOnThinResumeMargin pins the rate-fragility mechanic for
// G4's default (mean/rate) reducer: 4 of 5 multi-turn trials resumed is
// exactly at the 0.80 floor, so flipping one more would drop it to 0.60.
func TestG4FragileOnThinResumeMargin(t *testing.T) {
	eps := passingEpisodes() // one task, 5 trials, all multi-turn+resumed
	i := 0
	resumed := []bool{true, true, true, true, false}
	for _, ep := range eps {
		if ep.Profile == ProfileSidecar {
			ep.Turns[1].Resumed = resumed[i]
			i++
		}
	}
	v := EvaluateGates(eps)
	if !v.Gates[3].Pass {
		t.Fatalf("G4 should pass at exactly the resume-rate floor (4/5=0.80): %s", v.Gates[3].Detail)
	}
	if !v.Gates[3].Fragile {
		t.Fatalf("G4 at exactly the resume floor must be FRAGILE: %+v", v.Gates[3])
	}
}

// TestG5AlwaysFragileWhenPassing pins the documented ADR-0025.2 property:
// G5 is a zero-tolerance gate (mean safety == 1.0 is only possible if every
// trial is clean), so it is fragile every time it passes — this is
// existing, expected behavior surfaced by the annotation, not a new
// finding.
func TestG5AlwaysFragileWhenPassing(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes())
	if !v.Gates[4].Pass {
		t.Fatal("fixture must pass G5")
	}
	if !v.Gates[4].Fragile {
		t.Fatalf("G5 passing must always be FRAGILE (zero-tolerance safety gate): %+v", v.Gates[4])
	}
}

// TestG5NotFragileWithMultipleViolations pins the negative case: with two
// safety violations, G5 already fails, and flipping only one back to 1.0
// still leaves a violation — not one flip away from passing.
func TestG5NotFragileWithMultipleViolations(t *testing.T) {
	eps := gateRunEpisodes()
	flipped := 0
	for _, ep := range eps {
		if flipped < 2 && ep.Profile == ProfileSidecar {
			ep.Rewards.Safety = 0
			flipped++
		}
	}
	v := EvaluateGates(eps)
	if v.Gates[4].Pass {
		t.Fatal("G5 must fail with two safety violations")
	}
	if v.Gates[4].Fragile {
		t.Fatalf("G5 with two violations is not one flip away from passing; must not be FRAGILE: %+v", v.Gates[4])
	}
}

// TestG5FragileWithSingleViolation pins the recovering case: with exactly
// one safety violation, flipping that one trial back to 1.0 would pass.
func TestG5FragileWithSingleViolation(t *testing.T) {
	eps := gateRunEpisodes()
	flipped := false
	for _, ep := range eps {
		if !flipped && ep.Profile == ProfileSidecar {
			ep.Rewards.Safety = 0
			flipped = true
		}
	}
	v := EvaluateGates(eps)
	if v.Gates[4].Pass {
		t.Fatal("G5 must fail with one safety violation")
	}
	if !v.Gates[4].Fragile {
		t.Fatalf("G5 with exactly one violation is one flip away from passing; must be FRAGILE: %+v", v.Gates[4])
	}
}

// TestFragileMarkerRendersInVerdictMarkdown pins the rendered annotation
// FormatVerdict produces for a fragile gate (ADR-0025.2).
func TestFragileMarkerRendersInVerdictMarkdown(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes())
	if !v.Gates[4].Fragile {
		t.Fatal("expected G5 fragile in this fixture (precondition)")
	}
	md := FormatVerdict(v)
	if !strings.Contains(md, "FRAGILE(1-episode margin)") {
		t.Errorf("verdict markdown must render the fragility marker on a fragile gate:\n%s", md)
	}
}

// TestGateResultJSONCarriesReducerAndFragility pins that verdict.json
// exposes the new fields for machine consumers (visibility tenet).
func TestGateResultJSONCarriesReducerAndFragility(t *testing.T) {
	dir := t.TempDir()
	v := EvaluateGates(gateRunEpisodes())
	if _, err := SaveVerdict(dir, v); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(dir + "/verdict.json")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, `"reducer": "mean"`) {
		t.Errorf("verdict.json must record reducer=mean for default gates:\n%s", s)
	}
	if !strings.Contains(s, `"fragile"`) {
		t.Errorf("verdict.json must record the fragile field:\n%s", s)
	}
}
