# The Agentic-First Lens — Re-Pricing the Architecture Canon for an AI-Fleet-Maintained Codebase

*Research artifact #5 for the **arch-audit** skill (skill-forge Round 0 / Wave-2). Its
job is the MANDATORY context re-pricing pass: take the load-bearing claims the four
Wave-1 canon artifacts already produced and, for each, tag it **HOLDS / ADAPT /
OUTDATED-OR-INVERTED** for ghx's actual context. This is **not** the skill, and it does
**not** run a parallel research stream — it audits the canon in `[01]`–`[04]`.*

- **Audit target:** the Wave-1 canon — `[01]` idiomatic-Go architecture, `[02]`
  clean-architecture/DDD/reusable-modules, `[03]` tech-debt/metrics, `[04]`
  audit-methodology.
- **Context being priced against (the whole point):** ghx is a **solo-developer +
  AI-agent-fleet-maintained, agentic-first Go product** on the north-star path.
  Two facts re-price everything: **the code is read and refactored BY AI agents**, and
  **the audit itself is run BY AI agents**. It is 2026 — learn from the pre-AI canon,
  never cargo-cult an idea just because it once worked in a world that had no AI at this
  scale.
- **Grounding, not new research:** every verdict cites a Wave-1 artifact section, a
  `NORTH_STAR.md`/`AGENTS.md` section, or is labelled **[SYNTHESIS]**. One external
  anchor (Anthropic context-engineering) is verified fresh because the whole "loaded-unit
  = context tax" thesis rests on it and `[01]`–`[04]` never cite it; the self-preference
  and multi-agent-debate anchors are reused from `[04]`'s verified ledger.
- **Author:** Fable Wave-2 re-pricing agent (Opus 4.8), 2026-07-07. Method: text-only
  (one WebFetch), per AGENTS.md Tool Economy.

---

## 1. Executive summary

**The headline finding is not "the architecture canon inverts." It is that idiomatic Go
plus its counter-canon is, by luck of its ethos, the most agent-native body of software
architecture advice that exists — so it needs far *less* inversion than the product
canon did, and the auditor's real job is to *re-weight and re-metric* it, not overturn
it.** Rob Pike's line "clear is better than clever," Ben Johnson's dependency-light root,
Sandi Metz's "duplication is cheaper than the wrong abstraction," and Simon Brown's "lean
on the compiler" are all, underneath, one instruction: *minimize what must enter the
reader's context.* That is verbatim ghx's own thesis about the main agent's context — now
turned inward on the codebase the fleet maintains. The Go canon was already optimizing for
the agent; it just didn't know it.

Three things change, and they cluster cleanly:

1. **The *meaning* of "maintainability" changes — from human-team ergonomics to
   agent-legibility.** The maintainer is not a person who scrolls, forgets, ramps up over
   a tenure, and holds tribal knowledge; it is **a fleet of stateless agents, each a fresh
   hire every session, that re-derives understanding cold from the artifact, bounded by
   what fits in a finite context window.** Anthropic states the physics: *"Context…must be
   treated as a finite resource with diminishing marginal returns"* and good engineering
   is *"finding the smallest possible set of high-signal tokens that maximize the
   likelihood of some desired outcome"* [Anthropic, Context engineering — verified]. So
   maintainability becomes: **file:line groundability**, **fits-in-a-context-window**,
   **one-concern-per-loaded-unit**. The task hypothesis holds and sharpens — **cohesion
   and small, self-contained units matter MORE, not less** — with one correction: the
   target is *concern-locality*, and BOTH god-files AND over-fragmentation fail it (§5.1).

2. **The *weights* change — the counter-canon and the compiler-enforced boundaries move
   up; the enforcement mechanisms become load-bearing, not nice-to-have.** Agents generate
   and refactor code nearly for free, so the *build* cost of a speculative abstraction has
   collapsed — but Fowler's **cost of carry** has not, and in a signal-per-token regime it
   is *directly taxed*: every speculative layer is context a future agent must re-load to
   understand. So **YAGNI / "the wrong abstraction" / "a little copying" all weight UP**
   (`[02]`'s own agentic section already saw this; §5.2 sharpens *which* reuse). And
   because a **parallel agent fleet erodes convention-only boundaries faster than any human
   team** (memory: 8 workers in flight at once), the only enforcement that survives is
   compiler-enforced (`internal/`, unexported, acyclic imports) plus **committed fitness
   functions** — which re-price from `[03 §9]`/`[04 §4]`'s "anti-shelfware" to *the audit's
   only durable output* (§5.3).

3. **The *methodology* changes — because the audit is run BY agents on code written BY
   agents, self-preference becomes the central guardrail, and the pre-AI workshop ceremony
   must be stripped.** `[04]`'s Panickssery anchor (self-recognition causally drives
   self-preference) stops being one guardrail among many and becomes *the* deliverable-
   critical concern: the auditor may be the same model family as the author. Cross-family
   routing + same-family-convergence discounting move from `[04]`'s *optional* adjudication
   to *mandatory* (§5.4). Meanwhile ATAM/SAAM/4+1 keep their *taxonomies* (risks/non-risks/
   sensitivity/tradeoff; view grid) but shed their *workshop logistics* — batched multi-day
   human events exist because getting humans in a room is expensive; spawning an auditor
   persona is cheap and parallel, so the economics invert to **continuous cheap fan-out**,
   and code-review's "5 reviewers / small PRs for reviewer attention" re-prices to
   "legible per context window / cross-family independence" (§5.5).

