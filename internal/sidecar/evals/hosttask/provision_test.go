package hosttask

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// gitCall records one GitRunner invocation.
type gitCall struct {
	dir  string
	args []string
}

// recordingGit is a hermetic GitRunner: it records invocations, fails on an
// optional argv substring, and answers rev-parse with a canned SHA.
type recordingGit struct {
	sha    string
	failOn string
	calls  []gitCall
}

func (g *recordingGit) Run(_ context.Context, dir string, args ...string) (string, error) {
	g.calls = append(g.calls, gitCall{dir: dir, args: args})
	joined := strings.Join(args, " ")
	if g.failOn != "" && strings.Contains(joined, g.failOn) {
		return "", errors.New("fake git failure: " + joined)
	}
	if args[0] == "rev-parse" {
		return g.sha, nil
	}
	return "", nil
}

// TestProvisionExecutesSharedSHAPlan pins that the provisioner runs exactly
// the tier2 SHA-fetch plan (full blobs, shallow, no tags) inside the fresh
// workspace dir, then verifies HEAD.
func TestProvisionExecutesSharedSHAPlan(t *testing.T) {
	f := validFixture()
	git := &recordingGit{sha: f.PinnedSHA}
	p := &Provisioner{Git: git, Root: t.TempDir()}

	ws, err := p.Provision(context.Background(), f, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ws.FixtureID != f.ID || ws.SHA != f.PinnedSHA {
		t.Errorf("workspace identity = %+v", ws)
	}

	want := tier2.SHAFetchPlan(f.RemoteURL(), f.PinnedSHA, workspaceCloneStrategy())
	want = append(want, []string{"rev-parse", "HEAD"})
	if len(git.calls) != len(want) {
		t.Fatalf("git calls = %d, want %d: %v", len(git.calls), len(want), git.calls)
	}
	for i, call := range git.calls {
		if !reflect.DeepEqual(call.args, want[i]) {
			t.Errorf("call %d = %v, want %v", i, call.args, want[i])
		}
		if call.dir != ws.Dir {
			t.Errorf("call %d ran in %q, want workspace %q", i, call.dir, ws.Dir)
		}
	}
	// The workspace plan must NOT be blobless: a promisor clone would
	// lazy-fetch over the network and grader containers run network-off.
	fetch := strings.Join(git.calls[2].args, " ")
	if strings.Contains(fetch, "--filter") {
		t.Errorf("workspace fetch must carry full blobs, got %q", fetch)
	}
	// Derived URL points at the canonical GitHub remote.
	if remote := strings.Join(git.calls[1].args, " "); !strings.Contains(remote, "https://github.com/acme/widgets.git") {
		t.Errorf("remote add = %q, want canonical GitHub URL", remote)
	}
}

// TestProvisionFreshDirPerTrial pins the per-trial reset guarantee
// (ADR-0032 trap 5): every call yields a brand-new directory.
func TestProvisionFreshDirPerTrial(t *testing.T) {
	f := validFixture()
	root := t.TempDir()
	p := &Provisioner{Git: &recordingGit{sha: f.PinnedSHA}, Root: root}

	ws1, err := p.Provision(context.Background(), f, 1)
	if err != nil {
		t.Fatal(err)
	}
	ws2, err := p.Provision(context.Background(), f, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ws1.Dir == ws2.Dir {
		t.Fatalf("trials share a workspace dir: %s", ws1.Dir)
	}
	for _, ws := range []Workspace{ws1, ws2} {
		if filepath.Dir(ws.Dir) != root {
			t.Errorf("workspace %s not under root %s", ws.Dir, root)
		}
		if base := filepath.Base(ws.Dir); !strings.HasPrefix(base, "demo-task-trial001-") {
			t.Errorf("workspace dir name %q missing fixture/trial prefix", base)
		}
		if _, err := os.Stat(ws.Dir); err != nil {
			t.Errorf("workspace dir missing: %v", err)
		}
	}
	if err := ws1.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ws1.Dir); !os.IsNotExist(err) {
		t.Error("Remove must delete the workspace dir")
	}
}

func TestProvisionHEADMismatchFailsAndCleansUp(t *testing.T) {
	f := validFixture()
	root := t.TempDir()
	p := &Provisioner{Git: &recordingGit{sha: strings.Repeat("cd", 20)}, Root: root}
	_, err := p.Provision(context.Background(), f, 2)
	if err == nil || !strings.Contains(err.Error(), "does not match pinnedSha") {
		t.Fatalf("want HEAD-mismatch error, got %v", err)
	}
	assertEmptyDir(t, root)
}

func TestProvisionGitFailureCleansUp(t *testing.T) {
	f := validFixture()
	root := t.TempDir()
	p := &Provisioner{Git: &recordingGit{sha: f.PinnedSHA, failOn: "fetch"}, Root: root}
	if _, err := p.Provision(context.Background(), f, 1); err == nil {
		t.Fatal("want git failure to propagate")
	}
	assertEmptyDir(t, root)
}

func TestProvisionRejectsInvalidFixture(t *testing.T) {
	f := validFixture()
	f.PinnedSHA = "nope"
	git := &recordingGit{}
	p := &Provisioner{Git: git, Root: t.TempDir()}
	if _, err := p.Provision(context.Background(), f, 1); err == nil {
		t.Fatal("want validation error")
	}
	if len(git.calls) != 0 {
		t.Errorf("invalid fixture must not shell out, got %v", git.calls)
	}
}

// assertEmptyDir fails the test when dir has surviving entries — failed
// provisions must not leave half-materialized workspaces behind.
func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("failed provision left %d entries in %s", len(entries), dir)
	}
}

