package ghx

import (
	"fmt"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// treeEntry represents a single entry from the Git Trees API.
type treeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"` // "blob" or "tree"
}

// fetchTree returns the full recursive tree for a repo via the Git Trees API (one REST call).
func fetchTree(repo Repo) ([]treeEntry, error) {
	gql, err := githubClients.GraphQL()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	branchQuery := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			defaultBranchRef { name }
		}
	}`, repo.Owner, repo.Name)

	var branchResp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
		} `json:"repository"`
	}

	if err := gql.Do(branchQuery, nil, &branchResp); err != nil {
		return nil, fmt.Errorf("failed to get branch: %w", err)
	}

	branch := branchResp.Repository.DefaultBranchRef.Name
	if branch == "" {
		return nil, fmt.Errorf("could not determine default branch")
	}

	rest, err := githubClients.REST()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	endpoint := fmt.Sprintf("repos/%s/%s/git/trees/%s?recursive=1", repo.Owner, repo.Name, branch)
	var resp struct {
		Tree []treeEntry `json:"tree"`
	}

	if err := rest.Get(endpoint, &resp); err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	return resp.Tree, nil
}

// isGlob returns true if the path contains glob metacharacters.
func isGlob(path string) bool {
	return strings.ContainsAny(path, "*?[{")
}

// GlobResult holds the expansion of a single glob pattern.
type GlobResult struct {
	Pattern   string   // original glob
	Matches   []string // matched file paths
	Truncated bool     // true if matches exceeded maxFiles
}

// expandGlobs resolves glob patterns against a repo's tree.
// Exact paths pass through unchanged. Returns deduplicated paths (max maxFiles) and per-glob info.
func expandGlobs(patterns []string, tree []treeEntry, maxFiles int) (files []string, globs []GlobResult) {
	seen := make(map[string]bool)
	var addFile = func(f string) {
		if !seen[f] && len(files) < maxFiles {
			seen[f] = true
			files = append(files, f)
		}
	}

	// Collect blob paths once for matching
	var blobPaths []string
	for _, e := range tree {
		if e.Type != "tree" {
			blobPaths = append(blobPaths, e.Path)
		}
	}

	for _, p := range patterns {
		if !isGlob(p) {
			addFile(p)
			continue
		}

		gr := GlobResult{Pattern: p}
		for _, bp := range blobPaths {
			matched, _ := doublestar.Match(p, bp)
			if matched {
				gr.Matches = append(gr.Matches, bp)
				addFile(bp)
			}
		}
		if len(gr.Matches) > maxFiles {
			gr.Truncated = true
		}
		globs = append(globs, gr)
	}

	return files, globs
}
