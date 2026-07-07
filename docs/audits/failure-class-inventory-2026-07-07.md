---
title: "Audit: Failure-Class Representation Inventory Across Frontends"
date: "2026-07-07"
thread: "multi-frontend-architecture"
author: "Audit worker (read-only) — evidence for the PENDING ADR-0034 decision"
status: "evidence — no decision, no code change"
---

# Failure-Class Inventory Across Frontends (evidence for ADR-0034)

Read-only audit. It exists so the ADR-0034 decision (unified failure-class model,
`docs/adr/0034-failure-class-model.md`, **proposed**, awaiting Goga) can be made
with file:line facts instead of the ADR's prose summary. It does **not** decide,
edit ADR-0034, or refactor code.

## Method / basis

- **Commit basis: `47e2b7c` (mainline tip).** This worktree started at
  `e8816ec` (v2.6.0), which *predates* the A4 affordance machinery and ADR-0034
  itself. It was fast-forwarded to `47e2b7c` (a clean ancestor→tip ff, no
  mainline ref touched) so every file:line below is verifiable in place and
  matches what ADR-0034 was written against. Auditing the v2.6.0 checkout would
  have produced false "dead code / no tests" findings — the affordance layer and
  its tests only exist from the A4 batch onward.
- Every claim carries file:line. Counts are reproducible with the greps in the
  Evidence Contract at the end. `go test ./internal/cli` (affordance/exit tests)
  passes at this basis.

## Scope correction (task prompt vs. reality)

The task named `internal/cli/inspect.go` for "exit-code/substring coupling."
**That file does not exist.** Inspect is split:

- **core detection** in `internal/ghx/inspect.go` (returns bare errors), and
- **CLI wiring + the exit-code/substring coupling** in
  `internal/cli/ghx.go` (`inspectCmd`, lines 405-424).

The coupling the task asked about is at **`internal/cli/ghx.go:415`** (detailed
in B3 below). ADR-0034 itself never references `inspect.go`; this is only a
task-prompt path slip, noted so the reader looks in the right place.

---

## 1. Per-frontend inventory: how each surface represents failure TODAY

### 1a. CLI — the one frontend that has a taxonomy

Semantic exit codes and the whole affordance machinery live in
`internal/cli/errors.go`:

| Element | Location |
| --- | --- |
| Exit codes `ExitOK=0 / ExitNoResults=1 / ExitBadInvocation=2 / ExitUpstreamFailure=3` | `errors.go:9-18` |
| `ExitError{Code,Err}` type + `Error`/`Unwrap` | `errors.go:20-40` |
| `WithExitCode(code, err)` | `errors.go:42-48` |
| `CodeForError(err)` — `errors.As` to the code; **default fallback `ExitBadInvocation` for any untyped error** | `errors.go:50-60` (fallback at `:59`) |
| `affordanceMarker = "→ "` (fix-it line prefix) | `errors.go:65` |
| `withAffordance(code, err, hint)` — appends `"\n→ <hint>"` | `errors.go:73-82` |
| `upstreamRule` / `upstreamRules` table — **5 substring-matched classes**: rate-limit, auth/401, 404/not-found, 403/forbidden, network | `errors.go:89-122` |
| `upstreamAffordance(err)` — lowercases `err.Error()` and substring-matches the table | `errors.go:128-141` |
| `upstreamError(err)` — single entry point: `withAffordance(ExitUpstreamFailure, err, upstreamAffordance(err))` | `errors.go:147-149` |

**Where codes are actually returned** (the class is chosen at the call site, by
the handler, not by core):
- `upstreamError(err)` is wired at **13 sites**: `internal/cli/ghx.go:59,100,181,311,366,418,457` and `internal/cli/tier2.go:40,109,125,160,174,218`.
- Explicit `WithExitCode(ExitNoResults, …)`: `ghx.go:63,321,370,422`, `tier2.go:180,227`.
- Explicit `WithExitCode(ExitBadInvocation, …)`: `ghx.go:159,416,453,517,524,526`, `tier2.go:97`.
- `WithExitCode(ExitUpstreamFailure, …)` (non-`upstreamError`): `sidecar.go:261`.
- **Process exit** is set once: `cmd/ghx/main.go:17` — `os.Exit(cli.CodeForError(err))`.

