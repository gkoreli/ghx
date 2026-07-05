package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// RunConfig configures one episode run.
type RunConfig struct {
	// AgentCmd is the ACP agent binary spawned for every profile.
	AgentCmd string
	// SessionsDir is the sidecar session store for this run. Use a
	// per-run temporary directory so episodes never touch ~/.ghx-sidecar.
	SessionsDir string
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

	ep := &Episode{
		ID:        fmt.Sprintf("%s_%s_%d", task.ID, profile, time.Now().UnixMilli()),
		TaskID:    task.ID,
		Repo:      task.Repo,
		Profile:   profile,
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
	logEpisodeProgress(task, ep)
	return ep, err
}

// runSidecarEpisode drives the production sidecar path: one sidecar.Ask per
// question, exactly like repeated "ghx sidecar ask" CLI invocations. Follow-up
// turns exercise real ACP session resumption via the persisted session ID.
func runSidecarEpisode(ctx context.Context, cfg RunConfig, task Task, ep *Episode, rt episodeRuntime) error {
	scfg := sidecar.Config{AgentCmd: cfg.AgentCmd, SessionsDir: cfg.SessionsDir, Cwd: rt.Cwd, Env: rt.Env}
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
		if err != nil {
			rec.Error = err.Error()
			rec.Resumed = false
			ep.Turns = append(ep.Turns, rec)
			return fmt.Errorf("sidecar turn %d: %w", i, err)
		}

		rec.Text = turn.FullText
		rec.ReplayedText = turn.ReplayedText
		rec.ToolCalls = turn.ToolCalls
		rec.ToolOutputChars = turn.ToolOutputChars
		for _, tr := range turn.ToolTraces {
			rec.ToolTraces = append(rec.ToolTraces, convertSidecarTrace(tr))
		}
		for _, tr := range turn.ReplayedToolTraces {
			rec.ReplayedToolTraces = append(rec.ReplayedToolTraces, convertSidecarTrace(tr))
		}
		rebuildToolSummaries(&rec)
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

// runDirectEpisode drives a plain or ghx profile: one live agent process for
// the whole episode, one ACP session, one Prompt call per question. Follow-up
// turns are Resumed by construction — the session never ends between turns.
func runDirectEpisode(ctx context.Context, cfg RunConfig, task Task, profile Profile, ep *Episode, rt episodeRuntime) error {
	cmd := exec.CommandContext(ctx, cfg.AgentCmd)
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

	client := &evalClient{}
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
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

	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: rt.Cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		return fmt.Errorf("acp new session: %w", err)
	}

	for i, q := range task.Turns {
		record := TurnRecord{Turn: i, Question: q, Resumed: i > 0}
		client.current = &record
		client.promptSent = false
		start := time.Now()
		client.promptSent = true
		_, err := conn.Prompt(ctx, acp.PromptRequest{
			SessionId: sess.SessionId,
			Prompt:    []acp.ContentBlock{acp.TextBlock(directPrompt(profile, task.Repo, q, i))},
		})
		record.DurationMs = time.Since(start).Milliseconds()
		client.current = nil

		if err != nil {
			record.Error = err.Error()
			ep.Turns = append(ep.Turns, record)
			appendTraceProjections(ep, record)
			ep.Violations = append(ep.Violations, client.violations...)
			return fmt.Errorf("direct turn %d: %w", i, err)
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

// EpisodeAnomalies flags sidecar episodes whose reports contradict a
// verified precondition (ADR-0016.7): preflight guarantees ghx is
// installed, so a BLOCKED report means the downstream agent never ran
// ghx via the shell, and a WARN placeholder means a completed turn's
// report was lost. Both are loud harness alarms — smoke runs fail on
// them; gate runs score them honestly but the operator must investigate
// before spending further rounds.
func EpisodeAnomalies(ep *Episode) []string {
	if ep == nil || ep.Profile != ProfileSidecar {
		return nil
	}
	var anomalies []string
	for _, turn := range ep.Turns {
		if turn.Report == nil {
			continue
		}
		switch {
		case strings.HasPrefix(turn.Report.Answer, "BLOCKED:"):
			anomalies = append(anomalies, fmt.Sprintf(
				"turn %d report is BLOCKED (%q) despite preflight-verified ghx — agent never ran ghx via shell",
				turn.Turn, turn.Report.Answer))
		case strings.HasPrefix(turn.Report.Answer, "WARN: sidecar did not emit"):
			anomalies = append(anomalies, fmt.Sprintf(
				"turn %d produced no extractable <ghx-report> even after the one-shot retry",
				turn.Turn))
		}
	}
	return anomalies
}
