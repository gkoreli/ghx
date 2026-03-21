---
name: ghx-mcp
description: GitHub code exploration via MCP. 7 tools — 5 direct + code meta-tool + search_tools. The code tool lets you write JS programs that compose operations in one round-trip.
---

# ghx MCP — GitHub Code Exploration via MCP

7 tools for GitHub exploration. 5 direct tools for simple queries. 1 `code` meta-tool for complex multi-step operations. 1 `search_tools` for discovery.

## Tools

### Direct Tools (simple one-shot queries)

| Tool | Input | What it does |
|------|-------|-------------|
| `explore` | `repo` (required), `path` | Branch, file tree, README in 1 API call |
| `read` | `repo` + `paths` (required), `grep`, `lines`, `map` | Read 1-10 files in 1 API call. `map` = signatures only (~92% reduction) |
| `search` | `query` (required), `limit`, `full` | Code search with AND matching + matching lines |
| `repos` | `query` (required), `limit` | Search repos with README preview |
| `tree` | `repo` (required), `path` | Full recursive file tree |

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
type ReadInput = { repo: string; files: string[]; grep?: string; map?: boolean }
type ReposInput = { query: string; limit?: number }
type SearchInput = { query: string; limit?: number; fullMode?: boolean }
type TreeInput = { repo: string; path?: string }

declare const codemode: {
  explore: (input: ExploreInput) => Promise<any>;
  read: (input: ReadInput) => Promise<any>;
  repos: (input: ReposInput) => Promise<any>;
  search: (input: SearchInput) => Promise<any>;
  tree: (input: TreeInput) => Promise<any>;
}
```

Use `search_tools` to get the latest type stubs at runtime.

### Writing Code

```javascript
// ✅ Correct: plain JavaScript, return a value
var repo = codemode.explore({ repo: "dop251/goja" });
return { branch: repo.branch, fileCount: repo.files.length };

// ✅ Also correct: async arrow function (await is harmless but unnecessary)
async () => {
  const r = await codemode.explore({ repo: "dop251/goja" });
  return r.files.filter(f => f.name.endsWith(".go"));
}

// ❌ Wrong: TypeScript syntax
const r: ExploreResult = await codemode.explore(...)  // no type annotations

// ❌ Wrong: no return value
codemode.explore({ repo: "dop251/goja" });  // result is lost
```

### Rules

- Write plain JavaScript, not TypeScript (no type annotations, interfaces, generics)
- `codemode.*` calls are synchronous — `await` is accepted but not required
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

**Always start surgical, escalate only when needed.**

```
1. explore({ repo: "owner/repo" })                    → What's in this repo?
2. read({ repo: "...", paths: "f1,f2", map: true })   → What do these files define? (92% fewer tokens)
3. read({ repo: "...", paths: "f1", grep: "pattern" }) → Where exactly is X?
4. read({ repo: "...", paths: "f1" })                  → Full file (only when needed)
```

**When to escalate to `code`:**
- Step 1 result determines what to do in step 2 → use `code` (one round-trip)
- Need to filter/transform results before returning → use `code`
- Simple single query → use direct tool

## Search Query Syntax

Same as GitHub REST code search API. Multi-word = AND matching.

**Valid qualifiers:** `repo:`, `org:`, `path:`, `filename:`, `extension:`, `language:`, `in:file`, `in:path`

**DO NOT USE (web-only, silently wrong):** `OR`, `NOT`, `symbol:`, `content:`, `is:`, regex

**Rate limit:** 9 req/min for code search. Refine queries, don't paginate.

## Gotchas

1. **`read` paths are comma-separated.** `paths: "f1.go,f2.go"` not `paths: ["f1.go", "f2.go"]`.
2. **Response truncation at 24K chars.** Use `map: true` or `grep` to keep results small. The `code` tool is especially prone to this when reading multiple full files.
3. **`search` uses AND matching.** `"foo bar"` finds files with both words anywhere. For exact phrase, wrap in escaped quotes: `"\"foo bar\""`.
4. **Web-only qualifiers silently degrade.** `symbol:`, `OR`, `NOT` are treated as literal text in the REST API.
5. **`code` tool: no TypeScript.** Type stubs are for your reference. Write plain JS.
6. **`code` tool: return is required.** Bare expressions don't auto-return (except simple identifiers). Always use `return`.

## Anti-Patterns

- ❌ Three sequential tool calls when `code` can do it in one
- ❌ Reading full files when you need 10 lines — use `grep` or `map: true`
- ❌ Paginating broad searches — refine with `repo:`, `language:`, `path:`
- ❌ Writing TypeScript in the `code` tool — stripped by transpiler but may cause subtle issues
- ❌ Forgetting `return` in `code` — you get `undefined` back
- ❌ Reading 10 full files in `code` — hits 24K truncation. Use `map: true` first
