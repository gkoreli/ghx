# Idiomatic Go Architecture & Package/Module Design — Audit Canon

**Research artifact for the `architecture-audit` skill (Round 0 / R1).**
Angle: idiomatic Go architecture and package/module design. This is *research
provenance* — the sourced canon an audit persona applies when judging "is this
good idiomatic Go architecture?" It is **not** the skill itself and **not** an
audit of ghx.

- **Author:** Fable research agent (Opus 4.8), 2026-07-07
- **Scope target:** `ghx` Go codebase — `cmd/ghx`, `internal/{ghx,cli,codemode,mapengine,sidecar}`
- **Tooling:** text-only (WebSearch/WebFetch/gh/shell), per AGENTS.md Tool Economy
- **Sourcing discipline:** every substantive claim carries a specific deep URL,
  verified by WebFetch, with degradations disclosed. Synthesis is labeled
  `[SYNTHESIS]`. Full Source Ledger at the end.

---

## Sourcing method & honesty note (read first)

Every URL below was fetched with WebFetch and confirmed to be the claimed
document. **Caveat that colors every quotation:** WebFetch renders a page
through a small summarizer model, so "verbatim" quotes are high-confidence
reproductions *as WebFetch surfaced them*, not byte-for-byte guarantees. Where
a quote is load-bearing for an audit verdict, the auditor should re-pull the
primary URL. Specific degradations are called out inline as **[DEGRADED]** and
collected in Gaps.

---

## Executive summary

Idiomatic Go architecture is **anti-ceremony by construction**. The canon —
from Rob Pike's Go Proverbs and the official *Effective Go* / *Code Review
Comments*, through the community's load-bearing essays (Ben Johnson, Dave
Cheney, Peter Bourgon, Bill Kennedy, Jack Lindamood, Kat Zien) — converges on a
small number of forces that pull in the *opposite* direction from enterprise
OOP: organize packages by **dependency and capability, not by kind** (no
`models/`, `controllers/`, `util/`); keep the domain core dependency-free and
push external adapters to the leaves; **accept interfaces, return structs**,
with interfaces **small and defined by the consumer**, never pre-emptively by
the producer for mocking; prefer **composition (embedding + small interfaces +
functional options) over inheritance**; wire the whole object graph **explicitly
in `main`/`cmd` — the composition root** — instead of reaching for a DI
framework, service locator, or package-level singleton; keep imports **acyclic
and one-directional**, and let the `internal/` boundary do the API-surface
enforcement that "enterprise" projects try to get from layering ceremony.

The forward-looking (2026, agentic-first) reading sharpens two things.
**First, resist cargo-culting.** The single most-cited "layout" — `golang-standards/project-layout`
— is explicitly *not* a standard (Russ Cox says so, on the repo itself); Kat
Zien's 2023 retrospective walks back defaults she once taught (`cmd/` by
default, premature layering). An auditor's job is to check that ghx's structure
is *justified by ghx's actual dependencies and capabilities*, not copied from a
template. **Second, cohesion is the real metric, and it degrades silently.** Go
gives you almost no structural guardrails against a 1000-line god-file inside an
otherwise-clean package; cohesion has to be audited by reading, which is exactly
where a repeatable persona audit earns its keep.

