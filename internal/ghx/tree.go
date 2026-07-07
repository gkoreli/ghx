package ghx

import (
	"strings"
)

type TreeOpts struct {
	Depth int // 0 = full recursive (default), N = limit to N levels
}

// Tree returns file listing for a repo, optionally filtered to a path prefix.
// Without Depth: returns blobs only (backward compatible).
// With Depth: returns both blobs and directories (dirs suffixed with "/"), filtered to N levels.
func Tree(repo string, path string, opts TreeOpts) ([]string, error) {
	parsedRepo, err := ParseRepo(repo)
	if err != nil {
		return nil, err
	}
	entries, err := fetchTree(parsedRepo)
	if err != nil {
		return nil, err
	}

	var results []string
	for _, entry := range entries {
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
		} else {
			results = append(results, rel)
		}
	}

	return results, nil
}
