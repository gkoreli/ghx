package sidecar

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNewReconResultShape pins the ADR-0019.3 D2 result-contract mapping: the
// report unchanged under "report", the turn's routing decision under "route",
// and the artifacts pointer under "artifacts". A nil turn yields null route
// and artifacts; an empty ArtifactsRef (ask failed before a session dir
// existed) maps to null artifacts, not an empty object.
func TestNewReconResultShape(t *testing.T) {
	report := &Report{Answer: "middleware chains through wrap()"}
	turn := &TurnResult{
		Route: &RouteDecision{
			Session: "hono-hono",
			Source:  RouteSourceRepo,
		},
		Artifacts: ArtifactsRef{
			SessionDir: "/home/u/.ghx/sessions/hono-hono",
			TraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
		},
	}

	res := NewReconResult(report, turn)
	if res.Report != report {
		t.Fatal("ReconResult.Report must carry the report unchanged")
	}
	if res.Route == nil || res.Route.Session != "hono-hono" || res.Route.Source != RouteSourceRepo {
		t.Fatalf("ReconResult.Route = %+v", res.Route)
	}
	if res.Artifacts == nil || res.Artifacts.SessionDir != "/home/u/.ghx/sessions/hono-hono" ||
		res.Artifacts.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("ReconResult.Artifacts = %+v", res.Artifacts)
	}

	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(data) {
		t.Fatal("marshaled ReconResult is not valid JSON")
	}
	var decoded struct {
		Report struct {
			Answer string `json:"answer"`
		} `json:"report"`
		Route *struct {
			Session string `json:"session"`
			Source  string `json:"source"`
		} `json:"route"`
		Artifacts *struct {
			SessionDir string `json:"sessionDir"`
			TraceID    string `json:"traceId"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("recon result must unmarshal with encoding/json: %v", err)
	}
	if decoded.Report.Answer != "middleware chains through wrap()" {
		t.Fatalf("decoded report.answer = %q", decoded.Report.Answer)
	}
	if decoded.Route == nil || decoded.Route.Session != "hono-hono" {
		t.Fatalf("decoded route = %+v", decoded.Route)
	}
	if decoded.Artifacts == nil || decoded.Artifacts.SessionDir != "/home/u/.ghx/sessions/hono-hono" {
		t.Fatalf("decoded artifacts = %+v", decoded.Artifacts)
	}
}

func TestNewReconResultNilTurnAndEmptyArtifacts(t *testing.T) {
	// Nil turn (failure before any turn ran): route and artifacts are nil.
	nilTurn := NewReconResult(&Report{Answer: "BLOCKED: upstream unavailable"}, nil)
	if nilTurn.Route != nil || nilTurn.Artifacts != nil {
		t.Fatalf("nil turn must give nil route/artifacts, got %+v", nilTurn)
	}

	// Empty ArtifactsRef (no session dir): artifacts stays null instead of an
	// empty object.
	empty := NewReconResult(&Report{Answer: "ok"}, &TurnResult{})
	if empty.Route != nil {
		t.Fatalf("nil turn.Route must stay nil, got %+v", empty.Route)
	}
	if empty.Artifacts != nil {
		t.Fatalf("empty ArtifactsRef must map to nil, got %+v", empty.Artifacts)
	}

	for name, res := range map[string]ReconResult{"nil-turn": nilTurn, "empty-ref": empty} {
		data, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"report", "route", "artifacts"} {
			if _, ok := m[key]; !ok {
				t.Fatalf("%s: recon result JSON missing %q key: %s", name, key, data)
			}
		}
		if strings.Contains(string(data), "sessionDir\":{}") {
			t.Fatalf("%s: artifacts must be null, not an empty object: %s", name, data)
		}
	}
}

// TestReconResultKeysAlwaysPresent pins that the marshaled envelope carries
// all three keys on every shape — a consumer keys into "report" without
// probing for its presence first.
func TestReconResultKeysAlwaysPresent(t *testing.T) {
	full := NewReconResult(
		&Report{Answer: "ok"},
		&TurnResult{
			Route:     &RouteDecision{Session: "s", Source: RouteSourceExplicit},
			Artifacts: ArtifactsRef{SessionDir: "/tmp/s"},
		},
	)
	minimal := NewReconResult(&Report{Answer: "BLOCKED: invalid repo"}, nil)
	for name, res := range map[string]ReconResult{"full": full, "minimal": minimal} {
		data, err := json.Marshal(res)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"report", "route", "artifacts"} {
			raw, ok := m[key]
			if !ok {
				t.Fatalf("%s: missing %q key in %s", name, key, data)
			}
			if string(raw) == "null" && key == "report" {
				t.Fatalf("%s: report must never serialize as null: %s", name, data)
			}
		}
	}
}
