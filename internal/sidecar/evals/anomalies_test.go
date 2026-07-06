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

// TestDetectAnomaliesReportCoerced pins ADR-0021 D3: a turn whose report was
// obtained only via lenient coercion emits the soft sidecar_report_coerced
// anomaly so drift stays counted.
func TestDetectAnomaliesReportCoerced(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{
				Turn:          0,
				ReportCoerced: true,
				Report:        &sidecar.Report{Answer: "Recovered via coercion."},
			},
		},
	}
	a, ok := anomalyByKind(DetectAnomalies(ep), AnomalySidecarReportCoerced)
	if !ok {
		t.Fatalf("missing %s anomaly", AnomalySidecarReportCoerced)
	}
	if a.Severity != SeveritySoft {
		t.Fatalf("severity = %s, want soft", a.Severity)
	}
	// It must also be aggregated by CountAnomalies in taxonomy order.
	counts := CountAnomalies([]*Episode{ep})
	found := false
	for _, c := range counts {
		if c.Kind == AnomalySidecarReportCoerced && c.Count == 1 {
			found = true
		}
	}
	if !found {
		t.Fatalf("CountAnomalies did not aggregate %s: %+v", AnomalySidecarReportCoerced, counts)
	}
}

// TestDetectAnomaliesTurnCapWrapUp pins ADR-0027 D1: a turn recovered via the
// wrap-up resume counts as the soft turn_cap_wrapup anomaly — recovered, but
// the safety net's firing rate stays visible.
func TestDetectAnomaliesTurnCapWrapUp(t *testing.T) {
	ep := &Episode{
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{
				Turn:            0,
				WrapUpRecovered: true,
				Report:          &sidecar.Report{Answer: "Recovered by wrap-up."},
			},
		},
	}
	a, ok := anomalyByKind(DetectAnomalies(ep), AnomalyTurnCapWrapUp)
	if !ok {
		t.Fatalf("missing %s anomaly", AnomalyTurnCapWrapUp)
	}
	if a.Severity != SeveritySoft {
		t.Fatalf("severity = %s, want soft", a.Severity)
	}
	counts := CountAnomalies([]*Episode{ep})
	if c := mustAnomalyCount(t, counts, AnomalyTurnCapWrapUp); c.Count != 1 || c.Episodes != 1 {
		t.Fatalf("aggregate = %+v, want count 1 across 1 episode", c)
	}
}

// TestDetectAnomaliesEpisodeHangTimeout pins ADR-0027 D2: a turn whose error
// carries the liveness watchdog marker is the breaking episode_hang_timeout
// anomaly, on any profile — hang detection is profile-independent.
func TestDetectAnomaliesEpisodeHangTimeout(t *testing.T) {
	for _, profile := range []Profile{ProfileSidecar, ProfileGhx, ProfilePlain} {
		t.Run(string(profile), func(t *testing.T) {
			ep := &Episode{
				Profile: profile,
				Turns: []TurnRecord{
					{
						Turn:  0,
						Error: "sidecar turn 0: run turn: acp prompt: sidecar " + sidecar.LivenessTimeoutMarker + ": no ACP session update for 10m0s",
					},
				},
			}
			a, ok := anomalyByKind(DetectAnomalies(ep), AnomalyEpisodeHangTimeout)
			if !ok {
				t.Fatalf("missing %s anomaly", AnomalyEpisodeHangTimeout)
			}
			if a.Severity != SeverityBreaking {
				t.Fatalf("severity = %s, want breaking", a.Severity)
			}
		})
	}
	// A turn that failed for another reason must not be classified as a hang.
	ep := &Episode{Profile: ProfileSidecar, Turns: []TurnRecord{{Turn: 0, Error: "acp prompt: peer connection closed"}}}
	if _, ok := anomalyByKind(DetectAnomalies(ep), AnomalyEpisodeHangTimeout); ok {
		t.Fatal("non-watchdog failure misclassified as episode_hang_timeout")
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
