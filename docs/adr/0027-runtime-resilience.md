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
