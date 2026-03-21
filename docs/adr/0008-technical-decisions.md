# ADR-0008: Technical Decisions — Dependencies, Module Structure, and Interfaces

**Date**: 2026-03-21
**Status**: Research (pending decisions)
**Parent**: ADR-0007

## Purpose

ADR-0007 defined the architecture (core + CLI + MCP + codemode) and the build order. This ADR resolves the open technical decisions needed before implementation begins.

---

## 1. Go MCP Library: mark3labs/mcp-go vs metoro-io/mcp-golang

### Research

| Criteria | mark3labs/mcp-go | metoro-io/mcp-golang |
|----------|-----------------|---------------------|
| Stars | ~4,800+ (most popular) | 1,208 |
| MCP spec version | 2025-11-25 (latest) | Older |
| Transports | stdio, SSE, streamable HTTP | stdio, SSE |
| Tool definition | Builder pattern: `mcp.NewTool("name", mcp.WithString(...))` | Struct tags: `type Args struct { Name string \`json:"name" mcp:"required"\` }` |
| Session management | Full (per-session tools, filtering) | Basic |
| Task support | Yes (async tool execution) | No |
| Test helpers | `mcptest` package | None |
| Used by | gridctl, mcp-gopls, 50+ projects | incident-io, smaller projects |

### Decision: mark3labs/mcp-go

**Rationale:**
- Most widely adopted Go MCP library — largest community, most battle-tested
- Supports streamable HTTP (needed for non-stdio deployments)
- Builder pattern for tool definitions is cleaner for programmatic registration
- `mcptest` package simplifies testing
- gridctl (our primary codemode reference) uses it — less friction when drawing from their patterns

---

## 2. JS Runtime: dop251/goja

### Research

| Option | Language | Async/await | CGO | Binary size impact | Used by |
|--------|----------|-------------|-----|-------------------|---------|
| dop251/goja | Pure Go | ES2015 only (needs transpiler) | No | ~2-3MB | gridctl, agent-go, pocketbase, k6 |
| nicholasgasior/goja | Fork of dop251 | Same | No | Same | Few projects |
| nicholasgasior/nicholasgasior/nicholasgasior | — | — | — | — | — |
| AnywhereJS/nicholasgasior | — | — | — | — | — |
| AnywhereJS/nicholasgasior | — | — | — | — | — |

Only one real option: `dop251/goja`. It's the standard Go JS runtime. Used by Grafana k6 (load testing), PocketBase, gridctl, and agent-go. Pure Go, no CGO, well-maintained.

### The async/await question

goja only supports ES2015. LLMs sometimes generate modern JS with:
- `async/await`
- Optional chaining (`?.`)
- Nullish coalescing (`??`)
- Template literal tags

**Mitigation: esbuild transpilation** (proven in gridctl)
- esbuild has a Go API (`github.com/nicholasgasior/esbuild/pkg/api`) — pure Go, no CGO
- Transpile modern JS → ES2015 before execution
- Adds ~10MB to binary size (esbuild is large)

**Alternative: skip transpilation, constrain to ES2015**
- Add a note in the type stubs: "// ES2015 only — no async/await, use callbacks"
- LLMs can generate ES2015 when instructed
- Saves 10MB binary size
- Risk: LLMs occasionally ignore instructions and generate modern JS anyway

### Decision: dop251/goja + esbuild transpilation

**Rationale:**
- Proven combination in gridctl production
- LLMs generate modern JS by default — fighting this with prompt instructions is fragile
- 10MB binary size increase is acceptable for a developer tool (ghx is already 4.5MB)
- esbuild Go API means no external dependency at runtime

---

## 3. Module Structure: Monorepo vs Separate Module

### Options

**Option A: Everything in ghx repo**
```
github.com/gkoreli/ghx/v2
├── pkg/ghx/        # Core library
├── pkg/codemode/   # Codemode package
├── cmd/            # CLI + MCP server
├── go.mod          # Single module
```

**Option B: Codemode as separate Go module**
```
github.com/gkoreli/go-codemode   # Separate repo
├── executor.go
├── transpile.go
├── typegen.go
├── registry.go
├── normalize.go
├── go.mod

github.com/gkoreli/ghx/v2       # ghx repo
├── pkg/ghx/
├── cmd/
├── go.mod  # imports go-codemode
```

### Decision: Option A (monorepo), extract later if needed

**Rationale:**
- Faster iteration — no cross-repo dependency management during initial development
- The codemode package is ~400 lines — doesn't justify a separate repo yet
- If a second consumer appears, extract to `github.com/gkoreli/go-codemode` then
- Go modules support `pkg/codemode/` as an importable package within the same module

---

## 4. Tool Registration Interface

How does a Go developer register their functions with the codemode package?

### Options explored

**Option A: Struct tags (metoro-io style)**
```go
type ExploreArgs struct {
    Repo string `json:"repo" codemode:"required,description=Repository"`
    Path string `json:"path" codemode:"description=Subdirectory path"`
}
```
Pro: Declarative. Con: Requires reflection, limited expressiveness.

**Option B: Builder pattern (mark3labs/mcp-go style)**
```go
registry.Register("explore",
    codemode.WithDescription("Explore a GitHub repo"),
    codemode.WithString("repo", codemode.Required()),
    codemode.WithString("path"),
    codemode.WithHandler(func(args map[string]any) (any, error) {
        return ghx.Explore(args["repo"].(string), args["path"].(string))
    }),
)
```
Pro: Matches MCP library. Con: Verbose.

