package codemode

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	fenceRe      = regexp.MustCompile(`(?s)^` + "`" + `{3}[^\n]*\n(.*)\n` + "`" + `{3}$`)
	namedFuncRe  = regexp.MustCompile(`^function\s+(\w+)\s*\(`)
	bareExprRe   = regexp.MustCompile(`^[a-zA-Z_$][\w$]*(\.[a-zA-Z_$][\w$]*|\[.+\])*$`)
)

// Normalize cleans LLM-generated code for execution.
// Handles 4 cases:
// 1. Markdown fences: strips them
// 2. Async arrow functions: passes through as-is
// 3. Named function definitions: appends function call
// 4. Bare expressions: prepends return to last line if needed
func Normalize(code string) (string, error) {
	code = strings.TrimSpace(code)

	// Strip markdown fences
	if matches := fenceRe.FindStringSubmatch(code); matches != nil {
		code = matches[1]
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("empty code after normalization")
	}

	// Case 1: async arrow or already an expression (starts with async or () → pass through
	if strings.HasPrefix(code, "async") || strings.HasPrefix(code, "(") {
		return code, nil
	}

	// Case 2: named function definition → append function call
	if match := namedFuncRe.FindStringSubmatch(code); match != nil {
		funcName := match[1]
		code = code + "; " + funcName + "()"
		return code, nil
	}

	// Case 3: bare expression as last statement → prepend return
	lines := strings.Split(code, "\n")
	if len(lines) > 0 {
		lastLine := strings.TrimSpace(lines[len(lines)-1])
		// Check if last line looks like a bare expression (no assignment, no block, no semicolon, no keyword)
		if lastLine != "" && !strings.HasSuffix(lastLine, ";") && !strings.HasSuffix(lastLine, "{") &&
			!strings.HasPrefix(lastLine, "if ") && !strings.HasPrefix(lastLine, "for ") &&
			!strings.HasPrefix(lastLine, "while ") && !strings.HasPrefix(lastLine, "const ") &&
			!strings.HasPrefix(lastLine, "let ") && !strings.HasPrefix(lastLine, "var ") &&
			!strings.HasPrefix(lastLine, "function ") && !strings.HasPrefix(lastLine, "class ") &&
			bareExprRe.MatchString(lastLine) {
			lines[len(lines)-1] = "return " + lastLine
			code = strings.Join(lines, "\n")
		}
	}

	return code, nil
}
