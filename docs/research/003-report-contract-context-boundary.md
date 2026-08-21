# Research 003: Compact report contract and context-boundary economics

**Capability candidate:** compact-report contract — the workflow boundary where
internal exploration stays cheap/internal and only a structured evidence report
crosses back to the expensive main agent.

**Workstream:** NORTH_STAR workflow boundary; M4 verdict (THESIS SUPPORTED,
2026-07-05) — per AGENTS.md, that verdict is a conservative floor, never
quoted as a ceiling.

**Status:** research artifact / pre-proposal. Read-only analysis grounded in
committed code and eval artifacts. Proposes, does not implement. No existing
file modified. Repo state at authoring: branch `mainline`.

## 1. Problem statement

The sidecar thesis is a compression claim: internal exploration may be large,
but only a small structured report crosses back to the main agent. The report
contract (`internal/sidecar/prompt.go:161-166`) already says: answer first,
at most 2 sentences, whole report under 2000 characters, at most 5 relevant
files, one line per evidence entry, never paste file contents — cite path,
symbol, line. "The report replaces the transcript; it must be cheaper to read
than redoing the exploration."

Two gaps separate today's contract from a framework-grade boundary:

1. No machine validation of persona bounds. A >2000-char or >5-file report
   silently passes today; nothing distinguishes "compact because compliant"
   from "compact because truncated".
2. No schema version. Consumers cannot detect contract drift. Precedent:
   ADR-0031.2 changed `nextReads` semantics with no version marker to key off.

## 2. Evidence (what exists today)

### 2.1 The report schema is typed and rich

`internal/sidecar/report.go:38-56` — `Report{Answer, Verified, Inferred,
Unverified, RelevantFiles, Evidence, TierUsed, BackendsUsed, CommandsRun,
Uncertainty, NextReads}`. Claims are honesty-partitioned: verified vs inferred
vs unverified are separate fields (`report.go:19-20`); relevant files carry
reasons (`report.go:25-26`).

### 2.2 The boundary is modeled in the eval kernel

`internal/sidecar/evals/episode.go:105-109` — `ContextAccounting` records
`MainAgentChars` vs total; "for direct profiles main == total by construction".
This makes the ADR-0016.1 workflow boundary measurable.

### 2.3 G3 measures compression deterministically

`internal/sidecar/evals/gates.go:20-21` — sidecar main-agent chars must be
<= 0.35 x ghx main-agent chars (`g3Factor = 0.35`). `gates.go:371-385`
(`buildG3`) computes this over profile aggregates with optional `at_least(n)`
overrides.

Measured numbers from committed artifacts:

| Run | G3 result | Notes |
|---|---|---|
| Smoke pair 2026-07-03 (`docs/evals/2026-07-03-smoke-pair/`) | ratio 0.127 (31,432 -> 4,006 chars) | n=1 calibration pair |
| Preliminary r2 run 2026-07-04 (72/90 episodes, halted) | ratio 0.048 (46,252 -> 2,659 chars) | dataSufficient=false; kept per truthfulness rules |

Correctness did not degrade under compression so far: smoke G1 = 1.0 on both
profiles; r2 preliminary G1 sidecar 0.920 vs ghx 0.913 (floor 0.90x relative).

### 2.4 Contract evolution already happened once — without versioning

ADR-0031.2 (accepted 2026-07-06) revised the `nextReads` contract to concrete
paths with a lenient/visible normalizer. No version marker existed to key the
change off; comparability was preserved only by convention.

## 3. Cross-references

- `docs/NORTH_STAR.md` — workflow boundary is a named workstream; M4 verdict
  framing governs how these numbers may be quoted.
- ADR-0016.1 — gate definitions incl. G3 threshold provenance.
- ADR-0016.2 — validity hardening: word-boundary symbol matching, evidence
  citation floor, PRELIMINARY labeling, gaming-resistance tests.
