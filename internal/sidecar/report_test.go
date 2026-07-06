package sidecar

import (
	"errors"
	"strings"
	"testing"
)

// TestExtractReportErrDiagnostics pins ADR-0021 D3: the error-returning variant
// reports WHY extraction failed, so the corrective retry can name the defect.
func TestExtractReportErrDiagnostics(t *testing.T) {
	t.Run("no block", func(t *testing.T) {
		_, coerced, err := ExtractReportErr("plain prose, no block")
		if !errors.Is(err, ErrNoReportBlock) || coerced {
			t.Fatalf("err=%v coerced=%v, want ErrNoReportBlock", err, coerced)
		}
	})
	t.Run("invalid json", func(t *testing.T) {
		_, _, err := ExtractReportErr("<ghx-report>{not json at all}</ghx-report>")
		if err == nil || !strings.Contains(err.Error(), "invalid JSON") {
			t.Fatalf("err=%v, want invalid JSON diagnostic", err)
		}
	})
	t.Run("empty answer", func(t *testing.T) {
		_, _, err := ExtractReportErr(`<ghx-report>{"answer":""}</ghx-report>`)
		if !errors.Is(err, ErrEmptyAnswer) {
			t.Fatalf("err=%v, want ErrEmptyAnswer", err)
		}
	})
	t.Run("valid strict", func(t *testing.T) {
		r, coerced, err := ExtractReportErr(`<ghx-report>{"answer":"ok"}</ghx-report>`)
		if err != nil || r == nil || coerced {
			t.Fatalf("r=%v coerced=%v err=%v, want clean strict parse", r, coerced, err)
		}
	})
	t.Run("coerced flag set", func(t *testing.T) {
		r, coerced, err := ExtractReportErr(`<ghx-report>{"answer":"ok","verified":"one claim"}</ghx-report>`)
		if err != nil || r == nil || !coerced {
			t.Fatalf("r=%v coerced=%v err=%v, want coerced parse", r, coerced, err)
		}
		if len(r.Verified) != 1 {
			t.Fatalf("verified = %+v, want coerced single claim", r.Verified)
		}
	})
}

func TestExtractReportValid(t *testing.T) {
	text := `Some exploration narration.

<ghx-report>
{
  "answer": "Middleware composition lives in src/compose.ts.",
  "verified": [{"summary": "compose() dispatches handlers", "evidence": "src/compose.ts defines compose()"}],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [{"path": "src/compose.ts", "reason": "defines compose()"}],
  "evidence": [{"source": "ghx read honojs/hono src/compose.ts --map", "summary": "compose signature"}],
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx tree honojs/hono src --depth 2"],
  "uncertainty": ["did not inspect tests"],
  "nextReads": ["src/hono-base.ts"]
}
</ghx-report>`

	r := ExtractReport(text)
	if r == nil {
		t.Fatal("expected report, got nil")
	}
	if !strings.Contains(r.Answer, "src/compose.ts") {
		t.Errorf("answer = %q, want mention of src/compose.ts", r.Answer)
	}
	if len(r.Verified) != 1 || r.Verified[0].Evidence == "" {
		t.Errorf("verified = %+v, want 1 claim with evidence", r.Verified)
	}
	if len(r.RelevantFiles) != 1 || r.RelevantFiles[0].Path != "src/compose.ts" {
		t.Errorf("relevantFiles = %+v", r.RelevantFiles)
	}
	if len(r.CommandsRun) != 1 {
		t.Errorf("commandsRun = %+v, want 1 command", r.CommandsRun)
	}
}

func TestExtractReportMissingBlock(t *testing.T) {
	if r := ExtractReport("plain prose with no report block"); r != nil {
		t.Errorf("expected nil, got %+v", r)
	}
}

func TestExtractReportMalformedJSON(t *testing.T) {
	if r := ExtractReport("<ghx-report>{not json}</ghx-report>"); r != nil {
		t.Errorf("expected nil for malformed JSON, got %+v", r)
	}
}

func TestExtractReportEmptyAnswer(t *testing.T) {
	if r := ExtractReport(`<ghx-report>{"answer": ""}</ghx-report>`); r != nil {
		t.Errorf("expected nil for empty answer, got %+v", r)
	}
}

