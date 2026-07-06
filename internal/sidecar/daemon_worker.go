package sidecar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

// AgentPool owns warm ACP workers keyed by resolved ghx session name.
type AgentPool struct {
	mu            sync.Mutex
	workers       map[string]*AgentWorker
	ttl           time.Duration
	maxConcurrent chan struct{}
}

// NewAgentPool constructs an empty warm ACP worker pool.
func NewAgentPool() *AgentPool {
	return &AgentPool{
		workers:       map[string]*AgentWorker{},
		ttl:           30 * time.Minute,
		maxConcurrent: make(chan struct{}, 4),
	}
}

// RunnerFor returns the serialized turn runner for a session, creating it on
// demand. The worker owns one ACP subprocess and connection.
func (p *AgentPool) RunnerFor(session string, cfg Config) TurnRunner {
	p.mu.Lock()
	defer p.mu.Unlock()
	w := p.workers[session]
	if w == nil {
		w = &AgentWorker{pool: p, session: session, cfg: cfg}
		p.workers[session] = w
	}
	return w.RunTurn
}

// Shutdown gracefully closes every warm ACP worker.
func (p *AgentPool) Shutdown() {
	p.mu.Lock()
	workers := make([]*AgentWorker, 0, len(p.workers))
	for _, w := range p.workers {
		workers = append(workers, w)
	}
	p.workers = map[string]*AgentWorker{}
	p.mu.Unlock()
	for _, w := range workers {
		w.Shutdown()
	}
}

// Drop retires the warm worker for one session, closing its ACP process. The
// reroute path uses it so a corrected session's next turn starts from the
// durable ledger instead of a cached ACP conversation (ADR-0030.1 D5).
func (p *AgentPool) Drop(session string) {
	p.mu.Lock()
	w := p.workers[session]
	delete(p.workers, session)
	p.mu.Unlock()
	if w != nil {
		w.Shutdown()
	}
}

func (p *AgentPool) expire(session string, worker *AgentWorker) {
	p.mu.Lock()
	if p.workers[session] != worker {
		p.mu.Unlock()
		return
	}
	delete(p.workers, session)
	p.mu.Unlock()
	worker.Shutdown()
}

// AgentWorker serializes turns for one ghx session over one warm ACP process.
type AgentWorker struct {
	mu            sync.Mutex
	pool          *AgentPool
	session       string
	cfg           Config
	turnCtx       context.Context
	cancelTurnCtx context.CancelCauseFunc
	cmd           *exec.Cmd
	stdin         io.Closer
	stderrCloser  io.Closer
	conn          *acp.ClientSideConnection
	client        *denyClient
	loadSession   bool
	sessionID     string
	agentInfo     *ImplementationInfo
	lastActivity  atomic.Int64
	idleTimer     *time.Timer
}

// RunTurn executes one prompt on the worker's warm ACP connection. Before each
// prompt after session creation it reloads the ACP session with the current
// report-sink MCP server, preserving strict report capture without respawning
// the agent process.
func (w *AgentWorker) RunTurn(ctx context.Context, opts RunTurnOptions) (TurnResult, string, error) {
	w.pool.maxConcurrent <- struct{}{}
	defer func() { <-w.pool.maxConcurrent }()

	w.mu.Lock()
	defer w.mu.Unlock()
	w.stopIdleTimerLocked()
	defer w.markIdleLocked()

	// Live turn log (ADR-0022.1), opened per turn: the path is per-session and
	// append-only, so reopening on every prompt is safe and keeps the file
	// handle's lifetime bound to the turn rather than the warm worker.
	live := NewLiveLog(opts.LiveLogPath)
	defer live.Close()

	if err := w.ensureStarted(ctx, opts); err != nil {
		w.shutdownLocked()
		return TurnResult{}, "", err
	}
	result, sessionID, err := w.configureSession(ctx, opts, live)
	if err != nil {
		if IsPeerClosedError(err) {
			w.shutdownLocked()
		}
		return result, sessionID, err
	}
	result, err = w.prompt(ctx, opts, acp.SessionId(sessionID))
	if result.AgentInfo == nil {
		result.AgentInfo = w.agentInfo
	}
	if err != nil && IsPeerClosedError(err) {
		w.shutdownLocked()
	}
	return result, sessionID, err
}

