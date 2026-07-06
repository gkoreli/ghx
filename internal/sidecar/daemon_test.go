package sidecar_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func buildDaemonMockAgent(t *testing.T, replies []map[string]any) (agentCmd, promptLog, startsFile string) {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "mockagent")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/gkoreli/ghx/v2/internal/sidecar/evals/mockagent")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build mockagent: %v\n%s", err, out)
	}
	data, err := json.Marshal(replies)
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "script.json")
	if err := os.WriteFile(script, data, 0o644); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	promptLog = filepath.Join(dir, "prompts.log")
	startsFile = filepath.Join(dir, "starts")
	wrapper := filepath.Join(dir, "agent-wrapper.sh")
	body := fmt.Sprintf("#!/bin/sh\nn=0\nif [ -f %q ]; then n=$(cat %q); fi\nn=$((n+1))\nprintf '%%s' \"$n\" > %q\nMOCKAGENT_SCRIPT=%q MOCKAGENT_STATE=%q MOCKAGENT_PROMPT_LOG=%q exec %q\n",
		startsFile, startsFile, startsFile, script, state, promptLog, bin)
	if err := os.WriteFile(wrapper, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return wrapper, promptLog, startsFile
}

func shortGHXHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "ghx-daemon-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	t.Setenv("GHX_HOME", home)
	return home
}

func daemonReport(answer string) string {
	return fmt.Sprintf(`<ghx-report>{"answer":%q,"verified":[{"summary":"ok","evidence":"mock"}],"relevantFiles":[{"path":"a.go","reason":"mock"}],"commandsRun":["ghx mock"]}</ghx-report>`, answer)
}

func startTestDaemon(t *testing.T, version string, cfg sidecar.Config) {
	t.Helper()
	srv, err := sidecar.NewDaemonServer(version, cfg)
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(context.Background()) }()
	t.Cleanup(func() {
		srv.Shutdown()
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("daemon serve: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("daemon did not stop")
		}
	})
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", sidecar.SocketPath(), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("daemon socket did not become ready")
}

func TestDaemonSocketLifecycle(t *testing.T) {
	home := shortGHXHome(t)
	cfg := sidecar.Config{AgentCmd: "mock", SessionsDir: filepath.Join(home, "sessions")}
	startTestDaemon(t, "dev", cfg)
	if _, err := os.Stat(sidecar.SocketPath()); err != nil {
		t.Fatalf("socket not created: %v", err)
	}
	meta, err := sidecar.ReadDaemonMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if meta.Version != "dev" || meta.Socket != sidecar.SocketPath() {
		t.Fatalf("bad metadata: %+v", meta)
	}
	if err := sidecar.ShutdownDaemon(context.Background(), "dev", cfg); err != nil {
		t.Fatalf("shutdown rpc: %v", err)
	}
}

