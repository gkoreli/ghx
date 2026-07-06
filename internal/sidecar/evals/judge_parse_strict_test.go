package evals

import (
	"strings"
	"testing"
)

// Regression for the 2026-07-06 codex review finding: dec.More() does not
// detect a second top-level JSON value or trailing prose, so "strict" parsing
// accepted `{verdict}\nextra`. Only a clean EOF on the next token is strict.

func TestParseJudgeVerdictRejectsTrailingTopLevelValue(t *testing.T) {
	for _, trailing := range []string{
		"\n{\"another\":1}",
		"\nSome prose after the object.",
		" true",
	} {
		if _, err := parseJudgeVerdict(validVerdictJSON() + trailing); err == nil {
			t.Fatalf("accepted verdict with trailing content %q", trailing)
		} else if !strings.Contains(err.Error(), "trailing content") {
			t.Fatalf("trailing %q: error = %v, want trailing-content error", trailing, err)
		}
	}
}

func TestParseJudgeVerdictAcceptsTrailingWhitespaceOnly(t *testing.T) {
	if _, err := parseJudgeVerdict(validVerdictJSON() + "\n  \n"); err != nil {
		t.Fatalf("rejected verdict with only trailing whitespace: %v", err)
	}
}

func TestParseJudgeConfigRejectsTrailingTopLevelValue(t *testing.T) {
	data := string(defaultJudgeConfigJSON) + "\n{\"schemaVersion\":\"judge-config-v1\"}"
	if _, err := parseJudgeConfig([]byte(data), "test"); err == nil {
		t.Fatal("accepted judge config with a trailing second object")
	} else if !strings.Contains(err.Error(), "trailing content") {
		t.Fatalf("error = %v, want trailing-content error", err)
	}
}
