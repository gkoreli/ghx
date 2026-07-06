// Package evals implements the minimal agentic eval kernel from ADR-0016.1.
//
// It compares three profiles (plain, ghx, ghx-sidecar) on GitHub repository
// reconnaissance tasks, scores episodes with deterministic checklist rewards,
// and persists episode artifacts as JSON. The package compiles untagged;
// only tests that spawn a real ACP agent carry the agent_e2e build tag.
package evals

import (
	"fmt"
	"strings"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// TaskChecks holds the deterministic ground truth used by reward computation.
type TaskChecks struct {
	// ExpectedFiles are repo paths (suffix-matched) a correct answer identifies.
	ExpectedFiles []string `json:"expectedFiles,omitempty"`
	// ExpectedSymbols are function/type names that must appear in the answer.
	ExpectedSymbols []string `json:"expectedSymbols,omitempty"`
	// RequiredClaims are substrings that must appear in the answer text.
	RequiredClaims []string `json:"requiredClaims,omitempty"`
	// UnacceptableClaims zero out correctness when present (hallucinations).
	UnacceptableClaims []string `json:"unacceptableClaims,omitempty"`
	// AvoidPaths are path substrings the agent should not touch (e.g. "test/").
	AvoidPaths []string `json:"avoidPaths,omitempty"`
}

// Task is one reconnaissance scenario: a repo, one or more question turns,
// and the deterministic checks a correct investigation satisfies.
type Task struct {
	ID     string     `json:"id"`
	Repo   string     `json:"repo"`
	Turns  []string   `json:"turns"`
	Checks TaskChecks `json:"checks"`
	Tags   []string   `json:"tags,omitempty"`
}

// Validate reports whether the task is well-formed enough to score, and
// enforces the ADR-0016.2 benchmark validity rules: every check string must
// be non-blank and must be *discoverable* — a check that appears verbatim in
// a question is given away, so an agent could score it by parroting the
// question without exploring. Leaked unacceptable claims are rejected too:
// they would zero correctness on every honest echo of the question.
func (t Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task: id is required")
	}
	if t.Repo == "" {
		return fmt.Errorf("task %s: repo is required", t.ID)
	}
	if len(t.Turns) == 0 {
		return fmt.Errorf("task %s: at least one turn is required", t.ID)
	}
	c := t.Checks
	if len(c.ExpectedFiles) == 0 && len(c.ExpectedSymbols) == 0 && len(c.RequiredClaims) == 0 {
		return fmt.Errorf("task %s: at least one positive check is required", t.ID)
	}

	checkGroups := map[string][]string{
		"expectedFiles":      c.ExpectedFiles,
		"expectedSymbols":    c.ExpectedSymbols,
		"requiredClaims":     c.RequiredClaims,
		"unacceptableClaims": c.UnacceptableClaims,
	}
	for group, checks := range checkGroups {
		for _, check := range checks {
			if strings.TrimSpace(check) == "" {
				return fmt.Errorf("task %s: %s contains a blank entry (matches everything)", t.ID, group)
			}
			for i, turn := range t.Turns {
				if strings.Contains(strings.ToLower(turn), strings.ToLower(check)) {
					return fmt.Errorf(
						"task %s: %s entry %q leaks into question turn %d — checks must be discoverable, not given",
						t.ID, group, check, i)
				}
			}
		}
	}
	return nil
}

// TurnRecord captures what happened during one episode turn.
type TurnRecord struct {
	Turn     int    `json:"turn"`
	Question string `json:"question"`
	Text     string `json:"text"`
	// Thinking captures ACP agent_thought_chunk reasoning for this turn.
	Thinking  string   `json:"thinking,omitempty"`
	ToolCalls []string `json:"toolCalls"`
	// ToolTraces carries full ACP tool-call audit data for this turn.
	ToolTraces []ToolCallTrace `json:"toolTraces,omitempty"`
	// ReplayedText is message text replayed before this turn's prompt was sent.
	// It is audit-only and excluded from turn output/accounting.
	ReplayedText string `json:"replayedText,omitempty"`
	// ReplayedThinking is reasoning replayed before this turn's prompt was sent.
	// It is audit-only and excluded from live turn telemetry.
	ReplayedThinking string `json:"replayedThinking,omitempty"`
	// ReplayedToolTraces carries ACP tool-call history replayed before this
	// turn's prompt was sent. It is excluded from actions, observations,
	// memory/repeat-read scoring, ledger derivation, and char accounting.
	ReplayedToolTraces []ToolCallTrace `json:"replayedToolTraces,omitempty"`
	// Report is the structured report extracted this turn (sidecar profile).
	Report *sidecar.Report `json:"report,omitempty"`
	// ReportRetried records whether sidecar report extraction needed the
	// one-shot corrective retry before producing the final report.
	ReportRetried bool `json:"reportRetried,omitempty"`
	// Resumed reports whether this turn continued prior agent context
	// (ACP LoadSession for the sidecar profile; same live session for
	// direct profiles). Always false on turn 0.
	Resumed bool `json:"resumed"`
	// ToolOutputChars approximates tool-output content the agent consumed
	// this turn (from tool_call/tool_call_update events).
	ToolOutputChars int   `json:"toolOutputChars"`
	DurationMs      int64 `json:"durationMs"`
	// Error records a turn failure; the episode keeps partial data.
	Error string `json:"error,omitempty"`
}

