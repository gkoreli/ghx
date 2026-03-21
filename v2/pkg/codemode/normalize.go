package codemode

import (
	"fmt"
	"regexp"
	"strings"
)

// Normalize cleans LLM-generated code for execution.
// Strips markdown fences, trims whitespace, validates non-empty.
// Returns cleaned code or error if empty/invalid.
func Normalize(code string) (string, error) {
	code = strings.TrimSpace(code)

	// Try to match markdown fences: ```[language]\n...\n```
	re := regexp.MustCompile(`(?s)^` + "`" + `{3}[^\n]*\n(.*)\n` + "`" + `{3}$`)
	if matches := re.FindStringSubmatch(code); matches != nil {
		code = matches[1]
	}

	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("empty code after normalization")
	}
	return code, nil
}
