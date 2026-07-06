package tier2

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCloneStrategyLabel(t *testing.T) {
	cases := []struct {
		strategy CloneStrategy
		want     string
	}{
		{DefaultCloneStrategy(nil), "shallow-blobless"},
		{DefaultCloneStrategy([]string{"src"}), "shallow-blobless-sparse"},
		{CloneStrategy{Depth: 1}, "shallow"},
		{CloneStrategy{Depth: 1, Filter: "tree:0"}, "shallow-tree-0"},
	}
	for _, c := range cases {
		if got := c.strategy.Label(); got != c.want {
			t.Errorf("Label() = %q, want %q", got, c.want)
		}
	}
}

// TestCloneArgsConstruction pins the exact clone plan pre-registered in
// ADR-0024.1 "Clone and Cache Strategy".
func TestCloneArgsConstruction(t *testing.T) {
	s := DefaultCloneStrategy(nil)
	got := cloneArgs("https://github.com/o/r.git", "/tmp/snap", "", s)
	want := []string{"clone", "--depth=1", "--filter=blob:none", "--no-tags", "--", "https://github.com/o/r.git", "/tmp/snap"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("default clone args = %v, want %v", got, want)
	}

	sparse := DefaultCloneStrategy([]string{"src", "docs"})
	got = cloneArgs("u", "d", "main", sparse)
	want = []string{"clone", "--depth=1", "--filter=blob:none", "--no-tags", "--sparse", "--branch", "main", "--", "u", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sparse branch clone args = %v, want %v", got, want)
	}

	if got := sparseCheckoutArgs([]string{"src", "docs"}); !reflect.DeepEqual(got, []string{"sparse-checkout", "set", "src", "docs"}) {
		t.Errorf("sparseCheckoutArgs = %v", got)
	}
}

func TestSHAFetchPlanConstruction(t *testing.T) {
	sha := shaN(4)
	plan := shaFetchPlan("https://github.com/o/r.git", sha, DefaultCloneStrategy(nil))
	want := [][]string{
		{"init", "--quiet"},
		{"remote", "add", "origin", "https://github.com/o/r.git"},
		{"fetch", "--depth=1", "--filter=blob:none", "--no-tags", "origin", sha},
		{"checkout", "--quiet", "--detach", "FETCH_HEAD"},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Errorf("shaFetchPlan = %v, want %v", plan, want)
	}
}

// fakeGit records ls-remote invocations and returns canned output.
type fakeGit struct {
	out   string
	err   error
	calls [][]string
}

func (f *fakeGit) Run(_ context.Context, dir string, args ...string) (string, error) {
	f.calls = append(f.calls, args)
	return f.out, f.err
}

func TestResolveRefSHAPassthroughSkipsNetwork(t *testing.T) {
	git := &fakeGit{}
	sha := shaN(5)
	got, err := ResolveRef(context.Background(), git, "url", sha)
	if err != nil || got != sha {
		t.Fatalf("ResolveRef sha = %q, %v", got, err)
	}
	if len(git.calls) != 0 {
		t.Errorf("full-sha ref must not hit the network, got calls %v", git.calls)
	}
}

func TestResolveRefHEADAndBranchPriority(t *testing.T) {
	sha1, sha2 := shaN(1), shaN(2)
	git := &fakeGit{out: sha1 + "\tHEAD"}
	got, err := ResolveRef(context.Background(), git, "url", "")
	if err != nil || got != sha1 {
		t.Fatalf("HEAD resolve = %q, %v", got, err)
	}

	// A tag and branch sharing the short name: the branch wins.
	git = &fakeGit{out: sha2 + "\trefs/tags/main\n" + sha1 + "\trefs/heads/main"}
	got, err = ResolveRef(context.Background(), git, "url", "main")
	if err != nil || got != sha1 {
		t.Fatalf("branch-priority resolve = %q, %v (want heads sha)", got, err)
	}

	git = &fakeGit{out: ""}
	if _, err := ResolveRef(context.Background(), git, "url", "gone"); err == nil {
		t.Error("missing ref must error")
	}
}

// --- fixture-repo integration (local git, no network) ---

