---
name: fable-delegation
description: Use when Claude/Fable should delegate implementation, review, investigation, UI verification, or mechanical coding work to Codex CLI or another Claude Code worker from this project.
---

# Fable Delegation

Use this skill when Fable is orchestrating work and should offload execution to Codex CLI or a Claude Code worker.

Fable owns judgment: requirements, architecture, public API/CLI behavior, ADR direction, release calls, and final acceptance. Workers own bounded execution and must return evidence.

Project-local Claude skills live in `.claude/skills/<skill>/SKILL.md`. Global Claude skills live in `~/.claude/skills/<skill>/SKILL.md`. Claude Code checks both. This project uses the project-local path.

## Command Map

| Metric / feature | Claude Code | Codex CLI |
| --- | --- | --- |
| Non-interactive run | `claude -p` | `codex exec` |
| Model | `--model <model>` | `--model <model>` |
| Effort | `--effort <low|medium|high|xhigh|max>` | configure model reasoning through prompt/config when needed |
| Working directory | run from repo root or use shell `cd` | `-C /Users/goga/Documents/goga/ghx` |
| Read-only sandbox | prefer permission mode/default tools | `-s read-only` |
| Write code bypass flag | `--dangerously-skip-permissions` | `--dangerously-bypass-approvals-and-sandbox` |
| Review command | `claude -p "<review prompt>"` | `codex exec review` |
| Structured output | `--output-format json` or `--json-schema` | `--json`, `--output-schema`, `-o` |
| Session resume | `--resume` / `--continue` | `codex exec resume` |
| Native project guidance | `.claude/skills/` + `CLAUDE.md` | `AGENTS.md` |

For writing code through delegated CLI workers, use the bypass flags explicitly:

```bash
claude -p --model sonnet --effort low --dangerously-skip-permissions "<prompt>"
codex exec -C /Users/goga/Documents/goga/ghx --dangerously-bypass-approvals-and-sandbox "<prompt>" </dev/null
```

**Always append `</dev/null` when running `codex exec` from a background or
non-interactive shell.** With stdin open, codex exec prints "Reading
additional input from stdin..." and blocks forever before doing any work
(observed 2026-07-05: a delegation sat 40+ minutes at 0% CPU with no session
rollout file — diagnose via `ls ~/.codex/sessions/<today>` and process CPU).
Do not pipe codex stdout through `tail`/`head` in background tasks either —
it buffers until exit, hiding all progress; let stdout stream to the task
output file.

Use `codex exec -s read-only` for investigation, review, summarization, or planning when the worker must not edit files:

```bash
codex exec -C /Users/goga/Documents/goga/ghx -s read-only "<prompt>"
```

Use JSON output when another tool or wrapper needs to parse Codex events:

```bash
codex exec -C /Users/goga/Documents/goga/ghx --json --dangerously-bypass-approvals-and-sandbox "<prompt>"
codex exec resume --json --dangerously-bypass-approvals-and-sandbox <thread_id> "<prompt>"
```

If a command form is rejected, verify current local syntax:

```bash
claude --help
claude agents --help
codex exec --help
codex exec review --help
```

## Background Codex: run the shell directly, never via a parking sub-agent

To delegate a coding task to Codex in the background, run `codex exec`
**directly from Fable's own session** as a background shell, into a git worktree
Fable created by hand. Do **not** spawn a Claude sub-agent (Agent tool) whose
only job is to launch Codex and wait.

Why (incident 2026-07-07, cost two lost delegations): a sub-agent started with
`isolation: worktree` gets a harness-managed worktree that is **auto-cleaned the
moment the agent's turn ends without a commit**. If that sub-agent launches
Codex as a background job and then parks (ends its turn to wait), the harness
sees an "unchanged" worktree and deletes it **out from under the still-running
Codex process** — Codex then reports "worktree became inaccessible" / "did not
exist", no commit or branch survives, and the orphaned uncommitted files can
even leak into a sibling worker's worktree. The parking sub-agent races the
worktree GC and loses.

Correct pattern:

