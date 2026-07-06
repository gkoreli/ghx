package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// SaveEpisode writes one episode artifact as pretty-printed JSON under
// runDir, creating the directory if needed. Returns the written path.
// Episode JSON files are the canonical eval record (ADR-0016).
//
// The anomaly detectors run here, immediately before persistence
// (ADR-0016.8 D6): the stored anomalies field is always the complete
// detector output for the persisted fields, so re-deriving anomalies from
// any saved artifact equals its stored field — regardless of what the
// caller did (or forgot to do) between RunEpisode and SaveEpisode.
func SaveEpisode(runDir string, ep *Episode) (string, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", runDir, err)
	}
	ep.Anomalies = DetectAnomalies(ep)
	path := filepath.Join(runDir, ep.ID+".json")
	data, err := json.MarshalIndent(ep, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	if err := EmitEpisodeTraces(context.TODO(), runDir, ep); err != nil {
		return "", err
	}
	if err := EmitEpisodeMetrics(runDir, ep); err != nil {
		return "", err
	}
	return path, nil
}

// LoadEpisode reads one episode artifact back from disk.
func LoadEpisode(path string) (*Episode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ep Episode
	if err := json.Unmarshal(data, &ep); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &ep, nil
}

// ExpectedEpisodesEnv is the manual escape hatch (ADR-0016.8 D5): when set,
// it pins the whole planned run's expected episode total, overriding the sum
// of recorded rounds. Its use is recorded in the manifest
// (ExpectedEpisodesOverride) so provenance shows the total was asserted by a
// human, not derived from the planned matrix.
const ExpectedEpisodesEnv = "GHX_EVAL_EXPECTED_EPISODES"

// PlannedRound is one planned tranche of a run: the tasks × profiles × trials
// matrix a single harness invocation intends to add to the run directory
// (ADR-0016.8 D5). Multi-round gate runs accumulate rounds instead of
// overwriting, so the manifest always carries the full planned matrix and the
// derived expected total — the provenance the 2026-07-05 audit found missing
// (expectedEpisodes: 18 against a 90-episode run) and the true planned total
// the ADR-0025 D1 stopping math needs.
type PlannedRound struct {
	Tasks            int       `json:"tasks"`
	Profiles         int       `json:"profiles"`
	Trials           int       `json:"trials"`
	ExpectedEpisodes int       `json:"expectedEpisodes"`
	RecordedAt       time.Time `json:"recordedAt"`
}

// RunManifest records run-level eval identity and the planned episode matrix.
type RunManifest struct {
	RunDir string `json:"runDir"`
	// Rounds is the append-only planned-matrix history (ADR-0016.8 D5).
	Rounds []PlannedRound `json:"rounds,omitempty"`
	// ExpectedEpisodes is the planned total for the whole run: the sum of
	// round totals, unless ExpectedEpisodesOverride pins it.
	ExpectedEpisodes int `json:"expectedEpisodes,omitempty"`
	// ExpectedEpisodesOverride records the GHX_EVAL_EXPECTED_EPISODES manual
	// escape hatch when it was used for this run.
	ExpectedEpisodesOverride int            `json:"expectedEpisodesOverride,omitempty"`
	Identity                 AgentIdentity  `json:"identity,omitempty"`
	CreatedAt                time.Time      `json:"createdAt"`
	BaselineReuse            *BaselineReuse `json:"baselineReuse,omitempty"`
}

// BaselineReuse records the provenance for a run that copied plain/ghx
// baseline episodes from a prior committed run (ADR-0025.1 D2). Hashes name the
// exact identity inputs that admitted the prior run; Episodes names every
// copied JSON artifact and its raw-byte digest.
type BaselineReuse struct {
	ReusedFromRunID  string                        `json:"reusedFromRunId,omitempty"`
	ReusedFromRunDir string                        `json:"reusedFromRunDir,omitempty"`
	ReusedAt         time.Time                     `json:"reusedAt,omitempty"`
	MaxAgeDays       int                           `json:"maxAgeDays"`
	VerdictLabel     string                        `json:"verdictLabel,omitempty"`
	Hashes           []BaselineReuseHashRecord     `json:"hashes,omitempty"`
	Episodes         []BaselineReusedEpisodeRecord `json:"episodes,omitempty"`
}

// BaselineReuseHashRecord is one sha256 input in the baseline-reuse identity
// inventory. SourceKind is one of file, value, or generated.
type BaselineReuseHashRecord struct {
	Name       string `json:"name"`
	Algorithm  string `json:"algorithm"`
	Value      string `json:"value"`
	Source     string `json:"source"`
	SourceKind string `json:"sourceKind"`
}

// BaselineReusedEpisodeRecord records one byte-for-byte copied baseline
// episode JSON file.
type BaselineReusedEpisodeRecord struct {
	Profile    Profile `json:"profile"`
	TaskID     string  `json:"taskId"`
	SourceFile string  `json:"sourceFile"`
	TargetFile string  `json:"targetFile"`
	SHA256     string  `json:"sha256"`
}

func SaveRunManifest(runDir string, m RunManifest) error {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", runDir, err)
	}
	if m.RunDir == "" {
		m.RunDir = runDir
	}
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(runDir, "manifest.json"), data, 0o644)
}

// RecordManifestRound appends one planned round to the run manifest
// (creating the manifest if needed), recomputes the run-wide
// ExpectedEpisodes as the sum across rounds — or the
// GHX_EVAL_EXPECTED_EPISODES override, recorded when used — refreshes the
// identity when one is provided, and saves. Returns the updated manifest so
// callers (e.g. sequential stopping, ADR-0025 D1) can read the true planned
// total for the whole run.
func RecordManifestRound(runDir string, round PlannedRound, identity AgentIdentity) (*RunManifest, error) {
	m, err := LoadRunManifest(runDir)
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &RunManifest{RunDir: runDir, CreatedAt: time.Now().UTC()}
	}
	if round.ExpectedEpisodes == 0 {
		round.ExpectedEpisodes = round.Tasks * round.Profiles * round.Trials
	}
	if round.RecordedAt.IsZero() {
		round.RecordedAt = time.Now().UTC()
	}
	m.Rounds = append(m.Rounds, round)

	total := 0
	for _, r := range m.Rounds {
		total += r.ExpectedEpisodes
	}
	m.ExpectedEpisodes = total
	if v := strings.TrimSpace(os.Getenv(ExpectedEpisodesEnv)); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil && n > 0 {
			m.ExpectedEpisodesOverride = n
		}
	}
	if m.ExpectedEpisodesOverride > 0 {
		m.ExpectedEpisodes = m.ExpectedEpisodesOverride
	}
	if identityKey(identity) != "" {
		m.Identity = identity
	}
	return m, SaveRunManifest(runDir, *m)
}

// UpdateManifestIdentity refreshes the manifest's identity — e.g. once the
// first live episode reports the adapter's real identity — without adding a
// planned round.
func UpdateManifestIdentity(runDir string, identity AgentIdentity) error {
	m, err := LoadRunManifest(runDir)
	if err != nil {
		return err
	}
	if m == nil {
		m = &RunManifest{RunDir: runDir}
	}
	m.Identity = identity
	return SaveRunManifest(runDir, *m)
}

func LoadRunManifest(runDir string) (*RunManifest, error) {
	data, err := os.ReadFile(filepath.Join(runDir, "manifest.json"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var m RunManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
