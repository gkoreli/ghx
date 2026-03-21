package ghx

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// RepoResult holds one search result
type RepoResult struct {
	NameWithOwner string
	Description   string
	Stars         int
	Language      string
	ReadmePreview string // cleaned, truncated to 300 chars
}

type ReposOpts struct {
	Limit int // default 10, max 20
}

// Repos searches GitHub repositories with README preview.
// Returns results and total count. Empty results returns nil, 0, nil.
func Repos(query string, opts ReposOpts) ([]RepoResult, int, error) {
	limit := opts.Limit
	if limit == 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	graphqlQuery := fmt.Sprintf(`
	{
		search(query: %q, type: REPOSITORY, first: %d) {
			repositoryCount
			nodes {
				... on Repository {
					nameWithOwner
					description
					stargazerCount
					primaryLanguage { name }
					object(expression: "HEAD:README.md") {
						... on Blob { text }
					}
				}
			}
		}
	}`, query, limit)

	var resp struct {
		Search struct {
			RepositoryCount int `json:"repositoryCount"`
			Nodes           []struct {
				NameWithOwner   string `json:"nameWithOwner"`
				Description     string `json:"description"`
				StargazerCount  int    `json:"stargazerCount"`
				PrimaryLanguage *struct {
					Name string `json:"name"`
				} `json:"primaryLanguage"`
				Object struct {
					Text string `json:"text"`
				} `json:"object"`
			} `json:"nodes"`
		} `json:"search"`
	}

	if err := gql.Do(graphqlQuery, nil, &resp); err != nil {
		return nil, 0, fmt.Errorf("graphql query failed: %w", err)
	}

	if len(resp.Search.Nodes) == 0 {
		return nil, resp.Search.RepositoryCount, nil
	}

	results := make([]RepoResult, len(resp.Search.Nodes))
	for i, repo := range resp.Search.Nodes {
		lang := "?"
		if repo.PrimaryLanguage != nil {
			lang = repo.PrimaryLanguage.Name
		}

		results[i] = RepoResult{
			NameWithOwner: repo.NameWithOwner,
			Description:   repo.Description,
			Stars:         repo.StargazerCount,
			Language:      lang,
			ReadmePreview: cleanReadme(repo.Object.Text),
		}
	}

	return results, resp.Search.RepositoryCount, nil
}

func cleanReadme(text string) string {
	if text == "" {
		return ""
	}
	// Collapse newlines and whitespace
	text = strings.ReplaceAll(text, "\n", " ")
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Remove markdown image links: ![alt](url)
	text = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`).ReplaceAllString(text, "")
	// Remove badge links: [![alt](img)](url)
	text = regexp.MustCompile(`\[!\[([^\]]*)\]\([^)]*\)\]\([^)]*\)`).ReplaceAllString(text, "")
	// Remove badge alt part: [![alt](img)]
	text = regexp.MustCompile(`\[!\[[^\]]*\]\([^)]*\)\]`).ReplaceAllString(text, "")
	// Remove link syntax: [text](url)
	text = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`).ReplaceAllString(text, "")

	// Catch-all: strip any remaining HTML tags
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, "")

	// Collapse whitespace again and trim
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Truncate to 300 chars
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}
