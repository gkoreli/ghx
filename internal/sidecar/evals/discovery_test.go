package evals

import (
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func TestLoadDiscoveryTasksFromTestdata(t *testing.T) {
	tasks, err := LoadDiscoveryTasks("testdata/discovery-tasks")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(tasks), 3; got != want {
		t.Fatalf("discovery tasks = %d, want %d", got, want)
	}
	wantTargets := map[string][]string{
		"openai-compatible-ai-gateways": {
			"berriai/litellm",
			"portkey-ai/gateway",
			"helicone/helicone",
		},
		"mcp-streamable-http-sdks": {
			"modelcontextprotocol/typescript-sdk",
			"modelcontextprotocol/python-sdk",
			"modelcontextprotocol/go-sdk",
			"modelcontextprotocol/java-sdk",
			"modelcontextprotocol/csharp-sdk",
			"modelcontextprotocol/kotlin-sdk",
			"modelcontextprotocol/rust-sdk",
			"modelcontextprotocol/php-sdk",
			"modelcontextprotocol/ruby-sdk",
			"modelcontextprotocol/swift-sdk",
		},
		"llm-eval-observability-platforms": {
			"arize-ai/phoenix",
			"langfuse/langfuse",
			"promptfoo/promptfoo",
			"confident-ai/deepeval",
		},
	}
	for _, task := range tasks {
		var got []string
		for _, target := range task.Checks.TargetRepos {
			repo, err := normalizeRepoSlug(target.Repo)
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, repo)
			if len(target.RequiredEvidence) == 0 {
				t.Fatalf("%s target %s missing requiredEvidence anchor", task.ID, target.Repo)
			}
		}
		if strings.Join(got, ",") != strings.Join(wantTargets[task.ID], ",") {
			t.Fatalf("%s targets = %#v, want %#v", task.ID, got, wantTargets[task.ID])
		}
	}
}

