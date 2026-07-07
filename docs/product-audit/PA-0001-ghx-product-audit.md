---
title: "PA-0001 — ghx product audit (synthesis)"
date: "2026-07-07"
status: "audit"
audit: "PA-0001"
persona: "synthesis (orchestrator / Fable-Opus)"
scope: "5 personas × {north-star alignment, competitive/moat, AX, token-economics/SPT, evals-as-user-research/trust, distribution} · surfaces: NORTH_STAR+ADRs, README+skills, CLI+report contract, docs/evals+~/.ghx, dogfooded ghx · read-only, no code changes"
author: "product-audit skill (.claude/skills/product-audit) — Fable synthesis over PA-0001.1–.5"
---

# PA-0001 — ghx Product Audit (synthesis)

First live run of the `product-audit` skill on ghx. Five read-only adversarial persona-agents
(opus) each produced a threaded artifact; this is the orchestrator's judge-reconciled synthesis.

- **Thread:** `PA-0001.1` Agent Experience · `PA-0001.2` AI PM Pragmatist · `PA-0001.3`
  Platform/Scale Realist · `PA-0001.4` Competitive-Paranoid (generative) · `PA-0001.5`
  North-Star Zealot. Each finding's full evidence lives in its persona artifact; this doc
  ranks, de-duplicates, and records the judge step.

## Judge-step provenance (who judged this, and how to check)

Per the skill's directive 5 (a Claude orchestrator adjudicating Claude workers is a
self-preference regime — arXiv:2404.13076), findings were **not** accepted on assertion:

- **Orchestrator self re-derivation (done):** the load-bearing facts were recomputed by Fable
  from raw artifacts — `.claude-plugin/plugin.json` (H1); `docs/evals/corpus-discrimination-2026-07-07.md`
  table (H2); `grep -rn schemaVersion internal/` + `internal/sidecar/report.go` (M3);
  `internal/cli/serve.go:208-220` `handleRecon` (M1); `wc -l internal/sidecar/prompt.go` = 350,
  which **contradicts** a persona's "141 lines" figure (L1 nuance). Competitor repos verified
  live via `gh` (DeusData/codebase-memory-mcp 27.8k★, Aider 47k★, Repomix).
- **Shared-prior caution applied:** H2's convergence across the AI-PM and North-Star personas is
  **not** independent corroboration — both read the same `corpus-discrimination` doc; it was
  re-derived from the artifact directly, not from the agents' agreement.
- **Independent cross-family adjudication — PENDING (not run this session).** The Codex/gpt-5.5
  adjudicator was dispatched read-only but returned a usage-limit error ("try again at 12:03");
  no alternative non-Claude adjudicator is available. **Therefore every High finding below is
  self-verified but NOT yet cross-family-checked, and none may feed a milestone go/no-go until
  that check runs** (retry Codex, or Goga adjudicates). This gap is the honest state, recorded
  rather than hidden.

## Executive summary

**The single highest-leverage finding: the product's *default adoption surface inverts its own
north star* (H1).** `.claude-plugin/plugin.json` installs `skills/ghx` + `skills/ghx-mcp` — the
full ~3,900-token CLI skill and the multi-tool MCP — as the default, while the lightweight
`recon` skill / single-tool surface is opt-in behind flags. A product whose terminal thesis is
"*the main agent needs zero knowledge of the ghx CLI — it never loads the ghx skill file at
all*" ships, by default, the exact skill file it wants gone. This is the M5/B2 frontier and it
is a concrete, file-cited contradiction of the core thesis — not a vibe.

The audit's **connective theme** is more reassuring than any single finding: **ghx's honest
internal ledgers (`TRUST.md`, `corpus-discrimination`, the committed honest-negative, the
contamination guard) are consistently *more truthful than the flagship NORTH_STAR narrative*.**
The measurement discipline is genuinely excellent (see *What is actually fine*); the gap is that
the banner claims (quality advantage, "24×/25×", "THESIS SUPPORTED") run ahead of what those
ledgers actually support. **The fix for most High findings is narrative honesty + finishing
already-planned measurement (run C8, calibrate the judge), not new machinery.** And the recon
brain itself, dogfooded live during the audit, *worked* — proving the thesis in the act.

