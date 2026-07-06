package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// newRoot returns the root storage directory ghx writes to: $GHX_HOME when set,
// otherwise ~/.ghx (ADR-0022 D3). This is the location SaveConfig and new
// sessions always use; the legacy ~/.ghx-sidecar location is only ever read.
func newRoot() string {
	if h := strings.TrimSpace(os.Getenv("GHX_HOME")); h != "" {
		return h
	}
	return filepath.Join(os.Getenv("HOME"), ".ghx")
}

// legacyRoot is the pre-ADR-0022 storage location, read as a migration
// fallback but never written.
func legacyRoot() string {
	return filepath.Join(os.Getenv("HOME"), ".ghx-sidecar")
}

// activeRoot returns the root to READ from and whether it fell back to the
// legacy ~/.ghx-sidecar location. GHX_HOME (explicit override) and an existing
// ~/.ghx both win; only when neither exists and the legacy dir is present do we
// fall back to it so a pre-migration user keeps seeing their sessions.
func activeRoot() (dir string, legacy bool) {
	root := newRoot()
	if strings.TrimSpace(os.Getenv("GHX_HOME")) != "" {
		return root, false
	}
	if _, err := os.Stat(root); err == nil {
		return root, false
	}
	if _, err := os.Stat(legacyRoot()); err == nil {
		return legacyRoot(), true
	}
	return root, false
}

var migrationNoticeOnce sync.Once

// noteMigration prints a one-line migration notice to stderr, at most once per
// process, when config is being read from the legacy location (ADR-0022 D3).
func noteMigration(root string) {
	migrationNoticeOnce.Do(func() {
		fmt.Fprintf(os.Stderr,
			"note: reading legacy config from %s — run `ghx sidecar config init` to migrate to %s\n",
			root, newRoot())
	})
}

// VisibilityConfig controls the shared visibility runtime (ADR-0022 D4).
type VisibilityConfig struct {
	// CaptureContent, when set, overrides the default for GenAI message-content
	// log records in production. Nil means the default (on — local artifacts,
	// not exported telemetry). The OTel env var
	// OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT=false also disables it.
	CaptureContent *bool `json:"captureContent,omitempty"`
}

// ClaudeACPAgentCmd is the pinned Claude Code ACP adapter command line that
// `ghx sidecar config init --claude-acp` writes. It mirrors the eval wrapper
// scripts/eval-agent-acp.sh (same adapter, same version pin — bump them
// together, deliberately) minus the eval-only env (model pin, subject-model
// label, thinking budget). npx makes first-time setup a single command: the
// user needs Node and a Claude Code login, not a hand-written wrapper script
// (dogfood friction 2026-07-05 "ACP agent setup is the hardest step").
const ClaudeACPAgentCmd = "npx -y @agentclientprotocol/claude-agent-acp@0.55.0"

// Config holds ghx-sidecar user configuration.
type Config struct {
	// AgentCmd is the ACP agent binary name or a whitespace-separated command
	// line (e.g. "claude", "codex", or ClaudeACPAgentCmd). Multi-word values
	// are split into argv by splitAgentCmd — plain field splitting, no shell
	// quoting.
	AgentCmd string `json:"agent"`
	// SessionsDir is where named session artifacts are stored.
	// Defaults to ~/.ghx/sessions.
	SessionsDir string `json:"sessionsDir"`
	// Model pins the subject model for every session (e.g. "claude-sonnet-4-5").
	// Empty means the adapter's default. Set per-session via ACP _meta
	// claudeCode.options.model. Takes precedence over GHX_EVAL_SUBJECT_MODEL
	// for session creation but does NOT override the eval identity label.
	Model string `json:"model,omitempty"`
	// Visibility controls the shared telemetry/visibility runtime (ADR-0022).
	Visibility VisibilityConfig `json:"visibility,omitempty"`
	// Cwd overrides the ACP session cwd. Empty means current working directory.
	Cwd string `json:"-"`
	// Env overrides the spawned ACP adapter environment. Nil means inherit.
	Env []string `json:"-"`
	// EvalMode, when true, enables the raw SDK message audit channel
	// (emitRawSDKMessages:true in _meta.claudeCode.options). Off by default
	// in production. Set by the eval runner (ADR-0020.1 D5).
	EvalMode bool `json:"-"`
}

// CaptureContent reports whether GenAI message-content log records should be
// written for production sessions (ADR-0022 D4). Default on; disabled by the
// official OTel env var set to "false" or by visibility.captureContent=false.
func (c Config) CaptureContent() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT")), "false") {
		return false
	}
	if c.Visibility.CaptureContent != nil {
		return *c.Visibility.CaptureContent
	}
	return true
}

func defaultConfig(root string) Config {
	return Config{
		AgentCmd:    "claude",
		SessionsDir: filepath.Join(root, "sessions"),
	}
}

// NewDefaultConfig returns the default config rooted at the new ~/.ghx (or
// $GHX_HOME) root. `config init` uses this so migration always writes the new
// location (ADR-0022 D3).
func NewDefaultConfig() Config { return defaultConfig(newRoot()) }

