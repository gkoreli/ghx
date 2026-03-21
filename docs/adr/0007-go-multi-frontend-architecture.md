# ADR-0007: Go Rewrite — Multi-Frontend Architecture

**Date**: 2026-03-21
**Status**: Accepted
**Supersedes**: ADR-0006

## Context

ADR-0006 concluded "stay with bash" based on a swarm experiment that reported Go at "40% functional." This was wrong. The swarm-written Go binary needed 5 lines of manual fixes (QueryRaw → Do, unused imports/vars) to reach 100% functional parity:

| Command | Status |
|---------|--------|
| version | ✅ |
| repos --limit | ✅ GraphQL via go-gh, README stripping clean |
| search --limit | ✅ REST via go-gh with text_matches |
| explore | ✅ Single GraphQL call |
| read --grep | ✅ |
| read --map | ✅ |
| read --lines | ✅ |
| tree | ✅ |
| skill | ✅ |
| unknown flag | ⚠️ Rejects, exits 0 not 2 (minor) |

ADR-0006 optimized for the local maximum (bash is simpler for a CLI tool) and closed the door on the global maximum (Go enables capabilities bash fundamentally cannot provide).

The real question was never "can we rewrite bash in Go" — it was "what can we build next that bash can't do?"

### What changed

Research into the codemode pattern (Cloudflare's `@cloudflare/codemode`, `datalayer/agent-codemode`, `ast-grep`) revealed a convergence: the best agent tools aren't just CLI commands — they're libraries with multiple frontends. ghx should work as a CLI tool, as an MCP server, and as a codemode target. Bash can only do the first.

### Codemode landscape (March 2026)

| SDK | Language | Stars | Key idea |
|-----|----------|-------|----------|
| `@cloudflare/codemode` | TypeScript | (in cloudflare/agents) | 2 tools replace N: agent writes code that composes operations |
| `datalayer/agent-codemode` | Python | 3 | Generate typed bindings from MCP, agent writes `execute_code` |
| `deeflect/universal-codemode` | TypeScript | 4 | 56 APIs via 2 MCP tools, ~1K tokens vs 500K+ |
| `jx-codes/lootbox` | TypeScript | (successor to 116★ codemode-mcp) | Deno, scripts become reusable tools |
| `imran31415/codemode-sqlite-mcp` | **Go** | — | yaegi sandbox, 5.9x fewer tokens, 2.1x faster |
| `spachava753/cpe` | **Go** | — | Full agent CLI with Code Mode, MCP, skills |

No general-purpose Go codemode SDK exists. The Go implementations are application-specific.

## Decision

Rewrite ghx in Go with a core library + multiple frontends architecture.

## Architecture

```
                     ghx core (Go package)
                     ├── Explore(repo, path) → ExploreResult
                     ├── Read(repo, files, opts) → []FileResult
                     ├── Search(query, opts) → SearchResult
                     ├── Repos(query, opts) → []RepoResult
                     └── Tree(repo, path, opts) → TreeResult
                                  │
                 ┌────────────────┼────────────────┐
                 │                │                 │
              CLI             MCP Server        Codemode
           (cobra)           (stdio/http)     (goja executor)
          ghx explore       tool: explore     agent writes:
          ghx read          tool: read          ghx.Read(repo, [f1,f2])
          ghx search        tool: search        ghx.Search(query)
```

### Layer 1: Core package (`pkg/ghx/`)

Pure Go functions. No CLI concerns, no I/O formatting. Takes structured input, returns structured output. Uses `go-gh` for GitHub API access.

```go
type ExploreResult struct {
    Branch string
    Files  []string
    Readme string
}

func Explore(repo string, path string) (*ExploreResult, error)
func Read(repo string, files []string, opts ReadOpts) ([]FileResult, error)
func Search(query string, opts SearchOpts) (*SearchResult, error)
func Repos(query string, opts ReposOpts) ([]RepoResult, error)
func Tree(repo string, path string, opts TreeOpts) (*TreeResult, error)
```

### Layer 2: CLI frontend (`cmd/`)

Thin wrapper. Parses flags via cobra, calls core, formats output to stdout/stderr. This is what `v2/cmd/ghx.go` already does — refactor to call core instead of inlining logic.

### Layer 3: MCP server frontend (`cmd/mcp/` or flag `ghx serve`)

Exposes core functions as MCP tools via stdio or HTTP transport. Uses `mark3labs/mcp-go` or similar. Each core function becomes one MCP tool with JSON schema input.

### Layer 4: Codemode frontend (`cmd/codemode/` or flag `ghx codemode`)

Embeds a JS runtime (goja — pure Go, no CGO) that exposes typed bindings to core functions. Agent writes JS that composes multiple operations in one round-trip:

```js
async () => {
  const tree = ghx.explore("plausible/analytics")
  const maps = ghx.read("plausible/analytics", ["mix.exs", "lib/plausible/stats/query.ex"], {map: true})
  const hits = ghx.search("defmodule repo:plausible/analytics")
  return { tree, maps, hits }
}
```

One LLM round-trip instead of three separate tool calls.

### Why goja for the executor

| Option | Pros | Cons |
|--------|------|------|
| goja (JS in pure Go) | No CGO, embeds cleanly, LLMs generate great JS | Not full V8, no async/await natively |
| yaegi (Go interpreter) | Go-native, used by codemode-sqlite-mcp | Slower, limited stdlib |
| Shell out to Deno/Node | Full JS runtime | External dependency, defeats single-binary |
| Structured plan (JSON DSL) | Simplest, no sandbox | Less flexible, can't do conditionals/loops |

goja is the sweet spot: single binary, LLMs write JS well, sandboxed by default.

### Resolved technical decisions (see ADR-0008 for full analysis)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| MCP library | `mark3labs/mcp-go` | Most adopted (~4800★), streamable HTTP, `mcptest` package |
| JS runtime | `dop251/goja` + esbuild transpilation | Proven in gridctl, handles modern JS → ES2015 |
| Module structure | Monorepo (extract codemode later if needed) | ~400 lines doesn't justify separate repo |
| Distribution | Single binary with subcommands (`ghx serve`, not `ghx-mcp`) | One install, codemode exposed via MCP server |
| Type stubs | Runtime generation | No build step, always in sync, ~1ms for 5 tools |
| MCP transport | stdio (primary) + streamable HTTP (secondary) | Standard for local + remote |
| Estimated binary | ~15-17MB | All pure Go, no CGO, cross-compiles cleanly |

## Build Order

1. **Extract core** — Pull logic from `v2/cmd/ghx.go` into `pkg/ghx/`. Structured input/output, no formatting.
2. **Wire CLI** — Refactor `cmd/ghx.go` to call core. Verify parity with bash version.
3. **Go codemode package** — Executor (goja), transpiler (esbuild), type generator, tool registry, code normalizer. ~400 lines.
4. **MCP server** — Add `ghx serve` command. Expose core as MCP tools via `mcp-golang`. Include codemode `search` + `execute` meta-tools.
5. **Wire codemode** — Register ghx's 5 core functions with codemode registry. MCP server exposes both direct tools and codemode.

Steps 1-2 are the foundation. Step 3 is the reusable package. Steps 4-5 are the new capabilities bash can't do.

## Why Go (revisited)

ADR-0006's criteria were correct but applied to the wrong question:

| ADR-0006 criterion | For CLI rewrite | For multi-frontend architecture |
|--------------------|-----------------|---------------------------------|
| Zero-install | Bash wins | Go wins (single binary, no runtime) |
| Agent reproducibility | Bash wins | Irrelevant (humans maintain core) |
| Complexity | Bash wins | Go wins (one codebase, three interfaces) |
| Dependencies | Bash wins (0) | Go acceptable (cobra, go-gh, goja) |
| What it enables | Same CLI | CLI + MCP + Codemode |

The swarm experiment proved Go works for ghx (5 lines of fixes). The codemode research proved bash can't go where ghx needs to go.

## Consequences

- `v2/` becomes the main development directory
- Bash `ghx` stays published as v0.x (stable, maintained for existing users)
- Go `ghx` ships as v2.x with CLI parity first, then MCP + codemode
- ADR-0006 is superseded — its conclusion was based on incomplete swarm data and the wrong evaluation frame
- The exit code 2 bug (unknown flags exit 0) is the only known gap — fix in step 2

## Distribution Strategy

Go cross-compiles to every platform from a single `go build`:

```bash
GOOS=linux GOARCH=amd64 go build -o ghx-linux-amd64 .
GOOS=darwin GOARCH=arm64 go build -o ghx-darwin-arm64 .
GOOS=windows GOARCH=amd64 go build -o ghx-windows-amd64.exe .
```

### Four install channels from one git tag

| Channel | How | User command |
|---------|-----|-------------|
| Go native | `go install` fetches from git, compiles locally | `go install github.com/gkoreli/ghx/v2@latest` |
| npm | CI cross-compiles, npm package ships platform binaries via postinstall | `npm install -g @gkoreli/ghx` / `npx @gkoreli/ghx` |
| curl | GitHub release assets, install script detects OS/arch | `curl -sf https://... \| sh` |
| Homebrew | Tap formula pointing at GitHub release binaries | `brew install gkoreli/tap/ghx` |

### CI pipeline (`auto-tag.yml` evolution)

Current: version bump → git tag → npm publish (bash script).
New: version bump → git tag → GoReleaser cross-compiles → GitHub release with binaries → npm publish with platform wrapper → Homebrew formula update.

GoReleaser handles the cross-compile matrix, checksums, and release notes from one config file.

### npm wrapper pattern

The npm package stops shipping the bash script and instead ships a thin JS installer:

```
package.json          ← bin: { "ghx": "bin/ghx" }
bin/ghx               ← shell wrapper that runs the platform binary
install.js            ← postinstall: detects OS/arch, downloads correct binary from GitHub releases
```

This preserves `npx @gkoreli/ghx` and `npm install -g @gkoreli/ghx` for existing users while shipping a native binary underneath. No Node runtime needed after install.

### Versioning

- Go modules: git tags (`v2.0.0`, `v2.1.0`) — Go convention requires `v2` prefix in module path for major version 2+
- npm: same version in `package.json`, published by CI on tag push
- Bash v0.x stays on npm as `@gkoreli/ghx@0.x` for backward compatibility until Go v2 reaches full parity

## Scope: Thin Go Codemode SDK — ghx as First Consumer

Rather than hardcoding codemode bindings into ghx, we build a thin, reusable Go codemode package. ghx is the first consumer. The SDK is small enough to justify (~400 lines) and the patterns are proven in production Go projects.

### Why build the SDK now (not later)

- The gap is real — no general-purpose Go codemode SDK exists. Every implementation is either JS/Python or application-specific Go.
- The research is done — we know exactly what the interface looks like from studying Cloudflare, gridctl, and agent-go.
- It's small — ~400 lines across 4 files. Not a framework, not a platform. A package.
- ghx benefits immediately — codemode support comes from importing a package, not writing sandbox plumbing.

### Prior art to draw from

| Project | What it proves | Key reference |
|---------|---------------|---------------|
| [gridctl/gridctl](https://github.com/gridctl/gridctl) | goja sandbox + esbuild transpilation + tool ACLs work in production Go | `pkg/mcp/codemode_sandbox.go` (~150 lines), `codemode_transpile.go`, `codemode_search.go`, `codemode_tools.go` |
| [liliang-cn/agent-go](https://github.com/liliang-cn/agent-go) | Full Go agent framework with PTC (Programmatic Tool Calling) via goja. LLM writes JS, `callTool()` injected | `pkg/ptc/runtime/goja/runtime.go` |
| [imran31415/codemode-sqlite-mcp](https://github.com/imran31415/codemode-sqlite-mcp) | Go codemode with yaegi interpreter. Benchmarked: 5.9x fewer tokens, 2.1x faster than standard MCP | `codemode/agent.go`, `pkg/executor/executor.go` |
| [metoro-io/mcp-golang](https://github.com/metoro-io/mcp-golang) (1,208★) | The Go MCP server library. Type-safe tool definitions, stdio transport | Use for MCP frontend |
| [@cloudflare/codemode](https://github.com/cloudflare/agents/tree/main/packages/codemode) | The reference SDK design: `Executor` interface, `ToolProvider`, `normalizeCode`, `generateTypes` | Interface design inspiration |

### SDK components (~400 lines total)

| Component | ~Lines | What it does | Draws from |
|-----------|--------|-------------|------------|
| Executor | ~150 | goja sandbox: fresh runtime per execution, timeout via context, console capture, tool call routing | gridctl `codemode_sandbox.go` |
| Transpiler | ~50 | Modern JS → ES2015 via esbuild (goja only supports ES2015) | gridctl `codemode_transpile.go` |
| Type generator | ~100 | Go function signatures → TypeScript type stubs for LLM context | New — reflect over registered tools, emit TS interfaces |
| Tool registry | ~80 | Register tools with name, description, typed schema. Search/list for discovery | gridctl `codemode_search.go` |
| Code normalizer | ~30 | Strip markdown fences, handle various LLM output formats, validate structure | Cloudflare `normalizeCode` |

### Known risks and mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| goja doesn't support async/await | Low | esbuild transpiles modern JS → ES2015. Proven in gridctl. |
| LLMs generate invalid JS | Low | Code normalizer strips fences, fixes common patterns. Proven in Cloudflare SDK. |
| Sandboxing gaps | Low | goja is sandboxed by default — no filesystem, no network. Only injected bindings are accessible. |
| Type generation accuracy | Medium | Start simple (flat structs), iterate. LLMs are forgiving of imperfect types. |
| esbuild dependency | Low | Pure Go alternative exists (esbuild has a Go API). Or skip transpilation and constrain to ES2015. |

### What this is NOT

- Not a framework with plugin systems and provider interfaces
- Not a Go port of `@cloudflare/codemode` — simpler, smaller, opinionated
- Not tied to any specific MCP library — the executor takes a `func(toolName string, args map[string]any) (any, error)` callback

### Updated build order

1. **Core ghx package** (`pkg/ghx/`) — Extract logic from `v2/cmd/ghx.go` into structured input/output functions
2. **CLI frontend** — Refactor `cmd/` to call core. Verify parity with bash version.
3. **Go codemode package** (`pkg/codemode/` or separate module) — Executor, transpiler, type gen, registry, normalizer
4. **MCP server frontend** — Expose core as MCP tools via `mcp-golang`. Add codemode `search` + `execute` meta-tools.
5. **Wire it together** — ghx registers its 5 core functions with the codemode registry. MCP server exposes both direct tools and codemode meta-tools.

## Swarm Execution Evidence

### Wave 4 results (10 agents, ~3 min each)

| Task | File | Lines | Result |
|------|------|-------|--------|
| TASK-0526 | `pkg/ghx/repos.go` | 132 | ✅ Full implementation, compiles |
| TASK-0527 | `pkg/ghx/search.go` | 89 | ✅ Full implementation, compiles |
| TASK-0528 | `pkg/ghx/explore.go` | 122 | ✅ Full implementation, compiles |
| TASK-0529 | `pkg/ghx/read.go` | 207 | ✅ Full implementation, grep break bug fixed |
| TASK-0530 | `pkg/ghx/tree.go` | 79 | ✅ Full implementation, compiles |
| TASK-0531 | Exit code 2 fix | — | ✅ Unknown flags now exit 2 |
| TASK-0532 | `pkg/codemode/executor.go` | 4 | ❌ Stub only — agent wrote type alias, skipped implementation |
| TASK-0533 | `pkg/codemode/registry.go` | 8 | ❌ Stub only — agent wrote struct, skipped implementation |
| TASK-0534 | `pkg/codemode/normalize.go` | 26 | ✅ Full implementation |
| TASK-0535 | `pkg/codemode/typegen.go` | 107 | ✅ Full implementation + test file |

**Success rate**: 8/10 (80%). Both failures were codemode tasks where the agent had less concrete reference code to extract from. The ghx core extraction tasks (which had exact source code to reference) were 6/6.

### Key swarm insights from wave 4

1. **Extraction tasks > creation tasks.** When agents have existing code to extract from, they deliver 100%. When they need to create from a description, they sometimes stub.
2. **Pre-installing dependencies prevents go.mod conflicts.** Running `go get github.com/dop251/goja` before spawning prevented 4 agents from racing on go.mod.
3. **File isolation works.** Zero merge conflicts across 10 agents because each created a new file.
4. **Agents don't commit.** Must be done by the orchestrator between waves.
5. **~200s per agent** for Go file creation tasks. Consistent across all 10.

## Open Questions — Resolved

All resolved in ADR-0008:

| Question | Answer | ADR-0008 section |
|----------|--------|-----------------|
| Separate binary or subcommand? | Subcommand: `ghx serve` | §5 Distribution |
| Streamable HTTP or just stdio? | Both (stdio primary, HTTP secondary) | §7 MCP Transport |
| Type stubs: build time or runtime? | Runtime (~1ms for 5 tools) | §6 Type Stubs |
| goja sufficient or need async/await? | goja + esbuild transpilation (modern JS → ES2015) | §1 Script Runtime |

### Wave 5 results (3 agents, 3/3 ✅)

| Task | File | Before → After | Result |
|------|------|----------------|--------|
| TASK-0536 | `pkg/codemode/executor.go` | 4 → 92 lines | ✅ Full goja sandbox |
| TASK-0537 | `pkg/codemode/registry.go` | 8 → 78 lines | ✅ Full tool registry |
| TASK-0538 | `cmd/ghx.go` | ~500 → 245 lines | ✅ Thin wrapper over pkg/ghx/ |

**Cumulative: 16 agents, 14 delivered first try (87.5%)**

Net effect of wave 5: -231 lines. Codebase got smaller while gaining a core library + codemode SDK.

### Build order progress

1. ✅ **Extract core** (`pkg/ghx/`) — 5 files, 629 lines, all compile
2. ✅ **Wire CLI** — cmd/ghx.go calls pkg/ghx/, 245 lines
3. ✅ **Go codemode package** — executor (92), registry (78), normalize (26), typegen (107) = 303 lines
4. 🔄 **MCP server** — `ghx serve` command, mark3labs/mcp-go (ADR-0008 §2)
5. 🔄 **Wire codemode** — register ghx core functions, expose via MCP

### Wave 6 results (4 agents, 4/4 ✅)

| Task | File | Lines | Result |
|------|------|-------|--------|
| TASK-0539 | `pkg/codemode/executor.go` | 92 → 231 | ✅ Full ADR-0008 §11 contract (ctx, ToolCallRecord, limits, IIFE, callTool) |
| TASK-0540 | `pkg/codemode/transpile.go` | 21 (new) | ✅ esbuild modern JS → ES2015 |
| TASK-0541 | `cmd/serve.go` | 183 (new) | ✅ `ghx serve` MCP server via mark3labs/mcp-go |
| TASK-0542 | `pkg/ghx/register.go` | 135 (new) | ✅ Wire 5 core functions to codemode registry |

**Cumulative: 20 agents, 18 delivered first try (90%)**

### Key insight: ADR-driven tasks outperform description-only tasks

Wave 6 was 100% success (4/4) vs wave 4b's 50% (2/4 codemode stubs). The difference: wave 6 tasks referenced ADR-0008's interface contracts with exact type signatures. Wave 4b tasks described what to build in prose. Agents with concrete interface specs deliver complete implementations. Agents with prose descriptions sometimes stub.

### Build order: COMPLETE

1. ✅ **Extract core** (`pkg/ghx/`) — 5 files, 629 lines
2. ✅ **Wire CLI** — cmd/ghx.go, 245 lines
3. ✅ **Go codemode package** — executor (231), registry (78), normalize (26), typegen (107), transpile (21) = 463 lines
4. ✅ **MCP server** — cmd/serve.go, 183 lines, `ghx serve` command
5. ✅ **Wire codemode** — pkg/ghx/register.go, 135 lines

Total: 1736 lines of Go across 14 files. All compile. Tests pass.

### Integration gaps found during code review

After reading all wave 5-6 output, three integration issues were found and fixed manually:

1. **Transpile not wired** — executor.go had `Transpile()` available but never called it. Fixed: normalize → transpile → IIFE → execute pipeline.
2. **Dead method** — registry.go had `ToolFuncs()` returning `map[string]ToolFunc` but executor now takes `[]Tool`. Replaced with `Tools()`.
3. **MCP server bypasses codemode** — serve.go calls `pkg/ghx/` directly, doesn't expose codemode meta-tools (`search` + `execute`). This is the key differentiator from a standard MCP server. Needs wave 7 agent.

**Insight: agents build correct isolated components but don't wire cross-cutting concerns.** Each file was correct in isolation. The transpile→executor pipeline and the serve→codemode→executor chain required human integration review.

### Remaining work

All build order steps complete. Remaining work tracked in ADR-0010 (research directions) and ADR-0008 (implementation status).

### Wave 7 results (3 agents, 3/3 ✅)

| Task | File | What | Result |
|------|------|------|--------|
| TASK-0543 | `cmd/serve.go` | Codemode meta-tools (`code` + `search_tools`) in MCP server | ✅ |
| TASK-0544 | `pkg/codemode/executor_test.go` | 12 executor tests (happy path, timeout, ACL, console, IIFE) | ✅ |
| TASK-0545 | `cmd/serve.go` | `--http :8080` streamable HTTP transport flag | ✅ |

Also fixed manually: context interrupt (goroutine watches `ctx.Done()`, calls `vm.Interrupt()`).

### Wave 8 results (3 agents, 3/3 ✅)

| Task | File | What | Result |
|------|------|------|--------|
| TASK-0546 | `pkg/codemode/typegen.go` | `declare const codemode: { ... }` output format (ADR-0009) | ✅ |
| TASK-0547 | `pkg/codemode/executor.go` | `codemode` object injection (per-tool methods via closure) | ✅ |
| TASK-0548 | `pkg/ghx/read.go` | Batch file reads — 1 GraphQL call with aliases instead of N | ✅ |

### Wave 9 results (2 agents, 2/2 ✅)

| Task | File | What | Result |
|------|------|------|--------|
| TASK-0549 | `cmd/serve.go` | Rename to `code`/`search_tools`, type stubs in description, 24K truncation | ✅ |
| TASK-0550 | `pkg/codemode/normalize.go` | Handle named functions + bare expressions (not just fences) | ✅ |

### Wave 10 results (1 agent, 1/1 ✅)

| Task | File | What | Result |
|------|------|------|--------|
| TASK-0551 | `cmd/code.go` | `ghx code` CLI command — ADR-0010 CLI-first invariant | ✅ |

### P0/P1 bugs found and fixed (between waves)

| Bug | Severity | Fix | Commit |
|-----|----------|-----|--------|
| `url.QueryEscape` in read.go | P0 | Removed — GraphQL takes raw paths | `7680489` |
| Capitalized Go fields in JS | P0 | JSON roundtrip via `normalizeForJS()` + json tags on all structs | `167c690` |
| `LoaderJS` → `LoaderTS` | P1 | transpile.go fixed per ADR-0009 | `9bc5349` |
| `callTool` → `codemode.explore()` | P1 | Executor injects `codemode` object with per-tool methods | `9bc5349` |
| typegen output format | P1 | Now produces `declare const codemode: { ... }` | `9bc5349` |
| Batch file reads | P1 | read.go uses 1 GraphQL call with aliases instead of N | `9bc5349` |

### Cumulative swarm results

| Wave | Agents | Success | Key deliverables |
|------|--------|---------|-----------------|
| 4a (core) | 6 | 6/6 | pkg/ghx/ — 5 core functions |
| 4b (codemode) | 4 | 2/4 | normalize.go, typegen.go |
| 5 (respawn) | 3 | 3/3 | executor.go, registry.go, cmd/ghx.go |
| 6 (MCP+wiring) | 4 | 4/4 | serve.go, transpile.go, register.go |
| 7 (meta-tools) | 3 | 3/3 | codemode meta-tools, executor tests, HTTP transport |
| 8 (ADR-0009) | 3 | 3/3 | typegen, codemode object, batch reads |
| 9 (ADR-0010) | 2 | 2/2 | code/search_tools rename, normalize upgrade |
| 10 (CLI-first) | 1 | 1/1 | `ghx code` CLI command |
| **Total** | **26** | **24 (92%)** | **2,351 lines, 14 files, 13 commits** |
