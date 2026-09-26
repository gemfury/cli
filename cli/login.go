package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
)

// Machines for Gemfury in .netrc file
var (
	netrcMachines = []string{"api.fury.io", "git.fury.io"}
)

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogout() *cobra.Command {
	logoutCmd := &cobra.Command{
		Use:         "logout",
		Short:       "Clear CLI session credentials",
		Args:        noArgs,
		Annotations: skipAuthAnnotation,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			auth := ctx.Auther(cc)

			// Logout acts on saved credentials only; a token passed inline
			// is never stored, so there is nothing to clear for it.
			if ctx.GlobalFlags(cc).AuthToken != "" {
				return usageErrorf("Logout clears saved credentials only; do not pass --api-token")
			}

			_, token, err := auth.Auth()
			if err != nil {
				return err
			} else if token == "" {
				term.Println("You are logged out")
				return nil
			}

			if ok, err := terminal.PromptConfirm(term, "Are you sure you want to logout? [y/N]"); !ok {
				return err
			}

			wipeAnyway := "Do you want to remove credentials from .netrc anyway? [y/N]"
			if err := logoutCurrent(cc, token, wipeAnyway); err != nil {
				return err
			}

			term.Println("You have been logged out")
			return nil
		},
	}

	return logoutCmd
}

// Deactivates & deletes the saved CLI token. The token is passed explicitly
// so that the revocation always targets the saved credentials, never a token
// supplied via --api-token for the current invocation. If the server refuses
// the revocation, the user is asked onFailConfirm before wiping anyway.
func logoutCurrent(cc context.Context, token string, onFailConfirm string) error {
	c := newAPIClientWithToken(cc, token)

	if err := c.Logout(cc); err != nil {
		term := ctx.Terminal(cc)
		fmt.Fprintf(term.IOErr(), "Error deactivating your old CLI credentials: %s\n", err)
		if ok, _ := terminal.PromptConfirm(term, onFailConfirm); !ok {
			return err
		}
	}

	return ctx.Auther(cc).Wipe()
}

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogin() *cobra.Command {
	var interactiveFlag bool

	loginCmd := &cobra.Command{
		Use:         "login",
		Short:       "Authenticate into Gemfury account",
		Args:        noArgs,
		Annotations: skipAuthAnnotation,
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			auth := ctx.Auther(cc)
			tokenFlag := ctx.GlobalFlags(cc).AuthToken

			// Logout previous CLI token, if present in .netrc. With --api-token
			// we only verify the given token, so saved credentials are left alone.
			if tokenFlag == "" {
				if _, token, err := auth.Auth(); err == nil && token != "" {
					confirm := "Do you want to ignore & continue with your login? [y/N]"
					if err := logoutCurrent(cc, token, confirm); err != nil {
						return err
					}
				}
			}

			// Start browser or interactive authentication
			user, err := ensureAuthenticated(cmd, interactiveFlag)
			if errors.Is(err, errLoginCancelled) {
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

			if tokenFlag != "" {
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
