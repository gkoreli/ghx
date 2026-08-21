---
title: "Research 007 — A2 usage mining batch: friction ranked from eval + production traces"
date: "2026-08-21"
status: "research"
thread: "cli-ergonomics"
author: "Hermes engineer session (Goga Koreli)"
scope: "read-only research artifact; proposes, does not implement; pre-registration input for the next ergonomics batch (ADR-0028.1 pattern)"
builds-on: "NORTH_STAR workstream A2/A4 (docs/NORTH_STAR.md:302,304), ADR-0028.1, ADR-0040 §What-actually-stops-heavy-use, docs/dogfood/FRICTION.md, internal/sidecar/evals/.ghx-evals/runs/, ~/.ghx/sessions/, ~/.ghx/runtime/daemon.log"
---

# A2 Usage Mining Batch — Friction Ranked From Real Traces

Workstream anchor: NORTH_STAR A2 — "Agent-usage mining → CLI ergonomics batch:
mine eval/production traces for failed invocations, wasteful outputs, missing
flags; fix the top findings" (`docs/NORTH_STAR.md:302`, status ← frontier).
This artifact is the mining pass; the fixes it proposes are **not implemented
here** (task contract). It pre-registers the next ergonomics batch using the
ADR-0028.1 pattern: mine → rank → batch-decide (`docs/adr/0028.1-cli-ergonomics-batch.md`
was itself built on the 2026-07-06 miner's `CLI-FINDINGS.md`).

Corpus mined (all timestamps UTC):

| Source | Volume | Path |
|---|---|---|
| Eval episodes | 170 JSON files (80 completed + 90 rate-limit-aborted), 9 run dirs | `internal/sidecar/evals/.ghx-evals/runs/{20260703-*,gate-2026-07-03-*}` |
| Verdicts | 3 (one per gate run) | same dirs, `verdict.{md,json}` |
| Production sessions | 3 sessions, 12 asks total (4 + 2 + 2 recorded turns; badslugnoslash logged 4 attempts that never became turns) | `~/.ghx/sessions/{badslugnoslash,badslugnoslash2,modelcontextprotocol-typescript-sdk}` |
| OTel telemetry | traces/metrics/logs JSONL per session (42 span instances inventoried) | same dirs |
| Daemon log | 32 lines, 2026-08-21 window | `~/.ghx/runtime/daemon.log` |

Method: programmatic sweep over every episode/report/span (Python over the raw
JSON; commands in Appendix A). Every numeric claim below was recomputed from
the files, not copied from verdicts. Profile-aggregate numbers exclude the
aborted run unless stated.

---

## 1. Findings from the eval corpus

### F1 — Aborted episodes are stored as ordinary zero-reward episodes and poison aggregates (severity: measurement integrity)

The `gate-2026-07-03-aborted-rate-limit` run contains 90 episodes where the
backend died of the Anthropic session limit mid-run. Each is a well-formed
episode JSON with `rewards.overall = 0.167`, `correctness = 0`, empty text,
zero tool calls, duration 0.5–2.4s:

- `runs/gate-2026-07-03-aborted-rate-limit/gin-routing_ghx_1783096744968.json`
  (and 89 siblings): `turns[0].text == ""`, `toolCalls: []`,
  `context.mainAgentChars` = 52 (plain/ghx) or 0 (sidecar).
- Nothing in the episode marks it aborted — no status field, no error field.
  The only distinction is the run directory's name (`…-aborted-rate-limit`).
- Consequence measured: including this run drops `ghx-sidecar` profile mean
  overall from 0.897 → 0.518 and its correctness-hit rate from 73% → 45%
  (56 episodes incl. 30 aborted vs 26 live). Any naive aggregate query over
  `runs/*/*.json` — which is exactly what a usage miner does — produces garbage.
- The aborted run's own `verdict.md` correctly prints PRELIMINARY with FAIL
  gates, so the human-facing verdict path handles it; the *data* does not.
- Prior art in-repo: live canaries that cannot run are marked PRELIMINARY by
  convention (skill: fixture-first eval additions), but episode JSON has no
  equivalent marker.

**Fix direction:** stamp aborted/error episodes with an explicit
`status: aborted` (+ `abortReason`) at write time; teach aggregators/miners to
filter. Owner module: `internal/sidecar/evals/episode.go` (schema +
writer), consumers in `internal/sidecar/evals/gates.go`.

### F2 — Resume works everywhere it applies, but costs ~53% more wall time (efficiency observation)

All 32 multi-turn follow-ups in the live corpus resumed (`resumed: true`),
including every sidecar follow-up — the ADR-0016.1 G4 memory gate is genuinely
green on today's corpus. Cost side: resumed episodes average 84.4s vs 55.1s
for non-resumed (n=32/48 live episodes), +11.6 vs +8.8 tool calls. With
gin-routing already the slowest task family (ghx-profile average 98.9s,
plain 105.8s), resume overhead compounds the ADR-0040 #1 latency complaint
(`docs/adr/0040-investment-rebalance-product-framework-forward.md:153`).
Not a defect — a tracked cost line for the latency war.

### F3 — `ghx-mapengine` is currently unanswerable through the sidecar profile (correctness 0/5 vs plain 3/5)

Per-task × profile correctness on the live r2 corpus (5 episodes each):

| task | ghx-sidecar | ghx | plain |
|---|---|---|---|
| express-router-location | 4/5 | 5/5 | 5/5 |
| flask-routing | 5/5 | 5/5 | 5/5 |
| **ghx-mapengine** | **0/5** | 4/5 | 3/5 |
| gin-routing | 5/5 | 5/5 | 5/5 |
| hono-middleware | 4/5 | 6/6 | 6/6 |

Every mapengine sidecar episode scores `correctness = 0.75` (the judge's
partial floor) with high evidence (0.67–1.00) — the agents find the right code
but miss the expected answer component. Sample answer
(`runs/gate-2026-07-03-r2/ghx-mapengine_ghx-sidecar_1783141570489.json`):
locates `mapengine.Map()` dispatch + `mapAuto()` extension selection — which
matches the actual code (`internal/mapengine/types.go:143-158`: `.go` →
GoASTMapper with regex fallback; ts/tsx/js/jsx/py/rs → tree-sitter; default
regex). Either the fixture's expected answer demands more than the code
contains (e.g. treesitter language-list detail), or the judge prompt
under-credits. Baseline `ghx`/`plain` episodes carry `report: null` — their
answers live only in free text, so cross-profile diffing requires prose
parsing. This is the exact "fixture-first + Task.Validate" surface flagged in
repo conventions; needs a fixture audit, not a scorer change (scorer changes
are frozen behind pre-registration — AGENTS.md measurement-stack rule).

**Fix direction:** audit `ghx-mapengine` fixture expectation vs
`types.go:143`; if expectation is stale, fix the fixture; if judge
under-credits, extend judge bundle tests. Owner module:
`internal/sidecar/evals/` (fixtures/judge), with `internal/mapengine` as
ground truth.

### F4 — Episode tool-call traces are lossy placeholders ("Terminal (pending)") (observability debt)

Every episode's `turns[].toolCalls` is a list of literal `"Terminal (pending)"`
strings — e.g.
`runs/gate-2026-07-03-r2/express-router-location_ghx-sidecar_1783104440737.json`
has 10 identical entries carrying zero information about which command ran,
its status, or output size. Root cause found in code: `Turn.ToolCalls` is
`[]string` (`internal/sidecar/evals/episode.go:128`) populated from live-log
titles, while the real audit data lives in the sibling `ToolTraces`
(`sidecar.ToolCallTrace`, `episode.go:129-140`) added by ADR-0032.2's
tooltrace unification. All stored runs predate that landing, and the legacy
field was never backfilled or deprecated. A miner cannot answer "which ghx
invocations failed/were wasteful?" from eval episodes today — the single most
important A2 question. Production sessions do carry the full data (18
`tool.execute` spans with `ghx.sidecar.tool.output_chars` +
`output_excerpt`), proving the schema works when wired.

**Fix direction:** stop emitting the placeholder field (or populate it from
ToolTraces); consider a one-shot migration/backfill for future runs. Owner
module: `internal/sidecar/evals/episode.go` + the runner that fills turns
(`internal/sidecar/evals/runtime.go`).

---

## 2. Findings from production sessions (~/.ghx/sessions/)

Three sessions, all 2026-08-21, driven through the daemon over ACP
(claude-agent-acp@0.55.0):

### F5 — Invalid-slug sessions are created silently and burn full agent turns on unfailable input (top production friction)

Two of three production sessions (`badslugnoslash`, `badslugnoslash2`) were
created with repo names that are not `owner/repo`. ghx's core rejects such
slugs instantly with a teaching error (`internal/ghx/repo.go:28`: "invalid
repo %q: expected owner/repo, e.g. ghx explore gkoreli/ghx"), yet:

- `badslugnoslash/meta.json`: `"repo": "badslugnoslash"`,
  `turnCount: 0` — but its `tier-decisions.jsonl` records **four** tier
  evaluations at turn 1 (07:46:51 → 07:47:04, ~13s of retries), and
  `agent-stderr.log` ends in a claude-agent-sdk stack trace (session died).
- `badslugnoslash2`: two full turns ran (22:19:46 → 22:20:28). Both reports
  are BLOCKED answers whose `commandsRun` show the agent trying `ghx version`,
  `ghx explore badslugnoslash2`, `ghx inspect …`, `ghx search "repo:…"`, then
  retrying with `2>&1 | head -40` — and repeating nearly the same command set
  again in turn 2 (ledger confirms duplicates across turns). ~90s of paid
  agent time spent rediscovering what `repo.go:28` already knows.

The daemon never validates the slug before spawning the agent. Validation
exists one call away (`tier2/cache.go:122` enforces the same rule) — the gap
is wiring, not logic.

**Fix direction:** validate `owner/repo` shape at session creation/route time;
reject or rename with the teaching error instead of spawning. Owner modules:
`internal/sidecar/route.go` (RouteDecision) + `internal/sidecar/session.go`
(InitSession callers at `route.go`/`reroute.go:158`, `runtime.go:388`);
daemon entry `internal/sidecar/daemon_worker.go`.

### F6 — Backend rate-limit failures are retried blind, surfaced only as OTel noise, and lose the user's question

`badslugnoslash` hit Anthropic's session limit ("You've hit your session
limit · resets 8:30am (UTC)", `errorKind: rate_limit`) four times at turn 1:
four ERROR `sidecar.turn` spans in `traces.jsonl` with status message
`acp prompt: {"code":-32603,"message":"Internal error: …","data":{"errorKind":"rate_limit"}}`.
The same error string appears verbatim twice in `~/.ghx/runtime/daemon.log`.
Findings:

- No backoff/respect for the reset time anywhere in the sidecar daemon path
  (`grep -i 'backoff' internal/sidecar/{daemon,daemon_worker,acprunner}.go` →
  zero hits). Retries are immediate and identical.
- `meta.turnCount` stays 0 and no report lands — the ask evaporates except
  for telemetry. A user (or calling agent) watching stdout gets failure with
  no machine-readable class and no ETA, exactly the affordance gap ADR-0034
  phase 3 targets for MCP.
- The aborted-run corpus (F1) shows the same limit kills 30-episode eval
  batches mid-flight — one root cause, two blast radii.

**Fix direction:** classify `rate_limit` errors (already tagged `errorKind`
by the adapter), stop retrying until reset, propagate a typed error + reset
time to the caller, and mark the episode/session aborted. Owner modules:
`internal/sidecar/acprunner.go` (prompt loop), `internal/sidecar/emit.go`
(error span path, `emit.go:185-190`), daemon worker for scheduling.

### F7 — Free-form scope strings are stored raw; long scopes truncate silently downstream

`modelcontextprotocol-typescript-sdk/meta.json` has
`"scope": "We are writing a versioned standalone specification plus a conformance test suit"` —
a multi-clause sentence cut off mid-word ("suit"). The scope is echoed into
every tier-decision record and the ledger. `internal/sidecar/ledger.go:15`
declares `Scope string` with no length discipline; `session.go:107-121`
persists whatever arrived. The truncation itself happens upstream of storage
(daemon route payload), but the sidecar neither clamps nor documents a max —
so session names/dirs derived from scope-adjacent fields stay sane while the
scope content is corrupt. Minor, but it poisons every downstream consumer
that treats `scope` as a key.

**Fix direction:** clamp/normalize scope at InitSession; record
`scopeTruncated` flag. Owner module: `internal/sidecar/session.go`.

### F8 — Tier escalation never fires in practice; signals fire but policy declines

All 8 recorded tier decisions across the three sessions are
`tier1→tier2, allowed:false`. Two sessions fired a real signal
(`answer.low_confidence_remote_only`) — the detector works — but policy v1
declines anyway (`internal/sidecar/tier2/policy.go:245`): "no escalation rule
fired … remote evidence sufficient by policy", even when a signal *did* fire.
Wording nit aside (the reason string contradicts `firedSignals`), the observed
behavior is: low-confidence remote-only answers get no second-tier help, by
policy. If tier-2 is meant to rescue low-confidence answers, v1 policy makes
the detector dead code in production; if intentional, the reason string lies.
Either way the trace format can't distinguish "signal fired but rule weights
too strict" from "no rules exist yet" — `firedRules` is null everywhere.

**Fix direction:** reconcile policy reason text with firedSignals; add a
policy-rule counter to decision traces; decide whether
low_confidence_remote_only should gate escalation. Owner module:
`internal/sidecar/tier2/policy.go`.

### F9 — Report-sink auto-approval warning spams every session (known, still unfixed, now measured)

`agent-stderr.log` in both newer sessions ends with the
`CLAUDE_SDK_CAN_USE_TOOL_SHADOWED` warning for
`mcp__ghx-report-sink__submit_report`, repeated per spawn; daemon.log carries
it too. Already logged open in `docs/dogfood/FRICTION.md` ("SDK warning noise
on every ask") with disposition open; the intended auto-approve design is
documented (`internal/sidecar/session_options.go:43` "allowedTools is
auto-approve, NOT an allowlist"; `reportsink.go:34`). This mining pass adds
frequency evidence: it fires on **every** agent process spawn — 5 occurrences
across the three sessions' `agent-stderr.log` (1+2+2) and 6 in
`~/.ghx/runtime/daemon.log`.

**Fix direction:** PreToolUse hook or warning suppression in the adapter
wiring. Owner module: `internal/sidecar/session_options.go` /
`agentdiag.go:452`.

### F10 — Turn-level friction counters exist but were never non-zero in production; ask-spans carry no attributes

The OTel instrumentation records `report_retried`, `report_coerced`,
`wrap_up_recovered`, `session_recreated` per turn
(`internal/sidecar/emit.go:359-361`) — healthy observability design — but
across all 8 production turns they are all `false`, and `sidecar.ask` spans
carry **zero** attributes (empty span next to a fully-attributed
`sidecar.turn`). Consequence: we currently cannot distinguish "recovery paths
never needed" from "recovery paths never exercised by real traffic". Also
`gen_ai.usage.reasoning.output_tokens` appears on only some turns (adapter
dependent). Not a bug — a coverage gap that limits future mining rounds.

**Fix direction:** attribute ask-spans minimally (route source, tier, report
path) or fold ask+turn into one span pair contract. Owner module:
`internal/sidecar/emit.go`.

---

## 3. Daemon log (~/.ghx/runtime/daemon.log)

32 lines covering 2026-08-21. Contents: 1 INFO connection-close, the
session-limit error twice (→ F6), the SDK shadowed-tool warning six times
(→ F9), and four `▶ execute: Terminal (pending)` progress markers — which
are the same placeholder title shown in eval traces (F4), confirming the
live progress stream renders opaque titles, already logged in FRICTION.md
("live progress stream is opaque", disposition open). No new independent
findings; daemon.log corroborates F6/F9/F4.

---

## 4. Ranked fix list (top 10) — pre-registration for the next ergonomics batch

Ranking rationale: production-frequency × blast-radius first, then
measurement-integrity items that gate all future A2 mining, then efficiency/
coverage debt. Each row names the owning module so the batch can slice work
per owner.

| # | Fix | Finding | Evidence anchor | Owner module |
|---|-----|---------|-----------------|--------------|
| 1 | Validate `owner/repo` slug at session/route time; fail fast with the teaching error | F5 | `~/.ghx/sessions/badslugnoslash2/reports/1-*.json` BLOCKED commandsRun; `internal/ghx/repo.go:28` | `internal/sidecar/route.go` + `session.go` (+ daemon entry) |
| 2 | Rate-limit class: no blind retries, typed error + reset ETA to caller, mark episode/session aborted | F6 | 4 ERROR spans in badslugnoslash traces.jsonl; daemon.log lines; aborted run dir | `internal/sidecar/acprunner.go`, `daemon_worker.go`, `emit.go` |
| 3 | Episode abort status in eval JSON + aggregator filter | F1 | 90 zero-reward episodes in `runs/gate-2026-07-03-aborted-rate-limit/`; sidecar mean 0.897 vs 0.518 with/without | `internal/sidecar/evals/episode.go`, `gates.go` |
| 4 | Replace/populate lossy `toolCalls []string` placeholders from ToolTraces | F4 | `"Terminal (pending)"` lists in every pre-ADR-0032.2 episode; schema at `episode.go:128-140` | `internal/sidecar/evals/episode.go` + runner |
| 5 | Suppress/rewire report-sink auto-approval warning (100% of spawns) | F9 | agent-stderr.log ×2 sessions, daemon.log ×3; FRICTION.md open item | `internal/sidecar/session_options.go`, `agentdiag.go` |
| 6 | Audit `ghx-mapengine` fixture expectation vs `types.go:143` (0/5 sidecar correctness) | F3 | r2 mapengine episodes, corr=0.75 floor ×5 | `internal/sidecar/evals/` fixtures/judge |
| 7 | Tier policy: reconcile decline reason with firedSignals; decide low-confidence escalation; log rule ids | F8 | 8/8 decisions allowed:false, 5 with firedSignals non-null; `policy.go:245` | `internal/sidecar/tier2/policy.go` |
| 8 | Clamp/normalize session scope strings; flag truncation | F7 | mcp-typescript-sdk meta.json scope cut mid-word | `internal/sidecar/session.go` |
| 9 | Latency line-item: resume overhead +53% wall time; track per-task latency (gin-routing worst) | F2 | resumed 84.4s avg vs 55.1s non-resumed (80 live episodes); ADR-0040 latency list | measurement only — feed ADR-0040 latency war, owner `internal/sidecar/evals/` reporting |
| 10 | Attribute ask-spans (route source, tier, counters) for future minability | F10 | empty `sidecar.ask` spans ×8; counters all-false | `internal/sidecar/emit.go` |

Items 1–5 are the recommended batch core (user-visible failures +
mining unblockers). Items 6–8 are small scoped fixes. Items 9–10 are
measurement investments that make the *next* A2 round strictly stronger.
Deliberately excluded: scorer/gate changes (frozen behind pre-registration;
nothing here re-scores historical runs), and anything touching
`~/.ghx/` product storage.

## 5. Cross-references and proposed decision path

- NORTH_STAR A2 (`docs/NORTH_STAR.md:302`) — this artifact is the mining
  pass its frontier line calls for; supersedes nothing (the 2026-07-06
  miner's `docs/evals/mining-2026-07-06/CLI-FINDINGS.md` fed ADR-0028.1 and
  remains historical).
- ADR-0040 §"What actually stops heavy use"
  (`docs/adr/0040-investment-rebalance-product-framework-forward.md:149-153`)
  — F2/F6 quantify two of its listed stoppers (latency; quota fragility)
  with fresh trace data.
- ADR-0028.1 pattern — mine → rank → batch-decide; this document is stage 1.
  Proposed follow-up: a threaded **ADR-0042** "Ergonomics Batch 2 —
  fail-fast sessions, rate-limit honesty, trace fidelity" (parent-threaded
  under ADR-0028's ergonomics family per repo threading rules), carrying D1..D10
  mapped to rows above. Numbering check done against `docs/adr/` frontier
  (0041 is highest; no reserved decimal found in 0028.x).
- ADR-0030/0030.1 (daemon, session routing) own the surfaces in F5/F7;
  ADR-0016.1 owns gate semantics touched by F1/F3; ADR-0032.2 owns the
  tooltrace schema relevant to F4; ADR-0034 phases 3–4 own MCP error payloads
  adjacent to F6.
- Dogfood ledger overlap: FRICTION.md already-open items corroborated with
  frequency data here (F9 SDK warning; opaque progress stream → F4's title
  placeholders). No contradictions found with dispositions marked fixed
  (`code` exit-2, `--depth` validation).

## 6. Open questions

1. Is the `ghx-mapengine` expected answer stale relative to
   `internal/mapengine/types.go:143`? Needs fixture author confirmation —
   deliberately not guessed here (measurement freeze).
2. Should tier policy v1 ever escalate on `answer.low_confidence_remote_only`,
   or is that signal telemetry-only by design? Product decision, Goga's call
   (affects #7 scope).
3. Do we want a one-shot backfill of old run dirs' episode JSON (abort
   stamps, toolTraces), or forward-only fixes? Old dirs are referenced by
   committed verdicts; backfill risks churn on historical artifacts.

---

## Appendix A — evidence commands

Reproduce the corpus numbers:

```
# profile aggregates excluding aborted run
python3 - <<'EOF'  # (full sweep script preserved in session transcript; essence:)
import json,glob
rows=[json.load(open(f)) for f in glob.glob('internal/sidecar/evals/.ghx-evals/runs/*/*.json')]
rows=[r for r in rows if 'aborted' not in str(r.get('id',''))]
EOF

# counts used in this doc
ls internal/sidecar/evals/.ghx-evals/runs/gate-2026-07-03-aborted-rate-limit/*.json | wc -l   # 91 = 90 episodes + verdict.json (verdict.md is the 92nd file)
grep -c STATUS_CODE_ERROR ~/.ghx/sessions/badslugnoslash/traces.jsonl                          # 4
grep -o 'CLAUDE_SDK_CAN_USE_TOOL_SHADOWED' ~/.ghx/sessions/*/agent-stderr.log ~/.ghx/runtime/daemon.log | sort | uniq -c  # 5 sessions + 6 daemon
python3 -c "import json;m=json.load(open('/root/.ghx/sessions/badslugnoslash/meta.json'));print(m['turnCount'])"  # 0
grep -rn 'invalid repo' internal/ghx/repo.go internal/sidecar/tier2/cache.go                   # :28 / :122
grep -n 'ToolCalls' internal/sidecar/evals/episode.go                                          # :128
grep -rn 'no escalation rule fired' internal/sidecar/tier2/policy.go                           # :245
grep -in backoff internal/sidecar/{daemon,daemon_worker,acprunner}.go                          # 0 hits
wc -l ~/.ghx/runtime/daemon.log                                                                # 32
```

Per-episode/per-span sweeps cited in §1–§2 were executed programmatically
over all 170 episode files and all session OTel JSONL in this working session
(2026-08-21); intermediate tables are quoted inline in §1–§3 rather than
duplicated here. No code was modified; no files under `~/.ghx/` were written.
