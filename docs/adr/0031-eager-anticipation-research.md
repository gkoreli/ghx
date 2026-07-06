---
title: "ADR-0031: Eager Anticipation Research — Speculative Exploration Prior Art"
date: "2026-07-06"
status: "research"
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0031. Eager Anticipation Research — Speculative Exploration Prior Art

## Status

Research memo for NORTH_STAR capability **§2 (eager anticipation, configurable)**
and workstream **B9 / milestone M8**: the sidecar anticipates follow-up
questions and pre-explores so an anticipated question answers in under a
second instead of ~30. M8 presupposes the always-on daemon (ADR-0030, B6):
without a resident process there is no idle compute to spend and no owner for
speculative state. **No implementation and no decision in this ADR** — it maps
the prior-art landscape, the trigger candidates, the style ladder, the daemon
integration points, storage/invalidation, and how SAFE would measure it. A
follow-up decision ADR (0031.1) picks the design.

Method note (honesty about the evidence): web sources were read through
WebSearch snippets and WebFetch summarization, not full-page reads. Local
claims about ghx internals were verified by reading the code in this repo on
2026-07-06 (file/line citations below). Several arXiv papers cited are
2026 publications known only from their abstracts; see "Could not verify."

## The shape of the problem

Anticipation is speculative execution applied to reconnaissance. The classic
speculative-execution literature (CPUs, Hadoop, web prefetch) gives the full
cost model vocabulary and it transfers cleanly:

- **Prediction**: what follow-up will the main agent ask next?
- **Speculation depth**: warm a cache, prefetch evidence, or pre-compose the
  whole answer (the web analogy: prefetch vs prerender).
- **Commit/abort**: serve the pre-computed answer on a hit; discard on a miss.
- **Waste**: every miss burned tokens/API quota; every stale hit is worse
  than a miss (visibility tenet: a wrong instant answer is a trust failure).

Two properties make ghx unusually well-positioned relative to the generic
agent-latency papers:

1. **The prediction signal is already emitted.** Every accepted sidecar report
   carries `nextReads` ("Suggested next files or areas to read",
   `internal/sidecar/reportsink.go:247`; required schema field,
   `internal/sidecar/report.go:48,133`), and both `nextReads` and
   `uncertainty` flow into the session ledger's `OpenQuestions`
   (`internal/sidecar/ledger.go:104-112`). The sidecar is *already asked to
   anticipate* every turn; nothing consumes the signal yet.
2. **Speculative reconnaissance is side-effect-free.** ghx tiers 0/1 are
   read-only remote evidence gathering. The hardest constraint in the
   speculative-agent literature — only speculate on side-effect-free,
   idempotent edges (arXiv 2606.07846) — is satisfied by construction for
   the entire current tier range. The costs are only tokens, API rate
   limits, and staleness.

## Prior art

### Speculative execution in agent systems (papers)

- **Speculative Actions: A Lossless Framework for Faster Agentic Systems**
  (arXiv, Oct 2025) — <https://arxiv.org/abs/2510.04371>. Uses a faster
  model to predict the agent's next action and executes it in parallel;
  commits only when the prediction matches the main model's actual action.
  Reports up to 55% next-action prediction accuracy → up to 20% latency
  reduction, with a formal cost-latency analysis of speculative breadth
  (how many branches to launch) vs time saved. Lesson for ghx: even a
  *cheap* predictor with ~50% accuracy pays off when the speculated work is
  cheap relative to the latency it hides — and ghx's predictor input
  (`nextReads`, authored by the same model that will handle the follow-up)
  should beat a generic draft model at guessing its own continuation.
- **Cost-Aware Speculative Execution for LLM-Agent Workflows** (arXiv,
  2026) — <https://arxiv.org/pdf/2606.07846>. The most directly reusable
  cost-control frame. Five design decisions: (D1) when to start speculative
  work, (D2) price every speculation in real dollars at input/output rates,
  (D3) expose a **single operator dial** for latency vs cost, (D4) decide
  per-speculation via an expected-value rule with a failure-weighted cost
  term, (D5) estimate success probability with a Bayesian Beta-Binomial
  posterior keyed to a dependency-type taxonomy. Restricts speculation to
  side-effect-free/idempotent/stageable operations. The "single operator
  dial" is precisely NORTH_STAR §2's configurable style knob, discovered
  independently.
