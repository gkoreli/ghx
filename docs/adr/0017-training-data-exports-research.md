---
title: "ADR-0017: Training-Data Exports — Research"
date: 2026-07-05
status: research
thread: training-data
author: "Goga Koreli"
---

# ADR-0017: Training-Data Exports — Research Memo

## Purpose

This memo grounds the future ADR-0017 decision (SFT/preference/reward export
formats and pipeline for the custom-trained ghx-sidecar reconnaissance model,
NORTH_STAR M9/M10, P4). It answers six research questions, separates VERIFIED
from INFERRED findings, and closes with candidate design constraints — not
decisions. Decisions belong in the ADR proper, after calibrated judge scores
(ADR-0023.1 D5) and sufficient trajectory volume (M9 target) exist.

Repo context consumed: NORTH_STAR (M9/M10, P4, "The Moat", open-source-leverage
filter), ADR-0016 (training-data trajectory section, episode schema),
ADR-0016.6 (SPT: signal = correctness × evidence, chars/4 token estimate),
ADR-0023.1 (judge labels as trajectory-quality signals, D5 calibration gate,
sequencing rule: M9 exports may use judge labels only after D5 passes),
ADR-0022 (production sessions under `~/.ghx`, consent is "founder dogfood
only" currently), `internal/sidecar/evals/episode.go` (Episode struct:
TurnRecord with Thinking + ToolTraces, Actions, Observations, RewardBreakdown,
ContextAccounting, Anomalies, ExclusionReasons, Identity).

---

## Q1: Official Formats for Agentic SFT Data with Tool Use

### OpenAI Chat Completions SFT JSONL

VERIFIED (https://developers.openai.com/api/docs/guides/supervised-fine-tuning,
accessed 2026-07-05):

Each JSONL line is a JSON object:

```json
{
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "tool_calls": [
      {"id": "call_abc", "type": "function",
       "function": {"name": "ghx_explore", "arguments": "{\"repo\":\"...\"}"}}
    ]},
    {"role": "tool", "content": "...", "tool_call_id": "call_abc"},
    {"role": "assistant", "content": "Final answer..."}
  ],
  "tools": [{"type": "function", "function": {"name": "ghx_explore", ...}}]
}
```

Key properties:
- `arguments` is a **JSON-encoded string**, not an object — serialization trap.
- `tool_call_id` links tool response back to the call.
- Multi-turn: the full episode is one JSONL row; the messages array carries all
  turns in order.
- Reasoning/thinking: not in the standard format. The new Harmony format
  (gpt-oss-20b tokenizer) adds `"thinking"` as an assistant message field
  alongside separate `analysis`/`final`/`commentary` channels, but this is a
  single model family and not ecosystem-wide (VERIFIED via HF TRL dataset_formats
  docs, accessed 2026-07-05).
- Minimum 10 training examples required.

### HF TRL / SFTTrainer Conversational Format

VERIFIED (https://huggingface.co/docs/trl/dataset_formats, accessed
2026-07-05):

```json
{
  "messages": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "tool_calls": [
      {"type": "function",
       "function": {"name": "ghx_explore", "arguments": {"repo": "..."}}}
    ]},
    {"role": "tool", "name": "ghx_explore", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "tools": [{"type": "function", "function": {...}}]
}
```

Key differences from OpenAI format:
- `arguments` may be an **object** (not necessarily a JSON string) in TRL
  dataset representation; the chat template serializes it.
- `"role": "tool"` message carries `"name"` field (function name) in addition
  to `"content"`.
- `tools` column contains list of JSON schema objects.
- SFTTrainer auto-applies the model's chat template; the template is the
  normalization layer between this representation and token sequences.
- Reasoning/thinking: representable as `<think>...</think>` tags inside the
  `content` field (Qwen3, DeepSeek-R1 style) or via the Harmony `"thinking"`
  key — depends on the target model family's template.
