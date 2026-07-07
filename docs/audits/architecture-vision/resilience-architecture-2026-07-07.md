---
title: "Resilience & Runtime-Robustness Audit — ghx under failure, and the target design"
date: "2026-07-07"
status: "audit"
author: "reliability/runtime architect (background audit worker)"
scope: "internal/sidecar/**, internal/cli/errors.go — read-only audit, no code changes"
persona: "resilience, failure-handling, runtime robustness"
---

# Resilience & Runtime-Robustness Audit — 2026-07-07

Read-only audit of how ghx behaves under failure, and how that behaviour should
shape the target architecture as the runtime becomes pluggable across runners
(ACP-claude / codex-acp / Claude Agents SDK). Every finding cites `file:line`
against the working tree. Fable decides what to act on; this report finds,
ranks by real blast radius, and proposes a behavior-preserving sequence. It
changes no code and — by construction — nothing in the eval measurement stack
(`internal/sidecar/evals`).

Grounded in: `docs/NORTH_STAR.md` §4 (always-on runtime, "explorations are
never lost to process boundaries"); `AGENTS.md` Visibility/Truthfulness (frozen
measurement stack) and Evidence Contract; ADR-0027 (runtime resilience),
ADR-0030/0030.1 (warm daemon + routing), ADR-0034 (proposed unified
failure-class model); and direct reading of the resilience code.

**Input note.** Three inputs the task references do not exist in this worktree:
`docs/adr/0035-*`, `docs/audits/failure-class-inventory-2026-07-07.md`, and the
coupling-cohesion / adversarial-velocity audits (parallel-fleet WIP at audit
time). Findings that would have leaned on them are derived instead from ADR-0034
and the code directly, and the "B4 failure-class inventory" contract is
reconstructed from `internal/sidecar/evals/anomalies.go` + the eval runner's
error persistence. Also: `acp.go` is now 387 lines — the H2 god-file split in
`docs/audits/architecture-2026-07-07.md` (then 1012 lines) has landed
(`turn.go`, `turnresult.go`, `agentdiag.go` are its extracted siblings).

## Executive summary

**The resilience machinery ADR-0027 shipped is genuinely good, and most of it
sits in the right layer.** The recovery *policy* — wrap-up-on-turn-cap,
fresh-session-on-stale-session, report-retry, watchdog, artifacts-on-every-path
— lives in the runtime orchestrator (`internal/sidecar/runtime.go`), one layer
*above* the ACP adapter, exactly where a policy that must outlive the transport
belongs. Failed turns are auditable (D3), reports without evidence are rejected
in-band (D4), and the eval anomaly layer recomputes failure counts offline from
persisted strings. None of that is broken.

**The single highest-leverage *architectural* gap is the layer the policy sits
on: failure _classification_ is textual string-matching hard-wired to one
runner, and the recovery policy depends on those predicates.** `IsMaxTurnsError`
matches the claude-agent-acp string `"Reached maximum number of turns"`
(`acp.go:61,78`); `IsPeerClosedError` matches the `coder/acp-go-sdk` string
`"peer connection closed"` (`acp.go:64,84`); stale-session detection matches
JSON-RPC `-32002` + `"resource not found"` (`acp.go:67,96`). The runtime's
recovery branches call these predicates directly (`runtime.go:171,445`). **The
moment a second runner arrives (codex-acp, Claude Agents SDK), every recovery
path silently no-ops** — codex's turn-budget error is not that string, a native
SDK's context-cancellation is not that string — so resilience is
re-implemented-per-adapter *by default*, or worse, silently absent. This is the
finding that most directly threatens the founder's pluggable-runner goal.

