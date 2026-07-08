package evals

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
)

// Host-task arm-B wiring (ADR-0032.1 D5.3). One host-task trial is: provision
// a pinned workspace (S1), drive the host agent over ACP with the S2
// workspace write policy and — for arm B — the ghx recon MCP tool, attribute
// its tool calls (S2 classifier), then grade the workspace outcome (S1
// grader). Recon episodes, profiles, gates, and the production sidecar are
// untouched: host episodes run on their own profile labels and their own
// RunHostTrial entry point.

// HostArm identifies which ADR-0032.1 D1 arm a host-task trial runs under.
type HostArm string

const (
	// HostArmControl is arm A: the host explores the outside world natively
	// (shell, gh; web off).
	HostArmControl HostArm = "control"
	// HostArmSidecar is arm B: the same host with exactly one recon MCP tool
	// and the prompt contract discouraging external hand-exploration
	// (discouraged and flagged, never blocked).
	HostArmSidecar HostArm = "sidecar"
)

// Host-task episode profile labels. Additive: the three recon profiles in
// profiles.go are untouched, and host episodes never enter recon gate
// aggregation (which selects the recon profile constants explicitly).
const (
	// ProfileHostControl labels arm-A host-task episodes.
	ProfileHostControl Profile = "host-control"
	// ProfileHostSidecar labels arm-B host-task episodes.
	ProfileHostSidecar Profile = "host-sidecar"
)

// profile maps an arm to its episode profile label.
func (a HostArm) profile() (Profile, error) {
	switch a {
	case HostArmControl:
		return ProfileHostControl, nil
	case HostArmSidecar:
		return ProfileHostSidecar, nil
	}
	return "", fmt.Errorf("evals: unknown host arm %q (want %q or %q)", a, HostArmControl, HostArmSidecar)
}

// HostReconServerName is the ACP McpServer name for the arm-B recon server.
// Like ReportSinkServerName it becomes the middle segment of the SDK tool
// identifier mcp__<server>__<tool>, so it is part of the frozen recon tool
// identity.
const HostReconServerName = "ghx-recon"

// hostReconToolID is the fully-qualified SDK/adapter tool name for the recon
// MCP tool, following the mcp__<server>__<tool> convention documented on
// sidecar.SubmitReportToolID.
const hostReconToolID = "mcp__" + HostReconServerName + "__" + sidecar.ReconToolName

// HostReconToolTitles are the exact tool-call titles recognized as the recon
// MCP tool by the S2 classifier (rule R1) and the compliance detector: the
// fully-qualified SDK identifier and the bare tool name. Exact-match tool
// identity, frozen with the measurement stack and snapshotted onto every
// arm-B episode (HostTaskRecord.ReconToolTitles) so detection is
// recomputable offline. A live-adapter sighting of the actually-emitted
// title is still owed before the first registered run (ADR-0032.1 S3 note).
func HostReconToolTitles() []string {
	return []string{hostReconToolID, sidecar.ReconToolName}
}

// hostReconMcpServers returns the arm-B session's MCP server list: one stdio
// server spawning `<reconExe> serve --recon`, which exposes exactly the
// recon tool (sidecar.ReconMCPTool). Constructed per session following the
// reportSinkMcpServers pattern (internal/sidecar/acp.go).
func hostReconMcpServers(reconExe string) []acp.McpServer {
	return []acp.McpServer{{
		Stdio: &acp.McpServerStdio{
			Name:    HostReconServerName,
			Command: reconExe,
			Args:    []string{"serve", "--recon"},
			Env:     []acp.EnvVariable{},
		},
	}}
}

// HostTaskRecord is the host-task extension persisted on the episode
// artifact. It carries everything the offline host-task detectors need —
// arm, frozen recon tool titles, and the attribution table — because the
// workspace (and with it the path scope) is gone by re-analysis time.
type HostTaskRecord struct {
	// FixtureID is the graded fixture.
	FixtureID string `json:"fixtureId"`
	// Arm is the ADR-0032.1 D1 arm the trial ran under.
	Arm HostArm `json:"arm"`
	// Trial is the trial index within the run.
	Trial int `json:"trial"`
	// WorkspaceDir is the provisioned per-trial workspace (evidence pointer;
	// the directory itself is owned by the caller's workspaces root).
	WorkspaceDir string `json:"workspaceDir"`
	// WorkspaceSHA is the HEAD-verified pinned commit the trial started from.
	WorkspaceSHA string `json:"workspaceSha"`
	// ReconToolTitles snapshots the exact recon tool titles used for
	// classification and compliance detection. Empty on the control arm.
	ReconToolTitles []string `json:"reconToolTitles,omitempty"`
	// Attribution is the S2 exploration/engineering attribution table over
	// the episode's live tool calls, computed against the trial workspace
	// scope at run time (the scope cannot be reconstructed offline).
	Attribution hosttask.AttributionTable `json:"attribution"`
	// Grade is the S1 deterministic outcome grade; nil when grading failed.
	Grade *hosttask.Result `json:"grade,omitempty"`
	// GradeError records a grading infrastructure failure (e.g. docker
	// unavailable) verbatim. The episode artifact survives either way.
	GradeError string `json:"gradeError,omitempty"`
}

