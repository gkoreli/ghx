package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// judgeResultSchemaVersion versions the committed per-episode JudgeResult
// artifact (ADR-0023.1 D2/D4).
const judgeResultSchemaVersion = "judge-result-v1"

// defaultJudgeSamples is the k in k=3 self-consistency (ADR-0023.1 D4,
// cost-effective baseline per ADR-0023 Q6).
const defaultJudgeSamples = 3

// DimensionScore is one core-rubric dimension scored by a single judge call.
type DimensionScore struct {
	Dimension   string `json:"dimension"`
	Score       int    `json:"score"`
	Explanation string `json:"explanation,omitempty"`
}

// JudgeVerdict is the output of ONE judge model invocation (one self-
// consistency sample). Scores are on the core rubric's 1–5 scale.
type JudgeVerdict struct {
	Dimensions  []DimensionScore `json:"dimensions"`
	Overall     float64          `json:"overall"`
	Explanation string           `json:"explanation,omitempty"`
}

// JudgeClient is the thin seam behind which a real judge model plugs in
// (ADR-0023.1 D4). Everything else in this package — bundle building, prompt
// rendering, aggregation, emission — is deterministic and offline; only this
// interface reaches a model. No real (network) implementation ships yet; the
// stub/fake below cover the offline machinery.
type JudgeClient interface {
	// Evaluate runs one judge invocation over a fully rendered prompt.
	Evaluate(ctx context.Context, prompt string) (JudgeVerdict, error)
	// ModelID identifies the judge model for committed artifacts (e.g.
	// "gpt-5.5-2026-05"). It is recorded on every JudgeResult (D4).
	ModelID() string
}

type judgeRuntimeVersionClient interface {
	RuntimeVersion() string
}

// AggregatedDim is one dimension's self-consistency aggregate across k samples.
// Explanation is the reasoning from the sample whose score is nearest the
// median — a representative rationale carried into the emitted evaluation event
// (ADR-0023.1 D6, gen_ai.evaluation.explanation).
type AggregatedDim struct {
	Dimension    string  `json:"dimension"`
	MedianScore  float64 `json:"medianScore"`
	Scores       []int   `json:"scores"`
	Disagreement int     `json:"disagreement"` // max − min across samples
	Explanation  string  `json:"explanation,omitempty"`
}

// JudgeResult is the committed per-episode judge artifact (ADR-0023.1 D2).
// It carries the aggregated scores, the raw k samples for audit, the exact
// prompt/model/rubric versions, and the profile-blind bundle that was scored —
// so every judgment is recomputable from committed files. Calibrated is false
// until D5 passes; no score is citable before then.
type JudgeResult struct {
	SchemaVersion string `json:"schemaVersion"`
	EpisodeID     string `json:"episodeId"`
	TaskID        string `json:"taskId"`

	// Skipped records the anomaly pre-filter decision (D7). When true, no
	// judge call was made and SkipReason explains why.
	Skipped    bool   `json:"skipped"`
	SkipReason string `json:"skipReason,omitempty"`

	JudgeModelID        string `json:"judgeModelId,omitempty"`
	JudgeRuntimeVersion string `json:"judgeRuntimeVersion,omitempty"`
	PromptVersion       string `json:"promptVersion,omitempty"`
	RubricVersion       string `json:"rubricVersion,omitempty"`
	Samples             int    `json:"samples,omitempty"`

	Dimensions      []AggregatedDim `json:"dimensions,omitempty"`
	Overall         float64         `json:"overall,omitempty"`
	MaxDisagreement int             `json:"maxDisagreement,omitempty"`

	Verdicts []JudgeVerdict `json:"verdicts,omitempty"`
	Bundle   *JudgeBundle   `json:"bundle,omitempty"`

	// Calibrated is always false until the D5 calibration gate passes. It is a
	// structural reminder that these scores are PRELIMINARY and not citable.
	Calibrated bool      `json:"calibrated"`
	ScoredAt   time.Time `json:"scoredAt"`
}

