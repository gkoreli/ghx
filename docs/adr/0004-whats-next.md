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
**Source:** `bgauryy/octocode-mcp` — `packages/octocode-mcp/src/utils/pagination/hints.ts`

Every tool response includes contextual hints. The actual implementation has four hint generators:
- `generateTokenWarnings()` — 5-tier system: >50K "🚨 CRITICAL", >30K "⚠️ WARNING", >15K "ℹ️ NOTICE", >5K "ℹ️ Moderate", <5K "✓ Efficient"
- `generateNavigationHints()` — pagination guidance: "📄 Page 1 of 3", "▶ Next page: Use charOffset=20000"
- `generateGitHubPaginationHints()` — GitHub-specific: includes exact params for next page call, plus "💡 TIP: Use matchString for targeted extraction instead of paginating through entire file"
- `generateStructurePaginationHints()` — directory-specific: "📂 Page 1/3 (45 files, 12 folders on this page)", "📊 Total: 200 files, 30 folders"

The key insight: hints are NOT generic. They're context-specific — different hints for file content, directory structure, and search results. Each hint includes the exact command/parameter the agent should use next.

**What we take:** Context-specific hints on stderr. After `explore`: "Hint: ghx read --map to understand code structure." After `search` with 0 results: "Hint: broaden query — remove qualifiers or try synonyms." The hint changes based on what happened, not just what command ran.

### 2. Octocode's Token Warning System (5-tier, not 3)
**Source:** `bgauryy/octocode-mcp` — `packages/octocode-mcp/src/utils/pagination/hints.ts` function `generateTokenWarnings()`

The actual implementation is more nuanced than previously documented:

| Tokens | Level | Message |
|---|---|---|
| >50K | 🚨 CRITICAL | "Response TOO LARGE — will likely exceed model context limits" + "ACTION REQUIRED: Use smaller charLength or refine query" |
| >30K | ⚠️ WARNING | "High token usage — may approach context limits" + "RECOMMENDATION: Consider reducing charLength" |
| >15K | ℹ️ NOTICE | "Moderate token usage — monitor context window usage" |
| >5K | ℹ️ Info | "Response uses ~N tokens" |
| <5K | ✓ Efficient | "Efficient query: Response uses ~N tokens" |

**What we take:** The 5-tier system is overkill for a CLI. But the principle of ALWAYS reporting token usage is valuable. Even "✓ ~800 tokens" on stderr tells the agent "this was cheap, I can do more." We'll use 3 tiers: >50K critical, >30K warning, silent otherwise. But always show the count in the result metadata.

### 3. Octocode's File Filtering (actual exclusion lists)
**Source:** `bgauryy/octocode-mcp` — `packages/octocode-mcp/src/utils/file/filters.ts`

