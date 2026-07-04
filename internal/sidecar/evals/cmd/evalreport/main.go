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
