package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals"
)

func TestParseDisagreementCandidates_Fixture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SUMMARY.md")
	fixture := `# fixture

## Gate/Judge Disagreement (ADR-0023.1 D1)

Episodes to hand-review:
- alpha_ghx_111 — gates fail, judge overall 4.00 (good)
- beta_plain_222 — gates fail, judge overall 5.00 (good)

## Appendix — per-episode detail

| episode | judge overall | disagreement |
| --- | ---: | --- |
| ` + "`alpha_ghx_111`" + ` | 4.0 | gates fail / judge good |
| ` + "`gamma_ghx_333`" + ` | 5.0 | — |
`
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := parseDisagreementCandidates(path)
	if err != nil {
		t.Fatalf("parseDisagreementCandidates: %v", err)
	}
	// Must find exactly the two "Episodes to hand-review" bullets, deduped
	// against the differently-formatted appendix table row for the same
	// episode (alpha_ghx_111 appears in both places but must count once).
	want := map[string]float64{"alpha_ghx_111": 4.0, "beta_plain_222": 5.0}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d: %+v", len(got), len(want), got)
	}
	for _, c := range got {
		wantScore, ok := want[c.EpisodeID]
		if !ok {
			t.Errorf("unexpected candidate %s", c.EpisodeID)
			continue
		}
		if c.SameFamilyScore != wantScore {
			t.Errorf("%s: score = %v, want %v", c.EpisodeID, c.SameFamilyScore, wantScore)
		}
	}
	// gamma_ghx_333 has no disagreement marker and must not appear.
	for _, c := range got {
		if c.EpisodeID == "gamma_ghx_333" {
			t.Errorf("gamma_ghx_333 should not be a candidate (no disagreement marker)")
		}
	}
}

func TestParseDisagreementCandidates_NoMatches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "SUMMARY.md")
	if err := os.WriteFile(path, []byte("# nothing here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := parseDisagreementCandidates(path); err == nil {
		t.Fatal("expected an error when no candidates are present, got nil")
	}
}

// TestParseDisagreementCandidates_RealSummary locks in the exact candidate
// count and set this follow-up is scoped to: the committed first-sweep
// SUMMARY.md must still parse to exactly the 27 "gates fail, judge good"
// episodes the TRUST H1 follow-up was queued against. If this test starts
// failing, the committed SUMMARY.md changed shape or content — investigate
// before re-running the cross-family sweep against a different set.
func TestParseDisagreementCandidates_RealSummary(t *testing.T) {
	// defaultFirstSummary is a repo-root-relative path (correct for `go run`
	// invocations from the repo root); `go test` runs with the package
	// directory as its working directory, so walk up to the sweep dir here.
	const realSummaryFromPackageDir = "../../SUMMARY.md"
	got, err := parseDisagreementCandidates(realSummaryFromPackageDir)
	if err != nil {
		t.Fatalf("parseDisagreementCandidates(%s): %v", realSummaryFromPackageDir, err)
	}
	const wantCount = 27
	if len(got) != wantCount {
		t.Fatalf("got %d candidates from %s, want %d", len(got), defaultFirstSummary, wantCount)
	}
	wantIDs := []string{
		"express-router-location_ghx-sidecar_1783294920428",
		"express-router-location_ghx-sidecar_1783296136926",
		"express-router-location_ghx_1783293764685",
		"express-router-location_plain_1783298460843",
		"express-router-location_plain_1783310655275",
		"ghx-mapengine_ghx_1783294017695",
		"ghx-mapengine_ghx_1783311051515",
		"ghx-mapengine_plain_1783296331466",
		"ghx-mapengine_plain_1783310984090",
		"gin-routing_ghx-sidecar_1783309817215",
		"gin-routing_ghx-sidecar_1783311354381",
		"gin-routing_ghx-sidecar_1783313384747",
		"gin-routing_ghx_1783294157688",
		"gin-routing_ghx_1783295296326",
		"gin-routing_ghx_1783296534553",
		"gin-routing_ghx_1783297736205",
		"gin-routing_ghx_1783298949511",
		"gin-routing_ghx_1783311240390",
		"gin-routing_ghx_1783313256903",
		"gin-routing_plain_1783294098500",
		"gin-routing_plain_1783295236474",
		"gin-routing_plain_1783296475788",
		"gin-routing_plain_1783298875531",
		"gin-routing_plain_1783309625204",
		"gin-routing_plain_1783311159918",
		"gin-routing_plain_1783313186140",
		"hono-middleware_ghx-sidecar_1783310129190",
	}
	gotIDs := map[string]bool{}
	for _, c := range got {
		gotIDs[c.EpisodeID] = true
	}
	for _, id := range wantIDs {
		if !gotIDs[id] {
			t.Errorf("missing expected candidate %s", id)
		}
	}
	if len(wantIDs) != wantCount {
		t.Fatalf("test fixture drift: wantIDs has %d entries, wantCount is %d", len(wantIDs), wantCount)
	}
}

func TestMajorityVerdict(t *testing.T) {
	cases := []struct {
		name      string
		overalls  []float64
		wantGood  bool
		wantCount int
	}{
		{"all good", []float64{4, 5, 4.5}, true, 3},
		{"all poor", []float64{1, 2, 2.4}, false, 0},
		{"majority good 2-1", []float64{4, 2, 5}, true, 2},
		{"majority poor 1-2", []float64{4, 2, 1}, false, 1},
		{"boundary exactly poor threshold counts good", []float64{2.5, 1, 1}, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := &evals.JudgeResult{}
			for _, o := range tc.overalls {
				res.Verdicts = append(res.Verdicts, evals.JudgeVerdict{Overall: o})
			}
			good, count, total := majorityVerdict(res)
			if good != tc.wantGood {
				t.Errorf("good = %v, want %v", good, tc.wantGood)
			}
			if count != tc.wantCount {
				t.Errorf("goodCount = %d, want %d", count, tc.wantCount)
			}
			if total != len(tc.overalls) {
				t.Errorf("total = %d, want %d", total, len(tc.overalls))
			}
		})
	}
}
