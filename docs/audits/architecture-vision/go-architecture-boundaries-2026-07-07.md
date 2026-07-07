---
title: "Go Architecture & Boundaries — Target Design (idiomatic-Go package architecture)"
date: "2026-07-07"
status: "audit"
author: "Go-architecture audit worker (idiomatic-Go / package-oriented lens)"
scope: "internal/**, cmd/** — read-only design audit, no code changes"
lens: "long-term package/module boundaries; 'accept interfaces, return structs'; composition root; singletons vs services"
---

# Go Architecture & Boundaries — Target Design — 2026-07-07

A read-only, high-altitude audit of ghx's **package/module boundaries** through
an idiomatic-Go, package-oriented lens (Ben Johnson's *Standard Package Layout*;
Kat Zien's *How Do You Structure Your Go Apps*; Rob Pike's Go Proverbs; the
standard library as the taste reference). It answers Goga's four questions with
`file:line` evidence **and** idiomatic-Go reasoning, and proposes a concrete
target layout ghx can *grow into*, not rewrite toward.

This audit **builds on and does not repeat** the five 2026-07-07 audits or
ADR-0035. Those rank *mechanical* refactors (repo value object, client seam,
turn-engine consolidation, tool registry, harness seam, presentation
ownership). This one asks the layer above them: **given those fixes, what are
the right package boundaries, and does ghx use singletons and services the way
idiomatic Go wants?**

Verified against **current mainline (HEAD `1bda884`, the integrated shared
checkout)**, which is *ahead* of the five audits' worktrees: ADR-0035 **Tier-1
is already landed** on mainline (Repo value object `9e4a10e`, GitHub-client seam
`b972c4b`, turn-primitive extraction `26ae9ef`), so several audit findings are
now *shapes to evaluate*, not gaps to open. (This audit's own worktree is
branched from the older `d2cda8a`/v2.7.0; the doc cites and describes **current
mainline**, which is the tree it merges into — every citation below is checkable
against `1bda884`.)

## Executive summary

**ghx's package *shape* is already idiomatic — domain-oriented, not
layer-oriented — and that is the single most important thing it gets right.**
The real debt is one altitude up from the five mechanical audits: ghx has **no
single composition root.** Dependencies bootstrap themselves as ambient package
state instead of being constructed once in `main` and passed down. The GitHub
client is a mutable package global (`internal/ghx/client.go:33`), the storage
root re-reads the environment on every path call (`internal/sidecar/config.go:17`),
config is re-loaded from disk in 10+ separate command bodies
(`internal/cli/sidecar.go:90,144,146,240,290,340,368,430,466,518`), and three
test seams are package-global *function variables* swapped by assignment
(`runtime.go:15-16`, `serve.go:166`). Every one of these is the same
un-idiomatic move — *reach for the world at call time* — and it is the direct
cause of the client-global, the `rootDir()` finding (AUD3 M1), and the serial
`t.Setenv` tests. **Ben Johnson's fourth principle — "the main package ties
together dependencies" — is unmet.** Fixing it is cheap, behavior-preserving,
and is exactly what makes the P3 tool registry and P4 harness *injectable
values* rather than new globals.

**Direct answer to Goga's question — "should we use singleton patterns and
services?"** Services: **yes** — but "service" in idiomatic Go means *a struct
that holds its dependencies and exposes methods, constructed once at the
composition root and passed down*, **not** a DI-framework bean, a service
locator, or a registry of globals. ghx already does this well
(`tier2.NewService`, `telemetry.NewTracerProvider`, the injected `TurnRunner`).
Singletons — **mostly no.** A *stateless, documented, replaceable default* is
fine (the stdlib blesses exactly one shape: `http.DefaultClient`). A *mutable
package global swapped in tests* is not a design choice; it is the tell that you
actually wanted injection. `githubClients` is the exemplar: **the right
interface (`graphQLDoer`/`restGetter` are small and consumer-defined — textbook
Go), wrapped in the wrong container (a mutable global).** The fix is never "add
a DI container" (the un-Go over-correction) — it is "construct it once in the
composition root and pass it explicitly." That is Go's entire answer to
dependency injection: function parameters and struct fields.

