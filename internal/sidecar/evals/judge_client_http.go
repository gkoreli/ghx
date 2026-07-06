package evals

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Shared HTTP plumbing for the real JudgeClient implementations (ADR-0023.1
// D4). Both providers get identical behavior: a per-call timeout, bounded
// retries with jitter on 429/5xx, a hard cap on request prompt size and
// response body size, strict JSON parsing of the verdict against the schema
// the prompt template specifies, and exactly one re-ask (with the parse error
// appended) before a typed MalformedJudgeOutputError.

const (
	// defaultJudgeTimeout bounds one HTTP call (frontier judge models can
	// reason for a while before emitting the small JSON verdict).
	defaultJudgeTimeout = 120 * time.Second
	// defaultJudgeMaxRetries is the number of retries after the first attempt
	// on 429/5xx responses.
	defaultJudgeMaxRetries = 3
	// defaultJudgeBackoffBase / Max bound the jittered exponential backoff.
	defaultJudgeBackoffBase = 500 * time.Millisecond
	defaultJudgeBackoffMax  = 8 * time.Second
	// judgeMaxRespBytes is the hard cap on a provider response body; the
	// verdict JSON is tiny, so anything near this cap is malfunction.
	judgeMaxRespBytes = 1 << 20 // 1 MiB
	// judgeErrBodyMax bounds provider error bodies carried inside errors.
	judgeErrBodyMax = 2048
)

// judgeMessage is one provider-neutral chat turn.
type judgeMessage struct {
	Role    string // "user" | "assistant"
	Content string
}

// JudgeHTTPError is a non-2xx provider response that was not retried to
// success. The body is truncated; it never contains credentials.
type JudgeHTTPError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *JudgeHTTPError) Error() string {
	return fmt.Sprintf("%s judge: HTTP %d: %s", e.Provider, e.StatusCode, e.Body)
}

// MalformedJudgeOutputError is returned when the judge model's output failed
// strict schema parsing twice (original call + one re-ask carrying the parse
// error). Raw is the truncated second reply, kept for audit.
type MalformedJudgeOutputError struct {
	Provider string
	Model    string
	ParseErr error
	Raw      string
}

func (e *MalformedJudgeOutputError) Error() string {
	return fmt.Sprintf("%s judge model %s returned malformed output after re-ask: %v (raw: %s)",
		e.Provider, e.Model, e.ParseErr, e.Raw)
}

func (e *MalformedJudgeOutputError) Unwrap() error { return e.ParseErr }

// PromptTooLargeError is returned before any network call when the rendered
// prompt exceeds the committed per-call size cap (JudgeConfig.MaxPromptChars).
type PromptTooLargeError struct {
	Provider string
	Size     int
	Cap      int
}

func (e *PromptTooLargeError) Error() string {
	return fmt.Sprintf("%s judge: prompt is %d chars, exceeds the committed per-call cap %d", e.Provider, e.Size, e.Cap)
}

// judgeTransport performs one provider call with retries. It is shared by both
// clients; only the request builder and response decoder differ per provider.
type judgeTransport struct {
	provider   string
	client     *http.Client
	maxRetries int
	backoff    time.Duration
	backoffMax time.Duration
}

func newJudgeTransport(provider string) judgeTransport {
	return judgeTransport{
		provider:   provider,
		client:     &http.Client{Timeout: defaultJudgeTimeout},
		maxRetries: defaultJudgeMaxRetries,
		backoff:    defaultJudgeBackoffBase,
		backoffMax: defaultJudgeBackoffMax,
	}
}

