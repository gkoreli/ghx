package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"
)

const VERSION = "0.2.1"

var RootCmd = &cobra.Command{
	Use:   "ghx",
	Short: "GitHub code exploration for agents and humans",
	Long:  `ghx — GitHub code exploration for agents and humans`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

func init() {
	RootCmd.SilenceErrors = true
	RootCmd.SilenceUsage = true
	RootCmd.AddCommand(reposCmd, exploreCmd, readCmd, searchCmd, treeCmd, skillCmd, versionCmd)
}

// reposCmd searches GitHub repos with README preview
var reposCmd = &cobra.Command{
	Use:   "repos <query> [--limit N]",
	Short: "Search repos with README preview",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := args[0]
		limit, _ := cmd.Flags().GetInt("limit")
		if limit > 20 {
			fmt.Fprintln(os.Stderr, "⚠ --limit clamped to 20 (README fetching makes larger values slow)")
			limit = 20
		}
		return repos(query, limit)
	},
}

func init() {
	reposCmd.Flags().IntP("limit", "l", 10, "Max results (max 20)")
}

// exploreCmd returns branch, tree, and README for a repo
var exploreCmd = &cobra.Command{
	Use:   "explore <owner/repo> [path]",
	Short: "Branch + tree + README in 1 API call",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return explore(args)
	},
}

// readCmd reads 1-10 files in 1 API call
var readCmd = &cobra.Command{
	Use:   "read <owner/repo> <path1> [path2...]",
	Short: "Read 1-10 files in 1 API call",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return readCmdRun(args, cmd)
	},
}

func init() {
	readCmd.Flags().String("grep", "", "Filter output to matching lines")
	readCmd.Flags().String("lines", "", "Extract specific line range (e.g., 42-80)")
	readCmd.Flags().Bool("map", false, "Structural signatures only")
}

// searchCmd searches code via REST API
var searchCmd = &cobra.Command{
	Use:   "search <query> [--limit N] [--full]",
	Short: "Code search (AND matching, matching context)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return searchCmdRun(args, cmd)
	},
}

func init() {
	searchCmd.Flags().IntP("limit", "l", 30, "Number of results")
	searchCmd.Flags().Bool("full", false, "Show complete fragments without truncation")
}

// treeCmd returns full recursive directory listing
var treeCmd = &cobra.Command{
	Use:   "tree <owner/repo> [path]",
	Short: "Full recursive tree listing",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tree(args)
	},
}

// skillCmd outputs SKILL.md
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Output SKILL.md for agent context injection",
	Run: func(cmd *cobra.Command, args []string) {
		skill()
	},
}

// versionCmd prints version
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ghx %s\n", VERSION)
	},
}

func runGH(args ...string) (string, error) {
	// Deprecated: use go-gh API directly
	return "", fmt.Errorf("use go-gh API instead")
}

func runGHOutput(args ...string) (string, error) {
	// Deprecated: use go-gh API directly
	return "", fmt.Errorf("use go-gh API instead")
}

func skill() error {
	// Try multiple paths relative to executable
	scriptDir, _ := filepath.EvalSymlinks(os.Args[0])
	dir := filepath.Dir(scriptDir)

	paths := []string{
		filepath.Join(dir, "SKILL.md"),
		filepath.Join(dir, "..", "SKILL.md"),
		filepath.Join(dir, "..", "v2", "SKILL.md"),
		filepath.Join(dir, "..", "..", "SKILL.md"),
		"SKILL.md",
		"../SKILL.md",
		"v2/SKILL.md",
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			fmt.Print(string(data))
			return nil
		}
	}

	return fmt.Errorf("SKILL.md not found")
}