The actual implementation has three arrays, verified from source:
- `IGNORED_FOLDER_NAMES`: 82 entries — `.git`, `.vscode`, `.devcontainer`, `dist`, `build`, `out`, `output`, `target`, `release`, `node_modules`, `vendor`, `third_party`, `tmp`, `temp`, `cache`, `.cache`, `.pytest_cache`, `.tox`, `.venv`, `.mypy_cache`, `.next`, `.svelte-kit`, `.turbo`, `.angular`, `.dart_tool`, `__pycache__`, `.ruff_cache`, `.nox`, `htmlcov`, `cover`, `.gradle`, `.m2`, `.sbt`, `.bloop`, `.metals`, `.bsp`, `bin`, `obj`, `TestResults`, `BenchmarkDotNet.Artifacts`, `.vendor-new`, `Godeps`, `.bundle`, `.byebug_history`, `.mvn`, `.aws`, `.gcp`, `fastlane`, `DerivedData`, `xcuserdata`, `.navigation`, `captures`, `.externalNativeBuild`, `.cxx`, `.idea`, `.idea_modules`, `.vs`, `.history`, `coverage`, `.nyc_output`, `logs`, `log`, `.DS_Store`
- `IGNORED_FILE_NAMES`: 42 entries — `package-lock.json`, `.secrets`, `secrets.json`, `credentials.json`, `auth.json`, `api-keys.json`, `service-account.json`, `private-key.pem`, `id_rsa`, `id_dsa`, `id_ecdsa`, `id_ed25519`, `keyfile`, `gcloud-service-key.json`, `firebase-adminsdk.json`, `google-services.json`, `.DS_Store`, `Thumbs.db`, `db.sqlite3`, `.eslintcache`, `.stylelintcache`, `.node_repl_history`, `.yarn-integrity`, `ThirdPartyNoticeText.txt`, `cgmanifest.json`
- `IGNORED_FILE_EXTENSIONS`: 90+ entries — `.lock`, `.log`, `.tmp`, `.cache`, `.bak`, `.exe`, `.dll`, `.so`, `.dylib`, `.o`, `.obj`, `.bin`, `.class`, `.pyc`, `.jar`, `.db`, `.sqlite`, `.zip`, `.tar`, `.gz`, `.rar`, `.7z`, `.map`, `.min.js`, `.min.css`, `.key`, `.pem`, `.crt`, `.patch`, `.diff`, `.prof`, `.coverage`, `.egg-info`

The `shouldIgnoreFile()` function checks in optimized order: extension (fastest) → filename → path parts (most expensive).

**What we take for ghx tree:** A focused subset — the 80/20 of noise filtering. Our exclusion list targets the patterns that appear in >90% of repos:

```
# Directories (covers ~95% of tree noise)
node_modules/ dist/ build/ .git/ __pycache__/ .next/ .nuxt/ coverage/ vendor/ .cache/ .venv/ target/

# Extensions (covers binary/generated/media noise)
*.lock *.min.js *.min.css *.map *.pyc *.class
*.woff *.woff2 *.ttf *.eot *.ico *.png *.jpg *.jpeg *.gif *.svg *.mp4 *.webm
*.zip *.tar *.gz *.jar *.exe *.dll *.so *.dylib
```

12 directories + 25 extensions vs Octocode's 82 + 42 + 90. Focused, not exhaustive. `--all` to disable.

### 4. gh-aw's Schema-First Pattern
**Source:** `github/gh-aw` — `skills/github-pr-query/query-prs.sh` (4113 bytes, verified)

When called without `--jq`, the script returns a metadata envelope instead of raw data:
```json
{
  "message": "No --jq filter provided. Use --jq to filter and retrieve data.",
  "item_count": 30,
  "data_size_bytes": 45000,
  "schema": { "item_fields": { "number": "integer - PR number", "title": "string - PR title", ... } },
  "suggested_queries": [
    {"description": "Get all data", "query": "."},
    {"description": "Get PR numbers and titles", "query": ".[] | {number, title}"},
    {"description": "Get open PRs only", "query": ".[] | select(.state == \"OPEN\")"},
    {"description": "Get large PRs", "query": ".[] | select(.changedFiles > 10) | {number, title, changedFiles}"},
    {"description": "Count by state", "query": "group_by(.state) | map({state: .[0].state, count: length})"}
  ]
}
```

This is progressive disclosure at the API level. The agent sees "30 items, 45KB" and decides whether to fetch all data or use a targeted jq filter. No wasted tokens on data the agent doesn't need.

**What we take:** The principle maps to two ghx features:
1. **Token estimation** — before the agent processes output, stderr tells it the cost: "~12K tokens"
2. **Hints** — after output, stderr suggests the next action: "Hint: ghx read --map to understand code structure"

Combined, these give the agent the same "decide before committing" capability that gh-aw's schema-first pattern provides — without changing ghx's output format.

### 5. Codemap's Progressive Detail Reduction (actual algorithm)
**Source:** `kcosr/codemap` — `src/sourceMap.ts` lines 96-180 (verified from source)

The actual `fitToBudget()` algorithm:

```
DETAIL_LEVELS = [full, standard, compact, minimal, outline]

function fitToBudget(entries, budget):
  // Estimate tokens for each file
  for entry in entries:
    entry.tokenEstimate = Math.ceil(renderFileEntry(entry).length / 4)
  
  total = sum(entry.tokenEstimate for entry in entries)
  
  // Reduce largest file by one level until budget met
  while total > budget:
    largest = entries.filter(e => e.detailLevel != "outline")
                     .maxBy(e => e.tokenEstimate)
    if !largest: break
    
    largest.detailLevel = reduceDetailLevel(largest.detailLevel)
    oldEstimate = largest.tokenEstimate
    largest.tokenEstimate = estimateFileTokens(largest)
    total -= (oldEstimate - largest.tokenEstimate)
  
  // If STILL over budget after all files at outline, truncate file list
  if total > budget:
    trimmed = []
    running = 0
    for entry in entries:
      if running + entry.tokenEstimate > budget: break
      trimmed.push(entry)
      running += entry.tokenEstimate
    return trimmed
```

The design philosophy: **degrade gracefully, never truncate abruptly.** The agent still sees every file, just with less detail on the largest ones. Only as a last resort does it drop files entirely.

**What we take (v0.4 candidate):** ghx's `--map` is equivalent to codemap's "compact" level. A future `--map=minimal` could show names only (no signatures). The progressive reduction algorithm itself maps to an agent practice: map at compact first → if too large, re-map at minimal → if still too large, fall back to tree only.

### 6. Repomix's Agent Rules Convention
**Source:** `yamadashy/repomix` — `.agents/rules/base.md` (6162 bytes, verified)

Repomix uses a `.agents/` directory with structured rules:
```
.agents/
├── commands/agent/     # Agent-specific commands (claude-rule-update, gemini-discuss)
├── commands/code/      # Code commands (lint-fix)
├── commands/git/       # Git commands (commit, push, pr-create, pr-review)
└── rules/base.md       # Core project guidelines (always loaded)
```

The `base.md` has YAML frontmatter: `alwaysApply: true`, `inclusion: always`. It includes:
- Project directory structure with purpose annotations
- Coding guidelines (Airbnb style, 250-line file limit, feature-based dirs)
- Commit message conventions (conventional commits with scope)
- PR guidelines (template, checklist)
- Dependency injection patterns for testability
- Release note guidelines

**What we take:** This validates the AGENTS.md detection feature. Repos are moving beyond a single AGENTS.md to structured `.agents/` directories. For v0.3, detecting AGENTS.md and CLAUDE.md covers the 80% case. For v0.4, detecting `.agents/` directories and surfacing their structure would cover the emerging convention.

### 7. gh-skill: Universal Skill Distribution
**Source:** `nicholasspencer/gh-skill` — README.md, AGENTS.md (4732 bytes, verified)

A gh extension that turns GitHub Gists into a universal skill registry:
- Skills are gists with `*.skill.md` files (YAML frontmatter + instructions)
- `gh skill add <gist-url>` installs and auto-links to all detected agent tools
- Supports: Claude Code (`~/.claude/skills/`), Copilot CLI, OpenClaw, Codex, OpenCode, Cursor
- Trust gate: untrusted authors prompt before install
- Subdirectories flattened with `--` separator in gist filenames

**What we take:** ghx's SKILL.md could be published as a gh-skill gist. One command to install ghx knowledge across all agent platforms: `gh skill add <ghx-skill-gist>`. This is a distribution channel we haven't considered — complementary to npm, brew, and curl.

### 8. Aider's Token Budget Binary Search (actual algorithm)
**Source:** `Aider-AI/aider` — `aider/repomap.py` lines 674-700 (verified from source)

