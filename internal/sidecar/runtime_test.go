package sidecar

import (
	"context"
	"strings"
	"testing"
)

func TestAskFallbackPromptCarriesLedger(t *testing.T) {
	dir := t.TempDir()
	if err := InitSession(dir, "s", "o/r", "scope"); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	meta.TurnCount = 1
	meta.ACPSessionID = "adapter-session-that-may-not-load"
	if err := SaveMeta(dir, *meta); err != nil {
		t.Fatal(err)
	}
	if err := SaveLedger(dir, "s", &Ledger{
		Repo:           "o/r",
		Scope:          "scope",
		InspectedPaths: []LedgerEntry{{Value: "src/a.go", Turn: 1}},
		OpenQuestions:  []LedgerEntry{{Value: "inspect src/b.go", Turn: 1}},
		CommandsRun:    []LedgerEntry{{Value: "ghx read o/r src/a.go", Turn: 1}},
	}); err != nil {
		t.Fatal(err)
	}

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var prompt string
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		prompt = opts.Prompt
		if opts.ACPSessionID != "adapter-session-that-may-not-load" {
			t.Fatalf("Ask should pass existing transport id to runtime, got %q", opts.ACPSessionID)
		}
		return TurnResult{FullText: `<ghx-report>{"answer":"ok","verified":[],"inferred":[],"unverified":[],"relevantFiles":[],"evidence":[],"backendsUsed":["remote"],"commandsRun":[],"uncertainty":[],"nextReads":[]}</ghx-report>`}, "fresh-session", nil
	}

	if _, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  "s",
		Repo:     "o/r",
		Question: "follow up",
	}); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"## Evidence ledger",
		"src/a.go",
		"inspect src/b.go",
		"Files already inspected must not be re-read",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("captured prompt missing %q:\n%s", want, prompt)
		}
	}
	updated, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ACPSessionID != "fresh-session" || updated.TurnCount != 2 {
		t.Fatalf("session metadata not updated after fallback turn: %+v", updated)
	}
}
