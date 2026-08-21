---
title: "SAF Adoption Research — What Would Make the Agent Sidecar Framework a No-Brainer"
date: "2026-08-21"
status: "research"
thread: "sidecar-adoption"
author: "delegated research worker (ox-alpha)"
scope: "read-only research; no code, ADR, or doc changes; nothing committed"
---

# SAF Adoption Research — What Would Make the Agent Sidecar Framework a "No-Brainer"?

## Question

Goga's open question (NORTH_STAR M5/B2 frontier): **how does the Agent Sidecar
Framework (SAF) stop being an experimental internal runtime and become
something real-world users and their main agents adopt by default?** This memo
grounds the answer in (a) the repo's current adoption surface as it actually
ships today, and (b) the external landscape of delegation/subagent mechanisms a
prospective adopter can already choose for free. Every claim carries evidence.

**Working definition of "no-brainer":** an adoption path where the main agent
(or its human) goes from zero to first successful delegated recon question in
≤ 2 minutes, with zero new accounts, zero config files hand-written, zero CLI
grammar learned, and no visible risk of silent failure — and where the thing
they installed keeps paying rent on the second question.

---

## Part 1 — Findings from inside the repo (the adoption surface as shipped)

### 1.1 The install story is already good; the *wiring* story is where friction lives

The distribution surface is genuinely strong for a young tool: npm
(`npx @gkoreli/ghx sidecar doctor` — README.md:161), Homebrew tap
(README.md:164), `go install` (README.md:170). I verified the npx path live:
`npx -y @gkoreli/ghx --version` → **`ghx version 2.9.1`** with zero install
(commands appendix C1). Zero-config default agent is the pinned
claude-agent-acp adapter when Node + Claude Code login exist (README.md:179–187).

But the dogfood record shows the *first successful ask* has repeatedly been the
hardest step, not the easiest:

- **ACP setup was the hardest step** — the default `claude` binary fails the
  handshake correctly but the fix once required knowing about
  `@agentclientprotocol/claude-agent-acp` and hand-writing a wrapper that only
  existed in this repo's `scripts/` (FRICTION.md, entry "2026-07-05 ACP agent
  setup is the hardest step"; fixed by `config init --claude-acp`, commit 61a0e3c).
- **Silent nothing on a work laptop** — `ghx sidecar ask` returned exit 0 with
  no report and one breadcrumb in daemon.log; auth env cannot be pinned via any
  ACP session option, so out-of-box success is environment-dependent
  (ADR-0033 Context + Research sections; D2–D5 added per-session stderr capture,
  `doctor --live`, env-name provenance to make failures loud rather than absent).
- **Enterprise wrapper builds hang at prompt time** unless the user discovers
  `agentSettingSources: ["user"]` (README.md:216–237) — a config line a
  first-time adopter will not know exists.
- **Stale report-sink binary silently degrades reports to text fallback**
  (FRICTION.md 2026-07-05; doctor now fails loudly, same entry).

Net: install ≈ solved; **auth/env wiring and failure visibility are the actual
adoption tax**, and they are exactly what ADR-0033's diagnostics work attacks.
The remaining gap is that "works out of the box" is still conditional on the
user's Claude Code installation shape — there is no auth story that doesn't
lean on the host Claude Code login (see 1.4).

### 1.2 The recommended consumption surface still teaches more than one sentence

The thesis says the outside world learns one sentence (ADR-0019 D1). Today:

| Surface | Size | Evidence |
|---|---|---|
| Recon skill (`ghx skill --recon`) | 47 lines | measured live (C2); README.md:354 claims "≤ 30" |
| Classic CLI skill (`ghx skill`) | 232 lines | PA-0001.9 R1 recompute |
| Classic MCP skill | 216 lines | measured live (C2) |
| Default MCP mode | **7 direct tools** | serve.go registers explore/repos/search/read/tree/search_tools/code; README.md:542 |
| Recommended mode | 1 tool (`recon`) | `ghx serve --recon` (serve.go:416; verified in `--help` output, C2) |