For ghx specifically, the canon maps almost 1:1 onto the existing shape
(`cmd/ghx` as composition root; `internal/` boundary; a "core lives in
`internal/ghx`" rule; **no** `util/common/shared/models` packages present). So a
principled Go auditor's highest-yield targets are not the layout but the
**cohesion and dependency-direction** questions: oversized files
(`discovery.go` 1007 LOC, `anticipation_predictor.go` 974, `gates.go` 920,
`ghx.go` 823), the freshly-decomposed `acp.go` god-file (a positive signal to
verify held), the interface-placement discipline (consumer- vs producer-side),
and the **shared-core boundary between evals and the product** — the one place
Go's "a little copying is better than a little dependency" proverb and Ardan's
kit-vs-application distinction earn their money.

---

## The load-bearing principles an auditor checks

Each principle: the claim, its sources (deep URLs, verified), and a one-line
**Auditor→ghx** application.

### 1. Organize packages by dependency/capability, not by kind

Ben Johnson's "Standard Package Layout" is the canonical statement: the **root
package holds domain types with no dependencies** ("*The root package should not
depend on any other package in your application!*"); **subpackages isolate
external dependencies** as adapters (a `postgres` package implements the domain
interfaces); the **`main`/`cmd` package is the composition root** that wires them
together. He explicitly rejects Rails-style "by type" (`controller.UserController`)
and "by module" (`users.User`) layouts because they produce stutter names and
**circular dependencies**. Bill Kennedy's "Package-Oriented Design" reinforces
this: a package must have "a specific and foundational domain of functionality,"
packages should **provide, not contain**, and same-level packages should not
import each other.
- Ben Johnson, *Standard Package Layout* — https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1
- Bill Kennedy (Ardan Labs), *Package-Oriented Design* — https://www.ardanlabs.com/blog/2017/02/package-oriented-design.html
- **Auditor→ghx:** Confirm `internal/ghx` is the dependency-light capability core (the "core lives in `internal/ghx`" rule) and that adapters — ACP, `sidecar/tier2/repomap`, telemetry — sit as *leaf* subpackages depending inward, never the reverse. Any "by kind" grouping (`types/`, `handlers/`) is a smell.

### 2. Name packages for what they provide; ban `util`/`common`/`misc` god-packages

The official *Go Code Review Comments* says plainly: "*Avoid meaningless package
names like util, common, misc, api, types, and interfaces.*" Dave Cheney's
*Practical Go* gives the rule and the reason: "*Name your package for what it
provides, not what it contains,*" and "*Avoid package names like base, common,
or util*" — such packages exist only to break import cycles and signal a design
problem. A package name is also a prefix, so `chubby.File` beats
`chubby.ChubbyFile`.
- *Go Code Review Comments* (official Go wiki) — https://go.dev/wiki/CodeReviewComments
- Dave Cheney, *Practical Go* (QCon China 2019) — https://dave.cheney.net/practical-go/presentations/qcon-china.html
- **Auditor→ghx:** `find internal cmd -type d -iname 'util*' -o -iname 'common' -o -iname 'shared' -o -iname 'models'` — ghx currently has **none** (verified 2026-07-07), a clean pass. The audit keeps this a standing check because grab-bag packages accrete during refactors.

### 3. Accept interfaces, return structs — small, consumer-defined interfaces

Two canonical forces combine. Jack Lindamood coined the operational rule: accept
interfaces so a function is "*most specific in terms of its requirements … and
the most general in its function*," and return concrete structs so you don't
"*create this complexity until it's needed.*" The official *Code Review Comments*
locates the interface on the **consumer** side: "*Go interfaces generally belong
in the package that uses values of the interface type, not the package that
implements those values. The implementing package should return concrete …
types.*" And the Go Proverb sets the size bias: "*The bigger the interface, the
weaker the abstraction.*" Bourgon: "*Use many small interfaces to model
dependencies.*"
- Jack Lindamood, *What "accept interfaces, return structs" means in Go* — https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8
- *Go Code Review Comments* — Interfaces — https://go.dev/wiki/CodeReviewComments
- *Go Proverbs* (Rob Pike, GopherFest 2015) — https://go-proverbs.github.io/
- Peter Bourgon, *Go best practices, six years in* (QCon London 2016) — https://peter.bourgon.org/go-best-practices-2016/
- **Auditor→ghx:** Flag any interface declared next to its sole implementer purely to enable mocking (producer-side interface), and any constructor returning an interface where a struct would do. Consumers (`cli`, `sidecar`) should declare the *narrow* behavior they need from the `ghx` core, not import a fat exported interface.

### 4. Avoid premature abstraction (interface-YAGNI)

Lindamood's underlying philosophy — "*Always abstract things when you actually
need them, never when you just foresee that you need them*" — is echoed
normatively in *Code Review Comments*: "*Do not define interfaces before they're
used*" and "*Do not define interfaces on the implementor side … for mocking.*"
The Go Proverb "*A little copying is better than a little dependency*" is the same
instinct applied to imports: don't take a dependency (or an abstraction) to save
a few lines.
- Jack Lindamood — https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8
- *Go Code Review Comments* — https://go.dev/wiki/CodeReviewComments
- *Go Proverbs* — https://go-proverbs.github.io/
- **Auditor→ghx:** Hunt single-implementation interfaces with no second consumer and no test double that couldn't be a struct — each is speculative generality. This is a **[SYNTHESIS]** re-pricing for 2026: agentic codebases churn fast, so speculative interfaces age into dead abstraction faster than in slow-moving services — bias even harder toward deleting them.

### 5. Composition over inheritance — embedding, small interfaces, functional options

*Effective Go* is explicit that Go has no inheritance and substitutes
composition: "*Go does not provide the typical, type-driven notion of
subclassing, but it does have the ability to 'borrow' pieces of an
implementation by embedding types.*" Embedded methods "*come along for free*,"
and one-/two-method interfaces "*are common in Go code.*" ghx's own AGENTS.md
Engineering Tenets encode the non-dogmatic version: "*composition or inheritance
by use case, never dogma … interfaces + embedding + functional options chosen
for the actual shape of the problem.*"
- *Effective Go* — Embedding / Interfaces (official) — https://go.dev/doc/effective_go
- **Auditor→ghx:** Check that shared behavior is achieved by embedding + small interfaces, not by simulated inheritance (base structs with `Type` discriminators, deep option-struct hierarchies). Verify `functional options` are used where a constructor has many optional knobs (sidecar runtime, eval config) rather than growing a mega-config struct — **[SYNTHESIS]**: no dedicated functional-options source was pulled; see Gaps.

### 6. The composition root: wire dependencies in `main`/`cmd`; no DI framework, no global singletons

Three sources triangulate. Ben Johnson's fourth tenet: the `cmd/` main package
"*ties dependencies together*" — DI stays "*simple and trivial.*" Bourgon makes
it a law: "*Make dependencies explicit!*" — loggers, clients, and config are
constructor parameters, and "*only func main has the right to decide which flags
are available*"; package-level globals (`log.Print`'s global logger,
`http.Get`'s global client) hide dependencies and should be avoided. Mark
Seemann names the pattern: a **Composition Root** is "*a (preferably) unique
location … as close as possible to the application's entry point*" where the
object graph is composed, and **only** the composition root touches a DI
container; libraries have none. **[SYNTHESIS]** The idiomatic-Go corollary the
auditor enforces: `func main`/`cmd` *is* the DI container — a DI framework,
service locator, or `init()`-time singleton is usually a smell in Go, not a
best practice.
- Ben Johnson, *Standard Package Layout* (tenet 4) — https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1
- Peter Bourgon, *Go best practices, six years in* — https://peter.bourgon.org/go-best-practices-2016/
- Mark Seemann, *Composition Root* — https://blog.ploeh.dk/2011/07/28/CompositionRoot/
- **Auditor→ghx:** Verify `cmd/ghx` (and each `cli` command) constructs and injects the object graph; flag package-level mutable globals, `init()` side effects, and any hidden singleton in the sidecar runtime/session store. AGENTS.md's "one authoritative owner per domain concern … construct it once and inject/share it" is exactly this — the audit checks it is *injected*, not globally reached.

### 7. Acyclic, one-directional dependencies; `internal/` enforces the API surface

Go **forbids import cycles at the compiler level** (a package cannot, directly or
transitively, import itself) — so acyclicity is not advice, it is enforced; the
design question is *direction*. Ardan's package-oriented design sets it: a strict
hierarchy where foundational (kit / `internal/platform`) packages serve
higher-level ones, "*packages at the same level*" do not import each other, and
`internal/` cannot import `cmd/`. The `internal/` mechanism is **official**: "*a
package … in a directory named `internal` … can be imported only by code in the
directory tree rooted at the parent of the internal directory.*" That is how
idiomatic Go gets encapsulation and a stable public API **without** enterprise
layering ceremony.
- Bill Kennedy, *Package-Oriented Design* (dependency direction) — https://www.ardanlabs.com/blog/2017/02/package-oriented-design.html
- Official Go internal-packages rule (Go 1.4 release notes) — https://go.dev/doc/go1.4#internalpackages
- *Effective Go* (composition/import structure) — https://go.dev/doc/effective_go
- **Auditor→ghx:** Confirm dependency flow is one-way: `cli` → `sidecar` → `ghx` core (and `mapengine`/`codemode` as leaf capabilities), never inward-out. Sibling packages under `internal/sidecar` importing each other laterally is the Ardan smell to flag. The whole product living under `internal/` (nothing importable outside ghx) is correct for an application. **[DEGRADED]:** the compiler-enforced import-cycle prohibition is well-established Go behavior but was **not** deep-linked to the language spec here — see Gaps.

### 8. Package cohesion vs god-files/god-packages — the metric Go won't enforce for you

The Go Proverb "*Design the architecture, name the components, document the
details*" plus "*Clear is better than clever*" set the bar, but Go's tooling
does nothing to stop a cohesive-looking package from hiding a 1000-line
kitchen-sink file. Cohesion is a *read-it* property. This is where a
multi-persona audit adds signal a linter cannot.
- *Go Proverbs* — https://go-proverbs.github.io/
- Dave Cheney, *Practical Go* (package purpose) — https://dave.cheney.net/practical-go/presentations/qcon-china.html
- **Auditor→ghx:** Concrete hooks (measured 2026-07-07): `internal/sidecar/evals/discovery.go` (1007 LOC), `anticipation_predictor.go` (974), `gates.go` (920), `internal/cli/ghx.go` (823), `internal/cli/sidecar.go` (762), `internal/sidecar/daemon.go` (755). Ask of each: one concern or several? The recent decomposition of `acp.go` (commit log) is the *positive* pattern — verify it split along concern seams, not arbitrarily, and that the largest survivors get the same treatment.

### 9. Reusable/shared packages without ceremony — kit vs application; "a little copying > a little dependency"

Idiomatic Go builds reusable capability modules *without* the enterprise
apparatus (no framework, no plugin registry, no DI container). Ardan's kit-vs-application
split is the canon: **kit** packages are "*a company's standard library … maximum
portability*" that "*NOT allowed to set policy about any application concerns*" —
they **provide** capability and stay policy-free; **application** packages own
policy under `cmd/`, `internal/`. Seemann's rule that **libraries have no
composition root** is the same boundary from the DI side. And the Proverb "*A
little copying is better than a little dependency*" is the explicit license to
*duplicate small things* rather than couple two domains through a shared package.
- Bill Kennedy, *Package-Oriented Design* (kit vs application) — https://www.ardanlabs.com/blog/2017/02/package-oriented-design.html
- Mark Seemann, *Composition Root* (libraries have none) — https://blog.ploeh.dk/2011/07/28/CompositionRoot/
- *Go Proverbs* ("a little copying …") — https://go-proverbs.github.io/
- **Auditor→ghx:** The highest-value question for ghx's north star (SAF as reusable infra; the shared core between evals and the product). Check that shared modules — `internal/ghx` core, `internal/sidecar/telemetry`, eval capability packages — are **policy-free capability providers** consumed by both evals and the product, not two-way dependencies that drag eval policy into the product (or vice versa). Where a shared abstraction would couple unrelated policy, prefer small duplication (**[SYNTHESIS]**, applying the proverb to the eval/product seam).

### 10. Don't cargo-cult a layout — structure is emergent and justified, not templated

The most-linked "Go layout," `golang-standards/project-layout`, is **not** a
standard, and Russ Cox — a lead of the Go project — said so on the repo itself
(issue #117): the "golang-standards" namespace creates a false impression of
official status, most Go packages do **not** use a `pkg/` subdirectory, and the
layouts are more complex than real-world Go. Kat Zien's 2023 retrospective walks
back her own earlier defaults: "*As simple as possible, but no simpler*"; a
single `main.go` is often enough; "*A beginner trap is feeling the need to copy
what … bigger and more complicated services are doing off the bat*"; and even
`cmd/` — "*How often do you need multiple `main.go` files?*" — should not be a
reflex. **[SYNTHESIS] Forward-looking (2026):** learn from history, don't
freeze it — the auditor treats every layer of structure as something that must
earn its place against ghx's *actual* dependencies and capabilities; ceremony
copied from a template is tech debt, not architecture.
- Russ Cox, issue #117 "this is not a standard Go project layout" — https://github.com/golang-standards/project-layout/issues/117
- Kat Zien (JetBrains, 2023) *Catching up on the structure of Go apps* — https://blog.jetbrains.com/go/2023/04/11/catching-up-with-kat-zien-on-the-structure-of-go-apps-in-2023/
- Kat Zien, *How Do You Structure Your Go Apps* (GopherCon 2018 talk) — https://www.youtube.com/watch?v=oL6JBUk6tj0 **[DEGRADED: video]**
- **Auditor→ghx:** Note the *absence* of `pkg/` in ghx (Cox: most Go doesn't use it) is idiomatic, not a gap. For each structural choice (`internal/sidecar/tier2`, `evals` split, `cmd/ghx`), ask "what dependency or capability forces this?" — if the answer is "the template," flag it.

---

## Honestly-flagged gaps

1. **[DEGRADED] Kat Zien's primary artifact is a conference talk (video).** Per
   the Tool Economy (text-only), I did not watch it. The *content* is sourced via
   the text 2023 JetBrains retrospective (verified) plus search-confirmed talk
   metadata; the 2018 talk's specific slides (flat / layered / DDD / hexagonal
   examples) are known second-hand, not primary-verified. Her `app-structure-examples`
   repo README (with FAQ) was not fetched — a cheap follow-up if the skill wants
   her exact current recommendations.
2. **[DEGRADED] WebFetch summarizer mediation.** All "verbatim" quotes are
   WebFetch renderings, not byte-exact. Load-bearing quotes should be re-pulled
   from the primary URL before being cited in a shipped audit verdict.
3. **[GAP] Import-cycle prohibition not deep-linked to the language spec.**
   Principle 7 relies on Effective Go + Ardan + well-known compiler behavior
   rather than a spec anchor (the Go spec's package/import section, or
   `go/build` docs). Factually solid, but not maximally sourced.
4. **[GAP] Functional options pattern has no dedicated source here.** It appears
   in ghx's AGENTS.md tenet and Principle 5, but the canonical primary
   (Rob Pike, *Self-referential functions and the design of options*; Dave Cheney,
   *Functional options for friendly APIs*) was not fetched. If the audit weighs
   constructor ergonomics heavily, add one.
5. **[GAP] The anti-DI-framework stance is synthesis, not a single sourced
   claim.** It is triangulated from Ben Johnson + Bourgon + Seemann (composition
   root) and is mainstream idiomatic-Go opinion, but I did not source a direct
   "don't use `google/wire`/dig in Go" essay. Labeled **[SYNTHESIS]** in
   Principle 6. (`google/wire` is itself a *code-generated* composition root, not
   a runtime container — a nuance worth a dedicated source if the audit judges DI
   tooling.)
6. **[GAP] Bourgon's `pkg/` recommendation (2016) is dated and now contested.**
   Principle 6 cites his DI/globals guidance (still canonical) but I deliberately
   did **not** carry forward his "put library code under `pkg/`" line, because
   Cox (Principle 10) notes most Go doesn't use `pkg/`. Flagged so a future reader
   doesn't treat the older essay as uniformly current.
7. **[GAP] No 2024-2026 primary essay on "Go architecture in the agentic era."**
   The forward-looking, agentic-first re-pricing (faster interface-decay,
   cohesion-first auditing, template-as-debt) is **[SYNTHESIS]** — an application
   of the timeless canon, not a cited 2026 source. If such a source is wanted, it
   would need a fresh search; none was found to be canonical yet.
8. **[SCOPE] This artifact is one angle.** It deliberately omits concurrency
   design, error-handling architecture, generics-era design, testing
   architecture, and API/versioning — each is its own audit persona/research
   artifact. Cohesion between angles is the distillation step's job, not this
   file's.

---

## Source Ledger

Every URL used, with a one-line "why authoritative." All verified via WebFetch
on 2026-07-07 unless marked.

| # | Source | URL | Why authoritative |
|---|--------|-----|-------------------|
| 1 | Go Proverbs (Rob Pike, GopherFest SV 2015) | https://go-proverbs.github.io/ | The proverbs are Rob Pike's, Go's co-designer; the definitive short-form design values. (Site community-maintained; content is Pike's talk.) |
| 2 | Effective Go (official) | https://go.dev/doc/effective_go | Official Go documentation; the primary source on embedding/composition and interface idioms. |
| 3 | Go Code Review Comments (official Go wiki) | https://go.dev/wiki/CodeReviewComments | The Go team's own review-standard list; the normative source for interface-placement and package-naming rules. |
| 4 | Official internal-packages rule (Go 1.4 release notes) | https://go.dev/doc/go1.4#internalpackages | Official release notes defining the `internal/` import-restriction mechanism verbatim. |
| 5 | Ben Johnson — Standard Package Layout | https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1 | The single most-cited essay on Go package layout by dependency; author of BoltDB/Litestream. (gobeyond.dev mirror **403'd**; Medium original used and verified.) |
| 6 | Dave Cheney — Practical Go (QCon China 2019) | https://dave.cheney.net/practical-go/presentations/qcon-china.html | Long-form maintainability guide by a former Go core contributor; canonical on package naming, globals, interfaces. |
| 7 | Bill Kennedy / Ardan Labs — Package-Oriented Design | https://www.ardanlabs.com/blog/2017/02/package-oriented-design.html | The reference statement of Go package/kit-vs-application design and dependency direction; Ardan Labs is a primary Go-training authority. |
| 8 | Jack Lindamood — "accept interfaces, return structs" | https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8 | The essay that coined/popularized the proverb; the canonical explanation of the input/output abstraction bias. |
| 9 | Peter Bourgon — Go best practices, six years in (QCon London 2016) | https://peter.bourgon.org/go-best-practices-2016/ | Widely-cited production-Go practices by a well-known Go engineer (Go kit); canonical on explicit DI and avoiding globals. |
| 10 | Mark Seemann — Composition Root | https://blog.ploeh.dk/2011/07/28/CompositionRoot/ | The definitive definition of the Composition Root pattern (author of *Dependency Injection Principles, Practices, and Patterns*). Language-agnostic but the reference for "wire at the entry point." |
| 11 | Russ Cox — issue #117 "this is not a standard Go project layout" | https://github.com/golang-standards/project-layout/issues/117 | A Go project lead stating, on the repo itself, that the most-cited "standard layout" is not official — the primary anti-cargo-cult evidence. |
| 12 | Kat Zien (JetBrains, 2023) — Catching up on the structure of Go apps | https://blog.jetbrains.com/go/2023/04/11/catching-up-with-kat-zien-on-the-structure-of-go-apps-in-2023/ | Text retrospective by the author of the definitive "how to structure Go apps" talk; the forward-looking "don't over-structure / don't cargo-cult" source. |
| 13 | Kat Zien — How Do You Structure Your Go Apps (GopherCon 2018) | https://www.youtube.com/watch?v=oL6JBUk6tj0 | The original definitive survey talk (flat / layered / DDD / hexagonal). **[DEGRADED: video — not fetched per text-only Tool Economy; metadata/repo search-confirmed, content via #12 and liveblog summaries.]** |
