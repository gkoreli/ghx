# ghx North Star

This is the durable steering document for `ghx`, written from the founder's
articulation of the goal (2026-07-04). Every substantial piece of work should
be traceable to it. It is consistent with ADR-0002 (problem/vision),
ADR-0014.1 (sidecar vision), ADR-0015 (Go-native sidecar runtime), and
ADR-0016 (agentic eval and training data). When this document and an ADR
disagree, write or update an ADR — do not silently fork direction.

## The North Star

This project creates **two things**:

1. **The Agent Sidecar Framework** — a new mental model the industry adopts:
   expensive main agents delegate a whole competence domain to a cheap,
   proficient, specialized sidecar agent instead of loading tool knowledge
   into their own context.
2. **ghx** — the code reconnaissance agent sidecar, and the proof of the
   framework: **the only tool an agent reaches for to explore code and the
   GitHub open-source world.** People should use ghx and that's it — nothing
   else is needed.

The end state: a main agent (an engineer agent, Opus, Fable, anything) needs
**zero knowledge of the ghx CLI** — it never loads the ghx skill file at all.
The outside world knows only a very concise skill: how to talk to ghx as a
reconnaissance service. Everything else — the CLI grammar, the quirks, the
exploration doctrine, map-before-read discipline, search gotchas, backend
escalation — is the sidecar agent's responsibility, under the hood.

## Why

The cost of code exploration inside a main agent is larger than its token
count, and most of it is hard to quantify:

- **Context pollution and coherence loss.** An agent mid-task suddenly needs
  to explore code; the exploration floods its context, and it loses the
  thread of the engineering work it was doing. The lost engineering quality
  and time is the real cost, and it is nearly impossible to price.
- **Doctrine overhead.** The ghx skill file is ~170 lines of quirks. A working
  agent forgets quirks quickly and re-wastes tokens on them, and every wasted
  token degrades its primary work.
- **Expensive models doing cheap work.** Frontier-model tokens spent on
  tree/map/grep loops are pure waste when a specialized cheap agent can do
  the same reconnaissance better.

**The rebuttal to "my main agent is already good at code exploration":**
that misses where the cost lands. By deferring exploration to ghx — removing
the exploration skills, instructions, and step-by-step tool output from the
main agent's context — the main agent gets measurably better at *its own
objective* (engineering, or whatever it is good at) **and** receives better
exploration answers than it would have produced itself. Both, not one. The
dead weight of exploration steps is what fills context windows and forces
session restarts; without it, coding sessions live longer, coherence holds,
and cost drops drastically. The claim to measure is combined: engineering
SPT and exploration SPT both rise, just by using ghx (ADR-0016.6 defines
exploration SPT today; combined-objective measurement is the host-task eval
class noted there).

ghx is genuinely needed as a tool — so the resolution is not "less ghx," it
is moving ghx entirely behind a sidecar boundary. Eventually, with a
custom-trained model, the sidecar will be so cheap, fast, and proficient at
code exploration that it beats Opus, Fable, or any frontier model at using
ghx or anything reconnaissance-shaped.

## The Path

Each phase is valuable standalone and feeds the next:

| Phase | What | Status |
|-------|------|--------|
| P1 | **ghx CLI works quite well**: the #1 direct tool for GitHub exploration (explore/read/map/grep/search/codemode) | Shipped; keep sharpening |
| P2 | **Sidecar over ACP**: main agent delegates reconnaissance and receives compact, auditable evidence reports; persistent evidence-shaped session memory. ACP was chosen deliberately to make the sidecar real much faster | Runtime shipped (ADR-0015); value being proven (ADR-0016.x) |
| P3 | **Swallow the tools**: the ghx CLI itself, codemap, local clone + codemapping for deeper understanding — all become internal tools of the sidecar brain. Codemap is popular today; in this future it is just one tool under ghx. The outside world talks only to the sidecar | Future |
| P4 | **Below ACP**: a custom-trained reconnaissance model inside a lower-level agent runtime (agent SDK) that fully owns the sidecar's context, tools, knowledge, and model weights | Future; ADR-0016's training-data gold mine exists to enable this |

