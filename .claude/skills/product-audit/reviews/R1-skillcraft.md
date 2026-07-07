# R1 — Skill-Craft & Operational Usability Review

**Target:** `/Users/goga/Documents/goga/ghx/.claude/skills/product-audit/SKILL.md`
**Scope of this review:** is this an *effective SKILL.md* that a fresh orchestrator (Fable) can execute to produce a good product audit? NOT the PM theory (another reviewer owns that). Judged against Anthropic's current skill-authoring guidance (primary sources in the Source Ledger).
**Reviewer stance:** adversarial, DH5/DH6 bar — every finding quotes the target line and names the specific flaw. Equal-rigor "what it gets right" section included so the criticisms carry weight.

---

## Abstract

The draft is a **genuinely good skill by the letter of Anthropic's guidance on the axes that are easy to get wrong** — the description is textbook (third person, specific, trigger-rich, 597/1024 chars, carries a de-triggering negative boundary), progressive disclosure is done correctly (thin SKILL.md → one-level-deep `research/*.md`), it provides defaults instead of option-paralysis, and it explains the *why*. It is 350 lines, inside the 500-line rule.

The problems are not "too long" — they are **placement and self-consistency**. The single highest-leverage defect is structural: the two most action-critical sections (**Process** = how to run, **Output contract** = what to deliver) sit at the very bottom, beginning ~5.1k and ~5.5k tokens into a ~6.2k-token skill. Claude Code keeps only "the first 5,000 tokens of each" skill after auto-compaction — and a multi-agent audit is exactly the kind of long session that *will* compact. So the run-spine is the part most likely to be silently dropped mid-audit, leaving 5,000 tokens of catalog and doctrine. That is a one-move fix (reorder) with outsized payoff. Secondary: the `## Anti-patterns` section largely restates Prime Directive 2 and is explicitly copied "From research/06 §5" — a context tax a skill preaching "smallest set of high-signal tokens" should not levy on itself.

Net: strong bones, one important reorder, a handful of executability nice-to-haves. No charter problems — the scope fence is clean and I recommend keeping it.

---

## What it gets RIGHT (equal rigor)

