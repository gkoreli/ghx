package evals

import (
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func TestToolSummaryFallsBackWhenRawInputEmpty(t *testing.T) {
	got := toolSummary(sidecar.ToolCallTrace{
		ID:       "tool-1",
		Kind:     "execute",
		Title:    "gh api repos/o/r",
		RawInput: map[string]any{},
		StatusTransitions: []sidecar.ToolStatusTransition{
			{Status: "pending"},
		},
	})
	want := "execute: gh api repos/o/r (pending)"
	if got != want {
		t.Fatalf("toolSummary = %q, want %q", got, want)
	}
}

func TestToolSummaryFallsBackToIDWhenRawInputAndTitleEmpty(t *testing.T) {
	got := toolSummary(sidecar.ToolCallTrace{
		ID:       "tool-1",
		Kind:     "execute",
		RawInput: map[string]any{},
		StatusTransitions: []sidecar.ToolStatusTransition{
			{Status: "pending"},
		},
	})
	want := "execute: tool-1 (pending)"
	if got != want {
		t.Fatalf("toolSummary = %q, want %q", got, want)
	}
}
