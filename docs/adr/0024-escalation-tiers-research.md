---
title: "ADR-0024: Escalation Tiers (M7) — Research"
date: 2026-07-05
status: research
thread: escalation-tiers
author: Goga Koreli
---

# 0024. Escalation Tiers (M7) — Research

> Research memo grounding the future M7 decision ADR. Tier 2 is where the
> sidecar decides to pull a repo locally and run structural analysis (codemap
> et al.) as internal tools, invisible to the main agent except through the
> report's tier visibility field. This file collects evidence; the decision ADR
> (0024.1) writes the constraints into decisions.

---

## Repo context reviewed

| File | What it fixes in scope |
|------|------------------------|
| `docs/NORTH_STAR.md` P3, §1, M7, tenets | Tier model origin, "remote-first, escalation explicit", M7 goals |
| `docs/adr/0014.1-sidecar-agent-vision.md` | Tier 0–3 first definition, Codemap as optional backend |
| `docs/adr/0019-sidecar-adoption-zero-cli-surface.md` | ADR-0014.1 tiers "become visible in reports at M7"; `AllowedBackends` field in `Request` |
| `docs/adr/0022-shared-visibility-runtime.md` | M7/M8 tier decisions emit into the shared OTel stream (D2 follow-up) |
| `internal/sidecar/prompt.go` | Current persona: `AllowedBackends []string` default `["remote"]`; "name the deeper backend needed but do not perform it unless allowed" |
| `internal/mapengine/types.go` | File-level symbol extraction only: func/type/const/var/import/package — no cross-file reference graph |

---

## Q1 — codemap: current capabilities

### VERIFIED

- **Repo** `JordanCoin/codemap` — MIT, Go ≥ 1.21, 632 stars, 52 forks;
  latest release v4.1.10 published 2026-07-02; 5 releases in 90 days
  (v4.1.6 2026-04-08 → v4.1.10 2026-07-02). Active cadence.
  Source: `gh api repos/JordanCoin/codemap`, accessed 2026-07-05.

- **Install story** — three paths supported:
  (1) Homebrew (`brew tap JordanCoin/tap && brew install codemap`);
  (2) Scoop on Windows;
  (3) `go install` / release tarball.
  `--deps` flag requires `ast-grep` as a separate binary; `codemap-full`
  tarball bundles both. Not a Go importable library — invoked via `exec`.
  Source: README at `github.com/JordanCoin/codemap`, accessed 2026-07-05.

- **CLI surface relevant to M7 integration**:

  | Command | What it does | Notes |
  |---------|--------------|-------|
  | `codemap .` | Fast tree + hub summary (compact context) | Default; respects `.codemap/config.json` |
  | `codemap --json .` | Same as above, JSON output | Machine-readable; deterministic hash |
  | `codemap context` | Universal JSON `ContextEnvelope` for any AI tool | Includes intent, working set, matched skills, handoff ref |
  | `codemap context --compact` | Minimal envelope for token-constrained agents | |
  | `codemap --deps .` | Dependency flow: hub files, import chains | Requires `ast-grep` binary |
  | `codemap --importers <file>` | Who imports a file (fan-in) | Blast radius input |
  | `codemap blast-radius --json --ref main` | Files changed + importers → blast radius | Requires `ast-grep` |
  | `codemap github.com/user/repo` | Remote repo tree (clones temporarily) | Source: README "Other Commands" |
  | `codemap mcp` | Run as stdio MCP server | |
  | `codemap serve --port 9471` | HTTP API (REST) | |

- **Output formats**: Markdown, text, JSON. Outputs are deterministic and
  content-hashed; `context` envelope carries `prefix_hash`, `delta_hash`,
  `combined_hash` enabling cache-reuse across turns.

