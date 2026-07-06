package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// defaultConfigDir returns ~/.ghx-sidecar.
func defaultConfigDir() string { return filepath.Join(os.Getenv("HOME"), ".ghx-sidecar") }

// Config holds ghx-sidecar user configuration.
type Config struct {
	// AgentCmd is the ACP agent binary name or full command (e.g. "claude", "codex").
	AgentCmd string `json:"agent"`
	// SessionsDir is where named session artifacts are stored.
	// Defaults to ~/.ghx-sidecar/sessions.
	SessionsDir string `json:"sessionsDir"`
	// Model pins the subject model for every session (e.g. "claude-sonnet-4-5").
	// Empty means the adapter's default. Set per-session via ACP _meta
	// claudeCode.options.model. Takes precedence over GHX_EVAL_SUBJECT_MODEL
	// for session creation but does NOT override the eval identity label.
	Model string `json:"model,omitempty"`
	// Cwd overrides the ACP session cwd. Empty means current working directory.
	Cwd string `json:"-"`
	// Env overrides the spawned ACP adapter environment. Nil means inherit.
	Env []string `json:"-"`
	// EvalMode, when true, enables the raw SDK message audit channel
	// (emitRawSDKMessages:true in _meta.claudeCode.options). Off by default
	// in production. Set by the eval runner (ADR-0020.1 D5).
	EvalMode bool `json:"-"`
}

func defaultConfig() Config {
	dir := defaultConfigDir()
	return Config{
		AgentCmd:    "claude",
		SessionsDir: filepath.Join(dir, "sessions"),
	}
}

// configFilePath returns the path to the JSON config file.
func configFilePath() string { return filepath.Join(defaultConfigDir(), "config.json") }

// LoadConfig reads the config file, falling back to defaults on any error.
func LoadConfig() Config {
	data, err := os.ReadFile(configFilePath())
	if err != nil {
		return defaultConfig()
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return defaultConfig()
	}
	def := defaultConfig()
	if c.AgentCmd == "" {
		c.AgentCmd = def.AgentCmd
	}
	if c.SessionsDir == "" {
		c.SessionsDir = def.SessionsDir
	}
	return c
}

// SaveConfig writes cfg to disk, creating the config directory if needed.
func SaveConfig(cfg Config) error {
	dir := defaultConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(), append(data, '\n'), 0o644)
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
		cfg.AgentCmd, cfg.SessionsDir, configFilePath())
}
