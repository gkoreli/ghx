---
title: "ADR-0028: Agent-Facing CLI Research — Prior Art for the Ergonomics Batch"
date: "2026-07-06"
status: "research"
thread: "cli-ergonomics"
author: "Goga Koreli"
---

# 0028. Agent-Facing CLI Research — Prior Art for the Ergonomics Batch

## Status

Research memo (NORTH_STAR workstream A2 "agent-usage mining → CLI ergonomics
batch" and A4 "error messages as agent affordances"). Input one: the miner's
ranked defect list in `docs/evals/mining-2026-07-06/CLI-FINDINGS.md` (8
defects, mined from 2,068 tool traces). Input two: this memo — open-source
prior art that confirms, refines, or contradicts each recommended fix before
anything is implemented. No implementation in this ADR.

Method note (honesty about the evidence): web sources were read through
WebFetch summarization, not full-page reads; every claim that could be
reproduced locally *was* reproduced locally on 2026-07-06 (marked
"verified locally" below) — `gh` 2.83.0, `git` 2.50.1, `rg` (current
Homebrew), and the installed `ghx` binary. `jj` was not installed locally;
its behavior is cited from docs only.

## Sources consulted

Design guidelines and agent-specific guidance:

- Command Line Interface Guidelines (clig.dev) — <https://clig.dev/> —
  the de-facto standard for flag naming, error suggestions, `--version`,
  `--json`, paging.
- GNU Coding Standards, `--version` node —
  <https://www.gnu.org/prep/standards/html_node/_002d_002dversion.html> —
  every program must accept `--version` and `--help`.
- gh CLI / Primer CLI design principles —
  <https://primer.style/cli/getting-started/principles> (redirects to
  <https://github.com/primer/cli>) — "optimize for what most people need
  most of the time; adjust with flags".
- Anthropic, "Writing effective tools for agents" —
  <https://www.anthropic.com/engineering/writing-tools-for-agents> —
  response budgets (Claude Code caps tool responses at 25k tokens),
  truncation-with-steering, consolidated workflow tools, `response_format`
  concise/detailed enum.
- "Writing CLI Tools That AI Agents Actually Want to Use" —
  <https://dev.to/uenyioha/writing-cli-tools-that-ai-agents-actually-want-to-use-39no> —
  semantic exit codes, JSON-to-stdout/noise-to-stderr, help text as the
  agent's tool description, examples over descriptions.

Tools examined as prior art:

- ripgrep GUIDE — <https://github.com/BurntSushi/ripgrep/blob/master/GUIDE.md> —
  `--max-count`, `--max-columns(-preview)`, `-C`; flag-suggestion errors
  verified locally.
- jj (Jujutsu) FAQ / CLI reference — <https://docs.jj-vcs.dev/latest/faq/>,
  <https://docs.jj-vcs.dev/latest/cli-reference/> — `jj log` shows a
  curated subset by default (revset `@ | ancestors(immutable_heads().., 2)
  | trunk()`), full history behind `-r ::`.
- aider repo map — <https://aider.chat/docs/repomap.html> — `--map-tokens`
  (default 1k) budget + graph (PageRank-style) ranking of symbols.
- semgrep output formatting — <https://semgrep.dev/docs/getting-started/cli> —
  `--max-lines-per-finding` (default 10), `--text/--json/--sarif`.
- ast-grep — <https://ast-grep.github.io/guide/quick-start.html> — one
  invocation = structural search + snippet; `--lang/-l`, `--json`.
- Sourcegraph `src search` —
  <https://sourcegraph.com/docs/cli/references/search> — one-shot
  query → files+snippets, `-json`, `-explain-json`, `count:` limits.
- bat — <https://github.com/sharkdp/bat> — `--line-range=:500` slicing
  syntax (colon form).
- Cobra user guide — <https://github.com/spf13/cobra/blob/main/site/content/user_guide.md>
  and <https://pkg.go.dev/github.com/spf13/cobra> — `SuggestFor`,
  `SuggestionsMinimumDistance`, built-in `Version` field; directly
  relevant because ghx is Cobra-based and two observed defects are Cobra
  defaults misfiring.
- clap v4 / cargo — <https://github.com/rust-lang/cargo/pull/12529>,
  <https://github.com/clap-rs/clap/issues/4638> — unknown-argument
  suggestions ("a similar argument exists: '--option'") as a parser-level
  feature.
- gh `--json` field errors — <https://github.com/cli/cli/issues/10385> and
  verified locally — error output enumerates every valid field.

Total: 16 distinct sources (5 guidelines/essays, 11 tools/parsers), plus
local reproductions against `gh`, `git`, `rg`, and `ghx` itself.

## Findings

### Area 1 — What makes flags guessable, and errors that teach

**Guessability comes from convention reuse, not from alias abundance.**
clig.dev is explicit: "Use standard names for flags, if there is a
standard" so users can "guess an option without having to look at the help
text", and it enumerates the standard set (`--json`, `-q/--quiet`,
`--version`, `-o/--output`, ...). The gh/Primer principle is the same
shape: optimize the default, adjust with flags. For agents this matters
double: an LLM's "muscle memory" is the aggregate of conventions in its
training data, so the guessable flag is the one ripgrep/bat/head already
use, and the guessable command is the one every other tool has (`grep`).