**The single highest-leverage *runtime* risk is the daemon's blast radius.** The
sidecar is now a resident, multi-session process (ADR-0030), but two properties
of a one-shot command survived into it unchanged: (1) **zero panic recovery
anywhere** — `grep -rn 'recover()' internal/sidecar/*.go` (non-test) is empty —
so one poisoned turn's panic in a per-connection goroutine (`go s.handleConn`,
`daemon.go:151`) crashes the whole daemon and every warm session with it; and
(2) **the client's context is dropped at dispatch** — `dispatch(context.Back
ground(), req)` (`daemon.go:195`) and the worker's turn context is itself
`Background`-rooted (`daemon_worker.go:185`) — so a cancelled or dead client
leaves the turn running to the 10-minute watchdog, head-of-line-blocking the
serialized per-session worker and burning tokens no consumer will read. These
are risks the resident daemon *introduced*; ADR-0027 was written for a process
that died after one turn and never had to survive a second caller.

**The domain-model answer (ADR-0034 vs the frozen B4 strings) is: add a typed
class _below_ the marker string, never instead of it.** The markers are a
cross-artifact contract the eval layer recomputes from; a typed `TurnFault`
whose classes each *own* their canonical marker gives the runtime a
type-checked branch while the persisted `turn.Error` stays byte-identical — so
`internal/sidecar/evals` is untouched and no eval-ADR pre-registration is
needed for the runtime refactor.

### What is already right (verified, not assumed)

- **Recovery policy is above the adapter.** `recoverMaxTurns` (wrap-up),
  `runTurnWithStaleSessionFallback` (fresh session), `resolveReportWithRetry`
  (corrective retries) all live in `runtime.go` (`:444`, `:169`, `:478`), not in
  the ACP transport. Swapping the transport does not, in principle, move the
  policy — the coupling is only through the classifier predicates.
- **Artifacts flush on every exit path (D3).** `emitFailedTurnArtifacts`
  (`runtime.go:551`) runs on the error path, and `emitTurnArtifacts` wraps its
  context in `context.WithoutCancel` so a watchdog-cancelled or deadline-exceeded
  context cannot abort the flush that documents it (ADR-0027 D3 implementation
  note).
- **The cancellable wait is real.** `waitForACPPrompt` (`turn.go:100`) selects
  over prompt-completion, `conn.Done()` (dead peer), parent-ctx, and the
  watchdog turn-ctx — the round-4 zombie-hang root cause (a non-cancellable
  wait) is fixed, and the watchdog runs on both the daemonless path
  (`turn.go` via `RunTurnWithOptions`, `acp.go:266`) and the warm worker
  (`daemon_worker.go:227`).
- **The marker→anomaly contract is offline-recomputable.** `DetectAnomalies`
  (`anomalies.go:107`) derives `episode_hang_timeout` from
  `strings.Contains(turn.Error, LivenessTimeoutMarker)` (`:115`) with no agent,
  network, or scoring side-effect — the B4 invariant the frozen-stack tenet
  depends on.

---

## Current resilience map

Failure mode → where it is classified → where it is recovered → adequacy. "Class
site" is where the failure is *named*; "policy site" is where the runtime
*decides* what to do. The split is the whole story.

| # | Failure mode | Class site (`file:line`) | Policy site (`file:line`) | Adequacy |
|---|---|---|---|---|
| 1 | Turn-cap exhaustion | `IsMaxTurnsError` — string `"Reached maximum number of turns"` `acp.go:61,78` | `recoverMaxTurns` one-shot wrap-up resume `runtime.go:444-476` | **Good policy, runner-locked class** |
| 2 | Liveness / zombie hang | `startACPWatchdog` + `ErrLivenessTimeout` / `LivenessTimeoutMarker` `turn.go:19`, `acp.go:58,73` | cancellable wait `turn.go:100-133`; watchdog on both paths `acp.go:266`, `daemon_worker.go:227` | **Good** (runner-agnostic: ghx owns the clock) |
| 3 | Dead peer (adapter crash) | `IsPeerClosedError` — string `"peer connection closed"` / `"peer disconnected"` `acp.go:64,84` | fail-fast `turn.go:104-113`; worker `shutdownLocked` `daemon_worker.go:158,167` | **Good policy, runner-locked class** |
| 4 | Stale ACP session | `IsLoadSessionResourceNotFound` — code `-32002` + `"resource not found"` `acp.go:67,96` | fresh `NewSession` fallback `runtime.go:169-182` | **Good policy, runner-locked class** |
| 5 | Missing / invalid report | `ReadSinkReport` / `ExtractReportErr` `runtime.go:188-204` | ≤2 corrective retries → WARN fallback `runtime.go:478-508` | **Good, but retry-transport error is swallowed** (M4) |
| 6 | Evidence-less report | `ValidateReportEvidence` `reportsink.go:108-144` | in-band MCP tool error, model re-submits `reportsink.go:358-361` | **Good** (runner-agnostic; own MCP contract) |
| 7 | Artifacts on failure | — | `emitFailedTurnArtifacts` + `WithoutCancel` `runtime.go:551-571` | **Good, except SIGKILL-of-ghx** (M2) |
| 8 | Eval anomaly derivation | persisted `turn.Error` at `runner.go:445` | `DetectAnomalies` substring reads `anomalies.go:115`, `parallel.go:96` | **The frozen B4 contract** — see domain-model section |
| 9 | Client cancel / timeout (daemon) | — | dropped: `dispatch(context.Background())` `daemon.go:195`; worker `turnCtx` Background-rooted `daemon_worker.go:185` | **Fragile** (H3) |
| 10 | Turn panic (daemon) | — | none: no `recover()` in `internal/sidecar` | **Fragile — whole-daemon blast radius** (H2) |
| 11 | Daemon signal / upgrade drain | — | `RunDaemon(context.Background())` `sidecar.go:147`; `ctx.Done()` never fires `daemon.go:135` | **Fragile** — D6 drain unimplemented (M1) |
| 12 | Recon failure-class (no-results / bad-input / upstream) | CLI only: `errors.go:11-17` + `upstreamRules` | CLI exit codes; MCP flattens; sidecar bare `fmt.Errorf` | **Three divergent models** (ADR-0034 / M3) |

Two structural observations fall out of the table. First, **every "runner-locked
class" row (1, 3, 4) pairs a good, runner-agnostic policy with a
single-runner classifier** — that is precisely the seam to cut. Second, **rows
9–11 are all the same root**: the daemon inherited one-shot-process assumptions
(no per-request context, no panic isolation, no drain) into a resident,
multi-tenant process, where each of those assumptions now has an N-session
blast radius.

## Severity tiers

### High

#### H1 — Failure classification is runner-locked string-matching; recovery policy depends on it

**What.** The three recovery-triggering predicates classify by matching one
runner's error text/codes, and the runtime's recovery branches consume the
predicates directly.

**Evidence.** `maxTurnsErrorMarker = "Reached maximum number of turns"`
(`acp.go:61`, claude-agent-acp's phrasing); `peerClosedMarker = "peer
connection closed"` (`acp.go:64`, the `coder/acp-go-sdk` `connection.go`
cause); `resourceNotFoundCode = -32002` (`acp.go:67`). The runtime branches on
`IsMaxTurnsError(err)` (`runtime.go:445`) and `IsLoadSessionResourceNotFound(err)`
(`runtime.go:171`); the worker branches on `IsPeerClosedError(err)`
(`daemon_worker.go:158,167`). ADR-0027 D1 itself flags the fragility: matching
is "textual because the error arrives as an opaque JSON-RPC internal error from
the adapter" (`acp.go:76-77`).

