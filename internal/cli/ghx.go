package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	ghxlib "github.com/gkoreli/ghx/v2/internal/ghx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var VERSION = "dev"

var RootCmd = &cobra.Command{
	Use:   "ghx",
	Short: "GitHub code exploration for agents and humans",
	Long: `ghx — GitHub code exploration for agents and humans

Exit codes:
  0  ok
  1  no results
  2  bad invocation
  3  upstream/API failure`,
	Version: VERSION,
	Example: `  ghx --version
  ghx explore gkoreli/ghx
  ghx read gkoreli/ghx cmd/ghx/main.go --lines 1-40`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
		}
	},
}

func init() {
	RootCmd.SilenceErrors = true
	RootCmd.SilenceUsage = true
	RootCmd.SetFlagErrorFunc(teachingFlagError)
	RootCmd.AddCommand(reposCmd, exploreCmd, readCmd, searchCmd, grepCmd, inspectCmd, treeCmd, skillCmd, versionCmd, sidecarCmd, tier2Cmd)
}

var reposCmd = &cobra.Command{
	Use:   "repos <query> [--limit N]",
	Short: "Search repos with README preview",
	Long: `Discover GitHub repositories by topic, returning each hit's stars, language,
and a README preview in one GraphQL call. Reach for this when you do not yet know
which repo to explore — it answers "what libraries do X", not "where in this repo
is Y" (that is ` + "`ghx search`/`ghx grep`" + `). Exit code 1 means the query
matched no repos; broaden it.`,
	Example: `  ghx repos "go web framework" --limit 5
  ghx repos "tree-sitter parser"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		results, total, err := ghxlib.Repos(args[0], ghxlib.ReposOpts{Limit: limit})
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}
		fmt.Printf("%d repos found\n", total)
		if len(results) == 0 {
			return WithExitCode(ExitNoResults, fmt.Errorf("no repos found for %q", args[0]))
		}
		for _, r := range results {
			fmt.Printf("%s (%d★ %s) %s\n", r.NameWithOwner, r.Stars, r.Language, r.Description)
			if r.ReadmePreview != "" {
				fmt.Printf("  %s\n", r.ReadmePreview)
			}
		}
		return nil
	},
}

func init() {
	reposCmd.Flags().IntP("limit", "l", 10, "Max results (max 20)")
}

var exploreCmd = &cobra.Command{
	Use:   "explore <owner/repo> [path]",
	Short: "Branch + tree + README in 1 API call",
	Long: `Orient yourself in an unfamiliar repo: default branch, top-level file listing,
and a README summary in a single API call. This is the first move when you do not
know a repo's layout — pass a path to list a subdirectory instead of the root.
Output is compacted to ` + "`--budget`" + ` chars by default; add ` + "`--full`" + ` for the
complete tree and README.`,
	Example: `  ghx explore gkoreli/ghx
  ghx explore gkoreli/ghx internal/cli
  ghx explore gkoreli/ghx --full`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 1 {
			path = args[1]
		}
		budget, _ := cmd.Flags().GetInt("budget")
		fullMode, _ := cmd.Flags().GetBool("full")
		result, err := ghxlib.Explore(args[0], path)
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}
		if !fullMode {
			printCompactExplore(args[0], path, result, budget)
			return nil
		}
		fmt.Printf("description: %s\n", result.Description)
		fmt.Printf("branch: %s\n", result.Branch)
		if path == "" {
			fmt.Println("files:")
			for _, e := range result.Files {
				if e.Type == "tree" {
					fmt.Printf("  %s/\n", e.Name)
				} else {
					fmt.Printf("  %s\n", e.Name)
				}
			}
			readme := result.Readme
			if readme == "" {
				readme = "(no README.md)"
			}
			fmt.Printf("\nREADME:\n%s\n", readme)
		} else {
			fmt.Println("entries:")
			for _, e := range result.Files {
				if e.Type == "tree" {
					fmt.Printf("  %s/\n", e.Name)
				} else {
					fmt.Printf("  %s\n", e.Name)
				}
			}
		}
		return nil
	},
}

func init() {
	exploreCmd.Flags().Int("budget", 12000, "Approximate output budget in characters")
	exploreCmd.Flags().Bool("full", false, "Show complete explore output without budget compaction")
}

var readCmd = &cobra.Command{
	Use:   "read <owner/repo> <path1> [path2...]",
	Short: "Read 1-10 files in 1 API call",
	Long: `Fetch up to 10 files (or a glob) in one batched API call. Prefer the narrowing
flags over reading whole files: ` + "`--map`" + ` returns parser-backed signatures only,
` + "`--grep PATTERN`" + ` shows just matching lines, and ` + "`--lines START-END`" + ` extracts a
range. A directory path returns its listing instead of "not found". A full read
over ` + "`--budget`" + ` chars (without --map/--lines/--grep) falls back to a structural
map plus a narrowing hint rather than flooding output.`,
	Example: `  ghx read gkoreli/ghx cmd/ghx/main.go --lines 1-40
  ghx read gkoreli/ghx "internal/**/*.go" --map --kind func
  ghx read gkoreli/ghx internal/cli/ghx.go --grep "RunE|Use:"`,
	Args: cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		grepPattern, _ := cmd.Flags().GetString("grep")
		lineRange, _ := cmd.Flags().GetString("lines")
		normalizedLines, err := normalizedReadLineRange(cmd, lineRange)
		if err != nil {
			return WithExitCode(ExitBadInvocation, err)
		}
		lineRange = normalizedLines
		mapMode, _ := cmd.Flags().GetBool("map")
		mapLevel, _ := cmd.Flags().GetString("level")
		mapKind, _ := cmd.Flags().GetString("kind")
		mapEngine, _ := cmd.Flags().GetString("map-engine")
		budget, _ := cmd.Flags().GetInt("budget")
		fullMode, _ := cmd.Flags().GetBool("full")

		opts := &ghxlib.ReadOpts{
			Grep:      grepPattern,
			Lines:     lineRange,
			Map:       mapMode,
			MapLevel:  mapLevel,
			MapKind:   mapKind,
			MapEngine: mapEngine,
			Budget:    budget,
			FullMode:  fullMode,
		}
		results, err := ghxlib.Read(args[0], args[1:], opts)
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}

		// Show glob summary when matches were truncated
		for _, gr := range opts.Globs {
			n := len(gr.Matches)
			if n > len(results) {
				fmt.Printf("# glob %q matched %d files (reading first %d)\n", gr.Pattern, n, len(results))
				if n <= 50 {
					for _, m := range gr.Matches {
						fmt.Printf("#   %s\n", m)
					}
				}
				fmt.Println()
			}
		}

		for _, r := range results {
			if r.NotFound {
				fmt.Printf("=== %s (not found) ===\n\n", r.Path)
				continue
			}
			if len(r.DirEntries) > 0 {
				fmt.Printf("=== %s (directory, %d entries) ===\n", r.Path, len(r.DirEntries))
				for _, e := range r.DirEntries {
					suffix := ""
					if e.Type == "tree" {
						suffix = "/"
					}
					fmt.Printf("  %s%s\n", e.Name, suffix)
				}
				fmt.Printf("→ use: ghx read %s \"%s/*\" --map\n\n", args[0], r.Path)
				continue
			}
			// Skip files with no grep matches (avoid empty headers)
			if grepPattern != "" && len(r.GrepHits) == 0 {
				continue
			}
			header := fmt.Sprintf("=== %s (%d bytes) ===", r.Path, r.ByteSize)
			if r.GlobPattern != "" {
				header = fmt.Sprintf("=== %s (%d bytes, via %s) ===", r.Path, r.ByteSize, r.GlobPattern)
			}
			fmt.Println(header)
			if len(r.GrepHits) > 0 {
				for _, hit := range r.GrepHits {
					if hit.LineNum == -1 {
						fmt.Println("--")
						continue
					}
					prefix := "  "
					if hit.IsMatch {
						prefix = "> "
					}
					fmt.Printf("%s%d: %s\n", prefix, hit.LineNum, hit.Line)
				}
			} else if len(r.MapLines) > 0 {
				if r.BudgetedMap {
					fmt.Println(readBudgetHint(r.Path, r.ByteSize))
				}
				for _, line := range r.MapLines {
					fmt.Println(line)
				}
				for _, warning := range r.MapWarnings {
					fmt.Printf("# map warning: %s\n", warning)
				}
				mapOutput := strings.Join(r.MapLines, "\n")
				fmt.Printf("# map: %d/%d chars (~%d tokens full, ~%d tokens map)\n", len(mapOutput), r.MapChars, r.MapChars/4, len(mapOutput)/4)
			} else {
				fmt.Println(r.Content)
			}
			fmt.Println()
		}

		// Hint at the end when glob matched more than we could read
		for _, gr := range opts.Globs {
			if len(gr.Matches) > len(results) {
				fmt.Printf("→ glob %q matched %d files, showing %d. Narrow the pattern to see others.\n", gr.Pattern, len(gr.Matches), len(results))
			}
		}

		return nil
	},
}

func init() {
	readCmd.Flags().String("grep", "", "Filter output to matching lines")
	readCmd.Flags().String("lines", "", "Extract specific line range (e.g., 42-80)")
	readCmd.Flags().Int("start", 0, "Hidden alias for --lines START-END")
	readCmd.Flags().Int("end", 0, "Hidden alias for --lines START-END")
	readCmd.Flags().Int("offset", 0, "Hidden alias for --lines START-END")
	readCmd.Flags().Int("limit", 0, "Hidden alias for --lines START-END")
	readCmd.Flags().Bool("map", false, "Structural signatures only")
	readCmd.Flags().String("level", "compact", "Map detail level: outline|minimal|compact|standard")
	readCmd.Flags().String("kind", "", "Map symbol kind filter: func|type|import|const|var|package")
	readCmd.Flags().String("map-engine", "auto", "Map engine: auto|regex|tree-sitter")
	readCmd.Flags().Int("budget", 12000, "Approximate output budget in characters before structural fallback")
	readCmd.Flags().Bool("full", false, "Show complete file contents without budget fallback")
	_ = readCmd.Flags().MarkHidden("start")
	_ = readCmd.Flags().MarkHidden("end")
	_ = readCmd.Flags().MarkHidden("offset")
	_ = readCmd.Flags().MarkHidden("limit")
}

var searchCmd = &cobra.Command{
	Use:   "search [<owner/repo>] <query> [--limit N] [--full]",
	Short: "Code search (AND matching, matching context)",
	Long: `Search inside file contents across GitHub, returning matching fragments with
context. Every word is AND'd, so 1-2 terms match more than 5. Use the repo-first
form ` + "`ghx search owner/repo \"query\"`" + ` when you know the repo; the raw-query form
accepts GitHub qualifiers (` + "`repo:`, `language:`, `path:`" + `). To discover repos by
topic use ` + "`ghx repos`" + ` instead. Exit code 1 means no code matched.`,
	Example: `  ghx search gkoreli/ghx "func main" --lang go
  ghx search "repo:gkoreli/ghx cobra.Command" --limit 10
  ghx search gkoreli/ghx "SetFlagErrorFunc" --glob "internal/**/*.go"`,
	SuggestFor: []string{"find"},
	Args:       cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		fullMode, _ := cmd.Flags().GetBool("full")
		budget, _ := cmd.Flags().GetInt("budget")
		lang, _ := cmd.Flags().GetString("lang")
		glob, _ := cmd.Flags().GetString("glob")
		query := buildSearchQuery(args, lang, glob)

		result, err := ghxlib.Search(query, ghxlib.SearchOpts{
			Limit:    limit,
			FullMode: fullMode,
			Budget:   budget,
		})
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}

		fmt.Printf("%d results (showing %d)\n", result.Total, len(result.Matches))
		if result.Incomplete {
			fmt.Println("Results may be incomplete (query timed out)")
		}

		if len(result.Matches) == 0 {
			fmt.Println("→ To search repos by topic, use: ghx repos \"<query>\"")
			return WithExitCode(ExitNoResults, fmt.Errorf("no code results for %q", query))
		}

		for _, m := range result.Matches {
			fmt.Printf("%s %s: %s\n", m.Repo, m.Path, m.Fragment)
		}
		if result.Truncated {
			fmt.Println("→ output truncated by --budget; narrow with --lang/--glob or use --full.")
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().IntP("limit", "l", 30, "Number of results")
	searchCmd.Flags().Bool("full", false, "Show complete fragments without truncation")
	searchCmd.Flags().Int("budget", 12000, "Approximate output budget in characters")
	searchCmd.Flags().String("lang", "", "Limit repo-first search by language")
	searchCmd.Flags().String("glob", "", "Limit repo-first search by path glob")
}

