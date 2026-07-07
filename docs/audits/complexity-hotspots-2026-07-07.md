---
title: "Complexity Hotspots Audit — mechanical maintainability metrics"
date: "2026-07-07"
status: "audit"
author: "background audit worker"
scope: "internal/**, cmd/** — read-only audit, docs-only output"
---

# Complexity Hotspots Audit — 2026-07-07

Read-only maintainability audit focused on mechanical complexity metrics:
file size, cyclomatic complexity, repetition, `fmt.Errorf` sprawl, boolean
control coupling, and long parameter lists. This report intentionally does not
repeat the already-covered top findings from
`docs/audits/architecture-2026-07-07.md` and
`docs/audits/failure-class-inventory-2026-07-07.md`: `callTool` removal,
`acp.go` decomposition, and the failure-class/ADR-0034 work are excluded here.

North-star filter: each hotspot below slows agent and human engineering by
forcing readers to hold extra workflow state in context, making changes less
auditable, or scattering one concern across multiple call sites. Findings tie
back to AGENTS.md Engineering Tenets: one authoritative owner per concern,
decouple responsibilities that do not belong together, define proper domain
models, and refactor cleanly.

## Executive summary

The highest-leverage new debt is the sidecar turn runtime: `askWithTurnRunner`
has cyclomatic complexity 40 and spans the full ask lifecycle in one function
(`internal/sidecar/runtime.go:273`). It mixes routing, session creation,
workspace/env/provenance, ACP turn execution, max-turn recovery, report retry,
ledger writes, tier decisions, artifact persistence, and telemetry emission.
This is tractable because the seams are already visible and mostly correspond
to named comments and helper calls.

The second pattern is CLI growth by accumulation: `internal/cli/sidecar.go` is
760 lines with 12 inline `RunE` bodies, and `internal/cli/ghx.go` is 797 lines
with 8 inline `RunE` bodies. This is not just file size; command construction,
flag parsing, domain invocation, and presentation are co-located, so each new
command makes the command package harder to safely modify.

The eval package is the broadest mechanical hotspot. Non-test code has the
three largest files in the repo (`discovery.go` 1007 lines,
`anticipation_predictor.go` 974, `gates.go` 920), the highest `fmt.Errorf`
counts, and the top cyclomatic function (`TryReuseBaselines`, complexity 45).
Because this touches the measurement stack, refactoring must be ADR-led and
must preserve recomputability of committed artifacts.

---

## High severity

### H1 — Sidecar ask lifecycle is one high-complexity orchestration function

**What.** `askWithTurnRunner` is the largest non-test runtime complexity
hotspot: cyclomatic complexity **40** at
`internal/sidecar/runtime.go:273`. The file is also the seventh-largest
non-test file overall at **689 lines**.

**Evidence.**
- `gocyclo`: `40 sidecar askWithTurnRunner internal/sidecar/runtime.go:273:1`.
- File-size metric: `internal/sidecar/runtime.go` is **689 lines**.
- The function starts at `internal/sidecar/runtime.go:273`.
- It owns session routing/defaulting (`runtime.go:286-290`), session creation
  (`runtime.go:296-313`), workspace/env/provenance resolution
  (`runtime.go:329-343`), prompt/session metadata construction
  (`runtime.go:350-384`), ACP turn execution (`runtime.go:417-427`),
  max-turn wrap-up recovery (`runtime.go:435-481`), unrecovered failure
  artifact emission (`runtime.go:484-520`), report extraction/retry
  (`runtime.go:532-580`), ledger and metadata writes (`runtime.go:599-617`),
  and telemetry/artifact emission (`runtime.go:625-640`).
- It takes **6 parameters** including control-coupling inputs:
  `runner TurnRunner`, `preflight bool`, and `route *RouteDecision`
  (`runtime.go:273`).

**Why it is debt.** This violates "Encapsulate domain logic in services" and
"Decouple code whose responsibilities do not belong together." The function is
the authoritative owner for too many concerns, so changing report retry,
telemetry, stale-session fallback, or routing requires reasoning through the
same 300+ line control flow. The `preflight bool` also makes the same function
serve two modes instead of exposing named lifecycle operations.

**Recommendation.** Extract an `AskCoordinator`/`TurnWorkflow` service with a
small state object and explicit steps:
`prepareSession`, `buildTurnOptions`, `runPrimaryTurn`, `recoverMaxTurns`,
`resolveReportWithRetry`, `persistTurn`, and `emitArtifacts`. Replace the
`preflight bool` with two explicit entry points or an options struct whose
fields are named at call sites. Keep behavior byte-for-byte covered by the
existing sidecar runtime tests; this is a refactor, not a semantics change.

