package sidecar_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TestCheckMcpServeServesReconOverStdio proves the check against the real
// production surface: it builds the ghx binary and points
// GHX_MCP_SERVE_CMD at `<bin> serve` — exactly the command line an MCP client
// launches from ~/.claude.json mcpServers.ghx — then asserts CheckMcpServe
// completes initialize + tools/list over stdio and finds the recon tool
// (FRICTION 2026-08-22: the served-MCP path `ghx ask` never exercises).
// No live agent, no tokens.
func TestCheckMcpServeServesReconOverStdio(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-build served-MCP probe in -short mode")
	}
	bin := buildGhxBinary(t)
	t.Setenv("GHX_MCP_SERVE_CMD", bin+" serve")

	check := sidecar.CheckMcpServe(context.Background())
	if !check.Passed {
		t.Fatalf("mcp-serve check failed: %s\nremediation: %s", check.Message, check.Remediation)
	}
	if !strings.Contains(check.Message, "tools: recon") {
		t.Fatalf("message = %q, want it to list the recon tool", check.Message)
	}
	if check.Name != "mcp-serve" {
		t.Fatalf("check name = %q, want mcp-serve", check.Name)
	}
}

// TestCheckMcpServeFailsWithoutReconTool pins the failure half: a serve
// command that speaks MCP but serves no recon tool must fail with a message
// naming both the missing tool and the resolved source, plus remediation.
func TestCheckMcpServeFailsWithoutReconTool(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-build served-MCP probe in -short mode")
	}
	bin := buildGhxBinary(t)
	t.Setenv("GHX_MCP_SERVE_CMD", bin+" serve --direct")

	check := sidecar.CheckMcpServe(context.Background())
	if check.Passed {
		t.Fatalf("--direct serves seven tools but no recon; check must fail, got: %s", check.Message)
	}
	// The failure must show the served tool list while naming the missing
	// recon entry point.
	if !strings.Contains(check.Message, `"recon"`) || !strings.Contains(check.Message, "no ") || !strings.Contains(check.Message, "tool") {
		t.Fatalf("message = %q, want it to name the missing %q tool", check.Message, sidecar.ReconToolName)
	}
	for _, want := range []string{bin, "GHX_MCP_SERVE_CMD"} {
		if !strings.Contains(check.Message, want) {
			t.Fatalf("message = %q, want it to name %q (attribution)", check.Message, want)
		}
	}
	if check.Remediation == "" {
		t.Fatal("failing mcp-serve check must carry remediation text")
	}
}

func TestResolveMcpServeCmdPrefersEnvOverride(t *testing.T) {
	t.Setenv("GHX_MCP_SERVE_CMD", "ghx serve --direct")
	cmdLine, source, err := sidecar.ResolveMcpServeCmd()
	if err != nil {
		t.Fatalf("ResolveMcpServeCmd with env override: %v", err)
	}
	if cmdLine != "ghx serve --direct" || source != "GHX_MCP_SERVE_CMD" {
		t.Fatalf("got (%q, %q), want env override honored verbatim", cmdLine, source)
	}
}

func TestResolveMcpServeCmdReadsClaudeConfig(t *testing.T) {
	dir := t.TempDir()
	claudeJSON := `{"mcpServers":{"ghx":{"command":"npx","args":["-y","@gkoreli/ghx","serve"]}}}`
	if err := os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(claudeJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GHX_MCP_SERVE_CMD", "")
	t.Setenv("HOME", dir)

	cmdLine, source, err := sidecar.ResolveMcpServeCmd()
	if err != nil {
		t.Fatalf("ResolveMcpServeCmd from claude config: %v", err)
	}
	if cmdLine != "npx -y @gkoreli/ghx serve" {
		t.Fatalf("cmdLine = %q, want the joined mcpServers.ghx argv", cmdLine)
	}
	if !strings.Contains(source, "mcpServers.ghx") || !strings.Contains(source, ".claude.json") {
		t.Fatalf("source = %q, want attribution to the claude config entry", source)
	}
}

func TestResolveMcpServeCmdErrorsNameTheFix(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GHX_MCP_SERVE_CMD", "")
	t.Setenv("HOME", dir)

	_, _, err := sidecar.ResolveMcpServeCmd()
	if err == nil {
		t.Fatal("ResolveMcpServeCmd succeeded with neither env nor claude wiring, want error")
	}
	if !strings.Contains(err.Error(), "no mcpServers.ghx entry") &&
		!strings.Contains(err.Error(), filepath.Join(dir, ".claude.json")) {
		t.Fatalf("error = %v, want it to name the missing ~/.claude.json mcpServers.ghx wiring", err)
	}

	// Malformed JSON must fail loudly too, not silently skip the surface.
	if err := os.WriteFile(filepath.Join(dir, ".claude.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, _, err = sidecar.ResolveMcpServeCmd()
	if err == nil || !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("error = %v, want a parse-failure naming the config file", err)
	}
}
