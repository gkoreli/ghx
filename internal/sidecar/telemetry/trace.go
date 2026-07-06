// Package telemetry is the shared visibility runtime for the agent sidecar
// framework (ADR-0022). It owns everything generic about emitting spec-exact
// OTLP JSONL artifacts — the protobuf-JSON file writers (hex ID handling and
// all), tracer-provider construction, and the token/duration/size metric
// builders — so both the eval path (SAFE) and the production `sidecar.Ask`
// path (SAF) emit the same trace/log/metric artifacts from the same code.
// Domain-specific attribute shaping (eval reward spans, production session
// spans) is layered on top by the caller; this package stays neutral.
//
// The OTLP/JSON output follows the spec exactly, including its deviation from
// the proto3 JSON mapping for ID fields (trace_id/span_id/parent_span_id are
// lowercase hex, not base64) — see hexEncodeSpanIDs. This is a pure relocation
// of the emission layer that previously lived in package evals; eval artifact
// output is byte-identical after the move (ADR-0022 D1 extraction rule).
package telemetry

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// Artifact file names for a session/run directory. Both the eval path and the
// production path write these same three files alongside a reports/ directory.
const (
	TraceFileName  = "traces.jsonl"
	LogFileName    = "logs.jsonl"
	MetricFileName = "metrics.jsonl"
)

const traceFileName = TraceFileName

// NewTraceExporter returns an OTLP JSON File span exporter that appends one
// TracesData JSON object per export batch to <dir>/traces.jsonl.
func NewTraceExporter(dir string) sdktrace.SpanExporter {
	return newOTLPJSONFileExporter(dir)
}

// otlpJSONFileExporter writes OTLP JSON File records: one TracesData JSON
// object per line, appended for each SDK export batch.
//
// Concurrency (ADR-0025 D3): each episode/session builds a fresh TracerProvider
// — and therefore a fresh exporter instance — all pointed at the same
// <dir>/traces.jsonl. A per-instance mutex would NOT serialize two concurrent
// exporters writing the same file, so appends go through the shared per-path
// lock in AppendJSONLine instead.
type otlpJSONFileExporter struct {
	path string
}

func newOTLPJSONFileExporter(runDir string) *otlpJSONFileExporter {
	return &otlpJSONFileExporter{path: filepath.Join(runDir, traceFileName)}
}

func (e *otlpJSONFileExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if len(spans) == 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	data := &tracepb.TracesData{ResourceSpans: resourceSpans(spans)}
	if len(data.ResourceSpans) == 0 {
		return nil
	}
	sanitizeProtoStrings(data)
	line, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal OTLP trace json: %w", err)
	}
	line, err = hexEncodeSpanIDs(line)
	if err != nil {
		return fmt.Errorf("hex-encode OTLP span ids: %w", err)
	}
	line = append(line, '\n')

	return AppendJSONLine(e.path, line)
}

func (e *otlpJSONFileExporter) Shutdown(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// hexEncodeSpanIDs rewrites the base64 ID strings protojson produces for
// proto bytes fields into lowercase hex. The OTLP/JSON spec deviates from
// the proto3 JSON mapping for exactly these fields — trace_id, span_id,
// parent_span_id MUST be hex — and spec-compliant receivers (Jaeger,
// otel-desktop-viewer, the collector) reject base64 IDs.
func hexEncodeSpanIDs(line []byte) ([]byte, error) {
	var doc any
	if err := json.Unmarshal(line, &doc); err != nil {
		return nil, err
	}
	transcodeSpanIDs(doc)
	return json.Marshal(doc)
}

func transcodeSpanIDs(node any) {
	switch v := node.(type) {
	case map[string]any:
		for k, child := range v {
			switch k {
			case "traceId", "spanId", "parentSpanId":
				s, ok := child.(string)
				if !ok || s == "" {
					continue
				}
				raw, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					continue
				}
				v[k] = hex.EncodeToString(raw)
			default:
				transcodeSpanIDs(child)
			}
		}
	case []any:
		for _, child := range v {
			transcodeSpanIDs(child)
		}
	}
}