// repos searches GitHub repos with README preview
func repos(query string, limit int) error {
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("failed to create GraphQL client: %w", err)
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
		return fmt.Errorf("graphql query failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%d repos found\n", resp.Search.RepositoryCount)

	if len(resp.Search.Nodes) == 0 {
		os.Exit(1)
		return nil
	}

	for _, repo := range resp.Search.Nodes {
		lang := "?"
		if repo.PrimaryLanguage != nil {
			lang = repo.PrimaryLanguage.Name
		}

		readme := cleanReadme(repo.Object.Text)

		fmt.Printf("%s (%d★ %s) %s\n", repo.NameWithOwner, repo.StargazerCount, lang, repo.Description)
		if readme != "" {
			fmt.Printf("  %s\n", readme)
		}
	}
	return nil
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

// explore returns branch, tree, and README for a repo
func explore(args []string) error {
	repo := args[0]
	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	owner := strings.Split(repo, "/")[0]
	name := strings.Split(repo, "/")[1]

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	if path == "" {
		query := fmt.Sprintf(`{
			repository(owner: %q, name: %q) {
				defaultBranchRef { name }
				description
				tree: object(expression: "HEAD:") {
					... on Tree { entries { name type } }
				}
				readme: object(expression: "HEAD:README.md") {
					... on Blob { text }
				}
			}
		}`, owner, name)

		var resp struct {
			Repository struct {
				DefaultBranchRef struct {
					Name string `json:"name"`
				} `json:"defaultBranchRef"`
				Description string `json:"description"`
				Tree        struct {
					Entries []struct {
						Name string `json:"name"`
						Type string `json:"type"`
					} `json:"entries"`
				} `json:"tree"`
				Readme struct {
					Text string `json:"text"`
				} `json:"readme"`
			} `json:"repository"`
		}

		if err := gql.Do(query, nil, &resp); err != nil {
			return fmt.Errorf("GraphQL query failed: %w", err)
		}

		branch := resp.Repository.DefaultBranchRef.Name
		fmt.Printf("description: %s\n", resp.Repository.Description)
		fmt.Printf("branch: %s\n", branch)
		fmt.Println("files:")
		for _, e := range resp.Repository.Tree.Entries {
			if e.Type == "tree" {
				fmt.Printf("  %s/\n", e.Name)
			} else {
				fmt.Printf("  %s\n", e.Name)
			}
		}
		readme := resp.Repository.Readme.Text
		if readme == "" {
			readme = "(no README.md)"
		}
		fmt.Printf("\nREADME:\n%s\n", readme)
	} else {
		query := fmt.Sprintf(`{
			repository(owner: %q, name: %q) {
				defaultBranchRef { name }
				tree: object(expression: "HEAD:%s") {
					... on Tree { entries { name type } }
				}
			}
		}`, owner, name, path)

		var resp struct {
			Repository struct {
				DefaultBranchRef struct {
					Name string `json:"name"`
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
			return fmt.Errorf("GraphQL query failed: %w", err)
		}

		fmt.Printf("path: %s\n", path)
		fmt.Println("entries:")
		for _, e := range resp.Repository.Tree.Entries {
			if e.Type == "tree" {
				fmt.Printf("  %s/\n", e.Name)
			} else {
				fmt.Printf("  %s\n", e.Name)
			}
		}
	}
	return nil
}

// readCmdRun handles the read command with flags
func readCmdRun(args []string, cmd *cobra.Command) error {
	grepPattern, _ := cmd.Flags().GetString("grep")
	lineRange, _ := cmd.Flags().GetString("lines")
	mapMode, _ := cmd.Flags().GetBool("map")

	if len(args) < 2 {
		return fmt.Errorf("usage: ghx read <owner/repo> <path1> [path2...]")
	}

	repo := args[0]
	files := args[1:]

	owner := strings.Split(repo, "/")[0]
	name := strings.Split(repo, "/")[1]

	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	// Build GraphQL query with defaultBranchRef + all file blobs using aliases
	query := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			defaultBranchRef { name }`, owner, name)

	for i := range files {
		query += fmt.Sprintf(` f%d: object(expression: "HEAD:%s") { ... on Blob { text byteSize } }`, i, url.QueryEscape(files[i]))
	}
	query += " } }"

	var resp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
		} `json:"repository"`
	}

	if err := gql.Do(query, nil, &resp); err != nil {
		return fmt.Errorf("GraphQL query failed: %w", err)
	}

	// Process each file
	for _, f := range files {
		// Query individual file to get its content
		fileQuery := fmt.Sprintf(`{
			repository(owner: %q, name: %q) {
				object(expression: "HEAD:%s") {
					... on Blob { text byteSize }
				}
			}
		}`, owner, name, f)

		var fileData struct {
			Repository struct {
				Object struct {
					Text     string `json:"text"`
					ByteSize int    `json:"byteSize"`
				} `json:"object"`
			} `json:"repository"`
		}

		if err := gql.Do(fileQuery, nil, &fileData); err != nil {
			fmt.Printf("=== %s (not found) ===\n\n", f)
			continue
		}

		text := fileData.Repository.Object.Text
		byteSize := fileData.Repository.Object.ByteSize

		if text == "" {
			fmt.Printf("=== %s (not found) ===\n\n", f)
			continue
		}

		fmt.Printf("=== %s (%d bytes) ===\n", f, byteSize)

		if grepPattern != "" {
			lines := strings.Split(text, "\n")
			found := false
			for j, line := range lines {
				if strings.Contains(strings.ToLower(line), strings.ToLower(grepPattern)) {
					start := j - 2
					if start < 0 {
						start = 0
					}
					end := j + 3
					if end > len(lines) {
						end = len(lines)
					}
					for k := start; k < end; k++ {
						prefix := "  "
						if k == j {
							prefix = "> "
						}
						fmt.Printf("%s%d: %s\n", prefix, k+1, lines[k])
					}
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("(no matches for '%s')\n", grepPattern)
			}
		} else if lineRange != "" {
			parts := strings.Split(lineRange, "-")
			if len(parts) != 2 {
				return fmt.Errorf("invalid line range format, use N-M")
			}
			start := parseInt(parts[0]) - 1
			end := parseInt(parts[1])
			lines := strings.Split(text, "\n")
			if start < len(lines) {
				if end > len(lines) {
					end = len(lines)
				}
				for j := start; j < end; j++ {
					fmt.Printf("%d: %s\n", j+1, lines[j])
				}
			}
		} else if mapMode {
			ext := strings.Split(f, ".")
			extStr := ext[len(ext)-1]
			pat := getMapPattern(extStr)
			mapLines := []string{}
			fullLines := strings.Split(text, "\n")
			for _, line := range fullLines {
				if match, _ := regexp.MatchString(pat, line); match {
					mapLines = append(mapLines, line)
				}
			}
			if len(mapLines) == 0 {
				fmt.Println("(no signatures detected)")
			} else {
				for _, line := range mapLines {
					fmt.Println(line)
				}
			}
			chars := len(text)
			mapChars := strings.Join(mapLines, "\n")
			fmt.Printf("# map: %d/%d chars\n", len(mapChars), chars)
		} else {
			fmt.Println(text)
		}
		fmt.Println()
	}
	return nil
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
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// searchCmdRun handles the search command with flags
func searchCmdRun(args []string, cmd *cobra.Command) error {
	query := args[0]
	limit, _ := cmd.Flags().GetInt("limit")
	fullMode, _ := cmd.Flags().GetBool("full")

	if limit > 100 {
		fmt.Fprintln(os.Stderr, "⚠ --limit clamped to 100 (API max)")
		limit = 100
	}

	rest, err := api.NewRESTClient(api.ClientOptions{
		Headers: map[string]string{"Accept": "application/vnd.github.text-match+json"},
	})
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
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
				Matches  []struct {
					Indices [2]int `json:"indices"`
				} `json:"matches"`
			} `json:"text_matches"`
		} `json:"items"`
	}

	path := fmt.Sprintf("search/code?q=%s&per_page=%d", url.QueryEscape(query), limit)
	if err := rest.Get(path, &resp); err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	fmt.Fprintf(os.Stderr, "%d results (showing %d)\n", resp.TotalCount, len(resp.Items))
	if resp.IncompleteResults {
		fmt.Fprintln(os.Stderr, "⚠ Results may be incomplete (query timed out)")
	}

	for _, item := range resp.Items {
		if len(item.TextMatches) > 0 {
			fragment := item.TextMatches[0].Fragment
			fragment = strings.ReplaceAll(fragment, "\n", " ")
			fragment = strings.Join(strings.Fields(fragment), " ")

			if !fullMode && len(fragment) > 200 {
				fragment = fragment[:200] + "…"
			}
			fmt.Printf("%s %s: %s\n", item.Repository.FullName, item.Path, fragment)
		} else {
			fmt.Printf("%s %s\n", item.Repository.FullName, item.Path)
		}
	}
	return nil
}

// tree returns full recursive directory listing
func tree(args []string) error {
	repo := args[0]
	path := ""
	if len(args) > 1 {
		path = args[1]
	}

	owner := strings.Split(repo, "/")[0]
	name := strings.Split(repo, "/")[1]

	// Get default branch
	gql, err := api.DefaultGraphQLClient()
	if err != nil {
		return fmt.Errorf("failed to create GraphQL client: %w", err)
	}

	branchQuery := fmt.Sprintf(`{
		repository(owner: %q, name: %q) {
			defaultBranchRef { name }
		}
	}`, owner, name)

	var branchResp struct {
		Repository struct {
			DefaultBranchRef struct {
				Name string `json:"name"`
			} `json:"defaultBranchRef"`
		} `json:"repository"`
	}

	if err := gql.Do(branchQuery, nil, &branchResp); err != nil {
		return fmt.Errorf("failed to get branch: %w", err)
	}

	branch := branchResp.Repository.DefaultBranchRef.Name
	if branch == "" {
		return fmt.Errorf("could not determine default branch")
	}

	// Use REST client for tree API
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("failed to create REST client: %w", err)
	}

	endpoint := fmt.Sprintf("repos/%s/%s/git/trees/%s?recursive=1", owner, name, branch)
	var resp struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
		} `json:"tree"`
	}

	if err := rest.Get(endpoint, &resp); err != nil {
		return fmt.Errorf("failed to get tree: %w", err)
	}

	for _, entry := range resp.Tree {
		if entry.Type == "blob" {
			if path != "" && strings.HasPrefix(entry.Path, path+"/") {
				fmt.Println(strings.TrimPrefix(entry.Path, path+"/"))
			} else if path == "" {
				fmt.Println(entry.Path)
			}
		}
	}
	return nil
}
