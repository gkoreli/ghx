---
title: "H7-refresh — eval corpus discrimination refresh R1–R4"
date: "2026-08-21"
status: "research"
thread: "sidecar-agentic-eval"
author: "background loop (corpus track)"
scope: "read-only research artifact; proposes, does not implement; maintained under docs/research/"
builds-on: "ADR-0016.13, docs/evals/TRUST.md H7 row, docs/NORTH_STAR.md C7"
---

# H7-refresh: Eval corpus discrimination refresh R1–R4

recon-eval tasks so correctness gates can detect real sidecar improvements.
ceiling refresh (H7)", `docs/NORTH_STAR.md:330`); the corpus is shared design
ground with C8's host-task evals (`docs/NORTH_STAR.md:331`).
committed artifacts; proposes (does not implement) R1–R4 replacements and a
pre-registration path. No existing file was modified; nothing committed.
**Repo state at authoring:** branch `mainline`, HEAD `719cb29` ("docs:
normalize engineering record spacing"), clean tree; corpus fixtures at
`internal/sidecar/evals/testdata/tasks/*.json` unchanged.

**Sources this artifact is grounded in:** `docs/evals/corpus-discrimination-2026-07-07.md`
(the C7 discrimination analysis whose R1–R4 proposals this artifact develops),
`docs/evals/TRUST.md` rows H2/H7/H9, ADR-0016.9 (`docs/adr/0016.9-memorization-confound-audit.md`),
ADR-0016.8 (`docs/adr/0016.8-measurement-fix-batch.md`), ADR-0019.1/0019.2
(`docs/adr/0019.1-discovery-tier.md`, `docs/adr/0019.2-discovery-eval-tasks.md`),
the committed run artifacts under `docs/evals/gate-run-2026-07-06-fixbatch/`
and `docs/evals/memorization-audit/`, and the task-fixture/registration code
under `internal/sidecar/evals/`. Every number is recomputable from those
artifacts (recompute commands in §8).

## 0. TL;DR

The recon correctness gate can currently detect a sidecar improvement or
regression on **2 of its 6 tasks** (`express-router-location`,
`openai-node-streaming`). The other four are structurally incapable of moving:
three sit at profile ceiling (spread 0.000) and one is contamination-confounded.
This artifact specifies **R1–R4**, four replacement/hardened tasks, each with a
discrimination mechanism, an H2 closed-book rationale, candidate checks, and
rejected alternatives; plus the pre-registration contract (per-task closed-book
canary < 0.40 before commit, gates untouched, D-G-style run plan) and the ADR
thread it should land in (**ADR-0016.13**, extending the `sidecar-agentic-eval`
family — thread choice justified in §7).

## 1. Problem statement — why 2/6 discriminating tasks blocks correctness-gate progress

**What the gate needs to do.** The ADR-0016.1 gate family (G1–G5) scores
recon episodes per task × profile and compares profile means; a correctness
gate can only "see" a sidecar improvement or regression on a task where the
profile means are not pinned together. TRUST row H7 names the hole: "**Ceiling
effects.** Correctness 0.89–0.97 across profiles — tasks may be too easy to
discriminate. Risk: improvements invisible" (`docs/evals/TRUST.md:35`). The C7
analysis (`docs/evals/corpus-discrimination-2026-07-07.md`) quantified it: the
corpus's discrimination power rests on **only two tasks**:

- `express-router-location` — profile spread 0.500 (plain genuinely fails the
  delegation trap at 0.500);
- `openai-node-streaming` — spread 0.200 and the corpus's only real
  sidecar−ghx regression signal (−0.200; TRUST H6 already notes the G1
  `at_least(n)` reducer "exposes the openai-node-streaming cell the run mean
  absorbs", `TRUST.md:34`).

The other four cannot reveal a sidecar correctness change:

1. **`gin-routing`** — min-profile 1.000, spread 0.000 (analysis line 78):
   every profile answers correctly on every trial.
2. **`hono-middleware`** — min-profile 1.000, spread 0.000 (line 79): same
   ceiling; every agent finds `src/compose.ts`.
3. **`flask-routing`** — double fail (line 80): Axis A ceiling (spread 0.000)
   *and* closed-book mean 0.400 is not `< 0.40`
   (`docs/evals/memorization-audit/README.md`; ADR-0016.9 acceptance table) —
   half its open-book score is recall of `add_url_rule` from weights, not
   exploration.
4. **`ghx-mapengine`** — self-referential: its answer lives in the subject
   repo's own `docs/adr/0013-ghx-map-command.md`. The fixbatch run fired the
   ADR-0016.8 D2 `answer_doc_contamination` guard 7× on this one task
   (`verdict.json` `.notes`; recompute in §8), leaving a nominal "spread" that
   is an exclusion artifact with *inverted* anti-thesis ordering
   (ghx 0.722 < plain 0.900).

**Why this blocks progress.** Three compounding effects:

- **Gate sensitivity is halved.** With 4/6 tasks pinned at ceiling, a real
  sidecar proficiency win (the ADR-0016.7-class reliability fixes moved
  correctness 0.580 → 0.908, per AGENTS.md) has nowhere left to register as
  *task-level* separation; run means move only via two cells.
- **Regression sentinels are scarce.** Only `openai-node-streaming` shows a
  negative sidecar−ghx delta today. A future sidecar regression could absorb
  into run means before any single task flags it.
- **H2 discipline erodes.** `flask-routing` already fails the pre-registered
  acceptance bar ("closed-book gap (H2) becomes the task-acceptance test for
  any refreshed corpus", `TRUST.md:35`; bar = closed-book mean < 0.40 AND gap
  ≥ 0.30, ADR-0016.9 + `memorization-audit/README.md`). Every additional full
  run that cites it spends tokens on a partially-memorized measurement.

This refresh is workstream C7's named deliverable "corpus ceiling refresh
(H7)" (`docs/NORTH_STAR.md:330`) and shares design ground with C8's host-task
corpus, which already enforces a memorization canary at authoring time
(`docs/NORTH_STAR.md:331`).

## 2. Per-task evidence table

All numbers recomputed by the C7 analysis from committed artifacts
(`docs/evals/corpus-discrimination-2026-07-07.md`, provenance section): open-book
cells from `docs/evals/gate-run-2026-07-06-fixbatch/<task>_<profile>_<ts>.json`
with the 7 contaminated `ghx-mapengine` episodes excluded (same validity filter
as the gate verdict, ADR-0016.8 D2); closed-book from
`docs/evals/memorization-audit/episodes/*.json`. Reproduced the committed
`memorization-audit/closed-book-summary.json` field-for-field and the corpus
closed-book mean 0.067 exactly.

| Task | repo | plain | ghx | sidecar | spread (max−min) | sidecar−ghx | closed-book | Axis A (ceiling) | Axis B (exploration) | Verdict |
|---|---|---:|---:|---:|---:|---:|---:|---|---|---|
| express-router-location | expressjs/express | 0.500 | 1.000 | 0.900 | **0.500** | −0.100 | 0.000 | PASS (min 0.500) | PASS (gap 0.800) | KEEP — best discriminator; the design template |
| openai-node-streaming | openai/openai-node | 0.950 | 1.000 | 0.800 | 0.200 | **−0.200** | 0.000 | PASS (min 0.800) | PASS (gap 0.917) | KEEP — only real regression sentinel |
| ghx-mapengine | gkoreli/ghx | 0.900 | 0.722 | 0.893 | 0.178† | +0.171† | 0.000 | CONFOUNDED† | PASS (gap 0.838) | REPLACE (R4) — self-referential |
| gin-routing | gin-gonic/gin | 1.000 | 1.000 | 1.000 | **0.000** | 0.000 | 0.000 | FAIL | PASS (gap 1.000) | REPLACE (R2) |
| hono-middleware | honojs/hono | 1.000 | 1.000 | 1.000 | **0.000** | 0.000 | 0.000 | FAIL | PASS (gap 1.000) | REPLACE (R3) |
| flask-routing | pallets/flask | 1.000 | 1.000 | 1.000 | **0.000** | 0.000 | **0.400** | FAIL | **FAIL** (gap 0.600) | REPLACE (R1) — double fail |

† `ghx-mapengine`'s spread/sidecar−ghx sign are contamination-exclusion
artifacts with inverted ordering, not trustworthy discrimination (analysis
lines 82–86; the 7 exclusions are the sole source of the run's unequal-cell
data-quality warning in `verdict.json` `.notes`).

Axis definitions (analysis lines 93–108): **Axis A** = H7 profile
discrimination, measured by spread and min-profile (ceiling fail rule:
min-profile ≥ 0.90); **Axis B** = H2 exploration-bearing, measured by
closed-book mean and exploration gap (acceptance bar: closed-book < 0.40 AND
gap ≥ 0.30 — ADR-0016.9 "Future Task Authoring Rule" +
`memorization-audit/README.md`). Reference aggregates: fixbatch verdict means
plain 0.892 / ghx 0.946 / sidecar 0.930 (`verdict.json` `.aggregates`;
recomputed in §8), inside exactly the 0.89–0.97 band H7 quotes.

Fixture ground truth for each task lives at
`internal/sidecar/evals/testdata/tasks/<id>.json` (e.g. `gin-routing.json`
pins `expectedFiles: ["tree.go", "routergroup.go", "gin.go"]`,
`expectedSymbols: ["addRoute", "RouterGroup"]` — checks a shallow recall
agent can satisfy without reading insertion logic).

## 3. R1–R4 replacement designs

Design recipe (proven in-repo): the two tasks that discriminate today work
because of a **structural trap** — a delegation boundary or call path that
recall/shallow-browse gets wrong but exploration reveals
(`corpus-discrimination-2026-07-07.md` lines 185–193). The host-task corpus
README (`internal/sidecar/evals/hosttask/corpus/README.md`) codifies the other
half: non-recallable internals plus fixes that require reading *this* pinned
code, enforced by a `memorizationCanary`. A refreshed task must combine both:
**non-recallable internals (clears H2) + a structural trap that sinks the
plain/recall baseline (breaks the H7 ceiling)**.

**Honesty caveat (binding).** The exact `expectedFiles` / `expectedSymbols`
below are candidate hypotheses from worker knowledge, not verified against the
pinned repos (the C7 analysis says so explicitly at its lines 195–203). At
authoring time, before commit, each proposal must: (a) pin `repo@sha`;
(b) verify every path/symbol against that revision; (c) run the closed-book
canary (`tools:[]`, exact prompt) and confirm closed-book mean < 0.40 — the
same gate ADR-0016.9's "Future Task Authoring Rule" and the host-task canary
already enforce. What is durable below is the **discrimination mechanism**,
not the path strings.

### R1 — replace `flask-routing` (P0): Flask→Werkzeug URL-matching delegation

- **Repo:** `pallets/flask` (delegation target: `pallets/werkzeug`).
- **Question shape (single turn):** "When a request arrives, where is the URL
  actually *matched* to an endpoint — is that matching implemented in Flask
  itself, or delegated?"
- **Candidate checks (verify at authoring):** Flask builds a `MapAdapter` via
  `create_url_adapter` (`src/flask/app.py` / `src/flask/sansio/app.py`) and
  delegates matching to Werkzeug's routing (`MapAdapter.match`,
  `werkzeug/routing/map.py`). `unacceptableClaims`: any assertion that Flask
  implements route *matching* in-repo; candidate exceptions mirror express's
  ADR-0016.8 D7 pattern (`checks.unacceptableClaimExceptions`).
- **Why it clears H2:** the model demonstrably recalls `add_url_rule` (that is
  why flask scores 0.400 closed-book today — `memorization-audit/README.md`),
  but the `create_url_adapter` → Werkzeug `MapAdapter.match` delegation with
  current paths is unlikely to be recalled → closed-book should collapse
  toward 0.
- **Why it breaks the ceiling:** it is the express-style delegation trap kept
  inside the Python family. A recall/shallow agent asserts Flask matches its
  own routes (wrong); only exploration crosses the package boundary → plain
  should drop off 1.000 exactly as it does on express (plain 0.500),
  restoring profile spread.
- **Alternatives considered and rejected:** (a) keep flask-routing and only
  add harder checks — rejected: recall of `add_url_rule` is a measured fact;
  the task already landed "in no pre-registered bin" and was named "the first
  candidate for a harder variant" (`memorization-audit/README.md`). (b) Swap
  to a different Flask subsystem (signaling, blueprints) — rejected: stays on
  recall-heavy surface; delegation is the mechanism with proven discriminating
  power in this corpus. (c) Switch repo entirely — rejected: keeping
  `pallets/flask` preserves corpus family parity while fixing both axes.

### R2 — harden `gin-routing` (P1): radix-tree route-conflict detection

- **Repo:** `gin-gonic/gin` (same fixture id, rewritten turns/checks).
- **Question shape (multi-turn):** turn 1 — "When two route registrations
  would conflict in the routing tree (e.g. a `:param` wildcard and a static
  segment at the same position), how and where does gin detect and reject the
  conflict?"; turn 2 — follow-up on what happens at runtime for the
  conflicting pair and where that decision lives.
- **Candidate checks (verify at authoring):** `tree.go` `addRoute` /
  `insertChild` and the `findWildcard` path; the explicit panic on conflicting
  wildcard segments. `avoidPaths`: `_test.go`, `testdata/`.
- **Why it clears H2:** conflict-detection internals and their panic
  conditions are implementation detail, not a recallable famous API surface →
  closed-book stays ~0 (today's gin closed-book is already 0.000).
- **Why it breaks the ceiling:** today's question ("where are routes
  registered / how are params captured") resolves to naming
  `addRoute`/`RouterGroup`/`tree.go` — which is why all three profiles hit
  1.000. A conflict question forces reading actual insertion logic; a
  shallow/recall agent describes the happy path and misses the panic path →
  spread returns while keeping gin in the celebrity-routing family.
- **Alternatives considered and rejected:** (a) drop gin from the corpus —
  rejected: loses Go-framework parity; hardening keeps coverage. (b) Ask about
  route *performance* characteristics — rejected: benchmark lore is more
  recallable than code paths and would risk H2.

### R3 — harden `hono-middleware` (P1): multi-router selection & fallback

- **Repo:** `honojs/hono` (same fixture id, rewritten turns/checks).
- **Question shape (multi-turn):** turn 1 — "Hono ships several routers
  (RegExpRouter, TrieRouter, LinearRouter, SmartRouter). How does the
  framework decide which router to use at runtime, and where is the fallback
  when a router can't handle a path?"; turn 2 — trace one concrete request
  through that selection/fallback path.
- **Candidate checks (verify at authoring):** `SmartRouter`
  (`src/router/smart-router/`) tries registered routers in order and falls
  back on `UnsupportedPathError`; the `hono` vs `hono/quick` presets wire the
  router set. `avoidPaths`: `.test.ts`, `/runtime-tests/`.
- **Why it clears H2:** router-selection ordering and the fallback exception
  are non-obvious internals, not recallable from weights → closed-book ~0
  (today's hono closed-book is already 0.000).
- **Why it breaks the ceiling:** today's compose-question resolves to
  `src/compose.ts` for everyone (ceiling 1.000). A selection/fallback question
  is the "engine selected per input" shape — the interesting property
  ghx-mapengine tried to test — but on a real third-party repo with no
  self-reference. Naive agents point at one router; exploration finds the
  SmartRouter fallback → spread returns.
- **Alternatives considered and rejected:** (a) keep hono as the calibration
  tag carrier untouched — rejected: a calibration-tagged task that cannot
  discriminate still wastes 1/6 of run tokens; the judge criteria can stay
  calibration-flavored on the new turns. (b) Ask about middleware ordering
  semantics only — rejected: still answerable from `compose.ts` recall;
  selection/fallback is what forces reading multiple router implementations.

### R4 — replace `ghx-mapengine` (P2): engine-per-input selection without self-reference

- **Primary repo:** `golang-migrate/migrate` (mid-fame, non-celebrity).
- **Question shape (single turn):** "How is the database driver and the
  migration *source* selected from a URL scheme (e.g. `postgres://…`,
  `file://…`), and where is the fallback when a scheme is unregistered?"
- **Candidate checks (verify at authoring):** scheme-keyed driver registries
  with `database.Open` / `source.Open` scheme dispatch (`database/driver.go`,
  `source/driver.go`); the "unknown driver" error path. `avoidPaths`:
  `_test.go`. No `contaminationPaths` needed at all.
- **Alternative repo (pre-register one, not both):** `alecthomas/chroma` —
  "how is the lexer selected for a filename, and where is the analyser
  fallback when no lexer matches by name?" (`lexers` registry,
  `Match`/`Analyse` fallback).
- **Why it clears H2:** scheme/registry dispatch internals are not recallable
  → closed-book ~0; neither candidate repo is self-referential, so the
  contamination surface disappears entirely — no exclusions, no unequal cells,
  no anti-thesis artifacts (the exact failure chain of fixbatch's
  ghx-mapengine cell).
- **Why it breaks the ceiling:** preserves the genuinely interesting structural
  property ghx-mapengine reached for (per-input engine/backend selection with
  a fallback), which requires reading selection code rather than guessing;
  expect real spread from the fallback-reasoning requirement, cleanly measured
  for the first time.
- **Alternatives considered and rejected:** (a) keep ghx-mapengine with a
  wider `contaminationPaths` list — rejected: the task is self-referential by
  construction; every future run re-risks the same 7-exclusion failure and
  unequal cells. (b) Point it at another gkoreli/ghx subsystem — rejected:
  same self-reference defect, different paths. (c) chroma vs golang-migrate —
  both fit; pick at authoring time by whichever verifies cleanly against its
  pinned sha and passes the closed-book canary first.

## 4. Pre-registration requirements

The measurement-freeze rule binds this work: "the measurement stack is frozen
during a run. Any scorer/detector change mid-run must be pre-registered in an
ADR before rescoring" (AGENTS.md, "Visibility and Truthfulness"; precedent:
compliance-detector fix `afd6e99`, ADR-0016.8). A corpus swap changes what the
gates measure, so it must land as a pre-registered eval-corpus ADR **before**
the next full run — exactly as the C7 analysis's bottom line requires
(`corpus-discrimination-2026-07-07.md` lines 298–302).

1. **Per-task closed-book canary before commit (hard gate).** For each of
   R1–R4: pin `repo@sha`, verify checks against that revision, then run the
   ADR-0016.9 probe harness (`tools:[]`, `allowedTools:[]`, exact task prompt,
   n ≥ 3 for authoring smoke; harness: `internal/sidecar/evals/closedbook.go`
   `RunClosedBookEpisode`, proven live by `TestClosedBookProbe` per
   `memorization-audit/README.md`). Acceptance bar (ADR-0016.9 "Future Task
   Authoring Rule"): mean closed-book correctness < 0.40, max < 1.00 unless
   explicitly labeled a hallucination/freshness trap, and no required check
   satisfiable by generic framework lore alone. A candidate failing the canary
   does not enter the corpus; revise or fall back to the alternative repo.
2. **Gates unchanged.** G1–G5 thresholds, reducers (`gate_reducers.go`),
   scorers (`rewards.go`), and the contamination guard keep their current
   semantics; only the six fixtures under
   `internal/sidecar/evals/testdata/tasks/` change content. `Task.Validate`
   (episode.go:63) already enforces discoverable-check and blank-entry rules,
   so new fixtures need no scorer edits. No committed verdict is rescored;
   fixbatch stays citable as measured.
3. **D-G-style run plan** (mirroring ADR-0019.2 D4's discipline of separate,
   pre-registered gates with labeled verdicts — applied here to the corpus
   refresh itself):
   - Land the ADR + verified fixtures + canary artifacts first
     (`docs/evals/memorization-audit/` gains a refresh section or sibling dir).
   - Smoke each new task on one profile before any full run (measurement
     ladder, AGENTS.md: feature go/no-go rides smokes).
   - Full pre-registered run in the background (AGENTS.md: full runs never
     block engineering), same tasks × profiles × trials matrix recorded via
     the manifest rounds mechanism (ADR-0016.8 D5).
   - Success criterion, pre-registered now: all six tasks show nonzero
     profile spread AND every task's closed-book mean < 0.40 with gap ≥ 0.30
     — i.e. the corpus moves from 2/6 to 6/6 on both axes. If any refreshed
     task re-ceilings (min-profile ≥ 0.90 across profiles), it is named in the
     verdict and returns to the backlog rather than being silently kept.
   - Verdict self-labels PRELIMINARY until an independent cross-family
     recomputation lands (TRUST standing rule #1).

## 5. Risks

- **Fixture authoring cost (highest).** Each R1–R4 needs: sha-pinned
  verification of every expected file/symbol, judge-criteria authoring,
  `Task.Validate`-clean fixtures, plus canary + smoke episodes. Rough order:
  4 tasks × (verification reads + n≥3 closed-book trials + smoke trials) —
  real tokens and real wall-clock. Mitigation: the measurement ladder keeps
  this off the critical path (smokes gate features; full runs run in the
  background), and ADR-0025's baseline-reuse economics bound full-run cost.
- **Contamination discipline.** R4's whole point is removing self-reference;
  if a future candidate ever points at `gkoreli/ghx` again, the
  ADR-0016.8 D2 guard (`contaminationPaths`, `answer_doc_contamination`) must
  be registered in the fixture from day one — not retrofitted after
  exclusions, which is precisely what corrupted the ghx-mapengine cells.
- **Recall drift over time.** Celebrity repos (flask/gin/hono) stay exposed to
  training-data creep; a task that passes its canary today can decay. The
  closed-book probe is cheap enough to re-run per corpus change; consider a
  standing re-canary before each citable full run.
- **Trap brittleness / false zeros.** Delegation traps lean on
  `unacceptableClaims` substring zeroing; ADR-0016.8 D7 showed honest
  historical mentions can zero correctness. R1 should ship its exceptions list
  pre-registered (express's pattern), and any growing exception list is the
  pre-registered signal to simplify the check instead (D7's own rule).
- **Projection honesty.** "2/6 → 6/6" is a projection, not a result; it is
  falsifiable at the first post-refresh run and must never be quoted as
  measured (AGENTS.md: expectations are never citable results).

## 6. Open questions

1. **R4 repo pick:** `golang-migrate/migrate` vs `alecthomas/chroma` — decide
   by verification outcome at authoring time (§3 R4c), or verify both and
   keep one as a bench replacement task?
2. **Retire vs harden for gin/hono:** R2/R3 rewrite in place (keeps fixture
   ids and family parity). Alternative: retire one celebrity task and admit a
   mid-fame repo outright (host-task README's "non-celebrity" recipe).
   Deciding factor: how much corpus continuity matters for cross-run
   comparability of the two surviving cells' history.
3. **Canary cadence:** once per authoring (ADR-0016.9 minimum) or re-run
   before every citable full run? The latter bounds recall drift but costs
   ~30 episodes per run; ADR-0025 economics may make it near-free via
   baseline reuse.
4. **TRUST bookkeeping:** H7's row says Fable updates `docs/evals/TRUST.md`
   at merge (`corpus-discrimination-2026-07-07.md` lines 11–13); confirm the
   same disposition path applies when the refresh ADR lands and H7 moves from
   "Quantified" toward closed.
5. **Interaction with C8 host-task corpus:** the host-task corpus already has
   a memorization canary; should the recon-corpus canary results feed the
   same trust rows, or stay separate families (ADR-0019.2 D4's separation
   logic suggests separate)?

## 7. Proposed ADR outline — and thread choice

**Thread choice: extend the `0016.x` family as ADR-0016.13, thread
`sidecar-agentic-eval`.** Justification against existing threads in
`docs/adr/`:

- The 0016 family *is* the sidecar-agentic-eval measurement family
  (`0016` … `0016.12-repo-named-closed-book-variant.md`); corpus composition
  decisions already live there — ADR-0016.9 owns the closed-book acceptance
  bar this refresh must satisfy, ADR-0016.8 owns the contamination guard the
  refresh is designed around, ADR-0016.2 owns discoverable-check validity
  that `Task.Validate` enforces. A corpus-refresh decision that modifies
  those three decisions' scope belongs beside them (AGENTS.md: "Thread
  related decisions with decimal numbering when the work belongs to an
  existing decision family").
- The `0019.x` thread (`sidecar-adoption`) is the wrong home: it owns the
  *discovery* task family (ADR-0019.2), which is deliberately scored
  separately and never mixed with repo-scoped G1–G5. This refresh changes the
  repo-scoped corpus itself.
- `0032.x` (host-task evals) is also not the home: C8's corpus is a separate
  two-agent family with its own gates (NORTH_STAR C8 row); shared canary
  *design* is cited, not shared ownership.
- Numbering: `0016.10`–`0016.12` exist, so the next free decimal is
  **`0016.13`**. `status: proposed` at first commit, per the family's
  pre-registration pattern (ADR-0016.9 stayed `proposed` until its run
  landed).

**Proposed ADR-0016.13 outline** ("Eval Corpus Discrimination Refresh
R1–R4"):

1. **Status** — proposed; pre-registration binding before the next full run.
2. **Context** — H7 quantified (2/6 discriminating; §2 table); flask-routing
   fails the ADR-0016.9 bar; ghx-mapengine structurally contaminated.
3. **Decisions:**
   - D1: adopt R1 (flask→Werkzeug delegation trap) replacing flask-routing.
   - D2: adopt R2 (gin conflict detection) hardening gin-routing.
   - D3: adopt R3 (hono SmartRouter selection/fallback) hardening
     hono-middleware.
   - D4: adopt R4 (non-self-referential engine-selection task;
     golang-migrate vs chroma pick) replacing ghx-mapengine.
   - D5: acceptance contract — per-task closed-book canary < 0.40 (and max
     < 1.00 unless labeled trap) before any fixture commit; gates G1–G5 and
     scorers byte-unchanged; express + openai tasks untouched.
   - D6: run plan — smokes per task, then one full pre-registered run;
     success = 6/6 tasks with nonzero spread and all tasks
     exploration-bearing; re-ceiling tasks named and returned to backlog.
4. **Considered and rejected** — keep-as-is (H7 stays open), judge-only fix
   (gates stay deterministic per ADR-0023.1), corpus expansion instead of
   replacement (keeps paying 4 dead cells per run).
5. **Verification plan** — fixture `Task.Validate` unit tests, canary
   artifacts committed under `docs/evals/`, golden recompute of §2's table
   from committed episodes.
6. **Cross-references** — ADR-0016.1/0016.2/0016.8/0016.9, ADR-0019.2
   (canary precedent), ADR-0023.1 (judge layer), TRUST H2/H7,
   `corpus-discrimination-2026-07-07.md` (the analysis this executes).

## 8. Evidence appendix

All numbers below were recomputed from committed artifacts at HEAD `719cb29`
and cross-checked against the sources cited in §2:

**Fixbatch open-book aggregates** (`docs/evals/gate-run-2026-07-06-fixbatch/
verdict.md:7-9`):

```
| plain        | 30 | 0.892 | 0.917 | 0.922 | 0.000 | 1.000 | 73309 | 1.00 | 0.000 |
| ghx          | 31 | 0.946 | 0.935 | 0.943 | 0.000 | 1.000 | 82864 | 1.00 | 0.483 |
| ghx-sidecar  | 32 | 0.930 | 0.887 | 0.887 | 0.894 | 1.000 |  4985 | 1.00 | 0.427 |
```

G1 PASS line (`verdict.md:15`): "sidecar 0.930 vs ghx 0.946 (rel floor 0.852,
abs floor 0.60)". Contamination guard fired **7×** in verdict.md
(`grep -c CONTAMINATION` = 7).

**ghx-mapengine cells after exclusions**
(`docs/evals/corpus-discrimination-2026-07-07.md:77,84`):

```
| ghx-mapengine | gkoreli/ghx | 0.900 | 0.722 | 0.893 | 0.722 (0.278) | 0.178† | +0.171† | ...
"0.722 only after excluding 1 contaminated ghx episode (n=6), plain 0.900 after"
```

**Closed-book probe** (`docs/evals/memorization-audit/README.md:45,104`;
`TRUST.md:29`):

```
| flask-routing | 0.400 | ... | **0.600** | (none — see below) |
Closed-book corpus mean correctness: 0.067 (30 trials)
TRUST H2: closed-book corpus mean 0.067 vs 0.926 open-book; flask-routing
mean 0.40 — the corpus's only memorization-dominated task
```

**Per-task spread table** (`corpus-discrimination-2026-07-07.md:75-77`):
express-router-location spread **0.500** (plain fails the delegation trap),
openai-node-streaming sidecar−ghx **−0.200**; gin/hono/flask min-profile
1.000 / spread 0.000.

Commands run during authoring: recompute of per-task means from committed
episode JSONs under `docs/evals/gate-run-2026-07-06-fixbatch/`, closed-book
means from `docs/evals/memorization-audit/`, and fixture registration checks
under `internal/sidecar/evals/` (see §2 citations for file:line anchors).

