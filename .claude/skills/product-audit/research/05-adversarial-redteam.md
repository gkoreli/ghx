# Adversarial / Red-Team Methods for Honest Product & Strategy Critique

*Research artifact — methods for deliberately attacking your own product and
strategy to surface issues, gaps, and misalignments before a competitor,
customer, or postmortem does it for you.*

---

## Abstract

Most product and strategy reviews fail not because teams lack analytical
skill but because the review process itself is structurally biased toward
agreement: the people in the room built the thing, are attached to the
narrative that justifies it, and face social cost for dissent. The methods
below are not analysis techniques in the ordinary sense — they are
**structural interventions** that change who is allowed to say what, and
when, so that disconfirming evidence has a legal route into the room. Eight
families are covered: the pre-mortem (Gary Klein's prospective-hindsight
method), structured red-teaming (military/intelligence-derived alternative
analysis), devil's advocacy and dialectical inquiry (institutionalized
dissent, the "Tenth Man"), inversion (Munger/Jacobi — find failure by
engineering it), self-disruption exercises ("kill the company" / attacker
simulation), assumption-attack techniques (Key Assumptions Check, "What
Would Have to Be True"), the cognitive-bias literature that explains *why*
audits go soft (confirmation bias, sunk cost, narrative fallacy, planning
fallacy, resulting), and — the part most red-team writing skips — how to
keep the critique constructive so it produces a prioritized punch list
instead of paralysis or nihilism.

Every claim below is sourced to a specific, deep-linked, primary or
authoritative document, verified by direct fetch where the source allows
automated access (paywalled academic articles are marked as such — citation
verified via abstract/DOI, full text not independently confirmed). A
"Source Ledger" and a runnable "Adversarial Audit Protocol" close the
document, including a severity/priority scheme so findings resolve into
action rather than doom.

Throughout, a boxed **Applied to a technical/AI-agent product** note shows
how the generic method bites on (a) an AI coding-agent tool and (b) a
strategy/north-star document, without assuming any specific codebase.

---

## 1. Pre-mortem (Klein)

**(a) What it is.** A pre-mortem inverts the normal order of a project
retrospective. Instead of waiting for failure and asking "what went wrong,"
the team is told, *before launch*, to assume the project **has already
failed spectacularly**, and to generate reasons why. Klein calls this
"prospective hindsight": imagining an outcome has already happened measurably
improves people's ability to correctly identify the causal chain that leads
to it, compared to being asked to speculate about the future in the
ordinary, tentative "might happen" framing.

> "Research conducted in 1989 by Deborah J. Mitchell, of the Wharton School;
> Jay Russo, of Cornell; and Nancy Pennington, of the University of
> Colorado, found that prospective hindsight — imagining that an event has
> already occurred — increases the ability to correctly identify reasons
> for future outcomes by 30%."
> — Gary Klein, "Performing a Project Premortem," *Harvard Business Review*,
> September 2007, p. 18.

**(b) Step-by-step procedure** (reconstructed from the primary HBR text):

1. **Brief the group** on the plan as it currently stands (Klein notes real
   sessions run after a full kickoff meeting, sometimes 90 minutes long).
2. **The leader declares failure.** In one sentence: "It is [N months/a
   year] from now. We implemented this plan exactly as it stands. It has
   failed spectacularly." No hedging language — the failure is stipulated,
   not hypothesized.
3. **Silent, individual writing (5–10 minutes).** Each participant
   independently writes down every plausible reason for the failure —
   explicitly including "the kinds of things they ordinarily wouldn't
   mention... for fear of being impolitic." Doing this *silently and
   individually first* is load-bearing: it is what lets a quiet, junior, or
   politically exposed team member's real objection survive contact with a
   confident room, because no one has yet anchored the group on a shared
   story.
4. **Round-robin readout**, starting with the project lead: each person
   reads **one** reason, in turn, until every distinct reason on every list
   has been voiced — no repeats, no rebuttal during the round.
5. **The project lead reviews the full list afterward** and revises the
   plan to address the reasons that are real and actionable.

**(c) Output/finding it produces.** A ranked inventory of *causal* failure
stories — not vague risks, but "the CEO retired and interest waned," "the
algorithm doesn't fit on the laptops used in the field" — concrete enough
that the plan can be edited in response. Klein's own case examples show the
mechanism working: an algorithm-deployment project surfaced a hardware
constraint that had been known but suppressed for fear of looking difficult,
and a research project surfaced a corporate review deadline that no one had
raised in 90 minutes of kickoff discussion.

