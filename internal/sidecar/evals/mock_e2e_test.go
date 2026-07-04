package evals

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
			"toolCalls": []string{"ghx read honojs/hono src/hono-base.ts --grep onError"},
			"text":      turn1Report,
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
	path, err := SaveEpisode(t.TempDir(), ep)
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
