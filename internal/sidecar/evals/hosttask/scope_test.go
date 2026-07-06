package hosttask

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestScope builds a WorkspaceScope over a fresh temp workspace and
// returns it with its resolved root (t.TempDir may live behind symlinks,
// e.g. /var -> /private/var on darwin, so tests compare against Root()).
func newTestScope(t *testing.T) (WorkspaceScope, string) {
	t.Helper()
	scope, err := NewWorkspaceScope(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return scope, scope.Root()
}

func TestNewWorkspaceScopeMissingRoot(t *testing.T) {
	if _, err := NewWorkspaceScope(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("want error for a missing workspace root")
	}
}

func TestWorkspaceScopeContainment(t *testing.T) {
	scope, root := newTestScope(t)
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "file.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()

	cases := []struct {
		name   string
		path   string
		inside bool
	}{
		{"root itself", root, true},
		{"existing file inside", filepath.Join(root, "sub", "file.go"), true},
		{"new file inside", filepath.Join(root, "sub", "new.go"), true},
		{"new nested dirs inside", filepath.Join(root, "a", "b", "c.go"), true},
		{"dot-dot that stays inside", filepath.Join(root, "sub", "..", "sub", "file.go"), true},
		{"outside dir", outside, false},
		{"outside file", filepath.Join(outside, "f.txt"), false},
		{"dot-dot traversal escape", filepath.Join(root, "sub", "..", "..", "escape.txt"), false},
		{"root prefix sibling", root + "-sibling/f.txt", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := scope.Resolve(tc.path)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tc.path, err)
			}
			if got := scope.Contains(resolved); got != tc.inside {
				t.Fatalf("Contains(%q -> %q) = %v, want %v", tc.path, resolved, got, tc.inside)
			}
		})
	}
}

func TestWorkspaceScopeResolveRejects(t *testing.T) {
	scope, root := newTestScope(t)

	// Relative paths: the agent's cwd is not part of the recorded contract.
	if _, err := scope.Resolve("relative/path.go"); err == nil {
		t.Fatal("want error for a relative path")
	}

	// Dangling symlink: refusing beats guessing where the write would land.
	dangling := filepath.Join(root, "dangling")
	if err := os.Symlink(filepath.Join(root, "missing-target"), dangling); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Resolve(filepath.Join(dangling, "f.txt")); err == nil {
		t.Fatal("want error resolving through a dangling symlink")
	}
	if _, err := scope.Resolve(dangling); err == nil {
		t.Fatal("want error resolving a dangling symlink itself")
	}

	// Zero value fails closed.
	var zero WorkspaceScope
	if _, err := zero.Resolve(root); err == nil {
		t.Fatal("want error from the zero WorkspaceScope")
	}
	if zero.Contains(root) {
		t.Fatal("zero WorkspaceScope must contain nothing")
	}
}

func TestWorkspaceScopeSymlinks(t *testing.T) {
	scope, root := newTestScope(t)
	outside := t.TempDir()

	// Symlink escape: a dir inside the workspace pointing outside.
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	resolved, err := scope.Resolve(filepath.Join(root, "escape", "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if scope.Contains(resolved) {
		t.Fatalf("symlink escape not detected: %q resolved to %q", filepath.Join(root, "escape", "f.txt"), resolved)
	}

	// Symlink that stays inside is fine.
	if err := os.MkdirAll(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	resolved, err = scope.Resolve(filepath.Join(root, "alias", "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !scope.Contains(resolved) {
		t.Fatalf("inside symlink misclassified: resolved to %q, root %q", resolved, root)
	}
}
