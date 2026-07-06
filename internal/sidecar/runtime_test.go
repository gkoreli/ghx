package sidecar

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
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

func TestAskRecreatesStaleACPSessionOnce(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	if err := InitSession(dir, "s", "o/r", "scope"); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	meta.ACPSessionID = "stale-session"
	if err := SaveMeta(dir, *meta); err != nil {
		t.Fatal(err)
	}

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls []RunTurnOptions
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls = append(calls, opts)
		if len(calls) == 1 {
			return TurnResult{}, "", fmt.Errorf("acp load session: %w", &acp.RequestError{Code: -32002, Message: "Resource not found"})
		}
		if opts.ACPSessionID != "" {
			t.Fatalf("fresh retry must use NewSession, got ACPSessionID=%q", opts.ACPSessionID)
		}
		return TurnResult{FullText: `<ghx-report>{"answer":"ok"}</ghx-report>`}, "fresh-session", nil
	}

	_, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("turn calls = %d, want stale LoadSession + one NewSession retry", len(calls))
	}
	if calls[0].ACPSessionID != "stale-session" {
		t.Fatalf("first call session = %q, want stale-session", calls[0].ACPSessionID)
	}
	if !result.SessionRecreated {
		t.Fatal("SessionRecreated not recorded")
	}
	updated, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ACPSessionID != "fresh-session" {
		t.Fatalf("persisted session id = %q, want fresh-session", updated.ACPSessionID)
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

// TestAskFallsBackToWarnWhenRetriesExhausted: the WARN placeholder remains
// the honest last resort, and retries are bounded at maxReportRetries (two),
// so the total number of turns never exceeds 1 + maxReportRetries (ADR-0021 D2).
func TestAskFallsBackToWarnWhenRetriesExhausted(t *testing.T) {
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
	if want := 1 + maxReportRetries; calls != want {
		t.Fatalf("turn calls = %d, want exactly %d (initial + %d retries)", calls, want, maxReportRetries)
	}
	if !strings.HasPrefix(report.Answer, "WARN: sidecar did not emit") {
		t.Fatalf("answer = %q, want WARN placeholder", report.Answer)
	}
	if !result.ReportRetried {
		t.Fatal("ReportRetried should be recorded even when the retries fail")
	}
}

// TestAskPrefersSinkReport pins ADR-0021 D2: when the agent submits an
// accepted report through the report-sink (simulated by the stub writing to
// opts.ReportSinkPath), the runtime uses that report and prefers it over any
// <ghx-report> text block, with no corrective retry.
func TestAskPrefersSinkReport(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls int
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls++
		if opts.ReportSinkPath == "" {
			t.Fatal("Ask must pass a report sink path to the turn")
		}
		// Simulate a successful submit_report tool call writing the sink.
		if err := writeSinkReport(opts.ReportSinkPath, &Report{Answer: "from sink"}); err != nil {
			t.Fatal(err)
		}
		// A conflicting text block must be ignored in favour of the sink.
		return TurnResult{FullText: `<ghx-report>{"answer":"from text block"}</ghx-report>`}, "sess-1", nil
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("turn calls = %d, want 1 (no retry when the sink is accepted)", calls)
	}
	if report.Answer != "from sink" {
		t.Fatalf("answer = %q, want the sink report preferred over the text block", report.Answer)
	}
	if result.ReportRetried {
		t.Fatal("no retry expected when the sink report is present")
	}
	if result.ReportCoerced {
		t.Fatal("sink reports are strictly validated, ReportCoerced must be false")
	}
}

// TestAskRetryPromptCarriesConcreteError pins ADR-0021 D2/D3: when extraction
// fails, the corrective follow-up embeds the CONCRETE reason (here, an invalid
// JSON error), not a generic nudge.
func TestAskRetryPromptCarriesConcreteError(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls []RunTurnOptions
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls = append(calls, opts)
		if len(calls) == 1 {
			// A <ghx-report> block that is not valid JSON.
			return TurnResult{FullText: "<ghx-report>{not json at all}</ghx-report>"}, "sess-1", nil
		}
		return TurnResult{FullText: `<ghx-report>{"answer":"recovered"}</ghx-report>`}, "sess-1", nil
	}

	report, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) < 2 {
		t.Fatalf("expected at least one retry, got %d calls", len(calls))
	}
	if !strings.Contains(calls[1].Prompt, "invalid JSON") {
		t.Fatalf("retry prompt should embed the concrete reason, got:\n%s", calls[1].Prompt)
	}
	if report.Answer != "recovered" {
		t.Fatalf("answer = %q, want recovered", report.Answer)
	}
}

