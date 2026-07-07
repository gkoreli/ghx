package sidecar

import "github.com/mark3labs/mcp-go/mcp"

// Recon MCP tool contract (ADR-0019/0020.1 consumption surface). `ghx serve
// --recon` exposes exactly one tool that takes a whole English repo question
// and returns the sidecar's auditable evidence report. The definition lives
// here — next to the report-sink tool contract — so the CLI server
// (internal/cli/serve.go) and the host-task eval identity inventory
// (ADR-0032.1 S3: the frozen MCP tool schema hash) share one authority and
// the hashed schema can never drift from the served one.
const (
	// ReconToolName is the single tool `ghx serve --recon` exposes.
	ReconToolName = "recon"
)

// ReconMCPTool returns the recon tool definition served by `ghx serve
// --recon`. The marshaled JSON (name, description, input schema) is the
// frozen identity ADR-0032.1 S3 hashes into host-task run manifests, so any
// change here is a measurement-identity change for host-task evals.
//
// Params: question (required), plus optional smart-defaulted dials — repo
// (omit for cross-GitHub discovery, ADR-0019.1), session (advanced pin; ghx
// routes follow-ups automatically otherwise, ADR-0030.1), and depth
// (NORTH_STAR capability 3: cheap|normal|deep, defaulting to normal). The
// common case needs only question.
func ReconMCPTool() mcp.Tool {
	return mcp.NewTool(ReconToolName,
		mcp.WithDescription("Delegate a whole code-reconnaissance question to the ghx sidecar and get back a compact, auditable evidence report. ghx explores GitHub for you — do not step-drive repository exploration yourself; ask the goal in plain English and let ghx do the reading, mapping, and searching under the hood.\n\nThe report contains: answer (the direct answer); verified (claims ghx backed by evidence it actually read — trust these); evidence / relevantFiles (the sources and where to look); uncertainty / nextReads (what it could not confirm and what to read next). Every answer ends with an artifacts pointer (session dir + trace id) — the on-disk audit trail behind the report. Follow-up questions are routed to the right investigation session automatically, and the route is reported with each answer, so you never manage sessions yourself. A fresh question takes tens of seconds; follow-ups on the same thread are faster."),
		mcp.WithString("question", mcp.Required(), mcp.Description("The reconnaissance goal, in plain English. State what you want to know, not how to find it: \"How does hono implement middleware chaining, and which files define it?\" beats \"grep for middleware\". One question per call — follow up rather than bundling several asks together.")),
		mcp.WithString("repo", mcp.Description("owner/repo to scope the question to one repository. Optional: omit it for discovery questions that sweep across GitHub (\"which repos / libraries do X\") — ghx finds and verifies the candidates itself.")),
		mcp.WithString("session", mcp.Description("Advanced, rarely needed: pin this call to a specific investigation thread. Normally omit — ghx routes follow-ups to the right session for you.")),
		mcp.WithString("depth", mcp.Description("effort/budget dial: cheap|normal|deep (default normal). normal is the everyday ~8-command contract; cheap trades depth for speed on quick lookups; deep doubles the budget for hard or cross-cutting questions. Omit to get normal.")),
	)
}
