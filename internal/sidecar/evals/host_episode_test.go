package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// stubContainerRuntime is a docker-free ContainerRuntime for plumbing tests:
// every command "runs" and exits zero. Mock runs are never scored, so the
// grade numbers it produces are never asserted on.
type stubContainerRuntime struct{}

func (stubContainerRuntime) Available(context.Context) error { return nil }
func (stubContainerRuntime) Start(context.Context, hosttask.ContainerSpec) (string, error) {
	return "stub-container", nil
}
func (stubContainerRuntime) Exec(context.Context, string, string) (hosttask.ExecResult, error) {
	return hosttask.ExecResult{ExitCode: 0}, nil
}
func (stubContainerRuntime) Remove(context.Context, string) error { return nil }

// initHostFixtureRepo builds a tiny real git repo in t.TempDir and returns
// its file:// URL and HEAD SHA (the hosttask provision_test pattern — the
// provisioner's RemoteURL field is the injectable clone seam).
func initHostFixtureRepo(t *testing.T) (url, sha string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		out, err := tier2.ExecGit{}.Run(context.Background(), dir, args...)
		if err != nil {
			t.Fatalf("fixture git %v: %v", args, err)
		}
		return out
	}
	run("init", "--quiet", "--initial-branch=main")
	run("config", "user.email", "fixture@test")
	run("config", "user.name", "fixture")
	run("config", "uploadpack.allowAnySHA1InWant", "true")
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "--quiet", "-m", "c1")
	return "file://" + dir, run("rev-parse", "HEAD")
}

func hostFixture(sha string) hosttask.Fixture {
	return hosttask.Fixture{
		ID:             "mock-host-task",
		WorkspaceRepo:  "mock/host-task",
		PinnedSHA:      sha,
		Image:          "mock-image:1",
		FailToPassCmds: []string{"true"},
		PassToPassCmds: []string{"true"},
		ExplorationSubQuestions: []hosttask.SubQuestion{
			{Question: "what does the dependency do", GroundTruth: "it frobs", VerifiedAt: "2026-07-06"},
		},
		MemorizationCanary: hosttask.MemorizationCanary{CheckedAt: "2026-07-06", Result: hosttask.CanaryPass},
	}
}

