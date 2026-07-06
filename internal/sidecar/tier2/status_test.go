package tier2

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// fakeAdapters returns codemap/ast-grep adapters whose lookPath resolves via
// found: binary name -> resolved path. Absent names return an error, exactly
// like exec.LookPath on a missing binary.
func fakeAdapters(found map[string]string) (*Codemap, *AstGrep) {
	look := func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", errors.New("executable file not found in $PATH")
	}
	cm := NewCodemap()
	cm.lookPath = look
	ag := NewAstGrep()
	ag.lookPath = look
	return cm, ag
}

func TestBackendStatusesAllExternalsFound(t *testing.T) {
	cm, ag := fakeAdapters(map[string]string{
		"codemap":  "/fake/bin/codemap",
		"ast-grep": "/fake/bin/ast-grep",
	})
	statuses := backendStatuses(cm, ag)

	if len(statuses) != 3 {
		t.Fatalf("expected 3 backend statuses, got %d", len(statuses))
	}
	// Canonical order: embedded first, then externals.
	wantOrder := []string{BackendRepomap, BackendCodemap, BackendAstGrep}
	for i, want := range wantOrder {
		if statuses[i].Backend != want {
			t.Fatalf("statuses[%d].Backend = %q, want %q", i, statuses[i].Backend, want)
		}
	}

	repomap := statuses[0]
	if repomap.Kind != KindEmbedded || !repomap.Available {
		t.Fatalf("repomap must be embedded and always available, got %+v", repomap)
	}
	if !strings.Contains(repomap.Detail, "built into ghx") {
		t.Fatalf("repomap Detail should state it is built into ghx, got %q", repomap.Detail)
	}
	if repomap.InstallHint != "" {
		t.Fatalf("embedded backend must not carry an install hint, got %q", repomap.InstallHint)
	}

	for _, st := range statuses[1:] {
		if st.Kind != KindExternal {
			t.Fatalf("%s Kind = %q, want external", st.Backend, st.Kind)
		}
		if !st.Available {
			t.Fatalf("%s should be available with a resolvable binary: %+v", st.Backend, st)
		}
		if !strings.HasPrefix(st.Detail, "/fake/bin/") {
			t.Fatalf("%s Detail should be the resolved path, got %q", st.Backend, st.Detail)
		}
		if st.InstallHint != "" {
			t.Fatalf("%s available backend must not carry an install hint, got %q", st.Backend, st.InstallHint)
		}
	}
}

func TestBackendStatusesMissingExternalsCarryInstallHints(t *testing.T) {
	cm, ag := fakeAdapters(nil) // nothing on PATH
	statuses := backendStatuses(cm, ag)

	if !statuses[0].Available {
		t.Fatalf("repomap must stay available even with an empty PATH: %+v", statuses[0])
	}

	wantHints := map[string]string{
		BackendCodemap: CodemapInstallHint,
		BackendAstGrep: AstGrepInstallHint,
	}
	for _, st := range statuses[1:] {
		if st.Available {
			t.Fatalf("%s should be unavailable when the binary is absent: %+v", st.Backend, st)
		}
		if !strings.Contains(st.Detail, "not found on PATH") {
			t.Fatalf("%s Detail should say the binary is missing, got %q", st.Backend, st.Detail)
		}
		if st.InstallHint != wantHints[st.Backend] {
			t.Fatalf("%s InstallHint = %q, want the canonical install hint", st.Backend, st.InstallHint)
		}
	}
}

// TestBackendStatusesProductionDiscovery exercises the exported entry point
// against a controlled PATH: one external present, one absent — hermetic, no
// dependency on the machine's real tool installs.
func TestBackendStatusesProductionDiscovery(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("PATH fixture uses unix executable bits")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "ast-grep")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	byBackend := map[string]BackendStatus{}
	for _, st := range BackendStatuses() {
		byBackend[st.Backend] = st
	}
	if st := byBackend[BackendAstGrep]; !st.Available || st.Detail != fake {
		t.Fatalf("ast-grep should resolve to %q, got %+v", fake, st)
	}
	if st := byBackend[BackendCodemap]; st.Available || st.InstallHint == "" {
		t.Fatalf("codemap should be missing with an install hint, got %+v", st)
	}
	if st := byBackend[BackendRepomap]; !st.Available {
		t.Fatalf("repomap must be available regardless of PATH, got %+v", st)
	}
}
