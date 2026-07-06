---
title: "ADR-0023: Judge Scorer — Research"
date: "2026-07-05"
status: "research"
thread: "judge-scorer"
author: "Goga Koreli"
---

# ADR-0023: Judge Scorer — Research

Pre-decision research memo grounding the judge scorer ADR. Separates VERIFIED
findings (URL + access date 2026-07-05 unless noted) from INFERRED. Architecture
decisions are not in scope here; only constraints for the decision ADR.

---

## Repo context absorbed

- **NORTH_STAR.md**: "Observability is the substrate for self-improvement" — judge
  runs over both benchmark episodes and real production sessions from `~/.ghx`;
  same substrate as evals. Judge scorer is named as a required layer alongside
  deterministic gates; ADR before build; natural sequencing after ADR-0016.7
  confirmatory rerun and before M9 preference/reward exports.
- **AGENTS.md Visibility/Truthfulness**: judge must emit full OTel reasoning traces;
  calibration (agreement with hand labels, FP/FN rates) measured and reported next
  to scores; prompt and model version committed; disagreement between layers is a
  first-class signal, not noise to average away.
- **ADR-0016.1**: gates are deterministic Go over committed artifacts; correctness ×
  evidence = signal formula; G1–G5 pre-registered; judge explicitly deferred as
  "secondary explainer, never the gate."
- **ADR-0016.6**: SPT = signal ÷ (chars/4) × 1000; main-agent / sidecar-internal /
  workflow levels; token upgrade path when `gen_ai.usage.input_tokens` is available.
- **ADR-0016.7**: false-negative anatomy — 10/22 healthy (correctness 0.95, evidence
  0.94), 8 BLOCKED, 4 WARN-noreport. Judge needs to handle this mix fairly.
- **ADR-0018**: `gen_ai.evaluation.result` events already emitted for deterministic
  rewards; judge scorer must reuse the same event shape. Content logs via
  `gen_ai.client.inference.operation.details`. Reasoning as `{"type":"reasoning"}`
  output part. Agent thinking captured via `agent_thought_chunk`.
- **ADR-0021**: report contract enforced via validated `submit_report` MCP tool;
  reduces WARN-noreport anomalies structurally.
- **ADR-0022**: shared telemetry capability in `internal/sidecar/telemetry`; production
  sessions emit same OTLP JSONL as eval episodes; judge must consume both from
  `~/.ghx`; one substrate, not two.

---

## Q1 — State of the art: LLM-as-judge for agent trajectories

### VERIFIED

**Rubric design — task-adaptive over fixed dimensions.**
AdaRubric (Ding, arXiv:2603.21362, 2025) shows that fixed evaluation rubrics
systematically mis-evaluate diverse agent tasks. The framework generates task-specific
rubrics from task descriptions, evaluates step-by-step with confidence-weighted
per-dimension scoring, and produces dense reward signals. Results on WebArena/ToolBench/
AgentBench: Pearson r = 0.79 (+0.15 over fixed-rubric baselines), Krippendorff α = 0.83.
Models trained on AdaRubric-generated preference pairs improve task success +6.8–8.5%.
URL: https://arxiv.org/abs/2603.21362 (accessed 2026-07-05)

**Trace-grounded judging is a hard open problem.**
TRAIL (Deshpande et al., arXiv:2505.08638, May 2025) — 148 annotated traces, 841
errors, real software engineering and open-world retrieval agents. Best frontier model
(Gemini-2.5-pro) scored 11% on trace debugging. Modern LLMs perform poorly at
long-context trace reasoning. Benchmark and code: github.com/patronus-ai/trail-benchmark.
URL: https://arxiv.org/abs/2505.08638 (accessed 2026-07-05)

**RuVerBench — rubric verification in agentic coding/research (2025).**
arXiv:2606.29920: even frontier models achieve "substantial noise" at rubric
verification over long, complex agent outputs. Majority voting improves reliability
but plateaus. Deep research and agentic coding are the hardest domains. Batch
processing creates accuracy-efficiency tradeoffs.
URL: https://arxiv.org/abs/2606.29920 (accessed 2026-07-05)

