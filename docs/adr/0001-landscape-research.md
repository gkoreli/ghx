## GitHub Content: Use `ghx` and `gh` CLI

For anything on GitHub — repos, files, PRs, issues — use CLI tools via `execute_bash`. Authenticated, structured data, no HTML scraping.

### Problem: Agents Waste API Calls, Tokens, and Time Exploring GitHub

Agents exploring GitHub repos face a tooling gap: existing tools are either too heavy (MCPs with 10K+ token schemas injected into every conversation) or too crude (raw `gh` commands requiring 3-5 sequential calls for basic operations). The result: agents burn context on tool schemas they don't need, make redundant API calls, and dump entire files when they need 10 lines.

**What ghx solves**: Efficient agent-native GitHub exploration via GraphQL batching, targeted extraction, and zero context overhead. One `ghx read` call does what takes 3-5 separate API calls with any other tool.

**Why this gap exists**: GitHub's own team (github/gh-aw) builds agent skills as bash scripts wrapping `gh` CLI — the same pattern as ghx. Their skills cover PRs (`query-prs.sh`), issues (`query-issues.sh`), and discussions (`query-discussions.sh`). But they have NO code exploration skill. No `explore-repo.sh`, no `read-files.sh`, no `search-code.sh`. The MCP ecosystem went heavy (50+ tools, 10K+ tokens of schemas). Nobody built the lightweight middle ground for code exploration. Source: `github/gh-aw/skills/` directory — verified via `ghx tree`.

**Design principle**: Fail loudly, never silently. GitHub has two code search systems with different syntax. The REST API (legacy) supports qualifiers like `filename:`, `extension:`, `in:`. The web UI (new, Blackbird) supports `symbol:`, `OR`, `NOT`, regex — but has no API. Using new-only qualifiers in the REST API silently treats them as literal text. Agent-friendly tools must validate inputs and surface errors explicitly. See ADR-0003 for the complete reference.

### `ghx` — GitHub eXplorer (preferred for research)

Use `ghx` for repo exploration. It batches API calls via GraphQL — one command does what takes 3-5 separate `gh` calls.

```
ghx explore <owner/repo>              # Branch + tree + README in 1 API call
ghx explore <owner/repo> <path>       # Subdirectory listing
ghx read <owner/repo> <f1> [f2] [f3]  # Read 1-10 files in 1 API call
ghx read <repo> --grep "pattern" <f>  # Read file, show only matching lines (2 lines context)
ghx read <repo> --lines 42-80 <f>     # Read specific line range from file
ghx read <repo> --map <f1> [f2] [f3]  # Structural map: signatures, imports, types only (~92% reduction)
ghx search "<query>"                   # Code search (AND matching, qualifiers: repo:, org:, user:, path:, language:, extension:)
ghx tree <owner/repo> [path]           # Full recursive tree listing
```

Search query syntax (same as github.com search bar):
- Multi-word AND: `ghx search "bar width repo:plausible/analytics"`
- Scope to repo: `ghx search "bar repo:plausible/analytics"`
- Exact phrase: `ghx search '"progress_bar" repo:plausible/analytics'`
- Path filter: `ghx search "bar path:assets/js repo:plausible/analytics"`
- Extension: `ghx search "bar extension:tsx repo:plausible/analytics"`
- Language: `ghx search "bar language:javascript repo:plausible/analytics"`
- Find files by name: `ghx search "path:llms.txt"` or `ghx search "path:llms.txt extension:txt"`
- No regex support via API
- ⚠️ The REST API uses **legacy** code search syntax. Valid qualifiers: `repo:`, `org:`, `user:`, `path:`, `extension:`, `language:`, `in:`, `size:`, `filename:`, `fork:`. Web-only qualifiers (`OR`, `NOT`, `symbol:`, `content:`, `is:`, regex) are silently treated as literal text — no error, wrong results. See ADR-0003 for the complete two-system reference.

Rate limit: 10 code search requests/minute. Space out calls.

### `gh` CLI (for PRs, issues, and single operations)

**Discover & explore repos:**

- `gh search repos "<query>"` — find repositories by keyword
- `gh repo view <owner/repo>` — description + full README
- `gh repo view <owner/repo> --json name,description,defaultBranchRef --jq '.defaultBranchRef.name'` — structured metadata

**Read files without cloning (ALWAYS use the raw header):**

- `gh api repos/<owner>/<repo>/contents/<path> -H "Accept: application/vnd.github.raw+json"` — returns plain text, ready to use
- `gh api repos/<owner>/<repo>/git/trees/<branch> --jq '.tree[].path'` — list repo structure (branch varies per repo — query it first)

**PRs and issues:**

- `gh pr view <number> -R <owner/repo>` — title, body, status, reviewers
- `gh pr diff <number> -R <owner/repo>` — full diff
- `gh pr checks <number> -R <owner/repo>` — CI status per check
- `gh issue view <number> -R <owner/repo>` — issue body + comments
- `gh issue list -R <owner/repo> -S "<query>"` — search issues (uses `-S` flag)
- `gh search code "<query>"` — supports full query syntax including multi-word queries. Returns paths + matching line content. `ghx search` provides more compact output.

**Best practices (from gh maintainers):**

- Use `--json fields --jq 'expr'` to get structured output and reduce noise — built into most `gh` commands, no external `jq` needed
- Use `gh api` with `-H "Accept: application/vnd.github.raw+json"` for file contents — returns raw text, avoids base64
- Use `--paginate` on `gh api` to auto-fetch all pages — no loops or cursor management needed
- Use `--cache 1h` on `gh api` for repeated lookups — avoids redundant API calls within a session
- Query default branch first with `gh repo view --json defaultBranchRef` — not all repos use `main` (e.g., cli/cli uses `trunk`)
- Piped output (which agents always produce) is automatically machine-formatted: tab-delimited, no truncation, no color codes
- Shallow clone (`gh repo clone <r> -- --depth=1`) only when you need to explore many files at once

