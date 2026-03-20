# ADR-0005: Flag Firewall and Agentic-First Defaults

**Date**: 2026-03-20
**Status**: Accepted

## Context

An agent session using ghx to explore Playwright/Cypress MCP alternatives produced 5 consecutive failures, all from the same root cause. The agent passed `--limit 20` — a flag every CLI tool supports — and ghx silently absorbed it into the query string, corrupting every request.

### The Failure Cascade (verbatim from session)

```
ghx search "playwright mcp server" --limit 20  → 422 ERROR_TYPE_QUERY_PARSING_FATAL
ghx search "cypress mcp server" --limit 20     → 422 ERROR_TYPE_QUERY_PARSING_FATAL
ghx repos "playwright mcp" --limit 10          → 0 repos found
ghx repos "cypress mcp" --limit 10             → 0 repos found
ghx repos "browser mcp playwright" --limit 10  → 0 repos found
→ Agent abandoned ghx, fell back to raw gh CLI
gh search repos "playwright mcp" ...           → 15 results (worked immediately)
```

Without `--limit`, the same queries succeed: `ghx repos "playwright mcp"` → 2,093 repos, `ghx search "playwright mcp server"` → 42,304 results.

### Root Cause

Both `search` and `repos` commands lack flag parsing for `--limit`. Unrecognized flags get concatenated into the query string:

- `search`: `args` array collects everything except `--full`, then `query="${args[*]}"` joins them. Query becomes `"playwright mcp server --limit 20"` → GitHub API can't parse `--limit` as a qualifier → 422.
- `repos`: `query="$*"` captures all arguments verbatim. Query becomes `"playwright mcp --limit 10"` → GraphQL searches for that literal string → 0 results.

### Why This Is Agent-Specific

Humans don't speculatively try `--limit` on unfamiliar tools. Agents do — they generalize from every CLI in their training data. `--limit` appears in thousands of CLI tools. Every agent will try it. This is a behavior pattern unique to agents, and the silent corruption it causes is the exact failure mode ghx was built to prevent (ADR-0002: "Fail loudly").

### The Real Damage: Tool Abandonment

The catastrophic outcome wasn't 5 wasted API calls. It was **permanent loss of trust**. The agent abandoned ghx for the rest of the session. Every subsequent GitHub operation used raw `gh` instead of ghx's richer output (README previews, star counts, batched reads). The agent downgraded itself.

## Vulnerability Scope

| Command | Flag parsing | Unknown flag behavior | Severity |
|---------|-------------|----------------------|----------|
| `search` | `--full` only | Silently joins into query → API 422 or wrong results | Critical |
| `repos` | None | Silently joins into query → 0 results | Critical |
| `read` | `--grep`, `--lines`, `--map` | Becomes a "file path" → not found | Medium |
| `explore` | Positional only | N/A | Safe |
| `tree` | Positional only | N/A | Safe |

## Decision

Three changes that complete the promise from ADR-0002 ("the right behavior is the default behavior"):

### 1. Flag Firewall — unknown `--*` flags always error

Every command that parses flags gets a catch-all:
```bash
--*) echo "ghx $cmd: unknown flag '$1'" >&2; exit 2 ;;
```

This makes silent corruption impossible. An agent seeing exit 2 + clear error knows immediately: "my command was wrong."

### 2. `--limit N` — universal flag for list commands

| Command | Maps to | Default | Max |
|---------|---------|---------|-----|
| `search` | REST `per_page` param | 30 | 100 |
| `repos` | GraphQL `first:` param | 10 (up from 5) | 20 |

`repos` default increases from 5 → 10. Rationale: agents doing discovery need enough options to make a decision. 10 repos × ~300 chars README preview = ~750 tokens. Cheap enough to be a smart default.

### 3. Exit code contract

| Exit | Meaning | Agent action |
|------|---------|-------------|
| 0 | Success with results | Process output |
| 1 | No results / not found (query was valid) | Broaden query or try different approach |
| 2 | Usage error (bad flags, bad syntax) | Fix command |

Currently both "no results" and "error" can exit 0 or 1 inconsistently. This makes the three states unambiguous.

## Consequences

- Agents that try `--limit` (all of them) get correct behavior instead of silent corruption
- Agents can distinguish "nothing exists" from "I used the tool wrong" via exit codes
- `repos` returns 10 results by default — better for agent discovery workflows
- SKILL.md and help text must be updated to document `--limit` and exit codes
- Breaking change: scripts relying on unknown flags being silently absorbed will now error (this is correct behavior)

## Implementation Notes

- Flag firewall is ~4 lines per command (the `--*` catch-all in each case block)
- `--limit` for search: add `-f per_page="$limit"` to the `gh api` call
- `--limit` for repos: interpolate into GraphQL `first:` (currently hardcoded to 5)
- Exit codes: `exit 1` after "0 repos found" / "0 results", `exit 2` for usage errors
- `read` command: add `--*` catch-all to reject unknown flags (they currently become bad file paths)
