---
title: "Design-Patterns & Domain-Modeling Audit — ghx primitive obsession, anemic types, seam patterns"
date: "2026-07-07"
status: "audit"
author: "background audit worker"
scope: "internal/ghx, internal/sidecar, internal/cli, internal/codemode, internal/mapengine, cmd — read-only audit, no code changes"
lens: "design patterns & domain modeling (adversarial)"
---

# Design-Patterns & Domain-Modeling Audit — 2026-07-07

Read-only audit of the ghx codebase through a **design-patterns and
domain-modeling** lens: primitive obsession / stringly-typed APIs, anemic
models, missing interfaces at seams, constructor/option sprawl, and places a
named pattern genuinely reduces complexity. Every finding cites `file:line`,
ties to a named `AGENTS.md` Engineering Tenet, and is ranked by whether the
fix improves maintainability/velocity as ghx heads to **P3 (swallowing more
tools)** and **P4** — not by pattern purity. Fable decides what to act on.

This audit deliberately does **not** re-raise the already-actioned findings
from `architecture-2026-07-07.md`: the `callTool` compat path (removed —
`internal/codemode/executor.go` now injects only the `codemode` object,
executor.go:180-199), the `acp.go` god-file split (done — `turnresult.go`,
`tooltrace.go`, `denyclient.go` are now cohesive siblings), and the
failure-taxonomy work (ADR-0034).

Tenets audited against (`AGENTS.md` "Engineering Tenets", lines 207-220):

- **"Define proper domain models."** *"If a concept appears in two places, it
  deserves a type; the type is what makes the code understandable."*
- **"Encapsulate domain logic in services."** *"One authoritative owner per
  domain concern … construct it once and inject/share it."*
- **"Composition or inheritance by use case, never dogma."** interfaces +
  embedding + functional options chosen for the actual shape of the problem.

## Executive summary

**The highest-leverage modeling fix is to give ghx's central noun — the
`owner/repo` slug — a type. It is a bare `string` re-parsed and re-validated
at every core entry point with three divergent validators, one of which is
missing entirely and leaves a latent index-out-of-range panic on the `tree`
path (`internal/ghx/glob.go:19-20`, reachable unguarded from
`internal/ghx/tree.go:14-15` and `internal/cli/ghx.go:455`).** A `ghx.Repo`
value object (`ParseRepo`, `.Owner`, `.Name`, `.String()`) is the one change
that simultaneously (a) removes a crash class, (b) collapses three
inconsistent validators into one, and (c) compounds in value with every
repo-scoped tool P3 adds — each new tool inherits parsing and validation for
free instead of hand-rolling `strings.Split(repo, "/")` a fifth, sixth,
seventh time. It directly instantiates the "if a concept appears in two
places it deserves a type" tenet on the concept that appears in the *most*
places.

The important positive result: **the interface seams are healthy — this is
not a "missing interfaces" codebase.** Every place a small interface belongs,
one already exists (TurnRunner func-injection, JudgeClient, GitRunner,
ContainerRuntime, WritePolicy, Transport, mapengine.Mapper). Inventing more
would be pattern-for-pattern's-sake. The real modeling debt is narrower and
more mechanical: **primitive obsession at the domain nouns** (repo, tier,
depth) and **a tool-contract that is hand-duplicated across three frontends**
instead of owned once. Both bite hardest exactly as P3 multiplies the tool
count. Notably, the codebase already knows how to model these well —
`RouteDecision`/`RouteSource` (route.go), `tier2.EscalationPolicy` (policy.go),
and `mapengine.Engine/Level/Kind` (types.go) are exemplary — so the findings
below are asking to *extend an established precedent*, not import a new style.

### What is clean (verified, not assumed)

