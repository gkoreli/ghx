# 0002. Problem Statement, Goal, and Vision

**Date**: 2026-03-07
**Status**: Accepted

## Problem

AI agents exploring GitHub repositories face a tooling gap:

1. **MCP servers are too heavy.** GitHub MCP injects ~10K tokens of tool schemas (50+ tools) into every conversation. Octocode MCP requires npm + Docker + a local HTTP server. Agents pay the context cost whether they use the tools or not.

2. **Raw `gh` CLI is too low-level.** Exploring a repo requires 3-5 sequential calls: get default branch, get tree, get README, read files one at a time. Each call is a separate tool invocation, each burns context on the request/response cycle.

3. **Clone-first tools are too slow.** Gitingest (3.6s sparse clone) and Repomix (full clone + Tree-sitter parse) require downloading repo content to disk. For quick exploration across multiple repos, this is unacceptable overhead.

4. **No tool does multi-file batch reads.** Every tool in the landscape reads one file per API call. GraphQL's alias mechanism allows N files in 1 HTTP request — nobody uses it.

5. **No tool generates code maps from remote repos.** Aider's repo map and codemap require local files + Tree-sitter. There's no way to get a structural overview of a remote repo without cloning it first.

The result: agents waste context on tool schemas, make redundant API calls, dump entire files when they need 10 lines, and can't efficiently scan unfamiliar codebases.

## Goal

A single CLI tool that gives AI agents (and humans) efficient access to GitHub repository content:

- **Explore** a repo (tree + README) in 1 API call
- **Read** 1-10 files in 1 API call via GraphQL batching
- **Map** code structure (signatures, imports, types) with ~92% token reduction
- **Search** code with full GitHub search syntax
- **Zero context overhead** — no MCP schemas, no tool registration, just stdout

## Vision

`ggcode` is the first tool in the `gg` family — a collection of focused, independently installable CLI tools for AI-assisted development. Each tool does one thing well.

`ggcode` = GitHub code exploration. ~135 lines of bash. One dependency: `gh` CLI.

### Design Principles

1. **Zero friction.** No npm install, no Docker, no config files. If you have `gh` CLI, you have `ggcode`.

2. **API-first, not clone-first.** 0.9s GraphQL call vs 3.6s sparse clone. No disk usage, no cleanup.

3. **Progressive disclosure.** explore → map → grep → full read. Each step adds detail only when needed. Mirrors how developers actually work: scan structure, identify interesting files, read specific sections.

4. **Fail loudly.** GitHub's API silently degrades invalid search qualifiers to literal text. Agent-friendly tools must validate inputs and surface errors explicitly.

5. **Minimal code.** ~135 lines of bash. The entire tool fits on one screen. Less code = fewer bugs, easier to audit, easier to contribute.

### Architectural Decisions

| Decision | Choice | Why | Trade-off |
|----------|--------|-----|-----------|
| Language | Bash | Zero install, composable with pipes, matches GitHub's own pattern (gh-aw) | No Tree-sitter, no SQLite, no cross-file refs |
| API strategy | GraphQL batching | N files in 1 HTTP request, tree+README in 1 call | No code search in GraphQL (REST fallback) |
| Code mapping | Regex signatures | Zero deps, 92% reduction, works on remote content | Misses multi-line signatures |
| Token estimation | chars/4 | Industry standard (codemap, aider use same formula) | Approximation, not exact |
| Distribution | Multi-channel | brew, gh extension, npx, curl — same bash script everywhere | Thin wrappers needed per channel |

### Competitive Position

No direct competitor exists. The landscape has:
- **Search tools** (johnlindquist/ghx, gh-scout) — search across repos, not explore within one
- **TUI browsers** (gh-repo-explore, gh-xplr) — interactive for humans, not agents
- **Heavy MCPs** (GitHub MCP, Octocode) — rich features, 10K+ token overhead
- **Dump tools** (Repomix, Gitingest) — holistic understanding, not surgical exploration

`ggcode` occupies the empty niche: lightweight, agent-native, batch-capable remote code exploration.

### The `gg` Family

`ggcode` is scoped to GitHub code exploration. Future tools in the family are separate repos, separate installs, shared brand:

```
ggcode    — GitHub code exploration (this tool)
gg???     — future tools, each focused, each independent
```

