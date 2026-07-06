// Command sweep runs the FIRST JUDGE SWEEP (2026-07-06): it scores every
// gold-set candidate episode from the two committed gate runs with the merged
// ADR-0023.1 judge machinery, writing PRELIMINARY JudgeResult artifacts plus a
// SUMMARY.md whose only purpose is to sharpen the founder's gold-set labeling
// session (ranked labeling order, disagreements + extremes first).
//
// ── STRUCTURAL CAVEAT (state everywhere, always) ────────────────────────────
// The primary cross-family judge rail (GPT-5.5 via codex/OpenAI, ADR-0023.1
// D4) was OUT OF TOKENS at sweep time. This sweep uses the SECONDARY rail —
// the config's secondary model `claude-opus-4-8` — reached through the local
// `claude -p` CLI. Judge family == subject family, so every score carries a
// structural SAME-FAMILY self-preference caveat ON TOP of the uncalibrated
// PRELIMINARY status (D5). These scores exist ONLY to rank/triage episodes for
// human labeling and to exercise the pipeline. They are NEVER citable and are
// NEVER quality claims.
//
// Sweep-mode deviations from the committed judge config (economy, recorded):
//   - k=1 instead of k=3 self-consistency (one sample per episode).
//   - CLI transport instead of the HTTP JudgeClient; the malformed-output
//     re-ask is a single-shot re-prompt (original prompt + previous raw reply
//     + parse error) rather than a multi-message conversation replay.
//
// The driver is read-only over episodes, resumable (existing .judge.json
// artifacts are loaded, not re-scored), and deterministic apart from the judge
// model calls themselves.
//
// Usage (from the repo root):
//
//	go run ./docs/evals/judge-sweep-2026-07-06/sweep                # score everything pending
//	go run ./docs/evals/judge-sweep-2026-07-06/sweep -limit 30      # one progress batch
//	go run ./docs/evals/judge-sweep-2026-07-06/sweep -summary-only  # rebuild SUMMARY.md
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals"
)

// The two committed gate runs the gold-set candidates come from (must match
// docs/evals/judge-goldset/CANDIDATES.md).
var defaultRuns = []string{
	"docs/evals/gate-run-2026-07-05-confirmatory",
	"docs/evals/gate-run-2026-07-05-d2-0021-0022-partial",
}

const (
	defaultTasksDir   = "internal/sidecar/evals/testdata/tasks"
	defaultOutDir     = "docs/evals/judge-sweep-2026-07-06"
	defaultCandidates = "docs/evals/judge-goldset/CANDIDATES.md"

	// cliModel is the config's SECONDARY judge model (judgeconfig/
	// judge-config-v1.json .secondary.model), driven through `claude -p`.
	cliModel = "claude-opus-4-8"
	// recordedModelID is stamped on every JudgeResult artifact. It carries the
	// rail + caveat so no artifact can be read without the same-family /
	// PRELIMINARY context (the JudgeResult schema has no caveat field and the
	// judge code is frozen, so the caveat rides on the model identifier).
	recordedModelID = "claude-opus-4-8/claude-cli-sweep (SAME-FAMILY, PRELIMINARY, k=1)"

	// maxPromptChars mirrors judgeconfig judge-config-v1.json .maxPromptChars.
	maxPromptChars = 200000

	// Per-episode gate proxy floors, mirroring the unexported constants used
	// by ComputeDisagreement (internal/sidecar/evals/gates.go g1AbsFloor /
	// g2Floor; ADR-0016.1 pre-registered): correctness >= 0.60, evidence >=
	// 0.70, safety == 1.0. Keep in sync — the ranked labeling order must use
	// the same disagreement definition as the committed report.
	gateCorrectnessFloor = 0.60
	gateEvidenceFloor    = 0.70

	// judgePoorThreshold mirrors the provisional (uncalibrated) constant in
	// judge_disagreement.go.
	judgePoorThreshold = 2.5
)

