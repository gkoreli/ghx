package sidecar

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DaemonMetadata is the runtime/daemon.json lifecycle record.
type DaemonMetadata struct {
	// PID is the operating-system process id of the daemon.
	PID int `json:"pid"`
	// Socket is the Unix-domain socket path the daemon listens on.
	Socket string `json:"socket"`
	// Version is the ghx binary version that started the daemon.
	Version string `json:"version"`
	// ConfigDigest fingerprints process-shaping config fields.
	ConfigDigest string `json:"configDigest"`
	// StartedAt is the UTC daemon start timestamp.
	StartedAt time.Time `json:"startedAt"`
	// ExePath is the executable the daemon process is running. Part of the
	// client health handshake: two dev builds share Version "dev", so version
	// alone cannot detect a stale warm daemon serving old code (live incident
	// 2026-07-06: a fresh client was silently served by a 3-hour-old spot
	// binary and shipped no live.jsonl).
	ExePath string `json:"exePath,omitempty"`
}

// DaemonPing is the health response returned by ghx.sidecar.Ping.
type DaemonPing struct {
	// PID is the daemon process id.
	PID int `json:"pid"`
	// Version is the daemon ghx binary version.
	Version string `json:"version"`
	// ConfigDigest is the daemon's process-shaping config fingerprint.
	ConfigDigest string `json:"configDigest"`
	// Socket is the active Unix-domain socket path.
	Socket string `json:"socket"`
	// StartedAt is the UTC daemon start timestamp.
	StartedAt time.Time `json:"startedAt"`
	// ExePath is the executable the daemon process is running (see
	// DaemonMetadata.ExePath). Empty on daemons older than this field.
	ExePath string `json:"exePath,omitempty"`
}

// AskResponse is the daemon JSON-RPC response for ghx.sidecar.Ask.
type AskResponse struct {
	// Report is the validated sidecar evidence report.
	Report *Report `json:"report"`
	// Turn carries raw ACP turn accounting and the artifact pointer.
	Turn *TurnResult `json:"turn"`
	// Artifacts points at the durable session artifact bundle.
	Artifacts ArtifactsRef `json:"artifacts"`
}

// DaemonServer owns the local JSON-RPC socket and warm sidecar runtime.
type DaemonServer struct {
	version   string
	cfg       Config
	pool      *AgentPool
	meta      DaemonMetadata
	ln        net.Listener
	stop      chan struct{}
	once      sync.Once
	mu        sync.Mutex
	drain     sync.WaitGroup
	draining  bool
	runnerFor func(session string, cfg Config) TurnRunner
}

var daemonDrainTimeout = 30 * time.Second

// RunDaemon starts the foreground daemon and blocks until shutdown.
func RunDaemon(ctx context.Context, version string, cfg Config) error {
	if runtime.GOOS == "windows" {
		return errors.New("sidecar daemon sockets are not implemented on Windows")
	}
	s, err := NewDaemonServer(version, cfg)
	if err != nil {
		return err
	}
	return s.Serve(ctx)
}

