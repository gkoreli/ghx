package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gkoreli/ghx/v2/internal/sidecar"
	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
	"github.com/mark3labs/mcp-go/mcp"
)

func readConfigAgent(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "config.json"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(data)
}

func TestConfigInitClaudeACPWritesFreshConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := runSidecarConfigInit(true, false, "", false); err != nil {
		t.Fatalf("fresh init --claude-acp failed: %v", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != sidecar.ClaudeACPAgentCmd {
		t.Fatalf("agent = %q, want %q", got, sidecar.ClaudeACPAgentCmd)
	}
	if !strings.Contains(readConfigAgent(t, home), "@agentclientprotocol/claude-agent-acp") {
		t.Fatal("config.json does not contain the adapter command")
	}
}

func TestConfigInitClaudeACPRefusesExistingConfigWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := sidecar.SaveConfig(sidecar.Config{AgentCmd: "claude", SessionsDir: filepath.Join(home, "sessions")}); err != nil {
		t.Fatal(err)
	}
	err := runSidecarConfigInit(true, false, "", false)
	if err == nil {
		t.Fatal("init --claude-acp overwrote an existing config without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %q, want --force hint", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != "claude" {
		t.Fatalf("agent = %q after refusal, want unchanged %q", got, "claude")
	}
}

func TestConfigInitClaudeACPForceOverwritesAndKeepsModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := sidecar.SaveConfig(sidecar.Config{
		AgentCmd:    "claude",
		SessionsDir: filepath.Join(home, "sessions"),
		Model:       "claude-sonnet-4-5",
	}); err != nil {
		t.Fatal(err)
	}
	if err := runSidecarConfigInit(true, false, "", true); err != nil {
		t.Fatalf("init --claude-acp --force failed: %v", err)
	}
	cfg := sidecar.LoadConfig()
	if cfg.AgentCmd != sidecar.ClaudeACPAgentCmd {
		t.Fatalf("agent = %q, want %q", cfg.AgentCmd, sidecar.ClaudeACPAgentCmd)
	}
	if cfg.Model != "claude-sonnet-4-5" {
		t.Fatalf("model = %q, want carried over", cfg.Model)
	}
}

func TestConfigInitClaudeACPIdempotentWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	if err := runSidecarConfigInit(true, false, "", false); err != nil {
		t.Fatalf("first init failed: %v", err)
	}
	// Re-running on a config that already matches changes nothing and needs no --force.
	if err := runSidecarConfigInit(true, false, "", false); err != nil {
		t.Fatalf("re-run on matching config failed: %v", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != sidecar.ClaudeACPAgentCmd {
		t.Fatalf("agent = %q, want %q", got, sidecar.ClaudeACPAgentCmd)
	}
}

// writeFakeClaude creates an executable fake `claude` binary in dir and
// returns its path.
func writeFakeClaude(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "claude")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigInitClaudeExeWritesEnvPrefixedAdapterCmd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)
	exe := writeFakeClaude(t, t.TempDir())

	if err := runSidecarConfigInit(false, true, exe, false); err != nil {
		t.Fatalf("init --claude-exe %s failed: %v", exe, err)
	}
	want := "env CLAUDE_CODE_EXECUTABLE=" + exe + " " + sidecar.ClaudeACPAgentCmd
	if got := sidecar.LoadConfig().AgentCmd; got != want {
		t.Fatalf("agent = %q, want %q", got, want)
	}
	if !strings.Contains(readConfigAgent(t, home), "CLAUDE_CODE_EXECUTABLE") {
		t.Fatal("config.json does not carry CLAUDE_CODE_EXECUTABLE")
	}
}

func TestConfigInitClaudeExeAutoResolvesFromPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)
	bin := t.TempDir()
	exe := writeFakeClaude(t, bin)
	t.Setenv("PATH", bin)

	// Both spellings of auto mode: explicit "auto" and an explicit empty value.
	for _, val := range []string{"auto", ""} {
		if err := runSidecarConfigInit(false, true, val, true); err != nil {
			t.Fatalf("init --claude-exe %q failed: %v", val, err)
		}
		want := "env CLAUDE_CODE_EXECUTABLE=" + exe + " " + sidecar.ClaudeACPAgentCmd
		if got := sidecar.LoadConfig().AgentCmd; got != want {
			t.Fatalf("--claude-exe %q: agent = %q, want %q", val, got, want)
		}
	}
}

