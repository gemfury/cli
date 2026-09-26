package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"context"
	"fmt"
)

// NewRootCommand creates the root Cobra CLI command and context
func NewRootCommand(cc context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "fury",
		Short: "Command line interface to Gemfury API",
		Long:  `See https://gemfury.com/help/gemfury-cli`,

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
		return usageErrorf("%s", err)
	})

	// Ensure authentication for all commands (see skipsAuth for exceptions)
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return preRunCheckAuthentication(cmd, args)
	}

	// Global flags (account, verbose, etc)
	flags := ctx.GlobalFlags(cc)
	rootFlagSet := rootCmd.PersistentFlags()
	rootFlagSet.StringVar(&flags.AuthToken, "api-token", "", "Inline authentication token")
	rootFlagSet.StringVarP(&flags.Account, "account", "a", "", "Current account username")
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
	fmt.Fprintf(errOut, "Error: %s\n", err)

	if IsUsageError(err) {
		fmt.Fprint(errOut, cmd.UsageString())
	} else if cmd == rootCmd {
		// Unknown subcommand, or other failure to dispatch
		fmt.Fprintf(errOut, "Run '%s --help' for usage.\n", rootCmd.CommandPath())
	}

	return err
}

func globalFlagNormalization(f *pflag.FlagSet, name string) pflag.NormalizedName {

	// Apply aliases for legacy flags
	switch name {
	case "as":
		name = "account"
		break
	}

	return pflag.NormalizedName(name)
}
