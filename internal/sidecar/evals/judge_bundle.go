package evals

import (
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// bundleSchemaVersion versions the JudgeBundle shape. Any change to the fields
// the judge reads is a measurement-stack change (ADR-0023.1 D4) and must
// re-trigger calibration before scores are citable.
const bundleSchemaVersion = "judge-bundle-v1"

// Bounds keep the bundle a *summary*, never a raw transcript (ADR-0023.1 D2,
// constraint 2 — TRAIL evidence against dumping full traces). The agent's own
// answer/thinking is the material being judged, so it gets a generous bound;
// tool output is strictly a short summary, never the full output.
const (
	bundleAnswerMax     = 6000
	bundleThinkingMax   = 6000
	bundleToolSummryMax = 280
)

// JudgeBundle is the profile-blind evidence packet the judge reads for one
// episode (ADR-0023.1 D2). It is assembled deterministically from committed
// episode artifacts and stored next to the score so every judgment is
// reproducible.
//
// Profile-blindness is a *structural* property of this type: it has no field
// for the profile label, the agent command, the adapter/model identity, the
// reward scores, or the gate results. BuildJudgeBundle never reads those
// episode fields, so the judge cannot brand-favor the architecture under test.
// The exclusions are asserted in judge_bundle_test.go against a real fixture
// episode from each profile.
type JudgeBundle struct {
	SchemaVersion string        `json:"schemaVersion"`
	TaskID        string        `json:"taskId"`
	Repo          string        `json:"repo"`
	Questions     []string      `json:"questions"`
	Turns         []BundleTurn  `json:"turns"`
	Report        *BundleReport `json:"report,omitempty"`
}

// BundleTurn is one turn's judge-visible material: the question, the agent's
// reasoning (when captured) and answer text, and tool-call titles with bounded
// output summaries — never full tool outputs.
type BundleTurn struct {
	Turn      int              `json:"turn"`
	Question  string           `json:"question"`
	Thinking  string           `json:"thinking,omitempty"`
	AgentText string           `json:"agentText,omitempty"`
	ToolCalls []BundleToolCall `json:"toolCalls,omitempty"`
}

// BundleToolCall is a tool-call title plus a short output summary and the true
// output size. The summary is a truncated first-line digest, not the payload.
type BundleToolCall struct {
	Title         string `json:"title"`
	OutputSummary string `json:"outputSummary,omitempty"`
	OutputChars   int    `json:"outputChars"`
}

// BundleReport is the accepted structured report when the episode produced one
// (sidecar profile). Direct profiles have no structured report; their answer
// lives in the per-turn AgentText. The evidence ledger is the report's own
// Evidence list (source + summary) — the ADR-0014.1 standalone ledger does not
// exist yet, so "ledger content if present" resolves to this.
type BundleReport struct {
	Answer         string           `json:"answer"`
	Verified       []BundleClaim    `json:"verified,omitempty"`
	Inferred       []BundleClaim    `json:"inferred,omitempty"`
	Unverified     []BundleClaim    `json:"unverified,omitempty"`
	RelevantFiles  []BundleFile     `json:"relevantFiles,omitempty"`
	EvidenceLedger []BundleEvidence `json:"evidenceLedger,omitempty"`
	CommandsRun    []string         `json:"commandsRun,omitempty"`
	Uncertainty    []string         `json:"uncertainty,omitempty"`
}

// BundleClaim is a claim with its self-reported evidence anchor.
type BundleClaim struct {
	Summary  string `json:"summary"`
	Evidence string `json:"evidence,omitempty"`
}

// BundleFile is a relevant file with the reason it was surfaced.
type BundleFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

// BundleEvidence is one evidence-ledger entry: the command/source and what it
// showed.
type BundleEvidence struct {
	Source  string `json:"source"`
	Summary string `json:"summary"`
}

// BuildJudgeBundle assembles the profile-blind evidence bundle for one episode
// (ADR-0023.1 D2). It is deterministic and reads only judge-safe fields: it
// never touches ep.Profile, ep.Identity, ep.Rewards, ep.Violations, or any
// gate output.
func BuildJudgeBundle(ep *Episode) JudgeBundle {
	b := JudgeBundle{
		SchemaVersion: bundleSchemaVersion,
		TaskID:        ep.TaskID,
		Repo:          ep.Repo,
	}
	for _, t := range ep.Turns {
		b.Questions = append(b.Questions, t.Question)
		b.Turns = append(b.Turns, bundleTurn(t))
	}
	if ep.Report != nil {
		b.Report = bundleReport(ep.Report)
	}
	return b
}

func bundleTurn(t TurnRecord) BundleTurn {
	bt := BundleTurn{
		Turn:      t.Turn,
		Question:  t.Question,
		Thinking:  boundedString(strings.TrimSpace(t.Thinking), bundleThinkingMax),
		AgentText: boundedString(strings.TrimSpace(t.Text), bundleAnswerMax),
	}
	if len(t.ToolTraces) > 0 {
		for _, tr := range t.ToolTraces {
			bt.ToolCalls = append(bt.ToolCalls, BundleToolCall{
				Title:         strings.TrimSpace(tr.Title),
				OutputSummary: summarizeToolOutput(tr.OutputExcerpt),
				OutputChars:   tr.OutputSize,
			})
		}
		return bt
	}
	// Fallback: some adapters record only tool-call titles, not full traces.
	for _, call := range t.ToolCalls {
		bt.ToolCalls = append(bt.ToolCalls, BundleToolCall{Title: strings.TrimSpace(call)})
	}
	return bt
}

func bundleReport(r *sidecar.Report) *BundleReport {
	br := &BundleReport{
		Answer:      r.Answer,
		CommandsRun: append([]string(nil), r.CommandsRun...),
		Uncertainty: append([]string(nil), r.Uncertainty...),
	}
	for _, c := range r.Verified {
		br.Verified = append(br.Verified, BundleClaim{Summary: c.Summary, Evidence: c.Evidence})
	}
	for _, c := range r.Inferred {
		br.Inferred = append(br.Inferred, BundleClaim{Summary: c.Summary, Evidence: c.Evidence})
	}
	for _, c := range r.Unverified {
		br.Unverified = append(br.Unverified, BundleClaim{Summary: c.Summary, Evidence: c.Evidence})
	}
	for _, f := range r.RelevantFiles {
		br.RelevantFiles = append(br.RelevantFiles, BundleFile{Path: f.Path, Reason: f.Reason})
	}
	for _, e := range r.Evidence {
		br.EvidenceLedger = append(br.EvidenceLedger, BundleEvidence{Source: e.Source, Summary: e.Summary})
	}
	return br
}

// summarizeToolOutput reduces a captured tool-output excerpt to a short,
// single-line digest. This enforces the D2 rule that the judge sees output
// *summaries*, not full outputs — even the stored excerpt is truncated here.
func summarizeToolOutput(excerpt string) string {
	excerpt = strings.TrimSpace(excerpt)
	if excerpt == "" {
		return ""
	}
	if idx := strings.IndexByte(excerpt, '\n'); idx >= 0 {
		first := strings.TrimSpace(excerpt[:idx])
		if len(first) >= 24 { // a meaningful first line stands alone
			return boundedString(first, bundleToolSummryMax)
		}
	}
	return boundedString(strings.Join(strings.Fields(excerpt), " "), bundleToolSummryMax)
}
