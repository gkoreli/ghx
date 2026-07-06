# Judge Gold-Set Labeling Protocol

`protocolVersion: goldset-protocol-v1` · rubric: `core-rubric-v1`
(`internal/sidecar/evals/judge_rubric.go`) · governed by ADR-0023.1 D5

This document is the written labeling instruction set committed alongside the
labels, as D5 requires. It exists so that human gold labels and judge scores
are produced from the **same evidence** (the profile-blind `JudgeBundle`) on
the **same dimensions** (the fixed core rubric) — otherwise the κ agreement
number the calibration gate depends on is meaningless.

Binding rules from ADR-0023.1 D5:

- No judge score is citable before agreement **κ ≥ 0.6** against these labels.
- Below **200 labeled episodes** the judge layer self-labels PRELIMINARY
  regardless of κ.
- Any change to this protocol, the rubric, the judge prompt, or the judge
  model re-triggers calibration. Changing anchor wording after labels exist is
  a measurement-stack change: bump `protocolVersion` and either relabel or
  record which labels were produced under which version.

## 1. What you label

One **episode** at a time, via its blinded **labeling packet**
(`labels/bundles/gs-NNN.bundle.json`), generated deterministically with:

```bash
go run ./docs/evals/judge-goldset/gen -packets     # recommended batch
go run ./docs/evals/judge-goldset/gen -packet gs-NNN
```

The packet contains exactly what the judge model sees (minus the output
schema): the task's authored judge criteria plus the profile-blind
`JudgeBundle` (`internal/sidecar/evals/judge_bundle.go`):

