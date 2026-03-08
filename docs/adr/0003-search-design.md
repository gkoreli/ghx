# ADR-0003: Search Design — Best-in-Class `ghx search`

**Date**: 2026-03-08
**Status**: Implemented (search decisions 1-6 shipped)

## Context

ghx exists to give AI agents the highest-impact information with the fewest tokens. Search is the entry point — agents search first, then read. If search is bad, everything downstream is wasted.

GitHub has **two code search systems**. Every programmatic tool (ghx, gh CLI, GitHub MCP, Octocode) hits the same legacy one. The new one is web-only, no API.

## The Two Systems

### System 1: Legacy Code Search (REST API — what we use)

- **Endpoint**: `GET /search/code`
- **Docs**: [Searching code (legacy)](https://docs.github.com/en/search-github/searching-on-github/searching-code) — GitHub themselves label it "legacy"
- **Engine**: Unknown legacy infrastructure. Both `sort` and `order` fields marked "closing down."

#### Qualifiers (verified working via API)

| Qualifier | Example | Notes |
|---|---|---|
| `in:file` / `in:path` / `in:file,path` | `octocat in:file` | Restrict search scope. Default: file contents only |
| `user:USERNAME` | `user:defunkt extension:rb` | All repos of a user |
| `org:ORGNAME` | `org:github extension:js` | All repos of an org |
| `repo:OWNER/REPO` | `repo:jquery/jquery` | Specific repo |
| `path:DIRECTORY` | `form path:cgi-bin` | Directory path (matches subdirs too) |
| `path:/` | `octocat path:/` | Root-level files only |
| `language:LANG` | `display language:scss` | GitHub linguist detection |
| `size:N` | `function size:>10000` | File size in bytes. Supports `>`, `<`, ranges |
| `filename:NAME` | `filename:package.json` | **Exact filename match. VALID in legacy API.** |
| `extension:EXT` | `form extension:pm` | File extension (no dot) |
| `fork:true` / `fork:only` | `ghx fork:true` | Include forks (default: excluded) |

#### Multi-word behavior (CRITICAL — verified via `GH_DEBUG=api`)

| Input | What's sent | Matching |
|---|---|---|
| `ghx search "foo bar"` | `q=foo+bar` | **AND** — both words anywhere in file |
| `gh search code "foo bar"` | `q=%22foo+bar%22` | **Exact phrase** — words must be adjacent |

`ghx search` sends unquoted (AND). `gh search code` silently wraps in quotes (exact phrase). AND is almost always what agents want. **This is a competitive advantage.**

#### Special characters (EXPERIMENTALLY VERIFIED)

The docs say `. , : ; / \ ' " = * ! ? # $ & + ^ | ~ < > ( ) { } [ ] @` are "ignored." **This is misleading.**

Tested: `console.log` → 21 results. `consolelog` → 0 results. The dot acts as a **word separator**, not truly ignored. `console.log` matches files containing both `console` and `log` near each other. This matters for code search — agents searching for `console.log` get different results than `consolelog`.

#### Limitations (from docs + verified)

- Default branch only (usually `main`/`master`)
- Files < 384 KB only
- Must include at least one search term — BUT the API doesn't enforce this (qualifier-only queries return garbage, no error)
- Repos with < 500,000 files only
- Repos must have had activity or search results in the last year
- Archived repos not searchable
- Forks: only if more stars than parent + at least one pushed commit
- Max 2 fragments per file in text_matches
- Up to 4,000 repos searched per query (scope limit)
- Max 1,000 total results (100 per page × 10 pages)

#### Rate limits

Code search is the most restricted search endpoint:

- **Authentication required** — unauthenticated requests return `{"message": "Requires authentication"}`. Every other search endpoint works without auth.
- **9 req/min** (authenticated) per the "About search" section. The "Search code" section says 10/min — GitHub's docs contradict themselves. **Budget for 9.**
- Other search endpoints: 30 req/min authenticated, 10 req/min unauthenticated.
- 422 can also mean "endpoint has been spammed" — abuse beyond normal rate limits.

#### Response structure

Without `text-match` header — 1 result = ~4,455 bytes. Mostly repo metadata bloat (owner URLs, fork URLs, etc.)

With `Accept: application/vnd.github.text-match+json` — 1 result = ~4,890 bytes (~10% more). Adds `text_matches` array:

```json
{
  "text_matches": [{
    "property": "content",       // always "content" even for in:path searches
    "fragment": "}\n\njQuery.fn.extend( {\n\taddClass: function( value ) {\n\t\tvar classNames...",
    "matches": [{ "text": "addClass", "indices": [30, 38] }]
  }]
}
```

Fragment is ~5-8 lines of surrounding code context. Max 2 fragments per item. `property` is always `"content"` — never `"path"` even when searching `in:path`. (Docs claim text_matches works for both `content` and `path` fields — empirically false.)

No specific token permissions needed — any authenticated fine-grained PAT works for code search.

#### Pagination

- `per_page`: 1-100 (default 30)
- `page`: 1-10 (max 1,000 results total)
- `total_count`: total matches found
- `incomplete_results`: true if query timed out

#### Error responses

| Code | Meaning |
|---|---|
| 200 | OK |
| 304 | Not modified (conditional request) |
| 403 | Rate limited or forbidden |
| 422 | Validation failed (empty query, access error) |
| 503 | Service unavailable |

Empty query → 422 with `{"errors":[{"resource":"Search","field":"q","code":"missing"}]}`.
Query > 256 chars (excluding qualifiers) → returns results but may be truncated.
Max 5 `AND`/`OR`/`NOT` operators per query (general search limit, though `OR`/`NOT` don't work in legacy API anyway).

### System 2: New GitHub Code Search (Web UI only — Blackbird engine)

- **Docs**: [Code search syntax](https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax)
- **Engine**: Blackbird (Rust). [Blog post](https://github.blog/2023-02-06-the-technology-behind-githubs-new-code-search/).
- **API access**: **NONE.** Web UI only.

#### Additional features NOT available via any API

| Feature | Example | Notes |
|---|---|---|
| `OR` operator | `sparse OR index` | Boolean OR |
| `NOT` operator | `"fatal error" NOT path:__testing__` | Exclusion |
| Parentheses | `(language:ruby OR language:python) AND NOT path:"/tests/"` | Grouping |
| Regex | `/sparse.*index/` | Full regex in search terms |
| `symbol:` qualifier | `language:go symbol:WithContext` | Function/class definitions (Tree-sitter) |
| `content:` qualifier | `content:README.md` | Match file content only (not path) |
| `is:` qualifier | `log4j NOT is:archived` | Filter by repo properties |
| `enterprise:` qualifier | `enterprise:octocorp` | Enterprise-wide search |
| Glob in `path:` | `path:src/**/*.js` | Glob patterns |
| Case-sensitive regex | `/(?-i)True/` | Opt-in case sensitivity |
| Escape sequences | `"name = \"tensorflow\""` | Backslash escaping |

#### Limitations

- Files < 350 KiB (slightly smaller than legacy's 384 KB)
- Lines > 1,024 chars truncated
- Files with >1 line over 4,096 bytes excluded
- Default branch only
- Max 100 results (5 pages). No sorting.
- No exhaustive search
- Vendored/generated code excluded
- Binary files excluded
- Only UTF-8

**Implication**: No programmatic tool can match the web UI. `ghx search` is already using the best available API. The gap is a GitHub platform limitation.

## Current State of `ghx search`

```bash
search)
  query="$*"
  gh api /search/code --method GET -f q="$query" \
    --jq '.items[] | "\(.repository.full_name) \(.path):\(.name)"'
```

**What's good**: AND matching by default, compact output.

**What's missing**:
1. No matching lines — returns only paths
2. No `text_matches` — the API supports it, we don't use it
3. No result count — agent doesn't know if 1 result or 1,000
4. No `incomplete_results` check — silent timeout
5. No query validation — web-only qualifiers silently degrade

## Design Decisions

### Principle: Agent reads output → immediately knows what to do next.

### Decision 1: Add text_matches (matching fragments)

Add `Accept: application/vnd.github.text-match+json` header. Extract first fragment per result.

Default output:
```
owner/repo path/to/file.ext: matching line content here
owner/repo path/to/other.ext: another matching line
```

One line per result. Path + first match fragment, trimmed to first line of fragment.

**Cost**: ~10% more API response bytes. **Benefit**: Eliminates follow-up reads for irrelevant results. Net token savings.

### Decision 2: Result count + incomplete warning to stderr

```bash
# stderr: "23 results"
# stderr: "⚠ Results may be incomplete (query timed out)" — only if incomplete_results=true
```

Agent knows if query needs refining. Doesn't pollute stdout.

### Decision 3: Query validation (warn on web-only features)

Before sending, detect qualifiers that only work in the new web code search:

| Pattern | Warning |
|---|---|
| `symbol:` | "symbol: is web-only, not available via API" |
| `NOT ` | "NOT operator is web-only" |
| ` OR ` | "OR operator is web-only" |
| `/regex/` | "Regex is web-only" |
| `content:` | "content: is web-only" |
| `is:` | "is: is web-only" |

Print to stderr. Don't block the query — send it anyway (the API will treat them as literal text, which might still be useful).

### Decision 4: Rate limit awareness

Code search is 10 req/min. ghx should:
- Default to `per_page=30` (one page, no pagination)
- If agent needs more, they refine the query — not paginate
- Respect rate limits in SKILL.md guidance

### Decision 5: Token budget protection (default safe, opt-out with `--full`)

Two layers of protection against token explosion:

**Layer 1: Line truncation (200 chars per result)**
Each matching line is capped at 200 characters. Minified JS files can have 10,000+ char lines — one search result could consume more tokens than the entire rest of the output. Truncation is silent unless it actually triggers, then warns on stderr: `"⚠ Lines truncated to 200 chars (use --full for complete fragments)"`.

**Layer 2: Broad query warning (>1,000 results)**
When `total_count > 1000`, stderr warns: `"⚠ Query too broad — add repo:, language:, or path: to narrow"`. Agents immediately know to refine instead of trusting the first 30 results from a 200K-result query.

**Opt-out: `--full` flag**
`ghx search --full "query"` disables line truncation. Same result count, same warnings, just untruncated fragment lines. For when the agent specifically needs the full matching context.

**Design rationale**: Safe by default, explicit opt-out. Agents that don't know about `--full` are protected. Agents that need full context can ask for it. This mirrors the `--map` philosophy — compact by default, full on request.

### Decision 6: Response token optimization

The raw API response is ~4.5KB per result, mostly repo metadata URLs we never use. Our jq already strips this. With text_matches, we add ~50 chars per result (one fragment line). Total output for 30 results: ~2KB vs raw API's ~135KB. **~98% reduction.**

### Decision 7: Prerequisite checks with clear errors

`ghx search` depends on `gh` CLI being installed and authenticated. Both can fail with cryptic errors. Add upfront checks:

| Condition | Current behavior | ghx should do |
|---|---|---|
| `gh` not installed | `ghx: line N: gh: command not found` | `"ghx requires the GitHub CLI (gh). Install: https://cli.github.com"` |
| `gh` not authenticated | `{"message": "Requires authentication"}` | `"gh is not authenticated. Run: gh auth login"` |

Check once at startup, fail fast with actionable message. No wasted API call.

## Corrections to Previous ADRs

1. **ADR-0001 claimed `filename:` is invalid** — WRONG. `filename:` is a valid legacy API qualifier. Tested: `filename:package.json repo:jquery/jquery` returns results. The confusion was mixing up legacy (where `filename:` works) with new code search (where only `path:` works).

2. **ADR-0001 claimed `OR` works in ghx search** — WRONG. `OR` is only available in the new web code search. The legacy REST API treats `OR` as a literal search term.

3. **Docs claim special chars are "ignored"** — MISLEADING. Dots act as word separators. `console.log` ≠ `consolelog`.

## Implementation Rationale

### Why text_matches (Decision 1)

Without fragments, search returns only file paths. An agent seeing 30 paths has no way to know which files are relevant without reading each one — that's 30 follow-up API calls. With fragments, the agent sees the matching line and can immediately filter to the 2-3 files that matter. The 10% API overhead pays for itself by eliminating 90% of follow-up reads.

We extract only the first line of the first fragment. The API returns up to 2 fragments per file, each ~5-8 lines. Dumping all of that would bloat output. One line is enough for an agent to decide "relevant" or "skip."

### Why stderr for metadata (Decision 2)

`total_count` and `incomplete_results` are critical for agents but must not pollute stdout. An agent piping `ghx search` output into further processing needs clean `repo path: line` on stdout. The count on stderr lets the agent (or its orchestrator) decide if the query needs refining — `201472 results` means "too broad, add qualifiers."

### Why warn but don't block (Decision 3)

Web-only qualifiers (`symbol:`, `OR`, `NOT`, regex) silently become literal text in the REST API. `symbol:foo` searches for the string "symbol:foo" inside files. This is the most dangerous failure mode — plausible-looking wrong results with no error.

We warn on stderr but still send the query because: (a) the literal text might still match something useful, (b) blocking would be surprising if the agent is experimenting, (c) the warning teaches the agent what works.

### Why no pagination (Decision 4)

Code search is rate-limited to 9 req/min. Pagination burns those precious requests on the same query. If 30 results aren't enough, the query is too broad — the agent should add qualifiers (`repo:`, `language:`, `path:`) to narrow results, not paginate. This is a deliberate design choice: force precision over volume.

### Why token budget protection (Decision 5)

Minified JS files can have 10,000+ char lines. One search result hitting a minified file could consume more tokens than the entire rest of the output combined. The 200-char truncation is invisible when lines are short (most code) and critical when they're not. The `--full` opt-out follows the same pattern as `--map` — compact by default, full on explicit request.

### Why prerequisite checks (Decision 7)

`gh` not installed → bash error `command not found` on some random line number. `gh` not authenticated → JSON error blob from the API. Both are cryptic. A 4-line check at the top gives an actionable message with the exact fix command. Fail fast, no wasted API call.

### Output format rationale

Before: `jquery/jquery src/attributes/classes.js:classes.js` — redundant (path contains filename).
After: `jquery/jquery src/attributes/classes.js: addClass: function( value ) {` — path + matching context.

One line per result. Scannable. An agent processing 30 results consumes ~2KB total vs ~135KB raw API response (~98% reduction). The jq extraction is where all the token savings happen.

### AND vs exact phrase (competitive advantage)

`ghx search "foo bar"` sends `q=foo+bar` (AND — both words anywhere in file).
`gh search code "foo bar"` sends `q=%22foo+bar%22` (exact phrase — words must be adjacent).

AND is almost always what agents want. An agent searching for `useState fetchData` wants files containing both terms, not necessarily adjacent. `gh search code` silently returns zero results for non-adjacent terms with no error — the worst possible failure mode for agents.

## Benchmark: ghx vs gh (empirical, 2026-03-08)

All tests use local `./ghx` binary, 8s rate limit sleep between calls. Token counts via tiktoken (cl100k_base).

### Code Search

| Scenario | ghx tokens | gh tokens | Ratio | Notes |
|---|---|---|---|---|
| Multi-word AND (`bar width repo:plausible/analytics`) | 927 | 951 | ~1x | ghx adds matching context, gh shows only paths |
| Non-adjacent words (`ghx gkoreli`) | 73 (1 result) | 1 (0 results) | **ghx finds it** | gh wraps in quotes → exact phrase → miss |
| Scoped search (`addClass repo:jquery/jquery`) | 395 | 442 | ~1x | ghx shows matching line, gh shows path only |
| Minified files (`jQuery.fn.extend filename:jquery.min.js`) | 1,602 | 60,200 | **37x fewer** | gh dumps raw minified lines |
| Broad query (`useState language:typescript`) | 1,784 | 1,101 | 1.6x more | ghx adds matching context (worth the tokens) |

**Key findings:**
- ghx's 200-char truncation prevents token explosion on minified files (37x reduction)
- AND matching finds results that gh's exact-phrase matching misses entirely
- For normal code, token counts are comparable — but ghx includes matching context while gh shows only paths
- ghx is slower per-call (~800ms vs ~450ms) due to jq processing + text_matches parsing

### Repo Exploration (ghx explore vs gh)

| Operation | ghx | gh equivalent | API calls |
|---|---|---|---|
| Repo overview (description + tree + README) | `ghx explore repo` — 1 GraphQL call, ~3,500 tokens | `gh repo view` + `gh api contents/` + `gh api readme` — 3 REST calls, ~3,500 tokens | **1 vs 3** |
| Subdirectory listing | `ghx explore repo path` — 1 GraphQL call | `gh api contents/path` — 1 REST call | 1 vs 1 |

ghx explore batches 3 operations into 1 GraphQL call. Same data, fewer round-trips.

### File Reading (ghx read vs gh api)

| Operation | ghx | gh equivalent | API calls |
|---|---|---|---|
| Read 3 files | `ghx read repo f1 f2 f3` — 1 GraphQL call | 3× `gh api contents/f` — 3 REST calls, base64 decode needed | **1 vs 3** |
| Read + grep | `ghx read repo f --grep "pat"` — 1 call, filtered output | `gh api contents/f` + pipe to grep — 1 call + shell processing | 1 vs 1 (but ghx is simpler) |
| Code map | `ghx read repo f --map` — 1 call, ~92% token reduction | No equivalent | **unique** |

### Tree Listing (ghx tree vs gh api)

| Operation | ghx | gh equivalent |
|---|---|---|
| Recursive tree | `ghx tree repo [path]` | `gh api repos/.../git/trees/main?recursive=1 --jq ...` |

Same endpoint, same data. ghx tree is a convenience wrapper — no efficiency gain.

### What gh Does That ghx Cannot

| Capability | gh command | ghx equivalent |
|---|---|---|
| **Repo search** | `gh search repos "query"` | **None** — use gh |
| **Issues** | `gh issue list/view` | **None** — use gh |
| **Pull requests** | `gh pr list/view/diff/checks` | **None** — use gh |
| **Releases** | `gh release list/view` | **None** — use gh |
| **Repo metadata** (stars, forks, language) | `gh repo view --json` | **None** — use gh |
| **Authentication** | `gh auth login/status` | Depends on gh for auth |
| **Creating/updating** (issues, PRs, releases) | `gh issue create`, `gh pr create` | **None** — use gh |

### Rate Limits by Endpoint (from GitHub docs + /rate_limit API)

| Endpoint | Limit | Pool |
|---|---|---|
| Core REST API (contents, trees, repos) | **5,000/hour** | `core` |
| GraphQL API (ghx explore, ghx read) | **5,000/hour** | `graphql` |
| Search (repos, issues, users) | **30/min** | `search` |
| Code search | **10/min** (budget for 9) | `code_search` |
| Unauthenticated | 60/hour | `core` |

Source: [GitHub rate limits docs](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)

Code search is the most restricted endpoint — 50x more limited than core REST. This is why ghx's "refine don't paginate" design matters. For explore/read/tree, the 5,000/hr limit is generous — batching saves round-trips, not rate limit budget.

### Verdict: When to Use What

**ghx wins** (use ghx):
- Code search — AND matching, matching context, token protection, warnings
- Repo exploration — 1 call vs 3 for description + tree + README
- Batch file reading — 1 call for N files, plus --grep/--map/--lines
- Code maps — `--map` has no gh equivalent

**gh wins** (use gh):
- Repo search (`gh search repos`) — ghx has no equivalent
- Issues, PRs, releases — gh has purpose-built commands
- Repo metadata (stars, forks, language) — `gh repo view --json`
- Authentication management — ghx depends on gh for auth
- Creating/updating resources — gh is the only option

**Neither wins** (equivalent):
- Tree listing — same endpoint, ghx is just a convenience wrapper
- Single file read — both are 1 API call (but ghx adds --grep/--map)

**ghx is a complement to gh, not a replacement.** Use ghx for code exploration (search + read + explore). Use gh for everything else (repos, issues, PRs, releases, auth, metadata).

## Implementation Status

- [x] Decision 1: `text_matches` header + first fragment line extraction
- [x] Decision 2: `total_count` + `incomplete_results` to stderr
- [x] Decision 3: Web-only qualifier warnings to stderr
- [x] Decision 4: `per_page=30` default, no pagination
- [x] Decision 5: 200-char line truncation, `--full` opt-out, broad query warning
- [x] Decision 6: jq extraction (~98% token reduction)
- [x] Decision 7: Prerequisite checks (`gh` installed + authenticated)
- [x] Fix `filename:` claim in ADR-0001
- [x] Fix `OR` claim in ADR-0001
- [x] Update SKILL.md with correct qualifier reference
- [x] Benchmark: token output before/after (5 search pairs + explore/read/tree comparisons)

## Sources

- REST API search docs: https://docs.github.com/en/rest/search/search
- Legacy code search syntax: https://docs.github.com/en/search-github/searching-on-github/searching-code
- New code search syntax: https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax
- About new code search: https://docs.github.com/en/search-github/github-code-search/about-github-code-search
- Blackbird blog post: https://github.blog/2023-02-06-the-technology-behind-githubs-new-code-search/
- Experimental verification: `GH_DEBUG=api` traces, direct API calls with various queries
