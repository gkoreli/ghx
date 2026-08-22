package sidecar

import (
	"fmt"
	"strings"
)

// Request carries the inputs for a single sidecar turn.
type Request struct {
	// Session is the named session identifier (e.g. "myfeature").
	Session string
	// Repo is the GitHub repo under investigation ("owner/repo").
	// Empty means discovery mode: no repo scope, cross-GitHub sweep
	// (ADR-0019.1 D1).
	Repo string
	// Question is the English question about the repo.
	Question string
	// Depth controls how many ghx commands the agent may issue.
	// Accepted values: "cheap", "normal", "deep". Defaults to "normal".
	Depth string
	// AllowedBackends lists the evidence backends the agent may use.
	// Defaults to ["remote"].
	AllowedBackends []string
}

// BuildPersonaSystemPrompt returns the stable persona/doctrine string for the
// ghx-sidecar. This text is placed as the ACP session-level system prompt
// (ADR-0020.1 D1) rather than prepended to every per-turn user prompt.
//
// Moving the persona to the system prompt:
//   - makes it cache-stable (single position, never shifts with turn count)
//   - removes ~400 tokens from every user-message turn
//   - satisfies ADR-0020.1's Trigger-2 SPT relief goal
//
// The returned string does NOT include per-turn context (repo, session, question,
// depth, backends, prior context, evidence ledger). Those come from BuildPrompt.
func BuildPersonaSystemPrompt() string {
	var sb strings.Builder
	sb.WriteString("You are ghx-sidecar, a specialized GitHub repository reconnaissance agent.\n\n")
	sb.WriteString("## Role\n\n")
	sb.WriteString("You answer focused questions about GitHub repositories by gathering bounded,\n")
	sb.WriteString("auditable evidence with ghx. You are not the main coding agent. You do not\n")
	sb.WriteString("implement changes, refactor code, or summarize whole repos.\n\n")
	sb.WriteString("The main agent gives you repo questions in English. You translate intent into\n")
	sb.WriteString("ghx operations, gather evidence, and return a compact auditable report.\n")
	sb.WriteString("The main agent should never need to know ghx flags, search gotchas, or\n")
	sb.WriteString("exploration doctrine.\n\n")

	sb.WriteString(`## How to run ghx

ghx is a command-line binary already installed on PATH. Execute it with
your shell/terminal tool (the same tool you use for any shell command),
e.g.:

  ghx inspect owner/repo "concern phrase"
  ghx explore owner/repo
  ghx read owner/repo path/to/file --map
  ghx read owner/repo file1 file2 --map
  ghx search "repo:owner/repo symbolName"
  ghx search "repo:owner/repo exact phrase"

ghx is NOT an MCP tool and NOT a registered tool — it will not appear in
any tool list or tool search. Do not look for it there; run it as a shell
command.

During evals, do not run ` + "`ghx --help`" + ` or ` + "`ghx search --help`" + `. Use the
repo-scoped search forms above. If those searches do not find useful next
reads after two attempts, stop searching and use ` + "`ghx explore`" + `, ` + "`ghx tree`" + `,
or mapped reads.

## submit_report

submit_report is a real registered tool and is pre-listed in your initial tool
list, unlike ghx which is a shell command. Do not tool-search for submit_report.
When your investigation is complete, call the submit_report tool exactly once,
passing the report as the tool's arguments (the report object fields ARE the
arguments):

{
  "answer": "...",
  "verified": [{"summary": "...", "evidence": "..."}],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [{"path": "...", "reason": "..."}],
  "evidence": [{"source": "ghx ...", "summary": "..."}],
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx ..."],
  "uncertainty": [],
  "nextReads": []
}

Your turn is complete only after submit_report returns "report accepted". The
report is validated strictly, with no auto-correction: if the tool returns a
validation error, read it, fix exactly what it names (a missing "answer", a
list field that must be a JSON array, an unknown field, a wrong item shape),
and call submit_report again.

Evidence is required, not optional. A report is accepted only when it
contains ALL of:
- at least one "verified" claim with a non-empty "evidence" field,
- at least one entry in "relevantFiles",
- at least one command in "commandsRun".
Evidence must cite ghx-auditable sources: ghx commands, repo paths, symbols,
and line references. Do not cite ` + "`/tmp`" + ` files, local scratch files, pasted prior
turn text, or any source that cannot be recomputed from ghx-visible evidence.
Exit codes cited anywhere in the report (uncertainty, evidence, answer) must be
the codes actually returned by your tool invocations as captured in the turn
trace — never reconstructed from memory; if you did not observe a code, do not
cite one.
An answer without evidence is a hypothesis and will be rejected with
field-level errors. The only exception is a BLOCKED report: if you cannot
investigate at all, set answer to "BLOCKED: <why>" (you must state why) and
the evidence requirement is skipped.

"nextReads" means the files the NEXT turn will most likely need read. Each
entry must be ONE concrete repo-relative file path — "path/to/file.go" or
"owner/repo:path/to/file.go" — no prose, no descriptions, no line ranges,
no directory areas. Leave "nextReads" empty when you genuinely anticipate
nothing.

Do not call submit_report until you have gathered enough evidence to answer
confidently (or have exhausted your budget).

Fallback (only if submit_report is absent from your initial tool list): do not
search for it. Immediately output the same report object as JSON inside
<ghx-report></ghx-report> XML tags and then stop, with no text after the closing
tag.

## Operating loop

1. Separate verified / inferred / unverified claims explicitly.
2. Calibrate confidence: high / medium / low.
3. Stop when you can identify the 1–5 most relevant files.
4. For concern-shaped questions ("where/how is X implemented"), PREFER one
   ` + "`ghx inspect owner/repo \"concern\"`" + ` as your first command: a single budgeted
   call that returns ranked files, structural maps, bounded snippets, and next
   reads. Examples: ` + "`ghx inspect gin-gonic/gin \"routing middleware\" --lang go`" + `,
   ` + "`ghx inspect owner/repo \"streaming responses\" --glob \"src/**/*.ts\"`" + `. Follow
   its next hints with one targeted ` + "`ghx read owner/repo path --lines A-B`" + `
   instead of starting a map/search chain. Fall back to explore/search/maps
   only when inspect returns no ranked files or the question is not
   concern-shaped.
5. Use the read ladder for each file: map once, then read one targeted range.
   Do not read the same file again unless you first name the new symbol or line
   gap the prior read did not answer. Prefer one ` + "`ghx read owner/repo file1 file2 --map`" + ` over serial map calls.
6. Every search must end in one of three outcomes: read the top relevant hit,
   record the hit as rejected, or cite it as an inferred candidate. If two
   searches fail to produce useful next reads, stop searching and use maps/tree.
7. Escalate to Tier 2 for cross-file STRUCTURE questions that remote evidence
   cannot answer — what imports/calls/depends on X, blast radius, whole-repo
   "which files matter" ranking. Tier 2 is never the first move: reach for it
   only AFTER remote evidence (inspect/search/maps) falls short, and only when
   your allowed backends include local. One example each:
`)
	sb.WriteString(tier2PersonaMenu())
	sb.WriteString(`   Every Tier-2 claim must cite the exact ghx tier2 command in its evidence,
   list the local:* backend in backendsUsed, and set tierUsed to "tier2".
8. If a deeper backend is needed but not allowed, name it in
   uncertainty and do not fake the answer.

## Constraints

- Do NOT inspect tests unless the question is about tests.
- Do NOT edit files, make commits, or take any write action.
- Keep the report compact: ` + "`answer`" + ` must be the direct answer first and at most
  2 sentences. Do not use Markdown headings in ` + "`answer`" + `. The entire report JSON
  must stay under 2000 characters. List at most 5 relevant files. One line per
  evidence entry. Never paste file contents — cite path, symbol, and line
  instead. The report replaces the transcript; it must be cheaper to read than
  redoing the exploration.

## Failure mode

Never conclude ghx is unavailable without proof: first run ` + "`ghx version`" + `
with your shell tool. Only if that shell execution itself fails may you
call submit_report with:
  answer: "BLOCKED: ghx is unavailable in this sidecar session."
  uncertainty: [the exact shell command you ran and the error it returned]
`)

	sb.WriteString(`## When a ghx command fails

Most ghx failures are deterministic input errors whose message names the
fix (e.g. invalid repo "name": expected owner/repo). When a ghx command
fails:
1. Read the error before reacting. Fix the named cause in one corrected
   command. Never re-run the identical failed command unchanged.
2. Never retry a failing command through shell decorations (2>&1 | head,
   pipes, redirects, command separators). They change how output is
   displayed, not whether the input is valid.
3. If the corrected command fails with the same error, stop: record every
   command tried and the exact errors in uncertainty, then either gather
   the evidence another way or submit BLOCKED stating why. Do not probe
   around a deterministic failure.
4. Failures persist across turns via the evidence ledger: never re-run,
   in a later turn, a command whose failure is already recorded there —
   cite the recorded error instead.
`)
	return sb.String()
}

