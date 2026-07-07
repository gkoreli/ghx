---
title: "Domain-Model & Ubiquitous-Language Audit — the objects that must flow, not the primitives that sit still"
date: "2026-07-07"
status: "audit"
author: "domain-modeling architect (background audit worker)"
scope: "internal/ghx, internal/sidecar, internal/cli, internal/codemode — read-only audit, no code changes"
lens: "mid/low-level domain modeling + ubiquitous language; Go-pragmatic (plain structs + constructors + methods, no getter/setter Java)"
---

# Domain-Model & Ubiquitous-Language Audit — 2026-07-07

Read-only audit answering one founder question directly: **"What domain data
objects must exist and move around, instead of moving around plain
objects?"** Every finding cites `file:line`, ties to a named `AGENTS.md`
Engineering Tenet, and is ranked by real payoff for a solo-dev product heading
into P3 ("swallow the tools") and P4 ("below ACP").

This audit **builds on and does not re-litigate** two prior documents:

- `docs/audits/design-patterns-domain-modeling-2026-07-07.md` (AUD1) already
  owns *primitive obsession at rest*: `owner/repo`, `Depth`, `Tier`, `Backend`
  as bare strings re-parsed at every call site. Its `ghx.Repo` recommendation
  **shipped** (`internal/ghx/repo.go:9-28`). I extend it along a different
  axis.
- `ADR-0035` (`docs/adr/0035-architecture-hardening-refactor-sequence.md`) has
  already sequenced the *at-rest* fixes (Repo done as Tier-1 item 1; Depth/Tier
  as item 7; the Report/SessionMeta presenter split as item 6).

**My distinct lens is motion, not rest.** AUD1 asks "which strings deserve a
type." This audit asks the founder's actual question: **which typed objects
must _flow_ across the core → sidecar → report → frontend boundaries** so that
the value object is constructed once at the edge and carried, instead of being
reborn as a `string` at every boundary and reached-into field-by-field at every
consumer. That reframing changes the ranking: the highest-leverage move is no
longer "add types," it is "make the types ghx already believes in travel."

## Executive summary

**ghx already owns the pattern — it just applies it at rest, not in motion.**
The codebase has one genuinely exemplary boundary value object (`RouteDecision`,
`route.go:161-223`, with real behavior `Provenance`/`Line` over a typed
`RouteSource` enum) and just gained its first core value object (`ghx.Repo`).
But both are consumed *inside one package* and then discarded at the boundary.
The single most valuable domain-modeling investment is not inventing more types;
it is making the nouns ghx already models **flow** — parse once at the outermost
boundary, carry the value object inward and outward, render only at the edge.

Two gaps dominate everything else, and they are the direct answer to Goga's
question:

1. **Even `ghx.Repo` dies at the boundary.** The value object exists and is
   threaded *inside* core (`explore.go:22`, `read.go:54`, `tree.go:15`,
   `glob.go:17`), but every object that carries a repo *across* a boundary is
   still a bare `string`: `AskRequest.Repo` (`runtime.go:224`), `SessionMeta.Repo`
   (`session.go:17`), `SearchMatch.Repo`, `RepoResult.NameWithOwner`, and the CLI
   hands core a raw `args[0]` (`cli/ghx.go:57`). So core *re-parses* a string it
   was handed instead of *receiving* the type. The value object never crosses
   core→sidecar→report. Finishing the flow it already started is the cheapest,
   highest-leverage change in this document.

2. **The one value object that is actually missing: `Snapshot` (repo @ commit
   SHA).** Every read resolves `defaultBranchRef.name` *fresh* on each call
   (`explore.go:98`, `glob.go:41`) and `Read`/`Explore`/`Tree` take **no ref
   argument** (`read.go:49`). So two reads inside one investigation can observe a
   *different tree* if HEAD moves between them, and `Evidence` records a command
   string (`report.go:31-34`) but **not the commit it ran against** — a report's
   evidence is not deterministically re-fetchable. That quietly undercuts the
   tenet the entire eval thesis rests on: NORTH_STAR "every sidecar answer
   carries files, commands, snippets… the trace is visible," and AGENTS.md
   "every score recomputable from committed artifacts." A `Snapshot{Repo, SHA}`
   captured into `Evidence` is the value object that removes the
   non-reproducible-evidence bug class.

