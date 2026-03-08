# ADR-0004: v0.3 — First Contact Intelligence

**Date**: 2026-03-08
**Status**: Proposed

## The Vision

**v0.3 theme: an agent goes from "never seen this repo" to "productive" in 2 commands.**

Today, `ghx explore` gives you tree + README. That's enough to know WHAT files exist. But it's not enough to know HOW to work in the repo — what conventions to follow, which files are noise, how big the codebase is, or what to do next. The agent still needs 2-3 more calls to become productive.

v0.3 closes this gap. After `ghx explore`, the agent knows:
- The repo structure (tree) — already done
- The repo purpose (README) — already done
- **The repo conventions** (AGENTS.md / CLAUDE.md) — v0.3
- **Which files matter** (filtered tree, no noise) — v0.3
- **How big the output will be** (token estimation) — v0.3
- **What to do next** (hints) — v0.3

The principle: **MCP-level intelligence, zero-overhead CLI.** Octocode has hints, token warnings, file filtering, and content minification — but costs ~10K tokens of MCP schemas. ghx delivers the same intelligence through smart defaults in a bash script.

## The Problem: First Contact Is Expensive

When an agent encounters a new repo, it faces a cold-start problem. It knows nothing about the codebase — not the conventions, not the structure, not the testing strategy, not which directories are noise. Every decision is a guess.

**What agents actually need at first contact (in order):**

