package sidecar

import (
	"encoding/json"
	"fmt"
	"strings"

	acp "github.com/coder/acp-go-sdk"
)

// ContentSize approximates the character size of tool-call content blocks
// plus raw output. JSON length is used for non-string raw output.
func ContentSize(content []acp.ToolCallContent, rawOutput any) int {
	n := 0
	for _, c := range content {
		if c.Content != nil && c.Content.Content.Text != nil {
			n += len(c.Content.Content.Text.Text)
		}
	}
	switch v := rawOutput.(type) {
	case nil:
	case string:
		n += len(v)
	default:
		if data, err := json.Marshal(v); err == nil {
			n += len(data)
		}
	}
	return n
}

// ToolOutputText returns a bounded textual representation of tool output.
// ContentSize remains the exact accounting source; this is only for audit
// excerpts in local eval artifacts.
func ToolOutputText(content []acp.ToolCallContent, rawOutput any) string {
	var out string
	for _, c := range content {
		if c.Content != nil && c.Content.Content.Text != nil {
			out += c.Content.Content.Text.Text
		} else if data, err := json.Marshal(c); err == nil {
			out += string(data)
		}
	}
	switch v := rawOutput.(type) {
	case nil:
	case string:
		out += v
	default:
		if data, err := json.Marshal(v); err == nil {
			out += string(data)
		}
	}
	return out
}

func toolSummary(tr ToolCallTrace) string {
	status := ""
	if n := len(tr.StatusTransitions); n > 0 {
		status = tr.StatusTransitions[n-1].Status
	}
	fallback := tr.Title
	if fallback == "" {
		fallback = tr.ID
	}
	input := resolveToolInput(tr.RawInput, fallback)
	if tr.Kind != "" {
		input = tr.Kind + ": " + input
	}
	if status != "" {
		return fmt.Sprintf("%s (%s)", input, status)
	}
	return input
}

func resolveToolInput(raw any, fallback string) string {
	switch v := raw.(type) {
	case string:
		if v != "" {
			return v
		}
	case map[string]any:
		if s, ok := v["command"].(string); ok && s != "" {
			return commandWithArgs(s, v["arguments"])
		}
		if s, ok := v["cmd"].(string); ok && s != "" {
			return commandWithArgs(s, v["args"])
		}
		if len(v) > 0 {
			data, err := json.Marshal(v)
			if err == nil && string(data) != "{}" {
				return string(data)
			}
		}
	case nil:
		// Fall through to fallback below.
	default:
		if raw != nil {
			if data, err := json.Marshal(raw); err == nil && string(data) != "{}" {
				return string(data)
			}
		}
	}
	return fallback
}

func commandWithArgs(command string, rawArgs any) string {
	var parts []string
	switch args := rawArgs.(type) {
	case []string:
		parts = args
	case []any:
		for _, a := range args {
			parts = append(parts, fmt.Sprint(a))
		}
	case string:
		if args != "" {
			parts = append(parts, args)
		}
	}
	if len(parts) == 0 {
		return command
	}
	return command + " " + strings.Join(parts, " ")
}

func appendExcerpt(tr *ToolCallTrace, text string) {
	const max = 2048
	if text == "" || len(tr.OutputExcerpt) >= max {
		return
	}
	remain := max - len(tr.OutputExcerpt)
	if len(text) > remain {
		text = text[:remain]
	}
	tr.OutputExcerpt += text
}
