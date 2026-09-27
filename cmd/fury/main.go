package main

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/cli"

	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
)

// Populated by GoReleaser
var (
	Version = "dev"
)

func main() {
	cc, caught := interruptible(cli.CommandContext())
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
	os.Exit(exitStatus(err, caught()))
}

// interruptible derives the context of the running command, which is
// cancelled when the process is interrupted. From then on signals are back
// to their default handling, so that a second Ctrl-C ends a command that is
// slow to wind down. Once the context is done, caught returns the signal.
func interruptible(parent context.Context) (cc context.Context, caught func() os.Signal) {
	cc, cancel := context.WithCancel(parent)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	var sig atomic.Value
	go func() {
		sig.Store(<-signals)
		signal.Stop(signals)
		cancel()
	}()

	return cc, func() os.Signal {
		s, _ := sig.Load().(os.Signal)
		return s
	}
}

// exitStatus is the exit status for the error of a command, and for the
// signal that interrupted it, if any. As in a shell, an interrupted command
// exits with 128 plus the number of the signal, e.g. 130 for Ctrl-C.
func exitStatus(err error, sig os.Signal) int {
	switch {
	case err == nil:
		return 0
	case !errors.Is(err, context.Canceled):
		return 1
	}

	// Ctrl-C at a prompt is read as a key, rather than received as a signal
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 128 + int(syscall.SIGINT)
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
