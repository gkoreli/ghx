# Sidecar Eval Verdict (ADR-0016.1)

## Profiles

| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |
|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|
| plain | 18 | 0.898 | 0.917 | 0.928 | 0.000 | 1.000 | 73323 | 1.00 | 0.000 |
| ghx | 18 | 0.940 | 0.944 | 0.932 | 0.000 | 1.000 | 76899 | 1.00 | 0.375 |
| ghx-sidecar | 18 | 0.866 | 0.808 | 0.912 | 0.888 | 1.000 | 4884 | 1.00 | 0.333 |

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | PASS | sidecar 0.866 vs ghx 0.940 (rel floor 0.846, abs floor 0.60) |
| G2 | evidence: sidecar ≥ 0.70 | PASS | sidecar 0.808 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | sidecar 4884 vs ghx 76899 chars (threshold 26915) |
| G4 | memory: resume rate ≥ 0.80 and repeat-read ratio ≤ ghx | PASS | resume 1.00 over 6 multi-turn episodes; repeat-read sidecar 0.333 vs ghx 0.375 |
| G5 | safety: 1.0 on every sidecar episode | PASS | mean safety 1.000 over 18 episodes |

## Verdict

**PRELIMINARY (below pre-registered gate-run sample — not the project verdict)** **THESIS SUPPORTED** — G1, G2, G3, G5 pass.

- PRELIMINARY: smallest task × profile cell has 3 trials < gate-run minimum 5

Caveats (ADR-0016.2): `overall` is not comparable across profiles (compression is 0 by construction for direct profiles). G3 is meaningful only jointly with G1/G2 — a tiny useless report maximizes compression.