The three headline findings, ranked by architectural leverage:

1. **No composition root (H1).** Ambient globals/env instead of `main`-wired
   dependencies. The idiomatic root cause behind several audit findings.
2. **The domain core imports the execution frontend (H2).** `internal/ghx`
   imports `internal/codemode` (`register.go:3`) — a domain package depending on
   the executor it should know nothing about. The one genuine
   dependency-direction smell, and one the earlier architecture audit missed.
3. **`internal/sidecar` is a flat 29-file god-package (M1).** A cohesion smell —
   but full subpackaging is cycle-risky and partly premature; extract *only* the
   ACP harness now (the P4 enabler), nothing else.

Everything else in the layout is right and should be left alone. This is a
solo-dev product; every recommendation below is sized as *cheap-now,
behavior-preserving, extend-an-existing-pattern* — no speculative frameworks,
no touching the frozen measurement stack.

## What is already right (verified, not manufactured)

- **The layout is domain-oriented, which is the idiomatic choice — not
  layered.** Packages are named for *what they are about* (`ghx` = code recon,
  `sidecar` = the runtime, `codemode` = JS execution, `mapengine` = structural
  maps, `tier2` = absorbed tools, `telemetry` = OTel), never for a *role*
  (`services/`, `models/`, `controllers/`, `dto/`, `utils/`). This is exactly
  Kat Zien's recommended "group by domain/context" and Ben Johnson's rejection
  of role-layer packages. **The answer to "should there be explicit
  core/shared/domain layers?" is no — that would be the un-Go move, and ghx
  already avoids it.** Do not add layer packages.

- **The domain core is (almost) a leaf, and the sidecar↔core boundary is a
  *process* boundary, not an import.** `go list` confirms `internal/sidecar`
  imports only `internal/sidecar/telemetry` and `internal/sidecar/tier2` — it
  does **not** import `internal/ghx`. The sidecar reaches recon by shelling out
  to the `ghx` binary over ACP (verified in the architecture audit). So the much
  bigger question — "where do shared domain types live so both frontends and the
  sidecar share them without a dependency-direction violation?" — has a clean
  present-tense answer: **there is no cross-package shared-type problem today,
  because the two domains never share a Go process.** ADR-0010's "core lives in
  `internal/ghx`" is correct and *sufficient*; a speculative `internal/domain`
  shared layer would be solving a problem ghx does not have (see Posture).

- **The interface seams are small and consumer-defined — the good half of Go
  interface taste.** `graphQLDoer` (`internal/ghx/client.go:5-7`) and
  `restGetter` (`client.go:9-11`) are one- and one-method interfaces *defined by
  the consumer that needs them* — a direct instantiation of "the bigger the
  interface, the weaker the abstraction." The same holds for the seams the other
  audits already praised (`TurnRunner`, `JudgeClient`, `GitRunner`,
  `ContainerRuntime`, `WritePolicy`, `mapengine.Mapper`, `RouteSource`). This is
  **not** a missing-interface codebase.

- **Capability packages are grouped by dependency — Ben Johnson's principle 2.**
  `codemode` (goja), `mapengine` (tree-sitter/regex), `telemetry` (OTel SDK), and
  `tier2` (subprocess tools) are adapter subpackages that isolate a dependency
  behind ghx's own types. `codemode` is a true leaf (imports no internal
  package); `telemetry` is a leaf; `tier2` depends only on `mapengine`. Clean.

