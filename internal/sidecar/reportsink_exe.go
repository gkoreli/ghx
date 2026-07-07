package sidecar

import (
	"fmt"
	"os"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// ResolveReportSinkExe returns the ghx executable that will serve the
// session-scoped report-sink MCP server (`sidecar report-sink`), plus a human
// label for where the decision came from. This is the single resolution
// authority — reportSinkMcpServers (the ACP wiring) and the doctor
// report-sink-version check both use it, so what doctor diagnoses is exactly
// what a session would spawn. Order: GHX_REPORT_SINK_EXE when set, else the
// current executable. Errors cover the unusable cases: os.Executable failure
// and running under `go test` (the test binary cannot serve `sidecar
// report-sink`).
func ResolveReportSinkExe() (exe string, source string, err error) {
	if exe := os.Getenv("GHX_REPORT_SINK_EXE"); exe != "" {
		return exe, "GHX_REPORT_SINK_EXE", nil
	}
	exe, resolveErr := os.Executable()
	if resolveErr != nil || exe == "" {
		return "", "current executable", fmt.Errorf("cannot resolve executable: %v", resolveErr)
	}
	if strings.HasSuffix(exe, ".test") {
		return exe, "current executable", fmt.Errorf("running under go test (%s); set GHX_REPORT_SINK_EXE to a built ghx binary", exe)
	}
	return exe, "current executable", nil
}

// reportSinkMcpServers returns the ACP McpServer list to register for a turn.
// When a report-sink path is set it registers the ghx-report-sink stdio server,
// pointing the adapter at this same executable (os.Executable), or at
// GHX_REPORT_SINK_EXE when set. If no usable executable can be resolved the
// list is empty and the runtime falls back to the <ghx-report> text path
// (ADR-0021 D3) — the feature degrades, it does not break.
//
// GHX_REPORT_SINK_EXE exists because os.Executable is only correct when the
// runtime runs inside a real ghx binary. Under `go test` (live eval episodes,
// tags=agent_e2e) it resolves to the compiled test binary, which cannot serve
// `sidecar report-sink` — and the PATH ghx may be an older release without the
// command. Eval runs must build a fresh ghx and point this env var at it, or
// live episodes silently degrade to the text fallback and never exercise the
// submit_report contract.
func reportSinkMcpServers(sinkPath string) []acp.McpServer {
	if sinkPath == "" {
		return []acp.McpServer{}
	}
	exe, _, err := ResolveReportSinkExe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: report-sink disabled (%v)\n", err)
		return []acp.McpServer{}
	}
	return []acp.McpServer{{
		Stdio: &acp.McpServerStdio{
			Name:    ReportSinkServerName,
			Command: exe,
			Args:    []string{"sidecar", "report-sink", "--out", sinkPath},
			Env:     []acp.EnvVariable{},
		},
	}}
}
