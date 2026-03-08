---
name: ghx
description: GitHub code exploration for AI agents. Use for repo exploration, reading remote files, code search, code maps. Wraps gh CLI with GraphQL batching — one command does what takes 3-5 API calls.
---

# ghx — GitHub Code Exploration for AI Agents

Use `ghx` via `execute_bash` for anything on GitHub — repos, files, code search. Authenticated via `gh` CLI, structured output, zero context overhead.

## Why This Exists

Agents exploring GitHub waste API calls, tokens, and time. Existing tools are either too heavy (MCPs with 10K+ token schemas) or too crude (raw `gh` commands requiring 3-5 sequential calls). `ghx` batches operations via GraphQL — one command does what takes 3-5 separate calls.

GitHub's own team (github/gh-aw) builds agent skills as bash scripts wrapping `gh` CLI. They cover PRs, issues, discussions — but have NO code exploration skill. Nobody built the lightweight middle ground. `ghx` fills that gap.

## Commands

```bash
ghx explore <owner/repo>                    # Branch + tree + README in 1 API call
ghx explore <owner/repo> <path>             # Subdirectory listing
ghx read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files in 1 API call (GraphQL batching)
ghx read <owner/repo> --map <f1> [f2]       # Structural map: signatures, imports, types (~92% token reduction)
ghx read <owner/repo> --grep "pat" <f>      # Read file, show only matching lines (2 lines context)
ghx read <owner/repo> --lines 42-80 <f>     # Read specific line range
ghx search "<query>"                        # Code search (full GitHub syntax)
ghx tree <owner/repo> [path]                # Full recursive tree listing
```

## Chain of Thought: Progressive Disclosure

**Always start surgical, escalate only when needed.** This mirrors how developers work: scan structure → identify interesting files → read specific sections.

```
1. ghx explore owner/repo          → What's in this repo? (structure + README)
2. ghx read owner/repo --map *.ts  → What do these files define? (signatures only, 92% fewer tokens)
3. ghx read owner/repo --grep "X" f → Where exactly is X in this file? (targeted lines)
4. ghx read owner/repo f            → Show me the full file (only when needed)
```

**Why this order matters:** At 92% reduction, `--map` lets you scan 7 files in the space of reading 1 full file. The agent can understand an entire module's structure before committing context to any single file. Aider's docs confirm: *"The LLM can see classes, methods and function signatures from everywhere in the repo. This alone may give it enough context to solve many tasks."*

**When to escalate beyond ghx:**
- "Understand this entire module" → `gitingest https://github.com/owner/repo/tree/branch/path -i "*.ts" -o - 2>/dev/null`
- "Compressed view of a codebase" → `npx repomix --remote owner/repo --compress --include "src/**" --stdout`

## Search Query Syntax

Same as github.com search bar. Both `ghx search` and `gh search code` support full query syntax including multi-word queries.

```bash
ghx search "bar width repo:plausible/analytics"              # Multi-word AND
ghx search "bar OR percentage repo:plausible/analytics"       # OR
ghx search '"progress_bar" repo:plausible/analytics'          # Exact phrase
ghx search "bar path:assets/js repo:plausible/analytics"      # Path filter
ghx search "bar extension:tsx repo:plausible/analytics"       # Extension filter
ghx search "bar language:javascript repo:plausible/analytics"  # Language filter
ghx search "path:llms.txt"                                     # Find files by name
```

**Valid qualifiers:** `repo:`, `org:`, `user:`, `path:`, `extension:`, `language:`, `NOT`, `OR`

**Rate limit:** 10 code search requests/minute. Space out calls.

## Gotchas

1. **`filename:` is NOT a valid qualifier.** GitHub code search silently treats it as literal text — searches for the TEXT "filename:llms.txt" inside files, NOT for files named llms.txt. Use `path:llms.txt` instead. This is the most dangerous failure mode: no error, plausible-looking wrong results.

2. **`language:markdown` won't find `.txt` files.** GitHub's linguist detection doesn't classify .txt as markdown. Use `extension:txt` instead. `language:` = linguist detection, `extension:` = literal file extension.

3. **`gh search code` works for multi-word queries** (contrary to earlier claims). Both `ghx search` and `gh search code` handle multi-word queries. `ghx search` returns compact output (paths + first match per file). `gh search code` returns paths + all matching line content.

4. **GraphQL returns null for missing paths.** `object(expression: "branch:path")` returns null silently if the path doesn't exist. No error. `ghx` handles this, but if using `gh api graphql` directly, check for null.

5. **Flag ordering in `read` command.** `ghx read owner/repo file --map` works. `ghx read --map owner/repo file` does NOT — repo must be the first positional arg.

6. **Not all repos use `main`.** cli/cli uses `trunk`, others use `master`. `ghx` handles this automatically. For raw `gh api` calls, query the default branch first: `gh repo view owner/repo --json defaultBranchRef --jq '.defaultBranchRef.name'`

