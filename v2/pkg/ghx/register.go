package ghx

import "github.com/gkoreli/ghx/v2/pkg/codemode"

// RegisterTools registers all ghx core functions with the given codemode registry.
func RegisterTools(r *codemode.Registry) {
	r.Register(codemode.Tool{
		Name:        "explore",
		Description: "Explore a GitHub repo — returns branch, file tree, and README",
		Func:        wrapExplore,
		Returns:     "{ description: string; branch: string; files: { name: string; type: string }[]; readme: string }",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{"type": "string", "description": "owner/repo"},
				"path": map[string]any{"type": "string", "description": "subdirectory path (optional)"},
			},
			"required": []string{"repo"},
		},
	})

	r.Register(codemode.Tool{
		Name:        "repos",
		Description: "Search GitHub repositories with README preview",
		Func:        wrapRepos,
		Returns:     "{ results: { nameWithOwner: string; description: string; stars: number; language: string; readmePreview: string }[]; total: number }",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "search query"},
				"limit": map[string]any{"type": "number", "description": "max results (default 10, max 20)"},
			},
			"required": []string{"query"},
		},
	})

	r.Register(codemode.Tool{
		Name:        "search",
		Description: "Search code via GitHub with text_matches and context",
		Func:        wrapSearch,
		Returns:     "{ total: number; incomplete: boolean; matches: { repo: string; path: string; fragment: string }[] }",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":     map[string]any{"type": "string", "description": "search query"},
				"limit":     map[string]any{"type": "number", "description": "max results (default 30, max 100)"},
				"fullMode": map[string]any{"type": "boolean", "description": "disable 200-char truncation"},
			},
			"required": []string{"query"},
		},
	})

	r.Register(codemode.Tool{
		Name:        "read",
		Description: "Read multiple files from a GitHub repo in one call",
		Func:        wrapRead,
		Returns:     "{ path: string; content: string; byteSize: number; notFound: boolean; globPattern?: string; grepHits?: { lineNum: number; line: string; isMatch: boolean }[]; mapLines?: string[]; mapChars?: number }[]",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":  map[string]any{"type": "string", "description": "owner/repo"},
				"files": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "file paths or glob patterns (e.g. src/**/*.ts)"},
				"grep":  map[string]any{"type": "string", "description": "filter to matching lines with context"},
				"map":   map[string]any{"type": "boolean", "description": "extract code structure only"},
			},
			"required": []string{"repo", "files"},
		},
	})

	r.Register(codemode.Tool{
		Name:        "tree",
		Description: "Get full recursive file listing for a repo",
		Func:        wrapTree,
		Returns:     "string[]",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo":  map[string]any{"type": "string", "description": "owner/repo"},
				"path":  map[string]any{"type": "string", "description": "path prefix filter (optional)"},
				"depth": map[string]any{"type": "number", "description": "limit tree depth (0 = full recursive)"},
			},
			"required": []string{"repo"},
		},
	})
}

func wrapExplore(args map[string]any) (any, error) {
	repo, _ := args["repo"].(string)
	path, _ := args["path"].(string)
	return Explore(repo, path)
}

func wrapRepos(args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	results, total, err := Repos(query, ReposOpts{Limit: limit})
	if err != nil {
		return nil, err
	}
	return map[string]any{"results": results, "total": total}, nil
}

func wrapSearch(args map[string]any) (any, error) {
	query, _ := args["query"].(string)
	limit := 30
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}
	fullMode := false
	if fm, ok := args["fullMode"].(bool); ok {
		fullMode = fm
	}
	return Search(query, SearchOpts{Limit: limit, FullMode: fullMode})
}

func wrapRead(args map[string]any) (any, error) {
	repo, _ := args["repo"].(string)
	files := []string{}
	if f, ok := args["files"].([]any); ok {
		for _, file := range f {
			if s, ok := file.(string); ok {
				files = append(files, s)
			}
		}
	}
	grep, _ := args["grep"].(string)
	mapMode := false
	if m, ok := args["map"].(bool); ok {
		mapMode = m
	}
	return Read(repo, files, &ReadOpts{Grep: grep, Map: mapMode})
}

func wrapTree(args map[string]any) (any, error) {
	repo, _ := args["repo"].(string)
	path, _ := args["path"].(string)
	depth := 0
	if d, ok := args["depth"].(float64); ok {
		depth = int(d)
	}
	return Tree(repo, path, TreeOpts{Depth: depth})
}
