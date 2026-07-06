package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var (
	hexTraceIDRE = regexp.MustCompile(`^[0-9a-f]{32}$`)
	hexSpanIDRE  = regexp.MustCompile(`^[0-9a-f]{16}$`)
)

// exportedSpan is the subset of the OTLP/JSON span shape the tests assert
// on. IDs are asserted as spec hex strings, so the file is parsed as plain
// JSON rather than round-tripped through protojson (whose proto3 mapping
// the OTLP spec deviates from for ID fields).
type exportedSpan struct {
	TraceID      string `json:"traceId"`
	SpanID       string `json:"spanId"`
	ParentSpanID string `json:"parentSpanId"`
	Name         string `json:"name"`
	Events       []struct {
		Name       string              `json:"name"`
		Attributes []exportedAttribute `json:"attributes"`
	} `json:"events"`
}

type exportedAttribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue"`
		BoolValue   bool   `json:"boolValue"`
		IntValue    string `json:"intValue"`
	} `json:"value"`
}

func readExportedSpans(t *testing.T, runDir string) (lines int, spans []exportedSpan) {
	t.Helper()
	f, err := os.Open(filepath.Join(runDir, traceFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines++
		var td struct {
			ResourceSpans []struct {
				ScopeSpans []struct {
					Spans []exportedSpan `json:"spans"`
				} `json:"scopeSpans"`
			} `json:"resourceSpans"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &td); err != nil {
			t.Fatalf("line %d is not OTLP TracesData JSON: %v", lines, err)
		}
		for _, rs := range td.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				spans = append(spans, ss.Spans...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return lines, spans
}

func TestOTLPJSONFileExporterAppendsValidBatches(t *testing.T) {
	runDir := t.TempDir()
	exporter := newOTLPJSONFileExporter(runDir)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(sdkresource.NewSchemaless()),
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	tracer := tp.Tracer("test")

	_, first := tracer.Start(context.Background(), "first", trace.WithSpanKind(trace.SpanKindInternal))
	first.End()
	_, second := tracer.Start(context.Background(), "second", trace.WithSpanKind(trace.SpanKindInternal))
	second.End()
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	lines, spans := readExportedSpans(t, runDir)
	if lines < 2 {
		t.Fatalf("lines = %d, want at least two append batches", lines)
	}
	seen := map[string]bool{}
	for _, sp := range spans {
		seen[sp.Name] = true
	}
	if !seen["first"] || !seen["second"] {
		t.Fatalf("missing exported spans: %v", seen)
	}
}

// TestOTLPJSONFileExporterEmitsSpecHexIDs pins the OTLP/JSON spec deviation:
// trace_id/span_id/parent_span_id are lowercase hex, not protojson base64.
// Regression test for the gate-run-2026-07 finding where every span was
// rejected by spec-compliant OTLP receivers (ADR-0016.7 follow-up).
func TestOTLPJSONFileExporterEmitsSpecHexIDs(t *testing.T) {
	runDir := t.TempDir()
	exporter := newOTLPJSONFileExporter(runDir)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(sdkresource.NewSchemaless()),
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	tracer := tp.Tracer("test")

	ctx, parent := tracer.Start(context.Background(), "parent", trace.WithSpanKind(trace.SpanKindInternal))
	_, child := tracer.Start(ctx, "child", trace.WithSpanKind(trace.SpanKindInternal))
	child.End()
	parent.End()
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	_, spans := readExportedSpans(t, runDir)
	byName := map[string]exportedSpan{}
	for _, sp := range spans {
		byName[sp.Name] = sp
	}
	p, okP := byName["parent"]
	c, okC := byName["child"]
	if !okP || !okC {
		t.Fatalf("missing spans, got %v", byName)
	}
	for name, sp := range byName {
		if !hexTraceIDRE.MatchString(sp.TraceID) {
			t.Errorf("%s traceId = %q, want 32-char lowercase hex", name, sp.TraceID)
		}
		if !hexSpanIDRE.MatchString(sp.SpanID) {
			t.Errorf("%s spanId = %q, want 16-char lowercase hex", name, sp.SpanID)
		}
	}
	if !hexSpanIDRE.MatchString(c.ParentSpanID) {
		t.Errorf("child parentSpanId = %q, want 16-char lowercase hex", c.ParentSpanID)
	}
	if c.ParentSpanID != p.SpanID {
		t.Errorf("child parentSpanId = %q, want parent spanId %q", c.ParentSpanID, p.SpanID)
	}
	if c.TraceID != p.TraceID {
		t.Errorf("child traceId = %q, want parent traceId %q", c.TraceID, p.TraceID)
	}
}
