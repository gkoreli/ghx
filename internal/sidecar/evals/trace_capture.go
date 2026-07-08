package evals

import (
	"fmt"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// Trace-capture comparator (ADR-0016.10 D3, TRUST H3): diffs the raw-SDK
// tool blocks the provider actually emitted against the ToolTraces the ACP
// notification path captured. Pure over episode-artifact fields only, so
// any saved episode can be re-analyzed offline (ADR-0016.7 taxonomy rule).

// Trace-capture gap kinds — exactly the four pre-registered conditions
// (ADR-0016.10 D3). A run scored under that ADR may not add or remove
// conditions without a superseding registration.
const (
	// GapMissingToolCall: a raw tool_use block has no captured trace with
	// the same ID (the adapter reuses the SDK tool_use id verbatim as the
	// ACP toolCallId, so ID equality is exact).
	GapMissingToolCall = "missing_tool_call"
	// GapMissingTerminalStatus: a raw tool_result exists but the captured
	// trace never recorded a terminal (completed/failed) status.
	GapMissingTerminalStatus = "missing_terminal_status"
	// GapInputMismatch: both sides carry non-empty input and the canonical
	// JSON digests differ.
	GapInputMismatch = "input_mismatch"
	// GapMissingOutput: the raw tool_result carried non-empty output but the
	// captured trace recorded neither output size nor an excerpt.
	GapMissingOutput = "missing_output"
)

// suppressedRawTools are tool names the adapter never surfaces as ACP
// tool_calls by design (claude-agent-acp shouldEmitToolCall/isTaskTool:
// TodoWrite renders as a plan update; Task* is suppressed until
// tool_result). Their raw tool_uses are exempt from the missing-call check.
var suppressedRawTools = map[string]bool{
	"TodoWrite":  true,
	"TaskCreate": true,
	"TaskUpdate": true,
	"TaskList":   true,
	"TaskGet":    true,
}

// TraceCaptureGap is one detected divergence between the raw SDK stream and
// the captured tool traces.
type TraceCaptureGap struct {
	// Kind is one of the Gap* constants.
	Kind string `json:"kind"`
	// ToolUseID is the SDK tool_use id (== ACP toolCallId) the gap concerns.
	ToolUseID string `json:"toolUseId"`
	// Detail is a bounded human-readable explanation.
	Detail string `json:"detail,omitempty"`
}

// CompareTraceCapture diffs one turn's raw-SDK audit against its captured
// live tool traces and returns the pre-registered gaps (ADR-0016.10 D3).
//
// Skip rules (pre-registered): a nil audit or one that observed zero live
// messages means the audit channel never opened — absence of the stream is
// never evidence of a gap, so the comparator returns nothing. A truncated
// audit is compared only over the events present. Captured traces absent
// from the raw audit are deliberately NOT a gap (no overcount direction):
// H3 is about undercounting, and merged retry/wrap-up traces would make the
// reverse direction structurally noisy.
func CompareTraceCapture(raw *sidecar.RawSDKAudit, captured []sidecar.ToolCallTrace) []TraceCaptureGap {
	if raw == nil || raw.Messages == 0 {
		return nil
	}

	capturedByID := make(map[string]*sidecar.ToolCallTrace, len(captured))
	for i := range captured {
		capturedByID[captured[i].ID] = &captured[i]
	}
	rawUseByID := make(map[string]*sidecar.RawSDKToolUse, len(raw.ToolUses))
	for i := range raw.ToolUses {
		rawUseByID[raw.ToolUses[i].ID] = &raw.ToolUses[i]
	}

	var gaps []TraceCaptureGap

	for i := range raw.ToolUses {
		use := &raw.ToolUses[i]
		if suppressedRawTools[use.Name] {
			continue // adapter-by-design suppression, not a capture gap
		}
		trace, ok := capturedByID[use.ID]
		if !ok {
			gaps = append(gaps, TraceCaptureGap{
				Kind:      GapMissingToolCall,
				ToolUseID: use.ID,
				Detail:    fmt.Sprintf("raw tool_use %q (%s) has no captured tool trace", use.ID, use.Name),
			})
			continue
		}
		if use.InputDigest != "" {
			if capturedDigest := sidecar.CanonicalJSONDigest(trace.RawInput); capturedDigest != "" && capturedDigest != use.InputDigest {
				gaps = append(gaps, TraceCaptureGap{
					Kind:      GapInputMismatch,
					ToolUseID: use.ID,
					Detail:    fmt.Sprintf("captured rawInput digest %.12s differs from raw SDK input digest %.12s", capturedDigest, use.InputDigest),
				})
			}
		}
	}

	for _, res := range raw.ToolResults {
		use := rawUseByID[res.ToolUseID]
		if use != nil && suppressedRawTools[use.Name] {
			continue
		}
		trace, ok := capturedByID[res.ToolUseID]
		if !ok {
			continue // already reported as missing_tool_call (or an exempt tool)
		}
		if !hasTerminalStatus(trace) {
			gaps = append(gaps, TraceCaptureGap{
				Kind:      GapMissingTerminalStatus,
				ToolUseID: res.ToolUseID,
				Detail:    fmt.Sprintf("raw tool_result exists for %q but the captured trace never reached completed/failed", res.ToolUseID),
			})
		}
		if res.OutputSize > 0 && trace.OutputSize == 0 && trace.OutputExcerpt == "" {
			gaps = append(gaps, TraceCaptureGap{
				Kind:      GapMissingOutput,
				ToolUseID: res.ToolUseID,
				Detail:    fmt.Sprintf("raw tool_result carried %d output chars for %q but the captured trace recorded none", res.OutputSize, res.ToolUseID),
			})
		}
	}

	return gaps
}

// hasTerminalStatus reports whether a captured trace recorded a terminal
// ACP tool-call status.
func hasTerminalStatus(tr *sidecar.ToolCallTrace) bool {
	for _, st := range tr.StatusTransitions {
		if st.Status == "completed" || st.Status == "failed" {
			return true
		}
	}
	return false
}

// summarizeTraceCaptureGaps renders one bounded detail string for the
// per-turn trace_capture_gap anomaly: gap kind counts plus the first few
// tool-use IDs.
func summarizeTraceCaptureGaps(gaps []TraceCaptureGap) string {
	counts := map[string]int{}
	var ids []string
	for _, g := range gaps {
		counts[g.Kind]++
		if len(ids) < 3 {
			ids = append(ids, g.ToolUseID)
		}
	}
	detail := ""
	for _, kind := range []string{GapMissingToolCall, GapMissingTerminalStatus, GapInputMismatch, GapMissingOutput} {
		if counts[kind] == 0 {
			continue
		}
		if detail != "" {
			detail += ", "
		}
		detail += fmt.Sprintf("%s×%d", kind, counts[kind])
	}
	return fmt.Sprintf("%s (first ids: %v)", detail, ids)
}
