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
}

// PreflightResult aggregates all preflight checks.
type PreflightResult struct {
	Passed bool
	Checks []PreflightCheck
}

// RunPreflight executes all standard checks in parallel against the
// configured agent.
func RunPreflight(ctx context.Context) PreflightResult {
	return RunPreflightForAgent(ctx, "")
}

// RunPreflightForAgent runs the same checks but probes agentCmd instead of
// the configured agent when agentCmd is non-empty. Eval runs pass
// GHX_EVAL_AGENT here so the handshake checks the agent actually under
// test, not whatever ~/.ghx-sidecar/config.json points at.
func RunPreflightForAgent(ctx context.Context, agentCmd string) PreflightResult {
	type fn func(context.Context) PreflightCheck
	cfg := preflightAgentConfig(agentCmd)
	checks := []fn{checkGHToken, checkNetwork, checkGhxBinary, func(ctx context.Context) PreflightCheck {
		return checkACPAgent(ctx, cfg)
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
		return PreflightCheck{Name: "acp-handshake", Passed: false, Message: err.Error()}
	}
	return PreflightCheck{Name: "acp-handshake", Passed: true, Message: cfg.AgentCmd + " completed ACP initialize"}
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
