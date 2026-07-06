---
title: "ADR-0018: Agentic Observability — Official OTel GenAI Conventions, Traces, Metrics, Content"
date: "2026-07-05"
status: "proposed — research phase; implementation lands after the confirmatory gate run"
thread: "agentic-observability"
author: "Goga Koreli"
---

# 0018. Agentic Observability — Official OTel GenAI Conventions

## Status

Proposed. Opened at Goga's direction during the 2026-07-05 confirmatory
gate run, after hand-auditing traces in a viewer exposed how far our
emission is from full visibility. This is a new ADR thread (not 0016.x):
observability of agentic execution is its own competence area, spans both
evals and the production sidecar, and seeds NORTH_STAR M6 (shared SAF/SAFE
trace infrastructure). ADR-0016.4 chose the OTLP JSON file transport; this
thread governs *what* we emit on it.

## Context: the founder hand-audit found the gaps

Goga inspected live gate-run traces in otel-desktop-viewer (2026-07-05)
expecting full visibility — agent thinking, tool uses, reasoning, scores
with explanations — and got a skeleton. Verified root causes, each a real
emission gap on our side, none a viewer bug:

1. **Agent reasoning is dropped at capture.** ACP streams
   `agent_thought_chunk` session updates; `internal/sidecar/acp.go` has no
   handler for them (verified: zero matches for thought/thinking in the
   capture path). Thinking never reaches artifacts, spans, or scores.
2. **Zero metrics.** We emit traces only; the viewer's metrics page is
   empty because nothing produces OTel metrics — no token usage, no
   operation durations, no reward distributions.
3. **Span content is titles, not content.** Spans carry tool commands and
   char counts as attributes, but no message content events — prompts,
   completions, reports, and tool outputs are only in the episode JSONs,
   so a trace viewer shows *that* things happened, not *what* was said.
4. **Scores are opaque in-trace.** `eval.reward.compute` carries final
   numbers; the per-check breakdown (which expected file matched, which
   claim hit, why trajectory lost 0.13) lives only in rewards.go logic.
5. **Our gen_ai.* usage is ad-hoc.** We set a few attributes
   (`gen_ai.operation.name`, `gen_ai.request.model`) but do not follow
   the official OTel GenAI semantic conventions for agent spans, content
   events, or metrics — which is exactly the "hand-rolled dialect" the
   open-source-leverage tenet (AGENTS.md) prohibits.
6. **Wrong viewer class for the job.** otel-desktop-viewer is a generic
   span-timing tool. LLM-native viewers (Arize Phoenix — already named in
   ADR-0016.4's viewer guidance) render prompts/completions/reasoning as
   conversations, but only if we emit content per conventions they
   understand.

## Decision (proposed)

Adopt the **official OTel GenAI semantic conventions** as the emission
contract for every agentic surface — eval episodes, production sidecar
sessions, and (later) judge scorers:

1. **Traces**: agent/tool spans use the official `gen_ai.*` attribute set
   and span naming; message content (system/user prompts, assistant
   output, tool results, and captured reasoning) is emitted per the
   conventions' content-capture mechanism, opt-in via the standard
   environment control so PII posture stays explicit.
2. **Reasoning capture**: handle ACP `agent_thought_chunk` in the capture
   path, persist thinking on `TurnRecord`, and emit it as reasoning
   content per conventions. (Product change — post-run.)
3. **Metrics**: emit the conventions' client metrics (token usage,
   operation duration) where derivable, plus ghx-native metrics: reward
   components as histograms per profile/task, anomaly counts by kind,
   report size distributions. File-first transport like traces
   (`metrics.jsonl`, OTLP JSON) plus the standard OTLP endpoint override.
4. **Score explanations in-trace**: reward spans gain per-check events
   (matched/missed expected file, claim, penalty applied) so a score is
   auditable inside the trace, not only by rereading rewards.go.
5. **Viewer guidance**: Phoenix as the LLM-native default for content-
   level inspection; otel-desktop-viewer stays documented for quick span
   timing. No custom viewer unless both fail a concrete need (tenet).

## Research findings (delegated worker memo, 2026-07-05; re-verify against the live spec at implementation time)

A web-research pass against current official documentation (OTel semconv
GenAI docs, file-exporter spec, Phoenix docs, ACP schema) returned these
load-bearing facts, each changing part of the original proposal:

