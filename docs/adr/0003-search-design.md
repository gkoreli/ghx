# ADR-0003: Search Design — Best-in-Class `ghx search`

**Date**: 2026-03-08
**Status**: In Progress

## Context

ghx exists to give AI agents the highest-impact information with the fewest tokens. Search is the entry point — agents search first, then read. If search is bad, everything downstream is wasted.

GitHub has **two code search systems**. Every programmatic tool (ghx, gh CLI, GitHub MCP, Octocode) hits the same legacy one. The new one is web-only.

## The Two Systems

### Legacy Code Search (REST API — what we use)

Endpoint: `GET /search/code`
Docs: [Searching code (legacy)](https://docs.github.com/en/search-github/searching-on-github/searching-code)

**Qualifiers:**
| Qualifier | Example | Notes |
|---|---|---|
| `in:file,path` | `octocat in:file` | Restrict to file contents or path |
| `user:` | `user:defunkt extension:rb` | All repos of a user |
| `org:` | `org:github extension:js` | All repos of an org |
| `repo:` | `repo:mozilla/shumway extension:as` | Specific repo |
| `path:` | `form path:cgi-bin` | Directory or file path |
| `language:` | `display language:scss` | GitHub linguist detection |
| `size:` | `function size:>10000` | File size in bytes |
| `filename:` | `filename:linguist` | Exact filename match |
| `extension:` | `form extension:pm` | File extension |

**Limitations:**
- Default branch only
- Files < 384 KB only
- Must include at least one search term (can't search qualifiers alone)
- Repos with < 500,000 files only
- Repos must have recent activity or recent search results
- Forks: only if more stars than parent + at least one commit
- Max 2 fragments per file in results
- Wildcard chars `. , : ; / \ ' " = * ! ? # $ & + ^ | ~ < > ( ) { } [ ] @` are ignored
- No `OR`, no `NOT`, no regex, no `symbol:`

**Multi-word behavior:**
- Unquoted: AND matching (both terms must appear in file, anywhere)
- Quoted: exact phrase (terms must be adjacent)
- `gh search code` silently wraps in quotes → exact phrase (see ADR-0002 Issue 2)
- `ghx search` sends unquoted → AND matching (strictly better default)

### New GitHub Code Search (Web UI only — Blackbird)

Docs: [About GitHub Code Search](https://docs.github.com/en/search-github/github-code-search/about-github-code-search)
Syntax: [Code search syntax](https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax)

**Additional features (NOT available via API):**
- Boolean: `OR`, `NOT`
- Regex
- `symbol:` qualifier (function/class definitions)
- Better indexing (new repos appear faster)

**Limitations:**
- Files < 350 KB (slightly smaller than legacy)
- Lines > 1,024 chars truncated
- No sorting
- No exhaustive search
- Default branch only
- No API access — web UI only

**Implication:** No programmatic tool can match the web UI's capabilities. `ghx search` is already using the best available API.

## Current State of `ghx search`

```bash
search)
  query="$*"
  gh api /search/code --method GET -f q="$query" \
    --jq '.items[] | "\(.repository.full_name) \(.path):\(.name)"'
```

**What's good:**
- AND matching by default (better than `gh search code`'s exact phrase)
- Compact output (repo + path per line)

**What's missing:**
1. No matching lines — returns only paths, less useful than `gh search code` which shows fragments
2. No query validation — invalid qualifiers silently degrade
3. No `text_matches` — the API supports returning matching code fragments via `Accept: application/vnd.github.text-match+json`

## Design: Best-in-Class Search

### Principle: Intuitive defaults, highest-impact value, minimal tokens

Every design choice optimizes for: **an agent reads the output and immediately knows what to do next.**

### Decision 1: Default output includes first matching fragment

Add `Accept: application/vnd.github.text-match+json` header. Default output:

```
owner/repo path/to/file.ext: matching line content here
owner/repo path/to/other.ext: another matching line
```

One line per result. Path + first match fragment. Agent sees WHERE and WHAT matched.

Verbose mode (`--verbose` or `-v`) shows all fragments.

**Rationale:** Path-only output forces a follow-up read. Path + match lets agents decide if the file is relevant without reading it. Saves 1 API call per irrelevant result.

### Decision 2: Query validation

Before sending, check for qualifiers that don't work in the legacy REST API:

| Invalid | Suggest | Why |
|---|---|---|
| `symbol:` | (remove, web-only) | Not in REST API |
| `NOT` | (remove, web-only) | Not in REST API |
| `OR` | (remove, web-only) | Not in REST API |
| `/regex/` | (remove, web-only) | Not in REST API |

Also warn on common mistakes from the legacy qualifier set:
- `filename:X` works but is often confused with `path:X` — `filename:` matches exact name, `path:` matches anywhere in path

Print warning to stderr so agents see it but output stays clean for piping.

### Decision 3: Result count in stderr

```
# stderr: "12 results (showing 30)"
# stdout: compact results
```

Agent knows if it needs to refine the query or if results are exhaustive.

### Decision 4: Token budget awareness

REST API returns max 100 results per page, 1000 total. Default to first page (30 results). With matching fragments, this is enough for agents to triage.

No pagination by default — if 30 results aren't enough, the query needs refining, not more pages.

## Implementation Plan

1. Add `Accept: application/vnd.github.text-match+json` header
2. Extract first `text_matches[0].fragment` per result
3. Add result count to stderr
4. Add query validation (warn on web-only qualifiers)
5. Update SKILL.md with correct qualifier reference
6. Benchmark: compare token output before/after

## Consequences

**Positive:**
- Agents get actionable results in one call (path + what matched)
- Invalid queries caught before wasting API calls
- Token-efficient: one line per result, no pagination bloat

**Negative:**
- Slightly more complex jq expression
- text_matches header may increase API response size (but we only extract first fragment)

**Accepted trade-offs:**
- We can't match the web UI's capabilities (OR, NOT, regex, symbol:) — platform limitation
- We optimize for the legacy API we have, not the API we wish existed
