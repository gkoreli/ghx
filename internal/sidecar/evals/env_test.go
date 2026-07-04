package evals

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBuildPlainProfileEnvExcludesGhx(t *testing.T) {
	src := t.TempDir()
	for _, name := range append(requiredPlainTools, "ghx") {
		path := filepath.Join(src, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bin := filepath.Join(t.TempDir(), "bin")
	env, err := buildPlainProfileEnv([]string{"PATH=" + src}, bin)
	if err != nil {
		t.Fatal(err)
	}
	pathValue := envValue(env, "PATH")
	if pathValue != bin {
		t.Fatalf("PATH = %q, want scrubbed bin %q", pathValue, bin)
	}
	for _, name := range requiredPlainTools {
		if _, err := os.Lstat(filepath.Join(bin, name)); err != nil && name != "bash" {
			t.Fatalf("required tool %s not linked: %v", name, err)
		}
	}
	cmd := exec.Command("sh", "-c", "command -v ghx")
	cmd.Env = env
	if err := cmd.Run(); err == nil {
		t.Fatal("ghx resolved in plain-profile environment")
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
