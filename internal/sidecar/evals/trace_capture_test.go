package evals

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// decodedInput mirrors how ACP rawInput reaches a captured trace: JSON
// decoded into any (the digest canonicalization both sides rely on).
func decodedInput(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// rawUse builds one raw-SDK tool use whose digest matches inputJSON.
func rawUse(t *testing.T, id, name, inputJSON string) sidecar.RawSDKToolUse {
	t.Helper()
	return sidecar.RawSDKToolUse{
		ID:          id,
		Name:        name,
		InputDigest: sidecar.CanonicalJSONDigest(decodedInput(t, inputJSON)),
	}
}

// capturedTrace builds one captured trace with a terminal status and output.
func capturedTrace(t *testing.T, id, inputJSON string) ToolCallTrace {
	t.Helper()
	return ToolCallTrace{
		ID:       id,
		Kind:     "execute",
		RawInput: decodedInput(t, inputJSON),
		StatusTransitions: []ToolStatusTransition{
			{Status: "pending", At: time.Now().UTC()},
			{Status: "completed", At: time.Now().UTC()},
		},
		OutputSize:    12,
		OutputExcerpt: "some output",
	}
}

// TestCompareTraceCaptureMatrix covers the pre-registered comparator matrix
// (ADR-0016.10 D3): complete capture, each of the four gap conditions, the
// suppressed-tool exemption, and the skip rules.
func TestCompareTraceCaptureMatrix(t *testing.T) {
	const inputA = `{"command":"ghx tree gin-gonic/gin"}`
	const inputB = `{"command":"ghx read gin-gonic/gin gin.go"}`

	tests := []struct {
		name     string
		raw      *sidecar.RawSDKAudit
		captured []ToolCallTrace
		want     map[string]int // gap kind -> count
	}{
		{
			name: "complete capture yields no gaps",
			raw: &sidecar.RawSDKAudit{
				Messages:    3,
				ToolUses:    []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", inputA)},
				ToolResults: []sidecar.RawSDKToolResult{{ToolUseID: "toolu_1", OutputSize: 12}},
			},
			captured: []ToolCallTrace{capturedTrace(t, "toolu_1", inputA)},
			want:     nil,
		},
		{
			name: "raw tool_use missing from capture",
			raw: &sidecar.RawSDKAudit{
				Messages: 2,
				ToolUses: []sidecar.RawSDKToolUse{
					rawUse(t, "toolu_1", "Bash", inputA),
					rawUse(t, "toolu_2", "Bash", inputB),
				},
			},
			captured: []ToolCallTrace{capturedTrace(t, "toolu_1", inputA)},
			want:     map[string]int{GapMissingToolCall: 1},
		},
		{
			name: "raw terminal result but captured trace stuck pending",
			raw: &sidecar.RawSDKAudit{
				Messages:    2,
				ToolUses:    []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", inputA)},
				ToolResults: []sidecar.RawSDKToolResult{{ToolUseID: "toolu_1", OutputSize: 12}},
			},
			captured: []ToolCallTrace{{
				ID:                "toolu_1",
				RawInput:          decodedInput(t, inputA),
				StatusTransitions: []ToolStatusTransition{{Status: "pending", At: time.Now().UTC()}},
				OutputSize:        12,
				OutputExcerpt:     "x",
			}},
			want: map[string]int{GapMissingTerminalStatus: 1},
		},
		{
			name: "captured input differs from raw input",
			raw: &sidecar.RawSDKAudit{
				Messages: 1,
				ToolUses: []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", inputA)},
			},
			captured: []ToolCallTrace{capturedTrace(t, "toolu_1", inputB)},
			want:     map[string]int{GapInputMismatch: 1},
		},
		{
			name: "raw output present but capture recorded none",
			raw: &sidecar.RawSDKAudit{
				Messages:    2,
				ToolUses:    []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", inputA)},
				ToolResults: []sidecar.RawSDKToolResult{{ToolUseID: "toolu_1", OutputSize: 4096}},
			},
			captured: []ToolCallTrace{{
				ID:       "toolu_1",
				RawInput: decodedInput(t, inputA),
				StatusTransitions: []ToolStatusTransition{
					{Status: "pending", At: time.Now().UTC()},
					{Status: "completed", At: time.Now().UTC()},
				},
			}},
			want: map[string]int{GapMissingOutput: 1},
		},
		{
			name: "adapter-suppressed tools are exempt",
			raw: &sidecar.RawSDKAudit{
				Messages: 2,
				ToolUses: []sidecar.RawSDKToolUse{
					rawUse(t, "toolu_todo", "TodoWrite", `{"todos":[]}`),
					rawUse(t, "toolu_task", "TaskCreate", `{"title":"x"}`),
				},
				ToolResults: []sidecar.RawSDKToolResult{{ToolUseID: "toolu_todo", OutputSize: 9}},
			},
			captured: nil,
			want:     nil,
		},
		{
			name:     "nil raw audit skips (pre-ADR artifacts)",
			raw:      nil,
			captured: []ToolCallTrace{capturedTrace(t, "toolu_1", inputA)},
			want:     nil,
		},
		{
			name:     "zero live messages skips (channel never opened)",
			raw:      &sidecar.RawSDKAudit{},
			captured: []ToolCallTrace{capturedTrace(t, "toolu_1", inputA)},
			want:     nil,
		},
		{
			name: "extra captured traces are not a gap (no overcount direction)",
			raw: &sidecar.RawSDKAudit{
				Messages: 1,
				ToolUses: []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", inputA)},
			},
			captured: []ToolCallTrace{
				capturedTrace(t, "toolu_1", inputA),
				capturedTrace(t, "toolu_retry", inputB), // merged retry trace
			},
			want: nil,
		},
		{
			name: "truncated audit compares only present events",
			raw: &sidecar.RawSDKAudit{
				Messages:  5,
				ToolUses:  []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", inputA)},
				Truncated: true,
			},
			captured: []ToolCallTrace{capturedTrace(t, "toolu_1", inputA)},
			want:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gaps := CompareTraceCapture(tc.raw, tc.captured)
			got := map[string]int{}
			for _, g := range gaps {
				got[g.Kind]++
			}
			if len(got) != len(tc.want) {
				t.Fatalf("gaps = %+v, want kinds %+v", gaps, tc.want)
			}
			for kind, n := range tc.want {
				if got[kind] != n {
					t.Errorf("gap %s = %d, want %d (all: %+v)", kind, got[kind], n, gaps)
				}
			}
		})
	}
}