// --- real-git fixture test (local file:// transport, no network) ---

// initWorkspaceFixtureRepo builds a real git repo with two commits and
// returns its file:// URL plus both SHAs (the tier2 clone_test pattern).
func initWorkspaceFixtureRepo(t *testing.T) (url, headSHA, oldSHA string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := tier2.ExecGit{}.Run(context.Background(), dir, args...)
		if err != nil {
			t.Fatalf("fixture git %v: %v", args, err)
		}
		return out
	}
	run("init", "--quiet", "--initial-branch=main")
	run("config", "user.email", "fixture@test")
	run("config", "user.name", "fixture")
	// Allow arbitrary-SHA fetches over the file:// transport, mirroring
	// GitHub's reachable-SHA support that the provisioner relies on.
	run("config", "uploadpack.allowAnySHA1InWant", "true")

	writeWorkspaceFile(t, dir, "hello.txt", "hello world\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "c1")
	oldSHA = run("rev-parse", "HEAD")

	writeWorkspaceFile(t, dir, "later.txt", "added after the pin\n")
	run("add", ".")
	run("commit", "--quiet", "-m", "c2")
	headSHA = run("rev-parse", "HEAD")
	return "file://" + dir, headSHA, oldSHA
}

func writeWorkspaceFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestProvisionRealGitPinsExactSHA exercises the provisioner against real
// git over a local fixture: the workspace holds exactly the pinned commit,
// and a second trial starts clean even after the first was dirtied.
func TestProvisionRealGitPinsExactSHA(t *testing.T) {
	url, headSHA, oldSHA := initWorkspaceFixtureRepo(t)
	f := validFixture()
	f.PinnedSHA = oldSHA
	p := NewProvisioner(t.TempDir())
	p.RemoteURL = url

	ws, err := p.Provision(context.Background(), f, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ws.SHA != oldSHA || ws.SHA == headSHA {
		t.Errorf("workspace pinned %s, want %s (not HEAD %s)", ws.SHA, oldSHA, headSHA)
	}
	data, err := os.ReadFile(filepath.Join(ws.Dir, "hello.txt"))
	if err != nil || string(data) != "hello world\n" {
		t.Errorf("pinned file wrong: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(ws.Dir, "later.txt")); !os.IsNotExist(err) {
		t.Error("workspace must not contain files from commits after the pin")
	}

	// Dirty the first workspace, then provision the next trial: it must be
	// a different, pristine directory (per-trial reset with real git).
	writeWorkspaceFile(t, ws.Dir, "hello.txt", "agent edits\n")
	ws2, err := p.Provision(context.Background(), f, 2)
	if err != nil {
		t.Fatal(err)
	}
	if ws2.Dir == ws.Dir {
		t.Fatal("second trial reused the first workspace dir")
	}
	data, err = os.ReadFile(filepath.Join(ws2.Dir, "hello.txt"))
	if err != nil || string(data) != "hello world\n" {
		t.Errorf("fresh trial not pristine: %q, %v", data, err)
	}
}
