package ghx

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

type SearchMatch struct {
	Repo     string `json:"repo"` // "owner/repo"
	Path     string `json:"path"`
	Fragment string `json:"fragment"` // matching line context
}

type SearchResult struct {
	Total      int           `json:"total"`
	Incomplete bool          `json:"incomplete"`
	Matches    []SearchMatch `json:"matches"`
	Truncated  bool          `json:"truncated"`
}

type SearchOpts struct {
	Limit    int  // default 30, max 100
	FullMode bool // disable 200-char truncation
	Budget   int  // approximate output budget in characters
}

// Search searches code via GitHub REST API with text_matches.
func Search(query string, opts SearchOpts) (*SearchResult, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}

	rest, err := api.NewRESTClient(api.ClientOptions{
		Headers: map[string]string{"Accept": "application/vnd.github.text-match+json"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	var resp struct {
		TotalCount        int  `json:"total_count"`
		IncompleteResults bool `json:"incomplete_results"`
		Items             []struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
			Path        string `json:"path"`
			TextMatches []struct {
				Fragment string `json:"fragment"`
			} `json:"text_matches"`
		} `json:"items"`
	}

	path := fmt.Sprintf("search/code?q=%s&per_page=%d", url.QueryEscape(query), limit)
	if err := rest.Get(path, &resp); err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	matches := make([]SearchMatch, 0, len(resp.Items))
	usedBudget := 0
	truncated := false
	for _, item := range resp.Items {
		fragment := ""
		if len(item.TextMatches) > 0 {
			fragment = item.TextMatches[0].Fragment
			fragment = strings.ReplaceAll(fragment, "\n", " ")
			fragment = strings.Join(strings.Fields(fragment), " ")

			if !opts.FullMode && len(fragment) > 200 {
				fragment = fragment[:200] + "…"
			}
		}

		match := SearchMatch{
			Repo:     item.Repository.FullName,
			Path:     item.Path,
			Fragment: fragment,
		}
		if opts.Budget > 0 && !opts.FullMode {
			nextSize := len(match.Repo) + len(match.Path) + len(match.Fragment) + 4
			if usedBudget > 0 && usedBudget+nextSize > opts.Budget {
				truncated = true
				break
			}
			usedBudget += nextSize
		}
		matches = append(matches, match)
	}

	return &SearchResult{
		Total:      resp.TotalCount,
		Incomplete: resp.IncompleteResults,
		Matches:    matches,
		Truncated:  truncated || len(matches) < len(resp.Items),
	}, nil
}
