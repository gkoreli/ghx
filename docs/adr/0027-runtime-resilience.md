---
title: "ADR-0027: Sidecar Runtime Resilience — Never Lose an Exploration"
date: "2026-07-05"
status: "accepted"
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0027. Sidecar Runtime Resilience

## Status

Accepted 2026-07-05. Direction settled by founder during the dogfood
week: turn-cap exhaustion "is a SESSION RESUME problem, not error
handling" (FRICTION.md 2026-07-05), and reports without evidence must be
rejected in-band, no bandaids ("until the sidecar agent writes the report
in a proper json file it shouldn't be able to proceed"). Part of the
always-on-runtime capability (NORTH_STAR §4): explorations are never lost
to process, turn, or hang boundaries.

## Evidence

- **Breaking friction**: `Arize-ai/phoenix` ask died at the adapter's
  hard `Reached maximum number of turns (24)` after ~20+ tool calls — the
  entire exploration lost, no report, no partial
  (FRICTION.md "max-turns exhaustion destroys the exploration").
- **Breaking friction**: that failed session dir contains only
  `meta.json` — emission happens only after a successful turn, so exactly
  the turns that most need auditing are invisible (FRICTION.md "failed
  turns leave zero artifacts").
- **Zombie hang**: gate-run round 4 hung 67+ minutes after "peer
  connection closed"; the 15-minute context timeout never fired because
  the ACP client wait is not cancellable — the episode neither finished
  nor died.
- **Contract gap**: the 2026-07-05 confirmatory run contained
  answer-only reports (no verified claims, no evidence) that ADR-0021's
  schema validation accepted; the evidence contract (AGENTS.md) calls
  that a hypothesis, not a finding.

## Decisions

### D1. Resume-as-recovery on turn-cap exhaustion

maxTurns stays a hard safety net (~3× command budget; ADR-0020.1 notes),
but hitting it no longer destroys the exploration. On the adapter's
max-turns error, the runtime LoadSession-resumes the same ACP session and
sends one wrap-up prompt: submit_report now with what you have, marking
unverified items unverified. A fresh query gets a fresh turn budget; the
session's context (ledger, prior tool results) survives. Exactly one
wrap-up attempt; if it also fails, the turn is BLOCKED — with artifacts
(D3). The wrap-up is recorded on the turn record (`wrapUpRecovered:
true`) and as a soft anomaly `turn_cap_wrapup` so evals can count how
often the net fires. Applies identically in production `Ask` and eval
episodes — one runtime, one behavior (ADR-0022 principle).

### D2. Liveness watchdog

The ACP client's wait becomes cancellable (select on ctx.Done() — the
round-4 root cause is a non-cancellable wait that outlived a dead peer).
On top of that, a liveness watchdog cancels a turn when no session
update/notification has arrived for a configurable window (default 10
minutes, `GHX_SIDECAR_LIVENESS_TIMEOUT`); cancellation records the
`episode_hang_timeout` anomaly (evals) / an error entry in session logs
(production). A dead peer connection fails the turn immediately instead
of waiting for any timer. Eval run loops print per-episode liveness so a
human watching the run can see a stall as it happens.

### D3. Artifacts on every path, including failure

Telemetry/artifact emission moves from "after successful turn" to
incremental: spans/logs flush as they occur, and every error path
(turn-cap, watchdog, peer-closed, validation-rejected) flushes what
exists before returning. Acceptance test: a turn that dies mid-flight
still leaves traces.jsonl + logs in the session dir. This is the
visibility tenet at its most valuable moment — failed explorations are
the ones that must be auditable.

### D4. Evidence-required report validation (ADR-0021 tightening)

`submit_report` rejects, in-band with exact field-level errors, any
report that is not BLOCKED and lacks all three of: at least one verified
claim with evidence, at least one relevant file, at least one command
run. (A BLOCKED report states why it is blocked and skips the
requirement.) The agent sees the validation error as the tool result and
must fix the report to proceed — the same strict-schema loop ADR-0021
already runs for shape errors, now extended to the evidence contract.
Persona text is updated to state the requirement up front so rejection
is the backstop, not the teacher.

## Considered and rejected

- **Raising maxTurns until errors disappear** — unbounded cost, and the
  net would still fire eventually; recovery must exist regardless.
- **Treating turn-cap as a normal error with retry-from-scratch** —
  throws away the exploration the user paid for; resume preserves it.
- **Heartbeat pings to the agent** — adds protocol chatter; passive
  liveness on session updates plus a cancellable wait covers the observed
  failure.
- **Evidence validation as a lint warning** — founder-rejected bandaid;
  warnings that can be ignored will be.

## Verification plan

Unit: cancellable-wait (ctx cancellation unblocks), watchdog fires on
silence and not on activity, validation matrix (BLOCKED passes, evidence-
less non-BLOCKED rejected with exact errors). Integration: scripted ACP
fake that emits max-turns error → assert wrap-up prompt sent on resumed
session and report accepted; fake that goes silent → assert
`episode_hang_timeout` + artifacts present. Live spot check per CLAUDE.md
routing (one deep ask on a large repo that previously died — phoenix).

## Cross-references

- ADR-0021 (report contract this tightens), ADR-0022 (shared runtime the
  fixes land in once), ADR-0016.7 (prior reliability batch),
  ADR-0016.8 D6 (anomaly persistence these new anomalies ride on).
- FRICTION.md 2026-07-05 breaking entries (the evidence).
- NORTH_STAR §4 always-on runtime; Inspect AI log/_recover (ADR-0026) as
  prior art for interrupted-work recovery.

## Implementation Notes (2026-07-05)

Implemented in one batch on the shared runtime (`internal/sidecar`), commit
`ae832b3`. All four decisions shipped; the live spot check (phoenix deep ask)
remains the orchestrator's step per the verification plan.

### D1 — resume-as-recovery

- `Ask` (runtime.go) classifies the initial turn's error with
  `IsMaxTurnsError` (textual match on the adapter's opaque JSON-RPC
  `"Reached maximum number of turns"`) and, when it fires, runs exactly one
  recovery turn: same `ReportSinkPath`, `ACPSessionID` = the failed turn's
  session, prompt = `turnCapWrapUpPrompt` ("wrap up: call submit_report now
  with what you have; mark unverified items unverified").
- **Boundary change this required:** `RunTurnWithOptions` now returns the
  established ACP session ID *on failure too* (previously `""`), because the
  session exists the moment `NewSession` succeeds — the prompt error does not
  destroy it, and the wrap-up (plus any later ask) needs it. Failed turns also
  persist the session ID to meta (`persistACPSessionID`).
- Recovery is recorded as `TurnResult.WrapUpRecovered` →
  `TurnRecord.wrapUpRecovered` → soft anomaly `turn_cap_wrapup`
  (evals/anomalies.go). A recovered turn is NOT `ReportRetried` — the wrap-up
  is budget recovery, not a report correction; the ADR-0021 retry gate still
  applies after a reportless wrap-up.
- If the wrap-up itself fails, `Ask` ships a `BLOCKED:` report carrying both
  errors in `uncertainty` and returns **no error** — the BLOCKED report (a
  breaking `sidecar_blocked_report` anomaly in evals) plus D3 artifacts is the
  terminal state, not a lost exploration. No retries are spent after a failed
  wrap-up.
- The fix lives once in `Ask`; eval sidecar episodes call `Ask`
  (evals/runner.go), so production and evals get identical behavior
  (ADR-0022 principle) — proven by `TestMockSidecarEpisodeMaxTurnsWrapUp`,
  which asserts via the mockagent prompt log that the wrap-up arrived as a
  `LoadSession` on `mock-sess-1` with the exact prompt text.

### D2 — cancellable wait + liveness watchdog

- `RunTurnWithOptions` (acp.go) now runs `conn.Prompt` in a goroutine and
  selects over completion, `conn.Done()` (dead peer → fail immediately,
  classified by `IsPeerClosedError`), and a turn context cancelled by the
  watchdog. The turn context also feeds `exec.CommandContext`, so a watchdog
  cancellation kills the spawned agent instead of orphaning it.
- Race note found while implementing: when the agent answers and exits
  immediately (mock agents do), `promptDone` and `conn.Done()` are ready
  simultaneously and Go's select picks randomly — the dead-peer branch
  therefore drains `promptDone` within a short grace and lets a finished
  prompt win before synthesizing a peer-closed failure.
- Watchdog: `lastActivity` (atomic) is touched by every
  `denyClient.SessionUpdate` and after each successful RPC step; a goroutine
  sleeps exactly the remaining window and cancels with a cause wrapping
  `ErrLivenessTimeout`. Window resolution: `RunTurnOptions.LivenessTimeout`
  override → `GHX_SIDECAR_LIVENESS_TIMEOUT` (Go duration, `0` disables) →
  `DefaultLivenessTimeout` (10m). Pre-prompt hangs (initialize/session RPCs)
  classify the same way via `turnFailureCause`.
- `denyClient` gained a mutex + `closeAndSnapshot()`: the abandoned-turn
  paths freeze the `TurnResult` before returning so late notifications from a
  dying peer cannot race the caller's reads (`go test -race` clean, 5×).
- Signals out: `errors.Is(err, ErrLivenessTimeout)` in-process;
  `LivenessTimeoutMarker` ("liveness watchdog timeout") for artifact-only
  consumers — evals derive the **breaking** `episode_hang_timeout` anomaly
  from the persisted turn error string, profile-independent. Production
  writes the same failure into the session's `logs.jsonl` (see D3) plus the
  stderr warning path. Eval run loops print a per-episode liveness line every
  `episodeLivenessInterval` (1m) — `eval episode live task=… profile=…
  elapsed=…` — so a human sees a stall as it happens.
- Severity call made here: `episode_hang_timeout` is **breaking** (the turn
  died without a report), matching the anomaly taxonomy's definition; the ADR
  text did not pin a severity.

### D3 — artifacts on every path

- `emitTurnArtifacts` now runs on every `Ask` exit path and wraps its context
  in `context.WithoutCancel` — the flush must survive the very cancellation
  it documents (a watchdog-cancelled or deadline-exceeded context previously
  would have aborted the tracer-provider work). Acceptance tests:
  `TestAskEmitsArtifactsOnFailedTurn` (traces.jsonl + logs.jsonl with the
  partial tool spans and the error) and
  `TestAskEmitsArtifactsOnFailedTurnWithCancelledContext`.
- Failed turns mark the `sidecar.turn` span status Error and append a
  `sidecar.turn.error` log record (`error.message`, session/repo/turn attrs)
  — written regardless of the content-capture setting, since it carries the
  failure, not message content. `turnTelemetry` gained the `Error` field;
  wrap-up recovery is also visible as `ghx.sidecar.wrap_up_recovered` on the
  turn span.
- "Incremental" is implemented as *flush-on-every-exit-path of the shared
  runtime*, not as per-span streaming during the turn: the ACP client
  accumulates the turn in memory and every return path (success, turn-cap
  with/without recovery, watchdog, peer-closed, validation-terminal WARN)
  emits what exists. A turn killed by SIGKILL of the ghx process itself still
  loses its in-memory traces — that residual gap belongs to the always-on
  runtime milestone (NORTH_STAR §4), not this batch.
- `Ask` returns the partial `TurnResult` alongside the error (previously
  `nil`), and the eval runner folds it into the failed turn's record
  (`populateTurnRecord`), so failed episodes carry the same audit fields as
  successful ones.

### D4 — evidence-required validation

- `ValidateReportEvidence` (reportsink.go) is called from
  `DecodeReportStrict`, i.e. the strict submit_report path exactly as ADR-0021
  wired it — rejections are in-band MCP tool errors the model must fix.
  Requirement: a non-BLOCKED report needs **all three** of ≥1 `verified`
  claim with a non-empty `evidence` field, ≥1 `relevantFiles` entry with a
  path, ≥1 non-blank `commandsRun` entry; the error names each missing field
  exactly. A `BLOCKED` report must state why after the prefix and skips the
  requirement.
- Interpretation note: the decision text's "lacks all three of" was
  implemented as *must have all three* (reject when any is missing, naming
  the missing ones). The weaker reading (reject only when all three are
  absent) would still accept near-answer-only reports — e.g. one command and
  no claims — which is the exact contract gap in Evidence; "exact field-level
  errors" only makes sense per-field.
- The lenient `<ghx-report>` text fallback (ADR-0021 D3) is deliberately NOT
  gated on evidence: it exists for degraded transports, its use is already a
  counted anomaly, and hard-failing it would convert degraded-but-answerable
  turns into WARN losses. The strict tool path is where the contract binds.
- Persona (prompt.go `## submit_report`) states the requirement up front
  ("Evidence is required, not optional", the three bullets, the BLOCKED
  exception) — rejection is the backstop, not the teacher. The tool
  description and derived schema description state it too.

### Test inventory (all committed, no live tokens)

- Unit: `TestValidateReportEvidence_Matrix` (full/answer-only/partial/BLOCKED
  ±reason), `TestDecodeReportStrict_EvidenceRequired`, wrap-up runtime tests
  (`TestAskWrapsUpOnMaxTurns`, `TestAskBlockedWhenWrapUpFails`,
  `TestAskReturnsPartialResultOnUnrecoveredFailure`), anomaly detection
  (`TestDetectAnomaliesTurnCapWrapUp`, `TestDetectAnomaliesEpisodeHangTimeout`
  incl. non-hang non-classification).
- Scripted-ACP-fake integration (mockagent grew `promptError`, `hangMs`,
  `updateCount/updateIntervalMs`, `exitBeforeResponse`, and a
  `MOCKAGENT_PROMPT_LOG` audit channel): watchdog fires on silence
  (`TestRunTurnLivenessWatchdogCancelsSilentTurn`) and not on activity
  (`TestRunTurnWatchdogStaysQuietUnderActivity`), dead peer fails fast
  (`TestRunTurnDeadPeerFailsImmediately`), session ID survives prompt errors
  (`TestRunTurnReturnsSessionIDOnPromptError`), and full-episode proofs
  `TestMockSidecarEpisodeMaxTurnsWrapUp` /
  `TestMockSidecarEpisodeHangTimeout` (max-turns → wrap-up on resumed
  session → report accepted; silence → `episode_hang_timeout` + artifacts
  present).
- Transport: `TestReportSinkStdioEndToEnd` extended with the evidence-less
  rejection over the real spawned `ghx sidecar report-sink` stdio server.
- `go vet ./...`, `go test ./...`, and `go test -race` on
  `internal/sidecar{,/evals}` (5 consecutive runs) pass.

### Follow-ups

- D1-adjacent stale-session resilience shipped in `aeb86c3` (2026-07-06):
  when a persisted ACP session ID cannot be loaded because the adapter returns
  Resource not found (`-32002`), the shared `Ask` runtime creates exactly one
  fresh ACP session and continues the turn using the durable named-session
  ledger context already embedded in the prompt. The new transport session ID
  is persisted to `meta.json`, and the downgrade is visible as
  `TurnResult.SessionRecreated`, `TurnRecord.sessionRecreated`,
  `ghx.sidecar.session_recreated` / `ghx.eval.session_recreated`, and a
  `sidecar.session.recreated` log entry. Non-resource LoadSession failures and
  failures of the fresh session still fail loudly.
- Live spot check per the verification plan (phoenix deep ask) — orchestrator.
- D4 changes the sidecar product: per ADR-0021 D4's precedent, no
  reliability numbers are citable until the next pre-registered confirmatory
  run; expected (not citable): answer-only accepted reports go structurally
  to zero on the submit_report path.
- Direct (plain/ghx) eval episodes still prompt the ACP connection without
  the sidecar watchdog; they are bounded by the runner's episode context
  and now print liveness lines, but a shared-watchdog refactor for direct
  profiles is open if a direct-profile zombie is ever observed.
