package evals

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const knownSecretValue = "super-secret-eval-token"

func loadTraceData(t *testing.T, runDir string) []*tracepb.TracesData {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runDir, traceFileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), knownSecretValue) {
		t.Fatalf("traces.jsonl contains known secret value %q", knownSecretValue)
	}
	var out []*tracepb.TracesData
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var td tracepb.TracesData
		if err := protojson.Unmarshal([]byte(line), &td); err != nil {
			t.Fatalf("parse traces.jsonl line %d: %v\n%s", i+1, err, line)
		}
		out = append(out, &td)
	}
	if len(out) == 0 {
		t.Fatal("traces.jsonl had no records")
	}
	return out
}

func flattenTraceSpans(data []*tracepb.TracesData) (map[string]*tracepb.Span, map[string]string) {
	spans := map[string]*tracepb.Span{}
	resourceAttrs := map[string]string{}
	for _, td := range data {
		for _, rs := range td.ResourceSpans {
			for _, attr := range rs.GetResource().GetAttributes() {
				resourceAttrs[attr.Key] = anyValueString(attr.Value)
			}
			for _, ss := range rs.ScopeSpans {
				for _, sp := range ss.Spans {
					spans[string(sp.SpanId)] = sp
				}
			}
		}
	}
	return spans, resourceAttrs
}

func spanByName(t *testing.T, spans map[string]*tracepb.Span, name string) *tracepb.Span {
	t.Helper()
	for _, sp := range spans {
		if sp.Name == name {
			return sp
		}
	}
	t.Fatalf("span %q not found; got %v", name, spanNames(spans))
	return nil
}

func spansByNamePrefix(spans map[string]*tracepb.Span, prefix string) []*tracepb.Span {
	var out []*tracepb.Span
	for _, sp := range spans {
		if strings.HasPrefix(sp.Name, prefix) {
			out = append(out, sp)
		}
	}
	return out
}

func spanNames(spans map[string]*tracepb.Span) []string {
	var out []string
	for _, sp := range spans {
		out = append(out, sp.Name)
	}
	return out
}

func spanAttr(sp *tracepb.Span, key string) string {
	for _, attr := range sp.Attributes {
		if attr.Key == key {
			return anyValueString(attr.Value)
		}
	}
	return ""
}

func anyValueString(v *commonpb.AnyValue) string {
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(x.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(x.DoubleValue, 'f', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	default:
		return ""
	}
}

// buildMockAgent compiles the scripted ACP agent into a temp dir.
// Skips the test when no Go toolchain is available (the mock e2e is a
// plumbing test, not a build-environment requirement).
func buildMockAgent(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available; skipping mock e2e")
	}
	bin := filepath.Join(t.TempDir(), "mockagent")
	cmd := exec.Command("go", "build", "-o", bin, "./mockagent")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mockagent: %v\n%s", err, out)
	}
	return bin
}

// writeScript serializes scripted replies and points the mock agent at them
// via environment (inherited by the spawned agent processes).
func writeScript(t *testing.T, replies []map[string]any) {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(replies)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "script.json")
	if err := os.WriteFile(script, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MOCKAGENT_SCRIPT", script)
	t.Setenv("MOCKAGENT_STATE", filepath.Join(dir, "state"))
}

func honoTask(t *testing.T) Task {
	t.Helper()
	tasks, err := LoadTasks("testdata/tasks")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range tasks {
		if task.ID == "hono-middleware" {
			return task
		}
	}
	t.Fatal("hono-middleware task not found")
	return Task{}
}

