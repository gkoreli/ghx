# ghx — GitHub Code Exploration for AI Agents

One command does what takes 3-5 API calls. Batch file reads, code maps, search — all via `gh` CLI. Write JS programs that compose operations in one round-trip.

## Why

AI agents exploring GitHub face a reliability gap: *"Did I find nothing because nothing exists, or because I used the tool wrong?"* Raw `gh` commands have silent failure modes — `gh search code` wraps in quotes without telling you, `gh api contents/` returns base64, README requires a separate call. The agent can't distinguish "no results" from "wrong flags."

ghx eliminates this by encoding the right defaults into every command. One call returns enough context to decide the next action.

| Tool | Files per call | Matching context | Programmable | Dependencies |
|------|---------------|-----------------|-------------|-------------|
| GitHub MCP | 1 | No | No (~10K token schemas) | Go binary |
| `gh` CLI | 1 | No | No (exact phrase, base64, no README) | `gh` |
| **ghx** | **1-10 (batch)** | **Yes** | **Yes (codemode)** | **`gh`** |

## Install

```bash
# Zero install — just run it
npx @gkoreli/ghx explore vercel/next.js

# Homebrew
brew install gkoreli/tap/ghx

# npm (global)
npm install -g @gkoreli/ghx

# Go
go install github.com/gkoreli/ghx/v2@latest

# Build from source
cd v2 && go build -o ghx .
```

Requires: [gh CLI](https://cli.github.com/) authenticated (`gh auth login`).

### MCP Config (Claude Desktop, Cursor)

```json
{
  "mcpServers": {
    "ghx": {
      "command": "npx",
      "args": ["@gkoreli/ghx", "serve"]
    }
  }
}
```

No install step — npx downloads and caches the binary on first run.

## Commands

```bash
ghx explore <owner/repo>                    # Branch + tree + README in 1 API call
ghx explore <owner/repo> <path>             # Subdirectory listing
ghx read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files (GraphQL batching)
ghx search "<query>"                        # Code search with matching lines
ghx repos "<query>"                         # Repo search with README preview
ghx tree <owner/repo> [path]                # Full recursive tree
```

## Codemode

Write JS programs that compose multiple operations in one round-trip. All `codemode.*` calls are synchronous — no `await`. Full TypeScript type stubs with return types are injected into the sandbox.

```bash
# What branch is this repo on?
ghx code 'var r = codemode.explore({repo: "vercel/next.js"}); return r.branch;'

# Search + read composition
ghx code 'var hits = codemode.search({query: "useState repo:vercel/next.js", limit: 3});
var first = codemode.read({repo: "vercel/next.js", files: [hits.matches[0].path]});
return {file: hits.matches[0].path, lines: first[0].content.split("\n").length};'

# See what tools and types are available
ghx code --list
```

Type stubs tell the LLM exactly what fields exist — no guessing:

```typescript
declare const codemode: {
  explore: (input: ExploreInput) => { description: string; branch: string; files: { name: string; type: string }[]; readme: string };
  search: (input: SearchInput) => { total: number; incomplete: boolean; matches: { repo: string; path: string; fragment: string }[] };
  tree: (input: TreeInput) => string[];
  // ...
}
```

## MCP Server

```bash
ghx serve                                   # stdio (for Claude, Cursor, etc.)
ghx serve --http :8080                       # HTTP transport
```

7 tools: `explore`, `read`, `search`, `repos`, `tree`, `code` (meta-tool), `search_tools`.

## Agent Integration

```bash
ghx skill                                   # CLI skill (for SKILL.md injection)
ghx skill --mcp                             # MCP skill
```

Designed for eager context injection via spawn hooks — the agent always has the latest ghx knowledge without loading it mid-conversation.

## How It Works

Wraps `gh` CLI with GraphQL batching. `repos` and `explore` batch search + metadata + README into 1 call. `read` uses GraphQL aliases to fetch up to 10 files in 1 call. `search` hits REST `/search/code` with `text_matches` for matching context and 200-char token protection.

Codemode runs JS in a [goja](https://github.com/nicholasgasior/goja) sandbox with esbuild TypeScript transpilation. Tools are injected as synchronous functions on a `codemode` global object. Max 20 tool calls per execution, 64KB code size limit.

## Architecture

```
v2/
├── pkg/ghx/       — core library (Explore, Read, Search, Repos, Tree)
├── pkg/codemode/  — JS executor (goja sandbox, TS transpilation, type generation)
└── cmd/           — CLI frontend (cobra) + MCP server (mcp-go)
```

See [docs/adr/](docs/adr/) for architectural decisions.

## v1 (bash)

The original bash implementation is in [`v1/`](v1/). Zero dependencies beyond `gh` and `jq` — useful if you just want a shell script you can drop anywhere without compiling Go.

```bash
npm install -g @gkoreli/ghx    # npm
cp v1/ghx /usr/local/bin/ghx   # manual
```

## License

MIT
