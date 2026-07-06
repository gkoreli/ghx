# Judge Gold-Set Candidates

GENERATED FILE — do not edit by hand. Regenerate from the repo root with:

```bash
go run ./docs/evals/judge-goldset/gen
```

Candidate pool for the ADR-0023.1 D5 gold-set labeling protocol (see
`PROTOCOL.md` in this directory). Drawn from the committed gate runs:

- `docs/evals/gate-run-2026-07-05-confirmatory`
- `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial`

The D7 anomaly pre-filter (mirroring `judgePrefilter` in
`internal/sidecar/evals/judge.go`) excludes invalid (compliance-excluded),
BLOCKED, and WARN-noreport episodes — the same episodes the judge runner
skips, so human labels and judge scores cover the same pool.

## Counts (recomputed by this generator)

| run | episodes | invalid | BLOCKED | WARN-noreport | candidates |
| --- | ---: | ---: | ---: | ---: | ---: |
| gate-run-2026-07-05-confirmatory | 90 | 0 | 0 | 4 | 86 |
| gate-run-2026-07-05-d2-0021-0022-partial | 54 | 0 | 0 | 0 | 54 |
| **total** | 144 | 0 | 0 | 4 | 140 |

## Excluded episodes

| episode file | run | reason |
| --- | --- | --- |
| `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx-sidecar_1783298531855.json` | gate-run-2026-07-05-confirmatory | WARN-noreport episode — no report to judge (D7) |
| `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783294237204.json` | gate-run-2026-07-05-confirmatory | WARN-noreport episode — no report to judge (D7) |
| `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783299023061.json` | gate-run-2026-07-05-confirmatory | WARN-noreport episode — no report to judge (D7) |
| `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783296851961.json` | gate-run-2026-07-05-confirmatory | WARN-noreport episode — no report to judge (D7) |

## Candidates

140 candidates (45 multi-turn; reward bands: low 26 / mid 79 / high 35;
band cutoffs on deterministic `rewards.overall`: low < 0.75 ≤ mid < 0.85 ≤ high).

`label id` is the blinded ID used during labeling sessions (assigned in
sha256(episode-id) order so it does not mirror task/profile/run order).
The `stratum` tag is `task/turns/band` — sample labeling sessions evenly
across strata. Labelers: do NOT read this table during a session (the file
paths reveal the profile); see PROTOCOL.md.

