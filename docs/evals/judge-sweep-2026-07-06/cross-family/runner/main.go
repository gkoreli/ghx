// Command crossfamily re-scores the judge-sweep-2026-07-06 DISAGREEMENT SET
// ONLY — the 27 "gates fail, judge good" episodes flagged by the first sweep
// — on the committed PRIMARY judge rail (gpt-5.5 via `codex exec`,
// cross-family: the subject agents are all Claude models, so a GPT judge has
// no self-preference confound on them). This is the queued TRUST H1
// follow-up: the first sweep used the SECONDARY (claude-opus-4-8) rail only
// because the primary rail was out of tokens at the time, so its 19.3%
// disagreement rate carried a same-family confound.
//
// ── WHAT THIS DOES NOT DO ────────────────────────────────────────────────────
//   - It does not touch the measurement stack: judge prompt, rubric, parse
//     logic, gate thresholds, and reward computation are all unchanged. This
//     command only calls the pre-existing evals.JudgeRunner /
//     evals.NewPrimaryCLIJudgeClient machinery (internal/sidecar/evals) with
//     the committed config (judgeconfig/judge-config-v1.json) exactly as
//     production would.
//   - It does not re-score the other 113 episodes from the first sweep. The
//     candidate set is read back from the committed
//     docs/evals/judge-sweep-2026-07-06/SUMMARY.md "Episodes to hand-review"
//     list (the exact same 27 IDs the first sweep flagged), not recomputed
//     from thresholds duplicated in this file — avoiding constant drift.
//   - It does not run any live sidecar episodes; all scoring is over the two
//     already-committed gate runs.
//
// Samples: k=3 per candidate (the config default; the first sweep used k=1
// for economy). ~27 candidates * k=3 = ~81 codex calls.
//
// Usage (from the repo root):
//
//	go run ./docs/evals/judge-sweep-2026-07-06/cross-family/runner
//	go run ./docs/evals/judge-sweep-2026-07-06/cross-family/runner -limit 5   # partial batch, resumable
//	go run ./docs/evals/judge-sweep-2026-07-06/cross-family/runner -summary-only
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals"
)

// The two committed gate runs the disagreement-set episodes come from (must
// match docs/evals/judge-sweep-2026-07-06/sweep/main.go's defaultRuns — same
// runs, so episode IDs line up).
var defaultRuns = []string{
	"docs/evals/gate-run-2026-07-05-confirmatory",
	"docs/evals/gate-run-2026-07-05-d2-0021-0022-partial",
}

const (
	defaultTasksDir      = "internal/sidecar/evals/testdata/tasks"
	defaultOutDir        = "docs/evals/judge-sweep-2026-07-06/cross-family"
	defaultFirstSummary  = "docs/evals/judge-sweep-2026-07-06/SUMMARY.md"
	defaultFirstSweepDir = "docs/evals/judge-sweep-2026-07-06"

	// judgeSamples is k for this re-score. The committed config default is 3
	// (judgeconfig/judge-config-v1.json .samples); the first sweep deviated to
	// k=1 for economy. This follow-up restores k=3 for self-consistency.
	judgeSamples = 3

	// judgePoorThreshold mirrors the committed constant in
	// internal/sidecar/evals/judge_disagreement.go — used only to compute the
	// per-sample majority verdict for this report, never to re-derive the
	// candidate set itself (that comes from the committed SUMMARY.md).
	judgePoorThreshold = 2.5
)

// disagreementLineRE matches one "Episodes to hand-review" bullet from the
// committed first-sweep SUMMARY.md, e.g.:
//
//   - express-router-location_ghx-sidecar_1783294920428 — gates fail, judge overall 4.00 (good)
var disagreementLineRE = regexp.MustCompile(`^- (\S+) — gates fail, judge overall ([0-9.]+) \(good\)$`)

type candidate struct {
	EpisodeID       string
	SameFamilyScore float64
}

// resolved bundles a parsed candidate with its loaded episode and the first
// sweep's same-family judge result, so the scoring loop and the summary
// builder share one shape (package-level so it can cross function
// boundaries without an anonymous-struct conversion).
type resolved struct {
	candidate
	ep         *evals.Episode
	sameFamily *evals.JudgeResult
}

