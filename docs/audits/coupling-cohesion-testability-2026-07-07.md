---
title: "Coupling / Cohesion / Testability Audit — ghx"
date: "2026-07-07"
status: "audit"
author: "background audit worker"
scope: "internal/**, cmd/** — read-only audit, no code changes"
lens: "coupling, cohesion & testability"
---

# Coupling / Cohesion / Testability Audit — 2026-07-07

Read-only, adversarial audit of the ghx codebase through one lens: **is ghx
harder to change and test than it should be, and can we prove it with
`file:line` evidence?** Every finding cites lines verified against the working
tree; `go build ./...` succeeds at audit time (no code was modified). Fable
decides what to act on; this report finds, cites, and ranks by impact on
engineering velocity — which coupling, if cut, most increases our ability to
change ghx safely and fast.

Companion audit: `docs/audits/architecture-2026-07-07.md` covered the
core/frontend boundary (callTool compat cruft H1, the `acp.go` god-file H2 —
since split into `denyclient.go`/`turnresult.go`/etc., presentation-ownership
M1, error-taxonomy M2, eval god-files M3). **This audit deliberately does not
re-litigate those findings**; where a coupling finding here touches error
taxonomy or presentation ownership, it cross-references rather than repeats.

Tenets audited against (`AGENTS.md` **Engineering Tenets**):

- **"Encapsulate domain logic in services. One authoritative owner per domain
  concern … construct it once and inject/share it rather than scattering the
  same responsibility across packages. Decouple code whose responsibilities do
  not belong together; a caller should not inherit a dependency it never
  uses."**
- **"Refactor cleanly or not at all. A refactor that leaves both the old and
  new pattern in the tree is worse than no refactor: it doubles the surface
  every future agent must understand."**
- **"Define proper domain models … If a concept appears in two places, it
  deserves a type."** and **"Declarative, documented APIs."**
- The **Visibility and Truthfulness** tenet (every score recomputable; the
  eval turn path must faithfully mirror the product turn path) — invoked where
  a duplication threatens measurement fidelity.

## Executive summary

**The single highest-leverage testability gap is that the product's core value
— GitHub reconnaissance — has no dependency-injection seam: every core
operation constructs its own GitHub client inline (`api.DefaultGraphQLClient()`
/ `api.NewRESTClient()` at `internal/ghx/explore.go:32`, `read.go:93`,
`repos.go:35`, `glob.go:22`, `search.go:40`), so not one of `Explore`,
`Search`, `Repos`, `Read`, or `Tree` can be unit-tested without live GitHub
auth and network.** The only tests in `internal/ghx` cover the *pure*
sub-helpers the code already factored out (`parseFileResponse`, `IsGlob`,
`ExpandGlobs`, ranker features) — the query construction, pagination, and
response→domain mapping that is the actual recon logic is exercised only by
hitting the real API. Cutting one seam (inject a tiny `GraphQLDoer`) unlocks
regression tests for the thing that matters most, which is why it ranks first.

**The second-highest is a dual-code-path in the most safety-critical code in
the tree:** the ACP "run one turn" conversation is implemented twice in
production — once in `internal/sidecar/acp.go:218` (`RunTurnWithOptions`, the
one-shot daemonless path) and once in `internal/sidecar/daemon_worker.go:125`
(`AgentWorker.RunTurn` + `ensureStarted`/`configureSession`/`prompt`, the warm
path) — duplicating the liveness watchdog, the four-way prompt `select` loop,
and the NewSession/LoadSession session-meta re-assertion nearly line-for-line.
Two more hand-rolled copies of the same ACP handshake live in the eval runner
(`evals/runner.go:74,385`, `evals/closedbook.go:115`). Every ADR-0027
resilience or ADR-0020.2 session-meta change must be made in ≥2 places and kept
in sync by hand — exactly the "leaving both patterns doubles the surface"
anti-pattern, in the code where drift is most dangerous.

The remaining findings are ambient globals that force serial tests (the
storage-root env read), a scattered wall-clock with no injected seam despite
the pattern already existing two files over, and a digest function with a
hidden env input. All are real velocity taxes; none is a boundary break.

### What is clean (verified, not assumed)