func TestExtractReportTakesFirstBlock(t *testing.T) {
	text := `<ghx-report>{"answer": "first"}</ghx-report>
<ghx-report>{"answer": "second"}</ghx-report>`
	r := ExtractReport(text)
	if r == nil || r.Answer != "first" {
		t.Errorf("got %+v, want answer %q", r, "first")
	}
}

func TestExtractReportMultilineContent(t *testing.T) {
	// (?s) flag must let the block span newlines and inner braces.
	text := "prefix\n<ghx-report>\n{\"answer\": \"a\", \"verified\": [{\"summary\": \"s\"}]}\n</ghx-report>\nsuffix"
	r := ExtractReport(text)
	if r == nil || r.Answer != "a" {
		t.Fatalf("got %+v", r)
	}
}

// TestExtractReportCoercesNearConformantShapes is the ADR-0016.7 RC2
// regression: the exact shape drift observed in gate-run episode
// hono-middleware_ghx-sidecar_1783285987325 turn 1 — bare strings where
// the schema wants arrays ([]Claim, []string) and single objects where it
// wants object arrays. Strict parsing discarded that entire report and
// scored completed work as zero evidence.
func TestExtractReportCoercesNearConformantShapes(t *testing.T) {
	text := `<ghx-report>
{
  "answer": "Errors propagate on two layers: compose.ts dispatch and hono-base onError.",
  "verified": {"summary": "compose() dispatch try/catches handler calls", "evidence": "src/compose.ts"},
  "inferred": [],
  "unverified": "Whether HTTPException is the sole implementer of getResponse()",
  "relevantFiles": {"path": "src/compose.ts", "reason": "inner try/catch"},
  "evidence": {"source": "ghx read honojs/hono src/hono-base.ts --grep onError", "summary": "error handler wiring"},
  "backendsUsed": "remote",
  "commandsRun": "ghx read honojs/hono src/hono-base.ts --grep onError",
  "uncertainty": "Did not re-confirm HTTPException.getResponse() this turn",
  "nextReads": "src/http-exception.ts"
}
</ghx-report>`

	r := ExtractReport(text)
	if r == nil {
		t.Fatal("near-conformant report was rejected, want coerced parse")
	}
	if len(r.Verified) != 1 || r.Verified[0].Evidence != "src/compose.ts" {
		t.Errorf("verified = %+v, want single-object coerced to 1 claim", r.Verified)
	}
	if len(r.Unverified) != 1 || !strings.Contains(r.Unverified[0].Summary, "HTTPException") {
		t.Errorf("unverified = %+v, want bare string lifted to claim", r.Unverified)
	}
	if len(r.RelevantFiles) != 1 || r.RelevantFiles[0].Path != "src/compose.ts" {
		t.Errorf("relevantFiles = %+v, want single object wrapped", r.RelevantFiles)
	}
	if len(r.Evidence) != 1 || r.Evidence[0].Source == "" {
		t.Errorf("evidence = %+v, want single object wrapped", r.Evidence)
	}
	if len(r.CommandsRun) != 1 || len(r.BackendsUsed) != 1 || len(r.Uncertainty) != 1 || len(r.NextReads) != 1 {
		t.Errorf("string lists not wrapped: commands=%v backends=%v uncertainty=%v nextReads=%v",
			r.CommandsRun, r.BackendsUsed, r.Uncertainty, r.NextReads)
	}
}

func TestExtractReportCoercesStringObjectListItems(t *testing.T) {
	text := `<ghx-report>
{
  "answer": "Middleware composition lives in src/compose.ts.",
  "verified": [],
  "inferred": [],
  "unverified": [],
  "relevantFiles": ["src/compose.ts"],
  "evidence": ["compose() dispatches middleware"],
  "backendsUsed": ["remote"],
  "commandsRun": ["ghx read honojs/hono src/compose.ts"],
  "uncertainty": [],
  "nextReads": [42, true]
}
</ghx-report>`

	r := ExtractReport(text)
	if r == nil {
		t.Fatal("string/object drift report was rejected, want coerced parse")
	}
	if len(r.RelevantFiles) != 1 || r.RelevantFiles[0].Path != "src/compose.ts" {
		t.Errorf("relevantFiles = %+v, want bare string lifted to path object", r.RelevantFiles)
	}
	if len(r.Evidence) != 1 || r.Evidence[0].Summary != "compose() dispatches middleware" {
		t.Errorf("evidence = %+v, want bare string lifted to summary object", r.Evidence)
	}
	if len(r.NextReads) != 2 || r.NextReads[0] != "42" || r.NextReads[1] != "true" {
		t.Errorf("nextReads = %+v, want scalar items serialized as strings", r.NextReads)
	}
}

