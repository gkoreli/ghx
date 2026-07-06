package sidecar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFakeAgent(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func fakeACPAgentScript() string {
	return `#!/bin/sh
IFS= read -r line
printf '%s\n' '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":1,"agentCapabilities":{"loadSession":false},"authMethods":[]}}'
`
}

func TestCheckACPHandshakeAcceptsInitializeResponse(t *testing.T) {
	dir := t.TempDir()
	agent := writeFakeAgent(t, dir, "fake-acp", fakeACPAgentScript())

	// Generous timeout: the fake agent responds instantly, but parallel
	// builds on a loaded machine have pushed sh startup past 1s (observed
	// flake 2026-07-05).
	if err := CheckACPHandshake(context.Background(), agent, "", nil, 10*time.Second); err != nil {
		t.Fatalf("CheckACPHandshake returned error: %v", err)
	}
}

func TestCheckACPHandshakeTimesOutWithActionableMessage(t *testing.T) {
	dir := t.TempDir()
	agent := writeFakeAgent(t, dir, "silent-agent", "#!/bin/sh\nsleep 5\n")

	err := CheckACPHandshake(context.Background(), agent, "", nil, 50*time.Millisecond)
	if err == nil {
		t.Fatal("CheckACPHandshake succeeded, want timeout")
	}
	if !strings.Contains(err.Error(), "did not complete the ACP handshake") ||
		!strings.Contains(err.Error(), "ghx sidecar config init") {
		t.Fatalf("error = %q, want actionable handshake message", err.Error())
	}
}

func TestPreflightAgentConfigOverrideWins(t *testing.T) {
	if got := preflightAgentConfig("custom-agent").AgentCmd; got != "custom-agent" {
		t.Fatalf("AgentCmd = %q, want custom-agent", got)
	}
	if got := preflightAgentConfig("").AgentCmd; got != LoadConfig().AgentCmd {
		t.Fatalf("empty override changed AgentCmd to %q", got)
	}
}

func TestDetectAgentsRequiresACPHandshake(t *testing.T) {
	dir := t.TempDir()
	writeFakeAgent(t, dir, "ok-agent", fakeACPAgentScript())
	writeFakeAgent(t, dir, "path-only-agent", "#!/bin/sh\nsleep 5\n")
	t.Setenv("PATH", dir)

	oldKnown := knownAgents
	oldGrace := shutdownGrace
	knownAgents = []struct{ name string }{
		{name: "path-only-agent"},
		{name: "ok-agent"},
	}
	shutdownGrace = 10 * time.Millisecond
	defer func() {
		knownAgents = oldKnown
		shutdownGrace = oldGrace
	}()

	oldHandshake := checkACPHandshake
	checkACPHandshake = func(ctx context.Context, agentCmd, cwd string, env []string, timeout time.Duration) error {
		if agentCmd == "path-only-agent" {
			timeout = 50 * time.Millisecond
		}
		return CheckACPHandshake(ctx, agentCmd, cwd, env, timeout)
	}
	defer func() { checkACPHandshake = oldHandshake }()

	found := DetectAgents(context.Background())
	if len(found) != 1 || found[0] != "ok-agent" {
		t.Fatalf("DetectAgents = %v, want only ok-agent", found)
	}
}

func TestCheckACPHandshakeSplitsAgentCommandLine(t *testing.T) {
	dir := t.TempDir()
	agent := writeFakeAgent(t, dir, "fake-acp", fakeACPAgentScript())

	// A multi-word command line (the shape `config init --claude-acp` writes)
	// must be split into argv, not treated as one binary path.
	if err := CheckACPHandshake(context.Background(), "/bin/sh "+agent, "", nil, 10*time.Second); err != nil {
		t.Fatalf("CheckACPHandshake with command line returned error: %v", err)
	}
}

func writeFakeSinkExe(t *testing.T, body string) string {
	t.Helper()
	return writeFakeAgent(t, t.TempDir(), "fake-ghx", body)
}

func TestCheckReportSinkVersionMatch(t *testing.T) {
	exe := writeFakeSinkExe(t, "#!/bin/sh\necho 'ghx 9.9.9'\n")
	t.Setenv("GHX_REPORT_SINK_EXE", exe)

	c := checkReportSinkVersion(context.Background(), "9.9.9")
	if !c.Passed {
		t.Fatalf("check failed: %+v", c)
	}
	if !strings.Contains(c.Message, exe) || !strings.Contains(c.Message, "GHX_REPORT_SINK_EXE") {
		t.Fatalf("message = %q, want sink exe path and source", c.Message)
	}
}

func TestCheckReportSinkVersionMismatchFailsWithRemediation(t *testing.T) {
	exe := writeFakeSinkExe(t, "#!/bin/sh\necho 'ghx 2.1.13'\n")
	t.Setenv("GHX_REPORT_SINK_EXE", exe)

	c := checkReportSinkVersion(context.Background(), "2.2.0")
	if c.Passed {
		t.Fatalf("check passed with mismatched versions: %+v", c)
	}
	if !strings.Contains(c.Message, "2.1.13") || !strings.Contains(c.Message, "2.2.0") {
		t.Fatalf("message = %q, want both versions named", c.Message)
	}
	if !strings.Contains(c.Remediation, "GHX_REPORT_SINK_EXE") {
		t.Fatalf("remediation = %q, want concrete fix text", c.Remediation)
	}

	// The failure must surface loudly in the doctor output, not just in a field.
	out := FormatPreflight(PreflightResult{Passed: false, Checks: []PreflightCheck{c}})
	if !strings.Contains(out, "✗ report-sink-version") || !strings.Contains(out, "→") {
		t.Fatalf("FormatPreflight output missing loud failure + remediation:\n%s", out)
	}
}

func TestCheckReportSinkVersionCommandFailure(t *testing.T) {
	// A stale released binary without a working `version` (or a dead path)
	// must fail the check, not silently degrade later at ask time.
	exe := writeFakeSinkExe(t, "#!/bin/sh\nexit 1\n")
	t.Setenv("GHX_REPORT_SINK_EXE", exe)

	c := checkReportSinkVersion(context.Background(), "2.2.0")
	if c.Passed {
		t.Fatalf("check passed for failing version command: %+v", c)
	}
	if c.Remediation == "" {
		t.Fatal("want remediation text on version-command failure")
	}
}

func TestCheckReportSinkVersionUnparseableOutput(t *testing.T) {
	exe := writeFakeSinkExe(t, "#!/bin/sh\necho 'not ghx output'\n")
	t.Setenv("GHX_REPORT_SINK_EXE", exe)

	if c := checkReportSinkVersion(context.Background(), "2.2.0"); c.Passed {
		t.Fatalf("check passed for unparseable version output: %+v", c)
	}
}

func TestCheckReportSinkVersionUnknownRunningVersionOnlyProbes(t *testing.T) {
	// Under go test (eval preflight) the running version is unknown; the check
	// still verifies the sink exe resolves and answers `version`, but skips
	// the equality comparison.
	exe := writeFakeSinkExe(t, "#!/bin/sh\necho 'ghx 2.1.13'\n")
	t.Setenv("GHX_REPORT_SINK_EXE", exe)

	if c := checkReportSinkVersion(context.Background(), ""); !c.Passed {
		t.Fatalf("check failed with unknown running version: %+v", c)
	}
}

func TestCheckReportSinkVersionUnresolvableUnderGoTest(t *testing.T) {
	// Without GHX_REPORT_SINK_EXE the resolver sees the go-test binary and
	// refuses — the loud failure eval runs need instead of a silent fallback.
	t.Setenv("GHX_REPORT_SINK_EXE", "")

	c := checkReportSinkVersion(context.Background(), "")
	if c.Passed {
		t.Fatalf("check passed under go test without GHX_REPORT_SINK_EXE: %+v", c)
	}
	if !strings.Contains(c.Message, "go test") {
		t.Fatalf("message = %q, want go-test resolution failure", c.Message)
	}
}
