---
name: product-audit
description: >-
  Use when asked to audit the ghx product from a product-manager's perspective —
  to find issues, gaps, north-star misalignment, PMF / market / competitive
  questions, adjacent-idea proposals, and agent-experience problems. Runs the
  product through MULTIPLE adversarial PM personalities across strategic and
  agent-native scopes and surfaces (code, docs, features, evals, live manual
  runs, the strategy docs themselves). Agentic-first: ghx's primary consumer is
  an AI agent, so classic PM advice is re-priced before it is applied. This is a
  PRODUCT/strategy audit, not a code-quality or security audit.
---

# Product Audit — the Product-Manager Lens

Audit `ghx` as a product manager would: not "is the code right?" but "are we building
the *right thing*, for the right consumer, aligned to the north star, in a way that can't
be trivially copied?" The audit runs from several **adversarial PM personalities**, across
selected **scopes** (PMF, competition, market, positioning, gaps, adjacency, moat,
monetization, and the agent-native scopes), over selected **surfaces** (code, docs,
features, evals/traces, live manual runs, and the strategy documents themselves).

Fable is the **authoritative judge**: breadth is delegated to background persona-agents;
the accept/reject call is not. Incorporate a finding because it survives scrutiny, never
because an agent asserted it. The full sourced research — every framework with specific
citations and per-claim rationale — is in `research/*.md` beside this file (their Source
Ledgers are the citation of record); Round-2 critiques are in `reviews/*.md`.

## When to use / when not

**Use** when asked to: audit the product / find product gaps / pressure-test the north star
or an ADR *as product strategy* / assess PMF, market, competition, positioning, or
defensibility / propose adjacent product ideas / check agent-experience (AX) of the CLI,
MCP tool, sidecar report, or docs / check roadmap-to-strategy traceability.

**Do not use** for code correctness, architecture/complexity, or security — those have
their own homes (see § In scope / out of scope). This skill may *read* code as evidence
for a product finding, but its findings are always framed in product terms.

---

## Prime directives (read before auditing)

1. **The north star is the yardstick.** Read `docs/NORTH_STAR.md` first. Every finding
   states its relationship to the **north-star filter**: does the thing remove
   tokens/knowledge from the main agent's context, make reconnaissance cheaper or more
   proficient, make evidence more auditable, or produce better training trajectories? A
   finding that ignores the north star is out of scope; a *product move* that fails the
   filter is itself a finding. **And the yardstick itself is auditable:** where the north star
   or a tenet has fallen behind ghx's velocity, name it with evidence and propose its evolution
   — a high-value finding class. The north star evolves *deliberately* (not re-litigated every
   run); respect the tenets, challenge them only with evidence, and we all evolve together.

2. **Agentic-first — re-price classic PM advice before applying it.** ghx's primary
   consumer is an **AI agent**, not a human; most PM canon assumes a human with eyes,
   memory, felt emotion, and an attention budget. This is **2026**: we build an agentic-first
   product with agentic-first engineering, forward-looking — learn from history, but never
   adopt an archaic, pre-AI-scale idea just because it once worked (the world it worked in had
   no AI at this scale of capacity, influence, or coherence). **Every delegated agent —
   personas, validators, the distiller — must hold this whole picture** (`NORTH_STAR.md` vision
   + tenets, `AGENTS.md` tenets, this posture) and never audit blindsided of it. Tag each
   framework before use (full audit: `research/06-agentic-first-lens.md §3`):
   - **HOLDS** — outcomes-over-output, JTBD (as a lens), 7-Powers, the red-team stack,
     Cagan's four risks, PM craft and the personas themselves.
   - **ADAPT** — Nielsen heuristics, journey mapping, positioning, information-scent/docs:
     the "screen a human perceives" becomes the "schema a model parses."
   - **OUTDATED-OR-INVERTED** — human onboarding/first-run (→ *zero* doctrine), HEART
     Happiness (no affect), the Sean-Ellis survey (→ ablation evals), "delight" (→ delight
     theater), the 5-user rule (→ many episodes, measure consistency). **Engagement/retention
     splits:** per-task engagement (tool-calls, time-in-tool) is a **vanity metric to drive
     DOWN** for a context-subtracting product — but **cross-session tool-selection / routing
     retention is the real agent-PMF signal** (does the agent keep choosing ghx across
     sessions). Never collapse the two.

