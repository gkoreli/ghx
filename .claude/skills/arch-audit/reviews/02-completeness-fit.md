---
title: "arch-audit review 02 — Domain completeness & currency + fit-to-ghx"
date: "2026-07-07"
reviewer: "adversarial reviewer (skill-forge Round 2), Opus 4.8"
scope: "arch-audit SKILL.md + reference/personas.md + reference/artifact-template.md, against research/01-04 and ghx NORTH_STAR/AGENTS"
bar: "DH5/DH6 — quote the passage, name the flaw; tag MUST-FIX/SHOULD/OPTIONAL/REJECT-CANDIDATE + confidence"
mode: "read-only review; does NOT modify the skill"
worktree_head: "d2cda8a"
---

# arch-audit — Completeness/Currency & Fit review

Two sub-questions: (A) what audit lens/persona is **missing or outdated**, and
(B) would **running** this on ghx surface the right things or platitudes. The
review resists scope creep: adding a persona dilutes signal, so I argue for
**exactly one** missing lens and reject the rest, with evidence.

---

## Evidence contract

**Files read (in full):**
- `.claude/skills/arch-audit/SKILL.md`
- `.claude/skills/arch-audit/reference/personas.md`
- `.claude/skills/arch-audit/reference/artifact-template.md`
- `.claude/skills/arch-audit/research/01-idiomatic-go-architecture.md`
- `.claude/skills/arch-audit/research/02-clean-architecture-ddd-reusable-modules.md`
- `.claude/skills/arch-audit/research/03-tech-debt-detection-metrics.md`
- `.claude/skills/arch-audit/research/04-architecture-audit-methodology.md`
- `docs/NORTH_STAR.md`, `AGENTS.md`

**Commands run (worktree HEAD `d2cda8a`):**
- `grep -rn -E '\bgo (func|[a-z][\w.]*\()' internal/sidecar internal/ghx internal/cli` → **15 goroutine launch sites** (below).
- `grep -rln -E 'sync\.(Mutex|RWMutex|WaitGroup|Once)|chan |<-|go func' internal/sidecar` → **16 concurrency-bearing non-test files**.
- `grep -rc … internal/sidecar` hotspot ranking → `acp.go` (8), `daemon_worker.go` (7), `telemetry/jsonl_writer.go` (4), `preflight.go` (4), `daemon.go` (4).
- `grep -rn -- '-race' --include=*.yml --include=*.sh --include=Makefile .` → `-race` appears only in **ADR prose and two test comments**; the sole CI workflow is `.github/workflows/auto-tag.yml` (no standing race gate).
- `go build ./internal/sidecar/...` → clean.

**Deep URLs verified this session (WebFetch, text-only per Tool Economy):**
- Dave Cheney, *Never start a goroutine without knowing how it will stop* — https://dave.cheney.net/2016/12/22/never-start-a-goroutine-without-knowing-how-it-will-stop — verified verbatim: *"Every time you use the `go` keyword … you must know how, and when, that goroutine will exit."*
- Go blog, *Introducing the Go Race Detector* — https://go.dev/blog/race-detector — verified: `-race` usage across `test/run/build/install`; *"It is now part of our continuous build process, where it continues to catch race conditions as they arise."*
- Go blog, *Go Concurrency Patterns: Pipelines and cancellation* — https://go.dev/blog/pipelines — verified: goroutine leaks + `done`-channel cancellation (*"goroutines … must exit on their own"*).

**Load-bearing code evidence (worktree `d2cda8a`):**
- `internal/sidecar/daemon.go:148` `go s.handleConn(conn)` — the always-on runtime's accept loop spawns a goroutine per connection.
- `internal/sidecar/daemon_worker.go:222` `go w.watchLiveness(w.turnCtx, w.cancelTurnCtx, liveness)` — the ADR-0027 liveness watchdog is a goroutine.
- `internal/sidecar/daemon_worker.go:19,87` `mu sync.Mutex` — the warm worker/session pool holds mutex-guarded shared state accessed across entrypoints.
- `internal/sidecar/telemetry/jsonl_writer.go:25-54` — the **shared SAF/SAFE telemetry kernel** serialized by a **package-global `map[string]*sync.Mutex`**, with a self-authored comment that concurrent appends *"silently corrupts the trace/eval evidence — a truthfulness failure, not just a cosmetic one."*

---

## Honesty guard — what the skill gets RIGHT (verified, not conceded pro forma)

This is a strong skill; the criticism below is a seam gap, not a teardown.

