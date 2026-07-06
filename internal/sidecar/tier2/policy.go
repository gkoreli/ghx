package tier2

import (
	"fmt"
	"regexp"
	"strings"
)

// Declarative escalation policy (ADR-0024.1 "Escalation Policy"; ADR-0024.2
// D1/D2). The LLM does not get to silently decide "try local": escalation from
// remote Tier 1 to local Tier 2 is a recorded rule evaluation over named
// observables. Evaluate is a pure function so every decision is recomputable
// from the recorded observations alone.

// PolicyVersionV1 is the pre-registered first policy version. Later policy
// changes must bump the version and be ADR'd before any citable run.
const PolicyVersionV1 = "v1"

// Signal kinds: a hard signal escalates alone; soft signals escalate in pairs.
const (
	SignalKindHard = "hard"
	SignalKindSoft = "soft"
)

// Pre-registered signal IDs (ADR-0024.1 "Initial signal IDs").
const (
	SignalCrossFileDependency     = "question.cross_file_dependency"
	SignalPreciseReference        = "question.precise_reference"
	SignalSearchExhausted         = "remote.search_exhausted"
	SignalSymbolAbsentAfterMaps   = "remote.symbol_absent_after_maps"
	SignalCandidateAmbiguous      = "remote.candidate_ambiguous"
	SignalImportChainInvisible    = "remote.import_chain_invisible"
	SignalLowConfidenceRemoteOnly = "answer.low_confidence_remote_only"
)

// Canonical tier IDs for from/to fields and report tierUsed values.
const (
	TierRemote = "tier1"
	TierLocal  = "tier2"
)

// LocalBackendGrant is the AllowedBackends spelling that permits all local
// structural backends (ADR-0024.2 D1, resolving the ADR-0024.1 open
// question). Narrower "local:<tool>" grants are also accepted.
const LocalBackendGrant = "local"

// EscalationPolicy is the versioned declarative policy model (ADR-0024.1).
type EscalationPolicy struct {
	Version string             `json:"version"`
	Signals []EscalationSignal `json:"signals"`
	Rules   []EscalationRule   `json:"rules"`
}

// EscalationSignal describes one observable trigger.
type EscalationSignal struct {
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
	Weight      int    `json:"weight"`
}

// EscalationRule fires when at least MinFired of its AnyOf signals fired.
type EscalationRule struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	AnyOf       []string `json:"anyOf"`
	MinFired    int      `json:"minFired"`
}

// EscalationObservations are the named observables one turn exposes to the
// policy. The v1 runtime derives Question, AllowedBackends,
// SearchesWithoutAcceptedRead, and LowConfidenceRemoteOnly deterministically
// from recorded turn artifacts; the three remaining soft-signal fields are
// supported by the evaluator but never set by the v1 runtime — they await an
// agent-declared observation surface (ADR-0024.2 D2).
type EscalationObservations struct {
	// Question is the turn's English question, matched against the
	// pre-registered phrase tables for the two hard signals.
	Question string `json:"question"`
	// AllowedBackends is the ask's backend allowlist; the local grant gates
	// every escalation.
	AllowedBackends []string `json:"allowedBackends"`
	// SearchesWithoutAcceptedRead counts ghx search commands not followed by
	// any accepting read (ghx read / ghx inspect) before the next search or
	// turn end.
	SearchesWithoutAcceptedRead int `json:"searchesWithoutAcceptedRead"`
	// SymbolAbsentAfterMaps: target symbol/path term absent after mapped
	// reads of candidate files. Not runtime-derived in v1.
	SymbolAbsentAfterMaps bool `json:"symbolAbsentAfterMaps"`
	// CandidateAmbiguous: several plausible files but no evidence-backed top
	// 1-5 ranking. Not runtime-derived in v1.
	CandidateAmbiguous bool `json:"candidateAmbiguous"`
	// ImportChainInvisible: a mapped import/reference points outside the
	// remote evidence already fetched. Not runtime-derived in v1.
	ImportChainInvisible bool `json:"importChainInvisible"`
	// LowConfidenceRemoteOnly: the report ships non-empty uncertainty with
	// only remote evidence.
	LowConfidenceRemoteOnly bool `json:"lowConfidenceRemoteOnly"`
}

