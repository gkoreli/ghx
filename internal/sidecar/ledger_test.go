package sidecar

import "testing"

func TestDeriveCommandEvidenceReadGrepMapAndTree(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		paths    []string
		globs    []string
		patterns []string
	}{
		{
			name:    "read path",
			command: "ghx read honojs/hono src/compose.ts --lines 1-120",
			paths:   []string{"src/compose.ts"},
		},
		{
			name:     "read grep quoted",
			command:  `ghx read honojs/hono --grep "compose|middleware" "src/**/*.ts"`,
			paths:    []string{"src/**/*.ts"},
			patterns: []string{"compose|middleware"},
		},
		{
			name:    "read map glob",
			command: `ghx read honojs/hono "src/**/*.ts" --map --kind func,type`,
			globs:   []string{"src/**/*.ts"},
		},
		{
			name:    "tree path",
			command: "ghx tree honojs/hono src --depth 2",
			paths:   []string{"src"},
		},
		{
			name:     "search query",
			command:  `ghx search honojs/hono "ContextRenderer"`,
			patterns: []string{"ContextRenderer"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveCommandEvidence(tt.command)
			assertStrings(t, got.InspectedPaths, tt.paths)
			assertStrings(t, got.MappedGlobs, tt.globs)
			assertStrings(t, got.GrepPatterns, tt.patterns)
		})
	}
}

func TestUpdateLedgerFromReportAndToolTraces(t *testing.T) {
	ledger := &Ledger{}
	meta := &SessionMeta{Repo: "honojs/hono", Scope: "middleware"}
	report := &Report{
		RelevantFiles: []RelevantFile{{Path: "src/compose.ts", Reason: "defines compose"}},
		BackendsUsed:  []string{"remote"},
		CommandsRun:   []string{"ghx tree honojs/hono src --depth 2"},
		Uncertainty:   []string{"error propagation not inspected"},
		NextReads:     []string{"src/hono-base.ts"},
	}
	traces := []ToolCallTrace{
		{RawInput: map[string]any{"command": "ghx read honojs/hono src/compose.ts --lines 1-120"}},
		{RawInput: map[string]any{"command": "ghx read honojs/hono src/compose.ts --lines 1-120"}},
		{RawInput: map[string]any{"command": `ghx read honojs/hono --grep onError src/hono-base.ts`}},
	}

	UpdateLedgerFromTurn(ledger, meta, report, traces, 2)

	if ledger.Repo != "honojs/hono" || ledger.Scope != "middleware" {
		t.Fatalf("ledger identity = %q/%q", ledger.Repo, ledger.Scope)
	}
	assertStrings(t, entryValues(ledger.CommandsRun), []string{
		"ghx tree honojs/hono src --depth 2",
		"ghx read honojs/hono src/compose.ts --lines 1-120",
		"ghx read honojs/hono --grep onError src/hono-base.ts",
	})
	assertStrings(t, entryValues(ledger.InspectedPaths), []string{"src", "src/compose.ts", "src/hono-base.ts"})
	assertStrings(t, entryValues(ledger.GrepPatterns), []string{"onError"})
	assertStrings(t, entryValues(ledger.OpenQuestions), []string{"error propagation not inspected", "src/hono-base.ts"})
	if len(ledger.RelevantFiles) != 1 || ledger.RelevantFiles[0].Turn != 2 {
		t.Fatalf("relevant files not recorded with turn provenance: %+v", ledger.RelevantFiles)
	}
}

func TestLedgerPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := &Ledger{
		Repo:           "o/r",
		Scope:          "scope",
		CommandsRun:    []LedgerEntry{{Value: "ghx tree o/r src", Turn: 1}},
		InspectedPaths: []LedgerEntry{{Value: "src", Turn: 1}},
		RelevantFiles:  []RelevantFileEntry{{Path: "src/a.go", Reason: "entry", Turn: 1}},
	}
	if err := SaveLedger(dir, "s", in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadLedger(dir, "s")
	if err != nil {
		t.Fatal(err)
	}
	if out.Repo != in.Repo || out.UpdatedAt == "" {
		t.Fatalf("round-trip lost identity/timestamp: %+v", out)
	}
	assertStrings(t, entryValues(out.CommandsRun), []string{"ghx tree o/r src"})
	if len(out.RelevantFiles) != 1 || out.RelevantFiles[0].Reason != "entry" {
		t.Fatalf("round-trip lost relevant file: %+v", out.RelevantFiles)
	}
}

func entryValues(entries []LedgerEntry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Value
	}
	return out
}

func assertStrings(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestUpdateLedgerStampsSnapshotIdentity(t *testing.T) {
	ledger := &Ledger{}
	meta := &SessionMeta{Repo: "o/r", Scope: "s", Commit: "abc1234", Branch: "mainline"}
	UpdateLedgerFromTurn(ledger, meta, nil, nil, 1)
	if ledger.Commit != "abc1234" || ledger.Branch != "mainline" {
		t.Fatalf("Commit/Branch = %q/%q, want abc1234/mainline", ledger.Commit, ledger.Branch)
	}
	// Existing stamps are never overwritten by later turns.
	meta2 := &SessionMeta{Repo: "o/r", Commit: "def5678", Branch: "other"}
	UpdateLedgerFromTurn(ledger, meta2, nil, nil, 2)
	if ledger.Commit != "abc1234" || ledger.Branch != "mainline" {
		t.Fatalf("snapshot identity was overwritten: %q/%q", ledger.Commit, ledger.Branch)
	}
}

func TestUpdateLedgerSnapshotIdentityUnknownStaysEmpty(t *testing.T) {
	ledger := &Ledger{}
	UpdateLedgerFromTurn(ledger, &SessionMeta{Repo: "o/r"}, nil, nil, 1)
	if ledger.Commit != "" || ledger.Branch != "" {
		t.Fatalf("unknown identity must stay empty, got %q/%q", ledger.Commit, ledger.Branch)
	}
}
