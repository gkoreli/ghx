# Product Manager Personalities, Archetypes, and Audit Lenses

## Abstract

Product management has never formally codified its specializations the way design, engineering, marketing, and sales have — Reforge makes this observation explicitly, noting that "Product Management has not yet defined specialties" even as the industry informally spins up Growth PMs, Platform PMs, and 0→1 PMs without a shared vocabulary for what each optimizes for or where each has blind spots (Fishman, "The Growing Specialization of Product Management," Reforge, [reforge.com/blog/product-specializations-pt2](https://www.reforge.com/blog/product-specializations-pt2)). This creates a practical opportunity: because the real-world taxonomy of PM "types" already encodes a set of distinct, well-reasoned worldviews — what each type notices, what each optimizes for, and what each is structurally blind to — that taxonomy can be inverted into a set of adversarial **audit lenses**. An auditor who role-plays a Growth PM, then a Platform PM, then a Technical-Debt Realist against the same product will surface materially different issues than an auditor who audits once from a single, undifferentiated "product sense" perspective.

This document does two things. First (Sections 1–3), it surveys the real, sourced taxonomy of PM functional archetypes, personality/working-style archetypes, and organizational archetypes, drawing on Reforge, Ravi Mehta, Marty Cagan/SVPG, Shreyas Doshi, Sachin Rekhi, Adam Fishman, and Lenny Rachitsky's interview series — the primary frameworks practitioners actually use to talk about PM specialization. Second (Section 4), it converts that taxonomy — plus classic adversarial critique roles (the skeptic, the user-empath, the monetization hawk, and others) — into a **Persona Roster for Audits**: fifteen self-contained, role-playable lenses, each grounded in a named source, each with signature questions, reflexive flags, and — critically — its own blind spots, so the roster can be composed deliberately for coverage rather than redundancy. Section 5 gives guidance on composing a persona set for a given audit, including notes on applying these lenses to developer-tool and AI-agent-infrastructure products specifically (the class of product `ghx` belongs to). A Source Ledger closes the document.

Sections and claims marked **SYNTHESIS** are this document's own reasoning connecting sourced material — not a direct claim by the cited authority. Everything else is drawn from, and cited to, a named practitioner or publication with a specific URL, verified live during research (a small number of book citations note where the specific-chapter deep link could not be independently confirmed).

---

## 1. Functional Archetypes: What Each Type of PM Optimizes For

### 1.1 Reforge's canonical specialization taxonomy (Core / Growth / Platform / Innovation)

The most widely cited *structural* taxonomy comes from Reforge, in an article co-written by Adam Fishman (then EIR at Reforge, former CPO at Imperfect Foods, VP Product at Patreon, Head of Growth at Lyft) with contributions from Ravi Mehta (Reforge Partner/EIR, former CPO at Tinder), Crystal Widjaja (CPO at Kumu, former SVP Growth at Gojek), Fareed Mosavat (ex-Director of Product, Slack), and Casey Winters (CPO at Eventbrite, ex-Product Growth Lead at Pinterest). It grounds the specialization taxonomy in **four types of product work**, then maps each to a specialization:

| Type of product work | Specialization | What it optimizes for | Typical sub-titles |
|---|---|---|---|
| **Feature work** — extending functionality into incremental/adjacent areas once initial PMF exists | **Core PM** | Solving a specific customer pain point with holistic, end-to-end experience quality | Feature PM, UX PM, core consumer PM, enterprise PM, community PM, international PM |
| **Growth work** — connecting more people to value that already exists, rather than creating new value | **Growth PM** | Acquisition, activation, conversion, retention, monetization metrics; speed of experimentation | Activation PM, Engagement PM, Retention PM, Monetization PM, Conversion PM |
| **Scaling work** — removing bottlenecks that block the team's ability to ship as it grows | **Platform PM** | Internal stakeholders/teams as customers; scaled, reliable internal systems | Tech PM, Data PM, Infra PM, Security PM, Identity PM, Internal Tools PM |
| **PMF expansion** — increasing the ceiling on product-market fit non-incrementally, into adjacent markets/products | **Innovation PM** | Finding new product-market fit; comfort with ambiguity and pivoting | Expansion PM, 0→1 PM, New Verticals PM, Skunkworks PM, R&D PM |

Reforge is explicit that treating "a PM is a PM" as true causes five concrete organizational failure modes it names: tools that don't transfer across specializations, PMs who "slow-and-steady struggle" in a mismatched specialization, "focus fatigue" from unclear expectations, mis-hires ("talent turmoil"), and unfair comparisons between PMs with different specialties ("competing colleagues") (Fishman, [reforge.com/blog/product-specializations-pt2](https://www.reforge.com/blog/product-specializations-pt2)). A companion piece, co-written by Ravi Mehta and Adam Greiner (VP Marketing at MasterClass, former VP Product/Growth at Lambda School), extends this into career and team-composition guidance: as companies scale from pre-seed "generalist PM teams" (multi-sport athletes) through Series B/C "varied PM teams" (single-sport athletes, specialized but flexible) to Series D+ "specialized PM teams" (dedicated Directors per specialization), the *right number and mix* of specializations to staff for changes with company stage (Mehta & Greiner, "How To Navigate Product Management Specializations," [reforge.com/blog/product-specializations](https://www.reforge.com/blog/product-specializations)).

### 1.2 Peter Deng's five archetypes (Consumer / Growth / Business / Platform / Research-AI)

A convergent, independently-derived taxonomy comes from Peter Deng — who has held product leadership roles at OpenAI (VP Consumer Product), Instagram (first Head of Product), Uber (Head of Rider), Facebook (fourth PM), Airtable (CPO), and Oculus, an unusually broad cross-section of consumer, growth, platform, and AI product work. In a 2026 interview with Lenny Rachitsky, Deng lays out five PM archetypes and — notably — argues for deliberately staffing *across* them because "debates between them produce better products than consensus would," comparing team composition to "a role-playing-game party where everyone has different stats and abilities" (Deng, interviewed by Rachitsky, "From ChatGPT to Instagram to Uber," Lenny's Newsletter, [lennysnewsletter.com/p/the-quiet-architect-peter-deng](https://www.lennysnewsletter.com/p/the-quiet-architect-peter-deng)):

- **Consumer PM** — "half designer, half product person," obsessed with the details ("Is it delightful? Is it crafted well enough?").
- **Growth PM** — "half data scientist, half product person," data-driven, reflexively skeptical ("show me the data").
- **Business PM** — "half MBA, half product person," thinks in margins, opportunity cost, and value creation.
- **Platform PM** — "deeply wired to build tools for other people," builds the systems that let other teams move faster.
- **Research/AI PM** — "half researcher, half engineer, half product person," combines product taste with deep understanding of how the underlying models are trained.

The convergence between Deng's independently-stated taxonomy and Reforge's Core/Growth/Platform/Innovation split (Consumer≈Core, Growth≈Growth, Platform≈Platform, Research-AI≈a newer variant of Innovation specific to ML-era products, Business as an axis both frameworks otherwise treat as a cross-cutting skill) is itself evidence that this is a real, converged taxonomy rather than one author's invention — a point reinforced independently by First Round Review-adjacent commentary describing a four-archetype Feature(Business)/Growth/Technical/Data-ML split with similar boundaries (Pilawski, "Product Manager Archetypes," summarizing First Round Review-style hiring frameworks — cited here by title/author; the specific originating First Round essay could not be independently confirmed at a deep URL and should be treated as a secondary synthesis, not a primary Reforge/Deng-grade source).

### 1.3 Technical PM, Data/Analytics PM, and Developer-Experience PM

**Technical PM.** Distinguished less by a single canonical essay than by convergent practitioner consensus: a Technical PM combines product management with a strong engineering background, working closer to system architecture, APIs, data flows, and engineering constraints than to sales/marketing, and is disproportionately staffed by former software engineers (multiple practitioner sources, synthesized; no single primary essay dominates this category — **SYNTHESIS** of convergent secondary sourcing).

**Data/Analytics PM.** A data PM's defining trait, per practitioner consensus, is that "to a data PM, data is the product" — the role centers on collecting, organizing, and productizing data flows themselves (as opposed to merely being "data-driven" while managing a non-data product) (synthesized from multiple practitioner glossary sources — **SYNTHESIS**, no single named primary authority dominates this category the way Reforge/Deng dominate the Core/Growth/Platform split).

**Developer-Experience (DevEx) PM.** This is the newest, most rigorously instrumented functional archetype, led by Abi Noda — CEO of DX (formerly founder/CEO of Pull Panda, acquired by GitHub) — who, with Laura Tacho and researchers including Nicole Forsgren (a DORA co-creator), unified DORA, SPACE, and DevEx into the **DX Core 4** framework: four measurable dimensions of developer productivity — **Speed** (diffs/PRs per engineer), **Effectiveness** (a 14-item Developer Experience Index survey), **Quality** (change failure rate), and **Impact** (percentage of time on new capabilities vs. maintenance) (Noda, "Introducing the DX Core 4," [newsletter.getdx.com/p/introducing-the-dx-core-4](https://newsletter.getdx.com/p/introducing-the-dx-core-4)). A DevEx PM optimizes for exactly these — reduced friction and measured productivity for the *builders using the tool*, not for an end-consumer.

### 1.4 AI/ML PM

The AI PM archetype is unusually contested precisely because it is new. Marty Cagan (founder of Silicon Valley Product Group, author of *INSPIRED* and *EMPOWERED*) argues the job is not a separate specialization so much as an intensification of the existing PM job: "virtually all product managers will need to be AI product managers... the PM role becomes more essential but also more difficult with generative AI-powered products, not less" (Cagan, "AI Product Management 2 Years In," SVPG, [svpg.com/ai-product-management-2-years-in/](https://www.svpg.com/ai-product-management-2-years-in/)). Cagan explicitly separates two different conversations — "incorporating this new enabling technology into the products we are building" versus "how this technology changes how we build our products" — and states he is most worried about "the prospect of providing those [AI] tools to people that do not have the necessary product foundation," i.e., AI amplifies existing product judgment, good or bad, rather than substituting for it.

Andreessen Horowitz General Partner Anish Acharya frames the same shift more tactically, in five principles: (1) "interview your models" the way you'd interview customers, because model outputs are stochastic rather than deterministic; (2) don't shy away from extreme products at extreme price points (the $1,000/month tier question); (3) moats in AI are often "first and fast" rather than durable; (4) foundation models are platforms, not products — they require "opinionated workflows" built around them; and (5) reflexive daily personal use of AI tools is tipping from differentiator to baseline expectation for PMs (Acharya, "5 Principles for Product Managers Fending Off Obsolescence in the AI Era," a16z, [a16z.com/stay-relevant-in-ai/](https://a16z.com/stay-relevant-in-ai/)).

---

## 2. Personality / Working-Style Archetypes

Distinct from the *functional* taxonomy above (which is largely about what part of the product a PM works on) is a *dispositional* taxonomy — what a given PM notices, values, and is naturally good at, independent of current role. Three convergent frameworks dominate here.

### 2.1 Ravi Mehta's four-quadrant "Shape" framework

Ravi Mehta (former CPO at Tinder, VP Product at Tripadvisor, Product Director at Facebook, Reforge Partner) organizes 12 PM competencies into four areas — **Product Execution**, **Customer Insight**, **Product Strategy**, and **Influencing People** — and argues that because "no PM excels at all areas," the differentiator between average and peak PMs is *knowing your gaps and building a team that fills them*: "individuals should be spiky; teams should be well-rounded" (Mehta, "What's Your Shape? A Product Manager's Guide to Growing Yourself and Your Team," [ravi-mehta.com/product-manager-roles/](https://www.ravi-mehta.com/product-manager-roles/)). From this quadrant structure he derives four archetypes, each a warning as much as a description:

- **The Project Manager** — excels at execution, but companies that over-hire this type "risk fostering teams that know how to build but can't figure out what to build."
- **The People Manager** — excels at influencing/aligning people, but may underinvest in product knowledge itself; "Product Leaders must build both people and products."
- **The Growth Hacker** — combines speed, data fluency, and testing rigor, but risks the trap of "mistak[ing] growth optimization with product innovation."
- **The Product Innovator** — excels at envisioning future products and reading customer needs, but struggles with iterative execution and data validation discipline.

### 2.2 Shreyas Doshi's Craftsperson / Operator / Visionary triad

Shreyas Doshi (former product leader at Stripe, Twitter, Google, and Yahoo) described three archetypes of product *leaders* specifically, on Christian Idiodi's Product Therapy podcast: the **Craftsperson** ("it's all about the product"), the **Operator** ("it's about scale"), and the **Visionary** ("it's about the future"). Marty Cagan, reacting to and extending this framework in his own SVPG article, adds a critical nuance most secondary summaries drop: he argues craft is *not* one co-equal archetype among three, but the non-negotiable floor beneath the other two — "I don't care how skilled a product leader is in terms of an operator or a visionary if she is not, first and foremost, very accomplished regarding the craft of product." Cagan further splits "Operator" into two structurally different sub-types he considers underappreciated: operators who scale through *coaching* (which he calls "absolutely gold") versus operators who scale through *process* — the latter he calls, quoting Steve Jobs, "process people," people who "are essentially trying to use process as a scalable substitute for thinking" (Cagan, "Product Leadership Archetypes," SVPG, [svpg.com/product-leadership-archetypes/](https://www.svpg.com/product-leadership-archetypes/)). Doshi himself is explicit that "most people have skills from multiple archetypes" but that everyone has a genuine preference, and that preference shapes behavior under pressure.

### 2.3 Sachin Rekhi's Builder / Tuner / Innovator / Enabler split

Predating both of the above (published 2016), Sachin Rekhi — a longtime Silicon Valley PM and entrepreneur (LinkedIn, Notejoy) whose essays are widely used in PM mentorship programs — defines four types by what a PM is optimizing to move:

- **Builders** — traditional PMs driving an existing product's roadmap; superpower is "truly understanding a target user segment... and knowing how to listen to users for insights on the problems they are solving for."
- **Tuners** — PMs with "unwavering focus on a specific north star metric," described as "analytical ninjas" who spend as much energy improving their testing infrastructure and velocity as running any single test.
- **Innovators** — PMs finding product-market fit for something brand new; "truth seekers that take a hypothesis-driven approach," comfortable with failure and uncertainty; Rekhi explicitly advises PMs gain Builder experience before attempting this role.
- **Enablers** — PMs scaling *other teams'* capabilities via internal infrastructure/tooling; "systems thinkers" who understand how internal systems ultimately roll up into end-user or company value.

Rekhi's own framing note matters for the audit-lens purpose of this document: "every product manager in-fact does activities embodied in all four roles, but with differing levels of focus" — the archetypes are lenses/emphases, not exclusive categories (Rekhi, "The 4 Types of Product Managers," [sachinrekhi.com/p/3-types-of-product-managers-builders-tuners-innovators](https://www.sachinrekhi.com/p/3-types-of-product-managers-builders-tuners-innovators)).

### 2.4 Adam Fishman's seven finer-grained archetypes

Independently of the Reforge specialization piece he co-authored, Fishman published a more granular,七-way personal taxonomy in his own newsletter, explicitly naming what each archetype is good at *and* where it struggles — the most blind-spot-explicit of any source found:

| Archetype | Optimizes for | Struggles with |
|---|---|---|
| Growth | Speed/agility (shipping) over craft | Zero-to-one work, long-term strategy |
| UX-Inclined | User-centered, holistic experience | Growth/scaling work |
| Internal | Organizational efficiency, build-vs-buy | PMF expansion, revenue focus |
| General Manager | Maximizing the whole opportunity (P&L) | Zero-to-one, internal tooling |
| Optimizer | Incremental quality improvement via iteration | Zero-to-one/visionary work |
| Technician | Systems/scale in technically complex domains | Core feature work, customer empathy |
| White-Spacer | PMF-seeking, vision, ambiguity | Scaling/growth work (gets bored) |

(Fishman, "Which Type of Product Manager Are You?", [fishmanafnewsletter.com/p/identify-product-manager-archetypes-and-skills](https://www.fishmanafnewsletter.com/p/identify-product-manager-archetypes-and-skills))

---

## 3. A Third Axis: Organizational/Structural Archetypes

Orthogonal to *what* a PM works on and *how they're wired* is a third axis Marty Cagan insists is underappreciated: **how much authority the PM structurally has**. Cagan defines three organizationally distinct types of product management — **delivery team product ownership** (mostly administrative Scrum-style backlog management), **feature team product management** (a designer ensures usability, engineers ensure feasibility, but *value and business viability are the stakeholder's call, not the PM's*), and **empowered team product management** (the PM is explicitly, personally responsible for ensuring value and viability, the designer for usability, the tech lead for feasibility) (Cagan, referencing his own prior work in "AI Product Management 2 Years In," [svpg.com/ai-product-management-2-years-in/](https://www.svpg.com/ai-product-management-2-years-in/)). This matters for audit purposes: a persona critiquing "why doesn't this product make a hard tradeoff" is asking a different, often unfair, question of a feature-team PM (who structurally cannot make that call) than of an empowered-team PM (who owns it). **SYNTHESIS**: an auditor should establish which organizational mode a product team is operating in *before* deploying personas that assume full ownership authority (the Craft Purist, the Feature-Factory Skeptic) — otherwise the critique lands on the wrong target.

---

## 4. Persona Roster for Audits

Each persona below is self-contained: a worldview, what it optimizes for, what it reflexively flags, 5–8 signature questions it would ask in an audit, its own characteristic blind spots (so it can be deliberately paired with a complementary lens), what "good" looks like through its eyes, its source grounding, and — where natural — a one-line note on how it would apply to a developer-tool / AI-agent-infrastructure product like `ghx`.

---

### 1. The Feature-Factory Skeptic

**Worldview.** Teams default to shipping because shipping feels safe and legible; the actual job of product management is deciding what *not* to build until its value is validated.

**Optimizes for.** Outcomes over output; evidence of value created, not features delivered.

**Reflexively flags.** Roadmaps that are lists of features with no stated outcome; velocity/story-points reported as a success metric; "we shipped X" presented as an accomplishment in itself.

**Signature questions.**
- What outcome does this feature move, and how will we know if it didn't?
- What would we have learned if we hadn't built this?
- Who asked for this — a customer with a validated problem, or an internal stakeholder's opinion?
- What's the smallest thing we could ship to test the underlying hypothesis?
- If this shipped and nothing changed, would anyone notice, and when?

**Blind spots.** Can undervalue foundational/infrastructure investment whose payoff is compounding and not immediately visible as an "outcome"; can be weaponized to stall anything lacking an instant metric, including necessary bets.

**What "good" looks like.** A roadmap organized by problems/outcomes rather than features; kill criteria defined *before* build starts; a team that can articulate what it learned from what it chose not to build.

**Source grounding.** Melissa Perri (CEO, Produx Labs; author, *Escaping the Build Trap*; former Harvard Business School instructor), "The Build Trap," original 2014 post, [melissaperri.com/blog/2014/08/05/the-build-trap](https://melissaperri.com/blog/2014/08/05/the-build-trap) — coined the term and the outputs-vs-outcomes framing directly ("Building is the easy part... Figuring out what to build... is the hard part"; the reframe from "What have you delivered today?" to "What have you learned about our customers or our business?"). Rationale: this is the primary, original articulation, not a summary of the later book.

**Applied to `ghx`.** Flags "we added five new subcommands this sprint" as a non-answer; asks which agent-eval outcome (episode success rate, judge score, time-to-context) actually moved.

---

### 2. The User-Empath

**Worldview.** Product decisions are only as good as the last real, unscripted contact with the people who use the product; teams substitute internal opinion for external evidence constantly and don't notice they've done it.

**Optimizes for.** Continuous, weekly contact with real users; an opportunity space defined by observed pain, not assumed pain.

**Reflexively flags.** Roadmaps built from stakeholder requests or competitor-parity with no named opportunity behind them; "we know what users want" asserted with no research trail; a research effort that was a one-off sprint rather than a standing habit.

**Signature questions.**
- When did anyone on this team last watch a real user try to use this, unprompted?
- What specific opportunity — in the user's own words — does this address? Can you show me the note?
- Are we solving the most important opportunity, or just the loudest one?
- What did we do with the last piece of disconfirming feedback we received?
- If the target user were in this room, what would they say we got wrong?

**Blind spots.** Can over-index on the vocal, reachable minority (support tickets, the users willing to be interviewed) versus a silent, statistically-representative majority; qualitative empathy can override valid quantitative signal.

**What "good" looks like.** A visible artifact (e.g., an opportunity-solution tree) connecting outcome → opportunity → solution → test; a standing weekly cadence of user contact, not a one-time research push.

**Source grounding.** Teresa Torres (author, *Continuous Discovery Habits*; founder, Product Talk), "Opportunity Solution Trees," [producttalk.org/opportunity-solution-trees/](https://www.producttalk.org/opportunity-solution-trees/) — her own canonical explainer of the method, including the explicit warning that "when we generate opportunities off the top of our heads, we bring our own biases and half-truths into the picture." Secondary grounding: Colin Bryar (former Amazon VP, Chief of Staff to Jeff Bezos) & Bill Carr (former Amazon VP Digital Media), *Working Backwards* (St. Martin's Press, 2021) — the primary documented account of Amazon's PR/FAQ discipline of starting every product decision from a customer-facing press release before writing code (book citation; specific-chapter deep link not independently verified — companion site [workingbackwards.com](https://workingbackwards.com/)).

**Applied to `ghx`.** For an agent-facing sidecar, the "user" is frequently the coding agent itself plus the human supervising it; this persona asks whether anyone has recently read raw agent transcripts/episode traces, not just aggregate judge scores.

---

### 3. The Monetization Hawk

**Worldview.** A product without a clear, *owned* answer to "who pays, for what, when, how, and how much" is accumulating monetization debt that compounds silently until a crisis forces a reactive scramble.

**Optimizes for.** Margin; willingness-to-pay evidence; a monetization model whose value metric scales with the value actually delivered (not flat-rate seats regardless of usage).

**Reflexively flags.** Pricing/packaging treated as a launch-week afterthought; "we'll figure out monetization later" on anything with real usage; usage-based value delivered under flat-rate pricing (especially compute/LLM-heavy products); no named owner for monetization decisions.

**Signature questions.**
- Who explicitly owns this monetization decision, and who would you call this week to change it?
- What's the value metric — does what we charge for scale with what the customer gets?
- What share of the value we create are we capturing today, and is that widening or narrowing?
- If cost-to-serve doubled tomorrow (e.g., token costs), does this pricing model survive?
- What would an aggressive freemium/predatory competitor move do to us?

**Blind spots.** Can push premature monetization pressure onto pre-PMF products or internal-tooling products where the "customer" is another team, not a payer; can undervalue trust-building loss-leaders.

**What "good" looks like.** A named monetization owner or council; a pricing model tied to a real value metric; monetization treated as a living hypothesis revisited on a cadence, not a one-time launch decision.

**Source grounding.** Elena Verna (Reforge Partner; growth leadership at MongoDB, Miro, Amplitude, SurveyMonkey; currently Growth at Lovable), "You should probably form a monetization council," [elenaverna.com/p/you-should-probably-form-a-monetization](https://www.elenaverna.com/p/you-should-probably-form-a-monetization) — direct primary argument that monetization sits in organizational "no-man's-land" ("Everyone touches it, but no one fully owns it") and her explicit claim that "usage-based pricing will become the norm — especially in AI-powered tools where cost and value scale with consumption." Secondary grounding: Gibson Biddle (former VP Product/CPO, Netflix; later Chegg), "The DHM Model," [gibsonbiddle.medium.com/2-the-dhm-model-6ea5dfd80792](https://gibsonbiddle.medium.com/2-the-dhm-model-6ea5dfd80792) — the "Margin-enhancing" leg of his primary Netflix strategy framework ties monetization to strategy, not just pricing-page tweaks.

**Applied to `ghx`.** `ghx` is internal open-source infrastructure inside an Agent Sidecar Framework — this persona is most relevant if/when it's offered externally; internally it reframes as "cost to serve" (LLM judge calls, compute) versus value delivered per episode.

---

### 4. The Platform/Scale Realist

**Worldview.** Internal builders and downstream teams are customers too, and every platform decision ripples through every team built on top of it; the ecosystem flywheel only turns as long as the platform is trustworthy.

**Optimizes for.** Leverage — one platform investment multiplying many teams' output — and the health/trust of everyone building on top.

**Reflexively flags.** Interfaces/contracts changed without notice to downstream consumers; no clear line between what the platform controls and what builders-on-top control; success measured by feature count rather than adoption/leverage.

**Signature questions.**
- Who builds on top of this, and did we tell them before changing the contract?
- Is this a *product* decision (we control the experience) or a *platform* decision (we control who gets to build the experience)?
- What's the flywheel — does more usage attract more builders, who in turn attract more usage?
- What's our deprecation/versioning policy, and would a builder actually trust it?
- If this broke silently for one downstream consumer, would we find out before they complained?

**Blind spots.** Platform-first thinking can over-engineer for hypothetical future builders instead of shipping value to the first real one; can drift into internal tooling for its own sake, divorced from any end-outcome.

**What "good" looks like.** Stable, versioned contracts; visible adoption/leverage metrics (how many teams/agents build on this, not how many features it has); documented trust investment (docs, SLAs, changelogs) treated as first-class product work.

**Source grounding.** Brandon Chu (VP Product, Shopify; scaled Shopify's app-platform ecosystem and PM org from 5 to hundreds), "Platform Management," Mind the Product, [mindtheproduct.com/platform-management-by-brandon-chu/](https://www.mindtheproduct.com/platform-management-by-brandon-chu/) — his primary distinction that "a product is building something to ship to customers, a platform is building a place where other builders or creators can build things to ship to customers," and that trust is "the flywheel's grease." Secondary grounding: Reforge's Platform PM specialization definition, [reforge.com/blog/product-specializations-pt2](https://www.reforge.com/blog/product-specializations-pt2) (internal stakeholders as customers; scaling work).

**Applied to `ghx`.** This is `ghx`'s home turf — it is explicitly a "sidecar" other agents/tools build on top of. This persona asks whether `ghx`'s CLI surface (flags, output formats) is a contract other agents can rely on, not just today's convenient shape.

---

### 5. The Technical-Debt Realist

**Worldview.** Every unaddressed sustainability cost is a future outage, slowdown, or forced rewrite; engineers' honest framing of debt ("the code is messy, it slows us down") is true but useless to a non-engineer making resourcing calls unless translated into time, risk, or velocity.

**Optimizes for.** Long-run velocity and reliability, not just this quarter's feature count; a standing, protected allocation of capacity to sustainability work.

**Reflexively flags.** 100% of capacity allocated to net-new features quarter after quarter; "let's just rewrite it" proposed without acknowledging migration risk or the functionality a legacy system quietly carries; technical debt discussed only in engineering-only forums, never translated to business consequence.

**Signature questions.**
- What percentage of this team's capacity goes to sustainability work, and has that number been declining?
- What's the blast radius if this system fails at 3am, and who actually knows?
- Translate this debt into time, risk, or velocity terms a non-engineer would act on — can you?
- What functionality does the legacy system have today that a rewrite would silently drop?
- If we do nothing here for two more quarters, what breaks first?

**Blind spots.** Can become a blank check engineers use to gold-plate or rewrite for aesthetic reasons rather than real risk; "debt" framing can obscure that some past shortcuts were correct bets that already paid off.

**What "good" looks like.** A visible, protected allocation (Fournier suggests roughly 20%) for sustainability work every planning cycle; debt discussed in time/risk/velocity terms that survive translation to a roadmap review.

**Source grounding.** Camille Fournier (former CTO, Rent the Runway; former VP Technology, Goldman Sachs; author, *The Manager's Path*, O'Reilly), interviewed in "The things engineers are desperate for PMs to understand," Lenny's Newsletter, [lennysnewsletter.com/p/engineering-leadership-camille-fournier](https://www.lennysnewsletter.com/p/engineering-leadership-camille-fournier) — direct, PM-facing articulation of the tech-debt/prioritization translation problem from the engineering side, including her caution that "legacy systems, despite their flaws, often have significant functionality and thoughtful design built over time" and that rewrites can be "deceptively complex."

**Applied to `ghx`.** For a Go/Cobra CLI with an eval harness, this persona asks about the harness's *own* sustainability — flaky episode infrastructure, judge-prompt drift — and whether "one more eval feature" is crowding out fixes to known measurement-stack debt.

---

### 6. The North-Star Zealot

**Worldview.** Without one metric the whole org can rally behind, every team optimizes a local metric that doesn't add up to real value; alignment beats local cleverness.

**Optimizes for.** A single, customer-value-correlated metric, with legible input metrics feeding it, so many small decisions triangulate toward the same target.

**Reflexively flags.** Multiple competing "north star" candidates never resolved; a north star chosen for optimizability rather than genuine correlation to value; a metric fixed for years without being re-validated against real outcomes.

**Signature questions.**
- What's the one metric that best represents value delivered to the customer — and how do you know it actually correlates with retention/revenue, not just activity?
- What are the 3–5 input metrics that ladder up to it, and does every team know which one they own?
- Has this metric ever been gamed? What did that look like?
- If this metric rose 20% next quarter, would leadership actually believe the product got better?
- Is this a proxy for delight, or a proxy for whatever was easiest to move?

**Blind spots.** A single metric can be gamed, or can flatten genuinely different value across different user segments into one number; over-indexing on it can suppress exploration of adjacent opportunities that don't move it in the short run.

**What "good" looks like.** One clearly-owned, periodically-revalidated north star with documented input metrics and an explicit "how this could be gamed" analysis.

**Source grounding.** John Cutler (product coach) with Amplitude, "Introducing The North Star Playbook," [amplitude.com/blog/introducing-north-star-playbook](https://amplitude.com/blog/introducing-north-star-playbook) — Amplitude is the analytics platform that operationalized and popularized the North Star Metric framework industry-wide; the playbook frames the metric as representing "the value your product provides its customers" and existing to "accelerate decision-making" via alignment. Secondary grounding: Gibson Biddle, "The DHM Model" (cited above) — the Netflix-primary source for tying a single strategic frame to metric selection.

**Applied to `ghx`.** Asks what the single evaluable proxy for "the agent got better context, faster" actually is (e.g., judge-scored episode success rate), and whether it's being gamed via judge-prompt tuning rather than real capability gains — directly relevant to the project's own Evidence Contract discipline.

---

### 7. The Competitive-Paranoid

**Worldview.** Market fundamentals can change discontinuously — a "strategic inflection point" — and by the time it's obvious to the whole leadership team, insiders and people closest to the front line have usually seen it coming for months. Only vigilance catches it early.

**Optimizes for.** Early detection of 10x shifts in technology, competitors, customers, or regulation, and the organizational courage to act on weak signals before they're consensus.

**Reflexively flags.** Strategy decks that assume today's competitive landscape is stable; dismissing a new entrant or technology as "not a real threat yet"; ignoring dissenting signals from people closest to the front line (support, sales engineers, junior ICs).

**Signature questions.**
- What would have to be true for a competitor to make this product irrelevant within 12 months?
- Whose job is it to notice the next 10x shift, and are we actually listening to them?
- Where do our internal metrics/incentives disagree with what customers are telling us — what's the dissonance signal?
- If a well-funded team started from scratch today with today's technology, what would they build instead of this?
- What's the equivalent of "we're a DRAM company" belief we're still operating on that's already gone stale?

**Blind spots.** Chronic paranoia can produce reactive, thrash-y strategy that chases every shiny threat rather than building durable advantage; can undervalue steady, unglamorous execution.

**What "good" looks like.** A standing practice of listening to weak signals from the periphery of the org, not just leadership's own view; explicit "what would kill us" exercises revisited regularly; willingness to cannibalize a working product line before a competitor does.

**Source grounding.** Andy Grove (co-founder and former CEO, Intel), *Only the Paranoid Survive* (Doubleday, 1996) — originated "strategic inflection point" and "strategic dissonance" from Intel's own DRAM-to-microprocessor pivot. Analysis independently verified via Stanford Graduate School of Business, "What Do You Do When Industry Dynamics Fundamentally Change?", [gsb.stanford.edu/insights/what-do-you-do-when-industry-dynamics-fundamentally-change](https://www.gsb.stanford.edu/insights/what-do-you-do-when-industry-dynamics-fundamentally-change), which documents that "many members of the top management team still thought of Intel as a DRAM company, although its market share had dwindled to a measly 2 to 3 percent" and that early warning came from middle managers, not leadership. (Book cited by title/author; a specific-chapter deep link to the book itself was not independently confirmed — the Stanford GSB piece is the verified deep source.)

**Applied to `ghx`.** In AI-coding-agent infrastructure specifically, asks what happens to `ghx`'s value proposition if a frontier model vendor ships equivalent code-reconnaissance capability natively into the agent — a classic platform-absorbs-the-feature threat.

---

### 8. The Simplicity/Anti-Bloat Minimalist

**Worldview.** Every "yes" to a feature is an invisible tax on every future user and every future feature; the discipline of a product is defined as much by what it refuses to become as by what it does.

**Optimizes for.** A small, coherent product surface where every included capability is load-bearing.

**Reflexively flags.** Optional/configurable settings added to avoid making a hard prioritization call; scope creep justified as "it'll only take a few minutes"; a growing flags/settings surface with no corresponding removal.

**Signature questions.**
- What does this feature cost every other user, even the ones who never touch it?
- If we said no to this, what would we tell the person who asked?
- Does this make the product more itself, or less?
- What would we remove to make room for this?
- Is this genuinely load-bearing, or just "nice to have as an option"?

**Blind spots.** Minimalism can under-serve power users who genuinely need configurability/extensibility, especially in developer tools; "no" as a reflex can starve legitimate platform flexibility the Platform/Scale Realist would defend.

**What "good" looks like.** A product with a legible, describable identity ("this is an X that does Y, not Z"); visible practice of removing/deprecating as often as adding; features justified by total cost of ownership (support, docs, interaction effects), not just build cost.

**Source grounding.** Des Traynor (co-founder and Chief Strategy Officer, Intercom), "Product strategy means saying no," [intercom.com/blog/product-strategy-means-saying-no/](https://www.intercom.com/blog/product-strategy-means-saying-no/) — his own primary essay on feature creep and "death by preferences": "Building a great product isn't about creating tons of tactically useful features which are tangentially related. It's about delivering a cohesive product with well defined parameters," and "even the tiniest additions add hidden complexity that isn't accounted for" in effort estimates.

**Applied to `ghx`.** The natural check on flag/subcommand sprawl in a CLI — every new `ghx` flag is a permanent addition to the interface contract every future agent (and human) has to learn and every doc has to explain.

---

### 9. The Experimentation Rigorist

**Worldview.** Most "we tested it and it won" claims don't survive a rigor audit; getting a number is easy, getting a number you can trust is hard.

**Optimizes for.** Statistically and methodologically trustworthy measurement of experiments, surveys, and any claim that a change "worked."

**Reflexively flags.** Single-metric success claims with no guardrail metrics checked; sample sizes too small to support the claimed effect; a survey/test instrument run once and never revalidated as the population shifts; anecdote presented as quantitative proof.

**Signature questions.**
- What's the sample size, and is it powered to detect the effect you're claiming?
- What guardrail metrics did you check besides the one that moved in your favor?
- Could novelty effect, seasonality, or a concurrent change explain this result?
- Would this replicate if rerun next quarter with a fresh cohort?
- Is this a validated instrument (e.g., the 40%-"very disappointed" benchmark) or something invented ad hoc for this one measurement?

**Blind spots.** Rigor obsession can slow decisions that don't need statistical significance (early-stage, low-stakes, or easily-reversible calls); can dismiss valid qualitative signal for lacking a p-value.

**What "good" looks like.** A standing, validated measurement instrument (e.g., a PMF survey run quarterly, not once); guardrail metrics defined before an experiment ships; results reported with uncertainty, not just a headline number.

**Source grounding.** Ron Kohavi, Diane Tang, and Ya Xu (experimentation platform leaders at Microsoft, Google, Airbnb, and LinkedIn respectively), *Trustworthy Online Controlled Experiments* (Cambridge University Press, 2020) — the field's most-cited methodology reference, built on the premise that "getting numbers is easy; getting numbers you can trust is hard" (book citation; companion primary document verified live: Kohavi & Longbotham, "Online Controlled Experiments and A/B Tests," [exp-platform.com/Documents/2023-03-11EncyclopeiaMLDSABTestingFinal.pdf](https://exp-platform.com/Documents/2023-03-11EncyclopeiaMLDSABTestingFinal.pdf)). Secondary grounding: Rahul Vohra (founder/CEO, Superhuman), "How Superhuman Built an Engine to Find Product Market Fit," First Round Review, [review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/](https://review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/) — operationalizes Sean Ellis's validated 40%-"very disappointed" PMF benchmark and explicitly cautions that "your product-market fit score may well drop" as the user base matures past early adopters, requiring the roadmap to be rebuilt quarterly.

**Applied to `ghx`.** Maps directly onto `ghx`'s own eval harness — this persona audits episode sample sizes, judge calibration, and whether "the new sidecar view scored higher" survived a guardrail check against known confounds, per the project's own Evidence Contract.

---

### 10. The GTM/Positioning Strategist

**Worldview.** Most "the product isn't selling" problems are actually "nobody understands what this is or who it's for" problems; positioning is a deliberate choice a team makes, not a byproduct of the feature list.

**Optimizes for.** A product that is obviously the right choice for a well-defined best-fit customer, evaluated against the correct competitive alternative.

**Reflexively flags.** Every function of the company describing the product differently; positioning inherited by default rather than chosen; a "who is this for" answer that is actually "everyone."

**Signature questions.**
- What are customers actually comparing this to right now — what's the literal alternative they'd choose instead?
- What can this do that the alternative can't, and who genuinely cares about that?
- Who is the best-fit customer — not who *could* use this, but who needs it most?
- If sales, marketing, and support each described this product in one sentence, would the sentences agree?
- What percentage of "losses" are actually losses to "no decision," not to a named competitor?

**Blind spots.** Can push a product toward narrower positioning than its real addressable use, trading genuine market breadth for message clarity; can treat positioning as a wording exercise rather than a real product-scope decision.

**What "good" looks like.** One written positioning statement every function of the company recites the same way; an explicit, chosen competitive alternative and best-fit customer segment, revisited as the market shifts.

**Source grounding.** April Dunford (independent positioning consultant; has personally positioned 200+ B2B companies including Google, IBM, Postman, Epic Games; author, *Obviously Awesome*, self-published, 2019) — 5-step framework (competitive alternatives → unique attributes → value themes → target customers → market context) and the finding that "in B2B you lose about 40% of deals to 'no decision.'" Verified via summary interview: "Summary: April Dunford on product positioning, segmentation, and optimizing your sales process," Lenny's Newsletter, [lennysnewsletter.com/p/summary-april-dunford-on-product](https://www.lennysnewsletter.com/p/summary-april-dunford-on-product). (Book cited by title/author; a specific-chapter deep link to the book itself was not independently confirmed.)

**Applied to `ghx`.** For internal infra, "positioning" is still real — competing internally against "just let the agent grep/read files itself" or a generic RAG tool; this persona asks whether that comparison is being won on stated, provable terms.

---

### 11. The Growth Systems Thinker

**Worldview.** Growth is a system connecting acquisition, retention, and monetization — not a series of isolated tactics or a team bolted onto the roadmap. Connecting more people to *existing* value is a distinct discipline from *creating* new value.

**Optimizes for.** Compounding loops (acquisition → activation → retention → referral → monetization), not one-off campaigns or isolated features.

**Reflexively flags.** "Growth" work that is actually just relabeled core-feature work; a growth tactic proposed in isolation from the rest of the funnel; no visibility into where users actually drop off.

**Signature questions.**
- Where exactly in the funnel are we losing the most value right now, and is this initiative aimed there?
- Is this about creating new value or capturing more of existing value — does the team agree which one?
- What's the loop — does this feed back into more acquisition/retention, or is it a dead-end campaign?
- What happens to this metric if we stop investing in it for a quarter?
- Have we tested the riskiest assumption in this growth bet, or the easiest one?

**Blind spots.** Can over-optimize existing funnels at the expense of building genuinely new value — Ravi Mehta names this trap explicitly, warning against "mistak[ing] growth optimization with product innovation"; can chase short-term funnel wins that don't build durable retention.

**What "good" looks like.** A mapped, instrumented funnel with a named owner per stage; growth initiatives explicitly labeled by which part of the system they serve, tied back to the whole rather than siloed.

**Source grounding.** Reforge, "The Growing Specialization of Product Management" (cited above), [reforge.com/blog/product-specializations-pt2](https://www.reforge.com/blog/product-specializations-pt2) — the primary "connecting customers to existing value" definition of growth work (crediting Fareed Mosavat and Casey Winters's internal Reforge framing). Secondary grounding: Ravi Mehta's "Growth Hacker" archetype and its named blind spot, [ravi-mehta.com/product-manager-roles/](https://www.ravi-mehta.com/product-manager-roles/).

**Applied to `ghx`.** For an eval-harness-driven sidecar, asks about the "acquisition" funnel for internal adoption — how many other agents/workflows actually invoke `ghx`, where does that funnel leak, and whether "growth" is being confused with just adding more subcommands.

---

### 12. The Craft Purist

**Worldview.** Craft and product quality are table stakes beneath every other kind of product leadership skill; a leader or product that is operationally excellent or strategically visionary but not genuinely *good at the product itself* is building on sand.

**Optimizes for.** Quality of the thing itself — polish, coherence, hands-on product judgment — over scale or vision alone.

**Reflexively flags.** Leaders/PMs who spend all their time on process, decks, or stakeholder alignment and can't speak fluently about the product's actual details; excuses offered for poor outcomes (funding, engineering, competitors) instead of ownership; a team that can no longer articulate what "good" looks like for its own surface.

**Signature questions.**
- Could you use this product yourself for a real task right now, and would you be satisfied?
- Is this leader/PM excellent at the craft of product, or just good at the politics around it?
- What's the excuse being offered here — would a "CEO of the product" accept it?
- What's the target (the "what"), and is it crisply defined, separately from the "how"?
- If this were your only product, would you be proud of it today?

**Blind spots.** Craft obsession can resist necessary process/scale investment as an org grows — exactly the "operator" work Doshi and Cagan describe as also necessary; can privilege one strong individual's taste over broader signal.

**What "good" looks like.** A PM/leader who can go deep on the product's actual details on demand, takes full ownership of outcomes without excuses, and treats craft as the non-negotiable floor beneath strategy or scale work.

**Source grounding.** Shreyas Doshi's Craftsperson/Operator/Visionary taxonomy, transmitted and extended by Marty Cagan, "Product Leadership Archetypes," SVPG, [svpg.com/product-leadership-archetypes/](https://www.svpg.com/product-leadership-archetypes/) — Cagan's explicit argument that craft is "table stakes" beneath the other two archetypes. Secondary grounding: Ben Horowitz and David Weiden, "Good Product Manager/Bad Product Manager," originally an internal Netscape PM training memo (c. 1997), republished by Andreessen Horowitz (2012), [a16z.com/good-product-manager-bad-product-manager/](https://a16z.com/good-product-manager-bad-product-manager/) — the "CEO of the product," no-excuses ownership standard ("Bad product managers have lots of excuses... Good product managers crisply define the target, the 'what'... Bad product managers feel best about themselves when they figure out 'how'").

**Applied to `ghx`.** Asks whether anyone on the team has actually used `ghx` as their own daily driver for real code-reconnaissance tasks recently, versus only reasoning about it via eval scores.

---

### 13. The Zero-to-One Explorer

**Worldview.** Before product-market fit, the job isn't to build well — it's to find out, as cheaply and quickly as possible, whether there's anything worth building well. Comfort with being wrong is the core skill.

**Optimizes for.** Fast, cheap validation of the riskiest assumption; willingness to pivot or kill without attachment to the original idea.

**Reflexively flags.** A "new initiative" run with feature-team rigor (heavy process, long build cycles) before any evidence of demand exists; a team that has never seriously entertained killing the project; validation activity that only confirms what the team already believed.

**Signature questions.**
- What's the riskiest assumption in this idea, and have we tested that one first, or the easiest one?
- What would convince you this idea is wrong — and would you actually act on that signal?
- What's the cheapest, fastest version of this we could put in front of a real person this week?
- Are we iterating, or should we seriously be considering a pivot?
- If we had to defend killing this project tomorrow, what evidence would we point to?

**Blind spots.** Can under-invest in the operational rigor (reliability, docs, scale) a validated idea needs once it graduates past 0-to-1; can romanticize pivoting and never let an idea's evidence actually accumulate.

**What "good" looks like.** A ranked list of the strategy's riskiest assumptions, each with a test plan attached; a genuine track record of having killed or pivoted ideas based on evidence, not just shipped them regardless.

**Source grounding.** Sachin Rekhi, "The 4 Types of Product Managers" (cited above), [sachinrekhi.com/p/3-types-of-product-managers-builders-tuners-innovators](https://www.sachinrekhi.com/p/3-types-of-product-managers-builders-tuners-innovators) — his "Innovator" archetype defined explicitly around hypothesis-driven validation and comfort with failure. Secondary grounding: Reforge's Innovation PM specialization (organizational framing: skunkworks/new-verticals context), [reforge.com/blog/product-specializations-pt2](https://www.reforge.com/blog/product-specializations-pt2); Rahul Vohra's PMF engine (concrete validation instrument), [review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/](https://review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/).

**Applied to `ghx`.** For a still-forming "Agent Sidecar Framework," asks which parts of the framework are genuinely validated (evidence a sidecar improves task success) versus built on the unverified assumption that sidecars help.

---

### 14. The Developer-Experience Advocate

**Worldview.** For a tool used by other builders (human or agent), the interface, docs, error messages, and daily friction *are* the product; productivity is measurable, not just felt.

**Optimizes for.** Measurable reductions in friction and time-to-value for the people/agents building on top of the tool — speed, effectiveness, quality, and impact of *their* work, not the tool's own feature count.

**Reflexively flags.** No instrumentation of how the tool is actually used day to day; failure modes that assume a human is reading slowly rather than a program (or agent) parsing at speed; documentation that describes intent rather than exact, testable behavior.

**Signature questions.**
- If a new consumer (human or agent) had to integrate this today with zero prior context, how long would it take, and where would they get stuck?
- What does a failure actually look like from the caller's side — legible and actionable, or a raw stack trace?
- What do we actually measure about how this tool is used, beyond "it shipped"?
- Is time saved by this tool being reinvested in new capability, or eaten by working around this tool's own friction?
- Would the tool's heaviest user describe using it as effortless, or as a tax paid to get real work done?

**Blind spots.** Can over-invest in polish/ergonomics for a narrow set of well-resourced power users while ignoring core capability gaps; productivity metrics can themselves be gamed by optimizing easy proxies (e.g., PRs per engineer) instead of real friction.

**What "good" looks like.** Instrumented, quantified developer/agent experience, not just anecdote; a tool whose failure modes are as deliberately designed as its success modes; visible investment across speed, effectiveness, quality, and impact as a unified framework rather than one dimension in isolation.

**Source grounding.** Abi Noda (CEO, DX; formerly founder/CEO of Pull Panda, acquired by GitHub), "Introducing the DX Core 4," [newsletter.getdx.com/p/introducing-the-dx-core-4](https://newsletter.getdx.com/p/introducing-the-dx-core-4) — co-developed with Laura Tacho (DX CTO) and researchers including Nicole Forsgren (a DORA co-creator), unifying DORA/SPACE/DevEx into one measurement framework. Secondary grounding: Brandon Chu's platform-PM framing that "your product is code" and developer trust is "the flywheel's grease," [mindtheproduct.com/platform-management-by-brandon-chu/](https://www.mindtheproduct.com/platform-management-by-brandon-chu/).

**Applied to `ghx`.** The single most load-bearing lens for `ghx` specifically — its actual "user" is frequently an AI coding agent parsing CLI output under time/context pressure. This persona asks whether error messages, flag semantics, and output formats are legible to an agent, not just to a human reading docs.

---

### 15. The AI PM Pragmatist

**Worldview.** Generative AI changes *what* a PM/product needs to do, not *whether* a PM is needed — and only amplifies the judgment a team already has, good or bad. AI amplifies existing product sense; it equally amplifies its absence.

**Optimizes for.** Grounded, testable claims about what a model can and can't do, translated into concrete product decisions — not hype-driven roadmaps.

**Reflexively flags.** "AI-powered" claimed as a feature rather than a capability with a measured success rate; capability claims with no evals behind them; treating the underlying model as a fixed product rather than a moving-target platform needing "opinionated workflows" wrapped around it; assuming today's capability ceiling is permanent, in either direction.

**Signature questions.**
- Have you "interviewed" the model the way you'd interview a customer — probed its actual failure modes, not just its demo-day behavior?
- What's the eval that backs this capability claim, and who scored it, from what evidence?
- Is this a product, or a thin wrapper around a model a platform update could absorb next quarter?
- What does the extreme, high-value version of this look like — have you asked what the $1,000/month version would need to do?
- If the underlying model changed materially next month, does this product's value proposition survive?

**Blind spots.** Can over-index on model-capability optimism and under-invest in the deterministic, unglamorous reliability work (retries, error handling, guardrails) that makes AI products trustworthy in production; can chase state-of-the-art model swaps as a substitute for real product judgment.

**What "good" looks like.** Capability claims backed by traced, calibrated evals; a product built as an "opinionated workflow around a platform," not a thin prompt wrapper; a team that treats AI tooling as a daily-use amplifier of already-strong product sense.

**Source grounding.** Marty Cagan, "AI Product Management 2 Years In," SVPG, [svpg.com/ai-product-management-2-years-in/](https://www.svpg.com/ai-product-management-2-years-in/) — his running distinction between AI-as-enabling-technology and AI-as-changed-workflow, and his explicit worry about powerful AI tools in the hands of teams lacking product judgment. Secondary grounding: Anish Acharya (General Partner, Andreessen Horowitz), "5 Principles for Product Managers Fending Off Obsolescence in the AI Era," [a16z.com/stay-relevant-in-ai/](https://a16z.com/stay-relevant-in-ai/) — "interview your models," "models are platforms, not products," and the extreme-pricing principle.

**Applied to `ghx`.** The second most load-bearing lens for `ghx` — asks whether the project's own eval harness (episodes, personas, LLM judges) produces traced, calibrated evidence per the Evidence Contract, or whether "the sidecar helped" is asserted rather than measured.

---

## 5. How to Compose a Persona Set

**SYNTHESIS.** The fifteen personas above were deliberately derived to sit on several partially-independent axes drawn from Sections 1–3. Composing a persona set for a given audit is a coverage problem across those axes, not a popularity contest among personas.

**5.1 Map personas to the underlying axes before picking.**

- **Ravi Mehta's quadrants** (Execution / Customer Insight / Strategy / Influencing People): the Feature-Factory Skeptic and Technical-Debt Realist sit mostly in Execution; the User-Empath sits in Customer Insight; the North-Star Zealot, Competitive-Paranoid, and GTM/Positioning Strategist sit mostly in Strategy; the Craft Purist spans Execution and Strategy by design (that's Cagan's point about craft being foundational).
- **Doshi/Cagan's triad** (Craftsperson / Operator / Visionary): the Craft Purist *is* the Craftsperson lens; the Platform/Scale Realist and Technical-Debt Realist are Operator-flavored; the Zero-to-One Explorer and Competitive-Paranoid are Visionary-flavored.
- **Reforge's four types of product work** (Feature / Growth / Scaling / PMF-expansion): the Feature-Factory Skeptic and User-Empath map to Feature work; the Growth Systems Thinker and Monetization Hawk map to Growth work; the Platform/Scale Realist and Developer-Experience Advocate map to Scaling work; the Zero-to-One Explorer maps to PMF-expansion.

Running three personas that all sit in "Operator/Execution/Scaling" territory (e.g., Platform/Scale Realist + Technical-Debt Realist + Developer-Experience Advocate together, with nothing else) will surface a lot of engineering-reality findings and nothing about whether the product should exist at all, who it's for, or what threatens it externally. Deliberately span axes instead.

**5.2 A general-purpose starter set (five personas, low redundancy).** For a first-pass audit of most products: **Feature-Factory Skeptic** (outcome discipline) + **User-Empath** (grounding in real usage) + **Technical-Debt Realist** (engineering reality) + **Platform/Scale Realist** (ecosystem reality, if the product has any downstream consumers) + **Competitive-Paranoid** (external threat awareness), plus one measurement-anchor persona (**Experimentation Rigorist** or **North-Star Zealot**, not both, since they overlap on "is the metric trustworthy"). This set deliberately covers Execution, Customer Insight, Strategy, and an external lens with minimal pairwise overlap.

**5.3 For developer-tool / AI-agent-infrastructure products specifically** (the class `ghx` belongs to): treat the **Developer-Experience Advocate** and **AI PM Pragmatist** as near-mandatory rather than optional, since they are the two personas most directly grounded in the product's actual domain (a CLI consumed by agents, inside an eval-harness-driven framework) — every other persona in the roster applies generically but these two apply *specifically*. The **Platform/Scale Realist** is also unusually load-bearing for this class of product, since the entire point of "sidecar" infrastructure is that other things build on top of it.

**5.4 Known redundant pairs — pick one, not both, unless deliberately double-covering.**
- Feature-Factory Skeptic + Craft Purist: heavy overlap on "no excuses, own the outcome."
- Growth Systems Thinker + Monetization Hawk: both chase revenue-adjacent metrics; fine to pair for double coverage on business viability, but understand you're not adding a new axis.
- Experimentation Rigorist + North-Star Zealot: both ultimately ask "is this metric trustworthy," from slightly different angles (methodology vs. alignment).

**5.5 Sizing the set.** Three to five personas per audit round is a reasonable default: enough diversity to surface non-overlapping issues, few enough that findings stay legible rather than drowning the team in overlapping critiques from near-identical worldviews. Escalate to the full fifteen-persona roster only for a rare, high-stakes, milestone-grade audit — not as a routine practice — mirroring the general principle that broader sweeps should be event-driven rather than habitual.

---

## Source Ledger

| # | Source | Author / Org | Specific URL | Why authoritative | What it supports |
|---|---|---|---|---|---|
| 1 | "The Growing Specialization of Product Management" | Adam Fishman (Reforge EIR), w/ Ravi Mehta, Crystal Widjaja | [reforge.com/blog/product-specializations-pt2](https://www.reforge.com/blog/product-specializations-pt2) | Reforge is the leading PM professional-education platform; authored by senior operators (ex-Tinder CPO, ex-Patreon VP Product, ex-Gojek SVP Growth) | §1.1 functional taxonomy; Personas #4, #11, #13 |
| 2 | "How To Navigate Product Management Specializations" | Ravi Mehta, Adam Greiner (Reforge) | [reforge.com/blog/product-specializations](https://www.reforge.com/blog/product-specializations) | Companion Reforge piece; adds career/team-composition lens across company stages | §1.1, §5 (team composition analogy) |
| 3 | "What's Your Shape? A Product Manager's Guide..." | Ravi Mehta (ex-CPO Tinder, VP Product Tripadvisor, Product Dir. Facebook) | [ravi-mehta.com/product-manager-roles/](https://www.ravi-mehta.com/product-manager-roles/) | One of the most cited "shape"/competency frameworks in PM discourse; author is a senior operator across three major consumer companies | §2.1; Personas #12, #11 |
| 4 | "Which Type of Product Manager Are You?" | Adam Fishman (ex-CPO Imperfect Foods, VP Product Patreon, Head of Growth Lyft) | [fishmanafnewsletter.com/p/identify-product-manager-archetypes-and-skills](https://www.fishmanafnewsletter.com/p/identify-product-manager-archetypes-and-skills) | Same author who wrote Reforge's canonical taxonomy, independently elaborating a more granular, blind-spot-explicit personal framework | §2.4 |
| 5 | "The 4 Types of Product Managers" | Sachin Rekhi (PM/entrepreneur, LinkedIn, Notejoy) | [sachinrekhi.com/p/3-types-of-product-managers-builders-tuners-innovators](https://www.sachinrekhi.com/p/3-types-of-product-managers-builders-tuners-innovators) | One of the earliest (2016) and most widely reused PM personality frameworks, predating Reforge's | §2.3; Personas #13, #11 |
| 6 | "Product Leadership Archetypes" | Marty Cagan (Founder, SVPG; author, *INSPIRED*/*EMPOWERED*) | [svpg.com/product-leadership-archetypes/](https://www.svpg.com/product-leadership-archetypes/) | Cagan is arguably the most cited living PM authority; article transmits and critically extends Shreyas Doshi's craftsperson/operator/visionary taxonomy | §2.2; Personas #1, #12 |
| 7 | "AI Product Management 2 Years In" | Marty Cagan (SVPG) | [svpg.com/ai-product-management-2-years-in/](https://www.svpg.com/ai-product-management-2-years-in/) | Same authority; running critique of AI's effect on the PM job, and the three-types-of-PM (delivery/feature/empowered) framing | §1.4, §3; Persona #15 |
| 8 | "Good Product Manager/Bad Product Manager" | Ben Horowitz & David Weiden; orig. internal Netscape memo (c. 1997), republished by a16z (2012) | [a16z.com/good-product-manager-bad-product-manager/](https://a16z.com/good-product-manager-bad-product-manager/) | The most reprinted essay in PM canon; defines the "CEO of the product" ownership standard nearly every later archetype implicitly measures against | Persona #12 |
| 9 | "Product Manager Archetype" | Sara Paul, Nielsen Norman Group | [nngroup.com/articles/product-manager-archetype/](https://www.nngroup.com/articles/product-manager-archetype/) | NN/g is the leading UX-research-methodology authority; applies the archetype (vs. persona) construct specifically to PMs as a collaboration/audit tool | Methodological framing (archetype vs. persona); Persona #2 |
| 10 | "From ChatGPT to Instagram to Uber..." (Peter Deng interview) | Peter Deng, interviewed by Lenny Rachitsky | [lennysnewsletter.com/p/the-quiet-architect-peter-deng](https://www.lennysnewsletter.com/p/the-quiet-architect-peter-deng) | Deng held product leadership at OpenAI, Instagram, Uber, Facebook, Airtable, Oculus — unusually broad convergent cross-check on the Reforge taxonomy | §1.2; §5 (team-composition rationale) |
| 11 | "The things engineers are desperate for PMs to understand" (Camille Fournier interview) | Camille Fournier, interviewed by Lenny Rachitsky | [lennysnewsletter.com/p/engineering-leadership-camille-fournier](https://www.lennysnewsletter.com/p/engineering-leadership-camille-fournier) | Fournier is former CTO Rent the Runway, VP Technology Goldman Sachs, author *The Manager's Path* | Persona #5 |
| 12 | "The Build Trap" (original post) | Melissa Perri (CEO Produx Labs; author *Escaping the Build Trap*) | [melissaperri.com/blog/2014/08/05/the-build-trap](https://melissaperri.com/blog/2014/08/05/the-build-trap) | Primary, original 2014 articulation of the outputs-vs-outcomes critique later expanded into her book | Persona #1 |
| 13 | "Opportunity Solution Trees" | Teresa Torres (author *Continuous Discovery Habits*; founder, Product Talk) | [producttalk.org/opportunity-solution-trees/](https://www.producttalk.org/opportunity-solution-trees/) | Torres's own canonical explainer of the discovery method now taught industry-wide | Persona #2 |
| 14 | *Obviously Awesome*, summarized via Lenny interview | April Dunford (positioning consultant; positioned 200+ B2B cos.) | [lennysnewsletter.com/p/summary-april-dunford-on-product](https://www.lennysnewsletter.com/p/summary-april-dunford-on-product) | Author has personally positioned Google, IBM, Postman, Epic Games; primary 5-step framework and "40% no-decision" statistic | Persona #10 |
| 15 | "How Superhuman Built an Engine to Find Product Market Fit" | Rahul Vohra (founder/CEO, Superhuman) | [review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/](https://review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/) | First Round Review is a leading VC-backed practitioner publication; Vohra operationalized Sean Ellis's validated 40% PMF benchmark with explicit degradation caveats | Personas #9, #13 |
| 16 | *Trustworthy Online Controlled Experiments* / companion encyclopedia entry | Ron Kohavi, Diane Tang, Ya Xu; Kohavi & Longbotham | [exp-platform.com/Documents/2023-03-11EncyclopeiaMLDSABTestingFinal.pdf](https://exp-platform.com/Documents/2023-03-11EncyclopeiaMLDSABTestingFinal.pdf) | Kohavi ran experimentation platforms at Microsoft, Amazon, Airbnb; most-cited authority on trustworthy A/B methodology | Persona #9 |
| 17 | "Platform Management by Brandon Chu" | Brandon Chu (VP Product, Shopify) | [mindtheproduct.com/platform-management-by-brandon-chu/](https://www.mindtheproduct.com/platform-management-by-brandon-chu/) | Scaled Shopify's app-platform ecosystem and PM org from 5 to hundreds; primary platform-vs-product definition | Personas #4, #14 |
| 18 | "Introducing The North Star Playbook" | John Cutler with Amplitude | [amplitude.com/blog/introducing-north-star-playbook](https://amplitude.com/blog/introducing-north-star-playbook) | Amplitude popularized/operationalized the North Star Metric framework industry-wide | Persona #6 |
| 19 | "#1 The DHM Model" | Gibson Biddle (former VP Product/CPO, Netflix; later Chegg) | [gibsonbiddle.medium.com/2-the-dhm-model-6ea5dfd80792](https://gibsonbiddle.medium.com/2-the-dhm-model-6ea5dfd80792) | Primary articulation of the strategy framework he ran at Netflix | Personas #3, #6 |
| 20 | *Only the Paranoid Survive* / analysis | Andy Grove (former CEO, Intel); analysis via Stanford GSB | [gsb.stanford.edu/insights/what-do-you-do-when-industry-dynamics-fundamentally-change](https://www.gsb.stanford.edu/insights/what-do-you-do-when-industry-dynamics-fundamentally-change) | Grove originated "strategic inflection point" from Intel's own DRAM-to-microprocessor pivot | Persona #7 |
| 21 | "Product strategy means saying no" | Des Traynor (co-founder, Chief Strategy Officer, Intercom) | [intercom.com/blog/product-strategy-means-saying-no/](https://www.intercom.com/blog/product-strategy-means-saying-no/) | Primary essay on feature creep, "death by preferences," hidden cost of every "yes" | Persona #8 |
| 22 | "You should probably form a monetization council" | Elena Verna (Reforge Partner; growth leadership MongoDB, Miro, Amplitude, SurveyMonkey, Lovable) | [elenaverna.com/p/you-should-probably-form-a-monetization](https://www.elenaverna.com/p/you-should-probably-form-a-monetization) | Primary argument for monetization ownership and usage-based pricing in the AI era | Persona #3 |
| 23 | "Introducing the DX Core 4" | Abi Noda (CEO, DX; ex-founder/CEO Pull Panda, acq. GitHub) | [newsletter.getdx.com/p/introducing-the-dx-core-4](https://newsletter.getdx.com/p/introducing-the-dx-core-4) | Leading current authority unifying DORA/SPACE/DevEx into one measurable framework | §1.3; Persona #14 |
| 24 | "5 Principles for Product Managers Fending Off Obsolescence in the AI Era" | Anish Acharya (General Partner, a16z) | [a16z.com/stay-relevant-in-ai/](https://a16z.com/stay-relevant-in-ai/) | a16z's primary practitioner-facing framework for how AI changes the PM job | §1.4; Persona #15 |
| 25 | *Working Backwards: Insights, Stories, and Secrets from Inside Amazon* | Colin Bryar (ex-VP Amazon, Chief of Staff to Jeff Bezos), Bill Carr (ex-VP Digital Media, Amazon) | Book (St. Martin's Press, 2021); companion site [workingbackwards.com](https://workingbackwards.com/) | Primary documented account of Amazon's PR/FAQ customer-obsession discipline, from two long-serving Amazon executives | Persona #2 |

**Note on unverified deep links.** Three sources (#7/#20 Andy Grove's book itself, #14 April Dunford's book itself, #25 *Working Backwards*) are cited by title/author with a verified companion or analysis URL, because the specific in-book passage could not be independently confirmed at a stable deep URL using text-only tools. All other 22 sources were fetched and verified live during this research (either directly, or via a browser-user-agent `curl` fetch where the default fetch tool returned HTTP 403).

---

## Personas: Sourced vs. Synthesis Summary

All fifteen personas in the roster are **grounded in at least one named, verified primary source** (see each persona's "Source grounding" field and the Source Ledger above); none are invented from whole cloth. That said, the *construction* of each persona as a self-contained audit-lens block — worldview, reflexive flags, signature questions, blind spots, "what good looks like" — is this document's own synthesis, translating each source's descriptive framework into a prescriptive, role-playable interrogation tool. Specifically:

- **Directly sourced content** (quoted or closely paraphrased from the named authority): the worldview statement, the "optimizes for" framing, and most signature questions in each persona block.
- **This document's synthesis**: the explicit "blind spots" field for every persona (sources rarely name their own framework's weaknesses as directly as this document does — Adam Fishman's seven-archetype piece is the one partial exception, since it names struggles directly); the "Applied to `ghx`" notes throughout; all of Section 3 (organizational axis as a pre-condition for persona selection) and Section 5 (how to compose a persona set) in their entirety; and the cross-referencing in §5.1 mapping personas onto Mehta's/Doshi's/Reforge's underlying axes, which no single source states explicitly.
