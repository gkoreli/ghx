package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"go.opentelemetry.io/otel/attribute"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func TestWriteMetricsSanitizesInvalidUTF8(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, MetricFileName)
	start := time.Unix(1, 0)
	end := time.Unix(2, 0)
	metric := HistogramMetric(
		"metric\xffname",
		"description\xfevalue",
		"By\x80",
		[]*metricspb.HistogramDataPoint{
			HistogramPoint(start, end, 1, []attribute.KeyValue{
				attribute.String("point\xffkey", "point\xfevalue"),
			}),
		},
	)

	err := WriteMetrics(path,
		[]attribute.KeyValue{attribute.String("resource\xffkey", "resource\xfevalue")},
		"scope\xffname",
		"v\xfe1",
		[]*metricspb.Metric{metric},
	)
	if err != nil {
		t.Fatalf("WriteMetrics with invalid UTF-8 must not fail: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.Valid(raw) {
		t.Fatal("metrics.jsonl contains invalid UTF-8 after sanitizing")
	}
	content := string(raw)
	for _, want := range []string{
		"resource�key",
		"resource�value",
		"scope�name",
		"v�1",
		"metric�name",
		"description�value",
		"By�",
		"point�key",
		"point�value",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("metrics.jsonl missing sanitized value %q:\n%s", want, content)
		}
	}
}