7. **`gh` field names are inconsistent.** `stargazersCount` (search) vs `stargazerCount` (repo view). Always check with `--json` (no fields) to see available fields for any command.

## Anti-Patterns

- ❌ `web_fetch`/`web_search` on github.com — returns HTML noise, wastes thousands of tokens for zero useful information
- ❌ `gh api repos/.../contents/<path>` WITHOUT `-H "Accept: application/vnd.github.raw+json"` — returns base64-encoded JSON blob instead of readable text
- ❌ Reading entire large files when you need 10 lines — use `--grep "pattern"` or `--lines N-M`
- ❌ Multiple sequential `gh api` calls for explore workflows — use `ghx explore` (1 GraphQL call) or `ghx read` (batch files)
- ❌ Firing multiple code search requests in parallel — 10 req/min rate limit, you'll get 403s
- ❌ Dumping entire repos into context for a specific question — use targeted `ghx` commands. Reserve `gitingest`/`repomix` for "understand this whole module" tasks
- ❌ Using `gh search code` without `ghx search` when you need compact output — `gh search code` returns all matching lines (verbose), `ghx search` returns paths + first match (compact)

## Best Practices

- **Batch file reads.** `ghx read owner/repo f1 f2 f3` = 1 API call. Three separate reads = 3 calls.
- **Map before reading.** `ghx read --map` first to understand structure, then `--grep` or `--lines` for specifics.
- **Use `gh api --cache 1h`** for repeated lookups when using raw `gh` commands.
- **Use `--json fields --jq 'expr'`** on `gh` commands to get structured output and reduce noise.
- **Piped output is machine-formatted.** Tab-delimited, no truncation, no color codes — agents always get clean output.

## The `--map` Flag: Why It Matters

`--map` extracts only structural declarations (imports, exports, function/class/type signatures) via per-language regex patterns. Tested on 6 real files across TypeScript, Python, Go:

| Metric | Result |
|--------|--------|
| Average token reduction | 92% |
| Files scannable per context window | 7x more than full reads |
| Implementation | ~15 lines of bash, zero dependencies |

Output includes line numbers and token stats:
```
=== src/core/parseFile.ts (5544 bytes) ===
21:import type { RepomixConfigMerged } from '../../config/configSchema.js';
35:export const CHUNK_SEPARATOR = '⋮----';
38:export const parseFile = async (fileContent: string, filePath: string, config: RepomixConfigMerged) =>
107:const getLanguageParserSingleton = async () =>
# map: 812/5544 chars (~1386 tokens full, ~203 tokens map)
```

Supported: TypeScript/JavaScript, Python, Go, Rust, Java/Kotlin, Ruby. Generic fallback for unknown extensions.

## Examples

### Simple: Explore a repo and read a file

```bash
# What's in this repo?
ghx explore plausible/analytics

# Read the main config
ghx read plausible/analytics config/runtime.exs
```

### Advanced: Research a codebase you've never seen

```bash
# 1. Explore structure
ghx explore yamadashy/repomix

# 2. Map the core module — understand what exists (92% fewer tokens)
ghx read yamadashy/repomix --map src/core/output/outputGenerate.ts src/core/file/fileProcess.ts src/core/treeSitter/parseFile.ts

# 3. Found interesting function in map output — grep for usage details
ghx read yamadashy/repomix --grep "processFiles" src/core/file/fileProcess.ts

# 4. Search across the whole repo for a pattern
ghx search "CHUNK_SEPARATOR repo:yamadashy/repomix"

# 5. Read specific lines of a file you've narrowed down
ghx read yamadashy/repomix --lines 38-65 src/core/treeSitter/parseFile.ts

# 6. If you need the full picture of a subdirectory, escalate:
# gitingest https://github.com/yamadashy/repomix/tree/main/src/core -i "*.ts" -o - 2>/dev/null
```

## Complementary Tools

| Goal | Tool | Why |
|------|------|-----|
| Surgical exploration | `ghx` | Batched API calls, zero overhead, targeted extraction |
| Holistic understanding | `gitingest` / `repomix --compress` | Dump entire module for broad reasoning |
| PRs, issues, CI | `gh pr view`, `gh issue view`, `gh pr checks` | Purpose-built commands |

## `gh` CLI Quick Reference

```bash
# Repos
gh search repos "<query>" -L 10 --json fullName,description,stargazersCount
gh repo view owner/repo --json defaultBranchRef --jq '.defaultBranchRef.name'

# PRs
gh pr view 123 -R owner/repo                    # Title, body, status
gh pr diff 123 -R owner/repo                    # Full diff
gh pr checks 123 -R owner/repo                  # CI status

# Issues
gh issue view 456 -R owner/repo
gh issue list -R owner/repo -S "query" -L 20

# Raw API (always use the raw header for files)
gh api repos/owner/repo/contents/path -H "Accept: application/vnd.github.raw+json"
gh api repos/owner/repo/git/trees/main --jq '.tree[].path'   # List structure
```
