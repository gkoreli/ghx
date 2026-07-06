// Command gen assembles the judge gold-set candidate list (ADR-0023.1 D5,
// sequencing step 2) from the two committed gate runs, applying the same D7
// anomaly pre-filter the judge runner uses, and writes CANDIDATES.md plus
// blinded labeling packets.
//
// Everything here is deterministic: the same committed run directories always
// produce byte-identical output, so the candidate list and every labeling
// packet are recomputable evidence, not hand-maintained tables.
//
// Usage (from the repo root):
//
//	go run ./docs/evals/judge-goldset/gen                 # rewrite CANDIDATES.md
//	go run ./docs/evals/judge-goldset/gen -packet gs-042  # write one labeling packet
//	go run ./docs/evals/judge-goldset/gen -packets        # write packets for the recommended batch
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals"
)

// The two committed gate runs the gold set draws from (ADR-0023.1 D5).
var defaultRuns = []string{
	"docs/evals/gate-run-2026-07-05-confirmatory",
	"docs/evals/gate-run-2026-07-05-d2-0021-0022-partial",
}

const (
	defaultTasksDir  = "internal/sidecar/evals/testdata/tasks"
	defaultOut       = "docs/evals/judge-goldset/CANDIDATES.md"
	defaultPacketDir = "docs/evals/judge-goldset/labels/bundles"
	protocolVersion  = "goldset-protocol-v1"

	// Reward-band cutoffs on the committed deterministic rewards.overall,
	// chosen once from the combined distribution (p25 ≈ 0.75, p75 ≈ 0.85)
	// so bands split the pool roughly 25/50/25. Used only for stratified
	// sampling — never shown to the labeler.
	bandMidMin  = 0.75
	bandHighMin = 0.85

	// Recommended first labeling batch: perTask per task across all tasks.
	perTask = 5
)

type candidate struct {
	LabelID   string
	EpisodeID string
	Path      string // repo-relative episode file path
	Run       string // run directory base name
	TaskID    string
	Turns     int
	Band      string
	Overall   float64
	Hash      string // sha256(EpisodeID) hex — blinded ordering key
	InBatch   bool
}

type exclusion struct {
	EpisodeID string
	Path      string
	Run       string
	Reason    string
}

type runStats struct {
	Run        string
	Episodes   int
	Invalid    int
	Blocked    int
	WarnNoRep  int
	Candidates int
}

// labelPacket is the blinded labeling packet: exactly what the judge prompt
// contains for one episode (task criteria + profile-blind JudgeBundle), keyed
// by the blinded label ID. It carries no episode ID, file path, run name,
// reward score, or profile label.
type labelPacket struct {
	LabelID         string            `json:"labelId"`
	ProtocolVersion string            `json:"protocolVersion"`
	RubricVersion   string            `json:"rubricVersion"`
	TaskCriteria    []string          `json:"taskCriteria,omitempty"`
	Bundle          evals.JudgeBundle `json:"bundle"`
}

func main() {
	out := flag.String("out", defaultOut, "CANDIDATES.md output path")
	tasksDir := flag.String("tasks", defaultTasksDir, "task definitions directory")
	packet := flag.String("packet", "", "write one labeling packet for the given label id (e.g. gs-042)")
	packets := flag.Bool("packets", false, "write labeling packets for the recommended batch")
	packetDir := flag.String("packet-dir", defaultPacketDir, "directory for labeling packets")
	flag.Parse()

	runs := flag.Args()
	if len(runs) == 0 {
		runs = defaultRuns
	}

	tasks, err := evals.LoadTasks(*tasksDir)
	fatal(err)
	taskByID := map[string]evals.Task{}
	for _, t := range tasks {
		taskByID[t.ID] = t
	}

	var cands []candidate
	var excls []exclusion
	var stats []runStats
	episodeByID := map[string]*evals.Episode{}

	for _, runDir := range runs {
		eps, err := evals.LoadRunEpisodes(runDir)
		fatal(err)
		// EvaluateGates re-derives gate-time validity (including the
		// plain-profile ghx-compliance exclusion) on the loaded episodes,
		// mutating ep.Invalid/ep.ExclusionReasons exactly as a verdict run
		// would. The D7 pre-filter below then reads that state.
		evals.EvaluateGates(eps)

		st := runStats{Run: filepath.Base(runDir), Episodes: len(eps)}
		for _, ep := range eps {
			path := filepath.ToSlash(filepath.Join(runDir, ep.ID+".json"))
			episodeByID[ep.ID] = ep
			if reason, kind, skip := prefilter(ep); skip {
				switch kind {
				case "invalid":
					st.Invalid++
				case "blocked":
					st.Blocked++
				case "warn-noreport":
					st.WarnNoRep++
				}
				excls = append(excls, exclusion{EpisodeID: ep.ID, Path: path, Run: st.Run, Reason: reason})
				continue
			}
			st.Candidates++
			cands = append(cands, candidate{
				EpisodeID: ep.ID,
				Path:      path,
				Run:       st.Run,
				TaskID:    ep.TaskID,
				Turns:     len(ep.Turns),
				Band:      band(ep.Rewards.Overall),
				Overall:   ep.Rewards.Overall,
				Hash:      hashID(ep.ID),
			})
		}
		stats = append(stats, st)
	}

	// Blinded label IDs: assigned in sha256(episodeID) order across the
	// combined pool, so gs-NNN order does not mirror task/profile/run order.
	sort.Slice(cands, func(i, j int) bool { return cands[i].Hash < cands[j].Hash })
	for i := range cands {
		cands[i].LabelID = fmt.Sprintf("gs-%03d", i+1)
	}
	markBatch(cands)

	sort.Slice(excls, func(i, j int) bool { return excls[i].EpisodeID < excls[j].EpisodeID })

	switch {
	case *packet != "":
		fatal(writePackets(cands, taskByID, episodeByID, *packetDir, func(c candidate) bool { return c.LabelID == *packet }))
	case *packets:
		fatal(writePackets(cands, taskByID, episodeByID, *packetDir, func(c candidate) bool { return c.InBatch }))
	default:
		md := renderCandidatesMD(runs, stats, cands, excls)
		fatal(os.MkdirAll(filepath.Dir(*out), 0o755))
		fatal(os.WriteFile(*out, []byte(md), 0o644))
		fmt.Printf("wrote %s (%d candidates, %d excluded)\n", *out, len(cands), len(excls))
	}
}

