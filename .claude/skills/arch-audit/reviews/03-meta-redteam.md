---
title: "arch-audit review 03 — Meta Red-Team / Pre-Mortem (kill-the-artifact)"
date: "2026-07-07"
round: "skill-forge Round 2"
reviewer: "Meta red-team (Klein prospective-hindsight premortem)"
target: ".claude/skills/arch-audit/SKILL.md + reference/*"
mode: "PRE-MORTEM — assume 3 months out, repeated arch-audit runs were NET NEGATIVE; reconstruct how a FAITHFUL run got there"
disposition: "read-only review; one doc; skill unmodified"
---

# arch-audit — Pre-Mortem: how a faithful run still poisoned the well

## The framing (Klein premortem)

It is 2026-10. `arch-audit` has been run on a cadence for a quarter. The verdict
is in: it was a **net negative** for ghx. Not because anyone misused it — every
run followed the run-spine exactly, grounded every claim in `file:line`, included
the YAGNI persona, wrote the "what's already right" section, and honored the
frozen measurement stack. It went wrong *while being used correctly*. This doc
reconstructs the mechanism and gives the **minimal** change (usually a narrowing,
a deletion, or a gate — never an addition) that would have prevented each path.

Bar: DH5/DH6 — every finding quotes the exact passage and names the exact flaw
with a concrete input→bad-outcome scenario. Tags: **MUST-FIX / SHOULD / OPTIONAL
/ REJECT-CANDIDATE + confidence**.

## Evidence contract

- **Read (inputs):** `.claude/skills/arch-audit/SKILL.md` (all 205 lines);
  `reference/personas.md`; `reference/artifact-template.md`;
  `docs/audits/adversarial-velocity-2026-07-07.md` (the velocity red-team worked
  example — the named `architecture-vision/adversarial-velocity-*` does not exist;
  it lives at the flat path, minor provenance drift noted below);
  `docs/audits/architecture-vision/adversarial-yagni-2026-07-07.md`;
  `docs/adr/0036-target-architecture-runner-port-and-boundaries.md`;
  `docs/adr/0035-architecture-hardening-refactor-sequence.md` (existence
  confirmed); `docs/NORTH_STAR.md`; `AGENTS.md`; `CLAUDE.md` (eval-cadence rule).
- **Commands:** `git rev-parse` (worktree + main), `ls` over the skill/audit
  trees, `mkdir -p reviews`. No code executed; no build/test run (read-only
  review of a prose skill — nothing to exercise).
- **file:line references resolve at:** main-repo HEAD `7dcfc78`.
- **Method:** for each named premortem failure mode I located the exact enabling
  passage, then constructed a concrete 3-months-out scenario using ghx's own
  roadmap state (frontier M5/M8, workstreams B8 persona revs, C4/C7/C8 eval work)
  as the collision surface. Fixes are checked against the "don't bloat it" rule:
  4 of 6 are deletions/narrowings/gates; 2 tighten an existing clause by one line.
- **Scope discipline:** skill NOT modified. This is the only file I wrote.
  Written in worktree `agent-a1c6d665a5019444c`; committed there; not merged/pushed.

## Honesty guard — what genuinely already protects (calibrate the criticism)

The skill is not naive. Before the attack, credit the real defenses, because
each failure mode below survives *despite* one of them, and the fix is usually
"make the defense bind operationally, not just aspirationally":

- **The mandatory "What is already right" section** (SKILL.md:160–162; template
  :41–42) is a real anti-manufacture guard, and the worked examples honor it —
  the velocity red-team's "What is actually fine" defers framework extraction and
  concedes the ACP wire is clean (`adversarial-velocity:82–109`). This genuinely
  stops the crudest "invent debt to justify the run" failure.
- **The mandatory ≥1 adversarial persona** (SKILL.md:60–63, 124–126) produces
  live "do less / do nothing" pressure. The YAGNI artifact's entire "What NOT to
  do" list (`adversarial-yagni:411–431`) is the counterweight working.
