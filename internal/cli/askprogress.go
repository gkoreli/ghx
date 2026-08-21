// Ask progress streaming (ADR-0040 L2): while a `ghx sidecar ask` turn runs
// (40–106s), the caller previously saw nothing until the full report landed.
// The session's live.jsonl (ADR-0022.1) already carries turn activity as it
// happens — this file tails it and renders compact progress lines on STDERR,
// so first useful evidence is visible within seconds instead of after the
// whole turn. STDOUT stays reserved for the final report / --json envelope
// (ADR-0019.3 D2: streaming is human decoration only; machine surfaces are
// untouched).
//
// Rendering rules (docs/research/006-l2-streaming-ux-states.md):
//   - one scannable line per event, newest-wins; repeats of an identical
//     line collapse (the FRICTION.md ~48×`(pending)` pathology cannot recur);
//   - pending tool updates render nothing — they carry no new information
//     beyond the tool.call line already shown;
//   - a >3s event gap derives ONE static "thinking…" / "synthesizing… (N
//     sources so far)" line — never a fabricated activity name;
//   - non-TTY stderr gets the same information without glyphs, behind a
//     "# " prefix scripting agents can grep.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

const (
	// askProgressPollInterval is how often the poller re-reads live.jsonl.
	// Plain polling keeps this dependency-free (no fsnotify); 300ms keeps
	// first-evidence latency far inside the ADR-0040 L2 <5s budget even if
	// several ticks land before the writer flushes.
	askProgressPollInterval = 300 * time.Millisecond

	// askProgressIdleAfter is the event-gap after which the derived
	// thinking/synthesizing state renders (research 006: "absence of other
	// events >3s"). Any decoded event — even one that renders no line —
	// counts as activity and resets the gap.
	askProgressIdleAfter = 3 * time.Second

	// askProgressPrefix marks every non-TTY progress line so callers can
	// tell them apart from warnings or the final report on the same stream.
	askProgressPrefix = "# "

	// askProgressExcerptMax bounds quoted excerpts (tool titles, text,
	// blocked answers) on a progress line, so one event stays one scannable
	// terminal line.
	askProgressExcerptMax = 72
)

// askProgressMode selects rendering: askProgressTTY gets compact glyph
// lines for a human watching a terminal; askProgressPlain gets "# "-prefixed
// ASCII-safe lines for redirected output (scripts, CI logs). Both carry the
// same information; neither emits ANSI escapes.
type askProgressMode int

const (
	askProgressPlain askProgressMode = iota
	askProgressTTY
)

// askLiveEvent is the union decode target for every live.jsonl event shape
// written by internal/sidecar/live.go. Fields not carried by a given event
// type stay zero.
type askLiveEvent struct {
	Event       string `json:"event"`
	Ts          string `json:"ts"`
	Chars       int    `json:"chars"`
	TextExcerpt string `json:"textExcerpt"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	OutputSize  int    `json:"outputSize"`
	Session     string `json:"session"`
	Repo        string `json:"repo"`
	Question    string `json:"question"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
}

// askProgressFormatter renders decoded live.jsonl events as compact progress
// lines. It is stateful — tool.call events register id→title, the repeat-
// collapse remembers the last line, and the idle state tracks the last
// activity — and therefore not safe for concurrent use; each poller owns its
// own formatter.
type askProgressFormatter struct {
	// start is the moment the ask was dispatched; elapsed seconds on every
	// line are measured from it, not from the event's own timestamp.
	start time.Time
	// now overrides the clock (tests inject a fixed one); nil means time.Now.
	now func() time.Time
	// mode picks glyph vs prefixed rendering.
	mode askProgressMode
	// lastKey is the content key of the last rendered line (elapsed stamp
	// stripped) — a repeat collapses to silence.
	lastKey string
	// lastActivity is the wall-clock moment of the most recent decoded event;
	// zero until the first event arrives (the idle state never renders before
	// the first evidence of life).
	lastActivity time.Time
	// idleActive records that a derived thinking/synthesizing line is the
	// latest output, so it renders once per quiet stretch, not every second.
	idleActive bool
	// toolsSeen counts tool.call events — the "sources so far" in the
	// synthesizing state.
	toolsSeen int
	// toolTitles remembers tool.call titles for tool.update lines.
	toolTitles map[string]string
}

func newAskProgressFormatter(start time.Time, mode askProgressMode) *askProgressFormatter {
	return &askProgressFormatter{
		start:      start,
		mode:       mode,
		toolTitles: map[string]string{},
	}
}

