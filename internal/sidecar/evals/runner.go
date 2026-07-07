package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// RunConfig configures one episode run.
type RunConfig struct {
	// AgentCmd is the ACP agent binary spawned for every profile.
	AgentCmd string
	// SessionsDir is the sidecar session store for this run. Use a
	// per-run temporary directory so episodes never touch ~/.ghx.
	SessionsDir string
}

// episodeLivenessInterval paces the per-episode liveness heartbeat printed by
// RunEpisode (ADR-0027 D2): a human watching a run sees an elapsed line keep
// ticking for a stalled episode instead of silence. Package variable so tests
// can shorten it.
var episodeLivenessInterval = time.Minute

// startEpisodeLiveness prints a liveness line for one running episode every
// episodeLivenessInterval until the returned stop func is called.
func startEpisodeLiveness(task Task, profile Profile) (stop func()) {
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
				fmt.Fprintf(os.Stderr, "eval episode live task=%s profile=%s elapsed=%s (still running)\n",
					task.ID, profile, time.Since(start).Round(time.Second))
			}
		}
	}()
	return func() { close(done) }
}

func startDiscoveryEpisodeLiveness(task DiscoveryTask, profile Profile) (stop func()) {
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
				fmt.Fprintf(os.Stderr, "discovery eval episode live task=%s profile=%s elapsed=%s (still running)\n",
					task.ID, profile, time.Since(start).Round(time.Second))
			}
		}
	}()
	return func() { close(done) }
}

// ProbeAgentIdentity starts the configured ACP adapter, performs initialize,
// and returns the same run identity fields recorded on episodes. Baseline reuse
// needs this before deciding whether direct profiles may be skipped
// (ADR-0025.1 D1).
func ProbeAgentIdentity(ctx context.Context, cfg RunConfig) (AgentIdentity, error) {
	agentBin, agentArgs := sidecar.SplitAgentCmd(cfg.AgentCmd)
	cmd := exec.CommandContext(ctx, agentBin, agentArgs...)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return AgentIdentity{}, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return AgentIdentity{}, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return AgentIdentity{}, fmt.Errorf("start %q: %w", cfg.AgentCmd, err)
	}
	defer sidecar.ShutdownAgent(cmd, stdin)

	client := &evalClient{}
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	})
	if err != nil {
		return AgentIdentity{}, fmt.Errorf("acp initialize: %w", err)
	}
	id := agentIdentity(cfg, nil)
	if initResp.AgentInfo != nil {
		id.AdapterName = initResp.AgentInfo.Name
		id.AdapterVersion = initResp.AgentInfo.Version
		id.AdapterSubjectModel = modelFromMeta(initResp.AgentInfo.Meta)
	}
	return id, nil
}

// RunEpisode executes one task under one profile and returns the scored
// episode. A mid-episode turn failure ends the episode early; the partial
// episode (with the turn error recorded) is returned alongside the error so
// failed runs still produce artifacts (ADR-0016 implementation note 4).
func RunEpisode(ctx context.Context, cfg RunConfig, task Task, profile Profile) (*Episode, error) {
	rt, rtErr := prepareEpisodeRuntime(cfg, profile)
	if rtErr != nil {
		return nil, rtErr
	}
	defer rt.Cleanup()

	stopLiveness := startEpisodeLiveness(task, profile)
	defer stopLiveness()

	ep := &Episode{
		ID:        fmt.Sprintf("%s_%s_%d", task.ID, profile, time.Now().UnixMilli()),
		TaskID:    task.ID,
		Repo:      task.Repo,
		Profile:   profile,
		Checks:    task.Checks,
		StartedAt: time.Now().UTC(),
		Identity:  agentIdentity(cfg, nil),
	}

	var err error
	if profile == ProfileSidecar {
		err = runSidecarEpisode(ctx, cfg, task, ep, rt)
	} else {
		err = runDirectEpisode(ctx, cfg, task, profile, ep, rt)
	}

	ep.EndedAt = time.Now().UTC()
	finalizeContext(ep)
	ep.Rewards = ComputeRewards(task, ep)
	ep.Anomalies = DetectAnomalies(ep)
	logEpisodeProgress(task, ep)
	return ep, err
}

