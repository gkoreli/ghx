---
title: "ADR-0035: Architecture Hardening — Prioritized Refactor Sequence"
date: "2026-07-07"
status: "proposed"
thread: "architecture"
author: "Fable (synthesis of five architecture audits)"
---

# 0035. Architecture Hardening — Prioritized Refactor Sequence

## Status

Proposed. Governs a standing tech-debt/maintainability workstream: keep ghx
maintainable and preserve engineering velocity as it progresses toward P3
("swallow the tools") and P4 ("below ACP"). This ADR **does not** change eval
scoring, gates, detectors, or any committed benchmark claim — items that touch
the measurement stack are explicitly deferred behind a pre-registered eval ADR
(AGENTS.md frozen-measurement rule).

## Context

Goga (2026-07-07) asked for recurring **adversarial** architecture/tech-debt
audits (in the style of `docs/audits/architecture-2026-07-07.md`, whose top
three findings — `callTool` removal, `acp.go` decomposition, and the
failure-taxonomy work — are already done or captured in ADR-0034), followed by
high-impact refactoring so tech debt does not compound and cost velocity.

Five audits now exist, each read-only with `file:line` evidence and its own
ranked "Recommended sequence":

- `docs/audits/architecture-2026-07-07.md` — core/frontend boundary (verified
  healthy) + presentation-ownership + god-files.
- `docs/audits/design-patterns-domain-modeling-2026-07-07.md` (AUD1) —
  primitive obsession, stringly-typed domain concepts, anemic models.
- `docs/audits/complexity-hotspots-2026-07-07.md` (AUD2, GPT-5.5) —
  gocyclo/size/duplication/error-sprawl metrics.
- `docs/audits/coupling-cohesion-testability-2026-07-07.md` (AUD3) — hidden
  ambient dependencies, missing test seams, duplicated turn engine.
- `docs/audits/adversarial-velocity-2026-07-07.md` (AUD4) — roadmap blockers
  for P3/P4 and the framework/ghx-domain weld.

**Independent auditors converged on the same hotspots** — that convergence,
not any single report, is the signal this ADR ranks on:

| Hotspot | Converging findings |
| --- | --- |
| `owner/repo` is a bare string (3 validators, a **live panic**) | AUD1 H1 |
| The sidecar turn engine is both too complex **and** duplicated | AUD2 H1 (`askWithTurnRunner` cyclo 40) + AUD3 H2 (one-shot vs warm duplication) |
| Core recon has no client seam → **zero network-path tests** | AUD3 H1 |
| No sidecar tool registry → P3 absorption is a 6-place edit | AUD4 V1 |
| The ACP adapter leaks past ACP through the steering layer | AUD4 V2/V4/V5 |
| CLI presentation ownership is inconsistent / inline god-files | arch M1 + AUD2 M1 + AUD3 L1 |
| `Depth`/`Tier` stringly-typed, validated 3× with divergence | AUD1 M2/M3 |
| MCP handlers duplicate parse→call→marshal→error; tool contract triplicated | AUD2 M2 + AUD1 M1 (+ ADR-0034) |
| Ambient storage root + scattered `time.Now()` | AUD3 M1/M2/M3 |
| Eval god-files + `TryReuseBaselines` cyclo 45 | AUD2 H2 + arch M3 (**measurement stack — deferred**) |

Honest good news the audits also verified (so we do **not** over-refactor): the
core/frontend boundary is clean and has no dependency-direction violations
(arch audit); the ACP **wire** is swappable with no leak into `cmd/**` (AUD4);
and several interface seams are already exemplary — `TurnRunner`, `JudgeClient`,
`GitRunner`, `ContainerRuntime`, `WritePolicy`, `mapengine.Mapper`, `RouteSource`
(AUD1/AUD3). The refactors below **extend existing good patterns**, they do not
invent new frameworks (open-source-leverage / no-speculative-abstraction).

## Decision

Execute the following sequence, top-down. Each item is one worker branch,
landed rebase+ff (AGENTS.md "Integrating Delegated Work"), verified against this
ADR, gated by `go test ./...` + race. Every item is **behavior-preserving**
unless it explicitly says otherwise; pin with existing tests and goldens.

### Tier 1 — do first (high impact, tractable, low risk)

1. **`ghx.Repo` value object (AUD1 H1).** Replace the bare `owner/repo` string
   and its three divergent hand-rolled validators (`explore.go:25`, `read.go:55`,
   `inspect.go:395`, unchecked `glob.go:19-20`) with one parsed type. **This
   fixes a real crash**: `ghx tree noslash` panics today via the unchecked
   `[1]` index (`tree.go:14-15` → `glob.go:19-20`). Smallest change, largest
   benefit, removes a bug class, and sets the value-object precedent Tier 3
   extends. Bug-fix + refactor.

