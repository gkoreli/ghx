package ghx

import (
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gkoreli/ghx/v2/internal/mapengine"
)

const (
	defaultInspectBudget       = 12000
	defaultInspectLimit        = 30
	maxInspectLimit            = 100
	maxInspectFetchedFiles     = 10
	defaultInspectSnippetLines = 2
	defaultInspectSnippetChars = 900
)

// InspectOptions configures one-shot concern inspection.
type InspectOptions struct {
	// Budget is the approximate total text output budget in characters.
	Budget int
	// Limit is the candidate search limit before ranking.
	Limit int
	// Lang adds a GitHub code-search language qualifier.
	Lang string
	// Glob adds a GitHub code-search path qualifier and candidate filter hint.
	Glob string
	// Path constrains candidates to paths containing this subtree/prefix.
	Path string
}

// InspectResult contains ranked evidence for a repository concern.
type InspectResult struct {
	// Repo is the inspected owner/repo.
	Repo string
	// Query is the concern query supplied by the caller.
	Query string
	// Budget is the requested approximate output budget in characters.
	Budget int
	// Used is populated by FormatInspectText with emitted character count.
	Used int
	// TotalMatches is the GitHub code-search total before candidate filtering.
	TotalMatches int
	// SearchIncomplete reports GitHub search timeout/incompleteness.
	SearchIncomplete bool
	// Candidates is the deduplicated search candidate list before final truncation.
	Candidates []InspectCandidate
	// Files is the ranked file evidence.
	Files []InspectFile
	// Truncation records omitted evidence and next-step hints.
	Truncation InspectTruncation
}

// InspectCandidate records a deduplicated search candidate and its search evidence.
type InspectCandidate struct {
	// Path is the repository file path.
	Path string
	// SearchRank is the one-based first search-result rank.
	SearchRank int
	// MatchCount is the number of search results seen for this path.
	MatchCount int
	// Fragments are bounded text-match fragments returned by GitHub search.
	Fragments []string
}

// InspectFile contains ranked structural and snippet evidence for one file.
type InspectFile struct {
	// Path is the repository file path.
	Path string
	// Score is the deterministic rank score.
	Score int
	// Reasons explains the score features that fired.
	Reasons []string
	// ByteSize is the fetched blob size in bytes.
	ByteSize int
	// MapLines are structural signatures from mapengine.
	MapLines []string
	// MapEngine is the mapengine backend used for this file.
	MapEngine string
	// MapWarnings records map fallback or parse warnings.
	MapWarnings []string
	// Snippets are bounded line-anchored excerpts.
	Snippets []InspectSnippet

	content string
}

// InspectSnippet is a bounded line-anchored evidence excerpt.
type InspectSnippet struct {
	// StartLine is the first one-based line in Text.
	StartLine int
	// EndLine is the last one-based line in Text.
	EndLine int
	// Text is the excerpt body.
	Text string
}

// InspectHint is a concrete command or narrowing suggestion.
type InspectHint struct {
	// Reason explains why the hint is present.
	Reason string
	// Command is the exact next command or flag guidance.
	Command string
}

// InspectTruncation records omitted evidence with concrete next steps.
type InspectTruncation struct {
	// OmittedFiles is the count of candidate or ranked files omitted from output.
	OmittedFiles int
	// OmittedSnippets is the count of snippets not emitted by the text presenter.
	OmittedSnippets int
	// Hints are concrete follow-up commands or narrowing flags.
	Hints []InspectHint
}

// InspectRanker owns deterministic file scoring for inspect results.
type InspectRanker struct {
	terms []string
}

// NewInspectRanker creates a ranker for query terms.
func NewInspectRanker(query string) InspectRanker {
	return InspectRanker{terms: queryTerms(query)}
}

