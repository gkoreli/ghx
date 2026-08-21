---
title: "A4 research — error messages as agent affordances"
date: "2026-08-21"
status: "research"
thread: "sidecar-adoption"
author: "background research worker"
scope: "read-only research artifact; proposes, does not implement; maintained under docs/research/"
builds-on: "NORTH_STAR workstream A4, docs/dogfood/FRICTION.md"
---

# A4 Research: Error Messages as Agent Affordances

affordances: every CLI error tells the agent the correct next invocation"
(`docs/NORTH_STAR.md:304`, status "Future (feeds from A2 findings)").

post-v2.9.0). **Binary under test:** source build of HEAD
(`go build -o /tmp/ghx-a4 ./cmd/ghx`; self-reports `ghx dev` — itself a live
reproduction of the provenance friction in §1.2 G6). Every behavioral claim below
was re-run live against that binary unless marked as code/doc citation only.
Commands + excerpts in §8.

---

## 1. Problem statement

### 1.1 The claim A4 makes — and where it already holds

ghx's users are agents (`docs/adr/0028-agent-facing-cli-research.md`; AGENTS.md
North Star: "the main agent needs zero ghx CLI knowledge"). An agent that hits an
error has two recovery channels: its own prior knowledge (expensive, often wrong
for a young CLI) or the error itself. A4's thesis: the error surface is a
*teaching* surface — every failure should name the class of what went wrong
(machine-parsable exit code) *and* the concrete next invocation (readable hint).

This is not speculative design — much of it already exists and is verified
working on mainline:

- Semantic exit codes 0/1/2/3 (`internal/cli/errors.go:11-20`): OK / no-results /
  bad-invocation / upstream-failure; `cmd/ghx/main.go:15-17` maps any returned
  error through `cli.CodeForError`.
- Fix-it lines prefixed `→` naming the next command (`internal/cli/errors.go:92-109`),
  with hints selected by the core classifier (`internal/ghx/failure.go:106-112`,
  `:138-165`) from structured signals (`*api.HTTPError.StatusCode`,
  `*api.GraphQLError.Type`), not substring guessing (ADR-0034 phases 1–2).
- Live confirmation (§8): malformed slugs exit 2 with an example invocation on
  explore/read/tree/inspect alike; a live 401 exits 3 with the exact
  `gh auth login` remediation; a 0-result search exits 1 and points at `ghx repos`.

### 1.2 Why it still needs an ADR-grade push: the residual gaps

Dogfood friction (`docs/dogfood/FRICTION.md`) plus this run's live battery show
the promise is unmet on specific surfaces:

| # | Gap | Evidence |
|---|-----|----------|
| G1 | **Sidecar frontend swallows failure classes into exit 0.** A BLOCKED-because-invalid-slug ask returns a well-reasoned report but `$?`=0; answered turns also exit 0, so `$?` carries no signal at all for `ask`. The `--depth bogus` validation (exit 2, `internal/cli/sidecar.go:76-83`) is the only class the sidecar frontend maps today. | FRICTION.md "sidecar `ask` frontend always exits 0" (open); code check today: `internal/cli/sidecar.go:110-135` prints the report and `return nil` — no outcome→exit mapping. |
| G2 | **Report narration fabricates exit codes.** The sidecar's Uncertainty section cited "`ghx explore badslugnoslash` → exit 3" when the real code is 2 — plausible-sounding but wrong evidence violates the visibility/truthfulness tenet (AGENTS.md). | FRICTION.md "sidecar agent narration fabricated downstream exit codes" (open). |
| G3 | **`inspect` degrades a repo-404 to exit 1 (no-results) while explore/read exit 3.** GitHub code search on a nonexistent repo returns empty rather than an error (`internal/ghx/inspect.go:255-259`), so the upstream 404 never reaches `ClassifyUpstream`. | Live: `inspect tidwall/this-repo-does-not-exist-xyz concern` → exit 1 vs `explore`/`read` on the same slug → exit 3 (§8 cases 10–12). |
| G4 | **MCP flattens every failure to one opaque string.** 19 `NewToolResultError(err.Error())` sites in `internal/cli/serve.go`; a calling agent cannot branch "refine query" vs "fix auth" vs "fix call". ADR-0034 phases 3–4 remain open. | `grep -c NewToolResultError internal/cli/serve.go` → 19 (ADR-0034 counted 16 pre-v2.8.0). |
| G5 | **Bare-cobra errors carry no affordance.** `ghx grep` with no args prints only cobra's "accepts 2 arg(s), received 0"; unknown-command suggestions are Levenshtein-only (`grop`→`grep`) with no example invocation. | Live §8 cases 8–9. |
| G6 | **Provenance/version signals contradict each other.** Source builds self-report `dev`; doctor's ghx-binary check can report a stale PATH binary while report-sink reports the running one — three version signals in one preflight, none authoritative ("which build am I running?" unanswerable from the tool). | FRICTION.md "source-built binary self-reports version dev" (open); reproduced live: HEAD build prints `ghx dev`. |

