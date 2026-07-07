# Tech-Debt Detection, Code-Quality Metrics & Maintainability (Go)

Research provenance for the `architecture-audit` skill — Round 0, angle 03.
Scope: the canon + concrete Go tooling an auditor uses to systematically **find**
and **prioritize** tech debt in a Go codebase, applied to ghx
(`internal/{ghx,sidecar,cli,codemode,mapengine}`, `cmd/`; ~55.7k Go LOC, Go 1.25).

This is *provenance*, not the skill. Every substantive claim carries a specific
deep URL + a one-line rationale; verification method is disclosed per source in
the Source Ledger. Synthesis (my own reasoning, not from a source) is labelled
**[SYNTHESIS]**.

---

## Executive Summary

1. **Debt is a decision metaphor, not a synonym for "bad code."** Cunningham
   (1992) coined it to argue for *deliberate, promptly-repaid* shortcuts; Fowler's
   quadrant (deliberate/inadvertent × prudent/reckless) makes the auditor's real
   question explicit: not "is this debt?" but "is the interest actually being paid,
   and was taking it on a defensible choice?" Debt in a rarely-touched file may be
   correctly left unpaid.

2. **Static complexity metrics are necessary but weak on their own.** Cyclomatic
   complexity (McCabe 1976) measures test-path count, not understandability, and
   above the method level it essentially tracks lines-of-code; cognitive complexity
   (SonarSource) was designed to fix the understandability gap; the Maintainability
   Index is largely discredited as a single number (van Deursen). An auditor uses
   them to *locate* candidates, never to rank the codebase by a scalar.

3. **The highest-leverage signal is behavioral, not static.** Tornhill / CodeScene
   show that combining **complexity × change-frequency (churn)** finds *hotspots* —
   the ~1-2% of files where refactoring actually buys velocity. Change-coupling
   (files that change together across boundaries) surfaces missing abstractions
   that no static tool sees.

4. **Prioritize by impact × tractability, then defend it with fitness functions.**
   Rank debt by leverage (hotspot score, gated by blast radius and test coverage),
   pay down where interest is real, and encode the intended architecture as
   executable *fitness functions* in CI so cleaned-up boundaries don't drift back.

5. **For ghx specifically:** the Go toolchain gives most of this for free —
   `gocyclo`/`gocognit` for per-function complexity, `dupl` for duplication,
   `deadcode` for orphaned functions, `staticcheck`/`go vet`/`golangci-lint` for
   correctness+smell classes, and `go mod graph` + `go list -deps` for the coupling
   graph. Churn comes from `git log`. The evals package (multiple ~900-1000-line
   files that keep changing) is the obvious first hotspot to test this method on.

---

## Load-Bearing Methods & Metrics

Each entry: what it is → source (deep URL + why) → **how to apply to ghx**.

### 1. The technical-debt metaphor (the framing)

- **Ward Cunningham, "The WyCash Portfolio Management System," OOPSLA '92
  experience report** — the origin. "Shipping first time code is like going into
  debt. A little debt speeds development so long as it is paid back promptly with a
  rewrite… Every minute spent on not-quite-right code counts as interest on that
  debt." Cunningham later stressed the debt is about *encoding an incomplete
  understanding of the domain*, not merely messy code.
  <https://c2.com/doc/oopsla92.html> — primary source for the metaphor and the
  interest/principal framing every later tool inherits.
- **How to apply to ghx [SYNTHESIS]:** classify each finding as principal (the
  cost to fix now) vs interest (the recurring drag while it stays). A 1000-line
  `discovery.go` that changes weekly is paying interest; a 700-line `inspect.go`
  that is stable may not be. The audit's job is to find where ghx pays interest,
  not to enumerate every not-quite-right line.

### 2. Fowler's Technical-Debt Quadrant (the triage rubric)

