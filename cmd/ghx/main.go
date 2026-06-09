package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/gkoreli/ghx/v2/cmd"
)

//go:embed SKILL.md
var skillMD string

//go:embed MCP-SKILL.md
var mcpSkillMD string

func main() {
	cmd.SkillMD = skillMD
	cmd.MCPSkillMD = mcpSkillMD
	if err := cmd.RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
