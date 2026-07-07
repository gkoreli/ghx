---
title: "Independent Code Review — ADR-0035 / ADR-0036 Safety-Critical Refactors"
date: "2026-07-07"
status: "complete"
thread: "architecture"
reviewer: "Independent reviewer (Opus 4.8), separate worktree, read-only except this doc"
scope: "Second set of eyes on today's landed turn-engine / daemon-supervisor / Repo-flow refactors"
---

# Independent Code Review — ADR-0035 / ADR-0036 Refactors (2026-07-07)

## TL;DR verdict

**Safe to keep as landed.** No High-severity finding. The turn-engine
consolidation, daemon-supervisor hardening, and `ghx.Repo` flow all preserve the
behavior they claim to, with one honest exception:

- **One Medium finding** (T1.3): a named-return + deferred-closure interaction
  changes the **live turn-log error string on the failed-turn path** from the
  raw error to the `DiagnoseTurnError`-enriched string. Real, uncaught by tests,
  low real-world impact — but it contradicts the commit's "byte-identical"
  claim, so Fable should decide whether to pin it or accept it.
- Three Low findings (partial panic isolation; generic invalid-repo hint;
  conn-watcher cancels on any inbound byte).

Everything else I checked line-by-line reconciles as behavior-preserving.

## Evidence contract (how to reproduce this review yourself)

- **Base under review:** `mainline` @ `c0381bd` (my worktree was fast-forwarded
  to it from the `v2.7.0` tag `d2cda8a`; no code was modified).
- **Commits reviewed** (highest risk first):
  - T1.3 turn engine — `26ae9ef` `refactor(sidecar): extract shared ACP turn primitive; decompose askWithTurnRunner (ADR-0035 T1.3)`
  - A1 daemon supervisor — `d287013` `feat(sidecar): daemon supervisor hardening — panic isolation, context propagation, graceful drain (ADR-0036 A1)`
  - T1.1 `ghx.Repo` — `9e4a10e`; T1.2 client seam — `b972c4b`; B1 Repo-flow — `396e791`
- **Method:** `git show <sha>` for each diff; `git show 26ae9ef^:<path>` to diff
  against the pre-refactor source; read surrounding code; reason about edge
  cases, concurrency, and error paths. No live eval (behavior-preservation review).
