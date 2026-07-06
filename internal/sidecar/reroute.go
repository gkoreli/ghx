package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Mis-route recovery (ADR-0030.1 D5). The ledger is a derived structure:
// every entry is computed from per-turn report artifacts, so a polluted
// ledger is a cache that can be rebuilt by replaying the surviving turns'
// artifacts — not a corrupted database. RerouteTurn moves one turn's report
// artifact to another session and deterministically rebuilds BOTH ledgers.

// turnReportFile is one reports/<turn>-<ts>.json artifact on disk.
type turnReportFile struct {
	Turn int
	TS   int64
	Path string
}

// listTurnReportArtifacts returns the ~/.ghx-layout per-turn report artifacts
// of a session, sorted by (turn, timestamp) ascending — the deterministic
// replay order for RebuildLedger. Legacy flat report-<ts>.json files carry no
// turn attribution and are excluded.
func listTurnReportArtifacts(sessionsDir, name string) ([]turnReportFile, error) {
	dir := filepath.Join(sessionDir(sessionsDir, name), reportsSubdir)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []turnReportFile
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		turnStr, tsStr, ok := strings.Cut(base, "-")
		if !ok {
			continue
		}
		turn, err1 := strconv.Atoi(turnStr)
		ts, err2 := strconv.ParseInt(tsStr, 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, turnReportFile{Turn: turn, TS: ts, Path: filepath.Join(dir, e.Name())})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Turn != out[j].Turn {
			return out[i].Turn < out[j].Turn
		}
		return out[i].TS < out[j].TS
	})
	return out, nil
}

// ReadReportArtifact reads one persisted per-turn report artifact.
func ReadReportArtifact(path string) (*ReportArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var art ReportArtifact
	if err := json.Unmarshal(data, &art); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &art, nil
}

// RebuildLedger recomputes a session's ledger by replaying its reports/
// artifacts through the same derivation the live path uses
// (UpdateLedgerFromTurn + ApplyTraceCommands). This is the tested
// rebuild-determinism invariant of ADR-0030.1 D5: the result must equal the
// live ledger.json modulo UpdatedAt. The caller persists it with SaveLedger.
func RebuildLedger(sessionsDir, name string) (*Ledger, error) {
	meta, err := ReadMeta(sessionsDir, name)
	if err != nil {
		return nil, err
	}
	files, err := listTurnReportArtifacts(sessionsDir, name)
	if err != nil {
		return nil, err
	}
	ledger := &Ledger{}
	for _, f := range files {
		art, err := ReadReportArtifact(f.Path)
		if err != nil {
			return nil, err
		}
		UpdateLedgerFromTurn(ledger, meta, art.Report, nil, f.Turn)
		ApplyTraceCommands(ledger, art.TraceCommands, f.Turn)
	}
	return ledger, nil
}

// RerouteResult reports what one reroute did, for the CLI summary and the
// daemon RPC response.
type RerouteResult struct {
	// From is the source session the turn was moved out of.
	From string `json:"from"`
	// To is the destination session (created when absent).
	To string `json:"to"`
	// Turn is the source turn number that was moved.
	Turn int `json:"turn"`
	// NewTurn is the turn number the artifact carries in the destination.
	NewTurn int `json:"newTurn"`
	// MovedReports are the destination paths of the moved artifacts.
	MovedReports []string `json:"movedReports"`
	// ToCreated is true when the destination session was created.
	ToCreated bool `json:"toCreated"`
}