1. **The reuse counter-canon is correct and load-bearing.** SKILL.md guardrail
   *"no `internal/domain` layer cathedrals, no DI containers ('in Go the
   composition root *is* the DI container'), no abstraction of a component with
   exactly one implementation forever"* is exactly right for a solo-dev Go
   product, and the mandatory YAGNI persona enforces it. This is not a platitude —
   it names the specific over-build ghx would otherwise accrete.
2. **The highest-value scope is the right one.** "Choosing a scope" centers the
   **evals↔product shared kernel** and the **Agent Sidecar Framework boundary** —
   which *is* ghx's actual architectural crux (NORTH_STAR M6/ADR-0022). Research 02's
   Shared-Kernel pricing (REP/CCP/CRP tension) is applied correctly, not as "reuse good."
3. **The three named framings are applied correctly.** *Utility tree* (run-spine
   step 1, ranked quality attributes per scope) = ATAM front-end, correctly grafted.
   *Hotspot* (complexity × git-churn, "the ~1–2% of files") = Tornhill/CodeScene,
   correctly stated as behavioral-not-static. *Fitness functions* (step 6) =
   Ford/Parsons/Kua, correctly framed as the anti-shelfware move. The conceptual
   spine is sound.
4. **The agentic-first judge guardrail is genuinely sophisticated.** Routing the
   metrics pass to Codex/GPT-5.5, "discount shared-prior convergence," and the
   Panickssery self-preference citation are ahead of most human review practice and
   correctly sourced. Not decorative.
5. **Anti-cargo-cult is explicit and honored.** The "don't cargo-cult a layout"
   canon (research 01 §10, Russ Cox / Kat Zien) is carried into the guardrails.

So the skill is **not** cargo-culted enterprise ceremony on the static-structure
axis. The gap is a whole axis it under-weights — the one ghx's roadmap lives on.

---

## Sub-question A — the ONE missing lens: **concurrency / goroutine-lifecycle architecture**

### A1 — MUST-FIX (confidence: high). There is no concurrency-architecture persona; "Resilience" is not a substitute, and ghx's frontier IS concurrency.

**Quote the passage.** The closest persona in `reference/personas.md:44-49` is:

> **Resilience & runtime robustness.** Failure modes, recovery, timeouts, partial
> results, supervision. … Hunts: no panic recovery, dropped contexts,
> error-string-based control flow, per-adapter re-implementation.

**Name the flaw.** This persona is framed *entirely around failure and recovery*.
Its hunt-list is faults, not lifetimes. It does **not** own the questions that
Go's concurrency canon exists to force:

- **Goroutine lifetime ownership** — *"Every time you use the `go` keyword … you
  must know how, and when, that goroutine will exit"* (Cheney, verified). A program
  can have flawless panic-recovery and still leak goroutines forever; "resilience"
  and "concurrency" are orthogonal.
- **Data races on shared mutable state** — a race is a *correctness* defect the
  resilience lens never looks for. The Go race detector exists precisely because
  this is invisible to reading and to unit tests (Go blog, verified).
- **Channel/select shutdown and cancellation propagation** as *structure*, not as a
  timeout knob (Go pipelines: `done`-channel broadcast; *"goroutines … must exit on
  their own"*, verified).

**Why this is the lens that most matters for ghx (not a laundry-list add).** ghx's
*entire active frontier* is a concurrency-architecture problem, and the code already
proves it:

- The whole B-workstream frontier is runtime concurrency: **B6 always-on warm
  daemon shared across entrypoints, B7 session routing, ADR-0027 liveness
  watchdog/wrap-up recovery** (NORTH_STAR lines 306-318). These are not "features"
  — they are a shared-mutable-state, multi-goroutine, multi-*process* runtime.
- The code confirms the surface is live and concentrated exactly there:
  `daemon.go:148 go s.handleConn(conn)` (goroutine-per-connection accept loop),
  `daemon_worker.go:222 go w.watchLiveness(...)` (watchdog goroutine),
  `daemon_worker.go:19,87 mu sync.Mutex` (the warm worker/session pool's shared
  state). 15 `go` launch sites, 16 concurrency-bearing files.
- **The smoking gun is where concurrency and the shared kernel intersect.**
  `telemetry/jsonl_writer.go:25-54` serializes the SAF/SAFE trace substrate with a
  **package-global `map[string]*sync.Mutex`**, and its own comment says a race here
  is *"a truthfulness failure, not just a cosmetic one."* That single file is
  simultaneously (a) the reusable-core persona's shared kernel, (b) the
  Go-architecture persona's ambient-global smell, and (c) a concurrency-correctness
  boundary — **yet no current persona owns question (c).** The reusable-core lens
  checks the *boundary*; the Go lens flags the *global*; neither asks *"is the
  concurrency model correct and complete?"* Concretely: a per-path `sync.Mutex`
  **serializes goroutines inside one process only** — but ghx's own design (B6 warm
  daemon **and** daemonless `ask`, per ADR-0030) means **two OS processes** can hold
  the same `~/.ghx` session JSONL path, where an in-process mutex map provides
  **zero** cross-process serialization. I do not assert this is a live bug (I did not
  trace every writer); I assert it is precisely the class of finding the current
  persona set is **structurally unable to surface**, and it lands on the Visibility &
  Truthfulness core tenet.

