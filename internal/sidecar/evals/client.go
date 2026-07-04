package evals

import (
	"context"
	"fmt"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// evalClient implements acp.Client for the two direct profiles.
//
// It differs from the production sidecar denyClient deliberately: direct
// baselines must be able to run read-only tools (shell ghx/gh invocations),
// so read/search/execute/fetch permission requests are approved. Write-shaped
// requests (edit/delete/move) are rejected AND recorded as violations —
// reconnaissance is read-only in every profile (ADR-0016.1).
type evalClient struct {
	// current is the turn record being captured; the runner points this at
	// a fresh record before each prompt. Turns run sequentially.
	current *TurnRecord
	// promptSent is raised immediately before session/prompt is sent. Updates
	// received earlier are ACP history replay, not activity for this turn.
	promptSent bool
	// violations accumulates safety-contract breaches for the episode.
	violations []string
}

func (c *evalClient) upsertTrace(id string, replayed bool) *ToolCallTrace {
	traces := &c.current.ToolTraces
	if replayed {
		traces = &c.current.ReplayedToolTraces
	}
	for i := range *traces {
		if (*traces)[i].ID == id {
			return &(*traces)[i]
		}
	}
	*traces = append(*traces, ToolCallTrace{ID: id})
	return &(*traces)[len(*traces)-1]
}

func isWriteKind(k *acp.ToolKind) bool {
	if k == nil {
		return false
	}
	switch *k {
	case acp.ToolKindEdit, acp.ToolKindDelete, acp.ToolKindMove:
		return true
	}
	return false
}

// RequestPermission approves non-write tool use and rejects writes.
func (c *evalClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	allowed := !isWriteKind(params.ToolCall.Kind)
	if !allowed {
		title := ""
		if params.ToolCall.Title != nil {
			title = *params.ToolCall.Title
		}
		c.violations = append(c.violations, fmt.Sprintf("write-kind permission requested: %s", title))
	}

	var wantOnce, wantAlways acp.PermissionOptionKind
	if allowed {
		wantOnce, wantAlways = acp.PermissionOptionKindAllowOnce, acp.PermissionOptionKindAllowAlways
	} else {
		wantOnce, wantAlways = acp.PermissionOptionKindRejectOnce, acp.PermissionOptionKindRejectAlways
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
	return acp.RequestPermissionResponse{
		Outcome: acp.RequestPermissionOutcome{Cancelled: &acp.RequestPermissionOutcomeCancelled{}},
	}, nil
}

// SessionUpdate captures text and tool-call events into the current turn.
func (c *evalClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	if c.current == nil {
		return nil
	}
	u := params.Update
	replayed := !c.promptSent
	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			if replayed {
				c.current.ReplayedText += u.AgentMessageChunk.Content.Text.Text
				return nil
			}
			c.current.Text += u.AgentMessageChunk.Content.Text.Text
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		tr := c.upsertTrace(string(tc.ToolCallId), replayed)
		tr.Title = tc.Title
		tr.Kind = string(tc.Kind)
		if tc.RawInput != nil {
			tr.RawInput = tc.RawInput
		}
		tr.StatusTransitions = append(tr.StatusTransitions, ToolStatusTransition{Status: string(tc.Status), At: nowUTC()})
		size := sidecar.ContentSize(tc.Content, tc.RawOutput)
		tr.OutputSize += size
		appendExcerpt(tr, sidecar.ToolOutputText(tc.Content, tc.RawOutput))
		if !replayed {
			c.current.ToolOutputChars += size
			rebuildToolSummaries(c.current)
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
		if tcu.Status != nil {
			tr.StatusTransitions = append(tr.StatusTransitions, ToolStatusTransition{Status: string(*tcu.Status), At: nowUTC()})
		}
		size := sidecar.ContentSize(tcu.Content, tcu.RawOutput)
		tr.OutputSize += size
		appendExcerpt(tr, sidecar.ToolOutputText(tcu.Content, tcu.RawOutput))
		if !replayed {
			c.current.ToolOutputChars += size
			rebuildToolSummaries(c.current)
		}
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }

func appendExcerpt(tr *ToolCallTrace, text string) {
	const max = 2048
	if text == "" || len(tr.OutputExcerpt) >= max {
		return
	}
	remain := max - len(tr.OutputExcerpt)
	if len(text) > remain {
		text = text[:remain]
	}
	tr.OutputExcerpt += text
}

// WriteTextFile is a contract violation for every eval profile.
func (c *evalClient) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	c.violations = append(c.violations, fmt.Sprintf("write attempted: %s", params.Path))
	return acp.WriteTextFileResponse{}, fmt.Errorf("evals: write operations are not allowed")
}

// ReadTextFile is refused: reconnaissance evidence must come from repo tools,
// not the local filesystem. Not recorded as a violation (read-only, harmless).
func (c *evalClient) ReadTextFile(_ context.Context, _ acp.ReadTextFileRequest) (acp.ReadTextFileResponse, error) {
	return acp.ReadTextFileResponse{}, fmt.Errorf("evals: direct file reads are not allowed")
}

// Terminal methods are refused; agents run tools agent-side.
func (c *evalClient) CreateTerminal(_ context.Context, _ acp.CreateTerminalRequest) (acp.CreateTerminalResponse, error) {
	return acp.CreateTerminalResponse{}, fmt.Errorf("evals: terminal not allowed")
}
func (c *evalClient) TerminalOutput(_ context.Context, _ acp.TerminalOutputRequest) (acp.TerminalOutputResponse, error) {
	return acp.TerminalOutputResponse{}, fmt.Errorf("evals: terminal not allowed")
}
func (c *evalClient) ReleaseTerminal(_ context.Context, _ acp.ReleaseTerminalRequest) (acp.ReleaseTerminalResponse, error) {
	return acp.ReleaseTerminalResponse{}, fmt.Errorf("evals: terminal not allowed")
}
func (c *evalClient) WaitForTerminalExit(_ context.Context, _ acp.WaitForTerminalExitRequest) (acp.WaitForTerminalExitResponse, error) {
	return acp.WaitForTerminalExitResponse{}, fmt.Errorf("evals: terminal not allowed")
}
func (c *evalClient) KillTerminal(_ context.Context, _ acp.KillTerminalRequest) (acp.KillTerminalResponse, error) {
	return acp.KillTerminalResponse{}, fmt.Errorf("evals: terminal not allowed")
}
