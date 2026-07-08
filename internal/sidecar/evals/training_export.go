package evals

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

const (
	// TrainingExportVersion identifies the deterministic SFT export schema.
	TrainingExportVersion = "sft-trl-conversational-v1"
	// DefaultSFTRewardFloor is ADR-0017.1 D4's initial SFT quality floor.
	DefaultSFTRewardFloor = 0.6
)

// SFTExportOptions configures an offline TRL conversational JSONL export.
type SFTExportOptions struct {
	// RunDir is the committed eval run directory containing episode JSON files.
	RunDir string
	// OutPath is the JSONL path to write. A sibling manifest is written next to it.
	OutPath string
	// RewardFloor is the minimum episode Rewards.Overall required for SFT.
	RewardFloor float64
}

// SFTExportManifest records the recomputable provenance and filter accounting
// for one JSONL export.
type SFTExportManifest struct {
	ExporterVersion string         `json:"exporterVersion"`
	Format          string         `json:"format"`
	RunDir          string         `json:"runDir"`
	OutPath         string         `json:"outPath"`
	CreatedAt       string         `json:"createdAt"`
	Filters         SFTFilters     `json:"filters"`
	Counts          SFTCounts      `json:"counts"`
	SourceEpisodes  []string       `json:"sourceEpisodes"`
	IncludedRecords []SFTRecordRef `json:"includedRecords"`
}

// SFTFilters lists the pre-registered quality filters applied by the exporter.
type SFTFilters struct {
	ConsentSource                  string  `json:"consentSource"`
	SafetyRequired                 float64 `json:"safetyRequired"`
	RewardFloor                    float64 `json:"rewardFloor"`
	InvalidEpisodesExcluded        bool    `json:"invalidEpisodesExcluded"`
	ExclusionReasonsExcluded       bool    `json:"exclusionReasonsExcluded"`
	BreakingAnomaliesExcluded      bool    `json:"breakingAnomaliesExcluded"`
	AnswerDocContaminationExcluded bool    `json:"answerDocContaminationExcluded"`
	ExactRecordDedup               bool    `json:"exactRecordDedup"`
	TaskPromptDecontamination      string  `json:"taskPromptDecontamination"`
}

// SFTCounts summarizes export inputs, outputs, and exclusions by stable reason.
type SFTCounts struct {
	EpisodesRead     int            `json:"episodesRead"`
	EpisodesIncluded int            `json:"episodesIncluded"`
	RecordsWritten   int            `json:"recordsWritten"`
	Excluded         map[string]int `json:"excluded"`
}

// SFTRecordRef links one emitted JSONL record back to its source artifact.
type SFTRecordRef struct {
	EpisodeID string  `json:"episodeId"`
	TaskID    string  `json:"taskId"`
	Profile   Profile `json:"profile"`
	Turn      int     `json:"turn"`
	SHA256    string  `json:"sha256"`
}

// TRLRecord is one TRL SFTTrainer conversational example with tool schemas.
type TRLRecord struct {
	Messages []TRLMessage `json:"messages"`
	Tools    []TRLTool    `json:"tools,omitempty"`
}

// TRLMessage is one canonical chat message in TRL's conversational dataset.
type TRLMessage struct {
	Role      string        `json:"role"`
	Content   string        `json:"content,omitempty"`
	Name      string        `json:"name,omitempty"`
	ToolCalls []TRLToolCall `json:"tool_calls,omitempty"`
}

// TRLToolCall is one assistant function call in TRL's tool-calling format.
type TRLToolCall struct {
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type"`
	Function TRLFunctionCall `json:"function"`
}

// TRLFunctionCall names a function call and carries JSON-object arguments.
type TRLFunctionCall struct {
	Name      string `json:"name"`
	Arguments any    `json:"arguments,omitempty"`
}

// TRLTool is one JSON-schema tool definition attached to a TRL record.
type TRLTool struct {
	Type     string          `json:"type"`
	Function TRLToolFunction `json:"function"`
}

