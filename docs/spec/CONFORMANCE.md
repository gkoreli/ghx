# SAF-ENVELOPE Conformance Inventory — v0.1.0

Companion to `SAF-ENVELOPE.md`. Maps each spec clause to the tests that
already pin it in the reference implementation, and lists the unpinned gaps.
Audit commit: `c9a9c34` (mainline). Evidence base verified per file:line by
the 2026-08-21 spec research pass.

## 1. Clause → test mapping

| Spec clause (SAF-ENVELOPE.md §) | Pinned by |
|---|---|
| §2.1 evidence requirements (verified/evidence, relevantFiles, commandsRun) | `internal/sidecar/reportsink_test.go` (Valid, EvidenceRequired cases) |
| §2.1 non-recomputable source ban | `internal/sidecar/reportsink_test.go` (scratch/prior-turn rejection cases) |
| §2.2 BLOCKED escape hatch | `internal/sidecar/reportsink_test.go` (BlockedWithoutWhy / BlockedAccepted) |
| §2.2 strict validation (unknown fields, wrong shape, empty answer, trailing data) | `internal/sidecar/reportsink_test.go` (UnknownField, WrongShape, EmptyAnswer, TrailingData) |
| §2.3 report bounds constants | `internal/sidecar/report_bounds_test.go`; `CheckReportBounds` unit coverage |
| §2.4 schemaVersion stamping (incl. BLOCKED + warn-fallback paths) | `internal/sidecar/report_version_test.go` |
| §2 Report coercion + nextReads normalization | `internal/sidecar/report_test.go` (coercion; NormalizeNextReads :310) |
| §3 route cascade R1–R5 + defaults + decay | `internal/sidecar/route_test.go` (decision table :78, pinned scores :272, marker table :334) |
| §4 artifacts bundle layout | `internal/sidecar/artifacts_test.go` |
| §4 telemetry emission survives failure | `internal/sidecar/emit_test.go:20,156,196,257` |
| §4 OTLP hex-ID form | `internal/sidecar/telemetry/*_test.go` |
| §4 tier-decisions.jsonl per-turn recording | `internal/sidecar/tier_test.go:45-126+` (incl. negative decisions, failed turns) |
| §5 ledger budget + visible truncation | `internal/sidecar/prompt_ledger_test.go`, `prompt_ledger_edge_test.go` |
| §5 ledger rebuild determinism + reroute parity | `internal/sidecar/ledger_test.go`, `reroute_test.go:53,238,271` |
| §6 tierUsed agent-vs-runtime backfill | `internal/sidecar/tier_test.go` (D4 cases) |

## 2. Gap list (clauses NOT yet pinned — future conformance-suite work)

1. **AskResponse JSON-RPC wire shape** (§1): the daemon response envelope has
   no direct serialization test — the struct is exercised only transitively.
   A conformance suite needs a golden wire test (`ghx.sidecar.Ask` response
   byte shape).
2. **schemaVersion bump policy** (§2.4): stamping is tested; the *policy*
   (when major bumps, what consumers may assume) has no test and no spec
   section beyond ADR-0039.
3. **RouteConfigFor override path** (§3): the config-override branch of the
   routing defaults is lightly covered vs the default table.
4. **live.jsonl event contract** (§4): the realtime stream's event shapes
   (`internal/sidecar/live.go:56-94`) are outside the current clause set —
   either spec them (v0.2) or mark explicitly non-normative.
5. **Agent stderr log conventions**: redaction behavior is tested
   (secretEnvValues), but the log file's placement/rotation is not a spec
   clause.
6. **Cross-implementation conformance runner**: everything above is pinned by
   in-repo Go tests; no standalone suite exists yet that an *external*
   implementation can run against itself. That runner is the P2 deliverable
   this inventory scopes.

## 3. Maintenance rule

Any change touching a pinned clause MUST update the pinning test in the same
commit (existing repo rule: tests are the conformance suite until a standalone
runner exists). Any new normative clause in SAF-ENVELOPE.md MUST land with
either a pinning test or an entry in §2.
