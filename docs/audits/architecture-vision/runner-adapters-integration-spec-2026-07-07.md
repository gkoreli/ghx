---
title: "Runner-adapters integration spec — codex-acp (Phase C3) and claude-sdk (Phase D)"
date: "2026-07-07"
status: "design spec / builder handoff"
author: "background research + design worker (runner-adapters)"
scope: "De-risk ADR-0036 Phase C3 and Phase D by nailing the exact integration details the runner-port audit left at the design level. No code changed; only this document written."
---

# Runner-adapters integration spec: codex-acp + claude-sdk

This document turns the two *hardest-to-build* rows of the runner-port design
(ADR-0036 D1; `docs/audits/architecture-vision/runner-port-pluggability-2026-07-07.md`)
into a builder-ready integration spec:

- **Phase C3 — the `codex-acp` adapter** (ADR-0036 goal a: add a runner via config).
- **Phase D — the in-process `claude-sdk` adapter** (ADR-0036 goal b, P4: replace
  ACP with an owned in-process loop over `anthropic-sdk-go`).

For each adapter it gives (1) the config shape, (2) a field-by-field
`SteeringSpec → runtime knob` mapping, (3) how each `EventSink` event decodes
from that runtime's stream, (4) how each `Outcome`/`FailureClass` is detected,
(5) the report-sink and persona placement, and (6) the concrete unknowns/risks
a builder must resolve. The port's neutral `SteeringSpec`/`EventSink`/`Outcome`
types are the target contract as defined in the runner-port audit
(§"The Runner port"); this document does not redefine them.

The measurement-facing target is unchanged: an adapter's only job on the way out
is to repopulate the same neutral `TurnResult`/`ToolCallTrace`
(`internal/sidecar/turnresult.go`, imports only `time`) and to return the same
five `FailureClass` values, so `evals/anomalies.go` never learns which runtime ran.

---

## Evidence contract

**Method.** Read-only. Every ghx `file:line` was read in this worktree
(branch `agent-a92fa3c6c5a123840`); external facts are text-only research
(WebFetch/`gh`/`curl` — no browser, AGENTS.md Tool Economy). Codex config
enum values were extracted from the machine-generated JSON Schema with `jq`.

**ghx sources read (this worktree):**
- `internal/sidecar/acp.go` (markers `:41-106`, `RunTurnOptions` `:157-197`, `RunTurnWithOptions` `:218-315`, `CheckACPHandshake` `:340-387`).
- `internal/sidecar/session_options.go` (`SessionOptions` `:12-97`, `BuildSessionMeta` → `_meta.claudeCode.options` `:194-230`, `depthBudgets` `:159-166`).
- `internal/sidecar/denyclient.go` (`RequestPermission` `:158-187`, `isWriteToolKind` `:123-132`, `isSubmitReportTool` `:138-144`, `SessionUpdate` decode `:191-274`, `HandleExtensionMethod` `_claude/sdkMessage` `:50-67`).
- `internal/sidecar/turn.go` (`startACPWatchdog` `:19-36`, `configureACPSession` `:38-82`, `runPrompt`/`waitForACPPrompt` `:88-133`).
- `internal/sidecar/reportsink_exe.go` (`reportSinkMcpServers` `:48-65`), `reportsink.go` (`ReportSinkServerName="ghx-report-sink"` `:27`, `SubmitReportToolName="submit_report"` `:29`, `SubmitReportToolID="mcp__ghx-report-sink__submit_report"` `:38`), `rawsdk.go` (`RawSDKMessageMethod="_claude/sdkMessage"` `:20`).
- `internal/sidecar/turnresult.go` (`TurnResult` `:6-63`: has `FullText`, `Thinking`, `ToolOutputChars`, `AgentInfo`, `RawSDK` — **no `Usage` field**).
- `internal/sidecar/emit.go` (`:352-353,368` — prod tokens are char proxies: `len(Question)`, `len(FullText)`, `len(Thinking)`).

**External sources (URL-cited):**
- anthropic-sdk-go repo: <https://github.com/anthropics/anthropic-sdk-go> (README + `examples/message-streaming/main.go` + `examples/tools/main.go`).
- Go SDK docs (streaming accumulate, unions, error handling): <https://platform.claude.com/docs/en/api/sdks/go>.
- codex-acp (TypeScript wrapper; env-var steering): <https://github.com/agentclientprotocol/codex-acp> (README env-var table + `src/index.ts`).
- codex-acp (Rust reference; ACP wire + event mapping): <https://github.com/zed-industries/codex-acp> (`src/codex_agent.rs`, `src/thread.rs`, `src/main.rs`, `src/lib.rs`).
- Codex config schema (enum values, key names): <https://github.com/openai/codex/blob/main/codex-rs/core/config.schema.json> (fetched raw; parsed with `jq`). Human docs pointer: <https://developers.openai.com/codex/config>.

