---
title: "Codebase Reality Audit — what's built vs aspirational, module health, sidecar runtime state"
date: "2026-08-21"
status: "audit"
author: "background audit worker (reality inventory)"
scope: "internal/**, cmd/**, skills/**, experiments/**, docs/adr/** — read-only audit, no code changes, nothing committed"
builds-on: "architecture-2026-07-07.md, complexity-hotspots-2026-07-07.md, coupling-cohesion-testability-2026-07-07.md, AA-0002, ADR-0035/0036 — this audit re-verifies which of those findings still hold at HEAD (5af6692, 2026-08-21) and adds a milestone gap table"
---

# Codebase Reality Audit — 2026-08-21

Question: what is actually built and load-bearing today vs experimental/dead,
how healthy is each module, and where does the sidecar runtime really stand
against the NORTH_STAR milestone tables?

Answer in one paragraph: **the codebase is in a late-healthy state with no dead
architecture**. `go build ./...` and `go test ./...` are green (~22s wall for the
full suite). Every module has a live consumer; the only genuinely dead artifact
is the April `experiments/ghx-sidecar/` POC (explicitly quarantined by its own
INSTRUCTIONS.md). The sidecar runtime is real and wired through all three
entrypoints (CLI ask, MCP recon, auto-spawning warm daemon), but its brain is an
external ACP agent (`npx @agentclientprotocol/claude-agent-acp@0.55.0`) — the
P3/P4 "swallow the tools / below ACP" milestones remain documentation, not code.
Of the 2026-07-07 audit findings, the two High items were since fixed
(callTool removed; acp.go 1012→401 lines decomposed); the eval-package
god-files and the core recon client-seam gap remain open, exactly as the prior
audits ranked them.

## 1. Per-module inventory (LOC = non-test lines; tests counted separately)

Measured with `find <pkg> -name '*.go'` + `wc -l` and `grep -c '^func Test'`;
full command list in the appendix.

| Module | Code LOC (files) | Test LOC (files) | Test funcs | Test:code | Role & health |
|---|---|---|---|---|---|
| `internal/ghx` | 2,032 (11) | 952 (7) | 36 | 0.47 | Core evidence engine (explore/read/search/repos/tree/glob/inspect/map-fallback). Load-bearing: wrapped by CLI, MCP, and codemode. Tests cover pure helpers only (parseFileResponse, glob, rankers) — the GraphQL query→domain mapping still has no fake-transport tests (coupling audit H1, still open at HEAD: `explore.go:32`, `read.go:93` etc. construct `api.DefaultGraphQLClient()` inline). |
| `internal/codemode` | 574 (5) | 350 (2) | 15 | 0.61 | goja JS sandbox + esbuild + type gen. Load-bearing (CLI `ghx code`, MCP `code`). The callTool compat shim flagged by architecture audit H1 is **gone** at HEAD (grep for `callTool` in `internal/codemode/` + `serve.go` → empty). |
| `internal/mapengine` | 960 (4) | 488 (1) | 19 | 0.51 | Parser-backed structural map engine. Load-bearing behind `ghx map` / read map fallback. |
| `internal/sidecar` | 10,179 (35) | 7,519 (33) | 555 | 0.74 | The SAF runtime: ACP runner, daemon, sessions, routing, ledger, tier policy, reports, preflight. The heaviest test mass in the repo; 17.5s of the 22s suite lives here. Healthiest large module. |
| `internal/sidecar/evals` | 12,970 (44) | 9,672 (45) | 275 | 0.75 | SAFE: gates, rewards, discovery, judge clients, closed-book, host-task, baseline reuse, anomalies. Largest package; still carries the repo's three biggest files (discovery.go 1007, gates.go 999, anticipation_predictor.go 974) — frozen-measurement-stack, ADR-gated, deliberately not refactored (complexity audit H2; AA-0002.9 "DO NOT touch"). |
| `internal/sidecar/tier2` | 2,306 (9) | 1,647 (8) | 56 | 0.71 | Tier-2 structural backends: codemap, ast-grep, repomap + cache. Load-bearing since ADR-0024.1/.2; runtime-owned escalation (ADR-0024.4) landed 2026-08-21. |
| `internal/sidecar/telemetry` | 808 (6) | 380 (3) | 5 | 0.47 | Shared OTel substrate (AA-0002.9 confirms it is a genuine, correctly-sized shared kernel; instability 0.00 leaf). |
| `internal/cli` | 2,848 (8) | 1,259 (5) | 52 | 0.44 | Cobra composition root: core commands, MCP serve, sidecar ask/daemon/sessions/config, tier2, evals export. Two aggregator files remain large (ghx.go 843, sidecar.go 805) with ~26 inline RunE bodies — complexity audit M1 still open. |
| `cmd/ghx` | 19 (1) | 0 | — | — | Trivial main(). |
| **Total** | **~32,700** | **~22,300** | **1,013** | **0.68** | 16 Go packages; 52 direct+indirect deps in go.mod (go 1.25). |

