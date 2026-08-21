---
title: "Evidence Contract — clause-to-code map"
date: "2026-08-21"
status: "active-map"
thread: "sidecar-adoption"
author: "Goga Koreli"
scope: "maps every normative clause of SPEC.md to the Go code path and pinning test that enforces it in this repo; update alongside any contract change."
builds-on: "docs/spec/evidence-contract/SPEC.md, ADR-0040 P2, ADR-0019.3, ADR-0039"
---

# Evidence Contract — clause → code map

Each row: the SPEC.md clause, the enforcing code in this repository (the
product truth), and the test that pins it. If a clause's code path moves,
update this file in the same commit. Line numbers are as of the spec
skeleton commit; the symbol names are the durable anchor.

| Clause | Requirement | Enforcing code | Pinning tests |
|---|---|---|---|
| C1 | Envelope = one JSON object, `{report, route, artifacts}`, always-present keys, zero non-JSON bytes | `ReconResult` type — `internal/sidecar/reconresult.go:28`; envelope construction — `NewReconResult` `internal/sidecar/reconresult.go:43`; MCP emission — `internal/cli/serve.go:251-260` | `TestNewReconResultShape`, `TestNewReconResultNilTurnAndEmptyArtifacts`, `TestReconResultKeysAlwaysPresent` — `internal/sidecar/reconresult_test.go`; envelope-unmarshal assertions — `internal/cli/serve_test.go:203` |
| C2 | Report schema (fields, types, tier enum, backend IDs) | `Report` struct — `internal/sidecar/report.go:38-62`; canonical tiers — `ParseTier` (`internal/sidecar/tier_value.go:25`), canonical backend IDs on `report.go:50-57`; model-facing field docs — `internal/sidecar/reportsink.go:250-262` | typed unmarshal across report tests — `internal/sidecar/report_test.go`; tier validation — `internal/sidecar/tier_test.go` |
| C3 | Evidence requirement for non-BLOCKED reports | `ValidateReportEvidence` — `internal/sidecar/reportsink.go:108-144`; scratch-source denial — `citesScratchSource` `reportsink.go:195-208`; advertised in submit_report JSON Schema description — `reportsink.go:235` | evidence-requirement cases — `internal/sidecar/reportsink_test.go` |
| C4 | BLOCKED escape hatch (reason required, evidence-exempt) | prefix check + reason enforcement — `internal/sidecar/reportsink.go:91-119`; runtime BLOCKED synthesis on wrap-up failure — `internal/sidecar/runtime.go:537-541` | blocked-answer cases — `internal/sidecar/reportsink_test.go` |
| C5 | Tier/backend visibility; no silent escalation | `tierUsed` validation — `ValidateReport` `internal/sidecar/reportsink.go:215-226`; canonical tiers — `ParseTier` `internal/sidecar/tier_value.go:25`; tier decisions persisted per turn — `recordTierDecision` call `internal/sidecar/runtime.go:584` (writes `tier-decisions.jsonl`) | `internal/sidecar/tier_test.go` |
| C6 | Compactness bounds ≤2000 chars / ≤5 files / ≤2 sentences, flag-only | constants — `MaxReportChars`, `MaxRelevantFiles`, `MaxAnswerSentences` `internal/sidecar/report.go:314-324`; evaluator — `CheckReportBounds` `report.go:368-383`; flag-only wiring — `runtime.go:571-575` onto `TurnResult.ReportBoundViolations` (`internal/sidecar/turnresult.go:47-52`) | clean/oversize/too-many-files/sentences/nil — `internal/sidecar/report_bounds_test.go` |
| C7 | Strict + lenient ingestion; coercion surfaced, never silent | strict submission — `DecodeReportStrict` `internal/sidecar/reportsink.go:63-89` (DisallowUnknownFields + trailing-data check); lenient fallback — `ExtractReportErr` `internal/sidecar/report.go:103-137` with `coerceReportJSON` `report.go:197-213`; nextReads normalizer — `NormalizeNextReads` `report.go:156-178`; shared semantic minimum — `ValidateReport` `reportsink.go:215-226`; surfaced flags — `TurnResult.ReportCoerced/ReportRetried` `turnresult.go:40-46` | strict-path cases — `internal/sidecar/reportsink_test.go`; coercion cases — `internal/sidecar/report_test.go`; version stamping — `internal/sidecar/report_version_test.go` |
| C8 | Route object: cascade source enum, recomputable decision record | `RouteDecision` — `internal/sidecar/route.go:161-178`; deterministic R1–R5 cascade — `RouteQuestion` `route.go:229-320` (no LLM); provenance rendering — `Provenance()`/`Line()` `route.go:186-210`; marker-table versioning — `route.go:61-63` | decision-table pins — `internal/sidecar/route_test.go` ("the pinned specification"); reroute recovery — `internal/sidecar/reroute_test.go` |
| C9 | Artifacts pointer: sessionDir (+ traceId) on every response surface | `ArtifactsRef` — `internal/sidecar/artifacts.go:17-23`; ref construction — `newArtifactsRef` `artifacts.go:28-35`; human footer — `FooterLine()` `artifacts.go:36-45`; envelope field — `internal/sidecar/reconresult.go:15-21,36` | nil-turn/empty-artifacts shapes — `internal/sidecar/reconresult_test.go:73+` |
| C10 | MCP transport: result text is exactly the envelope JSON; human CLI exempt | MCP handler returns only `json.Marshal(NewReconResult(...))` — `internal/cli/serve.go:251-260`; tool surface — `ReconMCPTool()` `internal/sidecar/recontool.go:28` (frozen eval identity per ADR-0032.1 S3) | `json.Unmarshal` on every recon shape — `internal/cli/serve_test.go:203`; default-mode registration — `internal/cli/serve_test.go` |
| C11 | Failure semantics: degraded labeled answers, never dead asks | wrap-up recovery BLOCKED report — `internal/sidecar/runtime.go:530-543`; warn-no-report degraded answer — `runtime.go:577-579`; session recreation — `TurnResult.SessionRecreated` `turnresult.go:61` | wrap-up/session-recovery cases — `internal/sidecar/acp_resilience_test.go`, `internal/sidecar/runtime_test.go` |
| C12 | Session directory artifact set (OTel traces/logs/metrics + ledger + per-turn reports) | artifact set writer — `internal/sidecar/telemetry` package (OTel capture shared by runtime and evals); per-turn report artifacts — `SaveTurnReportArtifact` call `internal/sidecar/runtime.go:592` (`persistTurn`, runtime.go:581); session dir resolution — `sessionDir` `internal/sidecar/session.go:73` | live-emission proof — `internal/sidecar/live_test.go`, `emit_test.go`; replay recipe — ADR-0018 |
| C13 | `schemaVersion` stamped `"1"` on every resolved report | constant — `ReportSchemaVersion` `internal/sidecar/report.go:315-317`; stamping — `internal/sidecar/runtime.go:537,572,578` | `internal/sidecar/report_version_test.go` |
| C14 | Version bumps on semantic change; additive fields don't bump | policy text lives here + ADR-0039; bump precedent — `NormalizeNextReads` semantics change under ADR-0031.2 (`internal/sidecar/report.go:143-155`) | review-time obligation; no automated check by design (policy, not mechanism) |
| C15 | Consumer rules: ignore unknown envelope keys, missing report is a violation | consumer-side contract statement (this spec + recon skill guidance — `skills/ghx-recon/SKILL.md`); enforced for our own consumers by the typed envelope decode in `internal/cli/serve_test.go:203` | same |
| C16–C18 | Conformance levels 1–3 | Level 1 = envelope paths above (reconresult.go, serve.go); Level 2 = validators above (reportsink.go, report.go); Level 3 = artifact persistence above (telemetry, artifacts.go) | union of the rows above |

## How to verify conformance today

```bash
go build -o ghx ./cmd/ghx && go test ./internal/sidecar/... ./internal/cli/...
# live envelope smoke (Level 1): result text must parse as one JSON object
ghx serve &   # then call the recon tool over stdio MCP; assert json.Unmarshal succeeds
```

## Maintenance rule

This map is part of the evidence-contract deliverable (ADR-0040 P2). Any
commit that moves a symbol cited here updates the row in the same commit.
The symbol names are the durable anchors; line numbers are convenience.
