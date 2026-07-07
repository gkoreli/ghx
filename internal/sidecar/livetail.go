// Live turn log tailing (`ghx sidecar tail`): the human-readable reader for
// the live.jsonl activity stream that live.go appends during a turn
// (ADR-0022.1). It resolves which session to watch, starts at the last
// turn.started boundary, and renders each NDJSON event as one concise,
// scannable line. The writer is best-effort, so the reader is too: a
// malformed or unknown line renders as a low-key fallback, never an error.
package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// liveTailPollInterval is how often --follow re-reads live.jsonl for appended
// lines. Plain polling keeps the reader dependency-free (no fsnotify), and
// 200ms is comfortably realtime for a human watching a multi-minute turn.
const liveTailPollInterval = 200 * time.Millisecond

// liveTailExcerptMax bounds the quoted question/text excerpt on a rendered
// line, so one event stays one scannable terminal line.
const liveTailExcerptMax = 60

// ResolveTailSession picks the session `ghx sidecar tail` watches when the
// caller named none: the session directory with the newest live.jsonl or
// meta.json mtime. Unlike meta updatedAt (bumped only when a turn completes),
// the live.jsonl mtime moves while a turn is still running, so a
// currently-running first ask is found immediately.
func ResolveTailSession(sessionsDir string) (string, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var (
		best     string
		bestTime time.Time
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		for _, name := range []string{LiveLogName, "meta.json"} {
			fi, err := os.Stat(filepath.Join(sessionsDir, e.Name(), name))
			if err != nil {
				continue
			}
			if best == "" || fi.ModTime().After(bestTime) {
				best, bestTime = e.Name(), fi.ModTime()
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no sessions found under %s — run `ghx sidecar ask` first", sessionsDir)
	}
	return best, nil
}

// liveTailEvent is the union decode target for every live.jsonl event shape
// written by live.go. Fields not carried by a given event type stay zero.
type liveTailEvent struct {
	Ts          string `json:"ts"`
	Event       string `json:"event"`
	Chars       int    `json:"chars"`
	TextExcerpt string `json:"textExcerpt"`
	ID          string `json:"id"`
	Title       string `json:"title"`
	Kind        string `json:"kind"`
	Status      string `json:"status"`
	OutputSize  int    `json:"outputSize"`
	Session     string `json:"session"`
	Repo        string `json:"repo"`
	Question    string `json:"question"`
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	Tools       int    `json:"tools"`
}

// LiveTailFormatter renders live.jsonl NDJSON lines as one concise human line
// per event. It is stateful — tool.call events register their id→title so
// later tool.update lines can name the tool — and therefore not safe for
// concurrent use; each tail owns its own formatter.
type LiveTailFormatter struct {
	loc        *time.Location
	toolTitles map[string]string
}

// NewLiveTailFormatter returns a formatter rendering timestamps in loc;
// nil means time.Local.
func NewLiveTailFormatter(loc *time.Location) *LiveTailFormatter {
	if loc == nil {
		loc = time.Local
	}
	return &LiveTailFormatter{loc: loc, toolTitles: map[string]string{}}
}

// FormatLine renders one NDJSON line. Blank lines render as "" (callers skip
// them); malformed JSON or an event without a type renders as a single
// low-key fallback line — the reader must never crash on a best-effort writer.
func (f *LiveTailFormatter) FormatLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	var ev liveTailEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.Event == "" {
		return "--:--:-- · unparsed: " + liveTailTruncate(line, 80)
	}
	ts := f.formatTs(ev.Ts)
	switch ev.Event {
	case "turn.started":
		name := ev.Session
		if name == "" {
			name = ev.Repo
		}
		parts := []string{ts + " ▶ turn started"}
		if name != "" {
			parts = append(parts, name)
		}
		if ev.Question != "" {
			parts = append(parts, fmt.Sprintf("%q", liveTailTruncate(ev.Question, liveTailExcerptMax)))
		}
		return strings.Join(parts, "  ")
	case "text":
		out := fmt.Sprintf("%s ✎ text (%s chars)", ts, humanChars(ev.Chars))
		if ev.TextExcerpt != "" {
			out += fmt.Sprintf("  %q", liveTailTruncate(ev.TextExcerpt, liveTailExcerptMax))
		}
		return out
	case "thought":
		return fmt.Sprintf("%s 💭 thinking (%s chars)", ts, humanChars(ev.Chars))
	case "tool.call":
		if ev.Title != "" {
			f.toolTitles[ev.ID] = ev.Title
		}
		parts := []string{ts + " ⚙ " + f.toolLabel(ev.ID, ev.Title)}
		if ev.Kind != "" {
			parts = append(parts, "["+ev.Kind+"]")
		}
		if ev.Status != "" {
			parts = append(parts, ev.Status)
		}
		return strings.Join(parts, " ")
	case "tool.update":
		status := ev.Status
		if status == "" {
			status = "update"
		}
		out := fmt.Sprintf("%s ⚙ %s %s", ts, f.toolLabel(ev.ID, ""), status)
		if ev.OutputSize > 0 {
			out += fmt.Sprintf(" (%s chars)", humanChars(ev.OutputSize))
		}
		return out
	case "turn.completed":
		if !ev.OK {
			msg := ev.Error
			if msg == "" {
				msg = "unknown error"
			}
			return fmt.Sprintf("%s ✖ turn FAILED: %s", ts, msg)
		}
		return fmt.Sprintf("%s ✔ turn completed  %d tools  %s chars", ts, ev.Tools, humanChars(ev.Chars))
	default:
		// Unknown-but-valid event (a newer writer): name it, don't fail.
		return ts + " · " + ev.Event
	}
}

// toolLabel names a tool line: the call's own title, a title remembered from
// its tool.call, or the raw id, or "…" when even that is missing.
func (f *LiveTailFormatter) toolLabel(id, title string) string {
	if title != "" {
		return title
	}
	if remembered := f.toolTitles[id]; remembered != "" {
		return remembered
	}
	if id != "" {
		return id
	}
	return "…"
}

// formatTs renders an RFC3339Nano timestamp as local wall-clock HH:MM:SS;
// an unparsable timestamp renders as a placeholder instead of failing.
func (f *LiveTailFormatter) formatTs(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return "--:--:--"
	}
	return t.In(f.loc).Format("15:04:05")
}

