package sidecar

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var runTurnWithOptions = RunTurnWithOptions
var checkACPHandshake = CheckACPHandshake

// TurnRunner executes one ACP prompt turn. The daemon injects a warm worker
// implementation; daemonless calls use RunTurnWithOptions.
type TurnRunner func(context.Context, RunTurnOptions) (TurnResult, string, error)

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
	mergeRawSDK(dst, src)
}

// mergeRawSDK folds a follow-up attempt's raw-SDK audit into the primary
// turn (ADR-0016.10 D2) — the raw twin of appending ToolTraces above.
func mergeRawSDK(dst *TurnResult, src TurnResult) {
	if src.RawSDK == nil {
		return
	}
	if dst.RawSDK == nil {
		dst.RawSDK = &RawSDKAudit{}
	}
	dst.RawSDK.Merge(src.RawSDK)
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

// resolveSessionWorkspace resolves the ACP session working directory for a turn
// (ADR-0033 D1). Precedence: an explicit Config.Cwd override (evals pin a
// checkout) > the persisted SessionMeta.Cwd (resume determinism) > the neutral,
// ghx-owned session directory (SessionWorkspace). The neutral default is not a
// git repo and holds no host .claude settings, so the spawned agent adopts no
// foreign, nondeterministic workspace regardless of where ghx was invoked.
func resolveSessionWorkspace(cfg Config, sessionsDir, session string, meta *SessionMeta) string {
	if cfg.Cwd != "" {
		return cfg.Cwd
	}
	if meta != nil && meta.Cwd != "" {
		return meta.Cwd
	}
	return SessionWorkspace(sessionsDir, session)
}

// recordAgentProvenance persists the resolved workspace, the agent command, the
// creating process's spawn cwd, and the agent-relevant env-var NAMES into the
// session metadata (ADR-0033 D5) so an environment-specific failure is
// diagnosable from committed artifacts. It writes only when something changed,
// so it is cheap on follow-up turns, and backfills legacy sessions on their
// next turn. Non-fatal — provenance is diagnostic, never load-bearing for the
// turn. spawnCwd is captured once, when the fields are first written, so it
// records the environment that actually created the (possibly warm, daemon-
// owned) session rather than every later caller's directory. spawnEnv is the
// resolved agent spawn environment (nil means this process's env); only the
// allowlisted NAMES from it are ever persisted, and they refresh on every
// turn where the fingerprint changes — with client auth passthrough
// (ADR-0033.1) the last ask's view is the diagnostic truth.
func recordAgentProvenance(sessionsDir string, meta *SessionMeta, workspace, agentCmd string, spawnEnv []string) {
	if meta == nil {
		return
	}
	changed := false
	if meta.Cwd != workspace {
		meta.Cwd = workspace
		changed = true
	}
	if meta.AgentCmd != agentCmd {
		meta.AgentCmd = agentCmd
		changed = true
	}
	if meta.SpawnCwd == "" {
		if wd, err := os.Getwd(); err == nil {
			meta.SpawnCwd = wd
			changed = true
		}
	}
	if present := PresentAgentEnv(spawnEnv); !slices.Equal(meta.AgentEnv, present) {
		meta.AgentEnv = present
		changed = true
	}
	if !changed {
		return
	}
	if saveErr := SaveMeta(sessionsDir, *meta); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save agent provenance: %v\n", saveErr)
	}
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
	mergeRawSDK(dst, src)
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
	// AgentAuthEnv carries "NAME=VALUE" auth/transport env entries captured
	// from the ASKING process's shell (CaptureAgentAuthEnv allowlist) and
	// overlaid onto the agent spawn environment (ADR-0033.1). The daemon's
	// own env is frozen at first auto-start, so without this a Bedrock/
	// gateway shell silently loses its credentials on the daemon path.
	// SECRET-BEARING: rides only the local daemon socket; never persist or
	// log it — session metadata records env NAMES only. Ignored when
	// Config.Env is set (eval profiles pin the whole environment).
	AgentAuthEnv []string
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
	return askWithTurnRunner(ctx, cfg, req, runTurnWithOptions, true, nil)
}

