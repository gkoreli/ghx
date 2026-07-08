# Dogfood Friction Log (M5 exit bar — ADR-0019)

Declarative log of friction found while dogfooding ghx-with-sidecar for
daily development. Friction items outrank speculative features. M5 closes
when a week of daily use is logged here and every **breaking** item is
fixed or explicitly deferred with rationale.

Severity mirrors the eval anomaly taxonomy:

- **breaking** — blocked the task or forced a fallback to the bare CLI.
- **soft** — worked, but ground (extra steps, confusion, wasted tokens).

Demand tags (pre-registered triggers — add to the entry when they apply):

- `tier2-demand` — the question needed deeper structural understanding
  than remote evidence gives (feeds M7 escalation tiers).
- `a2a-demand` — an orchestrator-shaped consumer wanted to talk to the
  sidecar as an agent (feeds ADR-0020.1 D3).

Once ADR-0022 lands, cite the session trace (`~/.ghx/sessions/<session>/`)
in the entry — friction reports with traces are evidence, not anecdotes.

Entry format:

```
## YYYY-MM-DD <short title> — <breaking|soft> [tags]
- Attempted: <what you asked / ran>
- Ground: <what happened vs what should have>
- Trace: <session dir or "pre-ADR-0022">
- Disposition: <open | fixed <commit> | deferred: <rationale>>
```

---

## 2026-07-05 cross-repo discovery gap — soft [tier2-demand]
- Attempted: "find open-source prior art similar to our agent sidecar framework and evals" — a discovery question spanning unknown repos.
- Ground: `sidecar ask` is repo-scoped (`--repo` required); the candidate list had to come from the operator's own knowledge/web search first. The sidecar can answer "how does X work in repo Y" but not "which repos do X" — reconnaissance starts one step before the sidecar can.
- Trace: n/a (question never reached the sidecar).
- Disposition: fixed 8bc21e5 — ADR-0019.1 discovery tier: `--repo` is now optional scope; without it the sidecar runs discovery reconnaissance (question-derived session slug, discovery persona doctrine, repo-level `owner/repo` citations).

## 2026-07-05 ACP agent setup is the hardest step — soft
- Attempted: first-time production setup (`~/.ghx/config.json`).
- Ground: the default agent `claude` fails the ACP handshake (correct fail-fast), but the fix requires knowing about `@agentclientprotocol/claude-agent-acp` and hand-writing a wrapper command; the only working wrapper lives in this repo's `scripts/` for evals. A real user has neither. `config init` detection can only find agents that already speak ACP on PATH.
- Trace: doctor run (preflight PASS after manual config).
- Disposition: fixed 61a0e3c — `ghx sidecar config init --claude-acp` writes the pinned npx adapter command line (same pin as scripts/eval-agent-acp.sh); agent command lines are split into argv so no wrapper script is needed; README documents install → init → doctor → ask.

## 2026-07-05 report-sink depends on a current ghx binary — soft
- Attempted: real asks with PATH ghx 2.1.13 (released) while the sidecar features live on unreleased mainline.
- Ground: the submit_report MCP server is served by the ghx binary itself; a stale binary silently degrades the contract to the text fallback (loud stderr warning exists, but only if you read stderr). Needed manual `GHX_REPORT_SINK_EXE` to a fresh build.
- Trace: session dirs under ~/.ghx (see reports/ presence as the tell).
- Disposition: fixed 61a0e3c — `ghx sidecar doctor` now resolves the sink-serving binary exactly like the session wiring (GHX_REPORT_SINK_EXE, else current executable), invokes its `version`, and fails loudly with remediation on mismatch or a missing/stale binary. The release-cadence question stays open but is no longer silent.

## 2026-07-05 max-turns exhaustion destroys the exploration — breaking
- Attempted: `sidecar ask --repo Arize-ai/phoenix "how does LLM-as-judge eval work..."` (large repo, sprawling internals) at default depth.
- Ground: the agent explored ~20+ tool calls, then the adapter killed the prompt with `Internal error: Reached maximum number of turns (24)`. The entire exploration was lost — no report, not even a partial. The budget safety net fires as a hard error instead of forcing a "submit what you have" wrap-up.
- Trace: ~/.ghx/sessions/arize-ai-phoenix/ — see next entry.
- Disposition: fixed ae832b3 (ADR-0027 D1) — the runtime LoadSession-resumes the same ACP session on the max-turns error and sends one "wrap up: call submit_report now with what you have" prompt (fresh query = fresh turn budget); recovery recorded as `wrapUpRecovered` + soft anomaly `turn_cap_wrapup`; a failed wrap-up terminates as a BLOCKED report with artifacts instead of a lost exploration.

