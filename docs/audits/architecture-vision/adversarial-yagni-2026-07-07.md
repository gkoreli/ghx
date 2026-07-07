---
title: "Adversarial YAGNI Architecture Audit - ghx"
date: "2026-07-07"
status: "audit"
author: "Codex adversarial architecture worker"
scope: "short-term risk, solo-developer velocity, YAGNI counterweight - read-only audit"
---

# Adversarial YAGNI Architecture Audit - 2026-07-07

Read-only architecture audit from the anti-cathedral side. The prior audits are
useful, but their natural failure mode is obvious: turn every repeated string
into a type, every call boundary into a port, and every concrete package into a
layer cake. That is how Go projects become enterprise Java with shorter syntax.

ghx is a solo-developer product plus AI agents. The architecture must make the
next month cheaper, not satisfy an abstract org chart. The bar for a new
boundary is therefore: it removes a real bug class, protects a roadmap swap
that Goga has already named, or makes high-value behavior testable. Everything
else waits.

Go grounding:

- Rob Pike's Go proverb ["the bigger the interface, the weaker the abstraction"](https://go-proverbs.github.io/)
  is the right smell test here. A "Runner framework" interface with lifecycle,
  tools, permissions, telemetry, reports, sessions, and config is weak because
  it hides too many unrelated reasons to change.
