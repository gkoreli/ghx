package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/codemode"
	ghxlib "github.com/gkoreli/ghx/v2/internal/ghx"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// Transport defines the interface for MCP server transports
type Transport interface {
	Serve(s *server.MCPServer) error
}

// StdioTransport implements Transport for stdio communication
type StdioTransport struct{}

func (t *StdioTransport) Serve(s *server.MCPServer) error {
	return server.ServeStdio(s)
}

// HTTPTransport implements Transport for streamable HTTP communication
type HTTPTransport struct {
	Address string
}

func (t *HTTPTransport) Serve(s *server.MCPServer) error {
	httpServer := server.NewStreamableHTTPServer(s)
	fmt.Fprintf(os.Stderr, "ghx MCP server listening on %s\n", t.Address)
	return http.ListenAndServe(t.Address, httpServer)
}

// selectTransport returns the appropriate transport based on CLI flags
func selectTransport(cmd *cobra.Command) Transport {
	httpAddr, _ := cmd.Flags().GetString("http")
	if httpAddr != "" {
		return &HTTPTransport{Address: httpAddr}
	}
	return &StdioTransport{}
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio)",
	Long: `Run ghx as an MCP server so an agent can call its tools over the protocol.
Default (stdio) serves exactly one ` + "`recon`" + ` tool that takes a whole English repo
question and returns a machine-parseable JSON object — the auditable evidence
report plus route and artifacts provenance inside the payload — so a main agent
delegates reconnaissance instead of step-driving it (ADR-0019.3 D1/D2).
` + "`--direct`" + ` instead exposes the seven direct ghx tools plus the ` + "`code`" + ` meta-tool
for composed exploration (the pre-0019.3 default). ` + "`--recon`" + ` is still accepted
as a no-op synonym of the new default (deprecated; remove it from your config).
` + "`--http :PORT`" + ` switches from stdio to a streamable HTTP server on that address.`,
	Example: `  ghx serve
  ghx serve --http :8080
  ghx serve --direct
  ghx serve --recon`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if printCfg, _ := cmd.Flags().GetBool("print-mcp-config"); printCfg {
			return printMCPConfig()
		}
		return serveMCP(cmd)
	},
}

