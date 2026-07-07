package sidecar

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLiveTailFormatterEvents pins the rendered line for every live.jsonl
// event type, plus the never-crash fallbacks for unknown events, malformed
// JSON, and blank lines.
func TestLiveTailFormatterEvents(t *testing.T) {
	f := NewLiveTailFormatter(time.UTC)
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "turn started",
			line: `{"ts":"2026-07-06T17:18:22Z","event":"turn.started","session":"honojs-hono","repo":"honojs/hono","question":"Which router does Hono use by default?"}`,
			want: `17:18:22 ▶ turn started  honojs-hono  "Which router does Hono use by default?"`,
		},
		{
			name: "turn started without session falls back to repo",
			line: `{"ts":"2026-07-06T17:18:22Z","event":"turn.started","repo":"honojs/hono","question":"q"}`,
			want: `17:18:22 ▶ turn started  honojs/hono  "q"`,
		},
		{
			name: "text with excerpt",
			line: `{"ts":"2026-07-06T17:18:43.120Z","event":"text","chars":208,"textExcerpt":"Hono picks SmartRouter by\ndefault"}`,
			want: `17:18:43 ✎ text (208 chars)  "Hono picks SmartRouter by default"`,
		},
		{
			name: "thought",
			line: `{"ts":"2026-07-06T17:18:45Z","event":"thought","chars":412,"textExcerpt":"let me check"}`,
			want: `17:18:45 💭 thinking (412 chars)`,
		},
		{
			name: "tool call",
			line: `{"ts":"2026-07-06T17:18:36Z","event":"tool.call","id":"t1","title":"Terminal","kind":"execute","status":"pending"}`,
			want: `17:18:36 ⚙ Terminal [execute] pending`,
		},
		{
			name: "tool update names the tool from its call",
			line: `{"ts":"2026-07-06T17:18:40Z","event":"tool.update","id":"t1","status":"completed","outputSize":12300}`,
			want: `17:18:40 ⚙ Terminal completed (12.3k chars)`,
		},
		{
			name: "tool update for unseen id without status or output",
			line: `{"ts":"2026-07-06T17:18:41Z","event":"tool.update","id":"t9","outputSize":0}`,
			want: `17:18:41 ⚙ t9 update`,
		},
		{
			name: "turn completed ok",
			line: `{"ts":"2026-07-06T17:18:56Z","event":"turn.completed","ok":true,"tools":3,"chars":8200}`,
			want: `17:18:56 ✔ turn completed  3 tools  8.2k chars`,
		},
		{
			name: "turn failed",
			line: `{"ts":"2026-07-06T17:18:56Z","event":"turn.completed","ok":false,"error":"agent exited unexpectedly","tools":1,"chars":0}`,
			want: `17:18:56 ✖ turn FAILED: agent exited unexpectedly`,
		},
		{
			name: "unknown event renders as low-key fallback",
			line: `{"ts":"2026-07-06T17:18:57Z","event":"turn.paused","whatever":1}`,
			want: `17:18:57 · turn.paused`,
		},
		{
			name: "malformed JSON never crashes",
			line: `{"ts":"2026-07-06T17:18:58Z","event":`,
			want: `--:--:-- · unparsed: {"ts":"2026-07-06T17:18:58Z","event":`,
		},
		{
			name: "missing event type is treated as unparsed",
			line: `{"ts":"2026-07-06T17:18:59Z"}`,
			want: `--:--:-- · unparsed: {"ts":"2026-07-06T17:18:59Z"}`,
		},
		{
			name: "blank line renders empty",
			line: "   ",
			want: "",
		},
	}
	for _, tc := range cases {
		if got := f.FormatLine(tc.line); got != tc.want {
			t.Errorf("%s:\n got  %q\n want %q", tc.name, got, tc.want)
		}
	}
}

// TestLiveTailTruncate pins excerpt hygiene: whitespace collapse and the
// rune-safe ellipsis cut.
func TestLiveTailTruncate(t *testing.T) {
	if got := liveTailTruncate("a\n\tb   c", 60); got != "a b c" {
		t.Errorf("whitespace collapse: got %q", got)
	}
	long := strings.Repeat("é", 70)
	got := liveTailTruncate(long, 60)
	if want := strings.Repeat("é", 60) + "…"; got != want {
		t.Errorf("rune truncation: got %q want %q", got, want)
	}
}

// TestHumanChars pins the compact count rendering.
func TestHumanChars(t *testing.T) {
	for n, want := range map[int]string{412: "412", 8200: "8.2k", 12000: "12k", 12300: "12.3k", 1500000: "1.5M"} {
		if got := humanChars(n); got != want {
			t.Errorf("humanChars(%d) = %q, want %q", n, got, want)
		}
	}
}

