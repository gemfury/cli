package cli

import (
	"github.com/spf13/cobra"

	"fmt"
)

// NewCmdGitReset generates the Cobra command for "git:reset"
func NewCmdGitReset() *cobra.Command {
	resetCmd := &cobra.Command{
		Use:   "git:reset REPO",
		Short: "Remove Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			fmt.Printf("Removed %s repository\n", args[0])
			return nil
		},
	}

	return resetCmd
}

// NewCmdGitRename generates the Cobra command for "git:reset"
func NewCmdGitRename() *cobra.Command {
	renameCmd := &cobra.Command{
		Use:   "git:rename REPO NEWNAME",
		Short: "Rename a Git repository",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			fmt.Printf("Renamed %s repository to %s\n", args[0], args[1])
			return nil
		},
	}

	return renameCmd
}