- **Real services are already constructed-and-injected.** `tier2.NewService(ghxRoot, version)`
  (`internal/sidecar/tier2/service.go:202`) and
  `telemetry.NewTracerProvider(ctx, dir, attrs)`
  (`internal/sidecar/telemetry/provider.go:22`) take their dependencies as
  arguments and return a struct. This is precisely the AGENTS.md "encapsulate
  domain logic in services … construct it once and inject/share it" tenet done
  correctly — proof the codebase knows the idiomatic pattern.

- **ADR-0035 Tier-1 landed in idiomatic shape.** The `Repo` value object
  (`internal/ghx/repo.go`: `ParseRepo` + `Owner`/`Name`/`String()`) is a proper
  value type replacing primitive obsession; the client seam introduced *small
  consumer-defined interfaces*; the turn-primitive extraction (`26ae9ef`)
  decomposed a god-function. The *moves* are right — this audit only critiques
  the *container* one of them chose (H1/the singleton question).

---

## High severity (design)

### H1 — No composition root: dependencies self-initialize as ambient package state

**What.** ghx has no single place that constructs its dependencies and hands
them down. `cmd/ghx/main.go` wires three skill strings and calls
`cli.RootCmd.Execute()` — that is the entire entrypoint; there is no
`Build()`/`wire()` step. As a result every dependency bootstraps itself
ambiently, at call time, from package globals or the environment.

**Evidence.**
- **GitHub client = mutable package global.** `internal/ghx/client.go:33` —
  `var githubClients githubClientProvider = defaultGithubClientProvider{}`. It is
  never passed in; `Explore`/`Read`/`Repos`/`Tree`/`Search` reach out to it by
  reference (`explore.go:29`, `read.go:92`, `repos.go:33`, `glob.go:18,46`,
  `search.go:40`). Tests mutate the global and restore it:
  `internal/ghx/client_test.go:79-81` — `oldClients := githubClients;
  githubClients = fake…; t.Cleanup(func(){ githubClients = oldClients })`.
- **Storage root = env read on every call.** `internal/sidecar/config.go:17`
  `rootDir()` re-reads `os.Getenv("GHX_HOME")`/`HOME` each time; `RootDir()`,
  `RuntimeDir()`, `ConfigFilePath()`, `NewDefaultConfig()`, `LoadConfig()`,
  `SaveConfig()` all call it fresh (`config.go:170,174,177,183,188,275`). This is
  AUD3 M1; named here as an *instance of the missing composition root*, not a
  separate problem.
- **Config re-loaded ambiently per command.** `sidecar.LoadConfig()` is invoked
  independently in ≥10 CLI command bodies (`internal/cli/sidecar.go:90,144,146,
  240,290,340,368,430,466,518`) — each re-reads the file + env instead of a
  config resolved once and threaded in.
- **Function-var test seams = more mutable globals.** `runtime.go:15-16`
  (`var runTurnWithOptions = RunTurnWithOptions`, `var checkACPHandshake =
  CheckACPHandshake`) and `serve.go:166` (`var askSidecar = func(...)`) are
  package-global function variables that exist to be reassigned in tests — the
  same anti-pattern as `githubClients` (see M2).

**Why it violates a tenet.** AGENTS.md Engineering Tenets: *"construct it once
and inject/share it rather than scattering the same responsibility across
packages."* Ben Johnson's fourth Standard-Package-Layout principle: *"the main
package's job is to choose which dependencies to inject … the main package
simply wires up the pieces."* ghx's `main` wires nothing. The Go proverb
*"clear is better than clever"* applies to dependencies too: an argument you can
see beats a global you must discover. The concrete cost is already visible — the
client-swap-by-assignment and 16 `t.Setenv` sites (AUD3 M1) both forfeit
`t.Parallel()` because behavior depends on `os.Environ` and mutable package
state, not on arguments.

**Why it's the highest-leverage idiomatic fix.** Every future dependency P3/P4
adds — the sidecar tool registry, a swappable harness, a rate-limited client —
either becomes *another* ambient global (compounding the debt) or gets threaded
through an explicit root (retiring it). Establishing the root now is what makes
"inject the registry / inject the harness" a one-line wiring change instead of a
new singleton each time.