// prefilter mirrors the unexported judgePrefilter in
// internal/sidecar/evals/judge.go (ADR-0023.1 D7): invalid episodes first,
// then the first BLOCKED / WARN-noreport anomaly in detection order. Keep the
// two in sync — this generator exists so the labeled pool and the judged pool
// are the same pool.
func prefilter(ep *evals.Episode) (reason, kind string, skip bool) {
	if ep.Invalid {
		r := "episode excluded from the run (invalid)"
		if len(ep.ExclusionReasons) > 0 {
			r = "episode excluded: " + ep.ExclusionReasons[0]
		}
		return r, "invalid", true
	}
	for _, a := range evals.DetectAnomalies(ep) {
		switch a.Kind {
		case evals.AnomalySidecarBlocked:
			return "BLOCKED episode — zero exploration to judge (D7)", "blocked", true
		case evals.AnomalySidecarReportMissing:
			return "WARN-noreport episode — no report to judge (D7)", "warn-noreport", true
		}
	}
	return "", "", false
}

func band(overall float64) string {
	switch {
	case overall >= bandHighMin:
		return "high"
	case overall >= bandMidMin:
		return "mid"
	default:
		return "low"
	}
}

func hashID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

// markBatch selects the recommended first labeling batch: perTask episodes
// per task, round-robining the low → mid → high reward bands (hash order
// within a band) so every task contributes and every populated score range is
// covered. Deterministic given the candidate pool.
func markBatch(cands []candidate) {
	byTask := map[string][]*candidate{}
	var taskIDs []string
	for i := range cands {
		c := &cands[i]
		if len(byTask[c.TaskID]) == 0 {
			taskIDs = append(taskIDs, c.TaskID)
		}
		byTask[c.TaskID] = append(byTask[c.TaskID], c)
	}
	sort.Strings(taskIDs)
	for _, tid := range taskIDs {
		byBand := map[string][]*candidate{}
		for _, c := range byTask[tid] {
			byBand[c.Band] = append(byBand[c.Band], c)
		}
		for _, cs := range byBand {
			sort.Slice(cs, func(i, j int) bool { return cs[i].Hash < cs[j].Hash })
		}
		picked := 0
		for picked < perTask {
			progressed := false
			for _, b := range []string{"low", "mid", "high"} {
				if picked >= perTask {
					break
				}
				if len(byBand[b]) == 0 {
					continue
				}
				byBand[b][0].InBatch = true
				byBand[b] = byBand[b][1:]
				picked++
				progressed = true
			}
			if !progressed {
				break // task exhausted
			}
		}
	}
}

func writePackets(cands []candidate, tasks map[string]evals.Task, eps map[string]*evals.Episode, dir string, want func(candidate) bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	wrote := 0
	for _, c := range cands {
		if !want(c) {
			continue
		}
		ep := eps[c.EpisodeID]
		task, ok := tasks[c.TaskID]
		if !ok {
			return fmt.Errorf("no task definition for %s (episode %s)", c.TaskID, c.EpisodeID)
		}
		p := labelPacket{
			LabelID:         c.LabelID,
			ProtocolVersion: protocolVersion,
			RubricVersion:   evals.CoreRubricSpec().Version,
			Bundle:          evals.BuildJudgeBundle(ep),
		}
		if task.Judge != nil {
			p.TaskCriteria = append(p.TaskCriteria, task.Judge.Criteria...)
		}
		data, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		path := filepath.Join(dir, c.LabelID+".bundle.json")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
		wrote++
	}
	if wrote == 0 {
		return fmt.Errorf("no candidate matched")
	}
	return nil
}

