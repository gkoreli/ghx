package hosttask

import (
	"context"
	"fmt"
)

// Phase names the fixture command list a graded command came from.
type Phase string

const (
	// PhaseSetup is environment preparation; recorded but never scored.
	PhaseSetup Phase = "setup"
	// PhaseFailToPass is the fix-proving test set (SWE-bench F2P).
	PhaseFailToPass Phase = "failToPass"
	// PhasePassToPass is the no-regression test set (SWE-bench P2P).
	PhasePassToPass Phase = "passToPass"
)

// CommandResult is one command's outcome inside one grade attempt.
type CommandResult struct {
	// Phase is the command list the command belongs to.
	Phase Phase `json:"phase"`
	// Cmd is the fixture command as written.
	Cmd string `json:"cmd"`
	// Ran reports whether the command executed; false when a setup failure
	// aborted the attempt before it.
	Ran bool `json:"ran"`
	// Pass is true iff the command ran and exited zero.
	Pass bool `json:"pass"`
	// ExitCode is the shell exit status (meaningful only when Ran).
	ExitCode int `json:"exitCode"`
	// Output is the combined output, tail-capped (evidence, recomputable by
	// re-running the command in the fixture image).
	Output string `json:"output,omitempty"`
}

// Attempt is one full grader pass — setup, then failToPass, then passToPass,
// in fixture order, inside one fresh container.
type Attempt struct {
	// Commands holds every command's result in execution order.
	Commands []CommandResult `json:"commands"`
}

// failed reports whether any command in the attempt ran and failed, or was
// prevented from running — the condition that triggers the flake re-runs.
func (a Attempt) failed() bool {
	for _, c := range a.Commands {
		if !c.Ran || !c.Pass {
			return true
		}
	}
	return false
}

// Result is the deterministic outcome grade of one trial (ADR-0032.1 GH1).
// Grade, F2PFraction, and P2PPreservation are meaningful only when Flaky is
// false; a flaky result disqualifies the TASK and must never be scored
// (ADR-0032 trap 7).
type Result struct {
	// FixtureID is the graded fixture.
	FixtureID string `json:"fixtureId"`
	// Image is the container image the commands ran in (environment pin).
	Image string `json:"image"`
	// WorkspaceDir is the graded workspace (evidence pointer).
	WorkspaceDir string `json:"workspaceDir"`
	// Attempts holds every grader pass: one when the first pass was clean,
	// three when the flake rule re-ran a failure. Full per-command evidence
	// for every attempt is retained so the grade is recomputable.
	Attempts []Attempt `json:"attempts"`
	// Flaky is true when any command's outcome flipped across attempts —
	// the task is disqualified, the numeric fields below are zeroed and
	// must not be scored.
	Flaky bool `json:"flaky"`
	// SetupFailed is true when setup failed (stably) — no test signal
	// exists and the grade is 0 with every test command marked not-run.
	SetupFailed bool `json:"setupFailed"`
	// F2PFraction is the fraction of failToPass commands that passed.
	F2PFraction float64 `json:"f2pFraction"`
	// P2PPreservation is the fraction of passToPass commands still passing.
	P2PPreservation float64 `json:"p2pPreservation"`
	// Grade = F2PFraction × P2PPreservation (ADR-0032.1 GH1).
	Grade float64 `json:"grade"`
}

// Grader runs a fixture's command lists inside its pinned container image
// against a provisioned workspace and computes the outcome grade. It is
// deterministic — no LLM anywhere (ADR-0032.1 S1).
type Grader struct {
	// Runtime is the container engine; tests inject a fake.
	Runtime ContainerRuntime
}

// NewGrader returns a Grader backed by the docker CLI.
func NewGrader() *Grader { return &Grader{Runtime: DockerCLI{}} }

