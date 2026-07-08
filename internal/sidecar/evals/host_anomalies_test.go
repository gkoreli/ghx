package evals

import (
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
)

// hostEpisodeFixture builds a synthetic arm-B host episode whose persisted
// fields exercise the offline detectors.
func hostEpisodeFixture(arm HostArm) *Episode {
	return &Episode{
		ID:      "fx_host_1",
		TaskID:  "fx",
		Profile: ProfileHostSidecar,
		Turns:   []TurnRecord{{Turn: 0, Question: "fix it"}},
		HostTask: &HostTaskRecord{
			FixtureID:       "fx",
			Arm:             arm,
			ReconToolTitles: HostReconToolTitles(),
		},
	}
}

func anomalyKinds(anoms []Anomaly) []string {
	var out []string
	for _, a := range anoms {
		out = append(out, a.Kind)
	}
	return out
}

func countKind(anoms []Anomaly, kind string) int {
	n := 0
	for _, a := range anoms {
		if a.Kind == kind {
			n++
		}
	}
	return n
}

func TestHostExternalExplorationDetector(t *testing.T) {
	ep := hostEpisodeFixture(HostArmSidecar)
	ep.HostTask.Attribution = hosttask.AttributionTable{
		PerToolCall: []hosttask.AttributedToolCall{
			// The recon tool itself is exploration but compliant.
			{ID: "t1", Title: "mcp__ghx-recon__recon", Kind: "other", Class: hosttask.ClassExploration, Chars: 900},
			// Hand exploration: an execute-kind gh call — the violation.
			{ID: "t2", Title: "Terminal", Kind: "execute", Class: hosttask.ClassExploration, Chars: 300},
			// Engineering and unclassified rows never fire.
			{ID: "t3", Title: "Terminal", Kind: "execute", Class: hosttask.ClassEngineering, Chars: 100},
			{ID: "t4", Title: "mystery", Kind: "other", Class: hosttask.ClassUnclassified, Chars: 10},
		},
	}
	anoms := DetectAnomalies(ep)
	if got := countKind(anoms, AnomalyHostExternalExploration); got != 1 {
		t.Fatalf("host_external_exploration count = %d, want 1 (kinds=%v)", got, anomalyKinds(anoms))
	}
	for _, a := range anoms {
		if a.Kind == AnomalyHostExternalExploration {
			if a.Severity != SeveritySoft {
				t.Fatalf("severity = %s, want soft (flag and report, never block)", a.Severity)
			}
			if !strings.Contains(a.Detail, "kind=execute") {
				t.Fatalf("detail should carry the offending call identity: %q", a.Detail)
			}
		}
	}

	// The control arm explores natively by design: same table, no anomaly.
	ep.HostTask.Arm = HostArmControl
	ep.Profile = ProfileHostControl
	ep.HostTask.ReconToolTitles = nil
	if got := countKind(DetectAnomalies(ep), AnomalyHostExternalExploration); got != 0 {
		t.Fatalf("control arm must never fire host_external_exploration, got %d", got)
	}
}

func TestHostRateLimitedDetector(t *testing.T) {
	for _, arm := range []HostArm{HostArmControl, HostArmSidecar} {
		ep := hostEpisodeFixture(arm)
		ep.Turns[0].Error = "host turn: API error 429 rate_limit_error: rate limited"
		anoms := DetectAnomalies(ep)
		if got := countKind(anoms, AnomalyHostRateLimited); got != 1 {
			t.Fatalf("arm %s: host_rate_limited count = %d, want 1 (kinds=%v)", arm, got, anomalyKinds(anoms))
		}
		for _, a := range anoms {
			if a.Kind == AnomalyHostRateLimited && a.Severity != SeverityBreaking {
				t.Fatalf("host_rate_limited severity = %s, want breaking (D4 BLOCKED row)", a.Severity)
			}
		}
	}

	// Recon episodes (no HostTask) never derive host rows even on the same error.
	recon := &Episode{Profile: ProfileGhx, Turns: []TurnRecord{{Turn: 0, Error: "429 rate limited"}}}
	if got := countKind(DetectAnomalies(recon), AnomalyHostRateLimited); got != 0 {
		t.Fatalf("recon episode fired host_rate_limited %d times", got)
	}
}

