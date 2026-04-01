---
name: ghx
description: GitHub code exploration for AI agents. CLI + codemode. One command does what takes 3-5 API calls. Write JS programs that compose operations in one round-trip.
---

# ghx — GitHub Code Exploration for AI Agents

Use `ghx` via `execute_bash` for anything on GitHub — repos, files, code search, codemode. Authenticated via `gh` CLI, structured output, zero context overhead.

## Commands

```bash
ghx explore <owner/repo>                    # Branch + tree + README in 1 API call
ghx explore <owner/repo> <path>             # Subdirectory listing
ghx read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files in 1 API call (GraphQL batching)
ghx read <owner/repo> --map <f1> [f2]       # Structural map: signatures, imports, types (~92% token reduction)
ghx read <owner/repo> --grep "pat" <f>      # Read file, show only matching lines (2 lines context, regex)
ghx read <owner/repo> --lines 42-80 <f>     # Read specific line range
ghx repos "<query>"                         # Search repos with README preview in 1 GraphQL call
ghx search "<query>"                        # Code search (AND matching, shows matching lines)
ghx search --full "<query>"                 # Code search without line truncation
ghx tree <owner/repo> [path]                # Full recursive tree (default: all files, no depth limit)
ghx tree <owner/repo> [path] --depth N      # Tree limited to N levels (includes dirs with /)
ghx code "<js>"                             # Execute JS with access to all ghx tools
ghx code -                                  # Read code from stdin
ghx code --list                             # List available tools with type stubs
```

**Exit codes:** 0 = success, 1 = no results, 2 = usage error.

## Chain of Thought: Progressive Disclosure

**Start surgical, escalate only when needed.**

```
1. ghx explore owner/repo          → What's in this repo? (structure + README)
2. ghx read owner/repo --map *.ts  → What do these files define? (signatures only, 92% fewer tokens)
3. ghx read owner/repo --grep "X" f → Where exactly is X in this file? (targeted lines)
4. ghx read owner/repo f            → Show me the full file (only when needed)
```

At 92% reduction, `--map` lets you scan 7 files in the space of reading 1 full file.

## When to Use `ghx code`

Use direct commands for simple one-shot queries. Use `ghx code` when you need to:
- **Chain operations** — explore → filter → read in one shot
- **Filter results** — JS logic runs locally, not in the LLM
- **Conditional logic** — read different files based on what you find

```bash
# Simple: use direct command
ghx explore vercel/next.js

# Complex: use ghx code (one round-trip instead of three)
ghx code "
  var repo = codemode.explore({ repo: 'vercel/next.js' });
  var goFiles = repo.files.filter(f => f.name.endsWith('.go'));
  var contents = codemode.read({ repo: 'vercel/next.js', files: goFiles.map(f => f.name) });
  return contents;
"
```

### Codemode API

```bash
ghx code --list   # See all available tools with type stubs
```

```typescript
declare const codemode: {
  explore: (input: { repo: string; path?: string }) => { description: string; branch: string; files: { name: string; type: string }[]; readme: string };
  read: (input: { repo: string; files: string[]; grep?: string; map?: boolean }) => { path: string; content: string; byteSize: number; notFound: boolean }[];
  repos: (input: { query: string; limit?: number }) => { results: { nameWithOwner: string; description: string; stars: number; language: string; readmePreview: string }[]; total: number };
  search: (input: { query: string; limit?: number; fullMode?: boolean }) => { total: number; incomplete: boolean; matches: { repo: string; path: string; fragment: string }[] };
  tree: (input: { repo: string; path?: string; depth?: number }) => string[];
}
```

### Codemode Rules

- Write plain JavaScript, not TypeScript (no type annotations)
- `codemode.*` calls are synchronous — no `await` needed
- Must `return` a value — bare expressions don't auto-return (except simple identifiers)
- Must `return` a value — bare expressions don't auto-return (except simple identifiers)
- Console output goes to stderr, return value goes to stdout
- Max 20 tool calls per execution, 64KB code size limit

## Search Query Syntax

`ghx search` uses GitHub REST code search API. Every word is AND'd — a file must contain ALL words to match. More words = fewer results, not better results. Search 1-2 terms, not 5.

```bash
ghx search "addClass repo:jquery/jquery"                  # Scoped to repo
ghx search "useState language:typescript"                 # Language filter
ghx search "filename:package.json repo:owner/repo"        # Find specific filename
ghx search '"exact phrase" repo:plausible/analytics'      # Exact phrase (shell quotes)
ghx search "path:llms.txt"                                # Find files by name
```

**Valid qualifiers:** `repo:`, `org:`, `path:`, `filename:`, `extension:`, `language:`, `in:file`, `in:path`, `size:`, `fork:true`

**DO NOT USE (web-only, silently wrong):** `OR`, `NOT`, `symbol:`, `content:`, `is:`, regex

**Rate limit:** 9 req/min for code search. Refine queries, don't paginate.

## Gotchas

1. **Web-only qualifiers silently degrade.** `symbol:`, `OR`, `NOT` are treated as literal text. ghx warns on stderr.
2. **`gh search code` wraps in quotes.** `gh search code "foo bar"` = exact phrase. `ghx search "foo bar"` = AND. Use ghx.
3. **Flag ordering in `read`.** `ghx read owner/repo file --map` works. `ghx read --map owner/repo file` does NOT.
4. **Not all repos use `main`.** ghx handles this automatically.
5. **Unknown flags are rejected.** Exit 2 with clear error. Intentional — prevents silent query corruption.
6. **`--grep` uses ERE regex.** Use `|` for alternation, not `\|`. Example: `--grep "ref|defs|definition"`.

## Anti-Patterns

- ❌ `web_fetch` on github.com — HTML noise, zero useful info
- ❌ Reading entire large files when you need 10 lines — use `--grep` or `--lines`
- ❌ Multiple `gh api` calls for explore — use `ghx explore` (1 call)
- ❌ Using web-only qualifiers in search — silently wrong results
- ❌ Paginating broad searches — refine with `repo:`, `language:`, `path:` instead
- ❌ `gh search code` for multi-word queries — silently wraps in quotes, returns nothing

## Best Practices

- **Batch reads.** `ghx read repo f1 f2 f3` = 1 API call. Three separate reads = 3 calls.
- **Map before reading.** `--map` first, then `--grep` or `--lines` for specifics.
- **Refine search, don't paginate.** Add qualifiers instead of fetching page 2.
- **Use `ghx code` for multi-step workflows.** One round-trip beats three sequential commands.
- **Check exit codes.** 0 = results, 1 = no results (broaden query), 2 = usage error (fix command).