// parseDisagreementCandidates reads the first sweep's committed SUMMARY.md and
// extracts the exact "gates fail, judge good" candidate list it flagged —
// reusing the committed report's own disagreement computation rather than
// duplicating gate-floor / poor-threshold constants in a second place.
func parseDisagreementCandidates(summaryPath string) ([]candidate, error) {
	data, err := os.ReadFile(summaryPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", summaryPath, err)
	}
	var out []candidate
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		m := disagreementLineRE.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		id := m[1]
		if seen[id] {
			continue // the appendix table below repeats the same info; only take the first (hand-review list) occurrence
		}
		seen[id] = true
		score, err := strconv.ParseFloat(m[2], 64)
		if err != nil {
			return nil, fmt.Errorf("parse judge overall %q for %s: %w", m[2], id, err)
		}
		out = append(out, candidate{EpisodeID: id, SameFamilyScore: score})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no disagreement candidates parsed from %s", summaryPath)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EpisodeID < out[j].EpisodeID })
	return out, nil
}

func main() {
	tasksDir := flag.String("tasks", defaultTasksDir, "task definitions directory")
	outDir := flag.String("out", defaultOutDir, "cross-family artifact directory")
	firstSummary := flag.String("first-summary", defaultFirstSummary, "first sweep's committed SUMMARY.md (candidate source)")
	parallel := flag.Int("parallel", 3, "concurrent judge calls")
	limit := flag.Int("limit", 0, "max candidates to score this invocation (0 = all pending)")
	callTimeout := flag.Duration("call-timeout", 6*time.Minute, "per judge-call timeout")
	summaryOnly := flag.Bool("summary-only", false, "skip scoring; rebuild cross-family SUMMARY.md from existing artifacts")
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

	var episodes []*evals.Episode
	for _, runDir := range runs {
		eps, err := evals.LoadRunEpisodes(runDir)
		fatal(err)
		evals.EvaluateGates(eps)
		episodes = append(episodes, eps...)
	}
	epByID := map[string]*evals.Episode{}
	for _, ep := range episodes {
		epByID[ep.ID] = ep
	}

	candidates, err := parseDisagreementCandidates(*firstSummary)
	fatal(err)

	// Resolve each candidate's episode + first-sweep (same-family) judge
	// result for the report.
	var resolvedCands []resolved
	for _, c := range candidates {
		ep, ok := epByID[c.EpisodeID]
		if !ok {
			fatal(fmt.Errorf("candidate %s not found in loaded runs %v", c.EpisodeID, runs))
		}
		sfPath := filepath.Join(defaultFirstSweepDir, c.EpisodeID+".judge.json")
		sf, err := evals.LoadJudgeResult(sfPath)
		fatal(err)
		resolvedCands = append(resolvedCands, resolved{candidate: c, ep: ep, sameFamily: sf})
	}

	fatal(os.MkdirAll(*outDir, 0o755))

	// Resume: existing cross-family artifacts are evidence, never re-scored.
	results := map[string]*evals.JudgeResult{}
	var pending []resolved
	for _, rc := range resolvedCands {
		path := filepath.Join(*outDir, rc.EpisodeID+".judge.json")
		if _, statErr := os.Stat(path); statErr == nil {
			res, loadErr := evals.LoadJudgeResult(path)
			fatal(loadErr)
			results[rc.EpisodeID] = res
			continue
		}
		pending = append(pending, rc)
	}
	resumed := len(results)

	failures := map[string]string{}
	scoredNow := 0

	if !*summaryOnly {
		todo := pending
		if *limit > 0 && len(todo) > *limit {
			todo = todo[:*limit]
		}
		fmt.Printf("cross-family: %d candidates total, %d already scored (resumed), %d pending, %d in this batch\n",
			len(resolvedCands), resumed, len(pending), len(todo))

		cfg, err := evals.DefaultJudgeConfig()
		fatal(err)
		client, err := evals.NewPrimaryCLIJudgeClient(cfg)
		fatal(err)
		fmt.Printf("judge client: %s (runtime %s)\n", client.ModelID(), client.RuntimeVersion())
		runner := &evals.JudgeRunner{Client: client, Samples: judgeSamples}

		var mu sync.Mutex
		var wg sync.WaitGroup
		ch := make(chan resolved)
		for w := 0; w < *parallel; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for rc := range ch {
					task, ok := taskByID[rc.ep.TaskID]
					if !ok {
						mu.Lock()
						failures[rc.EpisodeID] = "no task definition for " + rc.ep.TaskID
						mu.Unlock()
						continue
					}
					start := time.Now()
					ctx, cancel := context.WithTimeout(context.Background(), *callTimeout*time.Duration(judgeSamples))
					res, err := runner.Score(ctx, task, rc.ep)
					cancel()
					mu.Lock()
					if err != nil {
						failures[rc.EpisodeID] = err.Error()
						fmt.Printf("  FAIL  %s: %v\n", rc.EpisodeID, err)
						mu.Unlock()
						continue
					}
					if _, saveErr := evals.SaveJudgeResult(*outDir, res); saveErr != nil {
						failures[rc.EpisodeID] = saveErr.Error()
						fmt.Printf("  FAIL  %s: save: %v\n", rc.EpisodeID, saveErr)
						mu.Unlock()
						continue
					}
					results[rc.EpisodeID] = res
					scoredNow++
					fmt.Printf("  ok    %s overall=%.2f (%s)\n", rc.EpisodeID, res.Overall, time.Since(start).Round(time.Second))
					mu.Unlock()
				}
			}()
		}
		for _, rc := range todo {
			ch <- rc
		}
		close(ch)
		wg.Wait()
	}

	summary := buildSummary(runs, resolvedCands, results, failures)
	fatal(os.WriteFile(filepath.Join(*outDir, "SUMMARY.md"), []byte(summary), 0o644))

	remaining := 0
	for _, rc := range resolvedCands {
		if results[rc.EpisodeID] == nil {
			remaining++
		}
	}
	fmt.Printf("cross-family done: scored %d this batch; %d resumed; %d failed; %d remaining; SUMMARY.md rewritten\n",
		scoredNow, resumed, len(failures), remaining)
}

