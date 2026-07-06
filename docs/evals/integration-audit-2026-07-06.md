# Integration Audit — five one-hour merges (b927f4c..2f78efa)

Date: 2026-07-06
Auditor: adversarial integration pass (isolated worktree, branch
`audit-integration-2f78efa` at HEAD `2f78efa`).
Scope: the interactions between five builds that landed within one hour, each
built and tested in isolation on branches cut from slightly different bases:

1. Judge CLI transport (ADR-0023.1) — `internal/sidecar/evals/judge_client_cli.go`, `judge_config.go`
2. Always-on daemon (ADR-0030) — `internal/sidecar/daemon.go`, `daemon_worker.go`, `internal/cli/sidecar.go`
3. `ghx inspect` (ADR-0028.2) — `internal/ghx/inspect.go`, `internal/cli/ghx.go`
4. TRL SFT exporter (ADR-0017.1) — `internal/sidecar/evals/training_export.go`, `internal/cli/sidecar.go`
5. Discovery eval class (ADR-0019.2) — `internal/sidecar/evals/discovery.go`, `anomalies.go`

Directive: fix nothing except `gofmt`; report everything else.

## Mechanical gate results (real exit codes)

| check | command | exit | result |
|---|---|---:|---|
| format | `gofmt -l .` (before fix) | 0 | 1 file: `internal/sidecar/evals/union_scoring_test.go` (comment alignment) — **fixed** per directive |
| format | `gofmt -l .` (after fix) | 0 | clean |
| vet | `go vet ./...` | 0 | clean |
| tests | `go test ./...` | 0 | all packages ok (sidecar 22.5s, evals 14.1s) |
| race | `go test -race ./internal/sidecar/... ./internal/cli/...` | 0 | all ok |

The only working-tree change is the sanctioned `gofmt -w` on
`union_scoring_test.go` (test-comment alignment; vet + full suite re-run green
afterward).

## Severity-ordered findings

### F1 — LOW/MEDIUM — warm daemon does not restart when `GHX_REPORT_SINK_EXE` changes

`internal/sidecar/daemon.go:242-252` (`ConfigDigest`) fingerprints only
`AgentCmd, Cwd, cfg.Env, Model`. The report-sink executable is resolved by
`ResolveReportSinkExe` (`internal/sidecar/acp.go:620-633`) from the **process
environment** `os.Getenv("GHX_REPORT_SINK_EXE")`, read live inside the daemon
process. The daemon inherits `os.Environ()` frozen at spawn time
(`daemon.go:438 cmd.Env = os.Environ()`) and `ensure()` only restarts on
version / config-digest / dead-pid / out-of-root mismatch
(`daemon.go:345-370`). Because `GHX_REPORT_SINK_EXE` is in neither the digest
nor `cfg.Env` (which is `json:"-"` and normally nil — `config.go:61`), the first
client to spawn the daemon pins the report-sink exe for the daemon's lifetime
(up to the 30-min idle TTL, `daemon_worker.go:29`).

Impact: narrow. In the ordinary CLI case `GHX_REPORT_SINK_EXE` is unset and
`os.Executable()` resolves to the running (correct) ghx daemon binary, so the
sink is right. The gap bites only when an eval harness or operator sets
`GHX_REPORT_SINK_EXE` to binary A, then re-invokes (same ghx version) pointing
at binary B: a still-warm daemon keeps serving A's sink. No incorrect artifact
is produced silently in the common path; the failure mode is "stale sink binary
under an explicit override", which is exactly the scenario eval runs use.

Repro (structural, no live model needed):
```
grep -n "GHX_REPORT_SINK_EXE\|os.Getenv" internal/sidecar/daemon.go   # -> none; not in digest
sed -n '/type digestConfig struct/,/}/p' internal/sidecar/daemon.go   # -> AgentCmd,Cwd,Env,Model only
```

Recommendation (not applied): fold the resolved report-sink exe (and the raw
`GHX_REPORT_SINK_EXE` value) into `ConfigDigest`, or resolve the sink exe on the
client and pass it through the `Ask` RPC so the daemon never depends on its own
frozen env. Not a correctness bug for the default path; a resilience gap for the
override path the evals build depends on.

### F2 — INFO — `sidecar evals export` collapses all failures to exit 2