// RunDiscoveryEpisode executes one ADR-0019.2 discovery task under one
// profile and returns the episode scored with discovery rewards. It keeps
// repo-scoped RewardBreakdown untouched except for safety/context fields
// needed by shared artifact consumers.
func RunDiscoveryEpisode(ctx context.Context, cfg RunConfig, task DiscoveryTask, profile Profile) (*Episode, error) {
	rt, rtErr := prepareEpisodeRuntime(cfg, profile)
	if rtErr != nil {
		return nil, rtErr
	}
	defer rt.Cleanup()

	stopLiveness := startDiscoveryEpisodeLiveness(task, profile)
	defer stopLiveness()

	ep := &Episode{
		ID:              fmt.Sprintf("%s_%s_%d", task.ID, profile, time.Now().UnixMilli()),
		TaskID:          task.ID,
		Profile:         profile,
		DiscoveryChecks: &task.Checks,
		StartedAt:       time.Now().UTC(),
		Identity:        agentIdentity(cfg, nil),
	}

	var err error
	if profile == ProfileSidecar {
		err = runDiscoverySidecarEpisode(ctx, cfg, task, ep, rt)
	} else {
		err = runDirectDiscoveryEpisode(ctx, cfg, task, profile, ep, rt)
	}

	ep.EndedAt = time.Now().UTC()
	finalizeContext(ep)
	rewards := ComputeDiscoveryRewards(task, ep)
	ep.DiscoveryRewards = &rewards
	ep.Rewards = RewardBreakdown{
		Evidence:    rewards.Evidence,
		Compression: rewards.Compression,
		Safety:      rewards.Safety,
		Overall:     mean([]float64{rewards.VerifiedRecall, rewards.VerifiedPrecision, rewards.Evidence, rewards.InferenceHonesty, rewards.Compression, rewards.Safety}),
	}
	ep.Anomalies = DetectAnomalies(ep)
	logDiscoveryEpisodeProgress(task, ep)
	return ep, err
}

func runDiscoverySidecarEpisode(ctx context.Context, cfg RunConfig, task DiscoveryTask, ep *Episode, rt episodeRuntime) error {
	scfg := sidecar.Config{
		AgentCmd:    cfg.AgentCmd,
		SessionsDir: cfg.SessionsDir,
		Cwd:         rt.Cwd,
		Env:         rt.Env,
		Model:       resolveSubjectModel(cfg.AgentCmd),
		EvalMode:    true,
	}
	session := ep.ID

	for i, q := range task.Turns {
		rec := TurnRecord{Turn: i, Question: q}
		start := time.Now()
		if i > 0 {
			meta, metaErr := sidecar.ReadMeta(cfg.SessionsDir, session)
			rec.Resumed = metaErr == nil && meta != nil && meta.ACPSessionID != ""
		}
		report, turn, err := sidecar.Ask(ctx, scfg, sidecar.AskRequest{
			Session:  session,
			Question: q,
			Depth:    "normal",
		})
		rec.DurationMs = time.Since(start).Milliseconds()
		if turn != nil {
			populateTurnRecord(&rec, turn)
		}
		if err != nil {
			rec.Error = err.Error()
			rec.Resumed = false
			ep.Turns = append(ep.Turns, rec)
			appendTraceProjections(ep, rec)
			return fmt.Errorf("sidecar discovery turn %d: %w", i, err)
		}
		rec.Report = report
		ep.Turns = append(ep.Turns, rec)
		appendTraceProjections(ep, rec)
		ep.Report = report
		if turn.AgentInfo != nil {
			ep.Identity.AdapterName = turn.AgentInfo.Name
			ep.Identity.AdapterVersion = turn.AgentInfo.Version
			ep.Identity.AdapterSubjectModel = modelFromMeta(turn.AgentInfo.Meta)
		}
	}
	return nil
}

