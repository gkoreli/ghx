# ADR-0010: The Code Meta-Tool — Collapsing N Tool Calls Into One Execution

**Date**: 2026-03-21
**Status**: Accepted
**Parent**: ADR-0008, ADR-0009

## Purpose

ADRs 0008-0009 built the executor and type system. This ADR defines how they're exposed to LLMs: a single `code` tool that wraps all ghx tools, letting the LLM write one program instead of making N individual tool calls.

---

## Invariant: Core Is the Source of Truth

```
pkg/ghx/       → defines what operations exist
pkg/codemode/  → defines how code execution works
cmd/ghx.go     → exposes EVERYTHING via CLI (including codemode)
cmd/serve.go   → exposes the same things via MCP (no unique features)
```

If you can't do it from `ghx <command>`, it doesn't exist yet. MCP never gets capabilities that CLI doesn't have. CLI is the first frontend for every feature — MCP wraps the same core functions.

This means:
- `ghx code "..."` must exist before the MCP `code` tool
- `ghx code --list` must exist before the MCP `search_tools` tool
- Any new tool added to MCP must have a CLI equivalent

---

## The Problem With Individual Tool Calls

Today, `ghx serve` exposes 5 MCP tools. An LLM exploring a repo does:

```
Round 1: call explore({ repo: "vercel/next.js" })     → wait → think
Round 2: call read({ repo: ..., files: ["src/..."] })  → wait → think
Round 3: call search({ query: "useState" })            → wait → think
Round 4: call read({ repo: ..., files: [...] })        → wait → think
```

Each round-trip: LLM generates a tool call → MCP server executes → result returns → LLM reasons about result → generates next call. 4 operations = 4 round-trips, 4 reasoning steps, 4x the tokens.

## The Solution: One `code` Tool

Add a 6th tool that accepts JavaScript. The LLM writes a program that calls all 5 tools in one shot:

```javascript
async () => {
  const repo = await codemode.explore({ repo: "vercel/next.js" });
  const goFiles = repo.files.filter(f => f.name.endsWith(".go"));
  const contents = await codemode.read({
    repo: "vercel/next.js",
    files: goFiles.map(f => f.name)
  });
  return contents.filter(c => c.content.includes("GraphQL"));
}
```

One round-trip. One reasoning step. The filtering logic (`endsWith`, `includes`) runs in the sandbox, not in the LLM.

### Why this works

Cloudflare's README states it directly: "LLMs are better at writing code than calling tools — they've seen millions of lines of real-world code but only contrived tool-calling examples."

The Ben Gurion CE-MCP paper measured it: ~98% token reduction, 60% faster execution.

---

## Design: Dual-Mode, CLI-First

### CLI commands (source of truth)

```bash
ghx code "var r = codemode.explore({repo: 'dop251/goja'}); return r.Branch;"
ghx code -                          # read code from stdin (for piping)
ghx code --list                     # list available tools with type stubs
```

### MCP tools (thin wrappers over the same functions)

```
ghx serve
├── explore    (direct tool — simple queries)
├── read       (direct tool — simple queries)
├── search     (direct tool — simple queries)
├── repos      (direct tool — simple queries)
├── tree       (direct tool — simple queries)
├── code       (meta-tool — wraps the same executor as ghx code)
└── search_tools (wraps the same listing as ghx code --list)
```

Both modes coexist. Simple queries ("explore this repo") use direct tools. Complex queries ("find all Go files that import GraphQL, read them, and summarize the patterns") use the `code` tool.

The LLM chooses which mode to use based on the task complexity. No orchestrator needed — the tool description guides the LLM.

---

## The `code` Tool Specification

### Tool Definition

```go
codeTool := mcp.NewTool("code",
    mcp.WithDescription(codeDescription), // includes type stubs
    mcp.WithString("code", mcp.Required(),
        mcp.Description("JavaScript async arrow function to execute")),
)
```

### Tool Description (injected into LLM context)

```
Execute code to achieve a goal.

Available:

type ExploreInput = { repo: string; path?: string }
type ExploreOutput = { branch: string; files: { name: string; type: string }[]; readme: string }
...
declare const codemode: {
  /** Explore a GitHub repo — returns branch, file tree, and README */
  explore: (input: ExploreInput) => Promise<ExploreOutput>;
  /** Read files from a repo */
  read: (input: ReadInput) => Promise<ReadOutput>;
  ...
}

Write an async arrow function in JavaScript that returns the result.
Do NOT use TypeScript syntax — no type annotations, interfaces, or generics.
Do NOT define named functions then call them — just write the arrow function body directly.

Example: async () => { const r = await codemode.explore({ repo: "vercel/next.js" }); return r.files; }
```

This is the ADR-0009 type stubs embedded in the tool description. The LLM sees the types when it sees the tool.

### Handler

```go
func handleCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    code, _ := request.RequireString("code")

    result, err := executor.Execute(ctx, code, registry.List())
    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    // Include console output if present
    output := result.Value
    if len(result.Console) > 0 {
        output = fmt.Sprintf("Console:\n%s\n\nResult:\n%s",
            strings.Join(result.Console, "\n"), result.Value)
    }

    return mcp.NewToolResultText(output), nil
}
```

