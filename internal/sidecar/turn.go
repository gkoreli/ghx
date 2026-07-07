package sidecar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

type acpSessionState struct {
	sessionID   string
	loadSession bool
}

func startACPWatchdog(ctx context.Context, lastActivity *atomic.Int64, cancel context.CancelCauseFunc, liveness time.Duration) {
	go func() {
		for {
			idle := time.Duration(time.Now().UnixNano() - lastActivity.Load())
			remaining := liveness - idle
			if remaining <= 0 {
				cancel(fmt.Errorf("%w: no ACP session update for %s (set %s to adjust)",
					ErrLivenessTimeout, liveness, LivenessTimeoutEnv))
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(remaining):
			}
		}
	}()
}

func configureACPSession(ctx context.Context, turnCtx context.Context, conn *acp.ClientSideConnection, state *acpSessionState, opts RunTurnOptions, cwd string) (string, error) {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	if state.sessionID == "" {
		if opts.ACPSessionID != "" && state.loadSession {
			_, err := conn.LoadSession(ctx, acp.LoadSessionRequest{
				SessionId:  acp.SessionId(opts.ACPSessionID),
				Cwd:        cwd,
				McpServers: reportSinkMcpServers(opts.ReportSinkPath),
				Meta:       opts.SessionMeta,
			})
			if err != nil {
				return "", fmt.Errorf("acp load session: %w", turnFailureCause(turnCtx, err))
			}
			state.sessionID = opts.ACPSessionID
			return state.sessionID, nil
		}
		req := acp.NewSessionRequest{
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
		}
		if opts.SessionMeta != nil {
			req.Meta = opts.SessionMeta
		}
		resp, err := conn.NewSession(ctx, req)
		if err != nil {
			return "", fmt.Errorf("acp new session: %w", turnFailureCause(turnCtx, err))
		}
		state.sessionID = string(resp.SessionId)
		return state.sessionID, nil
	}
	if state.loadSession {
		_, err := conn.LoadSession(ctx, acp.LoadSessionRequest{
			SessionId:  acp.SessionId(state.sessionID),
			Cwd:        cwd,
			McpServers: reportSinkMcpServers(opts.ReportSinkPath),
			Meta:       opts.SessionMeta,
		})
		if err != nil {
			return state.sessionID, fmt.Errorf("acp load session: %w", turnFailureCause(turnCtx, err))
		}
	}
	return state.sessionID, nil
}

func runACPPrompt(ctx context.Context, turnCtx context.Context, conn *acp.ClientSideConnection, sessionID acp.SessionId, prompt string) error {
	return waitForACPPrompt(runPrompt(ctx, conn, sessionID, prompt), conn.Done(), ctx.Done(), ctx.Err, turnCtx.Done(), turnCtx)
}

func runPrompt(ctx context.Context, conn *acp.ClientSideConnection, sessionID acp.SessionId, prompt string) <-chan error {
	promptDone := make(chan error, 1)
	go func() {
		_, perr := conn.Prompt(ctx, acp.PromptRequest{
			SessionId: sessionID,
			Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
		})
		promptDone <- perr
	}()
	return promptDone
}

func waitForACPPrompt(promptDone <-chan error, connDone <-chan struct{}, ctxDone <-chan struct{}, ctxErr func() error, turnDone <-chan struct{}, turnCtx context.Context) error {
	var promptErr error
	select {
	case promptErr = <-promptDone:
	case <-connDone:
		perr, finished := awaitPromptOrGrace(promptDone)
		if finished {
			promptErr = perr
		} else {
			promptErr = errors.New(peerClosedMarker + " before the prompt turn completed")
		}
		if promptErr != nil && !IsPeerClosedError(promptErr) {
			promptErr = fmt.Errorf("%s: %w", peerClosedMarker, promptErr)
		}
	case <-ctxDone:
		if perr, finished := awaitPromptOrGrace(promptDone); finished {
			promptErr = perr
		} else {
			if ctxErr != nil {
				promptErr = ctxErr()
			}
		}
	case <-turnDone:
		if perr, finished := awaitPromptOrGrace(promptDone); finished {
			promptErr = perr
		} else {
			promptErr = context.Cause(turnCtx)
		}
	}
	if cause := context.Cause(turnCtx); errors.Is(cause, ErrLivenessTimeout) {
		promptErr = cause
	}
	return promptErr
}
