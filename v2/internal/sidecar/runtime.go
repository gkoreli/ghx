package sidecar

import (
	"context"
	"fmt"
	"os"
)

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

// Ask executes one sidecar investigation turn and returns the evidence report.
//
// Session lifecycle:
//   - If the named session has never been used, InitSession is called and
//     an ACP NewSession is created.
//   - If the session exists and has an ACP session ID, LoadSession is called
//     so the agent retains its prior context.
//
// The structured Report is extracted from the <ghx-report> block in the
// agent's output. If no report is found the turn still succeeds but the
// returned report will have only an Answer field describing the failure.
func Ask(ctx context.Context, cfg Config, req AskRequest) (*Report, error) {
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
			return nil, fmt.Errorf("init session: %w", err)
		}
	}

	meta, err := ReadMeta(sessionsDir, req.Session)
	if err != nil {
		return nil, fmt.Errorf("read meta: %w", err)
	}

	prompt := BuildPrompt(Request{
		Session:         req.Session,
		Repo:            req.Repo,
		Question:        req.Question,
		Depth:           req.Depth,
		AllowedBackends: req.AllowedBackends,
	}, meta)

	acpSessionID := ""
	if meta != nil {
		acpSessionID = meta.ACPSessionID
	}

	turnResult, newSessionID, err := RunTurn(ctx, cfg.AgentCmd, acpSessionID, prompt)
	if err != nil {
		return nil, fmt.Errorf("run turn: %w", err)
	}

	// Persist the ACP session ID so the next turn can resume.
	if meta != nil && newSessionID != "" && meta.ACPSessionID != newSessionID {
		meta.ACPSessionID = newSessionID
		if saveErr := SaveMeta(sessionsDir, *meta); saveErr != nil {
			// Non-fatal: worst case is the next turn starts a fresh ACP session.
			fmt.Fprintf(os.Stderr, "warning: failed to save ACP session ID: %v\n", saveErr)
		}
	}

	if err := RecordTurn(sessionsDir, req.Session); err != nil {
		// Non-fatal: metadata is informational.
		fmt.Fprintf(os.Stderr, "warning: failed to record turn: %v\n", err)
	}

	report := ExtractReport(turnResult.FullText)
	if report == nil {
		report = &Report{
			Answer: "WARN: sidecar did not emit a <ghx-report> block — raw output was produced but no structured report was found.",
		}
	}

	if _, saveErr := SaveReport(sessionsDir, req.Session, report); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save report: %v\n", saveErr)
	}

	return report, nil
}
