package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckExpensiveBackendMatches(t *testing.T) {
	// Wrapper-content detection needs a readable script; build one in a temp
	// dir so the test does not depend on repo-relative cwd.
	wrapper := filepath.Join(t.TempDir(), "eval-agent-acp.sh")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec npx -y @agentclientprotocol/claude-agent-acp@0.55.0 \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		cmd      string
		wantRule string
	}{
		{"claude", "claude"},
		{"claude-agent-acp", "claude-agent-acp"},
		{"/tmp/x/claude-acp", "claude-acp"},
		{"/home/u/.npm/claude-agent-acp/bin/runner", "claude-agent-acp"},
		{wrapper, "claude-agent-acp (wrapper content)"},
		{"codex", ""},
		{"/usr/local/bin/codex", ""},
		{"./my-echo-agent.sh", ""},
	}
	for _, tc := range cases {
		g := CheckExpensiveBackend(tc.cmd)
		if g.MatchedRule != tc.wantRule {
			t.Errorf("CheckExpensiveBackend(%q).MatchedRule = %q, want %q", tc.cmd, g.MatchedRule, tc.wantRule)
		}
	}
}

func TestCheckExpensiveBackendExplicitOptIn(t *testing.T) {
	t.Setenv(EnvAllowExpensiveBackend, "formal-run")
	g := CheckExpensiveBackend("claude-agent-acp")
	if g.MatchedRule == "" {
		t.Fatal("expected match for claude-agent-acp")
	}
	if !g.ExplicitOptIn {
		t.Fatal("expected ExplicitOptIn=true with =formal-run")
	}
	if err := g.GuardrailError(); err != nil {
		t.Fatalf("GuardrailError with opt-in = %v, want nil", err)
	}
}

func TestCheckExpensiveBackendOptInValueMustBeDeliberate(t *testing.T) {
	for _, v := range []string{"1", "true", "yes", ""} {
		t.Setenv(EnvAllowExpensiveBackend, v)
		g := CheckExpensiveBackend("claude-agent-acp")
		if g.ExplicitOptIn {
			t.Errorf("ExplicitOptIn=true for value %q, want false", v)
		}
		if err := g.GuardrailError(); err == nil {
			t.Errorf("GuardrailError nil for value %q, want error", v)
		} else if !strings.Contains(err.Error(), EnvAllowExpensiveBackend) {
			t.Errorf("error should mention the override env var, got: %v", err)
		}
	}
}

func TestCheckExpensiveBackendCustomList(t *testing.T) {
	t.Setenv("GHX_EVAL_EXPENSIVE_BACKENDS", "gpt5-worker,o3-runner")
	g := CheckExpensiveBackend("/opt/o3-runner")
	if g.MatchedRule != "o3-runner" {
		t.Fatalf("MatchedRule = %q, want o3-runner", g.MatchedRule)
	}
	g = CheckExpensiveBackend("claude-agent-acp")
	if g.MatchedRule != "" {
		t.Fatalf("default list should not apply when custom list set; got %q", g.MatchedRule)
	}
}

func TestGuardrailErrorMentionsADR(t *testing.T) {
	g := CheckExpensiveBackend("claude")
	err := g.GuardrailError()
	if err == nil {
		t.Fatal("want error")
	}
	if !strings.Contains(err.Error(), "ADR-0038") {
		t.Errorf("error should cite ADR-0038, got: %v", err)
	}
}