func TestConfigInitClaudeExeAutoErrorsWithoutClaudeOnPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)
	t.Setenv("PATH", t.TempDir()) // empty dir: no claude anywhere

	err := runSidecarConfigInit(false, true, "auto", false)
	if err == nil {
		t.Fatal("init --claude-exe auto succeeded with no claude on PATH")
	}
	if !strings.Contains(err.Error(), "no claude on PATH") {
		t.Fatalf("error = %q, want 'no claude on PATH' hint", err)
	}
	if _, exists := sidecar.ConfigFileExists(); exists {
		t.Fatal("config was written despite the resolution error")
	}
}

func TestConfigInitClaudeExeRejectsNonexistentPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	missing := filepath.Join(t.TempDir(), "nope", "claude")
	err := runSidecarConfigInit(false, true, missing, false)
	if err == nil {
		t.Fatal("init --claude-exe accepted a nonexistent path")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %q, want a not-found message", err)
	}
}

func TestConfigInitClaudeExeRejectsNonExecutablePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable-bit check is skipped on windows")
	}
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	path := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(path, []byte("not a binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runSidecarConfigInit(false, true, path, false)
	if err == nil {
		t.Fatal("init --claude-exe accepted a non-executable file")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Fatalf("error = %q, want 'not executable' message", err)
	}
}

func TestConfigInitClaudeExeExclusiveWithClaudeACP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)

	err := runSidecarConfigInit(true, true, "/usr/local/bin/claude", false)
	if err == nil {
		t.Fatal("init accepted both --claude-acp and --claude-exe")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %q, want mutual-exclusion message", err)
	}
}

func TestConfigInitClaudeExeRefusesExistingConfigWithoutForce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("GHX_HOME", home)
	exe := writeFakeClaude(t, t.TempDir())

	if err := sidecar.SaveConfig(sidecar.Config{AgentCmd: "codex", SessionsDir: filepath.Join(home, "sessions")}); err != nil {
		t.Fatal(err)
	}
	err := runSidecarConfigInit(false, true, exe, false)
	if err == nil {
		t.Fatal("init --claude-exe overwrote an existing config without --force")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %q, want --force hint", err)
	}
	if got := sidecar.LoadConfig().AgentCmd; got != "codex" {
		t.Fatalf("agent = %q after refusal, want unchanged %q", got, "codex")
	}
	if err := runSidecarConfigInit(false, true, exe, true); err != nil {
		t.Fatalf("init --claude-exe --force failed: %v", err)
	}
	want := "env CLAUDE_CODE_EXECUTABLE=" + exe + " " + sidecar.ClaudeACPAgentCmd
	if got := sidecar.LoadConfig().AgentCmd; got != want {
		t.Fatalf("agent = %q, want %q", got, want)
	}
}

// questionSession derivation (ADR-0019.1 D2): kebab-cased leading words,
// ~40-char truncation, short content hash for uniqueness, stable across calls.

func TestQuestionSessionStable(t *testing.T) {
	q := "Which Go libraries provide OpenTelemetry OTLP file exporters?"
	if a, b := questionSession(q), questionSession(q); a != b {
		t.Fatalf("questionSession is not stable: %q vs %q", a, b)
	}
}

