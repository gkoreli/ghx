# Independent Eval Framework Audit — provenance

Auditor: OpenAI Codex (gpt-5.5-class) via `codex exec -s read-only`,
2026-07-05 — deliberately a different model family from the framework's
authors (Claude/Fable), per the anti-self-preference principle of
ADR-0023.1. Mandate: adversarial, recompute-everything, evidence-only.
Subject: internal/sidecar/evals + the committed 90-episode run
docs/evals/gate-run-2026-07-05-confirmatory/. Verbatim report below.

---

# Independent Eval Framework Audit

## 1. Independent Recompution

SEVERITY note: Exact numeric recomputation matched the committed verdict for rewards, aggregates, and gates.

I could not write throwaway scripts under `/tmp` because the sandbox rejected all temp writes, so I ran an inline independent Python recomputation with no repo imports and no repo modifications.

Recomputed from episode JSONs + task fixtures:

- Episode rewards: 90/90 exact match, mismatch count `0`.
- Per-profile aggregates: exact match to [verdict.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/verdict.json:2).
- Gates G1-G5: exact match, all pass as recorded in [verdict.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/verdict.json:46).
- Run anomaly counts: exact match to [verdict.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/verdict.json:78).

Diff summary: `reward_mismatch_count=0`, `aggregate_mismatch_count=0`, `gate_mismatches=[]`.

## 2. Formula vs Documentation

SEVERITY minor: Direct-profile evidence scoring is implemented but under-documented.

ADR-0016.1 defines evidence in report terms only: verified claim evidence, `commandsRun`, relevant-file reasons ([ADR](/Users/goga/Documents/goga/ghx/docs/adr/0016.1-sidecar-eval-implementation.md:119)). Code adds a direct-profile fallback: tool usage plus at least one path token when `ep.Report == nil` ([rewards.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/rewards.go:133)). This is reasonable, but the ADR table does not spell it out.

SEVERITY note: ADR-0016.2 correctly upgrades evidence from “non-empty evidence” to citation-bearing evidence. Code matches the hardened rule: `citesEvidence` requires a path-like token or `:line` reference ([rewards.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/rewards.go:109), [rewards.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/rewards.go:285)); ADR-0016.2 says verified evidence must cite inspectable code ([ADR](/Users/goga/Documents/goga/ghx/docs/adr/0016.2-benchmark-validity-hardening.md:99)).

SEVERITY minor: The sample sufficiency implementation omits ADR-0016.1’s “≥ 3 repos” condition.

ADR requires “≥ 6 tasks across ≥ 3 repos” ([ADR](/Users/goga/Documents/goga/ghx/docs/adr/0016.1-sidecar-eval-implementation.md:128)). `sampleSufficiency` checks distinct tasks, min trials, and multi-turn tasks only; it does not count repos ([gates.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/gates.go:325)). This run appears not harmed, but the code would label a 6-task/1-repo run sufficient.

SEVERITY note: SPT code matches ADR-0016.6 math.

ADR defines `signal = correctness × evidence`, chars/4 tokens, per-1000 scaling ([ADR](/Users/goga/Documents/goga/ghx/docs/adr/0016.6-signal-per-token.md:33)). Code implements exactly that ([spt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/spt.go:32)) and aggregates by summing signal and denominators first ([spt.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/spt.go:48)).

## 3. Contamination Vectors

SEVERITY major: Three `ghx-mapengine` plain-profile episodes read answer-bearing ADR documentation inside the target repo.

Evidence:
- [ghx-mapengine_plain_1783293965871.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783293965871.json:20) reads `docs/adr/0013-ghx-map-command.md`.
- [ghx-mapengine_plain_1783296331466.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783296331466.json:20) reads the same ADR.
- [ghx-mapengine_plain_1783297517822.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783297517822.json:23) reads the same ADR.

This is not the eval ground-truth file, but it is an answer-bearing design doc for the task. It contaminates the plain baseline, not the sidecar.

SEVERITY note: I found no prompt leakage of task check strings into episode questions. Independent scan count: `0`.

SEVERITY note: I found no `ghx-mapengine` tool reads of `internal/sidecar/evals/testdata`, `testdata/tasks`, `docs/evals`, or ADR-0016 eval docs. The contamination found was `docs/adr/0013`.

SEVERITY note: Replay decontamination appears structurally applied. Multi-turn sidecar episodes contain nonzero `replayedToolTraces` on follow-up turns while live `toolTraces` remain separate; ADR-0016.5 requires replayed traces be excluded from repeat-read/accounting ([ADR](/Users/goga/Documents/goga/ghx/docs/adr/0016.5-replay-decontamination-repeat-read-refinement.md:43)), and `TurnRecord` documents replay fields as excluded ([episode.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/episode.go:95)).

