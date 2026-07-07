---
title: "Architecture Vision — the Runner port: making the agentic runtime plug-in-replaceable"
date: "2026-07-07"
status: "audit / target design"
author: "background architecture-vision worker (ports-and-adapters)"
scope: "internal/sidecar/** — read-only design audit, no code changes; grounded in file:line + text-only SDK/protocol research"
---

# Runner port & pluggable runtimes — target design

Read-only design audit. The founder's top architectural priority: today ghx
**couples the agentic runner (`claude-agent-acp` over ACP) into the rest of the
product**, and that adapter is hitting limits. The goal is not to eradicate
ACP — it is an architecture where *"the ACP, or a runner, is plug-in-replaceable
with configuration and with ease, because we avoid coupling of runners from the
rest of the product."* Concretely, via config, the founder must be able to
(a) add a **Codex ACP** runner and (b) **entirely replace** the ACP runner with
the **Claude Agents SDK** as the main agentic runtime.

This document maps every place the ACP adapter leaks into the rest of ghx today
(file:line), designs the **Runner port** (small interface at the consumer,
adapters behind it), shows the three adapters and how config selects one,
reconciles resilience and the frozen eval-anomaly measurement contract across
the boundary, and gives a phased, non-rewrite sequence that ships the port with
**one** adapter first and proves it with a **second**.

It is consistent with **ADR-0015** (Go-native ACP chosen deliberately — the port
demotes ACP from *the runtime* to *one adapter*, it does not delete it) and
**NORTH_STAR P4 / "Protocols are stepping stones, not identity"** (the boundary
must survive replacing ACP, the brain, or both). The port is an **internal Go
interface**, never a new wire protocol — every adapter adopts an existing SDK
(`coder/acp-go-sdk`, `codex-acp`, `anthropic-sdk-go` / `@anthropic-ai/claude-agent-sdk`),
so it passes the open-source-leverage filter (AGENTS.md "Open Source Leverage").

## Executive summary

**The good news, verified first:** the *measurement-facing* side of the boundary
is already neutral. `TurnResult` (`internal/sidecar/turnresult.go:6-64`) and
`ToolCallTrace` (`turnresult.go:79-94`) import only `time` — they carry **no ACP
types** — and the whole eval/telemetry stack (`emit.go`, `evals/anomalies.go`,
gates) reads those plain structs. So the frozen measurement stack does **not**
have to change to swap runtimes; a new adapter just has to *populate the same
struct*. That is the single most important fact for a low-risk migration.

**The leak is entirely on three surfaces the runtime touches on the way *in* and
on the *classification* path on the way *out*:**

1. **Wire encoding of steering is claude-agent-acp-shaped.** `SessionOptions`
   (`session_options.go:12-97`, self-labeled "ADAPTER-SPECIFIC" at :10-11) is a
   direct mirror of `claude-agent-acp` SDK option names, and `BuildSessionMeta`
   wraps it under the literal `claudeCode` key (`session_options.go:222-224`).
   **This is the killer for goal (a):** config already lists `codex` as a known
   agent (`config.go:288-292`) and `codex-acp` exists and completes ACP
   `initialize` — but `codex-acp` reads its steering from `CODEX_CONFIG` /
   `MODEL_PROVIDER` / `INITIAL_AGENT_MODE` **env vars**, *not* from
   `_meta.claudeCode.options`. So a `codex` runner today would spawn, pass
   preflight, and then **silently drop the entire steering surface** — persona,
   isolation, tool policy, model pin, thinking. "Add a Codex runner via config"
   is currently a trap, not a feature.

2. **Failure classification is string-matching adapter error text.** The runtime
   decides recovery by sniffing `claude-agent-acp` strings —
   `IsMaxTurnsError` ("Reached maximum number of turns"),
   `IsPeerClosedError` ("peer connection closed"),
   `IsLoadSessionResourceNotFound` (JSON-RPC `-32002`) — at `runtime.go:171,445`
   and `daemon_worker.go:158,167` (`acp.go:59-106`). A different runtime emits
   different strings, so recovery breaks silently.

3. **The turn engine *is* the ACP transport.** `RunTurnWithOptions`
   (`acp.go:218-315`), `configureACPSession`/`runPrompt`/`waitForACPPrompt`
   (`turn.go:38-133`), and the warm `AgentWorker` (`daemon_worker.go:90-268`)
   all operate directly on `*acp.ClientSideConnection` and a subprocess. The
   request type even carries `AgentCmd`/`Env` (`acp.go:159-163`) — a hard
   "the runner is always an external subprocess" assumption that goal (b) breaks:
   the Claude Agents SDK path is **in-process** (no Go binding exists — see
   Adapters — so it is either a pure-Go loop over the official `anthropic-sdk-go`
   or a bridge to the TS SDK, neither of which is an ACP stdio subprocess).

