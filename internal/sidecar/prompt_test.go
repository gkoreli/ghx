package sidecar

import (
	"strings"
	"testing"
)

func TestBuildPromptFirstTurn(t *testing.T) {
	p := BuildPrompt(Request{
		Session:  "hono-middleware",
		Repo:     "honojs/hono",
		Question: "Where is middleware composition implemented?",
	}, nil)

	for _, want := range []string{
		"ghx-sidecar",
		"honojs/hono",
		"hono-middleware",
		"Where is middleware composition implemented?",
		"<ghx-report>",
		"max 8 ghx commands",
		"normal", // default depth
		"- remote", // default backend
	} {
		if !strings.Contains(p, want) {
			t.Errorf("first-turn prompt missing %q", want)
		}
	}
	if strings.Contains(p, "Prior session context") {
		t.Error("first-turn prompt must not contain prior session context")
	}
}

func TestBuildPromptDefaults(t *testing.T) {
	p := BuildPrompt(Request{Repo: "o/r", Question: "q", Depth: "deep", AllowedBackends: []string{"remote", "codemap"}}, nil)
	if !strings.Contains(p, "deep") {
		t.Error("explicit depth not rendered")
	}
	if !strings.Contains(p, "- codemap") {
		t.Error("explicit backend not rendered")
	}
}

func TestBuildPromptFollowUpIncludesPriorContext(t *testing.T) {
	meta := &SessionMeta{
		Name:      "hono-middleware",
		Repo:      "honojs/hono",
		Scope:     "middleware composition",
		TurnCount: 2,
	}
	p := BuildPrompt(Request{
		Session:  "hono-middleware",
		Repo:     "honojs/hono",
		Question: "How do errors propagate through that path?",
	}, meta)

	if !strings.Contains(p, "Prior session context") {
		t.Fatal("follow-up prompt missing prior session context block")
	}
	if !strings.Contains(p, "Turns completed: 2") {
		t.Error("follow-up prompt missing turn count")
	}
	if !strings.Contains(p, "middleware composition") {
		t.Error("follow-up prompt missing scope")
	}
}

func TestBuildPromptZeroTurnMetaOmitsPriorContext(t *testing.T) {
	// A freshly initialized session (TurnCount 0) has nothing to carry over.
	meta := &SessionMeta{Name: "s", Repo: "o/r", TurnCount: 0}
	p := BuildPrompt(Request{Repo: "o/r", Question: "q"}, meta)
	if strings.Contains(p, "Prior session context") {
		t.Error("zero-turn session must not render prior context")
	}
}
