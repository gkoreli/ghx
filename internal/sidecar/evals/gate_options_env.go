package evals

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GateAtLeastEnv names the environment variable that opts specific gates of
// a live gate run into the ReducerAtLeastN form (ADR-0025.2 residuals: CLI
// wiring). Unset or empty reproduces today's default — every gate scores
// under ReducerMean — so no run changes behavior without an explicit,
// pre-registered opt-in, following the same knob pattern as
// GHX_EVAL_PARALLEL and GHX_EVAL_BASELINE_REUSE_RUN_DIR.
//
// Format: comma-separated GATE=N entries. G3 additionally requires its
// pre-registered per-episode main-agent chars ceiling as GATE=N@CEILING:
//
//	GHX_EVAL_GATE_AT_LEAST="G1=5,G2=25,G3=25@16000,G4=8"
//
// Every entry must parse; an invalid or incomplete spec is an error, never a
// silent fallback — a caller must not believe an override took effect when
// it did not (visibility tenet). A parseable entry naming a gate the reducer
// does not support (G5) is passed through and surfaces as a GATE CONFIG
// verdict note, matching EvaluateGatesWithOptions' programmatic behavior.
const GateAtLeastEnv = "GHX_EVAL_GATE_AT_LEAST"

// GateOptionsFromEnv resolves the GateOptions for a live gate run from
// GHX_EVAL_GATE_AT_LEAST. Unset or blank returns GateOptions{} — exactly
// today's EvaluateGates behavior on every gate.
func GateOptionsFromEnv() (GateOptions, error) {
	return ParseGateAtLeast(os.Getenv(GateAtLeastEnv))
}

// ParseGateAtLeast parses a GHX_EVAL_GATE_AT_LEAST value (see GateAtLeastEnv
// for the format) into GateOptions. It is exported separately from
// GateOptionsFromEnv so tests and future run-config surfaces can parse the
// same spec without touching the process environment.
func ParseGateAtLeast(raw string) (GateOptions, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return GateOptions{}, nil
	}
	specs := map[string]AtLeastNSpec{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		id, value, ok := strings.Cut(entry, "=")
		id = strings.TrimSpace(id)
		if !ok || id == "" {
			return GateOptions{}, fmt.Errorf("%s: entry %q is not GATE=N", GateAtLeastEnv, entry)
		}
		if _, dup := specs[id]; dup {
			return GateOptions{}, fmt.Errorf("%s: duplicate entry for %s", GateAtLeastEnv, id)
		}
		nStr, ceilStr, hasCeiling := strings.Cut(strings.TrimSpace(value), "@")
		n, err := strconv.Atoi(strings.TrimSpace(nStr))
		if err != nil || n < 1 {
			return GateOptions{}, fmt.Errorf("%s: entry %q needs a positive integer N", GateAtLeastEnv, entry)
		}
		spec := AtLeastNSpec{N: n}
		if hasCeiling {
			if id != "G3" {
				return GateOptions{}, fmt.Errorf(
					"%s: entry %q sets a char ceiling but only G3 takes one (ADR-0025.2 residuals)", GateAtLeastEnv, entry)
			}
			c, err := strconv.Atoi(strings.TrimSpace(ceilStr))
			if err != nil || c < 1 {
				return GateOptions{}, fmt.Errorf(
					"%s: entry %q needs a positive integer char ceiling after '@'", GateAtLeastEnv, entry)
			}
			spec.CharCeiling = c
		}
		if id == "G3" && !hasCeiling {
			return GateOptions{}, fmt.Errorf(
				"%s: G3 requires its pre-registered per-episode char ceiling — use G3=N@CEILING (ADR-0025.2 residuals)", GateAtLeastEnv)
		}
		specs[id] = spec
	}
	if len(specs) == 0 {
		return GateOptions{}, nil
	}
	return GateOptions{AtLeastN: specs}, nil
}
