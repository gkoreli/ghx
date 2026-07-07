---
title: "arch-audit Round-2 review — Craft & Usability"
date: "2026-07-07"
status: "review"
skill: "arch-audit"
scope: "CRAFT & USABILITY — is it a good, runnable skill?"
reviewer: "adversarial reviewer (read-only; skill-forge Round 2)"
bar: "Graham DH5/DH6 — exact quote + exact flaw"
---

# arch-audit — Craft & Usability review

Adversarial Round-2 review of `arch-audit` from the seat of an operator who has to
**run** it: is the run-spine front-loaded and executable end-to-end, could Fable
follow it without getting stuck or guessing, is the altitude thin-and-operational,
does progressive disclosure work, are the scope boundary and neighbour cross-refs
correct, and would the artifact contract + `AA-000N` threading produce merge-clean,
distillable artifacts? Every finding quotes the exact passage and names the exact
flaw; every recommendation is tagged and confidence-rated; scope creep is argued
*against*, not for.

## Evidence contract

- **Artifact reviewed** (byte-identical to committed `HEAD:.claude/skills/arch-audit/SKILL.md`,
  confirmed by `diff` — 204 lines / 1694 words):
  `.claude/skills/arch-audit/SKILL.md`, `reference/personas.md` (86 lines),
  `reference/artifact-template.md` (102 lines).
- **Quality target read in full** (does following the skill reproduce this?):
  `docs/audits/architecture-2026-07-07.md` + `docs/adr/0035-architecture-hardening-refactor-sequence.md`.
- **Calibration siblings skimmed:** `.claude/skills/product-audit/SKILL.md` (structural twin,
  455 lines / 4873 words), `.claude/skills/skill-forge/SKILL.md` (meta-loop, 187 lines).
- **Commands run (recomputable):**
  - `wc -l -w` on the five SKILL/reference files (sizes above).
  - `grep -n '```' .claude/skills/arch-audit/SKILL.md` → **zero fenced code blocks** (basis of F1/F2).
  - `grep -rni security .claude/skills/arch-audit/` → security named only in a research file's
    out-of-angle note, **never in SKILL.md** (basis of F3).
  - Cross-ref resolution: `grep -n "Git Hygiene" CLAUDE.md` → **line 143 exists**;
    `grep -n "Integrating Delegated" AGENTS.md` → **line 313 exists**; every provenance path
    (`architecture-2026-07-07.md`, `refactor-review-2026-07-07.md`, `architecture-vision/` [6 files],
    `docs/adr/0035-*`, `docs/adr/0036-*` → resolves to `0036-target-architecture-runner-port-and-boundaries.md`,
    `research/01-04`) **resolves — no dead links**.
- **Not done:** I did not run the loop; this reviews the *document's* runnability, not a live audit.

---

## Honesty guard — what the skill gets right (equal rigor)

1. **The altitude is genuinely thin and operational-core-first.** 204 lines / 1694 words
   — *less than half* the twin `product-audit` (455 / 4873). The `## The run-spine
   (front-load this — how to run, what to deliver)` header leads at line 47 with a numbered
   1–6 executable sequence and a closing `**Deliverables:**` line. The truncation risk the
   skill itself worries about (Provenance: "The SKILL.md stays thin by design") is largely
   self-mitigated by *being short* — the whole operational core fits well inside any
   plausible retention window. This is the single hardest thing to get right and it is right.
2. **Every cross-reference resolves — I checked all of them.** `CLAUDE.md "Git Hygiene on a
   Shared Mainline"` (line 143), `AGENTS.md "Integrating Delegated Work"` (line 313), and all
   nine provenance artifacts exist. No dead links, which is rare for a skill this cross-ref-heavy.
3. **The two mandatory counterweights are IN the spine, not buried.** Run-spine step 2:
   "**at least one adversarial** (YAGNI skeptic and/or velocity red-team) as the mandatory
   counterweight, and route the **mechanical metrics** pass to a Codex/GPT worker for an
   independent-model-family perspective." An operator physically cannot skim the run-spine and
   miss the anti-groupthink requirement — the correct place for a load-bearing rule.
