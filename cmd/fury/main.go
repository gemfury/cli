package main

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"

	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

// Exit status for a command that was interrupted (128 + SIGINT)
const exitInterrupted = 130

// Populated by GoReleaser
var (
	Version = "dev"
)

func main() {
	// Interrupting the process cancels the context of the running command.
	// From then on signals are back to their default handling, so that
	// a second Ctrl-C ends a command that is slow to wind down.
	cc, stop := signal.NotifyContext(cli.CommandContext(), os.Interrupt, syscall.SIGTERM)
	context.AfterFunc(cc, stop)

	rootCmd := cli.NewRootCommand(cc)

	// Populate version strings everywhere
	api.DefaultConduit.Version = Version
	rootCmd.Version = Version

	// Support for legacy (Ruby) CLI command
	if args := convertLegacyArgs(os.Args); args != nil {
		rootCmd.SetArgs(args)
	}

	// Execute reports any error; only the exit status is left to set
	err := cli.Execute(cc, rootCmd)
	stop() // os.Exit skips deferred calls

	if errors.Is(err, context.Canceled) {
		os.Exit(exitInterrupted)
	} else if err != nil {
		os.Exit(1)
	}
}

// Convert legacy (Ruby CLI) commands with ":" separator
// Some special-casing is based on Cobra's arg processing
// TODO: This could be moved to Ruby CLI as a wrapper
func convertLegacyArgs(args []string) []string {
	if len(args) < 2 {
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
