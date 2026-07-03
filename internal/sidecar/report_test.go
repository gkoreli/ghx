package sidecar

import (
	"strings"
	"testing"
)

func TestExtractReportValid(t *testing.T) {
	text := `Some exploration narration.

<ghx-report>
{
  "answer": "Middleware composition lives in src/compose.ts.",
  "verified": [{"summary": "compose() dispatches handlers", "evidence": "src/compose.ts defines compose()"}],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [{"path": "src/compose.ts", "reason": "defines compose()"}],
  "evidence": [{"source": "ghx read honojs/hono src/compose.ts --map", "summary": "compose signature"}],
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx tree honojs/hono src --depth 2"],
  "uncertainty": ["did not inspect tests"],
  "nextReads": ["src/hono-base.ts"]
}
</ghx-report>`

	r := ExtractReport(text)
	if r == nil {
		t.Fatal("expected report, got nil")
	}
	if !strings.Contains(r.Answer, "src/compose.ts") {
		t.Errorf("answer = %q, want mention of src/compose.ts", r.Answer)
	}
	if len(r.Verified) != 1 || r.Verified[0].Evidence == "" {
		t.Errorf("verified = %+v, want 1 claim with evidence", r.Verified)
	}
	if len(r.RelevantFiles) != 1 || r.RelevantFiles[0].Path != "src/compose.ts" {
		t.Errorf("relevantFiles = %+v", r.RelevantFiles)
	}
	if len(r.CommandsRun) != 1 {
		t.Errorf("commandsRun = %+v, want 1 command", r.CommandsRun)
	}
}

func TestExtractReportMissingBlock(t *testing.T) {
	if r := ExtractReport("plain prose with no report block"); r != nil {
		t.Errorf("expected nil, got %+v", r)
	}
}

func TestExtractReportMalformedJSON(t *testing.T) {
	if r := ExtractReport("<ghx-report>{not json}</ghx-report>"); r != nil {
		t.Errorf("expected nil for malformed JSON, got %+v", r)
	}
}

func TestExtractReportEmptyAnswer(t *testing.T) {
	if r := ExtractReport(`<ghx-report>{"answer": ""}</ghx-report>`); r != nil {
		t.Errorf("expected nil for empty answer, got %+v", r)
	}
}

func TestExtractReportTakesFirstBlock(t *testing.T) {
	text := `<ghx-report>{"answer": "first"}</ghx-report>
<ghx-report>{"answer": "second"}</ghx-report>`
	r := ExtractReport(text)
	if r == nil || r.Answer != "first" {
		t.Errorf("got %+v, want answer %q", r, "first")
	}
}

func TestExtractReportMultilineContent(t *testing.T) {
	// (?s) flag must let the block span newlines and inner braces.
	text := "prefix\n<ghx-report>\n{\"answer\": \"a\", \"verified\": [{\"summary\": \"s\"}]}\n</ghx-report>\nsuffix"
	r := ExtractReport(text)
	if r == nil || r.Answer != "a" {
		t.Fatalf("got %+v", r)
	}
}
