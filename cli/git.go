package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"fmt"
)

// Root for Git subcommands
func NewCmdGitRoot() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:   "git",
		Short: "Git repository commands",
	}

	gitCmd.AddCommand(NewCmdGitRebuild())
	gitCmd.AddCommand(NewCmdGitRename())
	gitCmd.AddCommand(NewCmdGitReset())
	gitCmd.AddCommand(NewCmdGitList())

	return gitCmd
}

// NewCmdGitReset generates the Cobra command for "git:reset"
func NewCmdGitReset() *cobra.Command {
	resetCmd := &cobra.Command{
		Use:   "reset REPO",
		Short: "Remove Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctx.Terminal(cmd.Context())

			if len(args) != 1 {
				return fmt.Errorf("Please specify a repository")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			err = c.GitReset(cc, args[0])
			if err != nil {
				return err
			}

			term.Printf("Removed %s repository\n", args[0])
			return nil
		},
	}

	return resetCmd
}

// NewCmdGitRename generates the Cobra command for "git:reset"
func NewCmdGitRename() *cobra.Command {
	renameCmd := &cobra.Command{
		Use:   "rename REPO NEWNAME",
		Short: "Rename a Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctx.Terminal(cmd.Context())

			if len(args) != 2 {
				return fmt.Errorf("Please specify a repository")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			err = c.GitRename(cc, args[0], args[1])
			if err != nil {
				return err
			}

			term.Printf("Renamed %s repository to %s\n", args[0], args[1])
			return nil
		},
	}

	return renameCmd
}

// NewCmdGitConfigSet sets build configuration keys
func NewCmdGitRebuild() *cobra.Command {
	rebuildCmd := &cobra.Command{
		Use:   "rebuild REPO",
		Short: "Run the builder on the repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctx.Terminal(cmd.Context())

			if len(args) != 1 {
				return fmt.Errorf("Please specify a repository")
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			term.Printf("Building %s repository...\n", args[0])
			err = c.GitRebuild(cc, term.IOOut(), args[0])
			if err != nil {
				return err
			}

			return nil
		},
	}

	return rebuildCmd
}

// NewCmdGitConfigSet lists Git repositories
func NewCmdGitList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List repos in this account",
		RunE:  listRepos,
	}
}

func listRepos(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	term := ctx.Terminal(cc)
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	repos := []*api.GitRepo{}

	// Paginate over package listings until no more pages
	err = iterateAllPages(cc, func(pageReq *api.PaginationRequest) (*api.PaginationResponse, error) {
		resp, err := c.GitList(cc, pageReq)
		if err != nil {
			return nil, err
		}

		repos = append(repos, resp.Root.Repos...)
		return resp.Pagination, nil
	})

	// Handle no packages
	if len(repos) == 0 {
		term.Println("No Git repositories found in this account")
		return err
	}

	// Print results
	term.Printf("\n*** GEMFURY GIT REPOS ***\n\n")
	for _, r := range repos {
		term.Printf("%s\n", r.Name)
	}

	return err
}