**Confidence flags.** Facts I could not confirm from a primary source are marked
**[UNCONFIRMED]** inline and collected in each adapter's Risks section. The two
biggest: the exact **Go binding for `output_config.effort`** on the pinned
`anthropic-sdk-go` version, and the exact **`anthropic.Usage` field names** — both
are asserted from the SDK's established shape but should be pinned by the builder
against `go doc` on the vendored version.

---

## Part 0 — What both adapters inherit from the port

The port hands each adapter a neutral `TurnRequest{Prompt, Steering SteeringSpec, Budget TurnBudget}`
and an `EventSink`, and expects `(TurnResult, ResumeToken, Outcome)` back. The
fields to satisfy, verbatim from the runner-port audit:

- `SteeringSpec{SystemPrompt, Isolation, ToolPolicy, Model, Effort, Thinking, ReportSink, AuditRawStream}`.
- `EventSink{Text, Thinking, ToolStarted, ToolUpdated, Activity, RawStreamMessage}`.
- `Outcome{Class FailureClass, Err}` with `FailureClass ∈ {Success, TurnCapReached, LivenessTimeout, PeerDied, StaleSession, Other}`.

Two neutral additions the port needs that do **not** exist yet in `TurnResult`
(both adapters depend on them):

1. **`TurnResult.Usage`** — a neutral token slot (`{InputTokens, OutputTokens, CacheReadInputTokens, CacheCreationInputTokens int}` or similar). Absent today (`turnresult.go:6-63`); prod tokens are char proxies (`emit.go:352-353`). The claude-sdk adapter fills it with **real** tokens; the ACP/codex adapters fill what their runtime exposes (claude-acp from `_claude/sdkMessage`; codex from context-window usage — see C3). This is the additive schema change ADR-0036 Phase D gates.
2. **A runtime-owned marker writer** keyed by `FailureClass` (ADR-0036 D4 / audit L9). The adapter returns the class; the *runtime* writes the byte-identical `LivenessTimeoutMarker` / max-turns strings so `evals/anomalies.go:115,124` is untouched.

The current claude-acp path is the reference implementation for both: `SessionUpdate`
decode (`denyclient.go:191-274`) is the `EventSink` model, and the string-matchers
(`acp.go:78-106`) are the `Outcome` model.

---

# Part A — the `codex-acp` adapter (ADR-0036 Phase C3)

## A0. The single most important fact for the builder: **there are two codex-acp binaries, with two different steering channels**

"codex-acp" is not one project. The audit's env-var steering surface
(`CODEX_CONFIG`/`MODEL_PROVIDER`/`INITIAL_AGENT_MODE`) is specifically the
**agentclientprotocol/codex-acp** TypeScript wrapper; the **zed-industries/codex-acp**
Rust binary steers through **`-c key=value` CLI overrides** instead. Both drive the
*same* Codex `Config` and both are stdio **ACP** servers, so ghx's existing ACP
transport (`coder/acp-go-sdk`, the same client used by claude-acp) speaks to either.
The adapter must know which one the config selects, because the *channel* differs
even though the *target config keys* are identical.

| Binary | Steering channel | Auth env | Source |
|---|---|---|---|
| **agentclientprotocol/codex-acp** (TS) | `CODEX_CONFIG` (a JSON object `JSON.parse`-d and merged into the Codex session config), plus `MODEL_PROVIDER`, `INITIAL_AGENT_MODE` env vars | `CODEX_API_KEY` > `OPENAI_API_KEY` | README env table; `src/index.ts` reads `process.env["CODEX_CONFIG"]` → `JSON.parse`, `process.env["MODEL_PROVIDER"]`. <https://github.com/agentclientprotocol/codex-acp> |
| **zed-industries/codex-acp** (Rust) | `CliConfigOverrides` = `-c key=value` args parsed via `Config::load_with_cli_overrides_and_harness_overrides()`; **no** `CODEX_CONFIG`/`MODEL_PROVIDER`/`INITIAL_AGENT_MODE` env reads | `CODEX_API_KEY`, `OPENAI_API_KEY`, `NO_BROWSER` | `src/main.rs`, `src/lib.rs`, `src/codex_agent.rs`. <https://github.com/zed-industries/codex-acp> |

**Both encode into the same Codex config keys** — so the adapter's steering-translate
step targets one neutral key set and only varies *how it writes them* (env JSON vs
`-c` args). Recommendation: pick one binary for C3 (the TS wrapper's `CODEX_CONFIG`
JSON is the closest analog to today's `BuildSessionMeta` bag and the audit's stated
channel) and record the choice in the adapter config.

This is exactly the trap ADR-0036 C3 exists to fix: ghx's `BuildSessionMeta`
nests everything under `_meta.claudeCode.options` (`session_options.go:222-224`),
which **neither** codex-acp reads. A `codex` runner today spawns, passes the ACP
handshake, and silently drops all steering (audit L1/L2).

## A1. Adapter config shape (`RunnerConfig` for `kind: codex-acp`)

