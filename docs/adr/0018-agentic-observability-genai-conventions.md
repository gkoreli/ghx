---
title: "ADR-0018: Agentic Observability — Official OTel GenAI Conventions, Traces, Metrics, Content"
date: "2026-07-05"
status: "accepted — eval telemetry implementation landed; production-session reuse remains follow-up"
thread: "agentic-observability"
author: "Goga Koreli"
---

# 0018. Agentic Observability — Official OTel GenAI Conventions

## Status

Accepted. Opened at Goga's direction during the 2026-07-05 confirmatory
gate run, after hand-auditing traces in a viewer exposed how far our
emission was from full visibility. The first implementation slice landed
after the confirmatory verdict: eval episodes now emit OTLP JSON traces,
logs, and metrics using the GenAI semantic-convention names recorded below,
and ACP `agent_thought_chunk` reasoning is captured into episode artifacts.

This remains a live ADR thread (not 0016.x): observability of agentic
execution is its own competence area, spans both evals and the production
sidecar, and seeds NORTH_STAR M6 (shared SAF/SAFE trace infrastructure).
ADR-0016.4 chose the OTLP JSON file transport; this thread governs *what*
we emit on it.

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

## Implementation notes

Implemented eval/SAFE slice:

- ACP capture now handles `agent_thought_chunk` in both the production
  sidecar ACP client (`internal/sidecar/acp.go`) and the direct eval client
  (`internal/sidecar/evals/client.go`). Reasoning is persisted additively on
  `TurnResult.Thinking` and `TurnRecord.thinking`; replayed reasoning is
  audit-only as `replayedThinking`.
- `scripts/eval-agent-acp.sh` sets `MAX_THINKING_TOKENS=${MAX_THINKING_TOKENS:-4096}`
  for the pinned `claude-agent-acp@0.55.0` adapter.
- Content capture emits OTLP `LogsData` records into `logs.jsonl` only when
  `OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=true`. The live
  agent eval test sets this env var; normal production sidecar sessions keep
  the default-off posture.
- Content log records use event name
  `gen_ai.client.inference.operation.details` and attributes
  `gen_ai.input.messages` / `gen_ai.output.messages`. Captured thinking is
  represented as an output part with `{"type":"reasoning"}`.
- Metrics emit OTLP `MetricsData` into `metrics.jsonl`: official
  `gen_ai.client.operation.duration` (`s`) and
  `gen_ai.client.token.usage` (`{token}`, split by `gen_ai.token.type`),
  plus `ghx.eval.reward`, `ghx.eval.anomaly.count`, and
  `ghx.eval.report.size`.
- Score visibility now lives inside `eval.reward.compute`: official
  `gen_ai.evaluation.result` events carry `gen_ai.evaluation.name` and
  `gen_ai.evaluation.score.value`; ghx-namespaced check/penalty events carry
  expected files, required claims, evidence checks, repeat-read penalties,
  budget penalties, and safety violations. `Episode.checks` snapshots the
  task scoring contract so the trace explanation remains recomputable from
  committed artifacts.
- The traces/logs/metrics file writers use OTLP protobuf JSON mapping. Trace
  and log trace/span IDs are rewritten to the OTLP JSON file spec's required
  lowercase hex representation, mirroring the existing trace exporter fix.

Live validation (2026-07-05, one strict smoke episode
`ghx-mapengine/ghx-sidecar`, run `20260706-020002`, post-merge mainline):

- All three artifact files were produced (`traces.jsonl` 8 spans,
  `logs.jsonl` 1 content record, `metrics.jsonl` 4 metrics / 12 data
  points) and replayed into `otel-desktop-viewer` with 100% OTLP
  acceptance; the viewer's `getStats` RPC confirmed ingestion of every
  span, log, and data point. Replay recipe (viewer UI on :8000, OTLP
  ingest on :4318):

  ```sh
  otel-desktop-viewer --open-browser=false &
  cd internal/sidecar/evals/.ghx-evals/runs/<run-id>
  for kind in traces logs metrics; do
    while IFS= read -r line; do
      curl -s -X POST -H 'Content-Type: application/json' \
        -d "$line" "http://localhost:4318/v1/$kind" > /dev/null
    done < "$kind.jsonl"
  done
  ```

- Content capture verified live: `gen_ai.client.inference.operation.details`
  with `gen_ai.input.messages`/`gen_ai.output.messages` present and
  trace-correlated.
- Known gap: this episode's output parts were all `{"type":"text"}` — the
  adapter emitted zero `agent_thought_chunk` even with
  `MAX_THINKING_TOKENS=4096` exported by the eval wrapper. The capture
  path is unit-proven (mock agent), so the miss is on the emission side;
  the expected fix is ADR-0020.1 D2's session-level `thinking` option
  wiring. Re-verify reasoning parts on the D2 branch smoke.
- This smoke also exposed and fixed a preflight regression from the
  ADR-0019 merge: the handshake probe checked the host-configured agent
  instead of `GHX_EVAL_AGENT` and skipped live episodes
  (`RunPreflightForAgent`, commit a843e40).

Follow-up:

- Extend the same telemetry surface to long-lived production sidecar
  sessions under `~/.ghx`, not only eval episode saves.
- Phoenix rendering still unchecked (otel-desktop-viewer validated above).
- Judge scorer ADR/work should reuse `gen_ai.evaluation.result` and emit its
  prompt/model calibration alongside deterministic reward events rather than
  inventing another score format.

## Cross-references

- ADR-0016.4 — chose OTLP JSON file transport + named Phoenix; this ADR
  governs semantic content on that transport.
- AGENTS.md "Open Source Leverage" — official formats exactly; this ADR
  is that tenet applied to telemetry semantics.
- AGENTS.md "Visibility and Truthfulness" — judge scorers require full
  reasoning traces; those will be GenAI-convention traces.
- NORTH_STAR M6 — shared trace infrastructure / full visibility surface.