**Option C: Simple function signature (minimal)**
```go
registry.Register("explore", "Explore a GitHub repo", ghx.Explore)
// Type stubs auto-generated from function signature via reflect
```
Pro: Minimal boilerplate. Con: Reflection magic, less control over descriptions.

**Option D: Hybrid — simple registration + schema override**
```go
registry.Register(codemode.Tool{
    Name:        "explore",
    Description: "Explore a GitHub repo — returns branch, file tree, and README",
    Func:        ghx.Explore,
    // Schema auto-generated from Func signature
    // Override specific fields if needed:
    Args: map[string]codemode.Arg{
        "repo": {Required: true, Description: "owner/repo format"},
    },
})
```
Pro: Minimal for simple cases, full control when needed. Con: Slightly more complex type.

### Decision: Option D (hybrid)

**Rationale:**
- Auto-generates schema from Go function signatures (less boilerplate than B)
- Allows description overrides for LLM-friendly documentation (more control than C)
- The `Func` field accepts `any` — uses reflect to extract parameter types for TS stub generation
- Aligns with how Cloudflare's `ToolProvider` works conceptually

---

## 5. Binary Distribution: Single Binary vs Separate Binaries

### Question

Should `ghx serve` (MCP server) and `ghx codemode` be part of the main `ghx` binary, or separate binaries?

### Decision: Single binary, subcommands

```
ghx explore ...        # CLI mode
ghx serve              # MCP server mode (stdio by default)
ghx serve --http :8080 # MCP server mode (HTTP)
```

**Rationale:**
- One binary to install, one binary to update
- Codemode is exposed through the MCP server (as `search` + `execute` meta-tools), not as a separate interface
- Users don't need to think about "which binary" — `ghx` does everything
- Binary size (~15MB with goja + esbuild) is acceptable for a developer tool

---

## 6. Type Stub Generation: Build Time vs Runtime

### Question

When are TypeScript type stubs generated for the LLM context?

### Decision: Runtime generation

**Rationale:**
- Type stubs are generated when the MCP server starts or when codemode `search` is called
- No build step needed — register tools, stubs are generated automatically
- Stubs change when tools change — runtime generation keeps them in sync
- gridctl does this at runtime, Cloudflare does this at runtime — proven pattern
- The generation is fast (~1ms for 5 tools) — no performance concern

---

## 7. MCP Transport

### Decision: stdio (primary) + streamable HTTP (secondary)

**Rationale:**
- stdio is the standard for local MCP servers (Claude Desktop, Cursor, Claude Code all use it)
- Streamable HTTP enables remote usage and web-based clients
- mark3labs/mcp-go supports both out of the box
- SSE is deprecated in favor of streamable HTTP in the latest MCP spec

---

## 8. Testing Strategy

### Approach

| Layer | Testing method |
|-------|---------------|
| Core (`pkg/ghx/`) | Unit tests with mocked GitHub API responses |
| Codemode executor | Snapshot tests: JS input → expected output. Test timeout, ACL enforcement, console capture |
| Codemode type gen | Golden file tests: Go function signatures → expected TS stubs |
| MCP server | Integration tests using `mcptest` package from mark3labs/mcp-go |
| CLI | Existing bash test patterns (verify output format parity with v0.x) |

**Key test cases for codemode executor:**
- Happy path: JS code calls tools, returns result
- Timeout: code exceeds deadline, gets interrupted
- ACL: code tries to call unregistered tool, gets rejected
- Console capture: `console.log` output is returned alongside result
- Transpilation: modern JS (async/await, `?.`, `??`) transpiles and executes correctly
- Error handling: syntax errors, runtime errors return clean error messages
- Code normalization: markdown fences stripped, various function formats handled

---

## Summary of Decisions

| Decision | Choice | Key rationale |
|----------|--------|---------------|
| MCP library | mark3labs/mcp-go | Most adopted, streamable HTTP, builder pattern |
| JS runtime | dop251/goja + esbuild | Proven in gridctl, pure Go, handles modern JS |
| Module structure | Monorepo (extract later) | Faster iteration, ~400 lines doesn't justify separate repo |
| Tool registration | Hybrid (auto-gen + override) | Minimal boilerplate, full control when needed |
| Distribution | Single binary, subcommands | One install, codemode via MCP server |
| Type stubs | Runtime generation | No build step, always in sync |
| MCP transport | stdio + streamable HTTP | Standard local + remote support |

## Dependencies Summary

| Package | Purpose | Size impact | CGO |
|---------|---------|-------------|-----|
| `github.com/mark3labs/mcp-go` | MCP server | Minimal | No |
| `github.com/dop251/goja` | JS execution sandbox | ~2-3MB | No |
| `github.com/evanw/esbuild` | JS transpilation (modern → ES2015) | ~10MB | No |
| `github.com/cli/go-gh/v2` | GitHub API (existing) | Already included | No |
| `github.com/spf13/cobra` | CLI framework (existing) | Already included | No |

Total binary size estimate: ~15-17MB (up from 4.5MB). All pure Go, no CGO, cross-compiles cleanly.

## Open Questions (deferred to implementation)

- Should codemode `execute` support returning streaming results or just final output?
- Should the tool registry support hot-reloading (add/remove tools without restart)?
- What's the right prompt/system message to include with type stubs so LLMs generate clean ES2015?
