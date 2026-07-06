package sidecar

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
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

func checkGHToken(_ context.Context) PreflightCheck {
	// os.Getenv handled inline to avoid an os import just for two lookups.
	for _, key := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := os.Getenv(key); v != "" {
			return PreflightCheck{Name: "gh-token", Passed: true, Message: key + " is present"}
		}
	}
	return PreflightCheck{
		Name:    "gh-token",
		Passed:  false,
		Message: "neither GH_TOKEN nor GITHUB_TOKEN is set — ghx calls will fail",
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