// EscalationDecision is the recorded outcome of one policy evaluation
// (ADR-0024.1 model + FiredRules for explainability, ADR-0024.2 D3).
type EscalationDecision struct {
	PolicyVersion string                 `json:"policyVersion"`
	FromTier      string                 `json:"fromTier"`
	ToTier        string                 `json:"toTier"`
	Allowed       bool                   `json:"allowed"`
	FiredSignals  []string               `json:"firedSignals"`
	FiredRules    []string               `json:"firedRules"`
	Observations  EscalationObservations `json:"observations"`
	Reason        string                 `json:"reason"`
}

// DefaultPolicy returns the pre-registered v1 policy: escalate immediately on
// either hard question-shape signal, or on any two soft remote-insufficiency
// signals, always gated on the local backend grant (ADR-0024.1 "Initial
// rule").
func DefaultPolicy() EscalationPolicy {
	return EscalationPolicy{
		Version: PolicyVersionV1,
		Signals: []EscalationSignal{
			{ID: SignalCrossFileDependency, Kind: SignalKindHard, Weight: 1,
				Description: "Question asks what imports/calls/depends on something, blast radius, what breaks, or dependency flow."},
			{ID: SignalPreciseReference, Kind: SignalKindHard, Weight: 1,
				Description: "Question asks for call path, implementors, definition/reference, or exact symbol relationship."},
			{ID: SignalSearchExhausted, Kind: SignalKindSoft, Weight: 1,
				Description: "At least two repo-scoped searches produced no accepted next read."},
			{ID: SignalSymbolAbsentAfterMaps, Kind: SignalKindSoft, Weight: 1,
				Description: "Target symbol/path term is absent after mapped reads of candidate files."},
			{ID: SignalCandidateAmbiguous, Kind: SignalKindSoft, Weight: 1,
				Description: "Tier 1 finds several plausible files but cannot rank the top 1-5 with evidence."},
			{ID: SignalImportChainInvisible, Kind: SignalKindSoft, Weight: 1,
				Description: "A mapped import/reference points to a file or package not visible in fetched remote evidence."},
			{ID: SignalLowConfidenceRemoteOnly, Kind: SignalKindSoft, Weight: 1,
				Description: "The sidecar reports low confidence with only remote evidence."},
		},
		Rules: []EscalationRule{
			{ID: "hard-signal", MinFired: 1,
				Description: "Escalate immediately on either hard question-shape signal.",
				AnyOf:       []string{SignalCrossFileDependency, SignalPreciseReference}},
			{ID: "soft-pair", MinFired: 2,
				Description: "Escalate on any two soft remote-insufficiency signals.",
				AnyOf: []string{SignalSearchExhausted, SignalSymbolAbsentAfterMaps,
					SignalCandidateAmbiguous, SignalImportChainInvisible, SignalLowConfidenceRemoteOnly}},
		},
	}
}

// signalFires maps each signal ID to its pure predicate over observations.
// The table is the single owner of signal semantics; Evaluate only walks it.
var signalFires = map[string]func(EscalationObservations) bool{
	SignalCrossFileDependency: func(o EscalationObservations) bool {
		return crossFileDependencyRE.MatchString(o.Question)
	},
	SignalPreciseReference: func(o EscalationObservations) bool {
		return preciseReferenceRE.MatchString(o.Question)
	},
	SignalSearchExhausted: func(o EscalationObservations) bool {
		return o.SearchesWithoutAcceptedRead >= 2
	},
	SignalSymbolAbsentAfterMaps:   func(o EscalationObservations) bool { return o.SymbolAbsentAfterMaps },
	SignalCandidateAmbiguous:      func(o EscalationObservations) bool { return o.CandidateAmbiguous },
	SignalImportChainInvisible:    func(o EscalationObservations) bool { return o.ImportChainInvisible },
	SignalLowConfidenceRemoteOnly: func(o EscalationObservations) bool { return o.LowConfidenceRemoteOnly },
}