// AskWithTurnRunner executes one Ask using a caller-owned turn runner. It is
// used by the daemon worker pool so durable sidecar behavior stays centralized
// while ACP process ownership moves out of the one-shot path. route carries
// the daemon's pre-computed routing decision (the daemon must route before it
// can pick a per-session warm worker); nil means route here.
func AskWithTurnRunner(ctx context.Context, cfg Config, req AskRequest, runner TurnRunner, route *RouteDecision) (*Report, *TurnResult, error) {
	return askWithTurnRunner(ctx, cfg, req, runner, false, route)
}

func askWithTurnRunner(ctx context.Context, cfg Config, req AskRequest, turnRunner TurnRunner, preflight bool, route *RouteDecision) (*Report, *TurnResult, error) {
	if turnRunner == nil {
		turnRunner = runTurnWithOptions
	}
	// The claude-acp Runner (ADR-0036 D1) is the seam: Preflight is the ACP
	// handshake, Open/Turn wrap the injected TurnRunner (RunTurnWithOptions
	// one-shot, or a warm worker on the daemon path). All ACP knowledge stays
	// behind it; the runtime switches on the typed Outcome.
	portRunner := &claudeACPRunner{cfg: cfg, turnRun: turnRunner}
	if preflight {
		if err := portRunner.Preflight(ctx); err != nil {
			return nil, nil, err
		}
	}
	state, err := prepareSession(cfg, req, route)
	if err != nil {
		return nil, nil, err
	}
	defer state.cleanupSink()
	portRunner.workspace = state.workspace
	portRunner.spawnEnv = state.spawnEnv
	portRunner.stderrLog = state.stderrLog
	portRunner.livePath = state.livePath

	// The runtime owns the streamed side effects (ADR-0036 D1): one Ask-scoped
	// EventSink over the session's live.jsonl, threaded into every turn (primary,
	// wrap-up, retries), so the adapter never touches stdout/live directly.
	contentLive := NewLiveLog(state.livePath)
	defer contentLive.Close()
	sink := &streamEventSink{live: contentLive}

	// completedErr is the RAW turn error the live turn-log records — NOT the
	// DiagnoseTurnError-enriched string returned to the caller. It is a distinct
	// local (not a named return), which restores the pre-T1.3 byte-identical
	// live-log failure field (M1; docs/audits/refactor-review-2026-07-07.md).
	var completedErr error
	liveLog := NewLiveLog(state.livePath)
	defer func() {
		errMsg := ""
		if completedErr != nil {
			errMsg = completedErr.Error()
		}
		liveLog.TurnCompleted(completedErr == nil, errMsg, len(state.turnResult.ToolCalls), len(state.turnResult.FullText))
		liveLog.Close()
	}()

	liveLog.TurnStarted(state.req.Session, state.req.Repo, state.req.Question)
	session, outcome := runPrimaryTurn(ctx, portRunner, state, sink)
	blockedReport, maxTurnsHandled := recoverMaxTurns(ctx, session, state, outcome, sink)
	turnErr := outcome.Err
	if maxTurnsHandled {
		turnErr = nil
	}

	if turnErr != nil {
		// Live-log records the RAW turn error (M1); artifacts + the caller error
		// carry the runtime-owned canonical FailureClass marker (ADR-0036 D4).
		// For claude-acp the adapter already surfaced the marker, so failErr is
		// byte-identical to the raw error.
		completedErr = outcome.Err
		failErr := canonicalTurnError(outcome)
		emitFailedTurnArtifacts(ctx, state, failErr)
		return nil, &state.turnResult, DiagnoseTurnError(fmt.Errorf("run turn: %w", failErr), state.stderrLog)
	}

	report := resolveReportWithRetry(ctx, session, state, blockedReport, sink)
	tierDecision := persistTurn(state, report)
	emitArtifacts(ctx, state, report, tierDecision)

	return report, &state.turnResult, nil
}

type askTurnState struct {
	cfg          Config
	req          AskRequest
	route        *RouteDecision
	sessionsDir  string
	meta         *SessionMeta
	ledger       *Ledger
	workspace    string
	stderrLog    string
	livePath     string
	spawnEnv     []string
	prompt       string
	acpSessionID string
	// spec/budget are the neutral steering + budget the Runner port carries
	// (ADR-0036 D1); the claude-acp adapter encodes spec into the _meta bag.
	spec         SteeringSpec
	budget       TurnBudget
	sinkPath     string
	cleanupSink  func()
	startedAt    time.Time
	turnResult   TurnResult
	newSessionID string
}