// NewDaemonServer constructs a daemon server rooted in the active GHX_HOME.
func NewDaemonServer(version string, cfg Config) (*DaemonServer, error) {
	if err := cfg.ValidateDaemonRuntimeConfig(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(RuntimeDir(), 0o700); err != nil {
		return nil, fmt.Errorf("mkdir runtime dir: %w", err)
	}
	socket := SocketPath()
	exe, _ := os.Executable()
	meta := DaemonMetadata{
		PID:          os.Getpid(),
		Socket:       socket,
		Version:      version,
		ConfigDigest: ConfigDigest(cfg),
		StartedAt:    time.Now().UTC(),
		ExePath:      exe,
	}
	pool := NewAgentPoolForConfig(cfg)
	return &DaemonServer{
		version:   version,
		cfg:       cfg,
		pool:      pool,
		meta:      meta,
		stop:      make(chan struct{}),
		runnerFor: pool.RunnerFor,
	}, nil
}

// Serve accepts newline-delimited JSON-RPC requests on the daemon socket.
func (s *DaemonServer) Serve(ctx context.Context) error {
	if err := assertUnderRoot(s.meta.Socket); err != nil {
		return err
	}
	_ = os.Remove(s.meta.Socket)
	ln, err := net.Listen("unix", s.meta.Socket)
	if err != nil {
		return fmt.Errorf("listen unix socket: %w", err)
	}
	s.ln = ln
	_ = os.Chmod(s.meta.Socket, 0o600)
	if err := writeDaemonMetadata(s.meta); err != nil {
		_ = ln.Close()
		return err
	}
	defer func() {
		s.waitForDrain(daemonDrainTimeout)
		s.pool.Shutdown()
		_ = ln.Close()
		_ = os.Remove(s.meta.Socket)
	}()
	go func() {
		select {
		case <-ctx.Done():
			s.Shutdown()
		case <-s.stop:
		}
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.stop:
				return nil
			default:
				return err
			}
		}
		go s.handleConn(ctx, conn)
	}
}

// Shutdown stops the socket accept loop and warm workers.
func (s *DaemonServer) Shutdown() {
	s.once.Do(func() {
		s.mu.Lock()
		s.draining = true
		s.mu.Unlock()
		close(s.stop)
		if s.ln != nil {
			_ = s.ln.Close()
		}
	})
}

func (s *DaemonServer) beginAsk() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.draining {
		return errors.New("sidecar daemon is shutting down")
	}
	s.drain.Add(1)
	return nil
}

func (s *DaemonServer) endAsk() {
	s.drain.Done()
}

func (s *DaemonServer) waitForDrain(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		s.drain.Wait()
		close(done)
	}()
	if timeout <= 0 {
		<-done
		return
	}
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *DaemonServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	enc := json.NewEncoder(conn)
	for sc.Scan() {
		var req rpcRequest
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = enc.Encode(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: err.Error()}})
			continue
		}
		resp := s.handleRPCRequest(ctx, conn, req)
		_ = enc.Encode(resp)
	}
}

func (s *DaemonServer) handleRPCRequest(ctx context.Context, conn net.Conn, req rpcRequest) (resp rpcResponse) {
	resp = rpcResponse{JSONRPC: "2.0", ID: req.ID}
	reqCtx, cancel := context.WithCancel(ctx)
	var watchDone chan struct{}
	var watchStopped <-chan struct{}
	if req.Method == "ghx.sidecar.Ask" {
		watchDone, watchStopped = s.watchConnClosed(reqCtx, conn, cancel)
	}
	defer func() {
		cancel()
		if watchDone != nil {
			close(watchDone)
			_ = conn.SetReadDeadline(time.Now())
			<-watchStopped
			_ = conn.SetReadDeadline(time.Time{})
		}
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "sidecar daemon recovered panic in %s: %v\n%s", req.Method, r, debug.Stack())
			resp.Result = nil
			resp.Error = &rpcError{Code: -32603, Message: fmt.Sprintf("internal daemon panic while handling %s", req.Method)}
		}
	}()
	result, err := s.dispatch(reqCtx, req)
	if err != nil {
		resp.Error = &rpcError{Code: -32000, Message: err.Error()}
		return resp
	}
	resp.Result = result
	return resp
}

func (s *DaemonServer) watchConnClosed(ctx context.Context, conn net.Conn, cancel context.CancelFunc) (chan struct{}, <-chan struct{}) {
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		var buf [1]byte
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			default:
			}
			_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
			n, err := conn.Read(buf[:])
			if err == nil && n > 0 {
				cancel()
				return
			}
			if err == nil {
				continue
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			cancel()
			return
		}
	}()
	return done, stopped
}