The genuine inversions are few and they all sit in the **human-team-maintenance
assumptions** the canon carried silently: onboarding docs, ramp-up time, bus factor,
reviewer scarcity, workshop batching. Everything about **information flow and coupling**
— dependency direction, acyclic imports, small consumer-side interfaces, single-owner,
ports-and-adapters — **HOLDS wholesale**, because agents obey the same coupling physics
as humans. An auditor who "re-prices" those is manufacturing novelty; an auditor who fails
to re-weight the counter-canon and re-metric cohesion to context-tax is auditing a
codebase that no longer exists.

---

## 2. The agent-maintainer mental model (what replaces "maintainability")

Four substitutions. Each is the load-bearing swap the table in §4 audits against.
**[SYNTHESIS]** throughout, grounded in the cited north-star/AGENTS sections.

**(a) The maintainer is a stateless fleet, not a tenured team → every touch is a cold
read.** A human team amortizes the cost of understanding a gnarly module across months;
the knowledge lives in heads and survives between edits. An agent fleet has **no tenure and
no shared head** — it re-derives understanding from the artifact on every single touch,
from cold, bounded by context. This *kills the entire "invest in onboarding because it pays
back over a career" calculus* (there is no career) and *massively up-weights the artifact's
self-evidence* (cohesion, naming, doc-comments-as-in-context-spec, ubiquitous language,
single-owner), because self-evidence is what gets re-read cold every time. AGENTS.md
already encodes the corollary: *"every exported type, function, and field carries a doc
comment stating contract and units"* (AGENTS.md, Engineering Tenets) — for a human that is
courtesy; for an agent it is a way to *use a symbol without loading its body into context*,
a direct token saving.

**(b) The unit of loading is the file/package, and it costs tokens → cohesion is a context
budget, not an aesthetic.** A human reading a 1000-line file scrolls and holds ~40 lines in
view; god-file cost is `O(scroll)`, sub-linear, an annoyance. An agent that must *safely
edit* any part of that file loads the whole thing (it cannot assume concern A doesn't
interact with concern B without reading both); god-file cost is `O(file)` per edit, and
most of those tokens are irrelevant noise that dilutes attention and raises edit-error
rate. Anthropic's "finite resource with diminishing marginal returns" is the physics; the
**"main agent's context is sacred"** tenet (NORTH_STAR, Tenets), turned inward, is the
rule. Cohesion is how you keep the *minimal legible edit unit* small.