func TestQuestionSessionShape(t *testing.T) {
	s := questionSession("Which Go libraries provide OpenTelemetry OTLP file exporters?")
	re := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*-[0-9a-f]{8}$`)
	if !re.MatchString(s) {
		t.Fatalf("questionSession shape = %q, want kebab slug + 8-hex hash", s)
	}
	if !strings.HasPrefix(s, "which-go-libraries-provide") {
		t.Fatalf("questionSession = %q, want leading words of the question", s)
	}
}

func TestQuestionSessionTruncatesLongQuestions(t *testing.T) {
	q := "How does the middleware chaining implementation compose handlers across nested routers and groups?"
	s := questionSession(q)
	// Strip the "-<8 hex>" suffix; the slug part must respect the ~40-char cap.
	slug := s[:len(s)-9]
	if n := len([]rune(slug)); n > 40 {
		t.Fatalf("slug part %q has %d chars, want <= 40", slug, n)
	}
	if strings.HasSuffix(slug, "-") {
		t.Fatalf("slug part %q must not end with a dash", slug)
	}
}

func TestQuestionSessionUniqueOnDifferentQuestions(t *testing.T) {
	// Same leading words (identical slug part), different tails: the content
	// hash must keep the session names distinct.
	a := questionSession("which repos implement agent sidecar frameworks for code reconnaissance in Go")
	b := questionSession("which repos implement agent sidecar frameworks for code reconnaissance in Rust")
	if a == b {
		t.Fatalf("different questions produced the same session name %q", a)
	}
}

func TestQuestionSessionEmptyFallsBackToDiscovery(t *testing.T) {
	s := questionSession("???")
	if !strings.HasPrefix(s, "discovery-") {
		t.Fatalf("questionSession(%q) = %q, want discovery-<hash> fallback", "???", s)
	}
}

// handleRecon passes the caller's parameters through untouched (ADR-0030.1
// D4 phase 1): session routing lives in the daemon, so the tool no longer
// does any local session defaulting. Explicit session and repo still travel
// verbatim (R1/R2 preserve the ADR-0019.1 precedence inside the runtime).
func TestHandleReconSessionDefaults(t *testing.T) {
	orig := askSidecar
	defer func() { askSidecar = orig }()
	var got sidecar.AskRequest
	askSidecar = func(_ context.Context, _ sidecar.Config, req sidecar.AskRequest) (*sidecar.Report, *sidecar.TurnResult, error) {
		got = req
		return &sidecar.Report{Answer: "ok"}, nil, nil
	}

	// Discovery: no repo argument at all — the daemon routes (R3-R5), so the
	// tool must NOT pre-fill a question-derived session.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "which repos do X"}
	res, err := handleRecon(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("recon without repo must not error: %+v", res)
	}
	if got.Repo != "" {
		t.Fatalf("discovery ask must pass empty repo, got %q", got.Repo)
	}
	if got.Session != "" {
		t.Fatalf("discovery ask must leave session empty for daemon routing, got %q", got.Session)
	}

	// Repo-scoped: repo travels verbatim; the daemon's R2 resolves the same
	// repo-slug session as the old local defaulting did.
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q", "repo": "OwnerX/RepoY"}
	if _, err := handleRecon(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got.Repo != "OwnerX/RepoY" {
		t.Fatalf("repo not passed through, got %q", got.Repo)
	}
	if got.Session != "" {
		t.Fatalf("repo-scoped ask must leave session empty for daemon routing, got %q", got.Session)
	}

	// Explicit session always wins (R1) and travels verbatim.
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q", "session": "my-thread"}
	if _, err := handleRecon(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got.Session != "my-thread" {
		t.Fatalf("explicit session = %q, want %q", got.Session, "my-thread")
	}
}

// TestHandleReconDepthForwarding pins the depth dial (NORTH_STAR capability
// 3): the recon tool forwards the caller's depth through AskRequest.Depth —
// the same field `ask --depth` uses — omitting it keeps the "normal" smart
// default, and an invalid value is rejected with the valid set named rather
// than silently coerced.
func TestHandleReconDepthForwarding(t *testing.T) {
	orig := askSidecar
	defer func() { askSidecar = orig }()
	var got sidecar.AskRequest
	askSidecar = func(_ context.Context, _ sidecar.Config, req sidecar.AskRequest) (*sidecar.Report, *sidecar.TurnResult, error) {
		got = req
		return &sidecar.Report{Answer: "ok"}, nil, nil
	}

	// Each valid depth travels verbatim into AskRequest.Depth.
	for _, depth := range []string{"cheap", "normal", "deep"} {
		got = sidecar.AskRequest{}
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"question": "q", "depth": depth}
		res, err := handleRecon(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("valid depth %q must not error: %+v", depth, res)
		}
		if got.Depth != depth {
			t.Fatalf("depth %q not forwarded, got %q", depth, got.Depth)
		}
	}

	// Omitting depth keeps today's smart default ("normal") — behavior is
	// unchanged for callers that never set the dial.
	got = sidecar.AskRequest{}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q"}
	if _, err := handleRecon(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if got.Depth != "normal" {
		t.Fatalf("omitted depth must default to normal, got %q", got.Depth)
	}

	// An invalid value is rejected (not coerced): the tool returns an error
	// naming the valid options and never reaches the sidecar.
	got = sidecar.AskRequest{Depth: "sentinel"}
	req = mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q", "depth": "turbo"}
	res, err := handleRecon(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("invalid depth must return a tool error, got %+v", res)
	}
	if got.Depth != "sentinel" {
		t.Fatalf("invalid depth must not reach the sidecar; askSidecar was called with %q", got.Depth)
	}
	errText, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("error content[0] is %T, want TextContent", res.Content[0])
	}
	for _, want := range []string{"turbo", "cheap", "normal", "deep"} {
		if !strings.Contains(errText.Text, want) {
			t.Fatalf("invalid-depth error must name %q, got: %s", want, errText.Text)
		}
	}
}

// TestAskDepthValidationRejectsBogus pins that `ghx sidecar ask --depth <bogus>`
// is rejected with exit 2 naming the valid set, rather than silently running at
// an unintended budget (dogfood friction, FRICTION.md 2026-07-07 "--depth bogus
// is silently accepted"). Validation runs before any daemon call, so a bogus
// value never reaches the network. This matches the MCP recon path, which
// already rejects an out-of-set depth (serve.go / TestHandleReconDepthForwarding).
func TestAskDepthValidationRejectsBogus(t *testing.T) {
	t.Cleanup(func() { _ = sidecarAskCmd.Flags().Set("depth", "normal") })

	if err := sidecarAskCmd.Flags().Set("depth", "bogus"); err != nil {
		t.Fatal(err)
	}
	err := sidecarAskCmd.RunE(sidecarAskCmd, []string{"How is Get implemented?"})
	if err == nil {
		t.Fatal("bogus --depth returned nil error (silently accepted)")
	}
	if got := CodeForError(err); got != ExitBadInvocation {
		t.Fatalf("exit code = %d, want %d (ExitBadInvocation)", got, ExitBadInvocation)
	}
	for _, want := range []string{"bogus", "cheap", "normal", "deep"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("invalid --depth error must name %q, got: %s", want, err.Error())
		}
	}
}

// TestAskDepthValidationAcceptsValid pins that the three valid depths pass
// validation (they proceed past the depth check without a bad-invocation
// error), preserving today's valid-value behavior.
func TestAskDepthValidationAcceptsValid(t *testing.T) {
	for _, depth := range []string{"cheap", "normal", "deep"} {
		if _, ok := sidecar.ParseDepth(depth); !ok {
			t.Fatalf("valid depth %q rejected by validator the ask command reuses", depth)
		}
	}
}

// TestAskEnvelopeJSONShape pins the `ghx sidecar ask --json` contract: the
// report unchanged under "report", the artifacts pointer as a sibling under
// "artifacts" with sessionDir/traceId keys — the report schema itself stays
// untouched.
func TestAskEnvelopeJSONShape(t *testing.T) {
	env := askEnvelope{
		Report: &sidecar.Report{Answer: "routes live in router.go"},
		Artifacts: sidecar.ArtifactsRef{
			SessionDir: "/home/u/.ghx/sessions/gin-gonic-gin",
			TraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
		},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Report struct {
			Answer string `json:"answer"`
		} `json:"report"`
		Artifacts struct {
			SessionDir string `json:"sessionDir"`
			TraceID    string `json:"traceId"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Report.Answer != "routes live in router.go" {
		t.Fatalf("report.answer = %q", decoded.Report.Answer)
	}
	if decoded.Artifacts.SessionDir != "/home/u/.ghx/sessions/gin-gonic-gin" {
		t.Fatalf("artifacts.sessionDir = %q", decoded.Artifacts.SessionDir)
	}
	if decoded.Artifacts.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Fatalf("artifacts.traceId = %q", decoded.Artifacts.TraceID)
	}
}