const turn0Report = `Investigating middleware composition.

<ghx-report>
{
  "answer": "Middleware composition is implemented in src/compose.ts; hono-base wires it with error handling via dispatch.",
  "verified": [
    {"summary": "compose() builds the dispatch chain", "evidence": "src/compose.ts: dispatch(i) recursion"},
    {"summary": "hono-base calls compose with onError", "evidence": "src/hono-base.ts: compose(matchResult[0], this.errorHandler)"}
  ],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [
    {"path": "src/compose.ts", "reason": "defines compose() and dispatch"},
    {"path": "src/hono-base.ts", "reason": "invokes compose with error handler"}
  ],
  "evidence": [
    {"source": "ghx read honojs/hono src/compose.ts --map", "summary": "compose signature and dispatch loop"}
  ],
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx tree honojs/hono src --depth 2", "ghx read honojs/hono src/compose.ts --map"],
  "uncertainty": ["did not inspect tests"],
  "nextReads": ["src/hono-base.ts"]
}
</ghx-report>`

const turn1Report = `Building on prior findings about src/compose.ts.

<ghx-report>
{
  "answer": "Errors propagate through onError: dispatch catches throws and routes them to the error handler composed in src/hono-base.ts.",
  "verified": [
    {"summary": "onError receives errors from the dispatch chain", "evidence": "src/hono-base.ts: #handleError via onError"}
  ],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [
    {"path": "src/hono-base.ts", "reason": "error handler composition"}
  ],
  "evidence": [
    {"source": "ghx read honojs/hono src/hono-base.ts --grep onError", "summary": "error handler wiring"}
  ],
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx read honojs/hono src/hono-base.ts --grep onError"],
  "uncertainty": [],
  "nextReads": []
}
</ghx-report>`

