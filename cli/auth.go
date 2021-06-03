package cli

import (
	"github.com/gemfury/cli/api"
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
			fmt.Println("Please enter your Gemfury credentials.")

			ePrompt := promptui.Prompt{Label: "Email: "}
			eResult, err := ePrompt.Run()
			if err != nil {
				return err
			}

			pPrompt := promptui.Prompt{Label: "Password: ", Mask: '*'}
			pResult, err := pPrompt.Run()
			if err != nil {
				return err
			}

			c, err := newAPIClient(cmd.Context())
			if err != nil {
				return err
			}

			req := api.LoginRequest{Email: eResult, Password: pResult}
			resp, err := c.Login(cmd.Context(), &req)
			if err == api.ErrUnauthorized {
				cmd.SilenceErrors = true
				cmd.SilenceUsage = true
				return err
			} else if err != nil {
				return err
			}

			// Save credentials in .netrc
			err = netrcAppend(netrcMachines, resp.User.Email, resp.Token)
			if err != nil {
				return err
			}

			fmt.Printf("You are logged in as %q\n", resp.User.Name)
			return nil
		},
	}

	return loginCmd
}