func majorityVerdict(res *evals.JudgeResult) (good bool, goodCount, total int) {
	total = len(res.Verdicts)
	for _, v := range res.Verdicts {
		if v.Overall >= judgePoorThreshold {
			goodCount++
		}
	}
	return goodCount*2 > total, goodCount, total
}

const caveatBlock = `> **PRELIMINARY — NEVER CITABLE, NOT A QUALITY CLAIM.**
> This is a cross-family re-score (gpt-5.5 via ` + "`codex exec`" + `, ADR-0023.1 D4
> primary rail) of the 27-candidate disagreement set flagged by the first
> (same-family, k=1) judge sweep. It is still **uncalibrated** — the
> ADR-0023.1 D5 gold-set calibration has not run; no κ exists. Cross-family
> scoring removes the self-preference confound (judge family != subject
> family, since every subject agent here is Claude) but does NOT replace
> founder labeling: κ is the sole calibration unlock. This run sharpens the
> labeling-priority signal; it is not a quality verdict.`

func buildSummary(runs []string, cands []resolved, results map[string]*evals.JudgeResult, failures map[string]string) string {
	var sb strings.Builder
	sb.WriteString("# Judge Sweep 2026-07-06 — Cross-Family Re-Score (PRELIMINARY)\n\n")
	sb.WriteString("GENERATED by `go run ./docs/evals/judge-sweep-2026-07-06/cross-family/runner` — do not edit by hand.\n\n")
	sb.WriteString(caveatBlock)
	sb.WriteString("\n\n")

	cfg, cfgErr := evals.DefaultJudgeConfig()
	modelID := "gpt-5.5 via codex"
	if cfgErr == nil {
		modelID = cfg.Primary.Model + " via " + cfg.Primary.CLI.Command
	}
	fmt.Fprintf(&sb, "- Judge model: `%s` (committed PRIMARY rail, judgeconfig/judge-config-v1.json)\n", modelID)
	sb.WriteString("- Rail: `codex exec --json -s read-only --output-last-message <file> -m gpt-5.5 -`, prompt on stdin (same transport production would use)\n")
	sb.WriteString("- Prompt/rubric: `judge-prompt-v1` / `core-rubric-v1` (unchanged; per-artifact stamps)\n")
	fmt.Fprintf(&sb, "- Samples: k=%d per candidate (committed config default; first sweep used k=1 for economy)\n", judgeSamples)
	sb.WriteString("- Candidate set: the 27 \"gates fail, judge good\" episodes from the first sweep's committed SUMMARY.md (read back, not recomputed)\n")
	sb.WriteString("- Runs scored:\n")
	for _, r := range runs {
		fmt.Fprintf(&sb, "  - `%s`\n", r)
	}
	sb.WriteString("\n")

	if len(failures) > 0 {
		sb.WriteString("Failures this invocation (honest record; rerun resumes them):\n\n")
		var ids []string
		for id := range failures {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Fprintf(&sb, "- `%s`: %s\n", id, failures[id])
		}
		sb.WriteString("\n")
	}

	scored := 0
	confirmed, refuted := 0, 0
	sb.WriteString("## Per-candidate results\n\n")
	sb.WriteString("| episode | same-family (k=1) | cross-family median (k=3) | cross-family samples | majority | reading |\n")
	sb.WriteString("| --- | ---: | ---: | --- | --- | --- |\n")
	for _, c := range cands {
		res := results[c.EpisodeID]
		if res == nil {
			fmt.Fprintf(&sb, "| `%s` | %.2f | — | — | — | (unscored — see failures above) |\n", c.EpisodeID, c.SameFamilyScore)
			continue
		}
		scored++
		good, goodCount, total := majorityVerdict(res)
		var samples []string
		for _, v := range res.Verdicts {
			samples = append(samples, fmt.Sprintf("%.1f", v.Overall))
		}
		reading := "REFUTES (sides with gates)"
		if good {
			reading = "CONFIRMS (also good)"
			confirmed++
		} else {
			refuted++
		}
		majority := fmt.Sprintf("%d/%d good", goodCount, total)
		fmt.Fprintf(&sb, "| `%s` | %.2f | %.2f | %s | %s | %s |\n",
			c.EpisodeID, c.SameFamilyScore, res.Overall, strings.Join(samples, ", "), majority, reading)
	}
	sb.WriteString("\n")

	sb.WriteString("## Confirm / refute tally\n\n")
	sb.WriteString("| | count | rate of scored |\n| --- | ---: | ---: |\n")
	fmt.Fprintf(&sb, "| scored | %d | — |\n", scored)
	fmt.Fprintf(&sb, "| CONFIRMS (cross-family also says good) | %d | %s |\n", confirmed, ratePct(confirmed, scored))
	fmt.Fprintf(&sb, "| REFUTES (cross-family sides with the gates) | %d | %s |\n", refuted, ratePct(refuted, scored))
	sb.WriteString("\n")

	sb.WriteString("## Updated one-directionality picture\n\n")
	sb.WriteString("The first sweep's disagreement was strictly one-directional: all 27 " +
		"disagreements were \"gates fail, judge good\" — zero \"gates pass, judge poor\" " +
		"cases (0.0%). That asymmetry is unaffected by this re-score (this run only " +
		"rescored the already-one-directional set; it does not touch the 113 " +
		"agreeing episodes).\n\n")
	if scored > 0 {
		fmt.Fprintf(&sb, "Within the 27-candidate set, cross-family re-scoring at k=3 %s of the "+
			"original same-family \"judge good\" calls (%s), and %s (%s) — i.e. the cross-family "+
			"judge, with no self-preference confound on these Claude-authored episodes, %s.\n\n",
			pluralCount(confirmed, "CONFIRMS"), ratePct(confirmed, scored),
			pluralCount(refuted, "REFUTES"), ratePct(refuted, scored),
			directionalitySentence(confirmed, refuted, scored))
	}
	sb.WriteString("This narrows (does not resolve) the labeling priority: REFUTES rows are " +
		"the strongest candidates for the founder's next labeling pass, since two " +
		"independent model families now disagree with the gates in different ways. " +
		"CONFIRMS rows keep the original one-directional signal — still uncalibrated, " +
		"still not citable, but now cross-family-corroborated rather than same-family-only.\n\n")

	sb.WriteString(caveatBlock)
	sb.WriteString("\n")
	return sb.String()
}

func pluralCount(n int, verb string) string {
	return fmt.Sprintf("%s %d", verb, n)
}

func directionalitySentence(confirmed, refuted, scored int) string {
	switch {
	case scored == 0:
		return "produced no scored results yet"
	case confirmed == scored:
		return "agrees with the original same-family calls across the whole set — the same-family confound does not appear to explain this disagreement bucket"
	case refuted == scored:
		return "disagrees with the original same-family calls across the whole set — the same-family confound is a plausible explanation for this disagreement bucket"
	default:
		return "agrees on part of the set and disagrees on the rest — the same-family confound plausibly explains some, not all, of this disagreement bucket"
	}
}

func ratePct(n, d int) string {
	if d == 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", float64(n)/float64(d)*100)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