- **Seams are interface/function-injected, not concrete-coupled.** The ACP
  turn seam is a function type (`TurnRunner`, runtime.go:20, injected by
  `AgentPool.RunnerFor`, daemon_worker.go:36); the daemon call is a swappable
  var (`askSidecar`, serve.go:166); judges, git, containers, write-policy, MCP
  transport, and the mapper are all interfaces (`JudgeClient` judge.go:41,
  `GitRunner` clone.go:13, `ContainerRuntime` container.go:42, `WritePolicy`
  write_policy.go:17, `Transport` serve.go:20, `mapengine.Mapper` types.go:47).
  There is no concrete-type-at-a-seam finding to make.
- **Exemplary domain models already set the precedent.** `RouteDecision`
  carries behavior (`Provenance`, `Line`, `sessionNamedBy`, route.go:186-223)
  over a typed `RouteSource` enum; `tier2.EscalationPolicy.Evaluate` is a pure
  function over typed observations (policy.go:214); `mapengine` dispatches a
  `Mapper` strategy over typed `Engine/Level/Kind` enums (types.go:11-45,125).
- **`Config` is well-encapsulated, not an anemic bag.** Resolved-default
  methods (`DaemonWorkerIdleTTL`, `DaemonMaxConcurrentTurns`, `CaptureContent`,
  config.go:88+) and a `ValidateDaemonRuntimeConfig` live on the type.
- **`Report` has real behavior.** `NormalizeNextReads`, `ExtractReportErr`,
  and `ValidateReport` are methods/owners on the report concern
  (report.go:151, report.go:98, reportsink.go:219) — it is not a pure struct
  with logic scattered across callers.

---

## High severity

### H1 — `owner/repo` is a bare string re-parsed and re-validated everywhere, with divergent semantics and a latent panic

**What.** Repo identity — the single most central domain concept in a GitHub
reconnaissance tool — is modeled as a bare `string` (`repo string`) at every
core boundary. Each entry point re-implements `strings.Split(repo, "/")` with
its *own* validation rule, and one path has no validation at all, leaving a
reachable index-out-of-range panic.

**Evidence.**
- Three different validators for the same concept:
  - `internal/ghx/explore.go:25-28` — `strings.Split`, rejects `len(parts) != 2`.
  - `internal/ghx/read.go:55-58` — `strings.Split`, rejects `len(parts) != 2`.
  - `internal/ghx/inspect.go:394-397` — `validRepo`: `len(parts) == 2 &&
    parts[0] != "" && parts[1] != ""` (a *stricter* rule than the other two).
- **No validator, and a panic:** `internal/ghx/glob.go:19-20` does
  `owner := strings.Split(repo, "/")[0]` / `name := strings.Split(repo, "/")[1]`.
  `fetchTree` is reached from `Tree` (`internal/ghx/tree.go:14-15`) with **no
  prior repo check**; the CLI `tree` command guards only `--depth`, then calls
  `ghxlib.Tree(args[0], …)` unguarded (`internal/cli/ghx.go:451-455`), and the
  MCP/codemode wrappers don't validate shape either (`handleTree`
  serve.go:334-343; `wrapTree` register.go:152-159). Result: `ghx tree noslash`
  indexes `[1]` on a one-element slice → panic. `Read` happens to be safe only
  because it re-validates at read.go:55 first — the safety is per-call-site
  luck, not a property of the type.
- The concept then travels as a bare string on every model that carries it:
  `SearchMatch.Repo` (search.go:12), `RepoResult.NameWithOwner` (repos.go:13),
  `InspectResult.Repo` (inspect.go:41), `AskRequest.Repo` (runtime.go:221),
  `TierDecisionRecord.Repo` (tier.go:39) — and the split is even re-hand-rolled
  in the eval layer (`internal/sidecar/evals/discovery.go:667`).

**Why it violates a tenet.** `AGENTS.md` "Define proper domain models": *"If a
concept appears in two places, it deserves a type … never plain maps …
passed around with the same helper logic hand-rolled at every call site."*
`owner/repo` is hand-rolled at **six+** sites with **three** validation
semantics and one missing one. This is the textbook primitive-obsession case.

