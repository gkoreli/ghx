package ghx

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

// TestClassifyUpstreamSignatures pins the class and fix-it hint chosen for each
// observed upstream failure signature. These are the walls agents actually hit
// (auth, rate limit, 404, forbidden, network); every one classifies as
// ClassUpstream and the sub-classification must name a runnable recovery. Ported
// from the CLI's former substring table so the affordance contract is unchanged
// now that classification lives in core (ADR-0034).
func TestClassifyUpstreamSignatures(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantSub string // substring the recovery hint must contain
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
			class, hint := ClassifyUpstream(errors.New(tc.raw))
			if class != ClassUpstream {
				t.Fatalf("class for %q = %v, want %v", tc.raw, class, ClassUpstream)
			}
			if hint == "" {
				t.Fatalf("no hint for %q; want one containing %q", tc.raw, tc.wantSub)
			}
			if !strings.Contains(strings.ToLower(hint), strings.ToLower(tc.wantSub)) {
				t.Fatalf("hint for %q = %q, missing %q", tc.raw, hint, tc.wantSub)
			}
		})
	}
}

// TestClassifyUpstreamStructured proves classification reads structured signals —
// the HTTP status on an *api.HTTPError and the error type on an *api.GraphQLError —
// not just re-parsed English. This is the ADR-0034 win: the class is derived from
// the response the process actually received, even wrapped through fmt.Errorf.
func TestClassifyUpstreamStructured(t *testing.T) {
	u, _ := url.Parse("https://api.github.com/search/code")
	cases := []struct {
		name    string
		err     error
		wantSub string
	}{
		{"http 401", &api.HTTPError{StatusCode: 401, Message: "Bad credentials", RequestURL: u}, "gh auth login"},
		{"http 404", &api.HTTPError{StatusCode: 404, Message: "Not Found", RequestURL: u}, "not found"},
		{"http 403 forbidden", &api.HTTPError{StatusCode: 403, Message: "Forbidden", RequestURL: u}, "access denied"},
		{"http 403 rate limit", &api.HTTPError{StatusCode: 403, Message: "API rate limit exceeded", RequestURL: u}, "rate limit"},
		{"http 429", &api.HTTPError{StatusCode: 429, Message: "Too Many Requests", RequestURL: u}, "rate limit"},
		{"graphql not found", &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "NOT_FOUND", Message: "Could not resolve"}}}, "not found"},
		{"graphql rate limited", &api.GraphQLError{Errors: []api.GraphQLErrorItem{{Type: "RATE_LIMITED", Message: "quota"}}}, "rate limit"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Wrap through fmt.Errorf to prove errors.As still reaches the status/type.
			class, hint := ClassifyUpstream(fmt.Errorf("operation failed: %w", tc.err))
			if class != ClassUpstream {
				t.Fatalf("class = %v, want %v", class, ClassUpstream)
			}
			if !strings.Contains(strings.ToLower(hint), strings.ToLower(tc.wantSub)) {
				t.Fatalf("hint = %q, missing %q", hint, tc.wantSub)
			}
		})
	}
}

// TestClassifyUpstreamUnmatched confirms an unrecognized failure is still
// ClassUpstream (the safe default for a GitHub/gh dependency error) but carries
// no invented hint — a wrong recovery is worse than none — and that nil is
// ClassNone with no hint.
func TestClassifyUpstreamUnmatched(t *testing.T) {
	if class, hint := ClassifyUpstream(errors.New("some novel backend explosion")); class != ClassUpstream || hint != "" {
		t.Fatalf("unmatched = (%v, %q), want (%v, \"\")", class, hint, ClassUpstream)
	}
	if class, hint := ClassifyUpstream(nil); class != ClassNone || hint != "" {
		t.Fatalf("nil = (%v, %q), want (%v, \"\")", class, hint, ClassNone)
	}
}

// TestClassifyUpstreamRespectsAssignedClass proves a class attached at the source
// wins over re-derivation: an *Error already marked ClassBadInput keeps its class
// and hint even though its text would otherwise match an upstream signature.
func TestClassifyUpstreamRespectsAssignedClass(t *testing.T) {
	src := &Error{Class: ClassBadInput, Err: errors.New("bad flag: not found"), Hint: "fix the flag"}
	class, hint := ClassifyUpstream(fmt.Errorf("wrapped: %w", src))
	if class != ClassBadInput {
		t.Fatalf("class = %v, want %v (assigned class must win)", class, ClassBadInput)
	}
	if hint != "fix the flag" {
		t.Fatalf("hint = %q, want assigned hint", hint)
	}
}

// TestParseRepoCarriesBadInputClass pins that a malformed slug is a ClassBadInput
// domain error (the source of the CLI's exit-2 mapping for `ghx explore badslug`,
// dogfood friction F1), reachable via errors.As, with the message unchanged.
func TestParseRepoCarriesBadInputClass(t *testing.T) {
	_, err := ParseRepo("badslug")
	if err == nil {
		t.Fatal("ParseRepo returned nil for a malformed slug")
	}
	var ce *Error
	if !errors.As(err, &ce) {
		t.Fatalf("ParseRepo error is not a *ghx.Error: %T", err)
	}
	if ce.Class != ClassBadInput {
		t.Fatalf("class = %v, want %v", ce.Class, ClassBadInput)
	}
	if !strings.Contains(err.Error(), "invalid repo") {
		t.Fatalf("message = %q, want it to contain \"invalid repo\"", err.Error())
	}
}

// TestFailureClassString pins the stable tokens frontends surface for each class.
func TestFailureClassString(t *testing.T) {
	for class, want := range map[FailureClass]string{
		ClassNone:      "none",
		ClassNoResults: "no_results",
		ClassBadInput:  "bad_input",
		ClassUpstream:  "upstream",
	} {
		if got := class.String(); got != want {
			t.Fatalf("%d.String() = %q, want %q", class, got, want)
		}
	}
}
