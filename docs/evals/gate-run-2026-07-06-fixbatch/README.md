# Gate run 2026-07-06 "fixbatch" — THESIS SUPPORTED (citable)

The first verdict measured under the ADR-0016.8 measurement fix batch
(union-of-turn-reports scoring, answer-doc contamination guard,
sufficiency honesty) and the ADR-0027 resilient runtime. All five
pre-registered gates pass at contract sample; the ADR-0025 D1 sequential
stopping analysis reports STOP-SUCCESS-LOCKED (no remaining episode could
have changed any thesis gate).

## Result

| profile | episodes | correctness | evidence | compression | safety |
|---|---|---|---|---|---|
| plain | 30 | 0.892 | 0.917 | — | 1.000 |
| ghx | 31 | 0.946 | 0.935 | — | 1.000 |
| ghx-sidecar | 32 | 0.930 | 0.887 | 0.894 (4,985 vs 82,864 chars ≈ 16.6×) | 1.000 |

G1 sidecar/ghx correctness ratio **0.983** (floor 0.90); G2 evidence 0.887
(floor 0.70); G3 compression 16.6× (threshold 2.9×); G4 resume 1.00,
repeat-read 0.427 < ghx 0.483; G5 safety 1.0 on all 32 sidecar episodes.

**The instrument fix is now measured, not projected**: the 2026-07-05
partial run scored the identical product behavior at sidecar correctness
0.866 (G1 ratio 0.921) because final-report-only scoring penalized
delta-reporting; under pre-registered union scoring the same behavior
measures 0.930. The projection chain (INSIGHTS.md → ADR-0016.8 D1 →
this run) closed within noise of its prediction.

## Who scored this, from what, and how to check

Deterministic Go scorer (`internal/sidecar/evals/rewards.go`, `gates.go`
at mainline `7ed448d`+, rules pre-registered in ADR-0016.1/0016.2/0016.8
before the run). Every score recomputes from the episode JSONs in this
directory. Recompute: run the evals verdict recomputation over this
directory (zero-token: `go test ./internal/sidecar/evals -tags=agent_e2e
-run 'TestEpisodes/zz-none' with GHX_EVAL_RUN_DIR` pointed here) or
`go run ./internal/sidecar/evals/cmd/evalreport <this dir>`. traces.jsonl
/ logs.jsonl / metrics.jsonl are committed per ADR-0016.8 D9 and replay
into any OTLP viewer (`ghx sidecar view` pattern; ADR-0018 recipe).

## Run shape (honesty notes)

- Planned 6 tasks × 3 profiles × 5 trials = 90, executed in 5 rounds at
  `GHX_EVAL_PARALLEL=3` (~62 min wall clock), subject claude-sonnet-5.
- **Contamination guard fired 7 times** (`answer_doc_contamination`,
  ghx-mapengine only): agents read the answer-bearing
  `docs/adr/0013-ghx-map-command.md` inside the subject repo — 6× plain
  (raw `gh api` exploration finds docs/ naturally), 1× ghx. Flagged
  episodes are committed here but **excluded from gate aggregates**
  (ADR-0016.8 D2). Exclusions dropped the plain cell below the 5-valid-
  trial minimum, so interim verdicts self-labeled PRELIMINARY /
  THESIS NOT SUPPORTED (D3 honesty) until top-up rounds (mapengine cells
  only, then plain-only, same code identity — all commits between rounds
  were docs-only) restored contract: final cells 30/31/32 valid episodes.
- Structural finding routed to the task corpus backlog: a self-referential
  task whose answer exists in the subject repo's own design docs has a
  contamination-prone *plain baseline* by construction (nothing forbids a
  baseline agent from reading docs). Options (replace the task, or accept
  recurring exclusions + top-ups) belong to a pre-registered eval-corpus
  ADR before the next full run.
- One episode-count wrinkle: top-ups make profile counts unequal (30/31/32
  — the data-quality warning notes the skew); gates use episode-weighted
  means per current pre-registered aggregation.
- New ADR-0027 anomalies (`turn_cap_wrapup`, `episode_hang_timeout`):
  **zero occurrences** — no turn-cap deaths, no hangs, no zombie rounds
  (the 2026-07-05 partial run had one 67-minute zombie; the cancellable
  wait + watchdog ran clean).

## Cross-references

- ADR-0016.8 (the pre-registered fix batch this run measures),
  ADR-0025/0025.1 (economics; this run's baselines are reuse candidates
  for 7 days under the D2 hash rules), ADR-0027 (runtime), ADR-0023.1
  (judge layer — these episodes feed gold-set candidates).
- Prior committed runs: `gate-run-2026-07-05-confirmatory/` (old
  instrument, THESIS SUPPORTED as conservative floor),
  `gate-run-2026-07-05-d2-0021-0022-partial/` (validity stop + the
  defect quantification that predicted this run).
