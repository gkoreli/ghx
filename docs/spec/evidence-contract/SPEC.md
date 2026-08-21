---
title: "Spec 0001 — The ghx Evidence Contract: the {report, route, artifacts} envelope"
date: "2026-08-21"
status: "draft"
thread: "sidecar-adoption"
author: "Goga Koreli"
scope: "normative skeleton of the evidence-contract spec (ADR-0040 P2): schema, versioning policy, conformance levels. Codifies shipped behavior; proposes no normative changes to it."
builds-on: "ADR-0040 P2, ADR-0019.3 D2, ADR-0039, ADR-0021 D1/D3, ADR-0027 D4, ADR-0030.1, ADR-0024.1, docs/NORTH_STAR.md; MCP spec prior art per ~/.ghx/sessions/modelcontextprotocol-typescript-sdk/reports/"
spec-revision: "1"
---

# 0001. The ghx Evidence Contract

## 1. Problem and scope

An agent that delegates reconnaissance to another agent receives back a wall
of exploration transcript — unless the delegate speaks an evidence contract:
a compact, machine-parseable, auditable answer shape. The Agent Sidecar
Framework's claim (ADR-0040 P2) is that this contract, not the transport
carrying it, is what makes delegations auditable everywhere:

> "if your agent speaks this envelope, your delegations are auditable
> everywhere."

This document is the first normative skeleton of that contract. It extracts
the behavior ghx already ships — the `{report, route, artifacts}` envelope
(ADR-0019.3 D2), the versioned report schema with machine-checked bounds
(ADR-0039), the evidence requirement (ADR-0027 D4), the routing decision
record (ADR-0030.1 D7), and the on-disk audit trail (ADR-0018) — into a
spec any harness can implement and conformance-test against. It follows the
MCP pattern our sidecar recon documented: normative schema first, explicit
versioning policy, conformance levels, and a separate conformance suite
(prior art in §7).

Normative language: **MUST / MUST NOT / SHOULD / MAY** per RFC 2119. Every
normative clause carries an ID (`C1`…`C18`) and maps to enforcing code in
this repository via `README.md` in this directory.

## 2. Definitions

- **Producer** — the specialist agent (sidecar) answering a delegated
  reconnaissance question.
- **Consumer** — the delegating party (main agent, harness, or human tool)
  that receives the envelope.
- **Envelope** — the single JSON object a consumer receives on the contract
  boundary: `{report, route, artifacts}`.
- **Audit trail** — the persisted on-disk record of the producer's work that
  the envelope points at.

## 3. The envelope (normative)

### C1 — Envelope shape

The contract boundary returns **exactly one JSON object** with three keys:

```json
{
  "report":    { ...Report object, §C2... },
  "route":     { ...RouteDecision object, §C8... },
  "artifacts": { ...Artifacts object, §C9... }
}
```

- `report` MUST be present and non-null on every delivered result.
- `route` and `artifacts` MAY be `null` (conditions in §C8/§C9), but the
  keys themselves MUST always be present — a consumer MUST be able to
  unmarshal the envelope without key-existence checks.
- No bytes outside the JSON object are permitted on a machine boundary: no
  prose prefix, no provenance suffix, no markdown fencing.

### C2 — The Report object

The report is the evidence payload. Field names and types:

| Field | Type | Req | Meaning |
|---|---|---|---|
| `schemaVersion` | string | SHOULD | Contract version stamp, §C13. |
| `answer` | string | MUST | The direct answer to the question. |
| `verified` | array of Claim | SHOULD | Claims the producer backed by evidence it actually read; consumers may trust these. |
| `inferred` | array of Claim | MAY | Claims inferred but not directly verified. |
| `unverified` | array of Claim | MAY | Open claims the producer could not confirm. |
| `relevantFiles` | array of FileRef | SHOULD | The files that matter, ranked; bounded by §C6. |
| `evidence` | array of EvidenceEntry | SHOULD | What each gathered source showed; `source` names the command that produced it. |
| `tierUsed` | string | MAY | Highest escalation tier used: `tier0` \| `tier1` \| `tier2` \| `tier3`; MUST be one of these when present. |
| `backendsUsed` | array of string | MAY | Evidence backends used; canonical IDs `remote`, `local:codemap`, `local:ast-grep`, `local:repomap`. A `tier2` report MUST name the `local:*` backend(s) that produced its structural evidence. |
| `commandsRun` | array of string | SHOULD | The commands the producer actually ran. |
| `uncertainty` | array of string | MAY | What remains unconfirmed. |
| `nextReads` | array of string | MAY | Concrete next files to read: one repo-relative path per entry (`path/to/file.go` or `owner/repo:path/to/file.go`); no prose, no line ranges. |

Where `Claim = {summary: string, evidence?: string}` (evidence names what
was observed), `FileRef = {path: string, reason?: string}`, and
`EvidenceEntry = {source: string, summary: string}`.

### C3 — The evidence requirement

An answer without evidence is a hypothesis, not a finding. A report whose
`answer` does not start with `BLOCKED` (§C4) MUST carry ALL of:

1. at least one `verified` claim with a non-empty `evidence` field;
2. at least one `relevantFiles` entry with a non-empty `path`;
3. at least one `commandsRun` entry naming a command actually run.

Evidence citations MUST reference ghx-auditable repo paths or commands —
MUST NOT cite local scratch files (e.g. `/tmp/…`). The `answer` on this path
MUST NOT contain Markdown headings and SHOULD stay within the compactness
bounds of §C6.

### C4 — The BLOCKED escape hatch

When the investigation could not proceed at all, the producer MUST emit a
report whose `answer` starts with `BLOCKED:` followed by the reason. A
BLOCKED report is exempt from §C3 (a blocked investigation has no evidence
by definition) but MUST state why it is blocked. Runtime-produced BLOCKED
reports (turn-cap exhaustion) still carry `schemaVersion` and point at the
partial artifacts, so even failure is auditable.

### C5 — Tier and backend visibility

Which escalation depth answered is never hidden: `tierUsed` and
`backendsUsed` (§C2) expose it on every report. A consumer MAY gate trust on
tier (a `tier0` remote-evidence answer and a `tier2` local-clone answer are
different evidence classes); a producer MUST NOT escalate tiers silently.

### C6 — Compactness bounds (flag-only)

The report is a compression boundary — it replaces the transcript. A
conformant report SHOULD satisfy:

1. rendered JSON size ≤ 2000 characters;
2. at most 5 `relevantFiles` entries;
3. at most 2 sentences in `answer` (sentence count is a punctuation-count
   proxy; advisory by construction).

Bound violations are **flags, never rejections**: the report is delivered
unchanged with the violation recorded next to it. Compactness is a quality
signal; the semantic minimum remains §C2's required `answer`. A consumer
SHOULD surface the flags rather than silently absorb them.

### C7 — Strict and lenient ingestion; drift stays visible

A conformant consumer implementation provides two ingestion paths:

- **Strict path** (producer self-submission): reject unknown fields, reject
  trailing data, enforce §C2 shape and §C3/§C4 semantics. No coercion.
- **Lenient fallback path** (recovery from near-conformant producers): MAY
  wrap bare values into arrays, lift bare strings into `Claim` objects, and
  normalize `nextReads` entries. Every coercion MUST be surfaced as a
  boolean/flag on the result (`coerced`), never silently absorbed.

Both paths enforce the same semantic minimum: non-empty `answer`, and
`tierUsed` ∈ the canonical tier set when present.

### C8 — The Route object

The routing decision that placed the ask, so a consumer can see where an
answer came from before building on it. `route` is `null` when no cascade
decision was made (explicit session pin, or a failure before routing);
otherwise:

