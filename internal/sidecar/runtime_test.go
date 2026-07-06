package sidecar

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func stubHandshake(t *testing.T) {
	t.Helper()
	old := checkACPHandshake
	checkACPHandshake = func(context.Context, string, string, []string, time.Duration) error {
		return nil
	}
	t.Cleanup(func() { checkACPHandshake = old })
}

func TestAskFallbackPromptCarriesLedger(t *testing.T) {
	stubHandshake(t)
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

// TestAskRetriesOnceWhenReportMissing pins the ADR-0016.7 RC3 one-shot
// corrective follow-up: a turn without a <ghx-report> block gets exactly
// one nudge in the same ACP session; the retry's report is used, the
// retry is recorded, and the retried session ID is persisted.
func TestAskRetriesOnceWhenReportMissing(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls []RunTurnOptions
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls = append(calls, opts)
		if len(calls) == 1 {
			return TurnResult{FullText: "explored a lot but forgot the report block"}, "sess-1", nil
		}
		return TurnResult{FullText: `<ghx-report>{"answer":"recovered"}</ghx-report>`}, "sess-2", nil
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("turn calls = %d, want exactly 2 (turn + one retry)", len(calls))
	}
	if calls[1].ACPSessionID != "sess-1" {
		t.Fatalf("retry session id = %q, want the turn's session sess-1", calls[1].ACPSessionID)
	}
	if !strings.Contains(calls[1].Prompt, "<ghx-report>") || strings.Contains(calls[1].Prompt, "## Role") {
		t.Fatalf("retry prompt should be the corrective nudge, not a full persona:\n%s", calls[1].Prompt)
	}
	if report.Answer != "recovered" {
		t.Fatalf("answer = %q, want the retry's report", report.Answer)
	}
	if !result.ReportRetried {
		t.Fatal("ReportRetried not recorded")
	}
	meta, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if meta.ACPSessionID != "sess-2" {
		t.Fatalf("persisted session id = %q, want retry's sess-2", meta.ACPSessionID)
	}
}

// TestAskFallsBackToWarnWhenRetryAlsoFails: the WARN placeholder remains
// the honest last resort, and only one retry is ever attempted.
func TestAskFallsBackToWarnWhenRetryAlsoFails(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls int
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls++
		return TurnResult{FullText: "still no report block"}, "sess-1", nil
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("turn calls = %d, want exactly 2 (never more than one retry)", calls)
	}
	if !strings.HasPrefix(report.Answer, "WARN: sidecar did not emit") {
		t.Fatalf("answer = %q, want WARN placeholder", report.Answer)
	}
	if !result.ReportRetried {
		t.Fatal("ReportRetried should be recorded even when the retry fails")
	}
}

func TestAskFailsBeforeTurnWhenACPHandshakeFails(t *testing.T) {
	dir := t.TempDir()
	oldHandshake := checkACPHandshake
	oldRunTurn := runTurnWithOptions
	defer func() {
		checkACPHandshake = oldHandshake
		runTurnWithOptions = oldRunTurn
	}()

	wantErr := errors.New(ACPHandshakeFailureMessage("mock"))
	checkACPHandshake = func(context.Context, string, string, []string, time.Duration) error {
		return wantErr
	}
	runTurnWithOptions = func(context.Context, RunTurnOptions) (TurnResult, string, error) {
		t.Fatal("Ask should fail before running a prompt turn when handshake fails")
		return TurnResult{}, "", nil
	}

	_, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err == nil || !strings.Contains(err.Error(), "did not complete the ACP handshake") {
		t.Fatalf("Ask error = %v, want handshake failure", err)
	}
}
