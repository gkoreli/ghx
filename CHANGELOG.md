# Changelog

All notable changes to `ghx` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Where a change carries proof, the entry links the ADR that decided it or the
committed eval verdict that measured it — the artifacts are the evidence, not the
prose here.

## [Unreleased]

### Changed

- CI: `actions/setup-go` v5 → v6 and `goreleaser/goreleaser-action` v6 → v7
  (GitHub's Node 20 deprecation forced the old versions onto Node 24).

## [2.9.1] — 2026-08-21

Three parallel capability tracks landed from evidence-grounded research
(`docs/research/`) through ADR proposals to implementation — each branch built
and verified with `go test ./...` green.

### Added

- **Cheap-backend guardrail for formal gate runs (ADR-0038)** — live eval
  runs fail fast when the resolved ACP agent matches an expensive/shared-quota
  backend (default `claude*`), including detection via wrapper-script content;
  deliberate override via `GHX_EVAL_ALLOW_EXPENSIVE_BACKEND=formal-run`;
  operators reclassify via `GHX_EVAL_EXPENSIVE_BACKENDS`. Exists because the
  2026-07-03/04 formal run silently burned the main agent's Claude
  subscription. Episode artifacts already recorded backend identity
  (`AgentIdentity`); new `docs/evals/README.md` documents the convention.
- **Visible ledger eviction + session snapshot identity (ADR-0037 M-1/M-2)** —
  evidence-ledger truncation under the 1500-char prompt budget now emits a
  visible note instead of silent loss; `SessionMeta`/`Ledger` gain
  `Commit`/`Branch` snapshot stamps (first-write-wins, never fabricated) so
  reconnaissance evidence states which repo ref it was gathered against.
- **Report contract validation (ADR-0039)** — `Report.schemaVersion` (stamped
  on all three report paths: resolved, BLOCKED wrap-up, warn fallback) keys
  contract evolution; deterministic flag-only `CheckReportBounds` enforces the
  persona compactness contract (>2000 chars, >5 files, >2-sentence answer)
  with violations surfaced on `TurnResult.ReportBoundViolations` — never
  altering or rejecting reports.
- **Research artifacts** (`docs/research/`): 001-cheap-sidecar-backend,
  002-persistent-sidecar-session-memory, 003-report-contract-context-boundary.
- **ADR proposals accepted on merge**:
  [0037](docs/adr/0037-persistent-evidence-shaped-session-memory.md),
  [0038](docs/adr/0038-cheap-backend-governance.md),
  [0039](docs/adr/0039-report-contract-validation.md).

- **Tier-2 runtime-owned escalation (ADR-0024.4)** — the sidecar-decided half
  of M7/B9 completes: `ghx tier2 observe --signal <id>` lets the agent declare
  the three judgment-based escalation signals, accepted only when the observe
  invocation is recorded in the turn's traces (policy stays recomputable);
  pre-hoc grant gating via `GHX_TIER2_ALLOWED_BACKENDS` refuses ungranted
  `local:*` commands before any clone work with an affordance hint; new G6
  policy-precision eval gate (escalated correctness ≥ 0.90 × overall,
  additive, never a thesis input); `ghx cache ls|clean` surfaces over the
  existing TTL+LRU eviction.
- **Eval corpus discrimination refresh R1–R4 (ADR-0016.13)** — four
  replacement fixtures (werkzeug-delegation, gin-route-conflict,
  hono-smartrouter-fallback, gjson-engine-selection) target the corpus's 2/6
  discrimination ceiling (TRUST H7); unit-validated with
  `TestADR0016_13TaskFixturesValid`; live closed-book canaries are PRELIMINARY
  pending runs.
- **Research artifacts** (`docs/research/`): a4-error-affordances,
  m7-tier2-escalation, h7-corpus-discrimination — evidence-grounded with
  cross-references and per-file evidence appendices.
- **ADR proposals**: [0034.1](docs/adr/0034.1-failure-class-completion-across-frontends.md),
  [0024.4](docs/adr/0024.4-runtime-owned-escalation.md),
  [0016.13](docs/adr/0016.13-eval-corpus-discrimination-refresh.md).

### Changed

- **Failure-class completion across frontends (ADR-0034.1)** — `ghx sidecar
  ask` maps the report outcome class to semantic exit codes (bad-input BLOCKED
  → 2, upstream → 3, no-evidence → 1, answered → 0) instead of always exit 0;
  `inspect` on a nonexistent repo now exits 3 like explore/read instead of a
  silent 1; the persona requires cited exit codes to be trace-captured, never
  narrated from memory (persona golden hash updated deliberately).
- NORTH_STAR milestone rows updated: A3 shipped, A4 mostly shipped, C7 notes
  H7 fixture landing.

## [2.9.0] — 2026-07-07

Agent-experience hardening from the v2.8.0 dogfood: CLI failure exit codes are
now classified in core and consistent across commands, the resolved commit SHA
is surfaced to agents, and eval `Locations` are trustworthy for the ghx-sidecar
profile.

### Changed

- **The zero-knowledge recon skill now ships in the default install** —
  `.claude-plugin/plugin.json` now lists `./skills/ghx-recon` **first**, so an
  uninformed agent lands on the 47-line delegation surface (`ghx serve --recon` /
  `ghx skill --recon`) instead of only the heavy power-user CLI skill that the
  north star wants hidden. Additive — the `ghx`/`ghx-mcp` power-user skills are
  unchanged. Ships the product audit's #1 finding
  ([PA-0001](docs/product-audit/PA-0001-ghx-product-audit.md) H1, cross-family
  cleared in [PA-0001.10](docs/product-audit/PA-0001.10-cross-family-adjudication.md);
  workstream B2). Making recon the *default* of `ghx skill` is the deferred,
  non-additive half.
- **The resolved commit SHA is now visible to agents** — `ghx code` type stubs
  and the code-mode result advertise `snapshot: { repo: { owner; name }; sha }`
  on `explore`/`read`, so an agent can read *which commit* the reconnaissance
  saw (the [ADR-0036](docs/adr/0036-target-architecture-runner-port-and-boundaries.md)
  B2 `Snapshot` was computed but not surfaced); `ghx.Repo` JSON keys are
  lowercased for consistent serialization.
- **Failure classification moved into core** — `internal/ghx` now owns a
  `FailureClass` taxonomy and a typed `ghx.Error{Class,Err,Hint}`; the upstream
  GitHub/gh sub-classification (rate-limit vs auth vs not-found) is derived once,
  at the source, from the structured HTTP status / GraphQL error type rather than
  re-parsed English error strings. The CLI maps class → exit code + affordance
  and the duplicated substring table is deleted; exit codes and stderr stay
  byte-identical for every already-correct case
  ([ADR-0034](docs/adr/0034-failure-class-model.md); the MCP and sidecar
  mappings are phases 3–4, not yet built).

### Fixed

- **`ghx code` exits 2 on a transpile/parse/run failure** (was 0 in some paths)
  and no longer doubles the `transpile:` error prefix.
- **`ghx sidecar ask --depth <invalid>` is rejected with exit 2** naming the
  valid set (`cheap|normal|deep`) before any daemon call, instead of silently
  coercing — matching the MCP recon path.
- **CLI exit codes now match the documented taxonomy on three dogfooded edges**
  (`docs/dogfood/FRICTION.md`, 2026-07-07): `ghx read owner/repo <path>` on a
  nonexistent repo/ref returns exit 3 (was a silent exit 0), and returns exit 1
  when the repo resolves but no requested path does — a scripting agent checking
  `$?` can now tell an empty 404 from success (it still prints the per-file
  `(not found)` lines). `explore`/`read`/`tree`/`grep` all classify a malformed
  `owner/repo` slug as bad input (exit 2), and live 401/403 name the
  `gh auth login` / GH_TOKEN fix
  ([ADR-0034](docs/adr/0034-failure-class-model.md) phases 1–2).
- **Eval `Locations` are trustworthy for the ghx-sidecar profile** — the
  duplicated `ToolCallTrace`/`ToolStatusTransition` types (SAF runtime + SAFE)
  are unified onto one canonical owner, deleting the hand-written converter that
  silently dropped `Locations`, and the sidecar's execute-driven ghx recon now
  derives path-scope `Locations` from the `ghx` argv when ACP reports none
  (additive; never overrides real ACP locations). Closes TRUST hole H9. Type
  dedup is byte-identical for committed artifacts; the capture change was
  pre-registered and measured (1682 execute traces gain path-scope across the
  committed corpus, 0 host-task grader verdicts change — execute-kind traces
  never reach the R6 path-scope rule)
  ([ADR-0032.2](docs/adr/0032.2-tooltrace-unification-locations-flow.md)).

## [2.8.0] — 2026-07-07

Architecture hardening (ADR-0035 cleanup + ADR-0036 target architecture, incl.
the config-selectable Runner port) plus main-agent ergonomics — the invisible
half is a large behavior-preserving refactor that makes ghx faster to evolve.

### Added

- **Daemon runtime tunables are configurable** — `daemonWorkerIdleTTLMinutes`
  and `daemonMaxConcurrent` in `~/.ghx/config.json` set the warm-worker idle TTL
  (default 30m) and cross-session turn concurrency (default 4). Unset preserves
  today's behavior; invalid values are rejected; both participate in the daemon
  config digest so a running daemon stale-replaces on change
  ([ADR-0030](docs/adr/0030-always-on-daemon.md) follow-up).
- **`ghx read --line-range` alias for `--lines`** — agents that type
  `--line-range` (a real mined failure in production traces) now get identical
  behavior; hidden and conflict-checked (workstream A2).
- **`read`/`explore` now surface the resolved commit SHA** — results carry a
  `Snapshot{Repo,SHA}` recording which commit the reconnaissance actually read,
  so evidence is reproducible and two reads in one investigation can be checked
  against a moving `HEAD`
  ([ADR-0036](docs/adr/0036-target-architecture-runner-port-and-boundaries.md) B2).

### Changed

- **`--path` on `explore`/`search` teaches the fix** — an unknown `--path` flag
  now names the correct invocation (`ghx explore owner/repo src` /
  `ghx grep owner/repo PATTERN --path PATH`) instead of a bare error
  (workstream A4).
- **`ghx grep -i/--ignore-case` and `ghx tree --path` are now accepted** — `grep`
  takes the grep/rg muscle-memory `-i` (a documented no-op: GitHub code search is
  always case-insensitive), and `tree` accepts `--path <subtree>` as an alias for
  its positional path, unifying the `--path` grammar across `grep`/`inspect`/`tree`
  (surfaced dogfooding the [PA-0002](docs/product-audit/PA-0002-absorb-codebase-memory-mcp.md)
  competitive recon; workstream A).
- **Sidecar daemon is now supervisor-hardened** — a panic in one turn is
  isolated to that request instead of taking down the daemon and all warm
  sessions; a disconnected/cancelled client actually cancels its turn; and the
  daemon drains active turns gracefully on shutdown
  ([ADR-0036](docs/adr/0036-target-architecture-runner-port-and-boundaries.md)
  A1; implements ADR-0030 D6).
- **Internal architecture hardening** (behavior-preserving; no CLI/report/eval
  change): a `ghx.Repo` value object and a GitHub-client seam make core recon
  offline-testable; the sidecar turn engine is consolidated behind a
  config-selectable **Runner port** (`runner.kind` — the "below-ACP" seam so the
  runner is replaceable) with typed failure classes; `Depth`/`Tier`/`Backend`
  are value types; the sidecar tool set has a single registry. Guided by
  [ADR-0035](docs/adr/0035-architecture-hardening-refactor-sequence.md) and
  [ADR-0036](docs/adr/0036-target-architecture-runner-port-and-boundaries.md),
  synthesized from the `docs/audits/` architecture audits.

### Fixed

- **`ghx tree`/`explore`/`read` no longer panic or misreport a malformed repo
  slug.** A slug missing `owner/repo` (e.g. `ghx tree noslash`) previously
  panicked on an unchecked index; it now returns a clean bad-input error with
  exit code 2, unified behind a single `ghx.Repo` validator
  ([ADR-0035](docs/adr/0035-architecture-hardening-refactor-sequence.md) T1.1).

## [2.7.0] — 2026-07-07

Ergonomics for the main-agent consumer plus architecture and eval-trust
hardening.

### Added

- **CLI errors name the fix-it invocation** (workstream A4). Upstream failures
  (GitHub 404s, `gh` auth, rate limits) now append a `→` recovery line naming the
  exact command to run next — an error is an affordance, not a dead end. Exit-code
  semantics (0/1/2/3) are unchanged
  ([ADR-0028.1](docs/adr/0028.1-cli-ergonomics-batch.md)).
- **Depth dial on the MCP recon tool** (NORTH_STAR capability 3). The `recon`
  tool now takes an optional `depth` — `cheap|normal|deep`, default `normal` —
  forwarded through the same path as `ask --depth`, so a main agent can steer the
  reconnaissance budget without touching config. Invalid values are rejected,
  naming the valid set.
- **Host-task eval corpus** — 6 hand-authored, provenance-verified fixtures
  (each cloned at its pinned SHA and confirmed fails-before / passes-after) with
  memorization-canary enforcement at load. Completes ADR-0032.1's build (slices
  S1–S4) ([ADR-0032.1](docs/adr/0032.1-host-task-evals-decision.md)).