var grepCmd = &cobra.Command{
	Use:   "grep <owner/repo> <pattern>",
	Short: "Search a repo with grep-like flag names",
	Long: `Repo-scoped code search with grep-style ergonomics: ` + "`--path`, `--glob`, `--limit`" + `.
Reach for this when your muscle memory is ` + "`grep`/`rg`" + ` — it is backed by GitHub
code search (AND matching, indexing lag applies), not a local ripgrep over the
tree, so ` + "`--glob`" + ` maps to a GitHub ` + "`path:`" + ` qualifier. Exit code 1 means the
pattern matched nothing in the repo.`,
	Example: `  ghx grep gkoreli/ghx "func main"
  ghx grep gkoreli/ghx "cobra.Command" --path internal/cli
  ghx grep gkoreli/ghx "RunE" --glob "internal/**/*.go" --limit 10`,
	SuggestFor: []string{"rg"},
	Args:       cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		glob, _ := cmd.Flags().GetString("glob")
		path, _ := cmd.Flags().GetString("path")
		limit, _ := cmd.Flags().GetInt("limit")
		budget, _ := cmd.Flags().GetInt("budget")
		query := buildGrepQuery(args[0], args[1], glob, path)
		result, err := ghxlib.Search(query, ghxlib.SearchOpts{
			Limit:  limit,
			Budget: budget,
		})
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}
		fmt.Printf("%d results (showing %d)\n", result.Total, len(result.Matches))
		if len(result.Matches) == 0 {
			return WithExitCode(ExitNoResults, fmt.Errorf("no grep results for %q in %s", args[1], args[0]))
		}
		for _, m := range result.Matches {
			fmt.Printf("%s %s: %s\n", m.Repo, m.Path, m.Fragment)
		}
		if result.Truncated {
			fmt.Println("→ output truncated by --budget; narrow with --path/--glob or use --limit.")
		}
		return nil
	},
}

