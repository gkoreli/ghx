---
title: "Research — L4 dogfood week 1: founder daily-driver measurement (2026-08-22 start)"
date: "2026-08-22"
status: "research"
thread: "dogfood-l4"
author: "Hermes engineer session (Goga Koreli)"
scope: "L4a instrumentation slice + week-1 measurement log; weekly rollup one-liners appended in place as the week runs; week-end verdict against the ADR-0040 falsifier"
builds-on: "ADR-0040 L4 (docs/adr/0040-investment-rebalance-product-framework-forward.md), NORTH_STAR M5 exit bar, docs/dogfood/FRICTION.md, scripts/dogfood-week.mjs, scripts/mcp-recon-probe.mjs, ~/.ghx/sessions/"
---

# L4 Dogfood Week 1 — Founder Daily-Driver Measurement

**Capability candidate:** make daily ghx use measurable so the ADR-0040 L4
exit bar (two consecutive weeks of every repo question flowing through ghx,
friction logged) is judged on numbers, not vibes.
**Workstream:** NORTH_STAR M5 exit bar / ADR-0040 L4
(`docs/adr/0040-investment-rebalance-product-framework-forward.md`).
**Status:** research artifact / live measurement log. This file is the
week-end deliverable named by the L4a card; it starts as instrumentation +
baseline and accumulates daily rollups until the week closes.

## 1. Instrumentation landed (2026-08-22)

| Piece | What it measures | Evidence |
|---|---|---|
| `scripts/dogfood-week.mjs` | weekly one-liner: sessions, asks, answered/blocked/degraded reports, quota firings, p50 latency, degraded-rate — from `~/.ghx/sessions/*` artifacts only | run output §3; measurement rules pinned in script header |
| `scripts/mcp-recon-probe.mjs` | served-MCP path health over the exact founder `~/.claude.json` command line (`npx -y @gkoreli/ghx serve`): initialize → tools/list → one real recon ask | probe transcript, FRICTION.md 2026-08-22 confirmation entry |
| FRICTION.md L4 section | per-session friction entries under the existing breaking/soft taxonomy | docs/dogfood/FRICTION.md |

Measurement rules are deliberately fixed for the whole two-week bar (pinned in
the script header): session = dir with in-window `meta.json.createdAt`; ask =
one `turn.started`; latency = per-turn `sum/count` of the OTel
`gen_ai.client.operation.duration` histogram; p50 = lower median across turns;
degraded = report answer with the `DEGRADED` prefix (ADR-0040 L3 labels);
quota firing = degraded report or a quota-mentioning log line per session.
Changing these rules mid-bar would be moving the goalposts — any change needs
a dated note here and a re-baseline of prior days.

## 2. What the baseline already says (pre-week, honest)

The four sessions predating week 1 are agent-run dogfood probes, not founder
daily use: 9 asks, 4 answered, 2 BLOCKED (deliberate bad-input probes),
4 failed turns (the pre-fix `badslugnoslash` error-class batch), zero degraded,
p50 51.3s. Two structural facts stand out even before the week starts:

1. **Every real ask to date was driven by an engineer-agent loop, not Goga's
   own hand.** L4's bar is about the founder's daily loop; if day 5 looks like
   today, the falsifier is live regardless of what the numbers say.
2. **p50 ≈ 51s is far off the L1 target of ≤15s cheap-depth**, though week-1
   asks mix depths and the MCP-path cheap ask answered in ~34s wall clock —
   the rollup's latency number needs per-depth slicing before it can judge L1.

## 3. Daily rollups (append one line per day, verbatim script output)

```
dogfood 2026-08-21..2026-08-22: sessions=4 asks=10 answered=4 blocked=2 degraded=0 quotaFirings=0 p50=51.3s degradedRate=0%
```

(2026-08-22, instrumentation day: window includes the pre-week baseline
sessions because they fall inside the trailing 7-day default; the four new
asks beyond baseline are this task's verification traffic. From tomorrow the
line should reflect actual founder-loop usage.)

## 4. Friction entries this week

See docs/dogfood/FRICTION.md section "L4 dogfood week 1". So far:

- confirmation: recon MCP wiring verified end-to-end over the founder config
  (ghx 2.10.1 published == wired; single `recon` tool; cited answer ~34s).
- soft/open: `sidecar doctor` does not check the served-MCP surface — the one
  path L4 depends on has no doctor coverage (probe script is the reference fix).

## 5. Week-end verdict slot (fill at week close)

To be filled 2026-08-28/29 with: final rollup line, friction count by
severity, dispositions closed vs deferred, and the honest answer to the
ADR-0040 falsifier question: did usage stick after L2+L3?

## 6. Open questions

1. Should the daily driver go through the MCP recon tool inside Claude Code
   (agent-mediated) or the CLI? ADR-0019.3 says recon-first MCP; week 1 should
   count whichever surface actually gets used and say which.
2. Per-depth latency slicing in the rollup — needed before p50 can judge the
   L1 ≤15s target; candidate follow-up for dogfood-week.mjs (additive flag).
3. Doctor `mcp-serve` check (FRICTION.md open item) — route to the next
   ergonomics/fixbatch card rather than growing L4a scope.

## 7. Evidence appendix

Commands run while landing instrumentation (all read-only wrt the repo):

- `node scripts/dogfood-week.mjs --since 2026-08-15` → baseline one-liner above
  (after fixing a UTC month-index bug found on first run; verified --json,
  empty-window, and exit-2-on-bad-args paths).
- `node scripts/mcp-recon-probe.mjs` → initialize ok `ghx 2.10.1`;
  `tools/list` = `["recon"]`; recon ask (tidwall/gjson, depth=cheap) →
  schema-valid report citing gjson.go:982-1044 in ~34s; warm-daemon turn
  recorded (`turnCount` 1→2, live.jsonl `turn.completed ok:true`
  2026-08-22T00:10:18Z).
- `npm view @gkoreli/ghx version` → 2.10.1 (published matches wired config).