### Fixed

- **`ToolCallTrace.Locations` is now populated from ACP tool-call notifications.**
  The field was declared but never written, so the host-task R6 path-scope
  detector ran on always-empty data and reported false-clean — a
  visibility/truthfulness gap closed ahead of any host-task run (ADR-0032.1 S2).

### Changed

- **Zero-CLI recon consumer surface tightened** (workstream B2). The recon skill
  and MCP tool now describe ghx purely as a delegation service — no CLI flag
  grammar leaks into the main agent's context; the tool description spells out the
  evidence-report contract and that follow-ups auto-route.
- **Internal architecture** (from the [2026-07-07 audit](docs/audits/architecture-2026-07-07.md)):
  removed the dead `callTool` codemode compat binding so `codemode.<tool>()` is the
  sole API (ADR-0010); decomposed the 1012-line `acp.go` into concern-cohesive
  files (pure relocation, byte-identical).

### Docs

- [ADR-0034](docs/adr/0034-failure-class-model.md) **(proposed)** — unify the
  failure-class taxonomy in core so CLI exit codes, MCP tool errors, and sidecar
  reports map from one source of truth (audit M2). Architecture audit report;
  trust-ledger H3 re-verified from committed artifacts; CHANGELOG backfilled for
  v2.3.0–v2.5.0.

