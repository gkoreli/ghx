---
title: "ADR-0038: Cheap Backend Governance — Backend Identity in Artifacts + Expensive-Quota Preflight Guardrail"
date: "2026-08-21"
status: "accepted-on-merge"
parent: ADR-0033
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0038. Cheap backend governance

## Status

Accepted on merge (Goga, 2026-08-21). Research grounding: `docs/research/001-cheap-sidecar-backend.md`.
Complements ADR-0033 (out-of-box agent config) — that thread covers how agent
settings/auth flow; this one covers which backend ran and what a formal run
may spend.

## Problem (incident-backed)

The sidecar thesis requires a cheap specialist worker, but cheapness is
enforced nowhere:

- Default `AgentCmd` is `claude` (`internal/sidecar/config.go:28`); no config
  file existed on this machine, so the default silently applied.
- `GHX_EVAL_AGENT` overrides config with no warning
  (`internal/sidecar/evals/eval_test.go:33-36`).
- Episode artifacts do not record the agent command or config source, so runs
  are not self-describing.
- On 2026-07-03/04 the formal 90-episode gate run executed via
  `claude-agent-acp`, burning the same Claude/Fable subscription quota as the
  main engineering agent, and had to be halted at 72/90 episodes.

## Proposal (two additive slices)

1. **Backend identity in artifacts**: every episode artifact records
   `agentCommand` and `configSource` (`env` | `config` | `default`). Additive
   schema fields only — no scorer changes, no rescore of historical runs
   (measurement-stack freeze).
2. **Expensive-quota preflight guardrail**: eval preflight gains an
   `expensiveBackends` check (configured list, e.g. `claude*`). Formal gate
   runs hard-fail when the resolved agent matches unless an explicit override
   env is set for that run. Smoke/spot-check tiers may proceed with a visible
   warning, per the measurement ladder (ADR-0016.3): feature-level go/no-go
   never blocks on full runs, but full runs must be deliberate about budget.

## Alternatives rejected / deferred

- Per-model config keys now (provider/model/quotaPool fields): premature
  until a second real backend exists; AgentCmd already encodes wrapper choice.
- Making `config init` prefer codex: deferred until a Codex ACP adapter is
  actually validated — current detection is `--version`-only
  (`config.go:69-77`) and proves nothing about ACP capability.

## Consequences / follow-ups

Both slices are small diffs + unit tests in `internal/sidecar/evals`. The
guardrail list lives in config so operators can reclassify backends without
code changes. Follow-up ADR after validating a non-Claude adapter: extend
`DetectAgents` to verify a real ACP handshake and prefer verified-cheap
backends in `config init`.

## Implementation notes (2026-08-21)

Slice 1 (backend identity) was found **already implemented upstream** —
`AgentIdentity` (`internal/sidecar/evals/episode.go:230-238`) records
agentCommand/adapter name+version/subject model/wrapper SHA256, populated via
`agentIdentity()` (`env.go:114`) and `ProbeAgentIdentity` (`runner.go:70`) on
every episode; the pinned wrapper `scripts/eval-agent-acp.sh` pins
`claude-agent-acp@0.55.0` and a subject model. Nothing to add.

Slice 2 (guardrail) implemented as `internal/sidecar/evals/guardrail.go`:
`CheckExpensiveBackend` matches the resolved agent command against a
configurable list (`GHX_EVAL_EXPENSIVE_BACKENDS`, default
`claude-agent-acp,claude-acp,claude`) by full-string/base-name substring AND —
critically — by **wrapper content inspection** (the pinned wrapper's argv0
hides the real backend). `TestEpisodes` fails fast when a formal run matches
unless `GHX_EVAL_ALLOW_EXPENSIVE_BACKEND=formal-run` is set; the deliberate
value requirement prevents stale shell vars from silently opting in.
Unit-tested in `guardrail_test.go` (name matching, wrapper content, opt-in
semantics, custom list, error text). Full build/vet/test green.

