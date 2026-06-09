---
title: "ghx Sidecar Agentic Eval and Training Data Architecture"
date: 2026-05-13
status: Decided
depends_on: ADR-0015
---

# 0016. `ghx` Sidecar Agentic Eval and Training Data Architecture

## Context

ADR-0015 moved the `ghx` sidecar runtime from TypeScript/acpx into the Go `ghx`
binary using `coder/acp-go-sdk`. That migration solves the production runtime
problem, but it leaves a second question open: how should `ghx` evaluate whether
the sidecar architecture is actually better than direct agent exploration?

The evaluation target is not a generic LLM answer. The target is an agentic
repository-reconnaissance workflow:

1. A user asks a high-level repo question.
2. An agent decides which tools to use.
3. The agent reads/searches a remote GitHub repo.
4. The agent emits an answer or a structured sidecar report.
5. Follow-up turns should reuse prior findings instead of re-researching from scratch.

The long-term goal is larger than manual benchmarking. The evaluation environment
should become a durable source of training data for a future dedicated `ghx`
sidecar model: trajectories, observations, reports, reward breakdowns, and
OpenTelemetry traces.

## North Star

`ghx` should accumulate a gold mine of high-quality agent trajectories for GitHub
repository reconnaissance.

The sidecar is not only a feature. It is the environment in which a future
specialized model learns to operate:

```
repo + question + prior session memory + ghx tools
  -> actions: ghx explore/search/read/tree/code or stop/report
  -> observations: command output, errors, prior reports
  -> reward: correctness, evidence, trajectory, compression, memory, safety
  -> artifact: compact ghx-report for the main engineering agent
```

Today, the agent brain is a general model reached through an ACP adapter. Later,
the agent brain can be a trained `ghx-sidecar` model exposed through the same ACP
boundary or a direct local runner. The evaluation environment should not care which
brain is behind the policy as long as it can run the episode and capture the same
episode schema.

## Decision

Build a Go-native, development-only agentic eval suite for `ghx` sidecar.

The suite compares exactly three profiles:

| Profile | Meaning | Purpose |
|---------|---------|---------|
| `plain` | General agent without `ghx`; may use shell and `gh` CLI | Baseline for what a capable coding agent can do with common tools |
| `ghx` | General agent with `ghx` skill/instructions and direct `ghx` tool usage | Measures value of `ghx` as a repository exploration tool |
| `ghx-sidecar` | Main agent delegates reconnaissance to `ghx sidecar` and receives a structured report | Measures value of the sidecar boundary: compression, memory, evidence, and reduced main-agent context burden |

The suite is not a user-facing benchmark CLI. It is an e2e/eval test suite run
manually by engineers.

```
go test ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes
```

Multiple trials use normal Go test repetition:

```
go test ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes -count=5
```

## Architecture

```
internal/sidecar/evals/
  episode.go        – Task, Episode, Turn, Action, Observation, RewardBreakdown
  profiles.go       – plain, ghx, ghx-sidecar profile definitions
  runner.go         – ACP-backed episode runner using coder/acp-go-sdk
  rewards.go        – deterministic ghx-specific checklist rewards
  judge.go          – optional LLM-as-judge over captured episodes
  otel.go           – local OpenTelemetry span emission for episodes/actions/rewards
  store.go          – local JSON/JSONL/Markdown run artifacts
  reporters.go      – local JSON, JSONL, Markdown, and OTel reporters
  eval_test.go      – manual e2e entrypoint
  testdata/
    tasks/
```

The eval package owns the `ghx`-specific environment and reward schema. It does
not outsource the environment to Braintrust, LangSmith, Genkit, Eino, or another
agent framework.

### Agent Brain

The "agent brain" is the policy that chooses the next action in an episode.

Today, the brain is provided by a configured ACP agent adapter:

```
ghx eval runner
  -> coder/acp-go-sdk ClientSideConnection
    -> ACP adapter subprocess
      -> Codex / Claude / other local agent runtime
```

This means the eval suite does not require raw OpenAI, Anthropic, or model API keys.
It uses the engineer's locally configured agent runtime, exactly like the Go sidecar
runtime does.

If the configured ACP adapter needs authentication, that authentication belongs to
the adapter/runtime. Examples:

- Codex ACP adapter may use the engineer's existing Codex login/session.
- Claude ACP adapter may use the engineer's existing Claude/Anthropic configuration.
- A future local `ghx-sidecar` model adapter may need no cloud API key at all.

The eval suite itself only needs an ACP command to spawn.

### API Keys

The eval suite itself should require no eval-platform API key.

Local evals should work with no Braintrust account, LangSmith account, hosted eval
platform, or eval-service credential:

```
go test ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes
```

This writes local artifacts:

```
.ghx-evals/runs/<run-id>/
  manifest.json
  plain/episode.json
  ghx/episode.json
  ghx-sidecar/episode.json
  result.json
  result.md
  traces.jsonl
```