### Load-bearing vs experimental/dead

**Dead / quarantined:**
- `experiments/ghx-sidecar/` (973 lines, 5 md files) — the April 2026 manual
  POC (Codex session + pasted prompts). Single commit `5dbfd2d` (2026-04-25),
  untouched since; INSTRUCTIONS.md:3 says "Do not build architecture from this
  yet." Superseded by the entire Go runtime. Keep as history or delete; it is
  not load-bearing.

**Experimental-but-live (code shipped, citable value pending):**
- Judge layer (`evals/judge*.go`, ~10 files): offline machinery merged
  (ADR-0023.1), but no judge score is citable until founder-labeled κ ≥ 0.6
  calibration passes (C4 frontier).
- Host-task evals (`evals/hosttask/`, `host_episode.go`): ADR-0032.1 decided,
  gates registered; the run is Goga-triggered (C8).
- Anticipation: `evals/anticipation_predictor.go` (974 lines) exists, but the
  M8 v1 D1 gate FAILED (nextReads recall 0.071/0.000) and build was correctly
  halted; `match.go` is the only non-eval touchpoint. No prefetch path exists
  in the runtime (grep `prefetch` in `internal/sidecar/*.go` non-test → empty).

**Everything else is load-bearing**, including the daemon (contrary to the
NORTH_STAR B6 "Future" row — see §3).

### 2026-07-07 audit findings re-verified at HEAD (2026-08-21)

| Prior finding | Status at HEAD | Evidence |
|---|---|---|
| arch H1: callTool compat + serve.go:348 usage string | **Fixed** | `grep -rn callTool internal/codemode/ internal/cli/serve.go` → empty |
| arch H2: acp.go god-file (1012) | **Fixed** | acp.go now 401; denyclient.go 352, turn.go 133, tooltrace.go 136, turnresult.go 100 |
| coupling H1: no GitHub client seam in core | **Open** | `api.DefaultGraphQLClient()` still inline in explore.go/read.go/repos.go/glob.go; search.go uses NewRESTClient |
| coupling H2: dual ACP turn engines (one-shot vs warm) | **Partially addressed, not closed** | `acprunner.go` (claudeACPRunner) now exists as a seam (ADR-0036 C1/C2 direction), but `daemon_worker.go` warm path and `acp.go` one-shot path still coexist; `TurnRunner` func type at runtime.go:20 remains the actual seam |
| complexity H1: askWithTurnRunner complexity 40 | **Open** | still one function at runtime.go:254; runtime.go now 696 lines |
| complexity H2/M3: eval god-files | **Open (deliberate)** | gates.go 920→999 (grew with G6), discovery.go 1007, anticipation_predictor.go 974 |
| AA-0002.9: ToolCallTrace Locations drop | **Fixed via ADR-0032.2** | ADR-0032.2 status "accepted"; `argvlocations.go` + `tooltrace.go` in tree |

## 2. ADR thread census (shipped vs stalled)

`grep -h '^status:' docs/adr/*.md | sort | uniq -c`: 44 accepted-family
(accepted / accepted-on-merge / Decided / measured / validated), 17
proposed-family (proposed / Proposed / pre-registered), 8 research-family,
2 misc.

**Shipped threads (code + ADR both landed):** ACP runtime (0015), eval kernel +
validity (0016.1–.9), OTel visibility (0016.4, 0018, 0022), report contract
(0021), discovery tier (0019.1/.2), resume/steering (0020.x), resilience
(0027), CLI ergonomics + inspect (0028.x), persona revisions (0029.x), daemon +
session routing (0030, 0030.1), escalation tiers (0024.1/.2/.4), run economics
(0025.x), view absorption (0026.1), out-of-box config (0033.x), failure-class
model (0034, 0034.1), session memory / backend governance / report validation
(0037/0038/0039, all 2026-08-21), tooltrace unification (0032.2), corpus
refresh (0016.13).

