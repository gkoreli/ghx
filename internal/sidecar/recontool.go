package sidecar

import "github.com/mark3labs/mcp-go/mcp"

// Recon MCP tool contract (ADR-0019/0020.1 consumption surface; result
// contract in reconresult.go, ADR-0019.3 D2). `ghx serve` (the recon-first
// default) exposes exactly one tool that takes a whole English repo question
// and returns the sidecar's auditable evidence report as pure JSON. The
// definition lives here — next to the report-sink tool contract — so the CLI
// server (internal/cli/serve.go) and the host-task eval identity inventory
// (ADR-0032.1 S3: the frozen MCP tool schema hash) share one authority and
// the hashed schema can never drift from the served one.
const (
	// ReconToolName is the single tool the default `ghx serve` mode exposes.
	ReconToolName = "recon"
)

// ReconMCPTool returns the recon tool definition served by the default `ghx
// serve` mode. The marshaled JSON (name, description, input schema) is the
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
		mcp.WithDescription("ghx is a specialist agent that explores GitHub for you: ask a repo question in plain English and it returns a compact evidence report. Do not explore a remote repo step-by-step yourself — delegate the whole question and let ghx do the reading, mapping, and searching under the hood.\n\nThe report contains: answer (the direct answer); verified (claims ghx backed by evidence it actually read — trust these); evidence / relevantFiles (the sources and where to look); uncertainty / nextReads (what it could not confirm and what to read next). Every answer ends with an artifacts pointer (session dir + trace id) — the on-disk audit trail behind the report. State the goal, not the steps, and follow up rather than bundling several asks together; a fresh question takes tens of seconds, follow-ups are faster."),
		mcp.WithString("question", mcp.Required(), mcp.Description("The reconnaissance goal, in plain English. State what you want to know, not how to find it: \"How does hono implement middleware chaining, and which files define it?\" beats \"grep for middleware\". One question per call — follow up rather than bundling several asks together.")),
		mcp.WithString("repo", mcp.Description("owner/repo to scope the question to one repository. Optional: omit it for discovery questions that sweep across GitHub (\"which repos / libraries do X\") — ghx finds and verifies the candidates itself.")),
		mcp.WithString("session", mcp.Description("Advanced, rarely needed: pin this call to a specific investigation thread. Normally omit — ghx routes follow-ups to the right session for you.")),
		mcp.WithString("depth", mcp.Description("effort/budget dial: cheap|normal|deep (default normal). normal is the everyday ~8-command contract; cheap trades depth for speed on quick lookups; deep doubles the budget for hard or cross-cutting questions. Omit to get normal.")),
	)
}