func init() {
	grepCmd.Flags().String("glob", "", "Limit matches by path glob")
	grepCmd.Flags().String("path", "", "Limit matches to paths containing this value")
	grepCmd.Flags().IntP("limit", "l", 30, "Number of results")
	grepCmd.Flags().Int("budget", 12000, "Approximate output budget in characters")
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <owner/repo> <query>",
	Short: "Rank files, maps, snippets, and next reads for a concern",
	Long: `Answer "where does this repo handle X" in one budgeted call: inspect searches for
candidates, ranks the files that matter, and returns their structural maps,
bounded snippets, and suggested next reads. Use it when you have a concern but not
a filename — it collapses the explore → search → map → read loop into a single
result. Exit code 1 means nothing ranked for the query.`,
	Example: `  ghx inspect gkoreli/ghx "flag error suggestions"
  ghx inspect gin-gonic/gin "routing middleware" --lang go
  ghx inspect openai/openai-node "streaming responses" --glob "src/**/*.ts"`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		budget, _ := cmd.Flags().GetInt("budget")
		limit, _ := cmd.Flags().GetInt("limit")
		lang, _ := cmd.Flags().GetString("lang")
		glob, _ := cmd.Flags().GetString("glob")
		path, _ := cmd.Flags().GetString("path")
		result, err := ghxlib.Inspect(args[0], args[1], ghxlib.InspectOptions{
			Budget: budget,
			Limit:  limit,
			Lang:   lang,
			Glob:   glob,
			Path:   path,
		})
		if err != nil {
			if strings.Contains(err.Error(), "invalid repo format") || strings.Contains(err.Error(), "query must not be empty") {
				return WithExitCode(ExitBadInvocation, err)
			}
			return WithExitCode(ExitUpstreamFailure, err)
		}
		fmt.Print(ghxlib.FormatInspectText(result))
		if len(result.Files) == 0 {
			return WithExitCode(ExitNoResults, fmt.Errorf("no inspect results for %q in %s", args[1], args[0]))
		}
		return nil
	},
}

