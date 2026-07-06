package tier2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// installFakeAstGrep writes an executable shim named ast-grep that records
// its argv and cwd, prints canned stdout (and stderr when stderrMsg is
// non-empty), and exits with the given code. Points the adapter's lookPath at
// the shim and returns the argv record file path.
func installFakeAstGrep(t *testing.T, g *AstGrep, stdout, stderrMsg string, exitCode int) (recordFile string) {
	t.Helper()
	dir := t.TempDir()
	recordFile = filepath.Join(dir, "argv.txt")
	shim := filepath.Join(dir, "ast-grep")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + recordFile + "\n" +
		"pwd >> " + recordFile + "\n" +
		"printf '%s' '" + stdout + "'\n"
	if stderrMsg != "" {
		script += "echo '" + stderrMsg + "' >&2\n"
	}
	script += "exit " + strconv.Itoa(exitCode) + "\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	g.lookPath = func(name string) (string, error) {
		if name != "ast-grep" {
			return "", errors.New("unexpected lookup: " + name)
		}
		return shim, nil
	}
	return recordFile
}

func TestAstGrepRequestArgs(t *testing.T) {
	cases := []struct {
		req  AstGrepRequest
		want string
	}{
		{AstGrepRequest{Pattern: "fmt.Println($A)"}, "run --pattern fmt.Println($A) --json=compact ."},
		{AstGrepRequest{Pattern: "compose($$$)", Lang: "ts"}, "run --pattern compose($$$) --lang ts --json=compact ."},
		{AstGrepRequest{Pattern: "$A := $B", Lang: "go", Paths: []string{"src", "cmd"}}, "run --pattern $A := $B --lang go --json=compact src cmd"},
	}
	for _, c := range cases {
		args, err := c.req.args()
		if err != nil {
			t.Fatalf("args(%+v): %v", c.req, err)
		}
		if got := strings.Join(args, " "); got != c.want {
			t.Errorf("args(%+v) = %q, want %q", c.req, got, c.want)
		}
	}
	if _, err := (AstGrepRequest{}).args(); err == nil {
		t.Error("empty pattern must error")
	}
	if _, err := (AstGrepRequest{Pattern: "  "}).args(); err == nil {
		t.Error("blank pattern must error")
	}
}

func TestAstGrepArtifactKeyDeterministicAndBounded(t *testing.T) {
	req := AstGrepRequest{Pattern: "fmt.Println($A)", Lang: "go", Paths: []string{"src"}}
	key := req.ArtifactKey()
	if key != req.ArtifactKey() {
		t.Errorf("ArtifactKey not deterministic: %q vs %q", key, req.ArtifactKey())
	}
	if !strings.HasPrefix(key, "ast-grep:run:go:") || len(key) > 40 {
		t.Errorf("ArtifactKey = %q, want bounded ast-grep:run:go:<digest12>", key)
	}
	if (AstGrepRequest{Pattern: "x"}).ArtifactKey() == key {
		t.Error("different requests must produce different keys")
	}
	if !strings.HasPrefix((AstGrepRequest{Pattern: "x"}).ArtifactKey(), "ast-grep:run:auto:") {
		t.Error("empty lang must key as auto")
	}
}

func TestAstGrepRunCapturesEvidence(t *testing.T) {
	g := NewAstGrep()
	record := installFakeAstGrep(t, g, `[{"file":"main.go"}]`, "", 0)
	snapDir := t.TempDir()

	run, err := g.Run(context.Background(), snapDir, AstGrepRequest{Pattern: "func main() { $$$ }", Lang: "go"})
	if err != nil {
		t.Fatal(err)
	}
	if run.Backend != BackendAstGrep {
		t.Errorf("backend = %q, want %q", run.Backend, BackendAstGrep)
	}
	if run.ExitCode != 0 || run.Stdout != `[{"file":"main.go"}]` || !run.Matched() {
		t.Errorf("run = %+v", run)
	}
	if run.Command() != "ast-grep run --pattern func main() { $$$ } --lang go --json=compact ." {
		t.Errorf("Command() = %q", run.Command())
	}
	rec, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(rec)), "\n")
	if len(got) < 7 || got[0] != "run" || got[1] != "--pattern" || got[2] != "func main() { $$$ }" {
		t.Errorf("shim argv = %v", got)
	}
	wantDir, _ := filepath.EvalSymlinks(snapDir)
	gotDir, _ := filepath.EvalSymlinks(got[len(got)-1])
	if gotDir != wantDir {
		t.Errorf("shim cwd = %q, want snapshot dir %q", gotDir, wantDir)
	}
}