**Why it's High.** NORTH_STAR §P4 and the "protocols are stepping stones" tenet
require the boundary to survive replacing ACP or the runner. Today it does not:
a codex-acp turn-budget error or a Claude Agents SDK cancellation carries
different text, so `IsMaxTurnsError` returns false, `recoverMaxTurns` no-ops,
and the exploration is lost exactly as the pre-ADR-0027 phoenix incident lost
it — silently, with no compile-time signal that a runner is unsupported. The
policy is written once and correctly; the classifier is where per-runner
re-implementation will accrete unless the seam is cut now.

**Direction.** A `RunnerFault` port: the adapter classifies its own faults into
a shared typed vocabulary; the runtime branches on the vocabulary, never on the
string. (Target-architecture section.)

#### H2 — The daemon has no panic recovery; one turn crashes every warm session

**What.** No `recover()` exists anywhere in `internal/sidecar` (verified: empty
grep). The daemon serves each client on `go s.handleConn(conn)` (`daemon.go:151`)
and runs the turn synchronously inside it via `AskWithTurnRunner`
(`daemon.go:220`). Any panic — a nil-deref while parsing a malformed adapter
notification in `denyClient.SessionUpdate`, a bad type assertion in trace
summarization, an OOB slice — propagates up the goroutine and terminates the
process.