1. What is this repo? → README (have it)
2. What's the file structure? → tree (have it)
3. How should I work in this repo? → **AGENTS.md** (don't surface it)
4. Which files should I look at? → **filtered tree** (show everything including noise)
5. How much context will this cost me? → **token estimation** (no idea)
6. What should I do next? → **hints** (no guidance)

Steps 1-2 are solved. Steps 3-6 are the v0.3 scope.

**Evidence that this matters:** Next.js ships a 20KB AGENTS.md with monorepo structure, key entry points, testing commands, and coding conventions. An agent that reads this BEFORE touching code makes fundamentally better decisions. An agent that doesn't will guess wrong about the build system, test runner, and directory layout — then waste 5-10 calls recovering.

## Evidence: Agent Instruction Files Are Everywhere

Empirical survey of major open-source repos (2026-03-08, verified via GraphQL):

| Repository | AGENTS.md | CLAUDE.md | copilot-instructions | Size |
|---|---|---|---|---|
| vercel/next.js | ✅ 20KB | symlink → AGENTS.md | — | Monorepo structure, entry points, testing |
| langchain-ai/langchain | ✅ 10KB | ✅ 10KB (same content) | — | Architecture, contribution guide |
| biomejs/biome | ✅ 12KB | symlink → AGENTS.md | — | Rust crate structure, testing |
| denoland/deno | — | ✅ 11KB | ✅ 7KB | Build system, testing, V8 bindings |
| microsoft/vscode | ✅ 271B | — | ✅ 9.5KB | Extension API, testing |
| astral-sh/ruff | ✅ 4KB | symlink → AGENTS.md | — | Rust project structure |
| anthropics/anthropic-cookbook | — | ✅ 3.4KB | — | Notebook conventions |
| facebook/react | — | ✅ 208B | — | Monorepo overview |
| yamadashy/repomix | ✅ 21B | ✅ 21B | ✅ 24B | Pointers to .agents/rules/ |

**Pattern:** AGENTS.md is the emerging canonical name. CLAUDE.md often symlinks to it. `.github/copilot-instructions.md` is the GitHub Copilot variant. 8 of 9 major repos checked have at least one agent instruction file.

**What's in them:** Not just "use TypeScript." Next.js AGENTS.md includes:
- Complete monorepo directory map (`packages/`, `turbopack/`, `crates/`, `test/`)
- Key entry points (`src/cli/next-dev.ts → src/server/dev/next-dev-server.ts`)
- Testing commands and conventions
- Build system details (pnpm, turbopack)

This is exactly the context an agent needs BEFORE reading code. And `ghx explore` already makes a GraphQL call that could fetch it — for zero extra API cost.

## Inspirations (distilled, with references)

### 1. Octocode's Hints System
**Source:** `bgauryy/octocode-mcp` — `packages/octocode-mcp/src/utils/response/hints.ts`

Every tool response includes contextual hints in three categories:
- `hasResults`: suggest the next tool ("💡 TIP: Use matchString for targeted extraction")
- `empty`: suggest broadening ("💡 TIP: Try removing path filter")
- `error`: suggest recovery ("💡 TIP: Check repository name spelling")

**What we take:** The concept of guiding the agent's next action. But Octocode embeds hints in JSON responses inside an MCP. ghx prints them to stderr — zero context overhead, same guidance.

### 2. gh-aw's Schema-First Pattern
**Source:** `github/gh-aw` — `skills/github-pr-query/query-prs.sh`

When called without `--jq`, returns metadata instead of data:
```json
{
  "item_count": 30,
  "data_size_bytes": 45000,
  "schema": { "item_fields": { "number": "integer", "title": "string", ... } },
  "suggested_queries": [
    {"description": "Get PR numbers and titles", "query": ".[] | {number, title}"}
  ]
}
```

**What we take:** The principle of "tell the agent what's available before dumping data." For ghx, this maps to token estimation on stderr: "This output is ~45K tokens. Consider --grep or --map to narrow." The agent decides whether to proceed, not whether to recover.

### 3. Octocode's File Filtering
**Source:** `bgauryy/octocode-mcp` — `packages/octocode-mcp/src/utils/core/filter.ts`

85 folder exclusions (node_modules, dist, .git, __pycache__, vendor, coverage, .next, .nuxt, .cache, bower_components, ...), 40+ file exclusions (package-lock.json, yarn.lock, secrets.json, id_rsa, ...), 30+ extension exclusions (.lock, .log, .min.js, .map, .woff, .ttf, .ico, .png, .jpg, ...).

**What we take:** A curated exclusion list for `ghx tree`. Not 85 folders — a focused list of the highest-noise patterns. Default on, `--all` to disable. Follows principle 3: smart defaults, explicit opt-out.

### 4. Octocode's Token Warnings
**Source:** `bgauryy/octocode-mcp` — `packages/octocode-mcp/src/utils/response/tokenWarning.ts`

Three tiers: >50K tokens = "🚨 CRITICAL: Response TOO LARGE", >30K = "⚠️ WARNING", <30K = no warning. Uses chars/4 estimation (same as Aider's `repomap.py` and Codemap's `sourceMap.ts`).

**What we take:** Token estimation on stderr after every output-producing command. The chars/4 formula is industry standard. But we add a twist: suggest the specific flag that would help. ">30K tokens. Consider --map (92% reduction) or --grep to narrow."

### 5. Codemap's Progressive Detail Reduction
**Source:** `kcosr/codemap` — `src/sourceMap.ts` function `fitToBudget()`

Five detail levels: full → standard → compact → minimal → outline. Algorithm: start all files at full, while total > budget, reduce the largest file by one level. Repeat.

**What we take (as future direction):** The concept that degradation should be graceful, not abrupt. ghx's `--map` is codemap's "compact" level. A future `--map=names` could be "minimal" (names only, no signatures). But for v0.3, the simpler version: token estimation + hints that suggest --map when output is large.

### 6. Repomix's Git Change Frequency
**Source:** `yamadashy/repomix` — `src/core/git/gitCommand.ts`

Files sorted by git commit frequency — most-changed files go to the bottom of output (where LLMs pay most attention due to recency bias). Command: `git log --pretty=format: --name-only -n 100`, count occurrences, sort ascending.

**What we take (as agent practice, not tool feature):** Document this in SKILL.md as a research strategy. After `ghx explore`, if the agent clones for deep analysis, sort by change frequency.

## v0.3 Scope: 5 Features

### Feature 1: Agent Instruction File Detection in `explore`

**What:** Add AGENTS.md and CLAUDE.md to the `ghx explore` GraphQL query. Surface them alongside README.md.

**Why:** 8/9 major repos have agent instruction files. Next.js's is 20KB of structured guidance. An agent that reads this first makes fundamentally better decisions about the codebase.

**How:** Two additional GraphQL aliases in the existing query — zero extra API calls:
```graphql
agents: object(expression: "$branch:AGENTS.md") { ... on Blob { text } }
claude: object(expression: "$branch:CLAUDE.md") { ... on Blob { text } }
```
If both exist and are identical (common — CLAUDE.md symlinks to AGENTS.md), show only once. If neither exists, show nothing (no noise).

**Effort:** 2 GraphQL aliases + ~5 jq lines for dedup.
**Impact:** Agent gets repo conventions in the same call that gets tree + README. Zero extra cost.

### Feature 2: Tree Filtering (Smart Defaults)

**What:** `ghx tree` and `ghx explore` filter out known noise by default. `--all` to disable.

**Why:** A large repo tree can have 10,000+ entries. node_modules alone can be thousands. The agent wastes context processing paths it will never read. Octocode filters 85 folders — evidence that this matters at scale.

**How:** Focused exclusion list (not Octocode's 85 — the highest-noise patterns only):
```
# Directories
node_modules/ dist/ build/ .git/ __pycache__/ .next/ .nuxt/ coverage/ vendor/ .cache/

# Files
*.lock *.min.js *.min.css *.map *.woff *.woff2 *.ttf *.eot *.ico *.png *.jpg *.jpeg *.gif *.svg *.mp4 *.webm
```

Pipe tree output through `grep -vE` with the exclusion pattern. `--all` flag bypasses filtering.

**Effort:** ~15 lines (exclusion list + grep -v + --all flag parsing).
**Impact:** Clean tree = better decisions. Agent sees signal, not noise.

### Feature 3: Token Estimation Warnings

**What:** After every output-producing command, estimate tokens (`output_bytes / 4`) and warn on stderr if large.

**Why:** Context overflow is the #1 cause of agent degradation. Octocode, Codemap, and Aider all use chars/4 estimation. gh-aw's schema-first pattern exists specifically to prevent data dumps.

**How:**
```bash
output_size=$(echo "$output" | wc -c)
tokens=$((output_size / 4))
if [[ $tokens -gt 30000 ]]; then
  echo "⚠ Large output (~${tokens} tokens). Consider --map or --grep to narrow." >&2
elif [[ $tokens -gt 50000 ]]; then
  echo "🚨 Very large output (~${tokens} tokens). Risk of context overflow." >&2
fi
```

The suggestion is command-specific: after `read`, suggest `--map` or `--grep`. After `tree`, suggest filtering. After `search`, suggest adding qualifiers.

**Effort:** ~10 lines (shared function + per-command suggestions).
**Impact:** Prevents context overflow. Informs strategy before it's too late.

### Feature 4: Hints (Guide Next Action)

**What:** After each command, print a contextual hint to stderr suggesting the logical next step.

**Why:** Octocode's hints system prevents agents from using the wrong tool. gh-aw's suggested_queries guide agents to the right jq filter. The pattern: don't just give data, guide the agent's next action.

**How:** One echo per command, context-dependent:
```
# After explore (no AGENTS.md found)
Hint: ghx read --map to understand code structure

# After explore (AGENTS.md found)
Hint: AGENTS.md detected — read it for repo conventions

# After search (results found)
Hint: ghx read owner/repo path to view a file

# After search (0 results)
Hint: broaden query — remove qualifiers or try synonyms

# After read (large output)
Hint: use --grep "pattern" or --lines N-M to narrow

# After read --map
Hint: ghx read --grep "function_name" file to see implementation
```

**Effort:** ~15 lines (1-2 echo statements per command).
**Impact:** Guides agent's next action. Prevents wrong tool usage. A 50-token hint that prevents a 5000-token wrong read is 100x ROI.

### Feature 5: `ghx read --minify`

**What:** Strip single-line comments and collapse consecutive blank lines before output.

**Why:** Octocode has per-language minification strategies (conservative for TS/Python, aggressive for Go/Java). Conservative minification (comments + blanks) saves 20-30% tokens with zero information loss for structural understanding.

**How:**
```bash
# Strip single-line comments (// and # but not #! shebangs), collapse blank lines
sed '/^[[:space:]]*\/\//d; /^[[:space:]]*#[^!]/d' | cat -s
```

**Effort:** ~5 lines (flag parsing + sed pipeline).
**Impact:** 20-30% token savings on verbose codebases. Stacks with --map for maximum compression.

**Inspiration:** Octocode's `packages/octocode-mcp/src/utils/core/minify.ts` — per-file-type strategies. We take the conservative approach (strip comments + blanks) which works across all languages.

## What v0.3 Adds to the Competitive Position

| Capability | ghx v0.2 | ghx v0.3 | Octocode MCP | GitHub MCP |
|---|---|---|---|---|
| Agent instruction detection | ❌ | ✅ (AGENTS.md, CLAUDE.md) | ❌ | ❌ |
| Tree filtering | ❌ | ✅ (smart defaults) | ✅ (85 folders) | ❌ |
| Token estimation | ❌ | ✅ (stderr warnings) | ✅ (3-tier) | ❌ |
| Next-action hints | ❌ | ✅ (stderr) | ✅ (JSON hints) | ❌ |
| Content minification | ❌ | ✅ (--minify) | ✅ (per-language) | ❌ |
| Context overhead | 0 tokens | 0 tokens | ~10K tokens | ~10K tokens |

**The pattern:** v0.3 brings every intelligence feature that makes Octocode valuable — without the MCP overhead. Same guidance, same protection, same filtering. Zero schema cost.

## What NOT to Build (with evidence)

### ❌ Issues/PRs/releases
gh has purpose-built commands (`gh issue list`, `gh pr view`, `gh pr diff`). GitHub's own gh-aw team builds skills for these (`query-prs.sh`, `query-issues.sh`, `query-discussions.sh`). ghx's niche is code exploration — the gap gh-aw explicitly doesn't cover (no `explore-repo.sh`, no `read-files.sh`, no `search-code.sh` in their skills directory).

### ❌ Pagination for search
9 req/min rate limit makes pagination expensive. ADR-0003 Decision 4: "refine don't paginate." Adding one qualifier is always better than fetching page 2. Evidence: Octocode doesn't paginate search results either — they cap at 30 and suggest refinement.

### ❌ Tree-sitter integration
Requires WASM binaries or native modules. Breaks the zero-dependency promise. Repomix already does this well (`npx repomix --remote owner/repo --compress`). ghx's regex-based `--map` gets 92% reduction vs Repomix's 57% on the same files — because we extract ONLY structural declarations while Repomix keeps comments and interface bodies. The trade-off (missing multi-line signatures) is acceptable for structure scanning.

### ❌ MCP server
ghx's core value proposition is zero context overhead. An MCP wrapper would add ~10K tokens of tool schemas — the exact problem ghx exists to solve. If you want MCP, use Octocode or GitHub MCP. If you want zero overhead, use ghx. Don't dilute the niche.

### ❌ Caching layer
`gh api --cache 1h` already exists. Adding a separate cache to ghx would mean managing state, cache invalidation, and stale data. Let gh handle caching. Document `--cache` usage in SKILL.md as a best practice.

### ❌ Progressive detail reduction (for now)
Codemap's `fitToBudget()` algorithm is elegant — reduce fidelity per-file until output fits a token budget. But it requires knowing the budget upfront, which agents don't always specify. For v0.3, token estimation + hints ("consider --map") achieves 80% of the value with 5% of the complexity. Revisit when usage patterns show agents consistently hitting token limits.

## Implementation Order

1. **Agent instruction detection** — highest impact, lowest effort (2 GraphQL aliases)
2. **Tree filtering** — high impact, enables cleaner explore output
3. **Token estimation** — defensive, prevents context overflow
4. **Hints** — guides next action, builds on token estimation
5. **--minify** — token savings, independent of other features

Total estimated addition: ~50 lines of bash. Script grows from ~345 to ~395 lines. Still well within bash territory (Go trigger: 500+ lines per ADR-0002).

## The Bigger Picture

### v0.2 → v0.3 arc

v0.2 answered: "How do I efficiently read code on GitHub?"
v0.3 answers: "How do I efficiently UNDERSTAND a repo I've never seen?"

The difference is intelligence. v0.2 is a better pipe — fewer API calls, less noise, smarter defaults. v0.3 is a better guide — it tells you what matters, warns you about what's big, and suggests what to do next.

### The ghx philosophy, restated

ghx is not a GitHub client. It's an **agent's intuition about GitHub**, encoded in bash. When a human developer opens a new repo, they instinctively:
1. Read the README
2. Scan the file tree (ignoring node_modules, dist, .lock files)
3. Look for CONTRIBUTING.md or similar conventions
4. Open a few key files to understand the architecture
5. Search for specific patterns

ghx encodes this intuition into commands. `explore` does steps 1-3. `read --map` does step 4. `search` does step 5. The agent doesn't need to know the intuition — it's built into the defaults.

### What comes after v0.3

**v0.4 candidates** (evaluate based on v0.3 usage patterns):
- `ghx diff <owner/repo> <pr-number>` — PR diff with token protection (truncate large hunks, show file summary first). Inspired by gh-aw's schema-first pattern.
- `ghx clone <owner/repo>` — sparse checkout for deep local analysis. Bridge to local tools (ripgrep, Tree-sitter). Inspired by gitingest's sparse clone strategy.
- Progressive `--map` levels (compact → minimal → outline) — inspired by Codemap's `fitToBudget()`.
- `gh api --cache` internally for repeated reads within a session.

**Go rewrite trigger** (from ADR-0002): when the script exceeds ~500 lines, or when Windows native support becomes a real requirement, or when `gh` CLI dependency becomes friction. At ~395 lines post-v0.3, we're approaching but not at the threshold.

## Sources

- Octocode MCP: `bgauryy/octocode-mcp` — hints (`response/hints.ts`), filtering (`core/filter.ts`), token warnings (`response/tokenWarning.ts`), minification (`core/minify.ts`)
- gh-aw skills: `github/gh-aw` — schema-first pattern (`skills/github-pr-query/query-prs.sh`), developer skill (`skills/developer/SKILL.md`)
- Codemap: `kcosr/codemap` — progressive detail reduction (`src/sourceMap.ts` `fitToBudget()`), token estimation (`Math.ceil(rendered.length / 4)`)
- Repomix: `yamadashy/repomix` — Tree-sitter compression (`src/core/treeSitter/`), git change frequency (`src/core/git/gitCommand.ts`)
- Aider: `Aider-AI/aider` — PageRank file ranking (`aider/repomap.py`), token budget binary search
- Agent instruction files: empirical survey of 9 major repos via GraphQL (2026-03-08)
- GitHub rate limits: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- Blackbird engine: https://github.blog/2023-02-06-the-technology-behind-githubs-new-code-search/
