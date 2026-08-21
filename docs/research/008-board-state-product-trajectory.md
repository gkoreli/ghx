---
title: "Research 008 — Board State & Product Trajectory (post-swarm-1, 2026-08-21)"
date: "2026-08-21"
status: "synthesis"
thread: "sidecar-adoption"
author: "Hermes engineer session (Goga Koreli)"
scope: "board-state snapshot + product trajectory map; updates as swarms land"
builds-on: "ADR-0040, ADR-0041, docs/research/004/005/006/007, docs/spec/evidence-contract/SPEC.md, docs/NORTH_STAR.md"
---

# Board State & Product Trajectory

## Swarm 1 — complete (all merged to mainline, released v2.10.1)

| Card | Delivered | Milestone served |
|---|---|---|
| P2a spec skeleton | 18-clause RFC-2119 evidence contract + clause→code→test map (`docs/spec/evidence-contract/`) | ADR-0040 P2; the SAF standard-play anchor |
| L2 streaming | stderr progress from live.jsonl; first signal ~0.3s (budget <5s); JSON contract byte-identical | ADR-0040 L1/L2; kills felt-silence |
| L3 quota ladder | cache-ledger DEGRADED reports → cheaper-backend retry (`degraded:model`) → typed exit-3 after artifacts flush | ADR-0040 L3; dead asks eliminated |
| A2 usage mining | Research 007: F1–F10 findings + ranked top-10 fix list from 170 episodes + production traces | NORTH_STAR A2/A4 feed |

Orchestrator-layer between swarms: ADR-0041 governance, Research 005/006
(rhythm + UX criteria), FRICTION ops entries, dogfood wiring (recon skill +
global MCP registration).

## Swarm 2 — in flight (dispatched 2026-08-21)

| Card | Worker / effort | Serves |
|---|---|---|
| **P2b conformance suite** (`t_583ddffc`, todo → next) | ghx-worker-high / high | ADR-0041 D4: runnable L1/L2/L3 batteries, every clause covered, first conformance-run artifact |
| **A2-fixbatch** (`t_22a3b9b3`, running) | ghx-worker / medium | Implements Research 007's ranked fixes; twin-anchor rule applies if contracts move |
| **L4a dogfood instrumentation** (`t_fe5a36de`, running) | ghx-worker / medium | Weekly rollup (sessions, p50 latency, ladder firings) making the M5 exit bar measurable |
| **P2c publication pass** (`t_5793c618`, ready, gated on P2b) | unassigned | Standalone-ready spec+suite; L1 claim publishable after run artifact |

## Where the product is progressing

The trajectory maps directly onto the vision's three questions:

1. **"Why must this exist?"** → Now answered by an *object*, not prose:
   the evidence contract exists as a normative spec any harness can read.
   Delegation-without-evidence is the industry default; we own the
   counter-shape with a licensing path (L1 claim available immediately
   post-P2b).
2. **"What stops heavy use?"** → Both top blockers shipped fixes this swarm:
   silence broken (<5s first evidence), death impossible (degradation
   ladder). Remaining blockers are habit and distribution, not capability —
   which is why swarm 2 pairs product work (A2-fixbatch) with measurement
   (L4a) instead of more features.
3. **"When is it a no-brainer?"** → T1 frictionless install: done.
   T2 citable number: parked by design (ADR-0040), receipt-ready later.
   The near-term no-brainer path is T5 (auditability reflex): every card,
   report, and artifact on this board practices the auditable shape we're
   selling.

## Sequencing rule going forward

Product and proof alternate, never parallel-stack: swarm 2 = correctness of
what shipped (P2b validates; A2-fixbatch hardens; L4a measures). Swarm 3
candidates (post-L4a week): P2c publication, install-rails submissions
(needs Goga's account decision), M7/B6 leftovers only if usage data demands
them. Every card still ends in tested code or a cited artifact — no
findings-only-in-chat.
