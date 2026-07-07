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
- Disposition: open.

## 2026-07-07 `read` of a nonexistent repo returns exit 0 — soft
- Attempted: `/tmp/ghx-dogfood-w3 read totally/nonexistent-repo-xyz123 README.md`.
- Ground: prints "=== README.md (not found) ===" and exits **0**. A scripting agent checking `$?` reads this as success and would proceed on empty content. A nonexistent repo/ref or an unresolved requested file should not report OK.
- Expected: exit 1 (ExitNoResults) when no requested file resolves, or exit 3 when the repo/ref 404s.
- Suggested fix: in the read command, when zero requested files resolve (or the repo/ref lookup 404s), return `ExitNoResults`/`ExitUpstreamFailure` instead of nil.
- Trace: n/a (invocation error).
- Disposition: open.

## 2026-07-07 live 401 on `search` prints raw HTTP with no fix-it affordance — soft [A4 gap]
- Attempted: `GH_TOKEN=ghp_invalidbadtoken... /tmp/ghx-dogfood-w3 search charmbracelet/bubbletea "func eventLoop"`.
- Ground: exit **3** (correct) but the message is the bare upstream string "search failed: HTTP 401: Bad credentials (https://api.github.com/...)" — no affordance. `sidecar doctor`/preflight already owns the right remediation ("Run `gh auth login`, or set GH_TOKEN/GITHUB_TOKEN"; internal/sidecar/preflight.go:148), but the hot-path command error doesn't surface it. The A4 promise (errors name the fix) is unmet on the single most common live failure. Separately: an *empty* `GH_TOKEN=` degraded to exit 1 "0 results" (go-gh fell back to the `gh` keyring token), i.e. missing-auth can masquerade as no-results — no clean auth-absent signal.
- Expected: 401/403 upstream errors name `gh auth login` / GH_TOKEN as the fix.
- Suggested fix: map 401/403 in the CLI error path to an affordance reusing the preflight.go remediation string.
- Trace: n/a (live upstream error, no session emitted).
- Disposition: open.

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
