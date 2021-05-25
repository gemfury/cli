package main

import (
	"fmt"
	"github.com/gemfury/cli/cli"
	"os"
)

func main() {
	rootCmd, cmdCtx := cli.NewRootAndContext()
	if err := rootCmd.ExecuteContext(cmdCtx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}