**Substring coupling (the M2 / W2 fragility, live):** `internal/cli/ghx.go:415`
decides BadInput vs. upstream by matching the *English text core produced*:
```
if strings.Contains(err.Error(), "invalid repo") || strings.Contains(err.Error(), "query must not be empty") {
    return WithExitCode(ExitBadInvocation, err)
}
return upstreamError(err)
```
The matched strings are emitted by core at `internal/ghx/inspect.go:245` (`"invalid repo %q: expected owner/repo, e.g. …"`) and `:249` (`"query must not be empty, e.g. …"`). A core wording change that drops the substring `"invalid repo"` silently demotes a bad-input error (exit 2) into the `upstreamError` fallback (exit 3). This is the exact round-trip ADR-0034 wants to remove.

**Contract tests (the "A4 tests" ADR-0034 migration step 2 must prove
byte-identical against) — they exist, in `internal/cli/errors_test.go`:**
- `errors_test.go:12-38` pins the hint substring per class (`gh api rate_limit`, `gh auth login`, `ghx explore owner/repo`, `ghx repos`, `gh auth status`, `check connectivity`).
- `errors_test.go:42-49` — an unrecognized upstream error yields **no** invented hint.
- `errors_test.go:54-73` — `upstreamError` keeps exit 3, preserves the raw text verbatim, and appends the hint on its own final line behind `affordanceMarker`.
- `errors_test.go:77-86` — an unmatched upstream error is **unmutated** (`err.Error() == raw`).
- `errors_test.go:90-102` — empty hint is a no-op; nil stays nil.
- Exit-code mapping also pinned in `ghx_test.go:124-140` (incl. untyped→`ExitBadInvocation` at `:139`) and `tier2_test.go:96-136`.

### 1b. Core (`internal/ghx`) — no failure class at all