**Recommendation.** Introduce a `ghx.Repo` value object:
`ParseRepo(string) (Repo, error)` (one validator), `Owner`, `Name`,
`String()`. Core operations (`Explore`, `Read`, `Tree`, `Inspect`,
`fetchTree`) take `Repo`; parse once at each frontend boundary and pass the
type inward. This deletes the four ad-hoc splits, unifies the three
validators, and structurally removes the panic. As P3 adds repo-scoped tools,
each one gets validation for free. Small, mechanical, high-leverage.

---

## Medium severity

### M1 — The tool contract (arg names, defaults, valid enums) is hand-duplicated across three frontends instead of owned once

**What.** Each ghx tool is declared **three times** — in the codemode
registry (`RegisterTools`), in the MCP direct handlers (`serve.go`), and in
the Cobra CLI (`ghx.go`) — and the shared facts (defaults, arg decoding) are
copy-pasted between them rather than owned by one authority. Every new P3 tool
multiplies this by three, and the copies drift silently.

**Evidence.**
- Default values duplicated across surfaces:
  - repos limit `10` appears at `register.go:99` (`wrapRepos`), `repos.go:28`
    (`Repos` clamp), and `serve.go:259` (`GetInt("limit", 10)`).
  - search limit `30` appears at `register.go:112`, `search.go:34`, and
    `serve.go:280`. The core `*Opts` normalizers already own the clamp, so the
    frontend copies are pure duplication that can disagree.
- Arg-decoding logic duplicated: `wrapRead` decodes
  `grep/lines/map/level/kind/mapEngine` from a `map[string]any`
  (`register.go:123-150`); `handleRead` decodes the identical set from MCP
  request accessors (`serve.go:295-332`). Two hand-maintained copies of one
  contract.
- Three parallel schema declarations of the same tools: JSON-schema maps
  (`register.go:7-88`), `mcp.NewTool(...)` builders (`serve.go:81-117`), and
  Cobra flag registration (`ghx.go`).

**Why it violates a tenet.** `AGENTS.md` "Encapsulate domain logic in
services": *"One authoritative owner per domain concern."* The tool's arg
contract and defaults are a domain concern with, today, three owners. This is
the maintenance tax that grows linearly with the P3 tool count.

**Recommendation.** Make the `codemode.Tool`/`Registry` the single owner of
each tool's arg contract: the typed-arg decoder and any default live there
(or in the core `*Opts` normalizer, which already clamps), and the MCP
`handle*` and CLI flag layers *derive* from it. Minimum viable step: delete
the duplicated default literals in `register.go`/`serve.go` so defaults live
only in the core `*Opts`, removing the drift surface even before unifying the
schemas.

### M2 — "Tier" is a stringly-typed concept with no single owner; two constants exist but siblings hardcode the literals

**What.** The escalation tier — a first-class abstraction for P3 (more tools
→ more backends and tiers) — is a bare `string` (`"tier0"`…`"tier3"`) with no
owning type. The tier2 package defines *two* tier constants; the rest of the
codebase ignores them and re-spells the literals, and the "set of valid
tiers" is redefined independently.

**Evidence.**
- `internal/sidecar/tier2/policy.go:36-40` defines `TierRemote = "tier1"` and
  `TierLocal = "tier2"` — only two of the four, as bare string consts.
- `internal/sidecar/tier.go:142-152` (`deriveTierUsed`) returns bare literals
  `"tier2"`/`"tier1"`/`"tier0"` — it does **not** use `tier2.TierLocal`/
  `TierRemote` (grep for those symbols in `internal/sidecar/*.go` is empty).
- The valid set is re-listed from scratch as a map at
  `internal/sidecar/reportsink.go:212` (`{"tier0","tier1","tier2","tier3"}`)
  and re-spelled again in the validation error (reportsink.go:226-227), the
  field description (reportsink.go:262), and the doc comment (report.go:52).
