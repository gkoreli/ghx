package tier2

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitRunner executes one git command and returns its stdout. It exists so
// clone/resolve logic can be tested against recorded invocations; the real
// implementation is ExecGit.
type GitRunner interface {
	// Run executes git with args. dir is the working directory ("" means the
	// process cwd; resolve-only commands pass ""). Returns trimmed stdout.
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecGit runs the real git binary.
type ExecGit struct{}

// Run implements GitRunner via exec.CommandContext.
func (ExecGit) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// CloneStrategy describes how a snapshot is materialized. The M7 default is
// pre-registered in ADR-0024.1 "Clone and Cache Strategy": shallow (depth 1),
// blobless (--filter=blob:none), no tags, optionally sparse. Structural
// analysis needs a snapshot, not history.
type CloneStrategy struct {
	// Depth is the clone/fetch depth; the M7 default is 1.
	Depth int
	// Filter is the partial-clone filter; the M7 default is "blob:none".
	Filter string
	// NoTags skips tag fetching; the M7 default is true.
	NoTags bool
	// SparsePaths, when non-empty, restricts the checkout to these paths.
	SparsePaths []string
}

// DefaultCloneStrategy returns the pre-registered M7 clone plan, sparse when
// candidate paths are known (ADR-0024.1: "If Tier 1 already found candidate
// paths, start sparse").
func DefaultCloneStrategy(sparsePaths []string) CloneStrategy {
	return CloneStrategy{Depth: 1, Filter: "blob:none", NoTags: true, SparsePaths: sparsePaths}
}

// Label is the canonical strategy name recorded in snapshot metadata and
// provenance: "shallow-blobless" or "shallow-blobless-sparse".
func (s CloneStrategy) Label() string {
	label := "shallow"
	if s.Filter == "blob:none" {
		label += "-blobless"
	} else if s.Filter != "" {
		label += "-" + strings.ReplaceAll(s.Filter, ":", "-")
	}
	if len(s.SparsePaths) > 0 {
		label += "-sparse"
	}
	return label
}

// cloneArgs builds the git clone argv for a branch/tag/default-HEAD
// materialization, mirroring the ADR-0024.1 default clone plan:
//
//	git clone --depth=1 --filter=blob:none --no-tags [--sparse] [--branch <ref>] -- <url> <dir>
//
// branch is empty for the remote default branch.
func cloneArgs(url, dir, branch string, s CloneStrategy) []string {
	args := []string{"clone", fmt.Sprintf("--depth=%d", s.Depth)}
	if s.Filter != "" {
		args = append(args, "--filter="+s.Filter)
	}
	if s.NoTags {
		args = append(args, "--no-tags")
	}
	if len(s.SparsePaths) > 0 {
		args = append(args, "--sparse")
	}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	return append(args, "--", url, dir)
}

// shaFetchPlan builds the argv sequence that materializes an exact commit SHA,
// used when the requested ref is itself a full SHA (git clone --branch cannot
// take one). Each element runs with the snapshot temp dir as cwd:
//
//	git init
//	git remote add origin <url>
//	git fetch --depth=1 --filter=blob:none [--no-tags] origin <sha>
//	git checkout --detach FETCH_HEAD
//
// Requires the server to allow SHA fetches (GitHub does for reachable SHAs).
func shaFetchPlan(url, sha string, s CloneStrategy) [][]string {
	fetch := []string{"fetch", fmt.Sprintf("--depth=%d", s.Depth)}
	if s.Filter != "" {
		fetch = append(fetch, "--filter="+s.Filter)
	}
	if s.NoTags {
		fetch = append(fetch, "--no-tags")
	}
	fetch = append(fetch, "origin", sha)
	return [][]string{
		{"init", "--quiet"},
		{"remote", "add", "origin", url},
		fetch,
		{"checkout", "--quiet", "--detach", "FETCH_HEAD"},
	}
}

// sparseCheckoutArgs builds the sparse-checkout argv run inside the snapshot:
//
//	git sparse-checkout set <paths...>
func sparseCheckoutArgs(paths []string) []string {
	return append([]string{"sparse-checkout", "set"}, paths...)
}

// IsCommitSHA reports whether ref is a full 40-hex commit SHA.
func IsCommitSHA(ref string) bool { return shaRE.MatchString(ref) }

// ResolveRef resolves repo/ref to a full commit SHA without cloning, via
// `git ls-remote`. An empty ref resolves the remote HEAD (default branch).
// A full 40-hex SHA is returned as-is with no network round trip. Branch
// refs win over tag refs when both match the short name.
func ResolveRef(ctx context.Context, git GitRunner, url, ref string) (string, error) {
	if IsCommitSHA(ref) {
		return ref, nil
	}
	if ref == "" {
		out, err := git.Run(ctx, "", "ls-remote", url, "HEAD")
		if err != nil {
			return "", fmt.Errorf("resolve %s HEAD: %w", url, err)
		}
		sha := firstRefSHA(out, "HEAD")
		if sha == "" {
			return "", fmt.Errorf("resolve %s HEAD: no HEAD ref in ls-remote output", url)
		}
		return sha, nil
	}
	out, err := git.Run(ctx, "", "ls-remote", url, ref, "refs/heads/"+ref, "refs/tags/"+ref)
	if err != nil {
		return "", fmt.Errorf("resolve %s %s: %w", url, ref, err)
	}
	for _, want := range []string{"refs/heads/" + ref, "refs/tags/" + ref, ref} {
		if sha := firstRefSHA(out, want); sha != "" {
			return sha, nil
		}
	}
	return "", fmt.Errorf("resolve %s %s: ref not found on remote", url, ref)
}

// firstRefSHA scans ls-remote output ("<sha>\t<refname>" lines) for the exact
// refname and returns its SHA, or "" when absent.
func firstRefSHA(out, refname string) string {
	for _, line := range strings.Split(out, "\n") {
		sha, name, ok := strings.Cut(strings.TrimSpace(line), "\t")
		if ok && name == refname && shaRE.MatchString(sha) {
			return sha
		}
	}
	return ""
}
