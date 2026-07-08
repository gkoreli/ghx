package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/telemetry"
)

// stressEpisode builds a fake, fully-scored episode with large text/thinking so
// each emitted JSONL line is well over the platform pipe buffer — the regime
// where concurrent O_APPEND writes interleave without external serialization.
func stressEpisode(i int) *Episode {
	blob := strings.Repeat(fmt.Sprintf("episode-%03d-payload ", i), 900) // ~18KB
	now := time.Now().UTC()
	return &Episode{
		ID:        fmt.Sprintf("task-%03d_ghx-sidecar_%d", i, now.UnixNano()),
		TaskID:    fmt.Sprintf("task-%03d", i),
		Repo:      "honojs/hono",
		Profile:   ProfileSidecar,
		Parallel:  true,
		Identity:  AgentIdentity{SubjectModel: "test-model", AdapterName: "mock", AdapterVersion: "1"},
		Report:    &sidecar.Report{Answer: "found in src/a.ts"},
		Rewards:   RewardBreakdown{Correctness: 1, Evidence: 1, Safety: 1, Overall: 0.9},
		Context:   ContextAccounting{MainAgentChars: 500, TotalWorkflowChars: 9000},
		StartedAt: now.Add(-time.Second),
		EndedAt:   now,
		Turns: []TurnRecord{{
			Turn:            0,
			Question:        "where is routing handled? " + blob,
			Text:            "routing lives in src/router.ts. " + blob,
			Thinking:        "reasoning: " + blob,
			ToolOutputChars: len(blob),
			DurationMs:      1234,
			ToolTraces: []sidecar.ToolCallTrace{{
				ID:            fmt.Sprintf("tool-%03d", i),
				Kind:          "read",
				Title:         "read src/router.ts",
				OutputSize:    len(blob),
				OutputExcerpt: blob[:2048],
			}},
			Report: &sidecar.Report{Answer: "found in src/a.ts"},
		}},
	}
}

// assertAllLinesValidJSON reads a JSONL artifact and fails if any line is not a
// whole, parseable JSON object — the signature of an interleaved append.
func assertAllLinesValidJSON(t *testing.T, path string, wantAtLeast int) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Base(path), err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	valid := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("%s line %d is not valid JSON (interleaved append?): %v\nline: %.200s",
				filepath.Base(path), i+1, err, line)
		}
		valid++
	}
	if valid < wantAtLeast {
		t.Fatalf("%s: %d valid lines, want at least %d", filepath.Base(path), valid, wantAtLeast)
	}
	return valid
}

// TestParallelSaveEpisodeJSONLIntegrity saves many episodes concurrently into
// one run dir and proves the shared logs/metrics/traces JSONL files stay
// line-valid and unique episode JSONs are written. Run with -race to also
// prove there is no data race in the append path (ADR-0025 D3).
func TestParallelSaveEpisodeJSONLIntegrity(t *testing.T) {
	// Capture message content so logs.jsonl is populated for every episode.
	t.Setenv(genAICaptureMessageContentEnv, "true")

	runDir := t.TempDir()
	const n = 40

	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ep := stressEpisode(i)
			if _, err := SaveEpisode(runDir, ep); err != nil {
				errs <- fmt.Errorf("save episode %d: %w", i, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	// Unique episode JSON per save.
	eps, err := LoadRunEpisodes(runDir)
	if err != nil {
		t.Fatalf("load run episodes: %v", err)
	}
	if len(eps) != n {
		t.Fatalf("loaded %d episode files, want %d — a filename collision dropped one", len(eps), n)
	}

	// Metrics and logs write exactly one line per episode; traces write one or
	// more (SimpleSpanProcessor exports per span). Every line must parse whole.
	assertAllLinesValidJSON(t, filepath.Join(runDir, metricFileName), n)
	assertAllLinesValidJSON(t, filepath.Join(runDir, logFileName), n)
	assertAllLinesValidJSON(t, filepath.Join(runDir, traceFileName), n)
}

// TestAppendJSONLineConcurrentLargeLines hammers the shared appender directly
// with large distinct lines from many goroutines and verifies no line is
// spliced or lost.
func TestAppendJSONLineConcurrentLargeLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shared.jsonl")
	const n = 64

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload := strings.Repeat("x", 20000)
			line, _ := json.Marshal(map[string]any{"i": i, "payload": payload})
			line = append(line, '\n')
			if err := telemetry.AppendJSONLine(path, line); err != nil {
				t.Errorf("append %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	seen := map[int]bool{}
	count := assertAllLinesValidJSON(t, path, n)
	data, _ := os.ReadFile(path)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var doc struct {
			I int `json:"i"`
		}
		if err := json.Unmarshal([]byte(line), &doc); err == nil {
			seen[doc.I] = true
		}
	}
	if count != n || len(seen) != n {
		t.Fatalf("got %d lines / %d distinct indices, want %d each (a write was lost or spliced)", count, len(seen), n)
	}
}

// TestParallelRateLimitAnomaly proves the D3 rate-limit fallback: a parallel
// episode whose turn failed with a rate-limit-shaped error records the soft
// eval_parallel_rate_limited anomaly, and a sequential episode does not.
func TestParallelRateLimitAnomaly(t *testing.T) {
	parallel := &Episode{
		Profile:  ProfileSidecar,
		Parallel: true,
		Turns:    []TurnRecord{{Turn: 0, Error: "sidecar turn 0: HTTP 429 Too Many Requests (rate limit exceeded)"}},
	}
	got := DetectAnomalies(parallel)
	if !hasAnomaly(got, AnomalyParallelRateLimited, SeveritySoft) {
		t.Fatalf("parallel rate-limited turn should record %s (soft); got %v", AnomalyParallelRateLimited, got)
	}

	// Same error but not parallel → no parallel-rate-limit anomaly.
	sequential := &Episode{
		Profile: ProfileSidecar,
		Turns:   []TurnRecord{{Turn: 0, Error: "HTTP 429 rate limit"}},
	}
	if hasAnomaly(DetectAnomalies(sequential), AnomalyParallelRateLimited, SeveritySoft) {
		t.Fatal("a sequential episode must not record the parallel-rate-limit anomaly")
	}

	// A non-rate-limit failure under parallelism → no anomaly.
	other := &Episode{
		Profile:  ProfileSidecar,
		Parallel: true,
		Turns:    []TurnRecord{{Turn: 0, Error: "context deadline exceeded"}},
	}
	if hasAnomaly(DetectAnomalies(other), AnomalyParallelRateLimited, SeveritySoft) {
		t.Fatal("a non-rate-limit failure must not record the parallel-rate-limit anomaly")
	}
}

func hasAnomaly(as []Anomaly, kind string, sev AnomalySeverity) bool {
	for _, a := range as {
		if a.Kind == kind && a.Severity == sev {
			return true
		}
	}
	return false
}

// TestEvalParallelismEnv documents the GHX_EVAL_PARALLEL resolution.
func TestEvalParallelismEnv(t *testing.T) {
	cases := []struct {
		set  bool
		val  string
		want int
	}{
		{false, "", defaultEvalParallel},
		{true, "1", 1},
		{true, "5", 5},
		{true, "0", 1},
		{true, "-3", 1},
		{true, "garbage", 1},
	}
	for _, c := range cases {
		if c.set {
			t.Setenv(EvalParallelEnv, c.val)
		} else {
			os.Unsetenv(EvalParallelEnv)
		}
		if got := EvalParallelism(); got != c.want {
			t.Errorf("EvalParallelism() with %q = %d, want %d", c.val, got, c.want)
		}
	}
}