**Anti-patterns:**

- ❌ `web_fetch`/`web_search` on github.com — returns HTML noise that wastes thousands of context tokens for zero useful information
- ❌ `gh api repos/.../contents/<path>` WITHOUT `-H "Accept: application/vnd.github.raw+json"` — returns base64-encoded JSON blob instead of readable text. With the header, returns plain text for files and JSON array for directories.
- ❌ Hardcoding `main` as branch in API calls — repos use different defaults (`trunk`, `master`, etc.). `ghx` handles this automatically. For raw `gh api`, query with `gh repo view --json defaultBranchRef` first.
- ❌ Reading entire large files when you need 10 lines — use `ghx read --grep "pattern"` or `--lines N-M` to extract only what you need
- ❌ Multiple sequential `gh api` calls for explore workflows — use `ghx explore` (1 GraphQL call) or `ghx read` (batch files) instead
- ❌ ~~Using `gh search code` for multi-word queries~~ — **CORRECTED 2026-03-08**: `gh search code` handles multi-word queries correctly. Earlier claim was false. `ghx search` still adds value through compact output and (planned) configurable verbosity.
- ❌ Using web-only search qualifiers in the REST API — `symbol:`, `OR`, `NOT`, `content:`, `is:`, regex are only available in GitHub's new web code search (Blackbird). The REST API silently treats them as literal text. `symbol:foo` searches for the TEXT "symbol:foo" inside files. Valid REST API qualifiers: `repo:`, `org:`, `user:`, `path:`, `filename:`, `extension:`, `language:`, `in:`, `size:`, `fork:`. See ADR-0003.
- ❌ Using `language:markdown` to find `.txt` files — GitHub doesn't classify .txt as markdown. Use `extension:txt` instead. `language:` matches GitHub's linguist detection, `extension:` matches the literal file extension.
- ❌ Firing multiple code search requests in parallel — 10 req/min rate limit. Space them out or you'll get 403s.
- ❌ Dumping entire repos into context with `gitingest` for a specific question — use targeted `ghx` commands instead. Reserve `gitingest` for "understand this whole module" tasks.

## `gh` CLI Reference

**`gh` JSON fields (verified — field names are inconsistent across commands):**

`gh search repos --json`: fullName, description, stargazersCount, url, language, defaultBranch, forksCount, owner, updatedAt, isArchived, license, visibility, name
`gh repo view --json`: name, nameWithOwner, description, defaultBranchRef, stargazerCount, url, isPrivate, languages, primaryLanguage, owner, repositoryTopics, isEmpty, isFork
`gh pr view --json`: title, body, state, author, number, url, additions, deletions, files, comments, reviews, reviewDecision, headRefName, baseRefName, isDraft, labels, mergedAt, mergedBy, createdAt, updatedAt
`gh pr list --json`: title, number, author, state, url, headRefName, baseRefName, createdAt, updatedAt, isDraft, labels, reviewDecision
`gh issue view --json`: title, body, state, author, number, url, labels, comments, assignees, milestone, createdAt, closedAt, updatedAt
`gh issue list --json`: title, number, author, state, url, labels, assignees, createdAt, updatedAt
`gh search code --json`: path, repository, sha, textMatches, url

⚠️ `stargazersCount` (search) vs `stargazerCount` (repo view) — gh is inconsistent here.

**`gh` flag patterns (consistent across commands):**

- `-R <owner/repo>` — target a repo (pr, issue, release, run commands)
- `-S "<query>"` — search filter (issue list, pr list)
- `-L <n>` — limit results (all list/search commands, default 30)
- `-q '<jq>'` — jq filter (all commands that support --json)
- `--json <fields>` — structured output (run with no fields to see available fields for any command)

**`gh api` for GitHub REST API:**

- `-H "Accept: application/vnd.github.raw+json"` — returns plain text for files, JSON array for directories. Works for both.
- `--paginate` — auto-fetches all pages, no loops needed
- `--cache 1h` — caches responses, avoids redundant API calls
- `--jq '<expr>'` — filter response inline
- Contents API: `repos/<owner>/<repo>/contents/<path>` — works for files and directories
- Tree API: `repos/<owner>/<repo>/git/trees/<branch>` — requires actual branch name, use `gh repo view <r> --json defaultBranchRef --jq '.defaultBranchRef.name'` to get it

## `gh api graphql` — For Custom Batch Operations

`ghx` wraps GraphQL for common patterns. For custom queries beyond what `ghx` covers, use `gh api graphql` directly:

```
gh api graphql -f query='
{
  repository(owner: "OWNER", name: "REPO") {
    defaultBranchRef { name }
    tree: object(expression: "BRANCH:path/to/dir") {
      ... on Tree { entries { name type } }
    }
    file: object(expression: "BRANCH:path/to/file.js") {
      ... on Blob { text byteSize }
    }
  }
}' --jq '.data.repository'
```

Key patterns: use aliases (`f1:`, `f2:`) to read multiple files in one call. `... on Blob { text }` for file content, `... on Tree { entries { name type } }` for directory listing. No code search in GraphQL — use `ghx search` for that.

