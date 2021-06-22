package cli

import (
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"context"
)

// NewRootAndContext creates the root Cobra CLI command and context
func NewRootAndContext() (*cobra.Command, context.Context) {
	flags, cmdCtx := contextWithGlobalFlags(context.Background())

	rootCmd := &cobra.Command{
		Use:   "fury",
		Short: "Command line interface to Gemfury API",
		Long:  `See https://gemfury.com/help/gemfury-cli`,
	}

	// Configure input/output/error streams
	term, auth := terminal.New(), terminal.Netrc()
	cmdCtx = contextWithTerminal(cmdCtx, term, auth)

	// Ensure authentication for all commands except "logout"
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cmd.SetIn(term.IOIn())
		cmd.SetOut(term.IOOut())
		cmd.SetErr(term.IOErr())
		return preRunCheckAuthentication(cmd, args)
	}

	// Global flags (account, verbose, etc)
	rootFlagSet := rootCmd.PersistentFlags()
	rootFlagSet.StringVar(&flags.AuthToken, "api-token", "", "Inline authentication token")
	rootFlagSet.StringVar(&flags.Account, "account", "", "Current account username")
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

	return rootCmd, cmdCtx
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
