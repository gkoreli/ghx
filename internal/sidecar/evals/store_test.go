package evals

import (
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func TestSaveAndLoadEpisodeRoundtrip(t *testing.T) {
	dir := t.TempDir()
	ep := &Episode{
		ID:      "t1_ghx-sidecar_123",
		TaskID:  "t1",
		Repo:    "honojs/hono",
		Profile: ProfileSidecar,
		Turns: []TurnRecord{
			{Turn: 0, Question: "q", Text: "a", ToolCalls: []string{"ghx explore honojs/hono (completed)"}},
		},
		Report:    &sidecar.Report{Answer: "found"},
		Context:   ContextAccounting{MainAgentChars: 10, SidecarInternalChars: 90, TotalWorkflowChars: 100},
		Rewards:   RewardBreakdown{Correctness: 1, Overall: 0.8},
		StartedAt: time.Now().UTC().Truncate(time.Second),
		EndedAt:   time.Now().UTC().Truncate(time.Second),
	}

	path, err := SaveEpisode(dir, ep)
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadEpisode(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != ep.ID || got.Profile != ProfileSidecar || got.Report.Answer != "found" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if got.Rewards.Correctness != 1 {
		t.Errorf("rewards not preserved: %+v", got.Rewards)
	}
	if got.Context.SidecarInternalChars != 90 {
		t.Errorf("context not preserved: %+v", got.Context)
	}
}
