package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// mcpServeCheckTimeout bounds the served-MCP probe. Generous enough for a
// cold `npx -y @gkoreli/ghx serve` start (package resolution + node boot) but
// bounded so a hung or wedged server fails the check instead of hanging the
// doctor.
const mcpServeCheckTimeout = 30 * time.Second

// ResolveMcpServeCmd returns the ghx MCP serve command line exactly as an MCP
// client would launch it, plus where it was resolved from. Resolution order:
//
//  1. GHX_MCP_SERVE_CMD — explicit override (full command line, e.g.
//     "/usr/local/bin/ghx serve"); also the escape hatch for MCP clients
//     other than Claude Code.
//  2. ~/.claude.json mcpServers.ghx — the founder-daily wiring (FRICTION
//     2026-08-22): what Claude Code actually spawns for the ghx MCP server.
//
// The error names the missing wiring so the doctor failure is immediately
// attributable (ADR-0028.1 ergonomics style).
func ResolveMcpServeCmd() (cmdLine, source string, err error) {
	if cmdLine := strings.TrimSpace(os.Getenv("GHX_MCP_SERVE_CMD")); cmdLine != "" {
		return cmdLine, "GHX_MCP_SERVE_CMD", nil
	}
	home, herr := os.UserHomeDir()
	if herr != nil {
		return "", "claude config", fmt.Errorf("cannot resolve home directory: %v", herr)
	}
	if home == "" {
		return "", "claude config", fmt.Errorf("cannot resolve home directory: $HOME is empty")
	}
	claudeJSON := filepath.Join(home, ".claude.json")
	data, rerr := os.ReadFile(claudeJSON)
	if rerr != nil {
		return "", "claude config", fmt.Errorf("cannot read %s: %v", claudeJSON, rerr)
	}
	var parsed struct {
		McpServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if uerr := json.Unmarshal(data, &parsed); uerr != nil {
		return "", "claude config", fmt.Errorf("cannot parse %s: %v", claudeJSON, uerr)
	}
	srv, ok := parsed.McpServers["ghx"]
	if !ok || strings.TrimSpace(srv.Command) == "" {
		return "", "claude config", fmt.Errorf("no mcpServers.ghx entry in %s", claudeJSON)
	}
	parts := append([]string{strings.TrimSpace(srv.Command)}, srv.Args...)
	return strings.Join(parts, " "), "claude config (" + claudeJSON + " mcpServers.ghx)", nil
}

// CheckMcpServe validates the served-MCP surface — `ghx serve` exactly as an
// MCP client launches it (GHX_MCP_SERVE_CMD, else ~/.claude.json
// mcpServers.ghx). It spawns the resolved command, performs the MCP
// initialize handshake plus tools/list, and requires the single recon tool
// the default serve mode must expose (ADR-0019.3 D1). This is the
// founder-daily path `ghx ask` never exercises: when it is broken, MCP recon
// calls fail inside Claude Code even though every other doctor check passes
// (FRICTION 2026-08-22, probe scripts/mcp-recon-probe.mjs).
//
// Doctor-path-only, like CheckLiveTurn: it spawns a real (possibly
// network-resolving) process, so it is not part of the parallel preflight
// registry that eval runs share via RunPreflightForAgent.
func CheckMcpServe(ctx context.Context) PreflightCheck {
	const name = "mcp-serve"
	remediation := "The served-MCP surface is what Claude Code launches from ~/.claude.json mcpServers.ghx —\n" +
		"when it is broken, MCP recon calls fail there even though `ghx ask` works.\n" +
		"Fix: update ghx so the serve command serves the recon tool (`npm i -g @gkoreli/ghx`, or rebuild),\n" +
		"verify it by hand (`npx -y @gkoreli/ghx serve` should stay silent and alive), or re-wire the\n" +
		"mcpServers.ghx block from `ghx serve --print-mcp-config`. Point GHX_MCP_SERVE_CMD at a different\n" +
		"serve command to probe a non-default install."

	cmdLine, source, err := ResolveMcpServeCmd()
	if err != nil {
		return PreflightCheck{
			Name:    name,
			Passed:  false,
			Message: "cannot resolve the served-MCP command: " + err.Error(),
			Remediation: "Wire the ghx MCP server into Claude Code: paste the mcpServers block from\n" +
				"`ghx serve --print-mcp-config` into ~/.claude.json, or set GHX_MCP_SERVE_CMD to the\n" +
				"serve command line to probe (e.g. \"ghx serve\").",
		}
	}

	tctx, cancel := context.WithTimeout(ctx, mcpServeCheckTimeout)
	defer cancel()

	serveBin, serveArgs := SplitAgentCmd(cmdLine)
	c, err := client.NewStdioMCPClient(serveBin, nil, serveArgs...)
	if err != nil {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("serve command %q (via %s) failed to spawn: %v", cmdLine, source, err),
			Remediation: remediation,
		}
	}
	defer c.Close()

	init, err := c.Initialize(tctx, mcp.InitializeRequest{})
	if err != nil {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("serve command %q (via %s) did not complete the MCP initialize handshake: %v", cmdLine, source, err),
			Remediation: remediation,
		}
	}
	tools, err := c.ListTools(tctx, mcp.ListToolsRequest{})
	if err != nil {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("serve command %q (via %s, server %s %s) completed initialize but tools/list failed: %v", cmdLine, source, init.ServerInfo.Name, init.ServerInfo.Version, err),
			Remediation: remediation,
		}
	}
	names := make([]string, 0, len(tools.Tools))
	hasRecon := false
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
		if tl.Name == ReconToolName {
			hasRecon = true
		}
	}
	if !hasRecon {
		return PreflightCheck{
			Name:        name,
			Passed:      false,
			Message:     fmt.Sprintf("serve command %q (via %s, server %s %s) serves %s — no %q tool; MCP clients would get no recon entry point", cmdLine, source, init.ServerInfo.Name, init.ServerInfo.Version, toolListOrNone(names), ReconToolName),
			Remediation: remediation,
		}
	}
	return PreflightCheck{
		Name:    name,
		Passed:  true,
		Message: fmt.Sprintf("serve command %q (via %s, server %s %s) serves %s", cmdLine, source, init.ServerInfo.Name, init.ServerInfo.Version, toolListOrNone(names)),
	}
}

// toolListOrNone renders a tool-name list for check messages; an empty list
// must read as "no tools", not an empty string.
func toolListOrNone(names []string) string {
	if len(names) == 0 {
		return "no tools"
	}
	return "tools: " + strings.Join(names, ", ")
}
