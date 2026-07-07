package hosttask

import (
	"slices"
	"strings"

	acp "github.com/coder/acp-go-sdk"
	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// ToolCallClass is the structural exploration/engineering attribution of one
// host tool call (ADR-0032 metric table).
type ToolCallClass string

const (
	// ClassExploration marks reconnaissance of the world outside the local
	// checkout: recon MCP calls, web fetches, external gh/network commands,
	// and file access outside the workspace (the ADR-0032 structural
	// boundary — exploration ≡ outside the local checkout).
	ClassExploration ToolCallClass = "exploration"
	// ClassEngineering marks the host working its own objective: workspace
	// read/edit/test and other local commands.
	ClassEngineering ToolCallClass = "engineering"
	// ClassUnclassified marks calls the structural rules cannot attribute.
	// It is a NAMED bucket, never silently folded into either class — a
	// human must be able to recompute both totals from the published table
	// (visibility tenet; ADR-0032 trap 6).
	ClassUnclassified ToolCallClass = "unclassified"
)

// Classifier attributes host tool calls to exploration vs engineering by
// TOOL IDENTITY and PATH SCOPE only — never by content heuristics
// (ADR-0032 trap 6: a heuristic classifier can be gamed by an arm, so the
// rules are structural, unit-tested, and frozen with the measurement
// stack). For execute-kind calls the command line comes from the trace's
// RawInput "command" field — live claude-agent-acp execute tool calls carry
// the title "Terminal", NOT the command (sighted 2026-07-06, ADR-0032.1 S3
// note) — with the title as fallback for adapters that do title the command
// line. The command's leading tokens are its identity.
//
// Rules, evaluated in order, first match wins:
//
//	R1  title equals one of ReconToolNames                → exploration
//	R2  kind "fetch" (network by definition)              → exploration
//	R3  kind "execute", compound command — contains
//	    "&&", "||", ";", "|", "$(", "`" or a newline —
//	    or an empty command (one argv identity cannot
//	    speak for a whole pipeline)                       → unclassified
//	R4  kind "execute", leading tokens match an
//	    ExplorationCommands entry                         → exploration
//	R5  kind "execute", any other simple command          → engineering
//	R6  kinds read/edit/delete/move/search:
//	      no recorded locations                           → unclassified
//	      every location inside the workspace             → engineering
//	      every location outside the workspace            → exploration
//	      mixed, or any location unresolvable             → unclassified
//	R7  kind "think" (the host planning its own work)     → engineering
//	R8  anything else (unknown or empty kind)             → unclassified
//
// Every knob is an explicit field — nothing reads config, environment, or
// globals (frozen-measurement-stack requirement, ADR-0032.1).
type Classifier struct {
	// Scope is the provisioned trial workspace that defines "inside".
	Scope WorkspaceScope
	// ReconToolNames are the exact tool-call titles of the recon MCP
	// surface (arm B). Exact string match — tool identity, not substring.
	ReconToolNames []string
	// ExplorationCommands are token-prefix command identities counted as
	// external/network exploration for execute-kind calls, e.g. "gh" or
	// "git fetch": a command matches when its leading whitespace-separated
	// tokens equal an entry's tokens. See DefaultExplorationCommands.
	ExplorationCommands []string
}

// DefaultExplorationCommands is the frozen default command-identity list for
// the control arm's native exploration (ADR-0032 D1: shell + gh, web off):
// external/network reconnaissance tools. Changing this list mid-run is a
// measurement-stack change and must be pre-registered in an ADR first.
func DefaultExplorationCommands() []string {
	return []string{
		"gh",
		"curl",
		"wget",
		"git clone",
		"git fetch",
		"git pull",
		"git ls-remote",
	}
}

// AttributedToolCall is one row of the per-episode attribution table: one
// host tool call, its structural class, and its chars — the trace's
// OutputSize, i.e. the tool output that entered host context.
type AttributedToolCall struct {
	ID    string        `json:"id"`
	Title string        `json:"title,omitempty"`
	Kind  string        `json:"kind,omitempty"`
	Class ToolCallClass `json:"class"`
	Chars int           `json:"chars"`
}

// AttributionTable is the per-episode exploration/engineering token
// attribution (ADR-0032 metric table). It is published with the episode so
// a human can recompute every total from the rows (visibility tenet).
type AttributionTable struct {
	// PerToolCall has one row per host tool call, in trace order.
	PerToolCall []AttributedToolCall `json:"perToolCall"`
	// ExplorationChars sums Chars over exploration-classified rows.
	ExplorationChars int `json:"explorationChars"`
	// EngineeringChars sums Chars over engineering-classified rows.
	EngineeringChars int `json:"engineeringChars"`
	// Unclassified lists the tool-call IDs the rules could not attribute,
	// in trace order. Their chars count toward TotalChars but toward
	// neither class — the bucket is named, never folded away.
	Unclassified []string `json:"unclassified"`
}

// Attribute classifies traces into the per-episode attribution table.
func (c Classifier) Attribute(traces []sidecar.ToolCallTrace) AttributionTable {
	var t AttributionTable
	for _, tr := range traces {
		class := c.classify(tr)
		t.PerToolCall = append(t.PerToolCall, AttributedToolCall{
			ID:    tr.ID,
			Title: tr.Title,
			Kind:  tr.Kind,
			Class: class,
			Chars: tr.OutputSize,
		})
		switch class {
		case ClassExploration:
			t.ExplorationChars += tr.OutputSize
		case ClassEngineering:
			t.EngineeringChars += tr.OutputSize
		default:
			t.Unclassified = append(t.Unclassified, tr.ID)
		}
	}
	return t
}

// TotalChars sums Chars over every row, including unclassified ones.
func (t AttributionTable) TotalChars() int {
	total := 0
	for _, row := range t.PerToolCall {
		total += row.Chars
	}
	return total
}

// DeadWeightFraction is exploration chars ÷ total chars — the share of host
// tool output spent on exploration (ADR-0016.6's dead-weight notion,
// operationalized by ADR-0032's structural boundary). Zero for an empty
// table.
//
// Chars are the fallback accounting only: the registered GH1-efficiency
// metric uses real gen_ai.usage.* token counts (ADR-0032.1 D2 — chars/4 may
// be shown alongside but can never carry the gate by itself).
func (t AttributionTable) DeadWeightFraction() float64 {
	total := t.TotalChars()
	if total == 0 {
		return 0
	}
	return float64(t.ExplorationChars) / float64(total)
}

// classify applies the rule table (see Classifier) to one trace.
func (c Classifier) classify(tr sidecar.ToolCallTrace) ToolCallClass {
	if tr.Title != "" && slices.Contains(c.ReconToolNames, tr.Title) {
		return ClassExploration // R1
	}
	switch acp.ToolKind(tr.Kind) {
	case acp.ToolKindFetch:
		return ClassExploration // R2
	case acp.ToolKindExecute:
		return c.classifyCommand(ExecuteCommandLine(tr.RawInput, tr.Title)) // R3–R5
	case acp.ToolKindRead, acp.ToolKindEdit, acp.ToolKindDelete, acp.ToolKindMove, acp.ToolKindSearch:
		return c.classifyLocations(tr.Locations) // R6
	case acp.ToolKindThink:
		return ClassEngineering // R7
	}
	return ClassUnclassified // R8
}

// adapterTerminalTitle is the fixed label live claude-agent-acp puts on
// every execute tool call (sighted 2026-07-06; ADR-0032.1 S3 note). It is a
// tool label, never a command line — an execute trace with this title and no
// rawInput command carries no command identity at all.
const adapterTerminalTitle = "Terminal"

// ExecuteCommandLine returns the command-line identity of one execute-kind
// tool call, given its recorded rawInput and title. The authoritative source
// is rawInput's "command" field: the adapter forwards the SDK tool input,
// and live claude-agent-acp execute tool calls carry the title "Terminal",
// not the command (live sighting 2026-07-06; ADR-0032.1 S3 note). The title
// is the fallback for adapters — and older artifacts — whose execute titles
// are the command line, except the known "Terminal" label, which yields the
// empty command (rule R3: cannot be checked ⇒ unclassified, never guessed).
// JSON-roundtripped artifacts decode rawInput as map[string]any, so live and
// reloaded episodes resolve identically. Shared by the classifier (R3–R5)
// and the evals host_execute_outside_workspace detector — one extraction
// authority.
func ExecuteCommandLine(rawInput any, title string) string {
	if m, ok := rawInput.(map[string]any); ok {
		if s, ok := m["command"].(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	if title == adapterTerminalTitle {
		return ""
	}
	return title
}

// classifyCommand applies rules R3–R5 to an execute-kind command line.
func (c Classifier) classifyCommand(cmd string) ToolCallClass {
	tokens := strings.Fields(cmd)
	if len(tokens) == 0 || compoundShell(cmd) {
		return ClassUnclassified // R3
	}
	for _, prefix := range c.ExplorationCommands {
		if matchesCommandPrefix(tokens, prefix) {
			return ClassExploration // R4
		}
	}
	return ClassEngineering // R5
}

// classifyLocations applies rule R6 to a file-shaped tool call's paths.
func (c Classifier) classifyLocations(paths []string) ToolCallClass {
	if len(paths) == 0 {
		return ClassUnclassified
	}
	inside := 0
	for _, p := range paths {
		resolved, err := c.Scope.Resolve(p)
		if err != nil {
			return ClassUnclassified
		}
		if c.Scope.Contains(resolved) {
			inside++
		}
	}
	switch inside {
	case len(paths):
		return ClassEngineering
	case 0:
		return ClassExploration
	}
	return ClassUnclassified // mixed scopes
}

// compoundShell reports whether cmd chains multiple commands; a single argv
// identity cannot classify a pipeline (rule R3).
func compoundShell(cmd string) bool {
	for _, op := range []string{"&&", "||", ";", "|", "$(", "`", "\n"} {
		if strings.Contains(cmd, op) {
			return true
		}
	}
	return false
}

// matchesCommandPrefix reports whether the command's leading tokens equal
// the prefix entry's tokens ("git fetch" matches "git fetch origin main",
// never "git fetchx" or "notgit fetch").
func matchesCommandPrefix(tokens []string, prefix string) bool {
	p := strings.Fields(prefix)
	if len(p) == 0 || len(tokens) < len(p) {
		return false
	}
	for i := range p {
		if tokens[i] != p[i] {
			return false
		}
	}
	return true
}
