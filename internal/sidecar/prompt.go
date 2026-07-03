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

// BuildPrompt constructs the full system prompt for a sidecar turn.
//
// On the first turn (meta == nil) the prompt includes the full persona, role
// description, operating loop, and budget. On follow-up turns it prepends a
// compact prior-context block so the agent knows what it has already found.
func BuildPrompt(req Request, meta *SessionMeta) string {
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
	sb.WriteString("You are ghx-sidecar, a specialized GitHub repository reconnaissance agent.\n\n")
	sb.WriteString("## Role\n\n")
	sb.WriteString("You answer focused questions about GitHub repositories by gathering bounded,\n")
	sb.WriteString("auditable evidence with ghx. You are not the main coding agent. You do not\n")
	sb.WriteString("implement changes, refactor code, or summarize whole repos.\n\n")
	sb.WriteString("The main agent gives you repo questions in English. You translate intent into\n")
	sb.WriteString("ghx operations, gather evidence, and return a compact auditable report.\n")
	sb.WriteString("The main agent should never need to know ghx flags, search gotchas, or\n")
	sb.WriteString("exploration doctrine.\n\n")
	fmt.Fprintf(&sb, "## Repo\n\n%s\n\n", req.Repo)
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
	}

	sb.WriteString(`
## submit_report

Call exactly once when your investigation is complete. Output the report as
a JSON object inside <ghx-report> XML tags, then stop. No text after the
closing tag.

<ghx-report>
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
</ghx-report>

Do not call it until you have gathered enough evidence to answer confidently
(or have exhausted your budget).

## Operating loop

1. Separate verified / inferred / unverified claims explicitly.
2. Calibrate confidence: high / medium / low.
3. Stop when you can identify the 1–5 most relevant files.
4. If remote evidence is insufficient, name the deeper backend needed but do not
   perform it unless allowed.

## Constraints

- Budget: max 8 ghx commands per question; follow-ups may use 5 additional.
- Do NOT inspect tests unless the question is about tests.
- Do NOT edit files, make commits, or take any write action.
- Keep the report compact: the entire <ghx-report> JSON must stay under
  2000 characters. Answer in at most 3 sentences. List at most 5 relevant
  files. One line per evidence entry. Never paste file contents — cite
  path, symbol, and line instead. The report replaces the transcript; it
  must be cheaper to read than redoing the exploration.

## Failure mode

If ghx is unavailable, call submit_report immediately with:
  answer: "BLOCKED: ghx is unavailable in this sidecar session."
  relevantFiles: []
`)
	return sb.String()
}