Everything else in this audit is the *same move* applied to the next nouns:
`Depth`/`Tier`/`Backend` become value-object enums parsed at the boundary
(AUD1 already ranked this; I add the flow view); `Report` becomes an immutable
object with a presenter so the CLI stops reaching into its loose fields
(`cli/sidecar.go:718-721`); `FailureClass` (proposed, ADR-0034) becomes the
class that flows core→frontend. The unifying rule for the whole vocabulary:

> **Parse at the boundary, flow the type inward, render at the edge.** Today
> ghx does the inverse — it flows primitives inward, re-parses/re-validates at
> every interior site, and reaches into loose fields at the edge.

### What is already well-modeled (verified, not assumed)

Do not over-refactor: the following are the *precedent* to extend, not debt.

- **`RouteDecision` is the reference boundary value object.** Behavior lives on
  the type (`Provenance` route.go:186, `Line` route.go:208, `scored`
  route.go:182, `sessionNamedBy` route.go:214) over a typed `RouteSource` enum
  (route.go:18-27). It rides back on every response surface (`TurnResult.Route`,
  turnresult.go:23). This is exactly the shape the rest of the vocabulary should
  copy.
- **`Claim` / `RelevantFile` / `Evidence` are proper small value objects**
  (`report.go:17-34`), not stringly-typed lists. `Report` already carries real
  behavior (`NormalizeNextReads`, `ExtractReportErr`, `ValidateReport` per AUD1's
  "what is clean"). The debt is not that these are anemic; it is that they are
  *mutable and reached-into* (D4) and that `Evidence` lacks a `Snapshot` (D2).
- **`ghx.Repo` is a correct value object** (`repo.go:9-28`): one `ParseRepo`
  validator, `String()`, exported `Owner`/`Name`. The only debt is that it does
  not yet *travel* (D1).
- **`EscalationPolicy` and its observations are declarative and typed**
  (`tier2/policy.go:48-99`) — a clean policy model. Its debt is only that its
  `FromTier`/`ToTier`/`AllowedBackends` fields are bare strings (D3).

---

## The ubiquitous language of ghx

The ~14 core nouns, their canonical Go type, what kind of object each is, which
package should own it, and — the anti-anemic column — **which behavior belongs
ON the type vs. in a service.** "Go-pragmatic" means plain structs + a
constructor + value/method receivers; immutability by convention (construct via
`ParseX`, treat as read-only, pass by value), reserving unexported fields +
accessors only where an invariant must hold across a trust boundary.