**The stepping stone already exists.** `TurnRunner`
(`runtime.go:18-20`, `type TurnRunner func(context.Context, RunTurnOptions)
(TurnResult, string, error)`) is *the right seam at the wrong altitude*: it
already decouples "who runs a turn" (daemon warm worker vs daemonless one-shot)
from `Ask`. It leaks only because (i) its request `RunTurnOptions` carries
subprocess + `claudeCode` specifics, (ii) its resume handle is a bare ACP
session-id `string`, and (iii) the runtime reaches *around* it to classify
failures via ACP string-matchers. Promote it from a `func` over ACP-flavored
structs to a `Runner`/`Session` interface over **neutral types + a typed
`Outcome`**, and it *becomes* the port. Not a dead end — it is phase 0.

**The payoff to keep in view:** the in-process SDK runner does not merely
*translate* the leaks — it **removes several structurally**. No permission wire
(deny-writes becomes "never register a write tool"); native thinking blocks
(no `_meta...thinking.display` tarball archaeology, ADR-0018); native `usage`
on every response (real tokens in production, closing TRUST H5 — today prod
tokens are a **char proxy**, `emit.go:352-353`, and real tokens are eval-only
via a claude-specific extension, `rawsdk.go:18-20`). The port is what turns
NORTH_STAR P4 "below ACP" from a rewrite into a config flip.

---

## The leak map

Every row: an adapter-specific concern → where it leaks into the rest of ghx
today (file:line) → where it belongs under the port. "Neutral" = no `acp`
import; carried by the port's harness-neutral types.

| # | Adapter-specific concern | Where it leaks today (file:line) | Where it belongs |
|---|---|---|---|
| L1 | **Steering wire encoding** — `SessionOptions` mirrors `claude-agent-acp` SDK option names (`systemPrompt`, `settingSources`, `strictMcpConfig`, `tools`, `allowedTools`, `maxTurns`, `thinking.display`, `effort`, `model`) | `session_options.go:12-97` (self-labeled ADAPTER-SPECIFIC :10-11); built by `BuildSessionMeta` `:194-230` | Adapter-private. The port carries a neutral `SteeringSpec`; the ACP adapter translates it into `SessionOptions`; codex/SDK adapters translate it into their own channel. |
| L2 | **The `claudeCode` meta key** — persona/isolation/model bag is nested under a claude-only namespace | `session_options.go:222-224` (`claudeCode := map[string]any{"options": opts}`); emitRaw sibling `:225-228` | Adapter-private encoding. Nothing outside the ACP adapter should know the string `"claudeCode"`. |
| L3 | **Steering wired into the turn request** — `RunTurnOptions.SessionMeta map[string]any` is the `claudeCode` bag; `AgentCmd`/`Env` assume a subprocess | `acp.go:157-197`; assembled `runtime.go:405,424-436`; applied `turn.go:48,60-62,74` | `TurnRequest{Prompt, Steering, Budget}` — neutral. Subprocess `command`/`env` move into the ACP adapter's own config, off the port. |
| L4 | **Permission taxonomy** — read/search/execute/fetch/think allow vs write/unknown deny is expressed in `acp.ToolKind*`; approval in `acp.PermissionOptionKind*` | `denyclient.go:123-132` (`isWriteToolKind`), `:158-187` (`RequestPermission`), stubs `:277-302` | Neutral `ToolPolicy` intent on `SteeringSpec` ("allow shell+read, deny writes"). Each adapter enforces it in its native mechanism (ACP permission callback; codex sandbox/approval; SDK: simply never register a write tool). |
| L5 | **MCP-tool-classified-as-"other" workaround** — because `claude-agent-acp` labels every MCP call ACP kind "other" (write-shaped), the report-sink tool needs an `allowedTools` auto-approve bypass | `session_options.go:60-63,213`; `denyclient.go:161-166,138-144` | Adapter-private. The port exposes `ReportSink` *intent* (path + tool id); the adapter owns how it gets approved on its runtime. |
| L6 | **Report-sink registered as ACP MCP servers** — `[]acp.McpServer` on NewSession/LoadSession | `reportsink_exe.go` (`reportSinkMcpServers`); `turn.go:47,58,74`; id `reportsink.go:29,38` | Neutral `ReportSink{Path, ToolID}` on `SteeringSpec`; adapter maps to its tool-delivery mechanism (ACP mcpServers; SDK in-process MCP; codex mcpServers). |
| L7 | **Turn engine == ACP transport** — spawn + `acp.NewClientSideConnection` + `Initialize`/`NewSession`/`LoadSession`/`Prompt` | `acp.go:218-315`; `turn.go:38-133`; warm worker `daemon_worker.go:90-268` | Behind the port: `Session.Turn`. The ACP transport becomes one adapter's internals; the runtime never sees a connection. |
| L8 | **Failure classification by adapter string** — recovery decisions read `claude-agent-acp` error text / JSON-RPC codes | markers `acp.go:41-106`; consumed `runtime.go:171,300,445`, `daemon_worker.go:158,167` | Typed `Outcome{Class FailureClass}` returned *by the adapter*. The runtime switches on the class, never on strings. |
| L9 | **Eval-anomaly marker strings** — `LivenessTimeoutMarker` and the max-turns text double as the eval-anomaly contract read from persisted turn errors | defined `acp.go:41-68`; read cross-package `evals/anomalies.go:115,124` | Runtime-owned **canonical** marker keyed by `FailureClass` (see Resilience). Kept byte-identical so the frozen anomaly detector is untouched — but sourced from the *runtime*, not from whatever the adapter happened to print. |
| L10 | **Liveness watchdog bound to ACP session updates** — `onActivity` is fired only inside `denyClient.SessionUpdate`; watchdog lives in the ACP turn code | `turn.go:19-36` (`startACPWatchdog`); fed `denyclient.go:192-194,54` | Runtime concern. The port's `EventSink.Activity()` is the neutral heartbeat; the runtime owns the watchdog and wraps `Session.Turn`, so every runner gets the same liveness policy for free. |
| L11 | **Streamed activity typed as ACP notifications** — text/thought/tool events decoded from `acp.SessionNotification`; sizes from `[]acp.ToolCallContent`; locations from `[]acp.ToolCallLocation` | `denyclient.go:191-274`; `tooltrace.go:13-30`; `mergeLocations` `denyclient.go:101-111` | Neutral `EventSink` calls (`Text`/`Thinking`/`ToolStarted`/`ToolUpdated`). The adapter decodes its native stream into these; `ToolCallTrace` (already neutral) is the accumulation target. |
| L12 | **Resume handle is an ACP session id** — persisted as the durable resume token; capability-gated on `AgentCapabilities.LoadSession` | `runtime.go:95-103` (`persistACPSessionID`); `turn.go:42-82`; probe `acp.go:297`, `daemon_worker.go:239` | Opaque `ResumeToken` (adapter-defined). Safe because the durable ledger already rides in the prompt — "ACP resume is only an optimization" (`runtime.go:167-168`), so a runner that cannot cross-process-resume is still correct. |
| L13 | **Token accounting split** — prod tokens are a **char proxy** (`len(Question)`, `len(FullText)`, `len(Thinking)`); real tokens exist only in eval mode via the claude-specific `_claude/sdkMessage` extension | proxy `emit.go:352-353,368,413-419`; real `rawsdk.go:18-20,72-86,173-200` | Neutral, optional `EventSink.RawStreamMessage(raw)` (eval) + a neutral `Usage` slot on `TurnResult`. ACP adapter fills it from `_claude/sdkMessage`; the SDK adapter fills it from the Messages `usage` field directly (real tokens, in prod). |
| L14 | **Adapter identity typed as "ACP initialize" result** | `ImplementationInfo` "ACP adapter identity returned by initialize" `turnresult.go:66-71`; set `acp.go:289-295` | Neutral `RuntimeInfo{Kind, Name, Version}` from `Runner.Info()`; the string "ACP" leaves the shared type. |
| L15 | **Preflight == ACP handshake** — `Ask` gates on `CheckACPHandshake` (spawn + ACP `initialize`) | `runtime.go:16,274-278`; `acp.go:340-387`; detect `config.go:295-320` | `Runner.Preflight(ctx)`. ACP adapter implements it as the handshake; the SDK adapter implements it as "API key present + reachable." |
| L16 | **Config selects a runner by bare subprocess command** — `AgentCmd` is a command line, hard-coding "runner = subprocess" | `config.go:40,48,162-167`; split `acp.go:26-39` | `RunnerConfig{Kind, Command, Model, Effort, ...}`; a factory maps `Kind` → adapter. `Kind` defaults to `claude-acp` with today's command, so nothing changes by default. |

