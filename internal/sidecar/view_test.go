package sidecar

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// makeViewSession builds a fake session directory. reports files land under
// reports/ (the ADR-0022 layout); traces/logs/metrics are one-line JSONL files
// when requested.
func makeViewSession(t *testing.T, sessionsDir, name string, turns int, updatedAt string, reports int, kinds ...string) {
	t.Helper()
	dir := filepath.Join(sessionsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := SessionMeta{Name: name, Repo: "o/" + name, TurnCount: turns, CreatedAt: updatedAt, UpdatedAt: updatedAt}
	data, _ := json.Marshal(meta)
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if reports > 0 {
		rdir := filepath.Join(dir, "reports")
		if err := os.MkdirAll(rdir, 0o755); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < reports; i++ {
			path := filepath.Join(rdir, string(rune('1'+i))+"-100.json")
			if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, kind := range kinds {
		line := `{"resourceSpans":[]}` + "\n"
		if err := os.WriteFile(filepath.Join(dir, kind+".jsonl"), []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestListViewSessions(t *testing.T) {
	dir := t.TempDir()
	makeViewSession(t, dir, "older", 3, "2026-07-01T00:00:00Z", 2, "traces", "logs")
	makeViewSession(t, dir, "newer", 5, "2026-07-04T00:00:00Z", 1, "traces")
	makeViewSession(t, dir, "no-traces", 1, "2026-07-02T00:00:00Z", 0)
	// A stray file at the top level must be ignored.
	if err := os.WriteFile(filepath.Join(dir, "junk.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	sessions, err := ListViewSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 3 {
		t.Fatalf("got %d sessions, want 3: %+v", len(sessions), sessions)
	}
	// Newest first by UpdatedAt.
	wantOrder := []string{"newer", "no-traces", "older"}
	for i, want := range wantOrder {
		if sessions[i].Name != want {
			t.Errorf("order[%d] = %q, want %q", i, sessions[i].Name, want)
		}
	}
	byName := map[string]ViewSession{}
	for _, s := range sessions {
		byName[s.Name] = s
	}
	if s := byName["older"]; s.Turns != 3 || s.Reports != 2 || !s.HasTraces {
		t.Errorf("older = %+v, want turns=3 reports=2 traces", s)
	}
	if s := byName["no-traces"]; s.HasTraces {
		t.Errorf("no-traces flagged as having traces: %+v", s)
	}
}

func TestListViewSessionsArtifactOnlyDir(t *testing.T) {
	// A directory with traces.jsonl but no meta.json stays viewable.
	dir := t.TempDir()
	sdir := filepath.Join(dir, "artifact-only")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sdir, "traces.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A directory with neither meta nor artifacts is not a session.
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	sessions, err := ListViewSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Name != "artifact-only" || !sessions[0].HasTraces {
		t.Fatalf("got %+v, want single artifact-only session with traces", sessions)
	}
	if sessions[0].UpdatedAt == "" {
		t.Error("artifact-only session has empty UpdatedAt; want traces mtime")
	}
}

func TestListViewSessionsMissingDir(t *testing.T) {
	sessions, err := ListViewSessions(filepath.Join(t.TempDir(), "nope"))
	if err != nil || sessions != nil {
		t.Fatalf("got (%v, %v), want (nil, nil)", sessions, err)
	}
}

func TestResolveViewSessionNamed(t *testing.T) {
	dir := t.TempDir()
	makeViewSession(t, dir, "alpha", 1, "2026-07-01T00:00:00Z", 0, "traces")
	makeViewSession(t, dir, "beta", 2, "2026-07-02T00:00:00Z", 0, "traces")

	s, err := ResolveViewSession(dir, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "alpha" {
		t.Errorf("resolved %q, want alpha", s.Name)
	}
	if want := filepath.Join(dir, "alpha", "traces.jsonl"); s.TracesPath() != want {
		t.Errorf("TracesPath = %q, want %q", s.TracesPath(), want)
	}
}

func TestResolveViewSessionDefaultMostRecent(t *testing.T) {
	dir := t.TempDir()
	makeViewSession(t, dir, "old", 1, "2026-07-01T00:00:00Z", 0, "traces")
	makeViewSession(t, dir, "new", 2, "2026-07-03T00:00:00Z", 0, "traces")

	s, err := ResolveViewSession(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "new" {
		t.Errorf("resolved %q, want new (most recent)", s.Name)
	}
}

func TestResolveViewSessionNoTracesMentionsAlternatives(t *testing.T) {
	dir := t.TempDir()
	makeViewSession(t, dir, "has-traces", 1, "2026-07-01T00:00:00Z", 0, "traces")
	makeViewSession(t, dir, "bare", 1, "2026-07-04T00:00:00Z", 0)

	// Named session without traces.
	_, err := ResolveViewSession(dir, "bare")
	if err == nil || !strings.Contains(err.Error(), "has-traces") {
		t.Errorf("error %v should name the sessions that DO have traces", err)
	}
	// Default resolution picks "bare" (most recent) which has no traces.
	_, err = ResolveViewSession(dir, "")
	if err == nil || !strings.Contains(err.Error(), "has-traces") {
		t.Errorf("error %v should name the sessions that DO have traces", err)
	}
}

func TestResolveViewSessionErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := ResolveViewSession(dir, ""); err == nil || !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("empty dir: got %v, want no-sessions error", err)
	}
	makeViewSession(t, dir, "only", 1, "2026-07-01T00:00:00Z", 0, "traces")
	if _, err := ResolveViewSession(dir, "missing"); err == nil || !strings.Contains(err.Error(), "only") {
		t.Errorf("unknown name: got %v, want error listing available sessions", err)
	}
}

func TestFormatViewSessions(t *testing.T) {
	out := FormatViewSessions([]ViewSession{
		{Name: "alpha", Turns: 4, Reports: 2, HasTraces: true, UpdatedAt: "2026-07-01T00:00:00Z"},
		{Name: "beta", Turns: 0, Reports: 0, HasTraces: false, UpdatedAt: "2026-06-30T00:00:00Z"},
	})
	for _, want := range []string{"alpha", "turns=4", "reports=2", "traces", "beta", "no-traces"} {
		if !strings.Contains(out, want) {
			t.Errorf("list output missing %q:\n%s", want, out)
		}
	}
	if got := FormatViewSessions(nil); got != "(no sessions)" {
		t.Errorf("empty list output = %q", got)
	}
}

func TestReplayFilePostsEachLine(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	var bodies []string
	var contentTypes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		paths = append(paths, r.URL.Path)
		bodies = append(bodies, string(body))
		contentTypes = append(contentTypes, r.Header.Get("Content-Type"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "traces.jsonl")
	lines := `{"resourceSpans":[{"a":1}]}` + "\n\n" + `{"resourceSpans":[{"a":2}]}` + "\n"
	if err := os.WriteFile(path, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ReplayFile(context.Background(), srv.Client(), srv.URL+"/v1/traces", path)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("posted %d lines, want 2 (blank line skipped)", n)
	}
	if len(bodies) != 2 || bodies[0] != `{"resourceSpans":[{"a":1}]}` || bodies[1] != `{"resourceSpans":[{"a":2}]}` {
		t.Errorf("bodies = %q", bodies)
	}
	for _, p := range paths {
		if p != "/v1/traces" {
			t.Errorf("POST path = %q, want /v1/traces", p)
		}
	}
	for _, ct := range contentTypes {
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	}
}

func TestReplayFileRejectedLine(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad payload", http.StatusBadRequest)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "traces.jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := ReplayFile(context.Background(), srv.Client(), srv.URL+"/v1/traces", path)
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("got (%d, %v), want non-2xx error mentioning status", n, err)
	}
	if n != 0 {
		t.Errorf("posted = %d, want 0", n)
	}
}

func TestReplaySession(t *testing.T) {
	var mu sync.Mutex
	perPath := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		perPath[r.URL.Path]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	sessionsDir := t.TempDir()
	// traces + logs present, metrics absent.
	makeViewSession(t, sessionsDir, "s1", 1, "2026-07-01T00:00:00Z", 0, "traces", "logs")

	counts, err := ReplaySession(context.Background(), srv.Client(), srv.URL, filepath.Join(sessionsDir, "s1"))
	if err != nil {
		t.Fatal(err)
	}
	if counts["traces"] != 1 || counts["logs"] != 1 || counts["metrics"] != 0 {
		t.Errorf("counts = %v, want traces=1 logs=1 metrics=0", counts)
	}
	if perPath["/v1/traces"] != 1 || perPath["/v1/logs"] != 1 || perPath["/v1/metrics"] != 0 {
		t.Errorf("ingest paths hit = %v", perPath)
	}
}

func TestReplaySessionMissingTraces(t *testing.T) {
	sessionsDir := t.TempDir()
	makeViewSession(t, sessionsDir, "bare", 1, "2026-07-01T00:00:00Z", 0)
	_, err := ReplaySession(context.Background(), http.DefaultClient, "http://localhost:0", filepath.Join(sessionsDir, "bare"))
	if err == nil || !strings.Contains(err.Error(), "traces.jsonl") {
		t.Errorf("got %v, want missing-traces error", err)
	}
}

func TestWaitForIngest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed) // any response means the listener is up
	}))
	defer srv.Close()
	if err := WaitForIngest(context.Background(), srv.Client(), srv.URL, time.Second); err != nil {
		t.Errorf("up listener: %v", err)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	down.Close() // port is now refused
	err := WaitForIngest(context.Background(), http.DefaultClient, down.URL, 300*time.Millisecond)
	if err == nil {
		t.Error("closed listener: want timeout error")
	}
}
