---
title: "Research 002 — persistent sidecar sessions and evidence-shaped memory"
date: "2026-08-21"
status: "research"
thread: "sidecar-runtime"
author: "delegated research worker (ox-alpha)"
scope: "read-only research artifact; proposes, does not implement; maintained under docs/research/"
builds-on: "ADR-0014.1 memory contract, ADR-0016.1 G4 gate, internal/sidecar/ledger.go, internal/sidecar/session.go, docs/dogfood/FRICTION.md"
---

> **Outcome:** consumed by ADR-0037 (M-1 (visible eviction) and M-2 (Commit/Branch snapshot stamps) implemented in PR #8; M-3 recall tool and M-4 durability chaos test remain open). Status above reflects the investigation phase; see the ADR for the decision record.

# Research 002: Persistent sidecar sessions and evidence-shaped memory

durable, recoverable, evidence-shaped working notebook that makes every
follow-up question cheaper and better than the first.
session memory", `docs/NORTH_STAR.md:72`) and capability §4 "Always-on runtime
and session mastery" (`docs/NORTH_STAR.md:161-173`); SAF workstreams B6/B7.
committed code, docs, and dogfood artifacts; proposes (does not implement)
decisions D1–D4 and an ADR-0037 pre-registration path. No existing file was
modified; nothing committed.
**Repo state at authoring:** branch `mainline`.

## 1. Problem statement

ADR-0014.1 (the sidecar vision) makes persistence a product contract, not a
runtime convenience: "Persistence is part of the product contract, not only a
runtime convenience"; "Sidecar memory must be evidence-shaped, not
vibes-shaped." Reconnaissance is conversational — "Where is middleware
composition?" → "now trace error propagation through that path" — and "a
sidecar with no persistent session repeats work and loses the value of previous
observations" (ADR-0014.1, The memory opportunity).

Today the runtime has real persistence machinery (§2), but four gaps separate
it from the §4 end state ("the sidecar routes each incoming question to the
right active session internally"; sessions survive anything):

1. **Memory is prompt-injected only.** All cross-turn context reaches the agent
   through one ≤1500-char ledger block in the turn prompt
   (`internal/sidecar/prompt.go:283-299`). There is no agent-addressable
   recall tool; everything else lives in the adapter's opaque conversation.
2. **No snapshot identity.** ADR-0014.1's session contract lists `commit` as a
   first-class memory field; `SessionMeta`
   (`internal/sidecar/session.go:13-62`) has no commit/branch field and no
   evidence is pinned to a ref. Remote-first recon silently floats on moving
   default branches.
3. **Ledger growth is unbounded and lossy under pressure.** Entries are never
   evicted (`internal/sidecar/ledger.go`); when the rendered block exceeds
   1500 chars the renderer silently drops oldest commands then paths
   (`prompt.go:288-305`) — the agent loses exactly the history that made the
   session valuable, with no signal that anything was dropped.
4. **Recovery is per-mechanism, not composed.** Wrap-up recovery (ADR-0027 D1),
   dead-session recreation (FRICTION 2026-07-06 fix `aeb86c3`), reroute +
   ledger rebuild (ADR-0030.1 D5), and artifacts-on-failure (ADR-0027 D3) each
   work, but nothing states or tests the combined invariant: *after any
   failure, restart, or mis-route, the next turn on a session serves from
   durable memory with no lost exploration and no duplicated work.*

## 2. Current-state inventory (what persistence exists today)

| Layer | Mechanism | Evidence |
|---|---|---|
| Session identity | `~/.ghx/sessions/<name>/meta.json`: name, repo, scope, turnCount, ACP session id, namedBy, cwd, agentCmd, spawnCwd, agentEnv (names only) | `internal/sidecar/session.go:13-62` |
| Evidence ledger | `ledger.json`: commands run, inspected paths, mapped globs, grep patterns, relevant/rejected files, backends, open questions — every entry turn-stamped | `internal/sidecar/ledger.go:13-38` |
| Ledger derivation | `UpdateLedgerFromTurn` merges report + trace-derived commands; ignores model-authored memory outside the report | `internal/sidecar/ledger.go:87-115` |
| Rebuild determinism | Replaying `reports/<turn>-<ts>.json` artifacts reproduces `ledger.json` byte-for-byte (TraceCommands persisted per artifact) | `internal/sidecar/session.go:178-195`; ADR-0030.1 D5 |
| Prompt injection | "Prior session context" + "Evidence ledger" blocks prepended on follow-up turns; hard budget 1500 chars | `internal/sidecar/prompt.go:269-282` |
| ACP resume | `meta.json.acpSessionID` reused across turns; LoadSession miss → one fresh ACP session, ledger still rides in prompt (`SessionRecreated` downgrade) | `internal/sidecar/acprunner.go:121-188`; FRICTION fix `aeb86c3` |
| Wrap-up recovery | max-turns exhaustion → resume + one "submit now" prompt; recorded `wrapUpRecovered` | ADR-0027 D1; FRICTION fix `ae832b3` |
| Routing | R1–R5 deterministic cascade routes questions to the right existing session; `sessions reroute` + deterministic dual-ledger rebuild heals mis-routes | ADR-0030.1 D1/D5; `internal/sidecar/route.go:229` |
| Failure artifacts | Failed turns emit artifacts + agent stderr log; visibility at the most valuable moment | ADR-0027 D3; `session.go:76-86` |
| View surface | `ghx sidecar view` lists sessions with turns/reports/traces; trace replay per ADR-0018 recipe | `internal/sidecar/view.go:50-98` |

**What is already north-star-true:** memory is evidence-shaped (derived from
reports + traces, never free-form model notes); it is auditable (rebuild
invariant); it survives process boundaries (daemon warm worker is an
optimization, the durable ledger carries cross-turn context —
`internal/sidecar/daemon_worker.go:66-69`).

## 3. Gap analysis, with evidence

### G1. Memory reaches the agent only through the prompt

`BuildPrompt` renders the ledger into the turn prompt
(`prompt.go:269-282`); `formatEvidenceLedger` caps the block at 1500 chars and
evicts by decrementing `commandLimit` then `pathLimit`
(`prompt.go:283-305`). Consequences:

- The ACP conversation (warm worker, `daemon_worker.go:66-69`) carries the
  rich history; the ledger block is a lossy summary of it. When the adapter
  conversation is lost (recreate path, worker expiry, adapter upgrade), the
  agent drops to the 1500-char view — an unmeasured cliff.
- The agent cannot *query* its own memory: it cannot ask "what did we already
  grep for?" beyond what the block shows. ADR-0031's anticipation machinery
  and ADR-0030.1's routing both consume the ledger better than the sidecar
  agent itself can.
- The 1500-char budget is a fixed guess, not a measured SPT decision
  (NORTH_STAR: "signals per token" is the optimization target; no artifact
  measures ledger-block cost vs follow-up-turn quality).

### G2. No snapshot identity (the missing `commit` field)

ADR-0014.1's session-contract table lists `commit` — "the snapshot or
default-branch head used for evidence" — and `backends` state. `SessionMeta`
has neither; `grep -n '"commit"' internal/sidecar/{session,report}.go` → no
match. Every ledger entry is implicitly "whatever the default branch was at
turn N". For a recon product whose answers cite files and lines, evidence
without a ref is unauditable across time — the exact failure the visibility
tenet forbids. Discovery sessions (ADR-0019.1) drift worse: one session can
sweep many repos while `meta.json.repo` stays empty or stale (FRICTION
2026-07-05: `relevantFiles` bare paths vs `meta.json.repo` mismatch on
`all-hands-ai/openhands`).

### G3. Unbounded ledger, silent eviction

`addLedgerEntry` only dedupes; nothing caps, weights, or summarizes
(`ledger.go:247-258`). A long-lived session (the §4 end state is
always-on, shared per project) grows until the renderer starts silently
dropping oldest-first. No metric counts dropped entries; no anomaly is
recorded; the agent never learns that turn-3 evidence vanished from its
prompt.

### G4. Recovery invariants are per-mechanism, never composed

Each mechanism is tested in isolation (wrap-up: ADR-0027 D1; recreate:
`aeb86c3`; reroute rebuild: ADR-0030.1 D5; failed-turn artifacts: ADR-0027
D3). Nothing composes them into one testable contract, and at least one seam
is soft: `RecordTurn` does a read-modify-write of `meta.json` with no lock or
retry (`session.go:148-156`) — two concurrent turns on one session (possible
once routing + daemon parallelism meet) can lose a turn count or, worse,
race the ACP session ID on recreation.

## 4. Design rationale — what the capability should be

### D1. Three memory tiers, one durable source of truth

Keep `ledger.json` as the single durable source of truth (rebuild-determinism
invariant stays). Introduce *rendering* tiers, not new stores:

- **T0 prompt block (exists):** compact, ≤ budget, injected every turn.
- **T1 recall tool (new):** `session_memory(query)` — an internal sidecar tool
  (same registry path as tier-2 tools, ADR-0024.2) that greps/reads
  `ledger.json` + `reports/` on demand. The agent gets full-fidelity recall
  without the prompt paying for it; every recall is a traceable tool call
  (visibility tenet), unlike the adapter's opaque conversation memory.
- **T2 raw artifacts (exists):** `reports/`, `traces.jsonl`, `live.jsonl` —
  the audit surface; humans and the main agent read these per NORTH_STAR §3.

Rationale: the main agent's context stays sacred (the sidecar absorbs recall
cost); SPT rises because the prompt carries only the hot set while cold
evidence is one cheap tool call away; and the blackbox risk (ADR-0014.1) is
dodged because recall is visible in the trace, not hidden chain-of-thought.

### D2. Snapshot identity: pin evidence to refs

Add `commit`/`ref` to `SessionMeta` (per ADR-0014.1's contract) and stamp
ledger entries with the ref they were collected against. Remote-first
constraint respected: the ref comes from the ghx backends' own metadata
(tree/read responses already resolve a head) — no clone required. Reports
gain a `snapshot` field; consumers (main agent, evals) can finally ask "is
this evidence still true at HEAD?" This also gives M7's future tier-2
(local-clone) evidence a place to record exactly what was mapped.

### D3. Bounded, honest memory

Replace silent eviction with a policy: hot-set budget for T0 (configurable at
the standard scopes — global/project/session/question, NORTH_STAR §3
control-mirrors-visibility), plus a `ledgerDropped` count surfaced in
metrics/anomalies and rendered as a one-line notice in the block ("…12 older
entries: `session_memory` recall"). The agent is never silently blind.

### D4. Composed durability contract

State and test the invariant: *for any turn end — success, max-turns wrap-up,
adapter crash, daemon restart, mis-route — the session's next turn is served
from durable memory (ledger + reports + meta) with zero lost exploration.*
Concretely: a single chaos-style integration test that interleaves the four
existing mechanisms; plus a file-lock (or atomic-rename CAS) on `meta.json`
turn accounting so concurrent turns cannot corrupt session state.

## 5. Alternatives considered

| Alternative | Why not |
|---|---|
| Vector-DB / embedding memory | Violates evidence-shaping (retrieval by similarity returns vibes, not turn-stamped evidence); heavy new infra; routing already solves the "which session" problem deterministically (ADR-0030.1 explicitly rejected embeddings in the routing path) |
| Store full transcripts as memory | Context pollution; the ledger's compactness is the product; raw transcripts already exist as T2 artifacts |
| Model-authored memory notes (agent writes its own summary file) | `UpdateLedgerFromTurn` deliberately ignores model-authored memory outside the report (`ledger.go:85-86`) — self-authored memory is unauditable and gameable; keep derivation ghx-owned |
| Do nothing (ACP conversation is the memory) | Fails at every boundary: recreation, worker expiry, daemon restart; the 2026-07-06 dead-session friction is the counterexample |
| Move memory behind MCP for the main agent | Inverts the boundary — the main agent must stay ignorant of sessions (§4); memory is the sidecar brain's internal organ |

## 6. Risks

- **Recall-tool abuse:** the agent could lean on recall instead of the hot set, or spam recall calls. Mitigation: keep T0 as primary; measure recall frequency in traces before/after (pre-registered metric); the tool returns real file content, so abuse is auditable like any other tool call.
- **Budget policy complexity:** configurable budgets at four scopes can misconfigure into amnesia. Mitigation: smart defaults out of the box (NORTH_STAR section 3); a misconfigured budget is visible via the ledgerDropped honesty signal.
- **Concurrency:** meta.json races (G4) can corrupt turn accounting or the ACP session ID. Mitigation: atomic CAS rename + one retry (the pattern SaveLedger already uses, ledger.go:59-83); escalate to a session-dir lock only if CAS proves insufficient in the chaos test.
- **Ref-pinning latency/cost:** resolving a head ref per turn adds API calls. Mitigation: resolve once per turn from existing response metadata; degrade to ref: unknown (recorded, never fabricated) rather than fail turns.
- **Eval validity:** any prompt-composition change alters the measured condition; per the frozen-stack rule (AGENTS.md), pre-register in the ADR and re-run gates before any new citable claim.

## 7. Proposed ADR outline

**Recommendation: new ADR-0037**, "Persistent session memory: recall tiers, snapshot identity, bounded ledgers". Rationale: this is a new decision family — neither ADR-0027 (resilience mechanisms), nor ADR-0030/0030.1 (daemon + routing), nor ADR-0031 (anticipation) owns memory shape; ADR-0014.1 is vision, not an implementation decision. Thread follow-ups as 0037.x. Frontmatter: no parent; thread: "sidecar-runtime"; cross-reference ADR-0014.1 as the origin contract and ADR-0027/0030.1 as consumed mechanisms.

Sections:
1. Status/Context — gaps G1-G4 with evidence pointers (section 3 above).
2. D1 three-tier memory (recall tool design, registry integration per ADR-0024.2 precedent).
3. D2 snapshot identity (SessionMeta.commit, ledger entry refs, report.snapshot; back-compat like the namedBy/cwd notes in session.go:25-38).
4. D3 budget policy + ledgerDropped honesty signal.
5. D4 composed durability contract + chaos test; meta.json CAS.
6. Alternatives considered (section 5 table).
7. Eval-validity note: prompt-composition change declared to the measurement stack; gates re-run before any new citable claim.
8. Implementation notes (filled after build, per AGENTS.md living-record rule).

## 8. Interaction with the rest of the north star

- **B7 routing (ADR-0030.1):** memory is what makes routing safe — reroute rebuild works only because ledger derivation is deterministic. The D4 composed contract is the guarantee B7 phase-2 dogfood audit should check.
- **M8 anticipation (ADR-0031.x):** anticipation consumes OpenQuestions (the union of every report uncertainty + nextReads) as its trigger vocabulary; D3 bounded-but-honest ledger keeps that signal from being evicted first. ADR-0031 sequencing note (one resolver serves both) is already answered by ADR-0030.1 D6 — memory work must not fork it.
- **M7 tier-2 escalation (ADR-0024.x):** tier-2 evidence (local maps) needs D2 snapshot identity most — local evidence without a commit pin is the least auditable kind.
- **M9/M10 training track:** turn-stamped ledgers + reports are exactly the trajectory format ADR-0017.1 exports; durable sessions multiply training data per session at zero extra eval cost.

## 9. Open questions

1. One generic recall tool (session_memory(query)) or typed accessors (recall_greps, recall_files)? Generic keeps the tool surface small; typed keeps prompts precise — decide in ADR-0037 with persona-revision evidence (ADR-0029 pattern).
2. Should T0 budgets be depth-linked (cheap/normal/deep) or scope-configurable only? Depth-linking is simpler; scope-configurability is what NORTH_STAR section 3 promises for control.
3. Does snapshot identity extend to discovery sessions (many repos, one session)? Candidate: per-entry repo+ref stamps rather than a session-level commit.
4. Is a full flock on the session dir warranted, or is CAS-on-meta enough? Let the chaos test answer.
5. Recall-tool exposure: internal-only (sidecar brain) now; revisit main-agent visibility only via the reports/traces surface, never a new MCP tool (boundary rule).

## 10. Evidence appendix

Commands run (read-only) while authoring:
- ls docs/research docs/adr internal/sidecar — corpus inventory.
- grep -n "LoadLedger|UpdateLedgerFromTurn|ledger" across runtime.go/prompt.go/daemon_worker.go/route.go — injection + derivation paths.
- grep -n resume/acpSessionID across turn.go/acprunner.go — resume mechanics.
- grep -n session/memory/ledger/resume docs/dogfood/FRICTION.md — friction evidence (dead session aeb86c3, wrap-up ae832b3, openhands path/repo drift).
- grep -n registry/session internal/sidecar/daemon.go; grep -n func internal/sidecar/route.go — daemon + routing surface.
- sed -n reads of ADR-0014.1 (memory opportunity, persistent evidence memory, session contract), ADR-0027 (D1/D3), ADR-0030.1 (D1/D5/D6), prompt.go:230-360, session.go, ledger.go, runtime.go:220-340, daemon_worker.go:55-85.
- No ~/.ghx production sessions present on this machine (ls returned only a test slug); all session evidence cited from committed docs/tests instead.

Key file:line anchors: internal/sidecar/prompt.go:283-305 (1500-char budget + eviction); internal/sidecar/ledger.go:13-38 (schema), :87-115 (derivation), :247-258 (dedupe-only growth); internal/sidecar/session.go:13-62 (meta, no commit field), :148-156 (RecordTurn RMW race), :178-195 (rebuildable artifact); internal/sidecar/runtime.go:404-414 (ledger load into prompt), :566-568 (post-turn ledger save); internal/sidecar/acprunner.go:121-188 (resume token); internal/sidecar/daemon_worker.go:66-69 (warm worker as optimization); internal/sidecar/route.go:229 (RouteQuestion).
