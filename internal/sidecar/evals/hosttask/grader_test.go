package hosttask

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// fakeRuntime is a hermetic ContainerRuntime. Exit codes per command are
// scripted as sequences consumed across grade attempts (the last value
// repeats), which lets tests stage stable failures and flakes.
type fakeRuntime struct {
	availableErr error
	execInfraErr map[string]error
	exits        map[string][]int
	counts       map[string]int
	starts       []ContainerSpec
	execs        []string
	removed      []string
}

func (r *fakeRuntime) Available(context.Context) error { return r.availableErr }

func (r *fakeRuntime) Start(_ context.Context, spec ContainerSpec) (string, error) {
	r.starts = append(r.starts, spec)
	return fmt.Sprintf("ctr-%d", len(r.starts)), nil
}

func (r *fakeRuntime) Exec(_ context.Context, _ string, cmd string) (ExecResult, error) {
	r.execs = append(r.execs, cmd)
	if err := r.execInfraErr[cmd]; err != nil {
		return ExecResult{}, err
	}
	if r.counts == nil {
		r.counts = map[string]int{}
	}
	seq := r.exits[cmd]
	code := 0
	if len(seq) > 0 {
		idx := r.counts[cmd]
		if idx >= len(seq) {
			idx = len(seq) - 1
		}
		code = seq[idx]
	}
	r.counts[cmd]++
	return ExecResult{ExitCode: code, Output: "output of " + cmd}, nil
}

func (r *fakeRuntime) Remove(_ context.Context, id string) error {
	r.removed = append(r.removed, id)
	return nil
}

// gradeFixture returns a fixture with two commands per phase, exercising
// fraction math.
func gradeFixture() Fixture {
	f := validFixture()
	f.SetupCmds = []string{"setup-a", "setup-b"}
	f.FailToPassCmds = []string{"f2p-1", "f2p-2"}
	f.PassToPassCmds = []string{"p2p-1", "p2p-2"}
	return f
}

// TestGradeMath pins the GH1 grade form: grade = failToPass fraction ×
// passToPass preservation, on stable outcomes.
func TestGradeMath(t *testing.T) {
	cases := []struct {
		name             string
		exits            map[string][]int
		wantF2P, wantP2P float64
		wantGrade        float64
		wantAttempts     int
	}{
		{"all pass", nil, 1, 1, 1, 1},
		{"half f2p", map[string][]int{"f2p-1": {1}}, 0.5, 1, 0.5, 3},
		{"half p2p", map[string][]int{"p2p-2": {2}}, 1, 0.5, 0.5, 3},
		{"half both", map[string][]int{"f2p-1": {1}, "p2p-1": {1}}, 0.5, 0.5, 0.25, 3},
		{"fix absent, no regressions", map[string][]int{"f2p-1": {1}, "f2p-2": {1}}, 0, 1, 0, 3},
		{"fix present, all regressed", map[string][]int{"p2p-1": {1}, "p2p-2": {1}}, 1, 0, 0, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rt := &fakeRuntime{exits: c.exits}
			res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
			if err != nil {
				t.Fatal(err)
			}
			if res.Flaky || res.SetupFailed {
				t.Fatalf("stable outcomes misflagged: %+v", res)
			}
			if res.F2PFraction != c.wantF2P || res.P2PPreservation != c.wantP2P || res.Grade != c.wantGrade {
				t.Errorf("grade = %v (f2p %v, p2p %v), want %v (f2p %v, p2p %v)",
					res.Grade, res.F2PFraction, res.P2PPreservation, c.wantGrade, c.wantF2P, c.wantP2P)
			}
			if len(res.Attempts) != c.wantAttempts {
				t.Errorf("attempts = %d, want %d", len(res.Attempts), c.wantAttempts)
			}
		})
	}
}

