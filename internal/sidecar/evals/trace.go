package evals

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

func convertSidecarTrace(in sidecar.ToolCallTrace) ToolCallTrace {
	out := ToolCallTrace{
		ID:            in.ID,
		Kind:          in.Kind,
		Title:         in.Title,
		RawInput:      in.RawInput,
		OutputSize:    in.OutputSize,
		OutputExcerpt: in.OutputExcerpt,
	}
	for _, st := range in.StatusTransitions {
		out.StatusTransitions = append(out.StatusTransitions, ToolStatusTransition{Status: st.Status, At: st.At})
	}
	return out
}

func rebuildToolSummaries(turn *TurnRecord) {
	turn.ToolCalls = turn.ToolCalls[:0]
	for _, tr := range turn.ToolTraces {
		turn.ToolCalls = append(turn.ToolCalls, toolSummary(tr))
	}
}

func toolSummary(tr ToolCallTrace) string {
	status := ""
	if n := len(tr.StatusTransitions); n > 0 {
		status = tr.StatusTransitions[n-1].Status
	}
	input := resolvedToolInput(tr)
	if tr.Kind != "" {
		input = tr.Kind + ": " + input
	}
	if status != "" {
		return fmt.Sprintf("%s (%s)", input, status)
	}
	return input
}

func resolvedToolInput(tr ToolCallTrace) string {
	switch v := tr.RawInput.(type) {
	case string:
		if strings.TrimSpace(v) != "" {
			return v
		}
	case map[string]any:
		if s, ok := v["command"].(string); ok && strings.TrimSpace(s) != "" {
			return commandWithArgs(s, v["arguments"])
		}
		if s, ok := v["cmd"].(string); ok && strings.TrimSpace(s) != "" {
			return commandWithArgs(s, v["args"])
		}
		if len(v) > 0 {
			data, err := json.Marshal(v)
			if err == nil && string(data) != "{}" {
				return string(data)
			}
		}
	case nil:
		// Fall through to the title/id fallback below.
	default:
		if tr.RawInput != nil {
			if data, err := json.Marshal(tr.RawInput); err == nil && string(data) != "{}" {
				return string(data)
			}
		}
	}
	if strings.TrimSpace(tr.Title) != "" {
		return tr.Title
	}
	return tr.ID
}

func rawTokenInvokesGhx(token string) bool {
	token = strings.Trim(strings.ToLower(token), `"'()[],:;`)
	token = strings.TrimSuffix(token, ".cmd")
	if token == "" {
		return false
	}
	base := token
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	if base == "ghx" || base == "ghx.exe" {
		return true
	}
	return token == "@gkoreli/ghx" || strings.HasSuffix(token, "/@gkoreli/ghx")
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
		if strings.TrimSpace(args) != "" {
			parts = append(parts, args)
		}
	}
	if len(parts) == 0 {
		return command
	}
	return command + " " + strings.Join(parts, " ")
}

func appendTraceProjections(ep *Episode, turn TurnRecord) {
	for _, tr := range turn.ToolTraces {
		idx := len(ep.Actions)
		at := firstStatusTime(tr)
		ep.Actions = append(ep.Actions, Action{
			ActionIndex: idx,
			Turn:        turn.Turn,
			Type:        "tool",
			ToolCallID:  tr.ID,
			Kind:        tr.Kind,
			Name:        tr.Kind,
			Input:       resolvedToolInput(tr),
			At:          at,
		})
		ep.Observations = append(ep.Observations, Observation{
			ActionIndex: idx,
			Turn:        turn.Turn,
			ToolCallID:  tr.ID,
			Text:        tr.OutputExcerpt,
			OutputSize:  tr.OutputSize,
			At:          lastStatusTime(tr),
		})
	}
}

func firstStatusTime(tr ToolCallTrace) time.Time {
	if len(tr.StatusTransitions) > 0 {
		return tr.StatusTransitions[0].At
	}
	return time.Time{}
}

func lastStatusTime(tr ToolCallTrace) time.Time {
	if n := len(tr.StatusTransitions); n > 0 {
		return tr.StatusTransitions[n-1].At
	}
	return time.Time{}
}

func commandsFromEpisode(ep *Episode) []string {
	var out []string
	for _, a := range ep.Actions {
		if strings.TrimSpace(a.Input) != "" {
			out = append(out, a.Input)
		}
	}
	if len(out) > 0 {
		return out
	}
	for _, t := range ep.Turns {
		out = append(out, t.ToolCalls...)
	}
	return out
}

func invokesGhx(ep *Episode) bool {
	for _, cmd := range commandsFromEpisode(ep) {
		fields := strings.Fields(strings.ToLower(cmd))
		for _, f := range fields {
			if rawTokenInvokesGhx(f) {
				return true
			}
		}
	}
	return false
}
