package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Ledger is a durable evidence notebook for one sidecar session.
type Ledger struct {
	Repo  string `json:"repo"`
	Scope string `json:"scope"`
	// Commit/Branch pin the repo snapshot the evidence was gathered against
	// (ADR-0037 M-2). Copied from SessionMeta when known; empty means the
	// evidence floats on the remote default branch — consumers must treat
	// line-level claims accordingly.
	Commit         string              `json:"commit,omitempty"`
	Branch         string              `json:"branch,omitempty"`
	CommandsRun    []LedgerEntry       `json:"commands_run"`
	InspectedPaths []LedgerEntry       `json:"inspected_paths"`
	MappedGlobs    []LedgerEntry       `json:"mapped_globs"`
	GrepPatterns   []LedgerEntry       `json:"grep_patterns"`
	RelevantFiles  []RelevantFileEntry `json:"relevant_files"`
	RejectedPaths  []RelevantFileEntry `json:"rejected_paths"`
	Backends       []LedgerEntry       `json:"backends"`
	OpenQuestions  []LedgerEntry       `json:"open_questions"`
	UpdatedAt      string              `json:"updatedAt"`
}

// LedgerEntry records a unique evidence item and the turn that first added it.
type LedgerEntry struct {
	Value string `json:"value"`
	Turn  int    `json:"turn"`
}

// RelevantFileEntry records a relevant or rejected path with a one-line reason.
type RelevantFileEntry struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
	Turn   int    `json:"turn"`
}

