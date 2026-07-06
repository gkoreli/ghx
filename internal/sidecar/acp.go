package sidecar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// TurnResult holds the streamed output of one ACP prompt turn.
type TurnResult struct {
	// FullText is the complete text emitted by the agent.
	FullText string
	// Thinking is the internal reasoning streamed by ACP agent_thought_chunk
	// updates during this prompt turn.
	Thinking string
	// ReplayedText is message text replayed before this turn's prompt was sent.
	// It is retained for audit and excluded from turn output/accounting.
	ReplayedText string
	// ReplayedThinking is reasoning replayed before this turn's prompt was sent.
	// It is retained for audit and excluded from live turn telemetry.
	ReplayedThinking string
	// ToolCalls lists the tool calls observed during the turn (kind: command + status).
	ToolCalls []string
	// ToolTraces records full per-call audit data for eval/training artifacts.
	ToolTraces []ToolCallTrace
	// ReplayedToolTraces records tool history replayed before this turn's
	// prompt was sent. These traces are audit-only and excluded from turn
	// activity, ledger derivation, repeat-read scoring, and char accounting.
	ReplayedToolTraces []ToolCallTrace
	AgentInfo          *ImplementationInfo
	// ToolOutputChars approximates the size of tool outputs the agent
	// consumed (content blocks and raw output on tool_call/tool_call_update
	// events). Produced text alone understates context burden; this is the
	// other half.
	ToolOutputChars int
	// ReportRetried is true when the turn needed at least one corrective
	// follow-up (ADR-0016.7 / ADR-0021 D2) to obtain a usable report. Recorded
	// so evals can count retries honestly instead of hiding them.
	ReportRetried bool
	// ReportCoerced is true when the final report was obtained only via the
	// lenient <ghx-report> coercion fallback (ADR-0021 D3), i.e. the producer's
	// JSON did not fit the schema and had to be normalized. Surfaced so evals
	// count every coercion as a soft anomaly instead of silently absorbing
	// producer drift (Visibility and Truthfulness). Always false when the report
	// came from the strict submit_report sink.
	ReportCoerced bool
	// WrapUpRecovered is true when the turn hit the adapter's max-turns safety
	// net and the exploration was recovered by the one-shot LoadSession wrap-up
	// prompt (ADR-0027 D1). Recorded on the eval turn record and counted as the
	// soft anomaly turn_cap_wrapup so evals can see how often the net fires.
	WrapUpRecovered bool
	// SessionRecreated is true when a persisted ACP session ID was stale and
	// LoadSession returned Resource not found; Ask created one fresh ACP session
	// and continued the turn using the durable ledger context in the prompt.
	SessionRecreated bool
	// Artifacts points at the session's persisted audit trail for the ask this
	// turn belongs to (session dir + root trace ID). Populated by Ask after
	// artifact emission; zero for a bare RunTurnWithOptions result.
	Artifacts ArtifactsRef
}

// ImplementationInfo records the ACP adapter identity returned by initialize.
type ImplementationInfo struct {
	Name    string
	Version string
	Meta    map[string]any
}

const defaultHandshakeTimeout = 10 * time.Second

// splitAgentCmd splits a configured agent command into argv. The config value
// may be a bare binary name ("claude") or a whitespace-separated command line
// ("npx -y @agentclientprotocol/claude-agent-acp@0.55.0" — what `config init
// --claude-acp` writes), so first-time setup does not require hand-writing a
// wrapper script (dogfood friction 2026-07-05). Splitting is plain field
// splitting with no shell quoting; a binary path containing spaces still
// needs a wrapper script.
func splitAgentCmd(agentCmd string) (name string, args []string) {
	fields := strings.Fields(agentCmd)
	if len(fields) == 0 {
		return agentCmd, nil
	}
	return fields[0], fields[1:]
}

