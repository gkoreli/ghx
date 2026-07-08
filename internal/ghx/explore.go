package ghx

import (
	"fmt"
)

type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "blob" or "tree"
}

type ExploreResult struct {
	Description string      `json:"description"`
	Branch      string      `json:"branch"`
	Snapshot    Snapshot    `json:"snapshot"`
	Files       []FileEntry `json:"files"`
	Readme      string      `json:"readme"` // full README text, empty if not found
}

// Explore returns branch, tree, and README for a repo (or subdirectory listing).
// If path is empty, returns root tree + README. If path is set, returns subdirectory entries only.
func Explore(repo Repo, path string) (*ExploreResult, error) {
	owner := repo.Owner
	name := repo.Name

	gql, err := githubClients.GraphQL()
	if err != nil {
		return nil, upstream(fmt.Errorf("failed to create GraphQL client: %w", err))
	}

	if path == "" {
		query := fmt.Sprintf(`{
			repository(owner: %q, name: %q) {
				defaultBranchRef { name target { oid } }
				description
				tree: object(expression: "HEAD:") {
					... on Tree { entries { name type } }
				}
				readme1: object(expression: "HEAD:README.md") {
					... on Blob { text }
				}
				readme2: object(expression: "HEAD:readme.md") {
					... on Blob { text }
				}
				readme3: object(expression: "HEAD:Readme.md") {
					... on Blob { text }
				}
			}
		}`, owner, name)

		var resp struct {
			Repository struct {
				DefaultBranchRef struct {
					Name   string `json:"name"`
					Target struct {
						OID string `json:"oid"`
					} `json:"target"`
				} `json:"defaultBranchRef"`
				Description string `json:"description"`
				Tree        struct {
					Entries []struct {
						Name string `json:"name"`
						Type string `json:"type"`
					} `json:"entries"`
				} `json:"tree"`
				Readme1 struct {
					Text string `json:"text"`
				} `json:"readme1"`
				Readme2 struct {
					Text string `json:"text"`
				} `json:"readme2"`
				Readme3 struct {
					Text string `json:"text"`
				} `json:"readme3"`
			} `json:"repository"`
		}

		if err := gql.Do(query, nil, &resp); err != nil {
			return nil, upstream(fmt.Errorf("GraphQL query failed: %w", err))
		}

		files := make([]FileEntry, len(resp.Repository.Tree.Entries))
		for i, e := range resp.Repository.Tree.Entries {
			files[i] = FileEntry{Name: e.Name, Type: e.Type}
		}

		// Pick first non-empty README variant
		readme := resp.Repository.Readme1.Text
		if readme == "" {
			readme = resp.Repository.Readme2.Text
		}
		if readme == "" {
			readme = resp.Repository.Readme3.Text
		}

		return &ExploreResult{
			Description: resp.Repository.Description,
			Branch:      resp.Repository.DefaultBranchRef.Name,
			Snapshot:    Snapshot{Repo: repo, SHA: resp.Repository.DefaultBranchRef.Target.OID},
			Files:       files,
			Readme:      readme,
		}, nil
	}

	query := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			defaultBranchRef { name target { oid } }
			tree: object(expression: "HEAD:%s") {
				... on Tree { entries { name type } }
			}
		}
	}`, owner, name, path)

	var resp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name   string `json:"name"`
				Target struct {
					OID string `json:"oid"`
				} `json:"target"`
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
		return nil, upstream(fmt.Errorf("GraphQL query failed: %w", err))
	}

	files := make([]FileEntry, len(resp.Repository.Tree.Entries))
	for i, e := range resp.Repository.Tree.Entries {
		files[i] = FileEntry{Name: e.Name, Type: e.Type}
	}

	return &ExploreResult{
		Branch:   resp.Repository.DefaultBranchRef.Name,
		Snapshot: Snapshot{Repo: repo, SHA: resp.Repository.DefaultBranchRef.Target.OID},
		Files:    files,
	}, nil
}
