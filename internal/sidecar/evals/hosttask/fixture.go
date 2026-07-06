package hosttask

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// CanaryResult is the recorded outcome of the authoring-time memorization
// canary (ADR-0032 trap 3): the subject model, given only the issue text and
// no repository access, must NOT produce the fix from weights.
type CanaryResult string

const (
	// CanaryPass records that no from-weights fix appeared — the task is
	// eligible for the corpus.
	CanaryPass CanaryResult = "pass"
	// CanaryFail records a from-weights fix — the task is disqualified at
	// authoring time and must never be committed as a fixture.
	CanaryFail CanaryResult = "fail"
)

// MemorizationCanary is the authoring provenance of the memorization check.
// Every committed fixture must carry a passing canary (ADR-0032.1 D3).
type MemorizationCanary struct {
	// CheckedAt is the date the canary was run (YYYY-MM-DD).
	CheckedAt string `json:"checkedAt"`
	// Result is the canary outcome; only CanaryPass validates.
	Result CanaryResult `json:"result"`
}

// SubQuestion is one pre-registered exploration sub-question with its ground
// truth — the deterministic half of GH2-recon (ADR-0032.1 D2). S1 carries
// these as fixture data; scoring belongs to later slices.
type SubQuestion struct {
	// Question is what the correct exploration must establish.
	Question string `json:"question"`
	// GroundTruth is the pre-registered answer, verified against the pinned
	// dependency version at authoring time.
	GroundTruth string `json:"groundTruth"`
	// VerifiedAt is the authoring-time verification date (YYYY-MM-DD).
	VerifiedAt string `json:"verifiedAt"`
}

// Fixture is one hand-authored host task (see the package documentation for
// the full schema contract and an example JSON document).
type Fixture struct {
	// ID is the stable fixture identity; a safe path segment.
	ID string `json:"id"`
	// WorkspaceRepo is the "owner/repo" GitHub repository used as the host
	// agent's workspace.
	WorkspaceRepo string `json:"workspaceRepo"`
	// PinnedSHA is the full 40-hex commit the workspace is materialized at.
	PinnedSHA string `json:"pinnedSha"`
	// Image is the container image the grader runs commands in (must
	// provide /bin/sh).
	Image string `json:"image"`
	// Network is the docker network mode for grader containers; empty
	// defaults to "none" (hermetic grading).
	Network string `json:"network,omitempty"`
	// SetupCmds prepare the environment inside the container, in order.
	SetupCmds []string `json:"setupCmds,omitempty"`
	// FailToPassCmds fail on the pinned SHA and must pass after a correct
	// fix (SWE-bench F2P invariant). At least one is required.
	FailToPassCmds []string `json:"failToPassCmds"`
	// PassToPassCmds pass on the pinned SHA and must still pass after the
	// fix (SWE-bench P2P invariant). At least one is required.
	PassToPassCmds []string `json:"passToPassCmds"`
	// ExplorationSubQuestions is the pre-registered GH2 ground truth.
	ExplorationSubQuestions []SubQuestion `json:"explorationSubQuestions"`
	// MemorizationCanary is the authoring-time anti-memorization provenance.
	MemorizationCanary MemorizationCanary `json:"memorizationCanary"`
}

// fixtureIDRE matches a safe fixture ID: it is embedded in workspace
// directory names, so it must be a single clean path segment.
var fixtureIDRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// dateRE matches the YYYY-MM-DD provenance dates fixtures carry.
var dateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// NetworkMode returns the docker network mode grader containers run with:
// the fixture's Network, defaulting to "none" when unset.
func (f Fixture) NetworkMode() string {
	if f.Network == "" {
		return "none"
	}
	return f.Network
}

// RemoteURL returns the canonical clone URL for the workspace repository.
func (f Fixture) RemoteURL() string {
	return "https://github.com/" + f.WorkspaceRepo + ".git"
}

