package ghx

import (
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "blob" or "tree"
}

type ExploreResult struct {
	Description string      `json:"description"`
	Branch      string      `json:"branch"`
	Files       []FileEntry `json:"files"`
	Readme      string      `json:"readme"` // full README text, empty if not found
}

// Explore returns branch, tree, and README for a repo (or subdirectory listing).
// If path is empty, returns root tree + README. If path is set, returns subdirectory entries only.
func Explore(repo string, path string) (*ExploreResult, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, expected owner/name")
	}
	owner := parts[0]
	name := parts[1]

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	if path == "" {
		query := fmt.Sprintf(`{
			repository(owner: %q, name: %q) {
				defaultBranchRef { name }
				description
				tree: object(expression: "HEAD:") {
					... on Tree { entries { name type } }
				}
				readme: object(expression: "HEAD:README.md") {
					... on Blob { text }
				}
			}
		}`, owner, name)

		var resp struct {
			Repository struct {
				DefaultBranchRef struct {
					Name string `json:"name"`
				} `json:"defaultBranchRef"`
				Description string `json:"description"`
				Tree        struct {
					Entries []struct {
						Name string `json:"name"`
						Type string `json:"type"`
					} `json:"entries"`
				} `json:"tree"`
				Readme struct {
					Text string `json:"text"`
				} `json:"readme"`
			} `json:"repository"`
		}

		if err := gql.Do(query, nil, &resp); err != nil {
			return nil, fmt.Errorf("GraphQL query failed: %w", err)
		}

		files := make([]FileEntry, len(resp.Repository.Tree.Entries))
		for i, e := range resp.Repository.Tree.Entries {
			files[i] = FileEntry{Name: e.Name, Type: e.Type}
		}

		return &ExploreResult{
			Description: resp.Repository.Description,
			Branch:      resp.Repository.DefaultBranchRef.Name,
			Files:       files,
			Readme:      resp.Repository.Readme.Text,
		}, nil
	}

	query := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			defaultBranchRef { name }
			tree: object(expression: "HEAD:%s") {
				... on Tree { entries { name type } }
			}
		}
	}`, owner, name, path)

	var resp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
			Tree struct {
				Entries []struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"entries"`
			} `json:"tree"`
		} `json:"repository"`
	}

	if err := gql.Do(query, nil, &resp); err != nil {
		return nil, fmt.Errorf("GraphQL query failed: %w", err)
	}

	files := make([]FileEntry, len(resp.Repository.Tree.Entries))
	for i, e := range resp.Repository.Tree.Entries {
		files[i] = FileEntry{Name: e.Name, Type: e.Type}
	}

	return &ExploreResult{
		Branch: resp.Repository.DefaultBranchRef.Name,
		Files:  files,
	}, nil
}
