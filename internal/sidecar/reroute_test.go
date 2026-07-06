package sidecar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// askFakeTurn runs one Ask against session with a stubbed runner that
// produces a fixed report plus tool traces — the full live artifact path
// (ledger update, per-turn report artifact, telemetry).
func askFakeTurn(t *testing.T, dir, session, question string, report string, commands []string) {
	t.Helper()
	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		var traces []ToolCallTrace
		for i, c := range commands {
			traces = append(traces, ToolCallTrace{
				ID:       "call-" + question + string(rune('a'+i)),
				Kind:     "execute",
				RawInput: map[string]any{"command": c},
			})
		}
		return TurnResult{
			FullText:   "<ghx-report>" + report + "</ghx-report>",
			ToolTraces: traces,
		}, "acp-session-1", nil
	}
	if _, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  session,
		Question: question,
	}); err != nil {
		t.Fatal(err)
	}
}

func normalizedLedger(t *testing.T, l *Ledger) *Ledger {
	t.Helper()
	cp := *l
	cp.UpdatedAt = "" // the D5 invariant is modulo UpdatedAt
	return &cp
}

// TestLedgerRebuildReproducesLiveLedger is the ADR-0030.1 D5 invariant test:
// replaying a session's reports/ artifacts must reproduce its ledger.json
// (modulo UpdatedAt). Runs the real Ask path twice so the artifacts carry
// everything ledger derivation consumed (reports + trace-derived commands).
func TestLedgerRebuildReproducesLiveLedger(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	askFakeTurn(t, dir, "s", "first question",
		`{"answer":"a1","relevantFiles":[{"path":"pool/pool.go","reason":"core"}],"commandsRun":["ghx read o/r pool/pool.go"],"uncertainty":["is panic handling covered"],"nextReads":["pool/panics.go"]}`,
		[]string{"ghx read o/r pool/pool.go", "ghx search o/r --grep WaitGroup"})
	askFakeTurn(t, dir, "s", "second question",
		`{"answer":"a2","relevantFiles":[{"path":"pool/panics.go","reason":"panic path"}],"commandsRun":["ghx read o/r pool/panics.go"],"uncertainty":[],"nextReads":["docs/DESIGN.md"]}`,
		[]string{"ghx read o/r pool/panics.go"})

	live, err := LoadLedger(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if len(live.CommandsRun) == 0 || len(live.OpenQuestions) == 0 || len(live.RelevantFiles) != 2 {
		t.Fatalf("live ledger not populated as expected: %+v", live)
	}

	rebuilt, err := RebuildLedger(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalizedLedger(t, live), normalizedLedger(t, rebuilt)) {
		liveJSON, _ := json.MarshalIndent(normalizedLedger(t, live), "", "  ")
		rebuiltJSON, _ := json.MarshalIndent(normalizedLedger(t, rebuilt), "", "  ")
		t.Fatalf("rebuilt ledger diverges from live ledger\nlive:\n%s\nrebuilt:\n%s", liveJSON, rebuiltJSON)
	}

	// Determinism: rebuilding twice yields identical structures.
	again, err := RebuildLedger(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(rebuilt, again) {
		t.Fatal("RebuildLedger is not deterministic across runs")
	}
}

// TestRerouteTurnMovesArtifactAndRebuildsBothLedgers exercises the D5
// recovery path end to end: a stray turn moves to another session, both
// ledgers are rebuilt by replay, and the source's ACP session is retired.
func TestRerouteTurnMovesArtifactAndRebuildsBothLedgers(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	// Session A gets a legitimate turn plus a stray turn (the mis-route).
	askFakeTurn(t, dir, "session-a", "a question about pools",
		`{"answer":"a1","relevantFiles":[{"path":"pool/pool.go","reason":"core"}],"commandsRun":["ghx read o/r pool/pool.go"],"uncertainty":["panic handling"],"nextReads":[]}`,
		[]string{"ghx read o/r pool/pool.go"})
	askFakeTurn(t, dir, "session-a", "a stray question about routing",
		`{"answer":"stray","relevantFiles":[{"path":"router/route.go","reason":"stray"}],"commandsRun":["ghx read x/y router/route.go"],"uncertainty":["stray uncertainty"],"nextReads":["router/group.go"]}`,
		[]string{"ghx read x/y router/route.go"})
	// Session B exists with one turn of its own.
	askFakeTurn(t, dir, "session-b", "b question",
		`{"answer":"b1","relevantFiles":[{"path":"router/tree.go","reason":"radix"}],"commandsRun":["ghx read x/y router/tree.go"],"uncertainty":[],"nextReads":[]}`,
		[]string{"ghx read x/y router/tree.go"})

	res, err := RerouteTurn(dir, "session-a", 2, "session-b")
	if err != nil {
		t.Fatal(err)
	}
	if res.From != "session-a" || res.To != "session-b" || res.Turn != 2 || res.NewTurn != 2 {
		t.Fatalf("unexpected reroute result: %+v", res)
	}
	if len(res.MovedReports) != 1 {
		t.Fatalf("moved reports = %+v", res.MovedReports)
	}

	// The moved artifact lives in B with reroute provenance; A keeps turn 1.
	art, err := ReadReportArtifact(res.MovedReports[0])
	if err != nil {
		t.Fatal(err)
	}
	if art.Rerouted == nil || art.Rerouted.FromSession != "session-a" || art.Rerouted.FromTurn != 2 {
		t.Fatalf("moved artifact missing reroute provenance: %+v", art.Rerouted)
	}
	if art.Report == nil || art.Report.Answer != "stray" {
		t.Fatalf("moved artifact carries wrong report: %+v", art.Report)
	}
	aFiles, err := listTurnReportArtifacts(dir, "session-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(aFiles) != 1 || aFiles[0].Turn != 1 {
		t.Fatalf("source artifacts after reroute = %+v", aFiles)
	}

	// Both ledgers equal their deterministic rebuilds (the tested invariant),
	// the stray evidence left A and arrived in B.
	for _, name := range []string{"session-a", "session-b"} {
		onDisk, err := LoadLedger(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		rebuilt, err := RebuildLedger(dir, name)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(normalizedLedger(t, onDisk), normalizedLedger(t, rebuilt)) {
			t.Fatalf("%s: on-disk ledger != rebuild-by-replay", name)
		}
	}
	aLedger, _ := LoadLedger(dir, "session-a")
	if containsLedgerValue(aLedger.OpenQuestions, "stray uncertainty") {
		t.Fatalf("stray evidence still in source ledger: %+v", aLedger.OpenQuestions)
	}
	bLedger, _ := LoadLedger(dir, "session-b")
	if !containsLedgerValue(bLedger.OpenQuestions, "stray uncertainty") {
		t.Fatalf("stray evidence missing from destination ledger: %+v", bLedger.OpenQuestions)
	}
	// The moved turn's entries carry the destination turn number (provenance
	// via the artifact's Rerouted field, numbering via the new home).
	for _, e := range bLedger.OpenQuestions {
		if e.Value == "stray uncertainty" && e.Turn != 2 {
			t.Fatalf("moved entry turn = %d, want 2", e.Turn)
		}
	}

	// Source meta: ACP session retired (ADR-0027 stale path on next turn),
	// turn counter re-anchored on surviving artifacts.
	aMeta, err := ReadMeta(dir, "session-a")
	if err != nil {
		t.Fatal(err)
	}
	if aMeta.ACPSessionID != "" {
		t.Fatalf("source ACPSessionID not cleared: %q", aMeta.ACPSessionID)
	}
	if aMeta.TurnCount != 1 {
		t.Fatalf("source TurnCount = %d, want 1", aMeta.TurnCount)
	}
	bMeta, err := ReadMeta(dir, "session-b")
	if err != nil {
		t.Fatal(err)
	}
	if bMeta.TurnCount != 2 {
		t.Fatalf("destination TurnCount = %d, want 2", bMeta.TurnCount)
	}
}

// TestRerouteTurnToNewSession pins that rerouting into a nonexistent session
// creates it (the "--to new thread" shape) and rejects no-op/self moves.
func TestRerouteTurnToNewSession(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	askFakeTurn(t, dir, "session-a", "only question",
		`{"answer":"a1","relevantFiles":[],"commandsRun":[],"uncertainty":["u1"],"nextReads":[]}`,
		nil)

	if _, err := RerouteTurn(dir, "session-a", 1, "session-a"); err == nil {
		t.Fatal("self-reroute must fail")
	}
	if _, err := RerouteTurn(dir, "session-a", 7, "fresh"); err == nil ||
		!strings.Contains(err.Error(), "no report artifact for turn 7") {
		t.Fatalf("missing-turn error = %v", err)
	}

	res, err := RerouteTurn(dir, "session-a", 1, "fresh-thread")
	if err != nil {
		t.Fatal(err)
	}
	if !res.ToCreated || res.NewTurn != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	meta, err := ReadMeta(dir, "fresh-thread")
	if err != nil || meta == nil {
		t.Fatalf("destination session not created: %v %v", meta, err)
	}
	// A reroute target is operator-directed: it must not become an auto-join
	// candidate by accident.
	if meta.NamedBy != SessionNamedExplicit {
		t.Fatalf("destination NamedBy = %q, want explicit", meta.NamedBy)
	}
	ledger, err := LoadLedger(dir, "fresh-thread")
	if err != nil {
		t.Fatal(err)
	}
	if !containsLedgerValue(ledger.OpenQuestions, "u1") {
		t.Fatalf("destination ledger missing replayed evidence: %+v", ledger)
	}
}

// TestReportArtifactCarriesTraceCommands pins the artifact enrichment the D5
// invariant rests on: the exact trace-derived command strings the ledger
// consumed are persisted on the per-turn artifact.
func TestReportArtifactCarriesTraceCommands(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	askFakeTurn(t, dir, "s", "q",
		`{"answer":"a","relevantFiles":[],"commandsRun":[],"uncertainty":[],"nextReads":[]}`,
		[]string{"ghx read o/r main.go", "ghx search o/r --grep Router"})

	files, err := listTurnReportArtifacts(dir, "s")
	if err != nil || len(files) != 1 {
		t.Fatalf("artifacts = %+v, err %v", files, err)
	}
	art, err := ReadReportArtifact(files[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ghx read o/r main.go", "ghx search o/r --grep Router"}
	if !reflect.DeepEqual(art.TraceCommands, want) {
		t.Fatalf("TraceCommands = %+v, want %+v", art.TraceCommands, want)
	}
}

func containsLedgerValue(entries []LedgerEntry, value string) bool {
	for _, e := range entries {
		if e.Value == value {
			return true
		}
	}
	return false
}

// TestRouteRecordInSessionLogs pins D7 end to end on the artifact surface: a
// repo-less ask writes a sidecar.route record with the routing attributes
// into the session's logs.jsonl.
func TestRouteRecordInSessionLogs(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()
	askFakeTurn(t, dir, "", "which repos implement acp agents", // Session empty: routed (R5)
		`{"answer":"a","relevantFiles":[],"commandsRun":[],"uncertainty":[],"nextReads":[]}`,
		nil)

	session := QuestionSlug("which repos implement acp agents")
	data, err := os.ReadFile(filepath.Join(dir, session, "logs.jsonl"))
	if err != nil {
		t.Fatalf("read logs.jsonl for routed session %s: %v", session, err)
	}
	logs := string(data)
	for _, want := range []string{"sidecar.route", "ghx.sidecar.route.source", `"new"`} {
		if !strings.Contains(logs, want) {
			t.Fatalf("route record missing %q in logs:\n%s", want, logs)
		}
	}
	meta, err := ReadMeta(dir, session)
	if err != nil {
		t.Fatal(err)
	}
	if meta.NamedBy != SessionNamedQuestion {
		t.Fatalf("routed new session NamedBy = %q, want question", meta.NamedBy)
	}
}