// resumeID is the ACP session id to resume for a follow-up turn: the id the last
// turn established (newSessionID), falling back to the persisted id — the exact
// fallback the claude-acp Session's auto-managed resume mirrors.
func (state *askTurnState) resumeID() string {
	if state.newSessionID != "" {
		return state.newSessionID
	}
	return state.acpSessionID
}

// turnRequest builds the neutral TurnRequest for a prompt on this Ask: the same
// steering + budget for every turn (primary, wrap-up, retries).
func (state *askTurnState) turnRequest(prompt string) TurnRequest {
	return TurnRequest{Prompt: prompt, Steering: state.spec, Budget: state.budget}
}

func prepareSession(cfg Config, req AskRequest, route *RouteDecision) (*askTurnState, error) {
	sessionsDir := cfg.SessionsDir
	if route == nil {
		d := RouteQuestion(sessionsDir, req, RouteConfigFor(cfg))
		route = &d
	}
	req.Session = route.Session
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return nil, err
	}
	if !IsInitialized(sessionsDir, req.Session) {
		scope := req.Scope
		if scope == "" {
			scope = req.Question
			if len(scope) > 80 {
				scope = scope[:80]
			}
		}
		repo := req.Repo
		if repo == "" {
			repo = route.DetectedRepo
		}
		if err := InitSession(sessionsDir, req.Session, repo, scope, route.sessionNamedBy()); err != nil {
			return nil, fmt.Errorf("init session: %w", err)
		}
	}
	meta, err := ReadMeta(sessionsDir, req.Session)
	if err != nil {
		return nil, fmt.Errorf("read meta: %w", err)
	}
	workspace := resolveSessionWorkspace(cfg, sessionsDir, req.Session, meta)
	stderrLog := AgentStderrLogPath(sessionsDir, req.Session)
	livePath := LiveLogPath(sessionsDir, req.Session)
	spawnEnv := cfg.Env
	if spawnEnv == nil {
		spawnEnv = MergeAgentEnv(nil, req.AgentAuthEnv)
	}
	// Tier-2 grant propagation (ADR-0024.4 D2): the ask's AllowedBackends
	// decision rides into the agent's shell environment so the `ghx tier2`
	// CLI can enforce the same grant pre-hoc on both ask paths (daemonless
	// and daemon share this one code path via prepareSession).
	spawnEnv = append(spawnEnv, Tier2GrantEnv+"="+Tier2GrantValue(req.AllowedBackends))
	recordAgentProvenance(sessionsDir, meta, workspace, cfg.AgentCmd, spawnEnv)
	ledger, err := LoadLedger(sessionsDir, req.Session)
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
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
	sinkPath, cleanupSink := newReportSinkPath()
	spec, budget := buildSteeringSpec(cfg, req, sinkPath)
	return &askTurnState{
		cfg:          cfg,
		req:          req,
		route:        route,
		sessionsDir:  sessionsDir,
		meta:         meta,
		ledger:       ledger,
		workspace:    workspace,
		stderrLog:    stderrLog,
		livePath:     livePath,
		spawnEnv:     spawnEnv,
		prompt:       prompt,
		acpSessionID: acpSessionID,
		spec:         spec,
		budget:       budget,
		sinkPath:     sinkPath,
		cleanupSink:  cleanupSink,
		startedAt:    time.Now().UTC(),
	}, nil
}

