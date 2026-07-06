---
title: "ADR-0022: Shared Visibility Runtime — One Telemetry Capability for SAF and SAFE, ~/.ghx Root Storage"
date: "2026-07-05"
status: "accepted"
thread: "saf-visibility"
author: "Goga Koreli"
---

# 0022. Shared Visibility Runtime

## Status

Accepted (founder directive, 2026-07-05 — restating the original design
intent: "full visibility needs to be part of the agent sidecar framework
so that we can rely on it during the runtime or during the evals; we
reuse the same exact framework capability in both places"). Governs the
M6 remainder. Semantics stay governed by ADR-0018; this ADR moves the
capability to where the north star always said it lives.

## Context

The visibility tenet (NORTH_STAR "Sidecar Product Capabilities" §3, and
the SAF/SAFE framing: "a trace is a trace whether it came from an eval
episode or a real question") promises that everything the sidecar does
is traceable, per question, per session. Today that is true only for
evals: the ADR-0018 emission layer — spec-exact OTLP JSONL for traces,
GenAI content logs, and metrics — lives in `internal/sidecar/evals`
(`otel_trace.go`, `otel_logs.go`, `otel_metrics.go`,
`otel_reward_events.go`, `otel_exporter.go`) and fires only when an
eval episode is saved. The production path a consumer touches —
`ghx sidecar ask` → `sidecar.Ask` in runtime.go — emits **nothing**
(verified 2026-07-05: zero trace references in runtime.go).

Separately, product storage sits at `~/.ghx-sidecar/` (config.go
`defaultConfigDir`), while the north star names `~/.ghx` as the
product's root storage (sessions, ledgers, reports, traces, caches,
config).

## Decisions

### D1. Extract the emission layer into a shared SAF capability

The OTel builders and OTLP JSONL writers move from `package evals` to a
shared package, `internal/sidecar/telemetry`. It owns everything
generic: session/turn spans, tool-call spans, GenAI content log
records, token/duration metrics, and the spec-exact OTLP protobuf-JSON
file writers (hex IDs and all). Package `evals` keeps only what is
eval-specific — reward spans, `gen_ai.evaluation.result` events,
per-check/penalty events, anomaly metrics — layered on top of the
shared builders.

**Extraction rule (binding):** this is a pure move for the eval path.
Eval artifact output stays byte-identical; the existing
`otel_exporter_test.go` assertions move with the code and must pass
unchanged. Scoring, rewards, and gates are untouched — this refactor
does not alter measurement conditions for the pending confirmatory
re-run.

### D2. Production sessions emit the same artifacts

Every `sidecar.Ask` turn appends to the session's artifact set using
the shared capability:

- `traces.jsonl` — session/turn/tool spans, same shapes as eval traces;
- `logs.jsonl` — GenAI content records (see D4);
- `metrics.jsonl` — `gen_ai.client.operation.duration`,
  `gen_ai.client.token.usage`, `ghx.eval.report.size` (renamed
  namespace-neutral if needed);
- `reports/<turn>-<ts>.json` — the accepted report of every question,
  persisted, not just returned to stdout.

No reward/evaluation events in production — those are SAFE's layer.
Emission failures never fail the ask (warn to stderr, degrade); the
answer is the product, the trace is the audit.

### D3. `~/.ghx` becomes the root storage

Layout: `~/.ghx/config.json`, `~/.ghx/sessions/<session>/` (meta,
ledger, traces.jsonl, logs.jsonl, metrics.jsonl, reports/). Migration:
if `~/.ghx` is absent and `~/.ghx-sidecar` exists, read the old
location and print a one-line migration notice; `config init` writes
the new location. `GHX_HOME` env overrides the root (tests use temp
dirs, never the real home). Docs (README sidecar section, ADR) update
with the new paths — any human or agent can open `~/.ghx` and see
exactly what ghx did and why.

### D4. Content capture defaults: visibility-first, locally

Traces, metrics, and reports are always on — they are the product
promise. Message-content log records (`gen_ai.input/output.messages`)
default **on** in production too, because these are local artifacts in
the user's own home directory, not telemetry exported to a backend —
the OTel opt-in default targets exported data. The official switch is
honored for turning it off
(`OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false`), plus a
config field (`visibility.captureContent`). Documented plainly so a
consumer knows what is on disk.

### D5. The viewer story ships with it

The ADR-0018 replay recipe works unchanged on any
`~/.ghx/sessions/<session>/` directory. `ghx sidecar doctor` (or the
session inspection path) mentions where the artifacts live and how to
replay them. This is the "viewer guidance after real artifact checks"
item from the M6 row.

## Considered and rejected

- **Emit only from evals until a consumer asks.** Rejected by the
  founder's original framing: visibility is a framework capability,
  not an eval feature. The dogfood week (M5 exit bar) needs it —
  friction analysis without production traces is guesswork.
- **A separate, lighter production trace format.** Rejected: one
  format, one replay story, one viewer; official OTel exactly per spec
  (open-source-leverage tenet). Divergence is how the two stacks drift.
- **Network OTLP export by default.** Rejected: local-first tenet;
  files are canonical, replay is trivial, no service dependency.
- **Content capture opt-in in production (strict OTel default).**
  Rejected for local artifacts (D4 rationale); the off-switch is the
  same official env var.

## Follow-ups

- Anticipation (M8) and escalation tiers (M7) will emit their decisions
  into this same stream — the "which tier answered / what was
  anticipated" visibility rides on D1's shared capability.
- Persona/harness token-share metric (ADR-0020.1 follow-up 2) belongs
  in the shared metrics layer once emitted.

## Cross-references

- ADR-0018 — telemetry semantics this ADR relocates; its follow-up 1
  ("extend to production sessions") is this ADR.
- NORTH_STAR "Sidecar Product Capabilities" §3 and M6 — the promise and
  the milestone.
- ADR-0016.4 — OTLP JSON file transport decision.
- ADR-0021 — persisted reports join the session artifact set (D2).
- AGENTS.md "Open Source Leverage" — official formats exactly.
