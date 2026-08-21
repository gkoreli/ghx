---
title: "ADR-0039: Report Contract Validation — schemaVersion + Machine-Checked Persona Bounds"
date: "2026-08-21"
status: "proposed"
parent: ADR-0016.2
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0039. Report contract validation

## Status

Proposed. Research grounding:
`docs/research/003-report-contract-context-boundary.md`.

## Problem

The report is the workflow boundary — "the report replaces the transcript"
(`internal/sidecar/prompt.go:161-166`) — but its contract is enforced only by
persona prose:

- No machine validation: a >2000-char or >5-file report silently passes;
  nothing distinguishes "compact because compliant" from "compact because
  truncated".
- No schema version: ADR-0031.2 already changed `nextReads` semantics with no
  version marker to key drift off.
- G3 (compression, `gates.go:20-21`, factor 0.35) measures char ratios but
  cannot see contract violations; a verbose-report failure mode would inflate
  the sidecar baseline rather than get caught.

Measured context: compression results are strong on the deterministic layer
(smoke ratio 0.127; preliminary r2 0.048 at 72/90 episodes) with no
correctness loss (smoke G1 = 1.0 both profiles; r2 G1 sidecar 0.920 vs ghx
0.913). Per AGENTS.md this is a conservative floor; contract validation
protects that floor as the framework standardizes.

## Proposal (two additive slices)

1. **`schemaVersion` field** on `Report` JSON. Version bumps on semantic
   change (the 0031.2 precedent); normalizers stay lenient/visible.
2. **`ValidateReportBounds`** invoked after parse in the runtime: answer ≤2
   sentences, report <2000 chars, ≤5 relevant files, one-line evidence
   entries. Violations are recorded in the episode artifact and classified
   via ADR-0034 failure classes — flagged, never silently dropped. Whether a
   violation also fails the episode (changing gate denominators) is decided
   at review; default proposal is flag-only.

## Alternatives rejected / deferred

- Token-based accounting instead of chars: non-deterministic across
  tokenizers unless the estimator is committed+versioned; defer with an
  explicit caveat in gate docs.
- Main-agent-in-the-loop harness and judge-scored actionability: real gaps,
  but they belong to the host-task-eval track (ADR-0032) and the judge
  calibration protocol (AGENTS.md), not this slice.

## Consequences / follow-ups

Small runtime diff + unit tests + golden fixture update. No scorer changes;
no rescore of historical runs. Validation lives in `internal/sidecar`
(product truth); the eval kernel consumes its output rather than duplicating
it.

## Implementation notes (2026-08-21, shipped)

- **`SchemaVersion`** added to `Report` (`internal/sidecar/report.go`);
  `ReportSchemaVersion = "1"` stamped by the runtime on every resolved report
  (`runtime.go` `resolveReportWithRetry`). Empty means pre-versioning.
- **`CheckReportBounds`** implemented in `report.go`: deterministic, flag-only
  evaluation of the persona bounds — rendered JSON >2000 chars
  (`MaxReportChars`), relevantFiles >5 (`MaxRelevantFiles`), answer sentences
  >2 (`MaxAnswerSentences`, punctuation-count proxy documented as advisory).
  Violations render a stable one-line summary via `ReportBoundViolations`.
- **Runtime wiring**: `TurnResult.ReportBoundViolations`
  (`turnresult.go`) records the summary when violated; the report itself is
  never altered or rejected. Eval turn records can surface it as a soft
  anomaly in a follow-up slice.
- Tested: `report_bounds_test.go` (clean/oversize/too-many-files/sentences/
  nil); full build/vet/test green. The pre-existing ledger-budget test caught
  that M-1's truncation note must live inside the 1500-char budget — fixed by
  reserving room during trimming (see ADR-0037 notes).