**Agent-as-a-Judge (Zhuge et al., arXiv:2410.10934, 2024).**
Uses an agentic evaluator (not just a chat model) to evaluate agent trajectories.
DevAI benchmark: 55 tasks, 365 hierarchical requirements. Tested MetaGPT, GPT-Pilot,
OpenHands. Agent judge "dramatically outperforms LLM-as-Judge and is as reliable as
human evaluation." Key insight: agents can evaluate intermediate steps; LLM chat judges
only reliably evaluate final outputs.
URL: https://arxiv.org/abs/2410.10934 (accessed 2026-07-05)

**Pairwise vs absolute scoring tradeoff.**
Pairwise comparisons produce more stable results and smaller human-LLM annotation
deltas than absolute scoring (confirmed by multiple sources including
comet.com/site/blog/llm-as-a-judge). Downside: O(n²) comparisons; fits periodic A/B
tests (plain vs ghx vs ghx-sidecar), not continuous production monitoring.
Absolute scoring with rubric criteria is preferred for per-episode production monitoring;
pairwise for release decisions and comparative benchmarks. Position-consistency check
(swap A/B order, average) is low-cost mitigation.
URL: https://www.comet.com/site/blog/llm-as-a-judge/ (accessed 2026-07-05)

**An Empirical Study of Automating Agent Evaluation (2605.11378, 2025).**
Multi-agent meta-judge (rubric + multi-agent scoring) improves accuracy by 15.55pp
over raw LLM scores. Static LLM-as-Judge r ≈ 0.46; AdaRubric-style adaptive r ≈ 0.77.
URL: https://arxiv.org/abs/2605.11378 (accessed 2026-07-05)

### INFERRED

For code-reconnaissance trajectories, the hardest part is grounding the judge's
reasoning in actual tool output (ghx command results, API responses). A judge that
only sees the final answer misses the trajectory quality that is the core claim.
The TRAIL score of 11% for frontier models at trace debugging suggests a judge
operating over full raw traces will have high variance — pre-structured trace
summaries (our OTel spans + reward events already in `traces.jsonl`) are a better
judge input than dumping raw text.

---

## Q2 — Calibration practice

### VERIFIED

**Gold-set size: 200–500 hand-labeled traces per workload per rubric**, each labeled
by 2–3 humans to establish inter-annotator baseline.
Per-cell minimums of ~4 items are insufficient for per-model kappa estimation
(confirmed by arXiv:2606.01034 — "A Finite-Calibration Regime Map for LLM Judge Panels").
URL: https://futureagi.com/blog/llm-as-judge-best-practices-2026 (accessed 2026-07-05)

**Cohen's kappa threshold: 0.6 is the common floor; 0.7 is "substantial agreement."**
One validation study achieved 80.4% accuracy / κ = 0.67, below the 0.70 threshold.
For expert-knowledge domains (code), agreement drops to 64–68%, below inter-expert
baseline (~72–75%). Prompt scaffolding with explanations produces +0.05–0.10 kappa
improvement — the largest measurable intervention.
URL: https://futureagi.com/blog/llm-as-judge-best-practices-2026 (accessed 2026-07-05)