SEVERITY note: No evidence found of host `AGENTS.md` / `CLAUDE.md` instruction content affecting agent answers. Some plain episode outputs list `AGENTS.md`/`CLAUDE.md` as repository files, but my scan found no “North Star”, “Evidence Contract”, or ADR-driven behavior in answers/tool commands.

## 4. Aggregation Soundness

SEVERITY note: Current committed run is balanced: 6 tasks × 3 profiles × 5 trials = 90 episodes, matching README claim ([README](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/README.md:3)). Verdict aggregates show 30 per profile ([verdict.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/verdict.json:3)).

SEVERITY minor: Unbalanced-cell warning logic exists but only warns; aggregation remains episode-weighted. Code computes profile means over episodes ([gates.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/gates.go:82)) and only emits a data-quality warning for unequal counts ([gates.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/gates.go:409)). That matches current docs’ warning posture, but does not implement mean-of-cells weighting.

SEVERITY major: `thesisSupported` ignores `DataSufficient`.

`EvaluateGates` sets `ThesisSupported = valid && G1 && G2 && G3 && G5` ([gates.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/gates.go:230)), not requiring `DataSufficient`. ADR-0016.2 says below-minimum verdicts must be PRELIMINARY ([ADR](/Users/goga/Documents/goga/ghx/docs/adr/0016.2-benchmark-validity-hardening.md:108)). Markdown may self-label via report code, but JSON can still have `thesisSupported: true` on insufficient data.

## 5. Anomaly Detector Honesty

SEVERITY major: Two episode JSONs omit anomalies that the detector derives from their own fields.

Detector rule: sidecar report answer starting `WARN: sidecar did not emit` creates `sidecar_report_missing`, and raw text containing `<ghx-report>` also creates `sidecar_report_block_unparsed` ([anomalies.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/anomalies.go:92)).

Missing per-episode records:
- [gin-routing_ghx-sidecar_1783294237204.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783294237204.json:141) has a WARN report, but no stored `anomalies`.
- [hono-middleware_ghx-sidecar_1783296851961.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783296851961.json:225) same issue.

Run-level anomaly counts are still correct because `CountAnomalies` recomputes from episode fields ([anomalies.go](/Users/goga/Documents/goga/ghx/internal/sidecar/evals/anomalies.go:129)); [verdict.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/verdict.json:78) matches recomputation exactly.

## 6. Artifact Integrity

SEVERITY major: Manifest is stale or wrong.

[manifest.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/manifest.json:2) says `expectedEpisodes: 18`, but the committed run contains 90 episode JSONs and README claims 90 episodes ([README](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/README.md:3)). This does not affect recomputed scores, but it is a provenance defect.

SEVERITY note: Identity is consistent across all 90 episodes. The manifest identity matches the episode identity shape, including `claude-sonnet-5` and wrapper SHA ([manifest.json](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/manifest.json:4)).

SEVERITY note: No committed `traces.jsonl` exists in `docs/evals/gate-run-2026-07-05-confirmatory/`, despite README saying “the run dir’s local traces.jsonl” can be replayed ([README](/Users/goga/Documents/goga/ghx/docs/evals/gate-run-2026-07-05-confirmatory/README.md:80)). The in-flight run’s `traces.jsonl` spot-check has OTLP JSON hex IDs: `traceId` 32 lowercase hex, `spanId`/`parentSpanId` 16 lowercase hex, `bad=0`.

## Verdict on Trustworthiness

Proven recomputable: per-episode rewards, profile aggregates, G1-G5 pass/fail outcomes, thesis verdict under current gate code, and run-level anomaly counts all recompute exactly from committed episode artifacts.

Not fully trustworthy as an audit record: manifest identity/count metadata is stale; two episode files lack anomaly fields that the detector would derive; the sample-sufficiency code omits the ADR’s ≥3 repo rule; and the `ghx-mapengine` plain baseline has answer-bearing doc contamination from `docs/adr/0013`.

Top risks:
1. Artifact integrity/provenance: stale manifest and missing committed traces weaken independent auditability.
2. Contamination: target-repo design docs can leak answers for self-referential ghx tasks.
3. Sufficiency semantics: `thesisSupported` and `sampleSufficiency` can overstate below-contract runs unless report rendering compensates outside JSON.

Commands run: `rg --files`, `nl -ba`, `jq`, `find`, `python3 -c` independent recomputation, trace hex-id `awk` check. No repo files modified.