// Runtime resilience taxonomy (ADR-0027). These markers are the stable,
// cross-package contract for classifying turn failures: the runtime uses them
// to pick a recovery path (D1 wrap-up, D2 fail-fast), and the eval layer uses
// them to derive anomalies (turn_cap_wrapup, episode_hang_timeout) from the
// persisted turn error strings without importing runtime internals.
const (
	// LivenessTimeoutEnv configures the D2 liveness watchdog window. Accepts a
	// Go duration ("10m", "90s"); "0" disables the watchdog. Default
	// DefaultLivenessTimeout.
	LivenessTimeoutEnv = "GHX_SIDECAR_LIVENESS_TIMEOUT"
	// DefaultLivenessTimeout is the watchdog window when the env var is unset:
	// no ACP session update for this long cancels the turn (ADR-0027 D2).
	DefaultLivenessTimeout = 10 * time.Minute

	// LivenessTimeoutMarker appears in every watchdog-cancelled turn error so
	// artifact-only consumers (evals anomaly detection) can classify the
	// failure from the persisted string alone.
	LivenessTimeoutMarker = "liveness watchdog timeout"
	// maxTurnsErrorMarker is the adapter's hard turn-cap error text
	// (claude-agent-acp: "Reached maximum number of turns (N)").
	maxTurnsErrorMarker = "Reached maximum number of turns"
	// peerClosedMarker matches the ACP SDK's dead-peer cause ("peer connection
	// closed", connection.go) — "peer disconnected" is its request-level twin.
	peerClosedMarker = "peer connection closed"
	// resourceNotFoundCode is the JSON-RPC/ACP code adapters use when
	// LoadSession cannot find the persisted transport session ID.
	resourceNotFoundCode = -32002
)

// ErrLivenessTimeout is the sentinel for a turn cancelled by the D2 liveness
// watchdog. Wrapped errors carry the concrete window; match with errors.Is or,
// from persisted strings, with LivenessTimeoutMarker.
var ErrLivenessTimeout = errors.New("sidecar " + LivenessTimeoutMarker)

// IsMaxTurnsError reports whether err is the adapter's hard turn-cap error
// (ADR-0027 D1 recovery trigger). Matching is textual because the error
// arrives as an opaque JSON-RPC internal error from the adapter.
func IsMaxTurnsError(err error) bool {
	return err != nil && strings.Contains(err.Error(), maxTurnsErrorMarker)
}

// IsPeerClosedError reports whether err means the ACP peer died (process exit,
// closed stdio). Dead peers fail the turn immediately (ADR-0027 D2).
func IsPeerClosedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, peerClosedMarker) || strings.Contains(msg, "peer disconnected")
}

// IsLoadSessionResourceNotFound reports whether err is the stale-session miss
// that may be downgraded to a fresh ACP NewSession. It deliberately requires
// both the JSON-RPC resource-not-found code and the human message so unrelated
// LoadSession failures still fail loudly.
func IsLoadSessionResourceNotFound(err error) bool {
	if err == nil {
		return false
	}
	var reqErr *acp.RequestError
	if errors.As(err, &reqErr) {
		return reqErr.Code == resourceNotFoundCode && strings.Contains(strings.ToLower(reqErr.Message), "resource not found")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, fmt.Sprintf("%d", resourceNotFoundCode)) && strings.Contains(msg, "resource not found")
}

// resolveLivenessTimeout resolves the D2 watchdog window: an explicit option
// wins, then GHX_SIDECAR_LIVENESS_TIMEOUT, then DefaultLivenessTimeout.
// A zero (or negative) resolved value disables the watchdog.
func resolveLivenessTimeout(explicit time.Duration) time.Duration {
	if explicit != 0 {
		return explicit
	}
	raw := strings.TrimSpace(os.Getenv(LivenessTimeoutEnv))
	if raw == "" {
		return DefaultLivenessTimeout
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid %s=%q (want a Go duration like \"10m\"); using default %s\n",
			LivenessTimeoutEnv, raw, DefaultLivenessTimeout)
		return DefaultLivenessTimeout
	}
	return d
}

// ACPHandshakeFailureMessage is the actionable failure text shared by ask,
// doctor, and config init when a configured binary exists but does not speak
// ACP initialize on stdio.
func ACPHandshakeFailureMessage(agentCmd string) string {
	return fmt.Sprintf("agent %s did not complete the ACP handshake; run `ghx sidecar config init` or set an ACP-capable agent", agentCmd)
}

// ToolStatusTransition records one observed ACP tool-call status.
type ToolStatusTransition struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