`internal/cli/sidecar.go:139-162` returns plain `fmt.Errorf` for every failure
(bad `--format`, missing `--run`/`--out`, and a nonexistent run dir), so all map
to `ExitBadInvocation` (2) via the default in `errors.go:49-55`. A missing run
directory is arguably an upstream/IO failure (3), not a bad invocation. This is
**acceptable**: `export` is a hidden internal command, not a D7 agent-facing
surface, and ADR-0028.1 D7's exit-code contract is explicitly scoped to the
agent-branching CLI surface. Noted for completeness, not a defect.

Repro:
```
ghx sidecar evals export --format nope --run /tmp --out /tmp/o.jsonl; echo $?   # 2
ghx sidecar evals export --format sft  --run /tmp/nonexistent --out /tmp/o.jsonl; echo $?  # 2
```

## Verified-sound (adversarially checked, held up)

### Daemon path vs everything (live, with mock ACP agent)

Built `ghx` + `mockagent` from HEAD and ran a real daemon-vs-daemonless
comparison via an in-package probe (`sidecar.Ask` daemonless vs
`DaemonClient.Ask` through a live Unix-socket daemon, identical config + mock
agent; probe deleted before commit):

- **Artifact parity: IDENTICAL.** Same session-dir file tree
  (`initialized, ledger.json, logs.jsonl, meta.json, metrics.jsonl,
  reports/<turn>-<ms>.json, traces.jsonl`), byte-identical report JSON (290 B),
  identical artifacts-footer shape (`artifacts: <dir> (trace <hex>)`), identical
  session meta (repo, scope, `turns=1`, `acp=mock-sess-1`). Only the report
  filename epoch-ms suffix and the trace hex differ — both are per-run by design.
  Confirms ADR-0030 D7 ("daemon-handled asks call the same centralized
  `AskWithTurnRunner`", `daemon.go:202`).
- **ADR-0027 resilience survives daemon indirection.** A scripted max-turns
  (`promptError: "Reached maximum number of turns (24)"`) followed by a wrap-up
  `submitReport` round-tripped through the socket: `WrapUpRecovered=true`,
  recovered report answer intact, artifacts dir non-empty. Wrap-up recovery,
  partial-artifact persistence, and the resume path all work behind the RPC.
- **`GHX_REPORT_SINK_EXE` works daemon-side (positive case).** Both probes ran
  with `GHX_REPORT_SINK_EXE` set at daemon spawn and produced the strict
  submit_report artifacts correctly; the daemon resolves the sink per-turn via
  `reportSinkMcpServers` → `ResolveReportSinkExe`. (The staleness gap is F1.)
- **Path safety.** `assertUnderRoot` (`daemon.go:262-279`) is enforced in
  `Serve` and `ping`; socket/metadata are user-private (0600).

### CLI surface coherence

- `ghx sidecar --help`, `ghx inspect --help`, `ghx sidecar evals export --help`
  all render clean; every command carries an `Examples:` block (ADR-0028.1 D4).
- **No command registration collisions.** `RootCmd.AddCommand` unions across
  `ghx.go`, `code.go`, `serve.go`; `sidecarCmd.AddCommand` unions across
  `sidecar.go:408` and `sidecar_view.go:162` (the `view` subcommand). No
  duplicate `Use:` names across the merged `init()` blocks.
- **Exit codes (live).** `inspect` follows D7: bad repo / too-few-args → 2,
  no-results → 1, upstream → 3 (`ghx.go:383-392`), matching ADR-0028.2's claim.

### Evals package coherence

- **No duplicated helpers.** Each shared helper (`ratio`, `mean`,
  `normalizeRepoSlug`, `boundedString`, `containsString`, `normalizeToolCommand`,
  `invokesGhx`, `rawTokenInvokesGhx`, `scoringReport`) has exactly one definition;
  the three builds (judge/discovery/exporter) reuse them. No shadow copies.
- **Anomaly taxonomy is complete.** All 10 kinds emitted by `DetectAnomalies`
  appear in the `CountAnomalies` order list (`anomalies.go:206-219`) — nothing is
  detected-but-dropped from the verdict summary (visibility tenet holds). The
  exporter's dependency on the taxonomy (`training_export.go:243-249`, excluding
  `answer_doc_contamination` + `SeverityBreaking`) resolves against the complete
  set.