- **Martin Fowler, "Technical Debt Quadrant" (bliki)** — two axes,
  deliberate/inadvertent × prudent/reckless. Key auditor line (verified quote):
  "the prudent debt to reach a release may not be worth paying down if the interest
  payments are sufficiently small — such as if it were in a rarely touched part of
  the code-base." Prudent-inadvertent ("now we know how we should have done it") is
  *inevitable* even for excellent teams.
  <https://martinfowler.com/bliki/TechnicalDebtQuadrant.html> — the canonical triage
  frame; distinguishes debt worth paying from debt worth keeping.
- **How to apply to ghx [SYNTHESIS]:** tag each audited module. ghx already does
  prudent-deliberate decomposition (recent `acp.go` god-file split, "audit H2").
  The quadrant tells the fleet to *not* spend cycles on reckless-looking-but-frozen
  code and to fast-track reckless-inadvertent code in hot paths.

### 3. Cyclomatic Complexity — McCabe (locate untestable/branchy code)

- **Thomas J. McCabe, "A Complexity Measure," IEEE Trans. Software Eng. SE-2,
  308–320 (1976)** — V(G) = E − N + 2P over the control-flow graph; equivalently
  "1 + number of decision points." Predicts test-path count and, by NIST-235
  convention, a per-function ceiling around 10.
  Bibliographic anchor: <https://www.scirp.org/reference/referencespapers?referenceid=3056412>
  (DOI 10.1109/TSE.1976.233837). Corroborated on formula/history by the gocyclo
  README and the SonarSource paper's Introduction ("Formulated in a Fortran
  environment in 1976").
- **Limit:** McCabe measures *testability*, not understandability, and (see §5) at
  aggregate levels correlates strongly with size, so a high total often just means
  "big file."
- **How to apply to ghx:** `gocyclo -over 15 -top 25 ./internal/...` to list the
  worst functions; expect offenders in `sidecar/evals/{gates,gate_reducers,
  anticipation_predictor}.go`. Use it to nominate functions, not to score files.

### 4. Cognitive Complexity — SonarSource (locate hard-to-read code)

- **G. Ann Campbell / SonarSource, "Cognitive Complexity — a new way of measuring
  understandability," v1.7, 29 Aug 2023** (read directly from the PDF). Three basic
  rules: (1) *ignore* structures that shorthand multiple statements into one (method
  extraction, null-coalescing); (2) **+1 for each break in the linear flow** (loops,
  conditionals, `catch`, `switch`, sequences of `&&`/`||`, `goto`, recursion);
  (3) **+1 additional for each level of nesting** of those flow-breaks. Its headline
  illustration: `sumOfPrimes` and `getWords` both have cyclomatic complexity 4 but
  are "strikingly different in terms of understandability." Crucial admission for
  auditors: "it is widely acknowledged that the Cyclomatic Complexity scores of
  applications correlate to their lines of code totals… Cyclomatic Complexity is of
  little use above the method level."
  <https://www.sonarsource.com/docs/CognitiveComplexity.pdf> — the primary spec of
  the metric and the clearest statement of *why* cyclomatic ≠ maintainability.
- **How to apply to ghx:** `gocognit -over 15 -top 25 ./internal/...`. Because
  cognitive complexity penalizes *nesting*, it ranks the deeply-nested reducer/gate
  logic higher than raw branch counts do — a better "which function will an AI
  worker struggle to safely edit" signal than gocyclo alone.

### 5. The size-confound critique (why single numbers mislead)

- **Landman, Serebrenik & Vinju, "Empirical analysis of the relationship between CC
  and SLOC in a large corpus of Java methods and C functions," J. Softw. Evol. Proc.
  28(7):589–618 (2016)** — 17.6M Java methods + 6.3M C functions; contrary to prior
  belief they found only a *moderate* per-unit linear CC↔SLOC correlation (high
  variance), which becomes strong only after aggregation/power-transform.
  Preprint: <https://aserebre.win.tue.nl/Landman2015-ccsloc-jsep2015-preprint.pdf>
  — the empirical basis for "don't treat CC as an independent signal from size,
  especially aggregated."