// HostTrial configures one host-task trial: one fixture × one arm × one
// trial index, run end-to-end by RunHostTrial.
type HostTrial struct {
	// Run is the eval run configuration (host agent command, sessions dir).
	Run RunConfig
	// Fixture is the S1 host task.
	Fixture hosttask.Fixture
	// Trial is the trial index (auditability; per-trial isolation comes from
	// the provisioner, not this number).
	Trial int
	// Arm selects control (native exploration) or sidecar (recon MCP tool).
	Arm HostArm
	// Objective is the issue text the host is asked to fix. It rides on the
	// trial rather than the S1 fixture schema; S4 (corpus authoring) decides
	// its committed home.
	Objective string
	// Provisioner materializes the per-trial workspace (S1). Tests point its
	// RemoteURL at a local file:// fixture repo.
	Provisioner *hosttask.Provisioner
	// Grader grades the workspace after the episode (S1). Tests inject a
	// stub ContainerRuntime so no docker is required.
	Grader *hosttask.Grader
	// ReconExe is the ghx binary that serves the arm-B recon MCP server via
	// `serve --recon`. Required for arm B, unused for control. Explicit for
	// the same reason as GHX_REPORT_SINK_EXE: under `go test` os.Executable
	// is the test binary and cannot serve MCP.
	ReconExe string
}

// validate rejects trials that cannot run end-to-end.
func (t HostTrial) validate() error {
	if t.Provisioner == nil {
		return fmt.Errorf("evals: host trial needs a Provisioner (ADR-0032.1 S1)")
	}
	if t.Grader == nil {
		return fmt.Errorf("evals: host trial needs a Grader (ADR-0032.1 S1)")
	}
	if strings.TrimSpace(t.Objective) == "" {
		return fmt.Errorf("evals: host trial objective (issue text) is required")
	}
	if t.Arm == HostArmSidecar && strings.TrimSpace(t.ReconExe) == "" {
		return fmt.Errorf("evals: arm B needs ReconExe — the ghx binary serving `serve --recon`")
	}
	return nil
}

// startHostEpisodeLiveness mirrors startEpisodeLiveness for host-task trials
// (ADR-0027 D2 heartbeat).
func startHostEpisodeLiveness(fixtureID string, profile Profile) (stop func()) {
	done := make(chan struct{})
	start := time.Now()
	go func() {
		ticker := time.NewTicker(episodeLivenessInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(os.Stderr, "host-task episode live fixture=%s profile=%s elapsed=%s (still running)\n",
					fixtureID, profile, time.Since(start).Round(time.Second))
			}
		}
	}()
	return func() { close(done) }
}

// RunHostTrial executes one host-task trial end-to-end (ADR-0032.1 D5.3):
//
//  1. provision the pinned per-trial workspace (S1 provisioner);
//  2. spawn the host agent over ACP via the shared live-agent machinery with
//     the S2 WorkspaceWritePolicy installed, fs.writeTextFile advertised
//     TRUE (host episodes only — recon episodes keep false), and, for arm B,
//     the ghx recon MCP server registered on the session;
//  3. attribute the episode's live tool calls with the S2 classifier against
//     the trial workspace scope, persisting the table on the episode;
//  4. grade the workspace outcome (S1 grader).
//
// A mid-episode turn failure ends the episode early; the partial episode is
// still attributed, graded, and returned alongside the error so failed
// trials produce artifacts (the RunEpisode contract). Host episodes are
// never scored with recon rewards — Rewards stays zero; the outcome grade
// lives in HostTask.Grade. The workspace directory is left in place under
// the provisioner root so grades stay auditable; the caller owns cleanup.
func RunHostTrial(ctx context.Context, trial HostTrial) (*Episode, error) {
	if err := trial.validate(); err != nil {
		return nil, err
	}
	profile, err := trial.Arm.profile()
	if err != nil {
		return nil, err
	}

	ws, err := trial.Provisioner.Provision(ctx, trial.Fixture, trial.Trial)
	if err != nil {
		return nil, err
	}
	policy, err := NewWorkspaceWritePolicy(ws.Dir)
	if err != nil {
		return nil, err
	}
	scope, err := hosttask.NewWorkspaceScope(ws.Dir)
	if err != nil {
		return nil, err
	}

	stopLiveness := startHostEpisodeLiveness(trial.Fixture.ID, profile)
	defer stopLiveness()

	ep := &Episode{
		ID:        fmt.Sprintf("%s_%s_trial%03d_%d", trial.Fixture.ID, profile, trial.Trial, time.Now().UnixMilli()),
		TaskID:    trial.Fixture.ID,
		Repo:      trial.Fixture.WorkspaceRepo,
		Profile:   profile,
		StartedAt: time.Now().UTC(),
		Identity:  agentIdentity(trial.Run, nil),
		HostTask: &HostTaskRecord{
			FixtureID:    trial.Fixture.ID,
			Arm:          trial.Arm,
			Trial:        trial.Trial,
			WorkspaceDir: ws.Dir,
			WorkspaceSHA: ws.SHA,
		},
	}

	mcpServers := []acp.McpServer{}
	if trial.Arm == HostArmSidecar {
		ep.HostTask.ReconToolTitles = HostReconToolTitles()
		mcpServers = hostReconMcpServers(trial.ReconExe)
	}

	// The workspace IS the session cwd: the host works the checkout directly.
	rt := episodeRuntime{Cwd: ws.Dir, Env: os.Environ(), Cleanup: func() {}}
	runErr := runLiveAgentEpisode(ctx, trial.Run, ep, rt, liveAgentEpisodeSpec{
		client:          &evalClient{writes: policy},
		fsWriteTextFile: true,
		mcpServers:      mcpServers,
		questions:       []string{trial.Objective},
		prompt:          func(_ int, q string) string { return HostPrompt(trial.Arm, ws.Dir, q) },
		turnLabel:       "host",
	})

	// Attribution runs on the partial episode too — a failed trial's context
	// economics are still evidence (never a score).
	classifier := hosttask.Classifier{
		Scope:               scope,
		ReconToolNames:      ep.HostTask.ReconToolTitles,
		ExplorationCommands: hosttask.DefaultExplorationCommands(),
	}
	ep.HostTask.Attribution = classifier.Attribute(hostLiveTraces(ep))

	// Grade whatever the host left in the tree; grading failures are
	// recorded, never silently dropped.
	if grade, gerr := trial.Grader.Grade(ctx, trial.Fixture, ws.Dir); gerr != nil {
		ep.HostTask.GradeError = gerr.Error()
	} else {
		ep.HostTask.Grade = &grade
	}

	ep.EndedAt = time.Now().UTC()
	finalizeContext(ep)
	ep.Anomalies = DetectAnomalies(ep)
	fmt.Fprintf(os.Stderr, "host-task episode complete fixture=%s arm=%s turns=%d violations=%d deadWeight=%.3f duration=%s\n",
		trial.Fixture.ID, trial.Arm, len(ep.Turns), len(ep.Violations),
		ep.HostTask.Attribution.DeadWeightFraction(), ep.EndedAt.Sub(ep.StartedAt).Round(time.Millisecond))
	return ep, runErr
}

