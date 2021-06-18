package main

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"

	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Populated by GoReleaser
var (
	Version = "dev"
)

func main() {
	rootCmd, cmdCtx := cli.NewRootAndContext()

	// Populate version strings everywhere
	api.DefaultConduit.Version = Version
	rootCmd.Version = Version

	// Support for legacy (Ruby) CLI command
	if args := convertLegacyArgs(os.Args); args != nil {
		rootCmd.SetArgs(args)
	}

	// Process command and deliver results
	if err := rootCmd.ExecuteContext(cmdCtx); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
}

// Convert legacy (Ruby CLI) commands with ":" separator
// Some special-casing is based on Cobra's arg processing
// TODO: This could be moved to Ruby CLI as a wrapper
func convertLegacyArgs(args []string) []string {

	if len(args) < 2 || filepath.Base(args[0]) == "cobra.test" {
		return nil
	}

	firstArg := args[1]

	// Thor doesn't support flags before first subcommand, so
	// a legacy CLI user would receive a failure anyway
	if strings.HasPrefix(firstArg, "-") {
		return nil
	}

	// Split colon-divided command into multiple subdommands
	subcommands := strings.Split(firstArg, ":")
	return append(subcommands, args[2:]...)
}