// do POSTs the request produced by build, retrying 429 and 5xx responses with
// jittered exponential backoff (honoring a numeric Retry-After when present,
// capped at backoffMax). build is called per attempt because an *http.Request
// body cannot be reused.
func (t *judgeTransport) do(ctx context.Context, build func(ctx context.Context) (*http.Request, error)) ([]byte, error) {
	var lastErr error
	attempts := t.maxRetries + 1
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := t.wait(ctx, attempt, lastErr); err != nil {
				return nil, err
			}
		}
		req, err := build(ctx)
		if err != nil {
			return nil, err
		}
		resp, err := t.client.Do(req)
		if err != nil {
			// Transport-level failure (timeout, connection refused): retryable.
			lastErr = fmt.Errorf("%s judge: %w", t.provider, err)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, judgeMaxRespBytes+1))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("%s judge: read response: %w", t.provider, readErr)
			continue
		}
		if len(body) > judgeMaxRespBytes {
			return nil, fmt.Errorf("%s judge: response exceeds %d-byte cap", t.provider, judgeMaxRespBytes)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return body, nil
		}
		httpErr := &JudgeHTTPError{
			Provider:   t.provider,
			StatusCode: resp.StatusCode,
			Body:       boundedString(strings.TrimSpace(string(body)), judgeErrBodyMax),
		}
		if !retryableJudgeStatus(resp.StatusCode) {
			return nil, httpErr
		}
		httpErr.Body = withRetryAfter(httpErr.Body, resp.Header.Get("Retry-After"))
		lastErr = httpErr
	}
	return nil, fmt.Errorf("%s judge: giving up after %d attempts: %w", t.provider, attempts, lastErr)
}

