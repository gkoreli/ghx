---
title: "Search Console Demand Signal for ghx Direction"
date: 2026-04-25
status: Proposed
---

# 0014. Search Console Demand Signal for `ghx` Direction

## Context

Two public essays now describe `ghx` from different angles:

- [Build the GitHub Exploration Tool, No Mistakes](https://gkoreli.com/how-ghx-was-born), published 2026-03-29
- [You Don't Always Need Codemap](https://gkoreli.com/you-dont-need-codemap), published 2026-04-17

The first essay explains why `ghx` exists: agents waste context when they use generic web fetching or low-level GitHub commands to inspect code. The second essay makes the product boundary sharper: Codemap, Aider repo maps, Gitingest, Repomix, and `ghx` are all context tools, but they answer different decisions.

Search Console data from `gkoreli.com` gives an external signal about which parts of that story people are already looking for. This ADR treats the data as product research, not SEO guidance. The goal is not to optimize articles for traffic. The goal is to understand demand vocabulary and use it to sharpen the `ghx` vision.

Source exports:

- `/Users/goga/Downloads/gkoreli.com-Performance-on-Search-2026-04-25/Chart.csv`
- `/Users/goga/Downloads/gkoreli.com-Performance-on-Search-2026-04-25/Queries.csv`
- `/Users/goga/Downloads/gkoreli.com-Performance-on-Search-2026-04-25/Pages.csv`

The files do not reconcile exactly across dimensions. `Chart.csv` totals 1,650 impressions, `Pages.csv` totals 1,687 impressions, and the exported query rows account for 450 impressions. Treat this as directional evidence. Use the chart export for timing, the page export for page-level demand, and the query export for intent.

## Evidence

### Timing signal

The chart export spans 2026-03-09 through 2026-04-22.

| Period | Days | Clicks | Impressions | Avg impressions/day | Weighted position |
|--------|------|--------|-------------|---------------------|-------------------|
| Before 2026-04-17 | 39 | 5 | 615 | 15.8 | 8.18 |
| 2026-04-17 onward | 6 | 2 | 1,035 | 172.5 | 6.06 |

The Codemap essay was published on 2026-04-17. Search impressions rose materially after that point:

- 2026-04-17: 24 impressions
- 2026-04-18: 151 impressions
- 2026-04-19: 335 impressions
- 2026-04-20: 104 impressions
- 2026-04-21: 196 impressions
- 2026-04-22: 225 impressions

This does not prove causation, but it is strong directional evidence that the second essay intersects with existing search demand. The timing also matters because the first ghx essay had already been published for nearly three weeks. The new signal appears around the Codemap framing, not just around `ghx` as a brand.

### Page signal

| Page | Clicks | Impressions | CTR | Position | Share of page impressions |
|------|--------|-------------|-----|----------|---------------------------|
| `/you-dont-need-codemap` | 0 | 865 | 0.00% | 5.90 | 51.3% |
| `/how-ghx-was-born` | 0 | 119 | 0.00% | 6.61 | 7.1% |
| `/oss-radar-01-vercel-winter-2026-cohort` | 2 | 369 | 0.54% | 7.95 | 21.9% |
| `/` | 4 | 45 | 8.89% | 5.96 | 2.7% |

The Codemap essay has the strongest page-level signal by a wide margin: 865 impressions, 51.3% of exported page impressions, average position 5.90, and no clicks.

Do not overread the zero-click result. For this ADR, the useful signal is not traffic conversion. The useful signal is that Google is repeatedly matching this page to a coherent set of technical queries at roughly page-one positions.

### Query signal

The exported query rows are dominated by Codemap and code-search vocabulary:

| Query cluster | Rows | Clicks | Impressions | Share of exported query impressions | Weighted position |
|---------------|------|--------|-------------|------------------------------------|-------------------|
| Queries containing `codemap` | 31 | 0 | 388 | 86.2% | 5.81 |
| Syntax and qualifier queries | 21 | 0 | 225 | 50.0% | 5.41 |
| Import repository or GitHub URL queries | 5 | 0 | 21 | 4.7% | 8.86 |
| `git grep` difference query | 1 | 0 | 3 | 0.7% | 2.00 |
| Non-Codemap or brand/other queries | 7 | 0 | 62 | 13.8% | 19.35 |

Top exported queries:

| Query | Impressions | Position |
|-------|-------------|----------|
| `"codemap" "code search engine"` | 136 | 6.07 |
| `"codemap" "search syntax" code search` | 56 | 5.20 |
| `"codemap" "search syntax"` | 33 | 4.45 |
| `"codemap" "code search" "repo:"` | 20 | 6.35 |
| `"codemap" "search syntax" code search engine` | 16 | 4.81 |
| `"codemap" code search engine search syntax` | 15 | 5.20 |
| `"codemap" "search syntax" code` | 15 | 5.73 |
| `"codemap" "symbol:" qualifier` | 11 | 6.45 |
| `codemap code search engine import repository github url` | 11 | 9.82 |
| `"codemap" "regex" "code search"` | 7 | 2.86 |
| `"codemap" "search syntax" "repo:"` | 7 | 3.14 |

The demand is not generic "AI coding tools." It is specific:

- People are looking for a code search engine, not only a code summarizer.
- People are looking for query syntax and qualifiers: `repo:`, `path:`, `symbol:`, regex, and search syntax.
- People are trying to understand Codemap as an interface and as a mental model.
- Some people are trying to import a GitHub repository or URL into a code-search/mapping workflow.
- One small but precise query asks about Codemap vs `git grep`, which suggests demand for tool-boundary explanations.

### Cross-reference table

| Evidence from Search Console | Article cross-reference | What it tells us | `ghx` implication |
|-----------------------------|-------------------------|------------------|-------------------|
| `/you-dont-need-codemap` has 865 impressions, 51.3% of exported page impressions, position 5.90 | The Codemap essay argues that the first decision is whether a remote repo deserves to be read at all | The Codemap framing is the clearest external demand surface so far | Keep positioning `ghx` as the remote reconnaissance layer before clone, pack, or full read |
| `"codemap" "code search engine"` has 136 impressions, the largest exported query | The Codemap essay compares Codemap, Aider, Gitingest, Repomix, and `ghx` by cost model | Searchers see this space as "code search", not only "context compression" | Make bounded code search and map-driven file selection a first-class part of the product language |
| Syntax and qualifier queries account for 225 impressions, 50.0% of exported query impressions | The Codemap essay explains that `symbol:` works in GitHub.com but not through stable public GitHub APIs | People want operational syntax: `repo:`, `path:`, `symbol:`, regex, query syntax | Document `ghx` syntax clearly and warn when users try web-only GitHub search affordances |
| `"codemap" "symbol:" qualifier` and related `symbol:` searches appear in the top query set | ADR-0013 rejects `ghx search --symbol` as a fake GraphQL feature and favors parser-backed `--kind` | There is real demand for symbol-level orientation, but global symbol search is not honestly available through public APIs | Build around `ghx read --map --kind`, not unsupported global `symbol:` search |
| Import-repository/GitHub URL queries account for 21 impressions | The ghx origin essay starts from agents misusing GitHub URLs through `web_fetch`; the Codemap essay says remote inspection should happen before local commitment | Users are trying to move from GitHub URL to useful code understanding | Make "paste a GitHub repo/path and inspect it remotely" feel native in examples, help text, and agent skill guidance |
| `codemap code search engine git grep difference` appears with position 2.00 | The Codemap essay's tool-boundary section explains when to use Codemap, Aider, packers, or `ghx` | Some demand is comparative: users need to choose the right tool, not just learn a command | Keep explicit tool-boundary docs; trust comes from saying when `ghx` is not the right tool |
| `/how-ghx-was-born` has 119 impressions, while `/you-dont-need-codemap` has 865 | The origin essay explains the build story; the Codemap essay names the problem category and competing workflows | The market does not search for the origin story first; it searches for the problem vocabulary | Product docs should lead with the decision workflow, then link to origin/story material as supporting context |
| Post-2026-04-17 chart impressions rise from 15.8/day to 172.5/day | The Codemap essay was published on 2026-04-17; the origin essay was already live since 2026-03-29 | The demand spike aligns with Codemap/search-syntax vocabulary more than `ghx` brand vocabulary | Treat external vocabulary as a map of user intent, but keep the product boundary anchored in `ghx`'s remote-first cost model |

## Interpretation

### The strongest signal is intent, not volume

The volume is small. This is not market sizing. It is vocabulary discovery.

The important part is that the queries are unusually precise. They are not broad consumer searches. They look like developer searches made while someone is trying to operate or evaluate a tool:

- "How do I search by repo/path/symbol?"
- "What search syntax does this tool understand?"
- "Can I import a GitHub repository URL?"
- "Is this a code search engine?"
- "How is this different from grep?"

That is product-relevant because `ghx` already lives in the same decision space. It helps an agent inspect GitHub code before cloning, packing, or reading implementation.

### People are searching for affordances, not philosophy

The Codemap article's argument is philosophical: use the tool that matches the decision. The queries are operational: syntax, qualifiers, symbol search, importing repositories, regex, code search engine.

This mismatch is useful. It says the vision should keep the philosophy, but the product must expose concrete affordances:

- Show me the repo shape.
- Show me file structure.
- Show me symbols.
- Filter by kind.
- Search within a bounded repo or path.
- Read only the file or range that matters.
- Tell me when a query cannot be answered honestly through GitHub's public APIs.

### Codemap is demand vocabulary, not a product target to copy

The data does not say "`ghx` should become Codemap." It says users have learned to describe this problem with Codemap-shaped vocabulary.

Codemap's territory is local structural indexing. It can afford cache refresh, reference updates, call graphs, and deep repeated local analysis because the repository is already present. `ghx` has a different cost model: remote first-pass exploration through GitHub APIs, before the user or agent commits to cloning.

The useful move is not to chase Codemap feature parity. The useful move is to translate the demand into the `ghx` cost model:

- Codemap asks: "How is this local codebase structured?"
- `ghx` asks: "Is this remote GitHub repo, path, or file worth reading next?"

### Search syntax demand reinforces ADR-0013

[ADR-0013](./0013-ghx-map-command.md) already concluded that `ghx` should not pretend GitHub's public APIs expose modern code search. REST code search is legacy, GraphQL has no public `CODE` search type, and GitHub.com's modern search is not a stable public API.

The Search Console data strengthens that decision. People are actively searching for `symbol:`, `repo:`, regex, and query syntax. That creates a temptation to add a broad `ghx search --symbol` surface. This ADR rejects that temptation for the same reason as ADR-0013: a plausible but wrong result is worse than an explicit limitation.

The product should instead make bounded, honest operations feel first-class:

- `ghx read --map --kind func owner/repo path`
- `ghx read --map --kind type owner/repo path`
- `ghx read --grep pattern owner/repo path`
- `ghx map owner/repo "src/**/*.ts"`
- Future repo-scoped search over fetched candidate files, clearly distinct from GitHub-wide symbol search

## Decision

Use the Search Console signal to sharpen `ghx` around remote reconnaissance, not SEO and not Codemap parity.

`ghx` should own this product sentence:

> Before an agent clones, packs, or reads implementation, `ghx` gives it enough remote structure to choose the next smallest useful read.

This means:

1. Keep remote-first exploration as the core product boundary.
2. Treat "code search engine", "search syntax", `repo:`, `path:`, `symbol:`, regex, and import-repository queries as demand vocabulary.
3. Do not build unsupported global symbol search on top of GitHub.com internals or legacy REST search.
4. Make parser-backed mapping, symbol-kind filtering, glob/path selection, and grep-style bounded search feel like the main product, not advanced flags.
5. Update agent-facing guidance so the workflow is a decision sequence, not a command list.

## Product Direction

### 1. Make the decision ladder explicit

The product should keep teaching this sequence:

```text
Need to inspect a GitHub repo?
1. Explore repo shape.
2. List the relevant tree.
3. Map candidate files.
4. Filter structure by kind.
5. Grep only the bounded path.
6. Read the smallest file or range that can answer the question.
7. Clone or pack only after remote reconnaissance says the repo matters.
```

This should show up in `README.md` and the canonical `internal/skilldoc/SKILL.md`. The agent should learn a posture: delay irreversible context loading until structure justifies it.

### 2. Treat syntax as product surface

The query data shows that developers care about syntax and qualifiers. `ghx` should make its own syntax easy to discover:

- `ghx map --help` should explain path/glob behavior, levels, and `--kind`.
- `ghx search --help` should clearly separate legacy GitHub code search from local/parser-backed or fetched-file search.
- Invalid or web-only GitHub search syntax should produce explicit warnings.
- Examples should use the same vocabulary people search for: repo, path, symbol kind, regex, grep, code search, import GitHub URL.

This is not SEO copywriting. It is reducing operational ambiguity.

### 3. Prefer bounded search over global search

The data shows demand for code search, but `ghx` should answer that demand through bounded operations first:

- Search inside one repo.
- Search inside one path or glob.
- Search across candidate files selected by `tree` or `map`.
- Return context that helps decide the next read.

Global code search remains constrained by GitHub's public APIs. It can stay useful as a thin wrapper with warnings, but it should not become the strategic center.

### 4. Make maps actionable, not decorative

The map output should help the agent decide:

- Which file should be read next?
- Which symbol kind exists in this file?
- Which paths are probably public API versus implementation detail?
- Is the file relevant enough to spend full-file context?

That argues for practical next steps:

- Continue parser-backed mapping work from ADR-0013.
- Keep `ghx read --map` as the agent-native path.
- Add or refine `ghx map` only as a discoverable wrapper over the same engine.
- Avoid repo-wide uncapped mapping by default.
- Keep map levels and symbol-kind filtering understandable without requiring users to learn Codemap internals.

### 5. Position against misuse, not against tools

The Codemap article works because it does not say Codemap is bad. It says Codemap is often the wrong first move for remote reconnaissance.

`ghx` should keep that framing:

- Use Codemap when the repo is local and deep structural analysis is the job.
- Use Aider repo maps when context selection belongs inside an active Aider coding loop.
- Use Repomix or Gitingest when the desired artifact is a packed repo digest.
- Use `ghx` when the agent is still outside the repo and needs to decide what to inspect next.

This creates trust. A tool that can say when not to use it is easier for agents and developers to route correctly.

## Consequences

### Positive

- The project direction is grounded in observed demand vocabulary without becoming SEO-led.
- The data supports the existing `ghx` thesis: progressive disclosure beats premature context loading.
- ADR-0013's parser-backed mapping direction gets independent evidence from search behavior.
- The product boundary becomes sharper: `ghx` is the remote reconnaissance layer.

### Negative

- Demand for `symbol:` and code-search syntax may create pressure to build features that GitHub's public APIs cannot honestly support.
- Zero clicks mean the data does not yet prove that searchers choose this framing after seeing it.
- Codemap vocabulary can pull the project toward local-index feature creep if the boundary is not enforced.

### Neutral

- This ADR does not require immediate implementation.
- It does require future docs, help text, and feature proposals to use the same decision framework.
- The next product work should be evaluated by whether it makes the next remote read cheaper, narrower, or easier to verify.

## Follow-Ups

1. Update the canonical `internal/skilldoc/SKILL.md` with the decision ladder.
2. Audit `ghx search --help` for honest GitHub public API limitations and web-only qualifier warnings.
3. Audit `ghx map --help` and README examples for path/glob, `--kind`, and map-level discoverability.
4. Consider a bounded repo/path search design that composes `tree`, glob filtering, batched reads, and local matching.
5. Re-check Search Console after the data has another 2-4 weeks of history, but use it only as product-research input.
