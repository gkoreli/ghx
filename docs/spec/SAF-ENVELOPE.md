# SAF Envelope Contract — v0.1.0

**Status:** draft v0.1.0 (ADR-0040 P2 — "own the contract, rent the pipe")
**Scope:** the wire-and-artifact contract any Sidecar Agent Framework (SAF)
implementation MUST satisfy so delegations are auditable everywhere. This
spec documents what IS in the reference implementation (`gkoreli/ghx`
mainline @ `c9a9c34`); every normative statement cites file:line. Nothing
here is invented — clauses without a citation are out of scope until they
exist.

Normative language: **MUST** / **SHOULD** / **MAY** per RFC 2119.

---

## 1. The Ask Envelope

A sidecar ask returns exactly one envelope:

```json
{
  "report":    { ...Report... },
  "turn":      { ...TurnResult... },
  "artifacts": { "sessionDir": "...", "traceId": "..." }
}
```

- The daemon JSON-RPC response for `ghx.sidecar.Ask` is
  `AskResponse{report, turn, artifacts}`
  (`internal/sidecar/daemon.go:60-68`, assembled at `daemon.go:339-343`).
  Implementations MUST return all three members; `turn` MAY be null on
  transport-level failure.
- `ArtifactsRef` is `{sessionDir, traceId}` with a human footer line
  (`internal/sidecar/artifacts.go:17-23`, `FooterLine` :36-48). An
  implementation MUST point at durable on-disk artifacts; a report without an
  artifacts pointer is not conformant.

## 2. Report

The structured evidence report (`internal/sidecar/report.go:38-62`):

```json
{
  "answer": "…",
  "verified":   [{"summary":"…","evidence":"…"}],
  "inferred":   [], "unverified": [],
  "relevantFiles": [{"path":"…","reason":"…"}],
  "evidence":   [{"source":"ghx …","summary":"…"}],
  "tierUsed": "tier1",
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx …"],
  "uncertainty": [], "nextReads": []
}
```

### 2.1 Required fields (MUST)

An accepted report carries ALL of (`internal/sidecar/reportsink.go:95-144`):
- at least one `verified` claim with non-empty `evidence`;
- at least one `relevantFiles` entry;
- at least one `commandsRun` entry.

Evidence MUST cite recomputable sources (ghx commands, repo paths, symbols,
line references). Citing `/tmp` scratch, pasted prior-turn text, or any
source not recomputable from ghx-visible evidence is non-conformant
(`internal/sidecar/reportsink.go:173-208`). An answer without evidence is a
hypothesis and MUST be rejected with field-level errors.

### 2.2 Escape hatches

- **BLOCKED**: if investigation could not proceed at all, `answer` MUST start
  with `"BLOCKED: <why>"` and the evidence requirement is skipped
  (`internal/sidecar/reportsink.go:92-95`, validation :116).
- Validation is strict: unknown fields, wrong shapes, empty answers, and
  trailing data are rejected without auto-correction
  (`internal/sidecar/reportsink.go:63-89`).

### 2.3 Bounds

- Answer text cap: 700 chars (`reportsink.go` answer cap, :95 area).
- Field bounds constants (2000 chars / 5 files / 2-sentence claims) live at
  `internal/sidecar/report.go:314-324`; enforcement is flag-only via
  `CheckReportBounds` (`report.go:365-383`, wired `runtime.go:569-575`).
  Conformant implementations SHOULD ship the same bounds as a machine-
  checkable lint, not silently truncate.

### 2.4 schemaVersion

Every stamped path MUST carry `schemaVersion`
(`internal/sidecar/report.go:39-43`; stamping on every production path incl.
BLOCKED and warn-fallback reports: `runtime.go:567-578`, ADR-0039). Consumers
MAY reject envelopes whose major version they do not understand.

## 3. Route Decision

`RouteDecision` (`internal/sidecar/route.go:158-178`) records which session
answered and why: the deterministic R1–R5 cascade (`route.go:21-27`,
`:225-320`) over defaults (score thresholds 0.5/0.25, recency window 30m/7d;
`route.go:42-59`) with recency decay (`route.go:395-406`).

- Every ask response MUST carry the routing decision (`TurnResult.Route`,
  `internal/sidecar/turnresult.go:20-23`) so the caller sees where the answer
  came from *before* building on it.
- Route provenance MUST be emitted on traces (`sidecar.route` events).

## 4. Artifacts Bundle

Each session directory under `~/.ghx/sessions/<slug>/` is an OTLP-shaped
bundle (ADR-0022 D2/D3):

| File/dir | Purpose | Reference |
|---|---|---|
| `traces.jsonl`, `logs.jsonl`, `metrics.jsonl` | OTel telemetry | `internal/sidecar/telemetry/trace.go:40-44` |
| `reports/<turn>-<ts>.json` + `ReportArtifact{actualCommandLedger, traceCommands, rerouted}` | per-turn validated reports | `internal/sidecar/session.go:182-229` |
| `tier-decisions.jsonl` | one recorded tier decision per turn | `internal/sidecar/tier.go:22-24`, `:289-310` |
| `live.jsonl` | realtime turn activity stream | ADR-0022.1 |

Implementations MUST persist telemetry even when emission partially fails
(`internal/sidecar/emit.go:71-93`). Span IDs use OTLP hex-ID form (deviation
documented, `telemetry/trace.go:107-112`).

## 5. Ledger (session memory)

`Ledger` (`internal/sidecar/ledger.go:12-31`) is the cross-turn memory:
repo, scope, Commit/Branch snapshot stamps (ADR-0037 M-2; `ledger.go:101-108`),
commands run, inspected paths, mapped globs.

- The prompt-injected ledger block MUST stay ≤1500 chars with a visible
  truncation note when clipped (`internal/sidecar/prompt.go:287-296`).
- Ledger rebuild MUST be deterministic from turn records
  (`ledger.go:131-160`; reroute parity `reroute.go:82-84`).

## 6. Tier Provenance

`TierDecisionRecord` (`internal/sidecar/tier.go:32-58`) records, per turn:
the pure policy evaluation (signals + observations, recomputable), tierUsed,
its source (`agent` vs `runtime`, ADR-0024.2 D4 backfill :115-125), whether
escalation was used, and clone provenance when visible. Implementations MUST
record one decision per turn including negative decisions and failed turns.

---

## 7. Conformance (normative summary)

An implementation conforms to SAF-ENVELOPE v0.1.x iff:

1. Ask responses carry `{report, turn, artifacts}` with an artifacts pointer (§1).
2. Reports satisfy §2.1 evidence requirements or use §2.2 escape hatches honestly.
3. `schemaVersion` rides every stamped artifact (§2.4).
4. Routing provenance is present and trace-emitted (§3).
5. The session directory satisfies the §4 bundle layout with failure-safe telemetry.
6. Cross-turn memory respects §5 budget + determinism.
7. Tier decisions are recorded per turn and recomputable (§6).

See `CONFORMANCE.md` for the clause→test mapping and gap list.
