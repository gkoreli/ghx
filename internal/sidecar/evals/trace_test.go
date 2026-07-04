package evals

import "testing"

func TestToolSummaryFallsBackWhenRawInputEmpty(t *testing.T) {
	got := toolSummary(ToolCallTrace{
		ID:       "tool-1",
		Kind:     "execute",
		Title:    "gh api repos/o/r",
		RawInput: map[string]any{},
		StatusTransitions: []ToolStatusTransition{
			{Status: "pending"},
		},
	})
	want := "execute: gh api repos/o/r (pending)"
	if got != want {
		t.Fatalf("toolSummary = %q, want %q", got, want)
	}
}

func TestToolSummaryFallsBackToIDWhenRawInputAndTitleEmpty(t *testing.T) {
	got := toolSummary(ToolCallTrace{
		ID:       "tool-1",
		Kind:     "execute",
		RawInput: map[string]any{},
		StatusTransitions: []ToolStatusTransition{
			{Status: "pending"},
		},
	})
	want := "execute: tool-1 (pending)"
	if got != want {
		t.Fatalf("toolSummary = %q, want %q", got, want)
	}
}