- **Independent gates I ran (not just trusting the commits):**
  - `go build ./...` → exit 0.
  - `go test -race -count=1 ./internal/sidecar/... ./internal/ghx/... ./internal/cli/...`
    → all `ok`, no data races (sidecar 17.9s, ghx, cli, evals, tier2 all green).
  - The race detector is **green but does not clear the Medium finding**: M1 is a
    *logical* error-string change using correct synchronization, so `-race`
    cannot see it (confirmed by the fact that no test flows a failed turn through
    `askWithTurnRunner`'s deferred live-log closure — see M1).

Every claim below cites `file:line` on `mainline` @ `c0381bd`.

---

## Findings

| # | Sev | Area | One-line |
| --- | --- | --- | --- |
| M1 | Medium | T1.3 | Failed-turn **live-log** error field changed raw → `DiagnoseTurnError`-enriched via named-return + deferred closure; not byte-identical, untested |
| L1 | Low | A1 | `recover()` only catches **synchronous** dispatch panics; a panic in a turn's child goroutine still crashes the daemon (claim over-generalizes) |
| L2 | Low | T1.1/B1 | Invalid-repo hint is now generic (`e.g. ghx explore …`) for every command; read/inspect lost their command-specific examples |
| L3 | Low | A1 | `watchConnClosed` cancels the turn on **any** inbound byte (`n>0`), not only on disconnect — benign under today's synchronous protocol, latent otherwise |

No High findings.

### M1 (Medium) — Failed-turn live-log error string is no longer byte-identical

**Where:** `internal/sidecar/runtime.go:273` (new named returns), `:289–294`
(deferred live-log closure), `:305–307` (failed-turn return).

**What changed.** `askWithTurnRunner` was converted from unnamed returns
(`(*Report, *TurnResult, error)`, old `runtime.go:273`) to **named returns**
`(report *Report, result *TurnResult, err error)` (`runtime.go:273`). The
deferred live-log closure reads `err`:

```go
defer func() {
    errMsg := ""
    if err != nil { errMsg = err.Error() }
    liveLog.TurnCompleted(err == nil, errMsg, len(state.turnResult.ToolCalls), len(state.turnResult.FullText))
    liveLog.Close()
}()
...
if err != nil {
    emitFailedTurnArtifacts(ctx, state, err)              // telemetry uses RAW err — OK, matches old
    return nil, &state.turnResult, DiagnoseTurnError(fmt.Errorf("run turn: %w", err), state.stderrLog)
}
```

With **named** returns, `return …, DiagnoseTurnError(...)` **assigns** the
enriched error to `err` *before* the deferred closure runs (Go spec: result
params are set before deferred funcs execute). So the live-log `turn.completed`
error field now records the `DiagnoseTurnError` output.

The old code (unnamed returns, old `runtime.go:520`:
`return nil, &turnResult, DiagnoseTurnError(...)`) returned that value as a bare
expression — it never touched the function-local `err` the closure captured, so
the live log recorded the **raw** turn error.

**Concrete failure scenario.** A turn dies on the liveness watchdog. Old
`live.jsonl`:
`{"event":"turn.completed","ok":false,"error":"acp prompt: liveness timeout: no ACP session update for 2m0s (set GHX_… to adjust)", …}`.
New `live.jsonl` for the same event:
`{"event":"turn.completed","ok":false,"error":"run turn: acp prompt: liveness timeout: …\n\nAgent stderr (tail of /…/agent-stderr.log):\n<multi-line stderr tail>\n\nHint: <hint>", …}`
(`DiagnoseTurnError`, `internal/sidecar/agentdiag.go:515`, prepends `run turn: `
and splices the stderr tail + hints). The two strings **always** differ (minimum
divergence is the `run turn: ` prefix; maximum embeds the whole stderr tail).

**Why it slipped through.** No test flows a failed turn through
`askWithTurnRunner`; the live-log tests (`live_test.go`, `livetail_test.go`)
call `LiveLog.TurnCompleted` with hard-coded strings, and `daemon_test.go:238`
only asserts the `ok:true` case. So the suite passed "unmodified" without ever
exercising this path — the "byte-identical" proof has a blind spot here.

**Impact — honestly low, but real.** It only touches the ADR-0022.1 live
turn-log's failure event. It does **not** affect eval scoring/measurement
(`emitFailedTurnArtifacts`, `runtime.go:306`, still passes the **raw** `err` to
the OTLP telemetry `Error` field — verified) and does **not** change the error
the CLI caller sees (both old and new return `DiagnoseTurnError` to the caller).
The enriched content is arguably *more* diagnosable; the downside is a
potentially large multi-line blob inside one JSON-lines field (JSON-escaped, so
no parser break, but the `ghx` live-tail renderer will show a bulkier/garbled
failure line).

**Fix options (Fable's call):** (a) accept it and update the commit's
"byte-identical" claim to "byte-identical except the live-log failure string is
now enriched"; or (b) pin the old behavior with a one-liner — capture the raw
error for the closure, e.g. keep a `rawErr` local and have the closure read
`rawErr`, or compute `diagErr := DiagnoseTurnError(...)` and `return nil, result, diagErr`
while leaving `err` as the raw value the closure observes. Add one failed-turn
test through `askWithTurnRunner` to lock whichever is chosen.

### L1 (Low) — Panic isolation is partial (synchronous dispatch only)

**Where:** `internal/sidecar/daemon.go:248–277` (`handleRPCRequest` +
`recover()` at `:264`).

The per-request `recover()` sits on the **handler goroutine** and catches only
panics that unwind synchronously through `s.dispatch`. A panic raised in a
goroutine *spawned by* the turn — the ACP prompt goroutine (`turn.go:88–98`
`runPrompt`) or the liveness watchdog (`turn.go:19–36`) — is on a different
goroutine and will still crash the whole daemon (and every warm session),
exactly as before A1. The commit message ("one turn's panic returns a JSON-RPC
error … instead of killing the daemon") therefore over-generalizes.

I verified the recover is otherwise **safe**: it cannot swallow Go's
non-recoverable fatal conditions (concurrent map access, stack overflow, OOM),
and it logs `debug.Stack()` to stderr (`daemon.go:265`), so a recovered panic is
loud, not silent. No lock or semaphore leaks on the recovered path — see the
"verified safe" section. This is an inherent Go limitation, not a defect in the
code; flagging only because the claim reads broader than the mechanism delivers.

### L2 (Low) — Invalid-repo hint lost its per-command example

**Where:** `internal/ghx/repo.go:28` (single `ParseRepo` message), replacing
per-command messages in `explore.go`/`read.go`/`inspect.go`.

Before T1.1, `ghx read owner` and `ghx inspect owner "q"` produced
command-specific hints (`e.g. ghx read gkoreli/ghx cmd/ghx/main.go --lines 1-40`,
`e.g. ghx inspect gkoreli/ghx "exit code"`). Now every path emits the unified
`invalid repo %q: expected owner/repo, e.g. ghx explore gkoreli/ghx`. The exit-2
substring contract is unaffected (still contains `invalid repo`), so this is
purely a minor affordance regression — the fix-it example no longer matches the
command the agent actually ran.

### L3 (Low) — Conn watcher cancels on any inbound byte

**Where:** `internal/sidecar/daemon.go:279–306` (`watchConnClosed`), `:301`
`if err == nil && n > 0 { cancel(); return }`.

The watcher treats **any** byte read from the connection as a reason to cancel
the in-flight turn, not just a disconnect. Under today's synchronous
request/response protocol (`DaemonClient.call` sends one request then blocks on
the response, and pipelined bytes land in the handler's `bufio.Scanner` buffer
rather than back on the socket) this never mis-fires, so it is benign now. It is
a latent coupling to the protocol shape: if the client ever legitimately sends
mid-turn out-of-band data, the turn would be cancelled as if disconnected. Worth
a one-line comment documenting the assumption.

---

## What I verified as genuinely behavior-preserving (the "clean" part)

I traced these against the pre-refactor source and found them faithful — listing
them so the negative results are on the record, not just the findings.

**T1.3 turn primitive (`turn.go`) — both callers preserved:**
- **Watchdog** (`turn.go:19` `startACPWatchdog`) is byte-identical to both old
  inline loops (`acp.go` one-shot and `daemon_worker.go watchLiveness`): same
  idle/remaining math, same `ErrLivenessTimeout` message, same
  `ctx.Done()`/`time.After` select, correct `*atomic.Int64` + `CancelCauseFunc`
  wiring per caller.
- **Four-way prompt select** (`turn.go:100` `waitForACPPrompt`) reproduces the
  one-shot path via `ctxDone=nil, ctxErr=nil` (nil channel disables the extra
  case → old 3-way behavior; the `ctxErr` nil-guard at `turn.go:118` prevents a
  nil deref) and the warm path via `ctx.Done()/ctx.Err/turnCtx`. `peerClosedMarker`,
  `IsPeerClosedError`, `awaitPromptOrGrace` grace, and the
  `errors.Is(cause, ErrLivenessTimeout)` override all match. Prompt uses `turnCtx`
  (one-shot) vs `ctx` (warm) exactly as before.
- **Session config** (`turn.go:38` `configureACPSession`) reproduces the
  one-shot decision `LoadSession iff (ACPSessionID!="" && LoadSession-capable)`
  and the warm `sessionID==""` / reload branches; `NewSession`/`LoadSession` meta
  re-assertion, `cwd` fallback chain (`cfg.Cwd` → `os.Getwd`), `turnFailureCause`
  wrapping, and `w.sessionID` write-back are all preserved.

**T1.3 `askWithTurnRunner` decomposition:** max-turns recovery
(`recoverMaxTurns`), report-retry (`resolveReportWithRetry`), persist
(`persistTurn`), and emit (`emitArtifacts`/`emitFailedTurnArtifacts`) each match
the old inline blocks field-for-field, including the "failed wrap-up still ships
BLOCKED unless a real report arrived" precedence and the post-retry
`persistACPSessionID` ordering. I specifically checked the **double
`turnNumber()`** call (`persistTurn` then `emitArtifacts`) cannot diverge:
`RecordTurn` reads a *fresh* `meta` from disk (`session.go:149`) and
`persistACPSessionID` only writes `ACPSessionID` (`runtime.go:99`), so
`state.meta.TurnCount` is stable across the call — `turn` is identical in both.

**A1 concurrency:**
- **Panic → no lock/semaphore leak:** `AgentWorker.RunTurn` uses
  `w.mu.Lock(); defer w.mu.Unlock()` (`daemon_worker.go:135–136`); `RunnerFor`
  releases `p.mu` before returning the runner (`daemon_worker.go:44,50`); the
  `maxConcurrent` semaphore releases via defer (`:133`); `dispatch`'s
  `defer s.endAsk()` runs during unwind (`daemon.go`). All defers execute during
  panic unwinding, so a recovered panic leaves no mutex held and no
  WaitGroup/semaphore imbalanced.
- **Graceful-drain WaitGroup is race-free:** `beginAsk` does the `draining`
  check **and** `drain.Add(1)` under the same `s.mu` that `Shutdown` uses to set
  `draining` (`daemon.go:186–195, 155–158`). Every real `Add` therefore
  happens-before `Shutdown` sets `draining`, which happens-before the accept loop
  observes the closed listener, which happens-before `waitForDrain`'s `Wait()` —
  so `Wait` never races an `Add`. Rejected asks never `Add`, so the counter
  cannot go negative.
- **Request-ctx → turn-ctx bridge is teardown-safe:** the bridge goroutine
  (`cancelTurnOnRequestDoneLocked`, `daemon_worker.go:249`) parks in its
  `select` at turn start; `close(cancelBridgeDone)` happens-before the handler
  defer's `cancel(reqCtx)`, and a parked select woken by a channel *close* takes
  that specific case — so at normal turn end the bridge exits on `done` and does
  **not** spuriously cancel the shared `w.turnCtx`. `context.WithoutCancel(ctx)`
  (`daemon_worker.go:188`) correctly decouples warm-worker lifetime from any one
  request while the per-turn bridge supplies per-turn cancellation.
- **Signal-aware drain:** `Serve` spawns a `ctx.Done() → s.Shutdown()` watcher
  (`daemon.go`), so SIGTERM closes the listener → accept loop returns →
  `defer waitForDrain(30s); pool.Shutdown()`. In-flight asks return (cancelled
  and artifact-flushed on SIGTERM via the reqCtx→turnCtx bridge, or completed on
  `--stop`) before exit — a clean improvement over the old abrupt process death,
  bounded by `shutdownGrace` and capped at 30s, no deadlock. The
  `DaemonClient.call` cancel goroutine (`daemon.go`) is buffered (size 1) and
  cannot leak.

**T1.1 / T1.2 / B1:**
- **Exit-2 contract preserved and correctly extended.** Bad slug → `ParseRepo`
  error containing `invalid repo` (`repo.go:28`) → `ghxCoreError`
  (`ghx.go:767`) / inspect handler (`ghx.go:429`) → `ExitBadInvocation` (2).
  explore/read/tree moving from `upstreamError`(3) to `ghxCoreError`(2) is the
  **intended** fix (dogfood F1), not drift.
- **The `ghx tree noslash` panic is genuinely fixed:** old `fetchTree` did
  `strings.Split(repo,"/")[1]` unchecked; `Tree`/`Read` now `ParseRepo` first
  and pass a typed `Repo`, so a malformed slug returns a clean exit-2 error.
  Pinned by `TestTreeBadSlugReturnsErrorWithoutPanic`.
- **No validation bypass:** the only `Repo{…}` construction in non-test code is
  inside `ParseRepo` itself (`repo.go:30`); every `ParseRepo` call site is a
  documented edge (CLI `ghx.go`, MCP `serve.go`, codemode `register.go`,
  `Inspect`). The string→`Repo` signature change makes the compiler enforce that
  every caller parses first. No unchecked `Split(repo` remains in core.
- **Valid inputs unaffected:** for a well-formed `owner/repo`, `ParseRepo`
  yields the same owner/name the old split did. B1 is a pure type-threading; T1.2's
  default provider calls the identical `api.DefaultGraphQLClient()/DefaultRESTClient()/NewRESTClient()`
  constructors, and `RESTWithOptions` preserves the `ClientOptions` (not dropped).
  New strictness (`owner/`, `a/b/c`) only rejects already-malformed slugs.

## Per-commit verdict

| Commit | Verdict |
| --- | --- |
| `26ae9ef` T1.3 turn engine | Safe. Faithful extraction/decomposition; **one Medium** (M1) failure-path live-log deviation — accept-or-pin, Fable's call. |
| `d287013` A1 daemon supervisor | Safe. Panic isolation, context propagation, and drain are all sound under the race model; two Lows (L1 partial isolation, L3 watcher) are documentation-level. |
| `9e4a10e` T1.1 `ghx.Repo` | Safe. Real crash fixed, contract preserved; one Low (L2 generic hint). |
| `b972c4b` T1.2 client seam | Safe. Production path unchanged; options preserved; test override is cleanup-guarded. |
| `396e791` B1 Repo flow | Safe. Parse-at-the-edge with no bypass; exit-2 contract intact. |

## Overall verdict

**These changes are safe as landed.** I found no correctness, concurrency, or
error-path defect that warrants reverting or blocking. The single Medium (M1) is
a genuine, low-impact deviation from the "byte-identical" claim on the failed-turn
live-log string that tests do not cover — Fable should consciously accept it (and
soften the claim) or pin it with a one-liner + a failed-turn test. The three Lows
are documentation/affordance nits. The `-race` suite is green across all touched
packages, and the highest-risk item (T1.3) reconciles line-by-line with the
pre-refactor source on every safety-relevant path except the one noted.
