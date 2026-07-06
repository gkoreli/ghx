# ghx — GitHub Code Exploration for AI Agents

One command does what takes 3-5 API calls. Batch file reads, code maps, search — all via `gh` CLI. Write JS programs that compose operations in one round-trip.

## Why

An agent wants to understand `packages/shadcn/src/utils/` in [shadcn-ui/ui](https://github.com/shadcn-ui/ui):

**With `gh` CLI** — 4 turns, 4 API calls, reads 3 full files (3,761 tokens), sees 3 of 34 files:
```
gh api /repos/shadcn-ui/ui/contents/packages/shadcn/src/utils       → JSON with shas, urls, links (480 tokens for a file list)
gh api /repos/.../get-config.ts --jq '.content' | base64 -d         → full file (1,981 tokens — agent only needed exports)
gh api /repos/.../registries.ts --jq '.content' | base64 -d         → full file (676 tokens)
gh api /repos/.../frameworks.ts --jq '.content' | base64 -d         → full file (624 tokens)
```

**With ghx** — 2 turns, 2 API calls, maps 10 files (3,058 tokens), sees signatures of all 10:
```
ghx read shadcn-ui/ui packages/shadcn/src/utils                     → directory listing (199 tokens)
ghx read shadcn-ui/ui "packages/shadcn/src/utils/*.ts" --map        → signatures of 10 files (2,859 tokens)
```

Same token budget. The `gh` agent read 3 full files. The ghx agent saw the structure of 10 — imports, exports, function signatures — and knows which ones to drill into. Pass a file, get content. Pass a directory, get a listing. Pass a glob, get matching files. Same command, always useful output.

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
go install github.com/gkoreli/ghx/v2/cmd/ghx@latest

# Build from source
go build -o ghx ./cmd/ghx
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
ghx read <owner/repo> <dir>                 # Directory path → returns file listing
ghx read <owner/repo> "src/**/*.ts" --map   # Glob patterns with structural map
ghx read <owner/repo> --map <f1> [f2]       # Parser-backed structural map (~92% token reduction)
ghx read <owner/repo> --map --kind func <f> # Map only functions/methods
ghx read <owner/repo> --map --kind type <f> # Map only types/structs/interfaces
ghx read <owner/repo> --map --level minimal <f> # Symbol names only (e.g. UserService.GetUser)
ghx read <owner/repo> --grep "pat" <f>      # Matching lines only (ERE regex, 2 lines context)
ghx read <owner/repo> --lines 42-80 <f>     # Specific line range
ghx search "<query>"                        # Code search with matching lines
ghx repos "<query>"                         # Repo search with README preview
ghx tree <owner/repo> [path]                # Full recursive tree
ghx tree <owner/repo> [path] --depth N      # Tree limited to N levels
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

## Sidecar

`ghx sidecar` runs a specialist reconnaissance agent that turns English repo
questions into bounded ghx evidence exploration and returns compact, auditable
reports. Every session persists its artifacts — reports plus spec-exact OTel
traces/logs/metrics — under `~/.ghx/sessions/`. First-time setup (after
installing ghx, see Install above):

```bash
# 1. Write the agent config — pins the Claude Code ACP adapter, run via npx
#    (needs Node and a Claude Code login; no wrapper script to hand-write)
ghx sidecar config init --claude-acp

# 2. Verify the setup end to end
ghx sidecar doctor

# 3. Ask
ghx sidecar ask --repo vercel/next.js "how does the app router cache work?"
```

`config init --claude-acp` writes `~/.ghx/config.json` (or `$GHX_HOME/config.json`)
with the pinned `npx @agentclientprotocol/claude-agent-acp` command. Re-running
it against an existing config shows a field diff and refuses to overwrite
without `--force`. Plain `ghx sidecar config init` still auto-detects agents
already speaking ACP on your PATH.

`ghx sidecar doctor` checks your GitHub token, network, the PATH ghx binary,
the configured agent's ACP handshake, and that the binary serving the
report-sink MCP server matches the running ghx version — a stale sink binary
would silently degrade structured reports to a text fallback, so doctor fails
loudly with fix-it text instead.

Sessions persist across invocations, and every session's artifact trail is
browsable:

```bash
ghx sidecar sessions list                          # list named sessions
ghx sidecar sessions show <session>                # metadata + report history
ghx sidecar view [session]                         # browse a session's OTel artifacts in a UI
ghx sidecar view --list                            # sessions with turn/report counts
ghx sidecar view --port 9000                       # viewer UI port (default 8000)
```

`ghx sidecar view` spawns
[otel-desktop-viewer](https://github.com/CtrlSpice/otel-desktop-viewer) and
replays the session's traces (plus logs/metrics when present) into it — one
command instead of a curl loop. No argument means the most recent session.
Requires the viewer on PATH:
`go install github.com/CtrlSpice/otel-desktop-viewer@latest`.

## Agent Integration

```bash
ghx skill                                   # CLI skill (for SKILL.md injection)
ghx skill --mcp                             # MCP skill
```

Install the skills into Claude Code with the skills CLI:

```bash
npx skills add gkoreli/ghx -g -a claude-code --skill ghx -y
npx skills add gkoreli/ghx -g -a claude-code --skill ghx-mcp -y
```

Designed for eager context injection via spawn hooks — the agent always has the latest ghx knowledge without loading it mid-conversation.

## How It Was Built

23 agent sessions, 2,500+ conversation turns, 3 rewrites, 12 ADRs. The full story: **[Build the GitHub Exploration Tool, No Mistakes](https://gkoreli.com/how-ghx-was-born)**

## How It Works

Wraps `gh` CLI with GraphQL batching. `repos` and `explore` batch search + metadata + README into 1 call. `read` uses GraphQL aliases to fetch up to 10 files in 1 call — and if a path is a directory, returns its file listing instead of "not found" (via `... on Tree` inline fragments in the same query, zero extra API calls). Glob patterns (`src/**/*.ts`) auto-expand via tree fetch + [doublestar](https://github.com/bmatcuk/doublestar) matching in 2 API calls. `--grep` uses ERE regex with BRE normalization (agents trained on `grep` write `\|` for alternation — both styles work). `search` hits REST `/search/code` with `text_matches` for matching context and 200-char token protection.

`--map` runs a dedicated parser engine on the fetched content — no extra API calls. Engine selection is automatic: **Go** uses `go/ast` (top-level declarations only, full multi-line signatures, generics preserved), **TypeScript, JavaScript, Python, Rust** use [gotreesitter](https://github.com/odvcencio/gotreesitter) (captures class/impl methods that regex cannot reach), everything else falls back to regex. Methods carry a parent reference (`UserService.GetUser`) visible at `--level minimal`. `--map-engine regex` forces the fallback for any file.

Codemode runs JS in a [goja](https://github.com/nicholasgasior/goja) sandbox with esbuild TypeScript transpilation. Tools are injected as synchronous functions on a `codemode` global object. Max 20 tool calls per execution, 64KB code size limit.

## Architecture

```
cmd/ghx/             — binary entrypoint
internal/cli/        — CLI frontend (cobra) + MCP server commands
internal/ghx/        — core library (Explore, Read, Search, Repos, Tree, Glob)
internal/codemode/   — JS executor (goja sandbox, TS transpilation, type generation)
internal/mapengine/  — parser-backed map engine (GoAST, TreeSitter, Regex, engine routing)
internal/sidecar/    — sidecar runtime, sessions, reports, and ACP integration
skills/             — CLI and MCP agent skills, embedded into binary via go:embed
```

See [docs/adr/](docs/adr/) for architectural decisions.

## Built on Open Source, Openly

ghx exists because exploring open source for ideas is where good products
come from — "good artists copy, great artists steal," and we steal in the
Picasso sense: openly, with attribution, and with stewardship for future
generations. The rule we build by: **use official standards and existing
open-source tools, libraries, and ideas first; hand-roll a framework or
format only when nothing existing serves the need or the vision.** Agent
traces are official OTel (OTLP/JSON) so any industry tool — Jaeger,
[otel-desktop-viewer](https://github.com/CtrlSpice/otel-desktop-viewer),
the collector — consumes them in seconds; parsing rides on
[gotreesitter](https://github.com/odvcencio/gotreesitter) and `go/ast`;
globbing on [doublestar](https://github.com/bmatcuk/doublestar); codemode on
[goja](https://github.com/nicholasgasior/goja). Where we do build new —
ghx itself, the Agent Sidecar Framework — it is because the thing did not
exist, and it is inspired loudly by what does (tools like
[codemap](https://github.com/JordanCoin/codemap) shape where the sidecar's
internal toolbox goes next). MIT in, MIT out.

## License

MIT
