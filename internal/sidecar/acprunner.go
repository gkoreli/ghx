package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// Adapter 1 — claude-agent-acp behind the Runner port (ADR-0036 D1). This file
// is where the ACP knowledge that used to be spread across the runtime is
// sealed: the wire steering encoding (_meta.claudeCode.options), the streamed
// side effects (stdout/live.jsonl/stderr), and the failure-string matchers all
// live behind claudeACPRunner/claudeACPSession/streamEventSink. The runtime
// (Ask) depends only on the port; it never sees an acp type.
//
// It is behavior-identical to the pre-port code: the same TurnRunner
// (RunTurnWithOptions one-shot, or the warm AgentWorker) runs the turn, the same
// BuildSessionMeta encodes the steering, and the same markers classify failures.

// Compile-time proof the ACP path implements the port.
var (
	_ Runner    = (*claudeACPRunner)(nil)
	_ Session   = (*claudeACPSession)(nil)
	_ EventSink = (*streamEventSink)(nil)
)

// streamEventSink is the claude-acp EventSink: it owns the runtime-visible side
// effects of a streamed turn — assistant text to stdout, every event to the
// session's live.jsonl, and the tool-progress line to stderr — reproducing
// byte-for-byte what denyClient wrote inline before the Runner-port inversion
// (ADR-0036 D1). Activity and RawStreamMessage are no-ops in C1: the liveness
// watchdog stays adapter-internal (fed by denyClient.onActivity) and the
// raw-SDK audit stays in denyClient.HandleExtensionMethod.
type streamEventSink struct {
	live *LiveLog
}

func (s *streamEventSink) Text(delta string) {
	os.Stdout.WriteString(delta)
	s.live.Text(len(delta), delta)
}

func (s *streamEventSink) Thinking(delta string) {
	s.live.Thought(len(delta), delta)
}

func (s *streamEventSink) ToolStarted(call ToolCall) {
	s.live.ToolCall(call.ID, call.Title, call.Kind, call.Status)
	fmt.Fprintf(os.Stderr, "  ▶ %s\n", call.Summary)
}

func (s *streamEventSink) ToolUpdated(call ToolCall) {
	s.live.ToolUpdate(call.ID, call.Status, call.SizeDelta)
}

func (s *streamEventSink) Activity() {}

func (s *streamEventSink) RawStreamMessage(raw []byte) {}

// classifyOutcome maps a claude-acp turn error into the neutral Outcome the
// runtime switches on (ADR-0036 D1/D4). This is the ONLY place the ACP string /
// JSON-RPC matchers (IsLoadSessionResourceNotFound, IsMaxTurnsError,
// IsPeerClosedError, ErrLivenessTimeout) are consumed to name a failure — the
// runtime never string-matches. The classes are mutually exclusive in practice;
// order is specific-before-general for defensiveness.
func classifyOutcome(err error) Outcome {
	switch {
	case err == nil:
		return Outcome{Class: Success}
	case IsLoadSessionResourceNotFound(err):
		return Outcome{Class: StaleSession, Err: err}
	case IsQuotaExhausted(err):
		return Outcome{Class: QuotaExhausted, Err: err}
	case IsMaxTurnsError(err):
		return Outcome{Class: TurnCapReached, Err: err}
	case errors.Is(err, ErrLivenessTimeout):
		return Outcome{Class: LivenessTimeout, Err: err}
	case IsPeerClosedError(err):
		return Outcome{Class: PeerDied, Err: err}
	default:
		return Outcome{Class: Other, Err: err}
	}
}

// steeringToSessionMeta translates the neutral SteeringSpec into the claude-acp
// _meta.claudeCode.options bag (ADR-0036 D1). It reconstructs the depth budget
// from the spec's Effort/Thinking and the TurnBudget's MaxTurns, then defers to
// the shared buildSessionMetaFromBudget so the bag is byte-identical to
// BuildSessionMeta's — the wire encoding that used to leak into the runtime is
// now sealed inside this adapter.
func steeringToSessionMeta(spec SteeringSpec, maxTurns *int) map[string]any {
	budget := depthBudget{
		maxTurns: maxTurns,
		thinking: spec.Thinking.toPtr(),
		effort:   spec.Effort,
	}
	return buildSessionMetaFromBudget(spec.SystemPrompt, budget, spec.Model, spec.AuditRawStream, spec.Isolation.SettingSources)
}

