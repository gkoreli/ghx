---
name: skill-forge
description: >-
  Use when creating a NEW skill (or a substantial reference / strategy / design
  doc) that deserves real rigor — built via wide delegated research, an authored
  draft, and iterative adversarial review rounds, with the orchestrator as the
  authoritative final judge. This is the heavyweight path for artifacts that will
  be reused and must be high quality; do NOT use it for trivial or one-line
  skills, mechanical config, or a quick doc — just write those directly.
---

# Skill Forge — research-driven, adversarially-reviewed skill creation

A repeatable loop for producing a high-quality skill (or any load-bearing reference/strategy
doc) when the domain is ambiguous, knowledge-heavy, or benefits from many perspectives. The
orchestrator (Fable) **holds the goal, authors the artifact, and is the authoritative final
judge**; breadth — research and review — is delegated wide to background agents. Delegate
breadth, keep judgment; under-delegation wastes the plan, abdication wastes quality.

Worked example produced by this loop: `.claude/skills/product-audit/` (its `research/`,
`reviews/`, and commit history are the loop made concrete).

## When to use / when not

**Use** for a skill/doc that will be reused and must be right: a new capability the team
will lean on, a domain where conventional wisdom must be checked against *our* context, a
topic needing multiple expert lenses. **Don't** use it for a trivial skill, a one-liner, a
mechanical config change, or anything a single sharp pass already nails — the loop's cost
(several agent generations) only pays off when quality genuinely matters.

## The loop

| Round | Who | Output |
|---|---|---|
| **0 — Research (breadth)** | N background research agents, distinct angles | sourced `research/*.md` artifacts |
| **1 — Author the draft** | the orchestrator (authorship is *not* delegated) | `SKILL.md` draft + committed research provenance |
| **2 — Adversarial review (breadth)** | M background reviewers, distinct scopes | ranked `reviews/*.md` critiques |
| **3 — Reconcile (the judge)** | the orchestrator | accept/decline decisions + a revision |
| **↻ iterate** | — | another review round only if marginal value remains |
| **4 — Finalize** | the orchestrator | committed skill dir + provenance + a story in the log |

---

## Round 0 — Delegated research

- **Decompose the domain into ~5 distinct, non-overlapping angles**; one background agent per
  angle. Include **at least one adversarial/contrarian angle**, and — for any product/tech
  artifact — a **"does the conventional wisdom still hold in *our* context?"** lens. (In the
  worked example, that lens — "the consumer is an AI agent, not a human" — was the single
  most valuable artifact.)
- **Sourcing discipline is the biggest quality lever — enforce it verbatim on every agent:**
  every substantive claim carries a **specific deep URL** (the exact essay/doc/paper, never a
  homepage) **+ a one-line rationale** for why it is authoritative and why it matters here;
  verify links; **disclose degradations** (paywalls, 403s, unverified) instead of smoothing
  them over; **label the agent's own synthesis** as synthesis, not sourced fact; end with a
  **Source Ledger**.
- Each agent **writes an md artifact** to a known dir and returns an executive summary, source
  count, and honestly-flagged gaps. Pin **text-only tools** (WebSearch/WebFetch/curl/gh; no
  browser/screenshots) unless the task genuinely needs UI.
- **Model routing:** research/gathering can go to a cheaper-but-capable worker; keep synthesis
  in the orchestrator. (In this setup opus is both cheaper and smarter than sonnet — see
  `CLAUDE.md`. Never Haiku here.)
- **Commit the research artifacts as provenance** — the skill should ship with its evidence;
  every claim stays recomputable.

## Round 1 — Author the draft

- The **orchestrator reads the artifacts and writes the skill itself.** Do not delegate
  authorship — structure, altitude, taste, and the accept/reject of each research claim are
  the judgment this whole loop exists to protect. Read taste-critical artifacts in full.
- **Progressive disclosure:** a thin, operational `SKILL.md` + the research artifacts as
  committed provenance (their Source Ledgers are the citation of record). Don't inline 100
  URLs into the skill — point to the ledgers. The skill practicing what it preaches about
  token economy is itself a quality signal.
