package cli

import (
	"github.com/gemfury/cli/internal/ctx"
	"github.com/spf13/cobra"

	"context"
	"errors"
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

			// --api-token is rejected rather than ignored, as it would seem to
			// name the token to revoke; FURY_TOKEN is ignored, as it is set
			// for all commands
			if ctx.GlobalFlags(cc).AuthToken != "" {
				return usageErrorf("Logout clears saved credentials only; do not pass --api-token")
			}

			_, token, err := auth.Auth()
			if err != nil {
				return err
			} else if token == "" {
				term.Infof("You are logged out\n")
				return nil
			}

			if ok, err := term.Confirm("Are you sure you want to logout? [y/N]"); !ok {
				return err
			}

			wipeAnyway := "Do you want to remove saved credentials anyway? [y/N]"
			if err := logoutCurrent(cc, token, wipeAnyway); err != nil {
				return err
			}

			term.Infof("You have been logged out\n")
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
	c, err := newAPIClientWithToken(cc, token)
	if err != nil {
		return err
	}

	if err := c.Logout(cc); errors.Is(err, context.Canceled) {
		return err // Interrupted, rather than refused
	} else if err != nil {
		term := ctx.Terminal(cc)
		term.EPrintf("Error deactivating your old CLI credentials: %s\n", err)
		if ok, promptErr := term.Confirm(onFailConfirm); promptErr != nil {
			return promptErr
		} else if !ok {
			return err
		}
	}

	return ctx.Auther(cc).Wipe()
}

// logoutSaved revokes and clears the session saved before, if any, asking
// whether to go on when the server refuses. When the saved token is keep,
// it is left alone, as keep is to stay valid
func logoutSaved(cc context.Context, keep string) error {
	if _, token, err := ctx.Auther(cc).Auth(); err == nil && token != "" && token != keep {
		confirm := "Do you want to ignore & continue with your login? [y/N]"
		return logoutCurrent(cc, token, confirm)
	}
	return nil
}

// loginWithToken verifies token, revokes any session saved before,
// and saves token as the session
func loginWithToken(cc context.Context, token string) error {
	user, err := whoAMI(cc)
	if err != nil {
		return err
	}

	if err := logoutSaved(cc, token); err != nil {
		return err
	}

	login := user.Login()
	if err := saveSession(cc, login, token); err != nil {
		return err
	}

	ctx.Terminal(cc).Infof("You are logged in as %q\n", login)
	return nil
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

The session is saved in the system keychain, and Git is set to ask this
CLI for it when it authenticates with git.fury.io. Without a keychain,
it is saved in the .netrc file instead.

With --api-token, that token is verified and saved as the session, in
place of any saved before. With --api-token -, it is read from stdin, or
asked for at the terminal. With FURY_TOKEN alone, it is verified and
nothing is saved.`,
		Example: `  fury login
  fury login --interactive
  fury login --api-token -
  FURY_TOKEN=token fury login`,
		Args:        noArgs,
		Annotations: skipAuth(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cc := cmd.Context()
			term := ctx.Terminal(cc)
			inlineToken, err := inlineAuthToken(cc)
			if err != nil {
				return err
			}

			if ctx.GlobalFlags(cc).AuthToken != "" {
				return loginWithToken(cc, inlineToken)
			} else if inlineToken != "" { // FURY_TOKEN, set for every command, is only verified
				user, err := whoAMI(cc)
				if err != nil {
					return err
				}
				term.Infof("API token belongs to %q\n", user.Name)
				return nil
			}

			// Login needs a user to answer its prompts. Without one, fail
			// before the saved session is revoked, rather than after.
			if !term.IsInteractive() {
				return ErrLoginUnattended
			}

			if err := logoutSaved(cc, ""); err != nil {
				return err
			}

			// Start browser or interactive authentication
			user, err := ensureAuthenticated(cmd, interactiveFlag)
			if errors.Is(err, errLoginCancelled) {
				return nil // User-cancelled
			} else if err != nil {
				return err
			}

			term.Infof("You are logged in as %q\n", user.Login())
			return nil
		},
	}

	// Flags and options
	loginCmd.Flags().BoolVar(&interactiveFlag, "interactive", false, "Login by email and password, not by the browser")

	return loginCmd
}