func (s *DaemonServer) dispatch(ctx context.Context, req rpcRequest) (any, error) {
	switch req.Method {
	case "ghx.sidecar.Ping":
		return DaemonPing{PID: s.meta.PID, Version: s.meta.Version, ConfigDigest: s.meta.ConfigDigest, Socket: s.meta.Socket, StartedAt: s.meta.StartedAt, ExePath: s.meta.ExePath}, nil
	case "ghx.sidecar.Ask":
		if err := s.beginAsk(); err != nil {
			return nil, err
		}
		defer s.endAsk()
		var ask AskRequest
		if err := json.Unmarshal(req.Params, &ask); err != nil {
			return nil, err
		}
		// Route before picking a runner: the warm-worker pool is keyed by the
		// resolved session name (ADR-0030.1 D1; the decision rides into the
		// ask so it is emitted exactly once).
		decision := RouteQuestion(s.cfg.SessionsDir, ask, RouteConfigFor(s.cfg))
		ask.Session = decision.Session
		runnerFor := s.runnerFor
		if runnerFor == nil {
			runnerFor = s.pool.RunnerFor
		}
		runner := runnerFor(ask.Session, s.cfg)
		report, turn, err := AskWithTurnRunner(ctx, s.cfg, ask, runner, &decision)
		if err != nil {
			return nil, err
		}
		resp := AskResponse{Report: report, Turn: turn}
		if turn != nil {
			resp.Artifacts = turn.Artifacts
		}
		return resp, nil
	case "ghx.sidecar.ListSessions":
		return ListSessions(s.cfg.SessionsDir)
	case "ghx.sidecar.ShowSession":
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		meta, err := ReadMeta(s.cfg.SessionsDir, params.Name)
		if err != nil || meta == nil {
			return meta, err
		}
		reports, err := ListReports(s.cfg.SessionsDir, params.Name)
		return map[string]any{"meta": meta, "reports": reports}, err
	case "ghx.sidecar.Reroute":
		var params RerouteParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, err
		}
		// Retire the source session's warm worker first: its ACP conversation
		// remembers the stray turn, and the on-disk ACPSessionID clear below
		// must not be shadowed by a cached warm session (ADR-0030.1 D5).
		s.pool.Drop(params.From)
		return RerouteTurn(s.cfg.SessionsDir, params.From, params.Turn, params.To)
	case "ghx.sidecar.Shutdown":
		go s.Shutdown()
		return map[string]string{"status": "shutting_down"}, nil
	default:
		return nil, fmt.Errorf("unknown method %s", req.Method)
	}
}

// SocketPath returns the Unix socket path for the active GHX_HOME.
func SocketPath() string { return filepath.Join(RuntimeDir(), "sidecar.sock") }

// MetadataPath returns runtime/daemon.json for the active GHX_HOME.
func MetadataPath() string { return filepath.Join(RuntimeDir(), "daemon.json") }