**Why it's High.** For a one-shot command a panic killed one command. For the
resident daemon it kills the accept loop, every warm `AgentWorker`, and every
in-flight turn across every session (ADR-0030 D4's pool is a single process).
The blast radius scaled with the daemon; the isolation did not. This is the
classic resident-server failure the Go stdlib guards against — `net/http`
recovers per request precisely so one handler panic cannot take down the
server. The daemon is a server and must adopt the same discipline.

#### H3 — Client context dropped at dispatch; cancellation and timeouts never reach the turn

**What.** `handleConn` calls `s.dispatch(context.Background(), req)`
(`daemon.go:195`), so the per-request context is synthetic and un-cancellable.
Worse, the warm worker roots its own turn context at `context.Background()`
too (`daemon_worker.go:185`), so even threading a real ctx to `dispatch` would
not, by itself, let a cancel kill the adapter (its process is bound to
`w.turnCtx`). There is also no goroutine watching the client connection for
EOF during a long `Ask`, so a client that disconnects mid-turn is not noticed
until the daemon tries to write the response (`enc.Encode`, `daemon.go:201`).

**Why it's High.** The per-session worker serializes turns (ADR-0030 D4), so an
abandoned turn is not merely wasted tokens (running to the 10-minute watchdog,
`DefaultLivenessTimeout`, `acp.go:53`) — it head-of-line-blocks every queued ask
for that session. The CLI wires `signal.NotifyContext` for its follow tail
(`sidecar.go:308`) but the ask path passes `context.Background()`
(`sidecar.go:91`), and the daemon would discard a real ctx anyway. The only
liveness bound on a daemon turn today is the watchdog; there is no
client-driven cancellation and no per-Ask wall-clock deadline.

**Nuance for the fix.** Warm reuse means cancellation should abandon the
*prompt* and optionally send an ACP cancel, but keep the adapter process warm —
not kill it (killing defeats ADR-0030 D4). So H3's fix is: (a) a per-request
context derived from the RPC with a connection-close watcher, threaded into
`configureACPSession`/`runACPPrompt` (which already select on `ctx.Done()`,
`turn.go:85`), plus (b) a decision on whether a cancel also cancels
`w.turnCtx`. The current code decouples the two on purpose; the fix must make
the *prompt wait* cancellable without evicting the warm worker.

### Medium

#### M1 — No signal-aware graceful drain; SIGTERM orphans adapters and skips D6

**What.** `RunDaemon` is invoked with `context.Background()` (`sidecar.go:147`),
and `Serve` only shuts down on `ctx.Done()` or the internal `stop` channel
(`daemon.go:135-140`). Since ctx never cancels, a SIGTERM/SIGINT to the daemon
takes the OS default (immediate termination): warm adapter subprocesses bound to
`w.turnCtx` are orphaned (no `ShutdownAgent`), and in-flight turns never reach
`emitFailedTurnArtifacts`. The only clean stop is the `ghx.sidecar.Shutdown`
RPC / `--stop`.

**Why it matters.** ADR-0030 D6 promises an upgrade flow where the daemon "stops
accepting new asks, lets active turns finish up to a short deadline, flushes
artifacts, closes ACP subprocesses, and exits", and "if an active turn cannot
finish before the deadline, it is marked BLOCKED or failed with artifacts". None
of that drain is implemented — `Shutdown()` (`daemon.go:156-163`) closes the
listener and the pool hard-kills adapters; there is no deadline, no wait for
active `AskWithTurnRunner` goroutines (which are Background-rooted and
untracked), and no BLOCKED marking. Ranked Medium not High because the common
path (RPC shutdown on version-mismatch replacement, `daemon.go:478`) works and
the artifact loss is bounded to genuinely-in-flight turns.

#### M2 — In-flight state loss on hard kill, widened by the resident daemon

**What.** ADR-0027 D3 is explicit that "incremental" means flush-on-every-exit-
path, not per-event streaming: "A turn killed by SIGKILL of the ghx process
itself still loses its in-memory traces." ADR-0030 D6 restates this as the
residual v1 gap. The daemon *widens* the window: a one-shot command lost one
turn's traces; the daemon holds up to `DaemonMaxConcurrentTurns` in-flight turns
plus warm session state, all in one process's memory.

**Why it matters.** NORTH_STAR §4 — "explorations are never lost to process
boundaries" — is the literal promise this gap violates at the extreme. It is
Medium because SIGKILL (vs SIGTERM, which M1 addresses) is rare in normal
operation and the durable ledger/`ACPSessionID`/accepted reports survive on disk
(ADR-0030 D6); what is lost is only unflushed telemetry of the killed turn.
Full per-event streaming is correctly deferred (ADR-0030 out-of-scope), but the
daemon should at minimum honor D6's "emit artifacts on every completed or failed
Ask path" — which M1's drain delivers.

#### M3 — Three divergent failure models across surfaces (ADR-0034 territory)

**What.** Recon failure-class exists only in the CLI (`errors.go:11-17`
exit-code taxonomy + `upstreamRules` classifier); core returns bare
`fmt.Errorf`; MCP flattens to opaque strings; the sidecar has its own
BLOCKED/marker vocabulary. ADR-0034 (proposed) already documents this as M2 and
proposes lifting a typed `FailureClass` into core.

**Why it matters here.** This is a *different axis* from the runtime markers
(H1) — see the domain-model section — and its migration (ADR-0034) is
independent of the runner-resilience work. Flagged so the two are not conflated
into one refactor. Ranked Medium and deferred to ADR-0034's own acceptance.

#### M4 — The report-retry loop swallows transport errors into an indistinct WARN

**What.** `resolveReportWithRetry` breaks the loop on `retryErr != nil`
(`runtime.go:493-494`) and, if no report ever materializes, ships
`warnNoReportAnswer` (`runtime.go:507`). A retry that failed because the
*producer* drifted and a retry that failed because the *transport/adapter* died
mid-retry both land on the same WARN answer, which evals classify as the same
breaking `sidecar_report_missing` (`anomalies.go:151`).

**Why it matters.** It is a small honesty gap in an otherwise loud failure path:
a transport death during recovery is a different failure than a model that
cannot produce a valid report, and the merged signal hides which. Ranked Medium
because both are already breaking anomalies (the episode is not silently
counted as success) — the loss is diagnostic resolution, not correctness.

### Low

#### L1 — The watchdog bounds silence, not liveness-with-no-progress

**What.** The watchdog fires only on *no ACP session update* for the window
(`turn.go:22-27`). An adapter that keeps emitting updates but never converges —
a genuinely looping agent below the turn cap — is bounded only by the turn cap
(itself recovered, not stopped) and never by wall-clock. There is no per-Ask
maximum duration.

**Why it's Low.** In practice the max-turns net (`recoverMaxTurns`) catches the
runaway-tool-call shape, and adding a hard wall-clock cap risks killing
legitimately-deep asks. Flag for a *configurable* per-Ask deadline (off by
default), not a forced one.

#### L2 — The daemonless CLI ask is also un-cancellable

**What.** `sidecar.go:91` passes `context.Background()` to `AskViaDaemon`, so
Ctrl-C during a daemonless ask takes the process down via default SIGINT,
losing in-memory traces (the M2 shape on the CLI path). The watchdog and
peer-closed handling still apply; only user-driven cancellation is absent.

**Why it's Low.** Same underlying gap as H3/M2; called out separately so the fix
covers the CLI entrypoint, not only the daemon.

---

## Resilience in the target architecture

The founder wants pluggable runners with different failure modes. The organizing
principle: **classification is the runner's job; recovery is the shared layer's
job; the marker string is the contract between the shared layer and the eval
artifacts.** Two layers, one contract crossing each boundary.

### Layer 1 — the Runner port (per-adapter, the only place that knows the runner)

Each runner (ACP-claude, codex-acp, Claude Agents SDK) implements a small
interface. Its two responsibilities:

1. **Execute one turn and stream activity.** New/load/resume a session, send a
   prompt, stream text/thought/tool events, and — crucially — emit the liveness
   `touch` signal ghx's watchdog clock consumes (today `onActivity`,
   `acp.go:254`, `daemon_worker.go:223`). The watchdog *timer* stays in the
   shared layer; the runner only feeds it heartbeats.