// TestAskFlagsCoercedReport pins ADR-0021 D3: a report obtained only through
// lenient coercion on the text fallback path is flagged as coerced so evals
// count the drift.
func TestAskFlagsCoercedReport(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		// "verified" is a bare string where an array of claims is required —
		// coercible by coerceReportJSON.
		return TurnResult{FullText: `<ghx-report>{"answer":"ok","verified":"a single claim"}</ghx-report>`}, "sess-1", nil
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Answer != "ok" || len(report.Verified) != 1 {
		t.Fatalf("coerced report = %+v", report)
	}
	if !result.ReportCoerced {
		t.Fatal("ReportCoerced should be set when the report needed coercion")
	}
}

// TestAskWrapsUpOnMaxTurns pins ADR-0027 D1: the adapter's max-turns error
// triggers exactly one LoadSession wrap-up turn on the SAME ACP session with
// the exact wrap-up prompt; the recovered report ships, the recovery is
// recorded on the turn result, and no error escapes.
func TestAskWrapsUpOnMaxTurns(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls []RunTurnOptions
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls = append(calls, opts)
		if len(calls) == 1 {
			// The session was established before the adapter killed the prompt,
			// so the session ID is returned alongside the error (ADR-0027 D2).
			return TurnResult{FullText: "twenty tool calls of exploration"}, "sess-1",
				errors.New(`acp prompt: {"code":-32603,"message":"Internal error","data":{"error":"Reached maximum number of turns (24)"}}`)
		}
		// The wrap-up turn: agent submits what it has via the sink.
		if err := writeSinkReport(opts.ReportSinkPath, &Report{Answer: "recovered by wrap-up"}); err != nil {
			t.Fatal(err)
		}
		return TurnResult{FullText: "wrapping up"}, "sess-1", nil
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatalf("Ask must recover from max-turns, got error: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("turn calls = %d, want exactly 2 (turn + one wrap-up)", len(calls))
	}
	if calls[1].ACPSessionID != "sess-1" {
		t.Fatalf("wrap-up session id = %q, want the failed turn's sess-1 (same session resume)", calls[1].ACPSessionID)
	}
	if calls[1].Prompt != turnCapWrapUpPrompt {
		t.Fatalf("wrap-up prompt = %q, want exactly %q", calls[1].Prompt, turnCapWrapUpPrompt)
	}
	if calls[1].ReportSinkPath == "" || calls[1].ReportSinkPath != calls[0].ReportSinkPath {
		t.Fatalf("wrap-up must reuse the turn's report sink: %q vs %q", calls[1].ReportSinkPath, calls[0].ReportSinkPath)
	}
	if calls[1].SessionMeta == nil || !reflect.DeepEqual(calls[1].SessionMeta, calls[0].SessionMeta) {
		t.Fatalf("wrap-up must carry the same full session meta as the primary turn (ADR-0020.2):\nwrap-up: %v\nprimary: %v",
			calls[1].SessionMeta, calls[0].SessionMeta)
	}
	if report.Answer != "recovered by wrap-up" {
		t.Fatalf("answer = %q, want the wrap-up's report", report.Answer)
	}
	if !result.WrapUpRecovered {
		t.Fatal("WrapUpRecovered not recorded on the turn result")
	}
	if result.ReportRetried {
		t.Fatal("wrap-up recovery is not a report retry")
	}
}