func main() {
	tasksDir := flag.String("tasks", defaultTasksDir, "task definitions directory")
	outDir := flag.String("out", defaultOutDir, "sweep artifact directory")
	candidatesMD := flag.String("candidates", defaultCandidates, "committed CANDIDATES.md (label ids + batch-1)")
	parallel := flag.Int("parallel", 4, "concurrent judge calls")
	limit := flag.Int("limit", 0, "max episodes to score this invocation (0 = all pending)")
	callTimeout := flag.Duration("call-timeout", 5*time.Minute, "per judge-call timeout")
	summaryOnly := flag.Bool("summary-only", false, "skip scoring; rebuild SUMMARY.md from existing artifacts")
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

	// Load every episode from the committed runs and re-derive gate-time
	// validity, exactly as the gold-set generator does, so the D7 pre-filter
	// inside JudgeRunner.Score sees the same state a verdict run would.
	var episodes []*evals.Episode
	for _, runDir := range runs {
		eps, err := evals.LoadRunEpisodes(runDir)
		fatal(err)
		evals.EvaluateGates(eps)
		episodes = append(episodes, eps...)
	}
	sort.Slice(episodes, func(i, j int) bool { return episodes[i].ID < episodes[j].ID })

	fatal(os.MkdirAll(*outDir, 0o755))

	// Resume: existing artifacts are evidence, never re-scored.
	results := map[string]*evals.JudgeResult{}
	var pending []*evals.Episode
	for _, ep := range episodes {
		path := filepath.Join(*outDir, ep.ID+".judge.json")
		if _, statErr := os.Stat(path); statErr == nil {
			res, loadErr := evals.LoadJudgeResult(path)
			fatal(loadErr)
			results[ep.ID] = res
			continue
		}
		pending = append(pending, ep)
	}
	resumed := len(results)

	failures := map[string]string{}
	scoredNow, skippedNow := 0, 0

	if !*summaryOnly {
		todo := pending
		if *limit > 0 && len(todo) > *limit {
			todo = todo[:*limit]
		}
		fmt.Printf("sweep: %d episodes total, %d already scored (resumed), %d pending, %d in this batch\n",
			len(episodes), resumed, len(pending), len(todo))

		client := &claudeCLIClient{model: cliModel, timeout: *callTimeout}
		runner := &evals.JudgeRunner{Client: client, Samples: 1} // sweep-mode k=1 (economy; deviation from config k=3)

		var mu sync.Mutex
		var wg sync.WaitGroup
		ch := make(chan *evals.Episode)
		for w := 0; w < *parallel; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for ep := range ch {
					task, ok := taskByID[ep.TaskID]
					if !ok {
						mu.Lock()
						failures[ep.ID] = "no task definition for " + ep.TaskID
						mu.Unlock()
						continue
					}
					start := time.Now()
					res, err := runner.Score(context.Background(), task, ep)
					mu.Lock()
					if err != nil {
						failures[ep.ID] = err.Error()
						fmt.Printf("  FAIL  %s: %v\n", ep.ID, err)
						mu.Unlock()
						continue
					}
					if _, saveErr := evals.SaveJudgeResult(*outDir, res); saveErr != nil {
						failures[ep.ID] = saveErr.Error()
						fmt.Printf("  FAIL  %s: save: %v\n", ep.ID, saveErr)
						mu.Unlock()
						continue
					}
					results[ep.ID] = res
					if res.Skipped {
						skippedNow++
						fmt.Printf("  skip  %s (%s)\n", ep.ID, res.SkipReason)
					} else {
						scoredNow++
						fmt.Printf("  ok    %s overall=%.1f (%s)\n", ep.ID, res.Overall, time.Since(start).Round(time.Second))
					}
					mu.Unlock()
				}
			}()
		}
		for _, ep := range todo {
			ch <- ep
		}
		close(ch)
		wg.Wait()
	}

	summary := buildSummary(runs, episodes, results, failures, *candidatesMD)
	fatal(os.WriteFile(filepath.Join(*outDir, "SUMMARY.md"), []byte(summary), 0o644))

	remaining := 0
	for _, ep := range episodes {
		if results[ep.ID] == nil {
			remaining++
		}
	}
	fmt.Printf("sweep done: scored %d, skipped %d this batch; %d resumed; %d failed; %d remaining; SUMMARY.md rewritten\n",
		scoredNow, skippedNow, resumed, len(failures), remaining)
}

// ── claude -p CLI JudgeClient (sweep-only secondary rail) ────────────────────

// claudeCLIClient implements evals.JudgeClient by shelling out to the local
// `claude -p` CLI with the committed secondary judge model. Tools are
// disallowed; the prompt arrives on stdin; the reply must be the strict
// verdict JSON the prompt template specifies.
type claudeCLIClient struct {
	model   string
	timeout time.Duration
}

