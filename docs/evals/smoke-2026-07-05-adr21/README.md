# ADR-0021 live validation smoke — 2026-07-05

One strict episode (`ghx-mapengine` × `ghx-sidecar`) run on the ADR-0021
branch build to prove the submit_report contract against the real adapter
(`claude-agent-acp@0.55.0`, subject `claude-sonnet-5`):

- the adapter spawned the report-sink MCP server (`GHX_REPORT_SINK_EXE`
  pointed at the branch-built ghx binary);
- the model ran six ghx commands, then finished with a
  `mcp__ghx-report-sink__submit_report` tool call carrying the full report;
- no `<ghx-report>` text block in the output; zero anomalies; strict PASS;
  correctness 1.0.

`test-output.log` is the complete `go test` output (episode rewards,
context, tool calls, verdict). The run's episode JSON/OTel artifacts lived
in the build worktree, which was removed after merge — this log is the
preserved record. Invocation is documented in ADR-0021's implementation
notes ("Live validation"). This is a smoke, not a gate run: n=1, nothing
here is citable as a performance number (ADR-0021 D4 still pending).
