# ggcode

GitHub code exploration for agents and humans. One command does what takes 3-5 API calls with any other tool.

- **Explore** a repo (tree + README) in 1 API call
- **Read** 1-10 files in 1 API call via GraphQL batching
- **Map** code structure with ~92% token reduction
- **Search** code with full GitHub search syntax

~135 lines of bash. One dependency: `gh` CLI.

## Install

```bash
# Homebrew (macOS/Linux)
brew install gogakoreli/tap/ggcode

# gh extension (coming soon — requires separate gh-ggcode repo)
gh extension install gogakoreli/gh-ggcode

# npx (zero install)
npx ggcode --help

# curl
curl -sf https://raw.githubusercontent.com/gogakoreli/ggcode/main/install.sh | sh

# Manual — just copy the script
curl -sf https://raw.githubusercontent.com/gogakoreli/ggcode/main/ggcode -o ~/.local/bin/ggcode && chmod +x ~/.local/bin/ggcode
```

Requires [`gh` CLI](https://cli.github.com/) authenticated (`gh auth login`).

## Usage

```bash
# Explore a repo — branch, file tree, and README in 1 API call
ggcode explore plausible/analytics

# Read multiple files in 1 API call
ggcode read plausible/analytics mix.exs assets/js/dashboard/stats/bar.js

# Code map — signatures, imports, types only (~92% token reduction)
ggcode read plausible/analytics --map lib/plausible/stats/query.ex

# Grep within a remote file (2 lines context)
ggcode read plausible/analytics --grep "defmodule" lib/plausible/stats/query.ex

# Read specific line range
ggcode read plausible/analytics --lines 42-80 lib/plausible/stats/query.ex

# Search code (full GitHub search syntax)
ggcode search "useState repo:facebook/react"
ggcode search "path:llms.txt extension:txt"

# Full recursive tree
ggcode tree plausible/analytics assets/js
```

## Why

AI agents exploring GitHub repos face a tooling gap:

| Tool | Files per API call | Context overhead | Dependencies |
|------|-------------------|-----------------|-------------|
| GitHub MCP | 1 | ~10K tokens (50+ tool schemas) | Go binary |
| Octocode MCP | 1 (parallel) | ~10K tokens | npm + Docker |
| Raw `gh` CLI | 1 | 0 | `gh` |
| Gitingest | N (clones first) | 0 | pip + tiktoken |
| **ggcode** | **1-10 (GraphQL batch)** | **0** | **`gh`** |

`ggcode` reads 10 files in 1 API call. No other tool does this.

## Code Map (`--map`)

The `--map` flag extracts only structural declarations — imports, exports, function signatures, class definitions, type declarations. Implementation bodies are stripped.

Tested on 6 real files across TypeScript, Python, and Go:

| File | Full | Map | Reduction |
|------|------|-----|-----------|
| repomix/parseFile.ts | 5,599 | 812 | 86% |
| github-mcp/repositories.go | 68,862 | 1,551 | 97.7% |
| aider/repomap.py | 27,346 | 1,496 | 94.5% |

Average: **92% reduction**. An agent can map 16 files in the space of reading 1 file fully.

Supported: TypeScript/JavaScript, Python, Go, Rust, Java/Kotlin, Ruby. Falls back to generic pattern for unknown extensions.

## How It Works

`ggcode` wraps `gh` CLI with GraphQL batching. The `explore` command fetches tree + README in 1 GraphQL call. The `read` command uses GraphQL aliases (`f0:`, `f1:`, ...) to fetch up to 10 files in 1 call. The `search` command hits the REST `/search/code` endpoint directly (GraphQL has no code search).

The `--map` flag applies per-language regex patterns to extract structural declarations from the fetched content. No Tree-sitter, no AST parsing — just regex on the first line of each declaration. This works because structural declarations in most languages start at the beginning of a line with a keyword (`function`, `class`, `def`, `func`, `export`, `import`, etc.).

## License

MIT

## For AI Agents

See [`SKILL.md`](./SKILL.md) for agent-optimized instructions — chain of thought, gotchas, anti-patterns, and examples.
