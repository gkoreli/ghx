package cli

import "testing"

// TestCodeTranspileErrorExitsBadInvocation pins that a code-mode snippet that
// fails to transpile/parse exits 2 (ExitBadInvocation), not 0 — a scripting
// agent checking $? must never read a non-executed script as success (dogfood
// friction, FRICTION.md 2026-07-07 "code-mode transpile error returns exit 0").
func TestCodeTranspileErrorExitsBadInvocation(t *testing.T) {
	// Top-level await fails transpilation before any tool call runs (no network).
	err := codeCmd.RunE(codeCmd, []string{`const r = await codemode.explore({repo:"x/y"}); return r;`})
	if err == nil {
		t.Fatal("transpile-failing snippet returned nil error (would exit 0)")
	}
	if got := CodeForError(err); got != ExitBadInvocation {
		t.Fatalf("exit code = %d, want %d (ExitBadInvocation)", got, ExitBadInvocation)
	}
}
