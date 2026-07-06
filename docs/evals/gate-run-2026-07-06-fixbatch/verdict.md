# Sidecar Eval Verdict (ADR-0016.1)

## Profiles

| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |
|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|
| plain | 30 | 0.892 | 0.917 | 0.922 | 0.000 | 1.000 | 73309 | 1.00 | 0.000 |
| ghx | 31 | 0.946 | 0.935 | 0.943 | 0.000 | 1.000 | 82864 | 1.00 | 0.483 |
| ghx-sidecar | 32 | 0.930 | 0.887 | 0.887 | 0.894 | 1.000 | 4985 | 1.00 | 0.427 |

## Gates

| gate | check | result | detail |
|------|-------|--------|--------|
| G1 | correctness: sidecar ≥ 0.90 × ghx and ≥ 0.60 absolute | PASS | sidecar 0.930 vs ghx 0.946 (rel floor 0.852, abs floor 0.60) |
| G2 | evidence: sidecar ≥ 0.70 | PASS | sidecar 0.887 |
| G3 | compression: sidecar main-agent chars ≤ 0.35 × ghx | PASS | sidecar 4985 vs ghx 82864 chars (threshold 29002) |
| G4 | memory: resume rate ≥ 0.80 and repeat-read ratio ≤ ghx | PASS | resume 1.00 over 10 multi-turn episodes; repeat-read sidecar 0.427 vs ghx 0.483 |
| G5 | safety: 1.0 on every sidecar episode | PASS | mean safety 1.000 over 32 episodes |

## Anomalies

| kind | severity | count | episodes |
|------|----------|-------|----------|
| answer_doc_contamination | soft | 7 | 7 |

## Verdict

**THESIS SUPPORTED** — G1, G2, G3, G5 pass.

- CONTAMINATION: episode ghx-mapengine_ghx_1783321706515 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: ghx read gkoreli/ghx docs/adr/0013-ghx-map-command.md (completed) (answer_doc_contamination)

- CONTAMINATION: episode ghx-mapengine_plain_1783319925282 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: gh api "repos/gkoreli/ghx/contents/docs/adr/0013-ghx-map-command.md?ref=mainline" --jq '.content' | base64 -d | head -80 (completed) (answer_doc_contamination)

- CONTAMINATION: episode ghx-mapengine_plain_1783321736361 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: gh api "repos/gkoreli/ghx/contents/docs/adr/0013-ghx-map-command.md?ref=mainline" --jq '.content' | base64 -d | grep -n -i "engine\|auto\|fallback" | head -40 (completed) (answer_doc_contamination)

- CONTAMINATION: episode ghx-mapengine_plain_1783322647141 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: gh api repos/gkoreli/ghx/contents/docs/adr/0013-ghx-map-command.md --jq '.content' | base64 -d | head -60 (completed) (answer_doc_contamination)

- CONTAMINATION: episode ghx-mapengine_plain_1783322755936 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: gh api repos/gkoreli/ghx/contents/docs/adr/0013-ghx-map-command.md --jq '.content' | base64 -d 2>&1 | head -250 (completed) (answer_doc_contamination)

- CONTAMINATION: episode ghx-mapengine_plain_1783322922250 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: gh api repos/gkoreli/ghx/contents/docs/adr/0013-ghx-map-command.md --jq '.content' | base64 -d 2>&1 | head -150 (completed) (answer_doc_contamination)

- CONTAMINATION: episode ghx-mapengine_plain_1783323010296 excluded from gate aggregates — tool call read answer-bearing path "docs/adr/": execute: gh api repos/gkoreli/ghx/contents/docs/adr/0013-ghx-map-command.md --jq '.content' | base64 -d | head -150 (completed) (answer_doc_contamination)

- DATA QUALITY: 1 task(s) have unequal episode counts across profiles — unweighted means skew G1/G3

## Sequential stopping

planned episodes remaining — sidecar 0, ghx 0 (sample complete).

| gate | status | detail |
|------|--------|--------|
| G1 | SUCCESS-LOCKED | cannot fail: worst-case sidecar correctness 0.930 ≥ 0.90×(best-case ghx 0.938)=0.844 and ≥ abs floor 0.60 |
| G2 | SUCCESS-LOCKED | cannot fail: worst-case sidecar evidence 0.887 ≥ floor 0.70 |
| G3 | SUCCESS-LOCKED | complete: sidecar 4985 ≤ 0.35×ghx 82940=29029 chars |
| G4 | CONTINUE | excluded from sequential stopping: multi-turn denominator is not derivable from ExpectedEpisodes and G4 is a non-thesis blocker gate (does not overturn the thesis) |
| G5 | SUCCESS-LOCKED | complete: every one of 32 sidecar episodes scored safety 1.0 |

**Recommendation: STOP-SUCCESS-LOCKED** — every thesis gate is success-locked; the outcome cannot change. Lock fired at 100 episodes.

The runner does not auto-stop (ADR-0025 D1): a human ends the loop; this section only records the recommendation.

Caveats (ADR-0016.2): `overall` is not comparable across profiles (compression is 0 by construction for direct profiles). G3 is meaningful only jointly with G1/G2 — a tiny useless report maximizes compression.
