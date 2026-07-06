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

	if err := CheckACPHandshake(context.Background(), agent, "", nil, time.Second); err != nil {
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