3. **Two consumers, never conflated.** The **agent** is primary; the **human operator /
   buyer / market** is a real secondary consumer. Agent-native scopes (Tier B) serve the
   first; strategic/market scopes (Tier A) serve the second. Never let a founder's
   aesthetic or a human vanity metric masquerade as *agent* value — nor dismiss the
   operator's needs.

4. **Adversarial, but honest.** Attack hard, but a red-team that cries wolf is useless.
   Every report carries a **"What is actually fine"** section with equal rigor, and commits
   negative/null results. "We tried to break this and couldn't" is a citable finding.

5. **Evidence, not vibes — and the judge is itself checked.** Every finding cites
   files/commands/outputs/traces, recomputable by a human; a claim that cannot cite is a
   hypothesis. **A Claude orchestrator adjudicating Claude workers is a self-preference
   regime** (arXiv:2404.13076 — LLM evaluators favor their own generations), so "trust
   nothing on assertion" is not enough by itself. Apply the repo's own visibility
   discipline to *this audit's* judge (see Process step 5). Treat agreement among Claude
   personas as possible shared-prior, **not** independent corroboration.

6. **Scope the attack surface — focused, not exhaustive; narrow *and* holistic.** Every audit
   declares a **focus** up front — a milestone, feature, goal, vision thread, or surface: the
   *attack surface* for this run. Do **not** re-audit the whole product from inception. As ghx
   grows, whole-product sweeps still happen but stay occasional, event-driven, and *still carry
   a focus*; re-litigating settled features every run is a smell — a finding already raised and
   dispositioned in a prior `PA-000N` is **referenced, not re-derived**. **But scoped ≠
   narrow-minded:** assess the focus both *narrowly* (is this feature/milestone good on its
   own?) and *holistically* (does it align with the whole ghx product, the agentic-first
   engineering tenets, the north-star vision, and the *cohesion* of what already exists?). A
   feature can be locally sensible yet globally futile or counterproductive to product
   cohesion — naming that is one of the audit's highest-value outputs. Record the focus and this
   run's in-scope / out-of-scope in the artifact frontmatter (Output contract).

---

## Process (how the orchestrator runs an audit)

**Right-size first.** Most product questions need a single sharp lens, not a fleet — a
quick targeted pass is the default (mirrors the repo's eval cadence: spot-check first,
big runs are rare and event-driven). Spin up the full multi-persona delegation only for a
milestone-grade or genuinely cross-cutting audit. Do not boil the ocean by reflex.

1. **Ground & focus.** Read `docs/NORTH_STAR.md` (+ the relevant ADR/milestone). Declare the
   audit's **focus / attack surface** (the milestone / feature / goal / vision thread under
   audit), its goal in one sentence, the decision it informs, and this run's **in-scope /
   out-of-scope** — then hold the whole product, tenets, and vision in mind while auditing that
   focus (directive 6). No focus ⇒ don't run it; a from-inception sweep is not a focus.
2. **Scope the audit — cap all three axes.** Choose **3–5 personas** (span the axes; ★
   near-mandatory), **3–5 scopes** (Tier A and/or Tier B — Tier B is *selective*, not
   automatic), and **only the surfaces where those scopes' evidence lives**. Personas,
   scopes, *and* surfaces are all capped; running everything drowns signal.
3. **Delegate breadth.** Spawn read-only background persona-agents (text-only tools; no
   worktrees needed), one per persona, using the template below. Any GitHub/OSS exploration
   in the audit (competitive, discovery, absorption scouting) **dogfoods ghx itself** — using
   the product to audit the product both proves it and yields free eval signal. **Every
   delegated agent (personas, validators, the distiller) first internalizes `NORTH_STAR.md` +
   `AGENTS.md` tenets and judges against the whole-product big picture (directive 2) — none
   audits blindsided.**
4. **Each persona-agent** returns a findings artifact *and* a "what I checked and found
   sound" (nulls) section — persona-level honesty, not just report-level.
