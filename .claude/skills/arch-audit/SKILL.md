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
  the final judgment and a governing ADR. TRIGGER-GATED, not periodic. This is a
  CODE/ARCHITECTURE audit — for product/PMF/strategy use `product-audit`; for a
  quick one-file review just read it.
---

# Architecture & Codebase Audit — scoped, multi-persona, adversarial

A repeatable loop for raising ghx's **maintainability → engineering velocity**.
Clean architecture is the point: reuse existing pieces to build the next thing
10x faster, delete unnecessary complexity, no bandaids. But architecture is an
**enabler, never the frontier** — *"the moat is the brain, not the tools"*
(NORTH_STAR). This loop exists to keep the plumbing out of the way of the brain,
not to generate refactors. The orchestrator (Fable) **holds the scope, converges
the findings, and makes the final call**; breadth — the persona audits — fans out
to background agents. Delegate breadth, keep judgment.

Worked example already on disk (the loop made concrete, pre-convention naming):
`docs/audits/architecture-2026-07-07.md` (4-persona) + `docs/audits/architecture-vision/`
(5-persona) → distilled into `docs/adr/0035-*` and `docs/adr/0036-*`. Read those to
see the altitude, the file:line rigor, and the "Recommended sequence" shape this loop targets.

## When to use / when not — TRIGGER-GATED (this is load-bearing)

This loop is **event-driven, never a cadence.** It mirrors the repo's own binding
eval-cadence rule (`CLAUDE.md`): *"there is no 'deltas accumulated, time to re-run'
treadmill … explicitly rejected"* and *"'loop towards the north star' is never by
itself a trigger."* A periodic architecture sweep is a standing refactor generator
that competes with the frontier (M5/M8/C4/C7/C8) — the anti-pattern this skill must
not become.

**Run it only on a real trigger:**
- a boundary is **blocking the roadmap** (e.g. the runner needs a 2nd implementation → ADR-0036);
- a module is a **measured hotspot** (complexity × churn) *on the frontier critical path*;
- a capability should become **reusable core** and you must decide the boundary (the SAF, the evals↔ghx shared kernel);
- a **repeated friction / recurring bug** points at a structural cause;
- an occasional **whole-codebase health check** — rare, deliberate, and *still* trigger-justified, not calendar-driven.

**"Do nothing" is a first-class outcome.** If the audit finds the architecture
already fits the scope's needs, it says so (the "What is already right" section
carries equal weight) and emits a *watch-list*, not a refactor list. Never run it
for a single-file review (read it), a mechanical rename, or a product/market
question (`product-audit`).

