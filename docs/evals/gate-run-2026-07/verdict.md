# Sidecar Eval Verdict (ADR-0016.1)

## Profiles

| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |
|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|
| plain | 23 | 0.891 | 0.826 | 0.851 | 0.000 | 1.000 | 62118 | 1.00 | 0.000 |
| ghx | 23 | 0.851 | 0.826 | 0.870 | 0.000 | 1.000 | 63730 | 1.00 | 0.800 |
| ghx-sidecar | 22 | 0.580 | 0.463 | 0.971 | 0.842 | 1.000 | 1930 | 1.00 | 0.000 |

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | FAIL | sidecar 0.580 vs ghx 0.851 (rel floor 0.766, abs floor 0.60) |
| G2 | evidence: sidecar ≥ 0.70 | FAIL | sidecar 0.463 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | sidecar 1930 vs ghx 63730 chars (threshold 22306) |
| G4 | memory: resume rate ≥ 0.80 and repeat-read ratio ≤ ghx | PASS | resume 1.00 over 7 multi-turn episodes; repeat-read sidecar 0.000 vs ghx 0.800 |
| G5 | safety: 1.0 on every sidecar episode | PASS | mean safety 1.000 over 22 episodes |

## Verdict

**PRELIMINARY (below pre-registered gate-run sample — not the project verdict)** **THESIS NOT SUPPORTED** — see failed gates above.

- COMPLIANCE: ghx profile episode hono-middleware_ghx_1783211036522 recorded zero ghx invocations

- COMPLIANCE: ghx profile episode openai-node-streaming_ghx_1783211040304 recorded zero ghx invocations

- PRELIMINARY: smallest task × profile cell has 3 trials < gate-run minimum 5

- DATA QUALITY: 1 task(s) have unequal episode counts across profiles — unweighted means skew G1/G3

Caveats (ADR-0016.2): `overall` is not comparable across profiles (compression is 0 by construction for direct profiles). G3 is meaningful only jointly with G1/G2 — a tiny useless report maximizes compression.
