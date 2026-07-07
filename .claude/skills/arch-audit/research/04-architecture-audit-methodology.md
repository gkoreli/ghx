---
title: "Research: Architecture-Audit & Adversarial-Review Methodology Canon"
date: "2026-07-07"
status: "research-provenance"
skill: "architecture-audit"
round: "R4"
angle: "architecture-evaluation methods + multi-persona/adversarial review + artifact conventions"
author: "skill-forge research worker (Opus 4.8)"
note: >-
  Provenance for the architecture-audit skill. NOT the skill itself. Every
  substantive claim carries a specific deep URL + rationale; all links were
  verified with WebFetch (degradations disclosed inline and in the Source
  Ledger). Synthesis (my extrapolation, not in any single source) is LABELLED.
  Text-only research (WebSearch/WebFetch), no browser, per AGENTS.md Tool Economy.
---

# Architecture-Audit & Adversarial-Review Methodology Canon

## Executive summary

The architecture-audit skill is not an improvisation dressed as method — almost
every move it already makes has a named ancestor in 30 years of software-
architecture-evaluation practice, and the few places it *diverges* are exactly
where the biggest upgrades sit. The load-bearing findings:

1. **Scenario/quality-attribute evaluation (SAAM → ATAM, SEI, 1994–2000)** is the
   canonical spine of architecture review. ATAM's output taxonomy — **risks,
   non-risks, sensitivity points, tradeoff points**, synthesized into **risk
   themes each tied to a business driver** — is a near-exact ancestor of our
   severity tiers + mandatory "what's already right" (= ATAM *non-risks*) + "tie
   every finding to a named tenet" (= ATAM *risk themes → business drivers*). We
   already do ATAM's *reporting* discipline; we do **not** do its *front-end*
   (a prioritized quality-attribute **utility tree** that scopes what to look
   at). That's the single sharpest upgrade available.

2. **Multi-view models (Kruchten 4+1, 1995; C4, Simon Brown)** are the principled
   reason our audit fans out across *lenses/scopes* rather than one reviewer:
   different stakeholders need different concurrent views, and no single view is
   the architecture. This grounds "pick persona×scope lenses" — but the canon
   suggests enumerating coverage against explicit views/zoom-levels so a view is
   never left unaudited.

3. **Contrarian analysis is a discipline, not a vibe.** The CIA's *Tradecraft
   Primer* codifies **Devil's Advocacy** and **Team A/Team B** as standing
   techniques to break consensus/groupthink; Klein's **premortem** (HBR 2007)
   uses *prospective hindsight* ("assume we already failed") to raise risk-finding
   ~30% and to *legitimize dissent*; Paul Graham's **disagreement hierarchy**
   (DH0–DH6) defines what a *good* critique is — DH5 (quote-and-refute) and DH6
   (refute the central point). Together they justify our mandatory adversarial
   personas and set a *quality bar* for what an adversarial finding must contain.

4. **The agentic-first twist is real and sourced.** Multi-agent debate (Du et
   al., 2023) shows independent LLM instances that propose-and-debate improve
   factuality and reasoning — corroboration for fan-out-then-converge. But the LLM-
   as-judge **self-preference bias** result (Panickssery et al., NeurIPS 2024) —
   self-*recognition* causally drives self-*preference* — is the hard warning
   behind our two most important judge guardrails: **discount shared-prior
   convergence** and **get a cross-model-family adjudicator**. When the same model
   authors the audits *and* judges them, it is structurally biased toward its own
   findings.

5. **What separates action from shelfware** is executable, owned, living output.
   **Architecture fitness functions** (Ford/Parsons/Kua) are the canonical anti-
   shelfware move: encode an architectural invariant as an *automated, objective
   check that runs every build*. **ADR/MADR/Oxide-RFD** make decisions *living*
   (numbered, immutable-but-supersedable, state-tracked). **Google design docs**
   make *alternatives-considered + non-goals* first-class. Our audit stops at a
   one-shot metrics snapshot + a ranked sequence; the upgrade is to promote
   recurring invariants (import-direction, god-file thresholds) into committed
   fitness functions so the audit *enforces itself* between runs.