## [2.6.0] — 2026-07-06

Visibility and trust hardening on top of the always-on sidecar. You can now
watch a turn think in real time, credentials never leak into the stderr tee, a
stale daemon can no longer answer for a newer binary, and the host-task eval arm
that measures the sidecar against an unaided agent is wired end to end.

> Note: the `2.3.0`–`2.5.0` entries are being backfilled from their release
> commits; this section records the changes shipped in `2.6.0`.

### Added

- **`ghx sidecar tail [session]` — watch a turn think.** A human-readable live
  view of a turn's `live.jsonl`: text, thoughts, and tool calls stream as they
  happen, with `--follow` to tail an in-flight turn. The end-of-turn
  `traces.jsonl` OTLP record is unchanged; this is the low-latency companion for
  watching, not auditing ([ADR-0022.1](docs/adr/0022.1-shared-visibility.md)).
- **Host-task eval arm-B (ADR-0032.1 S3).** The host-task harness now runs a live
  agent episode against a sidecar-as-MCP recon server (`serve --recon`), with
  host-only filesystem writes, compliance detectors
  (`host_external_exploration`, `host_rate_limited`, `recon_tool_unavailable`,
  `host_execute_outside_workspace`), and a frozen host-identity manifest kept
  outside `identityHashes` so baseline reuse is unaffected
  ([ADR-0032.1](docs/adr/0032.1-host-task-eval.md)).