### H2 — Eval measurement code concentrates size, complexity, and error sprawl

**What.** `internal/sidecar/evals` dominates the mechanical debt metrics.
The top three non-test files by line count are all eval files, and the highest
cyclomatic function in non-test code is also eval code.

**Evidence.**
- File-size metric top three:
  `internal/sidecar/evals/discovery.go` **1007 lines**,
  `internal/sidecar/evals/anticipation_predictor.go` **974 lines**,
  `internal/sidecar/evals/gates.go` **920 lines**.
- `gocyclo`: `45 evals TryReuseBaselines internal/sidecar/evals/baseline_reuse.go:42:1`.
- Other eval complexity scores: `DetectAnomalies` complexity **24**
  (`internal/sidecar/evals/anomalies.go:107`), `mergeReports` complexity
  **23** (`internal/sidecar/evals/rewards.go:121`),
  `(DiscoveryTask).Validate` complexity **23**
  (`internal/sidecar/evals/discovery.go:155`),
  `EvaluateDiscoveryGates` complexity **21**
  (`internal/sidecar/evals/discovery.go:749`),
  `ComputeDiscoveryRewards` complexity **20**
  (`internal/sidecar/evals/discovery.go:234`).
- `fmt.Errorf` counts: **394** total in `internal`/`cmd`, **205** in
  `internal/sidecar/evals`; top files include
  `hosttask/fixture.go` **21**, `judge_config.go` **20**,
  `discovery.go` **19**, `runner.go` **12**,
  `judge_client_http.go` **12**, `judge_client_cli.go` **11**.
- `TryReuseBaselines` has **8 parameters** and a multi-value return
  `(*BaselineReuse, bool, string, error)` (`baseline_reuse.go:42`), then
  performs config hashing, manifest eligibility, episode loading, matrix
  validation, target collision checks, file copying, and manifest construction
  in one path (`baseline_reuse.go:49-190`).

**Why it is debt.** This package is the measurement stack, so every extra
branch and ad-hoc error site increases the cost of maintaining auditability.
AGENTS.md says every score must be recomputable and measurement changes must be
pre-registered. Large files and multi-concern functions make it harder to prove
that a scoring or eligibility refactor did not silently change the measurement
stack mid-run.

**Recommendation.** Treat eval complexity cleanup as an ADR-scoped refactor,
not drive-by cleanup. First split baseline reuse into domain types:
`BaselineReuseRequest`, `BaselineReuseEligibility`, `EpisodeReuseMatrix`, and
`BaselineReusePlan`; then make `TryReuseBaselines` a coordinator returning a
typed refusal instead of `(bool, string, error)`. Separately, split discovery
eval code by concern: task schema/validation, episode state extraction,
reward math, aggregate math, gate rendering. Preserve committed fixture output
and run eval package tests before and after.

---

## Medium severity

### M1 — CLI command package is accumulating inline command bodies

**What.** The CLI layer has two large command aggregator files:
`internal/cli/ghx.go` is **797 lines** and `internal/cli/sidecar.go` is
**760 lines**. `rg -c "RunE: func" internal/cli/*.go` finds **26** inline
`RunE` bodies, including **12** in `sidecar.go` and **8** in `ghx.go`.

**Evidence.**
- `internal/cli/sidecar.go:69` (`sidecar ask`), `:141` (`daemon`),
  `:175` (`report-sink`), `:198` (`evals export`), `:238` (`doctor`),
  `:287` (`tail`), `:339` (`sessions list`), `:367` (`sessions show`),
  `:429` (`sessions ledger`), `:461` (`sessions reroute`), `:517`
  (`config show`), `:546` (`config init`).
- `internal/cli/ghx.go:55`, `:91`, `:154`, `:303`, `:361`, `:407`,
  `:452`, `:487` define inline bodies for core commands.
- `sidecar ask` mixes flag reads, routing display, daemon invocation, JSON
  envelope formatting, human formatting, route output, and artifacts footer
  rendering in one body (`internal/cli/sidecar.go:69-127`).
- `config init` uses boolean mode flags and passes them to
  `runSidecarConfigInit(claudeACP, cmd.Flags().Changed("claude-exe"), claudeExe, force)`
  (`sidecar.go:546-550`), then the helper receives
  `runSidecarConfigInit(claudeACP, claudeExeSet bool, claudeExe string, force bool)`
  (`sidecar.go:557`).