Sources: **14 primary/canonical**, all link-verified (one via academic mirror
after the canonical host 403'd automated fetch). Full ledger at the end.

---

## The load-bearing methodology (each item: canon → source → how to apply to a ghx multi-agent audit)

### 1. How to pick persona × scope lenses

**Canon.** No single view *is* the architecture; different stakeholders need
different, concurrent views. Kruchten's **4+1** defines five: *logical, process,
development, physical, + scenarios* (scenarios validate and unify the other four)
— "different stakeholders … require distinct architectural perspectives."
[cs.ubc.ca/~gregor/teaching/papers/4+1view-architecture.pdf](https://www.cs.ubc.ca/~gregor/teaching/papers/4+1view-architecture.pdf)
— *rationale: the original 1995 IEEE-Software paper; establishes multi-view as the
discipline behind lens-based fan-out.* The **C4 model** reframes this as *zoom
levels* — system context → container → component → code — where "each abstraction
level tells a distinct story suited to particular audiences."
[c4model.com/abstractions](https://c4model.com/abstractions)
— *rationale: the canonical modern statement that scope = a deliberate zoom level,
not "everything at once."* ATAM formalizes stakeholder diversity as a *method
requirement*: "diverse participants — architects, project managers, end-users —
collectively develop scenarios," because analysis must address genuine concerns
"rather than theoretical possibilities."
[sei.cmu.edu/documents/629/2000_005_001_13706.pdf](https://www.sei.cmu.edu/documents/629/2000_005_001_13706.pdf)
— *rationale: primary ATAM technical report; diversity-of-lens is load-bearing,
not decorative.* The agentic corroboration: independent LLM instances that each
reason separately and then reconcile beat a single instance on factuality/reasoning.
[arxiv.org/abs/2305.14325](https://arxiv.org/abs/2305.14325)
— *rationale: Du et al. multi-agent debate; the mechanism (diverse independent
reasoners reduce correlated error) is why fan-out > one reviewer.*

**How to apply to a ghx audit.** Keep the persona catalog, but *cover a view
grid*, not an ad-hoc list: map each scope to a zoom level (C4 container = a module
like `internal/sidecar`; component = the ACP turn engine) and to a lens
(4+1-style: *development* = build/dependency direction, *process* = concurrency/
resilience, *logical* = domain modeling). Then check no view is unaudited before
spawning. Our "Go architecture / domain modeling / ports-&-adapters / resilience /
reusable-core" roster already spans most of the grid — the upgrade is to name the
grid explicitly in the charter so gaps are visible.

### 2. How to run adversarial passes

**Canon.** Adversarial critique is a *named, structured* technique with defined
value-added, not "someone being harsh." The CIA **Tradecraft Primer** groups
techniques into *Diagnostic / Contrarian / Imaginative*; the Contrarian family
exists specifically "to challenge prevailing assumptions and groupthink," and
includes **Devil's Advocacy** ("assigning someone the explicit task of arguing
against the consensus") and **Team A/Team B** ("dividing analysts into competing
groups to develop alternative interpretations"). Structured techniques make
"analytic assumptions and reasoning more transparent" and guard against individual
cognitive bias.
[cia.gov/resources/csi/static/Tradecraft-Primer-apr09.pdf](https://www.cia.gov/resources/csi/static/Tradecraft-Primer-apr09.pdf)
(canonical host returns HTTP 403 to automated fetch; **verified via academic
mirror** [stat.berkeley.edu/~aldous/157/Papers/Tradecraft Primer-apr09.pdf](https://www.stat.berkeley.edu/~aldous/157/Papers/Tradecraft%20Primer-apr09.pdf))
— *rationale: the institutional codification of adversarial review; our "YAGNI
skeptic" ≈ Devil's Advocate, "velocity red-team" ≈ a Team-B contrarian.*

Klein's **premortem** supplies the framing that makes a red-team *productive*:
"assume that the project … has just failed … and then generate plausible reasons
why." This *prospective hindsight* increased ability to identify reasons for
outcomes by ~30% (Mitchell/Russo), and crucially *legitimizes dissent* — "too many
people are reluctant to speak up about their reservations during the … planning
phase."
[hbr.org/2007/09/performing-a-project-premortem](https://hbr.org/2007/09/performing-a-project-premortem)
— *rationale: the exact prompt-shape for our velocity red-team, and why an
adversarial persona finds more than "what could go wrong?".*

Paul Graham's **disagreement hierarchy** sets the *quality bar* a critique must
clear: DH0 name-calling → DH3 mere contradiction → DH4 counterargument → **DH5
Refutation** ("to refute someone you probably have to quote them … find a smoking
gun") → **DH6 Refuting the Central Point** ("the most powerful form of disagreement").
[paulgraham.com/disagree.html](https://paulgraham.com/disagree.html)
— *rationale: an adversarial finding at DH3 ("this is over-engineered") is noise;
at DH5/DH6 (quote the file:line, refute the load-bearing justification) it is
signal — the standard our adversarial personas should be held to.*

**How to apply to a ghx audit.** (a) Make ≥1 adversarial persona *mandatory*
(already true) and frame it as a **premortem**: "It is a year from now; ghx's
architecture blocked P3/P4 and velocity collapsed under tech debt. Walk back the
file:line decisions that caused it." (b) Hold every adversarial finding to **DH5/
DH6**: it must *quote* the code/tenet it attacks and refute the *central*
justification, not the tone or a peripheral detail. (c) Consider a formal **Team
A/Team B** split for a genuinely contested call (e.g. "extract the Agent Sidecar
Framework as reusable infra" vs "YAGNI, it's welded and should stay") — two
personas each build the strongest opposing case, and the lead adjudicates the
contrast rather than a lone critique.

### 3. How to structure artifacts (numbering / threading / decision records)

**Canon.** Architectural decisions get **immutable, numbered, supersedable**
records. Nygard's original **ADR**: Title / **Status** (proposed·accepted·
deprecated·superseded) / Context / Decision / Consequences; "ADRs will be numbered
sequentially and monotonically. Numbers will not be reused"; superseded records
*stay* and are marked; "the whole document should be one or two pages … small,
modular documents have at least a chance at being updated."
[cognitect.com/blog/2011/11/15/documenting-architecture-decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
— *rationale: the source of ADR numbering + immutability + one-decision-per-record,
which our docs/adr/NNNN scheme follows directly.*

**MADR** extends this so the *reasoning* is explicit: adds **Decision Drivers**,
**Considered Options**, **Decision Outcome**, and **Pros and Cons of the Options** —
"transforming ADRs from simple records into structured decision documentation that
surfaces reasoning and trade-offs transparently."
[adr.github.io/madr](https://adr.github.io/madr/)
— *rationale: the options-comparison discipline our distillation needs when it
"holds tensions instead of averaging them."*

**Oxide's RFD** process is the closest analog to our *threaded, git-native,
state-tracked* practice: every RFD gets "a persistent number" on creation and moves
through explicit lifecycle states — *prediscussion → ideation → discussion
(as a PR branch) → published → committed → abandoned* — with discussion happening
on a branch before merge to master.
[rfd.shared.oxide.computer/rfd/0001](https://rfd.shared.oxide.computer/rfd/0001)
— *rationale: validates our `AA-000N` thread numbering, branch-per-worker, and
`status:` frontmatter as an established engineering pattern, not a local invention.*

ATAM supplies the *finding* taxonomy that structures an audit *body*: **risks,
non-risks, sensitivity points, tradeoff points**; "risks are synthesized into a set
of risk themes, showing how each one threatens a business driver."
[sei.cmu.edu/documents/629/2000_005_001_13706.pdf](https://www.sei.cmu.edu/documents/629/2000_005_001_13706.pdf)
— *rationale: our High/Med/Low + "tie to a named AGENTS.md tenet" is exactly
"risk theme → business driver"; and ATAM's non-risks are our mandatory "what's
already right."*

**How to apply to a ghx audit.** Our conventions already align tightly (see
§Alignment). Concrete upgrades from the canon: (a) borrow MADR's **Decision
Drivers** + **Considered Options** headings for the *distillation/governing ADR* so
the tension-resolution is legible; (b) adopt ATAM's four-way taxonomy inside
findings — distinguish a **sensitivity point** ("`owner/repo` as a bare string is
where a small change swings correctness — a live panic") from a **tradeoff point**
("the runner port trades one-shot simplicity against warm-session testability") so
the lead sees *which findings are load-bearing vs which are genuine tradeoffs*;
(c) keep the Oxide-style `status:` lifecycle (`audit` → `distillation` →
`proposed` ADR → `accepted`/`superseded`) explicit in frontmatter.

### 4. How to converge to ACTION (not a shelf-ware report)

**Canon.** Three mechanisms separate a review that changes the codebase from a
report that rots. First, **rank + tie to drivers**: ATAM does not emit a flat bug
list — it synthesizes risks into *risk themes*, each *tied to a business driver*,
which is what makes a finding un-ignorable (kill this risk *or* accept you're
threatening driver X).
[sei.cmu.edu/documents/629/2000_005_001_13706.pdf](https://www.sei.cmu.edu/documents/629/2000_005_001_13706.pdf)
— *rationale: ranking by impact-on-a-named-driver, not severity-in-a-vacuum, is
the action-forcing move.* Second, **make invariants executable**: an *architecture
fitness function* is "an objective integrity assessment of some architectural
characteristic," embedded as an automated gatekeeper in CI that gives "immediate,
objective signals about architectural drift as development happens" — a natural
extension of continuous integration.
[thoughtworks.com/insights/articles/fitness-function-driven-development](https://www.thoughtworks.com/insights/articles/fitness-function-driven-development)
— *rationale: the definitive anti-shelfware pattern; a rule that runs every build
cannot become stale advice.* Third, **make the decision + its alternatives
durable**: Google design docs exist to identify issues "while [they are] cheap to
fix" and to document "the high-level implementation strategy and key design
decisions with emphasis on the trade-offs that were considered," including an
explicit **Alternatives Considered** and **Non-Goals** ("things that could
reasonably be goals, but are explicitly chosen not to be").
[industrialempathy.com/posts/design-docs-at-google](https://www.industrialempathy.com/posts/design-docs-at-google/)
— *rationale: converging to action = recording not just the chosen refactor but the
rejected ones and the explicit non-goals, so the decision survives re-litigation.*

**How to apply to a ghx audit.** (a) Rank the distilled sequence by **cross-persona
convergence × impact-on-a-north-star-driver × tractability** (we do this) — but
name the *driver* each top item serves (P3 "swallow the tools", P4 "below ACP",
"main agent's context is sacred"), ATAM-style, so declining an item is an explicit
choice against a driver. (b) **Promote recurring findings into fitness functions**:
the 2026-07-07 audit's import-direction check (`grep` core→frontend = empty) and
god-file threshold (`wc -l`) are *exactly* fitness-function-shaped — commit them as
a `go test`/CI check so they enforce themselves between audits instead of being
re-discovered each run. This is our **largest divergence from canon and biggest
upgrade** (see §Divergence). (c) Give each top recommendation an **owner + a
non-goal** in the governing ADR (design-doc discipline) so it becomes a task, not
a paragraph.

### 5. How a lead adjudicates (the load-bearing step, agentic-first)

**Canon.** The person who ran the fan-out judging its output is a *self-preference
regime*, and in an agentic setting that is now a **measured, causal** failure mode.
Panickssery et al. show LLM evaluators "score their own outputs higher than others'
work of equivalent quality," and — the load-bearing result — "a linear correlation
between self-recognition capability and the strength of self-preference bias":
recognizing your own work *causes* you to favor it.
[arxiv.org/abs/2404.13076](https://arxiv.org/abs/2404.13076)
— *rationale: the empirical basis for "discount shared-prior convergence" and "get
a cross-model-family adjudicator" — when the judge model authored the audits, its
approval is biased toward itself.* Multi-agent debate shows the *upside* — diverse
independent instances converging improves correctness — but only when the instances
are genuinely independent; agents that "read the same code" share priors and their
agreement is correlated, not independent corroboration.
[arxiv.org/abs/2305.14325](https://arxiv.org/abs/2305.14325)
— *rationale: convergence is signal only to the degree the converging reasoners are
independent; head-count of same-model personas overstates it.* The intelligence-
tradecraft complement: Team A/Team B and Devil's Advocacy exist precisely because
consensus among similar analysts is *not* self-validating.
[stat.berkeley.edu/~aldous/157/Papers/Tradecraft Primer-apr09.pdf](https://www.stat.berkeley.edu/~aldous/157/Papers/Tradecraft%20Primer-apr09.pdf)
— *rationale: institutional precedent that a lead must actively break groupthink,
not tally it.*

**How to apply to a ghx audit.** The skill's "final judgment" step is already
canon-aligned and should be treated as *the* deliverable-critical step: (a)
**re-derive load-bearing claims** — open the cited `file:line`, re-run the recompute
— because you cannot trust a same-family fan-out's self-report; (b) **discount
shared-prior convergence** — three Claude personas agreeing because they read the
same `acp.go` is *one* observation, not three; the independent signal is an
*independent recompute* or a *different model family* reaching the same place;
(c) route the **mechanical metrics** pass to a Codex/GPT-5.5 worker so at least one
audit comes from a different model family (breaks the self-recognition→self-
preference loop by construction); (d) for the highest-stakes call, get a **cross-
family adjudicator** (Codex/gpt-5.5 or Goga). "No", "this is fine", and "the
premise is wrong" are valid, ATAM-*non-risk* outcomes — a clean boundary verified
is as valuable as a finding.

---

## Where ADR-0035/0036 (and the current audit practice) already ALIGN with the canon

Verified against `docs/audits/architecture-2026-07-07.md`, the `arch-audit`
SKILL.md, and ADR-0035/0036 (read from git; they live on other branches, not this
worktree's tree).

- **ATAM non-risks ≙ our mandatory "What is clean (verified, not assumed)" section.**
  The 2026-07-07 audit devotes its most valuable real estate to verifying the
  core/frontend boundary is *healthy* (core imports no frontend; CLI is the sole
  composition root). That is textbook ATAM *non-risk* reporting, and the skill
  correctly makes it mandatory to stop personas manufacturing debt.
- **ATAM risk-themes-tied-to-business-drivers ≙ "every finding ties to a named
  AGENTS.md tenet / NORTH_STAR phase."** H1's tie to the "no compatibility junk"
  tenet and "the main agent's context is sacred" north-star line is exactly ATAM's
  "how this risk threatens a driver."
- **ADR/Oxide-RFD numbering + supersession ≙ our `docs/adr/NNNN` + `NNNN.1`
  sub-decisions + `thread:`/`status:` frontmatter.** ADR-0036 explicitly
  "supersedes ADR-0035 item 5" — Nygard's immutable-but-superseded discipline,
  applied.
- **Convergence-as-signal ≙ multi-agent debate + Team A/B.** ADR-0035 ranks on the
  fact that "independent auditors converged on the same hotspots … that convergence,
  not any single report, is the signal" — the debate-improves-factuality mechanism,
  named.
- **Self-preference guardrails ≙ the skill's "discount shared-prior convergence" +
  "cross-family check" + routing metrics to GPT-5.5.** This is *ahead* of most
  human review practice and directly matches the Panickssery result — arguably the
  most sophisticated agentic-first move in the whole loop.
- **Adversarial personas ≙ Devil's Advocacy / premortem.** The "YAGNI skeptic" and
  "velocity red-team" (AUD4 "adversarial-velocity") are Contrarian-family techniques
  by another name, with the YAGNI persona explicitly enforcing right-sizing.
- **Behavior-preserving, phased, non-rewrite sequence ≙ MADR/design-doc rigor +
  fitness-function spirit** (each step shippable and provable byte-identical).

## Where it DIVERGES — honest gaps and the upgrades the canon implies

1. **No standing fitness functions (biggest gap).** Our metrics pass is a *one-shot
   snapshot* (`gocyclo`, `wc -l`, `grep` import-direction) inside an audit. The
   canon's anti-shelfware core (Ford/Parsons/Kua) is to encode those invariants as
   *executable checks that run every build*. Recurring findings — core→frontend
   import-direction, god-file line thresholds, "no `callTool` compat path" — should
   graduate into committed `go test`/CI fitness functions so drift is caught
   continuously, not re-discovered each audit. Highest-leverage divergence.
2. **No quality-attribute utility tree at the front end.** ATAM/SAAM *start* by
   operationalizing vague qualities into prioritized, scenario-backed concerns
   (SAAM: a scenario is "a short description of an anticipated or imagined use of a
   system"; ATAM ranks them in a *utility tree*). Our charter picks a scope but does
   not rank *which quality attributes* matter for it or back them with scenarios.
   Adding a lightweight utility tree ("for scope = sidecar runner: replaceability
   9, testability 8, resilience 7; scenario: 'swap claude-agent-acp for the Claude
   Agents SDK by config'") would make personas sharper and the ranking pre-justified.
   [Note: real ATAM/SAAM are multi-day human workshops with live stakeholder
   elicitation; I extract the *analytic scaffolding*, not the workshop logistics —
   the async-AI mapping is my synthesis, not SEI-validated.]
3. **View coverage is implicit, not gridded.** 4+1/C4 argue for *deliberate* view
   coverage; we pick personas from a catalog without a checklist that every
   relevant view/zoom-level is hit. A one-line coverage grid in the charter closes
   the "we never audited the process/concurrency view" risk.
4. **Adjudication/contrarian pass is optional, not standard.** The skill lists
   `.6-adjudication` as *optional*; Team A/B and Devil's Advocacy argue a contrarian
   pass should be *standard* for any contested call, and the self-preference result
   argues a cross-family adjudicator should be *mandatory* for high-stakes items.
5. **Owners + explicit non-goals are thin.** Design-doc discipline names an
   *Alternatives Considered*, *Non-Goals*, and an owner per decision. Our
   recommendations carry severity + sequence but the *owner* is implicit and
   *non-goals* are scattered — naming both per top item converts findings into tasks.
6. **DH5/DH6 bar is not enforced on findings.** Nothing currently *requires* an
   adversarial finding to quote-and-refute the central justification; adding that
   acceptance test would raise finding quality and cut DH3-level "this smells"
   noise.

## Honestly-flagged gaps in THIS research

- **ATAM/SAAM are pre-AI, stakeholder-workshop methods.** I extracted the
  transferable analytic core (utility tree, risk/non-risk/sensitivity/tradeoff,
  scenario operationalization). Mapping them onto an *asynchronous AI-agent fan-out*
  is my **labelled synthesis**; SEI never validated that transposition.
- **The agentic papers are not about architecture auditing.** Du et al. (math/
  reasoning benchmarks) and Panickssery et al. (judging text quality) establish the
  *mechanisms* (independent reasoners reduce correlated error; self-recognition
  drives self-preference). Applying them to "personas as debate participants" and
  "same-model judge is biased toward its own audits" is **labelled synthesis
  extrapolation** — the mechanism transfers cleanly, but no paper studies our exact
  setting.
- **"Reviews need severity + owner or they become shelfware" has no single canonical
  source.** That claim is **assembled synthesis** from ATAM (risk-themes→drivers),
  fitness functions (executable = enforced), and design-doc/RFC culture (a decision
  with a state + owner). Reasonable, but stitched, not quoted from one authority.
- **The "numbered + threaded artifacts for *AI-agent* consumption" angle is ours.**
  ADR/RFD numbering/threading canon is human-oriented; the agent-consumption framing
  is our context (**labelled synthesis**), not in the sources.
- **CIA primer degradation.** The canonical cia.gov PDF returned **HTTP 403** to
  automated fetch; I verified content via the **Berkeley academic mirror** of the
  identical `Tradecraft-Primer-apr09.pdf`. The mirror is a *scanned* PDF, so some
  extracted lines are paraphrase-grade, not verbatim (disclosed at use site).
- **Not covered (adjacent, out of scope):** DDD bounded-contexts / anti-corruption
  layers; George Fairbanks' *risk-driven* "just enough architecture"; empirical
  code-review research (e.g. Google's "Modern Code Review" study) — any could
  deepen a future round on *review economics* specifically.
- **MADR/SAAM/ATAM/4+1 PDFs** were summarized by WebFetch's small model, not read
  line-by-line by me; quotes marked with quotation marks are as WebFetch returned
  them. High-stakes verbatim quotes should be re-checked against the primary before
  external citation.

---

## Source Ledger

All URLs verified via WebFetch on 2026-07-07 unless noted. "Deep" = specific
method page/paper/post, never a homepage.

| # | Source (deep URL) | What it anchors | Verified |
|---|---|---|---|
| 1 | [Nygard, *Documenting Architecture Decisions* (cognitect.com, 2011)](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) | ADR: Title/Status/Context/Decision/Consequences; sequential monotonic numbering, no reuse; supersede-don't-delete; 1–2 pages | ✅ WebFetch |
| 2 | [MADR — *About MADR* (adr.github.io)](https://adr.github.io/madr/) | Adds Decision Drivers, Considered Options, Pros/Cons, Confirmation — explicit option comparison | ✅ WebFetch |
| 3 | [Kazman et al., *SAAM: Scenario-Based Analysis of Software Architecture* (SEI, ICSE'94 repr.)](https://www.sei.cmu.edu/documents/213/1996_019_001_29912.pdf) | Scenarios operationalize quality attributes; direct/indirect scenarios; scenario interactions; compare competing architectures | ✅ WebFetch (PDF) |
| 4 | [Kazman/Klein/Clements, *ATAM: Method for Architecture Evaluation*, CMU/SEI-2000-TR-004](https://www.sei.cmu.edu/documents/629/2000_005_001_13706.pdf) | 9 steps; utility tree; **risks/non-risks/sensitivity/tradeoff points**; risk-themes tied to business drivers; stakeholder diversity | ✅ WebFetch (PDF) |
| 5 | [Kruchten, *Architectural Blueprints — the 4+1 View Model* (IEEE Software 1995)](https://www.cs.ubc.ca/~gregor/teaching/papers/4+1view-architecture.pdf) | Logical/process/development/physical + scenarios; different stakeholders need concurrent views | ✅ WebFetch (PDF) |
| 6 | [Simon Brown, *C4 model — Abstractions* (c4model.com)](https://c4model.com/abstractions) | Context/container/component/code zoom levels; each level tells a different story to a different audience | ✅ WebFetch |
| 7 | [Ford/Parsons/Kua via Thoughtworks, *Fitness function-driven development*](https://www.thoughtworks.com/insights/articles/fitness-function-driven-development) | Fitness functions: objective, automated, CI-embedded architectural conformance checks (anti-shelfware) | ✅ WebFetch |
| 8 | [Malte Ubl, *Design Docs at Google* (industrialempathy.com)](https://www.industrialempathy.com/posts/design-docs-at-google/) | Design docs: cheap early issue-finding, consensus-before-code, Alternatives Considered, Non-Goals | ✅ WebFetch |
| 9 | [Oxide Computer, *RFD 1 — Requests for Discussion*](https://rfd.shared.oxide.computer/rfd/0001) | Persistent numbering; lifecycle states prediscussion→…→committed/abandoned; branch-based discussion | ✅ WebFetch |
| 10 | [Paul Graham, *How to Disagree* (DH0–DH6)](https://paulgraham.com/disagree.html) | Disagreement hierarchy; DH5 refutation (quote + smoking gun), DH6 refute the central point — critique-quality bar | ✅ WebFetch |
| 11 | [Gary Klein, *Performing a Project Premortem* (HBR 2007)](https://hbr.org/2007/09/performing-a-project-premortem) | Prospective hindsight ("assume we failed"); +~30% risk-finding; legitimizes dissent | ✅ WebFetch |
| 12 | [CIA, *A Tradecraft Primer: Structured Analytic Techniques* (2009)](https://www.cia.gov/resources/csi/static/Tradecraft-Primer-apr09.pdf) | Diagnostic/Contrarian/Imaginative families; **Devil's Advocacy**, **Team A/Team B** counter groupthink | ⚠️ canonical host **403**; verified via mirror ↓ |
| 12b | [— academic mirror (stat.berkeley.edu)](https://www.stat.berkeley.edu/~aldous/157/Papers/Tradecraft%20Primer-apr09.pdf) | Same document; scanned PDF (some paraphrase-grade extraction) | ✅ WebFetch (mirror) |
| 13 | [Du, Li, Torralba, Tenenbaum, Mordatch, *Improving Factuality and Reasoning … through Multiagent Debate* (arXiv:2305.14325)](https://arxiv.org/abs/2305.14325) | Independent LLM instances propose-and-debate over rounds → better factuality/reasoning, less hallucination | ✅ WebFetch |
| 14 | [Panickssery et al., *LLM Evaluators Recognize and Favor Their Own Generations* (arXiv:2404.13076, NeurIPS 2024)](https://arxiv.org/abs/2404.13076) | Self-preference bias; **linear correlation between self-recognition and self-preference** — the cross-family-judge rationale | ✅ WebFetch |

**Count: 14 primary/canonical sources** (one accessed via academic mirror after a
403 on the canonical host). All link-verified via WebFetch; degradations disclosed.
