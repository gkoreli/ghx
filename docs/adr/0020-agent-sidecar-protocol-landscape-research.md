---
title: "ADR-0020: Agent Sidecar Protocol & Harness Landscape — Research"
date: "2026-07-05"
status: "research — input to ADR-0020.1"
thread: "sidecar-protocol-strategy"
author: "Goga Koreli"
---

# 0020. Agent Sidecar Protocol & Harness Landscape — Research

## Status

Research only. Written to feed ADR-0020.1, which will make the architecture
decision. No recommendation is made here; all claims are either VERIFIED
(from a primary source fetched on 2026-07-05) or INFERRED (synthesis).
Conflicts with prior ADRs are flagged explicitly rather than resolved.

---

## Organising Frame: the Four Pre-Registered ACP Exit Triggers

NORTH_STAR "Protocols are stepping stones, not identity" tenet (clarified
2026-07-05, commit 1620b2e) registers four evidence-based triggers for the
P4 "below ACP" move. Each section below reports what the landscape offers
**today** if that trigger fires.

### Trigger 1 — Adapter opacity taxing us

**Recap of our evidence.** ADR-0018 documents (source: `npm pack
@agentclientprotocol/claude-agent-acp@0.55.0`, inspected 2026-07-05): the
adapter does emit `agent_thought_chunk` for non-empty thinking blocks, gated
by `MAX_THINKING_TOKENS` env or `_meta.claudeCode.options.thinking`; subagent
thinking is intentionally filtered. Upstream ACP docs specify the
`agent_thought_chunk` update kind in the schema (VERIFIED: ACP schema page,
accessed 2026-07-05 — it is listed as a valid `SessionUpdate` union type
alongside `agent_message_chunk`, `tool_call`, `tool_call_update`, `plan`,
`available_commands_update`, `current_mode_update`) but say nothing about
**when** or **whether** a specific adapter emits it.

**Upstream issue tracker confirms the gap is open.** Issue #297 in
agentclientprotocol/claude-agent-acp (opened Feb 8, 2026, no official
maintainer response as of 2026-07-05) requests the ability to enable
extended thinking, noting "the feature is not enabled by default, and I
cannot find a configuration setting to change it." Kiro issue #7099 (Apr 3,
2026) requests `agent_thought_chunk` be emitted when thinking is enabled via
`chat.enableThinking`, reporting no effect when that flag is set. Both
confirm our lived finding: thinking emission is adapter-internal, undocumented
upstream, and not yet user-configurable in standard ACP clients.
(VERIFIED: GitHub issue URLs fetched 2026-07-05.)

**What ACP's `_meta` mechanism offers.** The spec reserves `_meta` on all
protocol objects: "Implementations MUST NOT make assumptions about values at
these keys." This provides forward-compatible extension without breaking
core protocol, but it is entirely adapter-specific; no standard semantic is
defined for thinking configuration via `_meta`. The `_claude/sdkMessage`
extension notification (`emitRawSDKMessages`) is an adapter-specific firehose
(ADR-0018 source), not portable ACP — useful for eval telemetry but not a
stable cross-adapter surface.

**If trigger fires.** Dropping below ACP into the Claude Agent SDK would
give us direct control: subagents inherit the parent session's extended
thinking configuration as of Claude Code v2.1.198 (VERIFIED: Claude Agent
SDK subagents docs, code.claude.com, accessed 2026-07-05). We set `effort`
and thinking budget per subagent definition directly in Go code without
depending on adapter internals.

---

### Trigger 2 — Measured SPT plateau attributable to harness overhead

**Recap of our evidence.** ADR-0016.6 defines SPT at three levels;
confirmatory gate run (2026-07-05) measures 24× signal/token improvement
at the main-agent level. The ~400-line persona doctrine is re-injected every
first turn (ADR-0015, `BuildPrompt`). The per-session preamble is the primary
known harness overhead: with ACP the only lever is prompt compression; the
persona cannot be baked into weights or removed from the context window
without either a fine-tuned model or direct harness control.

**Claude Agent SDK context isolation.** VERIFIED (code.claude.com
subagents docs, 2026-07-05): each subagent starts a fresh context window;
"intermediate tool calls and results stay inside the subagent; only its final
message returns to the parent." The `AgentDefinition` `prompt` field is the
system prompt (equivalent to our persona). Tool registry is per-subagent via
`tools` / `disallowedTools` arrays. Session persistence: `resume: sessionId`
continues a prior session; subagent transcripts persist independently and
survive main-conversation compaction. The SDK auto-compacts the context when
approaching limits.

**Key limitation.** The Claude Agent SDK model field accepts Anthropic model
aliases or full model IDs only (`'fable'`, `'opus'`, `'sonnet'`, `'haiku'`,
or a full `claude-*` ID). Custom/non-Anthropic weights cannot be specified
directly via the `model` field — this is relevant to Trigger 4 below.
(INFERRED from AgentDefinition schema; confirmed by the architecture: the
SDK runs the Claude Code CLI binary as a child process and passes config
through env vars.)

**Via vLLM + ANTHROPIC_BASE_URL.** VERIFIED (vLLM Claude Code integration
docs, docs.vllm.ai, accessed 2026-07-05): setting `ANTHROPIC_BASE_URL` to a
vLLM server makes the SDK send requests there instead of Anthropic. Models
served by vLLM with OpenAI-compatible tool calling can act as a drop-in
replacement. Constraint: model must support the Anthropic Messages API format
(vLLM implements it). Important caveat from the same source: "Recent Claude
Code versions inject a per-request hash in the system prompt, which can
defeat prefix caching" — relevant to SPT if the persona is still re-injected
each turn.

---