1. **The `description` is a model example.** `Use when asked to audit the ghx product from a product-manager's perspective — to find issues, gaps, north-star misalignment, PMF / market / competitive questions … This is a PRODUCT/strategy audit, not a code-quality or security audit.` It is third person (the doc's hard rule: *"Always write in third person"*), leads with *when to use*, is dense with trigger terms, and — crucially — ends with an explicit negative boundary that de-triggers the adjacent code/security audits in this repo. 597 chars, well under the 1024 API cap (and the 1,536 Claude-Code `description`+`when_to_use` cap). This is exactly the "specific + when-to-use + key terms" shape the docs prescribe [S1, S3].

2. **Progressive disclosure is correct, not cargo-culted.** SKILL.md is the "table of contents"; the six `research/*.md` hold the sourced detail, and every reference is **one level deep** from SKILL.md (`research/02-pm-personalities.md`, etc.). That is precisely the doc's Pattern 1 and its "Keep references one level deep from SKILL.md" rule [S1]. The 16-persona table keeps a one-line signature question per row (high-signal, role-play-ready) while punting the full persona blocks to `research/02` — the right load-bearing/reference split.

3. **It provides defaults instead of options.** `General-purpose starter set (5): Feature-Factory Skeptic + User-Empath + Technical-Debt Realist + Platform/Scale Realist + Competitive-Paranoid …` and the `★ = near-mandatory` markers. This is the doc's "Provide a default (with escape hatch)" anti-pattern-avoidance done well [S1] — a fresh orchestrator is never left staring at 16 personas with no guidance.

4. **Appropriate degrees of freedom.** An audit is the doc's "open field with no hazards" case — many valid paths — and the skill correctly uses high-freedom prose instructions (numbered Process, heuristic pick-rules) rather than brittle low-freedom scripts [S1]. It does *not* over-specify.

5. **Explain-the-why throughout.** e.g. Prime Directive 2's `Engagement/Retention as a goal (INVERTED — for a context-subtracting product, more tool-calls/time-per-task is a vanity metric …)`. This is the doc's endorsed "state the rule, then explain why so Claude can generalize" pattern [S1, S2].

6. **Concrete, reusable output scaffolding.** The `## Output / evidence contract`, the 3×3 severity matrix, and the per-finding evidence requirements are a real template a worker can fill — matching the doc's "Template pattern (flexible guidance)" [S1].

7. **Clean scope fence.** `## In scope / out of scope` explicitly routes code/security/eval-mechanics findings to their existing homes. This both prevents charter creep and *reinforces the description's de-trigger* — good skill hygiene.

---

## Ranked findings

### F1 — The action spine (Process + Output contract) is at the bottom, past the 5,000-token post-compaction retention floor — MUST-FIX (confidence: high on ordering; med on exact token boundary)

**Quoted target:** `## Process (how the orchestrator runs an audit)` begins at ~byte 20,550 of a 24,814-byte file, and `## Output / evidence contract` at ~byte 22,154. By a chars/4 estimate that is **~5,138 and ~5,538 tokens into a ~6,200-token skill** — i.e. both fall in the last ~1,000 tokens.

**Flaw (DH6, addresses the load-bearing structural choice):** Claude Code's own docs state that after auto-compaction it "re-attaches the most recent invocation of each skill after the summary, **keeping the first 5,000 tokens of each**. Re-attached skills share a combined budget of 25,000 tokens" [S3]. A product audit that spawns background persona-agents, cross-validates, and synthesizes is precisely a long session that *will* hit auto-compaction. When it does, the 5,000-token window retains the intro, Prime Directives, the persona catalog, the scope catalog, and Adversarial discipline — and **drops the two sections that tell the orchestrator how to actually run the audit and what to deliver.** The most executable content is the most disposable under the exact mechanic that governs long tasks. Separately, even pre-compaction, ordering "what to know" (≈230 lines of catalog) ahead of "what to do" makes the spine hard to find on a partial/`head` read, which the docs warn about [S1, "Claude might use commands like `head -100` to preview"].

**Why it matters + source:** Skill content "enters the conversation as a single message and stays there," is not re-read on later turns, and is truncated to the first 5k tokens on compaction [S3]. The skill's whole value proposition is being executable across a long delegated audit — the one regime where this ordering fails.

**Recommendation:** Move `## Process` and `## Output / evidence contract` to **immediately after Prime Directives** (i.e. into the first ~2,500 tokens), and demote the two big catalogs (`## Personalities`, `## Scopes`) to *after* the spine — or push the full tables into `research/` and leave only the pick-rules + starter set inline. The run-spine and deliverable contract must live inside the 5,000-token floor. This is one reorder, no content loss. **MUST-FIX.**
*(Confidence note: the chars/4 token estimate is approximate; real tokenization could shift the boundary ±10-15%. But Process at ~5.1k and Output at ~5.5k are at or over the line either way, so the risk stands.)*

---

### F2 — `## Anti-patterns` duplicates Prime Directive 2 and is explicitly lifted "From research/06 §5" — the skill taxes its own token budget it tells others to guard — SHOULD (confidence: med-high)

**Quoted target:** `## Anti-patterns to WARN against (agentic-first cargo-culting)` opens `From research/06 §5.` and then lists e.g. `Vanity human funnels as success — stars, installs, DAU, and especially session-length / tool-calls-per-task` and `Tool-count / feature-count as progress`. Prime Directive 2 already carries `Engagement/Retention as a goal (INVERTED … more tool-calls/time-per-task is a vanity metric)`, and Tier B already has the token-economics/tool-selection warnings.

**Flaw (DH5):** roughly 20 lines restate material that is (a) already in Prime Directive 2's OUTDATED-OR-INVERTED list and the Tier-B scopes, and (b) by the section's own admission already fully written up in `research/06 §5`. The skill's stated ethos — its intro demands `evidence, not vibes` and the north-star's `zero doctrine` — is undercut by inlining a doctrine catalog that both repeats itself and pointer-duplicates a research file. This is exactly the doc's "Does this paragraph justify its token cost?" test failing [S1, "Concise is key … The context window is a public good"]. It is also the *self-consistency* charge the review was asked to press: the skill preaches smallest-high-signal-token-set and doesn't fully practice it here.

**Why it matters + source:** every token in SKILL.md "competes with conversation history and other context" once loaded, and once loaded it *persists for the whole session* [S1, S3]. A ~20-line section that is 70% redundant is ~350 tokens of standing tax on every audit.

**Recommendation:** Collapse `## Anti-patterns` to a 4-6 line pointer — the *distinct* warnings not already in Prime Directive 2 (e.g. "human-pretty output," "delight theater," "surveying the agent"), plus `full list: research/06 §5`. Do **not** delete the concept — flagging cargo-cult is core — just stop restating it three places. **SHOULD.**

---

### F3 — No copyable checklist for a "particularly complex workflow" the docs say should have one — SHOULD (confidence: med)

**Quoted target:** `## Process (how the orchestrator runs an audit)` is seven prose-numbered steps (`1. Ground. … 7. Iterate if warranted.`).

**Flaw (DH5):** the audit is a multi-round, multi-agent, cross-validate-then-synthesize workflow — the textbook case the docs single out: *"For particularly complex workflows, provide a checklist that Claude can copy into its response and check off as it progresses"* [S1]. Prose steps are followable, but across a long session with fan-out to persona-agents and a compaction event (see F1), a copyable checklist is what keeps the orchestrator from losing its place ("did I run the cross-validate step on agent 4's findings?").

**Why it matters + source:** the docs give this as a named pattern specifically because "Clear steps prevent Claude from skipping critical validation" [S1] — and this skill's cross-validate step (F4's judge gate) is exactly a step an orchestrator can skip under context pressure.