4. **The artifact contract faithfully reproduces the worked example's quality.** Every section
   the template mandates — Executive summary, "What is already right (verified, not assumed)",
   High/Med/Low tiers each with `file:line` + a named-tenet tie + a concrete recommendation, a
   phased behavior-preserving "Recommended sequence", and "Method / auditability" — is exactly
   what `architecture-2026-07-07.md` contains (H1 cites `executor.go:180-196`, ties to the
   "No legacy maintenance" tenet, recommends a scoped deletion; the "What is clean (verified,
   not assumed)" block is present). The central craft question — *would following this produce
   that doc?* — is **yes for the per-artifact shape.**
5. **Merge-clean and distillable by construction.** Disjoint per-run directory + one-artifact-
   per-worker + ff-only landing → the fan-out lands without conflicts; the distillation
   template's convergence table + "Tensions held (not averaged)" is genuinely distillable; and
   `AA-000N`/`.k`/`.9` mirrors `product-audit`'s `PA-000N`, so the house style is consistent
   across the two audit loops.
6. **The self-preference guard is a real methodological strength, not decoration.** The "final
   judgment" section grounds the self-preference risk in a citation (Panickssery et al.,
   NeurIPS 2024), demands re-derivation of load-bearing claims, discounts shared-prior
   convergence, and legitimizes refusal — "'No', 'this is fine', and 'the premise is wrong'
   are valid outcomes." Paired with the mandatory "what's already right" section, this is a
   well-designed defense against manufactured findings.

---

## Findings (tagged, confidence-rated)

### F1 — No copyable worker-delegation prompt; the fan-out is not runnable without hand-assembling the brief from four files. **MUST-FIX (confidence: high)**

**Exact passage (run-spine step 2):** "Pick 4–6 persona×lens pairs for the scope from the
catalog (`reference/personas.md`) … Each worker: read-only, writes **exactly ONE artifact** to
a disjoint path, grounds every claim in `file:line`, and returns the evidence contract. …
(Delegation syntax: `fable-delegation`.)"

**Flaw.** This *describes* a worker but ships no **copyable prompt** to spawn one.
`grep '```'` over the whole SKILL returns **zero fenced blocks**. To launch even one of the
4–6 workers, the operator must hand-stitch the brief from four separate places: the lens from
`reference/personas.md`, the output shape from `reference/artifact-template.md`, the constraints
from the "Guardrails" section, and the command syntax from `fable-delegation` — then repeat that
assembly 4–6 times per run. The structural twin does not make the operator do this:
`product-audit` ships a **"Copyable persona-agent delegation prompt:"** fenced block that bundles
the persona pointer, the north-star grounding, the surfaces, the tools, and the return shape in
one paste-able unit; `CLAUDE.md` itself ships a copyable Codex delegation prompt. For a
*fan-out* skill whose entire value is delegated breadth, the absence of the one artifact that
makes delegation turnkey is the sharpest "would get stuck / would guess" gap in the document.
The skill's own dictum — "Delegate breadth, keep judgment" — is undercut when the breadth step
has no ready launcher.

**Recommendation.** Add one fenced "Copyable worker prompt" block immediately after run-spine
step 2, with fill-in slots: `<persona from reference/personas.md>`, `<scope + ranked quality
attributes from the charter>`, the fixed grounding line (read `NORTH_STAR.md` + `AGENTS.md`
tenets first), the artifact path/template pointer, model routing (opus vs Codex), text-only
tools, and the explicit **return shape** (closes F2). This is ~12 lines and *removes* net
operator effort — it is not scope creep, it is the missing launcher. **This is the single
highest-value change** (see below); it also carries the vehicles for F2, F4, and F5.

### F2 — "returns the evidence contract" names a return but never specifies the return *shape*. **SHOULD (confidence: med)**

**Exact passage (run-spine step 2):** "…and returns the evidence contract." **Deliverables:**
"the thread charter, N persona artifacts, one distillation artifact, a governing ADR, and any
new fitness-function checks…"