- **The runtime has a real turn-runner seam.** `TurnRunner`
  (`internal/sidecar/runtime.go:20`) is a function type; the daemon injects the
  warm worker (`daemon_worker.go:44`) and daemonless calls inject
  `runTurnWithOptions` (`runtime.go:15,261`), which a test can stub via the
  package var. So the *runtime* is testable with a fake runner — the
  duplication finding (H2) is a maintenance/drift cost, not a runtime-test gap.
- **The route engine already injects its clock.** `RouteConfig.Now func()
  time.Time` (`route.go:109`, default `route.go:121`) and the judge's
  `now func() time.Time` (`evals/judge.go:105`) prove the injectable-clock
  pattern is known and accepted here — which is what makes the 67 bare
  `time.Now()` sites (M2) an *inconsistency*, not an unavoidable cost.
- **Worker concurrency is disciplined.** `AgentPool`/`AgentWorker`
  (`daemon_worker.go:18-108`) guard all shared maps and worker state under one
  `sync.Mutex` each, serialize turns per session, and bound global concurrency
  with a `maxConcurrent chan struct{}` (cap 4, `daemon_worker.go:30,126`). The
  `*Locked` method naming convention marks the lock contract. No obvious data
  race in the pool.
- **Response parsing is already split out for testing.** `read.go` factors
  `parseFileResponse` into a pure function with 14 table tests
  (`read_test.go`); the ranker/formatter helpers in `inspect.go` are likewise
  pure and tested. The core team knows how to make a seam — H1 is that they
  stopped one layer short (the client itself).

---

## High severity

### H1 — Core reconnaissance has no client-injection seam: the product's core value is untestable in isolation

**What.** Every core recon operation constructs its own GitHub transport inside
the function body, so there is no way to substitute a fake and exercise the
query-building / pagination / response-mapping logic without live network and
GitHub credentials.

**Evidence.**
- `internal/ghx/explore.go:32` — `gql, err := api.DefaultGraphQLClient()`
  inside `Explore`, then `gql.Do(...)` at `:81` and `:130`.
- `internal/ghx/read.go:93` — same, inside `Read`; `gql.Do` at `:119`.
- `internal/ghx/repos.go:35` — same, inside `Repos`; `gql.Do` at `:75`.
- `internal/ghx/glob.go:22` — same, inside `expandGlobs`; `gql.Do` at `:41`.
- `internal/ghx/search.go:40` — `rest, err := api.NewRESTClient(...)` inside
  `Search`.
- The exported functions take **no client parameter**: `func Explore(repo,
  path string)`, `func Read(repo, files, *ReadOpts)`, `func Repos(query,
  ReposOpts)`, `func Search(query, SearchOpts)`, `func Tree(...)`
  (`register.go:91-160` shows every `wrap*` calling them positionally).
- Coverage proof: `internal/ghx/*_test.go` is `glob_test.go`,
  `inspect_test.go`, `read_test.go` — **all** assert on pure helpers
  (`TestParseFileResponse_*`, `TestIsGlob`, `TestExpandGlobs*`,
  `TestInspectRanker*`, `TestFormatInspectText*`). There is **no**
  `TestExplore`, `TestSearch`, `TestRepos`, no `httptest.Server`, no fake
  `Do`; grep for `httptest|mock|fake` over `internal/ghx/*_test.go` returns
  nothing.

**Why it violates a tenet.** AGENTS.md Engineering Tenets: *"Encapsulate domain
logic in services … construct it once and inject/share it rather than
scattering the same responsibility across packages."* The GitHub client is a
service dependency; scattering `DefaultGraphQLClient()` across five operations
is the opposite of constructing-once-and-injecting. The direct consequence is
that the highest-value logic in the repo — the exact GraphQL a recon call
issues and how it maps the response into `ExploreResult`/`ReadResult` — is
change-unsafe: a refactor of pagination or the README-selection expression in
`explore.go` cannot be regression-guarded except by an integration run against
live GitHub. That is the biggest single brake on "change ghx safely and fast."