- The Go wiki says to [accept interfaces and return structs](https://go.dev/wiki/CodeReviewComments#interfaces):
  interfaces belong at the consumer seam, not sprayed across provider packages
  "for flexibility."
- Ben Johnson's package-oriented design argues packages should be grouped by
  domain dependency, not horizontal technical layers
  ([Standard Package Layout](https://www.gobeyond.dev/standard-package-layout/)).
  For ghx, `internal/ghx`, `internal/sidecar`, `internal/mapengine`, and
  `internal/codemode` already mostly do that.

## Executive summary

**The correct short-term posture is not "build core/shared/domain/service
layers." It is: keep the current package boundaries, preserve the already-good
core/frontend split, and add only two tactical seams where runtime facts demand
them: one harness seam around the agent runner, and small value types only for
concepts with demonstrated drift or bugs.**

Several Tier-1 findings in ADR-0035 are already stale in this worktree:
`internal/ghx/repo.go:8-27` now defines `Repo`/`ParseRepo`, `Explore`,
`Read`, and `Tree` parse through it (`explore.go:21-27`, `read.go:54-59`,
`tree.go:14-19`), and `fetchTree` now takes `Repo` instead of splitting a
string (`glob.go:16-27`). The core client seam also exists:
`internal/ghx/client.go:5-33`, with an offline injected-client test at
`internal/ghx/client_test.go:58-109`. Good. Do not convert these wins into a
general "domain model everything" campaign.

The real risk is now overcorrection:

- A `core/shared/domain` layer would duplicate what packages already express.
  `internal/ghx` is the core recon package; `internal/cli` composes frontends;
  `internal/sidecar` owns sidecar runtime; `internal/sidecar/tier2` owns local
  structural tools. That is idiomatic Go. Adding a `domain` package just makes
  everyone import the same junk drawer.
- A broad service-object pass would be premature. `tier2.Service` is a real
  service because it owns cache, clone materialization, tool adapters, and
  artifact hashes (`tier2/service.go:176-211`, `:363-405`). By contrast, a
  `RepoService`, `DepthService`, `TierService`, `ReportService`, and
  `PromptService` would mostly wrap functions and maps with ceremony.
- A ports/adapters pass "everywhere" is gold-plating. The code already has
  narrow seams where needed: `TurnRunner` (`runtime.go:18-20`), `githubClients`
  (`client.go:13-33`), `GitRunner`, `ContainerRuntime`, `WritePolicy`, and
  `mapengine.Mapper` cited by the prior audits. More interfaces should be
  earned by a second implementation or by tests that cannot be written without
  one.

The minimal runner replacement seam Goga wants is also smaller than the
architects are making it. The product needs to swap `claude-agent-acp` for
`codex-acp` or a Claude Agents SDK harness. That does **not** require a generic
Runner Framework. It requires one consumer-owned interface at the sidecar
runtime boundary: "given a prompt, session identity, steering spec, cwd/env,
report sink, and live log path, run one turn and return `TurnResult` plus a
resume token." Everything below that is adapter implementation detail.

---

## High severity

### H1 - Do not build `core/shared/domain` layers; Go packages are already the boundary

**What.** The current package split is mostly the right Go architecture:
`internal/ghx` contains recon operations and result types, `internal/cli` is
the composition/presentation frontend, `internal/sidecar` owns runtime/session
behavior, `internal/sidecar/tier2` owns local structural analysis, and
`internal/mapengine` owns structural mapping. A horizontal `core`, `shared`,
`domain`, `services`, `ports`, `adapters` rewrite would make every change cross
more files without adding a second product consumer.

**Evidence.**

- Core recon is already in one domain package. `Explore` returns
  `ExploreResult`/`FileEntry` in `internal/ghx/explore.go:7-21`; `Read`
  owns `ReadOpts`/`FileResult` in `internal/ghx/read.go:18-49`; `Tree`
  owns `TreeOpts` in `internal/ghx/tree.go:7-14`.
- The core/frontend split is intact in actual calls: CLI `explore` reads flags,
  calls `ghxlib.Explore`, then prints (`internal/cli/ghx.go:91-133`); MCP
  `handleExplore` reads request args, calls `ghxlib.Explore`, then JSON-marshals
  (`internal/cli/serve.go:236-250`).
- `tier2.Service` is already a proper domain service where a service is
  justified: cache, git runner, tool adapters, version, clock, active dirs
  (`internal/sidecar/tier2/service.go:176-211`). Its methods own the actual
  local-tool behaviors (`RunCodemap`, `RunAstGrep`, `RunRepomap`,
  `service.go:363-405`).
- The now-landed `Repo` type lives in the package that owns GitHub recon
  (`internal/ghx/repo.go:8-27`), not in a global `domain` package. That is the
  right precedent.

**Why over-engineering is harmful here.** A `domain` package would immediately
create import gravity: `Repo`, `Depth`, `Tier`, `Backend`, `Report`,
`FailureClass`, and maybe `SessionID` all land there because they sound
"domain-ish." Then `internal/ghx`, `internal/sidecar`, `internal/cli`, and
`internal/sidecar/tier2` all depend on the same generic package. That is worse
cohesion than today. Go packages should hide implementation details behind
directory boundaries; they should not become DDD vocabulary bins.

**Recommendation.** Keep domain nouns in the package that owns their behavior:

- `ghx.Repo` stays in `internal/ghx`.
- Sidecar depth/runner steering stays in `internal/sidecar`.
- Tier/backend vocabulary stays near `Report`, `tier.go`, and `tier2` unless a
  concrete drift bug demands a shared type.
- Do not create `internal/core`, `internal/shared`, `internal/domain`, or
  `internal/services`.

**Tenet tie.** This still satisfies "core capabilities live in `internal/ghx`"
and "one authoritative owner per concern." The owner is the package, not an
extra layer.

### H2 - The runner replacement seam should be one small harness interface, not a Runner Framework

**What.** Goga's real requirement is narrow: replace `claude-agent-acp` by
config with `codex-acp` or a Claude Agents SDK harness. Today adapter-specific
facts leak through `SessionOptions` and error classification, so a seam is
needed. But a general runner framework would be gold-plating; there is one
product, one runner role, and one report contract.

**Evidence.**

- `RunTurnOptions` is already the runtime's de facto runner boundary:
  agent command, ACP session ID, prompt, cwd/env, session meta, report sink,
  liveness, stderr path, live log path (`internal/sidecar/acp.go:157-197`).
- `TurnRunner` is the consumer-owned seam the runtime already calls:
  `type TurnRunner func(context.Context, RunTurnOptions) ...`
  (`internal/sidecar/runtime.go:18-20`). `Ask` injects the one-shot runner and
  daemon mode injects warm worker runners (`runtime.go:260-270`,
  `daemon_worker.go:39-50`).
- The adapter lock is real but localized. `SessionOptions` explicitly says it
  targets `claude-agent-acp` (`internal/sidecar/session_options.go:3-11`) and
  emits `_meta.claudeCode` (`session_options.go:222-229`).
- Permission and recovery are adapter-shaped: MCP submit-report auto-approval
  depends on claude-agent-acp's "other" tool kind (`session_options.go:49-65`),
  and max-turn recovery matches adapter English text
  (`internal/sidecar/acp.go:75-80`).
- The runtime already wants only one thing: `runPrimaryTurn` calls a runner with
  built options (`runtime.go:438-441`), `recoverMaxTurns` calls it again with a
  wrap-up prompt (`runtime.go:444-467`), and report retry uses the same shape.

**Minimal seam.** Replace or wrap `TurnRunner` with a tiny interface owned by
`internal/sidecar`, not by a generic framework package:

```go
type Harness interface {
	RunTurn(context.Context, TurnRequest) (TurnResult, ResumeToken, error)
}
```

Where `TurnRequest` is just today's `RunTurnOptions` made harness-neutral:
`Prompt`, `ResumeToken`, `SteeringSpec`, `Cwd`, `Env`, `ReportSinkPath`,
`AgentStderrPath`, `LiveLogPath`, `LivenessTimeout`. `SteeringSpec` carries
the portable facts already present today: persona, depth budget, model,
setting isolation, builtin tool allowlist, report-sink auto-approval. The
Claude ACP implementation alone encodes that as `_meta.claudeCode`.

**What would be gold-plating.**

- A runner plugin registry before there are two working harnesses.
- Lifecycle interfaces for every phase (`Starter`, `SessionLoader`,
  `Prompter`, `Reporter`, `TelemetryEmitter`) before the second harness proves
  the split.
- Generic event buses for tool calls. `TurnResult` already captures what the
  product needs; translate adapter events at the harness edge.
- A public SAF runner API. ghx is still proving the product; do not freeze a
  framework surface for imaginary external consumers.

**Recommendation.** Build the seam only at the point where the sidecar runtime
currently calls `TurnRunner`. Keep the existing `RunTurnWithOptions` and
`AgentWorker.RunTurn` as the Claude ACP implementation. Do not move session
routing, report validation, tier reconciliation, or telemetry behind the runner.
Those are sidecar product behavior, not runner behavior.

**Tenet tie.** "Protocols are stepping stones" is satisfied by isolating ACP
encoding. "Open source leverage" is satisfied by adopting existing harnesses.
Inventing a runner framework violates both.

### H3 - Treat the sidecar tool registry as a string-generation aid, not a tool-platform architecture

**What.** The prior velocity audit is right that Tier-2 tool knowledge is
spread across persona prose, CLI shell commands, report backend IDs, and
runtime reconciliation. But the wrong fix is a platform. The current sidecar
uses shell commands on purpose: the agent runs `ghx tier2 ...`, and the runtime
records what happened post-hoc (`internal/sidecar/tier.go:14-20`). That is
simple, auditable, and good enough for three tools.

**Evidence.**

- The persona lists the Tier-2 commands directly at
  `internal/sidecar/prompt.go:145-154`.
- `tier2.Service` has explicit fields and methods for current tools
  (`internal/sidecar/tier2/service.go:180-188`, `:363-405`).
- Runtime tier detection is deliberately command-based:
  `anyTier2Command` and `deriveTierUsed` inspect observed `ghx tier2` command
  lines (`internal/sidecar/tier.go:128-152`).
- Report/backend vocabulary is visible in the report contract
  (`internal/sidecar/report.go:45-53`) and submit_report schema descriptions
  (`internal/sidecar/reportsink.go:252-265`).

**What is premature.** A generic in-process tool execution framework with
registration, routing, permission models, telemetry hooks, CLI generation,
schema generation, and adapter-specific call interception is too much. The
agent does not call in-process tools today; it shells out. A framework that
pretends otherwise will fight the actual runtime.

**Recommendation.** If P3 adds a fourth or fifth Tier-2 tool, introduce a
small `sidecarToolSpec` table that generates only the duplicated strings:
backend ID, example command, prompt line, and report-field description. Keep
execution explicit in `tier2.Service` until there is evidence that generic
execution removes real duplication. The registry's first job is byte-identical
prompt/report vocabulary, not tool orchestration.

**Tenet tie.** "Moat is the agentic brain, not the tools." Do not accidentally
spend the month building the tool platform instead of improving the brain.

---

## Medium severity

### M1 - Domain types that genuinely deserve to exist

These types pass the YAGNI test because they already fixed or would prevent
observable drift.

1. **`ghx.Repo` - already done.** It removed repeated `owner/repo` validation
   and the old tree panic risk. Evidence: `Repo`/`ParseRepo`
   (`internal/ghx/repo.go:8-27`), callers in `Explore`, `Read`, `Tree`
   (`explore.go:21-27`, `read.go:54-59`, `tree.go:14-19`), and tests
   (`internal/ghx/repo_test.go`).
2. **Harness-neutral runner request/steering types - should exist.** This is
   not type maximalism; it is the minimal P4 seam. Today `SessionOptions` is
   explicitly adapter-specific (`session_options.go:3-11`) and `BuildSessionMeta`
   hardcodes `_meta.claudeCode` (`session_options.go:222-229`).
3. **`Depth` - maybe, but only if it replaces current divergent behavior.** MCP
   rejects invalid depth via `isValidReconDepth` (`internal/cli/serve.go:223-234`),
   while CLI forwards the string (`internal/cli/sidecar.go:69-97`) and
   `BuildSessionMeta` silently falls back to normal (`session_options.go:194-198`).
   A `Depth` type is justified if it makes both frontends choose one behavior.
4. **Failure class - yes for CLI/MCP recon-operation outcomes, no for all
   runtime/eval anomalies.** The inspect CLI still classifies bad input by
   substring (`internal/cli/ghx.go:420-424`). That is a real bug vector. But
   folding runtime liveness markers and eval anomaly buckets into the same enum
   would make one overloaded type that lies about different concepts.

### M2 - Domain types that should stay strings or plain structs for now

1. **`FileEntry.Type` / `treeEntry.Type`.** It is `"blob"` or `"tree"`
   (`internal/ghx/explore.go:7-10`, `internal/ghx/glob.go:10-14`) and has a
   couple of comparisons (`glob.go:88-90`, `tree.go:39-44`). A `FileType` enum
   buys almost nothing until there is a bug or more behavior.
2. **`TierUsed`.** It is tempting to type this, but today it is primarily a
   report JSON vocabulary and persisted artifact value (`report.go:45-53`,
   `reportsink.go:210-227`, `tier.go:139-152`). A type is fine if it stays in
   `internal/sidecar` and deletes duplicated maps. Do not introduce a global
   tier domain model or ordering service.
3. **Backend IDs.** `"remote"` and `"local:*"` are part of the report contract
   and prompt/schema prose (`report.go:50-53`, `reportsink.go:261-262`). A
   string table is enough. A backend class hierarchy is nonsense until backends
   have polymorphic behavior inside the runtime.
4. **`Report`.** It is already a proper product contract with parsing and
   validation behavior (`report.go:36-98`, `reportsink.go:214-230`). Do not
   wrap it in `DomainReport`, `EvidenceReport`, or generic report envelopes for
   a future browser sidecar. That future product is explicitly a consequence,
   not the steering goal.
5. **Session IDs, route sources, command strings.** These are persisted strings
   and user-visible artifacts. Type them only at a validation edge. Do not
   contaminate every call site with newtypes that only call `String()`.

### M3 - Service objects and singletons: use the ones with state, skip the wrappers

**Good service objects already present.**

- `tier2.Service` owns real mutable state and dependencies (`service.go:176-211`).
- `AgentPool` owns warm workers and concurrency (`daemon_worker.go:16-50`).
- The client provider seam in `internal/ghx/client.go:13-33` is just enough to
  test API paths offline.

**Bad service objects to avoid.**

- `RepoService` around `ParseRepo`.
- `DepthService` around a map.
- `TierService` around `deriveTierUsed`.
- `ReportValidatorService` around `ValidateReport`.
- `PromptBuilderService` around `BuildPersonaSystemPrompt` and `BuildPrompt`.

In Go, a function in a package is already a namespaced operation. Wrapping
stateless functions in structs just creates constructor traffic and fake DI.

### M4 - CLI and MCP cleanup should be opportunistic, not a grand frontend framework

The CLI files are large and MCP handlers repeat a parse-call-marshal shape, but
this is medium-grade friction, not an architectural emergency.

**Evidence.**

- CLI `explore` has inline rendering (`internal/cli/ghx.go:91-133`), while
  `inspect` delegates to `ghxlib.FormatInspectText` (`ghx.go:413-426`).
- MCP handlers repeat the same pattern across tools
  (`internal/cli/serve.go:236-350`).
- `sidecar ask` reads flags, calls daemon, chooses JSON vs human output, prints
  route/artifact footer in one body (`internal/cli/sidecar.go:69-128`).

**YAGNI position.** Do not build a frontend framework. Extract presenters or
`run*` functions only when touching a command for a user-facing change. The
first low-risk targets are `printHumanReport` and `sidecar ask`, because the
CLI reaches directly into `Report` fields (`sidecar.go:717-721`). A whole
`internal/cli/present` package can wait until there are two or three presenters.

---

## Low severity

### L1 - Avoid generic framework extraction for second-domain sidecars

The velocity audit is right that a future browser sidecar would not reuse
`package sidecar` cleanly: `AskRequest.Repo` is GitHub-specific
(`internal/sidecar/runtime.go:217-243`), `Report` is code-recon-specific
(`report.go:36-57`), and the persona says GitHub repository reconnaissance
(`prompt.go:37-47`). But the North Star labels other domain sidecars as a
consequence product, not the current steering target.

**Recommendation.** No framework extraction now. The only discipline needed:
when adding new ghx-specific behavior, keep it in named functions/types so a
future extraction has seams to cut. That means no anonymous mega-structs, no
cross-package globals, and no hidden environment dependencies. It does not mean
building `internal/framework`.

### L2 - Do not sweep all clocks, env vars, and errors

The coupling audit found real ambient dependencies. The YAGNI correction is
scope control:

- Inject clocks only where tests need deterministic time: worker TTL/liveness
  and session ID minting. Do not turn every timestamp into a clock parameter.
- Resolve `GHX_HOME` into config if touching daemon/storage code. Do not create
  a global application context object.
- Add failure classes at frontend boundaries. Do not mass-replace every
  `fmt.Errorf` or invent a universal error taxonomy that mixes GitHub API
  errors, ACP liveness, report validation, and eval anomalies.

### L3 - Do not convert tests into architecture

The client seam has one offline `Explore` test (`internal/ghx/client_test.go:58-109`).
That is useful. The next move is not a test harness framework. Add focused
tests for `Read`, `Search`, and `Tree` only when changing those paths. Let the
need for the fake grow from actual regressions.

---

## Recommended sequence (top changes, ordered)

1. **Freeze the already-landed Tier-1 wins; do not broaden them.** Keep
   `ghx.Repo` in `internal/ghx` and the `githubClients` seam in
   `internal/ghx/client.go`. Add missing focused tests only around changed
   recon paths. No new `domain` or `core` package.

2. **Add the minimal harness seam at the existing `TurnRunner` boundary.**
   Introduce a harness-neutral `TurnRequest`/`SteeringSpec` and a one-method
   `Harness` or equivalent function interface. Move `_meta.claudeCode`,
   Claude permission quirks, and max-turn text matching into the Claude ACP
   implementation. Keep sidecar routing, report validation, tier reconciliation,
   and telemetry outside the harness.

3. **Unify one-shot and warm ACP turn plumbing only as much as the harness seam
   requires.** The code already shares `configureACPSession` and prompt helpers
   (`acp.go:297-305`, `daemon_worker.go:246-264`). Avoid a large
   `acpConversation` object unless it deletes duplicated lifecycle code without
   changing behavior.

4. **Fix `Depth` divergence with the smallest type or parser.** One parser,
   one behavior: reject invalid depth at both CLI and MCP, or intentionally
   coerce at both. This can be a `ParseDepth` function plus constants; it does
   not need a service.

5. **Add a tiny Tier-2 tool spec table only when adding the next local tool.**
   The table should generate prompt/backend-description strings and detection
   constants. It should not execute tools, own permissions, or become a plugin
   framework.

6. **Opportunistically extract CLI presenters while touching commands.** Start
   with sidecar report/session human output; leave MCP direct handlers alone
   until ADR-0034's error mapping lands or another direct tool is added.

7. **Keep eval measurement refactors quarantined behind eval ADRs.** No drive-by
   package slicing in `internal/sidecar/evals`. Measurement auditability beats
   line-count aesthetics.

## What NOT to do

- Do not create `internal/core`, `internal/shared`, `internal/domain`,
  `internal/services`, `internal/ports`, or `internal/adapters`.
- Do not move `Repo`, `Report`, `Depth`, `Tier`, `Backend`, and `FailureClass`
  into one central domain package.
- Do not define provider-side interfaces for every concrete type. Keep
  interfaces at consumers that need substitution.
- Do not build a generic Runner Framework, tool platform, plugin registry, or
  public SAF API before a second harness or second product forces the shape.
- Do not refactor `tier2.Service` internals just because it has explicit fields
  per tool. Explicit is fine at three tools.
- Do not type every string. Type concepts with bugs or drift; leave artifact
  vocabulary strings alone unless the type deletes duplicated validation.
- Do not mass-inject clocks, env, config, loggers, or storage roots through a
  giant app container.
- Do not use `fmt.Errorf` counts or file length as an automatic refactor queue.
  Refactor around behavior, tests, and roadmap swaps.
- Do not extract a generic framework for browser sidecars now. That is a future
  consequence product, not this month's product.
- Do not touch eval scoring/detection outside a pre-registered eval ADR.

## Method / auditability

Files and docs read:

- `docs/NORTH_STAR.md`
- `AGENTS.md`
- `docs/adr/0035-architecture-hardening-refactor-sequence.md`
- `docs/audits/architecture-2026-07-07.md`
- `docs/audits/design-patterns-domain-modeling-2026-07-07.md`
- `docs/audits/complexity-hotspots-2026-07-07.md`
- `docs/audits/coupling-cohesion-testability-2026-07-07.md`
- `docs/audits/adversarial-velocity-2026-07-07.md`
- `docs/audits/failure-class-inventory-2026-07-07.md`
- Current source files under `internal/ghx`, `internal/sidecar`,
  `internal/sidecar/tier2`, and `internal/cli` cited above.

Commands run:

```bash
pwd && git rev-parse --show-toplevel && git status --short
rg --files -g 'AGENTS.md' -g 'docs/NORTH_STAR.md' -g 'docs/adr/0035-architecture-hardening-refactor-sequence.md' -g 'docs/audits/*2026-07-07*.md'
sed -n '1,260p' docs/NORTH_STAR.md
sed -n '1,260p' AGENTS.md
sed -n '1,260p' docs/adr/0035-architecture-hardening-refactor-sequence.md
sed -n '1,260p' docs/audits/architecture-2026-07-07.md
sed -n '1,280p' docs/audits/design-patterns-domain-modeling-2026-07-07.md
sed -n '1,280p' docs/audits/adversarial-velocity-2026-07-07.md
sed -n '280,620p' docs/audits/adversarial-velocity-2026-07-07.md
sed -n '1,300p' docs/audits/complexity-hotspots-2026-07-07.md
sed -n '300,620p' docs/audits/complexity-hotspots-2026-07-07.md
sed -n '1,300p' docs/audits/coupling-cohesion-testability-2026-07-07.md
sed -n '300,620p' docs/audits/coupling-cohesion-testability-2026-07-07.md
sed -n '1,260p' docs/audits/failure-class-inventory-2026-07-07.md
nl -ba internal/ghx/{repo.go,client.go,explore.go,read.go,glob.go,tree.go,client_test.go}
nl -ba internal/sidecar/{runtime.go,acp.go,daemon_worker.go,session_options.go,prompt.go,tier.go,report.go,reportsink.go}
nl -ba internal/sidecar/tier2/service.go
nl -ba internal/cli/{serve.go,ghx.go,sidecar.go}
rg -n 'type Repo|ParseRepo|githubClients|SetGitHubClients|GraphQL' internal/ghx -g '*.go'
```

Text-only external references used:

- Go Proverbs: <https://go-proverbs.github.io/>
- Go Code Review Comments, Interfaces: <https://go.dev/wiki/CodeReviewComments#interfaces>
- Ben Johnson, Standard Package Layout: <https://www.gobeyond.dev/standard-package-layout/>

Test results:

- No tests were run for this audit. This was a read-only architecture audit and
  the only written artifact is this markdown file.

Uncertainty:

- This audit reflects the current worktree, where `Repo` and the core client
  seam already exist. ADR-0035 and earlier audit line references describe an
  older state for those two items.
- The exact final shape of a Codex ACP or Claude Agents SDK harness should not
  be over-specified before one spike proves what metadata, permission, and
  resume semantics it actually exposes.

Suggested next step:

- Accept ADR-0035 only with this YAGNI constraint layered on top: Tier-1 bug and
  testability fixes stay, the runner seam is narrow, and all framework/layer
  extraction is explicitly out of scope until a second implementation exists.
