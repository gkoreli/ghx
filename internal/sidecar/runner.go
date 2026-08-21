package sidecar

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// The Runner port (ADR-0036 D1). This is the sidecar's agentic-runtime seam:
// a small, consumer-defined interface set so the turn engine is one adapter
// behind an interface, selectable by config. Phase C1 defines the port IN the
// consumer package `sidecar` and makes today's claude-agent-acp path implement
// it (see acprunner.go); it is behavior-identical. Later phases lift the port
// to a leaf package and add codex/SDK adapters.
//
// The types here carry NO acp types and NO subprocess fields — that is what
// makes them the neutral boundary a second adapter can implement. The
// measurement-facing result (TurnResult/ToolCallTrace, turnresult.go, which
// import only `time`) is already neutral, so the port reuses it unchanged.

// Runner turns a built prompt into bounded agentic turns behind an interface.
// Implementations wrap a concrete runtime — claude-agent-acp today, codex-acp
// or an in-process Claude Agents SDK loop later. The runtime layer (Ask, the
// daemon pool) depends only on this interface: no method exposes a subprocess,
// a wire connection, or an ACP type.
type Runner interface {
	// Preflight verifies the runtime can serve a turn without running a prompt
	// (for claude-acp: the binary is present and speaks ACP initialize). It
	// replaces the ACP-handshake gate that used to live inline in Ask.
	Preflight(ctx context.Context) error

	// Open returns a Session for a ghx session identity. resume is the opaque
	// token persisted from a prior turn (zero on first use); the adapter decides
	// whether it can honor it. Because the durable ledger rides in the prompt, an
	// adapter that cannot resume across processes still returns a correct Session.
	Open(ctx context.Context, id SessionID, resume ResumeToken) (Session, error)

	// Info reports the runtime identity for provenance/artifacts.
	Info() RuntimeInfo
}

// Session is one live agentic conversation. Turns are serialized by the caller
// (one ghx session == one Session); the adapter owns whatever process or client
// backs it. Close releases those resources.
type Session interface {
	// Turn runs one prompt to completion, streaming activity to sink, and
	// returns the neutral result, a refreshed ResumeToken, and a typed Outcome.
	// It never returns an error the runtime must string-match: every failure is
	// a classified Outcome (see FailureClass).
	Turn(ctx context.Context, req TurnRequest, sink EventSink) (TurnResult, ResumeToken, Outcome)

	// Close releases the session's resources.
	Close()
}

// TurnRequest is one prompt turn with NO ACP types and NO subprocess fields.
type TurnRequest struct {
	// Prompt is the fully built user prompt (BuildPrompt output).
	Prompt string
	// Steering is how the runtime must behave this turn.
	Steering SteeringSpec
	// Budget carries the mechanical caps + watchdog window.
	Budget TurnBudget
}

// SteeringSpec is the neutral steering surface: WHAT the runtime must do, not
// HOW an adapter encodes it. The claude-acp adapter translates it into
// _meta.claudeCode.options via the (unchanged) BuildSessionMeta; codex/SDK
// adapters would translate it into their own channel. A field an adapter cannot
// honor must degrade LOUDLY, never silently (Visibility & Truthfulness).
type SteeringSpec struct {
	// SystemPrompt is the persona/doctrine (cache-stable on runtimes that
	// support a system-prompt channel).
	SystemPrompt string
	// Isolation is the host-settings/host-MCP posture.
	Isolation Isolation
	// ToolPolicy is the reconnaissance tool policy (allow shell+read, deny
	// writes, auto-approve the report sink).
	ToolPolicy ToolPolicy
	// Model pins the subject model ("" = runtime default).
	Model string
	// Effort is the mapped effort tier ("low"|"medium"|"high"), not a raw SDK
	// enum.
	Effort string
	// Thinking is Off or a token budget.
	Thinking ThinkingBudget
	// ReportSink is the submit_report contract: sink path + tool id.
	ReportSink ReportSink
	// AuditRawStream captures the provider-native token/tool stream (eval-only).
	AuditRawStream bool
}

// TurnBudget carries the mechanical caps. MaxTurns is a hint the adapter maps
// to its own safety net; Liveness is the runtime-owned watchdog window
// (0 = default, negative = disabled).
type TurnBudget struct {
	MaxTurns *int
	Liveness time.Duration
}

// Isolation is the neutral host-isolation posture. SettingSources mirrors the
// claude-acp settingSources semantics: nil means the default full-isolation
// posture ([] on the wire, no host CLAUDE.md/MCP), a non-nil slice opts into
// the named host setting sources.
type Isolation struct {
	SettingSources []string
}

// ToolPolicy is the neutral reconnaissance tool policy. Its C1 value is fixed
// (reconToolPolicy): the claude-acp adapter enforces it via BuildSessionMeta's
// tools allowlist + allowedTools bypass and the denyClient permission gate.
type ToolPolicy struct {
	// AllowReconShell allows recon shell execution (ghx) and reads.
	AllowReconShell bool
	// DenyWrites rejects write/edit/delete/unknown tool kinds.
	DenyWrites bool
	// AutoApproveReportSink auto-approves the session-scoped report-sink tool.
	AutoApproveReportSink bool
}

// reconToolPolicy is the fixed sidecar recon policy for C1.
var reconToolPolicy = ToolPolicy{AllowReconShell: true, DenyWrites: true, AutoApproveReportSink: true}

// ThinkingBudget is the neutral thinking config. Set mirrors the claude-acp
// depthBudget.thinking pointer being non-nil; Tokens is the budget (0 disables
// thinking, >0 enables it with that budget).
type ThinkingBudget struct {
	Set    bool
	Tokens int
}

