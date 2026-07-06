package tier2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installFakeCodemap writes an executable shim named codemap that records its
// argv and cwd, prints canned stdout, and exits with the given code. Returns
// the record file path and points the adapter's lookPath at the shim.
func installFakeCodemap(t *testing.T, c *Codemap, stdout string, exitCode int) (recordFile string) {
	t.Helper()
	dir := t.TempDir()
	recordFile = filepath.Join(dir, "argv.txt")
	shim := filepath.Join(dir, "codemap")
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > " + recordFile + "\n" +
		"pwd >> " + recordFile + "\n" +
		"printf '%s' '" + stdout + "'\n"
	if exitCode != 0 {
		script += "echo 'boom: ast-grep missing' >&2\n"
	}
	script += "exit " + string(rune('0'+exitCode)) + "\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	c.lookPath = func(name string) (string, error) {
		if name != "codemap" {
			return "", errors.New("unexpected lookup: " + name)
		}
		return shim, nil
	}
	return recordFile
}

func TestCodemapRequestArgs(t *testing.T) {
	cases := []struct {
		req  CodemapRequest
		want string
	}{
		{CodemapRequest{Mode: CodemapOverview}, "--json ."},
		{CodemapRequest{Mode: CodemapContext}, "context"},
		{CodemapRequest{Mode: CodemapContext, Compact: true}, "context --compact"},
		{CodemapRequest{Mode: CodemapImporters, File: "src/lib.go"}, "--importers src/lib.go"},
		{CodemapRequest{Mode: CodemapDeps}, "--deps ."},
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
	if _, err := (CodemapRequest{Mode: CodemapImporters}).args(); err == nil {
		t.Error("importers without file must error")
	}
	if _, err := (CodemapRequest{Mode: "nope"}).args(); err == nil {
		t.Error("unknown mode must error")
	}
}

func TestCodemapRunCapturesEvidence(t *testing.T) {
	c := NewCodemap()
	record := installFakeCodemap(t, c, `{"files":3}`, 0)
	snapDir := t.TempDir()

	run, err := c.Run(context.Background(), snapDir, CodemapRequest{Mode: CodemapOverview})
	if err != nil {
		t.Fatal(err)
	}
	if run.Backend != BackendCodemap {
		t.Errorf("backend = %q, want %q", run.Backend, BackendCodemap)
	}
	if run.ExitCode != 0 || run.Stdout != `{"files":3}` {
		t.Errorf("run = %+v", run)
	}
	if run.Command() != "codemap --json ." {
		t.Errorf("Command() = %q", run.Command())
	}
	rec, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(rec)), "\n")
	if len(got) < 3 || got[0] != "--json" || got[1] != "." {
		t.Errorf("shim argv = %v, want [--json .] + cwd", got)
	}
	// The shim must run inside the snapshot dir (resolve symlinks: macOS
	// tempdirs live under /private).
	wantDir, _ := filepath.EvalSymlinks(snapDir)
	gotDir, _ := filepath.EvalSymlinks(got[len(got)-1])
	if gotDir != wantDir {
		t.Errorf("shim cwd = %q, want snapshot dir %q", gotDir, wantDir)
	}
}

func TestCodemapRunFailureIsEvidence(t *testing.T) {
	c := NewCodemap()
	installFakeCodemap(t, c, "", 2)
	run, err := c.Run(context.Background(), t.TempDir(), CodemapRequest{Mode: CodemapDeps})
	if err == nil {
		t.Fatal("non-zero exit must return an error")
	}
	if run.ExitCode != 2 {
		t.Errorf("exit code = %d, want 2", run.ExitCode)
	}
	if !strings.Contains(run.Stderr, "ast-grep missing") {
		t.Errorf("stderr not captured: %q", run.Stderr)
	}
	if !strings.Contains(err.Error(), "codemap --deps . exited 2") {
		t.Errorf("error lacks recomputable command: %v", err)
	}
}

func TestCodemapNotInstalledFallsBackGracefully(t *testing.T) {
	c := NewCodemap()
	c.lookPath = func(string) (string, error) { return "", errors.New("not found") }

	if _, err := c.Discover(); !errors.Is(err, ErrCodemapNotInstalled) {
		t.Fatalf("Discover error = %v, want ErrCodemapNotInstalled", err)
	}
	run, err := c.Run(context.Background(), t.TempDir(), CodemapRequest{Mode: CodemapOverview})
	if !errors.Is(err, ErrCodemapNotInstalled) {
		t.Fatalf("Run error = %v, want ErrCodemapNotInstalled", err)
	}
	if run.ExitCode != -1 {
		t.Errorf("exit code = %d, want -1 (never ran)", run.ExitCode)
	}
	for _, want := range []string{"brew tap JordanCoin/tap", "fall back", "remote Tier-1"} {
		if !strings.Contains(CodemapInstallHint, want) {
			t.Errorf("install hint missing %q", want)
		}
	}
}

func TestServiceRunCodemapRecordsArtifactHash(t *testing.T) {
	url, _, _, _ := initFixtureRepo(t)
	svc := newTestService(t)
	stdout := `{"hubs":["main.go"]}`
	installFakeCodemap(t, svc.Codemap, stdout, 0)

	snap, err := svc.Snapshot(context.Background(), SnapshotRequest{Repo: "fixture/repo", RemoteURL: url})
	if err != nil {
		t.Fatal(err)
	}
	run, err := svc.RunCodemap(context.Background(), snap, CodemapRequest{Mode: CodemapOverview})
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
	if got := meta.ToolArtifactHashes["codemap:overview"]; got != hex.EncodeToString(sum[:]) {
		t.Errorf("artifact hash = %q, want sha256 of stdout", got)
	}
}

func TestCodemapArtifactKey(t *testing.T) {
	cases := map[string]CodemapRequest{
		"codemap:overview":         {Mode: CodemapOverview},
		"codemap:context":          {Mode: CodemapContext},
		"codemap:context:compact":  {Mode: CodemapContext, Compact: true},
		"codemap:importers:a/b.go": {Mode: CodemapImporters, File: "a/b.go"},
		"codemap:deps":             {Mode: CodemapDeps},
	}
	for want, req := range cases {
		if got := req.ArtifactKey(); got != want {
			t.Errorf("ArtifactKey(%+v) = %q, want %q", req, got, want)
		}
	}
}
