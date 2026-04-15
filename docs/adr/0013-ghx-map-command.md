---
title: "First-Class Code Mapping in ghx"
date: 2026-04-07
status: Accepted — Phase 1 and Phase 2 complete as of 2026-04-14
---

# 0013. First-Class Code Mapping in `ghx`

## Context

### The capability

AI agents and developers need a cheap way to understand code structure before deciding what to read. They need function signatures, type definitions, imports, package declarations, and symbol relationships without spending tokens on full implementation bodies.

This is the exact problem `ghx read --map` already solves. The feature exists, but the implementation and product surface are not yet strong enough for ghx to own "remote code mapping" as a first-class capability.

Search Console is evidence of demand, not the product rationale. Developers searching for "codemap symbol: qualifier" are not asking for an origin story; they are looking for symbol-level code mapping documentation and tooling. ghx should answer that intent directly because it matches the core product: GitHub code exploration for agents without cloning.

The product question is no longer "should ghx build mapping?" ghx already has it. The question is how to make mapping a dedicated, reliable feature without making the agent interface harder to use.

### Current state

ghx's `--map` flag works as follows:

```bash
ghx read gkoreli/ghx --map v2/pkg/ghx/explore.go
# Returns: package declarations, imports, type definitions, function signatures
# Output: 202 characters instead of 3,111 bytes (92% token reduction)
```

This works. Agents that load `ghx skill` already know about it and use it effectively. But:
1. It is regex-based and can miss important declarations
2. There's no progressive depth control (codemap has 5 levels; ghx has one)
3. There's no symbol-kind filtering (`--kind func`, `--kind type`, etc.)
4. Output is a flat list, not a structured map optimized for deciding what to read next
5. The implementation lives inside `read.go` instead of a dedicated mapping engine

### Findings

#### GitHub public APIs do not expose modern code search

The earlier assumption that GitHub GraphQL can perform code search was wrong.

GitHub GraphQL has a `search` field, but the live `SearchType` enum does not include `CODE`. It supports repositories, issues, users, and discussions, not code blobs. This was verified against the live schema with:

```bash
gh api graphql -f query='{ __type(name: "SearchType") { enumValues { name description } } }'
```

Returned values:

```
ISSUE
ISSUE_ADVANCED
ISSUE_SEMANTIC
ISSUE_HYBRID
REPOSITORY
USER
DISCUSSION
```

GitHub's REST `/search/code` endpoint still exists, but it uses the legacy code search engine. The GitHub CLI manual for `gh search code` explicitly says results may not match github.com and that newer features like regex search are not available via the API.

Implication: `ghx search --symbol` is not a cheap Phase 0 feature. There is no stable public GraphQL code-search path to wrap.

#### GitHub.com has an internal modern code-search JSON route

The GitHub web UI fetches modern code-search results from `https://github.com/search?q=...&type=code` when called as an authenticated browser request with web-client headers.

That response includes the data ghx would want:

- `matched_symbols`
- `SYMBOL_KIND_TYPE_DEF`
- `SYMBOL_KIND_CLASS_DEF`
- `SYMBOL_KIND_FUNCTION_DEF`
- ranked snippets
- facets
- `query_id`
- timing and ranking metadata

This proves the modern GitHub.com code-search product has machine-readable symbol metadata.

But this route is not a public API:

- It is served from `github.com`, not `api.github.com`
- It depends on browser session cookies and web-client headers
- It uses React Router JSON transport headers such as `x-react-router: json`
- It requires a fetch nonce and other session-specific state
- It has no documented stability contract

Decision: do not build core ghx behavior on GitHub.com internal web routes. Treat this only as research evidence that the capability exists in the web product.

#### Parser-backed mapping is the durable ghx path

Regex extraction is acceptable as a fallback, but not as the long-term implementation for a serious code-mapping feature. Regex misses multi-line signatures, class methods, decorators, nested symbols, exported arrow functions, receiver methods, doc comments, and other structural information agents use when deciding what to read.

The durable path is:

1. Fetch remote files through documented GitHub APIs
2. Parse them locally in ghx
3. Render compact structural maps
4. Fall back gracefully when parsing is unavailable

This keeps ghx API-native and avoids depending on unsupported GitHub.com internals.

### Parser library research

