package evals

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// This file implements the ADR-0016.9 closed-book memorization probe.
//
// The probe answers one question per task: how much of the task's
// deterministic correctness can the subject model score from its weights
// alone, with no tools and no repository access? The resulting closed-book
// correctness is a contamination measurement, not a new leaderboard — see
// docs/adr/0016.9-memorization-confound-audit.md for the pre-registered
// interpretation rules.

// ProfileClosedBook labels ADR-0016.9 memorization-probe episodes. It is
// deliberately NOT part of AllProfiles(): closed-book episodes are audit
// artifacts, never gate inputs.
const ProfileClosedBook Profile = "closed-book"

// ClosedBookSessionMeta builds the _meta payload for a closed-book ACP
// session. It is a raw map, not sidecar.SessionOptions, because that struct
// marshals Tools/AllowedTools with omitempty — an empty []string would be
// omitted, and an omitted tools field means "all tools" (ADR-0016.9
// implementation note; internal/sidecar/session_options.go). A closed-book
// run where tools is omitted is invalid, so the empty arrays must serialize
// explicitly as "tools":[] and "allowedTools":[].
func ClosedBookSessionMeta(model string) map[string]any {
	options := map[string]any{
		// No host settings, no MCP fallback: same isolation as sidecar runs.
		"settingSources":  []string{},
		"strictMcpConfig": true,
		// Closed book: no built-in tools, no auto-approved tools. Explicit
		// empty arrays — see the doc comment above.
		"tools":        []string{},
		"allowedTools": []string{},
		// No extended thinking: the probe measures plain recall.
		"thinking": map[string]any{"type": "disabled"},
	}
	if model != "" && model != "unknown" {
		options["model"] = model
	}
	return map[string]any{
		"claudeCode": map[string]any{
			"options":            options,
			"emitRawSDKMessages": true,
		},
	}
}

// closedBookViolation describes tool activity observed on a closed-book turn.
// Any tool activity is a harness failure per ADR-0016.9: the probe is only
// valid when the answer came from model weights.
func closedBookViolation(rec TurnRecord) string {
	switch {
	case len(rec.ToolTraces) > 0:
		return fmt.Sprintf("closed-book: %d tool call(s) observed on turn %d (first: %s)",
			len(rec.ToolTraces), rec.Turn, rec.ToolTraces[0].Title)
	case len(rec.ToolCalls) > 0:
		return fmt.Sprintf("closed-book: tool call summary recorded on turn %d: %s", rec.Turn, rec.ToolCalls[0])
	case rec.ToolOutputChars > 0:
		return fmt.Sprintf("closed-book: %d tool output chars observed on turn %d", rec.ToolOutputChars, rec.Turn)
	}
	return ""
}

// RunClosedBookEpisode runs one ADR-0016.9 closed-book trial for a task:
// same subject agent command as live evals, but the ACP session is created
// with an explicit empty tool surface (ClosedBookSessionMeta) and each task
// turn is sent as the bare question text — no persona, no direct-profile
// preamble, no repo metadata beyond what the question itself says.
//
// The returned episode carries only assistant text; it is scored with the
// same ComputeRewards path as open-book episodes. Any observed tool activity
// is recorded as a violation and returned as an error (harness failure).
func RunClosedBookEpisode(ctx context.Context, cfg RunConfig, task Task, trial int) (*Episode, error) {
	cwd, err := os.MkdirTemp("", "ghx-eval-closedbook-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(cwd) }()

	ep := &Episode{
		// ADR-0016.9 artifact layout: <task-id>_closed-book_trial-<n>.json.
		ID:        fmt.Sprintf("%s_closed-book_trial-%d", task.ID, trial),
		TaskID:    task.ID,
		Repo:      task.Repo,
		Profile:   ProfileClosedBook,
		Checks:    task.Checks,
		StartedAt: time.Now().UTC(),
		Identity:  agentIdentity(cfg, nil),
	}

	runErr := runClosedBookTurns(ctx, cfg, task, ep, cwd)

	ep.EndedAt = time.Now().UTC()
	finalizeContext(ep)
	ep.Rewards = ComputeRewards(task, ep)
	ep.Anomalies = DetectAnomalies(ep)
	return ep, runErr
}

func runClosedBookTurns(ctx context.Context, cfg RunConfig, task Task, ep *Episode, cwd string) error {
	cmd := exec.CommandContext(ctx, cfg.AgentCmd)
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", cfg.AgentCmd, err)
	}
	defer sidecar.ShutdownAgent(cmd, stdin)

	client := &evalClient{}
	conn := acp.NewClientSideConnection(client, stdin, stdout)

	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{ReadTextFile: false, WriteTextFile: false},
		},
	})
	if err != nil {
		return fmt.Errorf("acp initialize: %w", err)
	}
	if initResp.AgentInfo != nil {
		ep.Identity.AdapterName = initResp.AgentInfo.Name
		ep.Identity.AdapterVersion = initResp.AgentInfo.Version
		ep.Identity.AdapterSubjectModel = modelFromMeta(initResp.AgentInfo.Meta)
	}

	sess, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: []acp.McpServer{},
		Meta:       ClosedBookSessionMeta(resolveSubjectModel(cfg.AgentCmd)),
	})
	if err != nil {
		return fmt.Errorf("acp new session: %w", err)
	}

	for i, q := range task.Turns {
		record := TurnRecord{Turn: i, Question: q, Resumed: i > 0}
		client.current = &record
		client.promptSent = true
		start := time.Now()
		// The exact eval question text, nothing else (ADR-0016.9 Decision).
		_, err := conn.Prompt(ctx, acp.PromptRequest{
			SessionId: sess.SessionId,
			Prompt:    []acp.ContentBlock{acp.TextBlock(q)},
		})
		record.DurationMs = time.Since(start).Milliseconds()
		client.current = nil

		if err != nil {
			record.Error = err.Error()
			ep.Turns = append(ep.Turns, record)
			ep.Violations = append(ep.Violations, client.violations...)
			return fmt.Errorf("closed-book turn %d: %w", i, err)
		}
		ep.Turns = append(ep.Turns, record)
		if v := closedBookViolation(record); v != "" {
			ep.Violations = append(ep.Violations, v)
			ep.Violations = append(ep.Violations, client.violations...)
			return fmt.Errorf("closed-book contract broken: %s", v)
		}
	}

	ep.Violations = append(ep.Violations, client.violations...)
	if len(client.violations) > 0 {
		return fmt.Errorf("closed-book contract broken: %s", client.violations[0])
	}
	return nil
}

