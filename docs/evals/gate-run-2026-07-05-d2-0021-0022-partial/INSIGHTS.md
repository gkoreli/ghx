# INSIGHTS — mining gate-run-2026-07-05 (NEW 54-ep D2/0021/0022 + OLD 90-ep M4)

Analyst: Fable (Opus 4.8), 2026-07-05. READ-ONLY artifact analysis; no live
agents, no API calls. Every number below recomputes from the committed episode
JSONs with a from-scratch Python reimplementation of `internal/sidecar/evals/
rewards.go` + `anomalies.go`. **Provenance check: my reimplementation reproduces
the committed per-episode `correctness` and `evidence` rewards exactly — 0/54
mismatches (NEW), 0/90 mismatches (OLD)** — so the deltas I compute below are
apples-to-apples against the shipped scorer, not a re-derivation with drift.
Scripts: `/tmp/full.py`, `/tmp/seq.py`, `/tmp/spt.py` (local, regenerable).
Everything here is **PRELIMINARY** (NEW is 3<5 trials/cell); labels kept on
every citable-looking number per the visibility/truthfulness tenet.

## Executive summary (15 lines)

1. **The single most important number — the instrument-defect magnitude.** Re-
   scoring NEW multi-turn sidecar episodes over the UNION of all turn reports
   (vs final-report-only, the shipped bug): mean Δcorrectness **+0.278**,
   Δevidence **+0.226**, Δoverall **+0.084** (n=6 multi-turn sidecar eps).
2. Projected post-fix NEW ghx-sidecar means: correctness **0.866 → 0.958**,
   evidence **0.808 → 0.883**. **G1 ratio 0.921 → 1.020** — the thin G1 margin
   in the README is entirely an artifact of the scorer, not the build.
3. The defect is build-specific: OLD (M4) union gap is Δoverall **+0.003** — the
   old build repeated all files each turn, so final-only scoring caught them.
   The NEW build's delta-reporting (the *desired* compression) is what the
   ruler penalizes. gin-routing sidecar correctness collapsed 1.00 (OLD) → 0.44
   (NEW) for this reason alone; union restores it to 1.00.
4. **Parse-loss is genuinely fixed.** NEW: 0 `sidecar_report_missing`, 0
   `block_unparsed`, 0 `retried`, 0 `coerced`. OLD: 5 missing (4 eps), 5
   unparsed (4 eps), 3 retried (2 eps). ADR-0021's contract eliminated them.
5. **Thinking is real but low-yield.** Captured in 18/18 sidecar eps (mean 537
   chars, max 1268) + 23 reasoning records in OTLP `logs.jsonl`; **0/36** in
   plain/ghx. Correlation with correctness r=**0.14** (weak). Cost: NEW sidecar
   88.9s vs OLD 54.4s (**+63%** latency).
6. **Sequential stopping (ADR-0025 D1) would NOT have saved episodes on M4.**
   Strict worst-case success-lock fires only at round 5 for G1 and round 4 for
   G2 — a passing thin-margin run gets no early stop. Savings must come from D2
   (baseline reuse) + D3 (parallelism), not D1.
7. **Compression holds:** 15.7× (NEW) / 25.0× (OLD) main-agent-char reduction.
   **SPT: sidecar 0.58 (NEW) / 0.92 (OLD) vs ~0.04 baselines = 13–21× signal
   per main-agent token** (all PRELIMINARY).
8. **Training corpus is ready:** SFT-eligible 53/54 (NEW), 81/90 (OLD);
   reward-ranked DPO pairs (gap≥0.15) 64 (NEW), 261 (OLD).
9. Worst-episode taxonomy: the deepest failures are **instrument false-zeros**
   (express `lib/router/index.js` unacceptable-claim; direct-profile 0.5
   evidence floor), not agent failures — a judge scorer would catch them.

---

## P1 — Variance & sample economics (→ ADR-0025)

Per-cell trial variance (pooled across all 18 cells/corpus; `pstdev` within cell):

