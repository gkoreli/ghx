---
title: "ADR-0030: Always-On Sidecar Runtime — Warm Daemon"
date: "2026-07-06"
status: "accepted"
thread: "sidecar-runtime"
author: "Goga Koreli"
---

# 0030. Always-On Sidecar Runtime — Warm Daemon

## Status

Proposed. Governs NORTH_STAR workstream **B6** (always-on runtime) and lays
the runtime ownership groundwork for **B7** (session routing). This ADR does
not change scoring, detectors, eval gates, or the measurement stack.

## Context

The north star says the sidecar is a runtime, not a command: a resident,
reusable process shared by every entrypoint (CLI, MCP, future A2A), with no
cold starts. It also says session mastery belongs inside the framework: the
end state is that the main agent sends a question and ghx routes it to the
right active session internally.

The current implementation is not there yet:

- `ghx sidecar ask` loads config and calls `sidecar.Ask` directly
  (`internal/cli/sidecar.go`). Each invocation is a separate process.
- `ghx serve --recon` exposes the right adoption surface, but its MCP
  handler also calls `sidecar.Ask` directly (`internal/cli/serve.go`).
- `sidecar.Ask` owns a single turn: it checks the ACP handshake, initializes
  or reads the named session, builds the prompt, calls `RunTurnWithOptions`,
  persists reports/ledger/meta, and emits artifacts (`internal/sidecar/runtime.go`).
- `RunTurnWithOptions` starts the configured ACP agent subprocess with
  `exec.CommandContext`, creates an ACP connection, calls `NewSession` or
  `LoadSession`, sends one prompt, snapshots the result, and then shuts the
  subprocess down (`internal/sidecar/acp.go`).
- Durable state already survives process boundaries: `meta.json` stores the
  named session and `ACPSessionID`; ledgers, reports, traces, logs, and metrics
  live under `~/.ghx/sessions/<session>/` (`internal/sidecar/session.go`,
  ADR-0022).
- ADR-0027 made failed turns auditable and resumable, but it still notes the
  residual gap: a hard kill of the ghx process can lose in-memory turn state
  until the always-on runtime exists.

So B6 is not a rewrite of the sidecar brain. It is an ownership move: the long
lived process owns ACP connections, session routing state, synchronization,
and lifecycle. CLI, MCP, and future A2A become clients.

## Decision

Build a local, per-user ghx sidecar daemon: a resident `ghx` process that owns
the sidecar runtime and is shared by every entrypoint.

The mental model is docker-sidecar, locally: one warm process beside the main
agent environment, serving local requests over an IPC socket, writing the same
`~/.ghx` artifacts, and retaining warm ACP agent subprocesses across asks.

## D1. Process lifecycle: auto-spawn plus explicit daemon command

V1 has two supported lifecycle paths:

1. **Auto-spawn on first use.** `ghx sidecar ask`, `ghx serve --recon`, and
   future A2A clients first try to connect to the daemon. If no healthy daemon
   is listening, they start one in the background and retry the request.
2. **Explicit foreground command.** Add `ghx sidecar daemon` to run the daemon
   in the foreground for debugging, logs, and supervised environments.

The daemon is per user and per `GHX_HOME` root. Runtime metadata lives under:

```text
~/.ghx/runtime/
  daemon.json      # pid, socket path, ghx version, startedAt, config digest
  daemon.log       # daemon lifecycle logs, not turn artifacts
  sidecar.sock     # Unix-domain socket on Unix-like systems
```

On Unix-like systems, auto-spawn starts the current `ghx` executable with
`sidecar daemon --background` or an equivalent hidden internal mode. On Windows,
the same JSON-RPC protocol should ride a named pipe; if named-pipe support is
not implemented in the first slice, Windows falls back to daemonless execution
rather than blocking the product.

`launchd` and `systemd` integration are not required for v1. They are installer
ergonomics, not runtime semantics. The explicit daemon command makes them easy
to add later without changing the client protocol.

## D2. IPC: Unix socket JSON-RPC, not local HTTP or MCP framing

Use JSON-RPC 2.0 over a local Unix-domain socket for daemon IPC.

Recommended transport:

```text
client process
  -> ~/.ghx/runtime/sidecar.sock
  -> newline-delimited JSON-RPC 2.0 request/response frames
  -> daemon
```

Initial methods:

```text
ghx.sidecar.Ping() -> daemon health/version/config digest
ghx.sidecar.Ask(request) -> report + artifacts pointer + turn summary
ghx.sidecar.ListSessions() -> session metadata
ghx.sidecar.ShowSession(name) -> metadata + reports
ghx.sidecar.Shutdown() -> graceful daemon stop
```

Rationale:

- **Unix socket** is local-only by construction, avoids port selection, avoids
  localhost firewall/proxy weirdness, and has filesystem permissions that match
  `~/.ghx`.
- **JSON-RPC** matches the request/response shape we need and resembles ACP's
  operational model without pretending the daemon is an ACP agent.
- **Not local HTTP** for v1: HTTP adds port binding, origin/security questions,
  and a public-service feel the product does not need. A future viewer or
  remote-control surface may use HTTP, but the internal runtime bus should stay
  local and private.
- **Not MCP framing** for daemon IPC: MCP is an external tool surface. Reusing
  it internally would blur the boundary and force every CLI/A2A caller through
  a tool protocol when they need a small runtime API.
- **Not ACP framing**: the daemon is not an ACP agent. ACP remains the adopted
  protocol between ghx and the configured specialist agent subprocess.

For streaming, v1 may either block until the ask completes or emit JSON-RPC
notifications such as `ghx.sidecar.TurnUpdate`. The durable report and artifacts
remain the contract; live streaming is user experience, not correctness.

## D3. Entry points become daemon clients, with daemonless fallback

All sidecar-facing entrypoints should share one client path:

```text
CLI ask
MCP recon tool
future A2A task handler
  -> sidecar daemon client
  -> daemon Ask
  -> sidecar runtime service
```

The daemon client has this behavior:

1. Try `Ping`.
2. If the daemon is healthy and version-compatible, send the request.
3. If no daemon exists, auto-spawn and retry once.
4. If auto-spawn fails, run the existing daemonless `sidecar.Ask` path and
   print a warning in human surfaces.

Daemonless invocation must keep working. This preserves scripts, CI, tests,
restricted environments, and platforms where the socket implementation is not
yet shipped. It also keeps the current eval runner from depending on a resident
process unless a later ADR explicitly moves eval execution through the daemon.

The fallback is a compatibility path, not the target path. Dogfood and normal
agent use should exercise the daemon.

## D4. ACP agent subprocesses are pooled and kept warm

The daemon owns an ACP pool. The current `RunTurnWithOptions` one-shot shape is
refactored into a longer-lived runtime service with explicit session workers.

Conceptual model:

```go
type Daemon struct {
    registry *SessionRegistry
    pool     *AgentPool
}

type AgentPool struct {
    bySession map[string]*AgentWorker
}

type AgentWorker struct {
    sessionName  string
    repo         string
    acpSessionID string
    agent        *ACPAgentProcess
    queue        chan AskRequest
}
```

Each `AgentWorker` owns one live ACP subprocess connection for one active ghx
session. It serializes turns for that session so two callers cannot prompt the
same ACP session concurrently. Different sessions may run concurrently, bounded
by a daemon-level max concurrency.

Warm behavior:

- First ask for a session creates or loads session metadata, starts the ACP
  agent subprocess, initializes ACP, and calls `NewSession` or `LoadSession`.
- Later asks for the same session reuse the same subprocess and ACP session
  without a new process spawn or handshake.
- Idle workers stay warm for a configurable TTL, for example 30 minutes.
- On TTL expiry, the worker shuts down gracefully using the existing
  `ShutdownAgent` semantics. Durable `ACPSessionID` remains in `meta.json`, so
  a later ask can `LoadSession` again.
- A daemon may preflight the configured agent at startup, but it should not
  eagerly start a subprocess for every session. Warm by demand, not by scanning
  disk.

This changes the cold-start profile from "every ask pays process spawn +
initialize + session load" to "first ask per active session pays it; follow-ups
reuse it." That directly advances B6 and unlocks anticipation work later.

## D5. Session registry ownership moves into the daemon

The daemon owns the authoritative in-memory session map and all per-session
locks. Callers send questions and hints; they do not coordinate sessions.

V1 request shape should still accept `session` and `repo` because existing
surfaces expose them:

```json
{
  "question": "How is middleware chained?",
  "repo": "honojs/hono",
  "session": "honojs-hono",
  "depth": "normal"
}
```

But the daemon, not the caller, resolves the effective session:

1. Explicit `session` wins.
2. Repo-scoped requests default to the repo slug, matching ADR-0019.
3. Discovery requests default to the question slug, matching current behavior.
4. The daemon records routing inputs and the chosen session in logs/traces.

