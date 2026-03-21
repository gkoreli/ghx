package codemode

import (
	"testing"
)

func TestGenerateTypes(t *testing.T) {
	tools := []Tool{
		{
			Name:        "repos",
			Description: "Search repos with README preview",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string"},
					"limit": map[string]any{"type": "number"},
				},
				"required": []any{"query"},
			},
		},
		{
			Name:        "search",
			Description: "Code search with text matches",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query":    map[string]any{"type": "string"},
					"limit":    map[string]any{"type": "number"},
					"fullMode": map[string]any{"type": "boolean"},
				},
				"required": []any{"query"},
			},
		},
	}

	result := GenerateTypes(tools)

	// Verify output contains expected patterns
	if !contains(result, "declare function repos") {
		t.Error("Missing repos function declaration")
	}
	if !contains(result, "declare function search") {
		t.Error("Missing search function declaration")
	}
	if !contains(result, "query: string") {
		t.Error("Missing required query parameter")
	}
	if !contains(result, "limit?: number") {
		t.Error("Missing optional limit parameter")
	}
	if !contains(result, "Search repos with README preview") {
		t.Error("Missing JSDoc comment")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
