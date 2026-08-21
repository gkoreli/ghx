---
title: "Proof ladder — from self-referential evals to externally-valid outcome-graded evidence"
date: "2026-08-21"
status: "research"
thread: "sidecar-agentic-eval"
author: "delegated research worker (ox-alpha)"
scope: "read-only research artifact; proposes, does not implement; maintained under docs/research/"
builds-on: "docs/NORTH_STAR.md C8/H-series, AGENTS.md visibility-and-truthfulness tenet, docs/evals/TRUST.md"
---

# Proof ladder: from self-referential evals to externally-valid, outcome-graded evidence

**Question:** how does the SAFE/ghx eval stack graduate from today's
self-referential, self-authored, deterministic-gate benchmarks to
externally-valid evidence graded by real-world outcomes — without breaking
the truthfulness tenets (AGENTS.md "Visibility and Truthfulness"; "read M4
as a floor, never quote it as a ceiling")?

`docs/research/h7-corpus-discrimination.md`). Proposes; registers nothing.
No gates, thresholds, or corpus are decided here — each rung's registered
form belongs in a follow-up ADR. **No existing file was modified; nothing
committed.**

**Repo state at authoring:** branch `mainline`, HEAD `f951403` ("docs:
accept ADRs 0037–0039 …"), clean tree except this artifact.

**Trigger:** Goga (2026-08-21): "we're comparing ghx to ghx, not learning
anything meaningful; the real test is a heavyweight outcome-graded benchmark
(e.g. SWE-bench-style) where the agent must explore/research via ghx first."

**Sources grounded in:** `AGENTS.md` (evidence contract + truthfulness),
`docs/NORTH_STAR.md` (workstream C, C8), `docs/adr/0016*.md` (eval kernel),
`docs/adr/0023.1` (judge), `docs/adr/0025*.md` (run economics),
`docs/adr/0032`/`0032.1` (host-task evals), `docs/evals/TRUST.md`,
the four committed `gate-run-*` verdicts, `docs/research/h7-corpus-discrimination.md`
(H7 ceiling quantification), `docs/research/m7-tier2-escalation.md` (tier-2
remote/local boundary — the differentiator analysis), and the code under
`internal/sidecar/evals/` (`hosttask/`, `host_episode.go`, `profiles.go`,
`gates.go`). Web facts cited inline with URLs.

---

## 1. What today's run levels can and cannot claim

The stack already has a measurement ladder for *rigor* (V0 mock → V1 smoke →
V2 spot → V3 full gate run, ADR-0016.3 "Run Economics Protocol"). It has no
ladder for *external validity* — every live rung above V0 measures the same
self-referential universe:

| Run level | Artifact | What it can claim | What it cannot claim |
|---|---|---|---|
| V0 mock | `internal/sidecar/evals/*_test.go` (hermetic) | The framework computes what it says (capture, compliance, identity, reward math) | Anything about agents |
| V1/V2 smoke & spot | `docs/evals/smoke-2026-07-05-adr21/`, `spot-instrumented-2026-07-06/` | Pipeline works end-to-end on real adapters; per-episode cost signal (§5) | Any comparative claim — PRELIMINARY by sample size |
| V3 recon gate run (G1–G6) | `gate-run-2026-07-05-confirmatory/verdict.md` (M4: sidecar correctness 0.908 vs ghx 0.931 at 25× compression); `gate-run-2026-07-06-fixbatch/verdict.md` (0.930 vs 0.946 at 16.6×) | *Inside the six-task corpus*: the sidecar boundary preserves deterministic fact-recall correctness while cutting main-agent context 16–25× | Quality (substring gates measure pre-registered fact recall — AGENTS.md tenet; TRUST H1), external validity (below), superiority of exploration quality |
| Discovery spot/gate (D-G1…D-G6) | `spot-instrumented-2026-07-06/discovery/discovery-verdict.md` — all quality gates FAIL, honestly labeled PRELIMINARY/DISCOVERY NOT SUPPORTED at n=1/task | The only arm that touches the open GitHub world (repo-optional scope, ADR-0019.1) — but at n=1 nothing is citable | Any discovery-quality claim yet; also still self-scored against hand-authored target sets |
| Judge layer | ADR-0023.1; sweep `judge-sweep-2026-07-06/` | Offline machinery shipped, cross-family clients shipped; disagreement machinery works (19.3% > 15% bound → INVESTIGATE, 27/27 confirmed judge-good on cross-family re-score) | **Zero citable scores** until founder gold-set labeling lands κ ≥ 0.6 (TRUST H1 — the sole remaining unlock) |
| Host-task (C8, GH1/GH2) | ADR-0032/0032.1; S1–S4 built (`hosttask/` provisioner+grader+corpus of 6 fixtures, `host_episode.go` arms) | The first *outcome-graded* class: fail-to-pass × pass-to-pass in pinned containers, memorization canary enforced at load (`Fixture.Validate`) | Not yet run (docker grader + ≤5-episode smoke owed); still **hand-authored** tasks and an **arm-A-only** baseline that is the host exploring alone |

Why the universe is self-referential — four compounding confounds, each with
a ledger row or guard:

1. **One evidence engine across all profiles.** plain/ghx/ghx-sidecar differ
   only in how reconnaissance reaches the agent; every profile is scored by
   ghx-authored checks over a corpus whose ground truth was authored by the
   same team that sells the result. `ghx-mapengine` demonstrated the failure
   mode concretely: self-referential task, contamination guard fired 7×,
   cells corrupted with inverted ordering (`gate-run-2026-07-06-fixbatch/verdict.md`;
   H7 analysis §1).
2. **Ceiling/discrimination collapse.** Only 2 of 6 recon tasks can detect
   any sidecar change (spread 0.000 on three tasks; flask-routing fails the
   closed-book bar at 0.400) — `docs/research/h7-corpus-discrimination.md`
   and TRUST H7. R1–R4 fixtures landed (ADR-0016.13) but live canaries are
   pending before any citable claim.
3. **Memorization surface.** Famous-repo tasks score ~0.89–0.95 across all
   profiles partly from weights; the closed-book probe bounded it (corpus
   mean 0.067 vs 0.926 open-book; flask-routing 0.400 = recall-dominated)
   — TRUST H2, `memorization-audit/`. The SWE-Bench Illusion evidence
   (76% file-path recall from issue text alone on benchmark repos;
   https://arxiv.org/abs/2506.12286, already cited by ADR-0032) applies to
   us the moment we pin celebrity repos.
4. **No generic-subagent baseline.** The strongest honest control — a
   frontier model with plain clone/grep/read subagents, no ghx anywhere —
   exists nowhere in any arm. Arm A of C8 is the host exploring *by itself*;
   the industry's actual alternative to a sidecar is *delegation to another
   general agent*, which is exactly the shape the Anthropic multi-agent
   research system post normalizes (ADR-0032 prior-art table). Until that
   arm exists, every verdict compares ghx to ghx — Goga's point.

**Consequently, the citable thesis claim today is exactly this narrow
sentence:** *within a self-authored six-task recon corpus, scored by
deterministic substring gates, a first-cut sidecar persona preserved
correctness within a pre-registered floor while compressing main-agent
context ~16–25×.* Every wider reading (better exploration, better value in
real engineering, advantage over alternatives that don't use ghx) is an
expectation, and expectations are never citable results (AGENTS.md).

---

## 2. The missing arm: generic-subagent baseline (frontier model + plain tools)

**What it is:** in the C8 host-task episodes, a third arm where the host's
exploration need is satisfied by delegating to a *generic subagent* — same
frontier model class as the host, given a fresh workspace/checkout (or `gh`)
and only stock tools (clone, grep, read, shell). No ghx binary, no ghx skill,
no recon MCP tool, no sidecar persona. It models "what a strong main agent
would do anyway" — spawn a cheap subagent to go read the upstream repo and
come back with a summary — which is the real-world competitor to the
sidecar boundary, and the strongest form of the control the C8 design
already worried about ("revisit only if the control's exploration is so
weak the comparison looks like a strawman", ADR-0032 D-notes #1).

### Where it slots into the existing structure

The plumbing was designed for exactly this kind of extension:

- Arms are a two-value enum with per-arm profile labels:
  `HostArmControl` / `HostArmSidecar` → `host-control` / `host-sidecar`
  (`internal/sidecar/evals/host_episode.go:24–54`). A third value
  (`HostArmSubagent` → label `host-subagent`) slots into the same
  `HostTrial` struct; the arm→profile mapping and prompt-contract selection
  are the only switch points.
- `runLiveAgentEpisode` (extracted in S3, ADR-0032.1) is parameterized by
  client/write-policy, advertised fs capabilities, MCP servers, and prompts.
  Arm C differs from arm A only in its prompt contract ("delegate outside-
  checkout reconnaissance to a fresh general-purpose subagent process; fold
  its findings into your context") plus whatever harness mechanism spawns
  the subagent. Two implementation shapes:
  - **(a) Prompt-level delegation via the existing terminal policy:** the
    host runs e.g. `claude -p "<research question>"` (or `codex exec`) as a
    shell command inside its sandbox. Zero new client policy; the subagent
    is just a command. Attribution needs one classifier rule addition:
    recognize the delegate-command prefix as *exploration-delegated*
    (currently R4 would class `claude`/`codex` prefixes as engineering —
    wrong for dead-weight accounting).
  - **(b) Harness-spawned subagent session:** the eval runner spawns a
    second ACP episode per exploration need and pipes question→report.
    Cleaner attribution, more new code, and it starts inventing an
    orchestration protocol — which the project explicitly refuses to do
    (NORTH_STAR "we never invent a sidecar protocol").
  - Lean: **(a)**. It is also the honest baseline: it is precisely what a
    Claude Code / Codex user gets today for free.
- Token attribution: `hosttask.Classifier` rules R1–R8 are declarative rows
  (ADR-0032.1 S2 note); add one rule for the delegate prefix (class
  `exploration-delegated`, counted as exploration in `DeadWeightFraction`)
  plus a rule treating the subagent's own token spend (not visible through
  arm (a)'s transcript!) as a measured unknown — see honesty note below.
- Anomaly rows: reuse `host_external_exploration` semantics inverted — on
  arm C, *direct* external exploration by the host (un-delegated `gh`/curl/
  clone-and-read by the host itself) is the flagged deviation, symmetric to
  how arm B flags un-delegated exploration today.
- Gates: GH1-success non-inferiority extends naturally to three paired
  comparisons (B≥A−δ stays the registered claim; B vs C becomes a second
  non-inferiority or, if data shows direction, a separately pre-registered
  superiority follow-up — never a post-hoc upgrade, ADR-0032's power
  section). GH1-efficiency should be expected to *weaken or flip* against C
  on raw tokens: multi-agent spends more total tokens for better outcomes
  (Anthropic multi-agent post; ADR-0032 prior art) — the honest framing is
  dead-weight fraction in the *host's* context and whole-workflow SPT
  reported side-by-side, never a blended win.

### Honesty notes (pre-commit these)

- **Hidden-token problem:** under shape (a), the subagent's internal spend
  is invisible to the episode unless the wrapper surfaces usage. Options:
  wrap the delegate command in a script that appends its own `gen_ai.usage`
  to a sidecar file next to the episode (small shim, keeps attribution
  recomputable — visibility tenet requires this before any efficiency claim
  names arm C).
- **Same-family judge risk:** arm-C reports and arm-B reports come from the
  same model family; the arm-blind bundle transform (already an open
  problem for A/B, ADR-0032 §machinery) must cover C too.
- **Contamination symmetry:** the generic subagent has web/clone access to
  the same pinned world; if it reads upstream fix commits, it "wins" by
  lookup. Decide and register up front whether upstream git history at the
  pin is in-bounds for all arms (it is part of the world both products
  serve) or out-of-bounds via container network=none + remote-API-only
  exploration (matches ghx's remote-first tenet).

### Rough implementation size

Prompt contract + arm enum + classifier row + anomaly row + delegate-wrapper
usage shim + tests: **~1 engineer-day plus a ≤5-episode smoke**, riding the
S1–S4 build that already landed. No changes to scoring of existing arms;
measurement-stack freeze respected by landing it as a pre-registered ADR
(natural home: ADR-0032.2 decimal in the `sidecar-agentic-eval` thread — note
ADR-0032.2 currently covers the Locations-flow unification, TRUST H9, so
this would be 0032.3 or a sibling).

---

## 3. Heavyweight options: externally-valid, outcome-graded tiers

These graduate the *task source* from self-authored to world-authored, and
the *grade* from substring recall to executed outcomes.

### 3a. SWE-bench-Live slice (recommended first heavyweight tier)

SWE-bench-Live (Microsoft, NeurIPS 2025 D&B): 1,319 tasks from issues filed
after 2024-01-01 across 93 repos, Docker-pinned environments, fully automated
RepoLaunch curation pipeline, monthly refresh cadence — built specifically as
contamination-resistant evaluation (https://github.com/microsoft/SWE-bench-Live,
https://swe-bench-live.github.io/, https://arxiv.org/abs/2505.23419).
ADR-0032 rejected it *as-is* because its tasks select single-repo resolution,
not external-exploration subneeds — that rejection stands for the combined-
objective claim, but a **slice** serves a different, narrower purpose:
external validity of the *exploration answer*, graded by downstream outcome.

Design sketch (to be pre-registered, not decided here):

1. Sample N tasks (first tranche ~20–30) whose issue touches a repo where
   the correct fix plausibly requires understanding *other* repos
   (dependency APIs, protocol details, upstream behavior) — the same filter
   ADR-0032's task family #1/#2 uses, applied at selection time.
2. Run the standard three recon profiles' *question-answering half* first
   ("where/how does X behave?"), score with existing deterministic +
   judge machinery — then feed each profile's synthesized understanding
   into an identical, blind implementer host that resolves the issue.
   Grade = F2P/P2P in the Live-provided container (their harness, stolen
   per the leverage tenet since we did not author these instances).
3. This yields the first **world-authored, outcome-graded** comparison:
   does sidecar-mediated exploration produce understanding that resolves
   real post-cutoff issues at least as well as self-exploration?
4. Contamination discipline: recency is inherited from Live (post-cutoff
   ingestion + refresh); additionally re-run the closed-book canary pattern
   (ADR-0016.9 harness) per selected task — drop tasks the subject model
   solves from the issue text alone (SWE-Bench Illusion filter, cheap and
   deterministic).
5. Repo-local mismatch analysis vs ghx's differentiator: expect and
   *pre-register the hypothesis* that a fraction of Live tasks are
   single-repo-internal (no genuine external subneed) and show no
   arm differences — report the stratified result (subneed vs no-subneed)
   rather than a headline mean. The differentiator hypothesis: ghx's edge
   concentrates in the remote-first/discovery stratum (repo-optional
   questions, cross-repo API tracking — ADR-0019.1; tier-2 escalation when
   structure demands local mapping, `m7-tier2-escalation.md`). Tasks where
   everything needed lives inside the checkout are ghx's *worst* case by
   construction, and saying so in advance is the anti-strawman discipline
   the tenets demand.

### 3b. Post-cutoff harvested issues (the freshness tier)

Steal Live's pipeline pattern at smaller scale: harvest issues filed after a
declared cutoff date (per subject model) from mid-size repos ghx already
serves, verify F2P/P2P executability, run the memorization canary, and
author the *exploration sub-question ground truth* the way the host-task
fixtures do (`explorationSubQuestions.verifiedAt`, S4 pattern). This is
"R-task authoring at industrial speed" — heavier than 3a on authoring,
lighter on adoption; justified only if Live's Go/TS mix underserves the
repos our users actually explore.

### 3c. Research-then-implement task designs (the differentiator-native tier)

The claim nobody else measures: *explore-then-build* — the implementing
agent's success depends materially on reconnaissance of the outside world
(upstream API moved, spec lives elsewhere, reference implementation in a
sibling repo). This is ADR-0032's corpus intent with world-authored
sources: take real migration commits (post-cutoff) between two pinned
versions of coupled repos, withhold the fix, grade F2P/P2P. Highest
fidelity to the north-star claim ("both, not one"), highest authoring cost;
the natural continuation after 3a demonstrates the pattern.

Ordering rationale: 3a buys external validity cheapest and fastest
(world-authored instances, their containers, their refresh);
3c is where the unique claim can actually pass or fail; 3b fills gaps only
if 3a's repo mix proves too narrow. All three inherit the C8 grader,
attribution, compliance detector, and frozen-identity manifest unchanged —
they change the *fixture source*, not the measurement kernel.

---

## 4. Cost / sample-size estimates per run level

Grounded in ADR-0025 economics and the only committed per-episode usage
data (spot-instrumented run, H5 machinery, real provider usage on 5
episodes — recomputed from `docs/evals/spot-instrumented-2026-07-06/*/​*.json`
`turns[].rawSDK.usage`):

- Recon single-question episodes, sonnet-class: sidecar ≈ $0.15–0.22 /
  53–81 s (3,052–3,532 output tokens; input≈15 because context rides the
  session); direct ghx $0.45 / 80 s (156,894 main-agent chars); plain
  $0.39 / 80 s (98,034 chars). These five episodes are the entire committed
  real-cost sample — treat as order-of-magnitude only.
- Full recon gate run: 90 episodes ≈ 62 min wall at parallelism 3, ~$15–25
  API-equivalent extrapolated (fixbatch README gives wall time; dollars
  extrapolated from the spot rates above — not previously committed, so
  labeled estimate).
- Host-task registered run (ADR-0032): 60 episodes ≈ $120–360,
  1–2 working days sequential, ~0.5 day at 3-way parallelism (ADR-0032 §cost;
  estimates by design, first run measures them).
- Adding arm C to C8: +20 episodes (6 tasks × 1 extra arm × ~3–5 trials,
  stopping-dependent) ≈ **+$40–120**, plus subagent-internal spend which
  the §2 wrapper makes visible before any efficiency claim.
- SWE-bench-Live tranche (~25 tasks × 3 recon profiles × 3 trials recon
  half + ~25 implementer resolutions × 2 arms × 2 trials): roughly 275
  episodes ≈ **$500–1,500** and 2–4 days at bounded parallelism — dominated
  by implementer episodes (10–30 min each, ADR-0032's control-arm range).
  Container builds amortize per task via Live's published images.
- Statistical sizing (inherited, not re-derived): binary success
  superiority needs ~90–400 episodes/arm — unaffordable; hence
  non-inferiority δ=0.10 on paired task means + superiority on continuous
  economics (dead-weight fraction, SPT) at 5 trials/cell, per ADR-0032's
  power section and ADR-0025.2's at_least(n)/FRAGILE machinery. Sequential
  stopping (D1) bounds worst-case spend; futility stops convert doomed
  tranches into cheap negatives, which are kept.

Every number in this section is either recomputable from committed
artifacts (the five spot episodes, wall clocks) or an explicit projection —
labeled as such per the "expectations are never citable" rule.

## 5. The proposed proof ladder (rungs, arms, gates, licensed claims)

Each rung keeps the previous rung's claims valid and adds one axis of
validity. Nothing below is registered; each rung needs its pre-registering
ADR before any run (freeze rule).

| Rung | Name (proposed) | Task source | Grade | Arms | Est. cost/run | Licensed claim (survives the tenets) |
|---|---|---|---|---|---|---|
| L0 | Mock/hermetic | fixtures | unit tests | mock | $0 | Framework computes what it says |
| L1 | Recon smokes/spots (exists) | self-authored | deterministic G-gates | plain/ghx/sidecar | <$5 | Pipeline integrity; per-episode cost signal |
| L2 | Full recon gate run (exists) | self-authored, refreshed corpus (ADR-0016.13) | G1–G6 (+judge once κ lands) | 3 profiles | ~$15–25, ~1 h | The narrow M4 sentence (§1), now on a corpus whose discrimination is measured rather than assumed |
| L3 | Host-task + **arm C** (build C8 run + §2 arm) | self-authored, canaried | GH1-success NI δ=0.10, GH1-efficiency sup., GH2-recon ≥0.70 | control/subagent/sidecar | ~$160–480, 1–2 d | Deferred exploration preserves *engineering outcomes* (non-inferiority, never quoted as a success win) while cutting host-context dead weight vs **both** self-exploration and generic-subagent delegation; workflow SPT reported, never blended |
| L4 | SWE-bench-Live tranche (§3a) | world-authored, post-cutoff, refreshed | F2P×P2P containers + recon-half gates; stratified by external-subneed | same as L3 | ~$0.5–1.5k, 2–4 d | On issues neither authored nor seen by us, understanding produced via the sidecar resolves real bugs at least as often (NI) at lower host-context cost; effect concentrated in the external-subneed stratum (pre-registered differentiator hypothesis) |
| L5 | Research-then-implement corpus (§3c) | harvested migration pairs, post-cutoff | F2P×P2P + sub-question ground truth | same as L3 | ~$1–2k (authoring-heavy) | The full north-star conjunction on tasks *selected for* the phenomenon: exploration deferred ⇒ engineering objective at least as good AND exploration answers better AND sessions live longer |

Ladder-wide truthfulness guards (each maps to an existing tenet):
verdict templates render success vs efficiency conclusions in separate
words (never quote an efficiency win as a success win); every under-sampled
or judge-uncalibrated cell self-labels PRELIMINARY and cannot feed
go/no-go; negative results commit and stay; the measurement stack freezes
per run; floors are floors — L2's compression sentence may never be
promoted to "best possible exploration" prose; and each rung's first run
is Goga-triggered (full runs are event-driven, never autonomous).

**Sequencing:** L3-arm-C is the critical unlock (cheap, closes the
"ghx vs ghx" hole even before any heavyweight tier); L4 reuses L3's
machinery with world-authored fixtures; L5 is where the unique claim wins
or dies, funded only after L4 shows the stratified signal.

---

## 6. Uncertainty

- The five-episode cost sample is tiny and sonnet-class only; API-dollar
  extrapolations across rungs could be off 2–3× (subscription rails make
  much of this rate-limit pressure, not dollars — ADR-0032 trap 2).
- Arm-C delegation shape (a) vs (b) is a lean, not a decision; if adapter
  CLI rails cannot surface subagent usage, the hidden-token problem blocks
  efficiency claims for arm C entirely (success claims survive).
- SWE-bench-Live selection for "external-exploration subneed" is a manual
  judgment call today; until operationalized, the L4 stratification could
  collapse (most tasks single-repo), which would itself be a finding about
  where the sidecar does *not* help — worth having either way, but the L4
  cost guess assumes ~half the tranche survives the filter.
- Judge calibration (κ) remains the sole unlock for any trajectory-quality
  language anywhere on the ladder (TRUST H1); without it, L3–L5 claims are
  confined to outcome grades + deterministic sub-question checks.
- R1–R4 refreshed corpus canaries are pending (C7/H7); L2's licensed claim
  inherits that caveat until they run live.

## 7. Suggested next reads

- `docs/adr/0025.1` / `0025.2` — identity hashes and at_least(n) reducers
  that any new gate forms must reuse.
- `docs/evals/judge-goldset/PROTOCOL.md` — the labeling session that
  unlocks κ, the cheapest single action that widens every rung's claims.
- `docs/adr/0038` — expensive-backend guardrail; L3–L5 budgets must run
  inside it (override discipline noted in `docs/evals/README.md`).
- `https://github.com/microsoft/SWE-bench-Live` (dataset browser + RepoLaunch
  pipeline) and `https://arxiv.org/abs/2506.12286` (contamination numbers)
  before writing the L4 pre-registration ADR.
- `internal/sidecar/evals/hosttask/doc.go` + `corpus/README.md` — fixture
  schema and provenance verification recipe that L4/L5 fixtures must satisfy.

## 8. Commands run appendix

```
ls docs/adr docs/evals docs/research internal/sidecar/evals
wc -l docs/evals/TRUST.md docs/adr/0032*.md docs/research/*.md
python3 (recompute turns[].rawSDK.usage sums over
         docs/evals/spot-instrumented-2026-07-06/{episodes,discovery}/*.json
         → 5 episodes with usage: sidecar $0.147/$0.222/$0.162,
         ghx $0.446, plain $0.386; durations 32–80 s)
grep -n "g1Pass\|G1\b" internal/sidecar/evals/gates.go
grep -n "Profile\|func " internal/sidecar/evals/profiles.go
grep -n "func \|Arm\|arm" internal/sidecar/evals/host_episode.go
ls internal/sidecar/evals/hosttask internal/sidecar/evals/hosttask/corpus
git log --oneline -3   # HEAD f951403, clean tree except this artifact
```

Web sources consulted: https://swe-bench-live.github.io/ ,
https://github.com/microsoft/SWE-bench-Live , https://arxiv.org/abs/2505.23419 ,
https://arxiv.org/abs/2506.12286 .
