package tier2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SnapshotRequest asks for a materialized local snapshot of repo at ref.
type SnapshotRequest struct {
	// Repo is the "owner/repo" full name.
	Repo string
	// RemoteURL overrides the clone URL. Empty derives
	// https://github.com/<owner>/<repo>.git. Tests point it at file:// fixtures.
	RemoteURL string
	// Ref is a branch, tag, or full 40-hex commit SHA. Empty means the remote
	// default branch (HEAD).
	Ref string
	// SparsePaths, when non-empty, materializes only these paths (Tier 1
	// candidate paths). Empty materializes the full working tree.
	SparsePaths []string
}

// remoteURL returns the effective clone URL for the request.
func (r SnapshotRequest) remoteURL() string {
	if r.RemoteURL != "" {
		return r.RemoteURL
	}
	return "https://github.com/" + r.Repo + ".git"
}

// Snapshot is a read-only materialized repo state pinned to one commit SHA.
// Read-only is the domain contract: nothing in ghx mutates the working tree,
// and Tier-2 tools only read from Dir. Cache paths under Dir are provenance,
// never primary report evidence (ADR-0024.1 "Visibility Contract").
type Snapshot struct {
	// Repo is the owner/repo full name.
	Repo string
	// RemoteURL is the URL the snapshot was cloned from.
	RemoteURL string
	// RequestedRef is what the caller asked for ("" = default branch).
	RequestedRef string
	// SHA is the resolved commit — the snapshot identity and cache key.
	SHA string
	// Dir is the local snapshot directory.
	Dir string
	// Strategy is how the snapshot was (originally) materialized.
	Strategy CloneStrategy
	// CacheHit is true when an existing snapshot for SHA was reused.
	CacheHit bool
	// Evictions lists snapshots removed by the pre-clone eviction pass.
	// Runtime-log material only.
	Evictions []EvictionEvent
}

// Provenance is the pre-tool visibility record: it is emitted before any
// structural tool runs so every Tier-2 answer is auditable back to an exact
// repo state and clone decision (NORTH_STAR "Remote-first, escalation
// explicit"; ADR-0024.1 "Visibility Contract").
type Provenance struct {
	// Repo is the owner/repo full name.
	Repo string
	// Ref is the requested ref ("" rendered as HEAD).
	Ref string
	// SHA is the resolved commit SHA.
	SHA string
	// Strategy is the canonical clone strategy label.
	Strategy string
	// SparsePaths lists sparse-checkout paths, empty for full checkout.
	SparsePaths []string
	// CacheHit reports whether the snapshot was reused from cache.
	CacheHit bool
	// SnapshotPath is the local snapshot directory (provenance, not evidence).
	SnapshotPath string
}

// Provenance derives the emission record from the snapshot.
func (s Snapshot) Provenance() Provenance {
	return Provenance{
		Repo: s.Repo, Ref: s.RequestedRef, SHA: s.SHA,
		Strategy: s.Strategy.Label(), SparsePaths: s.Strategy.SparsePaths,
		CacheHit: s.CacheHit, SnapshotPath: s.Dir,
	}
}

// Lines renders the human/agent-readable provenance block, one prefixed line
// per fact, printed before any structural tool output.
func (p Provenance) Lines() []string {
	ref := p.Ref
	if ref == "" {
		ref = "HEAD"
	}
	hit := "miss"
	if p.CacheHit {
		hit = "hit"
	}
	lines := []string{
		fmt.Sprintf("tier2 snapshot: repo=%s ref=%s sha=%s", p.Repo, ref, p.SHA),
		fmt.Sprintf("tier2 clone: strategy=%s cache=%s path=%s", p.Strategy, hit, p.SnapshotPath),
	}
	if len(p.SparsePaths) > 0 {
		lines = append(lines, "tier2 sparse: "+strings.Join(p.SparsePaths, ", "))
	}
	return lines
}

