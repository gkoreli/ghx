# How Expert Product Managers Audit and Manually Validate a Product

## A methodology reference: heuristic evaluation, teardown, discovery risk-testing, metrics audits, PMF mechanics, docs/onboarding review, and dogfooding — with transferable application to CLI and AI-agent-facing products

---

## Abstract

Expert product validation is not one activity but a stack of complementary inspection
methods, each cheap, fast, and biased toward catching a different class of failure.
Usability-inspection methods (heuristic evaluation, cognitive walkthrough, expert
review) find *interaction* defects without recruiting users. Discovery frameworks
(Cagan's four risks, Torres' assumption testing) find *strategic* defects — building
the wrong thing, for the wrong reason, before a line of code is written. Metrics
frameworks (HEART, North Star + inputs) find *measurement* defects — optimizing a
number that doesn't represent value. PMF mechanics (Sean Ellis's 40% test, Vohra's
PMF engine, Balfour's retention-curve method) find *market* defects — a product that
works but that not enough people need badly enough. Documentation/onboarding audits
find *comprehension* defects — a correct product that nobody can figure out how to
use. And dogfooding / structured self-use finds the defects that only show up when a
real task is run start-to-finish, evidence in hand, under real constraints.

None of these methods is sufficient alone, and none requires a lab: every procedure
described below can be run by one to six people, in hours to a few days, using the
product itself, a spreadsheet, and a written report. This document lays out each
method's origin, exact procedure, and evidence artifact, cites the primary source
for each, and closes with a single consolidated checklist an auditor can execute
against a live product — including an applied-synthesis section on what "manual
validation" means when the user being validated for is an AI coding agent rather
than a human.

Every source below was fetched and read (not inferred from memory); the *Source
Ledger* at the end gives one link and one rationale sentence per source. Sections
that go beyond what a cited source states are explicitly marked **[SYNTHESIS]**.

---

## 1. Expert & Heuristic Inspection Methods