// initFixtureRepo builds a real git repo in a tempdir with two commits on the
// default branch and a side branch, returning its file:// URL and commit SHAs.
func initFixtureRepo(t *testing.T) (url, headSHA, oldSHA, branchSHA string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := ExecGit{}.Run(context.Background(), dir, args...)
		if err != nil {
			t.Fatalf("fixture git %v: %v", args, err)
		}
		return out
	}
	run("init", "--quiet", "--initial-branch=main")
	run("config", "user.email", "fixture@test")
	run("config", "user.name", "fixture")
	// Allow arbitrary-SHA fetches so the exact-SHA materialization path works
	// against the file:// transport, mirroring GitHub's reachable-SHA support.
	run("config", "uploadpack.allowAnySHA1InWant", "true")

	writeFile(t, dir, "main.go", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hi\") }\n")
	writeFile(t, dir, filepath.Join("src", "lib.go"), "package src\n\nfunc Lib() int { return 1 }\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "c1")
	oldSHA = run("rev-parse", "HEAD")

	writeFile(t, dir, filepath.Join("docs", "readme.md"), "# fixture\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "c2")
	headSHA = run("rev-parse", "HEAD")

	run("checkout", "--quiet", "-b", "feature")
	writeFile(t, dir, "feature.txt", "feature\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "c3")
	branchSHA = run("rev-parse", "HEAD")
	run("checkout", "--quiet", "main")

	return "file://" + dir, headSHA, oldSHA, branchSHA
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// newTestService wires a Service against a temp ghx root and the real git.
func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(t.TempDir(), "test")
}

func TestSnapshotMaterializeAndCacheHit(t *testing.T) {
	url, headSHA, _, _ := initFixtureRepo(t)
	svc := newTestService(t)
	req := SnapshotRequest{Repo: "fixture/repo", RemoteURL: url}

	snap, err := svc.Snapshot(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CacheHit {
		t.Error("first materialization must be a cache miss")
	}
	if snap.SHA != headSHA {
		t.Errorf("snapshot sha = %s, want default-branch head %s", snap.SHA, headSHA)
	}
	if _, err := os.Stat(filepath.Join(snap.Dir, "main.go")); err != nil {
		t.Errorf("materialized file missing: %v", err)
	}
	meta, err := LoadMetadata(snap.Dir)
	if err != nil {
		t.Fatalf("snapshot metadata missing: %v", err)
	}
	if meta.ResolvedSHA != headSHA || meta.CloneStrategy != "shallow-blobless" ||
		meta.GhxVersion != "test" || meta.SizeBytes <= 0 || meta.CreatedAt.IsZero() {
		t.Errorf("metadata incomplete: %+v", meta)
	}

	// Second request: cache hit, same SHA snapshot reused, lastAccessedAt moves.
	later := time.Now().Add(time.Hour)
	svc.now = func() time.Time { return later }
	snap2, err := svc.Snapshot(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !snap2.CacheHit || snap2.Dir != snap.Dir || snap2.SHA != headSHA {
		t.Errorf("cache hit expected: %+v", snap2)
	}
	meta2, _ := LoadMetadata(snap.Dir)
	if !meta2.LastAccessedAt.After(meta.LastAccessedAt) {
		t.Error("cache hit must advance lastAccessedAt")
	}
}

func TestSnapshotBranchRef(t *testing.T) {
	url, _, _, branchSHA := initFixtureRepo(t)
	svc := newTestService(t)
	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url, Ref: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.SHA != branchSHA {
		t.Errorf("branch snapshot sha = %s, want %s", snap.SHA, branchSHA)
	}
	if _, err := os.Stat(filepath.Join(snap.Dir, "feature.txt")); err != nil {
		t.Errorf("branch file missing: %v", err)
	}
}

func TestSnapshotExactSHARef(t *testing.T) {
	url, headSHA, oldSHA, _ := initFixtureRepo(t)
	svc := newTestService(t)
	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url, Ref: oldSHA})
	if err != nil {
		t.Fatal(err)
	}
	if snap.SHA != oldSHA {
		t.Errorf("sha snapshot = %s, want requested %s", snap.SHA, oldSHA)
	}
	if snap.SHA == headSHA {
		t.Error("must pin the requested historical commit, not HEAD")
	}
	// c1 has no docs/ yet.
	if _, err := os.Stat(filepath.Join(snap.Dir, "docs", "readme.md")); !os.IsNotExist(err) {
		t.Error("historical snapshot should not contain files from newer commits")
	}
}

func TestSnapshotSparseCheckout(t *testing.T) {
	url, _, _, _ := initFixtureRepo(t)
	svc := newTestService(t)
	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url, SparsePaths: []string{"src"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(snap.Dir, "src", "lib.go")); err != nil {
		t.Errorf("sparse path missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(snap.Dir, "docs", "readme.md")); !os.IsNotExist(err) {
		t.Error("non-sparse path should not be checked out")
	}
	meta, _ := LoadMetadata(snap.Dir)
	if len(meta.SparsePaths) != 1 || meta.SparsePaths[0] != "src" {
		t.Errorf("sparse paths not recorded: %+v", meta.SparsePaths)
	}
	if meta.CloneStrategy != "shallow-blobless-sparse" {
		t.Errorf("strategy = %s, want shallow-blobless-sparse", meta.CloneStrategy)
	}
}

func TestSnapshotProvenanceLines(t *testing.T) {
	url, headSHA, _, _ := initFixtureRepo(t)
	svc := newTestService(t)
	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Join(snap.Provenance().Lines(), "\n")
	for _, want := range []string{"repo=fixture/repo", "ref=HEAD", "sha=" + headSHA, "strategy=shallow-blobless", "cache=miss", "path=" + snap.Dir} {
		if !strings.Contains(lines, want) {
			t.Errorf("provenance %q missing %q", lines, want)
		}
	}
}

func TestSnapshotRejectsInvalidRepo(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "../evil"}); err == nil {
		t.Fatal("invalid repo must be rejected before any network call")
	}
}