// ToolCallTrace is the full per-tool-call audit record captured from ACP
// tool_call and tool_call_update notifications.
type ToolCallTrace struct {
	ID                string                 `json:"id"`
	Kind              string                 `json:"kind,omitempty"`
	Title             string                 `json:"title,omitempty"`
	RawInput          any                    `json:"rawInput,omitempty"`
	StatusTransitions []ToolStatusTransition `json:"statusTransitions,omitempty"`
	OutputSize        int                    `json:"outputSize"`
	OutputExcerpt     string                 `json:"outputExcerpt,omitempty"`
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

// ToolOutputText returns a bounded textual representation of tool output.
// ContentSize remains the exact accounting source; this is only for audit
// excerpts in local eval artifacts.
func ToolOutputText(content []acp.ToolCallContent, rawOutput any) string {
	var out string
	for _, c := range content {
		if c.Content != nil && c.Content.Content.Text != nil {
			out += c.Content.Content.Text.Text
		} else if data, err := json.Marshal(c); err == nil {
			out += string(data)
		}
	}
	switch v := rawOutput.(type) {
	case nil:
	case string:
		out += v
	default:
		if data, err := json.Marshal(v); err == nil {
			out += string(data)
		}
	}
	return out
}

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
}

// markPromptSent flips replay accounting to live-turn accounting.
func (c *denyClient) markPromptSent() {
	c.mu.Lock()
	c.promptSent = true
	c.mu.Unlock()
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

func (c *denyClient) refreshToolSummaries() {
	c.result.ToolCalls = c.result.ToolCalls[:0]
	for _, tr := range c.result.ToolTraces {
		c.result.ToolCalls = append(c.result.ToolCalls, toolSummary(tr))
	}
}

func toolSummary(tr ToolCallTrace) string {
	status := ""
	if n := len(tr.StatusTransitions); n > 0 {
		status = tr.StatusTransitions[n-1].Status
	}
	fallback := tr.Title
	if fallback == "" {
		fallback = tr.ID
	}
	input := resolveToolInput(tr.RawInput, fallback)
	if tr.Kind != "" {
		input = tr.Kind + ": " + input
	}
	if status != "" {
		return fmt.Sprintf("%s (%s)", input, status)
	}
	return input
}

func resolveToolInput(raw any, fallback string) string {
	switch v := raw.(type) {
	case string:
		if v != "" {
			return v
		}
	case map[string]any:
		if s, ok := v["command"].(string); ok && s != "" {
			return commandWithArgs(s, v["arguments"])
		}
		if s, ok := v["cmd"].(string); ok && s != "" {
			return commandWithArgs(s, v["args"])
		}
		if len(v) > 0 {
			data, err := json.Marshal(v)
			if err == nil && string(data) != "{}" {
				return string(data)
			}
		}
	case nil:
		// Fall through to fallback below.
	default:
		if raw != nil {
			if data, err := json.Marshal(raw); err == nil && string(data) != "{}" {
				return string(data)
			}
		}
	}
	return fallback
}

func commandWithArgs(command string, rawArgs any) string {
	var parts []string
	switch args := rawArgs.(type) {
	case []string:
		parts = args
	case []any:
		for _, a := range args {
			parts = append(parts, fmt.Sprint(a))
		}
	case string:
		if args != "" {
			parts = append(parts, args)
		}
	}
	if len(parts) == 0 {
		return command
	}
	return command + " " + strings.Join(parts, " ")
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
		}
	case u.AgentThoughtChunk != nil:
		if u.AgentThoughtChunk.Content.Text != nil {
			text := u.AgentThoughtChunk.Content.Text.Text
			if replayed {
				c.result.ReplayedThinking += text
				return nil
			}
			c.result.Thinking += text
		}
	case u.ToolCall != nil:
		tc := u.ToolCall
		tr := c.upsertTrace(string(tc.ToolCallId), replayed)
		tr.Title = tc.Title
		tr.Kind = string(tc.Kind)
		if tc.RawInput != nil {
			tr.RawInput = tc.RawInput
		}
		tr.StatusTransitions = append(tr.StatusTransitions, ToolStatusTransition{Status: string(tc.Status), At: time.Now().UTC()})
		size := ContentSize(tc.Content, tc.RawOutput)
		tr.OutputSize += size
		appendExcerpt(tr, ToolOutputText(tc.Content, tc.RawOutput))
		if !replayed {
			c.result.ToolOutputChars += size
			c.refreshToolSummaries()
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
		if tcu.Status != nil {
			tr.StatusTransitions = append(tr.StatusTransitions, ToolStatusTransition{Status: string(*tcu.Status), At: time.Now().UTC()})
		}
		size := ContentSize(tcu.Content, tcu.RawOutput)
		tr.OutputSize += size
		appendExcerpt(tr, ToolOutputText(tcu.Content, tcu.RawOutput))
		if !replayed {
			c.result.ToolOutputChars += size
			c.refreshToolSummaries()
		}
	}
	return nil
}

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

