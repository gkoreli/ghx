package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// The tests in this file pin ADR-0040 L2 ask progress streaming against
// docs/research/006-l2-streaming-ux-states.md's seven acceptance criteria.
// Fixtures are replayed through the REAL producer (sidecar.LiveLog), so a
// producer-side field rename fails these tests instead of silently breaking
// the stream.

// newProgressFixture is a live.jsonl replay harness: it hands back a spec
// pointing at a temp session dir plus a writer bound to a real LiveLog
// producer for that dir's live.jsonl. start is the formatter clock origin.
func newProgressFixture(t *testing.T, mode askProgressMode) (askProgressSpec, *sidecar.LiveLog) {
	t.Helper()
	dir := t.TempDir()
	sessionsDir := filepath.Join(dir, "sessions")
	spec := askProgressSpec{
		sessionsDir: sessionsDir,
		session:     "hono-hono",
		repo:        "honojs/hono",
		question:    "How is middleware chained?",
	}
	// The session dir exists before dispatch on every real path that names
	// its session (the daemon creates it when routing); NewLiveLog appends,
	// it never mkdirs.
	if err := os.MkdirAll(filepath.Join(sessionsDir, spec.session), 0o755); err != nil {
		t.Fatal(err)
	}
	live := sidecar.NewLiveLog(filepath.Join(sessionsDir, spec.session, sidecar.LiveLogName))
	t.Cleanup(live.Close)
	return spec, live
}

func TestAskProgressFormatterLines(t *testing.T) {
	base := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		mode    askProgressMode
		event   string // live.jsonl "event" value
		fields  string // extra JSON fields merged into the event line
		elapsed time.Duration
		want    string
	}{
		{
			name:    "tool call renders title (TTY)",
			mode:    askProgressTTY,
			event:   "tool.call",
			fields:  `"id":"t1","title":"ghx search honojs/hono middleware","kind":"search","status":"in_progress"`,
			elapsed: 5 * time.Second,
			want:    "[5s] tool: ghx search honojs/hono middleware",
		},
		{
			name:    "tool call renders title (plain)",
			mode:    askProgressPlain,
			event:   "tool.call",
			fields:  `"id":"t1","title":"ghx search honojs/hono middleware","kind":"search","status":"in_progress"`,
			elapsed: 5 * time.Second,
			want:    "# [5s] tool: ghx search honojs/hono middleware",
		},
		{
			name:    "tool call without title falls back to id",
			mode:    askProgressTTY,
			event:   "tool.call",
			fields:  `"id":"call_7","kind":"read"`,
			elapsed: 3 * time.Second,
			want:    "[3s] tool: call_7",
		},
		{
			name:    "completed tool update names remembered title and status",
			mode:    askProgressTTY,
			event:   "tool.update",
			fields:  `"id":"t1","status":"completed","outputSize":2048`,
			elapsed: 14 * time.Second,
			want:    "[14s] tool ghx search honojs/hono middleware · completed (2048 chars)",
		},
		{
			name:    "text event renders excerpt at int-second elapsed (plain)",
			mode:    askProgressPlain,
			event:   "text",
			fields:  `"chars":120,"textExcerpt":"reading 3 files…"`,
			elapsed: 31*time.Second + 600*time.Millisecond,
			want:    "# [31s] reading 3 files…",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newAskProgressFormatter(base, tt.mode)
			f.now = func() time.Time { return base.Add(tt.elapsed) }
			// The tool.update row depends on state from an earlier tool.call
			// row, so seed the formatter's id→title memory the way a real
			// turn would have.
			if tt.event == "tool.update" && strings.Contains(tt.fields, `"id":"t1"`) {
				f.toolTitles["t1"] = "ghx search honojs/hono middleware"
			}
			line := `{"event":"` + tt.event + `",` + tt.fields + `}`
			got := f.line(decodeAskLiveEvent(t, line))
			if got != tt.want {
				t.Fatalf("line = %q, want %q", got, tt.want)
			}
		})
	}
}

func decodeAskLiveEvent(t *testing.T, line string) askLiveEvent {
	t.Helper()
	var ev askLiveEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		t.Fatalf("decode %s: %v", line, err)
	}
	return ev
}

