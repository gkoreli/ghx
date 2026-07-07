---
title: "Adversarial Velocity Red-Team — where the architecture will slow the P3/P4 roadmap"
date: "2026-07-07"
status: "audit"
author: "adversarial red-team audit worker"
scope: "internal/**, cmd/**, plus NORTH_STAR/ADRs for roadmap grounding — read-only, no code changes"
---

# Adversarial Velocity Red-Team — 2026-07-07

A maximally contrarian, read-only audit whose job is to argue the current
architecture **will slow us down against the roadmap**, then prove it with
`file:line` evidence and name the unblocking refactor. The lens is **tech debt
as a velocity killer**, measured against the four things the North Star says we
build next: P3 "swallow the tools" (M7), P4 "below ACP" (M9/M10), a second
reconnaissance domain (the "framework outgrows ghx" consequence), and the
compounding cost of debt over the next 1–3 months.

Companion to `docs/audits/architecture-2026-07-07.md`, which found the
core/frontend boundary healthy and ranked *correctness* debt. This audit does
the opposite job: it assumes the boundary is fine (it is) and asks **"what in
here makes the next four roadmap moves expensive?"** Findings are ranked by
velocity-cost, not correctness. Already-actioned findings from the companion
audit are **not** repeated (its H2 acp.go god-file split is done —
`acp.go` is now 471 lines with `turnresult.go`/`tooltrace.go`/`denyclient.go`/
`reportsink_exe.go` as siblings; its M2 failure-taxonomy is captured in
**ADR-0034, status `proposed`**, awaiting Goga — not built, and deliberately
out of scope here).

Red-team honesty guard: a red-team that cries wolf is useless. Section
**"What is actually fine"** records the supposed blockers that are *not* real,
with the same evidence rigor, so the High/Medium findings carry weight.

## Executive summary

