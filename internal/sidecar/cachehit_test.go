package sidecar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// readJSONLLines reads every line of a .jsonl artifact as raw bytes for
// per-line unmarshalling in tests.
func readJSONLLines(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out [][]byte
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, []byte(line))
		}
	}
	return out
}

// protojsonCount accepts an OTLP/protojson integer in either of its two wire
// shapes: a JSON string ("count":"1" — protojson serializes int64 as string)
// or a bare number. The 2026-07-05 protojson lesson: encoding/json rejects
// string→int, silently zeroing parsed datapoints.
type protojsonCount int64

func (c *protojsonCount) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "null" || s == "" {
		*c = 0
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*c = protojsonCount(v)
	return nil
}

func (c protojsonCount) Int64() int64 { return int64(c) }

// cacheHitSeedSession creates a warm session whose ledger + last report give
// the D1 gate everything it needs: durable ledger evidence, TurnCount ≥ 1,
// and an answered prior report whose claims can ride through as [cached].
func cacheHitSeedSession(t *testing.T, dir, name string) {
	t.Helper()
	if err := InitSession(dir, name, "o/r", "middleware", SessionNamedExplicit); err != nil {
		t.Fatal(err)
	}
	if err := SaveLedger(dir, name, &Ledger{
		Repo:           "o/r",
		Scope:          "middleware",
		InspectedPaths: []LedgerEntry{{Value: "src/mw.go", Turn: 1}},
		CommandsRun:    []LedgerEntry{{Value: "ghx read o/r src/mw.go", Turn: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveTurnReportArtifact(dir, name, 1, ReportArtifact{Report: &Report{
		Answer:        "the middleware chain is built in src/mw.go",
		Verified:      []Claim{{Summary: "chain lives in src/mw.go", Evidence: "src/mw.go:10"}},
		RelevantFiles: []RelevantFile{{Path: "src/mw.go", Reason: "core chain"}},
		BackendsUsed:  []string{"remote"},
		CommandsRun:   []string{"ghx read o/r src/mw.go"},
	}}); err != nil {
		t.Fatal(err)
	}
	// Turn 1 is the exploration turn that produced the seeded report; the
	// meta counter must reflect it or the gate's warm-session check misses.
	if err := RecordTurn(dir, name); err != nil {
		t.Fatal(err)
	}
}

// answeredTurnStub returns a runTurnWithOptions stub that fails the test if the
// runtime spawns ANY model turn, and otherwise returns a normal answered turn.
func answeredTurnStub(t *testing.T) func(context.Context, RunTurnOptions) (TurnResult, string, error) {
	t.Helper()
	return func(context.Context, RunTurnOptions) (TurnResult, string, error) {
		t.Fatal("model turn must not run when the cache-hit gate admits the ask")
		return TurnResult{}, "", nil
	}
}

// TestEvaluateCacheHitGateTable pins the D1 admission conditions one by one
// (ADR-0040.1 D1): every miss names its condition; only the full warm-covered
// shape admits. The threshold is the same RouteConfigFor number that governs
// R4 session joining.
func TestEvaluateCacheHitGateTable(t *testing.T) {
	dir := t.TempDir()
	if err := InitSession(dir, "s", "o/r", "middleware", SessionNamedExplicit); err != nil {
		t.Fatal(err)
	}
	meta, err := ReadMeta(dir, "s")
	if err != nil || meta == nil {
		t.Fatalf("read meta: %v", err)
	}
	warm := *meta
	warm.TurnCount = 1
	cold := *meta // TurnCount == 0

	warmLedger := &Ledger{
		Repo:           "o/r",
		Scope:          "middleware",
		InspectedPaths: []LedgerEntry{{Value: "src/mw.go", Turn: 1}},
		CommandsRun:    []LedgerEntry{{Value: "ghx read o/r src/mw.go", Turn: 1}},
	}
	emptyLedger := &Ledger{Repo: "o/r", Scope: "middleware"}

	coveredReport := &Report{Answer: "the middleware chain is built in src/mw.go", Verified: []Claim{{Summary: "chain", Evidence: "src/mw.go:10"}}}
	blockedReport := &Report{Answer: "BLOCKED: ghx unavailable in this sidecar session."}
	degradedReport := &Report{Answer: "DEGRADED (quota): answered from cached session evidence"}

	cfg := Config{}
	ask := func(q string) AskRequest {
		return AskRequest{Session: "s", Repo: "o/r", Question: q, Depth: "cheap"}
	}

	// Covered question: middleware vocabulary rides the ledger (scope +
	// inspected path segments), so overlap must clear the routing threshold.
	gate, ok := evaluateCacheHit(cfg, ask("how is the middleware chain built?"), &warm, warmLedger, coveredReport)
	if !ok {
		t.Fatalf("covered warm cheap ask must be admitted; got gate %+v (threshold %v)", gate, RouteConfigFor(cfg).OverlapThreshold)
	}
	if gate.Score < RouteConfigFor(cfg).OverlapThreshold {
		t.Fatalf("admitted gate score %.2f below threshold %.2f", gate.Score, gate.Threshold)
	}

	cases := []struct {
		name   string
		req    AskRequest
		meta   *SessionMeta
		ledger *Ledger
		report *Report
	}{
		{"normal depth always explores", AskRequest{Session: "s", Repo: "o/r", Question: "how is the middleware chain built?", Depth: "normal"}, &warm, warmLedger, coveredReport},
		{"deep depth always explores", AskRequest{Session: "s", Repo: "o/r", Question: "how is the middleware chain built?", Depth: "deep"}, &warm, warmLedger, coveredReport},
		{"cold session never serves", ask("how is the middleware chain built?"), &cold, warmLedger, coveredReport},
		{"empty ledger never serves", ask("how is the middleware chain built?"), &warm, emptyLedger, coveredReport},
		{"nil ledger never serves", ask("how is the middleware chain built?"), &warm, nil, coveredReport},
		{"nil lastReport never serves", ask("how is the middleware chain built?"), &warm, warmLedger, nil},
		{"BLOCKED prior never serves", ask("how is the middleware chain built?"), &warm, warmLedger, blockedReport},
		{"DEGRADED prior never serves", ask("how is the middleware chain built?"), &warm, warmLedger, degradedReport},
		{"uncovered question explores", ask("what license does the repo use?"), &warm, warmLedger, coveredReport},
	}
	for _, tc := range cases {
		if gate, ok := evaluateCacheHit(cfg, tc.req, tc.meta, tc.ledger, tc.report); ok {
			t.Errorf("%s: must NOT be admitted (gate %+v)", tc.name, gate)
		}
	}
}

// TestAskCacheHitFastPathServes pins the end-to-end D1 behavior: a covered
// cheap ask on a warm session is served from cached evidence with NO model
// turn, labeled DEGRADED (cache-hit) with BackendsUsed=[ledger-cache], the
// gate decision recorded in uncertainty, claims relabeled [cached], and the
// same bookkeeping as a real turn (persisted report artifact + TurnResult
// flag). A cache hit is a measured turn, not a bypass of measurement.
func TestAskCacheHitFastPathServes(t *testing.T) {
	stubQuotaHandshake(t)
	dir := t.TempDir()
	cacheHitSeedSession(t, dir, "s")

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = answeredTurnStub(t)

	report, turn, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  "s",
		Repo:     "o/r",
		Question: "how is the middleware chain built?",
		Depth:    "cheap",
	})
	if err != nil {
		t.Fatalf("cache-hit ask must succeed: %v", err)
	}
	if report == nil || turn == nil {
		t.Fatal("expected report and turn")
	}
	if !turn.CacheHit {
		t.Error("TurnResult.CacheHit must be true on the fast path")
	}
	if turn.QuotaDegraded {
		t.Error("TurnResult.QuotaDegraded must stay false — cache-hit is a latency choice, not a quota failure")
	}
	if !strings.HasPrefix(report.Answer, "DEGRADED (cache-hit)") {
		t.Errorf("answer must carry the DEGRADED (cache-hit) label, got: %s", report.Answer)
	}
	if !strings.Contains(report.Answer, "middleware chain is built in src/mw.go") {
		t.Errorf("answer must keep the prior answer's substance, got: %s", report.Answer)
	}
	backends := strings.Join(report.BackendsUsed, ",")
	if backends != LedgerCacheBackend {
		t.Errorf("BackendsUsed must be exactly [%s], got %v", LedgerCacheBackend, report.BackendsUsed)
	}
	if len(report.Verified) == 0 {
		t.Fatal("cached claims must ride through as verified")
	}
	for _, c := range report.Verified {
		if !strings.HasPrefix(c.Evidence, "[cached] ") {
			t.Errorf("every carried claim must be labeled [cached], got: %q", c.Evidence)
		}
	}
	foundGate := false
	for _, u := range report.Uncertainty {
		if strings.Contains(u, "cache-hit gate") && strings.Contains(u, "--depth normal") {
			foundGate = true
		}
	}
	if !foundGate {
		t.Errorf("uncertainty must record the auditable gate decision + fresh-recon affordance, got %v", report.Uncertainty)
	}
	if report.SchemaVersion != ReportSchemaVersion {
		t.Errorf("schemaVersion must be stamped %q, got %q", ReportSchemaVersion, report.SchemaVersion)
	}
	if len(report.RelevantFiles) == 0 {
		t.Error("ledger inspected paths must ride through as relevantFiles")
	}
	// Measured-turn bookkeeping: the served report must be persisted as a
	// report artifact and the turn counter must have advanced.
	reports, err := ListReports(dir, "s")
	if err != nil || len(reports) != 2 {
		t.Fatalf("cache-hit turn must persist a second report artifact, got %d reports (err %v)", len(reports), err)
	}
	meta, err := ReadMeta(dir, "s")
	if err != nil || meta == nil {
		t.Fatalf("read meta after cache hit: %v", err)
	}
	if meta.TurnCount != 2 { // seeded turn 1 + this cache-hit turn
		t.Errorf("TurnCount must advance on the cache-hit turn, got %d", meta.TurnCount)
	}
	// The weekly p50 must see cache-hit turns (ADR-0040.1 D1): even a
	// sub-millisecond fast-path turn emits its operation-duration datapoint.
	// The file export is protojson: int64 fields serialize as strings
	// ("count":"1"), so the parser must accept both shapes — encoding/json
	// rejects string→int and would silently zero every datapoint.
	datapoints := 0
	for _, line := range readJSONLLines(t, filepath.Join(dir, "s", "metrics.jsonl")) {
		var m struct {
			ResourceMetrics []struct {
				ScopeMetrics []struct {
					Metrics []struct {
						Name      string `json:"name"`
						Histogram *struct {
							DataPoints []struct {
								Count protojsonCount `json:"count"`
							} `json:"dataPoints"`
						} `json:"histogram"`
					} `json:"metrics"`
				} `json:"scopeMetrics"`
			} `json:"resourceMetrics"`
		}
		if json.Unmarshal(line, &m) != nil {
			continue
		}
		for _, rm := range m.ResourceMetrics {
			for _, sm := range rm.ScopeMetrics {
				for _, met := range sm.Metrics {
					if met.Name != "gen_ai.client.operation.duration" || met.Histogram == nil {
						continue
					}
					for _, dp := range met.Histogram.DataPoints {
						if dp.Count.Int64() > 0 {
							datapoints++
						}
					}
				}
			}
		}
	}
	if datapoints != 1 { // exactly this cache-hit turn; seeding writes no metrics line
		t.Errorf("cache-hit turn must emit an operation-duration datapoint (weekly p50 must see it), got %d", datapoints)
	}
}

// TestAskCacheHitMissExplores pins the conservative-miss side of D1: a fresh
// (uncovered) cheap question on the same warm session must proceed exactly as
// today — a real model turn, no cache-hit labeling.
func TestAskCacheHitMissExplores(t *testing.T) {
	stubQuotaHandshake(t)
	dir := t.TempDir()
	cacheHitSeedSession(t, dir, "s")

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	turned := false
	runTurnWithOptions = func(context.Context, RunTurnOptions) (TurnResult, string, error) {
		turned = true
		return TurnResult{FullText: `<ghx-report>{"answer":"the license file is LICENSE-MIT","verified":[{"summary":"LICENSE-MIT","evidence":"LICENSE-MIT:1"}],"relevantFiles":[{"path":"LICENSE-MIT"}],"backendsUsed":["remote"],"commandsRun":["ghx read o/r LICENSE-MIT"]}</ghx-report>`}, "sess", nil
	}

	report, turn, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  "s",
		Repo:     "o/r",
		Question: "what open-source license does this repository ship under?",
		Depth:    "cheap",
	})
	if err != nil {
		t.Fatalf("cache-miss ask must proceed as a normal turn: %v", err)
	}
	if !turned {
		t.Fatal("a cache miss must spawn a real model turn")
	}
	if turn.CacheHit {
		t.Error("TurnResult.CacheHit must stay false on a miss")
	}
	if strings.HasPrefix(report.Answer, "DEGRADED") {
		t.Errorf("a miss must answer fresh, got: %s", report.Answer)
	}
}