- **Maintainability Index critique — Arie van Deursen, "Think Twice Before Using the
  'Maintainability Index'" (2014).** MI (Oman & Hagemeister 1992) =
  `MAX(0,(171 − 5.2·ln(HalsteadVolume) − 0.23·CC − 16.2·ln(LOC))·100/171)`.
  Verified criticisms: coefficients are un-derived and never recalibrated since the
  1994 HP experiments; every input is confounded by size so the blend is redundant
  ("just measuring lines of code… is a much simpler metric"); *averaging* masks
  high-risk outliers under power-law distributions; validated only on small C/Pascal
  programs. Practitioner recommendation: "you'll be better off looking at lines of
  code."
  <https://avandeursen.com/2014/08/29/think-twice-before-using-the-maintainability-index/>
  — the reason not to rank ghx by a composite maintainability scalar.
- **How to apply to ghx [SYNTHESIS]:** do **not** produce a single "maintainability
  score" per file. Report LOC, cognitive complexity, and churn as *separate* axes
  and let the reader see the distribution (outliers, not averages). ghx's top files
  by LOC (`discovery.go` 1007, `anticipation_predictor.go` 974, `gates.go` 920) are
  candidates *because they are big and change*, not because a formula says so.

### 6. Coupling / cohesion metrics — Robert C. Martin package metrics (the graph)

- **Robert C. Martin, "OO Design Quality Metrics: An Analysis of Dependencies,"
  28 Oct 1994** (read directly from the PDF). Per "category" (≈ package):
  **Ca** afferent coupling = classes *outside* that depend *in*; **Ce** efferent =
  classes *inside* that depend *out*; **Instability I = Ce/(Ca+Ce) ∈ [0,1]** (0 =
  maximally stable/depended-upon, 1 = maximally unstable); **Abstractness A =
  #abstract / #total ∈ [0,1]**; the **Main Sequence** is the A vs I line, and
  distance from it, **D = |A + I − 1|**, flags packages that are either "painfully
  rigid" (stable+concrete) or "useless" (unstable+abstract).
  <https://linux.ime.usp.br/~joaomm/mac499/arquivos/referencias/oodmetrics.pdf> —
  primary source; the only classic metric set that ports cleanly to Go's
  package-not-class model.
- **Complement — CK metrics (Chidamber & Kemerer 1994, CBO/LCOM etc.)** apply at the
  class level and map poorly to Go (no inheritance-heavy OO); noted for completeness
  but Martin's *package* metrics are the right lens for ghx.
- **How to apply to ghx:** treat `internal/{ghx,sidecar,cli,codemode,mapengine}` as
  Martin's categories. Derive Ca/Ce from the import graph (`go list -deps ./...`,
  `go mod graph`): `cli` should be highly *unstable* (I→1, depends on everything,
  depended on by nothing) and `mapengine`/core should be *stable* (I→0). A core
  package that is both heavily depended-upon **and** volatile is the dangerous
  "rigid" case — pay it down first. This is the direct instrument for the audit's
  "core/reusable-capability boundary" question (the Agent Sidecar Framework).

### 7. Code smells — god-files, long methods, duplication (surface indicators)

- **Martin Fowler, "CodeSmell" (bliki)** — term coined by Kent Beck for
  *Refactoring* ch.3 "Bad Smells in Code." Definition: "a surface indication that
  usually corresponds to a deeper problem in the system." Explicitly heuristics, not
  certainties: "some long methods are just fine." Examples: Long Method, Large
  Class / God Class, Duplicated Code, Data Class.
  <https://martinfowler.com/bliki/CodeSmell.html> — the canonical statement that
  smells *prompt investigation* rather than prove a defect.