### Fixed

- **Credentials no longer leak into the agent stderr tee** (security, codex audit
  HIGH). The per-session `agent-stderr.log` now redacts the exact values of
  allowlisted secret env vars while keeping non-secret context (region, etc.)
  readable.
- **Daemon executable-identity handshake.** The daemon health check now compares
  executable paths (symlink-normalized), so a stale daemon from an older or
  different binary is detected and restarted instead of silently answering. The
  daemon also refuses to spawn from a `.test` binary, preventing lingering
  duplicate daemons during test runs.

## [2.5.0] — 2026-07-06

The release that completes the sidecar's ambient plumbing: agents can now
authenticate as themselves through the sidecar, enterprise Claude installs work
without wrapper-script friction, every turn writes a live event log as it
happens, and the measurement stack gains the at_least(n) gate forms that make
paired-cell comparisons sound.

### Added

- **Client auth-env passthrough** ([ADR-0033.1](docs/adr/0033.1-client-auth-env-passthrough.md)).
  The sidecar now forwards the calling shell's auth environment to the agent, so
  an agent that asks authenticates with its own credentials rather than the
  sidecar host's. No wrapper scripts, no key copying.
- **`agentSettingSources` opt-in** ([ADR-0033.2](docs/adr/0033.2-agent-setting-sources-optin.md)).
  Enterprise Claude Code installs (toolbox, wrapper-script launchers) can declare
  their settings sources at config time; the sidecar passes them through so the
  agent boots with the right profile without a `CLAUDE_CODE_EXECUTABLE` detour.
- **Real-time `live.jsonl` turn log** ([ADR-0022.1](docs/adr/0022.1-live-turn-log.md)).
  Every sidecar turn now appends events — text chunks, tool calls, thoughts — to
  `live.jsonl` as they happen, giving `tail -f` a human-readable window into an
  in-flight turn. The end-of-turn `traces.jsonl` OTLP record is unchanged.
- **`config init --claude-exe`** — pins the ACP adapter to a specific Claude Code
  binary (toolbox installs, side-by-side versions). Companion to the
  `agentSettingSources` opt-in.
- **Tier-2 backend availability in `sidecar doctor`** — reports which tier-2
  backends are available (embedded repomap vs optional external binaries like
  ast-grep), so setup problems surface before a real ask hits them.
- **Host-task eval S1 + S2** ([ADR-0032.1](docs/adr/0032.1-host-task-evals-decision.md)).
  The host-task harness gains a workspace provisioner and outcome grader (S1), plus
  write-policy enforcement and exploration/engineering attribution (S2), building
  the scaffold for the arm-B comparison shipped in 2.6.0.
