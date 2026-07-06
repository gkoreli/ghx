package hosttask

import (
	"context"
	"fmt"
	"os"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// workspaceCloneStrategy is the pinned-SHA materialization plan for host
// workspaces: shallow (depth 1), full blobs, no tags. It deliberately drops
// tier2's blob:none filter — a blobless promisor clone lazy-fetches blobs
// over the network, and a host workspace must be complete offline (grader
// containers default to network "none"; the host agent works the tree
// directly).
func workspaceCloneStrategy() tier2.CloneStrategy {
	return tier2.CloneStrategy{Depth: 1, NoTags: true}
}

// Workspace is one provisioned per-trial working tree: WorkspaceRepo checked
// out at exactly PinnedSHA in a directory no other trial has touched.
type Workspace struct {
	// Dir is the workspace directory (contains the checkout and its .git).
	Dir string
	// FixtureID is the fixture this workspace was provisioned for.
	FixtureID string
	// SHA is the verified HEAD commit — always the fixture's PinnedSHA.
	SHA string
}

// Remove deletes the workspace directory. Callers own workspace lifetime;
// the provisioner never reuses or cleans up prior trials' directories.
func (w Workspace) Remove() error { return os.RemoveAll(w.Dir) }

// Provisioner materializes fresh per-trial host workspaces (ADR-0032.1 S1).
// The per-trial reset guarantee (ADR-0032 trap 5) holds by construction:
// every Provision call creates a brand-new uniquely named directory and
// verifies its HEAD against the fixture pin; directories are never reused.
type Provisioner struct {
	// Git executes git commands; tests inject a recorder.
	Git tier2.GitRunner
	// Root is the parent directory workspaces are created under.
	Root string
	// RemoteURL overrides the clone URL for every fixture. Empty derives
	// https://github.com/<owner>/<repo>.git. Tests point it at file://
	// fixtures (the tier2 pattern).
	RemoteURL string
}

// NewProvisioner returns a Provisioner using the real git binary, creating
// workspaces under root.
func NewProvisioner(root string) *Provisioner {
	return &Provisioner{Git: tier2.ExecGit{}, Root: root}
}

// Provision creates the trial's workspace: a fresh directory under Root
// named after the fixture and trial, populated with WorkspaceRepo at exactly
// PinnedSHA via the shared tier2 SHA-fetch plan, HEAD-verified. The
// directory is removed on any failure; on success the caller owns it.
func (p *Provisioner) Provision(ctx context.Context, f Fixture, trial int) (Workspace, error) {
	if err := f.Validate(); err != nil {
		return Workspace{}, err
	}
	if err := os.MkdirAll(p.Root, 0o755); err != nil {
		return Workspace{}, fmt.Errorf("mkdir workspace root %s: %w", p.Root, err)
	}
	// MkdirTemp guarantees a new empty directory per call — the per-trial
	// reset. The trial number is in the name for auditability only.
	dir, err := os.MkdirTemp(p.Root, fmt.Sprintf("%s-trial%03d-", f.ID, trial))
	if err != nil {
		return Workspace{}, fmt.Errorf("create workspace dir: %w", err)
	}

	url := p.RemoteURL
	if url == "" {
		url = f.RemoteURL()
	}
	for _, args := range tier2.SHAFetchPlan(url, f.PinnedSHA, workspaceCloneStrategy()) {
		if _, err := p.Git.Run(ctx, dir, args...); err != nil {
			os.RemoveAll(dir)
			return Workspace{}, fmt.Errorf("provision %s trial %d: %w", f.ID, trial, err)
		}
	}

	// The materialized commit is the workspace identity and must match the
	// pin — a mismatch means the grade would not be about the pinned code.
	head, err := p.Git.Run(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		os.RemoveAll(dir)
		return Workspace{}, fmt.Errorf("provision %s trial %d: %w", f.ID, trial, err)
	}
	if head != f.PinnedSHA {
		os.RemoveAll(dir)
		return Workspace{}, fmt.Errorf("provision %s trial %d: workspace HEAD %s does not match pinnedSha %s", f.ID, trial, head, f.PinnedSHA)
	}
	return Workspace{Dir: dir, FixtureID: f.ID, SHA: head}, nil
}
