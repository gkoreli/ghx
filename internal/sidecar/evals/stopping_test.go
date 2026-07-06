package evals

import (
	"strings"
	"testing"
)

// stopEp builds a minimal scored episode for stopping-bounds tests.
func stopEp(p Profile, corr, evid, safety float64, mainChars int) *Episode {
	return &Episode{
		TaskID:  "task-a",
		Profile: p,
		Rewards: RewardBreakdown{Correctness: corr, Evidence: evid, Safety: safety},
		Context: ContextAccounting{MainAgentChars: mainChars},
	}
}

func boundFor(s StoppingBounds, id string) GateBound {
	for _, b := range s.Bounds {
		if b.ID == id {
			return b
		}
	}
	return GateBound{ID: id + "(missing)"}
}

// TestStoppingG1SuccessLock: sidecar correctness so far dominates ghx by enough
// that even the worst remaining sidecar and best remaining ghx keep it passing.
func TestStoppingG1SuccessLock(t *testing.T) {
	// expected 30 total → 10 per profile. 8 done each, 2 remaining each.
	var eps []*Episode
	for i := 0; i < 8; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 1.0, 1.0, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.30, 0.5, 1.0, 1000))
	}
	s := ComputeStoppingBounds(eps, 30)
	g1 := boundFor(s, "G1")
	if g1.Direction != StopSuccess {
		t.Fatalf("G1 should success-lock, got %q: %s", g1.Direction, g1.Detail)
	}
}

// TestStoppingG1FutilityLock: sidecar correctness is so low that even perfect
// remaining sidecar cannot clear the absolute floor / the ghx bar.
func TestStoppingG1FutilityLock(t *testing.T) {
	var eps []*Episode
	// 9 sidecar done at 0.0, 1 remaining → best-case mean 0.1 < abs floor 0.60.
	for i := 0; i < 9; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 0.0, 0.9, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
	}
	s := ComputeStoppingBounds(eps, 30)
	g1 := boundFor(s, "G1")
	if g1.Direction != StopFutility {
		t.Fatalf("G1 should futility-lock, got %q: %s", g1.Direction, g1.Detail)
	}
	if s.Recommendation != recStopFutility {
		t.Fatalf("recommendation should be STOP-FUTILITY, got %s", s.Recommendation)
	}
	if s.LockedAt == 0 {
		t.Fatal("LockedAt should record the episode count when the lock fired")
	}
}

// TestStoppingG2BothDirections locks G2 success and futility on evidence.
func TestStoppingG2BothDirections(t *testing.T) {
	// Success: 8 sidecar at evidence 1.0, 2 remaining → worst 0.8 ≥ 0.70.
	var pass []*Episode
	for i := 0; i < 8; i++ {
		pass = append(pass, stopEp(ProfileSidecar, 0.9, 1.0, 1.0, 100))
		pass = append(pass, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
	}
	if g2 := boundFor(ComputeStoppingBounds(pass, 30), "G2"); g2.Direction != StopSuccess {
		t.Fatalf("G2 should success-lock, got %q: %s", g2.Direction, g2.Detail)
	}

	// Futility: 9 sidecar at evidence 0.0, 1 remaining → best 0.1 < 0.70.
	var fail []*Episode
	for i := 0; i < 9; i++ {
		fail = append(fail, stopEp(ProfileSidecar, 0.9, 0.0, 1.0, 100))
		fail = append(fail, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
	}
	if g2 := boundFor(ComputeStoppingBounds(fail, 30), "G2"); g2.Direction != StopFutility {
		t.Fatalf("G2 should futility-lock, got %q: %s", g2.Direction, g2.Detail)
	}
}

// TestStoppingNearThresholdDoesNotLock: means straddle the thresholds so
// remaining episodes can still flip every bounded gate.
func TestStoppingNearThreshold(t *testing.T) {
	// 5 done each of 10 planned per profile → 5 remaining, wide open.
	var eps []*Episode
	for i := 0; i < 5; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 0.7, 0.72, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.75, 0.7, 1.0, 300))
	}
	s := ComputeStoppingBounds(eps, 30)
	for _, id := range []string{"G1", "G2"} {
		if b := boundFor(s, id); b.Locked {
			t.Errorf("%s must NOT lock near threshold: %s", id, b.Detail)
		}
	}
	if s.Recommendation != recContinue {
		t.Fatalf("near-threshold run should CONTINUE, got %s", s.Recommendation)
	}
}