// JudgeRunner scores episodes with a JudgeClient using the committed rubric,
// prompt, and k-sample self-consistency (ADR-0023.1 D3/D4/D7).
type JudgeRunner struct {
	Client  JudgeClient
	Samples int // k; defaults to defaultJudgeSamples when <= 0
	// now is injectable for deterministic tests; defaults to time.Now().UTC().
	now func() time.Time
}

// NewJudgeRunner builds a runner with the default k and clock.
func NewJudgeRunner(client JudgeClient) *JudgeRunner {
	return &JudgeRunner{Client: client, Samples: defaultJudgeSamples}
}

func (r *JudgeRunner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now().UTC()
}

// Score judges one episode. It applies the anomaly pre-filter first (returning
// a Skipped result with the reason, never an error, for excluded episodes),
// then renders the prompt, runs k judge samples, and aggregates by median per
// dimension with disagreement recorded.
func (r *JudgeRunner) Score(ctx context.Context, task Task, ep *Episode) (*JudgeResult, error) {
	res := &JudgeResult{
		SchemaVersion: judgeResultSchemaVersion,
		EpisodeID:     ep.ID,
		TaskID:        ep.TaskID,
		Calibrated:    false,
		ScoredAt:      r.clock(),
	}

	if skip, reason := judgePrefilter(ep); skip {
		res.Skipped = true
		res.SkipReason = reason
		return res, nil
	}

	bundle := BuildJudgeBundle(ep)
	prompt, err := RenderJudgePrompt(task, bundle)
	if err != nil {
		return nil, fmt.Errorf("render judge prompt for %s: %w", ep.ID, err)
	}

	k := r.Samples
	if k <= 0 {
		k = defaultJudgeSamples
	}

	verdicts := make([]JudgeVerdict, 0, k)
	for i := 0; i < k; i++ {
		v, err := r.Client.Evaluate(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("judge sample %d/%d for %s: %w", i+1, k, ep.ID, err)
		}
		verdicts = append(verdicts, v)
	}

	res.JudgeModelID = r.Client.ModelID()
	if versioned, ok := r.Client.(judgeRuntimeVersionClient); ok {
		res.JudgeRuntimeVersion = versioned.RuntimeVersion()
	}
	res.PromptVersion = judgePromptVersion
	res.RubricVersion = rubricVersion
	res.Samples = k
	res.Verdicts = verdicts
	res.Bundle = &bundle
	res.Dimensions, res.Overall, res.MaxDisagreement = aggregateVerdicts(verdicts)
	return res, nil
}

// judgePrefilter implements the ADR-0023.1 D7 scope guard: BLOCKED and
// WARN-noreport episodes have nothing to judge and are excluded, with the
// reason recorded. Invalid (compliance-excluded) episodes are also skipped —
// scoring an excluded episode would dilute aggregates. Safety stays a gate,
// never a judgment.
func judgePrefilter(ep *Episode) (skip bool, reason string) {
	if ep.Invalid {
		r := "episode excluded from the run (invalid)"
		if len(ep.ExclusionReasons) > 0 {
			r = "episode excluded: " + ep.ExclusionReasons[0]
		}
		return true, r
	}
	for _, a := range DetectAnomalies(ep) {
		switch a.Kind {
		case AnomalySidecarBlocked:
			return true, "BLOCKED episode — zero exploration to judge (D7)"
		case AnomalySidecarReportMissing:
			return true, "WARN-noreport episode — no report to judge (D7)"
		}
	}
	return false, ""
}