// TestBuildCacheHitReportNestedDegradedLabel pins the nested-label guard: when
// the prior answer is itself a DEGRADED report (a previous cache-hit turn
// became lastReport), the new label wraps the SUBSTANCE, never stacks a second
// DEGRADED prefix inside the first.
func TestBuildCacheHitReportNestedDegradedLabel(t *testing.T) {
	dir := t.TempDir()
	cacheHitSeedSession(t, dir, "s")
	meta, err := ReadMeta(dir, "s")
	if err != nil || meta == nil {
		t.Fatalf("read meta: %v", err)
	}
	meta.TurnCount = 2
	ledger, err := LoadLedger(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	state := &askTurnState{
		req:        AskRequest{Session: "s", Repo: "o/r", Question: "how is the middleware chain built?", Depth: "cheap"},
		meta:       meta,
		ledger:     ledger,
		lastReport: &Report{Answer: "DEGRADED (cache-hit): answered from cached session evidence; the middleware chain is built in src/mw.go"},
	}
	gate := cacheHitGate{Score: 1.5, Threshold: 0.5}
	report := buildCacheHitReport(state, gate)
	if strings.HasPrefix(strings.TrimPrefix(report.Answer, "DEGRADED (cache-hit):"), "DEGRADED") {
		t.Errorf("nested DEGRADED prefix must be unwrapped, got: %s", report.Answer)
	}
	if got := strings.Count(report.Answer, "DEGRADED"); got != 1 {
		t.Errorf("exactly one DEGRADED label expected, got %d in: %s", got, report.Answer)
	}
	if !strings.Contains(report.Answer, "middleware chain is built in src/mw.go") {
		t.Errorf("substance must survive the unwrap, got: %s", report.Answer)
	}
}
