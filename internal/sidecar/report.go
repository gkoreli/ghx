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
func ExtractReport(text string) *Report {
	m := ghxReportRE.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	var r Report
	if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &r); err != nil {
		return nil
	}
	if r.Answer == "" {
		return nil
	}
	return &r
}
