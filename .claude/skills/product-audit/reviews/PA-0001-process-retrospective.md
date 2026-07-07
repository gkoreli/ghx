---
title: "PA-0001 Process Retrospective — the product-audit skill's first live run"
date: "2026-07-07"
status: "review"
subject: "product-audit process — PA-0001 live run"
author: "process-improvement reviewer (Fable / Opus), read-only"
---

# PA-0001 Process Retrospective

**Subject: the audit *process*, not the ghx product.** This reviews the `product-audit`
skill *as executed* in PA-0001 — the failure patterns a static review (R4-meta-redteam)
could not see because they only appear when a real orchestrator runs a real fleet against
a real product. Every finding quotes the PA-0001 artifacts and names the specific flaw
(Graham DH5/DH6), proposes a specific skill edit, tags MUST-FIX/SHOULD/OPTIONAL + a
confidence, and argues it clears the skill's own anti-bloat bar (R4-F7: the skill is a
standing context tax on a premium orchestrator; only changes that earn their space ship).

**Scope discipline (dogfooding my own PF1 below).** Two obvious candidate findings —
"the run declared no focus" and "the cross-family check had no fallback" — were **already
fixed** by commit `c8d5eaa` (2026-07-07 10:13), which postdates the PA-0001 synthesis
(09:33) and added directive 6 (declare a focus) and the fresh-Claude-adjudicator fallback
*because of this run*. Re-proposing them would be the exact ledger-restatement this
retrospective flags as PF2. They are therefore **credited as already-corrected**, not
re-raised; my findings are the residue the skill has *not* yet closed.

**What the live run reveals that R4 could not.** R4-F1 predicted the judge would
*rubber-stamp* under fleet pressure — a plausibility skim of its own family's output. The
live run falsifies that specific prediction (see "What the process did well") but exposes a
subtler, more dangerous failure in its place: the judge **actively re-derived and got it
wrong**, manufacturing a false "contradiction" and dismissing a *correct* persona finding
with high confidence (PF1). Active mis-adjudication is harder to catch than a skim,
because the report *looks* more rigorous, not less.

---

## What the process did well (honesty guard — equal rigor)

A retrospective that only indicts is useless; these are load-bearing, and they are why the
criticisms below carry weight.

- **W1 — The judge step actually ran, and recorded its own provenance.** The synthesis's
  "Judge-step provenance" section names *which* facts were recomputed and from what:
  `.claude-plugin/plugin.json` (H1), the `corpus-discrimination` table (H2),
  `grep -rn schemaVersion internal/` (M3), competitor stars via `gh` (H3). This is
  directive 5 working as a mechanism, not a slogan — and it directly falsifies R4-F1's
  rubber-stamp scenario. The orchestrator re-opened evidence rather than skimming.
- **W2 — Shared-prior was caught, and named as such.** *"H2's convergence across the AI-PM
  and North-Star personas is **not** independent corroboration — both read the same
  `corpus-discrimination` doc; it was re-derived from the artifact directly, not from the
  agents' agreement."* This implements R4-F3 ("convergence is not corroboration") on the
  single most convergent finding. (Its limit is PF5: it was caught by the orchestrator's
  *memory*, not a mechanism, and only for 1 of the 2 convergences.)
- **W3 — The cross-family gap was made a first-class *blocking* state, not hidden.**
  *"every High finding below is self-verified but NOT yet cross-family-checked, and none
  may feed a milestone go/no-go until that check runs... This gap is the honest state,
  recorded rather than hidden."* Graceful degradation with an explicit citability gate is
  exactly the behavior the evidence contract demands. (PF3 hardens the *form* of this.)
- **W4 — Persona-level nulls were required and delivered with real content.** Every
  `PA-0001.k` carries a substantial "What I checked and found SOUND" section (PA-0001.2 has
  nine credited nulls; PA-0001.4 has five). This implements R4-F6 and prevents the
  finding-biased pile.
- **W5 — Fairness was structural, not decorative.** The headline finding H2 is explicitly
  labeled *"narrative/framing lag, NOT fabrication,"* and PA-0001.2 credits ghx's honest
  ledgers by name. A red-team that credits its target this precisely is DH6, and it is the
  correct antidote to the "demoralizing pile" failure mode.
