# R2 — PM Completeness, Correctness & 2026 Currency Review of `product-audit`

*Adversarial review as a senior product / AI-product leader. Scope: what important
audit lens, scope, framework, or failure mode is MISSING or WRONG, and is the
"agentic-first" reframe actually correct and current. Read against the SKILL.md,
`research/02` (personas), `research/04` (Tier-A scopes), `research/06` (agentic-first
verdicts), and `docs/NORTH_STAR.md`.*

---

## Abstract

This is a strong, unusually self-aware skill. Its agentic-first spine — context-as-
finite-resource, AX as the replacement for usability, evals as user research, the
HOLDS/ADAPT/INVERTED audit of classic canon — is correct and current for mid-2026, and
its anti-dilution discipline (redundant-pair callouts, "span the axes," ★-gating) is
exactly right. The reframe is **not** cargo-culted; the research files are more careful
than most published AI-PM writing.

I found **no fatal gap and no verdict I would overturn wholesale.** What I did find is a
small set of genuinely outcome-changing omissions, all of which are *additions to
existing scopes* rather than new personas (so they respect the signal-dilution cost):
(1) **provenance / license lineage of the code ghx surfaces** — a trust-and-viability
failure mode that none of the 17 scopes name, and that flows real copyleft/attribution
liability straight into the main agent's context; (2) the **Evals-as-user-research scope
cites the wrong benchmark class** — SWE-bench (issue *resolution*) and τ²-bench
(conversational policy) instead of the code-*localization/retrieval/navigation*
benchmarks that are ghx's actual job and that proliferated in 2025-26; and (3) the
**engagement/retention "INVERTED" verdict is over-compressed in the SKILL.md operational
layer** in a way that risks an auditor discarding cross-session tool-selection retention,
which is the single most important agent-PMF signal. Plus one under-weighted axis
(reliability/latency-as-product) and two removal/consolidation candidates (DevEx Advocate
is now subsumed by AX; Tier-B has real internal overlap). I close with an explicit "do
not add" (pricing/packaging depth) so my adds can be weighed against my resists.

---

## What's right / what NOT to add (and why)

Weight my findings below against these — I looked hard at each and concluded the skill
is already correct.

- **The agent-as-consumer mental model is current and load-bearing, not decoration.**
  `research/06 §2` grounds "signals per token" in Anthropic's context-engineering post
  and the ACI framing. As of July 2025 this is *empirically* backed by Chroma's
  context-rot study (18 frontier models, quality degrades monotonically with input
  length even on trivial tasks) — the reframe has gotten *more* correct since it was
  written, not less. Do not soften it.

- **The engagement/retention inversion is directionally right.** For a context-*subtracting*
  product, per-question main-agent tool-calls/dwell as a headline success metric is
  genuinely a vanity trap. `research/06 §3 row 3` handles this well (it moves retention
  to the fleet/routing level). My F3 is a precision fix to the SKILL.md summary, **not**
  an overturn.

- **"Onboarding → zero" is correct for the main agent** and the skill already carves out
  the human operator's real first-run (Prime Directive 3; `06 §3 row 2`: "The human
  operator still has a first run (secondary)"). No change needed; the context-budget
  scope correctly makes the *quality/compression of the minimal skill* the audit target,
  not its existence.

- **Do NOT add pricing/packaging depth (outcome-based / services-as-software).** This is
  the most tempting 2026 add and I reject it for ghx. The Monetization Hawk persona +
  Business Model scope already reframe correctly as **cost-to-serve per episode**, which
  is the right internal lens for a pre-monetization, founder-dogfooded OSS sidecar whose
  north star explicitly lists "a public `ghx bench` CLI / general-purpose eval framework"
  as a **non-goal**. Adding a16z's outcome-based-pricing apparatus would be answering a
  question ghx isn't asking. (One-line enrichment only — see F5.)

- **Do NOT add a Trust-&-Safety / Responsible-AI persona.** The harm surface for a code-
  recon tool is already covered by the Agent-safety/adversarial-tool-use scope (lethal
  trifecta, prompt injection, reversibility) + Trust & Verifiability + the red-team stack,
  and security proper is explicitly routed to `security-review`. A new persona here is
  dilution; the one real gap (provenance/license) belongs as a *sub-check* under an
  existing scope (F1), not a worldview.

