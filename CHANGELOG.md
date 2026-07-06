# Changelog

All notable changes to `ghx` are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Where a change carries proof, the entry links the ADR that decided it or the
committed eval verdict that measured it — the artifacts are the evidence, not the
prose here.

## [Unreleased]

_Nothing released yet. Work in progress lands here before it ships._

## [2.2.0] — 2026-07-06

The release that turns the sidecar from a working prototype into something you
can hand to another agent and trust. The headline is resilience — an exploration
is never silently lost — plus one-command setup, discovery without a repo, a
built-in trace viewer, an artifacts pointer on every answer, a batch of
agent-facing CLI ergonomics, and the eval machinery that lets the whole thing
carry a citable verdict.

### Added

- **Sidecar runtime resilience — never lose an exploration.** A liveness watchdog
  and wrap-up recovery keep a turn alive through slow or stalled model output, and
  a failed turn still writes its artifacts and returns a partial report instead of
  vanishing. Reports are now evidence-required at submission time: an answer with
  no traceable evidence is rejected before it can reach you
  ([ADR-0027](docs/adr/0027-runtime-resilience.md)).
- **One-command setup.** `ghx sidecar config init --claude-acp` writes a working
  `~/.ghx/config.json` using the claude-agent-acp adapter — setup is the hardest
  step, so it is one command. Re-running against an existing config shows a field
  diff and refuses to overwrite without `--force`. `ghx sidecar doctor` now also
  verifies that the report-sink MCP binary matches the running ghx version and
  fails loudly with fix-it text, since a stale sink would silently degrade
  structured reports to a text fallback
  ([ADR-0019](docs/adr/0019-sidecar-adoption-zero-cli-surface.md) D4).
- **Discovery tier — `--repo` is now optional scope.** With `--repo`: repo-scoped
  reconnaissance as before. Without it: discovery — "which repos/libraries do X" —
  where the sidecar sweeps GitHub for candidates and reads into the top ones before
  claiming anything, deriving a resumable session slug from the question
  ([ADR-0019.1](docs/adr/0019.1-discovery-tier.md)).
- **`ghx sidecar view` — a local trace UI in one command.** Absorbs the
  [otel-desktop-viewer](https://github.com/CtrlSpice/otel-desktop-viewer) replay
  recipe: it starts the viewer and loads a session's `traces.jsonl` (plus
  logs/metrics when present). No argument replays the most recent session;
  `--list` shows sessions with turn/report counts; `--port` sets the UI port.
  Everything it renders is the same file you can read by hand
  ([ADR-0026.1](docs/adr/0026.1-sidecar-view-absorption.md)). Requires the viewer
  on PATH.
- **Artifacts pointer in every response.** Every ask answer — human output,
  `--json`, or the MCP recon tool — ends with an `artifacts:` pointer naming the
  session directory and the ask's root trace ID, so the calling agent never has to
  guess where the audit trail lives. The `--json` envelope carries it as
  `{"report": {...}, "artifacts": {"sessionDir": "...", "traceId": "..."}}`.
- **`ghx grep`** — a grep-like repo search front door (`ghx grep <owner/repo>
  "pat" --path <dir> --limit N`), alongside the existing `search`, as part of the
  CLI ergonomics batch ([ADR-0028.1](docs/adr/0028.1-cli-ergonomics-batch.md)).
- **Judge gold-set labeling protocol and packet generator.** A deterministic
  generator (`go run ./docs/evals/judge-goldset/gen`) assembles the candidate pool
  from the committed gate runs, applies the same D7 anomaly pre-filter the judge
  runner uses, and writes profile-blind labeling packets — so human gold labels and
  judge scores are produced from the same evidence on the same rubric. The written
  labeling protocol is committed alongside
  ([ADR-0023.1](docs/adr/0023.1-judge-scorer-decision.md);
  `docs/evals/judge-goldset/PROTOCOL.md`).
- **Real cross-family judge client** behind the ADR-0023.1 D4 seam, plus the
  offline judge scorer machinery. No judge score is citable before calibration
  κ ≥ 0.6 against the gold labels; below 200 labeled episodes the judge layer
  self-labels PRELIMINARY regardless of κ — a threshold not yet crossed, so judge
  scores remain **PRELIMINARY** ([ADR-0023.1](docs/adr/0023.1-judge-scorer-decision.md)).