2. **Core GitHub-client injection seam (AUD3 H1).** Every core op news up
   `api.DefaultGraphQLClient()` / `NewRESTClient()` inline (`explore.go:32`,
   `read.go:93`, `repos.go:35`, `glob.go:22`, `search.go:40`); no test exercises
   a network op. Introduce a small `graphQLDoer`/client interface injected at the
   core seam so the product's core logic becomes regression-testable without
   live GitHub. Foundational for every later core change.

3. **Consolidate the sidecar turn engine (AUD2 H1 + AUD3 H2).** Two converging
   problems in the same subsystem: `askWithTurnRunner` (cyclo 40,
   `runtime.go:273`) owns ~10 concerns in one function, and the one-shot
   (`acp.go` `RunTurnWithOptions`) and warm (`daemon_worker.go` `AgentWorker.RunTurn`)
   paths duplicate the liveness watchdog, the four-way prompt `select`, and
   session/meta re-assertion. Extract one `acpConversation` turn primitive plus
   named lifecycle steps (`prepareSession`, `buildTurnOptions`, `runPrimaryTurn`,
   `recoverMaxTurns`, `resolveReportWithRetry`, `persistTurn`, `emitArtifacts`).
   Safety-critical; must stay byte-identical, pinned by existing runtime tests.

### Tier 2 — roadmap enablers (high value, larger)

4. **Sidecar `ToolSpec` registry (AUD4 V1).** Today adding a P3 tool touches ~6
   places (persona prose menu `prompt.go:145-154`, hardcoded `tier2.Service`,
   post-hoc stderr reconciliation `tier.go`) — and twice touches the frozen
   measurement stack. Introduce a real tool registry (model on the unused
   in-repo `codemode/registry.go` pattern) so tool absorption is one
   registration. **Constraint:** must reproduce persona prompt + backend-ID
   output **byte-for-byte** (golden test), keeping it a measurement-identity-
   neutral change, separate from any new-tool persona revision.

   Implementation note (2026-07-07): `internal/sidecar/tool_registry.go` now
   owns `ToolSpec` registration for the current Tier-2 tools and renders the
   persona menu, backend-ID schema description, runtime local-backend check,
   and CLI backend stderr identity from that registry. The concrete Tier-2
   subprocess substrate remains in `internal/sidecar/tier2` because that
   subpackage cannot import the parent `sidecar` package without an import
   cycle; typed `RunCodemap`/`RunAstGrep`/`RunRepomap` methods remain where
   their request models live. Measurement identity stayed unchanged:
   `TestRepoScopedPersonaByteStable` passed without modifying its golden hash.

5. **`Harness` interface + harness-neutral `SteeringSpec` (AUD4 V2; folds V4,
   V5).** The claude-agent-acp *adapter* (not ACP itself) leaks through the
   steering layer: the `claudeCode` wire key (`session_options.go:222`), the
   permission taxonomy (`denyclient.go:161-166`), and English-error-string
   recovery gating (`acp.go:59-64,78-80` → `runtime.go:435`). Extract a `Harness`
   seam with one claude-agent-acp implementation; anchor recovery on structured
   JSON-RPC codes + an adapter-version preflight instead of error-string
   matching (V4); neutralize `ToolCallTrace`/accounting at the harness edge (V5).
   This is the concrete P4 ("below ACP") prerequisite and is cheapest now while
   exactly one adapter exists.

### Tier 3 — consistency & quality (medium)

6. **CLI presentation ownership (arch M1 + AUD2 M1 + AUD3 L1).** Route every CLI
   command through named `Format*Text` presenters (matching the existing
   `FormatInspectText` precedent), extract the inline `RunE` bodies from the
   `ghx.go` (797 lines) / `sidecar.go` (760 lines) god-files into typed `run*`
   functions, and stop the CLI reaching into sidecar `Report`/`SessionMeta`
   fields directly. Keep MCP on `json.Marshal` deliberately and document the split.

7. **Type `Depth` and `Tier` (AUD1 M3 + M2).** Both are bare strings validated in
   three places with reject-vs-coerce divergence (MCP rejects invalid depth at
   `serve.go`, CLI silently coerces at `sidecar.go`). One value type each, one
   validator, one behavior.

