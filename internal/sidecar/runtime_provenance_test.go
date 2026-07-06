package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

// captureOpts installs a stub turn runner that records every RunTurnOptions it
// receives and returns the given result/err, optionally writing sideEffect to
// the turn's AgentStderrPath first (simulating the adapter's stderr).
func captureOpts(t *testing.T, result TurnResult, err error, sideEffect string) *[]RunTurnOptions {
	t.Helper()
	old := runTurnWithOptions
	t.Cleanup(func() { runTurnWithOptions = old })
	var calls []RunTurnOptions
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls = append(calls, opts)
		if sideEffect != "" && opts.AgentStderrPath != "" {
			_ = os.WriteFile(opts.AgentStderrPath, []byte(sideEffect), 0o644)
		}
		return result, "sess-1", err
	}
	return &calls
}

func okReport() TurnResult {
	return TurnResult{FullText: `<ghx-report>{"answer":"ok"}</ghx-report>`}
}

// ADR-0033 D1: with no override and no persisted cwd, the ACP session cwd
// defaults to the neutral ghx-owned session directory, the per-session stderr
// log path is passed to the turn, and provenance is persisted on the creating
// turn (ADR-0033 D5).
func TestAskDefaultsCwdToSessionWorkspace(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	calls := captureOpts(t, okReport(), nil, "")

	if _, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) == 0 {
		t.Fatal("no turn ran")
	}
	opts := (*calls)[0]
	wantCwd := SessionWorkspace(dir, "s")
	if opts.Cwd != wantCwd {
		t.Errorf("cwd = %q, want the neutral session workspace %q", opts.Cwd, wantCwd)
	}
	if opts.AgentStderrPath != AgentStderrLogPath(dir, "s") {
		t.Errorf("AgentStderrPath = %q, want %q", opts.AgentStderrPath, AgentStderrLogPath(dir, "s"))
	}
	meta, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if meta.Cwd != wantCwd {
		t.Errorf("persisted cwd = %q, want %q", meta.Cwd, wantCwd)
	}
	if meta.AgentCmd != "mock" {
		t.Errorf("persisted agentCmd = %q, want mock", meta.AgentCmd)
	}
	if meta.SpawnCwd == "" {
		t.Error("spawnCwd not recorded")
	}
}

// ADR-0033 D1: an explicit Config.Cwd override (evals) wins over the neutral
// default and is what the turn and the persisted provenance record.
func TestAskRespectsCwdOverride(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	override := t.TempDir()
	calls := captureOpts(t, okReport(), nil, "")

	if _, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock", Cwd: override}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	}); err != nil {
		t.Fatal(err)
	}
	if got := (*calls)[0].Cwd; got != override {
		t.Errorf("cwd = %q, want the explicit override %q", got, override)
	}
	meta, _ := ReadMeta(dir, "s")
	if meta.Cwd != override {
		t.Errorf("persisted cwd = %q, want the override %q", meta.Cwd, override)
	}
}

// ADR-0033 D1: on resume, a persisted SessionMeta.Cwd is reused so turns are
// deterministic regardless of where the follow-up ask is invoked.
func TestAskReusesPersistedCwdOnResume(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	if err := InitSession(dir, "s", "o/r", "scope", SessionNamedExplicit); err != nil {
		t.Fatal(err)
	}
	meta, _ := ReadMeta(dir, "s")
	meta.Cwd = "/persisted/workspace"
	meta.TurnCount = 1
	if err := SaveMeta(dir, *meta); err != nil {
		t.Fatal(err)
	}
	calls := captureOpts(t, okReport(), nil, "")

	if _, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "follow up",
	}); err != nil {
		t.Fatal(err)
	}
	if got := (*calls)[0].Cwd; got != "/persisted/workspace" {
		t.Errorf("cwd = %q, want the persisted %q reused on resume", got, "/persisted/workspace")
	}
}

// ADR-0033 D3: an unrecovered turn failure surfaces the adapter stderr tail,
// the log path, and the trust hint in the error the CLI caller receives, while
// preserving the errors.Is chain for the ADR-0027 classifiers.
func TestAskFailureSurfacesAgentStderr(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	trustLine := "Ignoring 40 permissions.allow entries: this workspace has not been trusted"
	captureOpts(t, TurnResult{FullText: "partial"},
		fmt.Errorf("acp prompt: %w", ErrLivenessTimeout), trustLine)

	_, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrLivenessTimeout) {
		t.Errorf("errors.Is(ErrLivenessTimeout) broken by diagnosis:\n%v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "has not been trusted") {
		t.Errorf("error missing adapter stderr tail:\n%s", msg)
	}
	if !strings.Contains(msg, AgentStderrLogPath(dir, "s")) {
		t.Errorf("error missing stderr log path:\n%s", msg)
	}
	if !strings.Contains(msg, "Hint:") {
		t.Errorf("error missing the trust hint:\n%s", msg)
	}
}

// ADR-0033 D3: the silent no-report case (empty turn, nil error — the founder's
// exact shape) becomes a loud report that names the stderr log and carries the
// trust remedy, instead of a bland WARN.
func TestAskSilentTurnBecomesLoudReport(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	trustLine := "this workspace has not been trusted"
	captureOpts(t, TurnResult{FullText: ""}, nil, trustLine)

	report, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatalf("a completed-but-empty turn must not hard-error: %v", err)
	}
	if !strings.HasPrefix(report.Answer, "WARN: sidecar did not emit") {
		t.Errorf("answer lost the WARN shape:\n%s", report.Answer)
	}
	if !strings.Contains(report.Answer, "has not been trusted") {
		t.Errorf("silent failure did not surface the adapter stderr:\n%s", report.Answer)
	}
	if !strings.Contains(report.Answer, "Hint:") {
		t.Errorf("silent trust failure missing the actionable hint:\n%s", report.Answer)
	}
}
