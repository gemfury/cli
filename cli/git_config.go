package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"
	"strings"
	"text/tabwriter"

	"fmt"
	"maps"
	"slices"
)

// NewCmdGitConfig is the root for Git Config
func NewCmdGitConfig() *cobra.Command {
	gitConfigCmd := jsonCommand(&cobra.Command{
		Use:   "config REPO",
		Short: "Configure Git build",
		Long: `List the build environment of a Git repository,
or get, set, and unset its keys.`,
		Example: `  fury git config my-repo`,
		Args:    repoArg,
		RunE:    filteredGitConfig,
	})

	gitConfigCmd.AddCommand(NewCmdGitConfigSet())
	gitConfigCmd.AddCommand(NewCmdGitConfigGet())
	gitConfigCmd.AddCommand(NewCmdGitConfigUnset())

	return gitConfigCmd
}

// NewCmdGitConfigGet retrieves one or more configuration keys
func NewCmdGitConfigGet() *cobra.Command {
	gitConfigGetCmd := jsonCommand(&cobra.Command{
		Use:   "get REPO KEY...",
		Short: "Get Git build environment key",
		Example: `  fury git config get my-repo KEY
  fury git config get my-repo KEY OTHER`,
		Args: usageArgs(cobra.MinimumNArgs(2), "Please specify a repository and at least one key"),
		RunE: filteredGitConfig,
	})

	return gitConfigGetCmd
}

// Filtered/unfiltered retrieval of Git Config for commands above
func filteredGitConfig(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	vars, err := c.GitConfig(cc, args[0])
	if err != nil {
		return about("Repository", args[0], err)
	}

	if keys := args[1:]; len(keys) > 0 {
		maps.DeleteFunc(vars, func(k, _ string) bool {
			return !slices.Contains(keys, k)
		})
	}

	// As an object, whose keys the encoder sorts
	if printsJSON(cmd) {
		return termPrintJSON(term, vars)
	}

	term.Infof("\n*** GIT CONFIG ***\n\n")
	w := tabwriter.NewWriter(term.IOOut(), 0, 0, 2, ' ', 0)

	for _, k := range slices.Sorted(maps.Keys(vars)) {
		fmt.Fprintf(w, "%s:\t%s\n", k, vars[k])
	}

	w.Flush()
	return nil
}

// NewCmdGitConfigSet updates one or more configuration keys
func NewCmdGitConfigSet() *cobra.Command {
	gitConfigSetCmd := &cobra.Command{
		Use:   "set REPO KEY=VAL...",
		Short: "Set Git build environment key",
		Example: `  fury git config set my-repo KEY=value
  fury git config set my-repo KEY=value OTHER=value`,
		Args: cobra.MatchAll(
			usageArgs(cobra.MinimumNArgs(2), "Please specify a repository and at least one KEY=VALUE"),
			func(cmd *cobra.Command, args []string) error {
				for _, pair := range args[1:] {
					if !strings.Contains(pair, "=") {
						return usageErrorf("Argument has no value: %s", pair)
					}
				}
				return nil
			},
		),
		RunE: func(cmd *cobra.Command, args []string) error {
			vars := map[string]*string{}
			for _, pair := range args[1:] {
				key, val, _ := strings.Cut(pair, "=")
				vars[key] = &val
			}

			return gitConfigUpdate(cmd, args[0], vars)
		},
	}

	return gitConfigSetCmd
}

// NewCmdGitConfigUnset removes one or more configuration keys
func NewCmdGitConfigUnset() *cobra.Command {
	gitConfigUnsetCmd := &cobra.Command{
		Use:   "unset REPO KEY...",
		Short: "Remove Git build environment key",
		Example: `  fury git config unset my-repo KEY
  fury git config unset my-repo KEY OTHER`,
		Args: usageArgs(cobra.MinimumNArgs(2), "Please specify a repository and at least one KEY"),
		RunE: func(cmd *cobra.Command, args []string) error {
			vars := map[string]*string{}
			for _, key := range args[1:] {
				vars[key] = nil
			}

			return gitConfigUpdate(cmd, args[0], vars)
		},
	}

	return gitConfigUnsetCmd
}

func gitConfigUpdate(cmd *cobra.Command, repo string, vars map[string]*string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	err = c.GitConfigSet(cc, repo, vars)
	if err != nil {
		return about("Repository", repo, err)
	}

	term.Infof("Updated %s repository config\n", repo)
	return nil
}