**Recalibration: monthly minimum for production workloads. Drift in 60–90 days
without it.** When judge prompt or model changes, recalibrate against the gold-set
before publishing scores. Track rubric prompts in the same version registry as
production prompts ("a rubric tweak that changes scores is a regression you cannot
bisect"). Three paths: tighten rubric, swap model, refresh gold-set.
URL: https://futureagi.com/blog/llm-as-judge-best-practices-2026 (accessed 2026-07-05)

**A Finite-Calibration Regime Map (arXiv:2606.01034, 2025):**
When per-model-per-trigger cell sizes are ~4 items, per-model kappa is statistically
unreliable. The paper maps which calibration regimes are tractable given realistic
label budgets.
URL: https://arxiv.org/abs/2606.01034 (accessed 2026-07-05)

**Survey on LLM-as-a-Judge (arXiv:2411.15594v4, 2024–2025):**
Comprehensive treatment of bias types, calibration methods, and practitioner guidance.
URL: https://arxiv.org/abs/2411.15594 (accessed 2026-07-05)

### INFERRED

The current gate-run M4 produced 22 valid sidecar episodes (ADR-0016.1), with 10
healthy / 8 BLOCKED / 4 WARN. A gold-set of 200–500 labeled traces is out of reach
initially; the practical path is a small pilot (20–50 episodes with known-good and
known-bad examples from the two gate runs) to establish a baseline kappa, then grow
the gold-set over successive runs. The first kappa will be noisy; that is expected
and should be reported as PRELIMINARY, consistent with the verdict-is-a-floor tenet.

---

## Q3 — Open-source scaffolding (local-first requirement)

### VERIFIED

**Braintrust autoevals** — open-source Python/TypeScript library; no Braintrust
platform dependency required; works with any OpenAI-compatible provider or custom
LLM; preconfigured scorers for factuality, relevance, closed QA, summarization.
Go package exists: `github.com/braintrustdata/braintrust-x-go/braintrust/autoevals`.
Judge runs entirely locally when pointed at a local or API-accessible model.
URLs: https://github.com/braintrustdata/autoevals (accessed 2026-07-05),
https://pkg.go.dev/github.com/braintrustdata/braintrust-x-go/braintrust/autoevals
(accessed 2026-07-05)

**Promptfoo** — MIT-licensed, CLI + YAML config, local-first (eval runner runs on
your machine, provider calls go direct to provider). Supports Ollama/local models,
custom provider adapters, LLM-as-judge assertions, string matching, and regex —
composable. Widely used (cited by OpenAI and Anthropic).
URL: https://github.com/promptfoo/promptfoo (accessed 2026-07-05)

**DeepEval** — Python framework; G-Eval (chain-of-thought judge) runs locally with
no Confident AI platform key required; supports custom local models. Metrics run
on your machine. Provides trajectory evaluation primitives.
URL: https://github.com/confident-ai/deepeval (accessed 2026-07-05)

**Arize/Phoenix** — open-source, OTel-native (OTLP ingest on :6006); supports
LLM-as-judge evaluators including deterministic and LLM-graded. But: native
rendering is OpenInference conventions, not OTel GenAI semconv. GenAI-semconv
spans from Go get a degraded UI — no Go GenAI→OpenInference translator exists
(confirmed in ADR-0018 research). Viable as secondary viewer; primary local
surface is otel-desktop-viewer per ADR-0018.
URL: https://github.com/Arize-ai/phoenix (accessed 2026-07-05)

**OpenAI Evals** — open-source registry + framework; model-graded and deterministic
graders; YAML config; supports local runs with custom model provider. However:
OpenAI is deprecating the Evals platform (read-only 2026-10-31, shutdown 2026-11-30).
The library itself (open-source fork) may continue but the ecosystem is fragmenting.
HealthBench design (2025): per-instance rubrics (10–40 criteria per conversation),
not one rubric per dataset — significant conceptual advance for ghx trajectories.
URL: https://github.com/openai/evals (accessed 2026-07-05)

**Inspect AI (UK AISI)** — strongest full-stack agentic eval framework (Task/Solver/
Scorer pattern, multi-turn, LLM-as-judge, sandboxed execution, VS Code log viewer).
Limitation: Python only. Not directly usable from Go; referenced as design inspiration
in ADR-0014.4.
URL: https://inspect.aisi.org.uk/ (accessed 2026-07-05)

**tau-bench / tau²-bench** — pass^k reliability metric (run N independent trials,
report P(correct on k of N)); trajectory scoring borrowed in ADR-0014.4; Python,
domain-specific. Borrowable concept, not a usable library.
URL: https://github.com/sierra-research/tau-bench (accessed 2026-07-05)

### KEY FINDING (VERIFIED)

**Braintrust autoevals has a Go package** (`braintrust-x-go`). This is the only
local-first judge scorer library with a native Go interface found. All others require
spawning a Python process or using a REST interface. The Go package supports custom
LLM callers, so it could call the locally-installed Claude agent via the existing
ACP adapter — no raw API key needed if the local call is proxied.

### INFERRED

None of the above frameworks have built-in support for OTel GenAI semconv input or
`gen_ai.evaluation.result` output. They produce their own score formats. To maintain
the tenet (official formats exactly, no forks), any adopted framework must be wrapped:
feed it trace JSON, get back scores, emit via `gen_ai.evaluation.result` in our Go
OTel exporter. The wrapping layer belongs in `internal/sidecar/evals` or the
`internal/sidecar/telemetry` package (ADR-0022 D1).

---

## Q4 — Devin/Cognition Critic pattern and adversarial self-review

### VERIFIED

**Devin Review (Cognition, launched early 2026):**
Cognition shipped "Devin Review" — a code review tool that analyzes PR diffs, groups
logically connected changes, and categorizes issues by severity (red = probable bugs,
yellow = warnings, gray = commentary). WWT partner overview attributes a 70–90%
remediation rate. The tool augments human reviewers rather than replacing them.
URL: https://cognition.com/blog/devin-review (accessed 2026-07-05)

**Self-verification bias problem (primary source):**
"Self-reflection in agentic trajectories is susceptible to self-verification bias
where the agent judges its own intermediate outputs from within the same trajectory
state that produced them. To break this coupling, extrinsic verification is introduced,
where verification signals are produced in an isolated context."
(From "DeepVerifier," referenced in search results, not independently fetched.)
Devin Review corroborates: "Humans will write a unit testing playbook for Devin...
code owners will check to see if all logic has been tested" — even Cognition
requires external human check for quality.
URL: https://cognition.com/blog/devin-annual-performance-review-2025 (accessed 2026-07-05)

**"Generative Adversarial Reviews: When LLMs Become the Critic" (arXiv:2412.10415):**
Proposes adversarial LLM review of generated content as a structured pattern. Separate
generator and critic, isolated contexts, critic cannot call the same tools as the
subject.
URL: https://arxiv.org/abs/2412.10415 (accessed 2026-07-05)

### INFERRED

The "Critic" mention in WWT's Cognition partner page ("The Critic is an adversarial
model that reviews code for security vulnerabilities and logic errors before execution")
appears to be marketing copy rather than a published technical design. No primary
technical source describing the architecture was found. The pattern itself — isolated
judge context, no tool access, adversarial posture — is well-attested in the academic
literature and already present in ADR-0014.4's design (judge runs via `acpxSend` with
`permissionMode: 'deny-all'`; fresh session per run; no cross-contamination).

