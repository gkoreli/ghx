package sidecar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// validReportArgs is a schema-valid submit_report payload.
func validReportArgs() map[string]any {
	return map[string]any{
		"answer":        "The router lives in internal/http/router.go.",
		"verified":      []any{map[string]any{"summary": "router registered in NewServer", "evidence": "router.go:42"}},
		"inferred":      []any{},
		"unverified":    []any{},
		"relevantFiles": []any{map[string]any{"path": "internal/http/router.go", "reason": "defines routes"}},
		"evidence":      []any{map[string]any{"source": "ghx read", "summary": "route table"}},
		"backendsUsed":  []any{"remote"},
		"commandsRun":   []any{"ghx read owner/repo internal/http/router.go"},
		"uncertainty":   []any{},
		"nextReads":     []any{},
	}
}

func TestDecodeReportStrict_Valid(t *testing.T) {
	data, _ := json.Marshal(validReportArgs())
	r, err := DecodeReportStrict(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Answer == "" || len(r.Verified) != 1 || r.Verified[0].Evidence != "router.go:42" {
		t.Fatalf("decoded report missing fields: %+v", r)
	}
}

func TestDecodeReportStrict_UnknownField(t *testing.T) {
	args := validReportArgs()
	args["answr"] = "typo field" // unknown top-level field
	data, _ := json.Marshal(args)
	_, err := DecodeReportStrict(data)
	if err == nil || !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), "answr") {
		t.Fatalf("want unknown-field error naming answr, got %v", err)
	}
}

func TestDecodeReportStrict_NestedUnknownField(t *testing.T) {
	args := validReportArgs()
	args["verified"] = []any{map[string]any{"summary": "x", "evidence": "y", "confidence": "high"}}
	data, _ := json.Marshal(args)
	_, err := DecodeReportStrict(data)
	if err == nil || !strings.Contains(err.Error(), "unknown field") || !strings.Contains(err.Error(), "confidence") {
		t.Fatalf("want nested unknown-field error naming confidence, got %v", err)
	}
}

func TestDecodeReportStrict_WrongShape(t *testing.T) {
	args := validReportArgs()
	args["verified"] = "a bare string where an array of claim objects is required"
	data, _ := json.Marshal(args)
	_, err := DecodeReportStrict(data)
	if err == nil || !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("want type-mismatch error, got %v", err)
	}
}

func TestDecodeReportStrict_EmptyAnswer(t *testing.T) {
	args := validReportArgs()
	args["answer"] = ""
	data, _ := json.Marshal(args)
	_, err := DecodeReportStrict(data)
	if err == nil || !strings.Contains(err.Error(), "answer") {
		t.Fatalf("want empty-answer error, got %v", err)
	}
}

func TestDecodeReportStrict_NoPayload(t *testing.T) {
	for _, in := range []string{"", "   ", "null"} {
		if _, err := DecodeReportStrict([]byte(in)); err == nil {
			t.Fatalf("input %q: want error for empty payload", in)
		}
	}
}

func TestDecodeReportStrict_TrailingData(t *testing.T) {
	data := []byte(`{"answer":"ok"} {"answer":"second"}`)
	_, err := DecodeReportStrict(data)
	if err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("want trailing-data error, got %v", err)
	}
}

func TestReportInputSchema_DerivedFromType(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(reportInputSchema(), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("top-level type = %v, want object", schema["type"])
	}
	if schema["additionalProperties"] != false {
		t.Fatalf("additionalProperties = %v, want false", schema["additionalProperties"])
	}
	req, _ := schema["required"].([]any)
	if len(req) != 1 || req[0] != "answer" {
		t.Fatalf("required = %v, want [answer]", schema["required"])
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("missing properties")
	}
	// Property names must match the Report json tags exactly (single source of truth).
	for _, want := range []string{"answer", "verified", "inferred", "unverified", "relevantFiles", "evidence", "backendsUsed", "commandsRun", "uncertainty", "nextReads"} {
		if _, ok := props[want]; !ok {
			t.Fatalf("schema missing property %q", want)
		}
	}
	// verified must be an array of objects with summary/evidence.
	verified, _ := props["verified"].(map[string]any)
	if verified["type"] != "array" {
		t.Fatalf("verified type = %v, want array", verified["type"])
	}
	items, _ := verified["items"].(map[string]any)
	iprops, _ := items["properties"].(map[string]any)
	if _, ok := iprops["summary"]; !ok {
		t.Fatal("verified items missing summary property")
	}
}

