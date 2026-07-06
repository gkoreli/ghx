package sidecar

import (
	"math"
	"strings"
	"testing"
	"time"
)

// This file is the pinned specification of the ADR-0030.1 routing cascade
// (D7): table-driven fixtures of question x on-disk session states →
// expected (session, source). Changing a routing default means changing a
// visible test row here — deliberately.

// routeNow is the fixed decision clock for the table.
var routeNow = time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)

func testRouteConfig() RouteConfig {
	rc := RouteConfigFor(Config{})
	rc.Now = func() time.Time { return routeNow }
	return rc
}

// routeSession is one on-disk session fixture.
type routeSession struct {
	name    string
	repo    string
	scope   string
	namedBy string
	age     time.Duration // UpdatedAt = routeNow - age
	ledger  *Ledger
}

func writeRouteSessions(t *testing.T, dir string, sessions []routeSession) {
	t.Helper()
	for _, s := range sessions {
		if err := InitSession(dir, s.name, s.repo, s.scope, s.namedBy); err != nil {
			t.Fatal(err)
		}
		meta, err := ReadMeta(dir, s.name)
		if err != nil {
			t.Fatal(err)
		}
		meta.TurnCount = 1
		meta.UpdatedAt = routeNow.Add(-s.age).Format(time.RFC3339)
		if err := SaveMeta(dir, *meta); err != nil {
			t.Fatal(err)
		}
		ledger := s.ledger
		if ledger == nil {
			ledger = &Ledger{}
		}
		if err := SaveLedger(dir, s.name, ledger); err != nil {
			t.Fatal(err)
		}
	}
}

// concurrencySession is the running example from the ADR: a discovery
// session whose OpenQuestions predicted the follow-up.
func concurrencySession(age time.Duration) routeSession {
	return routeSession{
		name:    "which-go-libraries-do-structured-conc-4be29e5c",
		namedBy: SessionNamedQuestion,
		scope:   "which Go libraries do structured concurrency",
		age:     age,
		ledger: &Ledger{
			OpenQuestions: []LedgerEntry{
				{Value: "how mature is sourcegraph/conc compared to errgroup", Turn: 1},
				{Value: "inspect conc waitgroup panics handling", Turn: 1},
			},
			RelevantFiles: []RelevantFileEntry{{Path: "pool/pool.go", Turn: 1}},
			GrepPatterns:  []LedgerEntry{{Value: "WaitGroup", Turn: 1}},
		},
	}
}

