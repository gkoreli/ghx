package cli

import (
	"encoding/json"
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

var (
	tier2CodemapSpec = sidecar.DefaultToolSpec("codemap")
	tier2AstGrepSpec = sidecar.DefaultToolSpec("astgrep")
	tier2RepomapSpec = sidecar.DefaultToolSpec("repomap")
)

// tier2Snapshot materializes the snapshot for a tier2 subcommand and emits
// the visibility-contract stderr lines: eviction events (runtime log, not a
// report claim), then provenance — always before any structural tool output.
//
// Pre-hoc gating (ADR-0024.4 D2): when running inside a sidecar ask turn,
// GHX_TIER2_ALLOWED_BACKENDS carries the ask's grant; without a local grant
// the escalation is refused before any clone/tool work with an affordance
// error naming the correct next invocation. Unset env (direct human/agent
// CLI use outside an ask) is unchanged.
func tier2Snapshot(cmd *cobra.Command, svc *tier2.Service, repo string) (tier2.Snapshot, error) {
	if grant := os.Getenv(sidecar.Tier2GrantEnv); grant != "" && !sidecar.Tier2GrantAllowed(grant) {
		return tier2.Snapshot{}, WithExitCode(ExitBadInvocation, fmt.Errorf(
			"tier-2 local analysis is not granted for this ask (GHX_TIER2_ALLOWED_BACKENDS=%s)\n→ re-ask with --local to grant tier-2, or answer from remote evidence (`ghx explore %s`, `ghx inspect %s \"<concern>\"`)",
			grant, repo, repo))
	}
	ref, _ := cmd.Flags().GetString("ref")
	sparse, _ := cmd.Flags().GetStringSlice("sparse")
	snap, err := svc.Snapshot(cmd.Context(), tier2.SnapshotRequest{
		Repo: repo, Ref: ref, SparsePaths: sparse,
	})
	if err != nil {
		return snap, coreError(err)
	}
	for _, e := range snap.Evictions {
		fmt.Fprintf(os.Stderr, "# tier2 evicted: %s@%s reason=%s freed=%dB\n", e.Repo, shortSHA(e.SHA), e.Reason, e.FreedBytes)
	}
	for _, line := range snap.Provenance().Lines() {
		fmt.Fprintln(os.Stderr, "# "+line)
	}
	return snap, nil
}

// tier2BackendLine emits the backend/artifact stderr line that precedes tool
// output, e.g. "# tier2 backend: local:codemap (codemap:overview)".
func tier2BackendLine(backend, artifactKey string) {
	fmt.Fprintf(os.Stderr, "# tier2 backend: %s (%s)\n", backend, artifactKey)
}

// tier2SnapshotFlags registers the snapshot selection flags shared by every
// tier2 subcommand.
func tier2SnapshotFlags(cmd *cobra.Command) {
	cmd.Flags().String("ref", "", "Branch, tag, or full commit SHA (default: remote HEAD)")
	cmd.Flags().StringSlice("sparse", nil, "Sparse-checkout paths (candidate dirs from Tier-1 evidence)")
}

// tier2CodemapCmd runs the absorbed codemap backend (local:codemap) over a
// snapshot. codemap (github.com/JordanCoin/codemap, MIT) is an internal
// subprocess tool here — absorbed, with attribution (ADR-0026 watchlist).
var tier2CodemapCmd = &cobra.Command{
	Use:   "codemap <owner/repo>",
	Short: "Cross-file structure via the absorbed codemap tool (backend local:codemap)",
	Long: `Run the absorbed codemap tool over a cached snapshot for cross-file structure:
overview by default, --context for the JSON envelope, --importers for fan-in and
--deps for dependency flow (both need ast-grep on PATH). Snapshot provenance and
the local:codemap backend line print to stderr before any output. Exit 3 when
codemap is not installed (install hint included); no clone happens in that case.`,
	Example: `  ghx tier2 codemap honojs/hono
  ghx tier2 codemap gin-gonic/gin --importers gin.go
  ghx tier2 codemap openai/openai-node --ref next --context --compact
  ghx tier2 codemap gkoreli/ghx --deps
  ghx tier2 codemap gkoreli/ghx --sparse internal/sidecar --importers internal/sidecar/report.go`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return coreError(err)
		}

		snap, err := tier2Snapshot(cmd, svc, args[0])
		if err != nil {
			return err
		}
		tier2BackendLine(tier2CodemapSpec.BackendID, req.ArtifactKey())

		run, err := svc.RunCodemap(cmd.Context(), snap, req)
		if err != nil {
			// A failed tool invocation is evidence: surface exactly what ran
			// and what it said, then fail with the upstream code.
			if run.Stderr != "" {
				fmt.Fprint(os.Stderr, run.Stderr)
			}
			return coreError(err)
		}
		fmt.Print(run.Stdout)
		return nil
	},
}

