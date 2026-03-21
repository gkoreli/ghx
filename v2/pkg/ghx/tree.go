package ghx

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

type TreeOpts struct {
	// reserved for future options (depth, filter)
}

// Tree returns full recursive file listing for a repo, optionally filtered to a path prefix.
// Returns list of file paths (blobs only, no directories).
func Tree(repo string, path string, opts TreeOpts) ([]string, error) {
	owner := strings.Split(repo, "/")[0]
	name := strings.Split(repo, "/")[1]

	// Get default branch
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

	// Use REST client for tree API
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

	var files []string
	for _, entry := range resp.Tree {
		if entry.Type == "blob" {
			if path != "" && strings.HasPrefix(entry.Path, path+"/") {
				files = append(files, strings.TrimPrefix(entry.Path, path+"/"))
			} else if path == "" {
				files = append(files, entry.Path)
			}
		}
	}

	return files, nil
}