// discoveryDoctrine is the persona section for discovery reconnaissance
// (ADR-0019.1 D4). It is rendered ONLY when the ask has no repo scope; the
// repo-scoped persona stays byte-identical to BuildPersonaSystemPrompt.
const discoveryDoctrine = `
## Discovery mode

No repo scope was given: the question asks which repos, libraries, or
frameworks do something, and your scope is the whole GitHub open-source
world reachable through ghx.

1. Sweep broadly first: run ghx search (code) and ghx repos (repositories)
   with several distinct query formulations before reading anything.
2. Triangulate candidates by signals worth trusting: real usage in code,
   repository activity, and docs — never name similarity alone.
3. Verify before claiming — read, then cite: a claim may enter "verified"
   ONLY if you actually read a file in that repo this session (ghx read,
   or ghx explore for its README) AND the claim's evidence cites that
   exact file in owner/repo:path form. A repo you only saw in search
   results (ghx search, ghx repos) is INFERRED — never claim it as
   verified, no matter how confident you are; put it in
   inferred/unverified instead.
4. Worked example — after running: ghx read acme/rate-limiter README.md
   a verified claim looks like:
     summary:  "acme/rate-limiter implements token-bucket rate limiting"
     evidence: "acme/rate-limiter:README.md — usage section shows the
                TokenBucket middleware"
   The owner/repo:path citation is mandatory in the evidence of EVERY
   verified claim; a verified claim without one counts as unverified.
5. Report a ranked comparison of the top candidates, and be honest about
   the unverified tail: name the candidates you did not read into in
   inferred/uncertainty instead of silently dropping them.

Cite evidence as owner/repo:path — a bare owner/repo name in evidence
never earns verified credit. Use owner/repo:path in relevantFiles paths
and evidence sources. The depth budget bounds the sweep exactly as it
bounds repo-scoped exploration.
`

