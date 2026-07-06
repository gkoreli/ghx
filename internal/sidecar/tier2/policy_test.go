package tier2

import (
	"encoding/json"
	"strings"
	"testing"
)

// allowLocal is the standard grant used across the matrix.
var allowLocal = []string{"remote", "local"}

// TestPolicyHardSignalsEscalateAlone pins the ADR-0024.1 initial rule: either
// hard question-shape signal escalates by itself when local is allowed.
func TestPolicyHardSignalsEscalateAlone(t *testing.T) {
	cases := []struct {
		name     string
		question string
		signal   string
	}{
		{"cross-file imports", "what imports compose?", SignalCrossFileDependency},
		{"cross-file blast radius", "what is the blast radius of changing route registration?", SignalCrossFileDependency},
		{"cross-file depends on", "which packages depend on the router? it depends on context handling", SignalCrossFileDependency},
		{"cross-file what breaks", "what breaks if I rename ServeHTTP?", SignalCrossFileDependency},
		{"precise call path", "show the call path from Engine.Run to handleHTTPRequest", SignalPreciseReference},
		{"precise implementors", "who implements the Renderer interface?", SignalPreciseReference},
		{"precise definition", "where is the definition of RouterGroup used?", SignalPreciseReference},
		{"precise references", "list all references to the tree node struct", SignalPreciseReference},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := DefaultPolicy().Evaluate(EscalationObservations{
				Question:        tc.question,
				AllowedBackends: allowLocal,
			})
			if !d.Allowed {
				t.Fatalf("Allowed = false, want true; decision: %+v", d)
			}
			if !containsStr(d.FiredSignals, tc.signal) {
				t.Fatalf("FiredSignals = %v, want %s", d.FiredSignals, tc.signal)
			}
			if !containsStr(d.FiredRules, "hard-signal") {
				t.Fatalf("FiredRules = %v, want hard-signal", d.FiredRules)
			}
			if d.FromTier != "tier1" || d.ToTier != "tier2" {
				t.Fatalf("tiers = %s -> %s, want tier1 -> tier2", d.FromTier, d.ToTier)
			}
		})
	}
}

// TestPolicyPhraseWordBoundaries pins the pre-registered matching rule:
// phrases fire on word boundaries only ("important" must not fire "imports").
func TestPolicyPhraseWordBoundaries(t *testing.T) {
	negatives := []string{
		"where is the most important configuration file?", // not "imports"
		"how does the importer registry work?",            // "importer" != "imports"
		"is this codebase well documented?",
		"how are routes configured?",
		"explain the middleware pipeline design",
	}
	for _, q := range negatives {
		d := DefaultPolicy().Evaluate(EscalationObservations{Question: q, AllowedBackends: allowLocal})
		if len(d.FiredSignals) != 0 {
			t.Fatalf("question %q fired %v, want none", q, d.FiredSignals)
		}
		if d.Allowed {
			t.Fatalf("question %q escalated, want not", q)
		}
	}
	// Case-insensitive positive.
	d := DefaultPolicy().Evaluate(EscalationObservations{Question: "BLAST RADIUS of touching pkg/a?", AllowedBackends: allowLocal})
	if !containsStr(d.FiredSignals, SignalCrossFileDependency) {
		t.Fatalf("uppercase phrase did not fire: %v", d.FiredSignals)
	}
}

// TestPolicySoftSignalsNeedAPair pins: one soft signal never escalates; every
// soft pair does (ADR-0024.1 "any two soft remote-insufficiency signals").
func TestPolicySoftSignalsNeedAPair(t *testing.T) {
	softSetters := map[string]func(*EscalationObservations){
		SignalSearchExhausted:         func(o *EscalationObservations) { o.SearchesWithoutAcceptedRead = 2 },
		SignalSymbolAbsentAfterMaps:   func(o *EscalationObservations) { o.SymbolAbsentAfterMaps = true },
		SignalCandidateAmbiguous:      func(o *EscalationObservations) { o.CandidateAmbiguous = true },
		SignalImportChainInvisible:    func(o *EscalationObservations) { o.ImportChainInvisible = true },
		SignalLowConfidenceRemoteOnly: func(o *EscalationObservations) { o.LowConfidenceRemoteOnly = true },
	}
	ids := make([]string, 0, len(softSetters))
	for id := range softSetters {
		ids = append(ids, id)
	}

	// Each soft signal alone: fires, but does not escalate.
	for id, set := range softSetters {
		obs := EscalationObservations{Question: "how does rendering work?", AllowedBackends: allowLocal}
		set(&obs)
		d := DefaultPolicy().Evaluate(obs)
		if !containsStr(d.FiredSignals, id) {
			t.Fatalf("signal %s did not fire: %v", id, d.FiredSignals)
		}
		if d.Allowed || len(d.FiredRules) != 0 {
			t.Fatalf("single soft signal %s escalated: %+v", id, d)
		}
	}

	// Every distinct pair escalates.
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			obs := EscalationObservations{Question: "how does rendering work?", AllowedBackends: allowLocal}
			softSetters[ids[i]](&obs)
			softSetters[ids[j]](&obs)
			d := DefaultPolicy().Evaluate(obs)
			if !d.Allowed || !containsStr(d.FiredRules, "soft-pair") {
				t.Fatalf("pair (%s, %s) did not escalate: %+v", ids[i], ids[j], d)
			}
		}
	}
}