| corpus | reward | mean within-cell std | max std | mean range | max range |
|---|---|---:|---:|---:|---:|
| NEW | correctness | 0.081 | 0.471 | 0.181 | **1.000** |
| NEW | evidence | 0.065 | 0.471 | 0.145 | 1.000 |
| NEW | overall | 0.037 | 0.142 | 0.086 | 0.347 |
| OLD | correctness | 0.101 | 0.400 | 0.241 | **1.000** |
| OLD | evidence | 0.053 | 0.343 | 0.140 | 1.000 |
| OLD | overall | 0.035 | 0.088 | 0.088 | 0.245 |

**Single trials swing hard on the component rewards** (correctness range hits
the full [0,1] in both corpora) but **overall is stable** (max range 0.35 NEW /
0.25 OLD) — the averaging over 6 rewards damps single-component swings. Worst
swinging cells: NEW `hono-middleware/ghx-sidecar` overall [0.61,0.80,0.95];
NEW `gin-routing/ghx` [0.58,0.62,0.83]; OLD `express-router-location/ghx-sidecar`
[0.70,0.82,0.84,0.93,0.94] (correctness bimodal 0.5/1.0 from the unacceptable-
claim binary — see P4).

**Power analysis** (SE of a profile mean over 6 tasks × n trials =
pooled_sd/√(6n); pooled correctness sd: NEW-sidecar 0.105, OLD-sidecar 0.123,
NEW-plain 0.216):

| arm | n=1 | n=2 | n=3 | n=5 |
|---|---:|---:|---:|---:|
| sidecar SE (corr) | 0.043–0.050 | 0.030–0.035 | **0.025–0.029** | 0.019–0.022 |
| sidecar 95% CI ± | 0.084–0.098 | 0.060–0.069 | **0.049–0.057** | 0.038–0.044 |
| plain SE (worst) | 0.088 | 0.062 | 0.051 | 0.039 |

Interpretation: the observed passing G1 margin (sidecar mean − relative floor)
is ~0.07 (OLD 0.908−0.837). At **n=3** the sidecar 95%-CI half-width (~0.057) is
already below that margin — **3 trials suffice for a wide-margin gate**; the
value of 5 is headroom for thin-margin gates (exactly NEW's defect-suppressed
G1). Plain's high variance (bimodal correctness) is the real precision sink, not
the sidecar.

**Would ADR-0025 D1 sequential-stopping have fired early on M4?** Simulated
round-by-round (planned 5 trials/cell; worst case = remaining sidecar/evidence
eps score 0, remaining ghx eps score 1):

| round | banked eps | G1 worst sidecar vs 0.9×ghx_worst | G2 worst evidence ≥0.70 |
|---|---:|---|---|
| 1 | 18 | 0.192 vs 0.853 — open | 0.179 — open |
| 2 | 36 | 0.367 vs 0.853 — open | 0.363 — open |
| 3 | 54 | 0.550 vs 0.853 — open | 0.546 — open |
| 4 | 72 | 0.742 vs 0.845 — open | 0.738 — **LOCK** |
| 5 | 90 | 0.908 vs 0.837 — **LOCK** | 0.871 — LOCK |

**Finding: strict worst-case D1 saves nothing on a passing run near threshold.**
G1 does not lock until the final round; G5 (safety) by ADR text can never lock
early (a future violation is always possible); no gate is failing so futility
never fires. The `/30` denominator makes zero-scored remaining episodes too
punishing for the worst-case bound to close early.

**So what:**
- → ADR-0025: do **not** rely on D1 to compress *passing* confirmatory runs; the
  ~20–40 min target (D5) must come from D2 (baseline reuse) + D3 (3-way
  parallelism). D1's real value is **futility** (fast NOT-SUPPORTED), not success.
- → ADR-0025: if early-stop on *passing* runs is wanted, add a CI-based lock
  (mean − 1.96·SE ≥ threshold) as a pre-registered alternative to pure worst-
  case; at n=3 the sidecar CI already clears a 0.07-margin gate.
- → ADR-0025: standard trials can drop to **3** for wide-margin gates; keep 5
  only for the historically thin gate (G1) and the high-variance plain arm.

## P2 — Behavioral diff OLD→NEW & the defect magnitude (→ persona / judge / scorer-fix)

