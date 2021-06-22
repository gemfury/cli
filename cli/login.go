package cli

import (
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
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
			cc := cmd.Context()
			term := ctxTerminal(cc)
			auth := ctxAuther(cc)

			if _, token, err := auth.Auth(); err != nil {
				return err
			} else if token == "" {
				term.Println("You are logged out")
				return nil
			}

			prompt := promptui.Prompt{
				Label:   "Are you sure you want to logout? [y/N]",
				Default: "N",
			}

			result, err := term.RunPrompt(&prompt)
			if err != nil {
				return err
			}

			if result != "y" && result != "Y" {
				return nil
			}

			c, err := newAPIClient(cc)
			if err != nil {
				return err
			}

			if err := c.Logout(cc); err != nil {
				return err
			}

			if err := ctxAuther(cc).Wipe(); err != nil {
				return err
			}

			term.Println("You have been logged out")
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

			term := ctxTerminal(cmd.Context())
			term.Printf("You are logged in as %q\n", user.Name)
			return nil
		},
	}

	return loginCmd
}
