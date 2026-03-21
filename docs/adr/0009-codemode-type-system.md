# ADR-0009: Codemode Type System — Stub Generation, Language Strategy, and Token Budget

**Date**: 2026-03-21
**Status**: Accepted
**Parent**: ADR-0008

## Purpose

ADR-0008 chose goja + esbuild as the script runtime. This ADR defines how type information flows from Go tool definitions to the LLM's prompt context, what language the LLM writes, and how we keep the token cost negligible.

---

## Why Types Matter for Codemode

Codemode without types is just `callTool("name", {arbitrary json})` — barely better than JSON tool calling. The accuracy win comes from constraining the LLM's output space with typed declarations:

```typescript
// Without types: LLM guesses argument names, return shapes, makes mistakes
const r = await callTool("explore", { repository: "foo/bar" })  // wrong arg name

// With types: LLM sees the contract, gets it right
declare const codemode: {
  explore: (input: { repo: string }) => Promise<{ branch: string; files: string[]; readme: string }>
}
const r = await codemode.explore({ repo: "foo/bar" })  // correct
r.files.filter(f => f.endsWith(".go"))                  // knows .files exists
```

Stainless calls this "SDK code mode delivers state-of-the-art accuracy." Cloudflare's entire `@cloudflare/agents` codemode package is built around this pattern.

---

## Decision: TypeScript Declarations as Prompt Context, JS Output

### The pattern (proven by Cloudflare)

```
Go function signatures
  → Type generator produces TS declarations (.d.ts style)
    → Declarations injected into LLM prompt as context
      → LLM writes plain JavaScript (not TypeScript)
        → esbuild(LoaderTS) transpiles (handles both TS and JS input)
          → goja executes
```

Cloudflare's `CODE_DESCRIPTION` prompt explicitly says:

> "Write an async arrow function in JavaScript that returns the result.
> Do NOT use TypeScript syntax — no type annotations, interfaces, or generics."

The types exist in the prompt as documentation. The LLM reads them for structure but writes plain JS. If an LLM writes TS anyway (they sometimes do), esbuild strips the annotations — no failure mode either way.

### Why not ask the LLM to write TypeScript?

- LLMs occasionally produce invalid TS (wrong generic syntax, missing imports for types they reference)
- Type errors in LLM output create a failure mode that doesn't exist with plain JS
- The types serve their purpose in the prompt — they don't need to be in the output
- esbuild doesn't type-check, it only strips annotations. Invalid TS that happens to parse still produces wrong JS.

### Why not skip types entirely and just use JSDoc comments?

- TS declarations are more compact than JSDoc equivalents
- LLMs are trained on millions of `.d.ts` files — they understand the format natively
- JSDoc `@param` / `@returns` is verbose and less structured
- Cloudflare, Stainless, and Lattice all converged on TS declarations independently

---

## Type Stub Generation

### Input: Go function signatures + MCP tool schemas

Each registered tool has:
- A Go function (with typed parameters and return values)
- An MCP JSON Schema (generated from the Go types via reflect, per ADR-0008 §4)

### Output: TypeScript declarations

For ghx's 5 tools:

```typescript
type ExploreInput = { repo: string }
type ExploreOutput = { branch: string; files: string[]; readme: string }

type ReadInput = { repo: string; files: string[]; grep?: string; lines?: string; map?: boolean }
type ReadOutput = { name: string; content: string; size: number }[]

type SearchInput = { query: string; limit?: number }
type SearchOutput = { total: number; results: { repo: string; file: string; line: string }[] }

type ReposInput = { query: string; limit?: number }
type ReposOutput = { name: string; description: string; stars: number; language: string }[]

type TreeInput = { repo: string; path?: string }
type TreeOutput = { entries: string[] }

declare const codemode: {
  /** Explore a GitHub repo — returns branch, file tree, and README */
  explore: (input: ExploreInput) => Promise<ExploreOutput>;
  /** Read files from a repo with optional grep, line range, or structural map */
  read: (input: ReadInput) => Promise<ReadOutput>;
  /** Search code across GitHub (AND matching) */
  search: (input: SearchInput) => Promise<SearchOutput>;
  /** Search repositories by name/description */
  repos: (input: ReposInput) => Promise<ReposOutput>;
  /** List directory tree of a repo */
  tree: (input: TreeInput) => Promise<TreeOutput>;
}
```

### Token cost: ~240 tokens for 5 tools

| Tool | Input + Output types | ~Tokens |
|------|---------------------|---------|
| explore | flat input, 3-field output | ~40 |
| read | 5-field input, array output | ~60 |
| search | 2-field input, nested output | ~50 |
| repos | 2-field input, array output | ~50 |
| tree | 2-field input, flat output | ~40 |
| **Total** | | **~240** |

For context: the CE-MCP paper showed codemode saves ~98% of tokens on execution (collapsing N tool calls into one script). Adding 240 tokens of type stubs to the prompt is a rounding error.

### Token budget cap