// TestPolicySearchExhaustedThreshold pins the >= 2 threshold.
func TestPolicySearchExhaustedThreshold(t *testing.T) {
	obs := EscalationObservations{SearchesWithoutAcceptedRead: 1}
	if d := DefaultPolicy().Evaluate(obs); containsStr(d.FiredSignals, SignalSearchExhausted) {
		t.Fatalf("1 unaccepted search fired search_exhausted")
	}
	obs.SearchesWithoutAcceptedRead = 2
	if d := DefaultPolicy().Evaluate(obs); !containsStr(d.FiredSignals, SignalSearchExhausted) {
		t.Fatalf("2 unaccepted searches did not fire search_exhausted")
	}
}

// TestPolicyBlockedWithoutLocalGrant pins: a fired rule without the local
// grant yields Allowed=false and a reason naming the blocked backend, so the
// report can surface it instead of faking a tier2 answer.
func TestPolicyBlockedWithoutLocalGrant(t *testing.T) {
	d := DefaultPolicy().Evaluate(EscalationObservations{
		Question:        "what imports compose?",
		AllowedBackends: []string{"remote"},
	})
	if d.Allowed {
		t.Fatalf("Allowed = true without local grant")
	}
	if !containsStr(d.FiredRules, "hard-signal") {
		t.Fatalf("blocked decision must keep fired rules visible: %+v", d)
	}
	if !strings.Contains(d.Reason, `"local"`) || !strings.Contains(d.Reason, "blocked") {
		t.Fatalf("reason must name the blocked backend: %q", d.Reason)
	}
}

// TestLocalBackendAllowedSpellings pins the ADR-0024.2 D1 grant spellings.
func TestLocalBackendAllowedSpellings(t *testing.T) {
	cases := []struct {
		backends []string
		want     bool
	}{
		{[]string{"remote"}, false},
		{[]string{"remote", "local"}, true},
		{[]string{"local:codemap"}, true},
		{[]string{"local:ast-grep"}, true},
		{[]string{" local "}, true},
		{[]string{"localhost"}, false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := LocalBackendAllowed(tc.backends); got != tc.want {
			t.Fatalf("LocalBackendAllowed(%v) = %v, want %v", tc.backends, got, tc.want)
		}
	}
}

// TestPolicyDecisionSerializationRoundTrip pins the ADR-0024.1 verification
// item "policy-version serialization": a decision round-trips through JSON
// with version, signals, observations, and reason intact — the jsonl record
// alone must be enough to recompute the decision.
func TestPolicyDecisionSerializationRoundTrip(t *testing.T) {
	obs := EscalationObservations{
		Question:                    "what imports compose?",
		AllowedBackends:             allowLocal,
		SearchesWithoutAcceptedRead: 3,
		LowConfidenceRemoteOnly:     true,
	}
	d := DefaultPolicy().Evaluate(obs)
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var back EscalationDecision
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if back.PolicyVersion != PolicyVersionV1 {
		t.Fatalf("policyVersion = %q, want %q", back.PolicyVersion, PolicyVersionV1)
	}
	// EscalationObservations contains a slice; compare via re-marshal.
	if a, b := mustJSON(t, back.Observations), mustJSON(t, obs); a != b {
		t.Fatalf("observations did not round-trip: %s vs %s", a, b)
	}
	// The recomputation property itself: evaluating the recorded observations
	// reproduces the recorded decision.
	again := DefaultPolicy().Evaluate(back.Observations)
	a, _ := json.Marshal(again)
	if string(a) != string(data) {
		t.Fatalf("re-evaluating recorded observations diverged:\n%s\n%s", a, data)
	}
}

// TestPolicyNoSignalsReason pins that negative decisions are explainable too.
func TestPolicyNoSignalsReason(t *testing.T) {
	d := DefaultPolicy().Evaluate(EscalationObservations{Question: "how are routes registered?", AllowedBackends: allowLocal})
	if d.Allowed || len(d.FiredRules) != 0 {
		t.Fatalf("unexpected escalation: %+v", d)
	}
	if !strings.Contains(d.Reason, "no escalation rule fired") {
		t.Fatalf("negative reason missing: %q", d.Reason)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func containsStr(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