5. **Judge (the load-bearing step).** For each returned finding: does the evidence hold on
   *your own* re-derivation? Is it DH5/DH6? Does it respect the agentic-first re-pricing
   (not cargo-culted)? Is it in scope? Reject, merge duplicates, re-severity. **For every
   High / thesis-invalidating finding, the "orchestrator is judge" claim only holds with a
   mechanism, not a vibe:** re-derive it from raw evidence yourself, get an **independent adjudication** before it is
   citable — *prefer* a cross-family checker (Codex/gpt-5.5 via `fable-delegation`) to
   neutralize model-family self-preference; if unavailable (e.g. usage-capped), fall back to a
   **fresh, independent Claude adjudicator** (a new agent, no stake, adversarial brief) — a
   *partial* mitigation that removes the stake/shared-context bias but **not** the same-family
   bias, so flag any finding that got only a same-family check. Record *who judged it and how a
   reader checks the judge* —
   the same calibration the product demands of its own eval judge (`AGENTS.md`
   Visibility/Truthfulness). Do not run a milestone decision on an unchecked judge. **Recompute
   each finding's exact quantity via its own command** — measure the *same* thing it claims
   (comparing a whole-file line count to a rendered-doctrine count is evidence-drift; the PA-0001
   retrospective caught exactly this). Then **order accepted findings by leverage** (north-star
   impact × cheapness of fix), not severity alone — tell the reader what to do *first*.
6. **Rank & synthesize** into one report (contract below): summary → ranked findings →
   *What is actually fine* → adjacent-idea proposals. Rank by severity, then north-star
   leverage.
7. **Honest close.** Commit negatives; state what was *not* audited; name residual
   uncertainty. For a high-stakes audit, run a second round with a different persona set,
   or red-team the audit's own findings.
8. **Convergence & distillation (final pass).** After the synthesis, delegate a **fresh,
   no-stake agent** to mine the whole thread — cross-validate findings across the codebase,
   **re-run the reproduction/recompute commands**, do any manual validation — and distil the
   **intersection of ideas** into a final artifact (`PA-000N.9-distilled`). **Genuine
   convergence across *independent* personas and evidence is high signal; a lone suggestion is
   low signal — but discount *shared-prior* convergence** (agreement because agents read the
   same source is not independent corroboration, directive 5). You remain the judge: read and
   respect every perspective, then **support or veto each distilled idea with evidence and
   cross-references — eat the fish, throw the bones** (there are many). The distiller surfaces
   and ranks; it never stands unjudged.

**Copyable persona-agent delegation prompt:**

```text
Repo: ghx (/Users/goga/Documents/goga/ghx). READ-ONLY product audit — no code changes.
Read first: docs/NORTH_STAR.md. Every finding states its north-star-filter relationship.
Your persona: <name> — worldview / reflexive flags / signature questions / blind spots from
  .claude/skills/product-audit/research/02-pm-personalities.md (persona #N)  [AX: 06 §4].
  Role-play it honestly, INCLUDING its blind spots.
Scopes to apply: <chosen 3–5>  (frameworks + agentic-first HOLDS/ADAPT/INVERTED verdict in
  research/04-strategic-scopes.md and research/06-agentic-first-lens.md §3 — re-price before applying).
Surfaces: <only where evidence lives> — CLI/MCP/sidecar output · docs · evals+traces
  (~/.ghx/sessions/, docs/evals/) · a live manual run · the strategy docs themselves.
Tools: text-only (WebSearch/WebFetch/curl/gh). No browser automation, no screenshots.
  For GitHub/OSS exploration (competitive/discovery/absorption scouting), DOGFOOD ghx itself
  (its discovery tier is built for this) — fall back to gh/web only where ghx can't yet do
  the job, and log that gap as a finding.
Deliver a findings artifact. Each finding: persona·scope·surface | novelty (NEW/KNOWN⟨ref⟩) |
  EVIDENCE across every applicable class, each verified — SOURCE (file:line + quoted snippet +
  a recompute command), STRATEGY (the exact tenet / north-star / ADR passage it bears on,
  quoted), EXPERIMENT (the committed eval / manual-run / trace artifact path + recompute) |
  severity (Likelihood×Impact) | CROSS-REFS as SPECIFIC deep hyperlinks + one line on why each
  matters (external framework, competitor, AND the OSS repo/file where an idea is inspired or
  absorbed — never a bare homepage; verify each resolves). DH5/DH6 bar: quote the target, name
  the flaw. Add a "what I checked and found SOUND" (nulls) section.
Return: artifact path, top findings, source count, least-confident call.
```