// TestHandleReconAppendsArtifactsFooter pins the MCP recon surface: the tool
// response text is the report JSON followed by the same artifacts line the
// CLI prints, so a parent agent never guesses where the audit trail lives.
func TestHandleReconAppendsArtifactsFooter(t *testing.T) {
	orig := askSidecar
	defer func() { askSidecar = orig }()
	askSidecar = func(_ context.Context, _ sidecar.Config, _ sidecar.AskRequest) (*sidecar.Report, *sidecar.TurnResult, error) {
		return &sidecar.Report{Answer: "ok"}, &sidecar.TurnResult{
			Artifacts: sidecar.ArtifactsRef{
				SessionDir: "/home/u/.ghx/sessions/ownerx-repoy",
				TraceID:    "4bf92f3577b34da6a3ce929d0e0e4736",
			},
		}, nil
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"question": "q", "repo": "OwnerX/RepoY"}
	res, err := handleRecon(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	text, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want TextContent", res.Content[0])
	}
	want := "\nartifacts: /home/u/.ghx/sessions/ownerx-repoy (trace 4bf92f3577b34da6a3ce929d0e0e4736)"
	if !strings.HasSuffix(text.Text, want) {
		t.Fatalf("recon text does not end with the artifacts line:\n%s", text.Text)
	}
	if !strings.HasPrefix(text.Text, "{") {
		t.Fatalf("recon text no longer starts with the report JSON:\n%s", text.Text)
	}
}

// askAllowedBackends (--local flag): default is nil (remote-only per the
// prompt default), the grant adds tier2.LocalBackendGrant so the escalation
// policy engine may allow local analysis (ADR-0024.2).
func TestAskAllowedBackends(t *testing.T) {
	if got := askAllowedBackends(false); got != nil {
		t.Fatalf("askAllowedBackends(false) = %v, want nil (remote-only default)", got)
	}
	got := askAllowedBackends(true)
	want := []string{"remote", tier2.LocalBackendGrant}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("askAllowedBackends(true) = %v, want %v", got, want)
	}
}