The research target was a battle-tested Go-compatible library that can extract symbols without breaking ghx's release model. ghx currently cross-compiles static binaries for Linux, macOS, and Windows with `CGO_ENABLED=0`.

| Library | Finding | Fit for ghx |
|---------|---------|-------------|
| Go standard library `go/parser`, `go/ast` | Battle-tested, pure Go, excellent for Go files | Strong for Go-only mapper |
| `tree-sitter/go-tree-sitter` | Official Go Tree-sitter bindings | High quality, but CGo-based |
| `smacker/go-tree-sitter` | Mature bindings with many grammars | High quality, but CGo-based |
| `malivvan/tree-sitter` | CGo-free Tree-sitter wrapper using WASM + `wazero`; README matches ghx constraints | Not usable yet for ghx target languages in v0.0.1: shipped WASM exports only `tree_sitter_c` and `tree_sitter_cpp` |
| `odvcencio/gotreesitter` | Pure-Go Tree-sitter runtime, no CGo, bundled grammar blobs, query engine, tagger API | Accepted for first implementation; supports static `CGO_ENABLED=0` builds and tags queries |
| `sourcegraph/go-ctags` | Universal Ctags wrapper | Requires external `universal-ctags`; README says Sourcegraph-only |
| `alecthomas/chroma` | Pure Go syntax highlighter | Useful lexer, not structural parser |
| `go-enry/go-enry` | Language detection and vendor/generated/test/binary filtering | Useful adjunct, not symbol extraction |

The strongest finding: official Tree-sitter grammars already ship `queries/tags.scm` files that encode symbol captures:

- `@definition.function`
- `@definition.method`
- `@definition.class`
- `@definition.interface`
- `@definition.type`
- `@definition.constant`
- `@reference.call`
- `@name`
- `@doc`

This means ghx does not need to invent symbol extraction rules per language if it can run Tree-sitter queries. It can reuse upstream grammar `tags.scm` queries and normalize captures into a ghx `MapSymbol` model.

### Insights

- The strongest ghx feature is not global code search. GitHub's public API limits make that a platform gap.
- The strongest ghx feature is remote structural orientation: "show me what this repo contains before I spend tokens reading it."
- `ghx read --map` already proves the workflow. The next step is quality, not a brand-new concept.
- A top-level `ghx map` can be useful for discoverability, but it must be a thin wrapper over the same mapping engine. `ghx read --map` remains valid and should not be deprecated.
- `--symbol` is the wrong flag name for filtering by function/type/import categories. Use `--kind` for symbol kinds. Reserve `--symbol` for symbol names if ghx later supports symbol-name lookup.
- Regex should become the fallback engine, not the main value proposition.
- Tree-sitter tags queries are the best path to serious multi-language mapping.
- CGo-free Tree-sitter is viable, but the implementation choice matters. `malivvan/tree-sitter` preserves the release model but does not currently expose the needed grammars. `gotreesitter` exposes the needed grammars now, at the cost of a larger binary.
- The next optimization frontier is grammar payload control: ghx should not permanently ship every bundled grammar if the map engine only commits to Go, TypeScript/TSX, JavaScript/JSX, and Python first.

### The competitive gap

| Tool | Structural extraction | Symbol search | Requires clone | API-native |
|------|--------------------|--------------|---------------|-----------|
| codemap | Yes (5 named levels) | Yes (find-refs, call-graph) | Yes | No |
| Aider repomap | Yes (Tree-sitter + PageRank) | No | Yes | No |
| Gitingest | No (full dump) | No | Yes | No |
| Repomix | No (full dump) | No | Yes | No |
| GitHub `symbol:` REST | No | Legacy / not modern code search | No | Yes (limited) |
| GitHub `symbol:` GraphQL | No | **No code search type** | No | No |
| GitHub.com internal search JSON | No | **Yes, with symbol metadata** | No | No (unsupported) |
| ghx `--map` (current) | Yes (1 level) | No | **No** | **Yes** |
| **ghx map/read --map (next)** | **Yes (parser-backed levels)** | No | **No** | **Yes** |
| **ghx map/read --map --kind** | **Yes** | Symbol-kind filtering | **No** | **Yes** |

