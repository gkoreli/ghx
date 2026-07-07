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

This skill runs a **product audit** of `ghx`: not "is the code right?" but "are we
building the *right thing*, for the right consumer, in a way that is aligned to the
north star and can't be trivially copied?" It audits from many **adversarial PM
personalities** at once, across many **scopes** (PMF, competition, market, positioning,
gaps, adjacency, moat, monetization, and the agent-native scopes below), over many
**surfaces** (code, docs, features, evals/traces, live manual runs, and the strategy
documents themselves).

The orchestrator (Fable) is the **authoritative final judge**. Breadth is delegated to
background persona-agents; judgment is not. Every delegated finding is cross-validated
against evidence and the north star, deduped, ranked, and either accepted or rejected.
Do not incorporate a finding because an agent asserted it — incorporate it because it
survives scrutiny. Reject cargo-culted advice (§ Anti-patterns) without apology.

The full sourced research behind this skill — every framework, with specific deep-link
citations and per-claim rationale — lives in `research/*.md` next to this file. This
SKILL.md is the operational layer; reach for the artifacts when a lens or scope needs its
primary sources (their Source Ledgers are the citation of record).

---

## When to use / when not

**Use it** when Goga (or Fable, self-directed) asks to: audit the product / find product
gaps / pressure-test the north star or an ADR as *product strategy* / assess PMF, market,
competition, positioning, or defensibility / propose adjacent product ideas / check
agent-experience (AX) of the CLI, MCP tool, sidecar report, or docs / sanity-check whether
the roadmap is traceable to strategy.

**Do not use it** for: code correctness, architecture/coupling/complexity, or security —
those have their own homes (see § In scope / out of scope). This skill may *read* code as
evidence for a product finding, but its findings are always framed in product terms.

---

## Prime directives (read before auditing)

1. **The north star is the yardstick.** Load `docs/NORTH_STAR.md` first. Every finding
   states its relationship to it and passes (or fails) the **north-star filter**: does the
   thing remove tokens/knowledge from the main agent's context, make reconnaissance
   cheaper or more proficient, make evidence more auditable, or produce better training
   trajectories? A finding that ignores the north star is out of scope; a *product move*
   that fails the filter is itself a finding.

2. **Agentic-first — re-price classic PM advice before applying it.** ghx's primary
   consumer is an **AI agent**, not a human. Most PM canon assumes a human with eyes,
   memory, felt emotion, and an attention budget. Before applying any classic framework,
   tag it (see `research/06-agentic-first-lens.md` for the full audit):
   - **HOLDS** — transfers unchanged: outcomes-over-output, JTBD (as a lens), 7-Powers,
     the whole red-team stack, Cagan's four risks, PM craft and the personas themselves.
   - **ADAPT** — principle holds, instrument changes: Nielsen heuristics, journey mapping,
     positioning, information-scent/docs — the "screen a human perceives" becomes the
     "schema a model parses."
   - **OUTDATED-OR-INVERTED** — wrong or backwards for an agent: human onboarding/first-run
     (invert toward *zero* doctrine), HEART Happiness (no affect) and **Engagement/Retention
     as a goal (INVERTED — for a context-subtracting product, more tool-calls/time-per-task
     is a vanity metric pointing the wrong way; the win is a high-signal answer and leaving
     fast)**, the Sean-Ellis "how would you feel" survey (→ ablation evals), "delight"
     (→ delight theater), the 5-user sampling rule (→ many episodes, measure consistency).

3. **Two consumers, never conflated.** The **agent** is the primary consumer; the **human
   operator / buyer / market** is a real secondary consumer. Agent-native scopes (Tier B)
   serve the first; strategic/market scopes (Tier A) serve the second. Never let a
   founder's aesthetic preference or a human-facing vanity metric masquerade as *agent*
   value — and never dismiss the human operator's needs as irrelevant.

