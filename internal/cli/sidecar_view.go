package cli

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/spf13/cobra"
)

// viewerBinary is the absorbed OTLP viewer (ADR-0026.1). We spawn it from
// PATH and replay session artifacts into it; we never vendor or auto-install.
const viewerBinary = "otel-desktop-viewer"

// viewerInstallHint is printed when the viewer binary is missing.
const viewerInstallHint = `otel-desktop-viewer not found on PATH.

Install it with:

  go install github.com/CtrlSpice/otel-desktop-viewer@latest

then re-run: ghx sidecar view`

// viewerIngestBase is the viewer's OTLP HTTP ingest endpoint. Fixed at the
// otel-desktop-viewer default (4318, the standard OTLP/HTTP port); only the
// browser UI port is configurable via --port.
const viewerIngestBase = "http://localhost:4318"

// sidecarViewCmd turns a session's committed OTel artifacts into a browsable
// UI: spawn otel-desktop-viewer, replay traces.jsonl (plus logs/metrics when
// present) into its OTLP HTTP ingest, and keep the viewer in the foreground
// until Ctrl-C (ADR-0026.1, absorbing the ADR-0018 curl replay recipe).
var sidecarViewCmd = &cobra.Command{
	Use:   "view [session]",
	Short: "Browse a session's OTel artifacts in a local viewer UI",
	Long: `ghx sidecar view — replay a session's committed OTel artifacts
(traces.jsonl, plus logs.jsonl/metrics.jsonl when present) into a local
otel-desktop-viewer UI.

With no argument it views the most recent session. Requires otel-desktop-viewer
on PATH (go install github.com/CtrlSpice/otel-desktop-viewer@latest).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		list, _ := cmd.Flags().GetBool("list")
		cfg := sidecar.LoadConfig()

		if list {
			sessions, err := sidecar.ListViewSessions(cfg.SessionsDir)
			if err != nil {
				return err
			}
			fmt.Println(sidecar.FormatViewSessions(sessions))
			return nil
		}

		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		session, err := sidecar.ResolveViewSession(cfg.SessionsDir, name)
		if err != nil {
			return err
		}

		viewerPath, err := exec.LookPath(viewerBinary)
		if err != nil {
			fmt.Fprintln(os.Stderr, viewerInstallHint)
			return fmt.Errorf("%s not found on PATH", viewerBinary)
		}

		return runViewer(cmd.Context(), viewerPath, port, session)
	},
}

// runViewer spawns the viewer, waits for its OTLP HTTP ingest, replays the
// session artifacts, then blocks in the foreground until Ctrl-C or viewer
// exit, cleaning up the child process.
func runViewer(parent context.Context, viewerPath string, port int, session sidecar.ViewSession) error {
	if parent == nil {
		parent = context.Background()
	}
	ctx, stop := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Fail fast if the OTLP ingest port is already occupied (most likely a
	// viewer from an earlier run). Spawning anyway would replay the session
	// into that foreign process while our own viewer dies on the bind.
	if conn, err := net.DialTimeout("tcp", "localhost:4318", 250*time.Millisecond); err == nil {
		_ = conn.Close()
		return fmt.Errorf("port 4318 (OTLP ingest) is already in use — another %s is likely running; stop it and re-run",
			viewerBinary)
	}

	viewer := exec.CommandContext(ctx, viewerPath,
		"--open-browser=false",
		"--browser-port", strconv.Itoa(port))
	viewer.Stdout = os.Stderr
	viewer.Stderr = os.Stderr
	// On Ctrl-C, interrupt the viewer politely; force-kill if it lingers.
	viewer.Cancel = func() error { return viewer.Process.Signal(os.Interrupt) }
	viewer.WaitDelay = 3 * time.Second
	if err := viewer.Start(); err != nil {
		return fmt.Errorf("start %s: %w", viewerBinary, err)
	}
	exited := make(chan error, 1)
	go func() { exited <- viewer.Wait() }()

	// Wait for the OTLP ingest, but notice if the viewer dies first (e.g.
	// ports already taken by another viewer instance) — otherwise the probe
	// could succeed against the foreign listener and replay into it.
	client := &http.Client{Timeout: 10 * time.Second}
	ingestUp := make(chan error, 1)
	go func() { ingestUp <- sidecar.WaitForIngest(ctx, client, viewerIngestBase, 15*time.Second) }()
	select {
	case err := <-exited:
		if err == nil {
			err = fmt.Errorf("exit status 0")
		}
		return fmt.Errorf("%s exited before its OTLP ingest came up — is another viewer already using port 4318 or %d? (%w)",
			viewerBinary, port, err)
	case err := <-ingestUp:
		if err != nil {
			_ = viewer.Cancel()
			<-exited
			return err
		}
	}

	counts, err := sidecar.ReplaySession(ctx, client, viewerIngestBase, session.Dir)
	if err != nil {
		_ = viewer.Cancel()
		<-exited
		return fmt.Errorf("replay %s: %w", session.Name, err)
	}
	fmt.Printf("Replayed session %q: %d trace, %d log, %d metric lines\n",
		session.Name, counts["traces"], counts["logs"], counts["metrics"])
	fmt.Printf("Viewer: http://localhost:%d  (Ctrl-C to stop)\n", port)

	err = <-exited
	if ctx.Err() != nil {
		// Clean Ctrl-C shutdown: the interrupt is the user's, not a failure.
		return nil
	}
	return err
}

func init() {
	sidecarViewCmd.Flags().Int("port", 8000, "Viewer UI port")
	sidecarViewCmd.Flags().Bool("list", false, "List sessions with turn/report counts instead of launching")
	sidecarCmd.AddCommand(sidecarViewCmd)
}