func resourceSpans(spans []sdktrace.ReadOnlySpan) []*tracepb.ResourceSpans {
	if len(spans) == 0 {
		return nil
	}

	type key struct {
		resource attribute.Distinct
		scope    instrumentation.Scope
	}
	resourceMap := map[attribute.Distinct]*tracepb.ResourceSpans{}
	scopeMap := map[key]*tracepb.ScopeSpans{}

	for _, span := range spans {
		if span == nil {
			continue
		}
		resourceKey := span.Resource().Equivalent()
		scopeKey := key{resource: resourceKey, scope: span.InstrumentationScope()}
		scopeSpans, seenScope := scopeMap[scopeKey]
		if !seenScope {
			scopeSpans = &tracepb.ScopeSpans{
				Scope:     instrumentationScope(span.InstrumentationScope()),
				Spans:     []*tracepb.Span{},
				SchemaUrl: span.InstrumentationScope().SchemaURL,
			}
			scopeMap[scopeKey] = scopeSpans
		}
		scopeSpans.Spans = append(scopeSpans.Spans, protoSpan(span))

		rs, seenResource := resourceMap[resourceKey]
		if !seenResource {
			resourceMap[resourceKey] = &tracepb.ResourceSpans{
				Resource:   protoResource(span.Resource()),
				ScopeSpans: []*tracepb.ScopeSpans{scopeSpans},
				SchemaUrl:  span.Resource().SchemaURL(),
			}
			continue
		}
		if !seenScope {
			rs.ScopeSpans = append(rs.ScopeSpans, scopeSpans)
		}
	}

	out := make([]*tracepb.ResourceSpans, 0, len(resourceMap))
	for _, rs := range resourceMap {
		out = append(out, rs)
	}
	return out
}

func protoSpan(span sdktrace.ReadOnlySpan) *tracepb.Span {
	tid := span.SpanContext().TraceID()
	sid := span.SpanContext().SpanID()
	out := &tracepb.Span{
		TraceId:                tid[:],
		SpanId:                 sid[:],
		TraceState:             validUTF8(span.SpanContext().TraceState().String()),
		Name:                   validUTF8(span.Name()),
		Kind:                   spanKind(span.SpanKind()),
		StartTimeUnixNano:      uint64(maxInt64(0, span.StartTime().UnixNano())),
		EndTimeUnixNano:        uint64(maxInt64(0, span.EndTime().UnixNano())),
		Attributes:             keyValues(span.Attributes()),
		Events:                 events(span.Events()),
		Links:                  links(span.Links()),
		Status:                 status(span.Status().Code, span.Status().Description),
		DroppedAttributesCount: clampUint32(span.DroppedAttributes()),
		DroppedEventsCount:     clampUint32(span.DroppedEvents()),
		DroppedLinksCount:      clampUint32(span.DroppedLinks()),
		Flags:                  spanFlags(span.SpanContext().TraceFlags(), span.Parent()),
	}
	if psid := span.Parent().SpanID(); psid.IsValid() {
		out.ParentSpanId = psid[:]
	}
	return out
}

func protoResource(r *sdkresource.Resource) *resourcepb.Resource {
	if r == nil {
		return nil
	}
	return &resourcepb.Resource{Attributes: iteratorValues(r.Iter())}
}

func instrumentationScope(scope instrumentation.Scope) *commonpb.InstrumentationScope {
	if scope == (instrumentation.Scope{}) {
		return nil
	}
	return &commonpb.InstrumentationScope{
		Name:       validUTF8(scope.Name),
		Version:    validUTF8(scope.Version),
		Attributes: iteratorValues(scope.Attributes.Iter()),
	}
}

func keyValues(attrs []attribute.KeyValue) []*commonpb.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]*commonpb.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, keyValue(attr))
	}
	return out
}

func iteratorValues(iter attribute.Iterator) []*commonpb.KeyValue {
	if iter.Len() == 0 {
		return nil
	}
	out := make([]*commonpb.KeyValue, 0, iter.Len())
	for iter.Next() {
		out = append(out, keyValue(iter.Attribute()))
	}
	return out
}

func keyValue(attr attribute.KeyValue) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: validUTF8(string(attr.Key)), Value: anyValue(attr.Value)}
}

