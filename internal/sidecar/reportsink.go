package sidecar

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Report submission contract (ADR-0021 D1). The sidecar registers a
// session-scoped MCP stdio server exposing exactly one tool, submit_report,
// whose input schema IS the Report schema. Validation is derived from the
// Report Go type (DecodeReportStrict), so the schema is never hand-duplicated.
const (
	// ReportSinkServerName is the ACP McpServer name for the report sink. It
	// becomes the middle segment of the SDK tool identifier
	// (mcp__<server>__<tool>), so it must stay in sync with SubmitReportToolID.
	ReportSinkServerName = "ghx-report-sink"
	// SubmitReportToolName is the single tool the report sink exposes.
	SubmitReportToolName = "submit_report"
)

// SubmitReportToolID is the fully-qualified SDK/adapter tool name the model
// sees for the submit_report MCP tool: mcp__<server>__<tool>. It is the value
// that must appear in the session allowedTools list so the adapter auto-approves
// the call instead of routing it through the read-only permission gate (which
// classifies MCP tools as kind "other" and would reject them). See
// session_options.go and acp.go for the wiring and the adapter-source citation.
const SubmitReportToolID = "mcp__" + ReportSinkServerName + "__" + SubmitReportToolName

// reportAcceptedMessage is returned to the model on a valid submission. It is
// the completion signal: the turn is done once the tool returns this.
const reportAcceptedMessage = "report accepted — end your turn now; do not repeat the answer in text"

// DecodeReportStrict validates and decodes a submit_report payload with NO
// coercion (ADR-0021 D1). It is the single validation authority, derived from
// the Report type:
//
//   - json.Decoder with DisallowUnknownFields rejects any field not in Report,
//     recursively (extra top-level keys and extra keys inside verified[i],
//     relevantFiles[i], evidence[i], …).
//   - the typed decode rejects shape drift (a bare string where a Claim object
//     is required, a scalar where an array is required, etc.) with a precise
//     Go type error the model can act on.
//   - a trailing-token check rejects concatenated / duplicated JSON documents.
//   - ValidateReport enforces the semantic minimum: a non-empty answer.
//
// The returned error is the exact validation failure, suitable for feeding
// straight back to the producer as tool output.
//
// After validation passes, the lenient ADR-0031.2 nextReads normalizer runs
// (Report.NormalizeNextReads): it cleans path-shaped nextReads entries and
// keeps prose entries verbatim, and it can never fail a report.
func DecodeReportStrict(data []byte) (*Report, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, errors.New("no report payload provided; call submit_report with the report object as arguments")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var r Report
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("report failed validation: %w", err)
	}
	if dec.More() {
		return nil, errors.New("report failed validation: trailing data after the report object (submit exactly one JSON object)")
	}
	if err := ValidateReport(&r); err != nil {
		return nil, err
	}
	if err := ValidateReportEvidence(&r); err != nil {
		return nil, err
	}
	// Lenient nextReads normalization (ADR-0031.2): steers stored entries
	// toward the concrete-path contract without ever rejecting a report. This
	// is deliberately NOT coercion in the ADR-0021 D1 sense — the shape the
	// model submitted already passed strict validation above.
	r.NormalizeNextReads()
	return &r, nil
}

// blockedAnswerPrefix marks the escape-hatch report for investigations that
// could not proceed at all. BLOCKED reports must state why (after the prefix)
// and are exempt from the evidence requirement — a blocked investigation has
// no evidence by definition.
const blockedAnswerPrefix = "BLOCKED"

const maxReportAnswerChars = 700

