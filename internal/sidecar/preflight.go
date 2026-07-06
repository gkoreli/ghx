package sidecar

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/auth"
)

// PreflightCheck is the result of a single diagnostic probe.
type PreflightCheck struct {
	Name    string
	Passed  bool
	Message string
	// Remediation is concrete fix-it text printed under a failing check.
	// Failures that would otherwise degrade silently (e.g. a stale report-sink
	// binary) must fail loudly here instead of relying on a buried stderr line.
	Remediation string
}

// PreflightResult aggregates all preflight checks.
type PreflightResult struct {
	Passed bool
	Checks []PreflightCheck
}

// RunPreflight executes all standard checks in parallel against the
// configured agent. runningVersion is the version of the calling binary
// (cli.VERSION); the report-sink check compares the sink-serving binary
// against it.
func RunPreflight(ctx context.Context, runningVersion string) PreflightResult {
	return runPreflight(ctx, "", runningVersion)
}

// RunPreflightForAgent runs the same checks but probes agentCmd instead of
// the configured agent when agentCmd is non-empty. Eval runs pass
// GHX_EVAL_AGENT here so the handshake checks the agent actually under
// test, not whatever ~/.ghx/config.json points at. The running version is
// unknown under go test, so the report-sink check only verifies that the
// resolved sink binary exists and answers `version` (which still requires
// GHX_REPORT_SINK_EXE under go test — exactly the loud failure eval runs
// need instead of a silent text-fallback degradation).
func RunPreflightForAgent(ctx context.Context, agentCmd string) PreflightResult {
	return runPreflight(ctx, agentCmd, "")
}

func runPreflight(ctx context.Context, agentCmd, runningVersion string) PreflightResult {
	type fn func(context.Context) PreflightCheck
	cfg := preflightAgentConfig(agentCmd)
	checks := []fn{checkGHToken, checkNetwork, checkGhxBinary,
		func(ctx context.Context) PreflightCheck {
			return checkACPAgent(ctx, cfg)
		},
		func(ctx context.Context) PreflightCheck {
			return checkReportSinkVersion(ctx, runningVersion)
		}}

	tctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	results := make([]PreflightCheck, len(checks))
	var wg sync.WaitGroup
	for i, f := range checks {
		i, f := i, f
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = f(tctx)
		}()
	}
	wg.Wait()

	passed := true
	for _, r := range results {
		if !r.Passed {
			passed = false
		}
	}
	return PreflightResult{Passed: passed, Checks: results}
}

func preflightAgentConfig(agentCmd string) Config {
	cfg := LoadConfig()
	if agentCmd != "" {
		cfg.AgentCmd = agentCmd
	}
	return cfg
}

func checkACPAgent(ctx context.Context, cfg Config) PreflightCheck {
	if err := checkACPHandshake(ctx, cfg.AgentCmd, cfg.Cwd, cfg.Env, defaultHandshakeTimeout); err != nil {
		return PreflightCheck{
			Name:    "acp-handshake",
			Passed:  false,
			Message: fmt.Sprintf("agent command %q: %s", cfg.AgentCmd, err.Error()),
			Remediation: "Run `ghx sidecar config init --claude-acp` to configure the pinned Claude ACP adapter\n" +
				"(needs Node/npx and a Claude Code login), or set an ACP-capable agent in " + ConfigFilePath() + ".",
		}
	}
	return PreflightCheck{Name: "acp-handshake", Passed: true, Message: fmt.Sprintf("agent command %q completed ACP initialize", cfg.AgentCmd)}
}

// FormatPreflight returns a human-readable diagnostic block.
func FormatPreflight(r PreflightResult) string {
	var sb strings.Builder
	status := "PASS"
	if !r.Passed {
		status = "FAIL"
	}
	fmt.Fprintf(&sb, "Preflight %s\n", status)
	for _, c := range r.Checks {
		mark := "✓"
		if !c.Passed {
			mark = "✗"
		}
		fmt.Fprintf(&sb, "  %s %s: %s\n", mark, c.Name, c.Message)
		if !c.Passed && c.Remediation != "" {
			for _, line := range strings.Split(c.Remediation, "\n") {
				fmt.Fprintf(&sb, "      → %s\n", line)
			}
		}
	}
	return sb.String()
}

