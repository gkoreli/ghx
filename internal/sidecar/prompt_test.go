package sidecar

import (
	"strings"
	"testing"
)

func TestBuildPromptFirstTurn(t *testing.T) {
	p := BuildPrompt(Request{
		Session:  "hono-middleware",
		Repo:     "honojs/hono",
		Question: "Where is middleware composition implemented?",
	}, nil)

	for _, want := range []string{
		"ghx-sidecar",
		"honojs/hono",
		"hono-middleware",
		"Where is middleware composition implemented?",
		"<ghx-report>",
		"max 8 ghx commands",
		"under\n  2000 characters", // report compactness bound (ADR-0016.1)
		"normal",                   // default depth
		"- remote",                 // default backend
	} {
		if !strings.Contains(p, want) {
			t.Errorf("first-turn prompt missing %q", want)
		}
	}
	if strings.Contains(p, "Prior session context") {
		t.Error("first-turn prompt must not contain prior session context")
	}
}

func TestBuildPromptDefaults(t *testing.T) {
	p := BuildPrompt(Request{Repo: "o/r", Question: "q", Depth: "deep", AllowedBackends: []string{"remote", "codemap"}}, nil)
	if !strings.Contains(p, "deep") {
		t.Error("explicit depth not rendered")
	}
	if !strings.Contains(p, "- codemap") {
		t.Error("explicit backend not rendered")
	}
}

func TestBuildPromptFollowUpIncludesPriorContext(t *testing.T) {
	meta := &SessionMeta{
		Name:      "hono-middleware",
		Repo:      "honojs/hono",
		Scope:     "middleware composition",
		TurnCount: 2,
	}
	ledger := &Ledger{
		InspectedPaths: []LedgerEntry{{Value: "src/compose.ts", Turn: 1}},
		RelevantFiles:  []RelevantFileEntry{{Path: "src/compose.ts", Reason: "defines compose", Turn: 1}},
		OpenQuestions:  []LedgerEntry{{Value: "trace onError path", Turn: 1}},
		CommandsRun:    []LedgerEntry{{Value: "ghx read honojs/hono src/compose.ts", Turn: 1}},
	}
	p := BuildPrompt(Request{
		Session:  "hono-middleware",
		Repo:     "honojs/hono",
		Question: "How do errors propagate through that path?",
	}, meta, ledger)

	if !strings.Contains(p, "Prior session context") {
		t.Fatal("follow-up prompt missing prior session context block")
	}
	if !strings.Contains(p, "Turns completed: 2") {
		t.Error("follow-up prompt missing turn count")
	}
	if !strings.Contains(p, "middleware composition") {
		t.Error("follow-up prompt missing scope")
	}
	for _, want := range []string{
		"## Evidence ledger",
		"Files already inspected must not be re-read unless the new question requires different lines",
		"src/compose.ts - defines compose (turn 1)",
		"trace onError path",
		"ghx read honojs/hono src/compose.ts",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("follow-up prompt missing ledger content %q", want)
		}
	}
}

func TestBuildPromptZeroTurnMetaOmitsPriorContext(t *testing.T) {
	// A freshly initialized session (TurnCount 0) has nothing to carry over.
	meta := &SessionMeta{Name: "s", Repo: "o/r", TurnCount: 0}
	p := BuildPrompt(Request{Repo: "o/r", Question: "q"}, meta)
	if strings.Contains(p, "Prior session context") {
		t.Error("zero-turn session must not render prior context")
	}
}

func TestBuildPromptEvidenceLedgerBoundedAndNewestFirst(t *testing.T) {
	meta := &SessionMeta{Name: "s", Repo: "o/r", Scope: "scope", TurnCount: 3}
	ledger := &Ledger{
		OpenQuestions: []LedgerEntry{{Value: "must keep even when trimming", Turn: 1}},
	}
	for i := 0; i < 80; i++ {
		ledger.CommandsRun = append(ledger.CommandsRun, LedgerEntry{Value: "ghx read o/r src/old-file-with-a-long-name.ts", Turn: i + 1})
		ledger.InspectedPaths = append(ledger.InspectedPaths, LedgerEntry{Value: "src/path-with-a-long-name-that-may-be-trimmed.ts", Turn: i + 1})
	}

	p := BuildPrompt(Request{Session: "s", Repo: "o/r", Question: "q"}, meta, ledger)
	start := strings.Index(p, "## Evidence ledger")
	if start == -1 {
		t.Fatal("missing evidence ledger")
	}
	end := strings.Index(p[start:], "## submit_report")
	if end == -1 {
		t.Fatal("missing submit_report after ledger")
	}
	block := p[start : start+end]
	if len(block) > 1500 {
		t.Fatalf("ledger block length = %d, want <= 1500\n%s", len(block), block)
	}
	if !strings.Contains(block, "must keep even when trimming") {
		t.Fatalf("open question was trimmed:\n%s", block)
	}
	if !strings.Contains(block, "(turn 80)") {
		t.Fatalf("newest evidence missing:\n%s", block)
	}
}
