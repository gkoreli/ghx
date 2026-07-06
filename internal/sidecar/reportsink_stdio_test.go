package sidecar_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestReportSinkStdioEndToEnd proves the PRODUCTION transport: it builds the
// ghx binary, spawns `ghx sidecar report-sink --out <path>` exactly as the ACP
// adapter would, and speaks MCP JSON-RPC to it over stdio — invalid submission
// → tool error, valid submission → acceptance + sink file (ADR-0021 D1). This
// covers the hidden CLI command + os.Executable wiring that the in-process test
// (TestReportSinkServer_MCPRoundTrip) cannot. No live agent, no tokens.
func TestReportSinkStdioEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping binary-build stdio e2e in -short mode")
	}
	bin := buildGhxBinary(t)
	dir := t.TempDir()
	sink := filepath.Join(dir, "report.json")

	c, err := client.NewStdioMCPClient(bin, nil, "sidecar", "report-sink", "--out", sink)
	if err != nil {
		t.Fatalf("spawn report-sink: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != sidecar.SubmitReportToolName {
		t.Fatalf("tools = %+v, want exactly submit_report", tools.Tools)
	}

	// Invalid: missing answer entirely.
	var badReq mcp.CallToolRequest
	badReq.Params.Name = sidecar.SubmitReportToolName
	badReq.Params.Arguments = map[string]any{"verified": []any{}}
	badRes, err := c.CallTool(ctx, badReq)
	if err != nil {
		t.Fatalf("invalid call transport error: %v", err)
	}
	if !badRes.IsError {
		t.Fatal("invalid submission over stdio should be a tool error")
	}
	if txt := textOf(badRes); !strings.Contains(txt, "answer") {
		t.Fatalf("invalid tool error = %q, want it to name the empty answer", txt)
	}

	// Evidence-less (ADR-0027 D4): an answer-only report is a hypothesis and
	// must be rejected in-band with field-level errors naming all three gaps.
	var noEvReq mcp.CallToolRequest
	noEvReq.Params.Name = sidecar.SubmitReportToolName
	noEvReq.Params.Arguments = map[string]any{"answer": "answer without any evidence"}
	noEvRes, err := c.CallTool(ctx, noEvReq)
	if err != nil {
		t.Fatalf("evidence-less call transport error: %v", err)
	}
	if !noEvRes.IsError {
		t.Fatal("evidence-less non-BLOCKED submission should be a tool error (ADR-0027 D4)")
	}
	for _, field := range []string{"verified", "relevantFiles", "commandsRun"} {
		if txt := textOf(noEvRes); !strings.Contains(txt, field) {
			t.Fatalf("evidence error = %q, want it to name %q", txt, field)
		}
	}

	// Valid: answer plus the required evidence trio.
	var okReq mcp.CallToolRequest
	okReq.Params.Name = sidecar.SubmitReportToolName
	okReq.Params.Arguments = map[string]any{
		"answer":        "stdio path works",
		"verified":      []any{map[string]any{"summary": "sink accepts evidence-carrying reports", "evidence": "this test's round-trip"}},
		"relevantFiles": []any{map[string]any{"path": "internal/sidecar/reportsink.go", "reason": "sink server"}},
		"commandsRun":   []any{"ghx version"},
	}
	okRes, err := c.CallTool(ctx, okReq)
	if err != nil {
		t.Fatalf("valid call: %v", err)
	}
	if okRes.IsError {
		t.Fatalf("valid submission errored: %s", textOf(okRes))
	}
	if !strings.Contains(textOf(okRes), "accepted") {
		t.Fatalf("acceptance text = %q", textOf(okRes))
	}

	got, err := sidecar.ReadSinkReport(sink)
	if err != nil || got == nil || got.Answer != "stdio path works" {
		t.Fatalf("sink report = %v, err = %v", got, err)
	}
}

func buildGhxBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "ghx-test-bin")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/gkoreli/ghx/v2/cmd/ghx")
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ghx: %v\n%s", err, out)
	}
	return bin
}

func textOf(res *mcp.CallToolResult) string {
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := mcp.AsTextContent(c); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}
