package hosttask

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func testClassifier(t *testing.T) (Classifier, string) {
	t.Helper()
	scope, root := newTestScope(t)
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Classifier{
		Scope:               scope,
		ReconToolNames:      []string{"ghx_recon"},
		ExplorationCommands: DefaultExplorationCommands(),
	}, root
}

func TestClassifierRules(t *testing.T) {
	c, root := testClassifier(t)
	inside := filepath.Join(root, "main.go")
	newInside := filepath.Join(root, "pkg", "new.go")
	outside := filepath.Join(t.TempDir(), "elsewhere.go")

	cases := []struct {
		name  string
		trace sidecar.ToolCallTrace
		want  ToolCallClass
	}{
		// R1: recon MCP tool identity wins regardless of kind.
		{"recon MCP call", sidecar.ToolCallTrace{Title: "ghx_recon", Kind: "other"}, ClassExploration},
		{"recon title is exact match only", sidecar.ToolCallTrace{Title: "ghx_recon_extra", Kind: "other"}, ClassUnclassified},
		// R2: fetch is network by definition.
		{"web fetch", sidecar.ToolCallTrace{Kind: "fetch", Title: "Fetch https://example.com"}, ClassExploration},
		// R3: compound or empty commands are unclassifiable by identity.
		{"piped command", sidecar.ToolCallTrace{Kind: "execute", Title: "gh api repos/x | jq ."}, ClassUnclassified},
		{"chained command", sidecar.ToolCallTrace{Kind: "execute", Title: "cd /tmp && gh api repos/x"}, ClassUnclassified},
		{"subshell command", sidecar.ToolCallTrace{Kind: "execute", Title: "echo $(gh api x)"}, ClassUnclassified},
		{"empty command", sidecar.ToolCallTrace{Kind: "execute", Title: ""}, ClassUnclassified},
		// R4: exploration command identities.
		{"gh call", sidecar.ToolCallTrace{Kind: "execute", Title: "gh api repos/acme/chi"}, ClassExploration},
		{"curl call", sidecar.ToolCallTrace{Kind: "execute", Title: "curl -s https://x"}, ClassExploration},
		{"git fetch", sidecar.ToolCallTrace{Kind: "execute", Title: "git fetch origin main"}, ClassExploration},
		{"git clone", sidecar.ToolCallTrace{Kind: "execute", Title: "git clone https://github.com/a/b"}, ClassExploration},
		// R5: any other simple command is the host working locally.
		{"go test", sidecar.ToolCallTrace{Kind: "execute", Title: "go test ./..."}, ClassEngineering},
		{"local git", sidecar.ToolCallTrace{Kind: "execute", Title: "git diff"}, ClassEngineering},
		{"prefix is token-wise", sidecar.ToolCallTrace{Kind: "execute", Title: "ghx search foo"}, ClassEngineering},
		// R6: file-shaped kinds classify by path scope.
		{"read inside", sidecar.ToolCallTrace{Kind: "read", Locations: []string{inside}}, ClassEngineering},
		{"edit new file inside", sidecar.ToolCallTrace{Kind: "edit", Locations: []string{newInside}}, ClassEngineering},
		{"read outside", sidecar.ToolCallTrace{Kind: "read", Locations: []string{outside}}, ClassExploration},
		{"search inside", sidecar.ToolCallTrace{Kind: "search", Locations: []string{root}}, ClassEngineering},
		{"mixed scopes", sidecar.ToolCallTrace{Kind: "edit", Locations: []string{inside, outside}}, ClassUnclassified},
		{"no locations", sidecar.ToolCallTrace{Kind: "edit"}, ClassUnclassified},
		{"unresolvable relative location", sidecar.ToolCallTrace{Kind: "read", Locations: []string{"relative.go"}}, ClassUnclassified},
		// Execute command source: rawInput.command is authoritative — live
		// claude-agent-acp execute calls title "Terminal", not the command
		// (sighted 2026-07-06, ADR-0032.1 S3); the title is fallback only.
		{"adapter-shaped gh call", sidecar.ToolCallTrace{Kind: "execute", Title: "Terminal", RawInput: map[string]any{"command": "gh pr list"}}, ClassExploration},
		{"adapter-shaped local build", sidecar.ToolCallTrace{Kind: "execute", Title: "Terminal", RawInput: map[string]any{"command": "go build ./..."}}, ClassEngineering},
		{"adapter-shaped compound", sidecar.ToolCallTrace{Kind: "execute", Title: "Terminal", RawInput: map[string]any{"command": "cd /tmp && gh api x"}}, ClassUnclassified},
		{"rawInput command beats command-line title", sidecar.ToolCallTrace{Kind: "execute", Title: "go test ./...", RawInput: map[string]any{"command": "gh api repos/x"}}, ClassExploration},
		{"non-map rawInput falls back to title", sidecar.ToolCallTrace{Kind: "execute", Title: "gh api repos/x", RawInput: "opaque"}, ClassExploration},
		{"blank rawInput command falls back to title", sidecar.ToolCallTrace{Kind: "execute", Title: "go vet ./...", RawInput: map[string]any{"command": "  "}}, ClassEngineering},
		{"terminal title with no command is unknowable", sidecar.ToolCallTrace{Kind: "execute", Title: "Terminal"}, ClassUnclassified},
		// R7: planning its own work is engineering.
		{"think", sidecar.ToolCallTrace{Kind: "think", Title: "Update todos"}, ClassEngineering},
		// R8: unknown identities stay visible as unclassified.
		{"other kind", sidecar.ToolCallTrace{Kind: "other", Title: "mystery"}, ClassUnclassified},
		{"empty kind", sidecar.ToolCallTrace{Title: "mystery"}, ClassUnclassified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.classify(tc.trace); got != tc.want {
				t.Fatalf("classify(%+v) = %q, want %q", tc.trace, got, tc.want)
			}
		})
	}
}

