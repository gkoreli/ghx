package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Live turn log (ADR-0022.1). During a multi-minute agent turn the session
// directory shows nothing until emitTurnArtifacts flushes traces/logs/metrics
// at Ask exit — the founder requirement (Goga, 2026-07-06) is that events
// append to a file AS THEY HAPPEN. LiveLog is that file: an append-only NDJSON
// stream at <sessionDir>/live.jsonl, written by the ACP client as session
// updates arrive and by the runtime at turn boundaries, so
// `tail -f ~/.ghx/sessions/<name>/live.jsonl` watches a turn in realtime.
//
// It is a progress surface, not an audit artifact: full text and tool traces
// still land in the report and the spec-exact OTLP files at end of turn
// (ADR-0022), which this file never replaces or mutates.

// LiveLogName is the per-session realtime turn activity file. It lives beside
// traces.jsonl; unlike the OTLP artifacts it is appended as events happen.
const LiveLogName = "live.jsonl"

// LiveLogPath returns the per-session live turn log path, mirroring
// AgentStderrLogPath (ADR-0033 D2) for the realtime activity stream.
func LiveLogPath(sessionsDir, name string) string {
	return filepath.Join(sessionDir(sessionsDir, name), LiveLogName)
}

// liveExcerptMax bounds the textExcerpt carried on each live text/thought
// event. Live is for watching progress; the full text lands in the final
// report and traces, so a small window per event is enough.
const liveExcerptMax = 256

// liveExcerpt truncates s to liveExcerptMax bytes.
func liveExcerpt(s string) string {
	if len(s) > liveExcerptMax {
		return s[:liveExcerptMax]
	}
	return s
}

// liveEventHeader is common to every live.jsonl line: an RFC3339Nano UTC
// timestamp and the event type.
type liveEventHeader struct {
	Ts    string `json:"ts"`
	Event string `json:"event"`
}

// liveTextEvent records a streamed agent message or thought chunk
// (event "text" / "thought").
type liveTextEvent struct {
	liveEventHeader
	// Chars is the full size of the chunk; TextExcerpt is capped at
	// liveExcerptMax bytes.
	Chars       int    `json:"chars"`
	TextExcerpt string `json:"textExcerpt,omitempty"`
}

// liveToolCallEvent records a new ACP tool call (event "tool.call").
type liveToolCallEvent struct {
	liveEventHeader
	ID     string `json:"id"`
	Title  string `json:"title,omitempty"`
	Kind   string `json:"kind,omitempty"`
	Status string `json:"status,omitempty"`
}

// liveToolUpdateEvent records an ACP tool_call_update (event "tool.update").
type liveToolUpdateEvent struct {
	liveEventHeader
	ID string `json:"id"`
	// Status is present only when the update carried one.
	Status string `json:"status,omitempty"`
	// OutputSize is the content size observed on this update.
	OutputSize int `json:"outputSize"`
}

// liveTurnStartedEvent marks the runtime handing a prompt to the agent
// (event "turn.started").
type liveTurnStartedEvent struct {
	liveEventHeader
	Session  string `json:"session"`
	Repo     string `json:"repo,omitempty"`
	Question string `json:"question"`
}

// liveTurnCompletedEvent marks the end of an Ask, on every exit path
// (event "turn.completed").
type liveTurnCompletedEvent struct {
	liveEventHeader
	OK bool `json:"ok"`
	// Error carries the turn failure text when OK is false.
	Error string `json:"error,omitempty"`
	// Tools and Chars summarize the turn's observed tool calls and produced text.
	Tools int `json:"tools"`
	Chars int `json:"chars"`
}

// LiveLog is a concurrency-safe, append-only NDJSON writer for a session's
// live.jsonl. It is best-effort everywhere: an empty path or any open/write
// error degrades it to a no-op (with a single stderr warning) — a live-log
// failure must never fail or slow a turn. All methods are nil-safe so call
// sites need no guards. Append mode makes concurrent opens of the same path
// safe (the runtime and the ACP client each hold their own LiveLog).
type LiveLog struct {
	mu     sync.Mutex
	f      *os.File
	warned bool
}

// NewLiveLog opens the live turn log at path for appending. An empty path or
// an open failure returns a disabled writer whose methods are no-ops.
func NewLiveLog(path string) *LiveLog {
	if path == "" {
		return &LiveLog{}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open live turn log %s: %v\n", path, err)
		return &LiveLog{}
	}
	return &LiveLog{f: f}
}

// Text records a streamed agent message chunk.
func (l *LiveLog) Text(chars int, excerpt string) {
	l.append(&liveTextEvent{liveEventHeader: liveHeader("text"), Chars: chars, TextExcerpt: liveExcerpt(excerpt)})
}

// Thought records a streamed agent reasoning chunk.
func (l *LiveLog) Thought(chars int, excerpt string) {
	l.append(&liveTextEvent{liveEventHeader: liveHeader("thought"), Chars: chars, TextExcerpt: liveExcerpt(excerpt)})
}

// ToolCall records a new tool call observed on the ACP session.
func (l *LiveLog) ToolCall(id, title, kind, status string) {
	l.append(&liveToolCallEvent{liveEventHeader: liveHeader("tool.call"), ID: id, Title: title, Kind: kind, Status: status})
}

// ToolUpdate records a tool_call_update; status may be empty when the update
// carried none.
func (l *LiveLog) ToolUpdate(id, status string, outputSize int) {
	l.append(&liveToolUpdateEvent{liveEventHeader: liveHeader("tool.update"), ID: id, Status: status, OutputSize: outputSize})
}

// TurnStarted marks the runtime sending the primary prompt for an Ask.
func (l *LiveLog) TurnStarted(session, repo, question string) {
	l.append(&liveTurnStartedEvent{liveEventHeader: liveHeader("turn.started"), Session: session, Repo: repo, Question: question})
}

// TurnCompleted marks the end of an Ask on every exit path. errMsg is empty
// on success; tools/chars summarize the turn's activity.
func (l *LiveLog) TurnCompleted(ok bool, errMsg string, tools, chars int) {
	l.append(&liveTurnCompletedEvent{liveEventHeader: liveHeader("turn.completed"), OK: ok, Error: errMsg, Tools: tools, Chars: chars})
}

// Close releases the underlying file. Safe on nil and disabled writers.
func (l *LiveLog) Close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
}

// liveHeader stamps a new event with the current UTC time.
func liveHeader(event string) liveEventHeader {
	return liveEventHeader{Ts: time.Now().UTC().Format(time.RFC3339Nano), Event: event}
}

// append marshals one event and writes it as a single NDJSON line. Marshal
// failures skip the line; write failures warn to stderr once, then stay silent.
func (l *LiveLog) append(event any) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	if _, err := l.f.Write(append(data, '\n')); err != nil && !l.warned {
		l.warned = true
		fmt.Fprintf(os.Stderr, "warning: live turn log write failed: %v\n", err)
	}
}