**(c) Groundability is the currency of trust → naming and single-owner are findability
infrastructure.** Agents cite `file:line` and pattern-match on identifiers. A concept that
lives as a *named type* with one authoritative owner (AGENTS.md: *"one authoritative owner
per domain concern"*; `[02 B3]` aggregate root) is a place the agent can *find* and *ground
a change in*. A concept passed as `string`/`map[string]any` and re-parsed at each call site
(`[02 B1]` primitive obsession) is *un-findable*: an agent refactoring it cannot reliably
locate all instances, so the fleet silently diverges. Groundability is why the canon's
"name things" rules weight *up*, not down.

**(d) The fleet lands changes fast and in parallel → convention decays, only the compiler
holds.** Human boundaries survive on a reviewer *remembering* a convention. An agent fleet
merging in parallel has no shared memory and erodes conventions faster than any human team
(memory: parallel background workers, one worktree each). Only boundaries a violation
*cannot compile past* — `internal/`, unexported types, acyclic imports — and invariants a
CI **fitness function** re-checks every build survive the fleet. This is `[02 C5]` "lean on
the compiler" and `[03 §9]`/`[04 §4]` fitness functions, both re-priced up.

---

## 3. Verdict key

- **HOLDS** — transfers essentially unchanged; agents obey the same information-flow/coupling
  physics as humans. (When it holds *and gains force*, marked **HOLDS+**.)
- **ADAPT** — the principle holds, but the *metric, weight, or instrument* must change for an
  agent maintainer.
- **OUTDATED-OR-INVERTED** — as literally practiced, the advice misleads, is absent, or
  points the wrong way for an agent-maintained codebase.

---

## 4. The re-pricing table over the Wave-1 canon

### 4A. Boundary, dependency & structure canon (from `[01]`, `[02]`)

| # | Load-bearing claim (Wave-1 cite) | Verdict | Why, for an AI-fleet maintainer / audit |
|---|---|---|---|
| 1 | **Organize packages by dependency/capability, not by kind** (`[01 P1]`, `[02 C8]`) | **HOLDS** | Information-flow rule; agents obey it identically. "By kind" (`types/`, `handlers/`) also *scatters a concern* across the tree, which taxes cold agent reads — so it holds for a second, agentic reason too. |
| 2 | **Ban `util`/`common`/`misc`; name a package for what it provides** (`[01 P2]`, `[02 C7]`) | **HOLDS+** | Re-priced up: the package name is the agent's *findability index*. A grab-bag name means the fleet can't locate a concern by name → drift. Groundability (§2c). |
| 3 | **Accept interfaces, return structs; small, consumer-defined interfaces** (`[01 P3]`, `[02 C6]`) | **HOLDS** | Consumer-side, small interfaces *reduce* the indirection an agent must chase to find real behavior. A producer-side "manager" interface is a context-tax join (§5.2). |
| 4 | **Avoid premature abstraction / interface-YAGNI** (`[01 P4]`, `[02 E1/E3]`) | **ADAPT (weight UP)** | Build cost of a speculative interface collapsed (agents emit them free); *carry* cost didn't. `[01 P4]` already flags "speculative interfaces age into dead abstraction faster" in a fast-churning agentic codebase — affirmed and central (§5.2). |
| 5 | **Composition over inheritance; embedding + functional options** (`[01 P5]`, `[02 D1]`) | **HOLDS** | Go-idiomatic, low-ceremony; nothing in the agent shift touches it. Functional options stay the right answer for ghx's many dials (depth/anticipation/budgets, NORTH_STAR §3). |
| 6 | **Composition root in `cmd`/`main`; no DI framework, no package globals** (`[01 P6]`) | **HOLDS+** | A package-level mutable global is *hidden state an agent cannot ground from the call site* — worse for a cold reader than for a human who knows the codebase. Explicit injection = legible provenance. |
| 7 | **Acyclic, one-directional imports; `internal/` enforces the API surface** (`[01 P7]`, `[02 A1/C4`) | **HOLDS+** | The Dependency Rule is pure information flow — unchanged. Weight up on *enforcement*: compiler-enforced acyclicity is the boundary that survives a parallel fleet; convention doesn't (§2d, §5.3). |
| 8 | **Ports & Adapters — one core, many drivers** (`[02 A2]`) | **HOLDS** | Exactly ghx's shape (one `internal/ghx` core; CLI/MCP/codemode/sidecar/eval-runner as adapters). Agent-neutral; the audit still checks no frontend re-implements core recon. |
| 9 | **Value Object / kill primitive obsession** (`[02 B1]`) | **HOLDS+** | Re-priced up hard: a named type is the **agentic cure for un-findable duplication** (§5.2). "If a concept appears in two places it deserves a type" (AGENTS.md) is *findability*, not ceremony. |
| 10 | **Aggregate / single authoritative owner per concern** (`[02 B3]`, AGENTS.md) | **HOLDS+** | One place for the fleet to look when changing "session"/"report"/"telemetry" logic. Legibility boundary, not just an invariant boundary. |
| 11 | **Ubiquitous language — symbols match the domain vocabulary** (`[02 B4]`) | **HOLDS+** | Agents pattern-match on names; agent-authored code drifts from ADR/NORTH_STAR vocabulary silently. **ADR-term ↔ symbol-name drift is a first-class agentic defect** the auditor should grep for. |
| 12 | **Shared Kernel: small + explicit + coordinated** (`[02 C2]`) | **ADAPT** | Re-priced (§5.2): the coordination-cost argument *softens* (agents propagate a kernel change across consumers cheaply), but the **cross-package traversal tax** it imposes on every cold reader becomes the dominant cost. Justify a shared core by *"does it reduce fleet-wide context tax,"* not by "it removes duplication." Directly the SAF/SAFE reusable-core question. |
| 13 | **Component cohesion/coupling: REP/CCP/CRP tension; ADP/SDP/SAP** (`[02 C3/C4]`, `[03 §6]`) | **HOLDS (tension tilts smaller)** | The pricing model survives; agent-legibility adds weight to CRP (exclusive / smaller) because a consumer forced to load a component it barely uses pays context tax. Martin's Ca/Ce/instability stays the instrument for the core/reusable boundary. |
| 14 | **Modular monolith; lean on the compiler, not discipline** (`[02 C5]`) | **HOLDS+** | `[02]`'s own agentic note is right and I amplify it: agent-authored PRs erode conventions *faster* than human ones. Compiler-enforced boundaries are the fleet-proof ones (§5.3). |
| 15 | **GoF patterns are mostly language workarounds; penalize ceremony** (`[02 D2]`) | **HOLDS+** | Agents trained on enterprise-OOP corpora readily emit `StrategyFactory`/`Visitor` scaffolding a `func` would express. Penalize *harder* — pattern ceremony is now a common *generation* artifact, not just a legacy one. |
| 16 | **"A little copying is better than a little dependency"** (`[01 P9]`, `[02 E4]`) | **HOLDS+ (re-based)** | Strengthens, for a new reason: a dependency is a **context-window join** — to understand A→B an agent loads both. But add the third axis: copy *behavior* (favored) vs. duplicate a *concept* (penalized → **name it**, don't depend). See §5.2. |
| 17 | **kit vs application; policy-free capability providers** (`[01 P9]`) | **HOLDS** | The SAF-as-reusable-infra boundary test is unchanged; the audit still checks shared modules stay policy-free (`internal/sidecar/telemetry` not absorbing eval-only policy). |
| 18 | **Don't cargo-cult a layout; structure is emergent and justified** (`[01 P10]`) | **HOLDS+** | Agents cargo-cult templates from training data *more* readily than humans (they've "seen" `golang-standards/project-layout` a million times). "What dependency forces this structure?" is a sharper question when the author is a pattern-matcher. |

### 4B. Tech-debt, metrics & the god-file question (from `[03]`)

| # | Load-bearing claim (Wave-1 cite) | Verdict | Why, for an AI-fleet maintainer / audit |
|---|---|---|---|
| 19 | **Debt = interest/principal; a decision metaphor, not "bad code"** (`[03 §1/§2]`) | **HOLDS** | The quadrant/triage frame is agent-neutral. Re-based: "interest" ≈ the recurring **context tax × churn** the fleet pays while the debt stands; frozen debt in a cold corner still costs ~0. |
| 20 | **Cyclomatic complexity (McCabe)** (`[03 §3]`) | **ADAPT (down-weight further)** | It measures *test-path count*, not edit-safety for an agent. `[03]` already subordinates it; for a fleet it is at best a weak nominator. |
| 21 | **Cognitive complexity (SonarSource) — nesting-weighted** (`[03 §4]`) | **HOLDS (promote)** | `[03]` already frames it as *"which function will an AI worker struggle to safely edit."* Best single per-function proxy for agent mis-edit risk. Keep it; rank above gocyclo. |
| 22 | **Size-confound critique; the Maintainability Index is discredited** (`[03 §5]`) | **ADAPT (partial inversion)** | "Don't blend metrics into one composite" **HOLDS**. But "size is *only* a confound, a weak proxy for understandability" **partially inverts**: for an agent, the size of the *minimal legible edit unit* is not a proxy — it *is* the per-edit context cost (§5.1). van Deursen's "just look at LOC" is truer for a machine than he meant, *when LOC = loaded-unit size gated by cohesion*. |
| 23 | **Martin package metrics (Ca/Ce/Instability/Abstractness/D)** (`[03 §6]`) | **HOLDS** | The import-graph instability instrument is the right lens for the reusable-core boundary; agent-neutral. A stable-and-volatile core is still the dangerous case. |
| 24 | **God-file / large-class smell** (`[01 P8]`, `[03 §7]`) | **ADAPT (intensify + re-metric)** | The single biggest re-metric. For a human a god-file is a scroll annoyance; for an agent it is a **recurring per-edit context tax** (§2b, §5.1). Re-metric from raw LOC to **"tokens an agent must load to safely change one concern."** The recent `acp.go` decomposition is the template, not a nicety. |
| 25 | **Hotspots = complexity × churn** (`[03 §8]`) | **HOLDS+** | Re-priced to *the* headline agentic-debt metric: **loaded-unit size × churn = the recurring context tax the fleet actually pays.** `[03 §10]`'s "cost of future features ≈ how often a worker must safely edit a file" is exactly this — affirm and make it primary. |
| 26 | **Change-coupling across boundaries = missing abstraction** (`[03 §8]`) | **HOLDS** | Files that co-change across package seams are as diagnostic for agents as humans; the extracted seam also *reduces* the cross-file loading a cold agent must do. |
| 27 | **Architecture fitness functions in CI** (`[03 §9]`, `[04 §4]`) | **ADAPT (weight WAY up)** | From "anti-shelfware, nice to have" to **the only fleet-proof enforcement and the audit's most durable output** (§5.3). Every recurring invariant (import direction, loaded-unit/LOC ceiling, "no compat path") should graduate into a committed `go test`/CI check. |
| 28 | **Prioritize debt by impact × tractability, not a smell dump** (`[03 §10]`, prioritization) | **HOLDS** | The leverage-ranked, tractability-gated backlog is exactly how a fleet should work; agent-neutral. Blast-radius/coverage gating still applies. |

### 4C. Audit methodology — because the audit is run BY agents (from `[04]`)

| # | Load-bearing claim (Wave-1 cite) | Verdict | Why, for an AI-run audit |
|---|---|---|---|
| 29 | **Multi-view / persona×scope fan-out (4+1, C4, ATAM utility tree)** (`[04 §1]`) | **ADAPT** | Keep the *view grid* and *risk taxonomy*; **strip the workshop logistics** (§5.5). Multi-agent debate (`[04]` Du et al.) grounds fan-out; spawning personas is cheap+parallel, so run many narrow passes continuously rather than one batched event. |
| 30 | **ATAM/SAAM as multi-day human stakeholder workshops** (`[04 §1]`, `[04]` gaps) | **OUTDATED-OR-INVERTED (the ceremony)** | The batched, facilitator-led, room-full-of-stakeholders *event* exists because human attention is scarce and rare. For async AI fan-out the economics invert — don't batch. `[04]` already flags the workshop wrapper as non-transferable; the analytic core (utility tree, scenarios) transfers. |
| 31 | **Adversarial passes; premortem; DH5/DH6 disagreement bar** (`[04 §2]`) | **HOLDS+** | Transfers intact *and* gains teeth (an agent adversary has no author's-pride to overcome), but its *independence must be engineered*: a same-family adversary shares priors, so its "adversarial" findings are partly correlated (§5.4). Keep the DH5/DH6 quote-and-refute quality bar. |
| 32 | **Artifact structure: ADR/MADR/Oxide-RFD numbering; ATAM risks/non-risks/sensitivity/tradeoff** (`[04 §3]`) | **HOLDS** | Model-agnostic conventions; and agents *read ADRs as in-context grounding*, so the numbered/threaded/immutable discipline is if anything more valuable. Keep MADR "considered options" for the governing ADR. |
| 33 | **Converge to action: rank + tie to a named driver; owner + non-goal per item** (`[04 §4]`) | **ADAPT** | Ranking by impact-on-a-north-star-driver HOLDS. "Owner per decision" re-prices: the owner is *the fleet*, so the useful artifact is a *fitness function or a scoped task*, not a named human — turn each recommendation into an executable check or a delegable worker prompt. |
| 34 | **Lead adjudication; self-preference bias (Panickssery); independence (Du)** (`[04 §5]`) | **HOLDS+ → CENTRAL** | Born-agentic and *the* load-bearing methodology item here: the auditor may be the same model family as the code's author *and* the judge. Make cross-family routing of the metrics pass + ≥1 adversarial pass **mandatory**, not optional; discount same-family convergence; the lead's re-derivation from `file:line` is deliverable-critical (§5.4). |
| 35 | **Code-review economics implied by human workshops (small PRs, ~5 reviewers, review latency)** (`[04 §1]`, gaps) | **ADAPT/INVERT** | Reviewer scarcity is a human-cost artifact. Agents are cheap+parallel: run *many* independent reviews, report variance, and gate on **correlated-error/self-preference**, not availability. "Small PRs for reviewer attention" → "legible per context window" (same conclusion, different reason). |

### 4D. Cross-cutting human-team-maintenance assumptions (the genuine inversions)

| # | Assumption carried silently by the general canon | Verdict | Why it inverts |
|---|---|---|---|
| 36 | **Onboarding docs / ramp-up time / bus factor as maintainability dimensions** (implicit across `[01]`–`[04]`, esp. "maintainability" framing) | **OUTDATED-OR-INVERTED** | The cleanest inversion. There is no tenure to ramp up and no shared head to lose to a bus — every agent is a fresh hire every session (§2a). An arch-audit persona that dings ghx for "high ramp-up" or "missing onboarding guide" is **cargo-culting a human-team concern**. Effort spent on human-onboarding artifacts is effort *against* agent-legibility. (ADRs and godoc are NOT this — they're in-context grounding, item 37.) |
| 37 | **"Minimize comments; code is self-documenting; comments rot"** (folk canon, adjacent to `[01]` clarity) | **ADAPT (partial inversion)** | Splits by comment type. **Contract doc-comments (godoc: contract + units, per AGENTS.md) INVERT to high-value** — they let an agent *use a symbol without loading its body*, a context saving. **Narration comments that restate code still cost tokens and still rot** — down-weight. The auditor rewards declarative contract comments and penalizes redundant narration. |
| 38 | **"Maintainability" as a single human-ergonomic quality** (framing shared by `[03]` metrics, `[04]` quality attributes) | **ADAPT → redefine** | Redefine the attribute itself to **agent-legibility**: groundability (file:line findable), context-window fit (loaded-unit size), concern-locality (one concern per unit), and vocabulary parity (symbol ↔ ADR term). Every persona's "is this maintainable?" becomes "could a fresh cold agent with only file:line change this one concern without loading unrelated ones or being misled?" (§5.1, the new lens). |

**Fairness summary.** *HOLDS / HOLDS+* — the entire boundary/dependency/coupling core
(items 1–3, 5–8, 10, 13, 17, 19–21, 23, 26, 28, 31, 32), because it is about information
flow, which agents obey identically. *ADAPT* — the counter-canon weighting (4, 16), the
Shared-Kernel pricing (12), the god-file/hotspot *metric* (22, 24, 25), fitness-function
*weight* (27), methodology economics (29, 33, 35), and comments (37). *OUTDATED-OR-
INVERTED* — the workshop ceremony (30) and the human-team-maintenance assumptions:
onboarding/ramp-up/bus-factor (36) and the human-ergonomic definition of "maintainability"
itself (38). Notably **nothing in the coupling/dependency canon inverts** — the inversions
are concentrated entirely in human-maintenance mechanism, exactly as the sibling
product-audit re-pricing found for *human-consumer* mechanism.

---

## 5. The re-pricings that most change how arch-audit should audit ghx

The five verdicts that should reshape the skill's persona lenses, metrics, or guardrails.
Each is **[SYNTHESIS]** built on the cited canon + north-star/AGENTS grounding.

### 5.1 Maintainability → agent-legibility, measured as the *minimal legible edit unit* (reshapes every persona's core question, and the god-file metric)

Replace the human-ergonomic definition of "maintainability" (items 24, 38) with a single,
measurable agent metric: **for a target concern X, how many tokens must a cold agent load
to change X safely, and how many unrelated concerns sit in those tokens to mislead it?**
Call it the *minimal legible edit unit (MLEU)*.

- **This corrects the task hypothesis in one important way.** "Small files matter more" is
  right, but the naive reading ("minimize LOC, split everything") pushes toward
  **over-fragmentation**, which *also* taxes the agent — a concern smeared across ten tiny
  files with cross-imports forces the agent to load ten files plus traverse the import
  graph (shotgun surgery). The real target is **concern-locality / cohesion**: one concern,
  one self-contained unit, fully loadable. **Both god-files and over-fragmentation fail
  MLEU.** The auditor scores the *sweet spot*, not "smaller is always better."
- **Re-metric the god-file finding.** `[03 §7]`/`[01 P8]` LOC-as-smell becomes MLEU-as-cost:
  rank not by raw LOC but by *loaded-unit size × churn* (`[03 §8]` hotspots, re-based) —
  the recurring context tax the fleet pays. This makes `[03 §5]`'s "size is just a
  confound" partially invert: loaded-unit size is a *direct* cost, not a proxy.
- **Reshapes the skill:** add MLEU/agent-legibility as the definition of the "maintainability"
  quality attribute in the charter; add a *cold-agent legibility* persona (§6.1); and drop
  onboarding/ramp-up as audit dimensions (item 36). External grounding: context is *"a
  finite resource with diminishing marginal returns,"* the goal is *"the smallest possible
  set of high-signal tokens"* [Anthropic, Context engineering — verified], which is ghx's
  own "signals per token" (NORTH_STAR, The Moat) turned onto its own source tree.

### 5.2 The abstraction axis is now a context-tax calculation: copy vs. depend vs. **name** (reshapes the reuse/YAGNI persona and the SAF/SAFE core question)

`[02]`'s own agentic section already said YAGNI and "a little copying" weight up; this
sharpens *which* reuse, because the naive version ("copy freely, agents keep copies in
sync") is a trap.

- **Judge every abstraction by one question: does it *reduce or increase* the tokens an
  agent must load to understand and safely change behavior?** A good abstraction (a named
  value object you can use *without* reading its body) reduces context tax. A bad one (an
  interface indirection or shared-policy package you must chase across files to find the
  real behavior) increases it. This gives `[02 E2]` "the wrong abstraction" a *measurable*
  test: if the fleet repeatedly loads the abstraction *and* its callers *and* its
  implementations to make a change, it is taxing, not serving — inline it.
- **The third axis is the point.** The debate isn't "copy vs. depend"; it's **copy vs.
  depend vs. name**:
  - *Copy behavior* (local, self-contained, legible in one unit) — **favored, more than
    before**: a local copy an agent reads in one file beats a dependency chain it must
    traverse (`[01 P9]`/`[02 E4]`, re-based as a context-window join).
  - *Depend on a shared-policy package* (`[02 C2]` Shared Kernel) — **penalized harder**:
    it's a cross-package traversal tax on every cold read, and a premature one is a wrong
    abstraction the whole fleet then pays for.
  - *Name the concept as a type* (`[02 B1]` value object) — **the agentic winner for a
    concept that recurs**: it gives legibility *and* findability (the fleet can locate every
    use of `EscalationTier` by name) *without* the coupling of a shared package. This is
    exactly AGENTS.md "if a concept appears in two places it deserves a type" — which is
    *not* a YAGNI violation, because it makes an *existing* concept legible rather than
    speculating a future one.
- **Reshapes the skill:** the reuse persona weights the counter-canon (`[02 E1–E4]`, `[01
  P4]`) up; re-prices the **SAF/SAFE reusable-core question** (Goga's central ask) — a
  shared core is justified only when it *reduces fleet-wide context tax*, not merely when it
  removes duplication; and adds "copy vs depend vs name" as the explicit decision the auditor
  applies to every shared abstraction and every duplicated concept.

### 5.3 Compiler-enforced boundaries + committed fitness functions are the ONLY fleet-proof enforcement (reshapes the guardrails from nicety to primary output)

A parallel agent fleet (memory: 8 workers in flight) erodes convention-only boundaries
faster than any human team, because there is no shared reviewer memory across parallel
merges (§2d).

- **Convention-only boundaries decay to zero under a fleet.** Anything a human reviewer
  "must remember to police" (`[02 C5]` Brown's warning) is already decaying; agent-authored
  parallel PRs finish the job. Only violations that *cannot compile* (`internal/`,
  unexported, acyclic imports — `[01 P7]`) and invariants a **CI fitness function**
  re-checks every build (`[03 §9]`, `[04 §4]`) survive between audits.
- **Fitness functions re-price from anti-shelfware to *the audit's most durable output*.**
  `[04]`'s "biggest divergence" (metrics are a one-shot snapshot; promote them to committed
  checks) is, under the fleet, not a divergence to note — it's the *primary deliverable*.
  Every recurring finding should graduate: core→frontend import-direction (`grep` = empty),
  an MLEU/LOC ceiling per file, "no `callTool` compat path" (AGENTS.md "no compatibility
  junk"). A finding that stays prose gets re-discovered — and re-violated — next audit.
- **Reshapes the skill:** the methodology/action persona *requires* that each recurring
  invariant lands as a `go test`/`golangci-lint depguard` check in the same PR as the fix,
  and the governing ADR lists the fitness functions it added, not just the refactors it
  recommended.

### 5.4 The audit is run BY agents on code written BY agents → self-preference is a mandatory guardrail, not an optional adjudication step (reshapes the orchestration)

`[04 §5]` already has the Panickssery anchor (self-recognition *causally* drives
self-preference) and calls the lead's adjudication canon-aligned — but the skill lists the
adjudication/contrarian pass as *optional* (`[04]` divergence #4). In ghx's context the
loop is closed twice: **the same model family may author the code, author the audit, and
judge the audit.** That makes self-preference not a risk to note but the *structural
default*.

- **Make cross-family routing mandatory, not optional.** Route the mechanical metrics pass
  (gocyclo/gocognit/dupl/import-graph) *and* at least one adversarial pass to a different
  model family (Codex/GPT-5.5), so at least one auditor cannot recognize its own priors in
  the code (`[04 §5]`(c)). This is cheap (mechanical work, clear spec — CLAUDE.md routing)
  and it breaks the self-recognition→self-preference loop *by construction*.
- **Discount same-family convergence.** Three same-model personas agreeing because they
  read the same `discovery.go` is *one* observation, not three (`[04 §5]`(b), Du et al.
  independence). The lead treats an *independent recompute from file:line* or a
  *cross-family agreement* as the real signal.
- **Reshapes the skill:** promote `.6-adjudication` from optional to standard for any
  contested call; make cross-family the default for the metrics pass and for high-stakes
  items; and make "the lead re-derived every load-bearing finding from `file:line`" a
  required line in the governing ADR (visibility/truthfulness tenet: *who scored this, from
  what evidence, how do I check it myself* — AGENTS.md).

### 5.5 Strip the pre-AI workshop ceremony; invert the economics from batched event to continuous cheap fan-out (reshapes the charter/cadence)

ATAM/SAAM/4+1 and classic code-review carry a hidden assumption: **human attention is
scarce, so batch everything into a rare heavyweight event.** For async AI fan-out the
assumption is false and the economics invert.

- **Keep the taxonomies; drop the batching.** The utility tree, risk/non-risk/sensitivity/
  tradeoff taxonomy, and the 4+1/C4 view grid all transfer (items 29, 32). The multi-day
  facilitated *workshop* (item 30), the "get stakeholders in a room," and the "5 reviewers /
  keep the PR small so a human will actually review it" scarcity heuristics are pre-AI
  ceremony — spawning a persona is cheap and parallel, so run **many narrow passes
  continuously**, report variance, and let the view grid ensure coverage rather than an
  eventful audit ensuring attention.
- **The binding constraint moves from availability to independence.** With cheap reviewers,
  the thing that limits signal is no longer "did anyone review it" but "were the reviewers
  correlated" (§5.4). So the cadence optimization is *cross-family independence and coverage*,
  not *reviewer scheduling*.
- **Reshapes the skill:** the charter names the view grid explicitly (so no view is
  unaudited), the audit runs as continuous parallel personas rather than a single event, and
  the "utility tree" is generated as a cheap pre-pass, not elicited in a workshop. This also
  aligns the audit's own working style with CLAUDE.md's "delegate wide, parallel background
  workers" fleet economics.

---

## 6. New agentic-first lenses no classical source names (skill additions)

Three lenses the classical arch-review canon (`[01]`–`[04]`) has no vocabulary for, because
classical reviewers were tenured humans who scrolled and navigated, not stateless agents who
load. **[SYNTHESIS]**, grounded as cited.

1. **The cold-agent legibility lens.** Signature question: *"If a fresh agent with only
   `file:line` grounding and a bounded context window had to change concern X, how many
   files/tokens must it load, and would unrelated concerns in those files mislead its
   edit?"* This is the architecture analog of the product-audit's AX persona. It directly
   scores cohesion (MLEU purity, §5.1), god-files (MLEU size), coupling (traversal depth),
   and naming/groundability (can the agent find X at all). Grounding: Anthropic context
   engineering (finite resource / smallest high-signal set — verified); NORTH_STAR "signals
   per token."

2. **The context-tax-of-an-abstraction lens.** Signature question: *"Does this abstraction/
   indirection reduce or increase the tokens an agent loads to understand and safely change
   behavior?"* Reframes the entire abstraction-vs-duplication debate (`[02 E1–E4]`,
   `[01 P4/P9]`) as a *measurable* context-tax calculation and gives "the wrong abstraction"
   an operational test (§5.2). No classical source has it because human "cognitive load" was
   never a token count.

3. **The fleet-provenance / self-preference lens (meta, on the audit itself).** Signature
   question: *"Was this auditor the same model family that authored this code — and if so,
   is its approval self-preference?"* Born from `[04 §5]`'s Panickssery anchor but named as a
   standing arch-audit concern *because the author and the auditor are both agents* — a
   situation no classical review method contemplated. Drives the mandatory cross-family
   routing of §5.4.

*(Two further lenses are really re-weightings of existing canon rather than new inventions,
so they are not listed above but noted: (a) the fitness-function-as-durable-output lens
(§5.3) and (b) the vocabulary-parity lens — ADR-term ↔ symbol drift as a first-class defect,
item 11.)*

---

## 7. What the skill should explicitly WARN against (mislead-if-applied-verbatim)

Named anti-patterns for an agentic-first architecture audit — classic advice that, applied
verbatim to an AI-fleet-maintained codebase, produces confident waste.

- **DRY / "extract every duplication" applied verbatim.** The most dangerous mis-transfer,
  and agents trained on Java-heavy corpora carry the reflex. Aggressive extraction creates
  premature Shared Kernels and wrong abstractions (`[02 C2/E2]`) that then tax every cold
  read. The corrective — copy-vs-depend-vs-**name** (§5.2) — must be weighted up, not treated
  as old caution.
- **Dinging the codebase for "missing onboarding docs" or "high ramp-up time."** A
  human-team maintainability concern with no agent referent (item 36). Cargo-culting it wastes
  the audit's attention on a non-problem.
- **Ranking the codebase by a single maintainability score / Maintainability Index.**
  Discredited already (`[03 §5]`); for agents the right replacement is *legibility metrics*
  (MLEU × churn, groundability), not a better composite (item 22).
- **Running the audit as a heavyweight batched ATAM-style event.** Pre-AI ceremony (item 30);
  the economics favor continuous cheap fan-out (§5.5).
- **Trusting same-family convergence as corroboration.** Three Claude personas agreeing that
  read the same file is one observation, not three (§5.4). An uncalibrated same-family judge
  is the arch-audit's "leading the witness."
- **"Design for extension / OCP / add plugin points" speculatively.** Speculative extension
  points are premature abstraction taxing the fleet (`[01 P4]`, `[02 E1]`); the Go canon
  already resists this, but an agent may over-apply enterprise OCP.
- **Treating "smaller files, always" as the cohesion goal.** Over-fragmentation fails MLEU
  as surely as god-files do (§5.1); the target is concern-locality, not minimized LOC.
- **Treating all comments as clutter to minimize.** Contract doc-comments are context-saving
  infrastructure for agents (item 37); only redundant narration is clutter.

---

## 8. Honestly-flagged gaps & scope

- **This is a re-pricing, not a code audit.** Like `[01]`–`[04]`, it produces the *questions
  and weightings* an agentic-first auditor applies; it does not read `internal/ghx` or
  measure ghx's actual MLEU/hotspots. The concrete numbers (`discovery.go` 1007 LOC, etc.)
  are inherited from `[01]`/`[03]` as illustrations, not re-measured here.
- **MLEU is a proposed metric, not a validated one.** "Minimal legible edit unit" (§5.1) is
  **[SYNTHESIS]** — a reframing of loaded-unit context cost, grounded in Anthropic's
  finite-context physics and ghx's signals-per-token thesis, but there is **no published
  empirical study of debt↔velocity for an *agent* maintainer** (a gap `[03]` also flagged
  for its Go-specific extrapolation). It is a lens to apply and later calibrate against real
  fleet edit-traces (which ghx uniquely logs), not a proven law.
- **The self-preference application is extrapolation.** Panickssery/Du (via `[04]`'s verified
  ledger) establish the *mechanisms*; applying them to "same-family arch-auditor on
  agent-authored Go" is labelled synthesis, as `[04]` already disclosed for its own use. The
  mechanism transfers cleanly; no paper studies this exact setting.
- **One external anchor, verified; the rest reused.** Only the Anthropic context-engineering
  quotes were fetched fresh this pass (deep URL below), per the instruction not to run a
  parallel research stream. The self-preference, multi-agent-debate, fitness-function, and
  all architecture-canon sources are cited *through* `[01]`–`[04]`'s already-verified
  ledgers — I did not re-verify them, and any load-bearing verbatim quote from those should
  be re-pulled from the Wave-1 ledgers before external citation.
- **Deliberately out of scope:** concurrency/resilience design, error-handling architecture,
  security, and testing architecture — each its own persona/angle. Cohesion across angles is
  the distillation step's job. This file re-prices the *general* canon; the *live* ghx
  findings come from the recon/code-reading personas.

**External source verified this pass:**
Anthropic Applied AI, *Effective context engineering for AI agents* —
`https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents`
(WebFetch, 2026-07-07): *"Context…must be treated as a finite resource with diminishing
marginal returns"*; *"Every new token introduced depletes this budget"*; good context
engineering is *"finding the smallest possible set of high-signal tokens that maximize the
likelihood of some desired outcome."* The external grounding for the loaded-unit / MLEU
re-pricing (§5.1) and ghx's own "signals per token" (NORTH_STAR, The Moat).

**Wave-1 artifacts audited (the re-pricing target, not external sources):**
`[01]` `01-idiomatic-go-architecture.md` · `[02]` `02-clean-architecture-ddd-reusable-modules.md`
· `[03]` `03-tech-debt-detection-metrics.md` · `[04]` `04-architecture-audit-methodology.md`.
