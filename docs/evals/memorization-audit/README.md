# Memorization Confound Audit — closed-book probe (ADR-0016.9)

Run: `memorization-audit-2026-07-06` · Subject: `claude-sonnet-5` via
`scripts/eval-agent-acp.sh` (claude-agent-acp@0.55.0, wrapper sha256
`30cb9d72…`) · n = 5 trials per task · Open-book reference:
`docs/evals/gate-run-2026-07-06-fixbatch` (committed run, gate-style
validity filter applied).

Who scored this: the unchanged deterministic checks
(`internal/sidecar/evals/rewards.go` `ComputeRewards`, scoring commit
`c086d38`) over the committed episode JSONs in `episodes/`. No judge, no
LLM scoring, no scorer/detector modifications. Recompute any number from
the artifacts: every episode carries its `checks` snapshot and full turn
text.

## Exact command

```sh
GHX_REPORT_SINK_EXE=/tmp/ghx-closedbook \
GHX_EVAL_AGENT="$(pwd)/scripts/eval-agent-acp.sh" \
GHX_EVAL_SUBJECT_MODEL=claude-sonnet-5 \
GHX_EVAL_CLOSEDBOOK_DIR="$(pwd)/docs/evals/memorization-audit" \
GHX_EVAL_OPENBOOK_RUN_DIR="$(pwd)/docs/evals/gate-run-2026-07-06-fixbatch" \
go test ./internal/sidecar/evals -tags=agent_e2e -run TestClosedBookProbe -v -timeout 90m
```

(`GHX_REPORT_SINK_EXE` only satisfies preflight; no report sink is
registered on closed-book sessions.)

Session steering proof (recorded verbatim in `manifest.json`): the ACP
session was created with explicit `"tools":[]` and `"allowedTools":[]`,
`settingSources:[]`, `strictMcpConfig:true`, thinking disabled, model
pinned. All 30 episodes recorded **zero tool traces, zero tool calls,
zero violations** — verified over the committed artifacts:

```sh
jq -s '[.[] | ((.violations//[])|length) + ([.turns[].toolTraces//[]|length]|add)] | add' episodes/*.json  # → 0
```

## Results (pre-registered metrics, ADR-0016.9)

| Task | Closed-book mean | Median | Max | SD | Open-book mean (pooled) | Exploration signal | Label |
|---|---:|---:|---:|---:|---:|---:|---|
| express-router-location | 0.000 | 0.000 | 0.000 | 0.000 | 0.800 | **0.800** | exploration-bearing |
| flask-routing | 0.400 | 0.500 | 0.500 | 0.200 | 1.000 | **0.600** | (none — see below) |
| ghx-mapengine | 0.000 | 0.000 | 0.000 | 0.000 | 0.838 | **0.838** | exploration-bearing |
| gin-routing | 0.000 | 0.000 | 0.000 | 0.000 | 1.000 | **1.000** | exploration-bearing |
| hono-middleware | 0.000 | 0.000 | 0.000 | 0.000 | 1.000 | **1.000** | exploration-bearing |
| openai-node-streaming | 0.000 | 0.000 | 0.000 | 0.000 | 0.917 | **0.917** | exploration-bearing |

Exploration signal = open-book pooled mean − closed-book mean
(= memorization-adjusted correctness). Per-profile open-book means and
per-trial scores are in `closed-book-summary.json`. Open-book means use
the same episode-validity filter as the gate verdict (invalid /
compliance / contamination episodes excluded), which is why
`ghx-mapengine` differs from the raw per-file average.

### Flag verdict

**No task is flagged recall-dominated, low-exploration-signal, or
memorization-risk.** Five of six tasks label exploration-bearing
(closed-book mean < 0.40 and gap ≥ 0.30). `flask-routing` lands in no
pre-registered bin: its mean is exactly 0.40 (the exploration-bearing
threshold is strict `< 0.40`) with max 0.50 — it shows the strongest
partial recall in the corpus and is the first candidate for a harder
variant in the next corpus revision, but under the pre-registered table
it carries no flag and requires no action.

## Interpretation — read before citing

1. **These closed-book scores are a lower bound on memorization, not a
   recall ceiling.** Per ADR-0016.9 the probe sends *only the exact eval
   question text* — and the questions (except flask's) do not name the
   target repository; open-book profiles receive the repo identity via
   the direct-profile preamble or `sidecar.Ask(Repo:…)`. Faced with
   "Where are routes registered…?" and an empty cwd, the subject model
   overwhelmingly asked *which* repo it should look at instead of
   guessing one (see any `gin-routing` or `express` episode). So part of
   each measured gap is repo-identity context, not exploration per se.
2. **Where the question itself identifies the framework, recall is real.**
   `flask-routing` names `route()` decorator / URL rules; the model
   recalled `add_url_rule` (the expected symbol) in 4/5 trials from
   weights alone, scoring 0.5 each time. It did *not* recall the current
   `src/flask/sansio/*` file layout — its path guesses
   (`flask/scaffold.py`, `flask/app.py`) match a pre-2.2 layout and were
   rejected by the deterministic file checks, which is exactly the
   freshness behavior ADR-0016.9 says to record: stale-weights answers
   fail path checks even when concept recall succeeds.
3. **Honesty observation (positive):** in 30/30 trials the model declined
   to fabricate rather than hallucinating paths for an unnamed repo. No
   unacceptable-claim zeroing fired anywhere (including express's
   `lib/router/index.js` trap).
4. **What this closes and what it doesn't.** H2's worst-case reading —
   "the ~0.90 plain baseline could be pure recall-from-weights" — is not
   supported for the corpus *as asked*: with the exact eval prompts and
   no tools, the subject scores 0.07 mean correctness corpus-wide vs
   ~0.92 open-book. It does **not** rule out that a repo-named
   closed-book variant would score higher on famous repos (flask's 0.4
   floor suggests it would); that variant needs its own pre-registered
   ADR before running, per the measurement-freeze rule.

## Corpus-level numbers

- Closed-book corpus mean correctness: **0.067** (30 trials)
- Open-book corpus mean (pooled over profiles, reference run): **0.926**
- Memorization-adjusted corpus correctness: **0.859**

## Layout

- `manifest.json` — run identity, git/scoring commits, task-corpus hash,
  wrapper hash, exact session-meta JSON (tools/allowedTools proof)
- `closed-book-summary.json` — per-task stats, per-trial scores,
  per-profile open-book references, exploration signals, labels
- `episodes/*.json` — 30 full episode artifacts (question, answer text,
  checks snapshot, rewards); plus `traces.jsonl` / `metrics.jsonl`
  emitted by the standard artifact writer