// LoadLedger reads ledger.json for a session, returning an empty ledger when no
// ledger has been written yet.
func LoadLedger(sessionsDir, name string) (*Ledger, error) {
	path := filepath.Join(sessionDir(sessionsDir, name), "ledger.json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &Ledger{}, nil
	}
	if err != nil {
		return nil, err
	}
	var l Ledger
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// SaveLedger atomically writes ledger.json for a session.
func SaveLedger(sessionsDir, name string, ledger *Ledger) error {
	dir := sessionDir(sessionsDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	ledger.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".ledger-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, filepath.Join(dir, "ledger.json"))
}

// UpdateLedgerFromTurn derives session memory from ghx-owned report and trace
// data. It intentionally ignores model-authored memory outside the report.
func UpdateLedgerFromTurn(ledger *Ledger, meta *SessionMeta, report *Report, traces []ToolCallTrace, turn int) {
	if meta != nil {
		if ledger.Repo == "" {
			ledger.Repo = meta.Repo
		}
		if ledger.Scope == "" {
			ledger.Scope = meta.Scope
		}
		// Snapshot identity (ADR-0037 M-2): stamp the ref evidence was
		// gathered against, when the session knows it.
		if ledger.Commit == "" {
			ledger.Commit = meta.Commit
		}
		if ledger.Branch == "" {
			ledger.Branch = meta.Branch
		}
	}
	if report != nil {
		for _, rf := range report.RelevantFiles {
			addRelevantFile(&ledger.RelevantFiles, rf.Path, rf.Reason, turn)
		}
		for _, b := range report.BackendsUsed {
			addLedgerEntry(&ledger.Backends, b, turn)
		}
		for _, c := range report.CommandsRun {
			addLedgerEntry(&ledger.CommandsRun, c, turn)
			addCommandEvidence(ledger, c, turn)
		}
		for _, q := range report.Uncertainty {
			addLedgerEntry(&ledger.OpenQuestions, q, turn)
		}
		for _, q := range report.NextReads {
			addLedgerEntry(&ledger.OpenQuestions, q, turn)
		}
	}
	ApplyTraceCommands(ledger, TraceCommandLedger(traces), turn)
}

// TraceCommandLedger extracts the bare trace-derived command strings the
// ledger consumes from a turn's tool traces. It is persisted verbatim in the
// per-turn ReportArtifact (ADR-0030.1 D5): the artifact must capture
// everything ledger derivation consumes, so replaying reports/ reproduces
// ledger.json exactly.
func TraceCommandLedger(traces []ToolCallTrace) []string {
	var out []string
	for _, tr := range traces {
		cmd := commandFromRawInput(tr.RawInput)
		if strings.TrimSpace(cmd) == "" {
			continue
		}
		out = append(out, cmd)
	}
	return out
}

// ApplyTraceCommands merges trace-derived command strings into the ledger,
// exactly as UpdateLedgerFromTurn does for a live turn. RebuildLedger replays
// the persisted TraceCommandLedger through this same code path (ADR-0030.1
// D5 rebuild-determinism invariant).
func ApplyTraceCommands(ledger *Ledger, commands []string, turn int) {
	for _, cmd := range commands {
		if strings.TrimSpace(cmd) == "" {
			continue
		}
		addLedgerEntry(&ledger.CommandsRun, cmd, turn)
		addCommandEvidence(ledger, cmd, turn)
	}
}

func addCommandEvidence(ledger *Ledger, command string, turn int) {
	e := DeriveCommandEvidence(command)
	for _, p := range e.InspectedPaths {
		addLedgerEntry(&ledger.InspectedPaths, p, turn)
	}
	for _, g := range e.MappedGlobs {
		addLedgerEntry(&ledger.MappedGlobs, g, turn)
	}
	for _, p := range e.GrepPatterns {
		addLedgerEntry(&ledger.GrepPatterns, p, turn)
	}
	for _, b := range e.Backends {
		addLedgerEntry(&ledger.Backends, b, turn)
	}
}

// CommandEvidence is the conservative parse result for a ghx command string.
type CommandEvidence struct {
	InspectedPaths []string
	MappedGlobs    []string
	GrepPatterns   []string
	Backends       []string
}

// DeriveCommandEvidence extracts paths, globs, grep patterns, and backend hints
// from common ghx read/tree/search command shapes.
//
// The input is a raw tool-call string as persisted in ReportArtifacts, not a
// well-formed single command: models batch several ghx invocations separated by
// newlines or shell operators, interleave echo markers, and copy ledger
// annotations onto cached turns ("(cached, turn 1)", "2>/dev/null || …"). The
// string is therefore segmented quote-aware on newlines and shell separators,
// and only segments whose first token is `ghx` are parsed; everything else
// (echo/decoration lines, prose, JSON blobs) contributes no evidence. Stored
// command strings themselves are never rewritten — hygiene lives entirely in
// this derivation (ADR-0030.1 D5 rebuild-determinism invariant).
func DeriveCommandEvidence(command string) CommandEvidence {
	var out CommandEvidence
	for _, segment := range ghxCommandSegments(command) {
		mergeCommandEvidence(&out, deriveSegmentEvidence(segment))
	}
	return out
}

// ghxCommandSegments splits a raw tool-call string into candidate command
// segments, cutting on newlines and shell separators (; | &) that sit outside
// quotes so quoted arguments spanning those characters survive intact.
func ghxCommandSegments(command string) []string {
	var segments []string
	var b strings.Builder
	var quote rune
	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			segments = append(segments, s)
		}
		b.Reset()
	}
	for _, r := range command {
		switch {
		case quote != 0:
			b.WriteRune(r)
			if r == quote {
				quote = 0
			}
		case r == '\'' || r == '"':
			b.WriteRune(r)
			quote = r
		case r == '\n' || r == '\r' || r == ';' || r == '|' || r == '&':
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return segments
}

// mergeCommandEvidence unions src into dst per category, preserving order and
// dropping duplicates across segments of one command string.
func mergeCommandEvidence(dst *CommandEvidence, src CommandEvidence) {
	dst.InspectedPaths = appendUnique(dst.InspectedPaths, src.InspectedPaths)
	dst.MappedGlobs = appendUnique(dst.MappedGlobs, src.MappedGlobs)
	dst.GrepPatterns = appendUnique(dst.GrepPatterns, src.GrepPatterns)
	dst.Backends = appendUnique(dst.Backends, src.Backends)
}

func appendUnique(dst []string, vals []string) []string {
	for _, v := range vals {
		dup := false
		for _, d := range dst {
			if d == v {
				dup = true
				break
			}
		}
		if !dup {
			dst = append(dst, v)
		}
	}
	return dst
}

// deriveSegmentEvidence parses one well-formed command segment (the output of
// ghxCommandSegments) for a single ghx read/tree/search invocation.
func deriveSegmentEvidence(segment string) CommandEvidence {
	fields := shellFields(segment)
	if len(fields) < 2 || fields[0] != "ghx" {
		return CommandEvidence{}
	}
	sub := fields[1]
	if sub != "read" && sub != "tree" && sub != "search" {
		return CommandEvidence{}
	}
	out := CommandEvidence{Backends: []string{"remote"}}
	args := fields[2:]
	if len(args) == 0 {
		return out
	}
	if looksRepo(args[0]) {
		args = args[1:]
	}
	// Positional arity per the CLI contract (internal/cli/ghx.go): read takes
	// 1-10 paths, tree at most one, search one free-form query. Capping the
	// positional run keeps prose that follows the real arguments (echo
	// markers, annotations, JSON debris) out of the evidence.
	maxPaths := 10
	if sub == "tree" {
		maxPaths = 1
	}
	var paths []string
	hasMap := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--map" {
			hasMap = true
			continue
		}
		if arg == "--grep" {
			if i+1 < len(args) {
				out.GrepPatterns = append(out.GrepPatterns, args[i+1])
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--grep=") {
			out.GrepPatterns = append(out.GrepPatterns, strings.TrimPrefix(arg, "--grep="))
			continue
		}
		if arg == "--backend" {
			if i+1 < len(args) {
				out.Backends = []string{args[i+1]}
				i++
			}
			continue
		}
		if strings.HasPrefix(arg, "--backend=") {
			out.Backends = []string{strings.TrimPrefix(arg, "--backend=")}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			if flagConsumesNext(arg) && i+1 < len(args) {
				i++
			}
			continue
		}
		if sub == "search" {
			if len(out.GrepPatterns) == 0 {
				out.GrepPatterns = append(out.GrepPatterns, arg)
			}
			continue
		}
		if !cleanPathToken(arg) {
			// Path-invalid token: annotation or redirection debris inside
			// the segment (e.g. "(cached," before "turn"). Nothing from
			// here on is trusted positional input.
			break
		}
		if len(paths) == maxPaths {
			continue
		}
		paths = append(paths, arg)
	}
	for _, p := range paths {
		if !cleanPathToken(p) {
			continue
		}
		if hasGlob(p) && hasMap {
			out.MappedGlobs = append(out.MappedGlobs, p)
			continue
		}
		out.InspectedPaths = append(out.InspectedPaths, p)
	}
	return out
}

// cleanPathToken reports whether a positional token can plausibly be a repo
// path. Tokens containing whitespace, parentheses, or shell metacharacters are
// annotation or redirection debris (e.g. "(cached,", "turn", "1)",
// "2>/dev/null", "=== ADD ==="), never paths.
func cleanPathToken(p string) bool {
	return !strings.ContainsAny(p, " ()<>;&|")
}

func commandFromRawInput(raw any) string {
	return resolveToolInput(raw, "")
}

func addLedgerEntry(entries *[]LedgerEntry, value string, turn int) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	for i := range *entries {
		if (*entries)[i].Value == value {
			return
		}
	}
	*entries = append(*entries, LedgerEntry{Value: value, Turn: turn})
}

func addRelevantFile(entries *[]RelevantFileEntry, path, reason string, turn int) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	reason = oneLine(reason)
	for i := range *entries {
		if (*entries)[i].Path == path {
			if (*entries)[i].Reason == "" && reason != "" {
				(*entries)[i].Reason = reason
			}
			return
		}
	}
	*entries = append(*entries, RelevantFileEntry{Path: path, Reason: reason, Turn: turn})
}

func shellFields(s string) []string {
	var out []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		switch {
		case escaped:
			b.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
		default:
			b.WriteRune(r)
		}
	}
	if escaped {
		b.WriteRune('\\')
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out
}

func looksRepo(s string) bool {
	return strings.Count(s, "/") == 1 && !strings.HasPrefix(s, "/")
}

func hasGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func flagConsumesNext(flag string) bool {
	switch flag {
	case "--lines", "--kind", "--depth", "--ref", "--branch", "--tag", "--limit", "--language":
		return true
	}
	return false
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	const max = 160
	if len(s) > max {
		return s[:max]
	}
	return s
}
