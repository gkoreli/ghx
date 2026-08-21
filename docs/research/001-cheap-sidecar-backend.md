# Research 001: Cheap configurable sidecar backend and quota governance

**Capability candidate:** cheap-configurable-backend — the sidecar worker runs
on a deliberately cheap, explicitly configured ACP agent, and no formal eval
can silently burn an expensive/shared quota pool.

**Workstream:** NORTH_STAR cheap-specialist thesis; SAF "sidecar over ACP".

**Status:** research artifact / pre-proposal. Read-only analysis grounded in
committed code and the 2026-07-03/04 incident. Proposes, does not implement.

## 1. Problem statement

The thesis requires the sidecar to be the cheap specialist. The original
experiment design even said so: `experiments/ghx-sidecar/AGENT_DEFINITION.md`
instructs "Paste this into a fresh Codex session that will act as the sidecar
agent." But nothing in the runtime or eval harness enforces cheapness or even
records which backend ran. On 2026-07-03/04 the formal 90-episode gate run
executed with `GHX_EVAL_AGENT=claude-agent-acp`, consuming the same
Claude/Fable subscription quota as the main engineering agent. No
`~/.ghx-sidecar/config.json` existed, so the code default `claude` applied
(`internal/sidecar/config.go:28`), and the eval env var overrode config
entirely anyway (`internal/sidecar/evals/eval_test.go:33-36`).

## 2. Evidence

- `internal/sidecar/config.go:17-31` — `Config{AgentCmd, SessionsDir}`;
  default `AgentCmd: "claude"`; `LoadConfig` falls back to defaults on any
  read/parse error. No model, provider, or quota-pool field exists.
- `internal/sidecar/config.go:69-77` — `knownAgents` probes
  `claude/codex/kiro` via `--version` only; detection proves CLI presence,
  not ACP capability.
- `internal/cli/sidecar.go:166-184` — `config init` writes
  `found[0]` (detection order, not preference); `config show` prints config.
- `internal/sidecar/evals/eval_test.go:33-36` — `GHX_EVAL_AGENT` env var
  bypasses sidecar config with no warning, no artifact metadata.
- Episode artifacts record profile and rewards but not the agent command or
  backend identity (`internal/sidecar/evals/episode.go`), so runs are not
  self-describing about what executed them.
- Incident artifacts: `internal/sidecar/evals/.ghx-evals/runs/gate-2026-07-03-r2/`
  (72/90 episodes; verdict PRELIMINARY; run halted to stop quota burn);
  scratchpad driver exported `GHX_EVAL_AGENT=$S/claude-acp` wrapping
  `@agentclientprotocol/claude-agent-acp`.
- Environment at incident time: `codex` CLI 0.142.5 installed; no verified
  Codex ACP stdio adapter present; `claude-agent-acp` npm adapter proven
  working (live episodes + session resume).

## 3. Cross-references

- ADR-0033 (sidecar out-of-box agent config) and children .1
  (client-auth-env-passthrough), .2 (agent-setting-sources-optin) — cover how
  agent settings/auth flow into the sidecar; neither adds backend identity to
  artifacts nor quota guardrails for eval runs. This artifact complements,
  not duplicates, that thread.
- ADR-0016.1/.2/.3 — gate definitions, validity hardening, measurement
  ladder; the ladder's smoke/spot-check tiers are where cheap-backend
  verification belongs before any formal run.
- AGENTS.md truthfulness rules — "every score recomputable from committed
  artifacts" implies artifacts must record what agent produced them.
- `docs/NORTH_STAR.md` — cheap-specialist thesis; M4 floor framing.

## 4. Rationale

Cheapness is currently a hope enforced nowhere: a default string, an env var
override, and artifacts that cannot say which backend produced them. The
incident showed the failure is not hypothetical — it already redirected a
formal run's cost onto the wrong budget. Governance must live in three places:
config (explicit selection), preflight (fail-fast on expensive backends for
formal runs), and artifacts (backend identity recorded per episode).

## 5. Options considered

| Option | What | Pros | Cons | Verdict |
|---|---|---|---|---|
| A. Status quo | Default claude + env override | Zero code | Silent quota burn; undescribed artifacts | Rejected by incident |
| B. Backend metadata in artifacts | Record AgentCmd/backend + config source per episode | Cheap; makes runs auditable; enables post-hoc filtering | Does not prevent burn alone | Recommended core |
| C. Preflight guardrail | Formal runs fail/warn when AgentCmd matches expensive/shared pool unless explicit override | Prevents recurrence; aligns with measurement ladder | Needs an explicit notion of "expensive" — configurable list | Recommended core |
| D. Config schema extension | Add provider/model/quotaPool fields beyond AgentCmd | More expressive | Bigger surface; AgentCmd already encodes wrapper choice | Defer until a second real backend exists |
| E. Codex ACP adapter as default cheap worker | Verify/provide codex-acp wrapper; init prefers verified-cheap agents | Fulfills original Codex intent; real quota separation | Adapter must be validated (detection today is --version only) | Recommended follow-up, gated on adapter validation |

## 6. Recommendation (pre-registration sketch)

1. ADR proposal: episode artifacts gain `agentCommand` + `configSource`
   (env/config/default) fields (Option B); preflight gains an
   `expensiveBackends` check with explicit override env for formal runs
   (Option C). Both are additive; no scorer changes; no rescore.
2. Follow-up ADR once a non-Claude ACP adapter is validated: extend
   `DetectAgents` to verify actual ACP handshake, and let `config init`
   prefer verified-cheap backends (Option E).
3. Docs: document the incident and the guardrail in the eval README so the
   convention survives session loss.

## 7. Open questions

1. Should the guardrail hard-fail or warn-and-continue for formal runs?
   Hard-fail matches the ladder's pre-registration discipline; decide in ADR.
2. Is "expensive" a property of the command pattern (claude*) or a configured
   list? Configured list generalizes; pattern match is simpler.
3. Where does backend identity belong in the artifact schema — top-level
   episode field or per-turn? Top-level matches how episodes are spawned.
