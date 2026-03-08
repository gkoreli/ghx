# ADR-0003: Search Design — Best-in-Class `ghx search`

**Date**: 2026-03-08
**Status**: In Progress

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

### Decision 5: Response token optimization

The raw API response is ~4.5KB per result, mostly repo metadata URLs we never use. Our jq already strips this. With text_matches, we add ~50 chars per result (one fragment line). Total output for 30 results: ~2KB vs raw API's ~135KB. **~98% reduction.**

### Decision 6: Prerequisite checks with clear errors

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

## Implementation Plan

1. Add `Accept: application/vnd.github.text-match+json` header to search
2. Update jq to extract first `text_matches[0].fragment` (first line only)
3. Add `total_count` and `incomplete_results` to stderr
4. Add query validation for web-only qualifiers (stderr warnings)
5. Update SKILL.md with correct qualifier reference
6. Fix `filename:` claim in ADR-0001
7. Benchmark: token output before/after

## Sources

- REST API search docs: https://docs.github.com/en/rest/search/search
- Legacy code search syntax: https://docs.github.com/en/search-github/searching-on-github/searching-code
- New code search syntax: https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax
- About new code search: https://docs.github.com/en/search-github/github-code-search/about-github-code-search
- Blackbird blog post: https://github.blog/2023-02-06-the-technology-behind-githubs-new-code-search/
- Experimental verification: `GH_DEBUG=api` traces, direct API calls with various queries