**Recommendation.** Introduce one explicit composition step — a `func
buildApp(...)` (or equivalent wiring in the `internal/cli` root, invoked from
`cmd/ghx/main.go`) — that resolves config + paths **once**, constructs the
GitHub client **once**, opens telemetry **once**, and (later) builds the tool
registry and harness **once**, then passes them into the command handlers. This
is **not a DI framework** and not a service locator — it is a handful of
constructor calls in one file, which *is* the Go way. Sequence it after H2 (so
core is a clean leaf first); see Recommended sequence Phase 2.

### H2 — The domain core imports the execution frontend (`internal/ghx` → `internal/codemode`)

**What.** The code-reconnaissance *domain* package imports the JS-execution
package purely to register its operations into that package's tool registry. A
domain package should depend on nothing but domain types; here it depends on the
executor it is supposed to know nothing about.

**Evidence.**
- `internal/ghx/register.go:3` — `import ".../internal/codemode"`.
- `RegisterTools(r *codemode.Registry)` (`register.go:6`) plus five `wrap*`
  adapters (`register.go:91-160`) map core ops into `codemode.Tool{Name, Schema,
  Func, …}` — schema literals, default clamps, and `map[string]any` arg-decoding
  all live in the domain package.
- `go list` confirms the edge: `internal/ghx <- … internal/codemode
  internal/mapengine`. `codemode` is a **leaf** (imports no internal package),
  so there is no cycle — the arrow points *from* core *to* the executor, the
  wrong direction.