// TestMockSidecarEpisode drives the full production sidecar path — spawn
// agent, ACP handshake, NewSession/LoadSession, prompt, tool-call events,
// report extraction, session persistence, rewards, artifact — against the
// scripted mock agent. This is the ADR-0016.1 checkpoint-d proof that the
// episode pipeline works end-to-end without a live LLM.
func TestMockSidecarEpisode(t *testing.T) {
	t.Setenv("GHX_EVAL_KNOWN_SECRET", knownSecretValue)
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "mock-sonnet-subject")
	bin := buildMockAgent(t)
	writeScript(t, []map[string]any{
		{
			"toolCalls": []string{
				"ghx tree honojs/hono src --depth 2",
				"ghx read honojs/hono src/compose.ts --map",
			},
			"text": turn0Report,
		},
		{
			"replayOnLoad": []string{"ghx read honojs/hono src/compose.ts --map"},
			"toolCalls":    []string{"ghx read honojs/hono src/hono-base.ts --grep onError"},
			"text":         turn1Report,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()}
	task := honoTask(t)

	ep, err := RunEpisode(ctx, cfg, task, ProfileSidecar)
	if err != nil {
		t.Fatalf("episode failed: %v", err)
	}

	if len(ep.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(ep.Turns))
	}
	if ep.Report == nil {
		t.Fatal("no final report extracted")
	}
	if ep.Turns[0].Report == nil || ep.Turns[1].Report == nil {
		t.Error("per-turn reports missing")
	}
	if !ep.Turns[1].Resumed {
		t.Error("follow-up turn did not resume the ACP session (LoadSession path)")
	}
	if len(ep.Turns[0].ToolCalls) != 2 {
		t.Errorf("turn 0 tool calls = %v, want 2 captured", ep.Turns[0].ToolCalls)
	}
	if len(ep.Turns[1].ToolCalls) != 1 || strings.Contains(strings.Join(ep.Turns[1].ToolCalls, "\n"), "src/compose.ts") {
		t.Errorf("turn 1 live tool calls = %v, want only prompt-era command", ep.Turns[1].ToolCalls)
	}
	if len(ep.Turns[1].ReplayedToolTraces) != 1 || !strings.Contains(ep.Turns[1].ReplayedText, "replayed prior answer") {
		t.Fatalf("turn 1 replay audit missing: text=%q traces=%+v", ep.Turns[1].ReplayedText, ep.Turns[1].ReplayedToolTraces)
	}
	if len(ep.Turns[1].ToolTraces) != 1 || ep.Turns[1].ToolOutputChars != ep.Turns[1].ToolTraces[0].OutputSize {
		t.Fatalf("turn 1 replay contaminated live trace/accounting: traces=%+v replay=%+v chars=%d",
			ep.Turns[1].ToolTraces, ep.Turns[1].ReplayedToolTraces, ep.Turns[1].ToolOutputChars)
	}
	if !strings.Contains(ep.Turns[0].ToolCalls[0], "execute: ghx tree honojs/hono") {
		t.Errorf("tool summary did not resolve command: %v", ep.Turns[0].ToolCalls)
	}
	if len(ep.Turns[0].ToolTraces) != 2 || len(ep.Actions) == 0 || len(ep.Observations) == 0 {
		t.Fatalf("trace/action/observation capture missing: traces=%d actions=%d observations=%d", len(ep.Turns[0].ToolTraces), len(ep.Actions), len(ep.Observations))
	}
	first := ep.Turns[0].ToolTraces[0]
	if first.ID == "" || first.Kind != "execute" || first.RawInput == nil || len(first.StatusTransitions) != 2 {
		t.Fatalf("incomplete tool trace: %+v", first)
	}
	if first.OutputSize == 0 || !strings.Contains(first.OutputExcerpt, "mock output") {
		t.Fatalf("tool output not captured: %+v", first)
	}
	if ep.Identity.AdapterName != "mockagent" || ep.Identity.AdapterVersion != "0.0.1" || ep.Identity.AdapterSubjectModel != "mock-sonnet" {
		t.Fatalf("identity not captured from initialize: %+v", ep.Identity)
	}
	if ep.Identity.SubjectModel != "mock-sonnet-subject" {
		t.Fatalf("subject model not captured from eval env: %+v", ep.Identity)
	}

	r := ep.Rewards
	if r.Correctness != 1.0 {
		t.Errorf("correctness = %v, want 1.0 (scripted report names all expected files+symbols)", r.Correctness)
	}
	if r.Evidence != 1.0 {
		t.Errorf("evidence = %v, want 1.0", r.Evidence)
	}
	if r.Safety != 1.0 {
		t.Errorf("safety = %v, want 1.0", r.Safety)
	}
	if !r.MemoryApplies || r.Memory != 1.0 {
		t.Errorf("memory = %v (applies=%v), want 1.0 (resumed, no repeat reads)", r.Memory, r.MemoryApplies)
	}
	if r.Compression <= 0 {
		t.Errorf("compression = %v, want > 0 (report smaller than full transcript)", r.Compression)
	}
	if ep.Context.MainAgentChars <= 0 || ep.Context.MainAgentChars >= ep.Context.TotalWorkflowChars {
		t.Errorf("context accounting broken: %+v", ep.Context)
	}

	// Artifact roundtrip.
	runDir := t.TempDir()
	path, err := SaveEpisode(runDir, ep)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadEpisode(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Rewards.Overall != ep.Rewards.Overall {
		t.Errorf("artifact roundtrip changed rewards: %v != %v", loaded.Rewards.Overall, ep.Rewards.Overall)
	}

	traceData := loadTraceData(t, runDir)
	traceBytes, err := os.ReadFile(filepath.Join(runDir, traceFileName))
	if err != nil {
		t.Fatal(err)
	}
	if line, _, ok := strings.Cut(string(traceBytes), "\n"); ok {
		t.Logf("sample trace line: %s", boundedString(line, 320))
	}
	spans, resourceAttrs := flattenTraceSpans(traceData)
	if len(spans) < 6 {
		t.Fatalf("span count = %d, want at least episode + 2 turns + 3 tools + reward", len(spans))
	}
	for key, want := range map[string]string{
		"service.name":           "ghx-evals",
		"otel.semconv_version":   otelSemconvVersion,
		"ghx.eval.task_id":       task.ID,
		"ghx.eval.profile":       string(ProfileSidecar),
		"ghx.eval.subject_model": "mock-sonnet-subject",
		"ghx.eval.adapter.name":  "mockagent",
	} {
		if got := resourceAttrs[key]; got != want {
			t.Fatalf("resource attr %s = %q, want %q (attrs=%v)", key, got, want, resourceAttrs)
		}
	}
	episodeSpan := spanByName(t, spans, "eval.episode")
	rewardSpan := spanByName(t, spans, "eval.reward.compute")
	if len(episodeSpan.ParentSpanId) != 0 {
		t.Fatalf("episode span has parent: %x", episodeSpan.ParentSpanId)
	}
	if string(rewardSpan.ParentSpanId) != string(episodeSpan.SpanId) {
		t.Fatalf("reward parent = %x, want episode %x", rewardSpan.ParentSpanId, episodeSpan.SpanId)
	}
	if got := spanAttr(episodeSpan, "ghx.eval.task_id"); got != task.ID {
		t.Fatalf("episode task attr = %q, want %q", got, task.ID)
	}
	turns := spansByNamePrefix(spans, "eval.turn")
	if len(turns) != 2 {
		t.Fatalf("turn spans = %d, want 2", len(turns))
	}
	for _, turn := range turns {
		if string(turn.ParentSpanId) != string(episodeSpan.SpanId) {
			t.Fatalf("turn parent = %x, want episode %x", turn.ParentSpanId, episodeSpan.SpanId)
		}
	}
	tools := spansByNamePrefix(spans, "tool.")
	if len(tools) != 3 {
		t.Fatalf("tool spans = %d, want 3", len(tools))
	}
	turnIDs := map[string]bool{}
	for _, turn := range turns {
		turnIDs[string(turn.SpanId)] = true
	}
	for _, tool := range tools {
		if !turnIDs[string(tool.ParentSpanId)] {
			t.Fatalf("tool %s parent = %x, want one of turn spans", tool.Name, tool.ParentSpanId)
		}
		if spanAttr(tool, "gen_ai.tool.call.id") == "" || spanAttr(tool, "ghx.eval.tool.input") == "" {
			t.Fatalf("tool span missing core attrs: %+v", tool.Attributes)
		}
	}
}

// TestMockDirectEpisode drives a direct (ghx-profile) episode against the
// mock agent: one live process, one session, two prompts, no report block.
func TestMockDirectEpisode(t *testing.T) {
	bin := buildMockAgent(t)
	writeScript(t, []map[string]any{
		{
			"toolCalls": []string{"ghx read honojs/hono src/compose.ts"},
			"text":      "Composition is in src/compose.ts via compose() and dispatch.",
		},
		{
			"toolCalls": []string{"ghx read honojs/hono src/hono-base.ts"},
			"text":      "Errors flow to onError wired in src/hono-base.ts.",
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()}
	task := honoTask(t)

	ep, err := RunEpisode(ctx, cfg, task, ProfileGhx)
	if err != nil {
		t.Fatalf("episode failed: %v", err)
	}
	if len(ep.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(ep.Turns))
	}
	if ep.Turns[0].Text == "" || ep.Turns[1].Text == "" {
		t.Error("turn text not captured")
	}
	if len(ep.Turns[0].ToolCalls) != 1 {
		t.Errorf("turn 0 tool calls = %v, want 1", ep.Turns[0].ToolCalls)
	}
	if len(ep.Actions) != 2 || len(ep.Observations) != 2 {
		t.Fatalf("actions/observations = %d/%d, want 2/2", len(ep.Actions), len(ep.Observations))
	}
	if got := ep.Actions[0].Input; got != "ghx read honojs/hono src/compose.ts" {
		t.Fatalf("action input = %q", got)
	}
	if ep.Turns[0].ToolOutputChars == 0 {
		t.Fatal("tool output chars not captured")
	}
	if ep.Context.MainAgentChars != ep.Context.TotalWorkflowChars {
		t.Errorf("direct profile must count all output as main-agent context: %+v", ep.Context)
	}
	if ep.Rewards.Compression != 0 {
		t.Errorf("compression = %v, want 0 for direct profile", ep.Rewards.Compression)
	}
	if ep.Rewards.Correctness == 0 {
		t.Errorf("correctness = 0; text mentions expected files/symbols so partial credit expected")
	}
}

func TestEvalReportRendersMockRun(t *testing.T) {
	dir := t.TempDir()
	eps := passingEpisodes()
	for i, ep := range eps[:3] {
		ep.ID = "episode-" + string(rune('a'+i))
		ep.Identity = AgentIdentity{AgentCommand: "mockagent", AdapterName: "mockagent", AdapterVersion: "0.0.1", SubjectModel: "mock-sonnet"}
		if _, err := SaveEpisode(dir, ep); err != nil {
			t.Fatal(err)
		}
	}
	if err := SaveRunManifest(dir, RunManifest{
		ExpectedEpisodes: 3,
		Identity:         eps[0].Identity,
	}); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/evalreport", dir)
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("evalreport failed: %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"Episodes: 3 / 3 expected", "Per-profile aggregates", "Gate status", "Compliance and identity notes", "PRELIMINARY"} {
		if !strings.Contains(text, want) {
			t.Fatalf("evalreport output missing %q:\n%s", want, text)
		}
	}
}

// TestMockSidecarEpisodeMaxTurnsWrapUp is the ADR-0027 D1 integration proof:
// a scripted agent kills the first prompt with the adapter's hard max-turns
// error; the runtime LoadSession-resumes the SAME session, sends exactly the
// wrap-up prompt, accepts the wrap-up's sink report, records wrapUpRecovered
// on the turn, and the run counts the soft turn_cap_wrapup anomaly.
func TestMockSidecarEpisodeMaxTurnsWrapUp(t *testing.T) {
	t.Setenv("GHX_REPORT_SINK_EXE", "/usr/local/bin/ghx")
	bin := buildMockAgent(t)
	promptLog := filepath.Join(t.TempDir(), "prompts.log")
	t.Setenv("MOCKAGENT_PROMPT_LOG", promptLog)

	wrapUpReport := map[string]any{
		"answer":        "Middleware composition lives in src/compose.ts (recovered by wrap-up).",
		"verified":      []any{map[string]any{"summary": "compose() builds the dispatch chain", "evidence": "src/compose.ts: dispatch recursion"}},
		"unverified":    []any{map[string]any{"summary": "hono-base wiring not re-confirmed before the turn cap", "evidence": ""}},
		"relevantFiles": []any{map[string]any{"path": "src/compose.ts", "reason": "defines compose()"}},
		"commandsRun":   []any{"ghx tree honojs/hono src --depth 2"},
		"backendsUsed":  []any{"remote"},
	}
	turn2Report := map[string]any{
		"answer":        "Errors propagate through onError wired in src/hono-base.ts.",
		"verified":      []any{map[string]any{"summary": "onError receives dispatch errors", "evidence": "src/hono-base.ts"}},
		"relevantFiles": []any{map[string]any{"path": "src/hono-base.ts", "reason": "error handler composition"}},
		"commandsRun":   []any{"ghx read honojs/hono src/hono-base.ts --grep onError"},
		"backendsUsed":  []any{"remote"},
	}
	writeScript(t, []map[string]any{
		{
			"toolCalls":   []string{"ghx tree honojs/hono src --depth 2"},
			"promptError": "Reached maximum number of turns (24)",
		},
		{
			"text":         "wrapping up with what I have",
			"submitReport": wrapUpReport,
		},
		{
			"toolCalls":    []string{"ghx read honojs/hono src/hono-base.ts --grep onError"},
			"text":         "second turn proceeds normally",
			"submitReport": turn2Report,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()}
	ep, err := RunEpisode(ctx, cfg, honoTask(t), ProfileSidecar)
	if err != nil {
		t.Fatalf("episode must survive the turn cap via wrap-up: %v", err)
	}

	if len(ep.Turns) != 2 {
		t.Fatalf("turns = %d, want 2", len(ep.Turns))
	}
	if !ep.Turns[0].WrapUpRecovered {
		t.Fatal("turn 0 must record wrapUpRecovered")
	}
	if ep.Turns[0].Report == nil || !strings.Contains(ep.Turns[0].Report.Answer, "recovered by wrap-up") {
		t.Fatalf("turn 0 report = %+v, want the wrap-up's sink report", ep.Turns[0].Report)
	}
	if ep.Turns[1].WrapUpRecovered {
		t.Fatal("turn 1 completed normally and must not record a wrap-up")
	}

	// The wrap-up must have resumed the SAME ACP session with the exact prompt.
	logData, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	if !strings.Contains(string(logData), "LOAD mock-sess-1") {
		t.Fatalf("wrap-up did not LoadSession-resume the session:\n%s", logData)
	}
	if !strings.Contains(string(logData), "PROMPT wrap up: call submit_report now with what you have; mark unverified items unverified") {
		t.Fatalf("exact wrap-up prompt not sent:\n%s", logData)
	}

	found := false
	for _, a := range DetectAnomalies(ep) {
		if a.Kind == AnomalyTurnCapWrapUp {
			found = true
			if a.Severity != SeveritySoft {
				t.Fatalf("turn_cap_wrapup severity = %s, want soft", a.Severity)
			}
		}
		if a.Severity == SeverityBreaking {
			t.Fatalf("recovered episode must not carry breaking anomalies: %s", a.String())
		}
	}
	if !found {
		t.Fatal("turn_cap_wrapup anomaly not detected")
	}
}

// TestMockSidecarEpisodeHangTimeout is the ADR-0027 D2/D3 integration proof:
// a scripted agent goes silent mid-prompt; the liveness watchdog cancels the
// turn, the episode records the episode_hang_timeout anomaly, and the failed
// turn still leaves traces.jsonl + logs.jsonl in the session dir (D3).
func TestMockSidecarEpisodeHangTimeout(t *testing.T) {
	t.Setenv("GHX_REPORT_SINK_EXE", "/usr/local/bin/ghx")
	t.Setenv("GHX_SIDECAR_LIVENESS_TIMEOUT", "500ms")
	bin := buildMockAgent(t)
	writeScript(t, []map[string]any{
		{"hangMs": 30000, "text": "never reached"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	sessions := t.TempDir()
	cfg := RunConfig{AgentCmd: bin, SessionsDir: sessions}
	start := time.Now()
	ep, err := RunEpisode(ctx, cfg, honoTask(t), ProfileSidecar)
	if err == nil {
		t.Fatal("a hung turn must fail the episode")
	}
	if elapsed := time.Since(start); elapsed > 30*time.Second {
		t.Fatalf("watchdog did not cancel the hang: episode took %s", elapsed)
	}
	if ep == nil || len(ep.Turns) != 1 {
		t.Fatalf("partial episode with the failed turn expected, got %+v", ep)
	}
	if !strings.Contains(ep.Turns[0].Error, "liveness watchdog timeout") {
		t.Fatalf("turn error = %q, want the liveness watchdog marker", ep.Turns[0].Error)
	}

	found := false
	for _, a := range ep.Anomalies {
		if a.Kind == AnomalyEpisodeHangTimeout && a.Severity == SeverityBreaking {
			found = true
		}
	}
	if !found {
		t.Fatalf("episode_hang_timeout anomaly not detected: %+v", ep.Anomalies)
	}

	// D3: the failed turn's session dir still carries traces + logs.
	sessionDir := filepath.Join(sessions, ep.ID)
	for _, artifact := range []string{"traces.jsonl", "logs.jsonl"} {
		if _, err := os.Stat(filepath.Join(sessionDir, artifact)); err != nil {
			t.Fatalf("failed turn left no %s (D3 violated): %v", artifact, err)
		}
	}
	logData, err := os.ReadFile(filepath.Join(sessionDir, "logs.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), "sidecar.turn.error") {
		t.Fatalf("logs.jsonl missing the turn error record:\n%s", logData)
	}
}

// TestMockSidecarEpisodeSubmitReportSink drives the full production sidecar
// path with the agent completing each turn via the report-sink instead of a
// <ghx-report> text block (ADR-0021 D1/D2). The scripted agent writes an
// accepted report to the sink path the runtime registered on the session; the
// runtime must prefer that sink report over the (deliberately report-less)
// turn text, with no missing-report anomaly and no retry.
func TestMockSidecarEpisodeSubmitReportSink(t *testing.T) {
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "mock-sonnet-subject")
	// Under go test the sink server is disabled unless an explicit binary is
	// named (mirrors live eval runs); the mock adapter never spawns it, it
	// only reads the --out path from the registration.
	t.Setenv("GHX_REPORT_SINK_EXE", "/usr/local/bin/ghx")
	bin := buildMockAgent(t)
	sinkReport0 := map[string]any{
		"answer":        "Middleware composition is implemented in src/compose.ts (submitted via the report sink).",
		"verified":      []any{map[string]any{"summary": "compose() builds the dispatch chain", "evidence": "src/compose.ts"}},
		"relevantFiles": []any{map[string]any{"path": "src/compose.ts", "reason": "defines compose()"}},
		"commandsRun":   []any{"ghx tree honojs/hono src --depth 2"},
		"backendsUsed":  []any{"remote"},
	}
	// Fixtures carry the D4 evidence trio (verified-with-evidence, relevant
	// file, command run) so they stay representative of what the real
	// submit_report tool accepts (ADR-0027 D4).
	sinkReport1 := map[string]any{
		"answer":        "Errors propagate through onError, wired in src/hono-base.ts (report sink).",
		"verified":      []any{map[string]any{"summary": "onError receives dispatch errors", "evidence": "src/hono-base.ts"}},
		"relevantFiles": []any{map[string]any{"path": "src/hono-base.ts", "reason": "error handler composition"}},
		"commandsRun":   []any{"ghx read honojs/hono src/hono-base.ts --grep onError"},
		"backendsUsed":  []any{"remote"},
	}
	writeScript(t, []map[string]any{
		{
			"toolCalls":    []string{"ghx tree honojs/hono src --depth 2"},
			"text":         "Investigated compose; submitting via the report tool. No inline report block here.",
			"submitReport": sinkReport0,
		},
		{
			"replayOnLoad": []string{"ghx tree honojs/hono src --depth 2"},
			"toolCalls":    []string{"ghx read honojs/hono src/hono-base.ts --grep onError"},
			"text":         "Followed up on error handling; submitting via the report tool. No inline block.",
			"submitReport": sinkReport1,
		},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cfg := RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()}
	ep, err := RunEpisode(ctx, cfg, honoTask(t), ProfileSidecar)
	if err != nil {
		t.Fatalf("episode failed: %v", err)
	}

	if ep.Report == nil {
		t.Fatal("no final report — the sink report was not picked up")
	}
	if !strings.Contains(ep.Report.Answer, "report sink") {
		t.Fatalf("final answer = %q, want the sink-submitted report", ep.Report.Answer)
	}
	for i, want := range []string{"src/compose.ts (submitted via the report sink)", "src/hono-base.ts (report sink)"} {
		if ep.Turns[i].Report == nil || !strings.Contains(ep.Turns[i].Report.Answer, want) {
			t.Fatalf("turn %d report = %+v, want sink answer containing %q", i, ep.Turns[i].Report, want)
		}
		if ep.Turns[i].ReportRetried {
			t.Errorf("turn %d should not retry — the sink report was accepted", i)
		}
		if ep.Turns[i].ReportCoerced {
			t.Errorf("turn %d sink report must not be coerced", i)
		}
	}
	for _, a := range DetectAnomalies(ep) {
		if a.Kind == AnomalySidecarReportMissing || a.Kind == AnomalySidecarReportUnparsed {
			t.Errorf("unexpected anomaly on the sink path: %s", a.String())
		}
	}
}
