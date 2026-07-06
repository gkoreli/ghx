// Session artifact viewing (ADR-0026.1): resolve a session's committed OTel
// artifacts and replay them into a local OTLP viewer. This absorbs the
// ADR-0018 "Live validation" curl recipe into the product — `ghx sidecar view`
// is the recipe as one command (the Inspect AI `inspect view` pattern).
package sidecar

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// tracesFile is the per-session OTLP trace artifact (ADR-0022 D2/D3).
const tracesFile = "traces.jsonl"

// replayKinds are the OTLP signal kinds replayed from a session directory,
// in replay order. Each maps to a `<kind>.jsonl` artifact and an OTLP HTTP
// `/v1/<kind>` ingest path. Traces are mandatory; logs and metrics are
// replayed when present.
var replayKinds = []string{"traces", "logs", "metrics"}

// ViewSession describes one session directory for `ghx sidecar view`.
type ViewSession struct {
	// Name is the session slug (directory name under sessionsDir).
	Name string
	// Dir is the absolute session directory.
	Dir string
	// Turns is the completed turn count from meta.json (0 when meta is absent).
	Turns int
	// Reports is the number of persisted report files (ADR-0021).
	Reports int
	// HasTraces reports whether traces.jsonl exists.
	HasTraces bool
	// UpdatedAt orders sessions; meta.json updatedAt when present, otherwise
	// the traces.jsonl modification time.
	UpdatedAt string
}

// TracesPath returns the session's traces.jsonl path.
func (v ViewSession) TracesPath() string { return filepath.Join(v.Dir, tracesFile) }

// ListViewSessions returns every session directory under sessionsDir with
// turn/report counts and trace presence, newest first. Unlike ListSessions it
// keeps directories without meta.json when they carry a traces.jsonl, so
// artifact-only sessions remain viewable.
func ListViewSessions(sessionsDir string) ([]ViewSession, error) {
	entries, err := os.ReadDir(sessionsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []ViewSession
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v := ViewSession{Name: e.Name(), Dir: filepath.Join(sessionsDir, e.Name())}
		if fi, err := os.Stat(v.TracesPath()); err == nil {
			v.HasTraces = true
			v.UpdatedAt = fi.ModTime().UTC().Format(time.RFC3339)
		}
		meta, err := ReadMeta(sessionsDir, e.Name())
		if err == nil && meta != nil {
			v.Turns = meta.TurnCount
			if meta.UpdatedAt != "" {
				v.UpdatedAt = meta.UpdatedAt
			}
		} else if !v.HasTraces {
			// Neither session metadata nor artifacts: not a session directory.
			continue
		}
		if reports, err := ListReports(sessionsDir, e.Name()); err == nil {
			v.Reports = len(reports)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ResolveViewSession picks the session to view. Empty name means the most
// recent session. The returned session always has traces.jsonl; otherwise the
// error names the sessions that DO have traces so the user can pick one.
func ResolveViewSession(sessionsDir, name string) (ViewSession, error) {
	sessions, err := ListViewSessions(sessionsDir)
	if err != nil {
		return ViewSession{}, err
	}
	if len(sessions) == 0 {
		return ViewSession{}, fmt.Errorf("no sessions found under %s — run `ghx sidecar ask` first", sessionsDir)
	}
	var picked *ViewSession
	if name == "" {
		picked = &sessions[0]
	} else {
		for i := range sessions {
			if sessions[i].Name == name {
				picked = &sessions[i]
				break
			}
		}
		if picked == nil {
			return ViewSession{}, fmt.Errorf("session %q not found under %s; available: %s",
				name, sessionsDir, strings.Join(sessionNames(sessions), ", "))
		}
	}
	if !picked.HasTraces {
		withTraces := sessionNames(filterWithTraces(sessions))
		if len(withTraces) == 0 {
			return ViewSession{}, fmt.Errorf("session %q has no traces.jsonl, and no session under %s has traces yet",
				picked.Name, sessionsDir)
		}
		return ViewSession{}, fmt.Errorf("session %q has no traces.jsonl; sessions with traces: %s",
			picked.Name, strings.Join(withTraces, ", "))
	}
	return *picked, nil
}

func filterWithTraces(sessions []ViewSession) []ViewSession {
	var out []ViewSession
	for _, s := range sessions {
		if s.HasTraces {
			out = append(out, s)
		}
	}
	return out
}

func sessionNames(sessions []ViewSession) []string {
	names := make([]string, len(sessions))
	for i, s := range sessions {
		names[i] = s.Name
	}
	return names
}

// FormatViewSessions renders the `ghx sidecar view --list` table.
func FormatViewSessions(sessions []ViewSession) string {
	if len(sessions) == 0 {
		return "(no sessions)"
	}
	var b strings.Builder
	for _, s := range sessions {
		traces := "traces"
		if !s.HasTraces {
			traces = "no-traces"
		}
		fmt.Fprintf(&b, "%-32s  turns=%-3d  reports=%-3d  %-9s  %s\n",
			s.Name, s.Turns, s.Reports, traces, s.UpdatedAt)
	}
	return strings.TrimRight(b.String(), "\n")
}

// WaitForIngest polls the viewer's OTLP HTTP ingest until it accepts TCP/HTTP
// requests or the timeout expires. Any HTTP response counts as up — we only
// need the listener, not a particular status.
func WaitForIngest(ctx context.Context, client *http.Client, ingestBase string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := strings.TrimRight(ingestBase, "/") + "/v1/traces"
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("OTLP ingest at %s did not come up within %s: %w", ingestBase, timeout, err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// ReplayFile posts each non-blank line of path (spec OTLP JSON, one export
// request per line — ADR-0018 "Live validation") to url and returns the number
// of lines posted. Any non-2xx response aborts with the offending line number.
func ReplayFile(ctx context.Context, client *http.Client, url, path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	posted, lineNo := 0, 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(line))
		if err != nil {
			return posted, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return posted, fmt.Errorf("%s line %d: %w", filepath.Base(path), lineNo, err)
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4*1024))
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return posted, fmt.Errorf("%s line %d: ingest rejected with %s: %s",
				filepath.Base(path), lineNo, resp.Status, strings.TrimSpace(string(body)))
		}
		posted++
	}
	if err := scanner.Err(); err != nil {
		return posted, fmt.Errorf("read %s: %w", path, err)
	}
	return posted, nil
}

// ReplaySession replays a session's OTLP artifacts into the viewer ingest at
// ingestBase (e.g. http://localhost:4318). traces.jsonl is required;
// logs.jsonl and metrics.jsonl are replayed when present. Returns lines posted
// per kind.
func ReplaySession(ctx context.Context, client *http.Client, ingestBase, sessionDir string) (map[string]int, error) {
	base := strings.TrimRight(ingestBase, "/")
	counts := map[string]int{}
	for _, kind := range replayKinds {
		path := filepath.Join(sessionDir, kind+".jsonl")
		if _, err := os.Stat(path); err != nil {
			if kind == "traces" {
				return counts, fmt.Errorf("no %s in %s: %w", tracesFile, sessionDir, err)
			}
			continue
		}
		n, err := ReplayFile(ctx, client, base+"/v1/"+kind, path)
		counts[kind] = n
		if err != nil {
			return counts, err
		}
	}
	return counts, nil
}