Two structural observations from the map:

- **The measurement stack reads only neutral structs already** (L11/L13/L14 land
  in `TurnResult`/`ToolCallTrace`, which have no `acp` import). This is why the
  migration can be behavior-preserving: freeze the struct, swap the populator.
- **The leaks cluster on `session_options.go`, `denyclient.go`, `turn.go`,
  `acp.go`, `daemon_worker.go`** — exactly the files AUD4's V2 finding flagged as
  the ACP adapter, and the marker set is exactly the failure-class inventory B4
  catalogued. This document's contribution is the *port* those findings imply.

---

## The Runner port

Design rule (Go idiom, AGENTS.md Engineering Tenets): **define the interface at
the consumer, keep it small, put adapters behind it.** The sole consumer is
`Ask` (`runtime.go:260`). The port and its neutral types live in a **leaf
package** `internal/sidecar/runner` that imports only stdlib — which makes
"no `acp` type leaks past the boundary" a *compile-time* guarantee the moment a
second adapter exists. `TurnResult`/`ToolCallTrace`/`Report` (already
`acp`-free) move into or are referenced from `runner` so the measurement stack
and the port share one definition.

### The interfaces

```go
package runner

// Runner is the sidecar's agentic-runtime port: it turns a built prompt into
// one bounded agentic turn of streamed activity and a harness-neutral result.
// Implementations wrap a concrete runtime — claude-agent-acp (today),
// codex-acp, or an in-process Claude Agents SDK loop. The runtime layer (Ask,
// the daemon pool) depends ONLY on this interface. No method exposes a
// subprocess, a wire connection, or an ACP type.
type Runner interface {
	// Preflight verifies the runtime can serve a turn without running a prompt
	// (binary present and speaks its protocol; or API key present and reachable
	// for an in-process runner). Replaces the ACP-handshake gate in Ask.
	Preflight(ctx context.Context) error

	// Open returns a Session for a ghx session identity. resume is the opaque
	// token persisted from a prior turn (zero on first use); the adapter decides
	// whether it can honor it. Because the durable ledger rides in the prompt,
	// an adapter that cannot resume across processes returns a fresh Session and
	// is still correct.
	Open(ctx context.Context, id SessionID, resume ResumeToken) (Session, error)

	// Info reports the runtime's identity for provenance/artifacts.
	Info() RuntimeInfo
}

// Session is one live agentic conversation. Turns are serialized by the caller
// (one ghx session == one Session); the adapter owns whatever process or client
// backs it. Close releases those resources.
type Session interface {
	// Turn runs one prompt to completion, streaming activity to sink, and
	// returns the neutral result, a refreshed ResumeToken, and a typed Outcome.
	// It never returns an error the runtime must string-match: every failure is
	// a classified Outcome (see FailureClass). sink is called from the adapter's
	// stream goroutine; the runtime's sink implementation owns stdout streaming,
	// the live.jsonl tailer, and the liveness heartbeat.
	Turn(ctx context.Context, req TurnRequest, sink EventSink) (TurnResult, ResumeToken, Outcome)

	Close()
}
```

