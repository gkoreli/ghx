package codemode

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateTypes produces TypeScript type declarations from registered tools.
// Output is a string suitable for injecting into LLM system prompts.
func GenerateTypes(tools []Tool) string {
	// Sort tools alphabetically for deterministic output
	sortedTools := make([]Tool, len(tools))
	copy(sortedTools, tools)
	sort.Slice(sortedTools, func(i, j int) bool {
		return sortedTools[i].Name < sortedTools[j].Name
	})

	// Generate input types for each tool
	var typeDecls []string
	for _, tool := range sortedTools {
		argsType := buildArgsType(tool.Schema)
		typeDecl := fmt.Sprintf("type %sInput = %s", capitalize(tool.Name), argsType)
		typeDecls = append(typeDecls, typeDecl)
	}

	// Generate codemode object with all tools
	var toolEntries []string
	for _, tool := range sortedTools {
		entry := fmt.Sprintf("  /** %s */\n  %s: (input: %sInput) => Promise<any>;", tool.Description, tool.Name, capitalize(tool.Name))
		toolEntries = append(toolEntries, entry)
	}

	result := strings.Join(typeDecls, "\n") + "\n\n"
	result += "declare const codemode: {\n"
	result += strings.Join(toolEntries, "\n")
	result += "\n}"

	return result
}

// capitalize returns the string with first letter uppercase.
func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// buildArgsType constructs TypeScript object type from JSON schema.
func buildArgsType(schema map[string]any) string {
	if schema == nil || len(schema) == 0 {
		return "Record<string, any>"
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return "Record<string, any>"
	}

	required := extractRequired(schema)
	requiredSet := make(map[string]bool)
	for _, r := range required {
		requiredSet[r] = true
	}

	// Sort keys for consistent output
	var keys []string
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var fields []string
	for _, key := range keys {
		prop, ok := props[key].(map[string]any)
		if !ok {
			fields = append(fields, fmt.Sprintf("%s: any", key))
			continue
		}

		tsType := schemaTypeToTS(prop)
		optional := ""
		if !requiredSet[key] {
			optional = "?"
		}
		fields = append(fields, fmt.Sprintf("%s%s: %s", key, optional, tsType))
	}

	return "{ " + strings.Join(fields, "; ") + " }"
}

// schemaTypeToTS converts JSON schema type to TypeScript type.
func schemaTypeToTS(prop map[string]any) string {
	schemaType, ok := prop["type"].(string)
	if !ok {
		return "any"
	}

	switch schemaType {
	case "string":
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		return "any[]"
	default:
		return "any"
	}
}

// extractRequired gets the required array from schema.
func extractRequired(schema map[string]any) []string {
	required, ok := schema["required"].([]any)
	if !ok {
		return nil
	}

	var result []string
	for _, r := range required {
		if s, ok := r.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
