package telemetry

import (
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

// WriteMetrics appends one OTLP MetricsData JSON object (spec-exact) to path
// carrying the supplied metrics under a single resource/scope. Callers build
// the metric list with the exported builders below (HistogramMetric, SumMetric,
// …) so both stacks share the exact serialization. No-op when metrics is empty.
func WriteMetrics(path string, resourceAttrs []attribute.KeyValue, scopeName, scopeVersion string, metrics []*metricspb.Metric) error {
	if len(metrics) == 0 {
		return nil
	}
	data := &metricspb.MetricsData{
		ResourceMetrics: []*metricspb.ResourceMetrics{
			{
				Resource:     protoResource(sdkresource.NewSchemaless(resourceAttrs...)),
				ScopeMetrics: []*metricspb.ScopeMetrics{{Scope: scope(scopeName, scopeVersion), Metrics: metrics}},
			},
		},
	}
	sanitizeProtoStrings(data)
	line, err := protojson.MarshalOptions{EmitUnpopulated: false}.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal OTLP metrics json: %w", err)
	}
	line = append(line, '\n')

	// Shared append; the per-path mutex in AppendJSONLine serializes concurrent
	// writers under episode-level parallelism (ADR-0025 D3, jsonl_writer.go).
	return AppendJSONLine(path, line)
}

// HistogramMetric wraps delta-temporality single-point histogram data points
// into a named OTLP metric.
func HistogramMetric(name, description, unit string, points []*metricspb.HistogramDataPoint) *metricspb.Metric {
	return &metricspb.Metric{
		Name:        validUTF8(name),
		Description: validUTF8(description),
		Unit:        validUTF8(unit),
		Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{
			DataPoints:             points,
			AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
		}},
	}
}

// HistogramPoint builds a single-observation histogram data point (count=1,
// sum=min=max=value) — the shape used for per-turn durations and report sizes.
func HistogramPoint(start, end time.Time, value float64, attrs []attribute.KeyValue) *metricspb.HistogramDataPoint {
	return &metricspb.HistogramDataPoint{
		Attributes:        keyValues(attrs),
		StartTimeUnixNano: UnixNano(start),
		TimeUnixNano:      UnixNano(end),
		Count:             1,
		Sum:               &value,
		Min:               &value,
		Max:               &value,
	}
}

// SumMetric wraps delta-temporality number data points into a named OTLP sum.
func SumMetric(name, description, unit string, points []*metricspb.NumberDataPoint, monotonic bool) *metricspb.Metric {
	return &metricspb.Metric{
		Name:        validUTF8(name),
		Description: validUTF8(description),
		Unit:        validUTF8(unit),
		Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
			DataPoints:             points,
			AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
			IsMonotonic:            monotonic,
		}},
	}
}

// IntPoint builds an integer sum data point (e.g. token counts).
func IntPoint(start, end time.Time, value int64, attrs []attribute.KeyValue) *metricspb.NumberDataPoint {
	return &metricspb.NumberDataPoint{
		Attributes:        keyValues(attrs),
		StartTimeUnixNano: UnixNano(start),
		TimeUnixNano:      UnixNano(end),
		Value:             &metricspb.NumberDataPoint_AsInt{AsInt: value},
	}
}

// AppendAttrs returns base with extra appended, without mutating base.
func AppendAttrs(base []attribute.KeyValue, extra ...attribute.KeyValue) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

// UnixNano returns t as a non-negative unix-nano timestamp.
func UnixNano(t time.Time) uint64 {
	return uint64(maxInt64(0, t.UnixNano()))
}
