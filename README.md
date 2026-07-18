# ghx — the code-reconnaissance sidecar for AI agents

**Expensive main agents shouldn't burn frontier tokens on tree/grep/read loops.**
ghx is a cheap, specialized **sidecar agent** that a main agent delegates code
reconnaissance to. Ask it a repo question in plain English; it explores GitHub
under the hood and hands back a **schema-validated evidence report** — claims
with citations, the commands it ran, and what it could not confirm — while every
artifact of the investigation lands on disk under `~/.ghx/sessions/` for you or
the parent agent to inspect.

This is the first public cut of the **Agent Sidecar Framework**: a main agent
delegates a whole competence domain to a proficient specialist instead of loading
that domain's tools and doctrine into its own context. ghx is the proof — the
code-reconnaissance sidecar — and, underneath it, a sharp standalone GitHub
exploration CLI. The durable vision lives in
[docs/NORTH_STAR.md](docs/NORTH_STAR.md).

> ghx is two products in one binary: a **sidecar brain** you delegate to (this
> README's lead), and the **classic exploration CLI** it drives under the hood
> (documented in full further down). Use whichever fits; they share one core.

## Engineering record

ghx has been developed in public through a sequence of maintainer field notes:

- [Build the GitHub Exploration Tool, No Mistakes](https://gkoreli.com/how-ghx-was-born)
  is the origin story: 23 agent sessions, three rewrites, and the constraints that
  shaped the original CLI.
- [You Don't Always Need Codemap](https://gkoreli.com/you-dont-need-codemap)
  places ghx alongside Codemap, Aider, Gitingest, and Repomix, and explains the
  boundary between mapping, packing, searching, and remote reconnaissance.
- [What If the Agent Was Better Before We Helped?](https://gkoreli.com/what-if-the-agent-was-better-before-we-helped)
  records the sidecar thesis, the evaluation design, and the unresolved question
  of whether any of it actually beats plain `gh`.

These are engineering records, including doubts and failed paths — not product
documentation or independent reviews.

## Why a sidecar

When a main agent stops mid-task to explore code, the real cost is not the API
calls — it is the context. Exploration output floods the window, the agent loses
the thread of the engineering work, and the ~400 lines of tool doctrine it needs
to explore well are dead weight it re-pays every session. Frontier-model tokens
spent on map/grep/read loops are pure waste when a cheap specialist can do the
same reconnaissance better.

Delegating to a sidecar removes all of it from the main agent's context. The main
agent gets measurably better at *its own* objective **and** receives better
exploration answers than it would have produced itself — both, not one. The
sidecar owns the CLI grammar, the search gotchas, the map-before-read discipline,
and the backend escalation; the outside world learns one sentence: *ask ghx repo
questions in English, get evidence reports back.*

## What comes back: an evidence contract, not a string

The sidecar's answer is a validated report, not free text. The report schema is
enforced at submission time — the sidecar agent cannot finish a turn until it has
produced a schema-valid report, and it sees the exact validation error if it
fails (see [ADR-0021](docs/adr/0021-report-contract-enforcement.md)). Fields:

- **answer** — the direct answer to your question.
- **verified** — claims the sidecar backed with evidence it actually read.
- **relevantFiles / evidence** — where to look, and what each source showed.
- **uncertainty / nextReads** — what it could not confirm, and what to read next.
- **commandsRun** — the exact ghx commands behind the claims.

Because the trace is visible, the sidecar can be opinionated: every claim is
auditable against the commands and files it cites.

### Full visibility under `~/.ghx`

`~/.ghx` is the product's root storage. Every question leaves a durable,
inspectable trail in its session directory — no framework, no service, just files
you can `ls` and `jq`:

```
~/.ghx/
  config.json
  sessions/<owner-repo>/
    meta.json                 # session identity
    ledger                    # evidence accumulated across questions
    reports/<turn>-<ts>.json  # the accepted report of every question
    traces.jsonl              # OTel spans: session → turn → each tool call
    logs.jsonl                # GenAI-convention message-content records
    metrics.jsonl             # duration, token, report-size metrics
    tier-decisions.jsonl      # escalation policy evaluation per turn (which tier, why)
    live.jsonl                # realtime turn activity, appended as it happens
```

The OTLP artifacts are written when a question completes; `live.jsonl` streams
turn activity in realtime — turn start/end, text and thought chunks, every tool
call and update as the agent works — so
`tail -f ~/.ghx/sessions/<name>/live.jsonl` watches a running question instead
of waiting minutes for the final report
(see [ADR-0022.1](docs/adr/0022.1-live-turn-log.md)). `ghx sidecar tail --follow`
renders that stream as one human-readable line per event, so you rarely need
the raw `tail -f`.

Sessions persist, so follow-up questions on the same repo are cheaper and
context-aware. The traces are **official OpenTelemetry** (OTLP/JSON, GenAI
semantic conventions) — any industry tool consumes them in seconds, and
`ghx sidecar view` (below) spawns a local trace UI over a session with one
command. The runtime and the eval harness emit the *same* artifacts from the
*same* code, so a real question and a benchmark episode are inspected the same
way (see [ADR-0022](docs/adr/0022-shared-visibility-runtime.md)).

Every ask response — human output, `--json`, or the MCP recon tool — ends with
an `artifacts:` pointer naming this session directory and the ask's root trace
ID, so the calling agent never has to guess where the audit trail lives.

## How this differs from other delegation

Delegation between agents is now common; auditable delegation is not. We surveyed
the field with the sidecar itself — six real investigations, one per framework,
each with a committed report and OTel trail (see
[ADR-0026](docs/adr/0026-prior-art-landscape.md)). The finding, stated carefully:

> Across the frameworks examined, **delegation returns strings or transcripts;
> none returns a schema-validated evidence contract** (claims with citations,
> commands, uncertainty), and none gives the parent agent artifact-level
> visibility it can read without the framework, an OTel audit trail of the
> delegate, or persistent cross-question session memory it can inspect. Each
> element exists somewhere in embryo; the *combination* — auditable, steerable,
> persistent, evidence-bearing delegation as a product — had no found occupant.

This claim is deliberately scoped: it is based on six repositories examined on one
evening, the landscape moves monthly, and it is re-scanned on that cadence. We
credit the prior art we learned from and, where useful, plan to absorb it (see
the ADR's steal list and attributions).

## How we know it works: pre-registered evals

Quality is measured, not asserted. The methodology lives in `docs/evals/`:

- **Pre-registered gates.** Correctness, evidence, compression, memory, and
  safety thresholds are decided *before* a run; an under-sampled run self-labels
  PRELIMINARY and cannot feed a go/no-go decision.
- **Deterministic scoring.** Every score recomputes from committed episode
  artifacts — no hidden state, no live model in the loop at scoring time.
- **Independent cross-family audit.** A different model family (OpenAI Codex, run
  read-only) adversarially recomputed **all 90 episode scores of the confirmatory
  run exactly** and independently confirmed the gate outcomes — and logged the
  provenance defects it found, which are tracked in the open. See
  [the audit report](docs/evals/audit-2026-07-05-independent/REPORT.md).

The committed verdict for the confirmatory run is **THESIS SUPPORTED**
([verdict](docs/evals/gate-run-2026-07-05-confirmatory/verdict.md),
[run dir](docs/evals/gate-run-2026-07-05-confirmatory/)). We link the verdicts
rather than headline numbers; the numbers are read as a conservative floor and
the artifacts are the proof either way. The honest negative that drove the fixes
is kept alongside it at
[docs/evals/gate-run-2026-07/](docs/evals/gate-run-2026-07/).

## Sidecar quickstart

### 1. Install

```bash
# Zero install — just run it
npx @gkoreli/ghx sidecar doctor

# Homebrew
brew install gkoreli/tap/ghx

# npm (global)
npm install -g @gkoreli/ghx

# Go
go install github.com/gkoreli/ghx/v2/cmd/ghx@latest
```

The sidecar drives an ACP-capable agent under the hood, and it reads GitHub
through the authenticated [gh CLI](https://cli.github.com/) (`gh auth login`).

### 2. Configure the agent (optional — zero-config works)

The sidecar speaks [Agent Client Protocol (ACP)](https://agentclientprotocol.com)
to its underlying model. **With no config at all, the pinned
[claude-agent-acp](https://www.npmjs.com/package/@agentclientprotocol/claude-agent-acp)
adapter is the default** — if you have Node/npx and are logged into Claude Code,
`ghx sidecar ask` works out of the box. Write an explicit config only to pin a
different agent or model:

```bash
ghx sidecar config init --claude-acp    # writes ~/.ghx/config.json with the ACP adapter
```

> Without `--claude-acp`, `config init` auto-detects any ACP-capable agent
> already on your PATH; a bare `claude` binary does **not** speak ACP on stdio
> and is rejected on purpose (it would hang every turn — see
> [ADR-0019](docs/adr/0019-sidecar-adoption-zero-cli-surface.md) D4).

Re-running `config init --claude-acp` against an existing config shows a field
diff and refuses to overwrite without `--force`.

**Using your installed Claude Code (toolbox / wrapper builds).** The adapter's
SDK ships its own bundled Claude Code binary and its own credential store — by
default your `claude` on PATH (and its `~/.claude/settings.json`, credential
hooks, model aliases) is **not** what runs. If your organization installs a
wrapped `claude` that carries work credentials, point the adapter at it with
`CLAUDE_CODE_EXECUTABLE` (honored by `claude-agent-acp` ≥ 0.55):

```json
{
  "agent": "env CLAUDE_CODE_EXECUTABLE=/path/to/your/claude npx -y @agentclientprotocol/claude-agent-acp@0.55.0"
}
```

`ghx sidecar config init --claude-exe /path/to/your/claude` writes exactly this
config for you (`--claude-exe auto` resolves `claude` from your PATH; the same
diff-and-`--force` rules apply).

> **Known upstream issue (enterprise wrapper builds):** when
> `CLAUDE_CODE_EXECUTABLE` points at a *wrapper* script/binary (e.g. a
> corporate toolbox `claude` that resolves credentials itself), the adapter
> can hang indefinitely at prompt time: ghx legitimately sends ACP
> `settingSources: []` for session isolation, and some wrappers stall instead
> of erroring when settings loading is suppressed (isolated 2026-07-06:
> `settingSources: []` alone reproduces the hang; `strictMcpConfig` and
> `tools` do not; mechanism: the native binary resolves Bedrock
> `modelOverrides` from `~/.claude/settings.json` and stalls instead of
> erroring when that read is suppressed). The fix is one config line:
>
> ```json
> {
>   "agent": "env CLAUDE_CODE_EXECUTABLE=/path/to/your/claude npx -y @agentclientprotocol/claude-agent-acp@0.55.0",
>   "agentSettingSources": ["user"]
> }
> ```
>
> `agentSettingSources` opts the spawned agent into loading your user
> settings (`~/.claude/settings.json`) so the wrapper's model aliases and
> credential hooks work. It deliberately weakens session isolation — host
> user settings apply to sidecar sessions — which is why it is opt-in and
> off by default (ADR-0033.2). Wrapping the *adapter* in a script that
> exports auth env itself remains a valid alternative.

Embedding `env VAR=...` in the agent command makes the setting travel with the
command itself, so it works no matter which process (CLI or resident daemon)
spawns the agent. Exporting `CLAUDE_CODE_EXECUTABLE` in your shell also works:
every ask forwards auth-relevant env to the agent
([ADR-0033.1](docs/adr/0033.1-client-auth-env-passthrough.md)), and the same
applies to Bedrock/Vertex/gateway env (`CLAUDE_CODE_USE_BEDROCK`, `AWS_*`,
`ANTHROPIC_BASE_URL`, ...). `ghx sidecar doctor` shows exactly which names your
shell would forward.

### 3. Verify

```bash
ghx sidecar doctor           # token, network, ghx binary, ACP handshake, report sink
ghx sidecar doctor --live    # also run a REAL one-prompt turn through the agent
```

`doctor` fails fast with an actionable message if the configured agent cannot
complete the ACP handshake, and it prints where session artifacts live and how to
replay them. It also verifies that the binary serving the report-sink MCP server
matches the running ghx version — a stale sink binary would silently degrade
structured reports to a text fallback, so doctor fails loudly with fix-it text
instead.

`--live` goes one step further: the plain handshake stops at ACP *initialize*,
which passes even on setups where a real turn then fails (auth typically
resolves lazily at prompt time). `--live` runs a full `session/new` + one tiny
prompt turn through the configured agent and, on failure, prints the failing
**stage** and the agent's **stderr tail** — the deterministic way to diagnose a
machine where `ask` produces nothing (see [ADR-0033](docs/adr/0033-sidecar-out-of-box-agent-config.md)).

### 4. Ask

```bash
ghx sidecar ask --repo hono/hono "How does Hono implement middleware chaining, and which files define it?"
```

- `--repo owner/repo` is optional scope. With it: repo-scoped reconnaissance.
  Without it: **discovery** — "which repos/libraries do X" — the sidecar sweeps
  GitHub for candidates and reads into the top ones before claiming anything
  (see [ADR-0019.1](docs/adr/0019.1-discovery-tier.md)).
- `--session <name>` is advanced: pin a specific session; normally omit — ghx
  routes each question to the right session for you
  ([ADR-0030.1](docs/adr/0030.1-session-routing.md)). With `--repo` the
  repo-slug session (like `hono-hono`) still stands; with neither flag the
  daemon runs a deterministic cascade (explicit → repo mention in the question
  → warm continuation → ledger overlap → new session) and reports the route it
  chose: `session: hono-hono (routed: overlap 0.62, next 0.21)`.
- `--json` prints an envelope — `{"report": {...}, "artifacts": {"sessionDir":
  "...", "traceId": "..."}}` — the validated report plus a pointer to the
  session's audit trail. Without it you get the answer plus compact verified /
  relevant-files / uncertainty sections, ending with the same pointer as a
  footer line: `artifacts: <session dir> (trace <root trace id>)`.
- `--depth cheap|normal|deep` sets the command budget for deeper repos.

Ask by stating the goal, not the steps. Follow-ups on the same repo reuse the
session automatically and answer faster. A fresh question takes tens of seconds.

### 5. Inspect

```bash
ghx sidecar sessions list                 # all sessions
ghx sidecar sessions show <session>       # details + report history
ghx sidecar sessions ledger <session>     # the accumulated evidence ledger
ghx sidecar sessions reroute <s> <turn> <dest>  # move a mis-routed turn; both ledgers rebuilt by replay
ghx sidecar tail [session] --follow       # human-readable live view of a running question
ghx sidecar view [session]                # spawn a local trace UI over the session's artifacts
ghx sidecar view --list                   # sessions with turn/report counts
ghx sidecar view --port 9000              # viewer UI port (default 8000)
```

`ghx sidecar view` absorbs the
[otel-desktop-viewer](https://github.com/CtrlSpice/otel-desktop-viewer) replay
recipe into one command: it starts the viewer and loads the session's
`traces.jsonl` (plus logs/metrics when present) for you — no argument means the
most recent session. Everything it shows is the same file you can read by hand.
Requires the viewer on PATH:
`go install github.com/CtrlSpice/otel-desktop-viewer@latest`.

### Troubleshooting

If an `ask` returns nothing useful, the failure is recorded, not lost
([ADR-0033](docs/adr/0033-sidecar-out-of-box-agent-config.md)):

- **Per-session agent stderr.** The spawned agent's stderr is captured to
  `~/.ghx/sessions/<session>/agent-stderr.log`. Under the always-on daemon it
  no longer disappears into the daemon's own log. A failed turn also splices the
  tail of that log — and its path — into the error the CLI prints.
- **Reproduce it deterministically.** `ghx sidecar doctor --live` runs a real
  prompt turn and prints the failing stage plus the agent's stderr, even when the
  plain handshake passes.
- **`this workspace has not been trusted`.** ghx already runs the agent in a
  neutral, ghx-owned session directory (outside any git repo, no `.claude`
  settings), so this warning should not appear. If it still does, the agent
  picked up a git-repo workspace: trust it once (run the agent interactively
  there and accept the dialog) or set
  `projects["<dir>"].hasTrustDialogAccepted: true` in `~/.claude.json`. ghx will
  never write that file for you.
- **"works from shell A, fails from shell B".** The daemon auto-spawns from the
  **first** caller's environment, so a missing auth/proxy env var is a common
  cause. `ghx sidecar sessions show <session>` lists the agent command, the
  session's spawn cwd, and the **names** (never values) of the agent-relevant
  environment variables present when it was created. If the wrong env was
  inherited, `ghx sidecar daemon --stop` and re-run `ask` from a shell that has
  the right variables.

### Delegating from a main agent (MCP)

A main agent talks to the sidecar through a single MCP tool — one tool it cannot
be tempted into step-driving:

```bash
ghx serve --recon     # exposes exactly one MCP tool: recon(question, repo?, session?)
```

The recommended skill for agents is the concise recon skill (`ghx skill --recon`,
≤ 30 lines: what the service is, how to phrase questions, what the report fields
mean). The main agent needs **zero** knowledge of the ghx CLI grammar — that is
the whole point ([ADR-0019](docs/adr/0019-sidecar-adoption-zero-cli-surface.md)).

---

# The classic ghx CLI (the tool layer)

Under the sidecar brain is a sharp, standalone GitHub exploration CLI — the #1
direct tool for reconnaissance, and the eval baseline the sidecar is measured
against. Use it directly when you want to drive exploration yourself. **One
command does what takes 3–5 API calls.** Batch file reads, structural code maps,
and search — all via the `gh` CLI, or composed in a single round-trip with
codemode.

### Why it's faster than raw `gh`

An agent wants to understand `packages/shadcn/src/utils/` in
[shadcn-ui/ui](https://github.com/shadcn-ui/ui):

**With `gh` CLI** — 4 turns, 4 API calls, reads 3 full files (3,761 tokens), sees 3 of 34 files:
```
gh api /repos/shadcn-ui/ui/contents/packages/shadcn/src/utils       → JSON with shas, urls, links (480 tokens for a file list)
gh api /repos/.../get-config.ts --jq '.content' | base64 -d         → full file (1,981 tokens — agent only needed exports)
gh api /repos/.../registries.ts --jq '.content' | base64 -d         → full file (676 tokens)
gh api /repos/.../frameworks.ts --jq '.content' | base64 -d         → full file (624 tokens)
```

**With ghx** — 2 turns, 2 API calls, maps 10 files (3,058 tokens), sees signatures of all 10:
```
ghx read shadcn-ui/ui packages/shadcn/src/utils                     → directory listing (199 tokens)
ghx read shadcn-ui/ui "packages/shadcn/src/utils/*.ts" --map        → signatures of 10 files (2,859 tokens)
```

Same token budget. The `gh` agent read 3 full files; the ghx agent saw the
structure of 10 — imports, exports, signatures — and knows which to drill into.
Pass a file, get content. Pass a directory, get a listing. Pass a glob, get
matching files. Same command, always useful output.

| Tool | Files per call | Matching context | Programmable | Dependencies |
|------|---------------|-----------------|-------------|-------------|
| GitHub MCP | 1 | No | No (~10K token schemas) | Go binary |
| `gh` CLI | 1 | No | No (exact phrase, base64, no README) | `gh` |
| **ghx** | **1-10 (batch)** | **Yes** | **Yes (codemode)** | **`gh`** |

### Commands

```bash
ghx --version                               # Health check / installed version (also: ghx version)
ghx explore <owner/repo>                    # Compact branch + tree + README orientation
ghx explore <owner/repo> --full             # Complete legacy explore output
ghx explore <owner/repo> <path>             # Compact subdirectory listing
ghx explore <owner/repo> --budget 20000     # Raise the compact output budget
ghx read <owner/repo> <f1> [f2] [f3]        # Read 1-10 files (large files map first)
ghx read <owner/repo> <dir>                 # Directory path → returns file listing
ghx read <owner/repo> "src/**/*.ts" --map   # Glob patterns with structural map
ghx read <owner/repo> --map <f1> [f2]       # Parser-backed structural map (~92% token reduction)
ghx read <owner/repo> --map --kind func <f> # Map only functions/methods
ghx read <owner/repo> --map --kind type <f> # Map only types/structs/interfaces
ghx read <owner/repo> --map --level minimal <f> # Symbol names only (e.g. UserService.GetUser)
ghx read <owner/repo> --grep "pat" <f>      # Matching lines only (ERE regex, 2 lines context)
ghx read <owner/repo> --lines 42-80 <f>     # Canonical line range
ghx read <owner/repo> <f> --full            # Force complete content past the budget
ghx search <owner/repo> "func main"         # Repo-first code search, auto-quotes the query
ghx search <owner/repo> "router" --lang go  # Narrow by language
ghx search <owner/repo> "router" --glob "**/*.go" # Narrow by path glob
ghx search "repo:gkoreli/ghx cobra.Command" # Advanced raw GitHub code-search query
ghx inspect <owner/repo> "routing middleware" # Concern-driven search + ranked files, maps, snippets
ghx grep <owner/repo> "func main"           # Grep-like repo search
ghx grep <owner/repo> "RunE" --path internal/cli --limit 10
ghx repos "<query>"                         # Repo search with README preview
ghx tree <owner/repo> [path]                # Full recursive tree
ghx tree <owner/repo> [path] --depth N      # Tree limited to N levels
ghx code "<js>"                             # Execute JS with access to all ghx tools
ghx code --list                             # List available tools with type stubs
ghx serve                                   # Start the direct-tools MCP server (stdio)
ghx serve --http :8080                      # Serve MCP over streamable HTTP
ghx serve --recon                           # Start the single-tool recon MCP service
ghx skill                                   # Print the classic CLI skill
ghx skill --mcp                             # Print the classic MCP skill
ghx skill --recon                           # Print the concise recon skill
ghx sidecar config init --claude-acp        # Write ~/.ghx/config.json for Claude ACP
ghx sidecar config show                     # Show resolved sidecar config
ghx sidecar doctor                          # Verify token, network, ghx binary, ACP, report sink
ghx sidecar doctor --live                   # Run a real one-prompt ACP turn for diagnosis
ghx sidecar ask --repo <owner/repo> "<q>"   # Delegate one repo question to the sidecar
ghx sidecar ask "<q>"                       # Discovery question across GitHub
ghx sidecar ask --repo <o/r> --local "<q>"  # Also allow tier-2 local analysis (codemap/ast-grep/repomap)
ghx sidecar daemon --status                 # Inspect the warm sidecar daemon
ghx sidecar daemon --stop                   # Stop the warm sidecar daemon
ghx sidecar sessions list                   # List persisted sidecar sessions
ghx sidecar sessions show <session>         # Metadata + report history
ghx sidecar sessions ledger <session>       # Accumulated evidence ledger JSON
ghx sidecar sessions reroute <s> <turn> <dest> # Move a mis-routed turn and rebuild ledgers
ghx sidecar tail [session] --follow         # Human-readable live view of turn activity (live.jsonl)
ghx sidecar view [session]                  # Local OTel viewer over session artifacts
ghx sidecar evals export --format sft --run <dir> --out <file> # Export committed eval episodes
ghx sidecar report-sink --out <path>        # Internal report-sink MCP server used by ACP turns
ghx tier2 codemap <owner/repo>              # Tier-2 cross-file structure as JSON (local snapshot)
ghx tier2 codemap <owner/repo> --context    # Agent-ready JSON context envelope (--compact to shrink)
ghx tier2 codemap <owner/repo> --importers <file>  # Who imports a file (fan-in; needs ast-grep)
ghx tier2 codemap <owner/repo> --deps       # Dependency flow / import chains (needs ast-grep)
ghx tier2 codemap <owner/repo> --ref v1.2.3 --sparse src  # Pin a ref, materialize only candidate paths
ghx tier2 astgrep <owner/repo> --pattern 'compose($$$A)' --lang ts  # Structural AST pattern search (JSON matches)
ghx tier2 astgrep <owner/repo> --pattern 'errors.Is($E, $T)' --lang go src  # Scope the search to paths
ghx tier2 repomap <owner/repo>              # Rank the files that matter (graph centrality, token budget)
ghx tier2 repomap <owner/repo> --query route --budget 512  # Bias ranking to the question, cap output tokens
```

`read` documents one range spelling, `--lines START-END`, but accepts hidden
agent-guess aliases: `--start N --end M` and `--offset N --limit M`. They work
for frictionless retries and stay out of help so examples converge on one form.

Output-heavy commands share `--budget CHARS` and `--full`: defaults are bounded,
and truncation text names the exact flag to lift or narrow the result.

Exit codes are semantic: `0` ok, `1` no results, `2` bad invocation, `3`
upstream/API failure.

`tier2` commands are the deliberate, visible escalation beyond remote evidence
([ADR-0024.1](docs/adr/0024.1-escalation-tiers-decision.md)): the repo/ref is
resolved to a commit SHA and materialized as a shallow, blobless, read-only
snapshot cached by SHA under `~/.ghx/cache/tier2` (`$GHX_HOME` respected;
5 GiB budget, 30-day LRU/TTL eviction). Snapshot provenance — repo, ref,
resolved SHA, clone strategy, cache hit — prints to stderr before any tool
output, and structural tools run as absorbed backends under canonical IDs:
`local:codemap` (subprocess, via
[codemap](https://github.com/JordanCoin/codemap), MIT), `local:ast-grep`
(subprocess, via [ast-grep](https://github.com/ast-grep/ast-grep), MIT — also
the binary codemap needs for `--importers`/`--deps`), and `local:repomap`
(built in: the budgeted definition/reference/import-graph PageRank ranking
stolen with attribution from
[aider's repo map](https://github.com/Aider-AI/aider), Apache-2.0 — an
algorithm, not a dependency; deterministic given the same snapshot and
query). Only `local:repomap` works with zero extra installs; codemap and
ast-grep are optional external binaries, and `ghx sidecar doctor` reports
which tier-2 backends resolve on your machine with install hints for the
missing ones. If a tool binary is missing, ghx prints the install hint and
exits before any clone; answers fall back to remote Tier-1 evidence — never
a silent or faked Tier-2 result. `astgrep` follows grep parity: exit `1` with
`[]` means the search ran and found nothing.

When the sidecar answers a question, escalation is never vibes: every turn
gets a declarative policy evaluation
([ADR-0024.2](docs/adr/0024.2-escalation-policy.md)) over named observables
(question shape, exhausted searches, low-confidence remote-only reports),
recorded as a `ghx.tier.decision` span in `traces.jsonl`, a line in the
session's `tier-decisions.jsonl`, and `tierUsed` provenance on the report —
so "which tier answered and why" is always recomputable from the artifacts.

### Codemode

Write JS programs that compose multiple operations in one round-trip. All
`codemode.*` calls are synchronous — no `await`. Full TypeScript type stubs with
return types are injected into the sandbox.

```bash
# What branch is this repo on?
ghx code 'var r = codemode.explore({repo: "vercel/next.js"}); return r.branch;'

# Search + read composition
ghx code 'var hits = codemode.search({query: "useState repo:vercel/next.js", limit: 3});
var first = codemode.read({repo: "vercel/next.js", files: [hits.matches[0].path]});
return {file: hits.matches[0].path, lines: first[0].content.split("\n").length};'

# See what tools and types are available
ghx code --list
```

Type stubs tell the LLM exactly what fields exist — no guessing:

```typescript
declare const codemode: {
  explore: (input: ExploreInput) => { description: string; branch: string; files: { name: string; type: string }[]; readme: string };
  search: (input: SearchInput) => { total: number; incomplete: boolean; matches: { repo: string; path: string; fragment: string }[] };
  tree: (input: TreeInput) => string[];
  // ...
}
```

### MCP server (direct tools)

```bash
ghx serve                                   # stdio (for Claude, Cursor, etc.)
ghx serve --http :8080                      # HTTP transport
ghx serve --recon                           # single-tool sidecar mode (see above)
```

Default mode exposes 7 tools: `explore`, `read`, `search`, `repos`, `tree`,
`code` (meta-tool), `search_tools`.

```json
{
  "mcpServers": {
    "ghx": {
      "command": "npx",
      "args": ["@gkoreli/ghx", "serve"]
    }
  }
}
```

No install step — npx downloads and caches the binary on first run.

### Agent skills

```bash
ghx skill                                   # classic CLI skill (power-user path)
ghx skill --mcp                             # classic MCP skill
ghx skill --recon                           # concise recon skill (recommended for agents)
```

```bash
npx skills add gkoreli/ghx -g -a claude-code --skill ghx -y
npx skills add gkoreli/ghx -g -a claude-code --skill ghx-mcp -y
```

## How it works

The classic CLI wraps `gh` with GraphQL batching. `repos` and `explore` batch
search + metadata + README into 1 call. `read` uses GraphQL aliases to fetch up
to 10 files in 1 call — and if a path is a directory, returns its listing instead
of "not found" (via `... on Tree` inline fragments, zero extra API calls). Glob
patterns (`src/**/*.ts`) auto-expand via tree fetch +
[doublestar](https://github.com/bmatcuk/doublestar) matching in 2 API calls.
`--grep` uses ERE regex with BRE normalization. `search` hits REST
`/search/code` with `text_matches` for matching context.

`--map` runs a dedicated parser engine on the fetched content — no extra API
calls. Engine selection is automatic: **Go** uses `go/ast`; **TypeScript,
JavaScript, Python, Rust** use tree-sitter (via
[gotreesitter](https://github.com/odvcencio/gotreesitter), capturing class/impl
methods regex cannot reach); everything else falls back to regex. Codemode runs
JS in a [goja](https://github.com/nicholasgasior/goja) sandbox with esbuild
TypeScript transpilation (max 20 tool calls, 64 KB code limit).

The **sidecar** wraps that same core behind an ACP agent that owns the
exploration doctrine, validates its own evidence report before finishing
([ADR-0021](docs/adr/0021-report-contract-enforcement.md)), and emits OTel
artifacts to `~/.ghx/sessions/`
([ADR-0022](docs/adr/0022-shared-visibility-runtime.md)).

## Architecture

```
cmd/ghx/             — binary entrypoint
internal/cli/        — CLI frontend (cobra) + MCP + sidecar commands
internal/ghx/        — core library (Explore, Read, Search, Repos, Tree, Glob)
internal/codemode/   — JS executor (goja sandbox, TS transpilation, type generation)
internal/mapengine/  — parser-backed map engine (GoAST, TreeSitter, Regex, routing)
internal/sidecar/    — sidecar runtime, sessions, report contract, ACP integration
internal/sidecar/telemetry/ — shared OTel emission (runtime + evals, one capability)
skills/              — CLI, MCP, and recon agent skills, embedded via go:embed
```

Core capabilities live in `internal/ghx`; every frontend — CLI, MCP, codemode,
sidecar — wraps the same core. See [docs/adr/](docs/adr/) for the full decision
record and [docs/NORTH_STAR.md](docs/NORTH_STAR.md) for direction.

## Built on open source, openly

ghx exists because exploring open source for ideas is where good products come
from — "good artists copy, great artists steal," and we steal in the Picasso
sense: openly, with attribution, and with stewardship for future generations. The
rule we build by: **use official standards and existing open-source tools,
libraries, and ideas first; hand-roll a framework or format only when nothing
existing serves the need or the vision.**

- Agent traces are **official OpenTelemetry** (OTLP/JSON) following the
  [OTel GenAI semantic conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/),
  so any industry tool — Jaeger, the collector,
  [otel-desktop-viewer](https://github.com/CtrlSpice/otel-desktop-viewer) — reads
  them in seconds.
- The sidecar speaks the [Agent Client Protocol (ACP)](https://agentclientprotocol.com)
  via the
  [claude-agent-acp](https://www.npmjs.com/package/@agentclientprotocol/claude-agent-acp)
  adapter — an adopted boundary, never a bespoke wire protocol.
- Parsing rides on [gotreesitter](https://github.com/odvcencio/gotreesitter) and
  `go/ast`; globbing on [doublestar](https://github.com/bmatcuk/doublestar);
  codemode on [goja](https://github.com/nicholasgasior/goja); GitHub access on
  the [gh CLI](https://cli.github.com/).

Where we do build new — ghx and the Agent Sidecar Framework — it is because the
thing did not exist, and it is inspired loudly by what does. Tools like
[codemap](https://github.com/JordanCoin/codemap) and
[ast-grep](https://github.com/ast-grep/ast-grep) shape the sidecar's internal
toolbox: `ghx tier2 codemap` and `ghx tier2 astgrep` absorb them as internal
subprocess backends (`local:codemap`, `local:ast-grep`, ADR-0024.1) rather
than competing on CLI surface (see
[ADR-0026](docs/adr/0026-prior-art-landscape.md)), and `ghx tier2 repomap`
steals the budgeted graph-ranking algorithm behind
[aider's repo map](https://github.com/Aider-AI/aider) without the dependency.
MIT in, MIT out.

## How it was built

23 agent sessions, 2,500+ conversation turns, 3 rewrites, and a growing ADR
record. The origin story:
**[Build the GitHub Exploration Tool, No Mistakes](https://gkoreli.com/how-ghx-was-born)**

## License

MIT
