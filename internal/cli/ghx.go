package cli

import (
	"fmt"
	"os"
	"strings"

	ghxlib "github.com/gkoreli/ghx/v2/internal/ghx"
	"github.com/spf13/cobra"
)

var VERSION = "dev"

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
	RootCmd.AddCommand(reposCmd, exploreCmd, readCmd, searchCmd, treeCmd, skillCmd, versionCmd, sidecarCmd)
}

var reposCmd = &cobra.Command{
	Use:   "repos <query> [--limit N]",
	Short: "Search repos with README preview",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		results, total, err := ghxlib.Repos(args[0], ghxlib.ReposOpts{Limit: limit})
		if err != nil {
			return err
		}
		fmt.Printf("%d repos found\n", total)
		if len(results) == 0 {
			os.Exit(1)
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
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ""
		if len(args) > 1 {
			path = args[1]
		}
		result, err := ghxlib.Explore(args[0], path)
		if err != nil {
			return err
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

var readCmd = &cobra.Command{
	Use:   "read <owner/repo> <path1> [path2...]",
	Short: "Read 1-10 files in 1 API call",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		grepPattern, _ := cmd.Flags().GetString("grep")
		lineRange, _ := cmd.Flags().GetString("lines")
		mapMode, _ := cmd.Flags().GetBool("map")
		mapLevel, _ := cmd.Flags().GetString("level")
		mapKind, _ := cmd.Flags().GetString("kind")
		mapEngine, _ := cmd.Flags().GetString("map-engine")

		opts := &ghxlib.ReadOpts{
			Grep:      grepPattern,
			Lines:     lineRange,
			Map:       mapMode,
			MapLevel:  mapLevel,
			MapKind:   mapKind,
			MapEngine: mapEngine,
		}
		results, err := ghxlib.Read(args[0], args[1:], opts)
		if err != nil {
			return err
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
	readCmd.Flags().Bool("map", false, "Structural signatures only")
	readCmd.Flags().String("level", "compact", "Map detail level: outline|minimal|compact|standard")
	readCmd.Flags().String("kind", "", "Map symbol kind filter: func|type|import|const|var|package")
	readCmd.Flags().String("map-engine", "auto", "Map engine: auto|regex|tree-sitter")
}

var searchCmd = &cobra.Command{
	Use:   "search <query> [--limit N] [--full]",
	Short: "Code search (AND matching, matching context)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		limit, _ := cmd.Flags().GetInt("limit")
		fullMode, _ := cmd.Flags().GetBool("full")

		result, err := ghxlib.Search(args[0], ghxlib.SearchOpts{
			Limit:    limit,
			FullMode: fullMode,
		})
		if err != nil {
			return err
		}

		fmt.Printf("%d results (showing %d)\n", result.Total, len(result.Matches))
		if result.Incomplete {
			fmt.Println("⚠ Results may be incomplete (query timed out)")
		}

		if len(result.Matches) == 0 {
			fmt.Println("→ To search repos by topic, use: ghx repos \"<query>\"")
			os.Exit(1)
		}

		for _, m := range result.Matches {
			fmt.Printf("%s %s: %s\n", m.Repo, m.Path, m.Fragment)
		}
		return nil
	},
}

func init() {
	searchCmd.Flags().IntP("limit", "l", 30, "Number of results")
	searchCmd.Flags().Bool("full", false, "Show complete fragments without truncation")
}

var treeCmd = &cobra.Command{
	Use:   "tree <owner/repo> [path]",
	Short: "Full recursive tree listing",
	Args:  cobra.RangeArgs(1, 2),
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
			return err
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
	RunE: func(cmd *cobra.Command, args []string) error {
		return printSkill()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ghx %s\n", VERSION)
	},
}

func init() {
	skillCmd.Flags().Bool("mcp", false, "Compatibility alias; output the same canonical skill")
}

var SkillMD string

func printSkill() error {
	fmt.Print(SkillMD)
	return nil
}