// Pre-registered phrase tables for the two hard question-shape signals
// (ADR-0024.2 D2 — the ADR lists these exact phrases). Matching is
// case-insensitive on word boundaries: "important" must not fire "imports".
var (
	crossFileDependencyRE = phraseTableRE(
		"imports", "imported by", "import graph", "depends on", "dependency",
		"dependencies", "dependents", "blast radius", "what breaks",
		"breaks if", "would break", "fan-in", "fan-out", "who calls",
		"what calls",
	)
	preciseReferenceRE = phraseTableRE(
		"call path", "call chain", "call sites", "callers of", "callees",
		"implementors", "implementers", "who implements", "what implements",
		"implementations of", "definition of", "where defined",
		"references to", "referenced by", "all references",
	)
)

// phraseTableRE compiles a case-insensitive, word-boundary alternation for a
// pre-registered phrase table.
func phraseTableRE(phrases ...string) *regexp.Regexp {
	quoted := make([]string, len(phrases))
	for i, p := range phrases {
		quoted[i] = regexp.QuoteMeta(p)
	}
	return regexp.MustCompile(`(?i)\b(` + strings.Join(quoted, "|") + `)\b`)
}

// LocalBackendAllowed reports whether the allowlist grants local structural
// analysis: the "local" grant covers all local:* backends; a "local:<tool>"
// entry is a narrower grant (ADR-0024.2 D1).
func LocalBackendAllowed(allowedBackends []string) bool {
	for _, b := range allowedBackends {
		b = strings.TrimSpace(b)
		if b == LocalBackendGrant || strings.HasPrefix(b, LocalBackendGrant+":") {
			return true
		}
	}
	return false
}

// Evaluate applies the policy to one turn's observations. Pure: the decision
// is fully determined by (policy, obs) and carries the observations so any
// auditor can recompute it. Allowed is true only when a rule fired AND the
// local backend grant is present; a fired-but-blocked decision keeps the
// fired signals visible and names the blocked backend in Reason (the report
// must surface it in uncertainty/nextReads, never fake a Tier-2 answer).
func (p EscalationPolicy) Evaluate(obs EscalationObservations) EscalationDecision {
	d := EscalationDecision{
		PolicyVersion: p.Version,
		FromTier:      TierRemote,
		ToTier:        TierLocal,
		Observations:  obs,
	}
	fired := map[string]bool{}
	for _, sig := range p.Signals {
		pred, ok := signalFires[sig.ID]
		if !ok || !pred(obs) {
			continue
		}
		fired[sig.ID] = true
		d.FiredSignals = append(d.FiredSignals, sig.ID)
	}
	for _, rule := range p.Rules {
		n := 0
		for _, id := range rule.AnyOf {
			if fired[id] {
				n++
			}
		}
		if n >= rule.MinFired && rule.MinFired > 0 {
			d.FiredRules = append(d.FiredRules, rule.ID)
		}
	}

	localAllowed := LocalBackendAllowed(obs.AllowedBackends)
	switch {
	case len(d.FiredRules) == 0:
		d.Reason = fmt.Sprintf("policy %s: no escalation rule fired (signals fired: %s); remote evidence sufficient by policy",
			p.Version, joinOrNone(d.FiredSignals))
	case localAllowed:
		d.Allowed = true
		d.Reason = fmt.Sprintf("policy %s: rule(s) %s fired via signals %s; %q backend allowed — tier2 escalation permitted",
			p.Version, strings.Join(d.FiredRules, ", "), joinOrNone(d.FiredSignals), LocalBackendGrant)
	default:
		d.Reason = fmt.Sprintf("policy %s: rule(s) %s fired via signals %s but %q is not in allowedBackends — tier2 blocked; the report must name the blocked backend in uncertainty/nextReads and must not fake a tier2 answer",
			p.Version, strings.Join(d.FiredRules, ", "), joinOrNone(d.FiredSignals), LocalBackendGrant)
	}
	return d
}

func joinOrNone(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}
	return strings.Join(ids, ", ")
}
