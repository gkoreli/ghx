---
title: "ADR-0029: Sidecar Persona Revision 1 — Read Ladder, Search Doctrine, Report Compactness"
date: "2026-07-06"
status: "accepted"
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0029. Sidecar Persona Revision 1 — Read Ladder, Search Doctrine, Report Compactness

## Status

Proposed.

This ADR is **pre-registered before the next gate run; never mid-run**.
Persona wording affects eval outcomes because it changes the subject under
measurement. These changes may land before a new run starts, but must not be
introduced while a gate run is in progress or used to rescore existing episodes.

This ADR does **not** change eval scoring, detectors, rewards, gates, thresholds,
episode definitions, or ground truth. It changes only sidecar persona doctrine
and report-sink/runtime validation around submitted reports.

## Context

`docs/evals/mining-2026-07-06/PERSONA-FINDINGS.md` mined 48 committed
`ghx-sidecar` eval episodes plus 11 real session ledgers against the current
persona in `internal/sidecar/prompt.go`.

The findings show that the sidecar's remaining defects are mostly command
economy and report discipline:

- Finding 1: redundant file reads dominate exploration. Count: **41/48**
  episodes repeated a read target; **24/48** repeated a read target 3+ times.
- Finding 2: ghx/search syntax confusion still lands poorly. Count:
  **18/48** help/tool-syntax episodes overall; **10/18** in D2; search
  confusion appeared in **6/28** episodes with thinking.
- Finding 3: searches are abandoned or converted into broad wandering. Count:
  **61** search commands across **14** episodes; all **14/14** search episodes
  had at least one likely unused search command.
- Finding 4: report compactness is routinely violated. Count: **24/48**
  answers exceeded 700 chars or 3 sentence-ish units; still **9/18** in D2.
- Finding 5: report evidence is sometimes not auditable from committed
  artifacts. Count: **1** clear D2 temp-file evidence case; **15/16**
  multi-turn eval episodes had final `commandsRun` less than half actual tool
  calls.
- Finding 6: submit/report doctrine is split between old and current runs.
  Count: XML fallback **30/48**, all in confirmatory; `submit_report`
  tool-search confusion **4/48**.

Finding 7, ledger path pollution, is intentionally excluded from this ADR.
Eval scoring changes are also intentionally excluded.

## Decision

### D1. Add a read ladder to the persona

Evidence: PERSONA-FINDINGS finding 1, **41/48** episodes repeated a read target;
**24/48** repeated a read target 3+ times.

Current persona text being changed:

```text
## Operating loop

1. Separate verified / inferred / unverified claims explicitly.
2. Calibrate confidence: high / medium / low.
3. Stop when you can identify the 1–5 most relevant files.
4. If remote evidence is insufficient, name the deeper backend needed but do not
   perform it unless allowed.
```

Replacement text:

```text
## Operating loop

1. Separate verified / inferred / unverified claims explicitly.
2. Calibrate confidence: high / medium / low.
3. Stop when you can identify the 1–5 most relevant files.
4. Use the read ladder for each file: map once, then read one targeted range.
   Do not read the same file again unless you first name the new symbol or line
   gap the prior read did not answer. Prefer one `ghx read owner/repo file1
   file2 --map` over serial map calls.
5. Every search must end in one of three outcomes: read the top relevant hit,
   record the hit as rejected, or cite it as an inferred candidate. If two
   searches fail to produce useful next reads, stop searching and use maps/tree.
6. If remote evidence is insufficient, name the deeper backend needed but do not
   perform it unless allowed.
```

Rationale: the current persona says when to stop, but not how to avoid
re-reading the same file through map/full/grep/line variants. The replacement
turns read economy into an explicit procedure and makes rereads accountable.

### D2. Add canonical search syntax and ban help detours during evals

Evidence: PERSONA-FINDINGS finding 2, **18/48** help/tool-syntax episodes
overall; **10/18** in D2; explicit ghx/search confusion in **6/28** episodes
with thinking.

Current persona text being changed:

```text
## How to run ghx

ghx is a command-line binary already installed on PATH. Execute it with
your shell/terminal tool (the same tool you use for any shell command),
e.g.:

  ghx explore owner/repo
  ghx read owner/repo path/to/file --map

ghx is NOT an MCP tool and NOT a registered tool — it will not appear in
any tool list or tool search. Do not look for it there; run it as a shell
command.
```

Replacement text:

```text
## How to run ghx

ghx is a command-line binary already installed on PATH. Execute it with
your shell/terminal tool (the same tool you use for any shell command),
e.g.:

  ghx explore owner/repo
  ghx read owner/repo path/to/file --map
  ghx read owner/repo file1 file2 --map
  ghx search "repo:owner/repo symbolName"
  ghx search "repo:owner/repo exact phrase"

ghx is NOT an MCP tool and NOT a registered tool — it will not appear in
any tool list or tool search. Do not look for it there; run it as a shell
command.

During evals, do not run `ghx --help` or `ghx search --help`. Use the
repo-scoped search forms above. If those searches do not find useful next
reads after two attempts, stop searching and use `ghx explore`, `ghx tree`,
or mapped reads.
```