// claudeACPRunner is the claude-agent-acp adapter's Runner (ADR-0036 D1). It
// wraps the existing ACP turn engine: Preflight is the ACP initialize handshake,
// and Open returns a Session backed by a TurnRunner — RunTurnWithOptions on the
// daemonless one-shot path, or a warm AgentWorker on the daemon path. The
// per-Ask session context (workspace, spawn env, stderr/live log paths) is
// stamped on after prepareSession resolves it; Preflight only needs cfg.
type claudeACPRunner struct {
	cfg     Config
	turnRun TurnRunner

	workspace string
	spawnEnv  []string
	stderrLog string
	livePath  string
}

// Preflight verifies the runtime can serve a turn (ACP initialize over stdio)
// without running a prompt. It replaces the inline CheckACPHandshake gate.
func (r *claudeACPRunner) Preflight(ctx context.Context) error {
	return checkACPHandshake(ctx, r.cfg.AgentCmd, r.cfg.Cwd, r.cfg.Env, defaultHandshakeTimeout)
}

// Open returns a Session bound to the given ghx session identity and resume
// token. Because each RunTurn spawns/reuses its own adapter, Open is cheap: it
// stamps the per-Ask context onto a fresh session value.
func (r *claudeACPRunner) Open(_ context.Context, id SessionID, resume ResumeToken) (Session, error) {
	return &claudeACPSession{
		id:        id,
		turnRun:   r.turnRun,
		agentCmd:  r.cfg.AgentCmd,
		workspace: r.workspace,
		spawnEnv:  r.spawnEnv,
		stderrLog: r.stderrLog,
		livePath:  r.livePath,
		resume:    resume,
	}, nil
}

// Info reports the runtime identity for provenance.
func (r *claudeACPRunner) Info() RuntimeInfo {
	return RuntimeInfo{Kind: "claude-acp", Name: r.cfg.AgentCmd}
}

// claudeACPSession is one warm-or-one-shot ACP conversation behind the port.
// Turn translates the neutral TurnRequest into RunTurnOptions (steering encoded
// via steeringToSessionMeta), runs the wrapped TurnRunner, refreshes the opaque
// resume token, and classifies the result into a typed Outcome.
type claudeACPSession struct {
	id      SessionID
	turnRun TurnRunner

	agentCmd  string
	workspace string
	spawnEnv  []string
	stderrLog string
	livePath  string

	// resume is the opaque ACP session id carried across this session's turns.
	// It updates to the id the runtime last established (non-empty), mirroring
	// the former newSessionID-else-acpSessionID fallback.
	resume ResumeToken
	// cachedMeta is the steering bag, built once and reused so a session's
	// turns (primary, D1 wrap-up, report retries) re-assert byte-identical
	// steering (ADR-0020.2 / TRUST H8).
	cachedMeta map[string]any
}

// Turn runs one prompt, streams to sink, and returns (result, refreshed resume,
// typed outcome). It builds RunTurnOptions byte-identically to the former
// buildTurnOptions and dispatches to the wrapped TurnRunner (the stubbable
// runTurnWithOptions var, or a warm worker).
func (s *claudeACPSession) Turn(ctx context.Context, req TurnRequest, sink EventSink) (TurnResult, ResumeToken, Outcome) {
	opts := RunTurnOptions{
		AgentCmd:        s.agentCmd,
		ACPSessionID:    string(s.resume),
		Prompt:          req.Prompt,
		Cwd:             s.workspace,
		Env:             s.spawnEnv,
		SessionMeta:     s.sessionMeta(req),
		ReportSinkPath:  req.Steering.ReportSink.Path,
		AgentStderrPath: s.stderrLog,
		LiveLogPath:     s.livePath,
		LivenessTimeout: req.Budget.Liveness,
		sink:            sink,
	}
	result, sessionID, err := s.turnRun(ctx, opts)
	if sessionID != "" {
		s.resume = ResumeToken(sessionID)
	}
	return result, ResumeToken(sessionID), classifyOutcome(err)
}

// Close releases the session's resources. The warm/one-shot ACP process
// lifetime is owned by the TurnRunner (the AgentWorker pool or RunTurnWithOptions
// per call), so there is nothing session-scoped to release here.
func (s *claudeACPSession) Close() {}

// sessionMeta builds (once) and caches the steering bag for this session.
func (s *claudeACPSession) sessionMeta(req TurnRequest) map[string]any {
	if s.cachedMeta == nil {
		s.cachedMeta = steeringToSessionMeta(req.Steering, req.Budget.MaxTurns)
	}
	return s.cachedMeta
}