## 2026-07-05 failed turns leave zero artifacts — breaking
- Attempted: audit the failed phoenix ask above.
- Ground: the session dir has only meta.json — no traces.jsonl, no logs, nothing. Emission happens after a successful turn, so exactly the turns that fail (the ones most needing audit) are invisible. Violates the visibility tenet at its most valuable moment.
- Trace: ~/.ghx/sessions/arize-ai-phoenix/ (absence of artifacts is the evidence).
- Disposition: fixed ae832b3 (ADR-0027 D3) — emission now runs on every Ask exit path (turn-cap, watchdog, peer-closed, terminal WARN) under a non-cancellable context; failed turns leave traces.jsonl + logs.jsonl with a `sidecar.turn.error` record, pinned by `TestAskEmitsArtifactsOnFailedTurn`.

## 2026-07-05 live progress stream is opaque — soft
- Attempted: watching two `--depth deep` asks live (ADR-0026 second wave: letta-ai/letta ~95s, mastra-ai/mastra ~240s).
- Ground: stdout during the run is almost entirely repeated `▶ execute: Terminal (pending)` lines — the mastra ask printed ~48 identical ones — with no command text, no completion status, and only the rare agent-thought line breaking the monotony. The operator (or a parent agent tailing the child) gets near-zero signal about what the delegate is doing until the final answer lands; auditing mid-flight means separately tailing `traces.jsonl`, which does have the per-tool detail. The visibility exists in the artifacts but not on the consumption surface while it matters.
- Trace: ~/.ghx/sessions/letta-ai-letta/, ~/.ghx/sessions/mastra-ai-mastra/ (contrast stdout vs traces.jsonl richness).
- Disposition: open — render tool-call titles (command text, resolved status) in the progress stream instead of the bare pending marker.