- **How to apply to ghx:** ghx has concrete god-file smells — nine files ≥ ~600 LOC,
  four ≥ ~900. The repo already treats these as actionable (the `acp.go`
  decomposition, "audit H2"). Detect: LOC per file (`wc -l`), Long Method via
  gocognit/gocyclo, Duplicated Code via `dupl` (§Tooling). Then apply Fowler's
  discipline — a large but cohesive, stable file is *not* automatically a finding.

### 8. Behavioral code analysis — hotspots & change-coupling (the prioritizer)

- **Adam Tornhill, "Your Code as a Crime Scene," 2nd ed. (Pragmatic Bookshelf,
  2024)** — the foundational text for using *version-control history*, not just
  static structure, to prioritize debt; ~1-2% of a codebase drives up to ~70% of the
  work.
  <https://pragprog.com/titles/atcrime2/your-code-as-a-crime-scene-second-edition/>
  — the primary method source for behavioral analysis.
- **CodeScene docs — "Hotspots"** — hotspot = overlap of two signals: *complexity*
  (LOC as a proxy) × *change-frequency* (commit count as a proxy for effort). Key
  finding: "change alone is the single most important metric when it comes to quality
  issues in code." Prioritization further weights whether a hotspot co-changes with
  many modules and spans many developers (coordination bottleneck).
  <https://docs.enterprise.codescene.io/versions/2.8.0/guides/technical/hotspots.html>
  — operational definition of the complexity×churn method.
- **CodeScene docs — "Temporal Coupling / Sum of Coupling"** — files that change
  together are architecturally significant; "Sum of Coupling" ranks a file by how
  often it changes with *any* other file. Temporal couples that **cross
  architectural boundaries** signal a missing shared abstraction (often duplicated
  knowledge) and are prime refactoring candidates.
  <https://docs.enterprise.codescene.io/versions/2.4.2/guides/technical/temporal-coupling.html>
  — the change-coupling instrument that static analysis cannot replicate.
- **How to apply to ghx:** compute churn with
  `git log --format=format: --name-only --since='6 months' | sort | uniq -c | sort -rn`,
  join against LOC/cognitive-complexity to build a hotspot ranking. **[SYNTHESIS]**
  Expectation: `sidecar/evals/{discovery,gates,anticipation_predictor,rewards}.go`
  dominate (large *and* actively evolving) — the first place a fleet refactor buys
  velocity. For change-coupling, mine co-commits to find cross-package couples
  (e.g. `sidecar/emit.go` ↔ `reportsink.go` ↔ `telemetry/trace.go`) that hint at a
  missing seam.

### 9. Architecture fitness functions — drift detection (keep debt paid down)

