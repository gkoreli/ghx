---
title: "Architecture Audit — ghx core/frontend boundary vs. tenets & ADRs"
date: "2026-07-07"
status: "audit"
author: "background audit worker"
scope: "internal/**, cmd/**, skills/** — read-only audit, no code changes"
---

# Architecture Audit — 2026-07-07

Read-only audit of the ghx codebase against its own tenets and ADRs. Every
finding cites `file:line`. Fable decides what to act on; this report only
finds and ranks.

Tenets/ADRs audited against:

- `docs/NORTH_STAR.md` tenet **"Core capabilities live in `internal/ghx`"**
  and **ADR-0010** invariant: core defines operations; CLI, MCP, codemode,
  sidecar **wrap** the same core, never reimplement it.
- `AGENTS.md` **Engineering Tenets**: domain models, service encapsulation
  (one authoritative owner per concern), composition-by-use-case, godoc, and
  **"No legacy maintenance, no compatibility junk"** (dead shims, dual code
  paths, "for backwards compatibility" branches).

## Executive summary

**The core/frontend boundary is genuinely healthy — the highest-leverage fix
is not a boundary repair but the removal of one specific piece of compat
cruft that the tenets explicitly forbid: the dead `callTool` backward-compat
binding in the codemode executor (`internal/codemode/executor.go:180-196`),
which is still advertised to agents in the MCP `code` tool's usage string
(`internal/cli/serve.go:348`) even though ADR-0010 declares `codemode.*` the
sole API surface.** It is the only finding that (a) directly violates a named
binding tenet ("no compatibility junk"), (b) has zero in-tree production
callers, and (c) leaks into an agent-facing contract, contradicting ADR-0010's
own type-stub/`codemode.explore` example. It is small, self-contained, and
test-covered, so removal is low-risk and clearly correct.

The important positive result of this audit: **no core-logic leakage or
dependency-direction violations were found.** Core (`internal/ghx`) imports no
frontend and no telemetry/config; the CLI is the sole composition root; the
sidecar does not reimplement reconnaissance (it spawns an external ACP agent
that shells out to the `ghx` CLI). The remaining findings are god-files,
formatting-ownership inconsistency, and doc/telemetry gaps — real, but lower
leverage than the boundary would have been had it been broken.

### What is clean (verified, not assumed)

- **Core imports no frontend.** `grep` for `internal/{cli,codemode,sidecar}`
  inside `internal/ghx/*.go` (non-test) returns empty — core has zero
  upward dependencies.
- **CLI is the single composition root.** Only `internal/cli` imports
  `codemode`, `ghx`, `sidecar`, `sidecar/evals`, `sidecar/tier2`. No frontend
  imports another frontend.
- **CLI and MCP share one executor + one tool registry.** `ghx code`
  (`internal/cli/code.go:33-56`) and MCP `code` (`internal/cli/serve.go:326-362`)
  both call `codemode.NewRegistry()` → `ghx.RegisterTools(reg)` →
  `codemode.NewExecutor().Execute()`. ADR-0010's CLI-first invariant holds.
- **MCP direct tools are thin wrappers over core.** `handleExplore`,
  `handleRepos`, `handleSearch`, `handleRead`, `handleTree`
  (`internal/cli/serve.go:210-324`) each just parse args and call
  `ghxlib.Explore/Repos/Search/Read/Tree`. No recon logic in the frontend.
- **Sidecar does not reimplement recon.** `internal/sidecar` never imports
  `internal/ghx`; it spawns an external ACP agent (`exec.CommandContext`,
  `internal/sidecar/daemon_worker.go:185`) that runs the `ghx` binary. The
  only GitHub call in the sidecar is a health HEAD probe
  (`internal/sidecar/preflight.go:174`), not reconnaissance.
- **Config and telemetry are single-owner.** `Config`/`LoadConfig` exist once
  (`internal/sidecar/config.go:43,146`); all OTel emission lives in
  `internal/sidecar/telemetry/*` (+ the `emit.go` writer). Core is
  telemetry/config-free.
