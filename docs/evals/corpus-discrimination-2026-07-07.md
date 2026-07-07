# Corpus discrimination analysis — toward H7 ceiling refresh (workstream C7)

**Question (TRUST hole H7):** are the current recon-eval corpus tasks too
easy to discriminate the three profiles (`plain` vs `ghx` vs `ghx-sidecar`)?
If a task pins all three profiles at ceiling, a sidecar correctness
regression is invisible there ("improvements invisible", `docs/evals/TRUST.md`
H7). This doc tabulates per-task discrimination, ranks tasks worst-first,
shortlists the least-discriminating, and **PROPOSES** (does not implement)
harder replacements that would clear the H2 closed-book-gap acceptance bar.

This is a **read-only analysis**. Nothing here edits `TRUST.md`, the corpus,
or any code. If Fable accepts these findings at merge, H7 in
`docs/evals/TRUST.md` should be updated by Fable then — not by this worker.

## Provenance — who computed what, from which artifacts

Every number below is recomputed from committed artifacts, not copied from a
worker summary. Two sources, both committed:

- **Open-book per-task/profile correctness** — recomputed by this worker from
  `docs/evals/gate-run-2026-07-06-fixbatch/<task>_<profile>_<ts>.json`
  (`.rewards.correctness` per episode), grouping by `taskId|profile` and
  **excluding the 7 contaminated ghx-mapengine episodes** named in
  `gate-run-2026-07-06-fixbatch/verdict.json` `.notes` (same validity filter
  the gate verdict applies, ADR-0016.8 D2). Result reproduced the committed
  `docs/evals/memorization-audit/closed-book-summary.json`
  `.tasks[].openBook.meanCorrectnessByProfile` field-for-field.
- **Closed-book per-task correctness** — recomputed by this worker from
  `docs/evals/memorization-audit/episodes/<task>_closed-book_trial-*.json`
  (`.rewards.correctness`), grouped by `taskId`. Reproduced
  `closed-book-summary.json` `.tasks[].meanCorrectness` / `.maxCorrectness`
  and the README corpus-wide mean **0.067** exactly.

Deterministic scorer only (`internal/sidecar/evals/rewards.go`
`ComputeRewards`), scoring commit `c086d38`
(`docs/evals/memorization-audit/manifest.json` `.scoringCodeCommit`). No
judge, no LLM scoring. Subject `claude-sonnet-5`, wrapper sha256 `30cb9d72…`,
task-corpus sha256 `8143854941d1…` (same manifest). Analysis worktree HEAD
`4d9a3e1`.

Recompute commands (zero-token, read-only over committed JSON):

```sh
# Open-book cells (with contamination filter)
cd docs/evals/gate-run-2026-07-06-fixbatch
EX="ghx-mapengine_ghx_1783321706515 ghx-mapengine_plain_1783319925282 \
ghx-mapengine_plain_1783321736361 ghx-mapengine_plain_1783322647141 \
ghx-mapengine_plain_1783322755936 ghx-mapengine_plain_1783322922250 \
ghx-mapengine_plain_1783323010296"
find . -maxdepth 1 -name '*_*_[0-9]*.json' -print0 | xargs -0 jq -n --arg excl "$EX" '
  ($excl|split(" ")) as $ex
  | [inputs | {id,taskId,profile,c:.rewards.correctness}]
  | map(select(.id as $i | ($ex|index($i))|not))
  | group_by(.taskId+"|"+.profile)
  | map({k:(.[0].taskId+"|"+.[0].profile),n:length,mean:((map(.c)|add)/length)})
  | sort_by(.k)[] | "\(.k) n=\(.n) mean=\(.mean)"'

# Closed-book cells
cd docs/evals/memorization-audit
find episodes -name '*.json' -print0 | xargs -0 jq -n '
  [inputs | {taskId,c:.rewards.correctness}] | group_by(.taskId)
  | map({t:.[0].taskId,mean:((map(.c)|add)/length),max:(map(.c)|max)})
  | sort_by(.t)[] | "\(.t) cb=\(.mean) max=\(.max)"'
```