```jsonc
// ~/.ghx/config.json
{
  "runner": {
    "kind": "codex-acp",
    "command": "codex-acp",              // the codex-acp binary (TS or Rust)
    "model": "gpt-5-codex",              // → Codex `model`
    "modelProvider": "openai",           // → Codex `model_provider` / MODEL_PROVIDER env
    "effort": "medium"                   // → Codex `model_reasoning_effort`
    // apiKeyEnv defaults to CODEX_API_KEY, fallback OPENAI_API_KEY
  }
}
```

The adapter builds the process env (`CODEX_CONFIG` JSON + `MODEL_PROVIDER` +
`INITIAL_AGENT_MODE` + `CODEX_API_KEY`) or the `-c` arg list, then reuses the
existing ACP transport (`RunTurnWithOptions` minus the `claudeCode` meta bag).
`AgentCmd`/`Cwd`/`Env`/stderr-tee/live-log plumbing from `RunTurnOptions`
(`acp.go:157-197`) is unchanged.

## A2. `SteeringSpec → Codex config knob` mapping (the C3 core)

Codex config keys and their **exact enumerated values** are from
`codex-rs/core/config.schema.json` (fetched raw, parsed with `jq`).

| `SteeringSpec` field | Codex knob (config key) | Exact values / shape | Notes for the builder |
|---|---|---|---|
| **`SystemPrompt`** | `instructions` (top-level: *"System instructions"*), or `developer_instructions` (*"inserted as a `developer` role message"*) | free string | **Corrects the audit.** The audit assumed codex has "no system-prompt channel" and persona must go into the prompt text. It does: `CODEX_CONFIG.instructions` (or `developer_instructions`) is a real config-level persona slot. **Do not** use `model_instructions_file` (schema: *"Users are STRONGLY DISCOURAGED"*, it overrides Codex's own base instructions). Prefer `developer_instructions` if you want ghx's persona layered *with* Codex's built-ins; use `instructions` to set the system message. Record which, because it shifts cache/SPT behavior — degrade loudly per Visibility & Truthfulness. |
| **`Isolation`** (no host settings / host MCP unless opted in) | `sandbox_mode` + `INITIAL_AGENT_MODE` + `mcp_servers` (only ghx-provided) | `sandbox_mode ∈ {"read-only","workspace-write","danger-full-access"}` | Isolation ≈ `sandbox_mode:"read-only"` + `INITIAL_AGENT_MODE:"read-only"`. Codex has no host `settingSources`/`strictMcpConfig` analog; isolation is by **not** registering host MCP servers and by sandbox mode. The `mcp_servers` map ghx sets *replaces* what the session sees (only ghx's report-sink appears). |
| **`ToolPolicy`** (allow recon shell + read; deny writes) | `sandbox_mode` = `"read-only"` **+** `approval_policy` + `INITIAL_AGENT_MODE` | `approval_policy` (`AskForApproval`) ∈ `{"untrusted","on-request","never", {granular:{…}}}`; `INITIAL_AGENT_MODE` ∈ `{"read-only","agent","agent-full-access"}` | Strongest deny-writes = `sandbox_mode:"read-only"` (Codex cannot write regardless of model intent) + `INITIAL_AGENT_MODE:"read-only"`. Belt-and-suspenders: ghx's `denyClient.RequestPermission` still runs (see A3) and rejects `ToolKind::Edit`. Prefer `approval_policy:"never"` so failed writes return to the model instead of prompting ghx (ghx is non-interactive). |
| **`Model`** | `model` (+ `model_provider`, `model_providers.<name>`) | strings | `model_provider` selects a `[model_providers.NAME]` entry (`ModelProviderInfo`: `name, base_url, env_key, wire_api, http_headers, …`). Set from `RunnerConfig.Model`/`.ModelProvider`. Mirrors ADR-0020.1 D4 wrong-model-impossible pinning. |
| **`Effort`** | `model_reasoning_effort` | schema type = non-empty string *"advertised by the model"* (not a fixed enum in current schema; historically `minimal`/`low`/`medium`/`high`) | Map ghx's `low/medium/high` intent through; **do not** assume a fixed enum — pass the value and let Codex validate. ghx's `depthBudgets` effort column (`session_options.go:159-166`) maps directly. |
| **`Thinking`** (Off or token budget) | *no direct token-budget knob* → `model_reasoning_effort` + `model_reasoning_summary`/`hide_agent_reasoning` | — | **Degrade loudly.** Codex has no `thinking.budgetTokens` equivalent; reasoning depth rides `model_reasoning_effort`. A ghx `ThinkingBudget` token count cannot be honored literally — map budget→effort tier and record a warning. Reasoning *visibility* is `model_reasoning_summary` / `hide_agent_reasoning` / `show_raw_agent_reasoning`. |
| **`ReportSink{Path, ToolID}`** | one `mcp_servers.<name>` stdio entry + tool auto-approval | `mcp_servers.ghx_report_sink = {command, args, env}` and `default_tools_approval_mode:"auto"` (or `enabled_tools:["submit_report"]`) | Directly parallels `reportSinkMcpServers` (`reportsink_exe.go:48-65`) but via Codex config, not `acp.McpServer`. `RawMcpServerConfig` supports stdio (`command`,`args`,`env`,`cwd`) and per-server tool gating (`enabled_tools`,`disabled_tools`,`default_tools_approval_mode` ∈ `AppToolApproval{"auto","prompt","approve"}`). Set `default_tools_approval_mode:"auto"` so `submit_report` is auto-approved server-side — **more robust than the title-match bypass** (see A3 risk). Note codex-acp merges client-provided ACP `mcpServers` into `config.mcp_servers` too (`codex_agent.rs`), so ghx can alternatively pass the report-sink via the ACP `mcpServers` field it already builds. |
| **`AuditRawStream`** (eval-only) | *no `_claude/sdkMessage` equivalent* | — | Codex has no ACP extension method carrying raw provider messages. Best available: the `TokenCountEvent` (context-window usage) and per-tool events. **codex-acp cannot fill `TurnResult.Usage` with per-turn input/output tokens** — see A4/A5. Set `AuditRawStream` to a no-op and record it. |

## A3. `EventSink` decode: Codex `EventMsg` → ACP `SessionUpdate` → `EventSink`

Because codex-acp is an ACP server, ghx's **existing** `denyClient.SessionUpdate`
decode (`denyclient.go:191-274`) works unchanged — the adapter reuses it and
routes to `EventSink` instead of stdout/live. The zed-Rust codex-acp maps Codex's
internal `EventMsg` to ACP `SessionUpdate` as follows (`src/thread.rs`,
`PromptState::handle_event`); this is what arrives on ghx's ACP client:

| Codex `EventMsg` | ACP `SessionUpdate` | ghx decode → `EventSink` | ghx `ToolCallTrace.Kind` |
|---|---|---|---|
| `AgentMessageContentDeltaEvent` (and non-delta `AgentMessage`) | `AgentMessageChunk` (`ContentChunk`) | `denyclient.go:203-213` → `EventSink.Text` | — |
| `ReasoningContentDeltaEvent`, `ReasoningRawContentDeltaEvent` | `AgentThoughtChunk` | `:214-223` → `EventSink.Thinking` | — |
| `ExecCommandBeginEvent` → `…OutputDeltaEvent` → `ExecCommandEndEvent` | `ToolCall(ToolKind::Execute)` → `ToolCallUpdate` (`InProgress`→`Completed`/`Failed`; exit 0 = `Completed`) | `:224-243` (`ToolCall`) / `:244-271` (`ToolCallUpdate`) → `EventSink.ToolStarted`/`ToolUpdated` | `"execute"` |
| `PatchApplyBeginEvent`/`ApplyPatchApprovalRequestEvent` → `PatchApplyEndEvent` | `ToolCall(ToolKind::Edit)` | same path | `"edit"` → **write-shaped** |
| `WebSearchBeginEvent`/`WebSearchEnd` | `ToolCall(ToolKind::Fetch)` | same path | `"fetch"` |
| `McpToolCallBeginEvent`/`McpToolCallEndEvent` | `ToolCall(title: "Tool: {server}/{tool}")` | same path | `"other"` |
| `TokenCountEvent` | `UsageUpdate(used, window_size)` | **not currently consumed by `denyClient`** | — (see A5) |
| every event | — | fires `onActivity` (`:192-194`) → `EventSink.Activity` | liveness heartbeat |

**Permission enforcement (`EventSink`-adjacent).** Codex emits
`ExecApprovalRequestEvent`/`ApplyPatchApprovalRequestEvent`/`ElicitationRequestEvent`
→ ACP `RequestPermissionRequest` with `ReviewDecision` options. ghx's
`denyClient.RequestPermission` (`denyclient.go:158-187`) already handles these:
`ToolKind::Execute`/`Read`/`Search`/`Fetch`/`Think` → allow, `ToolKind::Edit`/unknown
→ reject (`isWriteToolKind`, `:123-132`). So the deny-writes policy holds **at the
ACP permission layer** even before Codex's sandbox — good defense-in-depth.

**Report-sink auto-approve — a concrete C3 risk.** ghx's `isSubmitReportTool`
(`denyclient.go:138-144`) matches by title `mcp__<server>__<tool>` or suffix
`__submit_report`. Codex titles MCP calls **`"Tool: {server}/{tool}"`** (`thread.rs`
`McpToolCallBeginEvent`), which does **not** match either pattern. The title-based
bypass will therefore **not fire under codex**, and `submit_report` (kind `"other"`
→ write-shaped) would be rejected. **Fix:** auto-approve at the Codex config layer
via `default_tools_approval_mode:"auto"` on the report-sink `mcp_servers` entry
(A2), *and/or* widen `isSubmitReportTool` to also match the `"Tool: ghx-report-sink/submit_report"`
title. The audit flagged L5 as adapter-private; this is the concrete mechanism.

## A4. `Outcome`/`FailureClass` detection for codex-acp

The current claude-acp string-matchers (`acp.go:78-106`) become adapter-internal.
For codex the classification signals are the ACP-mapped turn-end and RPC errors:

| `FailureClass` | Codex/ACP signal | Detection |
|---|---|---|
| `Success` | `TurnCompleteEvent` → ACP `StopReason::EndTurn` | normal prompt completion, no error |
| `TurnCapReached` | Codex has no single hard "Reached maximum number of turns" string like claude-acp | **[UNCONFIRMED]** Codex enforces caps differently (token/context via `model_auto_compact_token_limit`, `ThreadGoalUpdatedEvent` status `"budget limited"`/`"usage limited"`). Map those statuses → `TurnCapReached`; **confirm** the exact terminal event. If none exists, the adapter's own turn counter is the net (as in Part B). |
| `LivenessTimeout` | runtime watchdog via `EventSink.Activity` | unchanged: runtime owns `startACPWatchdog` (`turn.go:19-36`); adapter returns `LivenessTimeout` on cancel. Marker written by runtime. |
| `PeerDied` | ACP peer closed / process exit | reuse `IsPeerClosedError` (`acp.go:84-90`, `peerClosedMarker="peer connection closed"`) — this is a `coder/acp-go-sdk` transport string, runtime-agnostic, so it already works for codex. |
| `StaleSession` | ACP `LoadSession` JSON-RPC `-32002` | codex-acp advertises `loadSession(true)` + resume capabilities (`codex_agent.rs`), so `IsLoadSessionResourceNotFound` (`acp.go:96-106`) applies unchanged if codex uses the same code. **[UNCONFIRMED]** codex's stale-session error code. |
| `Other` | `ErrorEvent`, `TurnAbortedEvent`→`StopReason::Cancelled` | fall-through; underlying `Err` for artifacts only. |

## A5. Token accounting under codex — **codex-acp does NOT close TRUST H5**

The claude-acp adapter fills real tokens from the `_claude/sdkMessage` ACP
extension (`denyclient.go:50-67`, `rawsdk.go:20`). **Codex has no equivalent
extension.** Its only usage signal is `TokenCountEvent` → ACP `UsageUpdate(used, window_size)`,
which is **context-window occupancy**, not per-turn input/output token counts. So:

- `TurnResult.Usage` from codex is either empty (keep char-proxy, `emit.go:352-353`)
  or a best-effort context-window delta — **not** the input/output split the eval
  stack's `gen_ai.usage.*` attributes expect.
- **Only the claude-sdk adapter (Part B) closes H5.** Record this in the adapter
  as a loud capability gap; do not silently emit context-window numbers as if they
  were input/output tokens (Visibility & Truthfulness).

## A6. codex-acp — unknowns/risks for the builder

1. **Which binary.** Pick TS (`CODEX_CONFIG` env) or Rust (`-c` overrides) and encode the channel; they are not interchangeable at the steering layer (A0).
2. **Report-sink title mismatch** (A3): codex's `"Tool: {server}/{tool}"` title breaks `isSubmitReportTool`. Auto-approve via `default_tools_approval_mode:"auto"` and/or widen the matcher. **Must verify live** or `submit_report` is rejected and every episode degrades to the `<ghx-report>` text fallback.
3. **Persona placement** (A2): `instructions` vs `developer_instructions` shifts cache/SPT — pick deliberately, record loudly. This is the honest tradeoff the audit named, but the channel *exists* (audit was pessimistic).
4. **Turn-cap terminal event** unconfirmed (A4). Find codex's analog of "Reached maximum number of turns" or rely on the adapter's own counter.
5. **Token accounting** (A5): codex cannot fill `TurnResult.Usage` with input/output tokens. C3 must **not** claim H5 progress.
6. **Auth**: `CODEX_API_KEY` > `OPENAI_API_KEY`; ChatGPT OAuth also offered (`auth_methods=[ChatGpt, CodexApiKey, OpenAiApiKey]`, `codex_agent.rs`). ghx's non-interactive preflight should require an API key present, set `NO_BROWSER`, and treat ChatGPT-login as unsupported.
7. **Preflight** = the same ACP `initialize` handshake (`CheckACPHandshake`, `acp.go:340-387`); codex-acp advertises `promptCapabilities{embedded_context, image}`, `mcpCapabilities.http`, `loadSession(true)` — the handshake passes, which is exactly why the *silent steering drop* is dangerous without C3.

---

# Part B — the in-process `claude-sdk` adapter (ADR-0036 Phase D)

## B0. Shape: an owned in-process agent loop over `anthropic-sdk-go`

No subprocess, no ACP, no permission wire. The adapter *is* the harness: it
assembles context, offers exactly two tools (a read-only ghx-shell tool + a
`submit_report` tool), streams model output to `EventSink`, and loops until
`submit_report` or the adapter's own turn cap. This is NORTH_STAR P4 "below ACP"
made literal, and it *removes* leaks rather than translating them (audit Adapter 3a).

**Confirmed Go SDK surface** (Go SDK docs <https://platform.claude.com/docs/en/api/sdks/go>;
`examples/message-streaming/main.go`, `examples/tools/main.go`):

```go
client := anthropic.NewClient(option.WithAPIKey(key)) // defaults to ANTHROPIC_API_KEY

// Streaming turn with accumulation:
stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
    Model:     anthropic.ModelClaudeSonnet5, // or the pinned subject model
    MaxTokens: 64000,
    System:    []anthropic.TextBlockParam{{Text: persona}},
    Messages:  history,
    Tools:     tools,
})
message := anthropic.Message{}
for stream.Next() {
    event := stream.Current()
    _ = message.Accumulate(event) // reassembles the full Message as deltas arrive
    switch ev := event.AsAny().(type) {
    case anthropic.ContentBlockDeltaEvent:
        switch d := ev.Delta.AsAny().(type) {
        case anthropic.TextDelta:       sink.Text(d.Text)
        case anthropic.ThinkingDelta:   sink.Thinking(d.Thinking) // [UNCONFIRMED] delta type name
        case anthropic.InputJSONDelta:  /* accumulate tool input */
        }
    }
    sink.Activity()
}
if stream.Err() != nil { /* → Outcome */ }

// After the stream, walk final blocks (examples/tools/main.go):
for _, block := range message.Content {
    switch b := block.AsAny().(type) {
    case anthropic.TextBlock:    /* already streamed */
    case anthropic.ToolUseBlock: sink.ToolStarted(...); out := run(b.Name, b.JSON.Input.Raw()); sink.ToolUpdated(...)
                                 toolResults = append(toolResults, anthropic.NewToolResultBlock(b.ID, out, isErr))
    // case anthropic.ThinkingBlock: native reasoning, no _meta archaeology
    }
}
history = append(history, message.ToParam())
if len(toolResults) > 0 { history = append(history, anthropic.NewUserMessage(toolResults...)) }
// loop until stop_reason == "end_turn", submit_report seen, or adapter turn cap
```

`block.AsAny()` narrowing, `message.ToParam()`, `anthropic.NewToolResultBlock(id, out, isErr)`,
and `anthropic.NewUserMessage(...)` are all verbatim from the SDK's tool-calling
example. `message.Accumulate(event)` + the two-level `AsAny()` switch are verbatim
from the streaming example.

## B1. Adapter config shape (`RunnerConfig` for `kind: claude-sdk`)

```jsonc
{
  "runner": {
    "kind": "claude-sdk",
    "apiKeyEnv": "ANTHROPIC_API_KEY",   // no "command"; there is no subprocess
    "model": "claude-sonnet-5",
    "effort": "high"
  }
}
```

`AgentCmd`/`Env`/`initialize` have **no meaning here** — which is exactly why
ADR-0036 keeps them off the port and on the ACP adapters' private config
(audit "the port must carry no subprocess/ACP assumption").

## B2. `SteeringSpec → anthropic-sdk-go knob` mapping (the Phase D core)

| `SteeringSpec` field | SDK knob | How | Notes |
|---|---|---|---|
| **`SystemPrompt`** | `MessageNewParams.System []anthropic.TextBlockParam` | `System: []anthropic.TextBlockParam{{Text: persona}}` | Native. Cache-stable by putting persona first (prompt-caching prefix rule). Removes `_meta.systemPrompt` and the whole `BuildSessionMeta` bag. |
| **`Isolation`** | *structural — nothing to set* | in-process | No host settings/MCP to inherit; the loop only ever sees the tools you register. `settingSources:[]`/`strictMcpConfig` (`session_options.go:22-35`) become no-ops. Isolation is inherent, not configured. |
| **`ToolPolicy`** (deny writes) | tool registry | register only a read-only ghx-shell tool + `submit_report`; **never register a write/edit tool** | Stronger than ACP's after-the-fact `denyClient` reject (audit L4/L5 vanish). The shell tool wrapper must itself only exec ghx recon (same residual-risk envelope as `denyClient` today, `denyclient.go:146-157`). |
| **`Model`** | `MessageNewParams.Model` | `Model: anthropic.Model(spec.Model)` (e.g. `anthropic.ModelClaudeSonnet5`, `ModelClaudeOpus4_8`) | Pinned; wrong-model impossible (ADR-0020.1 D4). |
| **`Effort`** | `output_config.effort` | **[UNCONFIRMED Go binding]** GA feature; set effort `low/medium/high/xhigh/max` on `MessageNewParams`. Confirm the exact Go field (`OutputConfig`/effort) via `go doc` on the pinned SDK version. | Effort is the primary depth control on current models; maps ghx's `depthBudgets` effort column. |
| **`Thinking`** (Off / budget) | `MessageNewParams.Thinking` | adaptive: `Thinking: anthropic.ThinkingConfigParamUnion{OfAdaptive: &anthropic.ThinkingConfigAdaptiveParam{Display: anthropic.ThinkingConfigAdaptiveDisplaySummarized}}` | **Behavior change to flag.** On Sonnet 5 / Opus 4.7/4.8, `thinking:{type:"enabled",budget_tokens:N}` returns **400** — the fixed token budget in ghx's `depthBudgets` (`session_options.go:159-166`) is *not* portable. Map budget→adaptive+effort tier and record a warning. `display:"summarized"` is required or thinking text is empty (default `"omitted"`). This *removes* ADR-0018 `thinking.display` archaeology entirely — native `ThinkingBlock`s. |
| **`ReportSink{Path, ToolID}`** | in-process tool | register a `submit_report` `anthropic.ToolParam` whose handler writes the accepted report to `ReportSink.Path` and ends the loop | No MCP subprocess, no `reportSinkMcpServers`, no `allowedTools` bypass. The tool can be named plainly `submit_report` (no `mcp__` prefix). **Reuse the existing report JSON schema/contract** so `TurnResult`/report parsing is unchanged. |
| **`AuditRawStream`** (eval-only) | `EventSink.RawStreamMessage` + `Usage` | marshal each raw `anthropic` stream event / final `Message` to bytes when the flag is set | And unconditionally read `message.Usage` (below) — real tokens are free here, unlike the claude-acp `_claude/sdkMessage` extension path. |

## B3. `EventSink` decode from the SDK stream

| `EventSink` call | Source (from the loop) |
|---|---|
| `Text(delta)` | `ContentBlockDeltaEvent` → `TextDelta.Text` |
| `Thinking(delta)` | `ContentBlockDeltaEvent` → `ThinkingDelta.Thinking` **[UNCONFIRMED delta type name]** — confirm via `go doc`; native `ThinkingBlock` also present in `message.Content` |
| `ToolStarted(call)` | a `ToolUseBlock` in `message.Content` after the stream (name + `b.JSON.Input.Raw()`); populate `ToolCallTrace{ID: b.ID, Title, Kind}` |
| `ToolUpdated(call)` | after the adapter executes the tool: status Completed/Failed, output size into `ToolCallTrace.OutputSize`/`ToolOutputChars` |
| `Activity()` | every stream event (drives the runtime watchdog identically) |
| `RawStreamMessage(raw)` | when `AuditRawStream`: JSON of the event/final message |

`ToolCallTrace.Kind` should be set so the deny-writes intent and any downstream
kind-based logic stay stable (the ghx-shell tool → `"execute"`, `submit_report`
→ its own kind). Because ghx *chooses* the tool set, there is no "MCP-classified-as-other"
workaround (audit L5 disappears).

## B4. `Outcome`/`FailureClass` detection for claude-sdk

| `FailureClass` | Signal | Detection |
|---|---|---|
| `Success` | final `message.StopReason == "end_turn"`, or the `submit_report` handler fired | end the loop cleanly |
| `TurnCapReached` | **adapter-owned counter** hits the mapped cap | there is no provider "max turns" string in an in-process loop — the adapter *is* the loop, so the cap is its own `for` bound (maps ghx's `depthBudgets.maxTurns`, `session_options.go:159-166`). Runtime then issues the D1 wrap-up resume via `Session.Turn` and writes the byte-identical max-turns marker. |
| `LivenessTimeout` | runtime watchdog via `EventSink.Activity` | unchanged; each stream event is a heartbeat. Runtime cancels ctx → adapter returns `LivenessTimeout`. |
| `PeerDied` | transport/connection failure | `stream.Err()` / `errors.As(err, &anthropic.Error)` connection errors (or unwrapped `*url.Error`). In-process has no "peer process", so this is a network/transport death → map to `PeerDied` (drop nothing warm) or `Other`. **Recommend `Other`** unless the runtime pool needs a distinct signal. |
| `StaleSession` | n/a | in-process resume = ghx's own persisted `[]MessageParam` history (`ResumeToken`); it can never be "resource not found", so this class is never returned. Resume is an optimization (audit L12). |
| `Other` | API errors / refusal | `errors.As(err, &apierr *anthropic.Error)` then `switch apierr.StatusCode` (Go SDK docs error-handling pattern). 429/5xx are auto-retried by the SDK (`WithMaxRetries`, default 2). Also check `message.StopReason == "refusal"` → fail the turn with a recorded reason (do not read `content[0]` blindly). |

## B5. Real token accounting — this is where Phase D closes TRUST H5

After each `client.Messages.NewStreaming` turn, read `message.Usage`:

- `message.Usage.InputTokens`, `message.Usage.OutputTokens`,
  `message.Usage.CacheReadInputTokens`, `message.Usage.CacheCreationInputTokens`
  **[UNCONFIRMED exact field names]** — assert the standard `anthropic.Usage` shape;
  pin via `go doc github.com/anthropics/anthropic-sdk-go.Usage` on the vendored version.
- Accumulate across the loop's turns into the new `TurnResult.Usage` (Part 0).
  This replaces the char proxies at `emit.go:352-353,368` with **real production
  tokens** — the exact H5 fix ADR-0036 Phase D pre-registers. Because it changes a
  measurement source, Phase D is ADR-gated and re-grounded with a sidecar-only
  comparison run (never a drive-by).

## B6. claude-sdk — unknowns/risks for the builder

1. **[UNCONFIRMED] `output_config.effort` Go binding** — GA feature, but the exact Go field on `MessageNewParams` (and whether it needs a beta path on the pinned SDK version) must be pinned via `go doc`. Fallback: `option.WithJSONSet("output_config.effort", …)` (documented escape hatch).
2. **[UNCONFIRMED] `ThinkingDelta` / `ThinkingBlock` / `Usage` field names** — asserted from the SDK's established shape; confirm via `go doc` before writing. Low risk (these are core types) but load-bearing for streaming decode and H5.
3. **Thinking budget is not portable** (B2): ghx's fixed `budget_tokens` depth mapping 400s on current models. Convert to adaptive + effort; record the mapping. This is a genuine steering-contract change, not a silent one.
4. **Refusal handling** (B4): the subject model can return `stop_reason:"refusal"`; the loop must branch before reading content. For a reconnaissance persona this should be rare, but a false-positive on security-adjacent recon is possible.
5. **Prompt caching** must be deliberately engineered (persona first, deterministic tool order) or the in-process loop pays cold cache-writes every turn — a cost regression vs the adapter, which the SDK path is supposed to *improve*. Put the stable persona/tools before any volatile content.
6. **Context assembly is now ghx's job.** The adapter owns the full `[]MessageParam` history across turns (resume token). The prior ACP path let the adapter process manage context; in-process, ghx must build/trim it — an opportunity (owned harness) and a responsibility (no free compaction).
7. **Cancellation/liveness** ride `ctx`; `client.Messages.NewStreaming` respects context cancellation, so the runtime watchdog wrapping `Session.Turn` cancels cleanly — no subprocess kill needed.

---

## Builder checklist (per phase)

**Phase C3 (codex-acp) — smallest second adapter, ACP transport reused:**
- [ ] Choose the codex-acp binary; encode its steering channel (`CODEX_CONFIG` env JSON vs `-c` overrides) (A0).
- [ ] Translate `SteeringSpec` → Codex config keys per A2 (persona via `instructions`/`developer_instructions`; deny-writes via `sandbox_mode:"read-only"` + `INITIAL_AGENT_MODE:"read-only"` + `approval_policy:"never"`).
- [ ] Register report-sink as a Codex `mcp_servers` stdio entry with `default_tools_approval_mode:"auto"`; widen/verify `isSubmitReportTool` for the `"Tool: ghx-report-sink/submit_report"` title (A3 — **live-verify or reports silently fall back**).
- [ ] Reuse `denyClient.SessionUpdate`/`RequestPermission` as-is; route to `EventSink` (audit C1).
- [ ] Map `Outcome` per A4; keep runtime-owned markers byte-identical.
- [ ] Record loudly: no per-turn token accounting (A5) — C3 does **not** touch H5.
- [ ] Verify: one live `codex-acp` ask (valid report, same artifact shape) + a sidecar-only 2–3-cell eval spot check for anomaly-contract drift (ADR-0036 C3).

**Phase D (claude-sdk) — ADR-gated, measurement change:**
- [ ] Add `TurnResult.Usage` (Part 0) — additive schema change.
- [ ] Build the in-process loop (B0); register only the read-only ghx-shell tool + `submit_report`.
- [ ] Map `SteeringSpec` per B2; convert fixed thinking budgets → adaptive+effort with a recorded warning (B6.3).
- [ ] Decode `EventSink` per B3; fill `TurnResult.Usage` from `message.Usage` (B5).
- [ ] Map `Outcome` per B4 (adapter-owned turn cap; refusal branch).
- [ ] Pin the four **[UNCONFIRMED]** SDK bindings via `go doc` before writing (B6.1–B6.2).
- [ ] Pre-register the token-source change in its own ADR; re-ground with a citable sidecar-only comparison run (ADR-0036 Phase D; eval cadence).

---

## Sources

- ghx worktree (branch `agent-a92fa3c6c5a123840`): `internal/sidecar/{acp,session_options,turn,denyclient,reportsink,reportsink_exe,rawsdk,turnresult,emit}.go` (file:line inline).
- ADR-0036 — `docs/adr/0036-target-architecture-runner-port-and-boundaries.md`.
- Runner-port audit — `docs/audits/architecture-vision/runner-port-pluggability-2026-07-07.md`.
- anthropic-sdk-go — <https://github.com/anthropics/anthropic-sdk-go> (`examples/message-streaming/main.go`, `examples/tools/main.go`).
- Go SDK docs — <https://platform.claude.com/docs/en/api/sdks/go> (streaming accumulate, response unions `AsAny()`, `errors.As(&anthropic.Error)`, retries).
- codex-acp (TS, env steering) — <https://github.com/agentclientprotocol/codex-acp> (README env table; `src/index.ts`).
- codex-acp (Rust, ACP wire + event map) — <https://github.com/zed-industries/codex-acp> (`src/codex_agent.rs`, `src/thread.rs`, `src/main.rs`, `src/lib.rs`).
- Codex config schema (enum values) — <https://github.com/openai/codex/blob/main/codex-rs/core/config.schema.json> (parsed with `jq`); config docs pointer <https://developers.openai.com/codex/config>.
