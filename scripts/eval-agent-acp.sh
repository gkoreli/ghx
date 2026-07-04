#!/bin/sh
# ACP agent wrapper for sidecar eval runs (ADR-0016.1).
#
# The eval runner execs GHX_EVAL_AGENT as a single binary with no args
# (internal/sidecar/evals/runner.go), so this script is the stable argv0
# for the Claude Code ACP adapter. Pinned to the version validated live
# in the 2026-07-03 smoke runs (docs/evals/) — bump deliberately, never
# mid-gate-run, so every round of a run uses an identical agent command.
#
# Usage: GHX_EVAL_AGENT="$(pwd)/scripts/eval-agent-acp.sh" go test \
#   ./internal/sidecar/evals -tags=agent_e2e -run TestEpisodes
exec npx -y @agentclientprotocol/claude-agent-acp@0.55.0 "$@"