// TRLToolFunction describes a callable tool in the JSON schema shape expected
// by chat templates and TRL's tool-calling dataset support.
type TRLToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ExportSFT writes deterministic TRL conversational JSONL records for one eval
// run and returns the sibling manifest. It reads episode artifacts only.
func ExportSFT(opts SFTExportOptions) (*SFTExportManifest, error) {
	if strings.TrimSpace(opts.RunDir) == "" {
		return nil, fmt.Errorf("run dir is required")
	}
	if strings.TrimSpace(opts.OutPath) == "" {
		return nil, fmt.Errorf("out path is required")
	}
	if opts.RewardFloor == 0 {
		opts.RewardFloor = DefaultSFTRewardFloor
	}

	episodes, err := LoadRunEpisodes(opts.RunDir)
	if err != nil {
		return nil, fmt.Errorf("load run episodes: %w", err)
	}
	sort.SliceStable(episodes, func(i, j int) bool {
		return episodes[i].ID < episodes[j].ID
	})

	manifest := &SFTExportManifest{
		ExporterVersion: TrainingExportVersion,
		Format:          "sft",
		RunDir:          opts.RunDir,
		OutPath:         opts.OutPath,
		CreatedAt:       "1970-01-01T00:00:00Z",
		Filters: SFTFilters{
			ConsentSource:                  "eval episodes over public repos; consented by ADR-0017.1 D5",
			SafetyRequired:                 1.0,
			RewardFloor:                    opts.RewardFloor,
			InvalidEpisodesExcluded:        true,
			ExclusionReasonsExcluded:       true,
			BreakingAnomaliesExcluded:      true,
			AnswerDocContaminationExcluded: true,
			ExactRecordDedup:               true,
			TaskPromptDecontamination:      "not applied to eval-episode source prompts in this SFT slice; existing answer_doc_contamination episode exclusions are enforced",
		},
		Counts: SFTCounts{Excluded: map[string]int{}},
	}

	var lines [][]byte
	seenRecords := map[string]bool{}
	includedEpisodes := map[string]bool{}
	for _, ep := range episodes {
		if ep == nil {
			continue
		}
		manifest.Counts.EpisodesRead++
		manifest.SourceEpisodes = append(manifest.SourceEpisodes, ep.ID)
		if reason := sftEpisodeExclusionReason(ep, opts.RewardFloor); reason != "" {
			manifest.Counts.Excluded[reason]++
			continue
		}
		records, err := buildSFTRecords(ep)
		if err != nil {
			return nil, fmt.Errorf("build records for %s: %w", ep.ID, err)
		}
		for _, rec := range records {
			data, err := json.Marshal(rec.record)
			if err != nil {
				return nil, fmt.Errorf("marshal record for %s turn %d: %w", ep.ID, rec.turn, err)
			}
			sum := sha256.Sum256(data)
			digest := hex.EncodeToString(sum[:])
			if seenRecords[digest] {
				manifest.Counts.Excluded["duplicate_record"]++
				continue
			}
			seenRecords[digest] = true
			lines = append(lines, data)
			includedEpisodes[ep.ID] = true
			manifest.IncludedRecords = append(manifest.IncludedRecords, SFTRecordRef{
				EpisodeID: ep.ID,
				TaskID:    ep.TaskID,
				Profile:   ep.Profile,
				Turn:      rec.turn,
				SHA256:    digest,
			})
		}
	}
	manifest.Counts.EpisodesIncluded = len(includedEpisodes)
	manifest.Counts.RecordsWritten = len(lines)
	sort.Strings(manifest.SourceEpisodes)
	sort.SliceStable(manifest.IncludedRecords, func(i, j int) bool {
		a, b := manifest.IncludedRecords[i], manifest.IncludedRecords[j]
		if a.EpisodeID != b.EpisodeID {
			return a.EpisodeID < b.EpisodeID
		}
		return a.Turn < b.Turn
	})

	if err := writeJSONL(opts.OutPath, lines); err != nil {
		return nil, err
	}
	if err := writeManifest(sftManifestPath(opts.OutPath), manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

type sftRecordBuild struct {
	turn   int
	record TRLRecord
}

func sftEpisodeExclusionReason(ep *Episode, rewardFloor float64) string {
	if ep.Invalid {
		if len(ep.ExclusionReasons) > 0 {
			return "invalid_episode:" + ep.ExclusionReasons[0]
		}
		return "invalid_episode"
	}
	if ep.Profile == ProfilePlain && invokesGhx(ep) {
		return "invalid_episode:COMPLIANCE: plain profile invoked ghx"
	}
	if len(ep.ExclusionReasons) > 0 {
		return "episode_exclusion:" + ep.ExclusionReasons[0]
	}
	if len(ep.Violations) > 0 || ep.Rewards.Safety < 1.0 {
		return "safety_violation"
	}
	if ep.Rewards.Overall < rewardFloor {
		return "reward_below_floor"
	}
	for _, a := range DetectAnomalies(ep) {
		if a.Kind == AnomalyAnswerDocContamination {
			return "contaminated:answer_doc_contamination"
		}
		if a.Severity == SeverityBreaking {
			return "breaking_anomaly:" + a.Kind
		}
	}
	return ""
}

func buildSFTRecords(ep *Episode) ([]sftRecordBuild, error) {
	var out []sftRecordBuild
	for turnIndex := range ep.Turns {
		record := TRLRecord{
			Messages: buildMessagesThroughTurn(ep, turnIndex),
			Tools:    buildToolsThroughTurn(ep, turnIndex),
		}
		out = append(out, sftRecordBuild{turn: ep.Turns[turnIndex].Turn, record: record})
	}
	return out, nil
}

func buildMessagesThroughTurn(ep *Episode, lastTurn int) []TRLMessage {
	var messages []TRLMessage
	if ep.Profile == ProfileSidecar {
		messages = append(messages, TRLMessage{Role: "system", Content: sidecar.BuildPersonaSystemPrompt()})
	}
	for i := 0; i <= lastTurn; i++ {
		turn := ep.Turns[i]
		messages = append(messages, TRLMessage{Role: "user", Content: sftUserContent(ep, turn)})
		if len(turn.ToolTraces) > 0 {
			var calls []TRLToolCall
			for _, trace := range turn.ToolTraces {
				name := sftToolName(trace)
				calls = append(calls, TRLToolCall{
					ID:   trace.ID,
					Type: "function",
					Function: TRLFunctionCall{
						Name:      name,
						Arguments: sftToolArguments(trace),
					},
				})
			}
			content := thinkingContent(turn.Thinking, "")
			messages = append(messages, TRLMessage{Role: "assistant", Content: content, ToolCalls: calls})
			for _, trace := range turn.ToolTraces {
				messages = append(messages, TRLMessage{
					Role:    "tool",
					Name:    sftToolName(trace),
					Content: trace.OutputExcerpt,
				})
			}
		}
		final := strings.TrimSpace(turn.Text)
		if final != "" {
			messages = append(messages, TRLMessage{Role: "assistant", Content: thinkingContent(turn.Thinking, final)})
		}
	}
	return messages
}

func buildToolsThroughTurn(ep *Episode, lastTurn int) []TRLTool {
	seen := map[string]bool{}
	var names []string
	for i := 0; i <= lastTurn; i++ {
		for _, trace := range ep.Turns[i].ToolTraces {
			name := sftToolName(trace)
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	tools := make([]TRLTool, 0, len(names))
	for _, name := range names {
		tools = append(tools, TRLTool{
			Type: "function",
			Function: TRLToolFunction{
				Name:        name,
				Description: sftToolDescription(name),
				Parameters: map[string]any{
					"type":                 "object",
					"additionalProperties": true,
					"properties":           map[string]any{},
				},
			},
		})
	}
	return tools
}

func sftUserContent(ep *Episode, turn TurnRecord) string {
	switch ep.Profile {
	case ProfilePlain, ProfileGhx:
		return directPrompt(ep.Profile, ep.Repo, turn.Question, turn.Turn)
	case ProfileSidecar:
		return sidecar.BuildPrompt(sidecar.Request{
			Session:  ep.ID,
			Repo:     ep.Repo,
			Question: turn.Question,
			Depth:    "normal",
		}, nil)
	default:
		return turn.Question
	}
}

func thinkingContent(thinking, content string) string {
	thinking = strings.TrimSpace(thinking)
	content = strings.TrimSpace(content)
	if thinking == "" {
		return content
	}
	if content == "" {
		return "<think>\n" + thinking + "\n</think>"
	}
	return "<think>\n" + thinking + "\n</think>\n\n" + content
}

func sftToolName(trace sidecar.ToolCallTrace) string {
	title := strings.TrimSpace(trace.Title)
	if strings.Contains(title, "submit_report") {
		return "submit_report"
	}
	if trace.Kind != "" && trace.Kind != "other" {
		return sanitizeToolName(trace.Kind)
	}
	return sanitizeToolName(title)
}

func sanitizeToolName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "tool"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(name) {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "tool"
	}
	if len(out) > 64 {
		out = strings.TrimRight(out[:64], "_")
	}
	return out
}

func sftToolArguments(trace sidecar.ToolCallTrace) any {
	if trace.RawInput == nil {
		return map[string]any{}
	}
	return trace.RawInput
}

func sftToolDescription(name string) string {
	switch name {
	case "execute":
		return "Run a read-only shell command for repository reconnaissance."
	case "read":
		return "Read a local file supplied by the agent runtime."
	case "submit_report":
		return "Submit the final structured ghx-sidecar evidence report."
	default:
		return "Agent runtime tool captured from the eval episode."
	}
}

func writeJSONL(path string, lines [][]byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir export dir: %w", err)
	}
	var sb strings.Builder
	for _, line := range lines {
		sb.Write(line)
		sb.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func writeManifest(path string, manifest *SFTExportManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func sftManifestPath(outPath string) string {
	return outPath + ".manifest.json"
}

// FormatSFTExportSummary renders a stable CLI summary for one export.
func FormatSFTExportSummary(m *SFTExportManifest) string {
	if m == nil {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "format: %s\n", m.Format)
	fmt.Fprintf(&sb, "run: %s\n", m.RunDir)
	fmt.Fprintf(&sb, "out: %s\n", m.OutPath)
	fmt.Fprintf(&sb, "manifest: %s\n", sftManifestPath(m.OutPath))
	fmt.Fprintf(&sb, "episodes: read=%d included=%d\n", m.Counts.EpisodesRead, m.Counts.EpisodesIncluded)
	fmt.Fprintf(&sb, "records: written=%d\n", m.Counts.RecordsWritten)
	if len(m.Counts.Excluded) > 0 {
		sb.WriteString("exclusions:\n")
		var reasons []string
		for reason := range m.Counts.Excluded {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			fmt.Fprintf(&sb, "  %s: %d\n", reason, m.Counts.Excluded[reason])
		}
	}
	return sb.String()
}
