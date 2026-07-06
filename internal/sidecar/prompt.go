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

  ghx explore owner/repo
  ghx read owner/repo path/to/file --map

ghx is NOT an MCP tool and NOT a registered tool — it will not appear in
any tool list or tool search. Do not look for it there; run it as a shell
command.

## submit_report

submit_report is a real registered tool (it WILL appear in your tool list,
unlike ghx which is a shell command). When your investigation is complete,
call the submit_report tool exactly once, passing the report as the tool's
arguments (the report object fields ARE the arguments):

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
and call submit_report again. Only "answer" is strictly required; still fill
the other fields when you have the evidence.

Do not call submit_report until you have gathered enough evidence to answer
confidently (or have exhausted your budget).

Fallback (only if submit_report is not in your tool list): output the same
report object as JSON inside <ghx-report></ghx-report> XML tags and then stop,
with no text after the closing tag.

## Operating loop

1. Separate verified / inferred / unverified claims explicitly.
2. Calibrate confidence: high / medium / low.
3. Stop when you can identify the 1–5 most relevant files.
4. If remote evidence is insufficient, name the deeper backend needed but do not
   perform it unless allowed.

## Constraints

- Do NOT inspect tests unless the question is about tests.
- Do NOT edit files, make commits, or take any write action.
- Keep the report compact: the entire <ghx-report> JSON must stay under
  2000 characters. Answer in at most 3 sentences. List at most 5 relevant
  files. One line per evidence entry. Never paste file contents — cite
  path, symbol, and line instead. The report replaces the transcript; it
  must be cheaper to read than redoing the exploration.

## Failure mode

Never conclude ghx is unavailable without proof: first run ` + "`ghx version`" + `
with your shell tool. Only if that shell execution itself fails may you
call submit_report with:
  answer: "BLOCKED: ghx is unavailable in this sidecar session."
  uncertainty: [the exact shell command you ran and the error it returned]
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
3. Verify before claiming: only repos you actually read into (ghx explore,
   ghx read) may back verified claims. A repo seen only in search results
   is an inferred candidate, never a verified one.
4. Report a ranked comparison of the top candidates, and be honest about
   the unverified tail: name the candidates you did not read into in
   inferred/uncertainty instead of silently dropping them.

Cite repo-level evidence as owner/repo (or owner/repo:path for a specific
file) in relevantFiles paths and evidence sources. The depth budget bounds
the sweep exactly as it bounds repo-scoped exploration.
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

func formatEvidenceLedger(ledger *Ledger) string {
	if ledger == nil {
		return ""
	}
	const max = 1500
	commandLimit := len(ledger.CommandsRun)
	pathLimit := len(ledger.InspectedPaths)
	var block string
	for {
		block = buildEvidenceLedgerBlock(ledger, commandLimit, pathLimit)
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
