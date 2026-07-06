package evals

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	logFileName                         = telemetry.LogFileName
	genAIInferenceOperationDetailsEvent = "gen_ai.client.inference.operation.details"
	genAIInputMessagesAttribute         = "gen_ai.input.messages"
	genAIOutputMessagesAttribute        = "gen_ai.output.messages"
	genAICaptureMessageContentEnv       = "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT"
	genAIUsageReasoningOutputTokensAttr = "gen_ai.usage.reasoning.output_tokens"
	genAIEvaluationResultEvent          = "gen_ai.evaluation.result"
	genAIEvaluationNameAttribute        = "gen_ai.evaluation.name"
	genAIEvaluationScoreValueAttribute  = "gen_ai.evaluation.score.value"
	genAIEvaluationExplanationAttribute = "gen_ai.evaluation.explanation"
	genAIRequestModelAttribute          = "gen_ai.request.model"
)

type genAIMessage struct {
	Role  string             `json:"role"`
	Parts []genAIMessagePart `json:"parts"`
}

type genAIMessagePart struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

// turnContentLogs builds the eval-specific GenAI content record for one turn,
// gated by the OTel capture-content env var. The generic OTLP log writer lives
// in the telemetry package (ADR-0022 D1); this only shapes attributes.
func turnContentLogs(span trace.SpanContext, ep *Episode, turn TurnRecord, at time.Time) []telemetry.LogRecord {
	if !captureGenAIMessageContent() {
		return nil
	}
	input, inputOK := marshalMessages(inputMessages(turn))
	output, outputOK := marshalMessages(outputMessages(turn))
	if !inputOK && !outputOK {
		return nil
	}
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.system", "ghx-evals"),
		attribute.String("gen_ai.operation.name", "chat"),
		attribute.String("ghx.eval.episode_id", ep.ID),
		attribute.String("ghx.eval.task_id", ep.TaskID),
		attribute.String("ghx.eval.profile", string(ep.Profile)),
		attribute.Int("ghx.eval.turn", turn.Turn),
	}
	if inputOK {
		attrs = append(attrs, attribute.String(genAIInputMessagesAttribute, input))
	}
	if outputOK {
		attrs = append(attrs, attribute.String(genAIOutputMessagesAttribute, output))
	}
	return []telemetry.LogRecord{{
		Time:       at,
		Span:       span,
		EventName:  genAIInferenceOperationDetailsEvent,
		Attributes: attrs,
	}}
}

func inputMessages(turn TurnRecord) []genAIMessage {
	msgs := []genAIMessage{{
		Role:  "user",
		Parts: []genAIMessagePart{{Type: "text", Content: turn.Question}},
	}}
	for _, tool := range turn.ToolTraces {
		if strings.TrimSpace(tool.OutputExcerpt) == "" {
			continue
		}
		msgs = append(msgs, genAIMessage{
			Role:  "tool",
			Parts: []genAIMessagePart{{Type: "text", Content: tool.OutputExcerpt}},
		})
	}
	return msgs
}

func outputMessages(turn TurnRecord) []genAIMessage {
	var parts []genAIMessagePart
	if strings.TrimSpace(turn.Thinking) != "" {
		parts = append(parts, genAIMessagePart{Type: "reasoning", Content: turn.Thinking})
	}
	if strings.TrimSpace(turn.Text) != "" {
		parts = append(parts, genAIMessagePart{Type: "text", Content: turn.Text})
	}
	if len(parts) == 0 {
		return nil
	}
	return []genAIMessage{{Role: "assistant", Parts: parts}}
}

func marshalMessages(messages []genAIMessage) (string, bool) {
	if len(messages) == 0 {
		return "", false
	}
	data, err := json.Marshal(messages)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func captureGenAIMessageContent() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(genAICaptureMessageContentEnv)), "true")
}