// TestAstGrepZeroMatchesIsAFindingNotAFailure pins the grep-parity exit
// semantics verified live on ast-grep 0.44.1: exit 1 with clean stderr means
// "ran fine, zero matches" and must not surface as an error.
func TestAstGrepZeroMatchesIsAFindingNotAFailure(t *testing.T) {
	g := NewAstGrep()
	installFakeAstGrep(t, g, "[]", "", 1)
	run, err := g.Run(context.Background(), t.TempDir(), AstGrepRequest{Pattern: "nope($X)"})
	if err != nil {
		t.Fatalf("zero-match run must not error: %v", err)
	}
	if run.ExitCode != 1 || run.Matched() {
		t.Errorf("run = %+v, want ExitCode 1 and Matched() false", run)
	}
	if run.Stdout != "[]" {
		t.Errorf("stdout = %q, want empty JSON array", run.Stdout)
	}
}

func TestAstGrepRealFailureIsEvidence(t *testing.T) {
	g := NewAstGrep()
	installFakeAstGrep(t, g, "", "language go not supported", 2)
	run, err := g.Run(context.Background(), t.TempDir(), AstGrepRequest{Pattern: "x", Lang: "go"})
	if err == nil {
		t.Fatal("exit 2 must return an error")
	}
	if run.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2", run.ExitCode)
	}
	if !strings.Contains(run.Stderr, "language go not supported") {
		t.Errorf("stderr not captured: %q", run.Stderr)
	}
	if !strings.Contains(err.Error(), "ast-grep run --pattern x --lang go --json=compact . exited 2") {
		t.Errorf("error lacks recomputable command: %v", err)
	}

	// Exit 1 WITH stderr is a real failure too, not a zero-match result.
	installFakeAstGrep(t, g, "", "cannot parse pattern", 1)
	if _, err := g.Run(context.Background(), t.TempDir(), AstGrepRequest{Pattern: "("}); err == nil {
		t.Error("exit 1 with stderr must stay an error")
	}
}

func TestAstGrepNotInstalledFallsBackGracefully(t *testing.T) {
	g := NewAstGrep()
	g.lookPath = func(string) (string, error) { return "", errors.New("not found") }

	if _, err := g.Discover(); !errors.Is(err, ErrAstGrepNotInstalled) {
		t.Fatalf("Discover error = %v, want ErrAstGrepNotInstalled", err)
	}
	run, err := g.Run(context.Background(), t.TempDir(), AstGrepRequest{Pattern: "x"})
	if !errors.Is(err, ErrAstGrepNotInstalled) {
		t.Fatalf("Run error = %v, want ErrAstGrepNotInstalled", err)
	}
	if run.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1 (never ran)", run.ExitCode)
	}
	for _, want := range []string{"brew install ast-grep", "codemap --importers", "fall back", "remote Tier-1"} {
		if !strings.Contains(AstGrepInstallHint, want) {
			t.Errorf("install hint missing %q", want)
		}
	}
}

func TestServiceRunAstGrepRecordsArtifactHash(t *testing.T) {
	url, _, _, _ := initFixtureRepo(t)
	svc := newTestService(t)
	stdout := `[{"file":"src/lib.go"}]`
	installFakeAstGrep(t, svc.AstGrep, stdout, "", 0)

	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url})
	if err != nil {
		t.Fatal(err)
	}
	req := AstGrepRequest{Pattern: "func Lib() $$$", Lang: "go"}
	run, err := svc.RunAstGrep(context.Background(), snap, req)
	if err != nil {
		t.Fatal(err)
	}
	if run.Stdout != stdout {
		t.Errorf("stdout = %q", run.Stdout)
	}
	meta, err := LoadMetadata(snap.Dir)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(stdout))
	if got := meta.ToolArtifactHashes[req.ArtifactKey()]; got != hex.EncodeToString(sum[:]) {
		t.Errorf("artifact hash = %q, want sha256 of stdout under %q", got, req.ArtifactKey())
	}
}
