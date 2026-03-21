# ADR-0008: Technical Decisions — Dependencies, Module Structure, and Interfaces

**Date**: 2026-03-21
**Status**: Accepted
**Parent**: ADR-0007

## Purpose

ADR-0007 defined the architecture (core + CLI + MCP + codemode) and the build order. This ADR resolves the technical decisions needed before implementation begins, grounded in research across the codemode ecosystem and oriented toward where the field is heading — not where it was.

---

## Where Codemode Is Heading

The shift from traditional MCP (one tool call per LLM round-trip) to Code Execution MCP (CE-MCP, agent writes a program) is well-documented. A Ben Gurion University study ("From Tool Orchestration to Code Execution", Feb 2026) benchmarked it: CE-MCP achieves ~98% token reduction and 60% faster execution by collapsing N tool calls into one script.

But that's the current state. The next phase is:

1. **Parallel tool execution within scripts.** Fan-out patterns where one script fires multiple tool calls concurrently. Already emerging in UTCP and Lattice frameworks.
2. **Streaming results mid-execution.** The executor emits partial results while still running. UTCP's `CallToolStream` already supports this.
3. **Multi-agent codemode.** Agent A writes code that calls Agent B as a tool. The executor becomes the coordination layer between agents.
4. **Long-running codemode sessions.** Persistent runtimes that maintain state across multiple script executions.

These directions all require **concurrency** at the executor level. Any runtime that serializes execution (mutex-locked, single-threaded) is a dead end.

---

## 1. Script Runtime: dop251/goja (JavaScript)

### Landscape — 5 runtime options evaluated

| Runtime | Script Language | Pure Go | Binary Size | Concurrency | Production Users |
|---------|----------------|---------|-------------|-------------|-----------------|
| **dop251/goja** | JavaScript (ES2015) | ✅ | +2-3MB | ✅ Fresh VM per call | gridctl, agent-go, k6, PocketBase |
| **goja + esbuild** | JavaScript (modern) | ✅ | +12-13MB | ✅ Same | gridctl |
| **monty-go/wazero** | Python (subset) | ✅ | +2.9MB | ❌ Mutex-locked | gollem |
| **google/starlark-go** | Starlark (Python-like) | ✅ | +1-2MB | ✅ | bifrost |
| **yaegi** | Go | ✅ | +3-4MB | ⚠️ Hacky stdout capture | codemode-sqlite-mcp |

### Code reviewed (not just READMEs)

- **gridctl** `pkg/mcp/codemode_sandbox.go` (~150 lines): Fresh goja VM per execution. `mcp.callTool(server, tool, args)` injected. ACL enforcement. Console capture. esbuild transpiles modern JS → ES2015. JSON result parsing back to native JS values.
- **agent-go** `pkg/ptc/runtime/goja/runtime.go` (~500 lines): Same goja pattern, more sophisticated. IIFE wrapping for top-level `return`. Fuzzy tool name resolution. `normalizeForJS` handles Go struct → JS object conversion. Max call limit. Promise handling.
- **monty-go** `monty.go` + `wasm.go`: WASM-based Python interpreter via wazero. Pause/resume model. 2.9MB embedded WASM. **Single-threaded — gollem's CodeMode wrapper has `cm.mu.Lock()` around every execution.**
- **yaegi** (codemode-sqlite-mcp) `pkg/executor/executor.go` (~350 lines): Interpreter pool. Captures stdout via `os.Pipe` (redirects global `os.Stdout`). `time.Sleep(10ms)` to flush output. Fragile.
- **bifrost** `core/mcp/codemode/starlark/starlark.go`: Google's Starlark. Very sandboxed but too limited — no try/except, no imports, no string formatting beyond basics.

### Why goja wins

**Concurrency is the deciding factor.** goja creates a fresh VM per execution — fully concurrent, zero shared state. This enables:

| Future direction | goja | monty-go (WASM) | yaegi | starlark |
|-----------------|------|-----------------|-------|----------|
| Parallel tool calls in one script | ✅ Event loop + goroutines | ❌ Single-threaded | ❌ | ❌ |
| Concurrent codemode requests | ✅ Fresh VM per call | ❌ Mutex-serialized | ⚠️ Pool | ✅ |
| Streaming mid-execution | ✅ Channel-based | ❌ Blocked until complete | ❌ | ❌ |
| Multi-agent codemode | ✅ Independent VMs | ❌ Bottleneck | ⚠️ | ⚠️ |

**LLM code quality is a close second.** LLMs generate excellent JS and excellent Python. But JS has the edge for tool orchestration: `Promise.all` for parallelism, native JSON handling, and the largest training corpus of any language.

**monty-go's architecture is elegant but wrong for this use case.** The WASM pause/resume model is clean for sequential scripts. But the single-threaded constraint is fundamental to WASM's execution model — it's not a bug to fix, it's a design choice that doesn't fit concurrent tool orchestration.

### Decision: dop251/goja + esbuild transpilation

- goja for the runtime (fresh VM per call, pure Go, no CGO)
- esbuild for transpilation (modern JS → ES2015, pure Go API)
- Combined binary size: +12-13MB. Acceptable for a developer tool.
- LLMs generate modern JS by default — fighting this with prompt instructions is fragile. esbuild handles it reliably. Proven in gridctl production.

---

## 2. Go MCP Library: mark3labs/mcp-go

### Landscape

| Criteria | mark3labs/mcp-go | metoro-io/mcp-golang |
|----------|-----------------|---------------------|
| Stars | ~4,800+ | 1,208 |
| MCP spec | 2025-11-25 (latest) | Older |
| Transports | stdio, SSE, streamable HTTP | stdio, SSE |
| Tool definition | Builder pattern | Struct tags |
| Session management | Full (per-session tools, filtering) | Basic |
| Async tasks | Yes | No |
| Test helpers | `mcptest` package | None |

### Decision: mark3labs/mcp-go

- Most widely adopted Go MCP library
- Supports streamable HTTP (needed for remote/multi-agent scenarios)
- Builder pattern for tool definitions is cleaner for programmatic registration
- `mcptest` package simplifies testing
- gridctl uses it — less friction when drawing from their patterns

---

## 3. Module Structure: Monorepo

```
github.com/gkoreli/ghx/v2
├── pkg/ghx/        # Core library (GitHub API operations)
├── pkg/codemode/   # Codemode SDK (executor, transpiler, typegen, registry)
├── cmd/            # CLI + MCP server
├── go.mod          # Single module
```

- Faster iteration — no cross-repo dependency management during initial development
- ~400 lines of codemode code doesn't justify a separate repo yet
- If a second consumer appears, extract to `github.com/gkoreli/go-codemode` then
- Go modules support `pkg/codemode/` as an importable package within the same module

---

## 4. Tool Registration: Hybrid Interface

```go
registry.Register(codemode.Tool{
    Name:        "explore",
    Description: "Explore a GitHub repo — returns branch, file tree, and README",
    Func:        ghx.Explore,
    // Schema auto-generated from Func signature via reflect
    // Override specific fields if needed:
    Args: map[string]codemode.Arg{
        "repo": {Required: true, Description: "owner/repo format"},
    },
})
```

- Auto-generates schema from Go function signatures (minimal boilerplate)
- Allows description overrides for LLM-friendly documentation (full control when needed)
- `Func` field accepts `any` — uses reflect to extract parameter types for TS stub generation
- Aligns with Cloudflare's `ToolProvider` pattern

---

## 5. Distribution: Single Binary

```
ghx explore ...        # CLI mode
ghx serve              # MCP server mode (stdio by default)
ghx serve --http :8080 # MCP server mode (streamable HTTP)
```

