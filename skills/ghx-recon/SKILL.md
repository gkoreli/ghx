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

ghx is a specialist agent that explores GitHub repos for you. Ask it
questions in plain English; it returns a compact evidence report. Never
explore a remote repo step-by-step yourself — delegate the whole question.

## Ask

    ghx sidecar ask [--repo <owner/repo>] "<question>"

Give --repo when you know the repo. Omit it for discovery questions —
"which repos/libraries do X" — and the sidecar sweeps GitHub, verifies
top candidates, and ranks them. Normally omit --session too: the daemon
routes each ask — explicit --session pins a thread, a repo mention routes
to the repo-slug session, else a warm continuation, then ledger overlap,
then a new question-derived discovery session — and prints the route as
"session: <name> (routed: <rule>)". Add --json for the full structured
report wrapped as {report, artifacts}.

## Asking well

State the goal, not the steps: "How does hono implement middleware
chaining, and which files define it?" beats "run grep for middleware".
One investigation per question; follow up rather than bundling.

## Reading the report

- answer — the direct answer
- verified — claims backed by evidence the sidecar actually read
- relevantFiles / evidence — where to look and what each source showed
- uncertainty / nextReads — what it could not confirm and what to read next

Every response ends with "artifacts: <session dir> (trace <id>)" — the
on-disk audit trail (traces, logs, reports) backing the report.
If a turn lands in the wrong session, correct it with
`ghx sidecar sessions reroute <session> <turn> <dest>`.

Trust verified claims; treat inferred/unverified ones as leads. A fresh
question takes tens of seconds; session follow-ups are faster.