// Validate checks the fixture against the schema contract. It rejects
// fixtures whose memorization canary did not pass: a from-weights fix
// disqualifies the task at authoring time (ADR-0032.1 D3).
func (f Fixture) Validate() error {
	if !fixtureIDRE.MatchString(f.ID) {
		return fmt.Errorf("fixture id %q: want a safe path segment matching %s", f.ID, fixtureIDRE)
	}
	if _, _, err := tier2.SplitRepo(f.WorkspaceRepo); err != nil {
		return fmt.Errorf("fixture %s: workspaceRepo: %w", f.ID, err)
	}
	if !tier2.IsCommitSHA(f.PinnedSHA) {
		return fmt.Errorf("fixture %s: pinnedSha %q: want a full 40-hex commit sha", f.ID, f.PinnedSHA)
	}
	if strings.TrimSpace(f.Image) == "" {
		return fmt.Errorf("fixture %s: image is required (the environment pin)", f.ID)
	}
	if len(f.FailToPassCmds) == 0 {
		return fmt.Errorf("fixture %s: at least one failToPass command is required", f.ID)
	}
	if len(f.PassToPassCmds) == 0 {
		return fmt.Errorf("fixture %s: at least one passToPass command is required (the no-regression half)", f.ID)
	}
	for phase, cmds := range map[string][]string{
		"setupCmds": f.SetupCmds, "failToPassCmds": f.FailToPassCmds, "passToPassCmds": f.PassToPassCmds,
	} {
		for i, cmd := range cmds {
			if strings.TrimSpace(cmd) == "" {
				return fmt.Errorf("fixture %s: %s[%d] is empty", f.ID, phase, i)
			}
		}
	}
	if len(f.ExplorationSubQuestions) == 0 {
		return fmt.Errorf("fixture %s: at least one exploration sub-question with ground truth is required (ADR-0032.1 GH2)", f.ID)
	}
	for i, q := range f.ExplorationSubQuestions {
		if strings.TrimSpace(q.Question) == "" || strings.TrimSpace(q.GroundTruth) == "" {
			return fmt.Errorf("fixture %s: explorationSubQuestions[%d]: question and groundTruth are required", f.ID, i)
		}
		if err := validDate(q.VerifiedAt); err != nil {
			return fmt.Errorf("fixture %s: explorationSubQuestions[%d].verifiedAt: %w", f.ID, i, err)
		}
	}
	if err := validDate(f.MemorizationCanary.CheckedAt); err != nil {
		return fmt.Errorf("fixture %s: memorizationCanary.checkedAt: %w", f.ID, err)
	}
	switch f.MemorizationCanary.Result {
	case CanaryPass:
	case CanaryFail:
		return fmt.Errorf("fixture %s: memorization canary failed — a from-weights fix disqualifies the task (ADR-0032.1 D3); do not commit it", f.ID)
	default:
		return fmt.Errorf("fixture %s: memorizationCanary.result %q: want %q or %q", f.ID, f.MemorizationCanary.Result, CanaryPass, CanaryFail)
	}
	return nil
}

// validDate checks a YYYY-MM-DD provenance date.
func validDate(s string) error {
	if !dateRE.MatchString(s) {
		return fmt.Errorf("date %q: want YYYY-MM-DD", s)
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("date %q: %w", s, err)
	}
	return nil
}

// LoadFixtures reads and validates all fixture files (*.json) in dir, sorted
// by filename so runs are deterministic (mirrors evals.LoadTasks).
func LoadFixtures(dir string) ([]Fixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read fixtures dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var fixtures []Fixture
	seen := map[string]string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		var f Fixture
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		if err := f.Validate(); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if prev, dup := seen[f.ID]; dup {
			return nil, fmt.Errorf("%s: duplicate fixture id %q (also in %s)", name, f.ID, prev)
		}
		seen[f.ID] = name
		fixtures = append(fixtures, f)
	}
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no fixture files in %s", dir)
	}
	return fixtures, nil
}
