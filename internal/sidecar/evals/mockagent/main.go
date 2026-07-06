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
//	MOCKAGENT_SCRIPT     — path to a JSON file:
//	                       [{"toolCalls": [...], "replayOnLoad": [...], "text": "..."}]
//	MOCKAGENT_STATE      — path to a counter file persisted across process spawns,
//	                       so per-turn agent processes (the sidecar production
//	                       pattern) advance through the script.
//	MOCKAGENT_PROMPT_LOG — optional path; every LoadSession and Prompt is
//	                       appended as "LOAD <sessionId>" / "PROMPT <text>"
//	                       lines so tests can assert what the runtime sent
//	                       (ADR-0027 D1 wrap-up assertions). Session _meta is
//	                       appended as "NEW_META <json>" / "LOAD_META <json>"
//	                       so tests can assert the steering meta rides on both
//	                       NewSession and LoadSession (ADR-0020.2).
//	MOCKAGENT_REJECT_UNKNOWN_LOAD — when "1", LoadSession rejects any session
//	                       ID that was not created in this process with
//	                       Resource not found (-32002).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// promptText flattens the prompt request's text blocks for the audit log.
func promptText(params acp.PromptRequest) string {
	var sb strings.Builder
	for _, block := range params.Prompt {
		if block.Text != nil {
			sb.WriteString(block.Text.Text)
		}
	}
	return sb.String()
}

// reply is one scripted prompt response.
type reply struct {
	ToolCalls    []string `json:"toolCalls"`
	ReplayOnLoad []string `json:"replayOnLoad"`
	Text         string   `json:"text"`
	// SubmitReport, when set, is written verbatim to the report-sink path the
	// runtime registered on the session (ADR-0021 D1). It simulates the agent
	// calling the submit_report MCP tool with an accepted report, exercising the
	// runtime's sink-preference gate through the real production path.
	SubmitReport json.RawMessage `json:"submitReport,omitempty"`
	// PromptError, when set, fails the prompt request with a JSON-RPC error
	// carrying this message — e.g. the adapter's hard
	// "Reached maximum number of turns (24)" (ADR-0027 D1 test hook).
	PromptError string `json:"promptError,omitempty"`
	// HangMs, when > 0, sleeps this long inside Prompt while emitting nothing:
	// a scripted zombie for liveness-watchdog tests (ADR-0027 D2).
	HangMs int `json:"hangMs,omitempty"`
	// UpdateCount/UpdateIntervalMs emit that many message chunks paced at the
	// interval before the reply completes, proving that steady activity keeps
	// the watchdog quiet (ADR-0027 D2).
	UpdateCount      int `json:"updateCount,omitempty"`
	UpdateIntervalMs int `json:"updateIntervalMs,omitempty"`
	// ExitBeforeResponse, when true, kills the agent process mid-prompt: the
	// dead-peer case (ADR-0027 D2).
	ExitBeforeResponse bool `json:"exitBeforeResponse,omitempty"`
}

// logMeta appends a session _meta line ("NEW_META"/"LOAD_META") to the prompt
// log. Go marshals map keys sorted, so the logged JSON is deterministic and
// tests can compare the NewSession and LoadSession metas byte-for-byte.
func logMeta(kind string, meta map[string]any) {
	data, err := json.Marshal(meta)
	if err != nil {
		data = []byte("null")
	}
	logEvent(kind, string(data))
}

// logEvent appends one line to MOCKAGENT_PROMPT_LOG when configured.
func logEvent(kind, detail string) {
	path := os.Getenv("MOCKAGENT_PROMPT_LOG")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", kind, strings.ReplaceAll(detail, "\n", "\\n"))
}

// mockAgent implements acp.Agent and acp.AgentLoader with scripted behavior.
type mockAgent struct {
	conn     *acp.AgentSideConnection
	replies  []reply
	state    string
	sinkPath string
}

// captureSink records the report-sink --out path from a session's registered
// MCP servers so a scripted SubmitReport can be persisted there.
func (m *mockAgent) captureSink(servers []acp.McpServer) {
	for _, s := range servers {
		if s.Stdio == nil {
			continue
		}
		args := s.Stdio.Args
		for i, a := range args {
			if a == "--out" && i+1 < len(args) {
				m.sinkPath = args[i+1]
			}
		}
	}
}

