package tier2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// BackendAstGrep is the canonical backend ID for ast-grep invocations, the
// value reports and traces must carry when this backend contributed evidence
// (ADR-0024.1 "Visibility Contract").
const BackendAstGrep = "local:ast-grep"

// astGrepBinary is the binary name discovered on PATH. ast-grep
// (github.com/ast-grep/ast-grep, MIT) is absorbed as an internal subprocess
// tool per ADR-0024.1 — it disappears from the user's operational surface,
// never from attribution. It is also the dependency codemap needs for its
// --importers and --deps surfaces.
const astGrepBinary = "ast-grep"

// AstGrepInstallHint tells a human how to make the local:ast-grep backend
// available. Shown whenever discovery fails; the ask itself must then fall
// back to remote Tier-1 evidence instead of hard-failing.
const AstGrepInstallHint = "ast-grep not found on PATH. Install it with:\n" +
	"  brew install ast-grep\n" +
	"or: cargo install ast-grep --locked / npm i -g @ast-grep/cli\n" +
	"or grab a release from https://github.com/ast-grep/ast-grep\n" +
	"(ast-grep also unblocks codemap --importers and --deps.)\n" +
	"Until installed, backend local:ast-grep is unavailable and answers fall back to remote Tier-1 evidence\n" +
	"(local:repomap is built into ghx and keeps working without it)."

// ErrAstGrepNotInstalled reports that the ast-grep binary is absent. Callers
// treat it as a graceful tier fallback signal — the blocked backend is named
// in uncertainty/nextReads, never faked (ADR-0024.1 "Escalation Policy").
var ErrAstGrepNotInstalled = errors.New("ast-grep binary not installed: backend local:ast-grep unavailable")

// AstGrepRequest describes one structural pattern search over a snapshot.
type AstGrepRequest struct {
	// Pattern is the ast-grep AST pattern, e.g. "fmt.Println($A)" or
	// "func $NAME($$$) { $$$ }". Required.
	Pattern string
	// Lang is the ast-grep language hint, e.g. "go", "ts", "py". Empty lets
	// ast-grep infer the language per file extension.
	Lang string
	// Paths are the search roots relative to the snapshot root. Empty searches
	// the whole snapshot (".").
	Paths []string
}

// args maps the request to the ast-grep CLI surface verified live against
// ast-grep 0.44.1 (2026-07-06): `run --pattern <p> [--lang <l>]
// --json=compact [paths...]`. Compact JSON keeps the evidence payload
// machine-parsable at minimal tokens (AGENTS.md "Tool Economy").
func (r AstGrepRequest) args() ([]string, error) {
	if strings.TrimSpace(r.Pattern) == "" {
		return nil, fmt.Errorf("ast-grep requires a non-empty pattern")
	}
	args := []string{"run", "--pattern", r.Pattern}
	if r.Lang != "" {
		args = append(args, "--lang", r.Lang)
	}
	args = append(args, "--json=compact")
	if len(r.Paths) > 0 {
		args = append(args, r.Paths...)
	} else {
		args = append(args, ".")
	}
	return args, nil
}

// ArtifactKey names the tool artifact slot in SnapshotMetadata
// .ToolArtifactHashes for this invocation. Patterns are arbitrary multi-line
// text, so the key carries the language plus a short digest of
// pattern+lang+paths, e.g. "ast-grep:run:go:a1b2c3d4e5f6" — deterministic for
// the same request, bounded for metadata.
func (r AstGrepRequest) ArtifactKey() string {
	lang := r.Lang
	if lang == "" {
		lang = "auto"
	}
	sum := sha256.Sum256([]byte(r.Pattern + "\x00" + r.Lang + "\x00" + strings.Join(r.Paths, "\x00")))
	return "ast-grep:run:" + lang + ":" + hex.EncodeToString(sum[:6])
}

// AstGrep is the subprocess adapter for the absorbed ast-grep tool. Discovery,
// invocation, and evidence capture live in the embedded subprocessAdapter —
// the rest of ghx sees only domain types.
type AstGrep struct {
	subprocessAdapter
}

// NewAstGrep returns the adapter with production discovery (PATH lookup).
func NewAstGrep() *AstGrep {
	return &AstGrep{subprocessAdapter{
		binary:          astGrepBinary,
		backend:         BackendAstGrep,
		errNotInstalled: ErrAstGrepNotInstalled,
		lookPath:        exec.LookPath,
		runTimeout:      2 * time.Minute,
	}}
}

// Run executes one structural pattern search inside snapshotDir and returns
// the ToolRun evidence record. ast-grep uses grep-parity exit codes (verified
// live on 0.44.1): 0 = matches found, 1 = ran fine but zero matches, other =
// real failure. A zero-match run returns a nil error with ExitCode 1 and the
// empty JSON payload — "no matches" is a finding, not a failure. Real
// failures return the populated ToolRun plus an error naming the recomputable
// command.
func (g *AstGrep) Run(ctx context.Context, snapshotDir string, req AstGrepRequest) (ToolRun, error) {
	args, err := req.args()
	if err != nil {
		return g.newRun(snapshotDir, nil), err
	}
	run, err := g.exec(ctx, snapshotDir, args)
	if err != nil && run.ExitCode == 1 && strings.TrimSpace(run.Stderr) == "" {
		return run, nil
	}
	return run, err
}
