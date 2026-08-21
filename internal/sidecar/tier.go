package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// Tier decision recording for one Ask turn (ADR-0024.2). Every turn —
// escalated or not, failed or not — gets one deterministic policy evaluation
// over recorded observables, persisted as a ghx.tier.decision span (emit.go),
// a tier-decisions.jsonl line in the session dir, and tierUsed provenance on
// the report. The evaluation is post-hoc by design in v1: the sidecar reaches
// tier2 via shell commands inside the ACP turn, so the runtime records and
// reconciles rather than intercepts (ADR-0024.2 D3 "honest deviation").

// tierDecisionsFileName is the session-dir audit file, one JSON line per turn
// (ADR-0024.1 "Session Artifacts").
const tierDecisionsFileName = "tier-decisions.jsonl"

// Tier-used provenance sources (ADR-0024.2 D4).
const (
	tierSourceAgent   = "agent"
	tierSourceRuntime = "runtime"
)

// TierDecisionRecord is one turn's recorded tier decision plus reconciliation
// against what the agent actually did.
type TierDecisionRecord struct {
	// Time is when the record was assembled (turn end).
	Time time.Time `json:"time"`
	// Session and Repo scope the record; Turn is the 1-based turn number.
	Session string `json:"session"`
	Repo    string `json:"repo,omitempty"`
	Turn    int    `json:"turn"`
	// Decision is the pure policy evaluation, observations included, so the
	// outcome is recomputable from this record alone.
	Decision tier2.EscalationDecision `json:"decision"`
	// TierUsed is the effective report tierUsed: the agent-declared value
	// when present (never overwritten), else the runtime-derived one.
	TierUsed string `json:"tierUsed"`
	// TierUsedSource is "agent" or "runtime" (ADR-0024.2 D4).
	TierUsedSource string `json:"tierUsedSource"`
	// RuntimeDerivedTierUsed is always recorded so an agent-declared value
	// that disagrees with observed commands stays a visible anomaly.
	RuntimeDerivedTierUsed string `json:"runtimeDerivedTierUsed"`
	// EscalationUsed reports whether any `ghx tier2` command was observed.
	// EscalationUsed && !Decision.Allowed is the loud anomaly shape.
	EscalationUsed bool `json:"escalationUsed"`
	// Clone is the tier2 provenance block parsed from recorded tool output
	// when tier2 ran via shell; nil when not visible (best-effort, D5).
	Clone *tier2.Provenance `json:"clone,omitempty"`
}

// buildTierDecisionRecord evaluates the pre-registered v1 policy over this
// turn's recorded observables and assembles the decision record. When the
// report is present and carries no tierUsed, it is backfilled with the
// runtime-derived tier (labeled tierUsedSource: "runtime") so "which tier
// answered" is visible in the report immediately; an agent-declared tierUsed
// is preserved untouched.
func buildTierDecisionRecord(req AskRequest, turn int, result *TurnResult, report *Report, at time.Time) TierDecisionRecord {
	var commands []string
	var outputs []string
	if result != nil {
		for _, tr := range result.ToolTraces {
			if cmd := strings.TrimSpace(commandFromRawInput(tr.RawInput)); cmd != "" {
				commands = append(commands, cmd)
			}
			if tr.OutputExcerpt != "" {
				outputs = append(outputs, tr.OutputExcerpt)
			}
		}
	}

	escalationUsed := anyTier2Command(commands)
	obs := tier2.EscalationObservations{
		Question:                    req.Question,
		AllowedBackends:             req.AllowedBackends,
		SearchesWithoutAcceptedRead: countSearchesWithoutAcceptedRead(commands),
		LowConfidenceRemoteOnly:     lowConfidenceRemoteOnly(report, escalationUsed),
	}

	// Agent-declared semantic signals (ADR-0024.4 D1): the agent may declare
	// the three judgment-based signals mid-turn via `ghx tier2 observe
	// --signal <id>`. A declaration is accepted only when an observe command
	// for that signal is actually present in the turn's recorded traces; the
	// accepted set is recorded on the decision so divergence stays auditable.
	for _, sig := range []struct {
		id    string
		isSet *bool
	}{
		{tier2.SignalSymbolAbsentAfterMaps, &obs.SymbolAbsentAfterMaps},
		{tier2.SignalCandidateAmbiguous, &obs.CandidateAmbiguous},
		{tier2.SignalImportChainInvisible, &obs.ImportChainInvisible},
	} {
		if declaredSignal(commands, sig.id) {
			*sig.isSet = true
		}
	}
	rec := TierDecisionRecord{
		Time:                   at,
		Session:                req.Session,
		Repo:                   req.Repo,
		Turn:                   turn,
		Decision:               tier2.DefaultPolicy().Evaluate(obs),
		RuntimeDerivedTierUsed: deriveTierUsedValue(commands).String(),
		EscalationUsed:         escalationUsed,
	}

	rec.TierUsed, rec.TierUsedSource = rec.RuntimeDerivedTierUsed, tierSourceRuntime
	if report != nil {
		if report.TierUsed != "" {
			// Agent-declared tierUsed is itself evidence: preserved, never
			// overwritten; divergence from RuntimeDerivedTierUsed stays
			// visible in this record (ADR-0024.2 D4).
			rec.TierUsed, rec.TierUsedSource = report.TierUsed, tierSourceAgent
		} else {
			report.TierUsed = rec.RuntimeDerivedTierUsed
		}
	}

	// Clone provenance: parse the first tier2 provenance block visible in the
	// turn's recorded tool output (best-effort, ADR-0024.2 D5). FullText is
	// checked too — some adapters replay subprocess stderr into message text.
	if escalationUsed {
		texts := outputs
		if result != nil {
			texts = append(texts, result.FullText)
		}
		for _, text := range texts {
			if prov, ok := tier2.FindProvenance(text); ok {
				rec.Clone = &prov
				break
			}
		}
	}
	return rec
}

