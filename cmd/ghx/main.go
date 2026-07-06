package main

import (
	"fmt"
	"os"

	"github.com/gkoreli/ghx/v2/internal/cli"
	"github.com/gkoreli/ghx/v2/skills"
)

func main() {
	cli.SkillMD = skills.SkillMD
	cli.MCPSkillMD = skills.MCPSkillMD
	cli.ReconSkillMD = skills.ReconSkillMD
	if err := cli.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