Two FRICTION.md items listed as open were found **already fixed on mainline**
post-v2.8.0 (commit `7dd74e0`, verified live): `code` transpile failure now exits
2 (`internal/cli/code.go:57-62`, pinned by `internal/cli/code_test.go`), and
`--depth bogus` is rejected with exit 2 naming the valid set
(`internal/cli/sidecar.go:76-83`). The empty-`GH_TOKEN=`→exit-1 masquerade was
investigated in dogfood and attributed to go-gh credential resolution, not
classification — left as-is by that entry's disposition.

---

## 2. Current-state inventory: commands × exit codes × messages

Exit-code contract (`internal/cli/errors.go:11-20`): **0** OK · **1** no-results ·
**2** bad-invocation · **3** upstream-failure. `CodeForError`
(`internal/cli/errors.go:71-87`) precedence: an explicit `ExitError` set by a
handler wins → core `ghx.Error` class via `errors.As` → default bad-invocation.

### 2.1 Verified live against the HEAD build (full excerpts in §8)

| Command shape | Condition | Exit | Message / affordance today |
|---|---|---|---|
| `explore / read / tree / inspect <badslug>` | malformed `owner/repo` | 2 | `invalid repo "noslash": expected owner/repo

| `explore / read / tree / inspect <badslug>` | malformed `owner/repo` | 2 | `invalid repo "noslash": expected owner/repo, e.g. ghx explore gkoreli/ghx` — names an example invocation |
| `read <repo> <path> --json` | unknown flag | 2 | `unknown flag --json; use --full, e.g. ghx read --full` (nearest-real-flag + example, ADR-0028.1 D3) |
| `search x y --repo z` | unknown flag | 2 | `unknown flag --repo; use --help, e.g. ghx search --help` |
| `code '<top-level await>'` | transpile failure | 2 | bare esbuild text; exit fixed post-v2.8.0 by `7dd74e0` (`internal/cli/code.go:57-62`) |
| `sidecar ask --depth bogus …` | out-of-set depth | 2 | `invalid --depth "bogus"; use cheap|normal|deep` (`internal/cli/sidecar.go:76-83`) |
| `grep` (no args) | cobra arity error | 2 | cobra default text only — no affordance (gap G5) |
| `grop` (unknown command) | unknown command | 2 | Levenshtein suggestion only — no example (gap G5) |
| `read nonexistent/repo README.md` | repo/ref 404 | 3 | upstream GraphQL text + `→ Repository, branch, or path not found. Confirm owner/repo with ghx repos <query> …` |
| `explore nonexistent/repo` | repo 404 | 3 | same class + same `→` hint as `read` (consistent) |
| `inspect nonexistent/repo <q>` | repo 404 | **1** | renders a normal no-candidates report + `ghx search …` footer — the 404 is invisible to `$?` (gap G3) |
| `search <repo> "<no-match>"` | empty result set | 1 | `→ To search repos by topic, use: ghx repos "<query>"` (`internal/cli/ghx.go:360-363`) |
| `search …` with bad token | HTTP 401 | 3 | `→ GitHub authentication failed. Run gh auth login (or set GH_TOKEN) …` — mirrors preflight wording |
| `sidecar ask --repo <badslug> …` | BLOCKED report | **0** | report prints; `$?`=0 — failure class swallowed by the frontend (gap G1) |

| `sidecar ask --depth bogus …` | out-of-set depth | 2 | `invalid --depth "bogus"; use cheap|normal|deep` (`internal/cli/sidecar.go:76-83`) — landed post-v2.8.0, verified live |

### 2.2 Where the machinery lives (code map)

- **Core classifier:** `internal/ghx/failure.go` — `FailureClass`
  (`ClassNone/ClassNoResults/ClassBadInput/ClassUpstream`, lines 15-46);
  typed `ghx.Error{Class, Err, Hint}` (48-78); hint table (106-112) whose auth
  wording mirrors sidecar preflight (`internal/sidecar/preflight.go:148`);
  `ClassifyUpstream` reads structured signals first — HTTP status, GraphQL error
  type — then a signature fallback (127-165); tests in `internal/ghx/failure_test.go`.
- **CLI mapping:** `internal/cli/errors.go` — `exitForClass` (52-66), the `→`
  marker convention (89-92), `withAffordance` (94-109), and `coreError` as the
  single entry point for core errors (111-135). The pre-ADR-0034 substring
  classifier (`upstreamRules`) is deleted.
- **Per-command wiring:** `internal/cli/ghx.go` — e.g. `repos` no-results (62-64),
  `read` zero-resolved-files → exit 1 (285-287), `search` zero-matches → exit 1 +
  topic-search pointer (360-363), `inspect` empty-query bad-input
  (`internal/ghx/inspect.go:250`).
- **Exit plumbing:** `cmd/ghx/main.go:15-17` — `os.Exit(cli.CodeForError(err))`.
- **Frontends without the class yet:** MCP `internal/cli/serve.go` (19 opaque
  `NewToolResultError` sites); sidecar report contract (`internal/sidecar/report.go`,
  `Evidence{Source, Summary}` carries no class/exit field).

- **Sidecar frontend:** `internal/cli/sidecar.go` — `ask` RunE validates `--depth`
  (76-83) but after a completed turn prints the report and returns nil
  (110-135): no outcome→exit mapping.

---

## 3. Design rationale: what affordance-style errors must guarantee

A4's end state decomposes into three guarantees. Each is grounded in an observed
failure, and each already has partial machinery to build on.

### 3.1 Next-invocation hints on every error

**Guarantee:** every error message names the concrete next command — an example
invocation for bad input (`invalid repo "x": expected owner/repo, e.g. ghx explore
gkoreli/ghx`), a remediation command for upstream failures (`gh auth login`),
a redirect for wrong-surface queries (`ghx repos "<query>"`), a flag name for
truncation (`use --lines A-B / --grep / --map`, ADR-0028.1 D5).

Evidence this matters: A2 mining found agents flail after failed guesses —
28 direct line-range flag failures plus 21 retry/flail runs
(`docs/evals/mining-2026-07-06/CLI-FINDINGS.md` defect 3); 9 `ghx grep`
failures each followed by retries into search/read/help (defect 5). The dogfood
confirmation entry ("A4 affordances name the fix on invocation errors",
FRICTION.md 2026-07-07) shows the pattern working: unknown-flag errors that name
the exact next command ended the flail immediately.

Design rule already established by ADR-0028.1 D3 and honored by the classifier:
hints are *imperative and runnable* — a copy-pasteable command, not advice.
The `→` marker gives agents a stable parse anchor at the tail of stderr
(`internal/cli/errors.go:89-92`: "an agent parsing the tail of stderr always
finds the recovery move in the same place").

### 3.2 Machine-parsable exit classes

**Guarantee:** `$?` alone tells a scripting agent which recovery branch to take:
0 proceed · 1 refine the query · 2 fix your own invocation · 3 fix auth/quota/
connectivity and optionally retry.

This is exactly the branching signal MCP callers lack today (G4) and the sidecar
frontend destroys today (G1). It is also the precondition for G2's fix: a report
can only cite truthful exit codes if the class is captured structurally at the
tool boundary instead of narrated from model memory — FRICTION.md's suggested
fix is precisely "capture and echo the real $? … rather than letting the model
narrate them."

### 3.3 Report evidence-fidelity

**Guarantee:** evidence cited in sidecar reports equals what the tools actually
returned — exit codes, commands, and outcomes captured from observation, not
reconstructed by the agent.

Grounding: the fabricated-exit-code friction (G2) is an evidence-fidelity bug of
exactly the kind AGENTS.md's visibility tenet targets ("we must not lie to
ourselves or our customers"). The same entry notes the report layer should cite
observed codes; A4 extends that from exit codes to all failure metadata: once
core classes flow to the sidecar report contract (ADR-0034 phase 4), a BLOCKED
report can say `class: bad_input` structurally, and narration has nothing left
to invent.

### 3.4 Why this passes the north-star filter

- **Removes tokens from the main agent:** a hint that names the next command
  replaces a retry-guess cycle (mining defects 3/5: dozens of wasted turns per
  run) and removes the need to re-derive recovery from `--help` re-reads.
- **More auditable evidence:** truthful exit codes in reports (G2/G3 fixes)
  make trajectories recomputable from artifacts, per AGENTS.md's evidence
  contract.
- **Lifts every profile honestly:** like ADR-0028.1's honesty note — CLI gains
  lift the `ghx` baseline and the sidecar alike; the eval isolates the sidecar's
  additional value (NORTH_STAR workstream A preamble). This is product work,
  not gate gaming.

---

## 4. Alternatives considered

| Alternative | Verdict | Rationale |
|---|---|---|
| Keep classification per-frontend (CLI substring tables copied into MCP/sidecar) | Rejected | Three copies of a domain concept invite divergence; explicitly rejected by ADR-0034 ("Considered and rejected"). The deleted `upstreamRules` table was the cautionary example. |
| Generic integer error codes shared by convention | Rejected | Typed `FailureClass` + `errors.As` is self-documenting and chain-safe; magic integers drift (ADR-0034 rejection #2). |
| JSON error objects on every CLI failure (`--json` everywhere first) | Deferred, not required | Highest fidelity, but changes human-facing output contracts repo-wide and is blocked on the still-missing shared JSON output contract (noted in ADR-0028.2 open questions). Exit codes + `→` lines deliver ~90% of the branching value at ~0 cost; JSON can layer on later without breaking the text contract. |
| Teach hints only in skills/docs (`skills/ghx/SKILL.md`) instead of in errors | Rejected as sole mechanism | Docs help only agents that read them before failing; errors reach exactly the agent that needs the fix at the moment of failure. Skills remain the complement (they already document exit codes), not the substitute. |
| Wrap cobra's arity/unknown-command errors with fully custom parser | Rejected for now | High surface churn for modest gain; cheaper: set `Args` validators with custom errors + curated `SuggestFor` (the ADR-0028.1 D2 pattern) and append examples via `SetFlagErrorFunc`-style hooks already proven in D3. |
| Do nothing — declare A4 done since phases 1–2 landed | Rejected | G1–G6 are live agent-facing gaps; NORTH_STAR lists A4 as its own sub-milestone, and the sidecar/MCP surfaces (where the main-agent consumer actually lives, NORTH_STAR capability 3) are precisely the ones still un-served. |

---
## 5. Risks

- **Hint rot.** Hints are strings that can drift from real flags/commands (the
  same fragility ADR-0034 removed for classification). Mitigation: golden-text
  tests per hint (the ADR-0028.1 "error-text goldens" verification pattern) and
  sourcing hint text next to the flag/command it names.
- **Output growth on hot failure paths.** Every error gains a line; agents pay
  for it on every failure. Mitigation: one `→` line max, empty-hint = no-op
  (already enforced by `withAffordance`, `internal/cli/errors.go:100-109`).
- **Exit-code contract changes break scripting consumers.** G1/G3 change codes
  agents may have baked in (0 → 2 for blocked-bad-slug; 1 → 3 for inspect-404).
  Mitigation: CHANGELOG `[Unreleased]` entries per AGENTS.md; verify no eval
  anomaly detector/scorer depends on the codes (ADR-0034 already verified this
  for its phase 2: "No eval anomaly detector or scorer depends on ghx CLI exit
  codes (verified against internal/sidecar/evals/)").
- **Sidecar exit-code mapping vs. answered-but-partial reports.** Mapping report
  outcome → exit must not punish honest BLOCKED-with-artifacts turns (ADR-0027
  made those first-class). Mitigation: map only the failure *class* — blocked
  because unusable input → 2, genuinely-no-evidence → 1 — exactly the FRICTION.md
  suggestion; answered → 0 regardless of caveats.
- **inspect 404 fix could mask real no-results.** If inspect starts treating
  empty search as upstream, a truly empty repo would misreport. Mitigation:
  disambiguate via a cheap repo-existence probe only when candidates == 0 and
  the repo was never confirmed (or classify from the GraphQL layer before
  search swallows it), keeping the empty-repo case exit 1.
- **Fabricated-narration fix could over-trust tool capture.** Echoing observed
  exit codes assumes the harness observes every invocation; unobserved calls
  must stay uncited rather than defaulted. Mitigation: cite only captured values,
  emit "not captured" otherwise (truthfulness over completeness).
---

## 6. Open questions

1. **Sidecar `ask` exit mapping granularity.** Should a WARN-outcome report
   (answered with caveats) stay 0, or introduce a distinct code? The 0/1/2/3
   contract has no slot for "answered but degraded"; adding one (e.g. 4) would
   extend the contract beyond ADR-0034's four classes — needs its own decision.
2. **Where inspect's 404 should be caught.** Classify in core before the search
   call (a repo-existence/ref check), or post-hoc when candidates==0? The first
   costs an API round-trip on every inspect; the second risks misclassifying
   genuinely empty repos (see §5).
3. **Does MCP error structuring wait for phases 3–4 sequencing?** ADR-0034
   explicitly sequenced MCP/sidecar behind "the host-task/eval frontier"; A4
   should either inherit that sequencing or argue the frontier has arrived.
4. **Version provenance (G6) scope.** Is ldflags stamping in the documented
   build recipe enough, or should doctor reconcile PATH-binary vs running-
   executable itself? FRICTION.md suggests both; ownership (CLI vs sidecar
   doctor) is undecided.
5. **Should bare-cobra errors (G5) get affordances via cobra hooks or per-command
   Args validators?** Hooks are centralized but blunt; validators are precise but
   per-command work across ~20 commands.
---

## 7. Proposed ADR outline

**Recommendation: thread as `ADR-0034.1`** — `parent: ADR-0034`,
`thread: multi-frontend-architecture`, status proposed.

### 7.1 Why 0034.x and not a new number

- **Same decision family, same taxonomy, same migration ladder.** ADR-0034 owns
  the FailureClass model and its phased frontend rollout; every remaining A4 gap
  (G1–G4) is literally one of its deferred phases or a direct consequence of a
  phase not having landed (MCP = phase 3, sidecar report = phase 4, sidecar CLI
  exit mapping = the phase-2 mapping applied to the sidecar *frontend* command).
  AGENTS.md is explicit: "Thread related decisions with decimal numbering when
  the work belongs to an existing decision family … Do not flatten related
  follow-up decisions into a single vague ADR" — but equally do not fork the
  family.
- **Precedent:** the repo already threads ergonomics work this way — ADR-0028 →
  0028.1 → 0028.2 (parent chain recorded in each frontmatter).
- **What would force a new number instead:** if ADR scoping included G5 (cobra
  surface) + G6 (version provenance) as first-class scope. Those are CLI-
  ergonomics-family items (ADR-0028's thread), not failure-class items. Cleanest
  split: **ADR-0034.1 carries G1–G4** (failure-class completion across
  frontends); G5/G6 either join a future ADR-0028.3 or ride along as explicitly
  labeled secondary scope in 0034.1.

### 7.2 Outline

1. **Status/Context** — cite this research doc; FRICTION.md open items (ask exit
   0, fabricated narration, inspect 404→1); ADR-0034 phases 3–4 open; mining
   defect evidence for why hints beat docs.
2. **Decision D1 — sidecar frontend maps outcome class → exit code** (G1):
   BLOCKED-because-bad-input → 2, no-evidence → 1, answered → 0; reuse
   `CodeForError` so the sidecar command and core commands share one mapping.
3. **Decision D2 — capture observed tool exit codes at the harness boundary**
   (G2): reports cite captured values only; uncaptured stays uncited.
4. **Decision D3 — close the inspect 404 divergence** (G3): classify upstream
   before rendering no-results; keep empty-repo at 1 (resolution of §6 Q2).
5. **Decision D4 — structure MCP error payloads with class+hint** (G4): execute
   ADR-0034 phase 3 verbatim; replace the 19 flatten sites.
6. **Decision D5 (secondary scope) — cobra-surface affordances + version
   provenance** (G5/G6) or explicit deferral to ADR-0028.3.
7. **Verification** — golden hint tests; exit-code table tests per frontend;
   byte-identical diffs for unchanged classes (the ADR-0034 phase-2 method);
   re-run the §8 battery as the acceptance script; dogfood re-check of the two
   confirmation entries to confirm no regression.
8. **Consequences/sequencing** — CHANGELOG entries; eval-stack independence
   check; alignment with NORTH_STAR A4 status flip (Future → frontier/done).
---
## 8. Evidence appendix - commands run + output excerpts

Environment: repo `/root/ghx` on `mainline` @ `719cb29` (clean tree).
Binary: source build of HEAD, `go build -o /tmp/ghx-a4 ./cmd/ghx` (self-reports `ghx dev`).
Live network cases used the environment's real GitHub credentials; the 401 case used an explicitly invalid token. All exit codes are `$?` as printed.

```text
$ git -C /root/ghx rev-parse --abbrev-ref HEAD && git log --oneline -1
mainline
719cb29 docs: normalize engineering record spacing

