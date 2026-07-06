// Package tier2 implements local structural analysis over cloned repo
// snapshots (ADR-0024.1, NORTH_STAR P3 "Swallow the tools", milestone M7).
//
// Tier 2 begins only after a local snapshot exists. The package owns three
// concerns behind one Service: the ~/.ghx snapshot cache (this file), shallow
// blobless clone materialization (clone.go), and absorbed structural tools
// starting with codemap (codemap.go). Every Tier-2 action is visible: snapshot
// provenance is emitted before any structural tool runs, tool invocations are
// returned as ToolRun evidence, and cache eviction surfaces as runtime events
// — never as report claims.
package tier2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultBudgetBytes is the pre-registered cache size cap: 5 GiB under
	// the tier2 cache root (ADR-0024.1 "Eviction").
	DefaultBudgetBytes int64 = 5 << 30
	// DefaultTTL is the pre-registered snapshot lifetime: 30 days since
	// lastAccessedAt (ADR-0024.1 "Eviction").
	DefaultTTL = 30 * 24 * time.Hour
	// MetadataFileName is the per-snapshot provenance file, written inside the
	// snapshot directory (ADR-0024.1 "Clone and Cache Strategy").
	MetadataFileName = "ghx-cache.json"
	// snapshotHost is the fixed host segment of the cache layout. ghx talks to
	// github.com only; the segment exists so the layout can grow other hosts
	// without a migration.
	snapshotHost = "github.com"
)

// SnapshotMetadata is the ghx-cache.json record stored inside every snapshot
// directory. It is provenance, not evidence: reports must cite session
// artifacts and recomputable commands, never these cache paths (ADR-0024.1
// "Visibility Contract").
type SnapshotMetadata struct {
	// Repo is the owner/repo full name the snapshot was created for.
	Repo string `json:"repo"`
	// RemoteURL is the clone URL the snapshot was fetched from.
	RemoteURL string `json:"remoteUrl"`
	// RequestedRef is the ref the caller asked for ("" means default branch).
	RequestedRef string `json:"requestedRef"`
	// ResolvedSHA is the full commit SHA the ref resolved to — the snapshot key.
	ResolvedSHA string `json:"resolvedSha"`
	// CloneStrategy is the canonical strategy label (see CloneStrategy.Label).
	CloneStrategy string `json:"cloneStrategy"`
	// SparsePaths lists the sparse-checkout paths, empty for a full checkout.
	SparsePaths []string `json:"sparsePaths,omitempty"`
	// CreatedAt is when the snapshot finished materializing (UTC).
	CreatedAt time.Time `json:"createdAt"`
	// LastAccessedAt drives TTL and LRU eviction; updated on every cache hit.
	LastAccessedAt time.Time `json:"lastAccessedAt"`
	// SizeBytes is the on-disk size of the snapshot directory at creation.
	SizeBytes int64 `json:"sizeBytes"`
	// GhxVersion is the ghx binary version that created the snapshot.
	GhxVersion string `json:"ghxVersion"`
	// ToolArtifactHashes maps a tool operation (e.g. "codemap:overview") to the
	// SHA-256 of its stdout, enabling cache-reuse detection across turns.
	ToolArtifactHashes map[string]string `json:"toolArtifactHashes,omitempty"`
}

// EvictionEvent records one evicted snapshot. Events are local runtime
// log material, not report claims (ADR-0024.1 "Eviction").
type EvictionEvent struct {
	// Repo is the owner/repo the evicted snapshot belonged to.
	Repo string
	// SHA is the evicted snapshot's resolved commit SHA.
	SHA string
	// Path is the removed snapshot directory.
	Path string
	// Reason is "ttl" (expired) or "budget" (LRU under the size cap).
	Reason string
	// FreedBytes is the metadata-recorded size of the removed snapshot.
	FreedBytes int64
}

// Cache owns the Tier-2 snapshot cache layout under
// <root>/repos/github.com/<owner>/<repo>/snapshots/<sha>/ and its
// pre-registered eviction policy (ADR-0024.1 "Clone and Cache Strategy").
type Cache struct {
	// Root is the tier2 cache root, e.g. ~/.ghx/cache/tier2.
	Root string
	// BudgetBytes caps the total snapshot bytes kept (LRU beyond it).
	BudgetBytes int64
	// TTL expires snapshots not accessed within the window.
	TTL time.Duration
}

// NewCache returns a Cache rooted at <ghxRoot>/cache/tier2 with the
// pre-registered defaults (5 GiB budget, 30-day TTL).
func NewCache(ghxRoot string) *Cache {
	return &Cache{
		Root:        filepath.Join(ghxRoot, "cache", "tier2"),
		BudgetBytes: DefaultBudgetBytes,
		TTL:         DefaultTTL,
	}
}

