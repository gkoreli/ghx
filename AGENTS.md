# AGENTS.md

## Project

ghx — GitHub code exploration for AI agents. Go binary distributed via npm, Homebrew, and `go install`.

## North Star

`docs/NORTH_STAR.md` is the durable steering document. The project creates two
things: the Agent Sidecar Framework (a new mental model — expensive main agents
delegate a whole competence domain to a cheap specialized sidecar) and ghx (the
code reconnaissance sidecar, the only tool an agent reaches for to explore the
GitHub open-source world). End state: the main agent needs zero ghx CLI
knowledge — the CLI, codemap, and local codemapping all become tools under the
sidecar brain, proven better by pre-registered benchmarks. Before taking on
substantial work, check it passes the north-star filter (removes tokens from
the main agent's context, cheaper/more proficient reconnaissance, more
auditable evidence, or better training trajectories) and identify which
milestone it advances. Update the milestone table there when status changes.

## Architecture

```
cmd/ghx/             — Go binary entrypoint
internal/cli/        — CLI commands (cobra)
internal/ghx/        — core library (explore, read, search, repos, tree, glob)
internal/codemode/   — JS executor (goja sandbox, esbuild transpilation, type generation)
internal/mapengine/  — parser-backed structural map engine
internal/sidecar/    — sidecar runtime, sessions, reports, ACP integration
skills/             — CLI and MCP agent skills, embedded into binary via go:embed
npm/             — platform-specific npm packages (one per OS/arch, contains Go binary)
scripts/         — CI and release scripts
```

## ADR-Driven Engineering

Use ADRs as living engineering records, not one-time proposals. For substantial
architecture, product, workflow, sidecar, eval, release, or agent-behavior
changes, write or update an ADR before implementation, engineer against it, and
then update the ADR after implementation with the decisions that were made while
building.

ADRs live in `docs/adr/` and use YAML frontmatter. Preserve the existing style:

```markdown
---
title: "ADR-0016.1: Sidecar Eval Implementation — Minimal Validation Kernel"
date: "2026-07-03"
status: "proposed"
parent: ADR-0016
thread: "sidecar-agentic-eval"
author: "Goga Koreli"
---
```

Thread related decisions with decimal numbering when the work belongs to an
existing decision family, for example `0015.1`, `0015.2`, or `0016.1`. Use
`parent`, `thread`, `supersedes`, or cross-reference sections when that
relationship is important. Do not flatten related follow-up decisions into a
single vague ADR.

Before writing an ADR:

- understand the relevant code paths, module boundaries, and runtime behavior
- read the directly relevant prior ADRs
- ground claims in evidence from code, docs, tests, command output, or committed
  artifacts
- do not assume how existing code works without checking authoritative sources

An ADR should include distilled sections with rationale, insights, bullets, and
cross-references. Prefer clear claims that explain why a decision is being made,
what alternatives were rejected, and what evidence supports the conclusion.
When referencing another ADR, source file, command, or test, explain why that
reference matters.

During implementation, keep track of engineering decisions made on the fly:
boundary changes, API shape changes, permission/runtime constraints, test
strategy, rejected approaches, and mismatches between the initial design and the
actual code.

After implementation, update the ADR so it is not a misleading historical plan.
Record what changed, what was learned, what decisions were revised, and what
follow-up remains. If the implementation disproves the original direction,
state that directly and update `status` or add a superseding ADR instead of
leaving stale guidance.

## Evidence Contract

Any completed task — delegated or direct — must report evidence, not vibes.

Required report shape:

```text
answer
files changed or inspected
commands run
test results
evidence snippets or line references
uncertainty
suggested next step
```

For repository exploration, prefer reports shaped like:

```text
question
answer
relevant files
evidence snippets
commands run
backends used
uncertainty
suggested next reads
```

A result that cannot cite files, commands, or outputs is a hypothesis, not a
finding.

## Visibility and Truthfulness (core tenet)

Full visibility for both humans and agents into every measurement, score, and
decision. Trust in the eval framework is earned by auditability, never by
assertion. Binding rules:

- **Every score must be recomputable by a human from committed artifacts.**
  Scoring code is reviewed, unit-tested, deterministic Go
  (`internal/sidecar/evals/rewards.go`, `gates.go`) — who scored, from which
  episode fields, by which rule must always be answerable from the repo.
- **The measurement stack is frozen during a run.** Any scorer/detector change
  mid-run must be pre-registered in an ADR before rescoring (precedent:
  the compliance-detector fix, commit `afd6e99`).
- **Eval truthfulness, deterministic or not, is a top ideology — we must not
  lie to ourselves or our customers** (Goga, 2026-07-05). Deterministic
  checks (substring/suffix matching against pre-registered ground truth) are
  frozen-in-time baselines: reproducible, cheap, gameable-in-principle, and
  honest only about what they actually measure — "did the pre-registered
  facts appear", never "was the reasoning, tool use, or output *better*".
  Exploration is a non-deterministic task; ranking the quality of plain vs
  ghx vs ghx-sidecar trajectories requires a judge scorer. Neither layer may
  masquerade as the other: never present an indexOf-style check as a quality
  judgment, and never present a judge opinion as ground truth.
- **The judge scorer is required, not optional** (upgrades ADR-0016.1's
  deferral): full confidence in north-star claims needs both layers —
  deterministic gates as the frozen backstop, a calibrated judge for
  trajectory/reasoning/output quality. The judge is never the gate alone;
  it must emit full OTel reasoning/thinking traces; its calibration
  (agreement with hand labels, false-positive/false-negative rates) is
  measured and reported next to its scores; its prompt and model version
  are committed. Disagreement between the two layers is a first-class
  signal to investigate, not noise to average away.
