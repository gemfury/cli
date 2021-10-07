package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"
	"sort"
	"strings"
	"text/tabwriter"

	"fmt"
)

// NewCmdGitConfig is the root for Git Config
func NewCmdGitConfig() *cobra.Command {
	gitConfigCmd := &cobra.Command{
		Use:   "config",
		Short: "Configure Git build",
		RunE: func(cmd *cobra.Command, args []string) error {
			return filteredGitConfig(cmd, args, false)
		},
	}

	gitConfigCmd.AddCommand(NewCmdGitConfigSet())
	gitConfigCmd.AddCommand(NewCmdGitConfigGet())

	return gitConfigCmd
}

// NewCmdGitConfigGet updates one or more configuration keys
func NewCmdGitConfigGet() *cobra.Command {
	gitConfigGetCmd := &cobra.Command{
		Use:   "get KEY",
		Short: "Get Git build environment key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return filteredGitConfig(cmd, args, true)
		},
	}

	return gitConfigGetCmd
}

// Filtered/unfiltered retrieval of Git Config for commands above
func filteredGitConfig(cmd *cobra.Command, args []string, filter bool) error {
	if filter && len(args) < 2 {
		return fmt.Errorf("Please specify a repository and a key")
	} else if !filter && len(args) != 1 {
		return fmt.Errorf("Command requires only a repository")
	}

	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	config, err := c.GitConfig(cc, args[0])
	if err != nil {
		return err
	}

	filteredConfig := config
	if keys := args[1:]; filter && len(keys) > 0 {
		filteredConfig = make([]api.GitConfigPair, 0, len(keys))
		for _, c := range config {
			for _, k := range keys {
				if c.Key == k {
					filteredConfig = append(filteredConfig, c)
					break
				}
			}
		}
	}

	if len(filteredConfig) > 0 {
		sort.Slice(filteredConfig, func(i, j int) bool {
			return filteredConfig[i].Key < filteredConfig[j].Key
		})
	}

	term.Printf("\n*** GIT CONFIG ***\n\n")
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)

	for _, c := range filteredConfig {
		fmt.Fprintf(w, "%s:\t%s\n", c.Key, c.Value)
	}

	w.Flush()
	return nil
}

// NewCmdGitConfigSet updates one or more configuration keys
func NewCmdGitConfigSet() *cobra.Command {
	gitConfigSetCmd := &cobra.Command{
		Use:   "set KEY=VAL",
		Short: "Set Git build environment key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("Please specify a repository and a KEY=VALUE")
			}

			cc := cmd.Context()
			term := ctx.Terminal(cc)
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			vars := map[string]string{}
			for _, pairStr := range args[1:] {
				pair := strings.SplitN(pairStr, "=", 2)
				if len(pair) != 2 {
					return fmt.Errorf("Argument has no value: %s", pairStr)
				}
				vars[pair[0]] = pair[1]
			}

			err = c.GitConfigSet(cc, args[0], vars)
			if err != nil {
				return err
			}

			term.Printf("Updated %s repository config\n", args[0])
			return nil
		},
	}

	return gitConfigSetCmd
}
