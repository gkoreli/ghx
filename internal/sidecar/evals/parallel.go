package evals

import (
	"os"
	"strconv"
	"strings"
)

// ghxEvalParallelAttr is the OTel attribute key marking metrics and spans that
// belong to an episode which ran under episode-level parallelism (ADR-0025 D3).
const ghxEvalParallelAttr = "ghx.eval.parallel"

// EvalParallelEnv names the environment variable that controls how many
// task × profile episode cells TestEpisodes runs concurrently (ADR-0025 D3).
const EvalParallelEnv = "GHX_EVAL_PARALLEL"

// defaultEvalParallel is the concurrency used when GHX_EVAL_PARALLEL is unset.
//
// ADR-0025 D3 decision: fast is the new default. A full sample is 6 tasks ×
// 3 profiles × 5 trials = 90 episodes, each a multi-minute live agent run;
// serial execution is hours of wall clock. Three concurrent cells is a
// conservative fan-out that a single developer machine and typical model
// rate limits tolerate, and it is the value the gate-run harness assumes.
// Sequential execution is one env var away: GHX_EVAL_PARALLEL=1.
const defaultEvalParallel = 3

// EvalParallelism resolves the configured episode concurrency limit.
//
//   - unset            → defaultEvalParallel (fast default)
//   - "1"              → 1 (fully sequential; parallelism off)
//   - "n" (n >= 1)     → n
//   - invalid or < 1   → 1 (conservative: a typo must never silently fan out
//     and contaminate latency measurements)
func EvalParallelism() int {
	raw := strings.TrimSpace(os.Getenv(EvalParallelEnv))
	if raw == "" {
		return defaultEvalParallel
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// parallelGate bounds how many t.Parallel() subtests execute their episode
// body at once. Go's t.Parallel() releases every subtest to run together once
// the parent returns; the gate is a counting semaphore that caps the real
// concurrency at the configured limit so a 90-episode run does not spawn 90
// live agents simultaneously.
//
// Cell exclusivity (ADR-0025 D3): within one TestEpisodes invocation each
// subtest is a distinct task × profile cell, so two trials of the same cell
// never run concurrently here — repeated trials come from separate sequential
// invocations into the same GHX_EVAL_RUN_DIR. The gate therefore never needs
// per-cell locking; it only bounds total fan-out.
type parallelGate struct {
	slots chan struct{}
	limit int
}

func newParallelGate(limit int) *parallelGate {
	if limit < 1 {
		limit = 1
	}
	return &parallelGate{slots: make(chan struct{}, limit), limit: limit}
}

// enabled reports whether episodes should run in parallel at all. At limit 1
// the gate is a no-op and subtests stay sequential.
func (g *parallelGate) enabled() bool { return g.limit > 1 }

func (g *parallelGate) acquire() { g.slots <- struct{}{} }
func (g *parallelGate) release() { <-g.slots }

// rateLimitSignals are substrings, matched case-insensitively, that identify a
// provider back-pressure / rate-limit error. Kept broad but specific enough to
// avoid matching ordinary tool errors (ADR-0025 D3 rate-limit fallback).
var rateLimitSignals = []string{
	"rate limit",
	"rate-limit",
	"ratelimit",
	"too many requests",
	"429",
	"overloaded",
	"quota exceeded",
	"resource_exhausted",
	"resource exhausted",
	"insufficient_quota",
	"throttl", // throttle / throttled / throttling
}

// looksRateLimited reports whether an episode/turn error message is shaped like
// a provider rate-limit or back-pressure error. Declarative and offline: it
// reads only the recorded error string, so any past run can be re-analyzed.
func looksRateLimited(msg string) bool {
	if strings.TrimSpace(msg) == "" {
		return false
	}
	lower := strings.ToLower(msg)
	for _, sig := range rateLimitSignals {
		if strings.Contains(lower, sig) {
			return true
		}
	}
	return false
}