- One binary to install, one binary to update
- Codemode exposed through MCP server as `search` + `execute` meta-tools
- Binary size ~15-17MB (up from 4.5MB). All pure Go, no CGO, cross-compiles cleanly.

---

## 6. Type Stubs: Runtime Generation

- Generated when MCP server starts or when codemode `search` is called
- No build step — register tools, stubs appear automatically
- Stubs change when tools change — always in sync
- gridctl and Cloudflare both do this at runtime
- Fast (~1ms for 5 tools)

---

## 7. MCP Transport: stdio + Streamable HTTP

- stdio is the standard for local MCP servers (Claude Desktop, Cursor, Claude Code)
- Streamable HTTP enables remote usage, web clients, and multi-agent scenarios
- mark3labs/mcp-go supports both out of the box
- SSE is deprecated in favor of streamable HTTP in the latest MCP spec

---

## 8. Testing Strategy

| Layer | Method |
|-------|--------|
| Core (`pkg/ghx/`) | Unit tests with mocked GitHub API responses |
| Codemode executor | Snapshot tests: JS input → expected output. Timeout, ACL, console capture |
| Codemode type gen | Golden file tests: Go function signatures → expected TS stubs |
| MCP server | Integration tests using `mcptest` from mark3labs/mcp-go |
| CLI | Output format parity tests with bash v0.x |

Key codemode executor test cases: happy path, timeout interruption, ACL rejection, console capture, transpilation of modern JS, syntax/runtime error messages, markdown fence stripping.

---

## Summary

| Decision | Choice | Key rationale |
|----------|--------|---------------|
| Script runtime | dop251/goja + esbuild | Concurrent (fresh VM per call), proven in 2 production codebases, LLMs generate great JS |
| MCP library | mark3labs/mcp-go | Most adopted, streamable HTTP, builder pattern, mcptest |
| Module structure | Monorepo | Fast iteration, extract later if second consumer appears |
| Tool registration | Hybrid (auto-gen + override) | Minimal boilerplate, full control when needed |
| Distribution | Single binary | One install, codemode via MCP server |
| Type stubs | Runtime generation | No build step, always in sync |
| MCP transport | stdio + streamable HTTP | Local + remote + multi-agent |
| Security | Sandbox (ACL, timeout, size limit, no FS/net) | Prevent accidental damage, not adversarial LLM control |
| Error recovery | No auto-retry, return error to caller | Avoids regeneration loop attack vector |
| Observability | Built into return type (console, tool call records) | No external tracing dependency |
| Evolution | 4 phases: sequential → parallel → streaming → sessions | Each phase additive, API stable across phases |

## Dependencies

| Package | Purpose | Size | CGO |
|---------|---------|------|-----|
| `github.com/mark3labs/mcp-go` | MCP server | Minimal | No |
| `github.com/dop251/goja` | JS execution sandbox | ~2-3MB | No |
| `github.com/evanw/esbuild` | JS transpilation | ~10MB | No |
| `github.com/cli/go-gh/v2` | GitHub API (existing) | — | No |
| `github.com/spf13/cobra` | CLI framework (existing) | — | No |

All pure Go. No CGO. Cross-compiles to all platforms.

---

## 9. Security Posture

The Ben Gurion CE-MCP study identified 16 attack classes across 5 execution phases. Our posture:

### In scope (built into the executor from day one)

