package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CLIJudgeClient is the default judge transport: it reaches subscription CLIs
// already used for delegation, instead of API keys. The prompt is passed on
// stdin, stdout is bounded, and the final message is parsed with the same
// strict verdict parser used by the HTTP clients.
type CLIJudgeClient struct {
	provider       string
	model          string
	modelID        string
	maxPromptChars int
	cmd            JudgeCommandConfig
	timeout        time.Duration
	runtimeVersion string
}

// NewPrimaryCLIJudgeClient builds the GPT-family primary judge from config.
func NewPrimaryCLIJudgeClient(cfg *JudgeConfig) (*CLIJudgeClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%s cli judge: nil config", judgeProviderOpenAI)
	}
	return newCLIJudgeClient(cfg, "primary", judgeProviderOpenAI, cfg.Primary)
}

// NewSecondaryCLIJudgeClient builds the Claude-family secondary judge from
// config.
func NewSecondaryCLIJudgeClient(cfg *JudgeConfig) (*CLIJudgeClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%s cli judge: nil config", judgeProviderAnthropic)
	}
	return newCLIJudgeClient(cfg, "secondary", judgeProviderAnthropic, cfg.Secondary)
}

func newCLIJudgeClient(cfg *JudgeConfig, slot, provider string, model JudgeModelConfig) (*CLIJudgeClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("%s cli judge: nil config", provider)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Transport != judgeTransportCLI {
		return nil, fmt.Errorf("%s cli judge: config transport %q, want %q", provider, cfg.Transport, judgeTransportCLI)
	}
	version, err := runJudgeVersion(context.Background(), model.CLI)
	if err != nil {
		return nil, fmt.Errorf("%s cli judge: capture %s.cli version: %w", provider, slot, err)
	}
	return &CLIJudgeClient{
		provider:       provider,
		model:          model.Model,
		modelID:        model.Model + " via " + model.CLI.Command,
		maxPromptChars: cfg.MaxPromptChars,
		cmd:            model.CLI,
		timeout:        defaultJudgeTimeout,
		runtimeVersion: version,
	}, nil
}

// ModelID identifies the model and CLI rail for committed artifacts.
func (c *CLIJudgeClient) ModelID() string { return c.modelID }

// RuntimeVersion reports the CLI binary version captured at construction.
func (c *CLIJudgeClient) RuntimeVersion() string { return c.runtimeVersion }

// Evaluate runs one CLI judge invocation, strict-parses the final message, and
// performs one fresh retry when the first reply is malformed.
func (c *CLIJudgeClient) Evaluate(ctx context.Context, prompt string) (JudgeVerdict, error) {
	if c.maxPromptChars > 0 && len(prompt) > c.maxPromptChars {
		return JudgeVerdict{}, &PromptTooLargeError{Provider: c.provider, Size: len(prompt), Cap: c.maxPromptChars}
	}
	raw, err := c.call(ctx, prompt)
	if err != nil {
		return JudgeVerdict{}, err
	}
	v, parseErr := parseJudgeVerdict(raw)
	if parseErr == nil {
		return v, nil
	}

	raw2, err := c.call(ctx, prompt+"\n\n"+reaskInstruction(parseErr))
	if err != nil {
		return JudgeVerdict{}, err
	}
	v2, parseErr2 := parseJudgeVerdict(raw2)
	if parseErr2 == nil {
		return v2, nil
	}
	return JudgeVerdict{}, &MalformedJudgeOutputError{
		Provider: c.provider,
		Model:    c.modelID,
		ParseErr: parseErr2,
		Raw:      boundedString(strings.TrimSpace(raw2), judgeErrBodyMax),
	}
}

func (c *CLIJudgeClient) call(ctx context.Context, prompt string) (string, error) {
	callCtx := ctx
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok && c.timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()

	dir, err := os.MkdirTemp("", "ghx-judge-cli-*")
	if err != nil {
		return "", fmt.Errorf("%s cli judge: create temp dir: %w", c.provider, err)
	}
	defer os.RemoveAll(dir)

	outputPath := filepath.Join(dir, "last-message.txt")
	args := renderJudgeCommandArgs(c.cmd.Args, c.model, outputPath)
	cmd := exec.CommandContext(callCtx, c.cmd.Command, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if callCtx.Err() != nil {
			return "", callCtx.Err()
		}
		return "", fmt.Errorf("%s cli judge: %s %s failed: %w: %s",
			c.provider, c.cmd.Command, strings.Join(args, " "), err, boundedString(strings.TrimSpace(stderr.String()), judgeErrBodyMax))
	}
	if stdout.Len() > judgeMaxRespBytes {
		return "", fmt.Errorf("%s cli judge: stdout exceeds %d-byte cap", c.provider, judgeMaxRespBytes)
	}

	if c.cmd.OutputLastMessage {
		data, err := os.ReadFile(outputPath)
		if err != nil {
			return "", fmt.Errorf("%s cli judge: read output-last-message: %w", c.provider, err)
		}
		return boundedString(string(data), judgeMaxRespBytes), nil
	}
	raw := stdout.String()
	if c.cmd.ExtractLastCodexEvent {
		if msg, ok := extractLastCodexMessage(raw); ok {
			return msg, nil
		}
	}
	return raw, nil
}

func runJudgeVersion(ctx context.Context, cfg JudgeCommandConfig) (string, error) {
	versionCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(versionCtx, cfg.Command, cfg.VersionArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if versionCtx.Err() != nil {
			return "", versionCtx.Err()
		}
		return "", fmt.Errorf("%s %s: %w: %s", cfg.Command, strings.Join(cfg.VersionArgs, " "), err, strings.TrimSpace(stderr.String()))
	}
	version := strings.TrimSpace(stdout.String())
	if version == "" {
		version = strings.TrimSpace(stderr.String())
	}
	if version == "" {
		return "", fmt.Errorf("%s %s produced empty version", cfg.Command, strings.Join(cfg.VersionArgs, " "))
	}
	return version, nil
}

func renderJudgeCommandArgs(args []string, modelID, outputPath string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		arg = strings.ReplaceAll(arg, "{model}", modelID)
		arg = strings.ReplaceAll(arg, "{output_file}", outputPath)
		out[i] = arg
	}
	return out
}

func extractLastCodexMessage(stdout string) (string, bool) {
	var last string
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if msg, ok := codexEventMessage(event); ok {
			last = msg
		}
	}
	return last, last != ""
}

func codexEventMessage(event map[string]any) (string, bool) {
	if eventType, _ := event["type"].(string); eventType == "agent_message" || eventType == "codex" || eventType == "message" {
		if msg, ok := event["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg, true
		}
		if msg, ok := event["content"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg, true
		}
	}
	for _, key := range []string{"item", "data"} {
		nested, _ := event[key].(map[string]any)
		if nested == nil {
			continue
		}
		if role, _ := nested["role"].(string); role != "" && role != "assistant" {
			continue
		}
		if msg, ok := nested["message"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg, true
		}
		if msg, ok := nested["content"].(string); ok && strings.TrimSpace(msg) != "" {
			return msg, true
		}
		if text, ok := nestedText(nested["content"]); ok {
			return text, true
		}
	}
	return "", false
}

func nestedText(v any) (string, bool) {
	switch x := v.(type) {
	case []any:
		var parts []string
		for _, item := range x {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			if text, _ := m["text"].(string); strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, ""), true
		}
	case map[string]any:
		if text, _ := x["text"].(string); strings.TrimSpace(text) != "" {
			return text, true
		}
	}
	return "", false
}
