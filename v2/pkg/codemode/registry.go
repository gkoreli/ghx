package codemode

// Tool represents a callable function with schema for LLM context generation.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}