The self-preference bias risk is real: when the judge model is the same family as the
subject model (e.g., Claude scoring Claude trajectories), bias is measurable. Mitigation:
use a maximally different judge family, or run a dual-judge ensemble across families and
report disagreements explicitly.

---

## Q5 — OTel GenAI alignment: gen_ai.evaluation.* semconv status

### VERIFIED

**`gen_ai.evaluation.result` event is in the official OTel GenAI spec.**
PR open-telemetry/semantic-conventions#2563 merged August 2025; included in
semconv v1.39.0. Attributes:
- `gen_ai.evaluation.name` (required) — evaluator identifier
- `gen_ai.evaluation.score.value` — numeric score
- `gen_ai.evaluation.score.label` — categorical label (e.g., "pass")
- `gen_ai.evaluation.explanation` — reasoning text
- `gen_ai.response.id` — correlation to agent response
- `error.type` — if evaluation failed
Source: github.com/strands-agents/sdk-python/issues/1633 (references the merged PR)
(accessed 2026-07-05)

**ADR-0018 implementation already uses this event shape** for deterministic reward
events: `gen_ai.evaluation.name` and `gen_ai.evaluation.score.value` are already
emitted in `internal/sidecar/evals/otel_reward_events.go`. The judge scorer must
emit the same event shape — no new format needed, no fork of the standard.

