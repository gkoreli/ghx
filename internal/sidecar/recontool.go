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
// Params: question (required), plus optional smart-defaulted scope — repo
// (omit for cross-GitHub discovery) and session (advanced pin) — and the
// depth dial (NORTH_STAR capability 3: cheap|normal|deep, defaulting to
// normal). The common case needs only question.
func ReconMCPTool() mcp.Tool {
	return mcp.NewTool(ReconToolName,
		mcp.WithDescription("Ask ghx repo questions in English; returns a compact, auditable evidence report. Delegate the whole reconnaissance question instead of step-driving repository exploration. Follow-up questions are routed to the right investigation session automatically (the route is reported with each answer)."),
		mcp.WithString("question", mcp.Required(), mcp.Description("English question about the repo")),
		mcp.WithString("repo", mcp.Description("owner/repo (optional scope; omit for cross-GitHub discovery questions like \"which repos do X\")")),
		mcp.WithString("session", mcp.Description("advanced: pin a specific session; normally omit — ghx routes for you (ADR-0030.1)")),
		mcp.WithString("depth", mcp.Description("effort/budget dial: cheap|normal|deep (default normal). normal is the everyday ~8-command contract; cheap trades depth for speed on quick lookups; deep doubles the budget for hard or cross-cutting questions. Omit to get normal.")),
	)
}
