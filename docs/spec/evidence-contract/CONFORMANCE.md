---
title: "Conformance Inventory — Spec 0001 clauses C1–C18 mapped to pinning tests"
date: "2026-08-21"
status: "audit"
thread: "sidecar-adoption"
author: "Hermes engineer session (Goga Koreli)"
scope: "clause→test mapping and gap list for Spec 0001; audit at mainline d699e30"
builds-on: "docs/spec/evidence-contract/SPEC.md (C1–C18), ADR-0040 P2, ADR-0041"
---

# Conformance Inventory — Spec 0001 (C1–C18) vs pinning tests

Every normative clause in `SPEC.md` should be pinned by a test in the
reference implementation. This inventory maps clause → pinning tests
(audit at mainline `d699e30`) and lists unpinned gaps. Maintenance rule:
a change touching a pinned clause updates its test in the same commit; a new
clause lands with either a pinning test or an entry in §2.

## 1. Clause → pinning tests

| Clause | Pinned by |
|---|---|
| **C1** envelope shape (`{report, route?, artifacts}` keys always present) | `internal/sidecar/daemon_test.go` (AskResponse round-trip); wire-shape golden test still missing — see §2.1 |
| **C2** Report object shape + strict/lenient ingestion | `internal/sidecar/reportsink_test.go` (UnknownField, WrongShape, EmptyAnswer, TrailingData); `report_test.go` (coercion) |
| **C3** evidence requirement | `reportsink_test.go` (Valid / EvidenceRequired cases) |
| **C4** BLOCKED escape hatch | `reportsink_test.go` (BlockedWithoutWhy / BlockedAccepted) |
| **C5** tier + backend visibility | `reportsink_test.go` (TierUsed enum); `tier_test.go` (tierUsed agent-vs-runtime backfill) |
| **C6** compactness bounds (flag-only) | `report_bounds_test.go`; `report.go` CheckReportBounds unit coverage |
| **C7** drift stays visible (coercion surfaced) | `report_test.go` coercion cases |
| **C8** Route object | `route_test.go` decision table (:78), pinned scores (:272), marker table (:334) |
| **C9** Artifacts object | `artifacts_test.go`; FooterLine coverage |
| **C10** transport binding (daemonless + daemon parity) | `daemon_test.go`; `acp_resilience_test.go`; H8-style meta wire pins |
| **C11** failure semantics | `emit_test.go:20,156,196,257` (emission survives failure); `acp_resilience_test.go` |
| **C12** session directory bundle | `artifacts_test.go`; `telemetry/*_test.go`; `session_test.go` (reports/<turn>-<ts>.json) |
| **C13** schemaVersion semantics | `report_version_test.go` |
| **C14** compatibility rules | partially pinned by old-JSON compat tests in `ledger_test.go` (ADR-0037 M-2 sweep) — see §2.2 |
| **C15** consumer rules | no dedicated test — consumers are external by definition; covered indirectly by §C7 ingestion tests |
| **C16–C18** conformance levels | classification of C1–C15; level assignment is a declaration, not a testable unit |

Additional pins added 2026-08-21 (post-SPEC authoring):

- **DEGRADED escape hatch** (ADR-0040 L3, `wt/l3-quota-ladder`):
  `reportsink_test.go` TestValidateReportEvidenceDegraded — parallel to C4,
  candidate for promotion into SPEC.md as §C4b on the next spec revision.
- **Quota exit mapping**: `cli/sidecar_test.go` TestAskQuotaExitMapping /
  TestQuotaErrorNeedsExplicitWrap.

## 2. Gap list (unpinned — future conformance-suite work)

### 2.1 No direct AskResponse JSON-RPC golden test

The daemon response envelope is exercised only transitively. A standalone
conformance suite needs a byte-level golden wire test of `ghx.sidecar.Ask`
responses (Spec 0001's own "pinned anchors" prior-art point).

### 2.2 schemaVersion bump policy

Stamping is pinned (§C13); the *policy* — when major bumps happen, what older
consumers may assume (§C14) — has no executable check.

### 2.3 RouteConfigFor override path

The config-override branch of routing defaults is lightly covered vs the
default cascade table.

### 2.4 live.jsonl event shapes

The realtime stream (`internal/sidecar/live.go:56-94`) is outside the current
clause set. Either spec it in v0.2 or mark explicitly non-normative.

### 2.5 External conformance runner

All pins above are in-repo Go tests. The P2 deliverable this inventory scopes:
a standalone runner any third-party implementation can execute against itself,
declaring a C16/C17/C18 level.
