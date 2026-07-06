package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"

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
	Use:   "ask [--repo <owner/repo>] <question>",
	Short: "Ask a repo question using the sidecar agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session, _ := cmd.Flags().GetString("session")
		repo, _ := cmd.Flags().GetString("repo")
		depth, _ := cmd.Flags().GetString("depth")
		jsonOut, _ := cmd.Flags().GetBool("json")

		// Session naming (ADR-0019.1 D2): --session wins; with --repo the
		// repo slug stands; with neither, derive a stable slug from the
		// question. The chosen name is printed so the caller can resume it.
		if session == "" {
			if repo != "" {
				session = defaultReconSession(repo)
			} else {
				session = questionSession(args[0])
			}
		}
		fmt.Fprintf(os.Stderr, "session: %s\n", session)

		cfg := sidecar.LoadConfig()
		report, turn, err := sidecar.Ask(context.Background(), cfg, sidecar.AskRequest{
			Session:  session,
			Repo:     repo,
			Question: args[0],
			Depth:    depth,
		})
		if err != nil {
			return err
		}

		artifacts := turn.Artifacts
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(askEnvelope{Report: report, Artifacts: artifacts})
		}
		printHumanReport(report)
		if footer := artifacts.FooterLine(); footer != "" {
			fmt.Printf("\n%s\n", footer)
		}
		return nil
	},
}

// askEnvelope is the `ghx sidecar ask --json` output shape: the validated
// report unchanged under "report", plus the artifacts pointer (session dir +
// root trace ID) as a sibling — the report schema (ADR-0021) itself stays
// untouched, and the caller never has to guess where the audit trail lives.
type askEnvelope struct {
	Report    *sidecar.Report      `json:"report"`
	Artifacts sidecar.ArtifactsRef `json:"artifacts"`
}

// sidecarReportSinkCmd is a hidden, internal command: it serves the
// report-sink MCP server over stdio, exposing exactly the submit_report tool
// (ADR-0021 D1). The sidecar runtime spawns it as a session-scoped MCP server
// (via ACP NewSessionRequest.McpServers) so the agent submits its final report
// through a strictly-validated tool call instead of a free-text block. It is
// not meant to be run by hand.
var sidecarReportSinkCmd = &cobra.Command{
	Use:    "report-sink --out <path>",
	Short:  "Internal: serve the submit_report MCP tool over stdio",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		if out == "" {
			return fmt.Errorf("--out is required")
		}
		return sidecar.RunReportSink(out)
	},
}

// sidecarDoctorCmd runs preflight diagnostics.
var sidecarDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run preflight diagnostics (token, network, ghx binary, ACP agent, report sink)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := sidecar.LoadConfig()
		// Show what the config resolves to before probing it, so a failing
		// handshake or sink check is immediately attributable.
		fmt.Printf("Agent command: %s\n", cfg.AgentCmd)
		fmt.Printf("Config file:   %s\n\n", sidecar.ConfigFilePath())
		result := sidecar.RunPreflight(context.Background(), VERSION)
		fmt.Print(sidecar.FormatPreflight(result))
		fmt.Println()
		fmt.Println(sidecar.ArtifactsHint(cfg))
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

// sidecarSessionsLedgerCmd prints the persisted evidence ledger for one session.
var sidecarSessionsLedgerCmd = &cobra.Command{
	Use:   "ledger <session-name>",
	Short: "Print the evidence ledger for a named session",
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
		ledger, err := sidecar.LoadLedger(cfg.SessionsDir, name)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(ledger)
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

// sidecarConfigInitCmd writes the initial config: either the pinned Claude ACP
// adapter (--claude-acp, the documented one-command setup) or PATH
// auto-detection (the original behavior, unchanged).
var sidecarConfigInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write initial config (--claude-acp for the pinned Claude ACP adapter, else auto-detect)",
	RunE: func(cmd *cobra.Command, args []string) error {
		claudeACP, _ := cmd.Flags().GetBool("claude-acp")
		force, _ := cmd.Flags().GetBool("force")
		return runSidecarConfigInit(claudeACP, force)
	},
}

func runSidecarConfigInit(claudeACP, force bool) error {
	if claudeACP {
		return initClaudeACPConfig(force)
	}
	found := sidecar.DetectAgents(context.Background())
	if len(found) == 0 {
		return fmt.Errorf("no ACP-compatible agents found on PATH (tried: claude, codex, kiro); " +
			"run `ghx sidecar config init --claude-acp` to configure the pinned Claude ACP adapter (needs Node/npx)")
	}
	// Start from the fresh default config (rooted at ~/.ghx or $GHX_HOME,
	// ADR-0022 D3) and carry over any existing model/visibility settings.
	existing := sidecar.LoadConfig()
	cfg := sidecar.NewDefaultConfig()
	cfg.Model = existing.Model
	cfg.Visibility = existing.Visibility
	cfg.AgentCmd = found[0]
	if err := sidecar.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Detected agents: %v\n", found)
	fmt.Printf("Using: %s\n\n", cfg.AgentCmd)
	fmt.Println(sidecar.FormatConfig(cfg))
	return nil
}

