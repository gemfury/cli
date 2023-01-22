package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"context"
)

// NewRootCommand creates the root Cobra CLI command and context
func NewRootCommand(cc context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "fury",
		Short: "Command line interface to Gemfury API",
		Long:  `See https://gemfury.com/help/gemfury-cli`,
	}

	// Connect I/O
	term := ctx.Terminal(cc)
	rootCmd.SetIn(term.IOIn())
	rootCmd.SetOut(term.IOOut())
	rootCmd.SetErr(term.IOErr())

	// // Ensure authentication for all commands except "logout"
	// rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
	// 	return preRunCheckAuthentication(cmd, args)
	// }

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

func globalFlagNormalization(f *pflag.FlagSet, name string) pflag.NormalizedName {

	// Apply aliases for legacy flags
	switch name {
	case "as":
		name = "account"
		break
	}

	return pflag.NormalizedName(name)
}
