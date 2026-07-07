package sidecar

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDaemonExeIdentity pins the stale-dev-daemon fix (2026-07-06 live
// incident: two different "dev" builds passed the version handshake, and a
// 3-hour-old daemon silently served a fresh client). A daemon reporting a
// different executable is unhealthy → restart; empty (pre-field) and
// symlink-skewed paths are compatible.
func TestDaemonExeIdentity(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	c := DaemonClient{}
	if !c.exeMatches(&DaemonPing{ExePath: ""}) {
		t.Error("pre-field daemon (empty exePath) must be accepted")
	}
	if !c.exeMatches(&DaemonPing{ExePath: self}) {
		t.Error("same executable must match")
	}
	if c.exeMatches(&DaemonPing{ExePath: "/tmp/other-ghx-binary"}) {
		t.Error("different executable must NOT match")
	}
	pinned := DaemonClient{ExePath: "/tmp/pinned-ghx"}
	if pinned.exeMatches(&DaemonPing{ExePath: self}) {
		t.Error("pinned client exe vs different daemon exe must NOT match")
	}
}

func testDaemonServer(t *testing.T) *DaemonServer {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "ghx-daemon-internal-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("GHX_HOME", home)
	cfg := Config{AgentCmd: "mock", SessionsDir: filepath.Join(home, "sessions")}
	s, err := NewDaemonServer("dev", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func daemonInternalReport(answer string) string {
	return `<ghx-report>{"answer":"` + answer + `","verified":[{"summary":"ok","evidence":"mock"}],"relevantFiles":[{"path":"a.go","reason":"mock"}],"commandsRun":["ghx mock"]}</ghx-report>`
}

func sendPipeRPC(t *testing.T, client net.Conn, req rpcRequest) rpcResponse {
	t.Helper()
	if err := json.NewEncoder(client).Encode(req); err != nil {
		t.Fatal(err)
	}
	var resp rpcResponse
	if err := json.NewDecoder(client).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestDaemonRecoversPanicPerRequest(t *testing.T) {
	s := testDaemonServer(t)
	s.runnerFor = func(string, Config) TurnRunner {
		return func(context.Context, RunTurnOptions) (TurnResult, string, error) {
			panic("boom")
		}
	}
	server, client := net.Pipe()
	defer client.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.handleConn(context.Background(), server)
	}()

	params, _ := json.Marshal(AskRequest{Session: "panic", Repo: "owner/repo", Question: "panic?"})
	resp := sendPipeRPC(t, client, rpcRequest{JSONRPC: "2.0", ID: "panic", Method: "ghx.sidecar.Ask", Params: params})
	if resp.Error == nil {
		t.Fatalf("panic response error = nil, response = %+v", resp)
	}
	if resp.Error.Code != -32603 || !strings.Contains(resp.Error.Message, "internal daemon panic") {
		t.Fatalf("panic response error = %+v", resp.Error)
	}

	resp = sendPipeRPC(t, client, rpcRequest{JSONRPC: "2.0", ID: "ping", Method: "ghx.sidecar.Ping"})
	if resp.Error != nil || resp.Result == nil {
		t.Fatalf("daemon did not serve after recovered panic: %+v", resp)
	}
	_ = client.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("daemon connection did not close")
	}
}

func TestDaemonCancelsTurnWhenClientDisconnects(t *testing.T) {
	s := testDaemonServer(t)
	started := make(chan struct{})
	cancelled := make(chan error, 1)
	s.runnerFor = func(string, Config) TurnRunner {
		return func(ctx context.Context, _ RunTurnOptions) (TurnResult, string, error) {
			close(started)
			<-ctx.Done()
			cancelled <- ctx.Err()
			return TurnResult{FullText: daemonInternalReport("cancelled")}, "sess", ctx.Err()
		}
	}
	server, client := net.Pipe()
	go s.handleConn(context.Background(), server)

	params, _ := json.Marshal(AskRequest{Session: "cancel", Repo: "owner/repo", Question: "cancel?"})
	if err := json.NewEncoder(client).Encode(rpcRequest{JSONRPC: "2.0", ID: "cancel", Method: "ghx.sidecar.Ask", Params: params}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	_ = client.Close()
	select {
	case err := <-cancelled:
		if err == nil {
			t.Fatal("runner context error = nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client disconnect did not cancel running turn")
	}
}

func TestDaemonShutdownDrainsActiveAsk(t *testing.T) {
	oldDrain := daemonDrainTimeout
	daemonDrainTimeout = 2 * time.Second
	t.Cleanup(func() { daemonDrainTimeout = oldDrain })

	s := testDaemonServer(t)
	started := make(chan struct{})
	release := make(chan struct{})
	s.runnerFor = func(string, Config) TurnRunner {
		return func(ctx context.Context, _ RunTurnOptions) (TurnResult, string, error) {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return TurnResult{}, "", ctx.Err()
			}
			return TurnResult{FullText: daemonInternalReport("drained")}, "sess", nil
		}
	}
	errCh := make(chan error, 1)
	go func() { errCh <- s.Serve(context.Background()) }()
	t.Cleanup(func() { s.Shutdown() })

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", SocketPath(), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	conn, err := net.DialTimeout("unix", SocketPath(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	respCh := make(chan rpcResponse, 1)
	errRespCh := make(chan error, 1)
	go func() {
		params, _ := json.Marshal(AskRequest{Session: "drain", Repo: "owner/repo", Question: "drain?"})
		if err := json.NewEncoder(conn).Encode(rpcRequest{JSONRPC: "2.0", ID: "drain", Method: "ghx.sidecar.Ask", Params: params}); err != nil {
			errRespCh <- err
			return
		}
		var resp rpcResponse
		if err := json.NewDecoder(conn).Decode(&resp); err != nil {
			errRespCh <- err
			return
		}
		respCh <- resp
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("runner did not start")
	}
	s.Shutdown()
	select {
	case err := <-errCh:
		t.Fatalf("Serve returned before active ask drained: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case resp := <-respCh:
		if resp.Error != nil {
			t.Fatalf("active ask failed during graceful drain: %+v", resp.Error)
		}
	case err := <-errRespCh:
		t.Fatalf("active ask RPC failed during graceful drain: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("active ask did not complete after release")
	}
	_ = conn.Close()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Serve returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after drained ask completed")
	}
}