| Noun | Canonical Go type | Kind | Owning pkg | Behavior ON the type | Behavior in a service |
| --- | --- | --- | --- | --- | --- |
| **Repo** | `ghx.Repo` *(exists)* | Value | `internal/ghx` | `ParseRepo`, `String`, `Owner`/`Name` | GitHub fetch, endpoint building |
| **Snapshot** | `ghx.Snapshot{Repo, SHA}` *(missing)* | Value | `internal/ghx` | `String`→`owner/repo@sha`, `Repo()`, `SHA()` | resolve HEAD→SHA; fetch-at-snapshot |
| **Question** | `string` (fine as-is) | — | `internal/sidecar` | none — it is genuinely just text | overlap scoring, routing |
| **AskRequest** | `sidecar.AskRequest` *(exists, all-primitive)* | Command/DTO | `internal/sidecar` | `Validate()`, typed fields | `Ask`/`RouteQuestion` consume it |
| **Depth** | `sidecar.Depth` enum *(bare string)* | Value | `internal/sidecar` | `ParseDepth`, `Budget()`, `Valid`, `String` | none — pure value |
| **Tier** | `sidecar.Tier` enum *(bare string)* | Value | `internal/sidecar` | `ParseTier`, ordering `Max`/`Above`, `Valid`, `String` | `deriveTierUsed` *returns* `Tier` |
| **Backend** | `sidecar.Backend` + `BackendGrant` *(bare string)* | Value | `internal/sidecar` | `ParseBackend`, `IsLocal`, `String`; grant `Expand()` | permission gating |
| **Session** | `sidecar.SessionMeta` *(exists)* | **Entity** | `internal/sidecar` | `RecordTurn`, `SetACPSessionID`, `FormatText` | `SessionStore` load/save; routing |
| **SessionNamedBy** | `sidecar.SessionNamedBy` enum *(bare string const)* | Value | `internal/sidecar` | `String`, `Valid` | inferred by `sessionOrigin` |
| **Route** | `sidecar.RouteDecision` *(exists, exemplary)* | Value | `internal/sidecar` | `Provenance`, `Line`, `scored` | `RouteQuestion` produces it |
| **Report** | `sidecar.Report` *(exists, mutable bag)* | Value (immutable) | `internal/sidecar` | `Validate`, `NormalizeNextReads`, `FormatText` | `ExtractReport` parses it from turn text |
| **Claim / Evidence / RelevantFile** | as-is *(good)* | Value | `internal/sidecar` | `Evidence.WithSnapshot` (new) | none |
| **Turn** | `sidecar.TurnResult` *(flags-not-a-class)* | Transient DTO | `internal/sidecar` | fold 4 bools → `TurnOutcome` | runtime populates it |
| **ToolCall** | `sidecar.ToolCallTrace` *(exists)* | Value | `internal/sidecar` | `Kind` → typed `ToolKind` | ACP notification parsing |
| **FailureClass** | `ghx.FailureClass` + `ghx.Error` *(proposed ADR-0034)* | Value | `internal/ghx` | `errors.As`, `Unwrap` | each frontend *maps* it (exit code / MCP payload / report field) |

