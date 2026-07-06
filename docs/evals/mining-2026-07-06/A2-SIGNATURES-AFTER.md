# A2 Defect Signatures — Post-Batch Comparison (AFTER)

## Provenance

- Miner: cross-family codex (GPT-5.5-class) analyst, read-only; report
  harvested verbatim from the worker log (`/tmp/codex-a2-signatures.log`)
  on 2026-07-06.
- Corpora compared: baseline = 155 units from `CLI-FINDINGS.md` in this
  directory (pre-batch mining corpus); post-batch = 22 units (16 eval
  episodes from `internal/sidecar/evals/.ghx-evals/runs/gate-run-2026-07-06-persona/`
  plus 6 live sessions with mtime after 2026-07-06 00:30, including
  `spot-a2-gin`).
- **Persona confound caveat — read before the table.** The ADR-0029
  persona revision landed between the two corpora, the corpus sizes
  differ sharply (155 vs 22 units), and counts are trace-level
  (multi-command shell traces can contain more than one logical ghx
  invocation). Behavioral reductions are NOT automatically attributable
  to CLI changes; the per-row confound notes are load-bearing, and rows
  the miner marked "reduced" mean behavioral reduction in this corpus,
  not proven CLI causation.

The report below is verbatim from the worker.

---

answer

Post-batch corpus: 22 units total: 16 eval episodes from `internal/sidecar/evals/.ghx-evals/runs/gate-run-2026-07-06-persona/` plus 6 live sessions with mtime after `2026-07-06 00:30`, including `spot-a2-gin`. Baseline denominator: 155 units from [CLI-FINDINGS.md](/Users/goga/Documents/goga/ghx/docs/evals/mining-2026-07-06/CLI-FINDINGS.md:16).

| defect signature | baseline | post-batch | verdict | confound notes |
|---|---:|---:|---|---|
| Oversized `ghx explore` | 65 / 155 = 0.419 per unit; 1,371,053 chars | 8 / 22 = 0.364 per unit; 245,038 chars | unchanged | Persona can affect whether `explore` is called, but size is a CLI affordance. Char rate got worse: 8,846 to 11,138 chars/unit. |
| `ghx read` piped to `sed/head/grep` | 501 / 155 = 3.232 per unit: 145 sed, 356 head/grep | 26 / 22 = 1.182 per unit: 14 sed, 12 head/grep | reduced | Strong persona confound: better sidecar behavior can avoid pipes even if CLI ergonomics are unchanged. Still not eliminated. |
| Failed natural range flags | 28 / 155 = 0.181 per unit | 1 / 22 = 0.045 per unit | reduced | Pure CLI affordance when attempted. One post-batch trace still shows `unknown flag: --start`, so not killed. |
| Missing inspect/focus workflow chains | 186 / 155 = 1.200 per unit: 77 map->read, 63 explore->tree/read, 46 search->read | 30 / 22 = 1.364 per unit: 9 map->read, 13 explore->read, 8 search->read | unchanged | Heavily persona/task dependent, but the same multi-command reconnaissance shape persists. |
| Failed `ghx grep` expectation | 9 / 155 = 0.058 per unit | 1 / 22 = 0.045 per unit | unchanged | Pure CLI affordance when attempted. One live trace still says `unknown command "grep" for "ghx"`. |
| Full-file reads >12k chars without narrowing | 22 / 155 = 0.142 per unit | 2 / 22 = 0.091 per unit | reduced | Persona can choose narrower reads; CLI output/defaults still matter. Remaining cases: `hono-base.ts` and `routergroup.go`. |
| Search ergonomics failures/broad output | 17 / 155 = 0.110 per unit: 8 broad, 7 `--lang`, 2 parse fatal | 7 / 22 = 0.318 per unit: 7 broad, 0 `--lang`, 0 parse fatal | unchanged | Mixed result: flag/parse failures disappeared, but broad high-output searches increased materially. Persona/task confound is high. |
| `ghx --version` failures | 30 / 155 = 0.194 per unit | 0 / 22 = 0.000 per unit | insufficient data | No post-batch `ghx --version` attempts found; traces used `ghx version`, so absence of failures does not prove the flag was fixed. |

files changed or inspected

No files modified. Inspected [CLI-FINDINGS.md](/Users/goga/Documents/goga/ghx/docs/evals/mining-2026-07-06/CLI-FINDINGS.md:16), post-batch eval JSONs under `/Users/goga/Documents/goga/ghx/internal/sidecar/evals/.ghx-evals/runs/gate-run-2026-07-06-persona/`, and live session traces under `/Users/goga/.ghx/sessions/`.

commands run

`sed`, `nl`, `find`, `rg`, `jq`, and an inline `python3 -c` read-only aggregator over eval JSON and OTLP JSONL session traces.

test results

No tests run; this was artifact mining only.

evidence snippets or line references

Baseline counts are in [CLI-FINDINGS.md](/Users/goga/Documents/goga/ghx/docs/evals/mining-2026-07-06/CLI-FINDINGS.md:16): explore lines 16-18, read pipes 21-23, range failures 26-28, inspect chains 31-33, grep 36-38, full reads 41-43, search 46-48, version 51-53.

Post-batch examples: range failure remains in [flask-routing_ghx-sidecar_1783349138773.json](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/.ghx-evals/runs/gate-run-2026-07-06-persona/flask-routing_ghx-sidecar_1783349138773.json:117). `ghx grep` failure remains in `/Users/goga/.ghx/sessions/gin-gonic-gin/traces.jsonl:4`.

uncertainty

Counts are trace-level, matching the miner’s caveat: multi-command shell traces can contain more than one logical ghx invocation. Corpus sizes differ sharply, and the persona changed, so behavioral reductions are not automatically attributable to CLI changes.

suggested next step

Treat `--start/--end`, `ghx grep`, compact `explore`, and search output budgeting as still-open ergonomics defects; only `read` pipe usage and large full-file reads show meaningful behavioral reduction in this post-batch corpus.