**(d) Citation.**
[Gary Klein, "Performing a Project Premortem," *Harvard Business Review*, September 2007](https://hbr.org/2007/09/performing-a-project-premortem) —
**why authoritative:** Klein is the practitioner who coined and popularized
the technique for a business audience; this is the original HBR article,
not a summary of it. *Verified by direct fetch of the full article text
(PDF mirror), including exact procedural steps and the 30% research
citation.*

Underlying the method: [Deborah J. Mitchell, J. Edward Russo, and Nancy
Pennington, "Back to the Future: Temporal Perspective in the Explanation of
Events," *Journal of Behavioral Decision Making* 2 (1989): 25–38, DOI
10.1002/bdm.3960020103](https://onlinelibrary.wiley.com/doi/abs/10.1002/bdm.3960020103) —
**why authoritative:** this is the peer-reviewed experimental study that
established the "prospective hindsight" effect Klein operationalized; it is
the empirical foundation, not the popularization. *Citation verified via
DOI/abstract record and via its citation inside Klein's own article; full
text is paywalled and not independently confirmed by fetch.*

Also see Daniel Kahneman's endorsement and restatement of the technique —
he describes gathering people "knowledgeable about the decision" and having
them write "a brief history of that disaster" — in *Thinking, Fast and
Slow* (Farrar, Straus and Giroux, 2011), ch. 24, "The Engine of Capitalism,"
discussed alongside his own **planning fallacy** research (see §7 below).
*Book content verified via multiple independent excerpt sources quoting the
same passage; direct primary-text URL not fetchable since the book is not
hosted openly.*

> **Applied to a technical/AI-agent product.** Run the pre-mortem before a
> release or before a north-star doc ships, not after a bad eval run. For a
> CLI/agent-sidecar tool: "It's six months from now, agents using this tool
> systematically produce worse code than agents that don't. Write down why."
> For a strategy doc: "It's a year from now and the roadmap in this doc was
> completed exactly as written, and the product still failed. Why?" — this
> flushes out the difference between "we shipped the plan" and "the plan
> was the wrong plan," which ordinary status reviews rarely separate.

---

## 2. Red-Teaming (structured alternative analysis)

**(a) What it is.** Red-teaming is an institutionalized adversary role: a
person or team whose *job* is to think and act as a capable, motivated
opponent against your own plan, in order to find the weaknesses a real
opponent would exploit, before they do. It originated in military wargaming
(the "red" force vs. the "blue" force) and was formalized into a doctrine of
structured analytic techniques by the U.S. intelligence community after
high-profile analytic failures.

> "We believe red teaming is especially important now… Aggressive red teams
> challenge emerging operational concepts in order to discover weaknesses
> before real adversaries do. Red teaming also tempers the complacency that
> often follows success."
> — U.S. Defense Science Board Task Force on The Role and Status of DoD Red
> Teaming Activities, September 2003, quoted in the UFMCS *Red Team
> Handbook*.

**(b) Step-by-step procedure** — the handbook lays out red-teaming less as
one technique than a **menu** of structured moves; the core, generically
transferable ones are:

1. **Establish independence and mandate.** A red team must be organizationally
   independent enough to publish a contrary view without career cost, but
   still have "the Commander's confidence, support, and direction" — a red
   team nobody has to listen to produces theater, not signal.
2. **Outline the mainline judgment/plan** and the evidence currently used to
   support it, in writing, so the target of the challenge is explicit.
3. **Apply one or more structured techniques** against that mainline view —
   the handbook enumerates, among others: **Key Assumptions Check**
   (§6 below), **Devil's Advocacy** (§3 below), **Analysis of Competing
   Hypotheses**, **"What If?" Analysis** (assume the plan has already
   failed/succeeded unexpectedly and work backward — a structural cousin of
   the pre-mortem), and **Alternative Futures Analysis** (systematically
   generate multiple plausible future states rather than one expected one).
4. **Report findings including the weak hypotheses**, explicitly flagging
   which should still be tracked as new information arrives, rather than
   collapsing to a single confident verdict.
5. **Feed findings back to the decision-maker directly**, not filtered
   through the team whose plan is being challenged — the Israeli Devil's
   Advocate Unit's memos, for example, route straight to the director of
   military intelligence and senior decision-makers (see §3).

**(c) Output/finding it produces.** Either (i) a documented, specific
critique of the plan's weakest assumptions and the alternative explanation
that better fits the evidence, or (ii) — importantly — a documented
*reaffirmation* that the plan holds up under adversarial scrutiny, which is
itself a valuable, citable finding rather than a null result.

**(d) Citation.**
[University of Foreign Military and Cultural Studies (UFMCS), *Red Team
Handbook*, v.5, 15 April 2011](https://newandimproved.com/wp-content/uploads/2014/04/ufmcs_red_team_handbook_apr2011.pdf) —
**why authoritative:** UFMCS, at Fort Leavenworth, is the U.S. Army's
dedicated red-teaming schoolhouse; this handbook is its doctrinal training
text, approved for unlimited public release, and is the most complete
single compilation of structured red-team/alternative-analysis techniques
available outside classified channels. *Verified by direct full-text
extraction (7,882 lines); quotes above pulled directly from the source
PDF.*

[Bryce G. Hoffman, *Red Teaming: Transform Your Business by Thinking Like
the Enemy* (Piatkus, 2017)](https://brycehoffman.com/books/red-teaming/) —
**why authoritative:** Hoffman is "the first and only civilian ever
admitted to the U.S. Army's elite Red Team Leaders Program" and translates
the military doctrine above into a business-audience playbook used at Ford,
citing Amazon, Google, and Toyota as corporate adopters. *Verified via the
author's own site describing the book's method in his own words.*

[Richards J. Heuer Jr., *Psychology of Intelligence Analysis*, CIA Center
for the Study of Intelligence, 1999](https://www.cia.gov/resources/csi/books-monographs/psychology-of-intelligence-analysis-2/) —
**why authoritative:** Heuer is the CIA veteran whose cognitive-bias
analysis of intelligence failure created the "alternative analysis"
discipline the intelligence community later renamed "structured analytic
techniques"; this book is the CIA's own officially declassified and
publicly hosted foundational text, still assigned in analyst training.
*Page confirmed live (HTTP 200) on cia.gov; the page is JavaScript-rendered
so full body text was not independently extracted by automated fetch — mark
as source-exists-verified, full-text-unverified. A stable text mirror
exists at [archive.org](https://archive.org/details/PsychologyOfIntelligenceAnalysis).*

> **Applied to a technical/AI-agent product.** For a CLI-based coding
> sidecar, red-teaming means literally trying to break the thing the way a
> competitor or a skeptical power user would: feed it adversarial repos,
> ambiguous instructions, or agents that misuse its output, and write up
> what a well-resourced clone would exploit. For a strategy doc, apply "What
> If?" analysis to the stated north star itself: what if the premise
> (agents need a code-reconnaissance sidecar at all) is simply wrong — what
> would that world look like, and how would you know you were in it?

---

## 3. Devil's Advocate / Dialectical Inquiry / "The Tenth Man"

**(a) What it is.** Where red-teaming is a broad menu of adversarial
techniques, devil's advocacy is the narrowest, most portable instance:
**one person is explicitly assigned to build the best possible case against
the prevailing view**, even if they privately agree with it. "Dialectical
inquiry" is the academic name for the broader family of structured-dissent
methods (devil's advocacy is one; formal debate between two constructed
teams — "Team A / Team B" in intelligence tradecraft — is another). The
"Tenth Man Rule" is the most quotable operationalization: in a group of ten
people who all reach the same conclusion, the tenth is *obligated* to argue
the group is wrong, regardless of personal belief, specifically to break
the social cost of being the lone dissenter.

**(b) Step-by-step procedure** (UFMCS "Devil's Advocacy" method, combined
with the Israeli Devil's Advocate Unit's institutional design):

1. **Select the target.** Devil's Advocacy is "most effective when used to
   challenge an analytic consensus or a key assumption regarding a
   critically important [decision] — on those issues that one cannot afford
   to get wrong." Don't spend it on low-stakes calls.
2. **Outline the mainline judgment and its key assumptions**, stated and
   unstated, and characterize the evidence currently offered for it.
3. **Select the assumption(s) most susceptible to challenge** — not the
   easiest strawman, the load-bearing one.
4. **Review the supporting information** for validity, possible deception,
   or major evidentiary gaps.
5. **Build the best possible case for the alternative hypothesis** —
   actively marshal evidence *for* the contrarian position, not just poke
   holes in the mainline one. This is the discipline that separates devil's
   advocacy from mere contrarianism.
6. **Present findings to the group/decision-maker directly**, producing one
   of three outcomes the handbook itself names: (i) the current line is
   confirmed sound, (ii) it is still strongest but needs more work in
   specific spots, or (iii) it has a real flaw and must change.

Institutionally, the Israeli Devil's Advocate Unit (est. after the 1973 Yom
Kippur War, on recommendation of the Agranat Commission that investigated
the intelligence failure) shows how to make this durable rather than
one-off: it is staffed by senior officers, is structurally independent of
the Research Department it critiques, and its head reports directly to the
Director of Military Intelligence — not through the chain being challenged.
In 2024 it emerged the unit had explicitly warned senior leadership of a
Hamas attack weeks before October 7, 2023, illustrating both the value and
the limits of the mechanism (a correct warning is not the same as a
warning that is acted on).

**(c) Output/finding it produces.** A single, well-argued written case for
"the mainline view is wrong, and here specifically is why," strong enough
that it must be rebutted on the merits rather than dismissed socially —
plus, per Paul Graham's disagreement hierarchy (§8), the discipline of
attacking the argument's *central point*, not a peripheral or tonal
weakness.

**(d) Citation.**
[UFMCS *Red Team Handbook*, "Devil's Advocacy," pp. 161–162](https://newandimproved.com/wp-content/uploads/2014/04/ufmcs_red_team_handbook_apr2011.pdf) —
same source and rationale as §2; this is the specific technique section
quoted directly above.

[Mind Collection, "The Tenth Man Rule: How to Take Devil's Advocacy to a
New Level"](https://themindcollection.com/the-tenth-man-rule-devils-advocacy/) —
**why authoritative:** a well-researched synthesis piece specifically
tracing the Tenth Man Rule's popularization (via *World War Z*) back to the
real Israeli devil's-advocate practice, and — its unique contribution —
offering concrete implementation guidance (rotate the role, use a round
table with no hierarchy, reserve it for high-stakes calls, commit to
sustained not superficial dissent). *Verified by direct fetch.*

[Wikipedia, "Devil's Advocate Unit"](https://en.wikipedia.org/wiki/Devil%27s_Advocate_Unit) —
**why authoritative:** consolidated, citation-backed account of the real
unit's founding (post-Agranat Commission), mandate, reporting line
(independent, head reports directly to the MID commander), and its verified
track record including the pre-October 7 warnings. *Verified by direct
fetch; used as a cross-check against the academic article below since the
latter is paywalled.*

[David Siman-Tov et al., "The Devil's Advocate in Intelligence: The Israeli
Experience," *Intelligence and National Security* 33, no. 6
(2018)](https://www.tandfonline.com/doi/full/10.1080/02684527.2018.1470062) —
**why authoritative:** the peer-reviewed academic treatment of the unit by
scholars with direct access to Israeli intelligence practitioners; this is
the primary scholarly source underlying the popularizations above. *Citation
verified to exist (DOI resolves, abstract indexed by multiple databases);
full text returned HTTP 403 (paywalled) and is unverified by direct fetch —
flagged accordingly.*

> **Applied to a technical/AI-agent product.** Assign a rotating, explicit
> devil's-advocate role before any eval verdict or release decision ships —
> someone whose job that review cycle is to build the strongest possible
> case that the eval methodology, not just the product, is wrong. For a
> north-star doc, run devil's advocacy on the single most load-bearing
> claim (e.g., "agents need a persistent code-reconnaissance sidecar
> distinct from their base tool loop") rather than on secondary roadmap
> items — per the UFMCS guidance above, spend it where you cannot afford to
> be wrong.

---

## 4. Inversion (Jacobi / Munger)

**(a) What it is.** Inversion is the practice of solving a problem
backward: instead of asking "how do I succeed," ask "how would I guarantee
failure," then simply avoid doing those things. It traces to 19th-century
mathematician Carl Gustav Jacob Jacobi's maxim *man muss immer umkehren* —
"invert, always invert" — and was popularized for business/investing
audiences by Charlie Munger.

> "Jacobi knew that it is in the nature of things that many hard problems
> are best solved when they are addressed backward... Invert, always
> invert."
> — Charlie Munger, quoted and explained in Shane Parrish, "Inversion: The
> Power of Avoiding Stupidity," Farnam Street.

Munger's own gloss captures the "avoid the graveyard" framing: "All I want
to know is where I'm going to die, so I'll never go there."

**(b) Step-by-step procedure:**

1. **State the goal plainly** (e.g., "ship a product that agents rely on").
2. **Invert it explicitly**: "What would guarantee this fails? What actions,
   taken deliberately, would destroy this?"
3. **List those failure-guaranteeing actions concretely** — this is
   generative in a way the forward question often isn't, because it is
   easier for people to recognize a stupid, destructive action than to
   invent a brilliant one from scratch.
4. **Invert the list back into constraints**: treat "never do X" as a hard
   design/process constraint going forward, not a one-time insight.

The underlying logic, per Farnam Street's synthesis: **"avoiding stupidity
is easier than seeking brilliance"** — subtraction of known failure modes
is a lower-variance strategy than addition of untested new initiatives.

**(c) Output/finding it produces.** A short, concrete "never do this" list
that functions as a standing constraint set — distinct from a premortem's
narrative failure stories in that inversion is meant to run continuously
as a filter on decisions, not just once before a launch.

**(d) Citation.**
[Shane Parrish, "Inversion: The Power of Avoiding Stupidity," Farnam
Street](https://fs.blog/inversion/) —
**why authoritative:** Farnam Street is the most careful, primary-source-
grounded popularizer of Munger's mental-model corpus (compiled from *Poor
Charlie's Almanack* and Munger's speeches), and this specific page traces
the idea to Jacobi rather than treating "invert, always invert" as
Munger-original — the correct, non-lossy attribution. *Verified by direct
fetch, including the exact Jacobi/Munger quotes above.*

> **Applied to a technical/AI-agent product.** Instead of asking "how do we
> make this sidecar valuable to agents," invert: "What would we do if we
> wanted an agent to actively distrust or ignore this tool's output?"
> (Silent failures, plausible-but-wrong recon, latency that makes agents
> route around it, a report format the agent can't parse.) Each becomes a
> concrete constraint ("recon output must be structurally distinguishable
> from a wrong answer"). Applied to a north-star doc: "What would we write
> in this document if our explicit goal were to produce wishful thinking
> that reads well but doesn't survive contact with an eval?" — the answer
> is usually: unfalsifiable claims, no named metric, no owner, no date. Grep
> the actual doc for those four absences.

---

## 5. "Kill the Company" / Self-Disruption Attack

**(a) What it is.** A facilitated exercise in which the team role-plays as
its own best-funded, least-scrupulous competitor and brainstorms exactly
how to put the real company out of business. Pioneered by Lisa Bodell
(futurethink); the companion idea — that market leaders systematically fail
to see the disruptive threat from below because it doesn't look like
competition until too late — is the classic Christensen/Bower disruption
thesis, and Andy Grove's "strategic inflection point" framing supplies the
paranoia-as-discipline rationale for running the exercise at all.

> "If you were our competitor with unlimited resources, what would you do
> right now to put us out of business?"
> — core facilitation question, Lisa Bodell, "Want Your Business To Survive
> The Next Five Years? Kill Your Company Now," *Forbes*, August 31, 2018.

> "A strategic inflection point is a time in the life of a business when
> its fundamentals are about to change. That change can mean an opportunity
> to rise to new heights. But it may just as likely signal the beginning of
> the end."
> — Andrew S. Grove, *Only the Paranoid Survive* (Currency Doubleday, 1996).

**(b) Step-by-step procedure** (Bodell's Kill the Company method, verified
against the primary Forbes text):

1. **Gather a cross-functional group** — not just leadership; the people
   closest to the product's actual weaknesses often know the most
   dangerous attack vectors.
2. **Ask the reversed question**: not "how do we beat the competition" but
   "what would you do to put us out of business today?"
3. **Brainstorm on sticky notes**, one threat per note — from the mundane
   (a competitor undercuts on price) to the existential (a security breach,
   a platform shift that obsoletes the product category).
4. **Cluster and rank by threat severity** on a board, smallest-to-largest
   or easiest-to-hardest-to-execute.
5. **Identify the top three threats** and look for clustering — multiple
   independent threats pointing at the same underlying vulnerability are
   the real signal.
6. **Generate preventative countermeasures** for the top threats, and —
   distinctly — **extract opportunities**: tactics that would work *against*
   competitors, and organizational strengths the exercise surfaced that had
   been overlooked.
7. **Repeat annually.** Bodell frames this as a standing ritual, not a
   one-off: "By killing your own company at least once a year, you'll
   prevent anyone else from writing its actual obituary."

Bodell's own explanation of why the exercise works is itself a debiasing
insight: **"When you give people freedom to say it like it is, they really
lean into"** generating ideas — because normal review settings implicitly
penalize criticizing colleagues' work, and this reframes the criticism as
aimed at a fictional external attacker instead.

**(c) Output/finding it produces.** A ranked list of the most credible
attack vectors a real competitor would use, translated into both defensive
countermeasures and, often, previously invisible strengths ("we have
already unconsciously blocked one of the plausible attacks").

**(d) Citation.**
[Lisa Bodell, "Want Your Business To Survive The Next Five Years? Kill Your
Company Now," *Forbes*, August 31, 2018](https://www.forbes.com/sites/lisabodell/2018/08/31/kill-your-company-with-lisabodell/) —
**why authoritative:** Bodell is the exercise's originator (CEO of
futurethink, author of *Kill the Company*), writing in her own Forbes
column; this is the primary description of the method by its creator, not
a third-party paraphrase. *Verified by direct fetch, quotes above extracted
from the live article.*

[Andrew S. Grove, *Only the Paranoid Survive: How to Exploit the Crisis
Points that Challenge Every Company and Career* (Currency Doubleday,
1996)](https://www.penguinrandomhouse.com/books/72469/only-the-paranoid-survive-by-andrew-grove/) —
**why authoritative:** Grove was CEO of Intel and wrote the book directly
from lived experience of a strategic inflection point (Intel's exit from
memory chips); "strategic inflection point" is his coinage and remains the
standard vocabulary for this class of self-disruption threat. *Publisher
bibliographic page verified live; the exact quote above was independently
verified against a quotation archive citing the same book and phrasing —
treat the quote as verified-by-cross-reference rather than direct
primary-text fetch, since the book itself is not hosted openly.*

[Joseph L. Bower and Clayton M. Christensen, "Disruptive Technologies:
Catching the Wave," *Harvard Business Review* 73, no. 1 (January–February
1995): 43–53](https://hbr.org/1995/01/disruptive-technologies-catching-the-wave) —
**why authoritative:** this is the original HBR article that introduced
the disruptive-innovation thesis — the theoretical explanation for *why*
"kill the company" exercises find real blind spots (leaders rationally
under-invest against threats that look unprofitable/low-end until it's too
late). *Article's existence and thesis statement verified by direct fetch
of the introduction; the full article body is paywalled beyond the opening
section and was not independently confirmed.*

> **Applied to a technical/AI-agent product.** Run this as: "If you were a
> well-funded team building a competing agent-sidecar tool, with full
> knowledge of this codebase's public design, what's the fastest way to
> make ours irrelevant?" Common answers in this space: ship the same recon
> capability natively inside the base agent loop (obsoleting the sidecar
> pattern entirely), undercut on latency, or exploit any place the sidecar's
> output format doesn't compose cleanly with the agent's existing tool
> calls. Applied to a north-star doc: ask what a competitor's north-star doc
> would say about *this* framework's stated differentiators — if theirs
> reads just as convincingly, the differentiation claim is unproven prose,
> not evidence.

---

## 6. Failure-Mode & Assumption Attack ("What Would Have to Be True" / Key Assumptions Check)

**(a) What it is.** Rather than arguing about what *is* true (which
degenerates into dueling data selection), this family of techniques makes
a team agree first on the **logical structure** of what would have to be
true for a plan to work, and only then goes looking for evidence. It
surfaces load-bearing assumptions explicitly — including the unconscious,
"never examined" ones — so the weakest one can be attacked directly instead
of hiding inside a plausible-sounding narrative.

> "The question is not concerned with what is true, or what could be true,
> but rather what would have to be true for the strategy to work as you
> envision it... teams can almost always agree on the logic of choices.
> They often have differing views as to what they think the data does
> show."
> — Roger Martin, "What Would Have to Be True?," Medium.

**(b) Step-by-step procedure** — two complementary versions:

*Roger Martin's WWHTBT (strategy version, from *Playing to Win*):*
1. For each strategic option under consideration, **reverse-engineer the
   logic**: lay out what conditions would have to hold for this option to
   be a great choice — separately from whether you currently believe those
   conditions hold.
2. **Reach agreement on the logic first**, before touching data — Martin's
   explicit warning is not to unilaterally pick "the data you think you
   should use"; get the team to agree on the WWHTBT logic, then jointly
   design the test.
3. **Identify the barriers to choice (BTC)**: which of the "must be true"
   conditions feel *least* likely to be true right now? Those are exactly
   the assumptions to attack first.
4. **Monitor continuously**: treat the WWHTBT list as a "canary in the coal
   mine" — revisit it against real-world data on a standing cadence, not
   just at the strategy off-site. "If they aren't [holding], start
   reviewing and revising your strategy immediately."

*CIA/UFMCS Key Assumptions Check (analytic version — a four-step process):*
1. **Write down the current line/plan** as it stands, for all to see.
2. **Articulate every premise**, stated and unstated, that must be true for
   that line to be valid.
3. **Challenge each assumption**: ask why it "must" be true, and whether it
   remains valid under all conditions, not just the current ones.
4. **Refine the list to only the assumptions that truly must hold**, and
   name the conditions or new information under which each could break.

**(c) Output/finding it produces.** A short, ranked list of load-bearing
assumptions, each tagged with (i) confidence level, (ii) what would
undermine it, and (iii) whether it is currently being tracked at all — the
handbook explicitly flags "identifying hidden assumptions" as one of the
hardest parts, since by definition they are "ideas held — often
unconsciously — to be true, and therefore seldom examined."

**(d) Citation.**
[Roger Martin, "What Would Have to Be True?," Medium](https://rogermartin.medium.com/what-would-have-to-be-true-83dac5bd2189) —
**why authoritative:** Martin co-created the Playing to Win strategy
methodology (with A.G. Lafley, former P&G CEO) that made WWHTBT a standard
strategy-consulting tool; this is his own explanation of the technique, not
a secondhand summary. *Verified by direct fetch, including the "barriers to
choice" and "canary in the coal mine" framing quoted above.*

Underlying framework: [A.G. Lafley and Roger L. Martin, *Playing to Win: How
Strategy Really Works* (Harvard Business Review Press,
2013)](https://store.hbr.org/product/playing-to-win-how-strategy-really-works/10739) —
**why authoritative:** the book-length treatment from which the WWHTBT tool
originates, co-authored by a Fortune-50 CEO who used it in practice.
*Publisher landing page cited for bibliographic verification; book content
not independently fetched.*

[UFMCS *Red Team Handbook*, "Key Assumptions Check," pp. 151–153](https://newandimproved.com/wp-content/uploads/2014/04/ufmcs_red_team_handbook_apr2011.pdf) —
same handbook and rationale as §§2–3; this specific four-step method and
the "seldom examined and almost never challenged" quote were extracted
directly from the source PDF.

> **Applied to a technical/AI-agent product.** Take the product's core
> value proposition and write its WWHTBT list explicitly: "For this to be
> worth an agent's tool-call budget, it would have to be true that (1) the
> agent's base context window/search is measurably worse without it, (2)
> the recon it returns is faster to act on than the agent doing its own
> exploration, (3) [etc.]." Then rank which of those you have *actually
> measured* versus merely assumed — an eval program that measures (1) but
> has never checked (2) has an unexamined load-bearing assumption. For a
> north-star doc, apply the Key Assumptions Check line-by-line to its
> stated differentiators — most wishful-thinking failures in strategy docs
> are exactly "ideas held unconsciously to be true, and therefore seldom
> examined."

---

## 7. Cognitive Biases That Corrupt Product Audits

**(a) What it is.** Structural interventions (§§1–6) work by *routing
around* known failure modes in human judgment. This section names the
failure modes directly, because a facilitator who doesn't recognize them in
the room will silently let them win even inside a well-designed process.

- **Confirmation bias** — "the seeking or interpreting of evidence in ways
  that are partial to existing beliefs, expectations, or a hypothesis in
  hand." Nickerson's exhaustive review argues it may by itself "account for
  a significant fraction of the disputes, altercations, and
  misunderstandings that occur among individuals, groups, and nations," and
  — critically for an audit process — that people do not naturally adopt a
  *falsifying* strategy: "we seldom seem to seek evidence naturally that
  would show a hypothesis to be wrong." His debiasing recommendations,
  directly actionable in an audit: (i) explicitly awareness-train the bias
  itself; (ii) force reasons *against* a judgment, not just for it; (iii)
  train people to generate the complement of a hypothesis early, before
  they've anchored on the first one that came to mind.

- **Sunk cost effect / escalation of commitment** — "a greater tendency to
  continue an endeavor once an investment... has been made," driven
  (per Arkes & Blumer's field and lab studies) by "the desire not to appear
  wasteful," not by rational forward-looking analysis. Their experiments
  showed people who had sunk more cost into a project *inflated their
  estimate of how likely it was to succeed* relative to people evaluating
  the identical project with no sunk cost — meaning sunk cost doesn't just
  bias the "continue or stop" decision, it corrupts the audit's own
  probability estimates.

- **Narrative fallacy / survivorship bias** — the tendency to "favor the
  visible, the embedded, the personal, the narrated, and the tangible" and
  to construct a compelling causal story around outcomes that were
  substantially luck (Taleb). Survivorship bias is the specific failure of
  studying only the winners: Abraham Wald's WWII aircraft-armor analysis is
  the canonical debiasing example — the planes that *didn't* return carried
  the real information, and were structurally invisible to an audit that
  only looks at survivors. In a product audit, this shows up as "look how
  well our launched features did" without a parallel look at what got
  killed and why, or at competitors who tried the identical strategy and
  failed.

- **Planning fallacy / optimism bias** — "the tendency to overestimate
  benefits and underestimate costs, impelling people to begin risky
  projects" that they would decline if forecast accurately (Kahneman,
  building on Kahneman & Tversky's 1979 distinction between an "inside
  view," reasoning from the specifics of *this* plan, and an "outside
  view," reasoning from the base rate of how similar plans actually went).
  This is the bias the pre-mortem (§1) is specifically engineered to defeat
  — it's why Kahneman discusses Klein's method approvingly in the same
  chapter of *Thinking, Fast and Slow* that introduces the planning
  fallacy.

- **"Resulting"** (Annie Duke) — evaluating the *quality of a decision* by
  the *quality of its outcome*, ignoring the role of luck. "People
  generally way too tightly link the quality of outcomes with the quality
  of decisions... particularly [when] evaluating the outcomes of others."
  In an audit context, resulting corrupts backward-looking self-assessment:
  a good decision that got a bad break gets over-punished, and a reckless
  decision that got lucky gets over-praised, meaning "look at what worked"
  is not a reliable audit signal on its own.

- **Ladder of inference** (Argyris/Senge) — the mechanism by which a bias
  becomes invisible in the room: people move from raw observable data, up
  through *selected* data, added meaning, assumptions, conclusions, and
  beliefs, to action — each rung reflexively feeding back to make the next
  round of data selection more biased. The debiasing move is to make people
  walk back down their own ladder in a review: "what data, specifically, is
  this conclusion built on, and what did we not select?"

**(b) Procedure for using this section in an audit:** name the bias
*before* it operates, not after. Practically: at the start of any audit
session, explicitly assign someone the job of watching for each of these
six patterns in real time and calling them out by name when they appear
("that's resulting" / "that's sunk cost") — naming a bias out loud measurably
reduces its grip more than a general awareness of biases in the abstract
(Nickerson's own conclusion: "a critical step in dealing with any type of
bias is recognizing its existence").

**(c) Output/finding it produces.** Not a finding about the product —
a finding about the *audit itself*: a log of moments where the audit's own
reasoning was compromised, which should be reported alongside the product
findings so a reader can discount appropriately.

**(d) Citations.**
[Raymond S. Nickerson, "Confirmation Bias: A Ubiquitous Phenomenon in Many
Guises," *Review of General Psychology* 2, no. 2 (1998):
175–220](https://pages.ucsd.edu/~mckenzie/nickersonConfirmationBias.pdf) —
**why authoritative:** the definitive, most-cited academic review
synthesizing the entire confirmation-bias literature across contexts, not a
single study. *Verified by direct full-text extraction; quotes above pulled
directly from the source.*

[Hal R. Arkes and Catherine Blumer, "The Psychology of Sunk Cost,"
*Organizational Behavior and Human Decision Processes* 35 (1985):
124–140](http://www.communicationcache.com/uploads/1/0/8/8/10887248/the_psychology_of_sunk_cost.pdf) —
**why authoritative:** the original controlled experimental demonstration
of the sunk cost effect, distinguishing it from related but distinct
phenomena like escalation of commitment; still the standard citation three
decades later. *Verified by direct full-text extraction.*

[Nassim Nicholas Taleb, *Fooled by Randomness* (2001) / *The Black Swan*
(2007)], via Farnam Street, ["Survivorship
Bias"](https://fs.blog/survivorship-bias/) —
**why authoritative:** Taleb coined "narrative fallacy" as a term of art and
made survivorship bias central to his broader critique of how experts
misread randomness; the Farnam Street page is used here as a verified
secondary source carrying a direct primary quote ("We favor the visible,
the embedded, the personal, the narrated, and the tangible; we scorn the
abstract"). *Farnam Street page verified by direct fetch; original book
text not independently fetched — quote verified via this intermediary only.*

[Daniel Kahneman, *Thinking, Fast and Slow* (Farrar, Straus and Giroux,
2011), ch. 23–24] and [Daniel Kahneman and Amos Tversky, "Intuitive
Prediction: Biases and Corrective Procedures," *TIMS Studies in Management
Science* 12 (1979): 313–327](https://apps.dtic.mil/sti/tr/pdf/ADA047747.pdf) —
**why authoritative:** Kahneman & Tversky's 1979 paper is the original
source of the planning fallacy and the inside-view/outside-view
distinction; Kahneman's 2011 book is the Nobel laureate's own popular
synthesis, in the same chapter that discusses Klein's pre-mortem
approvingly, making it the natural bridge citation between §1 and this
section. *1979 paper URL is a DTIC (Defense Technical Information Center)
government mirror — existence confirmed via search index; not independently
fetched in this session. Book content verified via cross-referenced
excerpt sources quoting identical passages, not fetched from a primary
hosted copy.*

[Annie Duke, *Thinking in Bets: Making Smarter Decisions When You Don't
Have All the Facts* (Portfolio, 2018)], quoted on [her own
site](https://www.annieduke.com/how-to-make-decisions-like-a-poker-champ/) —
**why authoritative:** Duke, a professional-poker-player-turned-
decision-scientist, coined "resulting" as the specific term for outcome/
decision-quality conflation and is the primary popularizer of the
decision-quality-vs-outcome-quality distinction for a general audience.
*Verified by direct fetch of the author's own site, including the
chess-vs-poker framing quoted above.*

[Chris Argyris and Peter Senge, *The Fifth Discipline* (1990); explained by
Action Design](https://actiondesign.com/resources/readings/ladder-of-inference) —
**why authoritative:** Argyris originated the ladder of inference as part
of his "action science"/double-loop-learning research program; Action
Design is a consultancy built directly on Argyris's own organizational-
learning methodology (not a generic secondary summarizer). *Verified by
direct fetch, including the rung-by-rung structure and double-loop-learning
framing quoted above.*

> **Applied to a technical/AI-agent product.** Confirmation bias shows up
> as "our eval was designed by the team that built the feature" —
> structurally the same failure the CIA's alternative-analysis discipline
> exists to prevent. Sunk cost shows up as continuing to trust a
> measurement stack because it took months to build, not because it's still
> the right one. Survivorship bias shows up as citing successful agent runs
> without looking at the runs where the agent silently ignored the tool.
> Resulting shows up as treating "the eval score went up" as proof the
> product change was good, without checking whether the eval itself moved
> for unrelated reasons. For a north-star doc specifically: optimism bias
> and the planning fallacy are close to the default failure mode of any
> "north star" document — it is, definitionally, an inside-view forecast —
> so it is exactly the kind of document a pre-mortem and an outside-view
> check (how have comparable past roadmaps in this space actually gone?)
> should be run against before it is trusted as a steering document.

---

## 8. Keeping Adversarial Critique Constructive

**(a) What it is.** Every method above can degrade into either (i)
performative contrarianism that produces no actionable signal, or (ii) a
demoralizing pile of ungraded criticism that reads as "everything is
broken" and therefore gets ignored. The fix is not less rigor — it is
*disciplined, ranked* rigor: separating strong disagreement from weak
disagreement, and separating severity from noise.

**(b) Step-by-step procedure:**

1. **Require refutation of the central point, not peripheral shots.** Paul
   Graham's hierarchy of disagreement, from weakest to strongest — name-
   calling (DH0), ad hominem (DH1), responding to tone (DH2), contradiction
   with no evidence (DH3), counterargument (DH4), refutation with evidence
   and direct quotation (DH5), and **refuting the central point** (DH6) —
   gives a facilitator an explicit bar: any adversarial finding that lands
   below DH4 should be discarded from the audit output, not because it's
   rude, but because it's low-information. "The most convincing form of
   disagreement is refutation. It's also the rarest, because it's the most
   work."
2. **Make the adversary role confrontational to the plan, not to people.**
   The UFMCS handbook states this as an explicit design principle: "Red
   Teaming is confrontational — challenging existing thought processes and
   estimates **without being confrontational to individuals or staffs.**"
   This is a facilitation rule, not a nicety: it is what keeps the exercise
   repeatable — a team that feels personally attacked will route around the
   process next time, or stop volunteering real information (exactly the
   failure the pre-mortem's silent-writing step exists to prevent, see §1).
3. **Score severity × likelihood, not vibes.** Borrow the standard
   risk-matrix discipline used in aerospace/defense risk management: rate
   each finding independently on its probable **impact if true** and its
   **likelihood of being true**, and let the product of the two — not the
   volume of discussion it generated — set the priority order.
4. **Route findings through a decision, every time.** Every finding gets
   one of three dispositions, mirroring the UFMCS Devil's Advocacy outcome
   set (§3): **(i) confirmed sound** — no action, but document that it was
   checked; **(ii) needs targeted work** — a scoped fix, owned and dated;
   **(iii) invalidates the plan** — escalate, don't bury.
5. **Report negative and null results as findings, not as failures of the
   audit.** A red-team pass that finds nothing wrong and can show its work
   is exactly as valuable as one that finds three critical flaws — treat
   "we tried hard to break this and couldn't" as a citable, positive
   finding, not an awkward non-event to omit.

**(c) Output/finding it produces.** A severity-ranked, disposition-tagged
list where every entry has (i) the strongest form of the argument against
it, per Graham's hierarchy, (ii) an impact × likelihood score, and (iii) an
explicit next action or an explicit "checked, sound" verdict — the opposite
of an undifferentiated pile of complaints.

**(d) Citation.**
[Paul Graham, "How to Disagree," paulgraham.com,
2008](https://www.paulgraham.com/disagree.html) —
**why authoritative:** Graham's hierarchy is the most widely adopted,
citable taxonomy of argument quality in general use (originating with a
working essayist/investor known for precise, load-bearing prose, not an
academic committee — but it is now standard reference vocabulary,
including in academic and journalistic contexts on argument quality).
*Verified by direct fetch, all seven levels and the "refutation... rarest,
because it's the most work" quote extracted from the primary source.*

[UFMCS *Red Team Handbook*, "Summary," p. 11](https://newandimproved.com/wp-content/uploads/2014/04/ufmcs_red_team_handbook_apr2011.pdf) —
same handbook as §§2–3, §6; the "confrontational... without being
confrontational to individuals" principle is quoted directly from the
source PDF's own summary of red-team fundamentals.

[NASA, *NASA Risk Management Handbook*, NASA/SP-2011-3422, Version 1.0,
November 2011](https://www.nasa.gov/wp-content/uploads/2023/08/nasa-risk-mgmt-handbook.pdf) —
**why authoritative:** NASA's own official risk-management doctrine,
codifying the likelihood × consequence/severity matrix used across
aerospace and defense programs where getting risk prioritization wrong is
catastrophic and expensive; a rigor-tested standard, not an invented
scoring scheme. *Verified by direct full-text extraction, confirming the
5×5 likelihood/severity matrix structure referenced (handbook §4.3.2.1,
"Likelihood and Severity Ranking").*

> **Applied to a technical/AI-agent product.** Any adversarial finding
> against an eval methodology or a north-star claim should be written up to
> DH5/DH6 standard — quote the specific claim, name the specific flaw —
> before it's allowed into a report; "this feels overconfident" without a
> quoted passage is a DH2 and should be rewritten or dropped. Confrontation
> is aimed at the doc/the metric/the plan, explicitly not at whoever wrote
> it — a north-star doc's author should be the one running the pre-mortem
> on their own doc, per the visibility/truthfulness discipline of
> committing negative results rather than only positive ones.

---

## Source Ledger

| # | Source | Type | Method(s) it supports | Verification |
|---|---|---|---|---|
| 1 | [Gary Klein, "Performing a Project Premortem," *HBR*, Sept 2007](https://hbr.org/2007/09/performing-a-project-premortem) | Primary (practitioner article) | §1 Pre-mortem | Full text verified via direct fetch (PDF mirror) |
| 2 | [Mitchell, Russo & Pennington, "Back to the Future," *J. Behavioral Decision Making* 2 (1989)](https://onlinelibrary.wiley.com/doi/abs/10.1002/bdm.3960020103) | Primary (peer-reviewed) | §1 Pre-mortem | Citation/abstract confirmed; full text paywalled (unverified) |
| 3 | [Daniel Kahneman, *Thinking, Fast and Slow* (2011)] | Primary (book) | §1, §7 | Content cross-verified via multiple excerpt sources; no open primary URL |
| 4 | [Kahneman & Tversky, "Intuitive Prediction," *TIMS Studies in Mgmt. Sci.* 12 (1979)](https://apps.dtic.mil/sti/tr/pdf/ADA047747.pdf) | Primary (peer-reviewed) | §7 Planning fallacy | URL located via search index; not independently fetched this session |
| 5 | [Bryce Hoffman, *Red Teaming* (2017)](https://brycehoffman.com/books/red-teaming/) | Primary (author's site) | §2 Red-teaming | Verified via direct fetch |
| 6 | [UFMCS, *Red Team Handbook*, v.5, 2011](https://newandimproved.com/wp-content/uploads/2014/04/ufmcs_red_team_handbook_apr2011.pdf) | Primary (official doctrine) | §2, §3, §6, §8 | Full text verified via direct extraction (7,882 lines) |
| 7 | [Richards Heuer, *Psychology of Intelligence Analysis*, CIA CSI (1999)](https://www.cia.gov/resources/csi/books-monographs/psychology-of-intelligence-analysis-2/) | Primary (official) | §2 Red-teaming | Page live-verified (HTTP 200); body text JS-rendered/unverified, [archive.org mirror](https://archive.org/details/PsychologyOfIntelligenceAnalysis) available |
| 8 | [Mind Collection, "The Tenth Man Rule"](https://themindcollection.com/the-tenth-man-rule-devils-advocacy/) | Secondary (synthesis) | §3 Devil's advocate | Verified via direct fetch |
| 9 | [Wikipedia, "Devil's Advocate Unit"](https://en.wikipedia.org/wiki/Devil%27s_Advocate_Unit) | Tertiary (cross-check) | §3 Devil's advocate | Verified via direct fetch |
| 10 | [Siman-Tov et al., "The Devil's Advocate in Intelligence," *Intelligence and National Security* 33:6 (2018)](https://www.tandfonline.com/doi/full/10.1080/02684527.2018.1470062) | Primary (peer-reviewed) | §3 Devil's advocate | Citation confirmed; full text HTTP 403 paywalled (unverified) |
| 11 | [Shane Parrish, "Inversion," Farnam Street](https://fs.blog/inversion/) | Secondary (curated synthesis of Munger/Jacobi) | §4 Inversion | Verified via direct fetch |
| 12 | [Lisa Bodell, "Kill Your Company Now," *Forbes*, 2018](https://www.forbes.com/sites/lisabodell/2018/08/31/kill-your-company-with-lisabodell/) | Primary (creator's own article) | §5 Kill the company | Verified via direct fetch |
| 13 | [Andy Grove, *Only the Paranoid Survive* (1996)](https://www.penguinrandomhouse.com/books/72469/only-the-paranoid-survive-by-andrew-grove/) | Primary (book) | §5 Kill the company | Publisher page verified; quote verified via cross-reference, not primary text fetch |
| 14 | [Bower & Christensen, "Disruptive Technologies," *HBR*, Jan–Feb 1995](https://hbr.org/1995/01/disruptive-technologies-catching-the-wave) | Primary (peer academic/practitioner) | §5 Kill the company | Thesis/intro verified via fetch; full body paywalled (unverified) |
| 15 | [Roger Martin, "What Would Have to Be True?," Medium](https://rogermartin.medium.com/what-would-have-to-be-true-83dac5bd2189) | Primary (creator's own writing) | §6 Assumption attack | Verified via direct fetch |
| 16 | [Lafley & Martin, *Playing to Win* (2013)](https://store.hbr.org/product/playing-to-win-how-strategy-really-works/10739) | Primary (book) | §6 Assumption attack | Publisher landing page cited; book text not fetched |
| 17 | [Raymond Nickerson, "Confirmation Bias," *Review of General Psychology* 2:2 (1998)](https://pages.ucsd.edu/~mckenzie/nickersonConfirmationBias.pdf) | Primary (peer-reviewed review) | §7 Biases | Full text verified via direct extraction |
| 18 | [Arkes & Blumer, "The Psychology of Sunk Cost," *OBHDP* 35 (1985)](http://www.communicationcache.com/uploads/1/0/8/8/10887248/the_psychology_of_sunk_cost.pdf) | Primary (peer-reviewed) | §7 Biases | Full text verified via direct extraction |
| 19 | [Taleb, *Fooled by Randomness* / *The Black Swan*, via Farnam Street "Survivorship Bias"](https://fs.blog/survivorship-bias/) | Secondary carrying primary quote | §7 Biases | Farnam St. page verified via fetch; book not independently fetched |
| 20 | [Argyris & Senge, *The Fifth Discipline* (1990), via Action Design](https://actiondesign.com/resources/readings/ladder-of-inference) | Secondary (practice built on primary author's own method) | §7 Biases | Verified via direct fetch |
| 21 | [Annie Duke, *Thinking in Bets* (2018), via annieduke.com](https://www.annieduke.com/how-to-make-decisions-like-a-poker-champ/) | Primary (author's own site) | §7 Biases | Verified via direct fetch |
| 22 | [Paul Graham, "How to Disagree" (2008)](https://www.paulgraham.com/disagree.html) | Primary (author's own site) | §8 Constructive critique | Verified via direct fetch |
| 23 | [NASA, *NASA Risk Management Handbook*, SP-2011-3422 (2011)](https://www.nasa.gov/wp-content/uploads/2023/08/nasa-risk-mgmt-handbook.pdf) | Primary (official standard) | §8 Constructive critique | Full text verified via direct extraction |
| 24 | [Shreyas Doshi, "Pre-mortems," Coda](https://coda.io/@shreyas/pre-mortems) | Primary (creator's own template) | §1 Pre-mortem (modern variant) | Verified via direct fetch |

**24 distinct sources**, spanning peer-reviewed research (5), official
government/military doctrine (3), primary practitioner books/articles by
the methods' originators (11), and carefully-sourced secondary synthesis
used only where a primary source was not independently fetchable (5).

**Methods that could not be fully source-verified at the deep-link level**
(flag per sourcing discipline, not excluded — each still has a legitimate,
resolvable citation, just not independently confirmed by direct fetch of
full body text in this session):
- Mitchell, Russo & Pennington (1989) — Wiley paywall (HTTP 402).
- Kahneman & Tversky (1979) — DTIC mirror located but not fetched.
- Kahneman, *Thinking, Fast and Slow* — no open full-text host exists; verified only via consistent cross-quotation.
- Siman-Tov et al. (2018) — Taylor & Francis paywall (HTTP 403).
- Bower & Christensen (1995) — HBR paywall beyond the introduction.
- Andy Grove's exact quote — verified via a quotation archive rather than the book's primary text directly.

---

## Adversarial Audit Protocol

A runnable sequence combining the above into one honest self-attack pass on
a product or a strategy document. Budget: a half-day for a focused product
area, a full day for a strategy/north-star document. Run it with the
people closest to the work in the room, plus at least one person with no
stake in the outcome.

### Sequence

1. **Frame (15 min).** State the plan/doc under audit in one paragraph,
   plain language, no marketing gloss. Assign roles: a facilitator, a
   scribe, and one rotating **devil's advocate** whose job this session is
   to build the strongest case against the plan (§3).

2. **Key Assumptions Check (30–45 min).** Before attacking anything, list
   every assumption the plan depends on, stated and unstated. Rank by
   confidence and identify the barriers to choice — the least-confident,
   most load-bearing assumptions (§6).

3. **Pre-mortem (30–45 min).** Declare the plan has already failed
   spectacularly. Silent individual writing, then round-robin, no
   rebuttal during the round (§1).

4. **Inversion pass (15–20 min).** Ask explicitly: "What would we do if we
   wanted this to fail?" Convert the answers into standing constraints
   (§4).

5. **Kill-the-plan / attacker simulation (30–45 min).** Role-play as a
   well-resourced competitor or adversary. Rank threats; extract both
   countermeasures and previously invisible strengths (§5).

6. **Devil's advocacy on the single most load-bearing claim (20–30 min).**
   The assigned devil's advocate presents the strongest evidence-backed
   case against the plan's central thesis — refutation-level (Graham DH5/
   DH6), not tone-policing (§3, §8).

7. **Bias check (ongoing + 10 min retrospective).** Someone was assigned to
   flag confirmation bias, sunk cost, resulting, and survivorship bias by
   name as they occurred during steps 2–6. Read that log back to the group
   (§7).

8. **Score and disposition (20–30 min).** Every finding generated in steps
   2–6 gets scored and dispositioned per the scheme below. No finding
   leaves the room unscored.

9. **Report, including negative results (15 min).** Write up what was
   checked and found sound, not only what was found broken. A clean pass
   on a load-bearing assumption is a citable finding.

### Severity / Priority Scheme

Score every finding on two independent axes, then take the product to set
priority — the standard likelihood × impact discipline (NASA/DoD 5×5 risk
matrix, adapted to a small-team scale as a 3×3):

| | **Impact: Low** (annoying, recoverable) | **Impact: Medium** (materially undermines a claim or a user's trust) | **Impact: High** (invalidates the plan/product thesis if true) |
|---|---|---|---|
| **Likelihood: Low** (plausible but speculative) | Log, no action | Track, revisit next audit | Escalate — verify before dismissing |
| **Likelihood: Medium** (some evidence) | Track | Scoped fix, owned + dated | Escalate now |
| **Likelihood: High** (strong evidence or already observed) | Scoped fix | Escalate now | Stop and resolve before proceeding |

Every scored finding also gets one of the three UFMCS-style dispositions
(§3, §8):

- **Confirmed sound** — attacked directly, held up; document how, so it
  isn't re-litigated from scratch next audit.
- **Needs targeted work** — a scoped, owned, dated fix; not a vague
  "someone should look at this."
- **Invalidates the plan** — this is not a backlog item, it's a decision
  point; escalate to whoever owns the plan, don't let it die quietly in a
  findings doc.

Findings that only reach Paul Graham's DH0–DH3 (name-calling, ad hominem,
tone, unsupported contradiction) are dropped from the report entirely —
they are noise, and including them dilutes the credibility of the DH5/DH6
findings that are real. The point of running all eight methods in sequence
is not to generate the largest possible pile of criticism; it's to make
sure the criticism that *does* survive to the report is the criticism a
smart, motivated adversary — or reality — would actually find first.