### Trigger 3 — Tool surface/isolation control the adapter cannot grant

**Our evidence from ADR-0016.3.** The eval isolation gap: (a) the plain
baseline could silently use `ghx` because the ACP adapter does not restrict
the agent's PATH; (b) subject agents ran inside the `ghx` repo directory and
inherited CLAUDE.md/AGENTS.md from the host. Fixes required patching the
runner to construct a scrubbed PATH and a fresh temp cwd — workarounds that
live outside ACP and cannot be configured via standard ACP protocol methods.

**ACP's tool surface control.** VERIFIED (ACP schema page, 2026-07-05): the
protocol's sole mechanism for tool surface control is `session/request_permission`,
which lets the **client** present a `PermissionOption[]` array to the user
before a tool executes, and return `RequestPermissionOutcome`. Our `denyClient`
(ADR-0015) implements this: `RequestPermission` always selects `reject_once`
or `reject_always`. This is a coarse allow/deny gate — the client **cannot**:
(a) restrict which OS commands the agent may form before they are requested,
(b) inject a different PATH into the agent process, (c) scope which `ghx`
subcommands are reachable, or (d) enforce a cwd. These controls require
spawning the agent process with the desired environment, which ACP does not
standardise.

**Claude Agent SDK tool surface control.** VERIFIED (code.claude.com,
2026-07-05): `AgentDefinition.tools` restricts available tools to an exact
list; `AgentDefinition.disallowedTools` removes specific tools including
whole MCP servers via `mcp__server` or `mcp__*` patterns.
`AgentDefinition.permissionMode` sets the permission mode for the subagent.
`AgentDefinition.mcpServers` scopes which MCP servers are available to that
subagent specifically. These are richer than ACP's coarse deny-all gate.

**Limitation.** Claude Agent SDK tool isolation is still scoped to the
Claude Code toolbox. It cannot restrict OS-level process access (PATH, cwd,
network) that the Bash tool's subprocess would have. For eval isolation
(ADR-0016.3), the runner-level workaround (scrubbed PATH via Go's
`exec.Command`) remains necessary regardless of which SDK layer is used.

---

### Trigger 4 — Serving a custom-trained model (M10)

**No ACP adapter will speak for a custom model.** INFERRED: the existing
ACP adapters (claude-agent-acp, codex-acp, etc.) are built for specific
upstream models and are not configurable to route to arbitrary fine-tuned
weights. A custom `ghx-sidecar` model would require either: (a) a new
ACP adapter built around the serving runtime, or (b) dropping below ACP
entirely.

**vLLM serving path.** VERIFIED (vLLM docs, 2026-07-05): vLLM exposes an
OpenAI-compatible (and Anthropic Messages API-compatible) HTTP endpoint.
Fine-tuned models can be served via `--served-model-name` with LoRA or
full-weight variants. Any Claude Code or Agent SDK that accepts
`ANTHROPIC_BASE_URL` can point at this endpoint. The model must support tool
calling in the OpenAI-compatible format.

**Claude Agent SDK custom model path.** VERIFIED (GitHub issue #410 on
anthropics/claude-agent-sdk-python, searched 2026-07-05; vLLM and LiteLLM
docs): the pattern is `ANTHROPIC_BASE_URL` → vLLM server →
`--served-model-name ghx-sidecar`. LiteLLM proxy adds multi-provider
routing (Bedrock, Azure, Vertex, any OpenAI-compatible endpoint) behind a
single Anthropic Messages API facade. `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1`
exposes custom model names in the picker. The fundamental requirement: the
model must have strong tool-calling capability, which any SFT/RLHF-trained
code-recon specialist model should have.

**OpenAI Agents SDK custom model path.** VERIFIED (openai.github.io/openai-agents-python,
2026-07-05): per-agent model override via `Agent.model`; custom base_url via
`AsyncOpenAI(base_url=..., api_key=...)` wrapped in `OpenAIChatCompletionsModel`.
LiteLLM and Any-LLM adapters add multi-provider routing. This is the same
pattern as the Claude Agent SDK path but using the OpenAI Chat Completions
format. Per-agent model routing is explicit: different agents in a run can
use different endpoints/weights.

---

## Per-Question Findings

### Q1. ACP — Current Spec State, Governance, Adoption, Session Model, Extension Mechanism

**Spec version and governance.** VERIFIED (github.com/agentclientprotocol/agent-client-protocol,
agentclientprotocol.com, accessed 2026-07-05): stable protocol wire version
is **v1**; schema artifact version is **v1.17.0** (June 29, 2026, as of
this research). The RFD (Request for Dialog) process governs changes:
proposals merge to Draft when championed, advance to Active when maintainers
allocate bandwidth, and reach Completed only after Preview review. Governance
is BDFL by Zed (lead maintainer), with JetBrains as a second Lead Maintainer
(Sergey Ignatov, announced Feb 2026). Licensed Apache 2.0; no CLA required.
SDKs available in Kotlin, Java, Python, Rust, TypeScript. Go SDK: the
project uses `github.com/coder/acp-go-sdk v0.13.0` (our pinned version in
`go.mod`).

**Adoption.** VERIFIED (agentclientprotocol.com/get-started/agents, accessed
2026-07-05): 30+ agents listed as ACP-compatible including Claude Agent (via
Zed's SDK adapter), Codex CLI (via Zed's adapter), Gemini CLI, GitHub Copilot
(public preview), Goose, Junie by JetBrains, Kiro CLI, OpenHands, OpenCode,
Factory Droid, Cursor, Cline, Qwen Code, and others. Client-side: Zed, Kiro,
JetBrains IDEs, plus VS Code community adapters.