- **The north-star filter + no-speculative-frameworks + idiomatic-Go guardrail**
  (SKILL.md:167–171) right-sizes to a solo-dev Go product and is echoed in the
  ADR's "What we deliberately do NOT do" (`0036:189–200`).
- **The frozen-measurement-stack guardrail** (SKILL.md:162–166) is explicit and
  honored: YAGNI item 7 "keep eval measurement refactors quarantined behind eval
  ADRs" (`adversarial-yagni:407–409`); ADR-0036 defers the one measurement change
  to a gated last phase (`0036:25–26, 173–174`).
- **The final-judgment section is Panickssery-aware** (SKILL.md:144–155): it
  names self-preference, prescribes re-derivation, says "'No' … are valid
  outcomes — adding a refactor has real cost."

These are strong. The failure modes below are the places where a defense is
**stated but not wired** — where a rushed but faithful operator follows the
*operational* instruction (the template section, the ranking formula, the "run
periodically" affordance) and the *caveat* two sections away never fires.

---

## FM-1 — The fitness-function step ossifies boundaries and fights the next good refactor  ·  MUST-FIX (size/persona goldens) + SHOULD (governance) · confidence HIGH

**Quote.** SKILL.md:79–86, step 6:

> "**Graduate the wins into standing fitness functions (anti-shelfware).** The
> structural invariants the audit relied on — import-direction greps, a god-file
> size ceiling, "no `acp`/transport import outside the adapter", **the
> persona-byte golden** — should become **committed, executable checks** (a test
> or CI step) **that fail if a boundary drifts back**."

**Flaw.** This step converts *audit-specific, point-in-time* verification greps
into *standing, un-owned, un-expiring* CI gates. An architecture audit exists
precisely because good boundaries *change*; a fitness function's whole job is to
prevent change. Graduated without an owner or a change-procedure, each guard
becomes a tax on the next legitimate refactor near that boundary — with zero
context to tell "regressed by accident" from "changed on purpose." Two of the
skill's own examples are actively wrong for ghx:

- **"the persona-byte golden."** The worked example that invented this golden
  used it as a **transient** check to prove one mechanism-refactor byte-identical,
  and *explicitly warned it must not become an invariant*:
  `adversarial-velocity:435–437` — "verify with a golden test that
  `BuildPersonaSystemPrompt()` output is unchanged … **Any *new* tool added
  through the new registry is a separate, pre-registered persona revision as
  today.** Keep the mechanism change and the content change in different commits."
  The persona is a **deliberately-mutated product surface**: workstream B8
  ("Persona proficiency: mining-driven persona revisions, each pre-registered
  before the next gate run", NORTH_STAR:317; ADR-0029). A *standing* persona-byte
  golden fires red on **every legitimate persona revision** — an architecture
  guard directly blocking the product's persona-tuning roadmap. Concrete 3-months
  scenario: B8 batch 2 lands a mining-driven persona edit; CI goes red on the
  arch-audit golden; the engineer either deletes the golden (guard was theatre)
  or contorts the edit — friction manufactured by the audit, paid by product work.

- **"a god-file size ceiling."** The skill's own metrics persona ranks by
  **"complexity × git-churn hotspots — the ~1–2% of files where refactoring buys
  the most velocity, not just the biggest files"** (SKILL.md:120–124). A standing
  line-count ceiling contradicts that: it fires on the *biggest*, not the
  *hottest*, files, and it incentivizes exactly the churn YAGNI forbids. Concrete:
  a real feature pushes `acp.go` (today 471 lines, `adversarial-velocity:26`) past
  a 500-line ceiling; CI forces a premature split of a cohesive file, or the
  ceiling constant gets bumped and means nothing. Either way the guard produces
  churn-for-churn, the treadmill this skill claims to prevent.

- **Even the "good" guard ("no `acp` import outside the adapter") can fight the
  roadmap.** ADR-0036 is careful — it scopes that guard to *after* a second
  adapter exists ("becomes a **compile-time** guarantee **once a second adapter
  exists**", `0036:70`; guard test is C0, *before* C1, within the runner-port
  work). Step 6 drops that scoping and says graduate it as a standing check. If
  the codex-acp adapter (Phase C3) never ships — a real possibility for a solo dev
  who reprioritizes to M5/M8 — the guard permanently forbids the *simplification*
  of merging the single adapter back inline (the correct YAGNI move when the
  second implementation never materializes). The guard outlives its justification.

**Deeper flaw.** Ford/Parsons/Kua fitness functions (the skill's cited authority,
SKILL.md:86) come with governance: an owner and a documented procedure to change
the invariant. Step 6 ships the trip-wire without the governance, so a guard from
run N silently becomes an obstacle to run N+1 — a self-inflicted refactor-fight.

**Minimal fix (narrowing + one-line governance, no bloat):**
1. **Delete "a god-file size ceiling" and "the persona-byte golden" from the
   example list.** Size is a hotspot *input*, not an invariant; persona bytes are
   a tuned product surface. (Pure deletion.)
2. **Gate graduation on the same bar the skill already imposes on new interfaces:**
   a guard is graduated only when the boundary is *load-bearing* — it has already
   been violated-and-caught at least once, **or** a second implementation exists
   that makes it real (mirrors SKILL.md:170–171 "no abstraction of a component
   with exactly one implementation forever" and ADR-0036's "once a second adapter
   exists"). One sentence.
3. **Every graduated guard carries a one-line "to change this boundary, update
   ADR-XXXX and this test together" comment naming its owning ADR.** Turns a
   context-free trip-wire into a governed invariant. One sentence.

**Honesty guard.** ADR-0036 already models the disciplined version (guard scoped
to a second adapter). The defect is that SKILL step 6 *generalizes carelessly*
past its own worked example — it is the newest and least battle-tested step, and
the two bad examples are the tell.

---

## FM-2 — "Convergence is the signal" ranks shared-prior groupthink as corroboration; the one independent voice is spent on `wc -l`  ·  SHOULD (→ MUST if runs go citable) · confidence HIGH

**Quote.** The ranking formula: SKILL.md:74 "Rank by **cross-persona convergence
× impact × tractability**"; template:86 "Ranked sequence (impact × tractability ×
**cross-persona convergence**)"; personas.md:5 "convergence across *independent*
personas is the signal the distillation ranks on; **a lone suggestion is low
signal**"; ADR-0036:39 "**The convergence is the signal.**"

**Flaw.** The fan-out is **not** a set of independent estimators. All design
personas are opus (personas.md:9–11), all read the *same code*, all handed the
*same* NORTH_STAR + AGENTS.md + "big picture" (personas.md:7), all told to ground
in the *same* Go canon (Go Proverbs / Ben Johnson / Kat Zien, personas.md:20–21).
Five opus agents on one reading list converge because they **share a model and a
prior**, not because a finding is true. The skill *knows* this — SKILL.md:150–153
says "**discount shared-prior convergence** … agents agreeing because they read
the same code is not independent corroboration — the signal is an independent
recompute, **not the head-count**." But the caveat lives in the judgment section,
while the **operational artifact the distiller fills out** (the template's Ranked
sequence, sorted by a formula that literally multiplies by convergence) rewards
head-count. Under time pressure the operator ranks by the column in front of them.
The caveat and the formula contradict each other, and the formula wins.

**Concrete 3-months scenario (the inversion).** A run scopes
`internal/sidecar/evals`. Four opus personas each independently note "the eval
god-files are large; split them" — they all read the same 1200-line file *and*
the same AGENTS.md line calling it "real debt, deliberately deferred." That is
4-way convergence → top of the ranked sequence by the formula. But it is the
**most-shared prior possible**, and the correct answer is the one the guardrail
already states: *do not touch it* (frozen stack, SKILL.md:162–166). The single
dissenting "leave it" voice is "low signal" by the catalog's own rule
(personas.md:5). So the mechanism inverts: the truth is the lone voice; the
groupthink is ranked #1. High convergence here is a pure artifact of a shared
reading list.

**The self-preference amplifier.** The orchestrator (Fable, opus) judges an
opus fan-out — the exact self-recognition→self-preference regime the skill cites
(Panickssery, SKILL.md:146–148). The one structural defense is a non-opus voice,
but the skill routes the **only** independent-family model (Codex/gpt-5.5) to the
**metrics** pass (SKILL.md:62–63; personas.md:56–62) — `wc -l`, `gocyclo`, error
counts. The single voice that could break opus design-consensus is spent on the
*least contestable* part of the audit; the cross-family *adjudicator* is optional
and only for "the highest-stakes calls" (SKILL.md:153). Groupthink on the design
findings goes unchallenged by construction.

**Minimal fix (align formula to the existing caveat — no new machinery):**
1. **Change the sort key.** The distillation template already has an
   "Independent-recompute check" column (template:79–84). Make **that** the
   ranking key and demote "converging personas" to context: rank by
   *impact × tractability × recompute-strength*, and treat convergence as a
   *flag to audit for shared priors*, never as a multiplier. (Rewrites one
   formula line; the caveat at SKILL.md:150–153 becomes the operative rule
   instead of a footnote.)
2. **Point the independent family at a contested design call, not only metrics.**
   One clause: the Codex/gpt-5.5 worker (or a cross-family adjudicator) must
   review at least one **top-ranked design finding**, not just recompute numbers.
   This spends the single non-opus voice where groupthink is likeliest.

**Honesty guard.** SKILL.md:144–155 is genuinely the strongest self-preference
awareness I've seen in a skill — it names the paper and prescribes the right
antidote. This is a *coherence bug between two sections of the same skill*, not
an absence of awareness; the fix is to make the good section bind the ranking.

---

## FM-3 — The loop is a refactor generator run on a cadence; it competes with the north star and measures nothing about itself  ·  REJECT-CANDIDATE (periodic use) / MUST-FIX (trigger-gate) · confidence HIGH

**Quote.** SKILL.md:18 "A repeatable loop for raising ghx's **maintainability →
engineering velocity**"; SKILL.md:35 use-case "a **periodic** 'is the
architecture still healthy' sweep"; SKILL.md:94 "**Whole codebase** — a periodic
health sweep"; template:55–57 mandates a **"Recommended sequence (phased,
non-rewrite)"** section in *every* persona artifact; SKILL.md:77–78 step 5
"execute the refactors as verified worker tasks."

**Flaw — the output is structurally always "here are N refactors."** The artifact
template *mandates* a Recommended-sequence section per persona (template:55–57;
SKILL.md:136–137). There is no artifact shape whose conclusion is "the
architecture is fine; go ship product." Even the adversary whose job is to say
"no" emitted a **7-item ordered sequence** plus 4 mediums
(`adversarial-yagni:373–410`). So a faithful run *cannot* net-conclude "do
nothing" — the mandatory section guarantees a refactor backlog every run. Put
that on a "periodic" cadence (SKILL.md:35, 94) and you have a standing generator
of plumbing-refactor backlog that competes head-on with the frontier the north
star actually cares about: M5 dogfood ergonomics, M8 anticipation, C4 judge
calibration, C7 trust ledger, C8 host-task evals (NORTH_STAR:280–331) — **none of
which this loop advances.** NORTH_STAR is explicit that "The moat is the agentic
brain, **not the tools**" (:78) and that plumbing quality is a Consequence-Product
concern "never worth steering by" (:246–262). A quarterly architecture sweep is,
by the product's own priorities, off-frontier work dressed as diligence.

**The seed already shows the volume.** One 2026-07-07 run produced ADR-0035 (9
tactical items) **plus** ADR-0036 (Phases A–D, ~10 sub-items). If each cadence
sweep adds a comparable sequence and the solo dev executes even half, that is a
steady diversion from M5/M8/C4/C7/C8. The YAGNI persona's own thesis is the
indictment of the cadence: *"The architecture must make the next month cheaper,
not satisfy an abstract org chart … Everything else waits"* (`adversarial-yagni:
18–21`).

**The fatal gap — no self-measurement.** The skill *asserts* the payoff ("reuse
existing pieces to build the next thing 10x faster", SKILL.md:19–20) but ships
**zero measurement of whether a completed audit actually raised velocity.** ghx
measures the *product* obsessively (SPT, correctness, compression; the entire C
workstream) and treats "Trust in the measurement is a tracked workstream, never
an assumed property" as a tenet (NORTH_STAR:190–199). The architecture loop holds
itself to none of that. So a treadmill can run a full quarter with no signal it
helped — which is *exactly the premortem condition*: there is no feedback that
would ever tell you the loop went net-negative.

**The precedent that already settles this.** ghx has *already* rejected the
"deltas accumulated, time to re-run" treadmill for its heaviest loop — the eval
run — in the strongest possible terms (CLAUDE.md eval-cadence rule): *"Full-rigor
citable runs are **rare, event-driven, and Goga-triggered** … there is no
'product deltas accumulated, time to re-run' treadmill — that rule never existed
and is explicitly rejected."* The architecture audit is a comparably heavy loop
and must inherit the *same* posture. A "periodic health sweep" is precisely the
treadmill the repo already killed elsewhere.

**Minimal fix (gate + delete, a strict narrowing — no additions):**
1. **Delete "periodic health sweep" as a standing use** (SKILL.md:35, 94).
   Replace "When to use" with an explicit **trigger list**: run only when
   (a) a *named* roadmap step is concretely blocked by a boundary (the velocity
   red-team's real job — V1 was a genuine P3 blocker), (b) a second
   consumer/implementation is imminent (the runner port's real trigger), or
   (c) a churn×complexity hotspot crosses a threshold *and* sits on the current
   frontier milestone's critical path. Mirror the eval-cadence rule verbatim in
   spirit: rare, event-driven, trigger-gated — never a cadence.
2. **Make "do nothing" a first-class output.** Add one line to the artifact
   contract: a persona MAY conclude with an empty Recommended-sequence and a
   *watch-list* instead — "the architecture serves the frontier; here is what to
   watch, deferred until it blocks something." (Converts the mandatory-refactor
   section into a mandatory-*disposition* section; net zero length.)
3. **The governing ADR carries an execution-gate line:** "no phase executes
   unless it unblocks the current frontier milestone or fixes a live bug." Makes
   the ADR a *conditional* plan, not a backlog. One sentence.

**Honesty guard.** SKILL.md:37–38 already flags cost ("pays off when
architectural quality genuinely moves velocity") and :155 blesses "No" as an
outcome — but both are advisory prose that a mandatory-refactor template and a
"periodic" affordance override in practice. The fix is to make the frontier
filter *gate the run*, not merely caveat it.

---

## FM-4 — Multi-agent cost per run (6–9 opus/Codex generations + serial orchestrator attention)  ·  OPTIONAL / folds into FM-3 · confidence MEDIUM

**Quote.** SKILL.md:37 "The loop costs **several agent generations**"; a run is
4–6 personas + a fresh distiller + the orchestrator + an optional cross-family
adjudicator (SKILL.md:59–75) — 6–9 generations, most on opus, each re-reading the
codebase + NORTH_STAR + AGENTS.md, *then* an execution phase of N "verified worker
tasks" (SKILL.md:77–78).

**Flaw (honestly re-priced).** In *this* setup the raw generation count is a weak
objection: CLAUDE.md fleet economics say the default is "**parallel background
Claude workers … spawned wide — under-delegation wastes the plan**," so opus
fan-out is close to free. The scarce resource is not tokens — it is the
**orchestrator's serial attention**: Fable is "the serial merge point" who must
re-derive load-bearing claims (SKILL.md:150), judge N artifacts, resolve tensions,
write the ADR, and supervise execution. That premium serial judgment competes
directly with shipping the frontier. But this is the *same* cost as FM-3's
treadmill, and its fix is FM-3's gate.

**Minimal fix.** None standalone — trigger-gating (FM-3 fix 1) caps the
generation count *and* the serial-attention cost at once. Do not add a
token-budget clause; that would be bloat against a non-binding constraint.

**Honesty guard.** The naive "too many opus calls" attack does **not** land here —
the plan economics defang it, and the skill's own "delegate breadth, keep
judgment" (SKILL.md:22–23) is the right division. I flag this only to record that
the real cost is orchestrator judgment + execution follow-through, which is the
treadmill problem, not the fan-out.

---

## FM-5 — Two-sided shelfware: stale "target architecture" ADRs that mislead, or endless hardening that starves the frontier  ·  SHOULD / folds into FM-3 · confidence MEDIUM

**Quote.** Step 6 is titled "(**anti-shelfware**)" (SKILL.md:79); step 5 "execute
the refactors as verified worker tasks" (SKILL.md:78); step 6 "An audit that only
writes a report **decays**" (:84–85).

**Flaw (both directions).**
- **Shelfware / stale map.** The run commits ADR-0036 titled "**Target
  Architecture**" (`0036:1, 9`) describing a runner port that **does not exist in
  the code** (status `proposed`). A future agent doing recon reads
  "Target Architecture," takes the port as real, and builds on a boundary that was
  only ever proposed. AGENTS.md warns exactly against this: "do not leave stale
  guidance … state that directly and update `status`" (:81–85). A half-executed
  audit sequence leaves the tree littered with aspirational ADRs that read as
  descriptive.
- **Over-execution.** The anti-shelfware pressure (step 6 + "execute the
  refactors") *plus* a cadence (FM-3) produces the opposite failure: the code
  keeps getting "cleaner" against audit findings while C4 judge calibration, M8
  anticipation, and C7 trust rows — the actual frontier — wait. "Endless
  hardening starves product/eval work" is the premortem's own named fear, and
  step 6 is its engine.

**Minimal fix.** Folds into FM-3 fixes 2–3: an execution-gate line ("no phase
executes unless it unblocks the frontier or fixes a live bug") makes the ADR a
*conditional* plan, and the default disposition for non-blocking findings is
**DEFERRED (documented, not scheduled)** — the exact posture AGENTS.md already
uses for the eval god-files ("real debt, deliberately deferred", SKILL.md:166).
No new mechanism.

**Honesty guard.** The skill genuinely tries to solve shelfware (step 6 exists
for it). The problem is that its solution (graduate guards + execute) is itself
FM-1 and FM-5b. The right anti-shelfware move for a solo dev is *fewer, gated*
runs whose small output actually ships — not more standing guards.

---

## FM-6 — "Behavior-preserving" ≠ "measurement-preserving"; a green-test eval refactor can silently move a citable number  ·  SHOULD · confidence MEDIUM-HIGH

**Quote.** Guardrail SKILL.md:162–166 "**Respect the frozen measurement stack.**
Never propose changing eval scoring, gates, detectors, or reward math as a
drive-by"; behavior-preservation check SKILL.md:172–174 "every recommended step
must be … **provable byte-identical (unmodified test suite + race)**."

**Flaw.** The frozen-stack guardrail forbids *proposing* a scoring change, but a
faithful refactor can *touch scoring code without anyone classifying it as a
measurement change* — and the skill's proof-of-safety ("unmodified test suite +
race") does **not** prove score-identity. The eval unit tests assert on small
fixtures; they do not re-run the frozen corpus. So a "behavior-preserving"
extraction of a shared helper used by `rewards.go`/`gates.go` can pass
`go test ./...`, land ff-only, and *still* perturb a corpus score distribution
(map-iteration order affecting a tie-break; a refactored float accumulation
order). The measurement stack's real invariant is **score recomputability**
(AGENTS.md:125 "Every score must be recomputable"), which is strictly stronger
than "tests pass." The worked examples show the discipline for *persona* output
(`adversarial-velocity:435–437` golden) but there is **no analogous
corpus-score-identity requirement** for eval-scoring refactors.

**Concrete scenario.** An audit of `internal/sidecar/evals` recommends
de-duplicating an accumulation helper shared by `rewards.go`. A worker does it;
unit tests (small fixtures) stay green; lands ff-only; the mid-run corpus's G1
ratio shifts at the 3rd decimal. A "citable" number moved via a refactor everyone
believed was byte-identical — the precise "we lie to ourselves" failure
Visibility & Truthfulness exists to prevent (AGENTS.md:119–141).

**Minimal fix (tighten one existing clause, no new section).** Add to the
frozen-stack guardrail: *"'behavior-preserving' for any file reachable in the
import graph of `internal/sidecar/evals` scoring means **frozen-corpus scores
bit-identical**, proven by re-running the seeded corpus — `go test` passing is
necessary but not sufficient. 'Touches the measurement stack' is defined by
import-graph reachability into scoring, not by intent."* One clause; turns a
guardrail that relies on the operator *recognizing* a scoring change into one
that defines it structurally.

**Honesty guard.** This is the skill's *strongest* guardrail and the worked
examples honor it faithfully; the residual is narrow (behavior-preservation ≠
measurement-preservation on shared helpers) but load-bearing given the eval stack
is the north star's proof machinery. I did **not** find any path where the
judge's self-preference *defeats* the frozen-stack guardrail directly — that
concern routes into FM-2 (the ranking formula rewards the convergence the judge
is told to discount, and the one cross-family voice is spent on metrics), not
here.

---

## Declined / did-not-flag (discipline record)

- **"The audit invents debt from nothing."** Declined — the mandatory "What is
  already right" section (SKILL.md:160–162) genuinely blocks the crudest form,
  and the worked examples demonstrate it. The real risk is subtler (FM-3: the
  *template* mandates a refactor list even when the honest answer is "nothing").
- **"Fan-out cost is unaffordable."** Declined as weak — CLAUDE.md fleet
  economics make wide opus fan-out near-free; the binding cost is serial
  orchestrator attention (FM-4), which folds into FM-3.
- **"It invents a framework/protocol against Open-Source-Leverage."** Declined —
  SKILL.md:167–171 + ADR-0036:189–200 explicitly forbid speculative frameworks,
  and the YAGNI persona enforces it. This defense holds.

---

## Top 3 failure modes + minimal fixes (for the orchestrator)

1. **FM-3 — Refactor treadmill run on a cadence, measuring nothing about itself,
   competing with the frontier (REJECT-CANDIDATE for "periodic sweep" / MUST-FIX,
   HIGH).** *Fix:* delete "periodic health sweep"; replace "When to use" with a
   trigger list (roadmap-blocked boundary / imminent 2nd implementation / hotspot
   on the frontier critical path) mirroring the repo's already-binding eval-cadence
   rule ("rare, event-driven, triggered — the treadmill is explicitly rejected");
   make "do nothing + watch-list" a first-class artifact output; add an
   execution-gate line to the governing ADR. Strict narrowing.

2. **FM-1 — The fitness-function step ossifies boundaries and fights the next good
   refactor; the persona-byte golden and size-ceiling examples are actively wrong
   (MUST-FIX for those two / SHOULD for governance, HIGH).** *Fix:* delete "god-file
   size ceiling" and "persona-byte golden" from step 6; graduate a guard only when
   the boundary is load-bearing (violated-and-caught once, or a 2nd implementation
   exists — the skill's own "one implementation forever" bar); every graduated
   guard names its owning ADR in a "change this only via ADR-XXXX" comment.

3. **FM-2 — "Convergence is the signal" ranks opus shared-prior groupthink as
   corroboration; the one independent-family voice is spent on `wc -l` (SHOULD, →
   MUST if runs go citable, HIGH).** *Fix:* re-key the ranked sequence on the
   template's existing "independent-recompute" column, demoting convergence to a
   flag-to-audit (multiply nothing by head-count); require the cross-family worker
   to review at least one top-ranked *design* finding, not only metrics. Aligns
   the ranking formula with the skill's own already-present caveat.

All three fixes are deletions/narrowings/gates or one-line tightenings of existing
clauses — none adds a section or bloats the skill. Net effect: the skill becomes
**narrower and trigger-gated** (run on a real trigger, cap the cadence, rank on
recompute not head-count, graduate guards only when load-bearing) — which is what
turns it from a quarterly ritual that starves the north star into a scalpel used
when a boundary actually blocks the frontier.
