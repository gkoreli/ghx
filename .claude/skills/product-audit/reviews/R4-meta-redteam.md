# R4 — Meta Red-Team / Pre-Mortem of the `product-audit` Skill

*Adversarial review of `.claude/skills/product-audit/SKILL.md` (350 lines) and its
`research/*` corpus. Scope: the skill **as an instrument** — will an orchestrator who
follows it faithfully produce a worthless, misleading, or busywork audit? I turn the
skill's own red-team stack (`research/05`) and agentic-first anti-patterns
(`research/06 §5`) back on the skill. Read-only; no files changed. Every finding is
scored Likelihood × Impact and dispositioned; the honesty guard ("What is actually
sound") is held to the same evidence bar as the attacks.*

---

## Abstract

The `product-audit` skill is unusually good at the thing most agent-PM audits fail:
**re-pricing human PM canon for an agent consumer** (Prime Directive 2, the anti-patterns
list, the two-consumers rule). That core is sound and worth keeping. But the skill's
entire load-bearing claim — *"breadth is delegated, judgment is kept in Fable, the
orchestrator is the authoritative final judge"* — rests on a judge with **no independent
check, no calibration, and a documented structural bias toward the exact outputs it is
judging.** Fable is a Claude model adjudicating findings produced by a fleet of *"parallel
background Claude workers"* (CLAUDE.md) — the textbook self-preference regime
(Panickssery et al., arXiv:2404.13076: LLM evaluators recognize and favor their own
generations). The skill enforces a calibrated-judge discipline (κ ≥ 0.6, named verifier,
"who scored this") on the *product's* evals, then exempts *its own* judge from it. Under
the fleet's standing "delegate much more" pressure, that judge degrades from "scrutinize
each finding" to a plausibility skim over a pile of near-duplicate, finding-biased agent
outputs — a well-formatted, evidence-decorated audit that is really Fable's priors
laundered through five personas and rubber-stamped by Fable. That is the single most
dangerous failure. Six further findings follow (scope/procedure explosion, false-consensus
inflation, human owner/date ritual, missing cheapest-tool-first guard, persona-level
null-result omission, self-imposed context tax). None invalidates the skill; F1 and F2
are MUST-FIX before it is trusted on a milestone-grade decision.

---

## What is actually sound (honesty guard — equal rigor)

A red-team that cries wolf is useless. These are the parts I attacked and could **not**
break; they are why the real findings below carry weight.

**S1 — The agentic-first re-pricing is the skill's reason to exist, and it is rigorous.**
Prime Directive 2 tags every classic framework HOLDS / ADAPT / OUTDATED-OR-INVERTED, and
the "Anti-patterns to WARN against" section names the specific cargo-cults (human-pretty
output, delight theater, *"session-length / tool-calls-per-task"* as success, first-run
polish, surveying the agent, tool-count-as-progress). The **engagement/retention
inversion** — *"for a context-subtracting product, more tool-calls/time-per-task is a
vanity metric pointing the wrong way"* — is a genuine, non-obvious insight, sourced to
Anthropic context-engineering in `research/06`. Most PM-audit-of-an-agent-product would
transpose HEART and Sean-Ellis wholesale; this one has a sourced defense against exactly
that. **Attacked ("is 16 human-PM personas itself the human-PM ritual it warns against?")
and held:** the skill pre-answers it — personas describe the *builder*, not the *consumer*
(`research/06 §3 row 15`, HOLDS), so role-playing them is not the transposition error. The
residual (do 16 personas actually find *different* things?) is a real cost, captured in F3,
not a defeat of the principle.

**S2 — The evidence contract gives the judge real teeth against un-cited claims.** The
output contract — *"Recomputable by a human; a claim that cannot cite is a hypothesis, not
a finding"* — plus CLAUDE.md's *"Reject or rerun any delegated result that cannot cite
files, commands, or outputs"* is a concrete cite-or-drop gate, not a platitude. It
directly kills the weakest failure mode (an agent asserting a vibe). This is inherited
from the repo's strongest discipline and it works. (Its limit — it catches *un*-cited
claims but not *mis*-cited ones — is F1's territory, not a rebuttal of S2.)

**S3 — "What is actually fine" + commit-negatives is built in, not bolted on.** Prime
Directive 4 and the Adversarial-discipline "Honesty guard" require a "What is actually
fine" section *"with the same evidence rigor as the findings."* This is the correct
structural defense against Graham DH0–DH3 noise and the "demoralizing pile" failure
(`research/05 §8`). The house-style audits already do it (`adversarial-velocity-2026-07-07.md`
has one). Sound.

**S4 — The two-consumers separation is correct and load-bearing.** Prime Directive 3
(agent primary, operator secondary, *"never conflated"*) prevents the most common category
error in this space and is applied consistently (Tier A vs Tier B scopes). Note it also
**defuses one obvious attack**: the report is human-pretty Markdown, but its *audience is a
human* (Goga), so it is **not** the "human-pretty output the agent doesn't read"
anti-pattern — that anti-pattern is about the *product's* output to its *agent* consumer.
The skill keeps the two straight. Clean.

**S5 — Two real anti-busywork forcing functions exist.** Process step 1 (*"State the
audit's goal in one sentence and what decision it will inform"*) and the report's *"What
was not audited"* section both push against aimless auditing. Step 1 in particular is the
right idea — an audit tied to a named decision. (F5 argues it does not go far enough, but
the seed is here and it is the correct seed.)

**S6 — Redundancy is anticipated at the selection layer.** *"Known redundant pairs — pick
one …"* plus *"Span the axes, don't stack near-duplicates"* is a real dedup guard. It is at
the wrong layer to stop the deeper problem (F3), but it is not absent, and crediting it is
what makes F3 a precise finding rather than a general gripe.

These six are strong enough that I would keep the skill. The findings below are about
making its central mechanism trustworthy, not about scrapping it.

---

## Ranked failure-mode findings

### F1 — The "orchestrator is the judge" mechanism has a self-preference blind spot and no independent or calibrated check; under fleet pressure it rubber-stamps. **[MUST-FIX]**

**Quoted target.**
> "The orchestrator (Fable) is the **authoritative final judge**. Breadth is delegated to
> background persona-agents; judgment is not." (SKILL.md §opening)
> Process step 4: "Cross-validate (the judge step). For each returned finding: does the
> evidence hold? Is it DH5/DH6? … Reject, merge duplicates, re-severity. **Trust nothing on
> assertion.**"
> Prime Directive 5: "Delegate breadth wide; **keep synthesis, ranking, and the
> accept/reject call in Fable**."

**Failure mode.** The judge is (a) the *same* entity that scoped the audit and wrote the
persona prompts, (b) *uncalibrated* — there is no gold set, no independent adjudicator, no
"who verified the judge," and (c) judging outputs from **its own model family**: CLAUDE.md's
default fleet is *"parallel background **Claude** workers."* This is the exact regime the
LLM-as-judge literature flags: LLM evaluators **recognize and favor their own generations**,
with a measured linear correlation between self-recognition and self-preference strength
(Panickssery, Bowman & Feng, arXiv:2404.13076, *verified by fetch*); self-preference,
verbosity, and position biases are documented and systematic (self-preference-bias
literature, search-corroborated). "Trust nothing on assertion" is an *instruction*, not a
*mechanism* — nothing structurally forces the judge to independently re-open the evidence
for any finding, so under CLAUDE.md's standing *"delegate much more" / "under-delegation
wastes the plan"* pressure, the judge step collapses to a plausibility skim. That is
**checklist/compliance theater** in the security sense: a passing review that reflects the
reviewer's confidence at a point in time, not verified reality — *"a passing grade tells
you documentation and controls appeared sufficient to the reviewer … not whether those
controls are actually operating"* (Security Magazine, "Compliance Theater," *verified*).

**The double standard is the sharpest cut.** The north star's own tenet: *"Trust in the
measurement is a tracked workstream, never an assumed property … every claim has a named
verifier and evidence"*; the eval judge is not citable until a *"founder-labeled κ ≥ 0.6
calibration gate passes"*; CLAUDE.md's Evidence Contract demands answering *"who scored
this, from what evidence, and how do I check it myself"* unprompted. The skill imposes
calibrated, independently-verified judging on the **product's** evals — and exempts its
**own** judge (Fable) from all of it. The audit's headline findings ship to
`docs/audits/product-*.md` and inform decisions with a judge held to a *lower* standard
than the product it audits.

**Scenario (six months out).** A "should we build M7 escalation tiers?" audit spawns five
Claude persona-agents. Each returns confident, file:line-cited findings. Fable, mid-loop
with eight other workers in flight, skims them, finds them plausible (they're from its own
family, phrased the way it would phrase them), merges duplicates, ranks by severity, and
ships a polished report recommending M7. Every claim *cites* a file; none was independently
re-derived; the load-bearing interpretation ("the tool registry can't absorb codemap
cheaply") was a persona-agent's confident guess Fable pattern-matched as correct. The audit
reads authoritative and is Fable's prior laundered through five personas. Step 7's optional
meta-pass would be *Fable red-teaming Fable* — same blind spot.

**Smallest fix.** Three cheap, targeted changes to Process step 4:
1. **Break the monochrome fleet for the judge step.** CLAUDE.md already says *"optionally
   ask gpt-5.5 for an extra independent perspective"* for reviews — make it **non-optional**
   for any Escalate/High finding: at least one adjudication pass from a *different model
   family* than the fleet that generated the findings. Cross-family disagreement is the
   cheapest available proxy for calibration.
2. **Force per-finding re-derivation for High severity.** Require the orchestrator to
   *actually re-open the file / re-run the command* for every finding rated Escalate/High
   before it ships, and record "re-derived: yes" in the output contract (which already
   demands "recomputable by a human" — make the orchestrator recompute the load-bearing
   ones, not just assert they're recomputable).
3. **Answer "who judged this, and how do I check the judge" in the report**, per the repo's
   own Evidence Contract — name the judge model, the cross-family check, and the
   re-derivation, so the audit meets the standard it enforces on the product.

**Severity: Likelihood High × Impact High → Stop & resolve before proceeding.**
**Disposition: invalidates-a-part** (invalidates the skill's central value claim until
fixed). **Tag: MUST-FIX.**

---

### F2 — Restraint is capped only on personas; scopes and surfaces have no cap, and *"Tier B near-always relevant"* actively drives the boil-the-ocean explosion the skill warns against. **[MUST-FIX / strong SHOULD]**

**Quoted target.**
> "**3–5 per round** is the default; escalate to the full sixteen only for a rare … audit."
> (personas — a real cap)
> "The scopes that match the audit's goal; **agent-native (Tier B) scopes are near-always
> relevant for ghx.**" (scopes — *no* cap, and a push to run all 8)
> Process step 3: "each role-playing one persona **over the chosen scopes/surfaces**."

**Failure mode.** The numeric restraint (3–5) is applied to *one* of three axes. Scopes get
the opposite steer — 8 Tier B scopes declared *"near-always relevant"* — and surfaces (6)
get none. So the faithful orchestrator hands each of 5 persona-agents a mandate of ~8 scopes
× 6 surfaces = **~48 cells each**. A single agent asked to sweep 48 cells produces a shallow
pass, not a DH5/DH6 finding — the exact *"attempts to tackle overly ambitious problems
without clear boundaries … trying to do everything at once → inefficiency, wasted resources,
failure"* the boil-the-ocean anti-pattern names (910advisors / educba, search-corroborated,
low-authority business-idiom sources). The skill *warns against boiling the ocean in the
product* and then structurally commits it *in the audit*. This is compounded by CLAUDE.md's
systemic incentive: *"spawned wide … under-delegation wastes the plan."* The one-sentence
soft restraint loses to the operating manual's standing push.

**Sub-point — procedural sprawl.** The orchestrator must also hold **four overlapping
step-lists** with no stated authority order: Process (7 steps), Adversarial-discipline
sequence (6 moves: frame → key-assumptions → pre-mortem → inversion → kill-the-product →
devil's advocacy), the Manual-validation playbook (Phase 0–6), and by reference
`research/05`'s 9-step protocol. Run naively "per surface per persona," these multiply into
checklist theater; run selectively, the skill never says *which* is the authoritative
runsheet.

**Scenario.** A routine "AX check of the sidecar report contract" balloons: 5 personas × all
8 Tier-B scopes × 6 surfaces, each agent also told to run the 6-move adversarial sequence
and the 7-phase manual playbook. Twelve hours of fleet time later, Fable has 200 shallow
observations, 60% overlapping, and the two findings that actually mattered (report field X
is unparseable; error Y names no next step) are buried in noise. The signal is *diluted by
the machinery meant to find it*.

**Smallest fix.**
1. **Cap all three axes, not one.** Add: "Default per audit: ≤5 personas, **≤4 scopes, ≤3
   surfaces** — chosen for the named decision. The full roster on any axis is milestone-only."
2. **Narrow the per-agent mandate.** Change step 3 to assign each persona-agent **one primary
   surface + its 2–3 most relevant scopes**, not "the chosen scopes/surfaces" wholesale.
3. **Name one authoritative runsheet.** State that Process (the 7 steps) is the spine and the
   adversarial sequence / manual playbook are *tools the orchestrator reaches into per
   finding*, not lists to run exhaustively.

**Severity: Likelihood Medium-High × Impact Medium → Escalate now / scoped fix.**
**Disposition: needs-work.** **Tag: MUST-FIX** (the missing scope/surface cap) **+ SHOULD**
(runsheet authority). *(Likelihood is my least-confident call — the 3–5 persona cap and "do
not run everything" language mean a careful orchestrator may self-restrain; the claim it
*will* over-run leans on CLAUDE.md's "delegate wide" incentive beating the soft cap.)*

---

### F3 — The redundancy guard is at the selection layer; it misses run-time convergence, which manufactures false consensus and inflates severity. **[SHOULD]**

**Quoted target.**
> "**Known redundant pairs — pick one:** Feature-Factory Skeptic ↔ Craft Purist; …"
> Process step 4: "Reject, **merge duplicates**, re-severity."

**Failure mode.** The dedup guard operates on *persona pairs at selection time* (don't pick
two overlapping lenses). It does nothing about the deeper convergence: distinct,
correctly-chosen personas independently sweeping the same surfaces will **rediscover the
same top findings**. The sidecar's ~130-line English persona prose will be flagged as
doctrine-tax by the AX lens, the token-economics scope, the context-budget scope, *and* the
Simplicity Minimalist — four "independent" hits on one finding. Process step 4 merges them
*after* the fleet has already burned the tokens, so the waste is real (tolerable under a 20×
plan). The un-tolerable part: **N personas converging on one finding reads as N-fold
corroboration** and can inflate its apparent severity/likelihood. The judge (F1) is prone to
treating manufactured consensus as strong evidence — the precise confirmation-bias trap
`research/05 §7` exists to name.

**Scenario.** Four personas flag "doctrine tax." Fable, seeing four independent lenses agree,
rates it Likelihood-High and escalates it above a single-lens finding that was actually more
severe. The severity ranking — the skill's whole action-routing mechanism — is corrupted by
counting one finding four times.

**Smallest fix.** Add to Process step 4: "**Convergence is not corroboration.** N personas
flagging the same finding is *one* finding at its own merits — do not let multiplicity raise
its severity. Record convergence as a note, never as evidence weight." Optionally, partition
*primary* surfaces across personas at spawn (F2's fix #2) so fan-out covers rather than
repeats — while keeping cross-lens looks deliberate, not accidental.

**Severity: Likelihood Medium × Impact Medium → scoped fix.** **Disposition: needs-work.**
**Tag: SHOULD.**

---

### F4 — The disposition scheme cargo-cults the human-team "owned + dated" ritual and does not route findings into the repo's actual action machinery (ADRs, workstreams, milestone table). **[SHOULD]**

**Quoted target.**
> "*Needs targeted work* (**scoped, owned, dated**)"
> "**Disposition, every finding:** … *Needs targeted work* (scoped, **owned + dated**) …"

**Failure mode.** "Owned + dated" is human-team process vocabulary. This repo is a
solo-founder + agent-fleet operation whose action currency is **ADRs, the A/B/C workstreams,
and the milestone table** (NORTH_STAR.md). "Owner" resolves to "Goga or a spawned worker";
"date" is close to meaningless for a fleet that ships continuously. So the highest-value
findings land as *"someone should, sometime"* items that don't self-route into the repo's
real decision loop. Worse, this is the same category the skill warns about in its own
anti-patterns — transposing a human-org ritual onto an agent-native operation — committed in
the audit's *own* process. And for the **strategy tier** specifically, the actionable output
of a High finding is "escalate to whoever owns the plan" = "Goga decides" — honest, but it
means the elaborate machinery's marginal output over *one sharp agent arguing the strongest
case against the north star* is thin exactly where the stakes are highest (this feeds F5).

**Scenario.** A "the CLI-based moat may commoditize as base models improve" finding
(genuinely High, genuinely load-bearing) is dispositioned "needs targeted work, owned: Goga,
dated: 2026-07-21." That date passes; nothing is wired to notice; the finding dies in a
findings doc — the *"invalidates the plan … don't let it die quietly"* failure the skill
itself names, caused by the skill's own routing vocabulary.

**Smallest fix.** Replace "owned + dated" with the repo's real currency: each non-sound
finding routes to **one of {→ write/update ADR X, → workstream A/B/C sub-milestone, →
north-star-filter re-check, → decision request to Goga}**. That makes findings actionable
*in this repo* instead of in a generic PM tracker.

**Severity: Likelihood High × Impact Low-Medium → scoped fix.** **Disposition: needs-work.**
**Tag: SHOULD.**

---

### F5 — No "cheapest-tool-first" guard: the skill lists many "use it when" triggers and never says that for a focused question one sharp north-star-loaded agent beats the fan-out. In a repo allergic to busywork, this invites overkill. **[SHOULD, borderline MUST]**

**Quoted target.**
> "**Use it** when Goga (or Fable, self-directed) asks to: audit the product / find product
> gaps / pressure-test the north star … / check agent-experience (AX) … / sanity-check
> whether the roadmap is traceable to strategy."
> "**Do not use it** for: code correctness, architecture/coupling/complexity, or security …"

**Failure mode.** The "when not" clause excludes only *other domains* (code/security). It
never says: *even for an in-scope product question, the multi-persona fan-out is often the
wrong tool.* For a focused question — "is this error message a good agent affordance?",
"does M8 anticipation pass the north-star filter?" — a **single sharp agent with the north
star loaded** produces the same DH5/DH6 finding faster, cheaper, and with less orchestrator
context spent, and *without* the fan-out → dedup → judge overhead (which F1/F3 show adds
risk, not just cost). The fan-out earns its keep only when breadth × stakes are both high (a
milestone verdict, a north-star revision, a "should we build M7" call). By listing broad
triggers with no cost gate, the skill invites its own invocation on jobs one agent does
better — the analysis-paralysis / over-engineering the repo (NORTH_STAR filter; CLAUDE.md
anti-abdication) is explicitly allergic to.

**Scenario.** Fable, self-directed mid-loop, hits "is the report contract a clean AX
surface?" and reaches for `product-audit` because the "use it" list says so. It spawns five
personas for a question one AX-lens agent answers in one turn. The audit is *correct* and
*wasteful* — the busywork the repo forbids.

**Smallest fix.** Add to "When to use / when not":
> "**Default to the cheapest tool.** For a focused product question, ask a single
> north-star-loaded PM agent (AX lens for agent surfaces). Invoke the full persona fan-out
> **only** when the question is genuinely broad *and* a decision of milestone weight rides on
> it. Fan-out is for breadth-under-uncertainty, not for questions one lens can answer."

**Severity: Likelihood Medium × Impact Medium → scoped fix.** **Disposition: needs-work.**
**Tag: SHOULD** (borderline MUST given the repo's stated busywork allergy).

---

### F6 — Persona-agents are not required to report null/negative results, so the *raw* fleet output is finding-biased; the "What is actually fine" section corrects for it only at the orchestrator layer — where the orchestrator is also the party that wanted findings. **[SHOULD / OPTIONAL]**

**Quoted target.**
> Prime Directive 4: "Every audit **report** carries a 'What is actually fine' section …
> Commit negative and null results."
> Process step 3: "each **writing a findings artifact**."

**Failure mode.** The commit-negatives discipline is applied at the *report* (orchestrator)
level, not the *persona-agent* level. Each agent is dispatched to produce "a findings
artifact" — an agent that returns "I looked hard and it's fine" reads as lazy/failed, so the
unit of delegation quietly rewards flagging. This is a mild Goodhart: the fleet's raw output
skews toward findings, and the orchestrator (who scoped the audit *wanting* findings) is the
one meant to subtract the noise — a confirmation-bias loop (`research/05 §7`, survivorship:
"citing successful runs without looking at the runs where the agent silently ignored the
tool"). The report-level "What is actually fine" is a real guard (S3) but it runs *after* the
bias has shaped the input.

**Smallest fix.** Push commit-negatives down to the persona-agent output contract: "Each
persona-agent must return at least one *'attacked X, it held'* result; 'we tried to break
this and couldn't' is a required deliverable, not a failed spawn."

**Severity: Likelihood Medium × Impact Low-Medium → track / scoped fix.** **Disposition:
needs-work.** **Tag: SHOULD / OPTIONAL.**

---

### F7 — The skill levies a standing context tax on the premium orchestrator whose context the audited north star calls "sacred." Ironic, real, low-impact. **[OPTIONAL]**

**Quoted target.**
> The skill: 350-line SKILL.md + a `research/*` corpus (~360 KB across six files) that the
> orchestrator "reaches for" when a lens needs primary sources.

**Failure mode.** To *run* this skill, Fable — the premium model whose *"context is
sacred"* per the very north star the skill enforces — loads 350 lines of dense doctrine
every invocation. A product-audit skill for a context-subtraction product itself fails the
north-star filter it applies to everything else. **Honest mitigation (why this is
OPTIONAL, not MUST):** progressive disclosure is already respected — the 360 KB research
corpus is *deferred* ("reach for the artifacts when a lens needs its primary sources"), so
only the 350-line operational layer is standing tax, and that is close to the operational
minimum for a skill this rich. The irony is real; the impact is small.

**Smallest fix (optional).** Extract a ~1-page "runnable core" (the 7-step Process + the
severity/disposition scheme + the axis caps from F2) as the always-loaded layer; defer the
persona table, scope tables, and anti-patterns to reach-for sections. This is a nice-to-have.

**Severity: Likelihood High × Impact Low → log.** **Disposition: needs-work (minor).**
**Tag: OPTIONAL.**

---

## When NOT to invoke this skill

Distilled from F1/F2/F5, stated plainly so an orchestrator has a real gate:

- **A focused, single-lens question** ("is this error message a good affordance?", "does
  feature X pass the north-star filter?"). One north-star-loaded PM/AX agent is faster,
  cheaper, and lower-risk. The fan-out adds dedup + judge overhead that *adds* failure modes
  (F1, F3) without adding coverage a single lens lacks.
- **When no decision rides on it.** Process step 1 demands a named decision the audit
  informs — if you can't name one that a finding could actually *change*, don't run it.
- **When the fleet that would generate findings and the judge are the same model family and
  you can't spare a cross-family adjudication pass** — you'll get a self-preference echo
  chamber (F1). Either fix the judge step first or don't trust the output on a real decision.
- **Code correctness / architecture / security / eval-mechanics** — already excluded by the
  skill; named here for completeness.

Positively: the skill *earns its cost* when breadth **and** stakes are both high — a
milestone go/no-go (build M7? revise the north star?), a competitive/PMF pressure-test, or a
first-principles "does this product deserve to exist" pass — where a single lens has real
blind spots and the fleet spend is justified by the weight of the decision.

---

## Source Ledger

| # | Source | Type | Verification | Supports |
|---|---|---|---|---|
| 1 | [Panickssery, Bowman & Feng, "LLM Evaluators Recognize and Favor Their Own Generations," arXiv:2404.13076](https://arxiv.org/abs/2404.13076) | Primary (peer-reviewed, NeurIPS 2024) | **Verified by direct fetch** — title/authors/finding confirmed; quoted: *"a linear correlation between self-recognition capability and the strength of self-preference bias."* | F1 (self-preference judge over own-family output) |
| 2 | [Security Magazine, "Compliance Theater: Why Cybersecurity's Favorite Shakespearean Tragedy is Failing Us"](https://www.securitymagazine.com/blogs/14-security-blog/post/102062-compliance-theater-why-cybersecuritys-favorite-shakespearean-tragedy-is-failing-us) | Secondary (practitioner) | **Verified by direct fetch** — quoted: *"Passing an audit in January tells you nothing meaningful about your security in March."* | F1 (rubber-stamp / checklist theater framing) |
| 3 | [Wu et al., "Self-Preference Bias in LLM-as-a-Judge," OpenReview Ns8zGZ0lmM](https://openreview.net/forum?id=Ns8zGZ0lmM) | Primary | **Existence confirmed via search; full text unverified** (page served a bot-verification wall on fetch) | F1 (corroboration of self-preference/verbosity/position bias) |
| 4 | [910advisors, "Boil the Ocean Consulting"](https://910advisors.com/2025/04/25/boil-the-ocean-consulting-the-perils-of-over-ambition-in-projects/) · [EducBA, "Boil the Ocean"](https://www.educba.com/boil-the-ocean/) | Secondary (business idiom, low authority) | **Search-corroborated; not deep-fetched** — used illustratively for a well-known anti-pattern, not as load-bearing evidence | F2 (scope/procedure explosion) |
| 5 | `research/05-adversarial-redteam.md` (Klein pre-mortem; UFMCS red-team; Graham "How to Disagree" DH0–DH6; Nickerson confirmation bias; NASA L×I matrix) | The skill's own cited corpus | Used **against the skill** per task instruction ("use its own methods against it"); underlying URLs verified in that artifact's own ledger | F1, F3, F6 (judge bias, false consensus, finding-bias); the DH5/DH6 + L×I discipline this review itself follows |
| 6 | `research/06-agentic-first-lens.md §5` (anti-patterns: vanity funnels, human ritual transposition, trusting un-auditable evals) | The skill's own cited corpus | Turned back on the skill's *own* process | F4 (owner/date ritual), F1 (un-auditable judge) |
| 7 | `docs/NORTH_STAR.md` (context-sacred tenet; "trust in the measurement is a tracked workstream"; κ ≥ 0.6 calibrated-judge gate; north-star filter) · `CLAUDE.md` (Evidence Contract "who scored this"; "parallel background Claude workers"; "delegate much more") | Repo canon | Read directly this session | F1 (double standard), F2 (delegate-wide incentive), F7 (context tax) |

**External sources: 4 distinct** (2 verified by fetch, 1 existence-only, 1 low-authority
illustrative). Plus 3 in-repo/own-corpus sources. Light research by design, per task scope.
