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
	// per-run temporary directory so episodes never touch ~/.ghx-sidecar.
	SessionsDir string
}

// RunEpisode executes one task under one profile and returns the scored
// episode. A mid-episode turn failure ends the episode early; the partial
// episode (with the turn error recorded) is returned alongside the error so
// failed runs still produce artifacts (ADR-0016 implementation note 4).
func RunEpisode(ctx context.Context, cfg RunConfig, task Task, profile Profile) (*Episode, error) {
	ep := &Episode{
		ID:        fmt.Sprintf("%s_%s_%d", task.ID, profile, time.Now().UnixMilli()),
		TaskID:    task.ID,
		Repo:      task.Repo,
		Profile:   profile,
		StartedAt: time.Now().UTC(),
	}

	var err error
	if profile == ProfileSidecar {
		err = runSidecarEpisode(ctx, cfg, task, ep)
	} else {
		err = runDirectEpisode(ctx, cfg, task, profile, ep)
	}

	ep.EndedAt = time.Now().UTC()
	finalizeContext(ep)
	ep.Rewards = ComputeRewards(task, ep)
	return ep, err
}

// runSidecarEpisode drives the production sidecar path: one sidecar.Ask per
// question, exactly like repeated "ghx sidecar ask" CLI invocations. Follow-up
// turns exercise real ACP session resumption via the persisted session ID.
func runSidecarEpisode(ctx context.Context, cfg RunConfig, task Task, ep *Episode) error {
	scfg := sidecar.Config{AgentCmd: cfg.AgentCmd, SessionsDir: cfg.SessionsDir}
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
		rec.ToolCalls = turn.ToolCalls
		rec.ToolOutputChars = turn.ToolOutputChars
		rec.Report = report
		ep.Turns = append(ep.Turns, rec)
		ep.Report = report
	}
	return nil
}

// runDirectEpisode drives a plain or ghx profile: one live agent process for
// the whole episode, one ACP session, one Prompt call per question. Follow-up
// turns are Resumed by construction — the session never ends between turns.
func runDirectEpisode(ctx context.Context, cfg RunConfig, task Task, profile Profile, ep *Episode) error {
	cmd := exec.CommandContext(ctx, cfg.AgentCmd)
	cmd.Stderr = os.Stderr

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
	cwd, _ := os.Getwd()

	if _, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	}); err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}

	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{Cwd: cwd, McpServers: []acp.McpServer{}})
	if err != nil {
		return fmt.Errorf("acp new session: %w", err)
	}

	for i, q := range task.Turns {
		record := TurnRecord{Turn: i, Question: q, Resumed: i > 0}
		client.current = &record
		start := time.Now()
		_, err := conn.Prompt(ctx, acp.PromptRequest{
			SessionId: sess.SessionId,
			Prompt:    []acp.ContentBlock{acp.TextBlock(directPrompt(profile, task.Repo, q, i))},
		})
		record.DurationMs = time.Since(start).Milliseconds()
		client.current = nil

		if err != nil {
			record.Error = err.Error()
			ep.Turns = append(ep.Turns, record)
			ep.Violations = append(ep.Violations, client.violations...)
			return fmt.Errorf("direct turn %d: %w", i, err)
		}
		ep.Turns = append(ep.Turns, record)
	}

	ep.Violations = append(ep.Violations, client.violations...)
	return nil
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