// Shutdown stops the worker's ACP process.
func (w *AgentWorker) Shutdown() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.shutdownLocked()
}

func (w *AgentWorker) ensureStarted(ctx context.Context, opts RunTurnOptions) error {
	if w.conn != nil {
		return nil
	}
	liveness := resolveLivenessTimeout(opts.LivenessTimeout)
	w.turnCtx, w.cancelTurnCtx = context.WithCancelCause(context.Background())
	agentBin, agentArgs := splitAgentCmd(w.cfg.AgentCmd)
	if opts.AgentCmd != "" {
		agentBin, agentArgs = splitAgentCmd(opts.AgentCmd)
	}
	cmd := exec.CommandContext(w.turnCtx, agentBin, agentArgs...)
	if w.cfg.Cwd != "" {
		cmd.Dir = w.cfg.Cwd
	}
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if w.cfg.Env != nil {
		cmd.Env = w.cfg.Env
	}
	if opts.Env != nil {
		cmd.Env = opts.Env
	}
	// Tee the warm adapter's stderr to the session's agent-stderr.log
	// (ADR-0033 D2). The warm worker owns one long-lived adapter process per
	// session, so the log path is stable for the worker's life and bound once
	// here; the closer is released in shutdownLocked.
	stderrW, stderrCloser := openAgentStderr(opts.AgentStderrPath)
	w.stderrCloser = stderrCloser
	cmd.Stderr = stderrW
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", w.cfg.AgentCmd, err)
	}
	w.cmd = cmd
	w.stdin = stdin
	w.lastActivity.Store(time.Now().UnixNano())
	touch := func() { w.lastActivity.Store(time.Now().UnixNano()) }
	w.client = &denyClient{result: &TurnResult{}, onActivity: touch}
	w.conn = acp.NewClientSideConnection(w.client, stdin, stdout)
	if liveness > 0 {
		go w.watchLiveness(w.turnCtx, w.cancelTurnCtx, liveness)
	}
	initResp, err := w.conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	})
	if err != nil {
		return fmt.Errorf("acp initialize: %w", turnFailureCause(w.turnCtx, err))
	}
	touch()
	w.loadSession = initResp.AgentCapabilities.LoadSession
	if initResp.AgentInfo != nil {
		w.agentInfo = &ImplementationInfo{Name: initResp.AgentInfo.Name, Version: initResp.AgentInfo.Version, Meta: initResp.AgentInfo.Meta}
	}
	return nil
}