**Overall semconv status as of mid-2026: Development/Experimental.**
Most GenAI semantic conventions remain in experimental status. Major vendors
(Datadog, Grafana, MLflow) have started supporting them. Production adoption uses
`OTEL_SEMCONV_STABILITY_OPT_IN` for dual-emission during version transitions.
URL: https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/ (accessed 2026-07-05)
URL: https://github.com/open-telemetry/semantic-conventions-genai (accessed 2026-07-05)

**Content logs:** since semconv ~v1.37, content is a `gen_ai.client.inference.operation.details`
log event, not span events. ADR-0018 implementation matches this.

**Judge reasoning trace:** no dedicated reasoning event in semconv; thinking is a
`{"type":"reasoning"}` part in output messages, plus `gen_ai.usage.reasoning.output_tokens`.
ADR-0018 already implements this representation.

### INFERRED

The judge scorer should emit exactly:
1. A parent span for the judge invocation (`gen_ai.operation.name = "chat"`,
   `gen_ai.request.model = <judge model id>`).
2. One `gen_ai.evaluation.result` event per scored dimension (reusing the existing
   reward-event emitter in `otel_reward_events.go`, extended to accept judge output).
3. A content log record carrying the judge prompt and reasoning output, gated by
   `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT`.
4. The judge's `gen_ai.evaluation.explanation` riding on the event alongside the
   numeric score.

This is additive over the existing infrastructure with no format changes.

---

## Q6 — Judge economics: cheap vs frontier, ensemble, when gate short-circuits

### VERIFIED

**Cost-effective techniques paper (arXiv:2604.13717, ~May 2026):**
Tested four drop-in techniques: ensemble scoring, task-specific criteria injection,
calibration context, adaptive model escalation. Key findings:
- Criteria injection (near free) + ensembling (k=3–8) dominates the cost-accuracy
  Pareto frontier: up to 85.8% accuracy, +13.5pp over single baseline model.
- **Small models benefit disproportionately from ensembling**: mini k=8 reaches 79.2%
  at 1.2× baseline cost; mini+criteria k=8 matches full-model k=8 (81.5%) at ~0.25×
  the cost.
- Adaptive escalation (route uncertain cases to a frontier model) improves over
  baseline but is dominated by criteria+ensembling on the cost-accuracy curve.
- Results generalize across OpenAI GPT and Anthropic Claude families.
URL: https://arxiv.org/abs/2604.13717 (accessed 2026-07-05)

**Position bias: 10–15pp winrate swing; verbosity bias: 15–30pp inflation.**
Mitigations:
- Swap augmentation (evaluate A vs B then B vs A, accept consistent, mark tie).
- Well-specified rubrics with explicit length-agnostic scoring criteria.
- Ensemble across different judge families for cross-family consensus.
URL: https://futureagi.com/blog/evaluating-llm-judge-bias-mitigation-2026/ (accessed 2026-07-05)
URL: https://mbrenndoerfer.com/writing/position-bias-in-llm-judges (accessed 2026-07-05)

**Self-preference bias (arXiv:2410.21819, 2024):**
Measurable when judge model = subject model family. Judge models that are newer
versions of older generator models show systematic self-preference.
URL: https://arxiv.org/abs/2410.21819 (accessed 2026-07-05)

**Ensemble of open-source models (Mixture-of-Agents, 2025):** committee of weaker
models can outperform a single strong model (AlpacaEval 2.0: ensemble 65.1% vs
GPT-4 57.5%). Practical for cost-constrained local setups.

**Deterministic gate as pre-filter (industry consensus 2025–2026):**
"Run the deterministic check, fail fast on mismatch, only call the judge for parts
that genuinely require natural-language reasoning." Deterministic gates at the
perimeter (non-negotiable, audit-grade); LLM judge downstream (advisory, not
blocking). Our G1–G5 gates already implement this architecture.
URL: https://braintrust.dev/articles/what-is-llm-as-a-judge (accessed 2026-07-05)
URL: https://data443.com/blog/deterministic-policy-vs-llm-filters/ (accessed 2026-07-05)

### INFERRED

The gates already short-circuit three cases where a judge adds noise, not signal:
- G5 (safety) failures: judge opinion on a safety violation is irrelevant; it is a
  gate violation, not a quality score.