// ── summary (ADR-0016.9 Artifacts section) ─────────────────────────────────

// ClosedBookOpenBookRef is the committed open-book reference a closed-book
// score is compared against. Means come from gate-style valid episodes only
// (same exclusion rules as EvaluateGates).
type ClosedBookOpenBookRef struct {
	// RunID names the committed source run (e.g. gate-run-2026-07-06-fixbatch).
	RunID string `json:"runId"`
	// MeanCorrectnessByProfile is the per-profile open-book correctness mean
	// for this task.
	MeanCorrectnessByProfile map[Profile]float64 `json:"meanCorrectnessByProfile"`
	// PooledMeanCorrectness is the unweighted mean of the profile means — the
	// single "open-book mean" the pre-registered thresholds compare against.
	PooledMeanCorrectness float64 `json:"pooledMeanCorrectness"`
}

// ClosedBookTaskSummary is the per-task record required by ADR-0016.9.
type ClosedBookTaskSummary struct {
	TaskID            string                 `json:"taskId"`
	Repo              string                 `json:"repo"`
	Trials            int                    `json:"trials"`
	MeanCorrectness   float64                `json:"meanCorrectness"`
	MedianCorrectness float64                `json:"medianCorrectness"`
	MaxCorrectness    float64                `json:"maxCorrectness"`
	StdDevCorrectness float64                `json:"stdDevCorrectness"`
	TrialCorrectness  []float64              `json:"trialCorrectness"`
	EpisodeIDs        []string               `json:"episodeIds"`
	OpenBook          *ClosedBookOpenBookRef `json:"openBook,omitempty"`
	// ExplorationSignal = open-book pooled mean − closed-book mean.
	ExplorationSignal float64 `json:"explorationSignal"`
	// ExplorationSignalByProfile breaks the signal down per open-book profile.
	ExplorationSignalByProfile map[Profile]float64 `json:"explorationSignalByProfile,omitempty"`
	// Labels are the pre-registered threshold labels from ADR-0016.9.
	Labels []string `json:"labels"`
}

// ClosedBookSummary is the run-level closed-book-summary.json payload.
type ClosedBookSummary struct {
	ADR           string                  `json:"adr"`
	RunID         string                  `json:"runId"`
	GeneratedAt   time.Time               `json:"generatedAt"`
	TrialsPerTask int                     `json:"trialsPerTask"`
	OpenBookRunID string                  `json:"openBookRunId,omitempty"`
	Tasks         []ClosedBookTaskSummary `json:"tasks"`
}

// OpenBookReferenceMeans computes per-task, per-profile open-book correctness
// means from a committed run's episodes, applying the same validity filter as
// the gate verdict (invalid/compliance/contamination episodes excluded).
func OpenBookReferenceMeans(eps []*Episode) map[string]map[Profile][]float64 {
	filtered, _, _ := validateEpisodesForVerdict(eps)
	out := map[string]map[Profile][]float64{}
	for _, ep := range filtered {
		if ep.DiscoveryChecks != nil {
			continue // discovery episodes are a different eval class
		}
		byProfile := out[ep.TaskID]
		if byProfile == nil {
			byProfile = map[Profile][]float64{}
			out[ep.TaskID] = byProfile
		}
		byProfile[ep.Profile] = append(byProfile[ep.Profile], ep.Rewards.Correctness)
	}
	return out
}