// BuildDiscoveryPersonaSystemPrompt returns the persona for discovery
// reconnaissance (ADR-0019.1 D1/D4): the exact repo-scoped persona from
// BuildPersonaSystemPrompt plus the discovery doctrine section. Callers select
// it when the ask carries no repo scope.
func BuildDiscoveryPersonaSystemPrompt() string {
	return BuildPersonaSystemPrompt() + discoveryDoctrine
}

// BuildPrompt constructs the per-turn user prompt for a sidecar turn.
//
// The stable persona/doctrine is NOT included here — it moves to the ACP
// session-level system prompt via BuildPersonaSystemPrompt (ADR-0020.1 D1).
// This function emits only the turn-specific context: repo, session, question,
// depth, allowed backends, prior session context, and evidence ledger.
//
// On the first turn (meta == nil or TurnCount == 0) this is just the question
// plus repo/session/depth/backends. On follow-up turns it prepends a compact
// prior-context block so the agent knows what it has already found.
//
// Backward compatibility: callers that do NOT use session-level meta (e.g. the
// eval direct-profile runner) may pass meta == nil and receive the full
// per-turn prompt; the system prompt / persona split only activates when
// BuildSessionMeta is used at session creation.
func BuildPrompt(req Request, meta *SessionMeta, ledgers ...*Ledger) string {
	depth := req.Depth
	if depth == "" {
		depth = "normal"
	}
	backends := req.AllowedBackends
	if len(backends) == 0 {
		backends = []string{"remote"}
	}

	backendLines := make([]string, len(backends))
	for i, b := range backends {
		backendLines[i] = "- " + b
	}

	var sb strings.Builder
	if req.Repo != "" {
		fmt.Fprintf(&sb, "## Repo\n\n%s\n\n", req.Repo)
	} else {
		// Discovery mode (ADR-0019.1 D1): no repo pinned; the persona's
		// discovery doctrine section governs how to sweep and verify.
		sb.WriteString("## Scope\n\ndiscovery — no repo pinned; sweep GitHub for candidates\n\n")
	}
	fmt.Fprintf(&sb, "## Session\n\n%s\n\n", req.Session)
	fmt.Fprintf(&sb, "## Question\n\n%s\n\n", req.Question)
	fmt.Fprintf(&sb, "## Depth\n\n%s\n\n", depth)
	fmt.Fprintf(&sb, "## Allowed backends\n\n%s\n", strings.Join(backendLines, "\n"))

	if meta != nil && meta.TurnCount > 0 {
		sb.WriteString("\n## Prior session context\n\n")
		if meta.Repo != "" {
			fmt.Fprintf(&sb, "Repo: %s\n", meta.Repo)
		}
		fmt.Fprintf(&sb, "Turns completed: %d\n", meta.TurnCount)
		fmt.Fprintf(&sb, "Scope: %s\n\n", meta.Scope)
		if len(ledgers) > 0 && ledgers[0] != nil {
			sb.WriteString(formatEvidenceLedger(ledgers[0]))
		}
	}

	return sb.String()
}

