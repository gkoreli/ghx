---
title: "ADR-0034: Unified Failure-Class Model Across Frontends"
date: "2026-07-07"
status: "accepted"
thread: "multi-frontend-architecture"
author: "Fable (orchestrator) — proposed autonomously; accepted by Goga"
---

# 0034. Unified Failure-Class Model Across Frontends

## Status

**Accepted** (Goga, 2026-07-07) — implement **phases 1–2 now** (core `FailureClass`
+ typed error + one upstream classifier with tests; then the CLI sources class from
core, fixing the three dogfooded exit-code frictions and deleting the substring
matcher). **Phases 3–4 (MCP, sidecar) are sequenced later** behind the host-task/eval
frontier, not built in this pass. Surfaced by the architecture audit
(`docs/audits/architecture-2026-07-07.md`, finding **M2**) and motivated concretely by
the v2.8.0 dogfood exit-code frictions (`docs/dogfood/FRICTION.md`: `explore badslug`→3
not 2, `read nonexistent`→0, live 401 with no affordance). It extends ADR-0007's
core/frontend rule ("core owns capabilities; frontends wrap the same core") and is
consistent with ADR-0010 and the AGENTS.md engineering tenets (domain models, service
encapsulation, no per-frontend divergence).

**Phases 1–2 are implemented** (core taxonomy + CLI mapping; see
"Implementation" below). Phases 3–4 (MCP `serve.go`, sidecar report) remain
open.

## Context

How a failure is *classified* is a domain concept — "no results" vs "bad
input" vs "an upstream dependency (GitHub/gh auth) failed" is a property of
the reconnaissance operation, not of the surface that presents it. Today
that concept exists in exactly one frontend and is re-derived or discarded
in the others:

- **CLI — has the taxonomy.** `internal/cli/errors.go` defines a clean
  semantic exit-code taxonomy (`ExitOK=0`, `ExitNoResults=1`,
  `ExitBadInvocation=2`, `ExitUpstreamFailure=3`), an `ExitError` type,
  `WithExitCode`/`CodeForError`, and — since ADR-0028.1 / the A4 affordance
  batch — an `upstreamRules` classifier that matches GitHub/gh failure
  signatures (rate-limit, `401`/bad-credentials, `404`/could-not-resolve) to
  recovery hints.
- **Core (`internal/ghx`) — has no failure-class at all.** It returns ~18
  bare `fmt.Errorf`/`errors.New` values with no type. The domain layer that
  actually *knows* whether GitHub returned 404 vs 401 vs empty results throws
  that knowledge away as a string; the CLI then re-classifies it by matching
  English substrings of the very error core produced. That substring round-trip
  is fragile (W2 flagged it: a GitHub phrasing change silently drops a class)
  and it is unavailable to any non-CLI frontend.
- **MCP — flattens everything.** `internal/cli/serve.go` has 16
  `NewToolResultError(err.Error())` sites: every failure, whatever its class,
  becomes one opaque error string. A calling agent cannot tell "no results"
  (refine the query) from "auth failed" (fix credentials) from "bad input"
  (fix the call) — exactly the recovery signal the CLI's A4 work exists to give.
- **Sidecar — ad-hoc.** 83 `fmt.Errorf` across 56 files, no shared class; a
  turn that fails upstream and a turn that fails on bad config are
  indistinguishable to the report contract.

The result: the affordance intelligence W2 built for the CLI (workstream A4)
cannot reach the MCP recon tool or the sidecar report, even though the
main-agent consumer of those surfaces (NORTH_STAR capability 3) needs it more
than a human at a terminal does. Failure-class is a domain concept core does
not export — the M2 finding.

## Decision (proposed)

Lift failure-class into core and map it once per frontend.

