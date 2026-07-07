package cli

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// ExitOK is returned by successful commands.
	ExitOK = 0
	// ExitNoResults means the command ran correctly but found no matching data.
	ExitNoResults = 1
	// ExitBadInvocation means arguments or flags were invalid.
	ExitBadInvocation = 2
	// ExitUpstreamFailure means GitHub, gh auth, or another upstream dependency failed.
	ExitUpstreamFailure = 3
)

// ExitError carries the process exit code for a CLI failure.
type ExitError struct {
	Code int
	Err  error
}

// Error returns the wrapped error text.
func (e *ExitError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e *ExitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WithExitCode marks err with the semantic CLI exit code.
func WithExitCode(code int, err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: code, Err: err}
}

// CodeForError returns the semantic exit code for err.
func CodeForError(err error) int {
	if err == nil {
		return ExitOK
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitBadInvocation
}

// affordanceMarker prefixes the fix-it line appended to an error. Matching the
// "→" convention already used for success-path truncation hints keeps a single
// visual language for "here is your next move" across the CLI surface.
const affordanceMarker = "→ "

// withAffordance wraps err with the given exit code and appends a fix-it line
// naming the correct next invocation. hint is the imperative next step (command
// or flag form); it is rendered on its own line so an agent parsing the tail of
// stderr always finds the recovery move in the same place. An empty hint leaves
// the error unchanged apart from the exit code, so callers can pass a computed
// hint that is sometimes absent without branching.
func withAffordance(code int, err error, hint string) error {
	if err == nil {
		return nil
	}
	hint = strings.TrimSpace(hint)
	if hint == "" {
		return WithExitCode(code, err)
	}
	return WithExitCode(code, fmt.Errorf("%w\n%s%s", err, affordanceMarker, hint))
}

// upstreamRule classifies a raw upstream error by substring signature and names
// the exact recovery command. Declarative table (anomaly-detector model): each
// row is an independent, ordered matcher, so adding a newly observed failure
// class is a one-line append with no control-flow surgery. Signatures are
// lowercased before matching.
type upstreamRule struct {
	// signatures are matched (case-insensitively) against the raw error text;
	// any one matching selects this rule.
	signatures []string
	// hint is the fix-it line: the concrete command or check that recovers.
	hint string
}

// upstreamRules maps observed GitHub/gh failure classes to the invocation that
// fixes them. Grounded in real eval + dogfood friction (auth, rate-limit, not
// found, network). Ordered most-specific first so a rate-limit 403 does not get
// captured by a generic auth rule.
var upstreamRules = []upstreamRule{
	{
		signatures: []string{"rate limit", "rate-limit", "api rate limit exceeded", "secondary rate"},
		hint:       "GitHub API rate limit hit. Authenticate for a higher quota with `gh auth login`, then retry; check remaining quota with `gh api rate_limit`.",
	},
	{
		signatures: []string{"gh auth", "authentication", "authenticated", "unauthorized", "bad credentials", "401", "http 401", "not logged", "gh: to use github"},
		hint:       "GitHub authentication failed. Run `gh auth login` (or set GH_TOKEN), then verify with `gh auth status` and retry.",
	},
	{
		signatures: []string{"404", "not found", "could not resolve to a repository", "name not resolvable", "no such repository"},
		hint:       "Repository, branch, or path not found. Confirm owner/repo with `ghx repos <query>` and the layout with `ghx explore owner/repo` before reading a path.",
	},
	{
		signatures: []string{"403", "forbidden", "access denied", "must have push access"},
		hint:       "Access denied by GitHub. Check `gh auth status` scopes; private repos need a token with `repo` scope via `gh auth login`.",
	},
	{
		signatures: []string{"connection refused", "timeout", "timed out", "no route to host", "no such host", "dial tcp", "network is unreachable", "tls handshake"},
		hint:       "Network reached GitHub but failed. Check connectivity and `gh auth status`, then retry the same command.",
	},
}

// upstreamAffordance returns the recovery hint for an upstream (exit 3) error,
// or "" when no rule matches. Callers pass the result straight to
// withAffordance, so an unclassified upstream error surfaces its raw text
// unchanged (never a misleading hint).
func upstreamAffordance(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	for _, rule := range upstreamRules {
		for _, sig := range rule.signatures {
			if strings.Contains(msg, sig) {
				return rule.hint
			}
		}
	}
	return ""
}

// upstreamError wraps a raw upstream dependency failure as an exit-3 error,
// appending a fix-it affordance when the failure class is recognized. This is
// the single entry point CLI handlers use for GitHub/gh/API errors so the
// affordance policy lives in one place.
func upstreamError(err error) error {
	return withAffordance(ExitUpstreamFailure, err, upstreamAffordance(err))
}
