# Agent-usage mining: sidecar persona & trail defects (workstream B8)

Miner: OpenAI Codex (gpt-5.5-class) via `codex exec -s read-only`, 2026-07-06.
Corpus: 48 committed ghx-sidecar episodes + 11 real session ledgers, compared
against the doctrine in internal/sidecar/prompt.go. Verbatim report below;
recommendations are split 'persona wording fix' vs 'framework capability gap'.

---

answer

Mined 48 committed `ghx-sidecar` eval episodes and 11 real session ledgers. The biggest behavioral defects are command economy and report discipline, not final correctness. The persona says the sidecar should hide ghx gotchas, stop at 1-5 relevant files, and produce a compact auditable report ([prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:45), [prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:117)); actual trails still show agents spending turns learning ghx syntax, rereading targets, and writing long reports.

**Ranked Findings**

1. **Redundant file reads dominate exploration**
Pattern: same read target reread with map/full/grep/line variants, often after enough evidence exists.
Count: 41/48 episodes repeated a read target; 24/48 repeated a read target 3+ times. Waste markers clustered start=35, middle=14, wrap-up=8; rereads mostly middle/wrap-up after initial maps.
Refs: `lib/application.js` read 3-4x in Express episodes; `tree.go` read 7x in [gin-routing](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783295375346.json:12); Hono reads `src/compose.ts`, then searches, then reads `hono-base.ts` via ghx and `/tmp` in [hono-middleware](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:31).
Recommendation: **persona wording fix**: add a “read ladder” rule: “For each file: map once, then read one targeted range. Do not read the same file again unless you name the new symbol/line gap first. Prefer one `ghx read repo file1 file2 --map` over serial maps.”

2. **ghx/search syntax confusion is still landing poorly**
Pattern: agents check help and reformulate searches mid-run instead of knowing `ghx search` doctrine.
Count: help/tool-syntax episodes 18/48 overall; still 10/18 in D2. Thinking explicitly shows ghx/search confusion in 6/28 episodes with thinking.
Refs: Hono thinking says it needs help for quoted search syntax and repo scoping, then runs failed/search/help detours ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:27), [hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:33)); OpenAI streaming has the same thinking and help detour ([openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:25), [openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:37)).
Recommendation: **persona wording fix**: include exact repo-scoped examples: `ghx search "repo:owner/repo symbolName"`, `ghx search "repo:owner/repo exact phrase"`, and “Do not run `ghx --help`/`ghx search --help` during evals; use these patterns.”

3. **Searches are abandoned or converted into broad wandering**
Pattern: search results are run, then final evidence cites later reads or omits those searches entirely.
Count: 61 search commands across 14 episodes; all 14 search episodes had at least one likely unused search command.
Refs: Flask runs 4-6 searches plus search help before normal file reads; Hono runs multiple compose/errorHandler searches but final evidence rests on file reads; OpenAI streaming runs 6 searches before settling on `parse.ts`/resource reads ([openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:33)).
Recommendation: **persona wording fix**: “Every search must end in one of: read the top hit, record it as rejected, or cite it as inferred. If two searches fail, stop searching and use maps/tree.”

4. **Report compactness is routinely violated**
Pattern: answers are comprehensive mini-briefs, not 1-3 sentence reconnaissance summaries.
Count: 24/48 answers exceeded 700 chars or 3 sentence-ish units; still 9/18 in D2.
Refs: Hono final answer is ~981 chars and buries the direct answer in mechanism detail ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:1556)); OpenAI streaming answer is ~915 chars ([openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:1126)); real session reports expand into multi-section essays despite sidecar role, e.g. CrewAI report stored in [ledger.json](/Users/goga/.ghx/sessions/crewaiinc-crewai/ledger.json:290).
Recommendation: **persona wording fix + runtime gap**: prompt should require `answer = direct answer first, max 2 sentences`; runtime validator should enforce answer length and reject Markdown headings in `answer`.

5. **Report evidence is sometimes not auditable from committed artifacts**
Pattern: reports cite local temp files or prior-turn text instead of ghx command evidence.
Count: 1 clear D2 temp-file evidence case; 15/16 multi-turn eval episodes had final `commandsRun` less than half actual tool calls, weakening audit trails.
Refs: Hono report cites `/tmp/hono-base.txt` lines as verified evidence ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:651)); final commands list only two commands from turn 2, omitting most turn-1 evidence ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:1603)).
Recommendation: **framework capability gap**: report sink should reject evidence sources beginning `/tmp/` and should auto-attach full actual command ledger separately from agent-written `commandsRun`.

6. **Submit/report doctrine split between old and current runs**
Pattern: confirmatory run used XML fallback despite prompt saying `submit_report` should be used; D2 mostly fixed via tool sink, but agents still search for the tool.
Count: XML fallback 30/48, all in confirmatory; `submit_report` tool-search confusion 4/48.
Refs: prompt forbids looking for ghx as a tool but says `submit_report` will appear ([prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:58), [prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:64)); Hono searches for `mcp__ghx-report-sink__submit_report` anyway ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:523)).
Recommendation: **framework capability gap**: expose `submit_report` in the initial tool list without requiring tool discovery; prompt should say “Do not tool-search for submit_report. If absent, immediately use XML fallback.”