// RunTurnOptions configures one ACP prompt turn.
type RunTurnOptions struct {
	AgentCmd     string
	ACPSessionID string
	Prompt       string
	Cwd          string
	Env          []string
	// SessionMeta is the _meta map forwarded to the adapter on session creation
	// (NewSession only — LoadSession and retry turns do not re-create the
	// session). Nil means no steering options. Populated by Ask via
	// BuildSessionMeta (ADR-0020.1 D2).
	SessionMeta map[string]any
	// ReportSinkPath, when non-empty, registers the session-scoped report-sink
	// MCP server (ADR-0021 D1). The adapter spawns `<this executable> sidecar
	// report-sink --out <ReportSinkPath>` and exposes its submit_report tool to
	// the model; the runtime reads the accepted report back from this path after
	// the turn. Registered on both NewSession and LoadSession so resumed turns
	// keep the tool.
	ReportSinkPath string
	// LivenessTimeout overrides the D2 watchdog window for this turn. Zero
	// means "resolve from GHX_SIDECAR_LIVENESS_TIMEOUT (default 10m)";
	// negative disables the watchdog explicitly.
	LivenessTimeout time.Duration
}

// ResolveReportSinkExe returns the ghx executable that will serve the
// session-scoped report-sink MCP server (`sidecar report-sink`), plus a human
// label for where the decision came from. This is the single resolution
// authority — reportSinkMcpServers (the ACP wiring) and the doctor
// report-sink-version check both use it, so what doctor diagnoses is exactly
// what a session would spawn. Order: GHX_REPORT_SINK_EXE when set, else the
// current executable. Errors cover the unusable cases: os.Executable failure
// and running under `go test` (the test binary cannot serve `sidecar
// report-sink`).
func ResolveReportSinkExe() (exe string, source string, err error) {
	if exe := os.Getenv("GHX_REPORT_SINK_EXE"); exe != "" {
		return exe, "GHX_REPORT_SINK_EXE", nil
	}
	exe, resolveErr := os.Executable()
	if resolveErr != nil || exe == "" {
		return "", "current executable", fmt.Errorf("cannot resolve executable: %v", resolveErr)
	}
	if strings.HasSuffix(exe, ".test") {
		return exe, "current executable", fmt.Errorf("running under go test (%s); set GHX_REPORT_SINK_EXE to a built ghx binary", exe)
	}
	return exe, "current executable", nil
}

// reportSinkMcpServers returns the ACP McpServer list to register for a turn.
// When a report-sink path is set it registers the ghx-report-sink stdio server,
// pointing the adapter at this same executable (os.Executable), or at
// GHX_REPORT_SINK_EXE when set. If no usable executable can be resolved the
// list is empty and the runtime falls back to the <ghx-report> text path
// (ADR-0021 D3) — the feature degrades, it does not break.
//
// GHX_REPORT_SINK_EXE exists because os.Executable is only correct when the
// runtime runs inside a real ghx binary. Under `go test` (live eval episodes,
// tags=agent_e2e) it resolves to the compiled test binary, which cannot serve
// `sidecar report-sink` — and the PATH ghx may be an older release without the
// command. Eval runs must build a fresh ghx and point this env var at it, or
// live episodes silently degrade to the text fallback and never exercise the
// submit_report contract.
func reportSinkMcpServers(sinkPath string) []acp.McpServer {
	if sinkPath == "" {
		return []acp.McpServer{}
	}
	exe, _, err := ResolveReportSinkExe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: report-sink disabled (%v)\n", err)
		return []acp.McpServer{}
	}
	return []acp.McpServer{{
		Stdio: &acp.McpServerStdio{
			Name:    ReportSinkServerName,
			Command: exe,
			Args:    []string{"sidecar", "report-sink", "--out", sinkPath},
			Env:     []acp.EnvVariable{},
		},
	}}
}