// RerouteTurn moves turn `turn` of session `from` to session `to` (created
// when absent), then rebuilds BOTH ledgers by replaying their remaining
// per-turn artifacts (ADR-0030.1 D5). The source session's ACPSessionID is
// cleared so its next turn takes the ADR-0027 stale-session path — a fresh
// ACP session seeded from the now-corrected durable ledger. The ACP
// conversation's memory of the stray turn cannot be moved; the durable
// artifacts and provenance can, and are.
func RerouteTurn(sessionsDir, from string, turn int, to string) (*RerouteResult, error) {
	if from == to {
		return nil, fmt.Errorf("source and destination session are both %q", from)
	}
	srcMeta, err := ReadMeta(sessionsDir, from)
	if err != nil {
		return nil, err
	}
	if srcMeta == nil {
		return nil, fmt.Errorf("session %q not found", from)
	}
	srcFiles, err := listTurnReportArtifacts(sessionsDir, from)
	if err != nil {
		return nil, err
	}
	var moving []turnReportFile
	for _, f := range srcFiles {
		if f.Turn == turn {
			moving = append(moving, f)
		}
	}
	if len(moving) == 0 {
		return nil, fmt.Errorf("session %q has no report artifact for turn %d", from, turn)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	created := false
	if !IsInitialized(sessionsDir, to) {
		scope := fmt.Sprintf("rerouted turn %d from %s", turn, from)
		if err := InitSession(sessionsDir, to, "", scope, SessionNamedExplicit); err != nil {
			return nil, fmt.Errorf("init destination session: %w", err)
		}
		created = true
	}
	destMeta, err := ReadMeta(sessionsDir, to)
	if err != nil {
		return nil, err
	}
	destFiles, err := listTurnReportArtifacts(sessionsDir, to)
	if err != nil {
		return nil, err
	}
	newTurn := maxTurnNumber(destFiles, destMeta.TurnCount) + 1

	// Move the artifact(s): rewrite with reroute provenance under the
	// destination turn number, then remove the source file.
	destDir := filepath.Join(sessionDir(sessionsDir, to), reportsSubdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, err
	}
	var movedPaths []string
	for _, f := range moving {
		art, err := ReadReportArtifact(f.Path)
		if err != nil {
			return nil, err
		}
		art.Rerouted = &RerouteProvenance{FromSession: from, FromTurn: turn, At: now}
		destPath := filepath.Join(destDir, fmt.Sprintf("%d-%d.json", newTurn, f.TS))
		data, err := json.MarshalIndent(art, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(destPath, data, 0o644); err != nil {
			return nil, err
		}
		if err := os.Remove(f.Path); err != nil {
			return nil, err
		}
		movedPaths = append(movedPaths, destPath)
	}

	// Source: clear the ACP session (its conversation remembers the stray
	// turn) and re-anchor the turn counter on the surviving artifacts.
	remaining, err := listTurnReportArtifacts(sessionsDir, from)
	if err != nil {
		return nil, err
	}
	srcMeta.ACPSessionID = ""
	srcMeta.TurnCount = maxTurnNumber(remaining, 0)
	srcMeta.UpdatedAt = now
	if err := SaveMeta(sessionsDir, *srcMeta); err != nil {
		return nil, err
	}
	destMeta.TurnCount = newTurn
	destMeta.UpdatedAt = now
	if err := SaveMeta(sessionsDir, *destMeta); err != nil {
		return nil, err
	}

	// Rebuild both ledgers by replay — the D5 invariant in action.
	for _, name := range []string{from, to} {
		ledger, err := RebuildLedger(sessionsDir, name)
		if err != nil {
			return nil, fmt.Errorf("rebuild ledger %s: %w", name, err)
		}
		if err := SaveLedger(sessionsDir, name, ledger); err != nil {
			return nil, fmt.Errorf("save rebuilt ledger %s: %w", name, err)
		}
	}

	return &RerouteResult{
		From:         from,
		To:           to,
		Turn:         turn,
		NewTurn:      newTurn,
		MovedReports: movedPaths,
		ToCreated:    created,
	}, nil
}

// maxTurnNumber returns the highest turn number among files, at least floor.
func maxTurnNumber(files []turnReportFile, floor int) int {
	max := floor
	for _, f := range files {
		if f.Turn > max {
			max = f.Turn
		}
	}
	return max
}
