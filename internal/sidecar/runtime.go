package sidecar

import (
	"context"
	"fmt"
	"os"
)

var runTurnWithOptions = RunTurnWithOptions

// reportRetryPrompt is the one-shot corrective follow-up sent when a turn
// produced no extractable <ghx-report> block (ADR-0016.7 RC3).
const reportRetryPrompt = `Your previous reply did not include a valid <ghx-report> block.
Emit it now based on the work you already did: output ONLY the report JSON
inside <ghx-report></ghx-report> tags, following the schema from your
instructions exactly (all list fields must be JSON arrays). Do not run any
more commands. No text before or after the tags.`

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

	turnResult, newSessionID, err := runTurnWithOptions(ctx, RunTurnOptions{
		AgentCmd:     cfg.AgentCmd,
		ACPSessionID: acpSessionID,
		Prompt:       prompt,
		Cwd:          cfg.Cwd,
		Env:          cfg.Env,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("run turn: %w", err)
	}

	report := ExtractReport(turnResult.FullText)
	if report == nil {
		// One-shot corrective follow-up in the same ACP session before
		// giving up (ADR-0016.7 RC3): the exploration is done and paid
		// for; a missing report block should cost one nudge, not the
		// whole turn's evidence. The retry is recorded on the result so
		// eval accounting stays honest.
		retrySessionID := newSessionID
		if retrySessionID == "" {
			retrySessionID = acpSessionID
		}
		if retrySessionID != "" {
			retryResult, retryNewID, retryErr := runTurnWithOptions(ctx, RunTurnOptions{
				AgentCmd:     cfg.AgentCmd,
				ACPSessionID: retrySessionID,
				Prompt:       reportRetryPrompt,
				Cwd:          cfg.Cwd,
				Env:          cfg.Env,
			})
			if retryErr == nil {
				report = ExtractReport(retryResult.FullText)
				turnResult.ReportRetried = true
				turnResult.FullText += retryResult.FullText
				turnResult.ToolCalls = append(turnResult.ToolCalls, retryResult.ToolCalls...)
				turnResult.ToolTraces = append(turnResult.ToolTraces, retryResult.ToolTraces...)
				turnResult.ToolOutputChars += retryResult.ToolOutputChars
				if retryNewID != "" {
					newSessionID = retryNewID
				}
			}
		}
	}
	if report == nil {
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
