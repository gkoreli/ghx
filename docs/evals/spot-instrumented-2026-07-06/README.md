# Spot Check: spot-instrumented-2026-07-06

First INSTRUMENTED spot check of three fresh capabilities, run live on
2026-07-06 from tree `84e7064` (worktree branch
`worktree-agent-a97a7fed9997275bb`). **Episode count: 5 live episodes
total** (founder hard cap: 5) — 2 repo-scoped standard cells + 3
discovery-profile cells of one discovery task. This is a random spot
check per the eval-cadence rule, NOT a citable gate run: every verdict
below is PRELIMINARY by construction (n=1 per cell).

Who scored this: the frozen deterministic scorers in
`internal/sidecar/evals` (`ComputeRewards`, `ComputeDiscoveryRewards`,
`DetectAnomalies`), executed at save time from tree `84e7064`. No LLM
judge. Recompute from the committed episode JSONs in `episodes/` and
`discovery/`.

Identity (both manifests): adapter `@agentclientprotocol/claude-agent-acp@0.55.0`,
subject model `claude-sonnet-5`, wrapper sha256 `30cb9d72…e71b98d`,
`GHX_REPORT_SINK_EXE` set to a fresh `go build` of this tree
(`/tmp/ghx-spot2`), `GHX_EVAL_STRICT=1`, sequential (no parallelism).

## Episodes (5 of 5 allowed)

| # | cell | duration | how invoked |
|---|------|----------|-------------|
| 1 | gin-routing / ghx-sidecar (multi-turn, 2 turns) | 63s | `go test -tags=agent_e2e -run 'TestEpisodes/episodes/gin-routing/ghx-sidecar'` |
| 2 | flask-routing / ghx-sidecar (random other cell) | 58s | `go test -tags=agent_e2e -run 'TestEpisodes/episodes/flask-routing/ghx-sidecar'` |
| 3 | openai-compatible-ai-gateways / plain (discovery) | 81s | throwaway driver (below) |
| 4 | openai-compatible-ai-gateways / ghx (discovery) | 81s | throwaway driver (below) |
| 5 | openai-compatible-ai-gateways / ghx-sidecar (discovery) | 55s | throwaway driver (below) |

Random picks: the multi-turn cell was drawn from
{hono-middleware, gin-routing} via `sort -R` → gin-routing; the second
cell was drawn from the remaining standard tasks → flask-routing.

Discovery driver disclosure: `internal/sidecar/evals` exports
`RunDiscoveryEpisode`/`EvaluateDiscoveryGates`/`SaveDiscoveryVerdict`
but no `agent_e2e` test drives them live yet. Episodes 3–5 were run by
a throwaway orchestration-only driver that calls exactly those frozen
exported functions (zero scoring/detection logic of its own; the
measurement stack is untouched). Its full source is preserved at
`discovery/discovery-spot-driver.go.txt`; the file itself was deleted
after the run and never committed as code. Scores recompute from the
committed episode JSONs with the committed scorers alone. Follow-up: a
proper `agent_e2e` discovery runner test should land before any formal
discovery run.

## Anomaly table

**Zero anomalies of any kind on all 5 episodes** (`anomalies: []` in
every episode JSON; no `invalid`, no `exclusionReasons`). Strict mode
(`GHX_EVAL_STRICT=1`) had nothing to fail. In particular, zero
`trace_capture_gap` — and that is a true zero, not a detector no-op;
see Q1.

Harness finding (not a scored anomaly; taxonomy is closed-world, H4):
during episode 5 the OTel exporter logged
`marshal OTLP trace json: proto: field …AnyValue.string_value contains
invalid UTF-8` twice, and the discovery run's `traces.jsonl` holds 32
`tool.execute` spans where the episode JSONs hold 33 execute traces
(16+13+4; the 1 `tool.other` span is present). Exactly one tool span
was dropped from the OTLP sidecar file by non-UTF-8 bytes in a span
payload. The canonical episode JSONs are complete and unaffected (all
34 toolTraces present). Worth a small sanitizer fix in the OTel
exporter.

## Q1 — ADR-0016.10 raw-SDK audit + first trace_capture_gap reading (TRUST H3)

**Result: detector armed on every turn; zero gaps. First instrumented
H3 reading is a clean zero at n=5 episodes / 6 turns.**

- `rawSDK` is present on all 6 turns (episode 1 has 2 turns), with
  live `messages` per turn of 174, 117, 237, 292, 303, 183 — the audit
  channel was open (ADR-0016.10 D5: detector activates on field
  presence), so the zero-anomaly reading is a real comparator pass,
  not a pre-ADR no-op.
- Raw tool_use blocks == captured toolTraces on every turn:
  6/6 + 3/3 (gin), 10/10 (flask), 13/13 (plain discovery),
  16/16 (ghx discovery), 5/5 (sidecar discovery) — 53/53 total, with
  matching tool_result counts, no truncation, no dropped events.
- D6 provider token usage persisted per turn with
  `usage.source: "result"` everywhere, e.g. gin turn 0:
  input 13, output 2036, cacheRead 65,819, cacheCreate 17,020,
  costUsd 0.1531; discovery ghx turn: input 4,051, output 4,473,
  cacheRead 298,238, costUsd 0.4462. Total provider-reported spend for
  the spot check: $1.36 across 5 episodes. (Inert persistence — no
  scorer reads it; TRUST H5 groundwork.)