func init() {
	inspectCmd.Flags().Int("budget", 12000, "Approximate total output budget in characters")
	inspectCmd.Flags().IntP("limit", "l", 30, "Candidate search limit before ranking (max 100)")
	inspectCmd.Flags().String("lang", "", "Limit search candidates by language")
	inspectCmd.Flags().String("glob", "", "Limit candidates by path glob")
	inspectCmd.Flags().String("path", "", "Prefer or constrain candidates to a subtree")
}

var treeCmd = &cobra.Command{
	Use:   "tree <owner/repo> [path]",
	Short: "Full recursive tree listing",
	Long: `Print the full recursive file tree of a repo (or a subtree, given a path). Use it
when you need the complete file layout rather than the compacted top-level view
` + "`ghx explore`" + ` gives you. Cap the depth with ` + "`--depth N`" + ` (0, the default, means
fully recursive); directories are shown with a trailing slash.`,
	Example: `  ghx tree gkoreli/ghx
  ghx tree gkoreli/ghx internal --depth 2`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 1 {
			path = args[1]
		}
		depth, _ := cmd.Flags().GetInt("depth")
		if depth < 0 {
			return fmt.Errorf("--depth must be a positive integer")
		}
		results, err := ghxlib.Tree(args[0], path, ghxlib.TreeOpts{Depth: depth})
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}
		for _, entry := range results {
			fmt.Println(entry)
		}
		return nil
	},
}