| Field | Type | Meaning |
|---|---|---|
| `session` | string | Resolved session name the ask ran in. |
| `source` | string | The cascade rule that fired: `explicit` \| `repo` \| `continuation` \| `overlap` \| `new`. |
| `detectedRepo` | string | `owner/repo` token detected in the question when `source=repo` fired without an explicit repo param. |
| `score` | number | Best overlap score when `source=overlap` (or evaluated). |
| `margin` | number | Best − runner-up when overlap was evaluated. |
| `candidates` | array of `{session, score}` | Top-3 scored sessions when overlap was evaluated. |
| `windowHit` | string | Continuation-window state when the continuation marker test fired. |

Routing is deterministic and recomputable: given the decision record and the
session states at decision time, the route can be re-derived by hand. No
LLM sits in the routing path.

### C9 — The Artifacts object

Pointer to the persisted audit trail behind the answer:

| Field | Type | Meaning |
|---|---|---|
| `sessionDir` | string | Absolute path to the session's artifact directory (contents in §C12). |
| `traceId` | string | 32-char hex trace ID of the ask's root span; omitted when trace emission failed. |

`artifacts` is `null` only when the ask failed before a session directory
existed. The ref is what turns "auditable in principle" into "audited in
practice": the consumer gets a concrete directory it can open or hand to a
judge, never a location it has to guess.

### C10 — Transport binding

The envelope is transport-independent; this revision binds one transport:

- **MCP tool result**: the result text is exactly the envelope JSON — no
  prose outside it, ever. Pinned by tests asserting `json.Unmarshal`
  succeeds on every result shape.
- **Human CLI**: rich text (route line, artifacts footer) is permitted and
  expected; the contract governs machine boundaries only.

### C11 — Failure semantics

- A producer-side hard failure (the ask never ran) MAY surface as a
  transport-level error with no envelope. Every path where a report exists —
  including BLOCKED and degraded answers — MUST deliver the envelope.
- A degraded answer (producer could not produce a real report) MUST still be
  an envelope whose `report.answer` says so and whose `artifacts` points at
  whatever partial trail exists. Dead asks are a contract violation; degraded
  labeled ones are not.

## 4. Audit-trail semantics (normative)

### C12 — The session directory

`artifacts.sessionDir` MUST contain, for every completed ask, the OTel
artifact set and the session state:

| Path | Content |
|---|---|
| `traces.jsonl` | OTel span export for the ask (the audit spine). |
| `logs.jsonl` | OTel log records. |
| `metrics.jsonl` | OTel metric samples. |
| `live.jsonl` | Live turn activity stream (streaming receipts). |
| `meta.json` | Session metadata (name, repo, origin, timestamps). |
| `ledger.json` | Persistent evidence-shaped session memory. |
| `reports/<n>-<ts>.json` | One artifact per turn: the report plus the actual command ledger and trace command ledger. |
| `tier-decisions.jsonl` | Per-turn escalation tier decisions. |
| `agent-stderr.log` | Raw producer stderr. |

Traces are official OTel exactly per spec — custom data rides in attributes
or sibling files, never as a fork of the standard. A conformant
implementation MUST persist this set and MUST expose it through
`artifacts.sessionDir`; replay tooling is desirable but not normative here.

## 5. Versioning policy (normative)

### C13 — `schemaVersion` semantics

- The report carries `schemaVersion` as a string. `"1"` is the contract as
  of 2026-08 (this revision). An empty/absent value means the report
  predates versioning; consumers apply current semantics to it.
- The runtime MUST stamp its build's current version onto every report it
  resolves, regardless of what the producer emitted.

### C14 — Compatibility rules

- The version bumps on **semantic change to any field's meaning** (the
  ADR-0031.2 `nextReads` precedent). Additive fields do not bump the
  version; removals, type changes, and semantic reinterpretations do.
- Strict-path consumers SHOULD reject unknown fields (forward-compatibility
  is negotiated by version, not by silent tolerance).
- Lenient-path consumers MAY tolerate unknown fields but MUST keep the
  tolerance visible (§C7).

### C15 — Consumer rules

