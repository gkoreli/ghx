package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// denyClient implements acp.Client with read-only permission semantics.
// Text deltas are streamed to stdout; tool call events are written to stderr.
// Write-shaped operations are rejected so the sidecar cannot mutate state.
//
// mu guards result/promptSent/closed: session updates arrive from the SDK's
// notification goroutine, and the D2 cancellable wait can abandon a turn while
// late notifications are still in flight — closeAndSnapshot freezes the result
// so the caller reads a consistent copy (ADR-0027 D2).
type denyClient struct {
	mu         sync.Mutex
	result     *TurnResult
	promptSent bool
	closed     bool
	// onActivity, when set, is invoked on every session update. It feeds the
	// D2 liveness watchdog; it must not block.
	onActivity func()
	// live, when set, streams each new (non-replayed) session update to the
	// session's live.jsonl as it happens (ADR-0022.1). LiveLog methods are
	// nil-safe and best-effort, so this never fails or slows a turn.
	live *LiveLog
}

// markPromptSent flips replay accounting to live-turn accounting.
func (c *denyClient) markPromptSent() {
	c.mu.Lock()
	c.promptSent = true
	c.mu.Unlock()
}

// HandleExtensionMethod receives ACP extension notifications. The only one
// consumed is the adapter's _claude/sdkMessage raw-SDK audit stream
// (ADR-0016.10 D1), recorded into the turn's RawSDKAudit under the same
// lock/closed/promptSent discipline as SessionUpdate. Raw messages count as
// liveness activity too — a turn streaming only raw messages is not hung.
func (c *denyClient) HandleExtensionMethod(_ context.Context, method string, params json.RawMessage) (any, error) {
	if method != RawSDKMessageMethod {
		return nil, acp.NewMethodNotFound(method)
	}
	if c.onActivity != nil {
		c.onActivity()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, nil
	}
	if c.result.RawSDK == nil {
		c.result.RawSDK = &RawSDKAudit{}
	}
	c.result.RawSDK.Record(params, !c.promptSent)
	return nil, nil
}

// closeAndSnapshot stops recording and returns a copy of the accumulated turn
// result that is safe to read even if the peer is still emitting updates.
func (c *denyClient) closeAndSnapshot() TurnResult {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return *c.result
}

func (c *denyClient) upsertTrace(id string, replayed bool) *ToolCallTrace {
	traces := &c.result.ToolTraces
	if replayed {
		traces = &c.result.ReplayedToolTraces
	}
	for i := range *traces {
		if (*traces)[i].ID == id {
			return &(*traces)[i]
		}
	}
	*traces = append(*traces, ToolCallTrace{ID: id})
	return &(*traces)[len(*traces)-1]
}

// mergeLocations unions the paths from an ACP tool-call notification's
// locations into the trace, deduped and order-stable. The ACP wire carries
// locations on both the tool_call start (SessionUpdateToolCall.Locations) and
// the tool_call_update (SessionToolCallUpdate.Locations); a start may seed
// initial paths and a later update may add more, so we accumulate the union of
// every path seen for the call rather than replacing — the classifier only
// needs the set of touched paths (ADR-0032.1 S2 path scope). Empty paths are
// skipped; an update carrying no locations (the omitempty nil case) leaves the
// trace's Locations unchanged.
func mergeLocations(tr *ToolCallTrace, locs []acp.ToolCallLocation) {
	for _, l := range locs {
		p := l.Path
		if p == "" {
			continue
		}
		if !slices.Contains(tr.Locations, p) {
			tr.Locations = append(tr.Locations, p)
		}
	}
}

func (c *denyClient) refreshToolSummaries() {
	c.result.ToolCalls = c.result.ToolCalls[:0]
	for _, tr := range c.result.ToolTraces {
		c.result.ToolCalls = append(c.result.ToolCalls, toolSummary(tr))
	}
}

// isWriteToolKind reports whether a permission request is for a mutating
// tool. Nil kind is treated as unknown and therefore write-shaped: agents
// that do not classify their tools do not get auto-approval.
func isWriteToolKind(k *acp.ToolKind) bool {
	if k == nil {
		return true
	}
	switch *k {
	case acp.ToolKindRead, acp.ToolKindSearch, acp.ToolKindExecute, acp.ToolKindFetch, acp.ToolKindThink:
		return false
	}
	return true
}

// isSubmitReportTool reports whether a permission request is for the sidecar's
// own report-sink submit_report tool. The adapter titles MCP tool calls with
// the fully-qualified name (mcp__<server>__<tool>) in its default tool-info
// branch, so a title match identifies our tool.
func isSubmitReportTool(tc acp.ToolCallUpdate) bool {
	if tc.Title == nil {
		return false
	}
	t := *tc.Title
	return t == SubmitReportToolID || strings.HasSuffix(t, "__"+SubmitReportToolName)
}

