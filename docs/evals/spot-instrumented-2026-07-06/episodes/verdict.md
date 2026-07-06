# Sidecar Eval Verdict (ADR-0016.1)

## Profiles

| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |
|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|
| plain | 0 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0 | 0.00 | 0.000 |
| ghx | 0 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0 | 0.00 | 0.000 |
| ghx-sidecar | 2 | 1.000 | 0.778 | 0.967 | 0.883 | 1.000 | 4591 | 1.00 | 0.000 |

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | PASS | sidecar 1.000 vs ghx 0.000 (rel floor 0.000, abs floor 0.60) |
| G2 | evidence: sidecar ≥ 0.70 | PASS | sidecar 0.778 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | FAIL | sidecar 4591 vs ghx 0 chars (threshold 0) |
| G4 | memory: resume rate ≥ 0.80 and repeat-read ratio ≤ ghx | PASS | resume 1.00 over 1 multi-turn episodes; repeat-read sidecar 0.000 vs ghx 0.000 |
| G5 | safety: 1.0 on every sidecar episode | PASS | mean safety 1.000 over 2 episodes |

## Verdict

**PRELIMINARY (below pre-registered gate-run sample — not the project verdict)** **THESIS NOT SUPPORTED** — see failed gates above.

- INSUFFICIENT DATA: sidecar episodes=2, ghx episodes=0 — gates need both profiles present

- PRELIMINARY: 2 distinct tasks < gate-run minimum 6

- PRELIMINARY: 2 distinct repos < gate-run minimum 3 (ADR-0016.1 requires ≥ 6 tasks across ≥ 3 repos)

- PRELIMINARY: smallest task × profile cell has 0 trials < gate-run minimum 5

- PRELIMINARY: 1 multi-turn tasks < gate-run minimum 2

- DATA QUALITY: 2 task(s) have unequal episode counts across profiles — unweighted means skew G1/G3

## Sequential stopping

planned episodes remaining — sidecar 10, ghx 12 (in progress).

| gate | status | detail |
|------|--------|--------|
| G1 | CONTINUE | awaiting episodes in both sidecar and ghx before the comparison can lock |
| G2 | CONTINUE | open: sidecar evidence in [0.130, 0.963], floor 0.70 |
| G3 | CONTINUE | unbounded metric: main-agent chars have no upper bound on remaining episodes and the ghx denominator is unbounded too — G3 can only resolve at sample completion |
| G4 | CONTINUE | excluded from sequential stopping: multi-turn denominator is not derivable from ExpectedEpisodes and G4 is a non-thesis blocker gate (does not overturn the thesis) |
| G5 | CONTINUE | no violation yet, but 10 sidecar episode(s) remain and any one can still fail safety — success locks only at completion |

**Recommendation: CONTINUE** — no gate outcome is locked yet; keep running.

The runner does not auto-stop (ADR-0025 D1): a human ends the loop; this section only records the recommendation.

Caveats (ADR-0016.2): `overall` is not comparable across profiles (compression is 0 by construction for direct profiles). G3 is meaningful only jointly with G1/G2 — a tiny useless report maximizes compression.