func (f *askProgressFormatter) elapsed() time.Duration {
	now := time.Now()
	if f.now != nil {
		now = f.now()
	}
	if now.Before(f.start) {
		return 0
	}
	return now.Sub(f.start)
}

// noteActivity records that a live.jsonl event arrived (whatever its kind —
// bare thinking chunks are activity too), clearing any derived idle line.
func (f *askProgressFormatter) noteActivity(now time.Time) {
	f.lastActivity = now
	f.idleActive = false
}

// idleLine renders the derived state for a quiet stretch: "thinking…" until
// the first tool call, "synthesizing… (N sources so far)" after. It renders
// once per quiet stretch and never before the first event. An empty result
// means "nothing to say yet / already said".
func (f *askProgressFormatter) idleLine() string {
	if f.lastActivity.IsZero() || f.idleActive {
		return ""
	}
	now := time.Now()
	if f.now != nil {
		now = f.now()
	}
	if now.Sub(f.lastActivity) < askProgressIdleAfter {
		return ""
	}
	f.idleActive = true
	label := "thinking…"
	if f.toolsSeen > 0 {
		label = fmt.Sprintf("synthesizing… (%d sources so far)", f.toolsSeen)
	}
	return f.stamp(label)
}

// line renders ONE compact progress line for one decoded event. Events that
// carry no useful caller-facing signal (turn boundaries, bare thinking,
// pending tool updates) and unknown-but-valid events (a newer writer) render
// as "" — the CLI must never crash on, or spam about, a future live.jsonl
// shape.
func (f *askProgressFormatter) line(ev askLiveEvent) string {
	switch ev.Event {
	case "turn.started", "turn.completed", "thought":
		// Boundaries duplicate the dispatch/discovery lines and the final
		// completion line; thinking produces no caller-facing phrase.
		return ""
	case "tool.call":
		title := strings.TrimSpace(ev.Title)
		if title == "" {
			title = strings.TrimSpace(ev.ID)
		}
		if title == "" {
			return ""
		}
		f.toolTitles[ev.ID] = title
		f.toolsSeen++
		return f.render("tool: " + title)
	case "tool.update":
		status := strings.TrimSpace(ev.Status)
		if status == "" || status == "in_progress" {
			// Pending updates are the FRICTION.md pathology: ~48 identical
			// lines saying nothing. The tool.call line already announced the
			// action; only a resolution adds information.
			return ""
		}
		label := status
		if title := f.toolTitles[ev.ID]; title != "" {
			label = title + " · " + status
		}
		if ev.OutputSize > 0 {
			label += fmt.Sprintf(" (%d chars)", ev.OutputSize)
		}
		return f.render("tool " + label)
	case "text":
		excerpt := strings.TrimSpace(ev.TextExcerpt)
		if excerpt == "" {
			return ""
		}
		return f.render(excerpt)
	default:
		return ""
	}
}

// render stamps and collapses one visible line. Identical consecutive content
// collapses regardless of the elapsed stamp, so a retried or repeated event
// cannot rebuild the wall of duplicate lines FRICTION.md recorded. Trailing
// ellipsis variants of the same content dedupe too.
func (f *askProgressFormatter) render(content string) string {
	key := strings.TrimSuffix(content, "…")
	if key == f.lastKey {
		return ""
	}
	f.lastKey = key
	return f.stamp(content)
}

// stamp prefixes a rendered line with the elapsed time measured from ask
// dispatch ("[12s]", sub-second ages clamp to [0s]). Non-TTY lines carry
// askProgressPrefix so scripts can grep them out of a shared stream.
func (f *askProgressFormatter) stamp(content string) string {
	elapsed := formatAskElapsed(f.elapsed())
	if f.mode == askProgressTTY {
		return fmt.Sprintf("%s %s", elapsed, content)
	}
	return fmt.Sprintf("%s%s %s", askProgressPrefix, elapsed, content)
}

// truncateAskExcerpt collapses whitespace runs and truncates an excerpt to
// askProgressExcerptMax runes with a trailing ellipsis, so one event stays
// one scannable terminal line.
func truncateAskExcerpt(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= askProgressExcerptMax {
		return s
	}
	return string(runes[:askProgressExcerptMax]) + "…"
}

// formatAskElapsed renders seconds since turn start as "[12s]". Kept as a
// named helper so the table test pins the exact bracket format callers see.
func formatAskElapsed(d time.Duration) string {
	s := int(d.Seconds())
	if s < 0 {
		s = 0
	}
	return fmt.Sprintf("[%ds]", s)
}