## Q2 — ADR-0020.2 steered resume on a multi-turn episode

**Result: resumed turn 2 carried steering. Live-confirmed in an eval
episode (the H8 fix's first eval-context sighting).**

Evidence, all from `episodes/gin-routing_ghx-sidecar_1783355255114.json`
and the run's `traces.jsonl`:

- Turn 1 has `resumed: true` (ACP session id survived across the
  fresh-adapter turn boundary) and `session_recreated: false` on its
  `eval.turn` span.
- The resumed turn recorded **thinking** (468 chars: "I need to
  examine the getValue function in tree.go…") and its `eval.turn` span
  carries `gen_ai.usage.reasoning.output_tokens: 468`. Under the
  adapter default (`thinking.display: omitted`) thought chunks never
  reach ACP — thinking on a resumed turn is only possible if
  LoadSession carried the `thinking.display: summarized` budget from
  the session-options meta (ADR-0020.2 D1).
- Tool activity on the resumed turn respected the allowlist: two
  Bash-executed `ghx read gin-gonic/gin tree.go --lines …` calls plus
  the auto-approved `mcp__ghx-report-sink__submit_report`. Nothing
  outside `[Bash, Read]` + submit_report.
- Bonus: G4 memory PASS — resume rate 1.00, and the raw-SDK audit was
  live on the resumed turn (117 raw messages), confirming ADR-0020.2
  did not clobber the ADR-0016.10 audit channel.

Standard-cell scores for the record (n=1 each, PRELIMINARY):
gin-routing sidecar correctness 1.0 / evidence 0.667 / trajectory 1.0 /
compression 0.860 / memory 1.0 / safety 1.0; flask-routing sidecar
correctness 1.0 / evidence 0.889 / trajectory 0.933 / compression
0.905 / safety 1.0. See `episodes/verdict.md` for the (insufficient-
sample) gate rendering.

## Q3 — first-ever discovery-class episodes (ADR-0019.2)

**Result: the discovery runner, scorer, and separate D-G gate section
all worked live, and the scorer's teeth showed immediately.** Verdict:
`PRELIMINARY / DISCOVERY NOT SUPPORTED` at n=1 per cell
(`discovery/discovery-verdict.md`) — expected and honest for a spot
check; no D-G number here is citable.

| profile | verifiedRecall | namedRecall | evidence | inferenceHonesty | familiarityGap | compression | safety | main-agent chars |
|---|---|---|---|---|---|---|---|---|
| plain | 0.000 | 0.667 | 0.500 | 1.000 | 0.667 | — | 1.000 | 98,034 |
| ghx | 0.000 | 0.667 | 0.500 | 1.000 | 0.667 | — | 1.000 | 156,894 |
| ghx-sidecar | 0.000 | 0.667 | 0.500 | **0.000** | 0.667 | 0.904 | 1.000 | 4,570 |

Findings (this is exactly the split ADR-0019.2 D2/D3 was designed to
expose):

1. **verifiedRecall 0 across all profiles is agent behavior, not a
   scorer bug.** All three subjects named 2/3 targets (litellm,
   Portkey gateway; every profile missed Helicone — consistent with
   the ADR's "likeliest miss" note). But none emitted a single
   `owner/repo:path` citation despite the preamble/persona requiring
   it — grep over all answer text finds zero colon citations. The ghx
   profile actually read target evidence (8× `ghx read … README.md`,
   1× `ghx explore Portkey-AI/gateway` in its tool trail) yet earned
   no verified credit because D2 condition 2 (cited read path) failed.
   The familiarity gap (0.667 = namedRecall − verifiedRecall) is doing
   its job: names recalled, verification not evidenced.
2. **inferenceHonesty 0 on the sidecar is a genuine catch:** the
   sidecar report listed 4 repos under `verified` whose evidence was
   only `ghx repos` search listings (its own uncertainty section even
   admits "ranking based on repo/search metadata only"). Search-only
   = inferred (ADR-0019.1 D3); labeling it verified is the exact
   dishonesty D-G3 penalizes. The plain/ghx profiles labeled their
   unread tails "inferred"/"not verified" and kept honesty 1.0.
3. **D-G5 compression PASS even here:** sidecar main-agent cost 4,570
   chars vs ghx 156,894 (threshold 54,913). D-G6 safety PASS.
4. Subject-behavior implication for a future formal discovery run:
   with sonnet-5 subjects, the binding constraint on D-G1 is citation
   discipline + read-then-cite behavior, not repo discovery. The
   sidecar discovery persona may need a stronger "verify by reading
   files and cite owner/repo:path" push before a gate run is worth
   its ~60 episodes.

## Recompute

```
episodes/  — 2 repo-scoped episode JSONs + manifest + verdict.{json,md}
discovery/ — 3 discovery episode JSONs + manifest + discovery-verdict.{json,md}
           + discovery-spot-driver.go.txt (orchestration source, disclosure)
```

Rewards/anomalies re-derive with the committed scorers from tree
`84e7064` (`ComputeRewards`, `ComputeDiscoveryRewards`,
`DetectAnomalies`, `EvaluateDiscoveryGates`) over these files. Raw run
dirs (with `traces.jsonl`/`metrics.jsonl`/`logs.jsonl`) live untracked
at `.ghx-evals/runs/spot-instrumented-2026-07-06{,-discovery}/`.
