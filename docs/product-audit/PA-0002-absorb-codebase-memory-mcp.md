---
title: "PA-0002 — should ghx absorb codebase-memory-mcp? (synthesis)"
date: "2026-07-07"
status: "audit"
audit: "PA-0002"
focus: "should ghx absorb DeusData/codebase-memory-mcp entirely as an internal tool/tier"
in-scope: "what it is; license/feasibility; competitive/moat; absorption shape; tier-2 fit; eval-reality; the counter-case"
out-of-scope: "implementing the absorption; a full ghx code/architecture audit (use arch-audit); rewriting ghx; running the bake-off"
persona: "synthesis (orchestrator / Fable-Opus)"
author: "product-audit skill — Fable synthesis over PA-0002.1–.5, .9"
---

# PA-0002 — Absorb `codebase-memory-mcp`? (synthesis)

Scoped PA-2 audit. **Focus / attack-surface:** should ghx absorb `DeusData/codebase-memory-mcp`
*entirely* as an internal tool/tier (the codemap/repomap swallow, NORTH_STAR P3/M7)? Five
read-only persona-agents (competitive, platform, engineering, eval, veto), each dogfooding ghx to
recon the target; then a fresh no-stake distiller cross-validated the load-bearing facts.

- **Thread:** `PA-0002.1` Competitive (generative) · `.2` Platform/Scale · `.3` Engineering
  Feasibility · `.4` AI PM Pragmatist · `.5` Anti-Bloat Veto · `.9` convergence & decision-grade ledger.

## Judge-step provenance (who judged, how to check)

