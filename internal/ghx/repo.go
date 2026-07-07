package ghx

import (
	"fmt"
	"strings"
)

// Repo identifies a GitHub repository by owner and repository name.
type Repo struct {
	// Owner is the GitHub account or organization segment of owner/repo.
	Owner string
	// Name is the repository name segment of owner/repo.
	Name string
}

// ParseRepo parses and validates a GitHub owner/repo slug.
func ParseRepo(s string) (Repo, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repo{}, fmt.Errorf("invalid repo %q: expected owner/repo, e.g. ghx explore gkoreli/ghx", s)
	}
	return Repo{Owner: parts[0], Name: parts[1]}, nil
}

// String returns the canonical owner/repo slug.
func (r Repo) String() string {
	return r.Owner + "/" + r.Name
}