### The harness-neutral request / steering

```go
// TurnRequest is one prompt turn with NO ACP types and NO subprocess fields.
type TurnRequest struct {
	Prompt   string       // the fully built user prompt (BuildPrompt output)
	Steering SteeringSpec // how the runtime must behave this turn
	Budget   TurnBudget   // caps + watchdog window
}

// SteeringSpec is the neutral steering surface: WHAT the runtime must do, not
// HOW an adapter encodes it. Each adapter translates it into its own channel —
// claude-agent-acp: _meta.claudeCode.options; codex-acp: CODEX_CONFIG env +
// MODEL_PROVIDER + INITIAL_AGENT_MODE; in-process SDK: fields on the Messages
// request plus a hand-rolled tool loop. A field an adapter cannot honor must
// degrade LOUDLY (a recorded warning), never silently (Visibility & Truthfulness).
type SteeringSpec struct {
	SystemPrompt   string         // persona/doctrine (cache-stable when the runtime supports it)
	Isolation      Isolation      // no host settings / no host MCP unless explicitly opted in
	ToolPolicy     ToolPolicy     // allow reconnaissance shell + read; deny writes
	Model          string         // pinned subject model ("" = runtime default)
	Effort         string         // "low"|"medium"|"high" — mapped, not a raw SDK enum
	Thinking       ThinkingBudget // Off, or a token budget
	ReportSink     ReportSink     // the submit_report contract: sink path + tool id
	AuditRawStream bool           // eval-only: capture the provider-native token/tool stream
}

// TurnBudget carries the mechanical caps. MaxTurns is a hint the adapter maps to
// its own safety net; Liveness is the runtime-owned watchdog window (0 = default,
// negative = disabled) — see Resilience.
type TurnBudget struct {
	MaxTurns *int
	Liveness time.Duration
}

// Isolation / ToolPolicy / ThinkingBudget / ReportSink / ResumeToken /
// SessionID / RuntimeInfo are small value types; ResumeToken is an opaque
// string the adapter defines (ACP session id, SDK session id, or "").
```

### The neutral stream + typed outcome (the two decouplers that matter)

```go
// EventSink receives streamed turn activity as neutral calls. The adapter
// decodes its native stream (ACP SessionNotification, or SDK message blocks)
// into these; the runtime's sink implementation is the ONLY place that knows
// about stdout, live.jsonl, and the watchdog. This is denyClient.SessionUpdate,
// inverted: the classification stays with the adapter, the side effects move to
// the runtime.
type EventSink interface {
	Text(delta string)           // assistant message text
	Thinking(delta string)       // reasoning delta
	ToolStarted(call ToolCall)   // a tool call began
	ToolUpdated(call ToolCall)   // status/output update for a tool call
	Activity()                   // liveness heartbeat (every event implies one)
	RawStreamMessage(raw []byte) // eval-only provider-native message (usage, raw tool blocks)
}

// Outcome is the typed classification the runtime acts on instead of matching
// adapter error strings. Every adapter MUST map its native failure into exactly
// one class, so the recovery policy and the eval-anomaly contract run on a
// stable vocabulary regardless of runtime.
type Outcome struct {
	Class FailureClass
	Err   error // underlying error, for artifacts/logs only — never string-matched by the runtime
}

type FailureClass int

const (
	Success         FailureClass = iota
	TurnCapReached               // runtime's max-turns net fired -> try the D1 wrap-up resume
	LivenessTimeout              // no activity for the watchdog window -> breaking (episode_hang_timeout)
	PeerDied                     // the backing process/connection died -> D2 fail-fast; drop warm session
	StaleSession                 // resume token no longer valid -> recreate a fresh session
	Other                        // anything else -> fail the turn with the underlying Err
)
```

**Why these two are the whole game.** `EventSink` moves *classification* to the
adapter and *side effects* to the runtime (killing L10/L11). `Outcome` moves
*failure naming* to the adapter as a typed value (killing L8/L9). With those two,
`Ask`'s orchestration — retry, wrap-up recovery, artifact flush, ledger update
(`runtime.go:298-529`) — becomes **runtime-generic**: it switches on
`FailureClass`, appends `ToolCallTrace`s, and never imports `acp`.