**The state of the art for teaching errors is parser-level suggestion on
both commands and flags.** Verified locally:

- `rg --max-cout` → `unrecognized flag --max-cout` + `similar flags that
  are available: --max-count`. ripgrep does this for *flags*, which
  Cobra/pflag does not do out of the box.
- clap v4 does the same generically ("a similar argument exists:
  '--option'"), and cargo uses `UnknownArgumentValueParser::suggest` to
  map known-foreign flags (e.g. git-style spellings) to the cargo
  equivalent — i.e. *pre-registering the guesses you expect*.
- Cobra provides this for *commands* (Levenshtein, min distance 2) plus
  `SuggestFor` to register suggestions that are semantically close but
  lexically far. The miner's "actively misleading" `ghx grep` → "Did you
  mean this? tree" (reproduced locally) is exactly the Levenshtein
  default firing with no `SuggestFor` curation: `grep`→`tree` is edit
  distance 2, so tree gets suggested; `search` is distance 5, so the
  right answer never appears.
- gh's `--json` with no fields errors with the complete list of valid
  fields — the error *is* the documentation for the correct next
  invocation. This is the strongest "exact-example error" pattern found.

One caution from clig.dev: suggest, don't auto-correct — "if you change
what the user typed, they won't learn the correct syntax", and guessing is
dangerous when the action modifies state. For a read-only tool like ghx
the danger is low, but the teaching argument still holds for agents:
sessions show retry sequences converge fastest when the error contains the
literal next command to run.

### Area 2 — Output budgeting

Convergent pattern across every tool examined: **compact, curated default;
escalation is explicit; budget is one dial, not many.**

- jj: `jj log` shows a curated revset subset by default ("you are more
  likely to interact with this set"); the escape hatch is `-r ::`, one
  flag, not per-section toggles.
- ripgrep bounds *dimensions* of output: `--max-count` (matches per
  file), `--max-columns` (line width, with `--max-columns-preview` so
  truncation is visible), `-C` (context). Each limit has a visible
  truncation marker.
- semgrep: `--max-lines-per-finding`, default 10 — a per-snippet budget
  baked into the default text format.
- aider repomap: a single token budget (`--map-tokens`, default 1k) plus
  a ranking algorithm that decides what fits. Budget + ranking, not
  budget + arbitrary truncation.
- Anthropic tool guidance: responses capped (Claude Code: 25k tokens);
  when truncating, "steer agents with helpful instructions" toward
  narrower follow-up calls; consider a `response_format` concise/detailed
  enum where concise is ~1/3 the tokens.
- clig.dev: "Display output on success, but keep it brief"; pagers only
  when stdout is a TTY (agents never are — so ghx must self-limit, it
  cannot rely on a pager).

The refinement this implies for ghx (see table, defects 1 and 6): prefer
**one budget dial + one `--full` escape + ranking of what fits**, over the
miner's basket of micro-flags (`--files-limit`, `--readme-lines`,
`--no-readme`). Micro-flags are themselves an ergonomics tax: each is one
more thing to guess wrong.

### Area 3 — One-shot inspect

The miner's #4 (missing `ghx inspect <repo> <query>`) has the strongest
prior-art support of all eight findings:

- Anthropic explicitly recommends consolidating multi-call workflows into
  one tool: a `search_logs` tool that "only returns relevant log lines
  and some surrounding context" instead of a fetch-then-filter chain;
  `schedule_event` instead of list+list+create.
- aider's repomap is precisely "ranked structure under a token budget":
  files + key symbols + "the critical lines of code for each definition",
  selected by graph ranking against the current conversation. It is the
  proof that ranking + budget beats exhaustive structure.
- ast-grep returns match + surrounding structure in a single invocation;
  semgrep's default text format is (file, rule, ≤10-line snippet) — i.e.
  the industry norm for "search result" already bundles bounded snippets.
- Sourcegraph `src search` is a one-shot query → files + snippets with
  `-json` and query-level `count:` limits.

Nothing found contradicts the inspect recommendation. The prior art adds
two requirements the miner didn't state: (a) it must be **budgeted from
birth** (aider/semgrep both cap; an unbudgeted inspect just recreates
defect 1 at higher rank), and (b) truncation must **carry next-step
hints** (Anthropic steering pattern), e.g. "N more files matched; narrow
with --path or raise --budget".

### Area 4 — Version and health-check conventions

- GNU Coding Standards: "All programs should support two standard
  options: '--version' and '--help'"; first output line parseable,
  version after the last space.
- clig.dev: `--version` is the standard; `-v` is ambiguous
  (verbose/version).
- Verified locally: `gh --version` and `gh version` both work and print
  identical output; `git --version` works. Agents guess `--version` first
  because virtually every non-Cobra tool in their training data only has
  the flag form; version-as-subcommand is a Cobra-community habit, not a
  cross-ecosystem convention.
- Cobra ships this for free: setting the root command's `Version` field
  auto-registers `--version`. The miner's 30 observed failures are the
  cheapest fix on the list.

### Area 5 — Contradictions and refinements of the miner

Three places where prior art pushes back on the miner's exact fix; none
overturn a finding, all narrow the fix:

1. **"Support all three range-alias families as canonical synonyms"
   (defect 3) — contradicted in part.** clig.dev's core rule is one
   standard name per concept; ripgrep/clap/cargo solve wrong-guess pain
   with *parser-level suggestions*, and cargo's `suggest` mechanism shows
   the mature pattern: pre-register expected foreign spellings and answer
   them in the error. Making three alias families *canonical* (documented,
   in help) fragments examples across traces and docs and grows the
   guess-surface for the next flag. Refined position: keep `--lines A-B`
   the only documented spelling; *accept* `--start/--end` and
   `--offset/--limit` as hidden aliases (they are structural guesses, not
   noise — Claude Code's own Read tool takes `offset`/`limit`
   parameters, and GitHub permalinks use `L42-L80`, which `--lines 42-80`
   already mirrors); and add a flag-error handler (Cobra
   `SetFlagErrorFunc`) so any *other* wrong guess gets a
   ripgrep/gh-style teaching error with the literal corrected command.
2. **Micro-flag basket on `explore` (defect 1) — refined.** jj, aider,
   semgrep, and Anthropic all converge on one budget dial + one escape
   hatch. `--full` yes; `--files-limit/--readme-lines/--no-readme`
   replaced by a single `--budget` (shared with `read`, defect 6) whose
   truncation output names the narrowing flags.
3. **`--lang` on search (defect 7) — miner under-specified, prior art
   splits.** ripgrep spells it `-t/--type`; ast-grep and semgrep spell it
   `-l/--lang`. Since the observed agent guess was `--lang` (7 failures)
   and ghx's domain is code search (ast-grep/semgrep territory), `--lang`
   is the right documented name, with `--type` answered by the teaching
   flag-error. This is a case where the *observed guesses* should
   outvote clig.dev-style prescription — the trace corpus is ghx's
   usability study.

One addition the miner missed entirely, from the agent-CLI guidance:
**exit-code semantics and help-example density.** Agents branch on exit
codes (0 success / 2 usage error is the near-universal convention rg and
grep follow: match/no-match/error tri-state), and "agents learn patterns
from examples faster than from flag descriptions" — yet `ghx explore
--help` (verified locally) contains zero examples and zero flags. Every
subcommand help should carry 2-3 copy-pasteable examples showing the
budget/range/grep idioms; that is the cheapest discoverability fix for
defect 2 (agents shell-piping instead of using `--lines/--grep`), because
`--help` is the first thing an agent runs on an unfamiliar tool.

## Recommendations table

| # | Defect (CLI-FINDINGS) | Miner's fix | Prior-art evidence | Refined recommendation |
|---|---|---|---|---|
| 1 | `explore` too verbose by default (65 traces, 1.37M chars) | Compact default; current behavior behind `--full`; add `--files-limit`, `--readme-lines`, `--no-readme` | **For compact default:** jj log curated subset; Anthropic concise-by-default (~1/3 tokens); clig.dev "keep it brief". **Against micro-flags:** every budgeting tool ships one dial (aider `--map-tokens`, semgrep `--max-lines-per-finding`) | Compact, ranked default + `--full`. Replace the three micro-flags with one `--budget` shared across commands. Truncation footer names the escalation path (Anthropic steering pattern) |
| 2 | Agents shell-filter `ghx read` (501 traces piping to sed/grep/head) | First-class aliases + hints in oversized-output headers | gh `--json` error enumerating valid fields (error/output as documentation); Anthropic "steer with instructions when truncating"; help-as-tool-description: examples beat descriptions | Adopt both miner ideas, plus: put 2-3 runnable examples of `--lines/--grep/--map` in every subcommand help (currently zero, verified). Oversized-output header cites the exact narrower command for *this* file |
| 3 | Natural range-flag guesses fail (`--start/--end` 22, `--offset/--limit` 5, `--line-range` 1) | Accept all three families as canonical synonyms; teaching error text | **For accepting guesses:** Claude Code Read tool itself uses `offset`/`limit` — the guess is structural; cargo pre-registers foreign flags via clap `suggest`. **Against canonical synonyms:** clig.dev one-standard-name; rg/clap answer wrong guesses with suggestions, not aliases | `--lines A-B` stays the only documented form (mirrors GitHub `#L42-L80`). Accept `--start/--end` and `--offset/--limit` as hidden aliases. Add Cobra `SetFlagErrorFunc` so unknown flags get rg-style "similar flags available / did you mean `--lines 42-80`" with a full corrected example |
| 4 | No one-shot inspect workflow (77+63+46 multi-command chains) | Add `ghx inspect <repo> <query>`: ranked files + maps + bounded snippets, budgeted | Strongest support of all: Anthropic workflow consolidation (`search_logs` example); aider repomap = ranking + token budget; semgrep ≤10-line snippets; src search one-shot files+snippets | Build it, with two hard requirements the miner implied but didn't state: budgeted from birth (share `--budget`), and truncation carries next-step hints. Rank before truncating (aider precedent), don't truncate positionally |
| 5 | Agents expect `ghx grep`; "Did you mean tree?" misleads (9+9) | Add `ghx grep` alias; fix suggestion text | Cobra `SuggestFor` exists precisely for lexically-far/semantically-near suggestions; the `tree` suggestion is the uncurated Levenshtein default (reproduced locally). `grep` is the single most conventional search spelling in training data | Two-step: (a) immediately add `SuggestFor: ["grep", "find", "rg"]` on `search` — one line, kills the misleading error; (b) add `ghx grep` as a real subcommand whose flags mirror ripgrep's (`--glob`, `-C`, `--max-count`) so rg muscle memory transfers |
| 6 | Full-file reads where maps/ranges would do (22 traces, 572k chars) | Default to `--map` + anchors for large files unless `--full`; or `--budget CHARS` with structural-first truncation | Anthropic 25k-token hard cap precedent; rg `--max-columns` + `--max-columns-preview` (truncate *visibly*); aider structural-first selection under budget | Take the `--budget` variant (one dial, shared with explore/inspect), default it generously, make truncation visible with a marker + steering footer. Structural-first (map + anchors) as the truncation strategy, per aider |
| 7 | Search too close to raw GitHub search (`--lang` failures, parse fatals, unbounded output) | `ghx search <repo> <query>` simple form; keep advanced form; `--lang/--ext/--glob`; auto-escape fatal chars | gh coexists simple commands with an advanced `gh api`; src search bounds via `count:`; ast-grep/semgrep spell it `--lang`, rg spells it `--type` | Adopt miner's fix; document `--lang` (code-search-tool convention, matches the 7 observed guesses), answer `--type/-t` via the teaching flag-error. Default result cap with "N more; raise --limit" footer |
| 8 | `ghx --version` fails (30 failures) | Support `--version` alongside `version` | GNU standards ("all programs"); clig.dev; `gh --version` and `gh version` both work (verified). Cobra `Version` field gives the flag for free | Adopt verbatim. Set root `Version`; keep the subcommand. Cheapest fix in the batch; do first |

Cross-cutting additions not in CLI-FINDINGS: (a) semantic exit codes —
distinguish usage error (2) from not-found from API failure so agents can
branch without parsing stderr; (b) examples in every `--help` (the
agent's first call on an unfamiliar tool is `--help`; it is the de-facto
tool description); (c) if/when `--json` is added anywhere, follow the
gh/clig.dev contract — JSON to stdout, noise to stderr, and an
empty-`--json` error that enumerates fields.

## What I could not verify

- Web pages were read via summarizing fetches, not raw; quotes are
  faithful to those summaries but line-level context was not inspected
  for clig.dev, primer, aider, semgrep, jj docs.
- `jj` behavior is docs-only (not installed locally).
- The claim that agents guess `--version` (and `grep`, and
  `offset/limit`) *because of training-data convention frequency* is an
  inference from the trace corpus + convention survey, not something a
  web source states; the trace counts are the miner's, spot-checked
  against three of its cited episode files but not independently
  recounted.
- `bat --line-range` colon syntax confirmed only via its fzf-integration
  example (`--line-range=:500`); full syntax variants (`40:60`, `40:`)
  not confirmed from the README fetched.
- No A/B evidence that hidden aliases outperform documented aliases for
  agents; that refinement rests on clig.dev's consistency argument and
  the cost of fragmented examples across future traces. The eval harness
  can test it (see next step).

## Suggested next step

Pre-register the ergonomics batch as ADR-0028.1 (decision, thread
`cli-ergonomics`): implement in prior-art-refined form — order: defect 8
(`Version` field), defect 5a (`SuggestFor`), defect 3 (flag-error
teacher + hidden aliases), help examples, then the budget dial (1/6),
`ghx grep` (5b), search simple form (7), and `inspect` (4) last since it
is the largest surface. Verify per the eval-routing rule: random spot
check of 2-3 task x profile cells, watching specifically for the defect
signatures (failed `--start` guesses, `| sed` pipes, grep retries)
disappearing from fresh traces.