**Why it is debt.** This violates "one authoritative owner per concern" at the
CLI boundary. Command declarations, flag extraction, application-service calls,
and presentation are fused. A future change to output shape or config-init
mode semantics must edit the same god-file that registers unrelated commands.
The boolean pair `claudeACP, claudeExeSet` is a control-coupling smell: callers
must know which combinations are valid.

**Recommendation.** Move command bodies into small `run*` functions with typed
request structs, one per command family. For config init, replace the boolean
pair with a `ConfigInitMode` enum/struct (`ModeAuto`, `ModePinnedACP`,
`ModeClaudeExecutable`) so invalid states are unrepresentable. For sidecar ask,
extract presentation into an `AskPresenter` that owns JSON vs. human output and
artifacts footers.

### M2 — MCP direct handlers repeat the same parse-call-marshal-error pattern

**What.** `internal/cli/serve.go` has repetitive MCP handler plumbing:
argument extraction, core call, `json.Marshal`, and
`mcp.NewToolResultError(err.Error())` are hand-written per tool.

**Evidence.**
- `serve.go` has **16** `NewToolResultError` calls and **7** `json.Marshal`
  calls.
- Direct handlers follow the same shape:
  `handleExplore` (`serve.go:236-250`), `handleRepos` (`serve.go:253-271`),
  `handleSearch` (`serve.go:274-292`), `handleRead` (`serve.go:295-331`),
  `handleTree` (`serve.go:334-349`), `handleSearchTools` (`serve.go:359-377`),
  and `handleCode` (`serve.go:381-391`).
- Example repetition: `RequireString` error -> `NewToolResultError(err.Error())`
  at `serve.go:237-239`, `:254-256`, `:275-277`, `:296-303`, `:335-337`,
  `:381-383`; core-call error -> same wrapper at `serve.go:244-247`,
  `:261-264`, `:283-289`, `:318-327`, `:342-345`, `:387-390`.

**Why it is debt.** This is copy-paste infrastructure around one concern:
mapping MCP requests/responses. Even after ADR-0034 handles failure classes,
every future tool still pays the same boilerplate tax unless the handler shape
has an owner. It also increases drift risk: `handleRecon` validates `depth`
locally (`serve.go:191-195`) while direct handlers rely on lower layers.

**Recommendation.** Introduce a tiny MCP handler adapter layer: typed argument
decoders per tool, `jsonToolResult(value any)`, and a single error mapping
function. Keep tool definitions where they are; remove per-handler marshal and
error wrapper duplication. This is orthogonal to ADR-0034 and can reuse its
structured error mapper once available.

### M3 — `internal/ghx/read.go` is a compact but control-dense core operation

**What.** The core `Read` path has cyclomatic complexity **19**
(`internal/ghx/read.go:50`) and combines repo validation, glob expansion,
GraphQL query construction, request execution, GraphQL object decoding, grep,
line extraction, map fallback, budget fallback, and output metadata mutation.
The file is not huge, but it is dense.

**Evidence.**
- `gocyclo`: `19 ghx Read internal/ghx/read.go:50:1`.
- `Read` validates and splits the repo (`read.go:55-60`), detects and expands
  globs (`read.go:62-87`), builds GraphQL aliases manually (`read.go:98-113`),
  silently converts GraphQL errors into per-file `NotFound` results
  (`read.go:119-126`), and loops through response aliases
  (`read.go:128-145`).
- `parseFileResponse` then type-asserts unstructured GraphQL maps and applies
  mode-specific output behavior (`read.go:149-199`): directory, grep, lines,
  map, budgeted map, or full content.
- `ReadOpts` is both input and output: `Globs []GlobResult` is documented as
  "Output: populated after Read()" (`read.go:44-45`).

**Why it is debt.** This conflicts with "Define proper domain models" and
"Decouple responsibilities that do not belong together." The current API
mutates the options object to return glob metadata, and the same operation owns
fetch planning, response decoding, and content shaping. That makes it harder to
extend `read` without changing agent-visible output by accident.

**Recommendation.** Split into `ReadPlanner` (repo/path/glob planning),
`ReadFetcher` (GraphQL query and raw response), and `ReadRenderer`/`ReadShaper`
(grep/lines/map/budget shaping). Return a `ReadResultSet` with `Files` and
`Globs` instead of mutating `ReadOpts`. Because this is user-visible core
behavior, preserve existing CLI/MCP JSON shape or version the change
deliberately.

