package sidecar

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Agent diagnostics (ADR-0033). A founder-dogfood run failed silently on a work
// laptop — no report, no error, the only breadcrumb buried in the daemon log.
// The failure was environment-specific and invisible: the spawned adapter's
// stderr went to the daemon's own stderr, and an empty turn degraded to a mute
// WARN. This file is the single owner of making such failures loud and
// diagnosable: capture the adapter's stderr per session, surface its tail on
// the CLI, and fingerprint the agent-relevant environment (names only).

// agentEnvVarNames is the curated allowlist of environment variable NAMES whose
// presence is recorded in SessionMeta.AgentEnv (ADR-0033 D5). It is deliberately
// names-only: the value of any of these may be a secret, so only presence is
// ever persisted. The set covers the environment that actually decides whether
// the Claude Agent SDK subprocess can authenticate and reach its endpoint —
// exactly the surface an ACP session option cannot pin (auth is process env,
// not `_meta`; see ADR-0033 research). Keep it grouped and documented so a
// future auth mode (a new provider) is a one-line, reviewable addition.
var agentEnvVarNames = []string{
	// Anthropic direct auth / routing.
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"CLAUDE_CODE_OAUTH_TOKEN",
	// Provider selection + cloud auth (Amazon Bedrock).
	"CLAUDE_CODE_USE_BEDROCK",
	"AWS_REGION",
	"AWS_PROFILE",
	"AWS_BEARER_TOKEN_BEDROCK",
	"AWS_ACCESS_KEY_ID",
	"AWS_SESSION_TOKEN",
	// Provider selection + cloud auth (Google Vertex).
	"CLAUDE_CODE_USE_VERTEX",
	"ANTHROPIC_VERTEX_PROJECT_ID",
	"CLOUD_ML_REGION",
	"GOOGLE_APPLICATION_CREDENTIALS",
	// Adapter / CLI resolution.
	"CLAUDE_CODE_EXECUTABLE",
	"CLAUDE_CONFIG_DIR",
	// Transport (corporate proxies / custom CAs are a common laptop-only diff).
	"HTTP_PROXY",
	"HTTPS_PROXY",
	"NO_PROXY",
	"NODE_EXTRA_CA_CERTS",
	// ghx's own GitHub access.
	"GH_TOKEN",
	"GITHUB_TOKEN",
	// Baseline process context the adapter forwards to the SDK subprocess.
	"PATH",
	"HOME",
}

// PresentAgentEnv returns the names (never values) from agentEnvVarNames that
// are set to a non-empty value in environ (ADR-0033 D5). environ is a slice of
// "NAME=VALUE" entries as returned by os.Environ; nil means read the current
// process environment. The result preserves agentEnvVarNames order so the
// fingerprint of two sessions is directly comparable.
func PresentAgentEnv(environ []string) []string {
	if environ == nil {
		environ = os.Environ()
	}
	set := make(map[string]struct{}, len(environ))
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i > 0 && i < len(kv)-1 {
			set[kv[:i]] = struct{}{}
		}
	}
	var present []string
	for _, name := range agentEnvVarNames {
		if _, ok := set[name]; ok {
			present = append(present, name)
		}
	}
	return present
}

// workspaceTrustMarker is the stable substring of Claude Code's untrusted
// workspace warning ("this workspace has not been trusted"; docs slug
// code.claude.com/docs/en/errors#workspace-has-not-been-trusted). ADR-0033's
// reproduction showed trust was a red herring for the founder's failure, but
// when the marker IS present it has an exact, non-destructive remedy, so it
// earns a specific hint.
const workspaceTrustMarker = "has not been trusted"

// IsWorkspaceTrustWarning reports whether adapter stderr contains the
// documented untrusted-workspace warning.
func IsWorkspaceTrustWarning(stderr string) bool {
	return strings.Contains(stderr, workspaceTrustMarker)
}

// workspaceTrustHint is shown only as guidance when the trust marker appears in
// adapter stderr (ADR-0033 D3). It never auto-applies anything and points the
// human at THEIR OWN config: ghx must never write ~/.claude.json or
// .claude/settings*.json (Open Source Leverage tenet — official knobs only).
const workspaceTrustHint = "The spawned agent reported an untrusted workspace and ignored its permission settings.\n" +
	"ghx runs the agent in a neutral session directory to avoid this; if you still see it,\n" +
	"the agent picked up a git-repo workspace. Fix it yourself (ghx will not touch your config):\n" +
	"trust that workspace once by running the agent interactively there and accepting the trust\n" +
	"dialog, or set projects[\"<dir>\"].hasTrustDialogAccepted: true in ~/.claude.json."

