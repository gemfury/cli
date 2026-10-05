package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"context"
	"errors"
	"fmt"
)

// NewRootCommand creates the root Cobra CLI command and context
func NewRootCommand(cc context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "fury",
		Short: "Command line interface to Gemfury API",
		Long: `Manage the packages, collaborators, and Git repositories of a Gemfury
account. See https://gemfury.com/help/gemfury-cli

Commands act on your own account, or on the one given by --account.
They authenticate by --api-token, or else by FURY_TOKEN, or else by
the session that "fury login" has saved. With --api-token -, the token
is read from stdin, or asked for at the terminal, so it is not in the
command line.

Environment variables:
  FURY_TOKEN     Authentication token, unless --api-token is given
  FURY_ACCOUNT   Account to act on, unless --account is given
  FURY_RETRIES   Times a request that reads is sent again (default 3)

Without a terminal, or with --no-input, nothing is asked: pass --yes
to confirm, and set FURY_TOKEN to authenticate.

With --quiet, only results, warnings, and errors are shown: the exit
status tells whether a change was made.

A request that reads is sent again when it is rate limited, the server
is unavailable, or the connection fails: FURY_RETRIES times, after a
wait of a few seconds, or as long as the server asks. A request that
takes longer than --timeout fails with exit status 6.

With --json, on the commands whose help lists it, the result is printed
as JSON on stdout, and nothing else is. Every field of a record is
printed, empty when the API does not provide it. Errors are as without
it, and nothing is asked.

Exit status:
  0   Success
  1   Any other error
  2   Usage: check the arguments and flags, or confirm with --yes
  3   Not found
  4   Not authenticated
  5   Already exists
  6   Unavailable: try again later
  130 Interrupted, or 143 when terminated`,
		Example: `  fury push package-1.0.0.tgz
  fury versions package --account my-org
  FURY_TOKEN=token fury yank package@1.0.0 --yes`,

		// Execute reports errors and usage, not Cobra
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	// Connect I/O
	term := ctx.Terminal(cc)
	rootCmd.SetIn(term.IOIn())
	rootCmd.SetOut(term.IOOut())
	rootCmd.SetErr(term.IOErr())

	// Flag parsing failures are usage errors
	rootCmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return asUsageError(err)
	})

	// Global flags (account, verbose, etc)
	flags := ctx.GlobalFlags(cc)

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		// Flags are parsed by now: questions are answered as they ask.
		// With --json none is asked, as it would be on stdout with the result.
		cc := cmd.Context()
		asJSON := printsJSON(cmd)
		flags.Quiet = flags.Quiet || asJSON
		term := terminal.Unattended(ctx.Terminal(cc), flags.Yes, flags.NoInput || asJSON)
		term = terminal.Quiet(term, flags.Quiet, flags.NoProgress)
		cmd.SetContext(ctx.WithTerminal(cc, term))

		// Ensure authentication for all commands (see skipsAuth for exceptions)
		return preRunCheckAuthentication(cmd, args)
	}

	rootFlagSet := rootCmd.PersistentFlags()
	rootFlagSet.StringVar(&flags.AuthToken, "api-token", "", "Authentication token; - reads it from stdin (or set FURY_TOKEN)")
	rootFlagSet.DurationVar(&flags.HTTPTimeout, "timeout", 0, "Give up on a request after this long (default 30s, or 10m for an upload or download)")
	rootFlagSet.StringVarP(&flags.Account, "account", "a", "", "Account to act on, if not your own (or set FURY_ACCOUNT)")
	rootFlagSet.BoolVarP(&flags.Yes, "yes", "y", false, "Answer yes to every confirmation")
	rootFlagSet.BoolVar(&flags.NoInput, "no-input", false, "Never ask; fail where input is needed")
	rootFlagSet.BoolVarP(&flags.Quiet, "quiet", "q", false, "Show only results, warnings, and errors")
	rootFlagSet.BoolVar(&flags.NoProgress, "no-progress", false, "Do not show progress bars and spinners")
	rootCmd.SetGlobalNormalizationFunc(globalFlagNormalization)

	// Connect child commands
	rootCmd.AddCommand(
		NewCmdPush(),
		NewCmdYank(),
		NewCmdWhoAmI(),
		NewCmdPackages(),
		NewCmdVersions(),
		NewCmdSharingRoot(),
		NewCmdAccounts(),
		NewCmdGitRoot(),
		NewCmdLogout(),
		NewCmdLogin(),
		// Beta/hidden experiments, etc
		NewCmdBeta(),
	)

	// FIXME: Disable "completion" command
	rootCmd.CompletionOptions = cobra.CompletionOptions{
		DisableDefaultCmd: true,
	}

	return rootCmd
}

// Execute runs the root command and reports any failure on its error
// stream, exactly once. Usage text accompanies only usage errors.
func Execute(cc context.Context, rootCmd *cobra.Command) error {
	cmd, err := rootCmd.ExecuteContextC(cc)
	if err == nil {
		return nil
	}

	errOut := rootCmd.ErrOrStderr()

	// An interrupted command has nothing to report but the interruption
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(errOut, "Cancelled")
		return err
	}

	fmt.Fprintf(errOut, "Error: %s\n", err)

	if IsUsageError(err) {
		fmt.Fprint(errOut, cmd.UsageString())
	} else if cmd == rootCmd {
		// Unknown subcommand, or other failure to dispatch: a usage
		// error, though with a hint instead of the usage of the root
		fmt.Fprintf(errOut, "Run '%s --help' for usage.\n", rootCmd.CommandPath())
		return asUsageError(err)
	}

	return err
}

func globalFlagNormalization(f *pflag.FlagSet, name string) pflag.NormalizedName {

	// Apply aliases for legacy flags
	switch name {
	case "as":
		name = "account"
	}

	return pflag.NormalizedName(name)
}
