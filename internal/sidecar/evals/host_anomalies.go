package evals

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals/hosttask"
)

// Host-task anomaly rows (ADR-0032.1 D1 compliance detector + D4 BLOCKED
// rows). Like the rest of the taxonomy they are declarative and derived from
// persisted episode fields only, so any host-task run can be re-analyzed
// offline. Registered by ADR-0032.1 S3; changing them mid-run is a
// measurement-stack change requiring pre-registration.
const (
	// AnomalyHostExternalExploration: an arm-B (host-sidecar) episode
	// explored the outside world by hand — an exploration-classified tool
	// call (S2 hosttask.Classifier) that is not the recon MCP tool. Soft by
	// design: the D1 contract is flag and report, NEVER block — the anomaly
	// is the compliance measurement, not an enforcement mechanism. Control
	// arm episodes never fire it: native exploration is their job.
	AnomalyHostExternalExploration = "host_external_exploration"
	// AnomalyHostRateLimited: a host-task turn failed with a provider
	// rate-limit / back-pressure shaped error (ADR-0032.1 D4). Breaking (the
	// D4 BLOCKED row): wall-time claims are excluded whenever it fires.
	// Detection reuses the frozen rateLimitSignals substring list.
	AnomalyHostRateLimited = "host_rate_limited"
	// AnomalyReconToolUnavailable: an arm-B episode shows the recon MCP tool
	// missing from the session or failing at the transport level
	// (ADR-0032.1 D4). Breaking (BLOCKED): the arm's defining capability was
	// absent, so the trial does not measure the sidecar arm. Substring
	// detection is best-effort — the D4 registration says "where
	// detectable" — over turn errors and recon-titled tool outputs.
	AnomalyReconToolUnavailable = "recon_tool_unavailable"
	// AnomalyHostExecuteOutsideWorkspace: a host-task execute command
	// referenced an absolute path outside the trial workspace. Execute-kind
	// tools are NOT policed by WorkspaceWritePolicy (the ACP write policy
	// sees only edit/delete/move kinds and client fs writes; a shell command
	// can touch anything the host user can) — that gap is a registered
	// ADR-0032.1 S3 limitation, with containers + per-trial fresh workspaces
	// as the real boundary. This detector is the best-effort visibility
	// layer over the gap: declarative absolute-path extraction from the
	// recorded rawInput command, flag-and-report only, both arms. Documented
	// evasions: computed/relative paths, paths built at runtime; documented
	// noise limit: referencing an outside path is not proof of writing it.
	AnomalyHostExecuteOutsideWorkspace = "host_execute_outside_workspace"
)

// hostExecuteAbsPathRE extracts absolute-path-shaped tokens from a command
// line: a "/"-rooted run of path characters preceded by start-of-string or a
// shell separator (whitespace, '=', quotes, backtick, '(', ',', redirects).
// The separator requirement keeps sed-style s/foo/bar/ patterns from
// matching.
var hostExecuteAbsPathRE = regexp.MustCompile("(?:^|[\\s='\"`(,<>])(/[A-Za-z0-9._~+-]+(?:/[A-Za-z0-9._~+-]*)*)")

// hostExecuteBenignPrefixes are absolute-path prefixes ordinary local
// commands reference (interpreters, system binaries, devices, read-only
// system config) that never indicate a workspace escape. Frozen with the
// measurement stack; everything else outside the workspace flags.
var hostExecuteBenignPrefixes = []string{
	"/bin/", "/sbin/", "/usr/", "/opt/", "/dev/", "/proc/", "/sys/", "/etc/", "/Library/", "/System/",
}

// reconUnavailableMarkers are the declarative substrings (matched
// case-insensitively) that mark the recon MCP tool as missing or dead. They
// are scanned only over arm-B turn errors and the output excerpts of
// recon-titled tool calls, so unrelated command output cannot false-positive.
// Frozen with the measurement stack.
var reconUnavailableMarkers = []string{
	"no such tool",      // adapter: the named tool is not available in the session
	"tool not found",    // adapter/MCP variant of the same
	"not connected",     // MCP client: server never connected or connection lost
	"connection closed", // MCP transport died mid-call
	"failed to connect", // MCP client could not reach the stdio server
	"failed to start",   // the recon server process did not come up
}

// firstReconUnavailableMarker returns the first matching marker in text, or
// "" when none match.
func firstReconUnavailableMarker(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, marker := range reconUnavailableMarkers {
		if strings.Contains(lower, marker) {
			return marker
		}
	}
	return ""
}