- **Ford, Parsons, Kua & Sadalage, "Building Evolutionary Architectures"** —
  definition (verified): "An architectural fitness function is any mechanism that
  provides an objective integrity assessment of some architectural
  characteristic(s)." Categories include atomic vs holistic and triggered vs
  continual. In CI they stop *architectural drift/erosion*: e.g. ArchUnit rules that
  forbid backend→frontend imports, ban cyclic dependencies in domain slices, or keep
  API packages free of implementation deps.
  Accessible source: <https://www.infoq.com/articles/fitness-functions-architecture/>
  (the O'Reilly ch.2 is paywalled — see Gaps). — the concept that turns a one-time
  audit into an enforced invariant.
- **How to apply to ghx:** encode the intended boundaries as Go tests. `go`-native
  options: a test using `go list -deps` assertions, or `depguard`/`import-boundaries`
  linters in `golangci-lint`, e.g. "`internal/mapengine` must not import
  `internal/sidecar/evals`", "`internal/cli` is import-terminal." **[SYNTHESIS]**
  ghx just decomposed `acp.go`; a fitness function that caps per-file LOC or forbids
  the old cross-imports prevents the god-file from silently regrowing — cheap
  insurance the audit should recommend alongside each structural fix.

### 10. The economics — pay vs defer (the decision)

- **Martin Fowler, "Is High Quality Software Worth the Cost?" (2019)** — internal
  quality is an *economic*, not moral, argument: high internal quality lowers the
  cost of future features, so past the "design payoff line" cruft costs money;
  interest = the extra cost per feature, principal = the cleanup.
  <https://martinfowler.com/articles/is-quality-worth-cost.html> — the framing for
  *when paying down debt is net-positive* vs premium wasted on frozen code.
- **How to apply to ghx [SYNTHESIS]:** combine with Fowler's quadrant (§2) and
  hotspots (§8): pay principal where interest is demonstrably accruing (hot,
  high-blast-radius files); defer in cold corners. For an AI fleet, "cost of future
  features" ≈ how often a worker must safely edit a file — which is exactly churn ×
  understandability, i.e. the hotspot score.

---

## Concrete Go Tooling

| Tool | What it measures | Key limits | Command (ghx) | Source |
|---|---|---|---|---|
| **gocyclo** | Cyclomatic complexity per func: base 1, +1 per `if`/`for`/`case`/`&&`/`\|\|` | Testability not understandability; ignores nesting; `//gocyclo:ignore` escapes | `gocyclo -over 15 -top 25 ./internal/...` | <https://github.com/fzipp/gocyclo> |
| **gocognit** | Cognitive complexity per func (nesting-weighted, ignores shorthands) | Newer, fewer baselines; still per-function only | `gocognit -over 15 ./internal/...` | <https://github.com/uudashr/gocognit> |
| **dupl** | Duplicated code via suffix-tree over serialized ASTs (type-only, ignores literals) | Structural only (renamed-but-same matches; explicit false positives); token threshold sensitive | `dupl -t 100 ./internal/sidecar/evals` | <https://github.com/mibk/dupl> |
| **deadcode** | Unreachable funcs via Rapid Type Analysis (whole-program from `main`) | Unsound across assembly/`go:linkname`/reflection; needs a `main`, not a library; may miss some dead funcs | `deadcode ./cmd/ghx` | <https://go.dev/blog/deadcode> |
| **go vet** | 40+ built-in analyzers: `printf`, `copylocks`, `lostcancel`, `loopclosure`, `unreachable`, `structtag`… | Heuristic; bugs/suspicious constructs only, not style or complexity | `go vet ./...` | <https://pkg.go.dev/cmd/vet> |
| **staticcheck** | 150+ checks: SA (stdlib misuse/bugs), simplifications, style; superset-adjacent to vet | Some false positives; opinionated; not a metrics tool | `staticcheck ./...` | <https://staticcheck.dev/docs/checks> |
| **golangci-lint** | Meta-runner: bundles govet, staticcheck, gocyclo, gocognit, dupl, depguard, etc.; one config, fast | Version/config drift across linters; can flood if unconfigured | `golangci-lint run ./...` | <https://golangci-lint.run/docs/linters/> |
| **go mod graph** / `go mod why` | Module requirement graph (`path@version → requirement` per line); shortest import path to a module | Module-level (not package); for intra-repo coupling use `go list -deps` | `go mod graph`; `go list -deps ./internal/cli` | <https://pkg.go.dev/cmd/go> |
| **git log (churn)** | Change-frequency & co-change (the behavioral half) | Not a "tool" per se; needs joining to complexity; history rewrites/renames distort | `git log --format=format: --name-only \| sort \| uniq -c \| sort -rn` | (behavioral method, §8) |

Note: gocyclo/gocognit/dupl are also embedded as linters inside `golangci-lint`, so
one config can produce most of the static half of the audit in a single run.

---

## How to Prioritize Debt by Leverage (not enumerate smells)

**[SYNTHESIS]**, grounded in §2, §8, §10. A smell list is worthless without a rank;
an auditor produces an *ordered* backlog by impact × tractability.

1. **Impact ≈ leverage score = complexity × churn (the hotspot).** Static complexity
   alone over-weights big-but-stable code; churn alone over-weights small-but-busy
   code. Their product is where paid interest is highest (CodeScene: change is the
   dominant quality signal). Rank ghx files by
   `cognitive_complexity × commits_last_6mo`.

2. **Gate by tractability, don't just sort by impact.** A high-impact file is only a
   good *next* target if it is safely changeable. Tractability inputs:
   - **Blast radius / afferent coupling (Ca):** a high-Ca core file is risky to
     touch — schedule with tests first, not first.
   - **Test coverage:** low coverage on a hotspot = raise coverage *before*
     refactor (higher tractability after).
   - **Change-coupling breadth:** a file that co-changes across package boundaries
     signals a missing abstraction — high impact, and extracting the shared seam is
     often *more* tractable than editing in place (breaks the coupling permanently).

3. **Apply the quadrant filter (Fowler §2):** drop reckless-looking but frozen,
   low-Ca code — its interest is ~0, paying principal is wasted spend. Fast-track
   reckless-inadvertent code sitting in a hotspot.

4. **Order the backlog:** high leverage **and** high tractability → do now; high
   leverage, low tractability → invest in tests/decomposition to raise tractability,
   then do; low leverage → leave. This directly serves ghx's goal (a fleet paying
   down debt to raise velocity): the fleet works the top of a leverage-ranked,
   tractability-gated list, not an alphabetical smell dump.