2. **Classify its own faults.** The runner is the only code that knows its own
   error dialect. It maps a raw failure to a shared, typed `RunnerFault`:

   ```go
   type FaultClass int
   const (
       FaultNone FaultClass = iota
       FaultBudgetExhausted   // claude: "Reached maximum number of turns"; codex: its own
       FaultPeerGone          // process exit / closed stdio, however the SDK words it
       FaultSessionNotFound   // ACP -32002; SDK: its own "no such session"
       FaultAuthRequired      // adapter -32000 auth (agentdiag.go:255 already models this)
       FaultLiveness          // ghx-owned; the shared layer sets this, not the runner
   )
   type RunnerFault struct {
       Class FaultClass
       Err   error   // wrapped raw error, errors.As-compatible
   }
   ```

   Classification happens **at the source, from structured signals where the
   runner has them** (JSON-RPC codes like `-32002`), textual matching only as
   the runner's *private* fallback — never re-parsed one layer up as it is
   today.

What belongs in the port: fault classification, one-turn execution, the activity
heartbeat, session new/load/resume primitives, and process lifecycle
(`ShutdownAgent`-equivalent). What must **not** leak into it: the recovery
policy, the watchdog timer, report retry, or the persisted-error shape.

### Layer 2 — the shared resilience layer (runner-agnostic, above the port)

