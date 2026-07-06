package sidecar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// turnCapWrapUpPrompt is the exact one-shot recovery prompt sent on a resumed
// ACP session after the adapter's max-turns safety net fires (ADR-0027 D1). A
// fresh query gets a fresh turn budget; the session's context (ledger, prior
// tool results) survives, so the exploration is salvaged instead of destroyed.
const turnCapWrapUpPrompt = "wrap up: call submit_report now with what you have; mark unverified items unverified"

// mergeWrapUpTurn folds the wrap-up recovery turn's telemetry into the primary
// turn result (ADR-0027 D1). Unlike mergeRetryTurn it does not mark the turn
// as report-retried: the wrap-up is budget recovery, not a report correction.
func mergeWrapUpTurn(dst *TurnResult, src TurnResult) {
	dst.FullText += src.FullText
	dst.Thinking += src.Thinking
	dst.ToolCalls = append(dst.ToolCalls, src.ToolCalls...)
	dst.ToolTraces = append(dst.ToolTraces, src.ToolTraces...)
	dst.ToolOutputChars += src.ToolOutputChars
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

// persistACPSessionID saves the ACP session ID for future resumption. It runs
// on failure paths too (ADR-0027 D1/D3): a failed turn's session is exactly
// the one a later wrap-up or follow-up ask needs to resume. Non-fatal — the
// worst case is that the next turn starts a fresh ACP session.
func persistACPSessionID(sessionsDir string, meta *SessionMeta, newSessionID string) {
	if meta == nil || newSessionID == "" || meta.ACPSessionID == newSessionID {
		return
	}
	meta.ACPSessionID = newSessionID
	if saveErr := SaveMeta(sessionsDir, *meta); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save ACP session ID: %v\n", saveErr)
	}
}

