package cli

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// directServeTools is the exact tool set `ghx serve --direct` must expose
// (ADR-0019.3 D1): the seven direct ghx exploration tools plus the code
// meta-tool — the pre-0019.3 default surface.
var directServeTools = map[string]bool{
	"explore":      true,
	"repos":        true,
	"search":       true,
	"read":         true,
	"tree":         true,
	"search_tools": true,
	"code":         true,
}

// serverToolNames returns the sorted tool names registered on an MCP server.
func serverToolNames(s *server.MCPServer) []string {
	tools := s.ListTools()
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestServeDefaultRegistersExactlyRecon pins ADR-0019.3 D1: bare `ghx serve`
// serves exactly one tool named "recon" — the flagship surface is the default,
// not the opt-in.
func TestServeDefaultRegistersExactlyRecon(t *testing.T) {
	s := newServeServer(false)
	got := serverToolNames(s)
	if len(got) != 1 {
		t.Fatalf("default serve must register exactly one tool, got %d: %v", len(got), got)
	}
	if got[0] != sidecar.ReconToolName {
		t.Fatalf("default serve tool = %q, want %q", got[0], sidecar.ReconToolName)
	}
}

// TestServeDirectRegistersDirectTools pins the `--direct` escape hatch
// (ADR-0019.3 D1): it registers exactly the seven direct tools plus code.
func TestServeDirectRegistersDirectTools(t *testing.T) {
	s := newServeServer(true)
	names := serverToolNames(s)
	if len(names) != len(directServeTools) {
		t.Fatalf("--direct must register %d tools, got %d: %v", len(directServeTools), len(names), names)
	}
	for _, n := range names {
		if !directServeTools[n] {
			t.Fatalf("unexpected --direct tool %q; registered: %v", n, names)
		}
		delete(directServeTools, n)
	}
	if len(directServeTools) != 0 {
		t.Fatalf("--direct is missing tools: %v", directServeTools)
	}
}

// TestServeReconFlagRemainsAcceptedSynonym pins that --recon still works as a
// deprecated no-op synonym of the recon-first default (ADR-0019.3 D1): same
// single recon tool, so existing configs do not break silently.
func TestServeReconFlagRemainsAcceptedSynonym(t *testing.T) {
	s := newServeServer(false)
	reconDefault := serverToolNames(s)

	cmd := serveCmd
	if err := cmd.Flags().Set("recon", "true"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Flags().Set("recon", "false") })

	direct, _ := cmd.Flags().GetBool("direct")
	recon, _ := cmd.Flags().GetBool("recon")
	if recon != true || direct != false {
		t.Fatalf("--recon flag parsing changed: recon=%v direct=%v", recon, direct)
	}

	// The synonym resolves to the same registration path as the default:
	// !direct ⇒ the single recon tool.
	synonym := newServeServer(direct)
	got := serverToolNames(synonym)
	want := append([]string{}, reconDefault...)
	if len(got) != len(want) {
		t.Fatalf("--recon synonym registers %v, want the default's %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("--recon synonym tool[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestHandleReconResultIsPureJSON is the D2 contract pin (ADR-0019.3): on
// every result shape the recon tool's text is exactly one machine-parseable
// JSON object — {"report":…,"route":…,"artifacts":…} — with no prose outside
// the JSON. Route provenance and the artifacts pointer live inside the
// payload; humans keep rich text on `ghx sidecar ask`.
func TestHandleReconResultIsPureJSON(t *testing.T) {
	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"

	cases := []struct {
		name   string
		report *sidecar.Report
		turn   *sidecar.TurnResult
		check  func(t *testing.T, res sidecar.ReconResult)
	}{
		{
			name:   "success shape",
			report: &sidecar.Report{Answer: "middleware chains through wrap()"},
			turn: &sidecar.TurnResult{
				Route: &sidecar.RouteDecision{Session: "hono-hono", Source: sidecar.RouteSourceRepo},
				Artifacts: sidecar.ArtifactsRef{
					SessionDir: "/home/u/.ghx/sessions/hono-hono",
					TraceID:    traceID,
				},
			},
			check: func(t *testing.T, res sidecar.ReconResult) {
				if res.Report == nil || res.Report.Answer != "middleware chains through wrap()" {
					t.Fatalf("report = %+v", res.Report)
				}
				if res.Route == nil || res.Route.Session != "hono-hono" {
					t.Fatalf("route = %+v", res.Route)
				}
				if res.Artifacts == nil || res.Artifacts.SessionDir != "/home/u/.ghx/sessions/hono-hono" ||
					res.Artifacts.TraceID != traceID {
					t.Fatalf("artifacts = %+v", res.Artifacts)
				}
			},
		},
		{
			name:   "BLOCKED-report shape",
			report: &sidecar.Report{Answer: "BLOCKED: GitHub auth missing; run ghx sidecar doctor"},
			turn: &sidecar.TurnResult{
				Artifacts: sidecar.ArtifactsRef{SessionDir: "/home/u/.ghx/sessions/discovery-x"},
			},
			check: func(t *testing.T, res sidecar.ReconResult) {
				if res.Report == nil || !strings.HasPrefix(res.Report.Answer, "BLOCKED") {
					t.Fatalf("report = %+v", res.Report)
				}
				if res.Route != nil {
					t.Fatalf("route = %+v, want null", res.Route)
				}
				if res.Artifacts == nil || res.Artifacts.TraceID != "" {
					t.Fatalf("artifacts = %+v", res.Artifacts)
				}
			},
		},
		{
			name:   "error-result shape (nil turn)",
			report: &sidecar.Report{Answer: "ok"},
			turn:   nil,
			check: func(t *testing.T, res sidecar.ReconResult) {
				if res.Report == nil {
					t.Fatal("report must never be null in the envelope")
				}
				if res.Route != nil || res.Artifacts != nil {
					t.Fatalf("route/artifacts = %+v/%+v, want both null", res.Route, res.Artifacts)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := askSidecar
			defer func() { askSidecar = orig }()
			var gotReq sidecar.AskRequest
			askSidecar = func(_ context.Context, _ sidecar.Config, req sidecar.AskRequest) (*sidecar.Report, *sidecar.TurnResult, error) {
				gotReq = req
				return tc.report, tc.turn, nil
			}

			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"question": "how is X wired?", "repo": "OwnerX/RepoY"}
			res, err := handleRecon(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				t.Fatalf("recon returned a tool error: %+v", res)
			}
			text, ok := res.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatalf("content[0] is %T, want TextContent", res.Content[0])
			}

			// THE contract: the whole text unmarshals with encoding/json. Any
			// prose outside the JSON object breaks this — today or ever after.
			var envelope sidecar.ReconResult
			if err := json.Unmarshal([]byte(text.Text), &envelope); err != nil {
				t.Fatalf("recon result text is not pure JSON (ADR-0019.3 D2):\n%q\nerror: %v", text.Text, err)
			}
			tc.check(t, envelope)

			// And nothing but the JSON object: no trailing newline+prose.
			if strings.ContainsAny(text.Text[jsonEnd(text.Text):], "{}[]\"") ||
				strings.TrimSpace(text.Text) != text.Text {
				t.Fatalf("recon text has bytes outside the JSON payload:\n%q", text.Text)
			}
			if gotReq.Question != "how is X wired?" || gotReq.Repo != "OwnerX/RepoY" {
				t.Fatalf("params not forwarded: %+v", gotReq)
			}
		})
	}
}

// jsonEnd returns the index just past the closing brace of a top-level JSON
// object rendered by encoding/json (which never emits trailing whitespace).
func jsonEnd(text string) int {
	depth, inStr, esc := 0, false, false
	for i, r := range text {
		switch {
		case esc:
			esc = false
		case r == '\\' && inStr:
			esc = true
		case r == '"':
			inStr = !inStr
		case inStr:
		case r == '{':
			depth++
		case r == '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(text)
}