This is B7 groundwork. A later ADR can replace the simple resolver with semantic
routing across active sessions. The important architectural boundary changes
now: routing state lives inside the runtime, not inside every entrypoint.

## D6. Crash and upgrade story

Daemon health is explicit and cheap. `daemon.json` records:

```json
{
  "pid": 12345,
  "socket": "/Users/goga/.ghx/runtime/sidecar.sock",
  "version": "2.x.y",
  "configDigest": "sha256:...",
  "startedAt": "2026-07-06T..."
}
```

Clients treat a daemon as stale when:

- the socket cannot be connected;
- the pid is dead;
- `Ping` returns a different ghx version than the client binary;
- `Ping` returns a config digest that no longer matches the current
  `~/.ghx/config.json`, for fields that affect agent process shape;
- the socket path points outside the active `GHX_HOME`.

Version mismatch follows the report-sink precedent in `RunPreflight`: fail
closed for the warm path because a stale helper binary can silently degrade the
contract. The client should ask the old daemon to shut down, then spawn the
current binary. If shutdown fails, it may remove the stale socket only after
pid checks prove the process is gone.

Daemon restarts must not lose sessions. This is already mostly true because
session state is on disk:

- `meta.json` carries `ACPSessionID`;
- ledger and accepted reports are persisted;
- traces/logs/metrics/reports are under the session directory;
- ADR-0027 already handles stale ACP session IDs by creating one fresh ACP
  session and continuing from durable ledger context.

What a restart can lose in v1 is only in-flight, not-yet-flushed turn telemetry.
The daemon reduces that window relative to short-lived commands because it can
centralize signal handling and graceful shutdown. Full per-event streaming flush
is a separate follow-up; v1 should at minimum emit artifacts on every completed
or failed `Ask` path exactly as ADR-0027 requires today.

Upgrade flow:

1. New client detects old daemon version.
2. Client sends graceful shutdown.
3. Daemon stops accepting new asks, lets active turns finish up to a short
   deadline, flushes artifacts, closes ACP subprocesses, and exits.
4. Client starts the new daemon and retries the original request.
5. If an active turn cannot finish before the deadline, it is marked BLOCKED or
   failed with artifacts rather than silently abandoned.

## D7. Observability stays in `~/.ghx`, unchanged in shape

The daemon emits lifecycle logs under `~/.ghx/runtime/`, but sidecar work
artifacts stay exactly where ADR-0022 put them:

```text
~/.ghx/sessions/<session>/
  meta.json
  ledger.json
  traces.jsonl
  logs.jsonl
  metrics.jsonl
  reports/
```

Every daemon-handled ask returns the same artifacts pointer that CLI and MCP
already return today. The daemon adds runtime-level attributes where useful:

```text
ghx.sidecar.daemon.pid
ghx.sidecar.daemon.version
ghx.sidecar.daemon.worker_reused
ghx.sidecar.daemon.worker_idle_ms
ghx.sidecar.session.route_source
```

No forked visibility stack. No daemon-only trace format. A human or agent must
still be able to inspect the session artifacts without knowing whether the ask
was daemon-backed or daemonless.

## D8. Security and permissions

The socket is local and private:

- create `~/.ghx/runtime` with user-only write permissions;
- create the socket with owner-only access where the OS supports it;
- reject requests when the socket path or runtime metadata is not under the
  active `GHX_HOME`;
- do not expose a TCP listener in v1;
- keep the existing `denyClient` read-only permission behavior for ACP tool
  requests.

The daemon is a local assistant runtime, not a network service.

## Considered and rejected

- **Keep spawning per command and call it good.** Rejected: it directly fails
  NORTH_STAR §4 and B6. It also makes anticipation and true session routing
  awkward because no process owns active state.
- **Require users to manually run `ghx sidecar daemon` before `ask`.**
  Rejected for normal use: the adoption surface should stay one command/tool.
  The explicit command is for debugging and supervisors; first ask should
  auto-spawn.
- **Install launchd/systemd services in v1.** Rejected as premature installer
  work. The daemon command creates a stable target for service files later.
- **Use localhost HTTP.** Rejected for the runtime bus: ports, firewalls,
  origins, and accidental remote exposure are unnecessary. HTTP can be a future
  viewer/control surface if needed.