- **`nextReads` concrete-paths contract + lenient normalizer**
  ([ADR-0031.2](docs/adr/0031.2-nextreads-contract-revision.md)). The sidecar
  persona now requires concrete file paths in `nextReads` hints; a lenient
  normalizer repairs common deviations rather than rejecting them hard.
- **ADR-0025.2 residuals closed** — G1 paired-cell and G3 char-ceiling
  `at_least(n)` gate forms wired; `GHX_EVAL_GATE_AT_LEAST` env override for CI
  ([ADR-0025.2](docs/adr/0025.2-gate-reducers-stability.md)).

### Fixed

- **Enterprise-wrapper hang resolved** — `settingSources: []` combined with a
  `CLAUDE_CODE_EXECUTABLE` wrapper stalled the agent on boot; documented and
  eliminated by the `agentSettingSources` opt-in path.

## [2.4.2] — 2026-07-06

A usability patch: the sidecar now works out of the box without manual adapter
configuration, authentication errors teach the fix, SDK noise is filtered from
surfaced diagnostics, and `ask --local` lets the calling agent grant tier-2
local backends from the CLI without touching config.

### Added

- **`ask --local`** — grants tier-2 local backends (repomap, ast-grep) from the
  CLI for a single ask, without requiring a config change. Agents can escalate
  opportunistically when local tools are available
  ([ADR-0028.1](docs/adr/0028.1-cli-ergonomics-batch.md)).
- **Zero-config quickstart** documented in README — `ghx sidecar ask` works
  immediately after `ghx sidecar config init --claude-acp` with the pinned
  adapter default; no extra configuration step.

### Fixed

- **Zero-config adapter default** — the ACP adapter is now the pinned default;
  first-time users no longer need to set the adapter field manually after `config
  init`.
- **Auth hint leads with the adapter's own login flow** — when the agent reports
  an auth failure the surfaced hint now shows the adapter-specific login command
  first, not a generic credential hint.
- **Benign SDK boilerplate filtered from surfaced stderr** — low-signal SDK
  startup noise no longer appears in the sidecar's diagnostics output, making
  real errors easier to spot.
- **Anticipation miner test is hermetic** — frozen eval artifacts are never
  rewritten by suite runs; provenance paths are portable across machines
  ([ADR-0031.1](docs/adr/0031.1-anticipation-v1-decision.md)).

## [2.4.1] — 2026-07-06

A hardening patch focused on agent-facing diagnostics and measurement-stack
integrity: the sidecar now surfaces loud, actionable errors instead of silent
failures, session working directories are neutral by default, and the eval
framework can annotate structurally fragile gates.

### Added

- **Loud sidecar diagnostics, neutral session cwd, agent provenance**
  ([ADR-0033](docs/adr/0033-sidecar-out-of-box-agent-config.md)). The sidecar
  surfaces loud, structured error messages when the agent cannot start or
  authenticate; session working directories default to a neutral location so agent
  file writes go somewhere predictable; provenance metadata is recorded per
  session.
- **`sidecar doctor --live`** — a live connectivity check that verifies the daemon
  can actually reach the configured adapter and report-sink, not just that config
  parses.
- **`at_least(n)` gate reducer + fragility annotation**
  ([ADR-0025.2](docs/adr/0025.2-gate-reducers-stability.md)). Gates that pass by
  a slim margin can now be flagged FRAGILE in the verdict, so readers know which
  thresholds to watch. A `FRAGILE` verdict is still a pass; it is an honest signal
  about headroom.

### Fixed

- **OTLP logs and metrics UTF-8 sanitization** — invalid UTF-8 sequences in log
  and metric strings no longer drop the span; a shared proto-string sanitizer
  covers all OTLP exporters (extends the trace-level fix from 2.4.0).

## [2.4.0] — 2026-07-06

The release that makes tier escalation deliberate and observable: every
exploration path now runs through an escalation policy engine that records its
decision, session routing is live with automatic reroute recovery, and the sidecar
persona is revised a third time with explicit tier-2 and discovery citation
doctrine. The measurement stack also gains real-token cost accounting alongside
the earlier chars/4 estimate.

### Added

- **Escalation policy engine** ([ADR-0024.2](docs/adr/0024.2-escalation-policy.md)).
  Every exploration path now evaluates a policy before escalating to tier-2 tools;
  decisions are recorded in `tier-decisions.jsonl` per session so escalation is
  auditable, not implicit.
- **Session routing cascade + reroute recovery**
  ([ADR-0030.1](docs/adr/0030.1-session-routing.md)). The always-on daemon routes
  asks through a deterministic R1–R5 cascade; a failed route recovers via reroute
  rather than erroring, and route decisions are observable in the session log.
- **Persona revision 3 — tier-2 doctrine + discovery read-then-cite**
  ([ADR-0029.2](docs/adr/0029.2-persona-revision-3-tier2-discovery-citations.md)).
  The sidecar persona now carries explicit guidance on when to escalate to tier-2
  tools and requires that discovery-mode answers cite the repos/files read before
  making a claim.