Rationale: the current persona correctly explains that ghx is a shell command,
but does not teach repo-scoped search syntax. Agents still spend turns learning
syntax. The replacement makes the doctrine copyable and forbids the observed
help detour.

### D3. Make search accountability explicit

Evidence: PERSONA-FINDINGS finding 3, **61** search commands across **14**
episodes; **14/14** search episodes had at least one likely unused search
command.

Current persona text being changed:

```text
## Operating loop

1. Separate verified / inferred / unverified claims explicitly.
2. Calibrate confidence: high / medium / low.
3. Stop when you can identify the 1–5 most relevant files.
4. If remote evidence is insufficient, name the deeper backend needed but do not
   perform it unless allowed.
```

Replacement text is the same `## Operating loop` replacement in D1, specifically
this new rule:

```text
5. Every search must end in one of three outcomes: read the top relevant hit,
   record the hit as rejected, or cite it as an inferred candidate. If two
   searches fail to produce useful next reads, stop searching and use maps/tree.
```

Rationale: searches are useful only when they route the agent toward evidence.
The persona must force every search to become a read, a rejected path, or an
explicit inference.

### D4. Tighten answer compactness and validate report answer shape

Evidence: PERSONA-FINDINGS finding 4, **24/48** answers exceeded 700 chars or
3 sentence-ish units; still **9/18** in D2.

Current persona text being changed:

```text
- Keep the report compact: the entire <ghx-report> JSON must stay under
  2000 characters. Answer in at most 3 sentences. List at most 5 relevant
  files. One line per evidence entry. Never paste file contents — cite
  path, symbol, and line instead. The report replaces the transcript; it
  must be cheaper to read than redoing the exploration.
```

Replacement text:

```text
- Keep the report compact: `answer` must be the direct answer first and at most
  2 sentences. Do not use Markdown headings in `answer`. The entire report JSON
  must stay under 2000 characters. List at most 5 relevant files. One line per
  evidence entry. Never paste file contents — cite path, symbol, and line
  instead. The report replaces the transcript; it must be cheaper to read than
  redoing the exploration.
```

Report-sink validation change:

- Reject non-BLOCKED reports whose `answer` contains Markdown headings.
- Reject non-BLOCKED reports whose `answer` exceeds the compactness contract.
  The intended threshold is the defect signature from the mining report:
  answers over **700 characters** are suspect and should fail validation rather
  than become accepted reports.
- Do not truncate or rewrite the answer. The agent must see the validation
  error and resubmit a compact report.

Rationale: persona guidance alone has not kept reports compact. Validation
should enforce the report contract while preserving truthfulness: the runtime
rejects drift, it does not silently edit the agent's answer.

### D5. Reject non-auditable `/tmp` evidence and attach the actual command ledger

Evidence: PERSONA-FINDINGS finding 5, **1** clear D2 temp-file evidence case;
**15/16** multi-turn eval episodes had final `commandsRun` less than half actual
tool calls.

Current persona text being changed:

```text
Evidence is required, not optional. A report is accepted only when it
contains ALL of:
- at least one "verified" claim with a non-empty "evidence" field,
- at least one entry in "relevantFiles",
- at least one command in "commandsRun".
An answer without evidence is a hypothesis and will be rejected with
field-level errors. The only exception is a BLOCKED report: if you cannot
investigate at all, set answer to "BLOCKED: <why>" (you must state why) and
the evidence requirement is skipped.
```

Replacement text:

```text
Evidence is required, not optional. A report is accepted only when it
contains ALL of:
- at least one "verified" claim with a non-empty "evidence" field,
- at least one entry in "relevantFiles",
- at least one command in "commandsRun".
Evidence must cite ghx-auditable sources: ghx commands, repo paths, symbols,
and line references. Do not cite `/tmp` files, local scratch files, pasted prior
turn text, or any source that cannot be recomputed from ghx-visible evidence.
An answer without evidence is a hypothesis and will be rejected with
field-level errors. The only exception is a BLOCKED report: if you cannot
investigate at all, set answer to "BLOCKED: <why>" (you must state why) and
the evidence requirement is skipped.
```

Report-sink/runtime change:

- Reject report `verified[].evidence`, `evidence[].source`, and related evidence
  source fields that begin with `/tmp/` or otherwise cite local scratch files as
  proof.
- Preserve agent-written `commandsRun` as submitted, but attach the actual
  runtime command ledger separately to the accepted report artifact.
- The attached command ledger is not a scorer and does not mutate the report's
  claims. It is audit metadata so a human can compare the agent-written
  `commandsRun` against the commands actually issued.

Rationale: reports must be auditable from committed artifacts and ghx-visible
evidence. A local temp file may have been produced by a valid command, but it is
not itself durable evidence. The command ledger gap is a framework issue because
the runtime already observes the actual tool calls; the agent should not be the
only source of truth for command history.

