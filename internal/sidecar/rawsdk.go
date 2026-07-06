package sidecar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Raw-SDK audit channel (ADR-0016.10). Eval-mode sessions enable
// emitRawSDKMessages (ADR-0020.1 D5); claude-agent-acp then forwards every
// SDK message as the ACP extension notification "_claude/sdkMessage". The
// types here are the normalized, bounded per-turn persistence of that
// stream: enough to diff the raw tool-use/tool-result blocks against the
// captured ToolTraces (TRUST H3) and to persist provider-reported token
// usage (TRUST H5 hook), without duplicating text/thinking content that
// already lives on the turn.

// RawSDKMessageMethod is the ACP extension notification carrying one raw
// SDK message from the claude-agent-acp adapter (acp-agent.js:885-891).
const RawSDKMessageMethod = "_claude/sdkMessage"

// Pre-registered size bounds (ADR-0016.10 D1).
const (
	// rawSDKInputExcerptMax bounds each persisted tool-use input excerpt,
	// matching the existing tool-output excerpt bound in appendExcerpt.
	rawSDKInputExcerptMax = 2048
	// rawSDKEventCapPerTurn bounds tool-use + tool-result events per turn.
	// Events beyond the cap are dropped and counted; the comparator then
	// only reasons about the events present.
	rawSDKEventCapPerTurn = 1024
)

// RawSDKToolUse is one tool_use block observed on a raw SDK assistant
// message. ID is the SDK tool_use block id — the adapter reuses it verbatim
// as the ACP toolCallId (acp-agent.js toolCallNotification), so comparator
// matching is exact ID equality.
type RawSDKToolUse struct {
	// Index is the ordinal of the raw message this block arrived on,
	// within the turn's live raw stream (audit ordering).
	Index int `json:"index"`
	// ID is the SDK tool_use block id.
	ID string `json:"id"`
	// Name is the SDK tool name (e.g. "Bash", "Read", "TodoWrite").
	Name string `json:"name,omitempty"`
	// InputDigest is sha256 hex of the canonical JSON of the decoded input
	// (encoding/json marshals map keys sorted, so both comparison sides
	// digest identically after a JSON round trip). Empty when the block
	// carried no input.
	InputDigest string `json:"inputDigest,omitempty"`
	// InputExcerpt is the canonical input JSON bounded to
	// rawSDKInputExcerptMax bytes, for human audit of digest mismatches.
	InputExcerpt string `json:"inputExcerpt,omitempty"`
	// ParentToolUseID is the message-level parent_tool_use_id: non-empty
	// for subagent-originated messages.
	ParentToolUseID string `json:"parentToolUseId,omitempty"`
}

// RawSDKToolResult is one tool_result block observed on a raw SDK user
// message — the provider-side terminal record for a tool call.
type RawSDKToolResult struct {
	// ToolUseID links back to the tool_use block (and the ACP toolCallId).
	ToolUseID string `json:"toolUseId"`
	// IsError mirrors the block's is_error flag.
	IsError bool `json:"isError,omitempty"`
	// OutputSize is the character size of the result content (string
	// length, or summed text-block lengths). The content itself is not
	// persisted.
	OutputSize int `json:"outputSize"`
}

// RawSDKUsage is provider-reported token usage for one turn (ADR-0016.10
// D6, TRUST H5 hook). Inert persistence: no scorer reads it.
type RawSDKUsage struct {
	InputTokens              int `json:"inputTokens"`
	OutputTokens             int `json:"outputTokens"`
	CacheReadInputTokens     int `json:"cacheReadInputTokens,omitempty"`
	CacheCreationInputTokens int `json:"cacheCreationInputTokens,omitempty"`
	// CostUSD is total_cost_usd from the terminal result message; zero when
	// the turn never produced one.
	CostUSD float64 `json:"costUsd,omitempty"`
	// Source records which provider signal the numbers hold: "result" (the
	// query-cumulative usage on the terminal result message) or
	// "assistant_sum" (per-API-call assistant usages summed, when no result
	// message was observed — e.g. a cancelled turn).
	Source string `json:"source,omitempty"`
}

