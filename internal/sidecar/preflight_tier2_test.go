package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCheckTier2ToolsMissingExternalsStillPasses pins the doctor contract for
// a machine with no Tier-2 external binaries installed (the out-of-the-box
// npm/brew install): the check PASSES — codemap and ast-grep are optional —
// but says exactly what is missing and how to install it, and states that
// local:repomap needs no install at all.
func TestCheckTier2ToolsMissingExternalsStillPasses(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty PATH: no codemap, no ast-grep

	check := checkTier2Tools(context.Background())
	if check.Name != "tier2-tools" {
		t.Fatalf("check name = %q, want tier2-tools", check.Name)
	}
	if !check.Passed {
		t.Fatalf("missing optional externals must not fail preflight: %+v", check)
	}
	for _, want := range []string{
		"local:repomap built into ghx",
		"local:codemap missing (optional)",
		"local:ast-grep missing (optional)",
		"brew tap JordanCoin/tap && brew install codemap", // codemap remediation
		"brew install ast-grep",                           // ast-grep remediation
	} {
		if !strings.Contains(check.Message, want) {
			t.Fatalf("message missing %q:\n%s", want, check.Message)
		}
	}
}

// TestCheckTier2ToolsReportsResolvedPaths pins the found shape: an installed
// external is reported with its resolved path and no remediation noise.
func TestCheckTier2ToolsReportsResolvedPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture uses unix executable bits")
	}
	dir := t.TempDir()
	for _, name := range []string{"codemap", "ast-grep"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)

	check := checkTier2Tools(context.Background())
	if !check.Passed {
		t.Fatalf("all tools present must pass: %+v", check)
	}
	for _, want := range []string{
		"local:codemap found (" + filepath.Join(dir, "codemap") + ")",
		"local:ast-grep found (" + filepath.Join(dir, "ast-grep") + ")",
	} {
		if !strings.Contains(check.Message, want) {
			t.Fatalf("message missing %q:\n%s", want, check.Message)
		}
	}
	if strings.Contains(check.Message, "Install it with") {
		t.Fatalf("no remediation should print when everything resolves:\n%s", check.Message)
	}
}