$ go build -o /tmp/ghx-a4 ./cmd/ghx && /tmp/ghx-a4 version
ghx dev

$ /tmp/ghx-a4 explore badslug; echo $?
invalid repo "badslug": expected owner/repo, e.g. ghx explore gkoreli/ghx
2

# same message+code for read/tree/inspect bad slugs:
$ /tmp/ghx-a4 tree noslash; echo $?
invalid repo "noslash": expected owner/repo, e.g. ghx explore gkoreli/ghx
2

$ /tmp/ghx-a4 inspect badslug concern; echo $?
invalid repo "badslug": expected owner/repo, e.g. ghx explore gkoreli/ghx
2

$ /tmp/ghx-a4 read gkoreli/ghx go.mod --json; echo $?
unknown flag --json; use --full, e.g. ghx read --full
2

$ /tmp/ghx-a4 search charmbracelet/bubbletea x --repo y; echo $?
unknown flag --repo; use --help, e.g. ghx search --help
2

$ /tmp/ghx-a4 code 'const r = await explore("x/y"); return r;'; echo $?
transpile: Top-level await is not available in the configured target environment ("es2015")
2   # FRICTION.md lists this item open at exit 0; fixed on mainline by 7dd74e0

$ /tmp/ghx-a4 grep; echo $?
accepts 2 arg(s), received 0
2   # cobra default text only, no affordance (gap G5)