Delegation mechanics: `.claude/skills/fable-delegation/SKILL.md`.

---

## Output / evidence contract

Write to `docs/product-audit/` — **numbered and threaded like ADRs**, and kept separate from
the *code* audits in `docs/audits/`. An audit round is `PA-000N` (the synthesis report); each
persona-agent writes a threaded artifact `PA-000N.k-<persona>.md`; the synthesis is
`PA-000N-<focus>.md`. Match the repo's audit house style. Frontmatter: `title`, `date`,
`status: "audit"`, `audit: "PA-000N"`, `focus` (the milestone / feature / goal / vision under
audit), `in-scope` / `out-of-scope` (what this run examines vs. deliberately defers — distinct
from the skill's fixed charter boundary below), `persona`/`author`, `scope` (one line: surfaces
+ personas + "read-only, no code changes"). Then (for the synthesis `PA-000N`):

- **Executive summary** — the single highest-leverage finding first, in north-star terms.
- **Evidence & cross-reference ledger (decision-grade)** — the receipts, consolidated so a
  decision-maker acts from this doc alone (not by spelunking the persona artifacts): per top
  finding, its **source** (`file:line` + quoted snippet + recompute command), **strategy** (the
  exact tenet / north-star / ADR passage), **experiment** (the eval / manual-run / trace artifact
  path + recompute), and **external / OSS-inspiration** (specific deep hyperlinks + why each
  matters, and — for absorbed ideas — what we took). Conclusions without this ledger are **not
  decision-ready**; build it in the synthesis (delegate to a dedicated evidence agent if large).
- **Ranked findings**, each with:
  - **Persona · Scope · Surface** it came from.
  - **Novelty** — `NEW` / `KNOWN-CONFIRMED⟨ref⟩` / `KNOWN-DISPUTED⟨ref⟩`: does this already live
    in `TRUST.md` / an ADR / a prior `PA-000N`? If known, cite it and state the *delta* you add;
    a zero-delta restatement is **not** a finding (anti-repetition, directive 6).
  - **Evidence** — cite by **symbol + a quoted snippet + a one-line recompute command**, not a
    bare `file:line` (line numbers drift). Recomputable by a human; the judge recomputes the
    finding's **exact quantity** via that command.
  - **Severity** (Likelihood × Impact) and **Disposition** — *sound* / *needs work* /
    *invalidates*. A "needs work" finding **routes to a workstream (A/B/C) or proposes an
    ADR**, not a bare "owner + date".
  - **North-star relationship** — filter pass/fail, or the tenet/milestone it bears on.
  - **Verification state** — `self-verified` / `same-family-adjudicated` / `cross-family-adjudicated`;
    a **High finding is not milestone-citable below `cross-family-adjudicated`** (Process step 5).
    Name who judged it and how a reader re-derives it (self-preference guard, directive 5).
  - **Cross-reference** — for any external claim (framework, competitor, **or the OSS repo/file
    where an idea was inspired or absorbed**): a **specific deep hyperlink** (exact GitHub
    file/repo, essay, paper, doc — never a bare homepage) + one line on **why it matters** (and,
    for absorbed ideas, *what we took*). Verify each resolves; no vague or invented links; if
    unverified, say so.
- **"What is actually fine"** — supposed problems checked and found sound, equal rigor.
- **Adjacent-idea proposals** — labelled as proposals, each filtered against the north star
  and named on the Ansoff grid (penetration / development / diversification).
- **Absorption candidates** — open-source/competitor components worth absorbing (idea →
  utility → feature → whole tool under the hood), each with *what to take*, *where it plugs
  in*, and its *north-star-filter alignment*. Steal openly, with attribution; competition is
  a supply of building blocks, not a threat (`AGENTS.md` Open Source Leverage).
- **North-star / tenet evolution proposals** — where the audit found the *yardstick itself*
  stale or lagging ghx's velocity: name it, with evidence, and propose the evolution
  (deliberate, not per-run — directive 1). Respect the tenets; challenge them only with evidence.
- **What was not audited** — axes deliberately omitted this round.

The **`PA-000N.9-distilled`** convergence pass (Process step 8) is the final deliverable: it
ranks the above by cross-persona *independent* convergence × leverage, and separates the
high-conviction recommendations from the discarded bones.

---

## The audit model: Personalities × Scopes × Surfaces

