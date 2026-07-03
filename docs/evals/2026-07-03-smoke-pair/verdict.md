# Sidecar Eval Verdict (ADR-0016.1)

## Profiles

| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |
|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|
| plain | 0 | 0.000 | 0.000 | 0.000 | 0.000 | 0.000 | 0 | 0.00 | 0.000 |
| ghx | 1 | 1.000 | 1.000 | 0.700 | 0.000 | 1.000 | 31432 | 1.00 | 0.000 |
| ghx-sidecar | 1 | 1.000 | 1.000 | 0.822 | 0.878 | 1.000 | 4006 | 1.00 | 0.000 |

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | PASS | sidecar 1.000 vs ghx 1.000 (rel floor 0.900, abs floor 0.60) |
| G2 | evidence: sidecar ≥ 0.70 | PASS | sidecar 1.000 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | sidecar 4006 vs ghx 31432 chars (threshold 11001) |
| G4 | memory: resume rate ≥ 0.80 and repeat-read ratio ≤ ghx | PASS | resume 1.00 over 1 multi-turn episodes; repeat-read sidecar 0.000 vs ghx 0.000 |
| G5 | safety: 1.0 on every sidecar episode | PASS | mean safety 1.000 over 1 episodes |

## Verdict

**THESIS SUPPORTED** — G1, G2, G3, G5 pass.
