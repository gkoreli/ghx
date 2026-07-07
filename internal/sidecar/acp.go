package sidecar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

const defaultHandshakeTimeout = 10 * time.Second

// splitAgentCmd splits a configured agent command into argv. The config value
// may be a bare binary name ("claude") or a whitespace-separated command line
// ("npx -y @agentclientprotocol/claude-agent-acp@0.55.0" — what `config init
// --claude-acp` writes), so first-time setup does not require hand-writing a
// wrapper script (dogfood friction 2026-07-05). Splitting is plain field
// splitting with no shell quoting; a binary path containing spaces still
// needs a wrapper script.
func splitAgentCmd(agentCmd string) (name string, args []string) {
	return SplitAgentCmd(agentCmd)
}

// SplitAgentCmd splits a configured agent command line into argv for exec.
// Exported for the eval runner's identity probes (a multi-word AgentCmd such
// as the pinned npx adapter line must not be passed as a single argv[0]).
func SplitAgentCmd(agentCmd string) (name string, args []string) {
	fields := strings.Fields(agentCmd)
	if len(fields) == 0 {
		return agentCmd, nil
	}
	return fields[0], fields[1:]
}

// Runtime resilience taxonomy (ADR-0027). These markers are the stable,
// cross-package contract for classifying turn failures: the runtime uses them
// to pick a recovery path (D1 wrap-up, D2 fail-fast), and the eval layer uses
// them to derive anomalies (turn_cap_wrapup, episode_hang_timeout) from the
// persisted turn error strings without importing runtime internals.
const (
	// LivenessTimeoutEnv configures the D2 liveness watchdog window. Accepts a
	// Go duration ("10m", "90s"); "0" disables the watchdog. Default
	// DefaultLivenessTimeout.
	LivenessTimeoutEnv = "GHX_SIDECAR_LIVENESS_TIMEOUT"
	// DefaultLivenessTimeout is the watchdog window when the env var is unset:
	// no ACP session update for this long cancels the turn (ADR-0027 D2).
	DefaultLivenessTimeout = 10 * time.Minute

	// LivenessTimeoutMarker appears in every watchdog-cancelled turn error so
	// artifact-only consumers (evals anomaly detection) can classify the
	// failure from the persisted string alone.
	LivenessTimeoutMarker = "liveness watchdog timeout"
	// maxTurnsErrorMarker is the adapter's hard turn-cap error text
	// (claude-agent-acp: "Reached maximum number of turns (N)").
	maxTurnsErrorMarker = "Reached maximum number of turns"
	// peerClosedMarker matches the ACP SDK's dead-peer cause ("peer connection
	// closed", connection.go) — "peer disconnected" is its request-level twin.
	peerClosedMarker = "peer connection closed"
	// resourceNotFoundCode is the JSON-RPC/ACP code adapters use when
	// LoadSession cannot find the persisted transport session ID.
	resourceNotFoundCode = -32002
)

// ErrLivenessTimeout is the sentinel for a turn cancelled by the D2 liveness
// watchdog. Wrapped errors carry the concrete window; match with errors.Is or,
// from persisted strings, with LivenessTimeoutMarker.
var ErrLivenessTimeout = errors.New("sidecar " + LivenessTimeoutMarker)

// IsMaxTurnsError reports whether err is the adapter's hard turn-cap error
// (ADR-0027 D1 recovery trigger). Matching is textual because the error
// arrives as an opaque JSON-RPC internal error from the adapter.
func IsMaxTurnsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), maxTurnsErrorMarker)
}

// IsPeerClosedError reports whether err means the ACP peer died (process exit,
// closed stdio). Dead peers fail the turn immediately (ADR-0027 D2).
func IsPeerClosedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, peerClosedMarker) || strings.Contains(msg, "peer disconnected")
}

// IsLoadSessionResourceNotFound reports whether err is the stale-session miss
// that may be downgraded to a fresh ACP NewSession. It deliberately requires
// both the JSON-RPC resource-not-found code and the human message so unrelated
// LoadSession failures still fail loudly.
func IsLoadSessionResourceNotFound(err error) bool {
	if err == nil {
		return false
	}
	var reqErr *acp.RequestError
	if errors.As(err, &reqErr) {
		return reqErr.Code == resourceNotFoundCode && strings.Contains(strings.ToLower(reqErr.Message), "resource not found")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, fmt.Sprintf("%d", resourceNotFoundCode)) && strings.Contains(msg, "resource not found")
}