func TestRouteQuestionDecisionTable(t *testing.T) {
	// Sessions reused across rows; each row builds its own dir.
	ginRepoSession := routeSession{
		name: "gin-gonic-gin", repo: "gin-gonic/gin", namedBy: SessionNamedRepo,
		scope: "routing internals", age: 5 * time.Minute,
		ledger: &Ledger{
			Repo:          "gin-gonic/gin",
			RelevantFiles: []RelevantFileEntry{{Path: "routergroup.go", Turn: 1}},
		},
	}
	namedWarm := routeSession{
		name: "auth-thread", namedBy: SessionNamedExplicit,
		scope: "auth middleware deep dive", age: 2 * time.Minute,
		ledger: &Ledger{
			OpenQuestions: []LedgerEntry{{Value: "auth middleware ordering", Turn: 1}},
		},
	}

	cases := []struct {
		row         string
		sessions    []routeSession
		req         AskRequest
		wantSession string
		wantSource  RouteSource
	}{
		{
			row:         "explicit wins over everything (R1)",
			sessions:    []routeSession{concurrencySession(time.Minute), ginRepoSession},
			req:         AskRequest{Session: "my-thread", Repo: "gin-gonic/gin", Question: "how mature is sourcegraph/conc"},
			wantSession: "my-thread",
			wantSource:  RouteSourceExplicit,
		},
		{
			row:         "repo param keeps the exact repo slug (R2, ADR-0019.1 pin)",
			sessions:    []routeSession{concurrencySession(time.Minute)},
			req:         AskRequest{Repo: "OwnerX/RepoY", Question: "q"},
			wantSession: "ownerx-repoy",
			wantSource:  RouteSourceRepo,
		},
		{
			row:         "owner/repo token in question text routes to the repo slug (R2)",
			req:         AskRequest{Question: "how does gin-gonic/gin group routes?"},
			wantSession: "gin-gonic-gin",
			wantSource:  RouteSourceRepo,
		},
		{
			row:         "source-tree path lookalike is not a repo token (denylist)",
			req:         AskRequest{Question: "explain internal/sidecar please"},
			wantSession: QuestionSlug("explain internal/sidecar please"),
			wantSource:  RouteSourceNew,
		},
		{
			row:         "continuation marker with exactly one warm session (R3 hit)",
			sessions:    []routeSession{concurrencySession(3 * time.Minute)},
			req:         AskRequest{Question: "how mature is the second one?"},
			wantSession: "which-go-libraries-do-structured-conc-4be29e5c",
			wantSource:  RouteSourceContinuation,
		},
		{
			row: "continuation marker with two warm no-overlap sessions declines to R5",
			sessions: []routeSession{
				{name: "flask-routing-review-11223344", namedBy: SessionNamedQuestion, age: 4 * time.Minute, ledger: &Ledger{}},
				{name: "hono-middleware-scan-55667788", namedBy: SessionNamedQuestion, age: 6 * time.Minute, ledger: &Ledger{}},
			},
			req:         AskRequest{Question: "what about the second one?"},
			wantSession: QuestionSlug("what about the second one?"),
			wantSource:  RouteSourceNew,
		},
		{
			row: "continuation marker with two warm sessions falls through to R4 when one predicted the question",
			sessions: []routeSession{
				concurrencySession(3 * time.Minute),
				{name: "flask-routing-review-11223344", namedBy: SessionNamedQuestion, age: 4 * time.Minute, ledger: &Ledger{}},
			},
			req:         AskRequest{Question: "how mature is the second one?"},
			wantSession: "which-go-libraries-do-structured-conc-4be29e5c",
			wantSource:  RouteSourceOverlap,
		},
		{
			row:         "continuation marker with zero warm sessions declines (R3 -> R5)",
			sessions:    []routeSession{concurrencySession(2 * time.Hour)},
			req:         AskRequest{Question: "and what about its tests?"},
			wantSession: QuestionSlug("and what about its tests?"),
			wantSource:  RouteSourceNew,
		},
		{
			row:         "overlap above threshold with clear margin (R4 hit)",
			sessions:    []routeSession{concurrencySession(time.Hour), ginRepoSession},
			req:         AskRequest{Question: "does conc handle waitgroup panics and errgroup interop?"},
			wantSession: "which-go-libraries-do-structured-conc-4be29e5c",
			wantSource:  RouteSourceOverlap,
		},
		{
			row: "overlap above threshold with thin margin declines (R4 -> R5)",
			sessions: []routeSession{
				concurrencySession(time.Hour),
				func() routeSession {
					s := concurrencySession(time.Hour)
					s.name = "twin-session-aabbccdd"
					return s
				}(),
			},
			req:         AskRequest{Question: "does conc handle waitgroup panics and errgroup interop?"},
			wantSession: QuestionSlug("does conc handle waitgroup panics and errgroup interop?"),
			wantSource:  RouteSourceNew,
		},
		{
			row: "OpenQuestions hit outweighs path-segment noise (R4 weighting)",
			sessions: []routeSession{
				concurrencySession(time.Hour),
				{
					name: "pathnoise-session-00112233", namedBy: SessionNamedQuestion, age: time.Hour,
					ledger: &Ledger{InspectedPaths: []LedgerEntry{
						{Value: "conc/waitgroup.go", Turn: 1},
						{Value: "conc/panics/panics.go", Turn: 1},
					}},
				},
			},
			req:         AskRequest{Question: "does conc handle waitgroup panics and errgroup interop?"},
			wantSession: "which-go-libraries-do-structured-conc-4be29e5c",
			wantSource:  RouteSourceOverlap,
		},
		{
			row: "short anaphoric question with no overlap is never an R4 false positive",
			sessions: []routeSession{
				func() routeSession {
					s := ginRepoSession
					s.age = 2 * time.Hour // cold: R3 has no warm candidate
					return s
				}(),
			},
			req:         AskRequest{Question: "what about the second one?"},
			wantSession: QuestionSlug("what about the second one?"),
			wantSource:  RouteSourceNew,
		},
		{
			row:         "punctuation-only question falls to a new discovery session",
			sessions:    []routeSession{concurrencySession(time.Minute)},
			req:         AskRequest{Question: "???"},
			wantSession: QuestionSlug("???"),
			wantSource:  RouteSourceNew,
		},
		{
			row:         "window-expired sessions are excluded even with perfect overlap",
			sessions:    []routeSession{concurrencySession(8 * 24 * time.Hour)},
			req:         AskRequest{Question: "does conc handle waitgroup panics and errgroup interop?"},
			wantSession: QuestionSlug("does conc handle waitgroup panics and errgroup interop?"),
			wantSource:  RouteSourceNew,
		},
		{
			row:         "explicitly-named sessions are reachable only via R1 (no auto-join)",
			sessions:    []routeSession{namedWarm},
			req:         AskRequest{Question: "and the auth middleware ordering?"},
			wantSession: QuestionSlug("and the auth middleware ordering?"),
			wantSource:  RouteSourceNew,
		},
	}

	for _, tc := range cases {
		t.Run(tc.row, func(t *testing.T) {
			dir := t.TempDir()
			writeRouteSessions(t, dir, tc.sessions)
			got := RouteQuestion(dir, tc.req, testRouteConfig())
			if got.Session != tc.wantSession || got.Source != tc.wantSource {
				t.Fatalf("route = (%q, %s), want (%q, %s)\ndecision: %+v",
					got.Session, got.Source, tc.wantSession, tc.wantSource, got)
			}
		})
	}
}