// TestAskBlockedWhenWrapUpFails pins the D1 terminal state: exactly one
// wrap-up attempt; if it also fails, the turn ships a BLOCKED report (with
// both failures recorded) instead of a hard error, so artifacts and session
// state survive.
func TestAskBlockedWhenWrapUpFails(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var calls int
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls++
		return TurnResult{}, "sess-1", errors.New("Reached maximum number of turns (24)")
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err != nil {
		t.Fatalf("BLOCKED terminal state must not surface as an error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("turn calls = %d, want exactly 2 (no retries after a failed wrap-up)", calls)
	}
	if !strings.HasPrefix(report.Answer, "BLOCKED:") {
		t.Fatalf("answer = %q, want a BLOCKED report", report.Answer)
	}
	if len(report.Uncertainty) < 2 || !strings.Contains(strings.Join(report.Uncertainty, "\n"), "Reached maximum number of turns") {
		t.Fatalf("uncertainty must record both failures, got %v", report.Uncertainty)
	}
	if result.WrapUpRecovered {
		t.Fatal("a failed wrap-up is not a recovery")
	}
	// The session ID must still be persisted for later resumption.
	meta, err := ReadMeta(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if meta.ACPSessionID != "sess-1" {
		t.Fatalf("persisted session id = %q, want sess-1", meta.ACPSessionID)
	}
}

// TestAskReturnsPartialResultOnUnrecoveredFailure pins the D3 contract at the
// API boundary: a non-turn-cap failure (watchdog, dead peer) returns the
// partial TurnResult alongside the error so callers can audit the failed turn.
func TestAskReturnsPartialResultOnUnrecoveredFailure(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		return TurnResult{
			FullText:  "partial exploration before the hang",
			ToolCalls: []string{"execute: ghx tree o/r (completed)"},
		}, "sess-1", fmt.Errorf("acp prompt: %w", ErrLivenessTimeout)
	}

	report, result, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err == nil || !errors.Is(err, ErrLivenessTimeout) {
		t.Fatalf("err = %v, want the liveness timeout to surface", err)
	}
	if report != nil {
		t.Fatalf("report = %+v, want nil on unrecovered failure", report)
	}
	if result == nil || result.FullText != "partial exploration before the hang" || len(result.ToolCalls) != 1 {
		t.Fatalf("partial turn result not returned: %+v", result)
	}
	meta, metaErr := ReadMeta(dir, "s")
	if metaErr != nil {
		t.Fatal(metaErr)
	}
	if meta.ACPSessionID != "sess-1" {
		t.Fatalf("failed turn must still persist the session id for resumption, got %q", meta.ACPSessionID)
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

// TestAskPersonaSelectionByRepoScope verifies ADR-0019.1 D4: a repo-scoped ask
// keeps the exact existing persona in the session meta, while an ask without
// repo scope gets the discovery persona.
func TestAskPersonaSelectionByRepoScope(t *testing.T) {
	stubHandshake(t)
	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()

	var systemPrompt string
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		systemPrompt = ""
		if opts.SessionMeta != nil {
			if cc, ok := opts.SessionMeta["claudeCode"].(map[string]any); ok {
				if so, ok := cc["options"].(SessionOptions); ok {
					systemPrompt = so.SystemPrompt
				}
			}
		}
		return TurnResult{FullText: `<ghx-report>{"answer":"ok"}</ghx-report>`}, "sid", nil
	}

	// Repo-scoped: byte-identical to the existing persona.
	if _, _, err := Ask(context.Background(), Config{SessionsDir: t.TempDir(), AgentCmd: "mock"}, AskRequest{
		Session: "s1", Repo: "o/r", Question: "q",
	}); err != nil {
		t.Fatal(err)
	}
	if systemPrompt != BuildPersonaSystemPrompt() {
		t.Fatal("repo-scoped ask must use the exact repo-scoped persona")
	}

	// Discovery: persona plus the discovery doctrine.
	if _, _, err := Ask(context.Background(), Config{SessionsDir: t.TempDir(), AgentCmd: "mock"}, AskRequest{
		Session: "s2", Question: "which repos do X",
	}); err != nil {
		t.Fatal(err)
	}
	if systemPrompt != BuildDiscoveryPersonaSystemPrompt() {
		t.Fatal("discovery ask must use the discovery persona")
	}
	if !strings.Contains(systemPrompt, "## Discovery mode") {
		t.Fatal("discovery persona missing discovery doctrine section")
	}
}
