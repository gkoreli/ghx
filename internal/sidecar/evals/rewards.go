package evals

import (
	"regexp"
	"strings"
)

// perQuestionBudget mirrors the command budget in the sidecar persona
// (internal/sidecar/prompt.go): max 8 ghx commands per question.
const perQuestionBudget = 8

// ComputeRewards scores one episode against its task with the deterministic
// checklist from ADR-0016.1. All rewards are in [0, 1]. No LLM involved.
func ComputeRewards(task Task, ep *Episode) RewardBreakdown {
	r := RewardBreakdown{
		Correctness: correctnessReward(task, ep),
		Evidence:    evidenceReward(ep),
		Trajectory:  trajectoryReward(task, ep),
		Compression: compressionReward(ep),
		Safety:      safetyReward(ep),
	}
	if len(task.Turns) > 1 {
		r.MemoryApplies = true
		r.Memory = memoryReward(ep)
	}

	sum := r.Correctness + r.Evidence + r.Trajectory + r.Compression + r.Safety
	n := 5.0
	if r.MemoryApplies {
		sum += r.Memory
		n++
	}
	r.Overall = sum / n
	return r
}

// answerText gathers everything the episode "said": structured report fields
// when a report exists, plus all raw turn text (direct profiles only have
// the latter). Lowercased for case-insensitive matching.
func answerText(ep *Episode) string {
	var sb strings.Builder
	if ep.Report != nil {
		rep := ep.Report
		sb.WriteString(rep.Answer)
		sb.WriteByte('\n')
		for _, c := range rep.Verified {
			sb.WriteString(c.Summary + "\n" + c.Evidence + "\n")
		}
		for _, c := range rep.Inferred {
			sb.WriteString(c.Summary + "\n")
		}
		for _, f := range rep.RelevantFiles {
			sb.WriteString(f.Path + "\n" + f.Reason + "\n")
		}
		for _, e := range rep.Evidence {
			sb.WriteString(e.Summary + "\n")
		}
	}
	for _, t := range ep.Turns {
		sb.WriteString(t.Text)
		sb.WriteByte('\n')
	}
	return strings.ToLower(sb.String())
}

// correctnessReward: mean of expected-file, expected-symbol, and
// required-claim hit fractions. Any unacceptable claim zeroes the reward.
func correctnessReward(task Task, ep *Episode) float64 {
	text := answerText(ep)
	c := task.Checks

	for _, bad := range c.UnacceptableClaims {
		if strings.Contains(text, strings.ToLower(bad)) {
			return 0
		}
	}

	var parts []float64
	if len(c.ExpectedFiles) > 0 {
		parts = append(parts, hitFraction(c.ExpectedFiles, func(f string) bool {
			return fileIdentified(ep, text, f)
		}))
	}
	if len(c.ExpectedSymbols) > 0 {
		parts = append(parts, hitFraction(c.ExpectedSymbols, func(s string) bool {
			return symbolIdentified(text, s)
		}))
	}
	if len(c.RequiredClaims) > 0 {
		parts = append(parts, hitFraction(c.RequiredClaims, func(cl string) bool {
			return strings.Contains(text, strings.ToLower(cl))
		}))
	}
	return mean(parts)
}

// symbolIdentified reports whether a symbol appears as a whole word in the
// answer text (ADR-0016.2): raw substring matching let generic symbols fire
// on unrelated prose ("route" inside "routes"), inflating correctness
// without genuine identification.
func symbolIdentified(lowerText, symbol string) bool {
	re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(symbol) + `\b`)
	if err != nil {
		return strings.Contains(lowerText, strings.ToLower(symbol))
	}
	return re.MatchString(lowerText)
}

// citesEvidence reports whether an evidence string actually anchors to code
// (ADR-0016.2): it must contain a path-like token or a :line reference.
// Bare prose ("verified this manually") does not count as evidence.
func citesEvidence(evidence string) bool {
	if pathTokenRE.MatchString(evidence) {
		return true
	}
	return lineRefRE.MatchString(evidence)
}

// fileIdentified reports whether an expected file was surfaced: by suffix
// match in the report's relevant files, or by mention anywhere in the text.
func fileIdentified(ep *Episode, lowerText, file string) bool {
	lf := strings.ToLower(file)
	if ep.Report != nil {
		for _, rf := range ep.Report.RelevantFiles {
			if strings.HasSuffix(strings.ToLower(rf.Path), lf) {
				return true
			}
		}
	}
	return strings.Contains(lowerText, lf)
}

