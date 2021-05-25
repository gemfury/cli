package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"context"
)

func NewRootAndContext() (*cobra.Command, context.Context) {
	flags, cmdCtx := contextWithGlobalFlags(context.Background())

	rootCmd := &cobra.Command{
		Use:   "fury",
		Short: "Command line interface to Gemfury API",
		Long:  `See https://gemfury.com/help/gemfury-cli`,
	}

	// Global flags (account, verbose, etc)
	rootFlagSet := rootCmd.PersistentFlags()
	rootFlagSet.StringVar(&flags.Account, "account", "", "Current account")
	rootCmd.SetGlobalNormalizationFunc(globalFlagNormalization)

	// Connect child commands
	rootCmd.AddCommand(NewCmdWhoAmI())
	rootCmd.AddCommand(NewCmdSharingAdd())

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