- **Reuse MCP as internal daemon IPC.** Rejected because MCP is the external
  tool boundary. The daemon API is runtime control, not a model-facing tool
  catalog.
- **Reuse ACP as internal daemon IPC.** Rejected because ghx is not acting as an
  ACP agent to its own CLI. ACP remains the protocol between ghx and the
  specialist subprocess.
- **One global ACP subprocess for all sessions.** Rejected: ACP sessions may
  have different system prompts, model settings, MCP report-sink paths, cwd, or
  recovery state. Pool by ghx session; bound concurrency instead.
- **Daemon-only with no fallback.** Rejected: scripts, tests, CI, and unsupported
  platforms need the existing direct path. The fallback also de-risks rollout.
- **Store authoritative session state only in memory.** Rejected by the
  visibility and resilience tenets. Disk artifacts remain the source of truth;
  memory is a warm cache and router.

## V1 scope

In scope:

- `ghx sidecar daemon` foreground command.
- Auto-spawn daemon client used by CLI ask and `serve --recon`.
- Unix socket JSON-RPC transport on Unix-like systems.
- Health/version/config-digest checks.
- Warm ACP worker pool keyed by effective session.
- Per-session serialization and bounded cross-session concurrency.
- Graceful shutdown and stale-daemon replacement.
- Same reports, ledgers, traces, logs, metrics, and artifact footer as today.
- Daemonless fallback.

Out of scope for v1:

- Semantic B7 session routing beyond the current explicit/default resolver.
- Anticipation/background follow-up work (M8/B9).
- A2A AgentCard/task implementation.
- launchd/systemd installer automation.
- Remote daemon access or TCP serving.
- Multi-user daemon sharing.
- A web control plane.
- Changing eval scoring, reward gates, anomaly detection, or citable benchmark
  claims.
- Replacing ACP or changing the sidecar report contract.
- Streaming every ACP event to disk before turn completion; v1 preserves the
  current flush-on-exit-path guarantee.

## Implementation sketch

Introduce `internal/sidecar/daemon` or equivalent with three domain services:

```text
DaemonServer      # socket accept loop, JSON-RPC dispatch, health/shutdown
RuntimeService    # Ask/List/Show API, owns registry and pool
AgentPool         # session workers, ACP subprocess lifecycle, TTL cleanup
```

Refactor current one-shot runtime so the durable pieces remain reusable:

- keep `AskRequest`, `Report`, `TurnResult`, session persistence, report sink,
  prompt building, telemetry emission, and resilience classification;
- split ACP process/connection lifecycle out of `RunTurnWithOptions` so a worker
  can hold it across prompts;
- keep a daemonless adapter that constructs a temporary one-turn worker and
  behaves like the current code path.

The first implementation should be proven with fake ACP agents, not live token
spend:

- daemon starts and answers `Ping`;
- client auto-spawns when socket is absent;
- version mismatch causes restart;
- two asks for one session reuse one ACP subprocess;
- concurrent asks for the same session serialize;
- asks for different sessions can run concurrently within the configured limit;
- daemon kill/restart resumes from on-disk `ACPSessionID` or falls back through
  the existing stale-session recovery path;
- `serve --recon` and `sidecar ask` return the same report/artifacts envelope
  through the daemon and through daemonless fallback.

## Cross-references

- NORTH_STAR §4 — always-on runtime and session mastery.
- NORTH_STAR workstream B6/B7 — warm daemon first, routing mastery next.
- ADR-0015 — Go-native runtime and explicit ACP/session ownership.
- ADR-0019 — zero-CLI adoption surface; `serve --recon` and CLI ask become
  clients of the same runtime.
- ADR-0022 — shared `~/.ghx` visibility substrate; daemon must not fork it.
- ADR-0027 — runtime resilience and session recovery; daemon reduces the
  remaining in-flight loss window and reuses the same recovery semantics.
- `internal/sidecar/runtime.go` — current `Ask` orchestration.
- `internal/sidecar/acp.go` — current one-shot ACP subprocess lifecycle.
- `internal/sidecar/config.go` — `~/.ghx` root and config shape.
- `internal/cli/serve.go` — current MCP recon handler calling `sidecar.Ask`
  directly.

## Provenance

Drafted by codex (read-only) against the mining evidence and ADR-0028 prior
art; reviewed and accepted by Fable 2026-07-06. Build queued behind the
judge-rails merge to serialize internal/sidecar churn.

## Implementation Notes — 2026-07-06

