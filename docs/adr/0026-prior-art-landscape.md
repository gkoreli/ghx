---
title: "ADR-0026: Prior-Art Landscape — Delegation Is Commodity; Auditable, Steerable, Persistent Delegation Is Not"
date: "2026-07-05"
status: "accepted"
thread: "prior-art"
author: "Goga Koreli"
---

# 0026. Prior-Art Landscape

## Status

Accepted as a landscape record (founder directive 2026-07-05: know the
competition, steal what serves, cross-reference with rationales). Evidence
has a property no earlier research ADR had: **the sidecar gathered it
itself.** Six real `ghx sidecar ask` investigations, one per subject repo
(ten questions total); every claim below traces to a report + full OTel
trail in `~/.ghx/sessions/<slug>/` (reports/, traces.jsonl with per-tool
spans, ledger). Two questions died to the 24-turn cap on first attempt
(crewai; phoenix at normal depth), logged as breaking friction in
`docs/dogfood/FRICTION.md`; both were later answered at `--depth deep` —
phoenix the same evening, crewai on the 2026-07-05 re-run after ADR-0027
landed (two turns, resuming the original session; see the CrewAI section
for the recovery evidence). This ADR is therefore both landscape and
dogfood artifact.

## The organizing lens (founder, 2026-07-05)

Main-agent ergonomics: when a parent agent delegates, **what does it
actually get back, what can it inspect afterward, and what can it steer?**
Visibility (artifacts an agent can read) and control (dials on the
consumption surface) with smart defaults — NORTH_STAR capability §3 as
sharpened today.

## Delegation frameworks (SAF-adjacent)

### smolagents (managed agents) — session `huggingface-smolagents`

Sub-agent = any agent with name+description, invoked like a tool with
`task: str`; **context isolation is real** (fresh AgentMemory/state per
sub-agent) — validation of our isolation instinct. But the parent
receives a **string** (final answer rendered through a prompt template);
no artifacts, no trace handle, no persistent memory across tasks, no
dials beyond the task text. Steal: the "team member = tool with a
description" consumption ergonomics (maps to our single MCP recon tool);
the isolation default.

### LangGraph (subgraphs, Send, Command.PARENT) — session `langchain-ai-langgraph`

The most engineered isolation model: subgraph state schemas share only
declared channels; `Send()` fan-out with reducers; `Command.PARENT` as
the explicit handoff primitive; checkpoint namespaces give replayable
sub-state — the closest thing found to visibility, but it is
**framework-internal state, not agent-readable artifacts**: a parent (or
human) needs LangGraph APIs and the same process, not `ls` and `jq`.
Steal: channel-schema thinking for what crosses the boundary (our report
schema is exactly a declared channel); checkpoint-namespace scoping as
prior art for session trees.

### AutoGen (AgentTool/TaskRunnerTool, handoffs) — session `microsoft-autogen`

Delegation returns a `TaskResult` = **the sub-agent's message transcript**,
stringified — either concatenated or last-message-only
(`return_value_as_last_message`). That flag is the field's crude version
of our compression dial: they let you choose between "everything" (context
flood) and "one message" (no evidence). The evidence-shaped middle — a
compact report *with* citations and an audit trail — is exactly the gap
ghx occupies. Swarm handoffs move control, not evidence.

### CrewAI — session `crewaiinc-crewai` (re-run 2026-07-05 post ADR-0027; 2 turns, resumed)

