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
	Aggregates      map[Profile]*ProfileAggregate `json:"aggregates"`
	Gates           []GateResult                  `json:"gates"`
	ThesisSupported bool                          `json:"thesisSupported"`
	// Notes carries verdict-rule context (e.g. G4 blocking ADR-0017) and
	// data-sufficiency caveats.
	Notes []string `json:"notes"`
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
	firstTurn := map[string]bool{}
	for _, tok := range pathTokens(strings.ToLower(strings.Join(ep.Turns[0].ToolCalls, "\n"))) {
		firstTurn[tok] = true
	}
	var later []string
	for _, t := range ep.Turns[1:] {
		later = append(later, pathTokens(strings.ToLower(strings.Join(t.ToolCalls, "\n")))...)
	}
	if len(later) == 0 {
		return 0
	}
	repeats := 0
	for _, tok := range later {
		if firstTurn[tok] {
			repeats++
		}
	}
	return float64(repeats) / float64(len(later))
}

// EvaluateGates applies the pre-registered ADR-0016.1 gates to a set of
// episodes and returns the verdict. It works on whatever episodes exist and
// reports data-sufficiency caveats in Notes; the formal gate run requires
// ≥ 6 tasks × 5 trials × 3 profiles.
func EvaluateGates(episodes []*Episode) Verdict {
	agg := Aggregate(episodes)
	sc := agg[ProfileSidecar]
	gx := agg[ProfileGhx]

	v := Verdict{Aggregates: agg}

	if sc.Episodes == 0 || gx.Episodes == 0 {
		v.Notes = append(v.Notes, fmt.Sprintf(
			"INSUFFICIENT DATA: sidecar episodes=%d, ghx episodes=%d — gates need both profiles present",
			sc.Episodes, gx.Episodes))
	}

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
	// ledger exists.
	v.ThesisSupported = g1.Pass && g2.Pass && g3.Pass && g5.Pass
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

// LoadRunEpisodes reads every episode artifact (*.json, excluding verdict
// files) from a run directory.
func LoadRunEpisodes(runDir string) ([]*Episode, error) {
	entries, err := os.ReadDir(runDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && !strings.HasPrefix(e.Name(), "verdict") {
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

	sb.WriteString("\n## Verdict\n\n")
	if v.ThesisSupported {
		sb.WriteString("**THESIS SUPPORTED** — G1, G2, G3, G5 pass.\n")
	} else {
		sb.WriteString("**THESIS NOT SUPPORTED** — see failed gates above.\n")
	}
	for _, n := range v.Notes {
		fmt.Fprintf(&sb, "\n- %s\n", n)
	}
	return sb.String()
}
