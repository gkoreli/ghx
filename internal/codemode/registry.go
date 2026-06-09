package codemode

import (
	"fmt"
	"sort"
	"strings"
)

// Tool describes a registered tool for codemode.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Func        ToolFunc       `json:"-"`      // from executor.go
	Schema      map[string]any `json:"schema"` // JSON schema for type generation
	Returns     string         `json:"-"`      // TypeScript return type (e.g. "ExploreResult"), empty = "any"
}

// Registry holds registered tools.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry returns an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. Panics if the tool name is already registered.
func (r *Registry) Register(t Tool) {
	if _, exists := r.tools[t.Name]; exists {
		panic(fmt.Sprintf("codemode: tool %q already registered", t.Name))
	}
	r.tools[t.Name] = t
}

// Get returns a pointer to a copy of the tool, or nil if not found.
func (r *Registry) Get(name string) *Tool {
	if t, ok := r.tools[name]; ok {
		return &t
	}
	return nil
}

// List returns all registered tools sorted by name.
func (r *Registry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

// Search returns tools matching the query (case-insensitive substring on name and description).
func (r *Registry) Search(query string) []Tool {
	query = strings.ToLower(query)
	var results []Tool
	for _, t := range r.tools {
		if strings.Contains(strings.ToLower(t.Name), query) ||
			strings.Contains(strings.ToLower(t.Description), query) {
			results = append(results, t)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})
	return results
}

// Tools returns a slice of all registered tools for passing to Executor.Execute().
func (r *Registry) Tools() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}
