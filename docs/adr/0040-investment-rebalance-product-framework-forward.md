---
title: "ADR-0040: Investment Rebalance — Product and Framework Forward, Proof Campaign Parked"
date: "2026-08-21"
status: "accepted"
thread: "sidecar-adoption"
author: "Goga Koreli"
builds-on: "docs/research/004-why-the-sidecar-framework-must-exist.md, docs/research/proof-external-validity.md, docs/NORTH_STAR.md, docs/dogfood/FRICTION.md, ADR-0019.3"
---

# 0040. Investment Rebalance — Product and Framework Forward, Proof Campaign Parked

## Status

Accepted (Goga, 2026-08-21). Supersedes the D1-first ordering in Research 004
§6 for this phase; the proof ladder design is parked, not discarded.

## Context: the market already voted on delegation

Subagent usage in Claude Code, Cursor, Copilot, Hermes, Codex — delegation
is default behavior now, adopted without anyone pre-registering a gate. The
thesis that needed due diligence ("does delegating reconnaissance make
sense?") has been answered by the market. What remains unproven is narrower
and can wait: whether *our* boundary beats a generic subagent by a citable
margin (M1–M4 in Research 004).

Meanwhile the eval campaign machinery — judge calibration (κ ≥ 0.6),
heavyweight runs, formal pre-registered campaigns — sits on the critical path
of nothing users feel, while the product itself still loses to host-native
subagents on latency and to zero competitors on distribution.

The rebalance, stated plainly: **iterate where users are (product speed,
efficiency, usefulness), build what standardizes (the framework contract),
park what persuades later (formal proof campaigns).** Nothing is deleted;
the eval stack demotes from critical path to regression harness.

The eval stack keeps its steering role — it earned it twice: M8 anticipation
was correctly killed by its D1 gate (nextReads recall 0.071), and the 0.580 →
0.908 correctness climb came from mined eval anomalies. Smokes stay mandatory
per change; TRUST ledger hygiene stays a standing obligation.

## Decision

Effort allocation until the next rebalance checkpoint:

| Axis | Share | What moves |
|---|---|---|
| **Product (ghx)** | ~45% | Latency war, dogfood-driven friction burn-down, install rails at scale |
| **Framework (SAF)** | ~35% | Evidence-contract spec extraction + conformance suite |
| **Evals (SAFE)** | ~20% | H7 corpus refresh completion, smokes as regression harness, TRUST hygiene |

### P1 — The latency war (product's first front)

Measured ground truth from FRICTION.md and daemon logs:

- Sidecar asks run **40.7s → 106.2s** wall clock across depth tiers
  (cheap→deep, v2.8.0 dogfood).
- Host-native Explore subagents answer comparable repo questions in
  **~10–30s**.
- The daemon log shows real asks dying on `You've hit your session limit ·
  resets 8:30am` — quota exhaustion surfaces as dead asks, not degraded ones.
- `~/.ghx/sessions/` holds exactly **one session** (`badslugnoslash`, an
  error case) — the founder's own daily loop does not currently flow through
  the sidecar. Dogfooding lapsed after the July week; the heaviest ghx user
  stopped being a sidecar user.

That last fact is the whole rebalance argument in one line: we are polishing
measurement on a product whose owner doesn't use it daily.

Targets (pre-registered here so they're not moved later):

- **L1**: cheap-depth p50 ≤ 15s for repo-scoped questions (warm daemon,
  cached evidence) — parity with Explore-class subagents.
- **L2**: streaming partial answers — first useful evidence visible < 5s
  (live.jsonl already carries turn activity; surface it in ask output).
- **L3**: quota-degradation ladder — on backend quota exhaustion, degrade to
  cached ledger answers / smaller models with explicit labeling, never die.
- **L4**: founder daily-driver bar — every Goga engineering question about
  any repo flows through ghx for two consecutive weeks; friction logged;
  this is also the M5 exit bar finally closed honestly.

### P2 — Framework: extract the contract (SAF's first front)

The `{report, route, artifacts}` envelope, `schemaVersion`, report bounds,
ledger/session semantics, and OTel trace conventions become **a standalone,
versioned, publishable spec** with a conformance test suite any harness can
run against its own implementation. This is how a pattern becomes a standard:
MCP won on spec quality + adoptability while riding commodity transports;
SAF does the same — **own the contract, rent the pipe** (ACP today; below-ACP
harness work happens only when M10 or a pre-registered exit trigger fires,
per NORTH_STAR §Protocols-are-stepping-stones).

Deliverables: spec document under `docs/spec/` (or standalone repo later),
conformance suite derived from existing report/route/artifact tests,
one-line claim: "if your agent speaks this envelope, your delegations are
auditable everywhere."

### P3 — Evals: park the campaign, keep the conscience

Paused until re-triggered (standard-setting need, M10 approach, or external
challenge): judge κ calibration, heavyweight SWE-bench-Live runs, formal
gate-run campaigns. Kept running always: H7 corpus refresh completion,
smokes per change, TRUST ledger hygiene, eval-mining of production traces
(B8 persona revisions feed product fixes, not papers).

## Consequences

- Research 004's T2 (citable number) moves right; if SAF approaches
  standard-setting before the campaign un-parks, un-park it then — the
  design doc survives untouched.
- NORTH_STAR C-workstream rows C4/C7/C8 shift from frontier to parked; B/A
  rows absorb the effort. Milestone table updated separately when status
  changes land.
- Risk accepted: a competitor could publish a benchmark number first. The
  counter is that standards follow usage + spec adoption more than
  leaderboard claims, and our parked design can produce the citable number
  quickly when needed.
- The rebalance is falsifiable like everything else: if L4 fails again
  (founder doesn't adopt daily), the problem is not effort allocation — it
  is the product's usefulness thesis, and that gets attacked directly.

## Implementation Notes

_(filled as slices land; first slice expected: L1/L2 latency work.)_

### 2026-08-21 — P2 first slice: evidence-contract spec skeleton

- `docs/spec/evidence-contract/SPEC.md` — normative skeleton of the
  `{report, route, artifacts}` envelope: 18 clauses (`C1`–`C18`) in RFC 2119
  form covering envelope shape, report schema, evidence requirement
  (ADR-0027 D4), BLOCKED escape hatch, tier visibility, flag-only bounds
  (ADR-0039), strict/lenient ingestion (ADR-0021 D1/D3), route record
  (ADR-0030.1 D7), artifacts pointer (ADR-0018/0026), MCP transport binding
  (ADR-0019.3 D2), failure semantics, session-directory audit set,
  `schemaVersion` semantics (ADR-0039), consumer rules, and three
  conformance levels (envelope-parseable / report-validating /
  audit-preserving). Structure deliberately modeled on the MCP spec layout
  our sidecar recon documented (`modelcontextprotocol/modelcontextprotocol`
  `schema/<revision>/`; prior art cited by session-report path in §7 of the
  spec). Codifies shipped behavior only; proposes nothing normative beyond
  it — deviations found while writing are recorded as spec §9 open
  questions instead.
- `docs/spec/evidence-contract/README.md` — clause-to-code map: every
  clause → enforcing Go symbol (file:line) → pinning test. All citations
  verified against the tree at commit time.
- Open follow-ups (spec §9): JSON Schema twin publication, runnable
  conformance-suite packaging from the pinning tests (the remaining ADR-0040
  P2 deliverable), BLOCKED taxonomy, envelope-level versioning.

## What actually stops heavy use — the honest list (2026-08-21 audit)

Grounded in this machine's real state, not speculation:

1. **Latency** — 40–106s asks vs 10–30s Explore-class subagents (FRICTION.md
   v2.8.0 depth study). The single biggest felt gap. → L1/L2.
2. **Quota fragility** — daemon log shows real asks dying on
   `You've hit your session limit · resets 8:30am`. A specialist that dies
   on quota is worse than a generalist that degrades. → L3.
3. **The founder stopped using it** — `~/.ghx/sessions/` contains exactly
   one session (an error case); last Claude-Code ghx-skill use: 2026-07-03.
   The daily loop reverted to bare CLI/host tools after dogfood week. → L4
   is not a nicety; it is the product-truth test.
4. **No ambient presence** — ghx is not wired into the environments where
   the questions actually arise (no global MCP registration; recon skill
   exists but isn't the default muscle). Adoption requires being *in the
   loop*, not *available*.
5. **Answer freshness/cost unknowns** — no visible cost-per-question or
   cache-hit story for repeated recon on the same repo; sessions persist
   but their economic advantage isn't surfaced to the caller.
6. **Trust transfer is manual** — the evidence report is only as trusted as
   the effort of checking it; streaming receipts (L2) and one-command
   replay are what turn "auditable in principle" into "audited in practice."

Each item maps to a P1/P2 slice; nothing on this list requires the parked
eval machinery.
