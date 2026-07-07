# The Agentic-First Lens — Auditing a Product Whose Customer Is an AI Agent

*Research artifact #6 for the "product audit" skill. Its job is twofold: (A) build
the mental model for what changes when the **primary consumer of a product is an LLM
agent, not a human**, and (B) **adversarially audit the five prior PM-research
artifacts** (01–05) against that model — labelling each load-bearing piece of classic
advice HOLDS / ADAPT / OUTDATED-OR-INVERTED for an agent-consumer product.*

Grounding product: `ghx`, a Go/Cobra CLI + "code reconnaissance sidecar" whose
customer is a **main AI agent** (Opus/Fable/an engineer agent). The main agent
delegates reconnaissance and receives a schema-validated evidence report. North-star
constraints (`docs/NORTH_STAR.md`): the main agent needs **zero knowledge** of the ghx
CLI; ghx **removes tokens/knowledge from the main agent's context**; "**the main
agent's context is sacred**"; "**evidence, not vibes**"; the optimization target is
"**signals per token**." The human founder is a *secondary* consumer (dogfooding).

Sourcing discipline: every substantive external claim carries a specific deep URL and a
one-line authority rationale (Source Ledger, §6). Prior artifacts are cited as `[01]`–
`[05]` with short verbatim quotes. My own extrapolation is marked **[SYNTHESIS]**;
claims I could not source cleanly are flagged in-line and collected in the return note.

---

## 1. Abstract

Ninety-five percent of the product-management canon assembled in artifacts 01–05 was
written about a consumer with a face, a visual field, imperfect memory, felt emotion,
and a monthly budget of *attention and clicks*. The agent consumer has none of these.
It is a **stochastic, context-limited language model that perceives the product only as
text, "remembers" only what currently sits in a finite context window, feels no
delight, cannot be surveyed, and pays in tokens**. This single substitution — human →
LLM — does not invalidate product management, but it re-prices every heuristic. Some
classic advice survives unchanged (outcomes-over-output, the build-trap warning,
jobs-to-be-done, 7-Powers, the entire adversarial/red-team stack, and the PM-craft
virtues, because those describe the *builder*, not the *consumer*). Some survives only
after its mechanism is swapped out (Nielsen's heuristics, journey mapping, positioning,
discovery-risk, docs audits — the *principle* holds, the *instrument* changes from
"screen a human perceives" to "schema a model parses"). And a hard core of it **inverts
or dies**: human onboarding and first-run polish (the north star is *zero* onboarding
for the main agent), the Sean-Ellis "how would you feel" survey (an agent has no felt
disappointment), "delight," and — the sharpest inversion — **engagement/retention as a
goal**: for a product whose entire thesis is *removing* itself from the customer's
context, more time-in-tool and more tool calls per task are *vanity metrics pointing
the wrong way*; the win is the agent getting a high-signal answer and **leaving fast**.
The mental model that replaces "usability" is **Agent Experience (AX)** (Biilmann): can
the agent *discover* the tool, *invoke* it reliably, *recover* from its errors, *not be
misled* by it, and *compose* its output — all measured not by surveys but by **evals**,
which are the agent-native form of user research. The unit of everything is
**signals per token**, because the agent's context is a finite resource with
diminishing marginal returns (Anthropic). This artifact ends with the new scopes,
methods, and personas the audit skill must add, and an explicit list of human-centric
heuristics it must **warn against cargo-culting** onto an agent product.

---

## 2. The Agent-as-Consumer Mental Model

Eight shifts. Each is the load-bearing substitution that the rest of the artifact
audits against.

### (a) The user is a stochastic, context-limited LLM that pays in tokens → "signals per token" / context as a scarce resource

The classic canon assumes attention is scarce and measured in seconds and clicks. For
an agent, the scarce resource is the **context window**, and it depletes per token.
Anthropic's context-engineering guidance is explicit: *"Context… must be treated as a
finite resource with diminishing marginal returns"*; *"Every new token introduced
depletes this budget… increasing the need to carefully curate the tokens available"*;
and the guiding principle is *"finding the smallest possible set of high-signal tokens
that maximize the likelihood of some desired outcome"* [Anthropic, Effective context
engineering]. This is the external, authoritative grounding for ghx's own "**signals
per token**" north star and its "**the main agent's context is sacred**" tenet — the
product exists to *subtract* low-signal tokens from the main agent's window while
returning high-signal evidence. Whereas a human PM asks "is this worth the user's
*time*," an agent PM asks "is this worth the model's *tokens/attention budget*." The
"cost of a feature" is no longer only build+support cost; it is the standing **context
tax** every tool description and every verbose report levies on the consumer's finite
window (Anthropic, Code execution with MCP: thousands of tools mean an agent must
*"process hundreds of thousands of tokens before reading a request"*; one worked
example cuts *"150,000 tokens to 2,000 tokens — a… saving of 98.7%"*).

### (b) The interface is schemas/tools/text contracts, not screens → tool design & affordances replace GUI usability