- **ACL enforcement.** Only registered tools are callable. Unregistered tool names panic with a clear error. Drawn from gridctl's `toolSet` pattern.
- **Execution timeout.** Context-based deadline with `vm.Interrupt()`. Prevents infinite loops and resource exhaustion (threat P5.5).
- **Code size limit.** Max 64KB input (gridctl's `MaxCodeSize`). Prevents context-stuffing attacks.
- **Fresh VM per execution.** No state leakage between executions. Each call gets an isolated runtime.
- **Max tool call limit.** Cap on how many `callTool` invocations per execution (agent-go uses 20). Prevents runaway tool loops.
- **No filesystem/network access.** goja is sandboxed by default — no `require()`, no `fs`, no `fetch`. Only injected bindings are accessible.
- **Code normalization.** Strip markdown fences, handle various LLM output formats before execution. Prevents injection via formatting artifacts.

### Out of scope (not needed for ghx, but noted for SDK consumers)

- Container/WASM isolation — overkill for a local dev tool. SDK consumers building multi-tenant systems should add this layer.
- Pre-execution semantic gating (LLM judge) — the paper recommends this for production CE-MCP. Not needed when the tool set is small and trusted.
- Post-execution output validation — relevant when tool outputs are untrusted. ghx's tools return GitHub API data, which is trusted.

### Security principle

The executor is a sandbox, not a fortress. It prevents accidental damage (infinite loops, unregistered tool calls, oversized inputs). It does not defend against a malicious actor who controls the LLM itself — that's a different threat model.

---

## 10. Error Recovery

When LLM-generated code fails:

1. **Syntax errors** → esbuild transpilation catches them before execution. Return clear error with line/column.
2. **Runtime errors** → goja returns the error with stack trace. The MCP server returns it as a tool error result.
3. **Timeout** → `vm.Interrupt()` fires, execution stops, return timeout error with the configured duration.
4. **Tool call failures** → Individual tool errors are thrown as JS exceptions. The LLM's code can `try/catch` them, or they propagate up.

**No automatic retry loop.** The executor runs code once and returns the result (or error). Retry decisions belong to the LLM/agent layer, not the executor. This avoids the "regeneration loop" attack vector identified in the Ben Gurion paper (threat P2.2) where adversarial exceptions manipulate re-planning.

---

## 11. Executor Interface Contract

The core API of the SDK:

```go
type Executor struct { /* timeout, transpiler config */ }

type ExecuteResult struct {
    Value   string   // Return value (JSON-encoded)
    Console []string // Captured console.log/warn/error output
    Calls   []ToolCallRecord // Tool calls made (name, args, duration, error)
}

type ToolCallRecord struct {
    Tool     string
    Args     map[string]any
    Duration time.Duration
    Error    string // empty if successful
}

func (e *Executor) Execute(ctx context.Context, code string, tools []Tool) (*ExecuteResult, error)
```

- `Execute` is stateless — fresh VM per call, no side effects
- `ctx` carries the timeout deadline
- `tools` defines what's callable (ACL is implicit — if it's not in the list, it's not callable)
- `ToolCallRecord` provides observability without requiring external tracing infrastructure
- Return value is JSON-encoded string (matches how MCP tool results work)

---

## 12. Observability

Drawn from agent-go's `ToolCallRecord` pattern and gridctl's console capture:

- **Console capture.** `console.log/warn/error` output returned in `ExecuteResult.Console`. Useful for debugging LLM-generated code.
- **Tool call records.** Every `callTool` invocation recorded with name, args, duration, and error. Returned in `ExecuteResult.Calls`.
- **Execution duration.** Total wall-clock time of the execution, available via standard Go timing.
- **No external tracing dependency.** Observability is built into the return type, not into OpenTelemetry or similar. SDK consumers can wire it to their own tracing if needed.

---

## 13. Architecture Evolution Pathway

### Phase 1: Sequential executor (MVP)
- `callTool` is synchronous — blocks until the Go function returns
- One tool call at a time within a script
- This is what gridctl ships today. Sufficient for ghx's 5 tools.

### Phase 2: Parallel tool calls
- Add `Promise.all` support via goja's event loop
- `callTool` returns a Promise. Multiple calls can be in-flight concurrently.
- Go side: each tool call runs in its own goroutine, results collected and resolved.
- Enables fan-out patterns: explore 5 repos in parallel within one script.

### Phase 3: Streaming results
- Add a `stream(value)` binding that emits partial results mid-execution
- Go side: results sent on a channel, MCP server forwards via streamable HTTP
- Enables progressive disclosure: stream file contents as they're read, not all at once.

### Phase 4: Persistent sessions
- VM persists across multiple `Execute` calls within a session
- State (variables, cached results) carries over
- Enables conversational codemode: "now filter those results by stars > 1000"

Each phase is additive — Phase 1's API doesn't break when Phase 2 lands. The `ExecuteResult` struct grows (adds streaming channel in Phase 3, session ID in Phase 4) but existing fields remain stable.

---

## 14. Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| esbuild adds ~10MB to binary | Low | Acceptable for dev tool. Can strip in future if pure-Go ES2015 transpiler emerges. |
| goja ES2015 limitation causes subtle bugs | Medium | esbuild transpilation handles modern syntax. Test suite covers async/await, `?.`, `??`. |
| LLMs generate code that exceeds 64KB limit | Low | 64KB is ~16K tokens of code. No realistic tool orchestration script is that large. |
| mark3labs/mcp-go introduces breaking changes | Low | Pin version. Library is stable (v1.x). Active community. |
| Reflect-based type generation produces wrong schemas | Medium | Golden file tests catch regressions. Start with flat structs, iterate. |
| goja memory usage under concurrent load | Low | Fresh VM per call means GC reclaims after each execution. No accumulation. |
| Tool call within script hangs indefinitely | Medium | Context timeout applies to entire execution including tool calls. Go side can also timeout individual tool calls. |

---

## Goal

Build a codemode executor that is concurrent from day one. Not because ghx needs it today — ghx is a single-user CLI tool. But because the codemode SDK is designed to be reusable, and the next wave of consumers will be multi-agent systems, streaming pipelines, and concurrent MCP servers. The architecture must not paint us into a single-threaded corner.

---

## Prior Art Referenced

| Project | What we learned | Key files read |
|---------|----------------|----------------|
| [gridctl/gridctl](https://github.com/gridctl/gridctl) | goja + esbuild sandbox pattern, ACL enforcement, tool binding injection | `pkg/mcp/codemode_sandbox.go`, `codemode_transpile.go`, `codemode_tools.go` |
| [liliang-cn/agent-go](https://github.com/liliang-cn/agent-go) | IIFE wrapping, fuzzy tool name resolution, Go→JS type normalization, max call limits | `pkg/ptc/runtime/goja/runtime.go` |
| [fugue-labs/gollem](https://github.com/fugue-labs/gollem) | monty-go WASM integration, mutex limitation, CodeMode wrapper pattern | `ext/monty/monty.go` |
| [fugue-labs/monty-go](https://github.com/fugue-labs/monty-go) | WASM Python interpreter, pause/resume model, wazero integration | `monty.go`, `wasm.go` |
| [maximhq/bifrost](https://github.com/maximhq/bifrost) | Starlark-based codemode, CodeMode interface design | `core/mcp/codemode/starlark/starlark.go` |
| [imran31415/codemode-sqlite-mcp](https://github.com/imran31415/codemode-sqlite-mcp) | yaegi Go interpreter, stdout capture hack, interpreter pool | `pkg/executor/executor.go` |
| [Protocol-Lattice/go-agent](https://github.com/Protocol-Lattice/go-agent) | UTCP + codemode, agent-as-tool, streaming tool calls | `agent_stream.go`, codemode examples |
| [universal-tool-calling-protocol/go-utcp](https://github.com/universal-tool-calling-protocol/go-utcp) | CallToolStream, codemode orchestrator with streaming | `src/plugins/codemode/orchestrator.go` |
| Ben Gurion University, "From Tool Orchestration to Code Execution" (Feb 2026) | CE-MCP formalization, context-coupled vs context-decoupled, security threat model | [arxiv.org/html/2602.15945v1](https://arxiv.org/html/2602.15945v1) |
