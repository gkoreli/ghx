package cli

import (
	"errors"
	"fmt"
	"strings"

	ghxlib "github.com/gkoreli/ghx/v2/internal/ghx"
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

// exitForClass maps a core FailureClass onto the CLI's semantic exit code. This
// is the CLI's single, declarative rendering of the shared taxonomy (ADR-0034):
// the class is decided once in core; the frontend only chooses its idiom.
func exitForClass(class ghxlib.FailureClass) int {
	switch class {
	case ghxlib.ClassNoResults:
		return ExitNoResults
	case ghxlib.ClassBadInput:
		return ExitBadInvocation
	case ghxlib.ClassUpstream:
		return ExitUpstreamFailure
	default:
		return ExitOK
	}
}

// CodeForError returns the semantic exit code for err. A CLI ExitError set by a
// handler wins (it is the deliberate, final mapping); otherwise the FailureClass
// a core ghx.Error carries is honored; an unclassified error is a bad invocation.
func CodeForError(err error) int {
	if err == nil {
		return ExitOK
	}
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	var coreErr *ghxlib.Error
	if errors.As(err, &coreErr) {
		if code := exitForClass(coreErr.Class); code != ExitOK {
			return code
		}
		return ExitUpstreamFailure
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

// coreError maps a core error to its CLI idiom in one place: it sources the
// FailureClass and recovery hint from the ghx.Error that core attached at the
// source, then wraps with the matching exit code and the fix-it affordance line.
// This is the CLI's sole entry point for errors returned by internal/ghx and the
// tier-2 backends; there is no substring classification table in the CLI anymore
// (ADR-0034 phase 2 — deleted the duplicated upstreamRules).
//
// An error that carries no ghx.Error (e.g. a local tier-2 git/tool failure) is
// classified once through the shared core classifier and defaults to an upstream
// failure, preserving the prior always-exit-3 behavior for those paths.
func coreError(err error) error {
	if err == nil {
		return nil
	}
	var coreErr *ghxlib.Error
	if errors.As(err, &coreErr) {
		code := exitForClass(coreErr.Class)
		if code == ExitOK {
			code = ExitUpstreamFailure
		}
		return withAffordance(code, err, coreErr.Hint)
	}
	_, hint := ghxlib.ClassifyUpstream(err)
	return withAffordance(ExitUpstreamFailure, err, hint)
}