// ConfigDigest returns the daemon compatibility digest for process-shaping
// config fields.
func ConfigDigest(cfg Config) string {
	type digestConfig struct {
		AgentCmd string   `json:"agent"`
		Cwd      string   `json:"cwd,omitempty"`
		Env      []string `json:"env,omitempty"`
		Model    string   `json:"model,omitempty"`
		// FallbackAgentCmd/FallbackModel shape the ADR-0040 L3
		// quota-degradation ladder: a warm daemon must not keep skipping (or
		// serving) the fallback backend on stale ladder config.
		FallbackAgentCmd string `json:"fallbackAgent,omitempty"`
		FallbackModel    string `json:"fallbackModel,omitempty"`
		// ReportSinkExe is the caller-resolved submit_report server binary.
		// Without it a warm daemon keeps serving a previously overridden
		// GHX_REPORT_SINK_EXE until idle restart (integration audit
		// 2026-07-06 F1) — a version-skewed sink is exactly the silent
		// degradation the doctor check exists to prevent.
		ReportSinkExe string `json:"reportSinkExe,omitempty"`
		// Route covers the session-routing knobs (ADR-0030.1 v1 scope): a
		// warm daemon must not keep routing on stale thresholds/windows
		// after the config changes.
		Route *RouteSettings `json:"route,omitempty"`
		// AgentSettingSources shapes every session's isolation posture
		// (settingSources on the wire); a warm daemon must not keep
		// serving the old posture after the config changes.
		AgentSettingSources []string `json:"agentSettingSources,omitempty"`
		// DaemonWorkerIdleTTLMinutes and DaemonMaxConcurrent shape warm
		// daemon process behavior; D6 treats changed runtime tunables as a
		// stale daemon instead of serving with old pool limits.
		DaemonWorkerIdleTTLMinutes *int `json:"daemonWorkerIdleTTLMinutes,omitempty"`
		DaemonMaxConcurrent        *int `json:"daemonMaxConcurrent,omitempty"`
	}
	// Hash the explicit override only: an empty value means "own executable",
	// which the version handshake already validates, and full resolution is
	// process-dependent (a spawned daemon and a go-test client resolve
	// differently, which would force restart loops).
	data, _ := json.Marshal(digestConfig{AgentCmd: cfg.AgentCmd, Cwd: cfg.Cwd, Env: cfg.Env, Model: cfg.Model, FallbackAgentCmd: cfg.FallbackAgentCmd, FallbackModel: cfg.FallbackModel, ReportSinkExe: os.Getenv("GHX_REPORT_SINK_EXE"), Route: cfg.Route, AgentSettingSources: cfg.AgentSettingSources, DaemonWorkerIdleTTLMinutes: cfg.DaemonWorkerIdleTTLMinutes, DaemonMaxConcurrent: cfg.DaemonMaxConcurrent})
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func writeDaemonMetadata(meta DaemonMetadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(MetadataPath(), append(data, '\n'), 0o600)
}

func assertUnderRoot(path string) error {
	root, err := filepath.Abs(RootDir())
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return err
	}
	if rel == "." || (!strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "..") {
		return nil
	}
	return fmt.Errorf("daemon path %s is outside GHX_HOME %s", path, RootDir())
}

// DaemonClient talks to the local sidecar daemon over newline JSON-RPC.
type DaemonClient struct {
	// Version is the client ghx binary version required for compatibility.
	Version string
	// ExePath is the ghx executable used for auto-spawn. Empty uses os.Executable.
	ExePath string
	// Stdout is reserved for future streaming notifications.
	Stdout ioWriter
	// Stderr receives daemon fallback warnings in higher-level helpers.
	Stderr ioWriter
}

type ioWriter interface {
	Write([]byte) (int, error)
}

// AskViaDaemon asks through the warm daemon, auto-spawning once and falling
// back to daemonless Ask if the daemon path is unavailable.
func AskViaDaemon(ctx context.Context, version string, cfg Config, req AskRequest) (*Report, *TurnResult, bool, error) {
	// Capture the asking shell's auth/transport env for the agent spawn
	// (ADR-0033.1): the daemon's own environment is frozen at first
	// auto-start, so Bedrock/Vertex/gateway credentials must travel with the
	// ask. Values ride only this local socket call; nothing persists them.
	if req.AgentAuthEnv == nil {
		req.AgentAuthEnv = CaptureAgentAuthEnv(nil)
	}
	c := DaemonClient{Version: version, Stdout: os.Stdout, Stderr: os.Stderr}
	resp, err := c.ask(ctx, cfg, req)
	if err == nil {
		return resp.Report, resp.Turn, true, nil
	}
	fmt.Fprintf(os.Stderr, "warning: sidecar daemon unavailable (%v); using daemonless fallback\n", err)
	report, turn, directErr := Ask(ctx, cfg, req)
	return report, turn, false, directErr
}

