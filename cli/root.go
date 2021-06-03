package cli

import (
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

	// Ensure authentication for all commands except "logout"
	rootCmd.PersistentPreRunE = preRunCheckAuthentication

	// Global flags (account, verbose, etc)
	rootFlagSet := rootCmd.PersistentFlags()
	rootFlagSet.StringVar(&flags.AuthToken, "api-token", "", "Inline authentication token")
	rootFlagSet.StringVar(&flags.Account, "account", "", "Current account username")
	rootCmd.SetGlobalNormalizationFunc(globalFlagNormalization)

	// Connect child commands
	rootCmd.AddCommand(NewCmdPush())
	rootCmd.AddCommand(NewCmdYank())
	rootCmd.AddCommand(NewCmdWhoAmI())
	rootCmd.AddCommand(NewCmdSharingRoot())
	rootCmd.AddCommand(NewCmdGitRoot())
	rootCmd.AddCommand(NewCmdLogout())
	rootCmd.AddCommand(NewCmdLogin())

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