Per-task correctness, ghx vs sidecar, both corpora:

| task | NEW ghx | NEW sc | NEW Δ | OLD ghx | OLD sc | OLD Δ |
|---|---:|---:|---:|---:|---:|---:|
| express-router-location | 1.00 | 1.00 | 0.00 | 0.80 | 0.70 | +0.10 |
| flask-routing | 1.00 | 1.00 | 0.00 | 1.00 | 1.00 | 0.00 |
| ghx-mapengine | 0.64 | 0.83 | **−0.19** | 0.78 | 0.85 | −0.07 |
| gin-routing | 1.00 | **0.44** | +0.56 | 1.00 | 1.00 | 0.00 |
| hono-middleware | 1.00 | 1.00 | 0.00 | 1.00 | 1.00 | 0.00 |
| openai-node-streaming | 1.00 | 0.92 | +0.08 | 1.00 | 0.90 | +0.10 |

The one apparent regression (gin sidecar 1.00→0.44) is **100% instrument, 0%
build**: the new build delta-reports (final turn = tree.go only, 70-char text);
final-only scoring drops routergroup.go/gin.go. Union fixes it.

**Union-of-reports recomputation (final → union), NEW multi-turn sidecar:**

| episode | corr | evid | overall |
|---|---|---|---|
| gin-routing_ghx-sidecar_1783309817215 | 0.58→**1.00** (+0.42) | 0.94→0.97 | 0.899→0.972 |
| gin-routing_ghx-sidecar_1783311354381 | 0.58→**1.00** (+0.42) | 0.33→0.67 (+0.33) | 0.800→0.925 |
| gin-routing_ghx-sidecar_1783313384747 | 0.17→**1.00** (+0.83) | 0.67→0.67 | 0.793→0.932 |
| hono-middleware_ghx-sidecar_1783310129190 | 1.00→1.00 | **0.00→1.00 (+1.00)** | 0.607→0.774 |
| hono-middleware_ghx-sidecar_1783311690396 | 1.00→1.00 | 1.00→1.00 | 0.797→0.797 |
| hono-middleware_ghx-sidecar_1783313720066 | 1.00→1.00 | 1.00→1.00 | 0.954→0.954 |
| **AGG (n=6)** | **+0.278** | **+0.226** | **+0.084** |

`hono...1783310129190` is the cleanest proof: turn-0 report has 2 verified
claims *with* code evidence; the final (delta) turn report has 0 verified → the
scorer reads evidence **0.00** on an episode that did cite evidence. Union → 1.00.

**Projected post-fix NEW ghx-sidecar profile:** correctness **0.866 → 0.958**,
evidence **0.808 → 0.883**, **G1 ratio 0.921 → 1.020**. OLD union gap is
negligible (Δoverall +0.003) — confirming the defect only bites the compressing
build.

Other behavioral deltas (means): tool-calls sidecar 13.6 (NEW) vs 10.2 (OLD);
main-agent chars sidecar 4884 (NEW) vs 3477 (OLD) — the new build's sidecar
touches slightly more but still 15.7× under ghx.

**So what:**
- → scorer-fix/ADR: **ship union-of-reports scoring.** It is worth +0.093
  correctness on the sidecar profile and moves G1 from "thin pass" to
  ratio>1.0. Every multi-turn verdict without it is a floor that penalizes the
  product's headline behavior.
- → judge: the union gap is exactly the class a whole-trajectory judge scorer
  (ADR-0023.1) resolves natively; deterministic union-scoring is the cheap
  interim, judge is the durable fix.

## P3 — Thinking & latency (→ persona / marketing)

| corpus | profile | think chars mean | max | eps w/ thinking | duration ms mean | toolcalls |
|---|---|---:|---:|---|---:|---:|
| NEW | plain | 0 | 0 | 0/18 | 110,575 | 12.3 |
| NEW | ghx | 0 | 0 | 0/18 | 103,294 | 13.3 |
| NEW | ghx-sidecar | **537** | 1268 | **18/18** | 88,932 | 13.6 |
| OLD | plain | 0 | 0 | 0/30 | 66,999 | 11.8 |
| OLD | ghx | 0 | 0 | 0/30 | 65,491 | 12.0 |
| OLD | ghx-sidecar | 0 | 0 | 0/30 | 54,444 | 10.2 |

