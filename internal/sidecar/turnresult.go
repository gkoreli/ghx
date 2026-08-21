package sidecar

import "time"

// TurnResult holds the streamed output of one ACP prompt turn.
type TurnResult struct {
	// FullText is the complete text emitted by the agent.
	FullText string
	// Thinking is the internal reasoning streamed by ACP agent_thought_chunk
	// updates during this prompt turn.
	Thinking string
	// ReplayedText is message text replayed before this turn's prompt was sent.
	// It is retained for audit and excluded from turn output/accounting.
	ReplayedText string
	// ReplayedThinking is reasoning replayed before this turn's prompt was sent.
	// It is retained for audit and excluded from live turn telemetry.
	ReplayedThinking string
	// ToolCalls lists the tool calls observed during the turn (kind: command + status).
	ToolCalls []string
	// Route is the session-routing decision that placed this ask
	// (ADR-0030.1 D7): it rides back on every response surface so the caller
	// sees where the answer came from before building on it.
	Route *RouteDecision `json:"route,omitempty"`
	// ToolTraces records full per-call audit data for eval/training artifacts.
	ToolTraces []ToolCallTrace
	// ReplayedToolTraces records tool history replayed before this turn's
	// prompt was sent. These traces are audit-only and excluded from turn
	// activity, ledger derivation, repeat-read scoring, and char accounting.
	ReplayedToolTraces []ToolCallTrace
	AgentInfo          *ImplementationInfo
	// ToolOutputChars approximates the size of tool outputs the agent
	// consumed (content blocks and raw output on tool_call/tool_call_update
	// events). Produced text alone understates context burden; this is the
	// other half.
	ToolOutputChars int
	// ReportRetried is true when the turn needed at least one corrective
	// follow-up (ADR-0016.7 / ADR-0021 D2) to obtain a usable report. Recorded
	// so evals can count retries honestly instead of hiding them.
	ReportRetried bool
	// ReportCoerced is true when the final report was obtained only via the
	// lenient <ghx-report> coercion fallback (ADR-0021 D3), i.e. the producer's
	// JSON did not fit the schema and had to be normalized. Surfaced so evals
	// count every coercion as a soft anomaly instead of silently absorbing
	// producer drift (Visibility and Truthfulness). Always false when the report
	// came from the strict submit_report sink.
	ReportCoerced bool
	// ReportBoundViolations is a one-line summary of persona compactness
	// bound breaches in the final report (ADR-0039: oversize chars,
	// relevantFiles over 5, answer sentences over 2). Empty when within
	// bounds. Flag-only — never alters the report; evals record it as a soft
	// anomaly so contract drift stays visible.
	ReportBoundViolations string
	// WrapUpRecovered is true when the turn hit the adapter's max-turns safety
	// net and the exploration was recovered by the one-shot LoadSession wrap-up
	// prompt (ADR-0027 D1). Recorded on the eval turn record and counted as the
	// soft anomaly turn_cap_wrapup so evals can see how often the net fires.
	WrapUpRecovered bool
	// SessionRecreated is true when a persisted ACP session ID was stale and
	// LoadSession returned Resource not found; Ask created one fresh ACP session
	// and continued the turn using the durable ledger context in the prompt.
	SessionRecreated bool
	// QuotaDegraded is true when the backend died of quota exhaustion and the
	// ADR-0040 L3 ladder shipped a DEGRADED report built from the session's
	// cached ledger evidence instead of failing the ask. Recorded so callers
	// (CLI notice, evals) can see the answer was NOT fresh exploration.
	QuotaDegraded bool
	// QuotaFallbackUsed names the fallback backend command line that served a
	// quota-recovered ask (ADR-0040 L3 rung 2). Empty unless Config.
	// FallbackAgentCmd was configured AND the primary backend died of quota
	// exhaustion AND the fallback turn answered — the degraded:model rung.
	FallbackBackend string
	// Artifacts points at the session's persisted audit trail for the ask this
	// turn belongs to (session dir + root trace ID). Populated by Ask after
	// artifact emission; zero for a bare RunTurnWithOptions result.
	Artifacts ArtifactsRef
	// RawSDK is the normalized raw-SDK audit for the turn (ADR-0016.10 D1),
	// captured from _claude/sdkMessage extension notifications. Nil when the
	// eval-only audit channel is off or the adapter never emitted.
	RawSDK *RawSDKAudit
}

// ImplementationInfo records the ACP adapter identity returned by initialize.
type ImplementationInfo struct {
	Name    string
	Version string
	Meta    map[string]any
}

// ToolStatusTransition records one observed ACP tool-call status.
type ToolStatusTransition struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

// ToolCallTrace is the full per-tool-call audit record captured from ACP
// tool_call and tool_call_update notifications.
type ToolCallTrace struct {
	ID       string `json:"id"`
	Kind     string `json:"kind,omitempty"`
	Title    string `json:"title,omitempty"`
	RawInput any    `json:"rawInput,omitempty"`
	// Locations lists the file paths the tool call reported touching (ACP
	// tool-call locations). Captured for path-scope decisions: the host-arm
	// write policy and the exploration/engineering attribution classifier
	// (ADR-0032.1 S2). Empty when the adapter reports no locations.
	Locations         []string               `json:"locations,omitempty"`
	StatusTransitions []ToolStatusTransition `json:"statusTransitions,omitempty"`
	OutputSize        int                    `json:"outputSize"`
	OutputExcerpt     string                 `json:"outputExcerpt,omitempty"`
}