8. **MCP handler adapter + single tool-contract owner (AUD2 M2 + AUD1 M1),
   sequenced with ADR-0034.** Collapse the per-handler parse→call→`json.Marshal`→
   `NewToolResultError` duplication in `serve.go` behind a typed decoder +
   `jsonToolResult` + one error mapper, and make the registry/core `*Opts` the
   single owner of the tool-contract defaults currently copy-pasted across
   `register.go`/`serve.go`/`ghx.go`. This is the natural integration point for
   ADR-0034's structured error mapping.

9. **Inject storage root + clock (AUD3 M1/M2/M3).** Resolve `~/.ghx` once into
   `Config.Home` instead of `rootDir()` re-reading `os.Getenv` at every path call
   (which forces 16 serial `t.Setenv` tests), thread the existing
   `Now func() time.Time` seam through the load-bearing `time.Now()` sites
   (worker TTL/liveness, session-ID minting), and close the two daemon hidden
   edges (`ConfigDigest` folding `os.Getenv`; dispatch dropping request context).

### Deferred — ADR-gated, never drive-by

- **Eval god-file splits + `TryReuseBaselines` (AUD2 H2 + arch M3).**
  `discovery.go` (1007), `anticipation_predictor.go` (974), `gates.go` (920),
  and `TryReuseBaselines` (cyclo 45) are the largest mechanical hotspots — but
  they are the **measurement stack**. Split them **only** inside a pre-registered
  eval ADR that preserves recomputability of committed artifacts. Not part of
  this ADR's execution.
- **`Read` planner/fetcher/shaper (AUD2 M3) and repomap `EdgeBuilder`
  (AUD2 M4).** Opportunistic; `Read` is user-visible core output, so pin or
  deliberately version the CLI/MCP JSON shape.

### Posture — not a refactor

- **Do not deepen the framework/ghx-domain weld (AUD4 V3).** The `Report`,
  `AskRequest.Repo`, and `const` persona are GitHub-specific and welded into one
  package. Extracting a generic framework now **fails the north-star filter** —
  do not do it. The obligation is only to stop *adding* new GitHub-specific
  coupling so the eventual second-domain boundary stays bounded and cheap.

## Constraints (binding on every item)

- Behavior byte-identical unless an item explicitly versions a user-visible
  change; pin with existing tests + goldens, add tests where a seam is new.
- **No change to eval scoring, gates, detectors, or reward math outside a
  pre-registered eval ADR** (AGENTS.md). Items 4 and 8 touch code near the
  sidecar; they must preserve persona-prompt and backend-identity output
  byte-for-byte (golden), so baseline reuse and identity hashes are unaffected.
- One worker branch per item, landed rebase+ff, `go test ./...` + race green
  after each, evidence audited against this ADR before landing.
- Prefer extending existing seams (`TurnRunner`, `mapengine.Mapper`,
  `RouteSource`, `codemode/registry.go`) over inventing abstractions.

## Consequences

- Tier 1 removes a crash class, makes core testable, and halves the
  comprehension cost of the busiest runtime file — the highest velocity return.
- Tiers 2 unblocks P3 (tool registry) and P4 (harness seam) at their cheapest
  point, before more tools and a second adapter make them expensive.
- Tier 3 pays down consistency debt that otherwise taxes every new command,
  tool, and error path.
- The eval hotspots are acknowledged but quarantined behind the frozen-
  measurement rule, so this hardening never risks a benchmark claim.

## Cross-references

- The five audit docs under `docs/audits/` (each finding's `file:line` lives there).
- ADR-0010 — core-in-`internal/ghx`; frontends wrap the core.
- ADR-0034 — failure-class model (prerequisite/companion for item 8; W4's
  inventory `docs/audits/failure-class-inventory-2026-07-07.md` is its evidence).
- ADR-0030 — always-on daemon (item 3/9 touch its worker/pool code).
- AGENTS.md — Engineering Tenets, frozen-measurement rule, "Integrating
  Delegated Work".
- NORTH_STAR §P3/P4 and "The Moat" (items 4/5 are the roadmap-critical enablers).

## Provenance

Synthesized by Fable from five independent read-only audits (four spawned
2026-07-07 with deliberately different/adversarial lenses: design-patterns,
complexity-metrics via GPT-5.5, coupling/testability, and roadmap red-team).
Ranking is by cross-auditor convergence × impact × tractability, respecting the
north-star filter and the frozen-measurement rule. Awaiting Goga's acceptance
before Tier-1 execution; the `ghx tree` panic (item 1) is a standalone bug that
may be fixed immediately on request.
