# Research 02 — Clean Architecture, DDD & Reusable-Module Canon

> Research provenance for the **architecture-audit** skill (skill-forge Round 0).
> **Angle:** clean architecture, DDD, design patterns, and reusable capability/core modules.
> **Not the skill.** This file is sourced raw material an author distills into the skill.
> **Date:** 2026-07-07 · **Method:** text-only (WebSearch/WebFetch), every deep URL verified unless flagged in *Degradations*.

## How to read this document

- **[SOURCED]** — a claim backed by a specific deep URL that was fetched and its content confirmed. Each carries a one-line rationale (why authoritative + why it matters for ghx) and a **Ghx-apply** line (the concrete audit question it produces).
- **[SYNTHESIS]** — my own connective tissue / re-pricing. Not from a single source; labeled so an author can challenge it.
- The **Source Ledger** at the end lists every URL, its authority, and whether it verified.

The auditor's job this research arms: judge ghx's **domain modeling**, **encapsulation**, **module/core boundaries**, and **reuse-vs-YAGNI** — concretely, *is there a genuine reusable core between the evals machinery (SAFE) and the product (SAF/ghx), and is a capability like the Agent Sidecar Framework a clean module or a leaky god-package?*

---

## Executive summary

The canon an architecture auditor applies to domain modeling and reuse resolves into **five load-bearing ideas plus one indispensable counter-canon**:

1. **Direction of dependency is the whole game.** Clean Architecture's Dependency Rule and Cockburn's Ports & Adapters say the same thing from two angles: business policy at the center depends on nothing; frameworks, transports, and databases are outer detail that depend inward through interfaces. An auditor traces the *import graph* and asks whether the core (ghx: `internal/ghx`, `internal/sidecar/telemetry`) is genuinely dependency-free or whether transport/CLI/eval concerns have leaked inward.