- BLOCKED anomalies (ADR-0016.7 RC1): zero exploration happened; judge has nothing
  to reason about. Confirmed in ADR-0016.7 — anomaly detection flags these before
  scoring.
- WARN-noreport anomalies: structurally addressed by ADR-0021 submit_report MCP tool;
  residual cases should still be excluded from judge scoring (judge cannot evaluate
  absent reports).

For the ghx judge specifically: the judge model should be a different family than
the subject model. If the subject runs on `claude-sonnet-5` (current eval adapter),
the judge should run on a non-Anthropic model (e.g., GPT-4o or a local model) to
avoid self-preference bias. Alternatively, a dual-judge ensemble (one Anthropic +
one OpenAI/local) with explicit disagreement reporting satisfies the calibration
tenet without picking a single family.

---

## Candidate Design Constraints for the Decision ADR

These are constraints, not decisions. The decision ADR resolves the tradeoffs.

1. **Judge must never be a gate.** It is a second, complementary layer over G1–G5.
   Disagreement between the two layers is a first-class signal (AGENTS.md
   Visibility/Truthfulness). Judge scores inform; deterministic gates decide.

2. **Judge context = structured trace summary, not raw text.** TRAIL's 11% frontier
   score on raw trace debugging argues against dumping full transcripts. Input to
   the judge should be the OTel span structure + reward events already in
   `traces.jsonl` + the accepted report JSON (from `reports/<turn>-<ts>.json`
   per ADR-0022 D2) — not raw ACP session text.

