---
title: "ADR-0037: Persistent Evidence-Shaped Session Memory — Ledger Bounds, Snapshot Identity, Recall"
date: "2026-08-21"
status: "proposed"
parent: ADR-0014.1
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0037. Persistent evidence-shaped session memory

## Status

Proposed. Research grounding: `docs/research/002-persistent-sidecar-session-memory.md`.

## Problem

ADR-0014.1 makes persistence a product contract ("evidence-shaped, not
vibes-shaped"), but four gaps remain (research 002, §1, all path:line-cited):

1. Memory is prompt-injected only — one ≤1500-char ledger block
   (`internal/sidecar/prompt.go:283-299`); no agent-addressable recall.
2. No snapshot identity — `SessionMeta` (`internal/sidecar/session.go:13-62`)
   has no commit/branch field; remote recon floats on moving default branches.
3. Ledger growth is unbounded and silently lossy under the 1500-char render
   budget (`internal/sidecar/ledger.go:247-258`, eviction drops oldest
   commands then paths without recording the drop).
4. Memory identity is backend-dependent — resume rides the ACP session token
   (`internal/sidecar/acprunner.go:121-188`); a backend swap or dead session
   (dogfood friction: dead session `aeb86c3`) loses everything opaque.

## Proposal (M-1..M-4 roadmap, each independently shippable)

- **M-1 Ledger bounds made honest**: explicit eviction policy; dropped
  entries are counted and surfaced in the report's uncertainty, never
  silently discarded.
- **M-2 Snapshot identity**: `commit`/`branch` fields on `SessionMeta`;
  per-ledger-entry repo+ref stamps; reports state which ref evidence was
  pinned to.
- **M-3 Recall tool**: internal `session_memory(query)` tool so the agent can
  pull prior evidence on demand instead of relying on the injected block;
  internal-only per the boundary rule (never a new main-agent MCP surface).
- **M-4 Durability chaos test**: memory must survive daemon restart and
  backend swap; decides flock vs CAS-on-meta (`session.go:148-156` RMW race)
  empirically.

## Measurement

G4 (memory gate, ADR-0016.1) currently scores resume-rate and repeat-read.
Pre-register a memory-reuse sub-score (ledger-hit evidence in follow-up
turns) before any rescore, per the measurement-stack freeze rule.

## Alternatives rejected

- Full semantic/vector memory DB: overkill; evidence ledger is the product
  contract and is recomputable.
- ACP-conversation-only memory: backend-dependent, unauditable, already
  demonstrated fragile (dead sessions in dogfood friction log).

## Consequences / follow-ups

M-1 and M-2 are small runtime diffs + tests; M-3 touches the tool surface
(persona revision required, ADR-0029 pattern); M-4 is a test-infra slice.

## Implementation notes (2026-08-21, M-1 + M-2 shipped)

- **M-1 eviction honesty**: `formatEvidenceLedger`
  (`internal/sidecar/prompt.go`) now appends a visible truncation note
  ("Ledger truncated to fit prompt budget: N older command(s) and M older
  inspected path(s) not shown…") whenever the 1500-char budget forces
  trimming. The agent can no longer assume the ledger view is complete.
  Tested: `prompt_ledger_test.go` (note present under pressure, absent when
  everything fits).
- **M-2 snapshot identity**: `SessionMeta.Commit`/`Branch`
  (`internal/sidecar/session.go`) and `Ledger.Commit`/`Branch`
  (`internal/sidecar/ledger.go`); `UpdateLedgerFromTurn` stamps them from the
  session meta on first sight and never overwrites an existing stamp; unknown
  stays empty (never fabricated). Callers that know the ref (ghx output,
  host integrations) can set it; remote-first sessions keep floating but now
  say so honestly.
  Tested: `ledger_test.go` (stamp + no-overwrite + unknown-stays-empty).
- M-3 (recall tool) and M-4 (durability chaos test) remain open follow-ups;
  both need wider review (tool surface / test infra).