- Consumers MUST ignore unrecognized envelope-level keys.
- Consumers MUST treat a missing `report` as a contract violation and
  surface it — not skip the message.
- Consumers SHOULD record `schemaVersion` alongside any downstream judgment
  so historical artifacts stay interpretable.

## 6. Conformance levels (normative)

An implementation (producer or harness) declares one of three levels. Each
level includes all lower ones.

### C16 — Level 1: Envelope-parseable

The implementation returns/parses the C1 envelope with always-present keys,
on every success and degraded path, on the C10 transport binding, with zero
non-JSON bytes on the machine boundary.

### C17 — Level 2: Report-validating

Level 1 plus: strict-path validation of §C2–§C5 (shape, evidence
requirement, BLOCKED escape hatch, tier enum), lenient fallback per §C7 with
surfaced coercion, §C6 bounds evaluated and flagged, §C11 failure
semantics, and the §C13–C15 versioning rules.

### C18 — Level 3: Audit-preserving

Level 2 plus: the §C12 session directory is persisted and exposed via
`artifacts.sessionDir`, with OTel-conformant traces and the per-turn report
artifacts, so any third party can recompute what happened.

## 7. Prior art and attribution

This spec's structure is deliberately modeled on how MCP won adoption
(recon session `~/.ghx/sessions/modelcontextprotocol-typescript-sdk/`,
reports `1-1787347069804.json` and `2-1787347156382.json`):

- **Normative schema in a versioned home.** MCP keeps the machine-readable
  spec per revision — `modelcontextprotocol/modelcontextprotocol`
  `schema/<revision>/` (`schema.ts`, `schema.json` twin, `schema.mdx`,
  `examples/`) — with the prose docs alongside
  (`docs/specification/<revision>/`). This document is the skeleton of that
  home for the evidence contract.
- **Pinned anchors + conformance twins.** The MCP TypeScript SDK vendors
  per-revision spec anchors and locks them with conformance tests
  (`packages/core-internal/test/spec.types.<rev>.test.ts`,
  `test/corpus/schema-twins/manifest.json` pinning upstream commit + sha256)
  plus a nightly refresh workflow that proposes, never auto-merges
  (`.github/workflows/update-spec-types.yml`). The ghx analogue: this repo's
  tests are the conformance suite (`README.md` maps each clause to its
  pinning test), and a future spec-repo split vendors anchors the same way.
- **traceId inside the return value** is Mastra's idea (ADR-0026), done at
  directory level: `artifacts.sessionDir` is the auditable-return
  differentiator.

The evidence contract itself is ours; the spec discipline is stolen openly.

## 8. Non-goals

- **Not a wire protocol.** The contract rides commodity transports (MCP
  today, ACP underneath); "own the contract, rent the pipe" (ADR-0040 P2).
- **Not a judge.** The contract makes evidence auditable; it does not rank
  answer quality. Quality judgment belongs to the eval layer (AGENTS.md
  Visibility tenets) and MUST NOT masquerade as this schema.
- **Not a hosted service.** Local-first holds: the audit trail lives on the
  consumer's disk.

## 9. Open questions (non-normative)

1. **Spec-repo split** — extract to `schema/<rev>/` in a standalone repo
   when a second implementation appears; until then this directory is the
   normative home.
2. **JSON Schema artifact** — publish a machine-checkable JSON Schema twin
   of §C2 (the repo already derives one from the Go type via reflection,
   `internal/sidecar/reportsink.go` `reportInputSchema`); pinning policy per
   §7.
3. **Conformance suite packaging** — derive a runnable suite from the
   pinning tests (ADR-0040 P2 deliverable) so an external harness can test
   its own implementation without this codebase.
4. **BLOCKED taxonomy** — whether BLOCKED reasons become an enumerated
   failure-class vocabulary (ADR-0034) inside the schema, or stay free-text.
5. **Envelope versioning** — whether `schemaVersion` should lift from the
   report to the envelope when route/artifacts semantics evolve
   independently.
