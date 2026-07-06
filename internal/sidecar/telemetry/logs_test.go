package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TestWriteLogsSanitizesInvalidUTF8(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LogFileName)
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01},
		SpanID:  trace.SpanID{0x02},
	})

	err := WriteLogs(path,
		[]attribute.KeyValue{attribute.String("resource\xffkey", "resource\xfevalue")},
		"scope\xffname",
		"v\xfe1",
		[]LogRecord{{
			Time:      time.Unix(1, 0),
			Span:      sc,
			EventName: "event\xffname",
			Attributes: []attribute.KeyValue{
				attribute.String("attr\xffkey", "attr\xfevalue"),
			},
		}},
	)
	if err != nil {
		t.Fatalf("WriteLogs with invalid UTF-8 must not fail: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(raw) {
		t.Fatal("logs.jsonl contains invalid UTF-8 after sanitizing")
	}
	content := string(raw)
	for _, want := range []string{
		"resource�key",
		"resource�value",
		"scope�name",
		"v�1",
		"event�name",
		"attr�key",
		"attr�value",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("logs.jsonl missing sanitized value %q:\n%s", want, content)
		}
	}
}