// printMCPConfig writes a ready-to-paste mcpServers JSON block (ADR-0019.3
// D4) so the install step for any MCP client is: paste this, run doctor.
func printMCPConfig() error {
	block := map[string]any{
		"mcpServers": map[string]any{
			"ghx": map[string]any{
				"command": "npx",
				"args":    []string{"-y", "@gkoreli/ghx", "serve"},
			},
		},
	}
	data, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func serveMCP(cmd *cobra.Command) error {
	direct, _ := cmd.Flags().GetBool("direct")
	s := newServeServer(direct)
	return selectAndServe(cmd, s)
}

// selectAndServe picks the transport from the command's flags and blocks
// serving the server over it.
func selectAndServe(cmd *cobra.Command, s *server.MCPServer) error {
	transport := selectTransport(cmd)
	return transport.Serve(s)
}

// newServeServer builds the MCP server for the given mode: direct=true
// registers the seven direct ghx tools plus the code meta-tool; direct=false
// is the recon-first default (ADR-0019.3 D1) serving exactly the single
// recon sidecar tool.
func newServeServer(direct bool) *server.MCPServer {
	s := server.NewMCPServer("ghx", VERSION,
		server.WithToolCapabilities(true),
	)
	if !direct {
		registerReconTool(s)
		return s
	}
	registerDirectTools(s)
	return s
}

// registerDirectTools registers the seven direct exploration tools plus the
// code meta-tool — the P1 standalone surface behind `ghx serve --direct`
// (ADR-0019.3 D1) and the eval baseline profile.
func registerDirectTools(s *server.MCPServer) {
	// Define tools
	exploreTool := mcp.NewTool("explore",
		mcp.WithDescription("Explore a GitHub repo — branch, file tree, README"),
		mcp.WithString("repo", mcp.Required(), mcp.Description("owner/repo")),
		mcp.WithString("path", mcp.Description("subdirectory path")),
	)

	reposTool := mcp.NewTool("repos",
		mcp.WithDescription("Search GitHub repositories with README preview"),
		mcp.WithString("query", mcp.Required(), mcp.Description("search query")),
		mcp.WithNumber("limit", mcp.Description("max results (default 10, max 20)")),
	)

	searchTool := mcp.NewTool("search",
		mcp.WithDescription("Search code across GitHub with matching context"),
		mcp.WithString("query", mcp.Required(), mcp.Description("search query")),
		mcp.WithNumber("limit", mcp.Description("max results (default 30, max 100)")),
		mcp.WithBoolean("full", mcp.Description("show complete fragments without truncation")),
	)

	readTool := mcp.NewTool("read",
		mcp.WithDescription("Read 1-10 files from a GitHub repo in one API call"),
		mcp.WithString("repo", mcp.Required(), mcp.Description("owner/repo")),
		mcp.WithString("paths", mcp.Required(), mcp.Description("comma-separated file paths")),
		mcp.WithString("grep", mcp.Description("filter to matching lines with context")),
		mcp.WithString("lines", mcp.Description("line range (e.g., 42-80)")),
		mcp.WithBoolean("map", mcp.Description("structural signatures only")),
		mcp.WithString("level", mcp.Description("map detail level: outline|minimal|compact|standard")),
		mcp.WithString("kind", mcp.Description("map symbol kind filter: func|type|import|const|var|package")),
		mcp.WithString("mapEngine", mcp.Description("map engine: auto|regex|tree-sitter")),
	)

	treeTool := mcp.NewTool("tree",
		mcp.WithDescription("Full recursive file tree listing for a repo"),
		mcp.WithString("repo", mcp.Required(), mcp.Description("owner/repo")),
		mcp.WithString("path", mcp.Description("subdirectory path")),
		mcp.WithNumber("depth", mcp.Description("limit tree depth (0 = full recursive)")),
	)

	searchToolsMeta := mcp.NewTool("search_tools",
		mcp.WithDescription("Search available tools and get TypeScript type stubs for writing code scripts"),
		mcp.WithString("query", mcp.Description("search query (optional, returns all if empty)")),
	)

	reg := buildRegistry()
	tools := reg.List()
	stubs := codemode.GenerateTypes(tools)

	codeDescription := fmt.Sprintf(`Execute code to achieve a goal.

Available:

%s

Write a plain JavaScript function body that returns the result.
Do NOT use await — all codemode calls are synchronous.
Do NOT use TypeScript syntax — no type annotations, interfaces, or generics.

Example: var r = codemode.explore({ repo: "vercel/next.js" }); return r.files;`, stubs)

	codeTool := mcp.NewTool("code",
		mcp.WithDescription(codeDescription),
		mcp.WithString("code", mcp.Required(), mcp.Description("JavaScript async arrow function to execute")),
	)

	// Register tools with handlers
	s.AddTool(exploreTool, handleExplore)
	s.AddTool(reposTool, handleRepos)
	s.AddTool(searchTool, handleSearch)
	s.AddTool(readTool, handleRead)
	s.AddTool(treeTool, handleTree)
	s.AddTool(searchToolsMeta, handleSearchTools)
	s.AddTool(codeTool, handleCode)
}

// registerReconTool serves the single recon tool. Its definition lives in
// sidecar.ReconMCPTool so the served schema and the host-task eval identity
// hash (ADR-0032.1 S3) can never drift apart.
func registerReconTool(s *server.MCPServer) {
	s.AddTool(sidecar.ReconMCPTool(), handleRecon)
}

var askSidecar = func(ctx context.Context, cfg sidecar.Config, req sidecar.AskRequest) (*sidecar.Report, *sidecar.TurnResult, error) {
	report, turn, _, err := sidecar.AskViaDaemon(ctx, VERSION, cfg, req)
	return report, turn, err
}

func handleRecon(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	question, err := request.RequireString("question")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	repo := request.GetString("repo", "")
	// Session routing lives in the daemon (ADR-0030.1 D4 phase 1): the tool
	// passes the caller's parameters through untouched. An explicit session
	// still wins (R1); a repo still names the repo-slug session (R2); with
	// neither, the daemon evaluates the R3-R5 cascade instead of the old
	// local question-slug defaulting.
	session := request.GetString("session", "")

	// Depth dial (NORTH_STAR capability 3): forward the caller's requested
	// recon budget through the same AskRequest.Depth field the CLI `--depth`
	// flag uses (session_options.go depthBudgets), so the MCP consumer path
	// and `ask --depth` behave identically. Omitting depth keeps today's
	// smart default ("normal"). An invalid value is rejected with the valid
	// set named, rather than silently coerced — the tool must not quietly do
	// something other than what the agent asked.
	depth := request.GetString("depth", "normal")
	parsedDepth, ok := sidecar.ParseDepth(depth)
	if !ok {
		return mcp.NewToolResultError(fmt.Sprintf(
			"invalid depth %q: valid values are cheap, normal, deep", depth)), nil
	}

	cfg := sidecar.LoadConfig()
	report, turn, err := askSidecar(ctx, cfg, sidecar.AskRequest{
		Session:  session,
		Repo:     repo,
		Question: question,
		Depth:    parsedDepth.String(),
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	// Contract purity (ADR-0019.3 D2): the result text is exactly one JSON
	// object — {report, route, artifacts} — with the provenance that used to
	// be glued on as prose lines (route.Line() + artifacts footer) now inside
	// the payload as structured fields. No prose outside the JSON, ever;
	// humans keep rich text on `ghx sidecar ask`.
	text, merr := json.Marshal(sidecar.NewReconResult(report, turn))
	if merr != nil {
		return mcp.NewToolResultError(merr.Error()), nil
	}
	return mcp.NewToolResultText(string(text)), nil
}

// isValidReconDepth reports whether depth is one of the recon budget dials
// the sidecar accepts (session_options.go depthBudgets). The handler rejects
// anything else instead of letting the daemon silently coerce it to normal,
// so the MCP consumer gets a clear error naming the valid options.
func isValidReconDepth(depth string) bool {
	_, ok := sidecar.ParseDepth(depth)
	return ok
}

func handleExplore(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo, err := request.RequireString("repo")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	parsedRepo, err := ghxlib.ParseRepo(repo)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path := request.GetString("path", "")

	result, err := ghxlib.Explore(parsedRepo, path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func handleRepos(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	limit := request.GetInt("limit", 10)

	results, total, err := ghxlib.Repos(query, ghxlib.ReposOpts{Limit: limit})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	response := map[string]interface{}{
		"total":   total,
		"results": results,
	}
	data, _ := json.Marshal(response)
	return mcp.NewToolResultText(string(data)), nil
}

func handleSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	limit := request.GetInt("limit", 30)
	fullMode := request.GetBool("full", false)

	result, err := ghxlib.Search(query, ghxlib.SearchOpts{
		Limit:    limit,
		FullMode: fullMode,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func handleRead(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo, err := request.RequireString("repo")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	parsedRepo, err := ghxlib.ParseRepo(repo)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	pathsArg, err := request.RequireString("paths")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	paths := strings.Split(pathsArg, ",")
	for i := range paths {
		paths[i] = strings.TrimSpace(paths[i])
	}

	grepPattern := request.GetString("grep", "")
	lineRange := request.GetString("lines", "")
	mapMode := request.GetBool("map", false)
	mapLevel := request.GetString("level", "")
	mapKind := request.GetString("kind", "")
	mapEngine := request.GetString("mapEngine", "")

	results, err := ghxlib.Read(parsedRepo, paths, &ghxlib.ReadOpts{
		Grep:      grepPattern,
		Lines:     lineRange,
		Map:       mapMode,
		MapLevel:  mapLevel,
		MapKind:   mapKind,
		MapEngine: mapEngine,
	})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, _ := json.Marshal(results)
	return mcp.NewToolResultText(string(data)), nil
}

func handleTree(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo, err := request.RequireString("repo")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	parsedRepo, err := ghxlib.ParseRepo(repo)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path := request.GetString("path", "")
	depth := request.GetInt("depth", 0)

	results, err := ghxlib.Tree(parsedRepo, path, ghxlib.TreeOpts{Depth: depth})
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	data, _ := json.Marshal(results)
	return mcp.NewToolResultText(string(data)), nil
}

func buildRegistry() *codemode.Registry {
	reg := codemode.NewRegistry()
	ghxlib.RegisterTools(reg)
	return reg
}

func handleSearchTools(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query := request.GetString("query", "")

	reg := buildRegistry()
	var tools []codemode.Tool
	if query == "" {
		tools = reg.List()
	} else {
		tools = reg.Search(query)
	}

	stubs := codemode.GenerateTypes(tools)

	result := map[string]interface{}{
		"tools":     tools,
		"typeStubs": stubs,
		"usage":     "Pass code to the code tool. Invoke tools as codemode.<tool>(args).",
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func handleCode(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	code, err := request.RequireString("code")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	reg := buildRegistry()
	executor := codemode.NewExecutor()
	result, err := executor.Execute(ctx, code, reg.Tools())
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	const maxChars = 24000
	if len(result.Value) > maxChars {
		result.Value = result.Value[:maxChars] + fmt.Sprintf("\n\n[truncated — result was %d chars, showing first %d]", len(result.Value), maxChars)
	}

	output := result.Value
	if len(result.Console) > 0 {
		output = fmt.Sprintf("Console:\n%s\n\nResult:\n%s", strings.Join(result.Console, "\n"), result.Value)
	}
	return mcp.NewToolResultText(output), nil
}

func init() {
	serveCmd.Flags().String("http", "", "Start streamable HTTP server on address (e.g., :8080)")
	// ADR-0019.3 D1: recon-first is the default; --direct restores the seven
	// direct ghx tools plus code. --recon stays accepted as a deprecated
	// no-op synonym of the default through one release cycle.
	serveCmd.Flags().Bool("direct", false, "Serve the seven direct ghx tools plus code instead of the default single recon tool")
	serveCmd.Flags().Bool("recon", false, "Deprecated: no-op synonym of the default (recon-first) mode")
	serveCmd.Flags().Bool("print-mcp-config", false, "Print a ready-to-paste mcpServers JSON block and exit")
	RootCmd.AddCommand(serveCmd)
}