- **The `~/.ghx-sidecar` fallback flagged in AGENTS.md is already gone**
  (`internal/sidecar/config.go:15-16` documents its 2026-07-05 removal —
  "exactly one path with no legacy shim"). Good.

---

## High severity

### H1 — Dead `callTool` backward-compat binding, still advertised to agents

**What.** The codemode executor injects two parallel tool-invocation APIs into
the goja VM: the ADR-0010 `codemode.<tool>(args)` object *and* a legacy
`callTool(name, args)` binding explicitly labeled "backward compatibility."

**Evidence.**
- `internal/codemode/executor.go:180-196` — `// Inject callTool binding
  (backward compatibility)` followed by `vm.Set("callTool", ...)`. The
  intended API (`codemode` object) is injected right after at lines 198-217.
- `internal/cli/serve.go:348` — the MCP `search_tools` response still tells
  agents: `"usage": "Pass code to code tool. Use callTool(name, args) to
  invoke tools."` — i.e. the dead API is *advertised* as the recommended one,
  while the `code` tool's own description example uses `codemode.explore(...)`
  (`internal/cli/serve.go:138`). The two agent-facing strings disagree.
- No in-tree production caller uses `callTool`: `grep` finds it only in the
  executor definition, the `serve.go:348` usage string, and
  `internal/codemode/executor_test.go` (tests that exist *only* to keep the
  compat path alive, e.g. `TestExecute_CodemodeAndCallToolBackwardCompat`,
  executor_test.go:275).

**Why it violates a tenet/ADR.** AGENTS.md Engineering Tenets, "No legacy
maintenance, no compatibility junk": *"do not keep deprecated shims, dual code
paths … or 'for backwards compatibility' branches. … We are the only consumers
of this product today."* ADR-0010 is explicit that the executor must inject a
`codemode` object *"(not bare `callTool`)"* (ADR-0010 §"The `codemode` Object
Injection") and lists `codemode.toolName(args)` as the single LLM API surface.
The `callTool` path is exactly the dual-code-path the tenet forbids, and it has
leaked into the agent contract (serve.go:348), degrading signal-per-token in
the main agent's context (NORTH_STAR: "the main agent's context is sacred").

**Recommendation.** Delete the `callTool` binding (executor.go:180-196), drop
the compat-only tests, and fix the `search_tools` usage string
(serve.go:348) to describe `codemode.<tool>(args)` — aligning it with the
`code` tool description and ADR-0010. Net: one API, one story to the agent.

### H2 — `internal/sidecar/acp.go` is a multi-concern god-file (1012 lines)

**What.** `acp.go` bundles at least five distinct responsibilities in one
file: the `TurnResult` domain model, the ACP `denyClient` protocol adapter,
tool-call trace parsing/summarization, report-sink executable resolution, and
turn-running orchestration (`RunTurn`/`RunTurnWithOptions`).

**Evidence** (all `internal/sidecar/acp.go`):
- `type TurnResult struct` (:20), `type ToolCallTrace struct` (:213) — domain
  models.
- `type denyClient struct` (:281) and its ~15 ACP interface methods
  (`RequestPermission` :464, `SessionUpdate` :497, `WriteTextFile` :593,
  `CreateTerminal` :604, …) — the protocol adapter.
- `resolveToolInput`/`commandWithArgs`/`toolSummary` (:375, :406, :356) —
  trace parsing.
- `ResolveReportSinkExe`/`reportSinkMcpServers` (:693, :721) — report-sink
  wiring, a different concern entirely.
- `RunTurn`/`RunTurnWithOptions` (:746, :759) — orchestration.

**Why it violates a tenet.** AGENTS.md Engineering Tenets: "Encapsulate domain
logic in services. One authoritative owner per domain concern … Decouple code
whose responsibilities do not belong together." A 1000-line file mixing domain
types, a wire adapter, a parser, and an orchestrator is the anti-pattern the
tenet names. It also raises the per-agent comprehension cost the "Refactor
cleanly" tenet warns about.

**Recommendation.** Split along the existing seams: `turn.go` (TurnResult +
RunTurn orchestration), `acp_client.go` (denyClient adapter + its interface
methods), `tooltrace.go` (ToolCallTrace + parsing/summarization), and fold
report-sink resolution into the existing `reportsink.go`. No behavior change —
pure decomposition, verifiable by `go test ./...`.

---

## Medium severity

### M1 — Inconsistent presentation-ownership across CLI commands

**What.** Some human-readable renderers live in **core** and are shared; others
are hand-inlined in the CLI `RunE` bodies. The boundary for "who owns output
formatting" is not consistent.

**Evidence.**
- `inspect` uses a core formatter: `internal/cli/ghx.go:420` calls
  `ghxlib.FormatInspectText(result)` (formatter lives in `internal/ghx`).
- `explore` inlines its full-mode and compact rendering directly in the CLI:
  `internal/cli/ghx.go:106-131` (inline `fmt.Printf` tree walk) plus the
  bespoke `printCompactExplore` helper (`internal/cli/ghx.go:654`).
- MCP, meanwhile, emits raw `json.Marshal` for every direct tool
  (`internal/cli/serve.go:223,244,265,304,322`) — a third rendering strategy.

**Why it matters.** AGENTS.md "Encapsulate domain logic in services / one
authoritative owner." Whether a presenter belongs in core is debatable (text
rendering is arguably a CLI concern), but the *inconsistency* is the smell:
`FormatInspectText` in core sets a precedent that `explore`/`search`/`read`
don't follow, so a future agent can't predict where a formatter lives. This is
a boundary-clarity issue, not a leak — ranked Medium.