var markdownHeadingRE = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+`)

// ValidateReportEvidence enforces the evidence contract on the strict
// submit_report path (ADR-0027 D4): a non-BLOCKED report is accepted only when
// it carries ALL of — at least one verified claim with non-empty evidence, at
// least one relevant file, and at least one command run. An answer without
// evidence is a hypothesis, not a finding (AGENTS.md "Evidence Contract").
// The returned error names every missing field exactly so the producer can fix
// its report in the same in-band loop ADR-0021 runs for shape errors.
func ValidateReportEvidence(r *Report) error {
	if r == nil {
		return errors.New("report is nil")
	}
	answer := strings.TrimSpace(r.Answer)
	if strings.HasPrefix(answer, blockedAnswerPrefix) {
		why := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(answer, blockedAnswerPrefix), ":"))
		if why == "" {
			return errors.New(`report failed validation: answer: a BLOCKED report must state why it is blocked ("BLOCKED: <reason>")`)
		}
		return nil
	}
	var missing []string
	if markdownHeadingRE.MatchString(answer) {
		missing = append(missing, `answer: Markdown headings are not allowed; put the direct answer first without headings`)
	}
	if len(answer) > maxReportAnswerChars {
		missing = append(missing, fmt.Sprintf(`answer: compact answer required; got %d characters, limit is %d`, len(answer), maxReportAnswerChars))
	}
	if !hasVerifiedClaimWithEvidence(r.Verified) {
		missing = append(missing, `verified: at least one verified claim with a non-empty "evidence" field is required`)
	}
	if !hasRelevantFile(r.RelevantFiles) {
		missing = append(missing, `relevantFiles: at least one relevant file with a non-empty "path" is required`)
	}
	if !hasCommandRun(r.CommandsRun) {
		missing = append(missing, `commandsRun: at least one command you actually ran is required`)
	}
	for _, bad := range scratchEvidenceSources(r) {
		missing = append(missing, bad)
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("report failed validation: evidence requirements not met — %s. Cite the evidence you gathered, or submit a BLOCKED report (answer starting %q) stating why you could not investigate",
		strings.Join(missing, "; "), blockedAnswerPrefix+": <reason>")
}

func hasVerifiedClaimWithEvidence(claims []Claim) bool {
	for _, c := range claims {
		if strings.TrimSpace(c.Summary) != "" && strings.TrimSpace(c.Evidence) != "" {
			return true
		}
	}
	return false
}

func hasRelevantFile(files []RelevantFile) bool {
	for _, f := range files {
		if strings.TrimSpace(f.Path) != "" {
			return true
		}
	}
	return false
}

func hasCommandRun(commands []string) bool {
	for _, c := range commands {
		if strings.TrimSpace(c) != "" {
			return true
		}
	}
	return false
}

func scratchEvidenceSources(r *Report) []string {
	var bad []string
	check := func(field, value string) {
		if citesScratchSource(value) {
			bad = append(bad, fmt.Sprintf(`%s: evidence must cite ghx-auditable repo paths/commands, not local scratch files: %q`, field, value))
		}
	}
	for i, c := range r.Verified {
		check(fmt.Sprintf("verified[%d].evidence", i), c.Evidence)
	}
	for i, c := range r.Inferred {
		check(fmt.Sprintf("inferred[%d].evidence", i), c.Evidence)
	}
	for i, c := range r.Unverified {
		check(fmt.Sprintf("unverified[%d].evidence", i), c.Evidence)
	}
	for i, e := range r.Evidence {
		check(fmt.Sprintf("evidence[%d].source", i), e.Source)
	}
	return bad
}

func citesScratchSource(value string) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	lower := strings.ToLower(v)
	return strings.HasPrefix(lower, "/tmp/") ||
		strings.HasPrefix(lower, "tmp/") ||
		strings.HasPrefix(lower, "file:///tmp/") ||
		strings.HasPrefix(lower, "/var/folders/") ||
		strings.Contains(lower, " /tmp/") ||
		strings.Contains(lower, " file:///tmp/") ||
		strings.Contains(lower, " /var/folders/")
}

// reportTiers are the canonical tierUsed values (ADR-0024.1 "Visibility
// Contract"): the highest escalation tier that produced the answer.
var reportTiers = map[string]bool{"tier0": true, "tier1": true, "tier2": true, "tier3": true}

// ValidateReport enforces the semantic minimum shared by the strict submission
// path and the lenient fallback path: a report must carry a non-empty answer,
// and tierUsed — when present — must be a canonical tier ID (ADR-0024.1).
// Shape validation is handled by the typed (de)serialization of the Report
// type itself, keeping these the only hand-written semantic rules.
func ValidateReport(r *Report) error {
	if r == nil {
		return errors.New("report is nil")
	}
	if r.Answer == "" {
		return ErrEmptyAnswer
	}
	if r.TierUsed != "" && !reportTiers[r.TierUsed] {
		return fmt.Errorf(`report failed validation: tierUsed: %q is not a canonical tier ("tier0" | "tier1" | "tier2" | "tier3")`, r.TierUsed)
	}
	return nil
}

// reportInputSchema derives the submit_report JSON Schema from the Report Go
// type via reflection, so the advertised schema cannot drift from the type the
// validator decodes into. Only the answer field is required; unknown properties
// are rejected (mirroring DisallowUnknownFields).
func reportInputSchema() json.RawMessage {
	schema := jsonSchemaForType(reflect.TypeOf(Report{}))
	schema["required"] = []string{"answer"}
	schema["description"] = "The ghx-sidecar evidence report. Submit the complete report object. A non-BLOCKED report must include at least one verified claim with evidence, at least one relevantFiles entry, and at least one commandsRun entry (ADR-0027 D4). No unknown fields."
	if props, ok := schema["properties"].(map[string]any); ok {
		applyFieldDescriptions(props, reportFieldDescriptions)
	}
	b, err := json.Marshal(schema)
	if err != nil {
		// Report is a fixed, reflectable struct; marshaling its derived schema
		// cannot fail in practice. Fall back to a permissive object schema.
		return json.RawMessage(`{"type":"object"}`)
	}
	return b
}

// reportFieldDescriptions annotates the top-level Report fields with model-facing
// hints. These are documentation only; validation lives in DecodeReportStrict.
var reportFieldDescriptions = map[string]string{
	"answer":        "Direct answer to the question, at most 2 sentences and no Markdown headings.",
	"verified":      "Claims backed by evidence you gathered: [{summary, evidence}].",
	"inferred":      "Claims you inferred but did not directly verify: [{summary, evidence}].",
	"unverified":    "Open claims you could not confirm: [{summary, evidence}].",
	"relevantFiles": "1-5 most relevant files: [{path, reason}].",
	"evidence":      "Evidence entries: [{source, summary}] — source is the ghx command.",
	"tierUsed":      "Highest escalation tier used: \"tier0\" | \"tier1\" | \"tier2\" | \"tier3\". Omit when tier tracking is unavailable.",
	"backendsUsed":  "Evidence backends used — canonical IDs: \"remote\", \"local:codemap\", \"local:ast-grep\", \"local:repomap\".",
	"commandsRun":   "ghx commands you ran.",
	"uncertainty":   "What remains uncertain.",
	"nextReads":     "Files the NEXT turn will most likely need read: concrete repo-relative paths (\"path/to/file.go\" or \"owner/repo:path/to/file.go\"), one path per entry, no prose or line ranges. Empty when nothing is anticipated.",
}

func applyFieldDescriptions(props map[string]any, desc map[string]string) {
	for name, d := range desc {
		if raw, ok := props[name].(map[string]any); ok {
			raw["description"] = d
		}
	}
}

// jsonSchemaForType builds a JSON Schema fragment for a Report-shaped Go type.
// It handles exactly the shapes the Report schema uses: strings, string slices,
// and slices of flat structs. Struct fields use their json tag names.
func jsonSchemaForType(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Slice:
		return map[string]any{"type": "array", "items": jsonSchemaForType(t.Elem())}
	case reflect.Struct:
		props := map[string]any{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := jsonFieldName(f)
			if name == "" || name == "-" {
				continue
			}
			props[name] = jsonSchemaForType(f.Type)
		}
		return map[string]any{
			"type":                 "object",
			"properties":           props,
			"additionalProperties": false,
		}
	default:
		// Report only uses the shapes above; anything else degrades to a
		// permissive node rather than panicking.
		return map[string]any{}
	}
}

func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name
	}
	if comma := indexByte(tag, ','); comma >= 0 {
		tag = tag[:comma]
	}
	return tag
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// NewReportSinkServer builds the single-tool MCP server that persists an
// accepted report to outPath. A valid submission writes the canonical report
// JSON and returns the acceptance message; an invalid submission returns the
// exact validation error as an MCP tool error (isError) so the model corrects
// itself mid-turn without the runtime ever coercing.
func NewReportSinkServer(outPath string) *server.MCPServer {
	s := server.NewMCPServer("ghx-report-sink", "1",
		server.WithToolCapabilities(true),
	)
	tool := mcp.NewToolWithRawSchema(
		SubmitReportToolName,
		"Submit your final ghx-sidecar evidence report. This is the ONLY way to "+
			"complete an investigation turn. The report is validated strictly against "+
			"the report schema: on success it is accepted and you end your turn without repeating the answer; "+
			"on failure the exact validation error is returned so you can fix the report "+
			"and call submit_report again. No coercion is applied. Evidence is required: "+
			"a non-BLOCKED report needs at least one verified claim with evidence, one "+
			"relevant file, and one command run; a BLOCKED report must state why it is blocked.",
		reportInputSchema(),
	)
	s.AddTool(tool, reportSinkHandler(outPath))
	return s
}

func reportSinkHandler(outPath string) server.ToolHandlerFunc {
	return func(_ context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw := request.GetRawArguments()
		data, err := json.Marshal(raw)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("report failed validation: could not read arguments: %v", err)), nil
		}
		report, err := DecodeReportStrict(data)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := writeSinkReport(outPath, report); err != nil {
			// A persistence failure is the sink's fault, not the model's; report
			// it as a tool error so the turn does not falsely appear complete.
			return mcp.NewToolResultError(fmt.Sprintf("report accepted but could not be persisted: %v", err)), nil
		}
		return mcp.NewToolResultText(reportAcceptedMessage), nil
	}
}

// writeSinkReport writes the canonical report JSON to outPath atomically
// (temp file + rename) so the runtime never observes a half-written sink.
func writeSinkReport(outPath string, r *Report) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(outPath)
	tmp, err := os.CreateTemp(dir, ".report-sink-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, outPath)
}

// ReadSinkReport reads and validates an accepted report from the sink path.
// It returns (nil, nil) when the sink does not exist (no accepted submission),
// and a validated report when it does. A malformed sink is a hard error: the
// sink is only ever written with canonical JSON by writeSinkReport.
func ReadSinkReport(outPath string) (*Report, error) {
	data, err := os.ReadFile(outPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var r Report
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("corrupt report sink %s: %w", outPath, err)
	}
	if err := ValidateReport(&r); err != nil {
		return nil, fmt.Errorf("invalid report in sink %s: %w", outPath, err)
	}
	return &r, nil
}

// RunReportSink serves the report-sink MCP server over stdio until stdin
// closes. It backs the hidden `ghx sidecar report-sink` command.
func RunReportSink(outPath string) error {
	return server.ServeStdio(NewReportSinkServer(outPath))
}
