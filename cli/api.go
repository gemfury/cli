package cli

import (
	"github.com/cenkalti/backoff/v7"
	"github.com/gemfury/cli/api"
	"github.com/gemfury/cli/internal/ctx"
	"github.com/gemfury/cli/pkg/terminal"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Initialize new Gemfury API client with authentication
func newAPIClient(cc context.Context) (*api.Client, error) {
	// Token comes from CLI flags or .netrc
	token, err := contextAuthToken(cc)
	if err != nil {
		return nil, err
	}

	return newAPIClientWithToken(cc, token), nil
}

// Initialize new Gemfury API client with an explicit token,
// bypassing the usual flag-then-netrc resolution
func newAPIClientWithToken(cc context.Context, token string) *api.Client {
	flags := ctx.GlobalFlags(cc)
	c := api.NewClient(token, flags.Account)

	// Endpoint overrides (testing, staging). The client joins paths onto
	// these and compares URLs against them, so normalize away a trailing "/".
	if e := flags.PushEndpoint; e != "" {
		c.PushEndpoint = strings.TrimSuffix(e, "/")
	}
	if e := flags.Endpoint; e != "" {
		c.Endpoint = strings.TrimSuffix(e, "/")
	}

	return c
}

// Extract authentication token from context (flag or .netrc)
func contextAuthToken(cc context.Context) (string, error) {
	if token := ctx.GlobalFlags(cc).AuthToken; token != "" {
		return token, nil
	}
	_, token, err := ctx.Auther(cc).Auth()
	return token, err
}

// skipAuthAnnotation marks a command that must run without authentication,
// such as the ones that manage the session itself
const skipAuthKey = "fury.skip-auth"

var skipAuthAnnotation = map[string]string{skipAuthKey: "true"}

// skipsAuth reports whether cmd runs without authentication: commands
// annotated with skipAuthAnnotation, plus Cobra's built-in top-level
// help and shell-completion commands
func skipsAuth(cmd *cobra.Command) bool {
	if cmd.Annotations[skipAuthKey] == "true" {
		return true
	}

	if cmd.Parent() == cmd.Root() {
		switch cmd.Name() {
		case "help", cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
			return true
		}
	}

	return false
}

// Hook for root command to ensure user is authenticated or prompt to login
func preRunCheckAuthentication(cmd *cobra.Command, args []string) error {
	if skipsAuth(cmd) {
		return nil
	}

	_, err := ensureAuthenticated(cmd, false)
	return err
}

// errLoginCancelled is returned when the user backs out of the login
// prompt. "login" exits quietly on it; other commands report it.
var errLoginCancelled = errors.New("Login cancelled")

func ensureAuthenticated(cmd *cobra.Command, interactive bool) (*api.AccountResponse, error) {
	cc := cmd.Context()
	var err error

	// Check whether we have login credentials from environment
	if token, err := contextAuthToken(cc); token != "" || err != nil {
		return nil, err
	}

	// Trigger browser login
	var resp *api.LoginResponse
	if !interactive {
		resp, err = browserLogin(cmd)
		interactive = errors.Is(err, api.ErrNotImplemented)
	}

	// Trigger interactive login if requested by user
	// or if browser returned "not-implemented"
	if interactive {
		resp, err = interactiveLogin(cmd)
	}

	if errors.Is(err, promptui.ErrAbort) {
		return nil, errLoginCancelled
	} else if err != nil {
		return nil, err
	}

	// Save credentials to .netrc for future commands
	err = ctx.Auther(cc).Append(resp.User.Email, resp.Token)
	if err != nil {
		return nil, err
	}

	return &resp.User, nil
}

// browserLogin is a challenge/response authentication via browser
func browserLogin(cmd *cobra.Command) (*api.LoginResponse, error) {
	cc := cmd.Context()
	term := ctx.Terminal(cc)

	c, err := newAPIClient(cc)
	if err != nil {
		return nil, err
	}

	// Generate authentication URLs
	createResp, err := c.LoginCreate(cc)
	if err != nil {
		return nil, err
	} else if createResp.BrowserURL == "" {
		return nil, fmt.Errorf("Internal error")
	}

	// Everything is ready. Confirm opening browser to login
	anyKey := "Press any key to login via the browser or q to exit: "
	if err := terminal.PromptAnyKeyOrQuit(term, anyKey); err != nil {
		return nil, err
	}

	// Attempt to open the browser to create CLI token
	term.Printf("Opening %s\n", createResp.BrowserURL)
	if ok := term.OpenBrowser(createResp.BrowserURL); !ok {
		term.Printf("Failed to open browser. You can continue CLI login by manually opening the URL\n")
	}

	// Start/end spinner while waiting for browser auth
	onDone := terminal.SpinIfTerminal(term, " Waiting ...")
	defer onDone()

	// LoginGet API will block & timeout, so we poll until a time limit.
	resp, err := backoff.Retry(cc, func() (*api.LoginGetResponse, error) {
		resp, err := c.LoginGet(cc, createResp)
		if !errors.Is(err, api.ErrTimeout) && !errors.Is(err, api.ErrNotFound) {
			err = backoff.Permanent(err) // Retry only on timeout or not-found
		}
		return resp, err
	},
		// We retry with constant backoff waiting for user to approve login.
		// Max elapsed time is shorter than the expiry of all the JWT tokens.
		backoff.WithBackOff(backoff.NewConstantBackOff(500*time.Millisecond)),
		backoff.WithMaxElapsedTime(3*time.Minute),
	)

	if resp == nil {
		return nil, err
	}

	if resp.Error != "" {
		err = errors.New(resp.Error)
	} else if errors.Is(err, api.ErrNotFound) {
		err = api.ErrTimeout
	}

	return &resp.LoginResponse, err
}

// interactiveLogin is an email/password authentication via terminal
func interactiveLogin(cmd *cobra.Command) (*api.LoginResponse, error) {
	cc := cmd.Context()

	// Interactive login
	term := ctx.Terminal(cc)
	term.Println("Please enter your Gemfury credentials.")

	ePrompt := promptui.Prompt{Label: "Email: "}
	eResult, err := term.RunPrompt(&ePrompt)
	if err != nil {
		return nil, err
	}

	pPrompt := promptui.Prompt{Label: "Password: ", Mask: '*'}
	pResult, err := term.RunPrompt(&pPrompt)
	if err != nil {
		return nil, err
	}

	c, err := newAPIClient(cc)
	if err != nil {
		return nil, err
	}

	req := api.LoginRequest{Email: eResult, Password: pResult}
	return c.Login(cc, &req)
}
