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

// TestValidateReportEvidence_Matrix pins the ADR-0027 D4 evidence contract:
// a non-BLOCKED report must carry ALL of a verified claim with evidence, a
// relevant file, and a command run; rejections name exactly the missing
// fields; BLOCKED reports state why and skip the requirement.
func TestValidateReportEvidence_Matrix(t *testing.T) {
	full := func() *Report {
		return &Report{
			Answer:        "the router lives in router.go",
			Verified:      []Claim{{Summary: "router registered", Evidence: "router.go:42"}},
			RelevantFiles: []RelevantFile{{Path: "router.go", Reason: "routes"}},
			CommandsRun:   []string{"ghx read o/r router.go"},
		}
	}
	cases := []struct {
		name        string
		mutate      func(*Report)
		wantMissing []string // fields the error must name; empty = accepted
		wantAbsent  []string // fields the error must NOT name
	}{
		{name: "full evidence accepted", mutate: func(r *Report) {}},
		{
			name: "answer-only rejected naming all three",
			mutate: func(r *Report) {
				r.Verified, r.RelevantFiles, r.CommandsRun = nil, nil, nil
			},
			wantMissing: []string{"verified", "relevantFiles", "commandsRun"},
		},
		{
			name:        "verified claim without evidence rejected",
			mutate:      func(r *Report) { r.Verified = []Claim{{Summary: "claim, no evidence"}} },
			wantMissing: []string{"verified"},
			wantAbsent:  []string{"relevantFiles", "commandsRun"},
		},
		{
			name:        "missing relevant files rejected naming only that field",
			mutate:      func(r *Report) { r.RelevantFiles = nil },
			wantMissing: []string{"relevantFiles"},
			wantAbsent:  []string{"verified:", "commandsRun"},
		},
		{
			name:        "blank command entries rejected",
			mutate:      func(r *Report) { r.CommandsRun = []string{"  "} },
			wantMissing: []string{"commandsRun"},
			wantAbsent:  []string{"verified:", "relevantFiles"},
		},
		{
			name: "BLOCKED with reason passes with zero evidence",
			mutate: func(r *Report) {
				r.Answer = "BLOCKED: ghx is unavailable in this sidecar session."
				r.Verified, r.RelevantFiles, r.CommandsRun = nil, nil, nil
			},
		},
		{
			name: "BLOCKED without a reason rejected",
			mutate: func(r *Report) {
				r.Answer = "BLOCKED:"
				r.Verified, r.RelevantFiles, r.CommandsRun = nil, nil, nil
			},
			wantMissing: []string{"state why"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := full()
			tc.mutate(r)
			err := ValidateReportEvidence(r)
			if len(tc.wantMissing) == 0 {
				if err != nil {
					t.Fatalf("want accepted, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("want rejection, got nil")
			}
			for _, want := range tc.wantMissing {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("error %q must name %q", err, want)
				}
			}
			for _, absent := range tc.wantAbsent {
				if strings.Contains(err.Error(), absent) {
					t.Fatalf("error %q must not name %q (field is present)", err, absent)
				}
			}
		})
	}
}

// TestDecodeReportStrict_EvidenceRequired proves the sink's strict decode path
// enforces the D4 evidence contract end to end (answer-only in, exact
// field-level errors out) and still accepts BLOCKED escape-hatch reports.
func TestDecodeReportStrict_EvidenceRequired(t *testing.T) {
	_, err := DecodeReportStrict([]byte(`{"answer":"an answer with no evidence at all"}`))
	if err == nil {
		t.Fatal("answer-only report must be rejected (ADR-0027 D4)")
	}
	for _, field := range []string{"verified", "relevantFiles", "commandsRun"} {
		if !strings.Contains(err.Error(), field) {
			t.Fatalf("error %q must name %q", err, field)
		}
	}
	if _, err := DecodeReportStrict([]byte(`{"answer":"BLOCKED: sandbox has no network","uncertainty":["curl: (6) could not resolve host"]}`)); err != nil {
		t.Fatalf("BLOCKED report with a reason must pass: %v", err)
	}
}

func TestDecodeReportStrict_ADR0029ReportShapeValidation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{
			name:   "markdown heading rejected",
			mutate: func(args map[string]any) { args["answer"] = "# Heading\nThe router lives in router.go." },
			want:   "Markdown headings",
		},
		{
			name: "long answer rejected",
			mutate: func(args map[string]any) {
				args["answer"] = strings.Repeat("a", maxReportAnswerChars+1)
			},
			want: "compact answer required",
		},
		{
			name: "tmp claim evidence rejected",
			mutate: func(args map[string]any) {
				args["verified"] = []any{map[string]any{"summary": "x", "evidence": "/tmp/sidecar-evidence.txt"}}
			},
			want: "local scratch files",
		},
		{
			name: "tmp evidence source rejected",
			mutate: func(args map[string]any) {
				args["evidence"] = []any{map[string]any{"source": "/tmp/ghx-output.txt", "summary": "scratch output"}}
			},
			want: "local scratch files",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := validReportArgs()
			tc.mutate(args)
			data, _ := json.Marshal(args)
			_, err := DecodeReportStrict(data)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection containing %q, got %v", tc.want, err)
			}
		})
	}

	if _, err := DecodeReportStrict(mustJSON(t, validReportArgs())); err != nil {
		t.Fatalf("compact valid report must pass: %v", err)
	}
	blocked := map[string]any{"answer": "# BLOCKED\nstill not a normal answer"}
	blocked["answer"] = "BLOCKED: could not start ghx; /tmp/diagnostic is not cited as evidence."
	if _, err := DecodeReportStrict(mustJSON(t, blocked)); err != nil {
		t.Fatalf("BLOCKED report remains exempt from evidence/shape checks: %v", err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
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
	backends, _ := props["backendsUsed"].(map[string]any)
	if got, want := backends["description"], `Evidence backends used — canonical IDs: "remote", "local:codemap", "local:ast-grep", "local:repomap".`; got != want {
		t.Fatalf("backendsUsed description = %q, want %q", got, want)
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
	meta := BuildSessionMeta("persona", "normal", "", false, nil)
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

// TestDecodeReportStrictAcceptsRepoLevelCitations verifies ADR-0019.1 D3: the
// report contract does not change for discovery, and nothing in strict
// validation rejects owner/repo or owner/repo:path citation forms.
func TestDecodeReportStrictAcceptsRepoLevelCitations(t *testing.T) {
	payload := `{
		"answer": "open-telemetry/opentelemetry-go ships a stdout OTLP-shaped exporter.",
		"verified": [{"summary": "stdouttrace exporter exists", "evidence": "open-telemetry/opentelemetry-go:exporters/stdout/stdouttrace/trace.go"}],
		"inferred": [{"summary": "tobert/otel-file-exporter looked relevant in search results only"}],
		"relevantFiles": [
			{"path": "open-telemetry/opentelemetry-go", "reason": "verified candidate (repo-level)"},
			{"path": "open-telemetry/opentelemetry-go:exporters/stdout/stdouttrace/trace.go", "reason": "exporter implementation"}
		],
		"evidence": [{"source": "ghx search \"stdouttrace\"", "summary": "candidate sweep"}],
		"commandsRun": ["ghx search \"stdouttrace\"", "ghx read open-telemetry/opentelemetry-go exporters/stdout/stdouttrace/trace.go"],
		"uncertainty": ["candidate sweep bounded at depth normal"]
	}`
	r, err := DecodeReportStrict([]byte(payload))
	if err != nil {
		t.Fatalf("repo-level citations must pass strict validation: %v", err)
	}
	if r.RelevantFiles[0].Path != "open-telemetry/opentelemetry-go" {
		t.Fatalf("owner/repo path mangled: %q", r.RelevantFiles[0].Path)
	}
	if r.RelevantFiles[1].Path != "open-telemetry/opentelemetry-go:exporters/stdout/stdouttrace/trace.go" {
		t.Fatalf("owner/repo:path form mangled: %q", r.RelevantFiles[1].Path)
	}
}

// TestDecodeReportStrict_TierUsed covers the ADR-0024.1 report contract
// extension: tierUsed is optional, canonical tier IDs are accepted, and any
// other value is rejected with a teaching error. A tier2 report carries its
// local:* backend in backendsUsed alongside the recomputable command.
func TestDecodeReportStrict_TierUsed(t *testing.T) {
	args := validReportArgs()
	args["tierUsed"] = "tier2"
	args["backendsUsed"] = []any{"remote", "local:codemap"}
	args["commandsRun"] = []any{"ghx tier2 codemap owner/repo --importers internal/http/router.go"}
	data, _ := json.Marshal(args)
	r, err := DecodeReportStrict(data)
	if err != nil {
		t.Fatalf("tier2 report rejected: %v", err)
	}
	if r.TierUsed != "tier2" || len(r.BackendsUsed) != 2 || r.BackendsUsed[1] != "local:codemap" {
		t.Fatalf("tier fields lost in decode: %+v", r)
	}

	for _, tier := range []string{"tier0", "tier1", "tier3"} {
		args["tierUsed"] = tier
		data, _ = json.Marshal(args)
		if _, err := DecodeReportStrict(data); err != nil {
			t.Errorf("canonical tier %q rejected: %v", tier, err)
		}
	}

	// Omitted tierUsed stays valid (pre-tier reports are unchanged).
	data, _ = json.Marshal(validReportArgs())
	if r, err := DecodeReportStrict(data); err != nil || r.TierUsed != "" {
		t.Fatalf("report without tierUsed must remain valid: %v", err)
	}

	args["tierUsed"] = "local"
	data, _ = json.Marshal(args)
	if _, err := DecodeReportStrict(data); err == nil || !strings.Contains(err.Error(), "tierUsed") {
		t.Fatalf("non-canonical tierUsed must be rejected with a tierUsed error, got %v", err)
	}
}

// TestDecodeReportStrict_NormalizesNextReads: the strict path applies the
// lenient ADR-0031.2 nextReads normalizer AFTER validation — path-shaped
// entries are cleaned, prose entries are kept, and no report fails over
// nextReads shape.
func TestDecodeReportStrict_NormalizesNextReads(t *testing.T) {
	args := validReportArgs()
	args["nextReads"] = []any{"`gin.go:364`", "tree.go lines 135-400 for insertion logic", "  ", "routergroup.go"}
	data, _ := json.Marshal(args)
	r, err := DecodeReportStrict(data)
	if err != nil {
		t.Fatalf("prose/blank nextReads must never reject a report: %v", err)
	}
	want := []string{"gin.go", "tree.go lines 135-400 for insertion logic", "routergroup.go"}
	if len(r.NextReads) != len(want) {
		t.Fatalf("NextReads = %q, want %q", r.NextReads, want)
	}
	for i := range want {
		if r.NextReads[i] != want[i] {
			t.Errorf("NextReads[%d] = %q, want %q", i, r.NextReads[i], want[i])
		}
	}
}
