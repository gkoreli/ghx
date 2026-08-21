---
title: "Research 005 — North-Star Working Rhythm: What the Orchestrator Does While the Swarm Drills"
date: "2026-08-21"
status: "synthesis"
thread: "sidecar-vision"
author: "Hermes engineer session (Goga Koreli)"
scope: "operating-rhythm artifact; maps standing work to NORTH_STAR milestones"
builds-on: "docs/NORTH_STAR.md, ADR-0040, docs/research/004-why-the-sidecar-framework-must-exist.md, docs/dogfood/FRICTION.md"
---

# North-Star Working Rhythm

Question: what does the orchestrator do while kanban workers drill? Answer:
every idle-hour task maps to a NORTH_STAR milestone. This doc is the standing
map so "work towards the north star" is never ambiguous.

## The milestone ladder we serve (NORTH_STAR, 2026-08-21 state)

| Stream | Frontier | What closes it |
|---|---|---|
| B2/M5 | Zero-CLI adoption + founder daily dogfooding | Two weeks of real use in FRICTION.md; breaking items fixed |
| A2/A4 | Usage mining → ergonomics batch; error affordances | Mined batch pre-registered, then shipped |
| P2a (new, ADR-0040) | Evidence-contract spec + conformance | docs/spec/ + suite any harness can run |
| L1–L4 (ADR-0040) | Latency war | p50 ≤15s warm; streaming <5s; quota ladder; daily-driver habit |

## Standing rhythm while workers drill

1. **Review landed work against pre-written criteria** (never post-hoc vibes).
   ADR-0041 for the spec; UX-state doc for L2 streaming.
2. **Restore the dogfood loop (L4 = M5's real exit bar).** Done 2026-08-21:
   ghx-recon skill re-exported into ~/.claude/skills/ (was stale since
   2026-07-03), recon MCP server registered globally in ~/.claude.json via
   the ADR-0019.3 D4 one-liner. Every Claude Code session on this machine
   now starts with the sidecar one tool-call away. Next: every repo question
   Goga or the orchestrator has routes through ghx; friction → FRICTION.md.
3. **Distribution grind (A-workstream).** Skills-ecosystem and plugin-catalog
   submissions — the install rails exist; they need listings.
4. **Ops friction logged like product friction.** Kanban spawn crash
   (bogus skill tag) + fix goes to FRICTION.md — the dogfood bar applies
   to our own tooling.
5. **Merge discipline.** Worker branches rebase-merge to mainline only after
   contract review (artifact or tested code, cross-refs present).

## The flywheel this creates

Every question anyone asks about any repo → ghx (sidecar or CLI) → session
artifacts under ~/.ghx → friction and usage mined (A2) → ergonomics batch →
better product → more use. The orchestrator's job is keeping every segment
of that loop turning at once.
