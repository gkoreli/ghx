package sidecar

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// TestBuildPersonaSystemPromptContract verifies the stable persona system
// prompt contains all doctrine, CLI-invocation contract, and report schema
// required by the sidecar persona (ADR-0020.1 D1).
func TestBuildPersonaSystemPromptContract(t *testing.T) {
	p := BuildPersonaSystemPrompt()

	for _, want := range []string{
		"ghx-sidecar",
		"<ghx-report>",
		"must stay under 2000 characters", // report compactness bound (ADR-0029)
		"ghx search \"repo:owner/repo symbolName\"",
		"Do not tool-search for submit_report.",
		"Use the read ladder for each file",
		// Inspect-first doctrine (ADR-0029.1): inspect is the preferred
		// first move for concern-shaped questions, and it appears in the
		// canonical command example list.
		"ghx inspect owner/repo \"concern phrase\"",
		"PREFER one\n   `ghx inspect owner/repo \"concern\"` as your first command",
		"Do not cite `/tmp` files",
		// Tier-2 escalation doctrine (ADR-0029.2 D1): tier2 tools exist for
		// cross-file structure questions AFTER remote evidence falls short,
		// with canonical local:* backend IDs and tierUsed in the report.
		"Escalate to Tier 2 for cross-file STRUCTURE questions",
		"only AFTER remote evidence (inspect/search/maps) falls short",
		"ghx tier2 codemap owner/repo --importers src/file.ts    (backend local:codemap)",
		"ghx tier2 astgrep owner/repo --pattern 'compose($$$ARGS)' --lang ts    (backend local:ast-grep)",
		"ghx tier2 repomap owner/repo --query concern    (backend local:repomap)",
		"list the local:* backend in backendsUsed, and set tierUsed to \"tier2\"",
		"`answer` must be the direct answer first and at most\n  2 sentences",
		// CLI-invocation contract (ADR-0016.7): the agent must know ghx is
		// a shell command, not a registered tool, and must verify before
		// ever reporting BLOCKED.
		"command-line binary already installed on PATH",
		"NOT an MCP tool",
		"ghx version",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("persona system prompt missing %q", want)
		}
	}
	if strings.Contains(p, "If ghx is unavailable, call submit_report immediately") {
		t.Error("blind BLOCKED escape hatch must be gone (ADR-0016.7 RC1)")
	}
	// Per-turn fields must NOT be in the persona.
	for _, absent := range []string{
		"## Repo", "## Session", "## Question", "## Depth", "## Allowed backends",
		"Prior session context",
	} {
		if strings.Contains(p, absent) {
			t.Errorf("persona system prompt must not contain per-turn field %q", absent)
		}
	}
}

// TestRepoScopedPersonaByteStable pins the repo-scoped persona bytes so
// accidental drift is caught: discovery mode must not perturb the persona used
// when a repo IS provided (ADR-0019.1 D4). Deliberate persona changes update
// the golden hash in the same commit as their ADR (current bytes: ADR-0029.2
// persona revision 3, tier-2 doctrine + discovery citation discipline).
func TestRepoScopedPersonaByteStable(t *testing.T) {
	base := BuildPersonaSystemPrompt()
	const wantSHA = "400ecd3da93343475f3148321b710e50c5b8b2ca9c842158e5f5381d2240287b"
	if got := fmt.Sprintf("%x", sha256.Sum256([]byte(base))); got != wantSHA {
		t.Fatalf("repo-scoped persona bytes changed: sha256 = %s, want %s (if the change is intentional, update the golden hash)", got, wantSHA)
	}
	if strings.Contains(base, "Discovery mode") {
		t.Error("repo-scoped persona must not contain the discovery doctrine")
	}
}