- **Delegated:** 5 opus persona-agents (read-only), artifacts `PA-0002.1–.5`.
- **Orchestrator self re-derivation:** license **MIT** (`gh repo view` + LICENSE), and the shell-out
  seam confirmed. **Same-family cross-validation** by a fresh no-stake distiller (`PA-0002.9`):
  every load-bearing fact CONFIRMED (MIT; arXiv 83/92; `toolrun.go:53` subprocessAdapter + target
  `src/main.c:9` CLI; ghx's existing tier-2 backends), over-reaches discarded as bones.
- **Shared-prior caution:** the MIT and 83/92 facts are **shared-prior** (one LICENSE, one arXiv
  abstract, all personas read them) — load-bearing but *not* independent corroboration. The
  **genuine independent** convergence is code-grounded (below); `PA-0002.3` (Engineering) is the
  most independent lens.
- **Independent cross-family adjudication — OWED (not run).** Codex/gpt-5.5 was usage-capped
  (twice). No thesis-bearing finding is milestone-citable, and **no governing ADR should merge**,
  until the cross-family check runs (retry Codex, or Goga adjudicates the overlap accounting + gate design).

## Executive summary — the decision

**Absorb the ENGINE, gated — not the product; and today's action is the EVAL, not the integration.**
All five lenses converge (unanimously): absorb `codebase-memory-mcp`'s knowledge-graph **engine** as
*one optional, brain-gated, stateless* Tier-2 shell-out backend (`local:cbm`), **refuse** "absorb
entirely" (its persistent-local-index-by-default posture, daemon/watcher, 11-agent auto-installer,
and in-context injection hook invert ghx's founding tenets), and **do not integrate until a
pre-registered ghx bake-off proves value** — because its additive value is a *token-efficiency
trade, not a quality win* (its own paper: 83% answer quality vs 92% baseline). Your instinct to
**absorb rather than compete is vindicated** — the target literally has no brain ("does not include
an LLM… relies on your MCP client to be the intelligence layer"), which is P3 "swallow the tools"
verbatim. The audit's one correction to *"entirely"*: this is a **product**, not a pure tool like
codemap, so we swallow its **engine under the brain** and reject its **product posture**.

## Decision-grade evidence & cross-reference ledger

Full receipts in `PA-0002.9` (cross-validated) and the persona artifacts. Top rows:

| Claim | Source (recompute) | Strategy (north star / tenet) | External (why / what to absorb) |
|---|---|---|---|
| **It's a brain-less tool → swallow it (P3)** | target README "does **not** include an LLM… relies on your MCP client to be the intelligence layer" | NORTH_STAR *The Moat* / P3: "codemap, ast-grep, repomap all become internal tools under one brain" | [README](https://github.com/DeusData/codebase-memory-mcp) — the whole thesis in their own words |
| **License GO** | LICENSE "Copyright (c) 2025 **DeusData**" — MIT; all transitive deps permissive, zero GPL/LGPL/AGPL (`PA-0002.3`) | AGENTS.md *Open Source Leverage*: "MIT in, MIT out" | [LICENSE](https://github.com/DeusData/codebase-memory-mcp/blob/main/LICENSE) — the hard absorption gate, passed |
| **Shell-out feasible (effort M)** | ghx `internal/sidecar/tier2/toolrun.go:53` `subprocessAdapter` (carries codemap/ast-grep) + `service.go:186,188`; target `src/main.c:9` one-shot `cli <tool> <json>` · plug-in point `internal/sidecar/tier2/cbm.go` (new) | remote-first, escalation explicit — a Tier-2 backend the brain *decides* to invoke | [src/main.c](https://github.com/DeusData/codebase-memory-mcp/blob/main/src/main.c) — the one-shot CLI seam |
| **Reject "entirely"** | target: persistent SQLite index + background watcher + `hook_augment.c` in-context injection + 11-agent installer | NORTH_STAR:207 "**Remote-first… No clone, no local index by default**"; "the main agent's **context is sacred**"; Non-Goal:268 | you can gate a *subprocess* behind tier-2; you can't gate a *product's soul* |
| **Value is a trade, gate it** | arXiv:2603.27277 abstract: **83% quality vs 92% baseline** at ~10× tokens, 19/31 langs; "99%" = a 5-query microbench (README:240) | NORTH_STAR: "**evals are how we know**"; signals-per-token | [arXiv:2603.27277](https://arxiv.org/abs/2603.27277) — vendor's own regression, hidden behind marketing |
| **~80% duplicative** | ghx already ships codemap + ast-grep + native repomap (`service.go:186,188`, `RunRepomap`) — additive ~20% = persistent Cypher call-graph + semantic search | AGENTS.md: "one authoritative owner per concern" | — |
| **Absorb candidate** | the Hybrid-LSP graph engine → `local:cbm` tier-2 backend; the host-wiring *pattern* (not the hook) → M5/B2 | P3/M7; closes ADR-0026 "precise reference" watchlist (HIGH/HIGH) | [aider repomap](https://github.com/Aider-AI/aider) / [repomix --compress](https://github.com/yamadashy/repomix) — complementary absorptions |

## The genuine (code-grounded, independent) convergence — the operational plan

1. **C1 — Absorb the engine as `local:cbm` Tier-2 backend** (`.2` and `.3` independently re-derived
   the *same* seam from ghx code). Shell-out to `cbm cli <tool> <json>` via `subprocessAdapter`; new
   `internal/sidecar/tier2/cbm.go`; wired to the ADR-0024.2 policy router. **No source-fork**
   (40k+ lines of C), **stateless** (watcher/auto-index off), **reject** the injection hook + installer.
2. **C2 — Reject "entirely"** (each persona found a *distinct* remote-first collision). The product
   posture is a Non-Goal; only the subprocess is sanctioned.
3. **C3 — Gate on a pre-registered ghx bake-off** (5 independent gate designs converged). `ghx-sidecar`
   vs `ghx-sidecar+cbm` in the SAFE harness, scoped to the **cross-file / precise-reference / call-path**
   query class (exclude single-file — grep suffices there), on ghx's own tasks/baseline/judge. Gate
   (reuse ADR-0032.1 vector verdict): **correctness non-inferiority (must NOT reproduce the 83-vs-92
   regression) AND SPT superiority** — else reject. Measure on **ghx's** numbers, never the vendor's.

**Sequencing: the gate is UNMET today, so the next action is the EVAL, not the integration.** A
governing **ADR-0024.x** (tier-2 backend bake-off) precedes any build.

## What is actually fine / validated (equal rigor)

- **The absorption machinery already exists** — ghx's `subprocessAdapter` tier-2 seam makes this a
  clean drop-in, exactly the codemap/ast-grep pattern; feasibility is a clear yes.
- **The moat thesis is *demonstrated*, not asserted:** the strongest OSS competitor in code recon is
  a brain-less tool ("relies on your MCP client to be the intelligence layer") — ghx's "brain not
  tools" bet is exactly right, and absorbing it *removes* a competitor's differentiator.
- **ghx dogfooded the recon well** — `explore`/`read`/`tree` carried the architecture read on multiple agents.

## Bones discarded (judged — eat the fish, throw the bones)

Vetoed as over-reach/unsupported: the "deepest structural signal per token / #1 candidate"
superlative (cbm is *cheaper, not better* — 83<92, 19/31); the precise "~80% duplication" fraction
(direction right, number unmeasured); "158 languages" as a *capability* (parse-count; semantic 9–11);
"persistent index justified by B6/M8" (tail-wags-dog — inverts remote-first); the "novel from-scratch
Hybrid-LSP core" (it's an ex-Go→C port, self-described as inspired by gopls/pyright).

## Dogfood findings (route to workstream A — CLI ergonomics)

Surfaced while the agents used ghx to recon the target: `ghx tree --path` errors (grammar is
positional `ghx tree <repo> [path]`); `ghx grep -i` fails (`unknown shorthand flag: 'i'`); **`ghx grep`
exhausts the GitHub code-search quota at ~10/hr** (recon carried by `read`/`tree` at 5000/hr); and
**ghx is GitHub-only** — the decision-relevant arXiv number needed a `curl` fallback (re-confirms PA-0001 **M5**: ghx is structurally blind to non-GitHub sources, incl. its rivals' own docs). These are real agent-experience gaps for absorption-scouting / discovery.

## What was not audited / owed

- **Cross-family (Codex) adjudication — OWED** (capped). No milestone go/no-go, and no ADR merge,
  until it (or Goga) checks the overlap accounting and the gate design.
- The **bake-off itself is not run** (out of scope this round) — it is the recommended *next* action.
- No deep code/architecture audit of the integration (that's `arch-audit`).
- Whether `local:cbm` actually *clears* the gate is genuinely unknown; the vendor evidence leans
  mildly discouraging but is the wrong baseline/tasks/judge, so it bounds but cannot settle it.