**Recommendation.** Introduce a one-method seam and thread it through. `go-gh`
already exposes the surface: define
`type graphQLDoer interface { Do(query string, vars map[string]any, resp any) error }`
in `internal/ghx`, default it to `api.DefaultGraphQLClient()` at the CLI/MCP
composition root (or via a package-level constructor `NewClient()` that the
`wrap*` functions hold), and give `Explore`/`Read`/`Repos`/`Glob` that doer
(REST likewise via a tiny `restDoer`). The existing pure `parseFileResponse`
tests then gain sibling end-to-end tests feeding canned GraphQL JSON through a
fake doer — no network, deterministic, and finally covering the query→domain
mapping. Small, mechanical, and the payoff is the largest here.

### H2 — The ACP "run one turn" conversation is implemented twice in production (plus twice more in evals): the highest-drift-risk dual path in the tree

**What.** "Establish an ACP session and run one prompt" is not owned by a
single service. Two production implementations duplicate the full
resilience + session-meta + prompt-select logic, differing only in *connection
lifecycle* (one-shot vs warm-reused). Two more copies of the same handshake
live in the eval runner.

**Evidence — the two production engines.**
- One-shot: `internal/sidecar/acp.go:218` `RunTurnWithOptions` —
  `exec.CommandContext` (`:224`), `denyClient` + `NewClientSideConnection`
  (`:262-263`), inline liveness goroutine (`:265-282`), NewSession/LoadSession
  with full `SessionMeta` re-assertion (`:312-346`), four-way prompt `select`
  over `promptDone`/`conn.Done()`/`turnCtx.Done()` (`:359-389`).
- Warm: `internal/sidecar/daemon_worker.go:125` `AgentWorker.RunTurn`, split
  across `ensureStarted` (`:175` — `exec.CommandContext` `:185`, `denyClient` +
  `NewClientSideConnection` `:219-220`), `watchLiveness` (`:355-370` — the same
  idle-since-`lastActivity` algorithm as `acp.go:265-282`), `configureSession`
  (`:241-294` — the same NewSession/LoadSession + `SessionMeta` re-assertion as
  `acp.go:312-346`, comments copied), and `prompt` (`:296-341` — the same
  four-way `select` as `acp.go:359-389`, including the identical
  `awaitPromptOrGrace`/`peerClosedMarker` handling).
- Both are live and reachable: daemonless `Ask` → `askWithTurnRunner(...,
  runTurnWithOptions, ...)` (`runtime.go:261`); daemon `Ask` →
  `AskWithTurnRunner(..., runner, ...)` with `runner = pool.RunnerFor(...)` →
  `AgentWorker.RunTurn` (`daemon.go:216-217`, `daemon_worker.go:44`).

**Evidence — two further hand-rolled copies of the same handshake in evals.**
- `evals/runner.go:74` `ProbeAgentIdentity` and `:385` `runLiveAgentEpisode`
  each redo `exec.CommandContext` → `NewClientSideConnection` →
  `conn.Initialize` (identical `ClientCapabilities{Fs:{false,false}}`) →
  `NewSession` → `Prompt` (`:76-95`, `:405-437`).
- `evals/closedbook.go:115-165` — the same handshake again for the closed-book
  baseline.

**Why it violates a tenet.** AGENTS.md: *"Refactor cleanly or not at all. A
refactor that leaves both the old and new pattern in the tree is worse than no
refactor: it doubles the surface every future agent must understand,"* and
*"One authoritative owner per domain concern."* "One ACP turn" is a single
domain concern with no owner. The two production copies are the sharpest case:
they are *meant* to behave identically (same resilience taxonomy ADR-0027, same
meta re-assertion ADR-0020.2) and differ only in whether the process/connection
persists — yet the liveness watchdog, the peer-death handling, and the meta
logic are maintained in parallel. A resilience fix applied to `acp.go` and
missed in `daemon_worker.go` (or vice-versa) is a silent divergence in the
sidecar's busiest, most safety-critical code. The eval copies additionally
risk the **Visibility and Truthfulness** tenet: the measured turn path and the
product turn path share no transport primitive, so a handshake/SDK change can
make evals exercise a subtly different conversation than production ships.

**Recommendation.** Extract one `acpConversation` primitive owning the wire
lifecycle: connect (`exec` + `NewClientSideConnection`), `initialize`,
`newSession`/`loadSession(meta, mcpServers)`, `prompt` (with the liveness
watchdog and the four-way `select` as its single implementation), and
`shutdown`. The one-shot path constructs it per turn and closes it; the warm
`AgentWorker` holds it across turns (the *only* real difference). Both
production engines collapse to that engine + a lifecycle policy. Eval probes
reuse the transport/handshake portion (they legitimately differ in
session-shaping — no report sink, raw-SDK audit only — but not in the wire
plumbing). Behaviour-preserving and verifiable by `go test ./...`.