5. **Lock in each fix with a fitness function (§9)** so the boundary you just paid
   for cannot silently drift back — otherwise the interest re-accrues.

---

## Honestly-Flagged Gaps

- **Two primary PDFs failed WebFetch's HTML conversion** (returned as binary):
  the SonarSource Cognitive Complexity paper and Robert Martin's OO Design Quality
  Metrics. Both were **recovered by reading the saved PDFs directly** (page images),
  so their content here is primary-source-verified, but not via the standard
  WebFetch path. Disclosed rather than hidden.
- **The Landman/Serebrenik/Vinju preprint URL was located via search, not
  independently fetched** — the finding (moderate, not strong, per-unit CC↔SLOC
  correlation) is corroborated by the SonarSource paper's own "correlate to lines of
  code totals" admission, which *was* read directly. Treat the exact corpus figures
  (17.6M / 6.3M) as search-reported.
- **CodeScene hotspot/temporal-coupling and Tornhill's book were located via search**
  with substantive quotes returned, not fully fetched page-by-page. The method is
  well-established; specific numeric claims (e.g. "1-2% of code → 70% of work") are
  the vendor's framing, not independently measured here.
- **The O'Reilly "Building Evolutionary Architectures" ch.2 is paywalled (HTTP 403).**
  I substituted the publicly-accessible InfoQ article, which carries the same
  verbatim book definition; the fine-grained category taxonomy
  (atomic/holistic/triggered/continual/static/dynamic) is only partially confirmed
  from the accessible source.
- **McCabe 1976 and CK 1994 were confirmed bibliographically, not from the paywalled
  IEEE full text.** Formula and history are corroborated by tool docs (gocyclo) and
  the SonarSource paper. CK metrics are noted but deliberately down-weighted — they
  target class-level OO and map poorly to Go's package model.
- **No Go-specific empirical study of debt↔velocity was found** in this pass. The
  hotspot method's evidence base is largely Java/C/multi-language (CodeScene,
  Landman). Its applicability to ghx's Go codebase is a reasonable **[SYNTHESIS]**
  extrapolation, not a Go-validated result — worth a targeted search if the skill
  needs Go-specific proof.
- **ghx file-size/name observations were read from the working tree at audit time**
  (Go 1.25, ~55.7k LOC); exact numbers will drift as the fleet lands changes. They
  illustrate *how to apply* the methods, not a frozen measurement.
- **`go list -deps`-based package coupling was described, not run** in this research
  pass — the Martin-metric application to ghx is a method recommendation to execute
  during the actual audit, not a computed result here.