// openAgentStderr returns the writer a spawned adapter's stderr is teed to: the
// per-session agent-stderr.log (truncated fresh for this process) plus the
// parent process stderr, so foreground `ghx sidecar daemon` still streams live
// logs while a durable per-session diagnostic artifact always exists
// (ADR-0033 D2). An empty path (or an unopenable file) yields os.Stderr and a
// no-op closer — capture degrades, it never breaks the turn.
func openAgentStderr(path string) (io.Writer, io.Closer) {
	if path == "" {
		return os.Stderr, io.NopCloser(nil)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open agent stderr log %s: %v\n", path, err)
		return os.Stderr, io.NopCloser(nil)
	}
	return io.MultiWriter(os.Stderr, f), f
}

// agentStderrTailBytes bounds how much of the adapter stderr log is read back
// into an error message. A few KB is enough to carry the failing lines without
// bloating the error the CLI prints.
const agentStderrTailBytes = 4096

// AgentStderrTail returns the last agentStderrTailBytes of the per-session
// adapter stderr log at path, trimmed, or "" when the log is missing or empty
// (ADR-0033 D2/D3). It is the evidence spliced into a turn-failure error and
// the loud-WARN answer.
func AgentStderrTail(path string) string {
	if path == "" {
		return ""
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return ""
	}
	size := info.Size()
	start := int64(0)
	if size > agentStderrTailBytes {
		start = size - agentStderrTailBytes
	}
	buf := make([]byte, size-start)
	if _, err := f.ReadAt(buf, start); err != nil && err != io.EOF {
		return ""
	}
	return strings.TrimSpace(string(buf))
}

// DiagnoseTurnError enriches a turn-failure error with the tail of the
// per-session adapter stderr log and its path, plus the workspace-trust hint
// when that specific warning is present (ADR-0033 D3). It wraps baseErr with
// %w so errors.Is chains (ErrLivenessTimeout, ACP RequestError) survive for the
// ADR-0027 resilience classifiers. When the log is empty (the mock/unit case,
// or a failure before the adapter wrote anything) baseErr is returned unchanged
// so nothing spurious is appended.
func DiagnoseTurnError(baseErr error, stderrLogPath string) error {
	if baseErr == nil {
		return nil
	}
	tail := AgentStderrTail(stderrLogPath)
	if tail == "" {
		return baseErr
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\nAgent stderr (tail of %s):\n%s", baseErr.Error(), stderrLogPath, tail)
	if IsWorkspaceTrustWarning(tail) {
		fmt.Fprintf(&b, "\n\nHint: %s", workspaceTrustHint)
	}
	return &diagnosedError{base: baseErr, msg: b.String()}
}

// diagnosedError carries an enriched, human-facing message while preserving the
// wrapped base error's identity for errors.Is/As.
type diagnosedError struct {
	base error
	msg  string
}

func (e *diagnosedError) Error() string { return e.msg }
func (e *diagnosedError) Unwrap() error { return e.base }

// warnNoReportBase is the honest placeholder when a turn completed but produced
// no structured report (ADR-0021 D2 last resort).
const warnNoReportBase = "WARN: sidecar did not emit a report — raw output was produced but no structured report was found."

// warnNoReportAnswer builds the loud, diagnosable answer for the no-report case
// (ADR-0033 D3). It always points the caller at the per-session adapter stderr
// log so the mute silent-failure the founder hit becomes a dead end no longer;
// when that log shows the untrusted-workspace warning, it appends the remedy.
func warnNoReportAnswer(stderrLogPath string) string {
	tail := AgentStderrTail(stderrLogPath)
	if tail == "" {
		if stderrLogPath == "" {
			return warnNoReportBase
		}
		return warnNoReportBase + "\nThe agent also produced no stderr; see " + stderrLogPath + " for adapter diagnostics."
	}
	msg := warnNoReportBase + "\nAdapter stderr (tail of " + stderrLogPath + "):\n" + tail
	if IsWorkspaceTrustWarning(tail) {
		msg += "\n\nHint: " + workspaceTrustHint
	}
	return msg
}
