package evals

import (
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func TestDetectAnomaliesTaxonomyKinds(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{
				Turn:   0,
				Report: &sidecar.Report{Answer: "BLOCKED: ghx is unavailable in this sidecar session."},
			},
			{
				Turn:   1,
				Report: &sidecar.Report{Answer: "WARN: sidecar did not emit a <ghx-report> block — raw output was produced but no structured report was found."},
			},
			{
				Turn:   2,
				Text:   `done <ghx-report>{"answer":42}</ghx-report>`,
				Report: &sidecar.Report{Answer: "WARN: sidecar did not emit a <ghx-report> block — raw output was produced but no structured report was found."},
			},
			{
				Turn:          3,
				ReportRetried: true,
				Report:        &sidecar.Report{Answer: "Recovered report."},
			},
		},
	}

	anomalies := DetectAnomalies(ep)
	want := map[string]AnomalySeverity{
		AnomalySidecarBlocked:        SeverityBreaking,
		AnomalySidecarReportMissing:  SeverityBreaking,
		AnomalySidecarReportUnparsed: SeveritySoft,
		AnomalySidecarReportRetried:  SeveritySoft,
	}
	for kind, severity := range want {
		a, ok := anomalyByKind(anomalies, kind)
		if !ok {
			t.Fatalf("missing anomaly kind %s in %#v", kind, anomalies)
		}
		if a.Severity != severity {
			t.Fatalf("%s severity = %s, want %s", kind, a.Severity, severity)
		}
		if !strings.Contains(a.String(), kind) || !strings.Contains(a.String(), string(severity)) {
			t.Fatalf("String() should include kind and severity, got %q", a.String())
		}
	}
}

func TestDetectAnomaliesDirectGhxNoncompliance(t *testing.T) {
	ep := &Episode{
		Profile: ProfileGhx,
		Actions: []Action{{Input: "gh api repos/o/r/contents/src/a.ts"}},
	}
	anomalies := DetectAnomalies(ep)
	if len(anomalies) != 1 {
		t.Fatalf("anomalies = %#v, want one", anomalies)
	}
	if anomalies[0].Kind != AnomalyDirectGhxNoncompliance || anomalies[0].Severity != SeveritySoft {
		t.Fatalf("unexpected anomaly: %#v", anomalies[0])
	}
}

func TestDetectAnomaliesNonSidecarProfilesDoNotEmitSidecarKinds(t *testing.T) {
	for _, profile := range []Profile{ProfilePlain, ProfileGhx} {
		t.Run(string(profile), func(t *testing.T) {
			ep := &Episode{
				Profile: profile,
				Turns: []TurnRecord{
					{
						Turn:          0,
						Text:          `<ghx-report>{bad}</ghx-report>`,
						ReportRetried: true,
						Report:        &sidecar.Report{Answer: "BLOCKED: not relevant"},
					},
				},
				Actions: []Action{{Input: "ghx read o/r src/a.ts"}},
			}
			for _, a := range DetectAnomalies(ep) {
				if strings.HasPrefix(a.Kind, "sidecar_") {
					t.Fatalf("non-sidecar profile emitted sidecar anomaly: %#v", a)
				}
			}
		})
	}
}

func TestCountAnomaliesAggregatesCountsAndEpisodes(t *testing.T) {
	episodes := []*Episode{
		{
			Profile: ProfileSidecar,
			Turns: []TurnRecord{
				{Turn: 0, Report: &sidecar.Report{Answer: "BLOCKED: no ghx"}},
				{Turn: 1, Report: &sidecar.Report{Answer: "BLOCKED: still no ghx"}},
			},
		},
		{
			Profile: ProfileSidecar,
			Turns: []TurnRecord{
				{Turn: 0, Report: &sidecar.Report{Answer: "BLOCKED: no ghx"}},
			},
		},
		{
			Profile: ProfileGhx,
			Actions: []Action{{Input: "gh api repos/o/r/contents/src/a.ts"}},
		},
	}

	counts := CountAnomalies(episodes)
	blocked := mustAnomalyCount(t, counts, AnomalySidecarBlocked)
	if blocked.Count != 3 || blocked.Episodes != 2 {
		t.Fatalf("blocked count = %+v, want count 3 across 2 episodes", blocked)
	}
	direct := mustAnomalyCount(t, counts, AnomalyDirectGhxNoncompliance)
	if direct.Count != 1 || direct.Episodes != 1 {
		t.Fatalf("direct count = %+v, want count 1 across 1 episode", direct)
	}
}

func anomalyByKind(anomalies []Anomaly, kind string) (Anomaly, bool) {
	for _, a := range anomalies {
		if a.Kind == kind {
			return a, true
		}
	}
	return Anomaly{}, false
}

func mustAnomalyCount(t *testing.T, counts []AnomalyCount, kind string) AnomalyCount {
	t.Helper()
	for _, c := range counts {
		if c.Kind == kind {
			return c
		}
	}
	t.Fatalf("missing anomaly count for %s in %#v", kind, counts)
	return AnomalyCount{}
}
