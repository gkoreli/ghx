---
title: "ADR-0041: Evidence-Contract Spec Governance — Ownership, Evolution, and Conformance Policy"
date: "2026-08-21"
status: "accepted"
parent: ADR-0040
thread: "sidecar-adoption"
author: "Goga Koreli"
builds-on: "docs/spec/evidence-contract/SPEC.md, docs/spec/evidence-contract/README.md, ADR-0019.3, ADR-0039, ADR-0040 P2, docs/research/005-north-star-working-rhythm.md"
---

# 0041. Evidence-Contract Spec Governance

## Status

Accepted (Goga, 2026-08-21). The spec skeleton (SPEC.md rev 1) landed via
kanban task t_a659e4a8 and is merged to mainline. This ADR fixes how the
spec is owned, how it evolves, and what acceptance means for the remaining
P2 slices — written *after* seeing the skeleton's shape but *before* the
conformance-suite slice starts.

## Context

ADR-0040 P2 committed to extracting the evidence contract into a standalone,
versioned, publishable spec with a conformance suite. The skeleton now
exists: 18 RFC-2119 clauses (C1–C18), a clause→code→test map, and three
conformance levels (L1 envelope-parseable, L2 report-validating, L3
audit-preserving). What was missing is governance: who changes the spec and
how, what keeps it honest against the code, when external implementations
can claim conformance, and what acceptance means for the next slices.
Without this ADR, the spec drifts from product truth or freezes into
documentation theater — both failure modes observed in other projects.

## Decision

### D1. Ownership and the twin-anchor rule

The spec is **product truth at the boundary**; `internal/sidecar` remains
implementation truth in the core. Neither outranks the other: they are
twins joined by the clause→code map (`docs/spec/evidence-contract/README.md`).
Rule: any commit that changes envelope/report/route/artifacts behavior in
code **MUST** update SPEC.md clauses and README map rows in the same commit,
and vice versa — a spec change without an implementation change must land as
a proposed revision (D3), never silently. Symbol names are the durable
anchors; line numbers are convenience (README already states this).

The map is verified by CI: a test fails if any code symbol cited by a
README row no longer exists (prevents silent rot; line-number drift alone
does not fail).

### D2. Revisions follow semver discipline on `spec-revision`

- **Patch (rev x.y.Z)** — clarifications, citation fixes, typo-level. No
  clause semantics change. Orchestrator may land directly.
- **Minor (rev x.Y)** — new clause, new conformance level, relaxation of a
  SHOULD/MAY. Requires an ADR note (may be an implementation-notes append
  to this ADR's thread) before merge.
- **Major (rev X)** — breaking change to any MUST clause (C1–C15) or to the
  versioning policy itself. Requires: (1) a pre-registered ADR with the
  compatibility matrix, (2) a shipped migration path for consumers of the
  previous major (lenient-ingest guidance per C7/C15), (3) `schemaVersion`
  bump in lockstep per C13. Major revisions are expected to be rare; C13's
  existing policy covers report-level drift, this covers spec-level drift.

### D3. Conformance claims — what licenses them

An implementation (ghx today; third parties later) MAY claim:

- **Level 1** after passing the public Level-1 check: every response on the
  transport parses as exactly one JSON object with always-present keys
  (C1, C10). ghx claims L1 today — pinned by `internal/cli/serve_test.go`.
- **Level 2** after passing the conformance suite's validator battery
  (C2–C7, C11, C13–C15). License: the P2b suite (next slice) must run
  green against mainline before any L2 claim is published.
- **Level 3** after passing the audit-preserving checks (C8–C9, C12): a
  third party can recompute the producer's work from the artifacts pointer
  alone. ghx claims L3 informally (OTel-conformant trails since ADR-0018);
  formal claim waits for the suite's L3 battery.

Claims are phrased as "conformant at Level N, revision R" and link the exact
suite run. No level claims without a committed run artifact — same
truthfulness bar as evals (AGENTS.md).

### D4. Acceptance criteria for the remaining P2 slices

**P2b — conformance suite** (next kanban card):

1. Suite lives at `docs/spec/evidence-contract/conformance/` as a runnable
   program (Go, stdlib + testify-free like the repo) taking an endpoint or
   fixture directory — not coupled to ghx internals.
2. Batteries per level: L1 (envelope parse × success/BLOCKED/error shapes),
   L2 (validator battery incl. bounds flag-only behavior, strict/lenient
   coercion visibility, schemaVersion rules), L3 (artifacts pointer
   resolves; trace replay recipe executes).
3. Suite runs green against ghx mainline in CI; result committed as the
   first conformance-run artifact under `docs/spec/evidence-contract/runs/`.
4. Every battery cites its SPEC.md clause IDs; every clause C1–C18 is
   covered by at least one battery.

**P2c — publication pass** (after P2b):

1. Spec + map + suite extracted publishable (standalone-friendly paths,
   no internal-only references required to read them).
2. One-sentence claim registered: "if your agent speaks this envelope, your
   delegations are auditable everywhere" with a link to the latest
   conformance-run artifact.

### D5. Non-goals

- No spec-by-committee process: Goga owns normative decisions; workers
  propose via PR/comment.
- No wire-protocol authorship: the contract binds at tool-result/payload
  level only; transports stay commodity (ACP/MCP/HTTP) per NORTH_STAR.
- No conformance certification program yet — that is a T4-era (distribution
  inversion) decision, explicitly deferred.

## Consequences

- Spec and code move in lockstep; the CI symbol-check makes drift loud.
- Third parties get a real adoption path: implement to SPEC.md, run the
  suite, claim a level with receipts — the standard-setting wedge (T2/T4).
- Cost: every boundary-touching PR now owes spec+map edits (~minutes);
  accepted as the price of owning a standard.
- Falsifier: if after P2b no external implementation attempts even L1
  within a quarter of publication, the "standard" framing gets revisited
  honestly (the contract may be right but adoption economics wrong).

## Implementation Notes

- 2026-08-21: skeleton landed (t_a659e4a8 → b5bf404). CI symbol-check and
  P2b suite are open work items on the board.
