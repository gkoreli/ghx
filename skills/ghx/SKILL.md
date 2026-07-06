---
name: ghx
description: "Use when exploring GitHub repositories from a shell with the ghx CLI: read files, search code, map symbols, inspect trees, discover repos, compose multi-step reconnaissance with codemode, or delegate whole repo questions to the ghx sidecar."
version: 1.3.0
author: ghx contributors
license: MIT
metadata:
  hermes:
    tags: [github, code-exploration, cli, codemode, repository-recon]
    related_skills: []
---

# ghx — GitHub Code Exploration for AI Agents

> **This is the power-user/tool layer** — for driving exploration yourself.
> If you just want answers about a repo, delegate the whole question to the
> recon service instead: `ghx serve --recon` exposes a single `recon` MCP
> tool, and `ghx skill --recon` prints its concise skill. You then need zero
> knowledge of the commands below.

Use `ghx` via `execute_bash` for anything on GitHub — repos, files, code search, codemode. Authenticated via `gh` CLI, structured output, zero context overhead.

## Commands

```bash
ghx explore <owner/repo>                    # Branch + tree + README in 1 API call (budgeted orientation)
ghx explore <owner/repo> <path>             # Subdirectory listing
ghx explore <owner/repo> --full             # Complete explore output, no budget compaction
ghx read <owner/repo> <f1> [f2] [f3]       # Read 1-10 files in 1 API call (GraphQL batching)
ghx read <owner/repo> <dir>                 # Directory path → returns file listing (not "not found")
ghx read <owner/repo> "src/**/*.ts" --map   # Glob patterns: auto-expands via tree (2 API calls)
ghx read <owner/repo> --map <f1> [f2]       # Parser-backed structural map: signatures, imports, types (~92% token reduction on typical source files)
ghx read <owner/repo> --map --kind func <f> # Map only function/method signatures
ghx read <owner/repo> --map --kind type <f> # Map only types/structs/interfaces/classes
ghx read <owner/repo> --map --level minimal <f> # Symbol names only — methods show as UserService.GetUser
ghx read <owner/repo> --grep "pat" <f>      # Read file, show only matching lines (2 lines context, regex)
ghx read <owner/repo> --lines 42-80 <f>     # Read specific line range
ghx read <owner/repo> <f> --full            # Force complete contents (skip the budget fallback)
ghx grep <owner/repo> "<pattern>"           # Repo code search, grep-style flags: --path, --glob, --limit
ghx repos "<query>"                         # Search repos with README preview in 1 GraphQL call
ghx search <owner/repo> "<query>"           # Repo-first code search (--lang, --glob filters)
ghx search "<query>"                        # Advanced form: raw GitHub query with qualifiers
ghx search --full "<query>"                 # Code search without line truncation
ghx tree <owner/repo> [path]                # Full recursive tree (default: all files, no depth limit)
ghx tree <owner/repo> [path] --depth N      # Tree limited to N levels (includes dirs with /)
ghx code "<js>"                             # Execute JS with access to all ghx tools
ghx code -                                  # Read code from stdin
ghx code --list                             # List available tools with type stubs
ghx --version                               # Version (also: ghx version)
```

**Exit codes:** 0 = success, 1 = no results, 2 = usage error, 3 = upstream/API failure.

**Budgeted output.** `explore`, `read`, and `search` share a `--budget CHARS` dial (default 12000). A full-file `read` over budget (without `--map`/`--lines`/`--grep`) returns the structural map plus a hint naming the narrowing flags instead of flooding output; every truncation names the flag that lifts it (`--full`).

## The sidecar: delegate the whole question

The commands above are the *tool layer* — you drive exploration yourself. The **sidecar** is the other half of ghx: a specialist agent you hand a plain-English repo question to, which explores GitHub itself and returns a compact, auditable evidence report. When you want an *answer* about a repo rather than to run the search-and-read loop by hand, ask the sidecar.

### One-time setup

```bash
ghx sidecar config init --claude-acp   # write ~/.ghx/config.json (pinned Claude ACP adapter; needs Node/npx + a Claude login)
ghx sidecar doctor                     # verify token, network, ghx binary, ACP handshake, report sink (exit 3 if any check fails)
ghx sidecar doctor --live              # also run a real one-prompt turn; on failure prints the failing stage + agent stderr
ghx sidecar ask --repo hono/hono "How is middleware chained, and which files define it?"
```

`config init` without `--claude-acp` auto-detects any ACP-capable agent already on PATH.

The sidecar runs the agent in a neutral, ghx-owned session directory (not your repo), so turns are deterministic regardless of where you invoke `ghx`. If an `ask` returns nothing, the agent's stderr is captured at `~/.ghx/sessions/<name>/agent-stderr.log`, a failed turn splices that tail into the CLI error, and `doctor --live` reproduces the failure with its stage. `sessions show <name>` records the agent command, spawn cwd, and agent-relevant env-var names (never values) so an environment-specific failure is diagnosable.