// checkGHToken resolves GitHub credentials exactly the way ghx's API client
// does — go-gh's TokenForHost, which reads GH_TOKEN/GITHUB_TOKEN env vars AND
// the gh CLI's stored login (hosts.yml / keychain). The old env-only sniff
// reported a false ✗ on machines where `gh auth login` was done and every ghx
// call worked fine (founder's work laptop, 2026-07-06).
func checkGHToken(_ context.Context) PreflightCheck {
	token, source := auth.TokenForHost("github.com")
	if token != "" {
		return PreflightCheck{Name: "gh-token", Passed: true, Message: "GitHub token resolved (source: " + source + ")"}
	}
	return PreflightCheck{
		Name:        "gh-token",
		Passed:      false,
		Message:     "no GitHub credentials found — ghx calls will fail",
		Remediation: "Run `gh auth login`, or set GH_TOKEN/GITHUB_TOKEN in the environment.",
	}
}

func checkNetwork(ctx context.Context) PreflightCheck {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://api.github.com", nil)
	if err != nil {
		return PreflightCheck{Name: "network", Passed: false, Message: "could not build request: " + err.Error()}
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return PreflightCheck{Name: "network", Passed: false, Message: "api.github.com unreachable: " + err.Error()}
	}
	resp.Body.Close()
	if resp.StatusCode < 500 {
		return PreflightCheck{Name: "network", Passed: true, Message: fmt.Sprintf("api.github.com reachable (HTTP %d)", resp.StatusCode)}
	}
	return PreflightCheck{Name: "network", Passed: false, Message: fmt.Sprintf("api.github.com returned HTTP %d", resp.StatusCode)}
}

// checkReportSinkVersion verifies that the ghx binary which will serve the
// session-scoped report-sink MCP server (`sidecar report-sink`, resolved by
// ResolveReportSinkExe — the exact binary a session would spawn) is the same
// version as the binary running preflight. A stale sink binary silently
// degrades the submit_report contract to the <ghx-report> text fallback
// (dogfood friction 2026-07-05 "report-sink depends on a current ghx
// binary"), so any mismatch or a binary that cannot report its version is a
// loud preflight failure with remediation, never just a stderr warning.
// runningVersion may be empty (unknown, e.g. under go test); then only the
// resolution and `version` invocation are checked, not equality.
func checkReportSinkVersion(ctx context.Context, runningVersion string) PreflightCheck {
	const name = "report-sink-version"
	remediation := "Without a working report sink, submit_report silently degrades to the <ghx-report> text fallback.\n" +
		"Fix: reinstall/update ghx so the running binary is current, or set GHX_REPORT_SINK_EXE to a\n" +
		"freshly built ghx of the same version (go build -o ghx ./cmd/ghx)."
	exe, source, err := ResolveReportSinkExe()
	if err != nil {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     "cannot resolve the report-sink binary: " + err.Error(),
			Remediation: remediation,
		}
	}
	out, err := exec.CommandContext(ctx, exe, "version").Output()
	if err != nil {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("report-sink binary %s (via %s) failed `version`: %v — likely a stale or broken install", exe, source, err),
			Remediation: remediation,
		}
	}
	sinkVersion := parseGhxVersion(string(out))
	if sinkVersion == "" {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("report-sink binary %s (via %s) printed unrecognized version output %q", exe, source, strings.TrimSpace(string(out))),
			Remediation: remediation,
		}
	}
	if runningVersion != "" && sinkVersion != runningVersion {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("report-sink binary %s (via %s) is version %s but this binary is %s — reports would silently degrade", exe, source, sinkVersion, runningVersion),
			Remediation: remediation,
		}
	}
	return PreflightCheck{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("report sink served by %s (via %s), version %s", exe, source, sinkVersion),
	}
}

// parseGhxVersion extracts the version from `ghx version` output ("ghx 2.1.13").
// Returns "" when the output is not in that shape.
func parseGhxVersion(out string) string {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) < 2 || fields[0] != "ghx" {
		return ""
	}
	return fields[1]
}

// liveTurnPrompt is the trivial one-shot prompt the --live doctor check sends.
// It asks for a single word and no tools, so a healthy agent completes fast; a
// broken one still exercises the full initialize → session/new → prompt path
// (where auth actually resolves — the acp-handshake check stops at initialize).
const liveTurnPrompt = "Reply with exactly the single word: ok. Do not use any tools."

// liveTurnTimeout bounds the live probe. Generous enough for a real model
// round-trip (including cold npx adapter start) but bounded so a hung auth or
// endpoint fails loudly instead of hanging the doctor.
const liveTurnTimeout = 90 * time.Second

