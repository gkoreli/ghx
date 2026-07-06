package sidecar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

var runTurnWithOptions = RunTurnWithOptions
var checkACPHandshake = CheckACPHandshake

// maxReportRetries bounds the corrective follow-ups after the initial turn
// (ADR-0021 D2). Two retries balance closing the loop on a recoverable
// producer mistake against runaway token cost on a fundamentally confused
// model; when they are exhausted the WARN fallback ships and evals count it as
// a breaking anomaly, keeping the failure loud.
const maxReportRetries = 2

// reportRetryPromptWithError is the corrective follow-up sent when a turn did
// not yield a valid report. Unlike the old static nudge (ADR-0016.7 RC3), it
// embeds the CONCRETE validation reason (ADR-0021 D2/D3) so the producer can
// fix the specific defect.
func reportRetryPromptWithError(reason string) string {
	if reason == "" {
		reason = "no valid report was produced"
	}
	return fmt.Sprintf(`Your previous reply did not produce a valid report.

Reason: %s

Fix it now based on the work you already did. If the submit_report tool is
available, call it again with a corrected report object — the turn is complete
only after submit_report accepts it. If that tool is unavailable, output ONLY
the report JSON inside <ghx-report></ghx-report> tags, following the schema from
your instructions exactly: all list fields must be JSON arrays and "answer" must
be a non-empty string. Do not run any more commands. No text before or after.`, reason)
}

// newReportSinkPath creates a runtime-owned sink path for one Ask invocation
// (ADR-0021 D1). The returned cleanup removes the temp dir. On failure it
// returns an empty path and a no-op cleanup; the runtime then simply falls back
// to the <ghx-report> text path.
func newReportSinkPath() (path string, cleanup func()) {
	dir, err := os.MkdirTemp("", "ghx-report-sink-")
	if err != nil {
		return "", func() {}
	}
	return filepath.Join(dir, "report.json"), func() { _ = os.RemoveAll(dir) }
}

// resolveTurnReport determines the turn's report, preferring a strictly-validated
// submit_report sink over the lenient <ghx-report> text block (ADR-0021 D2). It
// returns the report (nil if none), whether coercion was applied on the text
// path, and a concrete failure reason to feed the corrective retry.
func resolveTurnReport(sinkPath, text string) (report *Report, coerced bool, reason string) {
	if sinkPath != "" {
		r, err := ReadSinkReport(sinkPath)
		if err != nil {
			// The sink is only ever written canonical, so a read error is
			// unexpected; note it but still try the text path.
			reason = err.Error()
		} else if r != nil {
			return r, false, ""
		}
	}
	r, coerced, err := ExtractReportErr(text)
	if err != nil {
		return nil, false, err.Error()
	}
	return r, coerced, ""
}

// mergeRetryTurn folds a corrective retry's telemetry into the primary turn
// result so eval accounting stays honest across the whole Ask.
func mergeRetryTurn(dst *TurnResult, src TurnResult) {
	dst.ReportRetried = true
	dst.FullText += src.FullText
	dst.ToolCalls = append(dst.ToolCalls, src.ToolCalls...)
	dst.ToolTraces = append(dst.ToolTraces, src.ToolTraces...)
	dst.ToolOutputChars += src.ToolOutputChars
}

// AskRequest is the input for a single sidecar investigation.
type AskRequest struct {
	// Session is the named session to use (created if it does not exist).
	Session string
	// Repo is the GitHub repo under investigation ("owner/repo").
	Repo string
	// Question is the English question about the repo.
	Question string
	// Depth controls command budget: "cheap", "normal", or "deep".
	Depth string
	// AllowedBackends lists evidence backends the agent may use.
	AllowedBackends []string
	// Scope is a short label written into the session metadata.
	// Ignored on follow-up turns (metadata already exists).
	Scope string
}