This is almost exactly what `runtime.go` is today; the change is that it branches
on `FaultClass`, never on `IsMaxTurnsError(err)`:

- **Recovery policy** — wrap-up on `FaultBudgetExhausted`, fresh session on
  `FaultSessionNotFound`, report retry on missing report, BLOCKED on
  unrecovered failure. Written once, correct for every runner.
- **The watchdog timer** (ghx owns the clock; the runner only heartbeats).
- **Artifact flush on every exit path** (`emitFailedTurnArtifacts`), including
  the graceful-drain path M1 adds.
- **Panic isolation and context/deadline management** — the daemon-supervisor
  concerns below.
- **The marker registry** — the class↔string mapping that preserves B4.

### Layer 3 — the daemon as a supervisor (idiomatic Go server discipline)

The daemon is a resident server; it should adopt the discipline `net/http`
already encodes:

- **Per-request `recover()`** in `handleConn`/`dispatch`, converting a turn
  panic into a failed `Ask` (with artifacts) that kills *that* turn and,
  optionally, retires *that* warm worker — never the daemon (fixes H2).
- **Per-request context derived from the RPC** with a client-disconnect watcher
  and an optional deadline, threaded into the turn (fixes H3). Cancel abandons
  the prompt wait; the warm worker survives unless the fault says otherwise.
- **Signal-aware graceful drain** — `signal.NotifyContext` into `RunDaemon`, a
  bounded drain that stops accepting, lets active turns flush, and marks the
  rest BLOCKED (fixes M1, implements ADR-0030 D6).

This is a *supervision boundary*, the Go-idiomatic analogue of an
Erlang-style "let it crash, isolate the blast": the failure of one turn is
contained, observed (artifacts), and does not propagate to siblings.

### How recovery signals cross the boundary while preserving the frozen marker contract

This is the crux, and it is what makes the whole refactor behavior-preserving
for evals. The eval anomaly layer recomputes failure counts by substring-reading
the **persisted `turn.Error` string** (`anomalies.go:115` reads
`LivenessTimeoutMarker`; `parallel.go:96` reads rate-limit shapes;
`runner.go:445` is where the string is persisted). That is the B4 contract, and
`AGENTS.md` freezes it: any change to scoring/derivation is a measurement-stack
change requiring a pre-registered eval ADR.

The reconciliation: **each `FaultClass` owns its canonical marker string, and the
shared layer writes that string into `turn.Error` regardless of which runner
produced the fault.**

```go
func (c FaultClass) Marker() string {
    switch c {
    case FaultLiveness:      return LivenessTimeoutMarker    // "liveness watchdog timeout"
    case FaultBudgetExhausted: return maxTurnsErrorMarker    // stays the same string
    // ...
    }
}
```