Selection discipline (the "be selective" instruction): **`Question` stays a
`string`** — it is genuinely free text, has no invariant, and wrapping it buys
nothing. **`TurnResult` stays a transient DTO**, not an entity — it has no
identity or lifecycle; only its four boolean flags want folding (D5). The rest
earn a type because the concept appears in ≥2 places with hand-rolled
parse/validate/compare logic (`AGENTS.md`: "if a concept appears in two places
it deserves a type").

---

## Domain model — the objects that must flow

This is the direct answer table. "Where it flows" is the boundary crossing that
is a primitive today; "bug class removed" is the concrete payoff.

| Concept | Proposed Go type | Value / Entity | Where it must flow (boundary crossed) | Bug class it removes |
| --- | --- | --- | --- | --- |
| Repo identity | `ghx.Repo` (finish threading) | Value | frontend→sidecar→core→report (today reborn as `string` at each) | 3 divergent re-validations + a reachable `[1]` panic (AUD1 H1); repo passed unvalidated into a warm session |
| Repo snapshot | `ghx.Snapshot{Repo, SHA}` | Value | core read → `Evidence` → `Report` → trace | **non-reproducible evidence**: two reads see a moving HEAD; a committed report can't be re-fetched at the same commit |
| Recon depth | `sidecar.Depth` | Value | main-agent → `AskRequest` → budget resolver | reject-vs-coerce divergence (MCP rejects, CLI silently coerces) |
| Escalation tier | `sidecar.Tier` | Value | policy → `Report.TierUsed` → validator | literal sprawl across 5 sites; unenforced ordering for "highest tier used" |
| Evidence backend | `sidecar.Backend` | Value | grant → policy → `Report.BackendsUsed` | untyped `[]string` allowlist; `"local"` grant expansion hand-rolled |
| Inbound request | `sidecar.AskRequest` (typed fields) | Command/DTO | main-agent (MCP/CLI) → runtime | all-primitive command object; validation scattered across two frontends |
| Evidence report | `sidecar.Report` (immutable + presenter) | Value | sidecar → CLI / MCP / trace | frontends reach into loose fields (`cli/sidecar.go:718-721`) → coupling to field layout |
| Failure class | `ghx.FailureClass` + `ghx.Error` | Value | core → CLI / MCP / sidecar report | 3 non-interoperating string-matched taxonomies (ADR-0034) |
| Turn outcome | `sidecar.TurnOutcome` (fold 4 bools) | Value | runtime → eval anomaly derivation | boolean-soup instead of one classifiable outcome |
| Named-by origin | `sidecar.SessionNamedBy` | Value | route → `SessionMeta.NamedBy` | stringly-typed sibling of the typed `RouteSource` |

---

## High severity — the flow gaps

### D1 — `ghx.Repo` is constructed and destroyed inside core; it never crosses a boundary

**What.** The value object AUD1 recommended shipped, but only halfway. Core
*parses* a string at the top of each op instead of *receiving* a `Repo`, and no
object that carries a repo across a boundary uses the type.

**Evidence.**
- Core parses at each entry, discarding the caller's intent:
  `Explore` → `ParseRepo(repo)` (`internal/ghx/explore.go:22`),
  `Read` → `ParseRepo(repo)` (`internal/ghx/read.go:54`),
  `Tree` → `ParseRepo(repo)` (`internal/ghx/tree.go:15`),
  `Inspect` → `ParseRepo(repo)` (`internal/ghx/inspect.go:244`).
  The one internal function that *does* take the type — `fetchTree(repo Repo)`
  (`internal/ghx/glob.go:17`) — proves the ergonomics work; the public
  signatures simply don't match it.
- Every boundary-crossing carrier is still a bare `string`:
  `AskRequest.Repo string` (`internal/sidecar/runtime.go:224`),
  `SessionMeta.Repo string` (`internal/sidecar/session.go:17`),
  and the CLI hands core raw `args[0]` (`internal/cli/ghx.go:57`,
  `ghxlib.Explore(args[0], path)`).
- Net effect: the sidecar accepts an unvalidated repo string, stores it into a
  warm session (`SessionMeta.Repo`), and only when core is finally called does
  `ParseRepo` run — far from where a bad value entered. Validation happens at the
  *wrong* boundary.

**Why it violates a tenet.** `AGENTS.md` "Define proper domain models": a value
object that is parsed-and-discarded is primitive obsession wearing a type. The
concept still travels as a string; the type buys nothing on the wire.

**Recommendation.** Change the public core signatures to take `Repo` (or add
`ExploreRepo(Repo, …)` and deprecate the string overloads), make
`AskRequest.Repo` and `SessionMeta.Repo` of type `ghx.Repo` (empty `Repo{}` =
discovery mode, preserving the ADR-0019.1 sentinel), and **parse once** at the
two real boundaries: the CLI arg parse and the MCP `recon`/direct-tool decode.
This completes the work ADR-0035 item 1 started and makes the value object
actually flow. Small, mechanical, behavior-preserving (the validator is
byte-identical to `ParseRepo` today).

### D2 — `Snapshot` (repo @ commit SHA) is missing; reads track a moving HEAD and evidence is not re-fetchable

**What.** ghx has no concept of *"the repo as it was at one commit."* Reads
resolve the default branch fresh and read against whatever HEAD points to *now*;
the pinning that would make evidence reproducible does not exist as a type.

**Evidence.**
- No ref/SHA argument anywhere in the read path: `Read(repo string, files
  []string, opts *ReadOpts)` (`internal/ghx/read.go:49`) — nothing carries a
  commit. `Explore` and `Tree` likewise.
- The branch is re-resolved on every call from the *live* default ref:
  `resp.Repository.DefaultBranchRef.Name` (`internal/ghx/explore.go:98`) and
  `branchResp.Repository.DefaultBranchRef.Name` (`internal/ghx/glob.go:41`),
  then the tree endpoint is built against the mutable branch name
  (`git/trees/%s` with `branch`, `internal/ghx/glob.go:51`).
- `Evidence` records only a command + summary (`internal/sidecar/report.go:31-34`)
  — no commit. So `Report.Evidence` cannot be replayed deterministically: re-run
  the same `ghx read` a day later and HEAD may have moved.

**Why it matters (and why it is High).** This is the domain gap that touches the
project's *load-bearing* tenet, not a cosmetic one. NORTH_STAR: "Evidence, not
vibes… the sidecar may be opinionated because its trace is visible." AGENTS.md
Visibility/Truthfulness: "every score recomputable from committed artifacts."
ADR-0016.5 built replay-decontamination on the assumption that a recorded read
*is* what re-fetching yields — but without a pinned commit that assumption is
only true while HEAD is still. For a solo-dev product whose entire moat is
*auditable reconnaissance*, "the evidence cites a command but not the commit"
is a real correctness gap in the product's core promise.

**Recommendation (behavior-preserving first, then ADR-gated).** Introduce
`ghx.Snapshot{Repo, SHA}` (value object: `String()` → `owner/repo@sha`). **Phase
1 is observability-only and changes no read behavior:** core already resolves the
branch; also resolve the branch *tip SHA* it read against and thread it into
`Evidence` (`Evidence.WithSnapshot(Snapshot)`), so committed artifacts finally
record *which commit* produced them. This is purely additive and directly serves
the visibility tenet. **Phase 2 (optional, ADR-gated):** let a read *pin* to a
snapshot so an investigation reads a single consistent commit — this *changes*
what the agent sees and therefore touches the frozen measurement stack
(recon-identity, baseline reuse), so it must be pre-registered in an eval ADR,
never drive-by. The value object is worth introducing for Phase 1 alone.

---

## Medium severity

### D3 — the inbound `AskRequest` is an all-primitive command object; Depth/Tier/Backend want enums parsed at that boundary

**What.** `AskRequest` is *the* object that crosses main-agent → sidecar — the
one place the whole framework's contract is expressed — and every domain field
on it is a primitive. AUD1 (M2/M3/L1) already ranked the enums themselves; the
flow view adds *why this boundary specifically*: `AskRequest` is where these
values should be parsed once and typed, so no interior site re-validates.

**Evidence.**
- `AskRequest` (`internal/sidecar/runtime.go:218-243`): `Repo string`,
  `Depth string` (:227), `AllowedBackends []string` (:230), `Scope string`.
- **Depth** — reject-vs-coerce split, a direct symptom of no shared type: MCP
  *rejects* an invalid depth (`isValidReconDepth`, `internal/cli/serve.go:192-194,227`),
  the CLI *silently coerces* it (default `"normal"` at `internal/cli/sidecar.go:671`,
  forwarded verbatim at :95, no validation), and the budget map owns a *third*
  copy of the valid set with a silent fallback
  (`depthBudgets`, `internal/sidecar/session_options.go:159,195-197`).
- **Tier** — bare literals re-spelled everywhere: `deriveTierUsed` returns
  `"tier2"/"tier1"/"tier0"` (`internal/sidecar/tier.go:142-151`) and does *not*
  use the two-of-four constants `TierRemote`/`TierLocal`
  (`internal/sidecar/tier2/policy.go:38-39`); the valid set is re-listed at
  `internal/sidecar/reportsink.go:212,227,261`; carried as `string` on
  `Report.TierUsed` (report.go:52) and `EscalationDecision.FromTier`/`ToTier`
  (policy.go:105-106).
- **Backend** — `LocalBackendGrant = "local"` (policy.go:45); `[]string` on
  `AskRequest.AllowedBackends`, `EscalationObservations.AllowedBackends`
  (policy.go:82), and `Report.BackendsUsed` (report.go:53).

**Why it violates a tenet.** "Define proper domain models"; and the reject-vs-
coerce divergence is precisely the class of bug a single parse point eliminates.

**Recommendation.** Give `AskRequest` typed fields (`Repo ghx.Repo`, `Depth
Depth`, `AllowedBackends BackendSet`) and a `Validate()` method; each frontend
constructs it via one decoder so validation is uniform. `Depth.Budget()` becomes
the single owner of the budget map; `Tier` owns `Valid()`/ordering; `Backend`
owns `IsLocal()` and the grant expansion. This is AUD1 M2/M3 executed *at the
boundary object* rather than as scattered enum types.

### D4 — `Report` is a mutable bag that frontends reach into field-by-field; make it immutable + a presenter, and add the `FailureClass` field

**What.** `Report` is the outbound object crossing sidecar → CLI/MCP/trace, but
consumers reach into its loose fields with inline formatting, coupling every
frontend to the struct's field layout (AUD3's flag). It is also mutated after
construction and carries no failure class.