func (c *claudeCLIClient) ModelID() string { return recordedModelID }

func (c *claudeCLIClient) Evaluate(ctx context.Context, prompt string) (evals.JudgeVerdict, error) {
	if len(prompt) > maxPromptChars {
		return evals.JudgeVerdict{}, fmt.Errorf("prompt is %d chars, exceeds committed cap %d", len(prompt), maxPromptChars)
	}
	raw, err := c.call(ctx, prompt)
	if err != nil {
		return evals.JudgeVerdict{}, err
	}
	v, parseErr := parseVerdict(raw)
	if parseErr == nil {
		return v, nil
	}
	// One re-ask (single-shot variant of the HTTP clients' conversation
	// replay): original prompt + the model's bad reply + the parse error.
	reask := prompt + "\n\n## Correction required\n\nYour previous reply was:\n\n" +
		raw + "\n\nIt failed validation: " + parseErr.Error() +
		"\nReturn ONLY the JSON object in exactly the shape specified above — no prose, no markdown fences, all three core dimensions present exactly once."
	raw2, err := c.call(ctx, reask)
	if err != nil {
		return evals.JudgeVerdict{}, err
	}
	v2, parseErr2 := parseVerdict(raw2)
	if parseErr2 != nil {
		return evals.JudgeVerdict{}, fmt.Errorf("malformed judge output after re-ask: %w (raw: %.500s)", parseErr2, strings.TrimSpace(raw2))
	}
	return v2, nil
}

// call runs one `claude -p` invocation with bounded retries on process
// failure or empty output.
func (c *claudeCLIClient) call(ctx context.Context, prompt string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(attempt) * 10 * time.Second):
			}
		}
		cctx, cancel := context.WithTimeout(ctx, c.timeout)
		cmd := exec.CommandContext(cctx, "claude", "-p",
			"--model", c.model,
			"--output-format", "text",
			"--disallowedTools", "*")
		cmd.Stdin = strings.NewReader(prompt)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		cancel()
		if runErr != nil {
			lastErr = fmt.Errorf("claude -p (attempt %d): %w (stderr: %.300s)", attempt+1, runErr, strings.TrimSpace(stderr.String()))
			continue
		}
		out := strings.TrimSpace(stdout.String())
		if out == "" {
			lastErr = fmt.Errorf("claude -p (attempt %d): empty output", attempt+1)
			continue
		}
		return out, nil
	}
	return "", lastErr
}

// parseVerdict mirrors the strict verdict parsing of the HTTP JudgeClients
// (judge_client_http.go, unexported there): exactly one JSON object, unknown
// fields rejected, exactly the three core-rubric dimensions each once, integer
// scores and overall within the 1–5 scale; one enclosing markdown fence
// tolerated.
func parseVerdict(raw string) (evals.JudgeVerdict, error) {
	s := stripJSONFences(raw)
	if s == "" {
		return evals.JudgeVerdict{}, errors.New("empty reply")
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.DisallowUnknownFields()
	var v evals.JudgeVerdict
	if err := dec.Decode(&v); err != nil {
		return evals.JudgeVerdict{}, fmt.Errorf("invalid JSON: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return evals.JudgeVerdict{}, errors.New("trailing content after the JSON object")
	}
	rubric := evals.CoreRubricSpec()
	seen := map[string]bool{}
	for _, d := range v.Dimensions {
		valid := false
		for _, rd := range rubric.Dimensions {
			if d.Dimension == rd.Name {
				valid = true
				break
			}
		}
		if !valid {
			return evals.JudgeVerdict{}, fmt.Errorf("unknown dimension %q", d.Dimension)
		}
		if seen[d.Dimension] {
			return evals.JudgeVerdict{}, fmt.Errorf("dimension %q appears more than once", d.Dimension)
		}
		seen[d.Dimension] = true
		if d.Score < rubric.ScaleMin || d.Score > rubric.ScaleMax {
			return evals.JudgeVerdict{}, fmt.Errorf("dimension %q score %d out of scale", d.Dimension, d.Score)
		}
	}
	for _, rd := range rubric.Dimensions {
		if !seen[rd.Name] {
			return evals.JudgeVerdict{}, fmt.Errorf("missing dimension %q", rd.Name)
		}
	}
	if v.Overall < float64(rubric.ScaleMin) || v.Overall > float64(rubric.ScaleMax) {
		return evals.JudgeVerdict{}, fmt.Errorf("overall %v out of scale", v.Overall)
	}
	return v, nil
}

func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	rest := s[3:]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		lang := strings.TrimSpace(rest[:nl])
		if lang == "" || strings.EqualFold(lang, "json") {
			rest = rest[nl+1:]
			if end := strings.LastIndex(rest, "```"); end >= 0 {
				return strings.TrimSpace(rest[:end])
			}
		}
	}
	return s
}