// aggregateVerdicts computes the median per dimension across the k samples and
// records the per-dimension disagreement (max − min). Overall is the median of
// the sample overalls; MaxDisagreement is the largest dimension spread. The
// dimension order is fixed by the core rubric so results are stable.
func aggregateVerdicts(verdicts []JudgeVerdict) (dims []AggregatedDim, overall float64, maxDisagreement int) {
	if len(verdicts) == 0 {
		return nil, 0, 0
	}
	for _, name := range coreRubric.dimensionNames() {
		var scores []int
		type sample struct {
			score       int
			explanation string
		}
		var samples []sample
		for _, v := range verdicts {
			for _, d := range v.Dimensions {
				if d.Dimension == name {
					scores = append(scores, d.Score)
					samples = append(samples, sample{d.Score, d.Explanation})
				}
			}
		}
		if len(scores) == 0 {
			continue
		}
		spread := maxInt(scores) - minInt(scores)
		if spread > maxDisagreement {
			maxDisagreement = spread
		}
		median := medianInts(scores)
		// Representative explanation: from the sample whose score is nearest the
		// median (ties resolve to the earliest sample).
		explanation := ""
		best := -1.0
		for _, s := range samples {
			d := float64(s.score) - median
			if d < 0 {
				d = -d
			}
			if best < 0 || d < best {
				best = d
				explanation = s.explanation
			}
		}
		dims = append(dims, AggregatedDim{
			Dimension:    name,
			MedianScore:  median,
			Scores:       scores,
			Disagreement: spread,
			Explanation:  explanation,
		})
	}

	overalls := make([]float64, len(verdicts))
	for i, v := range verdicts {
		overalls[i] = v.Overall
	}
	overall = medianFloats(overalls)
	return dims, overall, maxDisagreement
}

func medianInts(v []int) float64 {
	f := make([]float64, len(v))
	for i, x := range v {
		f[i] = float64(x)
	}
	return medianFloats(f)
}

func medianFloats(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

func maxInt(v []int) int {
	m := v[0]
	for _, x := range v[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func minInt(v []int) int {
	m := v[0]
	for _, x := range v[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

// SaveJudgeResult writes one JudgeResult artifact as pretty-printed JSON under
// runDir as <episode-id>.judge.json, next to the episode it scores (D2).
func SaveJudgeResult(runDir string, res *JudgeResult) (string, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", runDir, err)
	}
	path := filepath.Join(runDir, res.EpisodeID+".judge.json")
	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadJudgeResult reads one JudgeResult artifact back from disk.
func LoadJudgeResult(path string) (*JudgeResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var res JudgeResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &res, nil
}

// ── stub / fake judge clients (tests + placeholder; NO network) ──────────────

// FixedJudgeClient always returns the same verdict. It is the stand-in judge
// used until a real cross-family JudgeClient ships (ADR-0023.1 D4) and the
// simplest fake for tests that do not exercise disagreement.
type FixedJudgeClient struct {
	Model   string
	Verdict JudgeVerdict
}

// Evaluate returns the fixed verdict, ignoring the prompt.
func (c FixedJudgeClient) Evaluate(context.Context, string) (JudgeVerdict, error) {
	return c.Verdict, nil
}

// ModelID reports the configured model identifier.
func (c FixedJudgeClient) ModelID() string { return c.Model }

// ScriptedJudgeClient returns queued verdicts in order, cycling if Evaluate is
// called more times than there are verdicts. It exists to test k-sample
// aggregation and disagreement recording deterministically.
type ScriptedJudgeClient struct {
	Model    string
	Verdicts []JudgeVerdict
	calls    int
}

// Evaluate returns the next scripted verdict.
func (c *ScriptedJudgeClient) Evaluate(context.Context, string) (JudgeVerdict, error) {
	if len(c.Verdicts) == 0 {
		return JudgeVerdict{}, fmt.Errorf("scripted judge has no verdicts")
	}
	v := c.Verdicts[c.calls%len(c.Verdicts)]
	c.calls++
	return v, nil
}

// ModelID reports the configured model identifier.
func (c *ScriptedJudgeClient) ModelID() string { return c.Model }

// NewVerdict is a small constructor for building test/stub verdicts from
// (dimension, score) pairs in core-rubric order, with the overall set to the
// mean of the dimension scores.
func NewVerdict(scores map[string]int, explanation string) JudgeVerdict {
	v := JudgeVerdict{Explanation: explanation}
	sum := 0
	n := 0
	for _, name := range coreRubric.dimensionNames() {
		s, ok := scores[name]
		if !ok {
			continue
		}
		v.Dimensions = append(v.Dimensions, DimensionScore{Dimension: name, Score: s})
		sum += s
		n++
	}
	if n > 0 {
		v.Overall = float64(sum) / float64(n)
	}
	return v
}
