package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPlainProfileEnvDropsOnlyPathDirsProvidingGhx(t *testing.T) {
	keepA := t.TempDir()
	keepB := t.TempDir()
	drop := t.TempDir()
	for _, name := range []string{"ghx", "ghx.exe"} {
		if err := os.WriteFile(filepath.Join(drop, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	env, err := buildPlainProfileEnv([]string{
		"HOME=/Users/tester",
		"GH_TOKEN=secret",
		"PATH=" + strings.Join([]string{keepA, drop, keepB}, string(os.PathListSeparator)),
	}, filepath.Join(t.TempDir(), "unused-bin"))
	if err != nil {
		t.Fatal(err)
	}

	gotPath := filepath.SplitList(envValue(env, "PATH"))
	wantPath := []string{keepA, keepB}
	if strings.Join(gotPath, "\n") != strings.Join(wantPath, "\n") {
		t.Fatalf("PATH dirs = %q, want %q", gotPath, wantPath)
	}
	if got := envValue(env, "HOME"); got != "/Users/tester" {
		t.Fatalf("HOME = %q, want inherited value", got)
	}
	if got := envValue(env, "GH_TOKEN"); got != "secret" {
		t.Fatalf("GH_TOKEN = %q, want inherited value", got)
	}
}

func TestWrapperHashOnlyForFilePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if hash, ok := wrapperHash(path); !ok || hash == "" {
		t.Fatalf("wrapperHash(%q) = %q,%v; want hash", path, hash, ok)
	}
	if hash, ok := wrapperHash("agent"); ok || hash != "" {
		t.Fatalf("wrapperHash(non-path) = %q,%v; want empty,false", hash, ok)
	}
}

func TestResolveSubjectModelFromProcessEnv(t *testing.T) {
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "env-sonnet")
	if got := resolveSubjectModel("missing"); got != "env-sonnet" {
		t.Fatalf("subject model = %q, want env-sonnet", got)
	}
}

func TestResolveSubjectModelFromWrapperDefault(t *testing.T) {
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.sh")
	if err := os.WriteFile(path, []byte(`#!/bin/sh
export ANTHROPIC_MODEL="${ANTHROPIC_MODEL:-claude-sonnet-5}"
export GHX_EVAL_SUBJECT_MODEL="${GHX_EVAL_SUBJECT_MODEL:-$ANTHROPIC_MODEL}"
exec npx -y @agentclientprotocol/claude-agent-acp@0.55.0 "$@"
`), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveSubjectModel(path); got != "claude-sonnet-5" {
		t.Fatalf("subject model = %q, want wrapper default", got)
	}
}

func TestResolveSubjectModelUnknown(t *testing.T) {
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "")
	if got := resolveSubjectModel(filepath.Join(t.TempDir(), "missing-agent")); got != "unknown" {
		t.Fatalf("subject model = %q, want unknown", got)
	}
}