**Neighbours (name the seam, don't overlap):** `product-audit` = product/strategy
red-team (its `adversarial-velocity` lens is the *product* velocity view; this skill
is the *code/architecture* view — companions). `security-review` = threat surface.
`code-review` / `simplify` = a single diff. `skill-forge` = the meta-loop that built
this. `fable-delegation` = delegation *mechanics*.

---

## The run-spine (front-load this — how to run, what to deliver)

1. **Pick a SCOPE + rank its quality attributes + write a one-paragraph charter.**
   The scope is the whole lever. Before fanning out, name the **2–3 quality
   attributes that matter most for this scope** — a lightweight ATAM-style *utility
   tree* (a reusable-core scope prioritizes replaceability > testability > cohesion;
   a hot runtime module prioritizes race-freedom > recoverability > legibility) — so
   personas target what matters. The charter names the trigger, what's in/out, the
   ranked attributes, the tenets/ADRs/north-star phases it serves, and the question
   to answer. Commit it as the thread header `docs/audits/AA-000N-<scope-slug>/AA-000N-charter.md`.
2. **Fan out 4–6 personas (breadth, background, isolated worktrees).** Pick
   persona×lens pairs for the scope from `reference/personas.md` — **≥1 adversarial**
   (YAGNI skeptic and/or velocity red-team; mandatory counterweight), the **metrics**
   pass routed to **Codex/GPT** (independent model family), and — for any
   runtime-touching scope — the **Concurrency & goroutine-lifecycle** persona.
   Each worker is read-only, writes **exactly ONE** artifact to a disjoint path,
   grounds every claim in `file:line`, and returns the evidence contract.

   **Copyable persona delegation prompt:**
   ```text
   Repo: ghx (/Users/goga/Documents/goga/ghx). READ-ONLY architecture audit — no code changes.
   Read first: docs/NORTH_STAR.md + AGENTS.md tenets + the charter AA-000N-charter.md. Hold the
     whole-codebase big picture; every finding ties to a named tenet / NORTH_STAR phase.
   Your persona/lens: <name + worldview + reflexive flags + blind spots> from
     .claude/skills/arch-audit/reference/personas.md (persona #N). Role-play it honestly.
   Scope: <the charter's scope + ranked quality attributes>. Ground EVERY claim in file:line.
   Re-price the canon for OUR context before applying it (research/05-agentic-first-lens.md):
     maintainability = agent-legibility (minimal legible edit unit / context-tax), NOT human-team
     ergonomics; YAGNI weights UP; both god-files AND over-fragmentation fail concern-locality.
   Tools: text-only (WebSearch/WebFetch/curl/gh); no browser/screenshots (AGENTS.md Tool Economy).
   Write ONE artifact to docs/audits/AA-000N-<scope>/AA-000N.<k>-<persona>.md (ADR-style, template in
     reference/artifact-template.md). Include "What is already right" (verified nulls, equal rigor).
   Frontmatter MUST include: persona, lens, and model+family (e.g. opus-4.8 / claude, or gpt-5.5 / openai)
     — the distiller uses family to tell independent voices from shared-prior ones.
   Return: artifact path · top findings · what you checked and found SOUND · least-confident call.
   ```
3. **Land each artifact ff-only** — rebase in its own worktree, then `git merge
   --ff-only` (never `--amend` on a shared mainline; a parallel engineer may be
   committing — see `CLAUDE.md` "Git Hygiene on a Shared Mainline"). Disjoint
   artifacts → no conflicts.
4. **Converge & distil.** When all persona artifacts are in, a **fresh, no-stake
   agent** drafts `AA-000N.9-distilled.md`: mine the intersection, **re-run every
   recompute/metric independently**, and rank. **Rank on INDEPENDENT recompute ×
   impact × tractability — NOT on head-count.** Cross-persona agreement among
   same-family (all-opus) personas is *shared prior, a flag to audit*, not
   corroboration (self-preference — Panickssery et al., NeurIPS 2024). Hold tensions
   explicitly; the lone correct voice can outrank a converged wrong one.
5. **Judge (see "The final judgment") and govern.** Turn the accepted, ranked
   sequence into a governing ADR — **with an execution gate**: each step names the
   frontier work it unblocks, or it is watch-list, not a task. Then execute the
   refactors as verified worker tasks (behavior-preserving, ADR-gated, landed ff-only).
6. **Graduate a *few* recurring findings into standing fitness functions — the
   audit's most durable output.** A parallel agent fleet erodes convention-only
   boundaries between audits, so the enforcement that survives is compiler-enforced
   (`internal/`, unexported, acyclic imports) + **committed executable checks**. The
   canonical one is **`go test -race` in CI** (ghx runs it per-ADR by hand today; it
   belongs in the standing build). Graduate a guard **only** when the boundary is
   load-bearing — it was violated-and-caught once, or a 2nd implementation now exists
   (the skill's own "one implementation forever" bar) — and each guard names its
   owning ADR (`// change only via ADR-XXXX`). **Do NOT** ossify a transient check:
   the persona-byte golden was explicitly a *refactor* check that must never become an
   invariant (it would block legitimate B8/ADR-0029 persona revisions); a god-file
   *size* ceiling contradicts the churn×complexity hotspot metric. A guard with no
   owner and no expiry is debt, not a fitness function.

**Deliverables:** the charter, N persona artifacts, one distillation artifact, a
governing ADR (or an explicit "do-nothing + watch-list"), and any new fitness-function
check — under `docs/audits/` + `docs/adr/` + tests, each recomputable from committed evidence.

## Choosing a scope (the run's biggest lever)

- **A capability / reusable core** — the highest-value scope for ghx's vision: is
  the **Agent Sidecar Framework** a reusable infrastructure boundary or welded to
  ghx? Is the **shared core between evals and the ghx product** a real DDD **Shared
  Kernel** (small, deliberately coordinated) or accidental duplication? Re-price the
  reuse decision as a **context-tax calc**: justify a shared core only when it
  *reduces fleet-wide context tax*, not merely because it removes duplication — the
  agentic winner for a recurring concept is often a *named type*, not a dependency
  (copy vs. depend vs. name; `research/05 §5.2`).
- **A runtime module on the frontier** — `internal/sidecar` (daemon B6, routing B7,
  the runner port ADR-0036, watchdog/recovery ADR-0027): the concurrency axis where
  ghx's velocity and risk actually live.
- **A module** — `internal/sidecar/evals`, `internal/ghx`, `internal/mapengine`,
  `internal/codemode` — when a measured hotspot justifies it.
- **A cross-cutting lens** — decoupling & replaceability, resilience, testability,
  dependency direction, domain modeling.

Narrow, trigger-justified scopes get sharper, more actionable artifacts than "audit everything".

## Personas × lenses (catalog in `reference/personas.md`)

Pick per scope; always include ≥1 adversarial and the Codex metrics worker; add the
**Concurrency** persona for any runtime-touching scope. The roster: **Go architecture
& boundaries**; **domain modeling** (value objects/entities that flow); **reusable-core
extraction** (framework-vs-product, evals↔ghx shared kernel, context-tax); **ports &
adapters / replaceability**; **resilience & runtime robustness**; **Concurrency &
goroutine-lifecycle** (goroutine ownership, data races, channel/select shutdown —
gated to runtime scopes); **complexity/tech-debt metrics** (gocyclo/gocognit/dup/
error-sprawl **+ complexity×git-churn hotspots** — the ~1–2% of files where refactoring
buys the most velocity, not the biggest files — routed to **Codex/GPT**); and the
adversaries — **YAGNI skeptic** and **velocity red-team**, run as a real discipline
(premortem / Team-A-vs-Team-B) at the **DH5/DH6 bar** (quote the exact passage, name the
exact flaw). Personas must genuinely *disagree*.

## Artifact contract (template in `reference/artifact-template.md`)

Every artifact is ADR-style: YAML frontmatter (`title, date, status: audit|distillation,
thread, scope, persona/lens, model, family, author`) + body: **Executive summary**;
**"What is already right"** (verified + cited — mandatory; it stops manufactured findings
and legitimizes "do nothing"); **severity tiers (High/Med/Low)** each with `file:line`
evidence, a tie to a named AGENTS.md tenet / NORTH_STAR phase, a **Novelty** tag
(`NEW` / `KNOWN-CONFIRMED⟨ref to prior AA-000N/ADR⟩` — a zero-delta restatement of a
dispositioned finding is *referenced, not re-derived*), and a concrete recommendation;
a **"Recommended sequence"** (phased, non-rewrite, each step shippable + behavior-
preserving, each naming the frontier work it unblocks — or it is watch-list); and
**"Method / auditability"** (the exact commands so every number recomputes). Where the
audit finds a **governing ADR or a tenet itself stale** (overtaken by the code's
reality — e.g. ADR-0010's boundary rule), that is a high-value finding class: name it
with evidence and propose its *evolution*, don't audit against a dead constitution.
The distillation artifact adds an **independent-recompute column** (the real ranking
signal) and explicit tension-resolution. **Threading:** `AA-000N` per run, `.1/.2/…`
per persona, `.9-distilled`, mirroring `product-audit`'s `PA-000N`.

## The final judgment (the load-bearing step)

You (the orchestrator) are the authoritative judge, and judging your own delegated
fan-out is a self-preference regime (Panickssery et al., NeurIPS 2024 — self-recognition
causally drives self-preference), made worse because the code, the audit, and the judge
may all be the same model family. So:
- **Re-derive load-bearing claims yourself** — open the cited `file:line`, run the
  recompute — before accepting. **Eat the fish, throw the bones** — support or veto each
  distilled idea with evidence.
- **Rank on the independent-recompute column, not convergence.** Same-family agreement
  is a shared-prior flag, not corroboration.
- **Cross-family adjudication is mandatory, not optional, for the highest stakes:** route
  the metrics to a Codex/gpt-5.5 worker AND have that worker review **≥1 top-ranked
  *design* finding** (not only `wc -l`). If no cross-family checker is available, a fresh
  no-stake same-family agent is a *partial* fallback (removes stake, not family bias) —
  flag anything checked only same-family.
- **"No", "this is fine", and "the premise is wrong" are valid outcomes** — adding a
  refactor has real cost (it competes with the brain). **Record what you declined and why.**

## Guardrails / hard rules (learned from the 2026-07-07 runs + adversarial review)

- **Ground every claim in `file:line`.** An unsourced finding is a hypothesis.
- **The "what's already right" section is mandatory** — it prevents manufactured debt
  and makes "do nothing" a real outcome (the 2026-07-07 run verified the core/frontend
  boundary was *healthy* — as valuable as the findings).
- **Maintainability = agent-legibility, not human-team ergonomics** (`research/05`).
  The maintainer is a stateless fleet re-reading cold every session, bounded by a context
  window — so cohesion is a *context budget*: measure the **minimal legible edit unit**
  (tokens a cold agent loads to change one concern). **Both god-files AND
  over-fragmentation fail concern-locality** — "smaller" is not always better.
- **Respect the frozen measurement stack.** Never propose changing eval scoring, gates,
  detectors, or reward math as a drive-by; flag and defer behind a pre-registered eval
  ADR (AGENTS.md Visibility & Truthfulness). **Behavior-preserving ≠ measurement-preserving:**
  a green-test refactor of an `evals`-reachable helper can silently move a citable score —
  treat any `internal/sidecar/evals`-reachable change as measurement-touching.
- **North-star filter + no speculative frameworks + idiomatic Go.** Right-size to a
  solo-dev Go product: no `internal/domain` cathedrals, no DI containers ("in Go the
  composition root *is* the DI container"), no abstraction of a component with exactly one
  implementation forever. The YAGNI persona enforces this; the judge honours it.
- **Behavior-preserving, phased, non-rewrite.** Every step ships on its own and is provable
  byte-identical (unmodified suite + race) unless it deliberately versions a change.
- **Forward-looking, agentic-first.** ghx's consumer is an AI agent and this is 2026 —
  learn from architecture history, never cargo-cult a pre-AI-scale dogma. The canon
  *holds* on coupling/dependency-direction; it inverts only on human-team-maintenance
  assumptions (onboarding, ramp-up, workshop ceremony) — `research/05`.
- **Integration mechanics:** isolated worktree per worker, commit ONE artifact, land
  ff-only, never `--amend` on a shared mainline. Every worker holds the big picture
  (NORTH_STAR + AGENTS.md tenets) before it starts. Mechanics: `fable-delegation`.

## Provenance

**Worked example** (the loop made concrete): the 2026-07-07 architecture run —
`docs/audits/architecture-2026-07-07.md`, `docs/audits/architecture-vision/*.md`,
`docs/audits/refactor-review-2026-07-07.md`, distilled into `docs/adr/0035-*` and
`docs/adr/0036-*`. Read those for the altitude and the `file:line` rigor.

**Canon & sourcing** (skill-forge Round 0 — the citation of record; the SKILL stays thin,
the ledgers carry the URLs): `research/01-idiomatic-go-architecture.md`,
`research/02-clean-architecture-ddd-reusable-modules.md`,
`research/03-tech-debt-detection-metrics.md`, `research/04-architecture-audit-methodology.md`,
and the mandatory Wave-2 `research/05-agentic-first-lens.md` (re-prices the canon for an
AI-fleet-maintained codebase). ~108 deep-URL sources across Go Proverbs/Ben Johnson/Kat Zien;
Clean Architecture/Cockburn/Evans Shared Kernel; Cunningham/Fowler/Tornhill hotspots;
ATAM/premortem/fitness functions; Anthropic context-engineering; LLM self-preference.
**Round-2 critiques:** `reviews/01-craft-usability.md`, `reviews/02-completeness-fit.md`,
`reviews/03-meta-redteam.md`.

**Reference material:** `reference/personas.md` (persona catalog), `reference/artifact-template.md`.
