package codemode

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateTypes produces TypeScript type declarations from registered tools.
// Output is a string suitable for injecting into LLM system prompts.
func GenerateTypes(tools []Tool) string {
	var declarations []string

	for _, tool := range tools {
		// JSDoc comment
		decl := fmt.Sprintf("/** %s */\n", tool.Description)

		// Build args type from schema
		argsType := buildArgsType(tool.Schema)

		// Function declaration
		decl += fmt.Sprintf("declare function %s(args: %s): any;", tool.Name, argsType)
		declarations = append(declarations, decl)
	}

	return strings.Join(declarations, "\n\n")
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
