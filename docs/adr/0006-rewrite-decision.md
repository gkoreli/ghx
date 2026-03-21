# ADR-0006: Go vs TypeScript vs Bash — Rewrite Decision

**Date**: 2026-03-21
**Status**: Superseded by ADR-0007
**Parent**: TASK-0508

## Context

ghx v0.2.1 is a ~350 line bash script. As an experiment, we spawned 10 MiniMax M2.5 agents (0.25x credits each) to rewrite it in Go and TypeScript in parallel. Both implementations landed in `/Users/gkoreli/Documents/goga/ghx/v2/`.

This ADR evaluates the evidence from that swarm experiment to decide whether to:
- Stay with bash (current)
- Rewrite in Go
- Rewrite in TypeScript

## Swarm Experiment Evidence

| Metric | Go | TypeScript | Bash (baseline) |
|--------|-----|------------|-----------------|
| Agents spawned | 5 | 5 | — |
| Functional commands | ~40% | ~75% | 100% |
| Binary size | 4.5MB | 4.6MB (dist/) | 350 lines |
| Dependencies | cobra | commander + 8 packages | none |
| Cross-platform | compiled | requires build | works via Git Bash |

### What worked

**Go:**
- `explore`, `tree`, `version` commands work
- Clean cobra CLI structure
- Compiles to single binary

**TypeScript:**
- `repos`, `search`, `read --grep` work
- Uses commander.js (solid library)
- Compiles to JS with types

### What broke (both)

- **HTML stripping regressed** — bash has sophisticated jq-based HTML stripping in `repos` command. Both rewrites stripped less aggressively, producing worse output.
- **skill command broken** — neither implementation correctly reads SKILL.md relative to the executable.
- **Go-specific**: Flags completely broken due to cobra architecture mistake (subcommands not wiring flags correctly).
- **TypeScript-specific**: `--map` regex broken (pattern mismatch), exit codes missing (no `process.exit(1)` on errors).

### Agent behavior observation

The swarm revealed a consistent pattern: **agents get structure right, details wrong.**

- Both implementations have correct file structure (cmd/, src/commands/, package.json, go.mod)
- Both use appropriate libraries (cobra, commander)
- Both implement the same 6 commands with similar GraphQL queries
- But: edge cases, error handling, flag edge cases, HTML stripping, exit codes — all broken

This is the fundamental limitation of current LLM agents: they can scaffold but not polish.

## Evaluation Criteria

### 1. Zero-install story

| Language | Distribution | Install complexity |
|----------|-------------|-------------------|
| Bash | curl, npx, brew, gh ext | Zero — just the script |
| Go | goreleaser to npm | Build step required |
| TypeScript | npm publish | npm install or build |

**Winner: Bash** — the #1 value prop per ADR-0002.

### 2. Flag parsing / CLI safety

This is the bug class that triggered ADR-0005. The question: which language prevents it?

| Language | How flags work | Unknown flag behavior |
|----------|---------------|----------------------|
| Bash | case statement, explicit `--*) catch-all | Errors loudly (ADR-0005 fix) |
| Go | cobra, declarative | Silent failure when miswired |
| TypeScript | commander, declarative | Errors at runtime |

**Winner: Bash** — explicit is safer than declarative when agents generate code.

### 3. Code complexity

| Language | Lines (functional) | Maintainability |
|----------|-------------------|-----------------|
| Bash | ~350 | Single file, readable |
| Go | ~500 (cmd/ghx.go) | Split across files |
| TypeScript | ~400 (src/) | Split across files |

**Winner: Bash** — less code = fewer bugs, easier to audit.

### 4. Swarm-friendliness

The experiment showed: agents produce worse code in Go/TS than bash. This is a real cost.

| Language | Agent output quality | Fixes needed |
|----------|---------------------|--------------|
| Bash | baseline | — |
| Go | 40% functional | 5 more agents spawned |
| TypeScript | 75% functional | 5 more agents spawned |

**Winner: Bash** — simpler languages are more agent-reproducible.

### 5. Dependency cost

| Language | Dependencies | Supply chain risk |
|----------|-------------|-------------------|
| Bash | none | none |
| Go | cobra, go-gh | 2 packages |
| TypeScript | commander, 7 others | 8 packages |

**Winner: Bash** — zero dependencies.

### 6. v0.3 readiness

ADR-0004 lists 5 features for v0.3:
- Agent instruction file detection (3 GraphQL aliases)
- Tree filtering (grep -v)
- Token estimation (wc -c / 4)
- Hints (echo to stderr)
- --minify (sed pipeline)

All are straightforward in bash. Go/TS add build complexity.

**Winner: Bash** — features fit naturally in bash.

### 7. Testing story

| Language | Testing | Complexity |
|----------|---------|-----------|
| Bash | shunit2, bats | moderate |
| Go | go test | standard |
| TypeScript | jest, vitest | standard |

**Winner: Tie** — all have testing options.

## Decision

**Selected: Stay with Bash**

### Rationale

1. **Zero-install is non-negotiable.** This is ghx's #1 value prop. Go/TS require build steps that break the promise.

2. **Agents can't write Go/TS correctly.** The swarm evidence is unambiguous: 10 agents produced worse code in compiled languages than the original bash. This isn't a bash vs Go/TS issue — it's a complexity issue. Simpler languages are more agent-reproducible.

3. **Flag safety is proven.** ADR-0005's flag firewall works in bash. The same pattern in cobra (Go) broke completely.

4. **v0.3 fits in bash.** At ~395 lines post-v0.3, we're still well under the 500-line Go rewrite trigger from ADR-0002.

5. **The "regressions" in agent output are telling.** Both Go and TS implementations lost the sophisticated HTML stripping that bash has. This is exactly the kind of detail that agents miss — and exactly the kind of detail that makes ghx better than raw `gh`.

### What the swarm proved

The swarm wasn't a failure — it was the most important research ghx has done. It proved:

- **Bash is the right language for agent-generated CLI tools** — agents can produce correct bash more reliably than Go/TS
- **The 350-line bash script is a local maximum** — rewriting in a "real" language made things worse, not better
- **The bottleneck is agent capability, not tool capability** — when agents can write Go/TS as well as bash, we can revisit

### Trade-offs accepted

- **No Windows native support** — works via Git Bash/WSL only (documented limitation)
- **Performance ceiling** — subprocess spawning has overhead (acceptable for CLI tool)
- **No Tree-sitter** — can't do multi-line signature extraction (noted in ADR-0004)

## Consequences

- Continue developing in bash
- Close the v2/ directory (Go/TS experiments)
- Document the swarm findings in SKILL.md as a case study
- Revisit Go/TS if: (a) script exceeds 500 lines, (b) Windows native support becomes critical, (c) agent capabilities improve significantly

## Implementation Notes

The v2/ directory contains valuable reference code:
- Go: `cmd/ghx.go` has clean GraphQL query patterns to port back
- TypeScript: `src/commands/` has good commander.js structure

Extract useful patterns, discard the rest.