// FindProvenance is the exact inverse of Provenance.Lines: it scans arbitrary
// captured text (a shell tool's recorded output) for the first tier2
// provenance block and reconstructs it. It lives next to Lines so the format
// has one owner. Used by the sidecar runtime to attach clone provenance to
// the recorded tier decision when Tier 2 ran via shell (ADR-0024.2 D5);
// best-effort by design — ok is false when no snapshot line is visible.
func FindProvenance(text string) (Provenance, bool) {
	var p Provenance
	found := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case !found && strings.HasPrefix(line, "tier2 snapshot: "):
			fields := provenanceFields(strings.TrimPrefix(line, "tier2 snapshot: "), "repo", "ref", "sha")
			p.Repo, p.SHA = fields["repo"], fields["sha"]
			if ref := fields["ref"]; ref != "HEAD" {
				p.Ref = ref
			}
			found = true
		case found && strings.HasPrefix(line, "tier2 clone: "):
			fields := provenanceFields(strings.TrimPrefix(line, "tier2 clone: "), "strategy", "cache", "path")
			p.Strategy, p.SnapshotPath = fields["strategy"], fields["path"]
			p.CacheHit = fields["cache"] == "hit"
		case found && strings.HasPrefix(line, "tier2 sparse: "):
			for _, sp := range strings.Split(strings.TrimPrefix(line, "tier2 sparse: "), ", ") {
				if sp = strings.TrimSpace(sp); sp != "" {
					p.SparsePaths = append(p.SparsePaths, sp)
				}
			}
		case found && line != "" && !strings.HasPrefix(line, "tier2 "):
			// The block is contiguous; stop at the first foreign line so a
			// later unrelated block cannot bleed into this provenance.
			return p, true
		}
	}
	return p, found
}

// provenanceFields splits "k1=v1 k2=v2 k3=v3" where only the listed keys are
// valid, tolerating values that contain spaces (e.g. paths) by cutting each
// value at the next known " key=" marker.
func provenanceFields(s string, keys ...string) map[string]string {
	out := map[string]string{}
	for i, key := range keys {
		marker := key + "="
		start := strings.Index(s, marker)
		if start < 0 {
			continue
		}
		val := s[start+len(marker):]
		end := len(val)
		for _, next := range keys[i+1:] {
			if idx := strings.Index(val, " "+next+"="); idx >= 0 && idx < end {
				end = idx
			}
		}
		out[key] = strings.TrimSpace(val[:end])
	}
	return out
}

// Service is the single owner of the Tier-2 substrate: snapshot cache, clone
// materialization, and absorbed structural tools. The sidecar runtime and the
// CLI both reach Tier 2 only through this type (ADR-0024.1 "Implementation
// Shape").
type Service struct {
	// Cache owns layout and eviction.
	Cache *Cache
	// Git runs git commands (swapped in tests).
	Git GitRunner
	// Codemap is the absorbed codemap adapter.
	Codemap *Codemap
	// AstGrep is the absorbed ast-grep adapter.
	AstGrep *AstGrep
	// Version is the ghx version recorded in snapshot metadata.
	Version string
	// now is the clock, swapped in tests.
	now func() time.Time

	// mu guards active — snapshot dirs served during this Service's lifetime,
	// which eviction must never remove (ADR-0024.1 "Eviction").
	mu     sync.Mutex
	active map[string]bool
}

// NewService builds the production Tier-2 service rooted at the ghx home
// directory (~/.ghx or $GHX_HOME).
func NewService(ghxRoot, version string) *Service {
	return &Service{
		Cache:   NewCache(ghxRoot),
		Git:     ExecGit{},
		Codemap: NewCodemap(),
		AstGrep: NewAstGrep(),
		Version: version,
		now:     time.Now,
		active:  map[string]bool{},
	}
}

// markActive registers dir as in use by the running turn.
func (s *Service) markActive(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		s.active = map[string]bool{}
	}
	s.active[dir] = true
}

// activeDirs snapshots the active set for an eviction pass.
func (s *Service) activeDirs() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]bool, len(s.active))
	for d := range s.active {
		out[d] = true
	}
	return out
}