- **Do NOT add a generic prioritization scope (RICE/WSJF/cost-of-delay).** That's
  execution hygiene, not product strategy, and JTBD/ODI opportunity-scoring + Zero-to-One
  "test the riskiest assumption first" already cover sequencing at the altitude this
  skill operates. For a solo-founder-plus-agent-fleet on a milestone ladder, RICE theater
  would be noise.

---

## Ranked findings

### F1 — Missing: provenance / license / IP lineage of the code ghx *surfaces* — MUST-FIX (as a sub-check), confidence HIGH

**Quoted target.** Tier-B scope **"Trust & verifiability of output"** (SKILL.md): *"Can a
downstream actor audit the claim? Is a confidently-wrong output structurally
distinguishable from a right one?"* And the Business Model scope, which discusses ghx's
**own** license (COSS/open-core) but never the license of what ghx **returns**.

**The gap.** ghx's job (NORTH_STAR: "sweeps the GitHub open-source world," returns
"files, commands, snippets," P3 "local clone + codemapping") is to pipe verbatim and
derived code from *arbitrary third-party repos* into the main agent's context, which then
writes code. Not one of the 17 scopes asks: *what is the license/provenance of the
snippet ghx just injected, and does the report preserve it?* "Auditability of the claim"
is scoped to *correctness*, never to *legal lineage*. This is a distinct failure mode:
a GPL/AGPL snippet surfaced without its license, adapted by the main agent, silently
creates copyleft/attribution obligations downstream — the exact "license laundering" the
2025 discourse names.

