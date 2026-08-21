package sidecar

// Recon MCP result contract (ADR-0019.3 D2). The recon tool returns exactly
// one machine-parseable JSON object — never prose, never a suffix. Provenance
// that used to be glued onto the report as human-readable lines (route line,
// artifacts footer) now travels inside the payload as structured fields, so a
// programmatic consumer can `json.Unmarshal` every result shape (success,
// BLOCKED report, error result) without scraping text. Humans keep rich text
// on `ghx sidecar ask`; agents get bytes.

// ReconArtifacts is the audit-trail pointer inside a ReconResult: the session
// directory holding the OTLP artifact set and, when trace emission succeeded,
// the ask's root trace ID. Null (omitted) when the ask failed before a
// session directory existed.
type ReconArtifacts struct {
	// SessionDir is the absolute path to the session's artifact directory.
	SessionDir string `json:"sessionDir"`
	// TraceID is the 32-char hex trace ID of the ask's root span, empty when
	// trace emission failed (emission is best-effort and never fails the ask).
	TraceID string `json:"traceId,omitempty"`
}

// ReconResult is the single JSON object the recon MCP tool returns on every
// path. Report carries the validated evidence report unchanged (ADR-0021
// schema); Route carries the session-routing decision (ADR-0030.1 D7) when
// the ask was routed (nil for an explicit-session pin, where there is no
// cascade decision to report); Artifacts points at the on-disk audit trail.
type ReconResult struct {
	// Report is the sidecar's evidence report, schema unchanged (ADR-0021).
	Report *Report `json:"report"`
	// Route is the routing decision that placed this ask, nil when none was
	// made (explicit session, or a failure before routing).
	Route *RouteDecision `json:"route"`
	// Artifacts points at the persisted audit trail, nil when the ask failed
	// before a session directory existed.
	Artifacts *ReconArtifacts `json:"artifacts"`
}

// NewReconResult wraps one ask outcome into the recon MCP result contract:
// the report unchanged under "report", the turn's routing decision under
// "route", and the artifacts pointer under "artifacts". A nil turn (ask
// failed before a turn ran) yields nil route and artifacts.
func NewReconResult(report *Report, turn *TurnResult) ReconResult {
	res := ReconResult{Report: report}
	if turn == nil {
		return res
	}
	res.Route = turn.Route
	if a := turn.Artifacts; a.SessionDir != "" {
		res.Artifacts = &ReconArtifacts{SessionDir: a.SessionDir, TraceID: a.TraceID}
	}
	return res
}
