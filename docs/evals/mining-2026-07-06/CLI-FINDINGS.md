# Agent-usage mining: ghx CLI defects (workstream A2)

Miner: OpenAI Codex (gpt-5.5-class) via `codex exec -s read-only`, 2026-07-06 —
cross-family per the anti-self-preference principle. Corpus: 144 committed
eval episodes (docs/evals/gate-run-2026-07-05-confirmatory/, -d2-partial/) +
11 live production sessions (~/.ghx/sessions/). Mandate: treat agents as the
CLI's users and their traces as usage analytics. Verbatim report below; no
files were modified by the miner.

---

answer

Ranked findings, by observed frequency x output cost:

1. `ghx explore <repo>` is too large as a default orientation command.
Count: 65 traces, 1,371,053 output chars.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783297015159.json:turn 0` (`57,311` chars), `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783310551196.json:turn 0` (`57,311`), `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783295296326.json:turn 0` (`26,091`).
Recommendation: make `explore` budgeted by default: compact README summary, capped root entries, suggested subpaths. Move current behavior behind `--full`, add `--files-limit`, `--readme-lines`, and `--no-readme`.

2. Agents repeatedly shell-filter `ghx read`, which means the native narrowing surface is not discoverable enough.
Count: 145 `ghx read | sed` traces, 994,097 chars; 356 `ghx read | grep/head` traces, 1,281,676 chars.
Refs: `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx_1783310407717.json:turn 0` (`ghx read ... | sed -n '1,320p'`), `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783298297949.json:turn 0` (`... | head -400`), `/Users/goga/.ghx/sessions/mastra-ai-mastra/traces.jsonl:turn 1` (`... | head -300`, `26,354` chars).
Recommendation: add first-class aliases and examples in errors/help: `--start N --end M`, `--offset N --limit M`, `path:START-END`, and `--around PATTERN[:N]`. When output exceeds a threshold, print “use `--lines A-B`, `--grep`, or `--map`” in the header.

3. The natural line-range guesses fail.
Count: 28 direct failures: `--start/--end` 22, `--offset/--limit` 5, `--line-range` 1. Retry/flail sequence count: 21 runs.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783297826596.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783295974379.json:turn 0`, `/Users/goga/.ghx/sessions/letta-ai-letta/traces.jsonl:turn 1`.
Recommendation: support all three aliases as canonical synonyms for `--lines`; error text should say: `unknown flag --start; did you mean --lines START-END? Also accepted: --start N --end M, --offset N --limit M`.

4. Missing “inspect this concern” workflow causes repeated multi-command chains.
Counts: `map -> line reads` 77 runs; `explore -> subdir explore/tree -> read` 63; `search -> read result files` 46.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783294017695.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783297015159.json:turn 0`, `/Users/goga/.ghx/sessions/letta-ai-letta/traces.jsonl:turn 1`.
Recommendation: add `ghx inspect <repo> <query>` or `ghx focus <repo> --query ...` that returns ranked files, structural maps, and bounded snippets in one budgeted response. This is the clearest missing-feature pattern.

5. Agents expect `ghx grep`.
Count: 9 failures, plus 9 retry sequences into `search`, `read --grep`, or help.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783295061437.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783296851961.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783297127420.json:turn 0`.
Recommendation: add `ghx grep <repo> <pattern> [--glob GLOB] [--path PATH]` as a friendly alias over search/read-grep semantics. Current “Did you mean tree?” is actively misleading; change to “Did you mean `ghx search` or `ghx read --grep`?”

6. Full-file reads still happen where maps/ranges would do.
Count: 22 full `ghx read` traces over 12k chars without `--map`, `--lines`, or `--grep`; 572,144 chars.
Refs: `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx_1783309743757.json:turn 0` (`gin.go` 54,525; `tree.go` 48,835), `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783296737539.json:turn 0` (`37,047`), `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783293764685.json:turn 0` (`36,883`).
Recommendation: for large files, default to `--map` plus “top matching line anchors” unless `--full` is supplied, or add `--budget CHARS` with automatic structural-first truncation.

7. Search ergonomics are too close to raw GitHub code search.
Counts: 8 unbounded broad searches over 8k chars, 95,248 chars; 7 `--lang` failures; 2 GitHub query parse fatals.
Refs: `/Users/goga/.ghx/sessions/microsoft-autogen/traces.jsonl:turn 1`, `/Users/goga/.ghx/sessions/arize-ai-phoenix/traces.jsonl:turn 1`, `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783296534553.json:turn 0`.
Recommendation: add `ghx search <repo> <query>` as a first-class form, keep current GitHub-query form as advanced mode, support `--lang/--ext/--glob`, and escape/quote fatal GitHub query characters automatically when possible.