- **What subset serves "deeper structural understanding"**:
  The `--deps` + `--importers` + `blast-radius` trio answers "what connects
  to X" and "what breaks if I change Y" for import-level dependency graphs.
  `codemap context --json` gives the agent-ready envelope. The `--diff`/
  `--ref` flags give a changed-file set, useful as the trigger signal.
  NOT provided by codemap: precise cross-file definition/reference graphs
  (call graph, symbol go-to-definition, interface implementors). Those
  require SCIP-class indexers or stack-graphs.

### INFERRED

- Remote repo support (`codemap github.com/user/repo`) likely performs a
  temporary local clone; the README does not document whether it respects
  partial-clone filters or caches the clone. This matters for M7 latency.
  **Needs verification before M7 build.**

- The `context` envelope with deterministic hashes is a natural cache key for
  the sidecar session ledger: a hash match means "same repo state, reuse tier-2
  artifacts."

---

## Q2 — Alternatives/complements for local structural analysis

### VERIFIED

**ast-grep** (`github.com/ast-grep/ast-grep`): Rust, supports 130+ languages,
pattern-based AST structural search. Ships an MCP server (`ast-grep-mcp`) and
an agent skill. v0.43 (2026) adds Markdown support.
Use: "find all call sites matching pattern P" — structural grep, not a
reference graph. Does NOT build a cross-file symbol graph.
Source: `github.com/ast-grep/ast-grep`, `ast-grep.github.io`, accessed 2026-07-05.

**Aider repomap** (`github.com/Aider-AI/aider`): tree-sitter tag extraction
(definitions + references) → directed graph → personalized PageRank → top
files ranked by relevance to chat context. SQLite-cached with mtime
invalidation. Token-budgeted (default 1K tokens). Solves "which files matter
for this chat" by importance ranking, NOT "trace this exact call path."
Source: DeepWiki `Aider-AI/aider/4.1-repository-mapping`, accessed 2026-07-05.

**SCIP** (`github.com/sourcegraph/scip`): language-agnostic code intelligence
format for precise go-to-definition and find-references. Requires per-language
indexers (`scip-typescript`, `scip-python`, `scip-go`, `scip-java`); most
require a full build/type-check pass. Produces an index file; CLI (`scip
snapshot`, `scip print`) inspects it. 10× CI speedup vs LSIF in Sourcegraph's
own data. Heavy — unsuitable for "clone and index in <60s."
Source: `sourcegraph.com/blog/announcing-scip`,
`github.com/sourcegraph/scip`, accessed 2026-07-05.

**Stack-graphs** (`github.com/github/stack-graphs`): tree-sitter-based name
resolution at scale. Purely syntactic (no build step). Incremental,
SQLite-persisted. CLI available via `tree-sitter-stack-graphs`. GitHub uses
this for code navigation on github.com. Zero-config for supported languages.
For M7: best cross-file reference option that does NOT need a build phase.
Source: `github.com/github/stack-graphs`, `crates.io/crates/tree-sitter-stack-graphs`,
accessed 2026-07-05.

**universal-ctags**: JSON output (`--output-format=json`) if built with
libjansson. ~150 languages. Returns symbol kind, name, line, signature. Fast,
no reference graph. Good for rapid symbol inventory as a Tier 1.5 step.
Source: `docs.ctags.io/en/latest/man/ctags-json-output.5.html`, accessed 2026-07-05.

**"Code Isn't Memory" paper** (arxiv 2606.22417, 2026): empirically tested
structural codebase index inside a coding agent. Finding: structural indexing
delivers "large localization gain...with no cost penalty" and lower cost per
solved task vs agentic-grep. Key threshold: "The deployment question is not
whether it is too expensive to run, but whether the workload includes
multi-file changes where structural ranking pays off." Single-file edits:
grep suffices. Multi-file change propagation: structural ranking pays.
Source: `arxiv.org/abs/2606.22417`, accessed 2026-07-05.

### Overlap with `internal/mapengine` — honest accounting

`internal/mapengine` (Go AST + tree-sitter + regex fallback) extracts
file-level symbols: func, type, const, var, import, package — with line
numbers and signatures. Already covers:

- Fast single-file symbol inventory (no clone needed; works on fetched content)
- Language autodetection for Go, TS/TSX, JS/JSX, Python, Rust
- Compact map output already in use for `ghx read --map`

`internal/mapengine` does NOT cover:

- Cross-file reference graph (who calls X, who imports X across the whole repo)
- Dependency flow / import chain graph
- Blast radius (which files break if X changes)
- Importance ranking across a whole repo (repomap-style PageRank)
- Definition lookup from a reference site

Implication: `internal/mapengine` is already the Tier 1 structural primitive.
Tier 2 adds what mapengine fundamentally cannot: cross-file structure.
codemap's `--deps`/`--importers` fills the import-level gap cheaply.
Stack-graphs fills the reference/definition gap without a build step.
Neither duplicates mapengine; they layer on top.

---

## Q3 — Clone economics

### VERIFIED

**Partial clone strategies** (GitHub Blog data study, accessed 2026-07-05):

| Strategy | Filter | Linux kernel clone time | Notes |
|----------|--------|------------------------|-------|
| Full clone | none | ~20 min / 4.6 GB | Baseline |
| Blobless | `--filter=blob:none` | ~4.5 min / 1.4 GB | Downloads commits + trees; defers blobs |
| Treeless | `--filter=tree:0` | ~2.4 min | Fastest initial; server overhead on fetch |
| Shallow | `--depth=1` | ~30s (small repos) | Best for single-use; avoid shallow fetch |
| Blobless + sparse-checkout | both | <1 min (typical repo) | "particularly effective" per GitHub Blog |

Source: `github.blog/open-source/git/git-clone-a-data-driven-study-on-cloning-behaviors/`,
accessed 2026-07-05.

**GitHub rate limits for git operations** (2025): Git clone/fetch is NOT
subject to the REST API rate limit (5,000 req/hr authenticated); bandwidth
caps exist but are not publicly documented as hard numbers. REST API calls
from codemap or ghx during analysis count against the 5,000/hr limit.
Source: `docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api`,
`github.com/orgs/community/discussions/44515`, accessed 2026-07-05.

**Shallow fetch overhead**: "13–25× greater computational costs than full
fetches on larger repositories" — avoid for repos that may need multi-turn
analysis. Blobless or full clone preferred for reusable cache.
Source: GitHub Blog data study, ibid.

### INFERRED

**Cache layout convention**: ADR-0022 D3 established `~/.ghx` as the product
root. Natural extension for Tier 2: `~/.ghx/cache/<owner>/<repo>/<sha>/`
(blobless clone) or `~/.ghx/cache/<owner>/<repo>/sparse/<sha>/` (sparse
checkout). One dir per resolved commit SHA makes eviction deterministic (`rm
-rf`). No eviction policy formalized yet.

**Latency estimate for blobless + sparse**: most agent-targeted repos
(not Linux kernel) are 10–200 MB trees. Blobless clone: 5–30s expected.
With sparse checkout limiting to relevant subtrees: potentially 2–10s.
Cache hit (SHA matches): near-zero.

**Private repo handling**: clone requires write-capable token scope; the
existing `GH_TOKEN`/`GITHUB_TOKEN` story from ADR-0014.1's environment
contract applies. Tier 2 must re-run the preflight check with clone
permission explicitly confirmed.

---

## Q4 — Escalation decision signals

### VERIFIED

**A2RAG policy** (paper, 2025 — cited in search result for agentic retrieval):
"local-first, escalation-on-demand policy, beginning with inexpensive local
operations and escalating to more global actions only when accumulated evidence
is judged insufficient, performing stage-wise evidence sufficiency checks."
This is the closest published analog to ghx's tier design.
Source: `arxiv.org/pdf/2601.21162`, accessed 2026-07-05 (indirect, via search summary).

