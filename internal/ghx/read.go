package ghx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/gkoreli/ghx/v2/internal/mapengine"
)

type GrepMatch struct {
	LineNum int    `json:"lineNum"`
	Line    string `json:"line"`
	IsMatch bool   `json:"isMatch"` // true for the matching line, false for context lines
}

type FileResult struct {
	Path        string      `json:"path"`
	Content     string      `json:"content"` // full text (empty if using grep/map)
	ByteSize    int         `json:"byteSize"`
	NotFound    bool        `json:"notFound"`
	DirEntries  []FileEntry `json:"dirEntries,omitempty"`  // populated when path is a directory
	GrepHits    []GrepMatch `json:"grepHits,omitempty"`    // populated when Grep is set
	MapLines    []string    `json:"mapLines,omitempty"`    // populated when Map is true
	MapChars    int         `json:"mapChars,omitempty"`    // original char count (for reduction stats)
	MapEngine   string      `json:"mapEngine,omitempty"`   // mapper engine used when Map is true
	MapWarnings []string    `json:"mapWarnings,omitempty"` // fallback/quality warnings from mapper
	GlobPattern string      `json:"globPattern,omitempty"` // the glob that matched this file (empty for exact paths)
	BudgetedMap bool        `json:"budgetedMap,omitempty"` // true when content exceeded Budget and was replaced by a structural map
}

type ReadOpts struct {
	Grep      string // filter to matching lines with 2 lines context
	Lines     string // line range "N-M"
	Map       bool   // structural signatures only
	MapLevel  string // map detail level: outline|minimal|compact|standard
	MapKind   string // map symbol kind filter: func|type|import|const|var
	MapEngine string // map engine: auto|regex|tree-sitter
	Budget    int    // output budget in characters before structural fallback
	FullMode  bool   // disable budget fallback and return complete content

	// Output: populated after Read() when globs are used
	Globs []GlobResult
}