**This gap is self-documented in the skill's own research.** Research 01's Gaps
(line 277) explicitly defers: *"[SCOPE] … deliberately omits concurrency design,
error-handling architecture …"* And research 04 (line ~110) maps the Kruchten
**process view** as *"process = concurrency/resilience"* — then the persona catalog
ships only **"resilience,"** collapsing two distinct 4+1 concerns into one and
auditing only the failure half. Research 04's own §Divergence point 3 warns *"we
never audited the process/concurrency view"* — and the skill did not close it.

**Recommendation (scoped, one persona — net signal, not dilution).** Add a single
**Concurrency & goroutine-lifecycle** persona (opus), sibling to Resilience, whose
hunt-list is the orthogonal half: *every `go` has a known stop condition; shared
mutable state has a single owner or a guarded one; `context` cancellation
propagates to leaf goroutines; select-loops shut down cleanly; cross-process writes
to `~/.ghx` are serialized where the runtime allows concurrent processes.* Ground it
in the three verified deep URLs above. Make it **mandatory whenever the scope
touches `internal/sidecar/daemon*`, session store, or telemetry** (i.e. the runtime),
optional otherwise — so it never dilutes a pure-`internal/ghx` static-layout audit.
This is one persona, gated to the runtime scopes, not a catalog expansion.

### A2 — SHOULD (confidence: high). The fitness-function catalog omits the single most important one for ghx: `go test -race` in CI.

**Quote the passage.** Run-spine step 6 lists the invariants to graduate:

> The structural invariants the audit relied on — **import-direction greps, a
> god-file size ceiling, "no `acp`/transport import outside the adapter", the
> persona-byte golden** — should become **committed, executable checks**.

**Name the flaw.** Every example is a **static** invariant. For a concurrent
resident daemon, the highest-value fitness function is **dynamic**: `go test -race`
running on every build — which the Go race-detector blog explicitly recommends
(*"part of our continuous build process"*, verified). The evidence that ghx needs
this and hasn't graduated it is decisive: `-race` appears **only in ADR prose**
(ADR-0027 line 177 *"`go test -race` clean, 5×"*, ADR-0030 line 500, ADR-0025 line
132) **and two test comments** — i.e. it is run *manually, ad hoc, per-ADR*, and the
only CI workflow is `auto-tag.yml`. This is **exactly** the "one-shot snapshot that
decays" failure mode the skill's own step 6 rails against (research 04 §Divergence 1),
applied to the very check ghx's team already knows matters. The skill's fitness-
function catalog should *name* `-race`-in-CI, because a catalog of only static
examples will not prompt a reviewer to graduate the dynamic one.

**Recommendation.** Add `go test -race ./internal/sidecar/... ./internal/cli/...`
(a subset, to bound the 10× cost) to the step-6 example list of invariants to
graduate, tied to any runtime-scoped audit.

### A3 — the utility-tree vocabulary can't point at concurrency (confidence: medium)

Run-spine step 1's utility-tree examples rank *replaceability / testability /
cohesion / maintainability / correctness*; research 04's worked example ranks
*"replaceability 9, testability 8, resilience 7."* **No example ever ranks a
liveness / race-freedom / concurrency-safety quality attribute.** In ATAM those are
first-class attributes (availability/performance). If the front-end vocabulary that
is supposed to *make personas target what matters* has no word for concurrency, the
front-end cannot aim at ghx's actual risk. Folded into A1's fix: when the utility
tree is drawn for a runtime scope, "concurrency-safety / liveness" must be an
available attribute to rank.

---

## Sub-question B — fit to ghx's actual context/goal