// TestRoutePreservesResolveSessionNamePrecedence pins that R1/R2 reproduce
// the pre-routing ResolveSessionName behavior byte-for-byte (ADR-0019.1 D2 /
// ADR-0030 D5): explicit session verbatim, repo slug via Slug, question slug
// via QuestionSlug.
func TestRoutePreservesResolveSessionNamePrecedence(t *testing.T) {
	dir := t.TempDir() // empty registry: R5 is the only repo-less outcome
	rc := testRouteConfig()

	if got := RouteQuestion(dir, AskRequest{Session: "Named-Thread", Repo: "o/r", Question: "q"}, rc); got.Session != "Named-Thread" {
		t.Fatalf("explicit session mutated: %q", got.Session)
	}
	if got := RouteQuestion(dir, AskRequest{Repo: "Honojs/Hono", Question: "q"}, rc); got.Session != Slug("Honojs/Hono", "repo") {
		t.Fatalf("repo slug = %q, want %q", got.Session, Slug("Honojs/Hono", "repo"))
	}
	q := "which repos implement ACP agents in the wild today by any chance" // no repo token, >6 content tokens? keep repo-less
	got := RouteQuestion(dir, AskRequest{Question: q}, rc)
	if got.Session != QuestionSlug(q) || got.Source != RouteSourceNew {
		t.Fatalf("repo-less empty-registry route = (%q, %s), want question slug (R5)", got.Session, got.Source)
	}
}

