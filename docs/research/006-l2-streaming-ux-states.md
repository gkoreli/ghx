---
title: "Research 006 — L2 Streaming UX: target output states for a 60-second ask"
date: "2026-08-21"
status: "design"
thread: "sidecar-adoption"
author: "Hermes engineer session (Goga Koreli)"
scope: "UX acceptance criteria for ADR-0040 L2 (task t_408a9a77); design artifact, not implementation"
builds-on: "ADR-0040 L1/L2, ADR-0022.1 live.jsonl, ADR-0019.3 D2 contract purity, FRICTION.md 2026-07-07 depth-latency study"
---

# L2 Streaming UX — what a 60-second ask should look like

## Problem

Today `ghx sidecar ask` is silent for 40–106s, then prints the report.
FRICTION.md (2026-07-05) documented the live stream as near-zero signal
("~48 identical `execute (pending)` lines"). Meanwhile `live.jsonl` already
captures rich turn activity (ADR-0022.1): text chunks, tool calls with
titles, status transitions — 100+ events per ask. The information exists;
it isn't rendered where the user is looking.

Target: **first useful signal < 5s**, continuous legibility thereafter,
final report byte-identical to today.

## Design principles

1. **One line at a time, newest-wins for status** — no scrolling wall.
2. **Tool calls render as verbs with objects**, not protocol events:
   `reading internal/sidecar/runtime.go` not `ToolCall kind=read`.
3. **The evidence contract stays pure**: streaming is *human stdout*
   decoration only; `--json`, MCP envelope, and captured artifacts are
   untouched (ADR-0019.3 D2 non-negotiable).
4. **Degrade gracefully**: non-TTY output falls back to today's silent mode
   plus one progress line per completed phase (scripting agents need `$?`
   semantics, not animation).

## Target states (TTY)

```
t=0s   ▶ question accepted · session <name> (<repo>)            [route line]
t≈2s   ◔ thinking…
t≈5s   ⎿ reading README.md                                      ← FIRST EVIDENCE ≤5s
t≈8s   ⎿ mapping internal/sidecar/ → 14 files
t≈15s  ⎿ searching "quota exhaustion" → 6 matches
t≈22s  ◔ synthesizing… (3 sources so far)
t≈40s  ⎿ cross-checking runtime.go:530 against ledger
t≈58s  ✓ report ready — answer + N verified claims, M citations
       [final report renders exactly as today]
```

Rules per state:

- **t=0 route line** reuses `RouteDecision.Line()` content (already inside
  the JSON payload per C8; human surface may echo it early).
- **Tool lines** (`⎿`) come from `tool_call`/`tool_update` events: verb +
  short object (path / query / count), resolved status replaces pending;
  max ~1 line per tool, collapse repeats of the same command.
- **Thinking/synthesizing** are spinner states derived from absence of
  other events >3s — never fabricated activity names.
- **Completion line** carries counts from the report itself (verified
  claims, citations) so the summary is truthful before the body renders.

## Acceptance criteria for t_408a9a77's implementation

1. First non-route event line renders ≤5s after ask start on a warm daemon
   (measured, not asserted — test fixture with scripted live.jsonl replay).
2. All five state types render from real event shapes in live.jsonl
   (replay-based tests; no synthetic event kinds invented).
3. `ask --json` output and MCP envelope byte-identical pre/post change
   (pinned by existing contract tests — they must not need edits).
4. Non-TTY: no ANSI, no spinners; phase-completion lines only; exit codes
   unchanged (ADR-0034.1 semantic codes).
5. Repeated identical pending lines collapsed (the FRICTION.md ~48-line
   pathology cannot recur).
6. `--quiet` flag suppresses streaming entirely (old behavior preserved).
7. FRICTION.md entry template added; the 2026-07-05 open item
   ("render tool-call titles… instead of bare pending marker") closes if
   criteria 1–5 hold in dogfood.

## Non-goals

- No interactive steering mid-stream (that's resume/steering territory,
  ADR-0020.2).
- No partial-report guessing: never show an "answer" before submit_report.
- No changes to report schema or envelope (C1–C18 untouched).
