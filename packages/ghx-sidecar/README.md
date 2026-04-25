# @gkoreli/ghx-sidecar

Experimental ACP sidecar harness for `ghx` repository reconnaissance.

This package owns the fast-moving sidecar runtime:

- natural-language repository questions
- named sidecar sessions
- sidecar prompt authoring
- report schema
- future native ACP client integration

The Go `ghx` binary remains the evidence engine.

## Current Status

Scaffold only. The package defines the sidecar contract and prompt, but does not yet connect to `codex-acp`.

## Target API

```ts
import { createSidecarPrompt } from "@gkoreli/ghx-sidecar";

const prompt = createSidecarPrompt({
  session: "ghx-sidecar:openai-node-streaming",
  repo: "openai/openai-node",
  question: "Find where streaming responses are exposed and implemented.",
  allowedBackends: ["remote"],
});
```
