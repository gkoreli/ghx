package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/evals"
	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
	"github.com/spf13/cobra"
)

// askAllowedBackends translates the ask command's --local flag into the
// request's backend allowlist. Default is remote-only: tier-2 local analysis
// (clone, codemap, ast-grep, repomap) never runs unless the caller grants it,
// because escalation is a recorded policy decision, not an agent whim
// (ADR-0024.2). With the grant, the escalation policy engine may allow
// `ghx tier2` commands and the decision lands in the session's
// tier-decisions.jsonl.
func askAllowedBackends(local bool) []string {
	if local {
		return []string{"remote", tier2.LocalBackendGrant}
	}
	return nil
}

// sidecarCmd is the top-level "ghx sidecar" command group.
var sidecarCmd = &cobra.Command{
	Use:   "sidecar",
	Short: "Persistent specialist agent for GitHub repo reconnaissance",
	Long: `ghx sidecar — specialist agent that translates English repo questions into
bounded ghx evidence exploration and returns compact, auditable reports.

Use "ghx sidecar ask" to investigate a repo. Session state is persisted across
invocations so follow-up questions retain prior context.`,
	Example: `  ghx sidecar doctor
  ghx sidecar ask --repo hono/hono "How is middleware chained?"
  ghx sidecar sessions list`,
	Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarAskCmd sends one investigation turn to the sidecar agent.
var sidecarAskCmd = &cobra.Command{
	Use:   "ask [--repo <owner/repo>] <question>",
	Short: "Ask a repo question using the sidecar agent",
	Long: `Delegate a whole reconnaissance question in plain English and get back a compact,
auditable evidence report — state the goal, not the exploration steps. Pass
` + "`--repo owner/repo`" + ` to scope to one repo; omit it for discovery ("which repos do
X"), where the sidecar sweeps GitHub and verifies candidates before ranking. The
chosen session name is printed to stderr and reused by follow-ups, so later asks
are faster and context-aware; ` + "`--json`" + ` returns the {report, artifacts} envelope
instead of the human summary. Every answer ends with an artifacts footer —
` + "`artifacts: <session dir> (trace <id>)`" + ` — pointing at the on-disk trace, logs,
and reports under ~/.ghx that back the report.`,
	Example: `  ghx sidecar ask --repo hono/hono "How is middleware chained?"
  ghx sidecar ask --repo gkoreli/ghx --depth deep "Where are sidecar reports validated?"
  ghx sidecar ask --repo golang/go --local "Which packages import internal/abi, and through what chains?"
  ghx sidecar ask --json "Which Go repos implement ACP agents?"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		session, _ := cmd.Flags().GetString("session")
		repo, _ := cmd.Flags().GetString("repo")
		depth, _ := cmd.Flags().GetString("depth")
		jsonOut, _ := cmd.Flags().GetBool("json")
		local, _ := cmd.Flags().GetBool("local")

		// Session routing (ADR-0030.1): --session and --repo keep the exact
		// ADR-0019.1 D2 behavior (R1/R2 — the name is deterministic, printed
		// up front as before). With neither, the daemon routes the question
		// through the R3-R5 cascade and the decision is printed after the
		// ask, with its provenance.
		routedByDaemon := session == "" && repo == ""
		if !routedByDaemon {
			name := session
			if name == "" {
				name = defaultReconSession(repo)
			}
			fmt.Fprintf(os.Stderr, "session: %s\n", name)
		}

		cfg := sidecar.LoadConfig()
		report, turn, _, err := sidecar.AskViaDaemon(context.Background(), VERSION, cfg, sidecar.AskRequest{
			Session:         session,
			Repo:            repo,
			Question:        args[0],
			Depth:           depth,
			AllowedBackends: askAllowedBackends(local),
		})
		if err != nil {
			return err
		}

		var route *sidecar.RouteDecision
		if routedByDaemon && turn != nil {
			route = turn.Route
		}
		if route != nil {
			fmt.Fprintf(os.Stderr, "%s\n", route.Line())
		}

		artifacts := turn.Artifacts
		if jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(askEnvelope{Report: report, Artifacts: artifacts, Route: route})
		}
		printHumanReport(report)
		if route != nil {
			fmt.Printf("\n%s\n", route.Line())
		}
		if footer := artifacts.FooterLine(); footer != "" {
			if route != nil {
				fmt.Printf("%s\n", footer)
			} else {
				fmt.Printf("\n%s\n", footer)
			}
		}
		return nil
	},
}

// sidecarDaemonCmd runs or controls the per-user sidecar daemon.
var sidecarDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run the warm sidecar daemon",
	Long: `Run the per-user warm daemon that keeps the ACP agent resident so ` + "`ask`" + ` turns
skip cold-start latency. You normally never run this by hand — ` + "`ask`" + ` auto-spawns
the daemon on demand — but it is useful for pre-warming or debugging. Run in the
foreground to watch its logs; ` + "`--stop`" + ` asks a running daemon to shut down cleanly.`,
	Example: `  ghx sidecar daemon
  ghx sidecar daemon --stop`,
	RunE: func(cmd *cobra.Command, args []string) error {
		stop, _ := cmd.Flags().GetBool("stop")
		if stop {
			return sidecar.ShutdownDaemon(context.Background(), VERSION, sidecar.LoadConfig())
		}
		cfg := sidecar.LoadConfig()
		return sidecar.RunDaemon(context.Background(), VERSION, cfg)
	},
}

// askEnvelope is the `ghx sidecar ask --json` output shape: the validated
// report unchanged under "report", plus the artifacts pointer (session dir +
// root trace ID) as a sibling — the report schema (ADR-0021) itself stays
// untouched, and the caller never has to guess where the audit trail lives.
type askEnvelope struct {
	Report    *sidecar.Report      `json:"report"`
	Artifacts sidecar.ArtifactsRef `json:"artifacts"`
	// Route carries the daemon's routing decision (ADR-0030.1 D7) when the
	// ask had neither --session nor --repo; omitted otherwise so the
	// flag-scoped envelope stays byte-identical to pre-routing behavior.
	Route *sidecar.RouteDecision `json:"route,omitempty"`
}

// sidecarReportSinkCmd is a hidden, internal command: it serves the
// report-sink MCP server over stdio, exposing exactly the submit_report tool
// (ADR-0021 D1). The sidecar runtime spawns it as a session-scoped MCP server
// (via ACP NewSessionRequest.McpServers) so the agent submits its final report
// through a strictly-validated tool call instead of a free-text block. It is
// not meant to be run by hand.
var sidecarReportSinkCmd = &cobra.Command{
	Use:     "report-sink --out <path>",
	Short:   "Internal: serve the submit_report MCP tool over stdio",
	Example: `  ghx sidecar report-sink --out /tmp/report.json`,
	Hidden:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, _ := cmd.Flags().GetString("out")
		if out == "" {
			return fmt.Errorf("--out is required")
		}
		return sidecar.RunReportSink(out)
	},
}

// sidecarEvalsCmd groups hidden eval-maintenance commands. These are
// automation surfaces for committed artifacts, not end-user sidecar commands.
var sidecarEvalsCmd = &cobra.Command{
	Use:    "evals",
	Short:  "Internal: sidecar eval artifact tools",
	Hidden: true,
	Run:    func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarEvalsExportCmd converts committed eval episodes to training records.
var sidecarEvalsExportCmd = &cobra.Command{
	Use:    "export --format sft --run <dir> --out <file>",
	Short:  "Internal: export sidecar eval episodes as training JSONL",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")
		runDir, _ := cmd.Flags().GetString("run")
		out, _ := cmd.Flags().GetString("out")
		if format != "sft" {
			return fmt.Errorf("--format must be sft")
		}
		if runDir == "" {
			return fmt.Errorf("--run is required")
		}
		if out == "" {
			return fmt.Errorf("--out is required")
		}
		manifest, err := evals.ExportSFT(evals.SFTExportOptions{
			RunDir:      runDir,
			OutPath:     out,
			RewardFloor: evals.DefaultSFTRewardFloor,
		})
		if err != nil {
			return err
		}
		fmt.Print(evals.FormatSFTExportSummary(manifest))
		return nil
	},
}

// sidecarDoctorCmd runs preflight diagnostics.
var sidecarDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run preflight diagnostics (token, network, ghx binary, ACP agent, report sink)",
	Long: `Verify the sidecar is ready to run: GitHub token, network reachability, the ghx
binary, the configured ACP agent handshake, and the report sink. Run this right
after ` + "`config init`" + ` and whenever an ` + "`ask`" + ` fails to set up — it prints the resolved
agent command and config path first, so a failing check is immediately
attributable, and ends with the ~/.ghx artifacts location. Exit code 3 if any
check fails.`,
	Example: `  ghx sidecar doctor
  ghx sidecar doctor --live`,
	RunE: func(cmd *cobra.Command, args []string) error {
		live, _ := cmd.Flags().GetBool("live")
		cfg := sidecar.LoadConfig()
		// Show what the config resolves to before probing it, so a failing
		// handshake or sink check is immediately attributable.
		fmt.Printf("Agent command: %s\n", cfg.AgentCmd)
		fmt.Printf("Config file:   %s\n\n", sidecar.ConfigFilePath())
		result := sidecar.RunPreflight(context.Background(), VERSION)
		fmt.Print(sidecar.FormatPreflight(result))
		passed := result.Passed
		// --live runs a real session/new + one-prompt turn through the
		// configured agent (ADR-0033 D4): the deterministic diagnosis for
		// setups where ACP initialize passes but a real turn dies (auth,
		// proxy/CA, adapter/Node skew). Slow and side-effecting, hence opt-in.
		if live {
			fmt.Println("\nRunning live prompt turn (this spawns the real agent)…")
			liveCheck := sidecar.CheckLiveTurn(context.Background(), cfg)
			fmt.Print(sidecar.FormatPreflight(sidecar.PreflightResult{Passed: liveCheck.Passed, Checks: []sidecar.PreflightCheck{liveCheck}}))
			passed = passed && liveCheck.Passed
		}
		fmt.Println()
		fmt.Println(sidecar.ArtifactsHint(cfg))
		if !passed {
			return WithExitCode(ExitUpstreamFailure, fmt.Errorf("sidecar preflight failed"))
		}
		return nil
	},
}

// sidecarSessionsCmd groups session management subcommands.
var sidecarSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "Manage sidecar named sessions",
	Long: `Inspect the named sessions the sidecar persists under ~/.ghx/sessions. Each
session accumulates report history and an evidence ledger across ` + "`ask`" + ` turns, so
these subcommands let you audit what a past investigation found and which session
to resume. Use ` + "`list`" + ` to enumerate, ` + "`show`" + ` for details and report history, and
` + "`ledger`" + ` for the raw accumulated evidence.`,
	Example: `  ghx sidecar sessions list
  ghx sidecar sessions show hono-hono
  ghx sidecar sessions ledger hono-hono`,
	Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarSessionsListCmd lists all sessions.
var sidecarSessionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all named sessions",
	Long: `List every persisted session with its repo, turn count, and last-updated time,
one per line. Use it to find the session name to pass to ` + "`show`, `ledger`, or `view`" + `.
Prints "(no sessions)" when none exist yet.`,
	Example: `  ghx sidecar sessions list`,
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
	Long: `Print one session's metadata — repo, scope, turn count, timestamps, ACP session
id — followed by its saved report files. Use it after ` + "`sessions list`" + ` to inspect
what a past investigation produced before resuming or replaying it. The session
name is the slug from ` + "`list`" + ` (a repo slug or a question-derived slug).`,
	Example: `  ghx sidecar sessions show hono-hono
  ghx sidecar sessions show discovery-which-go-repos-implement-acp`,
	Args: cobra.ExactArgs(1),
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
		// Agent provenance (ADR-0033 D5): the workspace, the agent command, the
		// spawn cwd (the daemon inherits the first caller's), and the NAMES of
		// agent-relevant env vars present at creation — never values. This is
		// the surface for diagnosing an environment-specific failure.
		if meta.AgentCmd != "" {
			fmt.Printf("agent:     %s\n", meta.AgentCmd)
		}
		if meta.Cwd != "" {
			fmt.Printf("cwd:       %s\n", meta.Cwd)
		}
		if meta.SpawnCwd != "" {
			fmt.Printf("spawn-cwd: %s\n", meta.SpawnCwd)
		}
		if len(meta.AgentEnv) > 0 {
			fmt.Printf("agent-env: %s\n", strings.Join(meta.AgentEnv, ", "))
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
	Long: `Print the accumulated evidence ledger for a session as JSON: the commands the
sidecar ran and the sources it read across all turns. Use it to audit exactly
what evidence backs a report, or to feed the trail into other tooling. Pass the
session name from ` + "`sessions list`" + `.`,
	Example: `  ghx sidecar sessions ledger hono-hono`,
	Args:    cobra.ExactArgs(1),
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

// sidecarSessionsRerouteCmd corrects a mis-routed turn (ADR-0030.1 D5): it
// moves the turn's report artifact to the destination session (created when
// absent) and deterministically rebuilds BOTH ledgers by replaying their
// remaining per-turn artifacts. The source session's ACP session ID is
// cleared so its next turn reseeds from the corrected durable ledger
// (the ADR-0027 stale-session path, pointed at a deliberately-retired one).
var sidecarSessionsRerouteCmd = &cobra.Command{
	Use:   "reroute <session> <turn> <dest>",
	Short: "Move a mis-routed turn to another session and rebuild both ledgers",
	Example: `  ghx sidecar sessions reroute honojs-hono 3 which-go-libraries-do-structured-conc-4be29e5c
  ghx sidecar sessions reroute gin-gonic-gin 2 my-new-thread`,
	Args: cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		turn, err := strconv.Atoi(args[1])
		if err != nil || turn < 1 {
			return fmt.Errorf("turn must be a positive integer, got %q", args[1])
		}
		cfg := sidecar.LoadConfig()
		res, viaDaemon, err := sidecar.RerouteViaDaemon(context.Background(), VERSION, cfg, sidecar.RerouteParams{
			From: args[0],
			Turn: turn,
			To:   args[2],
		})
		if err != nil {
			return err
		}
		fmt.Printf("moved turn %d of %s -> %s (as turn %d)\n", res.Turn, res.From, res.To, res.NewTurn)
		for _, p := range res.MovedReports {
			fmt.Printf("  report: %s\n", p)
		}
		if res.ToCreated {
			fmt.Printf("created session: %s\n", res.To)
		}
		fmt.Printf("ledgers rebuilt by replay: %s, %s\n", res.From, res.To)
		fmt.Printf("source ACP session cleared; next %s turn reseeds from the corrected ledger\n", res.From)
		if viaDaemon {
			fmt.Println("applied via the running daemon (warm worker retired)")
		} else {
			fmt.Println("applied directly on disk (no healthy daemon)")
		}
		return nil
	},
}

// sidecarConfigCmd groups config subcommands.
var sidecarConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage sidecar configuration",
	Long: `View and initialize the sidecar config at ~/.ghx/config.json (or $GHX_HOME),
which selects the ACP agent command and model/visibility settings. Run
` + "`config init`" + ` once to write it — ` + "`--claude-acp`" + ` for the pinned Claude ACP adapter,
otherwise PATH auto-detection — then ` + "`config show`" + ` to see the resolved values.`,
	Example: `  ghx sidecar config show
  ghx sidecar config init --claude-acp
  ghx sidecar config init --claude-acp --force`,
	Run: func(cmd *cobra.Command, args []string) { _ = cmd.Help() },
}

// sidecarConfigShowCmd prints the current config.
var sidecarConfigShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current sidecar configuration",
	Long: `Print the resolved sidecar configuration — agent command, model, visibility, and
storage paths. Use it to confirm what ` + "`ask`" + ` and ` + "`doctor`" + ` will use, especially
after editing the config or setting $GHX_HOME.`,
	Example: `  ghx sidecar config show`,
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
	Long: `Write the initial sidecar config. ` + "`--claude-acp`" + ` pins the Claude ACP adapter
(` + "`npx @agentclientprotocol/claude-agent-acp`" + `, needs Node/npx and a Claude login);
without it, ghx auto-detects an ACP-capable agent already on PATH. This is the
first setup step — follow it with ` + "`ghx sidecar doctor`" + `. An existing config is never
overwritten silently: the field diff is shown first, and ` + "`--force`" + ` (only with
` + "`--claude-acp`" + `) is required to apply it.`,
	Example: `  ghx sidecar config init --claude-acp
  ghx sidecar config init --claude-acp --force`,
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
	cfg.Route = existing.Route
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
	cfg.Route = existing.Route
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
	sidecarAskCmd.Flags().String("session", "", "Advanced: pin a specific named session; normally omit — ghx routes for you (ADR-0030.1)")
	sidecarAskCmd.Flags().String("repo", "", "GitHub repo owner/repo (optional scope; omit for cross-GitHub discovery)")
	sidecarAskCmd.Flags().String("depth", "normal", "Command budget: cheap|normal|deep")
	sidecarAskCmd.Flags().Bool("json", false, "Output full report as JSON")
	sidecarAskCmd.Flags().Bool("local", false, "Allow tier-2 local analysis (SHA-pinned clone, codemap, ast-grep, repomap) when remote evidence falls short")

	sidecarDoctorCmd.Flags().Bool("live", false, "Also run a real one-prompt turn through the agent (spawns it; diagnoses failures ACP initialize misses)")

	sidecarReportSinkCmd.Flags().String("out", "", "Path to write the accepted report JSON (required)")
	sidecarDaemonCmd.Flags().Bool("background", false, "Internal: daemon was auto-spawned in the background")
	_ = sidecarDaemonCmd.Flags().MarkHidden("background")
	sidecarDaemonCmd.Flags().Bool("stop", false, "Ask the running daemon to stop")

	sidecarEvalsExportCmd.Flags().String("format", "", "Training export format (sft)")
	sidecarEvalsExportCmd.Flags().String("run", "", "Eval run directory containing committed episode JSON files")
	sidecarEvalsExportCmd.Flags().String("out", "", "Output JSONL path")

	sidecarConfigInitCmd.Flags().Bool("claude-acp", false, "Write the pinned Claude ACP adapter command (npx @agentclientprotocol/claude-agent-acp) instead of auto-detecting")
	sidecarConfigInitCmd.Flags().Bool("force", false, "Overwrite an existing config (only with --claude-acp; a diff is shown first)")

	sidecarSessionsCmd.AddCommand(sidecarSessionsListCmd, sidecarSessionsShowCmd, sidecarSessionsLedgerCmd, sidecarSessionsRerouteCmd)
	sidecarConfigCmd.AddCommand(sidecarConfigShowCmd, sidecarConfigInitCmd)
	sidecarEvalsCmd.AddCommand(sidecarEvalsExportCmd)
	sidecarCmd.AddCommand(sidecarAskCmd, sidecarDaemonCmd, sidecarDoctorCmd, sidecarReportSinkCmd, sidecarEvalsCmd, sidecarSessionsCmd, sidecarConfigCmd)
}

// questionSession derives a stable session slug from the question when the ask
// has neither --session nor --repo (discovery mode, ADR-0019.1 D2): the
// kebab-cased leading words truncated to ~40 chars, plus a short content hash
// of the full question so distinct questions never collide.
func questionSession(question string) string {
	return sidecar.QuestionSlug(question)
}

func defaultReconSession(repo string) string {
	return sidecar.Slug(repo, "repo")
}

// kebabSlug lowercases s and collapses every non-alphanumeric run into a
// single dash, trimming leading/trailing dashes.
func kebabSlug(s string) string {
	return sidecar.Slug(s, "")
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