## The Moat

**The moat is the agentic brain, not the tools.** CLI tools, frameworks, and
ergonomics are reusable, replaceable building blocks that anyone can
replicate. Designing the code-reconnaissance brain — the mental models, the
exploration doctrine, what enters the context window, which tool to reach
for and when to stop — is the hard, defensible work. Tools like
[codemap](https://github.com/JordanCoin/codemap) looked like competition at
first; on this path they become tiny building blocks the sidecar swallows —
they may be more popular than us today, but they are tools, and we are
building a product with much higher-level thinking. There may be no real
competition in code reconnaissance at all: others design CLIs; designing
agent brains where the tools disappear into thin air is a different class of
problem.

**The optimization target is signals per token.** Every unit of context the
sidecar's brain consumes should carry maximum reconnaissance signal. That is
why the path bends below ACP eventually (P4): an owned harness (agent SDK)
with fine-grained control over the model's context, tools, and knowledge —
and ultimately a trained model where today's ~400 lines of quirky
instructions are baked into the weights. Deleting that preamble alone raises
signal per token, makes inference cheaper and faster, and improves
exploration quality. Getting rid of waste *is* getting better. Measured,
not aspirational: ADR-0016.6 defines SPT at three levels (main-agent,
sidecar-internal, whole-workflow) and every run reports it.

**The framework will outgrow ghx — later.** The same sidecar pattern applies
to web-browser exploration and many other domains; many brains, many
products; eventually many people will think this way. For now, everything
funnels into proving ghx and the framework: evals are the baseline answer to
"why does ghx deserve to exist," the confidence to make aggressive
engineering moves (like model training), the public marketing proof, and
eventually a published benchmark others can run their own agents against —
and see for themselves that they don't compare.

## Sidecar Product Capabilities (the P2→P3 feature arc)

The names: the **Sidecar Agent Framework (SAF)** is the runtime product; the
**Sidecar Agent Framework Evals (SAFE)** is the proof machinery. They share
one trace/artifact infrastructure — a trace is a trace whether it came from
an eval episode or a real question; the OTel capture layer is configurable,
modular, and reused across both.

1. **Escalation tiers, decided by the sidecar.** Tier 0/1: remote evidence
   via ghx CLI (explore/map/grep/search). Tier 2: when questions demand
   deeper structural understanding, the sidecar decides to pull the codebase
   locally and run codemapping under the hood — integrating
   [codemap](https://github.com/JordanCoin/codemap) (and repomap-style
   analysis) as internal tools. Tier 3: hand back to the main agent. The
   customer agent sees one sidecar and one small skill; which tier answered
   is always visible in the report and traces (ADR-0014.1's tiers, now a
   product phase).
2. **Eager anticipation, configurable.** Questions arrive about a project or
   an exploration thread; the sidecar anticipates the follow-ups and starts
   exploring *before* they are asked — capturing traces and reports so an
   anticipated question answers in under a second instead of 30. Anticipation
   has 3–5 configurable styles (from "answer precisely, do not anticipate"
   to "anticipate aggressively"), settable globally, per project, per
   session, or per question — smart defaults out of the box. Example: asked
   for open-source competitors to CodeRabbit, the sidecar has already mapped
   the landscape — cross-references, stars, recent commits/PRs/issues,
   maturity, direction — and returns a comparison instantly.
3. **Full visibility, full control.** Everything the sidecar does is
   traceable: every question and literal response, every command, every
   escalation decision, every anticipated question, every failure pattern —
   captured as OTel traces and reports per question, per session, per
   project. `~/.ghx` is the product's root storage (sessions, ledgers,
   reports, traces, caches, config). Any human or agent can open it and see
   exactly what ghx did and why. Control mirrors visibility: anticipation
   level, escalation policy, budgets — all configurable at every scope, with
   defaults that just work. Reconnaissance is abstracted away; awareness
   never is.
4. **Dogfooding is the ergonomics bar.** The founder uses ghx-with-sidecar
   for daily development instead of the bare CLI. Friction found while
   dogfooding outranks speculative features.

