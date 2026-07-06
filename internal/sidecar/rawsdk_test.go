package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	acp "github.com/coder/acp-go-sdk"
)

// rawMsg wraps one SDK message JSON into a _claude/sdkMessage params payload.
func rawMsg(t *testing.T, message string) json.RawMessage {
	t.Helper()
	return json.RawMessage(`{"sessionId":"sess-1","message":` + message + `}`)
}

func assistantToolUseMsg(id, name, inputJSON string) string {
	return `{"type":"assistant","parent_tool_use_id":null,"message":{"role":"assistant","content":[{"type":"tool_use","id":"` + id + `","name":"` + name + `","input":` + inputJSON + `}],"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":5,"cache_creation_input_tokens":3}}}`
}

func userToolResultMsg(toolUseID, content string, isError bool) string {
	return fmt.Sprintf(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"content":%s,"is_error":%v}]}}`, toolUseID, content, isError)
}

// TestRawSDKAuditRecordAssistant verifies tool_use extraction: ID, name,
// canonical input digest, bounded excerpt, and assistant usage accumulation.
func TestRawSDKAuditRecordAssistant(t *testing.T) {
	a := &RawSDKAudit{}
	a.Record(rawMsg(t, assistantToolUseMsg("toolu_1", "Bash", `{"command":"ghx tree gin-gonic/gin"}`)), false)

	if a.Messages != 1 {
		t.Fatalf("Messages = %d, want 1", a.Messages)
	}
	if len(a.ToolUses) != 1 {
		t.Fatalf("ToolUses = %d, want 1", len(a.ToolUses))
	}
	use := a.ToolUses[0]
	if use.ID != "toolu_1" || use.Name != "Bash" {
		t.Errorf("tool use = %+v, want id toolu_1 name Bash", use)
	}
	// The digest must match the canonical digest of the same input decoded
	// the way ACP rawInput is decoded (JSON → any → sorted marshal).
	var decoded any
	if err := json.Unmarshal([]byte(`{"command":"ghx tree gin-gonic/gin"}`), &decoded); err != nil {
		t.Fatal(err)
	}
	if want := CanonicalJSONDigest(decoded); use.InputDigest != want {
		t.Errorf("InputDigest = %s, want %s (canonical round-trip digest)", use.InputDigest, want)
	}
	if use.InputExcerpt == "" || !strings.Contains(use.InputExcerpt, "ghx tree") {
		t.Errorf("InputExcerpt = %q, want the input JSON", use.InputExcerpt)
	}
	if a.Usage == nil || a.Usage.Source != "assistant_sum" {
		t.Fatalf("Usage = %+v, want assistant_sum source", a.Usage)
	}
	if a.Usage.InputTokens != 100 || a.Usage.OutputTokens != 20 {
		t.Errorf("Usage tokens = %+v, want 100/20", a.Usage)
	}
}

// TestRawSDKAuditRecordToolResult verifies tool_result extraction with both
// string content and text-block content sizing.
func TestRawSDKAuditRecordToolResult(t *testing.T) {
	a := &RawSDKAudit{}
	a.Record(rawMsg(t, userToolResultMsg("toolu_1", `"twelve chars"`, false)), false)
	a.Record(rawMsg(t, userToolResultMsg("toolu_2", `[{"type":"text","text":"abcde"},{"type":"text","text":"fgh"}]`, true)), false)

	if len(a.ToolResults) != 2 {
		t.Fatalf("ToolResults = %d, want 2", len(a.ToolResults))
	}
	if r := a.ToolResults[0]; r.ToolUseID != "toolu_1" || r.OutputSize != len("twelve chars") || r.IsError {
		t.Errorf("result[0] = %+v, want toolu_1 size %d no error", r, len("twelve chars"))
	}
	if r := a.ToolResults[1]; r.ToolUseID != "toolu_2" || r.OutputSize != 8 || !r.IsError {
		t.Errorf("result[1] = %+v, want toolu_2 size 8 error", r)
	}
}

// TestRawSDKAuditResultUsageOverwritesAssistantSum verifies the terminal
// result message pins authoritative usage (ADR-0016.10 D6).
func TestRawSDKAuditResultUsageOverwritesAssistantSum(t *testing.T) {
	a := &RawSDKAudit{}
	a.Record(rawMsg(t, assistantToolUseMsg("toolu_1", "Bash", `{"c":1}`)), false)
	a.Record(rawMsg(t, `{"type":"result","subtype":"success","usage":{"input_tokens":900,"output_tokens":80,"cache_read_input_tokens":50,"cache_creation_input_tokens":10},"total_cost_usd":0.0123}`), false)

	if a.Usage == nil || a.Usage.Source != "result" {
		t.Fatalf("Usage = %+v, want result source", a.Usage)
	}
	if a.Usage.InputTokens != 900 || a.Usage.OutputTokens != 80 || a.Usage.CostUSD != 0.0123 {
		t.Errorf("Usage = %+v, want 900/80/$0.0123", a.Usage)
	}
	// A later assistant usage must not corrupt the pinned result numbers.
	a.Record(rawMsg(t, assistantToolUseMsg("toolu_2", "Bash", `{"c":2}`)), false)
	if a.Usage.InputTokens != 900 || a.Usage.Source != "result" {
		t.Errorf("Usage after late assistant = %+v, result numbers must stay pinned", a.Usage)
	}
}

// TestRawSDKAuditIgnoresNonToolMessages verifies stream_event/system count
// toward the denominator but contribute no tool events, and that replayed
// payloads are dropped and counted.
func TestRawSDKAuditIgnoresNonToolMessages(t *testing.T) {
	a := &RawSDKAudit{}
	a.Record(rawMsg(t, `{"type":"system","subtype":"init"}`), false)
	a.Record(rawMsg(t, `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"tool_use","id":"toolu_x","name":"Bash"}}}`), false)
	a.Record(rawMsg(t, assistantToolUseMsg("toolu_1", "Bash", `{"c":1}`)), true) // replayed

	if a.Messages != 2 {
		t.Errorf("Messages = %d, want 2 (replayed excluded)", a.Messages)
	}
	if a.ReplayedMessages != 1 {
		t.Errorf("ReplayedMessages = %d, want 1", a.ReplayedMessages)
	}
	if len(a.ToolUses) != 0 {
		t.Errorf("ToolUses = %d, want 0 (stream_event tool blocks must not double-count)", len(a.ToolUses))
	}
}

// TestRawSDKAuditSizeBounds verifies the pre-registered bounds: the input
// excerpt cap and the per-turn event cap with truncation accounting.
func TestRawSDKAuditSizeBounds(t *testing.T) {
	a := &RawSDKAudit{}
	huge := strings.Repeat("x", rawSDKInputExcerptMax*2)
	a.Record(rawMsg(t, assistantToolUseMsg("toolu_big", "Bash", `{"command":"`+huge+`"}`)), false)
	if got := len(a.ToolUses[0].InputExcerpt); got != rawSDKInputExcerptMax {
		t.Errorf("InputExcerpt length = %d, want capped at %d", got, rawSDKInputExcerptMax)
	}
	if a.ToolUses[0].InputDigest == "" {
		t.Error("InputDigest must be computed from the full input even when the excerpt is capped")
	}

	over := 10
	for i := 1; i < rawSDKEventCapPerTurn+over; i++ {
		a.Record(rawMsg(t, assistantToolUseMsg(fmt.Sprintf("toolu_%d", i), "Bash", `{"c":1}`)), false)
	}
	if got := len(a.ToolUses) + len(a.ToolResults); got != rawSDKEventCapPerTurn {
		t.Errorf("events = %d, want capped at %d", got, rawSDKEventCapPerTurn)
	}
	if !a.Truncated || a.DroppedEvents != over {
		t.Errorf("Truncated=%v DroppedEvents=%d, want true/%d", a.Truncated, a.DroppedEvents, over)
	}
}

// TestRawSDKAuditPersistenceRoundTrip verifies the audit survives the JSON
// round trip the episode artifact goes through.
func TestRawSDKAuditPersistenceRoundTrip(t *testing.T) {
	a := &RawSDKAudit{}
	a.Record(rawMsg(t, assistantToolUseMsg("toolu_1", "Bash", `{"command":"ghx read x"}`)), false)
	a.Record(rawMsg(t, userToolResultMsg("toolu_1", `"ok"`, false)), false)
	a.Record(rawMsg(t, `{"type":"result","subtype":"success","usage":{"input_tokens":10,"output_tokens":2},"total_cost_usd":0.001}`), false)

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var back RawSDKAudit
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Messages != a.Messages || len(back.ToolUses) != 1 || len(back.ToolResults) != 1 {
		t.Errorf("round trip lost events: %+v", back)
	}
	if back.ToolUses[0].InputDigest != a.ToolUses[0].InputDigest {
		t.Error("round trip changed the input digest")
	}
	if back.Usage == nil || back.Usage.CostUSD != 0.001 || back.Usage.Source != "result" {
		t.Errorf("round trip lost usage: %+v", back.Usage)
	}
}

// TestRawSDKAuditMerge verifies retry/wrap-up merging: counts sum, events
// append, and a terminal result usage wins over an assistant sum.
func TestRawSDKAuditMerge(t *testing.T) {
	dst := &RawSDKAudit{}
	dst.Record(rawMsg(t, assistantToolUseMsg("toolu_1", "Bash", `{"c":1}`)), false)

	src := &RawSDKAudit{}
	src.Record(rawMsg(t, assistantToolUseMsg("toolu_2", "Read", `{"c":2}`)), false)
	src.Record(rawMsg(t, `{"type":"result","subtype":"success","usage":{"input_tokens":500,"output_tokens":40},"total_cost_usd":0.02}`), false)

	dst.Merge(src)
	if dst.Messages != 3 {
		t.Errorf("Messages = %d, want 3", dst.Messages)
	}
	if len(dst.ToolUses) != 2 || dst.ToolUses[1].ID != "toolu_2" {
		t.Errorf("ToolUses = %+v, want appended toolu_2", dst.ToolUses)
	}
	if dst.Usage == nil || dst.Usage.Source != "result" || dst.Usage.InputTokens != 500 {
		t.Errorf("Usage = %+v, want the terminal result usage to win", dst.Usage)
	}
}

// TestDenyClientHandleExtensionMethod verifies the ACP wiring: raw messages
// are recorded live vs replayed, closed clients drop, and unknown extension
// methods return method-not-found.
func TestDenyClientHandleExtensionMethod(t *testing.T) {
	result := TurnResult{}
	c := &denyClient{result: &result}

	// Before markPromptSent: replayed bucket (defensive; adapter replay
	// never emits raw messages).
	if _, err := c.HandleExtensionMethod(context.Background(), RawSDKMessageMethod, rawMsg(t, assistantToolUseMsg("toolu_r", "Bash", `{"c":0}`))); err != nil {
		t.Fatal(err)
	}
	c.markPromptSent()
	if _, err := c.HandleExtensionMethod(context.Background(), RawSDKMessageMethod, rawMsg(t, assistantToolUseMsg("toolu_1", "Bash", `{"c":1}`))); err != nil {
		t.Fatal(err)
	}

	snap := c.closeAndSnapshot()
	if snap.RawSDK == nil {
		t.Fatal("RawSDK not recorded")
	}
	if snap.RawSDK.ReplayedMessages != 1 || snap.RawSDK.Messages != 1 {
		t.Errorf("replayed/live = %d/%d, want 1/1", snap.RawSDK.ReplayedMessages, snap.RawSDK.Messages)
	}
	if len(snap.RawSDK.ToolUses) != 1 || snap.RawSDK.ToolUses[0].ID != "toolu_1" {
		t.Errorf("ToolUses = %+v, want only the live toolu_1", snap.RawSDK.ToolUses)
	}

	// After close: dropped.
	if _, err := c.HandleExtensionMethod(context.Background(), RawSDKMessageMethod, rawMsg(t, assistantToolUseMsg("toolu_late", "Bash", `{"c":2}`))); err != nil {
		t.Fatal(err)
	}
	if got := len(c.result.RawSDK.ToolUses); got != 1 {
		t.Errorf("post-close ToolUses = %d, want 1 (closed client must not record)", got)
	}

	if _, err := c.HandleExtensionMethod(context.Background(), "_other/method", nil); err == nil {
		t.Error("unknown extension method must return method-not-found")
	} else {
		var reqErr *acp.RequestError
		if !acpErrorAs(err, &reqErr) {
			t.Errorf("unknown method error = %T, want *acp.RequestError", err)
		}
	}
}

// acpErrorAs is a tiny errors.As shim kept local to the test.
func acpErrorAs(err error, target **acp.RequestError) bool {
	re, ok := err.(*acp.RequestError)
	if ok {
		*target = re
	}
	return ok
}