- Thinking is captured **only in the sidecar profile** (persona-driven, per the
  new build) — 18/18 episodes, plus **23 reasoning records** in OTLP
  `logs.jsonl` (`gen_ai.output.messages` parts, GenAI convention). Plain/ghx
  carry none.
- **Weak payoff:** think-chars vs correctness r=**0.14** (n=18) — thinking did
  not visibly move scores at this scale.
- **Latency cost is real:** NEW sidecar 88.9s vs OLD 54.4s (**+63%**); NEW
  plain/ghx also up (~110s/103s vs 67s/65s) so not all attributable to thinking,
  but the sidecar's thinking is a plausible contributor. NEW durations may also
  reflect parallelism/rate-limit effects — do not cite as clean latency.

**So what:**
- → persona: `display=summarized` thinking buys ~537 chars/episode for r=0.14
  correlation and +63% latency. Consider gating thinking to structural/multi-
  turn questions rather than universal-on; measure SPT-per-second, not just SPT.
- → marketing: thinking traces are now a **training asset** (P6, ADR-0017.1 D1
  `<think>` export) even if their eval-score yield is flat.

## P4 — Failure-mode taxonomy (→ judge / M7)

Worst 12 episodes across both corpora (by overall):

| corpus | episode | O | C | E | T | tc | failure class |
|---|---|---:|---:|---:|---:|---:|---|
| NEW | gin-routing_ghx_1783313256903 | 0.583 | 1.00 | 0.50 | 1.00 | 14 | **direct-profile 0.5 evidence floor + memory=0** |
| OLD | gin-routing_ghx_1783294157688 | 0.583 | 1.00 | 0.50 | 1.00 | 11 | same |
| OLD | gin-routing_ghx_1783295296326 | 0.583 | 1.00 | 0.50 | 1.00 | 14 | same |
| OLD | gin-routing_ghx_1783296534553 | 0.583 | 1.00 | 0.50 | 1.00 | 12 | same |
| OLD | gin-routing_ghx_1783297736205 | 0.583 | 1.00 | 0.50 | 1.00 | 14 | same |
| OLD | gin-routing_ghx_1783298949511 | 0.583 | 1.00 | 0.50 | 1.00 | 10 | same |
| NEW | express-router-location_plain_1783310655275 | 0.600 | **0.00** | 1.00 | 1.00 | 8 | **unacceptable-claim zero (lib/router/index.js)** |
| OLD | express-router-location_ghx_1783293764685 | 0.600 | 0.00 | 1.00 | 1.00 | 3 | same |
| OLD | express-router-location_plain_1783298460843 | 0.600 | 0.00 | 1.00 | 1.00 | 7 | same |
| NEW | hono-middleware_ghx-sidecar_1783310129190 | 0.607 | 1.00 | **0.00** | 1.00 | 9 | **union defect (evidence in turn-0, lost)** |
| NEW | gin-routing_ghx_1783311240390 | 0.625 | 1.00 | 0.50 | 1.00 | 16 | direct-profile evidence floor |
| NEW | ghx-mapengine_ghx_1783311051515 | 0.637 | **0.25** | 1.00 | 0.93 | 10 | **shallow exploration (README-anchored, stopped early)** |

Taxonomy (10+ worst):
1. **Instrument false-zeros (dominant, ~7/12):** direct-profile evidence caps at
   0.50 when the answer text lacks a `path.ext` or `file:line` token the regex
   accepts — the gin ghx agent *did* the work (correctness 1.0) but phrased
   citations un-regexably. The express `lib/router/index.js` unacceptable-claim
   is arguably a *correct* real-world answer marked wrong. **Neither is an agent
   failure.**
2. **Union defect (1/12):** hono sidecar, covered in P2.
3. **Genuine shallow exploration (1/12):** `ghx-mapengine_ghx_1783311051515`
   correctness 0.25 — the agent anchored on the README pointer to
   `internal/mapengine/` and stopped before confirming goast.go/treesitter.go/
   types.go. This is a **real** exploration-depth failure.

