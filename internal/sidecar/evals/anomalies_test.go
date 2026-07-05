package evals

import (
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func TestEpisodeAnomaliesFlagsBlockedAndWarn(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{Turn: 0, Report: &sidecar.Report{Answer: "BLOCKED: ghx is unavailable in this sidecar session."}},
			{Turn: 1, Report: &sidecar.Report{Answer: "WARN: sidecar did not emit a <ghx-report> block — raw output was produced but no structured report was found."}},
			{Turn: 2, Report: &sidecar.Report{Answer: "Middleware composition lives in src/compose.ts."}},
		},
	}
	anomalies := EpisodeAnomalies(ep)
	if len(anomalies) != 2 {
		t.Fatalf("anomalies = %v, want 2", anomalies)
	}
	if !strings.Contains(anomalies[0], "BLOCKED") || !strings.Contains(anomalies[1], "retry") {
		t.Fatalf("unexpected anomaly wording: %v", anomalies)
	}
}

func TestEpisodeAnomaliesIgnoresDirectProfilesAndHealthyReports(t *testing.T) {
	direct := &Episode{
		Profile: ProfileGhx,
		Turns:   []TurnRecord{{Report: &sidecar.Report{Answer: "BLOCKED: whatever"}}},
	}
	if a := EpisodeAnomalies(direct); a != nil {
		t.Fatalf("direct profiles are not sidecar anomalies, got %v", a)
	}
	healthy := &Episode{
		Profile: ProfileSidecar,
		Turns:   []TurnRecord{{Report: &sidecar.Report{Answer: "found it"}}},
	}
	if a := EpisodeAnomalies(healthy); a != nil {
		t.Fatalf("healthy sidecar episode flagged: %v", a)
	}
	if a := EpisodeAnomalies(nil); a != nil {
		t.Fatalf("nil episode flagged: %v", a)
	}
}