**Session model.** VERIFIED (ACP schema page, 2026-07-05):
- `session/new` — creates context with optional MCP server connections
- `session/load` — resumes with full history replay (requires `loadSession` capability)
- `session/resume` — continues without history replay (requires `sessionCapabilities.resume`; stabilised April 22, 2026)
- `session/close` — cancels in-flight work, frees resources (stabilised April 23, 2026)
- `session/list` — enumerates stored sessions (stabilised March 9, 2026)

**Extension mechanism.** VERIFIED (ACP schema page, 2026-07-05): `_meta`
property on all protocol objects. Spec language: "Implementations MUST NOT
make assumptions about values at these keys." No standard semantics for any
specific extension. Adapter-specific extensions (e.g. `_claude/sdkMessage`
firehose) are out-of-band notifications, not part of the ACP spec.

**SessionUpdate kinds.** VERIFIED (ACP schema page + ACPex Hex docs,
accessed 2026-07-05): `agent_thought_chunk`, `agent_message_chunk`,
`tool_call`, `tool_call_update`, `plan`, `available_commands_update`,
`current_mode_update`, `user_message_chunk`. The `agent_thought_chunk` kind
is in the spec as a first-class update type but carries no spec-level
semantics for when an adapter must emit it.

**Documented vs undocumented areas relevant to us:**

| Area | Status |
|---|---|
| Session lifecycle (new/load/resume/close/list) | Fully specified, all stabilised |
| `agent_thought_chunk` update kind | Specified in schema; emission policy undocumented |
| `_meta` extension | Specified as opaque bag; no standard semantics |
| Thinking/reasoning enablement | Not in spec; adapter-internal (ADR-0018 finding confirmed by issues #297, #7099) |
| Subagent visibility | Not in spec; adapter-internal (intentionally filtered per ADR-0018) |
| Tool surface control by client | Only via `session/request_permission`; no PATH/cwd/command restriction |
| Session isolation (eval use case) | Not in spec; env-level concern outside ACP |

**Conflict with our ADR.** ADR-0018 states the adapter emits `agent_thought_chunk`
for non-empty thinking blocks (source: tarball inspection). The upstream spec
and issue tracker confirm there is no documented guarantee of this — the
tarball finding remains the most authoritative source, but it is not a stable
contract. No conflict per se; both sources agree thinking emission is
undocumented upstream. Issue #297 (Feb 2026) and Kiro #7099 (Apr 2026) are
both still open and unresolved, confirming the gap is known and unresolved
by maintainers.

---

### Q2. Adjacent Protocols

#### MCP (Model Context Protocol)

**What it is for.** VERIFIED (MCP docs, 2026-07-05): connecting a single
agent to tools — databases, APIs, file systems, code execution. One-way
capability provider model: agent calls, server responds. Tool discovery via
schema lists; execution in isolated server contexts. The agent's tool layer
is "the union of MCP servers I am connected to." Spec version 2025-11-05
is current stable; 2025-03-26 introduced Streamable HTTP replacing SSE as
recommended remote transport.

**Maturity/adoption.** VERIFIED: Anthropic-led, broad IDE and framework
adoption (Claude Code, Cursor, Copilot, etc.). The dominant tool-layer
protocol as of 2026.

**Fit for our sidecar boundary.** Not designed for this. MCP collapses the
sidecar into a tool call and loses session state, evidence-shaped memory, and
multi-turn exploration. ADR-0014.1's analysis remains correct: "useful for
direct tool access, but not sufficient by itself for specialized delegated
work." MCP is appropriate for the sidecar's internal tool access (ghx CLI
as an MCP-exposed tool inside the sidecar harness), not for the boundary
between main agent and sidecar.

#### Google/Linux Foundation A2A (Agent-to-Agent)

**What it is for.** VERIFIED (developers.googleblog.com A2A announcement;
rapidclaw.dev A2A guide; zylos.ai comparison, accessed 2026-07-05):
agent-to-agent delegation of full tasks. Both parties are autonomous agents;
neither exposes internal implementation. Structured `Task` object with
lifecycle states (`submitted → working → input-required → completed/failed/canceled`).
Artifacts (typed outputs: text, files, structured data). Agent discovery via
`AgentCard` at `/.well-known/agent.json` advertising capabilities, auth
requirements, supported task types. JSON-RPC 2.0 over HTTPS; SSE streaming
for partial results; gRPC bindings also available.

**Spec version and governance.** VERIFIED: v1.0 stable reached April 2026;
governed by Linux Foundation Agentic AI Foundation. Grew to 150+ production
organisations by April 9, 2026. Production deployments at Microsoft Copilot
Studio, Azure AI Foundry, Amazon Bedrock AgentCore, Salesforce, SAP,
ServiceNow. April 2026 added Signed Agent Cards and the Agent Payments
Protocol (AP2). IBM's ACP (a separate protocol of the same initialism,
not Zed's ACP) merged into A2A under Linux Foundation governance in
August 2025 — this is a distinct protocol from the ACP this project uses
and should not be confused with it.

**A2A vs ACP for our use case (main agent delegates whole competence domain
to cheap specialist that returns evidence reports).**

| Dimension | ACP (Zed) | A2A (Google/LF) |
|---|---|---|
| Primary use case | Editor/IDE ↔ coding agent | Agent ↔ agent task delegation |
| Session model | Named, persistent sessions with history | Stateful Task objects with lifecycle |
| Streaming | `session/update` notifications | SSE streaming of partial results |
| Discovery | Registry (stabilised March 2026) | AgentCard at `/.well-known/agent.json` |
| Internal opacity | Adapter-dependent | By design: "collaborate without exposing internal reasoning" |
| Tool surface control by caller | `session/request_permission` only | Caller does not control specialist's tools |
| Auth | Session-level, adapter-managed | Per-agent mTLS / OAuth 2.0 / OIDC |
| Governance | Zed BDFL, Apache 2.0 | Linux Foundation, Apache 2.0 |
| Interop with MCP | MCP server config passed in `session/new` | Specialists use MCP for their own tools |
| Maturity | Protocol v1, schema v1.17.0, 30+ agents | Spec v1.0, 150+ orgs in production |

