# Confirmatory gate run — 2026-07-05 (THESIS SUPPORTED, full sample)

**This is the ADR-0016.1 pre-registered gate run at full sample on the
ADR-0016.7-fixed build: 90 episodes, 6 tasks × 3 profiles × 5 trials,
every cell complete. Verdict: THESIS SUPPORTED — all five gates pass,
non-PRELIMINARY.** It is the confirmatory counterpart to the honest
negative in `docs/evals/gate-run-2026-07/` (same tasks, same thresholds,
same frozen scoring; the only product delta is the ADR-0016.7 reliability
fixes, giving a clean before/after).

## Run identity (frozen across all 90 episodes)

- agent: `scripts/eval-agent-acp.sh` · adapter
  `@agentclientprotocol/claude-agent-acp` 0.55.0 · subject model
  `claude-sonnet-5`
- 5 rounds accumulated into one dir on 2026-07-05; round-by-round
  cumulative verdicts stayed THESIS SUPPORTED at every checkpoint
- scoring/threshold code frozen throughout; the only mid-run change was
  the pre-registered declarative anomaly layer (ADR-0016.7 addendum,
  observability only, commit `7bbb831`)

## Gates (final, 30 episodes per profile)

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 | **PASS** | 0.908 vs ghx 0.931 (floor 0.837) |
| G2 | evidence ≥ 0.70 | **PASS** | 0.871 |
| G3 | compression ≤ 0.35 × ghx chars | **PASS** | 3,477 vs 86,851 chars — **25×** (threshold 30,398) |
| G4 | memory: resume ≥ 0.80, repeat-read ≤ ghx | **PASS** | resume 1.00 over 10 multi-turn; repeat-read 0.433 vs 0.892 |
| G5 | safety 1.0 every episode | **PASS** | 1.000 over 30 |

## Signal per token (ADR-0016.6)

| profile | n | mean signal | main-agent SPT | sidecar-internal SPT | workflow SPT |
|---------|---|-------------|----------------|----------------------|--------------|
| plain | 30 | 0.822 | 0.044 | — | 0.044 |
| ghx | 30 | 0.847 | 0.039 | — | 0.039 |
| ghx-sidecar | 30 | 0.803 | **0.923** | 0.101 | 0.091 |

**The sidecar delivers ~24× more signal per main-agent token than the ghx
profile** (0.923 vs 0.039) at 97.5% of its correctness — the compression
does not buy a worse answer, it buys the same answer at 1/25th of the
main agent's context cost, with better multi-turn memory discipline.

## Before/after: what the ADR-0016.7 fixes changed

| metric (sidecar profile) | negative run (2026-07) | this run |
|--------------------------|------------------------|----------|
| correctness | 0.580 | 0.908 |
| evidence | 0.463 | 0.871 |
| BLOCKED surrender rate | 8/22 (36%) | **0/30** |
| reports lost to parsing | 4/22 (18%) | 4/30 (13%, known gap, fix on branch) |
| verdict | NOT SUPPORTED | **SUPPORTED** |

## Anomalies (declarative, ADR-0016.7 addendum)

| kind | severity | count | episodes |
|------|----------|-------|----------|
| sidecar_report_missing | breaking | 5 | 4 |
| sidecar_report_block_unparsed | soft | 5 | 4 |
| sidecar_report_retried | soft | 3 | 2 |

Every breaking anomaly carries the `block_unparsed` diagnostic: the agent
emitted a well-formed report and the parser rejected shape drift (object
items in string-list fields). The fix exists on branch
`fix/sidecar-report-object-coercion` with regression fixtures extracted
from these exact artifacts — deliberately NOT merged mid-run to keep one
build across rounds. The BLOCKED escape-hatch failure mode from the
negative run did not occur once in 90 episodes.

## How to verify by hand

- `go run ./internal/sidecar/evals/cmd/evalreport <this dir>` recomputes
  everything from the artifacts.
- Any single score: open an episode JSON, compare its report against the
  pre-registered ground truth in
  `internal/sidecar/evals/testdata/tasks/<task>.json` per the rules in
  `internal/sidecar/evals/rewards.go` (deterministic, no LLM in the gate
  path).
- Traces: the run dir's local `traces.jsonl` (spec OTLP/JSON) replays
  into any OTLP viewer via the curl loop documented in
  `docs/evals/gate-run-2026-07/README.md`.

## Consequences

Per the pre-registered rules: the sidecar thesis is **supported on the
committed record**. ADR-0017 (framework standardization / training
investment) unblocks; next milestones are M5 (adoption/dogfooding — the
concise recon skill) and M6/ADR-0018 (agentic observability), with the
judge scorer required before M9 per the eval-truthfulness tenet.

## Postscript (2026-07-05, later the same day — nothing above altered)

The before/after table's "reports lost to parsing … known gap, fix on
branch" is no longer current: the coercion fix merged as 2889342 (with
regression fixtures extracted from these exact episodes), and the
open-loop parsing architecture it patched was then superseded entirely by
ADR-0021 (report contract enforcement — a validated `submit_report` MCP
tool; lenient parsing survives only as a fallback path that flags a
`sidecar_report_coerced` anomaly). The numbers in this record describe
the build at 6f69ff9 and remain the project's only citable verdict until
the pre-registered confirmatory re-run of the post-ADR-0020.1/0021 build.
