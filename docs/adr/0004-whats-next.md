# ADR-0004: What's Next for ghx

**Date**: 2026-03-08
**Status**: Proposed

## Current State (v0.2.0)

Six commands, all following the agentic-first philosophy (ADR-0003):

| Command | What it does | API calls |
|---|---|---|
| `ghx repos` | Search repos + stars + language + README preview | 1 GraphQL |
| `ghx explore` | Description + tree + README | 1 GraphQL |
| `ghx read` | 1-10 files, --grep, --lines, --map | 1 GraphQL |
| `ghx search` | Code search, AND matching, text_matches, token protection | 1 REST |
| `ghx tree` | Recursive file listing | 1 REST |
| `ghx skill` | Output SKILL.md for agent context injection | 0 (local) |

All ADR-0003 decisions implemented. Benchmark complete. Philosophy documented.

## Proposed Enhancements (ranked by impact)

### Tier 1: High impact, low effort (bash, no new deps)

**1. AGENTS.md / CLAUDE.md detection in `explore`**
Add these files to the GraphQL query alongside README.md — zero extra API calls (just more aliases). Repos increasingly ship agent instructions. `ghx explore` should surface them automatically.
```graphql
agents: object(expression: "$branch:AGENTS.md") { ... on Blob { text } }
claude: object(expression: "$branch:CLAUDE.md") { ... on Blob { text } }
```
Effort: 2 GraphQL aliases + 2 jq lines.

**2. Hints in output**
After search: `"Hint: ghx read owner/repo path to view a file"`. After large file read: `"Hint: use --grep to narrow"`. After explore: `"Hint: ghx read --map for code structure"`. Guides agent's next action. Inspired by Octocode's hints system (ADR-0001).
Effort: 1 echo per command.

**3. Token estimation warnings**
After reading files, estimate tokens (`wc -c / 4`). Warn on stderr if >30K tokens: `"⚠ Large output (~45K tokens). Consider --grep or --lines."` Prevents context overflow.
Effort: 3 lines per command.

**4. `ghx read --minify`**
Strip single-line comments and collapse blank lines. `sed '/^[[:space:]]*\/\//d; /^[[:space:]]*#[^!]/d; /^$/N;/^\n$/d'`. Saves 20-30% tokens on verbose codebases. Inspired by Octocode's content minification (ADR-0001).
Effort: ~5 lines.

### Tier 2: Medium impact, medium effort

**5. File filtering in `tree`**
Strip known noise: node_modules, .lock files, dist/, build/, __pycache__, .min.js, images. Default on, `--all` to disable. Reduces tree clutter on large repos.
Effort: ~15 lines (exclusion list + grep -v).

**6. `ghx diff <owner/repo> <pr-number>`**
PR diff with smart truncation — show changed files summary + truncated diff. Currently requires `gh pr diff` which dumps the entire diff. ghx could add token protection (truncate large hunks, show file-level summary first).
Effort: ~30 lines.

**7. `ghx clone <owner/repo> [path]`**
Sparse checkout for deep local analysis: `git clone --depth=1 --filter=blob:none --sparse`. Then local tools (ripgrep, Tree-sitter) can analyze without API rate limits. Auto-cleanup on exit.
Effort: ~20 lines.

### Tier 3: Larger scope / future consideration

**8. Go rewrite**
When bash hits its limits (~500+ lines, need for true cross-platform, performance). See ADR-0002 for triggers and trade-offs. The CLI interface is the contract — `ghx explore`, `ghx read`, `ghx search` — whether bash or Go underneath is invisible.

**9. gh extension distribution**
Publish as `gh extension install gkoreli/gh-ghx`. Requires separate `gh-ghx` repo with the script. See ADR-0002 for the shim strategy.

**10. Progressive detail reduction for `--map`**
Inspired by codemap (ADR-0001): when map output exceeds a token budget, automatically reduce fidelity (signatures → names → paths). Degrade gracefully instead of truncating.

## What NOT to Build

- ❌ **Issues/PRs/releases** — gh already handles these well. ghx is for code exploration.
- ❌ **Pagination for search** — refine queries instead. 9 req/min makes pagination expensive (ADR-0003 Decision 4).
- ❌ **Tree-sitter integration** — requires WASM/native deps, breaks the zero-dependency promise. Use repomix for AST-level compression.
- ❌ **MCP server** — ghx's value is zero context overhead. An MCP wrapper would add the ~10K token schema cost we exist to avoid.

## Decision

Start with Tier 1 items (AGENTS.md detection, hints, token warnings). Each is <10 lines, high impact, no risk. Evaluate Tier 2 based on real usage patterns.
