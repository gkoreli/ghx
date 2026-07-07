package sidecar

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar/tier2"
)

// RemoteBackendID is the canonical backend ID for remote ghx evidence.
const RemoteBackendID = "remote"

// ToolSpec describes one sidecar-owned tool surface. It is the single source
// of truth for sidecar-facing tool identity: command name, persona menu text,
// escalation tier, and report backend ID.
type ToolSpec struct {
	// Name is the stable sidecar tool name, matching the `ghx tier2 <name>`
	// command segment for Tier-2 tools.
	Name string
	// Tier is the canonical tierUsed value this tool implies when observed.
	Tier string
	// BackendID is the canonical report/tracing backend ID.
	BackendID string
	// PersonaMenuLine is the exact persona menu line advertised to the agent.
	PersonaMenuLine string
}

// ToolRegistry owns the sidecar tool set in registration order.
type ToolRegistry struct {
	order     []ToolSpec
	byName    map[string]ToolSpec
	byBackend map[string]ToolSpec
}

// NewToolRegistry returns an empty sidecar tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		byName:    map[string]ToolSpec{},
		byBackend: map[string]ToolSpec{},
	}
}

// Register adds a tool to the registry. It panics on duplicate tool names or
// backend IDs because duplicate identity would make report provenance
// ambiguous.
func (r *ToolRegistry) Register(t ToolSpec) {
	if t.Name == "" {
		panic("sidecar: tool name is required")
	}
	if t.BackendID == "" {
		panic(fmt.Sprintf("sidecar: backend ID is required for tool %q", t.Name))
	}
	if _, exists := r.byName[t.Name]; exists {
		panic(fmt.Sprintf("sidecar: tool %q already registered", t.Name))
	}
	if _, exists := r.byBackend[t.BackendID]; exists {
		panic(fmt.Sprintf("sidecar: backend %q already registered", t.BackendID))
	}
	r.order = append(r.order, t)
	r.byName[t.Name] = t
	r.byBackend[t.BackendID] = t
}

// Get returns a pointer to a copy of the named tool, or nil when absent.
func (r *ToolRegistry) Get(name string) *ToolSpec {
	if t, ok := r.byName[name]; ok {
		return &t
	}
	return nil
}

// GetByBackend returns a pointer to a copy of the backend's tool, or nil when
// the backend ID is not registered.
func (r *ToolRegistry) GetByBackend(backend string) *ToolSpec {
	if t, ok := r.byBackend[backend]; ok {
		return &t
	}
	return nil
}

// List returns registered tools in persona/CLI identity order.
func (r *ToolRegistry) List() []ToolSpec {
	out := make([]ToolSpec, len(r.order))
	copy(out, r.order)
	return out
}

// BackendIDs returns canonical backend IDs in registry order.
func (r *ToolRegistry) BackendIDs() []string {
	out := make([]string, 0, len(r.order))
	for _, t := range r.order {
		out = append(out, t.BackendID)
	}
	return out
}

// PersonaMenu renders the exact Tier-2 command menu embedded in the persona.
func (r *ToolRegistry) PersonaMenu() string {
	var sb strings.Builder
	for _, t := range r.order {
		sb.WriteString(t.PersonaMenuLine)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// QuotedBackendIDs renders the report-contract backend vocabulary.
func (r *ToolRegistry) QuotedBackendIDs(includeRemote bool) string {
	ids := r.BackendIDs()
	if includeRemote {
		ids = append([]string{RemoteBackendID}, ids...)
	}
	quoted := make([]string, 0, len(ids))
	for _, id := range ids {
		quoted = append(quoted, strconv.Quote(id))
	}
	return strings.Join(quoted, ", ")
}

// LocalBackend reports whether backend is one of the registered local
// structural backends.
func (r *ToolRegistry) LocalBackend(backend string) bool {
	return r.GetByBackend(strings.TrimSpace(backend)) != nil
}

// DefaultToolRegistry returns the registered sidecar tool set.
func DefaultToolRegistry() *ToolRegistry {
	r := NewToolRegistry()
	r.Register(ToolSpec{
		Name:            "codemap",
		Tier:            tier2.TierLocal,
		BackendID:       tier2.BackendCodemap,
		PersonaMenuLine: "     ghx tier2 codemap owner/repo --importers src/file.ts    (backend local:codemap)",
	})
	r.Register(ToolSpec{
		Name:            "astgrep",
		Tier:            tier2.TierLocal,
		BackendID:       tier2.BackendAstGrep,
		PersonaMenuLine: "     ghx tier2 astgrep owner/repo --pattern 'compose($$$ARGS)' --lang ts    (backend local:ast-grep)",
	})
	r.Register(ToolSpec{
		Name:            "repomap",
		Tier:            tier2.TierLocal,
		BackendID:       tier2.BackendRepomap,
		PersonaMenuLine: "     ghx tier2 repomap owner/repo --query concern    (backend local:repomap)",
	})
	return r
}

// DefaultToolSpec returns a registered sidecar tool by name, panicking when
// the default registry is missing that tool.
func DefaultToolSpec(name string) ToolSpec {
	t := DefaultToolRegistry().Get(name)
	if t == nil {
		panic(fmt.Sprintf("sidecar: default tool %q is not registered", name))
	}
	return *t
}

func sidecarToolRegistry() *ToolRegistry {
	return DefaultToolRegistry()
}

func tier2PersonaMenu() string {
	return sidecarToolRegistry().PersonaMenu()
}

func reportBackendIDsDescription() string {
	return sidecarToolRegistry().QuotedBackendIDs(true)
}

func isRegisteredLocalBackend(backend string) bool {
	return sidecarToolRegistry().LocalBackend(backend)
}
