# arch-audit artifact template & threading

## Threading convention

One run = one thread `AA-000N`, scope in the slug, artifacts under
`docs/audits/AA-000N-<scope-slug>/` (matches the `product-audit` `PA-000N` scheme):

```
docs/audits/AA-000N-<scope-slug>/
  AA-000N-<scope-slug>.md      # thread charter (scope, in/out, question, tenets served)
  AA-000N.1-<persona>.md       # persona artifacts, one per worker
  AA-000N.2-<persona>.md
  AA-000N.6-adjudication.md     # optional cross-family adjudication of contested findings
  AA-000N.9-distilled.md        # convergence & distillation (the orchestrator's judged synthesis)
```

The 2026-07-07 run predates this scheme (flat `docs/audits/*-2026-07-07.md` +
`architecture-vision/`); treat it as `AA-0001` in spirit. Start new runs at the next number.

## Persona artifact template

```markdown
---
title: "AA-000N.<k> <Scope> — <persona/lens>"
date: "YYYY-MM-DD"
status: "audit"
thread: "AA-000N-<scope-slug>"
scope: "<module / capability / mindset the run covers>"
persona: "<the lens, e.g. Go architecture & boundaries / YAGNI skeptic>"
author: "background audit worker (read-only)"
---

# <Title>

<1–2 lines: the lens and what it is looking for. Note it does not repeat findings
already actioned by prior audits/ADRs — cite and move on.>

## Executive summary
<The single highest-leverage finding first, and the one-line verdict.>

## What is already right (verified, not assumed)
<MANDATORY. What the code gets right, each cited — stops manufactured debt.>

## High severity
### H1 — <title>
**What.** … **Evidence.** `path:line` … **Why it's debt.** ties to a named
AGENTS.md Engineering Tenet / NORTH_STAR phase. **Recommendation.** concrete, scoped.

## Medium severity
### M1 — … (same shape)

## Low severity
### L1 — …

## Recommended sequence (phased, non-rewrite)
1. <highest impact×tractability first; each step shippable + behavior-preserving;
   note any measurement-stack items as DEFERRED/ADR-gated>

## Method / auditability
<the exact commands run so every number/size/complexity recomputes; the commit the
file:line references resolve at; `go build ./...` status at audit time>
```

## Distillation artifact template

```markdown
---
title: "AA-000N.9 <Scope> — Convergence & Distillation"
date: "YYYY-MM-DD"
status: "distillation"
thread: "AA-000N-<scope-slug>"
scope: "<scope>"
author: "distiller (fresh, no-stake) + Fable (final judge)"
---

# <Title>

## Convergence table (the trust signal)
| Hotspot | Converging personas | Independent-recompute check |
|---|---|---|
| … | AA-000N.1 H1 + AA-000N.4 H2 | <recomputed, not head-count> |

<Note which convergence is shared-prior (agents read the same code) vs genuinely
independent. Discount shared-prior; the signal is the recompute.>

## Ranked sequence (impact × tractability × cross-persona convergence)
### Tier 1 — foundational, low-risk
### Tier 2 — high value, larger
### Tier 3 — consistency/quality
### Deferred — measurement-stack / ADR-gated (never drive-by)
### What we deliberately do NOT do (the skeptic's guardrails, adopted)

## Tensions held (not averaged)
<where personas disagreed and how the judge resolved it, with evidence.>

## Governing ADR
<the ADR this distillation feeds (write/update it), and the execution posture:
each item a behavior-preserving worker task, ADR-gated, landed ff-only.>

## Provenance
<the persona artifacts, the recomputes re-run, cross-family checks, what was declined and why.>
```