// RunTurn spawns the agent binary, establishes an ACP session (new or resumed),
// sends prompt, streams output, and returns the collected TurnResult and the
// ACP session ID to persist for follow-up turns.
//
// Pass acpSessionID="" on the first turn; pass the returned sessionID on
// subsequent turns to resume the agent's context.
func RunTurn(ctx context.Context, agentCmd, acpSessionID, prompt string) (result TurnResult, newSessionID string, err error) {
	return RunTurnWithOptions(ctx, RunTurnOptions{AgentCmd: agentCmd, ACPSessionID: acpSessionID, Prompt: prompt})
}

// RunTurnWithOptions is RunTurn plus eval/runtime overrides for cwd and env.
//
// Resilience (ADR-0027 D2): the turn runs under a cancellable wait — the
// prompt is awaited in a select over completion, parent-context cancellation,
// the liveness watchdog, and the peer connection dying. A dead peer fails the
// turn immediately; silence beyond the liveness window cancels it with
// ErrLivenessTimeout. On failure the partial TurnResult AND the established
// ACP session ID are still returned so callers can flush artifacts (D3) and
// resume the same session for the wrap-up recovery prompt (D1).
func RunTurnWithOptions(ctx context.Context, opts RunTurnOptions) (result TurnResult, newSessionID string, err error) {
	liveness := resolveLivenessTimeout(opts.LivenessTimeout)
	turnCtx, cancelTurn := context.WithCancelCause(ctx)
	defer cancelTurn(nil)

	agentBin, agentArgs := splitAgentCmd(opts.AgentCmd)
	cmd := exec.CommandContext(turnCtx, agentBin, agentArgs...)
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
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
		return result, "", fmt.Errorf("start %q: %w", opts.AgentCmd, err)
	}
	defer ShutdownAgent(cmd, stdin)

	// Liveness clock: every ACP session update (and every completed RPC step)
	// counts as a sign of life. The watchdog cancels the turn — which also
	// kills the spawned agent via exec.CommandContext — when the clock goes
	// stale for the whole window (ADR-0027 D2).
	var lastActivity atomic.Int64
	lastActivity.Store(time.Now().UnixNano())
	touch := func() { lastActivity.Store(time.Now().UnixNano()) }

	client := &denyClient{result: &result, onActivity: touch}
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	if liveness > 0 {
		go func() {
			for {
				idle := time.Duration(time.Now().UnixNano() - lastActivity.Load())
				remaining := liveness - idle
				if remaining <= 0 {
					cancelTurn(fmt.Errorf("%w: no ACP session update for %s (set %s to adjust)",
						ErrLivenessTimeout, liveness, LivenessTimeoutEnv))
					return
				}
				select {
				case <-turnCtx.Done():
					return
				case <-time.After(remaining):
				}
			}
		}()
	}

	cwd := opts.Cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	initResp, err := conn.Initialize(turnCtx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	})
	if err != nil {
		return result, "", fmt.Errorf("acp initialize: %w", turnFailureCause(turnCtx, err))
	}
	touch()
	if initResp.AgentInfo != nil {
		result.AgentInfo = &ImplementationInfo{
			Name:    initResp.AgentInfo.Name,
			Version: initResp.AgentInfo.Version,
			Meta:    initResp.AgentInfo.Meta,
		}
	}

	var sessionID acp.SessionId
	if opts.ACPSessionID == "" || !initResp.AgentCapabilities.LoadSession {
		// New session: forward the session-level steering meta (ADR-0020.1 D2).
		// The _meta bag is adapter-specific; the ACP spec treats it as opaque.
		req := acp.NewSessionRequest{
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
		}
		if opts.SessionMeta != nil {
			req.Meta = opts.SessionMeta
		}
		resp, err := conn.NewSession(turnCtx, req)
		if err != nil {
			return result, "", fmt.Errorf("acp new session: %w", turnFailureCause(turnCtx, err))
		}
		sessionID = resp.SessionId
	} else {
		_, err := conn.LoadSession(turnCtx, acp.LoadSessionRequest{
			SessionId:  acp.SessionId(opts.ACPSessionID),
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
		})
		if err != nil {
			return result, "", fmt.Errorf("acp load session: %w", turnFailureCause(turnCtx, err))
		}
		sessionID = acp.SessionId(opts.ACPSessionID)
	}
	touch()

	client.markPromptSent()
	promptDone := make(chan error, 1)
	go func() {
		_, perr := conn.Prompt(turnCtx, acp.PromptRequest{
			SessionId: sessionID,
			Prompt:    []acp.ContentBlock{acp.TextBlock(opts.Prompt)},
		})
		promptDone <- perr
	}()

	var promptErr error
	select {
	case promptErr = <-promptDone:
	case <-conn.Done():
		// Dead peer: fail the turn immediately (ADR-0027 D2) — never wait for
		// a timer on a connection that cannot answer. Give the in-flight
		// Prompt call a short grace to unwind first: when both channels are
		// ready (agent answered and then exited) the completed prompt wins.
		perr, finished := awaitPromptOrGrace(promptDone)
		if finished {
			promptErr = perr
		} else {
			promptErr = errors.New(peerClosedMarker + " before the prompt turn completed")
		}
		if promptErr != nil && !IsPeerClosedError(promptErr) {
			promptErr = fmt.Errorf("%s: %w", peerClosedMarker, promptErr)
		}
	case <-turnCtx.Done():
		// Watchdog or parent cancellation. The SDK's own wait also selects on
		// this context; the grace only covers its unwinding.
		if perr, finished := awaitPromptOrGrace(promptDone); finished {
			promptErr = perr
		} else {
			promptErr = context.Cause(turnCtx)
		}
	}
	// Prefer the watchdog cause over the SDK's stringified rendering of it so
	// errors.Is(err, ErrLivenessTimeout) works for callers.
	if cause := context.Cause(turnCtx); errors.Is(cause, ErrLivenessTimeout) {
		promptErr = cause
	}
	// Freeze the result before returning: late notifications from a dying
	// peer must not race the caller's reads (D3 flushes partial results).
	result = client.closeAndSnapshot()
	if promptErr != nil {
		return result, string(sessionID), fmt.Errorf("acp prompt: %w", promptErr)
	}

	os.Stdout.WriteString("\n")
	return result, string(sessionID), nil
}