// TestTraceCaptureGapAnomalyDerivation verifies DetectAnomalies raises one
// soft trace_capture_gap per gapped turn and none on unregistered artifacts.
func TestTraceCaptureGapAnomalyDerivation(t *testing.T) {
	const input = `{"command":"ghx tree x/y"}`
	gapped := &Episode{
		ID: "e1", TaskID: "t1", Profile: ProfilePlain,
		Turns: []TurnRecord{
			{
				Turn: 0,
				RawSDK: &sidecar.RawSDKAudit{
					Messages: 2,
					ToolUses: []sidecar.RawSDKToolUse{
						rawUse(t, "toolu_seen", "Bash", input),
						rawUse(t, "toolu_dropped", "Bash", input),
					},
				},
				ToolTraces: []ToolCallTrace{capturedTrace(t, "toolu_seen", input)},
			},
			{Turn: 1}, // no raw audit — must derive nothing
		},
	}

	var found []Anomaly
	for _, a := range DetectAnomalies(gapped) {
		if a.Kind == AnomalyTraceCaptureGap {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		t.Fatalf("trace_capture_gap anomalies = %+v, want exactly 1", found)
	}
	if found[0].Severity != SeveritySoft || found[0].Turn != 0 {
		t.Errorf("anomaly = %+v, want soft on turn 0", found[0])
	}
	if found[0].Detail == "" {
		t.Error("anomaly detail must summarize the gaps")
	}

	// Pre-ADR artifact: no rawSDK field anywhere — zero anomalies of this kind.
	old := &Episode{
		ID: "e2", TaskID: "t1", Profile: ProfileSidecar,
		Turns: []TurnRecord{{Turn: 0, ToolTraces: []ToolCallTrace{capturedTrace(t, "toolu_1", input)}}},
	}
	for _, a := range DetectAnomalies(old) {
		if a.Kind == AnomalyTraceCaptureGap {
			t.Fatalf("pre-ADR artifact derived %+v; detector must no-op without rawSDK", a)
		}
	}

	// CountAnomalies includes the kind in the stable order.
	counts := CountAnomalies([]*Episode{gapped, old})
	var seen bool
	for _, c := range counts {
		if c.Kind == AnomalyTraceCaptureGap {
			seen = true
			if c.Severity != SeveritySoft || c.Count != 1 || c.Episodes != 1 {
				t.Errorf("count = %+v, want soft/1/1", c)
			}
		}
	}
	if !seen {
		t.Error("CountAnomalies missing trace_capture_gap")
	}
}

// TestRawSDKEpisodePersistenceRoundTrip verifies the rawSDK turn field and
// derived anomalies survive the canonical episode JSON round trip.
func TestRawSDKEpisodePersistenceRoundTrip(t *testing.T) {
	const input = `{"command":"ghx tree x/y"}`
	ep := &Episode{
		ID: "e1", TaskID: "t1", Profile: ProfileSidecar,
		Turns: []TurnRecord{{
			Turn: 0,
			RawSDK: &sidecar.RawSDKAudit{
				Messages:    3,
				ToolUses:    []sidecar.RawSDKToolUse{rawUse(t, "toolu_1", "Bash", input)},
				ToolResults: []sidecar.RawSDKToolResult{{ToolUseID: "toolu_1", OutputSize: 12}},
				Usage:       &sidecar.RawSDKUsage{InputTokens: 900, OutputTokens: 80, CostUSD: 0.01, Source: "result"},
			},
			ToolTraces: []ToolCallTrace{capturedTrace(t, "toolu_1", input)},
		}},
	}
	ep.Anomalies = DetectAnomalies(ep)

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatal(err)
	}
	var back Episode
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	raw := back.Turns[0].RawSDK
	if raw == nil || raw.Messages != 3 || len(raw.ToolUses) != 1 || len(raw.ToolResults) != 1 {
		t.Fatalf("round trip lost rawSDK: %+v", raw)
	}
	if raw.Usage == nil || raw.Usage.InputTokens != 900 || raw.Usage.Source != "result" {
		t.Errorf("round trip lost usage: %+v", raw.Usage)
	}
	// Re-derivation invariant (ADR-0016.8 D6): anomalies re-derived from the
	// reloaded artifact equal the stored field.
	rederived := DetectAnomalies(&back)
	if len(rederived) != len(back.Anomalies) {
		t.Errorf("re-derived anomalies = %+v, stored = %+v", rederived, back.Anomalies)
	}
}
