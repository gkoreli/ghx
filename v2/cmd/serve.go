package cmd

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	ghxlib "github.com/gkoreli/ghx/v2/pkg/ghx"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server (stdio)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return serveMCP()
	},
}

func serveMCP() error {
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

	// Register tools with handlers
	s.AddTool(exploreTool, handleExplore)
	s.AddTool(reposTool, handleRepos)
	s.AddTool(searchTool, handleSearch)
	s.AddTool(readTool, handleRead)
	s.AddTool(treeTool, handleTree)

	return server.ServeStdio(s)
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

func init() {
	RootCmd.AddCommand(serveCmd)
}
