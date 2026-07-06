package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	logFileName                         = "logs.jsonl"
	genAIInferenceOperationDetailsEvent = "gen_ai.client.inference.operation.details"
	genAIInputMessagesAttribute         = "gen_ai.input.messages"
	genAIOutputMessagesAttribute        = "gen_ai.output.messages"
	genAICaptureMessageContentEnv       = "OTEL_INSTRUMENTATION_GENAI_CAPTURE_MESSAGE_CONTENT"
	genAIUsageReasoningOutputTokensAttr = "gen_ai.usage.reasoning.output_tokens"
	genAIEvaluationResultEvent          = "gen_ai.evaluation.result"
	genAIEvaluationNameAttribute        = "gen_ai.evaluation.name"
	genAIEvaluationScoreValueAttribute  = "gen_ai.evaluation.score.value"
)

type episodeLogRecord struct {
	Time       time.Time
	Span       trace.SpanContext
	EventName  string
	Attributes []attribute.KeyValue
}

type genAIMessage struct {
	Role  string             `json:"role"`
	Parts []genAIMessagePart `json:"parts"`
}

type genAIMessagePart struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

func writeOTLPJSONLogs(runDir string, ep *Episode, records []episodeLogRecord) error {
	if len(records) == 0 {
		return nil
	}
	data := &logspb.LogsData{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource:  protoLogResource(runDir, ep),
				ScopeLogs: []*logspb.ScopeLogs{{Scope: instrumentationScopeName(), LogRecords: protoLogRecords(records)}},
			},
		},
	}
	line, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal OTLP logs json: %w", err)
	}
	line, err = hexEncodeSpanIDs(line)
	if err != nil {
		return fmt.Errorf("hex-encode OTLP log span ids: %w", err)
	}
	line = append(line, '\n')

	path := filepath.Join(runDir, logFileName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

func protoLogResource(runDir string, ep *Episode) *resourcepb.Resource {
	return protoResource(sdkresource.NewSchemaless(resourceAttributes(runDir, ep)...))
}

func instrumentationScopeName() *commonpb.InstrumentationScope {
	return &commonpb.InstrumentationScope{Name: tracerName, Version: otelSemconvVersion}
}

func protoLogRecords(records []episodeLogRecord) []*logspb.LogRecord {
	out := make([]*logspb.LogRecord, 0, len(records))
	for _, record := range records {
		if !record.Span.IsValid() {
			continue
		}
		tid := record.Span.TraceID()
		sid := record.Span.SpanID()
		at := nonZeroTime(record.Time, time.Now().UTC())
		out = append(out, &logspb.LogRecord{
			TimeUnixNano:         uint64(maxInt64(0, at.UnixNano())),
			ObservedTimeUnixNano: uint64(maxInt64(0, time.Now().UTC().UnixNano())),
			EventName:            record.EventName,
			Body:                 &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: record.EventName}},
			Attributes:           keyValues(record.Attributes),
			TraceId:              tid[:],
			SpanId:               sid[:],
			Flags:                uint32(record.Span.TraceFlags()),
		})
	}
	return out
}

func turnContentLogs(span trace.SpanContext, ep *Episode, turn TurnRecord, at time.Time) []episodeLogRecord {
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
	return []episodeLogRecord{{
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
