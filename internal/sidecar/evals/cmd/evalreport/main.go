package main

import (
	"fmt"
	"os"
	"sort"

	"github.com/gkoreli/ghx/v2/internal/sidecar/evals"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./internal/sidecar/evals/cmd/evalreport <run-dir>")
		os.Exit(2)
	}
	runDir := os.Args[1]
	manifest, err := evals.LoadRunManifest(runDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load manifest: %v\n", err)
		os.Exit(1)
	}
	episodes, err := evals.LoadRunEpisodes(runDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load episodes: %v\n", err)
		os.Exit(1)
	}
	verdict := evals.EvaluateGates(episodes)

	expected := "unknown"
	if manifest != nil && manifest.ExpectedEpisodes > 0 {
		expected = fmt.Sprint(manifest.ExpectedEpisodes)
	}
	label := "valid"
	if !verdict.Valid {
		label = "INVALID"
	} else if !verdict.DataSufficient {
		label = "PRELIMINARY"
	}

	fmt.Printf("Run: %s\n", runDir)
	fmt.Printf("Status: %s\n", label)
	fmt.Printf("Episodes: %d / %s expected\n\n", len(episodes), expected)
	if traces, spans, ok, err := evals.TraceFileStats(runDir); err != nil {
		fmt.Printf("Traces: unreadable (%v)\n\n", err)
	} else if ok {
		fmt.Printf("Traces: %d spans across %d trace(s) in traces.jsonl\n\n", spans, traces)
	}

	if manifest != nil {
		fmt.Println("Identity:")
		printIdentity(manifest.Identity)
		fmt.Println()
	}

	fmt.Println("Per-profile aggregates:")
	fmt.Println("profile episodes correctness evidence mainAgentChars")
	for _, p := range evals.AllProfiles() {
		a := verdict.Aggregates[p]
		if a == nil {
			continue
		}
		fmt.Printf("%s %d %.3f %.3f %.0f\n", p, a.Episodes, a.MeanCorrectness, a.MeanEvidence, a.MeanMainAgentChars)
	}
	fmt.Println()

	fmt.Println("Signal per token (informational, not a gate):")
	fmt.Println("chars/4 estimate (ADR-0016.6, primary):")
	fmt.Println("profile n meanSignal mainAgentSPT sidecarInternalSPT workflowSPT")
	spt := evals.AggregateSignalPerToken(episodes)
	for _, p := range evals.AllProfiles() {
		row := spt[p]
		fmt.Printf("%s %d %.3f %s %s %s\n",
			p, row.Episodes, row.MeanSignal,
			formatSPT(row.MainAgentSPT),
			formatSPT(row.SidecarInternalSPT),
			formatSPT(row.WorkflowSPT),
		)
	}
	fmt.Println()

	fmt.Println("provider-reported tokens (ADR-0016.11, session-level):")
	fmt.Println("profile n withUsage totalTokens sessionSPT")
	real := evals.AggregateRealTokenSPT(episodes)
	for _, p := range evals.AllProfiles() {
		row := real[p]
		if !row.Available {
			fmt.Printf("%s %d %d — UNAVAILABLE (%d/%d episodes carry usage)\n",
				p, row.Episodes, row.EpisodesWithUsage, row.EpisodesWithUsage, row.Episodes)
			continue
		}
		fmt.Printf("%s %d %d %d %s\n",
			p, row.Episodes, row.EpisodesWithUsage, row.TotalTokens, formatSessionSPT(row))
	}
	fmt.Println()

	fmt.Println("Run economics (ADR-0016.11, provider-reported costUsd):")
	printEconomics(evals.ComputeRunEconomics(episodes))
	fmt.Println()

	fmt.Println("Task x profile matrix:")
	printMatrix(episodes)
	fmt.Println()

	fmt.Println("Gate status:")
	for _, g := range verdict.Gates {
		status := "FAIL"
		if g.Pass {
			status = "PASS"
		}
		fmt.Printf("%s %s: %s\n", g.ID, status, g.Detail)
	}
	fmt.Println()

	fmt.Println("Compliance and identity notes:")
	if len(verdict.Notes) == 0 {
		fmt.Println("- none")
	} else {
		for _, n := range verdict.Notes {
			fmt.Printf("- %s\n", n)
		}
	}
	fmt.Println()
	fmt.Print(evals.FormatVerdict(verdict))
}

func printIdentity(id evals.AgentIdentity) {
	fmt.Printf("agentCommand: %s\n", id.AgentCommand)
	fmt.Printf("adapter: %s %s\n", id.AdapterName, id.AdapterVersion)
	fmt.Printf("subjectModel: %s\n", id.SubjectModel)
	if id.AdapterSubjectModel != "" {
		fmt.Printf("adapterSubjectModel: %s\n", id.AdapterSubjectModel)
	}
	if id.WrapperSHA256 != "" {
		fmt.Printf("wrapperSha256: %s\n", id.WrapperSHA256)
	}
}

func formatSPT(v evals.SPTValue) string {
	if !v.Defined {
		return "undefined"
	}
	return fmt.Sprintf("%.3f", v.Value)
}

// formatSessionSPT prints the real-token SPT with six decimals: provider
// tokens re-count cached context per API call, so values sit orders of
// magnitude below the chars/4 variant.
func formatSessionSPT(row evals.ProfileRealTokenSPT) string {
	if !row.SessionSPT.Defined {
		return "undefined"
	}
	return fmt.Sprintf("%.6f", row.SessionSPT.Value)
}

func printEconomics(econ evals.RunEconomics) {
	if econ.EpisodesWithCost == 0 {
		fmt.Printf("total: UNAVAILABLE (0/%d episodes carry cost data)\n", econ.Episodes)
		return
	}
	bound := ""
	if econ.EpisodesWithCost < econ.Episodes {
		bound = " (lower bound — coverage is partial)"
	}
	fmt.Printf("total: $%.4f over %d/%d episodes with cost data%s\n",
		econ.TotalCostUSD, econ.EpisodesWithCost, econ.Episodes, bound)
	for _, pe := range econ.PerProfile {
		if pe.Episodes == 0 {
			continue
		}
		if pe.EpisodesWithCost == 0 {
			fmt.Printf("%s: UNAVAILABLE (0/%d episodes carry cost data)\n", pe.Profile, pe.Episodes)
			continue
		}
		fmt.Printf("%s: $%.4f (%d/%d episodes)\n", pe.Profile, pe.CostUSD, pe.EpisodesWithCost, pe.Episodes)
	}
}

func printMatrix(episodes []*evals.Episode) {
	tasks := map[string]map[evals.Profile]int{}
	var names []string
	for _, ep := range episodes {
		if _, ok := tasks[ep.TaskID]; !ok {
			tasks[ep.TaskID] = map[evals.Profile]int{}
			names = append(names, ep.TaskID)
		}
		tasks[ep.TaskID][ep.Profile]++
	}
	sort.Strings(names)
	fmt.Println("task plain ghx ghx-sidecar")
	for _, task := range names {
		fmt.Printf("%s %d %d %d\n",
			task,
			tasks[task][evals.ProfilePlain],
			tasks[task][evals.ProfileGhx],
			tasks[task][evals.ProfileSidecar],
		)
	}
}
