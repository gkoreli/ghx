# Gate re-run 2026-07-05 (post-D2/0021/0022 build) — VALIDITY STOP at 54/90

**Status: PRELIMINARY — THESIS SUPPORTED at 54 episodes (3 of 5 trials per
cell), all five gates pass. Not the project's citable verdict** (that
remains `gate-run-2026-07-05-confirmatory/`, the M4 run).

## What this run was

The pre-registered ADR-0016.1 suite re-run over the build carrying
ADR-0020.1 D2 (persona→system prompt, budgets, isolation), ADR-0021
(submit_report contract), and ADR-0022 (shared visibility). 5 rounds
planned; stopped by decision during round 3 with 54 episodes banked
(perfect 6×3×3 matrix).

## Why it was stopped (validity stop, decided by the orchestrator)

Not for the results — the recomputed verdict above **passes** every gate.
Stopped because two instrument defects were diagnosed mid-run from round-1
artifacts, and an independent cross-family audit
(`docs/evals/audit-2026-07-05-independent/`) added more corrections:

1. Multi-turn scoring reads only the final turn's report while checks span
   the whole task — penalizing the new build's delta-reporting (the
   desired compression behavior). gin-routing episodes with full coverage
   across turns scored 0.58; an answer-only follow-up report scored
   evidence 0.00.
2. ADR-0021 validation accepts answer-only reports ("Evidence, not vibes"
   violation).
3. Audit majors: self-referential task contamination (plain baseline read
   answer-bearing `docs/adr/0013` in the target repo), `thesisSupported`
   JSON not gated on data sufficiency, stale multi-round manifest, two
   pre-detector episodes without stored anomaly fields.

Continuing would have spent ~36 more episodes measuring with a
known-mis-specified instrument whose fixes were already designed. The
remaining rounds' only deliverable — citability — was unreachable.

## What the 54 episodes honestly show

| gate | result | detail |
|---|---|---|
| G1 correctness | PASS | sidecar 0.866 vs ghx 0.940 (rel floor 0.846) |
| G2 evidence | PASS | 0.808 |
| G3 compression | PASS | 4,884 vs 76,899 main-agent chars (≈15.7×) |
| G4 memory | PASS | resume 1.00; repeat-read 0.333 vs ghx 0.375 |
| G5 safety | PASS | 1.0 on all 18 sidecar episodes |

The G1 margin is thin *because of* defect (1): known-mis-scored multi-turn
episodes drag the sidecar mean. These numbers are floors under a ruler
that shortchanges the subject — and still PRELIMINARY (3 < 5 trials), so
none of them are citable, in either direction. Do not quote them as a
regression or an improvement vs M4.

## What happens next (pre-registered before any new run)

The fix batch (union-of-reports scoring, evidence-required validation,
audit corrections) lands with its own ADR; the ADR-0025 machinery
(parallelism, sequential stopping, later baseline reuse) makes the clean
full-rigor re-run cheap. That run produces the next citable verdict.

Round logs: /tmp/gate-round-{1,2,3}.log (local); every score here is
recomputable from the episode JSONs in this directory (verified method:
the independent audit recomputed the M4 run 90/90 exact with the same
artifact schema).
