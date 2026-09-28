package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
)

// NewCmdLogout invalidates session and wipes credentials
func NewCmdLogout() *cobra.Command {
	logoutCmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear CLI session credentials",
		Example: `  fury logout
  fury logout --yes`,
		Args:        noArgs,
		Annotations: skipAuth(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			auth := ctx.Auther(cc)

			// Logout acts on saved credentials only. --api-token is rejected,
			// as a token given that way is never stored; FURY_TOKEN is ignored
			// rather than rejected, as it is set for all commands.
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

			if ok, err := term.Confirm("Are you sure you want to logout? [y/N]"); !ok {
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
// given inline for the current invocation. If the server refuses the
// revocation, the user is asked onFailConfirm before wiping anyway,
// and declining leaves that refusal as the error.
func logoutCurrent(cc context.Context, token string, onFailConfirm string) error {
	c := newAPIClientWithToken(cc, token)

	if err := c.Logout(cc); errors.Is(err, context.Canceled) {
		return err // Interrupted, rather than refused
	} else if err != nil {
		term := ctx.Terminal(cc)
		fmt.Fprintf(term.IOErr(), "Error deactivating your old CLI credentials: %s\n", err)
		if ok, promptErr := term.Confirm(onFailConfirm); promptErr != nil {
			return promptErr
		} else if !ok {
			return err
		}
	}

	return ctx.Auther(cc).Wipe()
}

// NewCmdLogin authenticates, replacing any saved CLI session
func NewCmdLogin() *cobra.Command {
	var interactiveFlag bool

	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate into Gemfury account",
		Long: `Authenticate into Gemfury account, and save the session for the
commands that follow. Login is by the browser, or with --interactive
by email and password at the terminal.

With a token, by --api-token or FURY_TOKEN, that token is verified and
nothing is saved.`,
		Example: `  fury login
  fury login --interactive
  FURY_TOKEN=token fury login`,
		Args:        noArgs,
		Annotations: skipAuth(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			auth := ctx.Auther(cc)
			term := ctx.Terminal(cc)
			inlineToken := inlineAuthToken(cc)

			// Login needs a user to answer its prompts. Without one, fail
			// before the saved session is revoked, rather than after.
			if inlineToken == "" && !term.IsInteractive() {
				return ErrLoginUnattended
			}

			// Logout previous CLI token, if present in .netrc. With an inline
			// token, we only verify it, so saved credentials are left alone.
			if inlineToken == "" {
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

			if inlineToken != "" {
				term.Printf("API token belongs to %q\n", user.Name)
			} else {
				term.Printf("You are logged in as %q\n", user.Email)
			}

			return nil
		},
	}

	// Flags and options
	loginCmd.Flags().BoolVar(&interactiveFlag, "interactive", false, "Login by email and password, not by the browser")

	return loginCmd
}