**Three structural welds will tax the roadmap, and one of them —
V1 — is the direct P3 blocker.** The sidecar has **no owned tool registry**:
the "brain" is an external ACP agent that learns its tools from ~130 lines of
English persona prose (`internal/sidecar/prompt.go:49-176`) and reaches Tier-2
tools as CLI shell-outs; the runtime discovers what the agent did *post-hoc by
string-parsing stderr* (`internal/sidecar/tier.go:19`, `:110-119`). Absorbing
one new tool under P3 ("codemap, ast-grep, repomap all become internal tools
under one brain") is therefore a **6-place, measurement-identity-coupled edit**
— and the repo already contains the abstraction it should have used
(`internal/codemode/registry.go`'s data-driven `Registry`), applied to the CLI
`code` tool but never to the sidecar's own tools. That divergence is what makes
M7's "grow the tool set" cadence slow.

**P4 "below ACP" is not blocked by ACP — it's blocked by the adapter leaking
past ACP.** The ACP *wire* is cleanly isolated (5 non-test files; zero leak
into `cmd/**` or other packages — genuine good news). But the *steering
contract* is welded to one specific adapter+SDK: `session_options.go` opens
with "This type is **ADAPTER-SPECIFIC**: it targets the claude-agent-acp
adapter" (`internal/sidecar/session_options.go:10-11`) and hardcodes
`claude-agent-acp@0.55.0` internals down to `acp-agent.js:2768–2801` line
numbers, emitting a `_meta.claudeCode` bag (`session_options.go:222`). The
recovery machinery that the dogfood week depended on triggers on the adapter's
English error text (`acp.go:59-64`, `IsMaxTurnsError` `acp.go:78-80`). Serving a
trained model (M10, P4 exit trigger 4) means **no claude-agent-sdk at all**, so
all of that is dead code to be re-authored — and there is no harness interface
seam that a non-Claude runtime could implement; `RunTurnOptions.SessionMeta` is
an opaque `map[string]any` (`acp.go:164-171`) only `BuildSessionMeta` knows how
to fill.

**A second recon domain forces a copy-paste fork of `package sidecar`.**
Everything framework-worthy (the warm-worker pool, session/route/ledger,
telemetry, the ACP harness, resilience) is welded into one flat package
alongside ghx code-recon semantics: a concrete code-recon `Report`
(`report.go:38-57`), an `AskRequest.Repo` that is a GitHub `owner/repo`
(`runtime.go:221`), and a persona `const` for a "GitHub repository
reconnaissance agent" (`prompt.go:39`). None of it is behind an interface, so a
web-browser sidecar reuses the runtime only by forking the package.

**The compounding mechanism, in one sentence:** every one of these is *cheap to
fix now, with one adapter / one domain / three tools, and structurally more
expensive after each ghx increment* — because each new `Report` field, each
persona revision, and each absorbed tool deepens the exact welds that P3/P4/the
second domain must later cut. Ranked by 1–3 month velocity-cost:
**V1 (tool registry) > V2 (adapter-not-ACP leak) > V3 (framework/domain weld) >
V4 (error-string recovery fragility).**

### What is actually fine (red-team honesty — verified, not assumed)

- **The ACP *wire* is genuinely swappable; only 5 files touch the SDK.**
  `grep -l 'coder/acp-go-sdk' internal/sidecar/*.go` (non-test) →
  `acp.go`, `daemon_worker.go`, `denyclient.go`, `reportsink_exe.go`,
  `tooltrace.go`. **Zero** ACP imports in `cmd/**` or any package outside
  `internal/sidecar`. The protocol is confined to the harness layer exactly as
  the "protocols are stepping stones" tenet wants. The P4 problem below is the
  *adapter*, not the *protocol* — an important distinction the pessimist must
  concede.
- **Core (`internal/ghx`) will not move for P3/P4 at all.** It imports no
  adapter, no telemetry, no config (confirmed by the companion audit and
  re-checked). All roadmap churn is confined to `internal/sidecar`, so the
  blast radius of V1–V4 is one package, not the codebase.
- **`tier2.Service` is clean *internally*.** The absorbed-tool substrate has a
  single shared subprocess body (`tier2/toolrun.go:53-126` `subprocessAdapter`)
  and one owner per concern; the debt (V1) is the *absence of a registry above
  it*, not the Service's own quality. Do not refactor `tier2/*` internals.
- **The domain *models* are portable even if their serialization isn't.** The
  depth→budget table (`session_options.go:159-166`), the persona doctrine, and
  the `Report` contract are sound, harness-neutral concepts. P4 is a rewrite of
  *placement/encoding*, not of the brain's logic — so it is bounded, not a
  from-scratch rebuild. Claiming otherwise would be crying wolf.
- **The companion audit's M2 (failure taxonomy) is already on the record** as
  ADR-0034 (`proposed`). This audit does not re-litigate it; note only that
  ADR-0034 unifies *error rendering across frontends* and does **not** address
  the tool-registry, adapter-coupling, or domain-weld problems below — they are
  orthogonal and uncovered.

---

## High severity

### V1 — No sidecar tool registry: P3 tool absorption is a 6-place, eval-coupled edit

**What.** P3/M7 is "swallow the tools": codemap, local clone, ast-grep, repomap
"all become internal tools of the sidecar brain … the outside world talks only
to the sidecar" (`docs/NORTH_STAR.md` P3 row). But the sidecar brain owns **no
tool registry**. It is an *external* ACP agent (`claude-agent-acp`) that:
1. learns which tools exist only from English persona prose, and
2. invokes Tier-2 tools as `ghx tier2 …` **shell commands**, after which
3. the runtime figures out what happened by **string-parsing the agent's
   captured stderr**.

There is no data structure the sidecar owns that says "these are my tools."
Adding the next tool is a hand-synchronized edit across six sites.

**Evidence.**
- **The tool "registry" is prose.** The persona hardcodes the exact Tier-2
  command menu — codemap/astgrep/repomap by hand, with their backend IDs — in a
  system-prompt string: `internal/sidecar/prompt.go:145-154`
  (`7. Escalate to Tier 2 …` followed by the three literal
  `ghx tier2 codemap|astgrep|repomap …` lines). The brain knows a tool exists
  only because this English paragraph names it.
- **The substrate is a hardcoded struct, not a registry.**
  `internal/sidecar/tier2/service.go:180-198` — `Service` has fixed fields
  `Codemap *Codemap` and `AstGrep *AstGrep`; adding a tool means adding a field.
  `service.go:367-405` — one bespoke method **per tool**
  (`RunCodemap`, `RunAstGrep`, `RunRepomap`), each with its own `*Request` type
  and `ArtifactKey`. There is no `Register(tool)` and no `[]Tool` to range over.
- **The runtime cannot intercept tool calls; it reconciles them from text.**
  `internal/sidecar/tier.go:19` states it outright: *"the sidecar reaches tier2
  via shell commands inside the ACP turn, so the runtime records and reconciles
  rather than intercepts."* Detection is string matching: `tier.go:140`
  ("tier2 when a `ghx tier2` command ran"), and clone provenance is recovered by
  scanning captured output (`tier.go:110-119` → `tier2.FindProvenance`,
  itself a hand-written stderr parser at `tier2/service.go:121-151`).
- **Canonical tool IDs are duplicated in prose contracts.** The backend-ID set
  is embedded in the `submit_report` tool description
  (`internal/sidecar/reportsink.go:262`: *"canonical IDs: "remote",
  "local:codemap", "local:ast-grep", "local:repomap""*) and re-stated in the
  `Report` godoc (`report.go:50-51`). A new backend must be added to both, and
  the report validation/reconciliation that reads `BackendsUsed`
  (`tier.go:186`) must learn it.
- **The abstraction already exists in-repo, unused by the sidecar.**
  `internal/codemode/registry.go:18-79` is a real data-driven registry —
  `NewRegistry`/`Register`/`Get`/`List`/`Search`/`Tools` over a
  `map[string]Tool` (`registry.go:9-16` `Tool{Name, Description, Schema, …}`).
  `ghx.RegisterTools` populates it and the CLI/MCP `code` tool consumes it
  (companion audit, "CLI and MCP share one executor + one tool registry"). The
  pattern is proven here — the sidecar's own tool surface simply doesn't use it.

**Why it blocks the roadmap.** M7's value is *cadence*: "Codemap is popular
today; in this future it is just one tool under ghx" — the thesis needs many
tools absorbed cheaply. Today absorbing one is a 6-file change
(`tier2/service.go` field+method, a new `*Request`, `cli/tier2.go` cobra
command, `prompt.go` persona prose, `tier.go`/`report.go`/`reportsink.go`
canonical-ID + detection strings, `tier2/service.go` provenance format). Two of
those six are **measurement-identity changes**: editing the persona is a
persona revision that must be pre-registered before a gate run
(ADR-0029 / workstream B8), and the `submit_report` backend-ID vocabulary is
part of the report contract evals score against. So "add a tool" silently means
"touch the frozen measurement stack," which the AGENTS.md frozen-stack rule
makes deliberately slow. That coupling — tool growth entangled with eval
identity — is the compounding mechanism: it gets worse with every tool and
every persona rev.

**Tenet ties.** NORTH_STAR "The moat is the agentic brain, not the tools" and
the P3 row (tools "disappear into thin air" under one brain) — you cannot make
tools disappear cleanly without a registry to disappear them into. AGENTS.md
Engineering Tenets "one authoritative owner per domain concern": the sidecar's
tool set today has *five* partial owners (persona prose, Service struct, CLI
commands, tier reconciliation strings, report-contract IDs) and no authoritative
one.

**Recommendation.** Introduce a sidecar-owned `ToolSpec` registry — one
declaration per tool carrying: canonical backend ID, invocation recipe, the
persona snippet, and the detection matcher. Generate (a) the persona's Tier-2
menu, (b) the CLI subcommand wiring, and (c) the `tier.go` reconciliation
matcher from that single source, so the six hand-synced sites collapse to one.
Model it on `codemode/registry.go`, which already proves the shape in-tree. This
passes the North-Star filter (makes reconnaissance tooling cheaper to grow and
keeps the audit trail authoritative) and is the highest-leverage P3 unblock.

### V2 — The *adapter*, not ACP, leaks through the steering surface; "below ACP" is a steering-layer rewrite, not a swap

**What.** The tenet is explicit: *"the boundary must survive replacing ACP, the
brain, or both (P4)"* and P4 "means dropping to an existing agent-SDK harness we
configure — owning context assembly, tool registry, and model choice"
(`docs/NORTH_STAR.md` tenet "Protocols are stepping stones"). The ACP *wire* is
isolated and swappable (see "What is fine"). But the **steering/permission/
recovery** layer is welded to one specific adapter + SDK, and that weld — not
ACP — is what P4 must cut. There is also no interface seam a non-Claude harness
could implement.

**Evidence.**
- **The steering options type declares its own adapter-lock.**
  `internal/sidecar/session_options.go:10-11`: *"This type is ADAPTER-SPECIFIC:
  it targets the claude-agent-acp adapter."* Its fields are annotated with
  `claude-agent-acp@0.55.0` source line numbers —
  `acp-agent.js:2768–2801` (`session_options.go:7-8`), `acp-agent.js:2748`
  (`:18-19`), `acp-agent.js:2800` (`:27`, `:64-65`), `acp-agent.js:2822`
  (`:210-212`), and `sdk.d.ts:1305/1367/1889-1895` (`:52-63`). This is a domain
  type shaped entirely by one vendor's minified JS internals.
- **The wire meta is keyed on the vendor.** `BuildSessionMeta`
  (`session_options.go:194-230`) emits `map[string]any{"claudeCode": {"options":
  …}}` (`:222-229`). The entire steering surface — persona, isolation, budgets,
  model pin, audit flag — is a `claudeCode`-namespaced blob. A different harness
  shares none of this encoding.
- **Permission logic encodes the adapter's tool taxonomy.**
  `internal/sidecar/denyclient.go:161-166` maps auto-approval decisions off
  *"the adapter classifies all MCP tools as kind 'other' (claude-agent-acp
  tools.js default case)."* `isWriteToolKind` (`denyclient.go:123-132`) and
  `isSubmitReportTool` (`:138-144`) depend on how *this* adapter titles and
  classifies calls.
- **Recovery rides on the adapter's English error text.**
  `internal/sidecar/acp.go:59-64` defines `maxTurnsErrorMarker =
  "Reached maximum number of turns"` and `peerClosedMarker`; `IsMaxTurnsError`
  (`acp.go:78-80`) and `IsPeerClosedError` (`:84-90`) `strings.Contains` on
  them; `runtime.go:435` gates the whole ADR-0027 D1 wrap-up recovery on
  `IsMaxTurnsError`. This is Claude-adapter phrasing, matched textually.
- **No harness interface seam exists.** The one plausible seam,
  `RunTurnOptions.SessionMeta`, is `map[string]any` (`acp.go:164-171`) built
  only by `BuildSessionMeta`; `TurnRunner` (`runtime.go:18-20`) is
  `func(ctx, RunTurnOptions) (TurnResult, string, error)` — a function alias,
  not an interface a second harness type implements. Swapping the harness means
  editing `RunTurnWithOptions`/`AgentWorker` in place, not registering an
  alternative.

**Why it blocks the roadmap.** P4 exit trigger 4 is "serving a custom-trained
model (M10), which no existing ACP adapter will speak for us"
(`docs/NORTH_STAR.md`). At that moment every `claudeCode`-namespaced field in
`session_options.go`, the `denyclient.go` permission taxonomy, and the
string-matched recovery become dead and must be re-authored against the new
harness — with no interface to slot into. The domain *contract* (persona,
budgets, isolation, model) is portable, so this is bounded, but it is a rewrite
of the steering layer under time pressure at exactly the milestone where the
model-serving work should dominate attention. Doing the extraction now, while
there is exactly one adapter to model, is far cheaper than after M10 forces it.

**Tenet ties.** "Protocols are stepping stones, not identity … the boundary must
survive replacing ACP, the brain, or both." Today it survives replacing the
*protocol* but not the *brain*: replace the Claude adapter and the steering,
permission, and recovery layers all break. Open-source-leverage tenet supports
the fix shape — a thin `Harness` interface *over* adopted SDKs is not "inventing
a protocol," it is the seam the tenet already anticipates ("owning context
assembly, tool registry, and model choice").

**Recommendation.** Define two harness-neutral types and one interface:
`SteeringSpec` (persona, depth budget, isolation, model, tool allowlist —
lifted verbatim from today's fields) and a neutral `TurnResult` (already close;
see V-note in "What is fine"), behind a `Harness` interface
(`Start/NewSession/Prompt/Resume`). Make `claude-agent-acp` the first
implementation by moving the `_meta.claudeCode` encoding, the permission
taxonomy, and the error-string classifiers *inside* it. `TurnRunner` becomes the
interface method. This is prerequisite work for M9/M10 and passes the North-Star
filter (a swappable harness is what lets the trained model own its context and
tools).

### V3 — Framework and ghx domain are welded in one flat package; a second recon domain forces a copy-paste fork

**What.** NORTH_STAR: *"the framework will outgrow ghx — later. The same sidecar
pattern applies to web-browser exploration and many other domains"*
(and Consequence Product "Sidecar brains for other domains"). Today there is
**no package boundary** between the reusable framework (warm-worker pool,
session/route/ledger, telemetry, ACP harness, resilience, report-sink) and the
ghx code-reconnaissance domain (persona, report schema, GitHub repo scope,
Tier-2 clone). Both live in one flat `package sidecar` with the domain concepts
hardcoded, not injected. A second domain reuses the runtime only by forking.

**Evidence.**
- **The report is a concrete code-recon struct, used everywhere as a type (not
  an interface).** `internal/sidecar/report.go:38-57` — `Report{Answer,
  Verified, Inferred, Unverified, RelevantFiles, Evidence, TierUsed,
  BackendsUsed, CommandsRun, Uncertainty, NextReads}` — every field is code-recon
  vocabulary (files, backends, tiers, next *reads*). It is threaded concretely
  through the ledger, report-sink, tier reconciliation, and telemetry emit — not
  behind a domain interface. A browser sidecar's "report" (URLs, DOM findings,
  screenshots) cannot reuse the runtime without either forking `Report` or
  generic-izing ~all its consumers.
- **The request hardcodes GitHub scope.** `internal/sidecar/runtime.go:221-224`
  — `AskRequest.Repo` is a GitHub `"owner/repo"`; routing, session naming
  (`route.DetectedRepo`, `runtime.go:309`), and discovery mode all key off it.
  A domain with no "repo" concept must fork the request/route model.
- **The persona is a package-level `const`-returning function, not injected.**
  `internal/sidecar/prompt.go:37-178` `BuildPersonaSystemPrompt` is a hardcoded
  "You are ghx-sidecar, a specialized GitHub repository reconnaissance agent"
  string (`prompt.go:39`). The runtime calls it directly (`runtime.go:374-377`).
  There is no `Persona` seam a second domain plugs a different doctrine into.
- **Everything shares one flat package namespace.** The reusable pieces
  (`AgentPool`/`AgentWorker` `daemon_worker.go:17-114`, `Ask` `runtime.go:260`,
  telemetry, route, ledger, resilience) are `package sidecar` siblings of the
  domain pieces. There is no `framework/`-vs-`ghxrecon/` split to lift the reusable
  half out of.

**Why it matters (and why it's ranked below V1/V2).** This is a *Consequence
Product* — explicitly "never worth steering by" (NORTH_STAR). So a full
framework extraction now would fail the North-Star filter and is correctly *not*
recommended. The velocity risk is subtler: the weld **deepens with every ghx
increment**. Each new `Report` field for a ghx feature, each `AskRequest`
addition, each persona rev makes the eventual extraction wider. The compounding
cost is the growth in the seam's diameter, paid later at fork time.

**Tenet ties.** NORTH_STAR "The framework will outgrow ghx" + Consequence
Product "Sidecar brains for other domains." AGENTS.md "define proper domain
models / one owner": today "the framework" is not a modeled concept at all.

**Recommendation.** Do **not** extract the framework now (premature; fails the
filter). Do the cheap, filter-passing thing: **stop deepening the weld.** When
touching `Report`, `AskRequest`, or the persona for a ghx feature, keep the new
surface behind a small seam (a `Persona` provider, a domain-tagged report
envelope) so a future extraction is bounded rather than open-ended. This is a
posture, not a project — and it costs nothing today while capping the future
fork cost.

---

## Medium severity

### V4 — Failure recovery keyed on the adapter's English error strings is a silent-breakage trap

**What.** The ADR-0027 resilience machinery — the wrap-up recovery and dead-peer
fast-fail that the dogfood week's breaking items depended on — is triggered by
`strings.Contains` matches against the adapter's human-readable error text. An
upstream reword silently disables recovery while tests stay green.

**Evidence.**
- `internal/sidecar/acp.go:59-64` — `maxTurnsErrorMarker = "Reached maximum
  number of turns"` and `peerClosedMarker = "peer connection closed"` are
  literal adapter/SDK phrasings, with the comment admitting the match "is
  textual because the error arrives as an opaque JSON-RPC internal error"
  (`acp.go:76-77`).
- `IsMaxTurnsError` (`acp.go:78-80`) and `IsPeerClosedError` (`:84-90`) drive
  the recovery branch at `runtime.go:435` (`if err != nil && IsMaxTurnsError(err)`
  → the whole wrap-up path `runtime.go:435-482`).
- Only `IsLoadSessionResourceNotFound` (`acp.go:96-106`) matches a *structured*
  JSON-RPC code (`-32002`) alongside text — the max-turns and peer-closed paths
  have no structured anchor.

**Why it matters.** `claude-agent-acp` is pinned at `@0.55.0` today, but P4's
own exit trigger 1 is "adapter opacity keeps taxing us" — the adapter *will*
change or be replaced. A version bump rewording "Reached maximum number of
turns" turns off the wrap-up recovery with no failing test (tests assert the same
literal string, so they pass while production regresses). It compounds directly
with V2 (same adapter coupling), and it is a live-fragility issue independent of
P4. Ranked Medium, not High, because it is currently correct and guarded — but
the guard is load-bearing on a string the vendor owns.

**Tenet ties.** "Protocols are stepping stones … the boundary must survive
replacing ACP." Recovery that breaks silently on an adapter reword does not
survive it. Visibility/Truthfulness: a silently-disabled recovery path fails
loudly in production but not in the test suite — the inverse of what the tenet
wants.

**Recommendation.** Anchor recovery on something structural, not English:
prefer a JSON-RPC error *code* where the adapter emits one, and where it does
not, add a handshake-time adapter-identity/version assertion (the version is
already known — `acp.go:21-25` writes the pinned `npx … claude-agent-acp@0.55.0`
line) so a drift in adapter version fails a preflight check loudly instead of a
recovery path quietly. Fold this into V2's `Harness` boundary — the adapter
implementation owns its own error-classification.

---

## Low severity

### V5 — `ToolCallTrace` and the accounting helpers are ACP-wire-shaped (minor P4 tax)

**What.** The sidecar's evidence/accounting types mirror ACP tool-call
notification shapes and two helpers take ACP SDK types directly. This is a small,
real P4 tax but far below V2 in cost.

**Evidence.** `internal/sidecar/turnresult.go:79-94` — `ToolCallTrace{ID, Kind,
Title, RawInput, Locations, StatusTransitions, …}` are the ACP `tool_call` /
`tool_call_update` wire fields; the godoc says so (`:79-81`). `tooltrace.go:13`
and `:35` — `ContentSize(content []acp.ToolCallContent, …)` and
`ToolOutputText(content []acp.ToolCallContent, …)` take the ACP SDK type as a
parameter, so a harness swap changes their signatures.

**Why it's Low.** These are audit/accounting conveniences, not the brain's
contract, and the field *set* (an id, a kind, touched locations, a status
timeline, an output size) is genuinely harness-neutral once the two
`acp.ToolCallContent` parameters are replaced with a small local struct. Bundle
it into V2's neutral-`TurnResult` work; not worth a standalone change.

**Recommendation.** When V2 lands, have the `Harness` implementation translate
adapter notifications into a harness-neutral `ToolCallTrace`, and change the two
`tooltrace.go` helpers to accept a local content type. No behavior change.

---

## Recommended sequence (top refactors, ordered by velocity-cost)

Ranked by how much each unblocks the *next 1–3 months* of roadmap building, and
by the compounding cost of leaving it.

1. **V1 — Give the sidecar a real tool registry (P3 unblock).** One `ToolSpec`
   per tool (backend ID + invocation + persona snippet + detection matcher);
   generate the persona menu, CLI wiring, and `tier.go` reconciliation from it.
   Model on `codemode/registry.go`, which already proves the shape in-tree.
   **Compounding mechanism:** without it, every M7 tool is a 6-place edit, two of
   which touch the frozen measurement stack — the tax grows per tool and per
   persona rev. Highest leverage; do first.
2. **V2 — Extract a `Harness` interface and a harness-neutral `SteeringSpec`
   (P4 prerequisite).** Move the `_meta.claudeCode` encoding, the permission
   taxonomy, and the error-string classifiers behind a `claude-agent-acp`
   implementation of one interface. **Compounding mechanism:** every persona/
   budget/isolation feature added before this deepens the `claudeCode` weld that
   M10 must cut under time pressure. Do while there is exactly one adapter.
3. **V4 — Anchor recovery on structured signals, not English (fold into V2).**
   Adapter-version preflight assertion + JSON-RPC codes where available, so an
   adapter reword fails loudly, not silently. Cheap once V2's boundary exists.
4. **V5 — Neutralize `ToolCallTrace` / accounting helpers (fold into V2).**
   Translate adapter notifications into a local trace type at the harness edge.
5. **V3 — Hold the framework/domain weld line (posture, not project).** Do not
   extract the framework now (fails the North-Star filter). Do keep new `Report`
   / `AskRequest` / persona surface behind small seams so the eventual
   second-domain fork stays bounded. Ongoing discipline, zero cost today.

Note on ordering vs. the frozen-measurement rule: V1's registry refactor is a
pure internal restructuring that must reproduce the *current* persona and
backend-ID output byte-for-byte (no persona-content change) so it is **not** a
measurement-identity change and needs no gate re-run — verify with a golden test
that `BuildPersonaSystemPrompt()` output is unchanged. Any *new* tool added
through the new registry is a separate, pre-registered persona revision as
today. Keep the mechanism change and the content change in different commits.

## Method / auditability

- **ACP-leak surface** from `grep -l 'coder/acp-go-sdk' internal/sidecar/*.go`
  (non-test) → 5 files; per-file reference counts via `grep -c 'acp\.'`
  (denyclient.go 32, acp.go 18, daemon_worker.go 15, reportsink_exe.go 6,
  tooltrace.go 2). Cross-package leak checked with
  `grep -rn 'coder/acp-go-sdk' internal cmd --include='*.go' | grep -v
  internal/sidecar/` → empty.
- **tier2 importers** from `grep -rln 'sidecar/tier2"' internal cmd
  --include='*.go'` → the sidecar *runtime* touches tier2 only via `tier.go`
  (post-hoc reconciliation) and `preflight.go`; the tools themselves are invoked
  as CLI shell-outs, confirming V1's "no in-process registry" claim.
- **Registry-pattern contrast** by reading `internal/codemode/registry.go`
  (data-driven) against `internal/sidecar/tier2/service.go` (hardcoded
  per-tool methods) and `internal/sidecar/prompt.go` (prose tool menu).
- **Adapter-coupling** read directly from `internal/sidecar/session_options.go`
  (self-labeled ADAPTER-SPECIFIC, vendor line numbers), `denyclient.go`
  (permission taxonomy), and `acp.go` (error-string markers).
- **Roadmap grounding** from `docs/NORTH_STAR.md` (P1–P4 table, tenets,
  Consequence Products, milestone ladder M7/M9/M10, workstreams A3/B6-B9) and the
  ADR inventory (`docs/adr/` — 0010, 0015, 0019.1, 0020.1, 0024.1–.2, 0027,
  0029, 0032.1, 0034).
- **Already-actioned exclusions verified:** companion audit H2 (acp.go split) —
  `wc -l internal/sidecar/acp.go` → 471, siblings present; M2 (failure taxonomy)
  — `docs/adr/0034-failure-class-model.md` status `proposed`, not built.
- Every `file:line` above was spot-checked against the working tree; `go build
  ./...` succeeds at audit time (exit 0). **No code was modified by this audit.**
