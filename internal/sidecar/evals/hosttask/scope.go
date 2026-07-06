package hosttask

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WorkspaceScope answers path-containment questions for one provisioned
// trial workspace. It is the single owner of the "inside the workspace"
// judgment (ADR-0032.1 S2) used by both the host write policy
// (evals.WorkspaceWritePolicy) and the exploration/engineering classifier
// (Classifier).
//
// Resolution strategy: paths must be absolute; `..` segments are removed by
// cleaning; the deepest existing ancestor is resolved with
// filepath.EvalSymlinks and the non-existing remainder re-joined — so
// traversal and symlinked directories cannot smuggle a path out of the
// workspace, while writes that create new files still resolve.
//
// TOCTOU honesty: containment is decided at call time. A symlink created or
// swapped between the decision and the actual filesystem operation can still
// redirect that operation. This scope is an eval-integrity guard for a
// cooperative-but-fallible agent, not a security sandbox against an
// adversarial one; container/network isolation is the security boundary
// (ADR-0032.1 S1 grader).
type WorkspaceScope struct {
	root string
}

// NewWorkspaceScope resolves root — which must exist — into a scope. The
// root is resolved through symlinks once here so every later containment
// check compares fully resolved paths.
func NewWorkspaceScope(root string) (WorkspaceScope, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return WorkspaceScope{}, fmt.Errorf("hosttask: workspace root %q: %w", root, err)
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return WorkspaceScope{}, fmt.Errorf("hosttask: workspace root %q: %w", root, err)
	}
	return WorkspaceScope{root: resolved}, nil
}

// Root returns the symlink-resolved absolute workspace root.
func (s WorkspaceScope) Root() string { return s.root }

// Resolve returns the symlink-resolved absolute form of path, or an error
// when the path cannot be resolved deterministically: relative paths (the
// agent's working directory is not part of the recorded contract), dangling
// symlinks, symlink loops, or filesystem errors. Non-existing suffixes are
// fine — a write creating a new file resolves through its deepest existing
// ancestor.
func (s WorkspaceScope) Resolve(path string) (string, error) {
	if s.root == "" {
		return "", errors.New("hosttask: zero WorkspaceScope; use NewWorkspaceScope")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path %q is not absolute", path)
	}
	cur := filepath.Clean(path)
	rest := ""
	for {
		if _, err := os.Lstat(cur); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return "", fmt.Errorf("resolve %q: %w", path, err)
			}
			parent := filepath.Dir(cur)
			if parent == cur {
				// The filesystem root does not exist — unreachable in
				// practice, but terminate deterministically.
				return "", fmt.Errorf("resolve %q: no existing ancestor", path)
			}
			rest = filepath.Join(filepath.Base(cur), rest)
			cur = parent
			continue
		}
		// cur exists (it may itself be a symlink): resolve it fully. A
		// failure here means a dangling symlink or a loop — refuse rather
		// than guess where the operation would actually land.
		resolved, err := filepath.EvalSymlinks(cur)
		if err != nil {
			return "", fmt.Errorf("resolve %q: %w", path, err)
		}
		return filepath.Join(resolved, rest), nil
	}
}

// Contains reports whether resolved — a Resolve output — lies inside the
// workspace root. The root itself counts as inside.
func (s WorkspaceScope) Contains(resolved string) bool {
	if s.root == "" {
		return false
	}
	return resolved == s.root || strings.HasPrefix(resolved, s.root+string(filepath.Separator))
}