// thinkingBudgetFromPtr maps the adapter's *int budget into a neutral
// ThinkingBudget (nil = unset).
func thinkingBudgetFromPtr(p *int) ThinkingBudget {
	if p == nil {
		return ThinkingBudget{}
	}
	return ThinkingBudget{Set: true, Tokens: *p}
}

// toPtr maps a neutral ThinkingBudget back to the adapter's *int budget.
func (t ThinkingBudget) toPtr() *int {
	if !t.Set {
		return nil
	}
	v := t.Tokens
	return &v
}

// ReportSink is the neutral submit_report contract: where the runtime reads the
// accepted report back (Path) and the fully-qualified tool id the model sees.
type ReportSink struct {
	Path   string
	ToolID string
}

// ResumeToken is an opaque, adapter-defined resume handle (an ACP session id
// for claude-acp; an SDK session id or "" for other adapters). The runtime
// never inspects it.
type ResumeToken string

// SessionID is a ghx session identity (the resolved session name).
type SessionID string

// RuntimeInfo reports a runtime's identity for provenance/artifacts.
type RuntimeInfo struct {
	Kind    string
	Name    string
	Version string
}

// EventSink receives streamed turn activity as neutral calls. The adapter
// decodes its native stream (ACP SessionNotification for claude-acp) into these;
// the sink implementation is the only place that knows about stdout, live.jsonl,
// and the tool-progress line. This is denyClient.SessionUpdate inverted: the
// classification stays with the adapter, the side effects move behind the sink.
type EventSink interface {
	// Text streams an assistant message delta.
	Text(delta string)
	// Thinking streams a reasoning delta.
	Thinking(delta string)
	// ToolStarted reports a tool call began.
	ToolStarted(call ToolCall)
	// ToolUpdated reports a status/output update for a tool call.
	ToolUpdated(call ToolCall)
	// Activity is a liveness heartbeat (every event implies one). In C1 the
	// watchdog stays adapter-internal (fed by denyClient.onActivity); this is the
	// neutral heartbeat a future runtime-owned watchdog wraps.
	Activity()
	// RawStreamMessage carries a provider-native message (eval-only: usage, raw
	// tool blocks). In C1 the claude-acp raw-SDK audit stays in
	// denyClient.HandleExtensionMethod; this is the neutral hook for it.
	RawStreamMessage(raw []byte)
}

// ToolCall is the neutral payload for ToolStarted/ToolUpdated. Summary is the
// adapter-computed human tool-progress line; SizeDelta is the incremental
// output size an update added.
type ToolCall struct {
	ID        string
	Title     string
	Kind      string
	Status    string
	Summary   string
	SizeDelta int
}

// Outcome is the typed failure classification the runtime acts on instead of
// matching adapter error strings. Every adapter maps its native failure into
// exactly one class, so the recovery policy and the eval-anomaly contract run
// on a stable vocabulary regardless of runtime.
type Outcome struct {
	Class FailureClass
	// Err is the underlying error, for artifacts/logs only — the runtime never
	// string-matches it (it switches on Class).
	Err error
}

// FailureClass is the turn/transport fault taxonomy (ADR-0036 D4). It is the
// axis BELOW the eval-anomaly marker string: the adapter returns the class, the
// runtime writes the byte-identical marker keyed by it (see canonicalTurnError),
// so evals/anomalies.go never learns which runtime ran.
type FailureClass int

const (
	// Success is a completed turn (Err nil).
	Success FailureClass = iota
	// TurnCapReached is the runtime's max-turns net firing -> D1 wrap-up resume.
	TurnCapReached
	// LivenessTimeout is no activity for the watchdog window -> episode_hang_timeout.
	LivenessTimeout
	// PeerDied is the backing process/connection dying -> D2 fail-fast.
	PeerDied
	// StaleSession is a resume token no longer valid -> recreate a fresh session.
	StaleSession
	// QuotaExhausted is the backend out of quota/session budget (ADR-0040 L3
	// trigger) -> the runtime's degrade-don't-die ladder: cheaper-backend
	// retry, cached-ledger report, else typed failure with artifacts kept.
	QuotaExhausted
	// Other is anything else -> fail the turn with the underlying Err.
	Other
)

// markerForFailureClass returns the canonical eval-anomaly marker string the
// runtime writes into a failed turn's error for a given class (ADR-0036 D4;
// audit L9). These strings are the frozen measurement contract read by
// evals/anomalies.go; they MUST stay byte-identical across runtimes. Classes
// with no anomaly-contract marker return "".
func markerForFailureClass(c FailureClass) string {
	switch c {
	case LivenessTimeout:
		return LivenessTimeoutMarker
	case TurnCapReached:
		return maxTurnsErrorMarker
	case PeerDied:
		return peerClosedMarker
	default:
		return ""
	}
}

// canonicalTurnError is the runtime's failed-turn error: it guarantees the
// canonical FailureClass marker is present in the persisted error string,
// regardless of what the adapter's underlying error happened to say (ADR-0036
// D4). When the adapter already surfaced the marker — today's claude-acp path
// always does, since it builds the marker into its own error — the underlying
// error is returned byte-identical; otherwise the runtime prepends the marker so
// a future adapter cannot silently break the eval-anomaly contract.
func canonicalTurnError(o Outcome) error {
	if o.Err == nil {
		return nil
	}
	marker := markerForFailureClass(o.Class)
	if marker == "" || strings.Contains(o.Err.Error(), marker) {
		return o.Err
	}
	return fmt.Errorf("%s: %w", marker, o.Err)
}
