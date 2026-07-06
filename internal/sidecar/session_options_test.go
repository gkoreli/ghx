package sidecar

import (
	"encoding/json"
	"testing"
)

// TestBuildSessionMetaShape verifies the exact _meta JSON shape sent at
// session creation (ADR-0020.1 D2 unit-test requirement).
//
// The shape must be:
//
//	{
//	  "claudeCode": {
//	    "options": {
//	      "systemPrompt":    "<persona>",
//	      "settingSources":  [],
//	      "strictMcpConfig": true,
//	      "tools":           ["Bash","Read"],
//	      "maxTurns":        <depth-dependent>,
//	      "thinking":        {"type":"enabled","budgetTokens":N} | {"type":"disabled"},
//	      "effort":          "low" | "medium" | "high",
//	      "model":           "<model or empty>"
//	    },
//	    "emitRawSDKMessages": <eval-only, sibling of options>
//	  }
//	}
func TestBuildSessionMetaShape(t *testing.T) {
	persona := BuildPersonaSystemPrompt()
	meta := BuildSessionMeta(persona, "normal", "claude-test-model", false, nil)

	// Marshal to JSON so we can inspect the exact wire shape.
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Logf("_meta JSON: %s", data)

	// Unmarshal back to map for field-level assertions.
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	ccAny, ok := m["claudeCode"]
	if !ok {
		t.Fatal("_meta missing 'claudeCode' key")
	}
	cc, ok := ccAny.(map[string]any)
	if !ok {
		t.Fatalf("claudeCode must be object, got %T", ccAny)
	}
	optsAny, ok := cc["options"]
	if !ok {
		t.Fatal("claudeCode missing 'options' key")
	}
	opts, ok := optsAny.(map[string]any)
	if !ok {
		t.Fatalf("options must be object, got %T", optsAny)
	}

	// D1: persona present as systemPrompt
	sp, _ := opts["systemPrompt"].(string)
	if sp == "" {
		t.Error("options.systemPrompt must be non-empty (persona)")
	}
	if sp != persona {
		t.Error("options.systemPrompt must equal BuildPersonaSystemPrompt() output")
	}

	// D2: settingSources: [] (empty array, NOT omitted)
	ssAny, ssPresent := opts["settingSources"]
	if !ssPresent {
		t.Error("options.settingSources must be present (even when empty)")
	}
	ss, ok := ssAny.([]any)
	if !ok {
		t.Fatalf("options.settingSources must be array, got %T", ssAny)
	}
	if len(ss) != 0 {
		t.Errorf("options.settingSources must be empty array [], got %v", ss)
	}

	// D2: strictMcpConfig: true
	if smc, _ := opts["strictMcpConfig"].(bool); !smc {
		t.Error("options.strictMcpConfig must be true")
	}

	// D2: tools allowlist
	toolsAny, ok := opts["tools"]
	if !ok {
		t.Error("options.tools must be present")
	}
	toolsArr, ok := toolsAny.([]any)
	if !ok {
		t.Fatalf("options.tools must be array, got %T", toolsAny)
	}
	wantTools := map[string]bool{"Bash": true, "Read": true}
	for _, ta := range toolsArr {
		s, _ := ta.(string)
		if !wantTools[s] {
			t.Errorf("unexpected tool in allowlist: %q", s)
		}
		delete(wantTools, s)
	}
	for missing := range wantTools {
		t.Errorf("tools allowlist missing %q", missing)
	}

	// D3: mechanical budget fields present for "normal" depth. Thinking is
	// the SDK ThinkingConfig object shape, never a bare number.
	if mt, _ := opts["maxTurns"].(float64); mt != 24 {
		t.Errorf("options.maxTurns = %v for depth=normal, want 24", mt)
	}
	th, ok := opts["thinking"].(map[string]any)
	if !ok {
		t.Fatalf("options.thinking must be ThinkingConfig object, got %T", opts["thinking"])
	}
	if th["type"] != "enabled" || th["budgetTokens"] != float64(2048) {
		t.Errorf("options.thinking = %v for depth=normal, want {enabled, 2048}", th)
	}
	if th["display"] != "summarized" {
		t.Errorf("options.thinking.display = %v, want summarized — the API-default 'omitted' streams empty thinking text the adapter drops", th["display"])
	}
	if eff, _ := opts["effort"].(string); eff != "medium" {
		t.Errorf("options.effort = %q for depth=normal, want medium (SDK enum has no 'normal')", eff)
	}

	// D4: model pin present
	if mdl, _ := opts["model"].(string); mdl != "claude-test-model" {
		t.Errorf("options.model = %q, want claude-test-model", mdl)
	}

	// D5: emitRawSDKMessages lives at claudeCode level (sibling of options,
	// per the adapter's read path) and must be absent in production mode.
	if _, present := cc["emitRawSDKMessages"]; present {
		t.Error("claudeCode.emitRawSDKMessages must be absent in production mode (emitRaw=false)")
	}
	if _, present := opts["emitRawSDKMessages"]; present {
		t.Error("emitRawSDKMessages must never be inside options — the adapter reads claudeCode.emitRawSDKMessages")
	}
}