func runDirectDiscoveryEpisode(ctx context.Context, cfg RunConfig, task DiscoveryTask, profile Profile, ep *Episode, rt episodeRuntime) error {
	repoScoped := Task{ID: task.ID, Turns: task.Turns}
	return runDirectEpisode(ctx, cfg, repoScoped, profile, ep, rt)
}

// runSidecarEpisode drives the production sidecar path: one sidecar.Ask per
// question, exactly like repeated "ghx sidecar ask" CLI invocations. Follow-up
// turns exercise real ACP session resumption via the persisted session ID.
func runSidecarEpisode(ctx context.Context, cfg RunConfig, task Task, ep *Episode, rt episodeRuntime) error {
	// EvalMode:true enables emitRawSDKMessages audit channel (ADR-0020.1 D5).
	// Model is wired from GHX_EVAL_SUBJECT_MODEL so the subject model is
	// structurally pinned at session creation (ADR-0020.1 D4). The identity
	// label (ep.Identity.SubjectModel) is set separately from the adapter
	// initialize response and the env var — they are independent signals.
	scfg := sidecar.Config{
		AgentCmd:    cfg.AgentCmd,
		SessionsDir: cfg.SessionsDir,
		Cwd:         rt.Cwd,
		Env:         rt.Env,
		Model:       resolveSubjectModel(cfg.AgentCmd), // from GHX_EVAL_SUBJECT_MODEL / wrapper
		EvalMode:    true,
	}
	session := ep.ID

	for i, q := range task.Turns {
		rec := TurnRecord{Turn: i, Question: q}
		start := time.Now()

		// A follow-up turn counts as resumed only when a prior ACP session
		// ID exists on disk for LoadSession to use.
		if i > 0 {
			meta, metaErr := sidecar.ReadMeta(cfg.SessionsDir, session)
			rec.Resumed = metaErr == nil && meta != nil && meta.ACPSessionID != ""
		}

		report, turn, err := sidecar.Ask(ctx, scfg, sidecar.AskRequest{
			Session:  session,
			Repo:     task.Repo,
			Question: q,
			Depth:    "normal",
		})
		rec.DurationMs = time.Since(start).Milliseconds()
		// Ask returns the partial TurnResult on failure too (ADR-0027 D3);
		// capture whatever the failed turn produced before recording the error
		// so failed episodes stay auditable.
		if turn != nil {
			populateTurnRecord(&rec, turn)
		}
		if err != nil {
			rec.Error = err.Error()
			rec.Resumed = false
			ep.Turns = append(ep.Turns, rec)
			appendTraceProjections(ep, rec)
			return fmt.Errorf("sidecar turn %d: %w", i, err)
		}

		rec.Report = report
		ep.Turns = append(ep.Turns, rec)
		appendTraceProjections(ep, rec)
		ep.Report = report
		if turn.AgentInfo != nil {
			ep.Identity.AdapterName = turn.AgentInfo.Name
			ep.Identity.AdapterVersion = turn.AgentInfo.Version
			ep.Identity.AdapterSubjectModel = modelFromMeta(turn.AgentInfo.Meta)
		}
	}
	return nil
}

// populateTurnRecord copies one sidecar TurnResult into the episode's turn
// record. Shared by the success and failure paths so partial results from
// failed turns carry the same audit fields (ADR-0027 D3).
func populateTurnRecord(rec *TurnRecord, turn *sidecar.TurnResult) {
	rec.Text = turn.FullText
	rec.Thinking = turn.Thinking
	rec.ReplayedText = turn.ReplayedText
	rec.ReplayedThinking = turn.ReplayedThinking
	rec.ToolCalls = turn.ToolCalls
	rec.ToolOutputChars = turn.ToolOutputChars
	rec.ReportRetried = turn.ReportRetried
	rec.ReportCoerced = turn.ReportCoerced
	rec.WrapUpRecovered = turn.WrapUpRecovered
	rec.SessionRecreated = turn.SessionRecreated
	rec.RawSDK = turn.RawSDK
	for _, tr := range turn.ToolTraces {
		rec.ToolTraces = append(rec.ToolTraces, convertSidecarTrace(tr))
	}
	for _, tr := range turn.ReplayedToolTraces {
		rec.ReplayedToolTraces = append(rec.ReplayedToolTraces, convertSidecarTrace(tr))
	}
	rebuildToolSummaries(rec)
}

