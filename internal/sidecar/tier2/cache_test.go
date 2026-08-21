package tier2

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSplitRepoValidation(t *testing.T) {
	valid := []string{"gin-gonic/gin", "openai/openai-node", "a/b", "Owner_1/repo.name-x"}
	for _, repo := range valid {
		if _, _, err := SplitRepo(repo); err != nil {
			t.Errorf("SplitRepo(%q) unexpected error: %v", repo, err)
		}
	}
	invalid := []string{"", "gin", "a/b/c", "../etc/passwd", "a/..", "../b", "a/.hidden", ".a/b", "a/", "/b", "owner/re po"}
	for _, repo := range invalid {
		if _, _, err := SplitRepo(repo); err == nil {
			t.Errorf("SplitRepo(%q) want error, got nil", repo)
		}
	}
}

func TestSnapshotDirLayoutAndSHAValidation(t *testing.T) {
	c := NewCache("/home/u/.ghx")
	sha := "0123456789abcdef0123456789abcdef01234567"
	dir, err := c.SnapshotDir("gin-gonic/gin", sha)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join("/home/u/.ghx", "cache", "tier2", "repos", "github.com", "gin-gonic", "gin", "snapshots", sha)
	if dir != want {
		t.Errorf("SnapshotDir = %q, want %q (ADR-0024.1 layout)", dir, want)
	}
	for _, bad := range []string{"", "main", "0123456789ABCDEF0123456789abcdef01234567", "../../../evil", "0123456789abcdef0123456789abcdef0123456"} {
		if _, err := c.SnapshotDir("gin-gonic/gin", bad); err == nil {
			t.Errorf("SnapshotDir sha=%q want error, got nil", bad)
		}
	}
}

func TestMetadataRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	meta := SnapshotMetadata{
		Repo: "o/r", RemoteURL: "https://github.com/o/r.git", RequestedRef: "main",
		ResolvedSHA:   "0123456789abcdef0123456789abcdef01234567",
		CloneStrategy: "shallow-blobless", SparsePaths: []string{"src"},
		CreatedAt: now, LastAccessedAt: now, SizeBytes: 42, GhxVersion: "test",
		ToolArtifactHashes: map[string]string{"codemap:overview": "abc"},
	}
	if err := WriteMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != meta.Repo || got.ResolvedSHA != meta.ResolvedSHA ||
		got.CloneStrategy != meta.CloneStrategy || got.SizeBytes != 42 ||
		!got.LastAccessedAt.Equal(now) || got.ToolArtifactHashes["codemap:overview"] != "abc" {
		t.Errorf("metadata round trip mismatch: %+v", got)
	}
}

func TestTouchUpdatesLastAccessed(t *testing.T) {
	dir := t.TempDir()
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := WriteMetadata(dir, SnapshotMetadata{Repo: "o/r", LastAccessedAt: old}); err != nil {
		t.Fatal(err)
	}
	newer := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	if err := Touch(dir, newer); err != nil {
		t.Fatal(err)
	}
	got, err := LoadMetadata(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastAccessedAt.Equal(newer) {
		t.Errorf("lastAccessedAt = %v, want %v", got.LastAccessedAt, newer)
	}
}

// makeSnapshot materializes a fake cached snapshot with metadata for eviction
// tests.
func makeSnapshot(t *testing.T, c *Cache, repo, sha string, accessed time.Time, size int64) string {
	t.Helper()
	dir, err := c.SnapshotDir(repo, sha)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := SnapshotMetadata{
		Repo: repo, ResolvedSHA: sha, CloneStrategy: "shallow-blobless",
		CreatedAt: accessed, LastAccessedAt: accessed, SizeBytes: size, GhxVersion: "test",
	}
	if err := WriteMetadata(dir, meta); err != nil {
		t.Fatal(err)
	}
	return dir
}

func shaN(n byte) string {
	b := make([]byte, 40)
	for i := range b {
		b[i] = "0123456789abcdef"[n%16]
	}
	return string(b)
}

func TestEvictTTL(t *testing.T) {
	c := NewCache(t.TempDir())
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	expired := makeSnapshot(t, c, "o/old", shaN(1), now.Add(-31*24*time.Hour), 10)
	fresh := makeSnapshot(t, c, "o/new", shaN(2), now.Add(-time.Hour), 10)

	events, err := c.Evict(now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Reason != "ttl" || events[0].Path != expired {
		t.Fatalf("events = %+v, want one ttl eviction of %s", events, expired)
	}
	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Error("expired snapshot still exists")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("fresh snapshot was evicted")
	}
}

func TestEvictLRUBudget(t *testing.T) {
	c := NewCache(t.TempDir())
	c.BudgetBytes = 25
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	oldest := makeSnapshot(t, c, "o/a", shaN(1), now.Add(-3*time.Hour), 10)
	middle := makeSnapshot(t, c, "o/b", shaN(2), now.Add(-2*time.Hour), 10)
	newest := makeSnapshot(t, c, "o/c", shaN(3), now.Add(-1*time.Hour), 10)

	events, err := c.Evict(now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Reason != "budget" || events[0].Path != oldest {
		t.Fatalf("events = %+v, want one budget eviction of oldest %s", events, oldest)
	}
	for _, dir := range []string{middle, newest} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("snapshot %s should survive: %v", dir, err)
		}
	}
}

