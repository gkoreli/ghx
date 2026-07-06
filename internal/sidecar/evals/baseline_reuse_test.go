package evals

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestBaselineReuseHashInventoryStability(t *testing.T) {
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "test-sonnet")
	taskDir, tasks := baselineTaskDir(t, map[string]string{
		"a.json": `{"id":"a","repo":"o/r","turns":["Where is dispatch implemented?"],"checks":{"expectedFiles":["a.go"]}}`,
	})
	wrapper := writeWrapper(t, "wrapper bytes v1\n")
	cfg := RunConfig{AgentCmd: wrapper}
	id := AgentIdentity{AdapterVersion: "adapter-v1", SubjectModel: "test-sonnet"}

	first, reason, err := BaselineReuseHashes(cfg, tasks, taskDir, id)
	if err != nil || reason != "" {
		t.Fatalf("hashes: reason=%q err=%v", reason, err)
	}
	second, reason, err := BaselineReuseHashes(cfg, tasks, taskDir, id)
	if err != nil || reason != "" {
		t.Fatalf("hashes second: reason=%q err=%v", reason, err)
	}
	if hashValue(first, "wrapperScript") != hashValue(second, "wrapperScript") ||
		hashValue(first, "taskCorpus") != hashValue(second, "taskCorpus") ||
		hashValue(first, "directProfilePreamble") != hashValue(second, "directProfilePreamble") {
		t.Fatalf("stable inputs produced unstable hashes:\n%+v\n%+v", first, second)
	}

	wrapper2 := writeWrapper(t, "wrapper bytes v2\n")
	changedWrapper, _, err := BaselineReuseHashes(RunConfig{AgentCmd: wrapper2}, tasks, taskDir, id)
	if err != nil {
		t.Fatal(err)
	}
	if hashValue(first, "wrapperScript") == hashValue(changedWrapper, "wrapperScript") {
		t.Fatal("wrapper byte change must change wrapperScript")
	}
	for _, name := range []string{"adapterVersion", "subjectModel", "taskCorpus", "directProfilePreamble"} {
		if hashValue(first, name) != hashValue(changedWrapper, name) {
			t.Fatalf("wrapper byte change unexpectedly changed %s", name)
		}
	}

	if err := os.WriteFile(filepath.Join(taskDir, "a.json"), []byte(`{"id":"a","repo":"o/r","turns":["Where is dispatch implemented?"],"checks":{"expectedFiles":["a.go"],"expectedSymbols":["composeChain"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tasksChanged, err := LoadTasks(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	changedTask, _, err := BaselineReuseHashes(cfg, tasksChanged, taskDir, id)
	if err != nil {
		t.Fatal(err)
	}
	if hashValue(first, "taskCorpus") == hashValue(changedTask, "taskCorpus") {
		t.Fatal("task JSON byte change must change taskCorpus")
	}
	if err := os.Rename(filepath.Join(taskDir, "a.json"), filepath.Join(taskDir, "renamed.json")); err != nil {
		t.Fatal(err)
	}
	renamedTask, err := taskCorpusHash(taskDir)
	if err != nil {
		t.Fatal(err)
	}
	if hashValue(changedTask, "taskCorpus") == renamedTask {
		t.Fatal("task filename change must change taskCorpus")
	}

	oldBuilder := directPreambleBuilder
	directPreambleBuilder = func(p Profile, repo string) string { return oldBuilder(p, repo) + "\nTEST HOOK" }
	t.Cleanup(func() { directPreambleBuilder = oldBuilder })
	changedPreamble, _, err := BaselineReuseHashes(cfg, tasks, taskDir, id)
	if err != nil {
		t.Fatal(err)
	}
	if hashValue(first, "directProfilePreamble") == hashValue(changedPreamble, "directProfilePreamble") {
		t.Fatal("direct preamble text change must change directProfilePreamble")
	}

	_, reason, err = BaselineReuseHashes(cfg, tasks, taskDir, AgentIdentity{AdapterVersion: "adapter-v1", SubjectModel: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reason, "subject model is unknown") {
		t.Fatalf("unknown subject model reason = %q", reason)
	}
}

func TestTaskCorpusHashOrderingAndRawBytes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writes := map[string]string{
		"b.json":   `{"id":"b","repo":"o/r","turns":["Where is beta implemented?"],"checks":{"expectedFiles":["b.go"]}}`,
		"a.json":   `{"id":"a","repo":"o/r","turns":["Where is alpha implemented?"],"checks":{"expectedFiles":["a.go"]}}`,
		"note.txt": "ignored",
	}
	for name, text := range writes {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	first, err := taskCorpusHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	second, err := taskCorpusHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("task corpus hash not stable: %s != %s", first, second)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.json"), []byte(writes["a.json"]+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := taskCorpusHash(dir)
	if err != nil {
		t.Fatal(err)
	}
	if first == changed {
		t.Fatal("raw JSON formatting change must affect task corpus hash")
	}
}

func TestBaselineReuseManifestRoundtrip(t *testing.T) {
	dir := t.TempDir()
	reuse := &BaselineReuse{
		ReusedFromRunID: "prior-run",
		MaxAgeDays:      BaselineReuseMaxAgeDays,
		VerdictLabel:    BaselineReuseLabel,
		Hashes:          []BaselineReuseHashRecord{{Name: "wrapperScript", Algorithm: "sha256", Value: "abc", Source: "agent", SourceKind: "file"}},
		Episodes:        []BaselineReusedEpisodeRecord{{Profile: ProfilePlain, TaskID: "a", SourceFile: "src.json", TargetFile: "dst.json", SHA256: "def"}},
	}
	if err := SaveRunManifest(dir, RunManifest{BaselineReuse: reuse}); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadRunManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.BaselineReuse == nil || loaded.BaselineReuse.Hashes[0].Name != "wrapperScript" || loaded.BaselineReuse.Episodes[0].SHA256 != "def" {
		t.Fatalf("baselineReuse did not roundtrip: %+v", loaded.BaselineReuse)
	}
	if _, err := RecordManifestRound(dir, PlannedRound{Tasks: 1, Profiles: 3, Trials: 1}, AgentIdentity{SubjectModel: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := UpdateManifestIdentity(dir, AgentIdentity{SubjectModel: "verified"}); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadRunManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.BaselineReuse == nil || loaded.BaselineReuse.VerdictLabel != BaselineReuseLabel {
		t.Fatalf("manifest updates dropped baselineReuse: %+v", loaded)
	}
}

func TestTryReuseBaselinesEligibilityAndCopySemantics(t *testing.T) {
	fixture := newBaselineReuseFixture(t, 1)
	reuse, ok, reason, err := TryReuseBaselines(fixture.targetRun, fixture.priorRun, fixture.cfg, fixture.tasks, fixture.taskDir, 1, fixture.identity, fixture.now)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("reuse rejected: %s", reason)
	}
	if len(reuse.Episodes) != 4 {
		t.Fatalf("copied episodes = %d, want 4", len(reuse.Episodes))
	}
	for _, rec := range reuse.Episodes {
		src, err := os.ReadFile(rec.SourceFile)
		if err != nil {
			t.Fatal(err)
		}
		dst, err := os.ReadFile(rec.TargetFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(src) != string(dst) {
			t.Fatalf("%s was not copied byte-for-byte", rec.TargetFile)
		}
		if rec.SHA256 != sha256Hex(dst) {
			t.Fatalf("manifest sha = %s, want %s", rec.SHA256, sha256Hex(dst))
		}
	}
	if _, err := LoadRunEpisodes(fixture.targetRun); err != nil {
		t.Fatalf("target run must load without source dir: %v", err)
	}
}

func TestTryReuseBaselinesMismatchTable(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*baselineReuseFixture)
		want   string
	}{
		{name: "hash mismatch", mutate: func(f *baselineReuseFixture) { f.priorManifest.BaselineReuse.Hashes[0].Value = "wrong" }, want: "hash mismatch"},
		{name: "old run", mutate: func(f *baselineReuseFixture) { f.priorManifest.CreatedAt = f.now.Add(-8 * 24 * time.Hour) }, want: "older than 7 days"},
		{name: "missing inventory", mutate: func(f *baselineReuseFixture) { f.priorManifest.BaselineReuse = nil }, want: "lacks baselineReuse hash inventory"},
		{name: "adapter mismatch", mutate: func(f *baselineReuseFixture) { f.priorManifest.Identity.AdapterVersion = "other" }, want: "adapterVersion mismatch"},
		{name: "blank adapter", mutate: func(f *baselineReuseFixture) { f.identity.AdapterVersion = "" }, want: "adapter version is blank"},
		{name: "unknown subject", mutate: func(f *baselineReuseFixture) { f.identity.SubjectModel = "unknown" }, want: "subject model is unknown"},
		{name: "missing plain", mutate: func(f *baselineReuseFixture) { os.Remove(filepath.Join(f.priorRun, "a_plain_0.json")) }, want: "incomplete baseline matrix"},
		{name: "only one profile", mutate: func(f *baselineReuseFixture) {
			for _, name := range []string{"a_ghx_0.json", "b_ghx_0.json"} {
				os.Remove(filepath.Join(f.priorRun, name))
			}
		}, want: "incomplete baseline matrix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newBaselineReuseFixture(t, 1)
			tt.mutate(f)
			if f.priorManifest != nil {
				if err := SaveRunManifest(f.priorRun, *f.priorManifest); err != nil {
					t.Fatal(err)
				}
			}
			_, ok, reason, err := TryReuseBaselines(f.targetRun, f.priorRun, f.cfg, f.tasks, f.taskDir, 1, f.identity, f.now)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				t.Fatal("reuse unexpectedly accepted")
			}
			if !strings.Contains(reason, tt.want) {
				t.Fatalf("reason = %q, want contains %q", reason, tt.want)
			}
		})
	}
}

func TestBaselineReuseVerdictLabeling(t *testing.T) {
	v := EvaluateGates(gateRunEpisodes())
	LabelBaselineReused(&v, &BaselineReuse{ReusedFromRunID: "run-a", VerdictLabel: BaselineReuseLabel})
	if !containsString(v.Labels, BaselineReuseLabel) {
		t.Fatalf("labels = %+v, want %s", v.Labels, BaselineReuseLabel)
	}
	md := FormatVerdict(v)
	if !strings.Contains(md, "**BASELINE-REUSED**") || !strings.Contains(md, "BASELINE-REUSED: plain and ghx episodes copied from run-a") {
		t.Fatalf("verdict markdown missing baseline reuse label/note:\n%s", md)
	}
	fresh := EvaluateGates(gateRunEpisodes())
	if containsString(fresh.Labels, BaselineReuseLabel) || strings.Contains(FormatVerdict(fresh), "BASELINE-REUSED") {
		t.Fatal("fresh run must not carry baseline reuse label")
	}
}

type baselineReuseFixture struct {
	taskDir       string
	tasks         []Task
	cfg           RunConfig
	identity      AgentIdentity
	priorRun      string
	targetRun     string
	now           time.Time
	priorManifest *RunManifest
}

func newBaselineReuseFixture(t *testing.T, trials int) *baselineReuseFixture {
	t.Helper()
	t.Setenv("GHX_EVAL_SUBJECT_MODEL", "test-sonnet")
	taskDir, tasks := baselineTaskDir(t, map[string]string{
		"a.json": `{"id":"a","repo":"o/r","turns":["Where is alpha implemented?"],"checks":{"expectedFiles":["a.go"]}}`,
		"b.json": `{"id":"b","repo":"o/r","turns":["Where is beta implemented?"],"checks":{"expectedFiles":["b.go"]}}`,
	})
	cfg := RunConfig{AgentCmd: writeWrapper(t, "wrapper bytes\n")}
	wrapperSHA, ok := wrapperHash(cfg.AgentCmd)
	if !ok {
		t.Fatal("wrapper hash missing")
	}
	identity := AgentIdentity{
		AgentCommand:   cfg.AgentCmd,
		AdapterVersion: "adapter-v1",
		SubjectModel:   "test-sonnet",
		WrapperSHA256:  wrapperSHA,
	}
	hashes, reason, err := BaselineReuseHashes(cfg, tasks, taskDir, identity)
	if err != nil || reason != "" {
		t.Fatalf("hashes reason=%q err=%v", reason, err)
	}
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	priorRun := t.TempDir()
	targetRun := t.TempDir()
	for trial := 0; trial < trials; trial++ {
		for _, task := range tasks {
			for _, profile := range []Profile{ProfilePlain, ProfileGhx} {
				ep := &Episode{
					ID:       task.ID + "_" + string(profile) + "_" + strconv.Itoa(trial),
					TaskID:   task.ID,
					Repo:     task.Repo,
					Profile:  profile,
					Identity: identity,
					Turns:    []TurnRecord{{Turn: 0}},
					Rewards:  RewardBreakdown{Correctness: 1, Evidence: 1, Safety: 1},
				}
				if _, err := SaveEpisode(priorRun, ep); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	manifest := &RunManifest{
		RunDir:    priorRun,
		Identity:  identity,
		CreatedAt: now.Add(-time.Hour),
		BaselineReuse: &BaselineReuse{
			ReusedFromRunID: "seed",
			MaxAgeDays:      BaselineReuseMaxAgeDays,
			VerdictLabel:    BaselineReuseLabel,
			Hashes:          hashes,
		},
	}
	if err := SaveRunManifest(priorRun, *manifest); err != nil {
		t.Fatal(err)
	}
	return &baselineReuseFixture{
		taskDir:       taskDir,
		tasks:         tasks,
		cfg:           cfg,
		identity:      identity,
		priorRun:      priorRun,
		targetRun:     targetRun,
		now:           now,
		priorManifest: manifest,
	}
}

func baselineTaskDir(t *testing.T, files map[string]string) (string, []Task) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := LoadTasks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return dir, tasks
}

func writeWrapper(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "agent.sh")
	if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