func init() {
	treeCmd.Flags().IntP("depth", "d", 0, "Limit tree depth (0 = full recursive)")
}

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Output SKILL.md for agent context injection",
	Long: `Print an embedded SKILL.md to stdout so a main agent can inject ghx usage into
its own context. The default is the classic-CLI skill; ` + "`--mcp`" + ` prints the
MCP-server skill and ` + "`--recon`" + ` prints the concise sidecar/recon skill (the one to
give an agent that should delegate whole questions rather than drive the CLI).
The flags are mutually exclusive.`,
	Example: `  ghx skill
  ghx skill --mcp
  ghx skill --recon`,
	RunE: func(cmd *cobra.Command, args []string) error {
		mcpFlag, _ := cmd.Flags().GetBool("mcp")
		reconFlag, _ := cmd.Flags().GetBool("recon")
		if mcpFlag && reconFlag {
			return fmt.Errorf("--mcp and --recon are mutually exclusive")
		}
		filename := "SKILL.md"
		if mcpFlag {
			filename = "MCP-SKILL.md"
		} else if reconFlag {
			filename = "RECON-SKILL.md"
		}
		return printSkill(filename)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Long: `Print the ghx build version. Equivalent to the root ` + "`ghx --version`" + ` flag; both
exist so version can be queried as a subcommand or a flag.`,
	Example: `  ghx version
  ghx --version`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ghx %s\n", VERSION)
	},
}

