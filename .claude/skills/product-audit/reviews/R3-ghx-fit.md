# R3 — ghx-fit & North-Star Alignment Review of the `product-audit` Skill

*Adversarial reviewer scope: does this skill serve THIS product and THIS north
star, or is it generic PM boilerplate? Target under review:
`.claude/skills/product-audit/SKILL.md` (+ `research/06-agentic-first-lens.md`).
Grounded in `docs/NORTH_STAR.md`, `AGENTS.md`, `README.md`, `docs/evals/TRUST.md`,
and `docs/audits/*`. External claims carry a specific deep URL; unverified ones
are marked.*

---

## Abstract

The skill is **not** generic PM boilerplate. Its agentic-first re-pricing (Prime
Directive 2; `research/06 §3`) is the best thing in it — it correctly inverts
engagement/retention, kills the survey in favor of ablation, and encodes the
two-consumer model, all grounded in ghx's own tenets and Anthropic's
context-engineering guidance. Run today, it *would* surface most of the real
high-leverage issues (the moat-vs-commoditization tension, the uncalibrated-judge
risk, zero-CLI adoption). The problem is not missing lenses; it is **mis-calibrated
ghx-specific anchors** in three places that would let an auditor conclude "ghx
obviously wins" against a strawman instead of against the real, shipped
competitor. The single most important gap: the skill's canonical "true
alternative" for the competitive scope is *"the agent greps files itself"*
(persona #10; `research/06` row 7) — but the actual alternative already shipped
inside ghx's primary host. **Claude Code ships a built-in `Explore` subagent: "a
fast, read-only agent optimized for searching and analyzing codebases" that
"keeps exploration results out of your main conversation context" and controls
cost "by routing tasks to faster, cheaper models like Haiku"**
([Claude Code docs, Create custom subagents](https://code.claude.com/docs/en/sub-agents),
verified). That is ghx's own value proposition — context-subtracting recon
delegated to a cheap read-only agent — as a native feature of the runtime ghx
targets. The skill must name this so the moat is tested against the real threat.
Secondary gaps: the token-economics scope collapses ghx's three-level
signals-per-token model to one level; the skill uses a *failed* milestone (M8
anticipation) as a positive "delight" exemplar; and `~/.ghx` artifacts are framed
as a human-debug surface when the north star makes them a first-class
**agent-facing API**. The in/out-of-scope boundary is **substantially correct and
non-overlapping** with the existing code audits, with two small completeness
fixes.

---

## What fits ghx well (checked with the same rigor as the findings)

These are real strengths, not courtesy. A red-team that can't say what's sound
has no credibility when it says what's broken (skill's own honesty guard, PD4).

1. **The agentic-first re-pricing is load-bearing and correct.** Prime Directive 2
   and `research/06 §3`'s HOLDS/ADAPT/OUTDATED-OR-INVERTED table are the antidote
   to boilerplate. The **engagement/retention INVERSION** — "for a
   context-subtracting product, more tool-calls/time-per-task is a vanity metric
   pointing the wrong way; the win is a high-signal answer and leaving fast"
   (SKILL PD2) — is exactly right for ghx and is precisely what a generic PM audit
   gets backwards. It is grounded in `NORTH_STAR.md` "the main agent's context is
   sacred" and the "signals per token" moat.

2. **The two-consumer model is cleanly encoded** (PD3): agent = primary (Tier B),
   human operator = real-but-secondary (Tier A), "never let a founder's aesthetic
   preference or a human-facing vanity metric masquerade as *agent* value." This
   matches `NORTH_STAR.md §3` ("the main agent is a first-class consumer") without
   dismissing the dogfooding founder.

3. **The moat LENS is present and pointed the right way.** The Moat scope
   (7-Powers, "brain not tools"), the Industry/Market scope ("is the tool layer
   commoditizing as base models improve?"), and the anti-pattern "assuming today's
   model capability is permanent … a CLI-based moat is a current advantage, not a
   Power" all correctly track `NORTH_STAR.md` "The Moat." The framework is right;
   only its *worked example* of the alternative is weak (Finding 1).

4. **★ persona selection is domain-true.** Treating AX, AI PM Pragmatist,
   Platform/Scale Realist, and Experimentation Rigorist as near-mandatory, and
   having the AX lens "subsume and outrank" the DevEx Advocate for agent surfaces,
   is the correct routing for a CLI consumed by an AI agent inside an eval harness.

5. **The anti-patterns section is ghx-grounded, not decorative** — human-pretty
   output as context tax, delight theater, vanity funnels, tool-count-as-progress,
   and "trusting evals you can't audit … an uncalibrated LLM-judge is the new
   leading-the-witness" map directly onto `AGENTS.md` Visibility/Truthfulness and
   `docs/evals/TRUST.md` hole H1 (judge κ pending).

6. **Boundary discipline and evidence contract are inherited, not reinvented.**
   The out-of-scope routing to the existing code audits, the "What is actually
   fine" section, and the recomputable-evidence requirement all mirror the repo's
   real discipline (`AGENTS.md` Evidence Contract; the code audits' own honesty
   guards).

---

## Ranked findings

### F1 — MUST-FIX (confidence: high). The competitive scope's canonical "true alternative" is a strawman; the real alternative shipped natively in ghx's host.

**Quoted target.** GTM/Positioning scope: *"What's the true alternative —
including 'the agent greps files itself'"* (SKILL Personas #10 signature
question; Scopes/Tier A "Competitive Analysis": *"Dunford competitive-alternatives
(incl. 'the agent does it itself')"*; `research/06` row 7: *"Dunford's 'true
alternative' for ghx = 'the agent greps/reads files itself'"*).

**Flaw (DH5/DH6 — quoted claim, named defect).** The canonical alternative is an
*unassisted* baseline. But ghx's primary consumer is a main agent running inside
a host that **already delegates context-subtracting reconnaissance to a cheap
read-only sub-agent**. The agent does not grep files itself; its host spawns
`Explore`. Anchoring the competitive/positioning scope on the weak alternative
lets an auditor conclude "ghx obviously beats raw grep" (true, and irrelevant)
and skip the audit that matters: does the sidecar's *brain + evidence contract +
GitHub-world discovery + persistence* beat a native subagent that already does
the context-subtraction for free?

**Evidence (external, verified).** Claude Code, *Create custom subagents*: Explore
is *"A fast, read-only agent optimized for searching and analyzing codebases,"*
with *"read-only tools; Write and Edit are denied,"* it *"keeps exploration
results out of your main conversation context,"* takes a thoroughness level
*"quick / medium / very thorough,"* and the docs pitch subagents to *"Control
costs by routing tasks to faster, cheaper models like Haiku"*
([code.claude.com/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents),
fetched + grep-verified 2026-07-07). This is a near-one-to-one restatement of
ghx's thesis (`README.md`: "cheap, specialized sidecar agent that a main agent
delegates code reconnaissance to"; `NORTH_STAR.md` tenet "the sidecar stops at
reconnaissance"). The tool layer is commoditizing in parallel: the **official
GitHub MCP server** does *"fast and precise code search across ALL GitHub
repositories using GitHub's native search engine"*
([github/github-mcp-server](https://github.com/github/github-mcp-server), search-
verified), and **zilliztech/claude-context** is a *"Code search MCP for Claude
Code"* ([repo](https://github.com/zilliztech/claude-context), search-verified).

**Why this SHARPENS rather than kills ghx (and why the skill must say so).** The
Explore alternative is exactly the case where "brain not tools" earns its keep:
Explore explores the *local working tree*, not the GitHub open-source world; it
returns a *prose summary*, not a schema-validated evidence contract with
citations/commands/uncertainty (`README.md` "an evidence contract, not a string",
ADR-0021); it has no persistent inspectable ledger, no OTel audit trail, no
discovery tier, no tier escalation. That is a defensible differentiation — but a
product audit only surfaces it if the skill names the real alternative.

**Recommendation.** In the Competitive, GTM/Positioning, and Moat scopes, replace
(or supplement) *"the agent greps files itself"* with the **strong, shipped
alternative**: *"the agent's host already delegates recon to a native,
context-subtracting, cheaper read-only sub-agent (e.g. Claude Code's Explore); a
GitHub-code-search MCP server already exists."* Add a Competitive-Paranoid
kill-the-product scenario: *"the host runtime ships a recon subagent with an
evidence-contract output and GitHub-world reach — what's left of ghx's moat?"*
And add the eval consequence (F1b): the eval baselines are `plain / ghx /
ghx-sidecar` (`NORTH_STAR.md` workstreams), which **do not include the native
host subagent**; a product audit should flag that the ablation measures ghx
against unassisted exploration, not against the real alternative. Tag F1b SHOULD
(confidence: medium — baseline choice is an eval-design call, but "does the
ablation compare to the true alternative" is squarely the Evals-as-user-research
scope's job).

---

### F2 — MUST-FIX (confidence: medium-high). Token-economics scope collapses ghx's three-level signals-per-token model to one level, hiding the sidecar-internal persona tax and the P4 exit trigger.

**Quoted target.** Tier-B "Token economics / signals-per-token" scope: *"What
context tax does it levy per unit of returned signal?"* elaborated in `research/06
§4.2` as *"how many tokens the product injects into the **main** agent's window
per unit of returned signal."*

**Flaw.** `NORTH_STAR.md` (lines 99–100) is explicit: *"ADR-0016.6 defines SPT at
three levels (main-agent, sidecar-internal, whole-workflow) and every run reports
it."* The skill's scope audits only the **main-agent** level. It therefore cannot
see (a) **sidecar-internal SPT**, where the *"~400-line persona doctrine paid
every session"* lives — which `NORTH_STAR.md` names as **P4 exit trigger #2** (*"a
measured SPT plateau attributable to harness overhead ACP cannot remove"*); and
(b) **whole-workflow SPT**, the only level at which ghx can be compared to the
native-Explore alternative (F1). A CLI-consumer product whose central metric is
three-level SPT should not have its token scope silently reduce to one level.

**Evidence.** `NORTH_STAR.md` "The Moat" ("SPT at three levels … every run reports
it"); "Protocols are stepping stones" tenet, exit trigger (2) ("the ~400-line
persona doctrine paid every session"); workstream B8 "Persona proficiency:
mining-driven persona revisions" — the sidecar's own doctrine is a tracked product
surface, not just the main agent's.

**Recommendation.** Expand the token-economics scope to name all three SPT levels,
and explicitly add **the sidecar's own persona/doctrine as an in-scope
context-budget surface** (it is the internal analog of the main-agent
progressive-disclosure audit). The AX lens already flags "doctrine tax" (#16); the
token scope should meet it at the sidecar-internal level. This also gives the P4
"below ACP" bet a home in the audit.

---

### F3 — SHOULD (confidence: high). The skill uses a milestone that FAILED its gate (M8 anticipation) as a positive "delight" exemplar.

**Quoted target.** `research/06 §2(e)` and row 5: *"The agent-native form of
'delight' … is **anticipation and zero-latency** (ghx M8: answering a question
before it is asked)"*; SKILL PD2 lists "delight (→ delight theater)" but carries
the anticipation framing forward from the research.

**Flaw.** `NORTH_STAR.md` M8 status: *"v1 decided (ADR-0031.1) but **D1 gate
FAILED** — nextReads recall 0.071/0.000, field sparse in practice; prefetch build
correctly halted."* The skill cites anticipation as the aspirational bright spot
while the feature *failed its predictive-recall gate*. An auditor reading the
skill's own example could log anticipation as a strength — the exact
"capability-claim-with-no-passing-eval" the AI PM Pragmatist (#15) and North-Star
Zealot (#6) exist to catch. The skill's example undercuts its own personas.

**Recommendation.** Re-cast M8 in the skill from a "delight" exemplar into a
**live worked example of a finding**: a decided capability whose enabling signal
(`nextReads` recall) failed its gate — precisely the North-Star Zealot's "gameable
/ aspirational metric" and the AI PM Pragmatist's "traced eval behind the claim."
Keep anticipation as the *category* of agent-native "delight," but stop presenting
ghx's specific unshipped/failed instance as evidence it works.

---

### F4 — SHOULD (confidence: medium). `~/.ghx` artifacts are framed as a human/eval-debug surface; the north star makes them a first-class *agent-facing API*.

**Quoted target.** SKILL Surfaces: *"Evals & traces — `~/.ghx/sessions/`, eval
reports under `docs/evals/`, OTel traces."* The artifact store appears only under
the eval/debug bucket.

**Flaw.** `NORTH_STAR.md §3` (sharpened 2026-07-05): *"the **main agent is a
first-class consumer** … visibility means an agent reads `~/.ghx` session
artifacts (reports, traces, ledgers) as an **API surface**."* `README.md`
reinforces it: every ask response *"ends with an `artifacts:` pointer naming this
session directory and the ask's root trace ID, so the calling agent never has to
guess where the audit trail lives."* The report/ledger/trace **schema legibility
to a consuming agent** is an AX concern, not a human-debug concern — and the skill
does not route it to the AX or Trust scope as such.

**Recommendation.** Add `~/.ghx` artifacts (report JSON, ledger, `live.jsonl`, the
`artifacts:` pointer) to the **Live-product / AX** surface, and give the AX and
Trust-&-verifiability scopes an explicit sub-question: *"is the on-disk artifact
schema legible and composable for a downstream **agent** reading it as an API — or
only for a human with `jq`?"* This is the F1/F4 pair that distinguishes ghx from
Explore's prose summary; the skill should make the auditor test it.

---

### F5 — SHOULD (confidence: medium). The discovery tier (repo-optional, GitHub-wide recon) is a distinct product surface and a competitive differentiator, and it is unnamed.

**Quoted target.** SKILL Surfaces: *"run the real CLI, the MCP tool, a real
sidecar `ask`."* Phase-6 playbook: *"run a real coding-agent session end-to-end."*
Neither names the no-repo **discovery** path.

**Flaw.** `NORTH_STAR.md §1` (ADR-0019.1): reconnaissance *"starts at **discovery**
— 'which repos do X' … A repo is optional scope, never a requirement … the
dogfood week proved the demand: the week's first real question couldn't reach a
repo-locked sidecar at all."* This is a shipped product surface (B5, "Done
2026-07-06") **and** the sharpest differentiator vs the native Explore alternative
(Explore reads the local checkout; ghx sweeps the GitHub open-source world). A ghx
product audit that never issues a repo-less `ask` misses both the surface and the
moat evidence.

**Recommendation.** Add a discovery `ask` (`ghx sidecar ask "which repos do X"`,
`README.md §4`) to the Surfaces list and to the Phase-6 playbook as a mandatory
cell, and tie it to the F1 competitive scope as the "what the host subagent
cannot do" probe.

---

### F6 — SHOULD (confidence: medium). The out-of-scope cross-reference list is stale/incomplete and doesn't distinguish product-sequencing YAGNI from structural YAGNI.

**Quoted target.** SKILL "In scope / out of scope": the out-of-scope list names
`architecture-*.md`, `coupling-cohesion-*.md`, `complexity-hotspots-*.md`,
`design-patterns-*.md`, and calls out `adversarial-velocity` as the
velocity/tech-debt companion.

**Flaw.** It omits `docs/audits/failure-class-inventory-*.md` and the
`docs/audits/architecture-vision/adversarial-yagni-*.md` audit. The YAGNI audit is
a genuine adjacency risk: *"should we build feature X yet"* (product sequencing,
premature-adjacency) is **in-scope** for product-audit (Simplicity/Anti-Bloat
persona #8; Adjacent-Opportunity "is it premature"), while *"should this be a new
package/interface/seam"* (structural YAGNI, the actual content of
`adversarial-yagni-2026-07-07.md`, scope: *"short-term risk, solo-developer
velocity, YAGNI counterweight"*) is **out-of-scope**. The skill should name the
audit and draw that line so the two don't collide or both punt.

**Recommendation.** Add both files to the cross-reference list; add one sentence:
*"Product-sequencing YAGNI (build-order / premature adjacency) is in scope;
structural YAGNI (packages/interfaces/seams) belongs to
`architecture-vision/adversarial-yagni-*`."*

---

### F7 — OPTIONAL (confidence: medium). Sharpen the eval-mechanics boundary with the concrete in-scope example (the uncalibrated-judge claim-validity question).

**Quoted target.** SKILL out-of-scope: *"Eval mechanics (scorer code, gate math) →
… `docs/evals/TRUST.md`. This skill audits whether the evals answer the product
question, not whether the Go is correct."*

**Flaw.** The line is correct but under-specified, and the most important ghx
product finding sits right on it: **the sidecar's headline quality claim currently
rests on deterministic fact-recall gates, and the calibrated judge that would
validate the *quality* comparison is not done** (`NORTH_STAR.md` M4 verdict is "a
conservative floor" scored "only by deterministic fact-recall gates"; C4 judge is
"← frontier … no judge score is citable until the founder-labeled κ ≥ 0.6
calibration gate passes"; `TRUST.md` H1 open). Whether "THESIS SUPPORTED" /
"sidecar beats ghx on quality" is currently *supportable by the evidence that
exists* is a **product-claim-validity** question (in scope), even though "is
`rewards.go` correct" is not. A reader could mis-apply the boundary and punt the
whole eval-trust question.

**Recommendation.** Add the worked example to the boundary text: *"In scope: is the
product's central quality claim currently supportable by committed evidence (e.g.
does a 'better trajectories/reasoning' claim rest on a judge that is not yet
calibrated — `TRUST.md` H1)? Out of scope: whether the scorer's Go is correct."*

---

### F8 — OPTIONAL (confidence: medium). Monetization/COSS scope is premature for a pre-commercial internal tool; state the live reframe up front.

**Quoted target.** Tier-A "Business Model / Monetization": *"Cagan four big risks;
Jacks COSS/open-core — internally reframes as cost-to-serve/episode."*

**Flaw (mild).** ghx today has one consumer: *"We are the only consumers of this
product today"* (`AGENTS.md` Engineering Tenets). Full COSS/open-core/who-pays
analysis is premature; the *only* live form is cost-to-serve per episode (token
cost to run the sidecar, which is a real north-star metric). The skill already
hedges ("internally reframes"), so this is a sharpening, not a defect.

**Recommendation.** Lead the scope with the reframe: *"Pre-commercial: monetization
proper is N/A; the live question is cost-to-serve per episode (sidecar token/latency
cost vs. signal delivered)."* Keep the COSS framing parked for when a commercial
motion exists.

---

### REJECT-CANDIDATE (considered, rejected — for honesty). "Hardcode current milestone state into the skill so audits catch M8-failed / judge-pending."

One could argue the skill should embed a live-state table (M8 gate failed, C4
pending, discovery shipped) so audits never miss them. **Reject.** A reusable
skill that hardcodes transient milestone status rots within a loop iteration and
duplicates `NORTH_STAR.md` (which the skill already mandates loading first: PD1,
Process step 1). The correct mechanism — *load the north star, trace every finding
to it* — is already present and is the right design. The fixes above (F3 remove a
stale positive example; F1 name a durable competitor class; F5 name a durable
surface) are about *durable* anchors, not a live-state snapshot. Confidence: high.

---

## Boundary check — is in/out-of-scope drawn correctly?

**Verdict: substantially correct and non-overlapping, with two completeness fixes
(F6, F7).** I read the existing audits to test the boundary against reality:

- `architecture-2026-07-07.md` (core/frontend boundary), `coupling-cohesion-*`,
  `complexity-hotspots-*`, `design-patterns-*` — all are `file:line`, ranked by
  **engineering velocity / correctness**, tied to `AGENTS.md` Engineering Tenets.
  The product-audit's findings are framed in **product/north-star** terms. No
  overlap; the skill's "read code as evidence, frame findings in product terms"
  rule (When-to-use, In-scope) is the right seam.
- `adversarial-velocity-2026-07-07.md` — the skill explicitly claims it as the
  *"velocity/tech-debt red-team … companion, not overlap."* Verified: that audit's
  own header says it is the velocity/tech-debt lens ranked by velocity-cost. Line
  drawn correctly.
- `failure-class-inventory-*` and `architecture-vision/adversarial-yagni-*` are
  **missing** from the cross-reference list (F6). The YAGNI audit is the one real
  adjacency risk, resolved by the product-vs-structural-YAGNI distinction in F6.
- The eval-mechanics → `TRUST.md` routing is correct in principle but the
  boundary's most consequential product finding sits on it (F7).

**Nothing is mis-routed as out-of-scope-that's-actually-product** except the
under-specified eval-claim-validity case (F7). **Nothing is claimed in-scope
that's actually out** — the Phase-6 "confidently-wrong report = severity-4" and
the trust/verifiability scope are legitimately product/AX concerns, not code
review. The boundary is the skill's strongest structural feature after the
agentic-first re-pricing.

---

## Source Ledger

External sources (deep URL + why it matters). Repo files (NORTH_STAR, AGENTS,
README, TRUST.md, the audits, `research/06`) are the **audit target / internal
grounding**, not external sources, and are cited inline by path.

| # | Source | URL | Verify status | Why it matters here |
|---|---|---|---|---|
| 1 | Claude Code — *Create custom subagents* (built-in `Explore` subagent) | https://code.claude.com/docs/en/sub-agents | **Verified** (WebFetch + grep 2026-07-07): "fast, read-only agent optimized for searching and analyzing codebases"; "keeps exploration results out of your main conversation context"; "read-only tools; Write and Edit are denied"; "Control costs by routing tasks to faster, cheaper models like Haiku"; thoroughness quick/medium/very-thorough | F1 anchor: the real, shipped alternative to ghx is the host's native context-subtracting recon subagent — not "the agent greps files itself." |
| 2 | Sourcegraph — *Changes to Cody Free/Pro/Enterprise* (Cody→Amp agentic pivot) | https://sourcegraph.com/blog/changes-to-cody-free-pro-and-enterprise-starter-plans | Search-verified (snippet; not deep-fetched) | F1 context: the code-graph-context incumbent is pivoting to agentic multi-repo workflows — a second competitive vector the skill's Competitive scope should name. |
| 3 | Official GitHub MCP Server | https://github.com/github/github-mcp-server | Search-verified (title/URL) | F1 tool-layer commoditization: "code search across ALL GitHub repositories using GitHub's native search engine" — direct pressure on ghx's CLI/tool layer (Wardley lens the skill already carries). |
| 4 | zilliztech/claude-context — code-search MCP for Claude Code | https://github.com/zilliztech/claude-context | Search-verified (title/URL) | F1: a code-search MCP built specifically for Claude Code — evidence the "reachable & default in MCP ecosystems" distribution scope has live occupants. |
| 5 | Anthropic — "subagent-heavy workflows can consume ~7x tokens" (secondary attribution) | (surfaced via secondary write-ups; primary not located this pass) | **UNVERIFIED** — do not cite as fact | Cautionary counter-data for the token-economics scope (F2): delegation is not free; ghx must beat native-Explore delegation on *whole-workflow* SPT, not just beat unassisted grep. Treated as a hypothesis only. |

**Distinct external sources: 4 verified/search-verified + 1 explicitly unverified.**
Research artifact `06`'s own 13-source ledger (Anthropic context-engineering, MCP
spec, Biilmann AX, Willison, Hamel, SWE-bench, τ²-bench, etc.) was read and found
sound; it is the skill's citation of record and is not re-litigated here.