- **Front-load the run-spine.** Put "how to run it" and "what to deliver" near the top; a
  harness may truncate a long doc (Claude Code retains only the leading portion of a SKILL.md
  after compaction), and catalogs/reference material belong below the operational core.
- **Name in-scope / out-of-scope explicitly** and cross-reference the neighbours a reader
  might confuse this with. You will have reviewers attack this boundary in Round 2.

## Round 2 — Adversarial review

Spawn background reviewers on **distinct scopes** (in the worked example): (a) **craft/
usability of the artifact itself** — is it a good, runnable skill?; (b) **domain completeness
& currency** — what expert lens or framework is missing or now outdated?; (c) **fit to our
actual context/goal** — would running it surface the *right* things, or platitudes?; (d) a
**meta red-team / pre-mortem** — reconstruct how a faithful run still produces a worthless
result, then kill-the-artifact. Instruct every reviewer to:

- Clear the **DH5/DH6 bar** (Graham) — quote the exact passage, name the exact flaw; no tone-
  policing or vague doom.
- **Tag every recommendation MUST-FIX / SHOULD / OPTIONAL / REJECT-CANDIDATE + a confidence.**
- Keep an **honesty guard** ("what the artifact gets right", equal rigor) so criticisms carry
  weight, and **explicitly resist scope creep** — argue *against* adding, since more isn't
  free. Same specific-URL sourcing discipline as Round 0.

## Round 3 — Reconcile (the judge — the load-bearing round)

You are the authoritative judge, and a model adjudicating its own delegated work is a
**self-preference regime** (arXiv:2404.13076 — LLM evaluators favor their own generations).
"Trust nothing on assertion" is an instruction, not a mechanism, so:

- **Re-derive/verify load-bearing claims yourself before accepting** — check the cited harness
  rule, the filename, the competitor fact, the number. (In the worked example this caught a
  reviewer citing audit files whose exact paths had to be confirmed, and a claimed retention
  rule that shouldn't be leaned on without checking.)
- **Accept only what is verified and high-value. Decline** cargo-cult additions, changes the
  reviewer itself rated low-confidence, and scope creep — and **record what you declined and
  why.** Adding has a real cost (signal dilution); removals and "no" are valid outcomes.
- **Treat agreement among same-family reviewers as possible shared-prior, not independent
  corroboration.** For the highest-stakes decisions, get an **independent cross-family check**
  (a Codex/gpt-5.5 adjudicator via `fable-delegation`, or the human).
- **Revise, then commit** with a message that states what was accepted *and* declined.

## Iterate / finalize

- Run another review round **only if marginal value remains**; stop when reviews turn
  cosmetic. Diminishing returns are real.
- **Finalize:** the skill directory ships as `SKILL.md` + `research/` + `reviews/`, and the
  commit history tells the story (research → draft → review → reconcile). Commit provenance;
  don't leave it in scratch.

---

## Guardrails / lessons (from real runs)

- **Delegate breadth, keep judgment.** Research and review fan out; authorship and the
  accept/reject call stay in the orchestrator.
- **Sourcing discipline is the top quality lever** — specific URLs + rationale + verification
  + disclosed degradations + labelled synthesis. Enforce it on every delegated agent.
- **Commit artifacts as provenance;** the skill should ship with its evidence, every claim
  recomputable (matches `AGENTS.md` Evidence Contract & Visibility/Truthfulness).
- **Front-load the run-spine;** harnesses may truncate long docs.
- **Name in/out-of-scope,** and have a reviewer attack it.
- **Guard the judge** against self-preference; verify load-bearing claims; cross-family check
  the highest stakes.
- **Background-agent mechanics** (`fable-delegation`, `CLAUDE.md`): run long commands in the
  foreground with generous timeouts; never end a turn while a delegated turn is pending; if a
  worker ends prematurely, relaunch with a state-check of what it already produced.

Delegation command syntax lives in `.claude/skills/fable-delegation/SKILL.md`; this skill owns
the *loop*, that one owns the *mechanics*.