## Tenets

- **Evals and benchmarking are how we know.** Heavy investment in
  benchmarking exists to prove, with really high confidence, that this
  architecture is a significant upgrade over every existing tool — token
  efficiency and effectiveness are measured, never assumed. Verdicts are
  pre-registered (gates and thresholds decided before runs); an
  under-sampled run self-labels PRELIMINARY and cannot feed a go/no-go
  decision. Eval results serve three roles, in order: confidence for our own
  build decisions, then marketing material and bragging rights when
  promoting ghx, then possibly a product of their own (see Consequence
  Products).
- **The main agent's context is sacred.** Every design choice is judged by
  what it removes from the customer agent's context.
- **Evidence, not vibes.** Every sidecar answer carries files, commands,
  snippets, backends, and uncertainty. Hidden repo analysis is not
  trustworthy; the sidecar may be opinionated because its trace is visible.
- **The sidecar stops at reconnaissance.** It hands back evidence; it never
  becomes the coding agent.
- **Remote-first, escalation explicit.** No clone, no local index by default.
  Pulling code locally and codemapping it is a deliberate, visible deeper
  layer (P3), never silent magic.
- **Protocols are stepping stones, not identity.** ACP is the fast path
  today; the boundary must survive replacing ACP, the brain, or both (P4).
  Clarified 2026-07-05: **we never invent a sidecar protocol or agent
  framework** — that would fail the open-source-leverage filter. What we
  own is the boundary contract (English question in → compact evidence
  report out) and the brain behind it; every layer underneath is adopted.
  "Below ACP" (P4) means dropping to an existing agent-SDK harness we
  configure — owning context assembly, tool registry, and model choice —
  not authoring a wire protocol. Pre-registered exit triggers, so the
  P4 move is evidence-driven rather than a rewrite itch: (1) adapter
  opacity keeps taxing us (e.g. claude-agent-acp thinking emission was
  undocumented — verified only by reading the shipped tarball, ADR-0018);
  (2) a measured SPT plateau attributable to harness overhead ACP cannot
  remove (the ~400-line persona doctrine paid every session); (3) tool
  surface/isolation control the adapter cannot grant (ADR-0016.3's eval
  isolation gap); (4) serving a custom-trained model (M10), which no
  existing ACP adapter will speak for us. Until triggers fire, ACP stays.
- **Core capabilities live in `internal/ghx`.** Frontends — CLI, MCP,
  codemode, sidecar — wrap the same core (ADR-0010 rule).
- **Local-first evals.** No hosted eval platform, no eval-service key; local
  artifacts are canonical and double as future training data.

## Consequence Products

Things that may fall out of pursuing the north star, worth recognizing but
never worth steering by:

- **The Agent Sidecar Framework itself** — adopted by others as a mental
  model and eventually as reusable infrastructure.
- **An eval framework for agent sidecar frameworks** — the local-first
  episode/reward/gate machinery built for ghx could generalize to evaluating
  any sidecar-shaped agent.
- **A public code-reconnaissance benchmark** — the SAFE task/gate corpus,
  published so others can run their agents against it and compare to ghx.
- **Sidecar brains for other domains** — the same framework applied to
  web-browser exploration and beyond, once ghx is proven.

These are consequences. The main goal is the ghx north star; a consequence
product only gets investment when the north-star filter passes and the ghx
frontier milestone is not starved by it.

## Non-Goals

- A full coding agent.
- Repo-wide summaries as the default output.
- Teaching every main agent the ghx CLI (the opposite of the goal).
- SEO-driven feature parity with search vocabulary.
- A public `ghx bench` CLI or general-purpose eval framework.

## Milestones and Current Position

