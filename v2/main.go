package main

import (
	"os"

	"github.com/gkoreli/ghx/v2/cmd"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}