// TestOverlapScorePinned pins the D3 scoring arithmetic on a hand-checkable
// example: the route record must be recomputable by a human.
func TestOverlapScorePinned(t *testing.T) {
	s := concurrencySession(0)
	meta := &SessionMeta{Name: s.name, Scope: s.scope}
	vocab := BuildSessionVocabulary(meta, s.ledger)

	// Question tokens: conc, handle, waitgroup, panics, errgroup, interop
	// (6 unique; "does"/"and" are stopwords).
	tokens := TokenizeQuestion("does conc handle waitgroup panics and errgroup interop?")
	if len(tokens) != 6 {
		t.Fatalf("tokens = %v, want 6 content tokens", tokens)
	}
	// Vocabulary hits: conc x3, waitgroup x3 (OpenQuestions; the x2 grep hit
	// never stacks), panics x3, errgroup x3; handle/interop miss (handling
	// != handle — no stemming, deliberately).
	// Score = (3+3+3+3+0+0)/6 = 2.0.
	score := OverlapScore(tokens, vocab)
	if math.Abs(score-2.0) > 1e-9 {
		t.Fatalf("score = %v, want 2.0", score)
	}

	// Recency decay: fresh = 1.0, half-window = 0.75, edge and beyond = 0.5.
	if d := recencyDecay(0, defaultRoutingWindow); d != 1.0 {
		t.Fatalf("decay(0) = %v", d)
	}
	if d := recencyDecay(defaultRoutingWindow/2, defaultRoutingWindow); math.Abs(d-0.75) > 1e-9 {
		t.Fatalf("decay(half) = %v, want 0.75", d)
	}
	if d := recencyDecay(defaultRoutingWindow, defaultRoutingWindow); d != 0.5 {
		t.Fatalf("decay(edge) = %v, want 0.5", d)
	}
}

// TestRouteDecisionRecordFields pins the D7 auditability payload on an R4
// decision: score, margin, and top-3 candidates present and consistent.
func TestRouteDecisionRecordFields(t *testing.T) {
	dir := t.TempDir()
	writeRouteSessions(t, dir, []routeSession{
		concurrencySession(time.Hour),
		{
			name: "pathnoise-session-00112233", namedBy: SessionNamedQuestion, age: time.Hour,
			ledger: &Ledger{InspectedPaths: []LedgerEntry{{Value: "conc/waitgroup.go", Turn: 1}}},
		},
	})
	got := RouteQuestion(dir, AskRequest{Question: "does conc handle waitgroup panics and errgroup interop?"}, testRouteConfig())
	if got.Source != RouteSourceOverlap {
		t.Fatalf("source = %s, want overlap; decision %+v", got.Source, got)
	}
	if got.Score <= 0 || got.Margin <= 0 || len(got.Candidates) != 2 {
		t.Fatalf("decision missing audit fields: %+v", got)
	}
	if got.Candidates[0].Session != got.Session || got.Candidates[0].Score != got.Score {
		t.Fatalf("best candidate inconsistent with decision: %+v", got)
	}
	if math.Abs((got.Candidates[0].Score-got.Candidates[1].Score)-got.Margin) > 1e-9 {
		t.Fatalf("margin != best - runner-up: %+v", got)
	}
	if !strings.Contains(got.Line(), "routed: overlap") {
		t.Fatalf("route line = %q", got.Line())
	}
}

// TestContinuationMarkerTable pins the D2 marker semantics (version 1).
func TestContinuationMarkerTable(t *testing.T) {
	cases := []struct {
		q    string
		want bool
	}{
		{"how mature is the second one?", true},
		{"what about its tests", true},
		{"and error handling?", true},
		{"is it thread safe", true},
		{"now check the scheduler", true},
		{"how does gin handle route grouping internals", false},         // no marker
		{"what about gin-gonic/gin middleware?", false},                 // repo token blocks
		{"and how do the seventeen internal packages interact with the scheduler runtime layers", false}, // too many content tokens
		{"???", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := hasContinuationMarker(tc.q); got != tc.want {
			t.Errorf("hasContinuationMarker(%q) = %v, want %v", tc.q, got, tc.want)
		}
	}
}

// TestDetectRepoToken pins the R2 token detector including the path denylist.
func TestDetectRepoToken(t *testing.T) {
	cases := []struct {
		q    string
		want string
	}{
		{"how does gin-gonic/gin group routes?", "gin-gonic/gin"},
		{"compare (vercel/next.js) routing", "vercel/next.js"},
		{"explain internal/sidecar please", ""},
		{"read src/main.go", ""},
		{"see https://github.com/gin-gonic/gin for details", ""}, // URLs are not bare tokens (v1)
		{"no repo here", ""},
		{"a/b", "a/b"},
	}
	for _, tc := range cases {
		if got := DetectRepoToken(tc.q); got != tc.want {
			t.Errorf("DetectRepoToken(%q) = %q, want %q", tc.q, got, tc.want)
		}
	}
}
