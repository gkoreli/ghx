package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// TurnResult holds the streamed output of one ACP prompt turn.
type TurnResult struct {
	// FullText is the complete text emitted by the agent.
	FullText string
	// ToolCalls lists the tool calls observed during the turn (title + status).
	ToolCalls []string
	// ToolOutputChars approximates the size of tool outputs the agent
	// consumed (content blocks and raw output on tool_call/tool_call_update
	// events). Produced text alone understates context burden; this is the
	// other half.
	ToolOutputChars int
}

// ContentSize approximates the character size of tool-call content blocks
// plus raw output. JSON length is used for non-string raw output.
func ContentSize(content []acp.ToolCallContent, rawOutput any) int {
	n := 0
	for _, c := range content {
		if c.Content != nil && c.Content.Content.Text != nil {
			n += len(c.Content.Content.Text.Text)
		}
	}
	switch v := rawOutput.(type) {
	case nil:
	case string:
		n += len(v)
	default:
		if data, err := json.Marshal(v); err == nil {
			n += len(data)
		}
	}
	return n
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
		c.result.ToolOutputChars += ContentSize(tc.Content, tc.RawOutput)
		fmt.Fprintf(os.Stderr, "  ▶ %s\n", entry)
	case u.ToolCallUpdate != nil:
		tcu := u.ToolCallUpdate
		c.result.ToolOutputChars += ContentSize(tcu.Content, tcu.RawOutput)
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

// shutdownGrace is how long ShutdownAgent waits for a spawned agent to exit
// on its own after stdin closes before falling back to a hard kill. Package
// variable (not const) so tests can shorten it.
var shutdownGrace = 2 * time.Second

// ShutdownAgent terminates a spawned ACP agent process gracefully: closing
// stdin signals EOF so a well-behaved stdio agent exits on its own — a hard
// kill mid-write makes Node-based adapters dump an EPIPE stack trace into
// logs. If the process is still running after shutdownGrace, it is killed.
// Safe to defer immediately after cmd.Start.
func ShutdownAgent(cmd *exec.Cmd, stdin io.Closer) {
	_ = stdin.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(shutdownGrace):
		_ = cmd.Process.Kill()
		<-done
	}
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
	defer ShutdownAgent(cmd, stdin)

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