- **Discovery e2e episode runner** — a reusable eval driver for discovery-class
  tasks replaces the throwaway spot driver; discovery episodes are now first-class
  eval citizens ([ADR-0019.2](docs/adr/0019.2-discovery-eval-tasks.md)).
- **Real-token SPT accounting**
  ([ADR-0016.11](docs/adr/0016.11-real-token-accounting.md)). Eval runs now report
  real token counts and costs alongside the chars/4 estimate used previously, with
  TRUST H5 promoted to the trust ledger.
- **M7 tier-2 slices 3 + 4 — ast-grep and repomap absorbed**
  ([ADR-0024.1](docs/adr/0024.1-escalation-tiers-decision.md)). `local:ast-grep`
  and `local:repomap` are now first-class tier-2 backends, selectable alongside the
  snapshot/codemap backends shipped in 2.3.0.

### Fixed

- **OTLP trace UTF-8 sanitization** — invalid UTF-8 sequences in span attributes
  no longer drop the span; the exporter sanitizes before serializing.
- **Artifact names unique across skills** — release artifacts no longer collide
  when multiple skills ship binaries with the same base name; brew formula updated
  accordingly.
- **Daemon config digest includes resolved report-sink exe** — the daemon
  correctly detects a changed sink binary and restarts, rather than serving stale
  config (integration audit F1).

### Changed

- **Persona revision 3** tightens tier-2 escalation guidance and adds
  discovery-mode citation discipline on top of the inspect-first posture introduced
  in revision 2 ([ADR-0029.2](docs/adr/0029.2-persona-revision-3-tier2-discovery-citations.md)).

## [2.3.0] — 2026-07-06

The release that operationalizes the always-on daemon, closes the tier-2
reconnaissance gap, and hardens the measurement stack against the biggest
remaining threats to its validity. The sidecar daemon now runs warm in the
background; `ghx inspect` gives agents a one-shot budgeted concern probe; tier-2
gains its first two absorbed backends (snapshot substrate + codemap); discovery
eval tasks are wired end-to-end; and the judge evaluation rail ships its first
live cross-family scores.

### Added

- **Always-on sidecar daemon** ([ADR-0030](docs/adr/0030-always-on-daemon.md)).
  The daemon runs warm in the background, holds a session registry, and accepts
  socket IPC from the CLI — so the first ask of a session does not pay a cold-start
  penalty. `ghx sidecar daemon start/stop/status` manages the lifecycle.
- **`ghx inspect`** ([ADR-0028.2](docs/adr/0028.2-ghx-inspect.md)). A one-shot
  budgeted concern-inspection command: given a concern, ghx reads into the relevant
  parts of a repo and returns a focused answer within the budget. Designed for
  agents that need a quick targeted probe, not a full exploration.
- **Tier-2 snapshot substrate + codemap absorbed**
  ([ADR-0024.1](docs/adr/0024.1-escalation-tiers-decision.md)). The first two M7
  tier-2 items land: a local snapshot/cache substrate that backs tier-2 reads
  without re-fetching, and `local:codemap` absorbed as a first-class tier-2
  backend. (`local:ast-grep` and `local:repomap` follow in 2.4.0.)
- **Discovery eval class** ([ADR-0019.2](docs/adr/0019.2-discovery-eval-tasks.md)).
  Discovery-mode tasks — "which repos/libraries do X" — are now a first-class eval
  class with their own task harness, scorer, and gates, separate from the
  repo-scoped exploration class.
- **Judge evaluation CLI rail** ([ADR-0023.1](docs/adr/0023.1-judge-scorer-decision.md)).
  A `claude-cli` secondary transport runs cross-family judge evaluations at k=1;
  first live scores are PRELIMINARY (same-family bias under investigation as TRUST
  H1). The full judge sweep of 144 episodes completes within this release window.
- **SFT training record exporter**
  ([ADR-0017.1](docs/adr/0017.1-training-data-exports-decision.md)). Committed
  episodes can now be exported as training JSONL for supervised fine-tuning
  pipelines.
- **Persona revision 2 — inspect-first exploration**
  ([ADR-0029.1](docs/adr/0029.1-persona-revision-2-inspect-first.md)). The sidecar
  persona is revised to lead with `ghx inspect` before committing to a full
  exploration, reducing unnecessary deep reads on shallow questions.
- **Session routing pre-registration** ([ADR-0030.1](docs/adr/0030.1-session-routing.md)).
  The deterministic routing cascade is pre-registered and accepted; the live
  implementation ships in 2.4.0.