- This is the one dependency-direction subtlety the earlier architecture audit
  missed: it asserted *"grep for internal/{cli,codemode,sidecar} inside
  internal/ghx/*.go returns empty — core has zero upward dependencies"*
  (`docs/audits/architecture-2026-07-07.md:49-51`). That is true only if
  `codemode` is not a frontend — but `codemode` is the executor the CLI/MCP
  `code` tool wraps, so core depending on it is a real (small) inversion.

**Why it violates a tenet.** Ben Johnson's first principle: *"the root package is
for domain types … [it] should not depend on external implementations."* The
`wrap*`/`Schema` code is *frontend adapter code* (it exists to expose core to the
`code` tool) living inside the domain. It is also the M1/AUD1 "tool contract
triplicated across three frontends" finding seen from the dependency angle: core
owns one of the three copies, and it does so by importing the executor.

**Recommendation.** Relocate `RegisterTools` + the `wrap*` adapters out of
`internal/ghx` into the composition root (`internal/cli`, which already imports
both `ghx` and `codemode`) or a tiny sibling adapter package. `internal/ghx`
then becomes a **pure domain leaf** — depending only on `mapengine` and the
stdlib — which is what makes it safely importable from anywhere (including, one
day, an in-process P3 tool host) with zero risk of a cycle. This is pure
relocation: byte-identical behavior, verifiable with `go test ./...`. It is also
the correct *placement precedent* for the P3 sidecar `ToolSpec` registry (AUD4
V1 / ADR-0035 item 4): registries are composition-root concerns, not domain
concerns.

### Goga's question, answered concretely: singletons vs. services

**Services — yes, the AGENTS.md tenet is correct idiomatic Go.** A Go "service"
is just a struct that owns its dependencies and exposes methods, built once and
shared. ghx already has good ones (`tier2.Service`, the telemetry provider, the
daemon `AgentPool`, the injected `TurnRunner` seam). Keep building these. The
guardrails that keep them idiomatic — and away from Java/enterprise DI:
- **Construct at the composition root, pass explicitly** (H1). No global
  registry of services, no service locator, no `init()` magic.
- **Accept interfaces, return structs.** Constructors return the concrete
  `*Service`; consumers that want to fake it define their *own* small interface
  at the call site (as `runtime.go`'s `TurnRunner` already does).
- **Keep the interface small** — `graphQLDoer` is one method; that is the target
  size, not a 15-method "manager."

**Singletons — mostly no; here specifically:**
- `githubClients` (`client.go:33`): **right interface, wrong container.** The
  seam is correct and the interfaces are exemplary; the *mutable global* holding
  them is the smell. The stdlib's own taste line: `http.DefaultClient` is an
  acceptable singleton because it is a *stateless, documented, replaceable
  default* used for ergonomics — not a value you reassign inside `t.Cleanup`.
  The client-swap-by-assignment in `client_test.go` is the signal that this
  wanted to be injected. Right-sized fix: keep a zero-config default constructor
  for the bare `ghx explore` path, but let the operations *accept* the client
  (as a parameter, or by promoting them onto a `ghx.Client` struct with methods)
  so tests and P3 can pass their own — Phase 5.
- `runTurnWithOptions` / `checkACPHandshake` / `askSidecar` (M2): same
  anti-pattern, and the codebase already has the idiomatic alternative two files
  over (`TurnRunner` injected through `AskWithTurnRunner`). Prefer that.
- **Acceptable package-level globals that are *not* singletons in the bad
  sense:** immutable lookup tables and compiled regexes (`mapengine/regex.go`,
  `route.go` connective tables, `session_options.go:159` `depthBudgets`) are
  stateless data — leave them. The `cobra.Command` vars in `internal/cli` are
  the framework's idiom — leave them. The rule is precise: **stateless/immutable
  package var = fine; mutable global holding a dependency or swapped in tests =
  make it an injected value.**

**The one-sentence rule for ghx:** use *services* (structs + explicit
injection) freely; treat every *mutable* package global as a bug-in-waiting and
resolve it at a real composition root — and never reach for a DI container or
service locator to do it, because in Go the composition root *is* the DI
container.

---

## Medium severity (design)

### M1 — `internal/sidecar` is a flat 29-file god-package; extract only the harness, and only now

**What.** The runtime domain and everything around it live in one flat package:
29 non-test files spanning the ACP harness, session store, route engine, ledger,
report contract, persona, tier reconciliation, config, daemon, and resilience.
Cohesion is low; a reader must hold the whole package to understand any part.

**Evidence.** `find internal/sidecar -maxdepth 1 -name '*.go' ! -name '*_test.go'`
→ 29 files. The domain concepts (`Report` `report.go:38`, `AskRequest`
`runtime.go:218`, persona `prompt.go:37`), the ACP adapter (5 SDK-touching files,
per AUD4), the session/route/ledger, and the daemon are all sibling files in one
namespace with no internal boundary.

**Why it violates a tenet.** AGENTS.md "one authoritative owner per domain
concern … decouple code whose responsibilities do not belong together." A flat
god-package is the package-scale version of the god-file the `acp.go` split
already fixed at file scale.

**Why the recommendation is narrow (right-sizing).** Two idiomatic-Go cautions
make a full subpackage decomposition the *wrong* move today:
1. **Cycle risk.** The sidecar's pieces are mutually referential (runtime →
   session, route, report, harness, telemetry). Splitting entangled files into
   subpackages in Go tends to produce import cycles that force awkward
   interface-shuffling — a real project, not a mechanical move.
2. **Premature domain/framework split.** AUD4 V3 is explicit that extracting the
   reusable "framework" from the ghx domain now *fails the north-star filter*.
   Agreed — do not create `internal/sidecar/framework` vs `.../ghxrecon`.

**Recommendation.** Make exactly one cut: extract the ACP harness into
`internal/sidecar/harness` (or `.../acp`) behind a consumer-defined `Harness`
interface. It is the cleanest seam (already the 5-file SDK cluster, AUD4), it is
the P4 enabler (AUD4 V2 / ADR-0035 item 5), and it has the least cycle risk
because the runtime already talks to it through the `TurnRunner` function seam —
promote that function to the interface method. Beyond the harness, prefer
*file-level* cohesion within `package sidecar` (continue what the `acp.go` split
started) over speculative subpackaging. **Do not** add role-layer subpackages
(`models/`, `services/`) — that is the layered anti-pattern.

### M2 — Function-var test seams are an un-idiomatic, inconsistent substitute for injection

**What.** Three package-global *function variables* exist solely to be swapped
in tests, duplicating the job the injected-interface pattern already does
elsewhere in the same package.

**Evidence.** `internal/sidecar/runtime.go:15` `var runTurnWithOptions =
RunTurnWithOptions`; `runtime.go:16` `var checkACPHandshake = CheckACPHandshake`;
`internal/cli/serve.go:166` `var askSidecar = func(ctx, cfg, req) (…)`. Contrast
the idiomatic alternative two files over: `TurnRunner` is a function *type*
threaded as a *parameter* through `AskWithTurnRunner` (`runtime.go`), so the
daemon injects the warm worker and daemonless calls inject `runTurnWithOptions`
— no global swap needed.

**Why it violates a tenet.** AGENTS.md "refactor cleanly or not at all — a
refactor that leaves both the old and new pattern in the tree doubles the surface
every future agent must understand." The package carries *both* the injected seam
and the global-var seam for the same kind of dependency; the inconsistency is the
smell.

**Recommendation.** Prefer the injected-parameter seam. As the composition root
lands (H1) and call paths get threaded explicitly, retire the global function
vars. Low priority; folds into H1's wiring work. Behavior-preserving.

---

## Low severity (design)

### L1 — The `Repo` value object stops at the core boundary; the eval layer still hand-splits

**What.** ADR-0035 T1.1 threaded `ghx.Repo` through `Explore/Read/Tree`, but two
callers still treat repo identity as a bare string: `inspect.go:244` calls
`ParseRepo(repo)` only to *validate*, then discards the value and keeps passing
the string; and `internal/sidecar/evals/discovery.go:667` still hand-rolls
`strings.Split(repo, "/")`. The value object exists but has not fully displaced
the primitive.

**Why it's Low.** No live panic (the panic path is fixed); this is
consistency/follow-through, and the eval site is in the frozen measurement
package (touch only under a pre-registered eval ADR). Flagged so the value-object
migration is finished deliberately rather than left half-applied — the exact
"two patterns in the tree" the refactor tenet warns about, at low blast radius.

**Recommendation.** When `inspect.go` is next touched, have it carry the parsed
`Repo` inward instead of re-passing the string. Leave the eval-layer split for
whatever eval ADR next opens `discovery.go`. Not a standalone change.

---

## Target package architecture

The target is **deliberately close to today's layout** — the shape is right, so
this is refinement, not a rewrite. Three changes: (a) make `internal/ghx` a pure
domain leaf, (b) add a real composition root, (c) extract the harness. Everything
else stays.

```
cmd/ghx/                        Composition root: resolve config+paths ONCE,
                                construct GitHub client, telemetry, tool
                                registry, harness ONCE; hand them to cli.
internal/cli/                   Delivery: cobra commands + MCP handlers (thin) +
                                presenters. OWNS tool registration (moved here
                                from core). Depends on everything; nothing
                                depends on it.
internal/ghx/                   Code-recon DOMAIN — a PURE LEAF.
                                Repo, ExploreResult, ReadResult, SearchMatch, …
                                Explore/Read/Search/Repos/Tree over an ACCEPTED
                                client (param or *ghx.Client), not a global.
                                Imports: mapengine + stdlib only.
internal/codemode/              Capability (leaf): goja executor + tool Registry.
internal/mapengine/             Capability (leaf): structural map engine.
internal/sidecar/               Sidecar-runtime DOMAIN: Report, AskRequest,
                                persona, runtime, session, route, ledger, daemon.
  internal/sidecar/harness/     NEW — ACP/agent adapter behind a consumer-defined
                                Harness interface (P4 seam). claude-agent-acp is
                                the first implementation; the _meta.claudeCode
                                encoding, permission taxonomy, and error-string
                                classifiers move INSIDE it.
  internal/sidecar/tier2/       Capability: absorbed structural tools.
  internal/sidecar/telemetry/   Capability (leaf): OTel emission.
  internal/sidecar/evals/       Measurement stack — FROZEN. Correct boundary
                                already; do not restructure outside an eval ADR.
```

**Rationale, tied to sources:**
- **Domain root + dependency subpackages + main-wires** is Ben Johnson's
  Standard Package Layout, applied twice (once per domain: `ghx` and `sidecar`).
  Each domain package holds its types and its consumer-defined interfaces;
  capability/adapters (`codemode`, `mapengine`, `harness`, `tier2`, `telemetry`)
  isolate a dependency; `cmd/ghx` wires them.
- **No core/shared/domain *layer* packages.** The domains are the packages;
  layering by role would be the un-Go move Kat Zien and Johnson both warn
  against.
- **Shared domain types stay put.** Per ADR-0010, code-recon types live in
  `internal/ghx`; sidecar types live in `internal/sidecar`. They are separated by
  a *process* boundary (ACP), so no shared-Go-type layer is needed — building one
  now fails the no-speculative-framework tenet. **If** P4 ever hosts a tool
  in-process and genuinely needs a type shared across a package boundary, that
  single type moves to the importing-nothing leaf (`internal/ghx`), not into a
  new `internal/domain` layer.
- **P3 registry and P4 harness are injected *values*, not globals.** The sidecar
  `ToolSpec` registry is a struct built at the composition root (model:
  `codemode.Registry`, which already proves the shape in-tree) and passed to the
  runtime; the `Harness` is a consumer-defined interface the runtime accepts.
  Both are "accept interfaces, return structs," and both are cheap *because* the
  composition root from H1 exists.

## Recommended sequence (phased, non-rewrite)

Every step is incremental, independently shippable, behavior-preserving, gated by
`go test ./...` + race, and **touches no eval scoring/gates/detectors** (frozen
measurement). Phases 3–4 align with ADR-0035 Tier-2; this audit adds the
idiomatic ordering and the composition-root prerequisite (Phases 0–2).

- **Phase 0 — Posture (zero-cost, immediately).** Stop adding mutable package
  globals and wrong-direction edges. Any new dependency is constructed at the
  root and passed; any new sidecar domain surface (`Report`/`AskRequest`/persona)
  stays behind a small seam (generalizes AUD4 V3). Discipline, not a project.

- **Phase 1 — Make `internal/ghx` a pure leaf (H2).** Move `RegisterTools` +
  `wrap*` from `internal/ghx/register.go` to `internal/cli` (or a tiny adapter
  package). Pure relocation, byte-identical, `go test ./...`. Unblocks Phase 2
  and positions the P3 registry at the right layer.

- **Phase 2 — Establish the composition root (H1, M2; folds ADR-0035 item 9).**
  Add one wiring step in `cmd/ghx`/`internal/cli` that resolves config+paths and
  constructs the GitHub client + telemetry once. Retire `rootDir()` per-call env
  reads (→ `Config.Home` resolved once) and the function-var seams
  (`runTurnWithOptions`/`checkACPHandshake`/`askSidecar`) as their call paths get
  threaded. Behavior-preserving; unlocks parallel storage/client tests.

- **Phase 3 — Sidecar `ToolSpec` registry (P3; ADR-0035 item 4).** One
  declaration per tool, injected as a value from the composition root; generate
  the persona menu, CLI wiring, and `tier.go` reconciliation from it. **Must
  reproduce persona + backend-ID output byte-for-byte (golden)** so it is
  measurement-identity-neutral (frozen-stack rule). New tools are separate,
  pre-registered persona revisions — keep mechanism and content in different
  commits.

- **Phase 4 — Extract `internal/sidecar/harness` behind a `Harness` interface
  (P4; M1 + ADR-0035 item 5).** Consumer-defined interface owned by the runtime;
  claude-agent-acp as the first impl, absorbing the `_meta.claudeCode` encoding,
  permission taxonomy, and error-string recovery. Promote `TurnRunner` to the
  interface method. Cheapest while exactly one adapter exists.

- **Phase 5 — Promote core ops onto a `ghx.Client` struct (optional; answers the
  singleton question structurally).** When a client must carry auth/rate-limit
  state (likely a P3 need), turn `Explore/Read/…` into methods on a `*ghx.Client`
  constructed once, retiring the `githubClients` global. Keep a zero-config
  default constructor so bare `ghx explore` still works. Do this *when the state
  arrives*, not speculatively.

- **Posture (ongoing) — do NOT build a shared `internal/domain` layer or extract
  the framework/domain split.** No cross-package shared-type problem exists today;
  building for one fails the north-star and no-speculative-framework filters
  (AUD4 V3).

## Method / auditability

- **Worktree/drift.** All `file:line` above verified against **current mainline,
  HEAD `1bda884`** (the integrated shared checkout), which is *ahead* of the five
  audits' worktrees — ADR-0035 Tier-1 landed there (`9e4a10e`, `b972c4b`,
  `26ae9ef`). This audit's own worktree is branched from the older
  `d2cda8a`/v2.7.0 and does *not* itself contain the Tier-1 files; the doc
  deliberately cites and describes current mainline, the tree it will merge into.
  Citations borrowed from AUD1/AUD3/AUD4 were re-checked against `1bda884`; where
  lines drifted the current number is used (e.g. `AskRequest.Repo` now
  `runtime.go:224`, not `:221`).
- **Import graph** from `go list -f '{{join .Imports "\n"}}' ./internal/...`
  filtered to `gkoreli` paths: `internal/ghx <- internal/codemode,
  internal/mapengine`; `internal/sidecar <- .../telemetry, .../tier2` (does **not**
  import `internal/ghx`); `internal/codemode`/`telemetry`/`mapengine` are leaves;
  `internal/cli` imports codemode+ghx+sidecar+evals+tier2 (sole composition root).
- **Singleton/service inventory** from `grep -rn '^var ' internal cmd
  --include='*.go' | grep -v _test.go` (separating immutable data tables and
  cobra vars from the mutable dependency-holding globals `githubClients`,
  `runTurnWithOptions`, `checkACPHandshake`, `askSidecar`), and reading each
  constructor (`tier2.NewService`, `telemetry.NewTracerProvider`) to confirm the
  injected-service pattern.
- **Composition-root diffuseness** from reading `cmd/ghx/main.go` (13 lines, no
  wiring) and `grep -n 'LoadConfig' internal/cli/sidecar.go` (10+ per-command
  reloads).
- `go build ./...` exits 0 against `1bda884` at audit time. **No code was
  modified by this audit; the only file created is this document.**

## Sources (idiomatic-Go grounding)

- Ben Johnson, *Standard Package Layout* — root package for domain types; group
  subpackages by dependency; the main package ties together (injects)
  dependencies. https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1
  / https://www.gobeyond.dev/standard-package-layout/
- Rob Pike, *Go Proverbs* — "The bigger the interface, the weaker the
  abstraction"; "A little copying is better than a little dependency"; "Clear is
  better than clever." https://go-proverbs.github.io/
- "Accept interfaces, return structs" — community Go wisdom (Jack Lindamood;
  *not* a canonical Pike proverb): the consumer defines the interface it needs;
  constructors return concrete structs.
- Kat Zien, *How Do You Structure Your Go Apps* (GopherCon 2018) — flat vs
  layered vs modular vs DDD; prefer grouping by domain/context over role layers.
- The Go standard library as the taste reference for acceptable defaults
  (`http.DefaultClient`: a stateless, documented, replaceable convenience — not
  a test-swap global).