PA-0001.9 flagged (H1/R1) that `.claude-plugin/plugin.json` shipped only the two
heavy skills. That specific finding is now fixed: plugin.json lists
`./skills/ghx-recon` **first** (verified, C3) — good. But the recon skill is 47
lines against its own stated ≤30 budget, and `ghx serve` without flags still
exposes the 7-tool surface; the single-tool recon mode remains opt-in behind a
flag (ADR-0019 D2 explicitly deferred flipping the default to post-dogfood).
Also still true from PA-0001.9 R3: the MCP recon handler marshals the report to
JSON **then appends human-prose lines** (route line + artifacts footer,
internal/cli/serve.go:208–221 read directly) — a consumer that `JSON.parse`s
breaks on the suffix. The CLI gets a clean typed envelope (`askEnvelope`);
the surface the north star recommends for agents is regex-bait.

**Finding:** the "one sentence" boundary exists but is not yet the default
experience on any channel except the recon skill file itself.

### 1.3 What the product uniquely offers vs every free alternative — and what it doesn't

ADR-0026's twelve self-sidecar investigations remain the sharpest internal
landscape scan: across smolagents, LangGraph, AutoGen, OpenAI Agents SDK,
Mastra, CrewAI, OpenHands, Google ADK, Letta — **delegation returns strings,
transcripts, or caller-schema objects; none returns a schema-validated evidence
contract with commands-run, uncertainty, and file-level artifacts a parent can
read with `ls`/jq** (ADR-0026 "What nobody found does", :345–377). My external
re-check (Part 2) found no occupant of that niche either; instead it confirmed
that **host-native subagents have commoditized the context-isolation benefit**
— which is the part of SAF's pitch a casual adopter feels first.

The honest internal audits already say this:

- The quality half ("both, not one") is **unmeasured**: only 2/6 eval tasks
  discriminate and the sidecar scores below ghx on both (PA-0001.9 R2 recompute;
  corpus-discrimination-2026-07-07.md; TRUST.md H7); C8 host-task evals are
  registered but have zero episodes (NORTH_STAR.md:331, run Goga-triggered);
  the judge is uncalibrated pending founder labeling (TRUST.md H1).
- "Brain-not-tools" is an advantage, **not yet a durable Power**: both moat
  barriers (M10 trained model, κ-calibrated judge) unrealized (PA-0001.9 H3).
- The strongest competitor signal: `DeusData/codebase-memory-mcp` — 27,884★ in
  ~4.5 months, arXiv benchmark published, **auto-installs into 11 agents**,
  brain-less by declaration (PA-0002.1 §1–§2, with gh api evidence). It proves
  distribution + benchmark-first framing wins mindshare even without a brain.

So the differentiators that could justify adoption are real (evidence contract,
artifacts-on-disk, OTel traces, session memory, discovery tier) but **none of
them is currently load-bearing for a new adopter's first five minutes**, and
the citable proof that the sidecar beats "just use my host's built-in Explore
subagent" does not exist yet (selection-eval arm explicitly missing — PA-0001.9
R4(c)).

### 1.4 The dependency stack is the hidden adoption cost

SAF's runtime chain today: ghx → `gh` auth → npx/Node → claude-agent-acp@0.55.0
(pinned) → Claude Agent SDK → Claude Code credentials. Each hop is a place a
new user's machine can differ (FRICTION.md entries on stale binaries, wrapper
builds, daemon env inheritance; ADR-0033's whole diagnosis). The cheap-backend
research note (docs/research/001-cheap-sidecar-backend.md) adds the economic
half: nothing structurally enforces that the sidecar runs on a *cheap* model —
the original gate run consumed the same subscription quota as the main agent.
For adoption, that means the "cheap specialist" promise is currently **an act
of faith plus a config field**, not a metered fact. (TRUST.md H5 shows token
accounting exists in evals; FRICTION.md 2026-07-07 shows production session
metrics undercount subject-agent tokens.)

---

## Part 2 — Findings from outside (the external landscape)

All external claims cite URLs; searches were snippets-level except where noted.

### 2.1 Host-native subagents are free, built-in, and commoditized isolation

