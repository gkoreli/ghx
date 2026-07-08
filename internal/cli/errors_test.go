package cli

import (
	"errors"
	"strings"
	"testing"
)

// TestCoreErrorPreservesExitCodeAndAppendsHint pins the full CLI affordance
// contract now that the class/hint are sourced from core (ADR-0034 phase 2):
// a plain upstream error is classified once through the shared core classifier,
// the exit code is 3, the original text is preserved verbatim, and the fix-it
// line is appended on its own line behind the "→" marker for a predictable parse
// point. Byte-for-byte the text internal/ghx.Search wraps around a go-gh 401.
func TestCoreErrorPreservesExitCodeAndAppendsHint(t *testing.T) {
	raw := errors.New("GraphQL query failed: HTTP 401: Bad credentials")
	err := coreError(raw)
	if got := CodeForError(err); got != ExitUpstreamFailure {
		t.Fatalf("exit code = %d, want %d", got, ExitUpstreamFailure)
	}
	msg := err.Error()
	if !strings.Contains(msg, raw.Error()) {
		t.Fatalf("wrapped error dropped raw text: %q", msg)
	}
	if !strings.Contains(msg, affordanceMarker+"GitHub authentication failed") {
		t.Fatalf("wrapped error missing marked hint line: %q", msg)
	}
	// The hint must be on its own trailing line.
	lines := strings.Split(msg, "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, affordanceMarker) {
		t.Fatalf("hint not on final line: %q", last)
	}
}

// TestCoreErrorUnmatchedPassesThrough confirms an unclassified upstream error is
// still exit 3 but carries no extra line (a wrong recovery is worse than none).
func TestCoreErrorUnmatchedPassesThrough(t *testing.T) {
	raw := errors.New("novel backend explosion")
	err := coreError(raw)
	if got := CodeForError(err); got != ExitUpstreamFailure {
		t.Fatalf("exit code = %d, want %d", got, ExitUpstreamFailure)
	}
	if err.Error() != raw.Error() {
		t.Fatalf("unmatched error mutated: %q", err.Error())
	}
}

// TestWithAffordanceEmptyHintIsNoop confirms an empty hint only sets the exit
// code without appending a blank marker line.
func TestWithAffordanceEmptyHintIsNoop(t *testing.T) {
	raw := errors.New("boom")
	err := withAffordance(ExitBadInvocation, raw, "  ")
	if err.Error() != "boom" {
		t.Fatalf("empty hint mutated error: %q", err.Error())
	}
	if got := CodeForError(err); got != ExitBadInvocation {
		t.Fatalf("exit code = %d, want %d", got, ExitBadInvocation)
	}
	if withAffordance(ExitUpstreamFailure, nil, "x") != nil {
		t.Fatal("nil error must stay nil")
	}
}
