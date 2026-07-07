# OSS Reconnaissance — "Product-Manager Agents" & Agentic-First Product Tooling

*Research artifact #7 for the `product-audit` skill. Prior artifacts (01–06) mined PM
**thought-leadership**; this one mines the **implementation** landscape — what people have
actually BUILT — and judges each against our authored skill
(`.claude/skills/product-audit/SKILL.md`). Tenet: explore OSS first, steal greatly
(Picasso), hand-roll only when nothing serves the vision.*

Sourcing discipline: every row carries a specific deep GitHub URL + one line on why it
matters; maturity (stars / last-push) verified live via `gh` on 2026-07-07. My own
synthesis is marked **[SYNTHESIS]**. Nothing here is inflated into "prior art" that isn't.

---

## 1. Abstract

The honest headline is a **split verdict**, not a single RICH/THIN. The literal category
we went hunting for — an OSS **"product-audit skill" or "PM agent" that pressure-tests a
product from adversarial PM personas** — is **THIN to empty**: it is a graveyard of
0–2-star demos (PRD generators, "turn an idea into a spec" toys, CrewAI three-agent
starters). The one real Claude *skill* in the space (`skill-ultra-product-manager`) is
pure human-SaaS canon (Nielsen literally, onboarding, SSO/GDPR/SOC2, pricing/PLG) — i.e.
exactly the advice our agentic-first lens (`06`) re-prices as OUTDATED-OR-INVERTED. So on
**prior art we are appropriately hand-rolling**, and the closest artifact actually
*confirms* our differentiator rather than threatening it.