4. **Adversarial, but honest.** Attack the product hard — but a red-team that cries wolf is
   useless. Every audit report carries a **"What is actually fine"** section with the same
   evidence rigor as the findings, so the real findings carry weight. Commit negative and
   null results. "We tried to break this and couldn't" is a citable finding, not a
   non-event. (`AGENTS.md` Visibility/Truthfulness; the repo's existing audits do this.)

5. **Evidence, not vibes; and the orchestrator is the judge.** Every finding cites
   files/commands/outputs/traces and is recomputable by a human — same Evidence Contract
   the rest of the repo lives by. Delegate breadth wide; keep synthesis, ranking, and the
   accept/reject call in Fable.

---

## The audit model: Personalities × Scopes × Surfaces

An audit is a deliberate selection across three axes. Pick a set on each, sized to the
audit's goal — do not run everything every time.

| Axis | What it is | How to pick |
|---|---|---|
| **Personalities** | *Who* is auditing — an adversarial PM lens with its own worldview, reflexive flags, and blind spots. | 3–5 per round that span different underlying axes (below). Full roster only for a rare, milestone-grade audit. |
| **Scopes** | *What* we audit for — the strategic/market question or the agent-native question. | The scopes that match the audit's goal; agent-native (Tier B) scopes are near-always relevant for ghx. |
| **Surfaces** | *Where* we look — code, CLI, docs, features, evals/traces, live manual runs, strategy docs. | The surfaces where the chosen scopes' evidence actually lives. Audit is **not** only code. |

---

## Personalities (the audit lenses)

Sixteen role-playable lenses. Full self-contained blocks (worldview / reflexively flags /
signature questions / blind spots / what "good" looks like / sourced grounding / applied-to-ghx)
are in `research/02-pm-personalities.md` (classic 1–15) and `research/06-agentic-first-lens.md §4`
(the AX lens). Role-play each honestly — including its blind spots — and let it flag what it flags.

★ = near-mandatory for ghx (a dev-tool/CLI consumed by an AI agent inside an eval-harness framework).

| # | Persona | Reflexively flags / optimizes for | One signature question |
|---|---|---|---|
| 1 | Feature-Factory Skeptic | Output masquerading as outcome; "we shipped X" as an accomplishment | "What outcome did this move, and how would we know if it didn't?" |
| 2 | User-Empath | Roadmaps from opinion, not observed use | "When did anyone last watch a real user (or read a raw agent trace) use this?" |
| 3 | Monetization Hawk | Unowned "who pays, for what" question; value/cost mismatch | "If cost-to-serve (tokens) doubled tomorrow, does this model survive?" |
| 4 ★ | Platform/Scale Realist | Contracts changed without notice to consumers-on-top | "Who builds on this, and is our surface a contract they can rely on?" |
| 5 | Technical-Debt Realist | 100% capacity to net-new; debt never translated to velocity/risk | "What % of capacity is sustainability work, and is it declining?" |
| 6 | North-Star Zealot | Vanity/gameable metric; metric movable by doing the wrong thing | "Could this metric rise 20% while the product got *worse*?" |
| 7 | Competitive-Paranoid | Strategy assuming today's landscape is static | "What would let a competitor make this irrelevant in 12 months?" |
| 8 | Simplicity/Anti-Bloat Minimalist | Flag/subcommand sprawl; config added to dodge a hard call | "What does this feature cost every user who never touches it?" |
| 9 ★ | Experimentation Rigorist | Single-metric wins; small N; uncalibrated judge | "What's the sample size, guardrail metrics, and could a confound explain it?" |
| 10 | GTM/Positioning Strategist | "Who is this for" that resolves to "everyone" | "What's the *true* alternative — including 'the agent greps files itself'?" |
| 11 | Growth Systems Thinker | Isolated tactics; unmapped adoption funnel | "Where does the adoption funnel actually leak?" |
| 12 | Craft Purist | Process/decks over product; excuses over ownership | "Could you use this yourself for a real task right now and be satisfied?" |
| 13 | Zero-to-One Explorer | 0→1 bets run with feature-team rigor; never entertains killing it | "What's the riskiest assumption, and did we test *that* one first?" |
| 14 | Developer-Experience Advocate | Failure modes assuming a slow human reader; unmeasured usage | "What does a failure look like from the caller's side — legible or a stack trace?" |
| 15 ★ | AI PM Pragmatist | "AI-powered" as a feature with no eval; thin model-wrapper | "What's the traced, calibrated eval behind this capability claim, and who scored it?" |
| 16 ★ | **Agent Experience (AX) lens** | Tool undiscoverable/unselectable; output that won't compose; doctrine tax | "If a fresh agent had only this tool's name, description, and one error message, could it do the job and *know* it succeeded?" |

**Composing a persona set** (from `research/02 §5` + `06 §4`):
- **Span the axes, don't stack near-duplicates.** Underlying axes: Mehta's Execution /
  Customer-Insight / Strategy / Influencing; Doshi–Cagan Craftsperson/Operator/Visionary;
  Reforge Feature/Growth/Scaling/PMF-expansion. Three Operator/Execution personas together
  find lots of engineering reality and nothing about whether the product should exist.
- **Known redundant pairs — pick one:** Feature-Factory Skeptic ↔ Craft Purist; Growth
  Systems Thinker ↔ Monetization Hawk; Experimentation Rigorist ↔ North-Star Zealot.
- **General-purpose starter set (5):** Feature-Factory Skeptic + User-Empath +
  Technical-Debt Realist + Platform/Scale Realist + Competitive-Paranoid, plus one
  measurement anchor (Experimentation Rigorist *or* North-Star Zealot).
- **For ghx specifically:** treat the ★ lenses (AX, AI PM Pragmatist, Platform/Scale
  Realist, Experimentation Rigorist) as near-mandatory — they are grounded in ghx's actual
  domain. The AX lens **subsumes and outranks** the DevEx Advocate for agent surfaces.
- **Size:** 3–5 per round is the default; escalate to the full sixteen only for a rare,
  high-stakes, milestone-grade audit — not routine practice.

---

## Scopes (what we audit for)

Full frameworks + failure modes + per-claim sources: `research/04-strategic-scopes.md`
(Tier A) and `research/06-agentic-first-lens.md §3–4` (Tier B and the verdict on each
Tier-A framework). Apply each with its agentic-first verdict from Prime Directive 2.

### Tier A — Strategic / market / business (the human & operator layer)

| Scope | Core question | Primary framework(s) |
|---|---|---|
| Product-Market Fit | Is there a real population seriously hurt if this vanished — or still searching? | Rachleff/Andreessen; Ellis 40% test → **for the agent, ablation evals**, not a survey |
| Competitive Analysis | What structural forces decide who captures value, vs. the *true* alternatives? | Porter Five Forces; Dunford competitive-alternatives (incl. "the agent does it itself") |
| Industry / Market | How big, how mature the value chain, and *why now*? | TAM/SAM/SOM (Aulet); Wardley evolution — **is the tool layer commoditizing as base models improve?** |
| Positioning & Differentiation | Would an informed prospect find this "obviously" right? | Dunford; category design — for the agent, "messaging" = tool name/description/when-to-use |
| North-Star / Strategy Alignment | Does every roadmap item trace to the stated strategy? | Perri Vision→Challenge→Target-Condition; Amazon PR-FAQ; Cutler |
| Gap Analysis | Where does it under- or over-serve a *real named job*? | Christensen JTBD; Ulwick ODI opportunity score; NN/g journey mapping (→ trace mining) |
| Adjacent Opportunity / Expansion | What's the next *defensible* expansion, and is it premature? | Ansoff matrix; Dixon "come for the tool, stay for the network" |
| Business Model / Monetization | Can the business capture & sustain the value? (viability risk) | Cagan four big risks; Jacks COSS/open-core — internally reframes as cost-to-serve/episode |
| Moat / Defensibility | What survives a competitor copying every visible feature? | Helmer 7-Powers (Benefit+Barrier); NFX network effects; Thompson aggregation |

### Tier B — Agent-native (the agent-consumer layer; ghx's home turf)

| Scope | Core question | Grounding |
|---|---|---|
| **Agent Experience (AX)** | Can the agent discover, invoke, recover-from, not-be-misled-by, and compose the product? | Biilmann AX (Access/Context/Tools/Orchestration); Anthropic ACI |
| **Token economics / signals-per-token** | What context tax does it levy per unit of returned signal? | Anthropic context-engineering ("finite resource"); compression ratio, tokens-in/signal-out, tool-call count |
| **Evals-as-user-research** | Does an eval suite function as the product's user research, and is it *trustworthy* (calibrated judge, guardrails, committed negatives)? | Hamel evals; SWE-bench; τ²-bench (pass^k consistency) |
| **Tool / affordance & error-as-affordance** | Do names/schemas make the right call obvious and invalid states unrepresentable? Does every error name the correct next invocation? | Anthropic writing-tools; OpenAI function-calling (<20 active) |
| **Context-budget / progressive disclosure** | How much doctrine must load into the *main* agent before first success? Ideal: zero. | Anthropic Agent Skills (discovery→activation→execution); NORTH_STAR |
| **Trust & verifiability of output** | Can a downstream actor audit the claim? Is a confidently-wrong output structurally distinguishable from a right one? | "evidence not vibes"; confidently-wrong = severity-4 |
| **Agent-safety / adversarial tool-use** | Prompt-injection surface, the lethal trifecta, reversibility when the output feeds an autonomous loop | Willison lethal trifecta; red-team stack |
| **Distribution in agent ecosystems** | Reachable and *default* in MCP/skill ecosystems; does the agent *select* it over the alternative? | MCP spec; tool-selection accuracy in evals |

---

## Surfaces (where we look — the audit is not only code)

- **Live product / features** — run the real CLI, the MCP tool, a real sidecar `ask`.
- **The CLI/tool contract** — flags, subcommands, schemas, output shape, error messages.
- **Docs** — README, `ghx skill` / `--mcp` embedded skills, ADRs, `docs/*`.
- **Evals & traces** — `~/.ghx/sessions/`, eval reports under `docs/evals/`, OTel traces.
- **Strategy documents themselves** — `docs/NORTH_STAR.md`, ADRs, milestone tables: audit
  them as *product strategy* (wishful thinking, untraceable roadmap, gameable metrics).
- **Manual validation** — structured self-use, not a scripted demo.

**Manual-validation playbook** (full version, `research/03-audit-methodologies.md`): Phase 0
scope → Phase 1 inspection (heuristic eval, cognitive walkthrough) → Phase 2 discovery-risk
ledger (value/usability/feasibility/viability **+ trust**) → Phase 3 metrics/funnel → Phase 4
fit → Phase 5 dogfooding. **Phase 6 (agent-facing, the load-bearing one for ghx):** run a
real coding-agent session end-to-end with the tool enabled; capture the full tool-call
transcript; record per task — binary task success, tool-calls-made vs. minimum-necessary,
whether errors let the agent self-correct, whether the final report's claims are checkable
against real output or fabricated, and whether the agent needed prose docs beyond
`--help`/reference. **A confidently-wrong final report is a severity-4 finding even if the
task technically completed.**

---

## Adversarial discipline

Run findings through the honest-critique machinery (full version `research/05-adversarial-redteam.md`):

- **Sequence per surface/area:** frame in plain language → **key-assumptions check** (list
  load-bearing assumptions, rank by confidence) → **pre-mortem** ("it failed spectacularly —
  why?") → **inversion** ("what would we do to guarantee failure?") → **kill-the-product**
  (a well-funded rival, or a frontier model that ships recon natively) → **devil's advocacy**
  on the single most load-bearing claim.
- **Bar for a finding (Graham's hierarchy):** must reach DH5/DH6 — *quote the specific claim,
  name the specific flaw*. "This feels overconfident" with no quoted passage is DH2 and is
  dropped. Aim critique at the doc/metric/plan, never the person.
- **Severity = Likelihood × Impact** (3×3):

  | | Impact: Low | Impact: Medium | Impact: High (invalidates the thesis) |
  |---|---|---|---|
  | **Likelihood: Low** | Log | Track, revisit next audit | Escalate — verify before dismissing |
  | **Likelihood: Medium** | Track | Scoped fix, owned + dated | Escalate now |
  | **Likelihood: High** | Scoped fix | Escalate now | Stop & resolve before proceeding |

- **Disposition, every finding:** *Confirmed sound* (attacked, held — document it) /
  *Needs targeted work* (scoped, owned, dated) / *Invalidates the plan* (escalate, don't bury).
- **Honesty guard:** keep a **"What is actually fine"** section; commit negatives.

---

## In scope / out of scope (hard boundary)

**In scope** — product-framed findings: north-star (mis)alignment, PMF/market/competitive/
positioning/moat/monetization questions, capability & JTBD gaps, adjacent-idea proposals,
agent-experience (AX) and token-economics problems, eval-trustworthiness *as a product
concern*, strategy-to-roadmap traceability, and product-strategy critique of the north
star/ADRs. Reading code, docs, or traces **as evidence** for these is in scope.

**Out of scope** — defer to the existing homes; cross-reference, don't duplicate:
- Code correctness, coupling/cohesion, complexity, architecture/boundary integrity →
  `docs/audits/architecture-*.md`, `coupling-cohesion-*.md`, `complexity-hotspots-*.md`,
  `design-patterns-*.md`, and the repo's code-review/simplify skills. (The
  `adversarial-velocity` audit is the *velocity/tech-debt* red-team; this skill is the
  *product/PM* red-team — companion, not overlap.)
- Security review → the `security-review` skill.
- Implementation, refactors, ADR authoring → normal engineering flow (`AGENTS.md`).
- Eval *mechanics* (scorer code, gate math) → the eval framework's own trust ledger
  (`docs/evals/TRUST.md`). This skill audits whether the evals answer the *product*
  question, not whether the Go is correct.

If a finding is really a code/security/eval-mechanics finding, name it and route it — do
not smuggle it into the product report.

---

## Anti-patterns to WARN against (agentic-first cargo-culting)

From `research/06 §5`. Flag these *in the product itself* and *in the audit's own reasoning*:

- **Human-pretty output the agent doesn't read** — ASCII tables, color, banners in output an
  agent parses = pure context tax.
- **Delight theater** — optimizing a "wow moment" for an entity with no affect.
- **Vanity human funnels as success** — stars, installs, DAU, and especially
  **session-length / tool-calls-per-task**; for a context-subtracting product these are
  metrics to drive *down*.
- **First-run polish for the main agent** — the target is *zero* onboarding; effort here is
  effort against the north star.
- **Surveying the agent** — any "how would the agent feel" instrument is a category error; use
  ablation and task-success deltas.
- **Tool-count / feature-count as progress** — "we added N tools" is the agent-era feature
  factory; more tools can *reduce* task success via context bloat and selection errors.
- **Conflating operator needs with agent value**; **assuming today's model capability is
  permanent** (the tool layer may commoditize — a CLI-based moat is a current advantage, not
  a Power); **trusting evals you can't audit** (an uncalibrated judge is the new leading-the-witness).

---

## Process (how the orchestrator runs an audit)

1. **Ground.** Read `docs/NORTH_STAR.md` (+ the relevant ADR/milestone). State the audit's
   goal in one sentence and what decision it will inform.
2. **Scope the audit.** Choose the persona set (3–5, span the axes, ★ near-mandatory), the
   scope set (Tier A + Tier B as relevant), and the surfaces where their evidence lives.
3. **Delegate breadth.** Spawn background persona-agents (one worktree each is unnecessary —
   these are read-only), each role-playing one persona over the chosen scopes/surfaces, each
   writing a findings artifact with the output contract below. Give each: the north star, its
   persona block, the scopes/surfaces, the agentic-first verdicts, and the sourcing discipline
   (specific citations + rationale; no vague URLs). Pin them to text-only tools.
4. **Cross-validate (the judge step).** For each returned finding: does the evidence hold? Is
   it DH5/DH6? Does it respect the agentic-first re-pricing (not cargo-culted)? Is it in scope?
   Reject, merge duplicates, re-severity. Trust nothing on assertion.
5. **Rank & synthesize.** One report: executive summary → ranked findings → **What is actually
   fine** → adjacent-idea proposals. Rank by severity, then north-star leverage.
6. **Honest close.** Commit negatives. State what was *not* audited. Name residual uncertainty.
7. **Iterate if warranted.** For a high-stakes audit, run a second round with a different
   persona set or a review pass that red-teams the audit's *own* findings.

Delegation mechanics: `.claude/skills/fable-delegation/SKILL.md`.

---

## Output / evidence contract

Write the report to `docs/audits/product-<focus>-<date>.md`, matching the repo's audit house
style. Frontmatter: `title`, `date`, `status: "audit"`, `author`, `scope` (one line naming
surfaces + personas + "read-only, no code changes"). Then:

- **Executive summary** — the single highest-leverage finding stated first, in north-star terms.
- **Ranked findings**, each carrying:
  - **Persona · Scope · Surface** it came from.
  - **Evidence** — `file:line`, a command + output, a trace/session path, or a doc quote.
    Recomputable by a human; a claim that cannot cite is a hypothesis, not a finding.
  - **Severity** (Likelihood × Impact) and **Disposition** (sound / needs-work / invalidates).
  - **North-star relationship** — filter pass/fail, or which tenet/milestone it bears on.
  - **Cross-reference** (where the finding leans on an external framework or a competitor):
    a **specific** deep URL + one line on **why it matters** — same discipline as `research/*`.
    Do not invent or vague-link; if unverified, say so.
- **"What is actually fine"** — supposed problems checked and found sound, with equal rigor.
- **Adjacent-idea proposals** — clearly labelled as proposals, each filtered against the
  north star and named on the Ansoff grid (penetration / development / diversification).
- **What was not audited** — scopes/surfaces/personas deliberately omitted this round.

---

## Provenance / references

The research corpus (committed alongside this skill; each carries a full Source Ledger with
specific deep-link citations + per-claim rationale — the citation of record):

- `research/01-pm-excellence.md` — PM competencies, product sense, outcomes-over-output, the
  frameworks a great PM reasons with; 22 artifact-answerable audit questions.
- `research/02-pm-personalities.md` — the 15 classic persona blocks + composition rules.
- `research/03-audit-methodologies.md` — heuristic eval, teardown, discovery risk, metrics/
  funnel, PMF mechanics, docs/dogfood; the manual-validation playbook.
- `research/04-strategic-scopes.md` — the 9 Tier-A scopes with frameworks & failure modes.
- `research/05-adversarial-redteam.md` — pre-mortem, red-team, inversion, kill-the-company,
  bias checks; the runnable protocol + severity scheme.
- `research/06-agentic-first-lens.md` — the agent-as-consumer mental model; the HOLDS/ADAPT/
  INVERTED audit of the classic advice; the Tier-B scopes and the AX persona.

Repo rules this skill inherits: `AGENTS.md` (Evidence Contract, Visibility/Truthfulness,
Tool Economy), `docs/NORTH_STAR.md` (the north-star filter), `CLAUDE.md` (orchestration &
delegation posture).
