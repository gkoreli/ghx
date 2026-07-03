package sidecar

import (
	"testing"
)

func TestSessionLifecycle(t *testing.T) {
	dir := t.TempDir()

	if IsInitialized(dir, "s1") {
		t.Fatal("session should not be initialized before InitSession")
	}
	if err := InitSession(dir, "s1", "honojs/hono", "middleware"); err != nil {
		t.Fatal(err)
	}
	if !IsInitialized(dir, "s1") {
		t.Fatal("session should be initialized after InitSession")
	}

	meta, err := ReadMeta(dir, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if meta == nil {
		t.Fatal("expected meta, got nil")
	}
	if meta.Repo != "honojs/hono" || meta.Scope != "middleware" || meta.TurnCount != 0 {
		t.Errorf("meta = %+v", meta)
	}

	if err := RecordTurn(dir, "s1"); err != nil {
		t.Fatal(err)
	}
	if err := RecordTurn(dir, "s1"); err != nil {
		t.Fatal(err)
	}
	meta, _ = ReadMeta(dir, "s1")
	if meta.TurnCount != 2 {
		t.Errorf("turnCount = %d, want 2", meta.TurnCount)
	}
}

func TestReadMetaMissingSession(t *testing.T) {
	meta, err := ReadMeta(t.TempDir(), "nope")
	if err != nil {
		t.Fatalf("missing session should not error: %v", err)
	}
	if meta != nil {
		t.Errorf("expected nil meta, got %+v", meta)
	}
}

func TestSaveMetaPersistsACPSessionID(t *testing.T) {
	dir := t.TempDir()
	if err := InitSession(dir, "s1", "o/r", "scope"); err != nil {
		t.Fatal(err)
	}
	meta, _ := ReadMeta(dir, "s1")
	meta.ACPSessionID = "sess_abc123"
	if err := SaveMeta(dir, *meta); err != nil {
		t.Fatal(err)
	}
	got, _ := ReadMeta(dir, "s1")
	if got.ACPSessionID != "sess_abc123" {
		t.Errorf("acpSessionId = %q, want sess_abc123", got.ACPSessionID)
	}
}

func TestSaveAndListReports(t *testing.T) {
	dir := t.TempDir()
	if err := InitSession(dir, "s1", "o/r", "scope"); err != nil {
		t.Fatal(err)
	}

	path, err := SaveReport(dir, "s1", &Report{Answer: "found it"})
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected report path")
	}

	reports, err := ListReports(dir, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0] != path {
		t.Errorf("reports = %v, want [%s]", reports, path)
	}
}

func TestListReportsIgnoresNonReportFiles(t *testing.T) {
	dir := t.TempDir()
	if err := InitSession(dir, "s1", "o/r", "scope"); err != nil {
		t.Fatal(err)
	}
	// meta.json and the initialized marker live in the same directory.
	reports, err := ListReports(dir, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Errorf("expected no reports, got %v", reports)
	}
}

func TestListSessionsSortedNewestFirst(t *testing.T) {
	dir := t.TempDir()
	if err := InitSession(dir, "older", "o/r1", "a"); err != nil {
		t.Fatal(err)
	}
	if err := InitSession(dir, "newer", "o/r2", "b"); err != nil {
		t.Fatal(err)
	}
	// Force distinct UpdatedAt ordering regardless of clock resolution.
	meta, _ := ReadMeta(dir, "newer")
	meta.UpdatedAt = "2099-01-01T00:00:00Z"
	if err := SaveMeta(dir, *meta); err != nil {
		t.Fatal(err)
	}

	metas, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(metas) != 2 {
		t.Fatalf("sessions = %d, want 2", len(metas))
	}
	if metas[0].Name != "newer" {
		t.Errorf("first session = %q, want newer", metas[0].Name)
	}
}

func TestListSessionsMissingDir(t *testing.T) {
	metas, err := ListSessions(t.TempDir() + "/does-not-exist")
	if err != nil {
		t.Fatalf("missing dir should not error: %v", err)
	}
	if metas != nil {
		t.Errorf("expected nil, got %v", metas)
	}
}
