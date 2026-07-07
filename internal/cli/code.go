package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gkoreli/ghx/v2/internal/codemode"
	"github.com/gkoreli/ghx/v2/internal/ghx"
	"github.com/spf13/cobra"
)

var codeCmd = &cobra.Command{
	Use:   "code [code | -]",
	Short: "Execute JavaScript with access to all ghx tools",
	Long: `Run a JavaScript snippet that calls the ghx tools (explore, read, search, repos,
tree) via the ` + "`codemode`" + ` object, so an explore → filter → read chain runs in one
round-trip instead of several commands. Reach for this when the result of one
call decides the next; use the plain subcommands for simple one-shot lookups.
Calls are synchronous (no ` + "`await`" + `), the body must ` + "`return`" + ` a value, console output
goes to stderr and the return value to stdout. ` + "`--list`" + ` prints the available tools
with TypeScript type stubs; pass ` + "`-`" + ` to read the script from stdin.`,
	Example: `  ghx code 'var r = codemode.explore({repo: "gkoreli/ghx"}); return r.branch;'
  ghx code --list
  ghx code - < script.js`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		listFlag, _ := cmd.Flags().GetBool("list")

		// Build registry (same as serve.go)
		reg := codemode.NewRegistry()
		ghx.RegisterTools(reg)
		tools := reg.Tools()

		if listFlag {
			fmt.Println(codemode.GenerateTypes(tools))
			return nil
		}

		// Get code from arg or stdin
		var code string
		if len(args) == 0 || args[0] == "-" {
			b, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading stdin: %w", err)
			}
			code = string(b)
		} else {
			code = args[0]
		}

		// Execute
		executor := codemode.NewExecutor()
		result, err := executor.Execute(context.Background(), code, tools)
		if err != nil {
			// A snippet that fails to transpile, parse, or run is a bad
			// invocation, not a success: exit 2 so a scripting agent checking
			// $? never treats a non-executed script as OK (dogfood friction,
			// FRICTION.md 2026-07-07 "code-mode transpile error returns exit 0").
			return WithExitCode(ExitBadInvocation, err)
		}

		// Print console output to stderr
		if len(result.Console) > 0 {
			fmt.Fprintln(os.Stderr, strings.Join(result.Console, "\n"))
		}

		// Print result to stdout
		fmt.Println(result.Value)
		return nil
	},
}

func init() {
	codeCmd.Flags().Bool("list", false, "List available tools with TypeScript type stubs")
	RootCmd.AddCommand(codeCmd)
}