Consequences:

- A codex-acp `FaultBudgetExhausted` and a claude-acp `FaultBudgetExhausted`
  persist the **same** marker, so evals recompute identically across runners —
  the measurement stack stays frozen *and* becomes runner-portable in one move.
- `internal/sidecar/evals` is **not touched**: it keeps reading the same strings
  from `turn.Error`. Therefore the runtime refactor needs **no eval-ADR
  pre-registration** — a golden test proving `turn.Error` strings byte-identical
  before/after is the evidence that the frozen stack is respected.
- The typed class is the **in-process** contract (the runtime branches on it);
  the marker is the **cross-artifact** contract (evals derive from it); one
  canonical mapping means they can never drift.

## The failure DOMAIN MODEL — typed classes vs marker strings

Reconciling ADR-0034 with the B4 recompute contract turns on one distinction the
two ADRs are about **two different domains on two different axes**:

- **ADR-0034's `FailureClass`** classifies the *reconnaissance operation*:
  no-results vs bad-input vs upstream (GitHub 404/401/rate-limit). It is a
  **core (`internal/ghx`)** concept surfaced to CLI exit codes and MCP payloads.
  Its consumer is the *main agent* deciding whether to refine a query or fix
  auth. It does **not** touch the eval marker strings.
- **ADR-0027's markers** classify the *turn/transport fault*: budget /
  liveness / peer-gone / stale-session. It is a **sidecar-runtime** concept.
  Its consumer is the recovery policy and the eval anomaly layer. It is the one
  bound by B4.