### Ask

```bash
ghx sidecar ask --repo <owner/repo> "<question>"   # repo-scoped reconnaissance
ghx sidecar ask "<question>"                        # no --repo → discovery: which repos/libraries do X
ghx sidecar ask --depth deep "<question>"           # command budget: cheap|normal|deep (default normal)
ghx sidecar ask --json "<question>"                 # {report, artifacts} envelope instead of the human summary
ghx sidecar ask --session <name> "<question>"       # keep parallel investigation threads apart
```

State the goal, not the steps: *"How does hono implement middleware chaining, and which files define it?"* beats *"grep for middleware"*. Normally omit `--session`: the daemon routes each ask. Explicit `--session` wins; `--repo` or an `owner/repo` mention routes to the repo-slug session; otherwise ghx checks for a warm continuation, then ledger overlap, then creates a new question-derived discovery session. Routed asks print `session: <name> (routed: <rule>)` to stderr, so follow-ups can stay context-aware without you naming sessions. Every answer ends with an artifacts footer — `artifacts: <session dir> (trace <id>)` — pointing at the on-disk trail.

### Where artifacts land, and how to inspect

Everything the sidecar does is durable under `~/.ghx/` (or `$GHX_HOME`): `sessions/<name>/` holds each turn's report JSON, the evidence ledger, and OTel `traces.jsonl`.

```bash
ghx sidecar sessions list              # all sessions with repo, turn count, last-updated
ghx sidecar sessions show <session>    # metadata + saved report history
ghx sidecar sessions ledger <session>  # accumulated evidence ledger (JSON) — the commands run and sources read
ghx sidecar sessions reroute <s> <turn> <dest>  # move a mis-routed turn; both ledgers rebuilt by replay
ghx sidecar view [session]             # spawn a local OTel viewer over the session's traces (needs otel-desktop-viewer on PATH)
ghx sidecar view --list                # sessions with turn/report counts
```

### For a main agent: the recon service

If *you* are the main agent and want zero knowledge of the CLI grammar above, expose the sidecar as one MCP tool and delegate whole questions to it:

```bash
ghx serve --recon     # serves exactly one tool: recon(question, repo?, session?) → evidence report
ghx skill --recon     # prints the concise recon skill (what it is, how to phrase questions, report fields)
```

## Chain of Thought: Pick Your Entry Point

**Don't follow a sequence. Pick the right starting point based on what you already know.**

**Know what you're looking for?** Start with `grep`/`search` or direct `read`:
```
ghx grep owner/repo "pattern" --path src     → Find files by content (grep-style)
ghx read owner/repo path/to/file --map       → Read it (works for files AND directories)
ghx read owner/repo "src/**/*.ts" --map      → Glob to scan many files at once
```

**Don't know the repo at all?** Start with `explore`:
```
ghx explore owner/repo                       → Structure + README (orientation)
```

**Then drill in — map before reading, grep before full read:**
```
ghx read owner/repo "src/**/*.ts" --map      → Signatures of many files (~92% fewer tokens; measured: a 15.3 KB Go file maps to 0.9 KB)
ghx read owner/repo --grep "X" f             → Just the matching lines
ghx read owner/repo f                        → Full file (only when needed)
```

