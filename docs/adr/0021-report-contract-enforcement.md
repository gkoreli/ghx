---
title: "ADR-0021: Report Contract Enforcement — Validated Submission, Not Lenient Parsing"
date: "2026-07-05"
status: "accepted"
thread: "sidecar-report-reliability"
author: "Goga Koreli"
---

# 0021. Report Contract Enforcement

## Status

Accepted (founder directive, 2026-07-05). Successor to ADR-0016.7's
parsing fixes; implementation sequenced after the ADR-0020.1 D2 steering
branch (same code area).

## Context

The M4 confirmatory run reported `reports lost to parsing: 4/30 (13%,
known gap, fix on branch)`. Status of that specific gap: **fixed and
merged** — the object/string coercion branch landed on mainline as
2889342 with regression fixtures extracted from those exact four
episodes; those reports would parse today.

But the founder's judgment on the architecture stands: shape-drift
losses are "a naive problem we shouldn't be having." The current
pipeline is open-loop:

1. The agent emits the report as free text inside `<ghx-report>` tags.
2. `ExtractReport` (internal/sidecar/report.go) regex-extracts and
   leniently coerces near-conformant JSON (ADR-0016.7 RC2).
3. On failure the runtime sends **one static retry nudge** that does not
   contain the actual validation error (runtime.go, RC3).
4. After that, a WARN fallback report ships and the exploration's answer
   is lost.

Nothing forces validity; the model never sees *why* its JSON was
rejected; coercion silently masks drift instead of correcting the
producer. The founder's requirement: **the sidecar agent must not be
able to complete until it has produced a schema-valid report, and it
must see the validation error when it fails.**

## Decision

### D1. Report submission becomes a validated tool call (primary path)

The sidecar registers a session-scoped MCP stdio server at ACP session
creation (`NewSessionRequest.McpServers`, currently empty —
internal/sidecar/acp.go) exposing exactly one tool: `submit_report`.
Its input schema **is** the report schema (mcp-go is already in-tree
via `ghx serve`).

- Valid submission → the server persists the report JSON to a sink path
  owned by the calling sidecar process and returns "report accepted —
  you may finish".
- Invalid submission → the tool call **fails with the exact validation
  error(s)** (missing `answer`, `verified[2]` must be an object with
  `summary`, etc.). The agent is still mid-turn, sees the error as tool
  output, and corrects itself in its natural loop. This is the closed
  loop the founder asked for: validation feedback in-band, completion
  gated on validity.
- The persona instructs: an investigation turn is complete only after a
  successful `submit_report` call. Strict validation, **no coercion on
  this path** — the producer fixes its own output.

### D2. The runtime gate

After the ACP turn ends, the runtime reads the sink. If no accepted
report exists, it sends a corrective follow-up in the same session —
now carrying the concrete reason (never a generic nudge) — bounded at
two retries. The WARN fallback report survives only as the
retries-exhausted terminal state; evals already count it as a breaking
anomaly, keeping the failure loud rather than masked.

### D3. Fallback path keeps parsing, gains diagnostics

Adapters/sessions without MCP support fall back to the `<ghx-report>`
text block. `ExtractReport` grows an error-returning variant so the
retry prompt embeds the actual JSON/schema error. Lenient coercion
survives **only on this fallback path**, and every coercion application
is recorded as a soft anomaly (`sidecar_report_coerced`) so drift stays
visible instead of silently absorbed (Visibility and Truthfulness).

### D4. Measurement rule (binding)

This changes the sidecar product, therefore measurement conditions. It
lands on a branch and joins the ADR-0020.1 D2 steering branch in one
pre-registered confirmatory re-run of the ADR-0016.1 gate suite; no
numbers quoted before that run. Expected (recorded per the
verdict-is-a-floor tenet, not citable): `sidecar_report_missing` and
`sidecar_report_block_unparsed` go structurally to zero on MCP-capable
adapters; the anomaly table of the re-run is the proof either way.

## Considered and rejected

- **Keep coercion as the primary mechanism.** Masks producer drift,
  loses reports on shapes the coercer never anticipated — the exact
  failure class the founder called incomprehensible. Rejected as
  primary; retained narrowly per D3.
- **Unbounded retry until valid.** Runaway token cost on a
  fundamentally confused model; bounded retries with loud terminal
  failure preserve both cost discipline and honesty.
- **Adapter-level structured output (JSON mode).** Not a verified
  capability of `claude-agent-acp` session options (`_meta` recon,
  ADR-0020 addendum, found no output-format option). Watch item: if the
  adapter or a future harness exposes schema-constrained output, it can
  replace D1's transport while keeping the same contract.
- **Sidecar-side "repair with a second LLM call".** Adds a scorer-like
  dependency to the product runtime and hides the producer's failure;
  the producer must fix its own report.

## Cross-references

- ADR-0016.7 — the lenient-parsing predecessor (RC2/RC3) this ADR
  supersedes as primary mechanism; its coercion code survives per D3.
- ADR-0020.1 D2 — session-creation steering this build rides on
  (`McpServers` registration, `tools` allowlist must include the
  report sink; sequencing dependency).
- ADR-0016.1 — the gate suite for the D4 confirmatory re-run.
- AGENTS.md "Visibility and Truthfulness" — coercion/retry accounting.
- NORTH_STAR "Evidence, not vibes" — the report is the product; its
  validity cannot be best-effort.