// Rank scores files and returns them in descending deterministic rank order.
func (r InspectRanker) Rank(files []InspectFile, candidates []InspectCandidate, opts InspectOptions) []InspectFile {
	candidateByPath := make(map[string]InspectCandidate, len(candidates))
	for _, candidate := range candidates {
		candidateByPath[candidate.Path] = candidate
	}

	referenceCounts := candidateLocalReferenceCounts(files)
	for i := range files {
		candidate := candidateByPath[files[i].Path]
		score, reasons := r.score(files[i], candidate, opts, referenceCounts[files[i].Path])
		files[i].Score = score
		files[i].Reasons = reasons
	}

	sort.SliceStable(files, func(i, j int) bool {
		if files[i].Score != files[j].Score {
			return files[i].Score > files[j].Score
		}
		return files[i].Path < files[j].Path
	})
	return files
}

func (r InspectRanker) score(file InspectFile, candidate InspectCandidate, opts InspectOptions, referenceCount int) (int, []string) {
	score := 0
	var reasons []string

	if candidate.SearchRank > 0 {
		points := max(1, 80-candidate.SearchRank)
		score += points
		reasons = append(reasons, fmt.Sprintf("search-rank:%d", candidate.SearchRank))
	}
	if candidate.MatchCount > 1 {
		points := candidate.MatchCount * 8
		score += points
		reasons = append(reasons, fmt.Sprintf("matches:%d", candidate.MatchCount))
	}
	if count := termHits(file.Path, r.terms); count > 0 {
		score += count * 12
		reasons = append(reasons, "path-term")
	}
	if count := symbolTermHits(file.MapLines, r.terms); count > 0 {
		score += count * 10
		reasons = append(reasons, "symbol-term")
	}
	if opts.Lang != "" && languageMatches(file.Path, opts.Lang) {
		score += 5
		reasons = append(reasons, "lang")
	}
	if opts.Glob != "" {
		if matched, _ := doublestar.Match(opts.Glob, file.Path); matched {
			score += 5
			reasons = append(reasons, "glob")
		}
	}
	if opts.Path != "" && strings.Contains(strings.ToLower(file.Path), strings.ToLower(opts.Path)) {
		score += 5
		reasons = append(reasons, "path")
	}
	if referenceCount > 0 {
		score += referenceCount * 6
		reasons = append(reasons, fmt.Sprintf("candidate-local-refs:%d", referenceCount))
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "fetched-candidate")
	}
	return score, reasons
}

// BudgetWriter writes deterministic text within an approximate character budget.
type BudgetWriter struct {
	budget int
	used   int
	b      strings.Builder
}

// NewBudgetWriter creates a writer with a positive character budget.
func NewBudgetWriter(budget int) *BudgetWriter {
	if budget <= 0 {
		budget = defaultInspectBudget
	}
	return &BudgetWriter{budget: budget}
}

// Write appends text if it fits in the budget.
func (w *BudgetWriter) Write(text string) bool {
	if w.used+len(text) > w.budget {
		return false
	}
	w.b.WriteString(text)
	w.used += len(text)
	return true
}

// Writef formats and appends text if it fits in the budget.
func (w *BudgetWriter) Writef(format string, args ...any) bool {
	return w.Write(fmt.Sprintf(format, args...))
}

// String returns written text.
func (w *BudgetWriter) String() string {
	return w.b.String()
}

// Used returns the emitted character count.
func (w *BudgetWriter) Used() int {
	return w.used
}