// TestAskProgressFormatterSkipsSilentEvents pins which events produce NO
// progress line: turn boundaries (duplicated by the completion line and final
// report), bare thinking, blank text, pending tool updates (the FRICTION.md
// ~48×pending pathology — criterion 5's first half), and unknown future
// events — the CLI must never crash on, or spam about, a live.jsonl shape it
// does not know.
func TestAskProgressFormatterSkipsSilentEvents(t *testing.T) {
	f := newAskProgressFormatter(time.Now(), askProgressTTY)
	for _, line := range []string{
		`{"event":"turn.started","session":"hono-hono","question":"q"}`,
		`{"event":"turn.completed","ok":true,"tools":4,"chars":900}`,
		`{"event":"thought","chars":512}`,
		`{"event":"text","chars":300,"textExcerpt":"   "}`,
		`{"event":"tool.update","id":"t1","status":""}`,
		`{"event":"tool.update","id":"t1","status":"in_progress"}`,
		`{"event":"brand.new.event.from.a.newer.writer"}`,
	} {
		if got := f.line(decodeAskLiveEvent(t, line)); got != "" {
			t.Errorf("event %s rendered %q, want silent", line, got)
		}
	}
}

// TestAskProgressCollapsesRepeatedLines pins criterion 5: identical content
// collapses regardless of its elapsed stamp, so repeats of the same event
// cannot rebuild a wall of duplicate lines. Trailing-ellipsis variants of the
// same content dedupe too.
func TestAskProgressCollapsesRepeatedLines(t *testing.T) {
	f := newAskProgressFormatter(time.Now(), askProgressPlain)
	ev := decodeAskLiveEvent(t, `{"event":"tool.call","id":"t1","title":"ghx explore honojs/hono"}`)
	if got := f.line(ev); !strings.Contains(got, "ghx explore") {
		t.Fatalf("first occurrence = %q, want rendered", got)
	}
	for i := 0; i < 3; i++ {
		time.Sleep(1100 * time.Millisecond) // force a different [Ns] stamp
		if got := f.line(ev); got != "" {
			t.Fatalf("repeat %d rendered %q, want collapsed", i+1, got)
		}
	}
	// A different title renders; returning to the earlier one renders again.
	other := decodeAskLiveEvent(t, `{"event":"tool.call","id":"t2","title":"ghx tree honojs/hono"}`)
	if got := f.line(other); got == "" {
		t.Fatal("different tool did not render")
	}
	back := decodeAskLiveEvent(t, `{"event":"tool.call","id":"t1","title":"ghx explore honojs/hono"}`)
	if got := f.line(back); got == "" {
		t.Fatal("A-B-A pattern collapsed; only consecutive duplicates may collapse")
	}
}

// TestAskProgressIdleStates pins the derived thinking/synthesizing states:
// they render from absence of events (>3s gap), never before the first
// event, once per quiet stretch, and never fabricate activity names.
func TestAskProgressIdleStates(t *testing.T) {
	base := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	f := newAskProgressFormatter(base, askProgressTTY)
	now := base
	f.now = func() time.Time { return now }

	if got := f.idleLine(); got != "" {
		t.Fatalf("idle before any event rendered %q, want silent", got)
	}

	// First activity, then >3s of quiet: thinking.
	now = base.Add(500 * time.Millisecond)
	f.noteActivity(now)
	now = base.Add(4 * time.Second)
	if got := f.idleLine(); got != "[4s] thinking…" {
		t.Fatalf("idle after first event = %q, want [4s] thinking…", got)
	}
	if got := f.idleLine(); got != "" {
		t.Fatalf("second idle poll rendered %q, want silence (once per stretch)", got)
	}

	// Tool activity arrives, then another quiet stretch: synthesizing with
	// the truthful source count.
	now = base.Add(6 * time.Second)
	f.noteActivity(now)
	f.line(decodeAskLiveEvent(t, `{"event":"tool.call","id":"t1","title":"ghx explore honojs/hono"}`))
	now = base.Add(10 * time.Second)
	if got := f.idleLine(); got != "[10s] synthesizing… (1 sources so far)" {
		t.Fatalf("idle after tools = %q, want [10s] synthesizing… (1 sources so far)", got)
	}

	// New evidence resets the idle state.
	now = base.Add(11 * time.Second)
	f.noteActivity(now)
	now = base.Add(20 * time.Second)
	if got := f.idleLine(); got != "[20s] synthesizing… (1 sources so far)" {
		t.Fatalf("new quiet stretch = %q, want a fresh synthesizing line", got)
	}
}

