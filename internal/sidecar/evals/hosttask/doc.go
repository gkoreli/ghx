// Package hosttask implements slices S1 and S2 of the combined-objective
// host-task eval class (ADR-0032.1 D5): the workspace provisioner, the
// deterministic outcome grader (S1), and the workspace scope + structural
// exploration/engineering token-attribution classifier (S2). Nothing here
// touches the eval runner, profiles, gates, or persona (arm wiring and the
// corpus are slices S3–S4).
//
// # Task fixture schema
//
// One host task is one hand-authored JSON fixture (ADR-0032.1 D3: six tasks,
// committed with authoring provenance). A fixture pins everything the grader
// needs to be deterministic — repository SHA, container image, and the exact
// command lists — following the "thin task-container convention" ADR-0032.1
// decided on: steal SWE-bench's invariants (pinned SHA, hidden test patch
// semantics, fail-to-pass + pass-to-pass), not its harness.
//
//	{
//	  "id": "chi-middleware-order",
//	  "workspaceRepo": "acme/chi-service",
//	  "pinnedSha": "8f0c1d2e3a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d",
//	  "image": "golang:1.24-bookworm",
//	  "network": "none",
//	  "setupCmds": ["go mod download"],
//	  "failToPassCmds": ["go test ./mw -run TestAbortPropagates"],
//	  "passToPassCmds": ["go test ./mw -run TestChainBasic"],
//	  "explorationSubQuestions": [
//	    {
//	      "question": "In go-chi/chi v5.0.12, does a middleware calling ...",
//	      "groundTruth": "next.ServeHTTP is never invoked; the chain stops",
//	      "verifiedAt": "2026-07-06"
//	    }
//	  ],
//	  "memorizationCanary": {"checkedAt": "2026-07-06", "result": "pass"}
//	}
//
// Field semantics:
//
//   - id: stable fixture identity; used in workspace directory names and
//     results, so it must be a safe path segment.
//   - workspaceRepo / pinnedSha: the host agent's workspace, an "owner/repo"
//     GitHub repository materialized at exactly this commit. The pin is what
//     makes the grade reproducible (SWE-bench invariant #1).
//   - image: the container image the grader runs commands in — the fixture's
//     environment pin. Images must provide /bin/sh. Amortized per task, never
//     rebuilt per trial.
//   - network: docker network mode for grader containers ("" defaults to
//     "none" — hermetic grading is the default; a fixture must opt in to
//     network explicitly).
//   - setupCmds: environment preparation (dependency download, build). Run
//     before any test command, in order, inside the container; a setup
//     failure aborts the attempt's remaining commands. Setup effects that
//     land outside the bind-mounted workspace live only as long as the
//     container (one grade attempt); effects inside the workspace persist
//     across attempts, so setup commands should be idempotent.
//   - failToPassCmds: commands that FAIL on the pinned SHA and must PASS
//     after a correct fix (SWE-bench F2P). The test patch/expectation they
//     encode is withheld from the agent — only the grader runs them.
//   - passToPassCmds: commands that pass on the pinned SHA and must still
//     pass after the fix (SWE-bench P2P — the no-regression half). Keep the
//     set small and deterministic (ADR-0032 trap 7).
//   - explorationSubQuestions: the pre-registered H2 ground truth — 1–3
//     facts the correct exploration establishes, each verified against the
//     pinned dependency version at authoring time (verifiedAt is that
//     provenance date). S1 only carries them; scoring is later slices.
//   - memorizationCanary: authoring-time provenance of the ADR-0032 trap-3
//     check — the subject model was asked for the fix from the issue text
//     alone, with no repository access. result "pass" means no from-weights
//     fix appeared; a "fail" fixture is disqualified and Validate rejects it.
//   - objective (S4, additive/optional): the issue/prompt text the host is
//     run with. Validate keeps it optional so the S1–S3 example and unit
//     fixtures still validate; the committed corpus carries it and LoadCorpus
//     enforces it (RequireCorpusReady).
//
// # Committed corpus (S4)
//
// The six hand-authored tasks (ADR-0032.1 D3) live as committed JSON under
// corpus/ with a provenance README. LoadFixtures is the lenient schema-only
// loader the S1–S3 tests use against testdata/fixtures; LoadCorpus is the
// strict loader for the real corpus/ directory — it additionally requires the
// objective. Each corpus fixture pins its workspace repo at the commit just
// before an upstream fix (bug present at pinnedSha) and injects its hidden F2P
// test via a setup command, so the agent never sees the grading criterion
// (SWE-bench's hidden-test invariant, expressed with the existing schema).
//
// # Provisioner
//
// Provisioner materializes one fresh workspace per trial: a brand-new
// directory (never reused — the per-trial reset guarantee, ADR-0032 trap 5)
// containing workspaceRepo checked out at exactly pinnedSha. It reuses
// tier2's pinned-SHA shallow-fetch plan (tier2.SHAFetchPlan) but with full
// blobs: unlike tier2 recon snapshots, a host workspace must work offline —
// a blobless promisor clone would lazy-fetch over the network, and grader
// containers default to network "none".
//
// # Grader
//
// Grader runs setupCmds, then failToPassCmds, then passToPassCmds inside the
// fixture's image with the workspace bind-mounted at /workspace, recording
// per-command pass/fail. The grade is the ADR-0032.1 GH1 form:
//
//	grade = failToPass fraction × passToPass preservation
//
// Flake rule (ADR-0032 trap 7): a fully passing first attempt is final; any
// command failure triggers exactly two re-runs, and any per-command
// pass/fail flip across the three attempts marks the result Flaky — the
// TASK is disqualified and the grade must never be scored. A stable failure
// is a real grade.
//
// Docker being unavailable surfaces as ErrDockerUnavailable, a typed error
// the eval layer (S3+) translates into a BLOCKED anomaly; S1 reports it
// honestly and nothing more. No LLM is involved anywhere in this package.
//
// # Workspace scope and token attribution (S2)
//
// WorkspaceScope is the single owner of the "inside the trial workspace"
// judgment: absolute paths, `..` cleaned, symlinks resolved through the
// deepest existing ancestor (with documented TOCTOU limits). The evals
// package's host write policy composes it to allow workspace-scoped writes
// and deny everything outside (ADR-0032.1 D5.2); Classifier composes it to
// attribute host tool calls to exploration vs engineering by tool identity
// and path scope only, producing a per-episode AttributionTable whose
// totals a human can recompute row by row (visibility tenet; ADR-0032
// trap 6). Chars are the fallback accounting — real gen_ai.usage.* token
// counts are the registered GH1-efficiency metric (ADR-0032.1 D2).
package hosttask
