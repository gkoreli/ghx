package evals

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// validVerdictJSON is a reply matching the strict output schema the prompt
// template specifies (all three core dimensions, in-scale scores).
func validVerdictJSON() string {
	return `{
  "dimensions": [
    {"dimension": "evidence_groundedness", "score": 4, "explanation": "claims cite files"},
    {"dimension": "exploration_efficiency", "score": 3, "explanation": "one redundant read"},
    {"dimension": "uncertainty_honesty", "score": 5, "explanation": "clean labeling"}
  ],
  "overall": 4,
  "explanation": "solid reconnaissance"
}`
}

// chatCompletionBody wraps content in an OpenAI chat-completions response.
func chatCompletionBody(t *testing.T, content string) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"content": content}, "finish_reason": "stop"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// newOpenAITestClient points a real OpenAIJudgeClient at an httptest server
// via the documented env overrides and makes retries fast.
func newOpenAITestClient(t *testing.T, serverURL string) *OpenAIJudgeClient {
	t.Helper()
	t.Setenv(envOpenAIAPIKey, "test-key-not-real")
	t.Setenv(envOpenAIJudgeBaseURL, serverURL)
	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewOpenAIJudgeClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAIJudgeClient: %v", err)
	}
	c.transport.backoff = time.Millisecond
	c.transport.backoffMax = 5 * time.Millisecond
	return c
}

func TestOpenAIJudgeEvaluateSuccess(t *testing.T) {
	var gotReq openAIChatRequest
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write(chatCompletionBody(t, validVerdictJSON()))
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	v, err := c.Evaluate(context.Background(), "judge this")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(v.Dimensions) != 3 || v.Overall != 4 {
		t.Errorf("verdict = %+v, want 3 dimensions and overall 4", v)
	}
	if gotAuth != "Bearer test-key-not-real" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", gotPath)
	}
	if gotReq.Model != "gpt-5.5" {
		t.Errorf("request model = %q, want gpt-5.5 (committed config)", gotReq.Model)
	}
	if gotReq.ResponseFormat == nil || gotReq.ResponseFormat.Type != "json_object" {
		t.Errorf("response_format = %+v, want json_object", gotReq.ResponseFormat)
	}
	if gotReq.MaxCompletionTokens != 8192 {
		t.Errorf("max_completion_tokens = %d, want 8192 (committed config)", gotReq.MaxCompletionTokens)
	}
	if len(gotReq.Messages) != 1 || gotReq.Messages[0].Role != "user" || gotReq.Messages[0].Content != "judge this" {
		t.Errorf("messages = %+v", gotReq.Messages)
	}
	if c.ModelID() != "gpt-5.5" {
		t.Errorf("ModelID() = %q", c.ModelID())
	}
}

func TestOpenAIJudgeReasksOnceOnMalformedOutput(t *testing.T) {
	var calls atomic.Int64
	var secondReq openAIChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n == 1 {
			_, _ = w.Write(chatCompletionBody(t, "I'd rate this about 4 out of 5 overall."))
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&secondReq); err != nil {
			t.Errorf("decode re-ask request: %v", err)
		}
		_, _ = w.Write(chatCompletionBody(t, "```json\n"+validVerdictJSON()+"\n```"))
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	v, err := c.Evaluate(context.Background(), "judge this")
	if err != nil {
		t.Fatalf("Evaluate after re-ask: %v", err)
	}
	if v.Overall != 4 {
		t.Errorf("overall = %v, want 4", v.Overall)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (original + one re-ask)", calls.Load())
	}
	// The re-ask replays the bad assistant turn and appends the parse error.
	if len(secondReq.Messages) != 3 {
		t.Fatalf("re-ask messages = %d, want 3 (user, assistant, correction)", len(secondReq.Messages))
	}
	if secondReq.Messages[1].Role != "assistant" || !strings.Contains(secondReq.Messages[1].Content, "4 out of 5") {
		t.Errorf("re-ask turn 2 = %+v, want the model's malformed reply", secondReq.Messages[1])
	}
	if secondReq.Messages[2].Role != "user" || !strings.Contains(secondReq.Messages[2].Content, "not valid") {
		t.Errorf("re-ask turn 3 = %+v, want the correction with the parse error", secondReq.Messages[2])
	}
}

