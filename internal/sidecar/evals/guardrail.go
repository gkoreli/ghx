package evals

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Expensive-backend guardrail (ADR-0038).
//
// The sidecar thesis requires the cheap specialist to run on a deliberately
// chosen backend. Governance lives in three places: config (explicit
// selection), artifacts (AgentIdentity records what ran), and here — a
// preflight guardrail so a formal gate run can never silently burn an
// expensive/shared quota pool (2026-07-03/04 incident: 90-episode run
// executed via claude-agent-acp against the main agent's subscription).
const (
	// EnvAllowExpensiveBackend opts a specific run into an expensive backend.
	// The value must be "formal-run" so the override is a deliberate act,
	// recorded in the verdict notes, never a stale shell variable.
	EnvAllowExpensiveBackend = "GHX_EVAL_ALLOW_EXPENSIVE_BACKEND"

	// defaultExpensiveBackends lists command substrings treated as
	// expensive/shared-quota. Overridable via GHX_EVAL_EXPENSIVE_BACKENDS
	// (comma-separated) for operator-controlled reclassification.
	defaultExpensiveBackends = "claude-agent-acp,claude-acp,claude"
)

// GuardrailCheck reports whether the resolved agent command trips the
// expensive-backend list and whether the run explicitly opted in.
type GuardrailCheck struct {
	AgentCmd      string
	MatchedRule   string // which comma-separated entry matched; "" if none
	ExplicitOptIn bool
}

// CheckExpensiveBackend evaluates agentCmd against the configured
// expensive-backend list. Matching is case-insensitive substring on the
// command's base name and full string, so both "claude" and wrapper paths
// ending in claude-agent-acp are caught.
func CheckExpensiveBackend(agentCmd string) GuardrailCheck {
	g := GuardrailCheck{AgentCmd: agentCmd}
	list := os.Getenv("GHX_EVAL_EXPENSIVE_BACKENDS")
	if strings.TrimSpace(list) == "" {
		list = defaultExpensiveBackends
	}
	base := filepath.Base(strings.TrimSpace(agentCmd))
	lowerBase := strings.ToLower(base)
	lowerFull := strings.ToLower(agentCmd)
	for _, entry := range strings.Split(list, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if strings.Contains(lowerFull, entry) || lowerBase == entry {
			g.MatchedRule = entry
			break
		}
	}
	// Wrapper scripts hide the real backend behind an unrelated argv0
	// (e.g. scripts/eval-agent-acp.sh execs claude-agent-acp). If the name
	// did not match and the command is a readable script, inspect its
	// contents for the expensive-backend entries too.
	if g.MatchedRule == "" {
		if data, err := os.ReadFile(agentCmd); err == nil && len(data) < 1<<20 {
			content := strings.ToLower(string(data))
			for _, entry := range strings.Split(list, ",") {
				entry = strings.ToLower(strings.TrimSpace(entry))
				if entry != "" && strings.Contains(content, entry) {
					g.MatchedRule = entry + " (wrapper content)"
					break
				}
			}
		}
	}
	val := strings.TrimSpace(os.Getenv(EnvAllowExpensiveBackend))
	g.ExplicitOptIn = val == "formal-run"
	return g
}

// GuardrailError renders the fail-fast error for formal runs.
func (g GuardrailCheck) GuardrailError() error {
	if g.MatchedRule == "" || g.ExplicitOptIn {
		return nil
	}
	return fmt.Errorf(
		"expensive-backend guardrail: agent command %q matches expensive/shared-quota rule %q.\n"+
			"A formal gate run must not silently burn a shared quota pool (ADR-0038).\n"+
			"Fix: point GHX_EVAL_AGENT at a cheap/dedicated ACP backend, or set\n"+
			"%s=formal-run to record a deliberate override.",
		g.AgentCmd, g.MatchedRule, EnvAllowExpensiveBackend)
}