// RawSDKAudit is the per-turn normalized raw-SDK audit record
// (ADR-0016.10 D1). Persisted inside the episode artifact as
// TurnRecord.RawSDK so the trace-capture comparator stays pure over one
// committed file.
type RawSDKAudit struct {
	// Messages counts live raw SDK messages observed this turn — the audit
	// denominator. Zero means the channel never opened; the comparator
	// must skip such turns.
	Messages int `json:"messages"`
	// ReplayedMessages counts raw messages dropped because they arrived
	// before the turn's prompt was sent. The adapter's replay path emits no
	// raw messages (acp-agent.js replaySessionHistory), so this is
	// defensive; non-zero values are themselves an audit signal.
	ReplayedMessages int                `json:"replayedMessages,omitempty"`
	ToolUses         []RawSDKToolUse    `json:"toolUses,omitempty"`
	ToolResults      []RawSDKToolResult `json:"toolResults,omitempty"`
	Usage            *RawSDKUsage       `json:"usage,omitempty"`
	// Truncated is true when the pre-registered per-turn event cap dropped
	// events; DroppedEvents counts them.
	Truncated     bool `json:"truncated,omitempty"`
	DroppedEvents int  `json:"droppedEvents,omitempty"`
}

// CanonicalJSONDigest returns the sha256 hex of json.Marshal(v). Both the
// raw SDK input and the captured ACP RawInput pass through the same
// JSON-decode/-marshal round trip (Go sorts map keys), so equal values
// digest equally. Returns "" for nil or unmarshalable values.
func CanonicalJSONDigest(v any) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// rawSDKNotification is the wire shape of a _claude/sdkMessage params
// object: {sessionId, message}.
type rawSDKNotification struct {
	Message json.RawMessage `json:"message"`
}

// rawSDKMessage is the subset of an SDK message envelope the audit reads.
type rawSDKMessage struct {
	Type            string          `json:"type"`
	ParentToolUseID string          `json:"parent_tool_use_id"`
	Message         json.RawMessage `json:"message"`
	Usage           *rawSDKAPIUsage `json:"usage"`          // result messages
	TotalCostUSD    float64         `json:"total_cost_usd"` // result messages
}

// rawSDKInnerMessage is the API message wrapped by assistant/user SDK
// messages.
type rawSDKInnerMessage struct {
	Content json.RawMessage `json:"content"`
	Usage   *rawSDKAPIUsage `json:"usage"`
}

// rawSDKAPIUsage matches the Anthropic API usage shape.
type rawSDKAPIUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

