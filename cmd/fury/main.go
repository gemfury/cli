package main

import (
	"fmt"
	"github.com/gemfury/cli/cli"
	"os"
)

func main() {
	rootCmd, cmdCtx := cli.NewRootAndContext()
	if err := rootCmd.ExecuteContext(cmdCtx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
