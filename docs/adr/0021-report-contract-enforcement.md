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

## Implementation Notes (2026-07-05)

Status stays **accepted**. D1–D3 are implemented; **D4 is NOT satisfied** — no
confirmatory re-run has been done, so no performance numbers are claimed here.
The expected outcomes in D4 remain expectations, not results.

### What shipped

- **D1 — submit_report MCP tool.** New hidden command
  `ghx sidecar report-sink --out <path>` (internal/cli/sidecar.go) serves an
  mcp-go stdio server (internal/sidecar/reportsink.go) exposing exactly
  `submit_report`. Its input schema is derived from the `Report` Go type by
  reflection (`reportInputSchema`), and validation runs through
  `DecodeReportStrict` — `json.Decoder` with `DisallowUnknownFields`, a
  trailing-token check, and the shared `ValidateReport` (non-empty answer).
  **No coercion on this path.** A valid submission writes canonical JSON to the
  sink atomically and returns `"report accepted — finish your reply now"`; an
  invalid one returns the exact validation error as an MCP tool error
  (`isError`). The server is registered on the ACP session at both `NewSession`
  and `LoadSession` (internal/sidecar/acp.go `reportSinkMcpServers`, command =
  `os.Executable()`). The persona's `## submit_report` section now describes the
  real tool; `<ghx-report>` is documented as the fallback only.

- **D2 — runtime completion gate.** `Ask` (internal/sidecar/runtime.go) creates
  a runtime-owned temp sink, passes it to every turn, and after each turn
  prefers the strict sink report over any text block (`resolveTurnReport`). On
  absence it falls back to `ExtractReportErr` and sends a corrective follow-up
  carrying the **concrete** reason (`reportRetryPromptWithError`), bounded at
  **two** retries (`maxReportRetries`), replacing the old single static nudge.
  The WARN fallback survives as the retries-exhausted terminal state.

- **D3 — fallback diagnostics + coercion visibility.**
  `ExtractReportErr(text) (*Report, bool, error)` reports the failure category
  (no block / invalid JSON / schema mismatch / empty answer) and whether
  coercion was applied; `ExtractReport` is now a thin wrapper. Coercion is
  confined to this fallback path and surfaced via `TurnResult.ReportCoerced`,
  plumbed to `TurnRecord.ReportCoerced`, and counted by the new soft anomaly
  `sidecar_report_coerced` (internal/sidecar/evals/anomalies.go).

### Adapter/SDK findings that shaped the design (verified against shipped source)

Checked against `claude-agent-acp@0.55.0`
(`~/.npm/_npx/1af57d800c077c6c/.../dist/acp-agent.js`, `dist/tools.js`) and
`@anthropic-ai/claude-agent-sdk` (`sdk.d.ts`):

- Our ACP-registered stdio server is **always merged** into the SDK's
  `mcpServers` (`acp-agent.js:2722-2746, 2822`); the Go `McpServer` stdio
  variant marshals with no `type` field (acp-go-sdk `types_gen.go:2918-2928`),
  matching the adapter's `!("type" in server)` stdio branch
  (`acp-agent.js:2735`).
- `tools: ["Bash","Read"]` is the SDK **built-in** tools base set
  (`sdk.d.ts:1367-1370`); MCP tools are a separate category, so the allowlist
  does **not** hide `submit_report`.
- `strictMcpConfig: true` only ignores *other* MCP config sources
  (`sdk.d.ts:1889-1895`); servers passed via the `mcpServers` option are kept —
  it does **not** block our own registered server.
- **Key gotcha:** the adapter classifies every MCP tool call as ACP kind
  `"other"` (`tools.js` `toolInfoFromToolUse` default case), and the sidecar's
  read-only `denyClient` treats `"other"` as write-shaped and would reject it.
  Fix: add the fully-qualified tool id `mcp__ghx-report-sink__submit_report` to
  session `allowedTools` (session_options.go), which the SDK auto-approves
  without the permission callback (`sdk.d.ts:1305`); the adapter forwards this
  field verbatim via `...userProvidedOptions` (`acp-agent.js:2800`). A narrow
  defensive allowance for this tool was also added to
  `denyClient.RequestPermission`.

### Test coverage (unit + fake-stdio, no live tokens)

- `DecodeReportStrict`: valid / unknown top-level & nested fields / wrong shape
  / empty answer / no payload / trailing data.
- `reportInputSchema`: derived property names match Report json tags,
  `additionalProperties:false`, `required:["answer"]`.
- **Full MCP round-trips proving the closed loop** invalid→error→valid→sink:
  in-process transport (`TestReportSinkServer_MCPRoundTrip`) and the real
  production spawn over stdio (`TestReportSinkStdioEndToEnd`, builds the ghx
  binary and speaks MCP JSON-RPC to `ghx sidecar report-sink`).
- Runtime gate: sink preference over text, concrete-error retry prompt,
  coerced-flag, two-retry bound.
- `sidecar_report_coerced` anomaly detection + aggregation.
- Mock e2e (`TestMockSidecarEpisodeSubmitReportSink`): the scripted agent
  completes turns via the sink through the real `Ask` path.

**Not covered / accepted gap:** a true *in-adapter* MCP round-trip inside the
mock e2e — the fake ACP adapter does not spawn MCP servers, and `os.Executable`
under `go test` is the test binary, not `ghx`. The tool's validation and the
real stdio transport are proven by the two round-trip tests above instead; the
mock e2e proves the runtime sink-preference wiring.

**Post-audit additions (Fable, 2026-07-05):**

- `GHX_REPORT_SINK_EXE` override: under `go test` — exactly how live eval
  episodes run — `os.Executable` is the test binary and the PATH `ghx` may
  be an older release without the hidden command, so the sink would silently
  degrade to the text fallback and the confirmatory re-run would never
  exercise the contract. Live runs must build a fresh ghx and set the env
  var; under `go test` without it the sink is disabled with a loud warning.
- Persona failure-mode probe fixed from `ghx --version` (which fails; the
  live smoke's first tool call proved it) to `ghx version`.

**Live validation (2026-07-05, one strict episode
`ghx-mapengine/ghx-sidecar`, run `20260706-024748`, branch build):**

```sh
go build -o /tmp/ghx-adr21 ./cmd/ghx
GHX_REPORT_SINK_EXE=/tmp/ghx-adr21 GHX_EVAL_STRICT=1 \
GHX_EVAL_AGENT="$(pwd)/scripts/eval-agent-acp.sh" \
go test ./internal/sidecar/evals -tags=agent_e2e \
  -run 'TestEpisodes/ghx-mapengine/ghx-sidecar' -v
```

The real adapter spawned the sink server, the model ran six ghx commands
and finished with a `mcp__ghx-report-sink__submit_report` tool call
carrying the full report; no `<ghx-report>` text block appeared in the
output, zero anomalies, strict PASS, correctness 1.0. The submit-path
contract is proven end-to-end against `claude-agent-acp@0.55.0`;
per-episode counts at scale remain D4's job.

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
