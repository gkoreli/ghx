package sidecar_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// ADR-0027 D2 unit coverage at the RunTurnWithOptions boundary, against the
// scripted mockagent (a real ACP peer over stdio, no LLM): the wait is
// cancellable, the liveness watchdog fires on silence and stays quiet under
// activity, a dead peer fails the turn immediately, and a failed prompt still
// returns the established ACP session ID for the D1 wrap-up resume.

// buildScriptedAgent compiles the evals mockagent and wires its script/state
// env for this test.
func buildScriptedAgent(t *testing.T, replies []map[string]any) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "mockagent")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/gkoreli/ghx/v2/internal/sidecar/evals/mockagent")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mockagent: %v\n%s", err, out)
	}
	data, err := json.Marshal(replies)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "script.json")
	if err := os.WriteFile(script, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCKAGENT_SCRIPT", script)
	t.Setenv("MOCKAGENT_STATE", filepath.Join(dir, "state"))
	return bin
}

func TestRunTurnLivenessWatchdogCancelsSilentTurn(t *testing.T) {
	bin := buildScriptedAgent(t, []map[string]any{
		{"hangMs": 30000, "text": "never reached"},
	})

	start := time.Now()
	_, _, err := sidecar.RunTurnWithOptions(context.Background(), sidecar.RunTurnOptions{
		AgentCmd:        bin,
		Prompt:          "q",
		LivenessTimeout: 300 * time.Millisecond,
	})
	if !errors.Is(err, sidecar.ErrLivenessTimeout) {
		t.Fatalf("err = %v, want ErrLivenessTimeout", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("watchdog cancellation took %s — the wait was not cancellable", elapsed)
	}
}

func TestRunTurnWatchdogStaysQuietUnderActivity(t *testing.T) {
	// Total prompt time (10 × 300ms) exceeds the 1.5s window, but every chunk
	// resets the liveness clock: activity, not total duration, is the signal.
	// The interval-to-window margin is wide (5×) so scheduler jitter under
	// -race load cannot fire the watchdog spuriously.
	bin := buildScriptedAgent(t, []map[string]any{
		{"updateCount": 10, "updateIntervalMs": 300, "text": "done after steady updates"},
	})

	result, _, err := sidecar.RunTurnWithOptions(context.Background(), sidecar.RunTurnOptions{
		AgentCmd:        bin,
		Prompt:          "q",
		LivenessTimeout: 1500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("steady activity must keep the watchdog quiet: %v", err)
	}
	if result.FullText == "" {
		t.Fatal("turn output lost")
	}
}

func TestRunTurnDeadPeerFailsImmediately(t *testing.T) {
	bin := buildScriptedAgent(t, []map[string]any{
		{"exitBeforeResponse": true},
	})

	start := time.Now()
	_, _, err := sidecar.RunTurnWithOptions(context.Background(), sidecar.RunTurnOptions{
		AgentCmd: bin,
		Prompt:   "q",
		// Long watchdog window: the dead peer must fail the turn well before
		// any timer (ADR-0027 D2).
		LivenessTimeout: time.Minute,
	})
	if err == nil {
		t.Fatal("dead peer must fail the turn")
	}
	if !sidecar.IsPeerClosedError(err) {
		t.Fatalf("err = %v, want a peer-closed classification", err)
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("dead peer took %s to fail — must be immediate, not timer-driven", elapsed)
	}
}

func TestRunTurnReturnsSessionIDOnPromptError(t *testing.T) {
	bin := buildScriptedAgent(t, []map[string]any{
		{"promptError": "Reached maximum number of turns (24)"},
	})

	_, sessionID, err := sidecar.RunTurnWithOptions(context.Background(), sidecar.RunTurnOptions{
		AgentCmd:        bin,
		Prompt:          "q",
		LivenessTimeout: time.Minute,
	})
	if !sidecar.IsMaxTurnsError(err) {
		t.Fatalf("err = %v, want the max-turns classification", err)
	}
	if sessionID != "mock-sess-1" {
		t.Fatalf("sessionID = %q, want the established session returned on failure (D1 resume depends on it)", sessionID)
	}
}

func TestAskRecreatesStaleACPSessionWithMockAgent(t *testing.T) {
	bin := buildScriptedAgent(t, []map[string]any{
		{"text": `<ghx-report>{"answer":"fresh session succeeded"}</ghx-report>`},
	})
	t.Setenv("MOCKAGENT_REJECT_UNKNOWN_LOAD", "1")

	dir := t.TempDir()
	promptLog := filepath.Join(dir, "prompts.log")
	t.Setenv("MOCKAGENT_PROMPT_LOG", promptLog)

	if err := sidecar.InitSession(dir, "s", "o/r", "scope"); err != nil {
		t.Fatal(err)
	}
	meta, err := sidecar.ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	meta.ACPSessionID = "stale-session"
	if err := sidecar.SaveMeta(dir, *meta); err != nil {
		t.Fatal(err)
	}

	report, result, err := sidecar.Ask(context.Background(), sidecar.Config{SessionsDir: dir, AgentCmd: bin}, sidecar.AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatalf("Ask must recreate a stale ACP session instead of failing: %v", err)
	}
	if report.Answer != "fresh session succeeded" {
		t.Fatalf("answer = %q, want fresh session succeeded", report.Answer)
	}
	if !result.SessionRecreated {
		t.Fatal("SessionRecreated not recorded")
	}

	updated, err := sidecar.ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ACPSessionID != "mock-sess-1" {
		t.Fatalf("persisted ACP session = %q, want mock-sess-1", updated.ACPSessionID)
	}

	logData, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "LOAD stale-session") || !strings.Contains(string(logData), "PROMPT ") {
		t.Fatalf("mockagent did not exercise stale LoadSession then fresh prompt:\n%s", logData)
	}

	sessionLog, err := os.ReadFile(filepath.Join(dir, "s", "logs.jsonl"))
	if err != nil {
		t.Fatalf("read logs.jsonl: %v", err)
	}
	if !strings.Contains(string(sessionLog), "sidecar.session.recreated") ||
		!strings.Contains(string(sessionLog), "acp_load_session_resource_not_found") {
		t.Fatalf("logs.jsonl missing session recreation downgrade:\n%s", sessionLog)
	}
}