- **Judge config drift checks intact.** `JudgeConfig.Validate`
  (`judge_config.go:141-179`) fails on `promptVersion`/`rubricVersion`/
  `thresholds.poor` drift vs code constants, rejects unknown fields, and rejects
  trailing content. Config + code must move in one diff.
- **Discovery G1-G5 separation.** `gates.go` (owner of `EvaluateGates`/`Verdict`/
  G1-G5) has zero references to any `Discovery*` type; no discovery metric feeds
  repo-scoped scoring (ADR-0019.2 D4).

### ADR Implementation-Notes claims verified against code (3 per build, not trusted)

- **ADR-0030 (daemon).** D7 same-`AskWithTurnRunner` path ✓ (`daemon.go:202`);
  D2 out-of-root rejection ✓ (`assertUnderRoot`); D5 `ResolveSessionName`
  session→repo→question ✓ (`runtime.go:461`).
- **ADR-0028.2 (inspect).** All 8 domain types + `InspectRanker` present ✓;
  reuses `Search`/`Read`/`mapengine.Map` ✓ (`inspect.go:254,301,467`);
  `maxInspectFetchedFiles = 10` matches "top 10 candidates" ✓.
- **ADR-0017.1 (exporter).** Frozen `CreatedAt: 1970-01-01T00:00:00Z` ✓;
  full filter list present in `sftEpisodeExclusionReason` ✓; **measured counts
  reproduce exactly** — `gate-run-2026-07-05-confirmatory` 90→81→103 (excl 4+5),
  `gate-run-2026-07-06-fixbatch` 100→91→119 (excl 7+2); re-export with the same
  out path is byte-identical (jsonl + manifest sha match).
- **ADR-0019.2 (discovery).** Fixtures separated under
  `testdata/discovery-tasks/` ✓; verified target sets match — gateways (3 +
  `QuantumNous/new-api`), mcp-sdks (10 `modelcontextprotocol/*-sdk`), llm-eval
  (4 + `vibrantlabsai/ragas` with `explodinggradients/ragas` alias) ✓; D4
  G1-G5 separation ✓.
- **ADR-0023.1 (judge CLI transport).** Committed config `transport: cli` ✓;
  primary gpt-5.5 via `codex exec --json -s read-only --output-last-message`,
  secondary `claude -p` ✓; `JudgeResult.JudgeRuntimeVersion` recorded via
  `RuntimeVersion()` captured at construction (`judge.go:160-161`,
  `judge_client_cli.go:56`) ✓.

## Reproduction — commands run

```
# build
go build -o /tmp/ghx-audit ./cmd/ghx
go build -o /tmp/mockagent-audit ./internal/sidecar/evals/mockagent

# gates
gofmt -l .
go vet ./...
go test ./...
go test -race ./internal/sidecar/... ./internal/cli/...

# exporter measured-count reproduction
GHX_REPORT_SINK_EXE=/tmp/ghx-audit /tmp/ghx-audit sidecar evals export --format sft \
  --run docs/evals/gate-run-2026-07-05-confirmatory --out /tmp/e1.jsonl
GHX_REPORT_SINK_EXE=/tmp/ghx-audit /tmp/ghx-audit sidecar evals export --format sft \
  --run docs/evals/gate-run-2026-07-06-fixbatch --out /tmp/e2.jsonl

# exit-code probes
/tmp/ghx-audit inspect notarepo q; echo $?           # 2
/tmp/ghx-audit sidecar evals export --format nope --run /tmp --out /tmp/o.jsonl; echo $?  # 2
```

The daemon-vs-daemonless parity and resilience checks used a temporary
in-package `_test.go` probe driving `sidecar.Ask` and `DaemonClient.Ask` against
a live socket daemon with a scripted mock ACP agent; it was removed before
commit (fix-nothing directive). The probe's assertions are re-expressible from
the shipped `internal/sidecar/daemon_test.go` harness helpers
(`buildDaemonMockAgent`, `startTestDaemon`).

## Bottom line

The five isolated builds integrate cleanly. Mechanical gates (vet, test, race,
gofmt) are green. The daemon preserves artifact/report/session-meta/footer
parity and ADR-0027 resilience through the RPC indirection. The only real
integration gap is **F1**: the warm daemon's config digest omits
`GHX_REPORT_SINK_EXE`, so an explicit sink-exe override that changes between
invocations is not honored until the daemon restarts — a resilience gap for the
eval-override path, not a default-path correctness bug. Reported, not repaired.