The only authentication involved is whatever the configured ACP agent adapter needs
to run its model/runtime. That authentication belongs to the adapter, not to the
eval framework.

## Why OpenTelemetry

OpenTelemetry is the portable trace format. It is not the eval framework and it
does not judge episodes. It gives `ghx` a standard way to store and export the
timeline of an episode without depending on a paid service or hosted dashboard.

The eval runner should emit spans for:

- episode start/end
- profile run
- ACP session initialization
- prompt turn
- tool/action event
- observation capture
- report extraction
- reward computation
- judge scoring

This creates an execution graph that can be:

- inspected locally,
- stored as local JSON/JSONL trace artifacts,
- exported to another OpenTelemetry-compatible platform,
- replayed into future analysis tooling,
- joined with episode JSON for training data generation.

## Why Not Braintrust Now

Braintrust has useful ideas and a real Go SDK, but it does not meet the current
`ghx` requirement: a local-first, Go-native, no-service, no-eval-platform-key
benchmark and training-data foundation.

Braintrust should be treated as design inspiration and a possible future exporter,
not as a selected dependency.

Useful concepts to borrow:

- dataset/case separation
- task/runner abstraction
- scorer abstraction
- experiment result object
- pluggable reporters
- trace integration

## Evaluated Alternatives

### Braintrust Go SDK

Not selected as a dependency.

Pros:

- Native Go SDK.
- Supports evals, custom scorers, datasets, experiments, and tracing.
- Uses OpenTelemetry.
- Integrates with Go AI stacks such as OpenAI Go, Anthropic, Genkit, ADK, Eino,
  and LangChainGo.
- Strong conceptual model: dataset, task, scorer, evaluator, experiment.

Cons:

- Beta API.
- Cloud reporting requires `BRAINTRUST_API_KEY`.
- Does not provide the local-first, no-service Go eval kernel `ghx` needs.
- Platform concepts are not the right foundation for future local training data.

Decision:

- Borrow the abstractions.
- Do not add the dependency.
- Keep a future `BraintrustReporter` possible if the project later wants hosted
  experiment dashboards.

### Genkit Go Evaluation

Not selected as the core harness.

Genkit has Go evaluation support, datasets, evaluator plugins, CLI eval commands,
and trace extraction. It is a good fit when the application is already built as
Genkit flows.

`ghx` is not a Genkit app. The sidecar runtime is ACP-backed and intentionally
Go-native inside the `ghx` binary. Adopting Genkit would bend the sidecar around a
different application framework.

### trpc-agent-go Evaluation

Not selected as the core harness.

`trpc-agent-go` has Go agent and evaluation abstractions, including evaluator
interfaces over actual and expected invocations. It is useful evidence that Go
agent evaluation is emerging, but adopting it would mean adopting a broad agent
framework that `ghx` does not otherwise need.

### Evalaf / Anteval

Not selected as the core harness.

Evalaf is close to a "promptfoo for Go": embeddable eval runner, datasets, custom
evaluators, reports, RAG and agent evaluators, and minimal core dependencies. It is
promising, but it is immature for this role: no stable tagged version on pkg.go.dev,
module path confusion, and no ACP-specific fit.

It may be useful later as inspiration for local report formatting or evaluator
interfaces, but it should not be the foundation of the sidecar eval suite.

### LangSmith Go / OpenTelemetry

Not selected as primary reporting.

LangSmith supports Go tracing through OpenTelemetry and can evaluate OTel traces.
It is strongest for LangChain/LangGraph ecosystems. Since `ghx` is not built on
LangChain and does not want a hosted eval dependency, LangSmith should be a future
exporter option only.

### Eino

Not selected.

Eino is a serious Go LLM application framework with ADK, orchestration, callbacks,
agent tooling, and lifecycle tooling. It is too broad for `ghx` sidecar evals. The
sidecar should remain an ACP client and should not migrate into another agent
orchestration framework.

## Episode Schema

The local artifact is the durable source of truth. OpenTelemetry traces are
diagnostics, replay data, and future exporter input.

```go
type Episode struct {
    ID           string
    Task         Task
    Profile      Profile
    Turns        []Turn
    Actions      []Action
    Observations []Observation
    Report       *sidecar.Report
    Rewards      RewardBreakdown
    StartedAt    time.Time
    EndedAt      time.Time
}
```

The serialized JSON should be stable enough to become training data.

