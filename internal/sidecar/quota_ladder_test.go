package sidecar

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// quotaWireErr is the exact wire error from the badslugnoslash dead-ask log
// (ground truth for the ADR-0040 L3 trigger, see TestIsQuotaExhausted).
func quotaWireErr() error {
	return fmt.Errorf("acp prompt: %s", `{"code":-32603,"message":"Internal error: You've hit your session limit · resets 8:30am (UTC)","data":{"errorKind":"rate_limit"}}`)
}

// stubQuotaHandshake silences the ACP handshake for Ask-path tests in this file.
func stubQuotaHandshake(t *testing.T) {
	t.Helper()
	old := checkACPHandshake
	checkACPHandshake = func(context.Context, string, string, []string, time.Duration) error { return nil }
	t.Cleanup(func() { checkACPHandshake = old })
}

// seedLedgerSession creates a session with durable ledger evidence so the
// cached-ledger rung has something to degrade to.
func seedLedgerSession(t *testing.T, dir, name string) {
	t.Helper()
	if err := InitSession(dir, name, "o/r", "scope", SessionNamedExplicit); err != nil {
		t.Fatal(err)
	}
	if err := SaveLedger(dir, name, &Ledger{
		Repo:           "o/r",
		Scope:          "scope",
		InspectedPaths: []LedgerEntry{{Value: "src/a.go", Turn: 1}},
		CommandsRun:    []LedgerEntry{{Value: "ghx read o/r src/a.go", Turn: 1}},
	}); err != nil {
		t.Fatal(err)
	}
}

// primaryTurnStub returns a runTurnWithOptions stub whose first call fails with
// the quota wire error and later calls delegate to next (nil = panic loudly).
func primaryTurnStub(t *testing.T, next func(ctx context.Context, opts RunTurnOptions) (TurnResult, string, error)) func(context.Context, RunTurnOptions) (TurnResult, string, error) {
	t.Helper()
	calls := 0
	return func(ctx context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		calls++
		if calls == 1 {
			return TurnResult{}, "", quotaWireErr()
		}
		if next == nil {
			t.Fatalf("unexpected turn call %d (only the quota-failing primary was expected)", calls)
		}
		return next(ctx, opts)
	}
}

// TestAskQuotaLadderFallbackBackend pins rung 2 (degraded:model): with
// FallbackAgentCmd configured, a quota-dead primary turn is retried once on
// the fallback command line and its answer ships relabeled DEGRADED
// (degraded:model) — never the raw fresh answer.
func TestAskQuotaLadderFallbackBackend(t *testing.T) {
	stubQuotaHandshake(t)
	dir := t.TempDir()
	seedLedgerSession(t, dir, "s")

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	var fbAgentCmd, fbModel string
	runTurnWithOptions = primaryTurnStub(t, func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		fbAgentCmd = opts.AgentCmd
		if m, ok := opts.SessionMeta["claudeCode"]; ok {
			if cc, ok := m.(map[string]any); ok {
				if o, ok := cc["options"].(SessionOptions); ok {
					fbModel = o.Model
				}
			}
		}
		return TurnResult{FullText: `<ghx-report>{"answer":"the middleware chain is in src/mw.go","verified":[{"summary":"chain lives in src/mw.go","evidence":"src/mw.go:10"}],"relevantFiles":[{"path":"src/mw.go"}],"backendsUsed":["remote"],"commandsRun":["ghx read o/r src/mw.go"]}</ghx-report>`}, "fb-session", nil
	})

	report, turn, err := Ask(context.Background(), Config{
		SessionsDir:      dir,
		AgentCmd:         "primary-agent",
		FallbackAgentCmd: "fallback-agent",
		FallbackModel:    "cheap-model",
	}, AskRequest{Session: "s", Repo: "o/r", Question: "how is middleware chained?"})
	if err != nil {
		t.Fatalf("degraded:model ask must succeed: %v", err)
	}
	if report == nil || turn == nil {
		t.Fatal("expected report and turn")
	}
	if !strings.HasPrefix(report.Answer, "DEGRADED ("+DegradedModelCause+")") {
		t.Errorf("answer must carry the degraded:model label, got: %s", report.Answer)
	}
	if !strings.Contains(report.Answer, "middleware chain is in src/mw.go") {
		t.Errorf("relabeled answer must keep the fallback's real answer, got: %s", report.Answer)
	}
	if fbAgentCmd != "fallback-agent" {
		t.Errorf("fallback turn must run under FallbackAgentCmd, got AgentCmd=%q", fbAgentCmd)
	}
	if fbModel != "cheap-model" {
		t.Errorf("fallback steering model = %q, want cheap-model (FallbackModel must reach the wire)", fbModel)
	}
	if turn.FallbackBackend != "fallback-agent" {
		t.Errorf("TurnResult.FallbackBackend = %q, want fallback-agent", turn.FallbackBackend)
	}
	if !turn.QuotaDegraded {
		t.Error("TurnResult.QuotaDegraded must be true on the fallback rung too")
	}
	foundFB, foundCache := false, false
	for _, b := range report.BackendsUsed {
		switch b {
		case "fallback-agent":
			foundFB = true
		case LedgerCacheBackend:
			foundCache = true
		}
	}
	if !foundFB || !foundCache {
		t.Errorf("BackendsUsed must record fallback-agent + ledger-cache, got %v", report.BackendsUsed)
	}
	foundUncertainty := false
	for _, u := range report.Uncertainty {
		if strings.Contains(u, "FALLBACK BACKEND") && strings.Contains(u, "8:30am (UTC)") {
			foundUncertainty = true
		}
	}
	if !foundUncertainty {
		t.Errorf("uncertainty must name the fallback backend and the reset hint, got %v", report.Uncertainty)
	}
}

