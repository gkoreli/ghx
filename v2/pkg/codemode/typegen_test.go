package codemode

import (
	"strings"
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
	if !strings.Contains(result, "declare const codemode") {
		t.Error("Missing declare const codemode")
	}
	if !strings.Contains(result, "type ReposInput") {
		t.Error("Missing ReposInput type")
	}
	if !strings.Contains(result, "repos: (input: ReposInput) => any;") {
		t.Error("Missing repos tool entry")
	}
	if !strings.Contains(result, "query: string") {
		t.Error("Missing required query parameter")
	}
	if !strings.Contains(result, "limit?: number") {
		t.Error("Missing optional limit parameter")
	}
	if !strings.Contains(result, "Search repos with README preview") {
		t.Error("Missing JSDoc comment")
	}
}