// TestDiscoveryPersonaExtendsBase verifies the discovery persona is exactly
// the repo-scoped persona plus the discovery doctrine (ADR-0019.1 D4).
func TestDiscoveryPersonaExtendsBase(t *testing.T) {
	base := BuildPersonaSystemPrompt()
	disc := BuildDiscoveryPersonaSystemPrompt()
	if !strings.HasPrefix(disc, base) {
		t.Fatal("discovery persona must start with the exact repo-scoped persona")
	}
	for _, want := range []string{
		"## Discovery mode",
		"several distinct query formulations", // broad sweep, multiple queries
		"real usage in code",                  // triangulation signals
		"ranked comparison", // ranked candidate output
		"unverified tail",   // honesty about the unswept rest
		// Discovery citation discipline (ADR-0029.2 D2): read-then-cite with
		// the owner/repo:path form mandatory on every verified claim; a repo
		// seen only in search results is INFERRED, never verified.
		"read, then cite",
		"cites that\n   exact file in owner/repo:path form",
		"is INFERRED — never claim it as\n   verified",
		"Worked example — after running: ghx read acme/rate-limiter README.md",
		"acme/rate-limiter:README.md",
		"a bare owner/repo name in evidence\nnever earns verified credit",
	} {
		if !strings.Contains(disc, want) {
			t.Errorf("discovery persona missing %q", want)
		}
	}
}

// TestBuildPromptFirstTurn verifies the per-turn prompt carries the
// question-specific context and NOT the stable persona (ADR-0020.1 D1).
func TestBuildPromptFirstTurn(t *testing.T) {
	p := BuildPrompt(Request{
		Session:  "hono-middleware",
		Repo:     "honojs/hono",
		Question: "Where is middleware composition implemented?",
	}, nil)

	for _, want := range []string{
		"honojs/hono",
		"hono-middleware",
		"Where is middleware composition implemented?",
		"normal",   // default depth
		"- remote", // default backend
	} {
		if !strings.Contains(p, want) {
			t.Errorf("first-turn prompt missing %q", want)
		}
	}
	// Persona content must NOT be duplicated in per-turn prompt (ADR-0020.1 D1).
	for _, absent := range []string{
		"You are ghx-sidecar",
		"command-line binary already installed on PATH",
		"NOT an MCP tool",
		"ghx version",
	} {
		if strings.Contains(p, absent) {
			t.Errorf("per-turn prompt must not contain persona text %q (goes in system prompt)", absent)
		}
	}
	if strings.Contains(p, "Prior session context") {
		t.Error("first-turn prompt must not contain prior session context")
	}
}

// TestBuildPromptRepoScopedByteStable pins the exact first-turn prompt when a
// repo is provided: discovery mode must not change repo-scoped prompts
// (ADR-0019.1 D1).
func TestBuildPromptRepoScopedByteStable(t *testing.T) {
	p := BuildPrompt(Request{Session: "s", Repo: "o/r", Question: "q"}, nil)
	want := "## Repo\n\no/r\n\n## Session\n\ns\n\n## Question\n\nq\n\n## Depth\n\nnormal\n\n## Allowed backends\n\n- remote\n"
	if p != want {
		t.Fatalf("repo-scoped first-turn prompt changed:\ngot  %q\nwant %q", p, want)
	}
}

// TestBuildPromptDiscoveryScope verifies the per-turn prompt without a repo
// declares the discovery scope instead of an empty Repo header (ADR-0019.1 D1).
func TestBuildPromptDiscoveryScope(t *testing.T) {
	p := BuildPrompt(Request{Session: "s", Question: "which repos do X"}, nil)
	if strings.Contains(p, "## Repo") {
		t.Error("discovery prompt must not render an empty Repo header")
	}
	if !strings.Contains(p, "## Scope\n\ndiscovery") {
		t.Errorf("discovery prompt missing discovery scope header:\n%s", p)
	}
	if !strings.Contains(p, "which repos do X") {
		t.Error("discovery prompt missing the question")
	}
}

func TestBuildPromptDefaults(t *testing.T) {
	p := BuildPrompt(Request{Repo: "o/r", Question: "q", Depth: "deep", AllowedBackends: []string{"remote", "codemap"}}, nil)
	if !strings.Contains(p, "deep") {
		t.Error("explicit depth not rendered")
	}
	if !strings.Contains(p, "- codemap") {
		t.Error("explicit backend not rendered")
	}
}