---

## Medium severity

### M1 — Ambient storage-root global: `rootDir()` reads process env at every path call, splitting ownership with `Config` and forcing serial tests

**What.** The single storage root is not a constructed, injected value — it is
a package-level function that re-reads `os.Getenv` on every call, so dozens of
path resolutions depend on ambient process state. Ownership is also split:
`SessionsDir` *is* a `Config` field (injected), but the daemon's runtime paths
bypass `Config` and read env directly.

**Evidence.**
- `internal/sidecar/config.go:17-22` — `rootDir()` returns
  `os.Getenv("GHX_HOME")` or `filepath.Join(os.Getenv("HOME"), ".ghx")`.
- Every path helper resolves through it fresh, not through `Config`:
  `RootDir()` (`:133`), `RuntimeDir()` (`:136`), `ConfigFilePath()` (`:142`),
  and in the daemon `SocketPath()`/`MetadataPath()`
  (`daemon.go:260,263`). `NewDefaultConfig`/`LoadConfig`/`SaveConfig` all call
  `rootDir()` (`:129,147,234`).
- Split ownership: `Config.SessionsDir` is a real injected field
  (`config.go:51`) threaded as `cfg.SessionsDir` (`daemon.go:214,227,239,250`),
  yet `RuntimeDir()`/`SocketPath()` ignore `Config` and re-read env — so "where
  does ghx store things" has two answers (a `Config` field for sessions, an
  ambient global for runtime).
- Test cost: 16 `Setenv("GHX_HOME"|"HOME")` sites across
  `internal/cli/sidecar_test.go`, `cli/tier2_test.go`,
  `sidecar/config_test.go`, `sidecar/daemon_test.go`. `t.Setenv` is
  incompatible with `t.Parallel()`, so every storage-touching test is pinned
  serial and mutates global process state.

**Why it violates a tenet.** AGENTS.md: *"One authoritative owner per domain
concern … construct it once and inject/share it."* The storage root is one
concern with two owners (a `Config` field and an env-reading global). The
ambient global is the classic hidden dependency: a function's behaviour depends
on `os.Environ` rather than its arguments, which is why the tests must reach for
`t.Setenv` and give up parallelism.

**Recommendation.** Make the root a field on `Config` (`Config.Home`, resolved
once by `LoadConfig`/`NewDefaultConfig` from `GHX_HOME`/`HOME`) and derive
`RuntimeDir`/`SocketPath`/`MetadataPath`/`ConfigFilePath` as methods on `Config`
(or a small `Paths` value built from it). Env is read exactly once at the
composition root; tests construct a `Config` with a `t.TempDir()` home and can
run in parallel. This also closes the split-ownership gap with `SessionsDir`.

### M2 — Wall-clock is a scattered ambient dependency (67 `time.Now()` sites) with no injected seam, despite the pattern already existing

**What.** Time-dependent logic across the sidecar reads `time.Now()` directly
instead of an injected clock, so behaviour that depends on *when* it runs — idle
TTL expiry, the liveness watchdog, session-ID minting — has no deterministic
test seam. The inconsistency is the smell: two files already inject a clock.

**Evidence.**
- 67 non-test `time.Now()` calls across `internal/{sidecar,codemode}`; the
  behaviourally load-bearing ones:
  - Idle-worker TTL: `daemon_worker.go:217-218,357` (`lastActivity` +
    `watchLiveness` compare against `time.Now().UnixNano()`); the same in the
    one-shot path `acp.go:253-254,268`.
  - Session-ID minting embeds wall-clock: `session.go:165,214`
    (`time.Now().UnixMilli()`), `evals/runner.go:128,167`,
    `evals/host_episode.go:234` — IDs are therefore non-reproducible and
    order-sensitive across fast successive calls.
- The seam already exists elsewhere: `route.go:109` `Now func() time.Time`
  (default `:121`) and `evals/judge.go:105` `now func() time.Time` (default
  `:117`) inject the clock for deterministic tests — so the codebase has
  *decided* injectable clocks are the right pattern, then applied it in only
  two of the many places that need it.