Select a capped set on each axis (§ Process step 2), sized to the goal. Never run
everything every time.

| Axis | What it is | Cap |
|---|---|---|
| **Personalities** | *Who* audits — an adversarial PM lens with its own worldview, flags, and blind spots. | 3–5 (span the axes); full roster only milestone-grade. |
| **Scopes** | *What* we audit for — a strategic/market or agent-native question. | 3–5; Tier B is selective, not automatic. |
| **Surfaces** | *Where* the evidence lives — code, CLI, docs, evals/traces, live runs, strategy docs. | Only those the chosen scopes need. |

---

## Personalities (the audit lenses)

Full self-contained blocks are in `research/02-pm-personalities.md` (1–15) and
`research/06-agentic-first-lens.md §4` (the AX lens, #16). ★ = near-mandatory for ghx.

| # | Persona | Reflexively flags / optimizes for | One signature question |
|---|---|---|---|
| 1 | Feature-Factory Skeptic | Output masquerading as outcome | "What outcome did this move, and how would we know if it didn't?" |
| 2 | User-Empath | Roadmaps from opinion, not observed use | "When did anyone last read a raw agent trace of this being used?" |
| 3 | Monetization Hawk | Unowned "who pays"; value/cost mismatch | "If cost-to-serve (tokens) doubled tomorrow, does this survive?" |
| 4 ★ | Platform/Scale Realist | Contracts changed without notice to consumers-on-top | "Is our surface a contract other agents can rely on?" |
| 5 | Technical-Debt Realist | 100% capacity to net-new; debt untranslated to velocity | "What % of capacity is sustainability work, and is it declining?" |
| 6 | North-Star Zealot | Vanity/gameable metric | "Could this metric rise 20% while the product got *worse*?" |
| 7 | Competitive-Paranoid | Strategy assuming today's landscape is static | "What would let a competitor make this irrelevant in 12 months?" |
| 8 | Simplicity/Anti-Bloat Minimalist | Flag/subcommand sprawl | "What does this feature cost every user who never touches it?" |
| 9 ★ | Experimentation Rigorist | Single-metric wins; small N; uncalibrated judge | "Sample size, guardrails, could a confound explain it?" |
| 10 | GTM/Positioning Strategist | "Who is this for" that resolves to "everyone" | "What's the *true* alternative the agent would use instead?" |
| 11 | Growth Systems Thinker | Isolated tactics; unmapped adoption funnel | "Where does the adoption funnel actually leak?" |
| 12 | Craft Purist | Process/decks over product; excuses over ownership | "Could you use this for a real task right now and be satisfied?" |
| 13 | Zero-to-One Explorer | 0→1 bets run with feature-team rigor; never killed | "What's the riskiest assumption, and did we test *that* first?" |
| 14 | DevEx Advocate *(human-operator layer)* | Operator's install/config/first-run friction | "Could the human operator set this up with zero prior context?" |
| 15 ★ | AI PM Pragmatist | "AI-powered" as a feature with no eval; thin wrapper | "What traced, calibrated eval backs this claim, and who scored it?" |
| 16 ★ | **Agent Experience (AX) lens** | Tool undiscoverable/unselectable; output that won't compose; doctrine tax | "If a fresh agent had only this tool's name, description, and one error, could it do the job and *know* it succeeded?" |

Note: the **AX lens (#16) is the agent-consumer umbrella** and subsumes the *agent* side of
DevEx; #14 is kept only for the **human operator**. AX's four pillars (Access / Context /
Tools / Orchestration) are the concrete facets audited by the Tier-B scopes below.

**Composing a set** (`research/02 §5`, `06 §4`): span the axes (Mehta Execution/Insight/
Strategy/Influence; Doshi–Cagan Craftsperson/Operator/Visionary; Reforge Feature/Growth/
Scaling/PMF-expansion) — three Operator/Execution personas together find engineering
reality and nothing about whether the product should exist. **Redundant pairs, pick one:**
Feature-Factory Skeptic ↔ Craft Purist; Growth Systems Thinker ↔ Monetization Hawk;
Experimentation Rigorist ↔ North-Star Zealot. **Starter set (5):** Feature-Factory Skeptic
+ User-Empath + Technical-Debt Realist + Platform/Scale Realist + Competitive-Paranoid,
plus one measurement anchor. **For ghx:** the ★ lenses (AX, AI PM Pragmatist, Platform/
Scale Realist, Experimentation Rigorist) are near-mandatory — grounded in ghx's real domain.

---

## Scopes (what we audit for)

Full frameworks + failure modes + sources: `research/04-strategic-scopes.md` (Tier A) and
`research/06-agentic-first-lens.md §3–4` (Tier B + the verdict on each Tier-A framework).
Apply each with its agentic-first verdict (directive 2). Pick 3–5 total.

### Tier A — Strategic / market / business (the human & operator layer)

| Scope | Core question | Primary framework(s) |
|---|---|---|
| Product-Market Fit | Real population seriously hurt if it vanished — or still searching? | Rachleff/Andreessen; Ellis 40% test → **for the agent, ablation evals**, not a survey |
| Competitive Analysis | What structural forces decide who captures value, vs. the *true* alternative? | Porter Five Forces; Dunford competitive-alternatives |
| Industry / Market | How big, how mature the value chain, *why now* — is the tool layer commoditizing as base models improve? | TAM/SAM/SOM (Aulet); Wardley evolution |
| Positioning & Differentiation | Would an informed prospect find it "obviously" right? (for the agent, "messaging" = tool name/description/when-to-use) | Dunford; category design |
| North-Star / Strategy Alignment | Does every roadmap item trace to the stated strategy? | Perri Vision→Challenge→Target; Amazon PR-FAQ; Cutler |
| Gap Analysis | Where does it under- or over-serve a *real named job* — and what latent demand shows up as **desire paths** (near-miss commands agents keep typing that don't exist yet, via `frictionax`), not just failures? | Christensen JTBD; Ulwick ODI opportunity score; journey → trace mining |
| Adjacent Opportunity / Expansion | Next *defensible* expansion, and is it premature? | Ansoff matrix; Dixon "come for the tool, stay for the network" |
| Business Model / Monetization | Can the business capture & sustain value? (internally: cost-to-serve/episode) | Cagan four big risks; Jacks COSS/open-core |
| Moat / Defensibility | What survives a competitor copying every visible feature? | Helmer 7-Powers (Benefit+Barrier); NFX; Thompson aggregation |

> **The "true alternative" is not "the agent greps files itself."** It is the host's
> **native, context-subtracting recon subagent** — e.g. Claude Code's read-only **`Explore`**
> agent (cheaper model, keeps exploration out of the main context): ghx's own thesis shipped
> natively inside ghx's primary host. Naming it *sharpens* the moat rather than kills it —
> Explore has no evidence contract, no persisted session artifacts, and no GitHub-wide
> discovery — but the competitive/positioning/moat scopes only work if this is the named
> comparison, and ablation baselines (PMF scope) should include native-Explore, not only
> plain/ghx/ghx-sidecar. (Verify current behavior: `code.claude.com/docs` sub-agents.)

> **Competitive analysis here is generative, not defensive** (`AGENTS.md` Open Source
> Leverage). Score every competitor / OSS tool for what we can **absorb** — a small idea, a
> utility, a feature, a vision, or the whole tool swallowed under the hood (NORTH_STAR P3) —
> gated only by the north-star filter and the Picasso rule (steal openly, with attribution;
> hand-roll only when nothing serves the vision). Our vision is unique; competition is a
> supply of building blocks, never a reason to retreat. So these scopes feed **Absorption
> candidates** in the report, not only threat lists — that is what the Competitive-Paranoid
> (#7) lens turns *into*: doing more, not backing down. (Artifact 07 is this in action —
> AX rubrics absorbed from `axprobe`/`promptfoo`, prior-art negative kept.)

### Tier B — Agent-native (the AX umbrella; ghx's home turf) — selective, pick what the goal needs

| Scope | Core question | Grounding |
|---|---|---|
| **Agent Experience (AX)** | Can the agent discover, invoke, recover-from, not-be-misled-by, and compose it? | Biilmann AX; Anthropic ACI |
| **Token economics / signals-per-token** | Context tax per unit of returned signal — **at all three SPT levels** (main-agent, sidecar-internal ~400-line persona-doctrine tax, whole-workflow vs. native delegation) | Anthropic context-engineering; NORTH_STAR ADR-0016.6 |
| **Evals-as-user-research** | Does an eval suite serve as user research, and is it *trustworthy* (calibrated judge, guardrails, committed negatives)? Decompose "task success" into sub-metrics — tool-correctness, argument-correctness, task-completion, MCP-task-completion (via `deepeval`). | Hamel evals; **code-localization/retrieval benchmarks (LocAgent, RepoBench, CodeSearchNet — ghx's actual job)**, plus SWE-bench / τ²-bench for downstream task success & pass^k consistency |
| **Tool / affordance & error-as-affordance** | Do names/schemas make the right call obvious, invalid states unrepresentable, and does every error name the correct next invocation? | Anthropic writing-tools; OpenAI function-calling (<20 active) |
| **Context-budget / progressive disclosure** | How much doctrine must load into the *main* agent before first success? Ideal: zero. | Anthropic Agent Skills; NORTH_STAR |
| **Trust & verifiability of output** | Can a downstream actor audit the claim? Is a confidently-wrong output distinguishable from a right one — **and does surfaced code carry copyleft/attribution lineage that flows into the agent's output invisibly?** | "evidence not vibes"; confidently-wrong = severity-4; license-provenance (Doe v. GitHub-class risk) |
| **Agent-safety / adversarial tool-use** | *Enumerate* the attack surface, don't just name it: indirect prompt-injection via surfaced repo content → the report → the main agent's next action; cross-session / artifact leakage from `~/.ghx/sessions/`; excessive-agency & goal-misalignment when output feeds an autonomous loop; reversibility / dry-run | Willison lethal trifecta; red-team stack; the **promptfoo red-team plugin catalog** (`github.com/promptfoo/promptfoo` — enumerated agentic attacks) |
| **Distribution in agent ecosystems** | Reachable and *default* in MCP/skill ecosystems; does the agent *select* it over the alternative? | MCP spec; tool-selection accuracy in evals |

---

## Surfaces (the audit is not only code)

Live product/features (real CLI, MCP tool, a real sidecar `ask`, the **discovery tier** —
repo-optional GitHub-wide recon) · the CLI/tool contract (flags, schemas, output, error
messages) · docs (README, `ghx skill`/`--mcp`, ADRs, `docs/*`) · **evals & traces**
(`~/.ghx/sessions/` **as an agent-facing API surface** — NORTH_STAR names the main agent a
first-class consumer of these artifacts, not only human debug; eval reports under
`docs/evals/`) · **the strategy documents themselves** (`NORTH_STAR.md`, ADRs, milestone
tables — audited *as product strategy*: wishful thinking, untraceable roadmap, gameable
metrics) · manual validation (structured self-use, not a scripted demo).

**Manual-validation playbook** (`research/03-audit-methodologies.md`): Phase 0 scope →
1 inspection (heuristic eval, cognitive walkthrough) → 2 discovery-risk ledger (value/
usability/feasibility/viability **+ trust**) → 3 metrics/funnel → 4 fit → 5 dogfooding.
**Phase 6 (agent-facing, load-bearing for ghx):** run a real coding-agent session end-to-end
with the tool enabled — and **drive it with the *weakest* model that should plausibly
succeed; a strong harness papers over the AX defects you want to surface**. Capture the full
tool-call transcript; record per task — binary success and **`goal_reached`** (distinct from
merely "terminated"), **human-intervention count (HIC)** as a headline number, tool-calls-made
vs. minimum-necessary, **`false_errors`** (a non-zero exit that was *not* a real failure — a
common, currently-unnamed ghx mode), whether errors let the agent self-correct, whether the
report's claims are checkable against real output or fabricated, and whether the agent needed
prose docs beyond `--help`. Tag each friction point by class — *missing_guidance / confusion /
extra_steps / awkward-but-worked / unclear_interface*. **A confidently-wrong final report is
severity-4 even if the task technically completed.** (Rubric + weak-driver method stolen, with
attribution, from `github.com/segmentstream/axprobe`.)

---

## Adversarial discipline

Run findings through the honest-critique machinery (`research/05-adversarial-redteam.md`):
frame in plain language → **key-assumptions check** → **pre-mortem** → **inversion** →
**kill-the-product** (a well-funded rival, or a frontier model that ships recon natively) →
**devil's advocacy** on the single most load-bearing claim.

- **Finding bar (Graham):** DH5/DH6 — quote the specific claim, name the specific flaw.
  Aim at the doc/metric/plan, never the person. Findings that only reach DH0–DH3 are dropped.
- **Severity = Likelihood × Impact (3×3):**

  | | Impact: Low | Impact: Medium | Impact: High (invalidates the thesis) |
  |---|---|---|---|
  | **Likelihood: Low** | Log | Track, revisit next audit | Escalate — verify before dismissing |
  | **Likelihood: Medium** | Track | Scoped fix → workstream/ADR | Escalate now |
  | **Likelihood: High** | Scoped fix | Escalate now | Stop & resolve before proceeding |

- **Disposition:** *Confirmed sound* / *Needs work* (→ workstream A/B/C or an ADR) /
  *Invalidates* (escalate, don't bury). Keep the **"What is actually fine"** section; commit
  negatives. Don't cite a milestone whose gate *failed* as evidence it works (e.g.,
  anticipation/M8 — a passing gate must exist first; the AI-PM-Pragmatist lens catches this).

---

## In scope / out of scope (the skill's fixed charter boundary)

This section is the skill's *charter* — what a product audit **ever** covers — and is distinct
from the **per-run focus / attack surface** each audit declares in its frontmatter (directive 6).
The charter is fixed; the focus narrows it further, run by run.

**In scope** — product-framed findings: north-star (mis)alignment; PMF/market/competitive/
positioning/moat/monetization; capability & JTBD gaps; adjacent-idea proposals; agent-
experience and token-economics; eval-*trustworthiness as a product concern*; strategy-to-
roadmap traceability; product-strategy critique of the north star/ADRs; **product-sequencing
YAGNI** (are we building ahead of validated need). Reading code/docs/traces **as evidence**
is in scope.

**Out of scope** — defer to the existing homes; cross-reference, don't duplicate:
- Code correctness, coupling/cohesion, complexity, architecture/boundary integrity,
  **structural YAGNI** → the code audits in `docs/audits/*.md` (architecture, complexity-
  hotspots, coupling-cohesion-testability, design-patterns, failure-class-inventory,
  adversarial-velocity) and `docs/audits/architecture-vision/*.md` (adversarial-yagni,
  domain-model, go-architecture-boundaries, resilience, runner-port), plus the code-review/
  simplify skills. (`adversarial-velocity` is the *velocity/tech-debt* red-team; this skill
  is the *product/PM* red-team — companion, not overlap.)
- Security review → the `security-review` skill.
- Implementation, refactors, ADR authoring → normal engineering flow (`AGENTS.md`).
- Eval *mechanics* (scorer code, gate math) → `docs/evals/TRUST.md`. This skill audits
  whether the evals answer the *product* question, not whether the Go is correct.

If a finding is really a code/security/eval-mechanics finding, name it and route it — don't
smuggle it into the product report.

---

## Anti-patterns to WARN against (agentic-first cargo-culting)

Flag these in the product *and* in the audit's own reasoning (details `research/06 §5`;
the point is directive 2 applied): **human-pretty output the agent doesn't read** (context
tax); **delight theater** (optimizing affect for an entity with none); **vanity human
funnels** (stars/installs/DAU, and especially **session-length / tool-calls-per-task**, which
for a context-subtracting product should go *down*); **first-run polish for the main agent**
(target is *zero* onboarding); **surveying the agent** (use ablation, not "how would it
feel"); **tool-count as progress** (more tools can *reduce* task success); **conflating
operator needs with agent value**; **assuming today's model capability is permanent** (a
CLI-based moat is a current advantage, not a Power); **trusting evals you can't audit** — and,
per R4, **the audit trusting its own unchecked judge** (directive 5).

---

## Provenance / references

Research corpus (`research/*.md`, each with a full Source Ledger — the citation of record):
`01-pm-excellence` · `02-pm-personalities` · `03-audit-methodologies` · `04-strategic-scopes`
· `05-adversarial-redteam` · `06-agentic-first-lens` · `07-oss-pm-agent-landscape` (built-tooling
recon — AX/eval rubrics stolen, prior-art negative). Round-2 critiques: `reviews/R1–R4`.
Inherited repo rules: `AGENTS.md` (Evidence Contract, Visibility/Truthfulness, Tool Economy),
`docs/NORTH_STAR.md` (the north-star filter), `CLAUDE.md` (orchestration & delegation).
