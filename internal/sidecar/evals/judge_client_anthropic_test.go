package evals

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// anthropicMessageBody wraps text in an Anthropic Messages API response.
func anthropicMessageBody(t *testing.T, text string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"content":     []map[string]any{{"type": "text", "text": text}},
		"stop_reason": "end_turn",
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// newAnthropicTestClient points a real AnthropicJudgeClient at an httptest
// server via the documented env overrides and makes retries fast.
func newAnthropicTestClient(t *testing.T, serverURL string) *AnthropicJudgeClient {
	t.Helper()
	t.Setenv(envAnthropicAPIKey, "test-key-not-real")
	t.Setenv(envAnthropicJudgeBaseURL, serverURL)
	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewAnthropicJudgeClient(cfg)
	if err != nil {
		t.Fatalf("NewAnthropicJudgeClient: %v", err)
	}
	c.transport.backoff = time.Millisecond
	c.transport.backoffMax = 5 * time.Millisecond
	return c
}

func TestAnthropicJudgeEvaluateSuccess(t *testing.T) {
	var gotPath, gotKey, gotVersion, rawBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		b, _ := io.ReadAll(r.Body)
		rawBody = string(b)
		_, _ = w.Write(anthropicMessageBody(t, validVerdictJSON()))
	}))
	defer srv.Close()

	c := newAnthropicTestClient(t, srv.URL)
	v, err := c.Evaluate(context.Background(), "judge this")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(v.Dimensions) != 3 || v.Overall != 4 {
		t.Errorf("verdict = %+v, want 3 dimensions and overall 4", v)
	}
	if gotPath != "/v1/messages" {
		t.Errorf("path = %q, want /v1/messages", gotPath)
	}
	if gotKey != "test-key-not-real" {
		t.Errorf("x-api-key header = %q", gotKey)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version = %q, want %q", gotVersion, anthropicVersion)
	}
	var req anthropicMessagesRequest
	if err := json.Unmarshal([]byte(rawBody), &req); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if req.Model != "claude-opus-4-8" {
		t.Errorf("request model = %q, want claude-opus-4-8 (committed config)", req.Model)
	}
	if req.MaxTokens != 4096 {
		t.Errorf("max_tokens = %d, want 4096 (committed config)", req.MaxTokens)
	}
	// Opus-class models reject sampling params; the committed config leaves
	// temperature unset and the wire request must not carry the key at all.
	if strings.Contains(rawBody, "temperature") {
		t.Errorf("request body carries temperature: %s", rawBody)
	}
	if c.ModelID() != "claude-opus-4-8" {
		t.Errorf("ModelID() = %q", c.ModelID())
	}
}

func TestAnthropicJudgeReasksOnceOnMalformedOutput(t *testing.T) {
	var calls atomic.Int64
	var secondReq anthropicMessagesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			// Out-of-scale score: valid JSON, invalid against the schema.
			_, _ = w.Write(anthropicMessageBody(t,
				`{"dimensions": [{"dimension": "evidence_groundedness", "score": 9}], "overall": 4}`))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&secondReq); err != nil {
			t.Errorf("decode re-ask request: %v", err)
		}
		_, _ = w.Write(anthropicMessageBody(t, validVerdictJSON()))
	}))
	defer srv.Close()

	c := newAnthropicTestClient(t, srv.URL)
	v, err := c.Evaluate(context.Background(), "judge this")
	if err != nil {
		t.Fatalf("Evaluate after re-ask: %v", err)
	}
	if v.Overall != 4 {
		t.Errorf("overall = %v, want 4", v.Overall)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2", calls.Load())
	}
	if len(secondReq.Messages) != 3 {
		t.Fatalf("re-ask messages = %d, want 3 (user, assistant, correction)", len(secondReq.Messages))
	}
	if secondReq.Messages[2].Role != "user" || !strings.Contains(secondReq.Messages[2].Content, "out of scale") {
		t.Errorf("re-ask correction = %+v, want the schema parse error", secondReq.Messages[2])
	}
}

func TestAnthropicJudgeMalformedTwiceIsTypedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(anthropicMessageBody(t, "The reconnaissance looks solid to me."))
	}))
	defer srv.Close()

	c := newAnthropicTestClient(t, srv.URL)
	_, err := c.Evaluate(context.Background(), "judge this")
	var malformed *MalformedJudgeOutputError
	if !errors.As(err, &malformed) {
		t.Fatalf("error = %v, want *MalformedJudgeOutputError", err)
	}
	if malformed.Provider != judgeProviderAnthropic || malformed.Model != "claude-opus-4-8" {
		t.Errorf("typed error = %+v", malformed)
	}
}

func TestAnthropicJudgeRetriesOnOverloaded(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(529) // Anthropic overloaded_error
			_, _ = w.Write([]byte(`{"type": "error", "error": {"type": "overloaded_error"}}`))
		case 2:
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"type": "error", "error": {"type": "rate_limit_error"}}`))
		default:
			_, _ = w.Write(anthropicMessageBody(t, validVerdictJSON()))
		}
	}))
	defer srv.Close()

	c := newAnthropicTestClient(t, srv.URL)
	if _, err := c.Evaluate(context.Background(), "judge this"); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 (529, 429, then success)", calls.Load())
	}
}

func TestAnthropicJudgeTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	c := newAnthropicTestClient(t, srv.URL)
	c.transport.client.Timeout = 30 * time.Millisecond
	c.transport.maxRetries = 0
	if _, err := c.Evaluate(context.Background(), "judge this"); err == nil {
		t.Fatal("Evaluate = nil error, want timeout")
	}
}

func TestNewAnthropicJudgeClientRequiresAPIKey(t *testing.T) {
	t.Setenv(envAnthropicAPIKey, "")
	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewAnthropicJudgeClient(cfg); err == nil || !strings.Contains(err.Error(), envAnthropicAPIKey) {
		t.Fatalf("NewAnthropicJudgeClient without key = %v, want missing-key error", err)
	}
}