1. **Core owns the taxonomy.** Introduce in `internal/ghx` a small
   `FailureClass` enum — `ClassNone`, `ClassNoResults`, `ClassBadInput`,
   `ClassUpstream` (names TBD at acceptance; they mirror the CLI's proven four)
   — and a typed error that carries it, e.g. `ghx.Error{Class, Err, Hint}`
   with `errors.As` support. The GitHub/gh **upstream sub-classification**
   (rate-limit vs auth vs not-found) moves from `cli/errors.go` into core where
   the HTTP status/response is actually known, so classification happens once,
   at the source, from structured signals rather than re-parsed English.

2. **Frontends map, never re-derive.** Each frontend has exactly one mapping
   from `FailureClass` to its own idiom:
   - **CLI**: `FailureClass → exit code` (the existing 0/1/2/3 mapping) plus the
     A4 affordance hint. `errors.go` keeps `ExitError`/`CodeForError` but sources
     the class from core instead of substring-matching.
   - **MCP**: `FailureClass →` a structured tool error whose payload names the
     class (and hint), so an agent can branch on it. Replaces the 16 opaque
     `NewToolResultError(err.Error())` flattenings.
   - **Sidecar**: `FailureClass →` a typed field on the report/uncertainty, so
     a failed turn says *why* in the contract, not only in a log line.

3. **One classifier, tested once.** The signature/behaviour table that maps a
   raw GitHub/gh failure to a class lives in core with its own tests; the A4
   hint text can stay near the CLI or move to core alongside the class
   (decide at acceptance). Frontends get classification for free.

## Consequences

- **A4 affordances become universal.** The recovery signal an agent needs
  ("auth failed → `gh auth login`") reaches the MCP tool and the sidecar
  report, not just the terminal — directly serving the main-agent-as-consumer
  goal (NORTH_STAR capability 3) and the north-star filter ("make the evidence
  more auditable / recon more proficient").
- **Fragility removed.** Classification stops depending on re-parsing English
  error strings the process itself just generated (W2's flagged risk).
- **Scope / cost.** Touches core plus ~18 core error sites, 16 MCP sites, and a
  subset of the 83 sidecar `fmt.Errorf` (only those that surface a
  user/agent-visible failure need the class; internal plumbing errors can stay
  bare). This is a phased migration, not a big-bang rewrite — see below.
- **Public-surface change (MCP).** Structuring the MCP error payload changes
  what the tool returns on failure; it is additive (the class is extra
  structure) but agents that parse the error string should be considered. Note
  at acceptance whether this is an ADR-0032.1 recon-identity concern (the tool
  *schema* is unchanged; only failure *payloads* gain structure).

## Migration (phased, only after acceptance)

1. Add `FailureClass` + typed error + the upstream classifier to `internal/ghx`
   with tests; no frontend behaviour change yet (core still returns errors that
   the CLI substring-matcher continues to handle).
2. Switch the CLI to source class from core; delete the substring `upstreamRules`
   duplication. Prove exit codes are byte-identical against the current A4 tests.
3. Structure the MCP error payloads; add agent-facing class/hint.
4. Thread the class into the sidecar report contract for agent-visible failures.

Each phase is independently shippable and independently verifiable, consistent
with ADR-0025's incremental-delivery posture.

## Implementation (phases 1–2, as built)

**Phase 1 — core owns the taxonomy** (`internal/ghx/failure.go`).
`FailureClass` is `ClassNone/ClassNoResults/ClassBadInput/ClassUpstream`
(mirroring the CLI's exit 0/1/2/3). `ghx.Error{Class, Err, Hint}` carries it;
`Error()`/`Unwrap()` delegate to `Err` so the rendered text and the wrapped
chain (e.g. an `*api.HTTPError`) stay intact — the message is byte-identical to
the pre-existing error. `ClassifyUpstream(err) (FailureClass, string)` reads
**structured signals first** — `*api.HTTPError.StatusCode` and the
`*api.GraphQLError` item `Type` (NOT_FOUND/FORBIDDEN/RATE_LIMITED), reachable via
`errors.As` even through `fmt.Errorf("...: %w", ...)` wrapping — then falls back
to the migrated substring table for transport errors that carry no status. The
ordered switch preserves the CLI's historical first-match precedence
(rate-limit → auth → not-found → forbidden → network) so ambiguous messages
resolve to the same hint. Core functions wrap upstream failures at the source
(`Search`, `Explore`, `Repos`, `fetchTree`, `Read`) as `ClassUpstream`;
`ParseRepo` and `Inspect`'s empty-query are `ClassBadInput`. The auth hint reuses
the sidecar preflight's `gh auth login` / GH_TOKEN remediation wording. Table-
driven classifier tests live in `internal/ghx/failure_test.go`.

**Phase 2 — CLI maps class → idiom** (`internal/cli/errors.go`, `ghx.go`,
`tier2.go`). `CodeForError`/`ExitError`/`WithExitCode` stay; a single
`coreError(err)` reads the class + hint from the `ghx.Error` (via `errors.As`)
and wraps with the matching exit code and the `→` affordance line. The
duplicated `upstreamRules`/`upstreamAffordance`/`upstreamError`/`ghxCoreError`
substring machinery is **deleted** — the CLI no longer re-classifies English
strings; an error without a `ghx.Error` (e.g. a local tier-2 git/tool failure)
is classified once through the shared core classifier and defaults to upstream,
preserving the prior always-exit-3 behavior for those paths.

**Three dogfood exit-code frictions fixed** (`docs/dogfood/FRICTION.md`,
2026-07-07). F1 (`explore badslug` → exit 2) and F3 (live 401/403 affordance,
exit 3) were already behaviorally correct on mainline (post-v2.6.0) and are now
sourced from the core class with **byte-identical** output. F2 was the genuine
remaining bug: core `Read` swallowed a repo/ref-level GraphQL failure to
`NotFound` with a nil error, so `read nonexistent/repo README.md` exited 0. It
now surfaces the classified upstream error (exit 3 on a 404); when the repo
resolves but zero requested paths do, the CLI returns `ExitNoResults` (exit 1)
after still printing the per-file `(not found)` lines. Byte-identical exit codes
+ stderr were proven for every already-correct case by diffing the pre-change
binary against the new one; only the two F2 cases (exit 0 → 3, exit 0 → 1)
change. No eval anomaly detector or scorer depends on ghx CLI exit codes
(verified against `internal/sidecar/evals/`).

## Considered and rejected

- **Leave the taxonomy in the CLI, duplicate it into MCP/sidecar.** Rejected:
  three copies of a domain concept is precisely the divergence M2 flags; it
  fails the ADR-0007 core/frontend rule and the "no per-frontend divergence"
  tenet.
- **A generic error-code integer shared by ad-hoc convention.** Rejected: a
  typed `FailureClass` with `errors.As` is safer and self-documenting; magic
  integers invite the same drift.
- **Do nothing (it's Medium severity).** Rejected as the default but noted:
  the cost is real and the host-task/eval work is the current frontier, so
  this waits for acceptance and is sequenced behind in-flight commitments —
  registering the decision now (per "write the ADR before substantial
  implementation") is the point, not rushing the build.
