package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/gkoreli/ghx/v2/pkg/codemode"
	ghxlib "github.com/gkoreli/ghx/v2/pkg/ghx"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveMCP(cmd)
	},
}

func serveMCP(cmd *cobra.Command) error {
	s := server.NewMCPServer("ghx", VERSION,
		server.WithToolCapabilities(true),
	)

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
	)

	treeTool := mcp.NewTool("tree",
		mcp.WithDescription("Full recursive file tree listing for a repo"),
		mcp.WithString("repo", mcp.Required(), mcp.Description("owner/repo")),
		mcp.WithString("path", mcp.Description("subdirectory path")),
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

Write an async arrow function in JavaScript that returns the result.
Do NOT use TypeScript syntax — no type annotations, interfaces, or generics.

Example: async () => { const r = await codemode.explore({ repo: "vercel/next.js" }); return r.files; }`, stubs)

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

	// Select and use transport
	transport := selectTransport(cmd)
	return transport.Serve(s)
}

func handleExplore(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	repo, err := request.RequireString("repo")
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	path := request.GetString("path", "")

	result, err := ghxlib.Explore(repo, path)
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

	results, err := ghxlib.Read(repo, paths, ghxlib.ReadOpts{
		Grep:  grepPattern,
		Lines: lineRange,
		Map:   mapMode,
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

	path := request.GetString("path", "")

	results, err := ghxlib.Tree(repo, path, ghxlib.TreeOpts{})
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
		"usage":     "Pass code to code tool. Use callTool(name, args) to invoke tools.",
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

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

func init() {
	serveCmd.Flags().String("http", "", "Start streamable HTTP server on address (e.g., :8080)")
	RootCmd.AddCommand(serveCmd)
}