---

## Source Ledger

Verification legend: **[WF]** WebFetch-verified this session · **[PDF]** read
directly from the saved PDF (page images) · **[S]** located via WebSearch with
substantive content returned, not independently fetched.

| # | Source | Deep URL | Verify | Load-bearing for |
|---|---|---|---|---|
| 1 | Cunningham, WyCash OOPSLA '92 (debt origin) | https://c2.com/doc/oopsla92.html | [S] | §1 metaphor, interest/principal |
| 2 | Fowler, Technical Debt Quadrant | https://martinfowler.com/bliki/TechnicalDebtQuadrant.html | [WF] | §2 triage rubric |
| 3 | McCabe 1976, "A Complexity Measure" (bib record) | https://www.scirp.org/reference/referencespapers?referenceid=3056412 | [S] | §3 cyclomatic complexity |
| 4 | SonarSource, Cognitive Complexity v1.7 | https://www.sonarsource.com/docs/CognitiveComplexity.pdf | [PDF] | §4 rules; §5 CC≠maintainability |
| 5 | Landman/Serebrenik/Vinju, CC↔SLOC (preprint) | https://aserebre.win.tue.nl/Landman2015-ccsloc-jsep2015-preprint.pdf | [S] | §5 size-confound |
| 6 | van Deursen, "Think Twice…Maintainability Index" | https://avandeursen.com/2014/08/29/think-twice-before-using-the-maintainability-index/ | [WF] | §5 MI critique |
| 7 | Robert C. Martin, OO Design Quality Metrics (1994) | https://linux.ime.usp.br/~joaomm/mac499/arquivos/referencias/oodmetrics.pdf | [PDF] | §6 Ca/Ce/I/A/D coupling |
| 8 | Fowler, "CodeSmell" | https://martinfowler.com/bliki/CodeSmell.html | [WF] | §7 smells as heuristics |
| 9 | Tornhill, "Your Code as a Crime Scene" 2nd ed. | https://pragprog.com/titles/atcrime2/your-code-as-a-crime-scene-second-edition/ | [S] | §8 behavioral analysis |
| 10 | CodeScene docs, Hotspots | https://docs.enterprise.codescene.io/versions/2.8.0/guides/technical/hotspots.html | [S] | §8 complexity×churn |
| 11 | CodeScene docs, Temporal Coupling | https://docs.enterprise.codescene.io/versions/2.4.2/guides/technical/temporal-coupling.html | [S] | §8 change-coupling |
| 12 | Ford et al., fitness functions (InfoQ) | https://www.infoq.com/articles/fitness-functions-architecture/ | [WF] | §9 drift detection |
| 13 | Fowler, "Is High Quality Software Worth the Cost?" | https://martinfowler.com/articles/is-quality-worth-cost.html | [S] | §10 debt economics |
| 14 | gocyclo | https://github.com/fzipp/gocyclo | [WF] | Tooling: cyclomatic |
| 15 | gocognit | https://github.com/uudashr/gocognit | [S] | Tooling: cognitive |
| 16 | dupl | https://github.com/mibk/dupl | [WF] | Tooling: duplication |
| 17 | deadcode (Go blog, Donovan) | https://go.dev/blog/deadcode | [WF] | Tooling: dead code / RTA |
| 18 | go vet | https://pkg.go.dev/cmd/vet | [WF] | Tooling: correctness analyzers |
| 19 | staticcheck | https://staticcheck.dev/docs/checks | [S] | Tooling: SA/style checks |
| 20 | golangci-lint | https://golangci-lint.run/docs/linters/ | [S] | Tooling: meta-runner |
| 21 | go command (go mod graph / why) | https://pkg.go.dev/cmd/go | [WF] | Tooling: dependency graph |

**Source count: 21** (10 WebFetch/PDF primary-verified, 11 search-located with
substantive content). Verified-primary breakdown: 8 [WF] + 2 [PDF].
