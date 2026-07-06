# Anticipation Predictor D1 — 2026-07

Deterministic offline miner for ADR-0031.1 D1. No live episodes and no LLM calls are used; the miner reads committed eval artifacts plus local `~/.ghx/sessions` traces.

## Matching Rule

Paths are normalized by trimming quotes/backticks, line suffixes, shell punctuation, leading ./, repo prefixes, and owner/repo: prefixes, then converting to slash paths. A nextReads item is one prediction entry; the miner extracts path-like candidates from that entry. Candidates with a file extension, or a single path token such as tree.go, are exact-file candidates and match only the same normalized path. Candidates without a file extension but containing a slash, single-token directory names such as docs, or entries whose wording names a directory/area, are area candidates and match an actual read when the actual normalized path is equal to the area or has area/ as a prefix. If one nextReads entry contains several candidates joined by words such as or/and, the entry is a hit when any candidate matches; precision denominator remains the original nextReads entry count. Recall denominator is the count of distinct files actually read in turn N+1.

## Corpus Summary

| corpus | units | multi-turn units | pairs | seeded pairs | actual-read pairs | micro recall | micro precision | seeded micro recall | seeded micro precision | D1 gate |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| committed-evals | 101 | 30 | 30 | 4 | 18 | 0.071 | 0.667 | 0.286 | 0.667 | FAIL |
| local-dogfood | 25 | 4 | 5 | 0 | 5 | 0.000 | 0.000 | 0.000 | 0.000 | FAIL |

## Committed evals per task

| task/session | pairs | seeded pairs | predictions | actual reads | recall hits | precision hits | micro recall | micro precision |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| gin-routing | 15 | 2 | 4 | 9 | 2 | 4 | 0.222 | 1.000 |
| hono-middleware | 15 | 2 | 2 | 19 | 0 | 0 | 0.000 | 0.000 |

## Local dogfood per session

| task/session | pairs | seeded pairs | predictions | actual reads | recall hits | precision hits | micro recall | micro precision |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| crewaiinc-crewai | 1 | 0 | 0 | 8 | 0 | 0 | 0.000 | 0.000 |
| gin-gonic-gin | 2 | 0 | 0 | 2 | 0 | 0 | 0.000 | 0.000 |
| honojs-hono | 1 | 0 | 0 | 14 | 0 | 0 | 0.000 | 0.000 |
| ukgovernmentbeis-inspect-ai | 1 | 0 | 0 | 12 | 0 | 0 | 0.000 | 0.000 |

## Distribution Notes

- committed-evals: top hit task/session `gin-routing` accounts for 100.0% of recall hits (2 total hits), so concentration is high.
- local-dogfood: no recall hits; hits are not concentrated because there are none. Seed sparsity dominates this corpus (0/5 pairs seeded).

## D1 Gate Verdict

- committed-evals: FAIL at micro recall 0.071 against threshold 0.30.
- local-dogfood: FAIL at micro recall 0.000 against threshold 0.30.

Caveat: the committed eval corpus has only 30 consecutive pairs and only 4 seeded pairs; the local dogfood corpus has only 5 consecutive pairs and 0 seeded pairs. These are honest availability measurements for D1, not stable behavioral estimates for all future sidecar sessions.

## Raw Data

Raw per-pair measurements are in `pairs.jsonl` in this directory.