**Assessment for ghx.** A2A is architecturally closer to the sidecar
boundary contract (English question in → evidence report out, whole task
delegated, specialist opacity preserved). The `input-required` lifecycle
state enables multi-turn clarification. SSE streaming fits evidence
accumulation. AgentCard discovery enables "any main agent finds ghx as a
registered reconnaissance specialist" without bespoke integrations.

However, A2A has no concept analogous to ACP's `session/request_permission`
for tool denial — the caller cannot enforce read-only semantics on the
specialist. For ghx's evidence-only contract, this is enforced today at
the `denyClient` layer; the same enforcement would move into the A2A
server-side specialist implementation.

ACP-to-A2A interoperability: INFERRED from the zylos.ai analysis
(accessed 2026-07-05) — "an A2A agent cannot yet natively delegate to
an ACP agent and receive a response in a unified task lifecycle"; a
Q3 2026 MCP/A2A joint specification is the first formal step. No
ACP-to-A2A bridge exists today.

**Convergence signal.** INFERRED: the protocol ecosystem is converging on
a two-layer model: MCP for tool access (vertical), A2A for agent delegation
(horizontal). ACP sits at the intersection (editor-to-agent, which is a
special case of agent-to-agent). No formal convergence of Zed's ACP with
A2A is documented or announced as of 2026-07-05.

#### AG-UI (Agent-User Interaction Protocol)

**What it is for.** VERIFIED (docs.ag-ui.com, aws.amazon.com/whats-new,
accessed 2026-07-05): AG-UI is for connecting AI agents to **user-facing
frontend applications** — streaming chat, state sync, UI component
generation, human-in-the-loop approvals, sub-agent composition with scoped
state. Event-based, ~16 standard event types, works over SSE/WebSockets.
First-party support from Microsoft Agent Framework, Google ADK, AWS Strands
Agents; Bedrock AgentCore Runtime added AG-UI support March 13, 2026.

**Fit for our sidecar boundary.** Not a fit. AG-UI is explicitly positioned
as the agent ↔ user layer; A2A handles agent ↔ agent coordination.
AG-UI's sub-agent composition feature is for composing agents within a
user-facing UI context, not for the headless sidecar delegation pattern ghx
implements. The protocol layering from the spec: MCP (tools), A2A (agent
coordination), AG-UI (user interaction). Our sidecar boundary is the A2A
layer by analogy, not the AG-UI layer. AG-UI is stealable inspiration for
the full-visibility surface (NORTH_STAR M6 trace viewer, human-in-the-loop
approvals) but not as the wire protocol.

#### Protocol Ecosystem Summary

```
User <--[AG-UI]--> Main Agent <--[A2A]--> Specialist Agent <--[MCP]--> Tools
                      |
                   [ACP today: editor-to-agent boundary;
                    overlaps with A2A for external deployment]
```

---

### Q3. Agent SDK Harnesses for the "Below ACP" Path (P4)

#### Claude Agent SDK

**What it is.** VERIFIED (code.claude.com, accessed 2026-07-05): Python
and TypeScript library running the Claude Code CLI as a child process.
Renamed from Claude Code SDK in September 2025. Ships with subagents,
sessions, MCP support, and a hosted execution model.

**Context window control.** Fresh context per subagent; no parent history
leaks in. Only channel parent→subagent is the `agents.prompt` string
(system prompt) + the Agent tool's prompt argument. Auto-compaction at
context limits. Session resume via `resume: sessionId` preserves full
transcript across query() calls.

**Tool registry/isolation.** Per-subagent via `AgentDefinition.tools`
(allowlist), `disallowedTools` (denylist including MCP server globs),
`permissionMode`, and `mcpServers` (scoped MCP server access).
`disallowedTools: ["mcp__*"]` removes all MCP tools from a subagent.
Nested subagent spawning up to 5 levels deep (as of v2.1.172).

**Model choice.** Accepts Anthropic model aliases and full model IDs only
via `AgentDefinition.model`. Custom weights require `ANTHROPIC_BASE_URL` →
vLLM/LiteLLM proxy (VERIFIED; see Trigger 4 above). This means the harness
still requires Claude Code CLI binary to be present; it is not a
model-agnostic runtime.

**OTel/telemetry hooks.** VERIFIED (code.claude.com/docs/en/agent-sdk/observability,
accessed 2026-07-05): The SDK exports three OTel signals — traces (beta,
requires `CLAUDE_CODE_ENHANCED_TELEMETRY_BETA=1`), metrics, and log events —
via standard OTLP. Span hierarchy: `claude_code.interaction` → `claude_code.llm_request` +
`claude_code.tool` + `claude_code.hook`. Subagent spans nest under the parent
agent's `claude_code.tool` span — the full delegation chain is one trace.
W3C trace context propagated into child CLI processes and into Bash tool
subprocesses. Custom resource attributes via `OTEL_RESOURCE_ATTRIBUTES`.
Content opt-in via `OTEL_LOG_USER_PROMPTS`, `OTEL_LOG_TOOL_DETAILS`,
`OTEL_LOG_TOOL_CONTENT`, `OTEL_LOG_RAW_API_BODIES`. Structural telemetry
(durations, model names, tool names, token counts) is on by default.

