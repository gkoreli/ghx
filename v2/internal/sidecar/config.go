package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	cmd  string
}{
	{"claude", "claude --version"},
	{"codex", "codex --version"},
	{"kiro", "kiro --version"},
}

// DetectAgents probes PATH for known ACP-compatible agents.
// All probes run in parallel with a 3-second timeout.
func DetectAgents(ctx context.Context) []string {
	type result struct {
		name string
		ok   bool
	}
	ch := make(chan result, len(knownAgents))
	tctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for _, a := range knownAgents {
		a := a
		go func() {
			cmd := exec.CommandContext(tctx, "sh", "-c", a.cmd)
			err := cmd.Run()
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