### What `Ask` looks like after the port (shape, not a rewrite)

```go
// preflight
if err := run.Preflight(ctx); err != nil { return nil, nil, err }         // was CheckACPHandshake
sess, err := run.Open(ctx, id, meta.Resume)                               // was implicit in RunTurn
res, resume, outcome := sess.Turn(ctx, TurnRequest{Prompt: prompt, Steering: spec, Budget: b}, sink)
switch outcome.Class {                                                     // was IsMaxTurnsError / IsPeerClosedError / ...
case TurnCapReached:  res = wrapUpRecover(ctx, sess, spec, &res)          // runtime.go:444 recoverMaxTurns, de-ACP'd
case StaleSession:    res, resume, outcome = recreateAndRetry(...)        // runtime.go:169 fallback, de-ACP'd
case LivenessTimeout: markLivenessError(&turnErr)                         // writes the canonical marker (Resilience)
case PeerDied:        pool.Drop(id)                                       // daemon_worker.go:158,167, de-ACP'd
}
```

`prepareSession`, the ledger, the report-retry loop, `emit*Artifacts`, and the
route decision are unchanged — they already operate on neutral `TurnResult`.

---

## Adapters & config selection

### Config shape and factory

```jsonc
// ~/.ghx/config.json — one "runner" object selects and configures the runtime.
{
  "runner": {
    "kind": "claude-acp",            // claude-acp | codex-acp | claude-sdk
    // ACP kinds: the subprocess command line (today's `agent` field).
    "command": "npx -y @agentclientprotocol/claude-agent-acp@0.55.0",
    "model": "claude-sonnet-4-5",
    "effort": "medium"
    // codex-acp adds e.g. "command": "codex-acp", "modelProvider": "openai".
    // claude-sdk drops "command" and adds "apiKeyEnv": "ANTHROPIC_API_KEY".
  }
}
```

```go
// NewRunner is the single wiring point. Default kind preserves today's behavior
// exactly, so a machine with the current config keeps running claude-agent-acp.
func NewRunner(cfg RunnerConfig) (runner.Runner, error) {
	switch cfg.Kind {
	case "", "claude-acp":
		return acprunner.New(cfg)   // wraps coder/acp-go-sdk; the code that exists today
	case "codex-acp":
		return codexrunner.New(cfg) // wraps codex-acp; ACP transport, env-var steering
	case "claude-sdk":
		return sdkrunner.New(cfg)   // in-process; anthropic-sdk-go loop (or TS-SDK bridge)
	default:
		return nil, fmt.Errorf("unknown runner kind %q (want claude-acp | codex-acp | claude-sdk)", cfg.Kind)
	}
}
```

Migration of the legacy `agent` string (`config.go:48`): map a bare `agent`
value to `runner.kind="claude-acp", runner.command=<agent>` at load. That is a
one-line alias, not a dual code path (AGENTS.md "no compatibility junk") — the
old field name resolves into the new struct and then only the struct is used.

### Adapter 1 — `claude-acp` (today, refactored behind the port)

Wraps `coder/acp-go-sdk`. This is *the existing code*, re-homed:

- `Open`/`Session.Turn` = `RunTurnWithOptions` (`acp.go:218-315`) and the warm
  `AgentWorker` (`daemon_worker.go`), now returning `(TurnResult, ResumeToken,
  Outcome)`.
- `SteeringSpec` → `BuildSessionMeta`'s `_meta.claudeCode.options`
  (`session_options.go:194-230`) — unchanged, just moved adapter-private.
- `EventSink`: `denyClient.SessionUpdate` (`denyclient.go:191-274`) calls the
  sink instead of writing stdout/live directly; its permission logic
  (`:158-187`) stays as the ACP enforcement of `ToolPolicy`.
- `Outcome` mapping (the string-matchers become adapter-internal, `acp.go:78-106`):
  `IsMaxTurnsError`→`TurnCapReached`, `IsPeerClosedError`→`PeerDied`,
  `ErrLivenessTimeout`→`LivenessTimeout`, `IsLoadSessionResourceNotFound`→
  `StaleSession`.
- `RawStreamMessage`: fed from the `_claude/sdkMessage` extension
  (`rawsdk.go:18-20`).
- `ResumeToken` = ACP session id (`runtime.go:95-103`).

Net: behavior-identical; the ACP knowledge that was spread across the runtime is
now sealed inside one adapter. **Preserves ADR-0015's Go-native ACP choice.**

### Adapter 2 — `codex-acp` (goal a: add a runner via config)

`codex-acp` (agentclientprotocol/codex-acp; also zed-industries/codex-acp) is a
stdio **ACP** server bridging the Codex runtime — so it *reuses the ACP transport*
(`Initialize`/`NewSession`/`Prompt`, the same `coder/acp-go-sdk` client). What
differs, and therefore what the port must let the adapter own:

- **Steering channel is env, not `_meta`.** codex-acp reads `CODEX_CONFIG` (a
  JSON blob merged into the Codex session config), `MODEL_PROVIDER`, and
  `INITIAL_AGENT_MODE`. The adapter maps `SteeringSpec.Model`/`Effort` into
  `CODEX_CONFIG` (model, reasoning effort), and `Isolation`/`ToolPolicy` into
  `INITIAL_AGENT_MODE=read-only` + Codex sandbox/approval. **This is exactly why
  L1/L2 must be adapter-private:** the same `SteeringSpec` produces a totally
  different encoding.
- **Persona placement differs.** codex-acp has no system-prompt-over-`_meta`
  channel equivalent to `systemPrompt`; the adapter must place
  `SteeringSpec.SystemPrompt` into the prompt text (the pre-ADR-0020.1 posture)
  or the closest Codex "instructions" config, and record that choice loudly (it
  shifts cache behavior and possibly SPT — an honest tradeoff, not a silent one).
- **Permission model differs.** Codex emits its own approval/permission and MCP
  events; the adapter enforces the same deny-writes `ToolPolicy` via Codex's
  sandbox + approval callback, and owns auto-approving the report-sink tool
  (L4/L5 are adapter-private).
- **Report sink** rides Codex's own MCP-server registration (it supports MCP tool
  calls), so `ReportSink` intent maps to a codex `mcpServers` entry.
- `Outcome`: Codex's turn-cap / disconnect / stale-session errors map to the same
  five classes. The runtime's recovery is unchanged.

Because codex-acp is still an ACP subprocess, adapter 2 is the *smallest possible
second adapter* — it shares the ACP transport with adapter 1 and only re-implements
steering-encoding, persona-placement, permission-enforcement, and outcome-mapping.
That is precisely the surface the port is designed to vary, which makes it the
ideal proof that the boundary holds.

### Adapter 3 — `claude-sdk` (goal b: entirely replace ACP with the Claude Agents SDK)

Research finding that shapes this adapter: **there is no Go binding for the
Claude Agents SDK.** The SDK ships as `@anthropic-ai/claude-agent-sdk` (TS) and
`claude-agent-sdk` (Python), and it *itself spawns the `claude` CLI as a
subprocess and talks JSON over stdin/stdout*. So "replace ACP with the Claude
Agents SDK" in a Go codebase has two honest realizations, both behind the *same*
port — and both prove **the port must not assume ACP or a stdio wire**:

- **3a — pure-Go in-process loop over `anthropic-sdk-go`** (the official Go SDK:
  `client.Messages.NewStreaming`, tool_use/tool_result blocks, thinking blocks,
  native `usage`). The adapter runs the agent loop *in-process*: assemble
  context, offer exactly a ghx-shell tool + a `submit_report` tool, stream deltas
  to `EventSink`, loop until `submit_report` or the turn cap. This is
  **NORTH_STAR P4 "below ACP" made literal** — an owned harness that controls
  context assembly, the tool registry, and the model. It removes leaks instead of
  translating them:
  - No permission wire: deny-writes = *never registering a write tool* (stronger
    than the ACP denyClient's after-the-fact reject, L4/L5 vanish).
  - Native thinking: no `thinking.display` archaeology (ADR-0018 opacity gone).
  - Real tokens in **production**: the Messages `usage` fills `TurnResult.Usage`
    directly — closing the char-proxy gap (`emit.go:352-353`) and TRUST H5,
    which today is eval-only via a claude-specific extension (`rawsdk.go`).
  - No subprocess, no ACP: `Open` returns an in-process `Session`; `ResumeToken`
    is ghx's own persisted message history (resume is an optimization anyway,
    L12). **`AgentCmd`/`Env`/`initialize` have no meaning here — proving they
    could never have lived on the port.**
- **3b — subprocess bridge to the TS SDK** (`@anthropic-ai/claude-agent-sdk` in
  streaming-input mode, its own JSON control protocol over stdio — *not* ACP).
  Keeps SDK-native features (subagents, compaction, hooks, in-process MCP) at the
  cost of a Node subprocess and the TS control protocol. Same port; the adapter
  translates `SteeringSpec` into SDK `Options` and decodes SDK messages into
  `EventSink`.

Recommendation: build **3a** as the P4 endgame (it is the "owned harness" the
north star names and the most Go-idiomatic), and keep **3b** on the shelf for
when a specific SDK-only feature (e.g. built-in subagents) earns its subprocess.
Either way, the port carries no assumption that a runner is a subprocess or
speaks ACP — that assumption stays sealed inside the two ACP adapters.

---

## Resilience across the boundary

The whole resilience taxonomy (ADR-0027) and its *eval-anomaly measurement
contract* must cross the port without leaking ACP specifics — and without
disturbing the **frozen measurement stack** (AGENTS.md Visibility & Truthfulness:
scores recomputable from committed artifacts; the stack is frozen mid-run).

**The invariant that makes this safe:** the anomaly detector reads two things —
neutral `TurnResult` fields and the persisted **turn-error string**
(`evals/anomalies.go:115,124`). Adapters already populate `TurnResult`
identically (it is `acp`-free). The only shared string is the marker set. So the
reconciliation is: **the runtime, not the adapter, writes the canonical marker,
keyed by `FailureClass`.**

