package cli

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/mark3labs/mcp-go/mcp"
)

// questionSession derivation (ADR-0019.1 D2): kebab-cased leading words,
// ~40-char truncation, short content hash for uniqueness, stable across calls.

func TestQuestionSessionStable(t *testing.T) {
	q := "Which Go libraries provide OpenTelemetry OTLP file exporters?"
	if a, b := questionSession(q), questionSession(q); a != b {
		t.Fatalf("questionSession is not stable: %q vs %q", a, b)
	}
}

func TestQuestionSessionShape(t *testing.T) {
	s := questionSession("Which Go libraries provide OpenTelemetry OTLP file exporters?")
	re := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*-[0-9a-f]{8}$`)
	if !re.MatchString(s) {
		t.Fatalf("questionSession shape = %q, want kebab slug + 8-hex hash", s)
	}
	if !strings.HasPrefix(s, "which-go-libraries-provide") {
		t.Fatalf("questionSession = %q, want leading words of the question", s)
	}
}

func TestQuestionSessionTruncatesLongQuestions(t *testing.T) {
	q := "How does the middleware chaining implementation compose handlers across nested routers and groups?"
	s := questionSession(q)
	// Strip the "-<8 hex>" suffix; the slug part must respect the ~40-char cap.
	slug := s[:len(s)-9]
	if n := len([]rune(slug)); n > 40 {
		t.Fatalf("slug part %q has %d chars, want <= 40", slug, n)
	}
	if strings.HasSuffix(slug, "-") {
		t.Fatalf("slug part %q must not end with a dash", slug)
	}
}

func TestQuestionSessionUniqueOnDifferentQuestions(t *testing.T) {
	// Same leading words (identical slug part), different tails: the content
	// hash must keep the session names distinct.
	a := questionSession("which repos implement agent sidecar frameworks for code reconnaissance in Go")
	b := questionSession("which repos implement agent sidecar frameworks for code reconnaissance in Rust")
	if a == b {
		t.Fatalf("different questions produced the same session name %q", a)
	}
}

func TestQuestionSessionEmptyFallsBackToDiscovery(t *testing.T) {
	s := questionSession("???")
	if !strings.HasPrefix(s, "discovery-") {
		t.Fatalf("questionSession(%q) = %q, want discovery-<hash> fallback", "???", s)
	}
}

// handleRecon session defaults mirror the CLI (ADR-0019.1 D1/D2): repo slug
// with a repo, question-derived slug without one; repo stays optional.
func TestHandleReconSessionDefaults(t *testing.T) {
	orig := askSidecar
	defer func() { askSidecar = orig }()
	var got sidecar.AskRequest
	askSidecar = func(_ context.Context, _ sidecar.Config, req sidecar.AskRequest) (*sidecar.Report, *sidecar.TurnResult, error) {
		got = req
		return &sidecar.Report{Answer: "ok"}, nil, nil
	}

	// Discovery: no repo argument at all.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "which repos do X"}
	res, err := handleRecon(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("recon without repo must not error: %+v", res)
	}
	if got.Repo != "" {
		t.Fatalf("discovery ask must pass empty repo, got %q", got.Repo)
	}
	if want := questionSession("which repos do X"); got.Session != want {
		t.Fatalf("discovery session = %q, want question-derived %q", got.Session, want)
	}

	// Repo-scoped: repo slug default, unchanged behavior.
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q", "repo": "OwnerX/RepoY"}
	if _, err := handleRecon(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got.Repo != "OwnerX/RepoY" {
		t.Fatalf("repo not passed through, got %q", got.Repo)
	}
	if got.Session != "ownerx-repoy" {
		t.Fatalf("repo-scoped session = %q, want %q", got.Session, "ownerx-repoy")
	}

	// Explicit session always wins.
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q", "session": "my-thread"}
	if _, err := handleRecon(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got.Session != "my-thread" {
		t.Fatalf("explicit session = %q, want %q", got.Session, "my-thread")
	}
}