8. `ghx --version` fails despite being the natural health check.
Count: 30 failures.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783294237204.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783297480460.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783293921812.json:turn 0`.
Recommendation: support `--version` alongside `ghx version`.

Cross-profile comparison

Plain-profile agents are sometimes more efficient because raw `gh api`/`curl` makes “fetch exact file then `sed`/`grep`” obvious. I counted 188 plain traces using `gh api contents + base64 -d + sed/head/grep` for 773,804 chars, plus 11 `curl raw + sed/head` traces for 45,612 chars. Example: `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783296050223.json:turn 0` uses raw GitHub + `sed -n '55,100p'`. The ghx equivalent often became full reads, shell pipes, or failed guessed range flags. Gap: ghx should make precise slices as natural as raw `curl | sed`.

Plain is not universally better: 9 plain recursive-tree traces produced 185,319 chars, so ghx’s `tree/explore` can still win when compacted.

files changed or inspected

No files modified. Inspected eval JSONs under:
`docs/evals/gate-run-2026-07-05-confirmatory/`
`docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/`
`/Users/goga/.ghx/sessions/*/traces.jsonl`

commands run

`rg --files ...`
`find ...`
`jq ... toolTraces/status/outputExcerpt ...`
Inline `python3 -c` aggregation over episode JSONs and live trace JSONL.

test results

No tests run; this was read-only artifact mining.

evidence snippets or line references

The refs above are file + turn references. Aggregate evidence from the mining command: 2,068 tool traces parsed, 1,535 ghx-related; profile totals: `ghx` 599 traces / 3,717,551 chars / 37 failed, `ghx-sidecar` 552 / 1,884,747 / 55 failed, `plain` 577 / 3,274,938 / 14 failed, live sidecar 340 / 1,192,946 / 13 failed.

uncertainty

Counts are trace-level, not unique command-level; multi-command shell traces can contain more than one ghx invocation. Output-size severity uses recorded tool output chars, not hidden network/API bytes.

suggested next step

Implement the top-5 quick wins:

1. Add `ghx --version`.
2. Add range aliases: `--start/--end`, `--offset/--limit`, `--line-range`, and `path:START-END`.
3. Add `ghx grep` with `--glob`, `--path`, `--limit`.
4. Make root `explore` compact by default; move current verbose output to `--full`.
5. Add `ghx inspect/focus <repo> <query>` for search + map + bounded snippets in one call.
tokens used
118,114
answer

Ranked findings, by observed frequency x output cost:

1. `ghx explore <repo>` is too large as a default orientation command.
Count: 65 traces, 1,371,053 output chars.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783297015159.json:turn 0` (`57,311` chars), `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783310551196.json:turn 0` (`57,311`), `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783295296326.json:turn 0` (`26,091`).
Recommendation: make `explore` budgeted by default: compact README summary, capped root entries, suggested subpaths. Move current behavior behind `--full`, add `--files-limit`, `--readme-lines`, and `--no-readme`.

2. Agents repeatedly shell-filter `ghx read`, which means the native narrowing surface is not discoverable enough.
Count: 145 `ghx read | sed` traces, 994,097 chars; 356 `ghx read | grep/head` traces, 1,281,676 chars.
Refs: `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx_1783310407717.json:turn 0` (`ghx read ... | sed -n '1,320p'`), `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783298297949.json:turn 0` (`... | head -400`), `/Users/goga/.ghx/sessions/mastra-ai-mastra/traces.jsonl:turn 1` (`... | head -300`, `26,354` chars).
Recommendation: add first-class aliases and examples in errors/help: `--start N --end M`, `--offset N --limit M`, `path:START-END`, and `--around PATTERN[:N]`. When output exceeds a threshold, print “use `--lines A-B`, `--grep`, or `--map`” in the header.

3. The natural line-range guesses fail.
Count: 28 direct failures: `--start/--end` 22, `--offset/--limit` 5, `--line-range` 1. Retry/flail sequence count: 21 runs.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783297826596.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783295974379.json:turn 0`, `/Users/goga/.ghx/sessions/letta-ai-letta/traces.jsonl:turn 1`.
Recommendation: support all three aliases as canonical synonyms for `--lines`; error text should say: `unknown flag --start; did you mean --lines START-END? Also accepted: --start N --end M, --offset N --limit M`.

4. Missing “inspect this concern” workflow causes repeated multi-command chains.
Counts: `map -> line reads` 77 runs; `explore -> subdir explore/tree -> read` 63; `search -> read result files` 46.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783294017695.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783297015159.json:turn 0`, `/Users/goga/.ghx/sessions/letta-ai-letta/traces.jsonl:turn 1`.
Recommendation: add `ghx inspect <repo> <query>` or `ghx focus <repo> --query ...` that returns ranked files, structural maps, and bounded snippets in one budgeted response. This is the clearest missing-feature pattern.

5. Agents expect `ghx grep`.
Count: 9 failures, plus 9 retry sequences into `search`, `read --grep`, or help.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783295061437.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783296851961.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783297127420.json:turn 0`.
Recommendation: add `ghx grep <repo> <pattern> [--glob GLOB] [--path PATH]` as a friendly alias over search/read-grep semantics. Current “Did you mean tree?” is actively misleading; change to “Did you mean `ghx search` or `ghx read --grep`?”

6. Full-file reads still happen where maps/ranges would do.
Count: 22 full `ghx read` traces over 12k chars without `--map`, `--lines`, or `--grep`; 572,144 chars.
Refs: `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx_1783309743757.json:turn 0` (`gin.go` 54,525; `tree.go` 48,835), `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783296737539.json:turn 0` (`37,047`), `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783293764685.json:turn 0` (`36,883`).
Recommendation: for large files, default to `--map` plus “top matching line anchors” unless `--full` is supplied, or add `--budget CHARS` with automatic structural-first truncation.

7. Search ergonomics are too close to raw GitHub code search.
Counts: 8 unbounded broad searches over 8k chars, 95,248 chars; 7 `--lang` failures; 2 GitHub query parse fatals.
Refs: `/Users/goga/.ghx/sessions/microsoft-autogen/traces.jsonl:turn 1`, `/Users/goga/.ghx/sessions/arize-ai-phoenix/traces.jsonl:turn 1`, `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783296534553.json:turn 0`.
Recommendation: add `ghx search <repo> <query>` as a first-class form, keep current GitHub-query form as advanced mode, support `--lang/--ext/--glob`, and escape/quote fatal GitHub query characters automatically when possible.

8. `ghx --version` fails despite being the natural health check.
Count: 30 failures.
Refs: `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783294237204.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783297480460.json:turn 0`, `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783293921812.json:turn 0`.
Recommendation: support `--version` alongside `ghx version`.

Cross-profile comparison

Plain-profile agents are sometimes more efficient because raw `gh api`/`curl` makes “fetch exact file then `sed`/`grep`” obvious. I counted 188 plain traces using `gh api contents + base64 -d + sed/head/grep` for 773,804 chars, plus 11 `curl raw + sed/head` traces for 45,612 chars. Example: `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783296050223.json:turn 0` uses raw GitHub + `sed -n '55,100p'`. The ghx equivalent often became full reads, shell pipes, or failed guessed range flags. Gap: ghx should make precise slices as natural as raw `curl | sed`.

Plain is not universally better: 9 plain recursive-tree traces produced 185,319 chars, so ghx’s `tree/explore` can still win when compacted.

files changed or inspected

No files modified. Inspected eval JSONs under:
`docs/evals/gate-run-2026-07-05-confirmatory/`
`docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/`
`/Users/goga/.ghx/sessions/*/traces.jsonl`

commands run

`rg --files ...`
`find ...`
`jq ... toolTraces/status/outputExcerpt ...`
Inline `python3 -c` aggregation over episode JSONs and live trace JSONL.

test results

No tests run; this was read-only artifact mining.

evidence snippets or line references

The refs above are file + turn references. Aggregate evidence from the mining command: 2,068 tool traces parsed, 1,535 ghx-related; profile totals: `ghx` 599 traces / 3,717,551 chars / 37 failed, `ghx-sidecar` 552 / 1,884,747 / 55 failed, `plain` 577 / 3,274,938 / 14 failed, live sidecar 340 / 1,192,946 / 13 failed.

uncertainty

Counts are trace-level, not unique command-level; multi-command shell traces can contain more than one ghx invocation. Output-size severity uses recorded tool output chars, not hidden network/API bytes.

suggested next step

Implement the top-5 quick wins:

1. Add `ghx --version`.
2. Add range aliases: `--start/--end`, `--offset/--limit`, `--line-range`, and `path:START-END`.
3. Add `ghx grep` with `--glob`, `--path`, `--limit`.
4. Make root `explore` compact by default; move current verbose output to `--full`.
5. Add `ghx inspect/focus <repo> <query>` for search + map + bounded snippets in one call.
