package evals

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Regression for the 2026-07-06 review HIGH finding: ProbeAgentIdentity passed
// the whole configured command line as argv[0], so a multi-word AgentCmd (the
// pinned npx adapter line that `config init --claude-acp` writes) could never
// exec. The probe must field-split the command like the turn runner does.
func TestProbeAgentIdentitySplitsMultiWordAgentCmd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// `sh -c 'exit 3'` only execs if the command line is field-split; as a
	// single argv[0] it fails with exec format/not-found, a different error.
	_, err := ProbeAgentIdentity(ctx, RunConfig{AgentCmd: "sh -c false"})
	if err == nil {
		return // improbable, but split+run clearly worked
	}
	msg := err.Error()
	if strings.Contains(msg, "no such file") || strings.Contains(msg, "executable file not found") {
		t.Fatalf("multi-word AgentCmd was not field-split before exec: %v", err)
	}
}