### B1 — HONESTY GUARD (fit is genuinely good on the static axis). Running this on the evals↔product scope would surface real findings, not platitudes.

The shared-kernel scope + Shared-Kernel pricing + "what's already right" mandatory
section would, on a static-structure audit, produce cited, actionable findings
(e.g. `telemetry/jsonl_writer.go`'s package-global lock map *is* the ambient-global
the Go persona hunts). The self-preference/cross-family judge discipline means the
convergence signal is trustworthy. On the axis it covers, this is not a platitude
generator. Credit stands.

### B2 — UNDER-FIT: MUST-FIX (confidence: high). The skill's center of gravity is the static-structure axis; ghx's velocity lives on the runtime axis. (Converges with A1 — same gap, fit framing.)

**Where it under-fits.** Of the seven personas, six are static-structure or
reuse lenses (Go boundaries, domain modeling, reusable-core, ports/adapters,
cohesion/testability, metrics). Exactly **one** (Resilience) touches runtime, it is
**optional**, and it audits only failure-recovery. But ghx's frontier milestones —
**B6 daemon, B7 session routing, M8 anticipation (which "presupposes exactly this
resident runtime")** — are all runtime-concurrency work (NORTH_STAR lines 161-173,
306-318). A solo dev + AI fleet shipping a warm daemon will have their velocity
gated by *goroutine leaks, races on the session store, and cross-process `~/.ghx`
contention* far more than by a mis-placed interface. The skill would run a
beautiful audit of the wrong axis. **This is the fit gap, and it is the same object
as A1** — which is why it ranks as the single most important gap: two independent
sub-questions converge on it.

### B3 — the agentic-first re-pricing is REAL on the reuse axis, DECORATIVE on the runtime axis (confidence: high, DH6)

**Quote the passage.** SKILL.md guardrail:

> **Forward-looking, agentic-first lens.** ghx's primary consumer is an AI agent,
> and this is 2026 … Re-price generic "best practice" against *our* actual context.

Research 02's re-pricing section (lines 193-201) then delivers substance:
*speculative reuse gets heavier* (agents generate premature abstraction cheaply,
cost-of-carry unchanged), *"a little copying" more attractive for context-bounded
readers*, *compiler-enforced boundaries beat convention more decisively because
agent-authored PRs erode conventions faster*. **All of that is real and correct.**

