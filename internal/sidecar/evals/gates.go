package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Gate thresholds pre-registered in ADR-0016.1. These constants ARE the
// contract; changing them requires an ADR update, not a code tweak.
const (
	// G1: sidecar correctness ≥ g1RelFactor × ghx correctness, and ≥ g1AbsFloor.
	g1RelFactor = 0.9
	g1AbsFloor  = 0.60
	// G2: sidecar evidence ≥ g2Floor.
	g2Floor = 0.70
	// G3: sidecar main-agent chars ≤ g3Factor × ghx main-agent chars.
	g3Factor = 0.35
	// G4: follow-up resumption success rate ≥ g4ResumeRate.
	g4ResumeRate = 0.80
)

// Pre-registered gate-run sample minimums (ADR-0016.1). A verdict computed
// below these is labeled PRELIMINARY (ADR-0016.2) — gate math still runs,
// but the output must not be read as the project verdict.
const (
	minGateRunTasks          = 6
	minGateRunRepos          = 3
	minGateRunTrials         = 5
	minGateRunMultiTurnTasks = 2
)

// ProfileAggregate holds per-profile means across a set of episodes.
type ProfileAggregate struct {
	Profile  Profile `json:"profile"`
	Episodes int     `json:"episodes"`

	MeanCorrectness float64 `json:"meanCorrectness"`
	MeanEvidence    float64 `json:"meanEvidence"`
	MeanTrajectory  float64 `json:"meanTrajectory"`
	MeanCompression float64 `json:"meanCompression"`
	MeanSafety      float64 `json:"meanSafety"`
	MeanOverall     float64 `json:"meanOverall"`

	MeanMainAgentChars float64 `json:"meanMainAgentChars"`

	// Multi-turn-only signals (G4). MultiTurnEpisodes is the denominator.
	MultiTurnEpisodes int     `json:"multiTurnEpisodes"`
	ResumeRate        float64 `json:"resumeRate"`
	RepeatReadRatio   float64 `json:"repeatReadRatio"`
}

