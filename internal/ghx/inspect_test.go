package ghx

import (
	"strings"
	"testing"
)

func TestInspectRankerDeterministicFeatures(t *testing.T) {
	files := []InspectFile{
		{
			Path:     "router/chain.go",
			MapLines: []string{"3: func ChainMiddleware() {", "9: func applyHandlers() {"},
			content:  `import "../middleware"`,
		},
		{
			Path:     "internal/plain.go",
			MapLines: []string{"3: func Other() {"},
		},
		{
			Path:     "middleware/middleware.go",
			MapLines: []string{"3: type Middleware interface {"},
		},
	}
	candidates := []InspectCandidate{
		{Path: "internal/plain.go", SearchRank: 1, MatchCount: 1},
		{Path: "router/chain.go", SearchRank: 2, MatchCount: 2},
		{Path: "middleware/middleware.go", SearchRank: 3, MatchCount: 1},
	}

	got := NewInspectRanker("middleware chain").Rank(files, candidates, InspectOptions{Lang: "go", Glob: "**/*.go", Path: "router"})
	if got[0].Path != "router/chain.go" {
		t.Fatalf("top path = %q, want router/chain.go; ranked=%#v", got[0].Path, got)
	}
	for _, want := range []string{"search-rank:2", "matches:2", "path-term", "symbol-term", "lang", "glob", "path"} {
		if !containsReason(got[0].Reasons, want) {
			t.Fatalf("top reasons = %#v, missing %q", got[0].Reasons, want)
		}
	}
	if !containsReason(got[1].Reasons, "candidate-local-refs:1") {
		t.Fatalf("second reasons = %#v, missing candidate-local refs", got[1].Reasons)
	}
}

func TestExtractInspectSnippetsBoundedContext(t *testing.T) {
	content := strings.Join([]string{
		"package router",
		"",
		"func first() {}",
		"func ChainMiddleware() {",
		"  useMiddleware()",
		"}",
		"func last() {}",
	}, "\n")

	snippets := ExtractInspectSnippets(content, "middleware", 1, 120)
	if len(snippets) != 1 {
		t.Fatalf("got %d snippets, want 1", len(snippets))
	}
	if snippets[0].StartLine != 3 || snippets[0].EndLine != 5 {
		t.Fatalf("range = %d-%d, want 3-5", snippets[0].StartLine, snippets[0].EndLine)
	}
	if !strings.Contains(snippets[0].Text, "4: func ChainMiddleware() {") {
		t.Fatalf("snippet text missing numbered hit: %q", snippets[0].Text)
	}
}

func TestFormatInspectTextBudgetAndTruncationHint(t *testing.T) {
	result := &InspectResult{
		Repo:         "owner/repo",
		Query:        "middleware chain",
		Budget:       700,
		TotalMatches: 5,
		Candidates: []InspectCandidate{
			{Path: "router/chain.go"},
			{Path: "middleware/middleware.go"},
			{Path: "internal/plain.go"},
		},
		Files: []InspectFile{
			fixtureInspectFile("router/chain.go", 120, "search-rank:1", "symbol-term"),
			fixtureInspectFile("middleware/middleware.go", 90, "search-rank:2", "path-term"),
			fixtureInspectFile("internal/plain.go", 30, "search-rank:3"),
		},
	}

	out := FormatInspectText(result)
	if len(out) > result.Budget+80 {
		t.Fatalf("output length = %d, want <= budget+tolerance %d\n%s", len(out), result.Budget+80, out)
	}
	for _, want := range []string{"inspect: owner/repo \"middleware chain\"", "ranked files:", "truncation:", "raise --budget", "ghx read owner/repo"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestInspectCandidatesDeduplicateAndFilter(t *testing.T) {
	matches := []SearchMatch{
		{Repo: "owner/repo", Path: "router/chain.go", Fragment: "one"},
		{Repo: "owner/repo", Path: "router/chain.go", Fragment: "two"},
		{Repo: "other/repo", Path: "router/chain.go", Fragment: "skip"},
		{Repo: "owner/repo", Path: "docs/readme.md", Fragment: "skip"},
	}

	got := inspectCandidates(matches, "owner/repo", InspectOptions{Path: "router", Glob: "**/*.go"})
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1: %#v", len(got), got)
	}
	if got[0].Path != "router/chain.go" || got[0].SearchRank != 1 || got[0].MatchCount != 2 {
		t.Fatalf("candidate = %#v, want deduped router/chain.go rank 1 count 2", got[0])
	}
	if len(got[0].Fragments) != 2 {
		t.Fatalf("fragments = %#v, want two", got[0].Fragments)
	}
}

func fixtureInspectFile(path string, score int, reasons ...string) InspectFile {
	return InspectFile{
		Path:     path,
		Score:    score,
		Reasons:  reasons,
		ByteSize: 200,
		MapLines: []string{
			"1: package router",
			"3: func ChainMiddleware() {",
			"9: func ApplyMiddleware() {",
		},
		Snippets: []InspectSnippet{
			{StartLine: 3, EndLine: 5, Text: "3: func ChainMiddleware() {\n4:   next()\n5: }"},
		},
	}
}

func containsReason(reasons []string, want string) bool {
	for _, reason := range reasons {
		if reason == want {
			return true
		}
	}
	return false
}
