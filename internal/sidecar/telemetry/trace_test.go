package telemetry

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// TestOTLPJSONFileExporterSanitizesInvalidUTF8 pins the fix for the
// spot-instrumented-2026-07-06 harness finding: a tool span whose output
// attribute carried invalid UTF-8 bytes made protojson fail the whole export
// batch ("AnyValue.string_value contains invalid UTF-8"), silently dropping
// the span from traces.jsonl (1 of 33 tool spans lost). Invalid bytes must
// be replaced with U+FFFD before marshal — a span is never dropped.
func TestOTLPJSONFileExporterSanitizesInvalidUTF8(t *testing.T) {
	runDir := t.TempDir()
	exporter := newOTLPJSONFileExporter(runDir)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(sdkresource.NewSchemaless()),
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
	)
	tracer := tp.Tracer("test")

	// Invalid UTF-8 shaped like real binary-contaminated tool output, plus
	// invalid bytes in every other proto string field on the span path.
	invalidToolOutput := "ghx read output: \xff\xfe binary tail \x80"
	_, sp := tracer.Start(context.Background(), "tool.execute \xf0\x28",
		trace.WithSpanKind(trace.SpanKindInternal))
	sp.SetAttributes(
		attribute.String("ghx.tool.output", invalidToolOutput),
		attribute.String("bad\xffkey", "clean value"),
		attribute.StringSlice("ghx.tool.chunks", []string{"ok", "bad\xc3(chunk"}),
	)
	sp.AddEvent("chunk \xff", trace.WithAttributes(attribute.String("payload", "ev\xffdata")))
	sp.SetStatus(codes.Error, "failed on \xff bytes")
	sp.End()
	// A clean sibling span in the same batch must survive untouched.
	_, clean := tracer.Start(context.Background(), "clean", trace.WithSpanKind(trace.SpanKindInternal))
	clean.End()
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("export with invalid UTF-8 must not fail: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(runDir, traceFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(raw) {
		t.Fatal("traces.jsonl contains invalid UTF-8 after sanitizing")
	}
	lines, spans := readExportedSpans(t, runDir)
	if lines == 0 {
		t.Fatal("no export batches written")
	}
	if len(spans) != 2 {
		t.Fatalf("exported %d spans, want 2 — a span was dropped", len(spans))
	}
	var tool *exportedSpan
	seenClean := false
	for i := range spans {
		switch {
		case strings.HasPrefix(spans[i].Name, "tool.execute"):
			tool = &spans[i]
		case spans[i].Name == "clean":
			seenClean = true
		}
	}
	if tool == nil || !seenClean {
		t.Fatalf("missing spans, got %+v", spans)
	}
	if !strings.Contains(tool.Name, "�") {
		t.Errorf("span name = %q, want invalid bytes replaced with U+FFFD", tool.Name)
	}
	content := string(raw)
	// strings.ToValidUTF8 replaces each RUN of invalid bytes with one
	// replacement rune, so "\xff\xfe" collapses to a single U+FFFD.
	if !strings.Contains(content, "ghx read output: � binary tail �") {
		t.Error("tool output attribute value not sanitized with replacement runes")
	}
	if !strings.Contains(content, "bad�key") {
		t.Error("attribute key not sanitized")
	}
	if !strings.Contains(content, "bad�(chunk") {
		t.Error("string-slice attribute value not sanitized")
	}
	if !strings.Contains(content, "chunk �") {
		t.Error("event name not sanitized")
	}
	if !strings.Contains(content, "ev�data") {
		t.Error("event attribute value not sanitized")
	}
	if !strings.Contains(content, "failed on � bytes") {
		t.Error("status message not sanitized")
	}
	if !strings.Contains(content, "clean value") {
		t.Error("clean sibling data missing from export")
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