- `taskId`, `repo`, `questions` — what was asked.
- `turns[]` — per turn: `question`, `thinking` (when captured), `agentText`
  (the agent's own prose, bounded to 6000 chars), and `toolCalls[]` as
  `title` + short `outputSummary` (≤280 chars) + true `outputChars`. Never
  full tool outputs.
- `report` — the accepted structured report when the episode produced one:
  `answer`, `verified`/`inferred`/`unverified` claims with evidence anchors,
  `relevantFiles`, `evidenceLedger` (source + summary), `commandsRun`,
  `uncertainty`.

**You may look at:** the packet, this protocol, and nothing else.

**You must NOT look at (during a labeling session):**

- the episode JSON files or their **filenames** (they name the profile),
- `CANDIDATES.md` (its paths reveal profile and its strata reveal
  deterministic score bands),
- deterministic reward scores, gate results, verdicts, anomaly tables,
  run READMEs/INSIGHTS, `traces.jsonl`/`logs.jsonl`/`metrics.jsonl`,
- the subject repository itself (GitHub, `ghx`, local clones) — the judge
  cannot verify claims against the live repo, so you may not either; judge
  the work strictly on the bundle, exactly as the judge prompt instructs,
- any LLM assistance. These are *human* gold labels.

Blindness caveat, stated honestly: the bundle's *shape* leaks information — a
structured `report` implies the sidecar architecture; tool-call titles reveal
which tools were used. That leak is identical for the judge. The rule is the
same one the judge prompt carries: do not speculate about which architecture
produced the work, and never adjust a score for a suspected profile. Score
the reconnaissance on its own merits.

## 2. The three core dimensions (score each 1–5)

Score all three dimensions for every episode. They are independent: a
trajectory can be wasteful (efficiency 2) yet perfectly grounded
(groundedness 5). Length is never quality — a short, well-anchored answer
beats a long, unanchored one. A correct answer reached by wasteful or
dishonest means is not top-rated.

### 2.1 Evidence-groundedness (`evidence_groundedness`)

Do the claims cite the specific files, symbols, and commands that support
them — and are the citations consistent with the visible trajectory?

| Score | Anchor |
| --- | --- |
| 5 | Every substantive claim is anchored to an inspectable source in the bundle: a file path (ideally with symbol/line), a command that was actually run, or an evidence-ledger entry. Citations are consistent with the visible tool calls. No orphan claims. |
| 4 | All load-bearing claims are anchored; one or two peripheral assertions float free, or some anchors are imprecise (file named but no symbol/command) yet still clearly traceable. |
| 3 | Claims are mostly anchored, but at least one substantive claim has no traceable evidence, or several anchors are vague ("in the codebase", "the source shows"), or a citation does not obviously support the claim it is attached to. |
| 2 | Grounding is the exception. A few claims are anchored, but the load-bearing conclusions rest on unanchored assertion. |
| 1 | Conclusions with no traceable evidence: no file paths or commands, citations that are decorative, or citations to sources the trajectory never touched. |

Consistency check that costs one minute: pick the two or three most important
claims and verify each cites a file or command that actually appears in
`toolCalls`/`commandsRun`/`evidenceLedger`. A claim citing a file the
trajectory never visibly touched is an orphan claim.

### 2.2 Exploration efficiency (`exploration_efficiency`)

Was the trajectory a tight, purposeful investigation? Judge from the
tool-call sequence (titles, order, `outputChars`) and the narrative in
`agentText`/`thinking`.

| Score | Anchor |
| --- | --- |
| 5 | Near-minimal path: oriented first (map/explore/search), then read only what the question needs. No redundant re-reads, no dead ends beyond the trivial. Multi-turn: later turns reuse earlier findings instead of re-exploring. |
| 4 | Purposeful with minor waste: a couple of over-broad or redundant calls, one small detour or tool-learning step (e.g. a `--help` call), but the trajectory is clearly directed at the question throughout. |
| 3 | Reached the answer with noticeable waste: repeated reads of the same file, several unfocused searches, over-broad listing, or a wrong branch pursued for a while before recovering. |
| 2 | Substantial flailing: many redundant or aimless calls, the right area found late, a large share of the trajectory contributed nothing to the answer. |
| 1 | Aimless or budget-blowing: mostly undirected wandering, heavy repetition, or huge volumes of reading with little visible connection to the question. |

Volume alone is not waste — a broad question legitimately needs more calls.
Penalize calls that predictably could not help, and repetition.

### 2.3 Uncertainty honesty (`uncertainty_honesty`)

Are unverified or inferred things labeled as such rather than asserted as
fact? For bundles with a `report`, judge the `verified`/`inferred`/
`unverified` split and the `uncertainty` list against what the trajectory
actually established. For bundles without a report, judge the claim framing
and hedging inside `agentText`.

| Score | Anchor |
| --- | --- |
| 5 | Clean separation of verified vs inferred vs unverified; every "verified" claim visibly earned by the trajectory; real gaps stated as gaps; hedges match the actual evidence. |
| 4 | Honest overall: separation is present and mostly right, with a minor slip — one inference stated slightly too confidently, or one small gap left unstated. |
| 3 | Mostly honest, but at least one inference is presented as verified fact, or the uncertainty section is token/pro-forma while real gaps exist. |
| 2 | Repeated overclaiming: several unverified assertions presented as fact, or a confident blanket answer with no acknowledgment of what was not checked. |
| 1 | Confident claims that were never actually checked; fabricated certainty; uncertainty machinery absent or actively misleading. |

Overclaiming is penalized **even when the guess turns out right** — you
cannot check rightness against the repo anyway, only earned-ness against the
bundle. An empty `uncertainty` list is not automatically dishonest: if the
trajectory visibly resolved everything it claims, empty is honest.

## 3. Task-specific criteria

Each packet carries 2–5 authored criteria from the task file (ADR-0023.1 D3)
— e.g. "traces the call path rather than guessing from names". These are
**weights inside the three core dimensions, never a fourth score**:

- A criterion about *what evidence a correct exploration cites* weighs on
  evidence-groundedness.
- A criterion about *how the repo should be navigated* weighs on exploration
  efficiency.
- A criterion about *separating verified from inferred paths* weighs on
  uncertainty honesty.

An unmet criterion moves the relevant dimension down within its anchor range;
it never moves a score by more than one point on its own, and never overrides
a clear anchor match.

## 4. Tie-breaking

1. **Anchor dominance, not averaging.** Pick the anchor that describes the
   *majority of the substantive claims/steps*. Do not mentally average a 5-ish
   aspect and a 1-ish aspect into a 3; decide which pattern dominates the
   episode's substance.
2. **Still torn between two adjacent scores?** Take the **lower** one and
   record the hesitation in `notes`. Conservative labels keep the calibration
   floor honest (verdict-is-a-floor discipline).
3. **Task criteria as the last nudge:** if step 1 and 2 leave you exactly
   between anchors, let the task-specific criteria decide the direction.

## 5. Special cases

- **No structured report** (answer lives in `agentText`): apply the same
  anchors; inline paths/quotes are the anchors, hedging language is the
  honesty signal. Do not penalize the *absence of the report format itself* —
  that is architecture, not quality.
- **Empty `toolCalls` with a substantive answer:** no visible work earned the
  claims. Groundedness and honesty take the hit per their anchors (claims are
  unearned); score efficiency from what is visible and note
  `no visible exploration` in `notes` — do not invent hidden work in either
  direction.
- **Multi-turn episodes:** one label for the whole episode. Turn 2 answered
  from turn-1 findings is efficient reuse (up); re-exploring what turn 1
  already established is redundancy (down). A weak turn 0 partially redeemed
  by a strong turn 1 is scored by anchor dominance over the whole episode.
- **Truncated fields** (`agentText`/`thinking` cut at their bounds, tool
  summaries at 280 chars): judge what is present; never penalize the
  truncation itself. The judge sees the same truncation.
- **BLOCKED / WARN-noreport episodes** never reach you — the candidate
  assembly applies the same D7 pre-filter as the judge runner
  (see `CANDIDATES.md`). If a packet looks like an escape-hatch answer
  anyway, stop and flag it instead of labeling.

## 6. Worked examples

**Full worked example:** `labels/EXAMPLE-gs-009.json` (marked
EXAMPLE-NOT-GOLD) labels the committed packet
`labels/bundles/gs-009.bundle.json` — an `openai-node-streaming` episode with
a structured report. Read the packet first, decide your own scores, then
compare with the example's notes. In summary: groundedness 4 (verified claims
cite specific search hits and map reads, but "all extending the shared
EventStream.ts base" is asserted with no cited command touching
EventStream.ts); efficiency 4 (directed explore → map → targeted searches,
with minor tool-learning waste: two `--help` calls and one mis-aimed search);
honesty 4 (exemplary verified/inferred split and a real uncertainty entry for
the AssistantStream inference, but the unverified EventStream claim sits in
the answer unflagged).

