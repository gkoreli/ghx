# ghx

GitHub code exploration for agents and humans. One command does what takes 3-5 API calls with any other tool.

- **Repos** — search repos with README preview in 1 GraphQL call
- **Explore** a repo (tree + README) in 1 API call
- **Read** 1-10 files in 1 API call via GraphQL batching
- **Map** code structure with ~92% token reduction
- **Search** code with AND matching, matching context, token protection

Bash script. One dependency: `gh` CLI. Cross-platform (macOS, Linux, Windows via Git Bash/WSL).

## Install

```bash
# npm (recommended)
npm install -g @gkoreli/ghx

# npx (zero install)
npx @gkoreli/ghx --help

# curl
curl -sf https://raw.githubusercontent.com/gkoreli/ghx/main/install.sh | sh
```

Requires [`gh` CLI](https://cli.github.com/) authenticated (`gh auth login`).

[![npm](https://img.shields.io/npm/v/@gkoreli/ghx)](https://www.npmjs.com/package/@gkoreli/ghx)

### Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| macOS | ✅ Native | bash + readlink -f (12.3+) |
| Linux | ✅ Native | bash + GNU coreutils |
| Windows | ✅ Git Bash / WSL | Ships with Git for Windows. Raw cmd.exe/PowerShell not supported |

If you have `gh` CLI working, ghx works too — same prerequisites.

## Usage

```bash
# Search repos — name, stars, language, README preview in 1 call
ghx repos "react state management"

# Explore a repo — branch, file tree, and README in 1 API call
ghx explore plausible/analytics

# Read multiple files in 1 API call
ghx read plausible/analytics mix.exs assets/js/dashboard/stats/bar.js

# Code map — signatures, imports, types only (~92% token reduction)
ghx read plausible/analytics --map lib/plausible/stats/query.ex

# Grep within a remote file (2 lines context)
ghx read plausible/analytics --grep "defmodule" lib/plausible/stats/query.ex

# Read specific line range
ghx read plausible/analytics --lines 42-80 lib/plausible/stats/query.ex

# Search code (AND matching, shows matching lines, token-protected)
ghx search "useState repo:facebook/react"
ghx search "path:llms.txt extension:txt"

# Full recursive tree
ghx tree plausible/analytics assets/js
```

## Why

AI agents exploring GitHub face a reliability gap: *"Did I find nothing because nothing exists, or because I used the tool wrong?"* ghx eliminates this with smart defaults — AND matching instead of exact phrase, README previews instead of bare names, matching context instead of bare paths. The right behavior is the default behavior.

| Tool | Files per call | Matching context | Smart defaults | Dependencies |
|------|---------------|-----------------|---------------|-------------|
| GitHub MCP | 1 | No | No (~10K token schemas) | Go binary |
| `gh` CLI | 1 | No | No (exact phrase, base64, no README) | `gh` |
| **ghx** | **1-10 (batch)** | **Yes** | **Yes** | **`gh`** |

## Agent Skill Integration

`ghx skill` outputs the full [`SKILL.md`](./SKILL.md) to stdout — designed for agent context injection via spawn hooks:

```json
{
  "hooks": {
    "agentSpawn": [
      {"command": "ghx skill"}
    ]
  }
}
```

Every agent session gets the latest ghx skill (commands, gotchas, best practices, search strategy) injected into context automatically. No manual copy/paste, always in sync with the installed version.

SKILL.md is included in the npm package and resolved via symlink, so this works with all installation methods.

## Code Map (`--map`)

The `--map` flag extracts only structural declarations — imports, exports, function signatures, class definitions, type declarations. Implementation bodies are stripped.

| File | Full | Map | Reduction |
|------|------|-----|-----------|
| repomix/parseFile.ts | 5,599 | 812 | 86% |
| github-mcp/repositories.go | 68,862 | 1,551 | 97.7% |
| aider/repomap.py | 27,346 | 1,496 | 94.5% |

Average: **92% reduction**. An agent can map 16 files in the space of reading 1 file fully.

Supported: TypeScript/JavaScript, Python, Go, Rust, Java/Kotlin, Ruby. Generic fallback for unknown extensions.

## How It Works

`ghx` wraps `gh` CLI with GraphQL batching. `repos` and `explore` use GraphQL to batch search + metadata + README into 1 call. `read` uses GraphQL aliases to fetch up to 10 files in 1 call. `search` hits the REST `/search/code` endpoint with `text_matches` for matching context and 200-char token protection.

## License

MIT
