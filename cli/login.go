package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
	"errors"
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
			term := ctx.Terminal(cc)
			auth := ctx.Auther(cc)

			if _, token, err := auth.Auth(); err != nil {
				return err
			} else if token == "" {
				term.Println("You are logged out")
				return nil
			}

			confirm := "Are you sure you want to logout? [y/N]"
			if ok, err := terminal.PromptConfirm(term, confirm); !ok {
				return err
			}

			if err := logoutCurrent(cc, false); err != nil {
				return err
			}

			term.Println("You have been logged out")
			return nil
		},
	}

	return logoutCmd
}

// Deactivates & deletes the saved CLI token, if present
func logoutCurrent(cc context.Context, isForLogin bool) error {
	c, err := newAPIClient(cc)
	if err != nil {
		return err
	}

	if err := c.Logout(cc); err != nil {
		confirm := "Do you want to remove credentials from .netrc anyway? [y/N]"
		if isForLogin {
			confirm = "Do you want to ignore & continue with your login? [y/N]"
		}
		term := ctx.Terminal(cc)
		term.Printf("Error deactivating your old CLI credentials: %s\n", err)
		if ok, _ := terminal.PromptConfirm(term, confirm); !ok {
			return err
		}
	}

	return ctx.Auther(cc).Wipe()
}

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogin() *cobra.Command {
	var interactiveFlag bool

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate into Gemfury account",
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			auth := ctx.Auther(cc)

			// Logout previous CLI token, if present in .netrc
			if _, token, err := auth.Auth(); err == nil && token != "" {
				if err := logoutCurrent(cc, true); err != nil {
					return err
				}
			}

			// Start browser or interactive authentication
			user, err := ensureAuthenticated(cmd, interactiveFlag)
			if errors.Is(err, promptui.ErrAbort) {
				return nil // User-cancelled
			} else if err != nil {
				return err
			}

			// Verify auth
			if user == nil {
				user, err = whoAMI(cc)
				if err != nil {
					return err
				}
			}

			term := ctx.Terminal(cc)

			if ctx.GlobalFlags(cmd.Context()).AuthToken != "" {
				term.Printf("API token belongs to %q\n", user.Name)
			} else {
				term.Printf("You are logged in as %q\n", user.Email)
			}

			return nil
		},
	}

	// Flags and options
	loginCmd.Flags().BoolVar(&interactiveFlag, "interactive", false, "Interactive login")

	return loginCmd
}
