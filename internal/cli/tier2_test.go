package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

func TestTier2CommandShape(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"tier2", "codemap"})
	if err != nil {
		t.Fatalf("Find tier2 codemap: %v", err)
	}
	if cmd != tier2CodemapCmd {
		t.Fatalf("RootCmd tier2 codemap = %v, want tier2CodemapCmd", cmd)
	}
	if tier2CodemapCmd.Use != "codemap <owner/repo>" {
		t.Fatalf("Use = %q", tier2CodemapCmd.Use)
	}
	// The ADR-0024.1 smoke repos should appear as examples so evidence
	// citations stay recomputable.
	if !strings.Contains(tier2CodemapCmd.Example, "ghx tier2 codemap honojs/hono") {
		t.Fatalf("tier2 codemap examples missing hono smoke: %q", tier2CodemapCmd.Example)
	}
	for _, name := range []string{"ref", "sparse", "context", "compact", "importers", "deps"} {
		if tier2CodemapCmd.Flag(name) == nil {
			t.Fatalf("tier2 codemap flag %q not registered", name)
		}
	}
}

// TestTier2CodemapMissingBinaryFallsBackBeforeClone proves the graceful
// fallback ordering: with no codemap on PATH the command fails with the
// typed not-installed error and the upstream exit code, before any network
// or clone work (an empty PATH would break git too if it were reached).
func TestTier2CodemapMissingBinaryFallsBackBeforeClone(t *testing.T) {
	t.Setenv("GHX_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // no codemap, no git

	RootCmd.SetArgs([]string{"tier2", "codemap", "gin-gonic/gin"})
	defer RootCmd.SetArgs(nil)
	err := RootCmd.Execute()
	if err == nil {
		t.Fatal("expected codemap-not-installed error")
	}
	if !errors.Is(err, tier2.ErrCodemapNotInstalled) {
		t.Fatalf("error = %v, want ErrCodemapNotInstalled", err)
	}
	if got := CodeForError(err); got != ExitUpstreamFailure {
		t.Fatalf("exit code = %d, want %d", got, ExitUpstreamFailure)
	}
}