`--map` doesn't just save tokens — it lets you see 10 files for the cost of reading 1. Engine selection is automatic: **Go** uses `go/ast` (full multi-line signatures, no local variable noise), **TS/JS/Python/Rust** use Tree-sitter (captures class and impl methods that regex cannot reach), everything else falls back to regex. Methods carry parent context — `--level minimal` renders `UserService.GetUser` instead of just `GetUser`.

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
  read: (input: { repo: string; files: string[]; grep?: string; lines?: string; map?: boolean; level?: "outline" | "minimal" | "compact" | "standard"; kind?: "func" | "type" | "import" | "const" | "var" | "package"; mapEngine?: "auto" | "regex" | "tree-sitter" }) => { path: string; content: string; byteSize: number; notFound: boolean; dirEntries?: { name: string; type: string }[]; globPattern?: string; grepHits?: { lineNum: number; line: string; isMatch: boolean }[]; mapLines?: string[]; mapChars?: number; mapEngine?: string; mapWarnings?: string[] }[];
  repos: (input: { query: string; limit?: number }) => { results: { nameWithOwner: string; description: string; stars: number; language: string; readmePreview: string }[]; total: number };
  search: (input: { query: string; limit?: number; fullMode?: boolean }) => { total: number; incomplete: boolean; matches: { repo: string; path: string; fragment: string }[] };
  tree: (input: { repo: string; path?: string; depth?: number }) => string[];
}
```

### Codemode Rules

- Write plain JavaScript, not TypeScript (no type annotations)
- `codemode.*` calls are synchronous — no `await` needed
- Must `return` a value — bare expressions don't auto-return (except simple identifiers)
- Console output goes to stderr, return value goes to stdout
- Max 20 tool calls per execution, 64KB code size limit

## Search Query Syntax

`ghx search` and `ghx grep` search **inside file contents** — use them to find code patterns, not to discover repos. To discover repos by topic, use `ghx repos`. When you know the repo, prefer the simple forms: `ghx grep owner/repo "pattern"` or `ghx search owner/repo "query"` — the raw-query form below is the advanced mode.

| Want to find... | Use |
|---|---|
| Repos about "slack bot" | `ghx repos "slack bot go"` |
| Code containing "useState" | `ghx search vercel/next.js useState` |
| Same, with grep muscle memory | `ghx grep vercel/next.js useState --glob "src/**/*.ts"` |
| Files named "go.mod" | `ghx search "filename:go.mod repo:owner/repo"` |

Every word is AND'd — a file must contain ALL words to match. More words = fewer results, not better results. Search 1-2 terms, not 5.

```bash
ghx search "addClass repo:jquery/jquery"                  # Scoped to repo
ghx search "useState language:typescript"                 # Language filter
ghx search "filename:package.json repo:owner/repo"        # Find specific filename
ghx search '"exact phrase" repo:plausible/analytics'      # Exact phrase (shell quotes)
```

**Valid qualifiers:** `repo:`, `org:`, `path:`, `filename:`, `extension:`, `language:`, `in:file`, `in:path`, `size:`, `fork:true`

`path:` filters by file path, not by "repos containing this file." `path:src` = only search files under `src/`. `path:go.mod` = only search inside files named `go.mod` (probably not what you want — use `filename:` instead).

**DO NOT USE (web-only, silently wrong):** `OR`, `NOT`, `symbol:`, `content:`, `is:`, regex

**Rate limit:** 9 req/min for code search. Refine queries, don't paginate.

## Gotchas

1. **Web-only qualifiers silently degrade.** `symbol:`, `OR`, `NOT` are treated as literal text. ghx warns on stderr.
2. **`gh search code` wraps in quotes.** `gh search code "foo bar"` = exact phrase. `ghx search "foo bar"` = AND. Use ghx.
3. **Not all repos use `main`.** ghx handles this automatically.
4. **Wrong flags fail with a teaching error.** Exit 2, naming the nearest real flag plus one corrected example (e.g. `--start` → `use --lines START-END, e.g. ghx read owner/repo main.go --lines 40-80`). Read the error; it hands you the fix.
5. **Big full reads fall back to a map.** Over `--budget` (default 12000 chars) without `--map`/`--lines`/`--grep`, `read` returns the structural map + a narrowing hint. Add `--full` if you truly need everything.
6. **`--grep` uses ERE regex.** Use `|` for alternation, not `\|`. Example: `--grep "ref|defs|definition"`.
7. **Glob + grep skips non-matching files.** Like `grep -r --include`, only files with hits are shown.
8. **`ghx grep --glob` maps to GitHub `path:`.** It is backed by GitHub code search (indexing lag applies), not a local ripgrep over the tree.

## Anti-Patterns

- ❌ `web_fetch` on github.com — HTML noise, zero useful info
- ❌ Reading entire large files when you need 10 lines — use `--grep` or `--lines`
- ❌ Multiple `gh api` calls for explore — use `ghx explore` (1 call)
- ❌ Using web-only qualifiers in search — silently wrong results
- ❌ Paginating broad searches — refine with `repo:`, `language:`, `path:` instead
- ❌ Broad globs like `**/*.ts` on large repos — matches thousands, reads only 10. Narrow the pattern first

## Best Practices

- **Directories just work.** `ghx read repo src/utils --map` returns a file listing if `src/utils` is a directory, with a hint to glob. No "not found" surprise — pass any path and get useful output.
- **Batch reads.** `ghx read repo f1 f2 f3` = 1 API call. Three separate reads = 3 calls.
- **Glob to scan.** `ghx read repo "src/**/*.ts" --map` = tree + read in 2 API calls. Max 10 files; shows all matched paths if 11-50, hint to narrow if more.
- **Map before reading.** `--map` first, then `--grep` or `--lines` for specifics.
- **Refine search, don't paginate.** Add qualifiers instead of fetching page 2.
- **Use `ghx code` for multi-step workflows.** One round-trip beats three sequential commands.
- **Check exit codes.** 0 = results, 1 = no results (broaden query), 2 = usage error (fix command — the error text shows how), 3 = upstream/API failure (retry or check `gh auth status`).