// writeSubmitReport simulates an accepted submit_report tool call by writing
// the scripted report JSON to the captured sink path.
func (m *mockAgent) writeSubmitReport(r reply) {
	if len(r.SubmitReport) == 0 || m.sinkPath == "" {
		return
	}
	_ = os.WriteFile(m.sinkPath, r.SubmitReport, 0o644)
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

func (m *mockAgent) currentIndex() int {
	n := 0
	if data, err := os.ReadFile(m.state); err == nil {
		n, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	return n
}

func (m *mockAgent) Initialize(_ context.Context, _ acp.InitializeRequest) (acp.InitializeResponse, error) {
	return acp.InitializeResponse{
		ProtocolVersion:   acp.ProtocolVersionNumber,
		AgentCapabilities: acp.AgentCapabilities{LoadSession: true},
		AgentInfo:         &acp.Implementation{Name: "mockagent", Version: "0.0.1", Meta: map[string]any{"subjectModel": "mock-sonnet"}},
	}, nil
}

func (m *mockAgent) NewSession(_ context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	m.captureSink(params.McpServers)
	logMeta("NEW_META", params.Meta)
	return acp.NewSessionResponse{SessionId: "mock-sess-1"}, nil
}

// LoadSession accepts any session ID by default. When
// MOCKAGENT_REJECT_UNKNOWN_LOAD=1, it rejects stale IDs with the ACP
// Resource-not-found code, matching real adapters that do not know a persisted
// transport session from a previous process/binary. Scripted replay updates
// simulate ACP adapters that emit prior session history before the follow-up
// prompt request is sent.
func (m *mockAgent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	m.captureSink(params.McpServers)
	logEvent("LOAD", string(params.SessionId))
	logMeta("LOAD_META", params.Meta)
	if os.Getenv("MOCKAGENT_REJECT_UNKNOWN_LOAD") == "1" && string(params.SessionId) != "mock-sess-1" {
		return acp.LoadSessionResponse{}, &acp.RequestError{Code: -32002, Message: "Resource not found"}
	}
	idx := m.currentIndex()
	if idx >= len(m.replies) {
		idx = len(m.replies) - 1
	}
	if idx >= 0 {
		if err := m.emitToolCalls(ctx, params.SessionId, idx, "replay", m.replies[idx].ReplayOnLoad); err != nil {
			return acp.LoadSessionResponse{}, err
		}
		if len(m.replies[idx].ReplayOnLoad) > 0 {
			if err := m.conn.SessionUpdate(ctx, acp.SessionNotification{
				SessionId: params.SessionId,
				Update: acp.SessionUpdate{
					AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
						Content: acp.TextBlock("replayed prior answer"),
					},
				},
			}); err != nil {
				return acp.LoadSessionResponse{}, err
			}
		}
	}
	return acp.LoadSessionResponse{}, nil
}

func (m *mockAgent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	idx := m.nextIndex()
	if idx >= len(m.replies) {
		idx = len(m.replies) - 1
	}
	r := m.replies[idx]
	logEvent("PROMPT", promptText(params))

	// Scripted failure modes (ADR-0027 test hooks) run before any output.
	if r.HangMs > 0 {
		time.Sleep(time.Duration(r.HangMs) * time.Millisecond)
	}
	for i := 0; i < r.UpdateCount; i++ {
		if err := m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: params.SessionId,
			Update: acp.SessionUpdate{
				AgentMessageChunk: &acp.SessionUpdateAgentMessageChunk{
					Content: acp.TextBlock(fmt.Sprintf("tick %d ", i)),
				},
			},
		}); err != nil {
			return acp.PromptResponse{}, err
		}
		time.Sleep(time.Duration(r.UpdateIntervalMs) * time.Millisecond)
	}
	if r.ExitBeforeResponse {
		os.Exit(1)
	}
	if r.PromptError != "" {
		return acp.PromptResponse{}, errors.New(r.PromptError)
	}

	if err := m.emitToolCalls(ctx, params.SessionId, idx, "tc", r.ToolCalls); err != nil {
		return acp.PromptResponse{}, err
	}

	// Simulate an accepted submit_report tool call for this turn (ADR-0021 D1).
	m.writeSubmitReport(r)

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

func (m *mockAgent) emitToolCalls(ctx context.Context, sessionID acp.SessionId, idx int, prefix string, calls []string) error {
	for i, title := range calls {
		id := acp.ToolCallId(fmt.Sprintf("%s-%d-%d", prefix, idx, i))
		update := acp.SessionUpdate{
			ToolCall: &acp.SessionUpdateToolCall{
				ToolCallId: id,
				Title:      "Terminal",
				Kind:       acp.ToolKindExecute,
				Status:     acp.ToolCallStatusPending,
				RawInput:   map[string]any{"command": title},
			},
		}
		if err := m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: sessionID,
			Update:    update,
		}); err != nil {
			return err
		}
		status := acp.ToolCallStatusCompleted
		if err := m.conn.SessionUpdate(ctx, acp.SessionNotification{
			SessionId: sessionID,
			Update: acp.SessionUpdate{
				ToolCallUpdate: &acp.SessionToolCallUpdate{
					ToolCallId: id,
					Status:     &status,
					RawInput:   map[string]any{"command": title},
					Content:    []acp.ToolCallContent{acp.ToolContent(acp.TextBlock("mock output for " + title))},
				},
			},
		}); err != nil {
			return err
		}
	}
	return nil
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

// Logout satisfies the acp.Agent interface (promoted from unstable in v0.13.5).
func (m *mockAgent) Logout(_ context.Context, _ acp.LogoutRequest) (acp.LogoutResponse, error) {
	return acp.LogoutResponse{}, nil
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
