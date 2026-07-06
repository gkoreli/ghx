// Package sidecar implements the ghx-sidecar specialist agent.
//
// The sidecar translates English repo questions into bounded ghx exploration
// and returns compact, auditable evidence reports. It is a cheap specialist
// that runs ahead of the expensive main coding agent, pre-digesting repo
// structure so the main agent never has to.
package sidecar

import (
	"encoding/json"
	"errors"
	"fmt"
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

// ErrNoReportBlock is returned by ExtractReportErr when the text contains no
// <ghx-report>…</ghx-report> block at all.
var ErrNoReportBlock = errors.New("no <ghx-report>…</ghx-report> block found in output")

// ErrEmptyAnswer is returned when a report parses but its answer field is empty.
var ErrEmptyAnswer = errors.New("report has an empty \"answer\" field")

// ExtractReport parses the first <ghx-report>…</ghx-report> block from text.
// Returns nil when no valid block is present.
//
// It is a thin, compatibility-preserving wrapper over ExtractReportErr: callers
// that only care about success/failure keep the original signature, while the
// runtime uses ExtractReportErr to learn WHY extraction failed (ADR-0021 D3).
func ExtractReport(text string) *Report {
	r, _, err := ExtractReportErr(text)
	if err != nil {
		return nil
	}
	return r
}

// ExtractReportErr is the diagnostic variant of ExtractReport (ADR-0021 D3).
//
// It returns the parsed report, whether lenient coercion had to be applied to
// obtain it, and a concrete error describing the failure category:
//
//   - ErrNoReportBlock       — no <ghx-report> block in the text
//   - invalid JSON           — the block is not JSON (wraps the json syntax error)
//   - schema mismatch        — the JSON does not fit the Report schema even
//     after coercion (wraps the typed unmarshal error)
//   - ErrEmptyAnswer         — the report parsed but has no answer
//
// The coerced flag is true only when the strict unmarshal failed and the
// lenient coercion path (ADR-0016.7 RC2) produced a usable report. Coercion
// stays confined to this fallback path; the MCP submit_report tool (ADR-0021
// D1) validates strictly with no coercion. Every coercion is surfaced so drift
// stays visible instead of silently absorbed (Visibility and Truthfulness).
func ExtractReportErr(text string) (report *Report, coerced bool, err error) {
	m := ghxReportRE.FindStringSubmatch(text)
	if m == nil {
		return nil, false, ErrNoReportBlock
	}
	body := []byte(strings.TrimSpace(m[1]))

	var r Report
	strictErr := json.Unmarshal(body, &r)
	if strictErr != nil {
		// Distinguish "not JSON at all" from "JSON but wrong shape": a JSON
		// syntax error means the block is not machine-readable; a type error
		// means the producer emitted a near-conformant shape we may coerce.
		var syntaxErr *json.SyntaxError
		if errors.As(strictErr, &syntaxErr) {
			// Confirm it is genuinely not valid JSON before reporting so.
			if !json.Valid(body) {
				return nil, false, fmt.Errorf("invalid JSON in <ghx-report> block: %w", strictErr)
			}
		}
		coercedBody, cerr := coerceReportJSON(body)
		if cerr != nil {
			return nil, false, fmt.Errorf("invalid JSON in <ghx-report> block: %w", cerr)
		}
		if cerr := json.Unmarshal(coercedBody, &r); cerr != nil {
			return nil, false, fmt.Errorf("<ghx-report> JSON does not fit the report schema: %w", cerr)
		}
		coerced = true
	}
	if r.Answer == "" {
		return nil, coerced, ErrEmptyAnswer
	}
	return &r, coerced, nil
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
// strings in claim arrays become {"summary": ...} objects. Field-specific
// list item drift is also normalized where report agents commonly swap
// string/object forms. Anything beyond these shapes is left untouched and
// will fail the typed unmarshal.
func coerceReportJSON(body []byte) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	for key, val := range doc {
		switch {
		case claimFields[key]:
			doc[key] = coerceClaimList(val)
		case objectListFields[key]:
			doc[key] = coerceObjectList(key, val)
		case stringListFields[key]:
			doc[key] = coerceStringList(val)
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

// coerceStringList wraps single values and converts common object/scalar drift
// into strings for fields whose schema is []string.
func coerceStringList(val any) any {
	list, ok := coerceList(val).([]any)
	if !ok {
		return val
	}
	for i, item := range list {
		switch v := item.(type) {
		case string:
			continue
		case map[string]any:
			list[i] = stringifyReportObject(v)
		case float64, bool, nil:
			list[i] = compactJSON(v)
		}
	}
	return list
}

// coerceObjectList wraps single values and converts bare string items into the
// object shape expected by relevantFiles and evidence.
func coerceObjectList(key string, val any) any {
	list, ok := coerceList(val).([]any)
	if !ok {
		return val
	}
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			continue
		}
		switch key {
		case "relevantFiles":
			list[i] = map[string]any{"path": s}
		case "evidence":
			list[i] = map[string]any{"summary": s}
		}
	}
	return list
}

func stringifyReportObject(obj map[string]any) string {
	for _, key := range []string{"path", "source", "summary", "value"} {
		if val, ok := obj[key]; ok {
			s := stringifyReportValue(val)
			if reason, ok := obj["reason"]; ok {
				rs := stringifyReportValue(reason)
				if rs != "" {
					s += " — " + rs
				}
			}
			return s
		}
	}
	return compactJSON(obj)
}

func stringifyReportValue(val any) string {
	if s, ok := val.(string); ok {
		return s
	}
	return compactJSON(val)
}

func compactJSON(val any) string {
	b, err := json.Marshal(val)
	if err != nil {
		return ""
	}
	return string(b)
}
