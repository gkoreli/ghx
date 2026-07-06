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

// OpenAIJudgeClient is the real primary JudgeClient (ADR-0023.1 D4): a
// cross-family, OpenAI-compatible chat-completions client for the GPT-5.5-
// class primary judge. The subject under evaluation is Anthropic, so the
// primary judge is deliberately non-Anthropic (constraint 6, self-preference).
//
// Credentials come from the environment only (OPENAI_API_KEY); the committed
// judge config carries the model identity and parameters, never secrets. The
// base URL is overridable via GHX_JUDGE_OPENAI_BASE_URL for tests.
type OpenAIJudgeClient struct {
	model          JudgeModelConfig
	maxPromptChars int
	apiKey         string
	baseURL        string
	transport      judgeTransport
}

// Environment contract for the OpenAI judge client.
const (
	envOpenAIAPIKey       = "OPENAI_API_KEY"
	envOpenAIJudgeBaseURL = "GHX_JUDGE_OPENAI_BASE_URL"
	defaultOpenAIBaseURL  = "https://api.openai.com/v1"
)

// NewOpenAIJudgeClient builds the primary judge client from the committed
// config. It fails fast when OPENAI_API_KEY is unset — a judge run must never
// start half-credentialed.
func NewOpenAIJudgeClient(cfg *JudgeConfig) (*OpenAIJudgeClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("openai judge: nil config")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	key := strings.TrimSpace(os.Getenv(envOpenAIAPIKey))
	if key == "" {
		return nil, fmt.Errorf("openai judge: %s is not set (API keys come from the environment only)", envOpenAIAPIKey)
	}
	base := strings.TrimSpace(os.Getenv(envOpenAIJudgeBaseURL))
	if base == "" {
		base = defaultOpenAIBaseURL
	}
	return &OpenAIJudgeClient{
		model:          cfg.Primary,
		maxPromptChars: cfg.MaxPromptChars,
		apiKey:         key,
		baseURL:        strings.TrimRight(base, "/"),
		transport:      newJudgeTransport(judgeProviderOpenAI),
	}, nil
}

// ModelID identifies the judge model for committed artifacts (D4).
func (c *OpenAIJudgeClient) ModelID() string { return c.model.Model }

// Evaluate runs one judge invocation: one chat-completions call, strict JSON
// parsing, one re-ask on malformed output, typed error after that.
func (c *OpenAIJudgeClient) Evaluate(ctx context.Context, prompt string) (JudgeVerdict, error) {
	return evaluateWithReask(ctx, judgeProviderOpenAI, c.model.Model, prompt, c.maxPromptChars, c.call)
}

// ── OpenAI chat-completions wire types ───────────────────────────────────────

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIChatRequest struct {
	Model               string                `json:"model"`
	Messages            []openAIChatMessage   `json:"messages"`
	Temperature         *float64              `json:"temperature,omitempty"`
	MaxCompletionTokens int                   `json:"max_completion_tokens,omitempty"`
	ResponseFormat      *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

// call performs one chat-completions request over the full message history
// (the re-ask replays the model's bad turn plus the correction).
func (c *OpenAIJudgeClient) call(ctx context.Context, msgs []judgeMessage) (string, error) {
	reqBody := openAIChatRequest{
		Model:               c.model.Model,
		Temperature:         c.model.Temperature,
		MaxCompletionTokens: c.model.MaxOutputTokens,
		// json_object nudges strict JSON at the decoding layer; the schema
		// itself is enforced by parseJudgeVerdict against the core rubric.
		ResponseFormat: &openAIResponseFormat{Type: "json_object"},
	}
	for _, m := range msgs {
		reqBody.Messages = append(reqBody.Messages, openAIChatMessage(m))
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("openai judge: marshal request: %w", err)
	}

	body, err := c.transport.do(ctx, func(ctx context.Context) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		return req, nil
	})
	if err != nil {
		return "", err
	}

	var resp openAIChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("openai judge: decode response: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai judge: response has no choices")
	}
	return resp.Choices[0].Message.Content, nil
}