1. **Content capture is Log events now, not span events.** Since semconv
   ~v1.37 the per-role span events (`gen_ai.system.message` etc.) are
   superseded: content is emitted via the **Logs API** as a
   `gen_ai.client.inference.operation.details` event on the active span,
   carrying `gen_ai.input.messages`/`gen_ai.output.messages` as
   `{role, parts}` JSON. Consequence: we need the Logs signal (emission +
   a `logs.jsonl` file exporter) in addition to traces and metrics.
2. **Scores have an official home**: the `gen_ai.evaluation.result` event
   (`gen_ai.evaluation.name`, `gen_ai.evaluation.score.value`). Our reward
   components and per-check explanations map onto it — no homegrown score
   event needed. The future judge scorer emits the same event.
3. **Reasoning**: no dedicated reasoning event exists; thinking is a
   `{"type": "reasoning"}` part inside output messages, plus
   `gen_ai.usage.reasoning.output_tokens` on the span. **Adapter verdict
   (source-level recon of the shipped `claude-agent-acp@0.55.0` tarball,
   2026-07-05): it DOES emit `agent_thought_chunk`** — for non-empty
   `thinking`/`thinking_delta` blocks (`dist/acp-agent.js:3880`), gated by
   SDK-level thinking being enabled: `MAX_THINKING_TOKENS` env (`0` →
   disabled, positive → budget; unset → SDK default) or
   `_meta.claudeCode.options.thinking`. Subagent thinking is intentionally
   filtered (`:1731`). The adapter also offers a raw-SDK firehose via
   `_claude/sdkMessage` extension notifications (`emitRawSDKMessages`) —
   adapter-specific, useful for eval telemetry but not portable ACP.
   Upstream docs do not specify any of this (their issue tracker shows
   extended-thinking enablement as an open question), which is why the
   shipped source was the authority. Consequence for capture scope: our
   `acp.go` must handle the `agent_thought_chunk` session-update kind
   (today it is silently dropped), and the eval wrapper should set
   `MAX_THINKING_TOKENS` so subject-agent reasoning is reliably present.
4. **Content is off by default**:
   `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true` gates it.
   Eval runs set it (our artifacts already store full text; traces should
   match); production sidecar sessions leave it off unless the user opts
   in.
5. **Metrics**: instruments `gen_ai.client.operation.duration` (s) and
   `gen_ai.client.token.usage` ({token}, split by `gen_ai.token.type`),
   plus our ghx-native reward/anomaly metrics. There is **no official Go
   OTLP file exporter**; the OTel file-exporter spec defines the exact
   JSONL encoding (protobuf JSON mapping, **hex IDs** — confirming our
   02c663c fix was spec-correct). A custom exporter must follow that spec
   exactly, mirroring the traces one.
6. **Viewer reality check**: otel-desktop-viewer v0.3.2 ingests traces,
   metrics, and logs (metrics/logs rendering maturity unverified — test
   early). Phoenix ingests OTLP at `:6006` but natively renders
   OpenInference conventions; GenAI-semconv spans from Go get a degraded
   (though queryable) UI, and no Go GenAI→OpenInference translator
   exists. Consequence: desktop viewer stays the primary local surface;
   Phoenix is secondary until its GenAI rendering matures or a thin
   mapping layer proves worth it (open-source-leverage tenet: prefer
   waiting on upstream over building a dialect converter).

## Sequencing and constraints

- **Nothing lands mid-gate-run.** Capture changes touch
  `internal/sidecar` (product); the confirmatory run keeps one build.
  Implementation starts after the run's verdict is committed.
- Natural order: research → finalize this ADR → implement capture +
  conventions on the eval path → extend to production sidecar sessions
  (this *is* the M6 slice) → judge scorer (ADR TBD) inherits the
  conventions for its reasoning traces.
- The declarative anomaly layer (ADR-0016.7 addendum) is compatible
  forward: anomaly span events fold into the conventions' event model.

## Cross-references

- ADR-0016.4 — chose OTLP JSON file transport + named Phoenix; this ADR
  governs semantic content on that transport.
- AGENTS.md "Open Source Leverage" — official formats exactly; this ADR
  is that tenet applied to telemetry semantics.
- AGENTS.md "Visibility and Truthfulness" — judge scorers require full
  reasoning traces; those will be GenAI-convention traces.
- NORTH_STAR M6 — shared trace infrastructure / full visibility surface.
