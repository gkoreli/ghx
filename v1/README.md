# ghx v1 — Bash Implementation

> For the current Go version with codemode, MCP server, and typed stubs, see the [root README](../README.md).

The bash implementation — zero dependencies beyond `gh` CLI. Useful if you want a single shell script you can drop anywhere without compiling Go.

## Install

```bash
# npm
npm install -g @gkoreli/ghx

# npx (zero install)
npx @gkoreli/ghx --help

# curl
curl -sf https://raw.githubusercontent.com/gkoreli/ghx/mainline/v1/install.sh | sh

# manual
cp v1/ghx /usr/local/bin/ghx
```

Requires [`gh` CLI](https://cli.github.com/) authenticated (`gh auth login`).

### Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| macOS | ✅ Native | bash + readlink -f (12.3+) |
| Linux | ✅ Native | bash + GNU coreutils |
| Windows | ✅ Git Bash / WSL | Ships with Git for Windows |

## Usage

```bash
ghx repos "react state management"              # Search repos with README preview
ghx explore plausible/analytics                  # Branch + tree + README in 1 API call
ghx read plausible/analytics mix.exs assets/js/dashboard/stats/bar.js  # Batch read
ghx read plausible/analytics --map lib/plausible/stats/query.ex        # Code map (~92% reduction)
ghx read plausible/analytics --grep "defmodule" lib/plausible/stats/query.ex
ghx read plausible/analytics --lines 42-80 lib/plausible/stats/query.ex
ghx search "useState repo:facebook/react"        # Code search with matching lines
ghx tree plausible/analytics assets/js           # Full recursive tree
```

## Code Map (`--map`)

Extracts only structural declarations — imports, exports, function signatures, class definitions, type declarations. Implementation bodies are stripped.

| File | Full | Map | Reduction |
|------|------|-----|-----------|
| repomix/parseFile.ts | 5,599 | 812 | 86% |
| github-mcp/repositories.go | 68,862 | 1,551 | 97.7% |
| aider/repomap.py | 27,346 | 1,496 | 94.5% |

Average: **92% reduction**. Map 16 files in the space of reading 1 file fully.

Supported: TypeScript/JavaScript, Python, Go, Rust, Java/Kotlin, Ruby. Generic fallback for unknown extensions.

## How It Works

Wraps `gh` CLI with GraphQL batching. `repos` and `explore` batch search + metadata + README into 1 call. `read` uses GraphQL aliases to fetch up to 10 files in 1 call. `search` hits REST `/search/code` with `text_matches` for matching context and 200-char token protection.

## Agent Skill

```bash
ghx skill    # outputs SKILL.md to stdout
```

See [`SKILL.md`](./SKILL.md) for agent-optimized instructions.

## License

MIT
