---
title: "ADR-0032: Combined-Objective Host-Task Evals — Research"
date: "2026-07-06"
status: "research"
parent: ADR-0016
thread: "sidecar-agentic-eval"
author: "Goga Koreli"
---

# 0032. Combined-Objective Host-Task Evals — Research

## Status

Research memo. **Design-explores, does not decide** — no gates, no
thresholds, no corpus is registered here; a follow-up decision ADR
(0032.1) must pre-register those before any run is scored. This memo
fills the slot ADR-0016.6 reserved: *"host-task fixtures with their own
ground truth — a new eval class, its own ADR when designed."* It is the
founder's standing ask: measure **both agents** — a main agent doing real
engineering with the sidecar available vs without — proving the
NORTH_STAR "Why" rebuttal.

## The claim, made falsifiable

NORTH_STAR "Why" (the rebuttal to "my main agent is already good at code
exploration"):

> By deferring exploration to ghx … the main agent gets measurably better
> at *its own objective* **and** receives better exploration answers than
> it would have produced itself. Both, not one.

Today's SAFE suite measures only the second half, in isolation: a
reconnaissance question is the whole task, and the three profiles
(`plain`/`ghx`/`ghx-sidecar`, `internal/sidecar/evals/profiles.go`)
compare *explorers*. Nothing measures what exploration dead weight does
to an agent whose objective is engineering. The combined claim decomposes
into two falsifiable halves per episode class:

- **H1 (host objective):** on real engineering tasks with exploration
  subneeds, the sidecar arm's host agent completes the engineering
  objective at least as well (task success) while spending materially
  less of its context on exploration (dead-weight fraction, per
  ADR-0016.6) and surviving longer (session longevity, context churn).
- **H2 (exploration quality):** the exploration answers the host actually
  received in the sidecar arm are at least as good as what the control
  host produced for itself mid-task — scored by the same recon
  machinery: deterministic checks where sub-question ground truth is
  pre-registrable, calibrated judge (ADR-0023.1) where it is not.

The combined verdict, whenever the decision ADR registers it, must
require **both** halves — a token win with worse engineering outcomes is
NOT SUPPORTED, and so is better recon answers that don't help the host.
That conjunction is the whole point of the eval class; a single blended
score would let one half hide the other (the exact failure mode
ADR-0016.2's verdict footnote guards against for G3-alone).

## Episode anatomy: two arms, one host harness

An episode is a **host agent** (a real coding agent — the same
ACP-driven subject the recon suite uses, but with a writable workspace)
executing one engineering task in a locally provisioned, SHA-pinned
repository, graded by tests. Two arms:

| Arm | Host toolset | What it models |
|-----|-------------|----------------|
| **control** | Host's own exploration means: shell, `gh`, web-off; plus its normal engineering tools (read/edit/test) | Today's status quo: the main agent explores for itself, exploration output floods its context |
| **sidecar** | Same engineering tools; exploration via **one recon MCP tool** (`ghx` sidecar per ADR-0019/0020.1) with the concise recon skill; direct exploration of the *external* world discouraged by the prompt contract | The Agent Sidecar Framework: exploration deferred, only compact evidence reports enter host context |

Design notes the decision ADR must settle, with the memo's current lean:

1. **Two arms, not three.** The recon suite needed `ghx` (direct) to
   isolate the tool's value from the boundary's value. Here the question
   is the *boundary*: control should be the strongest honest baseline —
   the host's native exploration habits, which for Claude-class agents
   means shell + `gh` + local grep. A `ghx`-direct third arm doubles cost
   for a question ADR-0016.1 already answered (G3: direct ghx floods
   context 33×). Revisit only if the control's exploration is so weak the
   comparison looks like a strawman.
2. **The engineering/exploration boundary must be structural, not
   judged.** The host must read and edit its *local* checkout freely in
   both arms — that is engineering, not exploration. "Exploration" for
   this eval class is **reconnaissance of the world outside the local
   checkout**: how an upstream dependency behaves, where an API moved,
   what a sibling repo does, which library implements X. This keeps the
   token attribution deterministic (below) and matches ghx's actual
   product surface (remote-first, NORTH_STAR tenet). Tasks whose only
   "exploration" is reading the local repo are out of scope — the
   sidecar has nothing to offer there and the eval would measure noise.
3. **The sidecar arm does not have exploration *removed*, it has it
   *deferred*.** Hard-blocking `gh`/network in the sidecar arm would
   manufacture failures the product never causes (echo of ADR-0016.1's
   strawman-avoidance in the permission model). The prompt contract says
   "use the recon tool for anything outside this checkout"; a compliance
   detector (the `afd6e99` pattern, declarative) flags episodes where the
   sidecar-arm host explored externally by hand — flagged and reported,
   like the plain-profile ghx-compliance violations today.
4. **Safety gate inversion.** The recon suite's G5 (no writes, enforced
   by `evals/client.go` rejecting write kinds) is *inverted* for the
   host: it must edit files and run tests in its workspace. The eval
   needs a third ACP client policy — workspace-scoped writes allowed,
   everything outside the sandbox denied. The **sidecar inside arm B
   keeps its read-only contract unchanged**; a sidecar write attempt
   remains a run-invalidating violation.
5. **Ref skew is a new structural hazard.** The host workspace is pinned
   to a SHA (it must be, for the grader); ghx explores the live default
   branch (ADR-0016.2 deliberately rejected fixture pinning for recon).
   If the task repo's own upstream is also the exploration target, the
   sidecar can truthfully describe HEAD while the host lives at an older
   pin — wrong-for-the-workspace answers that are right-for-the-world.
   Options: (a) choose tasks whose exploration targets are *other* repos
   at versions matching the pinned dependency manifest; (b) teach the
   recon question contract to carry a ref; (c) accept and measure the
   skew. Lean: (a) for the first corpus, (b) as a product feature the
   eval will eventually justify on its own evidence.

## Host task corpus

Requirements for one task fixture:

- **Small real engineering task** in a pinned public repo: a failing
  test to fix, or a small feature with a pre-written hidden acceptance
  test (SWE-bench's fail-to-pass + pass-to-pass invariants — the test
  patch is withheld from the agent, applied only by the grader).
- **A genuine external-exploration subneed**: the correct fix requires
  understanding something outside the checkout — upstream dependency
  behavior, an API that moved between versions, a protocol/spec detail
  living in another repo. This is the property no off-the-shelf corpus
  selects for and the reason the corpus is mostly hand-authored.
- **Deterministic verifiable outcome**: pinned SHA + pinned dependency
  environment (containerized, the SWE-bench harness pattern) + a test
  command whose pass/fail is the grader. No LLM in the outcome grade.
- **Pre-registered exploration sub-questions** (for H2's deterministic
  half): "what does the correct exploration establish" — 1–3 facts with
  ground truth in the style of today's task checks, verified against the
  pinned dependency version at authoring time.

Task families worth authoring first, in leverage order:

1. **Dependency-behavior bugfix** — a failing test whose fix requires
   knowing how the pinned upstream library actually behaves (e.g. an
   edge case in a middleware chain, a streaming API's event order). The
   exploration subneed is structural: the answer is in another repo.
2. **API-migration slice** — bump one dependency across a breaking
   change; acceptance test pins the new behavior. Exploration = "what
   changed and where did it move," precisely ghx's home turf; the
   changelog/release-notes trail gives clean sub-question ground truth.
3. **Conformance feature** — implement a small feature against an
   external spec or reference implementation (e.g. match how repo X
   canonicalizes Y); hidden acceptance test encodes the reference
   behavior.

Corpus size honesty: 6–10 tasks × 2 arms × 3–5 trials = 36–100 episodes
per full run. Given per-episode cost (below), the first registered corpus
should be **6 tasks**, mirroring the recon suite's minimum, with the
same ≥ 3 repos / multi-family spread discipline.

### Considered and rejected corpus candidates

- **SWE-bench Verified as-is** — the memorization evidence is
  disqualifying for a two-arm comparison: SOTA models identify buggy
  file paths from the issue text alone at up to 76% on SWE-bench repos
  vs 53% elsewhere ([SWE-Bench Illusion](https://arxiv.org/abs/2506.12286)),
  and >94% of original instances predate major training cutoffs. A host
  that remembers the fix has no exploration subneed, so both arms
  converge and the eval measures nothing. Also: its tasks rarely require
  *external* exploration — the bug and fix live in one repo.
- **SWE-bench-Live as-is** ([microsoft/SWE-bench-Live](https://github.com/microsoft/SWE-bench-Live),
  post-2024 issues, monthly refresh) — the freshness pipeline is the
  right *pattern* to steal for anti-memorization, but its tasks still
  select for single-repo issue resolution, not external-exploration
  subneeds. Use its curation ideas (recency filter, automated env
  build), not its instances.
- **Aider polyglot / Exercism** ([Aider-AI/polyglot-benchmark](https://github.com/Aider-AI/polyglot-benchmark))
  — self-contained algorithmic exercises; zero exploration subneed by
  construction. Rejected outright for this class (fine benchmark, wrong
  question).
- **Terminal-Bench** ([arXiv:2601.11868](https://arxiv.org/abs/2601.11868))
  — ops/CLI tasks in containers; harness patterns (containerized
  verification, neutral scaffold) are directly stealable, but the tasks
  are terminal proficiency, not engineering-with-reconnaissance.
- **Tasks inside the ghx repo itself** — ADR-0016.8 D2's contamination
  precedent (the repo's own ADRs leak pre-registered answers) applies
  with double force when the subject repo documents the eval that is
  measuring it. Rejected.
- **Large/famous repos (django, requests, express)** — maximal
  memorization surface, heavy environments, long test suites that
  dominate wall time. Prefer mid-size, actively moving, less-benchmarked
  repos; the recon corpus's repos are acceptable exploration *targets*
  but should not be the host *workspace* where they are benchmark
  celebrities.
- **LLM-generated synthetic tasks** (SWE-smith/SWE-Factory style
  generation) — attractive for scale, but synthetic bugs rarely carry a
  genuine external-exploration subneed, and a generator inside the
  measurement stack imports the same nondeterminism the gates exist to
  exclude. Revisit for training data (M9), not for the verdict corpus.

## What is measured on the host objective

All computable from artifacts already shaped like ours (episode JSON +
OTel traces per ADR-0022), no LLM in the loop except where marked judge:

| Metric | Definition (sketch) | Powers |
|--------|--------------------|--------|
| **Task success** | fail-to-pass tests pass AND pass-to-pass tests still pass, in the pinned container; pass@1 per cell, pass^k reported for reliability framing ([τ-bench's pass^k](https://arxiv.org/abs/2406.12045)) | H1 primary outcome (non-inferiority, see below) |
| **Dead-weight fraction** | share of host context occupied by exploration artifacts: exploration tool outputs + exploration skill/doctrine preamble + exploration-classified turns ÷ total host context (ADR-0016.6's reserved notion, now operationalized by the structural boundary above) | H1 efficiency |
| **Exploration vs engineering tokens** | token attribution by tool-call classification: recon MCP calls and external `gh`/network calls = exploration; workspace read/edit/test = engineering; prefer real `gen_ai.usage.*` counts (0016.6 upgrade path), chars/4 fallback | H1 efficiency |
| **Context churn / session longevity** | context high-water mark; number of compactions or forced session restarts; turns and engineering actions completed before exhaustion | H1, the "coding sessions live longer" claim |
| **Wall time** | episode duration; excluded from claims when `ghx.eval.parallel=true` (ADR-0025 D3 rule reused verbatim) | Informational |
| **Host-task SPT** | host signal ÷ (host-agent tokens/1000), where host signal = task success (graded: fail-to-pass fraction × pass-to-pass preservation) — the engineering analogue of 0016.6's signal = correctness × evidence | H1 headline |
| **Whole-workflow SPT** | host signal ÷ (host tokens + sidecar-internal tokens)/1000 — no hiding cost in the sidecar, exactly 0016.6's third level | Honesty check |
| **Exploration answer quality (H2)** | per pre-registered sub-question: deterministic checks against ground truth over what actually entered host context (arm A: the host's own exploration conclusions; arm B: the sidecar reports) + judge scoring via the profile-blind bundle machinery (0023.1 D2) | H2 |

**How exploration SPT composes with engineering SPT.** ADR-0016.6's
three levels carry over with one addition, not a redefinition. Within an
episode: exploration SPT is computable exactly as today over the
exploration slice (signal = sub-question correctness × evidence, tokens
= exploration-attributed tokens); host-task SPT is the new level (host
signal over all host tokens); whole-workflow SPT is the umbrella that
prevents either from lying (a sidecar that inflates its internal spend
to polish reports gets caught there). The combined-claim reading is a
**vector, never a blend**: report (host-task SPT, exploration SPT,
workflow SPT) side by side per arm. Pre-registering a scalar
combination is explicitly rejected — any weighting of engineering
against exploration signal is a product judgment that would be tuned to
the result.

**Statistical honesty — the power problem (read this before registering
gates).** Task success is binary and host-model variance is large.
Detecting a *superiority* difference of 10–20pp in pass rate at
conventional power needs on the order of 90–400 episodes per arm —
unaffordable (cost section). The claim the NORTH_STAR actually makes is
survivable at honest sample sizes if registered correctly:

- **Success: non-inferiority**, not superiority — the sidecar arm's pass
  rate is within a pre-registered margin of control, on paired per-task
  comparisons (same task, both arms — pairing removes task difficulty
  variance, the largest component).
- **Efficiency: superiority** on the continuous metrics (dead-weight
  fraction, exploration tokens, churn, host-task SPT), which are
  well-powered at 5 trials per cell — the recon suite's SPT gaps were
  ~19–24×, and effects of that size don't need large n.
- The seductive failure mode to pre-commit against: quoting an
  efficiency win *as if it were* a success win. The verdict template
  must render the two conclusions separately, in those words.

If early runs show a *directional* success advantage, that becomes its
own pre-registered superiority follow-up — never a post-hoc upgrade of
a non-inferiority result.

## Validity traps unique to two-agent episodes

Beyond everything ADR-0016.2/0016.8 already handles (leakage, junk
evidence, sufficiency, contamination paths — all reused):

1. **Host model variance dominating** — the trap above. Also: both arms
   must run the *identical* host harness, adapter version, model ID, and
   system prompt except for the exploration clause; harness deltas alone
   are worth 10–20pp on SWE-bench-class tasks
   ([Terminal-Bench](https://arxiv.org/abs/2601.11868) comparisons;
   [mini-swe-agent](https://github.com/SWE-agent/mini-swe-agent) shows a
   bash-only scaffold reaching >74% SWE-bench Verified — scaffolding is
   a first-order variable). Frozen-identity manifest (ADR-0025 D2's hash
   set) extends to: host prompt hash, arm-B skill hash, MCP tool schema
   hash.
2. **Shared subscription rate limits** — arm B runs two agents on one
   Claude subscription; rate-limit stalls hit it asymmetrically,
   corrupting wall time and possibly aborting episodes. Reuse the
   `eval_parallel_rate_limited` anomaly pattern; exclude wall-time
   claims whenever the anomaly fires; consider distinct credentials per
   agent role before the first registered run. A sidecar-arm failure
   caused by rate limiting is an *anomaly*, never scored as an
   architecture loss (BLOCKED taxonomy extension: `host_rate_limited`,
   `recon_tool_unavailable`).
3. **Task memorization** — the SWE-Bench Illusion applies to us the
   moment we pin popular repos. Mitigations, in strength order:
   hand-authored tasks (never upstreamed as issues), post-cutoff
   selection (SWE-bench-Live's recency filter), and a **memorization
   canary** per task: at authoring time, ask the subject model for the
   fix with *no repository access*; a task whose fix is produced from
   the issue text alone is disqualified (cheap, pre-registrable,
   deterministic to run).
4. **Ref skew** (described above) — sidecar answers about live HEAD vs a
   pinned workspace. First corpus dodges it by construction; the trap
   must still be detected (dependency-manifest version vs sidecar-cited
   ref recorded per episode) because a drifted exploration target
   invalidates the H2 ground truth the same way fixture rot does
   (ADR-0016.2 #5).
5. **Cross-arm session leakage** — `~/.ghx` persists sidecar sessions
   (ADR-0022); a warm session from a previous trial answering instantly
   would flatter arm B (or leak another trial's findings). Episodes get
   fresh session namespaces; the ADR-0016.5 decontamination rules extend
   to the sidecar's session store. Symmetrically, the *host* workspace
   must be re-provisioned per trial (no leftover edits).
6. **Exploration-classification gaming** — dead-weight fraction depends
   on the exploration/engineering classifier; if it is heuristic, an arm
   can look better by misclassification. Keep it structural (tool
   identity + path scope), unit-tested, and frozen with the rest of the
   measurement stack; publish per-episode attribution tables so a human
   can recompute (visibility tenet).
7. **Grader flakiness** — flaky tests convert into arm noise and, worse,
   into asymmetric noise if one arm's timing perturbs them. Container
   pinning (SWE-bench harness), 2× grader re-run on failure with
   any flip disqualifying the *task* (not the episode), and
   pass-to-pass sets kept small and deterministic.

## SAFE machinery: reused vs genuinely new

**Reused as-is or with small extension** (the episode/anomaly/artifact
spine survives intact):

- Episode schema + artifact store (`episode.go`, `store.go`):
  `TurnRecord.ToolOutputChars`, `MainAgentChars` /
  `SidecarInternalChars` / `TotalWorkflowChars` are exactly the
  accounting the host arms need; add host-outcome fields (test results,
  workspace diff stats) rather than a new schema.
- Anomaly taxonomy (`anomalies.go`) — declarative substring detectors
  were built to widen (ADR-0016.8 D2); the new anomalies above are rows,
  not architecture.
- OTel emission + shared substrate (ADR-0022, `telemetry/`) — host
  episodes are sessions like any other; grader results ride
  `gen_ai.evaluation.result` events like rewards do.
- SPT computation (`spt.go`) — parameterize signal; the three-level
  table gains a fourth row, the aggregation and invalid-episode
  filtering carry over.
- Sequential stopping (`stopping.go`, ADR-0025 D1) — bounds math is
  gate-generic; new gates plug in when registered. Bounded parallelism
  (D3) applies, with the rate-limit caveat sharpened (trap 2).
- Judge machinery (0023.1: bundle builder, k=3 runner, disagreement
  report) — H2's judge half reuses it; profile-blindness now also means
  *arm*-blindness (the bundle must not reveal whether exploration came
  from a sidecar report or the host's own narration — a real design
  problem, since report JSON is recognizably shaped; likely requires a
  normalizing bundle transform, flagged for 0032.1).
- Validity discipline wholesale: pre-registration, frozen measurement
  stack, PRELIMINARY self-labeling, sufficiency counters (ADR-0016.2,
  0016.8 D3–D6), contamination paths (D2).

**Genuinely new** (the actual build cost of this eval class):

1. **Workspace provisioner** — clone at pinned SHA, apply env pinning,
   containerized test execution, per-trial reset. This is the SWE-bench
   harness pattern ([swebench.com harness docs](https://www.swebench.com/SWE-bench/reference/harness/))
   and the open-source-leverage tenet says steal it, not rebuild it —
   candidate: drive their container conventions or Terminal-Bench-style
   task containers rather than inventing a format.
2. **Host ACP client policy** — workspace-scoped write allowance
   (safety inversion above); today's eval clients only know deny-writes.
3. **Outcome grader** — fail-to-pass/pass-to-pass runner + result
   parser, deterministic, inside the frozen stack.
4. **Token attribution layer** — exploration/engineering classification
   over tool calls + real usage counts (the 0016.6 upgrade path becomes
   load-bearing here; chars/4 is too blunt to carry a headline claim
   about token *splits*).
5. **Recon MCP arm wiring** — the eval has never driven the sidecar *as
   a tool inside another agent*; arm B is the first time SAFE exercises
   the ADR-0019/0020.1 consumption surface end-to-end. (That is also a
   product test we currently lack — a side benefit worth naming.)
6. **H2 in-situ scoring** — extracting "what exploration conclusions
   entered host context" from a control-arm transcript is new and the
   least deterministic piece; the fallback design is to score only the
   sidecar arm's reports deterministically and lean on the judge for
   the cross-arm comparison, with the limitation stated.

## Honest cost estimate per episode

Grounded in the recon suite's measured economics (ADR-0025: 90 episodes
≈ 2–3.5 h sequential; direct-profile episodes consumed ~64k chars of
main-agent context on a *single recon question*) and public data points
for coding-agent tasks:

- **Control arm episode**: a small engineering task (fix + test loop) on
  a sonnet-class host ≈ 10–30 min wall, ~150k–600k total tokens
  (multiple test runs, exploration by hand). API-equivalent ≈ $1–5;
  on-subscription the cost is rate-limit pressure, not dollars.
- **Sidecar arm episode**: host tokens drop (that's the thesis) but add
  1–3 sidecar questions ≈ 30–90k sidecar-internal tokens (sonnet-class,
  from recon-suite observations) + MCP round-trips. Similar wall time
  first; ≈ $1.5–6 API-equivalent.
- **Grader**: container build amortized per task (minutes once), test
  runs seconds-to-minutes per attempt.
- **First registered run**: 6 tasks × 2 arms × 5 trials = 60 episodes ≈
  **$120–360 API-equivalent, 1–2 working days wall sequential, ~0.5 day
  at 3-way parallelism** — roughly 3–5× the recon suite per episode and
  the reason D1 stopping, small corpus, and non-inferiority framing are
  load-bearing rather than nice-to-have. Numbers are estimates, not
  citable; the first run measures them (ADR-0025 D5 discipline).

## Prior art (what we steal, with attribution)

| Source | What it gives this design |
|--------|---------------------------|
| [SWE-bench harness](https://www.swebench.com/SWE-bench/reference/harness/) + [SWE-bench Verified](https://openai.com/index/introducing-swe-bench-verified/) | Pinned-SHA repos, Docker-pinned envs, hidden test patch, fail-to-pass + pass-to-pass invariants — the outcome-grading pattern, adopted not reinvented |
| [SWE-Bench Illusion (arXiv:2506.12286)](https://arxiv.org/abs/2506.12286) | Memorization evidence (76% file-path recall from issue text alone) → the memorization canary + fresh/hand-authored corpus requirement |
| [SWE-bench-Live (arXiv:2505.23419)](https://arxiv.org/abs/2505.23419), [repo](https://github.com/microsoft/SWE-bench-Live) | Post-cutoff recency filtering and automated env curation as the anti-contamination pattern |
| [τ-bench (arXiv:2406.12045)](https://arxiv.org/abs/2406.12045) | pass^k as the reliability metric alongside pass@1 — "all k trials succeed" matches how a sidecar-assisted engineering loop is actually consumed |
| [Terminal-Bench (arXiv:2601.11868)](https://arxiv.org/abs/2601.11868) | Neutral-scaffold comparisons; evidence that harness choice moves scores 10–20pp → frozen host-harness identity |
| [mini-swe-agent](https://github.com/SWE-agent/mini-swe-agent) | The minimal-harness control arm exists and is strong (>74% SWE-bench Verified, bash-only) — our control is not a strawman if built this way |
| [Anthropic: How we built our multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system) | Orchestrator–subagent token economics (multi-agent spends more total tokens for better outcomes) — exactly why whole-workflow SPT must be reported, not just host SPT |
| [Cognition: Don't Build Multi-Agents](https://cognition.com/blog/dont-build-multi-agents) | The strongest steelman *against* the sidecar arm: context fragmentation between agents causes conflicting decisions. Our rebuttal is scoped (recon is read-only evidence, not decision-splitting) — but H1's non-inferiority gate on task success is precisely where Cognition's failure mode would show up, which is why success is measured at all |
| [Chroma: Context Rot](https://research.trychroma.com/context-rot) | Independent evidence that performance degrades non-uniformly with input length and distractor content — the mechanism behind "dead weight of exploration steps forces restarts"; motivates context high-water mark and longevity as registered metrics |
| [Aider polyglot benchmark](https://github.com/Aider-AI/polyglot-benchmark) | Considered and rejected (self-contained tasks, no exploration subneed) — kept as the boundary example of what this corpus must NOT be |
| ADR-0026 (internal prior-art landscape) | No surveyed framework (smolagents, LangGraph, AutoGen, CrewAI, Mastra, Inspect, Phoenix) ships a two-agent host-task eval of this shape; Inspect AI remains the closest eval-harness pattern donor for task/scorer separation |

## Considered and rejected (design level)

- **Three arms (adding `ghx`-direct host)** — cost doubles for an
  already-answered question; revisit only on strawman evidence.
- **A single blended combined score** — hides the conjunction the claim
  requires; verdicts stay a vector (H1 success, H1 efficiency, H2),
  rendered separately.
- **Superiority gate on binary task success** — statistically
  unaffordable at realistic budgets; non-inferiority + continuous
  efficiency superiority instead (the trap section is the argument).
- **Hard-blocking external tools in the sidecar arm** — manufactures
  product-unrealistic failures; prompt contract + compliance detector
  instead.
- **Reusing SWE-bench Verified instances** — memorization; see corpus
  rejections.
- **Simulated hosts (scripted "main agent" replaying fixed exploration
  needs)** — cheap and deterministic, but it deletes the phenomenon
  under test (context pollution of a *live* reasoning agent);
  usable only as plumbing tests, mock-agent style (ADR-0016.1).
- **Dogfood telemetry instead of a controlled eval** — M5 dogfooding
  produces exactly these observations uncontrolled; it seeds task
  authoring and effect-size guesses but cannot be the citable
  comparison (no counterfactual arm). Both, in their lanes.

## Open questions for ADR-0032.1 (the decision memo)

1. Gate set and thresholds: which of the metric table rows become gates,
   non-inferiority margin for success, minimum sample. (Nothing here is
   registered.)
2. Host subject model: sonnet-class to match the recon suite, or the
   ADR-0025 D4 haiku arm question repeated at host level — cheaper hosts
   should benefit *more* from deferred exploration (the P4 thesis in
   miniature, again).
3. Arm-blind judge bundles for H2 (the report-shape recognizability
   problem).
4. Ref-carrying recon questions (product feature) vs corpus-level skew
   avoidance.
5. Whether workspace provisioning adopts the SWE-bench container format
   outright or a thinner task-container convention (open-source-leverage
   call, needs a spike).
6. Credential separation for the two agents vs anomaly-and-exclude under
   one subscription.

## Cross-references

- NORTH_STAR "Why" — the rebuttal this class exists to measure; "The
  Moat" (signal per token as the target).
- ADR-0016.6 — SPT three levels + the reserved combined-objective note
  this memo fills; dead-weight fraction, session longevity, host-task
  SPT named there first.
- ADR-0016.1/0016.2/0016.8 — episode kernel, validity rules, and
  measurement-fix discipline all inherited; safety-gate inversion and
  the new anomalies extend, never fork, that machinery.
- ADR-0016.5 — decontamination rules extended to sidecar session store
  and host workspaces.
- ADR-0023.1 — judge layer for H2; calibration gate applies unchanged.
- ADR-0025 — run economics; stopping bounds and parallelism reused, the
  cost table above is why they are load-bearing here.
- ADR-0019/0020.1 — the recon MCP consumption surface arm B exercises
  end-to-end for the first time.
- ADR-0026 — prior-art landscape; no surveyed framework ships this
  eval shape.
