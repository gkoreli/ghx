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
