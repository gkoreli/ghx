# ghx-sidecar POC Questions

Use these copy-paste prompts to test whether a persistent specialized sidecar beats the main general agent at GitHub reconnaissance.

## Session Rule

Start a new sidecar session when the repo or investigation topic changes.

Reuse the same sidecar session only for follow-ups in the same repo/topic thread.

Examples:

```text
ghx-sidecar:hono-middleware
ghx-sidecar:openai-node-streaming
ghx-sidecar:mcp-sdk-transports
ghx-sidecar:gh-cli-auth
ghx-sidecar:ghx-map-engine
```

## Standard Rules

Use these rules for every prompt:

```text
Budget:
- Max 8 ghx commands
- No clone
- No Codemap
- Do not inspect tests unless required

Return the standard ghx-sidecar report.
```

Use these rules for every follow-up:

```text
Rules:
- Do not restart exploration from zero.
- Reuse prior evidence memory.
- Max 5 additional ghx commands.
- No clone.
- No Codemap.
- Return the standard ghx-sidecar report and include a short "Memory reused" section listing what prior facts you used.
```

## Test 1: Hono Middleware

Already started in `SIDECAR_ANSWERS.md`.

Initial prompt:

```text
Repo: honojs/hono

Session name:
ghx-sidecar:hono-middleware

Question:
Where is middleware composition implemented?

Budget:
- Max 8 ghx commands
- No clone
- No Codemap
- Do not inspect tests unless required

Return the standard ghx-sidecar report.
```

Follow-up:

```text
Follow-up for the same session ghx-sidecar:hono-middleware:

Now use what you already learned to trace how errors propagate through the middleware composition path.

Rules:
- Do not restart exploration from zero.
- Reuse prior evidence memory.
- Max 5 additional ghx commands.
- No clone.
- No Codemap.
- Return the standard ghx-sidecar report and include a short "Memory reused" section listing what prior facts you used.
```

## Test 2: OpenAI Node Streaming

Start a new sidecar session.

Initial prompt:

```text
Repo: openai/openai-node

Session name:
ghx-sidecar:openai-node-streaming

Question:
Where are streaming responses implemented or exposed?

Budget:
- Max 8 ghx commands
- No clone
- No Codemap
- Do not inspect tests unless required

Return the standard ghx-sidecar report.
```

Follow-up:

```text
Follow-up for the same session ghx-sidecar:openai-node-streaming:

Now use what you already learned to identify which public API methods route into streaming.

Rules:
- Do not restart exploration from zero.
- Reuse prior evidence memory.
- Max 5 additional ghx commands.
- No clone.
- No Codemap.
- Return the standard ghx-sidecar report and include a short "Memory reused" section listing what prior facts you used.
```

## Test 3: MCP TypeScript SDK Transports

Start a new sidecar session.

Initial prompt:

```text
Repo: modelcontextprotocol/typescript-sdk

Session name:
ghx-sidecar:mcp-sdk-transports

Question:
Where is server transport setup implemented?

Budget:
- Max 8 ghx commands
- No clone
- No Codemap
- Do not inspect tests unless required

Return the standard ghx-sidecar report.
```

Follow-up:

```text
Follow-up for the same session ghx-sidecar:mcp-sdk-transports:

Now use what you already learned to identify which transports share the same server setup path.

Rules:
- Do not restart exploration from zero.
- Reuse prior evidence memory.
- Max 5 additional ghx commands.
- No clone.
- No Codemap.
- Return the standard ghx-sidecar report and include a short "Memory reused" section listing what prior facts you used.
```

## Test 4: GitHub CLI Auth

Start a new sidecar session.

Initial prompt:

```text
Repo: cli/cli

Session name:
ghx-sidecar:gh-cli-auth

Question:
Where does gh auth status assemble credential state?

Budget:
- Max 8 ghx commands
- No clone
- No Codemap
- Do not inspect tests unless required

Return the standard ghx-sidecar report.
```

Follow-up:

```text
Follow-up for the same session ghx-sidecar:gh-cli-auth:

Now use what you already learned to identify where keyring-backed tokens are read or resolved.

Rules:
- Do not restart exploration from zero.
- Reuse prior evidence memory.
- Max 5 additional ghx commands.
- No clone.
- No Codemap.
- Return the standard ghx-sidecar report and include a short "Memory reused" section listing what prior facts you used.
```

## Test 5: ghx Map Engine

Start a new sidecar session.

Initial prompt:

```text
Repo: gkoreli/ghx

Session name:
ghx-sidecar:ghx-map-engine

Question:
Where is read --map implemented?

Budget:
- Max 8 ghx commands
- No clone
- No Codemap
- Do not inspect tests unless required

Return the standard ghx-sidecar report.
```

Follow-up:

```text
Follow-up for the same session ghx-sidecar:ghx-map-engine:

Now use what you already learned to explain how parser fallback works when mapping fails or when a parser is unavailable.

Rules:
- Do not restart exploration from zero.
- Reuse prior evidence memory.
- Max 5 additional ghx commands.
- No clone.
- No Codemap.
- Return the standard ghx-sidecar report and include a short "Memory reused" section listing what prior facts you used.
```

## Where To Save Answers

Paste every sidecar response into:

```text
experiments/ghx-sidecar/SIDECAR_ANSWERS.md
```

Keep the raw output. Do not clean it up before scoring.
