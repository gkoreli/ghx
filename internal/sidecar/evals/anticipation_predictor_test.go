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

// TestWriteAnticipationPredictorArtifacts is hermetic: it mines only the
// COMMITTED eval corpus (in-repo artifacts) and writes to a temp dir. The
// committed analysis under docs/evals/anticipation-predictor-2026-07/ is the
// worker's frozen 2026-07-06 snapshot and must never be rewritten by a test
// run — regenerating it (e.g. after a report-contract revision per the
// ADR-0031.1 implementation note) is an explicit act: set
// GHX_ANTICIPATION_OUT_DIR to the target directory and, for the dogfood
// corpus, GHX_ANTICIPATION_LOCAL_SESSIONS to a sessions dir; local ~/.ghx
// mining is machine-specific and never part of the default suite.
func TestWriteAnticipationPredictorArtifacts(t *testing.T) {
	repoRoot := testRepoRoot(t)
	evalRunDir := filepath.Join(repoRoot, "docs", "evals", "gate-run-2026-07-06-fixbatch")

	evalPairs, evalSummary, err := MineAnticipationEvalCorpus(evalRunDir)
	if err != nil {
		t.Fatalf("mine eval corpus: %v", err)
	}
	if evalSummary.Pairs != 30 {
		t.Fatalf("eval pairs = %d, want 30", evalSummary.Pairs)
	}

	var localPairs []AnticipationPairResult
	var localSummary AnticipationCorpusSummary
	if sessionsDir := os.Getenv("GHX_ANTICIPATION_LOCAL_SESSIONS"); sessionsDir != "" {
		localPairs, localSummary, err = MineAnticipationLocalSessions(sessionsDir)
		if err != nil {
			t.Fatalf("mine local sessions: %v", err)
		}
	}

	outDir := os.Getenv("GHX_ANTICIPATION_OUT_DIR")
	if outDir == "" {
		outDir = t.TempDir()
	}
	if err := WriteAnticipationPredictorArtifacts(outDir, evalPairs, localPairs, evalSummary, localSummary); err != nil {
		t.Fatalf("write artifacts: %v", err)
	}
	t.Logf("committed-evals recall=%.3f precision=%.3f pairs=%d seeded=%d", evalSummary.MicroRecall, evalSummary.MicroPrecision, evalSummary.Pairs, evalSummary.SeededPairs)
	if localSummary.Pairs > 0 {
		t.Logf("local-dogfood recall=%.3f precision=%.3f pairs=%d seeded=%d", localSummary.MicroRecall, localSummary.MicroPrecision, localSummary.Pairs, localSummary.SeededPairs)
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