// runDirectEpisode drives a plain or ghx profile: one live agent process for
// the whole episode, one ACP session, one Prompt call per question. Follow-up
// turns are Resumed by construction — the session never ends between turns.
// The recon read-only contract is the spec's zero values: nil write policy
// (deny all writes), fs.writeTextFile advertised false, no MCP servers.
func runDirectEpisode(ctx context.Context, cfg RunConfig, task Task, profile Profile, ep *Episode, rt episodeRuntime) error {
	return runLiveAgentEpisode(ctx, cfg, ep, rt, liveAgentEpisodeSpec{
		client:     &evalClient{},
		mcpServers: []acp.McpServer{},
		questions:  task.Turns,
		prompt:     func(i int, q string) string { return directPrompt(profile, task.Repo, q, i) },
		turnLabel:  "direct",
	})
}

// liveAgentEpisodeSpec parameterizes one live-process ACP episode: which
// client (and therefore write policy), which fs capabilities are advertised,
// which MCP servers the session registers, and the per-turn prompts. It is
// the single spawn machinery shared by the direct recon profiles
// (runDirectEpisode) and the ADR-0032.1 host-task arms (RunHostTrial) —
// parameterized once, never forked.
type liveAgentEpisodeSpec struct {
	// client captures turns and violations; callers set its write policy
	// before the episode starts (nil = recon deny-all contract).
	client *evalClient
	// fsWriteTextFile is the fs.writeTextFile client capability advertised at
	// initialize. Recon profiles advertise false (read-only contract, never
	// flipped); host-task episodes advertise true so workspace-scoped client
	// fs writes reach the write policy (ADR-0032.1 S2 risk 4).
	fsWriteTextFile bool
	// mcpServers is registered on the session: empty for recon profiles and
	// the host control arm, the recon server for host arm B.
	mcpServers []acp.McpServer
	// questions are the per-turn questions recorded on the episode.
	questions []string
	// prompt renders the full prompt text sent for one turn.
	prompt func(turn int, question string) string
	// turnLabel names the episode flavor in turn-failure errors ("direct",
	// "host").
	turnLabel string
}

