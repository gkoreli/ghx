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
- Disposition: open — candidate product surface: a discovery tier (ghx search/explore across GitHub) inside the sidecar brain.

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
- Disposition: open — design settled (Goga, 2026-07-05): this is a SESSION RESUME problem, not error handling. The ACP session survives the turn error; the runtime must LoadSession-resume it and send a "wrap up: submit_report with what you have" prompt (fresh query = fresh turn budget). Part of the always-on-runtime capability (NORTH_STAR §4): explorations are never lost to process or turn boundaries.

## 2026-07-05 failed turns leave zero artifacts — breaking
- Attempted: audit the failed phoenix ask above.
- Ground: the session dir has only meta.json — no traces.jsonl, no logs, nothing. Emission happens after a successful turn, so exactly the turns that fail (the ones most needing audit) are invisible. Violates the visibility tenet at its most valuable moment.
- Trace: ~/.ghx/sessions/arize-ai-phoenix/ (absence of artifacts is the evidence).
- Disposition: open — emit per-turn artifacts incrementally or flush on error path.

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

_Setup for the week: `go build -o ghx ./cmd/ghx`,
`./ghx sidecar doctor`, then either `ghx sidecar ask --repo <owner/repo>
"<question>"` directly or wire `ghx serve --recon` into your agent's MCP
config and let the recon skill do the talking._