$ /tmp/ghx-a4 grop; echo $?
unknown command "grop" for "ghx"

Did you mean this?
	grep

2   # Levenshtein suggestion only, no example (gap G5)

$ /tmp/ghx-a4 read tidwall/this-repo-does-not-exist-xyz README.md; echo $?
read failed: GraphQL: Could not resolve to a Repository with the name 'tidwall/this-repo-does-not-exist-xyz'. (repository)
→ Repository, branch, or path not found. Confirm owner/repo with `ghx repos <query>` and the layout with `ghx explore owner/repo` before reading a path.
3

$ /tmp/ghx-a4 explore tidwall/this-repo-does-not-exist-xyz; echo $?
GraphQL query failed: GraphQL: Could not resolve to a Repository with the name 'tidwall/this-repo-does-not-exist-xyz'. (repository)
→ Repository, branch, or path not found. Confirm owner/repo with `ghx repos <query>` and the layout with `ghx explore owner/repo` before reading a path.
3

$ /tmp/ghx-a4 inspect tidwall/this-repo-does-not-exist-xyz concern; echo $?
inspect: tidwall/this-repo-does-not-exist-xyz "concern"
budget: 283/12000 chars
matches: total=0 candidates=0 ranked=0 incomplete=true
ranked files:
  (no ranked files)