There is no screen to lay out, no visual hierarchy, no color. The entire surface the
agent perceives is the **tool/CLI contract**: names, descriptions, parameter schemas,
and text output. Anthropic frames the design discipline directly: *"think about how much
effort goes into human-computer interfaces (HCI), and plan to invest just as much
effort in creating good agent-computer interfaces (ACI)"* [Anthropic, Building
effective agents]. And tools are *"a new kind of software which reflects a contract
between deterministic systems and non-deterministic agents"* [Anthropic, Writing
effective tools for AI agents] — they must be designed for a caller that may
hallucinate, retry, or take multiple valid paths, not a deterministic client.
Biilmann's "AX" makes this a named discipline with four pillars — **Access** (can the
agent reach the product), **Context** (how it understands it), **Tools** (what it can
do), **Orchestration** (how it combines them) [Biilmann, Introducing AX]. "Usability
testing a mockup" is replaced by "does the tool schema make the right call obvious and
the wrong call unrepresentable."

### (c) "Usability" = can the agent reliably invoke it, recover from errors, and not be misled → error messages as affordances

For a human, an error message is a fallback you hope they never hit. For an agent
running *tools in a loop* (Willison's definition of an agent), the error message is a
**primary control surface**: it is the only thing steering the next iteration of the
loop, and the agent *"can usually recover from mistakes… and figure out the right
incantations without extra guidance"* — *if* the tool tells it how [Willison, Designing
agentic loops]. Anthropic's rule: tools should return *"specific and actionable
improvements, rather than opaque error codes or tracebacks"* and *"return only high
signal information back to agents"* [Anthropic, Writing effective tools]. "Being misled"
is a first-class failure the human canon barely has a word for: an agent that receives
a confidently-wrong recon report will *act* on it (edit code, ship it) with no human's
skeptical pause — which is why ghx's "**evidence, not vibes**" tenet and 03's rule that
*"a confidently wrong final report is a catastrophe-severity failure"* `[03 §8]` are not
niceties but the core of agent usability.

### (d) "Onboarding" = the skill/tool description & progressive disclosure, ideally ZERO doctrine in the main agent

