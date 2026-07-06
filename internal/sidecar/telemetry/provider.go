package telemetry

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTracerProvider builds a tracer provider whose spans append to
// <dir>/traces.jsonl as spec-exact OTLP JSON File records. When
// OTEL_EXPORTER_OTLP_ENDPOINT is set it additionally ships spans to that
// collector over HTTP. The resource attributes are caller-supplied so each
// stack (evals, production sidecar) labels its resource in its own namespace
// while sharing this transport. AlwaysSample + SimpleSpanProcessor keep every
// span, one batch per export, which is what makes the JSONL deterministic.
func NewTracerProvider(ctx context.Context, dir string, resourceAttrs []attribute.KeyValue) (*sdktrace.TracerProvider, error) {
	res := resource.NewSchemaless(resourceAttrs...)
	exporters := []sdktrace.SpanExporter{newOTLPJSONFileExporter(dir)}
	if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != "" {
		httpExporter, err := otlptracehttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("create OTLP HTTP trace exporter: %w", err)
		}
		exporters = append(exporters, httpExporter)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	for _, exporter := range exporters {
		opts = append(opts, sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}
