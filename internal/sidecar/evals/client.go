package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// evalClient implements acp.Client for the two direct profiles.
//
// It differs from the production sidecar denyClient deliberately: direct
// baselines must be able to run read-only tools (shell ghx/gh invocations),
// so read/search/execute/fetch permission requests are approved. Write-shaped
// requests (edit/delete/move) go through the writes policy; with the default
// nil policy they are rejected AND recorded as violations — reconnaissance
// is read-only in every recon profile (ADR-0016.1). The host-task arms
// (ADR-0032.1 S2) set a WorkspaceWritePolicy so the host can edit its own
// provisioned workspace, and nothing else.
type evalClient struct {
	// current is the turn record being captured; the runner points this at
	// a fresh record before each prompt. Turns run sequentially.
	current *TurnRecord
	// promptSent is raised immediately before session/prompt is sent. Updates
	// received earlier are ACP history replay, not activity for this turn.
	promptSent bool
	// violations accumulates safety-contract breaches for the episode.
	violations []string
	// writes decides write-kind tool use. nil is the recon read-only
	// contract — deny every write and record it as a violation (ADR-0016.1
	// G5). The host-task arm injects a WorkspaceWritePolicy so
	// workspace-scoped writes are allowed and everything outside is denied
	// (ADR-0032.1 S2). Every denial, under any policy, is recorded.
	writes WritePolicy
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

// RequestPermission approves non-write tool use; write-kind requests go
// through the write policy (nil policy = deny all). Every denial is recorded
// as a violation so the episode shows it.
func (c *evalClient) RequestPermission(_ context.Context, params acp.RequestPermissionRequest) (acp.RequestPermissionResponse, error) {
	allowed := true
	if isWriteKind(params.ToolCall.Kind) {
		allowed = c.approveWrite(params.ToolCall)
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

// approveWrite decides one write-kind permission request against the write
// policy and records every denial as a violation. With a policy set,
// approval requires the request to name at least one target location and
// every location to pass containment — an unlocated write cannot be checked
// against the workspace, so it is denied (fail closed), never guessed about.
func (c *evalClient) approveWrite(tc acp.ToolCallUpdate) bool {
	title := ""
	if tc.Title != nil {
		title = *tc.Title
	}
	if c.writes == nil {
		c.violations = append(c.violations, fmt.Sprintf("write-kind permission requested: %s", title))
		return false
	}
	if len(tc.Locations) == 0 {
		c.violations = append(c.violations, fmt.Sprintf("write denied (no target path reported): %s", title))
		return false
	}
	for _, loc := range tc.Locations {
		if _, err := c.writes.AllowWrite(loc.Path); err != nil {
			c.violations = append(c.violations, fmt.Sprintf("write denied (%v): %s", err, title))
			return false
		}
	}
	return true
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
	case u.AgentThoughtChunk != nil:
		if u.AgentThoughtChunk.Content.Text != nil {
			if replayed {
				c.current.ReplayedThinking += u.AgentThoughtChunk.Content.Text.Text
				return nil
			}
			c.current.Thinking += u.AgentThoughtChunk.Content.Text.Text
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		tr := c.upsertTrace(string(tc.ToolCallId), replayed)
		tr.Title = tc.Title
		tr.Kind = string(tc.Kind)
		if tc.RawInput != nil {
			tr.RawInput = tc.RawInput
		}
		if len(tc.Locations) > 0 {
			tr.Locations = locationPaths(tc.Locations)
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
		if tcu.Locations != nil { // ACP semantics: replace the collection
			tr.Locations = locationPaths(tcu.Locations)
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

// HandleExtensionMethod records the adapter's _claude/sdkMessage raw-SDK
// audit stream into the current turn (ADR-0016.10 D1). Direct-profile
// sessions enable the channel via directSessionMeta; turns without a current
// record (between prompts) drop the message.
func (c *evalClient) HandleExtensionMethod(_ context.Context, method string, params json.RawMessage) (any, error) {
	if method != sidecar.RawSDKMessageMethod {
		return nil, acp.NewMethodNotFound(method)
	}
	if c.current == nil {
		return nil, nil
	}
	if c.current.RawSDK == nil {
		c.current.RawSDK = &sidecar.RawSDKAudit{}
	}
	c.current.RawSDK.Record(params, !c.promptSent)
	return nil, nil
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

// WriteTextFile performs the write when the policy allows it (host arm,
// workspace-scoped) and is a recorded contract violation otherwise — with a
// nil policy every write attempt stays run-invalidating (recon profiles and
// the sidecar inside arm B). The write targets the policy-resolved path, so
// the containment decision and the operation agree (within WorkspaceScope's
// stated TOCTOU limits).
func (c *evalClient) WriteTextFile(_ context.Context, params acp.WriteTextFileRequest) (acp.WriteTextFileResponse, error) {
	if c.writes == nil {
		c.violations = append(c.violations, fmt.Sprintf("write attempted: %s", params.Path))
		return acp.WriteTextFileResponse{}, fmt.Errorf("evals: write operations are not allowed")
	}
	resolved, err := c.writes.AllowWrite(params.Path)
	if err != nil {
		c.violations = append(c.violations, fmt.Sprintf("write denied (%v): %s", err, params.Path))
		return acp.WriteTextFileResponse{}, fmt.Errorf("evals: write denied (%v)", err)
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	if err := os.WriteFile(resolved, []byte(params.Content), 0o644); err != nil {
		return acp.WriteTextFileResponse{}, err
	}
	return acp.WriteTextFileResponse{}, nil
}

// locationPaths flattens ACP tool-call locations to their file paths.
func locationPaths(locs []acp.ToolCallLocation) []string {
	if len(locs) == 0 {
		return nil
	}
	paths := make([]string, len(locs))
	for i, l := range locs {
		paths[i] = l.Path
	}
	return paths
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