func TestReconToolUnavailableDetector(t *testing.T) {
	// Marker in the turn error.
	ep := hostEpisodeFixture(HostArmSidecar)
	ep.Turns[0].Error = "host turn 0: MCP server ghx-recon: connection closed"
	if got := countKind(DetectAnomalies(ep), AnomalyReconToolUnavailable); got != 1 {
		t.Fatalf("turn-error marker: count = %d, want 1", got)
	}

	// Marker in a recon-titled tool call's output.
	ep = hostEpisodeFixture(HostArmSidecar)
	ep.Turns[0].ToolTraces = []sidecar.ToolCallTrace{{
		ID: "t1", Title: "mcp__ghx-recon__recon", Kind: "other",
		OutputExcerpt: "Error: No such tool available: mcp__ghx-recon__recon",
	}}
	anoms := DetectAnomalies(ep)
	if got := countKind(anoms, AnomalyReconToolUnavailable); got != 1 {
		t.Fatalf("recon-output marker: count = %d, want 1 (kinds=%v)", got, anomalyKinds(anoms))
	}
	for _, a := range anoms {
		if a.Kind == AnomalyReconToolUnavailable && a.Severity != SeverityBreaking {
			t.Fatalf("severity = %s, want breaking (D4 BLOCKED row)", a.Severity)
		}
	}

	// The same marker on a NON-recon tool call must not fire — the scan is
	// scoped so unrelated command output cannot false-positive.
	ep = hostEpisodeFixture(HostArmSidecar)
	ep.Turns[0].ToolTraces = []sidecar.ToolCallTrace{{
		ID: "t1", Title: "Terminal", Kind: "execute",
		OutputExcerpt: "curl: (7) failed to connect to host",
	}}
	if got := countKind(DetectAnomalies(ep), AnomalyReconToolUnavailable); got != 0 {
		t.Fatalf("non-recon output must not fire, got %d", got)
	}

	// Control arm never fires it: there is no recon tool to be missing.
	ep = hostEpisodeFixture(HostArmControl)
	ep.HostTask.ReconToolTitles = nil
	ep.Turns[0].Error = "MCP server: connection closed"
	if got := countKind(DetectAnomalies(ep), AnomalyReconToolUnavailable); got != 0 {
		t.Fatalf("control arm must not fire recon_tool_unavailable, got %d", got)
	}
}

func TestHostExecuteOutsideWorkspaceDetector(t *testing.T) {
	const ws = "/eval/workspaces/fx-trial001-abc"
	execTrace := func(id, cmd string) sidecar.ToolCallTrace {
		return sidecar.ToolCallTrace{ID: id, Title: "Terminal", Kind: "execute", RawInput: map[string]any{"command": cmd}}
	}
	cases := []struct {
		name  string
		trace sidecar.ToolCallTrace
		want  int
	}{
		{"shell write escape via python", execTrace("t1", `python3 -c 'open("/tmp/pwned","w").write("x")'`), 1},
		{"redirect outside", execTrace("t2", "echo data > /Users/goga/notes.txt"), 1},
		{"workspace paths are fine", execTrace("t3", "go test "+ws+"/pkg"), 0},
		{"system binaries are benign", execTrace("t4", "/usr/bin/env go test ./..."), 0},
		{"dev null is benign", execTrace("t5", "go test ./... > /dev/null"), 0},
		{"sed patterns do not match", execTrace("t6", "sed s/foo/bar/ file.txt"), 0},
		{"relative paths cannot be checked", execTrace("t7", "cp secret.txt ../outside/"), 0},
		{"non-execute kinds are ignored", sidecar.ToolCallTrace{ID: "t8", Kind: "read", Title: "/tmp/pwned", Locations: []string{"/tmp/pwned"}}, 0},
	}
	for _, arm := range []HostArm{HostArmControl, HostArmSidecar} {
		for _, tc := range cases {
			t.Run(string(arm)+"/"+tc.name, func(t *testing.T) {
				ep := hostEpisodeFixture(arm)
				ep.HostTask.WorkspaceDir = ws
				ep.Turns[0].ToolTraces = []sidecar.ToolCallTrace{tc.trace}
				if got := countKind(DetectAnomalies(ep), AnomalyHostExecuteOutsideWorkspace); got != tc.want {
					t.Fatalf("count = %d, want %d", got, tc.want)
				}
			})
		}
	}
}

func TestCountAnomaliesIncludesHostTaskRows(t *testing.T) {
	ep := hostEpisodeFixture(HostArmSidecar)
	ep.Turns[0].Error = "429 rate limited by provider"
	ep.HostTask.Attribution = hosttask.AttributionTable{
		PerToolCall: []hosttask.AttributedToolCall{
			{ID: "t1", Title: "Terminal", Kind: "execute", Class: hosttask.ClassExploration, Chars: 10},
		},
	}
	counts := CountAnomalies([]*Episode{ep})
	got := map[string]AnomalySeverity{}
	for _, c := range counts {
		got[c.Kind] = c.Severity
	}
	if got[AnomalyHostRateLimited] != SeverityBreaking {
		t.Fatalf("host_rate_limited missing or wrong severity: %v", counts)
	}
	if got[AnomalyHostExternalExploration] != SeveritySoft {
		t.Fatalf("host_external_exploration missing or wrong severity: %v", counts)
	}
}
