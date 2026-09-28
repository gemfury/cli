package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"
)

// NewCmdGitStack is the root for Git Stack
func NewCmdGitStack() *cobra.Command {
	gitStackCmd := &cobra.Command{
		Use:   "stack REPO",
		Short: "Configure Git stack",
		Long: `List the stacks that a repository can be built on, marking
the current one, or set another.`,
		Example: `  fury git stack my-repo`,
		Args:    repoArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			return gitStackForRepo(cmd, args[0])
		},
	}

	gitStackCmd.AddCommand(NewCmdGitStackSet())

	return gitStackCmd
}

// gitStackForRepo lists the build stacks, marking that of the repo
func gitStackForRepo(cmd *cobra.Command, repoName string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	repo, err := c.GitInfo(cc, repoName)
	if err != nil {
		return about("Repository", repoName, err)
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

// NewCmdGitStackSet sets the build stack of a repo
func NewCmdGitStackSet() *cobra.Command {
	gitStackSetCmd := &cobra.Command{
		Use:     "set REPO STACK",
		Short:   "Set Git stack for repo",
		Long:    `Set Git stack for repo, to one that "fury git stack REPO" lists.`,
		Example: `  fury git stack set my-repo NAME`,
		Args:    usageArgs(cobra.ExactArgs(2), "Please specify a repository and a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {
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
		return about("Repository", repo, err)
	}

	term.Printf("Updated %s repository build stack\n", repo)
	return nil
}
