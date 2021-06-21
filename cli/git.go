package cli

import (
	"fmt"
	"github.com/spf13/cobra"
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

	return gitCmd
}

// NewCmdGitReset generates the Cobra command for "git:reset"
func NewCmdGitReset() *cobra.Command {
	resetCmd := &cobra.Command{
		Use:   "reset REPO",
		Short: "Remove Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			term := ctxTerminal(cmd.Context())

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
			term := ctxTerminal(cmd.Context())

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
			term := ctxTerminal(cmd.Context())

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