**GraphQL gotchas:**
- No code search type — only ISSUE, ISSUE_ADVANCED, REPOSITORY, USER, DISCUSSION. Code search is REST-only.
- `object(expression: "branch:path")` returns null silently if path doesn't exist — no error, just null. Check for it.
- Large files via `... on Blob { text }` return the full content — no line range or match filtering. For targeted extraction, use `ghx read --grep` or `--lines` which fetches via GraphQL then filters locally.

## Landscape: What Exists (and Why We Don't Use It)

**GitHub MCP Server** (github/github-mcp-server) — Official, 50+ tools. Their `search_code` uses `go-github` library's `client.Search.Code()` which hits REST `/search/code` directly — supports full query syntax (multi-word, path filters). `gh search code` also uses this endpoint but wraps multi-word queries in quotes (exact phrase matching — see ADR-0002 Issue 2). No GraphQL batching, no multi-file reads, ~10K tokens of tool schemas injected into every agent context. Each file read = 1 tool call. Their `get_file_contents` doesn't support line ranges or match filtering. Overkill for research.

**Octocode MCP** (bgauryy/octocode-mcp, 741⭐) — Best-in-class research MCP. Uses Octokit REST API (not GraphQL). Novel ideas discovered from source code:
- **Hints system**: Every tool response includes contextual hints that guide the agent's next action. Three categories: `hasResults` (suggest next tool), `empty` (suggest broadening search), `error` (suggest recovery). Example: "💡 TIP: Use matchString for targeted extraction instead of paginating through entire file". This prevents agents from wasting calls on wrong tools.
- **Content minification**: Per-file-type strategies — `terser` for JS, `conservative` (strip comments, collapse blank lines) for TS/Python/YAML, `aggressive` (collapse all whitespace) for Go/Java/CSS/HTML, `json` (JSON.stringify), `markdown` (custom). Saves 30-50% tokens. TypeScript uses conservative (not aggressive) because indentation matters for readability.
- **matchString parameter**: Read a file but only return lines matching a pattern (with configurable N context lines). Like grep-in-read. Avoids dumping 500-line files when agent needs 10 lines.
- **startLine/endLine**: Read specific line ranges from remote files. No need to fetch entire file.
- **charOffset/charLength pagination**: Large files auto-paginate at 20K chars/page. Hints tell agent exact params for next page: `"▶ Next page: Use charOffset=20000"`. Agent doesn't need to figure out pagination logic.
- **Token usage warnings**: Responses include token estimates. >50K tokens = "🚨 CRITICAL: Response TOO LARGE". >30K = "⚠️ WARNING". Prevents context overflow.
- **Content sanitization**: 18 categories of secret patterns (AWS keys, API tokens, private keys, database URLs, Slack tokens, etc.) auto-redacted to `[REDACTED-AWS-ACCESS-KEY]` before reaching agent. Prevents accidental secret exposure.
- **Caching**: All API responses cached per session (node-cache, 24h TTL, 5000 max keys). Re-reading same file = instant. Cache key excludes research context fields (mainResearchGoal, reasoning) so same file fetched for different reasons hits cache.
- **File filtering**: Extensive ignore lists — 85 folder names (node_modules, dist, .git, __pycache__, etc.), 40+ file names (package-lock.json, secrets.json, id_rsa, etc.), 30+ extensions (.lock, .log, .min.js, .map, .woff, .ttf, etc.). Prevents agents from reading noise.
- **Directory fetch**: `type: "directory"` downloads entire dir to local disk via sparse checkout, then local tools (ripgrep, LSP) can analyze it. Best of both worlds.
- BUT: Still an MCP (context bloat), requires npm/Docker, runs a local HTTP server on port 1987, no GraphQL batching. Their research skill is extremely process-heavy (5 phases, gates, checkpoints).

**Gitingest** (coderamp-labs/gitingest, 14K⭐) — Converts repo/subdir to a single text file. Installed via `pip install gitingest`. Key findings from testing:
- **Subpath support**: `gitingest https://github.com/owner/repo/tree/branch/path/to/dir` — only ingests that subdirectory. Uses git sparse checkout (`--filter=blob:none --sparse`) so it doesn't download the entire repo.
- **Include/exclude patterns**: `-i "*.tsx" -e "test*"` — shell-style glob filtering. Useful for "give me all TypeScript files in this dir".
- **Output format**: Tree structure + concatenated file contents with `================================================` separators and `FILE: path/to/file.ext` headers. Token count estimate included.
- **Speed**: ~3.6s for a subdirectory (clone + process) vs ~0.9s for `ghx explore` (GraphQL, no clone). 4x slower because it actually clones.
- **Token-optimized separator**: Uses exactly 48 `=` characters because tiktoken counts that as 2 tokens. More than 48 = more tokens for no benefit.
- **Binary detection**: Reads a small chunk, tries UTF-8 decode. Binary files get `[Binary file]` placeholder instead of garbage.
- **When to use**: "Understand this entire module/package" — dump all files at once for the agent to reason about holistically. NOT for targeted research (too much context for a specific question).
- **When NOT to use**: Large repos (>10K files or >500MB). Single file lookups. Searching for specific patterns. These are all better served by `ghx`.
- **Gotcha**: Logs to stderr by default — use `2>/dev/null` to suppress. Output goes to `digest.txt` by default — use `-o -` for stdout.

