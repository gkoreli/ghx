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
itself.** Six real `ghx sidecar ask` investigations, one per subject repo;
every claim below traces to a report + full OTel trail in
`~/.ghx/sessions/<slug>/` (reports/, traces.jsonl with per-tool spans,
ledger). Two of eight questions died to the 24-turn cap (crewai; phoenix
at normal depth — answered at `--depth deep`), logged as breaking friction
in `docs/dogfood/FRICTION.md`. This ADR is therefore both landscape and
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

### CrewAI — session `crewaiinc-crewai` (INCOMPLETE — recon died at turn cap)

Delegation-tool pattern known from public docs; not source-verified
tonight. Honest gap; re-run post max-turns fix.

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
scores); the *combination* — auditable, steerable, persistent,
evidence-bearing delegation as a product — has no found occupant. This is
the moat sentence, and it stays PRELIMINARY-honest: based on six repos on
one evening; the landscape moves monthly; re-scan quarterly or on any
funding-scale competitor signal.

## Steal list (routed)

| Idea | Source | Lands in |
|---|---|---|
| Epoch reducers / trial aggregation | Inspect AI | ADR-0025 statistics follow-up |
| Recorder abstraction (one model, many sinks) | Inspect AI | telemetry package evolution (M6+) |
| Committed judge-prompt config files | Phoenix | ADR-0023.1 D4 (already aligned; adopt the config-file shape) |
| Team-member-as-tool ergonomics | smolagents | recon skill / serve --recon copy |
| Channel-schema framing for boundary docs | LangGraph | boundary-contract docs, A2A mapping (ADR-0020.1 D3) |
| Transcript-vs-last-message dial as cautionary tale | AutoGen | marketing narrative: the evidence-shaped middle |

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
