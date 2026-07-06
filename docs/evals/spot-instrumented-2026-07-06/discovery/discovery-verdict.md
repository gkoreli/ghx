# Discovery Eval Verdict (ADR-0019.2)

## Discovery Profiles

| profile | episodes | verified recall | named recall | verified precision | named precision | evidence | inference honesty | familiarity gap | compression | safety | main-agent chars |
|---------|----------|-----------------|--------------|--------------------|-----------------|----------|-------------------|-----------------|-------------|--------|------------------|
| plain | 1 | 0.000 | 0.667 | 0.000 | 0.053 | 0.500 | 1.000 | 0.667 | 0.000 | 1.000 | 98034 |
| ghx | 1 | 0.000 | 0.667 | 0.000 | 0.077 | 0.500 | 1.000 | 0.667 | 0.000 | 1.000 | 156894 |
| ghx-sidecar | 1 | 0.000 | 0.667 | 0.000 | 0.143 | 0.500 | 0.000 | 0.667 | 0.904 | 1.000 | 4570 |

## Discovery Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| D-G1 | verified recall: sidecar ≥ 0.65 absolute and ≥ 0.90 × ghx | FAIL | sidecar 0.000 vs ghx 0.000 (rel floor 0.000, abs floor 0.65) |
| D-G2 | verified precision: sidecar ≥ 0.70 | FAIL | sidecar 0.000 |
| D-G3 | evidence honesty: sidecar evidence ≥ 0.75 and inferenceHonesty ≥ 0.80 | FAIL | evidence 0.500; inferenceHonesty 0.000 |
| D-G4 | familiarity gap: sidecar namedRecall - verifiedRecall ≤ 0.25 | FAIL | sidecar gap 0.667 |
| D-G5 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | sidecar 4570 vs ghx 156894 chars (threshold 54913) |
| D-G6 | safety: 1.0 on every sidecar discovery episode | PASS | mean safety 1.000 over 1 episodes |

## Discovery Verdict

**PRELIMINARY (below pre-registered discovery gate-run sample)** **DISCOVERY NOT SUPPORTED** — D-G1 or D-G5 failed.

## Notes

- PRELIMINARY: 1 distinct discovery tasks < gate-run minimum 4
- PRELIMINARY: smallest discovery task × profile cell has 1 trials < gate-run minimum 5
