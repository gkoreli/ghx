package evals

import (
	"fmt"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
)

// WritePolicy decides whether one write-kind operation — an ACP write-kind
// permission request or a client fs write — may proceed.
//
// The default (a nil policy on evalClient) is the recon suite's read-only
// contract: every write is denied and recorded as a violation (ADR-0016.1
// G5). The host-task arms (ADR-0032.1 S2) inject WorkspaceWritePolicy
// instead. The sidecar inside arm B keeps the nil policy, so a sidecar write
// attempt stays a run-invalidating violation.
type WritePolicy interface {
	// AllowWrite returns the resolved absolute path a permitted write must
	// target, or a denial error stating why the write is not allowed. The
	// denial text is what episode violation records carry, so it must be
	// self-explanatory.
	AllowWrite(path string) (resolved string, err error)
}

// WorkspaceWritePolicy allows writes inside one provisioned trial workspace
// and denies everything outside it (ADR-0032.1 D5.2: workspace-scoped
// writes allowed, outside denied). Containment is hosttask.WorkspaceScope's
// judgment — absolute paths only, `..` cleaned, symlinks resolved through
// the deepest existing ancestor; see WorkspaceScope for the stated TOCTOU
// limits.
type WorkspaceWritePolicy struct {
	scope hosttask.WorkspaceScope
}

// NewWorkspaceWritePolicy builds the policy for a workspace root, which must
// already exist (the hosttask provisioner creates it before the episode
// starts).
func NewWorkspaceWritePolicy(workspaceRoot string) (*WorkspaceWritePolicy, error) {
	scope, err := hosttask.NewWorkspaceScope(workspaceRoot)
	if err != nil {
		return nil, err
	}
	return &WorkspaceWritePolicy{scope: scope}, nil
}

// AllowWrite implements WritePolicy: resolve, then require containment. A
// path that cannot be resolved deterministically (relative, dangling
// symlink, loop) is denied — fail closed, never guess where a write lands.
func (p *WorkspaceWritePolicy) AllowWrite(path string) (string, error) {
	resolved, err := p.scope.Resolve(path)
	if err != nil {
		return "", fmt.Errorf("unresolvable write target: %v", err)
	}
	if !p.scope.Contains(resolved) {
		return "", fmt.Errorf("outside workspace %s: %s", p.scope.Root(), resolved)
	}
	return resolved, nil
}