- **ADR-0020.2 resume steering** ([ADR-0020.2](docs/adr/0020.2-resume-steering.md)).
  `LoadSession` now carries the full session-options meta so resumed turns receive
  correct steering context (TRUST H8 fixed, live-verified).
- **Trace-capture completeness audit** ([ADR-0016.10](docs/adr/0016.10-trace-capture-completeness.md)).
  A raw-SDK audit, comparator, and `trace_capture_gap` anomaly detector verify
  that the OTLP trace pipeline captures every span the SDK emits (TRUST H3).

### Changed

- **Trust ledger elevated to tracked workstream** — `TRUST.md` is now a
  first-class north-star workstream tracking C7/C8 open-questions; the
  visibility/truthfulness tenet is binding on all agents.
- **Embedded skills refreshed** — recon skill and classic CLI skill updated with
  sidecar-first framing, verified command surface, and the measured map claim.

### Fixed

- **Closed-book memorization confound ruled out** (TRUST H2). A closed-book probe
  harness (agents answer without repo access) shows 0.067 vs open-book 0.926,
  refuting the memorization confound for this corpus
  ([ADR-0016.9](docs/adr/0016.9-memorization-confound-audit.md)).

## [2.2.0] — 2026-07-06

The release that turns the sidecar from a working prototype into something you
can hand to another agent and trust. The headline is resilience — an exploration
is never silently lost — plus one-command setup, discovery without a repo, a
built-in trace viewer, an artifacts pointer on every answer, a batch of
agent-facing CLI ergonomics, and the eval machinery that lets the whole thing
carry a citable verdict.

### Added

- **Sidecar runtime resilience — never lose an exploration.** A liveness watchdog
  and wrap-up recovery keep a turn alive through slow or stalled model output, and
  a failed turn still writes its artifacts and returns a partial report instead of
  vanishing. Reports are now evidence-required at submission time: an answer with
  no traceable evidence is rejected before it can reach you
  ([ADR-0027](docs/adr/0027-runtime-resilience.md)).
- **One-command setup.** `ghx sidecar config init --claude-acp` writes a working
  `~/.ghx/config.json` using the claude-agent-acp adapter — setup is the hardest
  step, so it is one command. Re-running against an existing config shows a field
  diff and refuses to overwrite without `--force`. `ghx sidecar doctor` now also
  verifies that the report-sink MCP binary matches the running ghx version and
  fails loudly with fix-it text, since a stale sink would silently degrade
  structured reports to a text fallback
  ([ADR-0019](docs/adr/0019-sidecar-adoption-zero-cli-surface.md) D4).
- **Discovery tier — `--repo` is now optional scope.** With `--repo`: repo-scoped
  reconnaissance as before. Without it: discovery — "which repos/libraries do X" —
  where the sidecar sweeps GitHub for candidates and reads into the top ones before
  claiming anything, deriving a resumable session slug from the question
  ([ADR-0019.1](docs/adr/0019.1-discovery-tier.md)).
- **`ghx sidecar view` — a local trace UI in one command.** Absorbs the
  [otel-desktop-viewer](https://github.com/CtrlSpice/otel-desktop-viewer) replay
  recipe: it starts the viewer and loads a session's `traces.jsonl` (plus
  logs/metrics when present). No argument replays the most recent session;
  `--list` shows sessions with turn/report counts; `--port` sets the UI port.
  Everything it renders is the same file you can read by hand
  ([ADR-0026.1](docs/adr/0026.1-sidecar-view-absorption.md)). Requires the viewer
  on PATH.
- **Artifacts pointer in every response.** Every ask answer — human output,
  `--json`, or the MCP recon tool — ends with an `artifacts:` pointer naming the
  session directory and the ask's root trace ID, so the calling agent never has to
  guess where the audit trail lives. The `--json` envelope carries it as
  `{"report": {...}, "artifacts": {"sessionDir": "...", "traceId": "..."}}`.
- **`ghx grep`** — a grep-like repo search front door (`ghx grep <owner/repo>
  "pat" --path <dir> --limit N`), alongside the existing `search`, as part of the
  CLI ergonomics batch ([ADR-0028.1](docs/adr/0028.1-cli-ergonomics-batch.md)).
- **Judge gold-set labeling protocol and packet generator.** A deterministic
  generator (`go run ./docs/evals/judge-goldset/gen`) assembles the candidate pool
  from the committed gate runs, applies the same D7 anomaly pre-filter the judge
  runner uses, and writes profile-blind labeling packets — so human gold labels and
  judge scores are produced from the same evidence on the same rubric. The written
  labeling protocol is committed alongside
  ([ADR-0023.1](docs/adr/0023.1-judge-scorer-decision.md);
  `docs/evals/judge-goldset/PROTOCOL.md`).