**Flaw.** "The evidence contract" is a canonical `AGENTS.md` concept (per `CLAUDE.md`, "The
evidence contract lives in `AGENTS.md`"), so the *term* is legitimate — but it is neither
cross-referenced at point of use nor unpacked into the concrete **return-message fields** the
orchestrator needs to triage a completed worker. `product-audit` states them outright: "Return:
artifact path, top findings, source count, least-confident call"; `skill-forge` likewise asks
for "an executive summary, source count, and honestly-flagged gaps." arch-audit leaves the
worker to guess what to put in its final message versus what goes in the on-disk artifact. The
"what I checked and found SOUND (nulls)" honesty section that `product-audit` demands per-worker
is also absent from arch-audit's worker instructions (it lives only implicitly via the artifact
template's "What is already right").

**Recommendation.** In the F1 copyable block, add a `Return:` line — artifact path, top
findings, a per-worker "checked and found sound (nulls)" note, and the least-confident call.
One line; no new section.

### F3 — The scope boundary omits `security-review`, `code-review`, and `simplify` — three sibling code-lens skills this audit is genuinely confusable with. **SHOULD (confidence: med)**

**Exact passages.** When to use / when not: "**Don't** use it for a single-file review (just
read it), a mechanical rename, or a product/market question (use `product-audit`)." Neighbours:
"`product-audit` … `skill-forge` … `fable-delegation` …".

**Flaw.** `grep -rni security` over the skill confirms **security is never mentioned**. But
`security-review` is a live sibling skill and a *different code lens* on the same
codebase — the exact kind of boundary this audit owns. The structural twin routes both
directions explicitly ("Security review → the `security-review` skill"; single-diff code
correctness → "the code-review/simplify skills"), which is what makes its "when NOT" trustworthy.
arch-audit tells the operator to route product questions away but is silent on the two nearest
*code* neighbours, so a "security architecture" or "review this diff" request has no landing
spot. This is a correctness gap in the boundary, not just an omission.

**Recommendation.** Add one clause to "When to use / when not" (or one Neighbour line): "not a
security review (`security-review`), and not a single-diff quality pass (`code-review` /
`simplify`)." **Resist the reflex to import `product-audit`'s full 30-line "In scope / out of
scope" section** — see REJECT-CANDIDATE R1. One clause, not a section.

### F4 — Persona frontmatter has no model/family field, yet the whole ranking signal is "independent-family convergence." **SHOULD (confidence: med)**

**Exact passages.** `reference/artifact-template.md` persona frontmatter: `author: "background
audit worker (read-only)"` — a generic string. The skill's load-bearing ranking rule (final
judgment): "**discount shared-prior convergence** (agents agreeing because they read the same
code is not independent corroboration — the signal is an independent recompute, not the
head-count)"; `personas.md` on the metrics worker: "Independent model family is a feature — it
counters the Claude personas' shared priors."

**Flaw.** The distiller's convergence table (distillation template: "Note which convergence is
shared-prior … vs genuinely independent") must separate the opus personas (shared prior) from
the Codex/gpt-5.5 metrics worker (genuinely independent family). But the persona artifacts do
not record *which family authored them* — `author` is a fixed generic label — so the single
attribute the ranking turns on is not captured in the artifact's own frontmatter. The distiller
has to reconstruct family from memory of who was spawned. This makes the skill's central
methodology harder to execute than the skill claims it is.

**Recommendation.** Add `model:` (or `family:`) to the persona frontmatter template. This is
*not* scope creep — it makes an already-stated claim mechanically checkable, and it costs one
frontmatter line. High coherence with the skill's own thesis.

### F5 — The run-spine names the charter *file* but not the per-run *directory*; the load-bearing path convention lives only in the reference template. **OPTIONAL (confidence: med)**

**Exact passages.** Run-spine step 1: "Commit it as the thread header `AA-000N-<scope-slug>.md`."
Step 2: "writes **exactly ONE artifact** to a disjoint path." The actual convention —
`docs/audits/AA-000N-<scope-slug>/` per-run directory — appears *only* in
`reference/artifact-template.md`.

**Flaw.** "Disjoint artifacts → no merge conflicts" (step 2) is the mechanism that lets the
fan-out land ff-only, so the *directory* convention is load-bearing — yet the spine's single
path token (`AA-000N-<scope-slug>.md`, no directory) reads as a flat file, matching the
pre-convention worked example, not the template. If the harness ever does truncate below the
spine (the risk the skill cites), the operator has the launch sequence but not the layout.

