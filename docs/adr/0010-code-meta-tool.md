# ADR-0010: The Code Meta-Tool — Collapsing N Tool Calls Into One Execution

**Date**: 2026-03-21
**Status**: Accepted
**Parent**: ADR-0008, ADR-0009

## Purpose

ADRs 0008-0009 built the executor and type system. This ADR defines how they're exposed to LLMs: a single `code` tool that wraps all ghx tools, letting the LLM write one program instead of making N individual tool calls.

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

## Design: Dual-Mode MCP Server

```
ghx serve
├── explore    (direct tool — simple queries)
├── read       (direct tool — simple queries)
├── search     (direct tool — simple queries)
├── repos      (direct tool — simple queries)
├── tree       (direct tool — simple queries)
└── code       (meta-tool — complex multi-step operations)
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
| Exposure pattern | Dual-mode: 5 direct tools + 1 `code` meta-tool | Simple queries stay simple, complex queries use codemode |
| LLM API surface | `codemode.toolName(args)` object | Matches type stubs, clean namespace, implicit ACL |
| Code format | Async arrow function in JS | Cloudflare's proven pattern, LLMs generate this naturally |
| Normalization | Strip fences + wrap bare code in async arrow | Handle common LLM output formats |
| Response cap | 24K chars (6K tokens) | Prevent context window blowout on return path |
| Namespacing | Single `codemode` namespace (extensible) | Sufficient for ghx, supports multi-provider later |

## Prior Art

| Source | Key pattern adopted |
|--------|-------------------|
| [Cloudflare `@cloudflare/codemode`](https://github.com/cloudflare/agents/tree/main/packages/codemode) | `createCodeTool` wrapping existing tools into single `code` tool. Dual-mode (direct + codemode). `normalizeCode` via AST. Response truncation at 6K tokens. ToolProvider namespacing. |
| [Cloudflare `codeMcpServer`](https://github.com/cloudflare/agents/blob/main/packages/codemode/src/mcp.ts) | MCP-specific wrapper: connect to upstream MCP server, discover tools, generate types, expose single `code` tool. In-memory transport for zero-latency internal connection. |
| [Cloudflare `normalizeCode`](https://github.com/cloudflare/agents/blob/main/packages/codemode/src/normalize.ts) | AST-based normalization via acorn. Handles arrow functions, named functions, bare expressions, export default, markdown fences. |
