package ghx

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

type GrepMatch struct {
	LineNum int
	Line    string
	IsMatch bool // true for the matching line, false for context lines
}

type FileResult struct {
	Path     string
	Content  string       // full text (empty if using grep/map)
	ByteSize int
	NotFound bool
	GrepHits []GrepMatch  // populated when Grep is set
	MapLines []string     // populated when Map is true
	MapChars int          // original char count (for reduction stats)
}

type ReadOpts struct {
	Grep  string // filter to matching lines with 2 lines context
	Lines string // line range "N-M"
	Map   bool   // structural signatures only
}

// Read fetches 1-10 files from a GitHub repo in one API call.
// Returns one FileResult per requested file.
func Read(repo string, files []string, opts ReadOpts) ([]FileResult, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo format, use owner/repo")
	}
	owner := parts[0]
	name := parts[1]

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	var results []FileResult

	// Process each file individually
	for _, f := range files {
		fileQuery := fmt.Sprintf(`{
			repository(owner: %q, name: %q) {
				object(expression: "HEAD:%s") {
					... on Blob { text byteSize }
				}
			}
		}`, owner, name, url.QueryEscape(f))

		var fileData struct {
			Repository struct {
				Object struct {
					Text     string `json:"text"`
					ByteSize int    `json:"byteSize"`
				} `json:"object"`
			} `json:"repository"`
		}

		if err := gql.Do(fileQuery, nil, &fileData); err != nil {
			results = append(results, FileResult{
				Path:     f,
				NotFound: true,
			})
			continue
		}

		text := fileData.Repository.Object.Text
		byteSize := fileData.Repository.Object.ByteSize

		if text == "" {
			results = append(results, FileResult{
				Path:     f,
				NotFound: true,
			})
			continue
		}

		result := FileResult{
			Path:     f,
			ByteSize: byteSize,
		}

		if opts.Grep != "" {
			result.GrepHits = grepLines(text, opts.Grep)
		} else if opts.Lines != "" {
			result.Content = extractLines(text, opts.Lines)
		} else if opts.Map {
			ext := getFileExtension(f)
			pat := getMapPattern(ext)
			result.MapLines = mapLines(text, pat)
			result.MapChars = len(text)
		} else {
			result.Content = text
		}

		results = append(results, result)
	}

	return results, nil
}

func grepLines(text string, pattern string) []GrepMatch {
	lines := strings.Split(text, "\n")
	var matches []GrepMatch
	patternLower := strings.ToLower(pattern)

	for j, line := range lines {
		if strings.Contains(strings.ToLower(line), patternLower) {
			start := j - 2
			if start < 0 {
				start = 0
			}
			end := j + 3
			if end > len(lines) {
				end = len(lines)
			}

			for k := start; k < end; k++ {
				matches = append(matches, GrepMatch{
					LineNum: k + 1,
					Line:    lines[k],
					IsMatch: k == j,
				})
			}
		}
	}

	return matches
}

func extractLines(text string, lineRange string) string {
	parts := strings.Split(lineRange, "-")
	if len(parts) != 2 {
		return ""
	}

	start := parseInt(parts[0]) - 1
	end := parseInt(parts[1])

	lines := strings.Split(text, "\n")
	if start >= len(lines) {
		return ""
	}

	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func mapLines(text string, pattern string) []string {
	var result []string
	fullLines := strings.Split(text, "\n")

	for _, line := range fullLines {
		if match, _ := regexp.MatchString(pattern, line); match {
			result = append(result, line)
		}
	}

	return result
}

func getFileExtension(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

func getMapPattern(ext string) string {
	patterns := map[string]string{
		"ts":   `^(import |export |const |let |var |function |class |interface |type |enum )`,
		"tsx":  `^(import |export |const |let |var |function |class |interface |type |enum )`,
		"js":   `^(import |export |const |let |var |function |class |interface |type |enum )`,
		"jsx":  `^(import |export |const |let |var |function |class |interface |type |enum )`,
		"py":   `^(import |from |class |def |    def |        def |@)`,
		"go":   `^(package |import |func |type |var |const )`,
		"rs":   `^(use |pub |fn |struct |enum |trait |impl |type |mod |const )`,
		"java": `^(import |public |private |protected |class |interface |enum |@)`,
		"kt":   `^(import |public |private |protected |class |interface |enum |@)`,
		"rb":   `^(require |class |module |def |  def |    def )`,
	}
	if p, ok := patterns[ext]; ok {
		return p
	}
	return `^(import |export |function |class |def |func |type |const |pub )`
}

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
