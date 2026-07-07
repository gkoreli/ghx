---
title: "ADR-0036: Target Architecture — the Runner Port, Composition Root, and Domain Flow"
date: "2026-07-07"
status: "proposed"
thread: "architecture"
author: "Fable (synthesis of five architecture-vision audits)"
---

# 0036. Target Architecture — the Runner Port, Composition Root, and Domain Flow

## Status

Proposed. This is the **strategic** architecture ADR that ADR-0035 (tactical
tech-debt cleanup) pointed toward. It answers the founder's 2026-07-07
question: what should ghx's architecture *be* so it stays maintainable, so the
mental model is clean, and — the centerpiece — so the **agentic runner is
plug-in-replaceable by configuration** (add a Codex runner; entirely swap
claude-agent-acp for the Claude Agents SDK) because the runner is decoupled
from the rest of the product.

It supersedes ADR-0035 item 5 (the tactical "Harness seam") and reframes it as
the first slice of a real Runner port. It does **not** change eval scoring,
gates, detectors, or any committed benchmark claim; the one phase that changes a
measurement source (the in-process SDK runner's real-token accounting) is
explicitly ADR-gated and last.

## Context

Five independent read-only architecture-vision audits were run 2026-07-07 with
deliberately different personas and scopes, each grounded in idiomatic Go
(cited) and ghx `file:line`. They live under `docs/audits/architecture-vision/`:

- **Go architecture & boundaries** (high-level, long-term)
- **Domain model & ubiquitous language** (mid/low-level)
- **Runner port & pluggable runtimes** (the centerpiece)
- **Resilience & runtime robustness** (resilience)
- **Adversarial YAGNI skeptic** (short-term, risk — the counterweight, GPT-5.5)

**The convergence is the signal.** Two independent auditors (runner-port and
resilience) arrived at the *same* boundary design — typed failure `Outcome`
returned by the adapter, marker strings owned by the runtime and kept
byte-identical so the eval-anomaly detector never learns which runtime ran. The
boundaries auditor independently named the same missing seam (a composition
root) that the runner port needs to be wired cleanly. The skeptic did not reject
the runner port — it endorsed a *narrow* one and rejected everything around it
(layer packages, DI containers, framework extraction). The design below is where
they agree, with the disagreements resolved explicitly, not averaged.

**The keystone fact (verified, runner-port audit):** the measurement-facing side
is *already* neutral — `TurnResult` and `ToolCallTrace` (`internal/sidecar/turnresult.go`)
import only `time`, and the whole eval/telemetry stack reads those plain structs.
So swapping runtimes needs **no measurement-stack change**; an adapter just
repopulates the same struct. That is what makes this migration low-risk.

## Decision

Adopt four coordinated architectural decisions and execute them as one phased,
non-rewrite sequence. Each item is behavior-preserving unless it says otherwise;
each ships independently and is landed rebase+ff.

### D1 — The Runner port (the centerpiece)

Introduce a small, consumer-defined **Runner port** so the agentic runtime is
one adapter behind an interface, selected by config. Adopt the runner-port
audit's design verbatim as the target (`docs/audits/architecture-vision/runner-port-pluggability-2026-07-07.md`):

- **Interfaces:** `Runner{Preflight, Open, Info}` → `Session{Turn(ctx, TurnRequest, EventSink) (TurnResult, ResumeToken, Outcome), Close}`.
- **Neutral types (no `acp`, no subprocess):** `TurnRequest{Prompt, SteeringSpec, TurnBudget}`; `SteeringSpec` (system prompt, isolation, tool policy, model, effort, thinking, report-sink intent, raw-stream flag); `EventSink` (Text/Thinking/ToolStarted/ToolUpdated/Activity/RawStreamMessage); `Outcome{Class FailureClass, Err}`.
- **The two decouplers:** `EventSink` moves classification to the adapter and side effects (stdout, `live.jsonl`, watchdog) to the runtime; `Outcome` moves failure naming to the adapter as a *typed* value so the runtime switches on a class, never on adapter error strings.
- **Location:** a leaf package `internal/sidecar/runner` importing only stdlib, so "no ACP leak past the boundary" becomes a **compile-time** guarantee once a second adapter exists.
- **Config selection:** a `runner` object in `~/.ghx/config.json` with `kind: claude-acp | codex-acp | claude-sdk`; a `NewRunner(cfg)` factory maps kind → adapter; the legacy `agent` string aliases to `{kind: claude-acp, command: <agent>}` at load (a one-line alias, not a dual code path).

Three adapters realize the founder's two goals:

- **claude-acp** — today's code re-homed behind the port; behavior-identical; preserves ADR-0015's Go-native ACP choice (ACP is demoted from *the* runtime to *one* adapter, not deleted).
- **codex-acp** (goal a) — reuses the ACP transport; varies only steering encoding (env vars, not the `claudeCode` `_meta` key), persona placement, permission enforcement, and outcome mapping. The smallest possible second adapter — the ideal proof the boundary holds. It also fixes a **live trap**: config already accepts a `codex` agent, but steering is hard-coded to the `claudeCode` key, so a codex runner today would spawn, pass preflight, and silently drop all steering.
- **claude-sdk** (goal b, P4) — there is no Go binding for the Claude Agents SDK (it spawns the `claude` CLI itself), so the P4-aligned realization is a **pure-Go in-process loop over the official `anthropic-sdk-go`**. This is "below ACP" made literal: no subprocess, no permission wire (deny-writes = never registering a write tool), native thinking (no `thinking.display` archaeology), and **real production tokens** filling `TurnResult.Usage` directly — which closes trust hole H5 (today prod tokens are a char proxy). Because the SDK path is in-process, the port must carry no subprocess/ACP assumption — which is exactly why `AgentCmd`/`Env`/`initialize` do not belong on it.

### D2 — A composition root; no layer packages (the boundaries answer)

The Go-idiomatic answer to "should we use singletons and services?" (boundaries
audit): **services yes** (constructed-once, injected structs with methods — the
code already does this well with `tier2.Service`, telemetry, `TurnRunner`);
**mutable-global singletons no**; **never a DI container** — in Go the
composition root *is* the DI container.

- Add a **composition root** in `cmd/ghx` / `internal/cli` that constructs config
  + resolved paths, the GitHub client, telemetry, the tool registry, and the
  runner **once**, and passes them explicitly. Retire the ambient globals: the
  `githubClients` package var (right interface, wrong container), `rootDir()`
  re-reading env on every path call, and `LoadConfig()` re-invoked in 10+ command
  bodies (folds ADR-0035 item 9).
- Make `internal/ghx` a **pure leaf**: relocate `RegisterTools`/`wrap*` out to the
  composition root so core imports no frontend (removes the `internal/ghx` →
  `internal/codemode` edge).
- **Do not** introduce `internal/domain` / `internal/shared` / role-layer packages.
  ghx's layout is already domain-oriented, which is the idiomatic choice; core and
  sidecar are separated by a *process* boundary (ACP), not a shared-type import,
  so there is no cross-package shared-type problem to solve. Building layers would
  fail the no-speculative-framework tenet (skeptic + boundaries agree).

### D3 — Domain objects that flow (the domain-model answer)

Follow one rule (domain-model audit): **parse at the boundary, flow the type
inward, render at the edge.** ghx already owns the value-object pattern
(`RouteDecision`, the new `ghx.Repo`) but applies it at rest, not in motion — it
parses a string inside a package and discards the type at the boundary. Fix the
motion, selectively:

- **Finish the `ghx.Repo` flow:** public core signatures take `Repo`; type
  `AskRequest.Repo` and `SessionMeta.Repo`; parse once at the CLI/MCP edge.
- **Add `Snapshot{Repo, SHA}`** (genuinely missing): thread the resolved tip SHA
  into `Evidence` so committed evidence is re-fetchable and two reads in one
  investigation cannot silently straddle a moving HEAD. Phase 1 (capture into
  `Evidence`) is additive/observability-only; pinned reads are a later,
  ADR-gated step. This is a real gap in the "Evidence, not vibes" promise.
- **Type `Depth`, `Tier`, `Backend`** at the `AskRequest` boundary (one validator
  each; fixes the depth reject-vs-coerce divergence) — folds ADR-0035 item 7.
- **Immutable `Report` + a presenter** (`FormatText()`), so the CLI stops reaching
  into loose sidecar fields — folds ADR-0035 item 6.
- Selective, not maximalist: `Question` stays `string` (free text, no invariant);
  `TurnResult` stays a transient DTO (only its boolean flags fold into a
  `TurnOutcome`). `SessionMeta` is the one true entity.

### D4 — Resilience: typed fault owns its marker (the resilience answer)

Recovery policy is already in the right layer (the runtime), but failure
*classification* is runner-locked string-matching, which silently no-ops for a
second runner. Split it, and reconcile with the frozen measurement stack the way
the runner-port and resilience audits independently converged on:

- The adapter returns a typed `Outcome{Class}` (D1). The **runtime**, not the
  adapter, writes the **canonical marker string keyed by `FailureClass`** into the
  turn error. The strings stay byte-identical, so `evals/anomalies.go` is
  untouched and a codex or SDK turn produces the exact same anomaly rows — the
  measurement stack never learns which runtime ran.
- The failure model is **two axes, not one**: ADR-0034's `FailureClass` classifies
  the *recon operation* (core → CLI/MCP exit codes / tool errors); this ADR's
  `Outcome` classifies the *turn/transport fault* (sidecar runtime). Add the typed
  layer *below* the marker string (never instead of it — failure-class inventory
  B4). Changing the string↔class mapping is a measurement change (eval ADR);
  adding the typed layer while preserving the strings is behavior-preserving.
- Harden the daemon as a **supervisor** (resilience audit H2/H3/M1): per-request
  `recover()` (net/http discipline) so one turn's panic cannot kill all warm
  sessions; per-request context propagation (today client cancellation is dropped
  at dispatch, head-of-line-blocking the serialized worker); signal-aware graceful
  drain (implements the unimplemented ADR-0030 D6). Zero eval impact, highest
  reliability payoff.

## The phased, non-rewrite sequence

Ordered so every phase is shippable and behavior-preserving, foundations before
the headline, and the one measurement-changing phase last and ADR-gated. The
skeptic's constraint is binding throughout: **do not abstract ahead of a second
implementation; prove the port with codex-acp before generalizing further.**

**Phase A — foundations (zero eval impact, skeptic-endorsed).**
- A1. Daemon supervisor hardening: per-request `recover()`, context propagation, graceful drain (D4; ADR-0030 D6). Biggest reliability win, smallest change.
- A2. Composition-root skeleton + retire ambient globals (`githubClients`, `rootDir()`, repeated `LoadConfig`) (D2; folds ADR-0035 item 9).
- A3. Make `internal/ghx` a pure leaf (relocate `RegisterTools`/`wrap*`) (D2).

**Phase B — domain flow (behavior-preserving, high product value).**
- B1. Finish the `ghx.Repo` flow across the boundary (D3).
- B2. `Snapshot{Repo,SHA}` into `Evidence` (D3) — additive; the product-correctness win.
- B3. Type `Depth`/`Tier`/`Backend` at the boundary; immutable `Report` + presenter (D3; folds ADR-0035 items 6–7).

**Phase C — the Runner port (the centerpiece).**
- C0. A grep-based neutrality guard test (no `acp` import outside adapter files), so the boundary is enforced before it is built.
- C1. Define the port in the consumer package `sidecar`; make today's ACP path implement it, behavior-identical. Promote `TurnRunner` into `Session.Turn`; convert the string-matchers into an adapter-internal `Outcome` mapping; invert `denyClient.SessionUpdate` onto `EventSink`; keep `BuildSessionMeta` untouched, now adapter-private. **This is where the held ADR-0035 item-5 harness exploration is superseded** — it is the right seam at the wrong altitude; C1 raises its altitude (neutral request, opaque resume token, typed outcome). Verify: `go test ./...` + one live `ghx sidecar ask` smoke (same artifacts, same anomaly rows).
- C2. Lift the port + neutral types to a leaf `internal/sidecar/runner`; move the ACP code to `internal/sidecar/runner/acprunner`. The compiler now guarantees the boundary. Pure relocation (mirror the a49ce76 byte-identical-decomposition style).
- C3. **Prove it: the `codex-acp` adapter (goal a).** Fixes the silent-steering-drop trap. Verify: a live codex-acp ask + a sidecar-only eval spot check (2–3 cells incl. a `ghx-sidecar` cell) confirming no anomaly-contract drift.

**Phase D — the replaceable runtime (goal b, P4, ADR-gated).**
- D1. The `claude-sdk` in-process adapter (pure-Go loop over `anthropic-sdk-go`). This *removes* leaks (permission wire, thinking opacity, char-proxy tokens) and delivers real production tokens (TRUST H5). Because it changes a measurement source, it is pre-registered in its own ADR and re-grounded with a citable sidecar-only comparison run — never a drive-by.

## Consequences

- The runner becomes a config flip: `runner.kind` selects claude-acp, codex-acp,
  or an in-process SDK loop, with no change to `Ask`, the eval stack, or the
  report contract. NORTH_STAR P4 ("below ACP") turns from a rewrite into an
  adapter.
- Real production token accounting (Phase D) closes the last major trust hole
  (H5) for free, as a side effect of owning the harness.
- The composition root and pure-leaf core make future tools (P3) and runtimes
  (P4) injected values extending existing patterns, not new globals or rewrites.
- The frozen measurement stack is never touched by Phases A–C; the only
  measurement change (Phase D real tokens) is isolated and pre-registered.

## What we deliberately do NOT do (skeptic's guardrails, adopted)

- No `internal/domain` / `internal/shared` / role-layer packages; no DI
  container or service locator; no framework extraction (AUD4 V3 posture).
- No abstracting a component that has exactly one implementation forever — the
  Runner port earns its interface because a *second* adapter (codex-acp) is a
  concrete near-term goal, not a hypothetical.
- No steering field that an adapter silently drops: `SteeringSpec` is a
  lowest-common-denominator contract; a field a runtime cannot honor degrades
  **loudly** (recorded warning), per Visibility & Truthfulness.
- No change to the eval-anomaly string↔class mapping outside a pre-registered
  eval ADR.

## Cross-references

- The five audits under `docs/audits/architecture-vision/` (all `file:line` evidence).
- ADR-0035 — tactical cleanup; item 5 superseded here, items 6/7/9 folded into Phases B/A.
- ADR-0034 — recon-operation failure class (the other failure axis; complementary, not merged).
- ADR-0027 / ADR-0030 — resilience + daemon; Phase A implements D6 and adds supervisor discipline.
- ADR-0015 / ADR-0018 — Go-native ACP + adapter opacity; the port demotes ACP to one adapter and the SDK adapter removes the opacity.
- NORTH_STAR P4 + "Protocols are stepping stones, not identity"; the moat is the brain, and the port keeps the brain independent of any one runtime.

## Provenance

Synthesized by Fable from five independent read-only architecture-vision audits
(2026-07-07), holding their disagreements explicitly. The runner-port design is
adopted from the ports-and-adapters audit; the composition-root and no-layers
stance from the boundaries audit; the domain-flow rule from the domain-model
audit; the typed-fault-owns-marker resilience reconciliation from the resilience
audit (independently corroborated by the runner-port audit); the guardrails from
the YAGNI skeptic. Awaiting Goga's acceptance and a decision on how far to
execute; Phase A is low-risk and independently valuable regardless.