2. **Domain modeling is encapsulation with a name.** DDD tactical patterns — value objects, entities, aggregates, ubiquitous language — are the concrete form of "define proper domain models" and "one authoritative owner per domain concern" (ghx's own AGENTS.md tenet). The aggregate root *is* the "single owner" rule; the value object *is* the cure for the "plain maps passed around" anti-pattern the tenet forbids.

3. **A reusable core is a Shared Kernel, and Shared Kernels are expensive on purpose.** Evans' Shared Kernel is the exact pattern for "a reusable core shared between evals and product": designate a *small* explicit subset both sides depend on, and coordinate every change. Uncle Bob's component-cohesion principles (REP/CCP/CRP) price the tension precisely — reuse and change-locality pull a module *bigger*, minimal-coupling pulls it *smaller*, and the architect must sit deliberately in that tension, not drift.

4. **Module boundaries must be enforced by the compiler, not by discipline.** The modular-monolith canon (Brown) and Go-native package design (Cheney, the Go blog, Ben Johnson) agree: a "big ball of mud" is what you get when boundaries rely on good intentions. In Go the enforcement mechanisms are package visibility, an acyclic import graph, and a root/domain package with zero dependencies.

5. **Most "design patterns" are Go noise; a few are signal.** GoF patterns are frequently workarounds for missing language features (Norvig); in Go, first-class functions, interfaces, and embedding dissolve Strategy/Command/Template-Method/Builder into idiom (functional options, `io.Reader`). An auditor rewards small interfaces defined at the *consumer* and penalizes Java-style pattern ceremony.

6. **The counter-canon is non-negotiable: reuse has a wrong side.** YAGNI, "the wrong abstraction," AHA (Avoid Hasty Abstractions), and the Go proverb *"a little copying is better than a little dependency"* exist to stop speculative reuse. **Duplication is cheaper than the wrong abstraction.** An auditor must be able to say *"this shared core is premature/gold-plated — inline it"* as readily as *"this is duplicated — extract it."* Judging reuse means holding both edges at once.

The through-line for ghx: the north star already commits to a reusable core (`internal/ghx` behind every frontend; `internal/sidecar/telemetry` shared by SAF runtime and SAFE evals). The canon below turns that commitment into *checkable* questions — and gives the auditor the counter-canon to catch a shared core that should have stayed two copies.

---

## Cluster A — Direction of dependency (the boundary rule)

### A1. The Dependency Rule — inner circles know nothing of outer circles [SOURCED]
Robert C. Martin, *The Clean Architecture*: **"Source code dependencies can only point inwards. Nothing in an inner circle can know anything at all about something in an outer circle."** Layers: Entities (enterprise rules) → Use Cases (application rules) → Interface Adapters → Frameworks & Drivers. The system is independent of frameworks, UI, database, and any external agency.
- **URL:** https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html
- **Why it matters:** the single most-cited statement of the rule an auditor traces; the "independent of framework/DB/UI" test is the reuse test in disguise.
- **Ghx-apply:** does `internal/ghx` (core) import nothing from `internal/cli`, `internal/sidecar`, or ACP? A single inward-pointing arrow from core → transport is the canonical violation. The north star's own rule ("core capabilities live in `internal/ghx`; frontends wrap the same core") *is* the Dependency Rule; the audit verifies the code obeys the doc.

### A2. Ports & Adapters — one core, many drivers, testable in isolation [SOURCED]
Alistair Cockburn, *Hexagonal Architecture*, intent: **"Allow an application to equally be driven by users, programs, automated test or batch scripts, and to be developed and tested in isolation from its eventual run-time devices and databases."** *Ports* are technology-agnostic conversation protocols (defined by their API); *adapters* convert between a port and a concrete technology. Symmetry: **primary/driving** adapters (UI, tests) trigger the app; **secondary/driven** adapters (DB, mocks) are called by it.
- **URL:** https://alistair.cockburn.us/hexagonal-architecture/
- **Why it matters:** the canonical frame for "the same core answers a CLI, an MCP tool, an eval harness, and a test" — exactly ghx's multi-frontend and SAF/SAFE shape.
- **Ghx-apply:** are CLI, MCP, codemode, sidecar, and the eval runner all *adapters* over one core port, or does each re-implement recon logic? If a test or eval must boot ACP to exercise core behavior, the port is missing (ties to ADR-0016.3's eval-isolation gap the north star already flags).

---

## Cluster B — Domain modeling & encapsulation (DDD tactical)

> ghx's AGENTS.md "Engineering Tenets" already mandate this ("Define proper domain models"; "Encapsulate domain logic in services — one authoritative owner per domain concern"). The DDD canon is the *named* version of those tenets, so the auditor can cite a standard, not a house rule.

### B1. Value Object — identity-free, immutable, kills primitive obsession [SOURCED]
Martin Fowler: value objects are equal **by attribute value, not identity**; should be **immutable** ("if I want to change my party date, I create a new object"); and are the cure for **primitive obsession** — replacing bare strings/ints with a type that carries validation and forbids nonsense operations.
- **URL:** https://martinfowler.com/bliki/ValueObject.html
- **Why it matters:** this is the standard behind AGENTS.md's "never plain maps/anonymous structs passed around with the same helper logic at every call site." A value object is the type that "makes the code understandable."
- **Ghx-apply:** are concepts like a repo reference, a backend/tier, an escalation level, a score, or a session id modeled as named types with methods and validation — or passed as `string`/`map[string]any` and re-parsed at each call site? Recurring hand-rolled parsing of the same primitive is the audit smell.

### B2. Entity vs. Value — identity is a modeling decision [SOURCED]
Fowler (same page) contrasts value objects with **reference objects/entities**, which have conceptual identity (a sales order keyed by order number) and are equal by identity, not attributes.
- **URL:** https://martinfowler.com/bliki/ValueObject.html
- **Why it matters:** confusing the two produces either accidental aliasing bugs (mutable value) or duplicated-identity bugs (value where an entity was needed).
- **Ghx-apply:** is a *Session* an entity (identity, lifecycle, resumable — NORTH_STAR §4) modeled as one, with a single owning store, or is session state a bag of fields copied between packages?

### B3. Aggregate — a consistency boundary with one root [SOURCED]
Fowler: an aggregate is **"a cluster of domain objects that can be treated as a single unit."** External references go **only to the root**, which enforces invariants; **transactions should not cross aggregate boundaries.**
- **URL:** https://martinfowler.com/bliki/DDD_Aggregate.html
- **Why it matters:** the aggregate root is the precise, canonical form of AGENTS.md's "one authoritative owner per domain concern (a single session store, a single report validator, a single telemetry writer)."
- **Ghx-apply:** is there exactly one owner that mutates a session / validates a report / writes telemetry, with everyone else going through it? Two packages that both write `~/.ghx` session artifacts, or two report-validation code paths, are aggregate-boundary violations.

### B4. Ubiquitous Language — the code's vocabulary is the domain's [SOURCED]
Fowler: a **"common, rigorous language between developers and users … based on the Domain Model used in the software"**; "software doesn't cope well with ambiguity," so code vocabulary must match the domain model.
- **URL:** https://martinfowler.com/bliki/UbiquitousLanguage.html
- **Why it matters:** it's the readability substrate; drift between doc vocabulary (ADRs, NORTH_STAR: *tier, escalation, anticipation, episode, gate, report*) and code identifiers is a real, checkable defect.
- **Ghx-apply:** do package/type/function names use the north-star vocabulary (does an "escalation tier" appear as a named type, or as an untyped int with magic values)? Divergence between the ADR corpus and the symbol names is a ubiquitous-language failure the auditor can name.

---

## Cluster C — Module / context boundaries & the reusable core (the heart of Goga's ask)

### C1. Bounded Context — one model per boundary, mapped at the edges [SOURCED]
Fowler: total unification of a large model "will not be feasible or cost-effective"; split into bounded contexts, each with its own coherent model and language, with **explicit mapping** where they meet. A shared term ("Customer", "Report") may legitimately be *two different models* across contexts.
- **URL:** https://martinfowler.com/bliki/BoundedContext.html
- **Why it matters:** the decision framework for "should the product (SAF) and the evals (SAFE) share a model, or are they two contexts that only *look* alike?" — the exact question behind Goga's reusable-core ask.
- **Ghx-apply:** are SAF (runtime) and SAFE (evals) one context sharing types, or two contexts that should map at an edge? A "trace" is claimed shared (NORTH_STAR M6: "a trace is a trace"); an "episode/reward/gate" is eval-only. The audit distinguishes what is genuinely one model from what is superficial name-overlap.

### C2. Shared Kernel — the canonical "reusable core between two teams", and its price [SOURCED]
DDD Context Mapping (ddd-crew, quoting Evans): **"Designate with an explicit boundary some subset of the domain model that the teams agree to share. Keep this kernel small."** Changes require inter-team consultation; the pattern is **tight coupling with high coordination cost** and cannot be changed unilaterally.
- **URL:** https://github.com/ddd-crew/context-mapping
- **Why it matters:** this is *exactly* the pattern Goga names — "a reusable core shared between the evals machinery and the main ghx product." The canon's two rules are the audit criteria: **keep it small**, and **make the coupling explicit and coordinated**.
- **Ghx-apply:** is `internal/sidecar/telemetry` (the SAF/SAFE shared trace/artifact substrate, ADR-0022) a *small, explicit* shared kernel — or has it grown to absorb eval-only or runtime-only concerns, making every change a two-front coordination? A shared core that keeps growing is a failing shared kernel; a "shared" core that each side has quietly forked is worse.
- **Companion patterns on the same page (also [SOURCED]):** *Anticorruption Layer* (isolate your model from an upstream one — relevant to wrapping ACP/codemap as adopted-not-owned tools), *Open Host Service* / *Published Language* (the boundary contract as a published protocol — the north star's "English question in → compact evidence report out"), *Conformist* and *Separate Ways* (when *not* to integrate).

### C3. Component Cohesion — REP / CCP / CRP and the tension you must sit in [SOURCED]
Robert C. Martin, *Clean Architecture* Part IV (verified via community summary; primary site down — see Degradations):
- **REP (Reuse/Release Equivalence):** things grouped into a component must be **releasable together** — the granularity of reuse is the granularity of release.
- **CCP (Common Closure):** **"Gather into components those classes that change for the same reasons and at the same times."** (component-level SRP)
- **CRP (Common Reuse):** don't force a consumer to depend on things it doesn't use. (component-level ISP)
- **The tension:** REP and CCP are **inclusive** (bigger components); CRP is **exclusive** (smaller). The architect chooses the balance deliberately and re-balances as the project matures.
- **URL:** https://github.com/serodriguez68/clean-architecture/blob/master/part-4-component-principles.md
- **Why it matters:** this is the *pricing model* for reuse. "Make a reusable core" is not free-standing good — it is REP/CCP pulling bigger, checked against CRP pulling smaller. An auditor who only knows "reuse good" will over-merge.
- **Ghx-apply:** does the proposed evals↔product core group things that *change together* (CCP) and that *every consumer needs* (CRP), or does it staple together an eval-release cadence and a product-release cadence that move independently (REP violation)? If evals churn weekly and the product ships monthly, a single shared component fails REP and the audit should recommend splitting.

### C4. Component Coupling — ADP / SDP / SAP (the graph must be acyclic and point at stability) [SOURCED]
Same source: **ADP** — no cycles in the component dependency graph; **SDP** — depend in the direction of stability (volatile depends on stable, never the reverse); **SAP** — a component should be as abstract as it is stable (stable components are stable *because* they're abstractions others depend on).
- **URL:** https://github.com/serodriguez68/clean-architecture/blob/master/part-4-component-principles.md
- **Why it matters:** turns "clean boundaries" into three mechanical checks an auditor can run against `go list`/import graphs.
- **Ghx-apply:** is the import graph acyclic (ADP)? Does the volatile sidecar/CLI depend on the stable core and never the reverse (SDP)? Is the most-depended-on package the most abstract/stable one, or a concrete grab-bag everything imports (SAP violation — the classic `internal/ghx` "does it depend on nothing" test)?

### C5. Modular Monolith — modular ≠ microservices; enforce with the compiler [SOURCED]
Simon Brown: a component is **"a grouping of related functionality behind a well-defined interface."** A monolith can be well-structured; the failure mode is the **big ball of mud**, which happens when boundaries rely on discipline. **"Lean on the compiler to enforce your architectural principles, rather than discipline, post-compilation tooling."**
- **URL:** https://simonbrown.je/modular-monolith/
- **Why it matters:** ghx *is* a modular monolith (one Go binary, internal packages). This is the target architecture, and the enforcement lesson is the audit lever.
- **Ghx-apply:** are module boundaries enforced by Go's package-visibility (`internal/`, unexported types) so a violation *won't compile* — or only by convention? Anything a reviewer must remember to police is, by Brown's argument, already decaying.

### C6. SOLID, repriced for Go — interfaces and packages, not class hierarchies [SOURCED]
Dave Cheney, *SOLID Go Design*: SRP at **package** granularity ("one reason to change"; reject `utils`/`common` dumping grounds); OCP via **embedding**; LSP via **interface satisfaction** ("require no more, promise no less", `io.Reader`); ISP → **small interfaces**, "accept interfaces, return structs" (attrib. Jack Lindamood); DIP → an **acyclic, flat import graph** where low-level packages depend on abstractions. Thesis: "interfaces let you apply the SOLID principles to Go."
- **URL:** https://dave.cheney.net/2016/08/20/solid-go-design
- **Why it matters:** the authoritative translation of SOLID out of Java/OO idiom into the language ghx is written in — so the audit judges Go, not imagined Java.
- **Ghx-apply:** are interfaces small and defined at the *consumer* (ISP/DIP), or large "manager" interfaces exported from the *provider*? Does any package named `util`/`common`/`helpers`/`types` exist (SRP smell)? Is "accept interfaces, return structs" honored at core boundaries?

### C7. Go package naming — the name is the API's first word [SOURCED]
Go blog (Sameer Ajmani): **"Packages named `util`, `common`, or `misc` provide clients with no sense of what the package contains"**; the same warning covers dumping all interfaces into one `api`/`types`/`interfaces` package ("growing without bound … accumulating dependencies"). The package name + identifier is the caller's vocabulary; design from the client's point of view.
- **URL:** https://go.dev/blog/package-names
- **Why it matters:** primary, official Go guidance that operationalizes CCP/SRP as a naming test — one an auditor can grep for.
- **Ghx-apply:** grep the tree for `util|common|misc|helpers|shared` package names; each is a candidate cohesion failure. Does `stringset.New()`-style clarity hold, or is there `util.DoThing()` ambiguity?

### C8. Standard Package Layout — domain types at the root, dependencies as adapters [SOURCED]
Ben Johnson: the **root/domain package holds only domain structs and the interfaces that operate on them and depends on nothing**; each external dependency (Postgres, HTTP, a cache) gets a **subpackage implementing those interfaces** (an adapter); **mocks** live in a shared subpackage; **`main` wires it all together**. This kills circular dependencies and god-packages: "your names are your best documentation."
- **URL:** https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1
- **Why it matters:** the concrete Go realization of Clean Architecture + Hexagonal, and the closest published pattern to ghx's stated `internal/ghx`-core / frontend-adapters shape.
- **Ghx-apply:** does `internal/ghx` behave like Johnson's root (domain types + interfaces, zero deps), with `internal/cli`, `internal/sidecar`, MCP, codemode as dependency-adapters? If the "core" imports a transport, or a frontend defines the domain types, the layout is inverted.

---

## Cluster D — Design patterns in Go: signal vs. noise

### D1. Functional options — the Go-native replacement for Builder/telescoping constructors [SOURCED]
Rob Pike, *Self-referential functions and the design of options*: pass variadic option-functions to a constructor instead of a config struct or many constructors; it "requires minimal API surface … scales without creating bloat as dozens of options accumulate," and "it's really nice to use from the point of view of the package's client."
- **URL:** https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
- **Why it matters:** the canonical example of a GoF-era pattern (Builder) *dissolving* into a Go idiom — directly relevant to ghx's configurable dials (depth, anticipation, budgets, per-scope config in NORTH_STAR §3).
- **Ghx-apply:** are ghx's many configurable knobs expressed as functional options / clear config types, or as telescoping constructors and boolean-parameter soup? The former is signal; the latter is the pattern the idiom was invented to kill.

### D2. Many GoF patterns are language workarounds [SOURCED, degraded — see Degradations]
Peter Norvig, *Design Patterns in Dynamic Languages*: a large share of the 23 GoF patterns are **"invisible or simpler"** once a language has first-class functions and types (Strategy, Command, Template Method, Observer, Visitor, Chain of Responsibility collapse into ordinary function values). *(Headline "16 of 23" is the widely-reported figure; I could not extract it from the primary artifact — flagged.)*
- **URL:** https://norvig.com/design-patterns/
- **Why it matters:** the intellectual license to *penalize* pattern ceremony — an auditor should not reward a `StrategyFactory` where a `func` would do.
- **Ghx-apply:** does any ghx package reimplement Strategy/Factory/Visitor ceremony that a first-class `func`, an interface, or a map-of-functions would express more plainly? Go's own stdlib (`http.HandlerFunc`, `sort.Slice`) is the bar.

### D3. Go proverbs — the taste rules for interfaces and reuse [SOURCED]
Rob Pike: **"The bigger the interface, the weaker the abstraction."** · **"`interface{}` says nothing."** · **"A little copying is better than a little dependency."** · **"Clear is better than clever."** · **"Make the zero value useful."**
- **URL:** https://go-proverbs.github.io/
- **Why it matters:** compact, quotable, load-bearing — several are direct audit heuristics (small interfaces; avoid `any`; prefer copying to premature coupling).
- **Ghx-apply:** grep for wide interfaces and `interface{}`/`any` at boundaries (weak abstraction); check whether a shared helper was worth the import it created ("a little copying" — see the counter-canon below); check that core types have useful zero values.

---

## Cluster E — The counter-canon (when reuse/abstraction is gold-plating)

> An auditor that only knows Clusters A–D will *over-build*. This cluster is what lets the auditor say "delete this abstraction." It carries equal weight. It also aligns with ghx's own north star — "getting rid of waste *is* getting better," "signal per token" — and AGENTS.md's "no legacy/compatibility junk … refactor cleanly or not at all."

### E1. YAGNI — don't build presumptive capability; the cost is carry, not just build [SOURCED]
Martin Fowler: don't build features you presume you'll need. Four costs of ignoring it: **build, delay, carry, repair.** The subtle one is **cost of carry** — speculative code "makes subsequent modifications harder and slower" for everyone, every day, until removed. Crucial caveat: **YAGNI applies to presumptive *features*, not to effort that makes software easier to modify** (refactoring, tests are not YAGNI violations).
- **URL:** https://martinfowler.com/bliki/Yagni.html
- **Why it matters:** the standard authority for cutting speculative reuse; the caveat stops the auditor from mislabeling healthy refactoring as YAGNI.
- **Ghx-apply:** is a "reusable core / framework" being built *ahead* of a second real consumer (speculative), or extracted *because* two consumers already exist (justified)? The Agent Sidecar Framework's "reusable infrastructure" is a NORTH_STAR *consequence product* explicitly gated ("only gets investment when the filter passes") — the audit checks that generality is not being paid for before it's needed.

### E2. The Wrong Abstraction — duplication is cheaper than the wrong abstraction [SOURCED]
Sandi Metz: **"prefer duplication over the wrong abstraction."** The decay sequence: a good abstraction meets a case that *almost* fits → someone adds a parameter/conditional → repeat → the abstraction becomes an incomprehensible tangle of vaguely-related concerns. Remedy: **inline it back to call sites, then re-abstract from what you now actually know.** Rewinding is "advance in a better direction," not retreat.
- **URL:** https://sandimetz.com/blog/2016/1/20/the-wrong-abstraction
- **Why it matters:** the canonical description of *how* a shared core rots — parameters and flags accreting to serve divergent callers. Directly the failure mode of a too-eager evals↔product core.
- **Ghx-apply:** does the shared telemetry/core carry `if eval { … } else { … }` branches or option flags that only one side uses? That is the wrong-abstraction signature; the canon's advice is to inline and let two honest copies exist until the *true* shared shape is known.

### E3. AHA / Avoid Hasty Abstractions — wait for the third occurrence [SOURCED]
Kent C. Dodds: **"Optimize for change first"**; tolerate duplication until the pattern appears in enough places (≈ **three or more** — the Rule of Three) that "the commonalities will scream at you for abstraction." AHA is the middle path between dogmatic DRY (premature abstraction) and WET.
- **URL:** https://kentcdodds.com/blog/aha-programming
- **Why it matters:** gives the auditor a *timing* rule, not just a direction — "how many real call sites justify this extraction?"
- **Ghx-apply:** for each shared abstraction, count the *real* consumers today. One consumer → premature; two → watch; three+ with a stable shared shape → justified. A "reusable" package with a single caller fails AHA.

### E4. "A little copying is better than a little dependency" [SOURCED]
Go proverb (Rob Pike). Copying a few lines is often cheaper than importing a package — the dependency adds coupling, version surface, and an inward arrow that may violate the Dependency Rule.
- **URL:** https://go-proverbs.github.io/
- **Why it matters:** the Go-idiomatic counterweight to DRY; the reason a Go auditor tolerates *some* duplication that a Java auditor would extract.
- **Ghx-apply:** for a proposed shared helper between evals and product — would copying ~20 lines each side keep the two contexts decoupled and independently releasable (REP)? If the "dependency" it saves is small and the coupling it creates is large, the proverb says copy.

---

## Forward-looking re-pricing for an agentic-first codebase [SYNTHESIS]

The task warns against cargo-culting pre-AI-scale ideas. Where the canon shifts when the codebase is built and maintained largely by agents and read by agents (grounded in ghx's own "signal per token" north star and AGENTS.md Engineering Tenets):

- **YAGNI and the counter-canon get *heavier*, not lighter.** Agents generate plausible speculative abstraction cheaply and fast — the marginal cost of *writing* a premature framework has collapsed, but Fowler's **cost of carry** has not. Every speculative layer is context an agent (or the sidecar itself) must re-load to understand later. In a signal-per-token regime, gold-plated reuse is directly taxed. So E1–E4 should be weighted *up* in an agentic audit, not treated as old caution.
- **"A little copying" is *more* attractive when readers are context-bounded.** A local copy an agent can read in one file beats a deep dependency chain it must traverse across packages to understand. The Dependency Rule and the copying proverb both serve "minimize what must enter the reader's context" — which is literally ghx's thesis about the main agent's context.
- **Encapsulation earns its keep as an agent-legibility boundary.** DDD's aggregate-root / single-owner rule isn't just about invariants; it means an agent modifying "session" logic has *one* place to look. Ubiquitous language matters more when the reader pattern-matches on names: agent-written code drifts from doc vocabulary silently, so the ADR-vs-symbol-name check (B4) is a first-class agentic-audit item.
- **Compiler-enforced boundaries beat convention *more* decisively.** Brown's "lean on the compiler" is amplified: convention-only boundaries rely on a human reviewer's memory, and agent-authored PRs erode conventions faster than humans do. `internal/`, unexported types, and an acyclic import graph are the boundaries that survive an agent fleet.
- **What does *not* change:** the Dependency Rule, small interfaces, and Shared-Kernel discipline are as valid at AI scale as before — they are about information flow and coupling, which agents obey the same physics as humans. The re-pricing is on *speculative reuse* (down-weight building it) and *legibility* (up-weight it), not on the boundary rules themselves.

---

## Distilled auditor checklist (for the skill author to lift)

A compact rubric derived from the clusters above — each line traces to a [SOURCED] principle:

1. **Import graph, inward only.** Core (`internal/ghx`, shared telemetry) imports no frontend/transport/eval package. (A1, C4-ADP)
2. **One core, many adapters.** CLI/MCP/codemode/sidecar/eval-runner are adapters over one port; none re-implements core recon. (A2, C8)
3. **Named domain types, not primitive soup.** Repo-ref, tier, score, session-id, report are types with methods + validation. (B1)
4. **One authoritative owner per concern.** Single session store / report validator / telemetry writer; everyone routes through it. (B3)
5. **Vocabulary parity.** Symbol names match the ADR/NORTH_STAR ubiquitous language. (B4)
6. **Shared kernel is small + coordinated.** The evals↔product core is a *minimal* explicit subset, not a growing catch-all; changes are deliberate. (C2)
7. **Reuse priced against the tension.** Shared components group things that change together (CCP) and that all consumers need (CRP), and share a release cadence (REP). (C3)
8. **Boundaries enforced by the compiler.** `internal/`, unexported types — violations don't compile. (C5)
9. **Small consumer-side interfaces; no `util`/`common`/`any` boundaries.** (C6, C7, D3)
10. **Idiom over pattern ceremony.** Functional options and `func` values instead of Builder/Strategy/Factory scaffolding. (D1, D2)
11. **Counter-canon pass:** for every shared abstraction, is there a *second/third real consumer today*? Any `if eval {…}` branch in "shared" code? Any single-caller "reusable" package? If yes → recommend inline/duplicate. (E1–E4)

---

## Honestly-flagged gaps & degradations

- **Primary Evans DDD Reference PDF was inaccessible (HTTP 403).** Shared Kernel and the context-mapping patterns (C2) are sourced from **ddd-crew/context-mapping**, which reproduces Evans' pattern definitions faithfully and is maintained by recognized DDD practitioners — but it is one step removed from Evans' own text. An author quoting Shared Kernel verbatim should confirm against the Evans DDD Reference if a primary quote is needed.
- **Uncle Bob's primary site (butunclebob.com) was down (connection refused).** Component-cohesion/coupling principles (C3, C4) are sourced from a **community summary of Clean Architecture Part IV** (serodriguez68), verified for fidelity but secondary. The *principles* are Robert C. Martin's (Clean Architecture, 2017); the URL is a faithful transcription, not the author's page. Wikipedia's "Package principles" is an alternate verifiable secondary if needed.
- **Norvig's Design Patterns artifact (D2) could not be text-extracted** — the index page is a slide-deck launcher (no inline prose) and the PDF returned as unparseable binary via WebFetch. The thesis (patterns as language workarounds; many GoF patterns "invisible or simpler" with first-class functions/types) is accurately and widely reported, but the specific **"16 of 23"** figure is *unverified against the primary*. Treat as directionally sound, precise count unconfirmed.
- **`gobeyond.dev` (Ben Johnson's own site) returned 403;** C8 is sourced from his original **Medium** post of the same material (verified), not the newer gobeyond.dev version.
- **Scope boundaries (deliberate, not gaps):** this file is the *canon an auditor applies*, not a review of ghx's actual code. It does not read `internal/ghx`, `internal/sidecar/telemetry`, or the eval packages — the "Ghx-apply" lines are the *questions* the audit should ask, grounded in the doc-stated architecture (AGENTS.md, NORTH_STAR), not verified findings. Neighboring research files (recon of the actual tree; other canon angles) supply the code evidence.
- **Not covered here by design (owned by sibling research or out of angle):** microservices/distributed-systems tradeoffs (ghx is a monolith); event-sourcing/CQRS (no evidence ghx needs it); language-agnostic testing strategy; performance/concurrency patterns; security. Cross-family design-pattern catalogs beyond the Go-relevant subset were pruned per the "signal vs noise" mandate.

---

## Source Ledger

All URLs are specific deep links (essay/spec/pattern page), verified via WebFetch on 2026-07-07 unless the *Verified* column says otherwise.

| # | Source (author) | Deep URL | Authority | Verified | Grounds (cluster) |
|---|---|---|---|---|---|
| 1 | *The Clean Architecture* — Robert C. Martin | https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html | Primary (author's blog) | ✅ | A1 Dependency Rule |
| 2 | *Hexagonal Architecture* — Alistair Cockburn | https://alistair.cockburn.us/hexagonal-architecture/ | Primary (author, pattern originator) | ✅ | A2 Ports & Adapters |
| 3 | *Value Object* — Martin Fowler | https://martinfowler.com/bliki/ValueObject.html | Primary (recognized authority) | ✅ | B1, B2 |
| 4 | *DDD_Aggregate* — Martin Fowler | https://martinfowler.com/bliki/DDD_Aggregate.html | Primary | ✅ | B3 Aggregate/root |
| 5 | *Ubiquitous Language* — Martin Fowler | https://martinfowler.com/bliki/UbiquitousLanguage.html | Primary | ✅ | B4 |
| 6 | *Bounded Context* — Martin Fowler | https://martinfowler.com/bliki/BoundedContext.html | Primary | ✅ | C1 |
| 7 | Context Mapping (Shared Kernel, ACL, OHS…) — ddd-crew (Evans' patterns) | https://github.com/ddd-crew/context-mapping | Authoritative secondary (DDD practitioners; Evans' definitions) | ✅ | C2 Shared Kernel + companions |
| 8 | Clean Architecture Part IV — component principles (Robert C. Martin) | https://github.com/serodriguez68/clean-architecture/blob/master/part-4-component-principles.md | Secondary summary of primary book | ✅ (fidelity) | C3 REP/CCP/CRP, C4 ADP/SDP/SAP |
| 9 | *Modular Monolith* — Simon Brown | https://simonbrown.je/modular-monolith/ | Primary (author, C4 model) | ✅ | C5 |
| 10 | *SOLID Go Design* — Dave Cheney | https://dave.cheney.net/2016/08/20/solid-go-design | Primary (recognized Go authority) | ✅ | C6 SOLID-for-Go |
| 11 | *Package names* — Go blog (Sameer Ajmani) | https://go.dev/blog/package-names | Primary (official Go) | ✅ | C7 |
| 12 | *Standard Package Layout* — Ben Johnson | https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1 | Primary (author) | ✅ | C8 |
| 13 | *Self-referential functions & the design of options* — Rob Pike | https://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html | Primary (Go co-author) | ✅ | D1 functional options |
| 14 | *Design Patterns in Dynamic Languages* — Peter Norvig | https://norvig.com/design-patterns/ | Primary (author) | ⚠️ artifact not text-extractable (slide deck / binary PDF); thesis reported, "16/23" unverified | D2 |
| 15 | *Go Proverbs* — Rob Pike | https://go-proverbs.github.io/ | Primary (canonical transcription of Pike's talk) | ✅ | D3, E4 |
| 16 | *Yagni* — Martin Fowler | https://martinfowler.com/bliki/Yagni.html | Primary | ✅ | E1 |
| 17 | *The Wrong Abstraction* — Sandi Metz | https://sandimetz.com/blog/2016/1/20/the-wrong-abstraction | Primary (author) | ✅ | E2 |
| 18 | *AHA Programming* — Kent C. Dodds | https://kentcdodds.com/blog/aha-programming | Primary (author) | ✅ | E3 |

**Attempted-but-inaccessible (disclosed):** Eric Evans, *DDD Reference* PDF (domainlanguage.com) — HTTP 403; butunclebob.com `PrinciplesOfOod` — connection refused; gobeyond.dev `standard-package-layout` — HTTP 403. Substitutes used are noted in *Degradations* and the ledger.

**Source count:** 18 cited sources (16 fully verified, 1 degraded/partially-verified [Norvig], 1 verified-secondary standing in for a down primary [component principles]); 3 additional primaries attempted and disclosed as inaccessible.