But the hunt surfaced a **genuinely RICH adjacent vein we had not mined: built Agent
Experience (AX) tooling for CLIs.** Three small-but-real projects — `axprobe`,
`frictionax`, `ax-audit` — plus the mature eval frameworks (`promptfoo`, `deepeval`) and
one big-lab signal (Meta/Harvard's Confucius Code Agent framing harness design as
**AX / UX / DX**) contain **concrete, enumerated failure taxonomies and methodologies our
skill names but never itemizes.** Our AX lens (#16) is a correct umbrella with four
pillars; what these repos add is the **checklist under the umbrella** — a friction
taxonomy, an attack taxonomy, an agentic-metric decomposition, and a "deliberately weak
driver" harness method. That is the real deliverable: not "adopt a tool," but **steal
three specific structures** that make our AX and agent-safety scopes operational.

Category-wise: "a PM *agent* for agentic-first products" is **not** an OSS category yet
(the built artifacts don't exist); but "**Agent Experience tooling**" and "**agentic-SDLC
role fleets**" (BMAD, 50k★) *are* live categories worth tracking against ghx's "many
brains, many domains" thesis.

---

## 2. Landscape table

Verdict key: **ADOPT** (use directly) · **STEAL-PATTERN** (borrow a specific
rubric/taxonomy/method) · **IGNORE** (thin wrapper / abandoned / demo / wrong axis).

| Repo | What it does | Maturity | Verdict | Why + URL |
|---|---|---|---|---|
| **segmentstream/axprobe** | Single-binary Go harness that drives a real CLI inside a Docker box with an LLM and emits a structured **AX report** (outcome, goal-reached, human-intervention count, friction categories, false-errors, tokens/cost). | 1★, active (2026-06-25), real tool w/ JSON schemas & tests | **STEAL-PATTERN** (top pick) | Its report schema *is* an operational Phase-6 rubric; the "deliberately weak driver" principle is a cheap sharp method. https://github.com/segmentstream/axprobe/blob/main/schema/report.schema.json |
| **sageox/frictionax** | Go library: detects when a user/agent types a wrong CLI command, suggests/auto-fixes, and telemeters **"desire paths"** — commands agents keep expecting that don't exist yet. | 4★, 2026-03-27, extracted from a real product (ox CLI) | **STEAL-PATTERN** | "Desire-path" trace-mining is a concrete gap-analysis method; the `query`-command discovery story is a worked example. https://github.com/sageox/frictionax/blob/main/README.md |
| **promptfoo/promptfoo** | LLM/agent test + red-team framework; ~60 red-team plugins incl. an **agentic** family (memory-poisoning, excessive-agency, goal-misalignment, indirect-prompt-injection, MCP, cross-session-leak, BFLA/BOLA, coding-agent graders). | 23,000★, daily commits — flagship | **STEAL-PATTERN** | Its plugin catalog is the attack taxonomy our "agent-safety / adversarial tool-use" scope names but never enumerates. https://github.com/promptfoo/promptfoo/tree/main/src/redteam/plugins |
| **confident-ai/deepeval** | LLM eval framework with an explicit **agentic-metric** decomposition: ToolCorrectness, ArgumentCorrectness, TaskCompletion, ToolUse, MCPTaskCompletion, plus Faithfulness/Hallucination + community CitationFaithfulness. | 16,700★, daily commits | **STEAL-PATTERN** | Decomposes "did the agent do the job" into namable sub-metrics — sharpens evals-as-user-research beyond "task success". https://github.com/confident-ai/deepeval/blob/main/deepeval/metrics/__init__.py |
| **lucioduran/ax-audit** | TS CLI that audits **websites** for AX readiness — 18 weighted checks (llms.txt, robots for AI crawlers, JSON-LD, A2A `agent.json`, `/.well-known/mcp.json`) with exact source-extracted scoring. | 5★, 2026-06-09, real docs/CI | **STEAL-PATTERN (rubric rigor) / mostly IGNORE (web-only)** | Web-crawler checks don't apply to a CLI; but its **mcp/agent-card manifest-completeness rubric** + the discipline of "exact deduction per check" are stealable. https://github.com/lucioduran/ax-audit/blob/main/docs/checks.md |
| **facebookresearch/cca-swebench** (Confucius Code Agent) | Meta/Harvard production coding agent (Feb 2026); its SDK frames harness design around three perspectives: **Agent Experience (AX) / User Experience (UX) / Developer Experience (DX)**. | 39★ (repro repo); backed by a major lab | **STEAL-PATTERN (framing)** | Big-lab validation that AX is a first-class design axis, and a cleaner tri-split (AX/UX/DX) than our agent-vs-operator binary. https://github.com/facebookresearch/cca-swebench |
| **bmad-code-org/BMAD-METHOD** | "Agile AI-Driven Development": a **fleet of agent personas** across the SDLC (analyst, PM "John", architect, PO, UX, dev, QA), each a skill; PRD-authoring + `prd-validation-checklist.md`. | 50,188★ — the category's giant | **IGNORE (for adoption) / TRACK (category signal)** | It's a *build-side* PRD-author fleet for human-target software, not a consumer-side agentic-first auditor — wrong axis. But it's the closest live realization of "many brains, many domains." https://github.com/bmad-code-org/BMAD-METHOD/tree/main/src/bmm-skills/2-plan-workflows/bmad-agent-pm |
| **Lithium-Prime/skill-ultra-product-manager** | Claude skill: audits SaaS across 6 dimensions (strategy, UX, commercial, enterprise, analytics, architecture) via reference checklists + report template. | 2★, 2026-01-20, demo-grade | **IGNORE (confirms our thesis)** | The *only* real OSS "PM audit skill" — and it's 100% human-SaaS canon (Nielsen literal, onboarding, SSO/GDPR/SOC2/PLG). It is the artifact our `06` lens exists to *reject*. https://github.com/Lithium-Prime/skill-ultra-product-manager/blob/main/SKILL.md |
| **Ashishjain608/agent-experience** | Long-form AX field guide: "6 practices that survived 3-vote adversarial fact-checking + 16 popular claims that didn't"; case study mining 380 Claude Code sessions into a `workflow.md`; DIY checklist. | 0★, just published (2026-07-07), blog not tool | **STEAL-PATTERN (minor)** | The "16 claims that didn't survive" negative-list and "bloat is a correctness bug" framing reinforce our token-economics anti-patterns. https://github.com/Ashishjain608/agent-experience |
| **google/ax** | Google's open-source **distributed agent runtime** (name collision — "ax" = the runtime, not "agent experience"). Has audit/policy of agentic calls via a common controller. | 1,839★, active | **IGNORE (name collision)** | Not an AX-audit tool despite the name; a runtime. Noted to avoid mis-citing it as prior art. https://github.com/google/ax |
| **deepeshgupta12/pm-agent-os** | Full-stack "PM agent OS" web app (FastAPI/alembic, RBAC, evidence-store, approvals) that runs PM agents on artifacts. | 7★, 2026-03-20, substantial app | **IGNORE** | A workflow-automation product, not an audit rubric/skill; nothing to steal for our purpose. https://github.com/deepeshgupta12/pm-agent-os |
| **shugavibes/shuga-pm-agents** | "PM assistant on autopilot": agents for cards/design/stakeholders/mobile that draft tickets from meetings, push to Notion/Slack. | 7★, 2026-03-31 | **IGNORE** | PM *task automation* (drafting), not product auditing; human-workflow, no re-pricing. https://github.com/shugavibes/shuga-pm-agents |
| **~40 "PM agent / PRD generator / product-strategy agent" demos** | `crewai-product-*`, `prd-generator-*`, `product-strategy-agent-*`, `autogen-product-*`, `B-oxygen/prd-mas`, etc. | Almost all 0–2★, single-author, weeks-old | **IGNORE (committed negative)** | Assessed as a cluster: thin CrewAI/n8n wrappers that turn an idea → a spec. **No adversarial-persona product auditor exists among them.** e.g. https://github.com/nabian1990amber-cmd/crewai-product-planner-startertest |
| **awesome-claude-skills (Composio 67k★ / karanb192)** | Curated skill directories (1000+ / 50+ skills). | Flagship lists | **IGNORE (searched, negative)** | Scanned both: **no product-audit / PM-strategy skill exists.** Closest is `great_cto` (SDLC subagents incl. a *code* "project-auditor") — code/security, not product. https://github.com/ComposioHQ/awesome-claude-skills |

*(promptfoo/deepeval/ragas 14.7k / inspect_ai 2.3k / openai-evals 18.9k / Arize-phoenix
10.4k all verified mature; ragas/inspect/evals/phoenix are eval *infrastructure* already
pointed-to by our `Evals-as-user-research` scope — no new rubric beyond deepeval's metric
decomposition, so folded in rather than tabled separately.)*

---

## 3. What we can steal (each mapped to the exact section it improves)

**S1 — axprobe's AX report contract + "weak-driver" method → Manual-validation Playbook,
Phase 6 (SKILL.md "Surfaces"), and the Tier-B Tool/affordance scope.**
axprobe emits a per-run contract we should adopt almost verbatim as Phase-6 record fields:
`goal_reached` (distinct from "terminated"), **`human_interventions` (HIC)** as a headline
number, **`false_errors`** (a non-zero exit that was *not* a real failure — a specific,
common ghx failure mode we don't currently name), a **friction taxonomy** — `missing_guidance`
(tool didn't say what to do) · `confusion` (confusing/false error or misleading output) ·
`extra_steps` · `friction` (worked but awkward) · `unclear_interface` (confusing
names/flags/output) — plus `post_mortem` and `tokens/cost`. And its load-bearing *method*:
drive ghx with a **deliberately weak model** — "if a weak model can drive it, the UX is
genuinely good; a clever harness papers over the very defects you want to surface."
Also: `expect: {goal_reached, max_human_interventions}` as a **pass/fail gate** turns an AX
run into an *executable spec* (write it red before the feature). Our Phase 6 today records
"binary success / tool-calls vs minimum / self-correction / checkable claims"; S1 upgrades
it with HIC, false-errors, a 5-category friction vocabulary, and the weak-driver principle.
→ https://github.com/segmentstream/axprobe/blob/main/schema/report.schema.json ·
driver categories: https://github.com/segmentstream/axprobe/blob/main/internal/driver/prompt.go

**S2 — promptfoo's red-team plugin taxonomy → Tier-B "Agent-safety / adversarial tool-use"
scope.** Our scope currently names "lethal trifecta / prompt-injection surface" but gives
the auditor **no enumerated attack list**. promptfoo ships one, maintained and agent-specific:
`agentic/memoryPoisoning`, `excessiveAgency`, `goalMisalignment`, `indirectPromptInjection`,
`mcp`, `crossSessionLeak`, `debugAccess`, `bfla`/`bola` (broken function/object-level auth),
`contextComplianceAttack`, `asciiSmuggling`, `hijacking`, `dataExfil`, and a `codingAgent`
grader/verifier family. Lift the subset relevant to a recon sidecar (injection via
surfaced repo content → the sidecar's report → the main agent's next action;
cross-session/artifact leakage from `~/.ghx/sessions/`; excessive-agency if output feeds an
autonomous loop) as a **named checklist** in that scope.
→ https://github.com/promptfoo/promptfoo/tree/main/src/redteam/plugins

**S3 — frictionax's "desire-path" trace-mining → Gap-Analysis scope + Evals-as-user-research
surface.** Instead of only mining traces for *failed invocations* (our A2 framing),
frictionax mines the corpus of *wrong/near-miss commands agents typed* to find **desire
paths** — features the agent population keeps reaching for that don't exist. The worked
example (agents repeatedly typing a `query` command until the team built it) is exactly the
"observed use, not opinion" evidence the User-Empath (#2) and Gap-Analysis scope demand,
and it's agent-native (the trajectory is logged, not surveyed). Add "mine near-miss/desire
paths, not just failures" to the trace-mining method.
→ https://github.com/sageox/frictionax/blob/main/README.md

**Secondary steals (lower priority):**
- **deepeval's agentic-metric decomposition** (ToolCorrectness / ArgumentCorrectness /
  TaskCompletion / ToolUse / MCPTaskCompletion) → *Evals-as-user-research* scope: gives the
  auditor sub-metrics to demand beyond a single "task success," and MCPTaskCompletion is
  directly ghx-MCP-relevant. → https://github.com/confident-ai/deepeval/blob/main/deepeval/metrics/__init__.py
- **ax-audit's mcp/agent-card manifest-completeness rubric** → *Distribution-in-agent-
  ecosystems* scope: if ghx ever advertises an MCP server via `/.well-known/mcp.json` or an
  A2A card, its per-field deduction rubric (name/description/tools-have-descriptions/
  resources present) is a ready checklist. → https://github.com/lucioduran/ax-audit/blob/main/docs/checks.md
- **Confucius's AX/UX/DX triad** → *directive 3 ("two consumers")*: consider a **third**
  axis, Developer/operator-Experience-of-*extending*-ghx, distinct from the human *operator*
  and the *agent* consumer. → https://github.com/facebookresearch/cca-swebench

---

## 4. Blind spots this reveals in our skill

**Yes — one genuine, actionable blind spot, plus two smaller ones.**

1. **[GENUINE] Our AX and agent-safety scopes are umbrellas with no enumerated failure
   checklist.** SKILL.md #16 correctly lists four AX *pillars* and the Tier-B table names
   the safety scope — but an auditor handed these has no itemized list of *specific failure
   modes to probe*. The OSS world has already itemized both: axprobe's **friction taxonomy +
   HIC + false-errors** (S1) and promptfoo's **agentic attack taxonomy** (S2). This is the
   highest-value fill: convert two umbrellas into two checklists. Concretely missing from us
   today: `false_errors` (non-zero exit that isn't a real failure), **HIC as a headline
   metric**, `missing_guidance`/`unclear_interface` as named friction classes, and
   `memoryPoisoning`/`crossSessionLeak`/`excessiveAgency` as named attack classes.

2. **[METHOD] The "deliberately weak driver" inversion is absent from Phase 6.** We say "run
   a real coding-agent session" but don't specify driver strength. axprobe's principle —
   *a strong harness hides AX defects; drive with the weakest model that should plausibly
   succeed* — is a cheap, sharp upgrade that also lowers eval cost. **[SYNTHESIS]**

3. **[MINOR] Trace-mining is failure-only.** We mine failed invocations; frictionax shows
   the *desire-path* (near-miss) signal is equally rich and more forward-looking (S3).

Everything else our skill already covers *better* than the OSS: no OSS artifact has our
adversarial-persona roster, our agentic-first re-pricing table (`06 §3`), the self-preference
judge guard, or the north-star filter. The blind spot is **granularity inside two scopes**,
not a missing scope.

---

## 5. Is this a category? (relevant to ghx's "framework outgrows ghx" thesis)

**[SYNTHESIS] Split answer:**

- **"A PM *agent* for agentic-first products" is NOT an OSS category** — the built artifacts
  are 0-star demos. The *thought-leadership* is real and growing (Every's "Agent-native
  Product Management" guide; Mind the Product; the "TASK" framework; the "ladder of
  autonomy"), and the *roles* are institutionalizing (Microsoft "Principal PM, Agentic
  Experiences"; OpenAI "PM, API Agents") — but that's writing and hiring, not shipped code.
  Our prior research already covered the thought-leadership layer; nothing new to adopt here.
- **"Agent Experience (AX) tooling" IS an emerging built category** worth tracking
  competitively: `axprobe`, `frictionax`, `ax-audit`, `agent-xlsx`, plus big-lab
  institutionalization (Meta/Harvard **Confucius: AX/UX/DX**; Biilmann/Netlify coined AX;
  a nascent standards stack — llms.txt, A2A agent-card, `/.well-known/mcp.json`). This is
  the space adjacent to ghx's own home turf; ghx should watch it and could *contribute* a
  CLI/recon-sidecar AX benchmark to it.
- **"Agentic-SDLC role fleets" IS a live category** — **BMAD-METHOD (50k★)** is the closest
  existing realization of ghx's "**many brains, many domains**" north-star thesis: a fleet
  of specialized agent personas (analyst/PM/architect/PO/UX/dev/QA), each a skill, across
  the whole lifecycle. It's build-side and human-target (not a recon sidecar), so it's a
  **proof-of-thesis and a landscape marker, not a direct competitor** — but if the framework
  outgrows ghx into a multi-domain fleet, BMAD is the incumbent shape to study and
  differentiate against (ghx's edge: evidence contract, agent-as-consumer, north-star filter).

**Bottom line on category:** track **AX tooling** and **BMAD-class role fleets**; do *not*
treat the "PM agent" demo swarm as competition.

---

## 6. Source Ledger

| # | Repo / source | Deep URL | Verified | Why it matters |
|---|---|---|---|---|
| 1 | segmentstream/axprobe — report schema | https://github.com/segmentstream/axprobe/blob/main/schema/report.schema.json | ✓ gh (fields read) | Adoptable AX report contract (HIC, false-errors, friction, tokens) |
| 2 | axprobe — driver friction categories | https://github.com/segmentstream/axprobe/blob/main/internal/driver/prompt.go | ✓ gh (enum read) | 5-category friction taxonomy + "weak driver" method |
| 3 | sageox/frictionax — README | https://github.com/sageox/frictionax/blob/main/README.md | ✓ gh | Desire-path trace-mining; CLI error→correction for coding agents |
| 4 | promptfoo — red-team plugins dir | https://github.com/promptfoo/promptfoo/tree/main/src/redteam/plugins | ✓ gh (tree listed) | Enumerated agentic attack taxonomy |
| 5 | confident-ai/deepeval — metrics init | https://github.com/confident-ai/deepeval/blob/main/deepeval/metrics/__init__.py | ✓ gh (classes read) | Agentic-metric decomposition incl. MCPTaskCompletion |
| 6 | lucioduran/ax-audit — checks.md | https://github.com/lucioduran/ax-audit/blob/main/docs/checks.md | ✓ gh (full read) | 18-check web-AX rubric; mcp/agent-card manifest checks |
| 7 | facebookresearch/cca-swebench (Confucius) | https://github.com/facebookresearch/cca-swebench | ✓ gh (39★) + WebSearch | Big-lab AX/UX/DX design triad |
| 8 | bmad-code-org/BMAD-METHOD — pm skill | https://github.com/bmad-code-org/BMAD-METHOD/tree/main/src/bmm-skills/2-plan-workflows/bmad-agent-pm | ✓ gh (SKILL read, 50k★) | Category marker: agentic-SDLC role fleet |
| 9 | Lithium-Prime/skill-ultra-product-manager | https://github.com/Lithium-Prime/skill-ultra-product-manager/blob/main/SKILL.md | ✓ gh (full read) | Only real OSS PM-audit skill; pure human-SaaS canon |
| 10 | Ashishjain608/agent-experience | https://github.com/Ashishjain608/agent-experience | ✓ gh | AX field guide; "bloat is a correctness bug"; 16 debunked claims |
| 11 | ComposioHQ/awesome-claude-skills | https://github.com/ComposioHQ/awesome-claude-skills | ✓ gh (README grep) | Negative: no product-audit skill in 1000+ |
| 12 | Every — Agent-native Product Management | https://every.to/guides/ai-product-management-guide | WebSearch only (title/URL) | Category signal (thought-leadership) |
| 13 | Microsoft — "Principal PM, Agentic Experiences" | https://microsoft.ai/job/principal-product-manager-agentic-experiences/ | WebSearch only (title/URL) | Category signal (role institutionalizing) |

**Unverified / caveats:** rows 12–13 are confirmed only via WebSearch result metadata
(title + URL), not fetched full-text — cited as *category signal*, not as artifacts to
steal. `google/ax` was verified as a runtime (name collision), explicitly **not** counted
as AX-audit prior art. The Confucius **AX/UX/DX** framing comes from a WebSearch summary of
the CCA blog corroborated by the existence of the `facebookresearch/cca-swebench` repo; the
exact triad wording should be re-confirmed against the primary CCA post before quoting it
externally. Star counts and last-push dates are a 2026-07-07 snapshot.

---

## 7. Bottom-line verdict

**Split: THIN on the literal target, RICH one hop over — and it *does* change our skill.**

- **Prior art (Q1): THIN — we are correctly hand-rolling.** There is no OSS
  adversarial-persona, agentic-first product-audit skill. The single real PM-audit skill is
  human-SaaS canon and, tellingly, is precisely what our `06` lens rejects — so the closest
  artifact *validates* our approach rather than obsoleting it. **No adoption; keep hand-rolling.**

- **Blind spot (Q2): YES — a real, bounded one.** Not a missing scope, but missing
  *granularity inside two scopes we already have*. Add axprobe's **friction taxonomy + HIC +
  false-errors + weak-driver method** to Phase 6 / the Tool-affordance scope, and promptfoo's
  **agentic attack taxonomy** to the agent-safety scope. This is the one change worth making
  to the skill on the strength of this recon.

- **Category (Q3): worth tracking — but not the "PM agent" demos.** Track **AX tooling** (the
  live adjacent category) and **BMAD-class agentic-SDLC role fleets** (the closest realization
  of ghx's "many brains, many domains" thesis). The "PM agent" swarm is not competition.

Net: this was worth doing. It produced a committed negative on prior art (valuable on its
own), a RICH adjacent find we hadn't mined, and exactly two concrete edits to the skill.
