---
name: arch-audit
description: >-
  Use to run a SCOPED, multi-persona, partly-adversarial audit of ghx's
  architecture, codebase, and tech debt — Go architecture/design-pattern
  quality, encapsulation, domain modules, core/reusable-capability boundaries
  (the Agent Sidecar Framework as reusable infra; the shared core between evals
  and the main ghx product), and tech debt in specific modules. Background
  personas (several adversarial) each write one ADR-style artifact into
  docs/audits/; a distillation artifact converges them; the orchestrator makes
  the final judgment and a governing ADR. This is a CODE/ARCHITECTURE audit —
  for product/PMF/strategy use `product-audit`; for a quick one-file review just
  read it.
---

# Architecture & Codebase Audit — scoped, multi-persona, adversarial

A repeatable loop for raising ghx's **maintainability → engineering velocity**.
Clean architecture is the point: reuse existing pieces to build the next thing
10x faster, delete unnecessary complexity, no bandaids. The orchestrator (Fable)
**holds the scope, converges the findings, and makes the final call**; breadth —
the persona audits — fans out to background agents. Delegate breadth, keep
judgment.

Worked example already on disk (the loop made concrete, pre-convention naming):
`docs/audits/architecture-2026-07-07.md` (4-persona) + `docs/audits/architecture-vision/`
(5-persona) → distilled into `docs/adr/0035-*` and `docs/adr/0036-*`. Read those to
see the altitude, the file:line rigor, and the "Recommended sequence" shape this loop targets.

## When to use / when not

**Use** when you want to raise the architectural quality of ghx or a part of it:
a module feeling like a god-file, a capability that should become reusable core,
a boundary that's coupling things that should be replaceable, recurring tech
debt, or a periodic "is the architecture still healthy" sweep. **Don't** use it
for a single-file review (just read it), a mechanical rename, or a product/market
question (use `product-audit`). The loop costs several agent generations — it pays
off when architectural quality genuinely moves velocity.

**Neighbours:** `product-audit` is the product/strategy counterpart (same
fan-out→distil shape, different lens). `skill-forge` is the meta-loop that built
both. `fable-delegation` owns the delegation *mechanics* (command syntax,
worktrees, ff-only landing); this skill owns the *architecture-audit loop*.

---

## The run-spine (front-load this — how to run, what to deliver)

1. **Pick a SCOPE, rank the quality attributes, write a one-paragraph charter.**
   The scope is the whole lever of the run (see "Choosing a scope"). Before fanning
   out, name the **2–3 quality attributes that matter most for this scope** — a
   lightweight ATAM-style *utility tree* (e.g. a reusable-core scope prioritizes
   replaceability > testability > cohesion; a hot module prioritizes
   maintainability > correctness) — so the personas target what matters instead of
   auditing everything equally. The charter names: what's in/out, the ranked
   quality attributes, which tenets/ADRs/north-star phases it serves, and the
   question the run must answer. Commit it as the thread header
   `AA-000N-<scope-slug>.md`.
2. **Fan out the personas (breadth, background, isolated worktrees).** Pick 4–6
   persona×lens pairs for the scope from the catalog (`reference/personas.md`) —
   **at least one adversarial** (YAGNI skeptic and/or velocity red-team) as the
   mandatory counterweight, and route the **mechanical metrics** pass to a
   Codex/GPT worker for an independent-model-family perspective. Each worker:
   read-only, writes **exactly ONE artifact** to a disjoint path, grounds every
   claim in `file:line`, and returns the evidence contract. Disjoint artifacts →
   no merge conflicts. (Delegation syntax: `fable-delegation`.)
3. **Land each artifact** as it completes via **rebase-in-its-own-worktree +
   `git merge --ff-only`** (never `--amend` on a shared mainline — a parallel
   engineer may be committing; see CLAUDE.md "Git Hygiene on a Shared Mainline").
4. **Converge & distil.** When all persona artifacts are in, produce the
   distillation artifact `AA-000N.9-distilled.md` in the same thread — ideally
   drafted by a **fresh, no-stake agent** that mines the *intersection* and
   re-runs any recompute/metrics, then **the orchestrator judges it**. Rank by
   **cross-persona convergence × impact × tractability**; hold tensions
   explicitly, never average them.