// RerouteParams are the ghx.sidecar.Reroute RPC parameters (ADR-0030.1 D5):
// move turn Turn of session From to session To and rebuild both ledgers.
type RerouteParams struct {
	// From is the source session name.
	From string `json:"from"`
	// Turn is the source turn number to move.
	Turn int `json:"turn"`
	// To is the destination session name (created when absent).
	To string `json:"to"`
}

// RerouteViaDaemon performs a mis-route correction through the running daemon
// when one is healthy — so the source session's warm ACP worker is retired
// before the on-disk state changes — and directly on disk otherwise. It
// returns whether the daemon path was used. A version-skewed daemon is asked
// to shut down first (its warm workers would otherwise keep the stray ACP
// conversation alive past the correction).
func RerouteViaDaemon(ctx context.Context, version string, cfg Config, params RerouteParams) (*RerouteResult, bool, error) {
	c := DaemonClient{Version: version}
	if ping, err := c.ping(ctx); err == nil {
		if ping.Version == version {
			raw, err := c.call(ctx, "ghx.sidecar.Reroute", params)
			if err != nil {
				return nil, true, err
			}
			var res RerouteResult
			data, _ := json.Marshal(raw)
			if err := json.Unmarshal(data, &res); err != nil {
				return nil, true, err
			}
			return &res, true, nil
		}
		_, _ = c.call(ctx, "ghx.sidecar.Shutdown", map[string]any{})
		time.Sleep(150 * time.Millisecond)
	}
	res, err := RerouteTurn(cfg.SessionsDir, params.From, params.Turn, params.To)
	return res, false, err
}

// ShutdownDaemon asks the active daemon to stop.
func ShutdownDaemon(ctx context.Context, version string, cfg Config) error {
	c := DaemonClient{Version: version}
	if _, err := c.call(ctx, "ghx.sidecar.Shutdown", map[string]any{}); err != nil {
		return err
	}
	return nil
}

func (c DaemonClient) ask(ctx context.Context, cfg Config, req AskRequest) (*AskResponse, error) {
	if err := c.ensure(ctx, cfg); err != nil {
		return nil, err
	}
	raw, err := c.call(ctx, "ghx.sidecar.Ask", req)
	if err != nil {
		return nil, err
	}
	var resp AskResponse
	data, _ := json.Marshal(raw)
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	if resp.Turn != nil && resp.Turn.Artifacts.SessionDir == "" {
		resp.Turn.Artifacts = resp.Artifacts
	}
	return &resp, nil
}

// Ask sends one request through this daemon client. It is useful for callers
// that need explicit executable/version control instead of AskViaDaemon's
// daemonless fallback.
func (c DaemonClient) Ask(ctx context.Context, cfg Config, req AskRequest) (*AskResponse, error) {
	return c.ask(ctx, cfg, req)
}

// exeMatches reports whether a running daemon's executable identity is
// compatible with this client. Version strings cannot distinguish two dev
// builds ("dev" == "dev" across arbitrary code), so the daemon's recorded
// executable path must match the one this client would spawn; a mismatch
// restarts the daemon (cheap — one cold start) instead of silently serving
// stale code. Empty ping.ExePath (pre-field daemons) is accepted.
func (c DaemonClient) exeMatches(ping *DaemonPing) bool {
	if ping.ExePath == "" {
		return true
	}
	want := c.ExePath
	if want == "" {
		if exe, err := os.Executable(); err == nil {
			want = exe
		}
	}
	return want == "" || canonicalExe(ping.ExePath) == canonicalExe(want)
}

