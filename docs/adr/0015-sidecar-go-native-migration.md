---
title: "ghx Sidecar Go-Native Migration"
date: 2026-05-13
status: Decided
supersedes: ADR-0014.1, ADR-0014.2, ADR-0014.3, ADR-0014.4
---

# 0015. `ghx` Sidecar Go-Native Migration

## Context

ADR-0014.3 built `packages/ghx-sidecar` as a TypeScript ESM package using the
`acpx/runtime` SDK to drive ACP agents. The implementation was correct and the
sidecar experiments validated the core thesis: a cheap specialist agent can preprocess
repo reconnaissance and compress findings into a compact evidence report for the
expensive main coding agent.

However, after validating the product thesis and building the benchmark suite
(`packages/ghx-bench`), a structural problem with the TypeScript runtime became clear:
**the sidecar's operational envelope is owned by the wrong layer.**

The `acpx/runtime` SDK controls session management, agent lifecycle, permission mode
enforcement, and event routing. These are not incidental implementation details.
They are the core semantics of the sidecar contract: which operations are permitted,
what counts as a session turn, when a session is resumable, how permission requests are
answered. Delegating them to an external alpha SDK creates a boundary that cannot be
maintained without depending on `acpx` internal decisions.

Additionally, `ghx` is a Go binary. Having the sidecar live in a separate Node.js
package means:

- A Node.js runtime is required on the user's PATH alongside `ghx`.
- The `ghx-sidecar` binary and the `ghx` binary cannot be deployed as a single artifact.
- Benchmark scripts and session tooling must be maintained in two languages.
- The `acpx` npm package is at v0.6.1 and carries its own upgrade surface.

The natural home for the sidecar runtime is inside the `ghx` binary itself, implemented
against `coder/acp-go-sdk` — the upstream Go ACP client library from the protocol's
primary implementor.

## Decision

Migrate the `ghx` sidecar from TypeScript/acpx to Go inside the Go binary.

Delete `packages/ghx-sidecar` entirely. The Go implementation is the canonical sidecar
runtime going forward. `packages/ghx-bench` is kept as-is (no equivalent eval engine
exists in Go yet). *(Update 2026-07-04: the Go eval suite in `internal/sidecar/evals`
— ADR-0016.1/0016.2 — replaced it; `packages/ghx-bench` is deleted.)*

## What Changed

### Deleted

`packages/ghx-sidecar/` — the entire TypeScript sidecar package including CLI, library
exports, `acpx.ts`, `schema.ts`, `core/config.ts`, `core/prompt.ts`, `core/preflight.ts`,
`core/session.ts`, benchmarks, and build config.

### Created

`internal/sidecar/` — the Go sidecar runtime package:

```
internal/sidecar/
  report.go    – Report, Claim, RelevantFile, Evidence types + ExtractReport()
  prompt.go    – Request type + BuildPrompt() (full persona on first turn, compact
                 prior-context on follow-up)
  config.go    – Config, LoadConfig(), SaveConfig(), DetectAgents()
  session.go   – SessionMeta (with ACPSessionID field), Init/Read/Save/Record/List
  preflight.go – RunPreflight(), FormatPreflight() (GH token, network, ghx binary)
  acp.go       – denyClient (implements acp.Client), TurnResult, RunTurn()
  runtime.go   – Ask() orchestrator (the single entry point for a sidecar turn)
```

`internal/cli/sidecar.go` — Cobra command tree:

```
ghx sidecar ask --session --repo <question>
ghx sidecar doctor
ghx sidecar sessions list
ghx sidecar sessions show <name>
ghx sidecar config show
ghx sidecar config init
```

`internal/cli/ghx.go` — `sidecarCmd` added to `RootCmd.AddCommand(...)`.

`go.mod` — `github.com/coder/acp-go-sdk v0.13.0` promoted from indirect to direct.

## Architecture

```
ghx sidecar ask
  → runtime.Ask()
    → session.InitSession() / session.ReadMeta()   (disk-backed session registry)
    → prompt.BuildPrompt()                         (prompt factory)
    → acp.RunTurn()                                (ACP execution)
      → exec.Command(agentCmd)                     (spawn agent subprocess)
      → acp.NewClientSideConnection(denyClient)    (Go ACP client)
      → conn.Initialize()
      → conn.NewSession() or conn.LoadSession()    (first turn vs. follow-up)
      → conn.Prompt()                              (blocking prompt call)
      ← denyClient.SessionUpdate()                 (text streaming + tool call capture)
    → report.ExtractReport()                       (parse <ghx-report> block)
    → session.SaveReport() + session.RecordTurn()  (evidence store)
```

