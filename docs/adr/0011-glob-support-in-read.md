---
title: "Glob Pattern Support in Read"
date: 2026-04-01
status: Accepted
---

# 0011. Glob Pattern Support in Read

## Context

Agents write `ghx read repo "src/**/*.ts" --map` expecting it to work like every local filesystem tool they've been trained on. It doesn't — GitHub's API requires exact file paths. The current behavior is silent failure: each glob pattern is sent as a literal path to GraphQL `object(expression: "HEAD:src/**/*.ts")`, which returns null, and ghx reports "not found." The agent can't distinguish "file doesn't exist" from "I used a glob and the API doesn't support it."

### Source of Truth: GitHub API

- **REST Contents API** (`GET /repos/{owner}/{repo}/contents/{path}`): `path` is a literal file or directory path. No glob, no wildcard, no pattern matching. Confirmed from [official docs](https://docs.github.com/en/rest/repos/contents).
- **GraphQL** (`object(expression: "HEAD:path")`): Same — literal path to a blob or tree object. Returns null for anything that isn't an exact match.
- **Git Trees API** (`GET /repos/{owner}/{repo}/git/trees/{sha}?recursive=1`): Returns the full recursive tree as a flat list of path strings. This is the only API that gives us all paths in one call.

### Prior Art: glob-github (npm)

The [`glob-github`](https://github.com/mixu/glob-github) package solves this by implementing a virtual filesystem over the REST Contents API:
- `readdir` calls `repos.getContent(path)` per directory
- Results are cached to avoid re-fetching
- A glob library (`wildglob`) walks the virtual FS
- **Cost: N API calls** (one per directory level traversed)

### ghx Advantage

ghx already has `Tree()` which fetches the entire recursive tree via the Git Trees API in **one REST call**. The tree comes back as a flat list of paths — exactly what a glob matcher needs. No virtual filesystem, no per-directory traversal.

### The 10-File Limit

`Read()` uses GraphQL aliases to batch up to 10 files per call. This is a GraphQL complexity constraint. Glob expansion must respect this — if a glob matches 50 files, we can only read 10. This is fine for the primary use case (`--map` on a handful of files), and the limit should be communicated clearly in output.

## Proposed Solutions

### Option 1: Glob Expansion in `Read()` (Transparent)

Detect glob characters in file paths inside `Read()`. If any path contains `*`, `?`, `[`, or `{`:
1. Fetch tree (one REST call)
2. Match glob against tree entries using `doublestar.Match()`
3. Read matching files (one GraphQL call, max 10)

**Pros:**
- Transparent — works in CLI, MCP, and codemode without changes to callers
- Two API calls total (tree + batched read)
- Agent writes `ghx read repo "src/**/*.ts" --map` and it just works
- All existing flags (`--grep`, `--lines`, `--map`) compose with globs

**Cons:**
- Tree fetch adds latency (~200-500ms) even when glob matches few files
- Mixes concerns — `Read()` now does tree fetching
- 10-file limit means large globs silently truncate

**Complexity:** Low — ~30 lines of new code in `Read()`, one new dependency (`doublestar`)

### Option 2: Separate `Read()` and `GlobRead()` Functions

Keep `Read()` for exact paths. Add `GlobRead()` that does tree + match + read. CLI detects globs and routes to the right function.

**Pros:**
- Clean separation of concerns
- `Read()` stays simple and fast
- Explicit about when tree fetch happens

**Cons:**
- Three entry points (CLI, MCP, codemode) all need glob detection logic
- Duplicated routing code
- Agent-facing behavior is identical to Option 1

**Complexity:** Medium — more code, same result

### Option 3: No Glob Support, Error on Glob Characters

Reject glob patterns with a clear error message pointing to `ghx tree`.

**Pros:**
- Zero new code, zero new dependencies
- Forces agents to use the explicit tree → read workflow
- No hidden API calls

**Cons:**
- Agents will keep trying globs because every other tool supports them
- Two-step workflow (tree then read) costs more agent round-trips and tokens
- Doesn't match agent expectations — fights training data instead of working with it

**Complexity:** Trivial

## Decision

**Selected: Option 1 — Transparent glob expansion in `Read()`**

### Rationale

- The whole point of ghx is "one command does what takes 3-5 API calls." Glob expansion is exactly that — tree + read in one call instead of two separate commands.
- Agents are trained on glob patterns. Fighting that with error messages is a losing battle (same lesson as `--grep` and BRE — align with what agents expect).
- Two API calls is not crazy. `explore` already does 1-2 calls. `repos` does batched GraphQL. This is consistent with ghx's design philosophy.
- The 10-file limit is a natural guardrail. Agents using `--map` (the primary glob use case) get 92% token reduction per file, so 10 files via `--map` is plenty.
- `doublestar` is battle-tested: 285K+ downloads, supports `**`, `{alts}`, character classes — the full glob spec agents expect.

### Trade-offs Accepted

- Extra API call (tree fetch) when globs are used — acceptable, ~200-500ms
- 10-file cap on glob matches — acceptable, matches existing `Read()` limit, clearly communicated
- New dependency (`doublestar`) — acceptable, well-maintained, zero transitive deps

## Implementation Notes

### Architecture

```
pkg/ghx/
├── glob.go       — fetchTree(), isGlob(), expandGlobs()  [shared infrastructure]
├── tree.go       — Tree()  [refactored to use fetchTree()]
├── read.go       — Read()  [glob expansion as preprocessing, then existing logic]
├── glob_test.go  — 12 test cases for glob expansion
└── read_test.go  — 19 test cases for grep/BRE normalization
```

- `fetchTree(repo)` extracted from `Tree()` into `glob.go` as shared helper — DRY, used by both `Tree()` and `Read()`
- `tree.go` reduced from 92 to 45 lines by delegating to `fetchTree()`
- Glob expansion is a ~20-line preprocessing block at the top of `Read()` — the rest of the function is untouched
- `GlobPattern` field added to `FileResult` so callers can show provenance without changing the return type

### Glob Detection

`isGlob(path)` checks for `*`, `?`, `[`, `{` via `strings.ContainsAny`. If ANY path in the files list is a glob, fetch the tree once and expand all globs. Non-glob paths pass through unchanged — zero overhead for exact paths.

### Tree Fetch

`fetchTree(repo)` returns `[]treeEntry` (path + type). Two API calls: GraphQL for default branch name, REST Git Trees API with `?recursive=1` for full tree. Same calls `Tree()` was already making, now shared.

### Matching

`doublestar.Match(pattern, path)` against each blob entry (tree entries filtered out — globs match files, not directories). Results are deduplicated and capped at 10 (existing `Read()` limit).

### Grep + Glob Composition

When `--grep` is combined with globs, files with zero grep matches are silently skipped in CLI output. This matches `grep -r --include='*.ts' pattern` behavior — 50 years of convention: only show files with matches.

### Entry Points

All three entry points (CLI, MCP, codemode) call `Read()` — glob expansion happens inside `Read()` transparently. No caller changes needed.

### Dependencies

- `github.com/bmatcuk/doublestar/v4` — battle-tested glob matcher (285K+ downloads, zero transitive deps). Supports `**`, `{alts}`, `[classes]`, `?`.
