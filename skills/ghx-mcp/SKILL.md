---
name: ghx-mcp
description: "Use when exploring GitHub repositories through the ghx MCP server: choose direct tools for simple lookups, or the code meta-tool for composed reconnaissance."
version: 1.2.1
author: ghx contributors
license: MIT
metadata:
  hermes:
    tags: [github, code-exploration, mcp, codemode, repository-recon]
    related_skills: []
---

# ghx MCP — GitHub Code Exploration via MCP

> **This is the power-user/tool layer** — for driving exploration yourself.
> If you just want answers about a repo, connect to `ghx serve --recon`
> instead: it exposes a single `recon(question, repo?, session?)` tool that
> returns an evidence report. Normally omit `session`: the daemon routes each
> question, reports `session: <name> (routed: <rule>)`, and keeps follow-ups on
> the right investigation thread. Use `session` only as an advanced pin.
> `ghx skill --recon` prints the concise skill. You then need zero knowledge of
> the tools below. (One-time setup for the recon tool:
> `ghx sidecar config init --claude-acp` then `ghx sidecar doctor`; artifacts
> land under `~/.ghx/sessions/`; fix a mis-route with
> `ghx sidecar sessions reroute <session> <turn> <dest>`.)

7 tools for GitHub exploration (served by `ghx serve`). 5 direct tools for simple queries. 1 `code` meta-tool for complex multi-step operations. 1 `search_tools` for discovery.

## Tools

### Direct Tools (simple one-shot queries)

| Tool | Input | What it does |
|------|-------|-------------|
| `explore` | `repo` (required), `path` | Branch, file tree, README in 1 API call |
| `read` | `repo` + `paths` (required), `grep`, `lines`, `map`, `level`, `kind`, `mapEngine` | Read 1-10 files in 1 API call. Glob patterns supported. Directory paths return file listing. `map` = parser-backed structural map (Go→go/ast, TS/JS/Python/Rust→Tree-sitter, else→regex). `kind`: `func`/`type`/`import`/`const`/`var`. `level: "minimal"` shows `Parent.Method` names only |
| `search` | `query` (required), `limit`, `full` | Code search with AND matching + matching lines |
| `repos` | `query` (required), `limit` | Search repos with README preview |
| `tree` | `repo` (required), `path`, `depth` | File tree listing (default: all files, no depth limit. With depth: includes dirs with /) |

### Meta-Tools (compose operations)

| Tool | Input | What it does |
|------|-------|-------------|
| `code` | `code` (required) | Execute JS that calls any combination of the 5 tools above |
| `search_tools` | `query` (optional) | List available tools with TypeScript type stubs |

## When to Use `code` vs Direct Tools

**Direct tools** — simple, single-operation queries:
```
explore({ repo: "vercel/next.js" })              → one call, one result
read({ repo: "vercel/next.js", paths: "README.md" })  → one call, one result
```

**`code` tool** — multi-step, filtering, conditional logic:
```javascript
// Explore → filter → read in ONE round-trip (not three)
var repo = codemode.explore({ repo: "vercel/next.js" });
var tsFiles = repo.files.filter(f => f.name.endsWith(".ts")).slice(0, 5);
var contents = codemode.read({ repo: "vercel/next.js", files: tsFiles.map(f => f.name), map: true });
return contents;
```

**Rule of thumb:** If you need the result of tool A to decide what to call for tool B → use `code`. Otherwise use direct tools.

## The `code` Tool

### Available API

```typescript
type ExploreInput = { repo: string; path?: string }
type ReadInput = { repo: string; files: string[]; grep?: string; lines?: string; map?: boolean; level?: "outline" | "minimal" | "compact" | "standard"; kind?: "func" | "type" | "import" | "const" | "var" | "package"; mapEngine?: "auto" | "regex" | "tree-sitter" }
type ReposInput = { query: string; limit?: number }
type SearchInput = { query: string; limit?: number; fullMode?: boolean }
type TreeInput = { repo: string; path?: string; depth?: number }

declare const codemode: {
  explore: (input: ExploreInput) => { description: string; branch: string; files: { name: string; type: string }[]; readme: string };
  read: (input: ReadInput) => { path: string; content: string; byteSize: number; notFound: boolean; dirEntries?: { name: string; type: string }[]; globPattern?: string; grepHits?: { lineNum: number; line: string; isMatch: boolean }[]; mapLines?: string[]; mapChars?: number; mapEngine?: string; mapWarnings?: string[] }[];
  repos: (input: ReposInput) => { results: { nameWithOwner: string; description: string; stars: number; language: string; readmePreview: string }[]; total: number };
  search: (input: SearchInput) => { total: number; incomplete: boolean; matches: { repo: string; path: string; fragment: string }[] };
  tree: (input: TreeInput) => string[];
}
```

Use `search_tools` to get the latest type stubs at runtime.

### Writing Code

```javascript
// ✅ Correct: plain JavaScript, synchronous calls, return a value
var repo = codemode.explore({ repo: "dop251/goja" });
return { branch: repo.branch, fileCount: repo.files.length };

// ❌ Wrong: await (tools are synchronous, await causes transpile error)
const r = await codemode.explore(...)  // top-level await not supported

// ❌ Wrong: TypeScript syntax
const r: ExploreResult = codemode.explore(...)  // no type annotations

// ❌ Wrong: no return value
codemode.explore({ repo: "dop251/goja" });  // result is lost
```

### Rules

- Write plain JavaScript, not TypeScript (no type annotations, interfaces, generics)
- `codemode.*` calls are synchronous — do NOT use `await`
- Must `return` a value — the return value is what you see in the response
- `console.log()` output appears in the response under "Console:" (useful for debugging)
- Max 20 tool calls per execution
- Max 64KB code size
- Response truncated at 24K chars (use `map: true` or `grep` to reduce output)