7. **Ledger is used, but ledger quality is polluted**
Pattern: real multi-turn sessions usually avoid root re-exploration, but ledger path extraction stores shell syntax as inspected paths.
Count: 2/11 real sessions are multi-turn; both use prior context reasonably. 11/11 ledgers contain polluted inspected paths.
Refs: CrewAI turn 2 starts on new event/tracing areas, not root/version ([ledger](/Users/goga/.ghx/sessions/crewaiinc-crewai/ledger.json:250)); but inspected paths include `2>&1`, `echo`, `ghx`, `read` ([ledger](/Users/goga/.ghx/sessions/crewaiinc-crewai/ledger.json:308)); Inspect has `2>&1`, `|`, `head` as inspected paths ([ledger](/Users/goga/.ghx/sessions/ukgovernmentbeis-inspect-ai/ledger.json:280)).
Recommendation: **framework capability gap**: ledger extraction should parse ghx command AST, not shell tokens; only `ghx read` path arguments and `ghx explore` scopes should become inspected paths.

**Top-5 Persona Edits**

1. Add a “read ladder”: map once, targeted range once, reread only after naming the missing symbol/line gap.
2. Add canonical ghx search syntax and ban `ghx --help` / `ghx search --help` during eval runs.
3. Add “search accountability”: every search must lead to read, rejected path, or inferred candidate.
4. Replace compactness prose with a hard report template: `answer` max 2 sentences; no Markdown headings; direct answer first.
5. Add audit wording: evidence must cite ghx sources, never `/tmp`, pasted prior text, or local scratch files.

files changed or inspected

No files modified. Inspected `internal/sidecar/prompt.go`, 48 `*_ghx-sidecar_*.json` eval episodes under the two requested `docs/evals/...` directories, and 11 `~/.ghx/sessions/*/ledger.json` plus report files.

commands run

Used `sed`, `find`, `jq`, `nl`, and inline `python3 -c` read-only scans to count episodes, command patterns, report lengths, uncertainty fields, thinking markers, and ledger pollution.

test results

No tests run; this was a read-only artifact-mining task.

uncertainty

Counts use deterministic JSON scans plus spot checks. “Abandoned search” is a conservative text match: search command absent from final report evidence/commands, so it may undercount or overcount if a search informed a later read without being cited.

suggested next step

Implement prompt edits plus runtime validators for answer length, `/tmp` evidence rejection, full command-ledger attachment, and ledger path parsing cleanup.
tokens used
135,807
answer

Mined 48 committed `ghx-sidecar` eval episodes and 11 real session ledgers. The biggest behavioral defects are command economy and report discipline, not final correctness. The persona says the sidecar should hide ghx gotchas, stop at 1-5 relevant files, and produce a compact auditable report ([prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:45), [prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:117)); actual trails still show agents spending turns learning ghx syntax, rereading targets, and writing long reports.

**Ranked Findings**

1. **Redundant file reads dominate exploration**
Pattern: same read target reread with map/full/grep/line variants, often after enough evidence exists.
Count: 41/48 episodes repeated a read target; 24/48 repeated a read target 3+ times. Waste markers clustered start=35, middle=14, wrap-up=8; rereads mostly middle/wrap-up after initial maps.
Refs: `lib/application.js` read 3-4x in Express episodes; `tree.go` read 7x in [gin-routing](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783295375346.json:12); Hono reads `src/compose.ts`, then searches, then reads `hono-base.ts` via ghx and `/tmp` in [hono-middleware](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:31).
Recommendation: **persona wording fix**: add a “read ladder” rule: “For each file: map once, then read one targeted range. Do not read the same file again unless you name the new symbol/line gap first. Prefer one `ghx read repo file1 file2 --map` over serial maps.”

2. **ghx/search syntax confusion is still landing poorly**
Pattern: agents check help and reformulate searches mid-run instead of knowing `ghx search` doctrine.
Count: help/tool-syntax episodes 18/48 overall; still 10/18 in D2. Thinking explicitly shows ghx/search confusion in 6/28 episodes with thinking.
Refs: Hono thinking says it needs help for quoted search syntax and repo scoping, then runs failed/search/help detours ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:27), [hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:33)); OpenAI streaming has the same thinking and help detour ([openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:25), [openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:37)).
Recommendation: **persona wording fix**: include exact repo-scoped examples: `ghx search "repo:owner/repo symbolName"`, `ghx search "repo:owner/repo exact phrase"`, and “Do not run `ghx --help`/`ghx search --help` during evals; use these patterns.”

