package cli

import (
	"errors"
	"strings"
	"testing"
)

// TestUpstreamAffordanceClassifies pins the fix-it hint chosen for each observed
// upstream failure class. These are the walls agents actually hit (auth, rate
// limit, 404, forbidden, network); the hint must name a runnable recovery.
func TestUpstreamAffordanceClassifies(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantSub string // substring the recovery command/hint must contain
	}{
		{"rate limit", "GraphQL query failed: API rate limit exceeded", "gh api rate_limit"},
		{"secondary rate", "search failed: You have exceeded a secondary rate limit", "gh api rate_limit"},
		{"auth 401", "GraphQL query failed: HTTP 401: Bad credentials", "gh auth login"},
		{"requires auth", "search failed: This endpoint requires you to be authenticated", "gh auth login"},
		{"not found 404", "GraphQL query failed: HTTP 404: Not Found", "ghx explore owner/repo"},
		{"repo not resolvable", "Could not resolve to a Repository with the name 'x/y'", "ghx repos"},
		{"forbidden 403", "search failed: HTTP 403: Forbidden", "gh auth status"},
		{"network", "failed to create REST client: dial tcp: connection refused", "check connectivity"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := upstreamAffordance(errors.New(tc.raw))
			if got == "" {
				t.Fatalf("no affordance for %q; want one containing %q", tc.raw, tc.wantSub)
			}
			if !strings.Contains(strings.ToLower(got), strings.ToLower(tc.wantSub)) {
				t.Fatalf("affordance for %q = %q, missing %q", tc.raw, got, tc.wantSub)
			}
		})
	}
}

// TestUpstreamAffordanceUnmatched confirms an unrecognized failure surfaces its
// raw text with no invented hint — a wrong recovery is worse than none.
func TestUpstreamAffordanceUnmatched(t *testing.T) {
	if got := upstreamAffordance(errors.New("some novel backend explosion")); got != "" {
		t.Fatalf("unmatched error got affordance %q, want empty", got)
	}
	if got := upstreamAffordance(nil); got != "" {
		t.Fatalf("nil error got affordance %q, want empty", got)
	}
}

// TestUpstreamErrorPreservesExitCodeAndAppendsHint pins the full contract: exit
// code stays 3, the original text is preserved verbatim, and the fix-it line is
// appended on its own line behind the "→" marker for a predictable parse point.
func TestUpstreamErrorPreservesExitCodeAndAppendsHint(t *testing.T) {
	raw := errors.New("GraphQL query failed: HTTP 401: Bad credentials")
	err := upstreamError(raw)
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

// TestUpstreamErrorUnmatchedPassesThrough confirms an unclassified upstream
// error is still exit 3 but carries no extra line.
func TestUpstreamErrorUnmatchedPassesThrough(t *testing.T) {
	raw := errors.New("novel backend explosion")
	err := upstreamError(raw)
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