**Evidence.**
- CLI reaches straight into the fields with inline `fmt.Print`:
  `fmt.Println(report.Answer)` / `printClaims("Verified", report.Verified)` /
  `printRelevantFiles(report.RelevantFiles)` / `printStrings("Uncertainty",
  report.Uncertainty)` (`internal/cli/sidecar.go:718-721`), and the same pattern
  reaches into `SessionMeta` for the `sessions show` view (`meta.Name`,
  `meta.Repo`, `meta.TurnCount`, … `internal/cli/sidecar.go:377-400`).
- `Report` is mutated post-parse (`NormalizeNextReads`) and carries **no
  failure-class field** — the only place a failed turn can say *why* is the
  free-text `Uncertainty []string` (`report.go:55`), per the failure-class
  inventory §1f.

**Why it violates a tenet.** "Encapsulate domain logic… one authoritative
owner"; a value object whose presentation is hand-inlined at each consumer has no
owner of its own rendering. And `AGENTS.md` "if a concept appears in two places
it deserves a type" — failure *class* is such a concept, absent from the report.

**Recommendation.** Treat `Report` as **immutable after construction** (build it
once in the runtime, never mutate a returned report) and give it a
`FormatText()` presenter method; the CLI calls `report.FormatText()` instead of
reaching into fields (mirrors the existing `FormatInspectText` precedent AUD/arch
M1 cites). Same for `SessionMeta.FormatText()`. Add the ADR-0034 `FailureClass`
as an additive field so a failed turn classifies *why* in the contract, not only
in a log line. **Constraint:** this is the report *contract*; per ADR-0034 §1d/B5
and the failure-class inventory, the tool *schema* hash must stay byte-identical
(a new report field is additive and parser-safe per `report_test.go`), and any
change to the persona/report identity bytes is frozen-measurement territory —
the presenter and immutability are pure refactors, but the new field is
additive-only and must not alter existing golden output.

