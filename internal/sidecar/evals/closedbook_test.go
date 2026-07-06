package evals

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestClosedBookSessionMetaSerializesEmptyTools proves the ADR-0016.9
// invariant: the closed-book session meta must serialize tools and
// allowedTools as explicit empty arrays. An omitted tools field means "all
// tools" to the adapter, which would silently turn the closed-book probe
// into an open-book run.
func TestClosedBookSessionMetaSerializesEmptyTools(t *testing.T) {
	data, err := json.Marshal(ClosedBookSessionMeta("claude-sonnet-5"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	for _, want := range []string{
		`"tools":[]`,
		`"allowedTools":[]`,
		`"settingSources":[]`,
		`"strictMcpConfig":true`,
		`"thinking":{"type":"disabled"}`,
		`"model":"claude-sonnet-5"`,
		`"emitRawSDKMessages":true`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("closed-book session meta missing %s\ngot: %s", want, got)
		}
	}
}

// TestClosedBookSessionMetaOmitsUnknownModel: an unresolved subject model must
// not be forwarded as the literal string "unknown".
func TestClosedBookSessionMetaOmitsUnknownModel(t *testing.T) {
	for _, model := range []string{"", "unknown"} {
		data, err := json.Marshal(ClosedBookSessionMeta(model))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"model"`) {
			t.Errorf("model %q must be omitted from session meta, got: %s", model, data)
		}
	}
}

// TestClosedBookEpisodeRejectsToolCalls drives RunClosedBookEpisode against
// the scripted mock ACP agent emitting a tool call: the episode must record a
// closed-book violation and the run must fail (ADR-0016.9 verification plan).
func TestClosedBookEpisodeRejectsToolCalls(t *testing.T) {
	bin := buildMockAgent(t)
	script := []map[string]any{
		{"toolCalls": []string{"ghx read gin-gonic/gin gin.go"}, "text": "answer with a tool"},
	}
	writeScript(t, script)

	task := Task{
		ID:    "closed-book-mock",
		Repo:  "gin-gonic/gin",
		Turns: []string{"Where is routing implemented?"},
		Checks: TaskChecks{
			ExpectedFiles: []string{"tree.go"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ep, err := RunClosedBookEpisode(ctx, RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()}, task, 1)
	if err == nil {
		t.Fatal("expected closed-book tool-call violation error, got nil")
	}
	if !strings.Contains(err.Error(), "closed-book") {
		t.Fatalf("error should name the closed-book contract, got: %v", err)
	}
	if ep == nil {
		t.Fatal("failed closed-book episode must still return the partial artifact")
	}
	if len(ep.Violations) == 0 {
		t.Fatal("closed-book tool activity must be recorded as a violation")
	}
	if ep.Rewards.Safety != 0 {
		t.Fatalf("violated episode must score safety=0, got %v", ep.Rewards.Safety)
	}
}

// TestClosedBookEpisodeCapturesTextOnly: a compliant no-tool reply produces a
// scored closed-book episode whose correctness comes from the same
// answerText/checks path as open-book episodes.
func TestClosedBookEpisodeCapturesTextOnly(t *testing.T) {
	bin := buildMockAgent(t)
	script := []map[string]any{
		{"text": "Routing lives in tree.go and routergroup.go via addRoute."},
	}
	writeScript(t, script)

	task := Task{
		ID:    "closed-book-mock",
		Repo:  "gin-gonic/gin",
		Turns: []string{"Where is routing implemented?"},
		Checks: TaskChecks{
			ExpectedFiles:   []string{"tree.go", "gin.go"},
			ExpectedSymbols: []string{"addRoute"},
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ep, err := RunClosedBookEpisode(ctx, RunConfig{AgentCmd: bin, SessionsDir: t.TempDir()}, task, 1)
	if err != nil {
		t.Fatalf("closed-book episode failed: %v", err)
	}
	if ep.Profile != ProfileClosedBook {
		t.Fatalf("profile = %q, want %q", ep.Profile, ProfileClosedBook)
	}
	if ep.ID != "closed-book-mock_closed-book_trial-1" {
		t.Fatalf("episode id = %q, want ADR-0016.9 layout", ep.ID)
	}
	if len(ep.Turns) != 1 || ep.Turns[0].Text == "" {
		t.Fatalf("expected one text-bearing turn, got %+v", ep.Turns)
	}
	if ep.Report != nil || ep.Turns[0].Report != nil {
		t.Fatal("closed-book episodes must not carry reports")
	}

	// Scorer parity (ADR-0016.9 verification plan): the closed-book episode
	// scores identically to an offline episode built from the same fixed text
	// through the shared ComputeRewards path.
	offline := &Episode{
		TaskID:  task.ID,
		Profile: ProfileClosedBook,
		Checks:  task.Checks,
		Turns:   []TurnRecord{{Turn: 0, Question: task.Turns[0], Text: ep.Turns[0].Text}},
	}
	want := ComputeRewards(task, offline)
	if ep.Rewards.Correctness != want.Correctness {
		t.Fatalf("correctness mismatch: live %v vs offline %v", ep.Rewards.Correctness, want.Correctness)
	}
	// tree.go + addRoute hit, gin.go missed: (1/2 + 1/1) / 2 = 0.75.
	if ep.Rewards.Correctness != 0.75 {
		t.Fatalf("correctness = %v, want 0.75", ep.Rewards.Correctness)
	}
}

// TestComputeClosedBookSummaryLabels checks the pre-registered ADR-0016.9
// threshold labels and the exploration-signal arithmetic.
func TestComputeClosedBookSummaryLabels(t *testing.T) {
	tasks := []Task{
		{ID: "recall-task", Repo: "a/b", Turns: []string{"q"}, Checks: TaskChecks{RequiredClaims: []string{"x"}}},
		{ID: "exploration-task", Repo: "c/d", Turns: []string{"q"}, Checks: TaskChecks{RequiredClaims: []string{"x"}}},
	}
	closed := []*Episode{
		{ID: "recall-task_closed-book_trial-1", TaskID: "recall-task", Profile: ProfileClosedBook, Rewards: RewardBreakdown{Correctness: 1.0}},
		{ID: "recall-task_closed-book_trial-2", TaskID: "recall-task", Profile: ProfileClosedBook, Rewards: RewardBreakdown{Correctness: 0.9}},
		{ID: "exploration-task_closed-book_trial-1", TaskID: "exploration-task", Profile: ProfileClosedBook, Rewards: RewardBreakdown{Correctness: 0.2}},
		{ID: "exploration-task_closed-book_trial-2", TaskID: "exploration-task", Profile: ProfileClosedBook, Rewards: RewardBreakdown{Correctness: 0.0}},
	}
	identity := AgentIdentity{SubjectModel: "claude-sonnet-5"}
	open := []*Episode{
		{ID: "o1", TaskID: "recall-task", Profile: ProfilePlain, Identity: identity, Rewards: RewardBreakdown{Correctness: 1.0}},
		{ID: "o2", TaskID: "recall-task", Profile: ProfileGhx, Identity: identity, Rewards: RewardBreakdown{Correctness: 1.0}},
		{ID: "o3", TaskID: "recall-task", Profile: ProfileSidecar, Identity: identity, Rewards: RewardBreakdown{Correctness: 1.0}},
		{ID: "o4", TaskID: "exploration-task", Profile: ProfilePlain, Identity: identity, Rewards: RewardBreakdown{Correctness: 0.8}},
		{ID: "o5", TaskID: "exploration-task", Profile: ProfileGhx, Identity: identity, Rewards: RewardBreakdown{Correctness: 0.9}},
		{ID: "o6", TaskID: "exploration-task", Profile: ProfileSidecar, Identity: identity, Rewards: RewardBreakdown{Correctness: 1.0}},
	}

	summary := ComputeClosedBookSummary("run-x", 2, closed, open, "open-run", tasks)
	if len(summary.Tasks) != 2 {
		t.Fatalf("want 2 task summaries, got %d", len(summary.Tasks))
	}
	recall := summary.Tasks[0]
	if recall.TaskID != "recall-task" {
		t.Fatalf("task order should follow the corpus, got %q first", recall.TaskID)
	}
	if recall.MeanCorrectness != 0.95 || recall.MaxCorrectness != 1.0 {
		t.Fatalf("recall stats wrong: %+v", recall)
	}
	wantLabels := []string{"recall-dominated", "low-exploration-signal", "memorization-risk"}
	if strings.Join(recall.Labels, ",") != strings.Join(wantLabels, ",") {
		t.Fatalf("recall labels = %v, want %v", recall.Labels, wantLabels)
	}
	if recall.OpenBook == nil || recall.OpenBook.PooledMeanCorrectness != 1.0 {
		t.Fatalf("open-book ref wrong: %+v", recall.OpenBook)
	}

	expl := summary.Tasks[1]
	if expl.MeanCorrectness != 0.1 {
		t.Fatalf("exploration mean = %v, want 0.1", expl.MeanCorrectness)
	}
	if got := expl.ExplorationSignal; got < 0.79 || got > 0.81 {
		t.Fatalf("exploration signal = %v, want ~0.8", got)
	}
	if strings.Join(expl.Labels, ",") != "exploration-bearing" {
		t.Fatalf("exploration labels = %v, want [exploration-bearing]", expl.Labels)
	}
}
