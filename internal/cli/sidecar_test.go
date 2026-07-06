package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func readConfigAgent(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(data)
}

func TestConfigInitClaudeACPWritesFreshConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := runSidecarConfigInit(true, false); err != nil {
		t.Fatalf("fresh init --claude-acp failed: %v", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != sidecar.ClaudeACPAgentCmd {
		t.Fatalf("agent = %q, want %q", got, sidecar.ClaudeACPAgentCmd)
	}
	if !strings.Contains(readConfigAgent(t, home), "@agentclientprotocol/claude-agent-acp") {
		t.Fatal("config.json does not contain the adapter command")
	}
}

func TestConfigInitClaudeACPRefusesExistingConfigWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := sidecar.SaveConfig(sidecar.Config{AgentCmd: "claude", SessionsDir: filepath.Join(home, "sessions")}); err != nil {
		t.Fatal(err)
	}
	err := runSidecarConfigInit(true, false)
	if err == nil {
		t.Fatal("init --claude-acp overwrote an existing config without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %q, want --force hint", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != "claude" {
		t.Fatalf("agent = %q after refusal, want unchanged %q", got, "claude")
	}
}

func TestConfigInitClaudeACPForceOverwritesAndKeepsModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := sidecar.SaveConfig(sidecar.Config{
		AgentCmd:    "claude",
		SessionsDir: filepath.Join(home, "sessions"),
		Model:       "claude-sonnet-4-5",
	}); err != nil {
		t.Fatal(err)
	}
	if err := runSidecarConfigInit(true, true); err != nil {
		t.Fatalf("init --claude-acp --force failed: %v", err)
	}
	cfg := sidecar.LoadConfig()
	if cfg.AgentCmd != sidecar.ClaudeACPAgentCmd {
		t.Fatalf("agent = %q, want %q", cfg.AgentCmd, sidecar.ClaudeACPAgentCmd)
	}
	if cfg.Model != "claude-sonnet-4-5" {
		t.Fatalf("model = %q, want carried over", cfg.Model)
	}
}

func TestConfigInitClaudeACPIdempotentWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := runSidecarConfigInit(true, false); err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	// Re-running on a config that already matches changes nothing and needs no --force.
	if err := runSidecarConfigInit(true, false); err != nil {
		t.Fatalf("re-run on matching config failed: %v", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != sidecar.ClaudeACPAgentCmd {
		t.Fatalf("agent = %q, want %q", got, sidecar.ClaudeACPAgentCmd)
	}
}