// ── candidates (blinded label ids + batch-1) from committed CANDIDATES.md ────

type candidateRow struct {
	LabelID   string
	Path      string
	EpisodeID string
	InBatch   bool
}

// parseCandidates reads the committed CANDIDATES.md table rather than
// recomputing label IDs, so the sweep's labeling order uses exactly the IDs
// the founder's packets carry.
func parseCandidates(path string) ([]candidateRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var rows []candidateRow
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "| gs-") {
			continue
		}
		// A table row `| a | b | ... | f |` splits into a leading and trailing
		// empty cell: columns are cells[1..6] (label id, episode file, task,
		// turns, stratum, batch-1).
		cells := strings.Split(line, "|")
		if len(cells) < 8 {
			continue
		}
		labelID := strings.TrimSpace(cells[1])
		epPath := strings.Trim(strings.TrimSpace(cells[2]), "`")
		inBatch := strings.TrimSpace(cells[6]) == "x"
		rows = append(rows, candidateRow{
			LabelID:   labelID,
			Path:      epPath,
			EpisodeID: strings.TrimSuffix(filepath.Base(epPath), ".json"),
			InBatch:   inBatch,
		})
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no candidate rows parsed from %s", path)
	}
	return rows, nil
}

// ── summary ──────────────────────────────────────────────────────────────────

// gateProxyPass mirrors the unexported perEpisodeGatePass used by
// ComputeDisagreement (see constants above).
func gateProxyPass(ep *evals.Episode) bool {
	r := ep.Rewards
	return r.Correctness >= gateCorrectnessFloor && r.Evidence >= gateEvidenceFloor && r.Safety == 1.0
}

func isDisagreement(ep *evals.Episode, res *evals.JudgeResult) (bool, string) {
	if res == nil || res.Skipped {
		return false, ""
	}
	gate := gateProxyPass(ep)
	poor := res.Overall < judgePoorThreshold
	switch {
	case gate && poor:
		return true, "gates pass / judge poor"
	case !gate && !poor:
		return true, "gates fail / judge good"
	}
	return false, ""
}

func profileOf(episodeID string) string {
	// Episode IDs are <task>_<profile>_<timestamp>.
	parts := strings.Split(episodeID, "_")
	if len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	return "unknown"
}

const caveatBlock = `> **PRELIMINARY — NEVER CITABLE, NOT A QUALITY CLAIM.**
> This sweep's scores are (1) **uncalibrated** — the ADR-0023.1 D5 gold-set
> calibration has not run; no κ exists — and (2) **SAME-FAMILY**: the primary
> cross-family judge rail (GPT-5.5 via the codex/OpenAI rail) was out of
> tokens, so the sweep used the config's SECONDARY model ` + "`claude-opus-4-8`" + `
> through the local ` + "`claude -p`" + ` CLI. Judge family == subject family, a
> structural self-preference confound (ADR-0023.1 D4 exists precisely to
> avoid this). Sweep economy: **k=1** per episode (config is k=3), so no
> self-consistency spread exists either. These numbers exist ONLY to
> (a) rank/triage episodes for the founder's human labeling session and
> (b) exercise the judge pipeline end to end.`