- ADR-0029.2 — Tier-2 doctrine + discovery citation discipline; citations are
  persona doctrine today, not machine-checked contract.
- ADR-0031.2 — nextReads concrete-paths revision; the drift this artifact
  wants versioning to make detectable.
- ADR-0032.2 — unified ToolCallTrace + Locations capture flow; trace fidelity
  complements report fidelity at the boundary.
- ADR-0034 — failure-class model; the natural taxonomy for classifying
  bound-violation episodes.
- Historical note: `packages/ghx-bench/` (TS bench with MainAgentBurden metric
  and an LLM judge penalizing vague reports) existed during early ADR-0016.1
  design but is no longer in the tree; it survives only as a reference inside
  ADR-0016.1. Its ideas (burden metric, judge-scored actionability) are
  absorbed by the current Go kernel and the AGENTS.md judge mandate.

## 4. Rationale

The compression result is real on the deterministic layer, but "the main agent
can act on the report without re-exploring" is a stronger claim than char-ratio
can carry alone. Three things keep the claim honest:

1. Bounds must be enforced or violations counted — otherwise G3 rewards an
   agent that ignores its own contract.
2. The schema must be versioned — the contract has already changed once
   (nextReads) and will change again as the framework standardizes.
3. Char-ratio must be labeled a proxy — real main-agent token burden depends
   on tokenizer and consumer harness; per AGENTS.md, deterministic checks are
   frozen baselines that measure exactly what they measure, nothing more.

## 5. Options considered

| Option | What | Pros | Cons | Verdict |
|---|---|---|---|---|
| A. Status quo | Persona bounds only | Zero code | Silent violations; no drift detection | Insufficient |
| B. Report validator gate | Post-parse validation of bounds (<2000 chars, <=5 files, answer <=2 sentences); violations recorded, report flagged non-compliant | Small, testable, honest; feeds gates as data | Needs failure-class taxonomy (ADR-0034) to avoid new noise | Recommended core |
| C. Versioned schema field | Add `schemaVersion` to Report JSON | Cheap; enables evolution like 0031.2 without ambiguity | Touches producers/consumers; needs compat window | Recommended, trivial |
| D. Token-based accounting | Replace char counts with tokenizer estimates | Closer to real burden | Non-deterministic across tokenizers; violates recomputability unless estimator is committed+versioned | Defer; document as caveat |
| E. Main-agent-in-the-loop harness | Real consumer agent acts on report; measure downstream task success | Measures the actual claim | Expensive; overlaps judge mandate and host-task evals (ADR-0032) | Defer to NORTH_STAR C8/H-series track |
| F. Judge-scored report quality | LLM judge grades actionability | Sees what substring gates cannot | Must follow full judge-calibration protocol (kappa, FP/FN rates, committed prompt/model) | Align with existing judge track, not a new one |

## 6. Recommendation (pre-registration sketch)

Minimal ADR-first sequence consistent with repo style:

1. ADR proposal: add `schemaVersion` to `Report` (Option C) plus a
   `ValidateReportBounds` check invoked after parse in the runtime; violations
   recorded in the episode artifact and classified via ADR-0034 failure
   classes — never silently dropped.
2. Tests: unit tests for bounds validation (over/under cases); golden fixture
   update. No rescore of historical runs (measurement-stack freeze per
   AGENTS.md): this adds metadata, it does not alter any scorer.
3. Separately (not in the same ADR): token-based accounting (D),
   main-agent-in-the-loop (E), and judge-scored report quality (F) each get
   their own pre-registered decision when their track matures.

## 7. Open questions

1. Should bound violations fail an episode or only flag it? Flagging preserves
   data; failing changes gate denominators — decide in the ADR.
2. Does `schemaVersion` bump on every contract change or only breaking ones?
   Precedent 0031.2 suggests: version on semantic change, normalizer stays
   lenient/visible.
3. Where does validation live: runtime (`internal/sidecar`) or eval kernel?
   Runtime is the product truth; the kernel should consume its output.
