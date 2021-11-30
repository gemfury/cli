package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"fmt"
	"strings"
)

// Root for Git subcommands
func NewCmdGitRoot() *cobra.Command {
	gitCmd := &cobra.Command{
		Use:   "git",
		Short: "Git repository commands",
	}

	gitCmd.AddCommand(NewCmdGitConfig())
	gitCmd.AddCommand(NewCmdGitDestroy())
	gitCmd.AddCommand(NewCmdGitRebuild())
	gitCmd.AddCommand(NewCmdGitRename())
	gitCmd.AddCommand(NewCmdGitList())

	return gitCmd
}

// NewCmdGitDestroy generates the Cobra command for "git:destroy"
func NewCmdGitDestroy() *cobra.Command {
	var resetOnly bool

	destroyCmd := &cobra.Command{
		Use:     "destroy REPO",
		Aliases: []string{"reset"},
		Short:   "Remove Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctx.Terminal(cmd.Context())

			if len(args) != 1 {
				return fmt.Errorf("Please specify a repository")
			}

			// Reset-only when called as "git:reset"
			if cmd.CalledAs() == "reset" {
				resetOnly = true
			}

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			err = c.GitDestroy(cc, args[0], resetOnly)
			if err != nil {
				return err
			}

			if resetOnly {
				term.Printf("Reset %s repository\n", args[0])
			} else {
				term.Printf("Removed %s repository\n", args[0])
			}

			return nil
		},
	}

	// Flags and options
	destroyCmd.Flags().BoolVar(&resetOnly, "reset-only", false, "Reset repo without destroying")

	return destroyCmd
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
	var revisionFlag string

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

			repo, rev := args[0], ""
			if revisionFlag != "" {
				rev = revisionFlag
			} else if at := strings.LastIndex(repo, "@"); at > 0 {
				repo, rev = repo[0:at], repo[at+1:]
			}

			msg := fmt.Sprintf("Building %s repository", repo)
			if rev != "" {
				msg = msg + " at " + rev
			}
			msg = msg + " ...\n"

			term.Printf(msg)
			err = c.GitRebuild(cc, term.IOOut(), repo, rev)
			if err != nil {
				return err
			}

			return nil
		},
	}

	// Flags and options
	rebuildCmd.Flags().StringVarP(&revisionFlag, "revision", "r", "", "Revision")

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
