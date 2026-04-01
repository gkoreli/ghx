---
title: "GraphQL Underutilized Fields"
date: 2026-04-01
status: Accepted
---

# 0012. GraphQL Underutilized Fields

## Context

A real agent session exposed that `ghx read repo dir/path --map` returns "not found" when the path is a directory. The root cause: our GraphQL query only asks for `... on Blob { text byteSize }`. When the path resolves to a Tree (directory), the Blob fragment matches nothing, the response is nil, and we report `notFound: true`. The agent can't distinguish "file doesn't exist" from "this is a directory."

This was fixed by adding `... on Tree { entries { name type } }` to the same query — zero extra API calls, the data was always available.

Investigation of the full GitHub GraphQL schema revealed this is a pattern: we're leaving significant data on the table across every query we make.

### Source of Truth: GitHub GraphQL Schema Introspection

All fields below were verified via `__type` introspection queries and live tested against real repositories (fredrikalindh/ui, astral-sh/uv, facebook/react, cli/cli, kubernetes/kubernetes).

## Findings

### Category 1: Silent Wrong Behavior (Bugs)

Same class as the directory fix — the API tells us something important and we ignore it.

#### `isBinary` on Blob

- **Current behavior**: Binary files (images, compiled output) return `text: null`. We fall through to `notFound: true` or return empty content. Indistinguishable from "file doesn't exist."
- **Fix**: Add `isBinary` to the Blob fragment. When true, return a clear signal: "binary file, N bytes."
- **Verified**: `docs/assets/github-add-environment.png` in astral-sh/uv returns `{ isBinary: true, byteSize: 84757 }`.

#### `isTruncated` on Blob

- **Current behavior**: GitHub silently truncates `text` for files >100KB. We return partial content with no warning. The agent believes it read the complete file.
- **Fix**: Add `isTruncated` to the Blob fragment. When true, warn that content is incomplete.
- **Verified**: `yarn.lock` (823KB) in facebook/react returns `{ isTruncated: true }`. Agent currently gets partial content with zero indication.
- **This is arguably worse than the directory bug** — it's silently wrong data, not just a missing result.

### Category 2: Free Metadata on Existing Queries

Zero extra API calls. These are fields on types we already query.

#### `size` and `lineCount` on TreeEntry

- **Available on**: Every file entry returned by `explore` and directory listings.
- **Value**: Agent can estimate token cost before reading. "Should I `--map` this 926-line CSS file or skip it entirely?"
- **Verified**: `index.tsx` in fredrikalindh/ui returns `{ size: 5689, lineCount: 190 }`. `theme.css` returns `{ size: 26239, lineCount: 926 }`. Directories return `{ size: 0, lineCount: null }`.

#### Repo Metadata on Repository

- **Fields**: `stargazerCount`, `forkCount`, `updatedAt`, `createdAt`, `isArchived`, `isEmpty`, `diskUsage`, `primaryLanguage { name }`, `licenseInfo { spdxId name }`
- **Available on**: The `explore` query already hits `repository(owner, name)`. These are top-level fields on the same object.
- **Value**: "Is this project alive? Is it maintained? What license?" — currently requires a separate web fetch that often fails (npm 403'd in the session that triggered this investigation).
- **Verified**: fredrikalindh/ui returns `{ stargazerCount: 58, forkCount: 4, createdAt: "2025-10-27", updatedAt: "2026-04-01", isArchived: false, primaryLanguage: "TypeScript", licenseInfo: { spdxId: "MIT" } }`.

#### Last Commit via `defaultBranchRef.target`

- **Current behavior**: We fetch `defaultBranchRef { name }` — just the branch name.
- **Available**: `target { ... on Commit { committedDate messageHeadline author { name } } }` — last commit date, message, and author. Free on the same query.
- **Verified**: Returns `{ committedDate: "2026-03-26", messageHeadline: "Interviewing post: remove Midjourney from offer claims (#6)", author: { name: "fredrikalindh" } }`.

#### `isGenerated` on TreeEntry

- **Value**: GitHub marks generated files (lockfiles, compiled output, vendored code). Could mark or filter these in explore/tree output to reduce noise.
- **Verified**: Returns `false` for source files, would flag lockfiles and generated code.

## Decision

### Implemented (this session)

- **Directory detection in `read`**: Added `... on Tree { entries { name type } }` to the read query. When a path is a directory, returns `dirEntries` with file listing and an actionable hint instead of "not found."

### To Implement

Priority order based on impact:

1. **`isBinary` + `isTruncated` on Blob** — bug fixes, same class as directory detection. Add to the existing Blob fragment in `read`. Surface in `FileResult` and CLI output.
2. **`size` + `lineCount` on TreeEntry** — add to `explore` and directory listing responses. Helps agents make read/map/skip decisions.
3. **Repo metadata in `explore`** — add `stargazerCount`, `forkCount`, `updatedAt`, `createdAt`, `isArchived`, `primaryLanguage`, `licenseInfo` to the explore query. Eliminates the need for a separate `ghx info` command.
4. **Last commit in `explore`** — extend `defaultBranchRef` with `target { ... on Commit { committedDate messageHeadline author { name } } }`.
5. **`isGenerated` on TreeEntry** — lowest priority, nice-to-have for noise filtering.

### Not Doing

- **Separate `ghx info` command** — repo metadata belongs in `explore`, not a new command. One query already hits the Repository type.

## Consequences

- **Positive**: Every fix is zero additional API calls. We're extracting more value from queries we already make.
- **Positive**: Bug fixes (binary, truncated) eliminate silent wrong behavior that agents can't detect or recover from.
- **Positive**: Metadata (size, lineCount) enables agents to make better decisions about what to read, reducing wasted round-trips.
- **Negative**: Slightly larger GraphQL queries and response payloads. Negligible — a few extra fields on objects we already fetch.
- **Risk**: GitHub could deprecate or change field semantics. Low risk — these are stable, well-documented schema fields.

## Implementation Notes

- All changes are additive to existing GraphQL fragments — no new queries needed.
- `FileResult` struct gains `IsBinary bool` and `IsTruncated bool` fields.
- `ExploreResult` struct gains repo metadata fields.
- `FileEntry` struct gains `Size int` and `LineCount *int` (nullable for directories).
- Codemode type stubs in `register.go` must be updated for each new field.
- CLI output formatting in `cmd/ghx.go` needs rendering for new fields.