func TestWriteReadSinkReport_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")
	want := &Report{Answer: "hello", BackendsUsed: []string{"remote"}}
	if err := writeSinkReport(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadSinkReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Answer != "hello" || len(got.BackendsUsed) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestReadSinkReport_AbsentIsNilNil(t *testing.T) {
	got, err := ReadSinkReport(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || got != nil {
		t.Fatalf("absent sink: got (%v, %v), want (nil, nil)", got, err)
	}
}

// TestReportSinkServer_MCPRoundTrip proves the closed loop over the real MCP
// protocol (in-process transport): an invalid submission fails with the exact
// validation error and writes nothing; a subsequent valid submission is
// accepted and persisted (ADR-0021 D1). No process spawn, no tokens.
func TestReportSinkServer_MCPRoundTrip(t *testing.T) {
	dir := t.TempDir()
	sink := filepath.Join(dir, "report.json")
	srv := NewReportSinkServer(sink)

	c, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Initialize(ctx, mcp.InitializeRequest{}); err != nil {
		t.Fatal(err)
	}

	// tools/list advertises exactly submit_report.
	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != SubmitReportToolName {
		t.Fatalf("tools = %+v, want exactly [%s]", tools.Tools, SubmitReportToolName)
	}

	// Invalid submission: unknown field. Expect a tool error with the reason.
	invalid := validReportArgs()
	invalid["bogus"] = 123
	var invReq mcp.CallToolRequest
	invReq.Params.Name = SubmitReportToolName
	invReq.Params.Arguments = invalid
	invRes, err := c.CallTool(ctx, invReq)
	if err != nil {
		t.Fatalf("transport error on invalid call: %v", err)
	}
	if !invRes.IsError {
		t.Fatal("invalid submission should be a tool error")
	}
	invText := toolResultText(t, invRes)
	if !strings.Contains(invText, "unknown field") || !strings.Contains(invText, "bogus") {
		t.Fatalf("invalid tool error = %q, want unknown-field/bogus", invText)
	}
	t.Logf("captured invalid-submission tool error: %s", invText)
	if _, err := os.Stat(sink); !os.IsNotExist(err) {
		t.Fatalf("sink must not be written on invalid submission (stat err=%v)", err)
	}

	// Valid submission: accepted + persisted.
	var okReq mcp.CallToolRequest
	okReq.Params.Name = SubmitReportToolName
	okReq.Params.Arguments = validReportArgs()
	okRes, err := c.CallTool(ctx, okReq)
	if err != nil {
		t.Fatal(err)
	}
	if okRes.IsError {
		t.Fatalf("valid submission unexpectedly errored: %s", toolResultText(t, okRes))
	}
	okText := toolResultText(t, okRes)
	if !strings.Contains(okText, "accepted") {
		t.Fatalf("acceptance text = %q, want an acceptance message", okText)
	}
	t.Logf("captured acceptance result: %s", okText)

	persisted, err := ReadSinkReport(sink)
	if err != nil || persisted == nil {
		t.Fatalf("sink not persisted: report=%v err=%v", persisted, err)
	}
	if !strings.Contains(persisted.Answer, "router lives in internal/http/router.go") {
		t.Fatalf("persisted answer = %q", persisted.Answer)
	}
	raw, _ := os.ReadFile(sink)
	t.Logf("sink file content:\n%s", string(raw))
}

func toolResultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := mcp.AsTextContent(c); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// TestReportSinkMcpServers_Registration checks the ACP McpServer wiring: a set
// sink path registers the ghx-report-sink stdio server pointing at this
// executable with the hidden report-sink args; an empty path registers nothing.
func TestReportSinkMcpServers_Registration(t *testing.T) {
	// Under go test os.Executable is the test binary, which the runtime
	// refuses; point at an explicit binary the way eval runs do.
	t.Setenv("GHX_REPORT_SINK_EXE", "/usr/local/bin/ghx")

	if got := reportSinkMcpServers(""); len(got) != 0 {
		t.Fatalf("empty sink path: want no servers, got %+v", got)
	}
	got := reportSinkMcpServers("/tmp/x/report.json")
	if len(got) != 1 || got[0].Stdio == nil {
		t.Fatalf("want one stdio server, got %+v", got)
	}
	s := got[0].Stdio
	if s.Name != ReportSinkServerName {
		t.Fatalf("server name = %q, want %q", s.Name, ReportSinkServerName)
	}
	wantArgs := []string{"sidecar", "report-sink", "--out", "/tmp/x/report.json"}
	if strings.Join(s.Args, " ") != strings.Join(wantArgs, " ") {
		t.Fatalf("args = %v, want %v", s.Args, wantArgs)
	}
	if s.Command == "" {
		t.Fatal("server command (executable path) must not be empty")
	}
}

// TestBuildSessionMeta_AllowsSubmitReport verifies the auto-approve allowlist
// carries the fully-qualified submit_report tool id, so the adapter approves
// the MCP tool instead of routing it through the read-only permission gate.
func TestBuildSessionMeta_AllowsSubmitReport(t *testing.T) {
	meta := BuildSessionMeta("persona", "normal", "", false)
	cc, _ := meta["claudeCode"].(map[string]any)
	opts, _ := cc["options"].(SessionOptions)
	found := false
	for _, tool := range opts.AllowedTools {
		if tool == SubmitReportToolID {
			found = true
		}
	}
	if !found {
		t.Fatalf("allowedTools = %v, want it to contain %q", opts.AllowedTools, SubmitReportToolID)
	}
	if SubmitReportToolID != "mcp__ghx-report-sink__submit_report" {
		t.Fatalf("SubmitReportToolID = %q, unexpected qualified name", SubmitReportToolID)
	}
}
