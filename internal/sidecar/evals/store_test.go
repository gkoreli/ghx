package evals

import (
	"reflect"
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

// TestSaveEpisodePersistsCompleteAnomalies is the ADR-0016.8 D6 regression:
// detectors run at persistence time, so re-deriving anomalies from any
// persisted episode equals its stored field — even when the caller never set
// (or staled) ep.Anomalies. The 2026-07-05 audit found two committed episodes
// with detector-derivable anomalies missing from their stored field.
func TestSaveEpisodePersistsCompleteAnomalies(t *testing.T) {
	dir := t.TempDir()
	episodes := []*Episode{
		{ // WARN report + unparsed block, anomalies never set by the caller
			ID:      "warn_ghx-sidecar_1",
			Profile: ProfileSidecar,
			Turns: []TurnRecord{{
				Turn:   0,
				Text:   `oops <ghx-report>{"answer":42}</ghx-report>`,
				Report: &sidecar.Report{Answer: "WARN: sidecar did not emit a <ghx-report> block"},
			}},
		},
		{ // stale anomalies field must be overwritten, not trusted
			ID:        "stale_ghx-sidecar_2",
			Profile:   ProfileSidecar,
			Anomalies: []Anomaly{{Kind: "bogus_leftover", Severity: SeveritySoft}},
			Turns:     []TurnRecord{{Turn: 0, Report: &sidecar.Report{Answer: "fine"}}},
		},
		{ // contamination guard (ADR-0016.8 D2) derives from snapshotted checks
			ID:      "contam_plain_3",
			Profile: ProfilePlain,
			Checks:  TaskChecks{ContaminationPaths: []string{"docs/adr/"}},
			Turns: []TurnRecord{{
				Turn:      0,
				ToolCalls: []string{"gh api repos/o/r/contents/docs/adr/0013.md (completed)"},
			}},
		},
		{ // clean episode: derived nil equals stored nil
			ID:      "clean_ghx_4",
			Profile: ProfileGhx,
			Turns:   []TurnRecord{{Turn: 0, ToolCalls: []string{"ghx read o/r a.go (completed)"}}},
		},
	}
	for _, ep := range episodes {
		path, err := SaveEpisode(dir, ep)
		if err != nil {
			t.Fatal(err)
		}
		got, err := LoadEpisode(path)
		if err != nil {
			t.Fatal(err)
		}
		if derived := DetectAnomalies(got); !reflect.DeepEqual(derived, got.Anomalies) {
			t.Errorf("%s: re-derived anomalies %#v != stored %#v", ep.ID, derived, got.Anomalies)
		}
	}
}

// TestRecordManifestRoundAccumulates pins ADR-0016.8 D5: the manifest carries
// the full planned tasks × profiles × trials matrix; multi-round accumulation
// appends rounds instead of overwriting; identity refresh never drops rounds.
func TestRecordManifestRoundAccumulates(t *testing.T) {
	dir := t.TempDir()
	round := PlannedRound{Tasks: 6, Profiles: 3, Trials: 1}
	id := AgentIdentity{AgentCommand: "agent", SubjectModel: "sonnet"}

	m, err := RecordManifestRound(dir, round, id)
	if err != nil {
		t.Fatal(err)
	}
	if m.ExpectedEpisodes != 18 || len(m.Rounds) != 1 || m.Rounds[0].ExpectedEpisodes != 18 {
		t.Fatalf("first round: %+v", m)
	}
	for i := 0; i < 4; i++ {
		if m, err = RecordManifestRound(dir, round, id); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.Rounds) != 5 || m.ExpectedEpisodes != 90 {
		t.Fatalf("five rounds must accumulate to 90 expected episodes, got %+v", m)
	}
	if err := UpdateManifestIdentity(dir, AgentIdentity{AgentCommand: "agent", SubjectModel: "verified"}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRunManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Rounds) != 5 || loaded.ExpectedEpisodes != 90 {
		t.Fatalf("identity refresh dropped planned rounds: %+v", loaded)
	}
	if loaded.Identity.SubjectModel != "verified" {
		t.Fatalf("identity not refreshed: %+v", loaded.Identity)
	}
	if loaded.ExpectedEpisodesOverride != 0 {
		t.Fatalf("override recorded without the env escape hatch: %+v", loaded)
	}
}

// TestRecordManifestRoundEnvOverride pins the GHX_EVAL_EXPECTED_EPISODES
// escape hatch (ADR-0016.8 D5): it wins over the summed rounds AND is
// recorded in the manifest so provenance shows a human asserted the total.
func TestRecordManifestRoundEnvOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(ExpectedEpisodesEnv, "90")
	m, err := RecordManifestRound(dir, PlannedRound{Tasks: 6, Profiles: 3, Trials: 1}, AgentIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	if m.ExpectedEpisodes != 90 {
		t.Fatalf("expectedEpisodes = %d, want the 90 override", m.ExpectedEpisodes)
	}
	if m.ExpectedEpisodesOverride != 90 {
		t.Fatalf("override must be recorded in the manifest, got %+v", m)
	}
	if len(m.Rounds) != 1 || m.Rounds[0].ExpectedEpisodes != 18 {
		t.Fatalf("planned round must still record the real matrix: %+v", m.Rounds)
	}
}