// buildSteeringSpec maps the Ask config/request into the neutral steering
// surface + turn budget the Runner port carries (ADR-0036 D1). Persona selection
// (repo-scoped vs discovery) and the depth->budget resolution stay in the
// runtime; the claude-acp adapter (steeringToSessionMeta) re-encodes it into the
// _meta.claudeCode.options bag. The resolved Effort/Thinking/MaxTurns come from
// the SAME depthBudgets table BuildSessionMeta uses, so the encoded bag is
// byte-identical to the former buildTurnSessionMeta output.
func buildSteeringSpec(cfg Config, req AskRequest, sinkPath string) (SteeringSpec, TurnBudget) {
	persona := BuildPersonaSystemPrompt()
	if req.Repo == "" {
		persona = BuildDiscoveryPersonaSystemPrompt()
	}
	depth := req.Depth
	if depth == "" {
		depth = "normal"
	}
	parsedDepth, ok := ParseDepth(depth)
	if !ok {
		parsedDepth = DepthNormal
	}
	b := depthBudgets[parsedDepth]
	spec := SteeringSpec{
		SystemPrompt:   persona,
		Isolation:      Isolation{SettingSources: cfg.AgentSettingSources},
		ToolPolicy:     reconToolPolicy,
		Model:          cfg.Model,
		Effort:         b.effort,
		Thinking:       thinkingBudgetFromPtr(b.thinking),
		ReportSink:     ReportSink{Path: sinkPath, ToolID: SubmitReportToolID},
		AuditRawStream: cfg.EvalMode,
	}
	// The Ask path leaves Liveness at 0 (resolve from env/default), exactly as
	// the former buildTurnOptions left RunTurnOptions.LivenessTimeout unset.
	return spec, TurnBudget{MaxTurns: b.maxTurns}
}

// runPrimaryTurn runs the first turn behind the port, applying the stale-session
// fallback (ADR-0027): if a persisted resume is stale, open exactly one fresh
// session and continue the same prompt — the durable ledger already rides in the
// prompt, so ACP resume is only an optimization. It returns the (possibly
// re-opened) Session and the typed Outcome for the runtime to switch on.
func runPrimaryTurn(ctx context.Context, portRunner *claudeACPRunner, state *askTurnState, sink EventSink) (Session, Outcome) {
	session, _ := portRunner.Open(ctx, SessionID(state.req.Session), ResumeToken(state.acpSessionID))
	req := state.turnRequest(state.prompt)
	result, resume, outcome := session.Turn(ctx, req, sink)
	state.turnResult = result
	state.newSessionID = string(resume)
	if state.acpSessionID != "" && outcome.Class == StaleSession {
		session, _ = portRunner.Open(ctx, SessionID(state.req.Session), "")
		result, resume, outcome = session.Turn(ctx, req, sink)
		result.SessionRecreated = true
		state.turnResult = result
		state.newSessionID = string(resume)
	}
	return session, outcome
}

// recoverMaxTurns implements the ADR-0027 D1 turn-cap recovery, now switching on
// the typed Outcome instead of string-matching. When the runtime's max-turns net
// fired it issues exactly one wrap-up prompt on the same session; on success the
// exploration ships (WrapUpRecovered), otherwise a BLOCKED report records both
// failures. Any other class returns (nil, false) and the caller handles it.
func recoverMaxTurns(ctx context.Context, session Session, state *askTurnState, outcome Outcome, sink EventSink) (*Report, bool) {
	if outcome.Class != TurnCapReached {
		return nil, false
	}
	turnCapErr := outcome.Err
	var wrapUpErr error
	if state.resumeID() == "" {
		wrapUpErr = fmt.Errorf("no ACP session id available to resume")
	} else {
		wrapResult, wrapResume, wrapOutcome := session.Turn(ctx, state.turnRequest(turnCapWrapUpPrompt), sink)
		mergeWrapUpTurn(&state.turnResult, wrapResult)
		if wrapOutcome.Err != nil {
			wrapUpErr = wrapOutcome.Err
		} else {
			state.turnResult.WrapUpRecovered = true
			if string(wrapResume) != "" {
				state.newSessionID = string(wrapResume)
			}
			return nil, true
		}
	}
	return &Report{
		Answer: "BLOCKED: the exploration hit the adapter's max-turns safety net and the one-shot wrap-up attempt also failed; partial artifacts were kept in the session directory.",
		Uncertainty: []string{
			"turn-cap error: " + turnCapErr.Error(),
			"wrap-up failure: " + wrapUpErr.Error(),
		},
	}, true
}