**"Code Isn't Memory" finding** (arxiv 2606.22417): structural indexing pays
for "multi-file changes where structural ranking pays off." Single-file edits
do not need it.
Source: ibid., accessed 2026-07-05.

**Aider repomap boost heuristic**: rank boost for files whose path components
match identifiers mentioned in the chat. Implies: identifier match is a
positive signal, but absence of match after tree+grep is an escalation signal.
Source: DeepWiki `Aider-AI/aider/4.1-repository-mapping`, accessed 2026-07-05.

### INFERRED

Synthesizing from the above and the existing ADR-0014.1 tier model, candidate
escalation signals (not decisions):

| Signal class | Observable trigger | Tier 0/1 state that fires it |
|---|---|---|
| Grep sparsity | Zero or one hit after 2+ pattern attempts across mapped globs | `ghx read --grep` returns no relevant matches |
| Symbol not found in map | Target symbol absent from all file maps in candidate paths | `ghx read --map` output contains no matching symbol |
| Cross-file reference question | Question contains "what calls", "what imports", "blast radius", "what breaks if I change", "interface implementations" | Question classification at turn start |
| Unresolvable reference chain | File A referenced from B is not visible via remote tree/read | Map shows import but target file unreadable remotely |
| Budget exhausted without answer | `submit_report` called with low-confidence answer after N turns | Report `confidence: low` + `backendsUsed: ["remote"]` only |
| Explicit question class | "call graph", "dependency graph", "trace call path", "full reference" | Verbatim question text |

