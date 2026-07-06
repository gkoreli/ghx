package tier2

import (
	"strings"
	"testing"
)

// TestFindProvenanceRoundTrip pins ADR-0024.2 D5: FindProvenance is the exact
// inverse of Provenance.Lines, including sparse paths and the HEAD rendering
// of an empty ref.
func TestFindProvenanceRoundTrip(t *testing.T) {
	cases := []Provenance{
		{
			Repo: "honojs/hono", Ref: "", SHA: "626b185d0e80fa9787b6e2f25b6a5e97cbaad0de",
			Strategy: "shallow-blobless", CacheHit: true,
			SnapshotPath: "/home/u/.ghx/cache/tier2/repos/github.com/honojs/hono/snapshots/626b185",
		},
		{
			Repo: "gkoreli/ghx", Ref: "next", SHA: "abc123",
			Strategy: "shallow-blobless-sparse", CacheHit: false,
			SnapshotPath: "/tmp/path with spaces/snap",
			SparsePaths:  []string{"internal/sidecar", "docs/adr"},
		},
	}
	for _, want := range cases {
		text := "some tool banner\n" + strings.Join(want.Lines(), "\n") + "\n{\"json\": \"output\"}\n"
		got, ok := FindProvenance(text)
		if !ok {
			t.Fatalf("FindProvenance found nothing in:\n%s", text)
		}
		if got.Repo != want.Repo || got.Ref != want.Ref || got.SHA != want.SHA ||
			got.Strategy != want.Strategy || got.CacheHit != want.CacheHit ||
			got.SnapshotPath != want.SnapshotPath ||
			strings.Join(got.SparsePaths, "|") != strings.Join(want.SparsePaths, "|") {
			t.Fatalf("round-trip mismatch:\n got  %+v\n want %+v", got, want)
		}
	}
}

// TestFindProvenanceAbsent pins the honest-gap contract: unrelated text
// yields ok=false, never a guessed provenance.
func TestFindProvenanceAbsent(t *testing.T) {
	if _, ok := FindProvenance("ghx search results\nno tier2 here\n"); ok {
		t.Fatal("FindProvenance invented a provenance")
	}
}

// TestFindProvenanceStopsAtBlockEnd pins that a later unrelated block cannot
// bleed into the first parsed provenance.
func TestFindProvenanceStopsAtBlockEnd(t *testing.T) {
	first := Provenance{Repo: "a/b", SHA: "111", Strategy: "shallow-blobless", SnapshotPath: "/p1"}
	second := Provenance{Repo: "c/d", SHA: "222", Strategy: "shallow-blobless", SnapshotPath: "/p2",
		SparsePaths: []string{"x"}}
	text := strings.Join(first.Lines(), "\n") + "\ncodemap output line\n" + strings.Join(second.Lines(), "\n")
	got, ok := FindProvenance(text)
	if !ok || got.Repo != "a/b" || got.SHA != "111" || len(got.SparsePaths) != 0 {
		t.Fatalf("first block not isolated: %+v ok=%v", got, ok)
	}
}