func TestEvictNeverRemovesActiveSnapshot(t *testing.T) {
	c := NewCache(t.TempDir())
	c.BudgetBytes = 5 // everything over budget
	now := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	activeExpired := makeSnapshot(t, c, "o/a", shaN(1), now.Add(-40*24*time.Hour), 10)
	idle := makeSnapshot(t, c, "o/b", shaN(2), now.Add(-time.Hour), 10)

	events, err := c.Evict(now, map[string]bool{activeExpired: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(activeExpired); err != nil {
		t.Fatal("active snapshot was evicted — violates ADR-0024.1 eviction rule")
	}
	if _, err := os.Stat(idle); !os.IsNotExist(err) {
		t.Error("idle snapshot should have been evicted to chase the budget")
	}
	for _, e := range events {
		if e.Path == activeExpired {
			t.Error("eviction event recorded for the active snapshot")
		}
	}
}

func TestEvictEmptyCacheIsNoop(t *testing.T) {
	c := NewCache(t.TempDir())
	events, err := c.Evict(time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("events = %+v, want none", events)
	}
}

// ADR-0024.4 D4: ListSnapshots projects cached snapshots oldest-access first
// (eviction order); Clean removes everything with manual-clean events.
func TestCacheListAndClean(t *testing.T) {
	root := t.TempDir()
	c := NewCache(root)
	if err := os.MkdirAll(filepath.Join(c.Root, "repos", "host", "o", "r1", "snapshots", "aaa"), 0o755); err != nil {
		t.Fatal(err)
	}
	metaA := SnapshotMetadata{Repo: "o/r1", ResolvedSHA: "aaa", CloneStrategy: "blobless", SizeBytes: 100, CreatedAt: time.Now().Add(-time.Hour), LastAccessedAt: time.Now().Add(-2 * time.Hour)}
	if err := WriteMetadata(filepath.Join(c.Root, "repos", "host", "o", "r1", "snapshots", "aaa"), metaA); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(c.Root, "repos", "host", "o", "r2", "snapshots", "bbb"), 0o755); err != nil {
		t.Fatal(err)
	}
	metaB := SnapshotMetadata{Repo: "o/r2", ResolvedSHA: "bbb", CloneStrategy: "blobless", SizeBytes: 200, CreatedAt: time.Now(), LastAccessedAt: time.Now()}
	if err := WriteMetadata(filepath.Join(c.Root, "repos", "host", "o", "r2", "snapshots", "bbb"), metaB); err != nil {
		t.Fatal(err)
	}

	infos, err := c.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 {
		t.Fatalf("ListSnapshots = %d entries, want 2", len(infos))
	}
	if infos[0].Repo != "o/r1" || infos[1].Repo != "o/r2" {
		t.Fatalf("oldest-access-first order violated: %s before %s", infos[0].Repo, infos[1].Repo)
	}

	events, err := c.Clean()
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("Clean events = %d, want 2", len(events))
	}
	for _, e := range events {
		if e.Reason != "manual-clean" {
			t.Fatalf("event reason = %q, want manual-clean", e.Reason)
		}
	}
	after, err := c.ListSnapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 0 {
		t.Fatalf("cache not empty after Clean: %d entries", len(after))
	}
}