// tier2AstGrepCmd runs the absorbed ast-grep backend (local:ast-grep) over a
// snapshot: structural AST pattern search. ast-grep
// (github.com/ast-grep/ast-grep, MIT) is an internal subprocess tool here —
// absorbed, with attribution (ADR-0026 watchlist). It is also the binary
// codemap needs for its --importers/--deps surfaces.
var tier2AstGrepCmd = &cobra.Command{
	Use:   "astgrep <owner/repo> [paths...]",
	Short: "Structural AST pattern search via the absorbed ast-grep tool (backend local:ast-grep)",
	Long: `Structural AST pattern search over a cached snapshot via the absorbed ast-grep
tool (backend local:ast-grep). Exit 1 with [] means zero matches (grep parity);
exit 3 when ast-grep is not installed. Patterns pass through to ast-grep verbatim.`,
	Example: `  ghx tier2 astgrep honojs/hono --pattern 'compose($$$ARGS)' --lang ts
  ghx tier2 astgrep gin-gonic/gin --pattern 'func ($R $T) ServeHTTP($$$) { $$$ }' --lang go
  ghx tier2 astgrep openai/openai-node --pattern 'new Stream($$$)' --lang ts src
  ghx tier2 astgrep gkoreli/ghx --pattern 'errors.Is($E, $T)' --lang go --sparse internal`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern, _ := cmd.Flags().GetString("pattern")
		lang, _ := cmd.Flags().GetString("lang")
		req := tier2.AstGrepRequest{Pattern: pattern, Lang: lang, Paths: args[1:]}

		svc := tier2.NewService(sidecar.RootDir(), VERSION)

		// Fail fast on a missing binary — before any clone work.
		if _, err := svc.AstGrep.Discover(); err != nil {
			if errors.Is(err, tier2.ErrAstGrepNotInstalled) {
				fmt.Fprintln(os.Stderr, tier2.AstGrepInstallHint)
			}
			return coreError(err)
		}

		snap, err := tier2Snapshot(cmd, svc, args[0])
		if err != nil {
			return err
		}
		tier2BackendLine(tier2AstGrepSpec.BackendID, req.ArtifactKey())

		run, err := svc.RunAstGrep(cmd.Context(), snap, req)
		if err != nil {
			if run.Stderr != "" {
				fmt.Fprint(os.Stderr, run.Stderr)
			}
			return coreError(err)
		}
		fmt.Print(run.Stdout)
		if !run.Matched() {
			// Grep-parity: the search ran fine and found nothing — a finding,
			// reported with the no-results exit code.
			return WithExitCode(ExitNoResults, fmt.Errorf("no ast-grep matches for pattern %q in %s", pattern, args[0]))
		}
		return nil
	},
}

