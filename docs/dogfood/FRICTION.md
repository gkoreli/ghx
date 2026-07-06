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
- Disposition: open — ship a documented default agent command (or `config init --claude-acp` that writes the npx adapter line) so setup is one command.

## 2026-07-05 report-sink depends on a current ghx binary — soft
- Attempted: real asks with PATH ghx 2.1.13 (released) while the sidecar features live on unreleased mainline.
- Ground: the submit_report MCP server is served by the ghx binary itself; a stale binary silently degrades the contract to the text fallback (loud stderr warning exists, but only if you read stderr). Needed manual `GHX_REPORT_SINK_EXE` to a fresh build.
- Trace: session dirs under ~/.ghx (see reports/ presence as the tell).
- Disposition: open — release cadence question + doctor should check the sink-server version matches.

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

_Setup for the week: `go build -o ghx ./cmd/ghx`,
`./ghx sidecar doctor`, then either `ghx sidecar ask --repo <owner/repo>
"<question>"` directly or wire `ghx serve --recon` into your agent's MCP
config and let the recon skill do the talking._