5. **Govern with an ADR + execute.** Turn the distilled sequence into a governing
   ADR (the ADR-0035/0036 precedent), then execute the refactors as verified
   worker tasks — each behavior-preserving, ADR-gated, landed ff-only.
6. **Graduate the wins into standing fitness functions (anti-shelfware).** The
   structural invariants the audit relied on — import-direction greps, a god-file
   size ceiling, "no `acp`/transport import outside the adapter", the persona-byte
   golden — should become **committed, executable checks** (a test or CI step)
   that fail if a boundary drifts back. An audit that only writes a report decays;
   one that leaves behind executable guards compounds, so the *next* run doesn't
   re-discover the same debt (Ford/Parsons/Kua evolutionary-architecture fitness
   functions; the 2026-07-07 run's checks were one-shot greps — graduate them).

**Deliverables:** the thread charter, N persona artifacts, one distillation
artifact, a governing ADR, and any new fitness-function checks — all under
`docs/audits/` + `docs/adr/` + tests, each recomputable from committed evidence.

## Choosing a scope (the run's biggest lever)

- **Whole codebase** — a periodic health sweep (the 2026-07-07 run).
- **A module** — `internal/sidecar`, `internal/ghx`, `internal/sidecar/evals`,
  `internal/mapengine`, `internal/codemode`.
- **A capability / reusable core** — the highest-value scope for ghx's vision:
  is the **Agent Sidecar Framework** a reusable infrastructure boundary or welded
  to ghx? What is the **shared core between the evals machinery and the main ghx
  product** — a DDD **Shared Kernel** (small, deliberately coordinated, expensive
  on purpose), or accidental duplication drifting apart? What should become a
  reusable module vs. stay product-specific? (Guard both ways: a genuine shared
  kernel is worth the coordination cost; a premature one is the "wrong abstraction"
  that's costlier than duplication — the YAGNI persona argues the inline side.)
- **A mindset / cross-cutting lens** — decoupling & replaceability (the runner
  port), resilience, testability, dependency direction, domain modeling.

The scope decides the personas and the thread name. Narrow scopes get sharper,
more actionable artifacts than "audit everything."

## Personas × lenses (catalog in `reference/personas.md`)

Pick per scope; always include ≥1 adversarial and the metrics worker. The proven
roster: **Go architecture & boundaries** (idiomatic Go, core/shared/domain,
"services vs singletons", composition root, encapsulation); **domain modeling**
(value objects/entities that flow, ubiquitous language); **reusable-core
extraction** (framework-vs-product boundary, evals↔ghx shared core); **ports &
adapters / replaceability**; **resilience & runtime robustness**;
**complexity/tech-debt metrics** (gocyclo/gocognit/size/duplication/error-sprawl
**+ complexity×git-churn hotspots** — the ~1–2% of files where refactoring buys
the most velocity, not just the biggest files — routed to **Codex/GPT** for an
independent model family); and the adversaries — **YAGNI skeptic** (argue against
over-engineering) and **velocity red-team** (roadmap-blocking debt), run as a real
discipline (premortem / Team-A-vs-Team-B) and held to the **DH5/DH6 bar** (quote
the exact passage, name the exact flaw — no vague doom). Personas must genuinely
*disagree*; convergence across independent personas is the signal you rank on.

## Artifact contract (template in `reference/artifact-template.md`)

Every artifact is ADR-style: YAML frontmatter (`title, date, status:
audit|distillation, thread, scope, persona/lens, author`) + body:
**Executive summary**; **"What is already right"** (verified + cited — a mandatory
section, it stops manufactured findings); **severity tiers (High/Med/Low)** each
with `file:line` evidence, a tie to a named AGENTS.md tenet / NORTH_STAR phase,
and a concrete recommendation; a **"Recommended sequence"** (phased, non-rewrite,
each step shippable + behavior-preserving); and **"Method / auditability"** (the
exact commands so every number recomputes). The distillation artifact adds a
**convergence table** (which independent personas agreed) and explicit
tension-resolution. **Threading:** `AA-000N` per run, `.1/.2/…` per persona,
optional `.6-adjudication`, `.9-distilled` — matching the `product-audit`
`PA-000N` convention.

## The final judgment (the load-bearing step)

You (the orchestrator) are the authoritative judge, and judging your own
delegated fan-out is a self-preference regime (Panickssery et al., NeurIPS 2024 —
self-*recognition* causally drives self-*preference*). So: **re-derive load-bearing
claims yourself** (open the cited `file:line`, run the recompute) before
accepting; **eat the fish, throw the bones** — support or veto each distilled
idea with evidence; **discount shared-prior convergence** (agents agreeing
because they read the same code is not independent corroboration — the signal is
an independent recompute, not the head-count); for the highest-stakes calls get a
**cross-family check** (a Codex/gpt-5.5 adjudicator, or Goga). Record what you
declined and why. "No", "this is fine", and "the premise is wrong" are valid
outcomes — adding a refactor has real cost.

## Guardrails / hard rules (learned from the 2026-07-07 runs)

- **Ground every claim in `file:line`.** An unsourced finding is a hypothesis.
- **The "what's already right" section is mandatory** — it prevents personas
  from manufacturing debt to justify themselves (the 2026-07-07 run verified the
  core/frontend boundary was *healthy*, which is as valuable as the findings).
- **Respect the frozen measurement stack.** Never propose changing eval scoring,
  gates, detectors, or reward math as a drive-by; flag and defer behind a
  pre-registered eval ADR (AGENTS.md Visibility & Truthfulness). The eval
  god-files are the standing example: real debt, deliberately deferred.
- **North-star filter + no speculative frameworks + idiomatic Go.** Right-size
  every recommendation to a solo-dev Go product: no `internal/domain` layer
  cathedrals, no DI containers ("in Go the composition root *is* the DI
  container"), no abstraction of a component with exactly one implementation
  forever. The YAGNI persona enforces this; the judge honours it.
- **Behavior-preserving, phased, non-rewrite.** Every recommended step must be
  shippable on its own and provable byte-identical (unmodified test suite +
  race) unless it deliberately versions a change. No big-bang rewrites.
- **Forward-looking, agentic-first lens.** ghx's primary consumer is an AI
  agent, and this is 2026 — learn from architecture history, but never cargo-cult
  a pre-AI-scale dogma just because it once held. Re-price generic "best practice"
  against *our* actual context (a solo-dev, agentic-first Go product on the
  north-star path); the YAGNI skeptic and the distillation are where that
  re-pricing happens explicitly.
- **Integration mechanics:** isolated worktree per worker, commit ONE artifact,
  land ff-only, never `--amend` on a shared mainline. Text-only research
  (WebSearch/WebFetch/curl/gh — no browser). Every worker holds the big picture
  (goal, NORTH_STAR, AGENTS.md tenets) before it starts. Mechanics: `fable-delegation`.

## Provenance

**Worked example** (the loop made concrete): the 2026-07-07 architecture run —
`docs/audits/architecture-2026-07-07.md`, `docs/audits/architecture-vision/*.md`,
`docs/audits/refactor-review-2026-07-07.md`, distilled into `docs/adr/0035-*` and
`docs/adr/0036-*`. Read those for the altitude and the `file:line` rigor.

**Canon & sourcing** (why the methodology holds — skill-forge Round 0):
`research/01-idiomatic-go-architecture.md`,
`research/02-clean-architecture-ddd-reusable-modules.md`,
`research/03-tech-debt-detection-metrics.md`,
`research/04-architecture-audit-methodology.md` — 66 deep-URL sources with Source
Ledgers (Go Proverbs / Ben Johnson / Kat Zien; Clean Architecture / Cockburn /
Evans Shared Kernel; Cunningham / Fowler / Tornhill hotspots; ATAM / premortem /
fitness functions / LLM self-preference). The SKILL.md stays thin by design; the
ledgers are the citation of record (progressive disclosure).

**Reference material:** `reference/personas.md` (persona catalog),
`reference/artifact-template.md` (frontmatter + body template + threading).