func TestDiscoveryTaskValidateRejectsInvalid(t *testing.T) {
	task := discoveryScorerTask()
	for name, mutate := range map[string]func(*DiscoveryTask){
		"wrong kind":      func(t *DiscoveryTask) { t.Kind = "repo" },
		"too few targets": func(t *DiscoveryTask) { t.Checks.TargetRepos = t.Checks.TargetRepos[:2] },
		"bad alternate":   func(t *DiscoveryTask) { t.Checks.AcceptableAlternates[0].Policy = "post-hoc" },
		"blank contamination": func(t *DiscoveryTask) {
			t.Checks.ContaminationPaths = []string{" "}
		},
	} {
		t.Run(name, func(t *testing.T) {
			invalid := task
			mutate(&invalid)
			if err := invalid.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDiscoveryScorerMatrix(t *testing.T) {
	task := discoveryScorerTask()
	for name, tc := range map[string]struct {
		ep          *Episode
		verified    float64
		named       float64
		precision   float64
		namedPrec   float64
		honesty     float64
		familiarity float64
	}{
		"verified target": {
			ep:          discoveryEpisode("o/a", "README.md", "ghx read o/a README.md (completed)", sidecar.Claim{Summary: "o/a is verified", Evidence: "o/a:README.md"}),
			verified:    1.0 / 3.0,
			named:       1.0 / 3.0,
			precision:   1,
			namedPrec:   1,
			honesty:     1,
			familiarity: 0,
		},
		"named only target": {
			ep:          discoveryTextEpisode("o/a belongs in the answer, but I did not verify it."),
			verified:    0,
			named:       1.0 / 3.0,
			precision:   0,
			namedPrec:   1,
			honesty:     1,
			familiarity: 1.0 / 3.0,
		},
		"verified alternate accepted extra": {
			ep:          discoveryEpisode("o/alt", "README.md", "ghx read o/alt README.md (completed)", sidecar.Claim{Summary: "o/alt is also valid", Evidence: "o/alt:README.md"}),
			verified:    0,
			named:       0,
			precision:   1,
			namedPrec:   1,
			honesty:     1,
			familiarity: 0,
		},
		"excluded repo penalizes named precision": {
			ep:          discoveryTextEpisode("o/bad is a candidate."),
			verified:    0,
			named:       0,
			precision:   0,
			namedPrec:   0,
			honesty:     1,
			familiarity: 0,
		},
		"miss": {
			ep:          discoveryTextEpisode("No relevant repositories found."),
			verified:    0,
			named:       0,
			precision:   0,
			namedPrec:   0,
			honesty:     1,
			familiarity: 0,
		},
		"dishonest verified claim without read": {
			ep:          discoveryReportOnly(sidecar.Claim{Summary: "o/a is verified", Evidence: "o/a:README.md"}),
			verified:    0,
			named:       1.0 / 3.0,
			precision:   0,
			namedPrec:   1,
			honesty:     0,
			familiarity: 1.0 / 3.0,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := ComputeDiscoveryRewards(task, tc.ep)
			if got.VerifiedRecall != tc.verified {
				t.Fatalf("verified recall = %.3f, want %.3f", got.VerifiedRecall, tc.verified)
			}
			if got.NamedRecall != tc.named {
				t.Fatalf("named recall = %.3f, want %.3f", got.NamedRecall, tc.named)
			}
			if got.VerifiedPrecision != tc.precision {
				t.Fatalf("verified precision = %.3f, want %.3f", got.VerifiedPrecision, tc.precision)
			}
			if got.NamedPrecision != tc.namedPrec {
				t.Fatalf("named precision = %.3f, want %.3f", got.NamedPrecision, tc.namedPrec)
			}
			if got.InferenceHonesty != tc.honesty {
				t.Fatalf("inference honesty = %.3f, want %.3f", got.InferenceHonesty, tc.honesty)
			}
			if got.FamiliarityGap != tc.familiarity {
				t.Fatalf("familiarity gap = %.3f, want %.3f", got.FamiliarityGap, tc.familiarity)
			}
		})
	}
}

func TestDiscoveryGateThresholds(t *testing.T) {
	v := EvaluateDiscoveryGates(discoveryGateEpisodes())
	for _, gate := range v.Gates {
		if !gate.Pass {
			t.Fatalf("%s failed unexpectedly: %s", gate.ID, gate.Detail)
		}
	}
	if !v.DiscoverySupported {
		t.Fatal("discovery should be supported when all D-G gates pass on a sufficient sample")
	}
	if strings.Contains(FormatDiscoveryVerdict(v), "| G1 |") {
		t.Fatal("discovery verdict must not render repo-scoped G1-G5 gates")
	}

	for name, mutate := range map[string]func([]*Episode){
		"D-G1": func(eps []*Episode) {
			for _, ep := range eps {
				if ep.Profile == ProfileSidecar {
					ep.DiscoveryRewards.VerifiedRecall = 0.64
				}
			}
		},
		"D-G2": func(eps []*Episode) {
			for _, ep := range eps {
				if ep.Profile == ProfileSidecar {
					ep.DiscoveryRewards.VerifiedPrecision = 0.69
				}
			}
		},
		"D-G3": func(eps []*Episode) {
			for _, ep := range eps {
				if ep.Profile == ProfileSidecar {
					ep.DiscoveryRewards.Evidence = 0.74
				}
			}
		},
		"D-G4": func(eps []*Episode) {
			for _, ep := range eps {
				if ep.Profile == ProfileSidecar {
					ep.DiscoveryRewards.FamiliarityGap = 0.26
				}
			}
		},
		"D-G5": func(eps []*Episode) {
			for _, ep := range eps {
				if ep.Profile == ProfileSidecar {
					ep.Context.MainAgentChars = 4000
				}
			}
		},
		"D-G6": func(eps []*Episode) {
			for _, ep := range eps {
				if ep.Profile == ProfileSidecar {
					ep.DiscoveryRewards.Safety = 0.99
				}
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			eps := discoveryGateEpisodes()
			mutate(eps)
			v := EvaluateDiscoveryGates(eps)
			if gateByID(v.Gates, name).Pass {
				t.Fatalf("%s should fail: %+v", name, gateByID(v.Gates, name))
			}
			if v.DiscoverySupported {
				t.Fatal("discoverySupported must be false when any D-G gate fails")
			}
		})
	}
}

func TestDiscoveryContaminationExcluded(t *testing.T) {
	eps := discoveryGateEpisodes()
	contaminated := discoveryGateEpisode(ProfileGhx, 0)
	contaminated.ID = "contaminated"
	contaminated.Turns[0].ToolCalls = []string{"ghx read o/a docs/adr/answer.md (completed)"}
	eps = append(eps, contaminated)

	v := EvaluateDiscoveryGates(eps)
	if got := v.Aggregates[ProfileGhx].Episodes; got != minDiscoveryGateRunTasks*minDiscoveryGateRunTrials {
		t.Fatalf("ghx episodes = %d, contaminated episode should be excluded", got)
	}
	if !hasDiscoveryNote(v, "CONTAMINATION") {
		t.Fatalf("expected contamination note, got %v", v.Notes)
	}
}

func discoveryScorerTask() DiscoveryTask {
	return DiscoveryTask{
		ID:    "score",
		Kind:  "discovery",
		Turns: []string{"Which repos do the thing?"},
		Checks: DiscoveryChecks{
			TargetRepos: []DiscoveryTargetRepo{
				{Repo: "o/a", RequiredEvidence: []string{"README.md"}, Famous: true},
				{Repo: "o/b", RequiredEvidence: []string{"src/impl.go"}},
				{Repo: "o/c", RequiredEvidence: []string{"docs/usage.md"}},
			},
			AcceptableAlternates: []DiscoveryAlternateRepo{
				{Repo: "o/alt", Policy: "accepted-extra", Reason: "valid extra"},
			},
			ExcludedRepos: []DiscoveryExcludedRepo{
				{Repo: "o/bad", Reason: "false positive"},
			},
			ContaminationPaths: []string{"docs/adr/"},
		},
	}
}

func discoveryScorerChecks() *DiscoveryChecks {
	checks := discoveryScorerTask().Checks
	return &checks
}

func discoveryEpisode(repo, path, command string, claim sidecar.Claim) *Episode {
	return &Episode{
		TaskID:          "score",
		Profile:         ProfileSidecar,
		DiscoveryChecks: discoveryScorerChecks(),
		Turns: []TurnRecord{{
			Turn:      0,
			Text:      claim.Summary,
			ToolCalls: []string{command},
			Report:    &sidecar.Report{Answer: claim.Summary, Verified: []sidecar.Claim{claim}, CommandsRun: []string{command}},
		}},
		Report:  &sidecar.Report{Answer: claim.Summary, Verified: []sidecar.Claim{claim}, CommandsRun: []string{command}},
		Context: ContextAccounting{MainAgentChars: 800, TotalWorkflowChars: 4000},
	}
}

func discoveryReportOnly(claim sidecar.Claim) *Episode {
	return &Episode{
		TaskID:          "score",
		Profile:         ProfileSidecar,
		DiscoveryChecks: discoveryScorerChecks(),
		Turns: []TurnRecord{{
			Turn:   0,
			Text:   claim.Summary,
			Report: &sidecar.Report{Answer: claim.Summary, Verified: []sidecar.Claim{claim}},
		}},
		Report:  &sidecar.Report{Answer: claim.Summary, Verified: []sidecar.Claim{claim}},
		Context: ContextAccounting{MainAgentChars: 800, TotalWorkflowChars: 4000},
	}
}

func discoveryTextEpisode(text string) *Episode {
	return &Episode{
		TaskID:          "score",
		Profile:         ProfileGhx,
		DiscoveryChecks: discoveryScorerChecks(),
		Turns: []TurnRecord{{
			Turn:      0,
			Text:      text,
			ToolCalls: []string{"gh search repos thing"},
		}},
		Context: ContextAccounting{MainAgentChars: 2000, TotalWorkflowChars: 2000},
	}
}

func discoveryGateEpisodes() []*Episode {
	var eps []*Episode
	for task := 0; task < minDiscoveryGateRunTasks; task++ {
		for trial := 0; trial < minDiscoveryGateRunTrials; trial++ {
			eps = append(eps,
				discoveryGateEpisode(ProfileSidecar, task),
				discoveryGateEpisode(ProfileGhx, task),
				discoveryGateEpisode(ProfilePlain, task),
			)
		}
	}
	return eps
}

func discoveryGateEpisode(profile Profile, task int) *Episode {
	mainChars := 10000
	reward := DiscoveryRewardBreakdown{
		VerifiedRecall:    0.72,
		NamedRecall:       0.85,
		VerifiedPrecision: 0.80,
		NamedPrecision:    0.80,
		Evidence:          0.80,
		InferenceHonesty:  0.90,
		Compression:       0,
		Safety:            1,
		FamiliarityGap:    0.13,
	}
	command := "gh api repos/o/a/contents/README.md"
	if profile == ProfileSidecar {
		mainChars = 3000
		command = "ghx read o/a README.md (completed)"
		reward.Compression = 0.7
	}
	if profile == ProfilePlain {
		reward.VerifiedRecall = 0.60
		reward.NamedRecall = 0.70
	}
	return &Episode{
		ID:               string(profile) + "-discovery",
		TaskID:           "discovery-task-" + string(rune('a'+task)),
		Profile:          profile,
		DiscoveryChecks:  discoveryScorerChecks(),
		DiscoveryRewards: &reward,
		Context:          ContextAccounting{MainAgentChars: mainChars, TotalWorkflowChars: 10000},
		Turns: []TurnRecord{{
			Turn:            0,
			ToolCalls:       []string{command},
			ToolOutputChars: 1000,
		}},
	}
}

func hasDiscoveryNote(v DiscoveryVerdict, substr string) bool {
	for _, note := range v.Notes {
		if strings.Contains(note, substr) {
			return true
		}
	}
	return false
}