| label id | episode file | task | turns | stratum | batch-1 |
| --- | --- | --- | --- | --- | --- |
| gs-001 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783298056855.json` | hono-middleware | multi | `hono-middleware/multi/high` | x |
| gs-002 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783294157688.json` | gin-routing | multi | `gin-routing/multi/low` | x |
| gs-003 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783297736205.json` | gin-routing | multi | `gin-routing/multi/low` | x |
| gs-004 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_plain_1783295442978.json` | hono-middleware | multi | `hono-middleware/multi/mid` | x |
| gs-005 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx-sidecar_1783297631780.json` | ghx-mapengine | single | `ghx-mapengine/single/high` | x |
| gs-006 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783298493517.json` | express-router-location | single | `express-router-location/single/mid` | x |
| gs-007 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783295612980.json` | hono-middleware | multi | `hono-middleware/multi/high` |  |
| gs-008 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_plain_1783312939421.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` | x |
| gs-009 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783294756191.json` | openai-node-streaming | single | `openai-node-streaming/single/high` | x |
| gs-010 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx_1783313256903.json` | gin-routing | multi | `gin-routing/multi/low` |  |
| gs-011 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx_1783311240390.json` | gin-routing | multi | `gin-routing/multi/low` |  |
| gs-012 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783298460843.json` | express-router-location | single | `express-router-location/single/low` | x |
| gs-013 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_plain_1783294098500.json` | gin-routing | multi | `gin-routing/multi/mid` | x |
| gs-014 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_ghx-sidecar_1783310733569.json` | express-router-location | single | `express-router-location/single/high` | x |
| gs-015 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783298949511.json` | gin-routing | multi | `gin-routing/multi/low` |  |
| gs-016 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_plain_1783293821717.json` | flask-routing | single | `flask-routing/single/mid` | x |
| gs-017 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_plain_1783310655275.json` | express-router-location | single | `express-router-location/single/low` | x |
| gs-018 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx_1783311524766.json` | hono-middleware | multi | `hono-middleware/multi/mid` | x |
| gs-019 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783297127420.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-020 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783297276126.json` | express-router-location | single | `express-router-location/single/mid` | x |
| gs-021 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_ghx_1783311051515.json` | ghx-mapengine | single | `ghx-mapengine/single/low` | x |
| gs-022 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_plain_1783311800253.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` | x |
| gs-023 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_plain_1783310984090.json` | ghx-mapengine | single | `ghx-mapengine/single/low` | x |
| gs-024 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_plain_1783296170787.json` | flask-routing | single | `flask-routing/single/mid` | x |
| gs-025 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_plain_1783309163031.json` | flask-routing | single | `flask-routing/single/mid` | x |
| gs-026 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_plain_1783295236474.json` | gin-routing | multi | `gin-routing/multi/mid` | x |
| gs-027 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx_1783296226734.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-028 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_plain_1783312715092.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-029 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_ghx_1783313049849.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` | x |
| gs-030 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx_1783295014799.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-031 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_plain_1783297673915.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-032 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx_1783293879222.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-033 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_plain_1783311452180.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-034 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783293718349.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-035 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_ghx-sidecar_1783309338687.json` | flask-routing | single | `flask-routing/single/high` | x |
| gs-036 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783297480460.json` | flask-routing | single | `flask-routing/single/high` | x |
| gs-037 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_plain_1783297379153.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-038 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx_1783313572918.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-039 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_plain_1783313932599.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` | x |
| gs-040 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783295375346.json` | gin-routing | multi | `gin-routing/multi/high` | x |
| gs-041 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_plain_1783313487693.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-042 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783299392603.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-043 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx-sidecar_1783294920428.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-044 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783313720066.json` | hono-middleware | multi | `hono-middleware/multi/high` |  |
| gs-045 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_plain_1783309401580.json` | ghx-mapengine | single | `ghx-mapengine/single/low` |  |
| gs-046 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_plain_1783299288701.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-047 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_plain_1783294959645.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-048 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783311690396.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-049 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783298297949.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-050 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_plain_1783294299404.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-051 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783296083341.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-052 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_plain_1783299098940.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-053 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx-sidecar_1783296136926.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-054 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783297582829.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` |  |
| gs-055 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_ghx-sidecar_1783311109966.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-056 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783298713787.json` | flask-routing | single | `flask-routing/single/high` |  |
| gs-057 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783296534553.json` | gin-routing | multi | `gin-routing/multi/low` |  |
| gs-058 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_ghx-sidecar_1783312647938.json` | express-router-location | single | `express-router-location/single/high` |  |
| gs-059 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_plain_1783298585715.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-060 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783295857279.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-061 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_plain_1783311159918.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-062 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_plain_1783297880428.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-063 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_plain_1783296475788.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-064 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783299216598.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-065 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_plain_1783298140100.json` | openai-node-streaming | single | `openai-node-streaming/single/low` | x |
| gs-066 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783297949030.json` | hono-middleware | multi | `hono-middleware/multi/low` | x |
| gs-067 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_plain_1783294544689.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-068 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_ghx-sidecar_1783310893876.json` | flask-routing | single | `flask-routing/single/high` |  |
| gs-069 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx_1783298670074.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-070 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_ghx_1783310830555.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-071 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783296737539.json` | hono-middleware | multi | `hono-middleware/multi/low` | x |
| gs-072 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783297015159.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-073 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx-sidecar_1783310129190.json` | hono-middleware | multi | `hono-middleware/multi/low` |  |
| gs-074 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783298382894.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-075 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_plain_1783313186140.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-076 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx-sidecar_1783313384747.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-077 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx_1783297425489.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-078 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx_1783309743757.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-079 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_plain_1783310768622.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-080 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783312393950.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-081 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_plain_1783309625204.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-082 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783297311084.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-083 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_ghx-sidecar_1783312861941.json` | flask-routing | single | `flask-routing/single/high` |  |
| gs-084 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783295974379.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-085 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783295061437.json` | flask-routing | single | `flask-routing/single/high` |  |
| gs-086 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783294852289.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-087 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx_1783294652455.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-088 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_ghx_1783309247803.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-089 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783296331466.json` | ghx-mapengine | single | `ghx-mapengine/single/low` |  |
| gs-090 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx-sidecar_1783298847451.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-091 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_plain_1783310231436.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-092 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783299156361.json` | hono-middleware | multi | `hono-middleware/multi/low` |  |
| gs-093 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783293764685.json` | express-router-location | single | `express-router-location/single/low` |  |
| gs-094 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx-sidecar_1783294067848.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-095 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_plain_1783309912731.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-096 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx_1783310407717.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-097 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_plain_1783296677767.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-098 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783298761314.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` |  |
| gs-099 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx-sidecar_1783297343093.json` | express-router-location | single | `express-router-location/single/high` |  |
| gs-100 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783297826596.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-101 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783296276954.json` | flask-routing | single | `flask-routing/single/high` |  |
| gs-102 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783297517822.json` | ghx-mapengine | single | `ghx-mapengine/single/low` |  |
| gs-103 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_ghx-sidecar_1783299494416.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-104 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_plain_1783298875531.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-105 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783310551196.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-106 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_ghx_1783309054858.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-107 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx-sidecar_1783311354381.json` | gin-routing | multi | `gin-routing/multi/mid` |  |
| gs-108 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783294017695.json` | ghx-mapengine | single | `ghx-mapengine/single/low` |  |
| gs-109 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx-sidecar_1783296609142.json` | gin-routing | multi | `gin-routing/multi/high` |  |
| gs-110 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx-sidecar_1783293783009.json` | express-router-location | single | `express-router-location/single/high` |  |
| gs-111 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783294377396.json` | hono-middleware | multi | `hono-middleware/multi/low` |  |
| gs-112 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_ghx-sidecar_1783309569014.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-113 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_ghx-sidecar_1783313137012.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-114 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/gin-routing_ghx-sidecar_1783309817215.json` | gin-routing | multi | `gin-routing/multi/high` |  |
| gs-115 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx-sidecar_1783295207069.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-116 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx_1783314087857.json` | openai-node-streaming | single | `openai-node-streaming/single/low` | x |
| gs-117 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx-sidecar_1783296429851.json` | ghx-mapengine | single | `ghx-mapengine/single/high` |  |
| gs-118 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783298806382.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` |  |
| gs-119 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_plain_1783312512597.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-120 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx_1783312259418.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-121 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/ghx-mapengine_ghx_1783309510221.json` | ghx-mapengine | single | `ghx-mapengine/single/low` |  |
| gs-122 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_plain_1783296940592.json` | openai-node-streaming | single | `openai-node-streaming/single/mid` |  |
| gs-123 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/openai-node-streaming_ghx-sidecar_1783314398605.json` | openai-node-streaming | single | `openai-node-streaming/single/high` |  |
| gs-124 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx_1783295556081.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-125 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783296377537.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` |  |
| gs-126 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_ghx_1783295158709.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` |  |
| gs-127 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_ghx_1783312582919.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-128 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_ghx_1783294881353.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-129 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/hono-middleware_ghx_1783310038731.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-130 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783295115112.json` | ghx-mapengine | single | `ghx-mapengine/single/mid` |  |
| gs-131 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_plain_1783308989874.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-132 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_ghx-sidecar_1783309111738.json` | express-router-location | single | `express-router-location/single/high` |  |
| gs-133 | `docs/evals/gate-run-2026-07-05-confirmatory/gin-routing_ghx_1783295296326.json` | gin-routing | multi | `gin-routing/multi/low` |  |
| gs-134 | `docs/evals/gate-run-2026-07-05-confirmatory/openai-node-streaming_plain_1783295682478.json` | openai-node-streaming | single | `openai-node-streaming/single/low` |  |
| gs-135 | `docs/evals/gate-run-2026-07-05-confirmatory/ghx-mapengine_plain_1783293965871.json` | ghx-mapengine | single | `ghx-mapengine/single/low` |  |
| gs-136 | `docs/evals/gate-run-2026-07-05-confirmatory/express-router-location_plain_1783296050223.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-137 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/express-router-location_ghx_1783310698299.json` | express-router-location | single | `express-router-location/single/mid` |  |
| gs-138 | `docs/evals/gate-run-2026-07-05-d2-0021-0022-partial/flask-routing_ghx_1783312793219.json` | flask-routing | single | `flask-routing/single/mid` |  |
| gs-139 | `docs/evals/gate-run-2026-07-05-confirmatory/hono-middleware_ghx-sidecar_1783294454620.json` | hono-middleware | multi | `hono-middleware/multi/mid` |  |
| gs-140 | `docs/evals/gate-run-2026-07-05-confirmatory/flask-routing_ghx-sidecar_1783293921812.json` | flask-routing | single | `flask-routing/single/high` |  |

## Recommended first labeling batch

30 episodes (marked `batch-1` above): 5 per task, round-robining the
low → mid → high reward bands within each task so every task and every
populated score range is covered. Generate the blinded labeling packets with:

```bash
go run ./docs/evals/judge-goldset/gen -packets
```

Batch label IDs: gs-001, gs-002, gs-003, gs-004, gs-005, gs-006, gs-008, gs-009, gs-012, gs-013, gs-014, gs-016, gs-017, gs-018, gs-020, gs-021, gs-022, gs-023, gs-024, gs-025, gs-026, gs-029, gs-035, gs-036, gs-039, gs-040, gs-065, gs-066, gs-071, gs-116