**Why it violates a tenet.** AGENTS.md: *"Composition or inheritance by use
case … chosen for the actual shape of the problem"* and the testability spirit
of *"encapsulate … inject."* A watchdog whose window is measured against a
hidden global clock cannot be unit-tested without real sleeps; the route engine
proves the alternative. Divergent treatment of the same dependency (injected in
`route.go`, ambient everywhere else) is the "two patterns in the tree" cost the
refactor tenet warns about.

**Recommendation.** Where time is behaviourally load-bearing — the worker TTL /
liveness watchdog and session-ID minting — thread the existing `Now func()
time.Time` seam (reuse the `route.go`/`judge.go` shape) so tests drive expiry
and ID generation with a fake clock. Leave purely-observational
`time.Now().UTC()` timestamps (OTel span starts, `StartedAt`/`EndedAt` audit
fields) alone — injecting there is dogma, not use-case. This is a targeted
consistency pass, not a sweep of all 67 sites.

### M3 — Hidden ambient input in a pure-looking digest, and a dropped request context in the daemon dispatch

**What.** Two smaller temporal/ambient couplings in the daemon layer: a
function that presents as a pure function of its `Config` argument but secretly
reads process env, and an RPC dispatch that discards the caller's context so
cancellation never reaches turn execution.

**Evidence.**
- `daemon.go:267-295` `ConfigDigest(cfg Config)` — its signature promises a
  digest of the passed `Config`, but line `:292` folds
  `os.Getenv("GHX_REPORT_SINK_EXE")` into the hashed struct. Two identical
  `Config` values therefore produce different digests depending on ambient
  env, and the daemon's warm/stale decision (`ensure`, `:466,483`) silently
  depends on the env of whoever last called it. The comment (`:273-278`)
  justifies *including* the sink but not the *hidden* channel — an argument-shaped
  input arriving through env.
- `daemon.go:192` — `s.dispatch(context.Background(), req)`: the per-connection
  handler manufactures a fresh background context instead of one tied to the
  socket connection, so a client that disconnects or cancels mid-`Ask` cannot
  propagate cancellation into `AskWithTurnRunner`/the turn. The 12-hour conn
  deadline (`:519`) is the only backstop. This is temporal coupling: turn
  lifetime is decoupled from request lifetime.

**Why it violates a tenet.** AGENTS.md: *"Declarative, documented APIs"* and
*"Define proper domain models."* A digest of `Config` that also depends on env
is an undeclared dependency — the function is not what its signature says.
Dropping the request context couples turn cancellation to nothing, which is the
kind of hidden temporal edge that makes daemon behaviour hard to reason about
and test (you cannot write "client cancels, turn stops" because the wire never
carries it).

**Recommendation.** Make `GHX_REPORT_SINK_EXE` an explicit field resolved into
`Config` at the composition root (it already partly is via
`reportsink_exe.go:21`), then hash only `cfg` — the digest becomes a true pure
function of its argument, testable with two `Config` literals. Separately,
derive the dispatch context from the connection (a `context.WithCancel` closed
when `handleConn` returns) so client disconnect cancels in-flight work; keep the
long deadline as an upper bound.

---

## Low severity

### L1 — CLI is intimate with sidecar domain-type internals (`Report`, `SessionMeta`) with no presenter or test seam

**What.** The CLI hand-renders sidecar domain types field-by-field, reaching
into their internal layout, even though the sidecar package already owns
presenter functions for its *other* types. This is a narrower, coupling-lens
instance of the presentation-ownership inconsistency the architecture audit
raised as M1 — flagged here only for the *intimacy + missing test seam*, not to
re-litigate that recommendation.

**Evidence.**
- `internal/cli/sidecar.go:377-400` — the `show` command prints ~12
  `meta.<Field>` values one by one (`meta.Name`, `meta.Repo`, `meta.Scope`,
  `meta.TurnCount`, `meta.CreatedAt`, `meta.ACPSessionID`, `meta.AgentCmd`,
  `meta.Cwd`, `meta.SpawnCwd`, `meta.AgentEnv`, …). A rename of any
  `SessionMeta` field breaks the CLI.
