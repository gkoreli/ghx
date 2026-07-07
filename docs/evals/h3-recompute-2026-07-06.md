# TRUST H3 recompute — 2026-07-06 (workstream C7)

Re-verification of the TRUST.md H3 row ("Trace-capture completeness") from
committed artifacts alone. This note lets any reader reproduce the ledger's
"zero gaps at n=5" reading with the **frozen** comparator, not the run's own
self-report.

## What is checked

The H3 hole is: `toolTraces` are captured from ACP `session/update`
notifications, so a silent drop would under-count trajectory metrics. The
ADR-0016.10 comparator answers it by diffing the ACP-captured `toolTraces`
against a **second, independent** channel — the raw SDK message stream the
adapter forwards as the `_claude/sdkMessage` extension notification, parsed
into `rawSDK.toolUses` / `rawSDK.toolResults` by
`internal/sidecar/rawsdk.go`. Because these are two different wire paths from
the same adapter, `len(toolUses) == len(toolTraces)` is a genuine
completeness cross-check.

`CompareTraceCapture` (`internal/sidecar/evals/trace_capture.go`) does more
than count: it matches by exact tool_use ID (the adapter reuses the SDK
tool_use id verbatim as the ACP toolCallId), then checks input-digest
equality, terminal status, and output presence — the four pre-registered gap
kinds. `DetectAnomalies` wraps it into the per-turn `trace_capture_gap` soft
anomaly. Both are the frozen scoring functions; nothing in this recompute
reimplements scoring logic.

## Committed inputs (5 episodes / 6 turns)

```
docs/evals/spot-instrumented-2026-07-06/episodes/gin-routing_ghx-sidecar_1783355255114.json   (2 turns)
docs/evals/spot-instrumented-2026-07-06/episodes/flask-routing_ghx-sidecar_1783355329077.json (1 turn)
docs/evals/spot-instrumented-2026-07-06/discovery/openai-compatible-ai-gateways_plain_1783355441972.json      (1 turn)
docs/evals/spot-instrumented-2026-07-06/discovery/openai-compatible-ai-gateways_ghx_1783355532458.json        (1 turn)
docs/evals/spot-instrumented-2026-07-06/discovery/openai-compatible-ai-gateways_ghx-sidecar_1783355623811.json (1 turn)
```

No committed test drives these JSON files: `grep -rn 1783355255114 internal/`
returns only comment references (`discovery_e2e_test.go`, `telemetry/trace_test.go`),
not a recompute. So the recompute is a **throwaway** `_test.go` placed in
`internal/sidecar/evals/`, run, then deleted (same disclosure model the spot
run itself used for its discovery driver). Its only job is orchestration: it
calls `LoadEpisode`, then the exported frozen `DetectAnomalies` and
`CompareTraceCapture`, and prints tallies. Full source below.

## Throwaway recompute driver

Save as `internal/sidecar/evals/ztrust_h3_recompute_test.go`, run, delete:

```go
package evals

import (
	"fmt"
	"path/filepath"
	"sort"
	"testing"
)

func TestZTrustH3Recompute(t *testing.T) {
	root := "../../../docs/evals/spot-instrumented-2026-07-06"
	paths := []string{
		"episodes/gin-routing_ghx-sidecar_1783355255114.json",
		"episodes/flask-routing_ghx-sidecar_1783355329077.json",
		"discovery/openai-compatible-ai-gateways_plain_1783355441972.json",
		"discovery/openai-compatible-ai-gateways_ghx_1783355532458.json",
		"discovery/openai-compatible-ai-gateways_ghx-sidecar_1783355623811.json",
	}
	sort.Strings(paths)

	totalRaw, totalCap, totalTurns, armedTurns := 0, 0, 0, 0
	totalGaps, traceGapAnoms := 0, 0
	msgMin, msgMax := 1<<30, 0
	usageResultTurns, usageCostTurns := 0, 0

	for _, rel := range paths {
		ep, err := LoadEpisode(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("load %s: %v", rel, err)
		}
		for _, a := range DetectAnomalies(ep) {
			if a.Kind == AnomalyTraceCaptureGap {
				traceGapAnoms++
			}
		}
		for _, turn := range ep.Turns {
			totalTurns++
			if turn.RawSDK == nil {
				t.Errorf("%s turn %d: no RawSDK audit", rel, turn.Turn)
				continue
			}
			if turn.RawSDK.Messages == 0 {
				t.Errorf("%s turn %d: RawSDK.Messages==0 (channel never opened)", rel, turn.Turn)
			} else {
				armedTurns++
			}
			if turn.RawSDK.Messages < msgMin {
				msgMin = turn.RawSDK.Messages
			}
			if turn.RawSDK.Messages > msgMax {
				msgMax = turn.RawSDK.Messages
			}
			totalRaw += len(turn.RawSDK.ToolUses)
			totalCap += len(turn.ToolTraces)
			gaps := CompareTraceCapture(turn.RawSDK, turn.ToolTraces)
			totalGaps += len(gaps)
			if len(gaps) > 0 {
				t.Errorf("%s turn %d: %d gaps: %+v", rel, turn.Turn, len(gaps), gaps)
			}
			if u := turn.RawSDK.Usage; u != nil {
				if u.Source == "result" {
					usageResultTurns++
				}
				if u.CostUSD > 0 {
					usageCostTurns++
				}
			}
		}
	}

	fmt.Printf("H3-RECOMPUTE episodes=%d turns=%d armed(msgs>0)=%d\n", len(paths), totalTurns, armedTurns)
	fmt.Printf("H3-RECOMPUTE rawToolUses=%d capturedToolTraces=%d equal=%v\n", totalRaw, totalCap, totalRaw == totalCap)
	fmt.Printf("H3-RECOMPUTE frozen CompareTraceCapture gaps=%d ; DetectAnomalies trace_capture_gap anomalies=%d\n", totalGaps, traceGapAnoms)
	fmt.Printf("H3-RECOMPUTE messagesPerTurn min=%d max=%d\n", msgMin, msgMax)
	fmt.Printf("H5-HOOK usage.source==result turns=%d ; costUsd>0 turns=%d (of %d)\n", usageResultTurns, usageCostTurns, totalTurns)
}
```

Run:

```
go test ./internal/sidecar/evals/ -run TestZTrustH3Recompute -v
```

## Captured output (2026-07-06, tree mainline)

```
H3-RECOMPUTE episodes=5 turns=6 armed(msgs>0)=6
H3-RECOMPUTE rawToolUses=53 capturedToolTraces=53 equal=true
H3-RECOMPUTE frozen CompareTraceCapture gaps=0 ; DetectAnomalies trace_capture_gap anomalies=0
H3-RECOMPUTE messagesPerTurn min=117 max=303
H5-HOOK usage.source==result turns=6 ; costUsd>0 turns=6 (of 6)
--- PASS: TestZTrustH3Recompute (0.00s)
ok  	github.com/gkoreli/ghx/v2/internal/sidecar/evals
```

Every number in the H3 row reproduces exactly from the committed JSONs with
the frozen scorer.

## What this does NOT close

H3 stays **open**. This is an n=5 / 6-turn random spot check, not a
full-size run. The reading is a clean zero, but "zero gaps" at this scale
cannot rule out drops under load, parallelism, or long multi-turn sessions.
The row closes only when a full-size instrumented run reads the same zero
through the same frozen comparator. Nothing here rescopes that.

Verifier: workstream C7 (this ledger's recompute). Evidence: the five files
above + this note. Recompute: the recipe above.
