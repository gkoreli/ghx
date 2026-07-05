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

// TestExtractReportCoercesNearConformantShapes is the ADR-0016.7 RC2
// regression: the exact shape drift observed in gate-run episode
// hono-middleware_ghx-sidecar_1783285987325 turn 1 — bare strings where
// the schema wants arrays ([]Claim, []string) and single objects where it
// wants object arrays. Strict parsing discarded that entire report and
// scored completed work as zero evidence.
func TestExtractReportCoercesNearConformantShapes(t *testing.T) {
	text := `<ghx-report>
{
  "answer": "Errors propagate on two layers: compose.ts dispatch and hono-base onError.",
  "verified": {"summary": "compose() dispatch try/catches handler calls", "evidence": "src/compose.ts"},
  "inferred": [],
  "unverified": "Whether HTTPException is the sole implementer of getResponse()",
  "relevantFiles": {"path": "src/compose.ts", "reason": "inner try/catch"},
  "evidence": {"source": "ghx read honojs/hono src/hono-base.ts --grep onError", "summary": "error handler wiring"},
  "backendsUsed": "remote",
  "commandsRun": "ghx read honojs/hono src/hono-base.ts --grep onError",
  "uncertainty": "Did not re-confirm HTTPException.getResponse() this turn",
  "nextReads": "src/http-exception.ts"
}
</ghx-report>`

	r := ExtractReport(text)
	if r == nil {
		t.Fatal("near-conformant report was rejected, want coerced parse")
	}
	if len(r.Verified) != 1 || r.Verified[0].Evidence != "src/compose.ts" {
		t.Errorf("verified = %+v, want single-object coerced to 1 claim", r.Verified)
	}
	if len(r.Unverified) != 1 || !strings.Contains(r.Unverified[0].Summary, "HTTPException") {
		t.Errorf("unverified = %+v, want bare string lifted to claim", r.Unverified)
	}
	if len(r.RelevantFiles) != 1 || r.RelevantFiles[0].Path != "src/compose.ts" {
		t.Errorf("relevantFiles = %+v, want single object wrapped", r.RelevantFiles)
	}
	if len(r.Evidence) != 1 || r.Evidence[0].Source == "" {
		t.Errorf("evidence = %+v, want single object wrapped", r.Evidence)
	}
	if len(r.CommandsRun) != 1 || len(r.BackendsUsed) != 1 || len(r.Uncertainty) != 1 || len(r.NextReads) != 1 {
		t.Errorf("string lists not wrapped: commands=%v backends=%v uncertainty=%v nextReads=%v",
			r.CommandsRun, r.BackendsUsed, r.Uncertainty, r.NextReads)
	}
}

// TestExtractReportCoercionRejectsGarbage: coercion must not resurrect
// reports that are wrong beyond shape drift.
func TestExtractReportCoercionRejectsGarbage(t *testing.T) {
	if r := ExtractReport(`<ghx-report>{"answer": "x", "verified": 42}</ghx-report>`); r != nil {
		t.Errorf("numeric claim list should stay rejected, got %+v", r)
	}
}