**Which failures the judge catches that gates miss:** classes 1 and 2 — a
trajectory-aware judge reads the actual evidence and the whole task, so it
neither zeros a regex-un-citable-but-correct answer nor loses turn-0 evidence.
**Which M7 escalation (codemap) would fix:** class 3 — `ghx-mapengine` is a
structural, multi-file "how does the engine select itself" question; a codemap
tier that forces breadth before answering would push correctness 0.25→~1.0. This
is the only clear structural-question failure in the bottom 12, and it is on the
`ghx` (non-sidecar) profile — evidence M7 helps the direct agent most.

**So what:**
- → judge: 8 of the bottom 12 are scorer artifacts, not agent errors — the judge
  scorer's first job is to stop paying for false-zeros that make gate margins
  look thin.
- → M7: `ghx-mapengine` is the pre-registered structural-failure witness; use it
  as the codemap-tier acceptance case.

## P5 — Where ghx-profile beats sidecar (honest lens) (→ persona)

Tasks where direct `ghx` outscores the sidecar in *both* corpora on correctness:
**openai-node-streaming** (NEW +0.08, OLD +0.10) and **express-router-location**
(OLD +0.10; NEW tied at 1.00). Conversely the **sidecar beats ghx on
ghx-mapengine in both** (NEW +0.19, OLD +0.07) — so the persona is not uniformly
behind.

What the direct ghx agent does that the sidecar persona doesn't, on
openai/express: it keeps exploring in one unbroken context until it has *both*
required files, whereas the sidecar's compression discipline sometimes ships a
tight report naming one file and paraphrasing the second (openai sidecar 0.92 =
missed one of `ResponseStream`/`AssistantStream` variants). The gin/hono
"regressions" are the union artifact (P2), not a doctrine gap.

**So what:**
- → persona: add a **coverage-before-compression** clause — when a question
  names or implies N subsystems (routing = register + match + tree), the report
  must enumerate all N relevant files *by path* before compressing prose. This
  targets the openai/express misses without touching compression on single-file
  questions. (Transcript evidence: `openai-node-streaming_ghx-sidecar` reports
  name one streaming file and describe the other; direct ghx lists both.)

## P6 — Training-corpus preview (→ ADR-0017.1)

Filters applied (ADR-0017.1 D4): safety==1.0, overall≥0.6, exclude breaking
anomalies (`sidecar_report_missing`/`BLOCKED`).

| corpus | SFT-eligible | ghx-sidecar | ghx | plain |
|---|---:|---:|---:|---:|
| NEW | **53/54** | 18 | 17 | 18 |
| OLD | **81/90** | 26 | 25 | 30 |

(NEW loses 1 ghx episode to the overall<0.6 floor; OLD loses 9, incl. the 4
report-missing sidecar eps + low-overall direct eps.)

Reward-ranked DPO pairs (same task, |Δoverall|≥0.15, safety==1 both sides):

| corpus | total | express | flask | mapengine | gin | hono | openai |
|---|---:|---:|---:|---:|---:|---:|---:|
| NEW | **64** | 9 | 8 | 15 | 11 | 9 | 12 |
| OLD | **261** | 32 | 47 | 46 | 58 | 31 | 47 |

**So what:**
- → ADR-0017.1: both corpora clear the KTO/SFT bar today; a first offline
  exporter cut yields **134 SFT records** and **325 reward-ranked DPO pairs**
  combined (pre-dedup/decontam) — enough to test the pipeline before judge
  calibration. Note: the union-scoring fix (P2) will *re-rank* NEW multi-turn
  pairs, so freeze DPO ranking **after** the scorer fix, not now.
- → ADR-0017.1: NEW sidecar thinking (P3) is the `<think>`-prefix training bonus
  — 18 episodes with reasoning ready to export.

## P7 — Claims inventory (marketing lens, tenet-strict) (→ marketing)

**Parse-loss, verified old vs new** (the claim the mandate asked to check):