// Snapshot resolves req to a commit SHA and returns the materialized local
// snapshot, reusing the cache when the SHA is already present. The eviction
// pass runs before any new snapshot is created. The caller must emit
// snapshot.Provenance() before running structural tools.
func (s *Service) Snapshot(ctx context.Context, req SnapshotRequest) (Snapshot, error) {
	if _, _, err := SplitRepo(req.Repo); err != nil {
		return Snapshot{}, err
	}
	url := req.remoteURL()
	sha, err := ResolveRef(ctx, s.Git, url, req.Ref)
	if err != nil {
		return Snapshot{}, err
	}
	dir, err := s.Cache.SnapshotDir(req.Repo, sha)
	if err != nil {
		return Snapshot{}, err
	}
	strategy := DefaultCloneStrategy(req.SparsePaths)
	snap := Snapshot{
		Repo: req.Repo, RemoteURL: url, RequestedRef: req.Ref,
		SHA: sha, Dir: dir, Strategy: strategy,
	}

	if meta, err := LoadMetadata(dir); err == nil {
		// Cache hit: same resolved SHA snapshot is reused (ADR-0024.1 success
		// criterion). Keep the original materialization strategy visible.
		snap.CacheHit = true
		snap.Strategy = strategyFromMetadata(meta, strategy)
		s.markActive(dir)
		if err := Touch(dir, s.now()); err != nil {
			return Snapshot{}, fmt.Errorf("touch snapshot %s: %w", dir, err)
		}
		return snap, nil
	}

	// Eviction runs before creating a new snapshot, never touching snapshots
	// active in this turn.
	evictions, err := s.Cache.Evict(s.now(), s.activeDirs())
	if err != nil {
		return Snapshot{}, fmt.Errorf("cache eviction: %w", err)
	}
	snap.Evictions = evictions

	if err := s.materialize(ctx, url, req, sha, dir, strategy); err != nil {
		return Snapshot{}, err
	}
	s.markActive(dir)
	return snap, nil
}

// strategyFromMetadata reconstructs the recorded strategy of a cached
// snapshot so provenance reports what actually materialized it.
func strategyFromMetadata(meta SnapshotMetadata, fallback CloneStrategy) CloneStrategy {
	st := fallback
	st.SparsePaths = meta.SparsePaths
	return st
}

// materialize clones repo@sha into dir following the pre-registered plan:
// clone into a temp sibling, verify HEAD, write metadata, rename atomically.
func (s *Service) materialize(ctx context.Context, url string, req SnapshotRequest, sha, dir string, strategy CloneStrategy) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", parent, err)
	}
	tmp := filepath.Join(parent, ".tmp-"+randomSuffix())
	defer os.RemoveAll(tmp)

	if IsCommitSHA(req.Ref) {
		// Exact-SHA materialization: init + shallow fetch of the SHA.
		if err := os.MkdirAll(tmp, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", tmp, err)
		}
		for _, args := range shaFetchPlan(url, sha, strategy) {
			if len(strategy.SparsePaths) > 0 && args[0] == "checkout" {
				if _, err := s.Git.Run(ctx, tmp, sparseCheckoutArgs(strategy.SparsePaths)...); err != nil {
					return err
				}
			}
			if _, err := s.Git.Run(ctx, tmp, args...); err != nil {
				return err
			}
		}
	} else {
		if _, err := s.Git.Run(ctx, "", cloneArgs(url, tmp, req.Ref, strategy)...); err != nil {
			return err
		}
		if len(strategy.SparsePaths) > 0 {
			if _, err := s.Git.Run(ctx, tmp, sparseCheckoutArgs(strategy.SparsePaths)...); err != nil {
				return err
			}
		}
	}

	// The ADR plan ends with rev-parse HEAD: the materialized commit is the
	// snapshot identity and must match what the ref resolved to.
	head, err := s.Git.Run(ctx, tmp, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if head != sha {
		return fmt.Errorf("snapshot HEAD %s does not match resolved sha %s for %s@%s (remote moved mid-clone; retry)", head, sha, req.Repo, req.Ref)
	}

	size, err := dirSizeBytes(tmp)
	if err != nil {
		return fmt.Errorf("size snapshot: %w", err)
	}
	now := s.now().UTC()
	meta := SnapshotMetadata{
		Repo: req.Repo, RemoteURL: url, RequestedRef: req.Ref, ResolvedSHA: sha,
		CloneStrategy: strategy.Label(), SparsePaths: strategy.SparsePaths,
		CreatedAt: now, LastAccessedAt: now, SizeBytes: size, GhxVersion: s.Version,
	}
	if err := WriteMetadata(tmp, meta); err != nil {
		return fmt.Errorf("write snapshot metadata: %w", err)
	}
	if err := os.Rename(tmp, dir); err != nil {
		// A concurrent materialization of the same SHA won the rename; its
		// snapshot is identical by construction (keyed by commit SHA).
		if _, lerr := LoadMetadata(dir); lerr == nil {
			return nil
		}
		return fmt.Errorf("finalize snapshot %s: %w", dir, err)
	}
	return nil
}

