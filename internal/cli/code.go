package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gkoreli/ghx/v2/pkg/codemode"
	"github.com/gkoreli/ghx/v2/pkg/ghx"
	"github.com/spf13/cobra"
)

var codeCmd = &cobra.Command{
	Use:   "code [code | -]",
	Short: "Execute JavaScript with access to all ghx tools",
	Args:  cobra.MaximumNArgs(1),
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
			return err
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