- **W6 — Dogfooding produced genuine free eval signal.** PA-0001.4's discovery-tier `ghx
  sidecar ask` (66s) beat its own raw `ghx repos` calls at competitive recon — *"the moat
  thesis proven in the act."* The "use the product to audit the product" mandate paid off
  with a real behavioral datum (and, notably, the audit's *only* strong behavioral
  evidence — which is PF4).

These six are strong enough that the skill's core machinery is sound. The findings below
sharpen a working instrument.

---

## Ranked process findings

### PF1 — Evidence drift *inside the judge step*: the orchestrator re-derived the wrong quantity and dismissed a correct finding. **[MUST-FIX · high confidence]**

**Failure pattern.** Evidence drift — but the sharp, live-only instance is that it happened
in the *judge* step (directive 5), the one place the skill trusts most. The judge recomputed
a *different quantity* than the finding measured, found the two numbers unequal, and
declared the finding "unverified."

**Evidence (quote → flaw).** Synthesis, Judge-step provenance: *"`wc -l
internal/sidecar/prompt.go` = 350, which **contradicts** a persona's '141 lines' figure (L1
nuance)."* And L1: *"`internal/sidecar/prompt.go` is **350 lines total** (the persona's '141'
is an **unverified substring**; Fable did **not** adopt it)."* But PA-0001.3 F7 did not
measure the source file — it measured the **rendered output** of `BuildPersonaSystemPrompt()`:
*"141 lines, 6858 bytes, ~1714 tokens,"* with a stated reproduction method (*"add a throwaway
test... calling `BuildPersonaSystemPrompt()`... log `len`, line count"*). I verified the
ground truth: `internal/sidecar/prompt.go` is 350 lines **and contains two distinct persona
functions** — `func BuildPersonaSystemPrompt()` at line 37 and `func
BuildDiscoveryPersonaSystemPrompt()` at line 221 — plus their Go string literals, package
decl, and imports. So `wc -l` of the file **cannot** be the rendered per-session doctrine tax
the NORTH_STAR "~400-line persona doctrine" claim is about; 350 and 141 measure
non-comparable things and do not "contradict." The judge picked the wrong measurement,
called the correct one an "unverified substring," and discarded it. The persona had *even
predicted this exact confusion* in its least-confident call: *"it may be... a count of
`prompt.go` source lines (~351) rather than the rendered doctrine."* The judge walked into
the trap the finding flagged. (Corroborating fragility: `handleRecon` is cited as
`serve.go:208-220` in synthesis M1 but `serve.go:208-221` in PA-0001.3 F2 — bare `file:line`
ranges already wobble between persona and synthesis.)

**Specific skill change.** Two edits:
1. **Process step 5** (SKILL.md ~L126, "re-derive it from raw evidence yourself"): append a
   *same-quantity rule* — *"Re-derive the **exact quantity the finding measured**, using the
   finding's own stated recompute command — never a proxy. A different measurement yielding a
   different number is **not** a refutation; if you cannot run the finding's command,
   reconcile the two quantities explicitly before dispositioning. A judge that measures X to
   reject a finding about Y has drifted."*
2. **Output contract, Evidence bullet** (SKILL.md L185-186): promote the evidence form from
   bare `file:line` toward *"**symbol + a quoted snippet + a runnable recompute command**"* —
   line numbers rot and invite the 350-vs-141 category error; a symbol + snippet + command is
   stable and is the thing the judge re-runs.

**Anti-bloat.** Two sentences into the already-load-bearing judge step, plus tightening one
existing bullet. It earns its space by preventing the judge from *manufacturing* false
contradictions — a failure strictly worse than the un-cited-claim failure the evidence
contract already guards, because it destroys *correct* signal with the authority of
"re-derived."

---

### PF2 — No novelty axis: findings restate the team's own committed ledgers, so "known-confirmed" masquerades as "new High finding." **[MUST-FIX · high confidence] — single highest-leverage change.**

**Failure pattern.** Repetition risk. The audit re-derives what TRUST.md, `corpus-discrimination`,
and the milestone table already say, and presents it as discovery — the exact
ledger-restatement the founder's anti-repetition tenet (CLAUDE.md; MEMORY) forbids.

**Evidence (quote → flaw).** The personas *themselves* noticed they were restating known
material, but the contract gave them no place to say so structurally, so it surfaced as
prose asides while the synthesis still ranked the items as fresh Highs:
- PA-0001.2 null #7: *"The corpus's own weakness (2/6 discriminating) was found by the team,
  not by me... A red-team that only re-states the target's own committed self-critique is
  **corroborating rigor, not finding a hole**."* — yet that same fact is F2, a High.
- PA-0001.2 F1: *"The team knows this."* PA-0001.5 closing: *"the fix for F1–F5 is largely to
  make the flagship narrative say what the **ledgers already say**."*
- The synthesis concedes the whole pattern: *"ghx's honest internal ledgers... are
  consistently more truthful than the flagship NORTH_STAR narrative... The fix for most High
  findings is narrative honesty... not new machinery."*

So H2 (the report's *co-headline*), F1(AI-PM), F2, F5(Zealot), F6(Zealot), and the M8
anticipation finding are all **KNOWN-CONFIRMED** — the team documents each in a committed
ledger; the only *new* delta is "the banner overstates the ledger." Ranked as six severity-High/Med
findings, that delta is inflated ~6×; it is really **one** finding ("NORTH_STAR banner
outruns TRUST.md / corpus-discrimination") plus a pointer table.

**Specific skill change.** Output contract, "Ranked findings" (SKILL.md L181-193): add a
mandatory per-finding tag **`Novelty: NEW | KNOWN-CONFIRMED ⟨ledger ref⟩ | KNOWN-DISPUTED
⟨ledger ref⟩`**. Rule: a KNOWN-CONFIRMED finding **must cite the existing ADR/TRUST-hole/eval
doc it restates and state the delta** (what is new beyond the ledger). *If the delta is nil,
it is a citation, not a finding, and is dropped.* Add one line to Process step 5: *"Before
ranking, label each finding NEW / KNOWN-CONFIRMED / KNOWN-DISPUTED against existing ledgers
and prior `PA-000N`; collapse KNOWN-CONFIRMED restatements into a single 'delta vs the
ledger' finding."*

**Anti-bloat.** This is the rare change that makes the *output* dramatically smaller — it
would compress PA-0001's ranked list by roughly half and convert the co-headline from an
alarming "High" into an accurate "the ledger is right; the banner isn't." One tag on the
contract, one sentence in step 5. It also gives the shared-prior guard (W2) teeth: three
personas citing the same ledger *is* a KNOWN-CONFIRMED convergence by construction, detected
by the tag rather than by the orchestrator's memory. Highest leverage of any change here.

---

### PF3 — Verification state is prose, not a first-class structured field, so citability isn't mechanically gated. **[MUST-FIX (residual after c8d5eaa) · medium-high confidence]**

**Failure pattern.** Cross-family check as a single point of failure. The skill (post-`c8d5eaa`)
already added the fresh-Claude fallback and the "flag same-family-only" instruction — *credit
given*. What remains open: the verification *state* is narrated per-finding in free text
(*"self-verified... Cross-family: pending"*), not a structured field, so a reader must scan
prose to learn which Highs are citable, and nothing mechanically blocks a "pending" finding
from being quoted.

**Evidence (quote → flaw).** Every High in the synthesis ends with an ad-hoc prose line —
*"self-verified (`cat .claude-plugin/plugin.json`). **Cross-family: pending.**"* (H1);
*"**Cross-family: pending.**"* (H2, H3, M1, M2, M3). The state is real and honestly recorded
(W3), but it lives as a repeated sentence fragment rather than a field the report contract
defines — the same lossy, hand-maintained shape that PA-0001.3 F5 flags *in the product's own
report*. The audit under-applies to itself the structured-verification discipline it demands
of ghx. Note also the run's fallback was *not run* (the fallback didn't exist yet at 09:33) —
which is why the skill added it at 10:13; the remaining risk is that the next run *also* just
records the gap, because "record who judged it" reads as satisfiable by a prose sentence.

**Specific skill change.** Output contract (SKILL.md L188-189): replace the prose "Who judged
it & how to check" bullet's *form* with a required structured tag **`Verification:
{self-derived | same-family-adjudicated | cross-family-adjudicated} — recompute: ⟨command⟩`**,
and add to the Executive-summary contract: *"No finding tagged `self-derived` or
`same-family-adjudicated` may be cited in a milestone go/no-go; the report states this gate at
the top."* This makes W3's blocking state a mechanical gate, not a promise.

**Anti-bloat.** Replaces an existing free-text bullet with a three-value enum + a command;
net neutral on length, strictly positive on scannability and on preventing a "pending"
finding from leaking into a decision. MUST-FIX because it is the enforcement half of a
half-built control (the fallback exists; the gate doesn't).

---

### PF4 — Doc-reading vs live-behavior imbalance: an agent-first audit that surfaced its Highs almost entirely from reading files. **[SHOULD · high confidence]**

**Failure pattern.** Under-weighting behavioral/eval evidence for a product whose entire
thesis is the agent's runtime experience. The skill's own Phase-6 weak-driver playbook is
declared *"load-bearing for ghx"* — and the run ran none of it.

**Evidence (quote → flaw).** "What was not audited": *"no multi-task Phase-6 **weak-driver AX
sweep** (only two one-shot live `ask`s, by AX and Competitive)."* Every High (H1 default
surface, H2 narrative, H3 moat) is derived from reading `plugin.json`, the corpus doc, and
NORTH_STAR. The two live runs were single-shot happy-path `sidecar ask`s used almost entirely
as *nulls* ("the recon brain works," W6), not to surface findings. The one behavioral finding,
M2 (silent-success exit 0), came from a one-line CLI probe — and its author flagged in the
least-confident call that resolving even its *severity* *"needs production/eval trace
evidence... which I could not see from this surface pass."* The audit repeatedly hit "I'd need
real traces" (PA-0001.1 F2 and its nulls; PA-0001.3 F3 recommends *building* a selection eval
because none exists to read) and never crossed it — despite `~/.ghx/sessions/` and `docs/evals/`
being listed surfaces with live trajectories on disk.

**Specific skill change.** Process step 2 (SKILL.md L112-114): add a surface-quota clause —
*"For any milestone-grade audit of an agent-first product, **at least one persona's primary
surface must be live-behavior or trace evidence** (a Phase-6 weak-driver multi-task run, or
mining committed `~/.ghx/sessions/` + `docs/evals/` trajectories) — not documents. An audit
of the agent's experience that reads only files has not audited the agent's experience."*

**Anti-bloat.** One clause on the scope step, reusing the Phase-6 playbook the skill already
carries (no new machinery). It earns its space because the skill's differentiator *is*
agent-first behavioral evidence; a doc-only live run quietly inverts the skill's own thesis,
and W6 shows the payoff when behavior is actually exercised. SHOULD not MUST only because a
text-only single pass has real limits the honest-close already flags — the fix is to
pre-commit the surface at scope time, not to demand a full harness.

---

### PF5 — No coverage matrix at scope time: the skew (measurement/strategy heavy; agent-safety / desire-path / monetization at zero) is only visible in the post-hoc afterthought. **[SHOULD · medium-high confidence]**

**Failure pattern.** Coverage gaps + false-consensus. Persona/scope selection front-loaded
measurement-trust and moat, three of five lenses converged on one finding-family, and the
un-probed axes surface only as a confession at the end — not as a visible choice at spawn.

**Evidence (quote → flaw).** "What was not audited": *"**agent-safety** (enumerated, not
exercised — no injection/lethal-trifecta probe), business-model/monetization... gap-analysis
via live desire-path trace mining."* The five personas (AX, AI-PM, Platform/Scale,
Competitive, North-Star Zealot) cluster on measurement + moat; **three** (AI-PM, North-Star
Zealot, and half of Platform/Scale) landed on the *same* narrative-vs-ledger overstatement
(PF2's cluster) — the direct cost of the skew. Meanwhile agent-safety is a Tier-B scope the
skill itself grounds in the *lethal trifecta* — acutely relevant for a product that surfaces
**untrusted repo content → the report → the main agent's next action** — and it got zero
probe. The synthesis's shared-prior catch (W2) fired for H2 but not for H3's Competitive↔Platform
convergence, because detection was by memory, not mechanism. Directive 6 (added by `c8d5eaa`)
now demands a focus and in-scope/out-of-scope — *credit* — but a flat in/out list is not a
*matrix*: it does not force each axis to be marked covered/deferred *with a reason*, which is
what would have made "we are not probing the injection surface this round" a deliberate,
visible decision instead of an afterthought.

**Specific skill change.** Process step 2 (SKILL.md L112-114) + the frontmatter contract
(L174-178): require a **coverage matrix** — every scope axis (Tier A + Tier B) marked
`covered | deferred ⟨one-line reason⟩` *before* spawning, recorded in frontmatter. Add to the
persona-agent delegation prompt (L144-163): each persona **declares its assigned source set**;
at synthesis, any finding citing the same source across personas is auto-flagged shared-prior
(mechanizing W2 beyond memory).

**Anti-bloat.** Upgrades an existing frontmatter field (in/out-of-scope) to a matrix with
reasons, plus one line in the delegation template. Small, and it converts the honest-close
from a confession into a pre-registered plan — the same "pre-register, then measure"
discipline ghx demands of its evals. SHOULD.

---

### PF6 — Findings are severity-ranked but carry no fix-cost / reversibility axis, so the report doesn't say what to do *first by leverage*. **[SHOULD, borderline OPTIONAL · this is my least-confident call]**

**Failure pattern.** Severity vs actionability. Ranking by L×I + north-star leverage tells you
what's *scary*, not what's *worth doing Monday*. Several PA-0001 fixes are near-free narrative
edits; others are real engineering — and nothing structurally separates them.

**Evidence (quote → flaw).** The synthesis *notices* the split in prose — *"The fix for most
High findings is narrative honesty + finishing already-planned measurement... not new
machinery"* — but never structures it. H2, L1, L2, and the M8 finding are all one NORTH_STAR
honesty-pass away from resolution; H1 (re-plumb the default adoption surface), M1 (restructure
MCP output), and M3 (version the report contract) are genuine workstream builds. A reader
cannot see from the ranking that batching *all* the narrative edits into one doc pass is the
cheapest high-value move; the Executive summary elevates H1 (a build) as "highest-leverage" on
severity grounds while the cheapest wins sit unbatched across the list.

**Specific skill change.** Output contract (SKILL.md L181-193): add a light tag **`Cost:
{doc-edit | scoped-fix | workstream | new-ADR}`** (reversibility folded in only where it
matters), and one line to the Executive-summary contract: *"State a **do-first-by-leverage**
order — highest impact × lowest cost first — distinct from the severity ranking."*

**Anti-bloat — and why this is my least-confident call.** This is the change most at risk of
*failing* the anti-bloat bar: a cost/ROI/reversibility scoreboard is precisely the kind of
human-PM-tracker ceremony R4-F4 warns against cargo-culting onto an agent-native op, and
disposition already routes to workstreams (a real action currency). If it hardens into a
four-column rubric every finding must fill, it is bloat. I propose it as a **single enum + one
summary sentence** and hold it at low confidence: the *"do-first order"* line is clearly worth
it; the per-finding `Cost` tag is borderline and a reasonable reviewer could drop it as
ritual. I would ship the summary line and pilot the tag.

---

## Anti-bloat ledger (net budget)

| Finding | Net change to the skill | Net change to the *report* |
|---|---|---|
| PF1 | +2 sentences (step 5) + tighten 1 bullet | none |
| PF2 | +1 tag + 1 sentence (step 5) | **−~50%** (collapses restated findings) |
| PF3 | replace 1 prose bullet with an enum+gate | neutral, +scannability |
| PF4 | +1 clause (step 2) | may add 1 behavioral persona |
| PF5 | upgrade 1 frontmatter field + 1 template line | +1 matrix, −afterthought |
| PF6 | +1 enum + 1 summary line | +1 ordering |

Net skill growth is small and concentrated in the load-bearing steps; PF2 makes the *product
of the skill* materially smaller, which is the strongest anti-bloat argument available.

## Least-confident call

**PF6's per-finding `Cost` tag.** The *do-first-by-leverage* ordering is clearly right, but a
cost/reversibility axis is the one proposal here that flirts with R4-F4's "human-team ritual
transposition" anti-pattern and R4-F7's context-tax warning. It could be genuine leverage or
it could be ceremony; I would ship the one-line ordering and treat the tag as a pilot to kill
if it reads as busywork. (Runner-up uncertainty: PF4's severity — whether a text-only single
pass can be *required* to carry a behavioral primary surface, or whether that quota is only
honest at milestone grade.)

## Coda — what R4 got right, and what only the live run showed

R4-F1 predicted a *rubber-stamp*; the run instead shows the judge **re-derived and
mis-adjudicated** (PF1) — a failure that reads as *more* rigorous, not less, and is therefore
harder to catch. R4-F3 (convergence-is-not-corroboration) and R4-F6 (persona nulls) were
*implemented and worked* (W2, W4) — the skill's static reviews paid off. The two failures a
static review structurally could not surface are both here: (1) the judge's own recompute can
drift (you need a real re-derivation to see it go wrong), and (2) restating the target's own
committed ledgers only becomes visible once you point the audit at a product mature enough to
*have* honest ledgers — which is why PF2, not PF1, is the highest-leverage change for a repo
whose founder's standing instruction is "stop restating what we already wrote."