Implemented on branch `codex-daemon-b6` as the first B6 daemon slice.

- **D1 lifecycle.** Added `ghx sidecar daemon` as the foreground daemon command,
  with the hidden `--background` mode used by auto-spawn and `--stop` for the
  graceful shutdown RPC. Runtime metadata is written to
  `~/.ghx/runtime/daemon.json`, lifecycle logs from auto-spawn go to
  `~/.ghx/runtime/daemon.log`, and the socket is
  `~/.ghx/runtime/sidecar.sock` (`internal/cli/sidecar.go`,
  `internal/sidecar/daemon.go`).
- **D2 IPC.** Added newline-delimited JSON-RPC 2.0 over a Unix-domain socket
  with `ghx.sidecar.Ping`, `Ask`, `ListSessions`, `ShowSession`, and
  `Shutdown`. The daemon rejects runtime paths outside active `GHX_HOME` and
  creates runtime files with user-private permissions (`daemon.go`).
- **D3 clients and fallback.** `ghx sidecar ask` and `ghx serve --recon` now
  call the shared daemon client first. If auto-spawn or the socket path fails,
  the client prints a warning and runs the existing daemonless `sidecar.Ask`
  path (`internal/cli/sidecar.go`, `internal/cli/serve.go`,
  `internal/sidecar/daemon.go`).
- **D4 warm ACP pooling.** Added `AgentPool` and `AgentWorker`, keyed by
  resolved ghx session. A worker owns one live ACP process/connection and
  serializes turns for that session. To keep the strict `submit_report` sink
  fresh without respawning, warm follow-up turns call ACP `LoadSession` with
  the new per-turn MCP server before `Prompt`; this preserves report-sink
  semantics while skipping process spawn and initialize. Idle workers expire
  after 30 minutes, and cross-session turns are bounded by a daemon-level
  concurrency semaphore (`internal/sidecar/daemon_worker.go`).
- **D5 daemon-owned registry.** Session routing moved into the sidecar domain
  as `ResolveSessionName`: explicit session, then repo slug, then question slug.
  CLI and MCP still compute/display the same defaults for user ergonomics, but
  daemon requests are resolved again inside the runtime boundary
  (`internal/sidecar/runtime.go`).
- **D6 crash/upgrade.** The client treats failed ping, dead pid, version
  mismatch, config digest mismatch, and out-of-root socket paths as stale. A
  version/config mismatch sends `Shutdown`, starts the current executable, and
  retries the request. Tests cover version-mismatch replacement with a built
  `ghx` binary and scripted ACP agent (`internal/sidecar/daemon_test.go`).
- **D7 artifacts unchanged.** Daemon-handled asks still call the same
  centralized `AskWithTurnRunner` persistence path: `meta.json`, ledger,
  reports, traces, logs, metrics, and artifact footer remain under
  `~/.ghx/sessions/<session>/`. The daemon adds only lifecycle metadata/logs
  under `~/.ghx/runtime/`.

Verification:

- Unit/integration tests added in `internal/sidecar/daemon_test.go` for socket
  lifecycle, warm mockagent reuse, daemonless fallback, and version-mismatch
  restart.
- `go vet ./...` passed.
- `go test ./...` passed.
- `go test -race ./internal/sidecar` passed after fixing a worker shutdown
  liveness-goroutine race found by the race detector.
- Live smoke with a built `ghx` binary and scripted ACP mock agent:
  foreground daemon started, two CLI asks went through it, first ask `real
  0.41`, second ask `real 0.01`, mock agent start counter stayed `1`, prompt
  log showed `LOAD mock-sess-1` before the second prompt, and `ghx sidecar
  daemon --stop` removed the socket.

Remaining follow-up:

- The worker idle TTL and cross-session concurrency limit are fixed defaults
  in this slice; exposing them as supported config belongs in a later runtime
  ergonomics pass.
- Windows named-pipe support is still out of this Unix-socket implementation;
  Windows keeps the daemonless fallback path.

## Implementation Note — 2026-07-07

The runtime-ergonomics follow-up for daemon pool limits is implemented:
`~/.ghx/config.json` now accepts `daemonWorkerIdleTTLMinutes` and
`daemonMaxConcurrent`. Unset fields preserve the original daemon behavior
(30-minute warm-worker idle TTL, four cross-session turns), explicit zero or
negative values fail daemon startup with field-named errors, and both fields
participate in the D6 config digest so a running daemon with different runtime
tunables is treated as stale.