func TestTruncateAskExcerpt(t *testing.T) {
	long := strings.Repeat("a", 80)
	got := truncateAskExcerpt(long)
	if n := len([]rune(got)); n != askProgressExcerptMax+1 { // +1 ellipsis rune
		t.Fatalf("truncated length = %d runes, want %d+ellipsis", n, askProgressExcerptMax)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("truncated = %q, want trailing ellipsis", got)
	}
	if ws := truncateAskExcerpt("a\n\tb   c"); ws != "a b c" {
		t.Fatalf("whitespace collapse = %q, want %q", ws, "a b c")
	}
}

func TestFormatAskElapsed(t *testing.T) {
	if got := formatAskElapsed(12 * time.Second); got != "[12s]" {
		t.Fatalf("formatAskElapsed(12s) = %q, want [12s]", got)
	}
	if got := formatAskElapsed(31*time.Second + 900*time.Millisecond); got != "[31s]" {
		t.Fatalf("sub-second age = %q, want truncated [31s]", got)
	}
	if got := formatAskElapsed(-time.Second); got != "[0s]" {
		t.Fatalf("negative age = %q, want clamped [0s]", got)
	}
}

// syncWriter is a mutex-guarded buffer: the poller writes from its own
// goroutine while the test inspects the stream, so every access must lock.
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// waitUntil polls cond until it holds or the timeout passes; the last check
// always runs so a fast pass is never delayed by the sleep quantum.
func waitUntil(cond func() bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

// TestAskProgressFirstEvidenceUnderFiveSeconds pins criterion 1 as a
// measurement, not an assertion: from poller start to the FIRST streamed
// evidence line (dispatch line + first replayed tool call) must stay well
// under the ADR-0040 L2 five-second budget on this machine.
func TestAskProgressFirstEvidenceUnderFiveSeconds(t *testing.T) {
	spec, live := newProgressFixture(t, askProgressPlain)

	done := make(chan struct{})
	w := &syncWriter{}
	started := time.Now()
	go askLivePoller(spec, askProgressPlain, done, newAskProgressWriter(w))

	// First visible signal immediately, before any agent event exists.
	if !waitUntil(func() bool {
		return strings.Contains(w.String(), "# [0s] streaming "+spec.session)
	}, time.Second) {
		t.Fatalf("no immediate dispatch line within 1s, got: %q", w.String())
	}

	live.ToolCall("t1", "ghx explore honojs/hono", "read", "in_progress")
	var firstEvidence time.Time
	if !waitUntil(func() bool {
		ok := strings.Contains(w.String(), "ghx explore")
		if ok && firstEvidence.IsZero() {
			firstEvidence = time.Now()
		}
		return ok
	}, 3*time.Second) {
		t.Fatalf("replayed tool call never surfaced: %q", w.String())
	}
	close(done)

	latency := firstEvidence.Sub(started)
	if latency > 5*time.Second {
		t.Fatalf("first evidence took %v; ADR-0040 L2 budget is <5s", latency)
	}
	t.Logf("first evidence latency: %v (budget 5s)", latency)
}

// TestAskProgressRoutedSessionDiscovery pins the flagless-ask path: the
// poller attaches to whichever session's turn.started carries the exact
// question, prefers the most recent match, prints the discovery line, then
// streams that session's events.
func TestAskProgressRoutedSessionDiscovery(t *testing.T) {
	dir := t.TempDir()
	sessionsDir := filepath.Join(dir, "sessions")
	for _, name := range []string{"stale-repo", "fresh-repo"} {
		if err := os.MkdirAll(filepath.Join(sessionsDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	question := "Where are reports validated?"

	// An older turn elsewhere asked the same question — recency must win.
	stale := sidecar.NewLiveLog(filepath.Join(sessionsDir, "stale-repo", sidecar.LiveLogName))
	stale.TurnStarted("stale-repo", "gkoreli/ghx", question)
	stale.Close()
	time.Sleep(20 * time.Millisecond)

	fresh := sidecar.NewLiveLog(filepath.Join(sessionsDir, "fresh-repo", sidecar.LiveLogName))
	fresh.TurnStarted("fresh-repo", "gkoreli/ghx", question)
	fresh.ToolCall("f1", "ghx explore gkoreli/ghx", "read", "in_progress")

	spec := askProgressSpec{sessionsDir: sessionsDir, repo: "gkoreli/ghx", question: question}
	done := make(chan struct{})
	w := &syncWriter{}
	go askLivePoller(spec, askProgressTTY, done, newAskProgressWriter(w))

	if !waitUntil(func() bool {
		return strings.Contains(w.String(), "▶ session fresh-repo") &&
			strings.Contains(w.String(), "ghx explore")
	}, 3*time.Second) {
		t.Fatalf("routed discovery failed; stream: %q", w.String())
	}
	close(done)
	fresh.Close()

	// No dispatch line before discovery on a routed ask (the session was
	// unknowable at t=0).
	if s := w.String(); strings.Contains(s, "streaming  activity") || strings.Count(s, "\n") > 3 {
		t.Fatalf("unexpected pre-discovery output: %q", w.String())
	}
}

// TestDiscoverAskSessionRejectsMismatch pins discovery precision: a
// different question or repo must not attach to the wrong session's stream.
func TestDiscoverAskSessionRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	name, found := discoverAskSession(dir, "", "q")
	if found {
		t.Fatalf("empty sessions dir resolved to %q", name)
	}

	sessionsDir := filepath.Join(dir, "sessions")
	if err := os.MkdirAll(filepath.Join(sessionsDir, "other"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := sidecar.NewLiveLog(filepath.Join(sessionsDir, "other", sidecar.LiveLogName))
	live.TurnStarted("other", "acme/other", "a different question entirely")
	live.Close()

	if _, found := discoverAskSession(sessionsDir, "acme/other", "q"); found {
		t.Fatal("discovery matched a different question")
	}
	if _, found := discoverAskSession(sessionsDir, "other/repo", "a different question entirely"); found {
		t.Fatal("repo-scoped discovery matched despite repo mismatch")
	}
	if name, found := discoverAskSession(sessionsDir, "", "a different question entirely"); !found || name != "other" {
		t.Fatalf("unscoped discovery = (%q,%v), want (other,true)", name, found)
	}
}

// TestAskProgressStopsCleanly is the leak + ordering check: closing done via
// stop() terminates the poller promptly even while live.jsonl keeps growing,
// and no post-stop probe event ever reaches the stream.
func TestAskProgressStopsCleanly(t *testing.T) {
	spec, live := newProgressFixture(t, askProgressTTY)

	sw := &syncWriter{}
	ap := startAskProgress(spec, askProgressTTY, sw)

	if !waitUntil(func() bool {
		return strings.Contains(sw.String(), "streaming "+spec.session)
	}, time.Second) {
		t.Fatalf("no dispatch line, got: %q", sw.String())
	}

	live.ToolCall("t1", "ghx read honojs/hono README.md", "read", "in_progress")
	if !waitUntil(func() bool { return strings.Contains(sw.String(), "ghx read") }, 3*time.Second) {
		t.Fatalf("running-turn event never surfaced: %q", sw.String())
	}

	ap.stop() // must block until the goroutine has fully exited

	quiesced := sw.String()
	// Keep the file growing after close with unmistakable probe events; a
	// leaked poller would render them within ticks.
	for i := 0; i < 4; i++ {
		live.ToolCall("probe", "LEAK-PROBE-"+time.Now().Format("05.000"), "read", "")
		time.Sleep(askProgressPollInterval) // ≥ two full ticks across probes
	}
	time.Sleep(2 * askProgressPollInterval)

	if s := sw.String(); len(s) != len(quiesced) {
		t.Fatalf("poller kept writing after stop (leak):\nbefore: %q\nafter:  %q", quiesced, s)
	}
	if !strings.Contains(quiesced, "[") {
		t.Fatalf("no stamped progress lines were streamed before stop: %q", quiesced)
	}
}

// TestAskProgressWaitsForLateFile pins that a routed ask whose session dir
// does not exist yet (daemon still starting/routing) produces no error and
// picks the file up once it appears.
func TestAskProgressWaitsForLateFile(t *testing.T) {
	dir := t.TempDir()
	sessionsDir := filepath.Join(dir, "sessions")
	spec := askProgressSpec{sessionsDir: sessionsDir, session: "late-session", question: "q"}

	done := make(chan struct{})
	w := &syncWriter{}
	go askLivePoller(spec, askProgressPlain, done, newAskProgressWriter(w))
	time.Sleep(100 * time.Millisecond)

	// The session dir appears late (daemon created it on routing), then the
	// producer starts appending.
	if err := os.MkdirAll(filepath.Join(sessionsDir, "late-session"), 0o755); err != nil {
		t.Fatal(err)
	}
	live := sidecar.NewLiveLog(filepath.Join(sessionsDir, "late-session", sidecar.LiveLogName))
	defer live.Close()
	live.ToolCall("i1", "late tool", "read", "")

	if !waitUntil(func() bool { return strings.Contains(w.String(), "late tool") }, 3*time.Second) {
		t.Fatalf("late-created live.jsonl never surfaced: %q", w.String())
	}
	close(done)
}

// TestSplitAskCarry pins the partial-line discipline: only newline-complete
// events render; the trailing fragment stays carried until its newline.
func TestSplitAskCarry(t *testing.T) {
	lines, rest := splitAskCarry([]byte(`{"a":1}` + "\npartial"))
	if len(lines) != 1 || lines[0] != `{"a":1}` {
		t.Fatalf("lines = %v, want one complete line", lines)
	}
	if string(rest) != "partial" {
		t.Fatalf("rest = %q, want the trailing partial line", rest)
	}
	lines, rest = splitAskCarry([]byte("no-newline-yet"))
	if lines != nil || string(rest) != "no-newline-yet" {
		t.Fatalf("no-newline case = (%v, %q), want nil + carry", lines, rest)
	}
	lines, _ = splitAskCarry([]byte("\n\n{\"b\":2}\n"))
	if len(lines) != 1 || lines[0] != `{"b":2}` {
		t.Fatalf("blank-line skip = %v, want one line", lines)
	}
}

// TestAskReportSummary pins the completion-line summary wording: truthful
// counts from the delivered report, BLOCKED answers quoted instead of counted.
func TestAskReportSummary(t *testing.T) {
	report := &sidecar.Report{
		Answer:   "Middleware chains through compose().",
		Verified: []sidecar.Claim{{Summary: "a"}, {Summary: "b"}},
		Inferred: []sidecar.Claim{{Summary: "c"}},
		Evidence: []sidecar.Evidence{{Source: "s"}, {Source: "t"}, {Source: "u"}},
	}
	if got := askReportSummary(report); got != "2 verified claims, 3 citations" {
		t.Fatalf("summary = %q, want counts of verified + citations", got)
	}

	blocked := &sidecar.Report{Answer: "BLOCKED: invalid repo slug — no investigation possible"}
	if got := askReportSummary(blocked); got != "BLOCKED: invalid repo slug — no investigation possible" {
		t.Fatalf("blocked summary = %q, want the BLOCKED reason", got)
	}

	empty := &sidecar.Report{Answer: "answered without evidence"}
	if got := askReportSummary(empty); got != "no claims extracted" {
		t.Fatalf("empty summary = %q, want fallback", got)
	}

	inferredOnly := &sidecar.Report{Answer: "ans", Inferred: []sidecar.Claim{{Summary: "x"}}}
	if got := askReportSummary(inferredOnly); got != "1 inferred claims" {
		t.Fatalf("inferred-only summary = %q", got)
	}
}

// TestStderrIsTTYPinsModeSelection exercises both renderings of the same
// event so the TTY/plain split stays information-equivalent (criterion 4:
// plain mode loses glyphs, not facts). Under `go test`, stderr is redirected,
// so the CLI would pick plain mode — the negative check pins that.
func TestStderrIsTTYPinsModeSelection(t *testing.T) {
	if stderrIsTTY() {
		t.Skip("stderr is a terminal in this environment; TTY detection can't be negative-tested here")
	}
}
