package evals

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
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

type exportedLogRecord struct {
	TraceID    string `json:"traceId"`
	SpanID     string `json:"spanId"`
	EventName  string `json:"eventName"`
	Attributes []struct {
		Key   string `json:"key"`
		Value struct {
			StringValue string `json:"stringValue"`
		} `json:"value"`
	} `json:"attributes"`
}

type exportedMetric struct {
	Name      string `json:"name"`
	Histogram *struct {
		DataPoints []struct {
			Attributes []exportedAttribute `json:"attributes"`
			Count      string              `json:"count"`
		} `json:"dataPoints"`
	} `json:"histogram"`
	Sum *struct {
		DataPoints []struct {
			Attributes []exportedAttribute `json:"attributes"`
			AsInt      string              `json:"asInt"`
		} `json:"dataPoints"`
	} `json:"sum"`
}

type exportedAttribute struct {
	Key   string `json:"key"`
	Value struct {
		StringValue string `json:"stringValue"`
		BoolValue   bool   `json:"boolValue"`
		IntValue    string `json:"intValue"`
	} `json:"value"`
}

func readExportedMetrics(t *testing.T, runDir string) (lines int, metrics []exportedMetric) {
	t.Helper()
	f, err := os.Open(filepath.Join(runDir, metricFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines++
		var md struct {
			ResourceMetrics []struct {
				ScopeMetrics []struct {
					Metrics []exportedMetric `json:"metrics"`
				} `json:"scopeMetrics"`
			} `json:"resourceMetrics"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &md); err != nil {
			t.Fatalf("line %d is not OTLP MetricsData JSON: %v", lines, err)
		}
		for _, rm := range md.ResourceMetrics {
			for _, sm := range rm.ScopeMetrics {
				metrics = append(metrics, sm.Metrics...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return lines, metrics
}

func readExportedLogs(t *testing.T, runDir string) (lines int, logs []exportedLogRecord) {
	t.Helper()
	f, err := os.Open(filepath.Join(runDir, logFileName))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines++
		var ld struct {
			ResourceLogs []struct {
				ScopeLogs []struct {
					LogRecords []exportedLogRecord `json:"logRecords"`
				} `json:"scopeLogs"`
			} `json:"resourceLogs"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &ld); err != nil {
			t.Fatalf("line %d is not OTLP LogsData JSON: %v", lines, err)
		}
		for _, rl := range ld.ResourceLogs {
			for _, sl := range rl.ScopeLogs {
				logs = append(logs, sl.LogRecords...)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return lines, logs
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

func TestOTLPJSONLogsExporterEmitsSpecHexIDsAndGenAIContent(t *testing.T) {
	t.Setenv(genAICaptureMessageContentEnv, "true")
	runDir := t.TempDir()
	ep := &Episode{
		ID:      "ep-logs",
		TaskID:  "task-logs",
		Repo:    "owner/repo",
		Profile: ProfileGhx,
		Turns: []TurnRecord{{
			Turn:     0,
			Question: "Where is routing configured?",
			Text:     "Routing lives in router.go.",
			Thinking: "Need to inspect route files.",
		}},
		Identity:  AgentIdentity{SubjectModel: "test-model"},
		StartedAt: timeNowUTC(),
	}
	ep.EndedAt = ep.StartedAt.Add(10)

	if err := EmitEpisodeTraces(context.Background(), runDir, ep); err != nil {
		t.Fatal(err)
	}

	lines, logs := readExportedLogs(t, runDir)
	if lines != 1 {
		t.Fatalf("log lines = %d, want 1", lines)
	}
	if len(logs) != 1 {
		t.Fatalf("log records = %d, want 1", len(logs))
	}
	log := logs[0]
	if log.EventName != genAIInferenceOperationDetailsEvent {
		t.Fatalf("eventName = %q, want %q", log.EventName, genAIInferenceOperationDetailsEvent)
	}
	if !hexTraceIDRE.MatchString(log.TraceID) {
		t.Errorf("log traceId = %q, want 32-char lowercase hex", log.TraceID)
	}
	if !hexSpanIDRE.MatchString(log.SpanID) {
		t.Errorf("log spanId = %q, want 16-char lowercase hex", log.SpanID)
	}
	attrs := map[string]string{}
	for _, attr := range log.Attributes {
		attrs[attr.Key] = attr.Value.StringValue
	}
	if attrs[genAIInputMessagesAttribute] == "" {
		t.Fatalf("missing %s attribute", genAIInputMessagesAttribute)
	}
	if attrs[genAIOutputMessagesAttribute] == "" {
		t.Fatalf("missing %s attribute", genAIOutputMessagesAttribute)
	}
	if !strings.Contains(attrs[genAIOutputMessagesAttribute], `"type":"reasoning"`) {
		t.Fatalf("output messages do not include reasoning part: %s", attrs[genAIOutputMessagesAttribute])
	}
}

func timeNowUTC() time.Time {
	return time.Now().UTC()
}

func TestOTLPJSONMetricsExporterEmitsGenAIAndGhxMetrics(t *testing.T) {
	runDir := t.TempDir()
	ep := &Episode{
		ID:      "ep-metrics",
		TaskID:  "task-metrics",
		Repo:    "owner/repo",
		Profile: ProfileSidecar,
		Turns: []TurnRecord{{
			Turn:       0,
			Question:   "Where is routing configured?",
			Text:       "Routing lives in router.go.",
			Thinking:   "Need to inspect route files.",
			DurationMs: 1250,
		}},
		Identity:  AgentIdentity{SubjectModel: "test-model"},
		Rewards:   RewardBreakdown{Correctness: 1, Evidence: 0.5, Trajectory: 1, Compression: 0.2, Safety: 1, Overall: 0.74},
		Anomalies: []Anomaly{{Kind: AnomalySidecarReportRetried, Severity: SeveritySoft}},
		StartedAt: timeNowUTC(),
	}
	ep.EndedAt = ep.StartedAt.Add(2 * time.Second)

	if err := EmitEpisodeMetrics(runDir, ep); err != nil {
		t.Fatal(err)
	}

	lines, metrics := readExportedMetrics(t, runDir)
	if lines != 1 {
		t.Fatalf("metric lines = %d, want 1", lines)
	}
	byName := map[string]exportedMetric{}
	for _, metric := range metrics {
		byName[metric.Name] = metric
	}
	for _, name := range []string{
		genAIClientOperationDurationMetric,
		genAIClientTokenUsageMetric,
		ghxEvalRewardMetric,
		ghxEvalAnomalyCount,
	} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("missing metric %s; got %v", name, metricNames(metrics))
		}
	}
	tokenMetric := byName[genAIClientTokenUsageMetric]
	seenTokenTypes := map[string]bool{}
	for _, point := range tokenMetric.Sum.DataPoints {
		for _, attr := range point.Attributes {
			if attr.Key == genAITokenTypeAttribute {
				seenTokenTypes[attr.Value.StringValue] = true
			}
		}
	}
	for _, tokenType := range []string{"input", "output", "reasoning"} {
		if !seenTokenTypes[tokenType] {
			t.Fatalf("missing token type %q in %s", tokenType, genAIClientTokenUsageMetric)
		}
	}
}

func metricNames(metrics []exportedMetric) []string {
	names := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		names = append(names, metric.Name)
	}
	return names
}

func TestOTLPTraceRewardSpanIncludesEvaluationAndCheckEvents(t *testing.T) {
	runDir := t.TempDir()
	ep := &Episode{
		ID:      "ep-rewards",
		TaskID:  "task-rewards",
		Repo:    "owner/repo",
		Profile: ProfileGhx,
		Checks: TaskChecks{
			ExpectedFiles:  []string{"src/router.go"},
			RequiredClaims: []string{"routes are configured"},
		},
		Turns: []TurnRecord{{
			Turn:     0,
			Question: "Where are routes configured?",
			Text:     "Routes are configured in src/router.go.",
		}},
		Identity:  AgentIdentity{SubjectModel: "test-model"},
		Rewards:   RewardBreakdown{Correctness: 1, Evidence: 0.5, Trajectory: 0.75, Compression: 0, Safety: 1, Overall: 0.65},
		StartedAt: timeNowUTC(),
	}
	ep.EndedAt = ep.StartedAt.Add(2 * time.Second)

	if err := EmitEpisodeTraces(context.Background(), runDir, ep); err != nil {
		t.Fatal(err)
	}

	_, spans := readExportedSpans(t, runDir)
	var rewardSpan *exportedSpan
	for i := range spans {
		if spans[i].Name == "eval.reward.compute" {
			rewardSpan = &spans[i]
			break
		}
	}
	if rewardSpan == nil {
		t.Fatal("missing eval.reward.compute span")
	}
	if !spanHasEventWithStringAttr(*rewardSpan, genAIEvaluationResultEvent, genAIEvaluationNameAttribute, "ghx.eval.reward.correctness") {
		t.Fatalf("missing %s event for correctness: %+v", genAIEvaluationResultEvent, rewardSpan.Events)
	}
	if !spanHasEventWithStringAttr(*rewardSpan, ghxRewardCheckEvent, "ghx.eval.check.expected", "src/router.go") {
		t.Fatalf("missing expected-file check event: %+v", rewardSpan.Events)
	}
}

func spanHasEventWithStringAttr(span exportedSpan, eventName, key, value string) bool {
	for _, event := range span.Events {
		if event.Name != eventName {
			continue
		}
		for _, attr := range event.Attributes {
			if attr.Key == key && attr.Value.StringValue == value {
				return true
			}
		}
	}
	return false
}
