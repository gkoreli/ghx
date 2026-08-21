---
title: "Direction Synthesis — where ghx goes next (post-research fanout, 2026-08-21)"
date: "2026-08-21"
status: "synthesis"
author: "Goga Koreli + Hermes engineer session"
builds-on: "docs/research/saf-adoption-wedge.md, docs/research/codebase-reality-audit.md, docs/research/proof-external-validity.md, docs/NORTH_STAR.md, docs/dogfood/FRICTION.md"
---

# Direction Synthesis — 2026-08-21

Question: Goga is stuck on "where to take ghx — SAF is still experimental and
undecided how to make it a no-brainer to exist out there." Three parallel
research fanouts landed (adoption wedge, codebase reality, proof ladder).
This document synthesizes them into a decision and a build order. It is the
input to the next ADR, not itself an ADR.

## Inputs

| Artifact | Question it answers |
|---|---|
| `codebase-reality-audit.md` | What is built vs aspirational? |
| `saf-adoption-wedge.md` | Why hasn't adoption happened; what wedge would make it a no-brainer? |
| `proof-external-validity.md` | How do evals graduate from self-referential to externally valid proof? |

## Convergence diagnosis

All three artifacts independently converge on one statement:

**ghx has over-served the runtime and internal measurement while under-serving
the two surfaces that create external reality — adoption surface and external
proof.**

Evidence:

- B-stream (runtime) is nearly complete: daemon auto-spawns on first ask with
  daemonless fallback (`daemon.go:465–481`), three entrypoints wired, 555
  sidecar tests green in ~22s full suite. Further runtime polish yields no new
  information or users.
- C-stream proved the thesis *inside its own universe* (M4 THESIS SUPPORTED,
  correctness 0.908 vs 0.931 at 16–25× compression) and correctly halted on
  gates (M8 anticipation D1 FAILED; judge non-citable until κ ≥ 0.6). Nothing
  outside the repo can consume that proof yet.
- The adoption flagship is broken: `ghx serve` defaults to 7 direct CLI tools
  with the recon sidecar opt-in behind `--recon`, and the recon MCP tool
  returns JSON plus a prose suffix (`internal/cli/serve.go:208–221`) — an
  unusable contract for the programmatic consumer the product claims to serve.
- Host-native subagents commoditized context isolation (Claude Code
  Explore/Plan, Cursor). The defensible trio remains **remote/discovery +
  persistent session memory + auditable evidence contracts** — confirmed
  unoccupied externally (ADR-0026 re-check).

## Decision (the reframe)

Stop seeking adoption of "the Agent Sidecar Framework" — frameworks don't get
adopted, products do. The adoptable object is:

> **the MCP tool that answers repo questions with schema-validated evidence
> instead of strings.**

SAF is the architecture, ghx is the product, SAFE is the marketing. Every
next move must shorten the distance between `install` and `first useful
answer`, or widen the proof outsiders can consume.

## Build order

| Phase | Move | Size | Buys | Gate |
|---|---|---|---|---|
| 1 | **No-brainer surface**: recon-first `serve` default (one MCP tool), JSON-only contract (drop prose suffix), doctor-first failure UX, install rails (Claude plugin/MCP one-liner, `npx skills add`) | days | adoption tax → ~zero | ADR first; dogfood-week style verification |
| 2 | **Selection eval**: add generic-subagent arm to C8 host-task episodes (~1 engineer-day design exists), run on ADR-0032.1's registered gates | ~1–2 wks | the quotable number: sidecar ≥ host-native subagent at N× compression — or an honest cheap negative | pre-registered gates; PRELIMINARY labeling preserved |
| 3 | **Heavyweight rung**: SWE-bench-Live post-cutoff tranche stratified by external-exploration subneed; research-then-implement pairs | ~1 mo | outcome-graded external validity | corpus-refresh canaries + judge κ first (TRUST H1/H7) |
| 4 | **Publish SAFE** as public benchmark | gated | distribution (codebase-memory-mcp lesson: benchmark-first framing wins mindshare) | only after H2/H7 audits close |

Phase 2's risk is explicit and accepted: host-native subagents may tie the
sidecar on repo-locked questions. The pivot target if so is already evidenced:
discovery tier + persistent memory + auditability — the trio hosts cannot
commoditize. Learning this for ~$50 of episodes is the cheapest version of the
lesson the north star explicitly values.

## Immediate next actions

1. Draft the Phase-1 ADR (adoption surface flip; numbering/thread decided from
   the ADR index per AGENTS.md rules — likely 0019-family continuation).
2. Sync local mainline with origin (audit found the worktree ~7 commits behind).
3. After ADR acceptance: implementation fanout — (a) serve recon-first default +
   contract fix, (b) FRICTION.md small-batch (ask exit codes, `--depth`
   validation, depth-in-meta.json), (c) skills/install-rail polish.