Delegation is two LLM tools (`DelegateWorkTool` "Delegate work to
coworker", `AskQuestionTool` "Ask question to coworker");
`BaseAgentTool._execute` wraps the request into a brand-new **ephemeral
Task** and calls `coworker.execute_task()` directly, bypassing
`Task._execute_core` — so the parent receives a **plain str** (the
coworker's raw LLM answer): no TaskOutput, no guardrails/callbacks/
output_file on the sub-task. What persists after delegation: only
breadcrumbs on the *calling* task — a `delegations` counter and a
`processed_by_agents` set (`task.increment_delegations`); the sub-task is
discarded. Caller dials: `Agent(allow_delegation=...)` (off by default),
`Process.hierarchical` + `manager_agent` (the manager gets the same two
tools and the same plain str back — no richer payload), and the coworker
set (all crew agents minus the task's own). Turn 2 (event-bus follow-up):
delegation has **no dedicated event type** — it rides generic
`ToolUsageStarted/Finished` events, which do carry the full
`{task, context, coworker}` args and the coworker's raw output, and the
official `TraceCollectionListener` ships that payload verbatim to their
AMP tracing backend. So richer evidence of the exchange exists than the
delegating agent ever receives — visibility flows to the dashboard, not
to the agent in the loop. Through our lens, the sharpest foil found:
inspect = a counter, steer = a boolean, receive = a string. Steal: the
delegations/processed_by breadcrumb as prior art for cheap per-question
provenance in our ledger; the event-bus-vs-return-value split as a
marketing exhibit next to AutoGen's flag. Honest gaps: async path
(`aexecute_task`) parity unverified; whether a coworker `response_format`
BaseModel can flow through `_finalize_task_execution` unchecked; AMP
server-side rendering out of repo.

ADR-0027 live-test note: turn 1 took 29 tool calls — past the old
24-turn normal cap that killed the first attempt — and completed inside
deep's 48 budget; turn 2 (21 tool calls) ran on the ADR-0027 runtime and
resumed the session cleanly (ledger context carried, `turnCount: 2`, two
reports in one session dir). The D1 wrap-up net stayed armed but never
fired (`wrapUpRecovered` absent from all records — checked by grep). The
recovery that mattered here was resume-as-continuation plus the deep
budget, not the net.

## Eval harnesses (SAFE-adjacent)

### Inspect AI (UK AISI) — session `ukgovernmentbeis-inspect-ai`

The strongest architectural cousin to SAFE: `Task(dataset, solvers,
scorers)`, mutable `TaskState` threaded through a solver chain, `Score`
objects reduced per-epoch into `EvalMetric`/`EvalResults`, persisted as
Pydantic `EvalLog` via pluggable recorders. Steal candidates, each with a
target: **epoch reducers** (their trials-per-cell aggregation — compare
against our mean-of-episodes for the ADR-0025 D2/statistics work);
**recorder abstraction** (one log model, many sinks — mirrors our
telemetry package trajectory); solver/scorer separation (already our
shape — convergent evolution is validation). Differences that keep us
distinct: Inspect evaluates *models on tasks*; it has no boundary
contract, no evidence-citation scoring, no runtime/eval shared substrate,
no SPT economics.

### Phoenix (Arize) — session `arize-ai-phoenix-deep`

LLM-as-judge machinery: `ClassificationEvaluator` rendering committed
prompt-template configs (literal judge prompt text checked in as
generated config files — **prior art for our committed-judge-prompt rule,
ADR-0023.1 D4**), frozen `Score` dataclass, span annotations in a DB. No
calibration/agreement tooling found (confirms ADR-0023 memo: our κ-gated
calibration is differentiating, not table stakes). Their scores attach to
spans — same instinct as our `gen_ai.evaluation.result` events, theirs in
a service DB, ours in local OTel artifacts.

## What nobody found does (the competitive claim, stated carefully)

Across all six: **delegation returns strings or transcripts; none returns
a schema-validated evidence contract** (claims with citations, commands,
uncertainty) — and none gives the parent agent artifact-level visibility
(files it can read without the framework), an OTel audit trail of the
delegate, persistent cross-question session memory it can inspect, or
measured signal-per-token economics. Each element exists somewhere in
embryo (LangGraph checkpoints, AutoGen's last-message flag, Phoenix span
scores, CrewAI's event-bus trace payloads that never reach the caller);
the *combination* — auditable, steerable, persistent,
evidence-bearing delegation as a product — has no found occupant. This is
the moat sentence, and it stays PRELIMINARY-honest: based on six repos on
one evening; the landscape moves monthly; re-scan quarterly or on any
funding-scale competitor signal.

### Inspect AI deep-dive (turn 2, same session — resume dogfood PASSED)

Follow-up answered from the resumed session (2 reports, one session dir;
the ledger carried turn-1 context). The high-impact/low-effort findings:

- **Epoch reducers are richer than our mean**: registered ScoreReducer
  functions — mode, mean, median, `at_least(n)`, `pass_at(k)`, `pass_k`,
  max — collapse per-trial Scores per sample before aggregation
  (scorer/_reducer/reducer.py). `at_least(n-of-k)` semantics for gate
  inputs would be *more robust to single-trial flakes* than mean-vs-
  threshold (tonight's round-1 noise is the motivating exhibit). Route:
  ADR-0025 statistics follow-up — reducer choice as a pre-registered
  gate parameter.
- **`inspect view` = one command, local server, artifact browser**
  (_view/view.py → FastAPI over EvalLog). Direct prior art for
  `ghx sidecar view` (absorption watchlist row 1) and later an episode/
  verdict browser over `docs/evals/` artifacts.
- **Interrupted-run recovery is built in** (log/_recover) — prior art
  for our accumulation + the hang-watchdog/resume-as-recovery work
  (NORTH_STAR §4); they treat eval runs as resumable state, same
  instinct as our session mastery.

## Steal list (routed)

| Idea | Source | Lands in |
|---|---|---|
| Epoch reducers / trial aggregation (`at_least(n)`, `pass_at(k)`) | Inspect AI | ADR-0025 statistics follow-up |
| One-command local artifact viewer (`inspect view` pattern) | Inspect AI | `ghx sidecar view` (absorption watchlist) |
| Interrupted-run recovery (log/_recover) | Inspect AI | hang watchdog + resume-as-recovery (NORTH_STAR §4) |
| Recorder abstraction (one model, many sinks) | Inspect AI | telemetry package evolution (M6+) |
| Committed judge-prompt config files | Phoenix | ADR-0023.1 D4 (already aligned; adopt the config-file shape) |
| Team-member-as-tool ergonomics | smolagents | recon skill / serve --recon copy |
| Channel-schema framing for boundary docs | LangGraph | boundary-contract docs, A2A mapping (ADR-0020.1 D3) |
| Transcript-vs-last-message dial as cautionary tale | AutoGen | marketing narrative: the evidence-shaped middle |
| Event-bus-only delegation visibility (dashboard sees more than the caller) | CrewAI | marketing narrative: evidence must return to the caller; ledger provenance prior art (`delegations`/`processed_by_agents`) |

## Absorption watchlist (standing — the codemap pattern, generalized)

Founder directive 2026-07-05: the strongest competitive move is not to
beat a tool but to **consume it under the hood so it disappears into the
product** (NORTH_STAR P3 "Swallow the tools", The Moat). Standing
watchlist, impact/effort honest, each absorbed tool vanishes from the
user's world:

| Candidate | Absorbed as | Impact / effort | Route |
|---|---|---|---|
| otel-desktop-viewer | `ghx sidecar view [session]` — spawn viewer + auto-replay the session's artifacts; the ADR-0018 curl recipe becomes one command | HIGH / LOW | M6 polish; near-term |
| codemap | Tier-2 internal tool: dependency graphs, importers, blast radius | HIGH / MED | M7 (ADR-0024) |
| ast-grep | Tier-2 structural pattern search (single binary, shell-out) | MED / LOW | M7 |
| aider repomap PageRank | algorithm absorption, no dependency: rank files by import-graph centrality to prioritize exploration | MED / LOW | persona/doctrine or M7 |
| stack-graphs | precise call paths / implementors | HIGH / HIGH (language maturity risk) | M7, behind codemap |
| universal-ctags | cheap symbol fallback | LOW / LOW | only if mapengine gaps appear |
| Phoenix judge prompt configs | data absorption: committed config-file shape + adapted prompt texts (attributed) | MED / LOW | judge build (ADR-0023.1) |
| Inspect AI epoch reducers / viewer patterns | pattern absorption (deep-dive recon in flight, session `ukgovernmentbeis-inspect-ai` turn 2) | TBD | ADR-0025 stats; artifact-viewer UX |

Already absorbed and invisible: tree-sitter (inside mapengine), gh/GitHub
API (inside ghx). The scan is standing: every prior-art review asks
"steal the idea, or swallow the tool?"

## Cross-references

- Sessions: `~/.ghx/sessions/{ukgovernmentbeis-inspect-ai, huggingface-smolagents, langchain-ai-langgraph, microsoft-autogen, arize-ai-phoenix-deep, crewaiinc-crewai}` — reports + traces (the evidence).
- docs/dogfood/FRICTION.md — the two breaking entries this recon produced.
- ADR-0020 (protocol landscape), ADR-0023 (judge research), ADR-0024 (M7
  research) — sibling landscape memos this complements.
- NORTH_STAR "The Moat", capability §3 (main-agent-as-consumer lens).
