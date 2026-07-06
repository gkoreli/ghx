---
title: "ADR-0033: Sidecar Works Out Of The Box — Loud Diagnostics, Neutral Session Cwd, and Agent Provenance"
date: "2026-07-06"
status: "accepted"
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0033. Sidecar Works Out Of The Box — Loud Diagnostics, Neutral Session Cwd, and Agent Provenance

## Status

Accepted. A reliability fix on the always-on runtime (ADR-0030) and its ACP
session setup (ADR-0020.1/0020.2, ADR-0027). It does not change scoring,
detectors, eval gates, the persona prompt, or the measurement stack.

## Context

A founder-dogfood run on a work laptop, 2026-07-06, produced the worst failure
mode for a reconnaissance sidecar: **silent nothing**.

```
$ ghx sidecar ask "what is ghx repository?"
$            # no report, no error, exit 0
```

The only breadcrumb was one line buried in `daemon.log`:

```
Ignoring 40 permissions.allow entries from .claude/settings.local.json:
this workspace has not been trusted. ...
```

### What the reproduction proved — and disproved

The first hypothesis was workspace trust: the ACP session inherited the caller's
git-repo cwd, its committed `.claude/settings.local.json`, and its untrusted
state. That hypothesis was **tested and rejected**:

- On a healthy dev machine, `ghx sidecar ask` from a brand-new **untrusted**
  temp dir (`/tmp/untrusted-repro-0033`, not previously trusted) **succeeded in
  34s**. An untrusted cwd alone does not reproduce the failure.
- The founder reports that **accepting the workspace trust dialog did not fix
  it** on the work laptop.

So the trust warning is very likely a **red herring** — a benign stderr line
that happened to be the only visible artifact. The real failure is
**environment-specific to that laptop** and, critically, **we cannot see it**:
different Claude auth/config (plausibly enterprise Bedrock/Vertex env vars the
auto-spawned daemon never inherited), a different adapter or Node version, or
some other process-environment difference. None of it is knowable from the
committed artifacts of the failing run.

### The three real defects

1. **The failure was invisible to the caller.** Both spawn paths
   (`RunTurnWithOptions` in `internal/sidecar/acp.go`, the warm `AgentWorker` in
   `internal/sidecar/daemon_worker.go`) set `cmd.Stderr = os.Stderr`. Under the
   ADR-0030 auto-spawn daemon, `os.Stderr` is the *daemon's* stderr — the daemon
   log — so the adapter's diagnostics never reached the CLI caller and nothing
   in the session directory recorded them.

2. **An empty turn degraded to a mute WARN, not an error.** An empty adapter
   turn yields no `<ghx-report>` block; the bounded corrective retries
   (ADR-0021 D2) were also empty; the runtime shipped `WARN: sidecar did not
   emit a <ghx-report> block` with **`err == nil`**. The daemon returned a
   normal `AskResponse`; the CLI printed the WARN and exited 0. A hard failure
   wore the costume of a soft one.

3. **The run recorded no agent provenance.** `SessionMeta`
   (`internal/sidecar/session.go`) stored no cwd, no agent command, no spawn
   context, and no environment fingerprint — a direct violation of the
   Visibility and Truthfulness tenet: the run was not recomputable or
   diagnosable from committed artifacts. The daemon auto-spawn inherits the
   **first caller's** environment, so "works from shell A, fails from shell B"
   is exactly the class of failure the record must be able to describe — and it
   couldn't.

Because we cannot see the founder's failure, **loud, deterministic diagnostics
is the primary deliverable**, not any single guessed root cause.

## Research (authoritative sources)

Read with `ghx` itself (dogfooding — NORTH_STAR: ghx is the reconnaissance
sidecar) and the Claude Code docs. Founder directive: "we need a more
deterministic solution, like we should solve this configuration via acp
configuration itself somehow." The honest finding is that ACP config can pin
*workspace and permission* behavior per session, but **cannot** pin *auth*,
which is process-environment — so the deterministic answer is part config, part
diagnostics.

