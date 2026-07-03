package evals

import (
	"context"
	"fmt"

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
	// violations accumulates safety-contract breaches for the episode.
	violations []string
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
	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			c.current.Text += u.AgentMessageChunk.Content.Text.Text
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		c.current.ToolCalls = append(c.current.ToolCalls, fmt.Sprintf("%s (%s)", tc.Title, tc.Status))
		c.current.ToolOutputChars += sidecar.ContentSize(tc.Content, tc.RawOutput)
	case u.ToolCallUpdate != nil:
		tcu := u.ToolCallUpdate
		c.current.ToolOutputChars += sidecar.ContentSize(tcu.Content, tcu.RawOutput)
	}
	return nil
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
