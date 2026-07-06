package sidecar_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// ADR-0033 D1/D2 at the real ACP boundary: RunTurnWithOptions forwards the
// session cwd to the adapter's session/new and tees the adapter's stderr to the
// per-session log file.
func TestRunTurnCapturesStderrAndForwardsCwd(t *testing.T) {
	bin := buildScriptedAgent(t, []map[string]any{{"text": "ok"}})
	promptLog := filepath.Join(t.TempDir(), "prompts.log")
	t.Setenv("MOCKAGENT_PROMPT_LOG", promptLog)
	t.Setenv("MOCKAGENT_STDERR", "this workspace has not been trusted (injected)")

	workspace := t.TempDir()
	stderrLog := filepath.Join(t.TempDir(), "agent-stderr.log")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := sidecar.RunTurnWithOptions(ctx, sidecar.RunTurnOptions{
		AgentCmd:        bin,
		Prompt:          "q",
		Cwd:             workspace,
		AgentStderrPath: stderrLog,
		LivenessTimeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("turn: %v", err)
	}

	// The adapter received our neutral cwd on session/new.
	logData, _ := os.ReadFile(promptLog)
	if !strings.Contains(string(logData), "NEW_CWD "+workspace) {
		t.Errorf("adapter did not receive the session cwd; prompt log:\n%s", logData)
	}
	// The adapter's stderr was captured to the per-session log.
	tail := sidecar.AgentStderrTail(stderrLog)
	if !strings.Contains(tail, "this workspace has not been trusted") {
		t.Errorf("adapter stderr not captured to the session log; tail:\n%s", tail)
	}
	if !sidecar.IsWorkspaceTrustWarning(tail) {
		t.Error("captured stderr should classify as the trust warning")
	}
}

// ADR-0033 D4: CheckLiveTurn completes a real prompt turn through a healthy
// agent and reports the reply.
func TestCheckLiveTurnPasses(t *testing.T) {
	bin := buildScriptedAgent(t, []map[string]any{{"text": "ok"}})
	check := sidecar.CheckLiveTurn(context.Background(), sidecar.Config{AgentCmd: bin})
	if !check.Passed {
		t.Fatalf("live turn should pass against a healthy agent: %s", check.Message)
	}
	if check.Name != "live-turn" {
		t.Errorf("check name = %q, want live-turn", check.Name)
	}
}

// ADR-0033 D4: against an agent that starts but never speaks ACP, CheckLiveTurn
// fails with the failing stage and the agent's stderr tail — the deterministic
// diagnosis the acp-handshake check (initialize only) cannot give.
func TestCheckLiveTurnReportsStageAndStderr(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "broken-agent")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho broken-agent-diagnostic >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	check := sidecar.CheckLiveTurn(context.Background(), sidecar.Config{AgentCmd: script})
	if check.Passed {
		t.Fatal("live turn must fail against a non-ACP agent")
	}
	if !strings.Contains(check.Message, "acp initialize") {
		t.Errorf("failing stage not reported:\n%s", check.Message)
	}
	if !strings.Contains(check.Message, "broken-agent-diagnostic") {
		t.Errorf("agent stderr tail not surfaced:\n%s", check.Message)
	}
	if check.Remediation == "" {
		t.Error("a failing live-turn check must carry remediation")
	}
}