// TestGradeFlakeRule pins ADR-0032 trap 7: any per-command flip across the
// three attempts disqualifies the task — Flaky set, grade never computed.
func TestGradeFlakeRule(t *testing.T) {
	t.Run("fail then pass flips", func(t *testing.T) {
		rt := &fakeRuntime{exits: map[string][]int{"f2p-1": {1, 0, 0}}}
		res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Flaky {
			t.Fatal("flip across re-runs must mark the task flaky")
		}
		if len(res.Attempts) != 3 {
			t.Errorf("attempts = %d, want 3", len(res.Attempts))
		}
		if res.Grade != 0 || res.F2PFraction != 0 || res.P2PPreservation != 0 {
			t.Errorf("flaky result must never carry a score: %+v", res)
		}
	})

	t.Run("flip only on last re-run still disqualifies", func(t *testing.T) {
		rt := &fakeRuntime{exits: map[string][]int{"p2p-1": {1, 1, 0}}}
		res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
		if err != nil {
			t.Fatal(err)
		}
		if !res.Flaky {
			t.Error("third-attempt flip must mark the task flaky")
		}
	})

	t.Run("stable failure is a real grade, not a flake", func(t *testing.T) {
		rt := &fakeRuntime{exits: map[string][]int{"f2p-2": {1, 1, 1}}}
		res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
		if err != nil {
			t.Fatal(err)
		}
		if res.Flaky || res.Grade != 0.5 {
			t.Errorf("stable failure: flaky=%v grade=%v, want false/0.5", res.Flaky, res.Grade)
		}
	})

	t.Run("clean first attempt is final — no re-runs", func(t *testing.T) {
		// Exit sequences that WOULD flip are never observed because the
		// pre-registered rule re-runs only on failure.
		rt := &fakeRuntime{exits: map[string][]int{"f2p-1": {0, 1, 1}}}
		res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Attempts) != 1 || res.Flaky || res.Grade != 1 {
			t.Errorf("clean first attempt must be final: attempts=%d flaky=%v grade=%v",
				len(res.Attempts), res.Flaky, res.Grade)
		}
	})
}

// TestGradeSetupFailure pins setup semantics: a stable setup failure aborts
// each attempt's remaining commands (recorded as not-run) and grades an
// honest 0 with SetupFailed distinguishing "no test signal".
func TestGradeSetupFailure(t *testing.T) {
	rt := &fakeRuntime{exits: map[string][]int{"setup-a": {7}}}
	res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
	if err != nil {
		t.Fatal(err)
	}
	if !res.SetupFailed || res.Flaky {
		t.Fatalf("want stable SetupFailed, got %+v", res)
	}
	if res.Grade != 0 || res.F2PFraction != 0 || res.P2PPreservation != 0 {
		t.Errorf("setup failure must grade 0: %+v", res)
	}
	if len(res.Attempts) != 3 {
		t.Errorf("attempts = %d, want 3 (failure triggers re-runs)", len(res.Attempts))
	}
	for _, c := range res.Attempts[0].Commands {
		switch c.Cmd {
		case "setup-a":
			if !c.Ran || c.Pass || c.ExitCode != 7 {
				t.Errorf("setup-a = %+v, want ran/failed/exit 7", c)
			}
		default:
			if c.Ran || c.Pass {
				t.Errorf("%s must not run after setup failure: %+v", c.Cmd, c)
			}
		}
	}
	// Only setup-a executed per attempt.
	if len(rt.execs) != 3 {
		t.Errorf("execs = %v, want setup-a ×3", rt.execs)
	}
}

// TestGradeSetupFlake: a setup command flipping across attempts is a flake
// like any other — the task is disqualified.
func TestGradeSetupFlake(t *testing.T) {
	rt := &fakeRuntime{exits: map[string][]int{"setup-b": {1, 0, 0}}}
	res, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Flaky {
		t.Error("setup flip must mark the task flaky")
	}
}