- `assistant_only_loss=True` is required to train on assistant turns only
  (critical — otherwise token loss bleeds into user messages and tool outputs).
- Native tool-calling support added in trl 0.19.0.

### LLaMA-Factory ShareGPT Format

VERIFIED (https://github.com/hiyouga/LlamaFactory/blob/main/data/README.md,
accessed 2026-07-05 via search result content):

```json
{"conversations": [
  {"role": "human", "content": "..."},
  {"role": "function", "content": "{\"name\": \"ghx_explore\", \"args\": {...}}"},
  {"role": "observation", "content": "tool output..."},
  {"role": "gpt", "content": "final answer..."}
]}
```

- `human`/`observation` on odd positions; `gpt`/`function` on even positions.
- The model learns from `gpt` and `function` turns only.
- Less standardized than the OpenAI/HF schema; more framework-specific.

### Anthropic Fine-Tuning Availability

VERIFIED (search results, accessed 2026-07-05): Anthropic exposes no fine-tuning
through its public API as of mid-2026. The only path is Claude 3 Haiku SFT on
Amazon Bedrock (us-west-2). This means a custom ghx-sidecar model cannot be an
Anthropic-hosted model — any trained model will be open-weight (Qwen, Llama,
Mistral, etc.) served locally or via vLLM.

### Format Survival Matrix

| Requirement | OpenAI JSONL | TRL conversational | LLaMA-Factory ShareGPT |
|---|---|---|---|
| Multi-turn | Yes | Yes | Yes |
| Tool call + result | Yes | Yes | Yes (function/observation) |
| Thinking/reasoning | Harmony only (gpt-oss) | `<think>` tags or Harmony | Via content tags |
| Ecosystem momentum | High (single provider) | High (open-ecosystem) | Medium |
| Portable to vLLM | Requires conversion | Yes (chat_template native) | Yes (via template) |

INFERRED: The TRL conversational format with `<think>` tags in content is the
most portable for a small open-weight model served on vLLM because (a) most
strong small reasoning models (Qwen3, DeepSeek-R1-Distill) already use
`<think>` tags, (b) TRL's SFTTrainer handles it natively, and (c) vLLM serves
it with no format conversion.

---

## Q2: Preference/Reward Formats

### Outcome-Level Preference (DPO / ORPO / SimPO)

