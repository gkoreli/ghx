# M7/A3 research artifact: Tier-2 escalation — codemap CLI + local-clone codemapping as sidecar-decided internal tools

**Capability candidate:** M7/A3 "Tier-2 escalation" — absorb the codemap CLI
and local-clone + codemapping as sidecar-decided internal tools, fully visible
(NORTH_STAR M7, `docs/NORTH_STAR.md:282`; workstream A3, `docs/NORTH_STAR.md:303`;
brain half B9, `docs/NORTH_STAR.md:318`).
**Status:** research artifact / pre-proposal. Read-only analysis grounded in
committed artifacts and live command output; proposes (does not implement) the
remaining M7/A3+B9 moves. No existing file was modified; nothing committed.

**Repo state at authoring:** branch `mainline`, HEAD `719cb29` ("docs:
normalize engineering record spacing"), clean tree except this artifact under
`docs/research/` (untracked).

**Sources this artifact is grounded in:** `docs/NORTH_STAR.md` (P3 "Swallow the
tools", §1 escalation tiers, M7, A3, B9), the ADR-0024 series in full
(`docs/adr/0024-escalation-tiers-research.md`, `0024.1-escalation-tiers-decision.md`,
`0024.2-escalation-policy.md`, `0024.3-cbm-tier2-backend-bakeoff.md`),
ADR-0019 (`docs/adr/0019-sidecar-adoption-zero-cli-surface.md`) and ADR-0029.2
(`docs/adr/0029.2-persona-revision-3-tier2-discovery-citations.md`), ADR-0030 /
ADR-0030.1 (daemon + session routing, `docs/adr/0030-always-on-daemon.md`,
`docs/adr/0030.1-session-routing.md`), ADR-0036 / ADR-0035 (target architecture
and hardening sequence, `docs/adr/0036-target-architecture-runner-port-and-boundaries.md`,
`docs/adr/0035-architecture-hardening-refactor-sequence.md`),
`docs/evals/TRUST.md` row H9, `docs/dogfood/FRICTION.md`,
`skills/ghx-recon/SKILL.md`, and the shipped code under
`internal/sidecar/tier2/`, `internal/sidecar/`, `internal/cli/`, and
`internal/mapengine/`. Every claim cites a file:line or a command whose output
is excerpted in §8; live commands were run against a fresh build of this HEAD
(`/tmp/ghx-m7research`, see §8).

**Format note:** like `docs/research/h7-corpus-discrimination.md`, this is a
research artifact, not an ADR — it collects evidence and proposes; the decision
itself lands as an ADR (outline in §7).

## 0. TL;DR

M7/A3 is **mostly shipped, not future work**. The A3 tooling half is done
(`internal/sidecar/tier2/`: SHA-pinned cached snapshots, `local:codemap`,
`local:ast-grep`, `local:repomap`; CLI `ghx tier2 codemap|astgrep|repomap`,
`internal/cli/tier2.go:19`). The B9 brain half is done for the surfaces that
exist today: a declarative, pre-registered escalation policy v1
(`internal/sidecar/tier2/policy.go:17`), a recorded decision per turn in three
places (ADR-0024.2 D3; `internal/sidecar/runtime.go:565,606`),
persona doctrine teaching the escalation move (ADR-0029.2 D1; rendered persona
at `internal/sidecar/prompt.go:145`), and a caller-side grant flag
(`ghx sidecar ask --local`, `internal/cli/sidecar.go:684`).

What remains of "sidecar-decided" is narrow and precise: the v1 policy is
**evaluated post-hoc** from recorded turn artifacts (ADR-0024.2 D3 "Honest
deviation"), three of seven signals are never set by the runtime (ADR-0024.2
D2), the daemon does not yet own inline pre-gating of Tier-2 tool runs, cache
eviction has no user-visible surface (`ghx cache ls/clean` unbuilt,
ADR-0024.1:443), and no eval gate has ever measured the policy's precision
(did escalation improve correctness without over-triggering). This artifact
specifies that remaining slice — **runtime-owned escalation** — and proposes
it land as **ADR-0024.4** (new decimal in the `escalation-tiers` family, §7).

## 1. Problem statement — what exists today vs what is missing

### 1.1 What the candidate asks for

NORTH_STAR P3 "Swallow the tools": "the ghx CLI itself, codemap, local clone +
codemapping for deeper understanding — all become internal tools of the sidecar
brain" (`docs/NORTH_STAR.md:73`). Milestone M7: "Escalation tiers: codemap CLI
+ local clone as internal sidecar tools, sidecar-decided, fully visible —
Future (ADR before build)" (`docs/NORTH_STAR.md:282`). Workstream A3: "Tier-2
structural tools absorbed under the CLI: codemap, ast-grep, repomap ranking
(= M7 tooling half) — Future (ADR-0024 research done)"
(`docs/NORTH_STAR.md:303`). Workstream B9: "M7 half shipped (ADR-0024.2)"
(`docs/NORTH_STAR.md:318`). Capability §1 fixes the tier model: Tier 2 is
"when questions demand deeper structural understanding, the sidecar decides to
pull the codebase locally and run codemapping under the hood", and "which tier
answered is always visible in the report and traces"
(`docs/NORTH_STAR.md:119-127`). The tenet "Remote-first, escalation explicit —
no clone, no local index by default … never silent magic" pins the visibility
constraint (`docs/NORTH_STAR.md:207-209`).

### 1.2 What is actually missing — the precise gap

Verified against this HEAD (`719cb29`):

1. **Post-hoc, not pre-gating.** The runtime "records and reconciles rather
   than intercepts" (`internal/sidecar/tier.go:16-18`, ADR-0024.2 D3 "honest
   deviation"). The agent can run `ghx tier2 ...` shell commands and the
   policy only explains afterwards; nothing blocks an ungranted or unjustified
   escalation before it happens.
2. **Three of seven signals are never set.** `remote.symbol_absent_after_maps`,
   `remote.candidate_ambiguous`, `remote.import_chain_invisible` require
   semantic judgment the v1 runtime "cannot recompute deterministically"; they
   "fire only when a future surface supplies agent-declared observations"
   (ADR-0024.2 D2, `docs/adr/0024.2-escalation-policy.md:119-123`).
3. **No eval gate measures the policy.** `grep -rn "tier"
   internal/sidecar/evals/gates.go` returns nothing — no gate asks whether
   escalation improved correctness (precision) or over-triggered. The M4
   verdict measured profiles, not the policy.
4. **No cache user surface.** Eviction machinery exists
   (`internal/sidecar/tier2/cache.go:61-77`, TTL+LRU with events) but
   `ghx cache ls/clean` was deferred (ADR-0024.1 open question, line ~443).
5. **Grant plumbing exists end-to-end** — `--local` flag
   (`internal/cli/sidecar.go:684`), `AskRequest.AllowedBackends`
   (`internal/sidecar/runtime.go:210-211`), daemon JSON-RPC passthrough
   (`internal/sidecar/daemon.go:316-337`) — so the gap is genuinely only the
   decision layer, not transport.

So the remaining M7/A3+B9 slice is **runtime-owned escalation**: agent-declared
observations feeding the recorded policy pre-hoc, a policy-precision eval gate,
and the cache surface.

## 2. Design rationale — how sidecar-decided escalation completes

**D-candidate 1 — agent-declared observations, trace-cross-validated.** The
three unset signals need semantic judgment; the honest source is the agent
declaring them mid-turn (a structured `tier.observe` command or report-block
field), accepted only when consistent with the turn's wire traces
(`internal/sidecar/ledger.go` extraction already used by ADR-0024.2 D2).
Divergence = recorded anomaly (H4 backstop). The policy stays recomputable.

**D-candidate 2 — pre-hoc gating, both paths.** Before a `local:*` command
executes, the runtime evaluates the recorded policy; a blocked command gets an
affordance error (pairs with A4) and a `tier.decision` record with
`decision=blocked`. Must behave identically on daemonless
(`AskWithTurnRunner`) and daemon (`internal/sidecar/daemon.go:316-337`) paths,
wire-pinned like the H8 meta fix (TRUST H8).

**D-candidate 3 — policy-precision eval gate.** New pre-registered gate over
escalated episodes: escalated tasks must not regress correctness
(non-inferiority, δ style per ADR-0032.1) and the policy's fired-signal set
must beat a no-escalation counterfactual. Reuses ADR-0025 D1/D2 run economics;
"evals are how we know" tenet (`docs/NORTH_STAR.md:180-186`).

**D-candidate 4 — cache surface.** `ghx cache ls/clean` over the existing
TTL+LRU eviction (`internal/sidecar/tier2/cache.go:61-77`), discharging the
ADR-0024.1 open question; locks stay (`service.go:215-237`).

## 3. Alternatives considered

- **Keep post-hoc forever:** viable fallback if pre-hoc gating measurably
  hurts latency; but "sidecar-decided" stays bookkeeping, three signals stay
  dead, and nothing is measured — rejected as end-state, kept as fallback.
- **MCP-exposed codemap to the main agent:** rejected — violates P3 (main
  agent must never touch structural tools) and re-pollutes main context.
- **Main-agent-driven clone:** rejected — same violation, plus silent magic
  (tenet, `docs/NORTH_STAR.md:207-209`).
- **Do nothing / declare M7 done:** rejected — B9's own row says "M7 half
  shipped" (`docs/NORTH_STAR.md:318`); the north-star claim "the sidecar
  decides" would rest on an unaudited LLM choice.

## 4. Risks

- **Agent-declared signals can be gamed** → trace-cross-validation before
  acceptance; mismatch = anomaly.
- **Policy gate adds run economics cost** → ADR-0025 D1/D2 stopping + baseline
  reuse; gate only escalated cells.
- **Clone hygiene/security** unchanged from ADR-0024.1 (SHA-pinned snapshots,
  owned cache root, provenance lines).
- **Two-path divergence** (daemonless vs daemon) → wire-pinned parity tests.

## 5. Interaction with M8 anticipation + B7 session routing

- B7 routing picks the session pre-turn (`daemon.go:316-337`); tier decisions
  stay per-turn inside the routed session, so routing is orthogonal — but the
  routed session choice must not change tier semantics (parity tests again).
- M8 anticipation (ADR-0031.1) presupposes the resident runtime; pre-hoc
  gating gives anticipation a clean hook: an anticipated escalation can be
  pre-evaluated before the question even lands.

## 6. Open questions

1. Declaration channel for agent-declared observations: mid-turn structured
   command vs report-block field? Mid-turn composes with live tailing
   (`internal/sidecar/livetail.go`); report-block is simpler but late.
2. Does pre-hoc gating apply to eval profiles identically (steering parity,
   TRUST H8 lesson)?
3. Precision-floor threshold for D-candidate 3 — needs the fixbatch
   distribution before pre-registration.
4. `ghx cache ls/clean` before or after automatic-eviction proof
   (ADR-0024.1 open question ordering)?

## 7. Proposed ADR outline

**ADR-0024.4: Runtime-Owned Escalation — Agent-Declared Observations, Pre-Hoc
Gating, Policy Measurement** (`docs/adr/0024.4-runtime-owned-escalation.md`).

Thread justification: this is squarely inside the `escalation-tiers` family
(parent ADR-0024.1; sibling 0024.2 policy engine, 0024.3 cbm bake-off). It is
not a new concept (no new number) and not architecture-boundary work in
ADR-0036's sense — it completes an existing decision family. Sections:
Status (pre-registered before build), Context (the §1.2 gap), Decision
(D-candidates 1–4 as D1–D4 with alternatives from §3), Consequences,
Implementation Notes (filled post-build), plus explicit non-goals: no change
to eval scoring of existing gates, no new backend, no persona rewrite.

## 8. Evidence appendix

Commands run against a fresh build of HEAD `719cb29`:

- `ls internal/sidecar/tier2/` → astgrep.go cache.go clone.go codemap.go
  policy.go provenance_test.go repomap.go service.go status.go toolrun.go
  (+ _test files) — the absorbed-tool substrate exists.
- `git log --oneline -5 -- internal/sidecar/tier2/` → `a460937 feat(tier2):
  doctor reports backend availability…`, `1a25ccb feat(sidecar): escalation
  policy engine + recorded tier decisions (ADR-0024.2)`, `2ba85a7 feat(tier2):
  absorb ast-grep as local:ast-grep + repomap ranking as local:repomap
  (ADR-0024.1 items 3-4)` — implementation history matches the ADR series.
- `grep -rn "tier" internal/sidecar/evals/gates.go` → no matches — no policy
  measurement gate exists.
- `grep -n "Use:\|Short:" internal/cli/tier2.go` → `tier2 codemap|astgrep|
  repomap` subcommands confirmed.
- `sed -n '89,130p' docs/adr/0024.2-escalation-policy.md` → D2 signal table;
  three signals "not runtime-derivable in v1 … never set".
- `internal/cli/sidecar_test.go:527-531` → default is nil AllowedBackends
  (remote-only per tenet); `--local` grants `local`.