// TestGradeOrderingAndContainerSpec pins execution order (setup → failToPass
// → passToPass, each list in fixture order), one fresh container per
// attempt, the bind-mount spec, and the network default.
func TestGradeOrderingAndContainerSpec(t *testing.T) {
	rt := &fakeRuntime{}
	f := gradeFixture()
	res, err := (&Grader{Runtime: rt}).Grade(context.Background(), f, "/trial/ws")
	if err != nil {
		t.Fatal(err)
	}
	wantOrder := []string{"setup-a", "setup-b", "f2p-1", "f2p-2", "p2p-1", "p2p-2"}
	if strings.Join(rt.execs, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("exec order = %v, want %v", rt.execs, wantOrder)
	}
	if len(rt.starts) != 1 || len(rt.removed) != 1 {
		t.Errorf("containers: starts=%d removed=%d, want 1/1", len(rt.starts), len(rt.removed))
	}
	spec := rt.starts[0]
	if spec.Image != f.Image || spec.WorkspaceDir != "/trial/ws" || spec.Network != "none" {
		t.Errorf("container spec = %+v", spec)
	}
	if res.FixtureID != f.ID || res.Image != f.Image || res.WorkspaceDir != "/trial/ws" {
		t.Errorf("result identity = %+v", res)
	}
	// Per-command evidence is retained.
	for i, c := range res.Attempts[0].Commands {
		if c.Cmd != wantOrder[i] || !c.Ran || !c.Pass || c.Output == "" {
			t.Errorf("command %d evidence incomplete: %+v", i, c)
		}
	}

	// A failure means one fresh container per attempt.
	rt = &fakeRuntime{exits: map[string][]int{"f2p-1": {1}}}
	if _, err := (&Grader{Runtime: rt}).Grade(context.Background(), f, "/trial/ws"); err != nil {
		t.Fatal(err)
	}
	if len(rt.starts) != 3 || len(rt.removed) != 3 {
		t.Errorf("re-run containers: starts=%d removed=%d, want 3/3", len(rt.starts), len(rt.removed))
	}
}

// TestGradeDockerUnavailable pins the typed error contract: the eval layer
// must be able to detect a missing engine with errors.Is.
func TestGradeDockerUnavailable(t *testing.T) {
	rt := &fakeRuntime{availableErr: fmt.Errorf("%w: daemon not running", ErrDockerUnavailable)}
	_, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws")
	if !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("want ErrDockerUnavailable, got %v", err)
	}
	if len(rt.starts) != 0 {
		t.Error("no container may start when the engine is unavailable")
	}
}

// TestDockerCLIUnavailableIsTyped runs the real DockerCLI against a binary
// that cannot exist: the failure must be the typed sentinel, hermetically.
func TestDockerCLIUnavailableIsTyped(t *testing.T) {
	d := DockerCLI{Bin: "ghx-hosttask-no-such-engine"}
	if err := d.Available(context.Background()); !errors.Is(err, ErrDockerUnavailable) {
		t.Fatalf("want ErrDockerUnavailable, got %v", err)
	}
}

func TestGradeInfraErrorPropagates(t *testing.T) {
	rt := &fakeRuntime{execInfraErr: map[string]error{"f2p-1": errors.New("engine crashed")}}
	if _, err := (&Grader{Runtime: rt}).Grade(context.Background(), gradeFixture(), "/ws"); err == nil {
		t.Fatal("engine failure must be an error, never a grade")
	}
}

func TestGradeRejectsInvalidFixture(t *testing.T) {
	f := gradeFixture()
	f.FailToPassCmds = nil
	rt := &fakeRuntime{}
	if _, err := (&Grader{Runtime: rt}).Grade(context.Background(), f, "/ws"); err == nil {
		t.Fatal("want validation error")
	}
	if len(rt.starts) != 0 || len(rt.execs) != 0 {
		t.Error("invalid fixture must not touch the runtime")
	}
}

func TestCapTail(t *testing.T) {
	if got := capTail("short"); got != "short" {
		t.Errorf("small output altered: %q", got)
	}
	big := strings.Repeat("x", maxOutputBytes+100)
	got := capTail(big)
	if !strings.HasPrefix(got, "[... output truncated ...]") {
		t.Error("truncated output must carry the marker")
	}
	if len(got) > maxOutputBytes+64 {
		t.Errorf("capped output still %d bytes", len(got))
	}
}