VERIFIED (https://huggingface.co/docs/trl/dataset_formats, accessed 2026-07-05):

```json
{
  "prompt": [{"role": "user", "content": "..."}],
  "chosen": [{"role": "assistant", "content": "..."}],
  "rejected": [{"role": "assistant", "content": "..."}]
}
```

- Two trajectories for the same task, chosen = higher overall reward.
- DPO: needs a frozen reference model.
- ORPO: no reference model (single model in memory) — favorable for constrained
  compute.
- SimPO: length-normalized reward margin; no reference model.
- Our production path: one episode per task/profile generates a pair when
  two profiles (ghx-sidecar vs plain, or two ghx-sidecar runs) differ in
  overall reward.

### Unpaired Binary Preference (KTO)

VERIFIED (TRL docs, accessed 2026-07-05):

```json
{
  "prompt": [...],
  "completion": [...],
  "label": true
}
```

- One trajectory + one binary label; no pairing required.
- Maps directly to our episode reward filter: `label = (overall >= threshold)`.
- More data-efficient when pairs are scarce (single high-reward episode is a
  training example without needing a rejected counterpart).

### Step-Level / Process Supervision (PRMTrainer)

VERIFIED (TRL docs stepwise supervision, accessed 2026-07-05;
AgentProcessBench paper https://arxiv.org/html/2603.14465, accessed 2026-07-05):

```json
{
  "prompt": "repo question...",
  "completions": ["step text 1", "step text 2", "step text 3"],
  "labels": [true, false, false]
}
```

- TRL's PRMTrainer consumes this format directly.
- Each "completion" is one tool-call step (action + observation summary +
  reasoning).
- AgentProcessBench uses ternary labels (+1/0/-1): correct / neutral /
  incorrect. The +1/0/-1 must be collapsed to binary (True/False) for TRL
  PRMTrainer, or handled via a custom loss.
- Our ToolCallTrace + Anomaly data already captures per-turn anomalies and
  reward breakdowns, giving a natural per-step signal source without
  additional human labeling.

### OpenAI RFT Graders

VERIFIED (https://cookbook.openai.com/examples/reinforcement_fine_tuning,
accessed 2026-07-05; https://www.agentic-patterns.com/patterns/agent-reinforcement-fine-tuning/,
accessed 2026-07-05):

- OpenAI RFT adapts reasoning models using a programmable grader endpoint
  that scores each candidate response.
- Grader can be deterministic (unit-test pass, exact match) or LLM-based.
- Tool calls and graders are available as endpoints during training, enabling
  closed-loop agent tuning.
- RFT should be avoided when rewards are subjective/noisy or tools lack
  deterministic verifiers — not our situation; our rewards (correctness,
  evidence, trajectory) are deterministic or calibrated.
- Constraint: RFT is available only on OpenAI reasoning models, not open-weight
  models. Exporting episodes as RFT training examples would lock us into
  OpenAI's training pipeline.

INFERRED: Our deterministic rewards map cleanly to grader endpoints IF using
OpenAI RFT. For open-weight models (the likely path given the Anthropic
fine-tuning constraint), the equivalent path is GRPO with a deterministic
reward function — same reward logic, different training loop.

### GRPO for Agent RL

INFERRED from multiple sources (ToolPRMBench, AgentProcessBench, QUEST
abstracts, accessed 2026-07-05): GRPO (Group Relative Policy Optimization)
with deterministic reward functions has become the dominant RL training method
for small open-weight agent models in 2025-2026. It requires only a prompt
dataset and a reward function; no reference model or preference pairs. Our
six rewards (correctness, evidence, trajectory, compression, memory, safety)
are already Go functions; wrapping them as a Python grader for GRPO is
straightforward. AgentProcessBench showed GRPO on 800 step-labeled trajectories
improved StepAcc from 55.3% to 74.6%.

---

## Q3: Trajectory Quality Filtering Practice

### Structural / Format Filters

VERIFIED (from GEM paper, https://arxiv.org/html/2601.10355v1, accessed
2026-07-05; LLaMA-Factory full-parameter training via the same pipeline):

- Well-formed JSON schema compliance check.
- Proper role sequencing (no orphaned tool responses, no empty tool calls).
- Minimum turn count: discard trajectories shorter than N turns (N=2–3
  observed in literature; prevents degenerate "stopped immediately" episodes).
- Valid tool argument structure.

Our Episode schema already enforces most of these via the `ExclusionReasons`
and `Anomalies` fields.

### Reward-Threshold Filter

INFERRED from literature norms and our own reward model:

- Keep episodes where `overall >= threshold` (e.g., ≥ 0.6 for SFT cold start,
  ≥ 0.75 for later distillation rounds).
- Per-dimension floor: episodes with `correctness < 0.5` are poor SFT
  candidates regardless of other scores; they teach wrong answers.
- Safety = 1.0 hard requirement: episodes with safety < 1.0 must be excluded
  regardless of other rewards (ADR-0016 non-negotiable).

### Decontamination vs Eval Sets

VERIFIED (search results citing n-gram matching practice, accessed 2026-07-05):

- Standard practice: 13-gram overlap or Ratcliff-Obershelp on token sequences
  between training examples and benchmark prompts; exclude if overlap > 0.5.
- Our specific risk: eval tasks live in `testdata/tasks/`; their `turns`
  (question text) and `checks.requiredClaims` must not appear in training
  data. The `Task.Validate()` already enforces that checks are not leaked
  into question text — the same anti-leak principle applies to training-set
  filtering.
- Production sessions probe arbitrary repos; decontamination is against the
  eval task corpus, not against all GitHub content.

### Anomaly Exclusion

VERIFIED (ADR-0016.1/0016.7, our codebase): Episodes with `Invalid: true` or
`Anomalies` of class BLOCKED or WARN-noreport are already excluded from
gate scoring. The same filter must gate training inclusion. The `ExclusionReasons`
field is the durable hook for this.

### Judge-Label Threshold

VERIFIED (ADR-0023.1 D5 sequencing rule): M9 exports may use judge labels as
trajectory-quality signals only after calibration passes (κ ≥ 0.6, ≥ 200
labeled episodes). Pre-D5, judge scores are PRELIMINARY and must not gate
training inclusion. Post-D5, judge score can replace or complement overall
reward as the quality filter.

### Dedup

INFERRED: Near-duplicate episodes (same task, same repo state, nearly identical
tool-call sequences) add noise without new signal. Dedup on (taskID, toolCall
sequence hash, report summary embedding similarity > 0.95) recommended before
export; exact dedup at minimum.

### Distillation Practice (Frontier → Small Model)

VERIFIED (agent-distillation repo, https://github.com/Nardien/agent-distillation,
accessed 2026-07-05; AgentArk paper https://arxiv.org/html/2602.03955v1,
accessed 2026-07-05; SOD paper https://arxiv.org/html/2605.07725v1, accessed
2026-07-05):

- 20% fully teacher-annotated for high-quality cold-start SFT; 80% generated
  via specialized processes (capability-matched, deficiency-localized).
- AgentArk: multi-stage — Data Generation → Knowledge Extraction → Distillation
  with Standard SFT + Reasoning-enhanced SFT + Process-Aware Distillation.
- SOD: step-wise on-policy distillation; the student generates, the teacher
  corrects; outperforms behavior cloning on long-horizon tasks.
- Our analog: ghx-sidecar episodes with frontier model (current Sonnet/Opus
  brain) are the teacher trajectories; the trained small model is the student.

---

## Q4: Open-Source Tooling to Steal

### distilabel (Argilla)

VERIFIED (https://distilabel.argilla.io, accessed 2026-07-05 via search):

- Synthetic data generation + augmentation pipeline.
- Outputs JSONL ready for TRL, Axolotl, Unsloth, LLaMA-Factory, or OpenAI
  fine-tuning API.
- Supports UltraFeedback and DEITA techniques for SFT/DPO data generation.
- Does NOT directly ingest OTel JSONL traces; it generates new data from
  existing datasets or prompts.
- Steal: the pipeline pattern (source → transform → quality-filter → export
  JSONL); the UltraFeedback scoring approach for building preference pairs.

### TRL Dataset Utilities

VERIFIED (TRL docs, accessed 2026-07-05):

- `trl.extract_prompt` and `trl.unpair_preference_dataset`: conversion
  helpers between preference types.
- `examples/datasets/` scripts: standard conversions.
- `PRMTrainer`: step-level process supervision — directly usable with our
  per-turn ToolCallTrace data.
- `DPOTrainer`, `KTOTrainer`, `ORPOTrainer`, `GRPOTrainer`: all accept the
  standard conversational format.
- Steal: PRMTrainer stepwise supervision format; KTO unpaired preference
  as the low-friction first step (single episode = one training example).

### AgentTrace

VERIFIED (https://arxiv.org/abs/2602.10133, AAAI 2026 TrustAgent Workshop,
accessed 2026-07-05):

- Structured logging framework with three-surface taxonomy: cognitive
  (reasoning), operational (tool calls, errors), contextual (environment).
- OTel-compatible OTLP JSON; monkey-patches standard libraries at runtime.
- A trace processor converts OTEL JSONL into compact per-trace JSON artifacts
  by filtering to agent-relevant spans.
- Steal: the three-surface taxonomy as a framing for what fields to promote
  into training examples (thinking → cognitive, ToolCallTrace → operational,
  session context → contextual). Our episode schema already captures all three.

### LLaMA-Factory

VERIFIED (GitHub README, accessed 2026-07-05):

- Accepts ShareGPT format with tool calls natively.
- Full-parameter and LoRA fine-tuning.
- Supports GRPO and DPO training loops in addition to SFT.
- Steal: use as a training runner over our exported JSONL with minimal
  custom code; the ShareGPT conversion from our Episode is a straightforward
  Go function.

### EvalAgent / EVALAGENT

INFERRED from AgentTrace paper and Bittensor paper (accessed 2026-07-05):

- A trace processor pattern converts OTEL JSONL into compact per-trace JSON
  artifacts by filtering to agent-relevant spans and extracting a minimal
  field set.
- This is exactly the conversion we need: `traces.jsonl` →
  `sft.jsonl` / `preferences.jsonl`.
- No single off-the-shelf library does this for our exact episode schema;
  a thin Go or Python converter will be required. The conversion is simple:
  iterate Episode actions/observations/thinking → messages array; attach tools;
  emit rewards as metadata.

### Axolotl

INFERRED (referenced in multiple papers and blogs, accessed 2026-07-05):

- Supports ShareGPT and OpenAI chat formats; LoRA/QLoRA; FSDP multi-GPU.
- Accepts OTel-style structured logs indirectly via a dataset pre-processor.
- Steal: use for LoRA fine-tuning of a small model (7B class) as an
  alternative to LLaMA-Factory when multi-GPU FSDP is needed.

### Gaps (Hand-Roll Required)

INFERRED: No existing tool directly converts our `episode.json` (or
`traces.jsonl`) into the TRL conversational JSONL with `tools` column. A thin
converter is the minimum custom code:

```
Episode{Turns[].Thinking + ToolTraces, Actions, Observations, Report, Rewards}
  → {"messages": [...tool_calls + tool + assistant turns...],
     "tools": [...],
     "metadata": {"rewards": {...}, "profile": "...", "taskId": "..."}}
```

This converter is the core export artifact for M9. It should emit to all three
formats (SFT, preference, reward) from the same episode source, parameterized
by filtering thresholds. It requires no novel research; it is plumbing.

---

## Q5: Serving-Side Constraint Check (vLLM)

VERIFIED (https://docs.vllm.ai/en/latest/features/tool_calling/, accessed
2026-07-05):

### What vLLM Requires for Tool Calling at Inference

1. A Jinja2 chat template that handles `tool`-role messages and `assistant`
   messages containing `tool_calls`. Either:
   - Embedded in `tokenizer_config.json` (automatic for Qwen 2.5, Llama 3.1+,
     Mistral, Hermes, DeepSeek families), or
   - Passed via `--chat-template path/to/template.jinja`.
2. A tool-call parser flag: `--tool-call-parser hermes` / `llama3_json` /
   `qwen25` / etc. Each parser knows how the specific model family encodes
   tool calls in its output tokens.
3. `--enable-auto-tool-choice` for automatic tool selection (optional; can
   be required).

### Training → Serving Consistency Constraint

INFERRED: If we fine-tune model X for tool calling, the training data must use
the **same chat template** that vLLM will apply at inference. Training on one
template format and serving on another causes format confusion and degraded
tool-call reliability. Concretely:

- If we use Qwen 2.5 (7B or 14B) as the base: train with Qwen's chat template,
  serve with `--tool-call-parser qwen25`.
- If we use Llama 3.1 (8B): train with Llama's tool-calling template
  (`llama3_json`), serve with `--tool-call-parser llama3_json`.
- Tool definitions are passed via the API's `tools` parameter at inference,
  NOT baked into the training data — the chat template handles injecting them
  into the context. Training examples should include the `tools` column so
  the model learns to respect the tool schema structure, but individual tool
  names/signatures used during training need not be the exact ghx tool names
  (though using real names improves grounding).

### Format Dead-End Risk

INFERRED: Using a non-standard template during training (e.g., a custom XML
format) without a matching vLLM parser would require a custom tool-call parser
plugin. This is avoidable by choosing a base model whose template is already
supported by vLLM's built-in parsers. This is the "official formats exactly"
tenet applied to the serving layer.

---

## Q6: Licensing and Consent Hygiene

### GitHub Public Repo Trajectory Data

VERIFIED (search results, IT Pro, GitLab blog, accessed 2026-07-05):

- GitHub's April 2026 policy change: interaction data from Copilot Free/Pro/Pro+
  users used for AI training by default (opt-out available). Copilot Business
  and Enterprise users are exempt.
- "Public = free for AI" norm is actively contested. Developer communities are
  asserting opt-out rights via LICENSE files and `.github/copilot-ignore`.
- Emerging norm: treat opt-out as a release checklist item; unambiguous LICENSE
  language preferred.

INFERRED for ghx: Our trajectories record the **sidecar agent's exploration
process** (commands, outputs, reasoning) over public GitHub content, not the
repository content itself. The legal status is analogous to a search engine's
index cache of public content:
- The repo code explored is already public.
- The trajectory is a record of our agent's behavior, not a copy of the code.
- Our training data teaches the model how to use ghx tools, not what specific
  repos contain.

This distinction reduces (but does not eliminate) licensing risk. However,
training samples that include large verbatim excerpts of explored file content
as tool observations inherit the license of the source repo. Truncating tool
observations at a summary/excerpt boundary (as `OutputExcerpt` in our
`ToolCallTrace` already does at 2048 bytes) keeps us in clearly commentary
territory.

### Dogfood Session Consent

VERIFIED (ADR-0022): "Consent implied for founder dogfood only." This is
explicitly scoped to the current single-user phase.

INFERRED: Before M9 training uses production sessions, a consent model for
any third-party ghx users must be designed. The minimum viable norm for any
future user onboarding: explicit disclosure that sessions may be used for
model improvement, a per-session opt-out flag (`GHX_NO_TRAINING=1` or
equivalent), and a data deletion path.

### Eval Episode Consent

INFERRED: Eval episodes run synthetic tasks against public GitHub repos using
our own agent. No third-party user session data is involved. No consent issue;
the training data is our agent's own behavior, targeting public information.

---

## Candidate Design Constraints for the Decision ADR

These are constraints — necessary properties any design must satisfy — not
design choices. The ADR will make choices within them.

**C1. Format: TRL conversational JSONL with `tools` column is the primary
export target.** It is the most portable open-ecosystem format, directly
ingestible by TRL, LLaMA-Factory (via conversion), Axolotl, and OpenAI
fine-tuning API. Any export pipeline must be trivially convertible to this
format; other formats (ShareGPT, OpenAI JSONL) should be derivable from it.

**C2. No Anthropic-API fine-tuning path.** Anthropic does not expose fine-tuning
through its public API. Any trained ghx-sidecar model will be open-weight,
served locally (or via vLLM). The export format must not assume an Anthropic
training stack.

**C3. Chat template and training format must be chosen together with the serving
stack.** The base model family (Qwen, Llama, Mistral, Hermes, etc.) determines
the chat template; the template must be consistent across SFT training and vLLM
serving. Choosing the base model is a constraint on the export schema, not an
afterthought.

**C4. Reasoning/thinking content must be preserved in exports.** Our
`TurnRecord.Thinking` field captures ACP agent reasoning. This is the
sidecar-internal SPT signal (ADR-0016.6) and the most valuable distillation
target. Training on thinking-visible trajectories is a known path to strong
small reasoning models (DeepSeek-R1-Distill, Qwen3 distillation). The export
format must retain `Thinking` in the assistant message, using either `<think>`
tags in `content` or a separate `thinking` key, whichever matches the target
model's template.

**C5. Safety-violation episodes must be hard-excluded.** Episodes with
`safety < 1.0` or `violations` non-empty must never enter the training corpus.
This is not a quality threshold but a correctness constraint: training on
write-attempt or permission-escalation trajectories would teach the model to
replicate violations.

**C6. Judge labels gate preference/reward exports, not SFT exports.** SFT
exports (high-reward episodes as positive examples) can proceed after M9
trajectory accumulation. Preference pairs and reward-model training that
depend on judge-score-as-quality-signal must wait for ADR-0023.1 D5
calibration to pass. The export pipeline must support this two-phase structure
via a filter flag on export, not by delaying the entire M9 work.

**C7. Decontamination against the eval task corpus is mandatory before any
training run.** Overlap between training episodes and benchmark task prompts
invalidates eval results. The converter must accept the task corpus as input
and exclude episodes whose question text or tool outputs overlap with task
`turns` above a configurable n-gram threshold (13-gram matching is the
established norm).

**C8. The export pipeline is a Go or Python converter over committed Episode
JSON artifacts, not a new service.** It reads from `.ghx-evals/runs/` and
`~/.ghx/sessions/` (ADR-0022 layout), filters by reward thresholds and
anomaly flags, emits JSONL. No new runtime dependency; no hosted service.

**C9. Consent model must be explicit before any non-founder production session
is used for training.** The current "founder dogfood only" scope in ADR-0022
is the hard consent boundary. Training corpus must be tagged by source
(eval/dogfood/production) so the export can enforce this boundary.

**C10. Harness-neutral exports.** The JSONL format must not embed ACP-specific
identifiers or adapter-specific token sequences. The training corpus should
survive the P4 transition from ACP to a direct agent SDK. Episode identity
fields (`AdapterName`, `AgentCommand`) belong in metadata, not in the message
sequence.

---

## Open Questions

**OQ-1. Base model choice.** Which small model (Qwen 2.5 7B/14B, Llama 3.1 8B,
Mistral 7B, Hermes 3) gives the best tool-calling + reasoning quality per
inference cost for the ghx tool surface? This determines C3's constraint on
the training format. Cannot be resolved without a model evaluation against
ghx task types. Precedent: Qwen 2.5 and Hermes 3 have the strongest
out-of-box tool-calling templates in vLLM as of mid-2026.

**OQ-2. GRPO vs SFT+DPO sequencing.** For a small model starting from scratch
on ghx tools, is cold-start SFT (high-reward episodes) followed by GRPO
(deterministic rewards as live environment) the right recipe, or SFT+DPO
(preference pairs) without RL? Literature suggests GRPO is stronger for
agentic tasks but requires a live grader; SFT+DPO is simpler without needing
an online rollout environment. Resolved only by a small-scale training
experiment at M9.

**OQ-3. How many trajectories are needed for M9 before training is viable?**
The SOD and AgentArk papers suggest 5k–35k trajectories for meaningful
distillation; our eval suite at 6 tasks × 5 trials × 3 profiles = 90 episodes
per run is far below this. M9 requires a trajectory accumulation strategy
(more tasks, more trials, or synthetic augmentation). What is the minimum
viable corpus size before a first training experiment? Not resolved here.

**OQ-4. Thinking visibility vs training efficiency.** Including reasoning chains
in training increases context length (and cost) by 3–10×. Is per-step thinking
always worth including, or should it be selectively included based on turn
difficulty (e.g., only turns where the agent's exploration was non-trivial)?
Not resolved; depends on OQ-1 base model's reasoning template.

**OQ-5. Step-level PRM labeling automation.** Our `RewardBreakdown` is
trajectory-level. Generating step-level labels for PRMTrainer requires mapping
per-turn anomalies and tool-call traces to binary/ternary labels. The
AgentProcessBench approach (error-propagation rule: once a bad step happens,
all dependent steps get -1 until correction) is directly applicable to our
Anomaly taxonomy. Is this automation sufficient, or does it require founder
hand-labeling at calibration time (analogous to ADR-0023.1 D5)?

**OQ-6. Verbatim file content in tool observations.** Our `OutputExcerpt` is
capped at 2048 bytes. Is this sufficient to keep training data in "commentary"
territory for licensing purposes, or does any repo-specific file content
require explicit exclusion (e.g., GPL-licensed repos)?

---

## Sources (Primary, Verified)

URLs verified accessible on 2026-07-05:

1. OpenAI SFT docs: https://developers.openai.com/api/docs/guides/supervised-fine-tuning
2. TRL SFTTrainer docs: https://huggingface.co/docs/trl/sft_trainer
3. TRL Dataset Formats (tool calling section): https://huggingface.co/docs/trl/dataset_formats#tool-calling
4. vLLM Tool Calling docs: https://docs.vllm.ai/en/latest/features/tool_calling/
5. AgentTrace paper: https://arxiv.org/abs/2602.10133 (AAAI 2026 TrustAgent Workshop)
6. AgentProcessBench: https://arxiv.org/html/2603.14465
7. AgentPRM paper: https://arxiv.org/html/2511.08325v1
8. ToolPRMBench: https://arxiv.org/html/2601.12294v1
9. OpenAI RFT Cookbook: https://cookbook.openai.com/examples/reinforcement_fine_tuning
10. GEM / Unlocking Implicit Experience: https://arxiv.org/html/2601.10355v1
11. AgentArk: https://arxiv.org/html/2602.03955v1
12. Agent Distillation (Nardien): https://github.com/Nardien/agent-distillation
13. SOD (Step-wise On-policy Distillation): https://arxiv.org/html/2605.07725v1
14. LLaMA-Factory README: https://github.com/hiyouga/LlamaFactory/blob/main/data/README.md
15. Agentic Reward Modeling paper: https://arxiv.org/pdf/2502.19328
16. Philschmid DPO/RLHF in 2025 guide: https://www.philschmid.de/rl-with-llms-in-2025-dpo
17. GitHub AI training policy coverage: https://www.itpro.com/software/development/four-things-you-need-to-know-about-githubs-ai-model-training-policy-including-how-to-opt-out
18. Anthropic Claude fine-tuning status: https://aws.amazon.com/blogs/aws/fine-tuning-for-anthropics-claude-3-haiku-model-in-amazon-bedrock-is-now-generally-available/
19. Decontamination practices: https://arxiv.org/html/2601.04301
20. QUEST paper: https://arxiv.org/pdf/2605.24218

## What Could Not Be Verified

- **Exact OpenAI multi-turn tool-call JSONL schema with thinking/reasoning**: The standard OpenAI SFT docs do not show reasoning content in tool-call trajectories. Harmony format is documented for gpt-oss-20b specifically but ecosystem-wide adoption is INFERRED, not confirmed.
- **Anthropic fine-tuning roadmap**: Whether Anthropic will expose fine-tuning through its public API in 2026 beyond Bedrock/Haiku is unconfirmed. The search results show only historical Bedrock status; no official roadmap statement.
- **QUEST training data format specifics**: The QUEST paper abstract was accessible but the full technical methodology on trajectory format and filtering was not extractable from the abstract alone.
- **Distilabel direct OTel ingestion**: Distilabel's support for ingesting raw OTel JSONL and emitting training data was not confirmed from primary source (docs were referenced but not directly fetched). The claim is INFERRED from secondary sources.
- **vLLM tool-call parser for a custom fine-tuned model (non-standard family)**: If the trained ghx-sidecar model is fine-tuned to use a novel tool-call encoding (not matching Hermes/Llama/Qwen families), a custom vLLM parser plugin would be required. The docs describe only built-in parsers; custom parser documentation was not reviewed.
- **Exact n-gram threshold for decontamination that is calibrated for ghx task length**: The 13-gram threshold is published norm but may need adjustment for short ghx task prompts (our `turns` are 1–2 sentences, not multi-paragraph).
