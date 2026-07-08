package ghx

import (
	"fmt"
	"strings"
)

// Repo identifies a GitHub repository by owner and repository name.
type Repo struct {
	// Owner is the GitHub account or organization segment of owner/repo.
	Owner string `json:"owner"`
	// Name is the repository name segment of owner/repo.
	Name string `json:"name"`
}

// Snapshot identifies the repository commit observed by a read operation.
type Snapshot struct {
	// Repo is the GitHub repository whose default branch was resolved.
	Repo Repo `json:"repo"`
	// SHA is the commit oid reported by GitHub for the resolved branch target.
	SHA string `json:"sha,omitempty"`
}

// ParseRepo parses and validates a GitHub owner/repo slug.
func ParseRepo(s string) (Repo, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, badInput(fmt.Errorf("invalid repo %q: expected owner/repo, e.g. ghx explore gkoreli/ghx", s))
	}
	return Repo{Owner: parts[0], Name: parts[1]}, nil
}

// String returns the canonical owner/repo slug.
func (r Repo) String() string {
	return r.Owner + "/" + r.Name
}