**Recommendation:** Add a fenced, copyable progress checklist at the top of `## Process` (Ground → Scope → Delegate N persona-agents → Cross-validate each returned finding → Rank/synthesize → Honest close). Pairs naturally with moving Process up (F1). **SHOULD.**

---

### F4 — The skill's core mechanic — spawning persona-agents — ships without a delegation prompt template — SHOULD (confidence: med)

**Quoted target:** step 3: `Delegate breadth. Spawn background persona-agents … Give each: the north star, its persona block, the scopes/surfaces, the agentic-first verdicts, and the sourcing discipline … Pin them to text-only tools.`

**Flaw (DH5):** the list of *what to hand each agent* is right, but there is no copyable prompt block — so a fresh orchestrator must re-author the persona-agent prompt from scratch every audit. The sibling skill sets the bar: `fable-delegation` ships concrete "Codex Implementation Prompt" and "Claude Wrapper Prompt" blocks precisely so the orchestrator doesn't re-derive them. A skill whose central method *is* delegation should provide the delegation prompt, the same way [house style: `fable-delegation/SKILL.md` "Codex Implementation Prompt"]. The inputs are already enumerated in step 3 — this is a formatting-into-a-reusable-block task, not new content.

**Why it matters + source:** the docs' "Examples pattern" and "Template pattern" both exist because "Examples help Claude understand the desired … level of detail more clearly than descriptions alone" [S1]. A persona-agent prompt template is that example.

**Recommendation:** Add a fenced "Persona-agent prompt" template near step 3 (persona block + north star + scopes/surfaces + agentic-first verdicts + sourcing discipline + "text-only tools; do NOT use browser automation/screenshots" + the per-agent output contract). Reference `fable-delegation` for spawn mechanics as it already does (line 305). **SHOULD.**

---

### F5 — Terminology drift: "personalities" / "personas" / "lenses" for one concept — OPTIONAL (confidence: low-med)

**Quoted target:** header `## Personalities (the audit lenses)`; the model table axis `**Personalities**`; the table column `Persona`; composition rules `persona set`, `personas`; and `Sixteen role-playable lenses`. Three near-synonyms for the same object.

**Flaw (DH4/DH5):** the docs are explicit: *"Choose one term and use it throughout the Skill … Consistency helps Claude understand and follow instructions"* [S1]. Here a model could reasonably wonder whether "personality," "persona," and "lens" are three things or one, especially when the frontmatter says "personalities" but the process says "persona set."

**Why it matters + source:** minor, but it's a free fix that the doc names directly [S1], and it slightly muddies the axis vocabulary the whole method rests on.

**Recommendation:** Standardize on **persona** (matches the table column and `research/02-pm-personalities.md`); keep "lens" only as an occasional gloss if desired. **OPTIONAL.**

---

### F6 — Motif repetition (judge / evidence / "what is actually fine") — OPTIONAL, low-touch (confidence: low)

