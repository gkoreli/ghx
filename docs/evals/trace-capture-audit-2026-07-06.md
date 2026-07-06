# Trace-Capture Completeness Audit — 2026-07-06

## Provenance

- Purpose: TRUST.md hole **H3** evidence ("toolTraces come from ACP
  notifications; drops would be silent"). Contains (a) a read-only scan of
  committed fixbatch artifacts for capture gaps and (b) the design of the
  `trace_capture_gap` comparator/anomaly that would close the remaining gap.
- Auditor: cross-family codex (GPT-5.5-class) analyst in a read-only
  sandbox; report harvested verbatim from the worker log
  (`/tmp/codex-trace-audit.log`) on 2026-07-06.
- Corpus scanned: `docs/evals/gate-run-2026-07-06-fixbatch/` committed
  episodes and OTel traces (100 episodes, 130 turns, 1,355 toolTraces).
- Bounds: the scan proves **internal consistency** of committed episode
  JSON and the OTel traces derived from `ToolTraces`. It does NOT prove
  ACP captured every raw SDK tool-use block — no committed raw-SDK
  firehose artifact exists yet to diff against. H3 stays open until the
  comparator below is built and run against persisted raw SDK messages.

The report below is verbatim from the worker.

---

**answer**

Committed fixbatch artifacts show no trace-capture gap in the checks I could recompute:

- Scanned `100` episodes, `130` turns, `1,355` captured `toolTraces`.
- Stuck pending calls: `0`.
- Episode `toolTraces` vs OTel `tool.*` spans: `0` mismatches.
- OTel `eval.turn ghx.eval.tool_call_count` vs episode `len(toolTraces)`: `0` mismatches.
- Sidecar structured `report.commandsRun` audit: `32` sidecar episodes, `284` reported commands, `0` missing from captured tool trace inputs.
- Manual/prose sample: `5` turns across `4` episodes, no clear “agent did something invisible” case. The broad file-path heuristic is too noisy because answers contain conceptual paths/repo names and tool output excerpts are truncated.

The caveat: this proves internal consistency of committed episode JSON and OTel derived from `ToolTraces`. It does **not** prove ACP captured every raw SDK tool-use block, because I found no committed raw SDK firehose artifact to diff against.

**files changed or inspected**

No files modified.

Key inspected files:

- [internal/sidecar/acp.go](/Users/goga/Documents/goga/ghx/internal/sidecar/acp.go:196): `ToolCallTrace` fields.
- [internal/sidecar/acp.go](/Users/goga/Documents/goga/ghx/internal/sidecar/acp.go:449): `denyClient.SessionUpdate`.
- [internal/sidecar/evals/episode.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/episode.go:121): `TurnRecord.ToolTraces`.
- [internal/sidecar/evals/otel_trace.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/otel_trace.go:116): OTel emits one tool span per `turn.ToolTraces`.
- [docs/adr/0020.1-protocol-strategy-decision.md](/Users/goga/Documents/goga/ghx/docs/adr/0020.1-protocol-strategy-decision.md:66): `emitRawSDKMessages` is intended for eval audit.
- [internal/sidecar/evals/anomalies.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/anomalies.go:23): existing anomaly taxonomy.

Representative artifacts:

- [ghx-mapengine_ghx_1783321706515.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-06-fixbatch/ghx-mapengine_ghx_1783321706515.json:39): `8` tool traces, all completed.
- [openai-node-streaming_ghx-sidecar_1783321430820.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-06-fixbatch/openai-node-streaming_ghx-sidecar_1783321430820.json:49): `21` tool traces, all completed.
- [traces.jsonl](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-06-fixbatch/traces.jsonl:996): OTel turn count for `openai-node-streaming_ghx-sidecar_1783321430820` is `21`.

**evidence snippets or line references**

Capture path:

- `tool_call` upserts trace, records title/kind/rawInput/status/output: [acp.go](/Users/goga/Documents/goga/ghx/internal/sidecar/acp.go:480).
- `tool_call_update` upserts same ID and appends status/output: [acp.go](/Users/goga/Documents/goga/ghx/internal/sidecar/acp.go:498).
- OTel derives tool spans by iterating `turn.ToolTraces`: [otel_trace.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/otel_trace.go:116).
- Turn span stores `ghx.eval.tool_call_count = len(turn.ToolTraces)`: [otel_trace.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/otel_trace.go:193).

Representative count cross-checks:

```text
hono-middleware_plain_1783320397380 turn 0: episode=3, otelToolSpans=3, otelTurnAttr=3
hono-middleware_plain_1783320397380 turn 1: episode=2, otelToolSpans=2, otelTurnAttr=2
openai-node-streaming_ghx-sidecar_1783321430820 turn 0: episode=21, otelToolSpans=21, otelTurnAttr=21
ghx-mapengine_ghx_1783321706515 turn 0: episode=8, otelToolSpans=8, otelTurnAttr=8
flask-routing_ghx-sidecar_1783322294265 turn 0: episode=19, otelToolSpans=19, otelTurnAttr=19
```

Full scan output:

```text
totalEpisodes=100
totalTurns=130
totalTools=1355
stuckCount=0
mismatchCount=0
missingTurnSpanCount=0
```

**comparator design**

Add a durable raw-vs-captured comparator in `internal/sidecar/evals`, probably `trace_capture.go`.

Inputs:

- Episode JSON `turns[].toolTraces`.
- New persisted raw SDK audit artifact per episode, e.g. `raw-sdk-messages.jsonl`, emitted only when `EvalMode` enables `emitRawSDKMessages`.
- Raw SDK records must include episode id, turn, ACP notification order/index, SDK message type, raw tool-use id/name/input, and raw tool-result status/output metadata when present.

Comparator:

1. Normalize raw SDK tool-use blocks into `RawSDKToolUse{Turn, ID, Name/Kind, InputDigest, FirstSeenIndex}`.
2. Normalize raw SDK tool-result blocks into terminal status records keyed by tool ID.
3. Normalize captured `ToolCallTrace` into `CapturedToolUse{Turn, ID, Kind, InputDigest, LastStatus}`.
4. Diff by `(turn, tool id)` first; fallback to `(turn, input digest, ordinal)` only when raw SDK lacks an ID.
5. Raise `trace_capture_gap` when:
   - raw SDK has a tool-use block with no matching `ToolCallTrace`;
   - raw SDK has terminal result/status but captured trace remains pending;
   - captured trace raw input differs materially from raw SDK input digest;
   - raw SDK shows tool result/output but captured `OutputSize == 0` and no excerpt/status update.
6. Raise as `SeverityBreaking` if a raw tool-use is absent from `ToolTraces`; `SeveritySoft` for metadata/output truncation mismatches that do not hide the call itself.

Where it plugs in:

- Add `AnomalyTraceCaptureGap = "trace_capture_gap"` in [anomalies.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/anomalies.go:23).
- Include it in `CountAnomalies` ordering near other measurement-integrity issues.
- Have `DetectAnomalies` call a pure artifact-based comparator, not a live SDK path.
- Unit tests should construct a tiny episode plus raw JSONL fixture and assert missing raw tool-use, pending-with-raw-result, and clean-match cases.

**commands run**

Used read-only `rg`, `sed`, `nl`, `jq`, and `node -e` scans over `internal/sidecar`, ADRs, and `docs/evals/gate-run-2026-07-06-fixbatch`. Also ran `git status --short`; it reported pre-existing dirty/unmerged files, and macOS temp-cache warnings due the read-only sandbox.

**test results**

No test suite run; this was a read-only audit. The scan scripts completed successfully after avoiding here-docs, which the sandbox blocked.

**uncertainty**

The committed artifacts do not contain raw SDK firehose messages, so the strongest claim is “episode JSON and OTel are internally consistent.” The raw SDK comparator is still needed to answer whether ACP notification handling drops or mangles calls before `denyClient.SessionUpdate`.

**suggested next step**

Implement raw SDK persistence plus the pure comparator/anomaly before the next citable eval run, then re-run the same fixbatch-style scan with `trace_capture_gap` included in the anomaly table.
