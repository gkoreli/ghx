package ghx

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/codemode"
)

// TestRegisteredReturnsAdvertiseSnapshot pins ADR-0036 B2's agent-facing half:
// read/explore compute a resolved commit SHA (ghx.Snapshot), so the type stub
// handed to a code-mode agent must advertise it. Otherwise a stub-trusting
// agent never reads r.snapshot.sha even though the runtime JSON already carries
// it — dogfood friction, FRICTION.md 2026-07-07 "Snapshot SHA is computed but
// under-surfaced".
func TestRegisteredReturnsAdvertiseSnapshot(t *testing.T) {
	reg := codemode.NewRegistry()
	RegisterTools(reg)

	byName := map[string]codemode.Tool{}
	for _, tool := range reg.Tools() {
		byName[tool.Name] = tool
	}

	for _, name := range []string{"explore", "read"} {
		tool, ok := byName[name]
		if !ok {
			t.Fatalf("tool %q not registered", name)
		}
		for _, want := range []string{"snapshot", "sha", "owner", "name"} {
			if !strings.Contains(tool.Returns, want) {
				t.Errorf("%s Returns stub omits %q, so an agent cannot discover it: %s", name, want, tool.Returns)
			}
		}
	}
}

// TestSnapshotJSONIsLowercase pins that the snapshot an agent reads from the
// code-mode result serializes with lowercase keys matching the advertised type
// stub (owner/name/sha), not the struct-default PascalCase — otherwise the stub
// lies about the payload (dogfood friction gap 5, same entry).
func TestSnapshotJSONIsLowercase(t *testing.T) {
	snap := Snapshot{Repo: Repo{Owner: "tidwall", Name: "gjson"}, SHA: "deadbeef"}
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"repo"`, `"owner":"tidwall"`, `"name":"gjson"`, `"sha":"deadbeef"`} {
		if !strings.Contains(got, want) {
			t.Errorf("snapshot JSON missing %s: %s", want, got)
		}
	}
	for _, bad := range []string{`"Owner"`, `"Name"`} {
		if strings.Contains(got, bad) {
			t.Errorf("snapshot JSON leaks PascalCase %s (stub advertises lowercase): %s", bad, got)
		}
	}
}
