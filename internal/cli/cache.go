package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
	"github.com/spf13/cobra"
)

// cacheCmd groups the tier2 snapshot-cache user surface (ADR-0024.4 D4):
// list what is cached and clean it. The cache normally manages itself via
// the pre-registered TTL+LRU eviction policy (ADR-0024.1); these commands
// exist for visibility and manual recovery, not routine use.
var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Inspect and clean the tier2 snapshot cache",
	Long: `Inspect and clean the tier2 local-analysis snapshot cache (~/.ghx/cache/tier2,
$GHX_HOME respected; ADR-0024.4 D4).

The cache self-manages via the pre-registered eviction policy (TTL 30 days,
5 GiB LRU budget, ADR-0024.1). Use ` + "`ghx cache ls`" + ` to see what is cached
and ` + "`ghx cache clean`" + ` to remove everything manually.`,
}

var cacheLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List cached tier2 snapshots (oldest-access first — the eviction order)",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOut, _ := cmd.Flags().GetBool("json")
		cache := tier2.NewCache(sidecar.RootDir())
		infos, err := cache.ListSnapshots()
		if err != nil {
			return err
		}
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(infos)
		}
		if len(infos) == 0 {
			fmt.Println("tier2 snapshot cache is empty")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "REPO\tSHA\tREF\tSTRATEGY\tSIZE\tLAST ACCESSED\tPATH")
		var total int64
		for _, in := range infos {
			total += in.SizeBytes
			ref := in.Ref
			if ref == "" {
				ref = "(default)"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d B\t%s\t%s\n",
				in.Repo, shortSHA(in.SHA), ref, in.Strategy, in.SizeBytes,
				in.LastAccessedAt.UTC().Format("2006-01-02 15:04"), in.Path)
		}
		w.Flush()
		fmt.Printf("\n%d snapshot(s), %d bytes total\n", len(infos), total)
		return nil
	},
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all cached tier2 snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes {
			return WithExitCode(ExitBadInvocation, fmt.Errorf("refusing to clean the whole cache without --yes"))
		}
		cache := tier2.NewCache(sidecar.RootDir())
		events, err := cache.Clean()
		if err != nil {
			return err
		}
		var freed int64
		for _, e := range events {
			freed += e.FreedBytes
			fmt.Printf("removed %s @ %s (%d B)\n", e.Repo, shortSHA(e.SHA), e.FreedBytes)
		}
		fmt.Printf("\ncleaned %d snapshot(s), %d bytes freed\n", len(events), freed)
		return nil
	},
}

func init() {
	cacheLsCmd.Flags().Bool("json", false, "Output the snapshot list as JSON")
	cacheCleanCmd.Flags().Bool("yes", false, "Confirm removal of every cached snapshot")
	cacheCmd.AddCommand(cacheLsCmd, cacheCleanCmd)
	RootCmd.AddCommand(cacheCmd)
}