### Patterns

**Explore and filter:**
```javascript
var repo = codemode.explore({ repo: "owner/repo" });
var goFiles = repo.files.filter(f => f.name.endsWith(".go"));
return goFiles.map(f => f.name);
```

**Map multiple files (understand structure before reading):**
```javascript
var repo = codemode.explore({ repo: "owner/repo" });
var srcFiles = repo.files.filter(f => f.name.startsWith("src/") && f.type === "blob").slice(0, 8);
return codemode.read({ repo: "owner/repo", files: srcFiles.map(f => f.name), map: true });
```

**Search then read matches:**
```javascript
var hits = codemode.search({ query: "GraphQL repo:owner/repo", limit: 5 });
var files = hits.results.map(r => r.path);
return codemode.read({ repo: "owner/repo", files: files });
```

**Conditional logic:**
```javascript
var repo = codemode.explore({ repo: "owner/repo" });
var hasGo = repo.files.some(f => f.name === "go.mod");
var hasPython = repo.files.some(f => f.name === "requirements.txt");
if (hasGo) {
  return { lang: "go", mod: codemode.read({ repo: "owner/repo", files: ["go.mod"] }) };
} else if (hasPython) {
  return { lang: "python", reqs: codemode.read({ repo: "owner/repo", files: ["requirements.txt"] }) };
}
return { lang: "unknown", files: repo.files.slice(0, 10) };
```

## Chain of Thought

**Don't follow a sequence. Pick the right starting point based on what you already know.**

**Know what you're looking for?** Start with `search` or direct `read`:
```
search({ query: "pattern repo:owner/repo" })              → Find files by content
read({ repo: "...", paths: "src/utils", map: true })       → Works for files AND directories
read({ repo: "...", paths: "src/**/*.ts", map: true })     → Glob to scan many files at once
```

**Don't know the repo at all?** Start with `explore`:
```
explore({ repo: "owner/repo" })                            → Structure + README (orientation)
```

**Then drill in — map before reading, grep before full read:**
```
read({ repo: "...", paths: "f1,f2", map: true })           → Signatures of many files (~92% fewer tokens; measured: a 15.3 KB Go file maps to 0.9 KB)
read({ repo: "...", paths: "f1", grep: "pattern" })        → Just the matching lines
read({ repo: "...", paths: "f1" })                         → Full file (only when needed)
```

`map` doesn't just save tokens — it lets you see 10 files for the cost of reading 1. Engine selection is automatic: **Go** uses `go/ast` (full multi-line signatures, no local variable noise), **TS/JS/Python/Rust** use Tree-sitter (captures class and impl methods that regex cannot reach), everything else falls back to regex. Methods carry parent context — `level: "minimal"` renders `UserService.GetUser` instead of just `GetUser`.

**When to use `code` instead of direct tools:**
- Result of tool A determines what to call for tool B → use `code` (one round-trip)
- Need to filter/transform results before returning → use `code`
- Simple single query → use direct tool

## Search Query Syntax

`search` searches **inside file contents** — use it to find code patterns, not to discover repos. To discover repos by topic, use `repos`.

Every word is AND'd — a file must contain ALL words to match. More words = fewer results, not better results. Search 1-2 terms, not 5.

**Valid qualifiers:** `repo:`, `org:`, `path:`, `filename:`, `extension:`, `language:`, `in:file`, `in:path`

`path:` filters by file path, not by "repos containing this file." `path:src` = only search files under `src/`. Use `filename:` to find files by name.

**DO NOT USE (web-only, silently wrong):** `OR`, `NOT`, `symbol:`, `content:`, `is:`, regex

**Rate limit:** 9 req/min for code search. Refine queries, don't paginate.

## Gotchas

1. **`read` paths are comma-separated.** `paths: "f1.go,f2.go"` not `paths: ["f1.go", "f2.go"]`.
2. **`read` handles directories.** If a path is a directory, returns `dirEntries` (file listing) instead of `notFound`. No wasted round-trips.
3. **Response truncation at 24K chars.** Use `map: true` or `grep` to keep results small. The `code` tool is especially prone to this when reading multiple full files.
4. **`search` uses AND matching.** `"foo bar"` finds files with both words anywhere. For exact phrase, wrap in escaped quotes: `"\"foo bar\""`.
5. **Web-only qualifiers silently degrade.** `symbol:`, `OR`, `NOT` are treated as literal text in the REST API.
6. **`code` tool: no TypeScript.** Type stubs are for your reference. Write plain JS.
7. **`code` tool: return is required.** Bare expressions don't auto-return (except simple identifiers). Always use `return`.
8. **`grep` uses ERE regex.** Use `|` for alternation, not `\|`. Example: `grep: "ref|defs|definition"`.
9. **Glob + grep skips non-matching files.** Like `grep -r --include`, only files with hits are shown.

## Anti-Patterns

- ❌ Three sequential tool calls when `code` can do it in one
- ❌ Reading full files when you need 10 lines — use `grep` or `map: true`
- ❌ Paginating broad searches — refine with `repo:`, `language:`, `path:`
- ❌ Writing TypeScript in the `code` tool — stripped by transpiler but may cause subtle issues
- ❌ Forgetting `return` in `code` — you get `undefined` back
- ❌ Reading 10 full files in `code` — hits 24K truncation. Use `map: true` first
- ❌ Broad globs like `**/*.ts` on large repos — matches thousands, reads only 10. Narrow the pattern first
