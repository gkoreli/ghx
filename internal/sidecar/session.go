package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SessionMeta is the metadata file written inside each named session directory.
type SessionMeta struct {
	// Name is the bare session name (without any prefix).
	Name string `json:"name"`
	// Repo is the GitHub repo under investigation ("owner/repo").
	Repo string `json:"repo"`
	// Scope is a short human label describing the investigation focus.
	Scope string `json:"scope"`
	// TurnCount is the total number of completed turns.
	TurnCount int `json:"turnCount"`
	// ACPSessionID is the agent-assigned ACP session ID for resumption.
	// Empty on the first turn; set after the first NewSession call succeeds.
	ACPSessionID string `json:"acpSessionId,omitempty"`
	// CreatedAt is the ISO-8601 timestamp of session initialization.
	CreatedAt string `json:"createdAt"`
	// UpdatedAt is the ISO-8601 timestamp of the most recent turn.
	UpdatedAt string `json:"updatedAt"`
}

// sessionDir returns the directory for a named session.
func sessionDir(sessionsDir, name string) string { return filepath.Join(sessionsDir, name) }

// IsInitialized reports whether a named session has been initialized on disk.
func IsInitialized(sessionsDir, name string) bool {
	_, err := os.Stat(filepath.Join(sessionDir(sessionsDir, name), "initialized"))
	return err == nil
}

// InitSession creates the session directory, writes the "initialized" marker,
// and writes the initial meta.json. Safe to call multiple times (no-op if
// already initialized).
func InitSession(sessionsDir, name, repo, scope string) error {
	dir := sessionDir(sessionsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(dir, "initialized"), []byte(now), 0o644); err != nil {
		return err
	}
	meta := SessionMeta{
		Name:      name,
		Repo:      repo,
		Scope:     scope,
		TurnCount: 0,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return writeMeta(dir, meta)
}

// ReadMeta reads the metadata for a named session.
// Returns nil when the session does not exist.
func ReadMeta(sessionsDir, name string) (*SessionMeta, error) {
	path := filepath.Join(sessionDir(sessionsDir, name), "meta.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m SessionMeta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// SaveMeta overwrites the meta.json for a named session.
func SaveMeta(sessionsDir string, meta SessionMeta) error {
	dir := sessionDir(sessionsDir, meta.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return writeMeta(dir, meta)
}

// RecordTurn increments the turn counter and updates UpdatedAt.
func RecordTurn(sessionsDir, name string) error {
	meta, err := ReadMeta(sessionsDir, name)
	if err != nil || meta == nil {
		return err
	}
	meta.TurnCount++
	meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return SaveMeta(sessionsDir, *meta)
}

// SaveReport persists a structured evidence report for a completed turn.
// Returns the path of the written file.
func SaveReport(sessionsDir, name string, report *Report) (string, error) {
	dir := sessionDir(sessionsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	ts := time.Now().UnixMilli()
	path := filepath.Join(dir, fmt.Sprintf("report-%d.json", ts))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, data, 0o644)
}

// ListSessions returns all session metadata records, sorted newest first.
func ListSessions(sessionsDir string) ([]SessionMeta, error) {
	entries, err := os.ReadDir(sessionsDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var metas []SessionMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		m, err := ReadMeta(sessionsDir, e.Name())
		if err != nil || m == nil {
			continue
		}
		metas = append(metas, *m)
	}
	sort.Slice(metas, func(i, j int) bool { return metas[i].UpdatedAt > metas[j].UpdatedAt })
	return metas, nil
}

// ListReports returns report file paths for a session, newest first.
func ListReports(sessionsDir, name string) ([]string, error) {
	dir := sessionDir(sessionsDir, name)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 7 &&
			e.Name()[:7] == "report-" &&
			filepath.Ext(e.Name()) == ".json" {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	// Reverse-sort by filename (timestamps are embedded, so lexicographic == chronological)
	sort.Sort(sort.Reverse(sort.StringSlice(paths)))
	return paths, nil
}

func writeMeta(dir string, meta SessionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644)
}