func anyValue(v attribute.Value) *commonpb.AnyValue {
	out := &commonpb.AnyValue{}
	switch v.Type() {
	case attribute.BOOL:
		out.Value = &commonpb.AnyValue_BoolValue{BoolValue: v.AsBool()}
	case attribute.BOOLSLICE:
		out.Value = &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: boolValues(v.AsBoolSlice())}}
	case attribute.INT64:
		out.Value = &commonpb.AnyValue_IntValue{IntValue: v.AsInt64()}
	case attribute.INT64SLICE:
		out.Value = &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: int64Values(v.AsInt64Slice())}}
	case attribute.FLOAT64:
		out.Value = &commonpb.AnyValue_DoubleValue{DoubleValue: v.AsFloat64()}
	case attribute.FLOAT64SLICE:
		out.Value = &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: float64Values(v.AsFloat64Slice())}}
	case attribute.STRING:
		out.Value = &commonpb.AnyValue_StringValue{StringValue: validUTF8(v.AsString())}
	case attribute.STRINGSLICE:
		out.Value = &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: stringValues(v.AsStringSlice())}}
	case attribute.BYTESLICE:
		out.Value = &commonpb.AnyValue_BytesValue{BytesValue: v.AsByteSlice()}
	case attribute.SLICE:
		out.Value = &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: attributeValues(v.AsSlice())}}
	case attribute.EMPTY:
	}
	return out
}

func boolValues(vals []bool) []*commonpb.AnyValue {
	out := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		out[i] = &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: v}}
	}
	return out
}

func int64Values(vals []int64) []*commonpb.AnyValue {
	out := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		out[i] = &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}
	}
	return out
}

func float64Values(vals []float64) []*commonpb.AnyValue {
	out := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		out[i] = &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: v}}
	}
	return out
}

func stringValues(vals []string) []*commonpb.AnyValue {
	out := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		out[i] = &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: validUTF8(v)}}
	}
	return out
}

func attributeValues(vals []attribute.Value) []*commonpb.AnyValue {
	out := make([]*commonpb.AnyValue, len(vals))
	for i, v := range vals {
		out[i] = anyValue(v)
	}
	return out
}

func events(es []sdktrace.Event) []*tracepb.Span_Event {
	if len(es) == 0 {
		return nil
	}
	out := make([]*tracepb.Span_Event, len(es))
	for i, e := range es {
		out[i] = &tracepb.Span_Event{
			Name:                   validUTF8(e.Name),
			TimeUnixNano:           uint64(maxInt64(0, e.Time.UnixNano())),
			Attributes:             keyValues(e.Attributes),
			DroppedAttributesCount: clampUint32(e.DroppedAttributeCount),
		}
	}
	return out
}

func links(ls []sdktrace.Link) []*tracepb.Span_Link {
	if len(ls) == 0 {
		return nil
	}
	out := make([]*tracepb.Span_Link, len(ls))
	for i, l := range ls {
		tid := l.SpanContext.TraceID()
		sid := l.SpanContext.SpanID()
		out[i] = &tracepb.Span_Link{
			TraceId:                tid[:],
			SpanId:                 sid[:],
			Attributes:             keyValues(l.Attributes),
			DroppedAttributesCount: clampUint32(l.DroppedAttributeCount),
			Flags:                  spanFlags(l.SpanContext.TraceFlags(), l.SpanContext),
		}
	}
	return out
}

func status(code codes.Code, description string) *tracepb.Status {
	description = validUTF8(description)
	switch code {
	case codes.Ok:
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK, Message: description}
	case codes.Error:
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: description}
	default:
		return &tracepb.Status{Code: tracepb.Status_STATUS_CODE_UNSET, Message: description}
	}
}

func spanKind(kind oteltrace.SpanKind) tracepb.Span_SpanKind {
	switch kind {
	case oteltrace.SpanKindInternal:
		return tracepb.Span_SPAN_KIND_INTERNAL
	case oteltrace.SpanKindClient:
		return tracepb.Span_SPAN_KIND_CLIENT
	case oteltrace.SpanKindServer:
		return tracepb.Span_SPAN_KIND_SERVER
	case oteltrace.SpanKindProducer:
		return tracepb.Span_SPAN_KIND_PRODUCER
	case oteltrace.SpanKindConsumer:
		return tracepb.Span_SPAN_KIND_CONSUMER
	default:
		return tracepb.Span_SPAN_KIND_UNSPECIFIED
	}
}

func spanFlags(flags oteltrace.TraceFlags, parent oteltrace.SpanContext) uint32 {
	out := uint32(flags) | uint32(tracepb.SpanFlags_SPAN_FLAGS_CONTEXT_HAS_IS_REMOTE_MASK)
	if parent.IsRemote() {
		out |= uint32(tracepb.SpanFlags_SPAN_FLAGS_CONTEXT_IS_REMOTE_MASK)
	}
	return out
}

func clampUint32(v int) uint32 {
	if v < 0 {
		return 0
	}
	if int64(v) > math.MaxUint32 {
		return math.MaxUint32
	}
	return uint32(v)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