**Why it matters + source.** This is a live, litigated 2024-26 concern, not hypothetical:
*Doe v. GitHub* (filed Nov 2022, ongoing) alleges Copilot reproduces open-source code
while *stripping copyright notices, license terms, and attribution*
([case background, GitHub Copilot litigation](https://githubcopilotlitigation.com/)); GitHub
has acknowledged suggestions occasionally reproduce training-set code verbatim; and in
Sept 2025 ten open-source foundations published an open letter on AI-driven provenance
fragility ([reported here](https://developers.slashdot.org/story/25/10/26/208204/does-generative-ai-threaten-the-open-source-ecosystem)).
For a tool positioned as "the only tool an agent reaches for to explore the GitHub
open-source world," a recon layer that launders license/attribution out of what it
surfaces is a first-order **trust and business-viability** liability that scales with the
north star's "industry adopts it" end state — and it is *invisible* to every current lens.

**Recommendation.** Add one bullet to the **Trust & Verifiability** scope (and mirror it
in the Business Model viability questions): *"Does the report preserve the provenance and
license of surfaced third-party code (repo, path, commit, license), so a downstream actor
can assess copyleft/attribution obligations before adapting it? A snippet returned
without its license is a severity-3+ trust finding, not a formatting nit."* Cheap to add,
respects anti-dilution (sub-check, not persona), directly north-star-relevant
(auditability/evidence tenet).

---

### F2 — Wrong benchmark class: Evals-as-user-research cites issue-resolution, not code-localization/retrieval — SHOULD (arguably MUST for currency), confidence HIGH

**Quoted target.** Tier-B **"Evals-as-user-research"** grounding: *"Hamel evals; SWE-bench;
τ²-bench (pass^k consistency)"*; and `research/06 §4.3` / Source Ledger rows 11-12 which
cite **only** SWE-bench and τ²-bench as "the field's shared user research."

**The error.** SWE-bench measures *resolving a GitHub issue* (end-to-end patch); τ²-bench
measures *conversational policy compliance*. Neither measures ghx's actual job —
**finding, retrieving, localizing, and navigating** the right code under a token budget.
Grounding a recon tool's "user research" on issue-resolution benchmarks anchors the audit
to the wrong JTBD. The 2025-26 field has purpose-built benchmarks for exactly ghx's task:
**LocAgent** (graph-guided *code localization*, ACL 2025, arXiv 2503.09089 — file-level
localization accuracy is the headline metric); repository-retrieval/context benchmarks
(**ContextBench**, **RepoBench**); long-context code benchmarks (**LongCodeBench / LoCoBench**);
and, tellingly, **CodeSearchNet** — a code-*retrieval* benchmark co-authored by **Hamel
Husain**, whom the skill already cites for evals. The skill cites Hamel's evals blog but
not his retrieval benchmark, which is the more on-point artifact.

**Why it matters + source.** Two outcome changes. (a) An auditor would benchmark ghx
against the wrong task and miss the *direct comparables* — LocAgent/ContextBench are what
ghx's own consequence-product ("a public code-reconnaissance benchmark," NORTH_STAR) must
be positioned against; not knowing they exist is a Competitive-Analysis and Positioning
miss. (b) **LocAgent is also live external validation *and* a competitive-paranoia flag
for ghx's P4 bet**: it reports a fine-tuned Qwen-32B matching SOTA proprietary models on
localization at ~**86% lower cost** ([LocAgent, arXiv 2503.09089](https://arxiv.org/abs/2503.09089),
[repo](https://github.com/gersteinlab/LocAgent)). That is precisely the NORTH_STAR P4
thesis ("a cheap trained model beats frontier models at reconnaissance") — already
demonstrated for code localization by an outside team. An AI-PM-Pragmatist / Competitive-
Paranoid audit *must* see this: the recon-model bet is validated and contested at once.

**Recommendation.** In the Evals-as-user-research scope, replace/augment the SWE-bench
anchor with the **code-localization/retrieval/long-context** benchmark family (LocAgent,
ContextBench/RepoBench, CodeSearchNet, LoCoBench) as the *domain-correct* reference form;
keep τ²-bench for pass^k *consistency*. Add LocAgent to the Competitive-Analysis /
Industry (Wardley) evidence as a named data point on the recon-model cost curve.

---

### F3 — Over-compressed: the engagement/retention "INVERTED" verdict risks discarding the key agent-PMF signal — SHOULD, confidence MEDIUM (precision fix, not an overturn)

**Quoted target.** SKILL.md Prime Directive 2: *"Engagement/Retention as a goal (INVERTED
— … more tool-calls/time-per-task is a vanity metric pointing the wrong way; the win is a
high-signal answer and leaving fast)"*; and the anti-pattern *"session-length /
tool-calls-per-task; for a context-subtracting product these are metrics to drive down."*

**The error (of compression).** The SKILL.md operational layer collapses two very
different things into one "INVERTED." The research file gets it right —
`research/06 §3 row 3`: *"Move Adoption/Retention to the fleet/operator level (does the
agent/operator keep routing through the tool across sessions — a Balfour-style
revealed-preference curve), never per-session dwell."* That nuance does **not** survive
into SKILL.md. An auditor reading only the operational layer (as intended) could dismiss
retention wholesale — but **cross-session tool-selection retention is the single most
important agent-product signal there is**: the agent *choosing ghx again next turn* is the
revealed-preference / ablation proof of dependence, i.e. agent-native PMF. Two further
under-specified edges: "tool-calls-per-task down" is only true of the *main agent's*
calls — the **sidecar internally doing more work** (M8 eager anticipation deliberately
does *more* exploration for latency) is good; and the always-on resident-runtime vision
*wants* the sidecar continuously engaged. The blanket phrasing invites mis-application to
all three.

**Why it matters + source.** Tool-selection is now a named, hard AX problem — "tool
selection collapse" when definitions flood context ([Biilmann, Introducing AX](https://biilmann.blog/articles/introducing-ax/);
tool-bloat/selection-collapse discourse, 2025-26) — so *whether the agent keeps selecting
ghx* is exactly the metric to protect, not drive down. Mislabeling it "vanity" would cause
an audit to under-value the one number that proves the product is working.

**Recommendation.** In SKILL.md Prime Directive 2 and the anti-pattern, carry the row-3
split explicitly: *"Per-question **main-agent** engagement (dwell, main-agent tool-calls):
drive down. **Cross-session tool-selection / routing retention** (does the agent re-select
ghx over 'grep it myself'): the key agent-PMF signal — protect and measure it.
Sidecar-internal work is judged by signal-out, not minimized."* This is the one place I'd
call the agentic-first framing *incomplete as stated*, though not wrong.

---

### F4 — Under-weighted: reliability / latency as a first-class product value for an agent-dependent sidecar — SHOULD/OPTIONAL, confidence MEDIUM (my least-confident call)

**Quoted target.** Token-economics scope lists *"tool-call count per task"* and latency
only as a metric; AX covers *"recover-from"* errors. No scope treats **availability /
latency / graceful-degradation as a product promise**.

**The gap.** ghx is a *resident sidecar the main agent blocks on mid-turn* (NORTH_STAR:
"always-on runtime," "session recovery," "watchdog," "artifacts-on-failure"). For that
topology, reliability and tail latency *are* core product value: a sidecar that hangs or
dies mid-turn strands the main agent, and the "high-signal answer, leave fast" thesis is a
*latency* claim as much as a signal claim. The compounding-failure math is unforgiving —
an 85%-reliable step across 8 internal tool-calls is ~27% end-to-end
([reliability-in-production discourse, 2025](https://cleanlab.ai/ai-agents-in-production-2025/)).
τ²-bench pass^k captures *consistency*, but not availability/latency SLOs or
degradation-under-failure as a *product* attribute.

**Why it matters + source.** Production-agent practice now treats explicit SLOs (e.g.
"result within 5s, 95% of the time; degrade gracefully") as PM-owned product surface, and
Gartner's mid-2025 forecast pins >40% of agentic-AI project cancellations on cost /
unclear value / inadequate reliability controls (same source). ghx already *builds* for
this (ADR-0027 resilience) but the audit has no lens that *credits or interrogates it as
product value*.

**Recommendation.** Add a Tier-B **sub-check** (not a persona, not a full scope) under AX
or token-economics: *"Is availability/tail-latency/degradation-under-failure treated as a
product promise to the agent consumer, with explicit SLOs and artifacts-on-failure — or
only correctness/compression?"* **Caveat / why I'm least confident:** this flirts with the
SRE/engineering boundary the skill deliberately routes out of scope, and AX-recover +
token-econ-latency arguably already cover 70% of it. Keep the framing strictly *product*
(reliability as a value the consumer experiences), or drop it. I would not fight hard for
this one.

---

### F5 — Enrichment only: services-as-software cost curve as external framing for the P4 bet — OPTIONAL (lean REJECT-as-scope), confidence MEDIUM

**Quoted target.** Business Model scope: *"internally reframes as cost-to-serve/episode."*
Correct as-is.

**The (minor) enrichment.** a16z's outcome-based-pricing thesis — *"price at economically
viable levels short-term while expecting cost-to-serve to decline long-term, driving
future margin expansion"* ([a16z, Dec 2024](https://a16z.com/newsletter/december-2024-enterprise-newsletter-ai-is-driving-a-shift-towards-outcome-based-pricing/))
— is an external economic articulation of ghx's P4 bet (train a cheap recon model → cost
per episode collapses). It is a nice one-line cross-reference for the cost-to-serve
framing, nothing more.

**Recommendation.** At most, add one sentence to the Business Model scope tying the P4
cost curve to the services-as-software margin-expansion pattern. **Do not** import
pricing/packaging machinery — see "What NOT to add." Tag OPTIONAL; my honest lean is
skip-or-one-line.

---

## Optional removals / consolidations (the set may be slightly too big)

### R1 — Retire or re-scope the DevEx Advocate persona (#14) — SHOULD, confidence MEDIUM-HIGH

The skill *itself* says the AX lens (#16) *"subsumes and outranks the DevEx Advocate for
agent surfaces,"* and #14's "Applied to ghx" note reframes it as agent-facing — which is
now #16's job. As written, #14 double-covers AX and dilutes signal. Two clean options:
**(a) cut it**, folding its one distinct contribution (measurable DX / DX-Core-4
instrumentation) into AX; or **(b) re-scope #14 explicitly to the *human operator's* DevEx**
— the setup/config/docs first-run the skill says matters (Prime Directive 3, the dogfood
FRICTION setup gaps) but that *no persona currently owns*. Option (b) is my preference: it
turns a redundant lens into a load-bearing one and closes the "operator first-run" hole.

### R2 — Consolidate Tier-B scopes; 8 is more than the axes support — OPTIONAL, confidence MEDIUM

Biilmann's AX has four pillars — **Access / Context / Tools / Orchestration** — and three
separate Tier-B scopes are essentially those pillars re-listed: **Tool/affordance**
(≈ Tools), **Context-budget/progressive-disclosure** (≈ Context), **Distribution-in-agent-
ecosystems** (≈ Access). Eight parallel Tier-B scopes read as more coverage than there is
orthogonality, which is the exact signal-dilution the skill warns against elsewhere.
Recommendation: either explicitly frame #4/#5/#8 as *sub-lenses under the AX scope*, or
collapse Tier-B to ~5 genuinely orthogonal scopes (AX, Token-economics, Evals-as-research,
Trust/verifiability+provenance, Agent-safety). Low urgency; a legibility improvement.

---

## Source Ledger

| # | Source | Author / Org | URL | Verified | Supports |
|---|---|---|---|---|---|
| 1 | Context Rot: How Increasing Input Tokens Impacts LLM Performance | Kelly Hong, Anton Troynikov, Jeff Huber / Chroma (Jul 14 2025) | https://www.trychroma.com/research/context-rot | **Direct fetch** — 18 models, monotonic degradation confirmed | "What's right": empirical grounding that context subtraction improves quality |
| 2 | LocAgent: Graph-Guided LLM Agents for Code Localization | Chen et al. / ACL 2025 | https://arxiv.org/abs/2503.09089 · https://github.com/gersteinlab/LocAgent | **Verified** (arXiv + ACL Anthology + GitHub, consistent) | F2: domain-correct benchmark; P4 validation (32B ≈ SOTA at ~86% lower cost) |
| 3 | GitHub Copilot litigation (Doe v. GitHub) | Plaintiffs v. GitHub/Microsoft/OpenAI (filed Nov 2022, ongoing) | https://githubcopilotlitigation.com/ | Well-known case; verified via convergent search results | F1: provenance/attribution stripping is litigated |
| 4 | Does Generative AI Threaten the Open Source Ecosystem? (10-foundation open letter, Sept 2025) | Slashdot / OSS foundations | https://developers.slashdot.org/story/25/10/26/208204/does-generative-ai-threaten-the-open-source-ecosystem | Search snippet — **secondary**, mark corroborating | F1: provenance fragility / license-laundering discourse |
| 5 | AI License Laundering: How Code Generators Strip Open Source Obligations | pickuma / DEV Community | https://dev.to/pickuma/ai-license-laundering-how-code-generators-strip-open-source-obligations-2i0m | Search snippet — **secondary/practitioner**, corroborating only | F1: "contaminating license" mechanism |
| 6 | Introducing AX: Why Agent Experience Matters | Mathias Biilmann / Netlify (Jan 2025) | https://biilmann.blog/articles/introducing-ax/ | Already in skill's ledger; reused | F3: tool-selection as the AX metric to protect |
| 7 | AI Is Driving a Shift Towards Outcome-Based Pricing | Makarov, da Costa, Pinero / a16z (Dec 19 2024) | https://a16z.com/newsletter/december-2024-enterprise-newsletter-ai-is-driving-a-shift-towards-outcome-based-pricing/ | **Direct fetch** — quote confirmed | F5: services-as-software cost-curve framing |
| 8 | AI Agents in Production 2025 (reliability, SLOs, compounding failure, Gartner cancellation forecast) | Cleanlab | https://cleanlab.ai/ai-agents-in-production-2025/ | Search snippet — **secondary**, corroborating | F4: reliability/latency as product value |
| 9 | CodeSearchNet Challenge (code retrieval benchmark) | Husain et al. (2019) | https://arxiv.org/abs/1909.09436 | Well-known; **not re-fetched this pass** — mark for verification | F2: retrieval benchmark co-authored by the Hamel the skill already cites |

**Unverified / caution flags.** Rows 4, 5, 8 are practitioner/secondary sources cited to
corroborate a *discourse*, not as primary authorities — treat the specific figures
(GitClear cloning stats, "55K-token / tool-selection-collapse" numbers, the 0.85^8≈27%
illustration) as reported, not independently verified. Several 2026-dated arXiv IDs that
surfaced in search (ContextBench 2602.05892, LoCoBench/LongCodeBench, "99% Success
Paradox" 2605.18857) were **not** individually fetched; I cite the *benchmark family* by
name and lean on LocAgent (row 2) and CodeSearchNet (row 9) as the verified anchors. Row 9
(CodeSearchNet) I'm confident exists but did not re-fetch this pass — verify before
quoting a metric. Rows 1, 2, 7 are the load-bearing verified sources.
