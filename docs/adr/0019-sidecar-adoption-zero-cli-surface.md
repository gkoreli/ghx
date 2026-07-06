---
title: "ADR-0019: Sidecar Adoption — Zero-CLI-Knowledge Surface and Dogfooding"
date: "2026-07-05"
status: "accepted — D1-D4 implemented; dogfood exit pending"
thread: "sidecar-adoption"
author: "Goga Koreli"
---

# 0019. Sidecar Adoption — Zero-CLI-Knowledge Surface and Dogfooding

## Status

Accepted. Opened the day M4 closed with THESIS SUPPORTED
(`docs/evals/gate-run-2026-07-05-confirmatory/`, commit 6f69ff9). This ADR
governs NORTH_STAR **M5**: the concise "reconnaissance service" surface and
founder dogfooding. New thread (`sidecar-adoption`) — it is about the
customer-facing boundary, not the runtime (ADR-0015) or the evals
(ADR-0016.x).

D1-D4 were implemented on 2026-07-06. The ADR remains open for the M5 exit
bar: one week of founder dogfooding with breaking friction fixed or explicitly
deferred.

## Context: the measured thesis vs the shipped surface

The north-star end state is a main agent with **zero knowledge of the ghx
CLI**. M4 proved the boundary works when the persona knows the doctrine:
sidecar correctness 0.908 vs direct-ghx 0.931 at 25× compression, 24×
signal per main-agent token. Per AGENTS.md ("read the M4 verdict as a
conservative floor"), the numbers are expected to skew further toward the
sidecar as ergonomics and scoring fidelity improve — M5 is the ergonomics
lever.

But every surface we ship today teaches the main agent the CLI — the
opposite of the goal:

- `skills/ghx/SKILL.md` is **167 lines** of CLI grammar and quirks;
  `skills/ghx-mcp/SKILL.md` is **204 lines**. The north star's "Why"
  section names this doctrine overhead as a core cost.
- `ghx serve` (internal/cli/serve.go) registers **seven direct tools**
  (`explore`, `repos`, `search`, `read`, `tree`, `search_tools`, `code`) —
  the main agent drives every exploration step itself.
- `ghx sidecar ask` (internal/cli/sidecar.go) requires `--session` and
  `--repo` flags; without them it errors. Human output prints only
  `report.Answer`, discarding the evidence that is the product's whole
  differentiator.
- `sidecar.LoadConfig()` defaults `AgentCmd: "claude"`
  (internal/sidecar/config.go:32) — a binary that does **not** speak ACP on
  stdio. This exact trap cost an aborted eval round (ADR-0016.1 notes):
  every turn hangs to timeout. A dogfooding user hits it on first run, and
  neither `ask` nor `doctor` catches it (preflight checks token/network/
  binary, not the ACP handshake).

## Decision

Four decisions define the M5 surface. Guiding rule: **the outside world
learns one sentence — "ask ghx repo questions in English; it returns
evidence reports" — and nothing else.**

### D1. A concise recon skill, the new default entry point

New embedded skill `skills/ghx-recon/SKILL.md`, printed by
`ghx skill --recon`, hard budget **≤ 30 lines**. Contents: what the
service is, the one entry command (`ghx sidecar ask`) or the one MCP tool
(`recon`), how to phrase good questions (goal + repo, not step-by-step
instructions), what the report fields mean (answer / verified / evidence /
uncertainty / nextReads), that sessions persist so follow-ups are cheap,
and the latency expectation (tens of seconds per fresh question). Zero CLI
grammar, zero backend names, zero exploration doctrine — those belong to
the sidecar persona (internal/sidecar/prompt.go, ADR-0016.7).

The existing `ghx` and `ghx-mcp` skills remain shipped: the P1 direct CLI
is a standalone product and the eval baseline. Docs position them as the
power-user path; the recon skill is the recommended default for agents.

### D2. Single-tool MCP mode

`ghx serve --recon` exposes exactly **one tool**: `recon(question, repo,
session?)`, wrapping `sidecar.Ask` and returning the report (JSON).
Session defaults per D3. The seven direct tools stay behind the existing
default mode for now; flipping recon-only to the default is an explicit
post-dogfood decision recorded here when made. Rationale: a main agent
with one tool cannot be tempted into step-driving exploration, and the
tool description doubles as the skill's one sentence.

### D3. Frictionless `ask`: optional session, evidence-bearing output

- `--session` becomes optional, defaulting to a repo-derived name
  (`owner-repo` slug). Named sessions remain for parallel threads on one
  repo. `--repo` stays required (the question's anchor).
- Human output shows the evidence, not just the answer line: answer, then
  compact verified/relevant-files/uncertainty sections. `--json` unchanged
  (full report). Evidence-not-vibes is the product; hiding it behind
  `--json` undersells it on the first impression.

### D4. Fail-fast runtime preflight

- `sidecar doctor` gains an **ACP handshake check**: spawn `AgentCmd`,
  send `initialize`, require a response within a short timeout.
- `sidecar ask` fails fast with an actionable message ("agent <cmd> did
  not complete the ACP handshake; run `ghx sidecar config init` or set an
  ACP-capable agent") instead of hanging to the turn timeout.
- `config init`'s detection must verify the handshake, not just PATH
  presence — detecting bare `claude` and writing it to config reproduces
  the trap with extra steps.

This closes the pre-registered follow-up from the aborted eval round
(memory + ADR-0016.1 notes: "fail-fast on initialize timeout in runner +
preflight handshake validation").

## Dogfooding protocol (the M5 exit bar)

NORTH_STAR capability 4: dogfooding is the ergonomics bar. Protocol:

1. Goga wires the recon surface into daily Claude Code use (MCP `--recon`
   mode and/or the recon skill) and uses it instead of the bare CLI.
2. Friction is logged declaratively in `docs/dogfood/FRICTION.md` — date,
   what was attempted, what ground, severity (breaking/soft, mirroring the
   anomaly taxonomy). Friction items outrank speculative features.
3. M5 is DONE when: the four decisions are shipped, and one week of daily
   dogfooding produces a friction log where every breaking item is fixed
   or explicitly deferred with rationale.

Dogfood sessions also feed M6/ADR-0018: production sidecar sessions emit
the same OTel traces as evals, so friction reports can cite traces.

## Considered and rejected

- **Teaching main agents the `ghx sidecar` CLI surface** (flags, sessions,
  config): explicit north-star non-goal; the skill budget exists to make
  this impossible.
- **Removing the direct tools/skills now**: P1 is shipped, valuable
  standalone, and the eval baseline profile (`ghx`) depends on it.
  Deprecation is a P3 (M7) question.
- **A hosted recon service**: violates local-first; nothing about M5
  needs a server beyond the existing local MCP transport.
- **Auto-detecting repo from cwd git remote for `--repo`**: attractive
  ergonomics, but the sidecar is remote-first and repo-explicit; silent
  inference invites wrong-repo evidence. Revisit from the friction log if
  dogfooding demands it.

## Cross-references

- NORTH_STAR M5 (frontier), capability 4 (dogfooding bar).
- ADR-0016.7 — the persona contract; the skill/persona split this ADR
  completes: doctrine inside, one sentence outside.
- ADR-0018 — dogfood sessions inherit GenAI-convention traces (M6 slice).
- ADR-0014.1 — original sidecar vision (tiers; the recon tool is Tier 0/1
  today, tiers become visible in reports at M7).
- AGENTS.md "Visibility and Truthfulness" — verdict-is-a-floor rationale
  that makes ergonomics the highest-leverage post-M4 work.

## Implementation Notes (2026-07-06)

- D1 landed as `skills/ghx-recon/SKILL.md`, embedded via `skills/doc.go` and
  printed by `ghx skill --recon`. `skills/doc_test.go` now verifies the embed
  is non-empty, has complete frontmatter, and keeps the recon body at or under
  the 30 non-empty-line budget. The prose is intentionally concise and leaves
  CLI doctrine in the sidecar persona and the legacy power-user skills.
- D2 landed as `ghx serve --recon`. The flag registers exactly one MCP tool,
  `recon(question, repo, session?)`, and the no-flag server path still
  registers the existing direct tools. The handler loads sidecar config,
  defaults the session through the same repo slug used by CLI ask, calls
  `sidecar.Ask`, and returns the full report as JSON text.
- D3 landed in `internal/cli/sidecar.go`. `--session` is optional and defaults
  to a lowercase repo slug (`owner/repo` -> `owner-repo`, non-alphanumerics to
  dashes). Human output now prints the answer plus compact verified,
  relevant-files, and uncertainty sections; `--json` still emits the report
  struct unchanged.
- D4 landed by adding an initialize-only ACP handshake probe that reuses the
  same `acp.NewClientSideConnection` path as runtime turns. `sidecar ask`
  invokes it before session setup or prompting, `sidecar doctor` reports an
  `acp-handshake` check, and `config init` only accepts known agents that pass
  ACP initialize rather than mere PATH or `--version` presence.
- Tests use tiny fake stdio agents: one writes a valid initialize response,
  one stays silent to prove the fail-fast message, and detection excludes the
  silent PATH-only candidate.

No eval scorer, OTel, or release metadata files were changed in this slice.