// canonicalExe normalizes an executable path for identity comparison:
// os.Executable resolves symlinks (macOS /tmp → /private/tmp) while a
// caller-configured ExePath may not, and that skew must not read as a
// different binary (it would restart-loop the daemon).
func canonicalExe(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func (c DaemonClient) ensure(ctx context.Context, cfg Config) error {
	ping, err := c.ping(ctx)
	if err == nil && ping.Version == c.Version && ping.ConfigDigest == ConfigDigest(cfg) && c.exeMatches(ping) {
		return nil
	}
	if err == nil {
		_, _ = c.call(ctx, "ghx.sidecar.Shutdown", map[string]any{})
		time.Sleep(150 * time.Millisecond)
	}
	if runtime.GOOS == "windows" {
		return errors.New("daemon IPC unsupported on Windows")
	}
	if err := c.spawn(); err != nil {
		return err
	}
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		ping, last = c.ping(ctx)
		if last == nil && ping.Version == c.Version && ping.ConfigDigest == ConfigDigest(cfg) && c.exeMatches(ping) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("daemon did not become healthy: %v", last)
}

func (c DaemonClient) ping(ctx context.Context) (*DaemonPing, error) {
	raw, err := c.call(ctx, "ghx.sidecar.Ping", map[string]any{})
	if err != nil {
		return nil, err
	}
	var ping DaemonPing
	data, _ := json.Marshal(raw)
	if err := json.Unmarshal(data, &ping); err != nil {
		return nil, err
	}
	if err := assertUnderRoot(ping.Socket); err != nil {
		return nil, err
	}
	if ping.PID > 0 && !processAlive(ping.PID) {
		return nil, fmt.Errorf("daemon pid %d is dead", ping.PID)
	}
	return &ping, nil
}

func (c DaemonClient) call(ctx context.Context, method string, params any) (any, error) {
	if err := os.MkdirAll(RuntimeDir(), 0o700); err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("unix", SocketPath(), time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(12 * time.Hour))
	req := rpcRequest{JSONRPC: "2.0", ID: "1", Method: method}
	if params != nil {
		req.Params, _ = json.Marshal(params)
	}
	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	respCh := make(chan struct {
		resp rpcResponse
		err  error
	}, 1)
	go func() {
		var resp rpcResponse
		err := json.NewDecoder(conn).Decode(&resp)
		respCh <- struct {
			resp rpcResponse
			err  error
		}{resp: resp, err: err}
	}()
	var resp rpcResponse
	select {
	case <-ctx.Done():
		_ = conn.Close()
		return nil, ctx.Err()
	case got := <-respCh:
		if got.err != nil {
			return nil, got.err
		}
		resp = got.resp
	}
	if resp.Error != nil {
		return nil, errors.New(resp.Error.Message)
	}
	return resp.Result, nil
}

func (c DaemonClient) spawn() error {
	exe := c.ExePath
	if exe == "" {
		var err error
		exe, err = os.Executable()
		if err != nil {
			return err
		}
	}
	// A `go test` binary cannot serve `sidecar daemon`: spawning it forks a
	// duplicate test run that lingers forever (leak observed 2026-07-06 —
	// every full-suite run left a `sidecar.test sidecar daemon --background`
	// process behind). Same rule as ResolveReportSinkExe.
	if strings.HasSuffix(exe, ".test") {
		return fmt.Errorf("refusing to spawn daemon from test binary %s", exe)
	}
	logPath := filepath.Join(RuntimeDir(), "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "sidecar", "daemon", "--background")
	cmd.Env = os.Environ()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return err
	}
	_ = cmd.Process.Release()
	_ = logFile.Close()
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// ReadDaemonMetadata reads runtime/daemon.json when present.
func ReadDaemonMetadata() (*DaemonMetadata, error) {
	data, err := os.ReadFile(MetadataPath())
	if err != nil {
		return nil, err
	}
	var meta DaemonMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// DaemonPIDFromMetadata is a small helper used by tests and diagnostics.
func DaemonPIDFromMetadata() (int, error) {
	meta, err := ReadDaemonMetadata()
	if err != nil {
		return 0, err
	}
	if meta.PID == 0 {
		return 0, errors.New("daemon metadata has no pid")
	}
	return meta.PID, nil
}