// configFilePath returns the path to the JSON config file under root.
func configFilePath(root string) string { return filepath.Join(root, "config.json") }

// ConfigFilePath returns the config file path currently in effect (the active
// read root), for display.
func ConfigFilePath() string {
	root, _ := activeRoot()
	return configFilePath(root)
}

// LoadConfig reads the config file, falling back to defaults on any error.
// It reads from ~/.ghx (or $GHX_HOME), or the legacy ~/.ghx-sidecar location
// when that is the only one present (printing a one-line migration notice).
func LoadConfig() Config {
	root, legacy := activeRoot()
	if legacy {
		noteMigration(root)
	}
	def := defaultConfig(root)
	data, err := os.ReadFile(configFilePath(root))
	if err != nil {
		return def
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return def
	}
	if c.AgentCmd == "" {
		c.AgentCmd = def.AgentCmd
	}
	if c.SessionsDir == "" {
		c.SessionsDir = def.SessionsDir
	}
	return c
}

// ConfigFileExists reports whether a config file already exists at the active
// read root (including the legacy ~/.ghx-sidecar location), returning its
// path. `config init --claude-acp` uses it to refuse overwriting an existing
// config without --force.
func ConfigFileExists() (path string, exists bool) {
	root, _ := activeRoot()
	path = configFilePath(root)
	_, err := os.Stat(path)
	return path, err == nil
}

// DiffConfigs returns human-readable "field: old -> new" lines for the
// persisted fields that differ between two configs. An empty result means
// writing newCfg would change nothing. `config init --claude-acp` shows this
// diff before requiring --force, so the user sees exactly what would be
// overwritten.
func DiffConfigs(oldCfg, newCfg Config) []string {
	var diff []string
	add := func(field, o, n string) {
		if o != n {
			diff = append(diff, fmt.Sprintf("  %s: %s -> %s", field, o, n))
		}
	}
	add("agent", fmt.Sprintf("%q", oldCfg.AgentCmd), fmt.Sprintf("%q", newCfg.AgentCmd))
	add("sessionsDir", fmt.Sprintf("%q", oldCfg.SessionsDir), fmt.Sprintf("%q", newCfg.SessionsDir))
	add("model", fmt.Sprintf("%q", oldCfg.Model), fmt.Sprintf("%q", newCfg.Model))
	add("visibility.captureContent", formatBoolPtr(oldCfg.Visibility.CaptureContent), formatBoolPtr(newCfg.Visibility.CaptureContent))
	return diff
}

func formatBoolPtr(b *bool) string {
	if b == nil {
		return "(unset)"
	}
	return fmt.Sprintf("%t", *b)
}

// SaveConfig writes cfg to the new root (~/.ghx or $GHX_HOME), creating the
// directory if needed. It never writes the legacy location.
func SaveConfig(cfg Config) error {
	root := newRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", root, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(root), append(data, '\n'), 0o644)
}

// knownAgents are ACP-compatible agents probed by DetectAgents.
var knownAgents = []struct {
	name string
}{
	{"claude"},
	{"codex"},
	{"kiro"},
}

// DetectAgents probes PATH for known agents that complete ACP initialize.
// All probes run in parallel with a short timeout.
func DetectAgents(ctx context.Context) []string {
	type result struct {
		name string
		ok   bool
	}
	ch := make(chan result, len(knownAgents))
	tctx, cancel := context.WithTimeout(ctx, defaultHandshakeTimeout+time.Second)
	defer cancel()
	for _, a := range knownAgents {
		a := a
		go func() {
			err := checkACPHandshake(tctx, a.name, "", nil, defaultHandshakeTimeout)
			ch <- result{a.name, err == nil}
		}()
	}
	var found []string
	for range knownAgents {
		r := <-ch
		if r.ok {
			found = append(found, r.name)
		}
	}
	return found
}

// FormatConfig returns a human-readable summary of the config.
func FormatConfig(cfg Config) string {
	return fmt.Sprintf("agent:       %s\nsessionsDir: %s\nconfigFile:  %s",
		cfg.AgentCmd, cfg.SessionsDir, ConfigFilePath())
}

// ArtifactsHint tells a human where per-session OTel artifacts live and how to
// view them (ADR-0022 D5). Every session directory is a spec-exact OTLP
// bundle; `ghx sidecar view` replays it into a local viewer in one command
// (ADR-0026.1, absorbing the ADR-0018 "Live validation" curl recipe).
func ArtifactsHint(cfg Config) string {
	return fmt.Sprintf(
		"Session artifacts: %s/<session>/ (traces.jsonl, logs.jsonl, metrics.jsonl, reports/)\n"+
			"Browse any session in a local viewer UI: ghx sidecar view [session]\n"+
			"(spec-exact OTLP, so the ADR-0018 curl replay recipe also still works).",
		cfg.SessionsDir)
}
