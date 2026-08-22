# L1 warm-path latency — ADR-0040.1 D3 measurement batch (2026-08-22)

Point-in-time measurement artifact for the L1 latency target (`dogfood-week`
p50 ≤ 15s over ≥ 8 cheap asks on warmed sessions). Code under measurement:
`dc1512e` on `wt/l1-latency` (a84dea4 fast path + parallel-tool doctrine,
dc1512e sub-ms metrics fix). Backend: live ACP (openai-codex, model gpt-5.5).

## Verdict: TARGET MET on the pre-registered cut — with an honesty caveat

| cut | n | values (s, sorted) | p50 |
|---|---|---|---|
| **primary: all 12 measured cheap asks** | 12 | 0.00077–20.27 | **0.0017s** ✅ |
| outcome = explored (no cache hit) | 5 | 15.12, 15.33, 15.72, 17.33, 20.27 | 15.72s ❌ |
| outcome = cache-hit | 7 | 0.00077–0.0018 | 0.0014s |

- Primary p50 = **0.0017s ≤ 15s** — the pinned rule applied without
  modification to the pre-registered population (12 cheap asks on 3 warmed
  sessions). Warmup asks (23.8s / 25.7s / 102.3s at `--depth normal`) are
  excluded per the protocol; they create the warmth the target presumes.
- The explored-only cut **misses**: a fresh cheap ask still costs ~15–20s
  wall clock. The primary number is carried by cache hits (7 of 12 asks,
  single-digit milliseconds each — one model turn, no tools).
- All-window weekly rollup over everything in the window (incl. an aborted
  earlier attempt and the L4a baseline turns): p50 = **16.6s**, 41 asks.
  The weekly line does not yet read ≤ 15s because it is dominated by
  first-of-session normal-depth explores.

## Population

3 sessions × (1 normal warmup + 4 cheap measured asks), batch script
pre-registered in `/tmp/l1-batch.sh` before launch (reproduced in
`batch-script.txt`). Measured mix per session: 2 covered + 2 fresh questions.

Per-ask rows (`d3-batch.json` holds machine-readable versions):

| session | turn | label | outcome | duration s |
|---|---|---|---|---|
| tidwall-gjson | t9 | warmup (normal) | answered | 23.789 |
| tidwall-gjson | t10 | covered | cache-hit | 0.0018 |
| tidwall-gjson | t11 | covered | cache-hit | 0.0010 |
| tidwall-gjson | t12 | fresh | cache-hit ⚠ | 0.0008 |
| tidwall-gjson | t13 | fresh | cache-hit ⚠ | 0.0014 |
| sindresorhus-p-queue | t6 | warmup (normal) | answered | 25.673 |
| sindresorhus-p-queue | t7 | covered | cache-hit | 0.0013 |
| sindresorhus-p-queue | t8 | covered | answered | 17.326 |
| sindresorhus-p-queue | t9 | fresh | answered | 15.123 |
| sindresorhus-p-queue | t10 | fresh | answered | 15.328 |
| python-attrs-attrs | t6 | warmup (normal) | answered | 102.268 |
| python-attrs-attrs | t7 | covered | cache-hit | 0.0017 |
| python-attrs-attrs | t8 | covered | answered | 20.271 |
| python-attrs-attrs | t9 | fresh | cache-hit ⚠ | 0.0015 |
| python-attrs-attrs | t10 | fresh | answered | 15.715 |

⚠ = nominally-fresh question the overlap scorer judged covered.

## Gate fidelity (read this before quoting the primary number)

Pre-registered mix was 6 covered + 6 fresh; outcome was 7 cache-hit / 5
explored. The gate cached every covered ask and also 2 of 6 nominal fresh
asks; on `tidwall-gjson` (warmed narrowly by two escape-parsing turns) it
cached ALL FOUR measured asks. This is exactly the false-positive surface
ADR-0040.1 §Consequences acknowledges ("the cache-hit gate can
false-positive"). It is survivable by design — every cached answer carries
the `DEGRADED (cache-hit)` label, `[cached]` claim prefixes, snapshot pin +
age in uncertainty, and the overlap score vs threshold for audit — and it is
visible in aggregate as degraded share (58% of the measured population here),
never hidden. Spot-check of substance (gjson t11 "which function strips
escape characters from a path part?") returned the correct two-phase parse /
`unescape()` account citing gjson.go:695-726 and gjson.go:2254-2312.

## Known warts observed in this run (not fixed here)

1. Ledger `inspected_paths` token pollution: multi-line tool-call blocks
   (`ghx read … \n echo "=== X ===" \n ghx read …`) and mimicked annotations
   (`(cached, turn 1)`, `2>/dev/null`, `||`) leak prose tokens into
   `inspected_paths` via `DeriveCommandEvidence`; they surface as garbage
   `relevantFiles` entries in cache-hit reports. Follow-up card filed;
   hygiene fix belongs to the ADR-0030.1 D5 evidence-derivation family.
2. Cached claims can inherit stale wording from the prior turn's report text
   (one gjson cached claim says "(fresh read this turn)" while carrying a
   `[cached]` prefix and ledger-turn citation). Cosmetic; the label and
   provenance remain truthful.
3. The aborted attempt's turns emitted no operation-duration datapoint
   (the defect dc1512e fixed); they carry no latency observations and cannot
   contaminate these numbers, but they are why session dirs hold more turns
   than the batch script defines.

## How to recompute

Zero-token recompute from committed artifacts:

```
node scripts/dogfood-week.mjs --since 2026-08-16 --until 2026-08-22 --json   # all-window rollup
node scripts/dogfood-week.mjs --since 2026-08-16 --until 2026-08-22 --json \
  --sessions-dir <dir containing only the three batch session dirs>          # batch-only view
```

The primary cut is the lower median of the 12 `duration_seconds` values in
`d3-batch.json` → `measured_population_12_cheap_asks`. Raw sources:
`~/.ghx/sessions/{tidwall-gjson,sindresorhus-p-queue,python-attrs-attrs}/`
(`metrics.jsonl` operation-duration histograms keyed by
`ghx.sidecar.turn`; `reports/*.json` for outcome classification;
`live.jsonl` turn.started for question mapping). Session dirs are not
committed; the JSON artifact plus the rollup command reproduce every number.