**Stalled-at-proposed (written, not implemented):**
- **ADR-0035** (tactical hardening) and **ADR-0036** (Runner port +
  composition root + domain flow) — both `proposed`, dated 2026-07-07. Note
  ADR-0036 is *partially* pre-implemented: `acprunner.go` and `repo.go`
  (`ghx.Repo` value object, with tests) exist, and the failure-class work
  (0034.x) landed, but the composition root, the `internal/sidecar/runner`
  leaf package, codex-acp adapter, and Snapshot{Repo,SHA} do not exist.
- **ADR-0031.1** anticipation v1: decided, predictor built, D1 gate failed,
  build halted (this is a *successful* stall — the pre-registration worked).
- **ADR-0016.12/.13**: 0016.13 fixtures landed 2026-08-21 (live canaries
  pending); 0016.12 proposed.

## 3. Where the sidecar runtime is actually wired

**Entrypoints — all three real:**
1. **CLI**: `ghx sidecar ask [--repo] <question>` (internal/cli/sidecar.go:53),
   plus `sessions list/show/ledger/reroute`, `doctor`, `tail`, `config
   show/init`, `evals export`, `report-sink`, `daemon`.
2. **MCP**: `ghx serve` registers 7 direct tools (explore/repos/search/read/
   tree/search_tools/code, serve.go:81-152) **plus** the sidecar recon tool:
   `s.AddTool(sidecar.ReconMCPTool(), handleRecon)` (serve.go:163), routed
   through `askSidecar` → `sidecar.AskViaDaemon` (serve.go:166-167).
3. **Daemon**: `AskViaDaemon` (daemon.go:465) **auto-spawns** the warm daemon
   on first ask and falls back to daemonless `Ask` with a stderr warning
   (daemon.go:478). This means NORTH_STAR B6 ("Always-on runtime … Future") is
   **understated**: warm daemon shared across CLI+MCP with no manual cold start
   already ships; what's missing for full B6 is only supervisor hardening
   (per-request recover(), context propagation, graceful drain — ADR-0036
   Phase A1, still proposed) and the "resident across process invocations"
   polish (idle worker TTL default 30min, config.go:92-99).

**What a real `ghx sidecar ask` requires** (verified in source):
- **Auth**: a GitHub token via `gh auth login` or GH_TOKEN/GITHUB_TOKEN —
  preflight `gh-token` check at preflight.go:135-148 (go-gh TokenForHost).
- **ACP adapter**: the brain is an external agent speaking ACP. Default pinned
  command `npx -y @agentclientprotocol/claude-agent-acp@0.55.0`
  (config.go:40, `ClaudeACPAgentCmd`); bare `claude`/`codex` are probed by
  DetectAgents (config.go:286-291) but the comment at config.go:157 is explicit
  that a bare claude CLI cannot speak ACP — you need the adapter (or
  `config init --claude-acp` / `--claude-exe`). Node/npx must be on PATH.
- **Model creds**: the ACP agent's own Anthropic auth (or Bedrock/Vertex
  gateway env, captured and forwarded per-ask by ADR-0033.1's
  `CaptureAgentAuthEnv`, daemon.go:471-475).
- **Session routing** (ADR-0030.1): live in both daemon and daemonless paths —
  deterministic R1–R5 cascade (`route.go`), route provenance on every answer,
  `sessions reroute` recovery (reroute.go, RerouteViaDaemon daemon.go:497+).
- **Escalation tiers** (ADR-0024.x): tier-0/1 remote via ghx CLI inside the
  ACP turn; tier-2 (`internal/sidecar/tier2`: codemap/astgrep/repomap + cache)
  is agent-invoked via `ghx tier2 *` shell commands, with the runtime recording
  decisions (`tier.go:22` tier-decisions.jsonl), reconciling tierUsed
  provenance, and — since ADR-0024.4 (2026-08-21) — enforcing pre-hoc grants
  (`GHX_TIER2_ALLOWED_BACKENDS`) on both ask paths (runtime.go:403-407). The
  sidecar *decides* to escalate (B9/M7 brain half) via `ghx tier2 observe
  --signal <id>`; the runtime owns policy. Tier-3 handback is the report
  itself.
- **The turn path**: ask → route → session (create/resume) → prompt built
  (prompt.go persona + ledger) → ACP turn (claude-agent-acp spawns `claude`
  which shells out to `ghx` CLI) → report extracted/validated → ledger +
  OTel traces + reports + artifacts in `~/.ghx`.

So: the sidecar is **not** a model, not a Go agent loop — it is a Go runtime
that orchestrates an external Claude agent over ACP. P4 ("below ACP",
claude-sdk adapter) has zero code; the codex-acp adapter has zero code; both
wait on ADR-0036.

## 4. Milestone gap table (evidence refs; sizes are rough)