// resolveLivenessTimeout resolves the D2 watchdog window: an explicit option
// wins, then GHX_SIDECAR_LIVENESS_TIMEOUT, then DefaultLivenessTimeout.
// A zero (or negative) resolved value disables the watchdog.
func resolveLivenessTimeout(explicit time.Duration) time.Duration {
	if explicit != 0 {
		return explicit
	}
	raw := strings.TrimSpace(os.Getenv(LivenessTimeoutEnv))
	if raw == "" {
		return DefaultLivenessTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid %s=%q (want a Go duration like \"10m\"); using default %s\n",
			LivenessTimeoutEnv, raw, DefaultLivenessTimeout)
		return DefaultLivenessTimeout
	}
	return d
}

// ACPHandshakeFailureMessage is the actionable failure text shared by ask,
// doctor, and config init when a configured binary exists but does not speak
// ACP initialize on stdio.
func ACPHandshakeFailureMessage(agentCmd string) string {
	return fmt.Sprintf("agent %s did not complete the ACP handshake; run `ghx sidecar config init` or set an ACP-capable agent", agentCmd)
}

// shutdownGrace is how long ShutdownAgent waits for a spawned agent to exit
// on its own after stdin closes before falling back to a hard kill. Package
// variable (not const) so tests can shorten it.
var shutdownGrace = 2 * time.Second

// ShutdownAgent terminates a spawned ACP agent process gracefully: closing
// stdin signals EOF so a well-behaved stdio agent exits on its own — a hard
// kill mid-write makes Node-based adapters dump an EPIPE stack trace into
// logs. If the process is still running after shutdownGrace, it is killed.
// Safe to defer immediately after cmd.Start.
func ShutdownAgent(cmd *exec.Cmd, stdin io.Closer) {
	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		_ = cmd.Process.Kill()
		<-done
	}
}

// RunTurnOptions configures one ACP prompt turn.
type RunTurnOptions struct {
	AgentCmd     string
	ACPSessionID string
	Prompt       string
	Cwd          string
	Env          []string
	// SessionMeta is the _meta map forwarded to the adapter on BOTH session
	// creation (NewSession) and resume (LoadSession) — one map, built once by
	// BuildSessionMeta, carrying the full ADR-0020.1 D2 steering surface plus
	// the eval raw-SDK audit flag. Resumed turns must re-assert it because each
	// RunTurn spawns a fresh adapter process whose session options otherwise
	// fall back to adapter defaults (ADR-0020.2, TRUST H8). Nil means no
	// steering options.
	SessionMeta map[string]any
	// ReportSinkPath, when non-empty, registers the session-scoped report-sink
	// MCP server (ADR-0021 D1). The adapter spawns `<this executable> sidecar
	// report-sink --out <ReportSinkPath>` and exposes its submit_report tool to
	// the model; the runtime reads the accepted report back from this path after
	// the turn. Registered on both NewSession and LoadSession so resumed turns
	// keep the tool.
	ReportSinkPath string
	// LivenessTimeout overrides the D2 watchdog window for this turn. Zero
	// means "resolve from GHX_SIDECAR_LIVENESS_TIMEOUT (default 10m)";
	// negative disables the watchdog explicitly.
	LivenessTimeout time.Duration
	// AgentStderrPath, when non-empty, is the per-session file the spawned
	// adapter's stderr is teed to, in addition to the parent process stderr
	// (ADR-0033 D2). It makes a failed turn's adapter diagnostics recoverable
	// from committed artifacts instead of lost to the daemon's own log; the
	// runtime reads its tail into turn-failure errors and the loud-WARN answer.
	// Empty means stderr goes only to os.Stderr.
	AgentStderrPath string
	// LiveLogPath, when non-empty, is the per-session live turn log the ACP
	// client appends realtime NDJSON activity events to as session updates
	// arrive (ADR-0022.1) — text/thought chunks, tool calls, tool updates — so
	// a caller can `tail -f` a running turn instead of waiting for the OTLP
	// artifacts at Ask exit. Best-effort: an unopenable path degrades to a
	// no-op. Empty means no live log.
	LiveLogPath string
}

// RunTurn spawns the agent binary, establishes an ACP session (new or resumed),
// sends prompt, streams output, and returns the collected TurnResult and the
// ACP session ID to persist for follow-up turns.
//
// Pass acpSessionID="" on the first turn; pass the returned sessionID on
// subsequent turns to resume the agent's context.
func RunTurn(ctx context.Context, agentCmd, acpSessionID, prompt string) (result TurnResult, newSessionID string, err error) {
	return RunTurnWithOptions(ctx, RunTurnOptions{AgentCmd: agentCmd, ACPSessionID: acpSessionID, Prompt: prompt})
}

