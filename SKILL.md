# ggcode — GitHub Code Exploration for AI Agents

Use `ggcode` via `execute_bash` for anything on GitHub — repos, files, code search. Authenticated via `gh` CLI, structured output, zero context overhead.

## Why This Exists

Agents exploring GitHub waste API calls, tokens, and time. Existing tools are either too heavy (MCPs with 10K+ token schemas) or too crude (raw `gh` commands requiring 3-5 sequential calls). `ggcode` batches operations via GraphQL — one command does what takes 3-5 separate calls.

GitHub's own team (github/gh-aw) builds agent skills as bash scripts wrapping `gh` CLI. They cover PRs, issues, discussions — but have NO code exploration skill. Nobody built the lightweight middle ground. `ggcode` fills that gap.

## Commands

```bash
ggcode explore <owner/repo>                    # Branch + tree + README in 1 API call
ggcode explore <owner/repo> <path>             # Subdirectory listing
ggcode read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files in 1 API call (GraphQL batching)
ggcode read <owner/repo> --map <f1> [f2]       # Structural map: signatures, imports, types (~92% token reduction)
ggcode read <owner/repo> --grep "pat" <f>      # Read file, show only matching lines (2 lines context)
ggcode read <owner/repo> --lines 42-80 <f>     # Read specific line range
ggcode search "<query>"                        # Code search (full GitHub syntax)
ggcode tree <owner/repo> [path]                # Full recursive tree listing
```

## Chain of Thought: Progressive Disclosure

**Always start surgical, escalate only when needed.** This mirrors how developers work: scan structure → identify interesting files → read specific sections.

```
1. ggcode explore owner/repo          → What's in this repo? (structure + README)
2. ggcode read owner/repo --map *.ts  → What do these files define? (signatures only, 92% fewer tokens)
3. ggcode read owner/repo --grep "X" f → Where exactly is X in this file? (targeted lines)
4. ggcode read owner/repo f            → Show me the full file (only when needed)
```

**Why this order matters:** At 92% reduction, `--map` lets you scan 7 files in the space of reading 1 full file. The agent can understand an entire module's structure before committing context to any single file. Aider's docs confirm: *"The LLM can see classes, methods and function signatures from everywhere in the repo. This alone may give it enough context to solve many tasks."*

**When to escalate beyond ggcode:**
- "Understand this entire module" → `gitingest https://github.com/owner/repo/tree/branch/path -i "*.ts" -o - 2>/dev/null`
- "Compressed view of a codebase" → `npx repomix --remote owner/repo --compress --include "src/**" --stdout`

## Search Query Syntax

Same as github.com search bar. `ggcode search` hits the REST API directly — supports full syntax unlike `gh search code` which silently breaks on multi-word queries.

```bash
ggcode search "bar width repo:plausible/analytics"              # Multi-word AND
ggcode search "bar OR percentage repo:plausible/analytics"       # OR
ggcode search '"progress_bar" repo:plausible/analytics'          # Exact phrase
ggcode search "bar path:assets/js repo:plausible/analytics"      # Path filter
ggcode search "bar extension:tsx repo:plausible/analytics"       # Extension filter
ggcode search "bar language:javascript repo:plausible/analytics"  # Language filter
ggcode search "path:llms.txt"                                     # Find files by name
```

**Valid qualifiers:** `repo:`, `org:`, `user:`, `path:`, `extension:`, `language:`, `NOT`, `OR`

**Rate limit:** 10 code search requests/minute. Space out calls.

## Gotchas

1. **`filename:` is NOT a valid qualifier.** GitHub code search silently treats it as literal text — searches for the TEXT "filename:llms.txt" inside files, NOT for files named llms.txt. Use `path:llms.txt` instead. This is the most dangerous failure mode: no error, plausible-looking wrong results.

2. **`language:markdown` won't find `.txt` files.** GitHub's linguist detection doesn't classify .txt as markdown. Use `extension:txt` instead. `language:` = linguist detection, `extension:` = literal file extension.

3. **`gh search code` silently fails on multi-word queries.** Returns empty results with no error. Always use `ggcode search` instead — it hits the REST API directly.

4. **GraphQL returns null for missing paths.** `object(expression: "branch:path")` returns null silently if the path doesn't exist. No error. `ggcode` handles this, but if using `gh api graphql` directly, check for null.

5. **Flag ordering in `read` command.** `ggcode read owner/repo file --map` works. `ggcode read --map owner/repo file` does NOT — repo must be the first positional arg.

6. **Not all repos use `main`.** cli/cli uses `trunk`, others use `master`. `ggcode` handles this automatically. For raw `gh api` calls, query the default branch first: `gh repo view owner/repo --json defaultBranchRef --jq '.defaultBranchRef.name'`

7. **`gh` field names are inconsistent.** `stargazersCount` (search) vs `stargazerCount` (repo view). Always check with `--json` (no fields) to see available fields for any command.

## Anti-Patterns

- ❌ `web_fetch`/`web_search` on github.com — returns HTML noise, wastes thousands of tokens for zero useful information
- ❌ `gh api repos/.../contents/<path>` WITHOUT `-H "Accept: application/vnd.github.raw+json"` — returns base64-encoded JSON blob instead of readable text
- ❌ Reading entire large files when you need 10 lines — use `--grep "pattern"` or `--lines N-M`
- ❌ Multiple sequential `gh api` calls for explore workflows — use `ggcode explore` (1 GraphQL call) or `ggcode read` (batch files)
- ❌ Firing multiple code search requests in parallel — 10 req/min rate limit, you'll get 403s
- ❌ Dumping entire repos into context for a specific question — use targeted `ggcode` commands. Reserve `gitingest`/`repomix` for "understand this whole module" tasks
- ❌ Using `gh search code` for multi-word queries — silently returns empty. Use `ggcode search`

## Best Practices

- **Batch file reads.** `ggcode read owner/repo f1 f2 f3` = 1 API call. Three separate reads = 3 calls.
- **Map before reading.** `ggcode read --map` first to understand structure, then `--grep` or `--lines` for specifics.
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
ggcode explore plausible/analytics

# Read the main config
ggcode read plausible/analytics config/runtime.exs
```

### Advanced: Research a codebase you've never seen

```bash
# 1. Explore structure
ggcode explore yamadashy/repomix

# 2. Map the core module — understand what exists (92% fewer tokens)
ggcode read yamadashy/repomix --map src/core/output/outputGenerate.ts src/core/file/fileProcess.ts src/core/treeSitter/parseFile.ts

# 3. Found interesting function in map output — grep for usage details
ggcode read yamadashy/repomix --grep "processFiles" src/core/file/fileProcess.ts

# 4. Search across the whole repo for a pattern
ggcode search "CHUNK_SEPARATOR repo:yamadashy/repomix"

# 5. Read specific lines of a file you've narrowed down
ggcode read yamadashy/repomix --lines 38-65 src/core/treeSitter/parseFile.ts

# 6. If you need the full picture of a subdirectory, escalate:
# gitingest https://github.com/yamadashy/repomix/tree/main/src/core -i "*.ts" -o - 2>/dev/null
```

## Complementary Tools

| Goal | Tool | Why |
|------|------|-----|
| Surgical exploration | `ggcode` | Batched API calls, zero overhead, targeted extraction |
| Holistic understanding | `gitingest` / `repomix --compress` | Dump entire module for broad reasoning |
| PRs, issues, CI | `gh pr view`, `gh issue view`, `gh pr checks` | Purpose-built commands |
| Single file (simplest) | `gh api repos/o/r/contents/path -H "Accept: application/vnd.github.raw+json"` | One file, one call |

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