- **Adapter identity.** npm `@agentclientprotocol/claude-agent-acp@0.55.0`
  (`sidecar.ClaudeACPAgentCmd`) → repo
  [`agentclientprotocol/claude-agent-acp`](https://github.com/agentclientprotocol/claude-agent-acp),
  `dependencies`: `@anthropic-ai/claude-agent-sdk@0.3.198`. A thin ACP shim over
  the **Claude Agent SDK** (README).
- **What ACP `_meta` CAN pin per session** (`src/acp-agent.ts`): the SDK
  `query()` options default `settingSources: ["user","project","local"]` then
  spread `...userProvidedOptions` — so our `_meta.claudeCode.options` overrides
  win. `resolvePermissionMode` maps a per-session `permissionMode`
  (`default | acceptEdits | plan | dontAsk | bypassPermissions`; default
  `"default"`). The ACP session `cwd` flows into `SettingsManager(cwd)`
  (`src/settings.ts`), which resolves settings via the SDK's `resolveSettings` +
  `filterEscalatingDefaultMode`, "matching the CLI's trust policy." So
  **workspace isolation and permission mode are per-session ACP options** — we
  already pin `settingSources: []` (ADR-0016.3) and now pin the cwd (D1).
- **What ACP `_meta` CANNOT pin: auth/credentials.** The adapter derives the SDK
  subprocess environment from its own `process.env`
  (`src/tests/create-session-options.test.ts`: `capturedOptions.env.HOME` /
  `.PATH` come straight from `process.env`; `src/index.ts` copies managed
  `policy.effective.env` into `process.env`). Auth (`ANTHROPIC_API_KEY`,
  `CLAUDE_CODE_USE_BEDROCK`/`_USE_VERTEX`, AWS/Vertex vars) rides that
  environment. The CHANGELOG entry **`0.20.1 - fix: inherit process.env when
  spawning agent subprocess`** shows this exact surface has broken before across
  versions. Conclusion: **no ACP session option can repair a missing or
  mismatched auth environment** — a daemon that inherited the wrong shell's env
  will fail no matter how the session is configured. That residual is a
  *diagnostics* problem, addressed by D2–D4.
- **Trust is keyed on the git repo root** (docs, code.claude.com/docs/en/
  permissions): trust is "keyed on the git repository root or, outside a
  repository, the directory you started Claude Code from," and rules apply
  "without workspace trust" when "the directory you started Claude Code from
  isn't inside a git repository." This is why the neutral, non-repo session cwd
  (D1) is the right *deterministic* workspace even though trust was not the
  founder's root cause.
- **`bypassPermissions`** is CVE-2026-33068's vector when set from
  repo-controlled settings; rejected (see Alternatives).

## Decision

Make the sidecar work out of the box **and** make every failure loud and
diagnosable, so the next laptop-specific failure is a one-command diagnosis
instead of a silent nothing. No user config is ever written — we never touch
`~/.claude.json` or `.claude/settings*.json`; only process-level knobs (the ACP
session cwd, ACP `_meta` options) are used.

### D1. Neutral, persisted session workspace (cwd)

Default the ACP session cwd to the session's own ghx-owned directory,
`~/.ghx/sessions/<name>` (`SessionWorkspace`), instead of `os.Getwd()`.

Rationale: it is **not inside a git repository** (trust gate does not apply per
the docs), contains **no `.claude/settings*.json`** (nothing for the adapter to
ignore or warn about), and is **deterministic** — the same session runs in the
same directory regardless of where, or from which shell, the human invoked
`ghx`. This is the workspace half of the founder's "deterministic via acp
configuration" directive; combined with the existing `settingSources: []`, the
session's *workspace and settings* are fully pinned and host-independent.

Resolution precedence (in `askWithTurnRunner`, which owns session identity):

1. explicit `Config.Cwd` override — evals pin a repo checkout here;
2. persisted `SessionMeta.Cwd` — reuse on resume for determinism;
3. `SessionWorkspace(sessionsDir, name)` — the neutral default.

The resolved cwd is set on **every** turn (initial, stale-session retry,
wrap-up, corrective retry). The low-level `RunTurnWithOptions` / `AgentWorker`
keep `os.Getwd()` only as a last-resort default for bare callers; the
authoritative neutral-workspace policy lives one layer up, in the runtime that
knows the session.

### D2. Per-session agent stderr capture (top-priority diagnostics)

Spawned-adapter stderr is teed to `~/.ghx/sessions/<name>/agent-stderr.log`
(`AgentStderrLogName`) in addition to the parent process stderr, via a new
`RunTurnOptions.AgentStderrPath`. Foreground `ghx sidecar daemon` still shows
live logs; a durable, per-session diagnostic artifact now **always exists**.
This is what turns the founder's invisible failure into a `cat` away.

### D3. Loud, actionable failures

- **Genuine turn errors** (init/session/prompt/liveness/dead-peer) are wrapped
  by `DiagnoseTurnError`, which appends the tail of `agent-stderr.log`, the
  log's path, and — only when the tail matches the documented workspace-trust
  warning — a *hint* pointing the human at their own config (never auto-applied,
  never written). The wrap preserves the `errors.Is` chain so ADR-0027
  classification still holds. This tail now rides the daemon RPC error back to
  the CLI caller.
- **The silent-WARN case** (empty turn, no report, `err == nil` — the founder's
  exact path) is promoted from mute to loud: the WARN report's answer names the
  per-session `agent-stderr.log` so the caller is never handed a dead end, and
  carries the trust hint when that specific warning is present. Report contract
  and artifact emission are preserved (the turn genuinely completed).

### D4. `ghx sidecar doctor --live` — the deterministic laptop-diagnosis tool

The existing `doctor` `acp-handshake` check only runs ACP **initialize**, which
passes on setups where a real prompt turn then fails (auth resolves lazily at
prompt time, not at initialize). `--live` adds a check that runs a **full
`session/new` + one tiny prompt turn** through the real configured agent, in a
neutral temp cwd, with stderr captured. On failure it prints the **failure
stage** (initialize / new session / prompt — read from the staged error text)
and the **agent's stderr tail**. This is the tool a founder runs on the broken
laptop to see exactly where and why the real agent dies, without an eval and
without guessing.

### D5. Agent provenance in `SessionMeta` (env fingerprint, names only)

Persist, on the session's first turn and reused thereafter:

- `cwd` — the resolved ACP session workspace (D1);
- `agentCmd` — the exact agent command line the session used;
- `spawnCwd` — `os.Getwd()` of the process that created the session (for the
  daemon, the **first caller's** directory — the auto-spawn provenance);
- `agentEnv` — the **names** (never values) of agent-relevant environment
  variables present at creation, from a curated allowlist (auth: `ANTHROPIC_*`,
  `CLAUDE_CODE_USE_BEDROCK`/`_USE_VERTEX`, AWS/Vertex; transport: `*_PROXY`,
  `NODE_EXTRA_CA_CERTS`; ghx: `GH_TOKEN`/`GITHUB_TOKEN`; `PATH`/`HOME` presence).

Recording env *names* (presence, never secret values) is what makes "works from
shell A, fails from shell B" diagnosable from committed artifacts: a session
that ran with `CLAUDE_CODE_USE_BEDROCK` set but no `AWS_*` names present tells
the whole story at a glance. Legacy session dirs without these fields get them
written on their next turn — a backfill on write, no shim, no dual path.

## Alternatives considered

- **Blame workspace trust; write `hasTrustDialogAccepted: true` or drop a
  `.claude/settings.json`.** Rejected twice over: the reproduction shows trust
  is not the cause, and ghx must never mutate the user's Claude Code config.
  The neutral cwd (D1) still removes trust as a *variable* deterministically,
  which is worth doing regardless.
- **`permissionMode: "bypassPermissions"` in `_meta`.** Rejected. Unnecessary —
  `denyClient` + `settingSources: []` already isolate — and it is CVE-2026-33068's
  vector. It also would not touch the auth failure that actually bit the founder.
- **Guess it is auth and inject env into the daemon.** Rejected as a blind fix:
  we cannot see the failure, and silently reshaping the agent's environment is
  exactly the kind of non-deterministic magic that caused the confusion. D4/D5
  make the environment *visible* so the human fixes the right thing.
- **Hard-fail the silent-WARN case with a non-nil error by default.** Rejected:
  the turn genuinely completed with auditable artifacts, so the honest shape is
  a loud report, not a lost turn. Real transport failures already return errors
  (D3).

## Consequences

- The next environment-specific failure is diagnosable without the orchestrator:
  `agent-stderr.log`, the staged error tail in the CLI, `doctor --live`, and the
  `SessionMeta` env fingerprint together localize it to stage + environment.
- Every session directory is self-describing: cwd, agent command, spawn cwd,
  env-name fingerprint, and raw adapter stderr live beside the reports and
  traces.
- The agent no longer reads the caller's repo as its filesystem workspace
  (intended: the sidecar explores GitHub through `ghx`, `settingSources: []`,
  `Tools: [Bash, Read]`). Evals that need a local checkout set `Config.Cwd`
  explicitly, unchanged.
- Honest scope: D1 makes the *workspace* deterministic; it does **not** claim to
  fix auth. If the founder's laptop failure is an auth/env gap (the leading
  hypothesis), D2–D5 are what surface it — no ACP session option can repair it,
  and this ADR does not pretend otherwise.

## Cross-references

- ADR-0030 always-on daemon — the auto-spawn that routed adapter stderr into
  `daemon.log` and inherits the first caller's environment.
- ADR-0020.1/0020.2 session steering — `settingSources: []`, resume re-assert;
  this ADR adds the cwd and provenance those options assumed.
- ADR-0027 runtime resilience — the failure taxonomy this ADR makes legible to
  the caller and to `doctor --live`.
- ADR-0022 shared visibility runtime — `agent-stderr.log` and the provenance
  fields join the per-session artifact set.