// RunTurnWithOptions is RunTurn plus eval/runtime overrides for cwd and env.
//
// Resilience (ADR-0027 D2): the turn runs under a cancellable wait — the
// prompt is awaited in a select over completion, parent-context cancellation,
// the liveness watchdog, and the peer connection dying. A dead peer fails the
// turn immediately; silence beyond the liveness window cancels it with
// ErrLivenessTimeout. On failure the partial TurnResult AND the established
// ACP session ID are still returned so callers can flush artifacts (D3) and
// resume the same session for the wrap-up recovery prompt (D1).
func RunTurnWithOptions(ctx context.Context, opts RunTurnOptions) (result TurnResult, newSessionID string, err error) {
	liveness := resolveLivenessTimeout(opts.LivenessTimeout)
	turnCtx, cancelTurn := context.WithCancelCause(ctx)
	defer cancelTurn(nil)

	agentBin, agentArgs := splitAgentCmd(opts.AgentCmd)
	cmd := exec.CommandContext(turnCtx, agentBin, agentArgs...)
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	// Tee the adapter's stderr to the per-session log (ADR-0033 D2) so a failed
	// turn's diagnostics survive as a committed artifact, not only in whatever
	// stderr the daemon happens to own.
	stderrW, stderrCloser := openAgentStderr(opts.AgentStderrPath, opts.Env)
	defer stderrCloser.Close()
	cmd.Stderr = stderrW

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return result, "", fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, "", fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return result, "", fmt.Errorf("start %q: %w", opts.AgentCmd, err)
	}
	defer ShutdownAgent(cmd, stdin)

	// Liveness clock: every ACP session update (and every completed RPC step)
	// counts as a sign of life. The watchdog cancels the turn — which also
	// kills the spawned agent via exec.CommandContext — when the clock goes
	// stale for the whole window (ADR-0027 D2).
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())
	touch := func() { lastActivity.Store(time.Now().UnixNano()) }

	// Live turn log (ADR-0022.1): stream this turn's session updates to the
	// session's live.jsonl as they happen. Append mode makes the concurrent
	// runtime-owned writer for the same path safe.
	live := NewLiveLog(opts.LiveLogPath)
	defer live.Close()

	client := &denyClient{result: &result, onActivity: touch, live: live}
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	if liveness > 0 {
		go func() {
			for {
				idle := time.Duration(time.Now().UnixNano() - lastActivity.Load())
				remaining := liveness - idle
				if remaining <= 0 {
					cancelTurn(fmt.Errorf("%w: no ACP session update for %s (set %s to adjust)",
						ErrLivenessTimeout, liveness, LivenessTimeoutEnv))
					return
				}
				select {
				case <-turnCtx.Done():
					return
				case <-time.After(remaining):
				}
			}
		}()
	}

	// cwd is the ACP session workspace. The authoritative neutral-workspace
	// policy (the ghx-owned session directory) lives one layer up in the runtime
	// that knows the session identity (ADR-0033 D1); os.Getwd() is only a
	// last-resort default for bare callers (the RunTurn helper, probes) that do
	// not set it.
	cwd := opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	initResp, err := conn.Initialize(turnCtx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	})
	if err != nil {
		return result, "", fmt.Errorf("acp initialize: %w", turnFailureCause(turnCtx, err))
	}
	touch()
	if initResp.AgentInfo != nil {
		result.AgentInfo = &ImplementationInfo{
			Name:    initResp.AgentInfo.Name,
			Version: initResp.AgentInfo.Version,
			Meta:    initResp.AgentInfo.Meta,
		}
	}

	var sessionID acp.SessionId
	if opts.ACPSessionID == "" || !initResp.AgentCapabilities.LoadSession {
		// New session: forward the session-level steering meta (ADR-0020.1 D2).
		// The _meta bag is adapter-specific; the ACP spec treats it as opaque.
		req := acp.NewSessionRequest{
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
		}
		if opts.SessionMeta != nil {
			req.Meta = opts.SessionMeta
		}
		resp, err := conn.NewSession(turnCtx, req)
		if err != nil {
			return result, "", fmt.Errorf("acp new session: %w", turnFailureCause(turnCtx, err))
		}
		sessionID = resp.SessionId
	} else {
		// Resumed turns re-assert the FULL session meta (ADR-0020.2): each
		// RunTurn is a fresh adapter process, and without the meta the adapter
		// rebuilds the session on defaults — no persona system prompt, no
		// tools allowlist, no budgets/model pin, no isolation, and (in eval
		// mode) no raw-SDK audit channel. The same map NewSession sends is
		// forwarded verbatim; the adapter's session fingerprint excludes
		// _meta, so this cannot fork the session (ADR-0016.10 D2 recon).
		_, err := conn.LoadSession(turnCtx, acp.LoadSessionRequest{
			SessionId:  acp.SessionId(opts.ACPSessionID),
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
			Meta:       opts.SessionMeta,
		})
		if err != nil {
			return result, "", fmt.Errorf("acp load session: %w", turnFailureCause(turnCtx, err))
		}
		sessionID = acp.SessionId(opts.ACPSessionID)
	}
	touch()

	client.markPromptSent()
	promptDone := make(chan error, 1)
	go func() {
		_, perr := conn.Prompt(turnCtx, acp.PromptRequest{
			SessionId: sessionID,
			Prompt:    []acp.ContentBlock{acp.TextBlock(opts.Prompt)},
		})
		promptDone <- perr
	}()

	var promptErr error
	select {
	case promptErr = <-promptDone:
	case <-conn.Done():
		// Dead peer: fail the turn immediately (ADR-0027 D2) — never wait for
		// a timer on a connection that cannot answer. Give the in-flight
		// Prompt call a short grace to unwind first: when both channels are
		// ready (agent answered and then exited) the completed prompt wins.
		perr, finished := awaitPromptOrGrace(promptDone)
		if finished {
			promptErr = perr
		} else {
			promptErr = errors.New(peerClosedMarker + " before the prompt turn completed")
		}
		if promptErr != nil && !IsPeerClosedError(promptErr) {
			promptErr = fmt.Errorf("%s: %w", peerClosedMarker, promptErr)
		}
	case <-turnCtx.Done():
		// Watchdog or parent cancellation. The SDK's own wait also selects on
		// this context; the grace only covers its unwinding.
		if perr, finished := awaitPromptOrGrace(promptDone); finished {
			promptErr = perr
		} else {
			promptErr = context.Cause(turnCtx)
		}
	}
	// Prefer the watchdog cause over the SDK's stringified rendering of it so
	// errors.Is(err, ErrLivenessTimeout) works for callers.
	if cause := context.Cause(turnCtx); errors.Is(cause, ErrLivenessTimeout) {
		promptErr = cause
	}
	// Freeze the result before returning: late notifications from a dying
	// peer must not race the caller's reads (D3 flushes partial results).
	result = client.closeAndSnapshot()
	if promptErr != nil {
		return result, string(sessionID), fmt.Errorf("acp prompt: %w", promptErr)
	}

	os.Stdout.WriteString("\n")
	return result, string(sessionID), nil
}

