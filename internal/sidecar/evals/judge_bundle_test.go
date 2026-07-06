package evals

import (
	"encoding/json"
	"strings"
	"testing"
)

// loadJudgeFixture loads one committed per-profile episode fixture.
func loadJudgeFixture(t *testing.T, profile string) *Episode {
	t.Helper()
	ep, err := LoadEpisode("testdata/judge/episode_" + profile + ".json")
	if err != nil {
		t.Fatalf("load %s fixture: %v", profile, err)
	}
	return ep
}

// forbiddenBundleStrings are metadata values that must never reach the judge
// (ADR-0023.1 D2 profile-blindness). Each is a value from the fixtures'
// identity block or a scoring field name, verified absent from legitimate
// content before this test was written.
var forbiddenBundleStrings = []string{
	"eval-agent-acp.sh",                     // agent command
	"@agentclientprotocol/claude-agent-acp", // adapter name
	"claude-sonnet-5",                       // subject model
	"71350b8a61263d3d99278e3a3e94a80600d282f22942c7527f2a3b9fcbf67a4b", // wrapper sha
	"ghx-sidecar",          // profile label
	"\"profile\"",          // profile field
	"\"identity\"",         // identity field
	"\"rewards\"",          // reward scores
	"\"correctness\"",      // reward component
	"\"violations\"",       // safety violations
	"\"anomalies\"",        // anomaly taxonomy
	"\"exclusionReasons\"", // compliance exclusions
}

func TestBuildJudgeBundleIsProfileBlind(t *testing.T) {
	for _, profile := range []string{"plain", "ghx", "sidecar"} {
		t.Run(profile, func(t *testing.T) {
			ep := loadJudgeFixture(t, profile)
			bundle := BuildJudgeBundle(ep)

			data, err := json.MarshalIndent(bundle, "", "  ")
			if err != nil {
				t.Fatalf("marshal bundle: %v", err)
			}
			blob := string(data)

			for _, forbidden := range forbiddenBundleStrings {
				if strings.Contains(blob, forbidden) {
					t.Errorf("bundle leaks forbidden metadata %q — profile-blindness violated (ADR-0023.1 D2)", forbidden)
				}
			}

			// The bundle must still carry the judge-relevant content.
			if bundle.TaskID == "" || bundle.Repo == "" {
				t.Errorf("bundle missing task/repo context: taskID=%q repo=%q", bundle.TaskID, bundle.Repo)
			}
			if len(bundle.Questions) == 0 {
				t.Errorf("bundle has no questions")
			}
			if len(bundle.Turns) != len(ep.Turns) {
				t.Errorf("bundle turns=%d, episode turns=%d", len(bundle.Turns), len(ep.Turns))
			}
			// Every profile in the fixtures made tool calls.
			toolCalls := 0
			for _, tr := range bundle.Turns {
				toolCalls += len(tr.ToolCalls)
			}
			if toolCalls == 0 {
				t.Errorf("bundle recorded no tool calls for %s", profile)
			}
		})
	}
}

func TestBuildJudgeBundleReportPresenceMatchesProfile(t *testing.T) {
	// Only the sidecar profile produces a structured report; the direct
	// profiles carry their answer in per-turn AgentText.
	if b := BuildJudgeBundle(loadJudgeFixture(t, "sidecar")); b.Report == nil {
		t.Errorf("sidecar bundle should carry a structured report")
	} else if b.Report.Answer == "" {
		t.Errorf("sidecar bundle report has empty answer")
	}
	for _, profile := range []string{"plain", "ghx"} {
		if b := BuildJudgeBundle(loadJudgeFixture(t, profile)); b.Report != nil {
			t.Errorf("%s bundle should have no structured report", profile)
		}
	}
}

func TestBuildJudgeBundleSummarizesToolOutput(t *testing.T) {
	// Tool output must be a bounded summary, never the full payload (D2).
	ep := loadJudgeFixture(t, "ghx")
	bundle := BuildJudgeBundle(ep)
	for _, tr := range bundle.Turns {
		for _, tc := range tr.ToolCalls {
			if len(tc.OutputSummary) > bundleToolSummryMax {
				t.Errorf("tool output summary length %d exceeds bound %d", len(tc.OutputSummary), bundleToolSummryMax)
			}
			// Summary must never exceed the true output size it summarizes.
			if tc.OutputChars > 0 && len(tc.OutputSummary) > tc.OutputChars {
				t.Errorf("summary longer than the output it summarizes (%d > %d)", len(tc.OutputSummary), tc.OutputChars)
			}
		}
	}
}