// Inspect searches, fetches, maps, ranks, and snippets concern-local evidence.
func Inspect(repo string, query string, opts InspectOptions) (*InspectResult, error) {
	if !validRepo(repo) {
		return nil, fmt.Errorf("invalid repo %q: expected owner/repo, e.g. ghx inspect gkoreli/ghx \"exit code\"", repo)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query must not be empty, e.g. ghx inspect gkoreli/ghx \"exit code\"")
	}
	opts = normalizeInspectOptions(opts)

	searchQuery := buildInspectSearchQuery(repo, query, opts)
	searchResult, err := Search(searchQuery, SearchOpts{Limit: opts.Limit, FullMode: true})
	if err != nil {
		return nil, err
	}
	candidates := inspectCandidates(searchResult.Matches, repo, opts)
	fallbackHint := InspectHint{}
	if len(candidates) == 0 {
		for _, term := range queryTerms(query) {
			if term == strings.TrimSpace(strings.ToLower(query)) {
				continue
			}
			termQuery := buildInspectSearchQuery(repo, term, opts)
			termResult, err := Search(termQuery, SearchOpts{Limit: opts.Limit, FullMode: true})
			if err != nil {
				return nil, err
			}
			candidates = inspectCandidates(termResult.Matches, repo, opts)
			if len(candidates) > 0 {
				searchResult = termResult
				fallbackHint = InspectHint{
					Reason:  "candidate fallback",
					Command: fmt.Sprintf("exact phrase had no candidates; ranked evidence comes from term %q. Narrow with --path/--glob or try: ghx inspect %s %q --budget %d", term, repo, query, opts.Budget*2),
				}
				break
			}
		}
	}
	result := &InspectResult{
		Repo:             repo,
		Query:            query,
		Budget:           opts.Budget,
		TotalMatches:     searchResult.Total,
		SearchIncomplete: searchResult.Incomplete,
		Candidates:       candidates,
	}
	if fallbackHint.Command != "" {
		result.Truncation.Hints = append(result.Truncation.Hints, fallbackHint)
	}
	if len(candidates) == 0 {
		result.Truncation.Hints = append(result.Truncation.Hints, InspectHint{
			Reason:  "no candidates",
			Command: fmt.Sprintf("ghx search %s %q --lang LANG or --glob \"**/*\"", repo, query),
		})
		return result, nil
	}

	paths := candidatePaths(candidates, maxInspectFetchedFiles)
	readResults, err := Read(repo, paths, &ReadOpts{FullMode: true})
	if err != nil {
		return nil, err
	}

	files := make([]InspectFile, 0, len(readResults))
	for _, readResult := range readResults {
		if readResult.NotFound || readResult.Content == "" {
			continue
		}
		files = append(files, buildInspectFile(query, readResult))
	}
	result.Files = NewInspectRanker(query).Rank(files, candidates, opts)
	if len(candidates) > len(paths) {
		result.Truncation.OmittedFiles += len(candidates) - len(paths)
		result.Truncation.Hints = append(result.Truncation.Hints, InspectHint{
			Reason:  "candidate fetch limit",
			Command: fmt.Sprintf("raise --limit or narrow with --glob/--path, e.g. ghx inspect %s %q --glob \"**/*.go\"", repo, query),
		})
	}
	return result, nil
}

// FormatInspectText renders inspect output for agent consumption within the budget.
func FormatInspectText(result *InspectResult) string {
	if result == nil {
		return ""
	}
	budget := result.Budget
	if budget <= 0 {
		budget = defaultInspectBudget
	}
	writer := NewBudgetWriter(budget)
	_ = writer.Writef("inspect: %s %q\n", result.Repo, result.Query)
	_ = writer.Writef("budget: 0/%d chars\n", budget)
	_ = writer.Writef("matches: total=%d candidates=%d ranked=%d", result.TotalMatches, len(result.Candidates), len(result.Files))
	if result.SearchIncomplete {
		_ = writer.Write(" incomplete=true")
	}
	_ = writer.Write("\nranked files:\n")

	if len(result.Files) == 0 {
		_ = writer.Write("  (no ranked files)\n")
		truncation := result.Truncation
		if len(truncation.Hints) == 0 {
			truncation.Hints = append(truncation.Hints, InspectHint{
				Reason:  "no results",
				Command: fmt.Sprintf("ghx search %s %q --limit 30", result.Repo, result.Query),
			})
		}
		writeInspectTruncation(writer, truncation)
		return finalizeInspectBudgetLine(writer.String(), writer.Used(), budget)
	}

	truncation := result.Truncation
	reserveFooter := compactInspectTruncationHint()
	for i, file := range result.Files {
		section := formatInspectFileSection(result.Repo, i+1, file)
		if writer.Used()+len(section)+len(reserveFooter) > budget {
			truncation.OmittedFiles += len(result.Files) - i
			truncation.Hints = append(truncation.Hints, InspectHint{
				Reason:  "ranked file omitted",
				Command: fmt.Sprintf("raise --budget or read directly: ghx read %s %s --map", result.Repo, file.Path),
			})
			break
		}
		_ = writer.Write(section)
	}
	if truncation.OmittedFiles > 0 || truncation.OmittedSnippets > 0 || len(truncation.Hints) > 0 {
		writeInspectTruncation(writer, truncation)
	}

	out := finalizeInspectBudgetLine(writer.String(), writer.Used(), budget)
	result.Used = len(out)
	return out
}

