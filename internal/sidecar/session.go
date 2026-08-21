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
	// NamedBy records how the session got its name (ADR-0030.1):
	// SessionNamedExplicit (caller-pinned, R1 — excluded from auto-join
	// routing), SessionNamedRepo (repo slug, R2), or SessionNamedQuestion
	// (question-derived discovery slug, R5). Empty on sessions created before
	// routing shipped; sessionOrigin then infers it from the name shape.
	NamedBy string `json:"namedBy,omitempty"`
	// Cwd is the resolved ACP session working directory used for this session's
	// turns (ADR-0033 D1). Default is the neutral, ghx-owned session directory
	// (SessionWorkspace) — outside any git repo, with no host `.claude` settings
	// — so turns are deterministic regardless of where `ghx` was invoked. An
	// explicit Config.Cwd override (evals) is recorded here verbatim. Persisted
	// on the first turn and reused on resume; empty on sessions created before
	// ADR-0033, which fall back to the session dir and get it written next turn.
	Cwd string `json:"cwd,omitempty"`
	// AgentCmd is the exact ACP agent command line that served this session's
	// turns (ADR-0033 D5). Provenance for diagnosing "which adapter/binary ran
	// this session", especially under the always-on daemon whose warm worker is
	// spawned by whichever caller created the session.
	AgentCmd string `json:"agentCmd,omitempty"`
	// SpawnCwd is os.Getwd() of the process that created this session
	// (ADR-0033 D5). Under the auto-spawn daemon this is the FIRST caller's
	// directory — the environment the resident agent inherited — which is often
	// the difference behind "works from shell A, fails from shell B".
	SpawnCwd string `json:"spawnCwd,omitempty"`
	// AgentEnv lists the NAMES (never values) of agent-relevant environment
	// variables in the agent's resolved spawn environment, refreshed on any
	// ask where the set changes (ADR-0033 D5; client passthrough
	// ADR-0033.1): auth
	// (ANTHROPIC_*, CLAUDE_CODE_USE_BEDROCK/_USE_VERTEX, AWS/Vertex), transport
	// (*_PROXY, NODE_EXTRA_CA_CERTS), and ghx (GH_TOKEN/GITHUB_TOKEN), plus
	// PATH/HOME presence. Names only — no secret ever touches this file — so an
	// environment-specific failure is diagnosable from committed artifacts.
	AgentEnv []string `json:"agentEnv,omitempty"`
	// CreatedAt is the ISO-8601 timestamp of session initialization.
	CreatedAt string `json:"createdAt"`
	// UpdatedAt is the ISO-8601 timestamp of the most recent turn.
	UpdatedAt string `json:"updatedAt"`
	// Commit pins the repo snapshot this session's evidence was gathered
	// against (ADR-0037 M-2). Remote-first recon otherwise floats on a moving
	// default branch; recorded when known (e.g. from ghx output or caller).
	// Empty means unknown — never fabricated.
	Commit string `json:"commit,omitempty"`
	// Branch records the ref evidence was gathered against (e.g. "mainline").
	// Empty means unknown/default.
	Branch string `json:"branch,omitempty"`
}

// sessionDir returns the directory for a named session.
func sessionDir(sessionsDir, name string) string { return filepath.Join(sessionsDir, name) }

// SessionWorkspace returns the neutral ACP session working directory for a
// named session: the session's own ghx-owned directory (ADR-0033 D1). It is
// deterministic, outside any git repository, and contains no host `.claude`
// settings, so the spawned agent adopts no foreign workspace, settings, or
// (untrusted) trust state — making turns reproducible regardless of the
// caller's shell. The runtime uses this as the default cwd; an explicit
// Config.Cwd override (evals pinning a checkout) takes precedence.
func SessionWorkspace(sessionsDir, name string) string { return sessionDir(sessionsDir, name) }

// AgentStderrLogName is the per-session file that captures the spawned ACP
// adapter's stderr (ADR-0033 D2). It lives beside reports/ and traces.jsonl so
// a failed turn's adapter diagnostics are always recoverable from committed
// artifacts, not lost to the daemon's own log.
const AgentStderrLogName = "agent-stderr.log"