func renderCandidatesMD(runs []string, stats []runStats, cands []candidate, excls []exclusion) string {
	var sb strings.Builder
	sb.WriteString("# Judge Gold-Set Candidates\n\n")
	sb.WriteString("GENERATED FILE — do not edit by hand. Regenerate from the repo root with:\n\n")
	sb.WriteString("```bash\ngo run ./docs/evals/judge-goldset/gen\n```\n\n")
	sb.WriteString("Candidate pool for the ADR-0023.1 D5 gold-set labeling protocol (see\n")
	sb.WriteString("`PROTOCOL.md` in this directory). Drawn from the committed gate runs:\n\n")
	for _, r := range runs {
		fmt.Fprintf(&sb, "- `%s`\n", r)
	}
	sb.WriteString("\nThe D7 anomaly pre-filter (mirroring `judgePrefilter` in\n")
	sb.WriteString("`internal/sidecar/evals/judge.go`) excludes invalid (compliance-excluded),\n")
	sb.WriteString("BLOCKED, and WARN-noreport episodes — the same episodes the judge runner\n")
	sb.WriteString("skips, so human labels and judge scores cover the same pool.\n\n")

	sb.WriteString("## Counts (recomputed by this generator)\n\n")
	sb.WriteString("| run | episodes | invalid | BLOCKED | WARN-noreport | candidates |\n")
	sb.WriteString("| --- | ---: | ---: | ---: | ---: | ---: |\n")
	total := runStats{Run: "**total**"}
	for _, st := range stats {
		fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d | %d |\n",
			st.Run, st.Episodes, st.Invalid, st.Blocked, st.WarnNoRep, st.Candidates)
		total.Episodes += st.Episodes
		total.Invalid += st.Invalid
		total.Blocked += st.Blocked
		total.WarnNoRep += st.WarnNoRep
		total.Candidates += st.Candidates
	}
	fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d | %d |\n\n",
		total.Run, total.Episodes, total.Invalid, total.Blocked, total.WarnNoRep, total.Candidates)

	sb.WriteString("## Excluded episodes\n\n")
	if len(excls) == 0 {
		sb.WriteString("(none)\n\n")
	} else {
		sb.WriteString("| episode file | run | reason |\n")
		sb.WriteString("| --- | --- | --- |\n")
		for _, e := range excls {
			fmt.Fprintf(&sb, "| `%s` | %s | %s |\n", e.Path, e.Run, e.Reason)
		}
		sb.WriteString("\n")
	}

	batch := 0
	multi := 0
	bands := map[string]int{}
	for _, c := range cands {
		if c.InBatch {
			batch++
		}
		if c.Turns > 1 {
			multi++
		}
		bands[c.Band]++
	}
	sb.WriteString("## Candidates\n\n")
	fmt.Fprintf(&sb, "%d candidates (%d multi-turn; reward bands: low %d / mid %d / high %d;\n",
		len(cands), multi, bands["low"], bands["mid"], bands["high"])
	fmt.Fprintf(&sb, "band cutoffs on deterministic `rewards.overall`: low < %.2f ≤ mid < %.2f ≤ high).\n\n", bandMidMin, bandHighMin)
	sb.WriteString("`label id` is the blinded ID used during labeling sessions (assigned in\n")
	sb.WriteString("sha256(episode-id) order so it does not mirror task/profile/run order).\n")
	sb.WriteString("The `stratum` tag is `task/turns/band` — sample labeling sessions evenly\n")
	sb.WriteString("across strata. Labelers: do NOT read this table during a session (the file\n")
	sb.WriteString("paths reveal the profile); see PROTOCOL.md.\n\n")
	sb.WriteString("| label id | episode file | task | turns | stratum | batch-1 |\n")
	sb.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, c := range cands {
		turns := "single"
		if c.Turns > 1 {
			turns = "multi"
		}
		mark := ""
		if c.InBatch {
			mark = "x"
		}
		fmt.Fprintf(&sb, "| %s | `%s` | %s | %s | `%s/%s/%s` | %s |\n",
			c.LabelID, c.Path, c.TaskID, turns, c.TaskID, turns, c.Band, mark)
	}
	sb.WriteString("\n")

	sb.WriteString("## Recommended first labeling batch\n\n")
	fmt.Fprintf(&sb, "%d episodes (marked `batch-1` above): %d per task, round-robining the\n", batch, perTask)
	sb.WriteString("low → mid → high reward bands within each task so every task and every\n")
	sb.WriteString("populated score range is covered. Generate the blinded labeling packets with:\n\n")
	sb.WriteString("```bash\ngo run ./docs/evals/judge-goldset/gen -packets\n```\n\n")
	var ids []string
	for _, c := range cands {
		if c.InBatch {
			ids = append(ids, c.LabelID)
		}
	}
	fmt.Fprintf(&sb, "Batch label IDs: %s\n", strings.Join(ids, ", "))
	return sb.String()
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