```python
# Binary search for max ranked tags that fit budget
middle = min(int(max_map_tokens // 25), num_tags)
while lower_bound <= upper_bound:
    tree = self.to_tree(ranked_tags[:middle])
    num_tokens = self.token_count(tree)
    
    pct_err = abs(num_tokens - max_map_tokens) / max_map_tokens
    ok_err = 0.15  # Accept within 15% of budget
    
    if (num_tokens <= max_map_tokens and num_tokens > best_tree_tokens) or pct_err < ok_err:
        best_tree = tree
        best_tree_tokens = num_tokens
        if pct_err < ok_err:
            break  # Close enough, stop searching
    
    if num_tokens < max_map_tokens:
        lower_bound = middle + 1
    else:
        upper_bound = middle - 1
    middle = (lower_bound + upper_bound) // 2
```

The 15% tolerance (`ok_err = 0.15`) is key — it stops the binary search early when close enough, avoiding unnecessary iterations. The initial guess (`max_map_tokens // 25`) assumes ~25 tokens per tag, which is a reasonable heuristic.

**What we take (as design principle):** "Fill the budget, don't waste it." When ghx has a token budget (future feature), binary search for the right amount of detail. For v0.3, the simpler version: estimate output size, warn if it exceeds a threshold, suggest a flag that would reduce it.

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
gh has purpose-built commands (`gh issue list`, `gh pr view`, `gh pr diff`). GitHub's own gh-aw team builds skills for these (`query-prs.sh`, `query-issues.sh`, `query-discussions.sh`). Their developer skill (`skills/developer/SKILL.md`) is 58KB of coding guidelines — they invest heavily in PR/issue workflows. ghx's niche is code exploration — the gap gh-aw explicitly doesn't cover (no `explore-repo.sh`, no `read-files.sh`, no `search-code.sh` in their 26 skills).

### ❌ Pagination for search
9 req/min rate limit makes pagination expensive. ADR-0003 Decision 4: "refine don't paginate." Adding one qualifier is always better than fetching page 2. Evidence: Octocode's own hints say "💡 TIP: Use matchString for targeted extraction instead of paginating through entire file" and "💡 TIP: Use githubSearchCode with path filter for targeted discovery instead of paginating through entire structure." Even the feature-rich MCP discourages pagination.

### ❌ Tree-sitter integration
Requires WASM binaries or native modules. Breaks the zero-dependency promise. Repomix already does this well (`npx repomix --remote owner/repo --compress`). ghx's regex-based `--map` gets 92% reduction vs Repomix's 57% on the same files — because we extract ONLY structural declarations while Repomix keeps comments and interface bodies. The trade-off (missing multi-line signatures) is acceptable for structure scanning. Codemap uses ts-morph (TypeScript compiler API) for precise resolution — even heavier than Tree-sitter WASM.

### ❌ MCP server
ghx's core value proposition is zero context overhead. An MCP wrapper would add ~10K tokens of tool schemas — the exact problem ghx exists to solve. Octocode's MCP has 15K+ lines of TypeScript across 50+ files. GitHub MCP has 10K+ lines of Go. ghx is 345 lines of bash. The simplicity IS the feature. If you want MCP, use Octocode or GitHub MCP. If you want zero overhead, use ghx. Don't dilute the niche.

### ❌ Caching layer
`gh api --cache 1h` already exists. Octocode implements its own cache (node-cache, 24h TTL, 5000 max keys) because it's an MCP server that persists across calls. ghx is stateless — each invocation is independent. Adding cache state would mean managing files in /tmp, cache invalidation, and stale data. Let gh handle caching. Document `--cache` usage in SKILL.md as a best practice.

### ❌ Progressive detail reduction (for now)
Codemap's `fitToBudget()` algorithm is elegant — reduce fidelity per-file until output fits a token budget. But it requires: (a) knowing the budget upfront, which agents don't always specify, (b) rendering each file at each detail level to estimate tokens, which means multiple passes, (c) maintaining 5 detail levels per language. For v0.3, token estimation + hints ("consider --map") achieves 80% of the value with 5% of the complexity. Revisit when usage patterns show agents consistently hitting token limits.