// repoSegmentRE matches one safe path segment of an owner or repo name.
// GitHub allows alphanumerics, hyphens, underscores, and dots; the pattern
// additionally refuses leading dots so "." and ".." can never be segments.
var repoSegmentRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$`)

// shaRE matches a full 40-hex commit SHA — the only accepted snapshot key.
var shaRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

// SplitRepo validates and splits an "owner/repo" full name into safe path
// segments, rejecting anything that could traverse outside the cache layout.
func SplitRepo(repo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || !repoSegmentRE.MatchString(owner) || !repoSegmentRE.MatchString(name) {
		return "", "", fmt.Errorf("invalid repo %q: want owner/repo with [A-Za-z0-9._-] segments", repo)
	}
	return owner, name, nil
}

// repoDir returns <root>/repos/github.com/<owner>/<repo> for a validated repo.
func (c *Cache) repoDir(repo string) (string, error) {
	owner, name, err := SplitRepo(repo)
	if err != nil {
		return "", err
	}
	return filepath.Join(c.Root, "repos", snapshotHost, owner, name), nil
}

// SnapshotDir returns the snapshot directory for repo at sha, validating both
// inputs. The directory may not exist yet.
func (c *Cache) SnapshotDir(repo, sha string) (string, error) {
	dir, err := c.repoDir(repo)
	if err != nil {
		return "", err
	}
	if !shaRE.MatchString(sha) {
		return "", fmt.Errorf("invalid snapshot sha %q: want 40-hex commit sha", sha)
	}
	return filepath.Join(dir, "snapshots", sha), nil
}

// metadataPath returns the ghx-cache.json path inside a snapshot directory.
func metadataPath(snapshotDir string) string {
	return filepath.Join(snapshotDir, MetadataFileName)
}

// LoadMetadata reads the ghx-cache.json of one snapshot directory.
func LoadMetadata(snapshotDir string) (SnapshotMetadata, error) {
	var m SnapshotMetadata
	data, err := os.ReadFile(metadataPath(snapshotDir))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("corrupt %s in %s: %w", MetadataFileName, snapshotDir, err)
	}
	return m, nil
}

// WriteMetadata writes the ghx-cache.json of one snapshot directory.
func WriteMetadata(snapshotDir string, m SnapshotMetadata) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metadataPath(snapshotDir), append(data, '\n'), 0o644)
}

// Touch updates lastAccessedAt to now, keeping LRU order honest on cache hits.
func Touch(snapshotDir string, now time.Time) error {
	m, err := LoadMetadata(snapshotDir)
	if err != nil {
		return err
	}
	m.LastAccessedAt = now.UTC()
	return WriteMetadata(snapshotDir, m)
}

// snapshotEntry pairs a snapshot directory with its loaded metadata during an
// eviction pass.
type snapshotEntry struct {
	dir  string
	meta SnapshotMetadata
}

// Evict applies the pre-registered eviction policy (ADR-0024.1): first remove
// snapshots whose lastAccessedAt is older than the TTL, then remove
// least-recently-accessed snapshots until total size fits BudgetBytes.
// Snapshot directories in active are never evicted (they belong to the running
// turn). Evict runs before every new snapshot is created.
func (c *Cache) Evict(now time.Time, active map[string]bool) ([]EvictionEvent, error) {
	entries, err := c.listSnapshots()
	if err != nil {
		return nil, err
	}
	var events []EvictionEvent
	remove := func(e snapshotEntry, reason string) error {
		if err := os.RemoveAll(e.dir); err != nil {
			return fmt.Errorf("evict %s: %w", e.dir, err)
		}
		events = append(events, EvictionEvent{
			Repo: e.meta.Repo, SHA: e.meta.ResolvedSHA, Path: e.dir,
			Reason: reason, FreedBytes: e.meta.SizeBytes,
		})
		return nil
	}

	var kept []snapshotEntry
	var total int64
	cutoff := now.Add(-c.TTL)
	for _, e := range entries {
		if !active[e.dir] && e.meta.LastAccessedAt.Before(cutoff) {
			if err := remove(e, "ttl"); err != nil {
				return events, err
			}
			continue
		}
		kept = append(kept, e)
		total += e.meta.SizeBytes
	}

	// LRU under the byte budget: oldest lastAccessedAt goes first.
	sort.Slice(kept, func(i, j int) bool {
		return kept[i].meta.LastAccessedAt.Before(kept[j].meta.LastAccessedAt)
	})
	for _, e := range kept {
		if total <= c.BudgetBytes {
			break
		}
		if active[e.dir] {
			continue
		}
		if err := remove(e, "budget"); err != nil {
			return events, err
		}
		total -= e.meta.SizeBytes
	}
	return events, nil
}

// listSnapshots walks <root>/repos/<host>/<owner>/<repo>/snapshots/<sha> and
// returns every snapshot that carries readable metadata. Directories without
// metadata (interrupted materializations) are skipped: materialization is
// atomic-by-rename, so they can only be stray temp dirs which the next
// materialization overwrites.
func (c *Cache) listSnapshots() ([]snapshotEntry, error) {
	reposRoot := filepath.Join(c.Root, "repos")
	var entries []snapshotEntry
	err := filepath.WalkDir(reposRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() || filepath.Base(filepath.Dir(path)) != "snapshots" {
			return nil
		}
		meta, merr := LoadMetadata(path)
		if merr == nil {
			entries = append(entries, snapshotEntry{dir: path, meta: meta})
		}
		return fs.SkipDir // never descend into snapshot working trees
	})
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	return entries, err
}

// dirSizeBytes sums the file sizes under dir, recording snapshot weight for
// budget eviction.
func dirSizeBytes(dir string) (int64, error) {
	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