Legend: ✅ shipped · 🟡 partial · ❌ missing. "Missing slice" = rough LOC of
code that would have to be written.

| ID | Milestone | Status | Exists | Missing slice |
|---|---|---|---|---|
| M5 | Recon-service skill + integration ergonomics; daily dogfood | ✅ code / 🟡 exit | skills/ghx-recon/SKILL.md (47 lines — meets "concise"), MCP recon tool, config init, ADR-0019 D1-D4 | Dogfood-week exit is a founder activity, not code. |
| M6 | Shared SAF/SAFE trace infra, ~/.ghx, view | ✅ | internal/sidecar/telemetry + emit.go (556) + view.go; ADR-0018/0022 | — |
| M7 | Escalation tiers, sidecar-decided, fully visible | ✅ (2026-08-21) | tier2/ (2,306+1,647 test), tier.go, ADR-0024.1/.2/.4, G6 gate | — (M7 declared complete by NORTH_STAR A3 row + ADR-0024.4) |
| M8 | Eager anticipation, sub-second | ❌ halted at v1 gate | predictor (974, evals-only) + ADR-0031.1/.2; D1 FAILED (recall 0.071) | Runtime prefetch path (~300–600 LOC) + predictive nextReads persona rev — deliberately blocked pending gate re-design. |
| M9 | Trajectory accumulation + SFT/KTO exports | 🟡 | export cmd exists: `sidecar evals export --format sft` (sidecar.go:240); ADR-0017.1 decided | Calibrated judge labels (C4) gate trajectory quality; consented dogfood-session corpus doesn't exist yet. Missing: production-session ingestion (~500–1,000 LOC). |
| M10 | Trained ghx-sidecar model behind same boundary | ❌ | nothing (correctly) | The whole P4 layer; unblocked only via M9. ADR-0036 Phase D (claude-sdk adapter, ~1–2k LOC) is the *harness* prerequisite. |
| B1 | Go-native runtime over ACP | ✅ | internal/sidecar core | — |
| B2 | Zero-CLI adoption surface | ✅ | ghx-recon skill, MCP recon, config init | — |
| B3 | Visibility substrate + view | ✅ | see M6 | — |
| B4 | Runtime resilience (ADR-0027) | ✅ | acp_resilience_test.go, wrap-up recovery, artifacts-on-failure | — |
| B5 | Discovery tier (repo optional) | ✅ | evals/discovery.go, ADR-0019.1/.2; skill documents repo-less asks | Citable D-G run still pending (C5 caveat). |
| B6 | Always-on warm daemon | 🟡 (understated as Future) | auto-spawn + fallback (daemon.go:465-481), 30min idle TTL, shared CLI+MCP | Supervisor hardening (ADR-0036 A1, ~100–200 LOC); cross-boot residency if wanted. |
| B7 | Session routing | ✅ v1 | route.go + reroute.go, ADR-0030.1 | D4 phase-2 dogfood route-record audit. |
| B8 | Persona proficiency loop | 🟡 standing | prompt.go + revisions 1–3 (ADR-0029.x) | Each further rev is mining-driven; standing work. |
| B9 | Sidecar-decided escalation + anticipation | 🟡 half | escalation half ✅ (ADR-0024.2/.4) | anticipation half = M8 ❌. |
| C1 | Eval kernel + gates | ✅ | gates.go/rewards.go, M4 THESIS SUPPORTED | — |
| C2 | Run economics D1-D3 | ✅ / 🟡 D4 | baseline_reuse.go, ADR-0025.x | D4 haiku arm pending pre-registration. |
| C3 | Measurement fidelity | ✅ | ADR-0016.8, fixbatch run citable | — |
| C4 | Judge layer κ ≥ 0.6 | 🟡 | ~10 judge files, offline machinery merged | Founder gold-set labeling + calibration run; code slice small, process slice large. |
| C5 | Discovery eval tasks | ✅ | discovery fixtures + D-G gates | Formal citable D-G run pending. |
| C6 | Continuous eval of production sessions | ❌ | nothing | Gated on C4; then ~500–1,000 LOC (judge-over-sessions loop). |
| C7 | Trust ledger to green | 🟡 | H7 fixtures landed 2026-08-21 (ADR-0016.13); several rows closed | H1 (κ), H2/H3 audits, H5 real-token accounting (awaits ADR-0036 Phase D), live closed-book canaries. |
| C8 | Host-task combined evals | 🟡 decided | hosttask/ + host_episode.go + registered gates (ADR-0032.1) | Build slices S1–S4 + the Goga-triggered run. |