**Name the flaw (DH6, refuting the central claim of completeness).** Every clause of
the re-pricing is on the **static / reuse / legibility** axis. The *central*
agentic-first architectural risk for a **runtime** product is never named: an **AI
fleet editing a concurrent daemon introduces a data race or a goroutine leak that no
unit test and no reviewer catches** — the exact defect class the skill's own
research 02 argues agents cause faster (*"agent-authored PRs erode conventions
faster than humans do"*). The re-pricing applies that very insight to `internal/`
boundaries but **not** to the `go` keyword, where it bites hardest. The logical
consequence of the skill's own thesis is that `-race`-in-CI is the compiler-grade
boundary that "survives an agent fleet" (research 02 line 200) — yet the re-pricing
never reaches it (see A2). So the re-pricing is **substantively real where it
speaks, and conspicuously silent on the one runtime property an agent fleet most
endangers.** Closing A1+A2 is also what makes the agentic-first claim complete
rather than half-applied.

### B4 — OVER-FIT: OPTIONAL (confidence: medium). The 4–6-persona floor + full distillation+ADR apparatus is mild ceremony for narrow scopes — but correctly priced for THIS fleet.

**Quote.** Run-spine step 2: *"Pick **4–6** persona×lens pairs."* Plus the full
deliverable chain: charter → N artifacts → distillation → governing ADR → fitness
functions.

**Assessment.** For a *narrow module* audit (e.g. "is `mapengine` cohesive"), a
4-persona floor + distillation + governing ADR is more apparatus than the question
needs — a residual of ATAM's multi-day-workshop shape (which research 04 honestly
flags as *"pre-AI, stakeholder-workshop methods"*). **But** the CLAUDE.md fleet
economics (20× Claude plan, parallel background workers *"effectively free"*) genuinely
re-price this: the fan-out cost is near-zero for this setup, so the ceremony is
*affordable here specifically*, not universally. That is itself a correct agentic
re-pricing. **Do not cut the floor** — instead, the skill already hedges ("The loop
costs several agent generations — it pays off when architectural quality genuinely
moves velocity"). One light touch would help: state that below ~2 personas of real
disagreement the loop should degrade to "just read it" (the When-not-to-use door),
so a reviewer doesn't spin up the full apparatus for a one-file question. Low
priority; the skill mostly self-governs this already.

---

## Rejected candidate lenses (resisting scope creep — each argued against)

Adding a persona has a real cost (signal dilution, another artifact to converge). I
considered the prompt's other candidates and **reject all of them as the primary
add**, for ghx specifically:

- **Security / trust boundaries — REJECT-CANDIDATE (confidence: high).** ghx is a
  local, single-user reconnaissance CLI/daemon reading *public* GitHub; it is not a
  multi-tenant service. There is one real trust edge (the codemode `goja` sandbox +
  spawned `git clone`/ACP subprocesses), but the threat model is thin and the
  velocity payoff low. A security persona would mostly produce "harden the sandbox"
  platitudes. Note it as an *optional integration/prep persona* trigger if codemode
  ever executes third-party code for others; not a standing lens.
- **API/CLI-contract & versioning — REJECT (confidence: high).** This is an
  **anti-fit** for ghx: the north star's end state is *"the main agent needs zero
  ghx CLI knowledge"* (the CLI grammar disappears behind the sidecar), and AGENTS.md
  Engineering Tenets mandate *"No legacy maintenance, no compatibility junk … we are
  the only consumers."* CLI-contract versioning ceremony is exactly the pre-AI-scale
  dogma the skill correctly refuses. The *one* real contract (English-in →
  evidence-report-out; the `~/.ghx` artifact schema) is a **data-schema** concern —
  see next.
- **Data/artifact-schema evolution — REJECT as separate persona, FOLD into existing
  (confidence: medium).** This is the strongest runner-up: `~/.ghx` reports/traces
  are a triple-consumer contract (main-agent API surface, eval scoring that reads
  episode fields, M9 training exports). But it is **already partly owned**: the
  domain-modeling persona covers "types that flow across boundaries," and the
  "Respect the frozen measurement stack" guardrail covers eval-schema drift. The
  incremental signal of a dedicated persona is lower than concurrency's, and it does
  not touch the active B6/B7 frontier. Fold a one-line prompt into the domain-
  modeling persona ("do `~/.ghx` artifact schemas that cross SAF↔SAFE↔training carry
  a version / are they read-compatible?") rather than adding a persona.
- **Observability-as-architecture — REJECT (confidence: high).** Genuinely
  architectural for ghx (M6, the self-improvement tenet), but **substantially
  covered**: the reusable-core persona explicitly names `internal/sidecar/telemetry`
  as the shared kernel, and Open-Source-Leverage (OTLP-per-spec) is an AGENTS.md
  tenet a Go-boundaries auditor already checks. A separate persona would overlap
  ~70% with reusable-core. The *one* uncovered slice of observability is its
  **concurrency-safety** (the `jsonl_writer` race surface) — which A1 already claims.
- **Error-handling architecture — REJECT (confidence: medium).** Partly owned by the
  Resilience persona (*"error-string-based control flow … typed faults and marker
  strings reconciled"*). Not the frontier. Leave with Resilience.
- **Build/dependency-supply-chain — REJECT (confidence: high).** A single Go binary,
  solo dev, `go install`/npm/Homebrew via GoReleaser. Supply-chain audit is
  enterprise ceremony here; near-zero velocity payoff.

---

## The single most important gap

**Add one gated persona — Concurrency & goroutine-lifecycle — and name
`go test -race` in the fitness-function catalog.** ghx's active frontier (B6 warm
daemon, B7 session routing, ADR-0027 watchdog/recovery) is a shared-mutable-state,
multi-goroutine, multi-*process* runtime, proven by 15 `go` launch sites and the
`telemetry/jsonl_writer.go` global lock map that guards the SAF/SAFE trace substrate
whose corruption the code itself calls *"a truthfulness failure."* The current
persona set brushes this file from three angles and **owns it from none**; the
"Resilience" persona audits only the failure half of Kruchten's process view, a
collapse the skill's **own research (01 line 277, 04 line ~110 & §Divergence 3)
already flagged and shipped without closing.** Both sub-questions converge here:
it is the missing *lens* (A) **and** the axis where the skill under-fits ghx and
where its agentic-first re-pricing goes silent (B). Everything else on the candidate
list is either an anti-fit for ghx (CLI versioning, supply-chain) or already ~70%
owned (observability, error-handling, data-schema) — so this one add pays for
itself and the rest would dilute.