**Recommendation.** Pick one rule and apply it uniformly. Preferred: a small
`internal/cli/present` (or per-result `Format*Text` on the core result types,
matching the `FormatInspectText` precedent) so every CLI command renders
through named formatters instead of inline `fmt.Printf` blocks. Keep MCP on
`json.Marshal` (machine surface) deliberately, and document that split.

### M2 — Divergent error models across frontends, no shared taxonomy

**What.** Each frontend invents its own error-surfacing model with no shared
vocabulary mapping a core failure to a frontend outcome.

**Evidence.**
- CLI: semantic exit codes via `ExitError`/`WithExitCode`
  (`internal/cli/errors.go:5-56`), e.g. `WithExitCode(ExitUpstreamFailure, err)`
  at `internal/cli/ghx.go:100`.
- MCP: every handler collapses errors to `mcp.NewToolResultError(err.Error())`
  (`internal/cli/serve.go:174,192,213,220,…`) — no distinction between
  bad-invocation, no-results, and upstream failure that the CLI carefully
  separates.
- Sidecar: 78 ad-hoc `fmt.Errorf` sites (`internal/sidecar/*.go`) plus its own
  BLOCKED/report semantics — a third model.

**Why it matters.** AGENTS.md "Define proper domain models … If a concept
appears in two places, it deserves a type." Failure-class (bad-invocation /
no-results / upstream-failure) is a domain concept that the CLI models
(`errors.go`) but core does not export, so MCP cannot reuse it and flattens
everything to a string. An agent consuming MCP loses the actionable signal the
CLI exit codes carry — which is exactly the "errors as agent affordances"
direction ADR-0028.1/A4 wants.

**Recommendation.** Lift the failure taxonomy into core (e.g.
`ghx.ErrKind`/typed sentinel errors on the core result path), have the CLI map
it to exit codes and MCP map it to structured tool-error payloads. One source
of truth for "what kind of failure this is," two renderings.

### M3 — Second tier of god-files in `internal/sidecar/evals`

**What.** Beyond acp.go (H2), several eval files exceed ~900 lines and mix
concerns: `discovery.go` (1007), `anticipation_predictor.go` (974),
`gates.go` (920).

**Evidence.** `wc -l internal/sidecar/evals/*.go`:
`discovery.go:1007`, `anticipation_predictor.go:974`, `gates.go:920`,
`rewards.go:625`, `gate_reducers.go:545`.

**Why it matters.** Same "one owner per concern" tenet as H2. Ranked Medium not
High because these are eval-internal (not the product runtime or the
core/frontend boundary) **and** are guarded by the frozen-measurement-stack
rule (AGENTS.md: eval scoring changes must be pre-registered in an ADR) — so a
casual split is *not* free here and must not be done as drive-by refactoring.

