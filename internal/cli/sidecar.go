package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/spf13/cobra"
)

// sidecarCmd is the top-level "ghx sidecar" command group.
var sidecarCmd = &cobra.Command{
	Use:   "sidecar",
	Short: "Persistent specialist agent for GitHub repo reconnaissance",
	Long: `ghx sidecar — specialist agent that translates English repo questions into
bounded ghx evidence exploration and returns compact, auditable reports.

Use "ghx sidecar ask" to investigate a repo. Session state is persisted across
invocations so follow-up questions retain prior context.`,
	Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarAskCmd sends one investigation turn to the sidecar agent.
var sidecarAskCmd = &cobra.Command{
	Use:   "ask --session <name> --repo <owner/repo> <question>",
	Short: "Ask a repo question using the sidecar agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session, _ := cmd.Flags().GetString("session")
		repo, _ := cmd.Flags().GetString("repo")
		depth, _ := cmd.Flags().GetString("depth")
		jsonOut, _ := cmd.Flags().GetBool("json")

		if session == "" {
			return fmt.Errorf("--session is required")
		}
		if repo == "" {
			return fmt.Errorf("--repo is required")
		}

		cfg := sidecar.LoadConfig()
		report, err := sidecar.Ask(context.Background(), cfg, sidecar.AskRequest{
			Session:  session,
			Repo:     repo,
			Question: args[0],
			Depth:    depth,
		})
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		fmt.Println(report.Answer)
		return nil
	},
}

// sidecarDoctorCmd runs preflight diagnostics.
var sidecarDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run preflight diagnostics (token, network, ghx binary)",
	RunE: func(cmd *cobra.Command, args []string) error {
		result := sidecar.RunPreflight(context.Background())
		fmt.Print(sidecar.FormatPreflight(result))
		if !result.Passed {
			os.Exit(1)
		}
		return nil
	},
}

// sidecarSessionsCmd groups session management subcommands.
var sidecarSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage sidecar named sessions",
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarSessionsListCmd lists all sessions.
var sidecarSessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all named sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := sidecar.LoadConfig()
		metas, err := sidecar.ListSessions(cfg.SessionsDir)
		if err != nil {
			return err
		}
		if len(metas) == 0 {
			fmt.Println("(no sessions)")
			return nil
		}
		for _, m := range metas {
			fmt.Printf("%-24s  %-30s  turns=%-3d  %s\n", m.Name, m.Repo, m.TurnCount, m.UpdatedAt)
		}
		return nil
	},
}

// sidecarSessionsShowCmd prints metadata and report history for one session.
var sidecarSessionsShowCmd = &cobra.Command{
	Use:   "show <session-name>",
	Short: "Show details and report history for a named session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := sidecar.LoadConfig()
		name := args[0]
		meta, err := sidecar.ReadMeta(cfg.SessionsDir, name)
		if err != nil {
			return err
		}
		if meta == nil {
			return fmt.Errorf("session %q not found", name)
		}
		fmt.Printf("name:      %s\n", meta.Name)
		fmt.Printf("repo:      %s\n", meta.Repo)
		fmt.Printf("scope:     %s\n", meta.Scope)
		fmt.Printf("turns:     %d\n", meta.TurnCount)
		fmt.Printf("created:   %s\n", meta.CreatedAt)
		fmt.Printf("updated:   %s\n", meta.UpdatedAt)
		if meta.ACPSessionID != "" {
			fmt.Printf("acp-id:    %s\n", meta.ACPSessionID)
		}

		reports, err := sidecar.ListReports(cfg.SessionsDir, name)
		if err != nil {
			return err
		}
		if len(reports) == 0 {
			fmt.Println("\n(no saved reports)")
			return nil
		}
		fmt.Printf("\nReports (%d):\n", len(reports))
		for _, r := range reports {
			fmt.Printf("  %s\n", r)
		}
		return nil
	},
}

// sidecarConfigCmd groups config subcommands.
var sidecarConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sidecar configuration",
	Run:   func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarConfigShowCmd prints the current config.
var sidecarConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current sidecar configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := sidecar.LoadConfig()
		fmt.Println(sidecar.FormatConfig(cfg))
		return nil
	},
}

// sidecarConfigInitCmd auto-detects agents and writes initial config.
var sidecarConfigInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Auto-detect available ACP agents and write initial config",
	RunE: func(cmd *cobra.Command, args []string) error {
		found := sidecar.DetectAgents(context.Background())
		if len(found) == 0 {
			return fmt.Errorf("no ACP-compatible agents found on PATH (tried: claude, codex, kiro)")
		}
		cfg := sidecar.LoadConfig()
		cfg.AgentCmd = found[0]
		if err := sidecar.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Detected agents: %v\n", found)
		fmt.Printf("Using: %s\n\n", cfg.AgentCmd)
		fmt.Println(sidecar.FormatConfig(cfg))
		return nil
	},
}

func init() {
	sidecarAskCmd.Flags().String("session", "", "Named session (required)")
	sidecarAskCmd.Flags().String("repo", "", "GitHub repo owner/repo (required)")
	sidecarAskCmd.Flags().String("depth", "normal", "Command budget: cheap|normal|deep")
	sidecarAskCmd.Flags().Bool("json", false, "Output full report as JSON")

	sidecarSessionsCmd.AddCommand(sidecarSessionsListCmd, sidecarSessionsShowCmd)
	sidecarConfigCmd.AddCommand(sidecarConfigShowCmd, sidecarConfigInitCmd)
	sidecarCmd.AddCommand(sidecarAskCmd, sidecarDoctorCmd, sidecarSessionsCmd, sidecarConfigCmd)
}