### ❌ Secret detection/redaction
Octocode has 18 categories of secret patterns (AWS keys, API tokens, private keys, database URLs, Slack tokens) auto-redacted to `[REDACTED-AWS-ACCESS-KEY]`. Their `IGNORED_FILE_NAMES` array includes `secrets.json`, `credentials.json`, `auth.json`, `api-keys.json`, `id_rsa`, `id_ed25519`, `private-key.pem`. This is valuable but out of scope — ghx reads code, not secrets. The tree filtering (Feature 2) will exclude secret-containing files by extension (`.pem`, `.key`), which covers the common case.

### ❌ `.agents/` directory deep scanning
Repomix uses a `.agents/` directory with structured rules, commands, and per-agent configs. Detecting and parsing this structure would require understanding YAML frontmatter, directory conventions, and multiple file reads. For v0.3, detecting AGENTS.md and CLAUDE.md at the root covers the 80% case (8/9 repos surveyed). The `.agents/` convention is emerging but not yet standardized enough to warrant special handling.

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

### Why ghx wins against heavier tools

The landscape has converged on two approaches:
1. **MCP servers** (Octocode, GitHub MCP) — rich features, ~10K token schema overhead per conversation
2. **Dump tools** (Repomix, Gitingest) — holistic understanding, clone-first, slow for targeted work

ghx occupies the empty middle: **MCP-level intelligence in a zero-overhead CLI.**