```bash
# 1. Fable creates a real, self-managed worktree (NOT harness-managed) off mainline:
git worktree add -b codex-<task> "$SCRATCH/wt-<task>" mainline

# 2. Fable runs Codex directly as a background shell — Fable's own session is
#    never auto-cleaned and is reliably re-invoked when the job exits:
codex exec -C "$SCRATCH/wt-<task>" --dangerously-bypass-approvals-and-sandbox \
  "<self-contained prompt; end with: do NOT git commit — Fable handles git>" \
  </dev/null > /tmp/codex-<task>.log 2>&1 &

# 3. On completion Fable verifies (go test), commits the worktree branch,
#    merges from the repo root, and removes the worktree last.
```

One background `codex exec` per parallel task; Fable is the serial
verify/commit/merge point. Reserve the "Claude Wrapper Prompt" below strictly
for **synchronous** workflow slots that only accept Claude models — never as a
background launcher for Codex.

## When to Use Which Worker

Use Codex CLI for:

- clear-spec implementation
- mechanical edits
- test fixes
- repo search and codebase reconnaissance
- CI/log analysis
- independent code review

Use Claude Code workers for:

- Claude-only workflow slots
- thin wrappers around Codex when a workflow only accepts Claude models
- taste-sensitive review when Fable does not need to spend its own context
- background agents managed through Claude Code

Keep in Fable:

- product and architecture judgment
- ADR decisions and updates
- public CLI/API semantics
- final review and acceptance
- deciding whether delegated output is good enough

## Codex Implementation Prompt

Use this shape for code-writing delegation:

```text
Repo: ghx
Working directory: /Users/goga/Documents/goga/ghx
Goal: <one concrete outcome>

Relevant files or discovery:
- <paths, commands, or search instructions>

Constraints:
- Follow AGENTS.md.
- Preserve public CLI behavior unless explicitly asked to change it.
- Keep edits scoped.
- Do not bump versions unless explicitly asked.
- Do not touch release metadata unless required.

Verification:
- Run focused tests for the changed behavior.
- Run go test ./... when feasible.

Return:
- files changed
- behavior changed
- tests run and results
- evidence: file references, command output summary
- unresolved risks
```

Run it with:

```bash
codex exec -C /Users/goga/Documents/goga/ghx --dangerously-bypass-approvals-and-sandbox "<prompt>"
```

If the implementation needs parseable progress or a resumable thread id, add `--json`.

## Codex Review Prompt

Use this for independent review:

```bash
codex exec review --uncommitted "<review instructions>"
codex exec review --base mainline "<review instructions>"
codex exec review --commit <sha> "<review instructions>"
```

Review instructions should ask for findings first, ordered by severity, with file and line references. Ask the reviewer to focus on bugs, regressions, missing tests, incorrect assumptions, and docs drift.

## Claude Wrapper Prompt

Use this **only** for a *synchronous* workflow slot that accepts a Claude model but not Codex — the wrapper runs Codex in the foreground and returns its report. **Never** use it as a background launcher for Codex (see "Background Codex: run the shell directly" above — the parking sub-agent races the worktree GC and loses the work):

```text
You are a thin Claude wrapper. Do not solve the task yourself.

Write a self-contained Codex prompt for the task below, run it with:
codex exec -C /Users/goga/Documents/goga/ghx --dangerously-bypass-approvals-and-sandbox "<prompt>"

Return only Codex's final evidence report plus any command failure.

Task:
<task>
```

Run with:

```bash
claude -p --model sonnet --effort low --dangerously-skip-permissions "<wrapper prompt>"
```

## Evidence Contract

Every delegated worker must return:

```text
answer
files changed or inspected
commands run
test results
evidence snippets or file references
uncertainty
suggested next step
```

If a worker cannot cite files, commands, or outputs, treat its answer as a hypothesis and verify before acting.

## Operating Rules

- Start Codex delegation early when it can run independently, then continue Fable-side analysis.
- Give Codex concrete repository evidence in the prompt: paths, lines, `rg` hits, failing commands, or ADR references.
- Prefer file references and command summaries over pasted large snippets.
- Do not send secrets or environment values into worker prompts.
- Every research/investigation delegation prompt must pin the tool economy explicitly: "use text-only tools (WebSearch/WebFetch/curl/gh); do NOT use browser automation, Chrome tools, or screenshots — this is a text research task." Workers left unpinned have burned tokens screenshotting documentation pages (incident 2026-07-05; rule canonical in `AGENTS.md` "Tool Economy").
- After Codex returns, Fable must verify compatibility with `AGENTS.md`, current code, and the actual test results.
