package sidecar

import (
	"encoding/json"
	"strings"
	"testing"
)

// Adversarial sweep: for every ledger size 1..60 the rendered block (note
// included) must stay within the 1500-char prompt budget.
func TestFormatEvidenceLedgerBoundarySweep(t *testing.T) {
	for n := 1; n <= 60; n++ {
		l := &Ledger{}
		for i := 0; i < n; i++ {
			l.CommandsRun = append(l.CommandsRun, LedgerEntry{Value: strings.Repeat("c", 40), Turn: i})
			l.InspectedPaths = append(l.InspectedPaths, LedgerEntry{Value: strings.Repeat("p", 40), Turn: i})
		}
		if out := formatEvidenceLedger(l); len(out) > 1500 {
			t.Fatalf("n=%d: rendered %d chars exceeds budget", n, len(out))
		}
	}
}

// A single entry longer than the whole budget must not wedge the eviction
// loop: it gets dropped entirely and the note says so.
func TestFormatEvidenceLedgerHugeSingleEntry(t *testing.T) {
	l := &Ledger{CommandsRun: []LedgerEntry{{Value: strings.Repeat("x", 1600), Turn: 1}}}
	out := formatEvidenceLedger(l)
	if len(out) > 1500 || !strings.Contains(out, "Ledger truncated") {
		t.Fatalf("huge-entry case: len=%d hasNote=%v", len(out), strings.Contains(out, "Ledger truncated"))
	}
}

// Pre-ADR-0037 persisted files (no commit/branch keys) must parse cleanly.
func TestOldJSONParsesWithoutSnapshotFields(t *testing.T) {
	var l Ledger
	old := `{"repo":"o/r","commands_run":[{"value":"ghx explore o/r","turn":1}]}`
	if err := json.Unmarshal([]byte(old), &l); err != nil {
		t.Fatalf("old ledger.json incompatible: %v", err)
	}
	var m SessionMeta
	oldMeta := `{"name":"s","repo":"o/r","turnCount":2}`
	if err := json.Unmarshal([]byte(oldMeta), &m); err != nil {
		t.Fatalf("old meta.json incompatible: %v", err)
	}
}