**18 bare `fmt.Errorf` / `errors.New`** (non-test), by file: `explore.go` (4),
`read.go` (3), `repos.go` (2), `inspect.go` (2), `search.go` (2), `glob.go` (5).
None carry a type or class. Example: `inspect.go:245,249`. Every non-CLI frontend
receives these as opaque strings; the CLI re-classifies them by substring (1a).
*(Confirms ADR-0034's "~18" — exactly 18.)*

### 1c. MCP direct tools (`ghx serve`, `internal/cli/serve.go`)

**16 `NewToolResultError` sites in `serve.go`** — but not all are equivalent:
- **15 are opaque `err.Error()` flattenings**: `serve.go:174,205,239,246,256,263,277,288,298,303,327,337,345,383,390`. Every failure, whatever its class, becomes one string.
- **1 is already a structured bad-input message** (not `err.Error()`): `serve.go:193-194` rejects an invalid `depth` with `"invalid depth %q: valid values are cheap, normal, deep"`. This is the only MCP site that today behaves the way ADR-0034 wants all of them to — proof the pattern is viable, and a hint that the migrated surface will be *mixed*.
- `handleCode` (codemode) flattens the JS-boundary error at `serve.go:390`.

**3 more MCP-error sites live in `internal/sidecar/reportsink.go`** (the
`submit_report` sink tool), not counted by ADR-0034: `:356` (validation, wrapped),
`:360` (bare `err.Error()`), `:365` (persist failure, wrapped). **Total MCP error
surface = 19 sites** (16 + 3), of which 16 are effectively opaque.

### 1d. MCP recon (`ghx serve --recon`)

- Success path marshals the sidecar `Report` to JSON: `serve.go:207` (`json.Marshal(report)`), then appends route/artifacts footers (`serve.go:209-…`).
- **Every failure is flattened before the report contract is ever reached**: missing `question` → `serve.go:174`; invalid `depth` → `serve.go:193`; sidecar/ask error → `serve.go:205`. So a failed recon = one opaque string; the class-carrying report is not produced.
- Tool definition is a single frozen contract: `internal/sidecar/recontool.go:21-28` (`ReconMCPTool`). Its marshaled JSON (name/description/input schema) is hashed as `reconToolSchema` into host-task run manifests: `internal/sidecar/evals/host_identity.go:25-49`, verified recomputable by `internal/sidecar/evals/host_episode_test.go:293-298`. **The hash is over the schema only — not over error payloads or report output.** This *confirms* ADR-0034's line 104-105 claim: structuring MCP error *payloads* is schema-neutral and does not disturb the ADR-0032.1 S3 identity hash.

### 1e. Sidecar runtime (ADR-0027 resilience) — a *separate*, string-matched taxonomy

The runtime has its own failure taxonomy, distinct from the CLI's GitHub-upstream
one, and it is **also substring-based**:

| Element | Location |
| --- | --- |
| "Runtime resilience taxonomy (ADR-0027)" markers | `internal/sidecar/acp.go:41-64` |
| `LivenessTimeoutMarker = "liveness watchdog timeout"` | `acp.go:58` |
| `maxTurnsErrorMarker = "Reached maximum number of turns"` | `acp.go:61` |
| `peerClosedMarker = "peer connection closed"` | `acp.go:64` |
| `ErrLivenessTimeout` sentinel (`errors.Is`-matchable) | `acp.go:73` |
| `IsMaxTurnsError` — `strings.Contains` match | `acp.go:78-79` |
| `IsPeerClosedError` — `strings.Contains` match | `acp.go:84-89` |
| `IsLoadSessionResourceNotFound` — JSON-RPC code `-32002` + message match | `acp.go:96-…` |
| `DiagnoseTurnError` — enriches with stderr tail, **preserves the `errors.Is` chain** | `internal/sidecar/agentdiag.go:508-520` |

The taxonomy comment is explicit that these markers are a **cross-package
string contract**: the runtime picks a recovery path (D1 wrap-up / D2 fail-fast)
and *"the eval layer uses them to derive anomalies … from the persisted turn
error strings without importing runtime internals"* (`acp.go:41-47`).

`TurnResult` (`internal/sidecar/turnresult.go:6-…`) carries **boolean flags, not a
class**: `ReportRetried` (`:39`), `ReportCoerced` (`:46`), `WrapUpRecovered`
(`:51`), `SessionRecreated` (`:55`), plus an `Error` string set in
`runtime.go`. **`fmt.Errorf` footprint:** 108 sites across 26 files in the
sidecar runtime proper (excl. `evals/`); 306 across 57 files including `evals/`.

### 1f. Sidecar report contract — the agent/MCP-visible surface, no class field

`Report` struct, `internal/sidecar/report.go:38-56`:
`answer, verified, inferred, unverified, relevantFiles, evidence, tierUsed,
backendsUsed, commandsRun, uncertainty, nextReads`. **There is no failure-class
field.** The only place a failed turn can "say why" is `Uncertainty []string`
(`report.go:55`) — free text — or the flattened error string (1d). This is the
exact ADR-0034 gap: *"a turn that fails upstream and a turn that fails on bad
config are indistinguishable to the report contract."* This is also the object
`serve --recon` marshals (`serve.go:207`), so the report gap **is** the recon
gap. Report parsing is lenient (`report_test.go:25-136` shows string-vs-array
coercion tolerated), so a new additive class field would be parser-safe.

### 1g. Eval anomaly layer — a *third* taxonomy (observability)

`internal/sidecar/evals/anomalies.go:30-89` defines **12 anomaly buckets** with
`breaking`/`soft` severity: `sidecar_blocked_report`, `sidecar_report_missing`,
`sidecar_report_block_unparsed`, `sidecar_report_retried`,
`sidecar_report_coerced`, `direct_ghx_noncompliance`,
`eval_parallel_rate_limited`, `answer_doc_contamination`, `turn_cap_wrapup`,
`episode_hang_timeout`, `trace_capture_gap`. They are derived **from the persisted
turn-error strings** using the `acp.go` markers (comment `acp.go:41-47`),
explicitly "never reward inputs." A separate concept from failure-class, but
overlapping (`turn_cap_wrapup`/`episode_hang_timeout` are runtime failures).

### 1h. codemode (JS boundary)

- A tool error inside JS becomes a thrown JS error: `internal/codemode/executor.go:170` — `panic(vm.NewGoError(err))` (also the guard rails at `:113,120,126,190,210`).
- Uncaught → `execErr = "execution panic: …"` (`executor.go:253`) → `Execute` returns `(nil, execErr)` (`:260`) → `handleCode` flattens it: `serve.go:390`.
- Each call's error is recorded only as a string: `ToolCallRecord.Error` at `executor.go:158-166`.
- **No class survives the boundary twice over:** first at `vm.NewGoError` (Go error → JS value), then at `err.Error()` in `serve.go:390`. Even if core grew a typed `FailureClass`, codemode discards it unless the executor is taught to thread it.

---

## 2. Where the representations DIVERGE

The same domain outcome is rendered five incompatible ways, and there are **three
independent, non-interoperating failure taxonomies** plus two class-free surfaces:

| Domain outcome | CLI | Core | MCP direct | MCP recon | Sidecar report | Runtime | Eval anomaly |
| --- | --- | --- | --- | --- | --- | --- | --- |
| GitHub auth failed | exit **3** + `gh auth login` hint (`errors.go:107-108`) | bare string | opaque string (`serve.go:239…`) | opaque string (`serve.go:205`) | free-text `uncertainty` at best | n/a (not a runtime class) | none |
| No results | exit **1** (`ghx.go:321…`) | bare string / empty result | opaque string | opaque string | n/a | n/a | none |
| Bad input | exit **2** (`ghx.go:415`, substring-derived) | bare string | opaque string; **one** structured case (`serve.go:193`) | opaque string | n/a | n/a | none |
| Turn liveness timeout | n/a | n/a | n/a | opaque string | not distinguished | `LivenessTimeoutMarker` string (`acp.go:58`) | `episode_hang_timeout` (`anomalies.go:79`) |
| Turn hit max-turns | n/a | n/a | n/a | opaque string | not distinguished | `maxTurnsErrorMarker` string (`acp.go:61`) | `turn_cap_wrapup` (`anomalies.go:74`) |

The three taxonomies:
1. **CLI GitHub-upstream** — 5 hint classes, `errors.go:101-122`, substring-matched from core English.
2. **Runtime liveness/turn** — ~4 markers, `acp.go:41-96`, substring-matched from adapter English.
3. **Eval anomaly buckets** — 12 buckets, `anomalies.go:30-89`, substring-matched from persisted turn errors.

They do not share a type, do not interoperate, and each independently re-parses
strings the process itself produced.

---

## 3. What a unified core `FailureClass` must PRESERVE, per frontend

ADR-0034's contract is "lift the class into core, map once per frontend." Each
map must preserve:

- **CLI** — the exact class→exit integers **None→0, NoResults→1, BadInput→2, Upstream→3** (`errors.go:9-18`; tests `ghx_test.go:124-140`, `tier2_test.go:96-136`); the **default `ExitBadInvocation`=2 for any untyped error** (`errors.go:59`, `ghx_test.go:139`); the `affordanceMarker "→ "` on its own trailing line (`errors_test.go:54-73`); verbatim raw text for unmatched upstream (`errors_test.go:77-86`); and the 5 hint substrings (`errors_test.go:12-38`).
- **MCP direct** — the `{IsError:true, text:…}` shape. Structuring must be **additive** and must not flip `IsError` (asserted `sidecar_test.go:310,368,397`; `reportsink_test.go:370,390`; `reportsink_stdio_test.go:59,75,97`).
- **MCP recon** — the tool **schema** byte-identical (`reconToolSchema` hash, `host_identity.go:44-48`, `host_episode_test.go:293-298`). The report JSON is *not* hashed, so a class field is permitted but is a consumer-visible content change (map additively).
- **Sidecar report** — keep all existing keys incl. `answer`/`uncertainty` (`report.go:38-56`; `report_test.go` goldens); a class is a **new additive field** (lenient parser tolerates it).
- **Sidecar runtime** — preserve the `errors.Is` chain (`ErrLivenessTimeout`, `acp.go:73`; `DiagnoseTurnError`, `agentdiag.go:515`) **and the marker substrings** (`acp.go:58,61,64`), because the eval layer derives anomalies from those exact strings in committed artifacts (`acp.go:41-47`, `anomalies.go`).

---

## 4. BREAK flags — where a naïve ADR-0034 refactor would break an existing contract/test

- **B1 — Affordance format contract.** `errors_test.go:54-73` pins exit 3 + verbatim raw + trailing `"→ "` hint line. If upstream classification moves to core and the hint text moves with it, the CLI map must still reproduce this format exactly. Contract: `internal/cli/errors_test.go:54-73`.
- **B2 — Verbatim-passthrough contract.** `errors_test.go:77-86` requires an unmatched upstream error's `err.Error()` to equal the raw text. A core `ghx.Error{Class,Err,Hint}` that always re-renders (`"[upstream] …"`) would break this unless `Error()` returns the wrapped text unchanged for the no-hint case. Contract: `internal/cli/errors_test.go:83`.
- **B3 — The substring coupling must be *rewired*, not merely *supplemented*.** `internal/cli/ghx.go:415` still substring-matches `"invalid repo"`/`"query must not be empty"` against core's `inspect.go:245,249`. If the migration adds `ClassBadInput` in core but leaves `ghx.go:415` matching strings, a later cleanup that deletes those substrings (believing the class covers them) silently regresses "invalid repo" from exit **2** to the `upstreamError` fallback exit **3**. The phase-2 change must replace the substring test with a class read in the same commit. Coupling: `internal/cli/ghx.go:415` ↔ `internal/ghx/inspect.go:245,249`.
- **B4 — Marker strings are an artifact-recompute contract, not just plumbing.** `evals/anomalies.go:30-89` derives anomalies from persisted turn-error strings via the `acp.go:41-47` markers, and the Visibility/Truthfulness tenet requires every score recomputable from committed artifacts. A refactor that *replaces* marker strings with a typed field — and stops emitting the substrings — breaks anomaly detection on both new and **historical** runs. The class must be threaded **in addition to**, not **instead of**, the marker substrings. Contract: `internal/sidecar/acp.go:41-64` + `internal/sidecar/evals/anomalies.go:30-89`.
- **B5 — Recon identity hash.** Structuring error *payloads* is safe (schema-only hash, §1d). But if the class is also threaded into `ReconMCPTool()` (a new param/description), the `reconToolSchema` hash breaks and every host-task baseline must re-ground. Keep `internal/sidecar/recontool.go:21-28` frozen. Contract: `host_identity.go:44-48`, `host_episode_test.go:293-298`.
- **B6 — MCP `IsError` boolean.** Structured error payloads must keep `IsError=true` on failure / `false` on success. Contracts: `sidecar_test.go:310,368,397`; `reportsink_test.go:370,390`; `reportsink_stdio_test.go:59,75,97`.

---

## 5. ADR-0034 factual corrections (its own magnitude claims vs. `47e2b7c`)

- **"16 `NewToolResultError(err.Error())` sites"** (ADR lines 44, 79): `serve.go` has 16 `NewToolResultError` calls, but only **15** are `err.Error()` flattenings; `serve.go:193` is already a structured bad-input message. The ADR also omits the **3** MCP-error sites in `reportsink.go` (`:356,360,365`). Net MCP error surface = **19**, of which ~16 are opaque. Minor, but it changes the "replaces the 16 opaque flattenings" scope line.
- **"~18 bare `fmt.Errorf`/`errors.New`" in core** (ADR line 41): **confirmed exactly 18** (§1b).
- **"83 `fmt.Errorf` across 56 files" in the sidecar** (ADR line 98): **stale/undercounted.** Current `fmt.Errorf` = **108 across 26 files** in the sidecar runtime proper (excl. `evals/`), or **306 across 57 files** including `evals/`. The "56 files" ≈ the 57 files incl. evals, but "83" undercounts the runtime footprint by ~1.3x (excl. evals) to ~3.7x (incl. evals). Cost scoping should use the current figure — while noting the ADR's own caveat that only the agent-visible subset needs the class, so raw count overstates true migration scope.
- **Task-prompt path**: `internal/cli/inspect.go` does not exist; the coupling is `internal/cli/ghx.go:415` + `internal/ghx/inspect.go` (§Scope correction).

---

## 6. Risks & open questions for Goga

1. **ADR-0034 unifies *one* taxonomy, but the code has *three*.** The proposed
   `FailureClass{None,NoResults,BadInput,Upstream}` mirrors the CLI's 4 and does
   **not** subsume the runtime liveness/turn classes (`acp.go:58,61,64`) or the
   12 eval anomaly buckets (`anomalies.go:30-89`). Decision needed: is
   `FailureClass` strictly the *recon-operation outcome* class (what the CLI
   has), leaving runtime-resilience and eval-anomalies as separate axes? Or a
   two-axis model? The ADR should name which taxonomy it unifies and which it
   deliberately leaves alone, so phase 4 ("thread the class into the sidecar
   report") does not accidentally imply collapsing the ADR-0027 taxonomy.

2. **"Fragility removed" vs. artifact-recompute — these can conflict.** The ADR's
   headline win is "classification stops depending on re-parsing English error
   strings." But the eval anomaly layer *depends on those exact strings by
   design* (`acp.go:41-47`, Visibility/Truthfulness). You cannot both delete the
   substrings and keep artifact-derivable anomalies. Recommend the migration
   keep marker substrings in the emitted error text (typed **and** string-tagged)
   as an explicit compatibility contract (see B4).

3. **Is core's structured HTTP status actually available at the 18 sites?** The
   ADR wants upstream sub-classification moved into core "where the HTTP
   status/response is actually known." But the 18 core sites (§1b) build bare
   `fmt.Errorf` from `gh`/GraphQL *string* output. Before promising
   "structured signals rather than re-parsed English," phase 1 needs a
   spot-check of the gh/GraphQL client wrapper: if core does not retain the
   status code at the error-construction points, core would end up
   substring-matching too — relocating the fragility, not removing it. This is
   the single biggest unverified assumption in the ADR.

4. **Mixed surface during migration.** `serve.go:193` (structured) alongside 15
   flattened siblings shows the surface will be inconsistent mid-migration; an
   agent parsing MCP errors sees both shapes until phase 3 completes. Acceptable
   under the phased plan, but worth stating as an interim contract.

5. **Recon report additive field is consumer-visible.** Threading a class into
   `Report` changes the JSON `serve --recon` returns (`serve.go:207`). No
   test/golden pins recon output exactly (safe for CI), and host-task graders
   key on workspace outcome + tool attribution, not report field-set (no grader
   found parsing `Report` fields) — but confirm no external main-agent consumer
   keys on the current field-set before shipping phase 4.

---

## Evidence Contract

- **Basis:** worktree fast-forwarded v2.6.0 `e8816ec` → mainline `47e2b7c`
  (clean ancestor→tip ff; mainline ref untouched, nothing merged/pushed). All
  file:line references resolve at `47e2b7c`.
- **Who produced this:** read-only audit worker; no code changed except this
  file under `docs/audits/`.
- **How to recompute the counts:**
  - CLI exit-code / affordance call sites: `grep -rn 'WithExitCode(Exit\|upstreamError(err)\|CodeForError' internal/cli cmd | grep -v _test`
  - MCP flatten sites: `grep -c 'NewToolResultError' internal/cli/serve.go` (16); `grep -n 'NewToolResultError' internal/sidecar/reportsink.go` (3)
  - Core bare errors: `grep -rc 'fmt.Errorf\|errors.New' internal/ghx | grep -v _test` (sums to 18)
  - Sidecar `fmt.Errorf`: `grep -rn 'fmt.Errorf' internal/sidecar --exclude-dir=evals | grep -v _test | wc -l` (108, 26 files); include evals → 306, 57 files
  - Resilience markers: `grep -nE 'LivenessTimeoutMarker|maxTurnsErrorMarker|peerClosedMarker|Is(MaxTurns|PeerClosed|LoadSession)' internal/sidecar/acp.go`
  - Anomaly buckets: `grep -nE 'Anomaly[A-Z][A-Za-z]+ =' internal/sidecar/evals/anomalies.go` (12)
- **Tests exercised:** `go test ./internal/cli -run 'Upstream|Affordance|CodeForError|ExitCode'` → `ok` at `47e2b7c` (pins B1/B2/B3-adjacent behavior).
</content>
</invoke>