3. **Emit `gen_ai.evaluation.result` events.** The attribute schema is standardized
   (PR #2563, v1.39.0). The existing `otel_reward_events.go` emitter must be
   extended, not forked, to carry judge scores alongside deterministic rewards.
   Judge reasoning rides as `gen_ai.evaluation.explanation` and as a content log
   record with `{"type":"reasoning"}` output part.

4. **Judge prompt and model version are committed artifacts.** Version-controlled
   in the repo (not stored only in a service). Any change to either triggers a
   recalibration run against the gold-set before scores are cited. This is the
   same measurement-stack-frozen rule as the existing gates.

5. **Calibration first, production scores second.** Initial gold-set of 20–50 hand-
   labeled episodes from the two committed gate runs (`docs/evals/gate-run-2026-07/`
   and `docs/evals/gate-run-2026-07-05-confirmatory/`). First kappa will be noisy —
   must self-label PRELIMINARY, consistent with the verdict-is-a-floor tenet.
   Target for publishable scores: κ ≥ 0.6 on 200+ labeled episodes.

6. **Anti-self-preference: judge model family ≠ subject model family.** Subject runs
   on `claude-sonnet-5`; judge must use a non-Anthropic model, or run a dual-judge
   ensemble (Anthropic + non-Anthropic) with disagreement reported explicitly.

7. **Task-adaptive rubric, not fixed dimensions.** AdaRubric evidence: fixed rubric
   r ≈ 0.46, task-adaptive r ≈ 0.79. For ghx, the rubric per task should derive from
   the task's `expectedFiles`, `requiredClaims`, and `judgeContext` fields — matching
   what a correct answer looks like for that specific task, not generic quality axes.

8. **Anomaly pre-filter before judge invocation.** BLOCKED and WARN-noreport episodes
   (ADR-0016.7 anomaly taxonomy) must be excluded from judge scoring. The judge has
   no material to evaluate; scoring them would dilute aggregates. This mirrors the
   existing invalid-episode filtering in gates.go.

9. **Local-first, no eval-service key.** Judge must run via the same locally-installed
   agent infrastructure as eval subjects (existing ACP adapter or direct API if a key
   exists), or via a local model. No hosted eval platform dependency (NORTH_STAR tenet).
   Braintrust autoevals Go package and Promptfoo are the two open-source options that
   satisfy this; both are usable as rubric/prompt libraries even if their runner is
   not adopted.

10. **Same OTel substrate as benchmarks and production sessions.** ADR-0022's extraction
    rule: never fork the visibility stack. Judge invocations must produce traces,
    content logs, and evaluation result events that land in the same `~/.ghx` session
    directory as the episodes being judged — viewable with the same otel-desktop-viewer
    replay recipe.

11. **Pairwise scoring for release A/B decisions; absolute scoring for per-episode
    production monitoring.** These are different use cases with different cost profiles.
    The pairwise path (plain vs ghx vs ghx-sidecar) uses swap augmentation to cancel
    position bias. The absolute path (per-episode rubric score) uses criteria injection
    + small ensembles (k=3) as the cost-effective baseline.

12. **M9 dependency.** The judge scorer is a prerequisite for M9 trajectory-quality
    labels for preference/reward exports. Judge score labels need to be committed
    artifacts (not ephemeral) — stored in the episode JSON and in the OTel events,
    recomputable by humans.

---

## Open Questions

1. **Which model runs the judge?** The decision requires a concrete answer: a specific
   non-Anthropic model (GPT-4o, local Llama, etc.) or a dual-judge ensemble. The model
   choice determines cost, self-preference risk, and calibration sample needs. Requires
   testing before the decision ADR is written.

2. **Agent-as-judge vs chat-model judge.** The Agent-as-a-Judge pattern
   (arXiv:2410.10934) outperforms chat-model judges on code tasks. But it requires
   another ACP session with tool access — much higher cost and latency. For ghx
   trajectories, does tool access (e.g., the judge running `ghx read` to verify a
   claim) improve score quality enough to justify it? Unknown without an experiment.

3. **Gold-set construction protocol.** Who labels and how? The only episodes available
   today are the two gate runs (90 episodes each). How many are usable given the anomaly
   taxonomy? A hand-labeling session is required before calibration is possible.

4. **Task-adaptive rubric generation.** AdaRubric generates rubrics from task descriptions
   via LLM. For ghx, this could mean generating the rubric from `Task.Checks` fields
   at eval time. Risk: rubric generation adds a second LLM call per episode, increasing
   latency and cost. Is it worth it vs. hand-crafting rubrics per task type?

5. **Disagreement handling protocol.** When the judge score and the deterministic gate
   disagree (e.g., judge rates a trajectory poorly even though G1+G2 pass, or vice
   versa), what is the exact investigative protocol? This is called out as "first-class
   signal to investigate" in AGENTS.md but the protocol is undefined.

6. **Production session scoring cadence.** ADR-0022 promises production sessions emit
   the same traces as eval episodes. Should the judge run on every production session,
   on a sampled subset, or only on gate-run episodes? Cost and latency tradeoffs are
   unquantified.

7. **Phoenix secondary viewer.** ADR-0018 research found Phoenix does not natively
   render OTel GenAI semconv spans well (OpenInference mismatch). But Phoenix supports
   LLM-as-judge evaluation natively with its own format. Is there value in dual-format
   emission (OTel canonical + OpenInference compatibility layer) to unlock Phoenix's
   judge UI, or is this a case where the open-source-leverage tenet says wait for
   Phoenix to adopt the standard?

---

## Primary Sources

| # | Source | URL | Access date |
|---|--------|-----|-------------|
| 1 | AdaRubric (arXiv:2603.21362) | https://arxiv.org/abs/2603.21362 | 2026-07-05 |
| 2 | TRAIL benchmark (arXiv:2505.08638) | https://arxiv.org/abs/2505.08638 | 2026-07-05 |
| 3 | RuVerBench / rubric verification (arXiv:2606.29920) | https://arxiv.org/abs/2606.29920 | 2026-07-05 |
| 4 | Agent-as-a-Judge (arXiv:2410.10934) | https://arxiv.org/abs/2410.10934 | 2026-07-05 |
| 5 | Cost-effective LLM judge techniques (arXiv:2604.13717) | https://arxiv.org/abs/2604.13717 | 2026-07-05 |
| 6 | LLM-as-Judge survey (arXiv:2411.15594v4) | https://arxiv.org/abs/2411.15594 | 2026-07-05 |
| 7 | Finite-calibration regime map (arXiv:2606.01034) | https://arxiv.org/abs/2606.01034 | 2026-07-05 |
| 8 | Self-preference bias (arXiv:2410.21819) | https://arxiv.org/abs/2410.21819 | 2026-07-05 |
| 9 | Generative Adversarial Reviews (arXiv:2412.10415) | https://arxiv.org/abs/2412.10415 | 2026-07-05 |
| 10 | An Empirical Study of Automating Agent Evaluation (arXiv:2605.11378) | https://arxiv.org/abs/2605.11378 | 2026-07-05 |
| 11 | OTel genai semconv #2563 (via strands-agents issue) | https://github.com/strands-agents/sdk-python/issues/1633 | 2026-07-05 |
| 12 | OTel genai semconv repo | https://github.com/open-telemetry/semantic-conventions-genai | 2026-07-05 |
| 13 | FutureAGI LLM judge calibration best practices 2026 | https://futureagi.com/blog/llm-as-judge-best-practices-2026 | 2026-07-05 |
| 14 | FutureAGI LLM judge bias mitigation 2026 | https://futureagi.com/blog/evaluating-llm-judge-bias-mitigation-2026/ | 2026-07-05 |
| 15 | Braintrust autoevals Go package | https://pkg.go.dev/github.com/braintrustdata/braintrust-x-go/braintrust/autoevals | 2026-07-05 |
| 16 | Braintrust autoevals (open-source) | https://github.com/braintrustdata/autoevals | 2026-07-05 |
| 17 | Promptfoo GitHub | https://github.com/promptfoo/promptfoo | 2026-07-05 |
| 18 | DeepEval GitHub | https://github.com/confident-ai/deepeval | 2026-07-05 |
| 19 | Arize/Phoenix GitHub | https://github.com/Arize-ai/phoenix | 2026-07-05 |
| 20 | OpenAI Evals GitHub | https://github.com/openai/evals | 2026-07-05 |
| 21 | Devin Review (Cognition) | https://cognition.com/blog/devin-review | 2026-07-05 |
| 22 | Position bias in LLM judges (mbrenndoerfer.com) | https://mbrenndoerfer.com/writing/position-bias-in-llm-judges | 2026-07-05 |
| 23 | Braintrust: what is LLM-as-judge | https://www.braintrust.dev/articles/what-is-llm-as-a-judge | 2026-07-05 |
| 24 | Inspect AI (UK AISI) | https://inspect.aisi.org.uk/ | 2026-07-05 |
| 25 | tau-bench (Sierra Research) | https://github.com/sierra-research/tau-bench | 2026-07-05 |

---

## What Could Not Be Verified

- **Cognition "Critic" architecture.** WWT partner page describes it as "an adversarial
  model that reviews code for security vulnerabilities and logic errors before execution,"
  but this appears to be marketing copy. No Cognition primary technical paper or
  detailed architectural description was found. Treated as INFERRED.

- **Exact AdaRubric DPO training pipeline.** The paper claims +6.8–8.5% task success
  improvement from DPO training on AdaRubric preference pairs. Not independently
  verified against a separate source — trusted from abstract only.

- **OTel semconv v1.39.0 `gen_ai.evaluation.result` shape.** The PR number (#2563)
  and attributes are sourced from a GitHub issue referencing the merged PR, not from
  the official spec YAML directly. The attribute list may differ from what shipped.
  Re-verify against `github.com/open-telemetry/semantic-conventions-genai/model/` YAML
  at implementation time.

- **Phoenix OTel GenAI semconv rendering maturity.** ADR-0018 research noted Phoenix
  renders OpenInference natively, with degraded OTel GenAI span rendering. This was
  noted as "unverified" then. Still unverified in current research — Phoenix may have
  improved since.

- **DeepEval / Promptfoo agent trajectory primitives.** Both support LLM-as-judge for
  single responses; their trajectory-level primitives (multi-turn, tool-call sequences)
  were not deeply verified. Surface-level search suggests DeepEval has some trajectory
  metrics but the exact API was not examined.
