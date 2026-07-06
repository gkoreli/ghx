package sidecar

import (
	"fmt"
	"path/filepath"
)

// ArtifactsRef points a caller at the persisted audit trail of one Ask: the
// session directory holding the OTLP artifact set (traces.jsonl, logs.jsonl,
// metrics.jsonl, reports/) plus, when trace emission succeeded, the hex ID of
// the ask's root trace (the `sidecar.ask` span). It is Mastra's "traceId
// inside the return value" idea (ADR-0026) done at file level — the ghx
// differentiator: the parent agent gets a concrete directory it can open,
// replay (`ghx sidecar view`), or hand to a judge, never a location it has to
// guess. Every ask response surface (CLI human output, --json envelope, MCP
// recon text) carries this ref.
type ArtifactsRef struct {
	// SessionDir is the absolute path to the session's artifact directory.
	SessionDir string `json:"sessionDir"`
	// TraceID is the 32-char hex trace ID of the ask's root span, empty when
	// trace emission failed (emission is best-effort and never fails the ask).
	TraceID string `json:"traceId,omitempty"`
}

// newArtifactsRef builds the ref for one session, resolving the directory to
// an absolute path best-effort (the default ~/.ghx root is already absolute;
// a relative $GHX_HOME is anchored at the current working directory).
func newArtifactsRef(sessionsDir, session string) ArtifactsRef {
	dir := sessionDir(sessionsDir, session)
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return ArtifactsRef{SessionDir: dir}
}

// FooterLine renders the one-line artifacts pointer appended to human-readable
// ask responses: "artifacts: <dir>", plus the root trace ID when available.
// Empty when the ref itself is empty (e.g. the ask failed before a session
// directory existed).
func (a ArtifactsRef) FooterLine() string {
	if a.SessionDir == "" {
		return ""
	}
	if a.TraceID == "" {
		return "artifacts: " + a.SessionDir
	}
	return fmt.Sprintf("artifacts: %s (trace %s)", a.SessionDir, a.TraceID)
}