// hostLiveTraces flattens the episode's live tool traces (replayed traces are
// excluded — single-turn host episodes never replay, and the exclusion rule
// matches recon accounting, ADR-0016.5) into the sidecar trace shape the S2
// classifier consumes.
func hostLiveTraces(ep *Episode) []sidecar.ToolCallTrace {
	var out []sidecar.ToolCallTrace
	for _, turn := range ep.Turns {
		// ToolTraces are already sidecar.ToolCallTrace (ADR-0032.2 D1), so this
		// is a plain live-only flatten — no field-by-field copy to keep in sync.
		out = append(out, turn.ToolTraces...)
	}
	return out
}

// hostPromptBase is the frozen shared host prompt template (both arms).
// {workspace} and {objective} are substituted per trial; identity hashes
// cover the templates verbatim (placeholders included), so per-trial paths
// never change measurement identity.
const hostPromptBase = `You are a software engineer fixing a real issue in the repository checked out at {workspace}.
Work directly in this checkout: explore the code, implement the fix, and run whatever local commands you need.
Write files only inside the checkout — writes anywhere else are denied and recorded.
Leave the finished fix in the working tree. Do not commit and do not push.`

// hostPromptControlExploration is the frozen arm-A exploration block
// (ADR-0032.1 D1: native exploration — shell + gh, web off).
const hostPromptControlExploration = `You may research anything outside this checkout (dependency behavior, upstream repositories, documentation) using your shell and the gh CLI. Web browsing is off.`

// hostPromptSidecarContract is the frozen arm-B recon skill / prompt
// contract (ADR-0032.1 D1): external exploration is discouraged and flagged,
// never blocked — the compliance detector, not the prompt, is the
// measurement.
const hostPromptSidecarContract = `Reconnaissance contract: for ANY exploration outside this checkout — dependency behavior, upstream source, how other repositories use an API — call the ` + "`" + sidecar.ReconToolName + "`" + ` MCP tool (server "` + HostReconServerName + `") with your whole English question and build on its evidence report; follow-up questions route automatically. Do not explore externally by hand: no gh, no curl or wget, no cloning or fetching other repositories, no web. External hand-exploration is recorded and flagged.`

// hostPromptTemplate returns the full frozen prompt template for one arm,
// with {workspace} and {objective} placeholders intact. This exact string is
// what HostTaskIdentityHashes hashes.
func hostPromptTemplate(arm HostArm) string {
	exploration := hostPromptControlExploration
	if arm == HostArmSidecar {
		exploration = hostPromptSidecarContract
	}
	return hostPromptBase + "\n\n" + exploration + "\n\nIssue:\n{objective}"
}

// HostPrompt renders the host episode prompt for one arm, workspace, and
// objective (issue text).
func HostPrompt(arm HostArm, workspaceDir, objective string) string {
	s := strings.ReplaceAll(hostPromptTemplate(arm), "{workspace}", workspaceDir)
	return strings.ReplaceAll(s, "{objective}", objective)
}