// TestStoppingG5FutilityOnExistingViolation: a single safety violation locks
// G5 futility regardless of remaining, even with an unknown remaining count.
func TestStoppingG5FutilityViolation(t *testing.T) {
	eps := []*Episode{
		stopEp(ProfileSidecar, 0.9, 0.9, 1.0, 100),
		stopEp(ProfileSidecar, 0.9, 0.9, 0.0, 100), // violation
		stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000),
	}
	// Unknown remaining (expectedEpisodes = 0) still locks G5 futility.
	s := ComputeStoppingBounds(eps, 0)
	g5 := boundFor(s, "G5")
	if g5.Direction != StopFutility {
		t.Fatalf("G5 should futility-lock on a violation, got %q: %s", g5.Direction, g5.Detail)
	}
	if s.Recommendation != recStopFutility {
		t.Fatalf("recommendation should be STOP-FUTILITY, got %s", s.Recommendation)
	}
}

// TestStoppingG5NoSuccessLockBeforeCompletion: clean safety so far must NOT
// success-lock while sidecar episodes remain — a future episode can still fail.
func TestStoppingG5NoEarlySuccess(t *testing.T) {
	var eps []*Episode
	for i := 0; i < 5; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 0.9, 0.9, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
	}
	s := ComputeStoppingBounds(eps, 30) // 5 remaining per profile
	g5 := boundFor(s, "G5")
	if g5.Locked {
		t.Fatalf("G5 must not lock before completion, got %q: %s", g5.Direction, g5.Detail)
	}
}

// TestStoppingG3AndG5LockAtCompletion: with zero remaining, the unbounded
// gates resolve to their final pass/fail.
func TestStoppingCompletionLocks(t *testing.T) {
	// 10 per profile planned, 10 done → complete. sidecar chars small, ghx big.
	var eps []*Episode
	for i := 0; i < 10; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 1.0, 1.0, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
		eps = append(eps, stopEp(ProfilePlain, 0.5, 0.4, 1.0, 2000))
	}
	s := ComputeStoppingBounds(eps, 30)
	if !s.Complete {
		t.Fatal("run should be complete when no episodes remain")
	}
	if g3 := boundFor(s, "G3"); g3.Direction != StopSuccess {
		t.Fatalf("G3 should success-lock at completion (100 ≤ 0.35×1000), got %q: %s", g3.Direction, g3.Detail)
	}
	if g5 := boundFor(s, "G5"); g5.Direction != StopSuccess {
		t.Fatalf("G5 should success-lock at completion, got %q: %s", g5.Direction, g5.Detail)
	}
	if s.Recommendation != recStopSuccessLock {
		t.Fatalf("all thesis gates locked → STOP-SUCCESS-LOCKED, got %s", s.Recommendation)
	}
}

// TestStoppingG3UnboundedBeforeCompletion: chars can't be bounded mid-run.
func TestStoppingG3Unbounded(t *testing.T) {
	var eps []*Episode
	for i := 0; i < 5; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 1.0, 1.0, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
	}
	s := ComputeStoppingBounds(eps, 30) // remaining > 0
	if g3 := boundFor(s, "G3"); g3.Locked {
		t.Fatalf("G3 must not lock before completion (unbounded metric): %s", g3.Detail)
	}
}

// TestStoppingSectionRendersInVerdict shows the verdict markdown section.
func TestStoppingSectionRendersInVerdict(t *testing.T) {
	var eps []*Episode
	for i := 0; i < 9; i++ {
		eps = append(eps, stopEp(ProfileSidecar, 0.0, 0.9, 1.0, 100))
		eps = append(eps, stopEp(ProfileGhx, 0.9, 0.9, 1.0, 1000))
	}
	v := EvaluateGates(eps)
	stopping := ComputeStoppingBounds(eps, 30)
	v.Stopping = &stopping
	md := FormatVerdict(v)
	if !strings.Contains(md, "## Sequential stopping") {
		t.Fatalf("verdict missing sequential-stopping section:\n%s", md)
	}
	if !strings.Contains(md, "FUTILITY-LOCKED") {
		t.Fatalf("expected a futility lock in the section:\n%s", md)
	}
	if !strings.Contains(md, "STOP-FUTILITY") {
		t.Fatalf("expected STOP-FUTILITY recommendation:\n%s", md)
	}
	if !strings.Contains(md, "does not auto-stop") {
		t.Fatalf("section must state the runner does not auto-stop:\n%s", md)
	}
	// A nil Stopping must leave the verdict unchanged (backward compatible).
	v.Stopping = nil
	if strings.Contains(FormatVerdict(v), "## Sequential stopping") {
		t.Fatal("nil Stopping must not render the section")
	}
	t.Logf("sample section:\n%s", formatStoppingSection(&stopping))
}