func (w *AgentWorker) configureSession(ctx context.Context, opts RunTurnOptions, live *LiveLog) (TurnResult, string, error) {
	var result TurnResult
	w.resetClient(&result, live)
	cwd := opts.Cwd
	if cwd == "" {
		cwd = w.cfg.Cwd
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if w.sessionID == "" {
		if opts.ACPSessionID != "" && w.loadSession {
			// Fresh worker resuming a persisted ACP session: the adapter
			// process is new, so the full session meta must be re-asserted or
			// the resumed session runs on adapter defaults (ADR-0020.2).
			_, err := w.conn.LoadSession(ctx, acp.LoadSessionRequest{
				SessionId:  acp.SessionId(opts.ACPSessionID),
				Cwd:        cwd,
				McpServers: reportSinkMcpServers(opts.ReportSinkPath),
				Meta:       opts.SessionMeta,
			})
			if err != nil {
				return result, "", fmt.Errorf("acp load session: %w", turnFailureCause(w.turnCtx, err))
			}
			w.sessionID = opts.ACPSessionID
			return result, w.sessionID, nil
		}
		req := acp.NewSessionRequest{Cwd: cwd, McpServers: reportSinkMcpServers(opts.ReportSinkPath)}
		if opts.SessionMeta != nil {
			req.Meta = opts.SessionMeta
		}
		resp, err := w.conn.NewSession(ctx, req)
		if err != nil {
			return result, "", fmt.Errorf("acp new session: %w", turnFailureCause(w.turnCtx, err))
		}
		w.sessionID = string(resp.SessionId)
		return result, w.sessionID, nil
	}
	if w.loadSession {
		// Warm-worker reload (report-sink refresh): carry the meta here too so
		// steering never depends on which reload path the turn took
		// (ADR-0020.2).
		_, err := w.conn.LoadSession(ctx, acp.LoadSessionRequest{
			SessionId:  acp.SessionId(w.sessionID),
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
			Meta:       opts.SessionMeta,
		})
		if err != nil {
			return result, w.sessionID, fmt.Errorf("acp load session: %w", turnFailureCause(w.turnCtx, err))
		}
	}
	return result, w.sessionID, nil
}

func (w *AgentWorker) prompt(ctx context.Context, opts RunTurnOptions, sessionID acp.SessionId) (TurnResult, error) {
	w.client.markPromptSent()
	promptDone := make(chan error, 1)
	go func() {
		_, perr := w.conn.Prompt(ctx, acp.PromptRequest{
			SessionId: sessionID,
			Prompt:    []acp.ContentBlock{acp.TextBlock(opts.Prompt)},
		})
		promptDone <- perr
	}()
	var promptErr error
	select {
	case promptErr = <-promptDone:
	case <-w.conn.Done():
		perr, finished := awaitPromptOrGrace(promptDone)
		if finished {
			promptErr = perr
		} else {
			promptErr = errors.New(peerClosedMarker + " before the prompt turn completed")
		}
		if promptErr != nil && !IsPeerClosedError(promptErr) {
			promptErr = fmt.Errorf("%s: %w", peerClosedMarker, promptErr)
		}
	case <-ctx.Done():
		if perr, finished := awaitPromptOrGrace(promptDone); finished {
			promptErr = perr
		} else {
			promptErr = ctx.Err()
		}
	case <-w.turnCtx.Done():
		if perr, finished := awaitPromptOrGrace(promptDone); finished {
			promptErr = perr
		} else {
			promptErr = context.Cause(w.turnCtx)
		}
	}
	if cause := context.Cause(w.turnCtx); errors.Is(cause, ErrLivenessTimeout) {
		promptErr = cause
	}
	result := w.client.closeAndSnapshot()
	if promptErr != nil {
		return result, fmt.Errorf("acp prompt: %w", promptErr)
	}
	os.Stdout.WriteString("\n")
	return result, nil
}

// resetClient rebinds the warm client to a fresh turn: a new result, replay
// accounting reset, and this turn's live log (nil-safe; closed by RunTurn when
// the prompt returns).
func (w *AgentWorker) resetClient(result *TurnResult, live *LiveLog) {
	w.client.mu.Lock()
	w.client.result = result
	w.client.promptSent = false
	w.client.closed = false
	w.client.live = live
	w.client.mu.Unlock()
}

func (w *AgentWorker) watchLiveness(ctx context.Context, cancel context.CancelCauseFunc, liveness time.Duration) {
	for {
		idle := time.Duration(time.Now().UnixNano() - w.lastActivity.Load())
		remaining := liveness - idle
		if remaining <= 0 {
			cancel(fmt.Errorf("%w: no ACP session update for %s (set %s to adjust)",
				ErrLivenessTimeout, liveness, LivenessTimeoutEnv))
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(remaining):
		}
	}
}

func (w *AgentWorker) shutdownLocked() {
	w.stopIdleTimerLocked()
	if w.cancelTurnCtx != nil {
		w.cancelTurnCtx(nil)
	}
	if w.cmd != nil && w.stdin != nil {
		ShutdownAgent(w.cmd, w.stdin)
	}
	if w.stderrCloser != nil {
		w.stderrCloser.Close()
	}
	w.turnCtx = nil
	w.cancelTurnCtx = nil
	w.cmd = nil
	w.stdin = nil
	w.stderrCloser = nil
	w.conn = nil
	w.client = nil
	w.sessionID = ""
	w.agentInfo = nil
}

func (w *AgentWorker) stopIdleTimerLocked() {
	if w.idleTimer != nil {
		w.idleTimer.Stop()
		w.idleTimer = nil
	}
}

func (w *AgentWorker) markIdleLocked() {
	if w.pool == nil || w.pool.ttl <= 0 || w.cmd == nil {
		return
	}
	w.idleTimer = time.AfterFunc(w.pool.ttl, func() {
		w.pool.expire(w.session, w)
	})
}