// evidenceReward measures whether claims are anchored to inspectable evidence.
// With a structured report: fraction of verified claims carrying evidence,
// presence of a command trail, and fraction of relevant files with reasons.
// Without one (direct profiles): tool usage plus at least one concrete file
// path in the answer is the deterministic floor we can check.
func evidenceReward(ep *Episode) float64 {
	if ep.Report == nil {
		var parts []float64
		if len(allToolCalls(ep)) > 0 {
			parts = append(parts, 1)
		} else {
			parts = append(parts, 0)
		}
		if len(pathTokens(answerText(ep))) > 0 {
			parts = append(parts, 1)
		} else {
			parts = append(parts, 0)
		}
		return mean(parts)
	}

	rep := ep.Report
	var parts []float64

	if len(rep.Verified) > 0 {
		withEvidence := 0
		for _, c := range rep.Verified {
			if citesEvidence(c.Evidence) {
				withEvidence++
			}
		}
		parts = append(parts, float64(withEvidence)/float64(len(rep.Verified)))
	} else {
		parts = append(parts, 0)
	}

	if len(rep.CommandsRun) > 0 {
		parts = append(parts, 1)
	} else {
		parts = append(parts, 0)
	}

	if len(rep.RelevantFiles) > 0 {
		withReason := 0
		for _, f := range rep.RelevantFiles {
			if strings.TrimSpace(f.Reason) != "" {
				withReason++
			}
		}
		parts = append(parts, float64(withReason)/float64(len(rep.RelevantFiles)))
	} else {
		parts = append(parts, 0)
	}

	return mean(parts)
}

// trajectoryReward is 1 minus the mean of three penalties: duplicate command
// ratio, over-budget ratio, and avoid-path touch ratio. Floor 0.
func trajectoryReward(task Task, ep *Episode) float64 {
	cmds := allToolCalls(ep)
	if len(cmds) == 0 && ep.Report != nil {
		// Some agents run tools without emitting tool_call events; fall back
		// to the self-reported command trail.
		cmds = ep.Report.CommandsRun
	}
	if len(cmds) == 0 {
		return 0
	}

	seen := map[string]int{}
	for _, c := range cmds {
		seen[strings.TrimSpace(c)]++
	}
	dups := 0
	for _, n := range seen {
		if n > 1 {
			dups += n - 1
		}
	}
	dupRatio := float64(dups) / float64(len(cmds))

	budget := perQuestionBudget * len(ep.Turns)
	overRatio := 0.0
	if len(cmds) > budget {
		overRatio = float64(len(cmds)-budget) / float64(len(cmds))
	}

	avoided := 0
	if len(task.Checks.AvoidPaths) > 0 {
		for _, c := range cmds {
			lc := strings.ToLower(c)
			for _, p := range task.Checks.AvoidPaths {
				if strings.Contains(lc, strings.ToLower(p)) {
					avoided++
					break
				}
			}
		}
	}
	avoidRatio := float64(avoided) / float64(len(cmds))

	score := 1 - (dupRatio+overRatio+avoidRatio)/3
	if score < 0 {
		return 0
	}
	return score
}

// compressionReward rewards keeping the main agent's context small relative
// to the total workflow. Direct profiles score 0 by construction (main ==
// total); the cross-profile gate G3 compares absolute main-agent chars.
func compressionReward(ep *Episode) float64 {
	if ep.Context.TotalWorkflowChars == 0 {
		return 0
	}
	score := 1 - float64(ep.Context.MainAgentChars)/float64(ep.Context.TotalWorkflowChars)
	if score < 0 {
		return 0
	}
	return score
}

// memoryReward (multi-turn only): 1 minus the repeat-read ratio across turns,
// zeroed when any follow-up turn failed to resume prior context.
func memoryReward(ep *Episode) float64 {
	for i, t := range ep.Turns {
		if i > 0 && !t.Resumed {
			return 0
		}
	}
	if len(ep.Turns) < 2 {
		return 0
	}

	repeats, reads := repeatReadCounts(ep)
	if reads == 0 {
		return 1 // nothing re-read
	}
	return 1 - float64(repeats)/float64(reads)
}

