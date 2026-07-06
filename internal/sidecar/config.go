package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rootDir returns the single root storage directory ghx reads and writes:
// $GHX_HOME when set, otherwise ~/.ghx (ADR-0022 D3). The pre-ADR-0022
// ~/.ghx-sidecar read-fallback was removed 2026-07-05 — the migration happened
// on the only install, so there is exactly one path with no legacy shim.
func rootDir() string {
	if h := strings.TrimSpace(os.Getenv("GHX_HOME")); h != "" {
		return h
	}
	return filepath.Join(os.Getenv("HOME"), ".ghx")
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
	// Route overrides the session-routing knobs (ADR-0030.1 D3). Nil means
	// the pinned defaults; the knobs are part of the daemon config digest so
	// a change restarts the warm daemon (ADR-0030 D6).
	Route *RouteSettings `json:"route,omitempty"`
	// AgentSettingSources opts the spawned agent into loading host Claude Code
	// setting files: any of "user" (~/.claude/settings.json), "project",
	// "local". DEFAULT (nil/absent) is full isolation — the session sends
	// settingSources:[] so the agent inherits no host CLAUDE.md/settings
	// (the ADR-0016.3 isolation posture). Set ["user"] on machines whose
	// Claude install NEEDS user settings to function — enterprise/toolbox
	// builds resolve Bedrock model aliases (modelOverrides) from that file,
	// and blocking it hangs the agent at prompt time (founder isolation,
	// 2026-07-06). Opting in weakens session isolation: host user settings
	// (env, hooks, model overrides) apply to sidecar sessions. Eval runs
	// construct their own Config and are unaffected.
	AgentSettingSources []string `json:"agentSettingSources,omitempty"`
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

// RouteSettings are the user-tunable session-routing knobs (ADR-0030.1 D3),
// resolved over the pinned defaults by RouteConfigFor. Zero/nil fields keep
// the defaults; the decision-table tests in route_test.go are the pinned
// specification of those defaults.
type RouteSettings struct {
	// OverlapThreshold is the minimum R4 overlap score to join an existing
	// session (default 0.5).
	OverlapThreshold *float64 `json:"overlapThreshold,omitempty"`
	// OverlapMargin is the required lead over the runner-up (default 0.25).
	OverlapMargin *float64 `json:"overlapMargin,omitempty"`
	// ContinuationWindowMinutes bounds R3 warm candidates (default 30, the
	// daemon warm-worker TTL).
	ContinuationWindowMinutes int `json:"continuationWindowMinutes,omitempty"`
	// RoutingWindowHours bounds R4 candidates (default 168 = 7 days).
	RoutingWindowHours int `json:"routingWindowHours,omitempty"`
}

// defaultConfig is what a machine with no config file gets: the pinned Claude
// ACP adapter, NOT the bare `claude` CLI. Bare claude cannot speak ACP, so the
// old default meant every fresh install failed its first ask until the user
// discovered `config init --claude-acp` (founder's work-laptop friction,
// ADR-0033 follow-up). Zero-config first runs must work out of the box
// wherever Node/npx and a Claude login exist.
func defaultConfig(root string) Config {
	return Config{
		AgentCmd:    ClaudeACPAgentCmd,
		SessionsDir: filepath.Join(root, "sessions"),
	}
}

// NewDefaultConfig returns the default config rooted at ~/.ghx (or $GHX_HOME).
func NewDefaultConfig() Config { return defaultConfig(rootDir()) }

// RootDir returns the active ghx home directory: $GHX_HOME when set, otherwise
// ~/.ghx. Daemon runtime files and sidecar artifacts share this root.
func RootDir() string { return rootDir() }

// RuntimeDir returns the daemon lifecycle directory under the active ghx home.
func RuntimeDir() string { return filepath.Join(rootDir(), "runtime") }

// configFilePath returns the path to the JSON config file under root.
func configFilePath(root string) string { return filepath.Join(root, "config.json") }

// ConfigFilePath returns the config file path in effect, for display.
func ConfigFilePath() string { return configFilePath(rootDir()) }

// LoadConfig reads the config file from ~/.ghx (or $GHX_HOME), falling back to
// defaults on any error.
func LoadConfig() Config {
	root := rootDir()
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
	c.AgentSettingSources = validAgentSettingSources(c.AgentSettingSources)
	return c
}

// validAgentSettingSources filters agentSettingSources to the adapter's
// recognized values (user, project, local), warning about anything else so a
// typo degrades loudly to isolation instead of silently misconfiguring the
// session. nil in, nil out (full isolation default).
func validAgentSettingSources(sources []string) []string {
	if sources == nil {
		return nil
	}
	valid := sources[:0]
	for _, s := range sources {
		switch s {
		case "user", "project", "local":
			valid = append(valid, s)
		default:
			fmt.Fprintf(os.Stderr, "warning: agentSettingSources: unknown value %q ignored (valid: user, project, local)\n", s)
		}
	}
	return valid
}

// ConfigFileExists reports whether a config file already exists at the root,
// returning its path. `config init --claude-acp` uses it to refuse overwriting
// an existing config without --force.
func ConfigFileExists() (path string, exists bool) {
	path = configFilePath(rootDir())
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
	add("route", formatRouteSettings(oldCfg.Route), formatRouteSettings(newCfg.Route))
	return diff
}

func formatRouteSettings(r *RouteSettings) string {
	if r == nil {
		return "(defaults)"
	}
	data, _ := json.Marshal(r)
	return string(data)
}

func formatBoolPtr(b *bool) string {
	if b == nil {
		return "(unset)"
	}
	return fmt.Sprintf("%t", *b)
}

// SaveConfig writes cfg to the root (~/.ghx or $GHX_HOME), creating the
// directory if needed.
func SaveConfig(cfg Config) error {
	root := rootDir()
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