No published paper provides a quantitative threshold (e.g., "escalate when
grep recall < 0.3"). This is an open research question; the M7 ADR should
pre-register a starting heuristic and measure it via SAFE evals.

---

## Q5 — Visibility precedents

### VERIFIED

**OTel GenAI semantic conventions v1.42.0** (opentelemetry.io, accessed
2026-07-05): tool-call spans use `gen_ai.operation.name=execute_tool` and
carry `gen_ai.tool.name`, `gen_ai.tool.call.id`. Backend identification in
spans is covered by `gen_ai.system` (model system) and resource/scope
attributes. No standard attribute for "which sidecar tier answered" yet —
this is a ghx-specific attribute under the existing `ghx.*` namespace.
Source: `opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/`, accessed 2026-07-05.

**Evidence tracing paper** (arxiv 2606.04990, 2026): defines execution
provenance as "typed graph of an agent execution, with evidence tracing as
its projection onto evidence-support relations." Covers "trace sources,
evidence and execution units, provenance relations." Validates the requirement
that every claim's supporting tool call and backend be recorded.
Source: `arxiv.org/abs/2606.04990`, accessed 2026-07-05.

**ghx existing surface** (already shipped):
- `backendsUsed []string` in the report schema (ADR-0014.1 sidecar contract)
- Every turn emits tool spans via ADR-0022 D2 (`internal/sidecar/emit.go`)
- `AllowedBackends []string` in `Request` and `## Allowed backends` in
  per-turn prompt — so the agent already names the backend it used
- The prompt constraint: "name the deeper backend needed but do not perform
  it unless allowed" — tier visibility is already baked into the persona

### INFERRED

For Tier 2, the visibility machinery is already correct in design; M7
must only extend it:

1. The tier decision itself (why Tier 2 was chosen — which signals fired) needs
   a new span or attribute, e.g. `ghx.tier.decision` carrying the signal that
   triggered escalation.
2. The clone provenance (which commit SHA, which clone strategy, clone
   duration) should ride as span attributes on the Tier 2 session/turn span.
3. The codemap/stack-graphs invocation becomes a `tool.<kind>` child span
   with `gen_ai.operation.name=execute_tool` and `gen_ai.tool.name=codemap`
   (or `stack-graphs`).
4. ADR-0022 D2 follow-up explicitly names "escalation tiers" as a future
   emitter into the shared stream — the infrastructure contract is already set.

---

## Candidate design constraints for the decision ADR

These are constraints, not decisions. The decision ADR (0024.1) will accept,
reject, or refine each one.

**C1 — Tier 2 is always explicit and visible (tenet)**
Every Tier 2 invocation must appear in: (a) the report's `backendsUsed` field,
(b) a dedicated span with tier decision signals, (c) the `AllowedBackends`
gating — it must never fire if not in `AllowedBackends`. Source: NORTH_STAR
"remote-first, escalation explicit, never silent."

**C2 — codemap is the primary Tier 2 candidate for import-level structure**
codemap ships with: remote repo support, JSON output, deterministic hashing
for cache reuse, MCP mode, and active Go development (releases every few days
as of 2026-07). It fills the cross-file import/dependency gap not covered by
`internal/mapengine`. Decision ADR must choose: `exec` subprocess, MCP
sidecar, or HTTP mode. `exec` is the simplest integration; MCP avoids
re-spawning for multi-command sessions.

**C3 — `internal/mapengine` is NOT replaced; Tier 2 layers on top**
Tier 0/1 already does per-file symbol mapping via mapengine. Tier 2 adds
cross-file structure. Any M7 design that replaces mapengine loses the
remote-evidence path and breaks the eval baseline profile.

**C4 — Clone strategy: blobless + sparse-checkout as default**
For repos where the sidecar needs a local snapshot, blobless (`--filter=blob:none`)
+ sparse-checkout is "particularly effective" per GitHub's own data study.
Treeless is faster on initial clone but creates fetch overhead; avoid for
cached sessions. Cache at `~/.ghx/cache/<owner>/<repo>/<sha>/`.

**C5 — Escalation signals must be pre-registered before the build**
No quantitative escalation threshold is established in published literature
for this specific task type. The M7 ADR must specify: (a) which signals trigger
Tier 2, (b) a starting heuristic (e.g., "zero grep hits after 2 pattern attempts
+ cross-file reference question class"), (c) a SAFE eval gate to measure whether
the heuristic improves correctness without over-triggering (cost).

**C6 — stack-graphs is the precision reference alternative if codemap is insufficient**
For questions requiring precise cross-file definition/reference resolution ("what
implements this interface", "trace this call path exactly"), codemap's import-level
graph may be insufficient. Stack-graphs (GitHub's tool) provides syntactic
cross-file resolution with no build step and SQLite persistence. Decision ADR
should define the question classes that require stack-graphs vs codemap-level
resolution.

**C7 — Tier 2 emit must be best-effort (no answer degradation)**
Per ADR-0022 D2 precedent: emission failures must never fail the ask. Tier 2
artifact writes (clone, index, tool outputs) must follow the same pattern.

**C8 — Private repo Tier 2 requires explicit clone permission**
Clone requires a GitHub token with `contents: read` scope. Sidecar doctor must
check clone permission separately from read permission, especially for private
repos. Tier 2 should refuse silently-authed clones.

**C9 — Deprecation of direct-tool skills is a M7 question**
ADR-0019 explicitly defers direct-tools deprecation to M7. The decision ADR
for M7 must take a position on whether P3's "swallow the tools" removes the
`ghx-mcp` and `ghx` skill files from the recommended path, or whether they
remain as the eval baseline profile.

---

## Open questions

1. **codemap remote clone mechanics**: does `codemap github.com/user/repo`
   use partial-clone filters? Does it cache? What is its latency on a 100MB
   repo? Needs local measurement before M7 build.

2. **codemap as subprocess vs MCP**: subprocess is simpler but re-spawns per
   call; `codemap mcp` avoids re-spawn overhead. Which mode better fits the
   sidecar's turn-by-turn pattern? What is the session lifecycle?

3. **stack-graphs language coverage**: stack-graphs is well-developed for
   TypeScript (GitHub's primary use); Go, Python, Rust coverage is less clear.
   What is the fallback for unsupported languages in a Tier 2 session?

4. **Escalation threshold calibration**: the decision ADR must pre-register
   a Tier 2 trigger heuristic and a SAFE gate that measures: (a) correctness
   improvement when Tier 2 fires on appropriate questions, (b) false-positive
   rate (Tier 2 fires but adds no correctness). No published threshold exists
   for this specific task class — this is original work.

5. **Cache eviction policy**: `~/.ghx/cache/` will accumulate clones. What
   is the eviction strategy? LRU by access time? Fixed size cap? User-visible
   `ghx cache clean` command? ADR-0022 D3 does not address this.

6. **Tier 3 definition**: ADR-0014.1 says "Tier 3: hand off to main agent."
   M7 should sharpen: what does the hand-off artifact look like? A pre-cloned
   local path the main agent can use? A set of pre-run commands to execute?

7. **codemap `--deps` ast-grep dependency**: bundling ast-grep adds a second
   binary dependency for Tier 2. Does the sidecar install it, or fall back
   to codemap without `--deps` (import-level only, no dependency flow)?

---

## Primary sources

| Source | URL | Access date | Used for |
|--------|-----|-------------|---------|
| codemap repo metadata | `gh api repos/JordanCoin/codemap` | 2026-07-05 | Stars, license, language, cadence |
| codemap README | `github.com/JordanCoin/codemap/blob/main/README.md` | 2026-07-05 | CLI surface, install, remote support, JSON output |
| ast-grep repo | `github.com/ast-grep/ast-grep` | 2026-07-05 | Structural search, MCP, agent skill |
| Aider repomap (DeepWiki) | `deepwiki.com/Aider-AI/aider/4.1-repository-mapping` | 2026-07-05 | PageRank, token budget, SQLite cache |
| "Code Isn't Memory" | `arxiv.org/abs/2606.22417` | 2026-07-05 | Structural index vs grep empirical comparison |
| Evidence Tracing paper | `arxiv.org/abs/2606.04990` | 2026-07-05 | Provenance taxonomy for agent tool calls |
| SCIP announcement | `sourcegraph.com/blog/announcing-scip` | 2026-07-05 | SCIP vs LSIF |
| stack-graphs repo | `github.com/github/stack-graphs` | 2026-07-05 | Syntactic cross-file reference, SQLite |
| GitHub clone data study | `github.blog/open-source/git/git-clone-a-data-driven-study-on-cloning-behaviors/` | 2026-07-05 | Partial clone strategies, sizes, times |
| GitHub rate limits docs | `docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api` | 2026-07-05 | Clone vs API rate limit distinction |
| OTel GenAI semconv | `opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/` | 2026-07-05 | Tool span attributes |
| universal-ctags JSON | `docs.ctags.io/en/latest/man/ctags-json-output.5.html` | 2026-07-05 | JSON symbol output format |
| A2RAG paper | `arxiv.org/pdf/2601.21162` | 2026-07-05 | Escalation-on-demand policy |

---

## What could not be verified

- **codemap remote clone mechanics** (partial-clone flags, cache location,
  latency on real repos): the README shows `codemap github.com/user/repo`
  works but does not document internals. Needs local measurement.

- **stack-graphs Go/Python/Rust language maturity**: documentation confirms
  TypeScript support; other language coverage is not documented at the CLI
  level in publicly accessible sources.

- **Quantitative escalation thresholds**: no published paper establishes a
  "grep sparsity threshold → escalate to Tier 2" number for this task type.
  The M7 ADR must define and calibrate this original heuristic.

- **codemap `context` envelope token size in practice**: README describes the
  envelope but does not quantify typical token count. Needs measurement on
  representative repos (e.g., honojs/hono, expressjs/express) before the
  decision ADR can set a budget constraint.

- **A2RAG paper access**: the A2RAG escalation-on-demand result was extracted
  from a search snippet, not full paper review. The specific "stage-wise
  evidence sufficiency checks" detail is paraphrased from the snippet; the
  full paper was not fetched. The finding is plausible and consistent with
  the other evidence but should be verified if cited in the decision ADR.