- **Traces for everything, viewable by humans.** Eval runs, sidecar sessions,
  and ghx invocations emit OTel traces; the target state (NORTH_STAR M6) is a
  single visibility surface where a human can hand-check any agentic
  trajectory, score, and comparison in a UI — not only grep JSON.
- **Truthfulness over optics.** Verdicts self-label their limitations
  (PRELIMINARY, caveats, futility stops); negative results are committed and
  kept (`docs/evals/gate-run-2026-07/`), never buried or rerun-until-green.
- **Read the M4 verdict as a conservative floor, and never quote the floor as
  a ceiling** (Goga, 2026-07-05). THESIS SUPPORTED was earned by a first-cut
  sidecar persona, scored only by deterministic fact-recall gates. Both
  remaining levers are expected to widen the sidecar's advantage over plain
  and direct-ghx: (1) sidecar ergonomics/proficiency work (M5 onward — the
  ADR-0016.7 reliability fixes alone moved correctness 0.580 → 0.908), and
  (2) scoring fidelity (the judge scorer and richer metrics, which can see
  trajectory/reasoning quality that substring gates cannot). That expectation
  steers the roadmap — keep improving the agent, keep improving the
  measurement — but expectations are never citable results: any "even better"
  claim waits for the re-measured run that shows it.

- **Full gate runs never block engineering** (Goga, 2026-07-05). The
  measurement ladder (ADR-0016.3) assigns roles: feature-level go/no-go
  rides smokes and spot checks (minutes); full pre-registered runs exist
  only to re-ground the citable verdict after product deltas accumulate,
  and they run in the background while engineering continues. If a full
  run ever sits between an engineer and their next feature, the process
  is being used wrong — the only thing a full run gates is quoting new
  numbers. Reducing the cost of full-rigor runs (baseline reuse, bounded
  parallelism, sequential early-stopping) is standing eval-framework work,
  each change pre-registered by ADR before use.

## Open Source Leverage (core tenet)

Do not hand-roll OTel variants, eval formats, or frameworks when official
standards and open-source tools exist. We build with velocity by exploring
open source for ideas, libraries, and established patterns — "good artists
copy, great artists steal" in the Picasso sense: openly, vocally, with
attribution and stewardship (MIT in, MIT out). Binding rules:

- **Official formats exactly, not approximately.** Agent traces are OTLP
  per the actual spec (including its deviations, e.g. hex span IDs — the
  2026-07-05 protojson lesson), so industry tooling works within seconds.
  Custom data (our metrics, rewards, reports) may ride alongside in
  attributes or sibling files, never as a mutation of the standard format.
- **Engineer new things only when nothing existing serves the need or the
  vision.** ghx and the Agent Sidecar Framework pass that test; a trace
  format or a viewer does not. When something almost serves, prefer
  adopting it or being inspired by it (e.g. codemap as a future internal
  sidecar tool per NORTH_STAR M7) over rebuilding it.
- **Be vocal about inspiration.** Credit upstream projects in docs and
  README; hiding influences is both bad stewardship and bad marketing.

## Tool Economy

Match the tool to the information, not to what is available (incident:
2026-07-05, a delegated research agent browsed documentation via Chrome
screenshots when WebSearch existed — pure token waste).

- Research, docs reading, and web lookups use **text-only tools**:
  WebSearch, WebFetch, `curl`, `gh`, package registries, or ghx itself.
- Browser automation / computer use (screenshots, rendered pages) is
  reserved for tasks that inherently need a rendered UI — visual
  verification, form flows, UI debugging. Never for reading text.
- Prefer the cheapest faithful representation at every step: structured
  output over prose, `--json` over scraping, a targeted range read over a
  whole file, a grep over a directory listing.
- The optimization target is signal per token (NORTH_STAR "The Moat") —
  it applies to how agents work on this repo, not only to the product.

## Commits

Commit at meaningful checkpoints, not only at the end of a task. A meaningful
checkpoint is a coherent, verifiable unit: an ADR written or updated, a test
slice passing, a completed refactor step, a committed eval verdict. Rules:

- Follow the existing message style: `type(scope): summary` — e.g.
  `feat(evals):`, `fix(sidecar):`, `docs(adr):`.
- Code checkpoints must build and pass `go test ./...`, or the commit message
  must say why tests were skipped.
- Never push unless explicitly asked — pushing `package.json` changes to
  `mainline` triggers the release pipeline.
- `.ghx-evals/` is gitignored; curated eval evidence goes under `docs/evals/`.
- Never commit secrets, tokens, or personal config.

## Version Bumping

**Always use the bump script.** Never edit `package.json` version manually.

```bash
node scripts/bump.js          # patch (default)
node scripts/bump.js minor
node scripts/bump.js major
```

This updates both `version` and all `optionalDependencies` in `package.json`. The CI pipeline triggers on `package.json` changes to `mainline`: auto-tags → GoReleaser builds 6 platform binaries → npm publishes all 7 packages with OIDC provenance.

## Release Flow

1. `node scripts/bump.js` — bump version
2. `git commit` + `git push` — triggers CI
3. CI: tag → GoReleaser (GitHub release + Homebrew tap) → npm publish (OIDC, no token needed)

## SKILL.md Files

`skills/ghx/SKILL.md` and `skills/ghx-mcp/SKILL.md` are the canonical skill documents. They are embedded into the binary via `go:embed` from `skills/doc.go`. If you modify them, the binary must be rebuilt for changes to take effect. The `ghx skill` and `ghx skill --mcp` commands print the embedded content.

## Build

```bash
go build -o ghx ./cmd/ghx
```

## Test

```bash
go test ./...
```