Key insight: GitHub.com has modern symbol-aware code search, but the public APIs do not expose that modern engine as a stable code-search API. ghx should not wait for GitHub to expose Blackbird. ghx should build the reliable agent-facing subset itself: parser-backed structural maps over files fetched via documented repository/blob APIs.

ghx has one reliable path to compete here:

1. **Structural extraction**: fetch files through GraphQL and map them locally with parsers
2. **Repo-scoped search later**: tree → filter candidate files → batch-read blobs → local regex/parser matching
3. **Global symbol search**: blocked until GitHub exposes a supported public API

## Decision

### Phase 0: Correct the search story

Do not ship `ghx search --symbol` as a GraphQL feature.

`ghx search` currently uses REST `/search/code`, because public GraphQL has no code-search type. Keep `ghx search` as a legacy-API code search with matching context, and document the limitation clearly.

If users pass web-only code-search qualifiers such as `symbol:`, regex `/.../`, `OR`, or `NOT`, ghx should warn that GitHub's public code search API is legacy and may not behave like github.com.

### Phase 1: Dedicated mapping engine

Refactor `--map` out of `read.go` into a dedicated mapping package.

`ghx read --map` remains supported. A top-level `ghx map` command may be added for human discoverability, but it must be a thin wrapper over the same engine.

```bash
# Existing agent-friendly workflow
ghx read owner/repo --map path/to/file.go

# Optional discoverable entry point using the same engine
ghx map owner/repo path/to/file.go

# Progressive depth levels (default: compact)
ghx read owner/repo --map --level minimal file.go
ghx read owner/repo --map --level compact file.go
ghx read owner/repo --map --level standard file.go

# Symbol-kind filtering
ghx read owner/repo --map --kind func file.go
ghx read owner/repo --map --kind type file.go
ghx read owner/repo --map --kind import file.go

# Batch/glob mapping
ghx read owner/repo --map --level minimal "src/**/*.ts"
```