// CheckLiveTurn runs a full real prompt turn through the configured agent and
// reports the outcome (ADR-0033 D4). Unlike acp-handshake (ACP initialize
// only), it creates a session and sends one prompt in a neutral temp workspace
// with stderr captured, so setups where initialize passes but a real turn dies
// — the exact shape of the founder's laptop failure, typically auth resolving
// lazily at prompt time — are diagnosed with the failing STAGE and the agent's
// stderr tail. It is slow and side-effecting (spawns the real agent), so it is
// opt-in via `ghx sidecar doctor --live`, never part of the parallel preflight.
func CheckLiveTurn(ctx context.Context, cfg Config) PreflightCheck {
	const name = "live-turn"
	remediation := "The agent completes ACP initialize but a real prompt turn fails. This is usually an\n" +
		"auth/endpoint problem that resolves only at prompt time (missing or wrong ANTHROPIC_*,\n" +
		"CLAUDE_CODE_USE_BEDROCK/_USE_VERTEX, or AWS/Vertex credentials), a proxy/CA issue, or an\n" +
		"adapter/Node version mismatch. Read the stderr tail above and the failing stage; if ghx runs\n" +
		"under the daemon, note the daemon inherited the FIRST caller's environment (ghx sidecar\n" +
		"daemon --stop, then re-run from a shell with the right env)."

	dir, err := os.MkdirTemp("", "ghx-doctor-live-")
	if err != nil {
		return PreflightCheck{Name: name, Passed: false, Message: "cannot create probe workspace: " + err.Error()}
	}
	defer os.RemoveAll(dir)
	stderrLog := filepath.Join(dir, AgentStderrLogName)

	tctx, cancel := context.WithTimeout(ctx, liveTurnTimeout)
	defer cancel()

	persona := BuildPersonaSystemPrompt()
	result, _, runErr := runTurnWithOptions(tctx, RunTurnOptions{
		AgentCmd:        cfg.AgentCmd,
		Prompt:          liveTurnPrompt,
		Cwd:             dir,
		Env:             cfg.Env,
		SessionMeta:     BuildSessionMeta(persona, "cheap", cfg.Model, false),
		AgentStderrPath: stderrLog,
		LivenessTimeout: liveTurnTimeout,
	})
	if runErr != nil {
		msg := fmt.Sprintf("agent %q failed at stage %q: %s", cfg.AgentCmd, liveTurnStage(runErr), runErr.Error())
		if tail := AgentStderrTail(stderrLog); tail != "" {
			msg += "\n    agent stderr (tail):\n" + indentLines(tail, "      ")
		}
		return PreflightCheck{Name: name, Passed: false, Message: msg, Remediation: remediation}
	}
	reply := strings.TrimSpace(result.FullText)
	if reply == "" {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("agent %q completed a turn but produced NO output — the silent-failure shape (ADR-0033)", cfg.AgentCmd),
			Remediation: remediation,
		}
	}
	if len(reply) > 80 {
		reply = reply[:80] + "…"
	}
	return PreflightCheck{Name: name, Passed: true, Message: fmt.Sprintf("agent %q completed a real prompt turn (replied %q)", cfg.AgentCmd, reply)}
}

// liveTurnStage extracts the failing ACP stage from a RunTurnWithOptions error
// whose message is prefixed by the stage ("acp initialize:", "acp new
// session:", "acp prompt:", …). Returns "spawn/setup" when no stage prefix is
// present.
func liveTurnStage(err error) string {
	msg := err.Error()
	for _, stage := range []string{"acp initialize", "acp new session", "acp load session", "acp prompt"} {
		if strings.Contains(msg, stage) {
			return stage
		}
	}
	return "spawn/setup"
}

// indentLines prefixes every line of s with prefix, for nested diagnostic
// blocks under a preflight check.
func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func checkGhxBinary(ctx context.Context) PreflightCheck {
	out, err := exec.CommandContext(ctx, "ghx", "version").Output()
	if err != nil {
		return PreflightCheck{
			Name:    "ghx-binary",
			Passed:  false,
			Message: "ghx binary not found on PATH — add ghx to PATH or install it",
		}
	}
	return PreflightCheck{Name: "ghx-binary", Passed: true, Message: "ghx found: " + strings.TrimSpace(string(out))}
}