// awaitPromptOrGrace waits briefly for an abandoned Prompt call to unwind.
// It returns the prompt's error and whether the prompt finished within the
// grace window (a finished prompt with a nil error is a genuine success even
// when the peer exited right behind it).
func awaitPromptOrGrace(promptDone <-chan error) (error, bool) {
	select {
	case err := <-promptDone:
		return err, true
	case <-time.After(shutdownGrace):
		return nil, false
	}
}

// turnFailureCause maps an RPC error to the turn-level cause when the turn
// context was cancelled by the liveness watchdog, so pre-prompt hangs
// (initialize / session setup) classify the same way as prompt hangs.
func turnFailureCause(turnCtx context.Context, err error) error {
	if cause := context.Cause(turnCtx); errors.Is(cause, ErrLivenessTimeout) {
		return cause
	}
	return err
}

// CheckACPHandshake spawns an agent and verifies that ACP initialize completes
// over stdio before the timeout. It intentionally stops after initialize:
// preflight needs protocol compatibility, not a working prompt turn.
func CheckACPHandshake(ctx context.Context, agentCmd, cwd string, env []string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultHandshakeTimeout
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	agentBin, agentArgs := splitAgentCmd(agentCmd)
	cmd := exec.CommandContext(tctx, agentBin, agentArgs...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", agentCmd, err)
	}
	defer ShutdownAgent(cmd, stdin)

	result := TurnResult{}
	conn := acp.NewClientSideConnection(&denyClient{result: &result}, stdin, stdout)
	if _, err := conn.Initialize(tctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	}); err != nil {
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			return errors.New(ACPHandshakeFailureMessage(agentCmd))
		}
		return fmt.Errorf("%s: %w", ACPHandshakeFailureMessage(agentCmd), err)
	}
	return nil
}
