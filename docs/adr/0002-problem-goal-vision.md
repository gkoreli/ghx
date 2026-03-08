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

`ghx` is the first tool in the `gg` family — a collection of focused, independently installable CLI tools for AI-assisted development. Each tool does one thing well.

`ghx` = GitHub code exploration. ~135 lines of bash. One dependency: `gh` CLI.

### Design Principles

1. **Zero friction.** No npm install, no Docker, no config files. If you have `gh` CLI, you have `ghx`.

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

`ghx` occupies the empty niche: lightweight, agent-native, batch-capable remote code exploration.

### The `gg` Family

`ghx` is scoped to GitHub code exploration. Future tools in the family are separate repos, separate installs, shared brand:

```
ghx    — GitHub code exploration (this tool)
gg???     — future tools, each focused, each independent
```

The `gg` prefix is the platform. Each tool is a member. They don't depend on each other.

### Distribution Strategy

The tool is ~135 lines of bash. Distribution channels are thin wrappers around the same script:

| Channel | Install command | How it works |
|---------|----------------|-------------|
| Homebrew | `brew install gkoreli/tap/ghx` | Tap with formula pointing to release tarball |
| gh extension | `gh extension install gkoreli/gh-ghx` | Repo with bash script as entry point |
| npx | `npx @gkoreli/ghx` | npm package wrapping the bash script |
| curl | `curl -sf https://... \| sh` | Downloads script to ~/.local/bin |
| Manual | Copy `ghx` to PATH | Just the script |

All channels deliver the same bash script. The only runtime dependency is `gh` CLI (authenticated).

### gh Extension Constraint

`gh extension install` **requires** the repo name to start with `gh-`. The executable at root must match the repo name. This is a hard constraint — verified: `gh extension install` returns "extension name must start with `gh-`" for non-prefixed repos.

Bash-based gh extensions are fully supported and used by gh core maintainers (e.g., `mislav/gh-branch` — pure bash, 2.5KB). No Go required.

**Decision: separate `gh-ghx` repo.**
- `gkoreli/ghx` — the source of truth. Script, npm, curl, brew, docs, ADRs.
- `gkoreli/gh-ghx` — gh extension shim. Contains a `gh-ghx` executable that is an exact copy of the `ghx` script. The release workflow in `ghx` copies the script to `gh-ghx` and pushes a tagged release there, keeping them in sync automatically.

This keeps the main repo name clean (`ghx`), npm package name clean (`@gkoreli/ghx`), and satisfies gh's hard `gh-` prefix requirement without polluting the primary repo.

### Go Rewrite

Parked. Decision: ship bash today, revisit Go when triggers hit.

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

**The contract that survives a rewrite:** The CLI interface is the contract. `ghx explore owner/repo`, `ghx read owner/repo --map file`, `ghx search "query"`. Whether bash or Go executes underneath is invisible to users.

### One Source of Truth

**The script IS the source of truth.** Every distribution channel wraps the same code:
- npm: `package.json` bin points to the script
- gh extension: exact copy in `gh-ghx` repo, synced via CI
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

## Issues Found (v0.1 Self-Audit)

**Date**: 2026-03-08

After benchmarking ghx against `gh search code` and re-reading ADR-0001's landscape research, we found issues in the current implementation. Each issue is traced back to the research that identified the gap.

### Issue 1: Search discards matching lines (CRITICAL)

**What's wrong:** `ghx search` outputs only file paths: `repo path:name`. The REST `/search/code` API returns `text_matches` with matching code fragments when you send `Accept: application/vnd.github.text-match+json`. We don't use that header and our jq only extracts path/name.

**Why it matters:** `gh search code` shows `repo:path: matching line content` — our tool returns LESS information than the tool we claim to improve on. An agent searching for "bar width" gets file paths but no context about which lines matched or why. The agent must then do a follow-up `ghx read --grep` call for every file — defeating the purpose.

**Benchmark evidence (bench/RESULTS.md):**
- `ghx search`: 360 tokens, 20 lines (paths only)
- `gh search code`: 951 tokens, 36 lines (paths + matching lines)

ghx uses 62% fewer tokens but provides 0% of the matching context. The token savings are meaningless if the agent needs a follow-up call for every result.

**Fix:** Add `Accept: application/vnd.github.text-match+json` header. Default output: compact (path + first match fragment per file). `--verbose` flag: all fragments with context. This gives agents the matching lines they need while keeping output concise.

**Source:** ADR-0001 documents that Octocode's `matchString` parameter returns matching lines with configurable context — the same pattern we should follow.

### Issue 2: `gh search code` is unreliable, not "broken" or "working"

**What's wrong:** ADR-0001 and SKILL.md originally stated: "gh search code silently fails on multi-word queries." Initial re-testing on 2026-03-08 showed it working for `"bar width repo:plausible/analytics"`. But further testing revealed inconsistency: `gh search code "ghx gkoreli"` returns empty results for content that exists, while `gh search code "ghx"` returns noisy, irrelevant results.

**Revised assessment:** `gh search code` is unreliable — it works for some queries and fails silently for others. It's not categorically broken, but it's not trustworthy either. The original claim was too absolute ("silently fails on multi-word"), the correction was too generous ("works fine"). The truth is in between: it's flaky.

**Fix:** Remove absolute claims in either direction. State the observed behavior: `gh search code` works for some multi-word queries but returns inconsistent results. `ghx search` hits the REST API directly and provides consistent, compact output. Agents should prefer `ghx search` for reliability, not because `gh search code` is broken.

### Issue 3: No search query validation

**What's wrong:** ADR-0001 idea #1 identifies the most dangerous failure mode: GitHub silently treats invalid qualifiers (`filename:`, `in:`, `type:`) as literal text. `ghx search` passes queries through without checking.

**Why it matters:** An agent searching `filename:llms.txt` gets results — files that contain the TEXT "filename:llms.txt" — not files named llms.txt. No error, plausible-looking wrong results. Hours of wasted investigation.

**Fix:** 5-line bash check before sending the query. Warn on known-invalid qualifiers, suggest the correct alternative (`filename:` → `path:`).

### Remaining gaps from ADR-0001 research (TODO)

Ideas identified in ADR-0001 that are not yet implemented, prioritized by impact:

| # | Idea | Effort | Impact | Status |
|---|------|--------|--------|--------|
| 1 | Search query validation | 5 lines | High — prevents silent wrong results | TODO |
| 2 | Hints in output | 3 lines/cmd | Medium — guides agent's next action | TODO |
| 4 | AGENTS.md/CLAUDE.md in explore | 1 GraphQL alias | Medium — instant agent context | TODO |
| 5 | Token estimation warnings | 3 lines | Medium — prevents context overflow | TODO |
| 7 | `gh api --cache` internally | 1 flag | Low — helps repeated reads | TODO |
| 6 | File filtering in tree | ~10 lines | Low — reduces noise on large repos | TODO |