// awaitPromptOrGrace waits briefly for an abandoned Prompt call to unwind.
// It returns the prompt's error and whether the prompt finished within the
// grace window (a finished prompt with a nil error is a genuine success even
// when the peer exited right behind it).
func awaitPromptOrGrace(promptDone <-chan error) (error, bool) {
	select {
	case err := <-promptDone:
		return err, true
	case <-time.After(shutdownGrace):
		return nil, false
	}
}

// turnFailureCause maps an RPC error to the turn-level cause when the turn
// context was cancelled by the liveness watchdog, so pre-prompt hangs
// (initialize / session setup) classify the same way as prompt hangs.
func turnFailureCause(turnCtx context.Context, err error) error {
	if cause := context.Cause(turnCtx); errors.Is(cause, ErrLivenessTimeout) {
		return cause
	}
	return err
}

// CheckACPHandshake spawns an agent and verifies that ACP initialize completes
// over stdio before the timeout. It intentionally stops after initialize:
// preflight needs protocol compatibility, not a working prompt turn.
func CheckACPHandshake(ctx context.Context, agentCmd, cwd string, env []string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = defaultHandshakeTimeout
	}
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	agentBin, agentArgs := splitAgentCmd(agentCmd)
	cmd := exec.CommandContext(tctx, agentBin, agentArgs...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if env != nil {
		cmd.Env = env
	}
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", agentCmd, err)
	}
	defer ShutdownAgent(cmd, stdin)

	result := TurnResult{}
	conn := acp.NewClientSideConnection(&denyClient{result: &result}, stdin, stdout)
	if _, err := conn.Initialize(tctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	}); err != nil {
		if errors.Is(tctx.Err(), context.DeadlineExceeded) {
			return errors.New(ACPHandshakeFailureMessage(agentCmd))
		}
		return fmt.Errorf("%s: %w", ACPHandshakeFailureMessage(agentCmd), err)
	}
	return nil
}