| Signal | Today (leaks) | Under the port | Marker / anomaly preserved |
|---|---|---|---|
| **Liveness watchdog** | watchdog in ACP turn code, fed by `denyClient.SessionUpdate` (`turn.go:19`, `denyclient.go:192`) | runtime owns the watchdog, wraps `Session.Turn`, fed by `EventSink.Activity()` (L10). On fire, runtime returns `LivenessTimeout` and writes `LivenessTimeoutMarker` into the turn error | `episode_hang_timeout` (breaking) — `anomalies.go:115` reads the *runtime-written* marker, unchanged byte-for-byte |
| **Max-turns cap** | `IsMaxTurnsError` string match on adapter text (`runtime.go:445`) | adapter returns `TurnCapReached`; runtime issues the same D1 wrap-up resume (`turnCapWrapUpPrompt`, `runtime.go:53`) via `Session.Turn`; sets `WrapUpRecovered` | `turn_cap_wrapup` (soft) — `anomalies.go:124` reads `WrapUpRecovered`, unchanged |
| **Peer/connection death** | `IsPeerClosedError` drops warm worker (`daemon_worker.go:158,167`) | adapter returns `PeerDied`; pool drops the `Session` | no anomaly row today; behavior identical, now runtime-generic |
| **Stale resume** | `IsLoadSessionResourceNotFound` (JSON-RPC `-32002`) recreate (`runtime.go:171`) | adapter returns `StaleSession`; runtime recreates fresh (`SessionRecreated`) | `SessionRecreated` flag on `TurnResult`, unchanged |
| **Token accounting** | prod char-proxy (`emit.go:352-353`); real tokens eval-only via `_claude/sdkMessage` (`rawsdk.go`) | neutral `EventSink.RawStreamMessage` + `TurnResult.Usage`; ACP adapter fills from the extension, SDK adapter fills from Messages `usage` | measurement stack reads the same fields; a runner that reports real tokens simply *improves* fidelity without changing the schema |

Key property: because the marker strings become **runtime-owned constants keyed
by class** (L9), a `codex-acp` or `claude-sdk` turn that hits its cap or hangs
produces the **exact same anomaly rows** as claude-acp does today. The frozen
measurement stack never learns which runtime ran — which is the definition of a
clean boundary, and exactly what "the measurement stack must survive replacing
ACP" requires.

---

## Recommended sequence (phased, non-rewrite — ship the port with ONE adapter first, prove it with a second)

Incremental, behavior-preserving, ADR-gated. No speculative wire protocol; every
adapter adopts an existing SDK.

**Phase 0 — ADR + neutrality test (no behavior change).** Write the governing
ADR (this is ADR-0035, currently unwritten — the slot the task referenced does
not exist yet; `docs/adr/` ends at 0034). Add a grep-based guard test asserting
no `acp` import outside the future adapter files, so the boundary is enforced
before it is built. Deliverable: ADR + a failing-if-violated test. *Verify:*
`go test ./...` green.