// RequestPermission approves read/search/execute/fetch tool permissions and
// rejects everything write-shaped (edit, delete, move, unknown).
//
// The sidecar's evidence gathering runs through shell `ghx` invocations,
// which permission-requesting agents (e.g. the Claude ACP adapter) classify
// as "execute". The original deny-all policy rejected those requests too,
// which blocked the sidecar from doing any work at all — the first live
// eval episode (ADR-0016.1) surfaced this. Mutation safety is still layered:
// write-kind permissions are rejected here, WriteTextFile/ReadTextFile
// return errors, and terminal methods are refused. Residual risk — an agent
// mislabeling a mutating shell command as "execute" — is bounded by the
// persona contract and acceptable for a reconnaissance harness.
func (c *denyClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	var wantOnce, wantAlways acp.PermissionOptionKind
	// The report-sink submit_report tool is a session-scoped, sidecar-owned MCP
	// tool (ADR-0021 D1). The adapter classifies all MCP tools as kind "other"
	// (claude-agent-acp tools.js default case), which isWriteToolKind treats as
	// write-shaped and would reject. It is auto-approved via allowedTools so this
	// path is normally never hit; approving it here too is defense-in-depth for
	// adapters that still route it through permission. It writes only to the
	// runtime-owned sink file, never to the repo.
	if isSubmitReportTool(params.ToolCall) {
		wantOnce, wantAlways = acp.PermissionOptionKindAllowOnce, acp.PermissionOptionKindAllowAlways
	} else if isWriteToolKind(params.ToolCall.Kind) {
		wantOnce, wantAlways = acp.PermissionOptionKindRejectOnce, acp.PermissionOptionKindRejectAlways
	} else {
		wantOnce, wantAlways = acp.PermissionOptionKindAllowOnce, acp.PermissionOptionKindAllowAlways
	}
	for _, o := range params.Options {
		if o.Kind == wantOnce || o.Kind == wantAlways {
			return acp.RequestPermissionResponse{
				Outcome: acp.RequestPermissionOutcome{
					Selected: &acp.RequestPermissionOutcomeSelected{OptionId: o.OptionId},
				},
			}, nil
		}
	}
	// No matching option — cancel to be safe.
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{}},
	}, nil
}

// SessionUpdate streams text to stdout and records tool call observations.
// Every update also feeds the liveness watchdog (ADR-0027 D2).
func (c *denyClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	if c.onActivity != nil {
		c.onActivity()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	u := params.Update
	replayed := !c.promptSent
	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			text := u.AgentMessageChunk.Content.Text.Text
			if replayed {
				c.result.ReplayedText += text
				return nil
			}
			os.Stdout.WriteString(text)
			c.result.FullText += text
			c.live.Text(len(text), text)
		}
	case u.AgentThoughtChunk != nil:
		if u.AgentThoughtChunk.Content.Text != nil {
			text := u.AgentThoughtChunk.Content.Text.Text
			if replayed {
				c.result.ReplayedThinking += text
				return nil
			}
			c.result.Thinking += text
			c.live.Thought(len(text), text)
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		tr := c.upsertTrace(string(tc.ToolCallId), replayed)
		tr.Title = tc.Title
		tr.Kind = string(tc.Kind)
		if tc.RawInput != nil {
			tr.RawInput = tc.RawInput
		}
		mergeLocations(tr, tc.Locations)
		tr.StatusTransitions = append(tr.StatusTransitions, ToolStatusTransition{Status: string(tc.Status), At: time.Now().UTC()})
		size := ContentSize(tc.Content, tc.RawOutput)
		tr.OutputSize += size
		appendExcerpt(tr, ToolOutputText(tc.Content, tc.RawOutput))
		if !replayed {
			c.result.ToolOutputChars += size
			c.refreshToolSummaries()
			c.live.ToolCall(string(tc.ToolCallId), tc.Title, string(tc.Kind), string(tc.Status))
			entry := toolSummary(*tr)
			fmt.Fprintf(os.Stderr, "  ▶ %s\n", entry)
		}
	case u.ToolCallUpdate != nil:
		tcu := u.ToolCallUpdate
		tr := c.upsertTrace(string(tcu.ToolCallId), replayed)
		if tcu.Title != nil {
			tr.Title = *tcu.Title
		}
		if tcu.Kind != nil {
			tr.Kind = string(*tcu.Kind)
		}
		if tcu.RawInput != nil {
			tr.RawInput = tcu.RawInput
		}
		mergeLocations(tr, tcu.Locations)
		if tcu.Status != nil {
			tr.StatusTransitions = append(tr.StatusTransitions, ToolStatusTransition{Status: string(*tcu.Status), At: time.Now().UTC()})
		}
		size := ContentSize(tcu.Content, tcu.RawOutput)
		tr.OutputSize += size
		appendExcerpt(tr, ToolOutputText(tcu.Content, tcu.RawOutput))
		if !replayed {
			c.result.ToolOutputChars += size
			c.refreshToolSummaries()
			status := ""
			if tcu.Status != nil {
				status = string(*tcu.Status)
			}
			c.live.ToolUpdate(string(tcu.ToolCallId), status, size)
		}
	}
	return nil
}

// WriteTextFile rejects file writes — sidecar is read-only.
func (c *denyClient) WriteTextFile(_ context.Context, _ acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	return acp.WriteTextFileResponse{}, fmt.Errorf("sidecar: write operations are not allowed")
}

// ReadTextFile is not used by the sidecar (all reads go through ghx), but
// the interface requires it.
func (c *denyClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, fmt.Errorf("sidecar: direct file reads are not allowed; use ghx")
}

// Terminal stubs — required by the acp.Client interface.
func (c *denyClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, fmt.Errorf("sidecar: terminal not allowed")
}
func (c *denyClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, fmt.Errorf("sidecar: terminal not allowed")
}
func (c *denyClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, fmt.Errorf("sidecar: terminal not allowed")
}
func (c *denyClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, fmt.Errorf("sidecar: terminal not allowed")
}
func (c *denyClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, fmt.Errorf("sidecar: terminal not allowed")
}