### M4 — Repomap graph construction hides multiple algorithms in one loop

**What.** `buildEdges` has cyclomatic complexity **19**
(`internal/sidecar/tier2/repomap.go:224`) and assembles definition indexes,
reference edges, import edges, weight normalization, and deterministic
adjacency output in one function.

**Evidence.**
- `gocyclo`: `19 tier2 buildEdges internal/sidecar/tier2/repomap.go:224:1`.
- Definition indexing is built at `repomap.go:227-238`.
- Reference-edge weighting and dilution by number of definers is at
  `repomap.go:245-265`.
- Import resolution and weight assignment is at `repomap.go:267-283`.
- Deterministic target sorting/output is at `repomap.go:285-293`.

**Why it is debt.** This is a maintainability risk in an algorithmic core.
Reference edges and import edges are separate domain concepts but share a
mutable `weights` map inside one loop. That makes it hard to adjust one
weighting rule or add a new edge family without re-reading the whole algorithm.

**Recommendation.** Introduce an `EdgeBuilder` domain type with separate
methods for `definitionIndex`, `referenceWeights`, `importWeights`, and
`sortedAdjacency`. Keep the weight constants and deterministic ordering tests
close to the builder. This is a tractable, low-behavior-change extraction.

---

## Low severity

### L1 — `fmt.Errorf` wrapping is broad and unevenly owned

**What.** There are **394** `fmt.Errorf` sites in `internal`/`cmd`. The count is
not inherently bad, but the distribution identifies files where validation,
wrapping, and user-facing wording are mixed with domain logic.

**Evidence.**
- Top non-test files by `fmt.Errorf` count:
  `internal/sidecar/evals/hosttask/fixture.go` **21**,
  `internal/sidecar/evals/judge_config.go` **20**,
  `internal/sidecar/evals/discovery.go` **19**,
  `internal/cli/sidecar.go` **16**,
  `internal/sidecar/acp.go` **13**,
  `internal/cli/ghx.go` **13**,
  `internal/sidecar/evals/runner.go` **12**,
  `internal/sidecar/evals/judge_client_http.go` **12**,
  `internal/sidecar/evals/judge_client_cli.go` **11**,
  `internal/sidecar/daemon_worker.go` **10**.
- Package-level counts: `internal/sidecar/evals` **205**,
  `internal/sidecar` excluding evals **115**, `internal/cli` **40**,
  `internal/ghx` **18**.

**Why it is debt.** This overlaps with, but is not the same as, ADR-0034.
Even when failure classes are typed, validation-heavy packages still need a
single owner for message construction and refusal reasons. Otherwise
operator-facing messages, test fixture validation, and domain state transitions
continue to drift as prose strings.

**Recommendation.** Do not mass-replace `fmt.Errorf`. Instead, when touching a
high-count file, extract typed validators/refusal builders for that domain:
fixture validation, judge config validation, discovery task validation, and
CLI config-init validation. Treat a reduced `fmt.Errorf` footprint as a
secondary metric after the domain model exists.

### L2 — Long parameter lists persist where option structs already exist nearby

**What.** The codebase generally uses request/option structs, but the remaining
long-parameter functions are exactly in orchestration seams.

**Evidence.**
- `TryReuseBaselines(runDir, priorRunDir string, cfg RunConfig, tasks []Task,
  taskDir string, plannedTrials int, identity AgentIdentity, now time.Time)`
  has **8 parameters** and a 4-value return (`baseline_reuse.go:42`).
- `askWithTurnRunner(ctx context.Context, cfg Config, req AskRequest,
  runner TurnRunner, preflight bool, route *RouteDecision)` has **6
  parameters** and mode-control booleans/pointers (`runtime.go:273`).
- `(*Service).materialize(ctx context.Context, url string, req SnapshotRequest,
  sha, dir string, strategy CloneStrategy)` has **6 parameters**
  (`internal/sidecar/tier2/service.go:295`).
- `runSidecarConfigInit(claudeACP, claudeExeSet bool, claudeExe string,
  force bool)` has **4 parameters**, two of them control booleans
  (`internal/cli/sidecar.go:557`).

**Why it is debt.** AGENTS.md calls for declarative APIs and proper domain
models. Long parameter lists with booleans encode domain states in call-site
ordering rather than named types. They are easy to misuse and hard for future
agents to audit.

**Recommendation.** Convert each to a domain request struct at the next touch:
`BaselineReuseRequest`, `AskWorkflowOptions`, `MaterializeRequest`, and
`ConfigInitRequest`. Where booleans represent mutually exclusive modes, use an
enum-like type instead of multiple booleans.

