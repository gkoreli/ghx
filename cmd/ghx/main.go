package main

import (
	"fmt"
	"os"

	"github.com/gkoreli/ghx/v2/internal/cli"
	"github.com/gkoreli/ghx/v2/internal/skilldoc"
)

func main() {
	cli.SkillMD = skilldoc.SkillMD
	cli.MCPSkillMD = skilldoc.MCPSkillMD
	if err := cli.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
