package ghx

import (
	"errors"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// FailureClass categorizes a reconnaissance failure by the recovery it implies,
// independent of the frontend that surfaces it. It is the domain concept behind
// the CLI's exit-code taxonomy (0/1/2/3), lifted into core so every frontend —
// CLI exit codes, the MCP tool error, the sidecar report — maps one shared class
// to its own idiom instead of re-deriving it from error strings (ADR-0034).
type FailureClass int

const (
	// ClassNone means no failure: the operation succeeded.
	ClassNone FailureClass = iota
	// ClassNoResults means the operation ran correctly but matched no data. The
	// recovery is to refine or broaden the query, not to fix credentials.
	ClassNoResults
	// ClassBadInput means the caller's arguments or flags were invalid. The
	// recovery is to fix the invocation.
	ClassBadInput
	// ClassUpstream means an upstream dependency (the GitHub API, gh auth, or the
	// network) failed. The recovery is auth/quota/connectivity, named by Hint.
	ClassUpstream
)

// String returns a stable lowercase token for the class, for logs and structured
// payloads (frontends that surface the class to an agent read this).
func (c FailureClass) String() string {
	switch c {
	case ClassNone:
		return "none"
	case ClassNoResults:
		return "no_results"
	case ClassBadInput:
		return "bad_input"
	case ClassUpstream:
		return "upstream"
	default:
		return "unknown"
	}
}

// Error is ghx's typed domain error. It carries the FailureClass that classifies
// the failure and an optional Hint naming the concrete recovery command, so a
// frontend can branch on Class and surface Hint without re-parsing Err's text.
type Error struct {
	// Class is the failure category (see FailureClass).
	Class FailureClass
	// Err is the underlying error. Error() and Unwrap() delegate to it so the
	// original message and error chain (e.g. an *api.HTTPError) stay intact and
	// the rendered text is byte-identical to the wrapped error.
	Err error
	// Hint is the imperative recovery step (a runnable command or check), or ""
	// when the failure class has no specific recovery to name.
	Hint string
}

// Error returns the underlying error text verbatim, with no class/hint
// decoration, keeping the message identical to the wrapped error.
func (e *Error) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap returns the underlying error so errors.Is/As traverse the chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// badInput wraps err as a caller-invocation failure (ClassBadInput). Returns nil
// when err is nil so call sites can wrap unconditionally.
func badInput(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Class: ClassBadInput, Err: err}
}

// upstream wraps err as an upstream-dependency failure (ClassUpstream), running
// the shared classifier to attach the recovery hint at the source — where the
// structured HTTP status / GraphQL error type is still available. Returns nil
// when err is nil so call sites can wrap unconditionally.
func upstream(err error) error {
	if err == nil {
		return nil
	}
	_, hint := ClassifyUpstream(err)
	return &Error{Class: ClassUpstream, Err: err, Hint: hint}
}

// Recovery hints name the exact command that clears each observed GitHub/gh
// failure class. Grounded in real eval + dogfood friction (auth, rate-limit, not
// found, forbidden, network); the auth/GH_TOKEN remediation mirrors the sidecar
// preflight's (internal/sidecar/preflight.go) so the fix-it text is consistent
// across frontends.
const (
	rateLimitHint = "GitHub API rate limit hit. Authenticate for a higher quota with `gh auth login`, then retry; check remaining quota with `gh api rate_limit`."
	authHint      = "GitHub authentication failed. Run `gh auth login` (or set GH_TOKEN), then verify with `gh auth status` and retry."
	notFoundHint  = "Repository, branch, or path not found. Confirm owner/repo with `ghx repos <query>` and the layout with `ghx explore owner/repo` before reading a path."
	forbiddenHint = "Access denied by GitHub. Check `gh auth status` scopes; private repos need a token with `repo` scope via `gh auth login`."
	networkHint   = "Network reached GitHub but failed. Check connectivity and `gh auth status`, then retry the same command."
)

// Failure signatures matched (case-insensitively) against a raw error's text when
// no structured status is available. Ordered most-specific first at the call site
// so a rate-limit 403 is not captured by the generic forbidden rule. Declarative
// tables (the anomaly-detector model) keep adding a newly observed signature a
// one-line append.
var (
	rateLimitSignatures = []string{"rate limit", "rate-limit", "api rate limit exceeded", "secondary rate"}
	authSignatures      = []string{"gh auth", "authentication", "authenticated", "unauthorized", "bad credentials", "401", "http 401", "not logged", "gh: to use github"}
	notFoundSignatures  = []string{"404", "not found", "could not resolve to a repository", "name not resolvable", "no such repository"}
	forbiddenSignatures = []string{"403", "forbidden", "access denied", "must have push access"}
	networkSignatures   = []string{"connection refused", "timeout", "timed out", "no route to host", "no such host", "dial tcp", "network is unreachable", "tls handshake"}
)

// ClassifyUpstream inspects a raw GitHub/gh/network error and returns its
// FailureClass and the recovery hint. It reads structured signals first — the
// HTTP status on an *api.HTTPError and the error type on an *api.GraphQLError,
// where the real response is known — then falls back to signature matching for
// transport errors that carry no structured status (and for pre-stringified
// errors). Every classified GitHub/gh failure is ClassUpstream; the
// sub-classification only selects the hint. A nil error is ClassNone.
//
// The ordered switch preserves the historical first-match precedence (rate-limit
// before auth before not-found before forbidden before network) so an ambiguous
// message resolves to the same hint the CLI's prior substring table produced.
func ClassifyUpstream(err error) (FailureClass, string) {
	if err == nil {
		return ClassNone, ""
	}
	// A class assigned at the source wins: never re-derive what core already knows.
	var typed *Error
	if errors.As(err, &typed) && typed.Class != ClassNone {
		return typed.Class, typed.Hint
	}

	status := httpStatus(err)
	gqlType := graphQLType(err)
	msg := strings.ToLower(err.Error())

	switch {
	case status == 429 || gqlType == "RATE_LIMITED" || containsAny(msg, rateLimitSignatures):
		return ClassUpstream, rateLimitHint
	case status == 401 || gqlType == "UNAUTHORIZED" || containsAny(msg, authSignatures):
		return ClassUpstream, authHint
	case status == 404 || gqlType == "NOT_FOUND" || containsAny(msg, notFoundSignatures):
		return ClassUpstream, notFoundHint
	case status == 403 || gqlType == "FORBIDDEN" || containsAny(msg, forbiddenSignatures):
		return ClassUpstream, forbiddenHint
	case containsAny(msg, networkSignatures):
		return ClassUpstream, networkHint
	}
	return ClassUpstream, ""
}

// httpStatus returns the HTTP status code of the first *api.HTTPError in err's
// chain, or 0 when none is present.
func httpStatus(err error) int {
	var e *api.HTTPError
	if errors.As(err, &e) {
		return e.StatusCode
	}
	return 0
}

// graphQLType returns the upper-cased Type of the first GraphQL error item in
// err's chain that carries one, or "" when none is present. GitHub sets Type to
// values like NOT_FOUND, FORBIDDEN, and RATE_LIMITED.
func graphQLType(err error) string {
	var e *api.GraphQLError
	if errors.As(err, &e) {
		for _, item := range e.Errors {
			if item.Type != "" {
				return strings.ToUpper(item.Type)
			}
		}
	}
	return ""
}

// containsAny reports whether haystack contains any of the needles.
func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}
