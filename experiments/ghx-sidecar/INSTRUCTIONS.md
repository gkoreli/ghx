# ghx-sidecar POC Instructions

This folder is only for the manual proof-of-concept. Do not build architecture from this yet.

The goal is to test whether a specialized `ghx` sidecar agent is useful enough to pursue beyond a prompt.

## Files

- `INSTRUCTIONS.md`: what you are reading now.
- `AGENT_DEFINITION.md`: paste into a fresh Codex sidecar session.
- `QUESTIONS.md`: copy-paste test questions.
- `SIDECAR_ANSWERS.md`: paste raw sidecar answers here.

## Workflow

1. Open a fresh Codex session for the sidecar.
2. Paste the full prompt from `AGENT_DEFINITION.md`.
3. Open `QUESTIONS.md`.
4. For each new repo/topic, start a new sidecar session.
5. For a follow-up on the same repo/topic, reuse the same sidecar session.
6. Paste every raw answer into `SIDECAR_ANSWERS.md`.
7. Do not clean up or rewrite the sidecar answers before scoring.
8. After the test set is complete, ask the main agent to score the results against ADR 14.2.

## Current Gate

Do not build ACP, `acpx`, Codex `app-server`, a sidecar daemon, or Codemap escalation yet.

First prove the manual sidecar has repeatable value across the questions in `QUESTIONS.md`.

The scoring standard is defined in:

```text
docs/adr/0014.2-sidecar-proof-of-concept-gate.md
```