**GenAI semantic conventions.** The community package
`opentelemetry-instrumentation-claude-agent-sdk` (github.com/justinbarias/opentelemetry-instrumentation-claude-agent-sdk)
auto-generates OTel GenAI semantic convention spans and metrics via SDK
hooks — zero code changes. This is stealable open-source-leverage tenet
material for our ADR-0018 implementation.

**Non-Anthropic model serving (critical for P4/M10).** VERIFIED: The
architecture requires the Claude Code CLI binary (which is closed-source and
Anthropic-published). When pointing `ANTHROPIC_BASE_URL` at vLLM, the CLI
still runs and wraps the custom model but the CLI binary itself cannot be
swapped. For a fully custom model with modified agent loop behaviour, the
OpenAI Agents SDK is more appropriate (see below).

**Summary for ghx.** The Claude Agent SDK gives meaningful improvements
over ACP on Triggers 1–3 (thinking control, context isolation, tool
registry). It gives only a partial answer on Trigger 4 (custom model
weights are routable but the CLI harness is fixed). OTel conformance with
GenAI conventions is achievable with the community instrumentor. The
subagent session model matches ghx's named-session evidence-ledger contract.

#### OpenAI Agents SDK

**What it is.** VERIFIED (openai.github.io/openai-agents-python, accessed
2026-07-05): Python (and TypeScript) library for orchestrating multi-agent
workflows. Native OpenAI model support (`OpenAIResponsesModel`,
`OpenAIChatCompletionsModel`). Per-agent model override via `Agent.model`.

**Context window control.** "Local context" is a typed Python object never
sent to the LLM. LLM context flows through conversation history only.
Context management: `"auto"` truncation, or server-side compaction
(`compact_threshold`). Context of nested agents is isolated by default
when using `Agent.as_tool()`.

**Tool registry/isolation.** Tools defined per agent. No explicit MCP
server glob exclusion mechanism documented (unlike Claude Agent SDK's
`mcp__server__*` patterns). Function tools, agents-as-tools, MCP servers,
and built-in tools are composable. Tool isolation mechanics not explicitly
documented at the per-agent level — INFERRED to be per-agent definition.

**Model choice, including custom/self-hosted weights.** VERIFIED
(openai.github.io/openai-agents-python/models, accessed 2026-07-05):
first-class support. Create `AsyncOpenAI(base_url=<vllm-url>, api_key=<any>)`,
wrap in `OpenAIChatCompletionsModel`, assign to `Agent.model`. LiteLLM and
Any-LLM adapters add multi-provider routing. This is the clearest path
to running a custom `ghx-sidecar` fine-tuned model: no closed-source binary
dependency, no CLI wrapper, the agent loop is the SDK itself. For M10, this
is more flexible than the Claude Agent SDK path.

**OTel/telemetry hooks.** VERIFIED (openai.github.io/openai-agents-python,
2026-07-05): custom processor interface; `set_tracing_disabled()` opt-out;
`set_tracing_export_api_key()` for non-OpenAI trace destinations. The tracing
is not GenAI-semconv-native — no built-in OTel GenAI conventions. The
community MLflow auto-instrumentation covers it (mlflow.org, accessed
2026-07-05). For our ADR-0018 requirement (official OTel GenAI semantic
conventions), we would need to instrument manually or use a community
wrapper, vs the Claude Agent SDK's built-in OTLP export.

**Limitation for ghx today.** Our entire existing sidecar runtime
(`internal/sidecar/`) speaks ACP via `coder/acp-go-sdk` and spawns the
agent subprocess directly. Migrating to OpenAI Agents SDK would require
a Python/TypeScript harness wrapping our Go binary — the inverse of the
current architecture. This is not a lightweight P4 migration.

#### Other Harnesses (scoped survey)

**LangGraph.** INFERRED from search results: agent loop with node-based
state graphs; context is explicit state objects; any model via LangChain
integration; OTel via community MLflow instrumentation. Not evaluated in
depth: the node graph mental model differs from ghx's sequential
evidence-accumulation pattern, and LangChain's abstraction layer adds
indirection over raw model calls. No strong prior-art pull toward it.

**Google ADK (Agent Development Kit).** VERIFIED signal: first-party A2A
and AG-UI support (Google ADK 1.0, April 2026); designed around A2A task
lifecycle. INFERRED: if ghx migrates to A2A as its primary boundary
protocol (rather than internal harness), ADK is a candidate harness for
the A2A server side. Out of scope for current evaluation.

**fast-agent.** Listed in ACP agents list (agentclientprotocol.com/get-started/agents).
INFERRED: a lightweight Python harness; not evaluated for depth of context
control, tool isolation, or OTel support.

---

### Q4. Custom Model Serving Reality (M10)

**Production pattern.** VERIFIED (vLLM docs, vllm.ai; sitepoint.com vLLM
guide, 2026-07-05): vLLM serves fine-tuned models (full-weight or LoRA) via
OpenAI-compatible HTTP API (also implements Anthropic Messages API for Claude
Code). High throughput via PagedAttention; FlashAttention-2; concurrency
suitable for multi-session sidecar serving. `--served-model-name` aliases
the model for SDK routing. `--enable-auto-tool-choice` required for tool
calling.

**Devin's architecture as prior art.** VERIFIED (singularitymoments.com,
accessed 2026-07-05): Devin uses "a swarm of specialized models": Planner
(high-reasoning), Coder (trained on trillions of tokens of code), Critic
(adversarial reviewer), Browser (documentation agent). Cognition AI trained
Devin via RL on software engineering tasks. The exact infrastructure is not
disclosed. INFERRED: each specialist model is served behind a router that
selects the appropriate specialist by task type — the same pattern ghx's
sidecar embodies at the framework level.

