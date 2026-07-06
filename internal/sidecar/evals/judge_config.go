package evals

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// judgeConfigSchemaVersion versions the committed judge config artifact
// (ADR-0023.1 D4; Phoenix committed-config pattern per the ADR-0026 steal
// list). The config file is a measurement-stack artifact: any change to it is
// a committed diff that re-triggers calibration (D5) before scores are
// citable.
const judgeConfigSchemaVersion = "judge-config-v1"

// Judge model provider identifiers accepted in the committed config.
const (
	judgeProviderOpenAI    = "openai"
	judgeProviderAnthropic = "anthropic"
)

//go:embed judgeconfig/judge-config-v1.json
var defaultJudgeConfigJSON []byte

// JudgeModelConfig is one judge model's committed identity and parameters.
// It holds NO secrets — API keys come from the environment only (OPENAI_API_KEY
// / ANTHROPIC_API_KEY), never from this file.
type JudgeModelConfig struct {
	// Provider selects the wire protocol: "openai" (chat completions) or
	// "anthropic" (Messages API).
	Provider string `json:"provider"`
	// Model is the exact model ID recorded on every JudgeResult (D4).
	Model string `json:"model"`
	// Temperature is optional; when nil the provider default is used. Note:
	// current Anthropic opus-class models (and OpenAI reasoning-class models)
	// reject explicit sampling parameters — leave nil unless the target model
	// documents support.
	Temperature *float64 `json:"temperature,omitempty"`
	// MaxOutputTokens is the hard per-call output cap sent to the provider.
	MaxOutputTokens int `json:"maxOutputTokens"`
}

// JudgeThresholds are the committed decision thresholds (D4).
type JudgeThresholds struct {
	// Poor is the "poor" cutoff on the 1–5 scale. It must equal the
	// judgePoorThreshold constant used by the disagreement report — the config
	// and the code are changed together, in one committed diff.
	Poor float64 `json:"poor"`
	// SecondarySampleRate is the D4 share of non-poor primary results that
	// also get an Anthropic second opinion (0.2 = 20% sample).
	SecondarySampleRate float64 `json:"secondarySampleRate"`
}

// JudgeConfig is the committed judge configuration loaded at client
// construction (ADR-0023.1 D4). Every field is part of the frozen measurement
// stack; the canonical instance lives in
// internal/sidecar/evals/judgeconfig/judge-config-v1.json and is embedded so
// the binary always carries the committed version.
type JudgeConfig struct {
	SchemaVersion string `json:"schemaVersion"`
	// PromptVersion / RubricVersion must match the code constants
	// (judgePromptVersion / rubricVersion); validation fails on drift so a
	// prompt change cannot silently ship under an old config.
	PromptVersion string `json:"promptVersion"`
	RubricVersion string `json:"rubricVersion"`
	// Samples is k in k-sample self-consistency (D4; k=3 baseline).
	Samples int `json:"samples"`
	// MaxPromptChars is the hard per-call input size cap: prompts larger than
	// this are rejected before any network call.
	MaxPromptChars int `json:"maxPromptChars"`

	Primary    JudgeModelConfig `json:"primary"`
	Secondary  JudgeModelConfig `json:"secondary"`
	Thresholds JudgeThresholds  `json:"thresholds"`
}

// DefaultJudgeConfig parses and validates the embedded committed config.
func DefaultJudgeConfig() (*JudgeConfig, error) {
	return parseJudgeConfig(defaultJudgeConfigJSON, "embedded judgeconfig/judge-config-v1.json")
}

// LoadJudgeConfig reads and validates a judge config file from disk. Unknown
// fields are rejected: the config is a committed measurement artifact and must
// not carry silently ignored keys.
func LoadJudgeConfig(path string) (*JudgeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseJudgeConfig(data, path)
}

func parseJudgeConfig(data []byte, source string) (*JudgeConfig, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cfg JudgeConfig
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse judge config %s: %w", source, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("judge config %s: %w", source, err)
	}
	return &cfg, nil
}

// Validate enforces the committed-config invariants, including the
// no-silent-drift checks against the code-side prompt and rubric versions.
func (c *JudgeConfig) Validate() error {
	if c.SchemaVersion != judgeConfigSchemaVersion {
		return fmt.Errorf("schemaVersion %q, want %q", c.SchemaVersion, judgeConfigSchemaVersion)
	}
	if c.PromptVersion != judgePromptVersion {
		return fmt.Errorf("promptVersion %q does not match code judgePromptVersion %q — prompt and config must change in one committed diff",
			c.PromptVersion, judgePromptVersion)
	}
	if c.RubricVersion != rubricVersion {
		return fmt.Errorf("rubricVersion %q does not match code rubricVersion %q — rubric and config must change in one committed diff",
			c.RubricVersion, rubricVersion)
	}
	if c.Samples < 1 || c.Samples > 9 {
		return fmt.Errorf("samples %d out of range [1,9]", c.Samples)
	}
	if c.MaxPromptChars <= 0 {
		return fmt.Errorf("maxPromptChars must be > 0, got %d", c.MaxPromptChars)
	}
	if err := c.Primary.validate("primary", judgeProviderOpenAI); err != nil {
		return err
	}
	if err := c.Secondary.validate("secondary", judgeProviderAnthropic); err != nil {
		return err
	}
	if c.Thresholds.Poor != judgePoorThreshold {
		return fmt.Errorf("thresholds.poor %v does not match code judgePoorThreshold %v — threshold and config must change in one committed diff",
			c.Thresholds.Poor, judgePoorThreshold)
	}
	if c.Thresholds.SecondarySampleRate < 0 || c.Thresholds.SecondarySampleRate > 1 {
		return fmt.Errorf("thresholds.secondarySampleRate %v out of range [0,1]", c.Thresholds.SecondarySampleRate)
	}
	return nil
}

func (m *JudgeModelConfig) validate(slot, wantProvider string) error {
	if m.Provider != wantProvider {
		return fmt.Errorf("%s.provider %q, want %q (D4: cross-family primary, Anthropic secondary)", slot, m.Provider, wantProvider)
	}
	if strings.TrimSpace(m.Model) == "" {
		return fmt.Errorf("%s.model is blank", slot)
	}
	if m.MaxOutputTokens <= 0 {
		return fmt.Errorf("%s.maxOutputTokens must be > 0, got %d", slot, m.MaxOutputTokens)
	}
	if m.Temperature != nil && (*m.Temperature < 0 || *m.Temperature > 2) {
		return fmt.Errorf("%s.temperature %v out of range [0,2]", slot, *m.Temperature)
	}
	return nil
}