**Phase 1 — define the port in the consumer package; make today's ACP path
implement it (behavior-identical).** Introduce `runner.Runner`/`Session`/
`SteeringSpec`/`TurnRequest`/`Outcome`/`EventSink` (initially in package
`sidecar`, the consumer — the idiomatic first move). Promote `TurnRunner`
(`runtime.go:20`) into `Session.Turn`; convert the string-matchers (`acp.go:78-106`)
into an adapter-internal `Outcome` mapping; invert `denyClient.SessionUpdate`
onto `EventSink`; carry `SteeringSpec` and translate it *inside* the ACP adapter
via the untouched `BuildSessionMeta`. Default `kind=claude-acp`. *Verify:*
`go test ./...` plus one live `ghx sidecar ask` smoke — same session artifacts,
same anomaly rows (the eval cadence's "one live smoke of the touched path").

**Phase 2 — lift the port to a leaf package; enforce neutrality at compile time.**
Move the port + neutral types to `internal/sidecar/runner`; move the ACP code to
`internal/sidecar/runner/acprunner`. Now `runner` cannot import `acp` and the
compiler guarantees the boundary. *Verify:* build + tests; a diff review that the
move is pure relocation (mirror the a49ce76 "byte-identical decomposition" style).

**Phase 3 — prove it with the second adapter: `codex-acp` (goal a).** Smallest
real second runtime (shares the ACP transport; varies only steering-encoding,
persona-placement, permission-enforcement, outcome-mapping). Add `RunnerConfig`,
`NewRunner`, and the `codexrunner` package. Fix the current trap where a `codex`
`AgentCmd` silently drops steering (L1/L2). *Verify:* a live `codex-acp` ask
produces a valid report and the same artifact shape; a sidecar-only eval spot
check (2-3 cells incl. a `ghx-sidecar` cell) confirms no anomaly-contract drift.

**Phase 4 — the replaceable runtime: `claude-sdk` in-process (goal b, P4).** Build
`sdkrunner` as the pure-Go loop over `anthropic-sdk-go` (3a). This is the phase
that *removes* leaks (permission wire, thinking opacity, char-proxy tokens) and
delivers real production tokens (TRUST H5). Gate behind an explicit ADR update
because it changes the token-accounting source — a measurement-stack change that
must be pre-registered and re-grounded (AGENTS.md; NORTH_STAR eval cadence).
*Verify:* a citable sidecar-only comparison run, not a drive-by.

**On the in-flight `TurnRunner` "harness seam": stepping stone, not dead end.** It
is phase-0 already shipped in spirit — it decouples *who runs a turn* from `Ask`.
It is incomplete on exactly three axes, and Phase 1 closes them: neutralize the
**request** (drop `AgentCmd`/`Env`/`SessionMeta`-as-`claudeCode` → `SteeringSpec`),
neutralize the **resume handle** (bare ACP session-id `string` → opaque
`ResumeToken`), and lift **failure classification** into the return (`Outcome`)
so the runtime stops string-matching. Do not throw it away; raise its altitude.

**Honest tradeoffs.**
- **Steering is a lowest-common-denominator contract.** No two runtimes accept
  the same steering channel; `SteeringSpec` is the intersection, and adapters
  that cannot honor a field (codex has no `systemPrompt`-over-`_meta`) must
  degrade *loudly*. Persona placement moving to prompt text on codex shifts cache
  behavior and possibly SPT — measurable, not hidden.
- **Resume semantics genuinely differ** (ACP id vs SDK id vs ghx history). The
  opaque `ResumeToken` + "resume is only an optimization" (`runtime.go:167`)
  absorbs this; a non-resuming runner is correct, just colder.
- **Report-sink approval is per-runtime.** The port carries intent; each adapter
  owns the mechanics (ACP `allowedTools` bypass vs codex approval vs SDK "just
  don't offer a write tool").
- **The SDK adapter is the largest slice** and changes a measurement source; it
  is deliberately last and ADR-gated. Phases 1-3 deliver goal (a) and the clean
  boundary with zero measurement change; phase 4 delivers goal (b).

---

## Method / auditability

- **Every `file:line` was read in the working tree at audit time**, on branch
  `worktree-agent-a95986a9a211721e0`. `go build ./...` is green; no code was
  modified by this audit (only this document was written).
- **ACP import surface** confirmed by `grep -rln 'coder/acp-go-sdk'
  internal/sidecar/*.go` (non-test): the `acp` import is confined to
  `acp.go`, `turn.go`, `denyclient.go`, `daemon_worker.go`, `tooltrace.go`,
  `reportsink_exe.go`, `session_options.go`, `reportsink.go`, `config.go` — the
  exact set the port seals behind adapters. `turnresult.go` imports only `time`,
  proving the result DTO is already neutral (the migration's keystone).
- **The existing seam** is `TurnRunner` (`runtime.go:18-20`), injected by the
  daemon pool (`daemon_worker.go:41`) vs daemonless (`runtime.go:15,280`) — the
  proof it is a stepping stone, not a greenfield.
- **External realities are text-only research** (AGENTS.md Tool Economy — no
  browser/screenshots), each load-bearing claim tied to a source:
  - *No Go Claude Agent SDK; the SDK spawns the `claude` CLI over stdio* —
    code.claude.com/docs (Agent SDK hosting) and the SDK repos
    (`anthropics/claude-agent-sdk-python`, `-typescript`).
  - *`anthropic-sdk-go` is the official Go Messages API with tool_use/thinking/
    streaming* — `github.com/anthropics/anthropic-sdk-go`,
    platform.claude.com/docs/en/api/sdks/go.
  - *`codex-acp` is a stdio ACP server configured via `CODEX_CONFIG` /
    `MODEL_PROVIDER` / `INITIAL_AGENT_MODE` env, not a `claudeCode` `_meta` key* —
    `agentclientprotocol/codex-acp`, `zed-industries/codex-acp`.
  - *`claude-agent-acp` documented limitations* (thinking `display` opacity, PR
    unmerged; ANTHROPIC_API_KEY required, no Claude Max OAuth; extended thinking
    off by default with no setting) — `agentclientprotocol/claude-agent-acp`
    issues #297/#56375, openclaw issue #53456. These corroborate the founder's
    "hitting limitations" and NORTH_STAR P4 exit-trigger (1) "adapter opacity."
- **Prior art reconciled:** the leak sites match AUD4's V2 (ACP adapter leakage)
  and the marker inventory matches failure-class B4; this document adds the port
  those findings imply and the phased path to it. The ADR slot referenced as
  "0035" does not yet exist — Phase 0 writes it.

Sources: [Agent SDK hosting](https://code.claude.com/docs/en/agent-sdk/hosting) ·
[claude-agent-sdk-python](https://github.com/anthropics/claude-agent-sdk-python) ·
[anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) ·
[Go SDK docs](https://platform.claude.com/docs/en/api/sdks/go) ·
[codex-acp](https://github.com/agentclientprotocol/codex-acp) ·
[zed codex-acp](https://github.com/zed-industries/codex-acp) ·
[claude-agent-acp](https://github.com/agentclientprotocol/claude-agent-acp) ·
[thinking-blocks issue](https://github.com/zed-industries/zed/issues/56375) ·
[Claude Max OAuth issue](https://github.com/openclaw/openclaw/issues/53456).
