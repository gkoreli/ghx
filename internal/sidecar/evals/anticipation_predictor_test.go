package evals

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnticipationPredictionMatching(t *testing.T) {
	pair := measureAnticipationPair(pairInput{
		Corpus:    "test",
		EpisodeID: "ep",
		Repo:      "o/r",
		FromTurn:  0,
		ToTurn:    1,
		NextReads: []string{
			"tree.go lines 135-400 for full addRoute/insertChild insertion logic",
			"src/router/reg-exp-router or trie-router (to see how matchResult arrays are built)",
			"docs",
		},
		ActualReads: []string{
			"o/r:tree.go",
			"src/router/reg-exp-router/router.ts",
			"docs/guide.md",
			"unpredicted.go",
		},
	})

	if pair.Recall != 0.75 {
		t.Fatalf("recall = %.3f, want 0.750; hits=%v predictions=%v actual=%v", pair.Recall, pair.RecallHits, pair.Predictions, pair.ActualReads)
	}
	if pair.Precision != 1.0 {
		t.Fatalf("precision = %.3f, want 1.000; hits=%v predictions=%v", pair.Precision, pair.PrecisionHits, pair.Predictions)
	}
}

func TestWriteAnticipationPredictorArtifacts(t *testing.T) {
	repoRoot := testRepoRoot(t)
	evalRunDir := filepath.Join(repoRoot, "docs", "evals", "gate-run-2026-07-06-fixbatch")
	outDir := filepath.Join(repoRoot, "docs", "evals", "anticipation-predictor-2026-07")

	evalPairs, evalSummary, err := MineAnticipationEvalCorpus(evalRunDir)
	if err != nil {
		t.Fatalf("mine eval corpus: %v", err)
	}
	localPairs, localSummary, err := MineAnticipationLocalSessions(filepath.Join(homeDir(t), ".ghx", "sessions"))
	if err != nil {
		t.Fatalf("mine local sessions: %v", err)
	}
	if err := WriteAnticipationPredictorArtifacts(outDir, evalPairs, localPairs, evalSummary, localSummary); err != nil {
		t.Fatalf("write artifacts: %v", err)
	}

	if evalSummary.Pairs != 30 {
		t.Fatalf("eval pairs = %d, want 30", evalSummary.Pairs)
	}
	if localSummary.Pairs == 0 {
		t.Fatal("local dogfood corpus produced no pairs")
	}
	t.Logf("committed-evals recall=%.3f precision=%.3f pairs=%d seeded=%d", evalSummary.MicroRecall, evalSummary.MicroPrecision, evalSummary.Pairs, evalSummary.SeededPairs)
	t.Logf("local-dogfood recall=%.3f precision=%.3f pairs=%d seeded=%d", localSummary.MicroRecall, localSummary.MicroPrecision, localSummary.Pairs, localSummary.SeededPairs)
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func homeDir(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return home
}