// initClaudeACPConfig writes the pinned Claude ACP adapter command line
// (sidecar.ClaudeACPAgentCmd) into the config, carrying over any existing
// model/visibility settings. An existing config is never overwritten without
// --force; the field diff of what would change is always shown first.
func initClaudeACPConfig(force bool) error {
	existing := sidecar.LoadConfig()
	cfg := sidecar.NewDefaultConfig()
	cfg.Model = existing.Model
	cfg.Visibility = existing.Visibility
	cfg.AgentCmd = sidecar.ClaudeACPAgentCmd
	if path, exists := sidecar.ConfigFileExists(); exists {
		diff := sidecar.DiffConfigs(existing, cfg)
		if len(diff) == 0 {
			fmt.Printf("Config at %s already uses the Claude ACP adapter — nothing to change.\n\n", path)
			fmt.Println(sidecar.FormatConfig(cfg))
			return nil
		}
		fmt.Printf("Config already exists at %s — this would change:\n", path)
		for _, line := range diff {
			fmt.Println(line)
		}
		if !force {
			return fmt.Errorf("refusing to overwrite existing config %s; re-run with --force to apply the change above", path)
		}
		fmt.Println()
	}
	if err := sidecar.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Println("Wrote Claude ACP adapter config.")
	fmt.Println(sidecar.FormatConfig(cfg))
	fmt.Println()
	fmt.Println("Next: `ghx sidecar doctor` to verify the setup (needs Node/npx and a Claude Code login),")
	fmt.Println("then `ghx sidecar ask --repo <owner/repo> \"<question>\"`.")
	return nil
}

func init() {
	sidecarAskCmd.Flags().String("session", "", "Named session (default: repo slug, or a question-derived slug without --repo)")
	sidecarAskCmd.Flags().String("repo", "", "GitHub repo owner/repo (optional scope; omit for cross-GitHub discovery)")
	sidecarAskCmd.Flags().String("depth", "normal", "Command budget: cheap|normal|deep")
	sidecarAskCmd.Flags().Bool("json", false, "Output full report as JSON")

	sidecarReportSinkCmd.Flags().String("out", "", "Path to write the accepted report JSON (required)")

	sidecarConfigInitCmd.Flags().Bool("claude-acp", false, "Write the pinned Claude ACP adapter command (npx @agentclientprotocol/claude-agent-acp) instead of auto-detecting")
	sidecarConfigInitCmd.Flags().Bool("force", false, "Overwrite an existing config (only with --claude-acp; a diff is shown first)")

	sidecarSessionsCmd.AddCommand(sidecarSessionsListCmd, sidecarSessionsShowCmd, sidecarSessionsLedgerCmd)
	sidecarConfigCmd.AddCommand(sidecarConfigShowCmd, sidecarConfigInitCmd)
	sidecarCmd.AddCommand(sidecarAskCmd, sidecarDoctorCmd, sidecarReportSinkCmd, sidecarSessionsCmd, sidecarConfigCmd)
}

// questionSession derives a stable session slug from the question when the ask
// has neither --session nor --repo (discovery mode, ADR-0019.1 D2): the
// kebab-cased leading words truncated to ~40 chars, plus a short content hash
// of the full question so distinct questions never collide.
func questionSession(question string) string {
	slug := kebabSlug(question)
	if runes := []rune(slug); len(runes) > 40 {
		slug = string(runes[:40])
		// Prefer whole leading words: drop a trailing partial word when the
		// cut landed mid-word (keep single over-long words as-is).
		if i := strings.LastIndex(slug, "-"); i > 0 {
			slug = slug[:i]
		}
	}
	if slug == "" {
		slug = "discovery"
	}
	sum := sha256.Sum256([]byte(question))
	return slug + "-" + hex.EncodeToString(sum[:4])
}

func defaultReconSession(repo string) string {
	s := kebabSlug(repo)
	if s == "" {
		return "repo"
	}
	return s
}

// kebabSlug lowercases s and collapses every non-alphanumeric run into a
// single dash, trimming leading/trailing dashes.
func kebabSlug(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func printHumanReport(report *sidecar.Report) {
	fmt.Println(report.Answer)
	printClaims("Verified", report.Verified)
	printRelevantFiles(report.RelevantFiles)
	printStrings("Uncertainty", report.Uncertainty)
}

func printClaims(title string, claims []sidecar.Claim) {
	if len(claims) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, c := range claims {
		if c.Evidence != "" {
			fmt.Printf("- %s — %s\n", c.Summary, c.Evidence)
		} else {
			fmt.Printf("- %s\n", c.Summary)
		}
	}
}

func printRelevantFiles(files []sidecar.RelevantFile) {
	if len(files) == 0 {
		return
	}
	fmt.Println("\nRelevant files:")
	for _, f := range files {
		if f.Reason != "" {
			fmt.Printf("- %s — %s\n", f.Path, f.Reason)
		} else {
			fmt.Printf("- %s\n", f.Path)
		}
	}
}

func printStrings(title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Printf("\n%s:\n", title)
	for _, item := range items {
		fmt.Printf("- %s\n", item)
	}
}