// ToolStatusTransition records one observed status for an ACP tool call.
type ToolStatusTransition struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

// ToolCallTrace is the durable per-tool-call audit record captured from ACP.
type ToolCallTrace struct {
	ID                string                 `json:"id"`
	Kind              string                 `json:"kind,omitempty"`
	Title             string                 `json:"title,omitempty"`
	RawInput          any                    `json:"rawInput,omitempty"`
	StatusTransitions []ToolStatusTransition `json:"statusTransitions,omitempty"`
	OutputSize        int                    `json:"outputSize"`
	OutputExcerpt     string                 `json:"outputExcerpt,omitempty"`
}

// Action is the training/audit action projection of a tool invocation.
type Action struct {
	ActionIndex int       `json:"actionIndex"`
	Turn        int       `json:"turn"`
	Type        string    `json:"type"`
	ToolCallID  string    `json:"toolCallId,omitempty"`
	Kind        string    `json:"kind,omitempty"`
	Name        string    `json:"name,omitempty"`
	Input       string    `json:"input,omitempty"`
	At          time.Time `json:"at"`
}

// Observation is the bounded output projection attached to an action.
type Observation struct {
	ActionIndex int       `json:"actionIndex"`
	Turn        int       `json:"turn"`
	ToolCallID  string    `json:"toolCallId,omitempty"`
	Text        string    `json:"text,omitempty"`
	OutputSize  int       `json:"outputSize"`
	At          time.Time `json:"at"`
}

// ContextAccounting models the workflow boundary from ADR-0016.1: how many
// characters entered the expensive main agent's context versus stayed inside
// the cheap sidecar. For direct profiles main == total by construction.
type ContextAccounting struct {
	MainAgentChars       int `json:"mainAgentChars"`
	SidecarInternalChars int `json:"sidecarInternalChars"`
	TotalWorkflowChars   int `json:"totalWorkflowChars"`
}

// RewardBreakdown is the deterministic checklist score for one episode.
// All values are in [0, 1]. Memory only applies to multi-turn tasks.
type RewardBreakdown struct {
	Correctness   float64 `json:"correctness"`
	Evidence      float64 `json:"evidence"`
	Trajectory    float64 `json:"trajectory"`
	Compression   float64 `json:"compression"`
	Memory        float64 `json:"memory"`
	MemoryApplies bool    `json:"memoryApplies"`
	Safety        float64 `json:"safety"`
	Overall       float64 `json:"overall"`
}

// AgentIdentity records the executable/model identity for a run or episode.
type AgentIdentity struct {
	AgentCommand        string `json:"agentCommand,omitempty"`
	AdapterName         string `json:"adapterName,omitempty"`
	AdapterVersion      string `json:"adapterVersion,omitempty"`
	SubjectModel        string `json:"subjectModel,omitempty"`
	AdapterSubjectModel string `json:"adapterSubjectModel,omitempty"`
	WrapperSHA256       string `json:"wrapperSha256,omitempty"`
}

// Episode is the durable record of one task × profile run. Serialized JSON
// is the canonical artifact (ADR-0016: local artifacts are the source of truth).
type Episode struct {
	ID      string       `json:"id"`
	TaskID  string       `json:"taskId"`
	Repo    string       `json:"repo"`
	Profile Profile      `json:"profile"`
	Turns   []TurnRecord `json:"turns"`
	Actions []Action     `json:"actions,omitempty"`
	// Observations captures bounded tool outputs; OutputSize is exact even
	// when Text is truncated to the first 2048 bytes.
	Observations []Observation `json:"observations,omitempty"`
	Identity     AgentIdentity `json:"identity,omitempty"`
	// Report is the final structured report (sidecar profile only).
	Report           *sidecar.Report `json:"report,omitempty"`
	Invalid          bool            `json:"invalid,omitempty"`
	ExclusionReasons []string        `json:"exclusionReasons,omitempty"`
	// Violations lists safety-contract breaches observed during the run
	// (write attempts, terminal requests, write-kind permission requests).
	Violations []string          `json:"violations,omitempty"`
	Anomalies  []Anomaly         `json:"anomalies,omitempty"`
	Context    ContextAccounting `json:"context"`
	Rewards    RewardBreakdown   `json:"rewards"`
	StartedAt  time.Time         `json:"startedAt"`
	EndedAt    time.Time         `json:"endedAt"`
}