// RunCodemap executes one codemap invocation over the snapshot, records the
// stdout hash in the snapshot's tool artifact ledger on success, and returns
// the ToolRun evidence record. Failures return the populated ToolRun plus the
// error — a failed invocation is evidence, not a silent retry.
func (s *Service) RunCodemap(ctx context.Context, snap Snapshot, req CodemapRequest) (ToolRun, error) {
	run, err := s.Codemap.Run(ctx, snap.Dir, req)
	if err != nil {
		return run, err
	}
	s.recordToolArtifact(snap.Dir, req.ArtifactKey(), run.Stdout)
	return run, nil
}

// RunAstGrep executes one structural pattern search over the snapshot,
// records the stdout hash in the snapshot's tool artifact ledger when the
// search ran (matches or clean zero-match), and returns the ToolRun evidence
// record. Failures return the populated ToolRun plus the error — a failed
// invocation is evidence, not a silent retry.
func (s *Service) RunAstGrep(ctx context.Context, snap Snapshot, req AstGrepRequest) (ToolRun, error) {
	run, err := s.AstGrep.Run(ctx, snap.Dir, req)
	if err != nil {
		return run, err
	}
	s.recordToolArtifact(snap.Dir, req.ArtifactKey(), run.Stdout)
	return run, nil
}

// RunRepomap collects file facts from the snapshot working tree, runs the
// pure ranking function, records the canonical JSON hash in the snapshot's
// tool artifact ledger, and returns the budgeted ranking. The recomputable
// evidence command is the ghx invocation itself (`ghx tier2 repomap ...`) —
// there is no external binary behind local:repomap.
func (s *Service) RunRepomap(ctx context.Context, snap Snapshot, req RepomapRequest) (RepomapResult, error) {
	facts, err := CollectFileFacts(ctx, snap.Dir)
	if err != nil {
		return RepomapResult{}, fmt.Errorf("collect repomap facts in %s: %w", snap.Dir, err)
	}
	result := RankFiles(facts, req)
	if data, merr := json.Marshal(result); merr == nil {
		s.recordToolArtifact(snap.Dir, req.ArtifactKey(), string(data))
	}
	return result, nil
}

// recordToolArtifact stores sha256(stdout) under key in the snapshot
// metadata. Best-effort: the hash ledger aids cross-turn reuse detection and
// must never fail a successful tool run.
func (s *Service) recordToolArtifact(snapshotDir, key, stdout string) {
	meta, err := LoadMetadata(snapshotDir)
	if err != nil {
		return
	}
	if meta.ToolArtifactHashes == nil {
		meta.ToolArtifactHashes = map[string]string{}
	}
	sum := sha256.Sum256([]byte(stdout))
	meta.ToolArtifactHashes[key] = hex.EncodeToString(sum[:])
	meta.LastAccessedAt = s.now().UTC()
	_ = WriteMetadata(snapshotDir, meta)
}

// randomSuffix returns a short hex suffix for temp materialization dirs.
func randomSuffix() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