func normalizeInspectOptions(opts InspectOptions) InspectOptions {
	if opts.Budget <= 0 {
		opts.Budget = defaultInspectBudget
	}
	if opts.Limit <= 0 {
		opts.Limit = defaultInspectLimit
	}
	if opts.Limit > maxInspectLimit {
		opts.Limit = maxInspectLimit
	}
	opts.Lang = strings.TrimSpace(opts.Lang)
	opts.Glob = strings.TrimSpace(opts.Glob)
	opts.Path = strings.Trim(strings.TrimSpace(opts.Path), "/")
	return opts
}

func validRepo(repo string) bool {
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

func buildInspectSearchQuery(repo string, query string, opts InspectOptions) string {
	searchTerms := strings.Join(queryTerms(query), " ")
	if searchTerms == "" {
		searchTerms = strconv.Quote(query)
	}
	searchQuery := fmt.Sprintf("%s repo:%s", searchTerms, repo)
	if opts.Lang != "" {
		searchQuery += " language:" + opts.Lang
	}
	if opts.Glob != "" {
		searchQuery += " path:" + opts.Glob
	}
	if opts.Path != "" {
		searchQuery += " path:" + opts.Path
	}
	return searchQuery
}

func inspectCandidates(matches []SearchMatch, repo string, opts InspectOptions) []InspectCandidate {
	byPath := make(map[string]int)
	var candidates []InspectCandidate
	for i, match := range matches {
		if match.Repo != "" && !strings.EqualFold(match.Repo, repo) {
			continue
		}
		if opts.Path != "" && !strings.Contains(strings.ToLower(match.Path), strings.ToLower(opts.Path)) {
			continue
		}
		if opts.Glob != "" {
			matched, _ := doublestar.Match(opts.Glob, match.Path)
			if !matched && !strings.Contains(strings.ToLower(match.Path), strings.ToLower(strings.Trim(opts.Glob, "*"))) {
				continue
			}
		}
		if idx, ok := byPath[match.Path]; ok {
			candidates[idx].MatchCount++
			if match.Fragment != "" && len(candidates[idx].Fragments) < 3 {
				candidates[idx].Fragments = append(candidates[idx].Fragments, match.Fragment)
			}
			continue
		}
		candidate := InspectCandidate{
			Path:       match.Path,
			SearchRank: i + 1,
			MatchCount: 1,
		}
		if match.Fragment != "" {
			candidate.Fragments = []string{match.Fragment}
		}
		byPath[match.Path] = len(candidates)
		candidates = append(candidates, candidate)
	}
	return candidates
}

func candidatePaths(candidates []InspectCandidate, maxFiles int) []string {
	limit := len(candidates)
	if limit > maxFiles {
		limit = maxFiles
	}
	paths := make([]string, 0, limit)
	for _, candidate := range candidates[:limit] {
		paths = append(paths, candidate.Path)
	}
	return paths
}

func buildInspectFile(query string, readResult FileResult) InspectFile {
	mapResult, err := mapengine.Map(readResult.Path, []byte(readResult.Content), mapengine.Options{Level: mapengine.LevelCompact})
	mapLines := mapResult.Lines
	warnings := mapResult.Warnings
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	if len(mapLines) == 0 {
		mapLines = []string{"(no signatures detected)"}
	}
	return InspectFile{
		Path:        readResult.Path,
		ByteSize:    readResult.ByteSize,
		MapLines:    mapLines,
		MapEngine:   string(mapResult.Engine),
		MapWarnings: warnings,
		Snippets:    ExtractInspectSnippets(readResult.Content, query, defaultInspectSnippetLines, defaultInspectSnippetChars),
		content:     readResult.Content,
	}
}

// ExtractInspectSnippets returns bounded line-context snippets for query hits.
func ExtractInspectSnippets(content string, query string, contextLines int, maxChars int) []InspectSnippet {
	if contextLines < 0 {
		contextLines = defaultInspectSnippetLines
	}
	if maxChars <= 0 {
		maxChars = defaultInspectSnippetChars
	}
	lines := strings.Split(content, "\n")
	terms := queryTerms(query)
	var hitLines []int
	for i, line := range lines {
		if termHits(line, terms) > 0 {
			hitLines = append(hitLines, i+1)
		}
	}
	if len(hitLines) == 0 && len(lines) > 0 {
		hitLines = append(hitLines, 1)
	}

	var snippets []InspectSnippet
	for _, hitLine := range hitLines {
		start := max(1, hitLine-contextLines)
		end := min(len(lines), hitLine+contextLines)
		if overlapsExisting(snippets, start, end) {
			continue
		}
		text := linesWithNumbers(lines, start, end)
		if len(text) > maxChars {
			text = text[:maxChars]
			if lastNewline := strings.LastIndex(text, "\n"); lastNewline > 0 {
				text = text[:lastNewline]
			}
			text += "\n..."
		}
		snippets = append(snippets, InspectSnippet{StartLine: start, EndLine: end, Text: text})
		if len(snippets) >= 2 {
			break
		}
	}
	return snippets
}

func overlapsExisting(snippets []InspectSnippet, start int, end int) bool {
	for _, snippet := range snippets {
		if start <= snippet.EndLine && end >= snippet.StartLine {
			return true
		}
	}
	return false
}

func linesWithNumbers(lines []string, start int, end int) string {
	var b strings.Builder
	for lineNo := start; lineNo <= end; lineNo++ {
		fmt.Fprintf(&b, "%d: %s\n", lineNo, lines[lineNo-1])
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatInspectFileSection(repo string, rank int, file InspectFile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d. %s score=%d reasons=%s\n", rank, file.Path, file.Score, strings.Join(file.Reasons, ","))
	b.WriteString("   map:\n")
	mapLimit := min(8, len(file.MapLines))
	for _, line := range file.MapLines[:mapLimit] {
		fmt.Fprintf(&b, "     %s\n", line)
	}
	if len(file.MapLines) > mapLimit {
		fmt.Fprintf(&b, "     ... %d more map lines; use: ghx read %s %s --map\n", len(file.MapLines)-mapLimit, repo, file.Path)
	}
	for _, warning := range file.MapWarnings {
		fmt.Fprintf(&b, "     # map warning: %s\n", warning)
	}
	b.WriteString("   snippets:\n")
	snippetLimit := min(2, len(file.Snippets))
	for _, snippet := range file.Snippets[:snippetLimit] {
		fmt.Fprintf(&b, "     %d-%d:\n", snippet.StartLine, snippet.EndLine)
		for _, line := range strings.Split(snippet.Text, "\n") {
			fmt.Fprintf(&b, "       %s\n", line)
		}
	}
	if len(file.Snippets) > snippetLimit {
		fmt.Fprintf(&b, "     ... %d more snippets; raise --budget or narrow with --path %s\n", len(file.Snippets)-snippetLimit, file.Path)
	}
	if len(file.Snippets) == 0 {
		b.WriteString("     (no snippets)\n")
	}
	b.WriteString("   next:\n")
	if len(file.Snippets) > 0 {
		first := file.Snippets[0]
		fmt.Fprintf(&b, "     ghx read %s %s --lines %d-%d\n", repo, file.Path, first.StartLine, first.EndLine)
	} else {
		fmt.Fprintf(&b, "     ghx read %s %s --map\n", repo, file.Path)
	}
	return b.String()
}

func writeInspectTruncation(writer *BudgetWriter, truncation InspectTruncation) {
	var b strings.Builder
	b.WriteString("truncation:\n")
	if truncation.OmittedFiles > 0 {
		fmt.Fprintf(&b, "  omitted %d files\n", truncation.OmittedFiles)
	}
	if truncation.OmittedSnippets > 0 {
		fmt.Fprintf(&b, "  omitted %d snippets\n", truncation.OmittedSnippets)
	}
	for _, hint := range truncation.Hints {
		fmt.Fprintf(&b, "  %s: %s\n", hint.Reason, hint.Command)
	}
	if b.String() == "truncation:\n" {
		b.WriteString("  none\n")
	}
	if !writer.Write(b.String()) {
		_ = writer.Write(compactInspectTruncationHint())
	}
}

func compactInspectTruncationHint() string {
	return "truncation:\n  output omitted by --budget; raise --budget, narrow with --glob/--path/--lang, or read a ranked file range.\n"
}

func finalizeInspectBudgetLine(out string, used int, budget int) string {
	lines := strings.SplitN(out, "\n", 3)
	if len(lines) < 3 {
		return out
	}
	lines[1] = fmt.Sprintf("budget: %d/%d chars", len(out), budget)
	return strings.Join(lines, "\n")
}

func queryTerms(query string) []string {
	re := regexp.MustCompile(`[A-Za-z0-9_]+`)
	raw := re.FindAllString(strings.ToLower(query), -1)
	seen := make(map[string]bool, len(raw))
	var terms []string
	for _, term := range raw {
		if len(term) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func termHits(text string, terms []string) int {
	lower := strings.ToLower(text)
	hits := 0
	for _, term := range terms {
		if strings.Contains(lower, term) {
			hits++
		}
	}
	return hits
}

func symbolTermHits(lines []string, terms []string) int {
	hits := 0
	for _, line := range lines {
		hits += termHits(line, terms)
	}
	return hits
}

func languageMatches(filePath string, lang string) bool {
	ext := strings.ToLower(path.Ext(filePath))
	switch strings.ToLower(lang) {
	case "go", "golang":
		return ext == ".go"
	case "typescript", "ts":
		return ext == ".ts" || ext == ".tsx"
	case "javascript", "js":
		return ext == ".js" || ext == ".jsx"
	case "python", "py":
		return ext == ".py"
	case "rust", "rs":
		return ext == ".rs"
	default:
		return false
	}
}

func candidateLocalReferenceCounts(files []InspectFile) map[string]int {
	counts := make(map[string]int, len(files))
	names := make(map[string][]string, len(files))
	for _, file := range files {
		base := strings.TrimSuffix(path.Base(file.Path), path.Ext(file.Path))
		dir := path.Dir(file.Path)
		names[file.Path] = []string{file.Path, base}
		if dir != "." {
			names[file.Path] = append(names[file.Path], dir+"/"+base)
		}
	}
	for _, source := range files {
		haystack := source.content
		if haystack == "" {
			haystack = strings.Join(source.MapLines, "\n")
		}
		haystack = strings.ToLower(haystack)
		for targetPath, targetNames := range names {
			if source.Path == targetPath {
				continue
			}
			for _, name := range targetNames {
				if len(name) >= 3 && strings.Contains(haystack, strings.ToLower(name)) {
					counts[targetPath]++
					break
				}
			}
		}
	}
	return counts
}
