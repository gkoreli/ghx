package sidecar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigFileExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	path, exists := ConfigFileExists()
	if exists {
		t.Fatalf("ConfigFileExists = true in fresh GHX_HOME (%s)", path)
	}
	if want := filepath.Join(home, "config.json"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}

	if err := SaveConfig(NewDefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if _, exists := ConfigFileExists(); !exists {
		t.Fatal("ConfigFileExists = false after SaveConfig")
	}
}

func TestDiffConfigsReportsChangedFieldsOnly(t *testing.T) {
	oldCfg := Config{AgentCmd: "claude", SessionsDir: "/x/sessions"}
	newCfg := Config{AgentCmd: ClaudeACPAgentCmd, SessionsDir: "/x/sessions"}

	diff := DiffConfigs(oldCfg, newCfg)
	if len(diff) != 1 {
		t.Fatalf("diff = %v, want exactly one changed field", diff)
	}
	if !strings.Contains(diff[0], "agent:") ||
		!strings.Contains(diff[0], `"claude"`) ||
		!strings.Contains(diff[0], ClaudeACPAgentCmd) {
		t.Fatalf("diff line = %q, want agent old -> new", diff[0])
	}

	if diff := DiffConfigs(newCfg, newCfg); len(diff) != 0 {
		t.Fatalf("identical configs: diff = %v, want empty", diff)
	}
}

// TestClaudeACPAgentCmdMatchesEvalWrapperPin guards the "bump both together"
// contract: the adapter pin written by `config init --claude-acp` must be the
// same one the eval wrapper scripts/eval-agent-acp.sh execs, so production
// setup and eval runs exercise an identical agent command.
func TestClaudeACPAgentCmdMatchesEvalWrapperPin(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "eval-agent-acp.sh"))
	if err != nil {
		t.Fatal(err)
	}
	pin := strings.TrimPrefix(ClaudeACPAgentCmd, "npx -y ")
	if pin == ClaudeACPAgentCmd || !strings.Contains(pin, "@agentclientprotocol/claude-agent-acp@") {
		t.Fatalf("ClaudeACPAgentCmd = %q, want an `npx -y <pkg>@<version>` command line", ClaudeACPAgentCmd)
	}
	if !strings.Contains(string(data), pin) {
		t.Fatalf("scripts/eval-agent-acp.sh does not exec the pinned adapter %q — bump both together", pin)
	}
}

func TestSplitAgentCmd(t *testing.T) {
	name, args := splitAgentCmd("claude")
	if name != "claude" || len(args) != 0 {
		t.Fatalf("bare name: got %q %v", name, args)
	}
	name, args = splitAgentCmd(ClaudeACPAgentCmd)
	if name != "npx" || strings.Join(args, " ") != "-y @agentclientprotocol/claude-agent-acp@0.55.0" {
		t.Fatalf("command line: got %q %v", name, args)
	}
}
