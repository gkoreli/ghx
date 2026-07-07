---
title: "AA-0002 Charter — the reusable-core boundary: SAF-as-infra & the evals↔product shared kernel"
date: "2026-07-07"
status: "audit-charter"
thread: "AA-0002"
scope: "internal/sidecar (SAF runtime), internal/sidecar/evals (SAFE), internal/sidecar/telemetry — the boundary between them"
audit: "arch-audit (scoped, multi-persona, adversarial)"
author: "Fable"
---

# AA-0002 — the reusable-core boundary

## Trigger (why now — this is not a periodic sweep)

The north star creates **two things**: the **Agent Sidecar Framework** (a mental
model "eventually adopted as reusable infrastructure") and **ghx** (its proof).
NORTH_STAR §"Sidecar Product Capabilities" asserts the SAF runtime (SAF) and the
eval machinery (SAFE) **"share one trace/artifact infrastructure — a trace is a
trace whether it came from an eval episode or a real question; the OTel capture
layer is configurable, modular, and reused across both"** (ADR-0022's extraction
rule exists for exactly this). ADR-0036 just established the **Runner port** as one
config-selectable reusable boundary. Two concrete triggers converge: (a) the
framework half of the vision needs a real reusable boundary, not an aspiration;
(b) a capability-boundary decision is live (what else, beyond the runner port,
is genuinely shared vs. duplicated). Goga named this scope explicitly.

## The question the run must answer

Is the core shared between `internal/sidecar/evals` (SAFE) and the `internal/sidecar`
runtime (SAF) a **genuine DDD Shared Kernel** (small, deliberately coordinated,
context-tax-reducing) — or **accidental duplication drifting apart**? What, if
anything, should be consolidated/extracted to make the SAF a cleaner reusable
boundary — **without** extracting a framework before a second consumer proves it?

## Ranked quality attributes (the utility tree — personas target these)

1. **Real-shared-kernel vs. duplication** — is the eval↔runtime core actually one
   owner, or two copies silently diverging? (highest weight)
2. **Context-tax reduction** — per `research/05 §5.2`, a shared core earns its
   coordination cost only when it *reduces fleet-wide context tax*, not merely
   removes duplication; a recurring concept is often better a *named type* than a
   dependency (copy vs. depend vs. name).
3. **Replaceability / framework-extractability** — could the SAF boundary be lifted
   toward reusable infra without a rewrite (NORTH_STAR "the framework outgrows ghx —
   *later*"), and what coupling would force a copy-paste fork of a second domain?

## In / out of scope

**In:** `internal/sidecar/**` (the SAF runtime, incl. the runner port, daemon,
session, telemetry) and `internal/sidecar/evals/**` (SAFE), and the seam between
them — the shared trace/artifact/report/telemetry substrate.
**Out:** `internal/ghx` core recon (the CLI tool layer — a different scope), the
CLI/MCP frontends, product features, security. Reading them as evidence is fine.

## Binding constraints (from the skill + repo tenets)

- **Frozen measurement stack** — `internal/sidecar/evals` scoring/gates/detectors
  must not be proposed for drive-by change; behavior-preserving ≠ measurement-
  preserving (any evals-reachable change is measurement-touching → pre-registered
  eval ADR). This scope is *especially* exposed to it — flag, don't refactor.
- **North-star filter + YAGNI** — the mandatory adversary argues the *inline/
  do-nothing* side; "do nothing + watch-list" is a valid, first-class outcome. Do
  NOT recommend extracting a framework before a second consumer exists.
- **Agent-legibility** — maintainability = the minimal legible edit unit; both
  god-files and over-fragmentation fail concern-locality (`research/05`).
- Every claim `file:line`; ADR-style artifacts threaded `AA-0002.k`; land ff-only.

## Personas (5) — spanning the axes, ≥1 adversarial, Codex metrics

`.1` reusable-core extraction (lead) · `.2` Go architecture & boundaries ·
`.3` domain modeling · `.4` YAGNI skeptic (adversarial) ·
`.5` complexity/duplication metrics (Codex/gpt-5.5 — independent family, hunts
evals↔product duplication). Distillation → `AA-0002.9-distilled`; then Fable judges.