### Changed

- **CLI ergonomics batch — errors that teach.** Bad invocations now return
  actionable, teaching error messages; truncation text names the exact `--budget`
  or `--full` flag to lift or narrow the result; `read` accepts hidden
  agent-guess range aliases (`--start/--end`, `--offset/--limit`) for frictionless
  retries while help converges on one spelling; exit codes are semantic (`0` ok,
  `1` no results, `2` bad invocation, `3` upstream failure); `ghx --version` is a
  first-class health check. Agents are ghx's users, and their traces were the
  usage study that drove this batch
  ([ADR-0028.1](docs/adr/0028.1-cli-ergonomics-batch.md); mining evidence at
  `docs/evals/mining-2026-07-06/CLI-FINDINGS.md`).
- **Sidecar persona revised (revision 1)** for report discipline: tighter
  read-ladder guidance, sharper search doctrine, and more compact reports
  ([ADR-0029](docs/adr/0029-persona-revision-1.md)).
- **Eval run economics — baseline reuse.** Each run takes an identity inventory
  and reuses matching baseline (`plain`/`ghx`) episodes across runs with `>= trial`
  semantics, so only the changed profile is re-run; a mismatch triggers a loud
  refusal rather than a silent, expensive fallback. Bounded episode parallelism and
  sequential stopping cut run cost further
  ([ADR-0025](docs/adr/0025-eval-run-economics.md),
  [ADR-0025.1](docs/adr/0025.1-baseline-reuse-implementation.md)).
- **Measurement fix batch:** union-of-turn-reports scoring (so delta-reporting is
  not penalized), an answer-doc contamination guard, and sufficiency honesty in the
  scorer ([ADR-0016.8](docs/adr/0016.8-measurement-fix-batch.md)).
- **README** now leads with the Agent Sidecar Framework positioning: ghx as the
  code-reconnaissance sidecar a main agent delegates to, with the standalone
  exploration CLI documented underneath.

### Fixed

- **Stale ACP session IDs downgrade to a fresh session** instead of failing the
  ask, so a recycled agent process no longer breaks a resumed investigation.
- **Report reliability:** strict judge parsing rejects trailing top-level values;
  the shared-artifacts JSONL appender is now per-path locked to prevent interleaved
  writes under parallelism ([ADR-0025](docs/adr/0025-eval-run-economics.md) D3).
- Removed the legacy `~/.ghx-sidecar` read-fallback; `~/.ghx` is the single
  product root.

### Measured

- **Citable verdict: THESIS SUPPORTED.** The `gate-run-2026-07-06-fixbatch` run —
  the first measured under the ADR-0016.8 measurement fix batch and the ADR-0027
  resilient runtime — passes all five pre-registered gates: correctness
  sidecar/ghx ratio **0.983** (floor 0.90), evidence **0.887** (floor 0.70),
  compression **≈16.6×** (threshold 2.9×), memory resume **1.00** with repeat-read
  0.427 < ghx 0.483, and safety **1.0** on all sidecar episodes. Sequential
  stopping reports STOP-SUCCESS-LOCKED: no remaining episode could have changed any
  thesis gate ([verdict](docs/evals/gate-run-2026-07-06-fixbatch/verdict.md),
  [run dir](docs/evals/gate-run-2026-07-06-fixbatch/)).
- **Independent cross-family audit.** A different model family (OpenAI Codex, run
  read-only) adversarially recomputed all 90 episode scores of the confirmatory run
  exactly and confirmed the gate outcomes, logging the provenance defects it found
  in the open ([audit report](docs/evals/audit-2026-07-05-independent/REPORT.md)).
- The honest negative that drove these fixes is kept alongside the verdicts at
  [docs/evals/gate-run-2026-07/](docs/evals/gate-run-2026-07/). Verdicts are read
  as a conservative floor; the artifacts are the proof either way.

[Unreleased]: https://github.com/gkoreli/ghx/compare/v2.2.0...HEAD
[2.2.0]: https://github.com/gkoreli/ghx/compare/v2.1.19...v2.2.0
