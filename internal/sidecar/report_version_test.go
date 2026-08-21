package sidecar

import (
	"strings"
	"testing"
)

// The runtime must stamp the contract version on EVERY report it returns —
// resolved, BLOCKED wrap-up, and warn-fallback alike — otherwise consumers
// read empty as "pre-versioning" and apply wrong semantics.
func TestReportVersionStampsCoverAllReportPaths(t *testing.T) {
	blocked := &Report{
		Answer: "BLOCKED: the exploration hit the adapter's max-turns safety net.",
	}
	warn := &Report{Answer: warnNoReportAnswer("")}
	for i, r := range []*Report{blocked, warn} {
		r.SchemaVersion = ReportSchemaVersion
		if r.SchemaVersion != ReportSchemaVersion {
			t.Fatalf("report %d not stamped", i)
		}
		if v := CheckReportBounds(r); v.OversizeChars > 0 {
			t.Fatalf("report %d unexpectedly oversize: %v", i, v)
		}
	}
}

// The strict submission path must accept schemaVersion (it is now a known
// field of Report) and the derived submit_report schema advertises it.
func TestStrictPathAcceptsSchemaVersion(t *testing.T) {
	data := []byte(`{"answer":"ok","schemaVersion":"1","verified":[{"summary":"s","evidence":"e"}],"relevantFiles":[{"path":"a.go"}],"commandsRun":["ghx x"]}`)
	r, err := DecodeReportStrict(data)
	if err != nil {
		t.Fatalf("strict path rejected schemaVersion: %v", err)
	}
	if r.SchemaVersion != ReportSchemaVersion {
		t.Fatalf("SchemaVersion = %q, want %q", r.SchemaVersion, ReportSchemaVersion)
	}
	schema := string(reportInputSchema())
	if !strings.Contains(schema, "schemaVersion") {
		t.Fatal("derived submit_report schema does not advertise schemaVersion")
	}
}
