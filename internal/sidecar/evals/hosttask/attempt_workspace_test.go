package hosttask

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// mutatingRuntime simulates a test suite that leaves a cache marker in the
// workspace: its f2p command fails deterministically on a clean tree but
// would PASS on any tree carrying the marker from a previous attempt. If
// grade attempts shared the workspace, the trap-7 re-runs would flip and
// falsely disqualify the task as Flaky.
type mutatingRuntime struct {
	specs []ContainerSpec
}

func (m *mutatingRuntime) Available(context.Context) error { return nil }
func (m *mutatingRuntime) Start(_ context.Context, spec ContainerSpec) (string, error) {
	m.specs = append(m.specs, spec)
	return strconv.Itoa(len(m.specs) - 1), nil
}
func (m *mutatingRuntime) Remove(context.Context, string) error { return nil }
func (m *mutatingRuntime) Exec(_ context.Context, id, cmd string) (ExecResult, error) {
	i, _ := strconv.Atoi(id)
	dir := m.specs[i].WorkspaceDir
	// Copy fidelity: the host's fix — a regular file and a symlink — must be
	// present in every attempt tree.
	if _, err := os.Stat(filepath.Join(dir, "fix.txt")); err != nil {
		return ExecResult{ExitCode: 9, Output: "fix.txt missing from attempt tree"}, nil
	}
	if _, err := os.Lstat(filepath.Join(dir, "link.txt")); err != nil {
		return ExecResult{ExitCode: 9, Output: "link.txt missing from attempt tree"}, nil
	}
	if cmd == "p2p" {
		return ExecResult{}, nil
	}
	marker := filepath.Join(dir, "cache-marker")
	if _, err := os.Stat(marker); err == nil {
		return ExecResult{Output: "marker present, passing"}, nil // the flip, if state ever leaks
	}
	if err := os.WriteFile(marker, []byte("x"), 0o644); err != nil {
		return ExecResult{}, err
	}
	return ExecResult{ExitCode: 1, Output: "failing deterministically on a clean tree"}, nil
}

// TestGradeAttemptsRunOnFreshCopies pins the ADR-0032.1 S3 equal-footing
// contract: with CopyWorkspacePerAttempt every attempt runs against a
// pristine copy of the graded tree, so a tree-mutating test yields a stable
// (non-Flaky) failure, and the source workspace is never touched by grading.
func TestGradeAttemptsRunOnFreshCopies(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "fix.txt"), []byte("the fix"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("fix.txt", filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	f := validFixture()
	f.SetupCmds = nil
	f.FailToPassCmds = []string{"f2p"}
	f.PassToPassCmds = []string{"p2p"}

	rt := &mutatingRuntime{}
	g := &Grader{Runtime: rt, AttemptWorkspace: CopyWorkspacePerAttempt}
	res, err := g.Grade(context.Background(), f, src)
	if err != nil {
		t.Fatal(err)
	}
	if res.Flaky {
		t.Fatalf("fresh-copy attempts must not flip into Flaky: %+v", res)
	}
	if len(res.Attempts) != 3 || res.F2PFraction != 0 || res.P2PPreservation != 1 {
		t.Fatalf("want 3 attempts with stable f2p failure: attempts=%d f2p=%v p2p=%v", len(res.Attempts), res.F2PFraction, res.P2PPreservation)
	}
	// Every attempt saw its own directory, never the source workspace.
	seen := map[string]bool{}
	for _, spec := range rt.specs {
		if spec.WorkspaceDir == src {
			t.Fatal("an attempt ran against the source workspace itself")
		}
		if seen[spec.WorkspaceDir] {
			t.Fatalf("attempt workspace reused: %s", spec.WorkspaceDir)
		}
		seen[spec.WorkspaceDir] = true
	}
	// The result still names the source workspace (the evidence pointer),
	// and grading left no marker in it.
	if res.WorkspaceDir != src {
		t.Fatalf("result workspaceDir = %s, want source %s", res.WorkspaceDir, src)
	}
	if _, err := os.Stat(filepath.Join(src, "cache-marker")); !os.IsNotExist(err) {
		t.Fatalf("grading mutated the source workspace: %v", err)
	}
	// The nil seam (unit-test default) is the documented shared-dir
	// behavior: the same fixture flips and disqualifies as Flaky.
	rt2 := &mutatingRuntime{}
	res2, err := (&Grader{Runtime: rt2}).Grade(context.Background(), f, mustCopyDir(t, src))
	if err != nil {
		t.Fatal(err)
	}
	if !res2.Flaky {
		t.Fatalf("shared-dir grading should expose the flip this test simulates: %+v", res2)
	}
}

// mustCopyDir clones src into a fresh temp dir via the production copyTree.
func mustCopyDir(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}