- **Act While Thinking: Pattern-Aware Speculative Tool Execution (PASTE)**
  (arXiv, 2026) — <https://arxiv.org/html/2603.18897v1> — middleware
  between the LLM agent and the tool backend that pre-executes predicted
  tool calls; **B-PASTE** (<https://arxiv.org/pdf/2604.16469>) adds
  beam-aware speculation under resource constraints; **Speculative
  Interaction Agents** (<https://arxiv.org/pdf/2605.13360>) overlap
  reasoning with tool execution and streaming user input. Collectively:
  the field's consensus placement for speculation is a *runtime layer under
  the agent*, not prompt engineering — for ghx that layer is the daemon.
- **Sherlock: Reliable and Efficient Agentic Workflow Execution**
  (arXiv 2511.00330) — <https://arxiv.org/pdf/2511.00330> — speculating
  deeper along a workflow overlaps more computation but risks more wasted
  compute on rollback; depth is a dial, not a constant.

### Prefetching heuristics (the older, quantitative literature)

- **Chrome Speculation Rules API** —
  <https://developer.mozilla.org/en-US/docs/Web/API/Speculation_Rules_API>,
  <https://developer.chrome.com/docs/web-platform/prerender-pages>. The
  most mature *shipped* anticipation system with a user-facing style knob.
  Two spend levels — **prefetch** (fetch resources, don't render) and
  **prerender** (fully render the page invisibly; navigation is instant) —
  crossed with a four-step **eagerness ladder**: `immediate` / `eager` /
  `moderate` (speculate on 200ms link hover) / `conservative` (speculate on
  pointer-down). Cheap prefetch at low confidence is *upgraded* to full
  prerender as confidence grows. Chrome documents limits (prerender caps),
  and Google Search itself ships it
  (<https://developer.chrome.com/blog/search-speculation-rules>). This is
  the closest existing design to NORTH_STAR §2's "3–5 styles from off to
  aggressive," including the key refinement: eagerness (when/how much to
  trigger) and depth (how much each speculation spends) are **two separate
  axes**.
- **Web prefetch prediction models** — Markov predictor trees over access
  logs (Springer, <https://link.springer.com/chapter/10.1007/978-3-642-01203-7_9>),
  popularity-based prediction (IEEE,
  <https://ieeexplore.ieee.org/document/1185219/>). Standard evaluation
  vocabulary: **precision** (fraction of prefetched objects actually used),
  **recall/hit rate** (fraction of requests served from prefetch), wasted
  bandwidth as the cost term. SAFE should adopt these metric names rather
  than inventing new ones. Also the standing caveat: static pattern models
  decay as usage shifts — a reason to key prediction on per-session report
  signals rather than global historical patterns first.
- **Hadoop speculative execution / straggler mitigation** — e.g.
  <https://www2.seas.gwu.edu/~tlan/papers/Sigmetrics_2017_ab.pdf>,
  energy-impact studies
  (<https://www.researchgate.net/publication/304297799_On_Understanding_the_Energy_Impact_of_Speculative_Execution_in_Hadoop>).
  Decades of evidence that unbounded speculation wastes significant
  resources on killed copies, and that the fixes are always the same:
  launch criteria (expected-benefit thresholds), caps on concurrent
  speculative tasks, and prompt cancellation when the real result lands.

### Background indexing in coding products

- **Cursor codebase indexing** —
  <https://cursor.com/blog/secure-codebase-indexing>,
  <https://docs.cursor.com/context/codebase-indexing>, analysis at
  <https://towardsdatascience.com/how-cursor-actually-indexes-your-codebase/>.
  On folder open, a background process chunks code semantically, embeds it,
  and maintains a **Merkle tree of file hashes** so periodic syncs
  (~every 5 minutes) re-embed only changed files. The expensive step
  (embedding) is done asynchronously in the background precisely so
  question-time is cheap. Cursor's **shadow workspace**
  (<https://cursor.com/blog/shadow-workspace>, third-party analyses) runs a
  hidden editor instance so background AI work gets lints/LSP feedback
  without touching the user's view — background work is a *separate
  instance*, never contention on the foreground.
- **GitHub Copilot remote repository indexing** —
  <https://docs.github.com/en/copilot/concepts/context/repository-indexing>,
  <https://github.blog/changelog/2025-03-12-instant-semantic-code-search-indexing-now-generally-available-for-github-copilot/>.
  Remote embeddings index built in the background (seconds to ~60s), then
  incrementally updated "within seconds of starting a new conversation."
  Notable for ghx: GitHub already maintains this index server-side for
  Copilot — ghx cannot call it, but it validates that *per-repo background
  index warming keyed to conversation start* is a shipped, accepted pattern.
- Both products anticipate **capability** (a warm index for any future
  question), not **specific questions**. That is exactly ghx's cheapest
  anticipation style (below): warm the machinery before predicting content.

### Prompt-cache warming

- **Anthropic prompt caching** —
  <https://platform.claude.com/docs/en/build-with-claude/prompt-caching>.
  Cache hits cut cached-input cost ~90% and reduce latency; each hit
  refreshes the TTL; short default TTL (minutes) with a paid longer-TTL
  option. Community reports describe scheduled minimal "prewarm" requests
  before traffic arrives (e.g.
  <https://dev.to/whoffagents/claudes-prompt-cache-ttl-silently-dropped-from-1-hour-to-5-minutes-heres-what-to-do-13co>
  — see "Could not verify" for the TTL-change claim itself).
- **aider `--cache-keepalive-pings N`** —
  <https://aider.chat/docs/usage/caching.html>. A shipped coding-agent
  implementation of cache warming: aider pings the provider every ~5
  minutes, up to N times after each user message, to keep the cached system
  prompt + repo map warm. Direct precedent for the daemon keeping the ACP
  agent's prompt cache warm during idle TTL — anticipation's floor tier is
  literally "spend pennies on pings so the *next* question, whatever it is,
  starts hot."

### Follow-up prediction and proactive retrieval

- **Just-in-Time Information Retrieval (JITIR)** — Rhodes & Starner's
  Remembrance Agent lineage
  (<https://faculty.cc.gatech.edu/~thad/p/032_40_agents&ubicomp/remembrance-agent.html>,
  <https://dl.acm.org/doi/10.1147/sj.393.0685>). A continuously running
  agent retrieves documents relevant to the user's *current context*
  without being asked, presented non-intrusively. The 25-year-old headline
  finding still matters for SAFE's value story: JITIR users didn't just
  retrieve faster, they **retrieved and used more information than they
  otherwise would** — anticipation changes consumption, not just latency.
- **Learning to Retrieve Engaging Follow-Up Queries** (EACL 2023,
  <https://arxiv.org/abs/2302.10978>) and **Simulating Follow-Up Questions
  in Conversational Search** (ECIR 2024,
  <https://link.springer.com/chapter/10.1007/978-3-031-56060-6_25>) —
  follow-up prediction is a studied retrieval task; trained retrieval over
  conversation history beats generation for candidate follow-ups. Also the
  standard source of *simulated follow-ups* — the technique SAFE needs to
  build multi-turn anticipation eval tasks without waiting for organic
  dogfood distributions.
- **ChatGPT Pulse** —
  <https://openai.com/index/introducing-chatgpt-pulse/>. Shipped proactive
  research: overnight asynchronous research from memory + chat history +
  feedback, delivered as scannable summaries; user curates what gets
  researched. Precedent for the aggressive end of the ladder (unprompted
  full research runs) and for feedback-driven trigger curation.
- **LangChain "ambient agents"** —
  <https://www.langchain.com/blog/introducing-ambient-agents>,
  <https://www.blog.langchain.com/ux-for-agents-part-2-ambient/>. Framing:
  agents listening to event streams and acting in the background scale past
  chat UX; the design question is when they surface to the human. For ghx
  the "event stream" is the session's own turn stream, and surfacing is
  solved by the report/artifact contract — anticipated work is visible in
  `~/.ghx`, never pushed into the main agent's context uninvited.

### Storage and matching for anticipated answers

- **GPTCache / semantic caching** —
  <https://github.com/zilliztech/gptcache>,
  <https://arxiv.org/html/2411.05276v1>. The standard pattern for "did we
  already answer this question": embed the incoming query, nearest-neighbor
  against cached question embeddings, serve on similarity above a threshold
  (~0.8 typical), TTL for freshness. Known failure modes documented in the
  field (e.g.
  <https://tianpan.co/blog/2026-04-20-cache-invalidation-ai-semantic-rag>):
  fixed thresholds serve *semantically wrong* near-misses; cached wrong
  answers get amplified to every similar future query; changing the
  embedding model invalidates all stored distances. Any ghx design must
  treat similarity-matching as a **candidate gate, not an accept gate** —
  provenance-checked freshness (below) decides whether to serve.

### What does not exist

GitHub repo search (2026-07-06; `gh search repos` for "speculative execution
agent LLM", "speculative tool", "anticipatory agent", "prefetch agent
anticipate") found **no mature open-source framework** shipping
anticipatory exploration — only single-digit-star research stubs (e.g.
`joelvarun/speculative-tools`). The prior art is papers plus closed product
features (Cursor, Copilot, Pulse). Per the open-source-leverage tenet: there
is nothing to adopt wholesale here; the adoptable pieces are the *patterns*
(eagerness ladder, semantic cache, Merkle-style change detection, keepalive
warming) — the assembly would be ghx's own.

## Candidate anticipation triggers

Ordered by evidence quality and implementation cost:

1. **Report `nextReads` (the built-in seed).** Already emitted on every
   accepted report and ledgered into `OpenQuestions`
   (`internal/sidecar/ledger.go:110`). Authored by the model that just
   explored — the same-model self-prediction the Speculative Actions paper
   approximates with draft models, for free. Weakness: `nextReads` names
   files/areas, not questions; it predicts *where to look next*, not *what
   will be asked* — which suits evidence-prefetch tiers better than
   full-answer tiers.
2. **Report `uncertainty` entries.** Also ledgered into `OpenQuestions`
   (`ledger.go:107-109`). These are literally unanswered questions — the
   natural prompt for a speculative wrap-up turn ("resolve your own listed
   uncertainty"). Higher yield per item than nextReads but higher spend
   (needs a model turn, not just a fetch).
3. **Session ledger topic trajectory.** `RelevantFiles`, `GrepPatterns`,
   `MappedGlobs`, `InspectedPaths` across turns sketch the exploration
   frontier; the boundary of inspected vs referenced-but-uninspected paths
   is a mechanical prefetch list (no LLM needed to compute it).
4. **LLM follow-up generation at turn end.** Ask a cheap model (or the same
   warm ACP session as a zero-marginal-cost trailer): "given this Q/A and
   ledger, the 3 most likely follow-ups." The conversational-search
   literature (2302.10978) says retrieval beats generation when history
   exists — but ghx sessions are short, so generation from the report is
   the realistic v1; question-embedding retrieval over *past sessions'*
   question logs becomes viable as `~/.ghx` accumulates.
5. **Question-type templates.** The NORTH_STAR §2 example (competitor
   landscape → cross-refs, stars, activity, maturity per candidate) is a
   *fan-out pattern*: discovery answers predictably generate one follow-up
   per named candidate. A handful of templates (per-candidate deep-dive,
   "how does X work" after "where is X", "show me the tests" after any
   implementation answer) may capture much of the mass before any
   embeddings exist. Evidence for which templates: mine the dogfood/eval
   multi-turn traces already in `~/.ghx` (same method as ADR-0028's miners).
6. **Question embeddings** (deferred candidate). Needed for *serving*
   (matching an incoming question to a stored anticipated answer) more than
   for *triggering*. Costs a new dependency (embedding model, local or
   API); GPTCache's failure modes apply. An alternative worth weighing in
   0031.1: let the warm daemon do LLM-based matching ("is this incoming
   question answered by one of these 3 stored reports? yes/no") — slower
   than vector lookup but no new infra and no fixed-threshold wrongness.

## The style ladder (3–5 configurable styles)

Synthesizing Chrome's eagerness ladder with the prefetch/prerender depth
split, the cost-aware-speculation dial, and what each level spends. Names
are placeholders for 0031.1:

| Style | What runs | What it spends | Prior-art anchor |
|---|---|---|---|
| **off** | Nothing. Answer precisely, do not anticipate. | Zero. | NORTH_STAR §2's floor; must remain a hard guarantee for eval runs (no contamination of measured turns) |
| **warm** | Machinery only, no content prediction: ACP worker stays alive through idle TTL (already ADR-0030 D4), prompt-cache keepalive pings during TTL. | Pennies: ping requests at cached-input rates; daemon RSS. No exploration tokens. | aider `--cache-keepalive-pings`; Anthropic prewarm; Copilot/Cursor "warm index, unknown question" |
| **prefetch** | Deterministic evidence prefetch, no model turns: run ghx CLI reads/maps for `nextReads` paths + ledger frontier; stage outputs in the session dir as evidence bundles the *next real turn* consumes instantly. | GitHub API quota + disk; **zero LLM tokens**. | Chrome `prefetch`; web-prefetch precision/recall economics |
| **eager** | Full anticipated turns for top-k (k≈1–3) predicted follow-up *questions*, run as low-priority speculative asks in idle pool time; stored as anticipated reports. | Real model spend per speculation: roughly one sidecar turn each (the current ~30s answers cost); bounded by a per-session speculation budget. | Chrome `prerender`; Speculative Actions; expected-value launch rule from 2606.07846 |
| **aggressive** | Multi-branch: wider k, fan-out templates (per-candidate deep-dives after discovery answers), speculation on speculation (follow-ups of anticipated answers, depth ≤ 2). | Multiples of a turn per user turn; must be dollar-capped per session/day, and is where D5-style success-probability estimation stops being optional. | ChatGPT Pulse; Sherlock's depth-vs-rollback tradeoff |

Two structural notes the prior art forces:

- **Eagerness and depth are separable axes.** Chrome ships them separately
  and upgrades prefetch→prerender as confidence grows. A ghx equivalent:
  `prefetch` evidence for all of `nextReads`, then *upgrade* to a full
  anticipated turn only for the single highest-confidence follow-up.
- **Scope layering is already specified.** NORTH_STAR §3: anticipation
  level is settable globally / per project / per session / per question,
  with smart defaults — same resolution ladder the config system already
  uses; the daemon's config digest (ADR-0030 D6) must cover it.

## Daemon integration points (ADR-0030 as substrate)

- **Idle pool time is the compute source.** ADR-0030 D4 keeps an
  `AgentWorker` warm per session for a ~30min idle TTL. That idle window is
  exactly when speculation runs: the worker, its live ACP session, and its
  prompt cache are paid for and idle. `warm` style is almost free-riding on
  D4; `eager` style queues speculative `AskRequest`s onto the same worker.
- **Foreground always preempts.** Workers serialize turns per session
  (D4), so a speculative turn in flight *blocks* a real ask on that session
  — the one place anticipation could make latency worse. The design space:
  cancel the speculative turn (ACP supports cancellation; Hadoop lesson:
  cancel promptly, account the waste), or let it wrap up under a short
  deadline (ADR-0027 wrap-up semantics already exist for exactly this
  "stop now, report what you have" move). 0031.1 must decide; SAFE must
  gate on "anticipation never degrades foreground p95."
- **The daemon owns the budget ledger.** Per D5, callers send questions;
  the daemon owns runtime state. Speculation budgets (per session/day,
  token- or dollar-denominated per 2606.07846 D2) and the
  expected-value launch decision live in the daemon, configured through the
  same scope ladder as everything else.
- **Speculative turns are ordinary, marked turns.** Same `Ask` path, same
  artifacts (ADR-0022 shape unchanged), plus attributes in the ADR-0030 D7
  namespace, e.g. `ghx.sidecar.anticipation.trigger`
  (nextReads/uncertainty/template/llm), `.style`, `.speculative=true`, and
  on serve: `.hit=true`, `.served_from`, `.age_ms`. Full visibility (§3):
  "every anticipated question" is an explicit capability promise.
- **Concurrency bound is shared.** Speculative asks count against (or get a
  reserved slice of) the daemon-level max concurrency; they must never
  starve a different session's real ask.
- **Sequencing dependency:** none of this starts before B6 ships; B7
  session routing shares the "match incoming question to existing state"
  machinery an anticipation index needs — 0031.1 should check whether one
  resolver serves both.

## Storage, serving, invalidation (`~/.ghx`)

**Storage.** Anticipated answers are reports — same schema, same
`reports/` directory in the session (ADR-0022), flagged speculative in the
report metadata and *excluded from ledger derivation until served* (a wrong
speculative answer must not pollute `OpenQuestions`/`RelevantFiles` for real
turns). A small per-session index (anticipated question text, optional
embedding, trigger, creation turn, provenance) makes lookup cheap without
scanning reports.

**Serving.** On an incoming ask, the daemon checks the anticipation index
before dispatching a full turn: candidate match (embedding similarity or
LLM check, above) → freshness check → serve stored report in <1s with
provenance visible ("anticipated at turn N, evidence as of commit X"), else
fall through to a normal turn — ideally *seeded* with the anticipated
report's evidence so even a miss-by-staleness pays back partially.

**Invalidation.** Three mechanisms from the prior art, cheapest first:

1. **Provenance pinning (Cursor's Merkle move, adapted to remote).** Record
   the target repo's HEAD SHA (one cheap API call) when speculative
   evidence is gathered; on serve, re-check HEAD. Unchanged → evidence is
   provably fresh, serve instantly. Changed → either serve marked stale
   ("as of <sha>, repo has moved") or demote to seed material. Finer
   grain later: per-file blob SHAs for the files the report actually cites
   (`relevantFiles` gives the list).
2. **TTL** as the backstop for non-repo-pinned content (discovery-tier
   landscape answers: stars, recent activity — the NORTH_STAR §2 example
   is the *most* staleness-prone kind).
3. **Config/model digest**: persona revision (ADR-0029 lineage), model
   change, or ghx version change invalidates stored anticipated answers,
   mirroring the daemon's own config-digest staleness rule (ADR-0030 D6)
   and the GPTCache embedding-model-change failure mode.

The semantic-cache literature's sharpest warning applies with extra force
here: a served-stale answer *looks* like the product working (instant,
confident) while being wrong. Freshness must gate serving; similarity alone
never suffices.

## How SAFE measures anticipation

Metric names from the web-prefetch literature, adapted:

- **Hit rate (recall):** fraction of real follow-up questions served from
  anticipated state (full-report hits and evidence-prefetch partial hits
  counted separately).
- **Precision / waste:** fraction of speculative spend (tokens, API calls,
  turns) that was never used before invalidation. The citable cost claim is
  **cost per useful hit** = speculative spend ÷ hits.
- **Latency:** served-hit latency (target <1s, p95) vs the measured
  cold-turn baseline; *and* foreground-degradation guard: p95 of real asks
  with anticipation on vs off must not regress (the preemption gate).
- **Staleness correctness:** zero served answers whose pinned provenance
  was invalid at serve time; count of correctly-demoted stale entries.
- **Quality parity:** served anticipated answers scored by the same
  gates/judge as fresh answers (the ADR-0023.1 judge, once calibrated,
  compares anticipated vs fresh answers to the same question directly).

Task design: SAFE needs **multi-turn tasks with scripted follow-ups** —
turn 2 pre-registered but hidden from turn 1 (train/test discipline: the
sidecar must not see the follow-up before predicting). The
conversational-search simulation literature (ECIR 2024) is the recipe for
generating realistic follow-up distributions; dogfood session logs in
`~/.ghx` provide the organic distribution to validate the simulated one.
Per-style measurement: the whole ladder off→aggressive over the same task
set yields a hit-rate/cost frontier — the evidence for choosing smart
defaults rather than guessing them. Existing plumbing helps: eval episodes
and production sessions share the trace/artifact stack (M6), so anticipation
attributes land in traces SAFE already parses; and eval runs default to
style=off so measured turns stay uncontaminated.

## Sharpest transferable findings

1. **The seed signal is already flowing.** Every report emits `nextReads`
   and `uncertainty`, both already ledgered as `OpenQuestions` — M8's
   trigger input has been accumulating in every session since ADR-0021/0022,
   and nothing reads it yet.
2. **Eagerness and depth are two dials, not one.** Chrome's shipped design
   (prefetch vs prerender × 4 eagerness levels, cheap→deep upgrades as
   confidence grows) maps directly onto ghx's ladder and gives the styles
   non-arbitrary semantics; the cost-aware-speculation paper independently
   converges on "one operator dial over an expected-value rule."
3. **Speculation is safe here by construction, so the whole problem is
   economics + staleness.** Tier 0/1 recon is side-effect-free (the
   literature's hardest precondition, satisfied); what remains is the
   expected-value launch rule, prompt cancellation on preemption, budget
   caps, and provenance-pinned freshness — all with mature prior art.

## Could not verify

- **2026 arXiv papers** (2603.18897 PASTE, 2604.16469 B-PASTE, 2605.13360
  Speculative Interaction Agents, 2606.07846 Cost-Aware) are cited from
  abstracts/search snippets only; their evaluation quality and whether
  results replicate were not assessed. 2606.07846's comparative claims
  against other systems are the authors', unverified.
- **Speculative Actions' numbers** (55% prediction accuracy, 20% latency
  reduction) are from the abstract; domain transfer to code recon untested.
- **Cursor shadow workspace / indexing internals** come from Cursor's blog
  plus third-party analyses; the Merkle-tree and 5-minute-sync details were
  not verified against Cursor source (closed).
- **ChatGPT Pulse internals** (how triggers are ranked, budget mechanics)
  are not public; only the product behavior is documented.
- **The claimed Anthropic cache-TTL change** (1h→5min default, March 2026,
  per dev.to) was not verified against an official changelog; the memo
  relies only on the uncontested facts (short default TTL, hits refresh it,
  paid longer TTL exists, prewarming is documented practice).
- **GitHub repo search coverage**: absence of mature OSS anticipation
  frameworks is bounded by the four query phrasings listed above; a
  differently-named project could exist.
- All web sources were read via WebFetch/WebSearch summarization, not
  full-page reads (same method note as ADR-0028).

## Cross-references

- NORTH_STAR §2 (eager anticipation), §3 (visibility/control scopes), §4 +
  B6/B7 (resident runtime this presupposes), B9/M8 rows.
- ADR-0030 — always-on daemon: idle worker TTL (D4) is the compute slot;
  session registry (D5) is where routing/matching lives; config digest
  (D6) is the invalidation precedent; attribute namespace (D7).
- ADR-0027 — wrap-up semantics reusable for preempted speculative turns.
- ADR-0021/0022 — report contract (`nextReads`) and `~/.ghx` artifact
  shape that anticipated reports must not fork.
- ADR-0023.1 — the calibrated judge is how anticipated-answer quality
  parity gets scored.
- ADR-0019.1 — discovery tier: the most staleness-sensitive anticipation
  target (the NORTH_STAR §2 competitor-landscape example).
- `internal/sidecar/report.go:48`, `reportsink.go:247`,
  `ledger.go:104-112` — the existing trigger-signal plumbing.

## Provenance

Researched and drafted by a Fable worker (text-only web research + local
code reading) on 2026-07-06; no implementation, no decision. Next step:
ADR-0031.1 decision memo choosing trigger set, style semantics, and the
serving/invalidation design, pre-registered before build per AGENTS.md.
