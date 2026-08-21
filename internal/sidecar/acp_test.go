package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func kindPtr(k acp.ToolKind) *acp.ToolKind { return &k }

func permRequest(kind *acp.ToolKind) acp.RequestPermissionRequest {
	return acp.RequestPermissionRequest{
		SessionId: "s",
		ToolCall:  acp.ToolCallUpdate{Kind: kind},
		Options: []acp.PermissionOption{
			{OptionId: "allow", Kind: acp.PermissionOptionKindAllowOnce},
			{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	}
}

func selectedOption(t *testing.T, resp acp.RequestPermissionResponse) string {
	t.Helper()
	if resp.Outcome.Selected == nil {
		t.Fatal("expected selected outcome, got cancelled")
	}
	return string(resp.Outcome.Selected.OptionId)
}

func TestRequestPermissionApprovesReadOnlyKinds(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	for _, k := range []acp.ToolKind{acp.ToolKindExecute, acp.ToolKindRead, acp.ToolKindSearch, acp.ToolKindFetch} {
		resp, err := c.RequestPermission(context.Background(), permRequest(kindPtr(k)))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "allow" {
			t.Errorf("kind %s: selected %q, want allow — the sidecar must be able to run ghx", k, got)
		}
	}
}

func TestToolSummaryFallsBackWhenRawInputEmpty(t *testing.T) {
	got := toolSummary(ToolCallTrace{
		ID:       "tool-1",
		Kind:     "execute",
		Title:    "gh api repos/o/r",
		RawInput: map[string]any{},
		StatusTransitions: []ToolStatusTransition{
			{Status: "pending"},
		},
	})
	want := "execute: gh api repos/o/r (pending)"
	if got != want {
		t.Fatalf("toolSummary = %q, want %q", got, want)
	}
}

func TestToolSummaryFallsBackToIDWhenRawInputAndTitleEmpty(t *testing.T) {
	got := toolSummary(ToolCallTrace{
		ID:       "tool-1",
		Kind:     "execute",
		RawInput: map[string]any{},
		StatusTransitions: []ToolStatusTransition{
			{Status: "pending"},
		},
	})
	want := "execute: tool-1 (pending)"
	if got != want {
		t.Fatalf("toolSummary = %q, want %q", got, want)
	}
}

func TestRequestPermissionRejectsWriteKinds(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	for _, k := range []acp.ToolKind{acp.ToolKindEdit, acp.ToolKindDelete, acp.ToolKindMove} {
		resp, err := c.RequestPermission(context.Background(), permRequest(kindPtr(k)))
		if err != nil {
			t.Fatal(err)
		}
		if got := selectedOption(t, resp); got != "reject" {
			t.Errorf("kind %s: selected %q, want reject", k, got)
		}
	}
}

func TestRequestPermissionRejectsUnknownKind(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	resp, err := c.RequestPermission(context.Background(), permRequest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := selectedOption(t, resp); got != "reject" {
		t.Errorf("nil kind: selected %q, want reject (unclassified tools are not auto-approved)", got)
	}
}

func TestRequestPermissionCancelsWithoutMatchingOption(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	req := acp.RequestPermissionRequest{
		SessionId: "s",
		ToolCall:  acp.ToolCallUpdate{Kind: kindPtr(acp.ToolKindExecute)},
		Options: []acp.PermissionOption{
			{OptionId: "reject", Kind: acp.PermissionOptionKindRejectOnce},
		},
	}
	resp, err := c.RequestPermission(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Outcome.Cancelled == nil {
		t.Error("expected cancelled outcome when no allow option exists for an approvable kind")
	}
}

func toolCallNotification(id, command string, status acp.ToolCallStatus) acp.SessionNotification {
	return acp.SessionNotification{
		SessionId: "s",
		Update: acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				ToolCallId: acp.ToolCallId(id),
				Title:      "Terminal",
				Kind:       acp.ToolKindExecute,
				Status:     status,
				RawInput:   map[string]any{"command": command},
			},
		},
	}
}

func toolCallUpdateNotification(id, command, output string, status acp.ToolCallStatus) acp.SessionNotification {
	return acp.SessionNotification{
		SessionId: "s",
		Update: acp.SessionUpdate{
			ToolCallUpdate: &acp.SessionToolCallUpdate{
				ToolCallId: acp.ToolCallId(id),
				Status:     &status,
				RawInput:   map[string]any{"command": command},
				Content:    []acp.ToolCallContent{acp.ToolContent(acp.TextBlock(output))},
			},
		},
	}
}

func messageNotification(text string) acp.SessionNotification {
	return acp.SessionNotification{
		SessionId: "s",
		Update: acp.SessionUpdate{
			AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{Content: acp.TextBlock(text)},
		},
	}
}

func thoughtNotification(text string) acp.SessionNotification {
	return acp.SessionNotification{
		SessionId: "s",
		Update: acp.SessionUpdate{
			AgentThoughtChunk: &acp.SessionUpdateAgentThoughtChunk{Content: acp.TextBlock(text)},
		},
	}
}

func TestReplayBeforePromptIsAuditOnly(t *testing.T) {
	result := TurnResult{}
	c := &denyClient{result: &result}
	completed := acp.ToolCallStatusCompleted

	if err := c.SessionUpdate(context.Background(), toolCallNotification("replay-1", "ghx read o/r src/old.ts", acp.ToolCallStatusPending)); err != nil {
		t.Fatal(err)
	}
	if err := c.SessionUpdate(context.Background(), toolCallUpdateNotification("replay-1", "ghx read o/r src/old.ts", "replayed output", completed)); err != nil {
		t.Fatal(err)
	}
	if err := c.SessionUpdate(context.Background(), messageNotification("replayed answer")); err != nil {
		t.Fatal(err)
	}
	if err := c.SessionUpdate(context.Background(), thoughtNotification("replayed thinking")); err != nil {
		t.Fatal(err)
	}

	c.promptSent = true
	if err := c.SessionUpdate(context.Background(), toolCallNotification("live-1", "ghx read o/r src/new.ts", acp.ToolCallStatusPending)); err != nil {
		t.Fatal(err)
	}
	if err := c.SessionUpdate(context.Background(), toolCallUpdateNotification("live-1", "ghx read o/r src/new.ts", "live output", completed)); err != nil {
		t.Fatal(err)
	}
	if err := c.SessionUpdate(context.Background(), messageNotification("live answer")); err != nil {
		t.Fatal(err)
	}
	if err := c.SessionUpdate(context.Background(), thoughtNotification("live thinking")); err != nil {
		t.Fatal(err)
	}

	if len(result.ToolTraces) != 1 || result.ToolTraces[0].ID != "live-1" {
		t.Fatalf("live traces = %+v, want only live-1", result.ToolTraces)
	}
	if len(result.ReplayedToolTraces) != 1 || result.ReplayedToolTraces[0].ID != "replay-1" {
		t.Fatalf("replayed traces = %+v, want only replay-1", result.ReplayedToolTraces)
	}
	if strings.Contains(result.FullText, "replayed") || result.FullText != "live answer" {
		t.Fatalf("FullText = %q, want only live answer", result.FullText)
	}
	if result.ReplayedText != "replayed answer" {
		t.Fatalf("ReplayedText = %q, want replayed answer", result.ReplayedText)
	}
	if result.Thinking != "live thinking" {
		t.Fatalf("Thinking = %q, want live thinking", result.Thinking)
	}
	if result.ReplayedThinking != "replayed thinking" {
		t.Fatalf("ReplayedThinking = %q, want replayed thinking", result.ReplayedThinking)
	}
	if result.ToolOutputChars != len("live output") {
		t.Fatalf("ToolOutputChars = %d, want %d", result.ToolOutputChars, len("live output"))
	}
	if len(result.ToolCalls) != 1 || !strings.Contains(result.ToolCalls[0], "src/new.ts") || strings.Contains(result.ToolCalls[0], "src/old.ts") {
		t.Fatalf("ToolCalls = %v, want only live command", result.ToolCalls)
	}
}

func TestWriteAndTerminalStayRefused(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	if _, err := c.WriteTextFile(context.Background(), acp.WriteTextFileRequest{}); err == nil {
		t.Error("WriteTextFile must remain refused")
	}
	if _, err := c.ReadTextFile(context.Background(), acp.ReadTextFileRequest{}); err == nil {
		t.Error("ReadTextFile must remain refused")
	}
	if _, err := c.CreateTerminal(context.Background(), acp.CreateTerminalRequest{}); err == nil {
		t.Error("CreateTerminal must remain refused")
	}
}

// TestIsQuotaExhausted pins the ADR-0040 L3 quota classifier against the real
// dead-ask wire message (ground truth: ~/.ghx/sessions/badslugnoslash
// sidecar.turn.error, "acp prompt: {\"code\":-32603,...errorKind rate_limit}").
// Dual detection mirrors IsLoadSessionResourceNotFound: structured
// *acp.RequestError with errorKind rate_limit, or message text ("session
// limit"/"rate limit") for the opaque-wrapped persisted form.
func TestIsQuotaExhausted(t *testing.T) {
	// The exact wire error from the badslugnoslash dead-ask log.
	realWireErr := fmt.Errorf("acp prompt: %s", `{"code":-32603,"message":"Internal error: You've hit your session limit · resets 8:30am (UTC)","data":{"errorKind":"rate_limit"}}`)
	structuredErr := &acp.RequestError{
		Code:    -32603,
		Message: "Internal error: You've hit your session limit · resets 8:30am (UTC)",
		Data:    map[string]any{"errorKind": "rate_limit"},
	}
	wrappedStructuredErr := fmt.Errorf("run turn: %w", structuredErr)

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"real wire message", realWireErr, true},
		{"structured errorKind rate_limit", structuredErr, true},
		{"wrapped structured", wrappedStructuredErr, true},
		{"session limit text", errors.New("You've hit your session limit · resets 8:30am (UTC)"), true},
		{"rate limit text", errors.New("Error: rate limit exceeded"), true},
		{"structured non-quota kind", &acp.RequestError{Code: -32603, Message: "Internal error", Data: map[string]any{"errorKind": "auth"}}, false},
		{"resource not found is not quota", &acp.RequestError{Code: -32002, Message: "Resource not found"}, false},
		{"unrelated error", errors.New("acp prompt: connection refused"), false},
	}
	for _, tc := range cases {
		if got := IsQuotaExhausted(tc.err); got != tc.want {
			t.Errorf("%s: IsQuotaExhausted = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestQuotaResetHint pins the reset-time extraction used in the DEGRADED
// answer affordance ("resets 8:30am (UTC)" → "8:30am (UTC)").
func TestQuotaResetHint(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"no reset info", errors.New("rate limit exceeded"), ""},
		{"plain text", errors.New("You've hit your session limit · resets 8:30am (UTC)"), "8:30am (UTC)"},
		{"wire form", fmt.Errorf("acp prompt: %s", `{"code":-32603,"message":"Internal error: You've hit your session limit · resets 8:30am (UTC)","data":{"errorKind":"rate_limit"}}`), "8:30am (UTC)"},
	}
	for _, tc := range cases {
		if got := QuotaResetHint(tc.err); got != tc.want {
			t.Errorf("%s: QuotaResetHint = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestShutdownAgentGraceful verifies a well-behaved stdio agent exits on
// stdin close without needing the kill fallback.
func TestShutdownAgentGraceful(t *testing.T) {
	cmd := exec.Command("cat")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	ShutdownAgent(cmd, stdin)
	if elapsed := time.Since(start); elapsed >= shutdownGrace {
		t.Errorf("graceful path took %v — fell through to the kill fallback", elapsed)
	}
	if !cmd.ProcessState.Exited() {
		t.Error("process not reaped")
	}
}

// TestShutdownAgentKillsStubbornProcess verifies a process that ignores
// stdin EOF is killed after the grace period.
func TestShutdownAgentKillsStubbornProcess(t *testing.T) {
	old := shutdownGrace
	shutdownGrace = 100 * time.Millisecond
	defer func() { shutdownGrace = old }()

	cmd := exec.Command("sleep", "30")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	ShutdownAgent(cmd, stdin)
	if cmd.ProcessState == nil {
		t.Fatal("process not reaped")
	}
	if cmd.ProcessState.Exited() && cmd.ProcessState.Success() {
		t.Error("expected the process to be killed, but it exited cleanly")
	}
}

// readToolCallWithLocations builds a Read-kind tool_call START notification
// (SessionUpdateToolCall) carrying the given location paths — the ACP wire
// shape the classifier's R6 path-scope rule reads from.
func readToolCallWithLocations(id string, status acp.ToolCallStatus, paths ...string) acp.SessionNotification {
	locs := make([]acp.ToolCallLocation, 0, len(paths))
	for _, p := range paths {
		locs = append(locs, acp.ToolCallLocation{Path: p})
	}
	return acp.SessionNotification{
		SessionId: "s",
		Update: acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				ToolCallId: acp.ToolCallId(id),
				Title:      "Read",
				Kind:       acp.ToolKindRead,
				Status:     status,
				Locations:  locs,
			},
		},
	}
}

// readToolCallUpdateWithLocations builds a tool_call UPDATE notification
// (SessionToolCallUpdate) carrying the given location paths.
func readToolCallUpdateWithLocations(id string, status acp.ToolCallStatus, paths ...string) acp.SessionNotification {
	locs := make([]acp.ToolCallLocation, 0, len(paths))
	for _, p := range paths {
		locs = append(locs, acp.ToolCallLocation{Path: p})
	}
	return acp.SessionNotification{
		SessionId: "s",
		Update: acp.SessionUpdate{
			ToolCallUpdate: &acp.SessionToolCallUpdate{
				ToolCallId: acp.ToolCallId(id),
				Status:     &status,
				Locations:  locs,
			},
		},
	}
}

// TestSessionUpdatePopulatesLocations proves the ACP tool-call locations wire
// through denyClient.SessionUpdate into ToolCallTrace.Locations, which the
// host-task R6 path-scope classifier reads (ADR-0032.1 S2). Before this fix
// the field was declared but never written, so the detector ran on always-empty
// data. The test feeds a Read start carrying one path and an update adding a
// second (with the first repeated), and asserts the trace holds the deduped,
// order-stable union.
func TestSessionUpdatePopulatesLocations(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	c.markPromptSent()

	ctx := context.Background()
	if err := c.SessionUpdate(ctx, readToolCallWithLocations("read-1", acp.ToolCallStatusPending, "/repo/src/a.go")); err != nil {
		t.Fatalf("start update: %v", err)
	}
	// Update repeats the first path (must dedupe) and adds a second.
	if err := c.SessionUpdate(ctx, readToolCallUpdateWithLocations("read-1", acp.ToolCallStatusCompleted, "/repo/src/a.go", "/repo/src/b.go")); err != nil {
		t.Fatalf("update: %v", err)
	}

	if len(c.result.ToolTraces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(c.result.ToolTraces))
	}
	got := c.result.ToolTraces[0].Locations
	want := []string{"/repo/src/a.go", "/repo/src/b.go"}
	if len(got) != len(want) {
		t.Fatalf("Locations = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Locations = %v, want %v (order-stable union, deduped)", got, want)
		}
	}
}

// TestSessionUpdateLocationsEmptyUpdatePreserves proves an update carrying no
// locations (the omitempty nil case) leaves an existing trace's Locations
// intact rather than clearing them.
func TestSessionUpdateLocationsEmptyUpdatePreserves(t *testing.T) {
	c := &denyClient{result: &TurnResult{}}
	c.markPromptSent()

	ctx := context.Background()
	if err := c.SessionUpdate(ctx, readToolCallWithLocations("read-2", acp.ToolCallStatusPending, "/repo/src/a.go")); err != nil {
		t.Fatalf("start update: %v", err)
	}
	// A status-only update with no locations must not wipe the seeded path.
	if err := c.SessionUpdate(ctx, readToolCallUpdateWithLocations("read-2", acp.ToolCallStatusCompleted)); err != nil {
		t.Fatalf("empty update: %v", err)
	}

	got := c.result.ToolTraces[0].Locations
	if len(got) != 1 || got[0] != "/repo/src/a.go" {
		t.Fatalf("Locations = %v, want [/repo/src/a.go] preserved", got)
	}
}