// humanChars renders a character count compactly: 412, 8.2k, 1.5M.
func humanChars(n int) string {
	switch {
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1_000_000), ".0") + "M"
	case n >= 1_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1_000), ".0") + "k"
	default:
		return fmt.Sprintf("%d", n)
	}
}

// liveTailTruncate collapses whitespace runs to single spaces and truncates
// to max runes with an ellipsis, so excerpts stay on one terminal line.
func liveTailTruncate(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

// lastTurnStartIndex returns the index in lines of the last "turn.started"
// event, or 0 when none exists — tailing then shows everything it has rather
// than nothing.
func lastTurnStartIndex(lines []string) int {
	start := 0
	for i, line := range lines {
		var ev struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err == nil && ev.Event == "turn.started" {
			start = i
		}
	}
	return start
}

// LiveTailOptions configures TailLiveLog.
type LiveTailOptions struct {
	// Follow keeps reading appended events until ctx is done, waiting for the
	// file to appear when it does not exist yet. Without it, the last turn's
	// events are printed and the call returns.
	Follow bool
	// Raw prints the NDJSON lines verbatim instead of rendering them.
	Raw bool
	// Loc is the timezone for rendered timestamps; nil means time.Local.
	Loc *time.Location
}

// TailLiveLog prints a session's live turn log (live.jsonl, ADR-0022.1) to w,
// starting at the last turn.started boundary — so following during a running
// ask shows the current turn from its start, and a plain tail replays the
// last turn. Follow mode returns nil when ctx is cancelled (the user's
// Ctrl-C, not a failure).
func TailLiveLog(ctx context.Context, w io.Writer, path string, opts LiveTailOptions) error {
	f := NewLiveTailFormatter(opts.Loc)
	if !opts.Follow {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return fmt.Errorf("no live turn log at %s — no turn activity recorded yet (use --follow to wait for one)", path)
		}
		if err != nil {
			return err
		}
		lines, carry := splitLiveLines(data)
		if strings.TrimSpace(string(carry)) != "" {
			// A finished file normally ends in a newline; keep a truncated
			// final line (crashed writer) visible rather than dropping it.
			lines = append(lines, string(carry))
		}
		for _, line := range lines[lastTurnStartIndex(lines):] {
			printLiveLine(w, f, line, opts.Raw)
		}
		return nil
	}
	return followLiveLog(ctx, w, f, path, opts.Raw)
}

// followLiveLog waits for the file when absent, replays the last turn, then
// polls for appended lines until ctx is done.
func followLiveLog(ctx context.Context, w io.Writer, f *LiveTailFormatter, path string, raw bool) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Fprintf(w, "(waiting for %s — no turn activity yet)\n", path)
		if !waitForLiveLog(ctx, path) {
			return nil
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Replay what already exists from the last turn boundary, keeping any
	// trailing partial line buffered until its newline arrives.
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	lines, carry := splitLiveLines(data)
	for _, line := range lines[lastTurnStartIndex(lines):] {
		printLiveLine(w, f, line, raw)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(liveTailPollInterval):
		}
		chunk, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		if len(chunk) == 0 {
			continue
		}
		carry = append(carry, chunk...)
		lines, carry = splitLiveLines(carry)
		for _, line := range lines {
			printLiveLine(w, f, line, raw)
		}
	}
}

// waitForLiveLog polls until path exists; false means ctx ended first.
func waitForLiveLog(ctx context.Context, path string) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(liveTailPollInterval):
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
}

// splitLiveLines splits NDJSON bytes into complete lines plus the trailing
// partial line (bytes after the last newline), which follow mode carries
// until the writer finishes it.
func splitLiveLines(data []byte) (lines []string, carry []byte) {
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

// printLiveLine writes one event line: verbatim in raw mode, rendered
// otherwise. Lines that render empty are skipped.
func printLiveLine(w io.Writer, f *LiveTailFormatter, line string, raw bool) {
	if raw {
		fmt.Fprintln(w, line)
		return
	}
	if out := f.FormatLine(line); out != "" {
		fmt.Fprintln(w, out)
	}
}