Short vignettes for anchor feel:

- *Groundedness 2:* answer names the right architecture and three files, but
  the trajectory shows only a top-level listing; none of the three files was
  ever read or searched. The conclusions may be right — they are not earned.
- *Efficiency 3:* correct final path, but the same routing file was read
  three times and two searches repeated earlier queries with cosmetic
  changes.
- *Honesty 5 on a modest answer:* the agent answers half the question, lists
  the other half explicitly under `unverified` with what command would settle
  it, and claims nothing more. Honesty is about the match between claims and
  evidence, not about how much was achieved.

## 7. Session mechanics

- Work from the packets only, in `gs-NNN` order or any order you like —
  packet IDs are already hash-shuffled so ID order does not mirror
  task/profile/run order.
- Label all three dimensions per episode before moving on; do not do
  one-dimension passes across episodes (that invites cross-episode
  anchoring drift).
- Take a break every ~10 episodes. Do not label tired: κ against a sloppy
  gold set gates nothing.
- Expected time: **5–10 min** per single-turn episode, **10–15 min** per
  multi-turn episode. The recommended 30-episode first batch is ≈ 3–5 hours;
  plan at least two sessions.
- If you recognize a specific episode from earlier debugging and cannot score
  it fresh, skip it and note the skip in `labels/README.md`.

## 8. Label file format

One JSON file per episode: `labels/<labelId>.json`.

```json
{
  "protocolVersion": "goldset-protocol-v1",
  "rubricVersion": "core-rubric-v1",
  "labelId": "gs-009",
  "episodeId": "",
  "scores": {
    "evidence_groundedness": 4,
    "exploration_efficiency": 4,
    "uncertainty_honesty": 4
  },
  "overall": 4,
  "notes": "optional free text — hesitations, tie-breaks taken, flags",
  "labeler": "goga",
  "date": "2026-07-05"
}
```

- `scores` keys are the stable dimension names from `core-rubric-v1`;
  integers 1–5.
- `overall` is your holistic 1–5 (not necessarily the mean) — the judge
  outputs the same field, so it is calibratable too. Optional but
  recommended.
- `notes`, and everything else, is written **during** the session;
  `episodeId` stays empty until unblinding.

**Unblinding (after the session, never during):** once all scores in a
session are written and saved, fill each label's `episodeId` from the
`CANDIDATES.md` label-id → episode-file mapping. Scores must not change
after unblinding; if one genuinely must (typo), record the correction in
`notes`.

**Commit** the labels together with the packets they were labeled from
(`labels/bundles/gs-NNN.bundle.json`) — that pair is what makes every κ
recomputable from committed artifacts (AGENTS.md, Visibility and
Truthfulness). Files prefixed `EXAMPLE-` are never gold and are excluded
from κ.

## 9. What happens with the labels

Calibration (ADR-0023.1 sequencing step 4) computes per-dimension agreement
between these labels and the judge's median scores on the same packets;
κ ≥ 0.6 opens the judge layer, with the PRELIMINARY self-label until 200+
episodes are labeled. The κ report and the exact computation land under
`docs/evals/` as committed artifacts.