They should be **two typed enums in two packages**, not one. Conflating them
(e.g. adding `FaultLiveness` to core's `FailureClass`) would drag the frozen
eval strings into a core refactor and force an eval ADR for unrelated work.

The rule set:

1. **Add the typed class below the string, never instead of it.** Introduce
   `sidecar.TurnFault{Class, Err}` as the typed twin of today's markers.
   Reimplement `IsMaxTurnsError`/`IsPeerClosedError`/`IsLoadSessionResourceNot
   Found`/`ErrLivenessTimeout` as `FaultClassOf(err) == …`, via `errors.As` on
   the typed fault, keeping textual fallback only for opaque adapter errors.
2. **The persisted `turn.Error` must still contain `Class.Marker()`.** This is
   the invariant that keeps `internal/sidecar/evals` byte-stable and the refactor
   out of eval-ADR scope. Prove it with a golden test.
3. **The typed class is the in-process branch; the marker is the artifact
   contract.** Changing the class↔marker mapping *is* a measurement-stack change
   → pre-registered eval ADR. Adding the typed layer while preserving the mapping
   is behavior-preserving → no eval ADR.
4. **ADR-0034's core `FailureClass` proceeds on its own axis**, mapping core →
   CLI/MCP, untouched by and untouching the runtime markers.

This is the only design that satisfies both "typed domain models" (AGENTS.md
engineering tenet, ADR-0034) and "every score recomputable from committed
artifacts / measurement stack frozen" (Visibility & Truthfulness) at once.

## Recommended sequence (phased, non-rewrite)

Ordered by blast-radius reduction per unit of risk. Each phase is independently
shippable and independently verifiable; none touches `internal/sidecar/evals`.

1. **Daemon panic isolation + client-context propagation (H2 + H3).** Smallest
   change, largest blast-radius payoff, zero new abstraction, zero eval impact.
   Per-connection `recover()` in `handleConn`; a per-request context derived from
   the RPC with a connection-close watcher and optional deadline, threaded into
   `configureACPSession`/`runACPPrompt` (which already select on `ctx.Done()`).
   Verify: a fake-adapter panic test proves the daemon survives and the turn
   fails with artifacts; a client-cancel test proves the prompt wait unwinds
   while the warm worker survives. This directly de-risks the daemon Goga just
   shipped.

2. **Signal-aware graceful drain (M1, implements ADR-0030 D6).** Wire
   `signal.NotifyContext` into `RunDaemon`; on signal, stop accepting, cancel
   active turns with a bounded deadline, let `emitFailedTurnArtifacts` flush, and
   mark undrained turns BLOCKED. Verify: SIGTERM to a daemon mid-turn leaves
   `traces.jsonl` + a BLOCKED report in the session dir and no orphaned adapter
   process.

3. **Introduce the typed `TurnFault` below the markers (domain model; H1
   groundwork).** Add `TurnFault` + `FaultClass` + `Class.Marker()`; reimplement
   the four predicates as class checks with textual fallback retained. Verify:
   a golden test asserts `turn.Error` strings are byte-identical to today for
   every fault path — the proof that the frozen stack is untouched, so **no eval
   ADR is required**. State that invariant explicitly in the runtime ADR.

4. **Extract the Runner port (H1, the pluggable-runner enabler).** Define the
   runner interface (execute-turn + heartbeat + `ClassifyFault` + session
   primitives); make the existing ACP path its first implementation; the
   recovery policy in `runtime.go` now consumes `FaultClass`. Do this *after*
   step 3 so the port has a typed contract to speak. A second runner
   (codex-acp) then only implements `ClassifyFault` for its own dialect — the
   recovery policy is inherited, not re-implemented. Verify: a fake second
   runner whose budget error is a *different string* still triggers wrap-up
   recovery.

5. **(ADR-0034, separately) Lift recon `FailureClass` into core.** Independent
   axis, independent migration (core → CLI/MCP → sidecar report), gated on
   ADR-0034's own acceptance. Do not fold it into steps 1–4.

Rationale for the order: steps 1–2 are pure runtime-robustness on the resident
daemon, no new ports, immediate payoff, and the highest real risk today. Step 3
is the behavior-preserving domain-model move that unblocks step 4 *without*
tripping the frozen-stack rule. Step 4 is the architecture the founder asked
for. Step 5 is orthogonal and ADR-owned. Nothing here is a rewrite; each step is
an incremental, test-verifiable change consistent with ADR-0025's
incremental-delivery posture and ADR-0027/0030's own implementation style.

## Method / auditability

- **Every `file:line` was spot-checked against the working tree** at the audit
  commit (`d2cda8a`, worktree branch `worktree-agent-ae1c41b7a20edef9c`);
  `go build ./...` is clean and **no code was modified** by this audit.
- **Panic-recovery absence:** `grep -rn 'recover()' internal/sidecar/*.go`
  filtered to non-test files returns empty.
- **Dropped-context claim:** `context.Background()` at `daemon.go:195`
  (dispatch), `daemon_worker.go:185` (worker turn ctx), `sidecar.go:147`
  (daemon command), `sidecar.go:91` (CLI ask). The follow-tail signal wiring at
  `sidecar.go:308` was confirmed to belong to `sidecar live --follow`, not the
  ask/daemon path.
- **Marker/anomaly contract:** classifier constants `acp.go:46-73`; string
  derivation `anomalies.go:115` and `parallel.go:96`; persistence of
  `turn.Error` at `runner.go:445`. The typed-boolean anomalies
  (`turn_cap_wrapup`, `sidecar_report_retried`, `sidecar_report_coerced`) derive
  from `TurnResult` fields (`turnresult.go:39,46,51`), not strings — noted
  because it shows the codebase already mixes typed and string derivation, and
  the string ones are the constrained set.
- **CLI recon taxonomy (ADR-0034 input):** `errors.go:11-17` (exit codes),
  `errors.go:97-148` (`upstreamRules`/`CodeForError`).
- **Inputs absent from this worktree** (parallel-fleet WIP): `docs/adr/0035-*`,
  `docs/audits/failure-class-inventory-2026-07-07.md`, coupling-cohesion and
  adversarial-velocity audits — findings were derived from ADR-0034 and code
  directly rather than from those documents.
- **No web research was needed:** the idiomatic-Go patterns invoked
  (`errors.As`/typed sentinels, `context.WithCancelCause`/`context.Cause`,
  per-request `recover()` as in `net/http`, `signal.NotifyContext` graceful
  shutdown) are stable stdlib practice within the knowledge cutoff; per
  AGENTS.md Tool Economy, spending browse tokens on them would be waste.