// Read fetches 1-10 files from a GitHub repo in one API call using GraphQL aliases.
// Returns one FileResult per requested file.
func Read(repo string, files []string, opts *ReadOpts) ([]FileResult, error) {
	if opts == nil {
		opts = &ReadOpts{}
	}

	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo %q: expected owner/repo, e.g. ghx read gkoreli/ghx cmd/ghx/main.go --lines 1-40", repo)
	}
	owner := parts[0]
	name := parts[1]

	// Expand globs: detect glob patterns, fetch tree, resolve to exact paths
	hasGlobs := false
	for _, f := range files {
		if isGlob(f) {
			hasGlobs = true
			break
		}
	}

	// Track which glob each file came from (empty string for exact paths)
	globOrigin := make(map[string]string)

	if hasGlobs {
		tree, err := fetchTree(repo)
		if err != nil {
			return nil, fmt.Errorf("glob expansion failed: %w", err)
		}
		expanded, globs := expandGlobs(files, tree, 10)
		opts.Globs = globs
		for _, gr := range globs {
			for _, m := range gr.Matches {
				globOrigin[m] = gr.Pattern
			}
		}
		files = expanded
	}

	if len(files) == 0 {
		return nil, nil
	}

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	// Build batched query with aliases (max 10 files per call)
	var aliases []string
	for i, f := range files {
		if i >= 10 {
			break
		}
		alias := fmt.Sprintf("f%d", i)
		escapedPath := f
		aliases = append(aliases, fmt.Sprintf(`%s: object(expression: "HEAD:%s") { ... on Blob { text byteSize } ... on Tree { entries { name type } } }`, alias, escapedPath))
	}

	query := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			%s
		}
	}`, owner, name, strings.Join(aliases, "\n"))

	var resp struct {
		Repository map[string]interface{} `json:"repository"`
	}

	if err := gql.Do(query, nil, &resp); err != nil {
		// Return NotFound for all files on error
		var results []FileResult
		for _, f := range files {
			results = append(results, FileResult{Path: f, NotFound: true})
		}
		return results, nil
	}

	var results []FileResult
	for i, f := range files {
		if i >= 10 {
			break
		}

		alias := fmt.Sprintf("f%d", i)
		fileData, ok := resp.Repository[alias]
		if !ok || fileData == nil {
			results = append(results, FileResult{Path: f, NotFound: true})
			continue
		}

		result := parseFileResponse(f, fileData, globOrigin[f], opts)
		results = append(results, result)
	}

	return results, nil
}

// parseFileResponse converts a raw GraphQL response for a single file/directory into a FileResult.
func parseFileResponse(path string, data interface{}, globPattern string, opts *ReadOpts) FileResult {
	m, ok := data.(map[string]interface{})
	if !ok {
		return FileResult{Path: path, NotFound: true}
	}

	// Directory (Tree)
	if entries, ok := m["entries"].([]interface{}); ok {
		dirEntries := make([]FileEntry, 0, len(entries))
		for _, e := range entries {
			if em, ok := e.(map[string]interface{}); ok {
				name, _ := em["name"].(string)
				typ, _ := em["type"].(string)
				dirEntries = append(dirEntries, FileEntry{Name: name, Type: typ})
			}
		}
		return FileResult{Path: path, DirEntries: dirEntries}
	}

	// File (Blob)
	text, _ := m["text"].(string)
	byteSize := 0
	if bs, ok := m["byteSize"].(float64); ok {
		byteSize = int(bs)
	}

	if text == "" {
		return FileResult{Path: path, NotFound: true}
	}

	result := FileResult{
		Path:        path,
		ByteSize:    byteSize,
		GlobPattern: globPattern,
	}

	if opts.Grep != "" {
		result.GrepHits = grepLines(text, opts.Grep)
	} else if opts.Lines != "" {
		result.Content = extractLines(text, opts.Lines)
	} else if opts.Map {
		applyMapResult(&result, path, text, opts)
	} else if opts.Budget > 0 && !opts.FullMode && len(text) > opts.Budget {
		applyMapResult(&result, path, text, opts)
		result.BudgetedMap = true
	} else {
		result.Content = text
	}

	return result
}

func applyMapResult(result *FileResult, path string, text string, opts *ReadOpts) {
	mapResult, err := mapengine.Map(path, []byte(text), mapengine.Options{
		Engine: mapengine.Engine(opts.MapEngine),
		Level:  mapengine.Level(opts.MapLevel),
		Kind:   mapengine.Kind(opts.MapKind),
	})
	result.MapLines = mapResult.Lines
	result.MapChars = mapResult.OriginalChars
	result.MapEngine = string(mapResult.Engine)
	result.MapWarnings = mapResult.Warnings
	if err != nil {
		result.MapWarnings = append(result.MapWarnings, err.Error())
	}
	if len(result.MapLines) == 0 {
		result.MapLines = []string{"(no signatures detected)"}
	}
}

// normalizeBRE converts BRE-style escaped metacharacters to their ERE equivalents.
// In BRE (grep default), \| means alternation; in ERE/RE2, bare | does.
// Handles escaped backslashes: \\| (literal backslash + alternation) is left alone.
func normalizeBRE(pattern string) string {
	// BRE metacharacters that are special when escaped, but literal when bare.
	// In ERE/RE2 these are the opposite: special when bare, literal when escaped.
	breMetachars := map[byte]bool{'|': true, '(': true, ')': true, '{': true, '}': true, '+': true, '?': true}

	var out []byte
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '\\' && i+1 < len(pattern) {
			next := pattern[i+1]
			if next == '\\' {
				// Escaped backslash — emit both, skip ahead
				out = append(out, '\\', '\\')
				i++
			} else if breMetachars[next] {
				// BRE escape like \| → emit bare metachar (ERE style)
				out = append(out, next)
				i++
			} else {
				// Some other escape like \n, \d — pass through
				out = append(out, '\\', next)
				i++
			}
		} else {
			out = append(out, pattern[i])
		}
	}
	return string(out)
}

func grepLines(text string, pattern string) []GrepMatch {
	lines := strings.Split(text, "\n")
	var matches []GrepMatch
	emitted := make(map[int]bool)

	// Normalize BRE-style escapes to ERE (agents trained on grep write \| for alternation).
	// Only converts \x when the backslash isn't itself escaped (\\| stays as literal backslash + pipe).
	pattern = normalizeBRE(pattern)

	// Compile as case-insensitive regex; fall back to literal match on invalid pattern
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		re = regexp.MustCompile("(?i)" + regexp.QuoteMeta(pattern))
	}

	// First pass: find all matching line indices
	var matchIndices []int
	for j, line := range lines {
		if re.MatchString(line) {
			matchIndices = append(matchIndices, j)
		}
	}

	// Build set of matching line indices for O(1) lookup
	matchSet := make(map[int]bool, len(matchIndices))
	for _, j := range matchIndices {
		matchSet[j] = true
	}

	// Second pass: emit context windows, skipping already-emitted lines
	for _, j := range matchIndices {
		start := j - 2
		if start < 0 {
			start = 0
		}
		end := j + 3
		if end > len(lines) {
			end = len(lines)
		}

		// Add separator if there's a gap from previous output
		if len(matches) > 0 && !emitted[start] {
			matches = append(matches, GrepMatch{LineNum: -1, Line: "--", IsMatch: false})
		}

		for k := start; k < end; k++ {
			if emitted[k] {
				continue
			}
			emitted[k] = true
			matches = append(matches, GrepMatch{
				LineNum: k + 1,
				Line:    lines[k],
				IsMatch: matchSet[k],
			})
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

func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
