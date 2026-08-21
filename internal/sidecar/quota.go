package sidecar

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ADR-0040 L3 quota-degradation ladder: on backend quota exhaustion the ask
// must never die silently — it either ships a clearly-labeled DEGRADED report
// built from the session's durable evidence ledger, or fails as a typed error
// carrying an explicit next-move affordance.

// DegradedAnswerPrefix marks a quota-degraded report (ADR-0040 L3). Like
// BLOCKED, it is exempt from ValidateReportEvidence's fresh-evidence
// requirements, because a degraded answer by construction carries only cached
// ledger evidence and must not claim otherwise.
const DegradedAnswerPrefix = "DEGRADED"

// LedgerCacheBackend is the BackendsUsed ID stamped on degraded reports so no
// consumer mistakes cached evidence for a live backend run.
const LedgerCacheBackend = "ledger-cache"

// ErrQuotaExhausted is the typed failure for quota exhaustion with no usable
// ledger to degrade to (fresh sessions). The CLI maps it to exit code 3 plus
// the recovery affordance; the runtime never string-matches it.
var ErrQuotaExhausted = errors.New("backend quota exhausted")

// QuotaExhaustedError wraps ErrQuotaExhausted with the raw backend failure so
// diagnostics keep the underlying cause.
type QuotaExhaustedError struct {
	Err error
}

func (e *QuotaExhaustedError) Error() string {
	return "run turn: backend quota exhausted: " + e.Err.Error()
}

// Unwrap exposes both the sentinel (errors.Is → ErrQuotaExhausted) and the
// wrapped cause (errors.As/Is on the original turn failure).
func (e *QuotaExhaustedError) Unwrap() []error {
	return []error{ErrQuotaExhausted, e.Err}
}

// QuotaAffordanceHint is the fix-it line appended when a quota-dead ask has no
// ledger to degrade to.
const QuotaAffordanceHint = "try --depth cheap later or use ghx explore directly"

// NewQuotaExhaustedError builds the typed no-ledger quota failure.
func NewQuotaExhaustedError(cause error) error {
	return &QuotaExhaustedError{Err: cause}
}

// buildDegradedQuotaReport renders the DEGRADED report from the session's
// durable evidence ledger (ADR-0040 L3). It answers honestly from cache:
// prior verified claims and inspected paths ride through labeled as cached,
// staleness is called out against the ledger's snapshot pin (commit/branch,
// ADR-0037 M-2) and UpdatedAt, and BackendsUsed carries ONLY ledger-cache —
// no fresh evidence is claimed. Returns nil when there is nothing cached to
// answer from (no claims AND no paths), leaving the fail-fast path in charge.
func buildDegradedQuotaReport(state *askTurnState, quotaErr error) *Report {
	if state == nil || state.ledger == nil || !ledgerHasEvidence(state.ledger) {
		return nil
	}
	l := state.ledger

	reset := ""
	if hint := QuotaResetHint(quotaErr); hint != "" {
		reset = "; resets " + hint
	}

	answer := fmt.Sprintf("%s (quota): answered from cached session evidence; backend quota exhausted%s",
		DegradedAnswerPrefix, reset)
	if l.Repo != "" {
		answer += ". Repo: " + l.Repo
	}
	if len(l.RelevantFiles) > 0 {
		answer += "; most relevant prior paths: "
		paths := make([]string, 0, MaxRelevantFiles)
		for _, rf := range l.RelevantFiles {
			if rf.Path != "" {
				paths = append(paths, rf.Path)
			}
			if len(paths) == MaxRelevantFiles {
				break
			}
		}
		answer += strings.Join(paths, ", ")
	}

	report := &Report{
		SchemaVersion: ReportSchemaVersion,
		Answer:        answer,
		BackendsUsed:  []string{LedgerCacheBackend},
		Uncertainty:   []string{degradedStalenessNote(l)},
	}

	// Prior verified claims ride through as the degraded report's verified
	// list, each explicitly labeled [cached] so no reader mistakes them for
	// fresh evidence. Source: the session's prior reports (reports/*.json),
	// which is where verified claims with real evidence live — the ledger
	// itself stores commands/paths, not claims.
	if state.lastReport != nil {
		for _, c := range state.lastReport.Verified {
			if strings.TrimSpace(c.Summary) == "" {
				continue
			}
			ev := c.Evidence
			if strings.TrimSpace(ev) == "" {
				ev = "(cached report entry; original evidence text not recorded)"
			}
			report.Verified = append(report.Verified, Claim{
				Summary:  oneLine(c.Summary),
				Evidence: "[cached] " + oneLine(ev),
			})
			if len(report.Verified) >= maxDegradedClaims {
				break
			}
		}
	}
	for _, rf := range l.InspectedPaths {
		if strings.TrimSpace(rf.Value) == "" {
			continue
		}
		reason := "inspected in a prior turn"
		if rf.Turn > 0 {
			reason += fmt.Sprintf(" (turn %d)", rf.Turn)
		}
		report.RelevantFiles = append(report.RelevantFiles, RelevantFile{Path: rf.Value, Reason: reason})
		if len(report.RelevantFiles) >= MaxRelevantFiles {
			break
		}
	}

	return report
}

const maxDegradedClaims = 5

// degradedStalenessNote renders the honest staleness line for a degraded
// report's uncertainty list.
func degradedStalenessNote(l *Ledger) string {
	note := fmt.Sprintf("all evidence is CACHED from this session's ledger, not freshly gathered this turn; it may be stale (ledger last updated %s", l.UpdatedAt)
	if l.Commit != "" {
		note += "; snapshot pinned to commit " + l.Commit
	} else if l.Branch != "" {
		note += "; snapshot pinned to branch " + l.Branch
	} else {
		note += "; snapshot ref unknown"
	}
	if age, ok := ledgerAge(l); ok {
		note += fmt.Sprintf(", %s old at degradation time", age)
	}
	return note + ")"
}

// ledgerAge reports how long ago the ledger was written, best-effort from its
// RFC3339 UpdatedAt stamp; false when unparsable.
func ledgerAge(l *Ledger) (time.Duration, bool) {
	if l.UpdatedAt == "" {
		return 0, false
	}
	at, err := time.Parse(time.RFC3339, l.UpdatedAt)
	if err != nil {
		return 0, false
	}
	return time.Since(at).Round(time.Second), true
}

// ledgerHasEvidence reports whether the ledger holds anything worth degrading
// to: inspected paths, relevant files, or commands run (prior verified claims
// live in the session's last report, not the Ledger — see lastReport).
func ledgerHasEvidence(l *Ledger) bool {
	return l != nil && (len(l.InspectedPaths) > 0 ||
		len(l.RelevantFiles) > 0 ||
		len(l.CommandsRun) > 0)
}