**Repomix** (yamadashy/repomix, 55K⭐) — The most feature-rich repo-to-text tool. TypeScript, installable via `npx repomix`. Key innovations discovered from source code analysis:
- **Tree-sitter code compression** (`--compress`): Parses code into AST, extracts ONLY signatures (function defs, class defs, imports, types, interfaces, enums). Implementation bodies are replaced with `⋮----` separator. Per-language strategies: Python extracts `def name(args)` + decorators (strips body), TypeScript extracts function signatures up to `{` or `=>` (strips implementation), Go/CSS/Vue have custom strategies. Claims ~70% token reduction while preserving semantic meaning. This is AST-level compression — fundamentally more accurate than Octocode's text-based minification.
- **XML output format**: Uses XML tags (`<file path="...">`, `<directory_structure>`, `<file_summary>`) because Anthropic's docs say XML tags help Claude parse prompts more accurately. Custom instructions go at the END of output — Anthropic says "queries at the end can improve response quality by up to 30%".
- **Git change frequency sorting**: Files sorted by git commit frequency — most-changed files go to the BOTTOM of output. Why? LLMs pay more attention to content at the end of context (recency bias). Most active files get positioned where the model pays most attention.
- **`--stdin` pipe composability**: `find src -name "*.ts" | repomix --stdin` or `grep -l "TODO" **/*.ts | repomix --stdin`. Pipe file lists from any tool into repomix. Ultimate composability.
- **Pack-once-query-many** (MCP mode): `pack_codebase` creates an indexed output, then `grep_repomix_output` searches it repeatedly. Avoids re-fetching files for each question. The "index once, query many" pattern.
- **`⋮----` elision marker**: Both repomix and aider's grep-ast use `⋮` to indicate elided code. Becoming a standard for AI-readable code output. Tells the agent "there's more code here but it's not relevant."
- **Token-optimized separators**: Like gitingest's 48-char `=` separator (2 tiktoken tokens), repomix optimizes output format for minimal token overhead.
- **When to use**: `npx repomix --remote owner/repo --compress --include "src/**/*.ts" --stdout` — compressed view of a module. Or `npx repomix path/to/local/dir --compress` for local code.
- **When NOT to use**: Requires npm/npx. Slower than `ghx` for targeted lookups. Overkill for reading 1-3 files.

