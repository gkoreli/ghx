package sidecar

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestAskEmitsSessionArtifacts pins ADR-0022 D2: a production Ask turn appends
// spec-exact OTLP artifacts to the session directory (traces/logs/metrics) and
// persists the accepted report under reports/<turn>-<ts>.json — with no
// reward/evaluation events. It drives the real Ask code path with a mock turn.
func TestAskEmitsSessionArtifacts(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		now := time.Now().UTC()
		return TurnResult{
			FullText: `Here is what I found. <ghx-report>{"answer":"routes live in router.go","verified":[],"relevantFiles":[{"path":"router.go","reason":"defines routes"}]}</ghx-report>`,
			Thinking: "inspect the router files",
			ToolTraces: []ToolCallTrace{{
				ID:            "call-1",
				Kind:          "read",
				Title:         "read router.go",
				OutputSize:    42,
				OutputExcerpt: "package main // router",
				StatusTransitions: []ToolStatusTransition{
					{Status: "in_progress", At: now},
					{Status: "completed", At: now.Add(5 * time.Millisecond)},
				},
			}},
			ToolOutputChars: 42,
		}, "sess-1", nil
	}

	report, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock", Model: "test-model"}, AskRequest{
		Session:  "s",
		Repo:     "o/r",
		Question: "where are routes configured?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Answer != "routes live in router.go" {
		t.Fatalf("answer = %q", report.Answer)
	}

	sess := filepath.Join(dir, "s")

	// traces.jsonl: session + turn + tool spans, spec hex IDs, no eval spans.
	lines, spans := readProdSpans(t, filepath.Join(sess, "traces.jsonl"))
	if lines < 1 {
		t.Fatalf("traces.jsonl lines = %d, want >= 1", lines)
	}
	names := map[string]prodSpan{}
	for _, sp := range spans {
		names[sp.Name] = sp
	}
	for _, want := range []string{"sidecar.ask", "sidecar.turn", "tool.read"} {
		if _, ok := names[want]; !ok {
			t.Fatalf("missing span %q; got %v", want, spanNames(spans))
		}
	}
	if !hexTraceRE.MatchString(names["sidecar.turn"].TraceID) {
		t.Fatalf("turn traceId = %q, want 32-char hex", names["sidecar.turn"].TraceID)
	}
	for _, sp := range spans {
		if strings.HasPrefix(sp.Name, "eval.") || sp.Name == "eval.reward.compute" {
			t.Fatalf("production traces must not contain eval span %q", sp.Name)
		}
	}

	// reports/<turn>-<ts>.json persisted (turn 1 for a fresh session).
	reportsDir := filepath.Join(sess, "reports")
	entries, err := os.ReadDir(reportsDir)
	if err != nil {
		t.Fatalf("read reports dir: %v", err)
	}
	var reportFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "1-") && strings.HasSuffix(e.Name(), ".json") {
			reportFile = e.Name()
		}
	}
	if reportFile == "" {
		t.Fatalf("no reports/1-*.json persisted; got %v", entries)
	}

	// logs.jsonl: GenAI content record present (capture-content default on).
	logData, err := os.ReadFile(filepath.Join(sess, "logs.jsonl"))
	if err != nil {
		t.Fatalf("read logs.jsonl: %v", err)
	}
	if !strings.Contains(string(logData), "gen_ai.input.messages") {
		t.Fatalf("logs.jsonl missing content record: %s", logData)
	}

	// metrics.jsonl: token usage metric present.
	metricData, err := os.ReadFile(filepath.Join(sess, "metrics.jsonl"))
	if err != nil {
		t.Fatalf("read metrics.jsonl: %v", err)
	}
	if !strings.Contains(string(metricData), "gen_ai.client.token.usage") {
		t.Fatalf("metrics.jsonl missing token usage metric: %s", metricData)
	}
}

// TestAskContentCaptureDisabled pins ADR-0022 D4: setting the official OTel env
// var to false suppresses the GenAI content log records in production while
// traces and metrics stay on.
func TestAskContentCaptureDisabled(t *testing.T) {
	stubHandshake(t)
	t.Setenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT", "false")
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		return TurnResult{FullText: `<ghx-report>{"answer":"ok"}</ghx-report>`, Thinking: "x"}, "sess-1", nil
	}

	if _, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	}); err != nil {
		t.Fatal(err)
	}
	sess := filepath.Join(dir, "s")
	if _, err := os.Stat(filepath.Join(sess, "traces.jsonl")); err != nil {
		t.Fatalf("traces.jsonl should still be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sess, "logs.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("logs.jsonl should not exist when content capture is disabled (err=%v)", err)
	}
}

var hexTraceRE = regexp.MustCompile(`^[0-9a-f]{32}$`)

type prodSpan struct {
	TraceID string `json:"traceId"`
	SpanID  string `json:"spanId"`
	Name    string `json:"name"`
}

func readProdSpans(t *testing.T, path string) (lines int, spans []prodSpan) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines++
		var td struct {
			ResourceSpans []struct {
				ScopeSpans []struct {
					Spans []prodSpan `json:"spans"`
				} `json:"scopeSpans"`
			} `json:"resourceSpans"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &td); err != nil {
			t.Fatalf("line %d is not OTLP TracesData JSON: %v", lines, err)
		}
		for _, rs := range td.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				spans = append(spans, ss.Spans...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return lines, spans
}

func spanNames(spans []prodSpan) []string {
	out := make([]string, 0, len(spans))
	for _, sp := range spans {
		out = append(out, sp.Name)
	}
	return out
}