func TestAttributeTableTotals(t *testing.T) {
	c, root := testClassifier(t)
	inside := filepath.Join(root, "main.go")

	traces := []sidecar.ToolCallTrace{
		{ID: "t1", Title: "ghx_recon", Kind: "other", OutputSize: 100},                 // exploration
		{ID: "t2", Kind: "execute", Title: "gh api repos/x", OutputSize: 30},           // exploration
		{ID: "t3", Kind: "execute", Title: "go test ./...", OutputSize: 200},           // engineering
		{ID: "t4", Kind: "read", Locations: []string{inside}, OutputSize: 40},          // engineering
		{ID: "t5", Kind: "execute", Title: "gh api x | jq .", OutputSize: 7},           // unclassified
		{ID: "t6", Kind: "edit", OutputSize: 3},                                        // unclassified (no locations)
		{ID: "t7", Kind: "fetch", Title: "Fetch https://spec.example", OutputSize: 20}, // exploration
	}
	table := c.Attribute(traces)

	if len(table.PerToolCall) != len(traces) {
		t.Fatalf("PerToolCall rows = %d, want %d", len(table.PerToolCall), len(traces))
	}
	if got, want := table.ExplorationChars, 100+30+20; got != want {
		t.Fatalf("ExplorationChars = %d, want %d", got, want)
	}
	if got, want := table.EngineeringChars, 200+40; got != want {
		t.Fatalf("EngineeringChars = %d, want %d", got, want)
	}
	// Unclassified is a named bucket: ids listed, chars in neither class.
	if want := []string{"t5", "t6"}; !reflect.DeepEqual(table.Unclassified, want) {
		t.Fatalf("Unclassified = %v, want %v", table.Unclassified, want)
	}
	if got, want := table.TotalChars(), 400; got != want {
		t.Fatalf("TotalChars = %d, want %d", got, want)
	}
	if got, want := table.DeadWeightFraction(), 150.0/400.0; got != want {
		t.Fatalf("DeadWeightFraction = %v, want %v", got, want)
	}
	// Rows carry identity + class + chars so a human can recompute totals.
	if r := table.PerToolCall[0]; r.ID != "t1" || r.Class != ClassExploration || r.Chars != 100 || r.Title != "ghx_recon" || r.Kind != "other" {
		t.Fatalf("row 0 = %+v", r)
	}
}

func TestAttributeEmptyTraces(t *testing.T) {
	c, _ := testClassifier(t)
	table := c.Attribute(nil)
	if len(table.PerToolCall) != 0 || table.ExplorationChars != 0 || table.EngineeringChars != 0 || len(table.Unclassified) != 0 {
		t.Fatalf("empty traces must yield an empty table, got %+v", table)
	}
	if got := table.DeadWeightFraction(); got != 0 {
		t.Fatalf("DeadWeightFraction on empty table = %v, want 0", got)
	}
	if got := table.TotalChars(); got != 0 {
		t.Fatalf("TotalChars on empty table = %v, want 0", got)
	}
}

// TestArgvLocationsCannotChangeExecuteClassification is the ADR-0032.2 D4
// blast-radius proof for host-task attribution. D4 synthesizes Locations for
// execute-kind ghx calls that carried none. The R6 path-scope rule
// (classifyLocations) only runs for read/edit/delete/move/search kinds; an
// execute-kind call always routes through R3–R5 on its command line. So adding
// or removing Locations on an execute-kind trace must leave its class
// unchanged — no host-task grader verdict can move because of D4.
func TestArgvLocationsCannotChangeExecuteClassification(t *testing.T) {
	c, root := testClassifier(t)
	inside := filepath.Join(root, "main.go")
	outside := filepath.Join(t.TempDir(), "elsewhere.go")

	// A ghx recon read over the ACP execute tool: engineering by R5 (simple
	// command that is not an ExplorationCommand). D4 would synthesize a
	// repo-relative path here — set every scope, including one outside the
	// workspace, and confirm the execute class never budges.
	base := sidecar.ToolCallTrace{
		Kind:     "execute",
		Title:    "Terminal",
		RawInput: map[string]any{"command": "ghx read gin-gonic/gin routergroup.go"},
	}
	for _, locs := range [][]string{nil, {"routergroup.go"}, {inside}, {outside}, {inside, outside}} {
		tr := base
		tr.Locations = locs
		if got := c.classify(tr); got != ClassEngineering {
			t.Fatalf("execute ghx call classified as %q with Locations=%v; D4 must not change execute attribution", got, locs)
		}
	}
}