func TestBuildPromptFollowUpIncludesPriorContext(t *testing.T) {
	meta := &SessionMeta{
		Name:      "hono-middleware",
		Repo:      "honojs/hono",
		Scope:     "middleware composition",
		TurnCount: 2,
	}
	ledger := &Ledger{
		InspectedPaths: []LedgerEntry{{Value: "src/compose.ts", Turn: 1}},
		RelevantFiles:  []RelevantFileEntry{{Path: "src/compose.ts", Reason: "defines compose", Turn: 1}},
		OpenQuestions:  []LedgerEntry{{Value: "trace onError path", Turn: 1}},
		CommandsRun:    []LedgerEntry{{Value: "ghx read honojs/hono src/compose.ts", Turn: 1}},
	}
	p := BuildPrompt(Request{
		Session:  "hono-middleware",
		Repo:     "honojs/hono",
		Question: "How do errors propagate through that path?",
	}, meta, ledger)

	if !strings.Contains(p, "Prior session context") {
		t.Fatal("follow-up prompt missing prior session context block")
	}
	if !strings.Contains(p, "Turns completed: 2") {
		t.Error("follow-up prompt missing turn count")
	}
	if !strings.Contains(p, "middleware composition") {
		t.Error("follow-up prompt missing scope")
	}
	for _, want := range []string{
		"## Evidence ledger",
		"Files already inspected must not be re-read unless the new question requires different lines",
		"src/compose.ts - defines compose (turn 1)",
		"trace onError path",
		"ghx read honojs/hono src/compose.ts",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("follow-up prompt missing ledger content %q", want)
		}
	}
}

func TestBuildPromptZeroTurnMetaOmitsPriorContext(t *testing.T) {
	// A freshly initialized session (TurnCount 0) has nothing to carry over.
	meta := &SessionMeta{Name: "s", Repo: "o/r", TurnCount: 0}
	p := BuildPrompt(Request{Repo: "o/r", Question: "q"}, meta)
	if strings.Contains(p, "Prior session context") {
		t.Error("zero-turn session must not render prior context")
	}
}

func TestBuildPromptEvidenceLedgerBoundedAndNewestFirst(t *testing.T) {
	meta := &SessionMeta{Name: "s", Repo: "o/r", Scope: "scope", TurnCount: 3}
	ledger := &Ledger{
		OpenQuestions: []LedgerEntry{{Value: "must keep even when trimming", Turn: 1}},
	}
	for i := 0; i < 80; i++ {
		ledger.CommandsRun = append(ledger.CommandsRun, LedgerEntry{Value: "ghx read o/r src/old-file-with-a-long-name.ts", Turn: i + 1})
		ledger.InspectedPaths = append(ledger.InspectedPaths, LedgerEntry{Value: "src/path-with-a-long-name-that-may-be-trimmed.ts", Turn: i + 1})
	}

	p := BuildPrompt(Request{Session: "s", Repo: "o/r", Question: "q"}, meta, ledger)
	start := strings.Index(p, "## Evidence ledger")
	if start == -1 {
		t.Fatal("missing evidence ledger")
	}
	// The evidence ledger is the last section in the per-turn prompt (ADR-0020.1 D1
	// moved the persona to the system prompt, so no more ## sections follow it).
	// Take the block from the ledger heading to end of string.
	block := p[start:]
	// Check there is no subsequent ## section that would indicate the persona leaked
	// back into the per-turn prompt.
	rest := p[start+len("## Evidence ledger"):]
	if idx := strings.Index(rest, "\n## "); idx != -1 {
		t.Errorf("unexpected ## section after evidence ledger (persona leak?): %q", rest[idx:idx+40])
	}
	if len(block) > 1500 {
		t.Fatalf("ledger block length = %d, want <= 1500\n%s", len(block), block)
	}
	if !strings.Contains(block, "must keep even when trimming") {
		t.Fatalf("open question was trimmed:\n%s", block)
	}
	if !strings.Contains(block, "(turn 80)") {
		t.Fatalf("newest evidence missing:\n%s", block)
	}
}