// runTurnWithStaleSessionFallback runs the first turn for Ask. If a persisted
// ACP session ID is stale (LoadSession Resource not found), it creates exactly
// one fresh ACP session and continues the same prompt; the durable ledger
// context already rides in the prompt, so ACP resume is only an optimization.
func runTurnWithStaleSessionFallback(ctx context.Context, opts RunTurnOptions) (TurnResult, string, error) {
	result, newSessionID, err := runTurnWithOptions(ctx, opts)
	if opts.ACPSessionID == "" || err == nil || !IsLoadSessionResourceNotFound(err) {
		return result, newSessionID, err
	}
	freshOpts := opts
	freshOpts.ACPSessionID = ""
	freshResult, freshSessionID, freshErr := runTurnWithOptions(ctx, freshOpts)
	freshResult.SessionRecreated = true
	if freshErr != nil {
		return freshResult, freshSessionID, freshErr
	}
	return freshResult, freshSessionID, nil
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
	// Empty means discovery mode: no repo scope, cross-GitHub sweep
	// (ADR-0019.1 D1).
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
	// Persona selection (ADR-0019.1 D4): a repo-scoped ask keeps the exact
	// existing persona; an ask without repo scope gets the discovery persona
	// (same doctrine plus the discovery-mode section).
	persona := BuildPersonaSystemPrompt()
	if req.Repo == "" {
		persona = BuildDiscoveryPersonaSystemPrompt()
	}
	sessionMeta := BuildSessionMeta(
		persona,
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

	// Wall-clock window for the whole Ask (including any corrective retries),
	// used for the production duration metric (ADR-0022 D2). This is coarser
	// than the eval path's per-turn ACP timing — see emit.go / the ADR gap note.
	startedAt := time.Now().UTC()

	turnResult, newSessionID, err := runTurnWithStaleSessionFallback(ctx, RunTurnOptions{
		AgentCmd:       cfg.AgentCmd,
		ACPSessionID:   acpSessionID,
		Prompt:         prompt,
		Cwd:            cfg.Cwd,
		Env:            cfg.Env,
		SessionMeta:    sessionMeta,
		ReportSinkPath: sinkPath,
	})

	// Resume-as-recovery (ADR-0027 D1): the adapter's max-turns error is a
	// session-resume problem, not error handling. LoadSession-resume the same
	// ACP session and send exactly one wrap-up prompt; a fresh query gets a
	// fresh turn budget while the exploration's context survives. If the
	// wrap-up also fails, the turn is BLOCKED — with artifacts (D3).
	var blockedReport *Report
	if err != nil && IsMaxTurnsError(err) {
		turnCapErr := err
		resumeID := newSessionID
		if resumeID == "" {
			resumeID = acpSessionID
		}
		var wrapUpErr error
		if resumeID == "" {
			wrapUpErr = fmt.Errorf("no ACP session id available to resume")
		} else {
			wrapResult, wrapSessionID, werr := runTurnWithOptions(ctx, RunTurnOptions{
				AgentCmd:       cfg.AgentCmd,
				ACPSessionID:   resumeID,
				Prompt:         turnCapWrapUpPrompt,
				Cwd:            cfg.Cwd,
				Env:            cfg.Env,
				ReportSinkPath: sinkPath,
			})
			// Fold the wrap-up's telemetry in on both outcomes: even a failed
			// wrap-up attempt is part of this turn's auditable activity (D3).
			mergeWrapUpTurn(&turnResult, wrapResult)
			if werr != nil {
				wrapUpErr = werr
			} else {
				turnResult.WrapUpRecovered = true
				if wrapSessionID != "" {
					newSessionID = wrapSessionID
				}
				err = nil
			}
		}
		if wrapUpErr != nil {
			blockedReport = &Report{
				Answer: "BLOCKED: the exploration hit the adapter's max-turns safety net and the one-shot wrap-up attempt also failed; partial artifacts were kept in the session directory.",
				Uncertainty: []string{
					"turn-cap error: " + turnCapErr.Error(),
					"wrap-up failure: " + wrapUpErr.Error(),
				},
			}
			err = nil
		}
	}

	if err != nil {
		// Unrecovered turn failure (liveness watchdog, dead peer, transport
		// error). Never leave zero artifacts (ADR-0027 D3): persist the ACP
		// session ID for later resumption, flush the partial turn's
		// traces/logs/metrics with the error recorded, and hand the partial
		// TurnResult back so eval callers can audit the failed turn too.
		persistACPSessionID(sessionsDir, meta, newSessionID)
		turn := 1
		if meta != nil {
			turn = meta.TurnCount + 1
		}
		turnResult.Artifacts = emitTurnArtifacts(ctx, turnTelemetry{
			SessionsDir:    sessionsDir,
			Session:        req.Session,
			Repo:           req.Repo,
			Model:          cfg.Model,
			Turn:           turn,
			Question:       req.Question,
			Result:         turnResult,
			Error:          err.Error(),
			StartedAt:      startedAt,
			EndedAt:        time.Now().UTC(),
			CaptureContent: cfg.CaptureContent(),
		})
		return nil, &turnResult, fmt.Errorf("run turn: %w", err)
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
	// A failed wrap-up ships the BLOCKED report (ADR-0027 D1) unless the agent
	// managed to submit a real report before dying; no corrective retries are
	// spent on a session that already exhausted its budget twice.
	if report == nil && blockedReport != nil {
		report = blockedReport
	}
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
	persistACPSessionID(sessionsDir, meta, newSessionID)

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

	if _, saveErr := SaveTurnReportArtifact(sessionsDir, req.Session, turn, ReportArtifact{
		Report:              report,
		ActualCommandLedger: actualCommandLedger(turnResult.ToolCalls),
	}); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save report: %v\n", saveErr)
	}

	// Production visibility: append this turn's traces/logs/metrics to the
	// session artifact set via the shared telemetry runtime (ADR-0022 D2).
	// No reward/evaluation events — that is the eval layer. Failures degrade to
	// a stderr warning inside emitTurnArtifacts and never fail the ask. The
	// returned ArtifactsRef travels back on the TurnResult so every response
	// surface can point the caller at the audit trail.
	turnResult.Artifacts = emitTurnArtifacts(ctx, turnTelemetry{
		SessionsDir:    sessionsDir,
		Session:        req.Session,
		Repo:           req.Repo,
		Model:          cfg.Model,
		Turn:           turn,
		Question:       req.Question,
		Result:         turnResult,
		Report:         report,
		StartedAt:      startedAt,
		EndedAt:        time.Now().UTC(),
		CaptureContent: cfg.CaptureContent(),
	})

	return report, &turnResult, nil
}

func actualCommandLedger(toolCalls []string) []string {
	var out []string
	for _, call := range toolCalls {
		call = strings.TrimSpace(call)
		if call != "" {
			out = append(out, call)
		}
	}
	return out
}
