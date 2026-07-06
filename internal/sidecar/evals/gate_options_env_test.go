package evals

import (
	"reflect"
	"strings"
	"testing"
)

// TestParseGateAtLeast pins the GHX_EVAL_GATE_AT_LEAST spec format
// (ADR-0025.2 residuals: CLI wiring). Absent/blank must yield exactly the
// zero GateOptions — the same value EvaluateGates uses — so an unset env
// var can never change a run's scoring.
func TestParseGateAtLeast(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want GateOptions
	}{
		{"empty", "", GateOptions{}},
		{"blank", "   ", GateOptions{}},
		{"only separators", " , , ", GateOptions{}},
		{"g2 and g4", "G2=4,G4=9", GateOptions{AtLeastN: map[string]AtLeastNSpec{
			"G2": {N: 4}, "G4": {N: 9},
		}}},
		{"g1 paired form", "G1=5", GateOptions{AtLeastN: map[string]AtLeastNSpec{
			"G1": {N: 5},
		}}},
		{"g3 with ceiling", "G3=25@16000", GateOptions{AtLeastN: map[string]AtLeastNSpec{
			"G3": {N: 25, CharCeiling: 16000},
		}}},
		{"all with spaces", " G1=5, G2=25 ,G3=25@16000, G4=8 ", GateOptions{AtLeastN: map[string]AtLeastNSpec{
			"G1": {N: 5}, "G2": {N: 25}, "G3": {N: 25, CharCeiling: 16000}, "G4": {N: 8},
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseGateAtLeast(tc.raw)
			if err != nil {
				t.Fatalf("ParseGateAtLeast(%q): %v", tc.raw, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ParseGateAtLeast(%q) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}

// TestParseGateAtLeastRejectsInvalidSpecs pins the fail-loud contract: an
// invalid or incomplete spec is an error, never a silent fallback to
// default scoring (visibility tenet — a caller must not believe an override
// took effect when it did not).
func TestParseGateAtLeastRejectsInvalidSpecs(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		errPart string
	}{
		{"no equals", "G2", "is not GATE=N"},
		{"empty gate id", "=4", "is not GATE=N"},
		{"non-numeric n", "G2=four", "positive integer N"},
		{"zero n", "G2=0", "positive integer N"},
		{"negative n", "G2=-1", "positive integer N"},
		{"duplicate gate", "G2=4,G2=5", "duplicate entry for G2"},
		{"g3 without ceiling", "G3=4", "G3=N@CEILING"},
		{"g3 bad ceiling", "G3=4@zero", "positive integer char ceiling"},
		{"g3 zero ceiling", "G3=4@0", "positive integer char ceiling"},
		{"ceiling on g2", "G2=4@16000", "only G3 takes one"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGateAtLeast(tc.raw)
			if err == nil {
				t.Fatalf("ParseGateAtLeast(%q) must fail", tc.raw)
			}
			if !strings.Contains(err.Error(), tc.errPart) {
				t.Fatalf("ParseGateAtLeast(%q) error %q must mention %q", tc.raw, err, tc.errPart)
			}
		})
	}
}

// TestGateOptionsFromEnvRoundTrip pins the env-var entry point end to end:
// the parsed options drive EvaluateGatesWithOptions to the at_least(n)
// reducer, and clearing the variable restores the exact zero-options
// default.
func TestGateOptionsFromEnvRoundTrip(t *testing.T) {
	t.Setenv(GateAtLeastEnv, "G1=6,G2=30,G3=30@16000,G4=10")
	opts, err := GateOptionsFromEnv()
	if err != nil {
		t.Fatalf("GateOptionsFromEnv: %v", err)
	}
	v := EvaluateGatesWithOptions(gateRunEpisodes(), opts)
	for i, id := range []string{"G1", "G2", "G3", "G4"} {
		if v.Gates[i].Reducer != ReducerAtLeastN {
			t.Errorf("%s: reducer = %q, want %q under %s", id, v.Gates[i].Reducer, ReducerAtLeastN, GateAtLeastEnv)
		}
	}
	if v.Gates[4].Reducer != ReducerMean {
		t.Errorf("G5 must stay ReducerMean, got %q", v.Gates[4].Reducer)
	}

	t.Setenv(GateAtLeastEnv, "")
	opts, err = GateOptionsFromEnv()
	if err != nil {
		t.Fatalf("GateOptionsFromEnv (unset): %v", err)
	}
	if !reflect.DeepEqual(opts, GateOptions{}) {
		t.Fatalf("unset %s must yield GateOptions{}, got %+v", GateAtLeastEnv, opts)
	}
	def := EvaluateGatesWithOptions(gateRunEpisodes(), opts)
	if !reflect.DeepEqual(def, EvaluateGates(gateRunEpisodes())) {
		t.Fatal("blank env options must reproduce EvaluateGates exactly")
	}
}
