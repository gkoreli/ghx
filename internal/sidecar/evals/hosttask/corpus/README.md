# Host-task eval corpus (ADR-0032.1 S4)

Six hand-authored host tasks — the combined-objective host-task eval corpus
(ADR-0032.1 D3). Each task is one committed JSON fixture conforming to the S1
`hosttask.Fixture` schema and loaded through `hosttask.LoadCorpus`, which
applies the strict `RequireCorpusReady` gate (base schema **plus** a non-empty
`objective`). These are never upstreamed as issues.

## How a task is graded (thin container, not SWE-bench harness)

Each fixture pins a real workspace repo at a real commit **immediately before**
an upstream fix (the bug/feature is genuinely present at `pinnedSha`). The
`failToPass` test is the *hidden* regression/behavior test: a `setupCmds`
command writes it into the container as `ghx_f2p_test.go` at grade time, so the
agent working in the checked-out workspace never sees the grading criterion
(SWE-bench's hidden-test-patch invariant, expressed with the existing schema —
no patch field, no harness import). `passToPass` runs pre-existing in-tree
tests that already pass at the pin, guarding against regressions.

Every task was verified end-to-end at authoring time: the injected F2P test
**fails** at `pinnedSha` (bug present) and **passes** after the upstream fix is
applied, with `passToPass` preserved. The verification used
`golang:1.24-bookworm`-equivalent Go 1.26 locally; images pin Go 1.24 so the
`go1.23` build-tagged migration task compiles.

## The six tasks (D3 composition)

| id | family | repo | pinnedSha | what it tests |
|----|--------|------|-----------|---------------|
| `chi-fancywriter-readfrom-tee` | dependency-behavior bugfix | go-chi/chi | `a54874f` | `httpFancyWriter.ReadFrom` double-counts bytes when a Tee writer is set |
| `chi-routeheaders-double-next` | dependency-behavior bugfix | go-chi/chi | `6eb3588` | `RouteHeaders` calls the next handler twice with no routes (missing `return`) |
| `mux-multi-query-same-path` | bugfix | gorilla/mux | `2b030fc` | same-path routes differing by query constraint fail to match (stale `MatchErr`) |
| `mux-status-code-conformance` | conformance feature | gorilla/mux | `24c3e7f` | unsatisfied query on a shared path must yield 404 (`ErrNotFound`), not 405 |
| `chi-request-pattern-go123` | API-migration slice | go-chi/chi | `1c2d011` | adopt Go 1.23 `http.Request.Pattern`, build-tag-gated with a fallback |
| `echo-decompress-contentlength` | API-migration slice | labstack/echo | `6a390cb` | after gzip decompression, `Request.ContentLength` must be -1 per net/http |

D3 quotas satisfied: 3 dependency-behavior bugfixes (≥2), 2 API-migration
slices (≥2), 1 conformance feature (≥1), 3 distinct workspace repos (≥3, plus
echo makes 3 owners across 3 repos). `corpus_test.go` recomputes every quota
from the committed JSON.

## Memorization canary (ADR-0032 trap 3 / D3)

Every committed fixture carries `memorizationCanary.result: "pass"`: at
authoring time the subject model, given only the `objective` issue text and **no
repository access**, did not produce the fix from weights. Enforcement is not
advisory — `Fixture.Validate` (hence `LoadCorpus`) rejects any fixture whose
canary result is `fail`, so a from-weights-solvable task can never be committed.
`TestCorpusRejectsFailedCanary` proves that reject path end-to-end.

The tasks are deliberately mid-size, non-celebrity library internals (a
byte-counter off-by-one behind a Tee writer; a missing `return`; a stale match
error across same-path routes; a 404/405 conformance edge; a build-tagged
stdlib-field adoption; a ContentLength contract after streaming decompression)
— fixes that require reading *this* pinned code, not recalling a famous repo.

## Exploration sub-questions (GH2 ground truth)

Each fixture carries pre-registered `explorationSubQuestions` with ground truth
verified against the pinned revision (`verifiedAt`). These are the deterministic
half of GH2-recon (ADR-0032.1 D2): the sidecar arm's reports are scored against
these answers; the cross-arm judge comparison stays deferred until κ.