// TestBuildSessionMetaEvalMode verifies emitRawSDKMessages is set true only
// in eval mode (ADR-0020.1 D5).
func TestBuildSessionMetaEvalMode(t *testing.T) {
	meta := BuildSessionMeta("persona", "normal", "", true, nil)
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cc := m["claudeCode"].(map[string]any)
	if emit, _ := cc["emitRawSDKMessages"].(bool); !emit {
		t.Error("claudeCode.emitRawSDKMessages must be true in eval mode")
	}
	opts := cc["options"].(map[string]any)
	if _, present := opts["emitRawSDKMessages"]; present {
		t.Error("emitRawSDKMessages must never be inside options — the adapter reads claudeCode.emitRawSDKMessages")
	}
}

// TestBuildSessionMetaDepthBudgets verifies the depth→budget mapping table
// (ADR-0020.1 D3).
func TestBuildSessionMetaDepthBudgets(t *testing.T) {
	cases := []struct {
		depth        string
		maxTurns     float64
		thinkingType string
		budgetTokens float64
		effort       string
	}{
		// cheap: 4 turns, thinking disabled, low effort
		{"cheap", 12, "disabled", 0, "low"},
		// normal: 8 turns, 2048-token thinking, medium effort
		{"normal", 24, "enabled", 2048, "medium"},
		// deep: 16 turns, 4096-token thinking, high effort
		{"deep", 48, "enabled", 4096, "high"},
		// unknown depth defaults to normal
		{"", 24, "enabled", 2048, "medium"},
		{"bogus", 24, "enabled", 2048, "medium"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.depth, func(t *testing.T) {
			meta := BuildSessionMeta("p", tc.depth, "", false, nil)
			data, err := json.Marshal(meta)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			opts := m["claudeCode"].(map[string]any)["options"].(map[string]any)
			if mt, _ := opts["maxTurns"].(float64); mt != tc.maxTurns {
				t.Errorf("depth=%q maxTurns=%v, want %v", tc.depth, mt, tc.maxTurns)
			}
			th, ok := opts["thinking"].(map[string]any)
			if !ok {
				t.Fatalf("depth=%q thinking must be ThinkingConfig object, got %T", tc.depth, opts["thinking"])
			}
			if th["type"] != tc.thinkingType {
				t.Errorf("depth=%q thinking.type=%v, want %q", tc.depth, th["type"], tc.thinkingType)
			}
			bt, _ := th["budgetTokens"].(float64)
			if bt != tc.budgetTokens {
				t.Errorf("depth=%q thinking.budgetTokens=%v, want %v", tc.depth, bt, tc.budgetTokens)
			}
			if eff, _ := opts["effort"].(string); eff != tc.effort {
				t.Errorf("depth=%q effort=%q, want %q", tc.depth, eff, tc.effort)
			}
		})
	}
}

// TestBuildSessionMetaModelEmpty verifies that when no model is provided,
// the model field is omitted (adapter picks its default).
func TestBuildSessionMetaModelEmpty(t *testing.T) {
	meta := BuildSessionMeta("persona", "normal", "", false, nil)
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	opts := m["claudeCode"].(map[string]any)["options"].(map[string]any)
	if _, present := opts["model"]; present {
		t.Error("options.model must be absent when empty (adapter picks default)")
	}
}

// TestBuildSessionMetaSettingSourcesExplicitEmpty verifies that settingSources
// is always serialized as an explicit empty array [] even when empty, not
// omitted. This is required to override the adapter's default ["user","project","local"].
func TestBuildSessionMetaSettingSourcesExplicitEmpty(t *testing.T) {
	meta := BuildSessionMeta("p", "normal", "", false, nil)
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Check raw JSON for the key presence — if omitempty were set this would fail.
	jsonStr := string(data)
	if !containsKey(jsonStr, `"settingSources":[]`) && !containsKey(jsonStr, `"settingSources": []`) {
		t.Errorf("settingSources must be explicit [] in JSON, got: %s", jsonStr)
	}
}

func containsKey(s, substr string) bool {
	return len(s) >= len(substr) && indexOfString(s, substr) >= 0
}

func indexOfString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// TestBuildSessionMetaSettingSourcesOptIn verifies the ADR-0033.2 enterprise
// escape hatch: a non-nil Config.AgentSettingSources is forwarded verbatim,
// so a toolbox Claude install can load ~/.claude/settings.json (Bedrock
// modelOverrides resolution) while the default stays full isolation.
func TestBuildSessionMetaSettingSourcesOptIn(t *testing.T) {
	meta := BuildSessionMeta("p", "normal", "", false, []string{"user"})
	opts := meta["claudeCode"].(map[string]any)["options"].(SessionOptions)
	if len(opts.SettingSources) != 1 || opts.SettingSources[0] != "user" {
		t.Fatalf("settingSources = %v, want [user]", opts.SettingSources)
	}
}
