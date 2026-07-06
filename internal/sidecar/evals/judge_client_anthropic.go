package evals

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// AnthropicJudgeClient is the secondary JudgeClient (ADR-0023.1 D4): an
// opus-class Anthropic Messages API client used for the 20% sample plus every
// episode the primary scores below threshold. Family disagreement is reported,
// never averaged away — this client only produces its own JudgeResult.
//
// Credentials come from the environment only (ANTHROPIC_API_KEY); the
// committed judge config carries the model identity, never secrets. The base
// URL is overridable via GHX_JUDGE_ANTHROPIC_BASE_URL for tests.
//
// Note: current opus-class models reject explicit sampling parameters
// (temperature and friends return HTTP 400), so the committed config leaves
// Temperature nil for the secondary; it is only sent when explicitly set.
type AnthropicJudgeClient struct {
	model          JudgeModelConfig
	maxPromptChars int
	apiKey         string
	baseURL        string
	transport      judgeTransport
}

// Environment contract for the Anthropic judge client.
const (
	envAnthropicAPIKey       = "ANTHROPIC_API_KEY"
	envAnthropicJudgeBaseURL = "GHX_JUDGE_ANTHROPIC_BASE_URL"
	defaultAnthropicBaseURL  = "https://api.anthropic.com"
	anthropicVersion         = "2023-06-01"
)

// NewAnthropicJudgeClient builds the secondary judge client from the committed
// config. It fails fast when ANTHROPIC_API_KEY is unset.
func NewAnthropicJudgeClient(cfg *JudgeConfig) (*AnthropicJudgeClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("anthropic judge: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(os.Getenv(envAnthropicAPIKey))
	if key == "" {
		return nil, fmt.Errorf("anthropic judge: %s is not set (API keys come from the environment only)", envAnthropicAPIKey)
	}
	base := strings.TrimSpace(os.Getenv(envAnthropicJudgeBaseURL))
	if base == "" {
		base = defaultAnthropicBaseURL
	}
	return &AnthropicJudgeClient{
		model:          cfg.Secondary,
		maxPromptChars: cfg.MaxPromptChars,
		apiKey:         key,
		baseURL:        strings.TrimRight(base, "/"),
		transport:      newJudgeTransport(judgeProviderAnthropic),
	}, nil
}

// ModelID identifies the judge model for committed artifacts (D4).
func (c *AnthropicJudgeClient) ModelID() string { return c.model.Model }

// Evaluate runs one judge invocation: one Messages API call, strict JSON
// parsing, one re-ask on malformed output, typed error after that.
func (c *AnthropicJudgeClient) Evaluate(ctx context.Context, prompt string) (JudgeVerdict, error) {
	return evaluateWithReask(ctx, judgeProviderAnthropic, c.model.Model, prompt, c.maxPromptChars, c.call)
}

// ── Anthropic Messages API wire types ────────────────────────────────────────

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicMessagesRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	Messages    []anthropicMessage `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
}

type anthropicMessagesResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
}

// call performs one Messages API request over the full message history (the
// re-ask replays the model's bad turn plus the correction).
func (c *AnthropicJudgeClient) call(ctx context.Context, msgs []judgeMessage) (string, error) {
	reqBody := anthropicMessagesRequest{
		Model:       c.model.Model,
		MaxTokens:   c.model.MaxOutputTokens,
		Temperature: c.model.Temperature,
	}
	for _, m := range msgs {
		reqBody.Messages = append(reqBody.Messages, anthropicMessage(m))
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("anthropic judge: marshal request: %w", err)
	}

	body, err := c.transport.do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
		return req, nil
	})
	if err != nil {
		return "", err
	}

	var resp anthropicMessagesResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("anthropic judge: decode response: %w", err)
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("anthropic judge: response has no text content (stop_reason %q)", resp.StopReason)
	}
	return sb.String(), nil
}