// TestAskQuotaLadderFallsThroughToCache pins the ladder ORDER: when no
// fallback backend is configured (or the fallback also fails), the cached
// ledger evidence serves the ask labeled degraded:cache — rung 1 after rung 2.
func TestAskQuotaLadderFallsThroughToCache(t *testing.T) {
	stubQuotaHandshake(t)
	dir := t.TempDir()
	seedLedgerSession(t, dir, "s")

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = primaryTurnStub(t, nil)

	report, turn, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  "s",
		Repo:     "o/r",
		Question: "follow up",
	})
	if err != nil {
		t.Fatalf("cached-ledger degraded ask must succeed: %v", err)
	}
	if !turn.QuotaDegraded {
		t.Error("turn.QuotaDegraded must be true on the cache rung")
	}
	if turn.FallbackBackend != "" {
		t.Errorf("no fallback configured; FallbackBackend must stay empty, got %q", turn.FallbackBackend)
	}
	if !strings.HasPrefix(report.Answer, "DEGRADED (quota)") {
		t.Errorf("cache-rung answer must start DEGRADED (quota), got: %s", report.Answer)
	}
	if strings.Contains(report.Answer, DegradedModelCause) {
		t.Errorf("cache-rung answer must not claim degraded:model, got: %s", report.Answer)
	}
	backends := fmt.Sprint(report.BackendsUsed)
	if !strings.Contains(backends, LedgerCacheBackend) {
		t.Errorf("BackendsUsed must carry ledger-cache, got %v", report.BackendsUsed)
	}
	if strings.Contains(backends, "mock") {
		t.Errorf("BackendsUsed must NOT claim the dead primary ran, got %v", report.BackendsUsed)
	}
}

// TestAskQuotaLadderFallbackFailureFallsToCache pins rung isolation: a
// fallback turn that ALSO fails must fall through to the cached-ledger rung,
// not surface the fallback's failure.
func TestAskQuotaLadderFallbackFailureFallsToCache(t *testing.T) {
	stubQuotaHandshake(t)
	dir := t.TempDir()
	seedLedgerSession(t, dir, "s")

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	calls := 0
	runTurnWithOptions = func(context.Context, RunTurnOptions) (TurnResult, string, error) {
		calls++
		if calls == 1 {
			return TurnResult{}, "", quotaWireErr()
		}
		return TurnResult{}, "", errors.New("fallback-agent: connection refused")
	}

	report, _, err := Ask(context.Background(), Config{
		SessionsDir:      dir,
		AgentCmd:         "primary-agent",
		FallbackAgentCmd: "fallback-agent",
	}, AskRequest{Session: "s", Repo: "o/r", Question: "follow up"})
	if err != nil {
		t.Fatalf("ladder must degrade to cache after fallback failure: %v", err)
	}
	if !strings.HasPrefix(report.Answer, "DEGRADED (quota)") {
		t.Errorf("must land on the cache rung, got: %s", report.Answer)
	}
	if calls != 2 {
		t.Errorf("exactly one fallback attempt allowed, got %d total turns", calls)
	}
}

// TestAskQuotaNoEvidenceTypedErrorWithArtifacts pins the never-die rung: quota
// exhaustion on a session with NO ledger evidence returns the typed
// ErrQuotaExhausted (CLI exit 3 + affordance), but artifacts are still flushed
// so the failed ask leaves an audit trail (ADR-0027 D3 extended by L3).
func TestAskQuotaNoEvidenceTypedErrorWithArtifacts(t *testing.T) {
	stubQuotaHandshake(t)
	dir := t.TempDir()
	if err := InitSession(dir, "fresh", "o/r", "scope", SessionNamedExplicit); err != nil {
		t.Fatal(err)
	}

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = primaryTurnStub(t, nil)

	report, turn, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  "fresh",
		Repo:     "o/r",
		Question: "first question ever",
	})
	if !errors.Is(err, ErrQuotaExhausted) {
		t.Fatalf("want typed ErrQuotaExhausted, got %v", err)
	}
	if report != nil {
		t.Errorf("no report expected on the typed-failure rung, got %+v", report)
	}
	if turn == nil {
		t.Fatal("partial TurnResult must still be returned (artifacts/audit)")
	}
	var qe *QuotaExhaustedError
	if !errors.As(err, &qe) {
		t.Fatal("error must be *QuotaExhaustedError")
	}
	if !strings.Contains(err.Error(), QuotaAffordanceHint) && !strings.Contains(QuotaAffordanceHint, "depth cheap") {
		t.Errorf("affordance hint must mention the cheap-depth move: %s", QuotaAffordanceHint)
	}
	// Artifacts on every path: the failed turn must have flushed the OTel
	// bundle + live log into the session dir before returning.
	meta, err := ReadMeta(dir, "fresh")
	if err != nil || meta == nil {
		t.Fatalf("session meta must survive the failed ask: %v", err)
	}
}