---

## Low severity

### D5 — `TurnResult`'s four boolean flags want one `TurnOutcome` value object

**What.** `TurnResult` classifies a turn's outcome as four independent booleans
instead of one value object: `ReportRetried` (`turnresult.go:39`),
`ReportCoerced` (:46), `WrapUpRecovered` (:51), `SessionRecreated` (:55), plus a
free `Error` string set in `runtime.go`. The failure-class inventory §1e already
noted these are "boolean flags, not a class." Four bools = 16 states, most
meaningless; the eval anomaly layer then re-derives meaning from persisted error
strings.

**Recommendation.** Fold the flags into a small `TurnOutcome` value object (a
typed status + the retained flags as methods), so a turn's disposition is one
classifiable thing. **Careful:** the marker strings the eval layer derives
anomalies from are an artifact-recompute contract (failure-class inventory B4) —
`TurnOutcome` must be threaded *in addition to*, not *instead of*, the existing
flags/markers. Low priority; do it when the runtime turn engine is already open
(ADR-0035 item 3 consolidates that engine).

### D6 — `SessionMeta.NamedBy` is a bare string beside the typed `RouteSource`

**What.** `SessionMeta.NamedBy string` (`session.go:30`) and the
`SessionNamedExplicit`/`Repo`/`Question` bare consts (`route.go:34-36`) sit
directly next to the exemplary typed `RouteSource` enum (`route.go:18-27`) yet
are left untyped, and `RouteDecision.sessionNamedBy()` returns a bare string
(`route.go:214-223`). AUD1 L1 already flagged this.