## The full table (all numbers recomputed, sources attributed)

Task specs: `internal/sidecar/evals/testdata/tasks/<id>.json`. Open-book cells
& closed-book from the two sources above. `n` per open-book cell after the
contamination filter: 5 for every cell except `ghx-mapengine` (plain 5, ghx 6,
sidecar 7 — the run's 30/31/32 split lands entirely on this one task).

| Task | repo | plain | ghx | sidecar | **min-profile** (ceiling headroom = 1−min) | **spread** (max−min) | **sidecar−ghx** (regression visibility) | **closed-book** | pooled open | **exploration gap** (open−cb) |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| express-router-location | expressjs/express | 0.500 | 1.000 | 0.900 | 0.500 (0.500) | **0.500** | −0.100 | 0.000 | 0.800 | 0.800 |
| openai-node-streaming | openai/openai-node | 0.950 | 1.000 | 0.800 | 0.800 (0.200) | 0.200 | **−0.200** | 0.000 | 0.917 | 0.917 |
| ghx-mapengine | gkoreli/ghx | 0.900 | 0.722 | 0.893 | 0.722 (0.278) | 0.178† | +0.171† | 0.000 | 0.838 | 0.838 |
| gin-routing | gin-gonic/gin | 1.000 | 1.000 | 1.000 | 1.000 (**0.000**) | **0.000** | 0.000 | 0.000 | 1.000 | 1.000 |
| hono-middleware | honojs/hono | 1.000 | 1.000 | 1.000 | 1.000 (**0.000**) | **0.000** | 0.000 | 0.000 | 1.000 | 1.000 |
| flask-routing | pallets/flask | 1.000 | 1.000 | 1.000 | 1.000 (**0.000**) | **0.000** | 0.000 | **0.400** | 1.000 | **0.600** |

† `ghx-mapengine`'s spread and sidecar−ghx sign are **confounded artifacts**,
not trustworthy discrimination — see the shortlist. The ghx cell dropped to
0.722 only after excluding 1 contaminated ghx episode (n=6), plain 0.900 after
excluding 6 (n=5); the ordering is *inverted* (ghx < plain), which is
anti-thesis and driven by the exclusion mechanics, not by task difficulty.

Reference aggregates (context only, from `gate-run-2026-07-06-fixbatch/verdict.json`
/ `verdict.md`): corpus means plain 0.892 / ghx 0.946 / sidecar 0.930 — the
0.89–0.97 band H7 quotes (`TRUST.md` H7). Closed-book corpus mean 0.067
(`memorization-audit/README.md`).

## Two discrimination axes (both named in the H7/H2 framing)

- **Axis A — H7 ceiling / profile discrimination.** Can the task separate the
  profiles at all, i.e. would a sidecar correctness change *show*? Measured by
  **spread** (max−min across the three profiles) and **min-profile** (all
  three near 1.0 ⇒ no room). Ceiling FAIL rule (aligned to G1's own 0.90
  relative floor and H7's 0.89–0.97 band): min-profile ≥ 0.90 ⇒ "near-ceiling
  across ALL profiles."
- **Axis B — H2 exploration / memorization.** Is the score genuine
  exploration or recall-from-weights? Measured by **closed-book** mean and the
  **exploration gap**. The pre-registered acceptance bar (ADR-0016.9,
  `memorization-audit/README.md`): **exploration-bearing = closed-book mean
  `< 0.40` AND gap `≥ 0.30`**. This is exactly the bar H7 says a refreshed
  corpus must clear ("closed-book gap (H2) becomes the task-acceptance bar",
  `TRUST.md` H7).

Per-task axis verdicts:

| Task | Axis A (ceiling) | Axis B (exploration-bearing) |
|---|---|---|
| express-router-location | PASS (min 0.500, spread 0.500) | PASS (cb 0.000, gap 0.800) |
| openai-node-streaming | PASS (min 0.800, spread 0.200) | PASS (cb 0.000, gap 0.917) |
| ghx-mapengine | PASS nominal / CONFOUNDED | PASS (cb 0.000, gap 0.838) |
| gin-routing | **FAIL** (min 1.000, spread 0.000) | PASS (cb 0.000, gap 1.000) |
| hono-middleware | **FAIL** (min 1.000, spread 0.000) | PASS (cb 0.000, gap 1.000) |
| flask-routing | **FAIL** (min 1.000, spread 0.000) | **FAIL** (cb 0.400 — not `< 0.40`) |

`flask-routing` is the only task that fails **both** axes. The pre-registered
ADR-0016.9 table already left it unlabeled ("lands in no pre-registered bin …
the strongest partial recall in the corpus and is the first candidate for a
harder variant", `memorization-audit/README.md`): the model recalled
`add_url_rule` from weights in 4/5 closed-book trials (0.5 each), so half its
open-book score is recall, not exploration.

## Discrimination ranking (worst → best)

Mechanical rule, fully reproducible from the table: sort by **spread ascending,
then exploration gap ascending** (least discriminating first), then apply the
confound note for `ghx-mapengine`.

1. **flask-routing — WORST.** Spread 0.000 (all three at 1.000, Axis A fail)
   *and* the smallest exploration gap 0.600 with closed-book 0.400 (Axis B
   fail — the corpus's only memorization-dominated task). Fails both axes; a
   double-fail. Definite replacement.
2. **gin-routing — ceiling.** Spread 0.000, min-profile 1.000 (Axis A fail).
   Cannot reveal any sidecar value. Strong exploration gap (1.000) so it is not
   memorization-broken, but it is ceiling-capped. Marginally worse than hono as
   the more heavily-memorized celebrity Go router (higher latent recall-drift
   risk over time; H2 names gin explicitly) — a near-tie with hono.
3. **hono-middleware — ceiling.** Spread 0.000, min-profile 1.000 (Axis A
   fail). Same ceiling problem as gin, strong exploration gap (1.000). Carries
   a `calibration` tag (secondary eval value), so slightly preferred to keep
   over gin if only one of the pair is hardened.
4. **ghx-mapengine — CONFOUNDED (replace for a different reason).** Nominal
   spread 0.178 ranks it above the ceiling pair mechanically, but that spread
   is a contamination-exclusion artifact with an *inverted* (anti-thesis)
   ordering, and the task is **self-referential** (answer lives in the subject
   repo's own `docs/adr/0013-ghx-map-command.md`) so its plain baseline is
   contamination-prone by construction — 7 exclusions + top-ups in the fixbatch
   run, the sole source of the 30/31/32 unequal-cell data-quality warning
   (`verdict.json` `.notes`; `gate-run-2026-07-06-fixbatch/README.md` routed
   this exact structural finding to the corpus backlog). Not a trustworthy
   discriminator despite the nonzero number.
5. **openai-node-streaming — GOOD (keep).** Spread 0.200 with the **largest
   real sidecar−ghx gap in the corpus (−0.200)**: the one task where the
   sidecar visibly costs correctness (sidecar 0.800 < plain 0.950 < ghx 1.000).
   TRUST H6 already notes the G1 `at_least(n)` reducer "exposes the
   openai-node-streaming cell the run mean absorbs" (`TRUST.md` H6). Strong
   exploration gap 0.917. This is currently the corpus's best sidecar-regression
   sentinel.
6. **express-router-location — BEST (keep, and the design template).** Spread
   0.500 — the plain baseline genuinely fails (0.500) the evidence-honesty
   trap ("is the router itself implemented inside this repository?" — it is
   NOT; Express delegates to an external `router` package), while ghx 1.000 and
   sidecar 0.900. Exploration gap 0.800. This is what a discriminating recon
   task looks like: a structural trap that recall/shallow-browse gets wrong.

## Shortlist — least-discriminating tasks to replace/harden

| Priority | Task | Why it must change | Fails |
|---|---|---|---|
| P0 | **flask-routing** | Ceiling on all 3 profiles *and* memorization-dominated (cb 0.400) | Axis A + Axis B |
| P1 | **gin-routing** | Ceiling — zero profile spread, sidecar delta invisible | Axis A |
| P1 | **hono-middleware** | Ceiling — zero profile spread, sidecar delta invisible | Axis A |
| P2 | **ghx-mapengine** | Self-referential ⇒ recurring contamination + unequal cells + inverted spread | Structural (contamination) |

Keep unchanged: `express-router-location` (best), `openai-node-streaming`
(best sidecar-regression sentinel). These two are the discrimination that the
whole 6-task corpus currently rests on.

## PROPOSED harder replacements / variants (NOT implemented)

Design recipe (already proven in-repo). The two tasks that discriminate today
(express, openai) work because of a **structural trap**: a delegation boundary
or a call-path that recall/shallow-browse gets wrong but exploration reveals.
The host-task corpus README (`internal/sidecar/evals/hosttask/corpus/README.md`)
codifies the memorization half: "mid-size, non-celebrity library internals …
fixes that require reading *this* pinned code, not recalling a famous repo",
enforced by a `memorizationCanary` (from-weights solve ⇒ reject). A refreshed
recon task should combine both: **non-recallable internals (clears H2) + a
structural trap that sinks the plain/recall baseline (breaks H7 ceiling).**

**Honesty caveat (visibility/truthfulness tenet).** The exact `expectedFiles`
/ `expectedSymbols` below are **candidate hypotheses from this worker's
knowledge, not verified against the pinned repos** — verifying them would
itself risk the staleness H2 flags. Each proposal must, at authoring time and
before commit: (a) pin `repo@sha`, (b) verify the paths/symbols against that
revision, and (c) run a closed-book canary (`tools:[]`, exact prompt) and
confirm closed-book mean `< 0.40` — same gate ADR-0016.9 and the host-task
canary already enforce. What is durable and load-bearing below is the
**discrimination mechanism**, not the path strings.

### R1 — replace `flask-routing` (P0): Flask→Werkzeug URL-matching delegation

- **Repo:** `pallets/flask` (delegation target `pallets/werkzeug`).
- **Question:** "When a request arrives, where is the URL actually *matched*
  to an endpoint — is that matching implemented in Flask itself, or delegated?"
- **Expected answer / candidate checks (verify at authoring):** Flask builds a
  `MapAdapter` via `create_url_adapter` (`src/flask/app.py` / `sansio/app.py`)
  and **delegates matching to Werkzeug's routing** (`MapAdapter.match`,
  `werkzeug/routing/map.py`). `unacceptableClaims`: any assertion that Flask
  implements route *matching* in-repo.
- **Why it clears H2:** the model recalls `add_url_rule` (that's *why* flask
  scores 0.40 closed-book today) but is very unlikely to recall the
  `create_url_adapter` → Werkzeug `MapAdapter.match` delegation with current
  paths from weights → closed-book should collapse toward 0.
- **Why it discriminates (breaks ceiling):** it is an express-style delegation
  trap kept inside the Python/Flask family. A recall/shallow agent will assert
  Flask matches its own routes (wrong); only exploration crosses the package
  boundary → plain baseline should drop off ceiling exactly as it does on
  express (plain 0.500), restoring profile spread.

### R2 — harden `gin-routing` (P1): radix-tree route *conflict* detection

- **Repo:** `gin-gonic/gin`.
- **Question:** "When two registrations would conflict in the routing tree
  (e.g. a `:param` wildcard and a static segment at the same position), how and
  where does gin detect and reject the conflict?"
- **Candidate checks (verify):** `tree.go` `addRoute`/`insertChild` and the
  `findWildcard` path; the explicit panic on conflicting wildcard segments.
  `avoidPaths`: `_test.go`, `testdata/`.
- **Why it clears H2:** the specific conflict-detection code path and its
  panic conditions are internal implementation detail, not a recallable famous
  API surface — closed-book should stay ~0.
- **Why it discriminates:** the current question ("where are routes
  registered / how are params captured") is answerable by naming
  `addRoute`/`RouterGroup`/`tree.go` — which is why all three profiles hit
  1.000. A conflict-detection question forces reading the actual insertion
  logic; a shallow/recall agent describes the happy path and misses it →
  spread returns. Keeps gin (parity with the celebrity-routing family) while
  removing the ceiling.

### R3 — harden `hono-middleware` (P1): multi-router selection & fallback

- **Repo:** `honojs/hono`.
- **Question:** "Hono ships several routers (RegExpRouter, TrieRouter,
  LinearRouter, SmartRouter). How does the framework decide which router to use
  at runtime, and where is the fallback when a router can't handle a path?"
- **Candidate checks (verify):** `SmartRouter` (`src/router/smart-router/`)
  tries registered routers in order and falls back on `UnsupportedPathError`;
  the `hono` vs `hono/quick` presets wire the router set. `avoidPaths`:
  `.test.ts`, `/runtime-tests/`.
- **Why it clears H2:** the router-selection ordering and fallback exception
  are non-obvious internals; not recallable from weights → closed-book ~0.
- **Why it discriminates:** the current compose-question resolves to
  `src/compose.ts` for everyone (ceiling 1.000). A selection/fallback question
  is the "engine selected per input" shape (the interesting property
  ghx-mapengine tried to test) but on a real third-party repo with no
  self-reference. Naive agents point at one router; exploration finds the
  SmartRouter fallback → spread returns.

### R4 — replace `ghx-mapengine` (P2): keep the "engine-per-input selection" shape, drop the self-reference

- **Primary repo:** `golang-migrate/migrate` (mid-fame, non-celebrity).
- **Question:** "How is the database driver and the migration *source* selected
  from a URL scheme (e.g. `postgres://…`, `file://…`), and where is the
  fallback when a scheme is unregistered?"
- **Candidate checks (verify):** scheme-keyed driver registries with
  `database.Open` / `source.Open` scheme dispatch (`database/driver.go`,
  `source/driver.go`); the "unknown driver" error path. `avoidPaths`:
  `_test.go`.
- **Alternative repo:** `alecthomas/chroma` — "how is the lexer selected for a
  filename, and where is the analyser fallback when no lexer matches by name?"
  (`lexers` registry, `Match`/`Analyse` fallback).
- **Why it clears H2:** scheme/registry dispatch internals are not recallable →
  closed-book ~0; neither repo is self-referential, so **the contamination
  surface disappears entirely** (no `contaminationPaths` needed, no exclusions,
  no unequal cells).
- **Why it discriminates:** preserves the genuinely interesting structural
  property ghx-mapengine was reaching for (a per-input engine/backend
  selection with a fallback) — which requires reading selection code, not
  guessing — while removing the self-reference that made its plain baseline
  read the answer doc. Expect real spread from the fallback-reasoning
  requirement, cleanly measured.

## Bottom line for Fable (H7 disposition)

- The corpus's discrimination currently rests on **two** tasks
  (`express-router-location`, `openai-node-streaming`). The other four are
  ceiling-capped (`gin`, `hono`, `flask`) or contamination-confounded
  (`ghx-mapengine`); `flask-routing` additionally fails the H2 memorization bar.
- Adopting R1–R4 would move the corpus from 2/6 to a projected 6/6 tasks that
  both break the H7 ceiling (nonzero profile spread) and clear the H2
  closed-book-gap acceptance bar — each verified by a pre-registered
  closed-book canary before commit, per ADR-0016.9 and the host-task recipe.
- **These proposals are not implemented.** If accepted, they should land as a
  pre-registered eval-corpus ADR (the measurement-freeze rule and the fixbatch
  README both require this before the next full run), and **`docs/evals/TRUST.md`
  H7 should be updated by Fable at merge** to record the refresh criterion as
  applied — not by this read-only worker.