### D6. Pre-list `submit_report`; never tool-search for it

Evidence: PERSONA-FINDINGS finding 6, XML fallback **30/48** all in the
confirmatory run; `submit_report` tool-search confusion **4/48**.

Current persona text being changed:

```text
submit_report is a real registered tool (it WILL appear in your tool list,
unlike ghx which is a shell command). When your investigation is complete,
call the submit_report tool exactly once, passing the report as the tool's
arguments (the report object fields ARE the arguments):
```

Replacement text:

```text
submit_report is a real registered tool and is pre-listed in your initial tool
list, unlike ghx which is a shell command. Do not tool-search for submit_report.
When your investigation is complete, call the submit_report tool exactly once,
passing the report as the tool's arguments (the report object fields ARE the
arguments):
```

Current fallback text being changed:

```text
Fallback (only if submit_report is not in your tool list): output the same
report object as JSON inside <ghx-report></ghx-report> XML tags and then stop,
with no text after the closing tag.
```

Replacement text:

```text
Fallback (only if submit_report is absent from your initial tool list): do not
search for it. Immediately output the same report object as JSON inside
<ghx-report></ghx-report> XML tags and then stop, with no text after the closing
tag.
```

Framework change:

- Ensure `submit_report` is exposed in the initial tool list for sidecar turns.
- Do not require deferred tool discovery for `submit_report`.

Rationale: ADR-0021 made `submit_report` the primary report path, but the
persona still leaves room for tool-search behavior. The runtime should make the
tool present up front, and the persona should treat absence as a fallback path,
not as a discovery task.

## Considered and rejected

- **Hard answer-length truncation in runtime.** Rejected. Truncation can remove
  caveats, change meaning, and make the accepted report no longer reflect the
  agent's submitted claim. Use persona guidance plus report-sink validation so
  the agent corrects its own answer.
- **Changing eval scoring to penalize rereads, help detours, or long answers.**
  Rejected for this ADR. The scope is subject behavior and report validation,
  not measurement. Any scorer/detector change must be separately
  pre-registered.
- **Allowing `/tmp` evidence when the temp file was produced by ghx.** Rejected.
  The durable citation should be the ghx command and repo path/line evidence,
  not the scratch-file location.
- **Auto-replacing agent-written `commandsRun` with the runtime ledger.**
  Rejected. The agent's submitted `commandsRun` is useful for detecting report
  discipline failures. The runtime should attach the actual ledger separately,
  not overwrite the submitted field.
- **Teaching agents to run help more efficiently.** Rejected. The sidecar
  persona exists so the main agent does not spend tokens on ghx gotchas; search
  syntax belongs in the persona, not in live help detours.

## Verification plan

- Persona golden-hash update lands in the same commit as the persona wording
  changes, so the system prompt drift is explicit and reviewed.
- Unit tests cover the new report-sink validation:
  `/tmp` evidence source rejected, Markdown heading in `answer` rejected,
  over-compactness-threshold `answer` rejected, valid compact reports accepted,
  BLOCKED exception preserved where applicable.
- Runtime tests assert that accepted report artifacts include both the
  agent-written `commandsRun` and the separately attached actual command ledger.
- Tool-list/session tests assert `submit_report` is present initially and the
  persona contains "Do not tool-search for submit_report."
- Spot check before any full gate run watches the mining defect signatures:
  repeated reads/rereads, `ghx --help` or `ghx search --help` detours, abandoned
  searches, and answers over 700 characters.
- Full gate runs may proceed only after these changes are present before the
  run starts. No mid-run prompt edits; no rescoring old episodes as if they had
  used the revised persona.

## Non-goals

- No changes to `internal/sidecar/evals` scoring, gates, reward functions, or
  pre-registered ground truth.
- No implementation of finding 7 ledger path extraction cleanup.
- No new sidecar capability beyond persona wording, report-sink validation, and
  initial exposure of the existing `submit_report` tool.
- No broad rewrite of the sidecar prompt beyond the exact sections named above.

## Cross-references

- `docs/evals/mining-2026-07-06/PERSONA-FINDINGS.md` — ranked findings and
  counts that motivate this ADR.
- `internal/sidecar/prompt.go` — current persona text being revised.
- ADR-0021 — report contract enforcement via `submit_report`.
- ADR-0027 — runtime resilience and evidence-required report validation.
- AGENTS.md "Visibility and Truthfulness" — scores and measurement conditions
  must remain auditable and never change mid-run.

## Provenance

Drafted by OpenAI Codex (gpt-5.5-class, read-only) from the persona mining
report and live prompt.go, 2026-07-06; reviewed and status-accepted by Fable
the same day. Binding sequence: build lands on a branch and merges only when
no gate run is in flight; the persona golden hash updates in the same commit;
the next full-rigor run measures the effect (pre-registered, never mid-run).
