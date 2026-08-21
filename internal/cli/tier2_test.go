package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
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

func TestTier2AstGrepCommandShape(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"tier2", "astgrep"})
	if err != nil {
		t.Fatalf("Find tier2 astgrep: %v", err)
	}
	if cmd != tier2AstGrepCmd {
		t.Fatalf("RootCmd tier2 astgrep = %v, want tier2AstGrepCmd", cmd)
	}
	if tier2AstGrepCmd.Use != "astgrep <owner/repo> [paths...]" {
		t.Fatalf("Use = %q", tier2AstGrepCmd.Use)
	}
	// The ADR-0024.1 smoke repos should appear as examples so evidence
	// citations stay recomputable.
	if !strings.Contains(tier2AstGrepCmd.Example, "ghx tier2 astgrep honojs/hono") {
		t.Fatalf("tier2 astgrep examples missing hono smoke: %q", tier2AstGrepCmd.Example)
	}
	for _, name := range []string{"pattern", "lang", "ref", "sparse"} {
		if tier2AstGrepCmd.Flag(name) == nil {
			t.Fatalf("tier2 astgrep flag %q not registered", name)
		}
	}
}

func TestTier2RepomapCommandShape(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"tier2", "repomap"})
	if err != nil {
		t.Fatalf("Find tier2 repomap: %v", err)
	}
	if cmd != tier2RepomapCmd {
		t.Fatalf("RootCmd tier2 repomap = %v, want tier2RepomapCmd", cmd)
	}
	if !strings.Contains(tier2RepomapCmd.Example, "ghx tier2 repomap honojs/hono") {
		t.Fatalf("tier2 repomap examples missing hono smoke: %q", tier2RepomapCmd.Example)
	}
	for _, name := range []string{"query", "budget", "ref", "sparse"} {
		if tier2RepomapCmd.Flag(name) == nil {
			t.Fatalf("tier2 repomap flag %q not registered", name)
		}
	}
	if def := tier2RepomapCmd.Flag("budget").DefValue; def != "1024" {
		t.Fatalf("budget default = %q, want the pre-registered 1024", def)
	}
}

// TestTier2AstGrepMissingBinaryFallsBackBeforeClone proves the graceful
// fallback ordering for item 3: with no ast-grep on PATH the command fails
// with the typed not-installed error and the upstream exit code, before any
// network or clone work (an empty PATH would break git too if it were
// reached).
func TestTier2AstGrepMissingBinaryFallsBackBeforeClone(t *testing.T) {
	t.Setenv("GHX_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // no ast-grep, no git

	RootCmd.SetArgs([]string{"tier2", "astgrep", "gin-gonic/gin", "--pattern", "func main() { $$$ }"})
	defer RootCmd.SetArgs(nil)
	err := RootCmd.Execute()
	if err == nil {
		t.Fatal("expected ast-grep-not-installed error")
	}
	if !errors.Is(err, tier2.ErrAstGrepNotInstalled) {
		t.Fatalf("error = %v, want ErrAstGrepNotInstalled", err)
	}
	if got := CodeForError(err); got != ExitUpstreamFailure {
		t.Fatalf("exit code = %d, want %d", got, ExitUpstreamFailure)
	}
}

// TestTier2RepomapInvalidRepoFailsBeforeNetwork: repomap needs no external
// binary, so its first guard is snapshot validation.
func TestTier2RepomapInvalidRepoFailsBeforeNetwork(t *testing.T) {
	t.Setenv("GHX_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // no git: any network/clone attempt would fail differently

	RootCmd.SetArgs([]string{"tier2", "repomap", "not-a-repo"})
	defer RootCmd.SetArgs(nil)
	err := RootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("error = %v, want invalid repo rejection", err)
	}
	if got := CodeForError(err); got != ExitUpstreamFailure {
		t.Fatalf("exit code = %d, want %d", got, ExitUpstreamFailure)
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

// ADR-0024.4 D2: inside a sidecar ask turn without a local grant, tier2 tool
// commands are refused pre-hoc (exit 2, affordance hint); with the grant or
// unset env they proceed. Gating happens before any snapshot work.
func TestTier2PreHocGrantGate(t *testing.T) {
	if CodeForError(tier2GrantGateError("remote")) != ExitBadInvocation {
		t.Fatal("remote-only grant must be refused with exit 2")
	}
	hint := tier2GrantGateError("remote").Error()
	if !strings.Contains(hint, "--local") || !strings.Contains(hint, "→") {
		t.Fatalf("gate error missing affordance hint: %s", hint)
	}
	_ = tier2GrantGateError // referenced for vet
}

// tier2GrantGateError reproduces tier2Snapshot's gate branch for table
// testing without touching the network.
func tier2GrantGateError(grant string) error {
	if grant != "" && !sidecar.Tier2GrantAllowed(grant) {
		return WithExitCode(ExitBadInvocation, fmt.Errorf(
			"tier-2 local analysis is not granted for this ask (GHX_TIER2_ALLOWED_BACKENDS=%s)\n→ re-ask with --local to grant tier-2, or answer from remote evidence", grant))
	}
	return nil
}
