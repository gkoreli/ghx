package evals

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TestToolTraceSerializationByteIdentical is the ADR-0032.2 D1 proof. D1
// deleted the byte-identical evals copy of ToolCallTrace/ToolStatusTransition
// and pointed TurnRecord.ToolTraces/ReplayedToolTraces at the runtime's
// canonical sidecar types. This test proves that swap serializes committed
// artifacts identically: for every committed episode fixture, each turn's
// `toolTraces` and `replayedToolTraces` array, taken verbatim from the on-disk
// fixture, re-serializes byte-for-byte (canonically) after decoding through the
// unified []sidecar.ToolCallTrace.
//
// The comparison is deliberately scoped to the trace arrays — the only thing D1
// touched. A whole-Episode round-trip is NOT a clean D1 proof because it also
// surfaces pre-existing, unrelated schema gaps between older/hand-authored
// fixtures and the current Episode struct (e.g. Go's struct `omitempty` no-op
// always emitting `"checks":{}`, and non-omitempty zero fields like
// `context.sidecarInternalChars` and `rewards.memory`). Those predate this ADR
// and D1 does not change them.
func TestToolTraceSerializationByteIdentical(t *testing.T) {
	fixtures := traceFixtures(t)
	if len(fixtures) == 0 {
		t.Fatal("no committed episode fixtures with toolTraces found")
	}
	checkedArrays := 0
	for _, f := range fixtures {
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			// Pull the fixture apart just enough to reach each turn's raw trace
			// arrays, without imposing the full Episode schema.
			var doc struct {
				Turns []map[string]json.RawMessage `json:"turns"`
			}
			if err := json.Unmarshal(raw, &doc); err != nil {
				t.Fatalf("unmarshal turns: %v", err)
			}
			for ti, turn := range doc.Turns {
				for _, field := range []string{"toolTraces", "replayedToolTraces"} {
					original, ok := turn[field]
					if !ok {
						continue
					}
					var traces []sidecar.ToolCallTrace
					if err := json.Unmarshal(original, &traces); err != nil {
						t.Fatalf("turn %d %s: decode through unified type: %v", ti, field, err)
					}
					reser, err := json.Marshal(traces)
					if err != nil {
						t.Fatalf("turn %d %s: re-marshal: %v", ti, field, err)
					}
					if want, got := canonicalJSON(t, original), canonicalJSON(t, reser); !bytes.Equal(want, got) {
						t.Fatalf("turn %d %s: serialization drift after unification\nwant %s\ngot  %s", ti, field, want, got)
					}
					checkedArrays++
				}
			}
		})
	}
	t.Logf("D1 byte-identical: %d trace arrays across %d fixtures re-serialized with zero drift", checkedArrays, len(fixtures))
}

// traceFixtures returns committed episode JSON fixtures that carry toolTraces.
func traceFixtures(t *testing.T) []string {
	t.Helper()
	roots := []string{"testdata/judge", "testdata/export/run"}
	var out []string
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
				continue
			}
			p := filepath.Join(root, e.Name())
			b, err := os.ReadFile(p)
			if err == nil && bytes.Contains(b, []byte(`"toolTraces"`)) {
				out = append(out, p)
			}
		}
	}
	return out
}

// canonicalJSON re-encodes arbitrary JSON with sorted keys and no incidental
// whitespace so two documents with the same content compare equal regardless of
// formatting.
func canonicalJSON(t *testing.T, raw []byte) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("canonicalize marshal: %v", err)
	}
	return out
}
