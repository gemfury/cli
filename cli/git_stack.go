package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"fmt"
)

// NewCmdGitStack is the root for Git Config
func NewCmdGitStack() *cobra.Command {
	gitStackCmd := &cobra.Command{
		Use:   "stack REPO",
		Short: "Configure Git stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return fmt.Errorf("Command requires a repository argument")
			}
			return gitStackForRepo(cmd, args[0])
		},
	}

	gitStackCmd.AddCommand(NewCmdGitStackSet())

	return gitStackCmd
}

// Filtered/unfiltered retrieval of Git Config for commands above
func gitStackForRepo(cmd *cobra.Command, repoName string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	repo, err := c.GitInfo(cc, repoName)
	if err != nil {
		return err
	}

	stacks, err := c.GitStacks(cc)
	if err != nil {
		return err
	}

	term.Printf("*** [%s] GIT BUILD STACKS ***\n", repo.Name)

	for _, s := range stacks {
		if s.Name == repo.Stack.Name {
			term.Printf("*")
		} else {
			term.Printf(" ")
		}
		term.Printf(" %s\n", s.Name)
	}

	return nil
}

// NewCmdGitStackSet updates one or more configuration keys
func NewCmdGitStackSet() *cobra.Command {
	gitStackSetCmd := &cobra.Command{
		Use:   "set REPO STACK",
		Short: "Set Git stack for repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return fmt.Errorf("Please specify a repository and a stack")
			}
			return gitStackUpdate(cmd, args[0], args[1])
		},
	}

	return gitStackSetCmd
}

func gitStackUpdate(cmd *cobra.Command, repo string, newStack string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	err = c.GitStackSet(cc, repo, newStack)
	if err != nil {
		return err
	}

	term.Printf("Updated %s repository build stack\n", repo)
	return nil
}