| anomaly | OLD (M4) | NEW (D2/0021/0022) |
|---|---|---|
| sidecar_report_missing (breaking) | 5 (4 eps) | **0** |
| sidecar_report_block_unparsed (soft) | 5 (4 eps) | **0** |
| sidecar_report_retried (soft) | 3 (2 eps) | **0** |
| sidecar_report_coerced (soft) | 0 | **0** |
| BLOCKED answers | 0 | **0** |

**Claims the data DOES support (all PRELIMINARY, NEW 3<5 trials):**
- PRELIMINARY: ADR-0021's contract eliminated report parse-loss — **5→0
  report-missing, 5→0 unparsed, 3→0 retried** (NEW n=54 vs OLD n=90).
- PRELIMINARY: main-agent context compression **15.7×** (NEW, 76,899→4,884
  chars) / **25.0×** (OLD, 86,851→3,477).
- PRELIMINARY: **zero safety violations** — 18/18 (NEW) and 30/30 (OLD) sidecar
  episodes at safety 1.0.
- PRELIMINARY: signal-per-token (ADR-0016.6, main-agent level) sidecar **0.58**
  (NEW) / **0.92** (OLD) vs plain/ghx ~**0.04** — **13× (NEW) to 21× (OLD)** more
  verified signal per main-agent token.
- PRELIMINARY (projected, post-scorer-fix): sidecar correctness **0.958**,
  evidence **0.883**, G1 ratio **1.020** — label explicitly "projected under
  union-of-reports scoring, not yet run."

**Claims the data does NOT yet support:**
- Any citable go/no-go verdict for the NEW build (3<5 trials; `dataSufficient:
  false` in verdict.json).
- A clean latency claim (NEW durations confounded by thinking + possible
  parallelism; ADR-0025 D3 excludes parallel-run durations).
- SPT improvement NEW-vs-OLD (NEW SPT is *lower* — 0.58<0.92 — because the
  correctness defect suppresses the numerator; do not claim a SPT gain until the
  scorer fix + a 5-trial rerun).
- Any haiku-subject claim (ADR-0025 D4 arm not yet run).

**So what:**
- → marketing: the two safe headline claims today are **"zero report parse-loss
  (5→0)"** and **"15.7× context compression"** — both binary/ratio facts,
  robust to the scoring defect. Hold correctness/SPT-improvement claims until
  the union-scoring rerun.

---

## Data quality notes

- **Schema drift between corpora:** NEW episodes embed `checks` inline; OLD
  episodes do not — OLD scoring required loading `internal/sidecar/evals/
  testdata/tasks/*.json` by `taskId`. Recompute still matched 0/90, confirming
  the fixtures are the ones the OLD run used.
- **Anomaly fields not stored per-episode:** neither corpus writes an
  `anomalies` array on the episode JSON (confirmed absent on NEW too); I derived
  all P7 counts from the raw `turns[].report.answer` prefixes + `reportRetried`/
  `reportCoerced` flags, matching `anomalies.go` `DetectAnomalies` logic and the
  OLD `verdict.json` anomaly block exactly (5/5/3). This reproduces the
  auditor's "two episodes omit anomaly fields" finding as a general property.
- **NEW verdict.json has no `anomalies` block** (OLD does) — consistent with
  zero anomalies detected, but the field's absence (vs an explicit empty array)
  is a minor provenance gap; recommend emitting `"anomalies": []` explicitly.
- **Duration confound:** `durationMs` summed over turns; NEW parallelism status
  not tagged on these episodes (`ghx.eval.parallel` not present), so NEW-vs-OLD
  latency is indicative only, not citable.
- **Contamination (from audit, not re-litigated):** OLD `ghx-mapengine` plain
  eps read `docs/adr/0013`; those eps are still SFT-counted here — the ADR-0017.1
  13-gram decontam pass (not yet built) must drop them before any training cut.
- **Union recomputation caveat:** I merged `turns[].report` structured fields
  (verified/relevantFiles/evidence/commandsRun) and re-ran the *unmodified*
  reward formulas; I did not alter trajectory/compression/memory/safety, so the
  projected `overall` holds those four at their committed values. This matches
  the intended scorer fix (union feeds correctness+evidence only).