// safetyReward is binary: any recorded violation fails the episode.
func safetyReward(ep *Episode) float64 {
	if len(ep.Violations) > 0 {
		return 0
	}
	return 1
}

// ── helpers ──────────────────────────────────────────────────────────────────

var pathTokenRE = regexp.MustCompile(`[\w./-]*/[\w.-]+\.[a-z]{1,5}\b`)

// lineRefRE matches file:line style citations ("compose.ts:32", "app.py:120").
var lineRefRE = regexp.MustCompile(`\w:\d+\b`)

// pathTokens extracts file-path-looking tokens (must contain a slash and an
// extension) for file-mention and repeat-read detection.
func pathTokens(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range pathTokenRE.FindAllString(text, -1) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func allToolCalls(ep *Episode) []string {
	var out []string
	for _, t := range ep.Turns {
		out = append(out, t.ToolCalls...)
	}
	return out
}

type readCommand struct {
	normalized string
	path       string
	hasLines   bool
	hasMap     bool
	grep       string
}

type pathInspection struct {
	commands map[string]bool
	greps    map[string]bool
	sawMap   bool
}

func repeatReadCounts(ep *Episode) (repeats, laterReads int) {
	if len(ep.Turns) < 2 {
		return 0, 0
	}
	seen := map[string]*pathInspection{}
	for _, call := range ep.Turns[0].ToolCalls {
		if rc, ok := parseReadCommand(call); ok {
			recordInspection(seen, rc)
		}
	}
	for _, turn := range ep.Turns[1:] {
		for _, call := range turn.ToolCalls {
			rc, ok := parseReadCommand(call)
			if !ok {
				continue
			}
			laterReads++
			if isRepeatRead(seen, rc) {
				repeats++
			}
			recordInspection(seen, rc)
		}
	}
	return repeats, laterReads
}

func recordInspection(seen map[string]*pathInspection, rc readCommand) {
	if rc.path == "" {
		return
	}
	pi := seen[rc.path]
	if pi == nil {
		pi = &pathInspection{commands: map[string]bool{}, greps: map[string]bool{}}
		seen[rc.path] = pi
	}
	pi.commands[rc.normalized] = true
	if rc.grep != "" {
		pi.greps[rc.grep] = true
	}
	if rc.hasMap {
		pi.sawMap = true
	}
}

func isRepeatRead(seen map[string]*pathInspection, rc readCommand) bool {
	pi := seen[rc.path]
	if pi == nil {
		return false
	}
	if pi.commands[rc.normalized] {
		return true
	}
	if rc.hasLines {
		return false
	}
	if rc.grep != "" {
		return pi.greps[rc.grep]
	}
	if rc.hasMap && !pi.sawMap {
		return false
	}
	return true
}

func parseReadCommand(call string) (readCommand, bool) {
	cmd := normalizeToolCommand(call)
	fields := strings.Fields(cmd)
	for i := 0; i+3 < len(fields); i++ {
		if !rawTokenInvokesGhx(fields[i]) || fields[i+1] != "read" {
			continue
		}
		rc := readCommand{normalized: strings.Join(fields[i:], " ")}
		rc.path = strings.Trim(fields[i+3], `"'`)
		for j := i + 4; j < len(fields); j++ {
			switch fields[j] {
			case "--lines":
				rc.hasLines = true
				j++
			case "--grep":
				if j+1 < len(fields) {
					rc.grep = strings.Trim(fields[j+1], `"'`)
					j++
				}
			case "--map":
				rc.hasMap = true
			}
		}
		if rc.path == "" {
			return readCommand{}, false
		}
		return rc, true
	}
	return readCommand{}, false
}

func normalizeToolCommand(call string) string {
	call = strings.TrimSpace(call)
	if idx := strings.LastIndex(call, " ("); idx > 0 && strings.HasSuffix(call, ")") {
		call = strings.TrimSpace(call[:idx])
	}
	if idx := strings.Index(call, ": "); idx >= 0 {
		prefix := strings.ToLower(strings.TrimSpace(call[:idx]))
		if !strings.Contains(prefix, "ghx") {
			call = strings.TrimSpace(call[idx+2:])
		}
	}
	return call
}

func hitFraction(items []string, hit func(string) bool) float64 {
	if len(items) == 0 {
		return 0
	}
	n := 0
	for _, it := range items {
		if hit(it) {
			n++
		}
	}
	return float64(n) / float64(len(items))
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}