func TestExtractReportCoercesConfirmatoryGateObjectNextReads(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "gin-routing_ghx-sidecar_1783294237204 turn 0 retry report",
			text: `<ghx-report>
{
  "answer": "Routes are registered via RouterGroup methods (GET/POST/PUT/etc., routergroup.go:111-152) which call group.handle() (routergroup.go:86) → engine.addRoute() (gin.go:364), inserting handlers into a per-HTTP-method radix (compressed trie) tree (tree.go: node.addRoute at 135, node.insertChild at 288), stored in engine.trees (methodTrees). Incoming requests are matched in Engine.ServeHTTP (gin.go:662) → handleHTTPRequest (gin.go:690), which selects the tree for the request's HTTP method and calls root.getValue(path, ...) (tree.go:418) to walk the radix tree and resolve path params/wildcards, then executes the matched HandlersChain via c.Next(); unmatched paths fall through to trailing-slash/fixed-path redirects, 405 (method-not-allowed check across other trees), or 404 (allNoRoute).",
  "verified": [
    {"summary": "HTTP verb methods (GET, POST, etc.) delegate to group.handle, which builds absolute path + combined handlers and calls engine.addRoute", "evidence": "routergroup.go:86-152"},
    {"summary": "addRoute finds/creates the per-method radix tree root and inserts the route", "evidence": "gin.go:364-385"},
    {"summary": "ServeHTTP resets a pooled Context and calls handleHTTPRequest for each request", "evidence": "gin.go:662-674"},
    {"summary": "handleHTTPRequest iterates engine.trees for the matching method, calls root.getValue to match the path and extract params, then runs the handler chain; otherwise redirect/405/404 logic runs", "evidence": "gin.go:690-761"},
    {"summary": "Radix tree implementation: addRoute/insertChild build the trie; getValue performs longest-prefix matching including param(:x) and wildcard(*x) nodes", "evidence": "tree.go:135, tree.go:288, tree.go:418"}
  ],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [
    {"path": "routergroup.go", "reason": "Public route-registration API (GET/POST/Handle/Group) and handle()"},
    {"path": "gin.go", "reason": "Engine.addRoute, ServeHTTP, handleHTTPRequest — core dispatch loop"},
    {"path": "tree.go", "reason": "Radix tree node.addRoute/insertChild (registration) and node.getValue (matching)"}
  ],
  "evidence": [
    {"source": "ghx explore gin-gonic/gin", "summary": "Top-level file listing; identified routergroup.go, gin.go, tree.go as routing core"},
    {"source": "ghx read gin-gonic/gin routergroup.go --map", "summary": "Method map showing GET/POST/.../handle/Group definitions"},
    {"source": "ghx read gin-gonic/gin gin.go --map", "summary": "Method map showing addRoute, ServeHTTP, handleHTTPRequest, Routes/iterate"},
    {"source": "ghx read gin-gonic/gin gin.go --lines 364-420", "summary": "Full source of addRoute and Routes/iterate"},
    {"source": "ghx read gin-gonic/gin gin.go --lines 662-760", "summary": "Full source of ServeHTTP and handleHTTPRequest"},
    {"source": "ghx read gin-gonic/gin tree.go --map", "summary": "Method map showing node.addRoute, insertChild, getValue"}
  ],
  "backendsUsed": ["remote"],
  "commandsRun": [
    "ghx --version",
    "ghx explore gin-gonic/gin",
    "ghx read gin-gonic/gin routergroup.go --map",
    "ghx read gin-gonic/gin gin.go --map",
    "ghx read gin-gonic/gin gin.go --lines 364-420",
    "ghx read gin-gonic/gin gin.go --lines 662-760",
    "ghx read gin-gonic/gin tree.go --map"
  ],
  "uncertainty": [],
  "nextReads": [{"path": "tree.go:418-670", "reason": "Full getValue body for exact wildcard/param precedence rules, if deeper detail is needed"}]
}
</ghx-report>`,
			want: "tree.go:418-670 — Full getValue body for exact wildcard/param precedence rules, if deeper detail is needed",
		},
		{
			name: "hono-middleware_ghx-sidecar_1783296851961 turn 0 retry report",
			text: `<ghx-report>
{
  "answer": "Middleware composition is implemented in src/compose.ts, which exports compose() — a koa-compose-style function that turns an array of [handler, params] middleware tuples into a single async dispatcher using an internal recursive dispatch(i) that calls handler(context, () => dispatch(i+1)), handling next(), errors (onError), and not-found (onNotFound). It is consumed by src/hono-base.ts, where Hono's private #dispatch method (~line 407-455) matches routes via the router, then calls compose(matchResult[0], this.errorHandler, this.#notFoundHandler) to build the composed handler chain executed per-request; compose is also invoked at line 226 for app.use()-based mounting.",
  "verified": [
    {"summary": "compose() builds a recursive dispatch(i) closure chaining middleware via next()", "evidence": "src/compose.ts lines 15-71, function dispatch(i) calls handler(context, () => dispatch(i+1))"},
    {"summary": "compose() is imported and used by Hono's core request dispatch", "evidence": "src/hono-base.ts line 8 ` + "`" + `import { compose } from './compose'` + "`" + `; line 451 ` + "`" + `const composed = compose(matchResult[0], this.errorHandler, this.#notFoundHandler)` + "`" + `"},
    {"summary": "compose is also used for app.use()-style handler mounting", "evidence": "src/hono-base.ts line 226 ` + "`" + `compose([], app.errorHandler)(c, () => r.handler(c, next))` + "`" + `"}
  ],
  "inferred": [],
  "unverified": [],
  "relevantFiles": [
    {"path": "src/compose.ts", "reason": "Core middleware composition function (compose/dispatch chain)"},
    {"path": "src/hono-base.ts", "reason": "Consumes compose() in #dispatch (~L407-455) to run matched middleware/handlers per request, and at L226 for use() mounting"}
  ],
  "evidence": [
    {"source": "ghx read honojs/hono src/compose.ts", "summary": "Full source of compose(), showing dispatch/next/error/notFound logic"},
    {"source": "ghx read honojs/hono src/hono-base.ts --map", "summary": "Confirms import of compose at line 8"},
    {"source": "ghx read honojs/hono src/hono-base.ts | grep compose", "summary": "Located call sites at lines 226 and 451"}
  ],
  "backendsUsed": ["remote"],
  "commandsRun": [
    "ghx explore honojs/hono",
    "ghx read honojs/hono src/compose.ts --map",
    "ghx read honojs/hono src/compose.ts",
    "ghx read honojs/hono src/hono-base.ts --map",
    "ghx read honojs/hono src/hono-base.ts | grep -n compose"
  ],
  "uncertainty": [],
  "nextReads": [{"path": "src/hono-base.ts lines 400-460", "reason": "See full #dispatch method context around compose() call for route matching details"}]
}
</ghx-report>`,
			want: "src/hono-base.ts lines 400-460 — See full #dispatch method context around compose() call for route matching details",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := ExtractReport(tt.text)
			if r == nil {
				t.Fatal("confirmatory gate report was rejected, want coerced parse")
			}
			if r.Answer == "" {
				t.Fatal("answer is empty")
			}
			if len(r.NextReads) == 0 {
				t.Fatal("nextReads is empty")
			}
			if r.NextReads[0] != tt.want {
				t.Errorf("nextReads[0] = %q, want %q", r.NextReads[0], tt.want)
			}
		})
	}
}

// TestExtractReportCoercionRejectsGarbage: coercion must not resurrect
// reports that are wrong beyond shape drift.
func TestExtractReportCoercionRejectsGarbage(t *testing.T) {
	if r := ExtractReport(`<ghx-report>{"answer": "x", "verified": 42}</ghx-report>`); r != nil {
		t.Errorf("numeric claim list should stay rejected, got %+v", r)
	}
}