// tier2RepomapCmd runs the repomap-style ranking backend (local:repomap):
// snapshot files ranked by definition/reference/import-graph centrality with
// a personalized-PageRank boost under a strict token budget. The algorithm is
// stolen from aider's repo map (github.com/Aider-AI/aider, Apache-2.0) with
// attribution; there is no aider dependency and no external binary — the
// ranking runs over artifacts ghx already owns.
var tier2RepomapCmd = &cobra.Command{
	Use:   "repomap <owner/repo>",
	Short: "Rank the files that matter via graph centrality under a token budget (backend local:repomap)",
	Long: `Rank the files that matter most for a concern: definition/reference/import-graph
centrality (repomap-style PageRank, algorithm credited to aider) under a strict
token budget — deterministic for a given snapshot. Backend local:repomap; no
external binary required.`,
	Example: `  ghx tier2 repomap honojs/hono
  ghx tier2 repomap gin-gonic/gin --query route --query middleware
  ghx tier2 repomap openai/openai-node --budget 512 --query streaming
  ghx tier2 repomap gkoreli/ghx --sparse internal/sidecar --query report`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query, _ := cmd.Flags().GetStringSlice("query")
		budget, _ := cmd.Flags().GetInt("budget")
		req := tier2.RepomapRequest{QueryTerms: query, BudgetTokens: budget}

		svc := tier2.NewService(sidecar.RootDir(), VERSION)
		snap, err := tier2Snapshot(cmd, svc, args[0])
		if err != nil {
			return err
		}
		tier2BackendLine(tier2RepomapSpec.BackendID, req.ArtifactKey())

		result, err := svc.RunRepomap(cmd.Context(), snap, req)
		if err != nil {
			return coreError(err)
		}
		fmt.Fprintln(os.Stderr, "# "+result.String())
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
		if len(result.Files) == 0 {
			return WithExitCode(ExitNoResults, fmt.Errorf("no rankable source files in %s", args[0]))
		}
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
	// tier2 observe (ADR-0024.4 D1): the sidecar agent's declaration surface
	// for judgment-based escalation signals. The command validates the signal
	// ID against the pre-registered policy set and prints a machine-readable
	// receipt; acceptance happens at decision time, from the recorded traces.
	var observeSignal string
	observeCmd := &cobra.Command{
		Use:   "observe",
		Short: "Declare a judgment-based escalation signal for this turn (ADR-0024.4 D1)",
		Long: `Declare one of the three semantic escalation signals the runtime cannot
derive itself: remote.symbol_absent_after_maps, remote.candidate_ambiguous,
remote.import_chain_invisible.

The declaration only counts when the observe invocation itself is recorded in
the turn's tool traces — acceptance is recomputed from wire evidence at
decision time, never from the report.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			valid := map[string]bool{
				tier2.SignalSymbolAbsentAfterMaps: true,
				tier2.SignalCandidateAmbiguous:    true,
				tier2.SignalImportChainInvisible:  true,
			}
			if !valid[observeSignal] {
				return WithExitCode(ExitBadInvocation, fmt.Errorf("unknown --signal %q; valid: %s, %s, %s",
					observeSignal, tier2.SignalSymbolAbsentAfterMaps, tier2.SignalCandidateAmbiguous, tier2.SignalImportChainInvisible))
			}
			fmt.Printf("observed	%s\n", observeSignal)
			return nil
		},
	}
	observeCmd.Flags().StringVar(&observeSignal, "signal", "", "Signal ID to declare (required)")
	_ = observeCmd.MarkFlagRequired("signal")
	tier2Cmd.AddCommand(observeCmd)

	tier2SnapshotFlags(tier2CodemapCmd)
	tier2CodemapCmd.Flags().Bool("context", false, "Emit the codemap JSON context envelope")
	tier2CodemapCmd.Flags().Bool("compact", false, "Token-minimal context envelope (with --context)")
	tier2CodemapCmd.Flags().String("importers", "", "Show importers (fan-in) of this file (needs ast-grep)")
	tier2CodemapCmd.Flags().Bool("deps", false, "Dependency flow: hub files and import chains (needs ast-grep)")
	tier2CodemapCmd.MarkFlagsMutuallyExclusive("context", "importers", "deps")
	tier2Cmd.AddCommand(tier2CodemapCmd)

	tier2SnapshotFlags(tier2AstGrepCmd)
	tier2AstGrepCmd.Flags().String("pattern", "", "ast-grep AST pattern, e.g. 'fmt.Println($A)' (required)")
	tier2AstGrepCmd.Flags().String("lang", "", "Language hint, e.g. go, ts, py (default: infer per file extension)")
	_ = tier2AstGrepCmd.MarkFlagRequired("pattern")
	tier2Cmd.AddCommand(tier2AstGrepCmd)

	tier2SnapshotFlags(tier2RepomapCmd)
	tier2RepomapCmd.Flags().StringSlice("query", nil, "Query terms biasing the ranking (repeatable)")
	tier2RepomapCmd.Flags().Int("budget", tier2.DefaultRepomapBudget, "Output budget in estimated tokens (~4 chars/token)")
	tier2Cmd.AddCommand(tier2RepomapCmd)
}
