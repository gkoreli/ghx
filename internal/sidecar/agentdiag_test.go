package sidecar

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ADR-0033 D5: PresentAgentEnv records the NAMES of agent-relevant env vars
// present, never their values, and never a name that is unset.
func TestPresentAgentEnvNamesOnly(t *testing.T) {
	environ := []string{
		"ANTHROPIC_API_KEY=sk-secret-value-do-not-leak",
		"CLAUDE_CODE_USE_BEDROCK=1",
		"PATH=/usr/bin",
		"HOME=/home/x",
		"UNRELATED_VAR=whatever",
		"EMPTY_VAR=",
	}
	got := PresentAgentEnv(environ)
	want := map[string]bool{"ANTHROPIC_API_KEY": true, "CLAUDE_CODE_USE_BEDROCK": true, "PATH": true, "HOME": true}
	for _, name := range got {
		if !want[name] {
			t.Errorf("unexpected env name recorded: %q", name)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Errorf("missing expected env names: %v", want)
	}
	// Truthfulness: no secret value may appear anywhere in the fingerprint.
	joined := strings.Join(got, "\x00")
	if strings.Contains(joined, "sk-secret-value-do-not-leak") {
		t.Fatal("env value leaked into the names-only fingerprint")
	}
	if strings.Contains(joined, "UNRELATED_VAR") {
		t.Error("non-allowlisted var recorded")
	}
}

// ADR-0033.1: CaptureAgentAuthEnv forwards only allowlisted, non-empty
// NAME=VALUE entries — no PATH/HOME/GH_TOKEN, no unrelated shell vars.
func TestCaptureAgentAuthEnv(t *testing.T) {
	environ := []string{
		"CLAUDE_CODE_USE_BEDROCK=1",
		"AWS_REGION=us-west-2",
		"AWS_SECRET_ACCESS_KEY=super-secret",
		"ANTHROPIC_API_KEY=",   // empty value: not forwarded
		"PATH=/usr/bin",        // process context: never forwarded
		"HOME=/home/x",         // process context: never forwarded
		"GH_TOKEN=ghp_example", // ghx's own access: never forwarded
		"UNRELATED_VAR=whatever",
	}
	got := CaptureAgentAuthEnv(environ)
	want := []string{
		"CLAUDE_CODE_USE_BEDROCK=1",
		"AWS_REGION=us-west-2",
		"AWS_SECRET_ACCESS_KEY=super-secret",
	}
	if len(got) != len(want) {
		t.Fatalf("captured = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("captured[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ADR-0033.1: MergeAgentEnv overlay wins per name; base order preserved; new
// names appended; empty overlay keeps exec's inherit-everything nil.
func TestMergeAgentEnv(t *testing.T) {
	if got := MergeAgentEnv([]string{"A=1"}, nil); got != nil {
		t.Fatalf("empty overlay should return nil (inherit), got %v", got)
	}
	base := []string{"PATH=/usr/bin", "AWS_REGION=eu-west-1", "TERM=xterm"}
	overlay := []string{"AWS_REGION=us-west-2", "CLAUDE_CODE_USE_BEDROCK=1"}
	got := MergeAgentEnv(base, overlay)
	want := []string{"PATH=/usr/bin", "AWS_REGION=us-west-2", "TERM=xterm", "CLAUDE_CODE_USE_BEDROCK=1"}
	if len(got) != len(want) {
		t.Fatalf("merged = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("merged[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ADR-0033.1: the warm-worker respawn trigger — equal envs share a digest,
// any change (or a switch to/from inherit) changes it.
func TestAgentEnvDigest(t *testing.T) {
	if AgentEnvDigest(nil) != "" {
		t.Fatal("nil env (inherit) must digest to empty")
	}
	a := AgentEnvDigest([]string{"A=1", "B=2"})
	if a == "" || a != AgentEnvDigest([]string{"A=1", "B=2"}) {
		t.Fatal("equal envs must share a non-empty digest")
	}
	if a == AgentEnvDigest([]string{"A=1", "B=3"}) {
		t.Fatal("changed value must change the digest")
	}
	if strings.Contains(a, "A=1") {
		t.Fatal("digest must not contain env content")
	}
}

func TestIsWorkspaceTrustWarning(t *testing.T) {
	trust := "Ignoring 40 permissions.allow entries from .claude/settings.local.json: this workspace has not been trusted."
	if !IsWorkspaceTrustWarning(trust) {
		t.Error("did not detect the documented trust warning")
	}
	if IsWorkspaceTrustWarning("some unrelated adapter log line") {
		t.Error("false positive on unrelated stderr")
	}
}

func TestAgentStderrTail(t *testing.T) {
	if got := AgentStderrTail(""); got != "" {
		t.Errorf("empty path tail = %q, want empty", got)
	}
	if got := AgentStderrTail(filepath.Join(t.TempDir(), "missing.log")); got != "" {
		t.Errorf("missing file tail = %q, want empty", got)
	}
	path := filepath.Join(t.TempDir(), "agent-stderr.log")
	// Larger than the tail window so we exercise the last-N-bytes read.
	body := strings.Repeat("noise line\n", 1000) + "FINAL DIAGNOSTIC LINE"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	tail := AgentStderrTail(path)
	if !strings.HasSuffix(tail, "FINAL DIAGNOSTIC LINE") {
		t.Errorf("tail did not capture the final line:\n%s", tail)
	}
	if len(tail) > agentStderrTailBytes {
		t.Errorf("tail = %d bytes, want <= %d", len(tail), agentStderrTailBytes)
	}
}

// DiagnoseTurnError must preserve the errors.Is chain (ADR-0027 classifiers)
// while splicing in the stderr tail and, for the trust marker, the hint.
func TestDiagnoseTurnError(t *testing.T) {
	base := errors.New("acp prompt: boom")

	// No stderr → error returned unchanged (no spurious appendix).
	if got := DiagnoseTurnError(base, ""); got != base {
		t.Errorf("with no stderr, got %v, want the base error unchanged", got)
	}

	path := filepath.Join(t.TempDir(), "agent-stderr.log")
	if err := os.WriteFile(path, []byte("this workspace has not been trusted\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("sentinel cause")
	wrapped := DiagnoseTurnError(sentinelWrap(sentinel), path)
	if !errors.Is(wrapped, sentinel) {
		t.Fatal("DiagnoseTurnError broke the errors.Is chain")
	}
	msg := wrapped.Error()
	if !strings.Contains(msg, path) {
		t.Error("diagnosed error does not reference the stderr log path")
	}
	if !strings.Contains(msg, "has not been trusted") {
		t.Error("diagnosed error dropped the stderr tail")
	}
	if !strings.Contains(msg, "Hint:") || !strings.Contains(msg, "hasTrustDialogAccepted") {
		t.Error("trust marker present but the actionable hint was not appended")
	}
}

func sentinelWrap(err error) error { return &wrapErr{err} }

type wrapErr struct{ e error }

func (w *wrapErr) Error() string { return "acp prompt: " + w.e.Error() }
func (w *wrapErr) Unwrap() error { return w.e }

// warnNoReportAnswer turns the silent no-report case into a loud, diagnosable
// answer (ADR-0033 D3): it keeps the WARN prefix, always points at the log, and
// carries the trust hint when the marker is present.
func TestWarnNoReportAnswer(t *testing.T) {
	if got := warnNoReportAnswer(""); !strings.HasPrefix(got, "WARN: sidecar did not emit") {
		t.Errorf("empty path answer = %q, want WARN prefix", got)
	}

	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.log")
	if got := warnNoReportAnswer(empty); !strings.Contains(got, empty) {
		t.Errorf("answer must reference the stderr log path even when empty:\n%s", got)
	}

	trust := filepath.Join(dir, "trust.log")
	if err := os.WriteFile(trust, []byte("Ignoring entries: this workspace has not been trusted"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := warnNoReportAnswer(trust)
	if !strings.HasPrefix(got, "WARN: sidecar did not emit") {
		t.Errorf("answer lost the WARN prefix:\n%s", got)
	}
	if !strings.Contains(got, "has not been trusted") || !strings.Contains(got, "Hint:") {
		t.Errorf("trust warning not surfaced in the loud answer:\n%s", got)
	}
}

// TestDiagnoseTurnErrorAuthHint pins the auth failure class from the founder's
// work laptop (2026-07-06): the adapter's -32000 "Authentication required"
// arrives in the PROMPT ERROR (stderr carries only SDK cleanup noise), so the
// hint must fire from the error text alone, even with no stderr log at all.
func TestDiagnoseTurnErrorAuthHint(t *testing.T) {
	base := errors.New(`run turn: acp prompt: {"code":-32000,"message":"Authentication required"}`)
	err := DiagnoseTurnError(base, "")
	if !strings.Contains(err.Error(), "auth login --claudeai") {
		t.Fatalf("auth hint missing from diagnosed error:\n%s", err.Error())
	}
	if !errors.Is(err, base) {
		t.Fatal("diagnosed error lost the wrapped base error identity")
	}
}

// TestMatchAgentHintsDedupAndOrder: one hint per class even when the marker
// appears in both the error and stderr; registration order preserved.
func TestMatchAgentHints(t *testing.T) {
	hints := matchAgentHints("Authentication required", "Authentication required and has not been trusted")
	if len(hints) != 2 {
		t.Fatalf("got %d hints, want 2 (trust + auth, deduplicated)", len(hints))
	}
	if hints[0] != workspaceTrustHint || hints[1] != agentAuthHint {
		t.Fatal("hint registration order not preserved")
	}
	if got := matchAgentHints("clean stderr", ""); got != nil {
		t.Fatalf("no-marker case returned hints: %v", got)
	}
}

// TestAgentStderrTailFiltersBenignNoise pins the founder-reported failure
// shape (2026-07-06): repeated CLAUDE_SDK_CAN_USE_TOOL_SHADOWED blocks from
// every spawned process buried the one meaningful auth line. The surfaced
// tail must drop the boilerplate and keep the signal; an all-benign log
// surfaces as empty (so no misleading "stderr evidence" block is rendered).
func TestAgentStderrTailFiltersBenignNoise(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-stderr.log")
	noisy := "(node:73016) [CLAUDE_SDK_CAN_USE_TOOL_SHADOWED] Warning: canUseTool will not be invoked for: mcp__ghx-report-sink__submit_report.\n" +
		"(Use `node --trace-warnings ...` to show where the warning was created)\n" +
		"Unexpected case: post_turn_summary\n" +
		"Session abc: query stream error: ACP connection closed\n"
	if err := os.WriteFile(path, []byte(noisy), 0o644); err != nil {
		t.Fatal(err)
	}
	tail := AgentStderrTail(path)
	if strings.Contains(tail, "CLAUDE_SDK_CAN_USE_TOOL_SHADOWED") || strings.Contains(tail, "trace-warnings") || strings.Contains(tail, "post_turn_summary") {
		t.Fatalf("benign boilerplate not filtered:\n%s", tail)
	}
	if !strings.Contains(tail, "query stream error") {
		t.Fatalf("meaningful line lost:\n%s", tail)
	}

	if err := os.WriteFile(path, []byte("(node:1) [CLAUDE_SDK_CAN_USE_TOOL_SHADOWED] Warning: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := AgentStderrTail(path); got != "" {
		t.Fatalf("all-benign log must surface empty, got %q", got)
	}
}
