package evals

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TestGhxArgvLocations exercises the ADR-0032.2 D4 argv parser against the ghx
// reconnaissance subcommand shapes, including the compound/piped/redirected
// command lines the sidecar actually emits.
func TestGhxArgvLocations(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want []string
	}{
		// read: positional file paths after owner/repo; flags and their values
		// are skipped, never emitted as paths.
		{"read single path", "ghx read gin-gonic/gin routergroup.go --map", []string{"routergroup.go"}},
		{"read multiple paths", "ghx read gin-gonic/gin routergroup.go tree.go --map", []string{"routergroup.go", "tree.go"}},
		{"read with --lines value not a path", "ghx read gin-gonic/gin gin.go --lines 364-420", []string{"gin.go"}},
		{"read with --grep pattern not a path", `ghx read gkoreli/ghx internal/cli/ghx.go --grep "RunE|Use:"`, []string{"internal/cli/ghx.go"}},
		{"read glob positional", `ghx read gkoreli/ghx "internal/**/*.go" --map`, []string{"internal/**/*.go"}},

		// explore / tree: optional path positional after owner/repo.
		{"explore repo only", "ghx explore gin-gonic/gin", nil},
		{"tree with subtree", "ghx tree gin-gonic/gin internal", []string{"internal"}},

		// inspect / grep / search: only --path / --glob name a real subtree; the
		// concern/regex/query positional is NOT a file path and must not appear.
		{"inspect query is not a path", `ghx inspect gin-gonic/gin "routing middleware"`, nil},
		{"inspect --path scope", `ghx inspect gkoreli/ghx "flag errors" --path internal/cli`, []string{"internal/cli"}},
		{"inspect --glob scope", `ghx inspect openai/openai-node "streaming" --glob "src/**/*.ts"`, []string{"src/**/*.ts"}},
		{"grep pattern is not a path", `ghx grep gkoreli/ghx "func main"`, nil},
		{"grep --path scope", `ghx grep gkoreli/ghx "cobra.Command" --path internal/cli`, []string{"internal/cli"}},
		{"grep --path with pipe in quoted pattern", `ghx grep gin-gonic/gin "handle|Handle" --path internal`, []string{"internal"}},
		{"grep --glob= form", `ghx grep gkoreli/ghx "RunE" --glob=internal/**/*.go`, []string{"internal/**/*.go"}},
		{"search --glob scope", `ghx search gkoreli/ghx "SetFlagErrorFunc" --glob "internal/**/*.go"`, []string{"internal/**/*.go"}},

		// tier2 astgrep: recurse into the tier-2 subcommand, positional paths.
		{"tier2 astgrep positional", "ghx tier2 astgrep gkoreli/ghx --pattern 'errors.Is($E,$T)' --lang go internal", []string{"internal"}},

		// compound / piped / redirected — the real sidecar shapes.
		{"compound && two reads same file deduped",
			`ghx read gin-gonic/gin gin.go --lines 364-420 && echo "---SERVE---" && ghx read gin-gonic/gin gin.go --lines 662-760`,
			[]string{"gin.go"}},
		{"pipe into sed keeps only ghx segment",
			`ghx read gin-gonic/gin gin.go | sed -n '360,420p;655,762p'`,
			[]string{"gin.go"}},
		{"redirect 2>/dev/null is not a path",
			`ghx read gin-gonic/gin routergroup.go --grep "x" 2>/dev/null`,
			[]string{"routergroup.go"}},
		{"newline-separated with echo and second ghx",
			"ghx read gin-gonic/gin context.go --grep \"Next\"\necho \"---\"\nghx read gin-gonic/gin context.go | sed -n '/Next/,/^}/p'",
			[]string{"context.go"}},

		// npx invocation form.
		{"npx ghx read", "npx @gkoreli/ghx read gin-gonic/gin gin.go", []string{"gin.go"}},

		// non-recon / non-ghx / no path — honest empty.
		{"ghx --version has no path", "ghx --version", nil},
		{"non-ghx command", "Read /dev/null", nil},
		{"gh api is not ghx", "gh api repos/gin-gonic/gin", nil},
		{"empty", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ghxArgvLocations(tc.cmd)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ghxArgvLocations(%q) = %#v, want %#v", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestDeriveArgvLocationsIsAdditive proves the ADR-0032.2 D4 additive invariant:
// argv derivation fires only when ACP reported no locations, and never
// overrides locations the adapter already provided.
func TestDeriveArgvLocationsIsAdditive(t *testing.T) {
	t.Run("fills when empty", func(t *testing.T) {
		tr := sidecar.ToolCallTrace{
			Kind:     "execute",
			Title:    "ghx read gin-gonic/gin gin.go --map",
			RawInput: map[string]any{"command": "ghx read gin-gonic/gin gin.go --map"},
		}
		deriveArgvLocations(&tr)
		if !reflect.DeepEqual(tr.Locations, []string{"gin.go"}) {
			t.Fatalf("Locations = %#v, want [gin.go]", tr.Locations)
		}
	})
	t.Run("never overrides real ACP locations", func(t *testing.T) {
		tr := sidecar.ToolCallTrace{
			Kind:      "execute",
			Title:     "ghx read gin-gonic/gin gin.go --map",
			RawInput:  map[string]any{"command": "ghx read gin-gonic/gin gin.go --map"},
			Locations: []string{"real/acp/path.go"},
		}
		deriveArgvLocations(&tr)
		if !reflect.DeepEqual(tr.Locations, []string{"real/acp/path.go"}) {
			t.Fatalf("Locations = %#v, want the untouched ACP value", tr.Locations)
		}
	})
	t.Run("non-ghx execute stays empty", func(t *testing.T) {
		tr := sidecar.ToolCallTrace{
			Kind:     "execute",
			Title:    "Terminal",
			RawInput: map[string]any{"command": "gh api repos/gin-gonic/gin"},
		}
		deriveArgvLocations(&tr)
		if tr.Locations != nil {
			t.Fatalf("Locations = %#v, want nil (no ghx recon → no synthesized path)", tr.Locations)
		}
	})
}

// TestFillArgvLocationsOnCommittedSidecarFixture is the D4 delta measurement in
// test form: it loads the committed ghx-sidecar episode fixture (7 execute ghx
// calls, all with empty locations) and confirms fillArgvLocations populates the
// path scope the recon actually touched — the empty→populated delta ADR-0032.2
// D4 exists to produce.
func TestFillArgvLocationsOnCommittedSidecarFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/judge/episode_sidecar.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var ep Episode
	if err := json.Unmarshal(data, &ep); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	// Precondition: the committed artifact carries no locations (the H9 bug).
	for _, turn := range ep.Turns {
		for _, tr := range turn.ToolTraces {
			if len(tr.Locations) > 0 {
				t.Fatalf("fixture precondition violated: trace %s already has locations %v", tr.ID, tr.Locations)
			}
		}
	}
	// Expected derived path scope, by command line (all kind=execute ghx calls).
	want := map[string][]string{
		"ghx --version":                               nil,
		"ghx explore gin-gonic/gin":                   nil,
		"ghx read gin-gonic/gin routergroup.go --map": {"routergroup.go"},
		"ghx read gin-gonic/gin gin.go --map":         {"gin.go"},
		"ghx read gin-gonic/gin gin.go --lines 364-420 && echo \"---SERVE---\" && ghx read gin-gonic/gin gin.go --lines 662-760": {"gin.go"},
		"ghx read gin-gonic/gin tree.go --map":           {"tree.go"},
		"ghx read gin-gonic/gin tree.go --lines 418-670": {"tree.go"},
	}
	for i := range ep.Turns {
		fillArgvLocations(&ep.Turns[i])
	}
	populated := 0
	for _, turn := range ep.Turns {
		for _, tr := range turn.ToolTraces {
			cmd, _ := tr.RawInput.(map[string]any)
			key, _ := cmd["command"].(string)
			exp, known := want[key]
			if !known {
				t.Fatalf("unexpected command in fixture: %q", key)
			}
			if !reflect.DeepEqual(tr.Locations, exp) {
				t.Fatalf("command %q: Locations = %#v, want %#v", key, tr.Locations, exp)
			}
			if len(tr.Locations) > 0 {
				populated++
			}
		}
	}
	if populated == 0 {
		t.Fatal("expected some execute ghx calls to gain derived locations, got none")
	}
	t.Logf("D4 delta: %d/%d execute ghx traces gained argv-derived Locations", populated, countTraces(ep))
}

func countTraces(ep Episode) int {
	n := 0
	for _, turn := range ep.Turns {
		n += len(turn.ToolTraces)
	}
	return n
}

// TestToolCallTraceSerializationShape freezes the on-disk JSON key shape of the
// unified sidecar.ToolCallTrace so a future field/tag change that would break
// artifact compatibility fails loudly (ADR-0032.2 D1: the dedup is only safe
// because the tags are and stay byte-identical to the deleted evals copy).
func TestToolCallTraceSerializationShape(t *testing.T) {
	tr := sidecar.ToolCallTrace{
		ID:                "toolu_1",
		Kind:              "execute",
		Title:             "ghx read a/b c.go",
		RawInput:          map[string]any{"command": "ghx read a/b c.go"},
		Locations:         []string{"c.go"},
		StatusTransitions: []sidecar.ToolStatusTransition{{Status: "completed"}},
		OutputSize:        42,
		OutputExcerpt:     "hello",
	}
	b, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	wantKeys := []string{"id", "kind", "title", "rawInput", "locations", "statusTransitions", "outputSize", "outputExcerpt"}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("serialized trace missing key %q; got keys %v", k, keysOf(got))
		}
	}
	if len(got) != len(wantKeys) {
		t.Errorf("serialized trace key count = %d, want %d; got keys %v", len(got), len(wantKeys), keysOf(got))
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