**Quoted target:** the "What is actually fine" honesty-guard rule appears in Prime Directive 4, `## Adversarial discipline` ("Honesty guard: keep a **'What is actually fine'** section"), Process step 5, and the Output contract — 4×. "The orchestrator is the judge / breadth delegated, judgment is not" recurs ~5×.

**Flaw (DH4):** some of this is *deliberate* reinforcement, which the docs endorse ("make rules more prominent," "stronger language") — so this is a soft finding, not a mandate. But 4-5 restatements of the same two motifs is past reinforcement into tax.

**Why it matters + source:** balanced against [S1]'s own advice to make load-bearing rules prominent — hence low severity. Trim only the weakest one or two repeats; do not mechanically dedupe.

**Recommendation:** State the honesty-guard once in Prime Directives and once in the Output contract (where it's operational); drop the mid-document repeats. **OPTIONAL.**

---

## Resist these temptations (cargo-cult guardrails)

- **Do NOT broaden the charter.** The `In scope / out of scope` fence (code/security/eval-mechanics → their own homes) is correct and load-bearing for de-triggering. Any advice to "also cover code quality / add a security lens" is scope creep — reject it. (Confirmed: nothing in this review asks for that.)
- **Do NOT inline the full 16 persona blocks or the full frameworks "to make SKILL.md self-contained."** That would blow the token budget and defeat progressive disclosure; the `research/` split is exactly right [S1].
- **Do NOT mechanically dedupe every repeated phrase.** The docs explicitly favor making load-bearing rules prominent [S1]. Only F2 (Anti-patterns↔PD2) and the weakest F6 repeats are worth touching.
- **Do NOT reflexively set `disable-model-invocation: true` "because audits are expensive."** Auto-invocation only *loads the ~6.2k-token body*; it does **not** auto-spawn a persona fleet — the Process still requires the orchestrator to execute steps deliberately [S3]. The cost of an accidental trigger is bounded to a context load, and CLAUDE.md wants Fable to reach this skill self-directed. Not worth the friction.
- **Do NOT pad the `description` with more trigger words.** It is already comprehensive and its negative boundary is doing real de-trigger work in an audit-heavy repo; more terms raise over-trigger risk for marginal recall.

---

## Source Ledger

All three primary sources fetched and verified this session (July 2026). I deliberately leaned on Anthropic primary sources over blog aggregators.

| # | Source | URL | Status | Why authoritative / what it grounds |
|---|---|---|---|---|
| S1 | "Skill authoring best practices" — Claude Platform Docs | https://platform.claude.com/docs/en/agents-and-tools/agent-skills/best-practices | **VERIFIED** (full page fetched). Note: `https://docs.claude.com/en/docs/agents-and-tools/agent-skills/best-practices` **302-redirects here** (verified). | Anthropic's canonical authoring guide. Grounds: "Concise is key / context window is a public good," 500-line body rule, third-person description rule + 1024-char cap, "provide a default (with escape hatch)," degrees-of-freedom / open-field analogy, one-level-deep references, "consistent terminology," "checklist for complex workflows," Template/Examples patterns, "does this paragraph justify its token cost?" |
| S2 | "Equipping agents for the real world with Agent Skills" — Anthropic Engineering | https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills | **VERIFIED** (fetched). | Anthropic engineering post. Grounds: three-level progressive disclosure (metadata → lean SKILL.md → bundled resources), "table of contents" mental model, keep SKILL.md lean, name+description as the trigger signal, explain-the-why. |
| S3 | "Extend Claude with skills" — Claude Code Docs | https://code.claude.com/docs/en/skills | **VERIFIED** (full page fetched). | Claude Code-specific mechanics. Grounds F1 directly: skill content "enters the conversation as a single message and stays there," is not re-read on later turns, and after auto-compaction Claude Code keeps "the first 5,000 tokens of each" skill (25k combined budget). Also: frontmatter reference (`when_to_use` 1,536-char cap, `allowed-tools`, `disable-model-invocation`, `user-invocable`, `context: fork`), "Keep SKILL.md under 500 lines." |

Measurements in this review (F1) are from direct byte-offset inspection of the target file (`wc`, Python byte-offset probe); token figures are chars/4 estimates and are labeled as approximate where load-bearing.
