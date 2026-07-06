package sidecar_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
)

// ADR-0020.2 wire coverage at the RunTurnWithOptions boundary, against the
// scripted mockagent (a real ACP peer over stdio): the full session-options
// meta rides on BOTH session/new and session/load, byte-identical, so resumed
// turns are steered exactly like fresh ones (TRUST H8).

// runTwoTurnSession runs one NewSession turn and one LoadSession resume turn
// with the given session meta, returning the mockagent's logged NEW_META and
// LOAD_META JSON payloads.
func runTwoTurnSession(t *testing.T, meta map[string]any) (newMeta, loadMeta string) {
	t.Helper()
	bin := buildScriptedAgent(t, []map[string]any{
		{"text": "turn one"},
		{"text": "turn two"},
	})
	promptLog := filepath.Join(t.TempDir(), "prompts.log")
	t.Setenv("MOCKAGENT_PROMPT_LOG", promptLog)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, sessionID, err := sidecar.RunTurnWithOptions(ctx, sidecar.RunTurnOptions{
		AgentCmd:        bin,
		Prompt:          "q1",
		SessionMeta:     meta,
		LivenessTimeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("turn 1 (NewSession): %v", err)
	}
	if sessionID == "" {
		t.Fatal("no session id to resume")
	}
	// Turn 2 spawns a FRESH mockagent process (the production per-turn
	// pattern) and resumes via LoadSession — the exact H8 condition.
	if _, _, err := sidecar.RunTurnWithOptions(ctx, sidecar.RunTurnOptions{
		AgentCmd:        bin,
		ACPSessionID:    sessionID,
		Prompt:          "q2",
		SessionMeta:     meta,
		LivenessTimeout: time.Minute,
	}); err != nil {
		t.Fatalf("turn 2 (LoadSession): %v", err)
	}

	logData, err := os.ReadFile(promptLog)
	if err != nil {
		t.Fatalf("read prompt log: %v", err)
	}
	for _, line := range strings.Split(string(logData), "\n") {
		if rest, ok := strings.CutPrefix(line, "NEW_META "); ok {
			newMeta = rest
		}
		if rest, ok := strings.CutPrefix(line, "LOAD_META "); ok {
			loadMeta = rest
		}
	}
	if newMeta == "" || loadMeta == "" {
		t.Fatalf("prompt log missing NEW_META/LOAD_META lines:\n%s", logData)
	}
	return newMeta, loadMeta
}

// metaOptions digs claudeCode.options out of a logged meta payload.
func metaOptions(t *testing.T, metaJSON string) (claudeCode, options map[string]any) {
	t.Helper()
	var meta map[string]any
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		t.Fatalf("unmarshal meta %q: %v", metaJSON, err)
	}
	claudeCode, ok := meta["claudeCode"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing claudeCode bag: %s", metaJSON)
	}
	options, ok = claudeCode["options"].(map[string]any)
	if !ok {
		t.Fatalf("meta missing claudeCode.options: %s", metaJSON)
	}
	return claudeCode, options
}

// TestLoadSessionCarriesFullSteeringMeta pins ADR-0020.2 D1 in eval mode: the
// resumed turn's LoadSession receives the identical meta NewSession sent —
// persona, tools allowlist, budgets, model pin, isolation, and the raw-SDK
// audit flag — not the raw-channel-only subset ADR-0016.10 D2 used to send.
func TestLoadSessionCarriesFullSteeringMeta(t *testing.T) {
	const persona = "test persona doctrine"
	newMeta, loadMeta := runTwoTurnSession(t,
		sidecar.BuildSessionMeta(persona, "normal", "claude-test-model", true, nil))

	// One meta map, both concerns: NewSession and LoadSession wire payloads
	// must be byte-identical (mockagent re-marshals with sorted keys).
	if newMeta != loadMeta {
		t.Fatalf("LoadSession meta diverges from NewSession meta:\nnew:  %s\nload: %s", newMeta, loadMeta)
	}

	claudeCode, options := metaOptions(t, loadMeta)
	if claudeCode["emitRawSDKMessages"] != true {
		t.Error("resumed turn lost the eval raw-SDK audit flag (ADR-0016.10 D2)")
	}
	if options["systemPrompt"] != persona {
		t.Errorf("systemPrompt = %v, want the persona", options["systemPrompt"])
	}
	if tools, _ := options["tools"].([]any); len(tools) != 2 || tools[0] != "Bash" || tools[1] != "Read" {
		t.Errorf("tools allowlist = %v, want [Bash Read]", options["tools"])
	}
	if allowed, _ := options["allowedTools"].([]any); len(allowed) != 1 || allowed[0] != sidecar.SubmitReportToolID {
		t.Errorf("allowedTools = %v, want [%s]", options["allowedTools"], sidecar.SubmitReportToolID)
	}
	if options["maxTurns"] != float64(24) {
		t.Errorf("maxTurns = %v, want 24 (normal depth)", options["maxTurns"])
	}
	thinking, _ := options["thinking"].(map[string]any)
	if thinking["type"] != "enabled" || thinking["budgetTokens"] != float64(2048) || thinking["display"] != "summarized" {
		t.Errorf("thinking = %v, want enabled/2048/summarized", options["thinking"])
	}
	if options["effort"] != "medium" {
		t.Errorf("effort = %v, want medium", options["effort"])
	}
	if options["model"] != "claude-test-model" {
		t.Errorf("model pin = %v, want claude-test-model", options["model"])
	}
	if sources, ok := options["settingSources"].([]any); !ok || len(sources) != 0 {
		t.Errorf("settingSources = %v, want explicit [] (isolation)", options["settingSources"])
	}
	if options["strictMcpConfig"] != true {
		t.Error("strictMcpConfig lost on resume (isolation)")
	}
}

// TestLoadSessionCarriesSteeringMetaInProduction pins the production half of
// TRUST H8: outside eval mode there is no audit flag, but a follow-up ask's
// LoadSession still re-asserts the full steering options.
func TestLoadSessionCarriesSteeringMetaInProduction(t *testing.T) {
	newMeta, loadMeta := runTwoTurnSession(t,
		sidecar.BuildSessionMeta("prod persona", "normal", "", false, nil))

	if newMeta != loadMeta {
		t.Fatalf("production LoadSession meta diverges from NewSession meta:\nnew:  %s\nload: %s", newMeta, loadMeta)
	}
	claudeCode, options := metaOptions(t, loadMeta)
	if _, present := claudeCode["emitRawSDKMessages"]; present {
		t.Error("production meta must not enable the raw-SDK audit channel")
	}
	if options["systemPrompt"] != "prod persona" {
		t.Errorf("systemPrompt = %v, want the persona on the resumed production turn", options["systemPrompt"])
	}
}