- Every carrier is a bare string: `Report.TierUsed` (report.go:52),
  `EscalationDecision.FromTier`/`ToTier` (policy.go:105-106),
  `TierDecisionRecord.TierUsed`/`RuntimeDerivedTierUsed` (tier.go:46,51).

**Why it violates a tenet.** "Define proper domain models … if a concept
appears in two places it deserves a type." "Which tier" appears in at least
five places with two half-defined constants and a separately-maintained valid
set — no single owner of tier identity or ordering.

**Recommendation.** One `Tier` type (`Tier0`…`Tier3` constants, `Valid()`,
`String()`, and ordering for "highest tier used"), owned in one package.
`deriveTierUsed`, `EscalationDecision`, `Report.TierUsed`, and
`ValidateReport` all reference it. Do this before P3 raises the tier ceiling
and the literal-sprawl compounds. (Note: `ValidateReport`/`reportsink` are the
report *contract*, not the frozen eval scoring stack, so this is not
ADR-gated the way M3 in the architecture audit is.)

### M3 — "Depth" budget dial is a bare string validated three times, with divergent behavior between frontends

**What.** The recon depth dial `"cheap|normal|deep"` is a bare `string`
(`AskRequest.Depth`) whose valid set and budget mapping are re-implemented
independently in three places — and because there is no shared type, the two
frontends already **disagree** on what an invalid value does.

**Evidence.**
- `AskRequest.Depth string` (`internal/sidecar/runtime.go:227`).
- MCP path validates and **rejects** unknown values:
  `isValidReconDepth` (`internal/cli/serve.go:227-234`), called at
  serve.go:191-195.
- The budget mapping owns its own copy of the valid set:
  `depthBudgets` map keyed by `"cheap"/"normal"/"deep"`
  (`internal/sidecar/session_options.go:159-166`) with a silent fallback to
  `"normal"` on lookup miss (session_options.go:194-198).
- CLI path has **no** validation: `--depth` defaults to `"normal"`
  (`internal/cli/sidecar.go:671`) and is forwarded verbatim
  (sidecar.go:72,95). So `ghx sidecar ask --depth bogus` is silently coerced
  to normal, while the MCP `recon` tool rejects it — divergent handling of the
  same concept, a direct consequence of the missing type.

**Why it violates a tenet.** "Define proper domain models." Depth is one
concept with three owners and no single parse point, which is exactly how the
reject-vs-coerce divergence arose.

**Recommendation.** A `Depth` type owning the valid set and the budget
mapping (`ParseDepth(string) (Depth, error)`, `Budget() depthBudget`). Both
frontends parse through it, unifying validation and eliminating the
silent-coerce-vs-reject split. Pairs naturally with M2 — same fix shape,
different noun.

---

## Low severity

### L1 — Small enum-shaped concepts left as bare strings, against the codebase's own typing precedent

**What.** Several small domain enums are bare strings even though the codebase
demonstrably knows how to type them (`RouteSource` route.go:17;
`mapengine.Engine/Level/Kind` types.go:11-45). These are low-bug-surface, but
typing them is nearly free and removes stringly-typed comparisons.

**Evidence.**
- **File type** `"blob"`/`"tree"`: `FileEntry.Type string` (explore.go:10-13)
  and `treeEntry.Type string` (glob.go:14), compared as raw strings at
  `glob.go:93` (`e.Type != "tree"`) and `tree.go:35` (`entry.Type == "tree"`).
  A `FileType` enum removes the magic strings.
- **Session-naming origin** sits *directly beside* the well-typed
  `RouteSource` but is left untyped: `SessionNamedExplicit/Repo/Question` are
  bare `string` consts (route.go:33-37), `SessionMeta.NamedBy` is a bare
  string, and `sessionNamedBy()`/`sessionOrigin()` return bare strings
  (route.go:214-223, 367-378). It is a sibling concept to `RouteSource` and
  deserves the same treatment for consistency.
