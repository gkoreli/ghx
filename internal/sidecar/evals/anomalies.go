package evals

import (
	"fmt"
	"strings"
)

// AnomalySeverity classifies how an anomaly relates to the run.
//
//   - breaking: the episode's product contract was violated (a sidecar turn
//     produced no usable report despite a preflight-verified environment).
//   - soft: recovered or diagnostic patterns that cost nothing by themselves
//     but reveal failure modes worth tracking across runs.
type AnomalySeverity string

const (
	SeverityBreaking AnomalySeverity = "breaking"
	SeveritySoft     AnomalySeverity = "soft"
)

// Anomaly kinds — the declarative failure taxonomy (ADR-0016.7). Detection
// is derived from episode artifacts only, so any run (past or future) can be
// re-analyzed offline without re-running agents.
const (
	// AnomalySidecarBlocked: a sidecar turn's report is the BLOCKED escape
	// hatch even though preflight verified ghx is installed — the downstream
	// agent never ran ghx via the shell.
	AnomalySidecarBlocked = "sidecar_blocked_report"
	// AnomalySidecarReportMissing: a sidecar turn ended with the WARN
	// placeholder — no extractable <ghx-report>, even after the one-shot
	// corrective retry.
	AnomalySidecarReportMissing = "sidecar_report_missing"
	// AnomalySidecarReportUnparsed: diagnostic refinement of a missing
	// report — the turn's raw text does contain a <ghx-report> block, so the
	// agent complied and the parser rejected the payload (schema drift
	// beyond the current coercions).
	AnomalySidecarReportUnparsed = "sidecar_report_block_unparsed"
	// AnomalySidecarReportRetried: the turn only produced a report after
	// the one-shot corrective follow-up. Recovered, but the first attempt
	// failed — a soft reliability signal.
	AnomalySidecarReportRetried = "sidecar_report_retried"
	// AnomalySidecarReportCoerced: the final report was obtained only via the
	// lenient <ghx-report> coercion fallback (ADR-0021 D3) — the producer's JSON
	// did not fit the schema and had to be normalized. Recovered, but a soft
	// signal that the producer drifted; strict submit_report submissions never
	// set this. Counted so drift stays visible instead of silently absorbed
	// (Visibility and Truthfulness).
	AnomalySidecarReportCoerced = "sidecar_report_coerced"
	// AnomalyDirectGhxNoncompliance: a ghx-profile episode recorded zero
	// ghx invocations — the subject agent ignored the injected skill, which
	// weakens the ghx baseline that gates compare against.
	AnomalyDirectGhxNoncompliance = "direct_ghx_noncompliance"
	// AnomalyParallelRateLimited: an episode that ran under GHX_EVAL_PARALLEL
	// > 1 failed a turn with a provider rate-limit / back-pressure error
	// (ADR-0025 D3). Soft and purely declarative: the harness does NOT
	// auto-degrade parallelism mid-run — the signal is surfaced so a human
	// decides whether to lower GHX_EVAL_PARALLEL and re-run the affected cell.
	AnomalyParallelRateLimited = "eval_parallel_rate_limited"
)

// Anomaly is one declaratively-detected failure pattern on an episode.
type Anomaly struct {
	Kind     string          `json:"kind"`
	Severity AnomalySeverity `json:"severity"`
	Turn     int             `json:"turn"`
	Detail   string          `json:"detail,omitempty"`
}

func (a Anomaly) String() string {
	return fmt.Sprintf("[%s] %s turn %d: %s", a.Severity, a.Kind, a.Turn, a.Detail)
}