The `gg` prefix is the platform. Each tool is a member. They don't depend on each other.

### Distribution Strategy

The tool is ~135 lines of bash. Distribution channels are thin wrappers around the same script:

| Channel | Install command | How it works |
|---------|----------------|-------------|
| Homebrew | `brew install gogakoreli/tap/ggcode` | Tap with formula pointing to release tarball |
| gh extension | `gh extension install gogakoreli/gh-ggcode` | Repo with bash script as entry point |
| npx | `npx ggcode` | npm package wrapping the bash script |
| curl | `curl -sf https://... \| sh` | Downloads script to ~/.local/bin |
| Manual | Copy `ggcode` to PATH | Just the script |

All channels deliver the same bash script. The only runtime dependency is `gh` CLI (authenticated).

### gh Extension Constraint

`gh extension install` **requires** the repo name to start with `gh-`. The executable at root must match the repo name. This is a hard constraint — verified: `gh extension install` returns "extension name must start with `gh-`" for non-prefixed repos.

Bash-based gh extensions are fully supported and used by gh core maintainers (e.g., `mislav/gh-branch` — pure bash, 2.5KB). No Go required.

**Solution: two entry points, one source of truth.**
- Main repo: `ggcode` — the script, npm package, curl install, brew, docs, ADRs
- gh extension repo: `gh-ggcode` — contains a single `gh-ggcode` file that's either a copy or a thin wrapper that downloads/runs the real script

Alternatively, the main repo could be named `gh-ggcode` with both `gh-ggcode` (the script) and a `package.json` that maps `bin.ggcode → ./gh-ggcode`. This makes gh extension the primary distribution and npm/brew/curl all work from the same repo. Trade-off: the script filename is `gh-ggcode` instead of `ggcode`.

### Go Rewrite: When and Why

**Today: bash is correct.** The tool is 135 lines. Rewriting in Go would be 500-1000 lines for the same functionality. The `gh` CLI handles auth, API calls, and JSON parsing — bash just orchestrates.

**When Go becomes correct:**
1. When `gh` CLI dependency becomes a friction point (users who don't have `gh` installed)
2. When Windows support matters (bash doesn't work natively on Windows)
3. When performance matters (Go binary vs spawning `gh` subprocess per call)
4. When the tool grows beyond what bash handles cleanly (~500+ lines)

**What Go gives you:**
- Single binary, zero runtime dependencies (embeds GitHub API + OAuth)
- Native gh extension support (precompiled binary in GitHub releases, gh auto-downloads correct platform)
- Cross-platform: darwin-arm64, darwin-amd64, linux-arm64, linux-amd64, windows-amd64
- npm distribution via platform-specific binaries (same pattern as esbuild, turbo, biome)
- Faster execution (no subprocess spawning)

**What Go costs:**
- 5-10x more code for same functionality
- Build matrix CI (goreleaser handles this)
- Auth implementation (can use `go-gh` library from GitHub which reads `gh` auth config)
- Ongoing maintenance of a compiled codebase vs a script

**The contract that survives a rewrite:** The CLI interface is the contract. `ggcode explore owner/repo`, `ggcode read owner/repo --map file`, `ggcode search "query"`. Whether bash or Go executes underneath is invisible to users. Ship bash today, rewrite in Go when one of the triggers above is hit.

### One Source of Truth: The Decision

**The script IS the source of truth.** Every distribution channel wraps the same code:
- npm: `package.json` bin points to the script
- gh extension: script at repo root (named `gh-ggcode`)
- brew: formula downloads the script from a GitHub release
- curl: install.sh downloads the script
- Manual: copy the script

If/when Go replaces bash, the same structure holds — just swap the script for a binary. The distribution wrappers don't change.

## Evidence

All claims backed by source code analysis documented in [ADR-0001](./0001-landscape-research.md):
- 92% average reduction: tested on 6 real files across TypeScript, Python, Go
- 0.9s vs 3.6s: measured GraphQL vs sparse clone on same repo
- 10K token overhead: counted from GitHub MCP's 50+ tool schemas
- GraphQL batching: verified via alias mechanism (`f0:`, `f1:`, etc.)
- No competitor: searched gh extension registry, npm, GitHub repos — verified empty niche