```json
{
  "id": "express-routing/ghx-sidecar/2026-05-13T10-00-00Z",
  "task": {
    "id": "express-routing",
    "repo": "expressjs/express",
    "turns": ["Where is route dispatch implemented?"],
    "expectedFiles": ["lib/router/index.js", "lib/router/route.js"],
    "requiredClaims": ["dispatch walks matching layers"]
  },
  "profile": "ghx-sidecar",
  "actions": [
    {"type": "tool", "name": "ghx search", "input": "..."}
  ],
  "observations": [
    {"actionIndex": 0, "text": "..."}
  ],
  "report": {
    "answer": "...",
    "verified": [],
    "evidence": []
  },
  "rewards": {
    "correctness": 0.8,
    "evidence": 0.9,
    "trajectory": 0.7,
    "compression": 0.95,
    "memory": 0.6,
    "safety": 1.0,
    "overall": 0.825
  }
}
```

## Reward Model

Start with checklist rewards. They are easier to trust than a single LLM judge
score and are closer to the current direction of multi-turn agent RL research.

| Reward | Signal |
|--------|--------|
| `correctness` | expected files, expected symbols, required claims |
| `evidence` | claims cite inspected files, commands, or report evidence |
| `trajectory` | efficient search/read order, low duplicate reads, low irrelevant scope |
| `compression` | main-agent context is smaller than direct exploration while retaining answer quality |
| `memory` | follow-up turns reuse prior findings and avoid re-reading the same files |
| `safety` | no writes, no destructive commands, denied permission requests |

LLM-as-judge is optional and secondary. It can explain qualitative differences, but
it must not be the only pass/fail gate.

## Training Data Trajectory

The eval suite should support a future model training loop without redesign:

1. Run many episodes across the three profiles.
2. Store every action, observation, report, and reward breakdown.
3. Select high-reward `ghx-sidecar` episodes as supervised fine-tuning examples.
4. Generate preference pairs from multiple trajectories for the same task.
5. Export JSONL for external training frameworks.
6. Train a dedicated `ghx-sidecar` model.
7. Expose the trained model through ACP or a direct local runner.
8. Re-run the same eval suite against the trained brain.

The eval runner should therefore support export formats:

```
.ghx-evals/exports/sft.jsonl
.ghx-evals/exports/preferences.jsonl
.ghx-evals/exports/rewards.jsonl
.ghx-evals/exports/otel.jsonl
```

## Relationship To ACP

ACP remains the runtime bridge for agent brains.

The eval suite should reuse the same Go ACP runtime code path as `ghx sidecar` as
much as possible:

```
eval runner
  -> profile prompt
  -> coder/acp-go-sdk
  -> configured ACP agent
  -> events/actions/observations
  -> episode + local OTel traces + rewards
```

This keeps the benchmark honest. It evaluates the same class of runtime behavior
that users actually exercise through `ghx sidecar`.

## Consequences

### Positive

- `ghx` remains Go-native.
- No TypeScript eval package is needed long term.
- Local evals do not require hosted eval services or eval-platform API keys.
- OpenTelemetry keeps traces portable without making a service part of the architecture.
- The same artifacts support manual benchmarks, regression checks, and future model
  training.
- The three-profile comparison isolates the value of each layer: plain, `ghx`,
  and `ghx-sidecar`.

### Negative

- `ghx` still owns a small environment runner and reward schema.
- ACP event capture must be robust enough to produce reliable episodes.
- Local reporting will be less polished than a hosted eval dashboard at first.
- Training later will still require a separate model training stack; this ADR only
  defines the data-producing environment.

## Non-Goals

- Build a public `ghx bench` CLI.
- Build a general-purpose eval framework.
- Replace Braintrust, LangSmith, Genkit, Eino, or OpenRLHF as products.
- Require raw OpenAI or Anthropic API keys for local eval runs.
- Require Braintrust, LangSmith, or another hosted eval platform.
- Train the sidecar model in this repository today.

## Implementation Notes

1. Keep eval code behind `agent_e2e` build tags so normal tests stay fast and
   deterministic.
2. Keep local artifacts as the canonical record.
3. Prefer deterministic rewards before adding LLM-as-judge.
4. Persist partial episodes incrementally so failed runs still produce useful traces.
5. Never include secrets in episode JSON or OTel attributes.
6. Treat `plain` profile as a baseline, not a hard regression gate, because `gh`/shell
   behavior is less owned by `ghx`.

## References

- [ADR-0015: ghx Sidecar Go-Native Migration](./0015-sidecar-go-native-migration.md)
- [Braintrust Go SDK](https://github.com/braintrustdata/braintrust-sdk-go)
- [Braintrust Go eval package](https://pkg.go.dev/github.com/braintrustdata/braintrust-sdk-go/eval)
- [OpenTelemetry](https://opentelemetry.io/)
- [Genkit Go Evaluation](https://genkit.dev/docs/go/evaluation/)
- [trpc-agent-go evaluator package](https://pkg.go.dev/trpc.group/trpc-go/trpc-agent-go/evaluation/evaluator)
- [Evalaf / Anteval](https://pkg.go.dev/github.com/antflydb/antfly/pkg/evalaf)
- [OpenRLHF](https://github.com/OpenRLHF/OpenRLHF)
