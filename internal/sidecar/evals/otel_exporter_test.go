package evals

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"testing"

	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

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

	path := filepath.Join(runDir, traceFileName)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var lines int
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines++
		var td tracepb.TracesData
		if err := protojson.Unmarshal(scanner.Bytes(), &td); err != nil {
			t.Fatalf("line %d is not OTLP TracesData JSON: %v", lines, err)
		}
		for _, rs := range td.ResourceSpans {
			for _, ss := range rs.ScopeSpans {
				for _, sp := range ss.Spans {
					seen[sp.Name] = true
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if lines < 2 {
		t.Fatalf("lines = %d, want at least two append batches", lines)
	}
	if !seen["first"] || !seen["second"] {
		t.Fatalf("missing exported spans: %v", seen)
	}
}