| # | Milestone | Phase | Status |
|---|-----------|-------|--------|
| M1 | Evidence engine (CLI, MCP, codemode, map engine) | P1 | Shipped |
| M2 | Go-native sidecar runtime over ACP (ADR-0015) | P2 | Shipped |
| M3 | Eval kernel + validity hardening + observability (ADR-0016.1–.5, 0015.1) | P2 | Done; all five gates pass on live spot checks |
| M4 | **Formal gate run** (≥ 6 tasks × 5 trials × 3 profiles) → committed verdict | P2 | **DONE — THESIS SUPPORTED 2026-07-05** at full sample (90 eps, all five gates pass; sidecar correctness 0.908 vs ghx 0.931 at 25× compression, 24× signal/token). Evidence: `docs/evals/gate-run-2026-07-05-confirmatory/`; the honest negative that drove the ADR-0016.7 fixes is kept at `docs/evals/gate-run-2026-07/` |
| M5 | Concise "reconnaissance service" skill + integration ergonomics: main agent needs zero ghx CLI knowledge; founder dogfoods the sidecar daily | P2 | **← frontier** — governed by ADR-0019 (recon skill ≤30 lines, single-tool MCP mode, frictionless `ask`, ACP-handshake fail-fast, one-week dogfood exit bar); ADR-0016.7's persona contract was the first slice |
| M6 | Shared SAF/SAFE trace infrastructure: runtime sessions emit the same OTel traces/reports as evals; `~/.ghx` root storage; full visibility surface | P2 | Next after M4 rerun; governed by the ADR-0018 agentic-observability thread (official OTel GenAI conventions: reasoning capture, content events, metrics, score explanations, LLM-native viewer) |
| M7 | Escalation tiers: codemap CLI + local clone as internal sidecar tools, sidecar-decided, fully visible | P3 | Future (ADR before build) |
| M8 | Eager anticipation with configurable styles; sub-second answers to anticipated questions | P3 | Future (ADR before build) |
| M9 | Trajectory accumulation at scale (evals + consented dogfood sessions); SFT/preference/reward exports | P4 prep | Blocked on M4 verdict |
| M10 | Trained `ghx-sidecar` model behind the same boundary; re-run the same suite | P4 | Blocked on M9 |

M4's verdict gates the investment: G1 (correctness) or G3 (compression)
failing means the sidecar thesis is not supported and P3/P4 spending pauses.
The north star includes the possibility of learning the boundary is wrong —
cheaply and with evidence.

The eval stack itself has a required next layer (Goga, 2026-07-05, tenet in
AGENTS.md): the deterministic gates are frozen baselines that measure
pre-registered fact recall, not exploration quality. A **calibrated judge
scorer** (LLM judge with full OTel reasoning traces, hand-label calibration,
committed prompt/model version, never the gate alone) is needed to compare
plain vs ghx vs ghx-sidecar reasoning, tool calls, and outputs — real-world
quality claims to customers must rest on both layers. ADR before build;
natural sequencing: alongside or immediately after the ADR-0016.7
confirmatory rerun, and definitely before M9 preference/reward exports
(which need trajectory-quality labels anyway).

## The North-Star Filter

Before taking on work, ask: **does this remove tokens/knowledge from the main
agent's context, make reconnaissance cheaper or more proficient, make the
evidence more auditable, or produce better training trajectories?** If none
of the four, reject or defer it — ADR-0016.2's "Considered and rejected"
section shows the filter in action.

A second filter governs *how* we build (Goga, 2026-07-05; canonical rules in
AGENTS.md "Open Source Leverage"): explore open source for ideas, libraries,
and established patterns first — steal greatly in the Picasso sense, openly
and with attribution — and hand-roll frameworks, formats, or tools only when
nothing existing serves the need or the vision. That test is where ghx and
the Agent Sidecar Framework themselves come from: both are novel, both are
loudly inspired by what exists (codemap-style local mapping is slated to be
just another internal sidecar tool in M7). Concretely: agent traces are
official OTel exactly per spec so open-source tooling works within seconds;
our own metrics ride alongside in attributes or sibling artifacts, never as
a fork of the standard.

## Working the Loop

Iterations toward the north star follow ADR-driven engineering (AGENTS.md):

1. Identify the current frontier milestone above.
2. Pick the highest-leverage step; check it passes the north-star filter.
3. Write or update the governing ADR before substantial implementation.
4. Implement, verify with evidence, update the ADR with what was learned.
5. Update the milestone table here when a milestone's status changes.
