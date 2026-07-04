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
| M3 | Eval kernel + validity hardening (ADR-0016.1, 0016.2) | P2 | Built; smoke pairs pass all five gates |
| M4 | **Formal gate run** (≥ 6 tasks × 5 trials × 3 profiles) → committed verdict | P2 | **← staged**: framework proven (V0–V2 closed, ADR-0016.3/.4/.5), evidence ledger shipped (ADR-0015.1), all five gates pass on live spot checks — the ~90-episode V3 run awaits the explicit engineer trigger |
| M5 | Concise "reconnaissance service" skill: main agent needs zero ghx CLI knowledge | P2 | Blocked on M4 verdict |
| M6 | Trajectory accumulation at scale; SFT/preference/reward exports | P4 prep | Blocked on M4 verdict |
| M7 | Local escalation layer: clone + codemapping as internal sidecar tools | P3 | Future |
| M8 | Trained `ghx-sidecar` model behind the same boundary; re-run the same suite | P4 | Blocked on M6 |

M4's verdict gates the investment: G1 (correctness) or G3 (compression)
failing means the sidecar thesis is not supported and P3/P4 spending pauses.
The north star includes the possibility of learning the boundary is wrong —
cheaply and with evidence.

## The North-Star Filter

Before taking on work, ask: **does this remove tokens/knowledge from the main
agent's context, make reconnaissance cheaper or more proficient, make the
evidence more auditable, or produce better training trajectories?** If none
of the four, reject or defer it — ADR-0016.2's "Considered and rejected"
section shows the filter in action.

## Working the Loop

Iterations toward the north star follow ADR-driven engineering (AGENTS.md):

1. Identify the current frontier milestone above.
2. Pick the highest-leverage step; check it passes the north-star filter.
3. Write or update the governing ADR before substantial implementation.
4. Implement, verify with evidence, update the ADR with what was learned.
5. Update the milestone table here when a milestone's status changes.