**Level definitions** (named to match codemap's vocabulary):

| Level name | Content included |
|-----------|-----------------|
| `outline` | File path + line range markers only |
| `minimal` | Symbol names only — no signatures, no types |
| `compact` | Full signatures, no comments ← **default** |
| `standard` | Full signatures + doc comments (truncated to 160 chars) |
| `full` | Not needed as map level; use plain `ghx read` |

Naming matches codemap's named levels deliberately — agents and developers familiar with codemap's model will understand ghx's levels without a lookup.

**Symbol kinds for `--kind`:**

| Kind | Captured |
|--------|----------|
| `func` | Function/method signatures |
| `type` | Struct, interface, class, type alias definitions |
| `import` | Import/require statements |
| `const` | Constants |
| `var` | Package-level variables |

### Phase 2: Parser-backed extraction ✓ Done

The common model, as shipped:

```go
type Mapper interface {
    Map(path string, content []byte, opts Options) (Result, error)
}

type Symbol struct {
    Kind      Kind
    Name      string
    Signature string
    Line      int
    EndLine   int
    Parent    string
    Doc       string
}
```

Engines shipped:

1. `GoASTMapper`: Go stdlib `go/ast` + `go/parser` for `.go` files. Top-level declarations only, full multi-line signatures compacted to one line, generic type parameters preserved. Zero binary size cost — stdlib only.
2. `TreeSitterMapper`: pure-Go Tree-sitter tags through `odvcencio/gotreesitter` for TypeScript, TSX, JavaScript, JSX, Python. Captures class methods and nested symbols that regex cannot reach.
3. `RegexMapper`: fallback for all unsupported languages and safety net when parsers fail.

Key implementation decision made during development: **Go uses `GoASTMapper`, not `TreeSitterMapper`**. An earlier iteration routed Go through tree-sitter and regressed quality — tree-sitter's Go `tags.scm` captures local variable definitions inside function bodies, producing noise and duplicates. `go/ast` has explicit top-level/local scope distinction and is the correct tool. Tree-sitter is right for languages without a Go stdlib parser.

`malivvan/tree-sitter` remains a future option only if it exposes the target grammars or ghx vendors a custom WASM build.

### Engineering plan ✓ Done

All items shipped as of 2026-04-14:

1. ✓ `internal/mapengine` owns `Mapper`, `Options`, `Result`, `Symbol`, engine selection, fallback, and output formatting.
2. ✓ `pkg/ghx/read.go` calls `mapengine.Map(...)` and has no parser logic.
3. ✓ `RegexMapper` is the fallback, not the default for supported languages.
4. ✓ `GoASTMapper` is the primary engine for `.go` files. `TreeSitterMapper` handles TS/JS/Python. Both sit behind the same `Mapper` interface.
5. ✓ Tree-sitter language behavior is configurable inside `TreeSitterMapper` via `TreeSitterLanguageConfig`.
6. ✓ Engine selection is resilient: `auto` routes by file extension, `regex` forces regex, `tree-sitter` forces tree-sitter with fallback warning.
7. ✓ All engine output normalizes to the same `Result`/`Symbol` shape.
8. ✓ `Result` carries engine, fallback flag, warnings, original chars, mapped chars.
9. Tests are engine-agnostic where possible. Quality fixtures and benchmarks are future work.

Actual package shape as shipped:

```text
v2/internal/mapengine/
  types.go            # Mapper, Options, Result, Symbol, enums, engine routing
  regex.go            # regex fallback implementation
  goast.go            # Go stdlib go/ast engine for .go files
  treesitter.go       # pure-Go Tree-sitter engine + TreeSitterLanguageConfig
  regex_test.go       # 15 tests covering all three engines and routing
  fixtures/           # map-quality fixtures per language (not yet written)
  benchmarks_test.go  # parse/render performance tests (not yet written)
```

Engine routing in `EngineAuto`:

```
.go              → GoASTMapper    fallback → RegexMapper
.ts .tsx .js .jsx .py → TreeSitterMapper   fallback → RegexMapper
everything else  → RegexMapper
```

`EngineTreeSitter` flag forces `TreeSitterMapper` for any file, with a warning on fallback. `EngineRegex` forces `RegexMapper`.

### `TreeSitterLanguageConfig` — per-language hook points

Each language entry in `TreeSitterMapper.Languages` carries a `TreeSitterLanguageConfig`. Nil fields fall back to engine-wide defaults so a minimal config only overrides what the language actually needs:

```go
type TreeSitterLanguageConfig struct {
    TagsQuery       func(grammars.LangEntry) string          // nil → grammars.ResolveTagsQuery
    TagKinds        map[string]Kind                           // nil → defaultTreeSitterTagKinds()
    Signature       func(string, []byte, gotreesitter.Range) string // nil → brace/newline trimmer
    MergeRegex      bool                                      // merge RegexMapper output after Tree-sitter pass
    MergeRegexKinds map[Kind]bool                             // nil → accept all regex kinds
    TokenSource     func([]byte, *gotreesitter.Language) gotreesitter.TokenSource // nil → entry default
}
```

Default tag-kind mapping shared by all languages unless overridden:

| Tree-sitter capture | ghx Kind |
|---------------------|----------|
| `definition.function` | `func` |
| `definition.method` | `func` |
| `definition.constructor` | `func` |
| `definition.class` | `type` |
| `definition.interface` | `type` |
| `definition.type` | `type` |
| `definition.constant` | `const` |
| `definition.variable` | `var` |

Default language configs shipped in `defaultTreeSitterLanguageConfigs()`:

| Language | `MergeRegex` | `MergeRegexKinds` | Custom signature |
|----------|-------------|-------------------|------------------|
| `go` | true | package, import, func, type, const, var | — |
| `typescript` | true | all | — |
| `tsx` | true | all | — |
| `javascript` | true | all | — |
| `python` | true | all | colon-stop trimmer |

The colon-stop trimmer for Python captures `def f(x: int) -> str:` without including the body, which the brace-stop default cannot do.

The first parser-backed implementation targets four language families:

- Go
- TypeScript/TSX
- JavaScript/JSX
- Python

Acceptance criteria and current result:

- ghx still builds with `CGO_ENABLED=0` — verified locally with `CGO_ENABLED=0 go build -o ghx .`
- tests pass with `CGO_ENABLED=0 go test ./...`
- unsupported parser failures return regex maps instead of failing `read --map`
- map quality improves immediately for nested/class methods that regex misses
- binary size increased from approximately 27M to 48M in the local build after importing bundled grammars
- GoReleaser matrix still needs CI confirmation across darwin/linux/windows and amd64/arm64
- cold parse time and per-file parse time still need benchmarks
- grammar payload pruning is required before treating the size increase as final

### Phase 3: Repo-scoped scan/map

After mapping is parser-backed, consider repo-scoped search over documented APIs:

```
tree -> filter source files -> batch GraphQL blob reads -> parser/regex match -> snippets/map output
```

This does not replace global GitHub Code Search, but it gives agents a reliable modern search/mapping workflow inside a known repo without cloning.

## Implementation Notes

### Mapping implementation

Mapping is a file content post-processing step, not a separate GitHub API. ghx fetches file content via GraphQL, then maps structure locally.

Language support as shipped:

| Language | Engine | Notes |
|----------|--------|-------|
| Go | `GoASTMapper` (`go/ast`) | Top-level only, full multi-line sigs, generics preserved |
| TypeScript / TSX | `TreeSitterMapper` | Class methods, arrow funcs, interfaces |
| JavaScript / JSX | `TreeSitterMapper` | Class methods, exported functions |
| Python | `TreeSitterMapper` | Classes, methods, colon-stop signature trimmer |
| Rust, Java, Ruby, etc. | `RegexMapper` | Top-level patterns only |
| Markdown, YAML, etc. | `RegexMapper` | Structural hints only |

Tree-sitter beats regex for any language with nested scope (class bodies, decorators, methods). Go is the exception: `go/ast` is unambiguously better because it has explicit scope, handles multi-line signatures, preserves generic type parameters, and adds zero binary size.

**Verified quality difference (GoAST vs regex on `net/http/server.go`):**
- Regex includes function bodies on single-line functions (`func unlock() { cr.mu.Unlock() }`) — GoAST stops at `{`
- Regex truncates multi-line `var`/`const` literals at the first line — GoAST compacts the full value
- Regex includes inline comments on constants — GoAST strips them
- Both produce equivalent symbol counts; GoAST output is semantically more precise

**Verified quality difference (TreeSitter vs regex on TypeScript class bodies):**
- Regex finds zero methods inside a class body — TreeSitter captures all of them
- `TestTreeSitterBetterThanRegexForClassMethods` encodes this as a permanent regression guard

### `--kind` filter

Applied after level extraction. Example:

```
Compact output (all structural elements):
  package ghx
  import (...)
  type FileEntry struct
  type ExploreResult struct
  func Explore(repo string, path string) (*ExploreResult, error)

Compact + --kind func:
  func Explore(repo string, path string) (*ExploreResult, error)
```

### Batch and repo-wide map

Batch/glob mapping should continue to work through `read`:

```bash
ghx read owner/repo --map "src/**/*.ts"
```

A repo-wide map can be considered later, but must be bounded:

1. Fetch the repo file tree (already available via `ghx explore`)
2. Filter to non-test/non-generated/non-vendor source files by default
3. Batch-fetch file contents via GraphQL aliases (same batching strategy as `ghx read`)
4. Apply level extraction to each file
5. Return concatenated output with file headers

Token budget concern: a large repo at minimal level can still be enormous. Require or default a `--max-files N` cap.

### Backward compatibility

`ghx read --map` continues to work unchanged. It should not be deprecated because it is the agent-friendly workflow mode.

If `ghx map` is added, it is a discoverability wrapper over the same implementation, not a replacement.

## Alternatives considered

### Only improve the `--map` flag

Add `--level` and `--kind` directly to `ghx read --map`. This avoids adding a new command.

Partially accepted:
- `read --map` remains the canonical agent workflow
- `ghx map` may still be added for human discoverability
- Both must share the same implementation

### Delegate to codemap (local) and use own extraction (remote)

Have `ghx map` detect if codemap is installed and delegate to it for local paths.

Held for Phase 2. The complication: codemap's output format differs from ghx's. Normalization adds complexity. Start with ghx's own extraction for both local and remote, validate the format is right, then add codemap delegation as an optimization.

### Use GitHub's `symbol:` qualifier

Rejected for now. Public GraphQL has no code-search `SearchType`, and REST `/search/code` is legacy. GitHub.com has an internal web JSON route with symbol metadata, but it is not a public API.

Do not build core ghx features on unsupported web routes.

Symbol search and structural mapping remain different capabilities:
- Symbol search finds a known name
- Mapping summarizes what a file/package contains

ghx can reliably own mapping today.

### Use Tree-sitter via CGo

Tree-sitter is the right class of technology, but common Go bindings use CGo. ghx currently releases static binaries with `CGO_ENABLED=0` for Linux, macOS, and Windows across amd64/arm64. Introducing CGo would complicate npm and Homebrew distribution.

Held unless the quality gain justifies the release complexity.

### Use cgo-free Tree-sitter via WASM/wazero

Partially rejected for now. `malivvan/tree-sitter` matches ghx's CGo-free release constraint, but v0.0.1 only exports C and C++ from the shipped WASM module. It does not currently support the first ghx target languages.

Keep it on the watch list, but do not build ghx map around it yet.

### Use pure-Go Tree-sitter through gotreesitter

Accepted for the first parser-backed engine. `gotreesitter` is CGo-free and exposes the target grammars plus a tags API. It preserves `CGO_ENABLED=0` builds.

Tradeoff: importing the bundled grammar package increases binary size substantially. This is acceptable for the spike but must be tightened before declaring the map engine finished.

## Consequences

**Positive:**
- Mapping captures the "code mapping" and "symbol qualifier" search intent directly
- Positions ghx as codemap's API-native complement, not just another "it's like X but different" tool
- Gives agents a progressive disclosure API: ask for less, get what you need, escalate only when required
- Keeps remote GitHub exploration clone-free
- Builds on documented GitHub APIs instead of unsupported web internals

**Negative:**
- Parser-backed mapping adds implementation complexity
- Tree-sitter integration increased the local binary size from approximately 27M to 48M
- `gotreesitter` is promising but still needs ghx-specific benchmark and corpus validation
- Repo-wide map could be expensive for very large repos — needs rate limit awareness and batching caps
- Two entry points may overlap if `ghx map` is added (`ghx read --map` and `ghx map`)

## What's next

### Immediate (Phase 1 remainder)

- **`ghx map` top-level command**: Phase 1 specified a thin wrapper over `mapengine.Map`. `read --map` works; `ghx map` as a discoverable entry point has not been added yet.
- **`--level standard`**: The `standard` level (signatures + doc comments truncated to 160 chars) is defined but `GoASTMapper` and `TreeSitterMapper` do not yet populate `Symbol.Doc`. Implementing this requires reading `ast.CommentGroup` for Go and `@doc` captures for Tree-sitter.
- **GoReleaser CI confirmation**: `gotreesitter` builds pass locally (`CGO_ENABLED=0`). Cross-compilation across darwin/linux/windows × amd64/arm64 needs CI validation before declaring the map engine stable.

### Near-term

- **Grammar payload pruning**: `gotreesitter` bundles all supported grammars. ghx only needs Go, TypeScript, TSX, JavaScript, Python. Pruning or lazy-loading the bundle is required before treating the ~48M binary size as final.
- **Benchmarks**: Cold parse time and per-file parse time for `GoASTMapper` and `TreeSitterMapper` under realistic load have not been measured. Add `benchmarks_test.go` with a representative corpus before enabling in CI.
- **Python quality validation**: `TreeSitterMapper` with the colon-stop trimmer handles Python correctly in tests, but real-world Python (decorators, `@dataclass`, `async def`, type annotations) needs a wider fixture set.

### Phase 3: Repo-scoped map

After the map engine is stable, build the repo-scoped scan workflow:

```
ghx explore → file tree
  → filter: source files only (no test/, vendor/, generated)
  → batch GraphQL blob reads (same strategy as ghx read)
  → mapengine.Map per file
  → concatenated output with file headers
```

This is not global code search — it is bounded to a known repo and uses documented APIs. It gives agents a reliable "show me the whole codebase structure" primitive without cloning. Must be gated by a `--max-files N` cap (default TBD, likely 50–100 files).

### Later

- **`go-enry` integration**: Language detection and generated/vendor/test/binary filtering. Currently ghx uses file extension only. `go-enry` would let ghx skip minified JS, vendored deps, and generated protobuf files automatically.
- **Nested symbol representation**: `Symbol.Parent` is defined but not populated. Filling it would let agents understand that `render` is a method of `UserService`, not a top-level function.
- **Rust support via TreeSitter**: `RegexMapper` handles Rust today. `gotreesitter` has a Rust grammar. Adding a `"rust"` entry to `defaultTreeSitterLanguageConfigs()` is the only change needed.

## Open questions

1. Does `gotreesitter` build reliably in ghx's GoReleaser matrix? _(CI confirmation still needed across darwin/linux/windows × amd64/arm64)_
2. Can ghx prune or externalize grammar blobs so the binary does not carry every bundled grammar?
3. Should `ghx map` be added as a top-level command, or should `read --map` remain the only command surface?
4. What's the right default for `--max-files` in repo-wide mode?
5. Should ghx use `go-enry` for language detection and generated/vendor/test filtering?
6. How should map output represent nested symbols and parent relationships?
7. ~~Should parser-backed map be opt-in initially?~~ **Resolved**: Tree-sitter is default for TS/JS/Python; GoAST is default for Go; regex is the fallback. No opt-in needed.
8. What are cold-parse and per-file parse times under realistic load? _(benchmarks not yet written)_
9. ~~Should Go use tree-sitter or go/ast?~~ **Resolved**: `go/ast` — tree-sitter's Go tags.scm captures local variable definitions inside function bodies, producing noise and duplicates. `go/ast` has explicit scope and is strictly better for Go.

## References

| Item | Source |
|------|--------|
| codemap named levels + symbol format | [kcosr/codemap](https://github.com/kcosr/codemap) |
| GitHub `symbol:` qualifier docs | [GitHub code search syntax](https://docs.github.com/en/search-github/github-code-search/understanding-github-code-search-syntax) |
| GitHub code search about (Tree-sitter languages) | [About GitHub code search](https://docs.github.com/en/search-github/github-code-search/about-github-code-search) |
| C/C++ symbol: disabled discussion | [GitHub community](https://github.com/orgs/community/discussions/8594) |
| Public GraphQL search has no CODE type | [GitHub GraphQL SearchType enum](https://docs.github.com/en/graphql/reference/enums#searchtype) |
| GraphQL `search` field | [GitHub GraphQL queries](https://docs.github.com/en/graphql/reference/queries#search) |
| REST code search is legacy | [GitHub legacy code search docs](https://docs.github.com/en/search-github/searching-on-github/searching-code) |
| `gh search code` uses legacy engine | [GitHub CLI manual](https://cli.github.com/manual/gh_search_code) |
| New code search public beta | [GitHub changelog, 2023-02-23](https://github.blog/changelog/2023-02-23-no-more-waitlist-code-search-and-code-view-are-available-to-all-in-public-beta/) |
| Code search API changes | [GitHub changelog, 2023-03-10](https://github.blog/changelog/2023-03-10-changes-to-the-code-search-api/) |
| GitHub staff: new API not ready | [GitHub community discussion 54546](https://github.com/orgs/community/discussions/54546) |
| REST code search lacks regex support | [GitHub community discussion 112338](https://github.com/orgs/community/discussions/112338) |
| Tree-sitter Go bindings | [tree-sitter/go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter) |
| Go Tree-sitter bindings with bundled grammars | [smacker/go-tree-sitter](https://github.com/smacker/go-tree-sitter) |
| CGo-free Tree-sitter via wazero | [malivvan/tree-sitter](https://github.com/malivvan/tree-sitter) |
| Pure-Go Tree-sitter runtime | [odvcencio/gotreesitter](https://github.com/odvcencio/gotreesitter) |
| Universal Ctags wrapper | [sourcegraph/go-ctags](https://github.com/sourcegraph/go-ctags) |
| Language detection/filtering | [go-enry/go-enry](https://github.com/go-enry/go-enry) |
| Pure Go syntax highlighter | [alecthomas/chroma](https://github.com/alecthomas/chroma) |
| `symbol:` REST/API limitations | ghx ADR-0003 |
| Aider repomap (Tree-sitter + PageRank) | [aider.chat/docs/repomap](https://aider.chat/docs/repomap.html) |
| ghx current implementation | [gkoreli/ghx](https://github.com/gkoreli/ghx) |
| Article: You Don't Always Need Codemap | `packages/blog/posts/006-you-dont-need-codemap.md` |