// Grade grades one trial. It verifies the engine is available (returning an
// ErrDockerUnavailable-wrapping error when not), runs one attempt, and — on
// any command failure — re-runs twice more (the flake rule): any
// per-command outcome flip across attempts marks the Result Flaky. Errors
// are engine/infrastructure failures only; test failures are grades.
func (g *Grader) Grade(ctx context.Context, f Fixture, workspaceDir string) (Result, error) {
	if err := f.Validate(); err != nil {
		return Result{}, err
	}
	if err := g.Runtime.Available(ctx); err != nil {
		return Result{}, err
	}

	res := Result{FixtureID: f.ID, Image: f.Image, WorkspaceDir: workspaceDir}
	first, err := g.attempt(ctx, f, workspaceDir)
	if err != nil {
		return Result{}, err
	}
	res.Attempts = []Attempt{first}

	if first.failed() {
		// Flake rule (ADR-0032 trap 7): on grader failure, exactly two
		// re-runs; any flip disqualifies the task.
		for i := 0; i < 2; i++ {
			rerun, err := g.attempt(ctx, f, workspaceDir)
			if err != nil {
				return Result{}, err
			}
			res.Attempts = append(res.Attempts, rerun)
		}
		if flipped(res.Attempts) {
			res.Flaky = true
			return res, nil
		}
	}

	// Outcomes are identical across attempts here; score the first.
	res.SetupFailed, res.F2PFraction, res.P2PPreservation = score(first)
	res.Grade = res.F2PFraction * res.P2PPreservation
	return res, nil
}

// attempt runs one full grader pass in a fresh container: setup commands in
// order (a setup failure aborts the rest of the attempt — later commands are
// recorded as not-run), then every failToPass and passToPass command
// regardless of individual failures (they are independent measurements).
func (g *Grader) attempt(ctx context.Context, f Fixture, workspaceDir string) (Attempt, error) {
	id, err := g.Runtime.Start(ctx, ContainerSpec{
		Image: f.Image, WorkspaceDir: workspaceDir, Network: f.NetworkMode(),
	})
	if err != nil {
		return Attempt{}, fmt.Errorf("grade %s: %w", f.ID, err)
	}
	defer g.Runtime.Remove(context.WithoutCancel(ctx), id)

	var a Attempt
	aborted := false
	run := func(phase Phase, cmd string, stopPhaseOnFail bool) error {
		cr := CommandResult{Phase: phase, Cmd: cmd}
		if !aborted {
			out, err := g.Runtime.Exec(ctx, id, cmd)
			if err != nil {
				return fmt.Errorf("grade %s (%s %q): %w", f.ID, phase, cmd, err)
			}
			cr.Ran = true
			cr.ExitCode = out.ExitCode
			cr.Pass = out.ExitCode == 0
			cr.Output = out.Output
			if !cr.Pass && stopPhaseOnFail {
				aborted = true
			}
		}
		a.Commands = append(a.Commands, cr)
		return nil
	}
	for _, cmd := range f.SetupCmds {
		if err := run(PhaseSetup, cmd, true); err != nil {
			return Attempt{}, err
		}
	}
	for _, cmd := range f.FailToPassCmds {
		if err := run(PhaseFailToPass, cmd, false); err != nil {
			return Attempt{}, err
		}
	}
	for _, cmd := range f.PassToPassCmds {
		if err := run(PhasePassToPass, cmd, false); err != nil {
			return Attempt{}, err
		}
	}
	return a, nil
}

// flipped reports whether any command's (Ran, Pass) outcome differs across
// attempts. Attempts share command order by construction.
func flipped(attempts []Attempt) bool {
	base := attempts[0].Commands
	for _, a := range attempts[1:] {
		for i, c := range a.Commands {
			if c.Ran != base[i].Ran || c.Pass != base[i].Pass {
				return true
			}
		}
	}
	return false
}

// score computes the GH1 grade inputs from one attempt: whether setup
// failed, the failToPass pass fraction, and the passToPass preservation
// fraction. Commands that never ran count as not passed — with setup
// stably failed both fractions are 0 and the grade is an honest 0, with
// SetupFailed distinguishing "no test signal" from "tests failed".
func score(a Attempt) (setupFailed bool, f2p, p2p float64) {
	var f2pPass, f2pTotal, p2pPass, p2pTotal int
	for _, c := range a.Commands {
		switch c.Phase {
		case PhaseSetup:
			if !c.Ran || !c.Pass {
				setupFailed = true
			}
		case PhaseFailToPass:
			f2pTotal++
			if c.Ran && c.Pass {
				f2pPass++
			}
		case PhasePassToPass:
			p2pTotal++
			if c.Ran && c.Pass {
				p2pPass++
			}
		}
	}
	// Validate guarantees both totals are non-zero.
	return setupFailed, float64(f2pPass) / float64(f2pTotal), float64(p2pPass) / float64(p2pTotal)
}