**Rough total missing code across all open slices: ~3–6k LOC** (anticipation
runtime path, supervisor hardening, production-session ingestion, codex-acp
adapter, claude-sdk adapter) — small against a 32.7k-LOC tree, and every slice
is ADR-gated rather than unstarted-by-accident.

## 5. Commands run (appendix)

```
git status; git log --oneline -5                      # mainline @ 5af6692, clean, 7 behind origin
go build ./...                                        # exit 0
go test ./...                                         # all ok; sidecar 17.5s, evals 12.5s, total wall ~22s
go list ./... | wc -l                                 # 16 packages
for d in <9 pkgs>: find $d -name '*.go' ... | xargs wc -l   # LOC table above
grep -rh '^func Test' <pkg> --include='*_test.go' | wc -l   # test counts above
grep -h '^status:' docs/adr/*.md | sort | uniq -c     # ADR census
grep -n 'Use:' internal/cli/sidecar.go                # sidecar subcommand surface
grep -n 'mcp.NewTool\|AddTool' internal/cli/serve.go  # MCP surface incl. ReconMCPTool (serve.go:163)
grep -n 'AskViaDaemon\|daemonless fallback' internal/sidecar/daemon.go  # auto-spawn + fallback (465-481)
grep -n 'ClaudeACPAgentCmd\|TokenForHost' internal/sidecar/{config,preflight}.go
grep -rn 'callTool' internal/codemode internal/cli/serve.go   # empty → arch H1 fixed
wc -l internal/sidecar/{acp,turn,denyclient,tooltrace,turnresult}.go  # arch H2 fixed
grep -n 'func askWithTurnRunner' internal/sidecar/runtime.go  # still one function (254)
find experiments -type f | xargs wc -l; git log -- experiments/  # POC, 1 commit 2026-04-25
wc -l skills/*/SKILL.md                                # 47/216/232
```

Build/test output (verbatim, tail):

```
ok  github.com/gkoreli/ghx/v2/internal/cli          0.024s
ok  github.com/gkoreli/ghx/v2/internal/codemode     (cached)
ok  github.com/gkoreli/ghx/v2/internal/ghx          (cached)
ok  github.com/gkoreli/ghx/v2/internal/mapengine    (cached)
ok  github.com/gkoreli/ghx/v2/internal/sidecar      17.521s
ok  github.com/gkoreli/ghx/v2/internal/sidecar/evals 12.526s
ok  github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask 0.131s
ok  github.com/gkoreli/ghx/v2/internal/sidecar/telemetry (cached)
ok  github.com/gkoreli/ghx/v2/internal/sidecar/tier2 0.569s
ok  github.com/gkoreli/ghx/v2/skills                (cached)
real 0m21.747s
```

## 6. Uncertainty

- **Branch staleness**: the working tree is 7 commits behind `origin/mainline`
  (fast-forwardable). Findings describe local HEAD `5af6692`; the 7 unpulled
  commits could shift details (nothing in the milestone tables suggests a
  structural change).
- **No live `ghx sidecar ask` was executed** (requires GitHub token + the
  claude-agent-acp adapter + Anthropic auth). The wiring claims in §3 are from
  source reading + tests, not an end-to-end run; the M6 ADR records a live
  ask leaving traces/reports, which corroborates.
- **Missing-slice LOC estimates are order-of-magnitude**, inferred from
  neighboring code (e.g. acprunner.go as a template for codex-acp), not designs.
- Test-func counts understate table-driven cases (one func can cover dozens of
  cells); ratios are for shape comparison, not coverage claims. No coverage
  profile was run.
- The `~/.ghx` on-disk state of this machine was not inspected (single-user
  dogfood artifacts may exist beyond the repo).

## 7. Suggested next reads

1. `docs/adr/0036-target-architecture-runner-port-and-boundaries.md` — the
   decision that gates the largest missing slices (codex-acp, claude-sdk,
   supervisor hardening); it is still only `proposed`.
2. `docs/evals/TRUST.md` — which C7 holes (H1 κ, H2/H3, H5) are actually open
   vs closed; this audit inferred from ADR statuses only.
3. `internal/sidecar/runtime.go:254` (askWithTurnRunner) — the one function to
   read before any refactor work; complexity audit H1's recommendation
   (AskCoordinator extraction) is still the right first move.
4. `docs/adr/0031.2-nextreads-contract-revision.md` — the live decision point
   on whether M8 anticipation gets a predictive persona revision or stays
   halted.
5. `docs/audits/architecture-vision/` (five audits) — only if acting on
   ADR-0036; this audit did not re-derive them.