// runLiveAgentEpisode spawns the configured ACP agent, initializes with the
// spec's client capabilities, opens one session (with the spec's MCP
// servers), then sends one prompt per question, capturing turns, traces, and
// violations onto ep. All sessions enable only the raw-SDK audit channel
// (ADR-0016.10 D2): audit notifications for the trace-capture comparator
// without steering the subject agent, so baseline conditions are unchanged.
func runLiveAgentEpisode(ctx context.Context, cfg RunConfig, ep *Episode, rt episodeRuntime, spec liveAgentEpisodeSpec) error {
	agentBin, agentArgs := sidecar.SplitAgentCmd(cfg.AgentCmd)
	cmd := exec.CommandContext(ctx, agentBin, agentArgs...)
	cmd.Stderr = os.Stderr
	cmd.Env = rt.Env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", cfg.AgentCmd, err)
	}
	defer sidecar.ShutdownAgent(cmd, stdin)

	client := spec.client
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: spec.fsWriteTextFile},
		},
	})
	if err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}
	if initResp.AgentInfo != nil {
		ep.Identity.AdapterName = initResp.AgentInfo.Name
		ep.Identity.AdapterVersion = initResp.AgentInfo.Version
		ep.Identity.AdapterSubjectModel = modelFromMeta(initResp.AgentInfo.Meta)
	}

	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        rt.Cwd,
		McpServers: spec.mcpServers,
		Meta:       map[string]any{"claudeCode": map[string]any{"emitRawSDKMessages": true}},
	})
	if err != nil {
		return fmt.Errorf("acp new session: %w", err)
	}

	for i, q := range spec.questions {
		record := TurnRecord{Turn: i, Question: q, Resumed: i > 0}
		client.current = &record
		client.promptSent = false
		start := time.Now()
		client.promptSent = true
		_, err := conn.Prompt(ctx, acp.PromptRequest{
			SessionId: sess.SessionId,
			Prompt:    []acp.ContentBlock{acp.TextBlock(spec.prompt(i, q))},
		})
		record.DurationMs = time.Since(start).Milliseconds()
		client.current = nil

		if err != nil {
			record.Error = err.Error()
			ep.Turns = append(ep.Turns, record)
			appendTraceProjections(ep, record)
			ep.Violations = append(ep.Violations, client.violations...)
			return fmt.Errorf("%s turn %d: %w", spec.turnLabel, i, err)
		}
		ep.Turns = append(ep.Turns, record)
		appendTraceProjections(ep, record)
	}

	ep.Violations = append(ep.Violations, client.violations...)
	return nil
}

func logEpisodeProgress(task Task, ep *Episode) {
	fmt.Fprintf(os.Stderr, "eval episode complete task=%s profile=%s turns=%d overall=%.3f mainAgentChars=%d duration=%s\n",
		task.ID, ep.Profile, len(ep.Turns), ep.Rewards.Overall, ep.Context.MainAgentChars, ep.EndedAt.Sub(ep.StartedAt).Round(time.Millisecond))
}

func logDiscoveryEpisodeProgress(task DiscoveryTask, ep *Episode) {
	verifiedRecall := 0.0
	if ep.DiscoveryRewards != nil {
		verifiedRecall = ep.DiscoveryRewards.VerifiedRecall
	}
	fmt.Fprintf(os.Stderr, "discovery eval episode complete task=%s profile=%s turns=%d verifiedRecall=%.3f mainAgentChars=%d duration=%s\n",
		task.ID, ep.Profile, len(ep.Turns), verifiedRecall, ep.Context.MainAgentChars, ep.EndedAt.Sub(ep.StartedAt).Round(time.Millisecond))
}

func modelFromMeta(meta map[string]any) string {
	for _, key := range []string{"subjectModel", "model", "modelName"} {
		if v, ok := meta[key].(string); ok {
			return v
		}
	}
	return ""
}

// finalizeContext computes the workflow-boundary accounting from ADR-0016.1.
//
// Sidecar: the main agent receives only the serialized report JSON per turn;
// everything else — produced text AND tool outputs the agent consumed — is
// sidecar-internal. Direct profiles: the main agent IS the explorer, so all
// produced text plus consumed tool outputs are main-agent context. Tool
// output sizes come from tool_call/tool_call_update content when the agent
// adapter reports it; adapters that omit output content still undercount.
func finalizeContext(ep *Episode) {
	total := 0
	for _, t := range ep.Turns {
		total += len(t.Text) + t.ToolOutputChars
	}

	if ep.Profile != ProfileSidecar {
		ep.Context = ContextAccounting{
			MainAgentChars:       total,
			SidecarInternalChars: 0,
			TotalWorkflowChars:   total,
		}
		return
	}

	main := 0
	for _, t := range ep.Turns {
		if t.Report != nil {
			if data, err := json.Marshal(t.Report); err == nil {
				main += len(data)
			}
		}
	}
	internal := total - main
	if internal < 0 {
		internal = 0
	}
	ep.Context = ContextAccounting{
		MainAgentChars:       main,
		SidecarInternalChars: internal,
		TotalWorkflowChars:   total,
	}
}
