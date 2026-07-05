# Live smoke pair — 2026-07-03 (post-accounting-fix)

**Scope: 1 task × 1 trial × 2 profiles (`ghx`, `ghx-sidecar`), NOT the
ADR-0016.1 gate run.** Purpose: validate the two pre-gate-run follow-ups
from the first smoke (`docs/evals/2026-07-03-smoke/`) — tool-output
accounting and the compact-report prompt contract — before spending the
formal ~90-episode gate run. Task `hono-middleware` (2 turns), agent =
`@agentclientprotocol/claude-agent-acp` (Claude Code backend), produced by
`go test ./internal/sidecar/evals -tags=agent_e2e -run
'TestEpisodes/hono-middleware/(ghx|ghx-sidecar)$'`. The `plain` profile was
deliberately skipped — neither fix affects it and it does not enter G3.

## Result: both fixes verified live; G3 flips from FAIL to PASS

| measure | first smoke (pre-fix) | this pair (post-fix) |
|---------|----------------------|----------------------|
| ghx main-agent chars | 8,017 (produced text only) | 31,432 (text + 24,530 tool-output chars) |
| sidecar main-agent chars | 7,668 (unbounded reports) | 4,006 (2,006 per turn, ~2000-char contract) |
| G3 ratio (threshold 0.35) | 0.96 — FAIL | **0.127 — PASS** |

- **Accounting fix confirmed**: `toolOutputChars` is populated on every
  turn of both profiles (ghx: 9,426 + 15,104; sidecar-internal: 7,975 +
  17,582), so direct-profile burden now includes the file contents and
  search results the agent actually consumed.
- **Compact-report contract confirmed**: each turn's report JSON is
  2,006 chars against the 2000-char persona instruction (soft constraint,
  honored within noise), down from 7,668 for 2 turns pre-fix — with
  correctness and evidence still 1.0.
- G1/G2/G4/G5 pass as before (correctness 1.0 both profiles, evidence
  1.0, resume rate 1.00, safety 1.0).

## Caveats

- n=1 on one easy calibration task; the `verdict.md` "THESIS SUPPORTED"
  line applies the gate rules to this insufficient sample and is retained
  verbatim for honesty — it is **not** the project verdict. The formal
  gate run (≥ 6 tasks × 5 trials × 3 profiles) decides.
- Tool-output accounting depends on the adapter reporting content in
  `tool_call`/`tool_call_update`; adapters that omit it would undercount
  direct profiles again. The Claude Code adapter reports it.
- Both fixes move the metric in the sidecar's favor by construction; the
  reason they are legitimate is that the first was correcting a bias
  *against* the baseline's true cost, and the second is a real product
  change to the shipped persona contract, landed before (not after) the
  measured run.

Files: two episode artifacts plus `verdict.json` / `verdict.md` generated
by `evals.EvaluateGates`.

## Known capture gap in these artifacts (fixed before the gate run)

Same title-only `toolCalls` limitation as `2026-07-03-smoke` (see that
README): literal commands were not yet captured from ACP
`rawInput`/`tool_call_update`. Fixed via `ToolCallTrace` before the gate
run; kept verbatim as honest history.