Cloudflare uses `MAX_TOKENS = 6000` for type stubs. We adopt the same cap. At 240 tokens for 5 tools, we're at 4% of budget. Even at 50 tools with complex nested schemas, the cap prevents bloat.

If stubs exceed the budget: truncate descriptions first, then collapse nested types to `Record<string, unknown>`. Tool signatures (name + top-level args) are never truncated — they're the minimum the LLM needs.

---

## Generator Implementation

### Pipeline: Go types → JSON Schema → TypeScript declarations

```
Go struct tags + reflect
  → JSON Schema (standard MCP tool inputSchema)
    → TypeScript type string (jsonSchemaToType)
      → Assembled into `declare const codemode: { ... }`
```

We port Cloudflare's `generateTypesFromJsonSchema` approach to Go. Their implementation (~300 lines of TS) handles:
- `$ref` resolution (internal JSON pointers)
- `anyOf` / `oneOf` → union types
- `allOf` → intersection types
- `enum` → string literal unions
- Nested objects with required/optional fields
- Arrays with item types
- JSDoc comments from `description` fields
- Property name escaping

Our Go port will be simpler — ghx's tools use flat structs with primitive types. We start with:
- Object types with required/optional fields
- Arrays with typed items
- Primitive types (string, number, boolean)
- JSDoc from description fields

Add `$ref`, unions, intersections later if a consumer needs them.

### Generation timing

- Generated when MCP server starts (tool set is known)
- Cached — regenerated only if tool set changes
- Injected into the `code` tool's description field (the LLM sees it in the tool schema)
- Also available via a `types` resource for agents that want to fetch stubs separately

---

## esbuild Loader Configuration

```go
result := api.Transform(code, api.TransformOptions{
    Target: api.ES2015,
    Loader: api.LoaderTS,  // Accepts both TS and JS input
    Format: api.FormatDefault,
})
```

`LoaderTS` vs `LoaderJS`:
- `LoaderTS` strips type annotations, then transpiles. Accepts both TS and JS.
- `LoaderJS` rejects type annotations as syntax errors.

Using `LoaderTS` makes the executor accept both languages. No failure mode if the LLM writes TS despite being asked for JS.

### Proven in production

| Project | Loader | Target | Runtime |
|---------|--------|--------|---------|
| pocketci | `LoaderTS` | ES2017 | goja |
| codezone | `LoaderTS` | ESNext | goja (with Node fallback) |
| gridctl | `LoaderJS` | ES2015 | goja |

---

## What the LLM Prompt Looks Like

Assembled from Cloudflare's pattern, adapted for ghx:

```
Execute code to achieve a goal.

Available:

type ExploreInput = { repo: string }
type ExploreOutput = { branch: string; files: string[]; readme: string }
...

declare const codemode: {
  /** Explore a GitHub repo — returns branch, file tree, and README */
  explore: (input: ExploreInput) => Promise<ExploreOutput>;
  ...
}

Write an async arrow function in JavaScript that returns the result.
Do NOT use TypeScript syntax — no type annotations, interfaces, or generics.

Example:
async () => {
  const result = await codemode.explore({ repo: "vercel/next.js" });
  return result.files.filter(f => f.endsWith(".ts"));
}
```

---

## Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Type format | TypeScript declarations (.d.ts style) | LLMs trained on millions of .d.ts files, most compact structured format |
| LLM output language | Plain JavaScript (TS accepted as fallback) | Avoids type-error failure mode, types serve their purpose in prompt context |
| esbuild loader | `LoaderTS` | Accepts both TS and JS, strips annotations, no failure mode |
| Token budget | 6,000 token cap (Cloudflare's number) | ghx uses ~240 tokens (4% of budget), scales to 50+ tools |
| Generation source | Go types → JSON Schema → TS declarations | Standard MCP pipeline, no custom schema format |
| Generation timing | Server startup, cached | Fast (~1ms for 5 tools), always in sync |
| Truncation strategy | Descriptions first, then nested types → `Record<string, unknown>` | Preserves tool signatures (minimum LLM needs) |

## Prior Art

| Project | Approach | Key insight |
|---------|----------|-------------|
| [Cloudflare @cloudflare/agents](https://github.com/cloudflare/agents) | `generateTypesFromJsonSchema` — 300 lines, JSON Schema → TS declarations, 6K token cap | TS declarations as prompt context, JS output. The reference implementation. |
| [Stainless SDK code mode](https://www.stainless.com) | Typed SDK stubs for API clients | "SDK code mode delivers state-of-the-art accuracy" — types are the accuracy layer |
| [pocketci](https://github.com/jtarchie/pocketci) | `LoaderTS` + goja in production | Proves TS→esbuild→goja pipeline works |
| [codezone](https://github.com/sklymoshenko/codezone) | `LoaderTS` + goja with Node fallback | TypeScript executor with graceful degradation |
| [gridctl](https://github.com/gridctl/gridctl) | `LoaderJS` + esbuild + goja | Production codemode, JS-only but same transpilation pipeline |