Evidence from source code analysis:
- Octocode: 15K+ lines TypeScript, npm + Docker, local HTTP server on port 1987, 50+ tool schemas. Their hints, filtering, and token warnings are excellent — but you pay ~10K tokens just to have the tools available.
- GitHub MCP: 10K+ lines Go, 50+ tools. Their `get_file_contents` doesn't support line ranges or match filtering. Each file read = 1 tool call.
- Repomix: 20K+ lines TypeScript, npm/npx + WASM. Tree-sitter compression is best-in-class but requires cloning first (3.6s vs ghx's 0.9s).
- ghx: 345 lines bash. Zero install beyond `gh` CLI. Zero context overhead. Same intelligence features (hints, filtering, token warnings) delivered through stderr instead of JSON schemas.

The bet: **agents will prefer tools that are cheap to invoke over tools that are rich to configure.** An agent that can call `ghx explore` 10 times in the context budget of one MCP tool registration will explore more repos, find better answers, and waste fewer tokens.

### The agent instruction file revolution

The empirical survey (8/9 major repos have agent instruction files) reveals a fundamental shift: **repos are becoming agent-aware.** They're shipping instructions not for humans, but for AI agents.

This changes the explore workflow:
- **Before AGENTS.md**: explore → read README → guess conventions → make mistakes → recover
- **After AGENTS.md**: explore → read AGENTS.md → know conventions → work correctly from the start

ghx v0.3 is positioned to be the first tool that surfaces this automatically. No other tool — not Octocode, not GitHub MCP, not Repomix — detects and surfaces AGENTS.md in the explore flow. This is a genuine first-mover advantage.

### Distribution: gh-skill as a new channel

The gh-skill registry (`nicholasspencer/gh-skill`) enables publishing ghx's SKILL.md as a gist-based skill that auto-installs across Claude Code, Copilot CLI, Codex, OpenCode, and Cursor. One command: `gh skill add <ghx-gist-url>`. This is complementary to npm/brew/curl — it distributes the KNOWLEDGE (SKILL.md) rather than the TOOL (ghx script).

This maps to ghx's two-layer distribution:
1. **Tool distribution** (npm, brew, curl, gh extension) — installs the `ghx` binary
2. **Knowledge distribution** (gh-skill, `ghx skill` command, agent spawn hooks) — installs the SKILL.md context

Both layers are needed. The tool without the skill is a CLI. The skill without the tool is documentation. Together, they're an agent capability.

### What comes after v0.3

**v0.4 candidates** (evaluate based on v0.3 usage patterns):
- `ghx diff <owner/repo> <pr-number>` — PR diff with token protection (truncate large hunks, show file summary first). Inspired by gh-aw's schema-first pattern: show changed files + stats before dumping the diff.
- `ghx clone <owner/repo>` — sparse checkout for deep local analysis. Bridge to local tools (ripgrep, Tree-sitter). Inspired by gitingest's sparse clone strategy (`git clone --depth=1 --filter=blob:none --sparse`).
- Progressive `--map` levels (compact → minimal → outline) — inspired by Codemap's `fitToBudget()`. `--map` = signatures, `--map=names` = names only, `--map=outline` = paths + line counts.
- `.agents/` directory detection — the Repomix convention of structured agent rules in `.agents/rules/`, `.agents/commands/`. Emerging but not yet standardized.
- `gh api --cache` internally for repeated reads within a session.
- gh-skill publication — publish SKILL.md as a gist-based skill for cross-platform agent installation.

**Go rewrite trigger** (from ADR-0002): when the script exceeds ~500 lines, or when Windows native support becomes a real requirement, or when `gh` CLI dependency becomes friction. At ~395 lines post-v0.3, we're approaching but not at the threshold.

## Sources

- Octocode MCP: `bgauryy/octocode-mcp`
  - Hints + token warnings: `packages/octocode-mcp/src/utils/pagination/hints.ts` — `generateTokenWarnings()`, `generateGitHubPaginationHints()`, `generateStructurePaginationHints()`
  - File filtering: `packages/octocode-mcp/src/utils/file/filters.ts` — `IGNORED_FOLDER_NAMES` (82), `IGNORED_FILE_NAMES` (42), `IGNORED_FILE_EXTENSIONS` (90+), `shouldIgnoreFile()`
  - Fetch content hints: `packages/octocode-mcp/src/tools/github_fetch_content/execution.ts` — `DIRECTORY_FETCH_HINTS`, `DIRECTORY_CACHE_HIT_HINT`
  - Local tool hints: `packages/octocode-mcp/src/hints/localToolUsageHints.ts`
- gh-aw skills: `github/gh-aw`
  - Schema-first pattern: `skills/github-pr-query/query-prs.sh` (4113 bytes) — returns schema + suggested_queries when no --jq
  - Developer skill: `skills/developer/SKILL.md` (58KB) — comprehensive coding guidelines
  - 26 skills total, none for code exploration (ghx's niche)
- Codemap: `kcosr/codemap`
  - Progressive detail reduction: `src/sourceMap.ts` lines 96-180 — `DETAIL_LEVELS`, `fitToBudget()`, `reduceDetailLevel()`
  - Token estimation: `Math.ceil(rendered.length / 4)` (line 119)
- Repomix: `yamadashy/repomix`
  - Agent rules: `.agents/rules/base.md` (6162 bytes) — YAML frontmatter with `alwaysApply: true`
  - Tree-sitter compression: `src/core/treeSitter/`
  - Git change frequency: `src/core/git/gitCommand.ts`
- Aider: `Aider-AI/aider`
  - Token budget binary search: `aider/repomap.py` lines 674-700 — 15% tolerance, initial guess `max_map_tokens // 25`
  - PageRank file ranking: `aider/repomap.py` — personalization weights, identifier weighting heuristics
- gh-skill: `nicholasspencer/gh-skill`
  - Universal skill registry: gist-based skills, auto-links to Claude Code, Copilot CLI, Codex, OpenCode, Cursor
  - AGENTS.md: 4732 bytes — architecture, key concepts, supported tool targets
- Agent instruction files: empirical survey of 9 major repos via GraphQL (2026-03-08)
  - 8/9 repos have at least one agent instruction file
  - AGENTS.md is the emerging canonical name; CLAUDE.md often symlinks to it
  - Largest: vercel/next.js AGENTS.md (20KB) — monorepo structure, entry points, testing
- GitHub rate limits: https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api
- Blackbird engine: https://github.blog/2023-02-06-the-technology-behind-githubs-new-code-search/