**Recommendation.** Defer unless an eval ADR already touches these files.
When one does, decompose `gates.go`/`rewards.go` by gate/reward family. Do
**not** refactor the scoring stack outside a pre-registered ADR
(AGENTS.md "Delegated Workers": measurement changes must be pre-registered).

---

## Low severity

### L1 — NORTH_STAR "ghx invocations emit OTel traces" is unmet at the core layer

**What.** NORTH_STAR M6 / visibility tenet 3 says *"ghx invocations emit OTel
traces."* Core `internal/ghx` is entirely telemetry-free (grep for
`otel|telemetry` in `internal/ghx/*.go` non-test → empty); only sidecar-driven
invocations are traced (`internal/sidecar/telemetry/`, `emit.go`).

**Why it's Low.** This is arguably intentional — a bare `ghx explore` from a
human shell need not emit traces, and sidecar-mediated invocations *are* traced
at the ACP/turn layer. But the tenet's literal wording ("ghx invocations")
isn't satisfied at the CLI level. Flag for a doc/tenet clarification rather
than code: either narrow the tenet to "sidecar-mediated invocations" or note
that CLI-level tracing is a deliberate non-goal.

**Recommendation.** One-line clarification in NORTH_STAR / the relevant ADR;
no code change implied.

### L2 — Minor "backward compatible" notes worth a scan pass

**What.** A few smaller compat notes exist; most are benign API-shape comments,
not shims, but they deserve one confirming read given the "no compat junk"
tenet.

**Evidence.**
- `internal/ghx/tree.go:12` — "Without Depth: returns blobs only (backward
  compatible)." This is a default-behavior doc, not a shim (acceptable).
- `internal/sidecar/prompt.go:237` — "Backward compatibility: callers that do
  NOT use session-level meta …" — a live dual path; verify it isn't a
  removable shim now that all callers set session meta.
- Legacy-session backfill (`internal/sidecar/runtime.go:125,328`,
  `session.go:248`, `reroute.go:29`) reads pre-existing on-disk session
  formats. Since ghx is single-install (config.go:16 reasoning), confirm
  whether any `~/.ghx` sessions predate the new layout; if not, this backfill
  is removable compat per the tenet.

**Recommendation.** A targeted follow-up: audit `prompt.go:237` and the
legacy-session backfill against actual on-disk `~/.ghx` state; delete if no
old-format artifacts exist.

---

## Recommended sequence (top refactors, ordered)

1. **Kill the `callTool` compat path (H1).** Delete executor.go:180-196, drop
   the compat-only executor tests, and rewrite the `search_tools` usage string
   (serve.go:348) to `codemode.<tool>(args)`. One API, aligned with ADR-0010
   and the "no compat junk" tenet. Smallest, clearest, agent-facing win.
2. **Lift the failure taxonomy into core (M2).** Introduce typed error kinds on
   the core result path; map to CLI exit codes and MCP structured errors. This
   is a prerequisite for the ADR-0028.1/A4 "errors as agent affordances" arc
   and removes the current three-model divergence.
3. **Decompose `acp.go` (H2).** Split into turn / acp_client / tooltrace and
   fold report-sink resolution into `reportsink.go`. Pure, test-verifiable
   decomposition; halves the comprehension cost of the sidecar's busiest file.
4. **Unify CLI presentation ownership (M1).** Route every CLI command through
   named `Format*Text` formatters (matching the `FormatInspectText`
   precedent); keep MCP on JSON deliberately and document the split.
5. **(Deferred, ADR-gated) eval god-file split (M3)** — only inside a
   pre-registered eval ADR, never as drive-by refactoring.

## Method / auditability

- Import-direction claims from `grep -rn '<module>/internal/...'` over
  non-test `.go` files in each package.
- God-file sizes from `find internal cmd -name '*.go' ! -name '*_test.go' |
  xargs wc -l | sort -rn`.
- Every `file:line` above was spot-checked against the working tree; `go build
  ./...` succeeds at audit time (no code was modified by this audit).
