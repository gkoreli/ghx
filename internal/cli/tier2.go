package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
	"github.com/spf13/cobra"
)

// tier2Cmd groups Tier-2 local structural analysis commands (ADR-0024.1).
// Tier 2 is the deliberate, visible escalation beyond remote evidence: a
// shallow blobless snapshot is cached under ~/.ghx and absorbed structural
// tools run over it. Every invocation emits snapshot provenance (repo, ref,
// resolved SHA, clone strategy, cache hit) on stderr before any tool output.
var tier2Cmd = &cobra.Command{
	Use:   "tier2",
	Short: "Local structural analysis over a cached repo snapshot",
	Long: `Tier-2 local structural analysis (ADR-0024.1, NORTH_STAR P3).

Resolves repo/ref to a commit SHA, materializes a read-only snapshot under
~/.ghx/cache/tier2 (shallow, blobless, cached by SHA; $GHX_HOME respected),
and runs absorbed structural tools over it. Provenance is printed to stderr
before any tool runs; tool output goes to stdout.`,
}

// tier2CodemapCmd runs the absorbed codemap backend (local:codemap) over a
// snapshot. codemap (github.com/JordanCoin/codemap, MIT) is an internal
// subprocess tool here — absorbed, with attribution (ADR-0026 watchlist).
var tier2CodemapCmd = &cobra.Command{
	Use:   "codemap <owner/repo>",
	Short: "Cross-file structure via the absorbed codemap tool (backend local:codemap)",
	Example: `  ghx tier2 codemap honojs/hono
  ghx tier2 codemap gin-gonic/gin --importers gin.go
  ghx tier2 codemap openai/openai-node --ref next --context --compact
  ghx tier2 codemap gkoreli/ghx --deps
  ghx tier2 codemap gkoreli/ghx --sparse internal/sidecar --importers internal/sidecar/report.go`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ref, _ := cmd.Flags().GetString("ref")
		sparse, _ := cmd.Flags().GetStringSlice("sparse")
		contextMode, _ := cmd.Flags().GetBool("context")
		compact, _ := cmd.Flags().GetBool("compact")
		importers, _ := cmd.Flags().GetString("importers")
		deps, _ := cmd.Flags().GetBool("deps")

		req := tier2.CodemapRequest{Mode: tier2.CodemapOverview}
		switch {
		case contextMode:
			req = tier2.CodemapRequest{Mode: tier2.CodemapContext, Compact: compact}
		case importers != "":
			req = tier2.CodemapRequest{Mode: tier2.CodemapImporters, File: importers}
		case deps:
			req = tier2.CodemapRequest{Mode: tier2.CodemapDeps}
		}
		if compact && !contextMode {
			return WithExitCode(ExitBadInvocation, fmt.Errorf("--compact only applies to --context"))
		}

		svc := tier2.NewService(sidecar.RootDir(), VERSION)

		// Fail fast on a missing binary — before any clone work. The absence
		// is a graceful tier fallback, never a fake answer: name the blocked
		// backend and how to unblock it.
		if _, err := svc.Codemap.Discover(); err != nil {
			if errors.Is(err, tier2.ErrCodemapNotInstalled) {
				fmt.Fprintln(os.Stderr, tier2.CodemapInstallHint)
			}
			return WithExitCode(ExitUpstreamFailure, err)
		}

		snap, err := svc.Snapshot(cmd.Context(), tier2.SnapshotRequest{
			Repo: args[0], Ref: ref, SparsePaths: sparse,
		})
		if err != nil {
			return WithExitCode(ExitUpstreamFailure, err)
		}

		// Visibility contract: provenance before any structural tool runs.
		// Eviction is a runtime event, not a report claim — stderr, not stdout.
		for _, e := range snap.Evictions {
			fmt.Fprintf(os.Stderr, "# tier2 evicted: %s@%s reason=%s freed=%dB\n", e.Repo, shortSHA(e.SHA), e.Reason, e.FreedBytes)
		}
		for _, line := range snap.Provenance().Lines() {
			fmt.Fprintln(os.Stderr, "# "+line)
		}
		fmt.Fprintf(os.Stderr, "# tier2 backend: %s (%s)\n", tier2.BackendCodemap, req.ArtifactKey())

		run, err := svc.RunCodemap(cmd.Context(), snap, req)
		if err != nil {
			// A failed tool invocation is evidence: surface exactly what ran
			// and what it said, then fail with the upstream code.
			if run.Stderr != "" {
				fmt.Fprint(os.Stderr, run.Stderr)
			}
			return WithExitCode(ExitUpstreamFailure, err)
		}
		fmt.Print(run.Stdout)
		return nil
	},
}

// shortSHA abbreviates a commit SHA for log lines.
func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func init() {
	tier2CodemapCmd.Flags().String("ref", "", "Branch, tag, or full commit SHA (default: remote HEAD)")
	tier2CodemapCmd.Flags().StringSlice("sparse", nil, "Sparse-checkout paths (candidate dirs from Tier-1 evidence)")
	tier2CodemapCmd.Flags().Bool("context", false, "Emit the codemap JSON context envelope")
	tier2CodemapCmd.Flags().Bool("compact", false, "Token-minimal context envelope (with --context)")
	tier2CodemapCmd.Flags().String("importers", "", "Show importers (fan-in) of this file")
	tier2CodemapCmd.Flags().Bool("deps", false, "Dependency flow: hub files and import chains (needs ast-grep)")
	tier2CodemapCmd.MarkFlagsMutuallyExclusive("context", "importers", "deps")
	tier2Cmd.AddCommand(tier2CodemapCmd)
}