- `internal/cli/sidecar.go:718-721` — the answer renderer reaches into
  `report.Answer`, `report.Verified`, `report.RelevantFiles`,
  `report.Uncertainty` directly.
- The precedent for the other direction already exists in the sidecar package:
  `FormatConfig` (`config.go:282`), `FormatPreflight`, `FormatInspectText`
  (core) — so `Report`/`SessionMeta` are the outliers with no `FormatReport`/
  `FormatSessionMeta`, and therefore no unit test of their human rendering.

**Why it's Low.** It overlaps the architecture audit's M1 direction (route CLI
output through named `Format*` presenters) and is a comprehension/testability
cost rather than a correctness risk. The distinct point for this lens: a
frontend reaching into a domain type's field layout is inappropriate intimacy,
and the absence of a presenter means the rendering has no test seam.

**Recommendation.** Add `FormatSessionMeta(meta) string` and
`FormatReport(report) string` in the sidecar package (matching
`FormatConfig`/`FormatPreflight`), have `show`/`ask` call them, and unit-test
the presenters. Folds cleanly into whatever action lands architecture-audit M1.

---

## Recommended sequence (top refactors, ordered)

1. **Give core recon a client-injection seam (H1).** Add `graphQLDoer`/`restDoer`
   interfaces in `internal/ghx`, inject at the composition root, and add
   end-to-end tests feeding canned API JSON through fakes. Smallest change with
   the largest payoff: the product's core logic becomes regression-testable
   without network. Do this first — it de-risks every later change to recon.
2. **Unify the ACP turn engine (H2).** Extract one `acpConversation` primitive;
   fold `acp.go RunTurnWithOptions` and `daemon_worker.AgentWorker` onto it as
   one-shot vs warm lifecycle policies; reuse its transport in the eval probes.
   Removes the highest-drift dual path in the safety-critical turn code and
   protects eval/product fidelity. Verify with `go test ./...`.
3. **Make the storage root injected, not ambient (M1).** Resolve `GHX_HOME`/
   `HOME` once into `Config.Home`; derive runtime/socket/metadata/config paths
   from it; drop the `t.Setenv` serialization and close the split ownership with
   `SessionsDir`. Unblocks parallel storage tests.
4. **Thread the existing clock seam through the load-bearing time logic (M2).**
   Inject `Now func() time.Time` (the `route.go`/`judge.go` shape) into the
   worker TTL/liveness watchdog and session-ID minting so time-dependent
   behaviour is deterministically testable. Targeted, not a 67-site sweep.
5. **Close the two daemon hidden-dependency edges (M3).** Fold
   `GHX_REPORT_SINK_EXE` into `Config` so `ConfigDigest` is pure; derive the
   dispatch context from the connection so client cancellation propagates.
6. **(Folds into arch-audit M1) Add `FormatReport`/`FormatSessionMeta`
   presenters (L1)** so the CLI stops reaching into sidecar domain fields and
   the rendering gains a test seam.

## Method / auditability

- Ambient-dependency inventory from
  `grep -rn 'os\.Getenv|time\.Now()|exec\.Command|http\.|net\.Dial'
  internal cmd --include='*.go' | grep -v _test.go`, then each hit read in
  context to separate load-bearing dependencies from observational ones.
- Client-seam gap (H1) confirmed by reading every `internal/ghx/*.go` operation
  for a client parameter (none) and every `internal/ghx/*_test.go` for a fake
  transport (`grep httptest|mock|fake` → empty).
- Turn-engine duplication (H2) confirmed by reading `acp.go:218-399`,
  `daemon_worker.go:125-408`, `evals/runner.go:74-95,385-437`,
  `evals/closedbook.go:115-165` side by side, and tracing both production
  callers through `runtime.go`/`daemon.go`.
- Ambient-root blast radius (M1) from `grep -rn
  'RootDir()|RuntimeDir()|SocketPath()|rootDir()'` (18 non-test callers) and
  `grep -rn 'Setenv("GHX_HOME"|"HOME")' --include='*_test.go'` (16 sites).
- Clock inconsistency (M2) from the 67 `time.Now()` hits contrasted with the
  injected `Now func()` seams at `route.go:109` and `evals/judge.go:105`.
- Every `file:line` above was spot-checked against the working tree; `go build
  ./...` exits 0 at audit time and no code was modified by this audit.