## Ranked findings

### H1 — The default adoption surface inverts the north star  ·  Severity: High (L3×I2)
- **Persona·Scope·Surface:** Agent Experience · Context-budget/progressive-disclosure · distribution/plugin.
- **Evidence:** `.claude-plugin/plugin.json` → `"skills": ["./skills/ghx","./skills/ghx-mcp"]` (re-derived by Fable). The recon skill / single `recon` tool is opt-in on all three surfaces (`PA-0001.1` F1). `inspect` and `tier2` — the best signal-per-token collapses — are absent from `ghx skill` and MCP entirely (`PA-0001.1` F3).
- **North-star relationship:** fails the core thesis ("main agent needs zero ghx knowledge; context is sacred") at the *default* install.
- **Disposition:** Needs work → **workstream B2** (zero-CLI adoption surface, = M5 frontier): make the recon/single-tool surface the default; demote the CLI skill to opt-in; expose `inspect` on the recommended surface.
- **Who judged / how to check:** self-verified (`cat .claude-plugin/plugin.json`). **Cross-family: pending.**

### H2 — The flagship narrative overstates what the honest ledgers already say  ·  Severity: High (L2×I3)
- **Persona·Scope·Surface:** AI PM Pragmatist + North-Star Zealot · evals-as-user-research / north-star metric-trust · `docs/evals/*` + `NORTH_STAR.md`.
- **Evidence (re-derived from `docs/evals/corpus-discrimination-2026-07-07.md`):** only **2 of 6** tasks discriminate the profiles; the other four are ceiling-capped (`gin`/`hono`/`flask` all 1.000) or contamination-confounded (`ghx-mapengine`). On **both** discriminating tasks the sidecar is **below ghx** — `express` 0.900<1.000, `openai-node-streaming` 0.800<1.000 (and <plain 0.950, the corpus's "best sidecar-regression sentinel"). The **quality** half of "both, not one" is unmeasured: C8 host-task eval has 0 episodes run; the judge is uncalibrated (κ not yet run). The NSM **"signals-per-token" is a compression meter, not a quality meter** — in the confirmatory run the sidecar has the *lowest* mean signal yet the *highest* SPT; the advantage is the denominator (`PA-0001.5` F1). One number is quoted four ways (16×/24×/25×/35×); the citable run is 16.6× (`PA-0001.5` F3).
- **Fairness (the important part):** this is **narrative/framing lag, NOT fabrication.** Every caveat above is stated in ghx's *own* committed ledgers; `corpus-discrimination` is a self-authored honest analysis that *proposes fixes* (R1–R4). Credit is due (see nulls).
- **North-star relationship:** bears on the "both, not one" rebuttal, the moat-via-measurement thesis, and the Visibility/Truthfulness tenet ("read the M4 verdict as a conservative floor, never a ceiling").
- **Disposition:** Needs work → **workstream C** (run C8 host-task eval; land the κ-calibrated judge — C4) **+ a NORTH_STAR banner honesty pass** making the flagship say what the ledgers say; adopt the corpus refresh (R1–R4) via a pre-registered eval-corpus ADR.
- **Who judged / how to check:** self-verified against the committed doc (recompute commands are in `corpus-discrimination-2026-07-07.md` §Provenance). **Cross-family: pending.**

### H3 — "Brain not tools" is a current advantage, not yet a Power — and the true alternative ships in the host  ·  Severity: High (L2×I3)
- **Persona·Scope·Surface:** Competitive-Paranoid + Platform/Scale · competitive / moat / distribution · `NORTH_STAR.md` moat + dogfooded GitHub recon.
- **Evidence:** the moat's two named barriers are unrealized — cornered-resource trajectory data → trained model (M10, *blocked*) and the process-power judge (κ *not calibrated*); the brain is `NORTH_STAR.md`'s own admitted "~400 lines of quirky instructions" (no barrier yet). The **true alternative is the host's native `Explore` subagent** (read-only, cheaper model, context-isolated) — ghx's exact Benefit, free from the platform — not "the agent greps files itself" (`PA-0001.4` F1/F2). A benchmark-backed competitor, **DeusData/codebase-memory-mcp (27.8k★, arXiv, "99% fewer tokens", auto-installs into 11 agents incl. Claude Code)**, is claiming the aggregation/distribution Power; **no eval measures whether an agent *selects* ghx** (ADR-0032.1 arm-B is an ablation, not a selection contest) (`PA-0001.3` F3).
- **North-star relationship:** the moat thesis and P4. Per the generative-competition doctrine (`AGENTS.md`), the move is **accelerate + absorb**, not retreat.
- **Disposition:** Needs work → accelerate the data moat + make the **evidence contract a switching cost** (P4); add a **selection-eval arm** (workstream C); **publish the SAFE benchmark** before the category's numbers are set by someone else; absorb (below).
- **Who judged / how to check:** Explore behavior + competitor stars verified live (`gh`); **F2's assumption that Explore stays local/in-session with no persisted artifact is verified-now-not-future** — the single most important thing to monitor. **Cross-family: pending.**

### M1 — The *recommended* surface (MCP `recon`) has the worse contract  ·  Severity: Med-High (L3×I2)
- **Platform/Scale + AX · AX/tool-affordance · `internal/cli/serve.go`.** `handleRecon` (`serve.go:208-220`, re-derived) returns `json.Marshal(report)` **plus appended trailing prose** (`Route.Line()` + `Artifacts.FooterLine()`) via `NewToolResultText` — an agent parsing the output as JSON hits trailing non-JSON. The CLI `--json` path returns a clean `askEnvelope`. The north star *recommends* the MCP surface for delegation. → **workstream B** (return structured route/artifacts fields, not appended prose). Self-verified. Cross-family: pending.

### M2 — Silent-success affordances  ·  Severity: Med-High (L2×I2)
- **AX · error-as-affordance · CLI.** `ghx read` of a nonexistent repo returns **exit 0** "(not found)"; a `$?`-checking agent reads success and proceeds on empty content (`PA-0001.1` F2, confirmed live). Silent-success is the worst affordance class. → **workstream A4** (every error/empty names the correct next move; distinguish not-found from success). Cross-family: pending.

### M3 — The customer report contract is unversioned  ·  Severity: Med-High (L2×I2)
- **Platform/Scale · contract-stability · `internal/sidecar/report.go`.** `Report` — declared an "API surface," consumed over CLI/MCP/sidecar — carries **no `schemaVersion`**, while `internal/sidecar/evals/` bundles *are* versioned (`judge-config-v1`; `grep` re-derived). Evolution is by ambiguous `omitempty`. → **new ADR** (version the customer-facing report before the "many brains, many domains" substrate ossifies). Self-verified. Cross-family: pending.

### M4 — Report-contract composition gaps  ·  Severity: Med (L2×I2)
- **AX · Orchestration/compose · report schema.** Empty-`verified` / unverified answers are not elevated to the `answer` surface (F4); cited paths carry no repo qualifier, so they 404 on scope-drift (F5) (`PA-0001.1`). → **workstream B**.

### L1 — Number drift in the strategy docs  ·  Severity: Low-Med (L3×I1)
- **North-Star + Platform · strategy-doc-as-product · `NORTH_STAR.md`/ADRs.** SPT quoted 16×/24×/25×/35× (`PA-0001.5` F3); the "~400-line persona doctrine" that anchors the **P4 exit trigger** is imprecise — `internal/sidecar/prompt.go` is **350 lines total** (the persona's "141" is an unverified substring; Fable did **not** adopt it). → reconcile the figures; cite the **measured cache-stable token cost** for the exit trigger. Self-verified (`wc -l`).

### L2 — A failed milestone risks reading as working  ·  Severity: Low (L2×I1)
- **AI-PM + North-Star + Platform · capability-claims · `NORTH_STAR.md`/M8.** M8 anticipation's D1 gate FAILED (nextReads recall 0.071/0.000) but is described in prose alongside working capabilities; `nextReads` is promised-but-unenforced (`PA-0001.3` F6 → ADR-0031.2). The failed gate already halted the build correctly — the fix is narrative, ensuring nothing cites M8 as evidence it works.

## What is actually fine (checked with equal rigor — and load-bearing here)

- **The eval discipline is genuinely excellent and *more honest than the banner*:** the judge is withheld until κ-calibrated and self-stamps "NEVER CITABLE"; the honest-negative run is committed and linked from the README; memorization is measured (0.067 closed-book); the contamination guard fires (7×) and self-labels; correctness recomputed 90/90 cross-family; `corpus-discrimination` is a self-authored honest analysis proposing fixes. ghx passes the AI-PM's core test — "don't cite evals you can't audit" — **emphatically** (`PA-0001.2`, `PA-0001.5` nulls).
- **The recon brain works, proven in the act:** live `ghx sidecar ask` — 38s / 66s, exit 0, every verified claim citing its exact command, ground-truth `actualCommandLedger`+`traceCommands` on disk, **HIC 0, false_errors 0**; the discovery tier dramatically outperformed manual `ghx repos` calls — "the moat thesis proven in the act of competitive recon" (`PA-0001.1`, `PA-0001.4` nulls).
- **Producer-side rigor:** `submit_report` validation is strict (no coercion); the provenance data-model is clean (the defect is serialization only); the depth dial is validated not coerced; routing is byte-for-byte stable across daemon/daemonless; OTel rides the frozen spec (`PA-0001.3` nulls). One open FRICTION item is already fixed (`PA-0001.1`).

## Adjacent-idea proposals (labelled; Ansoff-named; north-star-filtered)

- **Selection eval** — measure whether an agent chooses ghx over native `Explore` (a contest, not an ablation). *Market Penetration / measurement.* Passes filter (auditable evidence). (H3, `PA-0001.3` F3.)
- **Publish the SAFE benchmark** for code reconnaissance before a competitor sets the category's numbers. *Market Development.* Filter: marketing proof + trajectories. (H3, a NORTH_STAR "Consequence Product".)
- **One-command host auto-install for `ghx serve`** (match codebase-memory-mcp's distribution). *Penetration.* (H1/H3.)
- **Maturity signal (stars/last-push) in `ghx explore`/discovery output** — turns a dogfood friction into a feature. *Penetration.* (`PA-0001.4` F6.)

## Absorption candidates (steal openly, with attribution — `AGENTS.md` Open Source Leverage)

- **DeusData/codebase-memory-mcp (27.8k★, MIT):** its tree-sitter + Hybrid-LSP **persistent knowledge graph** → a new `ghx tier2 graph` backend (remote-first stays ghx's line; absorb the graph as a local escalation tier); plus its **one-command host auto-install** mechanic for `ghx serve`. *utility→feature; north-star-aligned (deeper structural recon under the sidecar).* https://github.com/DeusData/codebase-memory-mcp
- **Aider (47k★):** its PageRank + `map_tokens` **repomap** → verify/upgrade the already-present `ghx tier2 repomap` against Aider's ranking quality. https://github.com/Aider-AI/aider
- **Repomix (26.9k★):** its tree-sitter `--compress` (~70% token cut) → a signature-level **compression pass** on `ghx read`/explore output — directly serves signals-per-token. https://github.com/yamadashy/repomix

## What was not audited (honest close)

- **Cross-family adjudication did not run** (Codex usage-capped) — High findings are self-verified only; **re-run before any milestone decision.**
- **Personas omitted** (5-cap): Feature-Factory Skeptic, User-Empath, Technical-Debt Realist, Experimentation Rigorist, Monetization Hawk, Simplicity Minimalist, GTM/Positioning, Growth, Craft Purist, Zero-to-One, DevEx (human-operator).
- **Scopes lightly touched / not run:** PMF-as-market, positioning/messaging depth, gap-analysis via live **desire-path** trace mining, **agent-safety** (enumerated, not exercised — no injection/lethal-trifecta probe), business-model/monetization (pre-monetization).
- **Surfaces:** no multi-task Phase-6 **weak-driver AX** sweep (only two one-shot live `ask`s, by AX and Competitive); no eval-mechanics review (out of scope by design → `docs/evals/TRUST.md`).
- **Second round / self-red-team** of these findings not run.