The handler is thin — it passes code to the executor (ADR-0008) with the registered tools (ADR-0008 §4), and returns the result.

---

## The `codemode` Object Injection

The executor must inject a `codemode` object (not bare `callTool`). This matches the type stubs and gives the LLM a clean API:

```go
// In executor.go — inject codemode namespace object
codemodeObj := vm.NewObject()
for _, tool := range tools {
    t := tool // capture
    codemodeObj.Set(t.Name, func(call goja.FunctionCall) goja.Value {
        args := call.Argument(0).Export().(map[string]any)
        result, err := t.Func(args)
        if err != nil {
            panic(vm.NewGoError(err))
        }
        return vm.ToValue(result)
    })
}
vm.Set("codemode", codemodeObj)
```

The LLM writes `codemode.explore(...)`, the executor routes it to the registered tool function. ACL is implicit — only registered tools appear on the object.

---

## Code Normalization (from Cloudflare)

LLMs produce code in various formats. Cloudflare's `normalizeCode` (which uses acorn AST parsing) handles:

| LLM output | Normalized to |
|------------|---------------|
| `async () => { ... }` | Pass through (already correct) |
| `const x = 1; x` | `async () => { const x = 1; return (x) }` |
| `` ```js\n...\n``` `` | Strip fences, then normalize |
| `export default function() { ... }` | Unwrap export, wrap in async arrow |
| `function doStuff() { ... }` | `async () => { function doStuff() { ... } return doStuff(); }` |

