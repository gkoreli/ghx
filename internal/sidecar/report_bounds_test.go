package sidecar

import (
	"strings"
	"testing"
)

func TestCheckReportBoundsWithinContract(t *testing.T) {
	r := &Report{
		Answer:        "Composition lives in src/compose.ts.",
		RelevantFiles: []RelevantFile{{Path: "src/compose.ts", Reason: "core"}},
	}
	v := CheckReportBounds(r)
	if v.Violated() {
		t.Fatalf("clean report flagged: %v", v)
	}
	if v.String() != "none" {
		t.Fatalf("String() = %q, want none", v.String())
	}
}

func TestCheckReportBoundsOversize(t *testing.T) {
	r := &Report{Answer: "ok"}
	r.Evidence = make([]Evidence, 0, 50)
	for i := 0; i < 50; i++ {
		r.Evidence = append(r.Evidence, Evidence{
			Source:  "ghx read o/r src/f.go",
			Summary: strings.Repeat("evidence padding to blow past the two-thousand character budget. ", 8),
		})
	}
	v := CheckReportBounds(r)
	if v.OversizeChars <= MaxReportChars {
		t.Fatalf("expected oversize detection, got %d", v.OversizeChars)
	}
}

func TestCheckReportBoundsTooManyFiles(t *testing.T) {
	r := &Report{Answer: "x"}
	for i := 0; i < 8; i++ {
		r.RelevantFiles = append(r.RelevantFiles, RelevantFile{Path: "src/f.go"})
	}
	v := CheckReportBounds(r)
	if v.ExtraRelevantFiles != 3 {
		t.Fatalf("ExtraRelevantFiles = %d, want 3", v.ExtraRelevantFiles)
	}
}

func TestCheckReportBoundsAnswerSentences(t *testing.T) {
	r := &Report{Answer: "One. Two. Three. Four."}
	v := CheckReportBounds(r)
	if v.AnswerSentences != MaxAnswerSentences+2 {
		t.Fatalf("AnswerSentences = %d, want %d", v.AnswerSentences, MaxAnswerSentences+2)
	}
}

func TestCheckReportBoundsNilSafe(t *testing.T) {
	if v := CheckReportBounds(nil); v.Violated() {
		t.Fatal("nil report must not violate")
	}
}