// detectHostTaskAnomalies derives the host-task anomaly rows for one episode
// from its persisted fields (HostTask record, turns, attribution table).
// Pure and offline like DetectAnomalies, which calls it; recon episodes
// (HostTask == nil) derive nothing here.
func detectHostTaskAnomalies(ep *Episode) []Anomaly {
	ht := ep.HostTask
	if ht == nil {
		return nil
	}
	var out []Anomaly

	// host_rate_limited (ADR-0032.1 D4): both arms, per failed turn.
	for _, turn := range ep.Turns {
		if looksRateLimited(turn.Error) {
			out = append(out, Anomaly{
				Kind: AnomalyHostRateLimited, Severity: SeverityBreaking, Turn: turn.Turn,
				Detail: fmt.Sprintf("host turn failed with a rate-limit-shaped error: %s", boundedString(turn.Error, 256)),
			})
		}
	}

	// host_execute_outside_workspace: both arms, one anomaly per execute
	// call that references outside absolute paths (best-effort visibility
	// over the unpoliced execute surface; see the kind's doc).
	for _, turn := range ep.Turns {
		for _, tr := range turn.ToolTraces {
			if tr.Kind != "execute" {
				continue
			}
			cmd := hosttask.ExecuteCommandLine(tr.RawInput, tr.Title)
			paths := executeOutsidePaths(cmd, ht.WorkspaceDir)
			if len(paths) == 0 {
				continue
			}
			out = append(out, Anomaly{
				Kind: AnomalyHostExecuteOutsideWorkspace, Severity: SeveritySoft, Turn: turn.Turn,
				Detail: fmt.Sprintf("execute command references paths outside the workspace (%s): %s",
					strings.Join(paths, ", "), boundedString(cmd, 256)),
			})
		}
	}

	if ht.Arm != HostArmSidecar {
		return out
	}
	recon := map[string]bool{}
	for _, title := range ht.ReconToolTitles {
		recon[title] = true
	}

	// host_external_exploration (ADR-0032.1 D1): one anomaly per
	// exploration-classified tool call that is not the recon MCP tool,
	// straight off the persisted attribution table. Classifier rule R1
	// guarantees recon-titled calls classify as exploration, so every other
	// exploration row is hand-exploration.
	for _, row := range ht.Attribution.PerToolCall {
		if row.Class != hosttask.ClassExploration || recon[row.Title] {
			continue
		}
		out = append(out, Anomaly{
			Kind: AnomalyHostExternalExploration, Severity: SeveritySoft, Turn: turnOfToolCall(ep, row.ID),
			Detail: fmt.Sprintf("arm-B host explored externally by hand (kind=%s): %s", row.Kind, boundedString(row.Title, 256)),
		})
	}

	// recon_tool_unavailable (ADR-0032.1 D4): declarative markers over turn
	// errors and recon-titled tool outputs. At most one anomaly per turn per
	// source keeps the list bounded.
	for _, turn := range ep.Turns {
		if marker := firstReconUnavailableMarker(turn.Error); marker != "" {
			out = append(out, Anomaly{
				Kind: AnomalyReconToolUnavailable, Severity: SeverityBreaking, Turn: turn.Turn,
				Detail: fmt.Sprintf("turn error matches recon-unavailable marker %q: %s", marker, boundedString(turn.Error, 256)),
			})
		}
		for _, tr := range turn.ToolTraces {
			if !recon[tr.Title] {
				continue
			}
			if marker := firstReconUnavailableMarker(tr.OutputExcerpt); marker != "" {
				out = append(out, Anomaly{
					Kind: AnomalyReconToolUnavailable, Severity: SeverityBreaking, Turn: turn.Turn,
					Detail: fmt.Sprintf("recon tool call %s output matches recon-unavailable marker %q: %s", tr.ID, marker, boundedString(tr.OutputExcerpt, 256)),
				})
				break
			}
		}
	}
	return out
}

// executeOutsidePaths returns the distinct absolute paths a command line
// references that lie outside the workspace root and match no benign system
// prefix. Purely lexical: no filesystem access (offline re-analysis) and no
// symlink resolution — the workspace root is compared in the exact form the
// episode recorded (the form the host was given as cwd), a documented
// best-effort limit.
func executeOutsidePaths(cmd, workspaceRoot string) []string {
	if cmd == "" || workspaceRoot == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range hostExecuteAbsPathRE.FindAllStringSubmatch(cmd, -1) {
		p := m[1]
		if p == "/" || seen[p] {
			continue
		}
		seen[p] = true
		if p == workspaceRoot || strings.HasPrefix(p, workspaceRoot+"/") {
			continue
		}
		benign := false
		for _, prefix := range hostExecuteBenignPrefixes {
			if strings.HasPrefix(p, prefix) {
				benign = true
				break
			}
		}
		if !benign {
			out = append(out, p)
		}
	}
	return out
}

// turnOfToolCall returns the turn index whose live traces contain the tool
// call ID, or 0 when not found (single-turn host episodes make 0 the honest
// default).
func turnOfToolCall(ep *Episode, id string) int {
	for _, turn := range ep.Turns {
		for _, tr := range turn.ToolTraces {
			if tr.ID == id {
				return turn.Turn
			}
		}
	}
	return 0
}