**Aider's RepoMap** (Aider-AI/aider, 35K⭐) — Not a tool we'd use directly, but its context selection algorithm is the most sophisticated in the landscape. Key insights from `repomap.py` source:
- **PageRank-based file ranking**: Builds a directed graph where nodes are files and edges are symbol references (file A references symbol defined in file B). Runs NetworkX PageRank with personalization to rank files by relevance to the current task.
- **Personalization weights**: Files in the active chat get base weight. Files whose path components match mentioned identifiers get boosted. References FROM chat files get 50x weight multiplier.
- **Identifier weighting heuristics**: snake_case/camelCase identifiers with 8+ chars get 10x weight (they're specific). Identifiers starting with `_` get 0.1x (private, less important). Identifiers defined in 5+ files get 0.1x (too generic, like `name` or `id`). Mentioned identifiers get 10x.
- **Binary search for token budget**: Given a max_map_tokens budget, binary-searches the number of ranked tags to include until output fits within 15% of budget. Efficient use of context window.
- **Token counting optimization**: For text >200 chars, samples every Nth line and extrapolates instead of counting every token. Fast approximation.
- **Inspiration for agents**: When exploring a large repo, don't read files alphabetically or randomly. Prioritize files that are most referenced by the files you're already looking at. Follow the reference graph.

**grep-ast** (paul-gauthier/grep-ast) — The library powering aider's repo map. Grep with AST-aware context:
- **Structural context**: Instead of showing matching line + N context lines (like regular grep), shows the STRUCTURAL context — the parent function/class/method that contains the match, with its header (first few lines of the definition).
- **`⋮` elision**: Lines between shown sections are replaced with `⋮`, telling the agent "code exists here but isn't relevant."
- **Scope tracking**: Every line knows which scopes (functions, classes) it belongs to. When showing a match, it walks up the scope tree to show parent headers.
- **Gap closing**: If lines i and i+2 are shown but i+1 isn't, it shows i+1 too (avoids distracting single-line gaps).
- **Inspiration for `ghx read --grep`**: Currently `--grep` does plain text grep. With Tree-sitter, it could show "this match is inside function X in class Y" — much more useful for understanding code structure.

## Research Workflow: When to Use What

| Goal | Tool | Why |
|---|---|---|
| "What's in this repo?" | `ghx explore owner/repo` | Branch + tree + README in 1 call |
| "Read these 3 files" | `ghx read owner/repo f1 f2 f3` | 1 GraphQL call for all files |
| "Find where X is used" | `ghx search "X repo:owner/repo"` | Full search syntax |
| "Show me line 42-80 of this file" | `ghx read owner/repo --lines 42-80 file.ts` | Targeted extraction |
| "Find X in this file" | `ghx read owner/repo --grep "X" file.ts` | Grep-in-read, saves tokens |
| "Understand this entire module" | `gitingest https://github.com/owner/repo/tree/branch/path -i "*.ts" -o -` | All files at once, filtered |
| "Compressed view of a module" | `npx repomix --remote owner/repo --compress --include "src/**" --stdout` | Signatures only, ~70% fewer tokens |
| "Read a single file" | `gh api repos/owner/repo/contents/path -H "Accept: application/vnd.github.raw+json"` | Simplest for one file |
| "Check PR diff" | `gh pr diff 123 -R owner/repo` | Purpose-built command |

**Escalation pattern**: Start surgical (`ghx explore` → `ghx search` → `ghx read --grep`). If you need broader understanding, escalate to holistic (`gitingest` or `repomix --compress`). Never start with a full dump.

## Novel Ideas — Inspired by the Landscape

Ideas worth implementing in `ghx` or adopting as agent practices, ranked by impact.

**Implementable in bash (no new dependencies):**

1. **Search query validation** (from the `filename:` bug): Before sending a search query, check for known-invalid qualifiers (`filename:`, `in:`, `type:`) and warn. GitHub's API silently degrades invalid qualifiers to literal text — the most dangerous failure mode for agents. A 5-line bash check prevents hours of wasted investigation on wrong results.

2. **Hints in output** (from Octocode): After search results, append `"Hint: use ghx read to view these files"`. After large file read, append `"Hint: use --grep to narrow"`. After explore, `"Hint: use ghx tree for recursive listing"`. Trivial to implement — just echo a line. Guides agent's next action without burning context on wrong tool calls.

3. **Schema-first response pattern** (from GitHub's own gh-aw skills): When a command could return overwhelming data, return schema + size + suggested queries first. GitHub's `query-prs.sh` does this: without `--jq`, it returns `{"item_count": 30, "data_size_bytes": 45000, "schema": {...}, "suggested_queries": [...]}`. Prevents agents from drowning in data. Could apply to `ghx tree` on large repos. Source: `github/gh-aw/skills/github-pr-query/query-prs.sh`.

4. **AGENTS.md / CLAUDE.md detection in explore** (emerging convention): `ghx explore` already reads README.md. Add AGENTS.md and CLAUDE.md to the GraphQL query — one extra alias, zero extra API calls. Gives agents immediate context about repo conventions. Repomix, cli/cli, github/gh-aw, and many popular repos already have these.

5. **Token estimation and warnings** (from Octocode/repomix): After reading files, estimate tokens (`wc -c` output / 4 is a rough approximation). Warn if >30K tokens. `"⚠️ Large output (~45K tokens). Consider using --grep or --lines to narrow."` Prevents context overflow.

6. **File filtering in tree** (from Octocode/gitingest): Pipe tree output through a filter that strips known noise (node_modules, .lock files, dist/, build/, __pycache__, .min.js, images). A `--filter` flag or default behavior. Reduces tree clutter on large repos.

7. **Content minification** (from Octocode): Strip single-line comments and collapse blank lines before returning. `sed '/^[[:space:]]*\/\//d; /^[[:space:]]*#[^!]/d; /^$/N;/^\n$/d'` handles most cases. Saves 20-30% tokens on verbose codebases. Could be a `ghx read --minify` flag.

8. **Sparse checkout for deep analysis** (from gitingest): `git clone --depth=1 --filter=blob:none --sparse <url> /tmp/ghx-$repo && git -C /tmp/ghx-$repo sparse-checkout set <path>`. Gives a local filesystem without downloading blobs. Then local tools (grep, ripgrep) can analyze without API rate limits. Could be a `ghx clone` command. Source: gitingest's `clone.py`.

**Requires external dependencies (not pure bash):**

9. **Tree-sitter code compression** (from repomix): Extract only signatures, imports, types — strip implementation bodies. `⋮----` marks elided code. ~70% token savings. Requires Tree-sitter WASM binaries — not bash-appropriate. Use `npx repomix --remote owner/repo --compress --stdout` as a complementary tool instead. Source: `yamadashy/repomix/src/core/treeSitter/parseFile.ts`.

10. **Structural grep context** (from grep-ast/aider): When grepping, show the parent function/class header, not just N surrounding lines. Requires Tree-sitter parsing. Source: `paul-gauthier/grep-ast/grep_ast/grep_ast.py` class `TreeContext`.

11. **Relevance-ranked exploration** (from aider): Build a reference graph and prioritize files that are most connected to what you're investigating. Requires NetworkX or similar graph library. Source: `Aider-AI/aider/aider/repomap.py` — PageRank with personalization weights.

**Agent practice (no tool changes needed):**

12. **Git change frequency sorting** (from repomix): When exploring a repo, most-changed files are often most important. Implementation: `git log --pretty=format: --name-only -n 100` → count filename occurrences → sort ascending. Agents can do this after `ghx clone`. Source: `yamadashy/repomix/src/core/git/gitCommand.ts`.

13. **Response caching** (from Octocode): `gh api --cache 1h` already exists. Agents should use `ghx` commands that internally use `gh api` with caching. Especially useful when agent reads a file, then re-reads it with `--grep`.

14. **Output format for AI consumption** (from repomix): XML tags help LLMs parse structured output. Custom instructions at END of output (Anthropic says "queries at the end improve response quality by up to 30%"). Source: `yamadashy/repomix/src/core/output/outputStyles/xmlStyle.ts`.

## Deep Dive: Regex-Based Code Map (Highest Impact Idea)

The single highest-impact enhancement for ghx, inspired by Repomix and Aider's repo map.

### The Principle: Show Structure, Hide Implementation

Agents exploring code need to understand WHAT exists and HOW things connect — not line-by-line implementation. Every tool in the landscape solves this differently:
- **Repomix**: Tree-sitter AST parsing → extracts signatures, types, imports. Requires WASM binaries.
- **Aider**: Tree-sitter + grep-ast → shows structural context (parent function/class headers) around matches. Requires Python + Tree-sitter.
- **ghx opportunity**: Regex-based signature extraction → zero dependencies, pure bash.

### Evidence: Regex Gets 92% Average Reduction (Tested on 6 Real Files)

| File | Language | Full | Map | Reduction | Map Lines |
|------|----------|------|-----|-----------|-----------|
| repomix/parseFile.ts | TypeScript | 5,599 | 812 | 86% | 13 |
| github-mcp/repositories.go | Go | 68,862 | 1,551 | 97.7% | 20 |
| aider/repomap.py | Python | 27,346 | 1,496 | 94.5% | 43 |
| repomix/cliRun.ts | TypeScript | 11,934 | 1,108 | 90.7% | 18 |
| octocode/bulk.ts | TypeScript | 8,513 | 712 | 91.6% | 13 |
| codemap 3-file batch | Mixed TS | 50,859 | 3,177 | 93.8% | 45 |

**Average: 92.4% reduction. Median: ~92%.** Consistent across languages and file sizes (5KB to 68KB).

For comparison: Repomix Tree-sitter `--compress` achieves 57% on the same parseFile.ts. Regex gets more compression because it extracts ONLY structural declarations. Repomix also keeps comments and interface bodies — useful for intent, noisy for structure scanning.

The 3-file batch test is the most telling: 50,859 chars (12,714 tokens) → 3,177 chars (794 tokens). An agent can map **16 files** in the space of reading 1 file fully.

### Now Implemented: `ghx read --map`

The `--map` flag is live in ghx. Per-language regex patterns detected from file extension:

```
ghx read <owner/repo> --map <file1> [file2] [file3]
```

Output includes line numbers and token stats:
```
=== src/core/treeSitter/parseFile.ts (5544 bytes) ===
21:import type { RepomixConfigMerged } from '../../config/configSchema.js';
35:export const CHUNK_SEPARATOR = '⋮----';
38:export const parseFile = async (fileContent: string, filePath: string, config: RepomixConfigMerged) =>
107:const getLanguageParserSingleton = async () =>
117:export const cleanupLanguageParser = async (): Promise<void> =>
130:const filterDuplicatedChunks = (chunks: CapturedChunk[]): CapturedChunk[] =>
153:const mergeAdjacentChunks = (chunks: CapturedChunk[]): CapturedChunk[] =>
# map: 812/5544 chars (~1386 tokens full, ~203 tokens map)
```

Supported languages: TypeScript/JavaScript, Python, Go, Rust, Java/Kotlin, Ruby. Falls back to generic pattern for unknown extensions.

### Why This Is Transformative for Agents

1. **7x more files per context window**: At 86% reduction, an agent reads 7 file maps in the space of 1 full file. Difference between "explore 3 files, run out of context" and "understand an entire module."

2. **Matches how developers work**: Scan structure first, zoom into specifics. Aider's docs: *"The LLM can see classes, methods and function signatures from everywhere in the repo. This alone may give it enough context to solve many tasks."* Source: `Aider-AI/aider/aider/website/docs/repomap.md`.

3. **Two-phase workflow**: `ghx read --map file1 file2 file3` (structure) → `ghx read --grep "specific_fn" file2` (detail). This is exactly Aider's approach: binary-search for maximum ranked tags that fit the token budget. Source: `aider/repomap.py` lines 670-710.

4. **Zero dependencies**: ~15 lines of bash. Language detection from file extension, per-language grep patterns.

### Per-Language Patterns (Tested)

```bash
# TypeScript/JavaScript
grep -nE '^(import |export |const |let |var |function |class |interface |type |enum )'

# Python
grep -nE '^(import |from |class |def |    def |        def |@)'

# Go
grep -nE '^(package |import |func |type |var |const )'

# Rust
grep -nE '^(use |pub |fn |struct |enum |trait |impl |type |mod |const )'

# Java
grep -nE '^(import |public |private |protected |class |interface |enum |@)'
```

### Complementary: Scope-Aware Grep

When using `--grep`, instead of showing N context lines, show the enclosing function/class header. Inspired by grep-ast's `TreeContext` which tracks scope chains and shows parent headers with `⋮` elision. Source: `paul-gauthier/grep-ast/grep_ast/grep_ast.py` class `TreeContext`, methods `add_parent_scopes()` and `walk_tree()`.

Approximation in bash (awk backward search for nearest declaration):
```bash
# Tracks most recent scope declaration, prints it with each match
awk '/^(export |const |function |class )/ { scope=$0; scope_line=NR }
     /PATTERN/ { printf "  [scope: %s]\n%d: %s\n", scope, NR, $0 }'
```

Transforms grep output from "line 103 with 2 context lines" to "line 103 inside `parseFile()` function" — infinitely more useful for agents.

### What Repomix Does That We Can't (and Don't Need To)

Repomix's TypeScript strategy (`TypeScriptParseStrategy.ts`) does sophisticated things:
- Extracts multi-line function signatures by finding `)` + `{` or `=>`
- Cleans signatures by stripping body openers (`{`, `=>`)
- Deduplicates by function name (`func:${name}` in processedChunks)
- Handles arrow functions assigned to `const`/`let`/`var`

For initial structure scanning, our regex approach captures the same information (function name + params) from the first line. The multi-line signature case (params spanning lines) is the one gap — but it's rare and the first line still tells the agent what the function is.

### What Aider Does That We Should Adopt as Practice

Aider's repo map has two ideas worth adopting as agent PRACTICES (not tool features):

1. **Token-budget binary search**: Aider includes as many ranked tags as fit within a token budget (default 1K). Binary search between 0 and N tags to find the maximum that fits. For agents: estimate output size before reading, warn if it'll exceed budget.

2. **Git change frequency sorting**: Most-changed files are most important. `git log --pretty=format: --name-only -n 100` → count occurrences → sort ascending (most-changed last, where LLMs pay most attention). The actual command from Repomix's `gitCommand.ts`:
```bash
git -C <dir> log --pretty=format: --name-only -n 100
```
Then count filename occurrences and sort. Files with more changes go to the bottom of output.

### New Discovery: Codemap — Progressive Detail Reduction

`kcosr/codemap` (15⭐) is the most architecturally sophisticated code mapping tool found. While low-star, its design is the most thoughtful in the landscape. Key innovations:

**1. Progressive detail reduction to fit a token budget.** Five detail levels per file:

| Level | Includes | Use case |
|-------|----------|----------|
| `full` | Signatures + JSDoc + nested members | Deep understanding |
| `standard` | Signatures + truncated comments (160 chars) | Normal exploration |
| `compact` | Signatures only, no comments | Broad scanning |
| `minimal` | Names only (no types/params) | Maximum coverage |
| `outline` | File path + line range only | Inventory |

The algorithm: start all files at `full`. While total tokens > budget, find the largest file and reduce its detail level by one step. Repeat until budget met. Source: `kcosr/codemap/src/sourceMap.ts` function `fitToBudget()`.

The design philosophy: **the context window is finite, so degrade gracefully instead of failing.** Don't truncate — reduce fidelity. The agent still sees every file, just with less detail on the largest ones. This is fundamentally better than hard truncation (which loses files entirely) or uniform reduction (which wastes budget on small files that could be shown in full).

ghx's `--map` is equivalent to codemap's `compact` level. A future `--map=names` could be the `minimal` level (names only, no signatures). The progressive reduction algorithm itself maps to an agent practice: map files at compact first, if output exceeds budget, re-map at minimal, if still too large, fall back to outline (just filenames).

**2. Persistent annotations on symbols.** Codemap stores notes and tags on files and symbols in a SQLite cache that survives reindexing. Example: `codemap annotate src/db.ts:Database:class "Singleton - use getInstance()"`. When the agent runs codemap again, the annotation appears in the output. This is novel for agent workflows: annotate important symbols during exploration, they persist across sessions. Source: `src/cache/annotations.ts` — 35 exported functions for CRUD on file/symbol annotations and tags.

**3. Cross-file reference tracking.** Dependency trees (`codemap deps src/index.ts`), call graphs (`codemap call-graph main`), type hierarchy (`codemap subtypes MyClass`). Uses TypeScript compiler API (ts-morph) for precise resolution. Not implementable in bash, but the CONCEPT is valuable: when exploring, follow imports to understand connections. An agent using `ghx read --map` can see imports in the map output and follow them manually. Source: `src/refs/extractor.ts`, `src/deps/tree.ts`, `src/refs/call-graph.ts`.

**4. Token estimation: `Math.ceil(rendered.length / 4)`.** Same chars/4 formula we use in ghx. Industry standard approximation. Source: `src/sourceMap.ts` line 119.

**5. Proof that maps alone are sufficient.** Codemap's `examples/chat-answer-no-file-reads.txt` shows Claude answering detailed architecture questions about the codemap codebase using ONLY the map output — no file reads needed. The agent correctly identified: core functionality, architecture, caching strategy, and design patterns. This validates our `--map` approach: for understanding WHAT a codebase does and HOW it's organized, signatures are enough.

**Codemap vs ghx**: Complementary, not competing. They solve different problems for different moments:
- **ghx** = discovery tool for remote repos you've never seen before. Zero setup, instant results, works across many repos in one session. Agents doing research across 5-10 repos need zero-friction access, not deep analysis.
- **codemap** = deep analysis tool for local repos you're actively working in. Rich features (references, call graphs, annotations) but requires Node.js + npm install + indexing + learning 20+ CLI commands.
- **The real workflow**: `ghx explore` → `ghx read --map` → understand structure → if deep analysis needed, clone and use codemap locally. Nobody uses codemap to browse a repo they've never seen. Nobody uses ghx to trace call graphs in their own project.

### Architectural Decisions and Rationale

**Why regex over Tree-sitter for `--map`:**
- Works on remote repos via API (Tree-sitter needs local files)
- Zero dependencies (Tree-sitter needs WASM binaries or native modules)
- 92% reduction is sufficient for structure scanning (Tree-sitter's 57% includes comments we don't need)
- ~15 lines of bash vs ~500 lines of TypeScript
- Trade-off accepted: misses multi-line signatures (params spanning lines). First line still identifies the function.

**Why bash over Node.js/Python:**
- Zero install — `gh` CLI is already on PATH for any GitHub user
- Composable with shell pipes (`ghx read ... | grep ... | wc -l`)
- No package manager, no node_modules, no virtual environments
- ~120 lines total — entire tool fits in one screen
- Trade-off accepted: no Tree-sitter, no SQLite caching, no cross-file references. These are complementary tool territory.

**Why API-first over clone-first:**
- Faster: 0.9s (GraphQL) vs 3.6s (sparse clone) for basic exploration
- No disk usage — works in constrained environments (CI, containers, cloud desktops)
- No cleanup — cloned repos accumulate in /tmp
- Rate limits are generous: 5000 GraphQL points/hour, 5000 REST calls/hour
- Trade-off accepted: can't use local tools (ripgrep, Tree-sitter) on API-fetched content. For deep analysis, use `ghx clone` (sparse checkout) then local tools.

**Why progressive disclosure (explore → map → grep → full read):**
- Each step adds detail only when needed, minimizing wasted tokens
- Mirrors how human developers work: scan structure → identify interesting files → read specific sections
- Prevents the "dump everything" anti-pattern that overwhelms agent context
- Aider's docs confirm: *"The LLM can see classes, methods and function signatures from everywhere in the repo. This alone may give it enough context to solve many tasks."*
- Codemap's example proves: an agent answered architecture questions using ONLY map output, zero file reads

## The Big Picture: Where This Is Going

### Why ghx Has No Direct Competitor

Evidence-based competitive analysis — every claim verified from source code:

| Capability | ghx | Octocode MCP | GitHub MCP | Repomix | Gitingest |
|---|---|---|---|---|---|
| Read N files in 1 API call | ✅ GraphQL aliases | ❌ N parallel REST calls¹ | ❌ N separate tool calls² | ❌ Clones first³ | ❌ Clones first |
| Explore repo (tree+README) in 1 call | ✅ | ❌ 2+ calls | ❌ 2+ calls | ❌ Full pack | ❌ Full clone |
| Code search (full syntax) | ✅ REST | ✅ REST | ✅ REST | ❌ grep packed output | ❌ No |
| Grep-in-read | ✅ --grep | ✅ matchString | ❌ | ❌ | ❌ |
| Line ranges | ✅ --lines | ✅ startLine/endLine | ❌ | ❌ | ❌ |
| Tree-sitter compression | ❌ | ❌ | ❌ | ✅ --compress | ❌ |
| Context overhead (tool schemas) | 0 tokens | ~10K tokens | ~10K tokens | 0 (CLI mode) | 0 |
| Dependencies | `gh` CLI | npm + Docker + Octokit | Go binary | npm/npx + WASM | pip + tiktoken |
| Lines of code | ~120 bash | ~15K TypeScript | ~10K Go | ~20K TypeScript | ~5K Python |
| Install | Already on PATH | npm install + config | Download binary | npx (no install) | pip install |

¹ Octocode: `executeBulkOperation()` → `processBulkQueries()` → `Promise.allSettled()` — each file is a separate REST call via Octokit, fired in parallel with concurrency control. Source: `packages/octocode-mcp/src/utils/response/bulk.ts`, `packages/octocode-mcp/src/utils/core/promise.ts`.

² GitHub MCP: `client.Repositories.GetContents(ctx, owner, repo, path, opts)` — one REST call per file. Source: `pkg/github/repositories.go` line 702.

³ Repomix: `readRawFile()` reads from LOCAL filesystem via `fs.readFile()`. For remote repos, it shallow-clones first (`git clone --depth 1`), then reads locally. Source: `src/core/file/fileRead.ts`, `src/core/git/gitCommand.ts`.

**The tools occupy different niches:**
- **ghx** = surgical agent CLI (interactive exploration, batched API calls, zero overhead)
- **Repomix** = holistic dump tool (pack everything for understanding, Tree-sitter compression)
- **Octocode** = feature-rich MCP (great UX patterns, but heavy context cost)
- **GitHub MCP** = official but bloated (50+ tools for every GitHub operation)
- **Gitingest** = simple dump tool (Python, good for data science workflows)

**What makes ghx unique**: GraphQL batching for multi-file reads. This is a fundamental architectural advantage — it's not a feature that can be added to REST-based tools. GraphQL's alias mechanism (`f0:`, `f1:`, etc.) allows combining N file reads into 1 HTTP request. The REST API has no equivalent.

### Three Complementary Strategies for Agent Code Exploration

1. **Surgical exploration** (`ghx`): Read specific files, search specific patterns, explore structure. Minimal API calls, minimal tokens. Best for: "find where X is defined", "read this config file", "what's the structure of this repo?"

2. **Holistic understanding** (`gitingest`, `repomix --compress`): Dump an entire module/package into context. Best for: "understand this whole module", "how does this subsystem work?", "what patterns does this codebase use?"

3. **Intelligent context selection** (aider's PageRank approach): Use reference graphs to pick the RIGHT files. Best for: "which files are most relevant to this change?", "what would I need to modify to add feature X?"

No single tool in the landscape combines all three. `ghx` occupies the sweet spot for strategy #1: lightweight (120 lines bash), zero dependencies beyond `gh` CLI, agent-friendly output, and it does the most common operations (explore, read, search, tree) in minimal API calls.

### Emerging Convention: AGENTS.md

Repos are adopting `AGENTS.md` (and `CLAUDE.md`) files that provide instructions to AI agents — coding guidelines, directory structure, testing commands, commit conventions. Repomix's `AGENTS.md` points to `.agents/rules/base.md` with 6K of structured guidance. `ghx explore` could detect and surface these files alongside README.md.

### Why Agents Make Mistakes (and How Tooling Fixes It)

The search qualifier bug (`filename:` silently treated as literal text) reveals a systemic problem: **APIs with silent failure modes are agent-hostile.** When an invalid qualifier returns plausible-looking results instead of an error, agents can't distinguish "valid query with no results" from "malformed query returning wrong results."

Root causes of agent mistakes on GitHub:
1. **Silent API degradation**: Invalid qualifiers become literal text. No error, no warning. Agent thinks the query worked.
2. **Inconsistent syntax across APIs**: `gh search code` uses different syntax than `/search/code` REST API. Old search had `filename:`, new search uses `path:`.
3. **Missing input validation**: Tools pass queries through without checking. A 5-line check for known-invalid qualifiers would prevent hours of wasted investigation.
4. **No output validation**: When search returns markdown files discussing "llms.txt" instead of actual llms.txt files, nothing flags the mismatch.

**Design principle for agent-friendly tools**: Validate inputs before sending. Warn on suspicious outputs. Fail loudly, never silently. This is why ghx should add search query validation as its first enhancement.

### Distribution: gh-skill Registry

`gh-skill` (nicholasspencer/gh-skill) is a skill registry using GitHub Gists. Skills are gists with `*.skill.md` files. Any agent can install with `gh skill add <gist-url>`. Auto-links to Claude Code, Copilot CLI, Codex, Cursor, OpenCode skill directories. ghx could be published as a skill — one command to install across all agent platforms. Source: `nicholasspencer/gh-skill` README.