// closedBookLabels applies the pre-registered ADR-0016.9 threshold table.
// openMean < 0 means "no open-book reference available" and skips the
// gap-based labels.
func closedBookLabels(meanC, maxC, openMean float64) []string {
	var labels []string
	if meanC >= 0.80 {
		labels = append(labels, "recall-dominated")
	}
	if openMean >= 0 && math.Abs(openMean-meanC) <= 0.10 {
		labels = append(labels, "low-exploration-signal")
	}
	if maxC == 1.00 && meanC >= 0.60 {
		labels = append(labels, "memorization-risk")
	}
	if openMean >= 0 && meanC < 0.40 && openMean-meanC >= 0.30 {
		labels = append(labels, "exploration-bearing")
	}
	return labels
}

// ComputeClosedBookSummary folds closed-book episodes plus an open-book
// reference run into the per-task summary required by ADR-0016.9. openEps
// may be nil when no reference run is supplied; gap-based fields are then
// omitted.
func ComputeClosedBookSummary(runID string, trialsPerTask int, closedEps []*Episode, openEps []*Episode, openRunID string, tasks []Task) ClosedBookSummary {
	repoByTask := map[string]string{}
	taskOrder := make([]string, 0, len(tasks))
	for _, t := range tasks {
		repoByTask[t.ID] = t.Repo
		taskOrder = append(taskOrder, t.ID)
	}

	byTask := map[string][]*Episode{}
	for _, ep := range closedEps {
		if ep.Profile != ProfileClosedBook {
			continue
		}
		byTask[ep.TaskID] = append(byTask[ep.TaskID], ep)
	}

	var openMeans map[string]map[Profile][]float64
	if len(openEps) > 0 {
		openMeans = OpenBookReferenceMeans(openEps)
	}

	summary := ClosedBookSummary{
		ADR:           "ADR-0016.9",
		RunID:         runID,
		GeneratedAt:   time.Now().UTC(),
		TrialsPerTask: trialsPerTask,
		OpenBookRunID: openRunID,
	}

	for _, taskID := range taskOrder {
		eps := byTask[taskID]
		if len(eps) == 0 {
			continue
		}
		sort.Slice(eps, func(i, j int) bool { return eps[i].ID < eps[j].ID })
		var scores []float64
		var ids []string
		for _, ep := range eps {
			scores = append(scores, ep.Rewards.Correctness)
			ids = append(ids, ep.ID)
		}

		ts := ClosedBookTaskSummary{
			TaskID:            taskID,
			Repo:              repoByTask[taskID],
			Trials:            len(eps),
			MeanCorrectness:   mean(scores),
			MedianCorrectness: median(scores),
			MaxCorrectness:    maxOf(scores),
			StdDevCorrectness: stdDev(scores),
			TrialCorrectness:  scores,
			EpisodeIDs:        ids,
		}

		openMean := -1.0
		if profileScores, ok := openMeans[taskID]; ok {
			ref := &ClosedBookOpenBookRef{RunID: openRunID, MeanCorrectnessByProfile: map[Profile]float64{}}
			var profileMeans []float64
			signals := map[Profile]float64{}
			for profile, vals := range profileScores {
				m := mean(vals)
				ref.MeanCorrectnessByProfile[profile] = m
				profileMeans = append(profileMeans, m)
				signals[profile] = m - ts.MeanCorrectness
			}
			ref.PooledMeanCorrectness = mean(profileMeans)
			openMean = ref.PooledMeanCorrectness
			ts.OpenBook = ref
			ts.ExplorationSignal = openMean - ts.MeanCorrectness
			ts.ExplorationSignalByProfile = signals
		}
		ts.Labels = closedBookLabels(ts.MeanCorrectness, ts.MaxCorrectness, openMean)
		summary.Tasks = append(summary.Tasks, ts)
	}
	return summary
}

func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	s := append([]float64(nil), vals...)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}

func maxOf(vals []float64) float64 {
	out := 0.0
	for _, v := range vals {
		if v > out {
			out = v
		}
	}
	return out
}

func stdDev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	m := mean(vals)
	sum := 0.0
	for _, v := range vals {
		sum += (v - m) * (v - m)
	}
	return math.Sqrt(sum / float64(len(vals)))
}

// ClosedBookManifest is the manifest.json payload for a closed-book audit run
// (ADR-0016.9 Artifacts section).
type ClosedBookManifest struct {
	ADR              string        `json:"adr"`
	RunID            string        `json:"runId"`
	GeneratedAt      time.Time     `json:"generatedAt"`
	GitCommit        string        `json:"gitCommit"`
	TaskCorpusSHA256 string        `json:"taskCorpusSha256"`
	Identity         AgentIdentity `json:"identity"`
	TrialsPerTask    int           `json:"trialsPerTask"`
	// SessionMetaJSON is the exact serialized _meta sent on session/new —
	// committed proof that "tools":[] and "allowedTools":[] were explicit.
	SessionMetaJSON string `json:"sessionMetaJson"`
	// ScoringCodeCommit is the commit whose ComputeRewards scored these
	// episodes (same as GitCommit for a clean tree).
	ScoringCodeCommit string `json:"scoringCodeCommit"`
	OpenBookRunID     string `json:"openBookRunId,omitempty"`
	OpenBookRunDir    string `json:"openBookRunDir,omitempty"`
}

// SaveClosedBookJSON writes one pretty-printed JSON artifact under dir.
func SaveClosedBookJSON(dir, name string, v any) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}