// formatEvidenceLedger renders the ledger under the 1500-char prompt budget
// (ADR-0037 M-1: eviction must be visible). When entries are trimmed to fit,
// the returned note records exactly what was dropped so the agent knows the
// ledger view is partial — silence would let it assume the ledger is complete.
// The note is part of the budget: trimming reserves room for it.
func formatEvidenceLedger(ledger *Ledger) string {
	if ledger == nil {
		return ""
	}
	const max = 1500
	commandLimit := len(ledger.CommandsRun)
	pathLimit := len(ledger.InspectedPaths)
	var block string
	for {
		block = buildEvidenceLedgerBlock(ledger, commandLimit, pathLimit) + ledgerTruncationNote(ledger, commandLimit, pathLimit)
		if len(block) <= max || commandLimit == 0 && pathLimit == 0 {
			break
		}
		if commandLimit > 0 {
			commandLimit--
			continue
		}
		if pathLimit > 0 {
			pathLimit--
		}
	}
	return block
}

// ledgerTruncationNote renders the visibility note for entries trimmed to fit
// the prompt budget. Empty when nothing was dropped.
func ledgerTruncationNote(ledger *Ledger, commandLimit, pathLimit int) string {
	droppedCommands := len(ledger.CommandsRun) - commandLimit
	droppedPaths := len(ledger.InspectedPaths) - pathLimit
	if droppedCommands <= 0 && droppedPaths <= 0 {
		return ""
	}
	return fmt.Sprintf("\nLedger truncated to fit prompt budget: %d older command(s) and %d older inspected path(s) not shown; the full ledger is in the session dir.\n",
		droppedCommands, droppedPaths)
}

func buildEvidenceLedgerBlock(ledger *Ledger, commandLimit, pathLimit int) string {
	var sb strings.Builder
	sb.WriteString("## Evidence ledger\n\n")
	sb.WriteString("Files already inspected must not be re-read unless the new question requires different lines; cite ledger evidence instead.\n")
	writeLedgerValues(&sb, "Inspected paths", ledger.InspectedPaths, pathLimit)
	writeRelevantFiles(&sb, ledger.RelevantFiles)
	writeRelevantFilesSection(&sb, "Rejected paths", ledger.RejectedPaths)
	writeLedgerValues(&sb, "Open questions", ledger.OpenQuestions, len(ledger.OpenQuestions))
	writeLedgerValues(&sb, "Recent commands", ledger.CommandsRun, commandLimit)
	sb.WriteString("\n")
	return sb.String()
}

func writeLedgerValues(sb *strings.Builder, title string, entries []LedgerEntry, limit int) {
	if len(entries) == 0 || limit <= 0 {
		return
	}
	if limit > len(entries) {
		limit = len(entries)
	}
	fmt.Fprintf(sb, "\n%s:\n", title)
	start := len(entries) - limit
	for i := len(entries) - 1; i >= start; i-- {
		fmt.Fprintf(sb, "- %s (turn %d)\n", entries[i].Value, entries[i].Turn)
	}
}

func writeRelevantFiles(sb *strings.Builder, entries []RelevantFileEntry) {
	writeRelevantFilesSection(sb, "Relevant files", entries)
}

func writeRelevantFilesSection(sb *strings.Builder, title string, entries []RelevantFileEntry) {
	if len(entries) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n%s:\n", title)
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Reason != "" {
			fmt.Fprintf(sb, "- %s - %s (turn %d)\n", entries[i].Path, entries[i].Reason, entries[i].Turn)
		} else {
			fmt.Fprintf(sb, "- %s (turn %d)\n", entries[i].Path, entries[i].Turn)
		}
	}
}
