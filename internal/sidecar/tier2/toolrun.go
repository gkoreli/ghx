package tier2

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ToolRun is the evidence record of one structural tool invocation: exactly
// what ran, where, and what came back. A failed run is first-class evidence
// (ADR-0024.1: "a failed codemap/ast-grep invocation is evidence and must be
// reported as uncertainty or a blocked Tier-2 path").
type ToolRun struct {
	// Backend is the canonical backend ID, e.g. "local:codemap".
	Backend string
	// Tool is the binary name as invoked (the first word of the recomputable
	// command), e.g. "codemap" or "ast-grep".
	Tool string
	// Binary is the resolved tool path that ran.
	Binary string
	// Args is the argv after the binary name.
	Args []string
	// Dir is the snapshot directory the tool ran in.
	Dir string
	// Started is when the invocation began (UTC).
	Started time.Time
	// Duration is the wall time of the invocation.
	Duration time.Duration
	// ExitCode is the process exit code (0 on success, -1 if it never ran).
	ExitCode int
	// Stdout is the captured tool output (the evidence payload).
	Stdout string
	// Stderr is the captured error stream (diagnostic evidence on failure).
	Stderr string
}

// Command renders the recomputable command line for evidence citations, e.g.
// "codemap --json ." or "ast-grep run --pattern $A --json=compact .".
func (t ToolRun) Command() string {
	return strings.Join(append([]string{t.Tool}, t.Args...), " ")
}

// Matched reports whether the tool found results, under grep-parity exit
// semantics (0 = found; 1 = ran fine, nothing found).
func (t ToolRun) Matched() bool {
	return t.ExitCode == 0
}

// subprocessAdapter is the shared body of every absorbed binary tool:
// PATH discovery with a typed not-installed error, and evidence-capturing
// execution inside the snapshot directory. Codemap and AstGrep embed it —
// discovery, invocation, and capture live in exactly one place (single-owner
// rule, AGENTS.md Engineering Tenets).
type subprocessAdapter struct {
	// binary is the name discovered on PATH and shown in Command().
	binary string
	// backend is the canonical backend ID stamped on every ToolRun.
	backend string
	// errNotInstalled is the tool's typed absence error, returned wrapped so
	// callers can errors.Is against it and fall back gracefully.
	errNotInstalled error
	// lookPath resolves the binary; swapped in tests. Defaults to exec.LookPath.
	lookPath func(string) (string, error)
	// runTimeout bounds one invocation.
	runTimeout time.Duration
}

// Discover resolves the tool binary on PATH. Absence returns the adapter's
// typed not-installed error (wrapped) so callers can fall back gracefully and
// print the tool's install hint.
func (a *subprocessAdapter) Discover() (string, error) {
	path, err := a.lookPath(a.binary)
	if err != nil {
		return "", fmt.Errorf("%w: %v", a.errNotInstalled, err)
	}
	return path, nil
}

// newRun seeds the ToolRun evidence record for one invocation attempt.
func (a *subprocessAdapter) newRun(dir string, args []string) ToolRun {
	return ToolRun{Backend: a.backend, Tool: a.binary, Dir: dir, Args: args, ExitCode: -1}
}

// exec runs the discovered binary with args inside snapshotDir and returns
// the ToolRun evidence record. The returned error is non-nil when the binary
// is missing or the process exited non-zero — the ToolRun is still populated
// in the exit-code case so failures stay auditable.
func (a *subprocessAdapter) exec(ctx context.Context, snapshotDir string, args []string) (ToolRun, error) {
	run := a.newRun(snapshotDir, args)
	binary, err := a.Discover()
	if err != nil {
		return run, err
	}
	run.Binary = binary

	if a.runTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, a.runTimeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = snapshotDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	run.Started = time.Now().UTC()
	err = cmd.Run()
	run.Duration = time.Since(run.Started)
	run.Stdout = stdout.String()
	run.Stderr = stderr.String()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			run.ExitCode = exitErr.ExitCode()
			return run, fmt.Errorf("%s exited %d: %s", run.Command(), run.ExitCode, strings.TrimSpace(run.Stderr))
		}
		return run, fmt.Errorf("run %s: %w", run.Command(), err)
	}
	run.ExitCode = 0
	return run, nil
}
