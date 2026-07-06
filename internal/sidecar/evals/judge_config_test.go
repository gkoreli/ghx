package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultJudgeConfigValidates(t *testing.T) {
	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatalf("DefaultJudgeConfig: %v", err)
	}
	if cfg.SchemaVersion != judgeConfigSchemaVersion {
		t.Errorf("schemaVersion = %q, want %q", cfg.SchemaVersion, judgeConfigSchemaVersion)
	}
	if cfg.PromptVersion != judgePromptVersion || cfg.RubricVersion != rubricVersion {
		t.Errorf("versions = (%q, %q), want (%q, %q)",
			cfg.PromptVersion, cfg.RubricVersion, judgePromptVersion, rubricVersion)
	}
	if cfg.Samples != defaultJudgeSamples {
		t.Errorf("samples = %d, want %d (D4 k=3 baseline)", cfg.Samples, defaultJudgeSamples)
	}
	if cfg.Primary.Provider != judgeProviderOpenAI || cfg.Secondary.Provider != judgeProviderAnthropic {
		t.Errorf("providers = (%q, %q), want (openai, anthropic) per D4", cfg.Primary.Provider, cfg.Secondary.Provider)
	}
	if cfg.Thresholds.Poor != judgePoorThreshold {
		t.Errorf("thresholds.poor = %v, want %v", cfg.Thresholds.Poor, judgePoorThreshold)
	}
	if cfg.Thresholds.SecondarySampleRate != 0.2 {
		t.Errorf("secondarySampleRate = %v, want 0.2 (D4)", cfg.Thresholds.SecondarySampleRate)
	}
	// The secondary must never carry sampling params: current opus-class
	// models reject them (HTTP 400).
	if cfg.Secondary.Temperature != nil {
		t.Errorf("secondary.temperature = %v, want unset", *cfg.Secondary.Temperature)
	}
}

// TestLoadJudgeConfigMatchesEmbedded proves the committed file on disk and the
// embedded copy are the same artifact.
func TestLoadJudgeConfigMatchesEmbedded(t *testing.T) {
	disk, err := LoadJudgeConfig(filepath.Join("judgeconfig", "judge-config-v1.json"))
	if err != nil {
		t.Fatalf("LoadJudgeConfig: %v", err)
	}
	embedded, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatalf("DefaultJudgeConfig: %v", err)
	}
	if *disk != *embedded {
		t.Errorf("disk config %+v != embedded config %+v", *disk, *embedded)
	}
}

func TestJudgeConfigValidationErrors(t *testing.T) {
	base := func(t *testing.T) *JudgeConfig {
		cfg, err := DefaultJudgeConfig()
		if err != nil {
			t.Fatalf("DefaultJudgeConfig: %v", err)
		}
		return cfg
	}
	temp := func(v float64) *float64 { return &v }

	cases := []struct {
		name    string
		mutate  func(*JudgeConfig)
		wantSub string
	}{
		{"bad schema version", func(c *JudgeConfig) { c.SchemaVersion = "judge-config-v0" }, "schemaVersion"},
		{"prompt version drift", func(c *JudgeConfig) { c.PromptVersion = "judge-prompt-v0" }, "judgePromptVersion"},
		{"rubric version drift", func(c *JudgeConfig) { c.RubricVersion = "core-rubric-v0" }, "rubricVersion"},
		{"zero samples", func(c *JudgeConfig) { c.Samples = 0 }, "samples"},
		{"zero prompt cap", func(c *JudgeConfig) { c.MaxPromptChars = 0 }, "maxPromptChars"},
		{"primary wrong family", func(c *JudgeConfig) { c.Primary.Provider = judgeProviderAnthropic }, "cross-family"},
		{"secondary wrong family", func(c *JudgeConfig) { c.Secondary.Provider = judgeProviderOpenAI }, "cross-family"},
		{"blank model", func(c *JudgeConfig) { c.Primary.Model = "  " }, "model is blank"},
		{"zero output tokens", func(c *JudgeConfig) { c.Secondary.MaxOutputTokens = 0 }, "maxOutputTokens"},
		{"temperature out of range", func(c *JudgeConfig) { c.Primary.Temperature = temp(2.5) }, "temperature"},
		{"poor threshold drift", func(c *JudgeConfig) { c.Thresholds.Poor = 3.0 }, "judgePoorThreshold"},
		{"sample rate out of range", func(c *JudgeConfig) { c.Thresholds.SecondarySampleRate = 1.5 }, "secondarySampleRate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base(t)
			tc.mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error containing %q", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("Validate() = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

func TestLoadJudgeConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "judge-config.json")
	data, err := os.ReadFile(filepath.Join("judgeconfig", "judge-config-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(data), "{\n", "{\n  \"apiKey\": \"sk-should-never-be-here\",\n", 1)
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadJudgeConfig(path); err == nil {
		t.Fatal("LoadJudgeConfig accepted an unknown field; the committed config must be strict")
	}
}