### Key differences from the TypeScript implementation

| Concern | TypeScript (acpx) | Go (coder/acp-go-sdk) |
|---------|-------------------|-----------------------|
| ACP client | `createAcpRuntime` from `acpx/runtime` | `acp.NewClientSideConnection` + `acp.Client` interface |
| Agent subprocess | Managed by `acpx` runtime internals | `exec.CommandContext` spawned directly in `RunTurn` |
| Permission enforcement | `permissionMode: 'deny-all'` SDK option | `denyClient.RequestPermission` always selects `reject_once`/`reject_always` |
| Session resumption | `ensureSession({ mode: 'persistent' })` inside acpx | `ACPSessionID` field on `SessionMeta`; `conn.LoadSession` on follow-up turns |
| Session store | Managed by `createFileSessionStore` in acpx | `internal/sidecar/session.go` owns all disk artifacts |
| Text streaming | `AcpRuntimeEvent` iterable loop | `denyClient.SessionUpdate` callback with `AgentMessageChunk` |
| Tool call capture | `event.type === 'tool_call'` event | `denyClient.SessionUpdate` with `u.ToolCall` branch |
| Report extraction | `extractReport(fullText)` regex parser | `report.ExtractReport(text)` using `ghxReportRE` |
| No-write enforcement | `permissionMode: 'deny-all'` + `nonInteractivePermissions: 'deny'` | `denyClient.WriteTextFile` returns error; `ReadTextFile` returns error |
| Deployment | Separate Node.js binary + `ghx` Go binary | Single `ghx` binary |

### ACPSessionID and session resumption

The TypeScript implementation relied on `acpx/runtime`'s file session store to handle ACP
session continuity invisibly. The Go implementation owns this explicitly: `SessionMeta`
carries an `ACPSessionID string` field. On the first turn, `RunTurn` calls `conn.NewSession`
and the returned `SessionId` is saved back into `meta.ACPSessionID`. On subsequent turns,
`RunTurn` calls `conn.LoadSession(SessionId)` to resume the agent's prior context.

This makes session continuity visible and auditable in the on-disk `meta.json`:

```json
{
  "name": "hono-middleware",
  "repo": "honojs/hono",
  "acpSessionId": "sess_01abc...",
  "turnCount": 3
}
```

If the agent process is killed between turns, the next call simply calls `LoadSession` with
the stored ID. If that fails (agent does not support session resumption), `RunTurn` returns
an error the caller can handle.

### denyClient and the permission contract

`denyClient` implements the `acp.Client` interface. It has two responsibilities:

1. **Permission denial**: `RequestPermission` always selects the first `reject_once` or
   `reject_always` option from the agent's proposed options. If neither is available, it
   cancels the request. This makes the read-only enforcement unconditional: no downstream
   prompt instruction can override it.

2. **Output collection**: `SessionUpdate` writes text chunks to stdout in real time and
   appends them to `TurnResult.FullText`. Tool call events are written to stderr and
   appended to `TurnResult.ToolCalls`.

Terminal methods (`CreateTerminal`, `TerminalOutput`, `ReleaseTerminal`,
`WaitForTerminalExit`, `KillTerminal`) all return errors. A sidecar that needs a terminal
is a sidecar doing something wrong.

`WriteTextFile` and `ReadTextFile` both return errors. All file reads go through `ghx`;
direct FS access is not part of the sidecar's tool contract.

## Why coder/acp-go-sdk

`coder/acp-go-sdk` v0.13.0 is the Go ACP client library published by Coder, which is also
the primary publisher of the ACP specification and the reference ACP tooling. It is the
natural upstream dependency for a Go ACP client.

Properties that made it the correct choice:

- **Zero third-party dependencies**: the SDK's own `go.mod` has no third-party deps. It
  will not pull unexpected transitive dependencies into `ghx`.
- **Generated from the ACP spec**: `types_gen.go`, `agent_gen.go`, `client_gen.go` are
  machine-generated from the protocol definition. Type shapes will track the spec.
- **`acp.Client` interface**: a clean interface with well-scoped methods that maps directly
  to the sidecar's intended behavior. Implementing it requires no SDK-internal knowledge.
- **`ClientSideConnection` manages the JSON-RPC loop**: goroutines, framing, and
  notification dispatch are handled inside the SDK.

The alternative — building directly against the ACP JSON-RPC 2.0 wire protocol — would
require maintaining protocol framing, message dispatch, and notification routing ourselves.
The SDK eliminates all of that.

## Why Delete the TypeScript Package