// DetectAnomalies derives the full anomaly list for one episode from its
// artifact fields. Pure and offline: no agent, no network, no scoring
// side effects — anomalies are observability, never reward inputs.
func DetectAnomalies(ep *Episode) []Anomaly {
	if ep == nil {
		return nil
	}
	var out []Anomaly
	if ep.Profile == ProfileSidecar {
		for _, turn := range ep.Turns {
			if turn.ReportRetried {
				out = append(out, Anomaly{
					Kind: AnomalySidecarReportRetried, Severity: SeveritySoft, Turn: turn.Turn,
					Detail: "report obtained only after a corrective retry",
				})
			}
			if turn.ReportCoerced {
				out = append(out, Anomaly{
					Kind: AnomalySidecarReportCoerced, Severity: SeveritySoft, Turn: turn.Turn,
					Detail: "report obtained only via lenient <ghx-report> coercion (schema drift on the fallback path)",
				})
			}
			if turn.Report == nil {
				continue
			}
			switch {
			case strings.HasPrefix(turn.Report.Answer, "BLOCKED:"):
				out = append(out, Anomaly{
					Kind: AnomalySidecarBlocked, Severity: SeverityBreaking, Turn: turn.Turn,
					Detail: fmt.Sprintf("report answer %q despite preflight-verified ghx", turn.Report.Answer),
				})
			case strings.HasPrefix(turn.Report.Answer, "WARN: sidecar did not emit"):
				out = append(out, Anomaly{
					Kind: AnomalySidecarReportMissing, Severity: SeverityBreaking, Turn: turn.Turn,
					Detail: "no extractable <ghx-report> after the one-shot retry",
				})
				if strings.Contains(turn.Text, "<ghx-report>") {
					out = append(out, Anomaly{
						Kind: AnomalySidecarReportUnparsed, Severity: SeveritySoft, Turn: turn.Turn,
						Detail: "raw text contains a <ghx-report> block — agent complied, parser rejected the payload (schema drift beyond current coercions)",
					})
				}
			}
		}
	}
	if ep.Profile == ProfileGhx && !invokesGhx(ep) {
		out = append(out, Anomaly{
			Kind: AnomalyDirectGhxNoncompliance, Severity: SeveritySoft, Turn: 0,
			Detail: "ghx profile episode recorded zero ghx invocations — baseline weakened",
		})
	}
	// Rate-limit fallback (ADR-0025 D3): only meaningful when the episode ran
	// concurrently — a rate-limit under parallelism is the visible cost of
	// fanning out, and the signal a human uses to dial GHX_EVAL_PARALLEL back.
	if ep.Parallel {
		for _, turn := range ep.Turns {
			if looksRateLimited(turn.Error) {
				out = append(out, Anomaly{
					Kind: AnomalyParallelRateLimited, Severity: SeveritySoft, Turn: turn.Turn,
					Detail: fmt.Sprintf("turn failed with a rate-limit-shaped error under GHX_EVAL_PARALLEL>1: %s", boundedString(turn.Error, 256)),
				})
			}
		}
	}
	return out
}

// AnomalyCount aggregates one taxonomy kind across a run.
type AnomalyCount struct {
	Kind     string          `json:"kind"`
	Severity AnomalySeverity `json:"severity"`
	Count    int             `json:"count"`
	Episodes int             `json:"episodes"`
}

// CountAnomalies aggregates DetectAnomalies over a run, in stable
// taxonomy order, for the verdict's declarative anomaly summary.
func CountAnomalies(episodes []*Episode) []AnomalyCount {
	order := []struct {
		kind     string
		severity AnomalySeverity
	}{
		{AnomalySidecarBlocked, SeverityBreaking},
		{AnomalySidecarReportMissing, SeverityBreaking},
		{AnomalySidecarReportUnparsed, SeveritySoft},
		{AnomalySidecarReportRetried, SeveritySoft},
		{AnomalySidecarReportCoerced, SeveritySoft},
		{AnomalyDirectGhxNoncompliance, SeveritySoft},
		{AnomalyParallelRateLimited, SeveritySoft},
	}
	counts := map[string]int{}
	episodesWith := map[string]int{}
	for _, ep := range episodes {
		seen := map[string]bool{}
		for _, a := range DetectAnomalies(ep) {
			counts[a.Kind]++
			if !seen[a.Kind] {
				episodesWith[a.Kind]++
				seen[a.Kind] = true
			}
		}
	}
	var out []AnomalyCount
	for _, o := range order {
		if counts[o.kind] > 0 {
			out = append(out, AnomalyCount{Kind: o.kind, Severity: o.severity, Count: counts[o.kind], Episodes: episodesWith[o.kind]})
		}
	}
	return out
}