**Recommendation.** Introduce `SessionNamedBy` as a typed enum mirroring
`RouteSource`; opportunistic, near-free, do it when `route.go`/`session.go` are
open. Not urgent.

---

## Where the types live — the package-boundary lens (types only)

Coordinating conceptually with the boundaries question but staying in my lane
(the **types**, not the package graph): a value object should be **owned by the
package that owns the concept, and imported downstream** — and the direction of
type-flow must match the dependency direction the architecture audit verified
healthy (frontends and sidecar import core; the CLI imports the sidecar; nothing
imports upward).

- **Core (`internal/ghx`) owns the GitHub-recon vocabulary:** `Repo`, `Snapshot`,
  `FailureClass`/`Error`, and the result value objects (`FileEntry` typed with a
  `FileType`). These are recon-domain nouns; the sidecar and every frontend
  import *inward* to use them. This is the ADR-0010 direction made concrete at
  the type level: core defines the nouns, frontends wrap them.
- **Sidecar (`internal/sidecar`) owns the SAF-runtime vocabulary:** `Depth`,
  `Tier`, `Backend`, `AskRequest`, `Report`, `SessionMeta`, `RouteDecision`,
  `TurnResult`. These are framework-runtime concepts, not core recon, so they
  stay in the sidecar — but they should consume the core value objects (a
  `Report` cites `Snapshot`; an `AskRequest.Repo` is a `ghx.Repo`).
- **Deliberate non-move (ADR-0035 posture, AUD4 V3):** `Report`, `AskRequest`,
  and the persona are GitHub-specific and welded into the sidecar package. Do
  **not** extract a generic "framework" value-object layer now — that fails the
  north-star filter. The only obligation is to stop *adding* new GitHub-specific
  coupling. Concretely for types: keep `Tier`/`Backend` in the sidecar until P3's
  tier-2 tools actually force a shared home; do not pre-hoist them into core on
  spec.

The clean result: **type ownership recapitulates the dependency graph.** A repo
identity born in core flows outward through the sidecar's `AskRequest` and back
in `Report.Evidence` as a `Snapshot`, and no consumer ever re-parses a string it
was handed.

---

## Recommended sequence (phased, non-rewrite)

Ranked by real payoff for a solo dev, each item behavior-preserving and
test-pinnable. Items already sequenced by ADR-0035 are cross-referenced, not
duplicated; the *new* framing each adds is the flow/immutability lens.

1. **Finish the `ghx.Repo` flow (D1).** Change public core signatures to take
   `Repo`; type `AskRequest.Repo` and `SessionMeta.Repo`; parse once at the CLI
   and MCP boundaries. Completes ADR-0035 item 1; smallest change, moves
   validation to the correct boundary, and every future P3 repo-scoped tool
   inherits the type. **Do first.**

2. **`ghx.Snapshot`, Phase 1 (D2) — observability-only.** Resolve and thread the
   read's tip SHA into `Evidence`; add `Snapshot{Repo, SHA}`. Purely additive,
   changes no read behavior, and closes a real gap in the product's core
   auditability promise. Highest *product-correctness* return in this document.
   (Phase 2 pinning is deferred behind an eval ADR — frozen measurement.)

3. **Type `Depth`, `Tier`, `Backend` at the `AskRequest` boundary (D3).** One
   value type each, one validator, one budget/ordering owner; parse in the
   frontend decoders. Fixes the depth reject-vs-coerce divergence as a side
   effect. Executes ADR-0035 item 7 with the boundary-object framing.

