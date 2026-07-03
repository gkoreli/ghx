package sidecar

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	acp "github.com/coder/acp-go-sdk"
)

// TurnResult holds the streamed output of one ACP prompt turn.
type TurnResult struct {
	// FullText is the complete text emitted by the agent.
	FullText string
	// ToolCalls lists the tool calls observed during the turn (title + status).
	ToolCalls []string
}

// denyClient implements acp.Client with read-only permission semantics.
// Text deltas are streamed to stdout; tool call events are written to stderr.
// Write-shaped operations are rejected so the sidecar cannot mutate state.
type denyClient struct {
	result *TurnResult
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
	if isWriteToolKind(params.ToolCall.Kind) {
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
func (c *denyClient) SessionUpdate(_ context.Context, params acp.SessionNotification) error {
	u := params.Update
	switch {
	case u.AgentMessageChunk != nil:
		if u.AgentMessageChunk.Content.Text != nil {
			text := u.AgentMessageChunk.Content.Text.Text
			os.Stdout.WriteString(text)
			c.result.FullText += text
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		entry := fmt.Sprintf("%s (%s)", tc.Title, tc.Status)
		c.result.ToolCalls = append(c.result.ToolCalls, entry)
		fmt.Fprintf(os.Stderr, "  ▶ %s\n", entry)
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

// RunTurn spawns the agent binary, establishes an ACP session (new or resumed),
// sends prompt, streams output, and returns the collected TurnResult and the
// ACP session ID to persist for follow-up turns.
//
// Pass acpSessionID="" on the first turn; pass the returned sessionID on
// subsequent turns to resume the agent's context.
func RunTurn(ctx context.Context, agentCmd, acpSessionID, prompt string) (result TurnResult, newSessionID string, err error) {
	cmd := exec.CommandContext(ctx, agentCmd)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return result, "", fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return result, "", fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return result, "", fmt.Errorf("start %q: %w", agentCmd, err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	client := &denyClient{result: &result}
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	cwd, _ := os.Getwd()

	if _, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	}); err != nil {
		return result, "", fmt.Errorf("acp initialize: %w", err)
	}

	var sessionID acp.SessionId
	if acpSessionID == "" {
		resp, err := conn.NewSession(ctx, acp.NewSessionRequest{
			Cwd:        cwd,
			McpServers: []acp.McpServer{},
		})
		if err != nil {
			return result, "", fmt.Errorf("acp new session: %w", err)
		}
		sessionID = resp.SessionId
	} else {
		_, err := conn.LoadSession(ctx, acp.LoadSessionRequest{
			SessionId:  acp.SessionId(acpSessionID),
			Cwd:        cwd,
			McpServers: []acp.McpServer{},
		})
		if err != nil {
			return result, "", fmt.Errorf("acp load session: %w", err)
		}
		sessionID = acp.SessionId(acpSessionID)
	}

	if _, err := conn.Prompt(ctx, acp.PromptRequest{
		SessionId: sessionID,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	}); err != nil {
		return result, "", fmt.Errorf("acp prompt: %w", err)
	}

	os.Stdout.WriteString("\n")
	return result, string(sessionID), nil
}