// AgentStderrLogPath returns the per-session adapter stderr log path
// (ADR-0033 D2).
func AgentStderrLogPath(sessionsDir, name string) string {
	return filepath.Join(sessionDir(sessionsDir, name), AgentStderrLogName)
}

// IsInitialized reports whether a named session has been initialized on disk.
func IsInitialized(sessionsDir, name string) bool {
	_, err := os.Stat(filepath.Join(sessionDir(sessionsDir, name), "initialized"))
	return err == nil
}

// InitSession creates the session directory, writes the "initialized" marker,
// and writes the initial meta.json. namedBy records the naming origin
// (SessionNamedExplicit/Repo/Question, ADR-0030.1) that routing uses to keep
// explicitly-named sessions out of auto-join. Safe to call multiple times
// (no-op if already initialized).
func InitSession(sessionsDir, name, repo, scope, namedBy string) error {
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
		NamedBy:   namedBy,
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

// reportsSubdir is the per-session directory that holds accepted turn reports
// under the ~/.ghx layout (ADR-0022 D2/D3): sessions/<name>/reports/.
const reportsSubdir = "reports"

// ReportArtifact is the durable accepted-report artifact for one sidecar turn.
// Report is exactly what the agent submitted; ActualCommandLedger is runtime
// metadata derived from observed tool calls so humans can audit omissions in
// the agent-written commandsRun field (ADR-0029 D5).
type ReportArtifact struct {
	Report              *Report  `json:"report"`
	ActualCommandLedger []string `json:"actualCommandLedger,omitempty"`
	// TraceCommands are the bare trace-derived command strings the runtime
	// merged into the session ledger for this turn (TraceCommandLedger). They
	// make the artifact self-sufficient for ledger rebuild: replaying
	// reports/ must reproduce ledger.json (ADR-0030.1 D5 invariant).
	TraceCommands []string `json:"traceCommands,omitempty"`
	// Rerouted records mis-route recovery provenance when this artifact was
	// moved from another session by `ghx sidecar sessions reroute`
	// (ADR-0030.1 D5): routing plus reroute keeps every turn's provenance
	// intact, unlike a destructive merge.
	Rerouted *RerouteProvenance `json:"rerouted,omitempty"`
}

// RerouteProvenance records where a rerouted turn artifact came from.
type RerouteProvenance struct {
	// FromSession is the session the turn was moved out of.
	FromSession string `json:"fromSession"`
	// FromTurn is the turn number the artifact had in the source session.
	FromTurn int `json:"fromTurn"`
	// At is the RFC3339 timestamp of the reroute.
	At string `json:"at"`
}

// SaveTurnReportArtifact persists the accepted report plus runtime audit
// metadata for one turn to sessions/<name>/reports/<turn>-<timestamp>.json.
func SaveTurnReportArtifact(sessionsDir, name string, turn int, artifact ReportArtifact) (string, error) {
	dir := filepath.Join(sessionDir(sessionsDir, name), reportsSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	ts := time.Now().UnixMilli()
	path := filepath.Join(dir, fmt.Sprintf("%d-%d.json", turn, ts))
	data, err := json.MarshalIndent(artifact, "", "  ")
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

// ListReports returns report file paths for a session, newest first. It
// includes both the legacy flat report-<ts>.json files in the session root and
// the ~/.ghx-layout reports/<turn>-<ts>.json files (ADR-0022 D2/D3).
func ListReports(sessionsDir, name string) ([]string, error) {
	dir := sessionDir(sessionsDir, name)
	var paths []string

	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) > 7 &&
			e.Name()[:7] == "report-" &&
			filepath.Ext(e.Name()) == ".json" {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}

	reportsDir := filepath.Join(dir, reportsSubdir)
	subEntries, err := os.ReadDir(reportsDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, e := range subEntries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			paths = append(paths, filepath.Join(reportsDir, e.Name()))
		}
	}

	// Reverse-sort by base filename (timestamps are embedded, so lexicographic
	// == chronological within each naming scheme).
	sort.Slice(paths, func(i, j int) bool {
		return filepath.Base(paths[i]) > filepath.Base(paths[j])
	})
	return paths, nil
}

func writeMeta(dir string, meta SessionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644)
}
