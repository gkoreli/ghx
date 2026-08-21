package sidecar

import (
	"strings"
	"testing"
)

func TestFormatEvidenceLedgerTruncationIsVisible(t *testing.T) {
	ledger := &Ledger{}
	for i := 0; i < 200; i++ {
		ledger.CommandsRun = append(ledger.CommandsRun, LedgerEntry{
			Value: "ghx read o/r src/pkg/file_i.go --lines 100-900 # doing some very long exploration command for padding",
			Turn:  i,
		})
	}
	out := formatEvidenceLedger(ledger)
	if len(out) > 1500+300 { // budget + truncation note headroom
		t.Fatalf("rendered ledger %d chars exceeds budget+note", len(out))
	}
	if !strings.Contains(out, "Ledger truncated to fit prompt budget") {
		t.Fatal("truncation note missing from rendered ledger")
	}
}

func TestFormatEvidenceLedgerNoNoteWhenEverythingFits(t *testing.T) {
	ledger := &Ledger{
		CommandsRun:    []LedgerEntry{{Value: "ghx explore o/r", Turn: 1}},
		InspectedPaths: []LedgerEntry{{Value: "src/a.go", Turn: 1}},
	}
	out := formatEvidenceLedger(ledger)
	if strings.Contains(out, "Ledger truncated") {
		t.Fatalf("small ledger should not carry a truncation note:\n%s", out)
	}
}
