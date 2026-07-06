package tier2

// BackendKind classifies how a Tier-2 backend executes.
type BackendKind string

const (
	// KindEmbedded backends run in-process inside the ghx binary: no external
	// install, available on every machine ghx runs on.
	KindEmbedded BackendKind = "embedded"
	// KindExternal backends exec an absorbed external binary discovered on
	// PATH (ADR-0024.1 "Absorb vs Steal"). They are optional installs: when
	// absent, asks fall back to remote Tier-1 evidence instead of hard-failing.
	KindExternal BackendKind = "external"
)

// BackendStatus reports how one Tier-2 backend resolves on this machine:
// what would run, whether it is available right now, and — for an absent
// external binary — the exact install remediation. It exists so doctor-style
// surfaces can state Tier-2 availability at setup time instead of letting an
// escalation discover a missing binary mid-ask (first production escalation,
// honojs-hono session, 2026-07-06: codemap exited 3 on an uninstalled
// machine and the turn fell back to remote evidence).
type BackendStatus struct {
	// Backend is the canonical backend ID, e.g. "local:repomap"
	// (ADR-0024.1 "Visibility Contract").
	Backend string
	// Kind says whether the backend is compiled into ghx or execs an external
	// binary.
	Kind BackendKind
	// Available reports whether the backend can run right now.
	Available bool
	// Detail is the human-readable resolution: the resolved binary path for a
	// found external, "not found on PATH" for a missing one, or the built-in
	// statement for an embedded backend.
	Detail string
	// InstallHint is the full remediation text when an external backend is
	// unavailable; empty otherwise.
	InstallHint string
}

// BackendStatuses resolves every Tier-2 backend with production discovery
// (PATH lookup), in canonical order: the embedded backend first, then the
// external subprocess backends.
func BackendStatuses() []BackendStatus {
	return backendStatuses(NewCodemap(), NewAstGrep())
}

// backendStatuses is the injectable body of BackendStatuses; tests pass
// adapters with a swapped lookPath.
func backendStatuses(cm *Codemap, ag *AstGrep) []BackendStatus {
	return []BackendStatus{
		{
			Backend:   BackendRepomap,
			Kind:      KindEmbedded,
			Available: true,
			Detail:    "built into ghx (`ghx tier2 repomap`), no install needed",
		},
		externalStatus(BackendCodemap, &cm.subprocessAdapter, CodemapInstallHint),
		externalStatus(BackendAstGrep, &ag.subprocessAdapter, AstGrepInstallHint),
	}
}

// externalStatus resolves one subprocess-backed backend to its status.
func externalStatus(backend string, a *subprocessAdapter, hint string) BackendStatus {
	st := BackendStatus{Backend: backend, Kind: KindExternal}
	path, err := a.Discover()
	if err != nil {
		st.Detail = a.binary + " not found on PATH"
		st.InstallHint = hint
		return st
	}
	st.Available = true
	st.Detail = path
	return st
}
