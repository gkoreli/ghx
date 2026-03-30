package ghx

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

type TreeOpts struct {
	Depth int // 0 = full recursive (default), N = limit to N levels
}

// Tree returns file listing for a repo, optionally filtered to a path prefix.
// Without Depth: returns blobs only (backward compatible).
// With Depth: returns both blobs and directories (dirs suffixed with "/"), filtered to N levels.
func Tree(repo string, path string, opts TreeOpts) ([]string, error) {
	owner := strings.Split(repo, "/")[0]
	name := strings.Split(repo, "/")[1]

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	branchQuery := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			defaultBranchRef { name }
		}
	}`, owner, name)

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

	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	endpoint := fmt.Sprintf("repos/%s/%s/git/trees/%s?recursive=1", owner, name, branch)
	var resp struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}

	if err := rest.Get(endpoint, &resp); err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var results []string
	for _, entry := range resp.Tree {
		rel := entry.Path
		if path != "" {
			if !strings.HasPrefix(entry.Path, path+"/") {
				continue
			}
			rel = strings.TrimPrefix(entry.Path, path+"/")
		}

		depth := strings.Count(rel, "/") + 1
		if opts.Depth > 0 && depth > opts.Depth {
			continue
		}

		if entry.Type == "tree" {
			if opts.Depth > 0 {
				results = append(results, rel+"/")
			}
			// Without --depth, skip directories (backward compatible)
		} else {
			results = append(results, rel)
		}
	}

	return results, nil
}