truncation:
  no candidates: ghx search tidwall/this-repo-does-not-exist-xyz "concern" --lang LANG or --glob "**/*"
no inspect results for "concern" in tidwall/this-repo-does-not-exist-xyz
1   # repo 404 degraded to no-results (gap G3)

$ /tmp/ghx-a4 search tidwall/gjson zzqqxxyyplughwz12345; echo $?
0 results (showing 0)
→ To search repos by topic, use: ghx repos "<query>"
no code results for "zzqqxxyyplughwz12345 repo:tidwall/gjson"
1

$ GH_TOKEN=ghp_invalidtoken0000000000000000000000 /tmp/ghx-a4 search tidwall/gjson func; echo $?
search failed: HTTP 401: Bad credentials (https://api.github.com/search/code?q=func+repo%3Atidwall%2Fgjson&per_page=30)
→ GitHub authentication failed. Run `gh auth login` (or set GH_TOKEN), then verify with `gh auth status` and retry.
3

$ /tmp/ghx-a4 sidecar ask --repo tidwall/gjson --depth bogus test; echo $?
invalid --depth "bogus"; use cheap|normal|deep
2

$ grep -c NewToolResultError internal/cli/serve.go   # MCP flatten sites (gap G4)
19
```

### 8.1 Code/doc citations not re-run live (cited from source)

- `internal/cli/errors.go:11-20,52-66,71-87,89-109,111-135` — exit taxonomy, class mapping, affordance wrapper.
- `internal/ghx/failure.go:15-46,48-78,106-112,127-165` — FailureClass, typed error, hint table, classifier.
- `internal/cli/ghx.go:62-64,285-287,360-363` — per-command no-results wiring.
- `internal/ghx/inspect.go:250,255-259` — empty-query bad-input; candidate search that swallows the 404.
- `internal/cli/sidecar.go:76-83,110-135` — depth validation; report-print-then-return-nil.
- `internal/cli/code.go:57-62` + `code_test.go` — transpile failure exits 2 (commit 7dd74e0).
- `cmd/ghx/main.go:15-17` — os.Exit(CodeForError(err)).
- `docs/NORTH_STAR.md:299-304` — workstream A table; A4 definition and status.
- `docs/dogfood/FRICTION.md` entries 2026-07-07: A4 confirmation; explore exit 3; read exit 0; 401 affordance; ask exit 0; fabricated narration; inspect/read 404 split; version dev; transpile exit 0; depth bogus.
- `docs/adr/0034-failure-class-model.md` (accepted; phases 1-2 as-built, 3-4 open); `docs/adr/0028.1-cli-ergonomics-batch.md` D2/D3/D5/D7; `docs/adr/0028.2-ghx-inspect.md`; `docs/evals/mining-2026-07-06/CLI-FINDINGS.md` defects 3/5.

---

*Research artifact for capability candidate A4. No existing files were modified; nothing committed.*

