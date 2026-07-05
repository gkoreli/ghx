// Package sidecar implements the ghx-sidecar specialist agent.
//
// The sidecar translates English repo questions into bounded ghx exploration
// and returns compact, auditable evidence reports. It is a cheap specialist
// that runs ahead of the expensive main coding agent, pre-digesting repo
// structure so the main agent never has to.
package sidecar

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Claim is a single factual assertion with optional supporting evidence.
type Claim struct {
	Summary  string `json:"summary"`
	Evidence string `json:"evidence,omitempty"`
}

// RelevantFile names a file that the sidecar identified as relevant to the question.
type RelevantFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// Evidence records one piece of gathered evidence: the ghx command that produced it
// and a human-readable summary of what it showed.
type Evidence struct {
	Source  string `json:"source"`
	Summary string `json:"summary"`
}

// Report is the structured output emitted inside the <ghx-report> XML block.
// The main coding agent receives this instead of the raw exploration transcript.
type Report struct {
	Answer        string         `json:"answer"`
	Verified      []Claim        `json:"verified"`
	Inferred      []Claim        `json:"inferred"`
	Unverified    []Claim        `json:"unverified"`
	RelevantFiles []RelevantFile `json:"relevantFiles"`
	Evidence      []Evidence     `json:"evidence"`
	BackendsUsed  []string       `json:"backendsUsed"`
	CommandsRun   []string       `json:"commandsRun"`
	Uncertainty   []string       `json:"uncertainty"`
	NextReads     []string       `json:"nextReads"`
}

var ghxReportRE = regexp.MustCompile(`(?s)<ghx-report>(.*?)</ghx-report>`)

// ExtractReport parses the first <ghx-report>…</ghx-report> block from text.
// Returns nil when no valid block is present.
//
// Parsing is lenient (ADR-0016.7 RC2): models under the report compactness
// bound routinely emit a bare string or a single object where the schema
// wants an array. Rejecting the whole report over such a shape mismatch
// throws away completed, auditable work, so near-conformant shapes are
// coerced before unmarshaling.
func ExtractReport(text string) *Report {
	m := ghxReportRE.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	body := []byte(strings.TrimSpace(m[1]))
	var r Report
	if err := json.Unmarshal(body, &r); err != nil {
		coerced, cerr := coerceReportJSON(body)
		if cerr != nil {
			return nil
		}
		if err := json.Unmarshal(coerced, &r); err != nil {
			return nil
		}
	}
	if r.Answer == "" {
		return nil
	}
	return &r
}

// claimFields want []Claim; string items become Claim{Summary: s}.
var claimFields = map[string]bool{"verified": true, "inferred": true, "unverified": true}

// objectListFields want an array of objects; a single object is wrapped.
var objectListFields = map[string]bool{"relevantFiles": true, "evidence": true}

// stringListFields want []string; a bare string is wrapped.
var stringListFields = map[string]bool{
	"backendsUsed": true, "commandsRun": true, "uncertainty": true, "nextReads": true,
}

// coerceReportJSON normalizes near-conformant report JSON into the Report
// schema: bare values where arrays are expected are wrapped, and bare
// strings in claim arrays become {"summary": ...} objects. Anything beyond
// these shapes is left untouched and will fail the typed unmarshal.
func coerceReportJSON(body []byte) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	for key, val := range doc {
		switch {
		case claimFields[key]:
			doc[key] = coerceClaimList(val)
		case objectListFields[key], stringListFields[key]:
			doc[key] = coerceList(val)
		}
	}
	return json.Marshal(doc)
}

// coerceList wraps a single non-array value into a one-element array.
func coerceList(val any) any {
	switch val.(type) {
	case []any, nil:
		return val
	default:
		return []any{val}
	}
}

// coerceClaimList wraps single values and lifts bare strings to claims.
func coerceClaimList(val any) any {
	list, ok := coerceList(val).([]any)
	if !ok {
		return val
	}
	for i, item := range list {
		if s, ok := item.(string); ok {
			list[i] = map[string]any{"summary": s}
		}
	}
	return list
}