3. **Searches are abandoned or converted into broad wandering**
Pattern: search results are run, then final evidence cites later reads or omits those searches entirely.
Count: 61 search commands across 14 episodes; all 14 search episodes had at least one likely unused search command.
Refs: Flask runs 4-6 searches plus search help before normal file reads; Hono runs multiple compose/errorHandler searches but final evidence rests on file reads; OpenAI streaming runs 6 searches before settling on `parse.ts`/resource reads ([openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:33)).
Recommendation: **persona wording fix**: “Every search must end in one of: read the top hit, record it as rejected, or cite it as inferred. If two searches fail, stop searching and use maps/tree.”

4. **Report compactness is routinely violated**
Pattern: answers are comprehensive mini-briefs, not 1-3 sentence reconnaissance summaries.
Count: 24/48 answers exceeded 700 chars or 3 sentence-ish units; still 9/18 in D2.
Refs: Hono final answer is ~981 chars and buries the direct answer in mechanism detail ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:1556)); OpenAI streaming answer is ~915 chars ([openai-node](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json:1126)); real session reports expand into multi-section essays despite sidecar role, e.g. CrewAI report stored in [ledger.json](/Users/goga/.ghx/sessions/crewaiinc-crewai/ledger.json:290).
Recommendation: **persona wording fix + runtime gap**: prompt should require `answer = direct answer first, max 2 sentences`; runtime validator should enforce answer length and reject Markdown headings in `answer`.

5. **Report evidence is sometimes not auditable from committed artifacts**
Pattern: reports cite local temp files or prior-turn text instead of ghx command evidence.
Count: 1 clear D2 temp-file evidence case; 15/16 multi-turn eval episodes had final `commandsRun` less than half actual tool calls, weakening audit trails.
Refs: Hono report cites `/tmp/hono-base.txt` lines as verified evidence ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:651)); final commands list only two commands from turn 2, omitting most turn-1 evidence ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:1603)).
Recommendation: **framework capability gap**: report sink should reject evidence sources beginning `/tmp/` and should auto-attach full actual command ledger separately from agent-written `commandsRun`.

6. **Submit/report doctrine split between old and current runs**
Pattern: confirmatory run used XML fallback despite prompt saying `submit_report` should be used; D2 mostly fixed via tool sink, but agents still search for the tool.
Count: XML fallback 30/48, all in confirmatory; `submit_report` tool-search confusion 4/48.
Refs: prompt forbids looking for ghx as a tool but says `submit_report` will appear ([prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:58), [prompt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/prompt.go:64)); Hono searches for `mcp__ghx-report-sink__submit_report` anyway ([hono](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json:523)).
Recommendation: **framework capability gap**: expose `submit_report` in the initial tool list without requiring tool discovery; prompt should say “Do not tool-search for submit_report. If absent, immediately use XML fallback.”

7. **Ledger is used, but ledger quality is polluted**
Pattern: real multi-turn sessions usually avoid root re-exploration, but ledger path extraction stores shell syntax as inspected paths.
Count: 2/11 real sessions are multi-turn; both use prior context reasonably. 11/11 ledgers contain polluted inspected paths.
Refs: CrewAI turn 2 starts on new event/tracing areas, not root/version ([ledger](/Users/goga/.ghx/sessions/crewaiinc-crewai/ledger.json:250)); but inspected paths include `2>&1`, `echo`, `ghx`, `read` ([ledger](/Users/goga/.ghx/sessions/crewaiinc-crewai/ledger.json:308)); Inspect has `2>&1`, `|`, `head` as inspected paths ([ledger](/Users/goga/.ghx/sessions/ukgovernmentbeis-inspect-ai/ledger.json:280)).
Recommendation: **framework capability gap**: ledger extraction should parse ghx command AST, not shell tokens; only `ghx read` path arguments and `ghx explore` scopes should become inspected paths.

**Top-5 Persona Edits**

1. Add a “read ladder”: map once, targeted range once, reread only after naming the missing symbol/line gap.
2. Add canonical ghx search syntax and ban `ghx --help` / `ghx search --help` during eval runs.
3. Add “search accountability”: every search must lead to read, rejected path, or inferred candidate.
4. Replace compactness prose with a hard report template: `answer` max 2 sentences; no Markdown headings; direct answer first.
5. Add audit wording: evidence must cite ghx sources, never `/tmp`, pasted prior text, or local scratch files.

files changed or inspected

No files modified. Inspected `internal/sidecar/prompt.go`, 48 `*_ghx-sidecar_*.json` eval episodes under the two requested `docs/evals/...` directories, and 11 `~/.ghx/sessions/*/ledger.json` plus report files.

commands run

Used `sed`, `find`, `jq`, `nl`, and inline `python3 -c` read-only scans to count episodes, command patterns, report lengths, uncertainty fields, thinking markers, and ledger pollution.

test results

No tests run; this was a read-only artifact-mining task.

uncertainty

Counts use deterministic JSON scans plus spot checks. “Abandoned search” is a conservative text match: search command absent from final report evidence/commands, so it may undercount or overcount if a search informed a later read without being cited.

suggested next step

Implement prompt edits plus runtime validators for answer length, `/tmp` evidence rejection, full command-ledger attachment, and ledger path parsing cleanup.
