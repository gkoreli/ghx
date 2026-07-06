// Package hosttask implements slice S1 of the combined-objective host-task
// eval class (ADR-0032.1 D5): the workspace provisioner and the deterministic
// outcome grader. It is self-contained — nothing here touches the eval
// runner, profiles, gates, or persona (those are slices S2–S4).
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
// the eval layer (S2+) translates into a BLOCKED anomaly; S1 reports it
// honestly and nothing more. No LLM is involved anywhere in this package.
package hosttask
