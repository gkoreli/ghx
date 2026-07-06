package sidecar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// readLiveLines parses every NDJSON line of a live.jsonl file, failing the
// test on any line that is not a valid JSON object with ts+event.
func readLiveLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live log: %v", err)
	}
	var events []map[string]any
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("line %d is not valid JSON: %v\n%s", i+1, err, line)
		}
		ts, _ := ev["ts"].(string)
		if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
			t.Errorf("line %d ts %q is not RFC3339Nano: %v", i+1, ts, err)
		}
		if event, _ := ev["event"].(string); event == "" {
			t.Errorf("line %d missing event type: %s", i+1, line)
		}
		events = append(events, ev)
	}
	return events
}

// ADR-0022.1: LiveLog writes one valid NDJSON object per event, each carrying
// ts (RFC3339Nano UTC) and event, with per-event-type fields.
func TestLiveLogWritesValidNDJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	l := NewLiveLog(path)
	l.TurnStarted("s", "owner/repo", "what does this do?")
	l.Text(11, "hello world")
	l.Thought(8, "thinking")
	l.ToolCall("call-1", "ghx read", "execute", "pending")
	l.ToolUpdate("call-1", "completed", 42)
	l.ToolUpdate("call-1", "", 7) // status absent on this update
	l.TurnCompleted(true, "", 1, 11)
	l.Close()

	events := readLiveLines(t, path)
	if len(events) != 7 {
		t.Fatalf("got %d events, want 7", len(events))
	}
	wantOrder := []string{"turn.started", "text", "thought", "tool.call", "tool.update", "tool.update", "turn.completed"}
	for i, want := range wantOrder {
		if got := events[i]["event"]; got != want {
			t.Errorf("event %d = %v, want %s", i, got, want)
		}
	}
	if events[0]["session"] != "s" || events[0]["repo"] != "owner/repo" || events[0]["question"] != "what does this do?" {
		t.Errorf("turn.started fields wrong: %v", events[0])
	}
	if events[1]["chars"] != float64(11) || events[1]["textExcerpt"] != "hello world" {
		t.Errorf("text fields wrong: %v", events[1])
	}
	if events[3]["id"] != "call-1" || events[3]["kind"] != "execute" || events[3]["status"] != "pending" {
		t.Errorf("tool.call fields wrong: %v", events[3])
	}
	if events[4]["outputSize"] != float64(42) || events[4]["status"] != "completed" {
		t.Errorf("tool.update fields wrong: %v", events[4])
	}
	if _, present := events[5]["status"]; present {
		t.Errorf("statusless tool.update must omit status: %v", events[5])
	}
	if events[6]["ok"] != true || events[6]["tools"] != float64(1) || events[6]["chars"] != float64(11) {
		t.Errorf("turn.completed fields wrong: %v", events[6])
	}
	if _, present := events[6]["error"]; present {
		t.Errorf("successful turn.completed must omit error: %v", events[6])
	}
}

// A failed turn's completion event carries ok=false and the error text.
func TestLiveLogTurnCompletedFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	l := NewLiveLog(path)
	l.TurnCompleted(false, "acp prompt: boom", 0, 0)
	l.Close()

	events := readLiveLines(t, path)
	if len(events) != 1 || events[0]["ok"] != false || events[0]["error"] != "acp prompt: boom" {
		t.Fatalf("failure event wrong: %v", events)
	}
}

// ADR-0022.1: an empty path (and a nil receiver) disables the log — every
// method is a safe no-op and nothing is written anywhere.
func TestLiveLogDisabledIsNoOp(t *testing.T) {
	exercise := func(l *LiveLog) {
		l.TurnStarted("s", "r", "q")
		l.Text(3, "abc")
		l.Thought(3, "abc")
		l.ToolCall("id", "t", "k", "s")
		l.ToolUpdate("id", "s", 1)
		l.TurnCompleted(true, "", 0, 0)
		l.Close()
	}
	exercise(NewLiveLog(""))
	var nilLog *LiveLog
	exercise(nilLog)
}

// An unopenable path degrades to a no-op instead of failing the turn.
func TestLiveLogUnopenablePathDegrades(t *testing.T) {
	l := NewLiveLog(filepath.Join(t.TempDir(), "no", "such", "dir", "live.jsonl"))
	l.Text(2, "ok")
	l.Close()
}

// ADR-0022.1: textExcerpt is capped at 256 bytes per event while chars keeps
// the full size.
func TestLiveLogExcerptCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "live.jsonl")
	long := strings.Repeat("x", 1000)
	l := NewLiveLog(path)
	l.Text(len(long), long)
	l.Close()

	events := readLiveLines(t, path)
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1", len(events))
	}
	excerpt, _ := events[0]["textExcerpt"].(string)
	if len(excerpt) != 256 {
		t.Errorf("excerpt length = %d, want 256", len(excerpt))
	}
	if events[0]["chars"] != float64(1000) {
		t.Errorf("chars = %v, want 1000", events[0]["chars"])
	}
}

// LiveLogPath mirrors AgentStderrLogPath into the session directory.
func TestLiveLogPath(t *testing.T) {
	if got, want := LiveLogPath("/tmp/sessions", "s"), filepath.Join("/tmp/sessions", "s", LiveLogName); got != want {
		t.Errorf("LiveLogPath = %q, want %q", got, want)
	}
}