// askProgressSpec locates the live.jsonl to stream for one ask.
type askProgressSpec struct {
	// sessionsDir is cfg.SessionsDir.
	sessionsDir string
	// session is the explicit --session value, empty when the ask is routed
	// by the daemon cascade (ADR-0030.1).
	session string
	// repo is the explicit --repo value, empty when absent.
	repo string
	// question is the exact question text; a routed ask recognizes its
	// session by the turn.started event carrying this question.
	question string
}

// dispatchLine is the immediate first-line signal for an explicit-session
// ask: something visible on stderr within the first millisecond, before any
// agent event can have landed. Routed asks render nothing here — their
// session line arrives at discovery (see discoverAskSession).
func (s askProgressSpec) dispatchLine(mode askProgressMode) string {
	if s.session == "" {
		return ""
	}
	if mode == askProgressTTY {
		return fmt.Sprintf("[0s] ▶ streaming %s activity…", s.session)
	}
	return fmt.Sprintf("%s[0s] streaming %s activity…", askProgressPrefix, s.session)
}

// resolve returns the live.jsonl path to tail. Explicit sessions resolve as
// soon as the file exists; routed asks resolve through turn.started discovery.
// ok is false while the path is not knowable/openable yet — the poller simply
// retries on its next tick.
func (s askProgressSpec) resolve() (path string, ok bool) {
	if s.session != "" {
		p := filepath.Join(s.sessionsDir, s.session, sidecar.LiveLogName)
		if _, err := os.Stat(p); err != nil {
			return "", false
		}
		return p, true
	}
	name, found := discoverAskSession(s.sessionsDir, s.repo, s.question)
	if !found {
		return "", false
	}
	return filepath.Join(s.sessionsDir, name, sidecar.LiveLogName), true
}

// discoverAskSession finds which session a daemon-routed ask landed in by
// scanning session dirs for a turn.started event carrying the exact question
// (and repo, when the ask named one). Among matches it prefers the most
// recent timestamp, so a re-ask resolves to the current turn's session. The
// scan reads live.jsonl files whole; they are bounded progress excerpts
// (256-byte windows per event), so this stays cheap.
func discoverAskSession(sessionsDir, repo, question string) (string, bool) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return "", false
	}
	bestName, bestTs := "", ""
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(sessionsDir, e.Name(), sidecar.LiveLogName))
		if err != nil {
			continue
		}
		lines, _ := splitAskCarry(data)
		for _, line := range lines {
			var ev askLiveEvent
			if json.Unmarshal([]byte(line), &ev) != nil || ev.Event != "turn.started" {
				continue
			}
			if ev.Question != question {
				continue
			}
			if repo != "" && ev.Repo != repo {
				continue
			}
			if bestName == "" || ev.Ts > bestTs {
				bestName, bestTs = e.Name(), ev.Ts
			}
		}
	}
	return bestName, bestName != ""
}

// askProgressWriter serializes poller output onto the caller's writer and
// signals the poller's exit, so stop() can guarantee no progress line
// interleaves with the final report.
type askProgressWriter struct {
	mu   sync.Mutex
	w    io.Writer
	once sync.Once
	done chan struct{}
}

func newAskProgressWriter(w io.Writer) *askProgressWriter {
	return &askProgressWriter{w: w, done: make(chan struct{})}
}

func (p *askProgressWriter) writeLine(line string) {
	if line == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintln(p.w, line)
}

// finish is the poller's exit hook; it runs exactly once.
func (p *askProgressWriter) finish() {
	p.once.Do(func() { close(p.done) })
}