// wait sleeps the jittered exponential backoff for the given attempt (1-based
// for the first retry), preferring a provider Retry-After hint when the last
// error carried one, and aborts early if ctx is done.
func (t *judgeTransport) wait(ctx context.Context, attempt int, lastErr error) error {
	d := t.backoff << (attempt - 1)
	if hint, ok := retryAfterHint(lastErr); ok && hint > d {
		d = hint
	}
	if d > t.backoffMax {
		d = t.backoffMax
	}
	// Full jitter: uniform in (d/2, d] keeps retries desynchronized without
	// collapsing to zero waits.
	d = d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retryableJudgeStatus: 429 rate limits and all 5xx (including Anthropic's
// 529 overloaded) are retryable; every other non-2xx is a hard error.
func retryableJudgeStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// withRetryAfter / retryAfterHint smuggle a numeric Retry-After (seconds)
// through the recorded error so the next wait can honor it.
func withRetryAfter(body, retryAfter string) string {
	retryAfter = strings.TrimSpace(retryAfter)
	if retryAfter == "" {
		return body
	}
	if _, err := strconv.Atoi(retryAfter); err != nil {
		return body
	}
	return body + " [retry-after:" + retryAfter + "s]"
}

func retryAfterHint(err error) (time.Duration, bool) {
	var httpErr *JudgeHTTPError
	if !errors.As(err, &httpErr) {
		return 0, false
	}
	idx := strings.LastIndex(httpErr.Body, "[retry-after:")
	if idx < 0 {
		return 0, false
	}
	rest := httpErr.Body[idx+len("[retry-after:"):]
	end := strings.Index(rest, "s]")
	if end < 0 {
		return 0, false
	}
	secs, err2 := strconv.Atoi(rest[:end])
	if err2 != nil || secs <= 0 {
		return 0, false
	}
	return time.Duration(secs) * time.Second, true
}

// evaluateWithReask runs the shared Evaluate flow: strict-parse the model's
// reply; on a parse error, re-ask exactly once with the model's raw reply and
// the parse error appended to the conversation; if the re-ask is still
// malformed, return a typed MalformedJudgeOutputError.
func evaluateWithReask(
	ctx context.Context,
	provider, model, prompt string,
	maxPromptChars int,
	call func(ctx context.Context, msgs []judgeMessage) (string, error),
) (JudgeVerdict, error) {
	if maxPromptChars > 0 && len(prompt) > maxPromptChars {
		return JudgeVerdict{}, &PromptTooLargeError{Provider: provider, Size: len(prompt), Cap: maxPromptChars}
	}

	msgs := []judgeMessage{{Role: "user", Content: prompt}}
	raw, err := call(ctx, msgs)
	if err != nil {
		return JudgeVerdict{}, err
	}
	v, parseErr := parseJudgeVerdict(raw)
	if parseErr == nil {
		return v, nil
	}

	msgs = append(msgs,
		judgeMessage{Role: "assistant", Content: raw},
		judgeMessage{Role: "user", Content: reaskInstruction(parseErr)},
	)
	raw2, err := call(ctx, msgs)
	if err != nil {
		return JudgeVerdict{}, err
	}
	v2, parseErr2 := parseJudgeVerdict(raw2)
	if parseErr2 == nil {
		return v2, nil
	}
	return JudgeVerdict{}, &MalformedJudgeOutputError{
		Provider: provider,
		Model:    model,
		ParseErr: parseErr2,
		Raw:      boundedString(strings.TrimSpace(raw2), judgeErrBodyMax),
	}
}

// reaskInstruction is the one-shot correction message appended after a
// malformed reply, carrying the exact parse error back to the model.
func reaskInstruction(parseErr error) string {
	return fmt.Sprintf(
		"Your previous reply was not valid against the required output schema: %v.\n"+
			"Return ONLY the JSON object in exactly the shape specified earlier — "+
			"no prose, no markdown fences, all %d core dimensions present.",
		parseErr, len(coreRubric.Dimensions))
}

// parseJudgeVerdict strictly parses one judge reply against the output schema
// the prompt template specifies: a single JSON object with a `dimensions`
// array covering exactly the core-rubric dimensions (each once, integer score
// within scale), an in-scale `overall`, and optional explanations. A leading/
// trailing markdown code fence is tolerated (models add them despite
// instructions); anything else non-JSON is a parse error.
func parseJudgeVerdict(raw string) (JudgeVerdict, error) {
	s := stripJSONFences(raw)
	if s == "" {
		return JudgeVerdict{}, fmt.Errorf("empty reply")
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	var v JudgeVerdict
	if err := dec.Decode(&v); err != nil {
		return JudgeVerdict{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return JudgeVerdict{}, fmt.Errorf("trailing content after the JSON object")
	}
	if err := validateVerdict(v); err != nil {
		return JudgeVerdict{}, err
	}
	return v, nil
}

// validateVerdict checks the parsed verdict against the committed core rubric.
func validateVerdict(v JudgeVerdict) error {
	rubric := coreRubric
	want := rubric.dimensionNames()
	seen := make(map[string]bool, len(want))
	for _, d := range v.Dimensions {
		valid := false
		for _, name := range want {
			if d.Dimension == name {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unknown dimension %q", d.Dimension)
		}
		if seen[d.Dimension] {
			return fmt.Errorf("dimension %q appears more than once", d.Dimension)
		}
		seen[d.Dimension] = true
		if d.Score < rubric.ScaleMin || d.Score > rubric.ScaleMax {
			return fmt.Errorf("dimension %q score %d out of scale [%d,%d]",
				d.Dimension, d.Score, rubric.ScaleMin, rubric.ScaleMax)
		}
	}
	for _, name := range want {
		if !seen[name] {
			return fmt.Errorf("missing dimension %q", name)
		}
	}
	if v.Overall < float64(rubric.ScaleMin) || v.Overall > float64(rubric.ScaleMax) {
		return fmt.Errorf("overall %v out of scale [%d,%d]", v.Overall, rubric.ScaleMin, rubric.ScaleMax)
	}
	return nil
}

// stripJSONFences removes one enclosing markdown code fence (``` or ```json)
// when present, leaving the interior untouched.
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	rest := s[3:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		lang := strings.TrimSpace(rest[:nl])
		if lang == "" || strings.EqualFold(lang, "json") {
			rest = rest[nl+1:]
			if end := strings.LastIndex(rest, "```"); end >= 0 {
				return strings.TrimSpace(rest[:end])
			}
		}
	}
	return s
}
