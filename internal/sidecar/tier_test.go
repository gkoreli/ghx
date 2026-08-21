package sidecar

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// Unit tests for the pre-registered v1 observation/tier derivations
// (ADR-0024.2 D2/D4) plus the Ask-path integration for decision recording
// (D3): tier-decisions.jsonl, the ghx.tier.decision span, and tierUsed
// round-trip through the report.

func TestCountSearchesWithoutAcceptedRead(t *testing.T) {
	cases := []struct {
		name     string
		commands []string
		want     int
	}{
		{"no commands", nil, 0},
		{"search then read", []string{`ghx search "repo:o/r x"`, "ghx read o/r a.go"}, 0},
		{"search then inspect", []string{`ghx search "repo:o/r x"`, `ghx inspect o/r "x"`}, 0},
		{"trailing search", []string{"ghx read o/r a.go", `ghx search "repo:o/r x"`}, 1},
		{"two dead searches", []string{`ghx search "repo:o/r x"`, `ghx search "repo:o/r y"`}, 2},
		{"dead search then accepted", []string{`ghx search "x"`, `ghx search "y"`, "ghx read o/r a.go"}, 1},
		{"explore does not accept", []string{`ghx search "x"`, "ghx explore o/r"}, 1},
		{"path-qualified ghx", []string{`./ghx search "x"`, `/usr/local/bin/ghx search "y"`}, 2},
		{"non-ghx ignored", []string{"grep -r x .", "ls"}, 0},
	}
	for _, tc := range cases {
		if got := countSearchesWithoutAcceptedRead(tc.commands); got != tc.want {
			t.Fatalf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestDeriveTierUsed(t *testing.T) {
	cases := []struct {
		name     string
		commands []string
		want     string
	}{
		{"no commands", nil, "tier0"},
		{"non-ghx only", []string{"ls -la"}, "tier0"},
		{"remote ghx", []string{"ghx read o/r a.go"}, "tier1"},
		{"tier2 command", []string{"ghx read o/r a.go", "ghx tier2 codemap o/r --importers a.go"}, "tier2"},
		{"path-qualified tier2", []string{"./ghx tier2 repomap o/r --query x"}, "tier2"},
	}
	for _, tc := range cases {
		if got := deriveTierUsed(tc.commands); got != tc.want {
			t.Fatalf("%s: got %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestLowConfidenceRemoteOnly(t *testing.T) {
	uncertain := &Report{Answer: "x", Uncertainty: []string{"could not verify"}, BackendsUsed: []string{"remote"}}
	if !lowConfidenceRemoteOnly(uncertain, false) {
		t.Fatal("uncertain remote-only report must fire")
	}
	if lowConfidenceRemoteOnly(uncertain, true) {
		t.Fatal("escalation already used must not fire remote-only")
	}
	local := &Report{Answer: "x", Uncertainty: []string{"u"}, BackendsUsed: []string{"remote", "local:codemap"}}
	if lowConfidenceRemoteOnly(local, false) {
		t.Fatal("local backend in report must not fire remote-only")
	}
	confident := &Report{Answer: "x", BackendsUsed: []string{"remote"}}
	if lowConfidenceRemoteOnly(confident, false) {
		t.Fatal("no uncertainty must not fire")
	}
	if lowConfidenceRemoteOnly(nil, false) {
		t.Fatal("nil report must not fire")
	}
}

// TestBuildTierDecisionRecordBackfillAndPreserve pins ADR-0024.2 D4: an empty
// report tierUsed is backfilled with the runtime-derived tier (labeled
// "runtime"); an agent-declared value is never overwritten and divergence
// stays visible via runtimeDerivedTierUsed.
func TestBuildTierDecisionRecordBackfillAndPreserve(t *testing.T) {
	req := AskRequest{Session: "s", Repo: "o/r", Question: "what imports compose?", AllowedBackends: []string{"remote", "local"}}
	result := &TurnResult{ToolTraces: []ToolCallTrace{
		{ID: "1", RawInput: "ghx tier2 codemap o/r --importers src/compose.ts",
			OutputExcerpt: "tier2 snapshot: repo=o/r ref=HEAD sha=abc123\ntier2 clone: strategy=shallow-blobless cache=hit path=/tmp/snap\n{}"},
	}}

	report := &Report{Answer: "a"}
	rec := buildTierDecisionRecord(req, 1, result, report, time.Now())
	if report.TierUsed != "tier2" {
		t.Fatalf("backfilled tierUsed = %q, want tier2", report.TierUsed)
	}
	if rec.TierUsed != "tier2" || rec.TierUsedSource != "runtime" || rec.RuntimeDerivedTierUsed != "tier2" {
		t.Fatalf("record = %+v", rec)
	}
	if !rec.EscalationUsed || !rec.Decision.Allowed {
		t.Fatalf("expected allowed escalation used: %+v", rec)
	}
	if rec.Clone == nil || rec.Clone.SHA != "abc123" || !rec.Clone.CacheHit {
		t.Fatalf("clone provenance not parsed: %+v", rec.Clone)
	}

	declared := &Report{Answer: "a", TierUsed: "tier1"}
	rec = buildTierDecisionRecord(req, 2, result, declared, time.Now())
	if declared.TierUsed != "tier1" {
		t.Fatalf("agent-declared tierUsed overwritten to %q", declared.TierUsed)
	}
	if rec.TierUsed != "tier1" || rec.TierUsedSource != "agent" || rec.RuntimeDerivedTierUsed != "tier2" {
		t.Fatalf("divergence not recorded: %+v", rec)
	}
}

// TestAskRecordsTierDecision drives the real Ask path (ADR-0024.2 D3): the
// turn's decision lands in tier-decisions.jsonl, the ghx.tier.decision span
// carries the pre-registered attributes (clone attrs included when tier2
// provenance is visible), and the returned report carries backfilled
// tierUsed provenance.
func TestAskRecordsTierDecision(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, opts RunTurnOptions) (TurnResult, string, error) {
		now := time.Now().UTC()
		return TurnResult{
			FullText: `<ghx-report>{"answer":"compose.ts is imported by hono-base and combine middleware","verified":[],"backendsUsed":["remote","local:codemap"],"commandsRun":["ghx tier2 codemap o/r --importers src/compose.ts"]}</ghx-report>`,
			ToolTraces: []ToolCallTrace{{
				ID: "call-1", Kind: "execute", Title: "ghx tier2 codemap",
				RawInput:      "ghx tier2 codemap o/r --importers src/compose.ts",
				OutputExcerpt: "tier2 snapshot: repo=o/r ref=HEAD sha=deadbeef\ntier2 clone: strategy=shallow-blobless cache=hit path=/tmp/snap\nfan-in: 3 dependents",
				StatusTransitions: []ToolStatusTransition{
					{Status: "in_progress", At: now},
					{Status: "completed", At: now.Add(time.Millisecond)},
				},
			}},
		}, "sess-1", nil
	}

	report, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:         "s",
		Repo:            "o/r",
		Question:        "what imports compose?",
		AllowedBackends: []string{"remote", "local"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// tierUsed round-trip: runtime backfill visible on the returned report.
	if report.TierUsed != "tier2" {
		t.Fatalf("report.TierUsed = %q, want tier2", report.TierUsed)
	}

	// tier-decisions.jsonl: one record, recomputable decision.
	rec := readTierDecisions(t, filepath.Join(dir, "s", "tier-decisions.jsonl"), 1)[0]
	if rec.Decision.PolicyVersion != tier2.PolicyVersionV1 {
		t.Fatalf("policyVersion = %q", rec.Decision.PolicyVersion)
	}
	if !rec.Decision.Allowed || !rec.EscalationUsed || rec.TierUsed != "tier2" || rec.TierUsedSource != "runtime" {
		t.Fatalf("record = %+v", rec)
	}
	if !containsString(rec.Decision.FiredSignals, tier2.SignalCrossFileDependency) {
		t.Fatalf("firedSignals = %v", rec.Decision.FiredSignals)
	}
	if rec.Turn != 1 || rec.Session != "s" || rec.Repo != "o/r" {
		t.Fatalf("record scope = %+v", rec)
	}
	if rec.Clone == nil || rec.Clone.SHA != "deadbeef" {
		t.Fatalf("clone = %+v", rec.Clone)
	}

	// Saved report artifact carries the backfilled provenance too.
	artifactPaths, err := ListReports(dir, "s")
	if err != nil || len(artifactPaths) == 0 {
		t.Fatalf("no report artifacts: %v", err)
	}
	data, err := os.ReadFile(artifactPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	var artifact ReportArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Report.TierUsed != "tier2" {
		t.Fatalf("persisted report tierUsed = %q", artifact.Report.TierUsed)
	}

	// ghx.tier.decision span with the pre-registered attributes.
	attrs := findSpanAttrs(t, filepath.Join(dir, "s", "traces.jsonl"), "ghx.tier.decision")
	wantStr := map[string]string{
		"ghx.tier.policy.version": tier2.PolicyVersionV1,
		"ghx.tier.from":           "tier1",
		"ghx.tier.to":             "tier2",
		"ghx.tier.used":           "tier2",
		"ghx.tier.used_source":    "runtime",
		"ghx.repo.full_name":      "o/r",
		"ghx.repo.ref":            "HEAD",
		"ghx.repo.sha":            "deadbeef",
		"ghx.clone.strategy":      "shallow-blobless",
	}
	for key, want := range wantStr {
		if got, _ := attrs[key].(string); got != want {
			t.Fatalf("span attr %s = %v, want %q (attrs: %v)", key, attrs[key], want, attrs)
		}
	}
	for _, key := range []string{"ghx.tier.allowed", "ghx.tier.escalation_used", "ghx.clone.cache_hit"} {
		if got, _ := attrs[key].(bool); !got {
			t.Fatalf("span attr %s = %v, want true", key, attrs[key])
		}
	}
	if _, ok := attrs["ghx.tier.signals"]; !ok {
		t.Fatalf("span missing ghx.tier.signals: %v", attrs)
	}
}

// TestAskRecordsTierDecisionOnFailedTurn pins the ADR-0027/0024.2 corner:
// even an unrecovered turn failure appends a tier decision record.
func TestAskRecordsTierDecisionOnFailedTurn(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, _ RunTurnOptions) (TurnResult, string, error) {
		return TurnResult{}, "", errors.New("dead peer")
	}

	_, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:  "s",
		Repo:     "o/r",
		Question: "how are routes registered?",
	})
	if err == nil {
		t.Fatal("expected turn failure")
	}
	rec := readTierDecisions(t, filepath.Join(dir, "s", "tier-decisions.jsonl"), 1)[0]
	if rec.Decision.Allowed || rec.EscalationUsed {
		t.Fatalf("failed empty turn must not show escalation: %+v", rec)
	}
	if rec.TierUsed != "tier0" || rec.TierUsedSource != "runtime" {
		t.Fatalf("record = %+v", rec)
	}
}

// TestAskBlockedEscalationDecision pins the blocked shape: a hard-signal
// question without the local grant records allowed=false with a reason
// naming the blocked backend.
func TestAskBlockedEscalationDecision(t *testing.T) {
	stubHandshake(t)
	dir := t.TempDir()

	old := runTurnWithOptions
	defer func() { runTurnWithOptions = old }()
	runTurnWithOptions = func(_ context.Context, _ RunTurnOptions) (TurnResult, string, error) {
		return TurnResult{
			FullText: `<ghx-report>{"answer":"cannot verify importers remotely","uncertainty":["local:codemap needed for importers"],"backendsUsed":["remote"],"commandsRun":["ghx search x"]}</ghx-report>`,
			ToolTraces: []ToolCallTrace{{
				ID: "c1", RawInput: `ghx search "repo:o/r compose"`,
				StatusTransitions: []ToolStatusTransition{{Status: "completed", At: time.Now().UTC()}},
			}},
		}, "sess-1", nil
	}

	report, _, err := Ask(context.Background(), Config{SessionsDir: dir, AgentCmd: "mock"}, AskRequest{
		Session:         "s",
		Repo:            "o/r",
		Question:        "what imports compose?",
		AllowedBackends: []string{"remote"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.TierUsed != "tier1" {
		t.Fatalf("report.TierUsed = %q, want tier1", report.TierUsed)
	}
	rec := readTierDecisions(t, filepath.Join(dir, "s", "tier-decisions.jsonl"), 1)[0]
	if rec.Decision.Allowed {
		t.Fatalf("blocked decision marked allowed: %+v", rec)
	}
	if !strings.Contains(rec.Decision.Reason, "blocked") {
		t.Fatalf("reason = %q", rec.Decision.Reason)
	}
	if rec.Clone != nil {
		t.Fatalf("no tier2 ran; clone must be nil: %+v", rec.Clone)
	}
}

// readTierDecisions reads and decodes the session's tier-decisions.jsonl.
func readTierDecisions(t *testing.T, path string, wantLines int) []TierDecisionRecord {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var recs []TierDecisionRecord
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var rec TierDecisionRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("bad jsonl line: %v\n%s", err, line)
		}
		recs = append(recs, rec)
	}
	if len(recs) != wantLines {
		t.Fatalf("tier-decisions.jsonl lines = %d, want %d", len(recs), wantLines)
	}
	return recs
}

// findSpanAttrs walks the OTLP traces.jsonl and returns the attributes of the
// first span with the given name, decoded to Go scalars.
func findSpanAttrs(t *testing.T, path, spanName string) map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var doc struct {
			ResourceSpans []struct {
				ScopeSpans []struct {
					Spans []struct {
						Name       string `json:"name"`
						Attributes []struct {
							Key   string          `json:"key"`
							Value json.RawMessage `json:"value"`
						} `json:"attributes"`
					} `json:"spans"`
				} `json:"scopeSpans"`
			} `json:"resourceSpans"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &doc); err != nil {
			continue
		}
		for _, rs := range doc.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, sp := range ss.Spans {
					if sp.Name != spanName {
						continue
					}
					attrs := map[string]any{}
					for _, kv := range sp.Attributes {
						attrs[kv.Key] = decodeOTLPValue(kv.Value)
					}
					return attrs
				}
			}
		}
	}
	t.Fatalf("span %q not found in %s", spanName, path)
	return nil
}

// decodeOTLPValue converts an OTLP AnyValue JSON object into a Go scalar.
func decodeOTLPValue(raw json.RawMessage) any {
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	if s, ok := v["stringValue"]; ok {
		return s
	}
	if b, ok := v["boolValue"]; ok {
		return b
	}
	if i, ok := v["intValue"]; ok {
		return i
	}
	if arr, ok := v["arrayValue"]; ok {
		return arr
	}
	return v
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// ADR-0024.4 D1: agent-declared signals are accepted only from recorded
// `ghx tier2 observe --signal <id>` invocations in the turn traces — never
// from report claims alone.
func TestBuildTierDecisionRecordAcceptsDeclaredSignals(t *testing.T) {
	mk := func(cmds ...string) *TurnResult {
		tr := &TurnResult{}
		for _, c := range cmds {
			tr.ToolTraces = append(tr.ToolTraces, ToolCallTrace{RawInput: map[string]any{"command": c}})
		}
		return tr
	}
	req := AskRequest{Session: "s", Repo: "o/r", Question: "who calls X", AllowedBackends: []string{"remote", "local"}}

	// No declaration: judgment signals stay unset.
	rec := buildTierDecisionRecord(req, 1, mk("ghx search o/r foo"), nil, time.Now())
	if rec.Decision.Observations.SymbolAbsentAfterMaps || rec.Decision.Observations.CandidateAmbiguous || rec.Decision.Observations.ImportChainInvisible {
		t.Fatal("judgment signals set without observe declarations")
	}

	// Declared via trace: accepted and visible in fired observations.
	tr := mk(
		"ghx search o/r foo",
		"ghx tier2 observe --signal "+tier2.SignalSymbolAbsentAfterMaps,
		"ghx tier2 observe --signal="+tier2.SignalImportChainInvisible,
	)
	rec = buildTierDecisionRecord(req, 1, tr, nil, time.Now())
	if !rec.Decision.Observations.SymbolAbsentAfterMaps {
		t.Fatal("declared symbol_absent_after_maps not accepted")
	}
	if !rec.Decision.Observations.ImportChainInvisible {
		t.Fatal("declared import_chain_invisible not accepted")
	}
	if rec.Decision.Observations.CandidateAmbiguous {
		t.Fatal("undeclared candidate_ambiguous must stay unset")
	}
}

// ADR-0024.4 D2: the grant env encodes the ask's AllowedBackends; "-" is an
// explicit empty grant, unset stays equivalent to today; only "local" opens
// tier-2.
func TestTier2GrantValueAndAllowed(t *testing.T) {
	if got := Tier2GrantValue(nil); got != "-" {
		t.Fatalf("Tier2GrantValue(nil) = %q, want \"-\"", got)
	}
	if got := Tier2GrantValue([]string{"remote"}); got != "remote" {
		t.Fatalf("Tier2GrantValue(remote) = %q", got)
	}
	if got := Tier2GrantValue([]string{"remote", tier2.LocalBackendGrant}); got != "remote,local" {
		t.Fatalf("Tier2GrantValue(remote+local) = %q", got)
	}
	if Tier2GrantAllowed("") || Tier2GrantAllowed("-") || Tier2GrantAllowed("remote") {
		t.Fatal("empty/remote-only grants must not allow tier-2")
	}
	if !Tier2GrantAllowed("remote,local") || !Tier2GrantAllowed("local") {
		t.Fatal("local grant spellings must be accepted")
	}
}

// ADR-0024.4 D2 parity: prepareSession stamps the grant env on BOTH ask
// paths because daemon and daemonless share it — pinned by asserting the
// env appears in the spawn environment for a local-granted ask.
func TestPrepareSessionStampsTier2Grant(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{SessionsDir: dir, AgentCmd: "true"}
	req := AskRequest{Session: "grant-parity", Repo: "o/r", Question: "q", AllowedBackends: []string{"remote", tier2.LocalBackendGrant}}
	state, err := prepareSession(cfg, req, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer state.cleanupSink()
	found := false
	for _, e := range state.spawnEnv {
		if e == Tier2GrantEnv+"=remote,local" {
			found = true
		}
	}
	if !found {
		t.Fatalf("spawnEnv missing tier-2 grant stamp: %v", state.spawnEnv)
	}
}
