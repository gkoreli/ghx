package sidecar

// SessionOptions carries the per-session steering options forwarded to the
// ACP adapter via the _meta.claudeCode.options bag (ADR-0020.1 D2).
//
// The adapter (claude-agent-acp@0.55.0) spreads the entire options object
// over its adapter defaults (acp-agent.js:2768–2801). Options set here
// override the adapter's defaults for the lifetime of the session.
//
// This type is ADAPTER-SPECIFIC: it targets the claude-agent-acp adapter.
// See ADR-0020.1 §D2 — accepted cost.
type SessionOptions struct {
	// SystemPrompt is placed as the custom system prompt for the session —
	// equivalent to the stable persona/doctrine that was previously embedded
	// in the per-turn user prompt by BuildPrompt. Putting it here makes it
	// cache-stable and removes it from the user-message token stream.
	//
	// Source: acp-agent.js:2748 (_meta.systemPrompt → system prompt string).
	SystemPrompt string `json:"systemPrompt,omitempty"`

	// SettingSources controls which host-filesystem setting files the adapter
	// loads. Empty slice ([]) disables host settings inheritance entirely —
	// the adapter default is ["user","project","local"], which caused the
	// ADR-0016.3 isolation gap where subject agents inherited CLAUDE.md/AGENTS.md
	// from the host. We always set this to [] for sidecar sessions.
	//
	// Source: acp-agent.js:2800 (settingSources default).
	SettingSources []string `json:"settingSources"`

	// StrictMcpConfig: when true the adapter does not fall back to the host's
	// MCP server configuration. Combined with SettingSources:[] this closes
	// the isolation gap at the source instead of at the runner level.
	//
	// Source: ADR-0020 addendum / acp-agent.js options spread.
	StrictMcpConfig bool `json:"strictMcpConfig"`

	// Tools is an allowlist of Claude Code built-in tool names available to the
	// session. An empty allowlist is different from omitting the field:
	// omitting means "all tools"; [] means "no built-in tools". We set a
	// conservative list: shell execution (Bash) and file reads (Read). No
	// editor or write tools. The denyClient provides a second safety layer.
	//
	// IMPORTANT: allowedTools is auto-approve, NOT an allowlist — do NOT use
	// it here (ADR-0020.1 §Considered and rejected).
	//
	// Source: ADR-0020 addendum + claude-agent-sdk AgentDefinition.tools.
	Tools []string `json:"tools,omitempty"`

	// MaxTurns caps the number of agentic turns the adapter will execute.
	// Maps to depth budget (see DepthBudget). Nil means adapter default.
	MaxTurns *int `json:"maxTurns,omitempty"`

	// Thinking configures the extended thinking budget. Nil means adapter
	// default; 0 disables thinking; >0 enables it with that token budget.
	// Maps to depth budget (see DepthBudget).
	Thinking *int `json:"thinking,omitempty"`

	// Effort is the model's internal effort/compute level. Accepted values
	// from the adapter: "low", "normal", "high". Maps to depth budget.
	Effort string `json:"effort,omitempty"`

	// Model pins the subject model for this session. Empty means adapter
	// default. Set from Config.Model or GHX_EVAL_SUBJECT_MODEL so the
	// wrong-model failure mode is structurally impossible (ADR-0020.1 D4).
	Model string `json:"model,omitempty"`

	// EmitRawSDKMessages enables the _claude/sdkMessage notification firehose
	// (adapter-specific audit channel). Set true only when running under evals;
	// off by default in production (ADR-0020.1 D5).
	EmitRawSDKMessages bool `json:"emitRawSDKMessages,omitempty"`
}

// depthBudget is one row of the depth→budget mapping table (ADR-0020.1 D3).
//
// Mapping rationale:
//
//   - cheap: minimal footprint, no extended thinking. The agent does a quick
//     surface scan. MaxTurns=4 matches "budget: max 4 ghx commands" (half the
//     normal 8). Effort="low" minimises cost. Thinking disabled (budget=0) to
//     keep latency low.
//
//   - normal (default): the current sidecar contract of ~8 commands. MaxTurns=8
//     matches the existing honor-system budget. Effort="normal". Thinking
//     enabled at 2048 tokens — enough for a reasoning step without ballooning
//     cost; keeps the existing adapter default behaviour roughly intact.
//
//   - deep: extended investigation. MaxTurns=16 doubles the command budget
//     for follow-up turns. Effort="high" unlocks the model's extended compute.
//     Thinking=4096 for thorough reasoning over large codebases.
//
// These are conservative initial values. Adjust after the confirmatory re-run
// required by ADR-0020.1's measurement rule.
type depthBudget struct {
	maxTurns *int
	thinking *int
	effort   string
}

func intPtr(n int) *int { return &n }

var depthBudgets = map[string]depthBudget{
	// cheap: 4 turns, no thinking, low effort
	"cheap": {maxTurns: intPtr(4), thinking: intPtr(0), effort: "low"},
	// normal: 8 turns, 2048-token thinking, normal effort (default)
	"normal": {maxTurns: intPtr(8), thinking: intPtr(2048), effort: "normal"},
	// deep: 16 turns, 4096-token thinking, high effort
	"deep": {maxTurns: intPtr(16), thinking: intPtr(4096), effort: "high"},
}

// sidecarToolsAllowlist is the conservative tool allowlist for the sidecar
// persona. Shell execution (Bash) is required so the agent can run ghx.
// Read allows the adapter to use the Read tool for any diagnostic reads.
// No editor, write, or web tools — the sidecar is a reconnaissance agent.
//
// Note: Claude Code tool names use PascalCase as shown in Claude Code docs.
var sidecarToolsAllowlist = []string{"Bash", "Read"}

// BuildSessionMeta constructs the _meta map for acp.NewSessionRequest.
//
// The returned map is suitable for direct assignment to
// acp.NewSessionRequest.Meta. It encodes the full D2 steering surface:
// persona as system prompt, isolation settings, mechanical budgets,
// model pinning, and audit channel.
//
// Parameters:
//   - persona: the stable system prompt string (from BuildPersonaSystemPrompt).
//   - depth: "cheap", "normal", or "deep". Unknown values default to "normal".
//   - model: subject model string (empty = adapter default).
//   - emitRaw: true only when running under evals.
func BuildSessionMeta(persona, depth, model string, emitRaw bool) map[string]any {
	budget, ok := depthBudgets[depth]
	if !ok {
		budget = depthBudgets["normal"]
	}

	opts := SessionOptions{
		SystemPrompt:    persona,
		SettingSources:  []string{}, // explicitly [] — disables host settings inheritance
		StrictMcpConfig: true,
		Tools:           sidecarToolsAllowlist,
		MaxTurns:        budget.maxTurns,
		Thinking:        budget.thinking,
		Effort:          budget.effort,
		Model:           model,
	}
	if emitRaw {
		opts.EmitRawSDKMessages = true
	}

	return map[string]any{
		"claudeCode": map[string]any{
			"options": opts,
		},
	}
}