func buildSummary(runs []string, episodes []*evals.Episode, results map[string]*evals.JudgeResult, failures map[string]string, candidatesPath string) string {
	var sb strings.Builder
	sb.WriteString("# Judge Sweep 2026-07-06 — SUMMARY (PRELIMINARY, SAME-FAMILY)\n\n")
	sb.WriteString("GENERATED by `go run ./docs/evals/judge-sweep-2026-07-06/sweep` — do not edit by hand.\n\n")
	sb.WriteString(caveatBlock)
	sb.WriteString("\n\n")

	fmt.Fprintf(&sb, "- Judge model (recorded on every artifact): `%s`\n", recordedModelID)
	sb.WriteString("- Rail: `claude -p --model claude-opus-4-8 --output-format text --disallowedTools \"*\"`, prompt on stdin\n")
	sb.WriteString("- Prompt/rubric: `judge-prompt-v1` / `core-rubric-v1` (per-artifact stamps)\n")
	sb.WriteString("- Samples: k=1 (sweep economy; deviation from config k=3, recorded above)\n")
	sb.WriteString("- Runs scored:\n")
	for _, r := range runs {
		fmt.Fprintf(&sb, "  - `%s`\n", r)
	}
	sb.WriteString("\n")

	// Counts.
	scored, skipped, unscored := 0, 0, 0
	var scoredResults []*evals.JudgeResult
	epByID := map[string]*evals.Episode{}
	for _, ep := range episodes {
		epByID[ep.ID] = ep
		res := results[ep.ID]
		switch {
		case res == nil:
			unscored++
		case res.Skipped:
			skipped++
		default:
			scored++
			scoredResults = append(scoredResults, res)
		}
	}
	sb.WriteString("## Coverage\n\n")
	sb.WriteString("| | count |\n| --- | ---: |\n")
	fmt.Fprintf(&sb, "| episodes in the two runs | %d |\n", len(episodes))
	fmt.Fprintf(&sb, "| judge-scored | %d |\n", scored)
	fmt.Fprintf(&sb, "| skipped (D7 pre-filter) | %d |\n", skipped)
	fmt.Fprintf(&sb, "| unscored (pending/failed) | %d |\n", unscored)
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

	// Ranked labeling order FIRST, blinded, so the founder can use the file
	// without reading score details below.
	sb.WriteString("## Ranked labeling order — batch-1 (30 packets)\n\n")
	sb.WriteString("**Founder: label packets in this order, then STOP READING this file until\n")
	sb.WriteString("the session is done.** The order itself encodes judge opinion (disagreements\n")
	sb.WriteString("and extreme scores first — max κ information per label), but the label IDs\n")
	sb.WriteString("below are blinded and carry no scores, paths, or profiles. Score details are\n")
	sb.WriteString("in the appendix at the bottom; reading them before labeling anchors the\n")
	sb.WriteString("labels and corrupts κ.\n\n")

	cands, candErr := parseCandidates(candidatesPath)
	if candErr != nil {
		fmt.Fprintf(&sb, "(could not parse %s: %v)\n\n", candidatesPath, candErr)
	} else {
		type ranked struct {
			row      candidateRow
			disagree bool
			reason   string
			extreme  float64
			overall  float64
			scored   bool
		}
		var batch []ranked
		for _, c := range cands {
			if !c.InBatch {
				continue
			}
			ep := epByID[c.EpisodeID]
			res := results[c.EpisodeID]
			r := ranked{row: c}
			if ep != nil && res != nil && !res.Skipped {
				r.scored = true
				r.overall = res.Overall
				r.disagree, r.reason = isDisagreement(ep, res)
				r.extreme = res.Overall - 3.0
				if r.extreme < 0 {
					r.extreme = -r.extreme
				}
			}
			batch = append(batch, r)
		}
		sort.Slice(batch, func(i, j int) bool {
			a, b := batch[i], batch[j]
			if a.scored != b.scored {
				return a.scored // unscored last
			}
			if a.disagree != b.disagree {
				return a.disagree
			}
			if a.extreme != b.extreme {
				return a.extreme > b.extreme
			}
			return a.row.LabelID < b.row.LabelID
		})
		for i, r := range batch {
			fmt.Fprintf(&sb, "%2d. %s\n", i+1, r.row.LabelID)
		}
		sb.WriteString("\n")

		// Disagreements outside batch-1 worth swapping in.
		var extra []string
		for _, c := range cands {
			if c.InBatch {
				continue
			}
			ep := epByID[c.EpisodeID]
			res := results[c.EpisodeID]
			if ep == nil || res == nil || res.Skipped {
				continue
			}
			if dis, _ := isDisagreement(ep, res); dis {
				extra = append(extra, c.LabelID)
			}
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			sb.WriteString("Gate/judge disagreements OUTSIDE batch-1 (consider swapping in or labeling\n")
			sb.WriteString("as extras — each is a high-information label): ")
			sb.WriteString(strings.Join(extra, ", "))
			sb.WriteString("\n\n")
		}
	}

	// Distributions.
	sb.WriteString("## Score distribution per dimension (PRELIMINARY, same-family, k=1)\n\n")
	if scored == 0 {
		sb.WriteString("(no scored episodes yet)\n\n")
	} else {
		rubric := evals.CoreRubricSpec()
		sb.WriteString("| dimension | 1 | 2 | 3 | 4 | 5 | mean |\n| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
		for _, rd := range rubric.Dimensions {
			hist := map[int]int{}
			sum, n := 0.0, 0
			for _, res := range scoredResults {
				for _, d := range res.Dimensions {
					if d.Dimension == rd.Name {
						hist[int(d.MedianScore+0.5)]++
						sum += d.MedianScore
						n++
					}
				}
			}
			mean := 0.0
			if n > 0 {
				mean = sum / float64(n)
			}
			fmt.Fprintf(&sb, "| %s | %d | %d | %d | %d | %d | %.2f |\n",
				rd.Name, hist[1], hist[2], hist[3], hist[4], hist[5], mean)
		}
		// Overall distribution.
		overallHist := map[int]int{}
		overallSum := 0.0
		for _, res := range scoredResults {
			overallHist[int(res.Overall+0.5)]++
			overallSum += res.Overall
		}
		fmt.Fprintf(&sb, "| overall (holistic) | %d | %d | %d | %d | %d | %.2f |\n",
			overallHist[1], overallHist[2], overallHist[3], overallHist[4], overallHist[5], overallSum/float64(scored))
		sb.WriteString("\n")

		// Per-profile means — triage only; with a same-family judge a Claude-
		// favoring skew here is expected noise, not signal.
		sb.WriteString("Per-profile mean judge overall (triage only — same-family confound applies\n")
		sb.WriteString("with full force to any cross-profile comparison):\n\n")
		profSum := map[string]float64{}
		profN := map[string]int{}
		for _, res := range scoredResults {
			p := profileOf(res.EpisodeID)
			profSum[p] += res.Overall
			profN[p]++
		}
		var profs []string
		for p := range profN {
			profs = append(profs, p)
		}
		sort.Strings(profs)
		sb.WriteString("| profile | n | mean overall |\n| --- | ---: | ---: |\n")
		for _, p := range profs {
			fmt.Fprintf(&sb, "| %s | %d | %.2f |\n", p, profN[p], profSum[p]/float64(profN[p]))
		}
		sb.WriteString("\n")
	}

	// Committed disagreement report (ADR-0023.1 D1 machinery).
	sb.WriteString(evals.RenderDisagreementReport(episodes, results))
	sb.WriteString("\n> Reminder: the judge side of this cross-tab is SAME-FAMILY and k=1 on top\n")
	sb.WriteString("> of the PRELIMINARY label above — the disagreement rate is a triage signal\n")
	sb.WriteString("> for labeling priority, nothing more.\n\n")

	// Appendix: full detail (paths reveal profiles — not for the labeler).
	sb.WriteString("## Appendix — per-episode detail (NOT for the labeler mid-session)\n\n")
	sb.WriteString("| episode | judge overall | ev.grd | expl.eff | unc.hon | gate proxy | disagreement |\n")
	sb.WriteString("| --- | ---: | ---: | ---: | ---: | --- | --- |\n")
	for _, ep := range episodes {
		res := results[ep.ID]
		if res == nil {
			fmt.Fprintf(&sb, "| `%s` | — | — | — | — | %s | (unscored) |\n", ep.ID, passFail(gateProxyPass(ep)))
			continue
		}
		if res.Skipped {
			fmt.Fprintf(&sb, "| `%s` | — | — | — | — | %s | (skipped: %s) |\n", ep.ID, passFail(gateProxyPass(ep)), res.SkipReason)
			continue
		}
		dims := map[string]float64{}
		for _, d := range res.Dimensions {
			dims[d.Dimension] = d.MedianScore
		}
		_, reason := isDisagreement(ep, res)
		if reason == "" {
			reason = "—"
		}
		fmt.Fprintf(&sb, "| `%s` | %.1f | %.0f | %.0f | %.0f | %s | %s |\n",
			ep.ID, res.Overall,
			dims["evidence_groundedness"], dims["exploration_efficiency"], dims["uncertainty_honesty"],
			passFail(gateProxyPass(ep)), reason)
	}
	sb.WriteString("\n")
	sb.WriteString(caveatBlock)
	sb.WriteString("\n")
	return sb.String()
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
