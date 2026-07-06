package sidecar

import (
	"crypto/sha256"
	"encoding/hex"
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
	"ANTHROPIC_CUSTOM_HEADERS",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"CLAUDE_CODE_OAUTH_TOKEN",
	// Provider selection + cloud auth (Amazon Bedrock).
	"CLAUDE_CODE_USE_BEDROCK",
	"AWS_REGION",
	"AWS_PROFILE",
	"AWS_BEARER_TOKEN_BEDROCK",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
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

// agentAuthEnvPassthrough is the subset of agentEnvVarNames whose VALUES the
// ask client captures from its own shell and forwards per-turn to the daemon
// (ADR-0033.1). The daemon is long-lived: its environment is frozen at
// whatever context first auto-started it, so ambient inheritance silently
// drops Bedrock/Vertex/gateway auth that IS present in the shell the human
// actually types in (founder work laptop, 2026-07-06: daemon env had only
// PATH+HOME; the adapter fell back to default Anthropic auth and died with
// "Authentication required"). Passthrough makes the rule deterministic: the
// agent authenticates as the shell that asked.
//
// Deliberately excluded: PATH and HOME (the daemon's own process context —
// overriding them can break adapter/node resolution mid-flight), and
// GH_TOKEN/GITHUB_TOKEN (ghx's own GitHub access, resolved by the ghx process
// serving the tools, not by the spawned agent).
//
// Values ride only the runtime-dir unix socket (0700) and process memory —
// they are never persisted or logged; session metadata records NAMES only.
var agentAuthEnvPassthrough = []string{
	// Anthropic direct auth / routing (incl. gateway: base URL + auth token +
	// custom headers is exactly the adapter's `gateway` ACP auth surface).
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_BASE_URL",
	"ANTHROPIC_CUSTOM_HEADERS",
	"ANTHROPIC_MODEL",
	"ANTHROPIC_SMALL_FAST_MODEL",
	"CLAUDE_CODE_OAUTH_TOKEN",
	// Provider selection + cloud auth (Amazon Bedrock).
	"CLAUDE_CODE_USE_BEDROCK",
	"AWS_REGION",
	"AWS_PROFILE",
	"AWS_BEARER_TOKEN_BEDROCK",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	// Provider selection + cloud auth (Google Vertex).
	"CLAUDE_CODE_USE_VERTEX",
	"ANTHROPIC_VERTEX_PROJECT_ID",
	"CLOUD_ML_REGION",
	"GOOGLE_APPLICATION_CREDENTIALS",
	// Adapter / CLI resolution.
	"CLAUDE_CODE_EXECUTABLE",
	"CLAUDE_CONFIG_DIR",
	// Transport (corporate proxies / custom CAs).
	"HTTP_PROXY",
	"HTTPS_PROXY",
	"NO_PROXY",
	"NODE_EXTRA_CA_CERTS",
}

// CaptureAgentAuthEnv returns the "NAME=VALUE" entries from environ whose
// names are on the passthrough allowlist and whose values are non-empty
// (ADR-0033.1). environ nil means the current process environment. The result
// preserves allowlist order. Callers must treat the result as secret-bearing:
// send it over the local daemon socket, never persist or log it.
func CaptureAgentAuthEnv(environ []string) []string {
	if environ == nil {
		environ = os.Environ()
	}
	values := make(map[string]string, len(environ))
	for _, kv := range environ {
		if i := strings.IndexByte(kv, '='); i > 0 && i < len(kv)-1 {
			values[kv[:i]] = kv[i+1:]
		}
	}
	var captured []string
	for _, name := range agentAuthEnvPassthrough {
		if v, ok := values[name]; ok && v != "" {
			captured = append(captured, name+"="+v)
		}
	}
	return captured
}

// MergeAgentEnv overlays "NAME=VALUE" entries onto a base environment,
// last-write-wins per name with base order preserved (ADR-0033.1). base nil
// means the current process environment. A nil/empty overlay returns nil so
// callers keep exec's default inherit-everything semantics instead of pinning
// a snapshot.
func MergeAgentEnv(base, overlay []string) []string {
	if len(overlay) == 0 {
		return nil
	}
	if base == nil {
		base = os.Environ()
	}
	overridden := make(map[string]string, len(overlay))
	for _, kv := range overlay {
		if i := strings.IndexByte(kv, '='); i > 0 {
			overridden[kv[:i]] = kv
		}
	}
	merged := make([]string, 0, len(base)+len(overlay))
	seen := make(map[string]struct{}, len(base))
	for _, kv := range base {
		name := kv
		if i := strings.IndexByte(kv, '='); i > 0 {
			name = kv[:i]
		}
		if repl, ok := overridden[name]; ok {
			merged = append(merged, repl)
			seen[name] = struct{}{}
			continue
		}
		merged = append(merged, kv)
		seen[name] = struct{}{}
	}
	for _, kv := range overlay {
		if i := strings.IndexByte(kv, '='); i > 0 {
			if _, ok := seen[kv[:i]]; !ok {
				merged = append(merged, kv)
			}
		}
	}
	return merged
}

// AgentEnvDigest fingerprints a spawn environment so a warm worker can detect
// that the asking shell's auth env changed and respawn its adapter instead of
// serving turns with stale credentials (ADR-0033.1). Only the hash ever
// leaves this function — values stay in memory. Empty env (inherit) digests
// to "".
func AgentEnvDigest(env []string) string {
	if len(env) == 0 {
		return ""
	}
	h := sha256.New()
	for _, kv := range env {
		h.Write([]byte(kv))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
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

// agentAuthMarker is the stable substring of the adapter's -32000 prompt-time
// failure when the spawned Claude Agent SDK has no usable credentials. Auth
// resolves lazily at prompt time, so ACP initialize (and thus the plain
// acp-handshake preflight) passes on machines that then fail every real turn —
// the founder's work-laptop failure shape (2026-07-06, post-ADR-0033
// diagnostics made it visible).
const agentAuthMarker = "Authentication required"

// agentAuthHint tells the operator how to give the spawned agent credentials.
// Like the trust hint it never auto-applies anything: logins and cloud env are
// the human's own config. The adapter's SDK keeps ITS OWN credential store —
// an installed `claude` being logged in proves nothing about the spawned
// agent (founder's work laptop: a toolbox-wrapped claude authenticated fine
// while the adapter's bundled Claude Code had no credentials at all), so the
// first remedy is the adapter's own login flow (the auth method it advertises
// over ACP: `--cli auth login`, zed-industries/claude-agent-acp
// src/acp-agent.ts initialize()).
const agentAuthHint = "The spawned Claude agent has no usable credentials (auth resolves at prompt time, so\n" +
	"handshake checks pass). The agent keeps its own credential store — your `claude` CLI\n" +
	"being logged in is NOT enough (wrapper/toolbox builds store credentials elsewhere).\n" +
	"Log the agent itself in once:\n" +
	"  npx -y @agentclientprotocol/claude-agent-acp --cli auth login --claudeai\n" +
	"If this machine authenticates through Bedrock/Vertex or a gateway instead, export those\n" +
	"env vars in the shell where you run ghx — every ask forwards them to the agent\n" +
	"automatically (ADR-0033.1), so no daemon restart is needed.\n" +
	"To use YOUR installed Claude Code (and its credentials/config) instead of the adapter's\n" +
	"bundled binary, set CLAUDE_CODE_EXECUTABLE to its path (see README: toolbox/wrapper builds).\n" +
	"`ghx sidecar sessions show <session>` lists which auth env names the agent actually saw."

// agentHints maps stable failure markers — observed in the turn error text or
// the adapter stderr tail — to operator guidance. One entry per diagnosed
// failure class (ADR-0033 D3); extend it as new classes earn a specific,
// non-destructive remedy.
var agentHints = []struct{ marker, hint string }{
	{workspaceTrustMarker, workspaceTrustHint},
	{agentAuthMarker, agentAuthHint},
}

// matchAgentHints returns the guidance for every failure marker present in any
// of the given texts, deduplicated, in registration order.
func matchAgentHints(texts ...string) []string {
	var hints []string
	for _, h := range agentHints {
		for _, t := range texts {
			if t != "" && strings.Contains(t, h.marker) {
				hints = append(hints, h.hint)
				break
			}
		}
	}
	return hints
}

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

// benignStderrMarkers identify adapter/SDK stderr lines that are pure noise
// in a failure diagnosis: boilerplate warnings every healthy spawn also
// prints. They stay in the on-disk agent-stderr.log (full evidence) but are
// dropped from the tail surfaced in errors, where they bury the one line
// that matters (founder feedback 2026-07-06: three repeated
// CLAUDE_SDK_CAN_USE_TOOL_SHADOWED blocks drowned an auth failure).
var benignStderrMarkers = []string{
	// The SDK notes our allowedTools entry auto-approves submit_report before
	// canUseTool runs — intentional sidecar wiring (ADR-0021), printed by
	// every spawned agent process.
	"CLAUDE_SDK_CAN_USE_TOOL_SHADOWED",
	"node --trace-warnings",
}

// filterBenignStderr drops known-benign lines from a stderr excerpt.
func filterBenignStderr(s string) string {
	lines := strings.Split(s, "\n")
	kept := lines[:0]
	for _, line := range lines {
		benign := false
		for _, m := range benignStderrMarkers {
			if strings.Contains(line, m) {
				benign = true
				break
			}
		}
		if !benign {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

// AgentStderrTail returns the last agentStderrTailBytes of the per-session
// adapter stderr log at path — with known-benign boilerplate lines filtered
// out — or "" when the log is missing, empty, or all-benign (ADR-0033 D2/D3).
// It is the evidence spliced into a turn-failure error and the loud-WARN
// answer; the unfiltered log on disk remains the complete record.
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
	return filterBenignStderr(strings.TrimSpace(string(buf)))
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
	// Hints match against the error text too, not just stderr: the adapter's
	// -32000 "Authentication required" arrives as the prompt error while stderr
	// carries only SDK cleanup noise (founder's laptop, 2026-07-06).
	hints := matchAgentHints(baseErr.Error(), tail)
	if tail == "" && len(hints) == 0 {
		return baseErr
	}
	var b strings.Builder
	b.WriteString(baseErr.Error())
	if tail != "" {
		fmt.Fprintf(&b, "\n\nAgent stderr (tail of %s):\n%s", stderrLogPath, tail)
	}
	for _, h := range hints {
		fmt.Fprintf(&b, "\n\nHint: %s", h)
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
	for _, h := range matchAgentHints(tail) {
		msg += "\n\nHint: " + h
	}
	return msg
}