func TestDaemonAskReusesWarmMockAgent(t *testing.T) {
	home := shortGHXHome(t)
	agent, promptLog, starts := buildDaemonMockAgent(t, []map[string]any{
		{"text": daemonReport("first")},
		{"text": daemonReport("second")},
	})
	cfg := sidecar.Config{AgentCmd: agent, SessionsDir: filepath.Join(home, "sessions")}
	startTestDaemon(t, "dev", cfg)

	for _, q := range []string{"first?", "second?"} {
		report, turn, usedDaemon, err := sidecar.AskViaDaemon(context.Background(), "dev", cfg, sidecar.AskRequest{
			Session:  "warm",
			Repo:     "owner/repo",
			Question: q,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !usedDaemon || report == nil || turn == nil || turn.Artifacts.SessionDir == "" {
			t.Fatalf("bad daemon ask: used=%v report=%+v turn=%+v", usedDaemon, report, turn)
		}
	}
	if data, err := os.ReadFile(starts); err != nil || strings.TrimSpace(string(data)) != "1" {
		t.Fatalf("agent starts = %q, %v; want 1", data, err)
	}
	logData, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(logData), "PROMPT "); got != 2 {
		t.Fatalf("prompt count = %d\n%s", got, logData)
	}
	if !strings.Contains(string(logData), "LOAD mock-sess-1") {
		t.Fatalf("second warm ask did not reload existing ACP session:\n%s", logData)
	}

	// ADR-0022.1: the daemon ask streamed realtime activity to the session's
	// live.jsonl — turn boundaries from the runtime, per-update events from the
	// warm worker's ACP client — every line valid NDJSON.
	livePath := sidecar.LiveLogPath(cfg.SessionsDir, "warm")
	liveData, err := os.ReadFile(livePath)
	if err != nil {
		t.Fatalf("live.jsonl missing after daemon ask: %v", err)
	}
	counts := map[string]int{}
	completedOK := 0
	for i, line := range strings.Split(strings.TrimSpace(string(liveData)), "\n") {
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("live.jsonl line %d is not valid JSON: %v\n%s", i+1, err, line)
		}
		event, _ := ev["event"].(string)
		if event == "" || ev["ts"] == "" {
			t.Fatalf("live.jsonl line %d missing ts/event: %s", i+1, line)
		}
		counts[event]++
		if event == "turn.completed" && ev["ok"] == true {
			completedOK++
		}
	}
	if counts["turn.started"] == 0 {
		t.Fatalf("live.jsonl has no turn.started line:\n%s", liveData)
	}
	if counts["text"] == 0 && counts["tool.call"] == 0 {
		t.Fatalf("live.jsonl has no text or tool.call activity:\n%s", liveData)
	}
	if completedOK == 0 {
		t.Fatalf("live.jsonl has no turn.completed with ok=true:\n%s", liveData)
	}
}

func TestAskViaDaemonFallsBackWhenSpawnFails(t *testing.T) {
	home := shortGHXHome(t)
	agent, _, _ := buildDaemonMockAgent(t, []map[string]any{{"text": daemonReport("fallback")}})
	cfg := sidecar.Config{AgentCmd: agent, SessionsDir: filepath.Join(home, "sessions")}
	report, _, usedDaemon, err := sidecar.AskViaDaemon(context.Background(), "dev", cfg, sidecar.AskRequest{
		Session:  "fallback",
		Repo:     "owner/repo",
		Question: "fallback?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if usedDaemon {
		t.Fatal("used daemon from go test process; want daemonless fallback")
	}
	if report == nil || report.Answer != "fallback" {
		t.Fatalf("report = %+v", report)
	}
}

func TestVersionMismatchRestartsDaemon(t *testing.T) {
	home := shortGHXHome(t)
	agent, _, starts := buildDaemonMockAgent(t, []map[string]any{{"text": daemonReport("restarted")}})
	cfg := sidecar.Config{AgentCmd: agent, SessionsDir: filepath.Join(home, "sessions")}
	if err := sidecar.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	ghx := filepath.Join(t.TempDir(), "ghx")
	cmd := exec.Command("go", "build", "-o", ghx, "./cmd/ghx")
	cmd.Env = os.Environ()
	cmd.Dir = filepath.Join("..", "..")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build ghx: %v\n%s", err, out)
	}
	startTestDaemon(t, "old", cfg)

	client := sidecar.DaemonClient{Version: "dev", ExePath: ghx}
	resp, err := client.Ask(context.Background(), cfg, sidecar.AskRequest{
		Session:  "upgrade",
		Repo:     "owner/repo",
		Question: "upgrade?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Report == nil || resp.Report.Answer != "restarted" {
		t.Fatalf("response = %+v", resp)
	}
	meta, err := sidecar.ReadDaemonMetadata()
	if err != nil {
		t.Fatal(err)
	}
	if meta.Version != "dev" {
		t.Fatalf("daemon version = %q, want dev", meta.Version)
	}
	if data, err := os.ReadFile(starts); err != nil || strings.TrimSpace(string(data)) != "1" {
		t.Fatalf("agent starts = %q, %v; want restarted daemon to serve ask once", data, err)
	}
	_ = sidecar.ShutdownDaemon(context.Background(), "dev", cfg)
}
