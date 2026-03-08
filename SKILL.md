---
name: ghx
description: GitHub code exploration for AI agents. Use for repo exploration, reading remote files, code search, code maps. Wraps gh CLI with GraphQL batching — one command does what takes 3-5 API calls.
---

# ghx — GitHub Code Exploration for AI Agents

Use `ghx` via `execute_bash` for anything on GitHub — repos, files, code search. Authenticated via `gh` CLI, structured output, zero context overhead.

## Why This Exists

Agents exploring GitHub face a reliability gap: *"Did I find nothing because nothing exists, or because I used the tool wrong?"* Raw `gh` commands have silent failure modes — `gh search code` wraps in quotes without telling you, `gh api contents/` returns base64, README requires a separate call. The agent can't distinguish "no results" from "wrong flags."

ghx eliminates this by encoding the right defaults into every command. One call returns enough context to decide the next action. You opt into the ghx skill and stop worrying about whether you searched correctly — the right behavior is the default behavior.

## Commands

```bash
ghx explore <owner/repo>                    # Branch + tree + README in 1 API call
ghx explore <owner/repo> <path>             # Subdirectory listing
ghx read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files in 1 API call (GraphQL batching)
ghx read <owner/repo> --map <f1> [f2]       # Structural map: signatures, imports, types (~92% token reduction)
ghx read <owner/repo> --grep "pat" <f>      # Read file, show only matching lines (2 lines context)
ghx read <owner/repo> --lines 42-80 <f>     # Read specific line range
ghx repos "<query>"                         # Search repos with README preview in 1 GraphQL call
ghx search "<query>"                        # Code search (REST API, AND matching, shows matching lines)
ghx search --full "<query>"                 # Code search without line truncation (for minified files)
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

`ghx search` uses the GitHub REST code search API (legacy). Multi-word queries use AND matching — both words must appear in the file but not necessarily adjacent. This is different from `gh search code` which silently wraps in quotes (exact phrase).

**Output format:**
```
201472 results (showing 30)                              ← stderr (total + page count)
jquery/jquery src/attributes/classes.js: addClass: function( value ) {   ← stdout (repo path: matching line)
```

Agents get: result count (stderr) + one line per result with matching context (stdout).

```bash
ghx search "addClass repo:jquery/jquery"                  # Scoped to repo
ghx search "useState language:typescript"                 # Language filter
ghx search "filename:package.json repo:owner/repo"        # Find specific filename
ghx search "form path:cgi-bin extension:py"               # Path + extension filter
ghx search '"progress_bar" repo:plausible/analytics'      # Exact phrase (shell quotes around double quotes)
ghx search "path:llms.txt"                                # Find files by name
```

**Valid REST API qualifiers:** `repo:`, `org:`, `user:`, `path:`, `filename:`, `extension:`, `language:`, `in:file`, `in:path`, `size:`, `fork:true`

**Web-only (DO NOT USE — silently treated as literal text):** `OR`, `NOT`, `symbol:`, `content:`, `is:`, regex (`/pattern/`), `enterprise:`, glob in `path:`. ghx warns on stderr if you use these.

**Rate limit:** 9 req/min for code search (strictest endpoint). Authentication required — `gh auth login` first.

**Special characters:** Dots act as word separators, not wildcards. `console.log` matches files with both `console` and `log` — it does NOT match `consolelog`.

## Search Strategy for Agents

**Search is the entry point.** Agents search first, then read. Bad search = wasted follow-up reads = token explosion. ghx search is designed to give you enough context to decide your next action in one call.

### Reading search output

```
90 results (showing 30)                                          ← stderr: is this too broad?
⚠ Lines truncated to 200 chars (use --full for complete fragments) ← stderr: token protection kicked in
⚠ Query too broad — add repo:, language:, or path: to narrow      ← stderr: >1000 results
jquery/jquery src/attributes/classes.js: addClass: function( value ) {  ← stdout: repo path: matching line
```

**Decision tree after seeing results:**
- `0 results` → query too specific, broaden (remove qualifiers, try synonyms)
- `1-30 results` → good. Scan matching lines, `ghx read` the relevant files
- `30-1000 results` → workable but noisy. Add `repo:`, `language:`, or `path:` to narrow
- `>1000 results` → too broad. MUST add qualifiers before trusting results
- `⚠ incomplete` → query timed out, results are partial. Narrow the scope

### Token protection (safe by default)

ghx truncates each matching line to 200 chars. This prevents minified JS files (10,000+ char lines) from exploding your context window. One untruncated minified result can consume more tokens than the other 29 results combined.

- **Default**: 200 char truncation. You see `⚠ Lines truncated` on stderr only when it triggers.
- **`--full`**: Disables truncation. Use when you specifically need the complete matching line.
- **When to use `--full`**: Almost never. The truncated line is enough to decide "relevant" or "skip." Use `ghx read` to get the full file context after you've identified the right file.

### Search refinement chain of thought

```
1. ghx search "useState"                              → 201K results. Too broad.
2. ghx search "useState language:typescript"           → 50K results. Still broad.
3. ghx search "useState repo:vercel/next.js"           → 89 results. Workable.
4. ghx search "useState path:packages/next extension:tsx repo:vercel/next.js"  → 12 results. Surgical.
```

**Refine, don't paginate.** At 9 req/min, pagination burns rate limit on the same broad query. Adding one qualifier is always better than fetching page 2.

### Two search systems (why some things don't work)

GitHub has two code search engines. The REST API (what ghx uses) is the legacy one. The web UI uses Blackbird (new). No programmatic tool — ghx, `gh` CLI, GitHub MCP, Octocode — can access Blackbird. This is a platform limitation, not a ghx limitation.

**What this means for agents:**
- `OR`, `NOT`, `symbol:`, regex, `content:`, `is:` → web-only, don't use
- `repo:`, `path:`, `filename:`, `language:`, `extension:`, `in:`, `size:`, `fork:` → work in REST API
- ghx warns on stderr if you use web-only qualifiers, but the results will be wrong

### ghx search vs `gh search code`

| Behavior | `ghx search` | `gh search code` |
|---|---|---|
| Multi-word matching | AND (both words anywhere) | Exact phrase (words must be adjacent) |
| Matching context | Shows matching line per result | No matching context |
| Result count | stderr: "90 results (showing 30)" | Not shown |
| Token protection | 200 char truncation, `--full` opt-out | None |
| Web-only warnings | Warns on stderr | Silent |
| Rate limit | Same (9 req/min) | Same |

AND matching is almost always what agents want. `gh search code "useState fetchData"` returns zero results if the words aren't adjacent — with no error. `ghx search "useState fetchData"` finds files containing both terms.

## Gotchas

1. **Web-only qualifiers silently degrade.** `symbol:`, `OR`, `NOT`, `content:`, `is:`, regex — these only work in GitHub's new web code search (Blackbird). The REST API treats them as literal text. `symbol:foo` searches for the TEXT "symbol:foo" inside files. ghx warns on stderr, but the results will be wrong. No programmatic tool can use these features — it's a GitHub platform limitation.

2. **`filename:` vs `path:` — both valid, different systems.** `filename:package.json` works in the REST API (legacy) for exact filename match. `path:` also works and is more flexible (matches directories too). In the NEW web code search, only `path:` works — `filename:` is not recognized. Since ghx uses the REST API, both work.

3. **`language:markdown` won't find `.txt` files.** GitHub's linguist detection doesn't classify .txt as markdown. Use `extension:txt` instead. `language:` = linguist detection, `extension:` = literal file extension.

4. **`gh search code` silently wraps queries in quotes.** `gh search code "foo bar"` sends `q="foo bar"` (exact phrase), not `q=foo bar` (AND). If the words aren't adjacent in the file, you get zero results with no error. `ghx search` sends AND queries — both words must appear but in any order. This is almost always what you want. ghx also shows result count on stderr and matching line context — `gh search code` shows neither.

5. **GraphQL returns null for missing paths.** `object(expression: "branch:path")` returns null silently if the path doesn't exist. No error. `ghx` handles this, but if using `gh api graphql` directly, check for null.

6. **Flag ordering in `read` command.** `ghx read owner/repo file --map` works. `ghx read --map owner/repo file` does NOT — repo must be the first positional arg.

7. **Not all repos use `main`.** cli/cli uses `trunk`, others use `master`. `ghx` handles this automatically. For raw `gh api` calls, query the default branch first: `gh repo view owner/repo --json defaultBranchRef --jq '.defaultBranchRef.name'`

8. **`gh` field names are inconsistent.** `stargazersCount` (search) vs `stargazerCount` (repo view). Always check with `--json` (no fields) to see available fields for any command.

9. **`gh api repos/.../contents/` returns base64 by default.** Without `-H "Accept: application/vnd.github.raw+json"`, you get a JSON blob with base64-encoded content. `ghx read` returns plain text via GraphQL — no decoding needed.

10. **`gh search repos` and `gh search code` use different rate limit pools.** Repo search: 30/min (generous). Code search: 10/min (restrictive). Don't assume one rate limit applies to both.

## Anti-Patterns

- ❌ `web_fetch`/`web_search` on github.com — returns HTML noise, wastes thousands of tokens for zero useful information
- ❌ `gh api repos/.../contents/<path>` WITHOUT `-H "Accept: application/vnd.github.raw+json"` — returns base64-encoded JSON blob instead of readable text
- ❌ Reading entire large files when you need 10 lines — use `--grep "pattern"` or `--lines N-M`
- ❌ Multiple sequential `gh api` calls for explore workflows — use `ghx explore` (1 GraphQL call) or `ghx read` (batch files)
- ❌ Using web-only qualifiers (`OR`, `NOT`, `symbol:`, regex) in `ghx search` — silently treated as literal text, returns wrong results. ghx warns but can't prevent it
- ❌ Firing multiple code search requests in parallel — 9 req/min rate limit, you'll get 403s
- ❌ Dumping entire repos into context for a specific question — use targeted `ghx` commands. Reserve `gitingest`/`repomix` for "understand this whole module" tasks
- ❌ Relying on `gh search code` for multi-word queries — silently wraps in quotes (exact phrase), returns nothing when words aren't adjacent. Use `ghx search` (AND matching + matching context)
- ❌ Using `ghx search` to find repos — ghx search is for code. Use `ghx repos "query"` for repo discovery
- ❌ Using `gh` for batch file reads — 1 API call per file, base64 encoded. Use `ghx read repo f1 f2 f3` (1 GraphQL call, plain text)
- ❌ Using `gh repo view` to explore a repo — gets metadata but not tree listing or README content in one call. Use `ghx explore` (1 call for all three)

## Best Practices

- **Batch file reads.** `ghx read owner/repo f1 f2 f3` = 1 API call. Three separate reads = 3 calls.
- **Map before reading.** `ghx read --map` first to understand structure, then `--grep` or `--lines` for specifics.
- **Refine search, don't paginate.** If `ghx search` shows "201472 results (showing 30)", add qualifiers (`repo:`, `language:`, `path:`) — don't try to page through. 9 req/min rate limit makes pagination expensive.
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
# → stderr: "3 results (showing 3)"
# → stdout: yamadashy/repomix src/core/output/outputGenerate.ts: const CHUNK_SEPARATOR = '⋮----';

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

## ghx vs gh: When to Use What

**ghx is a complement to gh, not a replacement.** Use ghx for code exploration. Use gh for everything else.

### Use ghx (code exploration)

| Task | Command | Why ghx wins |
|------|---------|-------------|
| Code search | `ghx search "query"` | AND matching (gh uses exact phrase), matching context, 37x token reduction on minified files, result count + warnings on stderr |
| Repo search | `ghx repos "query"` | 1 GraphQL call gets name + stars + language + README preview. gh needs 1+N calls for same info, returns worse ranking, no README |
| Repo overview | `ghx explore owner/repo` | 1 GraphQL call gets description + tree + README (gh needs 3 calls) |
| Read multiple files | `ghx read owner/repo f1 f2 f3` | 1 GraphQL call for N files (gh needs N calls, returns base64) |
| Targeted extraction | `ghx read --grep "pat" f` | Built-in grep with context lines — no shell piping |
| Code map | `ghx read --map f1 f2` | ~92% token reduction, no gh equivalent |

### Use gh (everything else)

| Task | Command | Why gh wins |
|------|---------|-------------|
| Issues | `gh issue list/view -R owner/repo` | ghx doesn't touch issues |
| Pull requests | `gh pr list/view/diff/checks -R owner/repo` | ghx doesn't touch PRs |
| Releases | `gh release list -R owner/repo` | ghx doesn't touch releases |
| Repo metadata | `gh repo view owner/repo --json stargazerCount,forkCount` | Detailed stats beyond what ghx repos shows |
| Auth | `gh auth login/status` | ghx depends on gh for auth |
| Create/update | `gh issue create`, `gh pr create` | ghx is read-only |

### Rate limits (from GitHub docs)

| Endpoint | Limit | Used by |
|---|---|---|
| Core REST | 5,000/hour | gh commands, ghx tree |
| GraphQL | 5,000/hour | ghx explore, ghx read |
| Search (repos, issues) | 30/min | `gh search repos/issues` |
| Code search | 10/min (budget 9) | `ghx search`, `gh search code` |

Code search is 50x more restricted than core REST. This is why "refine don't paginate" matters for search but not for explore/read.

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