The human "first-run experience" has no agent analog worth optimizing; its replacement
is **progressive disclosure of tool knowledge**. Anthropic's Agent Skills implement this
in three stages — *Discovery* (only the skill's name+description preload into context),
*Activation* (the full `SKILL.md` loads only if relevant), *Execution* (bundled
reference files load only when actually needed) [Anthropic, Equipping agents with Agent
Skills]. The design target for an *excellent* agent product is not "a great onboarding"
but **as little onboarding as physically possible** — the ghx north star's terminal
state is that *"a main agent… needs zero knowledge of the ghx CLI — it never loads the
ghx skill file at all"* (`NORTH_STAR.md`). This is a genuine inversion of the human
instinct to invest in a welcoming first-run: here, the ideal first-run is *invisible*.

### (e) "Delight/retention" = task success + low token cost + trust in evidence, measured by evals not surveys

An agent has no affect to delight and no habit to retain. The honest substitutes are
**task success** (did the agent complete the job correctly), **efficiency** (tokens,
tool calls, latency), and **trust/verifiability** (is the output checkable, or did it
fabricate). 03 already reaches this: it substitutes *"task success rate and evidence
quality for HEART's Happiness/Engagement and Ellis's 'very disappointed' survey — an
agent has no felt disappointment to survey"* `[03 §8]`. The agent-native form of
"delight" (if one insists) is **anticipation and zero-latency** (ghx M8: answering a
question before it is asked) and **zero-doctrine** (the tool that needs no skill file) —
but each is a *measured* token/latency/success gain, never an emotional surplus.

### (f) Discovery / user-research = eval suites, trace mining, and dogfooding real agent sessions

You cannot interview an agent about its needs, but you have something better: its full
**trajectory is logged**. Every "hesitation" is *"a literal logged action, not an
inferred mental state"* `[03 §8]`. So the agent-native replacements for interviews,
surveys, and usability labs are: **eval suites** (Hamel: *"unsuccessful products almost
always share a common root cause: a failure to create robust evaluation systems"*;
evals create a *"flywheel that allows you to iterate very quickly"* [Hamel, Your AI
product needs evals]); **trace mining** (reading real tool-call transcripts, the way ghx
mines eval/production traces for failed invocations — `NORTH_STAR.md` workstream A2);
and **dogfooding** real agent sessions (`[03 §7.1]`, which transfers essentially
unmodified). The public **agent benchmarks are the field's shared "user research"**:
SWE-bench (can a model resolve real GitHub issues) and τ²-bench (can an agent follow a
policy and coordinate tool use with a user, measured by *pass^k consistency* across
repeated trials) [SWE-bench; τ²-bench] are the discipline's standardized studies of "how
does the agent-consumer actually behave under real tasks."

### (g) Distribution / positioning = being the tool the agent (and its human) reaches for; MCP / skill ecosystems

"Getting adopted" splits into two motions. The **human operator** still installs,
configures, and routes (classic distribution/positioning holds for them). But the
**agent** must *select* the tool at call-time, and its "positioning" is literally the
tool's **name + description + when-to-use metadata** loaded into context — plus its
reachability through standard ecosystems. The Model Context Protocol exists precisely to
*"expose resources, tools and prompts via MCP primitives"* through a standard
client-host-server contract so any host can reach any server [MCP spec]. OpenAI's
guidance makes the call-time selection problem concrete: *"Use the system prompt to
describe when (and when not) to use each function,"* and *"aim for fewer than 20
functions… at any one time"* [OpenAI, Function calling]. Distribution in 2026 means:
reachable via MCP/skills, and *selected* by the agent over the alternative (which, for
ghx, is "the agent greps/reads files itself" — `[02]` persona #10).

### (h) What an "agent PM" optimizes for that a classic PM does not

**[SYNTHESIS]** A classic PM maximizes value *delivered to* and *captured from* a human
who spends attention and money. An agent PM adds three optimization targets the classic
canon has no vocabulary for:

1. **Context subtraction, not engagement.** Success is often the customer using the
   product *less per task* (fewer tokens, fewer round-trips) — the opposite of the
   engagement-maximizing instinct. ghx's whole rationale is *"what it removes from the
   customer agent's context."*
2. **Signals per token as the headline metric,** instrumentable at episode level in a
   way most software categories can only dream of (task success, judge score, and
   token/latency cost are all directly measurable — `[01 §7]`).
3. **Verifiability of a machine's output to a downstream actor** (the next agent, or the
   human who will trust the agent's action) — "evidence, not vibes." No human-facing
   product treats "can a *different* system audit this output" as a top-tier design
   goal; an agent-sidecar must.

The AI-native builder role that does this is itself new (swyx's "AI Engineer": someone
who *"builds with AI… focusing on rapid iteration, evaluation, and production
integration"* [swyx, Rise of the AI Engineer]) — the agent PM is that role's product
counterpart.

---

## 3. Audit of the Classic Advice (the centerpiece)

Columns: **Claim/framework** (with the artifact it came from + a short quote) |
**Verdict** for an agent-consumer product | **Why, in the agent-first world** |
**Agent-native reformulation** (when ADAPT/INVERTED) | **Source**.

Verdict key: **HOLDS** = transfers essentially unchanged; **ADAPT** = principle holds,
instrument/evidence must change; **OUTDATED-OR-INVERTED** = the advice as literally
practiced is wrong, absent, or points the wrong way for an agent consumer.

| # | Claim / framework (artifact + quote) | Verdict | Why, in the agent-first world | Agent-native reformulation | Source |
|---|---|---|---|---|---|
| 1 | **Nielsen's 10 usability heuristics** `[03 §1.1]` — "general principles evaluators apply with judgment" (visibility of status, match real world, recognition-not-recall, aesthetic/minimalist, error recovery, help/docs…) | **ADAPT** (with 2 sub-inversions) | The heuristics assume a *perceiving, forgetting human at a GUI*. Swap the screen for a context window and most survive with a new currency; two flip mechanism. `[03 §8]` already saw #1/#2/#9; this sharpens the rest. | **#1 Visibility of status** → status must live in stdout/exit-code/structured field (no spinner to see). **#2 Match real world** → use conventions the model saw in training (absolute paths, standard exit codes) — Anthropic's poka-yoke SWE-bench fix. **#5 Error prevention** → "make invalid states unrepresentable" via enum schemas (OpenAI). **#8 Aesthetic/minimalist** → *token* minimalism = "smallest set of high-signal tokens." **#9 Error recovery** → becomes the *primary* control surface (errors as affordances). **#6 Recognition-not-recall INVERTS** → a human recognizes options in a menu; an agent has no menu — it must "recall" the tool from its description in context, so the lever is the tool schema/progressive disclosure, not a visible affordance. **#10 Help/docs INVERTS toward zero** → the north-star ideal is *no* doctrine in the main agent. | Anthropic Building effective agents; Anthropic Context engineering; OpenAI Function calling; `[03]` |
| 2 | **Human onboarding / first-run experience** `[03 §6.3]` — "5-second test… cognitive walkthrough on the single most important 'zero to first value' task" | **OUTDATED-OR-INVERTED** (for the main-agent consumer) | The main agent has no eyes for a 5-second test and no "first run" to polish; the design goal is to *eliminate* onboarding, not perfect it. The human operator still has a first run (secondary). | Replace first-run polish with **progressive disclosure**: name+description at discovery, full `SKILL.md` on activation, references on execution — and drive toward **zero skill loaded in the main agent** (`NORTH_STAR.md`). Audit "how much doctrine must load before first success," minimize it. | Anthropic Agent Skills; `NORTH_STAR.md`; `[03]` |
| 3 | **HEART — Happiness / Engagement / Adoption / Retention / Task success** `[03 §4.1]` — "translating fuzzy UX goals into large-scale metrics" | **MIXED: Task-Success HOLDS; Happiness OUTDATED; Engagement/Retention INVERTED** | Task Success is already behavioral → transfers. Happiness has no agent referent. **Engagement/Retention invert**: for a product built to *leave the customer's context*, more tool calls / more time-in-tool per task is a *vanity metric pointing the wrong way*. `[03 §8]` substitutes task success + evidence quality but doesn't flag the inversion. **[SYNTHESIS on the inversion]** | Keep **Task Success** verbatim. Replace **Happiness** with **evidence quality / trust / calibration**. Replace **Engagement** with its inverse — **efficiency (tokens & tool calls *down* per task)**; treat rising per-task engagement as a regression. Move **Adoption/Retention** to the *fleet/operator* level (does the agent/operator keep *routing through* the tool across sessions — a Balfour-style revealed-preference curve `[03 §5.3]`), never per-session dwell. | Anthropic Context engineering; `NORTH_STAR.md`; `[03]` |
| 4 | **Sean-Ellis 40% "very disappointed" PMF survey** `[03 §5.1][04 §1]` — "'How would you feel if you could no longer use [Product]?' … ≥40% is the PMF benchmark" | **OUTDATED-OR-INVERTED** (as literally practiced) — but the underlying question gets a *stronger* answer | You cannot ask an LLM "how would you feel"; it has no felt disappointment `[03 §8]`. The survey instrument is a category error for the agent. But the *counterfactual behind it* ("who'd be hurt if it vanished") is answerable **better** for agents — by ablation. | Replace the survey with **ablation evals**: run the same tasks *with* vs *without* the tool; the drop in task success / rise in tokens **is** the "disappointment," measured behaviorally. This is exactly ghx's combined-objective host-task eval (`NORTH_STAR.md` C8) and its measured "24× signal/token" gate. Keep the human survey only for the *operator*. | τ²-bench; SWE-bench; `NORTH_STAR.md`; `[03][04]` |
| 5 | **"Delight" / Kano delighter** `[01 §4]` — "Attractive/Delighter (unexpected, disproportionate satisfaction)"; Craft Purist "Is it delightful?" `[02 #12]` | **OUTDATED-OR-INVERTED** | "Delight" is emotional surplus for a human; an agent has none. Optimizing it produces **delight theater** — pretty output the agent doesn't read. | The nearest honest analog to a "delighter" is **anticipation + sub-second latency** (ghx M8) and **zero-doctrine** — but framed as measured token/latency/success gains, never affect. Kano's *must-be* vs *indifferent* axis still helps if grounded in observed episode behavior `[01 §7]`, not intuition. | ghx M8 `NORTH_STAR.md`; `[01][02]` |
| 6 | **Journey mapping** `[04 §6]` — "uncover gaps in the user experience… at channel/tool handoffs where friction concentrates" | **ADAPT** (holds strongly; instrument upgrades) | The agent's **trajectory (tool-call transcript) *is* the journey map**, and it is fully logged rather than inferred `[03 §8]`. Handoff-friction maps to real boundaries: main-agent↔sidecar (report contract) and sidecar↔CLI/backends. | Replace hand-drawn journey maps with **trace/trajectory mining**: instrument every tool call, retry, and backtrack; the "steepest friction handoff" becomes the most wasteful step in the episode (tokens/calls). Diagnose each drop-off as UX-problem (tool affordance) vs value-problem (wrong recon) — `[04 §6]`'s split transfers. | Hamel evals; Anthropic Writing effective tools; `[03][04]` |
| 7 | **Positioning & messaging** `[04 §4][02 #10]` — Dunford: "competitive alternatives… what would the customer do if you didn't exist"; "positioning determines messaging" | **ADAPT** (bifurcates by consumer) | Two audiences now: the **human operator** (classic positioning holds) and the **agent selecting the tool at call-time**. The agent's "positioning" is machine-readable metadata, and it is *testable*. | For the agent, "messaging" becomes the **tool name + description + when-to-use metadata**; "does an informed prospect find it obviously right" becomes **"does the agent reliably select this tool over the alternative,"** measured as tool-selection accuracy in evals. Dunford's "true alternative" for ghx = "the agent greps/reads files itself" `[02 #10]`. OpenAI: *"describe when (and when not) to use each function."* | OpenAI Function calling; Anthropic Agent Skills (discovery layer); Biilmann AX; `[04][02]` |
| 8 | **Jobs-to-be-Done** `[01 §4][03 §7.2][04 §6]` — "customers 'hire' a product to do a job"; switch interviews coding push/pull/anxiety/habit | **HOLDS** (lens) / **ADAPT** (method) | JTBD is consumer-agnostic and already applied well: the job ghx is hired for is *"build enough confidence in an unfamiliar codebase to make a safe change fast"* `[01 §4]`. The *hirer* is the main agent; it "fires" the tool by routing around it. Switch *interviews* (reconstructing a human's decision) don't transfer — you can't interview the agent. | Keep JTBD as the framing. Replace **switch interviews** with **trace mining + ablation**: observe *when* the agent chose the tool vs bypassed it, and what job it was doing at that moment. Do JTBD for **both** consumers — the agent's job (low-token orientation) and the operator's job (trust the agent won't break things) `[01 §7]`. | `[01][03][04]`; Anthropic Writing effective tools (search vs list) |
| 9 | **Outcomes over output / build trap** `[01 §5][02 #1][04 §5]` — Perri: "ship features rather than cultivate the value"; "'we added five new subcommands' is a non-answer" `[02 #1]` | **HOLDS** (and intensifies) | The single most durable classic idea. Agent products hide behind "we added N tools" *more* easily, and adding tools can actively *hurt* (context bloat, selection errors). Instrumentation is unusually good here. | Unchanged in spirit; the NSM becomes **signals-per-token / task-success-per-token**, directly instrumentable. Guardrail: it *can* be Goodharted via judge tuning, so pair with calibration `[04 §5]`. "More tools" is the agent-era feature factory — OpenAI's "<20 functions" is the anti-bloat teeth. | `[01][02][04]`; OpenAI Function calling; Anthropic Context engineering |
| 10 | **Moat / 7-Powers** `[04 §9]` — Helmer: a Power needs "both a Benefit and a Barrier"; "tools vs brain" | **HOLDS** (framework) / **which powers are available INVERTS** | The Benefit+Barrier test is consumer-agnostic and `[04 §9]` already applied it to "brain not tools." But *which* moats an agent-native product can hold reshuffles. **[SYNTHESIS on the reshuffle]** | **Switching cost weakens** (an agent can swap tools per-call with no retrained muscle-memory; human UI-familiarity lock-in largely evaporates). **Branding-as-habit weakens** (the agent doesn't feel brand affinity). **New/strengthened powers**: being the **default in MCP/skill ecosystems** (Thompson aggregation / demand-side default), a **cornered resource** in proprietary trajectory/telemetry data (ghx's own thesis), and **process power** in the recon brain. The "tools vs brain" claim `[04 §9]` is exactly right and *more* urgent, because the tool layer may commoditize (Wardley). | Helmer/Thompson via `[04]`; Anthropic Context engineering; `NORTH_STAR.md` |
| 11 | **Adversarial / red-team stack** `[05]` — pre-mortem, inversion, WWHTBT, kill-the-company, devil's advocate, bias checks | **HOLDS** (and gains a literal new meaning) | These are *builder-side* disciplines about the team's reasoning, not the consumer — they transfer intact and `[05]`'s applied notes are already agent-aware. But "red-teaming" also acquires a concrete agent-native form. | Keep all of `[05]` as-is for strategy/eval-methodology critique. **Add adversarial *evals*** as a product-surface scope: prompt-injection and the **lethal trifecta** (Willison), tool-misuse, adversarial repos `[05 §2]`, and the inversion "what would make an agent *distrust or ignore* this tool's output?" `[05 §4]` — which is directly the affordance/error/trust audit. | `[05]`; Willison lethal trifecta / agentic loops |
| 12 | **Discovery — Cagan's Four Big Risks + Torres assumption testing** `[03 §3][01 §5][04 §8]` — value/usability/feasibility/viability; "retire risks before build" | **HOLDS** (usability-risk redefined; a 5th risk emerges) | `[03 §8]`: the risks don't change, only the *evidence* to retire them. Correct — but "usability risk" is redefined and a trust risk deserves naming. **[SYNTHESIS on the 5th risk]** | **Usability risk** = "can the agent reliably invoke it, recover from errors, and not be misled" — evidenced by trace/ablation, not human tests. Add a **5th risk: trust/verifiability** — will the downstream human/agent be able to *audit* the output ("evidence, not vibes"); a confidently-wrong report is severity-4 `[03 §8]`. Torres assumption tests → **eval hypotheses** (WWHTBT `[05 §6]`: "for this to be worth a tool-call it must be true that…"). | `[01][03][04][05]`; `NORTH_STAR.md` |
| 13 | **Information scent / Diátaxis docs audit** `[03 §6.1–6.2]` — flag/command names as "link labels an agent estimates the value of before spending a tool call"; four doc types | **HOLDS/ADAPT** (scent) / **ADAPT** (docs collapse toward reference) | `[03 §8]` already transfers both well. Information scent *is* Anthropic's tool-naming guidance (`search_contacts` beats `list_all_contacts`; namespacing). Diátaxis largely dissolves for the agent: it wants *reference* (the schema), not tutorial/explanation. | Scent audit = **tool/flag naming + namespacing** audit (poka-yoke names; no two flags that sound identical). Docs audit for the agent = "is the tool description self-sufficient; does anything beyond reference load into the main agent?" — ideally nothing. | Anthropic Writing effective tools; `[03]` |
| 14 | **"Test with 5 users / 3 rounds"; DIY usability (Krug)** `[03 §1.6]` — "testing beyond ~5 users… yields sharply diminishing returns" | **OUTDATED-OR-INVERTED** (sample economics) — expert-inspection HOLDS | The 5-user rule is an artifact of *human recruiting cost*. When the "user" is an LLM, episodes are cheap and you *want* large N to measure **consistency** (τ²-bench's pass^k). The heuristic-evaluation-*by-experts* method still holds (experts inspecting tool affordances). | Run **many episodes**, report variance/pass^k, not "5 users." Keep expert heuristic review of the tool surface. `[03]`'s own severity-rating and debrief mechanics transfer to eval anomaly review. | τ²-bench; Hamel evals; `[03]` |
| 15 | **PM competency / product-sense / high-agency / personas** `[01 §1,2,6][02]` — Mehta's 12 competencies; Doshi "product sense"; Horowitz ownership; the 15 audit personas | **HOLDS** (these describe the *builder*, not the consumer) | Almost none of this re-prices, because it is about the person/team making decisions, not the entity consuming the product. An agent PM still needs judgment, agency, outcome-ownership, and multi-lens critique. **Fairness note: do not discard PM wholesale.** | Keep intact. Two personas (`[02 #14]` Developer-Experience Advocate, `[02 #15]` AI PM Pragmatist) already lean agent-native and `[02 §5.3]` flags them "near-mandatory" for ghx — §4 below promotes AX to a *first-class* persona rather than a bolt-on. | `[01][02]`; Biilmann AX |
| 16 | **Market/business scopes — PMF-as-market, Porter Five Forces, TAM/Wardley/timing, COSS/open-core, monetization** `[04 §1–3,8]` | **HOLDS** (they concern the *human* market/business) — with one agent-first sharpening | The buyer/operator/economics side is still human, so these lenses apply to that layer. What changes is the **added agent-consumer layer beneath them**. | Wardley's evolution axis gains a sharp agent-first use: **is the tool layer commoditizing as base models get better at raw exploration?** — exactly ghx's P4 bet that recon may need to move to a trained model. Monetization (`[02 #3]`) reframes as **cost-to-serve per episode** vs value delivered while internal. | `[04]`; `[02]`; `NORTH_STAR.md` |

**Fairness summary.** Roughly: *HOLDS* — outcomes-over-output, JTBD (lens), 7-Powers
(framework), the full red-team stack, four-big-risks discovery, PM-craft/personas,
market/business scopes. *ADAPT* — Nielsen heuristics, journey mapping, positioning,
information-scent/docs. *OUTDATED-OR-INVERTED* — human onboarding/first-run, HEART's
Happiness + (sharply) Engagement/Retention-as-goal, the Sean-Ellis survey, "delight,"
the 5-user sampling rule. The prior artifacts' own agent-adaptation (`[03 §8]`) is the
closest existing work and is largely right; this audit's additions are (i) naming the
**engagement/retention inversion**, (ii) the **survey→ablation** upgrade, (iii) the
**which-7-Powers-invert** reshuffle, (iv) the **5th (trust) discovery risk**, and (v)
promoting **AX to a first-class lens**.

---

## 4. New Agent-Native Scopes, Methods, and Personas the Skill Must Add

Each with what it inspects and its source grounding. These are *additions*; they sit
alongside the classic scopes in `[03][04]`, not replacing them.

1. **Agent Experience (AX) persona / lens — first-class, not a bolt-on.**
   Role-play "the agent is the user." Inspects the four AX pillars: **Access** (can the
   agent reach the product — MCP/skill/CLI reachability), **Context** (can it understand
   the product from what loads into its window), **Tools** (can it *do* the job), and
   **Orchestration** (does the output *compose* into the agent's next action).
   Signature question: *"If a fresh agent with zero prior context had only this tool's
   name, description, and one error message, could it complete the job and know it
   succeeded?"* Grounding: Biilmann, Introducing AX; Anthropic Building effective agents
   (ACI). *This subsumes and outranks `[02 #14]` DevEx-Advocate for agent products.*

2. **Token economics / signals-per-token scope.** Inspects the **context tax**: how
   many tokens the product injects into the *main* agent's window per unit of returned
   signal — tool-definition bloat, verbose reports, redundant fields. Metrics: tokens-in
   vs signal-out, compression ratio (ghx measures ~16–25×), tool-call count per task.
   Grounding: Anthropic Effective context engineering ("finite resource," "smallest set
   of high-signal tokens"); Anthropic Code execution with MCP ("150,000 → 2,000 tokens").

3. **Evals-as-user-research method.** Replaces surveys, interviews, and usability labs.
   Inspects: does an **eval suite exist and function as the product's user research**
   (task success, pass^k consistency, ablation with/without the tool), and is it
   trustworthy (calibrated judge, guardrails, negative results committed)? The public
   benchmarks are the reference form. Grounding: Hamel, Your AI product needs evals
   ("failure to create robust evaluation systems"); SWE-bench; τ²-bench.

4. **Tool / affordance & error-as-affordance audit.** Inspects tool names, descriptions,
   namespacing, parameter schemas (enums / "invalid states unrepresentable"), poka-yoke
   constraints, high-signal returns, tool count (<20 active), and — critically —
   **does every error name the correct next invocation** rather than dumping a
   traceback. Grounding: Anthropic Writing effective tools; Anthropic Building effective
   agents; OpenAI Function calling. (This is also ghx workstream A4: "every CLI error
   tells the agent the correct next invocation.")

5. **Context-budget / progressive-disclosure audit.** Inspects how much doctrine must
   load into the **main** agent before first success, whether knowledge is progressively
   disclosed (discovery → activation → execution), and how close the product gets to the
   north-star ideal of **zero skill loaded in the main agent**. Grounding: Anthropic
   Agent Skills; Anthropic Code execution with MCP; `NORTH_STAR.md`.

6. **Trust & verifiability of agent output scope.** Inspects whether the tool's output
   lets a *downstream* actor (the next agent, or the supervising human) **verify** the
   claim: citations, commands, snippets, backends, explicit uncertainty, and full
   traces — and whether a confidently-wrong output is structurally distinguishable from
   a right one. Grounding: ghx "evidence, not vibes" + visibility/truthfulness tenet
   (`NORTH_STAR.md`); `[03 §8]` "confidently wrong = severity-4"; `[05 §4]` inversion.

7. **Agent-safety / adversarial-tool-use scope.** Inspects the product's exposure when
   its output feeds an autonomous loop: prompt-injection surface, the **lethal trifecta**
   (private data + untrusted content + exfiltration path), reversibility/dry-run, and
   whether the output can be weaponized into a harmful next action. Grounding: Willison,
   Designing agentic loops / lethal trifecta; `[05]` red-team.

8. **Distribution-in-agent-ecosystems scope.** Inspects reachability and *default-ness*
   in MCP/skill ecosystems and whether the agent **selects** this tool over the
   alternative at call-time (tool-selection accuracy). Grounding: MCP spec; Anthropic
   Agent Skills; Thompson aggregation via `[04 §9]`.

---

## 5. What the Skill Should Explicitly WARN Against

Cargo-culting human-centric heuristics onto an agent product produces confident,
expensive waste. The audit skill should carry these as named anti-patterns.

- **Human-pretty output the agent doesn't read.** ASCII tables, color, emoji, banner
  art, decorative formatting in output the agent *parses* — pure context tax against
  "smallest set of high-signal tokens." Mis-transposing Nielsen's "aesthetic" heuristic
  from *visual* clutter to *token* clutter is the tell. (Anthropic Context engineering.)
- **"Delight" theater.** Optimizing emotional surplus for an entity with no affect;
  polishing a "wow moment" the agent cannot feel. If a change's only defense is that it
  *feels* nicer, it is theater. (§3 row 5.)
- **Vanity human funnels as success.** GitHub stars, installs, downloads, DAU, and
  especially **session length / engagement / tool-calls-per-task** as headline metrics.
  For a context-subtracting product, per-task engagement is a metric to *drive down*;
  citing it as growth is the sharpest agent-era vanity trap. NSM must be
  **signals-per-token / task success**, not usage volume. (§3 row 3; `[04 §1]` vanity
  substitution.)
- **First-run polish / a "great onboarding" for the main agent.** Building a welcoming
  human first-run for a consumer that reads a schema. The target is *zero* onboarding;
  effort here is effort against the north star. (§3 row 2.)
- **Surveying the agent.** Any "how would the agent feel / rate this" instrument
  (Sean-Ellis-style) is a category error. Use ablation and task-success deltas. (§3 row
  4.)
- **Tool-count / feature-count as progress.** "We added N subcommands/tools" is the
  agent-era feature factory; more tools can *reduce* task success via context bloat and
  selection errors (OpenAI: keep <20 active). Adding a tool must retire a risk or move
  signals-per-token, or it is build-trap output. (§3 row 9.)
- **Conflating the human operator's needs with the agent consumer's.** Do JTBD for
  both, but never let a human-facing dashboard or a founder's aesthetic preference
  masquerade as *agent* value. Over-indexing on the vocal human (`[02 #2]` User-Empath
  blind spot) means reading raw agent traces, not just judge aggregates.
- **Assuming today's model capability is permanent.** The tool layer may commoditize as
  base models get better at raw exploration (Wardley; `[02 #15]` AI PM Pragmatist). A
  moat resting on the CLI rather than the brain/data is a current advantage, not a
  Power. (§3 rows 10, 16.)
- **Trusting evals you can't audit.** The agent-native replacement for user research is
  only as good as its calibration; an uncalibrated LLM-judge is the new "leading the
  witness." Commit negative results; name who scored what from which artifacts. (Hamel;
  `NORTH_STAR.md` visibility/truthfulness tenet.)

---

## 6. Source Ledger

| # | Source | Author / Org | Specific URL | Why authoritative | What it supports here |
|---|---|---|---|---|---|
| 1 | Building Effective AI Agents | Erik Schluntz & Barry Zhang, Anthropic (Dec 2024) | https://www.anthropic.com/engineering/building-effective-agents | The most-cited practitioner statement of agent design from the lab that ships Claude; origin of the "ACI" framing and poka-yoke tool design | §2(b) ACI = invest in agent-computer interface like HCI; §3 rows 1, 8, 12; §4.4 |
| 2 | Effective Context Engineering for AI Agents | Anthropic Applied AI | https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents | Primary source for "context as finite resource / attention budget / smallest high-signal tokens" — the external grounding of ghx's "signals per token" | §2(a); §3 rows 1, 3, 9, 10; §4.2; §5 |
| 3 | Code Execution with MCP: Building More Efficient Agents | Anthropic Engineering | https://www.anthropic.com/engineering/code-execution-with-mcp | Primary, quantified source on the token cost of loading tool definitions and progressive disclosure ("150,000 → 2,000 tokens") | §2(a) context tax; §4.2, §4.5 |
| 4 | Writing Effective Tools for AI Agents — Using AI Agents | Anthropic Engineering | https://www.anthropic.com/engineering/writing-tools-for-agents | The canonical tool-design guide: tools as a "contract between deterministic systems and non-deterministic agents," meaningful errors, high-signal returns, namespacing, eval-driven | §2(b),(c); §3 rows 1, 6, 8, 13; §4.4 |
| 5 | Equipping Agents for the Real World with Agent Skills | Anthropic Engineering | https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills | Primary source for progressive disclosure (discovery → activation → execution; SKILL.md) — the mechanism replacing "onboarding" | §2(d); §3 rows 2, 7; §4.5 |
| 6 | Model Context Protocol — Specification (Architecture) | Anthropic / MCP maintainers (spec 2025-06-18) | https://modelcontextprotocol.io/specification/2025-06-18/architecture | The open standard for exposing tools/resources/prompts to agents; grounds "distribution = MCP/skill ecosystems" | §2(g); §4.8 |
| 7 | Function Calling (API guide) | OpenAI | https://developers.openai.com/api/docs/guides/function-calling | Primary vendor guidance: clear names/descriptions, enums to make invalid states unrepresentable, "<20 functions," describe when/when-not to use | §2(g); §3 rows 1, 5, 7, 9, 14; §4.4 |
| 8 | Introducing AX: Why Agent Experience Matters | Mathias Biilmann (CEO, Netlify), Jan 28 2025 | https://biilmann.blog/articles/introducing-ax/ | The essay that named "Agent Experience" as a discipline and the agent as a primary user class; four pillars Access/Context/Tools/Orchestration | §2(b),(h); §4.1; §3 row 15 |
| 9 | Designing Agentic Loops (+ the lethal trifecta) | Simon Willison, Sep 30 2025 | https://simonwillison.net/2025/Sep/30/designing-agentic-loops/ | The most-read independent voice on LLM/tool ergonomics; "agent = tools in a loop," error-recovery, YOLO/safety, lethal trifecta | §2(c); §3 row 11; §4.7 |
| 10 | Your AI Product Needs Evals | Hamel Husain (ex-GitHub/CodeSearchNet) | https://hamel.dev/blog/posts/evals/ | The standard reference arguing evals are the foundation of AI products; the eval flywheel and "look at your data" | §2(f); §3 rows 6, 9; §4.3; §5 |
| 11 | SWE-bench: Can Language Models Resolve Real-World GitHub Issues? | Jimenez, Yang, Wettig, Yao, Pei, Press, Narasimhan (ICLR 2024) | https://arxiv.org/abs/2310.06770 | The canonical real-task agent benchmark; the "user research" analog for code-agent products (directly relevant to ghx's domain) | §2(f); §3 rows 4, 8; §4.3 |
| 12 | τ²-Bench: Evaluating Conversational Agents in a Dual-Control Environment | Barres, Dong, Ray, Si, Narasimhan (Sierra, 2025) | https://arxiv.org/abs/2506.07982 | Benchmark for tool-agent-user interaction & policy compliance measured by pass^k consistency; grounds "evals as user research" and the large-N/consistency point | §2(f); §3 rows 4, 14; §4.3 |
| 13 | The Rise of the AI Engineer | Shawn "swyx" Wang, Latent Space (Jun 2023) | https://www.latent.space/p/ai-engineer | Named the AI-native builder role (iteration + evaluation + production integration); the agent PM's counterpart | §2(h) |

**Distinct external sources cited: 13** (5 Anthropic engineering posts, the MCP spec,
OpenAI's function-calling guide, Biilmann's AX essay, Willison, Hamel, SWE-bench,
τ²-bench, swyx). All 13 deep URLs were verified live during this pass via WebFetch or
WebSearch result confirmation. The five prior artifacts (`[01]`–`[05]`) are cited as the
*audit target*, not as external sources.

**Verification notes.** Every Anthropic URL, the MCP architecture page, the AX essay,
Willison's post, and Hamel's post were fetched directly and quoted verbatim. The OpenAI
guide 301-redirects `platform.openai.com → developers.openai.com`; the redirected page
was fetched and quoted. SWE-bench (2310.06770), τ²-bench (2506.07982), and swyx's essay
were confirmed via search-result metadata (title, authors, arXiv ID) rather than
full-text fetch; the arXiv IDs and author lists are stated as returned and are
consistent across multiple independent results.
