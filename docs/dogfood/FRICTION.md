# Dogfood Friction Log (M5 exit bar — ADR-0019)

Declarative log of friction found while dogfooding ghx-with-sidecar for
daily development. Friction items outrank speculative features. M5 closes
when a week of daily use is logged here and every **breaking** item is
fixed or explicitly deferred with rationale.

Severity mirrors the eval anomaly taxonomy:

- **breaking** — blocked the task or forced a fallback to the bare CLI.
- **soft** — worked, but ground (extra steps, confusion, wasted tokens).

Demand tags (pre-registered triggers — add to the entry when they apply):

- `tier2-demand` — the question needed deeper structural understanding
  than remote evidence gives (feeds M7 escalation tiers).
- `a2a-demand` — an orchestrator-shaped consumer wanted to talk to the
  sidecar as an agent (feeds ADR-0020.1 D3).

Once ADR-0022 lands, cite the session trace (`~/.ghx/sessions/<session>/`)
in the entry — friction reports with traces are evidence, not anecdotes.

Entry format:

```
## YYYY-MM-DD <short title> — <breaking|soft> [tags]
- Attempted: <what you asked / ran>
- Ground: <what happened vs what should have>
- Trace: <session dir or "pre-ADR-0022">
- Disposition: <open | fixed <commit> | deferred: <rationale>>
```

---

_No entries yet. Setup for the week: `go build -o ghx ./cmd/ghx`,
`./ghx sidecar doctor`, then either `ghx sidecar ask --repo <owner/repo>
"<question>"` directly or wire `ghx serve --recon` into your agent's MCP
config and let the recon skill do the talking._
