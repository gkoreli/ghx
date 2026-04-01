package cmd

import (
	"fmt"
	"os"
	"strings"

	ghxlib "github.com/gkoreli/ghx/v2/pkg/ghx"
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
	RootCmd.AddCommand(reposCmd, exploreCmd, readCmd, searchCmd, treeCmd, skillCmd, versionCmd)
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
		fmt.Fprintf(os.Stderr, "%d repos found\n", total)
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

		results, err := ghxlib.Read(args[0], args[1:], ghxlib.ReadOpts{
			Grep:  grepPattern,
			Lines: lineRange,
			Map:   mapMode,
		})
		if err != nil {
			return err
		}

		for _, r := range results {
			if r.NotFound {
				fmt.Printf("=== %s (not found) ===\n\n", r.Path)
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
				mapOutput := strings.Join(r.MapLines, "\n")
				fmt.Printf("# map: %d/%d chars (~%d tokens full, ~%d tokens map)\n", len(mapOutput), r.MapChars, r.MapChars/4, len(mapOutput)/4)
			} else {
				fmt.Println(r.Content)
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	readCmd.Flags().String("grep", "", "Filter output to matching lines")
	readCmd.Flags().String("lines", "", "Extract specific line range (e.g., 42-80)")
	readCmd.Flags().Bool("map", false, "Structural signatures only")
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

		fmt.Fprintf(os.Stderr, "%d results (showing %d)\n", result.Total, len(result.Matches))
		if result.Incomplete {
			fmt.Fprintln(os.Stderr, "⚠ Results may be incomplete (query timed out)")
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
		mcpFlag, _ := cmd.Flags().GetBool("mcp")
		filename := "SKILL.md"
		if mcpFlag {
			filename = "MCP-SKILL.md"
		}
		return printSkill(filename)
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
	skillCmd.Flags().Bool("mcp", false, "Output MCP skill instead of CLI skill")
}

var SkillMD string
var MCPSkillMD string

func printSkill(filename string) error {
	if filename == "MCP-SKILL.md" {
		fmt.Print(MCPSkillMD)
	} else {
		fmt.Print(SkillMD)
	}
	return nil
}
