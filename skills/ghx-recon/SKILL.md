---
name: ghx-recon
description: "Use when delegating GitHub repository reconnaissance to the ghx sidecar service: ask repo questions in plain English and receive compact evidence reports."
version: 1.0.0
author: ghx contributors
license: MIT
metadata:
  hermes:
    tags: [github, code-exploration, sidecar, repository-recon]
    related_skills: []
---

# ghx — code reconnaissance service

ghx is a specialist agent that explores GitHub for you: ask a repo question
in plain English and it returns a compact evidence report. Never explore a
remote repo step-by-step yourself — delegate the whole question and let ghx
do the reading, mapping, and searching under the hood.

## Ask

Two entry points, same service:

- MCP: the `recon` tool — `recon(question, repo?)`.
- CLI: `ghx sidecar ask "<question>"`.

## Asking well

State the goal and name the repo when you know it — "How does hono implement
middleware chaining, and which files define it?" beats "run grep for
middleware". Omit the repo for discovery questions — "which repos or
libraries do X" — and ghx sweeps GitHub, verifies the top candidates by
reading them, and ranks them. One investigation per question; follow up
rather than bundling.

## Reading the report

- answer — the direct answer.
- verified — claims ghx backed by evidence it actually read; trust these.
- evidence / relevantFiles — what each source showed, and where to look.
- uncertainty / nextReads — what it could not confirm, and what to read next.

Every answer ends with an artifacts pointer (session dir + trace id) — the
on-disk audit trail behind the report. Follow-ups stay on the right
investigation automatically; you never manage sessions. A fresh question
takes tens of seconds; follow-ups are faster.