func resolveReportWithRetry(ctx context.Context, session Session, state *askTurnState, blockedReport *Report, sink EventSink) *Report {
	latestText := state.turnResult.FullText
	report, coerced, reason := resolveTurnReport(state.sinkPath, latestText)
	if report == nil && blockedReport != nil {
		report = blockedReport
	}
	for retries := 0; report == nil && retries < maxReportRetries; retries++ {
		if state.resumeID() == "" {
			break
		}
		retryResult, retryResume, retryOutcome := session.Turn(ctx, state.turnRequest(reportRetryPromptWithError(reason)), sink)
		if retryOutcome.Err != nil {
			break
		}
		mergeRetryTurn(&state.turnResult, retryResult)
		latestText = retryResult.FullText
		if string(retryResume) != "" {
			state.newSessionID = string(retryResume)
		}
		report, coerced, reason = resolveTurnReport(state.sinkPath, latestText)
	}
	if report != nil {
		state.turnResult.ReportCoerced = coerced
		return report
	}
	return &Report{Answer: warnNoReportAnswer(state.stderrLog)}
}

func persistTurn(state *askTurnState, report *Report) *TierDecisionRecord {
	persistACPSessionID(state.sessionsDir, state.meta, state.newSessionID)
	turn := state.turnNumber()
	tierDecision := recordTierDecision(state.sessionsDir, state.req, turn, &state.turnResult, report)
	UpdateLedgerFromTurn(state.ledger, state.meta, report, state.turnResult.ToolTraces, turn)
	if saveErr := SaveLedger(state.sessionsDir, state.req.Session, state.ledger); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save ledger: %v\n", saveErr)
	}
	if err := RecordTurn(state.sessionsDir, state.req.Session); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to record turn: %v\n", err)
	}
	if _, saveErr := SaveTurnReportArtifact(state.sessionsDir, state.req.Session, turn, ReportArtifact{
		Report:              report,
		ActualCommandLedger: actualCommandLedger(state.turnResult.ToolCalls),
		TraceCommands:       TraceCommandLedger(state.turnResult.ToolTraces),
	}); saveErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to save report: %v\n", saveErr)
	}
	return tierDecision
}

func emitArtifacts(ctx context.Context, state *askTurnState, report *Report, tierDecision *TierDecisionRecord) {
	turn := state.turnNumber()
	state.turnResult.Route = state.route
	state.turnResult.Artifacts = emitTurnArtifacts(ctx, turnTelemetry{
		SessionsDir:    state.sessionsDir,
		Session:        state.req.Session,
		Repo:           state.req.Repo,
		Model:          state.cfg.Model,
		Turn:           turn,
		Question:       state.req.Question,
		Result:         state.turnResult,
		Route:          state.route,
		Report:         report,
		TierDecision:   tierDecision,
		StartedAt:      state.startedAt,
		EndedAt:        time.Now().UTC(),
		CaptureContent: state.cfg.CaptureContent(),
	})
}

func emitFailedTurnArtifacts(ctx context.Context, state *askTurnState, turnErr error) {
	persistACPSessionID(state.sessionsDir, state.meta, state.newSessionID)
	turn := state.turnNumber()
	tierDecision := recordTierDecision(state.sessionsDir, state.req, turn, &state.turnResult, nil)
	state.turnResult.Route = state.route
	state.turnResult.Artifacts = emitTurnArtifacts(ctx, turnTelemetry{
		SessionsDir:    state.sessionsDir,
		Session:        state.req.Session,
		Repo:           state.req.Repo,
		Model:          state.cfg.Model,
		Turn:           turn,
		Question:       state.req.Question,
		Result:         state.turnResult,
		TierDecision:   tierDecision,
		Route:          state.route,
		Error:          turnErr.Error(),
		StartedAt:      state.startedAt,
		EndedAt:        time.Now().UTC(),
		CaptureContent: state.cfg.CaptureContent(),
	})
}

func (state *askTurnState) turnNumber() int {
	turn := 1
	if state.meta != nil {
		turn = state.meta.TurnCount + 1
	}
	return turn
}

// Slug lowercases s and collapses every non-alphanumeric run into one dash.
func Slug(s, fallback string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return fallback
	}
	return out
}

// QuestionSlug derives a stable discovery session name from the question.
func QuestionSlug(question string) string {
	slug := Slug(question, "discovery")
	if runes := []rune(slug); len(runes) > 40 {
		slug = string(runes[:40])
		if i := strings.LastIndex(slug, "-"); i > 0 {
			slug = slug[:i]
		}
	}
	sum := sha256.Sum256([]byte(question))
	return slug + "-" + hex.EncodeToString(sum[:4])
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
