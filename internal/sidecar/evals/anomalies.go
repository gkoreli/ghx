package evals

import (
	"fmt"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
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
	// AnomalyAnswerDocContamination: a tool call read a path matching one of
	// the task's pre-registered checks.contaminationPaths prefixes — an
	// answer-bearing document inside the subject repo (e.g. ghx's own ADRs on
	// a self-referential task, ADR-0016.8 D2). Soft and declarative like the
	// rest of the taxonomy, but gate aggregates exclude flagged episodes: the
	// score no longer measures exploration once the answer sheet was read.
	// Applies to every profile equally — it protected the baseline in the
	// 2026-07-05 audit; next time it could inflate the sidecar.
	AnomalyAnswerDocContamination = "answer_doc_contamination"
	// AnomalyTurnCapWrapUp: the adapter's max-turns safety net fired and the
	// runtime recovered the exploration with the one-shot wrap-up resume
	// (ADR-0027 D1). Soft: the episode recovered, but the count shows how
	// often the net fires — a rising count means budgets are mis-sized.
	AnomalyTurnCapWrapUp = "turn_cap_wrapup"
	// AnomalyEpisodeHangTimeout: a turn was cancelled by the sidecar liveness
	// watchdog after producing no ACP session update for the whole liveness
	// window (ADR-0027 D2). Breaking: the episode's turn died without a
	// usable report; detected from the persisted turn error string.
	AnomalyEpisodeHangTimeout = "episode_hang_timeout"
	// AnomalyTraceCaptureGap: the raw-SDK audit (ADR-0016.10, TRUST H3) shows
	// tool activity the ACP capture path missed — a raw tool_use with no
	// captured trace, a missing terminal status, a mangled input, or dropped
	// output. Soft, deliberately: it flags measurement *undercounting* of
	// trajectory/tool metrics, not a product-contract violation — the answer
	// and report (which correctness/evidence read) are unaffected, and the
	// bias makes the subject look less active, never better. Applies only to
	// runs whose turns carry the rawSDK field (ADR-0016.10 D5); a human
	// reading the anomaly table decides any exclusion.
	AnomalyTraceCaptureGap = "trace_capture_gap"
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
	// Liveness watchdog cancellations (ADR-0027 D2) are profile-independent:
	// any turn whose persisted error carries the watchdog marker hung.
	for _, turn := range ep.Turns {
		if strings.Contains(turn.Error, sidecar.LivenessTimeoutMarker) {
			out = append(out, Anomaly{
				Kind: AnomalyEpisodeHangTimeout, Severity: SeverityBreaking, Turn: turn.Turn,
				Detail: fmt.Sprintf("turn cancelled by the liveness watchdog: %s", boundedString(turn.Error, 256)),
			})
		}
	}
	if ep.Profile == ProfileSidecar {
		for _, turn := range ep.Turns {
			if turn.WrapUpRecovered {
				out = append(out, Anomaly{
					Kind: AnomalyTurnCapWrapUp, Severity: SeveritySoft, Turn: turn.Turn,
					Detail: "max-turns safety net fired; exploration recovered via the one-shot wrap-up resume (ADR-0027 D1)",
				})
			}
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
	// Trace-capture completeness (ADR-0016.10 D4): compare each turn's
	// raw-SDK audit against its live captured traces — replayed traces were
	// accounted on the turn that originally ran them (ADR-0016.5). At most
	// one anomaly per turn; turns without a raw audit are skipped inside the
	// comparator, so pre-ADR artifacts derive zero anomalies here.
	for _, turn := range ep.Turns {
		if gaps := CompareTraceCapture(turn.RawSDK, turn.ToolTraces); len(gaps) > 0 {
			out = append(out, Anomaly{
				Kind: AnomalyTraceCaptureGap, Severity: SeveritySoft, Turn: turn.Turn,
				Detail: boundedString(summarizeTraceCaptureGaps(gaps), 256),
			})
		}
	}
	// Contamination guard (ADR-0016.8 D2): declarative substring detection
	// over live tool calls only — replayed traces were already accounted on
	// the turn that originally ran them (ADR-0016.5). One anomaly per
	// turn × prefix keeps the list bounded on doc-heavy trajectories.
	for _, turn := range ep.Turns {
		for _, prefix := range ep.Checks.ContaminationPaths {
			lp := strings.ToLower(prefix)
			for _, call := range turn.ToolCalls {
				if strings.Contains(strings.ToLower(call), lp) {
					out = append(out, Anomaly{
						Kind: AnomalyAnswerDocContamination, Severity: SeveritySoft, Turn: turn.Turn,
						Detail: fmt.Sprintf("tool call read answer-bearing path %q: %s", prefix, boundedString(call, 256)),
					})
					break
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
	// Host-task rows (ADR-0032.1 S3): compliance + BLOCKED detection over the
	// persisted HostTask record; recon episodes derive nothing there.
	out = append(out, detectHostTaskAnomalies(ep)...)
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
		{AnomalyEpisodeHangTimeout, SeverityBreaking},
		{AnomalyHostRateLimited, SeverityBreaking},
		{AnomalyReconToolUnavailable, SeverityBreaking},
		{AnomalySidecarReportUnparsed, SeveritySoft},
		{AnomalySidecarReportRetried, SeveritySoft},
		{AnomalySidecarReportCoerced, SeveritySoft},
		{AnomalyTurnCapWrapUp, SeveritySoft},
		{AnomalyDirectGhxNoncompliance, SeveritySoft},
		{AnomalyParallelRateLimited, SeveritySoft},
		{AnomalyAnswerDocContamination, SeveritySoft},
		{AnomalyTraceCaptureGap, SeveritySoft},
		{AnomalyHostExternalExploration, SeveritySoft},
		{AnomalyHostExecuteOutsideWorkspace, SeveritySoft},
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
