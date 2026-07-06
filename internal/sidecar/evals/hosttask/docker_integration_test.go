package hosttask

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestDockerIntegrationTrivialFixture is the one non-hermetic test in the
// package: it provisions a local file:// fixture repo with real git and
// grades it with real docker against a tiny public image. Gated because it
// needs a running docker daemon (and network to pull alpine once).
//
//	GHX_HOSTTASK_DOCKER=1 go test ./internal/sidecar/evals/hosttask -run TestDockerIntegration -v
func TestDockerIntegrationTrivialFixture(t *testing.T) {
	if os.Getenv("GHX_HOSTTASK_DOCKER") != "1" {
		t.Skip("docker integration disabled; set GHX_HOSTTASK_DOCKER=1 to run")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := (DockerCLI{}).Available(ctx); err != nil {
		t.Fatalf("GHX_HOSTTASK_DOCKER=1 but docker is unavailable: %v", err)
	}

	url, _, oldSHA := initWorkspaceFixtureRepo(t)
	f := validFixture()
	f.PinnedSHA = oldSHA
	f.Image = "alpine:3.20"
	f.SetupCmds = []string{"echo setup-ok"}
	// hello.txt exists at the pin; later.txt does not (fail-to-pass stays
	// failing — this trial models an agent that did not do the work).
	f.FailToPassCmds = []string{"test -f later.txt"}
	f.PassToPassCmds = []string{"grep -q hello hello.txt"}

	p := NewProvisioner(t.TempDir())
	p.RemoteURL = url
	ws, err := p.Provision(ctx, f, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Remove()

	res, err := NewGrader().Grade(ctx, f, ws.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Flaky {
		t.Fatalf("trivial fixture graded flaky: %+v", res)
	}
	// F2P fails (no fix applied), P2P passes: grade = 0 × 1 = 0, and the
	// deterministic commands must be stable across the flake re-runs.
	if res.F2PFraction != 0 || res.P2PPreservation != 1 || res.Grade != 0 {
		t.Errorf("grade = %v (f2p %v, p2p %v), want 0 (0, 1)", res.Grade, res.F2PFraction, res.P2PPreservation)
	}
	if len(res.Attempts) != 3 {
		t.Errorf("attempts = %d, want 3 (failure triggers re-runs)", len(res.Attempts))
	}

	// Now "apply the fix" in the workspace and re-grade: everything passes
	// on the first attempt.
	writeWorkspaceFile(t, ws.Dir, "later.txt", "the fix\n")
	res, err = NewGrader().Grade(ctx, f, ws.Dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Flaky || res.Grade != 1 || len(res.Attempts) != 1 {
		t.Errorf("post-fix grade = %+v, want clean 1.0 in one attempt", res)
	}
}
