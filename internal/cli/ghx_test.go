package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootVersionIsRegistered(t *testing.T) {
	if RootCmd.Version != VERSION {
		t.Fatalf("RootCmd.Version = %q, want %q", RootCmd.Version, VERSION)
	}
}

func TestSuggestForCuration(t *testing.T) {
	if !contains(searchCmd.SuggestFor, "find") {
		t.Fatalf("search SuggestFor = %#v, want find", searchCmd.SuggestFor)
	}
	if !contains(grepCmd.SuggestFor, "rg") {
		t.Fatalf("grep SuggestFor = %#v, want rg", grepCmd.SuggestFor)
	}

	cmd := &cobra.Command{Use: "search", SuggestFor: []string{"find"}}
	if !contains(cmd.SuggestFor, "find") {
		t.Fatalf("dummy SuggestFor = %#v, want find", cmd.SuggestFor)
	}
}

func TestNormalizedReadLineRangeAliases(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "start end", args: []string{"--start", "3", "--end", "8"}, want: "3-8"},
		{name: "offset limit", args: []string{"--offset", "3", "--limit", "4"}, want: "3-6"},
		{name: "canonical", args: []string{"--lines", "10-20"}, want: "10-20"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newReadAliasCommand()
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("ParseFlags: %v", err)
			}
			lines, _ := cmd.Flags().GetString("lines")
			got, err := normalizedReadLineRange(cmd, lines)
			if err != nil {
				t.Fatalf("normalizedReadLineRange: %v", err)
			}
			if got != tt.want {
				t.Fatalf("range = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTeachingFlagErrorGolden(t *testing.T) {
	err := teachingFlagError(readCmd, errors.New("unknown flag: --line-range"))
	want := "unknown flag --line-range; use --lines START-END, e.g. ghx read owner/repo main.go --lines 40-80"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	if got := CodeForError(err); got != ExitBadInvocation {
		t.Fatalf("exit code = %d, want %d", got, ExitBadInvocation)
	}
}

func TestReadBudgetHint(t *testing.T) {
	got := readBudgetHint("main.go", 20000)
	for _, want := range []string{"--lines START-END", "--grep PATTERN", "--map", "--full"} {
		if !strings.Contains(got, want) {
			t.Fatalf("hint = %q, missing %q", got, want)
		}
	}
}

func TestSearchQueryRepoFirstAutoQuotes(t *testing.T) {
	got := buildSearchQuery([]string{"gkoreli/ghx", "func main"}, "go", "cmd/**/*.go")
	want := `"func main" repo:gkoreli/ghx language:go path:cmd/**/*.go`
	if got != want {
		t.Fatalf("query = %q, want %q", got, want)
	}
}

func TestGrepFlags(t *testing.T) {
	for _, name := range []string{"glob", "path", "limit"} {
		if grepCmd.Flag(name) == nil {
			t.Fatalf("grep flag %q not registered", name)
		}
	}
}

func TestSemanticExitCodes(t *testing.T) {
	if got := CodeForError(nil); got != ExitOK {
		t.Fatalf("nil code = %d, want %d", got, ExitOK)
	}
	if got := CodeForError(WithExitCode(ExitNoResults, errors.New("none"))); got != ExitNoResults {
		t.Fatalf("no-results code = %d, want %d", got, ExitNoResults)
	}
	if got := CodeForError(WithExitCode(ExitUpstreamFailure, errors.New("api"))); got != ExitUpstreamFailure {
		t.Fatalf("upstream code = %d, want %d", got, ExitUpstreamFailure)
	}
	if got := CodeForError(errors.New("plain")); got != ExitBadInvocation {
		t.Fatalf("plain error code = %d, want %d", got, ExitBadInvocation)
	}
}

func newReadAliasCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "read"}
	cmd.Flags().String("lines", "", "")
	cmd.Flags().Int("start", 0, "")
	cmd.Flags().Int("end", 0, "")
	cmd.Flags().Int("offset", 0, "")
	cmd.Flags().Int("limit", 0, "")
	return cmd
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
