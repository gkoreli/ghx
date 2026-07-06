package cli

import "errors"

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
