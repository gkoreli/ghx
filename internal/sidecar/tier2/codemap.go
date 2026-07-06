package tier2

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// BackendCodemap is the canonical backend ID for codemap invocations, the
// value reports and traces must carry when this backend contributed evidence
// (ADR-0024.1 "Visibility Contract").
const BackendCodemap = "local:codemap"

// codemapBinary is the binary name discovered on PATH. codemap
// (github.com/JordanCoin/codemap, MIT) is absorbed as an internal subprocess
// tool per ADR-0024.1 — it disappears from the user's operational surface,
// never from attribution.
const codemapBinary = "codemap"

// CodemapInstallHint tells a human how to make the local:codemap backend
// available. Shown whenever discovery fails; the ask itself must then fall
// back to remote Tier-1 evidence instead of hard-failing.
const CodemapInstallHint = "codemap not found on PATH. Install it with:\n" +
	"  brew tap JordanCoin/tap && brew install codemap\n" +
	"or grab a release from https://github.com/JordanCoin/codemap\n" +
	"(--deps and blast-radius additionally need the ast-grep binary).\n" +
	"Until installed, Tier 2 structural analysis is unavailable and answers fall back to remote Tier-1 evidence."

// ErrCodemapNotInstalled reports that the codemap binary is absent. Callers
// treat it as a graceful tier fallback signal — the blocked backend is named
// in uncertainty/nextReads, never faked (ADR-0024.1 "Escalation Policy").
var ErrCodemapNotInstalled = errors.New("codemap binary not installed: backend local:codemap unavailable")

// CodemapMode selects one of the absorbed codemap surfaces. The first Tier-2
// slice exposes the surfaces ADR-0024.1 names: JSON/context output, importers,
// and dependency flow.
type CodemapMode string

const (
	// CodemapOverview runs `codemap --json .`: fast tree + hub summary as JSON.
	CodemapOverview CodemapMode = "overview"
	// CodemapContext runs `codemap context`: the universal JSON ContextEnvelope.
	CodemapContext CodemapMode = "context"
	// CodemapImporters runs `codemap --importers <file>`: fan-in for one file.
	// Requires the ast-grep binary (item 3): codemap shells out to it for
	// fan-in resolution.
	CodemapImporters CodemapMode = "importers"
	// CodemapDeps runs `codemap --deps .`: dependency flow / import chains.
	// Requires the ast-grep binary; its absence surfaces as tool failure
	// evidence, not as a fake answer.
	CodemapDeps CodemapMode = "deps"
)

// CodemapRequest describes one codemap invocation over a snapshot.
type CodemapRequest struct {
	// Mode selects the codemap surface.
	Mode CodemapMode
	// File is the target file for CodemapImporters, relative to the snapshot
	// root. Ignored by other modes.
	File string
	// Compact requests the token-minimal envelope (CodemapContext only).
	Compact bool
}

// args maps the request to the codemap CLI surface verified in ADR-0024
// research (README table, accessed 2026-07-05).
func (r CodemapRequest) args() ([]string, error) {
	switch r.Mode {
	case CodemapOverview:
		return []string{"--json", "."}, nil
	case CodemapContext:
		if r.Compact {
			return []string{"context", "--compact"}, nil
		}
		return []string{"context"}, nil
	case CodemapImporters:
		if r.File == "" {
			return nil, fmt.Errorf("codemap importers mode requires a file")
		}
		return []string{"--importers", r.File}, nil
	case CodemapDeps:
		return []string{"--deps", "."}, nil
	default:
		return nil, fmt.Errorf("unknown codemap mode %q", r.Mode)
	}
}

// ArtifactKey names the tool artifact slot in SnapshotMetadata
// .ToolArtifactHashes for this invocation, e.g. "codemap:importers:cmd/main.go".
func (r CodemapRequest) ArtifactKey() string {
	key := "codemap:" + string(r.Mode)
	if r.Mode == CodemapImporters && r.File != "" {
		key += ":" + r.File
	}
	if r.Mode == CodemapContext && r.Compact {
		key += ":compact"
	}
	return key
}

// Codemap is the subprocess adapter for the absorbed codemap tool. Discovery,
// invocation, and evidence capture live in the embedded subprocessAdapter —
// the rest of ghx sees only domain types (single-owner rule, AGENTS.md
// Engineering Tenets).
type Codemap struct {
	subprocessAdapter
}

// NewCodemap returns the adapter with production discovery (PATH lookup).
func NewCodemap() *Codemap {
	return &Codemap{subprocessAdapter{
		binary:          codemapBinary,
		backend:         BackendCodemap,
		errNotInstalled: ErrCodemapNotInstalled,
		lookPath:        exec.LookPath,
		runTimeout:      2 * time.Minute,
	}}
}

// Run executes one codemap invocation inside snapshotDir and returns the
// ToolRun evidence record. The returned error is non-nil when the binary is
// missing, the request is invalid, or the process exited non-zero — the
// ToolRun is still populated in the exit-code case so failures stay auditable.
func (c *Codemap) Run(ctx context.Context, snapshotDir string, req CodemapRequest) (ToolRun, error) {
	args, err := req.args()
	if err != nil {
		return c.newRun(snapshotDir, nil), err
	}
	return c.exec(ctx, snapshotDir, args)
}