func init() {
	skillCmd.Flags().Bool("mcp", false, "Output MCP skill instead of CLI skill")
	skillCmd.Flags().Bool("recon", false, "Output recon sidecar skill instead of CLI skill")
}

func teachingFlagError(cmd *cobra.Command, err error) error {
	flag := unknownFlagName(err.Error())
	if flag == "" {
		return WithExitCode(ExitBadInvocation, err)
	}
	nearest := nearestFlag(cmd, flag)
	if alias := knownFlagAlias(flag); alias != "" {
		nearest = alias
	}
	if nearest == "" {
		return WithExitCode(ExitBadInvocation, fmt.Errorf("unknown flag --%s; run %s --help for valid flags", flag, cmd.CommandPath()))
	}
	return WithExitCode(ExitBadInvocation, fmt.Errorf("unknown flag --%s; use %s, e.g. %s", flag, nearest, exampleForFlag(cmd, nearest)))
}

func unknownFlagName(message string) string {
	const prefix = "unknown flag: --"
	if !strings.Contains(message, prefix) {
		return ""
	}
	flag := strings.TrimPrefix(message[strings.Index(message, prefix):], prefix)
	if i := strings.IndexAny(flag, " =\n\t"); i >= 0 {
		flag = flag[:i]
	}
	return strings.TrimSpace(flag)
}

func nearestFlag(cmd *cobra.Command, unknown string) string {
	type candidate struct {
		name     string
		distance int
	}
	var candidates []candidate
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		candidates = append(candidates, candidate{name: flag.Name, distance: levenshtein(unknown, flag.Name)})
	})
	cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}
		candidates = append(candidates, candidate{name: flag.Name, distance: levenshtein(unknown, flag.Name)})
	})
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].distance == candidates[j].distance {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].distance < candidates[j].distance
	})
	if candidates[0].distance > 4 {
		return ""
	}
	return "--" + candidates[0].name
}

func knownFlagAlias(flag string) string {
	switch flag {
	case "start", "end", "offset", "limit", "line-range":
		return "--lines START-END"
	case "type":
		return "--lang LANG"
	default:
		return ""
	}
}

func exampleForFlag(cmd *cobra.Command, nearest string) string {
	switch nearest {
	case "--lines START-END":
		return "ghx read owner/repo main.go --lines 40-80"
	case "--lang LANG":
		return "ghx search owner/repo \"middleware\" --lang go"
	case "--glob":
		return "ghx search owner/repo \"middleware\" --glob \"**/*.go\""
	case "--budget":
		return cmd.CommandPath() + " --budget 20000"
	case "--full":
		return cmd.CommandPath() + " --full"
	default:
		return cmd.CommandPath() + " " + nearest
	}
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr := make([]int, len(b)+1)
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev = curr
	}
	return prev[len(b)]
}

func normalizedReadLineRange(cmd *cobra.Command, lineRange string) (string, error) {
	start, _ := cmd.Flags().GetInt("start")
	end, _ := cmd.Flags().GetInt("end")
	offset, _ := cmd.Flags().GetInt("offset")
	limit, _ := cmd.Flags().GetInt("limit")
	hasStart := cmd.Flags().Changed("start") || cmd.Flags().Changed("end")
	hasOffset := cmd.Flags().Changed("offset") || cmd.Flags().Changed("limit")
	if lineRange != "" && (hasStart || hasOffset) {
		return "", fmt.Errorf("--lines cannot be combined with hidden aliases --start/--end or --offset/--limit")
	}
	if hasStart && hasOffset {
		return "", fmt.Errorf("--start/--end cannot be combined with --offset/--limit")
	}
	if hasStart {
		if start <= 0 || end <= 0 || end < start {
			return "", fmt.Errorf("use --lines START-END, e.g. ghx read owner/repo main.go --lines 40-80")
		}
		return fmt.Sprintf("%d-%d", start, end), nil
	}
	if hasOffset {
		if offset <= 0 || limit <= 0 {
			return "", fmt.Errorf("use --lines START-END, e.g. ghx read owner/repo main.go --lines 40-80")
		}
		return fmt.Sprintf("%d-%d", offset, offset+limit-1), nil
	}
	return lineRange, nil
}

