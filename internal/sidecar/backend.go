package sidecar

import (
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// Backend is a canonical evidence backend or backend grant ID.
type Backend string

const (
	// BackendRemote is the canonical backend ID for remote ghx evidence.
	BackendRemote Backend = "remote"
	// BackendLocalGrant allows all local structural backends in an ask.
	BackendLocalGrant Backend = "local"
	// BackendCodemap is the canonical codemap backend ID.
	BackendCodemap Backend = tier2.BackendCodemap
	// BackendAstGrep is the canonical ast-grep backend ID.
	BackendAstGrep Backend = tier2.BackendAstGrep
	// BackendRepomap is the canonical repomap backend ID.
	BackendRepomap Backend = tier2.BackendRepomap
)

var validBackends = map[Backend]bool{
	BackendRemote:     true,
	BackendLocalGrant: true,
	BackendCodemap:    true,
	BackendAstGrep:    true,
	BackendRepomap:    true,
}

// ParseBackend validates a backend or backend grant ID.
func ParseBackend(backend string) (Backend, bool) {
	b := Backend(strings.TrimSpace(backend))
	return b, validBackends[b]
}

// String returns the serialized backend ID.
func (b Backend) String() string {
	return string(b)
}

// IsLocal reports whether b grants or represents local structural evidence.
func (b Backend) IsLocal() bool {
	return b == BackendLocalGrant || strings.HasPrefix(b.String(), BackendLocalGrant.String()+":")
}