- **Backend IDs** are half-typed: `tier2` has `BackendRepomap`/`Codemap`/
  `AstGrep` consts (repomap.go:31, codemap.go:14, astgrep.go:17), but there is
  no `Backend` type, `"remote"` has no constant, and the canonical set is
  re-spelled as prose at report.go:50-51, reportsink.go:262, and
  prompt.go:150-152.

**Recommendation.** Type these opportunistically when touching the files:
`FileType`, a `SessionNamedBy` type (mirroring `RouteSource`), and a `Backend`
type that owns the canonical set + the `local:`/`remote` distinction (folding
in `LocalBackendAllowed`). Not urgent; do it as follow-through when the
neighboring code is already open.

### L2 — Anemic arg-decode boilerplate and copy-pasted closures

**What.** Minor repetition that a small helper collapses.

**Evidence.**
- The `wrap*` functions (`register.go:91-160`) are near-identical
  `map[string]any` → typed-arg extractions with hand-written defaults; a
  single typed-arg decoder (or code-generated stubs, given `typegen.go`
  already exists) would remove the boilerplate and is the natural home for the
  M1 default-ownership fix.
- `console.log`/`warn`/`error` are three byte-identical closures in the
  executor (`internal/codemode/executor.go:203-226`); one shared appender
  registered under three names removes the duplication.

**Recommendation.** Low priority; fold the `wrap*` cleanup into M1 rather than
as a standalone change.

---

## Recommended sequence (top refactors, ordered)

1. **Introduce `ghx.Repo` (H1).** One `ParseRepo` + `Owner`/`Name`/`String()`;
   thread it through `Explore`/`Read`/`Tree`/`Inspect`/`fetchTree`. Removes
   the `tree` panic, unifies three validators, and every future repo-scoped
   P3 tool inherits validation. Smallest change with the largest blast radius
   of benefit — do it first.
2. **Collapse the triplicated tool contract (M1).** Make the registry / core
   `*Opts` the single owner of defaults and arg decoding; have MCP handlers
   and CLI flags derive from it. Start by deleting the duplicated default
   literals. This is the change that keeps P3's tool growth linear instead of
   3×.
3. **Type `Tier` (M2) and `Depth` (M3) together.** Same fix shape (enum type +
   `Parse`/`Valid` + owned mapping), two central escalation/budget nouns.
   Fixes the depth reject-vs-coerce divergence as a side effect and stops the
   tier-literal sprawl before P3 widens it.
4. **Type the small enums (L1) opportunistically** — `FileType`,
   `SessionNamedBy`, `Backend` — as follow-through when their files are open,
   extending the `RouteSource`/`mapengine` precedent.
5. **(Cosmetic) fold the `wrap*` / console boilerplate (L2)** into the M1
   work; not worth a standalone change.

## Method / auditability

- Primitive-obsession sites found by `grep -rn 'strings.Split(repo'` and
  `grep -rn '"tier0"|"tier1"|"tier2"|"tier3"'` over `internal/**` non-test
  `.go` files, then each hit read in context to confirm the validator/owner
  divergence claimed above.
- Seam-interface inventory from `grep -rn '^type .* interface'` over
  `internal/**` and confirming the ACP-turn seam is the `TurnRunner` func type
  (runtime.go:20) — the basis for the "no missing-interface finding" claim.
- The H1 panic path was traced by reading, not executing (read-only audit,
  and the path makes a network call): `Tree` (tree.go:14) →
  `fetchTree` (glob.go:18-20, unchecked `[1]` index) with the only guard on
  the `tree` command being `--depth` (ghx.go:451-455). `Read` avoids it solely
  because read.go:55 re-validates first.
- Line numbers were verified against **this worktree's tree**
  (`worktree-agent-a81c4b04ac4352778`, HEAD `d2cda8a` / v2.7.0), which differs
  from the concurrent mainline checkout in a few files (`cli/ghx.go`,
  `config.go`, `daemon*.go`); the drifted files' citations were re-checked
  against the worktree. `go build ./...` succeeds at audit time; no code was
  modified by this audit.