func TestOpenAIJudgeMalformedTwiceIsTypedError(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write(chatCompletionBody(t, `{"scores": "not the schema"}`))
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	_, err := c.Evaluate(context.Background(), "judge this")
	var malformed *MalformedJudgeOutputError
	if !errors.As(err, &malformed) {
		t.Fatalf("Evaluate error = %v, want *MalformedJudgeOutputError", err)
	}
	if malformed.Provider != judgeProviderOpenAI || malformed.Model != "gpt-5.5" {
		t.Errorf("typed error = %+v", malformed)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want exactly 2 (one re-ask, then give up)", calls.Load())
	}
}

func TestOpenAIJudgeRetriesOn429(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"type": "rate_limit_error"}}`))
			return
		}
		_, _ = w.Write(chatCompletionBody(t, validVerdictJSON()))
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	if _, err := c.Evaluate(context.Background(), "judge this"); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (429 then success)", calls.Load())
	}
}

func TestOpenAIJudgeGivesUpAfterBoundedRetries(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "upstream exploded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	c.transport.maxRetries = 1
	_, err := c.Evaluate(context.Background(), "judge this")
	var httpErr *JudgeHTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("error = %v, want wrapped *JudgeHTTPError 500", err)
	}
	if !strings.Contains(err.Error(), "giving up after 2 attempts") {
		t.Errorf("error = %v, want bounded-retry give-up", err)
	}
	if calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 (initial + 1 retry)", calls.Load())
	}
}

func TestOpenAIJudgeDoesNotRetryClientErrors(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, `{"error": {"type": "invalid_request_error"}}`, http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	_, err := c.Evaluate(context.Background(), "judge this")
	var httpErr *JudgeHTTPError
	if !errors.As(err, &httpErr) || httpErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("error = %v, want *JudgeHTTPError 400", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (4xx is never retried)", calls.Load())
	}
}

func TestOpenAIJudgeTimesOut(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	c := newOpenAITestClient(t, srv.URL)
	c.transport.client.Timeout = 30 * time.Millisecond
	c.transport.maxRetries = 0
	start := time.Now()
	_, err := c.Evaluate(context.Background(), "judge this")
	if err == nil {
		t.Fatal("Evaluate = nil error, want timeout")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("timeout took %v, want bounded by the per-call timeout", elapsed)
	}
}

func TestOpenAIJudgeEnforcesPromptCap(t *testing.T) {
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	c.maxPromptChars = 16
	_, err := c.Evaluate(context.Background(), strings.Repeat("x", 17))
	var tooLarge *PromptTooLargeError
	if !errors.As(err, &tooLarge) {
		t.Fatalf("error = %v, want *PromptTooLargeError", err)
	}
	if calls.Load() != 0 {
		t.Errorf("calls = %d, want 0 (cap is enforced before any network call)", calls.Load())
	}
}

func TestOpenAIJudgeEnforcesResponseCap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, judgeMaxRespBytes+10))
	}))
	defer srv.Close()

	c := newOpenAITestClient(t, srv.URL)
	_, err := c.Evaluate(context.Background(), "judge this")
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("error = %v, want response-size cap error", err)
	}
}

func TestNewOpenAIJudgeClientRequiresAPIKey(t *testing.T) {
	t.Setenv(envOpenAIAPIKey, "")
	cfg, err := DefaultJudgeConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewOpenAIJudgeClient(cfg); err == nil || !strings.Contains(err.Error(), envOpenAIAPIKey) {
		t.Fatalf("NewOpenAIJudgeClient without key = %v, want missing-key error", err)
	}
}