Our `normalize.go` currently only strips markdown fences. It needs to grow to handle the "bare expression" and "named function" cases. The AST approach (acorn in Cloudflare's case) is more robust than regex, but for MVP, regex covering the top 3 cases is sufficient.

---

## Namespaced Tool Providers (Future)

Cloudflare's `ToolProvider` pattern supports multiple namespaces in one sandbox:

```javascript
// Multiple providers, each with its own namespace
const files = await state.glob("/src/**/*.ts");
const analysis = await codemode.analyzeFile({ path: files[0] });
await db.query("INSERT INTO results ...");
```

For ghx, everything lives under `codemode.*`. But the executor design supports this — `vm.Set("namespace", obj)` for each provider. If ghx later integrates with other tool sources (filesystem, database), they get their own namespace.

---

## Response Truncation

Cloudflare caps responses at 6,000 tokens (`MAX_TOKENS = 6000`, `CHARS_PER_TOKEN = 4`, `MAX_CHARS = 24000`). This prevents a `read` of a large file from blowing the LLM's context window on the return path.

For ghx: adopt the same cap. If the executor result exceeds 24K chars, truncate with a message: `"[truncated — result was {N} chars, showing first 24000]"`. The LLM can then refine (use `--grep`, `--map`, or `--lines` to get a smaller result).

---

## Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Source of truth | Core → CLI → MCP (never MCP-only features) | CLI is testable without MCP, agents with shell access use CLI directly |
| Exposure pattern | Dual-mode: 5 direct tools + 1 `code` meta-tool | Simple queries stay simple, complex queries use codemode |
| CLI command | `ghx code "..."` / `ghx code -` / `ghx code --list` | CLI-first, MCP wraps the same functions |
| LLM API surface | `codemode.toolName(args)` object | Matches type stubs, clean namespace, implicit ACL |
| Code format | Async arrow function in JS | Cloudflare's proven pattern, LLMs generate this naturally |
| Normalization | Strip fences + wrap bare code in async arrow | Handle common LLM output formats |
| Response cap | 24K chars (6K tokens) | Prevent context window blowout on return path |
| Namespacing | Single `codemode` namespace (extensible) | Sufficient for ghx, supports multi-provider later |

## Implementation Status (Wave 10 complete)

| Feature | Status | Evidence |
|---------|--------|----------|
| CLI `ghx code "..."` | ✅ | `cmd/code.go` (65 lines), commit `678ff22` |
| CLI `ghx code -` (stdin) | ✅ | Same file, tested: `echo 'return 1+1' \| ./ghx code -` → `2` |
| CLI `ghx code --list` | ✅ | Prints `declare const codemode: { ... }` type stubs |
| MCP `code` tool | ✅ | `cmd/serve.go`, commit `9bc5349` (wave 8-9) |
| MCP `search_tools` tool | ✅ | Same file, returns type stubs + tool descriptions |
| `codemode` object injection | ✅ | `executor.go` — `codemode.explore()` not `callTool()` |
| Type stubs in description | ✅ | `typegen.go` → `declare const codemode: { ... }` injected into tool description |
| 24K response truncation | ✅ | `serve.go` — MCP only (CLI outputs full result) |
| Code normalization | ✅ Partial | `normalize.go` (63 lines) — fences, named functions, bare expressions. Not full AST (acorn). |
| CLI-first invariant | ✅ | Every MCP capability has a CLI equivalent |

### Verified smoke tests

```bash
# CLI inline
./ghx code "return 1 + 1"                                    # → 2
# CLI stdin
echo 'return 1 + 1' | ./ghx code -                           # → 2
# CLI list
./ghx code --list                                             # → declare const codemode: { ... }
# CLI real API
./ghx code "var r = codemode.explore({repo: 'dop251/goja'}); return {branch: r.branch, fileCount: r.files.length};"
                                                              # → {"branch":"master","fileCount":107}
# MCP code tool
echo '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"code","arguments":{"code":"..."}}}' | ghx serve
                                                              # → {"branch":"master","fileCount":107}
```

### Codebase (2,351 lines total)

```
cmd/code.go          65   ← NEW (wave 10) — CLI frontend for codemode
cmd/serve.go        302   ← MCP server (code + search_tools + 5 direct tools)
cmd/ghx.go          245   ← CLI frontend (repos, explore, read, search, tree)
pkg/codemode/       955   ← executor (296), registry (78), normalize (63), transpile (21), typegen (128), tests (369)
pkg/ghx/            784   ← core library: explore, read, search, repos, tree, register
```

### Remaining gaps

| Gap | Severity | Notes |
|-----|----------|-------|
| Normalization depth | Low | Current: regex (fences, named funcs, bare expressions). Cloudflare uses acorn AST. Regex covers top 3 LLM output patterns — sufficient for now. |
| `export default` handling | Low | normalize.go doesn't unwrap `export default function`. Rare in LLM output for tool orchestration. |
| CLI `--timeout` flag | Low | MCP has context timeout. CLI uses `context.Background()` — no user-configurable timeout. |

---

## What's Next — Research Directions

### 1. Real-world agent accuracy testing

The entire premise of ADR-0010 is that LLMs write better code than tool calls. This is claimed by Cloudflare and measured by Ben Gurion (~98% token reduction). But we haven't tested it with ghx's specific tools.

**Research question**: Given ghx's 5 tools and type stubs, does an LLM (Claude, GPT-4) actually produce correct `codemode.explore()` / `codemode.read()` calls on first try? What's the error rate? What patterns fail?

**Method**: Feed the `code` tool description (with type stubs) to an LLM. Give it 10 tasks of increasing complexity. Measure: correct on first try, needed retry, failed entirely.

### 2. Normalization: regex vs AST

Current `normalize.go` is 63 lines of regex. Cloudflare's `normalizeCode` uses acorn (full JS parser). The question is whether the regex approach breaks on real LLM output.

**Research question**: Collect 50+ real LLM code outputs from agent sessions. How many does regex normalize correctly vs incorrectly? Is the failure rate high enough to justify an AST parser?

### 3. Distribution pipeline (ADR-0007)

GoReleaser + npm wrapper + Homebrew tap. The architecture is defined in ADR-0007 but none of it is implemented. This is the path from "works on my machine" to "anyone can install it."

### 4. Phase 2 executor: parallel tool calls (ADR-0008 §13)

`Promise.all([codemode.explore(...), codemode.search(...)])` — fan-out patterns. Requires goja event loop integration. gridctl doesn't do this yet. Would be novel for a Go codemode SDK.

**Research question**: How does goja's event loop work? Can we wire goroutine-backed promises? What's the concurrency model?

### 5. Streaming results mid-execution (ADR-0008 §13 Phase 3)

`stream(partialResult)` binding that emits results while the script is still running. Requires streamable HTTP transport (already supported by mcp-go). Enables progressive disclosure for large reads.

### 6. Persistent sessions (ADR-0008 §13 Phase 4)

VM persists across multiple `Execute` calls. State carries over. Enables conversational codemode: "now filter those results by stars > 1000." Requires session management, memory limits, GC strategy.

### 7. Multi-provider namespacing

ADR-0010 mentions `codemode.*` as the single namespace. But the executor supports `vm.Set("namespace", obj)` for any number of providers. If ghx integrates filesystem tools, database tools, or other MCP servers, they'd get their own namespace.

**Research question**: What's the right abstraction for multi-provider registration? How do type stubs compose across namespaces?

---

## Prior Art

| Source | Key pattern adopted |
|--------|-------------------|
| [Cloudflare `@cloudflare/codemode`](https://github.com/cloudflare/agents/tree/main/packages/codemode) | `createCodeTool` wrapping existing tools into single `code` tool. Dual-mode (direct + codemode). `normalizeCode` via AST. Response truncation at 6K tokens. ToolProvider namespacing. |
| [Cloudflare `codeMcpServer`](https://github.com/cloudflare/agents/blob/main/packages/codemode/src/mcp.ts) | MCP-specific wrapper: connect to upstream MCP server, discover tools, generate types, expose single `code` tool. In-memory transport for zero-latency internal connection. |
| [Cloudflare `normalizeCode`](https://github.com/cloudflare/agents/blob/main/packages/codemode/src/normalize.ts) | AST-based normalization via acorn. Handles arrow functions, named functions, bare expressions, export default, markdown fences. |
