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
| H1 | **Checks measure fact recall, not quality.** Substring gates can score a lucky mention. | Quality claims overstate; first sweep shows ceiling compression (judge means 4.8-4.9, k=1 same-family) — range-restriction risk for κ | **Sweep done** (2026-07-06, `judge-sweep-2026-07-06/`): 140/140 candidates scored via claude -p (SAME-FAMILY + PRELIMINARY + k=1 — triage only); disagreement 19.3% > 15% bound → INVESTIGATE, all one-directional (gates-fail/judge-good, gin multi-turn low band); ranked labeling order ready — **founder labeling session is the sole remaining unlock for κ**. Cross-family follow-up (2026-07-06, `judge-sweep-2026-07-06/cross-family/`): k=3 re-score of all 27 disagreement candidates on the codex/gpt-5.5 PRIMARY rail (no self-preference confound on these Claude-authored episodes) — 27/27 CONFIRMED judge-good, 0/27 sided with the gates, so the disagreement is not explained by same-family bias; still PRELIMINARY/uncalibrated, founder labeling remains the sole κ unlock |
| H2 | **Memorization confound.** Famous-repo tasks (gin/flask/express); plain baseline ~0.90 may be recall-from-weights, not exploration. | Residual risk now bounded: closed-book scores are a **lower bound** on recall because the eval questions don't name the repo — a repo-named closed-book variant needs its own pre-registered ADR | **Measured** (ADR-0016.9 closed-book probe, 2026-07-06): with the exact eval prompts and `tools:[]`, closed-book corpus mean 0.067 vs 0.926 open-book; no task flags recall-dominated/memorization-risk; 5/6 exploration-bearing; flask-routing shows real partial recall (mean 0.40, `add_url_rule` from weights, stale paths rejected) — first in line for a harder variant. Artifacts: `docs/evals/memorization-audit/` (30 episodes, deterministic scores, zero tool activity verified) |
| H3 | **Trace-capture completeness.** toolTraces come from ACP notifications; drops would be silent. | Trajectory metrics under-count | Comparator built (ADR-0016.10: raw-SDK audit persisted per turn, `trace_capture_gap` soft anomaly). **First instrumented reading (2026-07-06, n=5 episodes / 6 turns): zero gaps** — detector armed on every turn (`rawSDK` present, 117–303 live messages/turn), raw tool_use blocks == captured toolTraces 53/53, zero `trace_capture_gap` anomalies; provider usage persisted with `source: result` on all turns (H5 hook live). Small-n spot check, not a closure: stays open until a full-size instrumented run reads the same zero. Artifacts: `docs/evals/spot-instrumented-2026-07-06/` |
| H8 | **Resumed turns run unsteered** (found by the H3 build, ADR-0016.10 D2 note): LoadSession carried no `_meta`, so turn 2+ of multi-turn episodes and production follow-ups ran without persona/tools-allowlist/budgets/model-pin. | Multi-turn eval turns measured a partially-unsteered agent; production resumes ditto | **Fixed** (ADR-0020.2, commit `726db98`): LoadSession now carries the full `BuildSessionMeta` bag — steering + raw-audit flag, one map — in `RunTurnWithOptions` and both daemon-worker paths; wire-pinned by mockagent `NEW_META`/`LOAD_META` byte-equality tests (eval + production mode, wrap-up turn included). Live-verified 2026-07-06: fresh-adapter resume on turn 2 showed reasoning tokens (124) — impossible under the adapter default `display:omitted` — and the fresh adapter echoed accepted `allowedTools` (`CLAUDE_SDK_CAN_USE_TOOL_SHADOWED` warning on the resumed session). Honest note: all committed multi-turn episodes to date had turn 2+ partially unsteered; past verdicts are **not** rescored, the effect direction is unknown, and the delta is measured by the next pre-registered run that includes multi-turn tasks |
| H4 | **Anomaly taxonomy is closed-world.** Detectors only catch pre-registered patterns. | Unknown failure modes pass | Standing; judge + human trace review are the backstop |
| H5 | **Token accounting is a chars/4 proxy**, not provider-reported tokens. | SPT absolute values approximate (ratios robust); real-token variant coverage-gated | **Promoted** (ADR-0016.11, 2026-07-06): verdicts now report a real-token SPT variant + run economics (costUsd) ALONGSIDE chars/4 — both always shown, neither primary yet; UNAVAILABLE label when any episode lacks usage (all pre-0016.10 runs); switch criterion pre-registered for a future ADR after N dual-reported runs |
| H6 | **Sample economics.** n=5 trials/cell; thin-margin gates swing on single episodes. | Verdict fragility on close calls | **Partially closed** (ADR-0025.2, 2026-07-06): `at_least(n)` reducer landed for G2/G4 (opt-in, existing gates unaffected by default) plus a deterministic `FRAGILE(1-episode margin)` annotation on every gate — every rendered verdict now shows whether the call is one flipped trial away from the opposite outcome. Sequential stopping bounds (ADR-0025 D1) were already shipped. Still open: G1's relative comparison and G3's unbounded-chars ratio have no `at_least(n)` form yet (need trial pairing / an accepted char ceiling); no CLI wiring to opt a live gate run into `at_least(n)` yet. Evidence: `internal/sidecar/evals/gate_reducers.go`, `internal/sidecar/evals/gate_reducers_test.go` |
| H7 | **Ceiling effects.** Correctness 0.89–0.97 across profiles — tasks may be too easy to discriminate. | Improvements invisible | Corpus refresh criterion: closed-book gap (H2) becomes the task-acceptance bar |

## Standing rules

- Every full-rigor run gets an independent cross-family recomputation
  before its verdict is cited externally (precedent: 2026-07-05 audit).
- Worker/eval reports are never trusted over artifacts — verify by
  recompute or direct read (incident log: 2026-07-06 worker rebase).
- A new metric, scorer, or judge enters this ledger as a HOLE first and
  moves to Verified only with named evidence.