// Ask executes one sidecar investigation turn and returns the evidence report
// together with the raw TurnResult (full streamed text and observed tool
// calls) so callers such as the eval runner can account for the internal
// exploration the main agent never sees.
//
// Session lifecycle:
//   - If the named session has never been used, InitSession is called and
//     an ACP NewSession is created.
//   - If the session exists and the adapter supports ACP LoadSession, the
//     transport session is resumed. Otherwise a fresh ACP NewSession is used;
//     durable evidence memory still travels through the prompt ledger.
//
// The structured Report is extracted from the <ghx-report> block in the
// agent's output. If no report is found the turn still succeeds but the
// returned report will have only an Answer field describing the failure.
func Ask(ctx context.Context, cfg Config, req AskRequest) (*Report, *TurnResult, error) {
	sessionsDir := cfg.SessionsDir
	if err := checkACPHandshake(ctx, cfg.AgentCmd, cfg.Cwd, cfg.Env, defaultHandshakeTimeout); err != nil {
		return nil, nil, err
	}

	// Ensure session exists on disk.
	if !IsInitialized(sessionsDir, req.Session) {
		scope := req.Scope
		if scope == "" {
			scope = req.Question
			if len(scope) > 80 {
				scope = scope[:80]
			}
		}
		if err := InitSession(sessionsDir, req.Session, req.Repo, scope); err != nil {
			return nil, nil, fmt.Errorf("init session: %w", err)
		}
	}

	meta, err := ReadMeta(sessionsDir, req.Session)
	if err != nil {
		return nil, nil, fmt.Errorf("read meta: %w", err)
	}
	ledger, err := LoadLedger(sessionsDir, req.Session)
	if err != nil {
		return nil, nil, fmt.Errorf("read ledger: %w", err)
	}

	prompt := BuildPrompt(Request{
		Session:         req.Session,
		Repo:            req.Repo,
		Question:        req.Question,
		Depth:           req.Depth,
		AllowedBackends: req.AllowedBackends,
	}, meta, ledger)

	acpSessionID := ""
	if meta != nil {
		acpSessionID = meta.ACPSessionID
	}

	// Build session-level steering meta (ADR-0020.1 D2). Only attached to
	// new sessions (first turn or when the adapter does not support LoadSession).
	// Resume turns re-use the existing session, which already has the meta.
	depth := req.Depth
	if depth == "" {
		depth = "normal"
	}
	sessionMeta := BuildSessionMeta(
		BuildPersonaSystemPrompt(),
		depth,
		cfg.Model,
		cfg.EvalMode,
	)

	// Runtime-owned sink for the strict submit_report path (ADR-0021 D1). When
	// the executable/temp dir is available, the report-sink MCP server is
	// registered on the session and its accepted report is preferred over any
	// text block. On failure sinkPath is "" and the loop degrades to the
	// <ghx-report> text path.
	sinkPath, cleanupSink := newReportSinkPath()
	defer cleanupSink()

	turnResult, newSessionID, err := runTurnWithOptions(ctx, RunTurnOptions{
		AgentCmd:       cfg.AgentCmd,
		ACPSessionID:   acpSessionID,
		Prompt:         prompt,
		Cwd:            cfg.Cwd,
		Env:            cfg.Env,
		SessionMeta:    sessionMeta,
		ReportSinkPath: sinkPath,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("run turn: %w", err)
	}

	// Completion gate (ADR-0021 D2): prefer the strictly-validated sink report,
	// else the lenient text block. If neither yields a report, send a corrective
	// follow-up carrying the CONCRETE reason, bounded at maxReportRetries. The
	// exploration is done and paid for; a missing report should cost a bounded
	// number of nudges, not the whole turn's evidence.
	// Evaluate the report against the most recent turn's own text (plus the
	// cumulative sink), not the concatenated FullText: a stale invalid block
	// from an earlier attempt must not shadow a corrected block emitted by a
	// retry (the <ghx-report> regex takes the first block it finds).
	latestText := turnResult.FullText
	report, coerced, reason := resolveTurnReport(sinkPath, latestText)
	for retries := 0; report == nil && retries < maxReportRetries; retries++ {
		retrySessionID := newSessionID
		if retrySessionID == "" {
			retrySessionID = acpSessionID
		}
		if retrySessionID == "" {
			break
		}
		retryResult, retryNewID, retryErr := runTurnWithOptions(ctx, RunTurnOptions{
			AgentCmd:       cfg.AgentCmd,
			ACPSessionID:   retrySessionID,
			Prompt:         reportRetryPromptWithError(reason),
			Cwd:            cfg.Cwd,
			Env:            cfg.Env,
			ReportSinkPath: sinkPath,
		})
		if retryErr != nil {
			break
		}
		mergeRetryTurn(&turnResult, retryResult)
		latestText = retryResult.FullText
		if retryNewID != "" {
			newSessionID = retryNewID
		}
		report, coerced, reason = resolveTurnReport(sinkPath, latestText)
	}
	if report != nil {
		turnResult.ReportCoerced = coerced
	} else {
		report = &Report{
			Answer: "WARN: sidecar did not emit a <ghx-report> block — raw output was produced but no structured report was found.",
		}
	}

	// Persist the ACP session ID so the next turn can resume. Runs after
	// the retry block, which may advance the session ID.
	if meta != nil && newSessionID != "" && meta.ACPSessionID != newSessionID {
		meta.ACPSessionID = newSessionID
		if saveErr := SaveMeta(sessionsDir, *meta); saveErr != nil {
			// Non-fatal: worst case is the next turn starts a fresh ACP session.
			fmt.Fprintf(os.Stderr, "warning: failed to save ACP session ID: %v\n", saveErr)
		}
	}

	turn := 1
	if meta != nil {
		turn = meta.TurnCount + 1
	}
	UpdateLedgerFromTurn(ledger, meta, report, turnResult.ToolTraces, turn)
	if saveErr := SaveLedger(sessionsDir, req.Session, ledger); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save ledger: %v\n", saveErr)
	}

	if err := RecordTurn(sessionsDir, req.Session); err != nil {
		// Non-fatal: metadata is informational.
		fmt.Fprintf(os.Stderr, "warning: failed to record turn: %v\n", err)
	}

	if _, saveErr := SaveReport(sessionsDir, req.Session, report); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save report: %v\n", saveErr)
	}

	return report, &turnResult, nil
}