// rawSDKContentBlock is the subset of an API content block the audit reads.
type rawSDKContentBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`          // tool_use
	Name      string          `json:"name"`        // tool_use
	Input     any             `json:"input"`       // tool_use
	ToolUseID string          `json:"tool_use_id"` // tool_result
	IsError   bool            `json:"is_error"`    // tool_result
	Content   json.RawMessage `json:"content"`     // tool_result
}

// Record parses one _claude/sdkMessage notification payload into the audit.
// replayed marks payloads that arrived before the turn's prompt was sent;
// they are dropped and counted, mirroring the replayed-vs-live tool-trace
// split (ADR-0016.5). Unparsable payloads still count toward Messages so
// the denominator never understates what the adapter emitted.
func (a *RawSDKAudit) Record(params json.RawMessage, replayed bool) {
	if replayed {
		a.ReplayedMessages++
		return
	}
	a.Messages++

	var note rawSDKNotification
	if err := json.Unmarshal(params, &note); err != nil || len(note.Message) == 0 {
		return
	}
	var msg rawSDKMessage
	if err := json.Unmarshal(note.Message, &msg); err != nil {
		return
	}
	switch msg.Type {
	case "assistant":
		a.recordAssistant(msg)
	case "user":
		a.recordUser(msg)
	case "result":
		a.recordResult(msg)
		// system, stream_event, tool_progress, ...: counted, no tool events.
		// stream_event partials are deliberately ignored — tool_use blocks
		// arrive complete on the consolidated assistant message, and reading
		// both would double-count (ADR-0016.10 D1).
	}
}

func (a *RawSDKAudit) recordAssistant(msg rawSDKMessage) {
	var inner rawSDKInnerMessage
	if err := json.Unmarshal(msg.Message, &inner); err != nil {
		return
	}
	index := a.Messages - 1
	for _, block := range decodeContentBlocks(inner.Content) {
		if block.Type != "tool_use" && block.Type != "server_tool_use" && block.Type != "mcp_tool_use" {
			continue
		}
		use := RawSDKToolUse{
			Index:           index,
			ID:              block.ID,
			Name:            block.Name,
			ParentToolUseID: msg.ParentToolUseID,
		}
		if block.Input != nil {
			if data, err := json.Marshal(block.Input); err == nil && string(data) != "null" {
				sum := sha256.Sum256(data)
				use.InputDigest = hex.EncodeToString(sum[:])
				excerpt := string(data)
				if len(excerpt) > rawSDKInputExcerptMax {
					excerpt = excerpt[:rawSDKInputExcerptMax]
				}
				use.InputExcerpt = excerpt
			}
		}
		if !a.appendEvent() {
			return
		}
		a.ToolUses = append(a.ToolUses, use)
	}
	if inner.Usage != nil {
		a.accumulateAssistantUsage(*inner.Usage)
	}
}

func (a *RawSDKAudit) recordUser(msg rawSDKMessage) {
	var inner rawSDKInnerMessage
	if err := json.Unmarshal(msg.Message, &inner); err != nil {
		return
	}
	for _, block := range decodeContentBlocks(inner.Content) {
		if block.Type != "tool_result" {
			continue
		}
		if !a.appendEvent() {
			return
		}
		a.ToolResults = append(a.ToolResults, RawSDKToolResult{
			ToolUseID:  block.ToolUseID,
			IsError:    block.IsError,
			OutputSize: contentBlockSize(block.Content),
		})
	}
}

func (a *RawSDKAudit) recordResult(msg rawSDKMessage) {
	if msg.Usage == nil {
		return
	}
	// The terminal result usage is query-cumulative and authoritative; it
	// overwrites any assistant-usage sum (ADR-0016.10 D6).
	a.Usage = &RawSDKUsage{
		InputTokens:              msg.Usage.InputTokens,
		OutputTokens:             msg.Usage.OutputTokens,
		CacheReadInputTokens:     msg.Usage.CacheReadInputTokens,
		CacheCreationInputTokens: msg.Usage.CacheCreationInputTokens,
		CostUSD:                  msg.TotalCostUSD,
		Source:                   "result",
	}
}

func (a *RawSDKAudit) accumulateAssistantUsage(u rawSDKAPIUsage) {
	if a.Usage != nil && a.Usage.Source == "result" {
		return // a terminal result already pinned the authoritative numbers
	}
	if a.Usage == nil {
		a.Usage = &RawSDKUsage{Source: "assistant_sum"}
	}
	a.Usage.InputTokens += u.InputTokens
	a.Usage.OutputTokens += u.OutputTokens
	a.Usage.CacheReadInputTokens += u.CacheReadInputTokens
	a.Usage.CacheCreationInputTokens += u.CacheCreationInputTokens
}

// appendEvent enforces the pre-registered per-turn event cap. It returns
// false — and records the drop — when the cap is reached.
func (a *RawSDKAudit) appendEvent() bool {
	if len(a.ToolUses)+len(a.ToolResults) >= rawSDKEventCapPerTurn {
		a.Truncated = true
		a.DroppedEvents++
		return false
	}
	return true
}

// Merge folds another turn attempt's raw audit into this one — the raw
// twin of mergeRetryTurn/mergeWrapUpTurn appending ToolTraces: counts sum,
// events append (re-capped), and the latest terminal usage wins.
func (a *RawSDKAudit) Merge(src *RawSDKAudit) {
	if src == nil {
		return
	}
	a.Messages += src.Messages
	a.ReplayedMessages += src.ReplayedMessages
	a.DroppedEvents += src.DroppedEvents
	a.Truncated = a.Truncated || src.Truncated
	for _, use := range src.ToolUses {
		if !a.appendEvent() {
			break
		}
		a.ToolUses = append(a.ToolUses, use)
	}
	for _, res := range src.ToolResults {
		if !a.appendEvent() {
			break
		}
		a.ToolResults = append(a.ToolResults, res)
	}
	if src.Usage != nil {
		if src.Usage.Source == "result" || a.Usage == nil {
			a.Usage = src.Usage
		}
	}
}

// decodeContentBlocks decodes an API message content field, which is either
// a plain string (no blocks) or an array of content blocks.
func decodeContentBlocks(raw json.RawMessage) []rawSDKContentBlock {
	if len(raw) == 0 {
		return nil
	}
	var blocks []rawSDKContentBlock
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return nil // plain-string content — no tool blocks
	}
	return blocks
}

// contentBlockSize sizes a tool_result content field: string length, or the
// summed text lengths of its blocks.
func contentBlockSize(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return len(s)
	}
	var blocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return 0
	}
	n := 0
	for _, b := range blocks {
		n += len(b.Text)
	}
	return n
}

// NOTE: resumed turns keep the audit channel because LoadSession forwards the
// full session meta — the same BuildSessionMeta bag NewSession sends, whose
// eval-mode sibling flag is emitRawSDKMessages (ADR-0020.2, superseding the
// raw-channel-only subset from ADR-0016.10 D2).
