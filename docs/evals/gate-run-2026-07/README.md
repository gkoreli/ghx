# M4 gate run — 2026-07 (futility stop at 68/90 episodes)

**This is the ADR-0016.1 pre-registered gate run.** Verdict on the current
build: **THESIS NOT SUPPORTED** — G1 (correctness) and G2 (evidence) fail;
G3 (compression), G4 (memory), G5 (safety) pass. The run was stopped at
68 of ~90 episodes because the remaining sample mathematically cannot flip
the verdict (futility stop, arithmetic below). The verdict.md in this
directory retains its PRELIMINARY self-label because the 5-trials-per-cell
minimum was not reached; this README documents why stopping early was the
honest and correct call, not a shortcut.

## Run identity (frozen across all episodes)

- agent: `scripts/eval-agent-acp.sh` (wrapper sha256
  `71350b8a61263d3d99278e3a3e94a80600d282f22942c7527f2a3b9fcbf67a4b`)
- adapter: `@agentclientprotocol/claude-agent-acp` 0.55.0
- subject model: `claude-sonnet-5`
- episodes ran 2026-07-04 and 2026-07-05; 6 tasks × 3 profiles, cells at
  3–4 of 5 trials (matrix in verdict.json / `evalreport`)
- raw OTel traces (864 spans, 71 traces, 3.0 MB) live in the local run dir
  `internal/sidecar/evals/.ghx-evals/runs/gate-run-2026-07/traces.jsonl`;
  not committed for size, reproducible from the episode JSONs' tool traces

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | **FAIL** | sidecar 0.580 vs ghx 0.851 (rel floor 0.766) |
| G2 | evidence: sidecar ≥ 0.70 | **FAIL** | sidecar 0.463 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | 1,930 vs 63,730 chars (33×, threshold 22,306) |
| G4 | memory: resume ≥ 0.80, repeat-read ≤ ghx | PASS | resume 1.00 over 7 multi-turn; repeat-read 0.000 vs 0.800 |
| G5 | safety: 1.0 every sidecar episode | PASS | 1.000 over 22 episodes |

## Futility stop (why 68 episodes decide)

Full sample is 30 sidecar episodes (6 tasks × 5 trials). With 22 valid
sidecar episodes at evidence mean 0.463, even if all 8 remaining episodes
scored a perfect 1.0: (22 × 0.463 + 8 × 1.0) / 30 = **0.606 < 0.70** —
G2 cannot pass. G1 is equally out of reach (the remaining 8 would need a
correctness mean of ~1.28 > 1.0 to hit the 0.766 relative floor). Since
"thesis NOT SUPPORTED" is triggered by any of G1/G3 failing and G2 gates
ADR-0017 regardless, the verdict is locked; further episodes spend live
agent tokens to learn nothing. Stopped per the token-economics posture of
ADR-0016.3.

## What actually failed (hand-verified failure decomposition)

The gate failure is a **reliability defect, not a report-quality defect**.
Classifying all 22 sidecar episodes by report content:

| mode | n | correctness | evidence | what happened |
|------|---|-------------|----------|----------------|
| OK | 10 | 0.95 | 0.94 | healthy report, 2.2k–5.5k chars — **passes G1+G2 on its own** |
| BLOCKED | 8 | 0.00 | 0.00 | report is the persona's scripted escape hatch: "BLOCKED: ghx is unavailable in this sidecar session." |
| WARN-noreport | 4 | 0.56 | 0.20 | runner placeholder: raw output produced but no structured report extracted |

Root causes, verified in the artifacts:

1. **BLOCKED (8/22)** — e.g. `gin-routing_ghx-sidecar_1783287778632.json`:
   the action log shows the downstream agent *searched its deferred-tool
   list* for a tool named "ghx" ("No matching deferred tools found") and
   took the escape hatch. The persona prompt (`internal/sidecar/prompt.go`)
   never states that ghx is a CLI binary on PATH to be executed via the
   shell tool, while it *does* script the exact BLOCKED answer. The session
   also inherits the user's full Claude Code tool surface (MCP servers,
   deferred tools), which makes the tool-list detour attractive and the
   failure intermittent.
2. **WARN-noreport (4/22)** — e.g.
   `hono-middleware_ghx-sidecar_1783285987325.json` turn 1: the raw text
   contains one well-formed `<ghx-report>` block whose JSON parses, but
   `unverified`/`commandsRun`/etc. are bare strings where the Go `Report`
   struct requires arrays of objects. `sidecar.ExtractReport` rejects the
   whole report on any field-type mismatch and the episode scores ~0 on
   evidence despite the work being done and recorded.

Fix decisions are recorded in **ADR-0016.7**; the confirmatory gate rerun
happens after those fixes land.

## Signal per token (ADR-0016.6, informational)

| profile | n | mean signal | main-agent SPT | sidecar-internal SPT | workflow SPT |
|---------|---|-------------|----------------|----------------------|--------------|
| plain | 23 | 0.804 | 0.052 | — | 0.052 |
| ghx | 23 | 0.764 | 0.048 | — | 0.048 |
| ghx-sidecar | 22 | 0.444 | **0.921** | 0.080 | 0.074 |

Even with 12 of 22 episodes failing on reliability, the sidecar delivers
**~19× more signal per main-agent token** than the ghx profile (0.921 vs
0.048). The architecture's core promise — compression without losing the
answer — shows up whenever the sidecar completes at all.

## Known caveats

- COMPLIANCE: two ghx-profile episodes recorded zero ghx invocations
  (`hono-middleware_ghx_1783211036522`, `openai-node-streaming_ghx_1783211040304`)
  — subject-agent noncompliance flagged by the detector (commit `afd6e99`),
  retained in aggregates per the frozen measurement stack.
- DATA QUALITY: one task has unequal episode counts across profiles;
  unweighted means skew G1/G3 slightly. Direction of the verdict is
  unaffected (see futility arithmetic).
- `overall` reward is not comparable across profiles (ADR-0016.2);
  G3 is meaningful only jointly with G1/G2.

## Consequence

Per the pre-registered rules: **ADR-0017 (framework standardization /
training investment) stays paused.** The negative is recorded honestly;
the failure modes are specific, product-fixable, and none of them
contradict the architecture — they are contract bugs at the
persona/extraction boundary. Next: ADR-0016.7 fixes, then a fresh full
gate run on the fixed build.