**Path for ghx M10.** INFERRED synthesis:
1. Train `ghx-sidecar` model via SFT/RLHF on M9 trajectory exports
2. Serve via vLLM with `--served-model-name ghx-sidecar --enable-auto-tool-choice`
3. Point the Claude Agent SDK (via `ANTHROPIC_BASE_URL`) or OpenAI Agents SDK
   (via `AsyncOpenAI(base_url=...)`) at the vLLM endpoint
4. The agent loop (persona, tool calling, evidence report generation) runs
   identically to today, but the model weights are ghx-trained

The critical constraint: the model must support tool calling. ADR-0016's
training-data gold mine (tool call traces, episode JSON with full
`toolTraces`, `actions`, `observations`) provides exactly the structured
turn data needed for SFT of a tool-calling specialist.

---

### Q5. Prior Art on the Sidecar Pattern

**Industry sidecar-shaped patterns.** Research found several relevant
prior-art signals:

1. **Devin's specialist model swarm.** VERIFIED (above): multiple specialist
   models per competence domain (planning, coding, browsing, critique). The
   closest production match to the ghx Agent Sidecar Framework idea.
   Stealable: the "Critic" pattern (an adversarial model reviewing its own
   output) is relevant to the judge scorer (ADR-0016.x forthcoming).

2. **OpenCode's role-based sub-agent spawning.** VERIFIED (search results,
   2026-07-05): OpenCode uses task tools to spawn sub-agents with different
   specialisations and scaffold-enforced tool permissions; the LLM chooses
   which specialist to invoke. Open source. Stealable: the sub-agent
   specialisation dispatch pattern is directly applicable to P3 (codemap as
   an internal sidecar tool) and P4 (custom model selection by task type).

3. **A2A remote specialist delegation.** VERIFIED: A2A's documented
   production pattern is orchestrator + specialists, each specialist accessed
   via AgentCard discovery. "An orchestrator uses A2A to delegate to specialist
   agents, and each specialist uses MCP to reach the tools it needs."
   (zylos.ai, accessed 2026-07-05.) This is the industry-level mental model
   closest to the Agent Sidecar Framework claim.

4. **Claude Agent SDK subagent pattern.** VERIFIED (code.claude.com, 2026-07-05):
   the SDK's built-in "general-purpose subagent" can be delegated exploration
   tasks without custom agent definitions. The `research-assistant` example
   in the docs ("explore dozens of files without any of that content
   accumulating in the main conversation") is verbatim the ghx sidecar value
   proposition. INFERRED significance: the Claude Agent SDK is converging
   toward the same domain model — the framework is validated by the platform.

5. **AgentGateway.** VERIFIED (agentgateway.dev, accessed 2026-07-05):
   open-source Rust proxy (Solo.io → Linux Foundation, v1.0 March 2026,
   Apache 2.0, ~2K GitHub stars). Bridges MCP, A2A, and LLM routing in one
   data plane. Stealable: the MCP multiplexing feature (multiple MCP servers
   consolidated into one endpoint) is useful for P3, where ghx CLI + codemap
   CLI + local clone tooling could all be surfaced as one MCP endpoint to
   the sidecar agent.

6. **"Sidecar security pattern for agent communications."** VERIFIED:
   USPTO patent 12505131 (found via search, 2026-07-05) covers a "sidecar
   security pattern for agent communications" — a structural pattern in which
   an agent's communication channel is intercepted or augmented by a sidecar
   process. This documents the pattern's independent arrival in the security
   space; no IP risk is apparent for our use of the term in the software
   architecture sense.

**What is genuinely missing that ghx provides.** INFERRED synthesis:
- **Reconnaissance specialisation as a first-class product.** All sidecar
  patterns found are generic (code review, test execution, security scan).
  ghx's specialisation (GitHub code exploration doctrine, map-before-read
  discipline, tier escalation to codemap/local) is novel — no open-source
  equivalent was found.
- **Evidence-shaped output contract.** A2A's Artifact model and Claude
  Agent SDK's subagent final-message model both return opaque text. ghx's
  `<ghx-report>` schema (question, answer, relevant files, evidence snippets,
  commands run, backends used, uncertainty, suggested next reads) is not
  replicated anywhere in the found prior art.
- **SPT measurement at three levels.** The ADR-0016.6 signal-per-token
  framework (main-agent, sidecar-internal, whole-workflow) as a measurement
  discipline for sidecar evaluation is not found elsewhere. All competing
  benchmarks found measure throughput or task completion, not context economy.

---

## Open Questions for ADR-0020.1

These are the decision points the architecture ADR must settle, with evidence
pointers for each.

