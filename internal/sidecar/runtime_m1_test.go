package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestFailedTurnLiveLogRecordsRawError pins review finding M1
// (docs/audits/refactor-review-2026-07-07.md; folded into ADR-0036 C1): on the
// failed-turn path the live turn-log's turn.completed error field must record
// the RAW turn error, NOT the DiagnoseTurnError-enriched "run turn:"-prefixed /
// stderr-tail string the CLI caller receives. The T1.3 named-return + deferred-
// closure interaction had leaked the enriched string into the live log; the
// Runner-port rework restores the pre-T1.3 raw value via a distinct completedErr
// local. This is the one additive test the C1 refactor adds; every other test
// passes unmodified as the byte-identical proof.
func TestFailedTurnLiveLogRecordsRawError(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	rawErr := fmt.Errorf("acp prompt: %w", ErrLivenessTimeout)
	runTurnWithOptions = func(_ context.Context, _ RunTurnOptions) (TurnResult, string, error) {
		return TurnResult{FullText: "partial exploration before the hang"}, "sess-1", rawErr
	}

	_, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session: "s", Repo: "o/r", Question: "q",
	})
	if err == nil {
		t.Fatal("expected the failed turn to surface an error to the caller")
	}
	// The caller error IS the enriched form (DiagnoseTurnError prepends "run turn: ").
	if !strings.Contains(err.Error(), "run turn:") {
		t.Fatalf("caller error should be DiagnoseTurnError-enriched, got %q", err.Error())
	}

	// The live-log turn.completed event must carry the RAW turn error.
	got := lastTurnCompletedError(t, LiveLogPath(dir, "s"))
	want := rawErr.Error() // "acp prompt: sidecar liveness watchdog timeout"
	if got != want {
		t.Fatalf("live-log turn.completed error = %q, want the RAW turn error %q (M1: not the enriched string)", got, want)
	}
	if strings.Contains(got, "run turn:") {
		t.Fatalf("live-log error must not carry the enriched 'run turn:' prefix, got %q", got)
	}
}

// lastTurnCompletedError returns the error field of the last turn.completed
// event in a live.jsonl file.
func lastTurnCompletedError(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live log %s: %v", path, err)
	}
	found := false
	var errField string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev struct {
			Event string `json:"event"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Event == "turn.completed" {
			errField = ev.Error
			found = true
		}
	}
	if !found {
		t.Fatalf("no turn.completed event in %s", path)
	}
	return errField
}