4. **Immutable `Report` + presenter, and the `FailureClass` field (D4).** Give
   `Report`/`SessionMeta` a `FormatText()` so the CLI stops reaching into loose
   fields; treat `Report` as construct-once-immutable; add the ADR-0034
   `FailureClass` as an additive field. Pairs with ADR-0035 item 6 and item 8
   (the ADR-0034 integration). **Additive field only; golden/report-identity
   bytes must not drift outside a pre-registered eval ADR.**

5. **Fold `TurnResult` flags → `TurnOutcome` (D5) and type `SessionNamedBy`
   (D6).** Opportunistic, do inside ADR-0035 item 3 (turn-engine consolidation)
   and any `route.go` edit respectively. Keep marker strings alongside the type
   (artifact-recompute contract, inventory B4). Lowest priority.

**Non-goals (respecting the frozen measurement stack):** no change to eval
scoring, gates, detectors, persona bytes, or the recon tool *schema* hash. D2
Phase 2 (pinned reads) and any `Report`/persona identity-byte change are
ADR-gated. Items 1, 3, and the presenter/immutability half of 4 are pure
behavior-preserving refactors, pinnable by existing tests + goldens.

## Method / auditability

- **Who produced this:** a read-only domain-modeling audit worker. No code was
  modified; the only file written is this document, under `docs/audits/`.
- **Commit basis:** citations resolve at **mainline `1bda884`** (mainline tip at
  audit time). This worker's own worktree is based at `d2cda8a` (v2.7.0), which
  *predates* `internal/ghx/repo.go`, the AUD1 design-patterns audit, and the
  ADR-0034 failure-class inventory; those exist only from later mainline commits,
  so every `file:line` above was verified against the mainline checkout, not the
  worktree base. The worktree ref was left untouched (nothing merged or pushed),
  matching the failure-class inventory's basis convention.
- **How to recompute the key claims:**
  - `ghx.Repo` parsed-not-received: `grep -rn 'ParseRepo(' internal/ghx/*.go` (5
    interior parse sites) vs. `grep -rn 'Repo string' internal/sidecar/runtime.go
    internal/sidecar/session.go` (boundary carriers still string).
  - Snapshot absence: `grep -rn 'defaultBranchRef\|DefaultBranchRef' internal/ghx`
    (branch resolved fresh) and confirm `Read`/`Explore`/`Tree` signatures take no
    ref (`internal/ghx/read.go:49`, `explore.go`, `tree.go`).
  - Depth reject-vs-coerce: `internal/cli/serve.go:192-194,227` (reject) vs.
    `internal/cli/sidecar.go:671,95` (coerce) vs.
    `internal/sidecar/session_options.go:159,195-197` (third valid-set copy).
  - Tier/Backend literals: `grep -rn '"tier[0-3]"' internal/sidecar` and
    `internal/sidecar/tier2/policy.go:38-45`.
  - CLI reaching into loose fields: `internal/cli/sidecar.go:377-400,718-721`.
- **Tenets audited against:** `AGENTS.md` Engineering Tenets — "Define proper
  domain models," "Encapsulate domain logic in services," "Composition by use
  case"; NORTH_STAR "Evidence, not vibes" and the Visibility/Truthfulness
  recomputability rule; ADR-0010 core-owns-the-vocabulary; ADR-0035 refactor
  sequence; ADR-0034 failure-class; the failure-class inventory's B1-B6
  preservation contracts.
- **Prior work extended, not repeated:** AUD1
  (`design-patterns-domain-modeling-2026-07-07.md`) owns primitive-obsession-at-
  rest and the `ghx.Repo`/`Depth`/`Tier`/`Backend` type recommendations; this
  audit adds the *flow / immutability / boundary-object* lens and the two new
  findings AUD1 did not surface: `ghx.Repo` failing to cross the boundary (D1)
  and the missing `Snapshot` value object (D2).
- **Read-only integrity:** no build or test run was needed (no code changed);
  all findings are static, traced by reading. `go build ./...` was green on
  mainline at the cited basis (per the architecture and design-patterns audits'
  concurrent verification).
</content>
</invoke>