// TestMockHostTrialArmB drives one full arm-B host-task trial against the
// scripted mock agent: provision from a local git repo → ACP episode with
// the workspace write policy, fs.writeTextFile advertised true, and the
// recon MCP server registered → S2 attribution → compliance detection →
// stubbed grading → artifact roundtrip with recomputable anomalies. It is
// also the scripted proof of the two S2 obligations: (a) tool-call locations
// flow to sidecar.ToolCallTrace.Locations on the eval client path, and (b) execute
// identity resolves from rawInput.command under the live adapter's
// "Terminal" titles (sighting 2026-07-06).
func TestMockHostTrialArmB(t *testing.T) {
	bin := buildMockAgent(t)
	promptLog := filepath.Join(t.TempDir(), "prompts.log")
	t.Setenv("MOCKAGENT_PROMPT_LOG", promptLog)
	outsidePath := filepath.Join(t.TempDir(), "outside.txt")

	writeScript(t, []map[string]any{{
		"richToolCalls": []map[string]any{
			// Compliant exploration: the recon MCP tool (classifier R1).
			{"id": "recon-1", "title": "mcp__ghx-recon__recon", "kind": "other",
				"output": "recon evidence report about the dependency"},
			// Hand exploration: adapter-shaped execute — title "Terminal",
			// command in rawInput (the live claude-agent-acp shape). Must be
			// classified exploration (R4) from the command and flagged.
			{"id": "gh-1", "title": "Terminal", "kind": "execute",
				"command": "gh api repos/upstream/dep/releases", "output": "release data"},
			// Engineering: local test run, same adapter shape (R5).
			{"id": "test-1", "title": "Terminal", "kind": "execute",
				"command": "go test ./...", "output": "ok"},
			// Workspace read with locations (R6 + S2 obligation (a)).
			{"id": "read-1", "title": "Read hello.txt", "kind": "read",
				"locations": []string{"WORKSPACE/hello.txt"}, "output": "hello world"},
		},
		"writeFiles": []map[string]any{
			{"path": "WORKSPACE/fix.txt", "content": "patched"},
			{"path": outsidePath, "content": "escape attempt"},
		},
		"text": "Implemented the fix in fix.txt.",
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url, sha := initHostFixtureRepo(t)
	prov := hosttask.NewProvisioner(t.TempDir())
	prov.RemoteURL = url

	ep, err := RunHostTrial(ctx, HostTrial{
		Run:         RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()},
		Fixture:     hostFixture(sha),
		Trial:       1,
		Arm:         HostArmSidecar,
		Objective:   "Fix the frobnication bug described in issue #42.",
		Provisioner: prov,
		Grader:      &hosttask.Grader{Runtime: stubContainerRuntime{}},
		ReconExe:    "/usr/local/bin/ghx",
	})
	if err != nil {
		t.Fatalf("host trial failed: %v", err)
	}
	if ep.Profile != ProfileHostSidecar || ep.HostTask == nil || ep.HostTask.Arm != HostArmSidecar {
		t.Fatalf("host episode identity wrong: profile=%s hostTask=%+v", ep.Profile, ep.HostTask)
	}
	if ep.HostTask.WorkspaceSHA != sha {
		t.Fatalf("workspace SHA = %s, want pinned %s", ep.HostTask.WorkspaceSHA, sha)
	}
	if len(ep.Turns) != 1 {
		t.Fatalf("turns = %d, want 1", len(ep.Turns))
	}

	// Workspace write allowed; outside write denied and recorded — never on disk.
	data, err := os.ReadFile(filepath.Join(ep.HostTask.WorkspaceDir, "fix.txt"))
	if err != nil || string(data) != "patched" {
		t.Fatalf("workspace write missing: %q, %v", data, err)
	}
	if _, err := os.Stat(outsidePath); !os.IsNotExist(err) {
		t.Fatalf("outside write must never land: %v", err)
	}
	foundViolation := false
	for _, v := range ep.Violations {
		if strings.Contains(v, "outside workspace") && strings.Contains(v, outsidePath) {
			foundViolation = true
		}
	}
	if !foundViolation {
		t.Fatalf("outside write not recorded as a violation: %v", ep.Violations)
	}

	logData, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	// Host episodes advertise fs.writeTextFile TRUE (S2 risk 4 — host path only).
	if !strings.Contains(log, "INIT_CAPS readTextFile=false writeTextFile=true") {
		t.Fatalf("host episode must advertise writeTextFile=true:\n%s", log)
	}
	// Arm B registers exactly the recon MCP stdio server on the session.
	if !strings.Contains(log, "MCP_SERVER ghx-recon /usr/local/bin/ghx serve --recon") {
		t.Fatalf("recon MCP server registration missing:\n%s", log)
	}
	// The prompt carries the arm-B recon contract, the workspace path, and the issue.
	for _, want := range []string{"Reconnaissance contract:", ep.HostTask.WorkspaceDir, "issue #42"} {
		if !strings.Contains(log, want) {
			t.Fatalf("host prompt missing %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "using your shell and the gh CLI") {
		t.Fatalf("arm-B prompt must not carry the control-arm exploration block:\n%s", log)
	}

	// S2 obligation (a): locations flow to sidecar.ToolCallTrace.Locations.
	var readTrace *sidecar.ToolCallTrace
	for i := range ep.Turns[0].ToolTraces {
		if ep.Turns[0].ToolTraces[i].ID == "read-1" {
			readTrace = &ep.Turns[0].ToolTraces[i]
		}
	}
	if readTrace == nil || len(readTrace.Locations) != 1 ||
		readTrace.Locations[0] != filepath.Join(ep.HostTask.WorkspaceDir, "hello.txt") {
		t.Fatalf("tool-call locations did not flow to the trace: %+v", readTrace)
	}

	// S2 obligation (b) + attribution: adapter-shaped execute calls classify
	// from rawInput.command, recon call is exploration, workspace read is
	// engineering, and the unclassified bucket stays empty.
	wantClass := map[string]hosttask.ToolCallClass{
		"recon-1": hosttask.ClassExploration,
		"gh-1":    hosttask.ClassExploration,
		"test-1":  hosttask.ClassEngineering,
		"read-1":  hosttask.ClassEngineering,
	}
	gotClass := map[string]hosttask.ToolCallClass{}
	for _, row := range ep.HostTask.Attribution.PerToolCall {
		gotClass[row.ID] = row.Class
	}
	for id, want := range wantClass {
		if gotClass[id] != want {
			t.Fatalf("attribution class for %s = %q, want %q (table=%+v)", id, gotClass[id], want, ep.HostTask.Attribution)
		}
	}
	if len(ep.HostTask.Attribution.Unclassified) != 0 {
		t.Fatalf("unexpected unclassified rows: %v", ep.HostTask.Attribution.Unclassified)
	}
	if ep.HostTask.Attribution.ExplorationChars == 0 || ep.HostTask.Attribution.EngineeringChars == 0 {
		t.Fatalf("attribution totals empty: %+v", ep.HostTask.Attribution)
	}

	// Compliance detector: exactly one hand-exploration flag (the gh call),
	// never the recon tool; no BLOCKED rows on the healthy path.
	if got := countKind(ep.Anomalies, AnomalyHostExternalExploration); got != 1 {
		t.Fatalf("host_external_exploration count = %d, want 1 (anomalies=%v)", got, ep.Anomalies)
	}
	for _, a := range ep.Anomalies {
		if a.Kind == AnomalyHostExternalExploration && !strings.Contains(a.Detail, "kind=execute") {
			t.Fatalf("compliance anomaly should name the offending call: %q", a.Detail)
		}
		if a.Kind == AnomalyReconToolUnavailable || a.Kind == AnomalyHostRateLimited {
			t.Fatalf("unexpected BLOCKED anomaly on healthy mock run: %s", a.String())
		}
	}

	// Grading plumbing ran (numbers deliberately unasserted: mock runs are
	// never scored).
	if ep.HostTask.GradeError != "" || ep.HostTask.Grade == nil {
		t.Fatalf("grading plumbing failed: grade=%+v err=%q", ep.HostTask.Grade, ep.HostTask.GradeError)
	}

	// Host episodes carry no recon rewards.
	if ep.Rewards.Overall != 0 {
		t.Fatalf("host episode must not carry recon rewards: %+v", ep.Rewards)
	}

	// Artifact roundtrip: anomalies recompute identically from the saved
	// artifact (offline detection contract).
	runDir := t.TempDir()
	path, err := SaveEpisode(runDir, ep)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadEpisode(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.HostTask == nil || loaded.HostTask.Arm != HostArmSidecar {
		t.Fatalf("host record lost in roundtrip: %+v", loaded.HostTask)
	}
	if got := countKind(DetectAnomalies(loaded), AnomalyHostExternalExploration); got != 1 {
		t.Fatalf("offline re-detection from artifact = %d, want 1", got)
	}

	// Frozen-identity manifest extension: the three S3 hashes are recorded
	// and the recon schema hash is recomputable from the shared definition.
	hashes, err := HostTaskIdentityHashes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RecordManifestHostTaskIdentityHashes(runDir, hashes); err != nil {
		t.Fatal(err)
	}
	m, err := LoadRunManifest(runDir)
	if err != nil || m == nil {
		t.Fatalf("manifest missing: %v", err)
	}
	byName := map[string]BaselineReuseHashRecord{}
	for _, rec := range m.HostTaskIdentityHashes {
		byName[rec.Name] = rec
	}
	for _, name := range []string{"hostPromptContract", "armBPromptContract", "reconToolSchema"} {
		rec, ok := byName[name]
		if !ok || rec.Algorithm != "sha256" || len(rec.Value) != 64 {
			t.Fatalf("manifest hash %s missing or malformed: %+v", name, m.HostTaskIdentityHashes)
		}
	}
	if byName["hostPromptContract"].Value == byName["armBPromptContract"].Value {
		t.Fatal("control and arm-B prompt contracts must hash differently")
	}
	schema, err := json.Marshal(sidecar.ReconMCPTool())
	if err != nil {
		t.Fatal(err)
	}
	if byName["reconToolSchema"].Value != sha256Hex(schema) {
		t.Fatal("reconToolSchema hash does not recompute from sidecar.ReconMCPTool")
	}
	// Additive: the recon baseline-reuse inventory is untouched.
	if len(m.IdentityHashes) != 0 {
		t.Fatalf("host hashes must not leak into identityHashes: %+v", m.IdentityHashes)
	}
}

// TestMockHostTrialControlArm proves the control arm wiring: no MCP servers,
// the native-exploration prompt block, and no compliance anomaly for hand
// exploration (that is the control arm's job).
func TestMockHostTrialControlArm(t *testing.T) {
	bin := buildMockAgent(t)
	promptLog := filepath.Join(t.TempDir(), "prompts.log")
	t.Setenv("MOCKAGENT_PROMPT_LOG", promptLog)

	writeScript(t, []map[string]any{{
		"richToolCalls": []map[string]any{
			{"id": "gh-1", "title": "Terminal", "kind": "execute",
				"command": "gh api repos/upstream/dep", "output": "dep metadata"},
		},
		"text": "Fixed after checking the dependency.",
	}})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	url, sha := initHostFixtureRepo(t)
	prov := hosttask.NewProvisioner(t.TempDir())
	prov.RemoteURL = url

	ep, err := RunHostTrial(ctx, HostTrial{
		Run:         RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()},
		Fixture:     hostFixture(sha),
		Trial:       1,
		Arm:         HostArmControl,
		Objective:   "Fix the frobnication bug described in issue #42.",
		Provisioner: prov,
		Grader:      &hosttask.Grader{Runtime: stubContainerRuntime{}},
	})
	if err != nil {
		t.Fatalf("host trial failed: %v", err)
	}
	if ep.Profile != ProfileHostControl {
		t.Fatalf("profile = %s, want %s", ep.Profile, ProfileHostControl)
	}

	logData, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}
	log := string(logData)
	if strings.Contains(log, "MCP_SERVER") {
		t.Fatalf("control arm must register no MCP servers:\n%s", log)
	}
	if !strings.Contains(log, "using your shell and the gh CLI") || strings.Contains(log, "Reconnaissance contract:") {
		t.Fatalf("control prompt wrong:\n%s", log)
	}
	// Native exploration is attributed but never flagged on the control arm.
	if got := countKind(ep.Anomalies, AnomalyHostExternalExploration); got != 0 {
		t.Fatalf("control arm fired host_external_exploration %d times", got)
	}
	if ep.HostTask.Attribution.ExplorationChars == 0 {
		t.Fatalf("control exploration not attributed: %+v", ep.HostTask.Attribution)
	}
}

// TestRunHostTrialValidation pins the trial preconditions.
func TestRunHostTrialValidation(t *testing.T) {
	base := HostTrial{
		Fixture:     hostFixture(strings.Repeat("a", 40)),
		Arm:         HostArmSidecar,
		Objective:   "fix it",
		Provisioner: hosttask.NewProvisioner(t.TempDir()),
		Grader:      &hosttask.Grader{Runtime: stubContainerRuntime{}},
		ReconExe:    "/usr/local/bin/ghx",
	}
	cases := []struct {
		name   string
		mutate func(*HostTrial)
		want   string
	}{
		{"missing recon exe on arm B", func(tr *HostTrial) { tr.ReconExe = "" }, "ReconExe"},
		{"missing objective", func(tr *HostTrial) { tr.Objective = " " }, "objective"},
		{"missing provisioner", func(tr *HostTrial) { tr.Provisioner = nil }, "Provisioner"},
		{"missing grader", func(tr *HostTrial) { tr.Grader = nil }, "Grader"},
		{"unknown arm", func(tr *HostTrial) { tr.Arm = "both" }, "unknown host arm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trial := base
			tc.mutate(&trial)
			_, err := RunHostTrial(context.Background(), trial)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want mention of %q", err, tc.want)
			}
		})
	}
}

// TestHostPromptTemplates pins the frozen prompt contracts' substitution
// behavior: placeholders resolve, templates stay placeholder-clean.
func TestHostPromptTemplates(t *testing.T) {
	p := HostPrompt(HostArmSidecar, "/ws/dir", "the issue text")
	for _, want := range []string{"/ws/dir", "the issue text", "Reconnaissance contract:", sidecar.ReconToolName, HostReconServerName} {
		if !strings.Contains(p, want) {
			t.Fatalf("arm-B prompt missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(p, "{workspace}") || strings.Contains(p, "{objective}") {
		t.Fatalf("unsubstituted placeholder:\n%s", p)
	}
	c := HostPrompt(HostArmControl, "/ws/dir", "the issue text")
	if !strings.Contains(c, "using your shell and the gh CLI") || strings.Contains(c, "Reconnaissance contract:") {
		t.Fatalf("control prompt wrong:\n%s", c)
	}
	// Identity hashes are deterministic across calls.
	h1, err := HostTaskIdentityHashes()
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HostTaskIdentityHashes()
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%+v", h1) != fmt.Sprintf("%+v", h2) {
		t.Fatal("identity hashes not deterministic")
	}
}