// anyTier2Command reports whether any observed shell command invokes the
// tier2 CLI surface.
func anyTier2Command(commands []string) bool {
	for _, cmd := range commands {
		if isGhxSubcommand(cmd, "tier2") {
			return true
		}
	}
	return false
}

// declaredSignal reports whether the turn's recorded commands contain a
// `ghx tier2 observe --signal <id>` declaration for sig (ADR-0024.4 D1).
// The trace is the only acceptance source: an agent claiming a signal in its
// report without the recorded observe invocation is not accepted — the
// policy stays recomputable from wire evidence alone.
func declaredSignal(commands []string, signal string) bool {
	for _, cmd := range commands {
		if !isGhxSubcommand(cmd, "tier2") {
			continue
		}
		fields := strings.Fields(cmd)
		if len(fields) < 3 || fields[2] != "observe" {
			continue
		}
		for i, f := range fields {
			if f == "--signal" && i+1 < len(fields) && fields[i+1] == signal {
				return true
			}
			if strings.HasPrefix(f, "--signal=") && strings.TrimPrefix(f, "--signal=") == signal {
				return true
			}
		}
	}
	return false
}

// deriveTierUsed maps observed commands to the highest tier used
// (ADR-0024.2 D4): tier2 when a `ghx tier2` command ran, tier1 when any other
// ghx command ran, tier0 when the turn answered from session memory alone.
func deriveTierUsedValue(commands []string) Tier {
	if anyTier2Command(commands) {
		return Tier2
	}
	for _, cmd := range commands {
		if isGhxCommand(cmd) {
			return Tier1
		}
	}
	return Tier0
}

func deriveTierUsed(commands []string) string {
	return deriveTierUsedValue(commands).String()
}

// countSearchesWithoutAcceptedRead implements the pre-registered v1
// derivation of remote.search_exhausted (ADR-0024.2 D2): a `ghx search`
// counts as unaccepted when no `ghx read` or `ghx inspect` follows it before
// the next `ghx search` (or turn end).
func countSearchesWithoutAcceptedRead(commands []string) int {
	count := 0
	pending := false
	for _, cmd := range commands {
		switch {
		case isGhxSubcommand(cmd, "search"):
			if pending {
				count++
			}
			pending = true
		case isGhxSubcommand(cmd, "read") || isGhxSubcommand(cmd, "inspect"):
			pending = false
		}
	}
	if pending {
		count++
	}
	return count
}

// lowConfidenceRemoteOnly implements the pre-registered v1 derivation of
// answer.low_confidence_remote_only (ADR-0024.2 D2): the report ships
// non-empty uncertainty and neither its backendsUsed nor the observed
// commands show local structural evidence.
func lowConfidenceRemoteOnly(report *Report, escalationUsed bool) bool {
	if report == nil || len(report.Uncertainty) == 0 || escalationUsed {
		return false
	}
	for _, b := range report.BackendsUsed {
		backend, ok := ParseBackend(b)
		if ok && backend.IsLocal() && isRegisteredLocalBackend(backend.String()) {
			return false
		}
	}
	return true
}

// isGhxCommand reports whether the shell command line invokes ghx (possibly
// path-qualified, e.g. "./ghx" or "/usr/local/bin/ghx").
func isGhxCommand(cmd string) bool {
	fields := strings.Fields(cmd)
	return len(fields) > 0 && (fields[0] == "ghx" || strings.HasSuffix(fields[0], "/ghx"))
}

// isGhxSubcommand reports whether the shell command line is `ghx <sub> ...`.
func isGhxSubcommand(cmd, sub string) bool {
	if !isGhxCommand(cmd) {
		return false
	}
	fields := strings.Fields(cmd)
	return len(fields) > 1 && fields[1] == sub
}

// AppendTierDecision appends one record to the session's
// tier-decisions.jsonl. Best-effort contract at the call site: failures warn
// to stderr and never fail the ask (ADR-0022 emission model).
func AppendTierDecision(sessionsDir, session string, rec TierDecisionRecord) error {
	dir := sessionDir(sessionsDir, session)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, tierDecisionsFileName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(data, '\n')); err != nil {
		return err
	}
	return f.Close()
}

// recordTierDecision is the one-stop wiring used by the Ask runtime on every
// exit path that has a turn: evaluate, backfill report provenance, persist
// the jsonl line, and hand the record to telemetry emission.
func recordTierDecision(sessionsDir string, req AskRequest, turn int, result *TurnResult, report *Report) *TierDecisionRecord {
	rec := buildTierDecisionRecord(req, turn, result, report, time.Now().UTC())
	if err := AppendTierDecision(sessionsDir, req.Session, rec); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to append tier decision: %v\n", err)
	}
	return &rec
}
