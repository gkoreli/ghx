# SAFE Trust Ledger

The living answer to "do we trust these eval results, or are we looking at
fake data?" (founder, 2026-07-06). Trust is a **tracked workstream, not an
assumed property**: every claim row states who verified it, from which
artifacts, and when; every hole is named until an audit closes it. Update
this file whenever a verification lands or a hole is found — an eval
number quoted anywhere is only as strong as its rows here.

## Verified (evidence-backed, re-checkable)

| Layer | Claim | Verified by | Evidence | Last verified |
|---|---|---|---|---|
| Deterministic scores | Every episode reward/gate recomputes exactly from committed artifacts | Cross-family audit (codex, read-only): 90/90 exact | `audit-2026-07-05-independent/REPORT.md` | 2026-07-05 |
| Union scorer | Matches independent hand-computation to ±0.001 | Golden regression on committed episodes | `internal/sidecar/evals/union_scoring_test.go` vs INSIGHTS.md P2 | 2026-07-06 |
| Traces (format) | Spec-exact OTLP; industry tooling ingests 100% | Live replay into otel-desktop-viewer | ADR-0018 notes; `ghx sidecar view` | 2026-07-05 |
| Artifact provenance | Manifest identity + per-run hash inventory | ADR-0025.1 (post-fix: every run writes identityHashes) | `6cf3c77` | 2026-07-06 |
| Self-honesty | Framework rules against itself: contamination exclusions forced PRELIMINARY until top-ups | Live behavior on gate-run-2026-07-06-fixbatch | run README | 2026-07-06 |
| Anomaly persistence | Stored per-episode anomalies == re-derived | Regression test (ADR-0016.8 D6) | evals tests | 2026-07-06 |
| Map-compression copy claim | ~92% reproduces (93.9% typical file; 22.5% outlier scoped out) | Live measurement | skills SKILL.md notes, `b11abf7` | 2026-07-06 |
| Discovery target sets | Verified by the sidecar itself + direct reads | Dogfood verification (3 sessions, cited) | ADR-0019.2 notes, `7fdec51` | 2026-07-06 |
| Trace internal consistency (bounded) | On fixbatch committed episodes: zero stuck-pending toolTraces; episode toolTraces == OTel tool spans == turn counts (100 episodes / 130 turns / 1,355 tools); all 284 sidecar `report.commandsRun` present in captured tool inputs. Bound: proves episode-JSON/OTel consistency only — does NOT prove ACP captured every raw SDK block (H3 stays open) | Cross-family audit (codex, read-only) | `trace-capture-audit-2026-07-06.md` over `gate-run-2026-07-06-fixbatch/` | 2026-07-06 |

## Open holes (named, owned, not yet closed)

| # | Hole | Risk | Status |
|---|---|---|---|
| H1 | **Checks measure fact recall, not quality.** Substring gates can score a lucky mention. | Quality claims overstate | Judge layer built; **uncalibrated (κ never computed)** — gold-set packets ready, founder labeling session pending; judge moving to CLI rails |
| H2 | **Memorization confound.** Famous-repo tasks (gin/flask/express); plain baseline ~0.90 may be recall-from-weights, not exploration. Never quantified. | All profiles inflated; exploration signal unknown per task | ADR-0016.9 drafted (proposed), closed-book probe specified |
| H3 | **Trace-capture completeness.** toolTraces come from ACP notifications; drops would be silent. | Trajectory metrics under-count | Scan complete + comparator designed (see `trace-capture-audit-2026-07-06.md`); build pending |
| H4 | **Anomaly taxonomy is closed-world.** Detectors only catch pre-registered patterns. | Unknown failure modes pass | Standing; judge + human trace review are the backstop |
| H5 | **Token accounting is a chars/4 proxy**, not provider-reported tokens. | SPT absolute values approximate (ratios robust) | Documented in ADR-0016.6; real token capture is a follow-up |
| H6 | **Sample economics.** n=5 trials/cell; thin-margin gates swing on single episodes. | Verdict fragility on close calls | Stopping bounds + at_least(n) reducers queued (ADR-0025 stats follow-up) |
| H7 | **Ceiling effects.** Correctness 0.89–0.97 across profiles — tasks may be too easy to discriminate. | Improvements invisible | Corpus refresh criterion: closed-book gap (H2) becomes the task-acceptance bar |

## Standing rules

- Every full-rigor run gets an independent cross-family recomputation
  before its verdict is cited externally (precedent: 2026-07-05 audit).
- Worker/eval reports are never trusted over artifacts — verify by
  recompute or direct read (incident log: 2026-07-06 worker rebase).
- A new metric, scorer, or judge enters this ledger as a HOLE first and
  moves to Verified only with named evidence.
