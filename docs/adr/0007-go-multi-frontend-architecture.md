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

## Build Order

1. **Extract core** — Pull logic from `v2/cmd/ghx.go` into `pkg/ghx/`. Structured input/output, no formatting.
2. **Wire CLI** — Refactor `cmd/ghx.go` to call core. Verify parity with bash version.
3. **MCP server** — Add `ghx serve` command. Expose core as MCP tools.
4. **Codemode** — Embed goja. Generate type stubs from core function signatures. Add `execute` tool.

Steps 1-2 are the foundation. Steps 3-4 are the new capabilities bash can't do.

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

## Scope: What We're NOT Building

- **Not a generic codemode SDK.** Cloudflare built `@cloudflare/codemode` as a framework for turning *any* tools into codemode targets. That's infrastructure. We're building a tool that does something useful (GitHub exploration) with codemode as one of its interfaces.
- **Not a framework.** The goja executor, type stubs, and code normalization are ghx-specific — hardcoded bindings for 5 functions, not auto-generated from arbitrary tool schemas.
- **Not a Go port of `@cloudflare/codemode`.** No `ToolProvider` interface, no `resolveProvider()`, no plugin system.
- **Extract later if needed.** If the executor/type-gen/normalization code gets copy-pasted into a second project, extract a Go codemode SDK then. Let the framework emerge from real usage, don't design it upfront.

The build order reflects this: core package → CLI → MCP → codemode. Each step ships value. The codemode layer is ghx-specific glue, not a reusable SDK.

## Open Questions

- Should codemode be a separate binary (`ghx-codemode`) or a subcommand (`ghx codemode`)?
- Should the MCP server support streamable HTTP transport or just stdio?
- Should type stubs be generated at build time or runtime?
- Is goja sufficient or will we need async/await (which would push toward embedding quickjs)?