## 2026-07-05 SDK warning noise on every ask — soft
- Attempted: every `sidecar ask` in the ADR-0026 second wave.
- Ground: each run opens with a Node `CLAUDE_SDK_CAN_USE_TOOL_SHADOWED` warning about `mcp__ghx-report-sink__submit_report` being auto-approved by a bare allowedTools entry ("canUseTool will not be invoked..."). Harmless — the auto-approve is intentional — but it reads like a security misconfiguration to a first-time user and pollutes captured output.
- Trace: ~/.ghx/sessions/{letta-ai-letta, mastra-ai-mastra}/ (warning is on the ask's stderr, before the session stream).
- Disposition: open — either configure the adapter so the warning path isn't triggered (PreToolUse hook / drop the bare name) or suppress/explain it in the ask output.

## 2026-07-06 relevantFiles lose their repo after a scope drift — soft
- Attempted: `sidecar ask --repo All-Hands-AI/OpenHands "how does agent delegation work..."` (ADR-0026 third wave).
- Ground: the agent correctly discovered the code had moved (`README`: agent source lives in `OpenHands/software-agent-sdk`) and did all its digging there — good recon. But the report schema has no per-file repo qualifier: `relevantFiles` came back as bare paths (`openhands-tools/openhands/tools/delegate/impl.py`) while `meta.json` still pins `"repo": "All-Hands-AI/OpenHands"`. Any consumer resolving those paths against the session's repo gets 404s; the true repo is only recoverable by parsing prose in the `verified` items ("ghx read OpenHands/software-agent-sdk ..."). Discovery-tier reports already prefix paths with `owner/repo:` — the repo-scoped report format silently assumes the scope never drifts.
- Trace: ~/.ghx/sessions/all-hands-ai-openhands/ (compare reports/1-*.json `relevantFiles` vs meta.json `repo`).
- Disposition: open — add an optional `repo` qualifier to relevantFiles/evidence entries (or adopt the discovery tier's `owner/repo:path` convention everywhere) so cross-repo evidence stays resolvable.

_Setup for the week: `go build -o ghx ./cmd/ghx`,
`./ghx sidecar doctor`, then either `ghx sidecar ask --repo <owner/repo>
"<question>"` directly or wire `ghx serve --recon` into your agent's MCP
config and let the recon skill do the talking._

## 2026-07-06 stale ACP session ID kills the ask — breaking
- Attempted: `sidecar ask --repo gin-gonic/gin` (default session `gin-gonic-gin`, which existed from an earlier smoke with a prior binary).
- Ground: the runtime resumed the persisted ACP session ID from meta.json; the adapter answered `Resource not found` (-32002) and the whole ask failed. A LoadSession miss must fall back to a fresh ACP session (the ledger already carries cross-turn context) — resume is an optimization, never a hard dependency (ADR-0027 spirit).
- Trace: ~/.ghx/sessions/gin-gonic-gin/ (meta.json holds the dead session ID).
- Disposition: fixed aeb86c3 — on LoadSession Resource-not-found (-32002), `Ask` now creates exactly one fresh ACP session, continues the turn with the named-session ledger prompt, persists the new ACP session ID, and records the soft downgrade as `SessionRecreated` plus a `sidecar.session.recreated` session log entry.

## 2026-07-06 baseline-reuse refusal is silent and falls back to the expensive path — soft
- Attempted: first live reuse run (`GHX_EVAL_BASELINE_REUSE_RUN_DIR` at the fixbatch run) to measure the ADR-0029 persona with reused baselines.
- Ground: eligibility refused (reason not surfaced anywhere — manifest has no baselineReuse block, no log line) and the runner silently ran fresh plain+ghx+sidecar — the 3x-more-expensive shape the caller explicitly tried to avoid. Refusal must print WHICH check failed and require explicit opt-in before fresh-baseline fallback.
- Trace: internal/sidecar/evals/.ghx-evals/runs/gate-run-2026-07-06-persona-reuse/ (quarantined diagnostic round, 18 eps, not a verdict).
- Disposition: open — diagnose eligibility failure (suspect: prior-run mapengine trial count 7>5 from top-ups, or relative run-dir path handling) + make refusal loud.

---

_2026-07-07 dogfood run (worker w3): depth dial + A4 affordances + ADR-0032.1
Locations, on two repos NOT previously used — `charmbracelet/bubbletea` (Go TUI)
and `pallets/click` (Python CLI). Binary built from this worktree's HEAD
(`e8816ec` = release **v2.6.0**), used only as `/tmp/ghx-dogfood-w3`. Doctor:
preflight PASS (gh-token via GH_TOKEN, ACP handshake OK, sink served by the
dogfood binary; codemap optional-missing). Agent = scripts/eval-agent-acp.sh
(real Claude Code ACP adapter, claude-sonnet-5). Every ask returned exit 0 with
accurate, file-cited answers — the happy path is solid; items below are edges._

## 2026-07-07 depth dial produces a real evidence gradient — confirmation
- Attempted: identical question ("How does the Program event loop dispatch messages to a Model's Update method, and where is that wired?") on `charmbracelet/bubbletea` at `--depth cheap|normal|deep`, pinned to fresh sessions (`bt-cheap|bt-normal|bt-deep`) to defeat context carryover.
- Ground (works as designed): monotonic gradient in evidence breadth — files traced 1→2→3 (tea.go; +input.go; +input.go+tty.go), verified bullets 3→3→4, report.size 3092→3572→3934 bytes (metrics.jsonl), latency 47.6→48.2→54.0s. Deep alone surfaced the tty.go `initInputReader` input goroutine and added an Uncertainty caveat. The dial visibly buys depth for a modest (~13%) latency cost.
- Trace: ~/.ghx/sessions/{bt-cheap,bt-normal,bt-deep}/ (compare reports/*.json relevantFiles + metrics.jsonl report.size).
- Disposition: confirmation — depth dial is a genuine, observable knob.

## 2026-07-07 cross-language recon + structured citations hold up — confirmation
- Attempted: `sidecar ask --repo pallets/click --depth normal "How does the @click.command decorator turn a function into a Command...?"` (Python, first use of this repo).
- Ground: accurate answer citing exact symbols/lines (decorators.py `command()` L168, `_param_memo` L314). Report JSON carries the citations structurally, not just in prose: `report.commandsRun` lists the real invocations (`ghx inspect ...`, `ghx read ... --lines 137-260`), each `report.verified[].evidence` names the exact `ghx read --lines` command + what it showed, `report.relevantFiles` lists paths. Task-4a evidence bar (answers cite files AND commands) is met.
- Trace: ~/.ghx/sessions/click-normal/reports/1-*.json.
- Disposition: confirmation.

## 2026-07-07 A4 affordances name the fix on invocation errors — confirmation
- Attempted: `ghx search charmbracelet/bubbletea "x" --repo ...` (wrong flag) and a 0-result search.
- Ground: unknown-flag error printed "unknown flag --repo; use --help, e.g. ghx search --help" (exit 2) — names the exact next command (internal/cli/ghx.go:524-526); a 0-result search printed "→ To search repos by topic, use: ghx repos \"<query>\"". Both point at the fix, as A4 intends. Bare-cobra paths (`explore` no-args, unknown flag) correctly return exit 2.
- Trace: n/a (invocation errors, no session).
- Disposition: confirmation.

## 2026-07-07 `explore` maps a malformed slug to exit 3 (upstream) not 2 (bad invocation) — soft
- Attempted: `/tmp/ghx-dogfood-w3 explore badslug`.
- Ground: prints "invalid repo format, expected owner/name" but exits **3** (ExitUpstreamFailure). A malformed slug is a bad *invocation* → should be exit **2**. `inspect` already special-cases this exact error to `ExitBadInvocation` (internal/cli/ghx.go:415-416), but `exploreCmd` blankets every error to `ExitUpstreamFailure` (internal/cli/ghx.go:100), so the two commands disagree on the same error class. A scripting agent that distinguishes "my args were wrong" (2) from "GitHub is down" (3) is misled by `explore`. The message also names no example invocation.
- Expected: exit 2, ideally with an example (`ghx explore owner/name`).
- Suggested fix: give `exploreCmd` (and `read`/`tree`/`grep` if they share the gap) the `strings.Contains(err, "invalid repo format") → WithExitCode(ExitBadInvocation, ...)` guard that `inspect` already has.
- Trace: n/a (invocation error).
- Disposition: fixed 67ddf6d (ADR-0034 phase 2). Already exit 2 on mainline post-v2.6.0 (the dogfood binary was behind); now every repo-scoped command (explore/read/tree/grep/inspect) sources bad-input from `ParseRepo`'s core `ClassBadInput` rather than a per-command substring guard. Output byte-identical.

## 2026-07-07 `read` of a nonexistent repo returns exit 0 — soft
- Attempted: `/tmp/ghx-dogfood-w3 read totally/nonexistent-repo-xyz123 README.md`.
- Ground: prints "=== README.md (not found) ===" and exits **0**. A scripting agent checking `$?` reads this as success and would proceed on empty content. A nonexistent repo/ref or an unresolved requested file should not report OK.
- Expected: exit 1 (ExitNoResults) when no requested file resolves, or exit 3 when the repo/ref 404s.
- Suggested fix: in the read command, when zero requested files resolve (or the repo/ref lookup 404s), return `ExitNoResults`/`ExitUpstreamFailure` instead of nil.
- Trace: n/a (invocation error).
- Disposition: fixed f029add + 67ddf6d (ADR-0034 phases 1–2). Core `Read` no longer swallows a repo/ref-level GraphQL failure to `NotFound`+nil — it returns a classified upstream error, so `read totally/nonexistent-repo-xyz123 README.md` now exits 3 (404). When the repo resolves but zero requested paths do, the CLI returns `ExitNoResults` (exit 1) after still printing the per-file `(not found)` lines.

## 2026-07-07 live 401 on `search` prints raw HTTP with no fix-it affordance — soft [A4 gap]
- Attempted: `GH_TOKEN=ghp_invalidbadtoken... /tmp/ghx-dogfood-w3 search charmbracelet/bubbletea "func eventLoop"`.
- Ground: exit **3** (correct) but the message is the bare upstream string "search failed: HTTP 401: Bad credentials (https://api.github.com/...)" — no affordance. `sidecar doctor`/preflight already owns the right remediation ("Run `gh auth login`, or set GH_TOKEN/GITHUB_TOKEN"; internal/sidecar/preflight.go:148), but the hot-path command error doesn't surface it. The A4 promise (errors name the fix) is unmet on the single most common live failure. Separately: an *empty* `GH_TOKEN=` degraded to exit 1 "0 results" (go-gh fell back to the `gh` keyring token), i.e. missing-auth can masquerade as no-results — no clean auth-absent signal.
- Expected: 401/403 upstream errors name `gh auth login` / GH_TOKEN as the fix.
- Suggested fix: map 401/403 in the CLI error path to an affordance reusing the preflight.go remediation string.
- Trace: n/a (live upstream error, no session emitted).
- Disposition: fixed 67ddf6d (ADR-0034 phase 2). Already carried the `gh auth login` affordance on mainline post-v2.6.0; the hint is now selected by the core classifier from the structured HTTP status (`*api.HTTPError.StatusCode`) rather than a CLI substring table, and the auth-hint wording mirrors the preflight remediation. Exit 3 + affordance byte-identical. Note: the empty-`GH_TOKEN=` masquerade is go-gh credential resolution (an empty env var is ignored in favor of the stored `gh` login) — correct when a login exists, and not a classification bug; left as-is.

## 2026-07-07 tool-call Locations empty — binary predates the S2 fix; and the fix may not receive live data — soft [ADR-0032.1]
- Attempted: verify Task-4b — that ToolCallTrace.Locations is populated on file/read tool calls (the 5aec071 / ADR-0032.1 S2 fix). Checked traces.jsonl across all four sessions.
- Ground (two findings):
  (1) SETUP MISMATCH — this worktree's HEAD is `e8816ec` = release **v2.6.0**, one release behind mainline (v2.7.0). `git merge-base --is-ancestor 5aec071 HEAD` = **NO**: the S2 fix is *not* in the dogfood binary. In this binary's acp.go the `u.ToolCall`/`u.ToolCallUpdate` handlers (lines 530-576) never write `tr.Locations`. So the empty Locations I observe (0 location attrs across bt-cheap/bt-normal/bt-deep/click-normal) are expected-for-this-binary, **not** a mainline regression. A dogfood worker sent to verify a mainline fix was handed the pre-fix binary — cut dogfood worktrees from mainline HEAD, not a tag.
  (2) NUANCE the fix doesn't cover — mainline's `mergeLocations` (denyclient.go:232,256) only populates from whatever the ACP notification carries (`tc.Locations`/`tcu.Locations`). All 12 tool calls in every session are `kind=execute` (the agent shells out to `ghx read/search/inspect` via the ACP execute tool, not native ACP read/search tools). ACP execute-tool notifications generally don't carry file `locations`, so R6 path-scope likely still sees empty Locations in real sidecar usage. The fix's tests (acp_test.go) prove population from *synthetic* Locations; nothing proves the live Claude Code adapter emits Locations for the sidecar's execute-driven recon.
- Expected: on a fix-bearing binary, read/search-shaped tool calls populate Locations that R6 can scope on.
- Suggested fix: (process) build dogfood binaries from mainline HEAD so the binary matches the fix under test; (product) add a live assertion that the ACP adapter actually emits `tc.Locations` for the sidecar's tool-call pattern — if execute-kind calls never carry them, derive path-scope from the `ghx` argv in `tr.Title`/`RawInput` rather than relying on ACP Locations.
- Trace: ~/.ghx/sessions/{bt-cheap,bt-normal,bt-deep,click-normal}/traces.jsonl (0 `"locations"` attrs; all spans `ghx.sidecar.tool.kind=execute`).
- Disposition: open.

## 2026-07-07 sidecar token metric undercounts a turn's real cost — soft [observability]
- Attempted: measure per-depth *cost* from metrics.jsonl `gen_ai.client.token.usage`.
- Ground: input is a constant **102** tokens across cheap/normal/deep; output 812/145/170; reasoning absent/1057/554. These are far too small (and too input-flat) to be the 12-tool exploration — they reflect only the sidecar's own orchestration model, not the subject ACP agent (claude-sonnet-5) that does the actual work. Depth's effect on real token cost is therefore not observable from the committed artifacts, which pinches the depth-vs-cost half of the depth-dial story and the visibility tenet.
- Expected: a metric from which a reader can recover what a depth level actually cost.
- Suggested fix: surface the subject agent's token usage if the ACP adapter reports it, or relabel the metric so it's unambiguous it excludes subject-agent cost.
- Trace: ~/.ghx/sessions/{bt-cheap,bt-normal,bt-deep}/metrics.jsonl.
- Disposition: open.

<!-- ============================================================= -->
<!-- 2026-07-07 — v2.8.0 dogfood run (depth dial, Snapshot, panic   -->
<!-- fix, daemon). Binary: `go build -o /tmp/ghx-dogfood-280`,      -->
<!-- worktree HEAD 0a7ac7f = release tag v2.8.0 (ADR-0036 B2 commit -->
<!-- e4d7a1f present). ACP agent = production eval-agent-acp.sh.    -->
<!-- Unfamiliar repos: tidwall/gjson (Go), sindresorhus/p-queue     -->
<!-- (TS), python-attrs/attrs (Python).                             -->
<!-- ============================================================= -->

## 2026-07-07 v2.8.0 cross-language recon (3 unfamiliar repos, 3 depths) all land with cited evidence — confirmation
- Attempted: one real recon ask per repo, one per depth tier — `gjson --depth cheap` ("how does gjson parse a path expression without unmarshalling the whole doc"), `p-queue --depth normal` ("how is the concurrency limit enforced / which task runs next"), `attrs --depth deep` ("how is __init__ generated, exec-vs-slots decision"). All first-time repos.
- Ground (works as designed): all three returned accurate answers with structured citations — every `verified[]` bullet names the exact `ghx read … --lines A-B` (or `ghx inspect`) command and what it showed, `relevantFiles` lists real paths, and each ends with the `artifacts: <session dir> (trace <id>)` footer. gjson → gjson.go parseObjectPath/parseObject co-parse (L982/L1196); p-queue → `#tryToStartAnother`/`PriorityQueue.dequeue` (source/index.ts, source/priority-queue.ts); attrs → `_attrs_to_init_script`+`_compile_and_eval` exec, slots only picks `_create_slots_class` vs `_patch_original_class` (src/attr/_make.py). Each carried an honest Uncertainty section. Task-1 + Task-4a (answers cite files AND commands) met on all three. All exit 0.
- Trace: ~/.ghx/sessions/{tidwall-gjson,sindresorhus-p-queue,python-attrs-attrs}/reports/*.json (traces 71b9c57…, 5984e38…, b9757af…).
- Disposition: confirmation.

## 2026-07-07 depth dial (cheap|normal|deep) exercised end-to-end — confirmation, with a latency gradient
- Attempted: `sidecar ask --depth {cheap,normal,deep}` across the three repos above (one tier each).
- Ground: all three tiers run cleanly (exit 0). Latency gradient consistent with a widening command budget: **cheap 40.7s → normal 60.9s → deep 106.2s** (`time` wall clock). `--depth` is a real, documented flag (`Command budget: cheap|normal|deep`, default normal) and the on-disk artifacts show a per-turn `budget:` field. This complements the earlier same-question depth study (bt-cheap/normal/deep) — the dial is a genuine knob across repos too.
- Trace: session dirs above; grep `budget` in *.jsonl.
- Disposition: confirmation.

## 2026-07-07 ADR-0036 B2 Snapshot{Repo,SHA} is live in read/explore result JSON — confirmation
- Attempted: verify Task-2 — that read/explore now carry a resolved commit SHA. Probed the agent-facing code-mode surface: `ghx code 'var r = codemode.explore({repo:"tidwall/gjson"}); return {keys:Object.keys(r), snapshot:r.snapshot};'` and the same for `read({repo, files:["go.mod"]})`.
- Ground: both carry it. explore → `"snapshot":{"repo":{"Name":"gjson","Owner":"tidwall"},"sha":"7d8b3821e9d2acf35e8a226b63fcf801078e9b96"}`; read → same snapshot on each FileResult. The SHA **matches** `gh api repos/tidwall/gjson/commits/master --jq .sha` (7d8b3821…) — provenance is real and correct, not a placeholder. Source: internal/ghx/explore.go:99,142 and read.go:136,141 populate `Snapshot{Repo, SHA: …DefaultBranchRef.Target.OID}`; type at internal/ghx/repo.go:17; unit test TestReadCarriesResolvedSnapshotSHA (client_test.go:111).
- Trace: reproduce with the two `ghx code` snippets above.
- Disposition: confirmation.

## 2026-07-07 Snapshot SHA is computed but under-surfaced — an agent isn't told it exists — soft [ADR-0036 B2]
- Attempted: find every surface where a consuming agent/human could observe the resolved SHA it now pays to compute.
- Ground (five gaps that blunt the re-fetchability win B2 exists for):
  (1) **code-mode `--list` type stubs omit it.** `ghx code --list` declares `explore: (input) => { description; branch; files[]; readme }` and `read: (…) => { path; content; byteSize; notFound; … }[]` — **no `snapshot` field** on either. The runtime JSON carries `snapshot`, but the very type contract handed to the agent hides it, so a stub-trusting agent never reads `r.snapshot.sha`.
  (2) **register.go `Returns:` schema omits it** too (internal/ghx/register.go:11 = `{ description; branch; files[]; readme }`) — same omission at the MCP tool-declaration layer.
  (3) **Human CLI text output omits it.** `ghx explore tidwall/gjson` and `ghx read …` print no SHA; there is **no `--json`** on read/explore to extract it (`read --json` → "unknown flag --json; use --full"). A human dogfooding the bare CLI cannot see which commit was read.
  (4) **Not in any session artifact.** The resolved SHA (7d8b3821…) appears in **zero** of the gjson session's traces/logs/live/metrics jsonl — the sidecar consumes CLI *text* (which lacks it), so provenance never reaches the committed evidence a judge would audit.
  (5) **sidecar `Evidence` struct still `{Source, Summary}`** (internal/sidecar/report.go:31) — no SHA field. ADR-0036 B2's literal deliverable ("thread the resolved tip SHA into `Evidence`") is not implemented; ADR-0036 is status "proposed". Minor: nested `repo` serializes PascalCase `{"Name","Owner"}` while every sibling field is lowercase (`sha`, `branch`) — a JSON-casing inconsistency (the `ghx.Repo` struct lacks json tags).
- Expected: the surface that ships the value should advertise it — snapshot in the code-mode/`Returns` type stubs, threaded into `Evidence`, and reachable from the CLI (a `--json` on read/explore, or a `snapshot:` line in text). Lowercase the nested `repo` keys.
- Suggested fix: add `snapshot` to the register.go `Returns`/type-stub schema for explore+read; add the SHA to `sidecar.Evidence` and emit it in tool-call traces; add json tags to `ghx.Repo` (`json:"name"`,`json:"owner"`); offer `--json` (or a provenance footer) on read/explore.
- Trace: `ghx code --list`; internal/ghx/register.go:11; internal/sidecar/report.go:31; ~/.ghx/sessions/tidwall-gjson/*.jsonl (no SHA).
- Disposition: open.

## 2026-07-07 `tree`/`explore`/`read`/`inspect` all exit 2 on a malformed slug (2.8.0 panic fix) — confirmation, resolves a prior open item
- Attempted: verify Task-3 — the 2.8.0 panic fix and unified bad-invocation exit code. `ghx tree noslash`, and `explore|read|inspect` on `badslug`.
- Ground: `ghx tree noslash` → exit **2**, no panic, message "invalid repo \"noslash\": expected owner/repo, e.g. ghx explore gkoreli/ghx" (names an example invocation = A4 affordance). `explore badslug`, `read noslashhere x`, `inspect badslug concern` **all** exit **2** with the same shaped message. This **resolves** the earlier open item ("`explore` maps a malformed slug to exit 3 not 2", logged on the v2.6.0 binary): the CHANGELOG [2.8.0] "tree/explore/read no longer panic or misreport a malformed repo" fix has landed and is now consistent across commands.
- Trace: n/a (invocation errors, deterministic).
- Disposition: confirmation (supersedes the 2026-07-07 w3 "explore → exit 3" entry above).

## 2026-07-07 daemon auto-spawns and stays warm; supervisor-hardened asks "just work" — confirmation (+ log-path friction)
- Attempted: verify Task-4 daemon behavior. Ran the three live asks cold (no daemon at start), then checked process + logs.
- Ground: a warm daemon **auto-spawned** — `pgrep` shows `54167 /tmp/ghx-dogfood-280 sidecar daemon --background` (spawned by the ask itself, using the dogfood binary). All three asks completed with `Report accepted.` in the log; no crash. Supervisor hardening working as advertised.
- FRICTION (soft, docs/discoverability): the daemon log is at **`~/.ghx/runtime/daemon.log`**, not `~/.ghx/daemon.log` — the path named in the task brief (and worth checking in any support doc) **does not exist** (`ls ~/.ghx/daemon.log` → No such file). `sidecar doctor` prints the sessions dir but never the daemon log/socket location. Separately, the log carries repeated Node SDK noise: `(node:…) [CLAUDE_SDK_CAN_USE_TOOL_SHADOWED] Warning: canUseTool will not be invoked for: mcp__ghx-report-sink__submit_report` — benign but pollutes the log.
- Expected: docs/doctor should name the real log path (`~/.ghx/runtime/daemon.log`); suppress or downgrade the SDK shadow warning.
- Trace: `pgrep -fl "ghx.*daemon"`; `tail ~/.ghx/runtime/daemon.log`.
- Disposition: confirmation on the daemon; log-path/docs = open (soft).

## 2026-07-07 sidecar `ask` frontend always exits 0 — BLOCKED and invalid input map to success — soft [ADR-0034]
- Attempted: exercise sidecar-frontend failure classes. `sidecar ask --repo badslugnoslash "test"` (invalid slug) and `--repo tidwall/gjson --depth bogus "…"` (invalid depth).
- Ground: the bad-slug ask returned a well-reasoned **BLOCKED** report ("not a valid GitHub repository identifier … no investigation possible") but exit **0**. The core CLI maps the same malformed slug to exit 2 (see above), so the sidecar frontend swallows the failure class into success. Under ADR-0034's unified failure-class model, a BLOCKED-because-unusable-input turn should not exit 0 — a scripting agent checking `$?` cannot distinguish "answered" from "couldn't even start". (Answered turns also exit 0, so `$?` carries no signal at all for `ask`.)
- Expected: BLOCKED-due-to-invalid-invocation → exit 2; genuinely-no-evidence → exit 1; answered → 0.
- Suggested fix: map the report's outcome/failure-class to an exit code in the `sidecar ask` command wrapper, mirroring the core CLI's 0/1/2/3 contract.
- Trace: ~/.ghx/sessions/badslugnoslash/ (trace 7043be2…); rerun `ghx sidecar ask --repo badslugnoslash "test"; echo $?` → 0.
- Disposition: open.

## 2026-07-07 `--depth bogus` is silently accepted, not validated against cheap|normal|deep — soft
- Attempted: `sidecar ask --repo tidwall/gjson --depth bogus "How is Get implemented?"`.
- Ground: no validation error — the ask ran a **full normal report** and exited 0, silently treating the invalid depth as a default. The new depth dial names its valid set in `--help` ("cheap|normal|deep") but doesn't enforce it, so a typo (`--depth deeep`, `--depth high`) runs at an unintended budget with no warning. Contrast the crisp A4 affordances on other bad input (e.g. `read --json` → "unknown flag --json; use --full").
- Expected: reject an out-of-set `--depth` with exit 2 and a message naming the valid values, e.g. "invalid --depth \"bogus\"; use cheap|normal|deep".
- Suggested fix: validate the `--depth` flag against {cheap,normal,deep} in the ask command's PreRunE (or via a pflag enum) and error with the valid set.
- Trace: rerun the command; observe a normal report + exit 0.
- Disposition: open.

## 2026-07-07 `--depth` is not recorded in session `meta.json` — soft [visibility]
- Attempted: recover which depth produced a committed session from its artifacts.
- Ground: `meta.json` records name/repo/scope/turnCount/acpSessionId/agentCmd/env/timestamps but **no `depth`/command-budget field**. An auditor reading a session dir cannot tell whether it was cheap, normal, or deep — yet depth changes the evidence breadth and cost. This pinches the visibility/truthfulness tenet: a run parameter that materially shapes the result isn't recoverable from the committed record.
- Expected: `meta.json` (or the turn record) captures the depth/command-budget used.
- Suggested fix: persist the resolved `--depth` (and the numeric command budget it maps to) into `meta.json` / the per-turn ledger.
- Trace: ~/.ghx/sessions/tidwall-gjson/meta.json (no depth key).
- Disposition: open.

## 2026-07-07 sidecar agent narration fabricated downstream exit codes — soft [ADR-0034 / evidence accuracy]
- Attempted: reconcile the BLOCKED report's self-reported evidence with reality. On the bad-slug ask the agent's Uncertainty section stated: "Ran `ghx explore badslugnoslash` -> exit code **3**" and "Ran `ghx inspect badslugnoslash "test"` -> exit code **1**".
- Ground: both claims are **wrong** — running those exact commands against this binary gives exit **2** for *both* (verified directly: `explore badslug` → 2, `inspect badslug concern` → 2). The agent's narrated exit codes don't match the tool's actual codes, so a reader trusting the report's cited evidence would mis-model the CLI's failure-class contract (which is in fact consistent at 2 for malformed slugs). This is an evidence-fidelity gap in the report layer, adjacent to ADR-0034: the report should cite observed exit codes, not plausible-sounding ones.
- Expected: cited exit codes in a report equal the codes the tool actually returned.
- Suggested fix: capture and echo the real `$?` from the agent's tool invocations into the trace/report rather than letting the model narrate them from memory.
- Trace: ~/.ghx/sessions/badslugnoslash/reports/*.json vs `ghx explore badslug; echo $?` (=2) and `ghx inspect badslug c; echo $?` (=2).
- Disposition: open.

## 2026-07-07 404-repo failure class splits: `explore` exits 3, `read` exits 0 — soft [ADR-0034] (reconfirms prior open item)
- Attempted: nonexistent repo via both commands. `explore tidwall/this-repo-does-not-exist-xyz` and `read tidwall/this-repo-does-not-exist-xyz README.md`.
- Ground: `explore` → exit **3** with a good affordance ("→ Repository, branch, or path not found. Confirm owner/repo with `ghx repos <query>` and the layout with `ghx explore owner/repo`…"). `read` on the *same nonexistent repo* → exit **0** with body "=== README.md (not found) ===" — treating a missing repo as a missing file, giving a scripting agent a false success. This **reconfirms** the earlier open "`read` of a nonexistent repo returns exit 0" item on the 2.8.0 binary and adds the explore-vs-read divergence (3 vs 0 for one condition). For comparison, `search` with no matches correctly returns exit **1** and names `ghx repos "<query>"`.
- Expected: a nonexistent repo/ref → exit 3 (or ≥1) on `read` too, matching `explore`.
- Suggested fix: in the read path, when the repo/ref 404s or zero requested files resolve, return ExitUpstreamFailure/ExitNoResults instead of nil (as previously suggested for the standalone item).
- Trace: rerun both commands; `echo $?`.
- Disposition: open (reconfirmed).

## 2026-07-07 source-built binary self-reports version "dev"; doctor's ghx-binary check finds a stale 2.5.0 on PATH — soft [provenance]
- Attempted: confirm the binary under test is v2.8.0. `ghx version` and `sidecar doctor`.
- Ground: `ghx version` → **"ghx dev"** (a source `go build` injects no ldflags version). `sidecar doctor` compounds the confusion: its `ghx-binary` check reports "ghx found: **ghx 2.5.0**" (a *different, stale* binary on PATH), while `report-sink-version` reports "served by /tmp/ghx-dogfood-280 (via current executable), **version dev**". So three different version signals appear in one preflight, none of which says "2.8.0". Provenance is only recoverable out-of-band (worktree HEAD = `0a7ac7f`, tag v2.8.0; ADR-0036 B2 commit e4d7a1f present; CHANGELOG [2.8.0]). For dogfooding/support this makes "which build am I running?" unanswerable from the tool itself.
- Expected: a build (even from source) should be able to self-identify its release, and doctor should reconcile the PATH binary vs the running executable rather than reporting a stale third version.
- Suggested fix: stamp version via `-ldflags -X` in the documented build recipe (or derive from `git describe` at build time); in doctor, note when the PATH `ghx` differs from the running executable.
- Trace: `/tmp/ghx-dogfood-280 version`; `/tmp/ghx-dogfood-280 sidecar doctor`.
- Disposition: open (minor).

## 2026-07-07 code-mode transpile error returns exit 0 — soft [minor]
- Attempted: an early code-mode probe using top-level `await`: `ghx code '(async…)()'` and `ghx code 'const r = await explore(…)'`.
- Ground: prints "transpile: transpile: Top-level await is not available in the configured target environment (\"es2015\")" but exits **0**. A snippet that failed to transpile/run is a bad invocation; exit 0 lets a scripting agent treat a non-executed script as success. (Doubled "transpile: transpile:" prefix is also a minor cosmetic nit.) The `--help` does document the sync `codemode.explore({…}); return …` contract, so this is a validation/exit-code gap, not a usability blocker.
- Expected: a transpile/compile failure → exit 2.
- Suggested fix: return ExitBadInvocation when the JS fails to transpile; de-dupe the error prefix.
- Trace: `ghx code 'const r = await explore("x/y"); return r;'; echo $?` → 0.
- Disposition: open (minor).
