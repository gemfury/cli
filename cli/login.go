package cli

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"fmt"
)

// Machines for Gemfury in .netrc file
var (
	netrcMachines = []string{"api.fury.io", "git.fury.io"}
)

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogout() *cobra.Command {
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear CLI session credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token, err := netrcAuth(); err != nil {
				return err
			} else if token == "" {
				fmt.Println("You are logged out")
				return nil
			}

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

			if err := netrcWipe(netrcMachines); err != nil {
				return err
			}

			fmt.Println("You have been logged out")
			return nil
		},
	}

	return logoutCmd
}

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogin() *cobra.Command {
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate into Gemfury account",
		RunE: func(cmd *cobra.Command, args []string) error {
			user, err := ensureAuthenticated(cmd)
			if err != nil {
				return err
			}

			if user == nil {
				user, err = whoAMI(cmd.Context())
				if err != nil {
					return err
				}
			}

			fmt.Printf("You are logged in as %q\n", user.Name)
			return nil
		},
	}

	return loginCmd
}
