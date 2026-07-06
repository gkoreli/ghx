---
title: "ADR-0025: Eval Run Economics — Sequential Stopping, Baseline Reuse, Bounded Parallelism, Subject-Model Arms"
date: "2026-07-05"
status: "accepted"
thread: "eval-methodology"
author: "Goga Koreli"
---

# 0025. Eval Run Economics

## Status

Accepted 2026-07-05 (founder direction: "all 90 episodes are not needed
to make a go/no-go decision — we are actively engineering, we can't wait
hours between features"; "we should move to haiku for cheaper and faster
eval execution"). Applies to runs **after** the
gate-run-2026-07-05-d2-0021-0022 currently executing — stopping rules and
methodology changes bind only runs registered under them (changing rules
after seeing interim data is the validity hole this ADR exists to avoid).
Canonical policy anchor: AGENTS.md "full gate runs never block
engineering" (fa2178c).

## Context

The full ADR-0016.1 suite is 90 sequential live episodes (~2–3.5 h).
That cost is right for a milestone verdict and wrong for an engineering
loop. The ladder (ADR-0016.3) already gives feature-level go/no-go in
minutes via smokes and spot checks; this ADR compresses the *full-rigor*
tier itself, without weakening what a citable verdict means.

## Decisions

### D1. Sequential stopping (both directions), pre-registered per run

After every round, the runner computes worst-case bounds for each gate
over the remaining planned episodes:

- **Futility stop** (already exercised on gate-run-2026-07): if a
  failing gate cannot mathematically recover even with perfect remaining
  scores, stop; verdict = NOT SUPPORTED, fully citable.
- **Success lock**: if no gate can mathematically fall below threshold
  even with zero remaining scores (for G5: no remaining episode can
  un-fail safety, which any violation would), stop; verdict = SUPPORTED,
  fully citable at the achieved sample.
- Otherwise continue. The verdict records which rule fired and at what
  episode count. Bounds math lives in gates.go next to the thresholds it
  bounds, unit-tested against synthetic runs.

This is the rigorous version of "the picture is settled": lock-in is a
theorem about the data, not a feeling about the means. No peeking rule
beyond this — interim means are never a stopping reason.

### D2. Baseline reuse across runs (90 → 30 when conditions hold)

A run may reuse the plain and ghx baseline episodes of a previous
committed run iff ALL of: identical wrapper script hash, identical
adapter version, identical subject model ID, identical task corpus hash,
identical direct-profile preamble, and the prior run ≤ 7 days old. The
manifest records the reused run's ID and every hash; the verdict labels
itself BASELINE-REUSED. Any mismatch → fresh baselines, no exceptions.
(Not usable tonight: the wrapper env changed since M4 —
MAX_THINKING_TOKENS — which is exactly the kind of drift the hash check
exists to catch.)

### D3. Bounded episode parallelism

The runner may execute up to 3 episodes concurrently, never two trials
of the same task×profile cell at once. Duration/latency metrics from
parallel runs carry a `ghx.eval.parallel=true` attribute and are
excluded from any latency claims; reward scoring is content-based and
unaffected. Rate-limit failures under parallelism fall back to
sequential for the remainder of the round (recorded as an anomaly).

### D4. Subject-model policy: arms, then default

- **Sensitivity arm (next run after the current one)**: same matrix,
  subject `claude-haiku-4.5-class`, trials per D1 stopping. Pre-registered
  question: does the sidecar's relative advantage (G1 ratio, SPT ratio)
  hold or widen as the subject gets cheaper? This is the P4 thesis in
  miniature and a first-class marketing result either way.
- **Default switch rule**: if the arm shows all five gates pass with the
  haiku-class subject, the *production default* subject and therefore
  all eval tiers move to it — smokes and spot checks must always run the
  same subject as the full runs they predict (prediction validity), so
  the default moves everywhere at once, never tier-by-tier.
- Cross-arm comparisons are always same-arm-internal (haiku sidecar vs
  haiku baselines); never haiku sidecar vs sonnet baselines.

### D5. Combined effect (recorded expectation, not a promise)

D1+D2+D3 together put a full-rigor confirmatory run in the
~20–40 minute range in the common case (reused baselines, early lock,
3-way parallel) — inside an engineering session. Measured when first
exercised; this paragraph is not citable.

## Considered and rejected

- **Stop the current run under these rules** — rules adopted mid-run
  after seeing interim data; textbook invalidity. The current run
  finishes (or is stopped as honest PRELIMINARY by founder choice).
- **Interim-mean early stopping ("looks settled")** — single-trial cells
  swing (observed today: 1.0 → 0.75 correctness, same task, same build);
  only the bounds lock distinguishes settled from lucky.
- **Different subjects for different tiers** (haiku smokes predicting
  sonnet runs) — breaks the only property that makes cheap tiers useful.
- **Unbounded parallelism** — rate-limit cascade risk (observed M4-era),
  duration-metric pollution.

## Implementation Notes (2026-07-05 — D1 and D3 shipped; port-reconciled)

Built on a worktree during the live gate run (measurement stack
untouched), then semantically ported onto the post-ADR-0022 telemetry
refactor. The escalated ADR add/add (the build worker had recreated this
spec from an older base) was resolved by keeping this spec verbatim and
adapting the notes here.

- **D3 parallelism**: `parallel.go` (`GHX_EVAL_PARALLEL`, default 3;
  `<1`→1 so a typo never fans out), counting-semaphore-gated
  `t.Parallel()` cells nested under one parent subtest; cell exclusivity
  is structural (trials come from separate invocations).
  `ghx.eval.parallel=true` marks episode JSON, metrics, and spans;
  rate-limit-shaped failures record the soft anomaly
  `eval_parallel_rate_limited` (substring detector — declarative, so the
  taxonomy widens without re-running agents).
- **The race fix moved to the shared layer** (port decision): the
  per-path locked `AppendJSONLine` lives in
  `internal/sidecar/telemetry/jsonl_writer.go` and serializes ALL
  shared-file appends — eval logs/metrics/trace exports AND production
  session emission (`emit.go` funnels through the same writers). The
  pre-existing bug it fixes: each episode built a fresh trace exporter
  with a per-instance mutex over the same `traces.jsonl`, which never
  serialized concurrent episodes. `-race` coverage: 64 goroutines × 20KB
  lines, line-validity asserted.
- **D1 sequential stopping**: `stopping.go` `ComputeStoppingBounds` with
  per-gate monotonicity encoded honestly — G1/G2 lock both directions;
  G3 (unbounded char metric) never locks early; G4 excluded (multi-turn
  denominator not derivable mid-run); G5 futility-locks on any
  violation, success-locks only at completion. `verdict.md` gains a
  `## Sequential stopping` section with CONTINUE / STOP-FUTILITY /
  STOP-SUCCESS-LOCKED; the runner never auto-stops — a human ends the
  loop. Insight-mining result recorded here per D5's honesty rule: on
  the M4 data, worst-case success-locks fire only in rounds 4–5 — D1
  does not speed up thin-margin passing runs; the wall-clock wins come
  from D3 and (future) D2.

Deliberately not built: auto-stop; D2 baseline-reuse hashes (own
pre-registered slice — touches identity/decontamination, ADR-0016.5).
Known limitation: multi-round runs must set `GHX_EVAL_EXPECTED_EPISODES`
to the whole planned total or D1's remaining-count math degrades;
folding trials into the manifest is the follow-up (also fixes the
stale-manifest audit major).

## Cross-references

- ADR-0016.1 (gates/thresholds bounded by D1), 0016.3 (the ladder this
  compresses), 0016.2 (validity discipline).
- AGENTS.md "full gate runs never block engineering" — policy anchor.
- NORTH_STAR P4 + "Observability is the substrate for self-improvement"
  — D4's sensitivity arm is the cheap-brain bet measured early;
  continuous evaluation needs exactly this per-run cost profile.