---

## Recommended sequence (top refactors, ordered)

1. **Extract the sidecar ask workflow from `askWithTurnRunner`.**
   Payoff: highest reduction in runtime comprehension cost; separates turn
   execution, recovery, report retry, persistence, and telemetry. Effort:
   1-2 days with focused tests, no behavior change intended.

2. **ADR-scope baseline reuse into typed eligibility + copy plan objects.**
   Payoff: attacks the highest non-test cyclomatic score (45) while preserving
   eval auditability. Effort: 1-2 days plus ADR update and eval-package tests.

3. **Split CLI command bodies into typed run functions and presenters.**
   Payoff: reduces churn in 1557 lines of CLI aggregator files and makes future
   command changes local. Effort: 1 day for `sidecar ask/config init`, then
   incremental command-family cleanup.

4. **Add an MCP handler adapter for direct tools.**
   Payoff: removes repeated parse/call/marshal/error plumbing across 7 handlers
   and gives ADR-0034 a single future integration point. Effort: half day to
   one day, mostly tests around existing JSON output.

5. **Decompose `Read` into planning/fetching/shaping.**
   Payoff: makes the core read surface easier to extend without mutating
   `ReadOpts` as output state. Effort: 1-2 days because CLI/MCP JSON behavior
   must be pinned carefully.

6. **Extract repomap edge construction into an `EdgeBuilder`.**
   Payoff: isolates algorithm families and makes new edge signals cheaper to
   add. Effort: half day to one day if existing deterministic tests are kept
   green.

## Method / auditability

Exact commands run from the worktree root:

```bash
sed -n '1,240p' docs/audits/architecture-2026-07-07.md
sed -n '1,260p' docs/audits/failure-class-inventory-2026-07-07.md
git rev-parse --show-toplevel
git status --short
find internal cmd -name '*.go' ! -name '*_test.go' | xargs wc -l | sort -rn | head -30
go run github.com/fzipp/gocyclo/cmd/gocyclo@latest -top 40 internal cmd
go run github.com/fzipp/gocyclo/cmd/gocyclo@latest -top 80 internal cmd | rg -v '_test\.go|/testdata/' | head -40
rg -n "fmt\.Errorf" internal cmd | wc -l
rg -n "fmt\.Errorf" internal cmd | cut -d: -f1 | sort | uniq -c | sort -rn | head -30
rg -n "fmt\.Errorf" internal/sidecar/evals | wc -l
rg -n "fmt\.Errorf" internal/sidecar | rg -v '^internal/sidecar/evals/' | wc -l
rg -n "fmt\.Errorf" internal/cli | wc -l
rg -n "fmt\.Errorf" internal/ghx | wc -l
rg -n "func .*\\([^)]*,[^)]*,[^)]*,[^)]*,[^)]*" internal cmd
rg -n "func .*\\([^)]*,[^)]*,[^)]*,[^)]*,[^)]*,[^)]*" internal cmd
rg -n "\\bbool\\b" internal cmd | cut -d: -f1 | sort | uniq -c | sort -rn | head -40
rg -n "RunE: func" internal/cli/*.go
rg -c "RunE: func" internal/cli/*.go
rg -n "NewToolResultError|json\\.Marshal|RequireString|GetString|GetInt|GetBool" internal/cli/serve.go
rg -c "NewToolResultError" internal/cli/serve.go internal/sidecar/reportsink.go
rg -c "json\\.Marshal" internal/cli/serve.go
nl -ba internal/sidecar/runtime.go | sed -n '240,690p'
nl -ba internal/sidecar/evals/baseline_reuse.go | sed -n '1,190p'
nl -ba internal/cli/ghx.go | sed -n '1,220p'
nl -ba internal/cli/sidecar.go | sed -n '1,760p'
nl -ba internal/cli/serve.go | sed -n '120,335p'
nl -ba internal/ghx/read.go | sed -n '1,220p'
nl -ba internal/ghx/inspect.go | sed -n '220,360p'
nl -ba internal/sidecar/evals/discovery.go | sed -n '130,290p'
nl -ba internal/sidecar/evals/discovery.go | sed -n '730,820p'
nl -ba internal/sidecar/tier2/repomap.go | sed -n '200,310p'
nl -ba internal/sidecar/tier2/service.go | sed -n '260,340p'
```

No code was modified. The only file created by this audit is
`docs/audits/complexity-hotspots-2026-07-07.md`.
