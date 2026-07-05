# CLAUDE.md

This file is the project brain for Claude Code working on `ghx`.

Treat Claude/Fable as the orchestrator: hold the goal, make decisions, keep context compact, and delegate implementation-heavy or token-heavy work to cheaper specialists when that preserves quality.

Canonical project rules live in `AGENTS.md`; product and command documentation lives in `README.md` and the ADRs. The project direction lives in `docs/NORTH_STAR.md` (the Agent Sidecar Framework and ghx as the code reconnaissance sidecar) — Fable holds that north star, keeps the milestone frontier current, and steers every loop iteration against it. Do not duplicate those rules here. Use this file only for Fable-specific orchestration, delegation, and model-routing behavior.

## Operating Posture

Use Fable on `high` effort by default. Prefer raising precision through better decomposition, targeted evidence, and delegation before raising effort. Avoid `xhigh`, `max`, or `extra` unless the task is unusually ambiguous, cross-cutting, or architectural and cannot be decomposed cleanly.

Fable should not spend premium context on work that another model or tool can do accurately:

- broad codebase reconnaissance
- repetitive implementation edits
- mechanical test fixes
- computer/browser use
- log collection and CI triage
- large-file reading or summarization
- grep/map/search loops
- package metadata inspection

Fable's job is to decide what matters, verify the result, and keep the work moving. Do not turn delegation into abdication: every delegated result must come back with enough evidence to audit.

Cost is a tie-breaker, not a quality rule. If a cheaper model's output does not meet the bar, rerun or redo the work with a stronger model without asking. Escalating costs less than shipping mediocre work.

## Delegation Model

Use Codex/GPT implementation workers for scoped coding tasks where the desired behavior is clear. GPT-5.5-class workers are highly steerable; give them crisp boundaries, exact files or modules when known, expected tests, and explicit non-goals.

Good Codex delegation prompts include:

```text
Repo: ghx
Goal: <one concrete outcome>
Relevant files: <paths or discovery instructions>
Constraints:
- preserve public CLI behavior unless stated
- follow existing Go/Cobra patterns
- do not bump versions unless asked
- do not edit generated/release metadata unless required
- commit at meaningful checkpoints per AGENTS.md "Commits"; never push
Verification:
- go test ./...
- targeted command examples if CLI behavior changed
Return:
- files changed
- commits made (shas + messages)
- behavioral summary
- tests run
- unresolved risks
```

Delegate to specialist agents for:

- implementation after Fable has chosen the approach
- codebase search or map-based reconnaissance
- reproducing a bug with shell commands
- writing focused tests around an understood behavior
- collecting CI logs and identifying the failing command
- browser/computer tasks whose output can be summarized with screenshots or exact observations

Keep in Fable:

- product direction
- architecture tradeoffs
- public API and CLI semantics
- release decisions
- final review of delegated patches
- conflict resolution between agents
- security-sensitive or credential-sensitive choices

## Picking Models for Workflows and Subagents

These rankings are workflow defaults, not hard limits. Cost means effective cost in this working setup, including available limits and friction, not vendor list price. Intelligence is how hard a problem can be handled unsupervised. Taste covers UI/UX judgment, code quality, API design, and copy.

| Model | Cost | Intelligence | Taste |
| --- | ---: | ---: | ---: |
| `gpt-5.5` | 9 | 8 | 5 |
| `sonnet-5` | 5 | 5 | 7 |
| `opus-4.8` | 4 | 7 | 8 |
| `fable-5` | 2 | 9 | 9 |

How to apply:

- These are defaults, not limits. If a cheaper model's output does not meet the bar, rerun or redo with a smarter model.
- For anything that ships, use `intelligence > taste > cost` when the axes conflict.
- Bulk or mechanical work with a clear spec goes to `gpt-5.5`; in this setup it is effectively free and very steerable.
- User-facing work, API design, CLI semantics, docs, and copy need taste `>= 7`.
- Reviews of plans and implementations should use `fable-5` or `opus-4.8`; optionally ask `gpt-5.5` for an extra independent perspective.
- Do not use Haiku for this repo.

Mechanics:

- `gpt-5.5` is reached through the Codex CLI with `codex exec` or `codex review` when available.
- Prefer dedicated Codex skills or wrappers for `codex-implementation`, `codex-review`, and `codex-computer-use` style work.
- For work those wrappers do not cover, such as investigation or data analysis, run `codex exec -s read-only` with a self-contained prompt.
- Claude models such as `sonnet-5`, `opus-4.8`, and `fable-5` run through the Agent/Workflow model parameter when the workflow supports it.
- If a workflow or subagent slot only accepts Claude models but the desired worker is Codex, spawn a thin Claude wrapper with a low-effort `sonnet`-class model. Its only job is to write a self-contained Codex prompt, run `codex exec` through Bash, and return Codex's report.

Use capability roles when model names change:

| Work | Preferred worker | Effort |
| --- | --- | --- |
| Architectural decision, release judgment, ambiguous product behavior | Fable | high |
| Scoped Go implementation, tests, refactors with clear boundaries | Codex/GPT worker | medium/high |
| Repo reconnaissance, symbol maps, grep loops, dependency tracing | Cheap search/recon worker using `ghx`/shell | low/medium |
| CI log reading, package metadata, command output summarization | Cheap implementation worker | low/medium |
| Browser/computer use and UI verification | Codex/computer-use worker | low/medium |
| Final synthesis, acceptance decision, user-facing explanation | Fable | high |

Escalate effort only when the previous step produced concrete uncertainty that cannot be resolved with more evidence or a narrower task.

## Delegation CLI Mechanics

Do not rely on Fable remembering CLI flags. When delegating to Codex CLI or Claude Code workers, use the project-local `fable-delegation` skill in `.claude/skills/fable-delegation/SKILL.md`.

That skill is the canonical home for command syntax, bypass flags, wrapper prompts, and evidence report shape. Keep this file focused on routing judgment, not command tables.

## Evidence Contract

The evidence contract lives in `AGENTS.md` ("Evidence Contract") — it binds all
agents, not just Fable. So does the visibility/truthfulness core tenet
(`AGENTS.md` "Visibility and Truthfulness"): every score recomputable from
committed artifacts, measurement stack frozen mid-run, future LLM judges fully
traced and calibrated, negative results committed. Fable's specific duty:
enforce both on delegated work — and when reporting eval results to Goga,
always answer "who scored this, from what evidence, and how do I check it
myself" unprompted.
Reject or rerun any delegated result that cannot cite files, commands, or
outputs, and audit reports against the canonical repo rules rather than
trusting the worker's summary.

## Fable Working Style

Fable should read `AGENTS.md` before making repository changes and should treat it as authoritative for build, test, release, ADR, and skill-document rules.

Before delegating, Fable should reduce the task to a crisp objective and identify which facts must be verified. After delegation, Fable should review the result against the canonical repo rules rather than trusting the worker's summary.

When context grows:

- summarize decisions, not every observation
- keep unresolved questions visible
- delegate fresh reconnaissance instead of rereading large context
- keep final synthesis in Fable, especially for public API, CLI behavior, docs, release, and architecture decisions

## Final Review Checklist

Before handing work back:

- `git status --short` reviewed
- changed files are intentional
- tests or reason for not running tests are stated, following `AGENTS.md`
- public docs updated when behavior changed, following `AGENTS.md`
- release/version rules followed, following `AGENTS.md`
- delegated outputs were verified, not blindly trusted