Claude Code ships built-in subagents (Explore, Plan, general-purpose) that run
in their own context window and "return only the summary" — the exact
context-pollution benefit SAF leads with — plus custom subagents as markdown
files in `~/.claude/agents/`, per-subagent model choice ("Control costs by
routing tasks to faster, cheaper models like Haiku"), scoped MCP servers per
subagent, background execution, and teams (code.claude.com/docs/en/sub-agents,
page fetched in full 2026-08-21). Cursor has subagents/background agents and
SKILL.md support (aimakers.co/blog/cursor-2-4-subagents/;
forum.cursor.com/t/sub-agents-for-the-agent-to-delegate-tasks-to/123265 —
users explicitly request Anthropic-style subagents). GitHub Copilot supports
custom specialized agents + agent skills + MCP, GA'd 2026-07-29
(github.blog/changelog/2026-07-29-copilot-code-review-agent-skills-and-mcp-now-generally-available/).

Implication: **"delegate so exploration doesn't flood your context" is table
stakes now**. A no-brainer pitch cannot lead with isolation; it must lead with
what host subagents cannot do: remote/GitHub-wide recon without cloning,
discovery sweeps across repos, cross-question persistent memory, auditable
evidence contracts. PA-0001.9 already refuted the "native Explore competes on
capability" frame — Explore is local-filesystem-only (R4/Bones #1).

### 2.2 Framework SDKs make delegation easy but return strings — SAF's lane is still empty

OpenAI Agents SDK: handoffs transfer control (no value returns), agents-as-tools
return last-message text with a `custom_output_extractor` escape hatch
(openai.github.io/openai-agents-python/; deepwiki.com/openai/openai-agents-js/4.4-multi-agent-workflows:
"Sub-agent output becomes tool result"). Claude Agent SDK exposes subagents,
custom tools, sessions, MCP (code.claude.com/docs/en/agent-sdk/subagents).
Hermes Agent (Nous Research) has a first-class `delegate_task` primitive spawning
isolated child agents with inherited tool access
(hermes-agent.nousresearch.com/docs/user-guide/features/delegation). None of
these surfaces an evidence-shaped, schema-validated, artifact-backed contract —
consistent with ADR-0026's finding, uncontradicted by anything I found. The gap
is real; the problem is nobody is *feeling* the pain loudly enough yet because
strings "work".

### 2.3 Distribution channels have converged on one-liners — and SAF already speaks most of them

- **MCP:** Claude Desktop Extensions made MCP install one-click
  (anthropic.com/engineering/desktop-extensions); the de-facto stdio pattern is
  a JSON block with `npx <pkg>` (modelcontextprotocol.org docs;
  github.com/modelcontextprotocol/servers). ghx's direct-tools MCP config is
  already exactly this (README.md:545–554) and `ghx serve --recon` slots into
  the same block.
- **Skills ecosystems:** Vercel's `npx skills add` distributes SKILL.md files
  across Claude Code, Codex, Cursor, Gemini CLI, Copilot, Goose… "73+ agents",
  with skills.sh as directory/leaderboard (vercel.com/changelog/introducing-skills-the-open-agent-skills-ecosystem;
  github.com/vercel-labs/skills). ghx already prints copy-paste install lines
  for exactly this ecosystem (README.md:567–569).
- **Plugins:** Claude Code plugins bundle skills+agents+hooks+**MCP servers**
  into `/plugin install name@marketplace`, with an official marketplace
  auto-added on first interactive start (code.claude.com/docs/en/discover-plugins,
  fetched in full 2026-08-21). A plugin can carry both the recon skill AND the
  `ghx serve --recon` MCP registration — i.e., **the entire adoption surface in
  one marketplace line**. ghx already has the `.claude-plugin/plugin.json`
  manifest shape in-repo.
- **A2A:** 150+ orgs, LF-governed, v1.0, cloud-platform integrated
  (linuxfoundation.org press, Apr 2026; Forbes Aug 2026). ADR-0020.1 D3
  pre-registered the trigger: ship an AgentCard when the first concrete
  A2A-speaking consumer appears; the mapping is already designed
  (question→Task, report→typed Artifact, AgentCard=the small skill).

### 2.4 Benchmarks set category narratives before products do

The competitor that is winning attention did it with an arXiv preprint +
deterministic PASS/PARTIAL/FAIL benchmark, honestly reporting 83%-vs-92%
quality loss for 10× token savings (PA-0002.1 §2, citing arXiv 2603.27277).
SAFE — ghx's own pre-registered, audit-hardened eval machinery — is strictly
more rigorous (cross-family recomputation, TRUST.md ledger, committed negative
results) but is **not packaged as something an outsider can run**
(NORTH_STAR non-goal: "a public `ghx bench` CLI"). NORTH_STAR itself names the
public benchmark as a Consequence Product (docs/NORTH_STAR.md:255–257), and
PA-0001.9 tenet-evolution #3 proposes promoting it to a dated frontier item
because "whoever publishes the citable recon benchmark frames the category."

### 2.5 Cost/adoption levers others use that SAF lacks

Free-tier/host-bundled distribution (Desktop Extensions, official marketplaces),
metered cost transparency (competitor advertises "10× fewer tokens" as headline),
and auto-install into N hosts (codebase-memory-mcp writes MCP entries into 11
agent configs; PA-0002.1 E-anchor). ghx counters with npx/brew/go install and
`doctor`, but has no equivalent "write myself into your agent's config" command
and no public number a prospect can quote. The honest internal counter-number —
16.6× compression, THESIS SUPPORTED fixbatch (NORTH_STAR C3 row) — is buried in
evals docs, caveated by 2/6 discrimination (PA-0001.9 R2), and invisible in the
README above the fold.

---

## Part 3 — Candidate "no-brainer adoption paths"

Four concrete wedges, each with demand/cost/risk evidence. They are ordered by
(evidence-of-demand ÷ build-cost); none requires touching code in this task.

### Wedge A — "One-line MCP install" of the recon service as the flagship path

**What:** Make `ghx serve --recon` the documented, default, one-line adoption
path: a copy-paste `mcpServers` JSON block using `npx @gkoreli/ghx serve --recon`
(plus a `ghx mcp-config` printer command later), positioned above the fold in
the README next to the quickstart, and shipped as a Claude Code plugin whose
plugin manifest registers both the recon skill and the recon MCP server.

- **Demand evidence:** MCP one-click install is the industry's proven adoption
  mechanic (Anthropic Desktop Extensions, anthropic.com/engineering/
  desktop-extensions); plugins bundle "skills, agents, hooks, and MCP servers"
  with an official marketplace auto-added for every interactive user
  (code.claude.com/docs/en/discover-plugins). ghx's own README already shows
  the identical JSON shape for direct tools (README.md:545–556) — the recon
  variant is the same block with one flag.
- **Cost:** near-zero build (config text + docs ordering + manifest fields;
  the plugin manifest already lists skills — verified C3); medium writing cost
  to keep the ≤30-line recon promise honest while doing it.
- **Risk:** the recon MCP tool still returns JSON-plus-prose (serve.go:208–221)
  — shipping this wedge before fixing the envelope means the first programmatic
  consumer hits PA-0001.9 R3's broken-parse landmine on day one. Also: MCP
  wiring grants the tool but not auth; `doctor --live` must stay the advertised
  second step (ADR-0033 D4 exists precisely for this).
- **North-star filter:** passes cleanly — removes knowledge/tokens from the
  main agent's context at the point of adoption (B2 = current frontier).

### Wedge B — Skills-ecosystem distribution (the `npx skills add` rail)

**What:** Treat the recon skill as a distributable package on the Vercel-style
skills rails (skills.sh directory, `npx skills add gkoreli/ghx --skill ghx-recon`)
and make the recon skill the one that self-installs the *service expectation*
("this skill needs `ghx serve --recon`; run `npx @gkoreli/ghx sidecar doctor`").

- **Demand evidence:** the skills ecosystem reaches "73+" agents with one
  command (vercel.com/changelog/introducing-skills-the-open-agent-skills-ecosystem;
  github.com/vercel-labs/skills); marketplaces/directories exist for discovery
  (skillsmp.com, agenticskills.io). ghx README already prints these exact
  commands for the heavy skills (README.md:567–569) — but ships the heavy ones,
  not recon, in its examples.
- **Cost:** very low — the recon SKILL.md exists; the work is packaging,
  metadata frontmatter (already present), and choosing which skill the examples
  advertise. Fixing the 47-vs-30-line budget drift (README.md:354 vs measured)
  belongs here.
- **Risk:** a skill without a working backend is a dead end — the skill must
  degrade gracefully when ghx isn't installed/authed, or first impressions die
  in the FRICTION.md "silent nothing" class. Cross-agent parity is untested
  (skill assumes MCP-or-CLI fluency patterns of Claude Code).
- **Filter:** passes (same clause as A).

### Wedge C — Publish SAFE as a runnable public benchmark (benchmark-as-marketing)

**What:** Ship the narrowest possible public runner over the existing eval
corpus: fixed tasks, committed episodes, recomputable scoring, a leaderboard-
shaped page — explicitly NOT the general-purpose eval framework (non-goal,
NORTH_STAR.md:270), just "run your agent against our tasks, see your
compression/correctness next to ours."

- **Demand evidence:** the category's public numbers are being set *right now*
  by codebase-memory-mcp's arXiv benchmark (27.9k★ velocity; PA-0002.1 §2
  strategic warning: "whoever publishes the citable recon benchmark frames the
  category"); NORTH_STAR already lists the public benchmark as a Consequence
  Product (:255–257) and PA-0001.9 #3 promotes it to dated-frontier.
- **Cost:** moderate-high honesty-preserving engineering: the machinery exists
  (deterministic scorers, committed artifacts, TRUST.md discipline), but
  opening it invites contamination, gaming, and maintenance; gates/freeze rules
  must be respected (AGENTS.md measurement-stack rules). Needs its own ADR.
- **Risk:** highest of the four. Current citable numbers are conservative-floor
  with 2/6 discriminating tasks and the quality half unmeasured (PA-0001.9 R2;
  TRUST.md H1/H7) — publishing before the corpus refresh (R1–R4 landed
  Unreleased, canaries pending) and judge calibration (κ) would hand critics
  the "your own ledger says 2/6" quote. The repo's truthfulness tenets make
  premature publication a self-inflicted wound. Sequence: corpus refresh +
  κ unlock → then publish.
- **Filter:** passes via the third role of evals ("marketing material and
  bragging rights", NORTH_STAR.md:186–189).

### Wedge D — Selection-eval: prove sidecar ≥ host-native subagent on the host's own turf

**What:** Extend ADR-0032.1's host-task harness with the arm PA-0001.9 R4(c)
names: `plain-host` vs `host+ghx-recon` vs `host+native-subagent` on identical
engineering objectives — measuring BOTH halves of the thesis claim (main-task
quality non-inferiority + recon quality/compression), then publish that one
comparison as the hero artifact of the README.

- **Demand evidence:** this is the exact question a skeptical adopter asks
  ("why not just use Explore?") and the exact gap the audits identify: the
  "both, not one" rebuttal is asserted (NORTH_STAR.md:46–57) but C8 has zero
  episodes (NORTH_STAR.md:331; PA-0001.9 H2). Gates are already registered —
  success non-inferiority δ=0.10, efficiency superiority, deterministic recon
  quality, vector verdict (NORTH_STAR C8 row; ADR-0032.1).
- **Cost:** moderate — harness design decided (ADR-0032.1 S1–S4 slices), needs
  the founder-triggered run plus one added arm; no new theory.
- **Risk:** the result might come back ambiguous or negative on some cells —
  which, per the repo's own tenets (negative results kept, verdicts floor-
  labeled), is survivable and useful, but it must be pre-committed to publish
  regardless, or the exercise is marketing theater.
- **Filter:** passes decisively (it IS the measurement that converts
  "advantage" into "Power" per PA-0001.9 H3).

**Sequencing recommendation (from the evidence):** A+B immediately (near-free,
pure surface, B2-aligned) → D as the first measured milestone (gates already
registered) → C only after corpus-refresh canaries and κ land (else the
benchmark publishes the current 2/6 weakness). D's output becomes the number
A/B/C all quote.

---

## Commands run (appendix)

```
C1. cd /tmp && npx -y @gkoreli/ghx --version        → "ghx version 2.9.1"   (zero-install works)
C2. go build -o /tmp/ghx-research ./cmd/ghx && /tmp/ghx-research sidecar --help
    /tmp/ghx-research serve --help                  → --recon flag present; 7-tool default described
    /tmp/ghx-research skill --recon | wc -l         → 47 (vs claimed ≤30)
    wc -l skills/{ghx,ghx-mcp}/SKILL.md             → 232 / 216
C3. cat .claude-plugin/plugin.json                  → skills list = [ghx-recon FIRST, ghx, ghx-mcp] (H1 remediated)
    sed -n '195,235p' internal/cli/serve.go          → JSON+prose suffix on recon tool result (R3 still open)
    grep -n "schemaVersion" internal/sidecar/report.go → present (line 43; R5 since landed)
    git log --oneline -8                            → ADR-0037–0039 accepted at HEAD f951403
```

Files read (key): AGENTS.md, README.md, docs/NORTH_STAR.md, docs/dogfood/FRICTION.md,
docs/evals/TRUST.md, docs/adr/{0019, 0020.1, 0026, 0028.1, 0030, 0031(first 80),
0033}.md, docs/audits/adversarial-velocity-2026-07-07.md, docs/product-audit/
{PA-0001.9-distilled, PA-0002.1-competitive-absorption}.md, docs/research/
001-cheap-sidecar-backend.md, CHANGELOG.md (head), .claude-plugin/plugin.json,
internal/cli/serve.go (recon handler), skills/ghx-recon/SKILL.md.

External sources fetched beyond search snippets (full-page):
code.claude.com/docs/en/discover-plugins, code.claude.com/docs/en/sub-agents.
Snippet-level web searches (12 total): Claude Code subagents, OpenAI Agents SDK
returns, MCP one-click installs, Cursor subagents, ACP editor adoption, A2A/LF,
Copilot agents+MCP GA, Hermes Agent delegation, codebase-memory-mcp, skills
ecosystems (Vercel/skills.sh/marketplaces), Claude Agent SDK subagents.

## Uncertainty

1. **Adoption-channel share is unmeasured.** Whether prospects arrive via npm,
   brew, plugin marketplace, or skills directories is unknown; PA-0001.9's
   caveat that impact is channel-dependent applies to every wedge ranking here.
2. **No usage telemetry exists** (local-first tenet), so "what makes adoption
   annoying today" is inferred from maintainer dogfood friction, not from a
   cohort of external users. The first 10 external users' FRICTION-equivalent
   logs would likely reorder these findings.
3. **The landscape moves monthly** (ADR-0026's own caveat). Mastra/ADK were
   closest competitors in July; host-native subagents improve quarterly; any
   of them shipping an evidence-contract return would erode Wedge A/B's
   differentiation overnight. Re-scan cadence should be monthly until C lands.
4. **Wedge D's outcome is genuinely unknown.** If the selection-eval shows the
   host-native subagent matching ghx-sidecar on combined objectives, the
   correct move per the repo's own honesty rules is to publish that and
   reposition on remote/discovery — not to bury it.
5. **Cost claims are not independently verifiable end-to-end:** production
   session metrics undercount subject-agent tokens (FRICTION.md 2026-07-07,
   open), so any "cheap" marketing number derived from sessions is currently
   unsound; only eval-side real-token accounting (TRUST.md H5) partially covers.

## Suggested next reads

- ADR-0031.1 + 0031.2 (anticipation v1 decision and its failed gate) — the
  latency half of the "second question pays rent" experience.
- docs/product-audit/PA-0002.4-capability-eval-reality.md — capability claims
  vs reality, adjacent to Wedge C sequencing.
- ADR-0032 (combined-objective host-task evals research) + ADR-0032.1 — the
  registered gates Wedge D would extend.
- docs/adr/0037–0039 (session memory eviction, guardrails, report schemaVersion)
  — freshly landed at HEAD; they harden exactly the contract a new adopter
  consumes.
- External: OpenAI Agents SDK `custom_output_extractor` docs; Zed ACP blog +
  morphllm ACP-vs-MCP overview (editor-side adoption of ACP, relevant to
  future ACP-native distribution of SAF).