// twoTurnFixture is a live.jsonl with two complete turns; the tail must show
// only the second.
const twoTurnFixture = `{"ts":"2026-07-06T17:00:00Z","event":"turn.started","session":"honojs-hono","question":"old turn"}
{"ts":"2026-07-06T17:00:05Z","event":"turn.completed","ok":true,"tools":1,"chars":100}
{"ts":"2026-07-06T17:18:22Z","event":"turn.started","session":"honojs-hono","question":"Which router does Hono use by default?"}
{"ts":"2026-07-06T17:18:36Z","event":"tool.call","id":"t1","title":"Terminal","kind":"execute","status":"pending"}
{"ts":"2026-07-06T17:18:40Z","event":"tool.update","id":"t1","status":"completed","outputSize":12300}
{"ts":"2026-07-06T17:18:45Z","event":"thought","chars":412}
{"ts":"2026-07-06T17:18:56Z","event":"turn.completed","ok":true,"tools":3,"chars":8200}
`

// TestTailLiveLogLastTurnBoundary verifies the non-follow tail replays only
// the LAST turn of a multi-turn live.jsonl.
func TestTailLiveLogLastTurnBoundary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LiveLogName)
	if err := os.WriteFile(path, []byte(twoTurnFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := TailLiveLog(context.Background(), &out, path, LiveTailOptions{Loc: time.UTC}); err != nil {
		t.Fatalf("TailLiveLog: %v", err)
	}
	want := `17:18:22 ▶ turn started  honojs-hono  "Which router does Hono use by default?"
17:18:36 ⚙ Terminal [execute] pending
17:18:40 ⚙ Terminal completed (12.3k chars)
17:18:45 💭 thinking (412 chars)
17:18:56 ✔ turn completed  3 tools  8.2k chars
`
	if out.String() != want {
		t.Errorf("rendered tail:\n got:\n%s\n want:\n%s", out.String(), want)
	}
	if strings.Contains(out.String(), "old turn") {
		t.Error("output leaked the previous turn")
	}
}

// TestTailLiveLogRaw verifies --raw prints the last turn's NDJSON verbatim.
func TestTailLiveLogRaw(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LiveLogName)
	if err := os.WriteFile(path, []byte(twoTurnFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := TailLiveLog(context.Background(), &out, path, LiveTailOptions{Raw: true, Loc: time.UTC}); err != nil {
		t.Fatalf("TailLiveLog: %v", err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("raw tail printed %d lines, want 5:\n%s", len(lines), out.String())
	}
	fixtureLines := strings.Split(strings.TrimRight(twoTurnFixture, "\n"), "\n")
	for i, want := range fixtureLines[2:] {
		if lines[i] != want {
			t.Errorf("raw line %d = %q, want %q", i, lines[i], want)
		}
	}
}

// TestTailLiveLogNoTurnStarted verifies a file without any turn.started line
// (e.g. only client events survived) is shown from the beginning, not hidden.
func TestTailLiveLogNoTurnStarted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LiveLogName)
	content := `{"ts":"2026-07-06T17:18:45Z","event":"thought","chars":412}` + "\n" +
		`not json at all` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := TailLiveLog(context.Background(), &out, path, LiveTailOptions{Loc: time.UTC}); err != nil {
		t.Fatalf("TailLiveLog: %v", err)
	}
	want := "17:18:45 💭 thinking (412 chars)\n--:--:-- · unparsed: not json at all\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
}

// TestTailLiveLogMissingFile verifies the non-follow tail errors cleanly when
// the session has no live.jsonl.
func TestTailLiveLogMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), LiveLogName)
	var out bytes.Buffer
	err := TailLiveLog(context.Background(), &out, path, LiveTailOptions{Loc: time.UTC})
	if err == nil || !strings.Contains(err.Error(), "no live turn log") {
		t.Fatalf("want clean missing-file error, got %v", err)
	}
}

// TestResolveTailSession verifies no-arg resolution picks the session with
// the newest live.jsonl/meta.json mtime — including a running first turn
// whose meta.json is older than another session's.
func TestResolveTailSession(t *testing.T) {
	sessionsDir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)
	mid := time.Now().Add(-1 * time.Hour)

	mustWrite := func(session, file string, mtime time.Time) {
		t.Helper()
		dir := filepath.Join(sessionsDir, session)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, file)
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("older-session", "meta.json", old)
	mustWrite("older-session", LiveLogName, old)
	mustWrite("running-session", "meta.json", mid)
	mustWrite("running-session", LiveLogName, time.Now()) // live turn in flight

	got, err := ResolveTailSession(sessionsDir)
	if err != nil {
		t.Fatalf("ResolveTailSession: %v", err)
	}
	if got != "running-session" {
		t.Errorf("resolved %q, want running-session", got)
	}

	if _, err := ResolveTailSession(t.TempDir()); err == nil || !strings.Contains(err.Error(), "no sessions found") {
		t.Errorf("empty sessions dir: want 'no sessions found' error, got %v", err)
	}
}
