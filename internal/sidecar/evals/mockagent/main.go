// Command mockagent is a scripted ACP agent used by the eval kernel's
// end-to-end plumbing tests (ADR-0016.1). It speaks the real Agent Client
// Protocol over stdio — initialize, session/new, session/load, session/prompt,
// session/update notifications — but produces deterministic scripted output
// instead of calling an LLM. This lets the full episode path (spawn → ACP
// handshake → tool-call events → <ghx-report> extraction → rewards →
// artifacts) run in normal CI with no model, no token, and no network.
//
// Configuration via environment:
//
//	MOCKAGENT_SCRIPT — path to a JSON file: [{"toolCalls": [...], "text": "..."}]
//	MOCKAGENT_STATE  — path to a counter file persisted across process spawns,
//	                   so per-turn agent processes (the sidecar production
//	                   pattern) advance through the script.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// reply is one scripted prompt response.
type reply struct {
	ToolCalls []string `json:"toolCalls"`
	Text      string   `json:"text"`
}

// mockAgent implements acp.Agent and acp.AgentLoader with scripted behavior.
type mockAgent struct {
	conn    *acp.AgentSideConnection
	replies []reply
	state   string
}

// nextIndex reads and increments the persistent prompt counter.
func (m *mockAgent) nextIndex() int {
	n := 0
	if data, err := os.ReadFile(m.state); err == nil {
		n, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	_ = os.WriteFile(m.state, []byte(strconv.Itoa(n+1)), 0o644)
	return n
}

func (m *mockAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{LoadSession: true},
	}, nil
}

func (m *mockAgent) NewSession(_ context.Context, _ acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return acp.NewSessionResponse{SessionId: "mock-sess-1"}, nil
}

// LoadSession accepts any session ID — resumption always succeeds.
func (m *mockAgent) LoadSession(_ context.Context, _ acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return acp.LoadSessionResponse{}, nil
}

func (m *mockAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	idx := m.nextIndex()
	if idx >= len(m.replies) {
		idx = len(m.replies) - 1
	}
	r := m.replies[idx]

	for i, title := range r.ToolCalls {
		update := acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				ToolCallId: acp.ToolCallId(fmt.Sprintf("tc-%d-%d", idx, i)),
				Title:      title,
				Kind:       acp.ToolKindExecute,
				Status:     acp.ToolCallStatusCompleted,
			},
		}
		if err := m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update:    update,
		}); err != nil {
			return acp.PromptResponse{}, err
		}
	}

	if err := m.conn.SessionUpdate(ctx, acp.SessionNotification{
		SessionId: params.SessionId,
		Update: acp.SessionUpdate{
			AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
				Content: acp.TextBlock(r.Text),
			},
		},
	}); err != nil {
		return acp.PromptResponse{}, err
	}

	return acp.PromptResponse{StopReason: acp.StopReasonEndTurn}, nil
}

func (m *mockAgent) Authenticate(_ context.Context, _ acp.AuthenticateRequest) (acp.AuthenticateResponse, error) {
	return acp.AuthenticateResponse{}, nil
}
func (m *mockAgent) Cancel(_ context.Context, _ acp.CancelNotification) error { return nil }
func (m *mockAgent) CloseSession(_ context.Context, _ acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return acp.CloseSessionResponse{}, nil
}
func (m *mockAgent) ListSessions(_ context.Context, _ acp.ListSessionsRequest) (acp.ListSessionsResponse, error) {
	return acp.ListSessionsResponse{}, nil
}
func (m *mockAgent) ResumeSession(_ context.Context, _ acp.ResumeSessionRequest) (acp.ResumeSessionResponse, error) {
	return acp.ResumeSessionResponse{}, nil
}
func (m *mockAgent) SetSessionConfigOption(_ context.Context, _ acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	return acp.SetSessionConfigOptionResponse{}, nil
}
func (m *mockAgent) SetSessionMode(_ context.Context, _ acp.SetSessionModeRequest) (acp.SetSessionModeResponse, error) {
	return acp.SetSessionModeResponse{}, nil
}

func main() {
	scriptPath := os.Getenv("MOCKAGENT_SCRIPT")
	statePath := os.Getenv("MOCKAGENT_STATE")
	if scriptPath == "" || statePath == "" {
		fmt.Fprintln(os.Stderr, "mockagent: MOCKAGENT_SCRIPT and MOCKAGENT_STATE are required")
		os.Exit(2)
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mockagent: read script: %v\n", err)
		os.Exit(2)
	}
	var replies []reply
	if err := json.Unmarshal(data, &replies); err != nil || len(replies) == 0 {
		fmt.Fprintf(os.Stderr, "mockagent: invalid script: %v\n", err)
		os.Exit(2)
	}

	agent := &mockAgent{replies: replies, state: statePath}
	conn := acp.NewAgentSideConnection(agent, os.Stdout, os.Stdin)
	agent.conn = conn
	<-conn.Done()
}