- **Real cross-family judge client** behind the ADR-0023.1 D4 seam, plus the
  offline judge scorer machinery. No judge score is citable before calibration
  κ ≥ 0.6 against the gold labels; below 200 labeled episodes the judge layer
  self-labels PRELIMINARY regardless of κ — a threshold not yet crossed, so judge
  scores remain **PRELIMINARY** ([ADR-0023.1](docs/adr/0023.1-judge-scorer-decision.md)).

### Changed

- **CLI ergonomics batch — errors that teach.** Bad invocations now return
  actionable, teaching error messages; truncation text names the exact `--budget`
  or `--full` flag to lift or narrow the result; `read` accepts hidden
  agent-guess range aliases (`--start/--end`, `--offset/--limit`) for frictionless
  retries while help converges on one spelling; exit codes are semantic (`0` ok,
  `1` no results, `2` bad invocation, `3` upstream failure); `ghx --version` is a
  first-class health check. Agents are ghx's users, and their traces were the
  usage study that drove this batch
  ([ADR-0028.1](docs/adr/0028.1-cli-ergonomics-batch.md); mining evidence at
  `docs/evals/mining-2026-07-06/CLI-FINDINGS.md`).
- **Sidecar persona revised (revision 1)** for report discipline: tighter
  read-ladder guidance, sharper search doctrine, and more compact reports
  ([ADR-0029](docs/adr/0029-persona-revision-1.md)).
- **Eval run economics — baseline reuse.** Each run takes an identity inventory
  and reuses matching baseline (`plain`/`ghx`) episodes across runs with `>= trial`
  semantics, so only the changed profile is re-run; a mismatch triggers a loud
  refusal rather than a silent, expensive fallback. Bounded episode parallelism and
  sequential stopping cut run cost further
  ([ADR-0025](docs/adr/0025-eval-run-economics.md),
  [ADR-0025.1](docs/adr/0025.1-baseline-reuse-implementation.md)).
- **Measurement fix batch:** union-of-turn-reports scoring (so delta-reporting is
  not penalized), an answer-doc contamination guard, and sufficiency honesty in the
  scorer ([ADR-0016.8](docs/adr/0016.8-measurement-fix-batch.md)).
- **README** now leads with the Agent Sidecar Framework positioning: ghx as the
  code-reconnaissance sidecar a main agent delegates to, with the standalone
  exploration CLI documented underneath.

### Fixed

- **Stale ACP session IDs downgrade to a fresh session** instead of failing the
  ask, so a recycled agent process no longer breaks a resumed investigation.
- **Report reliability:** strict judge parsing rejects trailing top-level values;
  the shared-artifacts JSONL appender is now per-path locked to prevent interleaved
  writes under parallelism ([ADR-0025](docs/adr/0025-eval-run-economics.md) D3).
- Removed the legacy `~/.ghx-sidecar` read-fallback; `~/.ghx` is the single
  product root.

### Measured

- **Citable verdict: THESIS SUPPORTED.** The `gate-run-2026-07-06-fixbatch` run —
  the first measured under the ADR-0016.8 measurement fix batch and the ADR-0027
  resilient runtime — passes all five pre-registered gates: correctness
  sidecar/ghx ratio **0.983** (floor 0.90), evidence **0.887** (floor 0.70),
  compression **≈16.6×** (threshold 2.9×), memory resume **1.00** with repeat-read
  0.427 < ghx 0.483, and safety **1.0** on all sidecar episodes. Sequential
  stopping reports STOP-SUCCESS-LOCKED: no remaining episode could have changed any
  thesis gate ([verdict](docs/evals/gate-run-2026-07-06-fixbatch/verdict.md),
  [run dir](docs/evals/gate-run-2026-07-06-fixbatch/)).
- **Independent cross-family audit.** A different model family (OpenAI Codex, run
  read-only) adversarially recomputed all 90 episode scores of the confirmatory run
  exactly and confirmed the gate outcomes, logging the provenance defects it found
  in the open ([audit report](docs/evals/audit-2026-07-05-independent/REPORT.md)).
- The honest negative that drove these fixes is kept alongside the verdicts at
  [docs/evals/gate-run-2026-07/](docs/evals/gate-run-2026-07/). Verdicts are read
  as a conservative floor; the artifacts are the proof either way.

[Unreleased]: https://github.com/gkoreli/ghx/compare/v2.6.0...HEAD
[2.6.0]: https://github.com/gkoreli/ghx/compare/v2.5.0...v2.6.0
[2.5.0]: https://github.com/gkoreli/ghx/compare/v2.4.2...v2.5.0
[2.4.2]: https://github.com/gkoreli/ghx/compare/v2.4.1...v2.4.2
[2.4.1]: https://github.com/gkoreli/ghx/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/gkoreli/ghx/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/gkoreli/ghx/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/gkoreli/ghx/compare/v2.1.19...v2.2.0