These are the cheapest, fastest audit tools available: they need no users, no
recruiting, and no lab — only one to six people who know the method and a defined
scope. Nielsen's own summary of the field lists eight distinct inspection methods
(heuristic evaluation, heuristic estimation, cognitive walkthrough, pluralistic
walkthrough, feature inspection, consistency inspection, standards inspection, and
formal usability inspection), several of which are detailed below.
[[NNG — Summary of Usability Inspection Methods]](https://www.nngroup.com/articles/summary-of-usability-inspection-methods/)

### 1.1 Nielsen's 10 Usability Heuristics — the rubric

**What it is.** Ten broad rules of thumb for interface design, developed by Jakob
Nielsen with Rolf Molich in 1990 and refined in 1994 from a factor analysis of 249
real usability problems. They are not a checklist of specific UI rules but general
principles evaluators apply with judgment. The ten: (1) Visibility of system status,
(2) Match between the system and the real world, (3) User control and freedom,
(4) Consistency and standards, (5) Error prevention, (6) Recognition rather than
recall, (7) Flexibility and efficiency of use, (8) Aesthetic and minimalist design,
(9) Help users recognize, diagnose, and recover from errors, (10) Help and
documentation.

**Procedure.** Not run standalone — this is the rubric consumed by heuristic
evaluation (§1.2) and expert review (§1.5). To use it directly on a small surface:
read all ten before starting, hold each in mind while walking the product, and
write a specific violation ("no heuristic applies generically") wherever the
product doesn't speak the user's language, hides system status, blocks an exit,
etc.

**Evidence produced.** A list of concrete violations, each tagged to the specific
heuristic it breaks and the specific screen/command/output where it occurs.

**Source.** [Jakob Nielsen, "10 Usability Heuristics for User Interface Design," NN/g](https://www.nngroup.com/articles/ten-usability-heuristics/) — the original, canonical statement of the heuristics, maintained and periodically re-worded by the firm that originated them; the single most-cited rubric in usability inspection.

### 1.2 Heuristic Evaluation — the group procedure

**What it is.** A structured method for finding usability problems by having
multiple independent evaluators judge an interface against Nielsen's 10 heuristics,
then pool findings.

**Step-by-step procedure** (verified against the NN/g how-to):

1. **Staff it correctly.** Use 3–5 evaluators working *independently* — a single
   evaluator misses most problems; returns diminish sharply past 5.
2. **Prepare.** First-timers read and internalize the 10 heuristics; run a practice
   round on a simple design if this is the team's first evaluation.
3. **Narrow the scope.** Pick one task, one section, one user group, and one device
   at a time — narrower scope produces a more detailed, more useful evaluation.
4. **Evaluate individually, timeboxed.** Each evaluator does two passes over the
   scoped surface: first, walk it as a user completing the task to get familiar;
   second, walk it again hunting specifically for heuristic violations, writing
   down each with a note on which heuristic it breaks. Budget roughly 1–2 hours per
   evaluator.
5. **Debrief as a group.** Bring evaluators, any observer, and the design/build team
   together in a brainstorming-mode session; consolidate the independent lists
   (affinity-diagram style), discuss agreement/disagreement, business impact, and
   candidate fixes.
6. **Rate severity afterward, separately from discovery** (see §1.3) — don't mix
   severity-rating into the hunting pass, it changes what evaluators notice.

**Evidence produced.** A consolidated, de-duplicated list of usability problems,
each mapped to a heuristic, a location in the product, and (after step 6) a
severity score — directly prioritizable into a fix backlog.

**Source.** [NN/g, "How to Conduct a Heuristic Evaluation"](https://www.nngroup.com/articles/how-to-conduct-a-heuristic-evaluation/) — the firm's operational how-to, distinct from the heuristics list itself; it is the actual runbook, including team size, timeboxing, and debrief structure, that a team follows to execute an evaluation session.

### 1.3 Severity Rating — turning findings into a prioritized backlog

**What it is.** Nielsen's method for scoring how serious each usability problem is,
so a team can decide what must ship-block versus what can wait.

**Procedure.** Score every problem 0–4: **0** not a real problem; **1** cosmetic,
fix only if time allows; **2** minor, low priority; **3** major, high priority;
**4** catastrophe, must fix before release. Severity is a composite of three
factors — **frequency** (common vs. rare), **impact** (easy vs. hard for the user
to work around), and **persistence** (one-time vs. recurring) — plus, informally,
market/brand impact. Nielsen's specific procedural recommendation: don't rate
severity *during* the hunting pass; send evaluators a written questionnaire with
the *full, consolidated* list of found problems afterward, and average the ratings
across evaluators (mean of 3 raters is typically reliable enough for practical use).

**Evidence produced.** A severity-scored, sortable defect list — the artifact a PM
actually uses to decide "ship, fix-then-ship, or don't ship."

**Source.** [Jakob Nielsen, "Severity Ratings for Usability Problems," NN/g](https://www.nngroup.com/articles/how-to-rate-the-severity-of-usability-problems/) — the primary definition of the 0–4 scale and the three-factor severity model that nearly all downstream "usability audit" templates borrow from.

### 1.4 Cognitive Walkthrough — task-based, learnability-focused

**What it is.** A task-based inspection method that evaluates *learnability* — can
a first-time user, with no help, complete a specific task by recognizing the right
action at each step? Unlike heuristic evaluation (broad, principle-based), a
cognitive walkthrough is narrow and task-specific, and unlike usability testing, it
uses no real users — a cross-functional team plays the role of the user.

**Step-by-step procedure:**

1. **Assemble a small cross-functional team** (2–6 people): product/UX, an
   engineer, and ideally a domain expert or someone who has seen real user
   behavior.
2. **Define inputs before the session:** a specific user persona, a specific task,
   and the correct action sequence to complete it (including any valid alternate
   paths).
3. **Assign roles:** a facilitator (keeps discussion on track), a presenter (drives
   the interface as a proxy user), and a recorder (logs the group's determination
   at each step).
4. **Walk the task step by step.** At every step in the action sequence, the group
   answers four fixed questions: (1) Will the user *try* to achieve the right
   outcome at this point? (2) Will the user *notice* that the correct action is
   available? (3) Will the user *associate* the correct action with the outcome
   they're trying to achieve? (4) *After* taking the action, will the user see that
   they made progress toward their goal? A single "no" fails the step.
5. **Record and move on**, capturing a written rationale for every fail so it can
   be handed to design/engineering without re-litigating the walkthrough.

**Evidence produced.** A step-by-step pass/fail table over one task's action
sequence, each failure annotated with *which* of the four questions failed and why
— pinpoints exactly where a first-time user would get stuck, before any user is
ever recruited.

**Sources.** [NN/g, "Evaluate Interface Learnability with Cognitive Walkthroughs"](https://www.nngroup.com/articles/cognitive-walkthroughs/) — defines the method and when to use it over usability testing; [NN/g, "How to Conduct a Cognitive Walkthrough Workshop"](https://www.nngroup.com/articles/cognitive-walkthrough-workshop/) — the operational runbook with roles and the exact four questions, needed because the definitional article alone doesn't give the workshop mechanics.

### 1.5 UX Expert Review — the outside-eyes audit

**What it is.** A design review in which a UX expert (deliberately uninvolved in
the original design, for a fresh perspective) inspects a product for usability
issues. It's broader than heuristic evaluation: an expert reviewer draws not just
on the 10 heuristics but on the wider field (cognitive psychology, HCI research,
and their own accumulated experience of what breaks in practice).

**Procedure.** Bring in a reviewer with no stake in the original design decisions
and deep usability-research grounding (the source is explicit that expertise comes
from "doing UX research and getting substantial exposure to real user behavior" —
someone who has never watched real users struggle isn't yet an expert reviewer, no
matter how much design theory they know). The reviewer inspects the product
end-to-end against best practices and heuristics, and produces a written report:
usability strengths, a list of problems each mapped to a specific location, a
severity rating per problem (NN/g uses a simple High/Medium/Low scale for this
format), and concrete recommendations, often illustrated with best-practice
examples from comparable products.

**Evidence produced.** A ranked, written expert-review report — the deliverable a
PM can hand directly to design/eng as a fix backlog, or use as an independent
sanity check before a heuristic evaluation or usability test.

**Source.** [NN/g, "UX Expert Reviews"](https://www.nngroup.com/articles/ux-expert-reviews/) — defines what distinguishes an expert review from a heuristic evaluation (expertise and outside judgment vs. a fixed 10-item checklist) and specifies the qualifications a genuine expert reviewer needs.

### 1.6 Lightweight, Do-It-Yourself Usability Testing

**What it is.** Two closely related "you don't need a lab" findings from the
usability field, useful when even a heuristic evaluation feels like too much
process.

**Nielsen's 5-user rule.** Based on the model `N × (1 − L)ⁿ` (N = total problems,
L ≈ 31% = the typical fraction of problems one user surfaces), testing beyond ~5
users on a homogeneous user group yields sharply diminishing returns; Nielsen's
recommendation is not "test 5 users once" but "run 3 rounds of 5 users each,"
redesigning between rounds — iteration beats sample size. Caveats that matter for
an audit: this rule is for *qualitative* discovery, not for quantitative
benchmarking (which needs ~20 users per NN/g guidance), and it assumes one
reasonably homogeneous user population — a product with 2–3 distinct user segments
needs 3–4 users *per segment*.

**Krug's "one morning a month" method.** Steve Krug's operational version:
recruit just 3 participants, run the whole test in a single morning, and get as
many teammates as possible to observe live. Over lunch, the observing team debriefs
and picks — ruthlessly — the worst 3 problems seen, in severity order, working down
the list until the available fix budget for that month is exhausted, then commits
to actually fixing them.

**Evidence produced.** A small, fast, repeatable cadence of real-user observations
converted directly into a monthly fix commitment — the artifact is the team's own
written "top 3, this month" list plus session notes.

**Sources.** [Jakob Nielsen, "Why You Only Need to Test with 5 Users," NN/g](https://www.nngroup.com/articles/why-you-only-need-to-test-with-5-users/) — the original statistical justification and its explicit caveats (quantitative studies and heterogeneous populations need more); [Oliver Lindberg, "Interview with Steve Krug: how to get DIY usability testing right," The Lindberg Interviews (Medium)](https://medium.com/the-lindberg-interviews/interview-with-steve-krug-how-to-get-diy-usability-testing-right-63dedddbd0ae) — a direct interview with Krug (author of *Don't Make Me Think* and *Rocket Surgery Made Easy*) giving the exact operational cadence in his own words, which his book pages summarize but this interview states most concretely.

---

## 2. Product Teardown Methodology

**What it is.** A systematic critique of a *finished, shipping* product — your own
or a competitor's — done to build product judgment or to extract concrete lessons,
distinct from the pre-build validation methods in §3. A teardown treats the product
as evidence: what decisions did the team make, what tradeoffs do those decisions
imply, and did they pay off.

**Step-by-step procedure:**

1. **State the goal first.** A teardown without a question ("why does this
   onboarding convert so well?" vs. "should we clone this pricing model?") produces
   an unfocused report. The goal determines scope.
2. **Choose the product deliberately.** Direct competitors (same problem, same
   audience) teach the most transferable lessons; adjacent products (different
   category, comparable mechanic — e.g., a CLI team studying a different CLI's
   flag ergonomics) are useful for inspiration precisely because they're not
   in a head-to-head comparison.
3. **Scope the analysis.** Pick one of: feature-level (a specific capability),
   workflow-level (an end-to-end user journey), or design-level (UI/interaction
   choices) — trying to cover all three at once produces a shallow report.
4. **Use it, don't just read about it.** Run the product first-hand and record
   first impressions, friction points, and standout moments *before* consulting
   external sources; then supplement with app-store/marketplace reviews, forums,
   and — if feasible — direct user interviews, so the teardown isn't purely the
   auditor's own opinion.
5. **Analyze across four dimensions:** functionality (core features, what's
   surprisingly present or absent), usability (friction, ease of reaching the
   goal), technical implementation (what the architecture implies about
   scalability and constraints — inspectable via dev tools for web products,
   `--help`/error output/exit codes for CLIs), and business alignment (how design
   choices serve acquisition, retention, or monetization).
6. **Document systematically**, not as a stream-of-consciousness — structure
   findings so screenshots/transcripts/session recordings back every claim, using
   whatever collaborative tool fits (a shared doc, a spreadsheet of findings tagged
   by dimension and severity).
7. **Look for patterns across the raw findings**, then convert patterns into
   metrics-linked, actionable recommendations rather than a list of opinions.

**Evidence produced.** A structured teardown report: goal statement, scope, dated
walkthrough notes (first-person, "I did X, saw Y"), findings tagged by dimension,
and a short list of transferable recommendations tied to what metric or outcome
each would move.

**A complementary, *pre-build* instrument — Amazon's PR/FAQ.** Where a teardown
critiques a product that already exists, Amazon's "Working Backwards" process is
the mirror-image discipline applied *before* building: write a press release (under
one page) stating the customer experience as if the product already shipped, plus
an FAQ (five pages or less) covering internal and customer questions and a
clear-eyed cost/complexity assessment — then iterate the document itself, often
through ten or more drafts and five or more leadership reviews, *before* committing
engineering resources. Amazon states plainly that most PR/FAQs are never approved,
and treats that as the mechanism working correctly: "spending time up front... to
determine — without committing precious software development resources — which
products *not* to build" is the entire point. A team auditing its own roadmap can
borrow this instrument directly: write the PR/FAQ for a proposed feature and see
whether it survives its own FAQ section.

**Evidence produced (PR/FAQ variant).** A short written artifact (PR + FAQ) that
either survives internal scrutiny (proceed) or reveals its own weak assumptions
before code is written (kill or rescope) — the paper trail is the artifact.

**Sources.** [LogRocket, "What is a product teardown? Process, tools, and other insights"](https://blog.logrocket.com/product-management/product-teardown-process-tools/) — a practitioner-oriented, step-by-step teardown runbook from a developer-tooling company blog; cited and labeled as a *practitioner secondary source* since product teardown, unlike heuristic evaluation, has no single academic origin — it's craft knowledge, and this is among the more structured public write-ups of it. [About Amazon, "An insider look at Amazon's culture and processes"](https://www.aboutamazon.com/news/workplace/an-insider-look-at-amazons-culture-and-processes) — Amazon's own official account of the Working Backwards/PR-FAQ process (the process is documented at length in Bryar & Carr's book *Working Backwards*; this is the company's own public, primary-source description of the same mechanism).

**[SYNTHESIS]** In practice the strongest teardowns triangulate §1 (heuristic /
expert-review lens), §3 (does the product resolve Cagan's four risks?), and a
JTBD framing (§7.2: what job was this built to do, and does it do it) rather than
inventing a fifth ad hoc rubric — the "four dimensions" in step 5 above map almost
exactly onto usability (§1), business viability (§3.1), feasibility signals
(technical implementation), and value (functionality/JTBD fit).

---

## 3. Discovery & Assumption Validation

Where §1 audits an *existing* interface and §2 audits an *existing* product,
discovery methods audit an *idea* before it's built — testing whether the
underlying assumptions justify the investment.

### 3.1 Cagan's Four Big Risks

**What it is.** Marty Cagan's framework (from *Inspired*, restated and refined in
this article) for the risks that must be retired *before* a team commits
engineering resources to building something. Cagan's earlier three-attribute
framing ("valuable, usable, feasible") under-weighted business risk, so the current
framing explicitly splits customer value from business viability:

> "**value** risk (whether customers will buy it or users will choose to use it);
> **usability** risk (whether users can figure out how to use it); **feasibility**
> risk (whether our engineers can build what we need with the time, skills and
> technology we have); **business viability** risk (whether this solution also
> works for the various aspects of our business)" — including go-to-market fit,
> legal/compliance constraints, cost-effective acquisition, monetization, and
> brand consistency.

**Procedure / ownership.** Discovery exists specifically to retire all four risks
*before* delivery starts, and Cagan assigns clear ownership so no risk is silently
skipped: the **Product Manager** owns value and viability risk and is accountable
for product *outcomes*; the **Product Designer** owns usability risk and is
accountable for the *experience*; the **Product Lead Engineer** owns feasibility
risk and is accountable for *delivery*. To audit an existing or proposed product
against this framework: for each of the four risks, write down the specific
evidence (not opinion) that retires it — a value-risk claim needs a validated
assumption test (§3.2), not a stakeholder's confidence; a feasibility-risk claim
needs a spike or prototype, not an estimate. Any risk without cited evidence is
still open, regardless of how far along the build is.

**Evidence produced.** A four-row risk ledger (value / usability / feasibility /
viability), each row either backed by a specific evidence artifact or explicitly
flagged open — directly usable as a go/no-go gate before or after build.

**Source.** [Marty Cagan, "The Four Big Risks," Silicon Valley Product Group](https://www.svpg.com/four-big-risks/) — Cagan is the field's most cited authority on product discovery risk (SVPG has advised or trained product teams at hundreds of top tech companies); this is his own primary restatement of the framework, including the ownership model, verified by direct fetch of the full article text.

### 3.2 Torres' Assumption Mapping & Testing

**What it is.** Teresa Torres' method (from *Continuous Discovery Habits* and
Product Talk) for deconstructing a product idea into its underlying assumptions —
desirability, usability, feasibility, viability, and ethical — and testing the
riskiest ones cheaply, *before* building.

**Step-by-step procedure:**

1. **Generate assumptions from a story map or opportunity solution tree.** Walk
   each step of the user's journey (or each inferential link from opportunity to
   solution to outcome) and ask "what needs to be true for this step to work?" —
   each answer is an assumption. Desirability assumptions concern *why* customers
   would want this and do what's needed to get value from it; viability
   assumptions concern why it's good for the business; usability and feasibility
   mirror Cagan's terms; ethical assumptions concern potential harm.
2. **Phrase assumptions correctly.** State them as conditions that must hold true
   for the idea to succeed (not as predictions of what customers *won't* do), and
   tie each to a specific story-map step so it stays small and testable.
3. **Prioritize with an assumptions map.** Plot every assumption on a 2×2: evidence
   (weak → strong) on one axis, importance to success on the other. The riskiest
   assumptions — the ones to test first — sit in the high-importance,
   weak-evidence quadrant. (Torres draws this technique from David Bland's
   assumption-mapping work.)
4. **Test with the cheapest instrument that produces real evidence**, in ascending
   order of cost: **prototype tests** (simulate the moment of decision and observe
   behavior), **one-question surveys** (fast read on past/current customer
   behavior), **data mining** (existing analytics, support tickets, sales-call
   transcripts), and **research spikes** (a bounded engineering investigation for
   feasibility risk specifically). These substitute for full-scale experiments
   during discovery, enabling fast iteration before anything is built.

**Evidence produced.** An assumptions map (2×2 evidence-vs-importance) plus, for
each tested assumption, a small, dated test artifact (survey result, prototype
session notes, data-mining query + result, spike write-up) showing what was
learned and whether the assumption survived.

**Source.** [Teresa Torres, "Assumption Testing: Everything You Need to Know to Get Started," Product Talk](https://www.producttalk.org/assumption-testing/) — Torres is the most cited living authority on continuous, weekly-cadence product discovery (via *Continuous Discovery Habits* and Product Talk); this article is her direct operational statement of the assumption-generation → mapping → testing sequence, including the specific four test types.

---

## 4. Metrics & Funnel Audits

Heuristic and discovery methods find defects by inspection or by testing
assumptions before scale. Metrics audits find defects *in production*, at scale,
using what users actually do.

### 4.1 HEART — the framework for choosing what to measure

**What it is.** A framework from Google's UX research team — Kerry Rodden, Hilary
Hutchinson, and Xin Fu, published at CHI 2010 — for turning fuzzy UX goals into
concrete large-scale metrics. HEART = **H**appiness (satisfaction, perceived
ease/usefulness), **E**ngagement (frequency, depth, intensity of use),
**A**doption (new users starting to use a product/feature), **R**etention (whether
users keep coming back over time), and **T**ask success (efficiency, effectiveness,
error rate on specific tasks).

**Procedure — the Goals-Signals-Metrics (GSM) process.** For each category
relevant to the audit's scope: (1) write the **goal** in plain language (what
outcome matters here — e.g., "new users successfully complete onboarding");
(2) identify the **signals** that would indicate progress toward that goal
(concrete, observable user actions or stated feedback); (3) pick the **metrics**
that operationalize those signals into something trackable over time. Not every
category applies to every project — a one-time task tool may care about Task
Success and Happiness but not Adoption; a subscription product cares heavily about
Retention.

**Evidence produced.** A GSM table (Goal → Signal → Metric) per HEART category
relevant to the product, giving an auditor a defensible, non-arbitrary set of
metrics to actually pull and inspect — rather than whatever dashboard numbers
happen to exist already.

**Source.** [Kerry Rodden, Hilary Hutchinson, and Xin Fu, "Measuring the User Experience on a Large Scale: User-Centered Metrics for Web Applications," CHI 2010 (Google Research)](https://research.google/pubs/measuring-the-user-experience-on-a-large-scale-user-centered-metrics-for-web-applications/) — the original peer-reviewed paper (ACM CHI proceedings) that introduced HEART and the GSM process; still, over 15 years later, the most-cited framework for translating UX goals into measurable metrics.

### 4.2 North Star Metric + Input Metrics

**What it is.** Amplitude's framework for a single metric that captures the value
a product delivers to customers, correlated with durable business success, plus a
small set of "input" metrics a team can actually move that ladder up to it.

**Criteria for a good North Star.** Three tests: (1) it must reflect *realized
customer value* — DAU and similar vanity metrics fail this because they don't
distinguish real value from raw activity; (2) reading the metric should reveal the
company's *strategy* — ideally it's differentiated, not a generic count; (3) it
must be a *leading* indicator that predicts future outcomes, not a lagging one
(like monthly revenue) that only reports the past.

**Step-by-step identification procedure:**

1. Determine the business model archetype the product plays: an "attention game"
   (value = time spent), a "transaction game" (value = purchase/usage frequency),
   or a "productivity game" (value = work accomplished efficiently).
2. Research which specific user actions actually correlate with retention,
   satisfaction, and revenue — don't assume; check the data.
3. Trace how that realized value connects to revenue/business outcomes.
4. Validate the candidate metric is *predictive*, not just descriptive, by
   checking whether early movement in it forecasts later business results.
5. Define 3–5 **input metrics** — the concrete, team-controllable levers that
   together produce movement in the North Star. A useful pattern for choosing
   inputs is breadth × depth × frequency × efficiency.

**Evidence produced.** A one-metric North Star statement plus a linked table of
3–5 input metrics, each owned by a specific team/lever — the artifact that lets a
PM audit "are we even measuring the right thing" before auditing whether the
number looks good.

**Source.** [Amplitude, "Every Product Needs a North Star Metric: Here's How to Find Yours"](https://amplitude.com/blog/product-north-star-metric) — Amplitude popularized and operationalized the North Star framework industry-wide via this article and its companion North Star Playbook; verified by direct fetch, including the three criteria and the four-step identification process.

### 4.3 Funnel, Activation, and Retention Audits

**What it is.** The practice of tracing a cohort of users through a defined
sequence of steps (signup → setup → "aha" moment → habitual use) to find where and
why they drop off, and separately, of tracking what fraction of each cohort stays
active over time to see whether the product creates durable value.

**Procedure:**

1. **Define the funnel steps explicitly** — the concrete, observable actions that
   make up "activation" for this product (not "signed up," but the specific
   action(s) that correlate with the user reaching first real value).
2. **Plot cohort retention curves**: percent of a signup cohort still active at
   day N (or month N for longer-cycle products), for multiple cohorts over time,
   to see whether the shape is stable or improving release-over-release
   (see §5.3 for what the *shape* of this curve tells you about product-market
   fit specifically).
3. **Find the steepest drop-off** in the funnel and treat it as the highest-value
   audit target — for most products the steepest drop is at activation, not later
   in the funnel, so activation deserves disproportionate audit attention.
4. **Diagnose each drop-off qualitatively**, not just quantitatively: is this a
   *UX* problem (the user wanted to continue but got stuck — audit with §1's
   methods) or a *value* problem (the user understood the product and it wasn't
   for them — audit with §3's assumption tests)? Conflating these two diagnoses is
   the most common funnel-audit mistake.
5. **Segment retention by acquisition source, signup cohort, and (if available)
   demographic/usage variables** to find which segment actually retains — the
   aggregate curve often hides a well-retained core segment diluted by a
   poorly-fit majority (this step overlaps directly with §5.3).

**Evidence produced.** A funnel diagram annotated with conversion rate at each
step, a retention-curve chart per cohort/segment, and a written diagnosis (UX vs.
value) at the largest drop-off — the artifact a growth/PM team uses to decide
whether to fix the interface or rethink the offering.

**Leading vs. lagging.** Input metrics (§4.2) and funnel-step conversion rates are
*leading* indicators — they move first and predict the outcome. Revenue, LTV, and
aggregate retention at a long horizon are *lagging* — they confirm what already
happened. An audit that only inspects lagging metrics can't tell a PM what to do
next; it can only tell them whether the last quarter's bets paid off.

**Sourcing note.** This subsection synthesizes the mechanics of funnel/retention
tracing described in the Amplitude and Balfour sources already cited (§4.2, §5.3);
Reforge's dedicated funnel-analysis and activation guides describe the same
practice in more template-driven detail but sit behind Reforge's paid-membership
gate (confirmed: direct fetch returned HTTP 403), so they are not cited as a
verifiable deep link here per this document's sourcing discipline. **[SYNTHESIS]**

---

## 5. Product-Market Fit Validation Mechanics

Where §4 audits *whether the product is being measured correctly*, this section
covers the specific, well-known mechanics for measuring whether the product has
reached product-market fit at all.

### 5.1 The Sean Ellis 40% "Very Disappointed" Test

**What it is.** A single-question survey Sean Ellis used across hundreds of
startups to benchmark PMF: **"How would you feel if you could no longer use
[Product]?"** with four response options — *Very disappointed*, *Somewhat
disappointed*, *Not disappointed*, *N/A — I no longer use [Product]*. Ellis found
that above a **40%** "very disappointed" response rate, companies could generally
grow more easily, showed strong word-of-mouth, and their problems shifted to
scaling (a "good problem" to have); below 40%, growth was a persistent struggle.

**Procedure:**

1. **Survey only real, recent users** — people who have used the core product at
   least twice in the past two weeks (or the product-appropriate equivalent), not
   everyone who signed up. Ellis's own example: for Uber, survey people who
   actually took a ride, not everyone who downloaded the app.
2. **Get a large enough sample to trust the number** — minimum ~30 responses for
   directional signal, 40–100+ for a number worth acting on; below ~40, a handful
   of outliers can swing the score by 10+ points.
3. **Calculate**: % who answered "very disappointed" ÷ total valid responses.
   ≥40% is Ellis's PMF benchmark.
4. **Mine the rest of the survey**, not just the headline number — follow-up
   questions about *why* respondents would be disappointed and *who else* would
   benefit reveal what to double down on (this is exactly what Vohra's engine in
   §5.2 does next).

**Evidence produced.** A single percentage (the "PMF score") plus a segmented
breakdown of *why* the "very disappointed" segment feels that way — the artifact
used to decide whether to scale acquisition or keep iterating on the core product.

**Sources.** [Sean Ellis, "Using Product/Market Fit to Drive Sustainable Growth," GrowthHackers (Medium)](https://medium.com/growthhackers/using-product-market-fit-to-drive-sustainable-growth-58e9124ee8db) — Ellis's own direct statement of the survey question, response options, 40% threshold, and sample-size guidance, verified by direct fetch; [Sean Ellis & GoPractice, PMF Survey tool](https://pmfsurvey.com/) — the live, official survey instrument Ellis co-maintains today (his original Survey.io tool was discontinued when its parent business, Qualaroo, was sold), cited as the current canonical home of the actual tool, distinct from the methodology article.

### 5.2 Rahul Vohra's Superhuman PMF Engine

**What it is.** Superhuman CEO Rahul Vohra's method for turning Ellis's single
survey score into a repeatable, quantitative product-development *engine* rather
than a one-time measurement — published as one of First Round Review's most-read
posts.

**Step-by-step procedure (as Vohra describes it):**

1. **Measure**, using Ellis's exact survey, restricted to users active at least
   twice in the past two weeks. Superhuman's starting score: 22%.
2. **Segment your supporters, not your whole user base.** Group respondents by
   satisfaction level, then build personas for the "very disappointed" segment
   specifically. Vohra found this segment concentrated among founders, managers,
   executives, and business-development professionals — narrowing the addressable
   audience *raised* the measured score to 32% (surveying only people the product
   was actually built for). He crystallized this into a single "high-expectation
   customer" persona.
3. **Analyze feedback strategically.** Ask "very disappointed" users what they'd
   miss most (for Superhuman: speed, keyboard shortcuts) — that's what to protect
   and amplify. Ask "somewhat disappointed" users what's missing that would move
   them to "very" (for Superhuman: mobile app, integrations, calendaring, search,
   read receipts) — that's the gap-closing roadmap.
4. **Build a 50/50 roadmap**: half the roadmap doubles down on what the core
   segment already loves; half closes the gaps the "somewhat" segment named.
   Prioritize within each half by cost vs. impact (low-cost/high-impact first).
5. **Re-measure continuously** — weekly, monthly, and quarterly, on fresh cohorts
   — treating the score as a live metric, not a one-off study. Superhuman's score
   rose from 22% to 58% over three quarters using this loop.

**Evidence produced.** A tracked PMF-score time series, a named high-expectation
customer persona backed by real segment data, and a roadmap explicitly split and
labeled by which half of the survey evidence justifies each item — an unusually
concrete, auditable link from "user survey" to "what we're building next."

**Source.** [Rahul Vohra, "How Superhuman Built an Engine to Find Product/Market Fit," First Round Review](https://review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/) — Vohra's own detailed account, one of First Round Review's most-cited posts industry-wide; verified by direct fetch for the exact four-step sequence and the 22%→32%→58% score progression.

### 5.3 Retention-Curve Flattening (Balfour) — the behavioral cross-check

**What it is.** Brian Balfour's argument that a *survey-based* PMF signal (§5.1)
should be cross-checked against *actual behavior*: plot the percentage of each
signup cohort still active over time; if the curve **flattens** rather than
continuing to decay toward zero, that flat tail is a segment of users deriving
real, durable value — direct behavioral evidence of PMF for that segment,
independent of what anyone said in a survey.

**Procedure:**

1. **Plot % active over time** for multiple signup cohorts (days for B2C-style
   usage cadence, months for longer B2B/SaaS cycles).
2. **Look for a flattening point.** If the curve keeps decaying toward zero, PMF
   hasn't been found yet for any coherent segment; if it flattens at some non-zero
   floor, that floor is the retained core.
3. **Segment the curve** by acquisition source, signup timing/cohort, and
   demographic/firmographic variables to find *which* population produces the
   flattening — the aggregate curve often understates a strong sub-segment
   diluted by a weak majority.
4. **Follow up qualitatively**: survey/interview the retained segment vs. the
   churned segment specifically to articulate what differentiates them (this
   connects directly back to Vohra's segment-first approach in §5.2).

**Why this matters as a cross-check, in Balfour's own framing**: survey
methods measure stated intent ("what people say they would do"); retention curves
measure revealed behavior ("what people are doing"). Balfour states a explicit
preference for the latter as a truer signal, at the cost of requiring more time
and data to observe.

**Evidence produced.** A cohort retention-curve chart with a visibly flat (or
still-decaying) tail, segmented to identify the specific population the flat tail
represents — the artifact that either corroborates or contradicts a survey-based
PMF score.

**Source.** [Brian Balfour, "The Never Ending Road To Product Market Fit"](https://brianbalfour.com/essays/product-market-fit) — Balfour (former Reforge founder, ex-HubSpot VP Growth) is one of the field's most-cited voices specifically on retention-based growth measurement; verified by direct fetch for the plotting method, the flattening criterion, and the explicit survey-vs-behavior contrast.

---

## 6. Docs, Onboarding & Content Audit

A product can pass every heuristic evaluation and still fail because nobody can
figure out *that* a capability exists or *how* to invoke it — this is the audit
surface for documentation, first-run experience, and wayfinding.

### 6.1 Information Scent — auditing whether content leads users to the right place

**What it is.** A concept from information-foraging theory: users decide whether
to follow a link/label/menu item based on an *estimate* of how likely it is to
satisfy their need and how long that will take — the "scent" a piece of UI text
gives off before it's clicked. Weak or misleading scent is why users get lost in
documentation or navigation even when the right content exists somewhere in the
product.

**Procedure — auditing scent on docs/navigation:**

1. **Check link/heading text in isolation**: is it clear and self-explanatory, or
   does it require surrounding context to mean anything ("Learn More," "Advanced
   Options")? Vague labels leak users at exactly the decision point that matters.
2. **Check the text/imagery accompanying each link**: does it add real
   information beyond the label itself, or is it decorative?
3. **Check surrounding context**: does the page/section the link sits in reinforce
   what's on the other end, so the user's estimate is accurate before they click?
4. **Check the landing side**: when a user does click, does the destination confirm
   quickly that they're "on track," or does it force them to re-orient?
5. **Watch for false-strong scent** — a label that promises more than the
   destination delivers erodes trust in *all* future navigation cues in the
   product, not just that one link (NN/g documents this as a measurable
   "deceivingly strong scent" cost).

**Evidence produced.** A link/heading-by-heading scent audit table: label text,
scent strength (does it accurately predict the destination), and specific rewrite
recommendations where scent is weak or misleading.

**Source.** [NN/g, "Information Scent: How Users Decide Where to Go Next"](https://www.nngroup.com/articles/information-scent/) — the canonical statement of information-foraging theory applied to UX, including the concrete auditable dimensions (link text, accompanying content, context, brand trust) verified by direct fetch.

### 6.2 The Divio / Diátaxis Documentation System — the structural rubric for a docs audit

**What it is.** A framework (originating at Divio, now maintained as "Diátaxis")
asserting that good documentation must serve four distinct, mutually exclusive
purposes, each requiring a different writing approach: **tutorials** (a guided,
guaranteed-to-work learning path for a newcomer — "if you follow these steps
exactly, you will get a working result"), **how-to guides** (recipe-style, answer
one specific question, assume some existing competence), **reference** (complete,
neutral, description-only — no instruction, no explanation, just accurate facts
about what exists), and **explanation** (the "why," conceptual background —
deliberately *not* instructional or descriptive).

**Procedure — auditing docs against this rubric:**

1. **Inventory existing docs and tag each page/section by which of the four types
   it's actually trying to be.**
2. **Flag mixed-purpose pages** — the framework's core claim is that content
   mixing types (e.g., a "getting started" page that's half tutorial, half
   reference, half conceptual aside) actively harms usability for all four
   audiences at once, because a first-time learner and a returning expert doing a
   lookup need incompatible things from the same page.
3. **Check coverage**: does each of the four types exist at all for the product's
   core surface, or is (commonly) reference present while tutorials and
   explanation are missing?
4. **Check the tutorial's guarantee specifically** — Divio's own strongest claim
   is that a tutorial must be end-to-end reliable; a tutorial that fails partway
   through for a newcomer following it exactly is worse than no tutorial, because
   it burns the user's trust at their most vulnerable (lowest-competence) moment.

**Evidence produced.** A docs inventory tagged by type, a gap list (missing
types), and a mixed-purpose-page list — a structural audit that precedes any
line-level copy-editing pass.

**Source.** [Divio, "About the Documentation System" ("Diátaxis")](https://docs.divio.com/documentation-system/) — the originating articulation of the four-part framework, now widely adopted across large open-source and commercial documentation sets, verified by direct search-summary confirmation of the four categories and their defining distinctions.

### 6.3 Auditing the First-Run / Onboarding Experience

**[SYNTHESIS]** — this subsection combines the "5-second test" mechanics NN/g
documents for first-impressions testing with the cognitive-walkthrough procedure
from §1.4, applied specifically to a product's first-run path, since no single
source treats onboarding audit as a standalone named method distinct from these
two.

**Procedure:**

1. **Run a first-impressions check.** Show the product's landing/first screen (or
   a CLI's first `--help` output, first error message, or install-completion
   message) to someone unfamiliar with it for ~5 seconds, then ask what they
   think the product does and who it's for — a lightweight instrument for catching
   a first-run experience that doesn't communicate its own purpose.
2. **Run a cognitive walkthrough (§1.4) specifically on the "zero to first value"
   task** — the single most important task-flow to walkthrough-audit in any
   product, since it's the one every single user must pass and the one most
   likely to be evaluated by someone with zero accumulated context.
3. **Check information scent (§6.1) at every decision point in onboarding**
   specifically — onboarding is disproportionately made of exactly the
   label/link/prompt decision points information-scent auditing targets.

**Evidence produced.** A first-impressions read-out, a cognitive-walkthrough
pass/fail table scoped to the onboarding task, and a scent audit of the onboarding
path's labels/prompts — three complementary artifacts that together diagnose most
onboarding failure modes.

---

## 7. Dogfooding & Manual Validation Walkthroughs

Every method above inspects the product from the outside. Dogfooding and
task-based walkthroughs validate it from the inside — by actually using it to do
real work, under real constraints, and recording what happened.

### 7.1 Dogfooding — structured self-use as validation

**What it is.** The practice of an organization using its own product for real
internal work, as a form of continuous, high-fidelity quality control. The term
traces to a 1988 internal Microsoft email from manager Paul Maritz to Brian
Valentine (test manager for Microsoft LAN Manager), titled "Eating our own
Dogfood," challenging the team to increase real internal usage of their own
product; the practice — and the internal server literally named `\\dogfood` that
followed — spread from there. Its most cited proof point: Dave Cutler's 1991
insistence that the 200+-engineer Windows NT team run their own daily builds as
their actual working OS, which surfaced regressions the moment they broke real,
everyday engineering work rather than waiting for a formal test pass to catch them.

**Procedure — running a structured dogfooding pass:**

1. **Make it mandatory for real work, not a side exercise.** The Microsoft
   precedent's power came specifically from developers running daily builds *as
   their actual working environment* — a dogfood pass that's a scripted demo
   rather than genuine dependency catches far less.
2. **Pick a task that has real stakes for the dogfooder** — a task they'd
   otherwise do a different way, so friction is felt, not simulated.
3. **Record failures as they happen, in the moment**, not from memory afterward —
   the value of dogfooding over a lab test is exactly that it surfaces the
   failures a scripted test scenario wouldn't think to script.
4. **Feed findings back into the same fix loop as any other inspection method**
   (severity-rate per §1.3, prioritize per §1.2's debrief structure).

**Evidence produced.** A log of genuine task attempts (not scripted scenarios),
each annotated with what broke, what was confusing, and what workaround the
dogfooder improvised — improvised workarounds are themselves signal: they mark
exactly where the product's designed path and the real task diverged.

**Source.** [Wikipedia, "Eating your own dog food"](https://en.wikipedia.org/wiki/Eating_your_own_dog_food) — a well-cited historical account (drawing on contemporaneous tech-press and Microsoft-internal sourcing) of the term's 1988 Microsoft origin and the Windows NT dogfooding precedent, used here because it consolidates and cross-references the origin story more completely, and with more verifiable specificity (names, dates, the NT case), than any single practitioner blog post found during this research.

### 7.2 Jobs-to-be-Done Switch Interviews — task-based validation grounded in real decisions

**What it is.** Clayton Christensen's framework that people don't buy products,
they "hire" them to make progress on a specific job — and Bob Moesta's "switch
interview" technique for reconstructing, from a real customer, the actual causal
story of why they switched from an old solution (including "doing nothing") to a
new one.

**Core definition, in Christensen's own framing**: a job-to-be-done is "the
progress a person is trying to make in a particular circumstance" — jobs are
*causal* (they explain why someone buys), where demographics are merely
correlated. Christensen invokes Ted Levitt's line that "the customer is rarely
buying what the company thinks it's selling" — people don't want a quarter-inch
drill, they want a quarter-inch hole.

**Step-by-step switch interview procedure:**

1. **Reconstruct the timeline in five phases**: first thought (when the need was
   first recognized), passive looking (aware but not actively searching), active
   looking (deliberately searching for solutions), the decision (the specific
   moment of choosing), first use (the initial experience with what they chose).
2. **Work backward from the purchase/adoption moment**: "When did you buy/start
   using this? Walk me through that day," then progressively earlier: "When did
   you first start thinking about this? What was going on in your life?"
3. **Code the narrative for the four forces of progress**: **push** (what was
   wrong with the status quo that created pressure to look), **pull** (what
   attracted them to the new solution specifically), **anxiety** (fears/doubts
   about the new, unproven choice), and **habit** (comfort/inertia keeping them on
   the old solution). A switch happens when push + pull together outweigh anxiety
   + habit.
4. **Interview enough people to see the pattern, not the outlier** — practitioner
   guidance converges on roughly 10–12 interviews before the causal pattern
   becomes clear, well below what quantitative research would require, because
   the goal is causal structure, not statistical frequency.

**Evidence produced.** A set of coded switch narratives (push/pull/anxiety/habit
tagged), from which a shared causal "job" statement can be drafted and checked
against — evidence for whether the product actually resolves the job it's being
built or marketed around, distinct from and complementary to the assumption tests
in §3.2.

**Source.** [Clay Christensen on Jobs-to-be-Done, jobstobedone.org](https://jobstobedone.org/radio/clay-christensen-on-jobs-to-be-done/) — jobstobedone.org is maintained by the Rewired Group (Bob Moesta's consultancy, the originators of the switch-interview technique), and this page carries Christensen's own definitional framing directly; verified by direct fetch for the definition and the five-phase/four-forces interview structure.

### 7.3 Designing and Recording a Manual Validation Pass

**[SYNTHESIS]** — no single cited source prescribes this exact checklist; it
consolidates the evidence-capture discipline embedded across §1 (write down
specific violations, tagged to location), §2 (screenshot/session-record every
claim), §5.2 (track scores over time, not once), and §7.1–7.2 (record real task
attempts and improvised workarounds as signal) into one procedure for structuring
any ad hoc manual validation pass:

1. **State the task and the "job" it represents** before starting (§7.2) — a
   validation pass without a named task drifts into unfocused poking.
2. **Run the task start to finish as a real user would**, not a scripted happy
   path — note every deviation, workaround, or moment of hesitation as it happens
   (§7.1).
3. **Tag every finding to a specific location and a specific lens** (heuristic
   §1.1, risk category §3.1, HEART category §4.1, docs-type §6.2) — an untagged
   finding is hard to route to an owner or prioritize.
4. **Capture the artifact, not just the conclusion**: exact command/output,
   exact screen, exact error text, exact survey response — "it was confusing"
   is not evidence; the transcript that shows *what* was confusing is.
5. **Severity-rate after the pass, separately from the pass itself** (§1.3), so
   discovery isn't biased by premature triage.
6. **Convert findings into an owned backlog item, not a narrative report** — every
   method in this document ends in a backlog-ready artifact (a scored defect list,
   a risk ledger, a GSM table, a segmented roadmap); a manual validation pass that
   ends only in prose loses most of its value.

---

## 8. Applied Synthesis — Manual Validation When the User Is an AI Agent

**[SYNTHESIS — applied, not sourced]** Nothing in the cited literature above was
written with an AI agent as the end user; this section applies the generic methods
above to that case, reasoning from their stated mechanics rather than from any
agent-specific source, because no authoritative primary literature on this exact
question was found.

The generic methods transfer, but several assumptions they quietly make about
"the user" break and need explicit substitutes:

- **§1 (heuristic evaluation, cognitive walkthrough)** assumes a user with
  imperfect memory, imprecise reading, and a visual field — heuristics like
  "recognition rather than recall" (#6) and "aesthetic and minimalist design" (#8)
  don't map cleanly. What *does* transfer directly: "match between the system and
  the real world" (#2) becomes "does output use the vocabulary/format the agent's
  prompting or training would expect (e.g., standard exit codes, conventional flag
  names, parseable structure) rather than ghx-specific jargon"; "help users
  recognize, diagnose, and recover from errors" (#9) becomes "is the error message
  something an agent can act on autonomously — does it name the fix, not just the
  failure" (an agent has no forum to Google the error against); "visibility of
  system status" (#1) becomes "is progress/completion legible from stdout/exit
  code alone, since the agent cannot see a spinner." A cognitive-walkthrough's four
  questions still apply near-verbatim to a tool call: will the agent attempt the
  right subcommand; will it notice the right flag exists (from `--help` or a
  single doc read); will it associate that flag with its goal; will the tool's
  output confirm progress in a way the agent's own context window will parse
  correctly.

- **§1.6 / §7.3 ("write down what a human found confusing")** substitutes "trace
  what the agent actually did" for "what a human said." The equivalent evidence
  artifact for an agent is the full tool-call transcript: which command it ran,
  what it read from `--help`/docs first, where it retried or backtracked, and
  whether it eventually reached task success — this is strictly more inspectable
  than a human think-aloud, since every "hesitation" is a literal logged action,
  not an inferred mental state.

- **§4/§5 (metrics, funnel, PMF)** substitute *task success rate* and *evidence
  quality* for HEART's Happiness/Engagement and Ellis's "very disappointed"
  survey — an agent has no felt disappointment to survey. The nearest honest
  analogues: **task success** (did the agent complete the job correctly, per
  §4.1's Task Success pillar, which does transfer unmodified), **efficiency**
  (tokens spent, tool calls made, wall-clock time — the CLI/agent equivalent of a
  funnel's step-conversion audit in §4.3, where each unnecessary tool call is a
  "drop-off" worth diagnosing as UX-problem-vs-value-problem exactly as in §4.3),
  and **evidence quality** (did the agent's final report cite real, checkable
  outputs, or did it fabricate/guess — the closest analogue to §1.3's "severity"
  axis, since a confidently wrong report is a catastrophe-severity failure even if
  the underlying task technically completed). Retention/adoption (§4.1, §5.3) have
  no agent-native equivalent at the level of a single session, but do transfer at
  the level of *does this agent, or this agent's operator, keep choosing to invoke
  this tool over alternatives across many sessions* — closer to Balfour's
  behavioral retention-curve logic (§5.3) than to a survey.

- **§6 (docs audit)** transfers with unusually high fidelity, because an
  AI agent's relationship to documentation is *closer* to Divio's rubric's ideal
  reader than a human's: an agent reading `--help` output is doing exactly a
  reference-type lookup (§6.2), and an agent following a README's quickstart is
  doing exactly a tutorial-type read, so mixed-purpose docs (the failure mode
  §6.2 flags) actively cost an agent tokens and misdirected tool calls in a way
  that's directly measurable (compare tokens-to-first-successful-call against a
  cleanly-typed doc vs. a mixed one). Information scent (§6.1) also transfers
  directly: a flag or subcommand name is exactly a "link label" an agent must
  estimate the value of before spending a tool call to try it — vague or
  inconsistent naming (e.g., a `scan` command that doesn't scan, or two flags
  that sound like they do the same thing) burns exploratory tool calls exactly as
  a vague link burns a human click.

- **§7.1 (dogfooding)** transfers essentially unmodified and is arguably *more*
  load-bearing for an agent-facing tool than for a human-facing one: the
  cheapest, highest-fidelity validation of a CLI sidecar for coding agents is
  running a real coding agent against a real repository task with the tool
  actually enabled, end to end, and reading the full transcript — not a scripted
  "does `ghx scan` return exit code 0" smoke test. The Microsoft precedent's core
  insight (§7.1: make it real work, not a demo) is exactly the argument for
  running the tool inside genuine agent task sessions rather than only unit tests.

- **§2 (teardown) and §3 (Cagan/Torres risk framing)** transfer without
  modification to the *product-decision* layer, because the risks being retired
  (will this be used, can the agent figure out how to use it, can it be built,
  does it work for the business) don't change based on who the end user is — only
  the *evidence* used to retire "usability risk" changes, from a human usability
  test to an agent transcript per the point above.

**What this implies for a concrete audit target (applied example, not a source
claim):** a CLI/agent-sidecar product's manual validation pass should record, per
task: task success (binary), tool calls made vs. minimum necessary (efficiency),
whether error output let the agent self-correct without external help (§1.6's
"help users recover from errors," agent-adapted), whether the agent's final
report cited real command output or fabricated claims (evidence quality), and
whether the agent needed to consult docs beyond `--help`/reference to complete
the task (a §6.2-style docs-type gap signal — needing prose explanation mid-task
suggests the reference layer alone wasn't self-sufficient).

---

## Source Ledger

| # | Source | Deep URL | Why authoritative / unique contribution |
|---|---|---|---|
| 1 | Jakob Nielsen, "10 Usability Heuristics for User Interface Design" | https://www.nngroup.com/articles/ten-usability-heuristics/ | Original, canonical statement of the field's most-cited usability rubric, maintained by the firm Nielsen co-founded. |
| 2 | NN/g, "How to Conduct a Heuristic Evaluation" | https://www.nngroup.com/articles/how-to-conduct-a-heuristic-evaluation/ | The operational runbook (team size, timeboxing, debrief structure) distinct from the heuristics list itself. |
| 3 | Jakob Nielsen, "Severity Ratings for Usability Problems" | https://www.nngroup.com/articles/how-to-rate-the-severity-of-usability-problems/ | Primary source for the 0–4 severity scale and three-factor severity model used across the field. |
| 4 | Jakob Nielsen, "Summary of Usability Inspection Methods" | https://www.nngroup.com/articles/summary-of-usability-inspection-methods/ | Nielsen's own taxonomy of all inspection methods (heuristic eval, cognitive/pluralistic walkthrough, feature/consistency/standards inspection). |
| 5 | NN/g, "Evaluate Interface Learnability with Cognitive Walkthroughs" | https://www.nngroup.com/articles/cognitive-walkthroughs/ | Defines cognitive walkthrough and when to use it vs. usability testing. |
| 6 | NN/g, "How to Conduct a Cognitive Walkthrough Workshop" | https://www.nngroup.com/articles/cognitive-walkthrough-workshop/ | Operational runbook: roles, the exact four prescribed questions, documentation method. |
| 7 | NN/g, "UX Expert Reviews" | https://www.nngroup.com/articles/ux-expert-reviews/ | Distinguishes expert review from heuristic evaluation and specifies reviewer qualifications and deliverable format. |
| 8 | Jakob Nielsen, "Why You Only Need to Test with 5 Users" | https://www.nngroup.com/articles/why-you-only-need-to-test-with-5-users/ | Original statistical justification (N×(1−L)ⁿ model) plus explicit caveats for quantitative/heterogeneous cases. |
| 9 | Oliver Lindberg, "Interview with Steve Krug: how to get DIY usability testing right" | https://medium.com/the-lindberg-interviews/interview-with-steve-krug-how-to-get-diy-usability-testing-right-63dedddbd0ae | Direct interview with Krug giving the exact "3 users, one morning, debrief over lunch" cadence in his own words. |
| 10 | LogRocket, "What is a product teardown? Process, tools, and other insights" | https://blog.logrocket.com/product-management/product-teardown-process-tools/ | Most structured public step-by-step teardown runbook found; product teardown has no single academic origin, so this is cited as a practitioner secondary source. |
| 11 | About Amazon, "An insider look at Amazon's culture and processes" | https://www.aboutamazon.com/news/workplace/an-insider-look-at-amazons-culture-and-processes | Amazon's own official primary-source description of the Working Backwards / PR-FAQ pre-build validation process. |
| 12 | Marty Cagan, "The Four Big Risks," SVPG | https://www.svpg.com/four-big-risks/ | Cagan's own restatement of the field's most-cited discovery-risk framework, including the risk-ownership model; full text verified by direct fetch. |
| 13 | Teresa Torres, "Assumption Testing: Everything You Need to Know to Get Started" | https://www.producttalk.org/assumption-testing/ | Torres' direct operational statement of assumption generation, the evidence/importance prioritization map, and the four test types. |
| 14 | Kerry Rodden, Hilary Hutchinson, Xin Fu, "Measuring the User Experience on a Large Scale," CHI 2010 | https://research.google/pubs/measuring-the-user-experience-on-a-large-scale-user-centered-metrics-for-web-applications/ | Original peer-reviewed paper introducing HEART and the Goals-Signals-Metrics process; still the most-cited large-scale UX metrics framework. |
| 15 | Amplitude, "Every Product Needs a North Star Metric: Here's How to Find Yours" | https://amplitude.com/blog/product-north-star-metric | The article that popularized and operationalized the North Star + input-metrics framework industry-wide. |
| 16 | Sean Ellis, "Using Product/Market Fit to Drive Sustainable Growth," GrowthHackers | https://medium.com/growthhackers/using-product-market-fit-to-drive-sustainable-growth-58e9124ee8db | Ellis's own statement of the exact survey question, response options, 40% threshold, and sample-size guidance. |
| 17 | Sean Ellis & GoPractice, PMF Survey | https://pmfsurvey.com/ | The live, official survey instrument Ellis currently co-maintains (successor to the discontinued Survey.io). |
| 18 | Rahul Vohra, "How Superhuman Built an Engine to Find Product/Market Fit," First Round Review | https://review.firstround.com/how-superhuman-built-an-engine-to-find-product-market-fit/ | Vohra's own detailed account of the four-step PMF engine and Superhuman's 22%→32%→58% score progression. |
| 19 | Brian Balfour, "The Never Ending Road To Product Market Fit" | https://brianbalfour.com/essays/product-market-fit | Balfour's own statement of the retention-cohort-curve method and its explicit contrast with survey-based PMF measurement. |
| 20 | NN/g, "Information Scent: How Users Decide Where to Go Next" | https://www.nngroup.com/articles/information-scent/ | Canonical UX statement of information-foraging theory with concrete, auditable dimensions (label text, context, brand trust). |
| 21 | Divio, "About the Documentation System" (Diátaxis) | https://docs.divio.com/documentation-system/ | Originating articulation of the four-purpose documentation framework now widely adopted across major open-source/commercial docs sets. |
| 22 | Wikipedia, "Eating your own dog food" | https://en.wikipedia.org/wiki/Eating_your_own_dog_food | Consolidated, cross-referenced historical account of the 1988 Microsoft origin (Maritz email, Windows NT/Cutler precedent) with more verifiable specificity than found practitioner posts. |
| 23 | jobstobedone.org (Rewired Group), "Clay Christensen on Jobs-to-be-Done" | https://jobstobedone.org/radio/clay-christensen-on-jobs-to-be-done/ | Hosted by Bob Moesta's Rewired Group (originators of the switch-interview technique); carries Christensen's own definitional framing plus the five-phase/four-forces interview structure. |

**Methods referenced but not independently deep-linked:** David Bland's
assumption-mapping 2×2 (referenced within Torres' article, source #13, not
separately fetched); Reforge's funnel/activation/retention guides (confirmed to
exist via search but returned HTTP 403 on direct fetch — gated behind paid
membership, so not cited as a verifiable deep link; §4.3 is marked as synthesis
for this reason).

---

## Manual Validation Playbook — a concrete checklist to execute on a live product

Use this as a single audit runbook, borrowing the specific procedure from the
relevant section above for any step marked with a §.

### Phase 0 — Scope (30–60 min)

- [ ] Write the audit's goal in one sentence (§2 step 1) — what decision will this
      audit inform?
- [ ] Name the specific surfaces in scope: CLI/UI, docs, onboarding/first-run,
      metrics/dashboards, strategy artifacts (roadmap, PRDs).
- [ ] Name the specific task(s) or job(s) to validate against (§7.2) — an audit
      without a named task drifts.

### Phase 1 — Inspection (no users needed; hours to 1–2 days)

- [ ] Heuristic evaluation: 3–5 evaluators, independent, timeboxed 1–2 hrs each,
      scoped to one task/section (§1.2).
- [ ] Cognitive walkthrough on the single most important "zero to first value"
      task, 2–6 person cross-functional team, four fixed questions per step
      (§1.4, §6.3).
- [ ] Outside-eyes expert review if a genuinely uninvolved reviewer with real
      usability-research exposure is available (§1.5).
- [ ] Severity-rate the consolidated findings afterward, separately from
      discovery, using the 0–4 scale (§1.3).
- [ ] Documentation audit: inventory docs by Divio type (tutorial / how-to /
      reference / explanation), flag mixed-purpose pages and coverage gaps
      (§6.2).
- [ ] Information-scent pass over navigation, headings, flag/command names, and
      error messages (§6.1).

### Phase 2 — Discovery risk check (hours to days)

- [ ] Fill in a four-row risk ledger — value, usability, feasibility, business
      viability — citing specific evidence per row, not opinion (§3.1).
- [ ] For any proposed-but-unbuilt feature, draft assumptions from a story map
      or opportunity-solution tree; plot on an importance-vs-evidence 2×2; test
      the top-right quadrant with the cheapest adequate instrument (§3.2).

### Phase 3 — Metrics & funnel check (requires production data)

- [ ] Build (or confirm) a Goals→Signals→Metrics table per relevant HEART
      category (§4.1).
- [ ] Confirm the North Star metric passes all three tests (realized value,
      strategy-revealing, leading not lagging) and has 3–5 named input metrics
      (§4.2).
- [ ] Plot the activation funnel; find the steepest drop-off; diagnose UX vs.
      value at that step, don't guess (§4.3).
- [ ] Plot cohort retention curves; check for a flattening tail; segment by
      acquisition source/cohort to find the retained core (§5.3).

### Phase 4 — Market/fit check (requires a real user base)

- [ ] Run the Ellis 40% survey on users active ≥2x in the last 2 weeks, ≥40
      responses (§5.1).
- [ ] If below 40%, run Vohra's segmentation: isolate the "very disappointed"
      cohort, build their persona, split the roadmap 50/50 between doubling
      down and closing gaps (§5.2).
- [ ] Cross-check the survey score against the retention-curve flattening signal
      (§5.3) — don't trust either alone.

### Phase 5 — Manual walkthrough & dogfooding (ongoing discipline, not one-time)

- [ ] Run the product on a real task with real stakes, not a scripted demo;
      log failures and improvised workarounds as they happen (§7.1).
- [ ] Run switch interviews (5–12) with real adopters or churned users; code for
      push/pull/anxiety/habit; check the resulting job statement against what
      the product actually delivers (§7.2).
- [ ] For every finding across every phase: tag to a specific location and
      lens, capture the literal artifact (transcript/screenshot/output), and
      route to an owned backlog item — not a narrative-only report (§7.3).

### Phase 6 — For an agent-facing surface specifically (applied synthesis, §8)

- [ ] Run a real coding-agent session against a real task with the tool enabled
      end-to-end; capture the full tool-call transcript (§7.1, agent-adapted).
- [ ] Record per task: binary task success, tool calls made vs. minimum
      necessary, whether errors let the agent self-correct without external
      help, whether the final report's claims are checkable against real
      output or fabricated (evidence quality), and whether the agent needed
      prose docs beyond reference/`--help` to finish (§6.2 gap signal).
- [ ] Treat a confidently wrong final report as a severity-4 finding regardless
      of whether the underlying task technically completed (§1.3, agent-adapted).
