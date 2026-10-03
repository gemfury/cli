package cli

import (
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"fmt"
	"strings"
)

// repoArg is the Args check for commands taking a single REPO
var repoArg = usageArgs(cobra.ExactArgs(1), "Please specify exactly one repository")

// Root for Git subcommands
func NewCmdGitRoot() *cobra.Command {
	gitCmd := groupCommand(&cobra.Command{
		Use:   "git",
		Short: "Git repository commands",
	})

	gitCmd.AddCommand(NewCmdGitConfig())
	gitCmd.AddCommand(NewCmdGitCredentials())
	gitCmd.AddCommand(NewCmdGitDestroy())
	gitCmd.AddCommand(NewCmdGitRebuild())
	gitCmd.AddCommand(NewCmdGitRename())
	gitCmd.AddCommand(NewCmdGitStack())
	gitCmd.AddCommand(NewCmdGitList())

	return gitCmd
}

// NewCmdGitDestroy generates the Cobra command for "git:destroy"
func NewCmdGitDestroy() *cobra.Command {
	var resetOnly bool
	var forceFlag bool

	destroyCmd := &cobra.Command{
		Use:     "destroy REPO",
		Aliases: []string{"reset"},
		Short:   "Remove Git repository",
		Long: `Remove Git repository, once confirmed. As "reset", or with
--reset-only, the repository is reset and not removed.`,
		Example: `  fury git destroy my-repo
  fury git destroy my-repo --force
  fury git reset my-repo`,
		Args: repoArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)

			// Reset-only when called as "git:reset"
			if cmd.CalledAs() == "reset" {
				resetOnly = true
			}

			action, done := "remove", "Removed"
			if resetOnly {
				action, done = "reset", "Reset"
			}

			if !forceFlag {
				confirm := fmt.Sprintf("Are you sure you want to %s the %s repository? [y/N]", action, args[0])
				if ok, err := term.Confirm(confirm); !ok {
					return err
				}
			}

			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			err = c.GitDestroy(cc, args[0], resetOnly)
			if err != nil {
				return about("Repository", args[0], err)
			}

			term.Infof("%s %s repository\n", done, args[0])
			return nil
		},
	}

	// Flags and options
	destroyCmd.Flags().BoolVar(&resetOnly, "reset-only", false, "Reset repo without destroying")
	destroyCmd.Flags().BoolVarP(&forceFlag, "force", "f", false, "Skip confirmation")

	return destroyCmd
}

// NewCmdGitRename generates the Cobra command for "git:rename"
func NewCmdGitRename() *cobra.Command {
	renameCmd := &cobra.Command{
		Use:     "rename REPO NEWNAME",
		Short:   "Rename a Git repository",
		Example: `  fury git rename my-repo new-name`,
		Args:    usageArgs(cobra.ExactArgs(2), "Please specify a repository and its new name"),
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctx.Terminal(cmd.Context())

			cc := cmd.Context()
			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			err = c.GitRename(cc, args[0], args[1])
			if err != nil {
				return about("Repository", args[0], err)
			}

			term.Infof("Renamed %s repository to %s\n", args[0], args[1])
			return nil
		},
	}

	return renameCmd
}

// NewCmdGitRebuild runs the builder on a repository
func NewCmdGitRebuild() *cobra.Command {
	var revisionFlag string

	rebuildCmd := &cobra.Command{
		Use:   "rebuild REPO",
		Short: "Run the builder on the repo",
		Example: `  fury git rebuild my-repo
  fury git rebuild my-repo --revision v1.0.0
  fury git rebuild my-repo@v1.0.0`,
		Args: repoArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctx.Terminal(cmd.Context())

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

			msg, name := "Building "+repo+" repository", repo
			if rev != "" {
				msg, name = msg+" at "+rev, name+"@"+rev
			}

			term.Infof("%s ...\n", msg)
			err = c.GitRebuild(cc, term.IOOut(), repo, rev)
			return about("Repository", name, err)
		},
	}

	// Flags and options
	rebuildCmd.Flags().StringVarP(&revisionFlag, "revision", "r", "", "Revision to build: a branch, a tag, or a commit")

	return rebuildCmd
}

// NewCmdGitList lists Git repositories
func NewCmdGitList() *cobra.Command {
	return jsonCommand(&cobra.Command{
		Use:   "list",
		Short: "List repos in this account",
		Example: `  fury git list
  fury git list --account my-org`,
		Args: noArgs,
		RunE: listRepos,
	})
}

func listRepos(cmd *cobra.Command, args []string) error {
	cc := cmd.Context()
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	repos, err := fetchAll[*api.GitRepo](cc, c.GitList)
	return printListing(cmd, repos, err, "No Git repositories found in this account", func(term terminal.Terminal) {
		term.Infof("\n*** GEMFURY GIT REPOS ***\n\n")
		for _, r := range repos {
			term.Printf("%s\n", r.Name)
		}
	})
}
