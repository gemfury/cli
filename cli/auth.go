package cli

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"fmt"
)

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogout() *cobra.Command {
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear & invalidate CLI session credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := promptui.Prompt{
				Label:   "Are you sure you want to logout? [y/N]",
				Default: "N",
			}

			result, err := prompt.Run()
			if err != nil {
				return err
			}

			if result != "y" && result != "Y" {
				return nil
			}

			c, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}

			if err := c.Logout(cmd.Context()); err != nil {
				return err
			}

			machines := []string{"api.fury.io", "git.fury.io"}
			if err := netrcWipe(machines); err != nil {
				return err
			}

			fmt.Println("You have been logged out")
			return nil
		},
	}

	return logoutCmd
}