func readBudgetHint(path string, byteSize int) string {
	return fmt.Sprintf("# budget: %s is %d bytes; showing structural map. Use --lines START-END, --grep PATTERN, --map, or --full to read more.", path, byteSize)
}

func printCompactExplore(repo string, path string, result *ghxlib.ExploreResult, budget int) {
	if budget <= 0 {
		budget = 12000
	}
	var b strings.Builder
	fmt.Fprintf(&b, "description: %s\n", result.Description)
	fmt.Fprintf(&b, "branch: %s\n", result.Branch)
	label := "files"
	if path != "" {
		label = "entries"
	}
	fmt.Fprintf(&b, "%s:\n", label)
	limit := len(result.Files)
	if limit > 40 {
		limit = 40
	}
	for _, e := range result.Files[:limit] {
		suffix := ""
		if e.Type == "tree" {
			suffix = "/"
		}
		fmt.Fprintf(&b, "  %s%s\n", e.Name, suffix)
	}
	if len(result.Files) > limit {
		fmt.Fprintf(&b, "  ... %d more entries; use --full to show all.\n", len(result.Files)-limit)
	}
	if path == "" {
		readme := compactReadme(result.Readme, budget-b.Len())
		if readme == "" {
			readme = "(no README.md)"
		}
		fmt.Fprintf(&b, "\nREADME summary:\n%s\n", readme)
	}
	if b.Len() > budget {
		out := b.String()[:budget]
		fmt.Print(out)
		fmt.Println("\n→ output truncated by --budget; use --full to show complete explore output.")
		return
	}
	fmt.Print(b.String())
}

func compactReadme(readme string, budget int) string {
	readme = strings.TrimSpace(readme)
	if readme == "" {
		return ""
	}
	lines := strings.Split(readme, "\n")
	var kept []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && len(kept) > 0 {
			continue
		}
		kept = append(kept, line)
		if len(strings.Join(kept, "\n")) >= budget || len(kept) >= 80 {
			break
		}
	}
	out := strings.Join(kept, "\n")
	if len(out) > budget && budget > 0 {
		out = out[:budget]
	}
	if len(lines) > len(kept) || len(readme) > len(out) {
		out += "\n→ README truncated by --budget; use --full to show complete README."
	}
	return out
}

func buildSearchQuery(args []string, lang string, glob string) string {
	if len(args) == 1 {
		return appendSearchQualifiers(args[0], lang, glob)
	}
	query := fmt.Sprintf("%s repo:%s", quoteCodeSearchTerm(args[1]), args[0])
	return appendSearchQualifiers(query, lang, glob)
}

func buildGrepQuery(repo string, pattern string, glob string, path string) string {
	query := fmt.Sprintf("%s repo:%s", quoteCodeSearchTerm(pattern), repo)
	if path != "" {
		query += " path:" + path
	}
	return appendSearchQualifiers(query, "", glob)
}

func appendSearchQualifiers(query string, lang string, glob string) string {
	if lang != "" {
		query += " language:" + lang
	}
	if glob != "" {
		query += " path:" + glob
	}
	return query
}

func quoteCodeSearchTerm(term string) string {
	if strings.HasPrefix(term, "\"") && strings.HasSuffix(term, "\"") {
		return term
	}
	if strings.ContainsAny(term, " \t(){}[]:") {
		return strconv.Quote(term)
	}
	return term
}

var SkillMD string
var MCPSkillMD string
var ReconSkillMD string

func printSkill(filename string) error {
	switch filename {
	case "MCP-SKILL.md":
		fmt.Print(MCPSkillMD)
	case "RECON-SKILL.md":
		fmt.Print(ReconSkillMD)
	default:
		fmt.Print(SkillMD)
	}
	return nil
}
