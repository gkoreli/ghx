package hosttask

import (
	"path/filepath"
	"strings"
	"testing"
)

// corpusDir is the committed S4 host-task corpus (ADR-0032.1 D3/S4).
const corpusDir = "corpus"

// TestCorpusLoadsAndMeetsD3 loads the committed corpus through the strict
// LoadCorpus gate and asserts the ADR-0032.1 D3 composition rules: six tasks,
// >=2 dependency-behavior bugfixes, >=2 API-migration slices, >=1 conformance
// feature, and >=3 distinct workspace repos. It is the corpus-validation test
// the S4 note promises — every check is recomputable from the committed JSON.
func TestCorpusLoadsAndMeetsD3(t *testing.T) {
	fixtures, err := LoadCorpus(filepath.Join(corpusDir))
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(fixtures) != 6 {
		t.Fatalf("corpus has %d fixtures, want exactly 6 (ADR-0032.1 D3)", len(fixtures))
	}

	// D3 family quotas, keyed by fixture id (the family a task belongs to is an
	// authoring fact recorded here, not inferred). Each family must be covered.
	family := map[string]string{
		"chi-fancywriter-readfrom-tee":  "bugfix",
		"chi-routeheaders-double-next":  "bugfix",
		"mux-multi-query-same-path":     "bugfix",
		"mux-status-code-conformance":   "conformance",
		"chi-request-pattern-go123":     "migration",
		"echo-decompress-contentlength": "migration",
	}
	counts := map[string]int{}
	repos := map[string]struct{}{}
	for _, f := range fixtures {
		fam, ok := family[f.ID]
		if !ok {
			t.Fatalf("fixture %q has no recorded D3 family — update the corpus_test family map when adding a task", f.ID)
		}
		counts[fam]++
		repos[f.WorkspaceRepo] = struct{}{}

		// Every corpus fixture must carry the run-driving issue text and a
		// passing memorization canary (Validate already rejects a failed one;
		// this pins the positive expectation).
		if strings.TrimSpace(f.Objective) == "" {
			t.Errorf("fixture %q: empty objective", f.ID)
		}
		if f.MemorizationCanary.Result != CanaryPass {
			t.Errorf("fixture %q: canary result %q, want pass", f.ID, f.MemorizationCanary.Result)
		}
		if len(f.ExplorationSubQuestions) == 0 {
			t.Errorf("fixture %q: no exploration sub-questions (GH2 ground truth)", f.ID)
		}
	}

	if counts["bugfix"] < 2 {
		t.Errorf("D3: %d dependency-behavior bugfixes, want >=2", counts["bugfix"])
	}
	if counts["migration"] < 2 {
		t.Errorf("D3: %d API-migration slices, want >=2", counts["migration"])
	}
	if counts["conformance"] < 1 {
		t.Errorf("D3: %d conformance features, want >=1", counts["conformance"])
	}
	if len(repos) < 3 {
		t.Errorf("D3: %d distinct workspace repos, want >=3", len(repos))
	}
}

// TestCorpusFixturesHaveHiddenF2PAndProvenance pins the S4 authoring
// invariants that make each grade auditable: the F2P test is HIDDEN (injected
// by a setup command, never present in the agent-visible workspace), and every
// exploration sub-question carries a verifiedAt provenance date.
func TestCorpusFixturesHaveHiddenF2PAndProvenance(t *testing.T) {
	fixtures, err := LoadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		joinedSetup := strings.Join(f.SetupCmds, "\n")
		if !strings.Contains(joinedSetup, "ghx_f2p_test.go") {
			t.Errorf("fixture %q: setup does not inject a hidden F2P test file (ghx_f2p_test.go)", f.ID)
		}
		// The F2P command must run the injected hidden test, not a pre-existing
		// in-tree one, so the agent cannot see the grading criterion.
		joinedF2P := strings.Join(f.FailToPassCmds, "\n")
		if !strings.Contains(joinedF2P, "TestGHX") {
			t.Errorf("fixture %q: failToPass does not run a hidden TestGHX* test", f.ID)
		}
		for i, q := range f.ExplorationSubQuestions {
			if strings.TrimSpace(q.VerifiedAt) == "" {
				t.Errorf("fixture %q sub-question %d: missing verifiedAt provenance", f.ID, i)
			}
		}
	}
}

// TestCorpusRejectsFailedCanary proves the memorization-canary reject path is
// wired end-to-end: a fixture whose canary FAILED (a from-weights fix appeared
// at authoring time, ADR-0032.1 D3) is refused by the loader and never enters
// the corpus. This is the enforcement the S4 goal requires — canaries are not
// advisory metadata; a failed one disqualifies the task at load.
func TestCorpusRejectsFailedCanary(t *testing.T) {
	fixtures, err := LoadCorpus(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	// Take a real, otherwise-valid corpus fixture and flip only its canary.
	f := fixtures[0]
	f.MemorizationCanary.Result = CanaryFail
	err = f.RequireCorpusReady()
	if err == nil {
		t.Fatal("a failed memorization canary must disqualify the task, but RequireCorpusReady accepted it")
	}
	if !strings.Contains(err.Error(), "disqualifies the task") {
		t.Errorf("canary-reject error = %q, want it to mention disqualification", err)
	}
	// The same fixture with a passing canary is accepted — proving the canary
	// result is the load-time discriminator, not some unrelated field.
	f.MemorizationCanary.Result = CanaryPass
	if err := f.RequireCorpusReady(); err != nil {
		t.Fatalf("otherwise-valid fixture rejected with passing canary: %v", err)
	}
}
