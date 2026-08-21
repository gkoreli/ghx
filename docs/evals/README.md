# Evals

Deterministic, artifact-backed evaluation of the sidecar thesis. Gates,
rewards, and validity rules are pre-registered in `docs/adr/0016.*`; verdicts
self-label their limitations (PRELIMINARY, data-sufficiency notes) per the
AGENTS.md truthfulness tenet.

## Running episodes

```bash
GHX_EVAL_AGENT="$(pwd)/scripts/eval-agent-acp.sh" \
  go test ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes -v
```

- `GHX_EVAL_AGENT` — ACP agent command under test (falls back to
  `~/.ghx-sidecar/config.json`, then the built-in default).
- `GHX_EVAL_RUN_DIR` — accumulate multiple invocations into one run dir so a
  single verdict covers the whole sample.
- Every episode artifact records what ran via `AgentIdentity`
  (agentCommand, adapter name/version, subject model, wrapper SHA256).

## Expensive-backend guardrail (ADR-0038)

Formal gate runs **fail fast** when the resolved agent matches an
expensive/shared-quota backend (default list: `claude*`), including detection
via wrapper-script content — the pinned wrapper's argv0 hides the real
backend. This exists because the 2026-07-03/04 formal run silently burned the
main agent's Claude subscription through `claude-agent-acp`.

Deliberate override for a specific run:

```bash
GHX_EVAL_ALLOW_EXPENSIVE_BACKEND=formal-run   # value must be exactly this
```

Operators can reclassify backends without code changes:

```bash
GHX_EVAL_EXPENSIVE_BACKENDS="claude,gpt5-worker"   # comma-separated
```

## Truthfulness rules that bind eval work

- Negative and interim results are committed and kept
  (`gate-run-2026-07/`), never rerun-until-green.
- The measurement stack is frozen during a run; scorer changes require a
  pre-registered ADR before any rescore.
- Deterministic gates measure exactly what they measure; quality judgments
  belong to the calibrated judge layer (AGENTS.md).