The TypeScript sidecar was built to answer a product question: does the sidecar thesis
hold? The bench experiments answered yes — especially the architecture-aware LLM judge in
ADR-0014.4, which showed the sidecar winning on `contextBurden` and `reportUsefulness`
when scored fairly.

Once the thesis is validated and the Go implementation is in place, the TypeScript package
becomes maintenance overhead with no unique value:

- It runs on a Node.js runtime that `ghx` users may not have.
- It depends on `acpx` v0.6.1, an alpha package with its own upgrade surface.
- It has different session disk layout semantics than the Go implementation.
- Keeping both means two canons for the report schema, the session format, and the CLI.

The clean decision is to have one canonical sidecar implementation in the `ghx` binary.

## What Is Preserved

The following contracts are unchanged from the TypeScript implementation:

- **Report schema**: `Report` has the same fields as `SidecarReport` in `schema.ts`.
  `<ghx-report>` XML extraction uses the same regex pattern.
- **Prompt contract**: `BuildPrompt` produces the same persona, operating loop, budget
  (8 commands first turn, 5 follow-up), `submit_report` format, and failure mode block.
- **Session disk layout**: `~/.ghx-sidecar/sessions/<name>/` with `initialized` marker,
  `meta.json`, and `report-<ts>.json` files. Follow-up turns detect prior sessions by
  checking the `initialized` marker.
- **Preflight checks**: GH token presence, HEAD api.github.com, ghx binary on PATH.
- **Config file**: `~/.ghx-sidecar/config.json` with `agent` and `sessionsDir` fields.

## Architectural Insights

**Protocol role clarity is the prerequisite.** Both the TypeScript and Go implementations
make the same choice: `ghx-sidecar` is an ACP client, not an ACP server. The Go migration
did not change that decision; it only moved the client implementation to the right layer
(native binary, owned interface) instead of delegating it to an external runtime.

**Owning the permission contract matters.** `acpx/runtime` enforced `deny-all` through a
config option whose semantics were controlled by the `acpx` SDK. `denyClient.RequestPermission`
in Go enforces the same contract but in code that is part of `ghx` and tested as part of
`ghx`. The enforcement logic is no longer a dependency on an external package's interpretation
of a permission flag.

**Session ID visibility is an improvement.** The TypeScript implementation hid ACP session
continuity inside `acpx/runtime`'s file session store. The Go implementation surfaces the
ACP session ID in `meta.json`. This is a better contract: engineers can inspect what session
ID the agent assigned, reason about session continuity, and debug resumption failures
without opening opaque acpx store files.

**A single binary is the right deployment model.** `ghx` is a Go binary distributed as a
single executable. Having the sidecar inside `ghx` means no separate install step, no Node
runtime requirement, and no version drift between the CLI and the sidecar harness.

## Open Questions

- Should `ghx sidecar ask` support `--no-wait` (fire and forget, return immediately after
  spawning the turn, similar to the TypeScript `noWait` option)?
- Should `sessions clear <name>` be added to remove session artifacts from disk?
- Should `ghx sidecar doctor` also probe that the configured agent binary is reachable
  (equivalent to the TypeScript `acpxDoctor` check)?
- Should the session disk layout be extended with an `inspected_paths`, `mapped_globs`,
  and `rejected_paths` evidence ledger per ADR-0014.1 session memory proposal?
- What is the upgrade path when `coder/acp-go-sdk` releases a version that changes the
  `acp.Client` interface?

## What Is Next

The Go sidecar enables the `ghx-bench` A/B experiments to run against the native binary
instead of the TypeScript harness. The immediate next step is to run the
`express-routing` benchmark task defined in `packages/ghx-bench` against three agent
configurations: sidecar agent (Go), ghx-skill agent (direct baseline), and plain agent
(no ghx). Results from this run will determine whether the bench evaluation can be
promoted to a test suite.

## References

- [ADR-0014.1: ghx Sidecar Agent Vision](./0014.1-sidecar-agent-vision.md)
- [ADR-0014.2: ghx Sidecar Proof-of-Concept Gate](./0014.2-sidecar-proof-of-concept-gate.md)
- [ADR-0014.3: ghx-sidecar MVP Implementation Architecture](./0014.3-sidecar-mvp-implementation.md)
- [ADR-0014.4: ghx-sidecar Benchmark and LLM Judge Architecture](./0014.4-sidecar-bench-llm-judge.md)
- [internal/sidecar/](../../internal/sidecar/)
- [internal/cli/sidecar.go](../../internal/cli/sidecar.go)
- [coder/acp-go-sdk v0.13.0](https://github.com/coder/acp-go-sdk)
- [Agent Client Protocol](https://agentclientprotocol.com/get-started/introduction)