**Recommendation.** Add the directory to step 1: "under `docs/audits/AA-000N-<scope-slug>/`."
Tagged **OPTIONAL** and not SHOULD because the skill is short enough that truncation is unlikely
and progressive disclosure legitimately keeps the full template in the reference file. Marginal.

### F6 — Step 6 ("fitness functions") is the least actionable step: direction without a concrete wiring. **OPTIONAL (confidence: low)** — leaning *leave it*

**Exact passage (run-spine step 6):** the structural invariants "should become **committed,
executable checks** (a test or CI step) that fail if a boundary drifts back … the 2026-07-07
run's checks were one-shot greps — graduate them."

**Flaw and counter-flaw.** There is no concrete "grep-as-Go-test" example and no pointer to
where such a check lives in the repo, so of the six spine steps this is the one an operator is
most likely to defer. *But* the step names concrete, actionable candidate checks
("import-direction greps, a god-file size ceiling, 'no `acp`/transport import outside the
adapter', the persona-byte golden"), and demanding a tutorial would bloat a skill whose thinness
is a feature. I flag it as the weakest link for honesty, but **recommend leaving it as-is** —
adding a how-to here would be the exact over-engineering the YAGNI persona exists to prevent.

### F7 — Minor redundancy: `file:line`-grounding stated 3×; judge/shared-prior logic spans spine step 4 and the "final judgment" section. **OPTIONAL (confidence: low)** — light trim at most

**Exact passages.** "Ground every claim in `file:line`" recurs in run-spine step 2, the Personas
section, `personas.md`, and Guardrail #1. "the orchestrator judges it" + shared-prior discount
appear in both run-spine step 4 *and* the "final judgment" section.

**Flaw and counter-flaw.** Some tokens repeat in a skill whose own thesis is token economy. But
the "final judgment" section earns its place: it adds the Panickssery citation, "eat the fish,
throw the bones," and the cross-family adjudicator — content the spine lacks. So this is
elaboration, not pure duplication. A one-line trim of the *triple*-stated `file:line` rule is
the most I'd do. Included mainly to keep this review honest that the fix is not always "add."

---

## REJECT-CANDIDATES (arguing *against* additions)

- **R1 — Do NOT import `product-audit`'s full "In scope / out of scope (the skill's fixed
  charter boundary)" section (confidence: high).** That section is ~30 lines and lives in a
  skill twice arch-audit's length. arch-audit's "When to use / when not" + "Neighbours" already
  carry the boundary; F3's fix is a *single clause*, not a mirrored section. Cloning the twin's
  structure would cargo-cult the wrong thing and bloat a skill whose thinness is its best
  feature.
- **R2 — Do NOT inline the persona catalog or the research URLs into SKILL.md (confidence:
  high).** Progressive disclosure is working: the catalog is in `personas.md`, the citations in
  the `research/01–04` Source Ledgers. Inlining would be precisely the anti-pattern the skill
  preaches against ("Don't inline 100 URLs … point to the ledgers"). Leave it.

---

## The single highest-value change

**Add a copyable worker-delegation prompt (F1)** as one fenced block right after run-spine
step 2. It is the highest-leverage, lowest-cost edit because it converts the skill from
"assemble each worker's brief from four files, 4–6 times per run" into "fill three slots and
spawn," and it is the natural carrier for three other findings at once: the concrete **return
shape** (F2), the **model/family** capture (F4), and the per-run **directory path** (F5). It
matches what the structural twin `product-audit` and `CLAUDE.md` already ship, and it directly
answers this review's governing question — *could Fable run it without getting stuck or
guessing?* Today, at the fan-out step, the honest answer is "not without hand-assembly." One
fenced block fixes that.

**Net verdict:** a well-shaped, genuinely thin, correctly-cross-referenced skill whose
per-artifact contract provably reproduces the worked example — with **one** real runnability
hole at the delegation step (F1) and three cheap tightenings (F2/F3/F4). No REJECT of the skill;
no premise problem. Fix F1 and it is turnkey.
