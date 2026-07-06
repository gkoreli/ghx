package telemetry

import (
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// LogRecord is a neutral, span-correlated OTLP log record. Callers build these
// (e.g. GenAI content records) and hand them to WriteLogs; the domain-specific
// attribute keys live entirely in the caller's slice.
type LogRecord struct {
	Time       time.Time
	Span       trace.SpanContext
	EventName  string
	Attributes []attribute.KeyValue
}

// WriteLogs appends one OTLP LogsData JSON object (spec-exact, hex IDs) to path
// carrying the supplied records under a single resource/scope. It is a no-op
// when there are no records, matching the previous eval-path behavior so log
// output stays byte-identical.
func WriteLogs(path string, resourceAttrs []attribute.KeyValue, scopeName, scopeVersion string, records []LogRecord) error {
	if len(records) == 0 {
		return nil
	}
	data := &logspb.LogsData{
		ResourceLogs: []*logspb.ResourceLogs{
			{
				Resource:  protoResource(sdkresource.NewSchemaless(resourceAttrs...)),
				ScopeLogs: []*logspb.ScopeLogs{{Scope: scope(scopeName, scopeVersion), LogRecords: protoLogRecords(records)}},
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

	// Shared append; the per-path mutex in AppendJSONLine serializes concurrent
	// writers under episode-level parallelism (ADR-0025 D3, jsonl_writer.go).
	return AppendJSONLine(path, line)
}

// scope is the shared instrumentation-scope shape (name + version only) used by
// both the log and metric writers, mirroring the previous evals helper.
func scope(name, version string) *commonpb.InstrumentationScope {
	return &commonpb.InstrumentationScope{Name: name, Version: version}
}

func protoLogRecords(records []LogRecord) []*logspb.LogRecord {
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

func nonZeroTime(t, fallback time.Time) time.Time {
	if t.IsZero() {
		return fallback
	}
	return t
}
