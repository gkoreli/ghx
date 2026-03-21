# ghx — GitHub Code Exploration for AI Agents

One command does what takes 3-5 API calls. Batch file reads, code maps, search — all via `gh` CLI.

## Install

```bash
cd v2 && go build -o ghx .
```

Requires: [gh CLI](https://cli.github.com/) authenticated (`gh auth login`).

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

Write JS programs that compose multiple operations in one round-trip:

```bash
ghx code 'var r = codemode.explore({repo: "vercel/next.js"}); return r.branch;'
ghx code --list                             # Show type stubs
```

All `codemode.*` calls are synchronous — no `await`. Full TypeScript type stubs with return types are injected into the sandbox.

## MCP Server

```bash
ghx serve                                   # stdio (for Claude, Cursor, etc.)
ghx serve --http :8080                       # HTTP transport
```

## Agent Integration

```bash
ghx skill                                   # CLI skill (for SKILL.md injection)
ghx skill --mcp                             # MCP skill
```

## Architecture

- `v2/pkg/ghx/` — core library (Explore, Read, Search, Repos, Tree)
- `v2/pkg/codemode/` — JS executor (goja sandbox, TypeScript transpilation, type generation)
- `v2/cmd/` — CLI frontend (cobra) + MCP server (mcp-go)
- [`docs/adr/`](docs/adr/) — architectural decisions

## v1 (bash)

The original bash implementation is in [`v1/`](v1/). Zero dependencies beyond `gh` and `jq` — useful if you just want a shell script you can drop anywhere without compiling Go.

```bash
cp v1/ghx /usr/local/bin/ghx
```

## License

MIT
