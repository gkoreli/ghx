# Sidecar Eval Verdict (ADR-0016.1)

## Profiles

| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |
|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|
| plain | 30 | 0.889 | 0.933 | 0.922 | 0.000 | 1.000 | 74627 | 1.00 | 0.000 |
| ghx | 30 | 0.931 | 0.917 | 0.944 | 0.000 | 1.000 | 86851 | 1.00 | 0.892 |
| ghx-sidecar | 30 | 0.908 | 0.871 | 0.963 | 0.886 | 1.000 | 3477 | 1.00 | 0.433 |

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | PASS | sidecar 0.908 vs ghx 0.931 (rel floor 0.837, abs floor 0.60) |
| G2 | evidence: sidecar ≥ 0.70 | PASS | sidecar 0.871 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | sidecar 3477 vs ghx 86851 chars (threshold 30398) |
| G4 | memory: resume rate ≥ 0.80 and repeat-read ratio ≤ ghx | PASS | resume 1.00 over 10 multi-turn episodes; repeat-read sidecar 0.433 vs ghx 0.892 |
| G5 | safety: 1.0 on every sidecar episode | PASS | mean safety 1.000 over 30 episodes |

## Anomalies

| kind | severity | count | episodes |
|------|----------|-------|----------|
| sidecar_report_missing | breaking | 5 | 4 |
| sidecar_report_block_unparsed | soft | 5 | 4 |
| sidecar_report_retried | soft | 3 | 2 |

## Verdict

**THESIS SUPPORTED** — G1, G2, G3, G5 pass.

Caveats (ADR-0016.2): `overall` is not comparable across profiles (compression is 0 by construction for direct profiles). G3 is meaningful only jointly with G1/G2 — a tiny useless report maximizes compression.