**OQ-1. ACP vs A2A as the primary sidecar boundary protocol.**
ACP: lower operational friction today (runtime shipped, 30+ agents, our
Go SDK integration live). A2A: architecturally closer to the delegated-task
model, 150+ organisations, Linux Foundation governance, cleaner opacity
semantics. Key evidence: Q2 comparison table above; zylos.ai protocol
comparison; A2A v1.0 April 2026 scope (Signed Agent Cards, Agent Payments).
Decision point: does ghx optimise for editor integration (ACP's home) or for
agent-to-agent delegation (A2A's home)? The two are not identical — ACP's
primary adopters are IDEs; A2A's are orchestrator frameworks.

**OQ-2. Timing of P4 migration — which trigger fires first?**
Current evidence: Trigger 1 (thinking emission) is a known cost but our
`acp.go` gap is patchable (handle `agent_thought_chunk`) without leaving ACP.
Trigger 3 (eval tool isolation) is already worked around at the runner level.
Trigger 2 (SPT plateau) has not been measured as harness-attributable vs
persona-attributable. Trigger 4 (custom model) is the hardest forcing
function because ACP has no path at all. Decision point: should the P4 move
be deferred until Trigger 4, or should Trigger 2 measurement be added to
the next gate run to detect harness overhead earlier?

**OQ-3. Claude Agent SDK vs OpenAI Agents SDK for P4.**
Claude Agent SDK: native OTel OTLP export, built-in GenAI semconv community
instrumentor, tighter tool-surface control, session persistence — but the
CLI binary is closed and the model abstraction requires vLLM proxy for custom
weights. OpenAI Agents SDK: fully open model routing (any OpenAI-compatible
endpoint per agent), explicit per-agent model selection, but no native OTel
GenAI conventions and a Python/TypeScript harness that inverts our Go-native
architecture. Decision point: which matters more for M10 — harness openness
(OpenAI SDK path) or toolchain continuity + OTel compliance (Claude SDK path)?

**OQ-4. A2A adoption — incremental vs architectural shift.**
A2A is HTTP-native (AgentCard at `/.well-known/agent.json`, JSON-RPC 2.0).
Adopting A2A for external deployment (M5 "main agent needs zero ghx CLI
knowledge") does not require abandoning ACP internally — ghx could present
an A2A interface externally while the internal sidecar brain remains ACP or
moves to an SDK. Decision point: should ghx publish an AgentCard and expose
an A2A endpoint as the "one small skill" main agents use, independently of
the internal harness decision?

**OQ-5. OTel GenAI conventions convergence.**
ADR-0018 proposes emitting official OTel GenAI conventions from our custom
Go exporter. The Claude Agent SDK emits OTel OTLP natively (with beta traces
flag). If we migrate to the Claude Agent SDK for P4, the OTel emission
question partially resolves — but the schema gap (ADR-0018's Log events for
content, `gen_ai.evaluation.result` for scores) remains our own work
regardless of harness. Decision point: should ADR-0018 implementation
prioritise the ACP path (custom Go OTel exporter) or the SDK path (hook
into Claude Agent SDK telemetry)?

**OQ-6. AgentGateway as infrastructure.**
AgentGateway (Solo.io, Linux Foundation, Apache 2.0, v1.0 March 2026)
bridges MCP, A2A, and LLM routing in one data plane. For P3 (codemap +
ghx CLI as internal sidecar tools), its MCP multiplexing feature could
consolidate the tool surface into one endpoint the sidecar brain addresses.
Decision point: evaluate AgentGateway for P3 tool consolidation before
building a custom MCP aggregator.

---

## Cross-References

| Reference | Why it matters here |
|---|---|
| NORTH_STAR "Protocols are stepping stones" tenet (commit 1620b2e) | Defines the four exit triggers this research answers; establishes "below ACP means adopting an existing SDK, not authoring a wire protocol" |
| ADR-0014.1 | First protocol landscape survey; ACP/MCP/codex-app-server analysis that the current research updates |
| ADR-0015 | Why ACP was chosen and Go SDK migration; the `denyClient` permission contract; Trigger 3 context |
| ADR-0016.3 | Documented the tool-surface isolation gap (eval isolation) — Trigger 3 evidence |
| ADR-0018 | Documents `agent_thought_chunk` emission as undocumented upstream (adapter tarball was the authority); Trigger 1 evidence; OTel GenAI conventions goal |
| ADR-0016.6 | SPT definition and measurement at three levels; Trigger 2 measurement framework |
| ADR-0019 | Current M5 frontier (zero-CLI-surface); any protocol change must preserve the recon-service boundary this ADR establishes |

---

## Primary Sources Consulted

- agentclientprotocol.com/get-started/introduction — ACP overview, accessed 2026-07-05
- agentclientprotocol.com/get-started/agents — ACP agent adoption list, accessed 2026-07-05
- agentclientprotocol.com/get-started/architecture — ACP architecture, accessed 2026-07-05
- agentclientprotocol.com/rfds/about — ACP RFD governance process, accessed 2026-07-05
- agentclientprotocol.com/updates — ACP protocol update history, accessed 2026-07-05
- agentclientprotocol.com/protocol/schema — ACP schema, SessionUpdate kinds, `_meta`, session methods, accessed 2026-07-05
- github.com/agentclientprotocol/agent-client-protocol — ACP repo, schema v1.17.0 (June 29, 2026), governance, accessed 2026-07-05
- github.com/agentclientprotocol/claude-agent-acp/issues/297 — "Enabling extended thinking" issue (open, Feb 2026), accessed 2026-07-05
- github.com/kirodotdev/Kiro/issues/7099 — ACP agent_thought_chunk Kiro issue (open, Apr 2026), accessed 2026-07-05
- code.claude.com/docs/en/agent-sdk/subagents — Claude Agent SDK subagents full spec, accessed 2026-07-05
- code.claude.com/docs/en/agent-sdk/observability — Claude Agent SDK OTel full spec, accessed 2026-07-05
- openai.github.io/openai-agents-python/models — OpenAI Agents SDK model routing, accessed 2026-07-05
- openai.github.io/openai-agents-python/context — OpenAI Agents SDK context management, accessed 2026-07-05
- docs.vllm.ai/en/stable/serving/integrations/claude_code — vLLM + Claude Code integration, accessed 2026-07-05
- docs.ag-ui.com/introduction — AG-UI overview and protocol layering, accessed 2026-07-05
- developers.googleblog.com/en/a2a-a-new-era-of-agent-interoperability — A2A announcement (Google), accessed 2026-07-05
- rapidclaw.dev/blog/a2a-protocol-complete-guide-2026 — A2A v1.2 spec and task lifecycle, accessed 2026-07-05
- zylos.ai/research/2026-03-26-agent-interoperability-protocols-mcp-a2a-acp-convergence — MCP/A2A/ACP comparison, accessed 2026-07-05
- zuplo.com/blog/agent-protocol-stack-mcp-a2a-acp-2026 — Protocol stack roles; IBM ACP → A2A merger note, accessed 2026-07-05
- stellagent.ai/insights/a2a-protocol-google-agent-to-agent — A2A 150+ orgs, April 2026 v1.0, accessed 2026-07-05
- agentgateway.dev — AgentGateway: Solo.io → Linux Foundation, v1.0 March 2026, accessed 2026-07-05
- singularitymoments.com/devin-ai-coding-agent-guide — Devin specialist model swarm architecture, accessed 2026-07-05
- github.com/justinbarias/opentelemetry-instrumentation-claude-agent-sdk — Community OTel GenAI conventions instrumentor for Claude Agent SDK, accessed 2026-07-05

---

## Claims That Could Not Be Verified

1. **ACP schema v1.17.0 release content.** The GitHub releases page returned
   "June 29, 2025" for v1.17.0 (likely a page rendering artefact — the Updates
   page lists May 2026 announcements). Unable to fetch the raw release notes;
   the schema version number is VERIFIED but the exact changelog is not.

2. **ACP-to-A2A interoperability timeline.** The "Q3 2026 MCP/A2A joint
   specification" was cited in one source (zylos.ai) but not found in official
   MCP or A2A announcements. Treated as INFERRED.

3. **Claude Agent SDK model field accepting full model IDs only.** The
   AgentDefinition schema table lists aliases and a `model` string field; the
   exact validation (whether it rejects non-Anthropic endpoint strings) was
   not verified by running code. The ANTHROPIC_BASE_URL workaround is
   VERIFIED independently.

4. **Devin's internal serving infrastructure.** Cognition AI does not
   disclose the stack. The specialist model architecture is VERIFIED from a
   secondary analysis source; the serving runtime is INFERRED.

5. **IBM ACP merger date.** The Zuplo article states "August 2025"; the
   stellagent.ai article states "August 2025." This is consistent but sourced
   from secondary analysis, not a primary announcement. Note: "IBM ACP" is a
   different protocol from "Zed ACP" (agentclientprotocol.com) — the merger
   is between IBM's Agent Communication Protocol and Google's A2A, not between
   Zed's ACP and A2A. This distinction is critical and confirmed by both
   sources.

---

## Suggested Next Step

Write ADR-0020.1 to settle the six open questions above. The most
time-sensitive decision is OQ-4 (A2A external interface) because M5
(ADR-0019, zero-CLI-knowledge surface) ships before P4 — if ghx is going to
publish an AgentCard and accept A2A task requests as its "one small skill"
surface, that decision shapes M5's integration design, not just M10's harness
choice. OQ-3 (SDK choice) and OQ-5 (OTel convergence) are the next most
consequential and can be resolved together since ADR-0018 implementation is
already on the M6 frontier.

---

## Addendum: Source-Level ACP Steering Recon (2026-07-05, same day)

A parallel read-only source recon (Codex worker; local Go module cache +
the shipped `claude-agent-acp@0.55.0` / bundled `claude-agent-sdk@0.3.198`
sources; spot-verified by Fable at the cited lines) **materially revises
the Trigger 2/3 picture above**. The web research correctly reports what
the ACP *spec* offers; the shipped *adapter* offers far more through the
`_meta` extension bag.

**Founder's challenge, answered: "prompt text is the only steering ACP
gives us" is REFUTED.** True only of our current code (`acp.go` sends bare
`NewSessionRequest{Cwd, McpServers}`). The adapter forwards session-level
`_meta.claudeCode.options` into the full Claude Agent SDK options object
(`acp-agent.js:2768–2801`, spread as `...userProvidedOptions` over adapter
defaults), and `_meta.systemPrompt` as a top-level shortcut
(`acp-agent.js:2748`: a plain string becomes the custom system prompt).

Steering available today, per session, without leaving ACP (evidence:
`sdk.d.ts` / `acp-agent.js` line references in the recon report):

| Option | Effect for ghx |
|---|---|
| `systemPrompt` (string or preset+append, cache-boundary support) | Persona doctrine moves from per-turn user prompt to cacheable system prompt — Trigger 2 relief |
| `tools` (allowlist; `[]` disables built-ins) + `disallowedTools` | Real tool-surface restriction — Trigger 3 relief; note `allowedTools` is auto-approve, NOT an allowlist |
| `settingSources: []` | Disables filesystem settings inheritance — closes the ADR-0016.3 leak where subject agents inherited the host's user/project settings (adapter default is `["user","project","local"]`, verified at `acp-agent.js:2800`) |
| `strictMcpConfig: true` + explicit `mcpServers` | MCP isolation |
| `model` | Per-session model pinning (structurally prevents the silent-wrong-subject-model eval incident) |
| `maxTurns`, `maxBudgetUsd`, `thinking`, `effort` | `--depth` budgets become mechanical, not prompt-text honor system |
| `emitRawSDKMessages` | Raw SDK firehose for audit/eval telemetry (adapter-specific) |

Caveats verified: prompt-level `_meta` is ignored by the adapter (steering
is session-scoped only); `permissionMode` set at creation is overwritten —
use `session/set_mode` after `NewSession` (Go SDK: `SetSessionMode`,
`client_gen.go:300`). All of this is adapter-specific, not portable ACP —
the spec-level findings above stand for any other adapter. Version skew:
`claude-agent-acp@0.55.0` is latest (published 2026-07-02);
`coder/acp-go-sdk` has v0.13.5 (June 2, 2026) vs our v0.13.0 pin.
