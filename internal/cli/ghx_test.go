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
		{name: "line range alias", args: []string{"--line-range", "10-20"}, want: "10-20"},
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

func TestReadLineRangeAliasMatchesLines(t *testing.T) {
	canonical := newReadAliasCommand()
	if err := canonical.ParseFlags([]string{"--lines", "40-80"}); err != nil {
		t.Fatalf("ParseFlags --lines: %v", err)
	}
	canonicalLines, _ := canonical.Flags().GetString("lines")
	canonicalRange, err := normalizedReadLineRange(canonical, canonicalLines)
	if err != nil {
		t.Fatalf("normalizedReadLineRange --lines: %v", err)
	}

	alias := newReadAliasCommand()
	if err := alias.ParseFlags([]string{"--line-range", "40-80"}); err != nil {
		t.Fatalf("ParseFlags --line-range: %v", err)
	}
	aliasLines, _ := alias.Flags().GetString("lines")
	aliasRange, err := normalizedReadLineRange(alias, aliasLines)
	if err != nil {
		t.Fatalf("normalizedReadLineRange --line-range: %v", err)
	}

	if aliasRange != canonicalRange {
		t.Fatalf("--line-range normalized to %q, want same as --lines %q", aliasRange, canonicalRange)
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

func TestGhxCoreErrorInvalidRepoIsBadInvocation(t *testing.T) {
	err := ghxCoreError(errors.New(`invalid repo "noslash": expected owner/repo, e.g. ghx explore gkoreli/ghx`))
	if got := CodeForError(err); got != ExitBadInvocation {
		t.Fatalf("exit code = %d, want %d", got, ExitBadInvocation)
	}
}

func TestRepoScopedCommandsParseRepoAtEdge(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
	}{
		{name: "explore", cmd: exploreCmd, args: []string{"noslash"}},
		{name: "read", cmd: readCmd, args: []string{"noslash", "README.md"}},
		{name: "tree", cmd: treeCmd, args: []string{"noslash"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.RunE(tt.cmd, tt.args)
			if err == nil || !strings.Contains(err.Error(), "invalid repo") {
				t.Fatalf("RunE error = %v, want invalid repo", err)
			}
			if got := CodeForError(err); got != ExitBadInvocation {
				t.Fatalf("exit code = %d, want %d", got, ExitBadInvocation)
			}
		})
	}
}

func TestTeachingFlagErrorPathSuggestions(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		want string
	}{
		{
			name: "explore path is positional",
			cmd:  exploreCmd,
			want: "unknown flag --path; use path argument, e.g. ghx explore owner/repo src",
		},
		{
			name: "search path points to grep",
			cmd:  searchCmd,
			want: "unknown flag --path; use ghx grep owner/repo PATTERN --path PATH, e.g. ghx grep owner/repo \"query\" --path src",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := teachingFlagError(tt.cmd, errors.New("unknown flag: --path"))
			if err.Error() != tt.want {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
			if got := CodeForError(err); got != ExitBadInvocation {
				t.Fatalf("exit code = %d, want %d", got, ExitBadInvocation)
			}
		})
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

func TestInspectCommandShape(t *testing.T) {
	cmd, _, err := RootCmd.Find([]string{"inspect"})
	if err != nil {
		t.Fatalf("Find inspect: %v", err)
	}
	if cmd != inspectCmd {
		t.Fatalf("RootCmd inspect command = %v, want inspectCmd", cmd)
	}
	if inspectCmd.Use != "inspect <owner/repo> <query>" {
		t.Fatalf("Use = %q", inspectCmd.Use)
	}
	if !strings.Contains(inspectCmd.Example, "ghx inspect gin-gonic/gin") {
		t.Fatalf("inspect examples missing gin smoke: %q", inspectCmd.Example)
	}
	for _, name := range []string{"budget", "limit", "lang", "glob", "path"} {
		if inspectCmd.Flag(name) == nil {
			t.Fatalf("inspect flag %q not registered", name)
		}
	}
	if inspectCmd.Flag("json") != nil {
		t.Fatal("inspect --json registered before shared JSON contract exists")
	}
}

func TestInspectBadInvocationExitCode(t *testing.T) {
	err := inspectCmd.Args(inspectCmd, []string{"owner/repo"})
	if err == nil {
		t.Fatal("inspect Args returned nil for missing query")
	}
	if got := CodeForError(WithExitCode(ExitBadInvocation, err)); got != ExitBadInvocation {
		t.Fatalf("exit code = %d, want %d", got, ExitBadInvocation)
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
	cmd.Flags().String("line-range", "", "")
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