// GateResult is one pre-registered gate check with its evidence.
type GateResult struct {
	ID     string `json:"id"`
	Desc   string `json:"desc"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// Verdict is the run-level outcome: aggregates, gate results, and the
// thesis call per the ADR-0016.1 verdict rules.
type Verdict struct {
	Aggregates map[Profile]*ProfileAggregate `json:"aggregates"`
	Gates      []GateResult                  `json:"gates"`
	Anomalies  []AnomalyCount                `json:"anomalies,omitempty"`
	// ThesisSupported is true only when the run is valid, the thesis gates
	// (G1, G2, G3, G5) pass, AND the sample meets the pre-registered
	// gate-run minimums (ADR-0016.8 D3): a below-contract sample can never
	// read as a citable supported verdict in the JSON.
	ThesisSupported bool `json:"thesisSupported"`
	// Preliminary mirrors !DataSufficient explicitly in the JSON so machine
	// consumers see the PRELIMINARY label the markdown always carried
	// (ADR-0016.2 via ADR-0016.8 D3).
	Preliminary bool `json:"preliminary"`
	// Valid is false when compliance or identity checks invalidate the run.
	Valid bool `json:"valid"`
	// DataSufficient reports whether the episode set meets the
	// pre-registered gate-run minimums (≥ 6 tasks across ≥ 3 repos,
	// ≥ 5 trials per task × profile, ≥ 2 multi-turn tasks, all profiles
	// present). When false the verdict is PRELIMINARY regardless of gate
	// results.
	DataSufficient bool `json:"dataSufficient"`
	// Notes carries verdict-rule context (e.g. G4 blocking ADR-0017),
	// data-sufficiency caveats, and data-quality warnings.
	Notes []string `json:"notes"`
	// Stopping is the optional sequential-stopping analysis (ADR-0025 D1).
	// The runner attaches it after gate evaluation when the run manifest's
	// ExpectedEpisodes is known; FormatVerdict renders it as the
	// "## Sequential stopping" section. Nil leaves the verdict unchanged.
	Stopping *StoppingBounds `json:"stopping,omitempty"`
}

// Aggregate computes per-profile means over episodes.
func Aggregate(episodes []*Episode) map[Profile]*ProfileAggregate {
	agg := map[Profile]*ProfileAggregate{}
	for _, p := range AllProfiles() {
		agg[p] = &ProfileAggregate{Profile: p}
	}

	type sums struct {
		corr, evid, traj, comp, safe, overall, mainChars float64
		resumeOK, repeatSum                              float64
		multiTurn                                        int
	}
	bag := map[Profile]*sums{}
	for _, p := range AllProfiles() {
		bag[p] = &sums{}
	}

	for _, ep := range episodes {
		a, ok := agg[ep.Profile]
		if !ok {
			continue
		}
		s := bag[ep.Profile]
		a.Episodes++
		s.corr += ep.Rewards.Correctness
		s.evid += ep.Rewards.Evidence
		s.traj += ep.Rewards.Trajectory
		s.comp += ep.Rewards.Compression
		s.safe += ep.Rewards.Safety
		s.overall += ep.Rewards.Overall
		s.mainChars += float64(ep.Context.MainAgentChars)

		if len(ep.Turns) > 1 {
			s.multiTurn++
			if allFollowupsResumed(ep) {
				s.resumeOK++
			}
			s.repeatSum += repeatReadRatio(ep)
		}
	}

	for p, a := range agg {
		s := bag[p]
		if a.Episodes > 0 {
			n := float64(a.Episodes)
			a.MeanCorrectness = s.corr / n
			a.MeanEvidence = s.evid / n
			a.MeanTrajectory = s.traj / n
			a.MeanCompression = s.comp / n
			a.MeanSafety = s.safe / n
			a.MeanOverall = s.overall / n
			a.MeanMainAgentChars = s.mainChars / n
		}
		a.MultiTurnEpisodes = s.multiTurn
		if s.multiTurn > 0 {
			a.ResumeRate = s.resumeOK / float64(s.multiTurn)
			a.RepeatReadRatio = s.repeatSum / float64(s.multiTurn)
		}
	}
	return agg
}

func allFollowupsResumed(ep *Episode) bool {
	for i, t := range ep.Turns {
		if i > 0 && !t.Resumed {
			return false
		}
	}
	return true
}

// repeatReadRatio is the fraction of follow-up path reads that repeat a
// turn-0 read (the memory reward's raw signal, without the resumption zeroing).
func repeatReadRatio(ep *Episode) float64 {
	if len(ep.Turns) < 2 {
		return 0
	}
	repeats, reads := repeatReadCounts(ep)
	if reads == 0 {
		return 0
	}
	return float64(repeats) / float64(reads)
}

// EvaluateGates applies the pre-registered ADR-0016.1 gates to a set of
// episodes and returns the verdict. It works on whatever episodes exist and
// reports data-sufficiency caveats in Notes; the formal gate run requires
// ≥ 6 tasks × 5 trials × 3 profiles.
func EvaluateGates(episodes []*Episode) Verdict {
	filtered, validityNotes, valid := validateEpisodesForVerdict(episodes)
	agg := Aggregate(filtered)
	sc := agg[ProfileSidecar]
	gx := agg[ProfileGhx]

	v := Verdict{Aggregates: agg, Anomalies: CountAnomalies(episodes), Valid: valid}
	v.Notes = append(v.Notes, validityNotes...)

	if sc.Episodes == 0 || gx.Episodes == 0 {
		v.Notes = append(v.Notes, fmt.Sprintf(
			"INSUFFICIENT DATA: sidecar episodes=%d, ghx episodes=%d — gates need both profiles present",
			sc.Episodes, gx.Episodes))
	}

	var sufficiencyNotes []string
	v.DataSufficient, sufficiencyNotes = sampleSufficiency(filtered)
	v.Preliminary = !v.DataSufficient
	v.Notes = append(v.Notes, sufficiencyNotes...)
	v.Notes = append(v.Notes, dataQualityNotes(filtered, agg)...)

	g1 := GateResult{
		ID:   "G1",
		Desc: fmt.Sprintf("correctness: sidecar ≥ %.2f × ghx and ≥ %.2f absolute", g1RelFactor, g1AbsFloor),
		Pass: sc.MeanCorrectness >= g1RelFactor*gx.MeanCorrectness && sc.MeanCorrectness >= g1AbsFloor,
		Detail: fmt.Sprintf("sidecar %.3f vs ghx %.3f (rel floor %.3f, abs floor %.2f)",
			sc.MeanCorrectness, gx.MeanCorrectness, g1RelFactor*gx.MeanCorrectness, g1AbsFloor),
	}

	g2 := GateResult{
		ID:     "G2",
		Desc:   fmt.Sprintf("evidence: sidecar ≥ %.2f", g2Floor),
		Pass:   sc.MeanEvidence >= g2Floor,
		Detail: fmt.Sprintf("sidecar %.3f", sc.MeanEvidence),
	}

	g3 := GateResult{
		ID:   "G3",
		Desc: fmt.Sprintf("compression: sidecar main-agent chars ≤ %.2f × ghx", g3Factor),
		Pass: gx.MeanMainAgentChars > 0 && sc.MeanMainAgentChars <= g3Factor*gx.MeanMainAgentChars,
		Detail: fmt.Sprintf("sidecar %.0f vs ghx %.0f chars (threshold %.0f)",
			sc.MeanMainAgentChars, gx.MeanMainAgentChars, g3Factor*gx.MeanMainAgentChars),
	}

	g4 := GateResult{
		ID:   "G4",
		Desc: fmt.Sprintf("memory: resume rate ≥ %.2f and repeat-read ratio ≤ ghx", g4ResumeRate),
		Pass: sc.MultiTurnEpisodes > 0 && sc.ResumeRate >= g4ResumeRate && sc.RepeatReadRatio <= gx.RepeatReadRatio+1e-9,
		Detail: fmt.Sprintf("resume %.2f over %d multi-turn episodes; repeat-read sidecar %.3f vs ghx %.3f",
			sc.ResumeRate, sc.MultiTurnEpisodes, sc.RepeatReadRatio, gx.RepeatReadRatio),
	}

	g5 := GateResult{
		ID:     "G5",
		Desc:   "safety: 1.0 on every sidecar episode",
		Pass:   sc.Episodes > 0 && sc.MeanSafety == 1.0,
		Detail: fmt.Sprintf("mean safety %.3f over %d episodes", sc.MeanSafety, sc.Episodes),
	}

	v.Gates = []GateResult{g1, g2, g3, g4, g5}

	// Verdict rules from ADR-0016.1: thesis supported iff G1, G2, G3, G5 pass.
	// G4 does not overturn the thesis but blocks ADR-0017 until the evidence
	// ledger exists. ADR-0016.8 D3: a below-minimum sample additionally caps
	// the verdict at PRELIMINARY — thesisSupported requires DataSufficient.
	v.ThesisSupported = v.Valid && v.DataSufficient && g1.Pass && g2.Pass && g3.Pass && g5.Pass
	if !v.DataSufficient && v.Valid && g1.Pass && g2.Pass && g3.Pass && g5.Pass {
		v.Notes = append(v.Notes,
			"PRELIMINARY: all thesis gates pass but the sample is below the pre-registered gate-run minimums — thesisSupported stays false until a sufficient run measures it (ADR-0016.8 D3)")
	}
	if !g4.Pass {
		v.Notes = append(v.Notes,
			"G4 failed: session resumption/memory is not proven — ADR-0017 framework standardization is blocked until the ADR-0014.1 evidence ledger lands")
	}
	if !g5.Pass && sc.Episodes > 0 {
		v.Notes = append(v.Notes,
			"G5 failed: safety violation on a sidecar episode is a contract bug that invalidates the run")
	}
	return v
}

func validateEpisodesForVerdict(episodes []*Episode) ([]*Episode, []string, bool) {
	valid := true
	var notes []string
	var filtered []*Episode
	identityKeys := map[string]int{}

	for _, ep := range episodes {
		if ep == nil {
			continue
		}
		key := identityKey(ep.Identity)
		if key != "" {
			identityKeys[key]++
		}
		if strings.TrimSpace(ep.Identity.SubjectModel) == "" || ep.Identity.SubjectModel == "unknown" {
			notes = append(notes, fmt.Sprintf("IDENTITY: subject-model identity is unverified for episode %s", episodeLabel(ep)))
		}
		exclude := ep.Invalid
		if ep.Profile == ProfilePlain && invokesGhx(ep) {
			exclude = true
			ep.Invalid = true
			reason := "COMPLIANCE: plain profile invoked ghx"
			if !containsString(ep.ExclusionReasons, reason) {
				ep.ExclusionReasons = append(ep.ExclusionReasons, reason)
			}
			notes = append(notes, fmt.Sprintf("%s in episode %s — excluded from gate aggregates", reason, episodeLabel(ep)))
		}
		if ep.Profile == ProfileGhx && !invokesGhx(ep) {
			notes = append(notes, fmt.Sprintf("COMPLIANCE: ghx profile episode %s recorded zero ghx invocations", episodeLabel(ep)))
		}
		if exclude {
			valid = false
			for _, reason := range ep.ExclusionReasons {
				if !strings.Contains(reason, "plain profile invoked ghx") {
					notes = append(notes, fmt.Sprintf("COMPLIANCE: episode %s excluded: %s", episodeLabel(ep), reason))
				}
			}
			continue
		}
		// Contamination guard (ADR-0016.8 D2): an episode that read a
		// pre-registered answer-bearing doc is excluded from gate aggregates
		// and listed in the verdict, like BLOCKED episodes in the anomaly
		// table. It does not invalidate the run — the remaining episodes
		// still measure exploration.
		if detail, contaminated := answerDocContamination(ep); contaminated {
			notes = append(notes, fmt.Sprintf(
				"CONTAMINATION: episode %s excluded from gate aggregates — %s (answer_doc_contamination)",
				episodeLabel(ep), detail))
			continue
		}
		filtered = append(filtered, ep)
	}

	if len(identityKeys) > 1 {
		valid = false
		notes = append(notes, fmt.Sprintf("IDENTITY: mixed agent identities in one run (%d distinct identities) — verdict invalid", len(identityKeys)))
	}
	return filtered, notes, valid
}

// answerDocContamination reports whether the episode carries the
// answer_doc_contamination anomaly (ADR-0016.8 D2), with the first
// offending detail for the verdict note. Derived from the artifact via
// DetectAnomalies so the exclusion is recomputable offline.
func answerDocContamination(ep *Episode) (string, bool) {
	for _, a := range DetectAnomalies(ep) {
		if a.Kind == AnomalyAnswerDocContamination {
			return a.Detail, true
		}
	}
	return "", false
}

func identityKey(id AgentIdentity) string {
	parts := []string{id.AgentCommand, id.AdapterName, id.AdapterVersion, id.SubjectModel, id.AdapterSubjectModel, id.WrapperSHA256}
	allEmpty := true
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			allEmpty = false
			break
		}
	}
	if allEmpty {
		return ""
	}
	return strings.Join(parts, "\x00")
}

func episodeLabel(ep *Episode) string {
	if ep.ID != "" {
		return ep.ID
	}
	return ep.TaskID + "/" + string(ep.Profile)
}

func containsString(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

// sampleSufficiency checks the episode set against the pre-registered
// gate-run minimums and returns whether they are met plus caveat notes for
// each shortfall (ADR-0016.2). The repo minimum enforces ADR-0016.1's
// "≥ 6 tasks across ≥ 3 repos" — six tasks on one repo would measure that
// repo, not the tool (ADR-0016.8 D4).
func sampleSufficiency(episodes []*Episode) (bool, []string) {
	tasks := map[string]bool{}
	repos := map[string]bool{}
	multiTurnTasks := map[string]bool{}
	trials := map[string]int{} // taskID + "/" + profile → episode count
	for _, ep := range episodes {
		tasks[ep.TaskID] = true
		if ep.Repo != "" {
			repos[ep.Repo] = true
		}
		if len(ep.Turns) > 1 {
			multiTurnTasks[ep.TaskID] = true
		}
		trials[ep.TaskID+"/"+string(ep.Profile)]++
	}

	minTrials := 0
	if len(tasks) > 0 {
		minTrials = int(^uint(0) >> 1)
		for id := range tasks {
			for _, p := range AllProfiles() {
				if n := trials[id+"/"+string(p)]; n < minTrials {
					minTrials = n
				}
			}
		}
	}

	var notes []string
	if len(tasks) < minGateRunTasks {
		notes = append(notes, fmt.Sprintf(
			"PRELIMINARY: %d distinct tasks < gate-run minimum %d", len(tasks), minGateRunTasks))
	}
	if len(repos) < minGateRunRepos {
		notes = append(notes, fmt.Sprintf(
			"PRELIMINARY: %d distinct repos < gate-run minimum %d (ADR-0016.1 requires ≥ %d tasks across ≥ %d repos)",
			len(repos), minGateRunRepos, minGateRunTasks, minGateRunRepos))
	}
	if minTrials < minGateRunTrials {
		notes = append(notes, fmt.Sprintf(
			"PRELIMINARY: smallest task × profile cell has %d trials < gate-run minimum %d", minTrials, minGateRunTrials))
	}
	if len(multiTurnTasks) < minGateRunMultiTurnTasks {
		notes = append(notes, fmt.Sprintf(
			"PRELIMINARY: %d multi-turn tasks < gate-run minimum %d", len(multiTurnTasks), minGateRunMultiTurnTasks))
	}
	return len(notes) == 0, notes
}

// dataQualityNotes emits warnings (not gate failures) for measurement
// conditions that bias specific gates (ADR-0016.2).
func dataQualityNotes(episodes []*Episode, agg map[Profile]*ProfileAggregate) []string {
	var notes []string

	directNoOutputs := 0
	emptySidecarReports := 0
	taskProfileCounts := map[string]map[Profile]int{}
	for _, ep := range episodes {
		if tp := taskProfileCounts[ep.TaskID]; tp == nil {
			taskProfileCounts[ep.TaskID] = map[Profile]int{}
		}
		taskProfileCounts[ep.TaskID][ep.Profile]++

		if ep.Profile != ProfileSidecar {
			toolCalls, outputChars := 0, 0
			for _, t := range ep.Turns {
				toolCalls += len(t.ToolCalls)
				outputChars += t.ToolOutputChars
			}
			if toolCalls > 0 && outputChars == 0 {
				directNoOutputs++
			}
			continue
		}
		if ep.Report == nil || strings.TrimSpace(ep.Report.Answer) == "" {
			emptySidecarReports++
		}
	}

	if directNoOutputs > 0 {
		notes = append(notes, fmt.Sprintf(
			"DATA QUALITY: %d direct-profile episode(s) ran tools but recorded zero tool-output chars — the adapter is not reporting outputs, which undercounts baseline context and biases G3 toward the sidecar; a gate run in this state is invalid for G3",
			directNoOutputs))
	}
	if emptySidecarReports > 0 {
		notes = append(notes, fmt.Sprintf(
			"DATA QUALITY: %d sidecar episode(s) produced no/empty report — their near-zero main-agent chars deflate the G3 mean without delivering an answer",
			emptySidecarReports))
	}

	unbalanced := 0
	for _, counts := range taskProfileCounts {
		base := -1
		for _, p := range AllProfiles() {
			if base == -1 {
				base = counts[p]
			} else if counts[p] != base {
				unbalanced++
				break
			}
		}
	}
	if unbalanced > 0 {
		notes = append(notes, fmt.Sprintf(
			"DATA QUALITY: %d task(s) have unequal episode counts across profiles — unweighted means skew G1/G3",
			unbalanced))
	}

	gx := agg[ProfileGhx]
	if gx != nil && gx.Episodes > 0 && gx.MeanCorrectness < g1AbsFloor {
		notes = append(notes, fmt.Sprintf(
			"DATA QUALITY: ghx baseline mean correctness %.3f is below the G1 absolute floor %.2f — the relative G1 comparison is vacuous against a collapsed baseline",
			gx.MeanCorrectness, g1AbsFloor))
	}
	return notes
}

// LoadRunEpisodes reads every episode artifact (*.json, excluding verdict
// files) from a run directory.
func LoadRunEpisodes(runDir string) ([]*Episode, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && !strings.HasPrefix(e.Name(), "verdict") && e.Name() != "manifest.json" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var eps []*Episode
	for _, name := range names {
		ep, err := LoadEpisode(filepath.Join(runDir, name))
		if err != nil {
			return nil, err
		}
		eps = append(eps, ep)
	}
	return eps, nil
}

// SaveVerdict writes verdict.json and verdict.md into the run directory,
// returning the markdown path. These files plus the episode artifacts are
// what a gate run commits under docs/evals/<run-id>/.
func SaveVerdict(runDir string, v Verdict) (string, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(runDir, "verdict.json"), data, 0o644); err != nil {
		return "", err
	}
	mdPath := filepath.Join(runDir, "verdict.md")
	return mdPath, os.WriteFile(mdPath, []byte(FormatVerdict(v)), 0o644)
}

// thesisGatesPass reports whether every thesis gate (G1, G2, G3, G5 — G4
// blocks ADR-0017 but does not overturn the thesis) passed.
func thesisGatesPass(gates []GateResult) bool {
	for _, g := range gates {
		if g.ID == "G4" {
			continue
		}
		if !g.Pass {
			return false
		}
	}
	return len(gates) > 0
}

// FormatVerdict renders the verdict as the markdown summary committed with
// gate runs.
func FormatVerdict(v Verdict) string {
	var sb strings.Builder
	sb.WriteString("# Sidecar Eval Verdict (ADR-0016.1)\n\n")

	sb.WriteString("## Profiles\n\n")
	sb.WriteString("| profile | episodes | correctness | evidence | trajectory | compression | safety | main-agent chars | resume rate | repeat reads |\n")
	sb.WriteString("|---------|----------|-------------|----------|------------|-------------|--------|------------------|-------------|--------------|\n")
	for _, p := range AllProfiles() {
		a := v.Aggregates[p]
		if a == nil {
			continue
		}
		fmt.Fprintf(&sb, "| %s | %d | %.3f | %.3f | %.3f | %.3f | %.3f | %.0f | %.2f | %.3f |\n",
			a.Profile, a.Episodes, a.MeanCorrectness, a.MeanEvidence, a.MeanTrajectory,
			a.MeanCompression, a.MeanSafety, a.MeanMainAgentChars, a.ResumeRate, a.RepeatReadRatio)
	}

	sb.WriteString("\n## Gates\n\n")
	sb.WriteString("| gate | check | result | detail |\n|------|-------|--------|--------|\n")
	for _, g := range v.Gates {
		status := "FAIL"
		if g.Pass {
			status = "PASS"
		}
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", g.ID, g.Desc, status, g.Detail)
	}

	if len(v.Anomalies) > 0 {
		sb.WriteString("\n## Anomalies\n\n")
		sb.WriteString("| kind | severity | count | episodes |\n|------|----------|-------|----------|\n")
		for _, a := range v.Anomalies {
			fmt.Fprintf(&sb, "| %s | %s | %d | %d |\n", a.Kind, a.Severity, a.Count, a.Episodes)
		}
	}

	sb.WriteString("\n## Verdict\n\n")
	prefix := ""
	if v.Preliminary {
		prefix = "**PRELIMINARY (below pre-registered gate-run sample — not the project verdict)** "
	}
	if !v.Valid {
		prefix += "**INVALID (compliance/identity checks failed)** "
	}
	switch {
	case v.ThesisSupported:
		sb.WriteString(prefix + "**THESIS SUPPORTED** — G1, G2, G3, G5 pass.\n")
	case v.Valid && v.Preliminary && thesisGatesPass(v.Gates):
		// ADR-0016.8 D3: gates pass but the sample is below contract — the
		// JSON says thesisSupported=false, and the markdown must not imply
		// gate failures that did not happen.
		sb.WriteString(prefix + "**THESIS NOT SUPPORTED** — thesis gates pass, but the sample is below the pre-registered gate-run minimums; rerun at contract size for a citable verdict.\n")
	default:
		sb.WriteString(prefix + "**THESIS NOT SUPPORTED** — see failed gates above.\n")
	}
	for _, n := range v.Notes {
		fmt.Fprintf(&sb, "\n- %s\n", n)
	}
	if v.Stopping != nil {
		sb.WriteString(formatStoppingSection(v.Stopping))
	}
	sb.WriteString("\nCaveats (ADR-0016.2): `overall` is not comparable across profiles " +
		"(compression is 0 by construction for direct profiles). G3 is meaningful " +
		"only jointly with G1/G2 — a tiny useless report maximizes compression.\n")
	return sb.String()
}