// askLivePoller tails the session's live.jsonl while the ask runs, rendering
// one compact progress line per new event to w. It waits for the file to
// appear — routed asks don't know their session until turn.started lands —
// and stops when done closes. Termination is ordered: the caller's stop()
// blocks until this function has returned and flushed.
func askLivePoller(spec askProgressSpec, mode askProgressMode, done <-chan struct{}, w *askProgressWriter) {
	defer w.finish()

	f := newAskProgressFormatter(time.Now(), mode)
	if line := spec.dispatchLine(mode); line != "" {
		w.writeLine(line)
	}

	var (
		file  *os.File
		carry []byte
	)
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	ticker := time.NewTicker(askProgressPollInterval)
	defer ticker.Stop()
	idle := time.NewTicker(time.Second)
	defer idle.Stop()

	for {
		select {
		case <-done:
			return
		case <-idle.C:
			w.writeLine(f.idleLine())
			continue
		case <-ticker.C:
		}

		if file == nil {
			path, ok := spec.resolve()
			if !ok {
				continue
			}
			if spec.session == "" {
				// First contact with a routed ask: say which session won.
				dir := filepath.Base(filepath.Dir(path))
				w.writeLine(f.stamp(fmt.Sprintf("▶ session %s — streaming activity…", dir)))
				f.lastKey = "" // keep the first event line visible too
			}
			opened, err := os.Open(path)
			if err != nil {
				continue // transient (rotation, permissions): retry next tick
			}
			file = opened
		}

		chunk, err := io.ReadAll(file)
		if err != nil || len(chunk) == 0 {
			continue // best-effort surface: never fail the ask over progress
		}
		carry = append(carry, chunk...)
		lines, rest := splitAskCarry(carry)
		carry = rest
		for _, lineJSON := range lines {
			var ev askLiveEvent
			if json.Unmarshal([]byte(lineJSON), &ev) != nil {
				continue // malformed line: skip, never crash
			}
			f.noteActivity(time.Now())
			w.writeLine(f.line(ev))
		}
	}
}

// splitAskCarry splits buffered live.jsonl bytes into newline-terminated
// lines (blank lines dropped) plus the trailing partial line, which stays
// carried until its newline arrives — the same carry discipline `sidecar
// tail` uses, so a half-written event is never rendered mid-write.
func splitAskCarry(data []byte) (lines []string, rest []byte) {
	last := strings.LastIndexByte(string(data), '\n')
	if last < 0 {
		return nil, append([]byte(nil), data...)
	}
	for _, line := range strings.Split(string(data[:last]), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines, append([]byte(nil), data[last+1:]...)
}

// askProgress owns the background poller for one running ask. The CLI creates
// it via startAskProgress right before dispatching, and stops it the moment
// AskViaDaemon returns — before anything else prints — so progress lines can
// never interleave with the route line, the completion line, or the report.
type askProgress struct {
	done chan struct{}
	w    *askProgressWriter
	mode askProgressMode
	// start is the poller's dispatch moment; the completion line's elapsed
	// stamp is measured from it so it agrees with the streaming lines.
	start time.Time
}

// startAskProgress launches the poller. mode should come from stderrIsTTY().
func startAskProgress(spec askProgressSpec, mode askProgressMode, w io.Writer) *askProgress {
	pw := newAskProgressWriter(w)
	ap := &askProgress{done: make(chan struct{}), w: pw, mode: mode, start: time.Now()}
	go askLivePoller(spec, mode, ap.done, pw)
	return ap
}

// stop halts polling and waits for the poller goroutine to exit. Nil-safe:
// --quiet asks never started one.
func (ap *askProgress) stop() {
	if ap == nil {
		return
	}
	close(ap.done)
	<-ap.w.done
}

// completed writes the final progress line summarizing the delivered report
// (research 006 completion state: truthful counts from the report itself, so
// the summary lands before the body renders). Nil-safe; skipped entirely for
// failed asks — the error path already reports through cobra. Call it AFTER
// stop(): the writer mutex keeps this ordered after every streaming line.
func (ap *askProgress) completed(report *sidecar.Report) {
	if ap == nil || report == nil {
		return
	}
	elapsed := formatAskElapsed(time.Since(ap.start))
	detail := askReportSummary(report)
	var line string
	if ap.mode == askProgressTTY {
		line = fmt.Sprintf("%s ✔ report ready — %s", elapsed, detail)
	} else {
		line = fmt.Sprintf("%s%s report ready — %s", askProgressPrefix, elapsed, detail)
	}
	ap.w.writeLine(line)
}

// askReportSummary distills a delivered report into the truthful claim/citation
// counts shown on the completion line. A BLOCKED answer says so up front with
// a truncated reason instead of counts (there are none to truthfully count).
func askReportSummary(report *sidecar.Report) string {
	answer := strings.TrimSpace(report.Answer)
	if strings.HasPrefix(answer, "BLOCKED") {
		return truncateAskExcerpt(answer)
	}
	parts := []string{}
	switch {
	case len(report.Verified) > 0:
		parts = append(parts, fmt.Sprintf("%d verified claims", len(report.Verified)))
	case len(report.Inferred) > 0:
		parts = append(parts, fmt.Sprintf("%d inferred claims", len(report.Inferred)))
	}
	if n := len(report.Evidence); n > 0 {
		parts = append(parts, fmt.Sprintf("%d citations", n))
	}
	if len(parts) == 0 {
		return "no claims extracted"
	}
	return strings.Join(parts, ", ")
}
