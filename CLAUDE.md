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

Fleet economics (Goga, 2026-07-06, 20x Claude plan): the default execution
fleet is **parallel background Claude workers**, one isolated worktree per
worker, spawned wide — under-delegation wastes the plan, and Goga has said
so explicitly ("delegate much more"). Codex/GPT workers are supplementary
capacity for mechanical, clear-spec work when their tokens are available.
Fable stays the serial merge point: review each worker's evidence against
the ADR and AGENTS.md before merging; plan conflict-watch pairs at spawn
time (workers touching neighboring files merge in a deliberate order).

Background-worker mechanics learned the hard way:

- Tell every worker explicitly: run long commands (live asks, gate rounds)
  in the **foreground** with a generous Bash timeout, and never end the
  turn while work is pending — a worker that "completes" mid-wait strands
  its task.
- If a worker ends prematurely anyway, relaunch with a continuation prompt
  that includes a state check of what the first attempt already produced
  (session dirs, commits); mid-flight steering of a running worker is not
  always available.
- Workers commit on their branch, never merge, never push. Worktrees with
  commits survive worker completion; merge and clean up from mainline.

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

## Git Hygiene on a Shared Mainline (learned 2026-07-07)

Parallel engineers — other Claude sessions or Goga — may be committing to
`mainline` in the same working tree at the same time. Plain appended commits are
safe: git serializes ref updates and history just interleaves. The danger is
**history rewrites under concurrency**:

- **Never `git commit --amend` on `mainline`.** Amend rewrites whatever `HEAD`
  currently points at — and `HEAD` may have just moved to a parallel engineer's
  commit, silently clobbering it (incident 2026-07-07: an amend meant to reword a
  worker branch overwrote a product-audit commit's message).
- **If you must reword/amend, target a SPECIFIC commit id — never "the recent
  one" (blind `HEAD`).** Prefer rewording inside the worker's OWN worktree/branch
  before merging (isolated). To fix a commit already on `mainline`, rebuild it
  deterministically with `git commit-tree` (identical tree, corrected message)
  and move the branch with an atomic CAS `git update-ref refs/heads/mainline
  <new> <expected-old>` so a racing commit makes it fail safely — do NOT
  `git rebase` a shared mainline whose working tree holds another engineer's
  uncommitted files.
- **Land worker branches by fast-forward only**: rebase the branch in ITS OWN
  worktree, then `git merge --ff-only` from `mainline`. If a parallel commit
  lands between the rebase and the ff-merge, the ff-merge fails LOUDLY — just
  retry; it never corrupts.
- Before any history-touching op on `mainline`, check `git status` is clean of
  other engineers' changes and that no commit is actively racing.

## Remote Control (learned 2026-07-06)

How to make a ghx session controllable from claude.ai/code or the Claude
mobile app, learned the hard way after a dead session left a stale env:

- Resume an existing session AND expose it remotely in one shot:
  `claude --resume <session-id> --remote-control "ghx-remote" --dangerously-skip-permissions`.
  Requires Claude Code ≥ 2.1.200 (`claude update` first; 2.1.169 rejects
  the resume flags). Session ids = newest `*.jsonl` in
  `~/.claude/projects/-Users-goga-Documents-goga-ghx/`.
- The TUI needs a real TTY. A headless/background launch fails with
  "provide a prompt to continue". Run it inside tmux:
  `tmux new-session -d -s ghx-remote -x 200 -y 50 'claude --resume <id> --remote-control "ghx-remote" --dangerously-skip-permissions'`
  then `tmux attach -t ghx-remote` to watch locally (`ctrl+b d` to detach;
  killing the tmux session kills the remote session).
- Resuming a large session prompts summary-vs-full: pick **resume from
  summary** — full replay of a ~750k-token session burns a large slice of
  usage limits, and loop state is persisted in memory files anyway.
- Standalone host mode (`claude remote-control --permission-mode
  bypassPermissions`) serves the current dir for NEW sessions only; it does
  not carry existing session context.
- Stale environments: envs listed at claude.ai/code persist server-side
  after their host process dies — connecting then yields "bridged Claude
  Code process stopped responding". Before debugging anything else, verify
  a live host process exists (`pgrep -f remote-control`, or the session's
  worker: `ps ax | grep sdk-url`). Endless spinner + 503s in the web
  console = claude.ai outage; check status.claude.com.

## Eval Cadence (Goga, 2026-07-05/06 — binding)

Most changes need NO live eval at all — unit tests and one live smoke of
the touched path suffice. When a change plausibly shifts agent behavior,
run a **random spot check** (2-5 episodes MAX — hard cap, Goga 2026-07-06: 2-3 task×profile cells,
include a `ghx-sidecar` cell; sidecar-only deltas like persona/report
contract get sidecar-only cells), read the anomaly table, move on.

Full-rigor citable runs are **rare, event-driven, and Goga-triggered**:
a milestone verdict, numbers to quote externally, or a measurement-stack
change that must re-ground the record. There is no "product deltas
accumulated, time to re-run" treadmill — that rule never existed and is
explicitly rejected (Goga, 2026-07-06, after a night that burned two
full runs plus aborted attempts). When a citable run does happen: use
baseline reuse (~30 episodes; ADR-0025.1) once a seeded source exists,
run it in the background, and never let it block engineering
(AGENTS.md; ladder ADR-0016.3; economics ADR-0025). "Loop towards the
north star" is never by itself a trigger for a full run.

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

